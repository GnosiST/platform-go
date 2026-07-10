package bootstrap

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"platform-go/internal/platform/config"
)

func TestSessionsFromConfigUsesMemoryStoreByDefault(t *testing.T) {
	store, err := SessionsFromConfig(config.Config{})
	if err != nil {
		t.Fatalf("SessionsFromConfig() error = %v", err)
	}
	issued, err := store.Issue("ops")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if _, ok := store.Resolve(issued.Token); !ok {
		t.Fatalf("Resolve() ok = false, want true")
	}
}

func TestSessionsFromConfigUsesFileBackedStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")
	cfg := config.Config{SessionFile: path}
	store, err := SessionsFromConfig(cfg)
	if err != nil {
		t.Fatalf("SessionsFromConfig() error = %v", err)
	}
	issued, err := store.Issue("ops")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	reloaded, err := SessionsFromConfig(cfg)
	if err != nil {
		t.Fatalf("reload SessionsFromConfig() error = %v", err)
	}
	if resolved, ok := reloaded.Resolve(issued.Token); !ok || resolved.Username != "ops" {
		t.Fatalf("Resolve() after reload = %+v, %v; want ops session", resolved, ok)
	}
	if !reloaded.Revoke(issued.Token) {
		t.Fatalf("Revoke() after reload = false, want true")
	}
	finalReload, err := SessionsFromConfig(cfg)
	if err != nil {
		t.Fatalf("final reload SessionsFromConfig() error = %v", err)
	}
	if _, ok := finalReload.Resolve(issued.Token); ok {
		t.Fatalf("Resolve() after revoke reload ok = true, want false")
	}
}

func TestSessionsFromConfigUsesGORMBackedStore(t *testing.T) {
	cfg := config.Config{
		SessionDriver: "sqlite",
		SessionDSN:    filepath.Join(t.TempDir(), "sessions.db"),
	}
	store, err := SessionsFromConfig(cfg)
	if err != nil {
		t.Fatalf("SessionsFromConfig() error = %v", err)
	}
	issued, err := store.Issue("ops")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	reloaded, err := SessionsFromConfig(cfg)
	if err != nil {
		t.Fatalf("reload SessionsFromConfig() error = %v", err)
	}
	if resolved, ok := reloaded.Resolve(issued.Token); !ok || resolved.Username != "ops" {
		t.Fatalf("Resolve() after GORM reload = %+v, %v; want ops session", resolved, ok)
	}
	if !reloaded.Revoke(issued.Token) {
		t.Fatalf("Revoke() after GORM reload = false, want true")
	}
	finalReload, err := SessionsFromConfig(cfg)
	if err != nil {
		t.Fatalf("final reload SessionsFromConfig() error = %v", err)
	}
	if _, ok := finalReload.Resolve(issued.Token); ok {
		t.Fatalf("Resolve() after GORM revoke reload ok = true, want false")
	}
}

func TestSessionsFromConfigRejectsGORMStoreWithoutDSN(t *testing.T) {
	_, err := SessionsFromConfig(config.Config{SessionDriver: "sqlite"})
	if err == nil {
		t.Fatalf("SessionsFromConfig() error = nil, want missing DSN")
	}
	if !strings.Contains(err.Error(), "session dsn is required") {
		t.Fatalf("SessionsFromConfig() error = %v, want missing DSN", err)
	}
}

func TestSessionsFromConfigUsesDefaultTTL(t *testing.T) {
	store, err := SessionsFromConfig(config.Config{})
	if err != nil {
		t.Fatalf("SessionsFromConfig() error = %v", err)
	}
	issued, err := store.Issue("ops")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if issued.ExpiresAt.Sub(issued.IssuedAt) != 8*time.Hour {
		t.Fatalf("session ttl = %s, want 8h", issued.ExpiresAt.Sub(issued.IssuedAt))
	}
}
