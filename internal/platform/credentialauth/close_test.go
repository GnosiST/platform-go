package credentialauth

import "testing"

func TestServiceCloseClosesOptionalRepository(t *testing.T) {
	repository := &closableRepository{MemoryRepository: NewMemoryRepository()}
	service, err := NewService(Options{Repository: repository})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	if err := service.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if repository.closeCalls != 1 {
		t.Fatalf("Close() calls = %d, want 1", repository.closeCalls)
	}
}

type closableRepository struct {
	*MemoryRepository
	closeCalls int
}

func (r *closableRepository) Close() error {
	r.closeCalls++
	return nil
}
