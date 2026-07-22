package session

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"
)

const DefaultTTL = 8 * time.Hour

const sessionDigestPrefix = "sha256:v1:"

var (
	ErrInvalidSession       = errors.New("invalid session")
	ErrInvalidSessionDigest = errors.New("invalid session digest")
)

type Options struct {
	TTL time.Duration
	Now func() time.Time
}

type Snapshot struct {
	Sessions map[string]StoredSession `json:"sessions"`
}

type Repository interface {
	Load(context.Context) (Snapshot, error)
	Create(context.Context, StoredSession) error
	Resolve(context.Context, string, time.Time) (StoredSession, bool, error)
	Renew(context.Context, string, time.Time, time.Time) (StoredSession, bool, error)
	Revoke(context.Context, string, time.Time) (StoredSession, bool, error)
}

type Session struct {
	Token     string    `json:"-"`
	Username  string    `json:"username"`
	IssuedAt  time.Time `json:"issuedAt"`
	ExpiresAt time.Time `json:"expiresAt"`
	RevokedAt time.Time `json:"revokedAt,omitempty"`
}

type StoredSession struct {
	TokenDigest string    `json:"tokenDigest"`
	Username    string    `json:"username"`
	IssuedAt    time.Time `json:"issuedAt"`
	ExpiresAt   time.Time `json:"expiresAt"`
	RevokedAt   time.Time `json:"revokedAt,omitempty"`
}

type Store struct {
	mu         sync.Mutex
	sessions   map[string]StoredSession
	ttl        time.Duration
	now        func() time.Time
	repository Repository
}

func NewStore(options Options) *Store {
	ttl := options.TTL
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &Store{
		sessions: map[string]StoredSession{},
		ttl:      ttl,
		now:      now,
	}
}

// Close releases an optional repository connection owned by the store.
func (s *Store) Close() error {
	s.mu.Lock()
	repository := s.repository
	s.mu.Unlock()
	if closer, ok := repository.(interface{ Close() error }); ok {
		return closer.Close()
	}
	return nil
}

func NewRepositoryBackedStore(options Options, repository Repository) (*Store, error) {
	store := NewStore(options)
	if repository == nil {
		return store, nil
	}
	snapshot, err := repository.Load(context.Background())
	if err != nil {
		return nil, err
	}
	if err := validateSnapshot(snapshot); err != nil {
		return nil, err
	}
	store.sessions = cloneStoredSessions(snapshot.Sessions)
	store.repository = repository
	return store, nil
}

func (s *Store) Reload() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.repository == nil {
		return nil
	}
	snapshot, err := s.repository.Load(context.Background())
	if err != nil {
		return err
	}
	if err := validateSnapshot(snapshot); err != nil {
		return err
	}
	s.sessions = cloneStoredSessions(snapshot.Sessions)
	return nil
}

func (s *Store) Issue(username string) (Session, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return Session{}, ErrInvalidSession
	}
	token, err := newToken()
	if err != nil {
		return Session{}, err
	}
	issuedAt := s.now().UTC()
	session := Session{
		Token:     token,
		Username:  username,
		IssuedAt:  issuedAt,
		ExpiresAt: issuedAt.Add(s.ttl),
	}
	tokenDigest := DigestToken(token)
	stored := storedSession(tokenDigest, session)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.repository != nil {
		if err := s.repository.Create(context.Background(), stored); err != nil {
			return Session{}, err
		}
	}
	s.sessions[tokenDigest] = stored
	return session, nil
}

func (s *Store) ResolveContext(ctx context.Context, token string) (Session, bool, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return Session{}, false, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now().UTC()
	tokenDigest := DigestToken(token)
	if s.repository != nil {
		stored, ok, err := s.repository.Resolve(ctx, tokenDigest, now)
		if err != nil || !ok {
			return Session{}, false, err
		}
		if err := validateStoredSessionForKey(tokenDigest, stored); err != nil {
			return Session{}, false, err
		}
		s.sessions[tokenDigest] = stored
		return publicSession(token, stored), true, nil
	}
	stored, ok := s.sessions[tokenDigest]
	if !ok || !stored.RevokedAt.IsZero() || !now.Before(stored.ExpiresAt) {
		return Session{}, false, nil
	}
	return publicSession(token, stored), true, nil
}

func (s *Store) Resolve(token string) (Session, bool) {
	session, ok, err := s.ResolveContext(context.Background(), token)
	if err != nil {
		return Session{}, false
	}
	return session, ok
}

func (s *Store) RenewContext(ctx context.Context, token string) (Session, bool, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return Session{}, false, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now().UTC()
	expiresAt := now.Add(s.ttl)
	tokenDigest := DigestToken(token)
	if s.repository != nil {
		stored, ok, err := s.repository.Renew(ctx, tokenDigest, now, expiresAt)
		if err != nil || !ok {
			return Session{}, false, err
		}
		if err := validateStoredSessionForKey(tokenDigest, stored); err != nil {
			return Session{}, false, err
		}
		s.sessions[tokenDigest] = stored
		return publicSession(token, stored), true, nil
	}
	stored, ok := s.sessions[tokenDigest]
	if !ok || !stored.RevokedAt.IsZero() || !now.Before(stored.ExpiresAt) {
		return Session{}, false, nil
	}
	stored.ExpiresAt = expiresAt
	s.sessions[tokenDigest] = stored
	return publicSession(token, stored), true, nil
}

func (s *Store) Renew(token string) (Session, bool) {
	session, ok, err := s.RenewContext(context.Background(), token)
	if err != nil {
		return Session{}, false
	}
	return session, ok
}

func (s *Store) RevokeContext(ctx context.Context, token string) (bool, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return false, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now().UTC()
	tokenDigest := DigestToken(token)
	if s.repository != nil {
		stored, ok, err := s.repository.Revoke(ctx, tokenDigest, now)
		if err != nil || !ok {
			return false, err
		}
		if err := validateStoredSessionForKey(tokenDigest, stored); err != nil {
			return false, err
		}
		s.sessions[tokenDigest] = stored
		return true, nil
	}
	stored, ok := s.sessions[tokenDigest]
	if !ok || !stored.RevokedAt.IsZero() || !now.Before(stored.ExpiresAt) {
		return false, nil
	}
	stored.RevokedAt = now
	s.sessions[tokenDigest] = stored
	return true, nil
}

func (s *Store) Revoke(token string) bool {
	ok, err := s.RevokeContext(context.Background(), token)
	if err != nil {
		return false
	}
	return ok
}

func DigestToken(raw string) string {
	sum := sha256.Sum256(append([]byte("platform-session\x00"), []byte(raw)...))
	return sessionDigestPrefix + hex.EncodeToString(sum[:])
}

func validateTokenDigest(tokenDigest string) error {
	if len(tokenDigest) != len(sessionDigestPrefix)+sha256.Size*2 || !strings.HasPrefix(tokenDigest, sessionDigestPrefix) {
		return ErrInvalidSessionDigest
	}
	for index := len(sessionDigestPrefix); index < len(tokenDigest); index++ {
		character := tokenDigest[index]
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return ErrInvalidSessionDigest
		}
	}
	return nil
}

func validateStoredSessionForKey(tokenDigest string, session StoredSession) error {
	if err := validateTokenDigest(tokenDigest); err != nil {
		return err
	}
	if err := validateTokenDigest(session.TokenDigest); err != nil {
		return err
	}
	if session.TokenDigest != tokenDigest {
		return ErrInvalidSessionDigest
	}
	return nil
}

func validateSnapshot(snapshot Snapshot) error {
	for tokenDigest, session := range snapshot.Sessions {
		if err := validateStoredSessionForKey(tokenDigest, session); err != nil {
			return err
		}
	}
	return nil
}

func storedSession(tokenDigest string, session Session) StoredSession {
	return StoredSession{
		TokenDigest: tokenDigest,
		Username:    session.Username,
		IssuedAt:    session.IssuedAt,
		ExpiresAt:   session.ExpiresAt,
		RevokedAt:   session.RevokedAt,
	}
}

func publicSession(rawToken string, stored StoredSession) Session {
	return Session{
		Token:     rawToken,
		Username:  stored.Username,
		IssuedAt:  stored.IssuedAt,
		ExpiresAt: stored.ExpiresAt,
		RevokedAt: stored.RevokedAt,
	}
}

func cloneStoredSessions(sessions map[string]StoredSession) map[string]StoredSession {
	cloned := make(map[string]StoredSession, len(sessions))
	for tokenDigest, session := range sessions {
		cloned[tokenDigest] = session
	}
	return cloned
}

func newToken() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}
