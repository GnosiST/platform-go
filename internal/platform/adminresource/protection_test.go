package adminresource

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/GnosiST/platform-go/internal/platform/capability"
	"github.com/GnosiST/platform-go/internal/platform/core"
	"github.com/GnosiST/platform-go/internal/platform/dataprotection"
	"github.com/GnosiST/platform-go/internal/platform/masking"
)

func TestProtectedCreateAndUpdatePathsKeepOnlyEnvelopes(t *testing.T) {
	for _, test := range []struct {
		name   string
		create func(*Store, WriteInput) (Record, error)
		update func(*Store, string, WriteInput) (Record, error)
	}{
		{
			name: "ordinary",
			create: func(store *Store, input WriteInput) (Record, error) {
				return store.Create(protectedTestResource, input)
			},
			update: func(store *Store, id string, input WriteInput) (Record, error) {
				return store.Update(protectedTestResource, id, input)
			},
		},
		{
			name: "audited",
			create: func(store *Store, input WriteInput) (Record, error) {
				result, err := store.CreateWithAudit(protectedTestResource, input, AuditEvent{Actor: "tester", Action: "protected.create"})
				return result.Record, err
			},
			update: func(store *Store, id string, input WriteInput) (Record, error) {
				result, err := store.UpdateWithAudit(protectedTestResource, id, input, AuditEvent{Actor: "tester", Action: "protected.update"})
				return result.Record, err
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			tracking := newTrackingProtectionRuntime(t, 'e', 'i')
			store, err := NewStoreFromCapabilitiesWithProtection(protectedTestManifests(), tracking)
			if err != nil {
				t.Fatal(err)
			}
			created, err := test.create(store, protectedWriteInput("tenant-a", "marker-create-secret"))
			if err != nil {
				t.Fatalf("create error = %v", err)
			}
			assertProtectedProjection(t, created)
			stored, err := store.InternalRecord(protectedTestResource, created.ID)
			if err != nil {
				t.Fatal(err)
			}
			firstEnvelope := stored.Values[protectedTestField]
			if !dataprotection.IsEnvelope(firstEnvelope) || strings.Contains(fmt.Sprint(store.snapshotLocked()), "marker-create-secret") {
				t.Fatalf("stored value is not an opaque envelope: %q", firstEnvelope)
			}
			if len(tracking.protectContexts) != 1 || tracking.protectContexts[0].RecordID != created.ID {
				t.Fatalf("Protect contexts = %+v, want stable record ID %q", tracking.protectContexts, created.ID)
			}

			updated, err := test.update(store, created.ID, WriteInput{
				Code: "protected-1", Name: "Protected", Status: "enabled", Values: map[string]string{protectedTenantField: "tenant-a"},
			})
			if err != nil {
				t.Fatalf("update preserving omitted field error = %v", err)
			}
			assertProtectedProjection(t, updated)
			stored, _ = store.InternalRecord(protectedTestResource, created.ID)
			if stored.Values[protectedTestField] != firstEnvelope {
				t.Fatal("omitted protected value was not preserved")
			}

			updated, err = test.update(store, created.ID, protectedWriteInput("tenant-a", "marker-update-secret"))
			if err != nil {
				t.Fatalf("update submitted field error = %v", err)
			}
			assertProtectedProjection(t, updated)
			stored, _ = store.InternalRecord(protectedTestResource, created.ID)
			if stored.Values[protectedTestField] == firstEnvelope || !dataprotection.IsEnvelope(stored.Values[protectedTestField]) {
				t.Fatal("submitted protected value was not re-encrypted")
			}
			if strings.Contains(fmt.Sprint(store.snapshotLocked()), "marker-update-secret") {
				t.Fatal("updated plaintext remained in Store state")
			}
			if _, err := test.update(store, created.ID, protectedWriteInput("tenant-b", "other")); !errors.Is(err, ErrInvalidRecord) {
				t.Fatalf("tenant mutation error = %v, want ErrInvalidRecord", err)
			}
		})
	}
}

func TestProtectedStoreRejectsCiphertextAndMissingRuntime(t *testing.T) {
	runtime := newTrackingProtectionRuntime(t, 'e', 'i')
	store, err := NewStoreFromCapabilitiesWithProtection(protectedTestManifests(), runtime)
	if err != nil {
		t.Fatal(err)
	}
	created, err := store.Create(protectedTestResource, protectedWriteInput("tenant-a", "marker-secret"))
	if err != nil {
		t.Fatal(err)
	}
	stored, _ := store.InternalRecord(protectedTestResource, created.ID)
	input := protectedWriteInput("tenant-a", stored.Values[protectedTestField])
	if _, err := store.Create(protectedTestResource, input); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("Create(client envelope) error = %v, want ErrInvalidRecord", err)
	}

	if _, err := NewStoreFromCapabilitiesWithProtection(protectedTestManifests(), nil); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("NewStoreFromCapabilitiesWithProtection(nil) error = %v, want ErrInvalidRecord", err)
	}
	if _, err := NewRepositoryBackedStoreFromCapabilities(&securityRecordingRepository{snapshot: ResourceSnapshot{Resources: map[string][]Record{}}}, protectedTestManifests()); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("NewRepositoryBackedStoreFromCapabilities(encrypted manifest) error = %v, want ErrInvalidRecord", err)
	}
	legacy := NewStoreFromCapabilities(protectedTestManifests())
	if _, err := legacy.Create(protectedTestResource, protectedWriteInput("tenant-a", "plain")); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("legacy Create(encrypted manifest) error = %v, want ErrInvalidRecord", err)
	}
	if _, err := legacy.Query(protectedTestResource, QueryInput{Conditions: []QueryCondition{{Field: protectedTestField, Operator: "=", Value: "plain"}}}); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("legacy Query(encrypted manifest) error = %v, want ErrInvalidRecord", err)
	}
}

func TestProtectedConstructorEncryptsDeclaredSeedValues(t *testing.T) {
	manifests := protectedSeedManifests(t)
	runtime := newTrackingProtectionRuntime(t, 'e', 'i')
	store, err := NewStoreFromCapabilitiesWithProtection(manifests, runtime)
	if err != nil {
		t.Fatal(err)
	}

	seeded, err := store.InternalRecord("users", "user-admin")
	if err != nil {
		t.Fatal(err)
	}
	envelope := seeded.Values["roles"]
	if !dataprotection.IsEnvelope(envelope) || strings.Contains(envelope, "super-admin") {
		t.Fatalf("seeded protected value = %q, want opaque envelope", envelope)
	}
	if strings.Contains(fmt.Sprint(store.snapshotLocked().Resources["users"]), "roles:super-admin") {
		t.Fatal("seeded plaintext remained in Store state")
	}
}

func TestProtectedInternalMutationResultOmitsEncryptedEnvelope(t *testing.T) {
	runtime := newTrackingProtectionRuntime(t, 'e', 'i')
	store, err := NewStoreFromCapabilitiesWithProtection(protectedTestManifests(), runtime)
	if err != nil {
		t.Fatal(err)
	}

	created, err := store.CreateInternal(protectedTestResource, protectedWriteInput("tenant-a", "internal-secret"))
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := created.Values[protectedTestField]; exists || strings.Contains(fmt.Sprint(created), "pgo:enc:v1:") {
		t.Fatalf("internal mutation result exposed protected envelope: %+v", created)
	}
	stored, err := store.InternalRecord(protectedTestResource, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !dataprotection.IsEnvelope(stored.Values[protectedTestField]) {
		t.Fatalf("stored protected value = %q, want envelope", stored.Values[protectedTestField])
	}
}

func TestProtectedProjectionAuthorizesBeforeRevealAndBlindIndexQueryDoesNotReveal(t *testing.T) {
	runtime := newTrackingProtectionRuntime(t, 'e', 'i')
	store, err := NewStoreFromCapabilitiesWithProtection(protectedTestManifests(), runtime)
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.Create(protectedTestResource, protectedWriteInput("tenant-a", "  REF-1001  "))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(protectedTestResource, protectedWriteInput("tenant-a", "REF-2002")); err != nil {
		t.Fatal(err)
	}
	withoutSecret := protectedWriteInput("tenant-a", "unused")
	delete(withoutSecret.Values, protectedTestField)
	withoutSecret.Code = "protected-3"
	if _, err := store.Create(protectedTestResource, withoutSecret); err != nil {
		t.Fatal(err)
	}

	listed, err := store.Query(protectedTestResource, QueryInput{PageSize: 100})
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range listed.Items {
		assertProtectedProjection(t, record)
	}
	result, err := store.Query(protectedTestResource, QueryInput{Conditions: []QueryCondition{{Field: protectedTestField, Operator: "=", Value: "REF-1001"}}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 || result.Items[0].ID != first.ID {
		t.Fatalf("exact encrypted query = %+v", result)
	}
	assertProtectedProjection(t, result.Items[0])
	if runtime.matchCalls == 0 || runtime.revealCalls != 0 {
		t.Fatalf("MatchExact calls = %d, Reveal calls = %d", runtime.matchCalls, runtime.revealCalls)
	}
	if _, err := store.Query(protectedTestResource, QueryInput{Conditions: []QueryCondition{{Field: protectedTestField, Operator: "contains", Value: "REF"}}}); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("partial encrypted query error = %v, want ErrInvalidRecord", err)
	}

	internal, _ := store.InternalRecord(protectedTestResource, first.ID)
	denied := protectedAuthorizerFunc(func(context.Context, string, string, string, ProjectionPurpose) error {
		return errors.New("denied")
	})
	if _, err := store.ProjectRecordPrivileged(context.Background(), protectedTestResource, internal, ProjectionResponse, denied); err == nil {
		t.Fatal("ProjectRecordPrivileged() error = nil, want denial")
	}
	if runtime.revealCalls != 0 {
		t.Fatalf("Reveal calls after denial = %d, want 0", runtime.revealCalls)
	}
	authorized := protectedAuthorizerFunc(func(_ context.Context, resource, recordID, field string, purpose ProjectionPurpose) error {
		if resource != protectedTestResource || recordID != first.ID || field != protectedTestField || purpose != ProjectionResponse {
			t.Fatalf("authorization args = %q %q %q %q", resource, recordID, field, purpose)
		}
		return nil
	})
	projected, err := store.ProjectRecordPrivileged(context.Background(), protectedTestResource, internal, ProjectionResponse, authorized)
	if err != nil {
		t.Fatal(err)
	}
	if projected.Values[protectedTestField] != "  REF-1001  " || runtime.revealCalls != 1 {
		t.Fatalf("privileged projection = %+v, Reveal calls = %d", projected, runtime.revealCalls)
	}

	runtime.revealCalls = 0
	value, err := store.RevealProtectedField(context.Background(), ProtectedFieldRevealRequest{
		Resource: protectedTestResource, RecordID: first.ID, Field: protectedTestField, Purpose: ProtectedFieldPurposeSensitiveReveal,
	})
	if err != nil {
		t.Fatal(err)
	}
	if value != "  REF-1001  " || runtime.revealCalls != 1 {
		t.Fatalf("single field reveal = %q, Reveal calls = %d", value, runtime.revealCalls)
	}
	if _, err := store.RevealProtectedField(context.Background(), ProtectedFieldRevealRequest{
		Resource: protectedTestResource, RecordID: first.ID, Field: protectedTenantField, Purpose: ProtectedFieldPurposeSensitiveReveal,
	}); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("RevealProtectedField(plain field) error = %v, want ErrInvalidRecord", err)
	}
}

func TestProtectedDataValidationAuthenticatesWithoutRevealAndRejectsPolicyOrKeyChange(t *testing.T) {
	runtime := newTrackingProtectionRuntime(t, 'e', 'i')
	store, err := NewStoreFromCapabilitiesWithProtection(protectedTestManifests(), runtime)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(protectedTestResource, protectedWriteInput("tenant-a", "marker-history")); err != nil {
		t.Fatal(err)
	}
	if err := store.ValidateProtectedData(context.Background()); err != nil {
		t.Fatalf("ValidateProtectedData() error = %v", err)
	}
	if runtime.revealCalls != 0 {
		t.Fatalf("ValidateProtectedData() Reveal calls = %d", runtime.revealCalls)
	}

	snapshot := store.snapshotLocked()
	repository := &securityRecordingRepository{snapshot: snapshot}
	if _, err := NewRepositoryBackedStoreFromCapabilitiesWithProtection(repository, protectedTestManifests(), runtime); err != nil {
		t.Fatalf("reload matching policy error = %v", err)
	}
	changed := protectedTestManifests()
	custom := &changed[len(changed)-1].Admin.Resources[0]
	for index := range custom.Fields {
		if custom.Fields[index].Key == protectedTestField {
			custom.Fields[index].Protection.Normalization = dataprotection.NormalizationRawV1
		}
	}
	if _, err := NewRepositoryBackedStoreFromCapabilitiesWithProtection(repository, changed, runtime); err == nil {
		t.Fatal("reload changed normalization error = nil")
	}
	replaced := newTrackingProtectionRuntime(t, 'x', 'y')
	if _, err := NewRepositoryBackedStoreFromCapabilitiesWithProtection(repository, protectedTestManifests(), replaced); err == nil {
		t.Fatal("reload replaced historical key error = nil")
	}
}

func TestEncryptedMaskedProjectionUsesOneBackendStrategyAcrossResponseQueryAndExport(t *testing.T) {
	manifests := protectedMaskedTestManifests(t, masking.StrategyIdentityCNV1)
	runtime := newTrackingProtectionRuntime(t, 'e', 'i')
	store, err := NewStoreFromCapabilitiesWithProtection(manifests, runtime)
	if err != nil {
		t.Fatal(err)
	}
	maskRuntime := &trackingMaskingRuntime{delegate: masking.NewRuntime()}
	store.masking = maskRuntime

	created, err := store.Create(protectedTestResource, protectedWriteInput("tenant-a", "170101199001011204"))
	if err != nil {
		t.Fatal(err)
	}
	if got := created.Values[protectedTestField]; got != "17******04" {
		t.Fatalf("Create() masked value = %q, want 17******04", got)
	}
	internal, err := store.InternalRecord(protectedTestResource, created.ID)
	if err != nil || !dataprotection.IsEnvelope(internal.Values[protectedTestField]) || strings.Contains(internal.Values[protectedTestField], "170101199001011204") {
		t.Fatalf("internal record does not contain only an encrypted envelope: %+v", internal)
	}

	queried, err := store.Query(protectedTestResource, QueryInput{PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if got := queried.Items[0].Values[protectedTestField]; got != "17******04" {
		t.Fatalf("Query() masked value = %q, want 17******04", got)
	}
	exported, err := store.ProjectRecord(protectedTestResource, internal, ProjectionExport)
	if err != nil {
		t.Fatal(err)
	}
	if got := exported.Values[protectedTestField]; got != "17******04" {
		t.Fatalf("ProjectionExport masked value = %q, want 17******04", got)
	}
	if runtime.revealCalls != 3 {
		t.Fatalf("Reveal calls = %d, want one per response/query/export projection", runtime.revealCalls)
	}
	if maskRuntime.maskCalls != 3 {
		t.Fatalf("Mask calls = %d, want one per response/query/export projection", maskRuntime.maskCalls)
	}
	store.masking = nil
	if _, err := store.ProjectRecord(protectedTestResource, internal, ProjectionExport); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("ProjectRecord() without masking runtime error = %v, want ErrInvalidRecord", err)
	}
}

func TestProtectedMutationProjectionFailureDoesNotCommit(t *testing.T) {
	for _, test := range []struct {
		name   string
		create func(*Store, WriteInput) (Record, error)
		update func(*Store, string, WriteInput) (Record, error)
	}{
		{
			name: "ordinary",
			create: func(store *Store, input WriteInput) (Record, error) {
				return store.Create(protectedTestResource, input)
			},
			update: func(store *Store, id string, input WriteInput) (Record, error) {
				return store.Update(protectedTestResource, id, input)
			},
		},
		{
			name: "audited",
			create: func(store *Store, input WriteInput) (Record, error) {
				result, err := store.CreateWithAudit(protectedTestResource, input, AuditEvent{Actor: "tester", Action: "protected.create"})
				return result.Record, err
			},
			update: func(store *Store, id string, input WriteInput) (Record, error) {
				result, err := store.UpdateWithAudit(protectedTestResource, id, input, AuditEvent{Actor: "tester", Action: "protected.update"})
				return result.Record, err
			},
		},
	} {
		t.Run(test.name+" create", func(t *testing.T) {
			store, err := NewStoreFromCapabilitiesWithProtection(protectedMaskedTestManifests(t, masking.StrategyIdentityCNV1), newTrackingProtectionRuntime(t, 'e', 'i'))
			if err != nil {
				t.Fatal(err)
			}
			beforeID := store.nextID
			beforeSnapshot := store.snapshotLocked()
			store.masking = nil
			if _, err := test.create(store, protectedWriteInput("tenant-a", "170101199001011204")); !errors.Is(err, ErrInvalidRecord) {
				t.Fatalf("create projection error = %v, want ErrInvalidRecord", err)
			}
			afterSnapshot := store.snapshotLocked()
			if len(afterSnapshot.Resources[protectedTestResource]) != len(beforeSnapshot.Resources[protectedTestResource]) ||
				len(afterSnapshot.Resources["audit-logs"]) != len(beforeSnapshot.Resources["audit-logs"]) || store.nextID != beforeID {
				t.Fatalf("failed create committed state: records=%d audits=%d nextID=%d", len(afterSnapshot.Resources[protectedTestResource]), len(afterSnapshot.Resources["audit-logs"]), store.nextID)
			}
		})

		t.Run(test.name+" update", func(t *testing.T) {
			store, err := NewStoreFromCapabilitiesWithProtection(protectedMaskedTestManifests(t, masking.StrategyIdentityCNV1), newTrackingProtectionRuntime(t, 'e', 'i'))
			if err != nil {
				t.Fatal(err)
			}
			created, err := test.create(store, protectedWriteInput("tenant-a", "170101199001011204"))
			if err != nil {
				t.Fatal(err)
			}
			before, err := store.InternalRecord(protectedTestResource, created.ID)
			if err != nil {
				t.Fatal(err)
			}
			beforeID := store.nextID
			beforeAuditCount := len(store.snapshotLocked().Resources["audit-logs"])
			store.masking = nil
			if _, err := test.update(store, created.ID, protectedWriteInput("tenant-a", "110101199001011234")); !errors.Is(err, ErrInvalidRecord) {
				t.Fatalf("update projection error = %v, want ErrInvalidRecord", err)
			}
			after, err := store.InternalRecord(protectedTestResource, created.ID)
			if err != nil {
				t.Fatal(err)
			}
			auditCount := len(store.snapshotLocked().Resources["audit-logs"])
			if !reflect.DeepEqual(after, before) || auditCount != beforeAuditCount || store.nextID != beforeID {
				t.Fatalf("failed update committed state: before=%+v after=%+v audits=%d nextID=%d", before, after, auditCount, store.nextID)
			}
		})
	}
}

func TestSchemaCloneIsolatesMaskingMetadata(t *testing.T) {
	store, err := NewStoreFromCapabilitiesWithProtection(protectedMaskedTestManifests(t, masking.StrategyIdentityCNV1), newTrackingProtectionRuntime(t, 'e', 'i'))
	if err != nil {
		t.Fatal(err)
	}
	schema, err := store.Schema(protectedTestResource)
	if err != nil {
		t.Fatal(err)
	}
	for index := range schema.Fields {
		if schema.Fields[index].Key == protectedTestField {
			schema.Fields[index].Masking.Strategy = masking.StrategyPartialV1
			schema.Fields[index].Masking.PreservePrefix = 64
		}
	}
	fresh, err := store.Schema(protectedTestResource)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range fresh.Fields {
		if field.Key == protectedTestField && (field.Masking == nil || field.Masking.Strategy != masking.StrategyIdentityCNV1 || field.Masking.PreservePrefix != 0) {
			t.Fatalf("Schema() leaked masking metadata mutation: %+v", field.Masking)
		}
	}
}

func TestProtectedGlobalScopeUsesStableSentinel(t *testing.T) {
	manifests := protectedTestManifests()
	resource := &manifests[len(manifests)-1].Admin.Resources[0]
	resource.Protection = &capability.AdminResourceProtection{SchemaVersion: 3, Scope: "global"}
	fields := make([]capability.AdminField, 0, len(resource.Fields)-1)
	for _, field := range resource.Fields {
		if field.Key != protectedTenantField {
			fields = append(fields, field)
		}
	}
	resource.Fields = fields
	runtime := newTrackingProtectionRuntime(t, 'e', 'i')
	store, err := NewStoreFromCapabilitiesWithProtection(manifests, runtime)
	if err != nil {
		t.Fatal(err)
	}
	input := protectedWriteInput("", "global-secret")
	delete(input.Values, protectedTenantField)
	if _, err := store.Create(protectedTestResource, input); err != nil {
		t.Fatal(err)
	}
	if len(runtime.protectContexts) != 1 || runtime.protectContexts[0].TenantID != dataprotection.GlobalTenantID {
		t.Fatalf("Protect context = %+v, want global sentinel", runtime.protectContexts)
	}
}

func TestProtectedRawNormalizationPreservesExactQueryValue(t *testing.T) {
	manifests := protectedTestManifests()
	resource := &manifests[len(manifests)-1].Admin.Resources[0]
	for index := range resource.Fields {
		if resource.Fields[index].Key == protectedTestField {
			resource.Fields[index].Protection.Normalization = dataprotection.NormalizationRawV1
		}
	}
	runtime := newTrackingProtectionRuntime(t, 'e', 'i')
	store, err := NewStoreFromCapabilitiesWithProtection(manifests, runtime)
	if err != nil {
		t.Fatal(err)
	}
	created, err := store.Create(protectedTestResource, protectedWriteInput("tenant-a", " REF "))
	if err != nil {
		t.Fatal(err)
	}

	result, err := store.Query(protectedTestResource, QueryInput{Conditions: []QueryCondition{{Field: protectedTestField, Operator: "=", Value: " REF "}}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 || result.Items[0].ID != created.ID {
		t.Fatalf("raw exact query = %+v, want record %q", result, created.ID)
	}
	spaces, err := store.Create(protectedTestResource, protectedWriteInput("tenant-a", "   "))
	if err != nil {
		t.Fatal(err)
	}
	result, err = store.Query(protectedTestResource, QueryInput{Conditions: []QueryCondition{{Field: protectedTestField, Operator: "=", Value: "   "}}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 || result.Items[0].ID != spaces.ID {
		t.Fatalf("raw whitespace query = %+v, want record %q", result, spaces.ID)
	}
}

func TestProtectedFileSQLAndGORMPersistenceContainNoPlaintext(t *testing.T) {
	const marker = "marker-persistence-secret"
	manifests := protectedTestManifests()

	t.Run("file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "admin-resources.json")
		store, err := NewRepositoryBackedStoreFromCapabilitiesWithProtection(NewFileAdminResourceRepository(path), manifests, newTrackingProtectionRuntime(t, 'e', 'i'))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := store.Create(protectedTestResource, protectedWriteInput("tenant-a", marker)); err != nil {
			t.Fatal(err)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(content), marker) || !strings.Contains(string(content), "pgo:enc:v1:") {
			t.Fatalf("file snapshot does not contain only envelope data: %s", content)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("file mode = %o, want 600", info.Mode().Perm())
		}
	})

	t.Run("sql", func(t *testing.T) {
		db := openAdminResourceTestDB(t)
		repository, err := NewSQLAdminResourceRepository(context.Background(), db)
		if err != nil {
			t.Fatal(err)
		}
		store, err := NewRepositoryBackedStoreFromCapabilitiesWithProtection(repository, manifests, newTrackingProtectionRuntime(t, 'e', 'i'))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := store.Create(protectedTestResource, protectedWriteInput("tenant-a", marker)); err != nil {
			t.Fatal(err)
		}
		loaded, err := repository.Load(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		serialized := fmt.Sprint(loaded.Resources[protectedTestResource])
		if strings.Contains(serialized, marker) || !strings.Contains(serialized, "pgo:enc:v1:") {
			t.Fatalf("SQL snapshot does not contain only envelope data: %s", serialized)
		}
	})

	t.Run("gorm", func(t *testing.T) {
		db := openAdminResourceGORMDB(t)
		repository, err := NewGORMAdminResourceRepository(context.Background(), db)
		if err != nil {
			t.Fatal(err)
		}
		store, err := NewRepositoryBackedStoreFromCapabilitiesWithProtection(repository, manifests, newTrackingProtectionRuntime(t, 'e', 'i'))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := store.Create(protectedTestResource, protectedWriteInput("tenant-a", marker)); err != nil {
			t.Fatal(err)
		}
		var valuesJSON string
		if err := db.Table(adminResourceRecordsTable).Select("values_json").Where("resource = ?", protectedTestResource).Scan(&valuesJSON).Error; err != nil {
			t.Fatal(err)
		}
		if strings.Contains(valuesJSON, marker) || !strings.Contains(valuesJSON, "pgo:enc:v1:") {
			t.Fatalf("GORM values_json does not contain only envelope data: %s", valuesJSON)
		}
	})
}

const (
	protectedTestResource = "protected-records"
	protectedTenantField  = "tenantCode"
	protectedTestField    = "governmentReference"
)

func protectedTestManifests() []capability.Manifest {
	return append(core.DefaultManifests(), capability.Manifest{
		ID: "protected-test",
		Admin: capability.AdminSurface{Resources: []capability.AdminResource{{
			Resource: protectedTestResource, Title: capability.Text("受保护记录", "Protected Records"), Description: capability.Text("测试。", "Test."),
			PermissionPrefix: "admin:protected-record", Protection: &capability.AdminResourceProtection{SchemaVersion: 3, Scope: "tenant-field", TenantField: protectedTenantField},
			Fields: []capability.AdminField{
				{Key: "code", Label: capability.Text("编码", "Code"), Type: "text", Source: "record", Required: true, InForm: true},
				{Key: "name", Label: capability.Text("名称", "Name"), Type: "text", Source: "record", Required: true, InForm: true},
				{Key: "status", Label: capability.Text("状态", "Status"), Type: "text", Source: "record", InForm: true},
				{Key: protectedTenantField, Label: capability.Text("租户", "Tenant"), Type: "text", Source: "values", Required: true, InForm: true, Filterable: true},
				{Key: protectedTestField, Label: capability.Text("政府引用", "Government Reference"), Type: "text", Source: "values", InForm: true, Filterable: true,
					Sensitivity: capability.FieldSensitivitySensitive, StorageMode: capability.FieldStorageEncrypted,
					ResponseMode: capability.FieldProjectionPrivileged, ExportMode: capability.FieldProjectionOmitted,
					Protection: &capability.AdminFieldProtection{Format: dataprotection.FormatAES256GCMV1, Normalization: dataprotection.NormalizationTrimV1, BlindIndexNamespace: "protected-government-reference"}},
			},
		}}},
	})
}

func protectedMaskedTestManifests(t *testing.T, strategy string) []capability.Manifest {
	t.Helper()
	manifests := protectedTestManifests()
	resource := &manifests[len(manifests)-1].Admin.Resources[0]
	for index := range resource.Fields {
		field := &resource.Fields[index]
		if field.Key != protectedTestField {
			continue
		}
		field.ResponseMode = capability.FieldProjectionMasked
		field.ExportMode = capability.FieldProjectionMasked
		field.Masking = &capability.AdminFieldMasking{Strategy: strategy}
		return manifests
	}
	t.Fatal("protected test field was not found")
	return nil
}

func protectedSeedManifests(t *testing.T) []capability.Manifest {
	t.Helper()
	manifests := core.DefaultManifests()
	for manifestIndex := range manifests {
		for resourceIndex := range manifests[manifestIndex].Admin.Resources {
			resource := &manifests[manifestIndex].Admin.Resources[resourceIndex]
			if resource.Resource != "users" {
				continue
			}
			resource.Protection = &capability.AdminResourceProtection{SchemaVersion: 1, Scope: "tenant-field", TenantField: "tenantCode"}
			resource.SearchFields = removeString(resource.SearchFields, "roles")
			for fieldIndex := range resource.Fields {
				field := &resource.Fields[fieldIndex]
				if field.Key != "roles" {
					continue
				}
				field.Searchable = false
				field.Filterable = false
				field.Sortable = false
				field.Sensitivity = capability.FieldSensitivitySensitive
				field.StorageMode = capability.FieldStorageEncrypted
				field.ResponseMode = capability.FieldProjectionPrivileged
				field.ExportMode = capability.FieldProjectionOmitted
				field.Protection = &capability.AdminFieldProtection{Format: dataprotection.FormatAES256GCMV1, Normalization: dataprotection.NormalizationRawV1}
				return manifests
			}
		}
	}
	t.Fatal("users.roles field was not found")
	return nil
}

func removeString(values []string, target string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != target {
			result = append(result, value)
		}
	}
	return result
}

func protectedWriteInput(tenant string, secret string) WriteInput {
	return WriteInput{Code: "protected-1", Name: "Protected", Status: "enabled", Values: map[string]string{
		protectedTenantField: tenant, protectedTestField: secret,
	}}
}

func assertProtectedProjection(t *testing.T, record Record) {
	t.Helper()
	if _, ok := record.Values[protectedTestField]; ok || strings.Contains(fmt.Sprint(record), "pgo:enc:v1:") {
		t.Fatalf("ordinary projection exposed protected value: %+v", record)
	}
}

type trackingMaskingRuntime struct {
	delegate  masking.Runtime
	maskCalls int
}

func (r *trackingMaskingRuntime) Validate(policy masking.Policy) error {
	return r.delegate.Validate(policy)
}

func (r *trackingMaskingRuntime) Mask(ctx context.Context, policy masking.Policy, value string) (string, error) {
	r.maskCalls++
	return r.delegate.Mask(ctx, policy, value)
}

type trackingProtectionRuntime struct {
	delegate        dataprotection.Runtime
	protectContexts []dataprotection.FieldContext
	revealCalls     int
	matchCalls      int
}

func newTrackingProtectionRuntime(t *testing.T, encryptionByte byte, indexByte byte) *trackingProtectionRuntime {
	t.Helper()
	provider, err := dataprotection.NewStaticKeyProvider(dataprotection.StaticKeyProviderConfig{
		Kind: dataprotection.ProviderEnvAES256, ActiveEncryptionKeyID: "enc-v1", ActiveBlindIndexKeyID: "idx-v1",
		EncryptionKeys: map[string][]byte{"enc-v1": []byte(strings.Repeat(string(encryptionByte), 32))},
		BlindIndexKeys: map[string][]byte{"idx-v1": []byte(strings.Repeat(string(indexByte), 32))},
	})
	if err != nil {
		t.Fatal(err)
	}
	return &trackingProtectionRuntime{delegate: dataprotection.NewRuntime(provider)}
}

func (r *trackingProtectionRuntime) Protect(ctx context.Context, value string, policy dataprotection.FieldPolicy, fieldContext dataprotection.FieldContext) (string, error) {
	r.protectContexts = append(r.protectContexts, fieldContext)
	return r.delegate.Protect(ctx, value, policy, fieldContext)
}

func (r *trackingProtectionRuntime) Validate(ctx context.Context, value string, policy dataprotection.FieldPolicy, fieldContext dataprotection.FieldContext) error {
	return r.delegate.Validate(ctx, value, policy, fieldContext)
}

func (r *trackingProtectionRuntime) Reveal(ctx context.Context, value string, policy dataprotection.FieldPolicy, fieldContext dataprotection.FieldContext) (string, error) {
	r.revealCalls++
	return r.delegate.Reveal(ctx, value, policy, fieldContext)
}

func (r *trackingProtectionRuntime) MatchExact(ctx context.Context, envelope string, candidate string, policy dataprotection.FieldPolicy, fieldContext dataprotection.FieldContext) (bool, error) {
	r.matchCalls++
	return r.delegate.MatchExact(ctx, envelope, candidate, policy, fieldContext)
}

type protectedAuthorizerFunc func(context.Context, string, string, string, ProjectionPurpose) error

func (f protectedAuthorizerFunc) AuthorizeProtectedField(ctx context.Context, resource, recordID, field string, purpose ProjectionPurpose) error {
	return f(ctx, resource, recordID, field, purpose)
}
