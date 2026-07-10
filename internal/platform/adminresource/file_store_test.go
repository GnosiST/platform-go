package adminresource

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFileAdminResourceRepositoryLoadMissingFileReturnsEmptySnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "admin-resources.json")
	repository := NewFileAdminResourceRepository(path)

	snapshot, err := repository.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if snapshot.NextID != 0 {
		t.Fatalf("NextID = %d, want 0", snapshot.NextID)
	}
	if snapshot.Revision != 0 {
		t.Fatalf("Revision = %d, want 0", snapshot.Revision)
	}
	if snapshot.Resources == nil || len(snapshot.Resources) != 0 {
		t.Fatalf("Resources = %#v, want empty non-nil map", snapshot.Resources)
	}
}

func TestFileAdminResourceRepositoryLoadsLegacySnapshotWithZeroRevision(t *testing.T) {
	path := filepath.Join(t.TempDir(), "admin-resources.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"nextId":1007,"resources":{"tenants":[]}}`), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	snapshot, err := NewFileAdminResourceRepository(path).Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if snapshot.Revision != 0 {
		t.Fatalf("Revision = %d, want 0 for legacy JSON", snapshot.Revision)
	}
}

func TestFileAdminResourceRepositoryPersistsSnapshots(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "admin-resources.json")
	repository := NewFileAdminResourceRepository("  " + path + "  ")
	snapshot := ResourceSnapshot{
		Revision: 6,
		NextID:   2048,
		Resources: map[string][]Record{
			"tenants": {
				{
					ID:          "tenant-2048",
					Code:        "acme",
					Name:        "Acme Tenant",
					Status:      "enabled",
					Description: "File tenant",
					UpdatedAt:   "2026-07-06T00:00:00Z",
					Values:      map[string]string{"areaCode": "110000", "isolation": "sandbox"},
				},
			},
			"roles": {
				{
					ID:        "role-operator",
					Code:      "operator",
					Name:      "Operator",
					Status:    "enabled",
					UpdatedAt: "2026-07-06T00:00:00Z",
					Values:    map[string]string{"groupCode": "operations", "dataScope": "current_org", "permissions": "admin:user:read"},
				},
			},
		},
	}

	committed, err := repository.Save(context.Background(), snapshot)
	if err != nil || committed != 7 {
		t.Fatalf("Save() = %d, %v; want revision 7", committed, err)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temporary file %s exists after save or stat error = %v", path+".tmp", err)
	}

	loaded, err := repository.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.NextID != snapshot.NextID {
		t.Fatalf("NextID = %d, want %d", loaded.NextID, snapshot.NextID)
	}
	if loaded.Revision != committed {
		t.Fatalf("Revision = %d, want %d", loaded.Revision, committed)
	}
	if !reflect.DeepEqual(loaded.Resources, snapshot.Resources) {
		t.Fatalf("Resources = %#v, want %#v", loaded.Resources, snapshot.Resources)
	}
}

func TestFileAdminResourceRepositorySaveWithEmptyPathIsNoop(t *testing.T) {
	repository := NewFileAdminResourceRepository("   ")

	committed, err := repository.Save(context.Background(), ResourceSnapshot{
		Revision: 1,
		NextID:   1001,
		Resources: map[string][]Record{
			"tenants": {{ID: "tenant-1001", Code: "noop", Name: "Noop", Status: "enabled"}},
		},
	})
	if err != nil || committed != 2 {
		t.Fatalf("Save() = %d, %v; want revision 2", committed, err)
	}
}

func TestMergePersistedResourcesFiltersUnknownAndRestoresSeedMenusAndPermissions(t *testing.T) {
	base := map[string][]Record{
		"menus": {
			{ID: "menu-users", Code: "users", Name: "Users", Status: "enabled"},
			{ID: "menu-roles", Code: "roles", Name: "Roles", Status: "enabled"},
		},
		"permissions": {
			{ID: "permission-user-read", Code: "admin:user:read", Name: "Read Users", Status: "enabled"},
			{ID: "permission-role-read", Code: "admin:role:read", Name: "Read Roles", Status: "enabled"},
		},
		"tenants": {
			{ID: "tenant-platform", Code: "platform", Name: "Platform", Status: "enabled"},
		},
	}
	persisted := map[string][]Record{
		"menus": {
			{ID: "menu-users", Code: "users", Name: "Users Custom", Status: "enabled"},
		},
		"permissions": {
			{ID: "permission-user-read", Code: "admin:user:read", Name: "Read Users Custom", Status: "enabled"},
		},
		"tenants": {
			{ID: "tenant-acme", Code: "acme", Name: "Acme", Status: "enabled"},
		},
		"unknown": {
			{ID: "unknown-1", Code: "unknown", Name: "Unknown", Status: "enabled"},
		},
	}
	schemas := map[string]Schema{
		"menus":       {},
		"permissions": {},
		"tenants":     {},
	}

	merged := mergePersistedResources(base, persisted, schemas)

	if _, ok := merged["unknown"]; ok {
		t.Fatalf("merged resources include unknown resource: %+v", merged["unknown"])
	}
	if got := findRecordByCode(merged["tenants"], "acme"); got == nil {
		t.Fatalf("merged tenants = %+v, want persisted acme tenant", merged["tenants"])
	}
	if got := findRecordByCode(merged["tenants"], "platform"); got != nil {
		t.Fatalf("merged tenants kept base platform record after persisted tenant replacement: %+v", got)
	}
	if !hasRecordID(merged["menus"], "menu-users") || !hasRecordID(merged["menus"], "menu-roles") {
		t.Fatalf("merged menus = %+v, want persisted users plus base roles menu", merged["menus"])
	}
	if name := findRecordByCode(merged["menus"], "users").Name; name != "Users Custom" {
		t.Fatalf("merged users menu name = %q, want persisted custom name", name)
	}
	if !hasRecordID(merged["permissions"], "permission-user-read") || !hasRecordID(merged["permissions"], "permission-role-read") {
		t.Fatalf("merged permissions = %+v, want persisted user read plus base role read", merged["permissions"])
	}
	if len(base["menus"]) != 2 || base["menus"][0].Name != "Users" {
		t.Fatalf("base resources mutated: %+v", base["menus"])
	}
}
