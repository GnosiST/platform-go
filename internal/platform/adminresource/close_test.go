package adminresource

import (
	"context"
	"testing"
)

func TestStoreCloseClosesOptionalRepository(t *testing.T) {
	repository := &closableRepository{}
	store := NewStore()
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

func (r *closableRepository) Load(context.Context) (ResourceSnapshot, error) {
	return ResourceSnapshot{}, nil
}

func (r *closableRepository) Save(context.Context, ResourceSnapshot) (uint64, error) {
	return 0, nil
}

func (r *closableRepository) Close() error {
	r.closeCalls++
	return nil
}
