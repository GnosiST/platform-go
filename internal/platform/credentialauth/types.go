package credentialauth

import (
	"errors"
	"strings"
	"time"
)

const (
	PrincipalTypeAdmin PrincipalType = "admin"
	PrincipalTypeApp   PrincipalType = "app"

	IdentifierTypeUsername IdentifierType = "username"
	IdentifierTypePhone    IdentifierType = "phone"
	IdentifierTypeEmail    IdentifierType = "email"

	StatusEnabled  RecordStatus = "enabled"
	StatusDisabled RecordStatus = "disabled"

	ChallengeKindCaptcha ChallengeKind = "captcha"
	ChallengeKindSlider  ChallengeKind = "slider"

	ChallengePurposeLogin ChallengePurpose = "login"

	DefaultMaxPasswordAttempts  = 5
	DefaultPasswordLockDuration = 15 * time.Minute
	DefaultCredentialChallengeTTL = 5 * time.Minute
	DefaultMaxChallengeAttempts = 5
	DefaultMaxSMSOTPAttempts    = 5
)

var (
	ErrInvalidInput        = errors.New("credential-auth input is invalid")
	ErrCredentialRejected  = errors.New("credential rejected")
	ErrCredentialLocked    = errors.New("credential locked")
	ErrInvalidSecret       = errors.New("credential secret is invalid")
	ErrChallengeRejected   = errors.New("credential challenge rejected")
	ErrChallengeExpired    = errors.New("credential challenge expired")
	ErrChallengeConsumed   = errors.New("credential challenge consumed")
	ErrRepositoryInvariant = errors.New("credential-auth repository invariant failed")
)

type PrincipalType string

type IdentifierType string

type RecordStatus string

type ChallengeKind string

type ChallengePurpose string

type PrincipalRef struct {
	Type PrincipalType
	ID   string
}

type Identifier struct {
	Type  IdentifierType
	Value string
}

type NormalizedIdentifier struct {
	Type             IdentifierType
	Value            string
	MaskedIdentifier string
}

type IdentifierRecord struct {
	Principal        PrincipalRef
	IdentifierType   IdentifierType
	IdentifierHash   string
	MaskedIdentifier string
	VerifiedAt       time.Time
	Status           RecordStatus
}

type PasswordCredential struct {
	Principal         PrincipalRef
	PasswordHash      string
	Algorithm         string
	ParamsVersion     string
	PasswordUpdatedAt time.Time
	MustChange        bool
	FailedAttempts    int
	LockedUntil       time.Time
	Status            RecordStatus
}

type CredentialChallenge struct {
	ChallengeID           string
	Kind                  ChallengeKind
	Purpose               ChallengePurpose
	AnswerDigest          string
	ExpiresAt             time.Time
	Attempts              int
	UsedAt                time.Time
	ClientFingerprintHash string
	Status                RecordStatus
}

type SMSOTPChallenge struct {
	ChallengeID string
	PhoneHash   string
	CodeDigest  string
	ExpiresAt   time.Time
	Attempts    int
	MessageID   string
	UsedAt      time.Time
	Status      RecordStatus
}

func normalizeStatus(status RecordStatus) RecordStatus {
	if strings.TrimSpace(string(status)) == "" {
		return StatusEnabled
	}
	return RecordStatus(strings.TrimSpace(string(status)))
}

func (p PrincipalRef) normalized() (PrincipalRef, error) {
	p.Type = PrincipalType(strings.TrimSpace(string(p.Type)))
	p.ID = strings.TrimSpace(p.ID)
	if p.ID == "" {
		return PrincipalRef{}, ErrInvalidInput
	}
	switch p.Type {
	case PrincipalTypeAdmin, PrincipalTypeApp:
		return p, nil
	default:
		return PrincipalRef{}, ErrInvalidInput
	}
}

func validIdentifierType(identifierType IdentifierType) bool {
	switch identifierType {
	case IdentifierTypeUsername, IdentifierTypePhone, IdentifierTypeEmail:
		return true
	default:
		return false
	}
}

func validStatus(status RecordStatus) bool {
	switch normalizeStatus(status) {
	case StatusEnabled, StatusDisabled:
		return true
	default:
		return false
	}
}

func validChallengeKind(kind ChallengeKind) bool {
	switch kind {
	case ChallengeKindCaptcha, ChallengeKindSlider:
		return true
	default:
		return false
	}
}

func validChallengePurpose(purpose ChallengePurpose) bool {
	return strings.TrimSpace(string(purpose)) != ""
}
