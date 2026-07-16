package sensitivemigration

import (
	"crypto/sha256"
	"encoding/hex"
	"reflect"
	"strings"
	"testing"

	"github.com/GnosiST/platform-go/internal/platform/capability"
	"github.com/GnosiST/platform-go/internal/platform/dataprotection"
)

func TestPlanHashUsesCanonicalOrderedCompleteProtectionMetadata(t *testing.T) {
	forward := Plan{Resources: []ResourcePlan{
		{
			Resource: "zeta-records", Scope: "global", SchemaVersion: 3,
			Fields: []FieldPlan{
				{Key: "zetaSecret", Policy: dataprotection.FieldPolicy{Format: "aes-256-gcm-v1", Normalization: "trim-v1", BlindIndexNamespace: "zeta-index"}},
				{Key: "alphaSecret", Policy: dataprotection.FieldPolicy{Format: "aes-256-gcm-v1", Normalization: "raw-v1"}},
			},
		},
		{
			Resource: "alpha-records", Scope: "tenant-field", TenantField: "tenantCode", SchemaVersion: 7,
			Fields: []FieldPlan{{Key: "emailSecret", Policy: dataprotection.FieldPolicy{
				Format: "aes-256-gcm-v1", Normalization: "email-v1", BlindIndexNamespace: "email-index",
			}}},
		},
	}}
	reverse := Plan{Resources: []ResourcePlan{forward.Resources[1], forward.Resources[0]}}
	reverse.Resources[1].Fields = []FieldPlan{forward.Resources[0].Fields[1], forward.Resources[0].Fields[0]}

	forwardHash := PlanHash(forward)
	if forwardHash != PlanHash(reverse) || !strings.HasPrefix(forwardHash, "sha256:") || len(forwardHash) != 71 {
		t.Fatalf("canonical hashes forward=%q reverse=%q", forwardHash, PlanHash(reverse))
	}

	changed := forward
	changed.Resources = append([]ResourcePlan(nil), forward.Resources...)
	changed.Resources[0].Fields = append([]FieldPlan(nil), forward.Resources[0].Fields...)
	changed.Resources[0].Fields[0].Policy.BlindIndexNamespace = "changed-index"
	if PlanHash(changed) == forwardHash {
		t.Fatal("plan hash did not bind complete protection policy")
	}

	canonicalJSON := []byte(`[{"resource":"alpha-records","scope":"tenant-field","tenantField":"tenantCode","schemaVersion":7,"fields":[{"key":"emailSecret","format":"aes-256-gcm-v1","normalization":"email-v1","blindIndexNamespace":"email-index"}]},{"resource":"zeta-records","scope":"global","tenantField":"","schemaVersion":3,"fields":[{"key":"alphaSecret","format":"aes-256-gcm-v1","normalization":"raw-v1","blindIndexNamespace":""},{"key":"zetaSecret","format":"aes-256-gcm-v1","normalization":"trim-v1","blindIndexNamespace":"zeta-index"}]}]`)
	digest := sha256.Sum256(canonicalJSON)
	if want := "sha256:" + hex.EncodeToString(digest[:]); forwardHash != want {
		t.Fatalf("PlanHash() = %q, want canonical JSON hash %q", forwardHash, want)
	}
}

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

func TestMigrationPlanRejectsWhitespaceEquivalentResourceKeys(t *testing.T) {
	canonical := migrationResource("duplicate-records", "global", "", []capability.AdminField{
		encryptedMigrationField("alphaSecret", "raw-v1", ""),
	})
	whitespace := migrationResource(" duplicate-records ", "global", "", []capability.AdminField{
		encryptedMigrationField("zetaSecret", "raw-v1", ""),
	})
	whitespace.PermissionPrefix = "admin:duplicate-records-alt"
	whitespace.Menu.Route = "/duplicate-records-alt"

	tests := []struct {
		name      string
		resources []capability.AdminResource
	}{
		{name: "canonical first", resources: []capability.AdminResource{canonical, whitespace}},
		{name: "whitespace first", resources: []capability.AdminResource{whitespace, canonical}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := PlanFromManifests([]capability.Manifest{{
				ID: "duplicates", Admin: capability.AdminSurface{Resources: tt.resources},
			}})
			if err == nil || !strings.Contains(err.Error(), "duplicate resource") {
				t.Fatalf("PlanFromManifests() error = %v, want canonical duplicate resource error", err)
			}
		})
	}
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
		Deletion:         &capability.AdminResourceDeletionPolicy{Mode: capability.AdminDeletionSoftDelete, PolicyVersion: 1},
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
