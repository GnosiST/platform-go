package credentialauth

import (
	"context"
	"testing"
	"time"

	"github.com/GnosiST/platform-go/internal/platform/storage"
)

func TestMemoryRepositoryAppliesChallengeTransitionsAtomically(t *testing.T) {
	assertRepositoryChallengeTransitions(t, NewMemoryRepository())
}

func TestGORMRepositoryAppliesChallengeTransitionsAtomically(t *testing.T) {
	ctx := context.Background()
	dsn := t.TempDir() + "/challenge-transitions.db?_journal_mode=WAL&_busy_timeout=5000"
	db, err := storage.OpenGORM(storage.Config{Driver: "sqlite", DSN: dsn})
	if err != nil {
		t.Fatalf("OpenGORM() error = %v", err)
	}
	repository, err := NewGORMRepository(ctx, db)
	if err != nil {
		t.Fatalf("NewGORMRepository() error = %v", err)
	}
	t.Cleanup(func() {
		if err := repository.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
	assertRepositoryChallengeTransitions(t, repository)
}

func assertRepositoryChallengeTransitions(t *testing.T, repository Repository) {
	t.Helper()
	ctx := context.Background()
	now := time.Date(2026, 7, 19, 10, 0, 0, 0, time.UTC)

	t.Run("credential failure consume and replay", func(t *testing.T) {
		const challengeID = "credential-transition"
		if err := repository.UpsertCredentialChallenge(ctx, CredentialChallenge{
			ChallengeID:           challengeID,
			Kind:                  ChallengeKindCaptcha,
			Purpose:               ChallengePurposeLogin,
			AnswerDigest:          "answer-digest",
			ExpiresAt:             now.Add(time.Minute),
			ClientFingerprintHash: "fingerprint-hash",
		}); err != nil {
			t.Fatalf("UpsertCredentialChallenge() error = %v", err)
		}
		if applied, err := repository.RecordCredentialChallengeFailure(ctx, challengeID, now, 3); err != nil || !applied {
			t.Fatalf("RecordCredentialChallengeFailure() = %v, %v; want applied", applied, err)
		}
		consumed := runConcurrentChallengeTransitions(t, 2, func() (bool, error) {
			return repository.ConsumeCredentialChallenge(ctx, challengeID, now)
		})
		if consumed != 1 {
			t.Fatalf("successful credential consumers = %d, want 1", consumed)
		}
		if applied, err := repository.RecordCredentialChallengeFailure(ctx, challengeID, now, 3); err != nil || applied {
			t.Fatalf("RecordCredentialChallengeFailure(consumed) = %v, %v; want no change", applied, err)
		}
		stored, ok, err := repository.CredentialChallenge(ctx, challengeID)
		if err != nil || !ok {
			t.Fatalf("CredentialChallenge() = %+v, %v, %v; want stored challenge", stored, ok, err)
		}
		if stored.Attempts != 1 || !stored.UsedAt.Equal(now) || stored.Status != StatusEnabled || stored.AnswerDigest != "answer-digest" || stored.ClientFingerprintHash != "fingerprint-hash" {
			t.Fatalf("stored credential challenge = %+v, want one failure followed by consumption without unrelated changes", stored)
		}
	})

	t.Run("credential concurrent failures", func(t *testing.T) {
		const challengeID = "credential-concurrent-failures"
		const failures = 8
		if err := repository.UpsertCredentialChallenge(ctx, CredentialChallenge{
			ChallengeID:  challengeID,
			Kind:         ChallengeKindCaptcha,
			Purpose:      ChallengePurposeLogin,
			AnswerDigest: "answer-digest",
			ExpiresAt:    now.Add(time.Minute),
		}); err != nil {
			t.Fatalf("UpsertCredentialChallenge() error = %v", err)
		}
		applied := runConcurrentChallengeTransitions(t, failures, func() (bool, error) {
			return repository.RecordCredentialChallengeFailure(ctx, challengeID, now, failures)
		})
		if applied != failures {
			t.Fatalf("applied credential failures = %d, want %d", applied, failures)
		}
		stored, ok, err := repository.CredentialChallenge(ctx, challengeID)
		if err != nil || !ok || stored.Attempts != failures || stored.Status != StatusDisabled || !stored.UsedAt.IsZero() {
			t.Fatalf("CredentialChallenge() = %+v, %v, %v; want exact attempts and disabled state", stored, ok, err)
		}
		if applied, err := repository.RecordCredentialChallengeFailure(ctx, challengeID, now, failures); err != nil || applied {
			t.Fatalf("RecordCredentialChallengeFailure(disabled) = %v, %v; want no change", applied, err)
		}
	})

	t.Run("sms otp failure consume and replay", func(t *testing.T) {
		const challengeID = "sms-otp-transition"
		if err := repository.UpsertSMSOTPChallenge(ctx, SMSOTPChallenge{
			ChallengeID: challengeID,
			PhoneHash:   "phone-hash",
			CodeDigest:  "code-digest",
			ExpiresAt:   now.Add(time.Minute),
			MessageID:   "message-id",
		}); err != nil {
			t.Fatalf("UpsertSMSOTPChallenge() error = %v", err)
		}
		if applied, err := repository.RecordSMSOTPChallengeFailure(ctx, challengeID, now, 3); err != nil || !applied {
			t.Fatalf("RecordSMSOTPChallengeFailure() = %v, %v; want applied", applied, err)
		}
		consumed := runConcurrentChallengeTransitions(t, 2, func() (bool, error) {
			return repository.ConsumeSMSOTPChallenge(ctx, challengeID, now)
		})
		if consumed != 1 {
			t.Fatalf("successful sms otp consumers = %d, want 1", consumed)
		}
		if applied, err := repository.RecordSMSOTPChallengeFailure(ctx, challengeID, now, 3); err != nil || applied {
			t.Fatalf("RecordSMSOTPChallengeFailure(consumed) = %v, %v; want no change", applied, err)
		}
		stored, ok, err := repository.SMSOTPChallenge(ctx, challengeID)
		if err != nil || !ok {
			t.Fatalf("SMSOTPChallenge() = %+v, %v, %v; want stored challenge", stored, ok, err)
		}
		if stored.Attempts != 1 || !stored.UsedAt.Equal(now) || stored.Status != StatusEnabled || stored.CodeDigest != "code-digest" || stored.MessageID != "message-id" {
			t.Fatalf("stored sms otp challenge = %+v, want one failure followed by consumption without unrelated changes", stored)
		}
	})

	t.Run("sms otp concurrent failures", func(t *testing.T) {
		const challengeID = "sms-otp-concurrent-failures"
		const failures = 8
		if err := repository.UpsertSMSOTPChallenge(ctx, SMSOTPChallenge{
			ChallengeID: challengeID,
			PhoneHash:   "phone-hash",
			CodeDigest:  "code-digest",
			ExpiresAt:   now.Add(time.Minute),
		}); err != nil {
			t.Fatalf("UpsertSMSOTPChallenge() error = %v", err)
		}
		applied := runConcurrentChallengeTransitions(t, failures, func() (bool, error) {
			return repository.RecordSMSOTPChallengeFailure(ctx, challengeID, now, failures)
		})
		if applied != failures {
			t.Fatalf("applied sms otp failures = %d, want %d", applied, failures)
		}
		stored, ok, err := repository.SMSOTPChallenge(ctx, challengeID)
		if err != nil || !ok || stored.Attempts != failures || stored.Status != StatusDisabled || !stored.UsedAt.IsZero() {
			t.Fatalf("SMSOTPChallenge() = %+v, %v, %v; want exact attempts and disabled state", stored, ok, err)
		}
		if applied, err := repository.RecordSMSOTPChallengeFailure(ctx, challengeID, now, failures); err != nil || applied {
			t.Fatalf("RecordSMSOTPChallengeFailure(disabled) = %v, %v; want no change", applied, err)
		}
	})

	t.Run("expired and disabled challenges block transitions", func(t *testing.T) {
		for _, test := range []struct {
			name      string
			expiresAt time.Time
			status    RecordStatus
		}{
			{name: "expired", expiresAt: now, status: StatusEnabled},
			{name: "disabled", expiresAt: now.Add(time.Minute), status: StatusDisabled},
		} {
			t.Run(test.name, func(t *testing.T) {
				credentialID := "credential-blocked-" + test.name
				if err := repository.UpsertCredentialChallenge(ctx, CredentialChallenge{
					ChallengeID:  credentialID,
					Kind:         ChallengeKindCaptcha,
					Purpose:      ChallengePurposeLogin,
					AnswerDigest: "answer-digest",
					ExpiresAt:    test.expiresAt,
					Status:       test.status,
				}); err != nil {
					t.Fatalf("UpsertCredentialChallenge() error = %v", err)
				}
				if consumed, err := repository.ConsumeCredentialChallenge(ctx, credentialID, now); err != nil || consumed {
					t.Fatalf("ConsumeCredentialChallenge(%s) = %v, %v; want no change", test.name, consumed, err)
				}
				if applied, err := repository.RecordCredentialChallengeFailure(ctx, credentialID, now, 3); err != nil || applied {
					t.Fatalf("RecordCredentialChallengeFailure(%s) = %v, %v; want no change", test.name, applied, err)
				}

				smsOTPID := "sms-otp-blocked-" + test.name
				if err := repository.UpsertSMSOTPChallenge(ctx, SMSOTPChallenge{
					ChallengeID: smsOTPID,
					PhoneHash:   "phone-hash",
					CodeDigest:  "code-digest",
					ExpiresAt:   test.expiresAt,
					Status:      test.status,
				}); err != nil {
					t.Fatalf("UpsertSMSOTPChallenge() error = %v", err)
				}
				if consumed, err := repository.ConsumeSMSOTPChallenge(ctx, smsOTPID, now); err != nil || consumed {
					t.Fatalf("ConsumeSMSOTPChallenge(%s) = %v, %v; want no change", test.name, consumed, err)
				}
				if applied, err := repository.RecordSMSOTPChallengeFailure(ctx, smsOTPID, now, 3); err != nil || applied {
					t.Fatalf("RecordSMSOTPChallengeFailure(%s) = %v, %v; want no change", test.name, applied, err)
				}
			})
		}
	})
}

func runConcurrentChallengeTransitions(t *testing.T, count int, transition func() (bool, error)) int {
	t.Helper()
	start := make(chan struct{})
	results := make(chan struct {
		applied bool
		err     error
	}, count)
	for range count {
		go func() {
			<-start
			applied, err := transition()
			results <- struct {
				applied bool
				err     error
			}{applied: applied, err: err}
		}()
	}
	close(start)
	applied := 0
	for range count {
		result := <-results
		if result.err != nil {
			t.Errorf("challenge transition error = %v", result.err)
		}
		if result.applied {
			applied++
		}
	}
	return applied
}
