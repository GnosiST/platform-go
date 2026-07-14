package serviceobject

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestMemoryIdempotencyStoreCoordinatesConcurrentReplay(t *testing.T) {
	store := NewMemoryIdempotencyStore()
	scope := testIdempotencyScope("same-key")
	entered := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	execute := func(context.Context) (CommandResult, error) {
		if calls.Add(1) == 1 {
			close(entered)
		}
		<-release
		return CommandResult{Values: map[string]any{"name": "updated"}}, nil
	}

	results := make(chan CommandResult, 2)
	errorsCh := make(chan error, 2)
	for range 2 {
		go func() {
			result, err := store.Execute(context.Background(), scope, "fingerprint", execute)
			results <- result
			errorsCh <- err
		}()
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("command did not start")
	}
	close(release)
	for range 2 {
		if err := <-errorsCh; err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if got := (<-results).Values["name"]; got != "updated" {
			t.Fatalf("Execute() result name = %v, want updated", got)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("execute calls = %d, want 1", got)
	}
}

func TestMemoryIdempotencyStoreDoesNotSerializeDifferentScopes(t *testing.T) {
	store := NewMemoryIdempotencyStore()
	firstEntered := make(chan struct{})
	secondEntered := make(chan struct{})
	release := make(chan struct{})

	go func() {
		_, _ = store.Execute(context.Background(), testIdempotencyScope("first"), "first-fingerprint", func(context.Context) (CommandResult, error) {
			close(firstEntered)
			<-release
			return CommandResult{}, nil
		})
	}()
	select {
	case <-firstEntered:
	case <-time.After(time.Second):
		t.Fatal("first command did not start")
	}
	go func() {
		_, _ = store.Execute(context.Background(), testIdempotencyScope("second"), "second-fingerprint", func(context.Context) (CommandResult, error) {
			close(secondEntered)
			<-release
			return CommandResult{}, nil
		})
	}()
	select {
	case <-secondEntered:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("different idempotency scopes were serialized")
	}
	close(release)
}

func TestMemoryIdempotencyStoreConflictAndFailedClaimRetry(t *testing.T) {
	store := NewMemoryIdempotencyStore()
	scope := testIdempotencyScope("retry")
	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, err := store.Execute(context.Background(), scope, "original", func(context.Context) (CommandResult, error) {
			close(entered)
			<-release
			return CommandResult{}, errors.New("command failed")
		})
		done <- err
	}()
	<-entered
	if _, err := store.Execute(context.Background(), scope, "different", func(context.Context) (CommandResult, error) {
		return CommandResult{}, nil
	}); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("conflicting Execute() error = %v, want ErrIdempotencyConflict", err)
	}
	close(release)
	if err := <-done; err == nil || err.Error() != "command failed" {
		t.Fatalf("failed Execute() error = %v, want command error", err)
	}
	if _, err := store.Execute(context.Background(), scope, "replacement", func(context.Context) (CommandResult, error) {
		return CommandResult{Values: map[string]any{"retried": true}}, nil
	}); err != nil {
		t.Fatalf("retry Execute() error = %v", err)
	}
}

func TestMemoryIdempotencyStoreBoundsCompletedRecordsAndExpiresKeys(t *testing.T) {
	now := time.Date(2026, time.July, 15, 10, 0, 0, 0, time.UTC)
	store := newMemoryIdempotencyStore(time.Minute, 1, func() time.Time { return now })
	var calls atomic.Int32
	execute := func(context.Context) (CommandResult, error) {
		return CommandResult{Values: map[string]any{"call": calls.Add(1)}}, nil
	}

	if _, err := store.Execute(context.Background(), testIdempotencyScope("one"), "one", execute); err != nil {
		t.Fatalf("first Execute() error = %v", err)
	}
	if _, err := store.Execute(context.Background(), testIdempotencyScope("two"), "two", execute); err != nil {
		t.Fatalf("second Execute() error = %v", err)
	}
	if got := len(store.records); got != 1 {
		t.Fatalf("record count = %d, want bounded count 1", got)
	}
	if _, err := store.Execute(context.Background(), testIdempotencyScope("two"), "changed", execute); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("unexpired key error = %v, want ErrIdempotencyConflict", err)
	}
	now = now.Add(time.Minute)
	if _, err := store.Execute(context.Background(), testIdempotencyScope("two"), "changed", execute); err != nil {
		t.Fatalf("expired key Execute() error = %v", err)
	}
	if got := calls.Load(); got != 3 {
		t.Fatalf("execute calls = %d, want 3", got)
	}
}

func TestMemoryIdempotencyStoreRejectsCapacityWhenAllRecordsAreInFlight(t *testing.T) {
	store := newMemoryIdempotencyStore(time.Minute, 1, time.Now)
	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = store.Execute(context.Background(), testIdempotencyScope("in-flight"), "fingerprint", func(context.Context) (CommandResult, error) {
			close(entered)
			<-release
			return CommandResult{}, nil
		})
	}()
	<-entered
	if _, err := store.Execute(context.Background(), testIdempotencyScope("over-capacity"), "fingerprint", func(context.Context) (CommandResult, error) {
		return CommandResult{}, nil
	}); !errors.Is(err, ErrExecutionFailed) {
		t.Fatalf("over-capacity Execute() error = %v, want ErrExecutionFailed", err)
	}
	close(release)
	<-done
}

func testIdempotencyScope(key string) IdempotencyScope {
	return IdempotencyScope{
		CommandID: "platform.records.rename",
		Version:   "1.0.0",
		Actor:     "operator",
		TenantID:  "tenant-a",
		Key:       key,
	}
}
