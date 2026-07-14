package serviceobject

import (
	"context"
	"sync"
	"time"
)

const (
	defaultMemoryIdempotencyTTL      = 15 * time.Minute
	defaultMemoryIdempotencyCapacity = 1024
)

type MemoryIdempotencyStore struct {
	mu       sync.Mutex
	records  map[IdempotencyScope]*memoryIdempotencyRecord
	ttl      time.Duration
	capacity int
	now      func() time.Time
}

type memoryIdempotencyRecord struct {
	fingerprint string
	result      CommandResult
	ready       chan struct{}
	completed   bool
	createdAt   time.Time
	expiresAt   time.Time
}

func NewMemoryIdempotencyStore() *MemoryIdempotencyStore {
	return newMemoryIdempotencyStore(defaultMemoryIdempotencyTTL, defaultMemoryIdempotencyCapacity, time.Now)
}

func newMemoryIdempotencyStore(ttl time.Duration, capacity int, now func() time.Time) *MemoryIdempotencyStore {
	return &MemoryIdempotencyStore{
		records:  map[IdempotencyScope]*memoryIdempotencyRecord{},
		ttl:      ttl,
		capacity: capacity,
		now:      now,
	}
}

func (s *MemoryIdempotencyStore) Execute(ctx context.Context, scope IdempotencyScope, fingerprint string, execute func(context.Context) (CommandResult, error)) (CommandResult, error) {
	if s == nil || ctx == nil || execute == nil || scope.Key == "" || fingerprint == "" || s.ttl <= 0 || s.capacity <= 0 || s.now == nil {
		return CommandResult{}, ErrRequestInvalid
	}

	for {
		now := s.now().UTC()
		s.mu.Lock()
		s.removeExpiredLocked(now)
		if record, exists := s.records[scope]; exists {
			if record.fingerprint != fingerprint {
				s.mu.Unlock()
				return CommandResult{}, ErrIdempotencyConflict
			}
			if record.completed {
				result := cloneCommandResult(record.result)
				s.mu.Unlock()
				return result, nil
			}
			ready := record.ready
			s.mu.Unlock()
			select {
			case <-ctx.Done():
				return CommandResult{}, ctx.Err()
			case <-ready:
				continue
			}
		}
		if !s.reserveCapacityLocked() {
			s.mu.Unlock()
			return CommandResult{}, ErrExecutionFailed
		}
		record := &memoryIdempotencyRecord{
			fingerprint: fingerprint,
			ready:       make(chan struct{}),
			createdAt:   now,
		}
		s.records[scope] = record
		s.mu.Unlock()

		result, err := execute(ctx)
		s.mu.Lock()
		current, exists := s.records[scope]
		if err != nil {
			if exists && current == record {
				delete(s.records, scope)
				close(record.ready)
			}
			s.mu.Unlock()
			return CommandResult{}, err
		}
		if !exists || current != record {
			s.mu.Unlock()
			return CommandResult{}, ErrExecutionFailed
		}
		record.result = cloneCommandResult(result)
		record.completed = true
		record.expiresAt = s.now().UTC().Add(s.ttl)
		close(record.ready)
		s.mu.Unlock()
		return cloneCommandResult(result), nil
	}
}

func (s *MemoryIdempotencyStore) removeExpiredLocked(now time.Time) {
	for scope, record := range s.records {
		if record.completed && !now.Before(record.expiresAt) {
			delete(s.records, scope)
		}
	}
}

func (s *MemoryIdempotencyStore) reserveCapacityLocked() bool {
	if len(s.records) < s.capacity {
		return true
	}
	var oldestScope IdempotencyScope
	var oldest *memoryIdempotencyRecord
	for scope, record := range s.records {
		if !record.completed || (oldest != nil && !record.createdAt.Before(oldest.createdAt)) {
			continue
		}
		oldestScope = scope
		oldest = record
	}
	if oldest == nil {
		return false
	}
	delete(s.records, oldestScope)
	return true
}

func cloneCommandResult(result CommandResult) CommandResult {
	values := make(map[string]any, len(result.Values))
	for key, value := range result.Values {
		values[key] = value
	}
	return CommandResult{Values: values}
}
