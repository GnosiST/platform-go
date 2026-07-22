package cache

import (
	"context"
	"sync"
	"time"
)

const (
	cacheGetFailed          = "CACHE_GET_FAILED"
	cacheSetFailed          = "CACHE_SET_FAILED"
	cacheDeleteFailed       = "CACHE_DELETE_FAILED"
	cacheDeletePrefixFailed = "CACHE_DELETE_PREFIX_FAILED"
)

type Stats struct {
	Driver         string `json:"driver"`
	Hits           uint64 `json:"hits"`
	Misses         uint64 `json:"misses"`
	Sets           uint64 `json:"sets"`
	Deletes        uint64 `json:"deletes"`
	DeletePrefixes uint64 `json:"deletePrefixes"`
	Errors         uint64 `json:"errors"`
	LastError      string `json:"lastError,omitempty"`
}

type StatsProvider interface {
	Stats() Stats
}

type MeteredStore struct {
	driver string
	inner  Store
	mu     sync.Mutex
	stats  Stats
}

func NewMeteredStore(driver string, inner Store) *MeteredStore {
	if inner == nil {
		inner = NewNoopStore()
	}
	if driver == "" {
		driver = "custom"
	}
	return &MeteredStore{driver: driver, inner: inner, stats: Stats{Driver: driver}}
}

func (s *MeteredStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	value, ok, err := s.inner.Get(ctx, key)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		s.stats.Errors++
		s.stats.LastError = cacheGetFailed
		return value, ok, err
	}
	if ok {
		s.stats.Hits++
	} else {
		s.stats.Misses++
	}
	return value, ok, nil
}

func (s *MeteredStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	err := s.inner.Set(ctx, key, value, ttl)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		s.stats.Errors++
		s.stats.LastError = cacheSetFailed
		return err
	}
	s.stats.Sets++
	return nil
}

func (s *MeteredStore) Delete(ctx context.Context, keys ...string) error {
	err := s.inner.Delete(ctx, keys...)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		s.stats.Errors++
		s.stats.LastError = cacheDeleteFailed
		return err
	}
	if len(keys) > 0 {
		s.stats.Deletes++
	}
	return nil
}

func (s *MeteredStore) DeletePrefix(ctx context.Context, prefix string) error {
	err := s.inner.DeletePrefix(ctx, prefix)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		s.stats.Errors++
		s.stats.LastError = cacheDeletePrefixFailed
		return err
	}
	if prefix != "" {
		s.stats.DeletePrefixes++
	}
	return nil
}

func (s *MeteredStore) Stats() Stats {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stats
}

func (s *MeteredStore) Close() error {
	if closer, ok := s.inner.(interface{ Close() error }); ok {
		return closer.Close()
	}
	return nil
}

func (s *MeteredStore) CheckReadiness(ctx context.Context) error {
	if checker, ok := s.inner.(ReadinessChecker); ok {
		return checker.CheckReadiness(ctx)
	}
	return nil
}
