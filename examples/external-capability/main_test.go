package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/GnosiST/platform-go/pkg/platform/capability"
)

func TestExampleManifestResolvesThroughPublicContracts(t *testing.T) {
	manifests, err := ResolveExampleManifests()
	if err != nil {
		t.Fatalf("ResolveExampleManifests() error = %v", err)
	}
	if len(manifests) != 1 || manifests[0].ID != "example-catalog" {
		t.Fatalf("resolved manifests = %+v, want example-catalog", manifests)
	}
	if got := manifests[0].Admin.Resources[0].Resource; got != "catalog-items" {
		t.Fatalf("admin resource = %q, want catalog-items", got)
	}
}

func TestManifestDeclaresSettingsResource(t *testing.T) {
	manifests, err := ResolveExampleManifests()
	if err != nil {
		t.Fatalf("ResolveExampleManifests() error = %v", err)
	}
	for _, resource := range manifests[0].Admin.Resources {
		if resource.Resource != "catalog-settings" {
			continue
		}
		if resource.PermissionPrefix != "admin:catalog-setting" {
			t.Fatalf("settings permission prefix = %q, want admin:catalog-setting", resource.PermissionPrefix)
		}
		if resource.Menu.Parent != "configuration" || resource.Menu.Route != "/catalog-settings" {
			t.Fatalf("settings menu = %+v, want configuration parent and /catalog-settings route", resource.Menu)
		}
		return
	}
	t.Fatalf("manifest missing catalog-settings resource")
}

func TestContractPreviewIsJSONSafe(t *testing.T) {
	preview, err := BuildContractPreview()
	if err != nil {
		t.Fatalf("BuildContractPreview() error = %v", err)
	}
	encoded, err := json.Marshal(preview)
	if err != nil {
		t.Fatalf("json.Marshal(preview) error = %v", err)
	}
	if !strings.Contains(string(encoded), `"catalog-items"`) {
		t.Fatalf("preview JSON = %s, want catalog-items", encoded)
	}
	if !strings.Contains(string(encoded), `"catalog-settings"`) {
		t.Fatalf("preview JSON = %s, want catalog-settings", encoded)
	}
	if strings.Join(preview.ConfigResources, ",") != "catalog-settings" {
		t.Fatalf("config resources = %v, want catalog-settings", preview.ConfigResources)
	}
	if !strings.HasPrefix(preview.ServiceContractHash, "sha256:") || preview.ServiceCount != 1 {
		t.Fatalf("service preview = hash:%q count:%d, want one hashed service contract", preview.ServiceContractHash, preview.ServiceCount)
	}
}

func TestLifecycleRunsThroughPublicRuntime(t *testing.T) {
	manifests, err := ResolveExampleManifests()
	if err != nil {
		t.Fatalf("ResolveExampleManifests() error = %v", err)
	}
	history := capability.NewMemoryLifecycleHistory()
	executor := capability.NewRecordedLifecycleExecutor(history)
	runtime := capability.Runtime{MigrationExecutor: executor, SeedExecutor: executor}

	if err := capability.RunLifecycle(context.Background(), manifests, runtime); err != nil {
		t.Fatalf("RunLifecycle() error = %v", err)
	}
	records := history.Records()
	got := make([]string, 0, len(records))
	for _, record := range records {
		got = append(got, string(record.CapabilityID)+":"+record.StepID+":"+string(record.Kind))
	}
	want := "example-catalog:catalog-0001:migration,example-catalog:catalog-seed-0001:seed"
	if strings.Join(got, ",") != want {
		t.Fatalf("lifecycle records = %v, want %s", got, want)
	}
}
