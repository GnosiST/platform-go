package cache

import (
	"context"
	"errors"
	"testing"
	"time"
)

type runtimeStoreStub struct {
	closeCalls int
	readyErr   error
}

func (s *runtimeStoreStub) Get(context.Context, string) ([]byte, bool, error) { return nil, false, nil }
func (s *runtimeStoreStub) Set(context.Context, string, []byte, time.Duration) error {
	return nil
}
func (s *runtimeStoreStub) Delete(context.Context, ...string) error    { return nil }
func (s *runtimeStoreStub) DeletePrefix(context.Context, string) error { return nil }
func (s *runtimeStoreStub) Close() error {
	s.closeCalls++
	return nil
}
func (s *runtimeStoreStub) CheckReadiness(context.Context) error { return s.readyErr }

type runtimeBusStub struct {
	closeCalls int
	readyErr   error
}

func (b *runtimeBusStub) PublishInvalidation(context.Context, InvalidationEvent) error { return nil }
func (b *runtimeBusStub) SubscribeInvalidations(context.Context, InvalidationHandler) error {
	return nil
}
func (b *runtimeBusStub) Close() error {
	b.closeCalls++
	return nil
}
func (b *runtimeBusStub) CheckReadiness(context.Context) error { return b.readyErr }

func TestRuntimeClosesOwnedCacheResources(t *testing.T) {
	store := &runtimeStoreStub{}
	bus := &runtimeBusStub{}
	runtime := Runtime{Store: store, InvalidationBus: bus}

	if err := runtime.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if store.closeCalls != 1 || bus.closeCalls != 1 {
		t.Fatalf("close calls store=%d bus=%d, want 1 each", store.closeCalls, bus.closeCalls)
	}
}

func TestRuntimeReadinessChecksConfiguredDependencies(t *testing.T) {
	dependencyFailure := errors.New("dependency unavailable")
	runtime := Runtime{
		Store:           &runtimeStoreStub{},
		InvalidationBus: &runtimeBusStub{readyErr: dependencyFailure},
	}

	if err := runtime.CheckReadiness(context.Background()); !errors.Is(err, dependencyFailure) {
		t.Fatalf("CheckReadiness() error = %v, want dependency failure", err)
	}
}
