package sensitivereveal

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"time"
)

type CompleteFactorCommand struct {
	ChallengeDigest     string
	ExpectedChallengeID string
	TransactionDigest   string
	VerificationDigest  string
	Verified            bool
	Now                 time.Time
	CandidateGrant      GrantRecord
}

type CancelFactorCommand struct {
	ChallengeDigest     string
	ExpectedChallengeID string
	TransactionDigest   string
	Reason              string
	Now                 time.Time
}

func constantTimeDigestEqual(stored, candidate string) bool {
	storedHash := sha256.Sum256([]byte(stored))
	candidateHash := sha256.Sum256([]byte(candidate))
	return subtle.ConstantTimeCompare(storedHash[:], candidateHash[:]) == 1
}

type BeginFactorStoreResult struct {
	ChallengeID string
	ExpiresAt   time.Time
}

type CompleteFactorStoreResult struct {
	ChallengeID     string
	PolicySatisfied bool
	GrantIssued     bool
	Grant           GrantRecord
}

type ConsumeGrantCommand struct {
	GrantDigest string
	Scope       Scope
	Now         time.Time
}

type RecordRevealResultCommand struct {
	GrantID string
	Scope   Scope
	Success bool
	Reason  string
	Now     time.Time
}

// Store persists reveal state transitions. Implementations must make
// CompleteFactor and ConsumeGrant atomic with their audit writes.
type Store interface {
	CreateChallenge(context.Context, ChallengeRecord) error
	CreateFactorTransaction(context.Context, string, string, FactorTransactionRecord, time.Time) (BeginFactorStoreResult, error)
	CancelFactorTransaction(context.Context, CancelFactorCommand) error
	CompleteFactor(context.Context, CompleteFactorCommand) (CompleteFactorStoreResult, error)
	ConsumeGrant(context.Context, ConsumeGrantCommand) (GrantRecord, error)
	RecordRevealResult(context.Context, RecordRevealResultCommand) error
	ListAudit(context.Context) ([]AuditEvent, error)
}

func auditEvent(eventType AuditEventType, outcome, reason string, challenge ChallengeRecord, grant GrantRecord, factor Factor, now time.Time) AuditEvent {
	return AuditEvent{
		Type:        eventType,
		Outcome:     outcome,
		Reason:      reason,
		ChallengeID: challenge.ID,
		GrantID:     grant.ID,
		Factor:      factor,
		Scope:       challenge.Scope,
		CreatedAt:   now.UTC(),
	}
}

func deniedAudit(scope Scope, reason string, grant GrantRecord, now time.Time) AuditEvent {
	return AuditEvent{
		Type:      AuditRevealDenied,
		Outcome:   "denied",
		Reason:    reason,
		GrantID:   grant.ID,
		Scope:     scope.normalized(),
		CreatedAt: now.UTC(),
	}
}
