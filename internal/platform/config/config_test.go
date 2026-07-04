package config

import (
	"reflect"
	"testing"
)

func TestLoadUsesDefaults(t *testing.T) {
	t.Setenv("PLATFORM_HTTP_ADDR", "")
	t.Setenv("PLATFORM_CAPABILITIES", "")

	cfg := Load()

	if cfg.HTTPAddr != "127.0.0.1:9200" {
		t.Fatalf("HTTPAddr = %q", cfg.HTTPAddr)
	}
	if len(cfg.Capabilities) == 0 {
		t.Fatalf("Capabilities is empty")
	}
}

func TestLoadParsesCapabilities(t *testing.T) {
	t.Setenv("PLATFORM_CAPABILITIES", "tenant, identity, audit")

	cfg := Load()
	want := []string{"tenant", "identity", "audit"}
	if !reflect.DeepEqual(cfg.Capabilities, want) {
		t.Fatalf("Capabilities = %#v, want %#v", cfg.Capabilities, want)
	}
}
