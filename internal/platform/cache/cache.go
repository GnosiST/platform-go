package cache

import (
	"context"
	"strings"
	"sync"
	"time"
)

type Store interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, keys ...string) error
	DeletePrefix(ctx context.Context, prefix string) error
}

type NoopStore struct{}

func NewNoopStore() NoopStore {
	return NoopStore{}
}

func (NoopStore) Get(context.Context, string) ([]byte, bool, error) {
	return nil, false, nil
}

func (NoopStore) Set(context.Context, string, []byte, time.Duration) error {
	return nil
}

func (NoopStore) Delete(context.Context, ...string) error {
	return nil
}

func (NoopStore) DeletePrefix(context.Context, string) error {
	return nil
}

type MemoryStoreOptions struct {
	Now func() time.Time
}

type MemoryStore struct {
	mu      sync.Mutex
	entries map[string]memoryEntry
	now     func() time.Time
}

type memoryEntry struct {
	value     []byte
	expiresAt time.Time
}

func NewMemoryStore(options MemoryStoreOptions) *MemoryStore {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &MemoryStore{
		entries: map[string]memoryEntry{},
		now:     now,
	}
}

func (s *MemoryStore) Get(_ context.Context, key string) ([]byte, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.entries[key]
	if !ok {
		return nil, false, nil
	}
	if !entry.expiresAt.IsZero() && !s.now().Before(entry.expiresAt) {
		delete(s.entries, key)
		return nil, false, nil
	}
	return append([]byte(nil), entry.value...), true, nil
}

func (s *MemoryStore) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := memoryEntry{value: append([]byte(nil), value...)}
	if ttl > 0 {
		entry.expiresAt = s.now().Add(ttl)
	}
	s.entries[key] = entry
	return nil
}

func (s *MemoryStore) Delete(_ context.Context, keys ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, key := range keys {
		delete(s.entries, key)
	}
	return nil
}

func (s *MemoryStore) DeletePrefix(_ context.Context, prefix string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key := range s.entries {
		if strings.HasPrefix(key, prefix) {
			delete(s.entries, key)
		}
	}
	return nil
}
