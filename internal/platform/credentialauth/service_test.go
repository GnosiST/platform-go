package credentialauth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestServiceRegistersIdentifierWithNormalizationHashAndLookup(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 19, 10, 0, 0, 0, time.UTC)
	repository := NewMemoryRepository()
	hasher, err := NewHMACIdentifierHasher([]byte(strings.Repeat("i", 32)))
	if err != nil {
		t.Fatalf("NewHMACIdentifierHasher() error = %v", err)
	}
	service, err := NewService(Options{
		Repository:       repository,
		IdentifierHasher: hasher,
		Now:              func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	record, err := service.RegisterIdentifier(ctx, RegisterIdentifierInput{
		Principal:  PrincipalRef{Type: PrincipalTypeAdmin, ID: "user-1"},
		Identifier: Identifier{Type: IdentifierTypePhone, Value: "+86 (138) 0013-8000"},
		VerifiedAt: now,
	})
	if err != nil {
		t.Fatalf("RegisterIdentifier() error = %v", err)
	}
	if !strings.HasPrefix(record.IdentifierHash, identifierHashPrefix) {
		t.Fatalf("identifier hash = %q, want versioned HMAC", record.IdentifierHash)
	}
	if strings.Contains(record.IdentifierHash, "13800138000") || strings.Contains(record.IdentifierHash, "+8613800138000") {
		t.Fatalf("identifier hash leaked raw phone: %q", record.IdentifierHash)
	}
	if record.MaskedIdentifier != "+86****8000" {
		t.Fatalf("masked identifier = %q, want +86****8000", record.MaskedIdentifier)
	}

	resolved, ok, err := service.ResolveIdentifier(ctx, Identifier{Type: IdentifierTypePhone, Value: "+8613800138000"})
	if err != nil || !ok {
		t.Fatalf("ResolveIdentifier() = %+v, %v, %v; want record", resolved, ok, err)
	}
	if resolved.Principal != (PrincipalRef{Type: PrincipalTypeAdmin, ID: "user-1"}) {
		t.Fatalf("resolved principal = %+v, want admin user-1", resolved.Principal)
	}

	otherHasher, err := NewHMACIdentifierHasher([]byte(strings.Repeat("j", 32)))
	if err != nil {
		t.Fatalf("NewHMACIdentifierHasher(other) error = %v", err)
	}
	otherHash, err := otherHasher.HashIdentifier(IdentifierTypePhone, "+8613800138000")
	if err != nil {
		t.Fatalf("HashIdentifier(other) error = %v", err)
	}
	if otherHash == record.IdentifierHash {
		t.Fatalf("identifier hash did not separate HMAC keys: %q", otherHash)
	}
}

func TestServiceVerifiesPasswordThroughBoundaryAndLocksAfterFailures(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 19, 10, 0, 0, 0, time.UTC)
	repository, service := newPasswordServiceForTest(t, &passwordVerifierStub{validSecret: "correct"}, &now, 2)
	principal := PrincipalRef{Type: PrincipalTypeAdmin, ID: "admin-1"}
	putIdentifierAndPasswordForTest(t, ctx, service, principal, "admin", "hash:v1", now)

	if _, err := service.VerifyPassword(ctx, PasswordLoginInput{Identifier: Identifier{Type: IdentifierTypeUsername, Value: "admin"}, Secret: "wrong"}); !errors.Is(err, ErrCredentialRejected) {
		t.Fatalf("VerifyPassword(first wrong) error = %v, want ErrCredentialRejected", err)
	}
	credential := passwordCredentialForTest(t, ctx, repository, principal)
	if credential.FailedAttempts != 1 || !credential.LockedUntil.IsZero() {
		t.Fatalf("credential after first wrong = %+v, want one failure and no lock", credential)
	}

	if _, err := service.VerifyPassword(ctx, PasswordLoginInput{Identifier: Identifier{Type: IdentifierTypeUsername, Value: "admin"}, Secret: "wrong"}); !errors.Is(err, ErrCredentialRejected) {
		t.Fatalf("VerifyPassword(second wrong) error = %v, want ErrCredentialRejected", err)
	}
	credential = passwordCredentialForTest(t, ctx, repository, principal)
	if credential.FailedAttempts != 2 || !credential.LockedUntil.Equal(now.Add(DefaultPasswordLockDuration)) {
		t.Fatalf("credential after second wrong = %+v, want locked credential", credential)
	}

	if result, err := service.VerifyPassword(ctx, PasswordLoginInput{Identifier: Identifier{Type: IdentifierTypeUsername, Value: "admin"}, Secret: "correct"}); !errors.Is(err, ErrCredentialLocked) || !result.LockedUntil.Equal(credential.LockedUntil) {
		t.Fatalf("VerifyPassword(locked correct) = %+v, %v; want locked result", result, err)
	}

	now = now.Add(DefaultPasswordLockDuration + time.Second)
	result, err := service.VerifyPassword(ctx, PasswordLoginInput{Identifier: Identifier{Type: IdentifierTypeUsername, Value: "admin"}, Secret: "correct"})
	if err != nil {
		t.Fatalf("VerifyPassword(after lock) error = %v", err)
	}
	if result.Principal != principal || result.Rehashed {
		t.Fatalf("VerifyPassword(after lock) result = %+v, want principal without rehash", result)
	}
	credential = passwordCredentialForTest(t, ctx, repository, principal)
	if credential.FailedAttempts != 0 || !credential.LockedUntil.IsZero() {
		t.Fatalf("credential after success = %+v, want failures reset and lock cleared", credential)
	}
}

func TestServiceUpdatesPasswordHashWhenVerifierRequestsRehash(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 19, 10, 0, 0, 0, time.UTC)
	repository, service := newPasswordServiceForTest(t, &passwordVerifierStub{
		validSecret: "correct",
		rehash: PasswordVerification{
			Valid:           true,
			NeedsRehash:     true,
			ReplacementHash: "hash:v2",
			Algorithm:       "argon2id",
			ParamsVersion:   "v2",
		},
	}, &now, 5)
	principal := PrincipalRef{Type: PrincipalTypeAdmin, ID: "admin-1"}
	putIdentifierAndPasswordForTest(t, ctx, service, principal, "admin", "hash:v1", now.Add(-time.Hour))

	result, err := service.VerifyPassword(ctx, PasswordLoginInput{Identifier: Identifier{Type: IdentifierTypeUsername, Value: "admin"}, Secret: "correct"})
	if err != nil {
		t.Fatalf("VerifyPassword() error = %v", err)
	}
	if !result.Rehashed || result.ParamsVersion != "v2" {
		t.Fatalf("VerifyPassword() result = %+v, want rehash to v2", result)
	}
	credential := passwordCredentialForTest(t, ctx, repository, principal)
	if credential.PasswordHash != "hash:v2" || credential.Algorithm != "argon2id" || credential.ParamsVersion != "v2" || !credential.PasswordUpdatedAt.Equal(now) {
		t.Fatalf("credential after rehash = %+v, want v2 hash written at now", credential)
	}
}

func TestServiceConsumesCredentialChallengeAndSMSOTPOnce(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 19, 10, 0, 0, 0, time.UTC)
	repository := NewMemoryRepository()
	service, err := NewService(Options{
		Repository: repository,
		Now:        func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	challenge := CredentialChallenge{
		ChallengeID:           "captcha-1",
		Kind:                  ChallengeKindCaptcha,
		Purpose:               ChallengePurposeLogin,
		AnswerDigest:          "digest:answer",
		ClientFingerprintHash: "fingerprint:1",
		ExpiresAt:             now.Add(time.Minute),
	}
	if err := service.PutCredentialChallenge(ctx, challenge); err != nil {
		t.Fatalf("PutCredentialChallenge() error = %v", err)
	}
	if err := service.VerifyCredentialChallenge(ctx, CredentialChallengeProof{
		ChallengeID:           "captcha-1",
		Kind:                  ChallengeKindCaptcha,
		Purpose:               ChallengePurposeLogin,
		AnswerDigest:          "digest:wrong",
		ClientFingerprintHash: "fingerprint:1",
	}); !errors.Is(err, ErrChallengeRejected) {
		t.Fatalf("VerifyCredentialChallenge(wrong) error = %v, want ErrChallengeRejected", err)
	}
	storedChallenge, ok, err := repository.CredentialChallenge(ctx, "captcha-1")
	if err != nil || !ok || storedChallenge.Attempts != 1 || !storedChallenge.UsedAt.IsZero() {
		t.Fatalf("stored challenge after wrong = %+v, %v, %v; want one attempt and unused", storedChallenge, ok, err)
	}
	if err := service.VerifyCredentialChallenge(ctx, CredentialChallengeProof{
		ChallengeID:           "captcha-1",
		Kind:                  ChallengeKindCaptcha,
		Purpose:               ChallengePurposeLogin,
		AnswerDigest:          "digest:answer",
		ClientFingerprintHash: "fingerprint:1",
	}); err != nil {
		t.Fatalf("VerifyCredentialChallenge(correct) error = %v", err)
	}
	if err := service.VerifyCredentialChallenge(ctx, CredentialChallengeProof{
		ChallengeID:           "captcha-1",
		Kind:                  ChallengeKindCaptcha,
		Purpose:               ChallengePurposeLogin,
		AnswerDigest:          "digest:answer",
		ClientFingerprintHash: "fingerprint:1",
	}); !errors.Is(err, ErrChallengeConsumed) {
		t.Fatalf("VerifyCredentialChallenge(replay) error = %v, want ErrChallengeConsumed", err)
	}

	otp := SMSOTPChallenge{
		ChallengeID: "otp-1",
		PhoneHash:   "phone-hash",
		CodeDigest:  "code-digest",
		MessageID:   "message-1",
		ExpiresAt:   now.Add(time.Minute),
	}
	if err := service.PutSMSOTPChallenge(ctx, otp); err != nil {
		t.Fatalf("PutSMSOTPChallenge() error = %v", err)
	}
	if err := service.VerifySMSOTP(ctx, SMSOTPProof{ChallengeID: "otp-1", PhoneHash: "other-phone-hash", CodeDigest: "code-digest"}); !errors.Is(err, ErrChallengeRejected) {
		t.Fatalf("VerifySMSOTP(wrong phone) error = %v, want ErrChallengeRejected", err)
	}
	storedOTP, ok, err := repository.SMSOTPChallenge(ctx, "otp-1")
	if err != nil || !ok || storedOTP.Attempts != 1 || !storedOTP.UsedAt.IsZero() {
		t.Fatalf("stored otp after wrong = %+v, %v, %v; want one attempt and unused", storedOTP, ok, err)
	}
	if err := service.VerifySMSOTP(ctx, SMSOTPProof{ChallengeID: "otp-1", PhoneHash: "phone-hash", CodeDigest: "code-digest"}); err != nil {
		t.Fatalf("VerifySMSOTP(correct) error = %v", err)
	}
	if err := service.VerifySMSOTP(ctx, SMSOTPProof{ChallengeID: "otp-1", PhoneHash: "phone-hash", CodeDigest: "code-digest"}); !errors.Is(err, ErrChallengeConsumed) {
		t.Fatalf("VerifySMSOTP(replay) error = %v, want ErrChallengeConsumed", err)
	}
}

func TestServiceRejectsExpiredChallenges(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 19, 10, 0, 0, 0, time.UTC)
	repository := NewMemoryRepository()
	service, err := NewService(Options{Repository: repository, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if err := service.PutCredentialChallenge(ctx, CredentialChallenge{
		ChallengeID:  "expired",
		Kind:         ChallengeKindSlider,
		Purpose:      ChallengePurposeLogin,
		AnswerDigest: "digest",
		ExpiresAt:    now,
	}); err != nil {
		t.Fatalf("PutCredentialChallenge() error = %v", err)
	}
	if err := service.VerifyCredentialChallenge(ctx, CredentialChallengeProof{
		ChallengeID:  "expired",
		Kind:         ChallengeKindSlider,
		Purpose:      ChallengePurposeLogin,
		AnswerDigest: "digest",
	}); !errors.Is(err, ErrChallengeExpired) {
		t.Fatalf("VerifyCredentialChallenge(expired) error = %v, want ErrChallengeExpired", err)
	}
}

func newPasswordServiceForTest(t *testing.T, verifier PasswordVerifier, now *time.Time, maxAttempts int) (*MemoryRepository, *Service) {
	t.Helper()
	repository := NewMemoryRepository()
	hasher, err := NewHMACIdentifierHasher([]byte(strings.Repeat("i", 32)))
	if err != nil {
		t.Fatalf("NewHMACIdentifierHasher() error = %v", err)
	}
	service, err := NewService(Options{
		Repository:          repository,
		IdentifierHasher:    hasher,
		PasswordVerifier:    verifier,
		MaxPasswordAttempts: maxAttempts,
		Now: func() time.Time {
			return *now
		},
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return repository, service
}

func putIdentifierAndPasswordForTest(t *testing.T, ctx context.Context, service *Service, principal PrincipalRef, username string, hash string, updatedAt time.Time) {
	t.Helper()
	if _, err := service.RegisterIdentifier(ctx, RegisterIdentifierInput{
		Principal:  principal,
		Identifier: Identifier{Type: IdentifierTypeUsername, Value: username},
		VerifiedAt: updatedAt,
	}); err != nil {
		t.Fatalf("RegisterIdentifier() error = %v", err)
	}
	if err := service.PutPasswordCredential(ctx, PasswordCredential{
		Principal:         principal,
		PasswordHash:      hash,
		Algorithm:         "argon2id",
		ParamsVersion:     "v1",
		PasswordUpdatedAt: updatedAt,
	}); err != nil {
		t.Fatalf("PutPasswordCredential() error = %v", err)
	}
}

func passwordCredentialForTest(t *testing.T, ctx context.Context, repository *MemoryRepository, principal PrincipalRef) PasswordCredential {
	t.Helper()
	credential, ok, err := repository.PasswordCredential(ctx, principal)
	if err != nil || !ok {
		t.Fatalf("PasswordCredential() = %+v, %v, %v; want credential", credential, ok, err)
	}
	return credential
}

type passwordVerifierStub struct {
	validSecret string
	rehash      PasswordVerification
}

func (v *passwordVerifierStub) VerifyPassword(_ context.Context, _ PasswordCredential, secret string) (PasswordVerification, error) {
	if secret != v.validSecret {
		return PasswordVerification{}, nil
	}
	if v.rehash.Valid {
		return v.rehash, nil
	}
	return PasswordVerification{Valid: true}, nil
}
