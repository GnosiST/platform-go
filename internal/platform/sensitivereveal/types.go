package sensitivereveal

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	DefaultChallengeTTL = 5 * time.Minute
	DefaultGrantTTL     = 60 * time.Second
)

var (
	ErrInvalidConfiguration      = errors.New("invalid sensitive reveal configuration")
	ErrInvalidPolicy             = errors.New("invalid sensitive reveal policy")
	ErrPolicyNotFound            = errors.New("sensitive reveal policy not found")
	ErrPurposeNotAllowed         = errors.New("sensitive reveal purpose is not allowed")
	ErrInvalidScope              = errors.New("invalid sensitive reveal scope")
	ErrChallengeNotFound         = errors.New("sensitive reveal challenge not found")
	ErrChallengeExpired          = errors.New("sensitive reveal challenge expired")
	ErrChallengeClosed           = errors.New("sensitive reveal challenge is closed")
	ErrFactorNotAllowed          = errors.New("sensitive reveal factor is not allowed")
	ErrFactorAlreadyStarted      = errors.New("sensitive reveal factor already started")
	ErrFactorTransactionNotFound = errors.New("sensitive reveal factor transaction not found")
	ErrFactorAlreadyCompleted    = errors.New("sensitive reveal factor already completed")
	ErrFactorLocked              = errors.New("sensitive reveal factor is locked")
	ErrVerificationFailed        = errors.New("sensitive reveal factor verification failed")
	ErrGrantNotFound             = errors.New("sensitive reveal grant not found")
	ErrGrantExpired              = errors.New("sensitive reveal grant expired")
	ErrGrantConsumed             = errors.New("sensitive reveal grant already consumed")
	ErrGrantNotConsumed          = errors.New("sensitive reveal grant has not been consumed")
	ErrScopeMismatch             = errors.New("sensitive reveal grant scope mismatch")
	ErrInvalidRevealReason       = errors.New("invalid sensitive reveal result reason")
	ErrRevealResultRecorded      = errors.New("sensitive reveal result already recorded")
)

type PolicyMode string

const (
	PolicyAnyOf PolicyMode = "anyOf"
	PolicyAllOf PolicyMode = "allOf"
)

type Factor string

const (
	FactorOIDCReauthentication Factor = "oidc-reauthentication"
	FactorSMSOTP               Factor = "sms-otp"
)

type FactorRule struct {
	Factor      Factor `json:"factor"`
	MaxAttempts int    `json:"maxAttempts"`
}

type Policy struct {
	ID           string        `json:"id"`
	Mode         PolicyMode    `json:"mode"`
	Factors      []FactorRule  `json:"factors"`
	PurposeCodes []string      `json:"purposeCodes"`
	ChallengeTTL time.Duration `json:"challengeTtl"`
	GrantTTL     time.Duration `json:"grantTtl"`
}

type Scope struct {
	Actor         string `json:"actor"`
	SessionDigest string `json:"sessionDigest"`
	Tenant        string `json:"tenant"`
	Resource      string `json:"resource"`
	Record        string `json:"record"`
	Field         string `json:"field"`
	Purpose       string `json:"purpose"`
	Permission    string `json:"permission"`
}

func (s Scope) normalized() Scope {
	return Scope{
		Actor:         strings.TrimSpace(s.Actor),
		SessionDigest: strings.TrimSpace(s.SessionDigest),
		Tenant:        strings.TrimSpace(s.Tenant),
		Resource:      strings.TrimSpace(s.Resource),
		Record:        strings.TrimSpace(s.Record),
		Field:         strings.TrimSpace(s.Field),
		Purpose:       strings.TrimSpace(s.Purpose),
		Permission:    strings.TrimSpace(s.Permission),
	}
}

func (s Scope) validate() error {
	s = s.normalized()
	if s.Actor == "" || s.SessionDigest == "" || s.Tenant == "" || s.Resource == "" ||
		s.Record == "" || s.Field == "" || s.Purpose == "" || s.Permission == "" {
		return ErrInvalidScope
	}
	return nil
}

func (s Scope) equal(other Scope) bool {
	return s.normalized() == other.normalized()
}

func (p Policy) normalized() Policy {
	p.ID = strings.TrimSpace(p.ID)
	p.Factors = append([]FactorRule(nil), p.Factors...)
	for index := range p.Factors {
		p.Factors[index].Factor = Factor(strings.TrimSpace(string(p.Factors[index].Factor)))
	}
	p.PurposeCodes = append([]string(nil), p.PurposeCodes...)
	for index := range p.PurposeCodes {
		p.PurposeCodes[index] = strings.TrimSpace(p.PurposeCodes[index])
	}
	if p.ChallengeTTL <= 0 {
		p.ChallengeTTL = DefaultChallengeTTL
	}
	if p.GrantTTL <= 0 {
		p.GrantTTL = DefaultGrantTTL
	}
	return p
}

func (p Policy) validate() error {
	p = p.normalized()
	if p.ID == "" || (p.Mode != PolicyAnyOf && p.Mode != PolicyAllOf) || len(p.Factors) == 0 || len(p.PurposeCodes) == 0 {
		return ErrInvalidPolicy
	}
	factors := make(map[Factor]struct{}, len(p.Factors))
	for _, rule := range p.Factors {
		if rule.Factor == "" || rule.MaxAttempts <= 0 {
			return ErrInvalidPolicy
		}
		if _, exists := factors[rule.Factor]; exists {
			return fmt.Errorf("%w: duplicate factor %q", ErrInvalidPolicy, rule.Factor)
		}
		factors[rule.Factor] = struct{}{}
	}
	purposes := make(map[string]struct{}, len(p.PurposeCodes))
	for _, purpose := range p.PurposeCodes {
		if purpose == "" {
			return ErrInvalidPolicy
		}
		if _, exists := purposes[purpose]; exists {
			return fmt.Errorf("%w: duplicate purpose %q", ErrInvalidPolicy, purpose)
		}
		purposes[purpose] = struct{}{}
	}
	return nil
}

func (p Policy) allowsPurpose(purpose string) bool {
	purpose = strings.TrimSpace(purpose)
	for _, allowed := range p.PurposeCodes {
		if allowed == purpose {
			return true
		}
	}
	return false
}

func (p Policy) factorRule(factor Factor) (FactorRule, bool) {
	for _, rule := range p.Factors {
		if rule.Factor == factor {
			return rule, true
		}
	}
	return FactorRule{}, false
}

func (p Policy) satisfied(completed map[Factor]bool) bool {
	if p.Mode == PolicyAnyOf {
		for _, rule := range p.Factors {
			if completed[rule.Factor] {
				return true
			}
		}
		return false
	}
	for _, rule := range p.Factors {
		if !completed[rule.Factor] {
			return false
		}
	}
	return true
}

type ChallengeRecord struct {
	ID            string
	TokenDigest   string
	Policy        Policy
	Scope         Scope
	CreatedAt     time.Time
	ExpiresAt     time.Time
	GrantIssuedAt time.Time
}

type FactorTransactionRecord struct {
	ID                 string
	ChallengeID        string
	TransactionDigest  string
	VerificationDigest string
	Factor             Factor
	AttemptCount       int
	MaxAttempts        int
	CreatedAt          time.Time
	CompletedAt        time.Time
	LockedAt           time.Time
}

type GrantRecord struct {
	ID          string
	ChallengeID string
	TokenDigest string
	Scope       Scope
	CreatedAt   time.Time
	ExpiresAt   time.Time
	ConsumedAt  time.Time
}

type AuditEventType string

const (
	AuditChallengeCreated AuditEventType = "challenge.created"
	AuditFactorStarted    AuditEventType = "factor.started"
	AuditFactorFailed     AuditEventType = "factor.failed"
	AuditFactorCompleted  AuditEventType = "factor.completed"
	AuditGrantIssued      AuditEventType = "grant.issued"
	AuditRevealAllowed    AuditEventType = "reveal.allowed"
	AuditRevealDenied     AuditEventType = "reveal.denied"
	AuditRevealSucceeded  AuditEventType = "reveal.succeeded"
	AuditRevealFailed     AuditEventType = "reveal.failed"
)

const (
	RevealReasonCompleted                 = "completed"
	RevealReasonProtectedValueUnavailable = "protected_value_unavailable"
	RevealReasonDecryptionFailed          = "decryption_failed"
	RevealReasonProjectionFailed          = "projection_failed"
	RevealReasonResponseAborted           = "response_aborted"
	FactorCancelReasonDeliveryFailed      = "delivery_failed"
)

type AuditEvent struct {
	ID          uint64         `json:"id"`
	Type        AuditEventType `json:"type"`
	Outcome     string         `json:"outcome"`
	Reason      string         `json:"reason,omitempty"`
	ChallengeID string         `json:"challengeId,omitempty"`
	GrantID     string         `json:"grantId,omitempty"`
	Factor      Factor         `json:"factor,omitempty"`
	Scope       Scope          `json:"scope"`
	CreatedAt   time.Time      `json:"createdAt"`
}

type BeginChallengeRequest struct {
	PolicyID string
	Scope    Scope
}

type BeginChallengeResult struct {
	ChallengeID    string
	ChallengeToken string
	PolicyID       string
	Mode           PolicyMode
	Factors        []FactorRule
	ExpiresAt      time.Time
}

type BeginFactorRequest struct {
	ChallengeToken      string
	ExpectedChallengeID string
	Factor              Factor
	VerificationSecret  string `json:"-"`
}

type BeginFactorResult struct {
	ChallengeID      string
	Factor           Factor
	TransactionToken string
	ExpiresAt        time.Time
}

type CancelFactorRequest struct {
	ChallengeToken      string
	ExpectedChallengeID string
	TransactionToken    string
	Reason              string
}

type CompleteFactorRequest struct {
	ChallengeToken      string
	ExpectedChallengeID string
	TransactionToken    string
	VerificationProof   string
	// Verified is a trusted result from the server-side factor adapter. HTTP
	// handlers must never bind it directly from a client request. When the
	// transaction has a VerificationSecret, Verified is ignored.
	Verified bool `json:"-"`
}

type CompleteFactorResult struct {
	ChallengeID     string
	PolicySatisfied bool
	GrantToken      string
	GrantExpiresAt  time.Time
}

type ConsumeGrantRequest struct {
	GrantToken string
	Scope      Scope
}

type ConsumeGrantResult struct {
	GrantID    string
	ConsumedAt time.Time
	ExpiresAt  time.Time
}
