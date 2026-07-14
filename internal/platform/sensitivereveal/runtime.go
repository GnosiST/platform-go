package sensitivereveal

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"
)

const digestPrefix = "hmac-sha256:v1:"

type RuntimeOptions struct {
	Store    Store
	Policies []Policy
	HashKey  []byte
	Now      func() time.Time
	Random   io.Reader
}

type Runtime struct {
	store    Store
	policies map[string]Policy
	hashKey  []byte
	now      func() time.Time
	random   io.Reader
}

// NewRuntime creates a reveal runtime. HashKey must contain at least 32 bytes
// and must remain server-side; it protects every stored token digest.
func NewRuntime(options RuntimeOptions) (*Runtime, error) {
	if options.Store == nil || len(options.HashKey) < 32 {
		return nil, ErrInvalidConfiguration
	}
	policies := make(map[string]Policy, len(options.Policies))
	for _, input := range options.Policies {
		policy := input.normalized()
		if err := policy.validate(); err != nil {
			return nil, err
		}
		if _, exists := policies[policy.ID]; exists {
			return nil, fmt.Errorf("%w: duplicate policy %q", ErrInvalidPolicy, policy.ID)
		}
		policies[policy.ID] = policy
	}
	if len(policies) == 0 {
		return nil, ErrInvalidConfiguration
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	random := options.Random
	if random == nil {
		random = rand.Reader
	}
	return &Runtime{
		store:    options.Store,
		policies: policies,
		hashKey:  append([]byte(nil), options.HashKey...),
		now:      now,
		random:   random,
	}, nil
}

func (r *Runtime) BeginChallenge(ctx context.Context, request BeginChallengeRequest) (BeginChallengeResult, error) {
	policy, ok := r.policies[strings.TrimSpace(request.PolicyID)]
	if !ok {
		return BeginChallengeResult{}, ErrPolicyNotFound
	}
	scope := request.Scope.normalized()
	if err := scope.validate(); err != nil {
		return BeginChallengeResult{}, err
	}
	if !policy.allowsPurpose(scope.Purpose) {
		return BeginChallengeResult{}, ErrPurposeNotAllowed
	}
	challengeToken, err := r.newToken()
	if err != nil {
		return BeginChallengeResult{}, err
	}
	challengeID, err := r.newID()
	if err != nil {
		return BeginChallengeResult{}, err
	}
	now := r.now().UTC()
	challenge := ChallengeRecord{
		ID:          challengeID,
		TokenDigest: r.digest("challenge", challengeToken),
		Policy:      policy,
		Scope:       scope,
		CreatedAt:   now,
		ExpiresAt:   now.Add(policy.ChallengeTTL),
	}
	if err := r.store.CreateChallenge(ctx, challenge); err != nil {
		return BeginChallengeResult{}, err
	}
	return BeginChallengeResult{
		ChallengeID:    challenge.ID,
		ChallengeToken: challengeToken,
		PolicyID:       policy.ID,
		Mode:           policy.Mode,
		Factors:        append([]FactorRule(nil), policy.Factors...),
		ExpiresAt:      challenge.ExpiresAt,
	}, nil
}

func (r *Runtime) BeginFactor(ctx context.Context, request BeginFactorRequest) (BeginFactorResult, error) {
	challengeToken := strings.TrimSpace(request.ChallengeToken)
	factor := Factor(strings.TrimSpace(string(request.Factor)))
	if challengeToken == "" || factor == "" {
		return BeginFactorResult{}, ErrFactorNotAllowed
	}
	transactionToken, err := r.newToken()
	if err != nil {
		return BeginFactorResult{}, err
	}
	transactionID, err := r.newID()
	if err != nil {
		return BeginFactorResult{}, err
	}
	now := r.now().UTC()
	record := FactorTransactionRecord{
		ID:                transactionID,
		TransactionDigest: r.digest("factor-transaction", transactionToken),
		Factor:            factor,
		CreatedAt:         now,
	}
	if request.VerificationSecret != "" {
		record.VerificationDigest = r.digest("factor-verification\x00"+transactionToken, request.VerificationSecret)
	}
	challengeDigest := r.digest("challenge", challengeToken)
	storeResult, err := r.store.CreateFactorTransaction(ctx, challengeDigest, strings.TrimSpace(request.ExpectedChallengeID), record, now)
	if err != nil {
		return BeginFactorResult{}, err
	}
	return BeginFactorResult{
		ChallengeID: storeResult.ChallengeID, Factor: factor, TransactionToken: transactionToken, ExpiresAt: storeResult.ExpiresAt,
	}, nil
}

// CancelFactor removes an uncompleted factor transaction when an external
// dependency fails before verification reaches the user.
func (r *Runtime) CancelFactor(ctx context.Context, request CancelFactorRequest) error {
	challengeToken := strings.TrimSpace(request.ChallengeToken)
	transactionToken := strings.TrimSpace(request.TransactionToken)
	reason := strings.TrimSpace(request.Reason)
	if challengeToken == "" || transactionToken == "" {
		return ErrFactorTransactionNotFound
	}
	if reason != FactorCancelReasonDeliveryFailed {
		return ErrInvalidConfiguration
	}
	return r.store.CancelFactorTransaction(ctx, CancelFactorCommand{
		ChallengeDigest:     r.digest("challenge", challengeToken),
		ExpectedChallengeID: strings.TrimSpace(request.ExpectedChallengeID),
		TransactionDigest:   r.digest("factor-transaction", transactionToken),
		Reason:              reason,
		Now:                 r.now().UTC(),
	})
}

func (r *Runtime) CompleteFactor(ctx context.Context, request CompleteFactorRequest) (CompleteFactorResult, error) {
	challengeToken := strings.TrimSpace(request.ChallengeToken)
	transactionToken := strings.TrimSpace(request.TransactionToken)
	if challengeToken == "" || transactionToken == "" {
		return CompleteFactorResult{}, ErrFactorTransactionNotFound
	}
	grantToken, err := r.newToken()
	if err != nil {
		return CompleteFactorResult{}, err
	}
	grantID, err := r.newID()
	if err != nil {
		return CompleteFactorResult{}, err
	}
	now := r.now().UTC()
	result, err := r.store.CompleteFactor(ctx, CompleteFactorCommand{
		ChallengeDigest:     r.digest("challenge", challengeToken),
		ExpectedChallengeID: strings.TrimSpace(request.ExpectedChallengeID),
		TransactionDigest:   r.digest("factor-transaction", transactionToken),
		VerificationDigest:  r.digest("factor-verification\x00"+transactionToken, request.VerificationProof),
		Verified:            request.Verified,
		Now:                 now,
		CandidateGrant: GrantRecord{
			ID:          grantID,
			TokenDigest: r.digest("grant", grantToken),
			CreatedAt:   now,
		},
	})
	if err != nil {
		return CompleteFactorResult{}, err
	}
	response := CompleteFactorResult{ChallengeID: result.ChallengeID, PolicySatisfied: result.PolicySatisfied}
	if result.GrantIssued {
		response.GrantToken = grantToken
		response.GrantExpiresAt = result.Grant.ExpiresAt
	}
	return response, nil
}

func (r *Runtime) ConsumeGrant(ctx context.Context, request ConsumeGrantRequest) (ConsumeGrantResult, error) {
	grantToken := strings.TrimSpace(request.GrantToken)
	scope := request.Scope.normalized()
	if grantToken == "" {
		return ConsumeGrantResult{}, ErrGrantNotFound
	}
	if err := scope.validate(); err != nil {
		return ConsumeGrantResult{}, err
	}
	now := r.now().UTC()
	grant, err := r.store.ConsumeGrant(ctx, ConsumeGrantCommand{
		GrantDigest: r.digest("grant", grantToken),
		Scope:       scope,
		Now:         now,
	})
	if err != nil {
		return ConsumeGrantResult{}, err
	}
	return ConsumeGrantResult{GrantID: grant.ID, ConsumedAt: grant.ConsumedAt, ExpiresAt: grant.ExpiresAt}, nil
}

// RecordRevealResult appends the terminal audit outcome after the protected
// value has been projected. Reason must be one of the exported RevealReason
// constants; arbitrary error text and plaintext values are rejected.
func (r *Runtime) RecordRevealResult(ctx context.Context, grantID string, scope Scope, success bool, reason string) error {
	grantID = strings.TrimSpace(grantID)
	scope = scope.normalized()
	reason = strings.TrimSpace(reason)
	if grantID == "" {
		return ErrGrantNotFound
	}
	if err := scope.validate(); err != nil {
		return err
	}
	if !validRevealReason(success, reason) {
		return ErrInvalidRevealReason
	}
	return r.store.RecordRevealResult(ctx, RecordRevealResultCommand{
		GrantID: grantID,
		Scope:   scope,
		Success: success,
		Reason:  reason,
		Now:     r.now().UTC(),
	})
}

func (r *Runtime) AuditEvents(ctx context.Context) ([]AuditEvent, error) {
	return r.store.ListAudit(ctx)
}

func validRevealReason(success bool, reason string) bool {
	if success {
		return reason == RevealReasonCompleted
	}
	switch reason {
	case RevealReasonProtectedValueUnavailable, RevealReasonDecryptionFailed, RevealReasonProjectionFailed, RevealReasonResponseAborted:
		return true
	default:
		return false
	}
}

func (r *Runtime) newToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := io.ReadFull(r.random, raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func (r *Runtime) newID() (string, error) {
	raw := make([]byte, 16)
	if _, err := io.ReadFull(r.random, raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func (r *Runtime) digest(domain, raw string) string {
	mac := hmac.New(sha256.New, r.hashKey)
	_, _ = mac.Write([]byte("platform-sensitive-reveal\x00" + domain + "\x00" + raw))
	return digestPrefix + hex.EncodeToString(mac.Sum(nil))
}
