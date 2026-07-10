package cache

import (
	"context"
	"testing"
	"time"
)

func TestMemoryStoreCachesUntilTTLExpires(t *testing.T) {
	now := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
	store := NewMemoryStore(MemoryStoreOptions{Now: func() time.Time { return now }})
	ctx := context.Background()

	if err := store.Set(ctx, "admin:branding", []byte(`{"ok":true}`), time.Minute); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	value, ok, err := store.Get(ctx, "admin:branding")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !ok || string(value) != `{"ok":true}` {
		t.Fatalf("Get() = %q, %v; want cached value", string(value), ok)
	}

	now = now.Add(2 * time.Minute)
	if _, ok, err := store.Get(ctx, "admin:branding"); err != nil || ok {
		t.Fatalf("Get(expired) ok = %v err = %v, want miss", ok, err)
	}
}

func TestMemoryStoreDeletesByPrefix(t *testing.T) {
	store := NewMemoryStore(MemoryStoreOptions{})
	ctx := context.Background()
	if err := store.Set(ctx, "admin:menus:admin", []byte("admin"), 0); err != nil {
		t.Fatalf("Set(admin) error = %v", err)
	}
	if err := store.Set(ctx, "admin:menus:ops", []byte("ops"), 0); err != nil {
		t.Fatalf("Set(ops) error = %v", err)
	}
	if err := store.Set(ctx, "admin:branding", []byte("branding"), 0); err != nil {
		t.Fatalf("Set(branding) error = %v", err)
	}

	if err := store.DeletePrefix(ctx, "admin:menus:"); err != nil {
		t.Fatalf("DeletePrefix() error = %v", err)
	}
	if _, ok, err := store.Get(ctx, "admin:menus:admin"); err != nil || ok {
		t.Fatalf("Get(admin menu) ok = %v err = %v, want miss", ok, err)
	}
	if value, ok, err := store.Get(ctx, "admin:branding"); err != nil || !ok || string(value) != "branding" {
		t.Fatalf("Get(branding) = %q, %v, %v; want preserved", string(value), ok, err)
	}
}

func TestNoopStoreAlwaysMisses(t *testing.T) {
	store := NewNoopStore()
	ctx := context.Background()
	if err := store.Set(ctx, "key", []byte("value"), time.Minute); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if _, ok, err := store.Get(ctx, "key"); err != nil || ok {
		t.Fatalf("Get() ok = %v err = %v, want miss", ok, err)
	}
}

func TestMemoryInvalidationBusPublishesResourceEvents(t *testing.T) {
	bus := NewMemoryInvalidationBus()
	ctx := context.Background()
	var resources []string
	if err := bus.SubscribeInvalidations(ctx, func(_ context.Context, event InvalidationEvent) {
		resources = append(resources, event.Resource)
	}); err != nil {
		t.Fatalf("SubscribeInvalidations() error = %v", err)
	}

	if err := bus.PublishInvalidation(ctx, InvalidationEvent{}); err != nil {
		t.Fatalf("PublishInvalidation(empty) error = %v", err)
	}
	if err := bus.PublishInvalidation(ctx, InvalidationEvent{Resource: "roles"}); err != nil {
		t.Fatalf("PublishInvalidation(roles) error = %v", err)
	}

	if len(resources) != 1 || resources[0] != "roles" {
		t.Fatalf("published resources = %+v, want roles only", resources)
	}
}

func TestMeteredStoreRecordsCacheStats(t *testing.T) {
	store := NewMeteredStore("memory", NewMemoryStore(MemoryStoreOptions{}))
	ctx := context.Background()

	if _, ok, err := store.Get(ctx, "missing"); err != nil || ok {
		t.Fatalf("Get(missing) ok = %v err = %v, want miss", ok, err)
	}
	if err := store.Set(ctx, "admin:principal:ops", []byte("ops"), time.Minute); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if _, ok, err := store.Get(ctx, "admin:principal:ops"); err != nil || !ok {
		t.Fatalf("Get(hit) ok = %v err = %v, want hit", ok, err)
	}
	if err := store.Delete(ctx, "admin:principal:ops"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if err := store.Set(ctx, "admin:menus:ops", []byte("menus"), time.Minute); err != nil {
		t.Fatalf("Set(menu) error = %v", err)
	}
	if err := store.DeletePrefix(ctx, "admin:menus:"); err != nil {
		t.Fatalf("DeletePrefix() error = %v", err)
	}

	stats := store.Stats()
	if stats.Driver != "memory" {
		t.Fatalf("Driver = %q, want memory", stats.Driver)
	}
	if stats.Hits != 1 || stats.Misses != 1 || stats.Sets != 2 || stats.Deletes != 1 || stats.DeletePrefixes != 1 || stats.Errors != 0 {
		t.Fatalf("Stats() = %+v, want hit/miss/set/delete counts", stats)
	}
}
