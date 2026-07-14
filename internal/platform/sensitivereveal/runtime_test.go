package sensitivereveal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *testClock) Add(duration time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(duration)
}

type runtimeHarness struct {
	runtime *Runtime
	store   Store
	clock   *testClock
	db      *gorm.DB
}

type storeFactory struct {
	name string
	open func(*testing.T, []Policy) runtimeHarness
}

func storeFactories() []storeFactory {
	return []storeFactory{
		{
			name: "memory",
			open: func(t *testing.T, policies []Policy) runtimeHarness {
				return newRuntimeHarness(t, NewMemoryStore(), nil, policies)
			},
		},
		{
			name: "gorm",
			open: func(t *testing.T, policies []Policy) runtimeHarness {
				dsn := fmt.Sprintf("file:sensitive-reveal-%d?mode=memory&cache=shared&_busy_timeout=5000", time.Now().UnixNano())
				db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
				if err != nil {
					t.Fatalf("gorm.Open() error = %v", err)
				}
				sqlDB, err := db.DB()
				if err != nil {
					t.Fatalf("db.DB() error = %v", err)
				}
				sqlDB.SetMaxOpenConns(8)
				t.Cleanup(func() { _ = sqlDB.Close() })
				store, err := NewGORMStore(context.Background(), db)
				if err != nil {
					t.Fatalf("NewGORMStore() error = %v", err)
				}
				return newRuntimeHarness(t, store, db, policies)
			},
		},
	}
}

func newRuntimeHarness(t *testing.T, store Store, db *gorm.DB, policies []Policy) runtimeHarness {
	t.Helper()
	clock := &testClock{now: time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)}
	runtime, err := NewRuntime(RuntimeOptions{
		Store:    store,
		Policies: policies,
		HashKey:  []byte("0123456789abcdef0123456789abcdef"),
		Now:      clock.Now,
	})
	if err != nil {
		t.Fatalf("NewRuntime() error = %v", err)
	}
	return runtimeHarness{runtime: runtime, store: store, clock: clock, db: db}
}

func anyOfPolicy() Policy {
	return Policy{
		ID:   "reveal-any",
		Mode: PolicyAnyOf,
		Factors: []FactorRule{
			{Factor: FactorOIDCReauthentication, MaxAttempts: 2},
			{Factor: FactorSMSOTP, MaxAttempts: 3},
		},
		PurposeCodes: []string{"support-case", "compliance-review"},
		ChallengeTTL: 2 * time.Minute,
		GrantTTL:     30 * time.Second,
	}
}

func allOfPolicy() Policy {
	policy := anyOfPolicy()
	policy.ID = "reveal-all"
	policy.Mode = PolicyAllOf
	return policy
}

func testScope() Scope {
	return Scope{
		Actor:         "admin-7",
		SessionDigest: "sha256:v1:session-7",
		Tenant:        "tenant-2",
		Resource:      "personnel",
		Record:        "employee-91",
		Field:         "phone",
		Purpose:       "support-case",
		Permission:    "personnel.sensitive.reveal",
	}
}

func TestRuntimePolicyModes(t *testing.T) {
	for _, factory := range storeFactories() {
		t.Run(factory.name, func(t *testing.T) {
			t.Run("anyOf issues after one factor", func(t *testing.T) {
				harness := factory.open(t, []Policy{anyOfPolicy()})
				challenge := beginChallenge(t, harness.runtime, "reveal-any", testScope())
				transaction := beginFactor(t, harness.runtime, challenge.ChallengeToken, FactorSMSOTP)
				result, err := harness.runtime.CompleteFactor(context.Background(), CompleteFactorRequest{
					ChallengeToken: challenge.ChallengeToken, TransactionToken: transaction.TransactionToken, Verified: true,
				})
				if err != nil {
					t.Fatalf("CompleteFactor() error = %v", err)
				}
				if !result.PolicySatisfied || result.GrantToken == "" {
					t.Fatalf("CompleteFactor() = %+v, want satisfied grant", result)
				}
			})

			t.Run("allOf waits for every factor", func(t *testing.T) {
				harness := factory.open(t, []Policy{allOfPolicy()})
				challenge := beginChallenge(t, harness.runtime, "reveal-all", testScope())
				oidc := beginFactor(t, harness.runtime, challenge.ChallengeToken, FactorOIDCReauthentication)
				sms := beginFactor(t, harness.runtime, challenge.ChallengeToken, FactorSMSOTP)
				first, err := harness.runtime.CompleteFactor(context.Background(), CompleteFactorRequest{
					ChallengeToken: challenge.ChallengeToken, TransactionToken: oidc.TransactionToken, Verified: true,
				})
				if err != nil {
					t.Fatalf("first CompleteFactor() error = %v", err)
				}
				if first.PolicySatisfied || first.GrantToken != "" {
					t.Fatalf("first CompleteFactor() = %+v, want unsatisfied", first)
				}
				second, err := harness.runtime.CompleteFactor(context.Background(), CompleteFactorRequest{
					ChallengeToken: challenge.ChallengeToken, TransactionToken: sms.TransactionToken, Verified: true,
				})
				if err != nil {
					t.Fatalf("second CompleteFactor() error = %v", err)
				}
				if !second.PolicySatisfied || second.GrantToken == "" {
					t.Fatalf("second CompleteFactor() = %+v, want satisfied grant", second)
				}
			})
		})
	}
}

func TestRuntimeFactorResultsExposeMatchedChallengeID(t *testing.T) {
	for _, factory := range storeFactories() {
		t.Run(factory.name, func(t *testing.T) {
			harness := factory.open(t, []Policy{allOfPolicy()})
			challenge := beginChallenge(t, harness.runtime, "reveal-all", testScope())
			_, err := harness.runtime.BeginFactor(context.Background(), BeginFactorRequest{
				ChallengeToken: challenge.ChallengeToken, ExpectedChallengeID: "wrong-challenge", Factor: FactorOIDCReauthentication,
			})
			if !errors.Is(err, ErrChallengeNotFound) {
				t.Fatalf("BeginFactor() mismatched challenge error = %v, want %v", err, ErrChallengeNotFound)
			}
			transaction, err := harness.runtime.BeginFactor(context.Background(), BeginFactorRequest{
				ChallengeToken: challenge.ChallengeToken, ExpectedChallengeID: challenge.ChallengeID, Factor: FactorOIDCReauthentication,
			})
			if err != nil {
				t.Fatalf("BeginFactor() error = %v", err)
			}
			if transaction.ChallengeID != challenge.ChallengeID {
				t.Fatalf("BeginFactor().ChallengeID = %q, want %q", transaction.ChallengeID, challenge.ChallengeID)
			}
			_, err = harness.runtime.CompleteFactor(context.Background(), CompleteFactorRequest{
				ChallengeToken: challenge.ChallengeToken, ExpectedChallengeID: "wrong-challenge", TransactionToken: transaction.TransactionToken, Verified: true,
			})
			if !errors.Is(err, ErrChallengeNotFound) {
				t.Fatalf("CompleteFactor() mismatched challenge error = %v, want %v", err, ErrChallengeNotFound)
			}
			completion, err := harness.runtime.CompleteFactor(context.Background(), CompleteFactorRequest{
				ChallengeToken: challenge.ChallengeToken, ExpectedChallengeID: challenge.ChallengeID, TransactionToken: transaction.TransactionToken, Verified: true,
			})
			if err != nil {
				t.Fatalf("CompleteFactor() error = %v", err)
			}
			if completion.ChallengeID != challenge.ChallengeID {
				t.Fatalf("CompleteFactor().ChallengeID = %q, want %q", completion.ChallengeID, challenge.ChallengeID)
			}
			if completion.PolicySatisfied {
				t.Fatalf("CompleteFactor() = %+v, want unsatisfied allOf result", completion)
			}
		})
	}
}

func TestRuntimeCancelledFactorCanBeStartedAgain(t *testing.T) {
	for _, factory := range storeFactories() {
		t.Run(factory.name, func(t *testing.T) {
			harness := factory.open(t, []Policy{anyOfPolicy()})
			challenge := beginChallenge(t, harness.runtime, "reveal-any", testScope())
			started := beginFactor(t, harness.runtime, challenge.ChallengeToken, FactorSMSOTP)
			if err := harness.runtime.CancelFactor(context.Background(), CancelFactorRequest{
				ChallengeToken: challenge.ChallengeToken, ExpectedChallengeID: challenge.ChallengeID,
				TransactionToken: started.TransactionToken, Reason: FactorCancelReasonDeliveryFailed,
			}); err != nil {
				t.Fatalf("CancelFactor() error = %v", err)
			}
			retried := beginFactor(t, harness.runtime, challenge.ChallengeToken, FactorSMSOTP)
			if retried.TransactionToken == started.TransactionToken {
				t.Fatalf("retried transaction token reused cancelled token %q", retried.TransactionToken)
			}
			if _, err := harness.runtime.CompleteFactor(context.Background(), CompleteFactorRequest{
				ChallengeToken: challenge.ChallengeToken, TransactionToken: started.TransactionToken, Verified: true,
			}); !errors.Is(err, ErrFactorTransactionNotFound) {
				t.Fatalf("CompleteFactor(cancelled) error = %v, want %v", err, ErrFactorTransactionNotFound)
			}
			events, err := harness.runtime.AuditEvents(context.Background())
			if err != nil {
				t.Fatalf("AuditEvents() error = %v", err)
			}
			found := false
			for _, event := range events {
				if event.Type == AuditFactorFailed && event.Outcome == "cancelled" && event.Reason == FactorCancelReasonDeliveryFailed {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("audit events = %+v, want factor cancellation", events)
			}
		})
	}
}

func TestRuntimeExpirationAndReplay(t *testing.T) {
	for _, factory := range storeFactories() {
		t.Run(factory.name, func(t *testing.T) {
			t.Run("challenge expires", func(t *testing.T) {
				harness := factory.open(t, []Policy{anyOfPolicy()})
				challenge := beginChallenge(t, harness.runtime, "reveal-any", testScope())
				harness.clock.Add(2 * time.Minute)
				_, err := harness.runtime.BeginFactor(context.Background(), BeginFactorRequest{ChallengeToken: challenge.ChallengeToken, Factor: FactorSMSOTP})
				if !errors.Is(err, ErrChallengeExpired) {
					t.Fatalf("BeginFactor() error = %v, want %v", err, ErrChallengeExpired)
				}
			})

			t.Run("grant expires", func(t *testing.T) {
				harness := factory.open(t, []Policy{anyOfPolicy()})
				grantToken := issueGrant(t, harness.runtime, "reveal-any", testScope(), FactorSMSOTP)
				harness.clock.Add(30 * time.Second)
				_, err := harness.runtime.ConsumeGrant(context.Background(), ConsumeGrantRequest{GrantToken: grantToken, Scope: testScope()})
				if !errors.Is(err, ErrGrantExpired) {
					t.Fatalf("ConsumeGrant() error = %v, want %v", err, ErrGrantExpired)
				}
			})

			t.Run("grant cannot be replayed", func(t *testing.T) {
				harness := factory.open(t, []Policy{anyOfPolicy()})
				grantToken := issueGrant(t, harness.runtime, "reveal-any", testScope(), FactorSMSOTP)
				if _, err := harness.runtime.ConsumeGrant(context.Background(), ConsumeGrantRequest{GrantToken: grantToken, Scope: testScope()}); err != nil {
					t.Fatalf("first ConsumeGrant() error = %v", err)
				}
				_, err := harness.runtime.ConsumeGrant(context.Background(), ConsumeGrantRequest{GrantToken: grantToken, Scope: testScope()})
				if !errors.Is(err, ErrGrantConsumed) {
					t.Fatalf("second ConsumeGrant() error = %v, want %v", err, ErrGrantConsumed)
				}
			})
		})
	}
}

func TestRuntimeGrantRequiresExactScope(t *testing.T) {
	mutations := map[string]func(*Scope){
		"actor":          func(scope *Scope) { scope.Actor = "other-admin" },
		"session digest": func(scope *Scope) { scope.SessionDigest = "sha256:v1:other-session" },
		"tenant":         func(scope *Scope) { scope.Tenant = "other-tenant" },
		"resource":       func(scope *Scope) { scope.Resource = "other-resource" },
		"record":         func(scope *Scope) { scope.Record = "other-record" },
		"field":          func(scope *Scope) { scope.Field = "email" },
		"purpose":        func(scope *Scope) { scope.Purpose = "compliance-review" },
		"permission":     func(scope *Scope) { scope.Permission = "other.permission" },
	}
	for _, factory := range storeFactories() {
		t.Run(factory.name, func(t *testing.T) {
			for name, mutate := range mutations {
				t.Run(name, func(t *testing.T) {
					harness := factory.open(t, []Policy{anyOfPolicy()})
					grantToken := issueGrant(t, harness.runtime, "reveal-any", testScope(), FactorOIDCReauthentication)
					wrongScope := testScope()
					mutate(&wrongScope)
					_, err := harness.runtime.ConsumeGrant(context.Background(), ConsumeGrantRequest{GrantToken: grantToken, Scope: wrongScope})
					if !errors.Is(err, ErrScopeMismatch) {
						t.Fatalf("ConsumeGrant() error = %v, want %v", err, ErrScopeMismatch)
					}
					if _, err := harness.runtime.ConsumeGrant(context.Background(), ConsumeGrantRequest{GrantToken: grantToken, Scope: testScope()}); err != nil {
						t.Fatalf("matching ConsumeGrant() after mismatch error = %v", err)
					}
				})
			}
		})
	}
}

func TestRuntimeConcurrentGrantConsumption(t *testing.T) {
	for _, factory := range storeFactories() {
		t.Run(factory.name, func(t *testing.T) {
			harness := factory.open(t, []Policy{anyOfPolicy()})
			grantToken := issueGrant(t, harness.runtime, "reveal-any", testScope(), FactorSMSOTP)
			start := make(chan struct{})
			errorsChannel := make(chan error, 2)
			for index := 0; index < 2; index++ {
				go func() {
					<-start
					_, err := harness.runtime.ConsumeGrant(context.Background(), ConsumeGrantRequest{GrantToken: grantToken, Scope: testScope()})
					errorsChannel <- err
				}()
			}
			close(start)
			var allowed, consumed int
			for index := 0; index < 2; index++ {
				err := <-errorsChannel
				switch {
				case err == nil:
					allowed++
				case errors.Is(err, ErrGrantConsumed):
					consumed++
				default:
					t.Fatalf("ConsumeGrant() concurrent error = %v", err)
				}
			}
			if allowed != 1 || consumed != 1 {
				t.Fatalf("concurrent outcomes allowed=%d consumed=%d, want 1/1", allowed, consumed)
			}
			events, err := harness.runtime.AuditEvents(context.Background())
			if err != nil {
				t.Fatalf("AuditEvents() error = %v", err)
			}
			if countAudit(events, AuditRevealAllowed) != 1 {
				t.Fatalf("allowed audit count = %d, want 1", countAudit(events, AuditRevealAllowed))
			}
		})
	}
}

func TestRuntimeSMSAttemptLockIsAtomic(t *testing.T) {
	policy := Policy{
		ID:           "sms-only",
		Mode:         PolicyAnyOf,
		Factors:      []FactorRule{{Factor: FactorSMSOTP, MaxAttempts: 3}},
		PurposeCodes: []string{"support-case"},
		ChallengeTTL: time.Minute,
		GrantTTL:     30 * time.Second,
	}
	for _, factory := range storeFactories() {
		t.Run(factory.name, func(t *testing.T) {
			harness := factory.open(t, []Policy{policy})
			challenge := beginChallenge(t, harness.runtime, "sms-only", testScope())
			transaction, err := harness.runtime.BeginFactor(context.Background(), BeginFactorRequest{
				ChallengeToken: challenge.ChallengeToken, Factor: FactorSMSOTP, VerificationSecret: "593104",
			})
			if err != nil {
				t.Fatalf("BeginFactor() error = %v", err)
			}
			start := make(chan struct{})
			results := make(chan error, 10)
			for index := 0; index < 10; index++ {
				go func() {
					<-start
					_, err := harness.runtime.CompleteFactor(context.Background(), CompleteFactorRequest{
						ChallengeToken: challenge.ChallengeToken, TransactionToken: transaction.TransactionToken,
						VerificationProof: "wrong-proof", Verified: true,
					})
					results <- err
				}()
			}
			close(start)
			for index := 0; index < 10; index++ {
				err := <-results
				if !errors.Is(err, ErrVerificationFailed) && !errors.Is(err, ErrFactorLocked) {
					t.Fatalf("CompleteFactor() error = %v, want verification failure or lock", err)
				}
			}
			_, err = harness.runtime.CompleteFactor(context.Background(), CompleteFactorRequest{
				ChallengeToken: challenge.ChallengeToken, TransactionToken: transaction.TransactionToken,
				VerificationProof: "593104", Verified: true,
			})
			if !errors.Is(err, ErrFactorLocked) {
				t.Fatalf("CompleteFactor() after failures error = %v, want %v", err, ErrFactorLocked)
			}
			events, err := harness.runtime.AuditEvents(context.Background())
			if err != nil {
				t.Fatalf("AuditEvents() error = %v", err)
			}
			if countAudit(events, AuditFactorFailed) != 3 {
				t.Fatalf("factor failure audit count = %d, want 3", countAudit(events, AuditFactorFailed))
			}
			if events[len(events)-1].Type != AuditFactorFailed || events[len(events)-1].Reason != "attempt_limit_reached" {
				t.Fatalf("last audit = %+v, want attempt limit", events[len(events)-1])
			}
		})
	}
}

func TestRuntimeAuditContainsNoRawSecretsOrPlaintext(t *testing.T) {
	const plaintext = "13800138000"
	for _, factory := range storeFactories() {
		t.Run(factory.name, func(t *testing.T) {
			harness := factory.open(t, []Policy{anyOfPolicy()})
			challenge := beginChallenge(t, harness.runtime, "reveal-any", testScope())
			transaction := beginFactor(t, harness.runtime, challenge.ChallengeToken, FactorSMSOTP)
			completion, err := harness.runtime.CompleteFactor(context.Background(), CompleteFactorRequest{
				ChallengeToken: challenge.ChallengeToken, TransactionToken: transaction.TransactionToken, Verified: true,
			})
			if err != nil {
				t.Fatalf("CompleteFactor() error = %v", err)
			}
			if _, err := harness.runtime.ConsumeGrant(context.Background(), ConsumeGrantRequest{GrantToken: completion.GrantToken, Scope: testScope()}); err != nil {
				t.Fatalf("ConsumeGrant() error = %v", err)
			}
			events, err := harness.runtime.AuditEvents(context.Background())
			if err != nil {
				t.Fatalf("AuditEvents() error = %v", err)
			}
			encoded, err := json.Marshal(events)
			if err != nil {
				t.Fatalf("json.Marshal(audit) error = %v", err)
			}
			serialized := string(encoded)
			for _, secret := range []string{plaintext, challenge.ChallengeToken, transaction.TransactionToken, completion.GrantToken} {
				if strings.Contains(serialized, secret) {
					t.Fatalf("audit contains raw secret %q: %s", secret, serialized)
				}
			}
			if strings.Contains(serialized, `"value"`) || strings.Contains(serialized, `"plaintext"`) {
				t.Fatalf("audit exposes plaintext-bearing property: %s", serialized)
			}
		})
	}
}

func TestRuntimeVerificationSecretCannotBeBypassed(t *testing.T) {
	const (
		verificationSecret = "481927"
		wrongProof         = "000000"
	)
	policy := Policy{
		ID:           "otp-proof",
		Mode:         PolicyAnyOf,
		Factors:      []FactorRule{{Factor: FactorSMSOTP, MaxAttempts: 3}},
		PurposeCodes: []string{"support-case"},
		ChallengeTTL: time.Minute,
		GrantTTL:     30 * time.Second,
	}
	for _, factory := range storeFactories() {
		t.Run(factory.name, func(t *testing.T) {
			harness := factory.open(t, []Policy{policy})
			challenge := beginChallenge(t, harness.runtime, "otp-proof", testScope())
			transaction, err := harness.runtime.BeginFactor(context.Background(), BeginFactorRequest{
				ChallengeToken:     challenge.ChallengeToken,
				Factor:             FactorSMSOTP,
				VerificationSecret: verificationSecret,
			})
			if err != nil {
				t.Fatalf("BeginFactor() error = %v", err)
			}
			_, err = harness.runtime.CompleteFactor(context.Background(), CompleteFactorRequest{
				ChallengeToken:    challenge.ChallengeToken,
				TransactionToken:  transaction.TransactionToken,
				VerificationProof: wrongProof,
				Verified:          true,
			})
			if !errors.Is(err, ErrVerificationFailed) {
				t.Fatalf("CompleteFactor(wrong proof, Verified=true) error = %v, want %v", err, ErrVerificationFailed)
			}
			result, err := harness.runtime.CompleteFactor(context.Background(), CompleteFactorRequest{
				ChallengeToken:    challenge.ChallengeToken,
				TransactionToken:  transaction.TransactionToken,
				VerificationProof: verificationSecret,
				Verified:          false,
			})
			if err != nil {
				t.Fatalf("CompleteFactor(correct proof) error = %v", err)
			}
			if result.GrantToken == "" {
				t.Fatalf("CompleteFactor(correct proof) = %+v, want grant", result)
			}
			assertVerificationSecretNotStored(t, harness, verificationSecret, wrongProof)
		})
	}
}

func TestRuntimeVerificationProofAttemptLimit(t *testing.T) {
	const verificationSecret = "734821"
	policy := Policy{
		ID:           "otp-lock",
		Mode:         PolicyAnyOf,
		Factors:      []FactorRule{{Factor: FactorSMSOTP, MaxAttempts: 2}},
		PurposeCodes: []string{"support-case"},
		ChallengeTTL: time.Minute,
		GrantTTL:     30 * time.Second,
	}
	for _, factory := range storeFactories() {
		t.Run(factory.name, func(t *testing.T) {
			harness := factory.open(t, []Policy{policy})
			challenge := beginChallenge(t, harness.runtime, "otp-lock", testScope())
			transaction, err := harness.runtime.BeginFactor(context.Background(), BeginFactorRequest{
				ChallengeToken: challenge.ChallengeToken, Factor: FactorSMSOTP, VerificationSecret: verificationSecret,
			})
			if err != nil {
				t.Fatalf("BeginFactor() error = %v", err)
			}
			for attempt, expected := range []error{ErrVerificationFailed, ErrFactorLocked} {
				_, err := harness.runtime.CompleteFactor(context.Background(), CompleteFactorRequest{
					ChallengeToken: challenge.ChallengeToken, TransactionToken: transaction.TransactionToken, VerificationProof: "wrong-proof",
				})
				if !errors.Is(err, expected) {
					t.Fatalf("attempt %d error = %v, want %v", attempt+1, err, expected)
				}
			}
			_, err = harness.runtime.CompleteFactor(context.Background(), CompleteFactorRequest{
				ChallengeToken: challenge.ChallengeToken, TransactionToken: transaction.TransactionToken, VerificationProof: verificationSecret,
			})
			if !errors.Is(err, ErrFactorLocked) {
				t.Fatalf("correct proof after lock error = %v, want %v", err, ErrFactorLocked)
			}
			events, err := harness.runtime.AuditEvents(context.Background())
			if err != nil {
				t.Fatalf("AuditEvents() error = %v", err)
			}
			if countAudit(events, AuditFactorFailed) != 2 {
				t.Fatalf("factor failure audit count = %d, want 2", countAudit(events, AuditFactorFailed))
			}
		})
	}
}

func TestGORMVerificationProofWorksAcrossRuntimeInstances(t *testing.T) {
	const verificationSecret = "902614"
	policy := Policy{
		ID:           "multi-instance-otp",
		Mode:         PolicyAnyOf,
		Factors:      []FactorRule{{Factor: FactorSMSOTP, MaxAttempts: 3}},
		PurposeCodes: []string{"support-case"},
		ChallengeTTL: time.Minute,
		GrantTTL:     30 * time.Second,
	}
	first := storeFactories()[1].open(t, []Policy{policy})
	challenge := beginChallenge(t, first.runtime, policy.ID, testScope())
	transaction, err := first.runtime.BeginFactor(context.Background(), BeginFactorRequest{
		ChallengeToken: challenge.ChallengeToken, Factor: FactorSMSOTP, VerificationSecret: verificationSecret,
	})
	if err != nil {
		t.Fatalf("first runtime BeginFactor() error = %v", err)
	}
	secondStore, err := NewGORMStore(context.Background(), first.db)
	if err != nil {
		t.Fatalf("second NewGORMStore() error = %v", err)
	}
	secondRuntime, err := NewRuntime(RuntimeOptions{
		Store: secondStore, Policies: []Policy{policy}, HashKey: []byte("0123456789abcdef0123456789abcdef"), Now: first.clock.Now,
	})
	if err != nil {
		t.Fatalf("second NewRuntime() error = %v", err)
	}
	result, err := secondRuntime.CompleteFactor(context.Background(), CompleteFactorRequest{
		ChallengeToken: challenge.ChallengeToken, TransactionToken: transaction.TransactionToken, VerificationProof: verificationSecret,
	})
	if err != nil {
		t.Fatalf("second runtime CompleteFactor() error = %v", err)
	}
	if result.GrantToken == "" {
		t.Fatalf("second runtime CompleteFactor() = %+v, want grant", result)
	}
}

func TestRuntimeTrustedFactorWithoutSecretStillUsesServerVerdict(t *testing.T) {
	for _, factory := range storeFactories() {
		t.Run(factory.name, func(t *testing.T) {
			harness := factory.open(t, []Policy{anyOfPolicy()})
			challenge := beginChallenge(t, harness.runtime, "reveal-any", testScope())
			transaction := beginFactor(t, harness.runtime, challenge.ChallengeToken, FactorOIDCReauthentication)
			result, err := harness.runtime.CompleteFactor(context.Background(), CompleteFactorRequest{
				ChallengeToken: challenge.ChallengeToken, TransactionToken: transaction.TransactionToken, Verified: true,
			})
			if err != nil {
				t.Fatalf("CompleteFactor() error = %v", err)
			}
			if result.GrantToken == "" {
				t.Fatalf("CompleteFactor() = %+v, want grant", result)
			}
		})
	}
}

func TestRuntimeRecordsTerminalRevealResult(t *testing.T) {
	for _, factory := range storeFactories() {
		t.Run(factory.name, func(t *testing.T) {
			t.Run("succeeded with exact scope", func(t *testing.T) {
				harness := factory.open(t, []Policy{anyOfPolicy()})
				grantToken := issueGrant(t, harness.runtime, "reveal-any", testScope(), FactorOIDCReauthentication)
				consumed, err := harness.runtime.ConsumeGrant(context.Background(), ConsumeGrantRequest{GrantToken: grantToken, Scope: testScope()})
				if err != nil {
					t.Fatalf("ConsumeGrant() error = %v", err)
				}
				wrongScope := testScope()
				wrongScope.Record = "other-record"
				if err := harness.runtime.RecordRevealResult(context.Background(), consumed.GrantID, wrongScope, true, RevealReasonCompleted); !errors.Is(err, ErrScopeMismatch) {
					t.Fatalf("RecordRevealResult(wrong scope) error = %v, want %v", err, ErrScopeMismatch)
				}
				if err := harness.runtime.RecordRevealResult(context.Background(), consumed.GrantID, testScope(), true, RevealReasonCompleted); err != nil {
					t.Fatalf("RecordRevealResult(success) error = %v", err)
				}
				if err := harness.runtime.RecordRevealResult(context.Background(), consumed.GrantID, testScope(), true, RevealReasonCompleted); !errors.Is(err, ErrRevealResultRecorded) {
					t.Fatalf("duplicate RecordRevealResult() error = %v, want %v", err, ErrRevealResultRecorded)
				}
				events, err := harness.runtime.AuditEvents(context.Background())
				if err != nil {
					t.Fatalf("AuditEvents() error = %v", err)
				}
				last := events[len(events)-1]
				if last.Type != AuditRevealSucceeded || last.Outcome != "succeeded" || last.Reason != RevealReasonCompleted || !last.Scope.equal(testScope()) {
					t.Fatalf("terminal success audit = %+v", last)
				}
				if countAudit(events, AuditRevealAllowed) != 1 {
					t.Fatalf("reveal.allowed count = %d, want 1", countAudit(events, AuditRevealAllowed))
				}
			})

			t.Run("failed with normalized reason", func(t *testing.T) {
				harness := factory.open(t, []Policy{anyOfPolicy()})
				grantToken := issueGrant(t, harness.runtime, "reveal-any", testScope(), FactorOIDCReauthentication)
				consumed, err := harness.runtime.ConsumeGrant(context.Background(), ConsumeGrantRequest{GrantToken: grantToken, Scope: testScope()})
				if err != nil {
					t.Fatalf("ConsumeGrant() error = %v", err)
				}
				if err := harness.runtime.RecordRevealResult(context.Background(), consumed.GrantID, testScope(), false, RevealReasonDecryptionFailed); err != nil {
					t.Fatalf("RecordRevealResult(failure) error = %v", err)
				}
				events, err := harness.runtime.AuditEvents(context.Background())
				if err != nil {
					t.Fatalf("AuditEvents() error = %v", err)
				}
				last := events[len(events)-1]
				if last.Type != AuditRevealFailed || last.Outcome != "failed" || last.Reason != RevealReasonDecryptionFailed {
					t.Fatalf("terminal failure audit = %+v", last)
				}
				serialized, err := json.Marshal(events)
				if err != nil {
					t.Fatalf("json.Marshal(audit) error = %v", err)
				}
				if strings.Contains(string(serialized), grantToken) {
					t.Fatalf("terminal audit contains grant token: %s", serialized)
				}
			})

			t.Run("rejects arbitrary reason text", func(t *testing.T) {
				harness := factory.open(t, []Policy{anyOfPolicy()})
				grantToken := issueGrant(t, harness.runtime, "reveal-any", testScope(), FactorOIDCReauthentication)
				consumed, err := harness.runtime.ConsumeGrant(context.Background(), ConsumeGrantRequest{GrantToken: grantToken, Scope: testScope()})
				if err != nil {
					t.Fatalf("ConsumeGrant() error = %v", err)
				}
				before, err := harness.runtime.AuditEvents(context.Background())
				if err != nil {
					t.Fatalf("AuditEvents(before) error = %v", err)
				}
				err = harness.runtime.RecordRevealResult(context.Background(), consumed.GrantID, testScope(), false, "decrypt failed for 13800138000")
				if !errors.Is(err, ErrInvalidRevealReason) {
					t.Fatalf("RecordRevealResult(arbitrary reason) error = %v, want %v", err, ErrInvalidRevealReason)
				}
				after, err := harness.runtime.AuditEvents(context.Background())
				if err != nil {
					t.Fatalf("AuditEvents(after) error = %v", err)
				}
				if len(after) != len(before) {
					t.Fatalf("invalid reason appended audit: before=%d after=%d", len(before), len(after))
				}
			})
		})
	}
}

func TestTrustedVerificationInputsCannotBeJSONBound(t *testing.T) {
	var begin BeginFactorRequest
	if err := json.Unmarshal([]byte(`{"ChallengeToken":"challenge","Factor":"sms-otp","VerificationSecret":"123456"}`), &begin); err != nil {
		t.Fatalf("json.Unmarshal(begin) error = %v", err)
	}
	if begin.VerificationSecret != "" {
		t.Fatalf("JSON bound VerificationSecret = %q, want empty", begin.VerificationSecret)
	}
	var complete CompleteFactorRequest
	if err := json.Unmarshal([]byte(`{"ChallengeToken":"challenge","TransactionToken":"transaction","VerificationProof":"123456","Verified":true}`), &complete); err != nil {
		t.Fatalf("json.Unmarshal(complete) error = %v", err)
	}
	if complete.Verified {
		t.Fatal("JSON bound trusted Verified=true")
	}
	if complete.VerificationProof != "123456" {
		t.Fatalf("JSON bound VerificationProof = %q, want client proof", complete.VerificationProof)
	}
}

func TestGORMStoreUsesIndependentDigestOnlyTables(t *testing.T) {
	harness := storeFactories()[1].open(t, []Policy{anyOfPolicy()})
	for _, table := range []string{challengesTable, factorTransactionsTable, grantsTable, auditEventsTable} {
		if !harness.db.Migrator().HasTable(table) {
			t.Fatalf("AutoMigrate did not create %q", table)
		}
		if strings.Contains(table, "admin_resource") {
			t.Fatalf("sensitive reveal table %q overlaps admin resource storage", table)
		}
	}
	challenge := beginChallenge(t, harness.runtime, "reveal-any", testScope())
	transaction := beginFactor(t, harness.runtime, challenge.ChallengeToken, FactorSMSOTP)
	completion, err := harness.runtime.CompleteFactor(context.Background(), CompleteFactorRequest{
		ChallengeToken: challenge.ChallengeToken, TransactionToken: transaction.TransactionToken, Verified: true,
	})
	if err != nil {
		t.Fatalf("CompleteFactor() error = %v", err)
	}
	var challengeModel gormChallenge
	if err := harness.db.First(&challengeModel).Error; err != nil {
		t.Fatalf("load challenge error = %v", err)
	}
	var transactionModel gormFactorTransaction
	if err := harness.db.First(&transactionModel).Error; err != nil {
		t.Fatalf("load transaction error = %v", err)
	}
	var grantModel gormGrant
	if err := harness.db.First(&grantModel).Error; err != nil {
		t.Fatalf("load grant error = %v", err)
	}
	for name, pair := range map[string][2]string{
		"challenge":   {challengeModel.TokenDigest, challenge.ChallengeToken},
		"transaction": {transactionModel.TransactionDigest, transaction.TransactionToken},
		"grant":       {grantModel.TokenDigest, completion.GrantToken},
	} {
		if pair[0] == pair[1] || !strings.HasPrefix(pair[0], digestPrefix) {
			t.Fatalf("%s stored value %q is not a digest", name, pair[0])
		}
	}
	for _, model := range []interface{}{&gormChallenge{}, &gormFactorTransaction{}, &gormGrant{}} {
		columns, err := harness.db.Migrator().ColumnTypes(model)
		if err != nil {
			t.Fatalf("ColumnTypes(%T) error = %v", model, err)
		}
		for _, column := range columns {
			name := strings.ToLower(column.Name())
			if name == "token" || name == "transaction_token" || name == "raw_token" ||
				name == "verification_secret" || name == "verification_proof" {
				t.Fatalf("%T contains raw secret column %q", model, name)
			}
		}
	}
}

func TestGORMFactorCompletionRollsBackWhenGrantAuditFails(t *testing.T) {
	harness := storeFactories()[1].open(t, []Policy{anyOfPolicy()})
	challenge := beginChallenge(t, harness.runtime, "reveal-any", testScope())
	transaction := beginFactor(t, harness.runtime, challenge.ChallengeToken, FactorSMSOTP)
	callbackName := "test:reject-grant-issued-audit"
	if err := harness.db.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Table != auditEventsTable {
			return
		}
		if event, ok := tx.Statement.Dest.(*gormAuditEvent); ok && event.Type == string(AuditGrantIssued) {
			tx.AddError(errors.New("grant audit unavailable"))
		}
	}); err != nil {
		t.Fatalf("register callback error = %v", err)
	}
	_, err := harness.runtime.CompleteFactor(context.Background(), CompleteFactorRequest{
		ChallengeToken: challenge.ChallengeToken, TransactionToken: transaction.TransactionToken, Verified: true,
	})
	if err == nil || !strings.Contains(err.Error(), "grant audit unavailable") {
		t.Fatalf("CompleteFactor() error = %v, want audit failure", err)
	}
	if err := harness.db.Callback().Create().Remove(callbackName); err != nil {
		t.Fatalf("remove callback error = %v", err)
	}
	result, err := harness.runtime.CompleteFactor(context.Background(), CompleteFactorRequest{
		ChallengeToken: challenge.ChallengeToken, TransactionToken: transaction.TransactionToken, Verified: true,
	})
	if err != nil {
		t.Fatalf("CompleteFactor() after rollback error = %v", err)
	}
	if result.GrantToken == "" {
		t.Fatalf("CompleteFactor() after rollback = %+v, want grant", result)
	}
}

func TestGORMGrantConsumptionRollsBackWhenAllowedAuditFails(t *testing.T) {
	harness := storeFactories()[1].open(t, []Policy{anyOfPolicy()})
	grantToken := issueGrant(t, harness.runtime, "reveal-any", testScope(), FactorSMSOTP)
	callbackName := "test:reject-reveal-allowed-audit"
	if err := harness.db.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Table != auditEventsTable {
			return
		}
		if event, ok := tx.Statement.Dest.(*gormAuditEvent); ok && event.Type == string(AuditRevealAllowed) {
			tx.AddError(errors.New("allowed audit unavailable"))
		}
	}); err != nil {
		t.Fatalf("register callback error = %v", err)
	}
	_, err := harness.runtime.ConsumeGrant(context.Background(), ConsumeGrantRequest{GrantToken: grantToken, Scope: testScope()})
	if err == nil || !strings.Contains(err.Error(), "allowed audit unavailable") {
		t.Fatalf("ConsumeGrant() error = %v, want audit failure", err)
	}
	if err := harness.db.Callback().Create().Remove(callbackName); err != nil {
		t.Fatalf("remove callback error = %v", err)
	}
	if _, err := harness.runtime.ConsumeGrant(context.Background(), ConsumeGrantRequest{GrantToken: grantToken, Scope: testScope()}); err != nil {
		t.Fatalf("ConsumeGrant() after rollback error = %v", err)
	}
}

func TestRuntimeRejectsUnapprovedPurpose(t *testing.T) {
	harness := newRuntimeHarness(t, NewMemoryStore(), nil, []Policy{anyOfPolicy()})
	scope := testScope()
	scope.Purpose = "curiosity"
	_, err := harness.runtime.BeginChallenge(context.Background(), BeginChallengeRequest{PolicyID: "reveal-any", Scope: scope})
	if !errors.Is(err, ErrPurposeNotAllowed) {
		t.Fatalf("BeginChallenge() error = %v, want %v", err, ErrPurposeNotAllowed)
	}
}

func TestRuntimeDoesNotIssueMultipleGrantsForAnyOf(t *testing.T) {
	for _, factory := range storeFactories() {
		t.Run(factory.name, func(t *testing.T) {
			harness := factory.open(t, []Policy{anyOfPolicy()})
			challenge := beginChallenge(t, harness.runtime, "reveal-any", testScope())
			oidc := beginFactor(t, harness.runtime, challenge.ChallengeToken, FactorOIDCReauthentication)
			sms := beginFactor(t, harness.runtime, challenge.ChallengeToken, FactorSMSOTP)
			start := make(chan struct{})
			results := make(chan CompleteFactorResult, 2)
			errorsChannel := make(chan error, 2)
			for _, token := range []string{oidc.TransactionToken, sms.TransactionToken} {
				token := token
				go func() {
					<-start
					result, err := harness.runtime.CompleteFactor(context.Background(), CompleteFactorRequest{
						ChallengeToken: challenge.ChallengeToken, TransactionToken: token, Verified: true,
					})
					results <- result
					errorsChannel <- err
				}()
			}
			close(start)
			var grants atomic.Int32
			for index := 0; index < 2; index++ {
				result := <-results
				err := <-errorsChannel
				if err != nil && !errors.Is(err, ErrChallengeClosed) {
					t.Fatalf("CompleteFactor() error = %v", err)
				}
				if result.GrantToken != "" {
					grants.Add(1)
				}
			}
			if grants.Load() != 1 {
				t.Fatalf("issued grant count = %d, want 1", grants.Load())
			}
		})
	}
}

func beginChallenge(t *testing.T, runtime *Runtime, policyID string, scope Scope) BeginChallengeResult {
	t.Helper()
	result, err := runtime.BeginChallenge(context.Background(), BeginChallengeRequest{PolicyID: policyID, Scope: scope})
	if err != nil {
		t.Fatalf("BeginChallenge() error = %v", err)
	}
	if result.ChallengeToken == "" || result.ChallengeID == "" || result.ExpiresAt.IsZero() {
		t.Fatalf("BeginChallenge() = %+v, want complete result", result)
	}
	return result
}

func beginFactor(t *testing.T, runtime *Runtime, challengeToken string, factor Factor) BeginFactorResult {
	t.Helper()
	result, err := runtime.BeginFactor(context.Background(), BeginFactorRequest{ChallengeToken: challengeToken, Factor: factor})
	if err != nil {
		t.Fatalf("BeginFactor() error = %v", err)
	}
	if result.TransactionToken == "" || result.ExpiresAt.IsZero() {
		t.Fatalf("BeginFactor() = %+v, want complete result", result)
	}
	return result
}

func issueGrant(t *testing.T, runtime *Runtime, policyID string, scope Scope, factor Factor) string {
	t.Helper()
	challenge := beginChallenge(t, runtime, policyID, scope)
	transaction := beginFactor(t, runtime, challenge.ChallengeToken, factor)
	result, err := runtime.CompleteFactor(context.Background(), CompleteFactorRequest{
		ChallengeToken: challenge.ChallengeToken, TransactionToken: transaction.TransactionToken, Verified: true,
	})
	if err != nil {
		t.Fatalf("CompleteFactor() error = %v", err)
	}
	if result.GrantToken == "" {
		t.Fatalf("CompleteFactor() = %+v, want grant", result)
	}
	return result.GrantToken
}

func countAudit(events []AuditEvent, eventType AuditEventType) int {
	count := 0
	for _, event := range events {
		if event.Type == eventType {
			count++
		}
	}
	return count
}

func assertVerificationSecretNotStored(t *testing.T, harness runtimeHarness, secrets ...string) {
	t.Helper()
	var stored string
	if memory, ok := harness.store.(*MemoryStore); ok {
		memory.mu.Lock()
		for _, transaction := range memory.transactions {
			stored = transaction.VerificationDigest
			break
		}
		memory.mu.Unlock()
	} else {
		var transaction gormFactorTransaction
		if err := harness.db.First(&transaction).Error; err != nil {
			t.Fatalf("load factor transaction error = %v", err)
		}
		stored = transaction.VerificationDigest
	}
	if !strings.HasPrefix(stored, digestPrefix) {
		t.Fatalf("stored verification value %q is not a digest", stored)
	}
	events, err := harness.runtime.AuditEvents(context.Background())
	if err != nil {
		t.Fatalf("AuditEvents() error = %v", err)
	}
	encoded, err := json.Marshal(events)
	if err != nil {
		t.Fatalf("json.Marshal(audit) error = %v", err)
	}
	serialized := string(encoded) + stored
	for _, secret := range secrets {
		if strings.Contains(serialized, secret) {
			t.Fatalf("persistent state contains raw verification secret %q", secret)
		}
	}
}
