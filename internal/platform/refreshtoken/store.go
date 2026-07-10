package refreshtoken

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"
)

const DefaultTTL = 30 * 24 * time.Hour

var (
	ErrInvalidRefreshToken       = errors.New("invalid refresh token")
	ErrRefreshTokenReuseDetected = errors.New("refresh token reuse detected")
)

type Status string

const (
	StatusActive  Status = "active"
	StatusRotated Status = "rotated"
	StatusRevoked Status = "revoked"
	StatusReused  Status = "reused"
	StatusExpired Status = "expired"
)

type Options struct {
	TTL     time.Duration
	HashKey []byte
	Now     func() time.Time
}

type IssueInput struct {
	SessionID string
	Username  string
	TenantID  string
	TokenType string
}

type IssueResult struct {
	RefreshToken string
	Record       Record
}

type RotationResult struct {
	RefreshToken     string
	Record           Record
	SessionExpiresAt time.Time
}

type Effects struct {
	RenewSession        func(context.Context, string) (time.Time, error)
	RevokeSession       func(context.Context, string) error
	PublishInvalidation func(context.Context) error
	Audit               func(context.Context, AuditEvent) error
}

type AuditEvent struct {
	Action       string
	Actor        string
	Resource     string
	FamilyID     string
	TokenID      string
	SessionID    string
	CreatedAt    time.Time
	RefreshToken string
	TokenHash    string
}

type Record struct {
	FamilyID          string    `json:"familyId"`
	TokenID           string    `json:"tokenId"`
	ParentTokenID     string    `json:"parentTokenId,omitempty"`
	SessionID         string    `json:"sessionId"`
	Username          string    `json:"username"`
	TenantID          string    `json:"tenantId"`
	TokenType         string    `json:"tokenType"`
	IssuedAt          time.Time `json:"issuedAt"`
	ExpiresAt         time.Time `json:"expiresAt"`
	RotatedAt         time.Time `json:"rotatedAt,omitempty"`
	RevokedAt         time.Time `json:"revokedAt,omitempty"`
	ReusedAt          time.Time `json:"reusedAt,omitempty"`
	ReplacedByTokenID string    `json:"replacedByTokenId,omitempty"`
	TokenHash         string    `json:"tokenHash,omitempty"`
}

type PublicRecord struct {
	FamilyID          string    `json:"familyId"`
	TokenID           string    `json:"tokenId"`
	ParentTokenID     string    `json:"parentTokenId,omitempty"`
	SessionID         string    `json:"sessionId"`
	Username          string    `json:"username"`
	TenantID          string    `json:"tenantId"`
	TokenType         string    `json:"tokenType"`
	Status            Status    `json:"status"`
	IssuedAt          time.Time `json:"issuedAt"`
	ExpiresAt         time.Time `json:"expiresAt"`
	RotatedAt         time.Time `json:"rotatedAt,omitempty"`
	RevokedAt         time.Time `json:"revokedAt,omitempty"`
	ReusedAt          time.Time `json:"reusedAt,omitempty"`
	ReplacedByTokenID string    `json:"replacedByTokenId,omitempty"`
	TokenHash         string    `json:"-"`
}

type Snapshot struct {
	Records map[string]Record `json:"records"`
}

type Repository interface {
	Load(context.Context) (Snapshot, error)
	Save(context.Context, Snapshot) error
}

type Store struct {
	mu         sync.Mutex
	records    map[string]Record
	ttl        time.Duration
	hashKey    []byte
	now        func() time.Time
	repository Repository
}

func NewStore(options Options) (*Store, error) {
	store, err := newEmptyStore(options)
	if err != nil {
		return nil, err
	}
	return store, nil
}

func NewRepositoryBackedStore(options Options, repository Repository) (*Store, error) {
	store, err := newEmptyStore(options)
	if err != nil {
		return nil, err
	}
	if repository == nil {
		return store, nil
	}
	snapshot, err := repository.Load(context.Background())
	if err != nil {
		return nil, err
	}
	store.records = cloneRecords(snapshot.Records)
	store.repository = repository
	return store, nil
}

func newEmptyStore(options Options) (*Store, error) {
	if len(options.HashKey) == 0 {
		return nil, errors.New("refresh token hash key is required")
	}
	ttl := options.TTL
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &Store{
		records: map[string]Record{},
		ttl:     ttl,
		hashKey: append([]byte(nil), options.HashKey...),
		now:     now,
	}, nil
}

func (s *Store) Issue(ctx context.Context, input IssueInput) (IssueResult, error) {
	input.SessionID = strings.TrimSpace(input.SessionID)
	input.Username = strings.TrimSpace(input.Username)
	input.TenantID = strings.TrimSpace(input.TenantID)
	input.TokenType = strings.TrimSpace(input.TokenType)
	if input.SessionID == "" || input.Username == "" || input.TenantID == "" || input.TokenType == "" {
		return IssueResult{}, ErrInvalidRefreshToken
	}
	refreshToken, err := newToken()
	if err != nil {
		return IssueResult{}, err
	}
	familyID, err := newToken()
	if err != nil {
		return IssueResult{}, err
	}
	tokenID, err := newToken()
	if err != nil {
		return IssueResult{}, err
	}
	now := s.now().UTC()
	record := Record{
		FamilyID:  familyID,
		TokenID:   tokenID,
		SessionID: input.SessionID,
		Username:  input.Username,
		TenantID:  input.TenantID,
		TokenType: input.TokenType,
		IssuedAt:  now,
		ExpiresAt: now.Add(s.ttl),
		TokenHash: s.hash(refreshToken),
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.records[record.TokenID] = record
	if err := s.persistLocked(ctx); err != nil {
		delete(s.records, record.TokenID)
		return IssueResult{}, err
	}
	return IssueResult{RefreshToken: refreshToken, Record: record}, nil
}

func (s *Store) Rotate(ctx context.Context, refreshToken string, effects Effects) (RotationResult, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return RotationResult{}, ErrInvalidRefreshToken
	}
	now := s.now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.findByHashLocked(s.hash(refreshToken))
	if !ok {
		_ = callAudit(ctx, effects, AuditEvent{Action: "auth.refresh.reuse_detected", Resource: "auth", CreatedAt: now})
		return RotationResult{}, ErrRefreshTokenReuseDetected
	}
	if !current.ReusedAt.IsZero() || !current.RevokedAt.IsZero() || !current.RotatedAt.IsZero() || !now.Before(current.ExpiresAt) {
		if err := s.markReuseLocked(ctx, current, now, effects); err != nil {
			return RotationResult{}, err
		}
		return RotationResult{}, ErrRefreshTokenReuseDetected
	}

	nextRefreshToken, err := newToken()
	if err != nil {
		return RotationResult{}, err
	}
	nextTokenID, err := newToken()
	if err != nil {
		return RotationResult{}, err
	}
	sessionExpiresAt := time.Time{}
	if effects.RenewSession != nil {
		renewedAt, err := effects.RenewSession(ctx, current.SessionID)
		if err != nil {
			return RotationResult{}, err
		}
		sessionExpiresAt = renewedAt.UTC()
	}
	previous := current
	current.RotatedAt = now
	current.ReplacedByTokenID = nextTokenID
	next := Record{
		FamilyID:      current.FamilyID,
		TokenID:       nextTokenID,
		ParentTokenID: current.TokenID,
		SessionID:     current.SessionID,
		Username:      current.Username,
		TenantID:      current.TenantID,
		TokenType:     current.TokenType,
		IssuedAt:      now,
		ExpiresAt:     now.Add(s.ttl),
		TokenHash:     s.hash(nextRefreshToken),
	}
	s.records[current.TokenID] = current
	s.records[next.TokenID] = next
	if err := s.persistLocked(ctx); err != nil {
		s.records[previous.TokenID] = previous
		delete(s.records, next.TokenID)
		return RotationResult{}, err
	}
	if err := callInvalidation(ctx, effects); err != nil {
		return RotationResult{}, err
	}
	if err := callAudit(ctx, effects, AuditEvent{
		Action:    "auth.refresh.rotate",
		Actor:     current.Username,
		Resource:  "auth",
		FamilyID:  current.FamilyID,
		TokenID:   next.TokenID,
		SessionID: current.SessionID,
		CreatedAt: now,
	}); err != nil {
		return RotationResult{}, err
	}
	return RotationResult{RefreshToken: nextRefreshToken, Record: next, SessionExpiresAt: sessionExpiresAt}, nil
}

func (s *Store) RevokeFamily(ctx context.Context, familyID string, effects Effects) error {
	familyID = strings.TrimSpace(familyID)
	if familyID == "" {
		return ErrInvalidRefreshToken
	}
	now := s.now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	sessionIDs := map[string]struct{}{}
	found := false
	for tokenID, record := range s.records {
		if record.FamilyID != familyID {
			continue
		}
		found = true
		if record.RevokedAt.IsZero() {
			record.RevokedAt = now
			s.records[tokenID] = record
		}
		sessionIDs[record.SessionID] = struct{}{}
	}
	if !found {
		return ErrInvalidRefreshToken
	}
	if err := s.persistLocked(ctx); err != nil {
		return err
	}
	for sessionID := range sessionIDs {
		if effects.RevokeSession != nil {
			if err := effects.RevokeSession(ctx, sessionID); err != nil {
				return err
			}
		}
	}
	if err := callInvalidation(ctx, effects); err != nil {
		return err
	}
	return callAudit(ctx, effects, AuditEvent{Action: "auth.logout", Resource: "auth", FamilyID: familyID, CreatedAt: now})
}

func (s *Store) Family(familyID string) []PublicRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now().UTC()
	records := make([]PublicRecord, 0)
	for _, record := range s.records {
		if record.FamilyID == familyID {
			records = append(records, publicRecord(record, now))
		}
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].IssuedAt.Equal(records[j].IssuedAt) {
			return records[i].TokenID < records[j].TokenID
		}
		return records[i].IssuedAt.Before(records[j].IssuedAt)
	})
	return records
}

func (s *Store) findByHashLocked(tokenHash string) (Record, bool) {
	for _, record := range s.records {
		if hmac.Equal([]byte(record.TokenHash), []byte(tokenHash)) {
			return record, true
		}
	}
	return Record{}, false
}

func (s *Store) markReuseLocked(ctx context.Context, reused Record, now time.Time, effects Effects) error {
	sessionIDs := map[string]struct{}{}
	for tokenID, record := range s.records {
		if record.FamilyID != reused.FamilyID {
			continue
		}
		if record.TokenID == reused.TokenID && record.ReusedAt.IsZero() {
			record.ReusedAt = now
		}
		if record.RevokedAt.IsZero() {
			record.RevokedAt = now
		}
		s.records[tokenID] = record
		sessionIDs[record.SessionID] = struct{}{}
	}
	if err := s.persistLocked(ctx); err != nil {
		return err
	}
	for sessionID := range sessionIDs {
		if effects.RevokeSession != nil {
			if err := effects.RevokeSession(ctx, sessionID); err != nil {
				return err
			}
		}
	}
	if err := callInvalidation(ctx, effects); err != nil {
		return err
	}
	return callAudit(ctx, effects, AuditEvent{
		Action:    "auth.refresh.reuse_detected",
		Actor:     reused.Username,
		Resource:  "auth",
		FamilyID:  reused.FamilyID,
		TokenID:   reused.TokenID,
		SessionID: reused.SessionID,
		CreatedAt: now,
	})
}

func (s *Store) persistLocked(ctx context.Context) error {
	if s.repository == nil {
		return nil
	}
	return s.repository.Save(ctx, Snapshot{Records: cloneRecords(s.records)})
}

func (s *Store) hash(refreshToken string) string {
	mac := hmac.New(sha256.New, s.hashKey)
	_, _ = mac.Write([]byte(refreshToken))
	return hex.EncodeToString(mac.Sum(nil))
}

func publicRecord(record Record, now time.Time) PublicRecord {
	status := StatusActive
	switch {
	case !record.ReusedAt.IsZero():
		status = StatusReused
	case !record.RevokedAt.IsZero():
		status = StatusRevoked
	case !record.RotatedAt.IsZero():
		status = StatusRotated
	case !now.Before(record.ExpiresAt):
		status = StatusExpired
	}
	return PublicRecord{
		FamilyID:          record.FamilyID,
		TokenID:           record.TokenID,
		ParentTokenID:     record.ParentTokenID,
		SessionID:         record.SessionID,
		Username:          record.Username,
		TenantID:          record.TenantID,
		TokenType:         record.TokenType,
		Status:            status,
		IssuedAt:          record.IssuedAt,
		ExpiresAt:         record.ExpiresAt,
		RotatedAt:         record.RotatedAt,
		RevokedAt:         record.RevokedAt,
		ReusedAt:          record.ReusedAt,
		ReplacedByTokenID: record.ReplacedByTokenID,
	}
}

func cloneRecords(records map[string]Record) map[string]Record {
	cloned := make(map[string]Record, len(records))
	for tokenID, record := range records {
		cloned[tokenID] = record
	}
	return cloned
}

func callInvalidation(ctx context.Context, effects Effects) error {
	if effects.PublishInvalidation == nil {
		return nil
	}
	return effects.PublishInvalidation(ctx)
}

func callAudit(ctx context.Context, effects Effects, event AuditEvent) error {
	if effects.Audit == nil {
		return nil
	}
	event.RefreshToken = ""
	event.TokenHash = ""
	return effects.Audit(ctx, event)
}

func newToken() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}
