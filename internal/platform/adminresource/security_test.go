package adminresource

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"platform-go/internal/platform/capability"
	"platform-go/internal/platform/core"
)

func TestStoreCreateAndUpdateRejectUndeclaredValuesBeforeSave(t *testing.T) {
	store := NewStoreFromCapabilities(core.DefaultManifests())
	if _, err := store.Create("tenants", WriteInput{
		Code: "blocked-create", Name: "Blocked", Values: map[string]string{"password": "marker-secret"},
	}); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("Create() error = %v, want ErrInvalidRecord", err)
	}
	records, err := store.List("tenants")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(fmt.Sprint(records), "marker-secret") {
		t.Fatal("rejected create value persisted")
	}

	if _, err := store.Update("tenants", "tenant-platform", WriteInput{
		Code: "platform", Name: "Platform Tenant", Values: map[string]string{"isolation": "shared", "password": "marker-update-secret"},
	}); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("Update() error = %v, want ErrInvalidRecord", err)
	}
	records, err = store.List("tenants")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(fmt.Sprint(records), "marker-update-secret") {
		t.Fatal("rejected update value persisted")
	}

	if _, err := store.Update("settings", "setting-branding", WriteInput{
		Name: "Branding Settings", Status: "enabled", Values: map[string]string{"supportEmail": "security@example.test"},
	}); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("Update(settings supportEmail) error = %v, want ErrInvalidRecord", err)
	}
}

func TestStoreInternalWriteAllowsDeclaredDerivedValuesAndRejectsRawSecrets(t *testing.T) {
	store := NewStoreFromCapabilities(core.DefaultManifests())
	record, err := store.CreateInternal("api-tokens", WriteInput{
		Code: "pgo_test", Name: "Internal Token", Status: "active",
		Values: map[string]string{
			"scope": "admin:tenant:read", "tokenPrefix": "pgo_test", "tokenHash": "derived-token-hash", "createdAt": "2026-07-12T00:00:00Z",
		},
	})
	if err != nil {
		t.Fatalf("CreateInternal() error = %v", err)
	}
	if record.Values["tokenHash"] != "derived-token-hash" {
		t.Fatalf("tokenHash = %q, want derived hash", record.Values["tokenHash"])
	}
	if _, err := store.CreateInternal("api-tokens", WriteInput{
		Code: "pgo_raw", Name: "Raw Token", Status: "active",
		Values: map[string]string{"scope": "admin:tenant:read", "token": "marker-raw-token"},
	}); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("CreateInternal(raw token) error = %v, want ErrInvalidRecord", err)
	}
}

func TestStoreExternalWriteRejectsProtectedRecordFields(t *testing.T) {
	store := NewStoreFromCapabilities(core.DefaultManifests())
	if _, err := store.Create("files", WriteInput{Code: "physical-object-key", Name: "report.txt"}); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("Create(files with code) error = %v, want ErrInvalidRecord", err)
	}
}

func TestStoreRejectsHashNamedFieldWithoutProtectedPolicy(t *testing.T) {
	store := NewStoreFromCapabilities(core.DefaultManifests())
	schema := store.schemas["api-tokens"]
	for index := range schema.Fields {
		if schema.Fields[index].Key == "tokenHash" {
			schema.Fields[index].Sensitivity = capability.FieldSensitivityPublic
			schema.Fields[index].StorageMode = capability.FieldStoragePlain
			schema.Fields[index].ResponseMode = capability.FieldProjectionFull
			schema.Fields[index].ExportMode = capability.FieldProjectionFull
		}
	}
	store.schemas["api-tokens"] = schema
	_, err := store.CreateInternal("api-tokens", WriteInput{
		Name: "Invalid Token", Values: map[string]string{"scope": "admin:tenant:read", "tokenHash": "marker-token-hash"},
	})
	if !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("CreateInternal(api-tokens) error = %v, want ErrInvalidRecord", err)
	}
}

func TestStoreRejectsPlainPersonalAndMalformedMaskedValues(t *testing.T) {
	store := NewStoreFromCapabilities(core.DefaultManifests())
	schema := store.schemas["tenants"]
	for index := range schema.Fields {
		if schema.Fields[index].Key == "isolation" {
			schema.Fields[index].Sensitivity = capability.FieldSensitivityPersonal
			schema.Fields[index].StorageMode = capability.FieldStoragePlain
			schema.Fields[index].ResponseMode = capability.FieldProjectionOmitted
			schema.Fields[index].ExportMode = capability.FieldProjectionOmitted
		}
	}
	store.schemas["tenants"] = schema
	if err := store.validateWriteValues("tenants", map[string]string{"isolation": "private-value"}, WriteOriginInternal); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("validateWriteValues(personal plain) error = %v, want ErrInvalidRecord", err)
	}

	store = NewStoreFromCapabilities(core.DefaultManifests())
	if err := store.validateWriteValues("app-phone-bindings", map[string]string{
		"maskedPhone": "13800138000",
	}, WriteOriginInternal); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("validateWriteValues(unmasked maskedPhone) error = %v, want ErrInvalidRecord", err)
	}

	for _, tt := range []struct {
		name   string
		mutate func(*FieldDefinition)
	}{
		{name: "public sensitivity", mutate: func(field *FieldDefinition) { field.Sensitivity = capability.FieldSensitivityPublic }},
		{name: "full response", mutate: func(field *FieldDefinition) { field.ResponseMode = capability.FieldProjectionFull }},
		{name: "full export", mutate: func(field *FieldDefinition) { field.ExportMode = capability.FieldProjectionFull }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			store := NewStoreFromCapabilities(core.DefaultManifests())
			schema := store.schemas["app-phone-bindings"]
			for index := range schema.Fields {
				if schema.Fields[index].Key == "maskedPhone" {
					tt.mutate(&schema.Fields[index])
				}
			}
			store.schemas["app-phone-bindings"] = schema
			if err := store.validateWriteValues("app-phone-bindings", map[string]string{"maskedPhone": "138****8000"}, WriteOriginInternal); !errors.Is(err, ErrInvalidRecord) {
				t.Fatalf("validateWriteValues(maskedPhone) error = %v, want ErrInvalidRecord", err)
			}
		})
	}
}

func TestStoreRejectsCredentialLikeCompoundNamesWithoutProtectedPolicy(t *testing.T) {
	store := NewStoreFromCapabilities(core.DefaultManifests())
	schema := store.schemas["tenants"]
	schema.Fields = append(schema.Fields,
		FieldDefinition{Key: "apiToken", Source: "values", Sensitivity: capability.FieldSensitivityPublic, StorageMode: capability.FieldStoragePlain, ResponseMode: capability.FieldProjectionFull, ExportMode: capability.FieldProjectionFull},
		FieldDefinition{Key: "authSecret", Source: "values", Sensitivity: capability.FieldSensitivityPublic, StorageMode: capability.FieldStoragePlain, ResponseMode: capability.FieldProjectionFull, ExportMode: capability.FieldProjectionFull},
		FieldDefinition{Key: "adminSessionId", Source: "values", Sensitivity: capability.FieldSensitivityPublic, StorageMode: capability.FieldStoragePlain, ResponseMode: capability.FieldProjectionFull, ExportMode: capability.FieldProjectionFull},
		FieldDefinition{Key: "maskedPassword", Source: "values", Sensitivity: capability.FieldSensitivityPublic, StorageMode: capability.FieldStoragePlain, ResponseMode: capability.FieldProjectionFull, ExportMode: capability.FieldProjectionFull},
	)
	store.schemas["tenants"] = schema
	for key, value := range map[string]string{
		"apiToken": "raw-token-marker", "authSecret": "raw-secret-marker",
		"adminSessionId": "raw-session-marker", "maskedPassword": "not-actually-masked",
	} {
		if err := store.validateWriteValues("tenants", map[string]string{key: value}, WriteOriginInternal); !errors.Is(err, ErrInvalidRecord) {
			t.Fatalf("validateWriteValues(%s) error = %v, want ErrInvalidRecord", key, err)
		}
	}
	for index := range schema.Fields {
		if schema.Fields[index].Key == "apiToken" {
			schema.Fields[index].StorageMode = capability.FieldStorageHashed
			schema.Fields[index].ResponseMode = capability.FieldProjectionOmitted
			schema.Fields[index].ExportMode = capability.FieldProjectionOmitted
		}
	}
	store.schemas["tenants"] = schema
	if err := store.validateWriteValues("tenants", map[string]string{"apiToken": "derived-token-hash"}, WriteOriginInternal); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("validateWriteValues(public hashed apiToken) error = %v, want ErrInvalidRecord", err)
	}
}

func TestStoreRejectsUnsupportedPoliciesAndSensitiveRecordStorage(t *testing.T) {
	for _, tt := range []struct {
		name   string
		mutate func(*FieldDefinition)
	}{
		{name: "sensitivity", mutate: func(field *FieldDefinition) { field.Sensitivity = "classified" }},
		{name: "storage mode", mutate: func(field *FieldDefinition) { field.StorageMode = "vault" }},
		{name: "response mode", mutate: func(field *FieldDefinition) { field.ResponseMode = "redacted" }},
		{name: "export mode", mutate: func(field *FieldDefinition) { field.ExportMode = "redacted" }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			store := NewStoreFromCapabilities(core.DefaultManifests())
			schema := store.schemas["tenants"]
			for index := range schema.Fields {
				if schema.Fields[index].Key == "isolation" {
					tt.mutate(&schema.Fields[index])
				}
			}
			store.schemas["tenants"] = schema
			if err := store.validateWriteValues("tenants", map[string]string{"isolation": "shared"}, WriteOriginInternal); !errors.Is(err, ErrInvalidRecord) {
				t.Fatalf("validateWriteValues() error = %v, want ErrInvalidRecord", err)
			}
		})
	}

	store := NewStoreFromCapabilities(core.DefaultManifests())
	schema := store.schemas["tenants"]
	for index := range schema.Fields {
		if schema.Fields[index].Key == "name" {
			schema.Fields[index].Sensitivity = capability.FieldSensitivitySensitive
			schema.Fields[index].StorageMode = capability.FieldStorageHashed
			schema.Fields[index].ResponseMode = capability.FieldProjectionOmitted
			schema.Fields[index].ExportMode = capability.FieldProjectionOmitted
		}
	}
	store.schemas["tenants"] = schema
	if err := store.validateWriteInput("tenants", WriteInput{Name: "derived-name-hash"}, WriteOriginInternal); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("validateWriteInput(sensitive record field) error = %v, want ErrInvalidRecord", err)
	}
}

func TestStoreRejectsCredentialNamesWithRepeatedDerivedSuffixes(t *testing.T) {
	store := NewStoreFromCapabilities(core.DefaultManifests())
	schema := store.schemas["tenants"]
	schema.Fields = append(schema.Fields, FieldDefinition{
		Key: "apiTokenHashDigest", Source: "values", Sensitivity: capability.FieldSensitivityPublic,
		StorageMode: capability.FieldStoragePlain, ResponseMode: capability.FieldProjectionFull, ExportMode: capability.FieldProjectionFull,
	})
	store.schemas["tenants"] = schema
	if err := store.validateWriteValues("tenants", map[string]string{"apiTokenHashDigest": "raw-token-marker"}, WriteOriginInternal); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("validateWriteValues(apiTokenHashDigest) error = %v, want ErrInvalidRecord", err)
	}
}

func TestProjectRecordDropsLegacyUnknownAndResponseOmittedValues(t *testing.T) {
	store := NewStoreFromCapabilities(core.DefaultManifests())
	legacyRecord := Record{
		ID: "verification-legacy", Code: "verification-legacy", Name: "Legacy", Status: "pending",
		Values: map[string]string{
			"maskedPhone": "138****0000", "phoneHash": "derived-phone-hash", "codeHash": "derived-code-hash", "legacyUnknown": "marker-unknown",
		},
	}
	projected, err := store.ProjectRecord("app-phone-verifications", legacyRecord, ProjectionResponse)
	if err != nil {
		t.Fatal(err)
	}
	if projected.Values["maskedPhone"] != "138****0000" {
		t.Fatalf("maskedPhone = %q, want masked value", projected.Values["maskedPhone"])
	}
	for _, key := range []string{"phoneHash", "codeHash", "legacyUnknown"} {
		if _, ok := projected.Values[key]; ok {
			t.Fatalf("%s exposed in response projection", key)
		}
	}

	exported, err := store.ProjectRecord("app-phone-verifications", legacyRecord, ProjectionExport)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"phoneHash", "codeHash", "legacyUnknown"} {
		if _, ok := exported.Values[key]; ok {
			t.Fatalf("%s exposed in export projection", key)
		}
	}
}

func TestProjectRecordAppliesRecordFieldPolicyAndRejectsUnknownMode(t *testing.T) {
	store := NewStoreFromCapabilities(core.DefaultManifests())
	schema := store.schemas["tenants"]
	for index := range schema.Fields {
		if schema.Fields[index].Key == "code" {
			schema.Fields[index].ResponseMode = capability.FieldProjectionOmitted
		}
	}
	store.schemas["tenants"] = schema
	projected, err := store.ProjectRecord("tenants", Record{ID: "tenant-1", Code: "secret-code", Name: "Tenant"}, ProjectionResponse)
	if err != nil {
		t.Fatal(err)
	}
	if projected.Code != "" || projected.Name != "Tenant" || projected.ID != "tenant-1" {
		t.Fatalf("projected record = %+v, want omitted code and preserved identity/name", projected)
	}

	schema = store.schemas["tenants"]
	for index := range schema.Fields {
		if schema.Fields[index].Key == "name" {
			schema.Fields[index].ResponseMode = "unknown-mode"
		}
	}
	store.schemas["tenants"] = schema
	if _, err := store.ProjectRecord("tenants", Record{ID: "tenant-1", Name: "Tenant"}, ProjectionResponse); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("ProjectRecord(unknown mode) error = %v, want ErrInvalidRecord", err)
	}
}

func TestPersistBoundaryRejectsInvalidDirectSnapshotWrites(t *testing.T) {
	repository := &securityRecordingRepository{snapshot: ResourceSnapshot{Resources: map[string][]Record{}}}
	store := NewStoreFromCapabilities(core.DefaultManifests())
	store.repository = repository

	_, err := store.ApplyDemoDataSet(capability.DemoDataSet{
		ID: "malicious-demo", Resource: "tenants",
		Records: []capability.DemoRecord{{ID: "tenant-malicious", Code: "malicious", Name: "Malicious", Status: "enabled", Values: map[string]string{"password": "marker-demo-secret"}}},
	})
	if !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("ApplyDemoDataSet() error = %v, want ErrInvalidRecord", err)
	}
	if repository.saveCount != 0 {
		t.Fatalf("repository saveCount = %d, want 0", repository.saveCount)
	}
	records, listErr := store.List("tenants")
	if listErr != nil {
		t.Fatal(listErr)
	}
	if strings.Contains(fmt.Sprint(records), "marker-demo-secret") {
		t.Fatal("invalid direct snapshot mutation was not rolled back")
	}
}

func TestPersistBoundaryRejectsPolicyReviewDirectSnapshotMutation(t *testing.T) {
	repository := &securityRecordingRepository{snapshot: ResourceSnapshot{Resources: map[string][]Record{}}}
	store := NewStoreFromCapabilities(core.DefaultManifests())
	store.repository = repository

	review, err := store.Create("policy-reviews", WriteInput{
		Code: "PR-SECURITY-1001", Name: "Security boundary review", Status: "enabled",
		Values: map[string]string{
			"policyType": "role_permission", "requestedAction": "update", "reviewStatus": "pending",
			"roleCode": "operator", "permissionCodes": "admin:user:read", "requestedBy": "admin",
		},
	})
	if err != nil {
		t.Fatalf("Create(policy-reviews) error = %v", err)
	}
	for index := range repository.snapshot.Resources["audit-logs"] {
		delete(repository.snapshot.Resources["audit-logs"][index].Values, "eventId")
	}
	removeSecuritySchemaField(store, "audit-logs", "eventId")
	repository.saveCount = 0
	wantSnapshot := cloneSecuritySnapshot(repository.snapshot)

	_, err = store.ApprovePolicyReview(review.ID, "admin")
	if !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("ApprovePolicyReview() error = %v, want ErrInvalidRecord", err)
	}
	if !strings.Contains(err.Error(), "eventId") {
		t.Fatalf("ApprovePolicyReview() error = %v, want final snapshot rejection for eventId", err)
	}
	if repository.saveCount != 0 {
		t.Fatalf("repository saveCount = %d, want 0", repository.saveCount)
	}
	if got := store.snapshotLocked(); !reflect.DeepEqual(got, wantSnapshot) {
		t.Fatalf("store snapshot changed after rejected policy-review mutation\ngot:  %+v\nwant: %+v", got, wantSnapshot)
	}
	if hasSecurityRecordCode(store.resources["audit-logs"], "policy-review:PR-SECURITY-1001:approved") {
		t.Fatal("rejected policy-review audit remained in memory")
	}
}

func TestPersistBoundaryRejectsAdminIdentityDirectSnapshotMutation(t *testing.T) {
	repository := &securityRecordingRepository{snapshot: ResourceSnapshot{Resources: map[string][]Record{}}}
	store := NewStoreFromCapabilities(core.DefaultManifests())
	store.repository = repository
	input := AdminIdentityBindingProvisionInput{
		Key: AdminIdentityBindingKey{
			Provider: "oidc", ProviderKind: "oidc",
			IssuerHash: strings.Repeat("a", 64), ProviderSubjectHash: strings.Repeat("b", 64),
		},
		PlatformUsername: "admin",
		Now:              time.Date(2026, time.July, 12, 8, 0, 0, 0, time.UTC),
	}
	created, err := store.ProvisionAdminIdentityBinding(context.Background(), input)
	if err != nil {
		t.Fatalf("ProvisionAdminIdentityBinding() error = %v", err)
	}
	for index := range repository.snapshot.Resources[adminIdentitiesResource] {
		if repository.snapshot.Resources[adminIdentitiesResource][index].ID == created.RecordID {
			delete(repository.snapshot.Resources[adminIdentitiesResource][index].Values, adminIdentityBindingLastLoginAtField)
		}
	}
	removeSecuritySchemaField(store, adminIdentitiesResource, adminIdentityBindingLastLoginAtField)
	repository.saveCount = 0
	wantSnapshot := cloneSecuritySnapshot(repository.snapshot)

	_, err = store.ResolveAdminIdentityBinding(context.Background(), AdminIdentityBindingResolveInput{
		Key: input.Key,
		Now: time.Date(2026, time.July, 12, 9, 0, 0, 0, time.UTC),
	})
	if !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("ResolveAdminIdentityBinding() error = %v, want ErrInvalidRecord", err)
	}
	if !strings.Contains(err.Error(), adminIdentityBindingLastLoginAtField) {
		t.Fatalf("ResolveAdminIdentityBinding() error = %v, want final snapshot rejection for %s", err, adminIdentityBindingLastLoginAtField)
	}
	if repository.saveCount != 0 {
		t.Fatalf("repository saveCount = %d, want 0", repository.saveCount)
	}
	if got := store.snapshotLocked(); !reflect.DeepEqual(got, wantSnapshot) {
		t.Fatalf("store snapshot changed after rejected identity mutation\ngot:  %+v\nwant: %+v", got, wantSnapshot)
	}
}

func TestRepositoryLoadScrubsLegacyUnknownAndProhibitedValues(t *testing.T) {
	repository := &securityRecordingRepository{snapshot: ResourceSnapshot{
		Revision: 3,
		Resources: map[string][]Record{
			"app-phone-verifications": {{
				ID: "verification-legacy", Code: "verification-legacy", Name: "Legacy", Status: "pending",
				Values: map[string]string{
					"maskedPhone": "138****0000", "phoneHash": "derived-phone-hash", "codeHash": "derived-code-hash",
					"phone": "13800000000", "password": "marker-secret", "legacyUnknown": "marker-unknown",
				},
			}},
			"audit-logs": {{
				ID: "audit-legacy", Code: "legacy", Name: "Legacy Audit", Status: "recorded",
				Values: map[string]string{"actor": "admin", "action": "auth.login", "resource": "auth", "sessionId": "raw-session-marker", "targetName": "personal-name-marker"},
			}},
			"settings": {{
				ID: "setting-branding", Code: "branding", Name: "Branding Settings", Status: "enabled",
				Values: map[string]string{"productName": "Platform Go", "supportEmail": "legacy-email@example.test"},
			}},
			"files": {{
				ID: "file-legacy", Code: "file-legacy", Name: "Legacy File", Status: "enabled",
				Values: map[string]string{"storageKey": "safe-object-key", "storagePath": "/private/raw-path-marker", "publicUrl": "https://public.example.test/raw-url-marker"},
			}},
		},
	}}
	store := NewStoreFromCapabilities(core.DefaultManifests())
	store.repository = repository
	if err := store.Reload(); err != nil {
		t.Fatalf("Reload() error = %v", err)
	}

	records, err := store.List("app-phone-verifications")
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("records = %+v, want one legacy record", records)
	}
	values := records[0].Values
	for _, key := range []string{"phone", "password", "legacyUnknown"} {
		if _, ok := values[key]; ok {
			t.Fatalf("legacy field %s survived scrub", key)
		}
	}
	if values["phoneHash"] != "derived-phone-hash" || values["codeHash"] != "derived-code-hash" {
		t.Fatalf("declared derived values were removed: %+v", values)
	}
	if repository.saveCount != 1 {
		t.Fatalf("repository saveCount = %d, want one containment rewrite", repository.saveCount)
	}
	serialized := fmt.Sprint(repository.snapshot.Resources)
	for _, marker := range []string{"13800000000", "marker-secret", "marker-unknown", "raw-session-marker", "personal-name-marker", "legacy-email@example.test", "raw-path-marker", "raw-url-marker"} {
		if strings.Contains(serialized, marker) {
			t.Fatalf("rewritten snapshot contains prohibited marker %q", marker)
		}
	}
	if !strings.Contains(serialized, "safe-object-key") {
		t.Fatal("rewritten snapshot removed internal object key")
	}
}

func TestScrubSnapshotRemovesMalformedMaskedAndPlainPersonalValues(t *testing.T) {
	store := NewStoreFromCapabilities(core.DefaultManifests())
	tenantSchema := store.schemas["tenants"]
	for index := range tenantSchema.Fields {
		if tenantSchema.Fields[index].Key == "isolation" {
			tenantSchema.Fields[index].Sensitivity = capability.FieldSensitivityPersonal
			tenantSchema.Fields[index].StorageMode = capability.FieldStoragePlain
			tenantSchema.Fields[index].ResponseMode = capability.FieldProjectionOmitted
			tenantSchema.Fields[index].ExportMode = capability.FieldProjectionOmitted
		}
	}
	store.schemas["tenants"] = tenantSchema

	clean, changed, err := store.scrubSnapshot(ResourceSnapshot{Resources: map[string][]Record{
		"tenants":            {{ID: "tenant-legacy", Values: map[string]string{"isolation": "private-value"}}},
		"app-phone-bindings": {{ID: "binding-legacy", Values: map[string]string{"maskedPhone": "13800138000"}}},
	}})
	if err != nil {
		t.Fatalf("scrubSnapshot() error = %v", err)
	}
	if !changed {
		t.Fatal("scrubSnapshot() changed = false, want true")
	}
	if values := clean.Resources["tenants"][0].Values; len(values) != 0 {
		t.Fatalf("tenant values = %+v, want scrubbed personal plaintext", values)
	}
	if values := clean.Resources["app-phone-bindings"][0].Values; len(values) != 0 {
		t.Fatalf("phone binding values = %+v, want scrubbed malformed masked value", values)
	}
}

type securityRecordingRepository struct {
	snapshot  ResourceSnapshot
	saveCount int
}

func removeSecuritySchemaField(store *Store, resource string, key string) {
	schema := store.schemas[resource]
	fields := make([]FieldDefinition, 0, len(schema.Fields))
	for _, field := range schema.Fields {
		if field.Key != key {
			fields = append(fields, field)
		}
	}
	schema.Fields = fields
	store.schemas[resource] = schema
}

func cloneSecuritySnapshot(snapshot ResourceSnapshot) ResourceSnapshot {
	return ResourceSnapshot{
		Revision: snapshot.Revision, NextID: snapshot.NextID, Resources: cloneResourceMap(snapshot.Resources),
	}
}

func hasSecurityRecordCode(records []Record, code string) bool {
	for _, record := range records {
		if record.Code == code {
			return true
		}
	}
	return false
}

func (r *securityRecordingRepository) Load(context.Context) (ResourceSnapshot, error) {
	return ResourceSnapshot{Revision: r.snapshot.Revision, NextID: r.snapshot.NextID, Resources: cloneResourceMap(r.snapshot.Resources)}, nil
}

func (r *securityRecordingRepository) Save(_ context.Context, snapshot ResourceSnapshot) (uint64, error) {
	r.saveCount++
	snapshot.Revision++
	snapshot.Resources = cloneResourceMap(snapshot.Resources)
	r.snapshot = snapshot
	return snapshot.Revision, nil
}
