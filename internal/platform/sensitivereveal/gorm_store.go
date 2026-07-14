package sensitivereveal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	challengesTable         = "platform_sensitive_reveal_challenges"
	factorTransactionsTable = "platform_sensitive_reveal_factor_transactions"
	grantsTable             = "platform_sensitive_reveal_grants"
	auditEventsTable        = "platform_sensitive_reveal_audit_events"
)

var errConcurrentMutation = errors.New("sensitive reveal concurrent mutation")

type GORMStore struct {
	db *gorm.DB
}

type gormChallenge struct {
	ID            string     `gorm:"primaryKey;size:64;column:id"`
	TokenDigest   string     `gorm:"size:128;not null;uniqueIndex;column:token_digest"`
	PolicyID      string     `gorm:"size:128;not null;index;column:policy_id"`
	PolicyJSON    string     `gorm:"type:text;not null;column:policy_json"`
	Actor         string     `gorm:"size:256;not null;index;column:actor"`
	SessionDigest string     `gorm:"size:256;not null;index;column:session_digest"`
	Tenant        string     `gorm:"size:256;not null;index;column:tenant"`
	Resource      string     `gorm:"size:256;not null;index;column:resource"`
	Record        string     `gorm:"size:256;not null;column:record"`
	Field         string     `gorm:"size:256;not null;column:field"`
	Purpose       string     `gorm:"size:128;not null;column:purpose"`
	Permission    string     `gorm:"size:256;not null;column:permission"`
	CreatedAt     time.Time  `gorm:"not null;column:created_at"`
	ExpiresAt     time.Time  `gorm:"not null;index;column:expires_at"`
	GrantIssuedAt *time.Time `gorm:"column:grant_issued_at"`
}

func (gormChallenge) TableName() string { return challengesTable }

type gormFactorTransaction struct {
	ID                 string     `gorm:"primaryKey;size:64;column:id"`
	ChallengeID        string     `gorm:"size:64;not null;uniqueIndex:idx_sensitive_reveal_challenge_factor,priority:1;index;column:challenge_id"`
	TransactionDigest  string     `gorm:"size:128;not null;uniqueIndex;column:transaction_digest"`
	VerificationDigest string     `gorm:"size:128;not null;default:'';column:verification_digest"`
	Factor             string     `gorm:"size:64;not null;uniqueIndex:idx_sensitive_reveal_challenge_factor,priority:2;column:factor"`
	AttemptCount       int        `gorm:"not null;column:attempt_count"`
	MaxAttempts        int        `gorm:"not null;column:max_attempts"`
	CreatedAt          time.Time  `gorm:"not null;column:created_at"`
	CompletedAt        *time.Time `gorm:"column:completed_at"`
	LockedAt           *time.Time `gorm:"column:locked_at"`
}

func (gormFactorTransaction) TableName() string { return factorTransactionsTable }

type gormGrant struct {
	ID            string     `gorm:"primaryKey;size:64;column:id"`
	ChallengeID   string     `gorm:"size:64;not null;uniqueIndex;column:challenge_id"`
	TokenDigest   string     `gorm:"size:128;not null;uniqueIndex;column:token_digest"`
	Actor         string     `gorm:"size:256;not null;index;column:actor"`
	SessionDigest string     `gorm:"size:256;not null;index;column:session_digest"`
	Tenant        string     `gorm:"size:256;not null;index;column:tenant"`
	Resource      string     `gorm:"size:256;not null;index;column:resource"`
	Record        string     `gorm:"size:256;not null;column:record"`
	Field         string     `gorm:"size:256;not null;column:field"`
	Purpose       string     `gorm:"size:128;not null;column:purpose"`
	Permission    string     `gorm:"size:256;not null;column:permission"`
	CreatedAt     time.Time  `gorm:"not null;column:created_at"`
	ExpiresAt     time.Time  `gorm:"not null;index;column:expires_at"`
	ConsumedAt    *time.Time `gorm:"column:consumed_at"`
}

func (gormGrant) TableName() string { return grantsTable }

type gormAuditEvent struct {
	ID            uint64    `gorm:"primaryKey;autoIncrement;column:id"`
	Type          string    `gorm:"size:64;not null;index;column:event_type"`
	Outcome       string    `gorm:"size:32;not null;index;column:outcome"`
	Reason        string    `gorm:"size:128;not null;column:reason"`
	ChallengeID   string    `gorm:"size:64;not null;index;column:challenge_id"`
	GrantID       string    `gorm:"size:64;not null;index;column:grant_id"`
	Factor        string    `gorm:"size:64;not null;column:factor"`
	Actor         string    `gorm:"size:256;not null;index;column:actor"`
	SessionDigest string    `gorm:"size:256;not null;column:session_digest"`
	Tenant        string    `gorm:"size:256;not null;index;column:tenant"`
	Resource      string    `gorm:"size:256;not null;index;column:resource"`
	Record        string    `gorm:"size:256;not null;column:record"`
	Field         string    `gorm:"size:256;not null;column:field"`
	Purpose       string    `gorm:"size:128;not null;column:purpose"`
	Permission    string    `gorm:"size:256;not null;column:permission"`
	CreatedAt     time.Time `gorm:"not null;index;column:created_at"`
}

func (gormAuditEvent) TableName() string { return auditEventsTable }

func NewGORMStore(ctx context.Context, db *gorm.DB) (*GORMStore, error) {
	if db == nil {
		return nil, ErrInvalidConfiguration
	}
	store := &GORMStore{db: db}
	if err := db.WithContext(ctx).AutoMigrate(
		&gormChallenge{},
		&gormFactorTransaction{},
		&gormGrant{},
		&gormAuditEvent{},
	); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *GORMStore) CreateChallenge(ctx context.Context, challenge ChallengeRecord) error {
	model, err := challengeToGORM(challenge)
	if err != nil {
		return err
	}
	event := auditEvent(AuditChallengeCreated, "created", "", challenge, GrantRecord{}, "", challenge.CreatedAt)
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&model).Error; err != nil {
			return err
		}
		return tx.Create(auditToGORM(event)).Error
	})
}

func (s *GORMStore) CreateFactorTransaction(ctx context.Context, challengeDigest string, expectedChallengeID string, transaction FactorTransactionRecord, now time.Time) (BeginFactorStoreResult, error) {
	var result BeginFactorStoreResult
	var domainErr error
	err := s.withRetry(func() error {
		domainErr = nil
		return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			challengeModel, challenge, err := lockedChallengeByDigest(tx, challengeDigest)
			if errors.Is(err, gorm.ErrRecordNotFound) {
				domainErr = ErrChallengeNotFound
				return nil
			}
			if err != nil {
				return err
			}
			if expectedChallengeID != "" && expectedChallengeID != challenge.ID {
				domainErr = ErrChallengeNotFound
				return nil
			}
			now = now.UTC()
			result = BeginFactorStoreResult{ChallengeID: challenge.ID, ExpiresAt: challenge.ExpiresAt}
			if !now.Before(challenge.ExpiresAt) {
				domainErr = ErrChallengeExpired
				return tx.Create(auditToGORM(auditEvent(AuditFactorFailed, "denied", "challenge_expired", challenge, GrantRecord{}, transaction.Factor, now))).Error
			}
			if challengeModel.GrantIssuedAt != nil {
				domainErr = ErrChallengeClosed
				return nil
			}
			rule, allowed := challenge.Policy.factorRule(transaction.Factor)
			if !allowed {
				domainErr = ErrFactorNotAllowed
				return tx.Create(auditToGORM(auditEvent(AuditFactorFailed, "denied", "factor_not_allowed", challenge, GrantRecord{}, transaction.Factor, now))).Error
			}
			var count int64
			if err := tx.Model(&gormFactorTransaction{}).
				Where("challenge_id = ? AND factor = ?", challenge.ID, transaction.Factor).
				Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				domainErr = ErrFactorAlreadyStarted
				return nil
			}
			transaction.ChallengeID = challenge.ID
			transaction.MaxAttempts = rule.MaxAttempts
			transaction.CreatedAt = now
			model := factorTransactionToGORM(transaction)
			if err := tx.Create(&model).Error; err != nil {
				if isDuplicateError(err) {
					return errConcurrentMutation
				}
				return err
			}
			return tx.Create(auditToGORM(auditEvent(AuditFactorStarted, "started", "", challenge, GrantRecord{}, transaction.Factor, now))).Error
		})
	})
	if err != nil {
		return BeginFactorStoreResult{}, err
	}
	return result, domainErr
}

func (s *GORMStore) CancelFactorTransaction(ctx context.Context, command CancelFactorCommand) error {
	var domainErr error
	err := s.withRetry(func() error {
		domainErr = nil
		return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			_, challenge, err := lockedChallengeByDigest(tx, command.ChallengeDigest)
			if errors.Is(err, gorm.ErrRecordNotFound) {
				domainErr = ErrChallengeNotFound
				return nil
			}
			if err != nil {
				return err
			}
			if command.ExpectedChallengeID != "" && command.ExpectedChallengeID != challenge.ID {
				domainErr = ErrChallengeNotFound
				return nil
			}
			var model gormFactorTransaction
			err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("transaction_digest = ? AND challenge_id = ?", command.TransactionDigest, challenge.ID).
				First(&model).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				domainErr = ErrFactorTransactionNotFound
				return nil
			}
			if err != nil {
				return err
			}
			transaction := factorTransactionFromGORM(model)
			if !transaction.CompletedAt.IsZero() {
				domainErr = ErrFactorAlreadyCompleted
				return nil
			}
			if !transaction.LockedAt.IsZero() {
				domainErr = ErrFactorLocked
				return nil
			}
			deleted := tx.Where("id = ? AND completed_at IS NULL AND locked_at IS NULL", transaction.ID).Delete(&gormFactorTransaction{})
			if deleted.Error != nil {
				return deleted.Error
			}
			if deleted.RowsAffected != 1 {
				return errConcurrentMutation
			}
			return tx.Create(auditToGORM(auditEvent(
				AuditFactorFailed,
				"cancelled",
				command.Reason,
				challenge,
				GrantRecord{},
				transaction.Factor,
				command.Now.UTC(),
			))).Error
		})
	})
	if err != nil {
		return err
	}
	return domainErr
}

func (s *GORMStore) CompleteFactor(ctx context.Context, command CompleteFactorCommand) (CompleteFactorStoreResult, error) {
	var result CompleteFactorStoreResult
	var domainErr error
	err := s.withRetry(func() error {
		result = CompleteFactorStoreResult{}
		domainErr = nil
		return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			challengeModel, challenge, err := lockedChallengeByDigest(tx, command.ChallengeDigest)
			if errors.Is(err, gorm.ErrRecordNotFound) {
				domainErr = ErrChallengeNotFound
				return nil
			}
			if err != nil {
				return err
			}
			if command.ExpectedChallengeID != "" && command.ExpectedChallengeID != challenge.ID {
				domainErr = ErrChallengeNotFound
				return nil
			}
			result.ChallengeID = challenge.ID
			now := command.Now.UTC()
			if !now.Before(challenge.ExpiresAt) {
				domainErr = ErrChallengeExpired
				return tx.Create(auditToGORM(auditEvent(AuditFactorFailed, "denied", "challenge_expired", challenge, GrantRecord{}, "", now))).Error
			}
			if challengeModel.GrantIssuedAt != nil {
				domainErr = ErrChallengeClosed
				return nil
			}
			var transactionModel gormFactorTransaction
			err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("transaction_digest = ? AND challenge_id = ?", command.TransactionDigest, challenge.ID).
				First(&transactionModel).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				domainErr = ErrFactorTransactionNotFound
				return nil
			}
			if err != nil {
				return err
			}
			transaction := factorTransactionFromGORM(transactionModel)
			if !transaction.CompletedAt.IsZero() {
				domainErr = ErrFactorAlreadyCompleted
				return nil
			}
			if !transaction.LockedAt.IsZero() {
				domainErr = ErrFactorLocked
				return nil
			}
			verified := command.Verified
			if transaction.VerificationDigest != "" {
				verified = constantTimeDigestEqual(transaction.VerificationDigest, command.VerificationDigest)
			}
			if !verified {
				attemptCount := transaction.AttemptCount + 1
				updates := map[string]interface{}{"attempt_count": attemptCount}
				reason := "verification_failed"
				domainErr = ErrVerificationFailed
				if attemptCount >= transaction.MaxAttempts {
					updates["locked_at"] = now
					reason = "attempt_limit_reached"
					domainErr = ErrFactorLocked
				}
				updated := tx.Model(&gormFactorTransaction{}).
					Where("id = ? AND attempt_count = ? AND completed_at IS NULL AND locked_at IS NULL", transaction.ID, transaction.AttemptCount).
					Updates(updates)
				if updated.Error != nil {
					return updated.Error
				}
				if updated.RowsAffected != 1 {
					return errConcurrentMutation
				}
				return tx.Create(auditToGORM(auditEvent(AuditFactorFailed, "denied", reason, challenge, GrantRecord{}, transaction.Factor, now))).Error
			}

			updated := tx.Model(&gormFactorTransaction{}).
				Where("id = ? AND completed_at IS NULL AND locked_at IS NULL", transaction.ID).
				Update("completed_at", now)
			if updated.Error != nil {
				return updated.Error
			}
			if updated.RowsAffected != 1 {
				return errConcurrentMutation
			}
			if err := tx.Create(auditToGORM(auditEvent(AuditFactorCompleted, "verified", "", challenge, GrantRecord{}, transaction.Factor, now))).Error; err != nil {
				return err
			}
			var completedFactors []string
			if err := tx.Model(&gormFactorTransaction{}).
				Where("challenge_id = ? AND completed_at IS NOT NULL", challenge.ID).
				Distinct().Pluck("factor", &completedFactors).Error; err != nil {
				return err
			}
			completed := make(map[Factor]bool, len(completedFactors))
			for _, factor := range completedFactors {
				completed[Factor(factor)] = true
			}
			result.PolicySatisfied = challenge.Policy.satisfied(completed)
			if !result.PolicySatisfied {
				return nil
			}
			issued := tx.Model(&gormChallenge{}).
				Where("id = ? AND grant_issued_at IS NULL", challenge.ID).
				Update("grant_issued_at", now)
			if issued.Error != nil {
				return issued.Error
			}
			if issued.RowsAffected != 1 {
				return errConcurrentMutation
			}
			grant := command.CandidateGrant
			grant.ChallengeID = challenge.ID
			grant.Scope = challenge.Scope
			grant.CreatedAt = now
			grant.ExpiresAt = now.Add(challenge.Policy.GrantTTL)
			grantModel := grantToGORM(grant)
			if err := tx.Create(&grantModel).Error; err != nil {
				return err
			}
			if err := tx.Create(auditToGORM(auditEvent(AuditGrantIssued, "issued", "", challenge, grant, transaction.Factor, now))).Error; err != nil {
				return err
			}
			result.GrantIssued = true
			result.Grant = grant
			return nil
		})
	})
	if err != nil {
		return CompleteFactorStoreResult{}, err
	}
	return result, domainErr
}

func (s *GORMStore) ConsumeGrant(ctx context.Context, command ConsumeGrantCommand) (GrantRecord, error) {
	var consumed GrantRecord
	var domainErr error
	err := s.withRetry(func() error {
		consumed = GrantRecord{}
		domainErr = nil
		return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			now := command.Now.UTC()
			var model gormGrant
			err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("token_digest = ?", command.GrantDigest).First(&model).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				domainErr = ErrGrantNotFound
				return tx.Create(auditToGORM(deniedAudit(command.Scope, "grant_not_found", GrantRecord{}, now))).Error
			}
			if err != nil {
				return err
			}
			grant := grantFromGORM(model)
			if !grant.Scope.equal(command.Scope) {
				domainErr = ErrScopeMismatch
				return tx.Create(auditToGORM(deniedAudit(command.Scope, "scope_mismatch", grant, now))).Error
			}
			if !now.Before(grant.ExpiresAt) {
				domainErr = ErrGrantExpired
				return tx.Create(auditToGORM(deniedAudit(command.Scope, "grant_expired", grant, now))).Error
			}
			if !grant.ConsumedAt.IsZero() {
				domainErr = ErrGrantConsumed
				return tx.Create(auditToGORM(deniedAudit(command.Scope, "grant_consumed", grant, now))).Error
			}
			updated := tx.Model(&gormGrant{}).
				Where(`token_digest = ? AND actor = ? AND session_digest = ? AND tenant = ? AND resource = ? AND record = ? AND field = ? AND purpose = ? AND permission = ? AND expires_at > ? AND consumed_at IS NULL`,
					command.GrantDigest, command.Scope.Actor, command.Scope.SessionDigest, command.Scope.Tenant, command.Scope.Resource,
					command.Scope.Record, command.Scope.Field, command.Scope.Purpose, command.Scope.Permission, now).
				Update("consumed_at", now)
			if updated.Error != nil {
				return updated.Error
			}
			if updated.RowsAffected != 1 {
				return errConcurrentMutation
			}
			grant.ConsumedAt = now
			event := AuditEvent{
				Type:        AuditRevealAllowed,
				Outcome:     "allowed",
				ChallengeID: grant.ChallengeID,
				GrantID:     grant.ID,
				Scope:       grant.Scope,
				CreatedAt:   now,
			}
			if err := tx.Create(auditToGORM(event)).Error; err != nil {
				return err
			}
			consumed = grant
			return nil
		})
	})
	if err != nil {
		return GrantRecord{}, err
	}
	return consumed, domainErr
}

func (s *GORMStore) RecordRevealResult(ctx context.Context, command RecordRevealResultCommand) error {
	command.GrantID = strings.TrimSpace(command.GrantID)
	command.Scope = command.Scope.normalized()
	command.Reason = strings.TrimSpace(command.Reason)
	if command.GrantID == "" {
		return ErrGrantNotFound
	}
	if err := command.Scope.validate(); err != nil {
		return err
	}
	if !validRevealReason(command.Success, command.Reason) {
		return ErrInvalidRevealReason
	}
	var domainErr error
	err := s.withRetry(func() error {
		domainErr = nil
		return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var model gormGrant
			err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", command.GrantID).First(&model).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				domainErr = ErrGrantNotFound
				return nil
			}
			if err != nil {
				return err
			}
			grant := grantFromGORM(model)
			if !grant.Scope.equal(command.Scope) {
				domainErr = ErrScopeMismatch
				return nil
			}
			if grant.ConsumedAt.IsZero() {
				domainErr = ErrGrantNotConsumed
				return nil
			}
			var count int64
			if err := tx.Model(&gormAuditEvent{}).
				Where("grant_id = ? AND event_type IN ?", grant.ID, []string{string(AuditRevealSucceeded), string(AuditRevealFailed)}).
				Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				domainErr = ErrRevealResultRecorded
				return nil
			}
			eventType := AuditRevealFailed
			outcome := "failed"
			if command.Success {
				eventType = AuditRevealSucceeded
				outcome = "succeeded"
			}
			return tx.Create(auditToGORM(AuditEvent{
				Type:        eventType,
				Outcome:     outcome,
				Reason:      command.Reason,
				ChallengeID: grant.ChallengeID,
				GrantID:     grant.ID,
				Scope:       grant.Scope,
				CreatedAt:   command.Now.UTC(),
			})).Error
		})
	})
	if err != nil {
		return err
	}
	return domainErr
}

func (s *GORMStore) ListAudit(ctx context.Context) ([]AuditEvent, error) {
	var records []gormAuditEvent
	if err := s.db.WithContext(ctx).Order("id ASC").Find(&records).Error; err != nil {
		return nil, err
	}
	events := make([]AuditEvent, 0, len(records))
	for _, record := range records {
		events = append(events, auditFromGORM(record))
	}
	return events, nil
}

func (s *GORMStore) withRetry(operation func() error) error {
	var err error
	for attempt := 0; attempt < 8; attempt++ {
		err = operation()
		if err == nil || !isRetryableError(err) {
			return err
		}
		time.Sleep(time.Duration(attempt+1) * time.Millisecond)
	}
	return err
}

func lockedChallengeByDigest(tx *gorm.DB, digest string) (gormChallenge, ChallengeRecord, error) {
	var model gormChallenge
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("token_digest = ?", digest).First(&model).Error
	if err != nil {
		return gormChallenge{}, ChallengeRecord{}, err
	}
	challenge, err := challengeFromGORM(model)
	return model, challenge, err
}

func challengeToGORM(challenge ChallengeRecord) (gormChallenge, error) {
	policy := challenge.Policy.normalized()
	if err := policy.validate(); err != nil {
		return gormChallenge{}, err
	}
	scope := challenge.Scope.normalized()
	if err := scope.validate(); err != nil {
		return gormChallenge{}, err
	}
	policyJSON, err := json.Marshal(policy)
	if err != nil {
		return gormChallenge{}, err
	}
	model := gormChallenge{
		ID:          challenge.ID,
		TokenDigest: challenge.TokenDigest,
		PolicyID:    policy.ID,
		PolicyJSON:  string(policyJSON),
		CreatedAt:   challenge.CreatedAt.UTC(),
		ExpiresAt:   challenge.ExpiresAt.UTC(),
	}
	setChallengeScope(&model, scope)
	if !challenge.GrantIssuedAt.IsZero() {
		value := challenge.GrantIssuedAt.UTC()
		model.GrantIssuedAt = &value
	}
	return model, nil
}

func challengeFromGORM(model gormChallenge) (ChallengeRecord, error) {
	var policy Policy
	if err := json.Unmarshal([]byte(model.PolicyJSON), &policy); err != nil {
		return ChallengeRecord{}, fmt.Errorf("decode sensitive reveal policy: %w", err)
	}
	policy = policy.normalized()
	if err := policy.validate(); err != nil {
		return ChallengeRecord{}, fmt.Errorf("validate sensitive reveal policy: %w", err)
	}
	scope := scopeFromChallenge(model).normalized()
	if err := scope.validate(); err != nil {
		return ChallengeRecord{}, fmt.Errorf("validate sensitive reveal scope: %w", err)
	}
	challenge := ChallengeRecord{
		ID:          model.ID,
		TokenDigest: model.TokenDigest,
		Policy:      policy,
		Scope:       scope,
		CreatedAt:   model.CreatedAt.UTC(),
		ExpiresAt:   model.ExpiresAt.UTC(),
	}
	if model.GrantIssuedAt != nil {
		challenge.GrantIssuedAt = model.GrantIssuedAt.UTC()
	}
	return challenge, nil
}

func factorTransactionToGORM(record FactorTransactionRecord) gormFactorTransaction {
	model := gormFactorTransaction{
		ID:                 record.ID,
		ChallengeID:        record.ChallengeID,
		TransactionDigest:  record.TransactionDigest,
		VerificationDigest: record.VerificationDigest,
		Factor:             string(record.Factor),
		AttemptCount:       record.AttemptCount,
		MaxAttempts:        record.MaxAttempts,
		CreatedAt:          record.CreatedAt.UTC(),
	}
	if !record.CompletedAt.IsZero() {
		value := record.CompletedAt.UTC()
		model.CompletedAt = &value
	}
	if !record.LockedAt.IsZero() {
		value := record.LockedAt.UTC()
		model.LockedAt = &value
	}
	return model
}

func factorTransactionFromGORM(model gormFactorTransaction) FactorTransactionRecord {
	record := FactorTransactionRecord{
		ID:                 model.ID,
		ChallengeID:        model.ChallengeID,
		TransactionDigest:  model.TransactionDigest,
		VerificationDigest: model.VerificationDigest,
		Factor:             Factor(model.Factor),
		AttemptCount:       model.AttemptCount,
		MaxAttempts:        model.MaxAttempts,
		CreatedAt:          model.CreatedAt.UTC(),
	}
	if model.CompletedAt != nil {
		record.CompletedAt = model.CompletedAt.UTC()
	}
	if model.LockedAt != nil {
		record.LockedAt = model.LockedAt.UTC()
	}
	return record
}

func grantToGORM(grant GrantRecord) gormGrant {
	model := gormGrant{
		ID:          grant.ID,
		ChallengeID: grant.ChallengeID,
		TokenDigest: grant.TokenDigest,
		CreatedAt:   grant.CreatedAt.UTC(),
		ExpiresAt:   grant.ExpiresAt.UTC(),
	}
	setGrantScope(&model, grant.Scope.normalized())
	if !grant.ConsumedAt.IsZero() {
		value := grant.ConsumedAt.UTC()
		model.ConsumedAt = &value
	}
	return model
}

func grantFromGORM(model gormGrant) GrantRecord {
	grant := GrantRecord{
		ID:          model.ID,
		ChallengeID: model.ChallengeID,
		TokenDigest: model.TokenDigest,
		Scope:       scopeFromGrant(model),
		CreatedAt:   model.CreatedAt.UTC(),
		ExpiresAt:   model.ExpiresAt.UTC(),
	}
	if model.ConsumedAt != nil {
		grant.ConsumedAt = model.ConsumedAt.UTC()
	}
	return grant
}

func auditToGORM(event AuditEvent) *gormAuditEvent {
	return &gormAuditEvent{
		ID:            event.ID,
		Type:          string(event.Type),
		Outcome:       event.Outcome,
		Reason:        event.Reason,
		ChallengeID:   event.ChallengeID,
		GrantID:       event.GrantID,
		Factor:        string(event.Factor),
		Actor:         event.Scope.Actor,
		SessionDigest: event.Scope.SessionDigest,
		Tenant:        event.Scope.Tenant,
		Resource:      event.Scope.Resource,
		Record:        event.Scope.Record,
		Field:         event.Scope.Field,
		Purpose:       event.Scope.Purpose,
		Permission:    event.Scope.Permission,
		CreatedAt:     event.CreatedAt.UTC(),
	}
}

func auditFromGORM(model gormAuditEvent) AuditEvent {
	return AuditEvent{
		ID:          model.ID,
		Type:        AuditEventType(model.Type),
		Outcome:     model.Outcome,
		Reason:      model.Reason,
		ChallengeID: model.ChallengeID,
		GrantID:     model.GrantID,
		Factor:      Factor(model.Factor),
		Scope: Scope{
			Actor:         model.Actor,
			SessionDigest: model.SessionDigest,
			Tenant:        model.Tenant,
			Resource:      model.Resource,
			Record:        model.Record,
			Field:         model.Field,
			Purpose:       model.Purpose,
			Permission:    model.Permission,
		},
		CreatedAt: model.CreatedAt.UTC(),
	}
}

func setChallengeScope(model *gormChallenge, scope Scope) {
	model.Actor = scope.Actor
	model.SessionDigest = scope.SessionDigest
	model.Tenant = scope.Tenant
	model.Resource = scope.Resource
	model.Record = scope.Record
	model.Field = scope.Field
	model.Purpose = scope.Purpose
	model.Permission = scope.Permission
}

func scopeFromChallenge(model gormChallenge) Scope {
	return Scope{Actor: model.Actor, SessionDigest: model.SessionDigest, Tenant: model.Tenant, Resource: model.Resource, Record: model.Record, Field: model.Field, Purpose: model.Purpose, Permission: model.Permission}
}

func setGrantScope(model *gormGrant, scope Scope) {
	model.Actor = scope.Actor
	model.SessionDigest = scope.SessionDigest
	model.Tenant = scope.Tenant
	model.Resource = scope.Resource
	model.Record = scope.Record
	model.Field = scope.Field
	model.Purpose = scope.Purpose
	model.Permission = scope.Permission
}

func scopeFromGrant(model gormGrant) Scope {
	return Scope{Actor: model.Actor, SessionDigest: model.SessionDigest, Tenant: model.Tenant, Resource: model.Resource, Record: model.Record, Field: model.Field, Purpose: model.Purpose, Permission: model.Permission}
}

func isRetryableError(err error) bool {
	if errors.Is(err, errConcurrentMutation) {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "database is locked") ||
		strings.Contains(message, "database table is locked") ||
		strings.Contains(message, "deadlock") ||
		strings.Contains(message, "serialization failure") ||
		strings.Contains(message, "could not serialize")
}

func isDuplicateError(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique constraint") || strings.Contains(message, "duplicate entry") || strings.Contains(message, "duplicate key")
}
