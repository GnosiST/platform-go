package session

import (
	"context"
	"testing"
	"time"
)

func TestStoreCloseClosesOptionalRepository(t *testing.T) {
	repository := &closableRepository{}
	store := NewStore(Options{})
	store.repository = repository

	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if repository.closeCalls != 1 {
		t.Fatalf("Close() calls = %d, want 1", repository.closeCalls)
	}
}

type closableRepository struct {
	closeCalls int
}

func (r *closableRepository) Load(context.Context) (Snapshot, error) {
	return Snapshot{}, nil
}

func (r *closableRepository) Create(context.Context, StoredSession) error { return nil }

func (r *closableRepository) Resolve(context.Context, string, time.Time) (StoredSession, bool, error) {
	return StoredSession{}, false, nil
}

func (r *closableRepository) Renew(context.Context, string, time.Time, time.Time) (StoredSession, bool, error) {
	return StoredSession{}, false, nil
}

func (r *closableRepository) Revoke(context.Context, string, time.Time) (StoredSession, bool, error) {
	return StoredSession{}, false, nil
}

func (r *closableRepository) Close() error {
	r.closeCalls++
	return nil
}
