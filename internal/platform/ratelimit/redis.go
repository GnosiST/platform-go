package ratelimit

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const redisCounterScript = `
local count = redis.call("INCR", KEYS[1])
if count == 1 then
  redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
local ttl = redis.call("PTTL", KEYS[1])
if ttl < 0 then
  redis.call("PEXPIRE", KEYS[1], ARGV[1])
  ttl = tonumber(ARGV[1])
end
return {count, ttl}
`

type RedisOptions struct {
	Addr     string
	Password string
	DB       int
}

type redisEvaler interface {
	Eval(context.Context, string, []string, ...any) *redis.Cmd
}

type RedisLimiter struct {
	evaler redisEvaler
	close  func() error
}

func NewRedisLimiter(options RedisOptions) *RedisLimiter {
	addr := options.Addr
	if addr == "" {
		addr = "127.0.0.1:6379"
	}
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: options.Password,
		DB:       options.DB,
	})
	return &RedisLimiter{evaler: client, close: client.Close}
}

func newRedisLimiter(evaler redisEvaler) *RedisLimiter {
	return &RedisLimiter{evaler: evaler}
}

func (l *RedisLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (Decision, error) {
	if err := validateAllowInput(key, limit, window); err != nil {
		return Decision{}, err
	}
	if l == nil || l.evaler == nil {
		return Decision{}, fmt.Errorf("rate limit Redis backend is unavailable")
	}
	result, err := l.evaler.Eval(ctx, redisCounterScript, []string{key}, window.Milliseconds()).Slice()
	if err != nil {
		return Decision{}, fmt.Errorf("apply Redis rate limit: %w", err)
	}
	if len(result) != 2 {
		return Decision{}, fmt.Errorf("apply Redis rate limit: invalid result")
	}
	count, err := redisResultInt64(result[0])
	if err != nil {
		return Decision{}, fmt.Errorf("apply Redis rate limit count: %w", err)
	}
	ttlMillis, err := redisResultInt64(result[1])
	if err != nil {
		return Decision{}, fmt.Errorf("apply Redis rate limit ttl: %w", err)
	}
	if count <= int64(limit) {
		return Decision{Allowed: true}, nil
	}
	if ttlMillis < 1 {
		ttlMillis = 1
	}
	return Decision{RetryAfter: time.Duration(ttlMillis) * time.Millisecond}, nil
}

func (l *RedisLimiter) Close() error {
	if l == nil || l.close == nil {
		return nil
	}
	return l.close()
}

func redisResultInt64(value any) (int64, error) {
	switch typed := value.(type) {
	case int64:
		return typed, nil
	case int:
		return int64(typed), nil
	case string:
		return strconv.ParseInt(typed, 10, 64)
	case []byte:
		return strconv.ParseInt(string(typed), 10, 64)
	default:
		return 0, fmt.Errorf("unexpected %T", value)
	}
}
