package sensitivemigration

import (
	"reflect"
	"strings"
	"testing"

	"platform-go/internal/platform/capability"
	"platform-go/internal/platform/dataprotection"
)

func TestMigrationPlanAcceptsArbitraryValuesSourceAndRetainsMetadata(t *testing.T) {
	manifests := []capability.Manifest{
		{
			ID: "zeta",
			Admin: capability.AdminSurface{Resources: []capability.AdminResource{
				migrationResource("zeta-records", "global", "", []capability.AdminField{
					encryptedMigrationField("zetaCipher", "raw-v1", ""),
					hashedMigrationField("irreversibleDigest"),
					encryptedMigrationField("alphaCipher", "trim-v1", "alpha-cipher"),
				}),
			}},
		},
		{
			ID: "alpha",
			Admin: capability.AdminSurface{Resources: []capability.AdminResource{
				migrationResource("alpha-records", "tenant-field", "tenantCode", []capability.AdminField{
					plainMigrationField("tenantCode", true),
					encryptedMigrationField("frostedQuartz", "email-v1", "frosted-quartz"),
				}),
			}},
		},
	}

	got, err := PlanFromManifests(manifests)
	if err != nil {
		t.Fatalf("PlanFromManifests() error = %v", err)
	}
	want := Plan{Resources: []ResourcePlan{
		{
			Resource: "alpha-records", Scope: "tenant-field", TenantField: "tenantCode", SchemaVersion: 7,
			Fields: []FieldPlan{{
				Key:    "frostedQuartz",
				Policy: dataprotection.FieldPolicy{Format: "aes-256-gcm-v1", Normalization: "email-v1", BlindIndexNamespace: "frosted-quartz"},
			}},
		},
		{
			Resource: "zeta-records", Scope: "global", SchemaVersion: 7,
			Fields: []FieldPlan{
				{Key: "alphaCipher", Policy: dataprotection.FieldPolicy{Format: "aes-256-gcm-v1", Normalization: "trim-v1", BlindIndexNamespace: "alpha-cipher"}},
				{Key: "zetaCipher", Policy: dataprotection.FieldPolicy{Format: "aes-256-gcm-v1", Normalization: "raw-v1"}},
			},
		},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("PlanFromManifests() = %#v, want %#v", got, want)
	}
}

func TestMigrationPlanOrderingIsIndependentOfManifestOrder(t *testing.T) {
	alpha := capability.Manifest{ID: "alpha", Admin: capability.AdminSurface{Resources: []capability.AdminResource{
		migrationResource("alpha-records", "global", "", []capability.AdminField{encryptedMigrationField("secretValue", "raw-v1", "")}),
	}}}
	zeta := capability.Manifest{ID: "zeta", Admin: capability.AdminSurface{Resources: []capability.AdminResource{
		migrationResource("zeta-records", "global", "", []capability.AdminField{encryptedMigrationField("secretValue", "raw-v1", "")}),
	}}}

	forward, err := PlanFromManifests([]capability.Manifest{alpha, zeta})
	if err != nil {
		t.Fatalf("PlanFromManifests(forward) error = %v", err)
	}
	reverse, err := PlanFromManifests([]capability.Manifest{zeta, alpha})
	if err != nil {
		t.Fatalf("PlanFromManifests(reverse) error = %v", err)
	}
	if !reflect.DeepEqual(forward, reverse) {
		t.Fatalf("manifest order changed plan: forward = %#v reverse = %#v", forward, reverse)
	}
}

func TestMigrationPlanRejectsDuplicates(t *testing.T) {
	t.Run("resource", func(t *testing.T) {
		resource := migrationResource("duplicate-records", "global", "", []capability.AdminField{encryptedMigrationField("secretValue", "raw-v1", "")})
		_, err := PlanFromManifests([]capability.Manifest{
			{ID: "first", Admin: capability.AdminSurface{Resources: []capability.AdminResource{resource}}},
			{ID: "second", Admin: capability.AdminSurface{Resources: []capability.AdminResource{resource}}},
		})
		if err == nil || !strings.Contains(err.Error(), "already registered") {
			t.Fatalf("PlanFromManifests() error = %v, want duplicate resource error", err)
		}
	})

	t.Run("field", func(t *testing.T) {
		resource := migrationResource("duplicate-fields", "global", "", []capability.AdminField{
			encryptedMigrationField("secretValue", "raw-v1", ""),
			encryptedMigrationField("secretValue", "trim-v1", ""),
		})
		_, err := PlanFromManifests([]capability.Manifest{{ID: "duplicate", Admin: capability.AdminSurface{Resources: []capability.AdminResource{resource}}}})
		if err == nil || !strings.Contains(err.Error(), "duplicate field key") {
			t.Fatalf("PlanFromManifests() error = %v, want duplicate field error", err)
		}
	})
}

func TestMigrationPlanRejectsNoEncryptedFields(t *testing.T) {
	resource := migrationResource("plain-records", "global", "", []capability.AdminField{
		plainMigrationField("displayName", false),
		hashedMigrationField("irreversibleDigest"),
	})
	_, err := PlanFromManifests([]capability.Manifest{{ID: "plain", Admin: capability.AdminSurface{Resources: []capability.AdminResource{resource}}}})
	if err == nil || !strings.Contains(err.Error(), "no encrypted fields") {
		t.Fatalf("PlanFromManifests() error = %v, want empty plan error", err)
	}
}

func migrationResource(resource string, scope string, tenantField string, fields []capability.AdminField) capability.AdminResource {
	return capability.AdminResource{
		Resource: resource, Title: capability.Text("迁移资源", "Migration Resource"),
		Description:      capability.Text("迁移测试资源。", "Migration test resource."),
		PermissionPrefix: "admin:" + resource,
		Menu:             capability.AdminMenu{Route: "/" + resource, Group: "foundation", Icon: "overview", Order: 10},
		Fields:           fields,
		Protection:       &capability.AdminResourceProtection{SchemaVersion: 7, Scope: scope, TenantField: tenantField},
	}
}

func encryptedMigrationField(key string, normalization string, namespace string) capability.AdminField {
	return capability.AdminField{
		Key: key, Label: capability.Text("加密值", "Encrypted Value"), Type: "text", Source: "values",
		Sensitivity: capability.FieldSensitivitySensitive, StorageMode: capability.FieldStorageEncrypted,
		ResponseMode: capability.FieldProjectionPrivileged, ExportMode: capability.FieldProjectionOmitted,
		Protection: &capability.AdminFieldProtection{Format: "aes-256-gcm-v1", Normalization: normalization, BlindIndexNamespace: namespace},
	}
}

func hashedMigrationField(key string) capability.AdminField {
	return capability.AdminField{
		Key: key, Label: capability.Text("哈希值", "Hashed Value"), Type: "text", Source: "values",
		Sensitivity: capability.FieldSensitivitySecret, StorageMode: capability.FieldStorageHashed,
		ResponseMode: capability.FieldProjectionOmitted, ExportMode: capability.FieldProjectionOmitted,
	}
}

func plainMigrationField(key string, required bool) capability.AdminField {
	return capability.AdminField{Key: key, Label: capability.Text("普通值", "Plain Value"), Type: "text", Source: "values", Required: required}
}
