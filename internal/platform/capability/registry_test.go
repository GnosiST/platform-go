package capability

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

func TestRegistryResolvesDependenciesInOrder(t *testing.T) {
	registry := NewRegistry()
	mustRegister(t, registry, testManifest("audit"))
	mustRegister(t, registry, testManifest("file-storage", ID("audit")))

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
	mustRegister(t, registry, testManifest("wechat-login", ID("identity")))

	_, err := registry.ResolveEnabled([]ID{"wechat-login"})
	if err == nil {
		t.Fatalf("ResolveEnabled() error = nil, want missing dependency")
	}
}

func TestRegistryFailsOnDependencyCycle(t *testing.T) {
	registry := NewRegistry()
	mustRegister(t, registry, testManifest("a", ID("b")))
	mustRegister(t, registry, testManifest("b", ID("a")))

	_, err := registry.ResolveEnabled([]ID{"a", "b"})
	if err == nil {
		t.Fatalf("ResolveEnabled() error = nil, want cycle")
	}
}

func TestRunLifecycleCallsHooksInDependencyOrder(t *testing.T) {
	registry := NewRegistry()
	var calls []string
	audit := testManifest("audit")
	audit.Hooks = Hooks{
		Configure: func(context.Context, Runtime) error {
			calls = append(calls, "audit.configure")
			return nil
		},
		Start: func(context.Context, Runtime) error {
			calls = append(calls, "audit.start")
			return nil
		},
	}
	mustRegister(t, registry, audit)
	files := testManifest("files", ID("audit"))
	files.Hooks = Hooks{
		Configure: func(context.Context, Runtime) error {
			calls = append(calls, "files.configure")
			return nil
		},
		Start: func(context.Context, Runtime) error {
			calls = append(calls, "files.start")
			return nil
		},
	}
	mustRegister(t, registry, files)

	err := registry.RunLifecycle(context.Background(), []ID{"files", "audit"}, Runtime{})
	if err != nil {
		t.Fatalf("RunLifecycle() error = %v", err)
	}
	want := []string{"audit.configure", "files.configure", "audit.start", "files.start"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
}

func TestRegistryRejectsInvalidManifestMetadata(t *testing.T) {
	tests := []struct {
		name     string
		manifest Manifest
		want     string
	}{
		{
			name:     "blank id after trim",
			manifest: Manifest{ID: " ", Name: "Blank", Version: "0.1.0"},
			want:     "capability id is required",
		},
		{
			name:     "uppercase id",
			manifest: Manifest{ID: "WeChat", Name: "WeChat", Version: "0.1.0"},
			want:     "must use lowercase letters, numbers, and hyphens",
		},
		{
			name:     "underscore id",
			manifest: Manifest{ID: "wechat_login", Name: "WeChat", Version: "0.1.0"},
			want:     "must use lowercase letters, numbers, and hyphens",
		},
		{
			name:     "missing version",
			manifest: Manifest{ID: "wechat-login", Name: "WeChat"},
			want:     "capability \"wechat-login\" version is required",
		},
		{
			name:     "invalid version",
			manifest: Manifest{ID: "wechat-login", Name: "WeChat", Version: "v1"},
			want:     "version must use numeric semver",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewRegistry()
			err := registry.Register(tt.manifest)
			if err == nil {
				t.Fatalf("Register() error = nil, want %q", tt.want)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Register() error = %q, want containing %q", err.Error(), tt.want)
			}
		})
	}
}

func TestRegistryNormalizesManifestIDAndRejectsTrimmedDuplicates(t *testing.T) {
	registry := NewRegistry()
	mustRegister(t, registry, Manifest{ID: " audit ", Name: "Audit", Version: "0.1.0"})

	err := registry.Register(testManifest("audit"))
	if err == nil {
		t.Fatalf("Register() error = nil, want duplicate")
	}
	if !strings.Contains(err.Error(), "capability \"audit\" already registered") {
		t.Fatalf("Register() error = %q, want duplicate audit", err.Error())
	}

	ordered, err := registry.ResolveEnabled([]ID{"audit"})
	if err != nil {
		t.Fatalf("ResolveEnabled() error = %v", err)
	}
	if got := ordered[0].ID; got != "audit" {
		t.Fatalf("registered capability ID = %q, want normalized audit", got)
	}
}

func TestRegistryRejectsInvalidDependencies(t *testing.T) {
	tests := []struct {
		name         string
		dependencies []ID
		want         string
	}{
		{
			name:         "blank dependency after trim",
			dependencies: []ID{" "},
			want:         "dependency id is required",
		},
		{
			name:         "invalid dependency id",
			dependencies: []ID{"Identity"},
			want:         "dependency \"Identity\" must use lowercase letters, numbers, and hyphens",
		},
		{
			name:         "duplicate dependency after trim",
			dependencies: []ID{"identity", " identity "},
			want:         "duplicate dependency \"identity\"",
		},
		{
			name:         "self dependency after trim",
			dependencies: []ID{" platform "},
			want:         "cannot depend on itself",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewRegistry()
			err := registry.Register(Manifest{ID: "platform", Name: "Platform", Version: "0.1.0", Dependencies: tt.dependencies})
			if err == nil {
				t.Fatalf("Register() error = nil, want %q", tt.want)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Register() error = %q, want containing %q", err.Error(), tt.want)
			}
		})
	}
}

func TestResolveEnabledNormalizesEnabledIDs(t *testing.T) {
	registry := NewRegistry()
	mustRegister(t, registry, testManifest("audit"))
	mustRegister(t, registry, testManifest("file-storage", ID("audit")))

	ordered, err := registry.ResolveEnabled([]ID{" file-storage ", " audit "})
	if err != nil {
		t.Fatalf("ResolveEnabled() error = %v", err)
	}
	got := []ID{ordered[0].ID, ordered[1].ID}
	want := []ID{"audit", "file-storage"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ResolveEnabled() = %#v, want %#v", got, want)
	}
}

func TestResolveEnabledRejectsInvalidEnabledIDs(t *testing.T) {
	tests := []struct {
		name    string
		enabled []ID
		want    string
	}{
		{
			name:    "nil enabled list",
			enabled: nil,
			want:    "enabled capabilities must not be empty",
		},
		{
			name:    "empty enabled list",
			enabled: []ID{},
			want:    "enabled capabilities must not be empty",
		},
		{
			name:    "blank id after trim",
			enabled: []ID{" "},
			want:    "enabled capability id is required",
		},
		{
			name:    "invalid id",
			enabled: []ID{"Audit"},
			want:    "enabled capability \"Audit\" must use lowercase letters, numbers, and hyphens",
		},
		{
			name:    "duplicate id after trim",
			enabled: []ID{"audit", " audit "},
			want:    "duplicate enabled capability \"audit\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewRegistry()
			mustRegister(t, registry, testManifest("audit"))

			_, err := registry.ResolveEnabled(tt.enabled)
			if err == nil {
				t.Fatalf("ResolveEnabled() error = nil, want %q", tt.want)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ResolveEnabled() error = %q, want containing %q", err.Error(), tt.want)
			}
		})
	}
}

func testManifest(id ID, dependencies ...ID) Manifest {
	return Manifest{
		ID:           id,
		Name:         string(id),
		Version:      "0.1.0",
		Dependencies: dependencies,
	}
}

func mustRegister(t *testing.T, registry *Registry, manifest Manifest) {
	t.Helper()
	if err := registry.Register(manifest); err != nil {
		t.Fatalf("Register(%q) error = %v", manifest.ID, err)
	}
}
