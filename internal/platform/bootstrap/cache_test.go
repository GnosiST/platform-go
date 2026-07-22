package bootstrap

import (
	"testing"

	"github.com/GnosiST/platform-go/internal/platform/cache"
	"github.com/GnosiST/platform-go/internal/platform/config"
)

func TestCacheFromConfigUsesNoopByDefault(t *testing.T) {
	store, err := CacheFromConfig(config.Config{})
	if err != nil {
		t.Fatalf("CacheFromConfig() error = %v", err)
	}
	statsProvider, ok := store.(cache.StatsProvider)
	if !ok {
		t.Fatalf("CacheFromConfig() = %T, want stats provider", store)
	}
	if stats := statsProvider.Stats(); stats.Driver != "noop" {
		t.Fatalf("Stats().Driver = %q, want noop", stats.Driver)
	}
}

func TestCacheFromConfigUsesMemoryStore(t *testing.T) {
	store, err := CacheFromConfig(config.Config{CacheDriver: "memory"})
	if err != nil {
		t.Fatalf("CacheFromConfig() error = %v", err)
	}
	statsProvider, ok := store.(cache.StatsProvider)
	if !ok {
		t.Fatalf("CacheFromConfig() = %T, want stats provider", store)
	}
	if stats := statsProvider.Stats(); stats.Driver != "memory" {
		t.Fatalf("Stats().Driver = %q, want memory", stats.Driver)
	}
}

func TestCacheFromConfigRejectsUnknownDriver(t *testing.T) {
	_, err := CacheFromConfig(config.Config{CacheDriver: "memcached"})
	if err == nil {
		t.Fatalf("CacheFromConfig() error = nil, want unsupported driver")
	}
}

func TestCacheRuntimeFromConfigOwnsStoreAndInvalidationBus(t *testing.T) {
	runtime, err := CacheRuntimeFromConfig(config.Config{CacheDriver: "memory"})
	if err != nil {
		t.Fatalf("CacheRuntimeFromConfig() error = %v", err)
	}
	if runtime.Store == nil || runtime.InvalidationBus == nil {
		t.Fatalf("runtime = %#v, want store and invalidation bus", runtime)
	}
	if err := runtime.Close(); err != nil {
		t.Fatalf("runtime.Close() error = %v", err)
	}
}
