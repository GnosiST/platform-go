package credentialauth

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

type Repository interface {
	UpsertIdentifier(context.Context, IdentifierRecord) error
	LookupIdentifier(context.Context, IdentifierType, string) (IdentifierRecord, bool, error)

	UpsertPasswordCredential(context.Context, PasswordCredential) error
	PasswordCredential(context.Context, PrincipalRef) (PasswordCredential, bool, error)

	UpsertCredentialChallenge(context.Context, CredentialChallenge) error
	CredentialChallenge(context.Context, string) (CredentialChallenge, bool, error)
	ConsumeCredentialChallenge(context.Context, string, time.Time) (bool, error)
	RecordCredentialChallengeFailure(context.Context, string, time.Time, int) (bool, error)

	UpsertSMSOTPChallenge(context.Context, SMSOTPChallenge) error
	SMSOTPChallenge(context.Context, string) (SMSOTPChallenge, bool, error)
	ConsumeSMSOTPChallenge(context.Context, string, time.Time) (bool, error)
	RecordSMSOTPChallengeFailure(context.Context, string, time.Time, int) (bool, error)
}

type MemoryRepository struct {
	mu                  sync.Mutex
	identifiers         map[string]IdentifierRecord
	passwordCredentials map[string]PasswordCredential
	challenges          map[string]CredentialChallenge
	smsOTPs             map[string]SMSOTPChallenge
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		identifiers:         map[string]IdentifierRecord{},
		passwordCredentials: map[string]PasswordCredential{},
		challenges:          map[string]CredentialChallenge{},
		smsOTPs:             map[string]SMSOTPChallenge{},
	}
}

func (r *MemoryRepository) UpsertIdentifier(ctx context.Context, record IdentifierRecord) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	record, err := normalizedIdentifierRecord(record)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.identifiers[identifierKey(record.IdentifierType, record.IdentifierHash)] = record
	return nil
}

func (r *MemoryRepository) LookupIdentifier(ctx context.Context, identifierType IdentifierType, identifierHash string) (IdentifierRecord, bool, error) {
	if err := checkContext(ctx); err != nil {
		return IdentifierRecord{}, false, err
	}
	identifierType = IdentifierType(strings.TrimSpace(string(identifierType)))
	identifierHash = strings.TrimSpace(identifierHash)
	if !validIdentifierType(identifierType) || identifierHash == "" {
		return IdentifierRecord{}, false, fmt.Errorf("%w: identifier lookup key is invalid", ErrInvalidInput)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	record, ok := r.identifiers[identifierKey(identifierType, identifierHash)]
	return record, ok, nil
}

func (r *MemoryRepository) UpsertPasswordCredential(ctx context.Context, credential PasswordCredential) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	credential, err := normalizedPasswordCredential(credential)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.passwordCredentials[principalKey(credential.Principal)] = credential
	return nil
}

func (r *MemoryRepository) PasswordCredential(ctx context.Context, principal PrincipalRef) (PasswordCredential, bool, error) {
	if err := checkContext(ctx); err != nil {
		return PasswordCredential{}, false, err
	}
	principal, err := principal.normalized()
	if err != nil {
		return PasswordCredential{}, false, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	credential, ok := r.passwordCredentials[principalKey(principal)]
	return credential, ok, nil
}

func (r *MemoryRepository) UpsertCredentialChallenge(ctx context.Context, challenge CredentialChallenge) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	challenge, err := normalizedCredentialChallenge(challenge)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.challenges[challenge.ChallengeID] = challenge
	return nil
}

func (r *MemoryRepository) CredentialChallenge(ctx context.Context, challengeID string) (CredentialChallenge, bool, error) {
	if err := checkContext(ctx); err != nil {
		return CredentialChallenge{}, false, err
	}
	challengeID = strings.TrimSpace(challengeID)
	if challengeID == "" {
		return CredentialChallenge{}, false, fmt.Errorf("%w: challenge id is required", ErrInvalidInput)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	challenge, ok := r.challenges[challengeID]
	return challenge, ok, nil
}

func (r *MemoryRepository) ConsumeCredentialChallenge(ctx context.Context, challengeID string, now time.Time) (bool, error) {
	if err := checkContext(ctx); err != nil {
		return false, err
	}
	challengeID, now, err := normalizedChallengeTransitionKey(challengeID, now)
	if err != nil {
		return false, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	challenge, ok := r.challenges[challengeID]
	if !ok || !challengeTransitionAllowed(challenge.Status, challenge.UsedAt, challenge.ExpiresAt, now) {
		return false, nil
	}
	challenge.UsedAt = now
	r.challenges[challengeID] = challenge
	return true, nil
}

func (r *MemoryRepository) RecordCredentialChallengeFailure(ctx context.Context, challengeID string, now time.Time, maxAttempts int) (bool, error) {
	if err := checkContext(ctx); err != nil {
		return false, err
	}
	challengeID, now, err := normalizedChallengeTransitionInput(challengeID, now, maxAttempts)
	if err != nil {
		return false, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	challenge, ok := r.challenges[challengeID]
	if !ok || !challengeTransitionAllowed(challenge.Status, challenge.UsedAt, challenge.ExpiresAt, now) {
		return false, nil
	}
	challenge.Attempts++
	if challenge.Attempts >= maxAttempts {
		challenge.Status = StatusDisabled
	}
	r.challenges[challengeID] = challenge
	return true, nil
}

func (r *MemoryRepository) UpsertSMSOTPChallenge(ctx context.Context, challenge SMSOTPChallenge) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	challenge, err := normalizedSMSOTPChallenge(challenge)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.smsOTPs[challenge.ChallengeID] = challenge
	return nil
}

func (r *MemoryRepository) SMSOTPChallenge(ctx context.Context, challengeID string) (SMSOTPChallenge, bool, error) {
	if err := checkContext(ctx); err != nil {
		return SMSOTPChallenge{}, false, err
	}
	challengeID = strings.TrimSpace(challengeID)
	if challengeID == "" {
		return SMSOTPChallenge{}, false, fmt.Errorf("%w: sms otp challenge id is required", ErrInvalidInput)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	challenge, ok := r.smsOTPs[challengeID]
	return challenge, ok, nil
}

func (r *MemoryRepository) ConsumeSMSOTPChallenge(ctx context.Context, challengeID string, now time.Time) (bool, error) {
	if err := checkContext(ctx); err != nil {
		return false, err
	}
	challengeID, now, err := normalizedChallengeTransitionKey(challengeID, now)
	if err != nil {
		return false, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	challenge, ok := r.smsOTPs[challengeID]
	if !ok || !challengeTransitionAllowed(challenge.Status, challenge.UsedAt, challenge.ExpiresAt, now) {
		return false, nil
	}
	challenge.UsedAt = now
	r.smsOTPs[challengeID] = challenge
	return true, nil
}

func (r *MemoryRepository) RecordSMSOTPChallengeFailure(ctx context.Context, challengeID string, now time.Time, maxAttempts int) (bool, error) {
	if err := checkContext(ctx); err != nil {
		return false, err
	}
	challengeID, now, err := normalizedChallengeTransitionInput(challengeID, now, maxAttempts)
	if err != nil {
		return false, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	challenge, ok := r.smsOTPs[challengeID]
	if !ok || !challengeTransitionAllowed(challenge.Status, challenge.UsedAt, challenge.ExpiresAt, now) {
		return false, nil
	}
	challenge.Attempts++
	if challenge.Attempts >= maxAttempts {
		challenge.Status = StatusDisabled
	}
	r.smsOTPs[challengeID] = challenge
	return true, nil
}

func normalizedIdentifierRecord(record IdentifierRecord) (IdentifierRecord, error) {
	principal, err := record.Principal.normalized()
	if err != nil {
		return IdentifierRecord{}, err
	}
	record.Principal = principal
	record.IdentifierType = IdentifierType(strings.TrimSpace(string(record.IdentifierType)))
	record.IdentifierHash = strings.TrimSpace(record.IdentifierHash)
	record.MaskedIdentifier = strings.TrimSpace(record.MaskedIdentifier)
	record.Status = normalizeStatus(record.Status)
	if !validIdentifierType(record.IdentifierType) || record.IdentifierHash == "" || record.MaskedIdentifier == "" || record.VerifiedAt.IsZero() || !validStatus(record.Status) {
		return IdentifierRecord{}, fmt.Errorf("%w: identifier record is invalid", ErrInvalidInput)
	}
	return record, nil
}

func normalizedPasswordCredential(credential PasswordCredential) (PasswordCredential, error) {
	principal, err := credential.Principal.normalized()
	if err != nil {
		return PasswordCredential{}, err
	}
	credential.Principal = principal
	credential.PasswordHash = strings.TrimSpace(credential.PasswordHash)
	credential.Algorithm = strings.TrimSpace(credential.Algorithm)
	credential.ParamsVersion = strings.TrimSpace(credential.ParamsVersion)
	credential.Status = normalizeStatus(credential.Status)
	if credential.PasswordHash == "" || credential.Algorithm == "" || credential.ParamsVersion == "" || credential.PasswordUpdatedAt.IsZero() || credential.FailedAttempts < 0 || !validStatus(credential.Status) {
		return PasswordCredential{}, fmt.Errorf("%w: password credential is invalid", ErrInvalidInput)
	}
	return credential, nil
}

func normalizedCredentialChallenge(challenge CredentialChallenge) (CredentialChallenge, error) {
	challenge.ChallengeID = strings.TrimSpace(challenge.ChallengeID)
	challenge.Kind = ChallengeKind(strings.TrimSpace(string(challenge.Kind)))
	challenge.Purpose = ChallengePurpose(strings.TrimSpace(string(challenge.Purpose)))
	challenge.AnswerDigest = strings.TrimSpace(challenge.AnswerDigest)
	challenge.ClientFingerprintHash = strings.TrimSpace(challenge.ClientFingerprintHash)
	challenge.Status = normalizeStatus(challenge.Status)
	if challenge.ChallengeID == "" || !validChallengeKind(challenge.Kind) || !validChallengePurpose(challenge.Purpose) || challenge.AnswerDigest == "" || challenge.ExpiresAt.IsZero() || challenge.Attempts < 0 || !validStatus(challenge.Status) {
		return CredentialChallenge{}, fmt.Errorf("%w: credential challenge is invalid", ErrInvalidInput)
	}
	return challenge, nil
}

func normalizedSMSOTPChallenge(challenge SMSOTPChallenge) (SMSOTPChallenge, error) {
	challenge.ChallengeID = strings.TrimSpace(challenge.ChallengeID)
	challenge.PhoneHash = strings.TrimSpace(challenge.PhoneHash)
	challenge.CodeDigest = strings.TrimSpace(challenge.CodeDigest)
	challenge.MessageID = strings.TrimSpace(challenge.MessageID)
	challenge.Status = normalizeStatus(challenge.Status)
	if challenge.ChallengeID == "" || challenge.PhoneHash == "" || challenge.CodeDigest == "" || challenge.ExpiresAt.IsZero() || challenge.Attempts < 0 || !validStatus(challenge.Status) {
		return SMSOTPChallenge{}, fmt.Errorf("%w: sms otp challenge is invalid", ErrInvalidInput)
	}
	return challenge, nil
}

func normalizedChallengeTransitionKey(challengeID string, now time.Time) (string, time.Time, error) {
	challengeID = strings.TrimSpace(challengeID)
	if challengeID == "" || now.IsZero() {
		return "", time.Time{}, fmt.Errorf("%w: challenge transition key is invalid", ErrInvalidInput)
	}
	return challengeID, now.UTC(), nil
}

func normalizedChallengeTransitionInput(challengeID string, now time.Time, maxAttempts int) (string, time.Time, error) {
	challengeID, now, err := normalizedChallengeTransitionKey(challengeID, now)
	if err != nil {
		return "", time.Time{}, err
	}
	if maxAttempts <= 0 {
		return "", time.Time{}, fmt.Errorf("%w: max challenge attempts must be positive", ErrInvalidInput)
	}
	return challengeID, now, nil
}

func challengeTransitionAllowed(status RecordStatus, usedAt time.Time, expiresAt time.Time, now time.Time) bool {
	return normalizeStatus(status) == StatusEnabled && usedAt.IsZero() && now.Before(expiresAt)
}

func checkContext(ctx context.Context) error {
	if ctx == nil {
		return context.Canceled
	}
	return ctx.Err()
}

func identifierKey(identifierType IdentifierType, identifierHash string) string {
	return string(identifierType) + "\x00" + identifierHash
}

func principalKey(principal PrincipalRef) string {
	return string(principal.Type) + "\x00" + principal.ID
}
