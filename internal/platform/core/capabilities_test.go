package core

import (
	"testing"

	"platform-go/internal/platform/capability"
)

func TestDefaultManifestsResolve(t *testing.T) {
	registry := capability.NewRegistry()
	for _, manifest := range DefaultManifests() {
		if err := registry.Register(manifest); err != nil {
			t.Fatalf("Register(%q) error = %v", manifest.ID, err)
		}
	}

	enabled := make([]capability.ID, 0, len(DefaultManifests()))
	for _, manifest := range DefaultManifests() {
		enabled = append(enabled, manifest.ID)
	}

	ordered, err := registry.ResolveEnabled(enabled)
	if err != nil {
		t.Fatalf("ResolveEnabled() error = %v", err)
	}
	if len(ordered) != len(DefaultManifests()) {
		t.Fatalf("ResolveEnabled() returned %d manifests, want %d", len(ordered), len(DefaultManifests()))
	}
	if ordered[0].ID != "tenant" {
		t.Fatalf("first core capability = %q, want tenant", ordered[0].ID)
	}
}
