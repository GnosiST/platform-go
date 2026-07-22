package credentialauth

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
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

func TestServiceCreatesCredentialChallengeWithDigestAndConsumesProof(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 19, 10, 0, 0, 0, time.UTC)
	repository := NewMemoryRepository()
	hasher, err := NewHMACIdentifierHasher([]byte(strings.Repeat("i", 32)))
	if err != nil {
		t.Fatalf("NewHMACIdentifierHasher() error = %v", err)
	}
	service, err := NewService(Options{
		Repository:           repository,
		ChallengeProofHasher: hasher,
		ChallengeIDGenerator: func() (string, error) { return "challenge-fixed", nil },
		ChallengeMaterialGenerator: func(kind ChallengeKind) (ChallengeMaterial, error) {
			if kind != ChallengeKindSlider {
				t.Fatalf("challenge kind = %q, want slider", kind)
			}
			return ChallengeMaterial{
				Prompt:     "Move the slider.",
				Parameters: map[string]string{"tileX": "5", "unit": "px"},
				Proof:      "x=72&y=14",
			}, nil
		},
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	created, err := service.CreateCredentialChallenge(ctx, CreateCredentialChallengeInput{
		Kind:                  ChallengeKindSlider,
		Purpose:               ChallengePurposeLogin,
		ClientFingerprintHash: "fingerprint-hash",
		TTL:                   2 * time.Minute,
	})
	if err != nil {
		t.Fatalf("CreateCredentialChallenge() error = %v", err)
	}
	if _, ok := reflect.TypeOf(CreatedCredentialChallenge{}).FieldByName("Proof"); ok {
		t.Fatal("CreatedCredentialChallenge must not expose server proof")
	}
	if created.ChallengeID != "challenge-fixed" || created.Kind != ChallengeKindSlider || created.Parameters["tileX"] != "5" || created.Parameters["targetOffset"] != "" || !created.ExpiresAt.Equal(now.Add(2*time.Minute)) {
		t.Fatalf("created challenge = %+v, want fixed slider material", created)
	}
	encoded, err := json.Marshal(created)
	if err != nil {
		t.Fatalf("marshal created challenge: %v", err)
	}
	if strings.Contains(string(encoded), "x=72&y=14") || strings.Contains(string(encoded), "targetOffset") {
		t.Fatalf("created challenge leaked proof material: %s", encoded)
	}
	stored, ok, err := repository.CredentialChallenge(ctx, "challenge-fixed")
	if err != nil || !ok {
		t.Fatalf("CredentialChallenge() = %+v, %v, %v; want stored challenge", stored, ok, err)
	}
	if stored.AnswerDigest == "x=72&y=14" || (!strings.HasPrefix(stored.AnswerDigest, challengeProofHashPrefix) && !strings.HasPrefix(stored.AnswerDigest, challengeProofDigestSetPrefix)) || strings.Contains(stored.AnswerDigest, "x=72") || stored.ClientFingerprintHash != "fingerprint-hash" {
		t.Fatalf("stored challenge = %+v, want digest and fingerprint binding", stored)
	}
	if err := service.VerifyCredentialChallenge(ctx, CredentialChallengeProof{
		ChallengeID:           "challenge-fixed",
		Kind:                  ChallengeKindSlider,
		Purpose:               ChallengePurposeLogin,
		Proof:                 "x=72&y=14",
		ClientFingerprintHash: "fingerprint-hash",
	}); err != nil {
		t.Fatalf("VerifyCredentialChallenge() error = %v", err)
	}
	if err := service.VerifyCredentialChallenge(ctx, CredentialChallengeProof{
		ChallengeID:           "challenge-fixed",
		Kind:                  ChallengeKindSlider,
		Purpose:               ChallengePurposeLogin,
		Proof:                 "x=72&y=14",
		ClientFingerprintHash: "fingerprint-hash",
	}); !errors.Is(err, ErrChallengeConsumed) {
		t.Fatalf("VerifyCredentialChallenge(reuse) error = %v, want ErrChallengeConsumed", err)
	}
}

func TestServiceAcceptsNormalizedSliderChallengeProofWithinTolerance(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 19, 10, 0, 0, 0, time.UTC)
	repository := NewMemoryRepository()
	hasher, err := NewHMACIdentifierHasher([]byte(strings.Repeat("i", 32)))
	if err != nil {
		t.Fatalf("NewHMACIdentifierHasher() error = %v", err)
	}
	nextChallenge := 0
	service, err := NewService(Options{
		Repository:           repository,
		ChallengeProofHasher: hasher,
		ChallengeIDGenerator: func() (string, error) {
			nextChallenge++
			return "slider-normalized-" + strconv.Itoa(nextChallenge), nil
		},
		ChallengeMaterialGenerator: func(kind ChallengeKind) (ChallengeMaterial, error) {
			return ChallengeMaterial{
				Prompt:     "Move the slider.",
				Parameters: map[string]string{"tileX": "5", "tileY": "14", "unit": "px"},
				Proof:      "x=72&y=14",
			}, nil
		},
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	created, err := service.CreateCredentialChallenge(ctx, CreateCredentialChallengeInput{
		Kind:    ChallengeKindSlider,
		Purpose: ChallengePurposeLogin,
	})
	if err != nil {
		t.Fatalf("CreateCredentialChallenge() error = %v", err)
	}
	if err := service.VerifyCredentialChallenge(ctx, CredentialChallengeProof{
		ChallengeID: created.ChallengeID,
		Kind:        ChallengeKindSlider,
		Purpose:     ChallengePurposeLogin,
		Proof:       " y = 14 & x = 74.4 ",
	}); err != nil {
		t.Fatalf("VerifyCredentialChallenge(normalized in-tolerance proof) error = %v", err)
	}

	created, err = service.CreateCredentialChallenge(ctx, CreateCredentialChallengeInput{
		Kind:    ChallengeKindSlider,
		Purpose: ChallengePurposeLogin,
	})
	if err != nil {
		t.Fatalf("CreateCredentialChallenge(second) error = %v", err)
	}
	if err := service.VerifyCredentialChallenge(ctx, CredentialChallengeProof{
		ChallengeID: created.ChallengeID,
		Kind:        ChallengeKindSlider,
		Purpose:     ChallengePurposeLogin,
		Proof:       `{"x":76.1,"y":14}`,
	}); !errors.Is(err, ErrChallengeRejected) {
		t.Fatalf("VerifyCredentialChallenge(out-of-tolerance proof) error = %v, want ErrChallengeRejected", err)
	}
}

func TestServiceCountsMalformedSliderChallengeProofAsAttempt(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 19, 10, 0, 0, 0, time.UTC)
	repository := NewMemoryRepository()
	hasher, err := NewHMACIdentifierHasher([]byte(strings.Repeat("i", 32)))
	if err != nil {
		t.Fatalf("NewHMACIdentifierHasher() error = %v", err)
	}
	service, err := NewService(Options{
		Repository:           repository,
		ChallengeProofHasher: hasher,
		ChallengeIDGenerator: func() (string, error) { return "slider-malformed", nil },
		ChallengeMaterialGenerator: func(kind ChallengeKind) (ChallengeMaterial, error) {
			return ChallengeMaterial{
				Prompt:     "Complete the test challenge.",
				Parameters: map[string]string{"tileX": "5", "tileY": "14", "unit": "px"},
				Proof:      "x=72&y=14",
			}, nil
		},
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	created, err := service.CreateCredentialChallenge(ctx, CreateCredentialChallengeInput{
		Kind:    ChallengeKindSlider,
		Purpose: ChallengePurposeLogin,
	})
	if err != nil {
		t.Fatalf("CreateCredentialChallenge() error = %v", err)
	}

	if err := service.VerifyCredentialChallenge(ctx, CredentialChallengeProof{
		ChallengeID: created.ChallengeID,
		Kind:        ChallengeKindSlider,
		Purpose:     ChallengePurposeLogin,
		Proof:       "not-a-coordinate",
		MaxAttempts: 2,
	}); !errors.Is(err, ErrChallengeRejected) {
		t.Fatalf("VerifyCredentialChallenge(malformed proof) error = %v, want ErrChallengeRejected", err)
	}
	stored, ok, err := repository.CredentialChallenge(ctx, created.ChallengeID)
	if err != nil || !ok {
		t.Fatalf("CredentialChallenge() = %+v, %v, %v; want stored challenge", stored, ok, err)
	}
	if stored.Attempts != 1 || !stored.UsedAt.IsZero() {
		t.Fatalf("stored challenge after malformed proof = %+v, want one rejected attempt and unused", stored)
	}

	if err := service.VerifyCredentialChallenge(ctx, CredentialChallengeProof{
		ChallengeID: created.ChallengeID,
		Kind:        ChallengeKindSlider,
		Purpose:     ChallengePurposeLogin,
		Proof:       "x=72&y=14",
		MaxAttempts: 2,
	}); err != nil {
		t.Fatalf("VerifyCredentialChallenge(correct proof after malformed) error = %v", err)
	}
}

func TestServiceDisablesCredentialChallengeAtMaxAttempts(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 19, 10, 0, 0, 0, time.UTC)
	repository := NewMemoryRepository()
	service, err := NewService(Options{Repository: repository, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if err := service.PutCredentialChallenge(ctx, CredentialChallenge{
		ChallengeID:  "captcha-max",
		Kind:         ChallengeKindCaptcha,
		Purpose:      ChallengePurposeLogin,
		AnswerDigest: "digest:answer",
		ExpiresAt:    now.Add(time.Minute),
	}); err != nil {
		t.Fatalf("PutCredentialChallenge() error = %v", err)
	}

	for attempt := 1; attempt <= 2; attempt++ {
		if err := service.VerifyCredentialChallenge(ctx, CredentialChallengeProof{
			ChallengeID:  "captcha-max",
			Kind:         ChallengeKindCaptcha,
			Purpose:      ChallengePurposeLogin,
			AnswerDigest: "digest:wrong",
			MaxAttempts:  2,
		}); !errors.Is(err, ErrChallengeRejected) {
			t.Fatalf("VerifyCredentialChallenge(wrong attempt %d) error = %v, want ErrChallengeRejected", attempt, err)
		}
	}
	stored, ok, err := repository.CredentialChallenge(ctx, "captcha-max")
	if err != nil || !ok {
		t.Fatalf("CredentialChallenge() = %+v, %v, %v; want stored challenge", stored, ok, err)
	}
	if stored.Attempts != 2 || stored.Status != StatusDisabled || !stored.UsedAt.IsZero() {
		t.Fatalf("stored challenge after max attempts = %+v, want disabled at exactly two attempts", stored)
	}
	if err := service.VerifyCredentialChallenge(ctx, CredentialChallengeProof{
		ChallengeID:  "captcha-max",
		Kind:         ChallengeKindCaptcha,
		Purpose:      ChallengePurposeLogin,
		AnswerDigest: "digest:answer",
		MaxAttempts:  2,
	}); !errors.Is(err, ErrChallengeRejected) {
		t.Fatalf("VerifyCredentialChallenge(correct after max attempts) error = %v, want ErrChallengeRejected", err)
	}
}

func TestServiceDisablesSMSOTPAtMaxAttempts(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 19, 10, 0, 0, 0, time.UTC)
	repository := NewMemoryRepository()
	service, err := NewService(Options{Repository: repository, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if err := service.PutSMSOTPChallenge(ctx, SMSOTPChallenge{
		ChallengeID: "otp-max",
		PhoneHash:   "phone-hash",
		CodeDigest:  "code-digest",
		ExpiresAt:   now.Add(time.Minute),
	}); err != nil {
		t.Fatalf("PutSMSOTPChallenge() error = %v", err)
	}

	for attempt := 1; attempt <= 2; attempt++ {
		if err := service.VerifySMSOTP(ctx, SMSOTPProof{
			ChallengeID: "otp-max",
			PhoneHash:   "phone-hash",
			CodeDigest:  "wrong-code-digest",
			MaxAttempts: 2,
		}); !errors.Is(err, ErrChallengeRejected) {
			t.Fatalf("VerifySMSOTP(wrong attempt %d) error = %v, want ErrChallengeRejected", attempt, err)
		}
	}
	stored, ok, err := repository.SMSOTPChallenge(ctx, "otp-max")
	if err != nil || !ok {
		t.Fatalf("SMSOTPChallenge() = %+v, %v, %v; want stored challenge", stored, ok, err)
	}
	if stored.Attempts != 2 || stored.Status != StatusDisabled || !stored.UsedAt.IsZero() {
		t.Fatalf("stored otp after max attempts = %+v, want disabled at exactly two attempts", stored)
	}
	if err := service.VerifySMSOTP(ctx, SMSOTPProof{
		ChallengeID: "otp-max",
		PhoneHash:   "phone-hash",
		CodeDigest:  "code-digest",
		MaxAttempts: 2,
	}); !errors.Is(err, ErrChallengeRejected) {
		t.Fatalf("VerifySMSOTP(correct after max attempts) error = %v, want ErrChallengeRejected", err)
	}
}

func TestGoCaptchaSlideChallengeDoesNotExposeProofAndRequiresCoordinates(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 19, 10, 0, 0, 0, time.UTC)
	repository := NewMemoryRepository()
	hasher, err := NewHMACIdentifierHasher([]byte(strings.Repeat("i", 32)))
	if err != nil {
		t.Fatalf("NewHMACIdentifierHasher() error = %v", err)
	}
	generator, err := NewGoCaptchaSlideMaterialGenerator(GoCaptchaSlideOptions{
		ImageWidth:   240,
		ImageHeight:  160,
		GraphSizeMin: 42,
		GraphSizeMax: 48,
	})
	if err != nil {
		t.Fatalf("NewGoCaptchaSlideMaterialGenerator() error = %v", err)
	}
	serverProof := ""
	service, err := NewService(Options{
		Repository:           repository,
		ChallengeProofHasher: hasher,
		ChallengeIDGenerator: func() (string, error) { return "go-captcha-slide-fixed", nil },
		ChallengeMaterialGenerator: func(kind ChallengeKind) (ChallengeMaterial, error) {
			material, err := generator(kind)
			if err == nil {
				serverProof = material.Proof
			}
			return material, err
		},
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	created, err := service.CreateCredentialChallenge(ctx, CreateCredentialChallengeInput{
		Kind:    ChallengeKindSlider,
		Purpose: ChallengePurposeLogin,
	})
	if err != nil {
		t.Fatalf("CreateCredentialChallenge() error = %v", err)
	}
	if serverProof == "" {
		t.Fatal("go-captcha slide generator did not produce a server proof")
	}
	if created.Kind != ChallengeKindSlider || created.Prompt == "" || created.Parameters["masterImage"] == "" || created.Parameters["tileImage"] == "" {
		t.Fatalf("created go-captcha challenge = %+v, want slide display material", created)
	}
	if !strings.HasPrefix(created.Parameters["masterImage"], "data:image/jpeg;base64,") || !strings.HasPrefix(created.Parameters["tileImage"], "data:image/png;base64,") {
		t.Fatalf("created image parameters = %+v, want data URLs", created.Parameters)
	}
	if created.Parameters["imageWidth"] != "240" || created.Parameters["imageHeight"] != "160" || created.Parameters["proofFormat"] == "" {
		t.Fatalf("created slide parameters = %+v, want display metadata", created.Parameters)
	}
	for _, forbidden := range []string{"proof", "answer", "targetX", "targetY", "targetOffset"} {
		if _, ok := created.Parameters[forbidden]; ok {
			t.Fatalf("created parameters exposed %q: %+v", forbidden, created.Parameters)
		}
	}
	encoded, err := json.Marshal(created)
	if err != nil {
		t.Fatalf("marshal go-captcha challenge: %v", err)
	}
	if strings.Contains(string(encoded), serverProof) {
		t.Fatalf("created go-captcha challenge leaked proof %q in %s", serverProof, encoded)
	}

	if err := service.VerifyCredentialChallenge(ctx, CredentialChallengeProof{
		ChallengeID: "go-captcha-slide-fixed",
		Kind:        ChallengeKindSlider,
		Purpose:     ChallengePurposeLogin,
		Proof:       "x=0&y=0",
	}); !errors.Is(err, ErrChallengeRejected) {
		t.Fatalf("VerifyCredentialChallenge(wrong coordinates) error = %v, want ErrChallengeRejected", err)
	}
	stored, ok, err := repository.CredentialChallenge(ctx, "go-captcha-slide-fixed")
	if err != nil || !ok || stored.Attempts != 1 || !stored.UsedAt.IsZero() {
		t.Fatalf("stored challenge after wrong proof = %+v, %v, %v; want one rejected attempt", stored, ok, err)
	}
	if err := service.VerifyCredentialChallenge(ctx, CredentialChallengeProof{
		ChallengeID: "go-captcha-slide-fixed",
		Kind:        ChallengeKindSlider,
		Purpose:     ChallengePurposeLogin,
		Proof:       serverProof,
	}); err != nil {
		t.Fatalf("VerifyCredentialChallenge(correct coordinates) error = %v", err)
	}
}

func TestDefaultChallengeMaterialDoesNotExposeProof(t *testing.T) {
	for _, kind := range []ChallengeKind{ChallengeKindCaptcha, ChallengeKindSlider} {
		t.Run(string(kind), func(t *testing.T) {
			material, err := defaultChallengeMaterial(kind)
			if err != nil {
				t.Fatalf("defaultChallengeMaterial(%q) error = %v", kind, err)
			}
			if material.Proof == "" || material.Prompt == "" {
				t.Fatalf("defaultChallengeMaterial(%q) = %+v, want proof and prompt for server use", kind, material)
			}
			for _, forbidden := range []string{"text", "proof", "answer", "targetX", "targetY", "targetOffset"} {
				if _, ok := material.Parameters[forbidden]; ok {
					t.Fatalf("defaultChallengeMaterial(%q) exposed %q: %+v", kind, forbidden, material.Parameters)
				}
			}
			encoded, err := json.Marshal(material.Parameters)
			if err != nil {
				t.Fatalf("marshal default material parameters: %v", err)
			}
			if strings.Contains(string(encoded), material.Proof) {
				t.Fatalf("defaultChallengeMaterial(%q) leaked proof %q in %s", kind, material.Proof, encoded)
			}
		})
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
