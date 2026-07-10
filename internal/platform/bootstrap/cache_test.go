package bootstrap

import (
	"testing"

	"platform-go/internal/platform/cache"
	"platform-go/internal/platform/config"
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
