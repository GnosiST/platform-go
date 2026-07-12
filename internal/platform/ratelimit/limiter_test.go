package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestRateLimitPoliciesMatchSecurityBaseline(t *testing.T) {
	tests := []struct {
		operation Operation
		limit     int
		window    time.Duration
	}{
		{OperationAdminLogin, 10, 5 * time.Minute},
		{OperationAppLogin, 10, 5 * time.Minute},
		{OperationAdminOIDCStart, 20, 5 * time.Minute},
		{OperationPhoneVerificationRequest, 5, 10 * time.Minute},
		{OperationPhoneBindingVerification, 10, 10 * time.Minute},
		{OperationAdminUpload, 30, time.Minute},
		{OperationAppUpload, 30, time.Minute},
	}

	for _, tt := range tests {
		t.Run(string(tt.operation), func(t *testing.T) {
			policy, ok := PolicyFor(tt.operation)
			if !ok {
				t.Fatalf("PolicyFor(%q) ok = false", tt.operation)
			}
			if policy.Limit != tt.limit || policy.Window != tt.window {
				t.Fatalf("PolicyFor(%q) = %+v, want limit=%d window=%s", tt.operation, policy, tt.limit, tt.window)
			}
		})
	}
}

func TestMemoryRateLimitThresholdAndWindowReset(t *testing.T) {
	now := time.Date(2026, 7, 12, 9, 0, 0, 0, time.UTC)
	limiter := NewMemoryLimiter(MemoryOptions{Now: func() time.Time { return now }})

	for attempt := 1; attempt <= 2; attempt++ {
		decision, err := limiter.Allow(context.Background(), "platform:ratelimit:v1:test:digest", 2, time.Minute)
		if err != nil || !decision.Allowed || decision.RetryAfter != 0 {
			t.Fatalf("Allow(attempt %d) = %+v, %v; want allowed", attempt, decision, err)
		}
	}
	decision, err := limiter.Allow(context.Background(), "platform:ratelimit:v1:test:digest", 2, time.Minute)
	if err != nil || decision.Allowed || decision.RetryAfter != time.Minute {
		t.Fatalf("Allow(over limit) = %+v, %v; want denied for one minute", decision, err)
	}

	now = now.Add(time.Minute)
	decision, err = limiter.Allow(context.Background(), "platform:ratelimit:v1:test:digest", 2, time.Minute)
	if err != nil || !decision.Allowed || decision.RetryAfter != 0 {
		t.Fatalf("Allow(after reset) = %+v, %v; want allowed", decision, err)
	}
}

func TestRedisRateLimitUsesAtomicLuaAndSharesState(t *testing.T) {
	backend := newSharedRedisEvaler()
	first := newRedisLimiter(backend)
	second := newRedisLimiter(backend)
	key := "platform:ratelimit:v1:admin-login:digest"

	for attempt, limiter := range []*RedisLimiter{first, second} {
		decision, err := limiter.Allow(context.Background(), key, 2, 5*time.Minute)
		if err != nil || !decision.Allowed {
			t.Fatalf("Allow(shared attempt %d) = %+v, %v; want allowed", attempt+1, decision, err)
		}
	}
	decision, err := first.Allow(context.Background(), key, 2, 5*time.Minute)
	if err != nil || decision.Allowed || decision.RetryAfter != 5*time.Minute {
		t.Fatalf("Allow(shared over limit) = %+v, %v; want denied", decision, err)
	}
	if backend.lastKey != key {
		t.Fatalf("Redis key = %q, want %q", backend.lastKey, key)
	}
	for _, command := range []string{"INCR", "PEXPIRE", "PTTL"} {
		if !strings.Contains(backend.lastScript, command) {
			t.Fatalf("Redis Lua script missing %s: %s", command, backend.lastScript)
		}
	}
}

func TestRedisRateLimitFailsClosedOnBackendError(t *testing.T) {
	backend := newSharedRedisEvaler()
	backend.err = errors.New("redis detail must stay internal")
	decision, err := newRedisLimiter(backend).Allow(context.Background(), "platform:ratelimit:v1:test:digest", 1, time.Minute)
	if err == nil || decision.Allowed {
		t.Fatalf("Allow(backend error) = %+v, %v; want fail-closed error", decision, err)
	}
}

func TestRateLimitKeyBuilderNormalizesAndRedactsDimensions(t *testing.T) {
	builder, err := NewKeyBuilder([]byte(strings.Repeat("k", 32)))
	if err != nil {
		t.Fatalf("NewKeyBuilder() error = %v", err)
	}
	rawMarkers := []string{"Sensitive.User", "+8613800138000", "203.0.113.25"}
	first := builder.Build(OperationPhoneVerificationRequest, " Sensitive.User ", " +8613800138000 ", " 203.0.113.25 ")
	second := builder.Build(OperationPhoneVerificationRequest, "sensitive.user", "+8613800138000", "203.0.113.25")
	if first != second {
		t.Fatalf("normalized keys differ: %q != %q", first, second)
	}
	if !strings.HasPrefix(first, "platform:ratelimit:v1:phone-verification-request:") {
		t.Fatalf("key = %q, want stable operation prefix", first)
	}
	for _, marker := range rawMarkers {
		if strings.Contains(strings.ToLower(first), strings.ToLower(marker)) {
			t.Fatalf("key %q leaked raw marker %q", first, marker)
		}
	}
	if other := builder.Build(OperationPhoneVerificationRequest, "other.user", "+8613800138000", "203.0.113.25"); other == first {
		t.Fatalf("different normalized dimensions produced the same key %q", first)
	}
}

type sharedRedisEvaler struct {
	mu         sync.Mutex
	counts     map[string]int64
	ttl        map[string]time.Duration
	err        error
	lastKey    string
	lastScript string
}

func newSharedRedisEvaler() *sharedRedisEvaler {
	return &sharedRedisEvaler{counts: map[string]int64{}, ttl: map[string]time.Duration{}}
}

func (f *sharedRedisEvaler) Eval(_ context.Context, script string, keys []string, args ...any) *redis.Cmd {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return redis.NewCmdResult(nil, f.err)
	}
	if len(keys) != 1 || len(args) != 1 {
		return redis.NewCmdResult(nil, fmt.Errorf("unexpected eval inputs"))
	}
	windowMillis, ok := args[0].(int64)
	if !ok {
		return redis.NewCmdResult(nil, fmt.Errorf("window argument = %T, want int64", args[0]))
	}
	key := keys[0]
	f.lastKey = key
	f.lastScript = script
	f.counts[key]++
	if f.counts[key] == 1 {
		f.ttl[key] = time.Duration(windowMillis) * time.Millisecond
	}
	return redis.NewCmdResult([]any{f.counts[key], f.ttl[key].Milliseconds()}, nil)
}
