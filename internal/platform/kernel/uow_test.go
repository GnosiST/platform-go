package kernel

import (
	"context"
	"errors"
	"testing"
)

func TestNoopUnitOfWorkRunsCallback(t *testing.T) {
	uow := NoopUnitOfWork{}
	called := false

	err := uow.Do(context.Background(), func(ctx context.Context) error {
		called = true
		return nil
	})

	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	if !called {
		t.Fatalf("Do() did not call callback")
	}
}

func TestNoopUnitOfWorkReturnsCallbackError(t *testing.T) {
	uow := NoopUnitOfWork{}
	want := errors.New("boom")

	err := uow.Do(context.Background(), func(ctx context.Context) error {
		return want
	})

	if !errors.Is(err, want) {
		t.Fatalf("Do() error = %v, want %v", err, want)
	}
}
