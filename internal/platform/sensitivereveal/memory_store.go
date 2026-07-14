package sensitivereveal

import (
	"context"
	"strings"
	"sync"
	"time"
)

type MemoryStore struct {
	mu           sync.Mutex
	challenges   map[string]ChallengeRecord
	transactions map[string]FactorTransactionRecord
	grants       map[string]GrantRecord
	audit        []AuditEvent
	nextAuditID  uint64
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		challenges:   make(map[string]ChallengeRecord),
		transactions: make(map[string]FactorTransactionRecord),
		grants:       make(map[string]GrantRecord),
	}
}

func (s *MemoryStore) CreateChallenge(_ context.Context, challenge ChallengeRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	challenge.Policy = challenge.Policy.normalized()
	if err := challenge.Policy.validate(); err != nil {
		return err
	}
	challenge.Scope = challenge.Scope.normalized()
	if err := challenge.Scope.validate(); err != nil {
		return err
	}
	if _, exists := s.challenges[challenge.TokenDigest]; exists {
		return ErrInvalidConfiguration
	}
	challenge.CreatedAt = challenge.CreatedAt.UTC()
	challenge.ExpiresAt = challenge.ExpiresAt.UTC()
	s.challenges[challenge.TokenDigest] = challenge
	s.appendAuditLocked(auditEvent(AuditChallengeCreated, "created", "", challenge, GrantRecord{}, "", challenge.CreatedAt))
	return nil
}

func (s *MemoryStore) CreateFactorTransaction(_ context.Context, challengeDigest string, expectedChallengeID string, transaction FactorTransactionRecord, now time.Time) (BeginFactorStoreResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now = now.UTC()
	challenge, ok := s.challenges[challengeDigest]
	if !ok {
		return BeginFactorStoreResult{}, ErrChallengeNotFound
	}
	if expectedChallengeID != "" && expectedChallengeID != challenge.ID {
		return BeginFactorStoreResult{}, ErrChallengeNotFound
	}
	if !now.Before(challenge.ExpiresAt) {
		s.appendAuditLocked(auditEvent(AuditFactorFailed, "denied", "challenge_expired", challenge, GrantRecord{}, transaction.Factor, now))
		return BeginFactorStoreResult{}, ErrChallengeExpired
	}
	if !challenge.GrantIssuedAt.IsZero() {
		return BeginFactorStoreResult{}, ErrChallengeClosed
	}
	rule, allowed := challenge.Policy.factorRule(transaction.Factor)
	if !allowed {
		s.appendAuditLocked(auditEvent(AuditFactorFailed, "denied", "factor_not_allowed", challenge, GrantRecord{}, transaction.Factor, now))
		return BeginFactorStoreResult{}, ErrFactorNotAllowed
	}
	for _, existing := range s.transactions {
		if existing.ChallengeID == challenge.ID && existing.Factor == transaction.Factor {
			return BeginFactorStoreResult{}, ErrFactorAlreadyStarted
		}
	}
	if _, exists := s.transactions[transaction.TransactionDigest]; exists {
		return BeginFactorStoreResult{}, ErrInvalidConfiguration
	}
	transaction.ChallengeID = challenge.ID
	transaction.MaxAttempts = rule.MaxAttempts
	transaction.CreatedAt = now
	s.transactions[transaction.TransactionDigest] = transaction
	s.appendAuditLocked(auditEvent(AuditFactorStarted, "started", "", challenge, GrantRecord{}, transaction.Factor, now))
	return BeginFactorStoreResult{ChallengeID: challenge.ID, ExpiresAt: challenge.ExpiresAt}, nil
}

func (s *MemoryStore) CancelFactorTransaction(_ context.Context, command CancelFactorCommand) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	challenge, ok := s.challenges[command.ChallengeDigest]
	if !ok || (command.ExpectedChallengeID != "" && command.ExpectedChallengeID != challenge.ID) {
		return ErrChallengeNotFound
	}
	transaction, ok := s.transactions[command.TransactionDigest]
	if !ok || transaction.ChallengeID != challenge.ID {
		return ErrFactorTransactionNotFound
	}
	if !transaction.CompletedAt.IsZero() {
		return ErrFactorAlreadyCompleted
	}
	if !transaction.LockedAt.IsZero() {
		return ErrFactorLocked
	}
	delete(s.transactions, command.TransactionDigest)
	s.appendAuditLocked(auditEvent(AuditFactorFailed, "cancelled", command.Reason, challenge, GrantRecord{}, transaction.Factor, command.Now.UTC()))
	return nil
}

func (s *MemoryStore) CompleteFactor(_ context.Context, command CompleteFactorCommand) (CompleteFactorStoreResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := command.Now.UTC()
	challenge, ok := s.challenges[command.ChallengeDigest]
	if !ok {
		return CompleteFactorStoreResult{}, ErrChallengeNotFound
	}
	if command.ExpectedChallengeID != "" && command.ExpectedChallengeID != challenge.ID {
		return CompleteFactorStoreResult{}, ErrChallengeNotFound
	}
	if !now.Before(challenge.ExpiresAt) {
		s.appendAuditLocked(auditEvent(AuditFactorFailed, "denied", "challenge_expired", challenge, GrantRecord{}, "", now))
		return CompleteFactorStoreResult{}, ErrChallengeExpired
	}
	if !challenge.GrantIssuedAt.IsZero() {
		return CompleteFactorStoreResult{}, ErrChallengeClosed
	}
	transaction, ok := s.transactions[command.TransactionDigest]
	if !ok || transaction.ChallengeID != challenge.ID {
		return CompleteFactorStoreResult{}, ErrFactorTransactionNotFound
	}
	if !transaction.CompletedAt.IsZero() {
		return CompleteFactorStoreResult{}, ErrFactorAlreadyCompleted
	}
	if !transaction.LockedAt.IsZero() {
		return CompleteFactorStoreResult{}, ErrFactorLocked
	}
	verified := command.Verified
	if transaction.VerificationDigest != "" {
		verified = constantTimeDigestEqual(transaction.VerificationDigest, command.VerificationDigest)
	}
	if !verified {
		transaction.AttemptCount++
		reason := "verification_failed"
		failure := ErrVerificationFailed
		if transaction.AttemptCount >= transaction.MaxAttempts {
			transaction.LockedAt = now
			reason = "attempt_limit_reached"
			failure = ErrFactorLocked
		}
		s.transactions[command.TransactionDigest] = transaction
		s.appendAuditLocked(auditEvent(AuditFactorFailed, "denied", reason, challenge, GrantRecord{}, transaction.Factor, now))
		return CompleteFactorStoreResult{}, failure
	}

	transaction.CompletedAt = now
	s.transactions[command.TransactionDigest] = transaction
	s.appendAuditLocked(auditEvent(AuditFactorCompleted, "verified", "", challenge, GrantRecord{}, transaction.Factor, now))
	completed := make(map[Factor]bool, len(challenge.Policy.Factors))
	for _, existing := range s.transactions {
		if existing.ChallengeID == challenge.ID && !existing.CompletedAt.IsZero() {
			completed[existing.Factor] = true
		}
	}
	result := CompleteFactorStoreResult{ChallengeID: challenge.ID, PolicySatisfied: challenge.Policy.satisfied(completed)}
	if !result.PolicySatisfied {
		return result, nil
	}
	grant := command.CandidateGrant
	grant.ChallengeID = challenge.ID
	grant.Scope = challenge.Scope
	grant.CreatedAt = now
	grant.ExpiresAt = now.Add(challenge.Policy.GrantTTL)
	if _, exists := s.grants[grant.TokenDigest]; exists {
		return CompleteFactorStoreResult{}, ErrInvalidConfiguration
	}
	challenge.GrantIssuedAt = now
	s.challenges[command.ChallengeDigest] = challenge
	s.grants[grant.TokenDigest] = grant
	s.appendAuditLocked(auditEvent(AuditGrantIssued, "issued", "", challenge, grant, transaction.Factor, now))
	result.GrantIssued = true
	result.Grant = grant
	return result, nil
}

func (s *MemoryStore) ConsumeGrant(_ context.Context, command ConsumeGrantCommand) (GrantRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := command.Now.UTC()
	grant, ok := s.grants[command.GrantDigest]
	if !ok {
		s.appendAuditLocked(deniedAudit(command.Scope, "grant_not_found", GrantRecord{}, now))
		return GrantRecord{}, ErrGrantNotFound
	}
	if !grant.Scope.equal(command.Scope) {
		s.appendAuditLocked(deniedAudit(command.Scope, "scope_mismatch", grant, now))
		return GrantRecord{}, ErrScopeMismatch
	}
	if !now.Before(grant.ExpiresAt) {
		s.appendAuditLocked(deniedAudit(command.Scope, "grant_expired", grant, now))
		return GrantRecord{}, ErrGrantExpired
	}
	if !grant.ConsumedAt.IsZero() {
		s.appendAuditLocked(deniedAudit(command.Scope, "grant_consumed", grant, now))
		return GrantRecord{}, ErrGrantConsumed
	}
	grant.ConsumedAt = now
	s.grants[command.GrantDigest] = grant
	s.appendAuditLocked(AuditEvent{
		Type:        AuditRevealAllowed,
		Outcome:     "allowed",
		ChallengeID: grant.ChallengeID,
		GrantID:     grant.ID,
		Scope:       grant.Scope,
		CreatedAt:   now,
	})
	return grant, nil
}

func (s *MemoryStore) RecordRevealResult(_ context.Context, command RecordRevealResultCommand) error {
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
	s.mu.Lock()
	defer s.mu.Unlock()
	var grant GrantRecord
	found := false
	for _, candidate := range s.grants {
		if candidate.ID == command.GrantID {
			grant = candidate
			found = true
			break
		}
	}
	if !found {
		return ErrGrantNotFound
	}
	if !grant.Scope.equal(command.Scope) {
		return ErrScopeMismatch
	}
	if grant.ConsumedAt.IsZero() {
		return ErrGrantNotConsumed
	}
	for _, event := range s.audit {
		if event.GrantID == grant.ID && (event.Type == AuditRevealSucceeded || event.Type == AuditRevealFailed) {
			return ErrRevealResultRecorded
		}
	}
	eventType := AuditRevealFailed
	outcome := "failed"
	if command.Success {
		eventType = AuditRevealSucceeded
		outcome = "succeeded"
	}
	s.appendAuditLocked(AuditEvent{
		Type:        eventType,
		Outcome:     outcome,
		Reason:      command.Reason,
		ChallengeID: grant.ChallengeID,
		GrantID:     grant.ID,
		Scope:       grant.Scope,
		CreatedAt:   command.Now.UTC(),
	})
	return nil
}

func (s *MemoryStore) ListAudit(_ context.Context) ([]AuditEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]AuditEvent(nil), s.audit...), nil
}

func (s *MemoryStore) appendAuditLocked(event AuditEvent) {
	s.nextAuditID++
	event.ID = s.nextAuditID
	event.CreatedAt = event.CreatedAt.UTC()
	s.audit = append(s.audit, event)
}
