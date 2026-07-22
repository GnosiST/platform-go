package cache

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestConsumeRedisInvalidationsStopsWhenLifecycleIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	messages := make(chan *redis.Message)
	done := make(chan struct{})
	go func() {
		consumeRedisInvalidations(ctx, messages, func(context.Context, InvalidationEvent) {})
		close(done)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("consumeRedisInvalidations() did not stop after lifecycle cancellation")
	}
}
