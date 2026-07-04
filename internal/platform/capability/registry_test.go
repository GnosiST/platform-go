package capability

import (
	"context"
	"reflect"
	"testing"
)

func TestRegistryResolvesDependenciesInOrder(t *testing.T) {
	registry := NewRegistry()
	mustRegister(t, registry, Manifest{ID: "audit"})
	mustRegister(t, registry, Manifest{ID: "file-storage", Dependencies: []ID{"audit"}})

	ordered, err := registry.ResolveEnabled([]ID{"file-storage", "audit"})
	if err != nil {
		t.Fatalf("ResolveEnabled() error = %v", err)
	}
	got := []ID{ordered[0].ID, ordered[1].ID}
	want := []ID{"audit", "file-storage"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ResolveEnabled() = %#v, want %#v", got, want)
	}
}

func TestRegistryFailsWhenDependencyMissing(t *testing.T) {
	registry := NewRegistry()
	mustRegister(t, registry, Manifest{ID: "wechat-login", Dependencies: []ID{"identity"}})

	_, err := registry.ResolveEnabled([]ID{"wechat-login"})
	if err == nil {
		t.Fatalf("ResolveEnabled() error = nil, want missing dependency")
	}
}

func TestRegistryFailsOnDependencyCycle(t *testing.T) {
	registry := NewRegistry()
	mustRegister(t, registry, Manifest{ID: "a", Dependencies: []ID{"b"}})
	mustRegister(t, registry, Manifest{ID: "b", Dependencies: []ID{"a"}})

	_, err := registry.ResolveEnabled([]ID{"a", "b"})
	if err == nil {
		t.Fatalf("ResolveEnabled() error = nil, want cycle")
	}
}

func TestRunLifecycleCallsHooksInDependencyOrder(t *testing.T) {
	registry := NewRegistry()
	var calls []string
	mustRegister(t, registry, Manifest{
		ID: "audit",
		Hooks: Hooks{
			Configure: func(context.Context, Runtime) error {
				calls = append(calls, "audit.configure")
				return nil
			},
			Start: func(context.Context, Runtime) error {
				calls = append(calls, "audit.start")
				return nil
			},
		},
	})
	mustRegister(t, registry, Manifest{
		ID:           "files",
		Dependencies: []ID{"audit"},
		Hooks: Hooks{
			Configure: func(context.Context, Runtime) error {
				calls = append(calls, "files.configure")
				return nil
			},
			Start: func(context.Context, Runtime) error {
				calls = append(calls, "files.start")
				return nil
			},
		},
	})

	err := registry.RunLifecycle(context.Background(), []ID{"files", "audit"}, Runtime{})
	if err != nil {
		t.Fatalf("RunLifecycle() error = %v", err)
	}
	want := []string{"audit.configure", "files.configure", "audit.start", "files.start"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
}

func mustRegister(t *testing.T, registry *Registry, manifest Manifest) {
	t.Helper()
	if err := registry.Register(manifest); err != nil {
		t.Fatalf("Register(%q) error = %v", manifest.ID, err)
	}
}
