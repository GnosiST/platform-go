package cache

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRedisInvalidationCertificationSmoke(t *testing.T) {
	addr := strings.TrimSpace(os.Getenv("PLATFORM_CERTIFICATION_REDIS_ADDR"))
	if addr == "" {
		t.Skip("PLATFORM_CERTIFICATION_REDIS_ADDR is not configured")
	}

	options := RedisOptions{Addr: addr}
	writer := NewRedisStore(options)
	reader := NewRedisStore(options)
	subscriber := NewRedisInvalidationBus(options)
	publisher := NewRedisInvalidationBus(options)
	t.Cleanup(func() {
		_ = writer.Delete(context.Background(), "platform:certification:redis:cross-instance")
		_ = writer.Close()
		_ = reader.Close()
		_ = subscriber.Close()
		_ = publisher.Close()
	})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	for _, checker := range []interface{ CheckReadiness(context.Context) error }{writer, reader, subscriber, publisher} {
		if err := checker.CheckReadiness(ctx); err != nil {
			t.Fatalf("Redis readiness check failed: %v", err)
		}
	}

	key := "platform:certification:redis:cross-instance"
	if err := writer.Set(ctx, key, []byte("ready"), time.Minute); err != nil {
		t.Fatal(err)
	}
	value, found, err := reader.Get(ctx, key)
	if err != nil || !found || string(value) != "ready" {
		t.Fatalf("cross-instance Redis read = %q, found=%t, err=%v", value, found, err)
	}

	received := make(chan InvalidationEvent, 1)
	if err := subscriber.SubscribeInvalidations(ctx, func(_ context.Context, event InvalidationEvent) {
		if event.Resource == "platform-certification" {
			received <- event
		}
	}); err != nil {
		t.Fatal(err)
	}
	waitForRedisSubscription(t, ctx, subscriber)

	event := InvalidationEvent{Resource: "platform-certification"}
	if err := publisher.PublishInvalidation(ctx, event); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-received:
		if got != event {
			t.Fatalf("invalidation event = %+v, want %+v", got, event)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Redis did not deliver the cross-instance invalidation event")
	}
}

func waitForRedisSubscription(t *testing.T, ctx context.Context, bus *RedisInvalidationBus) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		channels, err := bus.client.PubSubChannels(ctx, defaultInvalidationChannel).Result()
		if err != nil {
			t.Fatal(err)
		}
		for _, channel := range channels {
			if channel == defaultInvalidationChannel {
				return
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("Redis invalidation subscription did not become ready")
}
