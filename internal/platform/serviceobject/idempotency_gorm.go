package serviceobject

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync/atomic"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	idempotencyTable             = "platform_service_object_idempotency"
	idempotencyStatusProcessing  = "processing"
	idempotencyStatusCompleted   = "completed"
	defaultIdempotencyTTL        = 24 * time.Hour
	defaultIdempotencyLease      = 30 * time.Second
	defaultIdempotencyPoll       = 20 * time.Millisecond
	defaultIdempotencyCleanup    = 5 * time.Minute
	defaultIdempotencyFinalize   = 5 * time.Second
	idempotencyOwnerTokenBytes   = 24
	idempotencyFingerprintLength = 128
)

type GORMIdempotencyStoreOptions struct {
	TTL             time.Duration
	LeaseDuration   time.Duration
	PollInterval    time.Duration
	CleanupInterval time.Duration
	Now             func() time.Time
}

type GORMIdempotencyStore struct {
	db              *gorm.DB
	ttl             time.Duration
	leaseDuration   time.Duration
	pollInterval    time.Duration
	cleanupInterval time.Duration
	now             func() time.Time
	lastCleanup     atomic.Int64
}

type gormIdempotencyRecord struct {
	ScopeDigest    string    `gorm:"primaryKey;size:64;column:scope_digest"`
	CommandID      string    `gorm:"size:256;not null;column:command_id"`
	Version        string    `gorm:"size:64;not null;column:version"`
	Actor          string    `gorm:"size:256;not null;column:actor"`
	TenantID       string    `gorm:"size:256;not null;column:tenant_id"`
	IdempotencyKey string    `gorm:"size:128;not null;column:idempotency_key"`
	Fingerprint    string    `gorm:"size:128;not null;column:fingerprint"`
	Status         string    `gorm:"size:32;not null;index;column:status"`
	OwnerToken     string    `gorm:"size:64;not null;column:owner_token"`
	ResultJSON     []byte    `gorm:"column:result_json"`
	LeaseExpiresAt time.Time `gorm:"not null;index;column:lease_expires_at"`
	ExpiresAt      time.Time `gorm:"not null;index;column:expires_at"`
	CreatedAt      time.Time `gorm:"not null;column:created_at"`
	UpdatedAt      time.Time `gorm:"not null;column:updated_at"`
}

func (gormIdempotencyRecord) TableName() string {
	return idempotencyTable
}

func NewGORMIdempotencyStore(ctx context.Context, db *gorm.DB, options GORMIdempotencyStoreOptions) (*GORMIdempotencyStore, error) {
	if ctx == nil || db == nil {
		return nil, ErrRequestInvalid
	}
	options = defaultGORMIdempotencyStoreOptions(options)
	if options.TTL <= 0 || options.LeaseDuration <= 0 || options.PollInterval <= 0 || options.CleanupInterval <= 0 || options.Now == nil {
		return nil, ErrRequestInvalid
	}
	if err := db.WithContext(ctx).AutoMigrate(&gormIdempotencyRecord{}); err != nil {
		return nil, idempotencyDatabaseError(ctx)
	}
	return newGORMIdempotencyStore(db, options), nil
}

func OpenGORMIdempotencyStore(ctx context.Context, db *gorm.DB, options GORMIdempotencyStoreOptions) (*GORMIdempotencyStore, error) {
	if ctx == nil || db == nil {
		return nil, ErrRequestInvalid
	}
	options = defaultGORMIdempotencyStoreOptions(options)
	if options.TTL <= 0 || options.LeaseDuration <= 0 || options.PollInterval <= 0 || options.CleanupInterval <= 0 || options.Now == nil {
		return nil, ErrRequestInvalid
	}
	if !db.WithContext(ctx).Migrator().HasTable(&gormIdempotencyRecord{}) {
		return nil, ErrObjectUnavailable
	}
	return newGORMIdempotencyStore(db, options), nil
}

func newGORMIdempotencyStore(db *gorm.DB, options GORMIdempotencyStoreOptions) *GORMIdempotencyStore {
	return &GORMIdempotencyStore{
		db:              db,
		ttl:             options.TTL,
		leaseDuration:   options.LeaseDuration,
		pollInterval:    options.PollInterval,
		cleanupInterval: options.CleanupInterval,
		now:             options.Now,
	}
}

func (s *GORMIdempotencyStore) Execute(ctx context.Context, scope IdempotencyScope, fingerprint string, execute func(context.Context) (CommandResult, error)) (CommandResult, error) {
	if s == nil || s.db == nil || ctx == nil || execute == nil || !validIdempotencyScope(scope) || !validIdempotencyFingerprint(fingerprint) {
		return CommandResult{}, ErrRequestInvalid
	}
	digest := idempotencyScopeDigest(scope)

	for {
		if err := ctx.Err(); err != nil {
			return CommandResult{}, err
		}
		now := s.now().UTC()
		s.cleanupExpired(ctx, now)
		ownerToken, err := newIdempotencyOwnerToken(rand.Reader)
		if err != nil {
			return CommandResult{}, ErrExecutionFailed
		}
		leaseExpiresAt := s.leaseExpiresAt(ctx, now)
		claim := newGORMIdempotencyRecord(scope, digest, fingerprint, ownerToken, now, leaseExpiresAt, s.ttl)
		claimed, err := s.insertClaim(ctx, &claim)
		if err != nil {
			return CommandResult{}, err
		}
		if claimed {
			return s.executeClaim(ctx, claim, execute)
		}

		record, err := s.load(ctx, digest)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			continue
		}
		if err != nil {
			return CommandResult{}, idempotencyDatabaseError(ctx)
		}
		if !record.matchesScope(scope) {
			return CommandResult{}, ErrExecutionFailed
		}
		if !now.Before(record.ExpiresAt) {
			claimed, err = s.reclaimExpired(ctx, record.ScopeDigest, claim, now)
			if err != nil {
				return CommandResult{}, err
			}
			if claimed {
				return s.executeClaim(ctx, claim, execute)
			}
			continue
		}
		if record.Fingerprint != fingerprint {
			return CommandResult{}, ErrIdempotencyConflict
		}
		switch record.Status {
		case idempotencyStatusCompleted:
			return decodeIdempotencyResult(record.ResultJSON)
		case idempotencyStatusProcessing:
			if !now.Before(record.LeaseExpiresAt) {
				claimed, err = s.reclaimLease(ctx, record.ScopeDigest, fingerprint, claim, now)
				if err != nil {
					return CommandResult{}, err
				}
				if claimed {
					return s.executeClaim(ctx, claim, execute)
				}
				continue
			}
		default:
			return CommandResult{}, ErrExecutionFailed
		}
		if err := waitForIdempotencyRecord(ctx, s.pollInterval); err != nil {
			return CommandResult{}, err
		}
	}
}

func defaultGORMIdempotencyStoreOptions(options GORMIdempotencyStoreOptions) GORMIdempotencyStoreOptions {
	if options.TTL == 0 {
		options.TTL = defaultIdempotencyTTL
	}
	if options.LeaseDuration == 0 {
		options.LeaseDuration = defaultIdempotencyLease
	}
	if options.PollInterval == 0 {
		options.PollInterval = defaultIdempotencyPoll
	}
	if options.CleanupInterval == 0 {
		options.CleanupInterval = defaultIdempotencyCleanup
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	return options
}

func validIdempotencyScope(scope IdempotencyScope) bool {
	return strings.TrimSpace(scope.CommandID) != "" && len(scope.CommandID) <= 256 &&
		strings.TrimSpace(scope.Version) != "" && len(scope.Version) <= 64 &&
		strings.TrimSpace(scope.Actor) != "" && len(scope.Actor) <= 256 &&
		len(scope.TenantID) <= 256 && strings.TrimSpace(scope.Key) != "" && len(scope.Key) <= 128
}

func validIdempotencyFingerprint(fingerprint string) bool {
	return strings.TrimSpace(fingerprint) != "" && len(fingerprint) <= idempotencyFingerprintLength
}

func idempotencyScopeDigest(scope IdempotencyScope) string {
	hash := sha256.New()
	for _, value := range []string{scope.CommandID, scope.Version, scope.Actor, scope.TenantID, scope.Key} {
		_ = binary.Write(hash, binary.BigEndian, uint32(len(value)))
		_, _ = io.WriteString(hash, value)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func newGORMIdempotencyRecord(scope IdempotencyScope, digest string, fingerprint string, ownerToken string, now time.Time, leaseExpiresAt time.Time, ttl time.Duration) gormIdempotencyRecord {
	return gormIdempotencyRecord{
		ScopeDigest: digest, CommandID: scope.CommandID, Version: scope.Version, Actor: scope.Actor,
		TenantID: scope.TenantID, IdempotencyKey: scope.Key, Fingerprint: fingerprint,
		Status: idempotencyStatusProcessing, OwnerToken: ownerToken,
		LeaseExpiresAt: leaseExpiresAt, ExpiresAt: leaseExpiresAt.Add(ttl), CreatedAt: now, UpdatedAt: now,
	}
}

func (r gormIdempotencyRecord) matchesScope(scope IdempotencyScope) bool {
	return r.CommandID == scope.CommandID && r.Version == scope.Version && r.Actor == scope.Actor &&
		r.TenantID == scope.TenantID && r.IdempotencyKey == scope.Key
}

func (s *GORMIdempotencyStore) insertClaim(ctx context.Context, record *gormIdempotencyRecord) (bool, error) {
	result := s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(record)
	if result.Error != nil {
		return false, idempotencyDatabaseError(ctx)
	}
	return result.RowsAffected == 1, nil
}

func (s *GORMIdempotencyStore) load(ctx context.Context, digest string) (gormIdempotencyRecord, error) {
	var record gormIdempotencyRecord
	err := s.db.WithContext(ctx).Where("scope_digest = ?", digest).Take(&record).Error
	return record, err
}

func (s *GORMIdempotencyStore) reclaimExpired(ctx context.Context, digest string, claim gormIdempotencyRecord, now time.Time) (bool, error) {
	result := s.db.WithContext(ctx).Model(&gormIdempotencyRecord{}).
		Where("scope_digest = ? AND expires_at <= ?", digest, now).
		Updates(claim.claimUpdates())
	if result.Error != nil {
		return false, idempotencyDatabaseError(ctx)
	}
	return result.RowsAffected == 1, nil
}

func (s *GORMIdempotencyStore) reclaimLease(ctx context.Context, digest string, fingerprint string, claim gormIdempotencyRecord, now time.Time) (bool, error) {
	result := s.db.WithContext(ctx).Model(&gormIdempotencyRecord{}).
		Where("scope_digest = ? AND status = ? AND fingerprint = ? AND lease_expires_at <= ?", digest, idempotencyStatusProcessing, fingerprint, now).
		Updates(claim.claimUpdates())
	if result.Error != nil {
		return false, idempotencyDatabaseError(ctx)
	}
	return result.RowsAffected == 1, nil
}

func (r gormIdempotencyRecord) claimUpdates() map[string]any {
	return map[string]any{
		"fingerprint":      r.Fingerprint,
		"status":           r.Status,
		"owner_token":      r.OwnerToken,
		"result_json":      nil,
		"lease_expires_at": r.LeaseExpiresAt,
		"expires_at":       r.ExpiresAt,
		"created_at":       r.CreatedAt,
		"updated_at":       r.UpdatedAt,
	}
}

func (s *GORMIdempotencyStore) executeClaim(ctx context.Context, claim gormIdempotencyRecord, execute func(context.Context) (CommandResult, error)) (CommandResult, error) {
	result, err := execute(ctx)
	if err != nil {
		s.releaseClaim(claim, ctx)
		return CommandResult{}, err
	}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		s.releaseClaim(claim, ctx)
		return CommandResult{}, ErrExecutionFailed
	}
	now := s.now().UTC()
	finalizeContext, cancel := idempotencyFinalizeContext(ctx)
	defer cancel()
	updated := s.db.WithContext(finalizeContext).Model(&gormIdempotencyRecord{}).
		Where("scope_digest = ? AND status = ? AND owner_token = ?", claim.ScopeDigest, idempotencyStatusProcessing, claim.OwnerToken).
		Updates(map[string]any{
			"status": idempotencyStatusCompleted, "result_json": resultJSON,
			"lease_expires_at": now, "expires_at": now.Add(s.ttl), "updated_at": now,
		})
	if updated.Error != nil || updated.RowsAffected != 1 {
		return CommandResult{}, idempotencyDatabaseError(finalizeContext)
	}
	return cloneCommandResult(result), nil
}

func (s *GORMIdempotencyStore) releaseClaim(claim gormIdempotencyRecord, requestContext context.Context) {
	ctx, cancel := idempotencyFinalizeContext(requestContext)
	defer cancel()
	s.db.WithContext(ctx).
		Where("scope_digest = ? AND status = ? AND owner_token = ?", claim.ScopeDigest, idempotencyStatusProcessing, claim.OwnerToken).
		Delete(&gormIdempotencyRecord{})
}

func idempotencyFinalizeContext(requestContext context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(requestContext), defaultIdempotencyFinalize)
}

func (s *GORMIdempotencyStore) leaseExpiresAt(ctx context.Context, now time.Time) time.Time {
	leaseExpiresAt := now.Add(s.leaseDuration)
	if deadline, ok := ctx.Deadline(); ok && deadline.After(leaseExpiresAt) {
		return deadline.UTC().Add(s.pollInterval)
	}
	return leaseExpiresAt
}

func (s *GORMIdempotencyStore) cleanupExpired(ctx context.Context, now time.Time) {
	last := s.lastCleanup.Load()
	if last != 0 && now.Sub(time.Unix(0, last)) < s.cleanupInterval {
		return
	}
	if !s.lastCleanup.CompareAndSwap(last, now.UnixNano()) {
		return
	}
	s.db.WithContext(ctx).Where("expires_at <= ?", now).Delete(&gormIdempotencyRecord{})
}

func newIdempotencyOwnerToken(reader io.Reader) (string, error) {
	raw := make([]byte, idempotencyOwnerTokenBytes)
	if _, err := io.ReadFull(reader, raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func decodeIdempotencyResult(raw []byte) (CommandResult, error) {
	if len(raw) == 0 {
		return CommandResult{}, ErrExecutionFailed
	}
	var result CommandResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return CommandResult{}, ErrExecutionFailed
	}
	return cloneCommandResult(result), nil
}

func waitForIdempotencyRecord(ctx context.Context, interval time.Duration) error {
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func idempotencyDatabaseError(ctx context.Context) error {
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	return ErrExecutionFailed
}
