package ratelimit

import (
	"context"
	"sync"
	"time"
)

type MemoryOptions struct {
	Now func() time.Time
}

type memoryEntry struct {
	count     int
	expiresAt time.Time
}

type MemoryLimiter struct {
	mu      sync.Mutex
	entries map[string]memoryEntry
	now     func() time.Time
}

func NewMemoryLimiter(options MemoryOptions) *MemoryLimiter {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &MemoryLimiter{entries: map[string]memoryEntry{}, now: now}
}

func (l *MemoryLimiter) Allow(_ context.Context, key string, limit int, window time.Duration) (Decision, error) {
	if err := validateAllowInput(key, limit, window); err != nil {
		return Decision{}, err
	}
	now := l.now().UTC()
	l.mu.Lock()
	defer l.mu.Unlock()
	entry, ok := l.entries[key]
	if !ok || !entry.expiresAt.After(now) {
		l.entries[key] = memoryEntry{count: 1, expiresAt: now.Add(window)}
		return Decision{Allowed: true}, nil
	}
	entry.count++
	l.entries[key] = entry
	if entry.count <= limit {
		return Decision{Allowed: true}, nil
	}
	return Decision{RetryAfter: entry.expiresAt.Sub(now)}, nil
}
