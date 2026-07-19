package credentialauth

import (
	"context"
	"testing"
	"time"

	"github.com/GnosiST/platform-go/internal/platform/storage"
)

func TestGORMRepositoryPersistsCredentialAuthDedicatedStores(t *testing.T) {
	ctx := context.Background()
	db, err := storage.OpenGORM(storage.Config{Driver: "sqlite", DSN: t.TempDir() + "/credential-auth.db"})
	if err != nil {
		t.Fatalf("OpenGORM() error = %v", err)
	}
	repository, err := NewGORMRepository(ctx, db)
	if err != nil {
		t.Fatalf("NewGORMRepository() error = %v", err)
	}
	now := time.Date(2026, time.July, 19, 10, 0, 0, 0, time.UTC)
	principal := PrincipalRef{Type: PrincipalTypeAdmin, ID: "admin"}

	if err := repository.UpsertIdentifier(ctx, IdentifierRecord{
		Principal:        principal,
		IdentifierType:   IdentifierTypeUsername,
		IdentifierHash:   "identifier-hash",
		MaskedIdentifier: "a***n",
		VerifiedAt:       now,
		Status:           StatusEnabled,
	}); err != nil {
		t.Fatalf("UpsertIdentifier() error = %v", err)
	}
	if err := repository.UpsertPasswordCredential(ctx, PasswordCredential{
		Principal:         principal,
		PasswordHash:      "$argon2id$v=19$m=1,t=1,p=1$salt$hash",
		Algorithm:         PasswordAlgorithmArgon2id,
		ParamsVersion:     "argon2id-default",
		PasswordUpdatedAt: now,
		FailedAttempts:    2,
		LockedUntil:       now.Add(time.Minute),
		Status:            StatusEnabled,
	}); err != nil {
		t.Fatalf("UpsertPasswordCredential() error = %v", err)
	}
	if err := repository.UpsertCredentialChallenge(ctx, CredentialChallenge{
		ChallengeID:           "challenge-1",
		Kind:                  ChallengeKindSlider,
		Purpose:               ChallengePurposeLogin,
		AnswerDigest:          "answer-digest",
		ExpiresAt:             now.Add(time.Minute),
		Attempts:              1,
		ClientFingerprintHash: "fingerprint",
		Status:                StatusEnabled,
	}); err != nil {
		t.Fatalf("UpsertCredentialChallenge() error = %v", err)
	}
	if err := repository.UpsertSMSOTPChallenge(ctx, SMSOTPChallenge{
		ChallengeID: "otp-1",
		PhoneHash:   "phone-hash",
		CodeDigest:  "code-digest",
		ExpiresAt:   now.Add(time.Minute),
		Attempts:    1,
		MessageID:   "message-1",
		Status:      StatusEnabled,
	}); err != nil {
		t.Fatalf("UpsertSMSOTPChallenge() error = %v", err)
	}

	reopened, err := OpenGORMRepository(ctx, db)
	if err != nil {
		t.Fatalf("OpenGORMRepository() error = %v", err)
	}
	identifier, ok, err := reopened.LookupIdentifier(ctx, IdentifierTypeUsername, "identifier-hash")
	if err != nil || !ok {
		t.Fatalf("LookupIdentifier() = %+v/%t/%v, want persisted record", identifier, ok, err)
	}
	if identifier.Principal != principal || identifier.MaskedIdentifier != "a***n" {
		t.Fatalf("identifier = %+v, want persisted principal and mask", identifier)
	}
	password, ok, err := reopened.PasswordCredential(ctx, principal)
	if err != nil || !ok {
		t.Fatalf("PasswordCredential() = %+v/%t/%v, want persisted record", password, ok, err)
	}
	if password.FailedAttempts != 2 || password.LockedUntil.IsZero() {
		t.Fatalf("password credential = %+v, want attempts and lock state", password)
	}
	challenge, ok, err := reopened.CredentialChallenge(ctx, "challenge-1")
	if err != nil || !ok {
		t.Fatalf("CredentialChallenge() = %+v/%t/%v, want persisted record", challenge, ok, err)
	}
	if challenge.ClientFingerprintHash != "fingerprint" || challenge.Attempts != 1 {
		t.Fatalf("challenge = %+v, want fingerprint and attempts", challenge)
	}
	otp, ok, err := reopened.SMSOTPChallenge(ctx, "otp-1")
	if err != nil || !ok {
		t.Fatalf("SMSOTPChallenge() = %+v/%t/%v, want persisted record", otp, ok, err)
	}
	if otp.MessageID != "message-1" || otp.PhoneHash != "phone-hash" {
		t.Fatalf("sms otp = %+v, want message id and phone hash", otp)
	}
}

func TestOpenGORMRepositoryRequiresPreparedSchema(t *testing.T) {
	db, err := storage.OpenGORM(storage.Config{Driver: "sqlite", DSN: ":memory:"})
	if err != nil {
		t.Fatalf("OpenGORM() error = %v", err)
	}
	if _, err := OpenGORMRepository(context.Background(), db); err == nil {
		t.Fatal("OpenGORMRepository(unprepared) error = nil, want schema gate")
	}
	for _, table := range []string{authIdentifiersTable, passwordCredentialsTable, credentialChallengesTable, smsOTPChallengesTable} {
		if db.Migrator().HasTable(table) {
			t.Fatalf("OpenGORMRepository(unprepared) created %q", table)
		}
	}
}
