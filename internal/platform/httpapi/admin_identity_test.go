package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/GnosiST/platform-go/internal/platform/adminresource"
	"github.com/GnosiST/platform-go/internal/platform/capability"
	"github.com/GnosiST/platform-go/internal/platform/core"
)

func TestAdminIdentityBindingProvisionResolveAndPersistHashesOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "admin-resources.json")
	store, err := adminresource.NewRepositoryBackedStoreFromCapabilitiesWithProtection(adminresource.NewFileAdminResourceRepository(path), core.DefaultManifests(), newHTTPTestDataProtectionRuntime())
	if err != nil {
		t.Fatalf("NewRepositoryBackedStoreFromCapabilitiesWithProtection() error = %v", err)
	}
	createdAt := time.Date(2026, time.July, 11, 8, 0, 0, 0, time.UTC)
	lastLoginAt := createdAt.Add(time.Hour)
	bindings := NewResourceAdminIdentityBindingStore(store, func() time.Time { return createdAt })
	provider := adminOIDCProviderForTest()

	provisioned, err := bindings.ProvisionAdminIdentityBinding(context.Background(), AdminIdentityProvisionInput{
		Provider: provider, Issuer: " https://id.example/realms/platform ", ProviderSubject: " subject-123 ", Username: " admin ", Now: createdAt,
	})
	if err != nil || provisioned.Username != "admin" || provisioned.RecordID == "" || !provisioned.Created {
		t.Fatalf("ProvisionAdminIdentityBinding() = %+v, %v", provisioned, err)
	}

	records, err := store.List(adminIdentitiesResource)
	if err != nil || len(records) != 1 {
		t.Fatalf("List(admin-identities) = %d, %v, want one record", len(records), err)
	}
	record := records[0]
	issuerSum := sha256.Sum256([]byte("https://id.example/realms/platform"))
	if record.Values["issuerHash"] != hex.EncodeToString(issuerSum[:]) {
		t.Fatalf("issuerHash = %q, want normalized SHA-256", record.Values["issuerHash"])
	}
	if record.Values["providerSubjectHash"] != adminProviderSubjectHash(provider, "https://id.example/realms/platform", "subject-123") {
		t.Fatalf("providerSubjectHash does not match normalized tuple")
	}
	wantKeys := []string{"createdAt", "issuerHash", "lastLoginAt", "platformUsername", "provider", "providerKind", "providerSubjectHash"}
	if got := sortedStringKeys(record.Values); strings.Join(got, ",") != strings.Join(wantKeys, ",") {
		t.Fatalf("persisted value keys = %v, want %v", got, wantKeys)
	}
	serialized, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("Marshal(record) error = %v", err)
	}
	assertAdminIdentityRedacted(t, string(serialized), "https://id.example/realms/platform", "subject-123")
	persisted, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(admin resources) error = %v", err)
	}
	assertAdminIdentityRedacted(t, string(persisted), "https://id.example/realms/platform", "subject-123")

	resolved, err := bindings.ResolveAdminIdentityBinding(context.Background(), AdminIdentityBindingInput{
		Provider: provider, Issuer: "https://id.example/realms/platform", ProviderSubject: "subject-123", Now: lastLoginAt,
	})
	if err != nil || resolved.Username != "admin" {
		t.Fatalf("ResolveAdminIdentityBinding() = %+v, %v", resolved, err)
	}
	records, err = store.List(adminIdentitiesResource)
	if err != nil || records[0].Values["lastLoginAt"] != lastLoginAt.Format(time.RFC3339) {
		t.Fatalf("lastLoginAt = %q, %v, want %s", records[0].Values["lastLoginAt"], err, lastLoginAt.Format(time.RFC3339))
	}
}

func TestAdminIdentityBindingResolveDoesNotAutoCreate(t *testing.T) {
	store := adminresource.NewStoreFromCapabilities(core.DefaultManifests())
	bindings := NewResourceAdminIdentityBindingStore(store, time.Now)
	_, err := bindings.ResolveAdminIdentityBinding(context.Background(), AdminIdentityBindingInput{
		Provider: adminOIDCProviderForTest(), Issuer: "https://id.example", ProviderSubject: "missing-subject",
	})
	if !errors.Is(err, ErrAdminIdentityBindingInvalid) {
		t.Fatalf("ResolveAdminIdentityBinding() error = %v, want invalid binding", err)
	}
	records, listErr := store.List(adminIdentitiesResource)
	if listErr != nil || len(records) != 0 {
		t.Fatalf("resolve created bindings: count = %d, error = %v", len(records), listErr)
	}
}

func TestAdminIdentityBindingRejectsDisabledDuplicateAndConflictingMappings(t *testing.T) {
	provider := adminOIDCProviderForTest()
	issuer := "https://id.example"
	subject := "subject-123"
	now := time.Date(2026, time.July, 11, 9, 0, 0, 0, time.UTC)

	t.Run("disabled", func(t *testing.T) {
		store := adminresource.NewStoreFromCapabilities(core.DefaultManifests())
		createAdminBindingRecord(t, store, provider, issuer, subject, "admin", "disabled", now)
		bindings := NewResourceAdminIdentityBindingStore(store, func() time.Time { return now })
		_, err := bindings.ResolveAdminIdentityBinding(context.Background(), AdminIdentityBindingInput{Provider: provider, Issuer: issuer, ProviderSubject: subject})
		if !errors.Is(err, ErrAdminIdentityBindingInvalid) {
			t.Fatalf("ResolveAdminIdentityBinding() error = %v, want disabled rejection", err)
		}
	})

	t.Run("duplicate", func(t *testing.T) {
		store := adminresource.NewStoreFromCapabilities(core.DefaultManifests())
		createAdminBindingRecord(t, store, provider, issuer, subject, "admin", "enabled", now)
		createAdminBindingRecord(t, store, provider, issuer, subject, "admin", "enabled", now)
		bindings := NewResourceAdminIdentityBindingStore(store, func() time.Time { return now })
		_, err := bindings.ResolveAdminIdentityBinding(context.Background(), AdminIdentityBindingInput{Provider: provider, Issuer: issuer, ProviderSubject: subject})
		if !errors.Is(err, ErrAdminIdentityBindingInvalid) {
			t.Fatalf("ResolveAdminIdentityBinding() error = %v, want duplicate rejection", err)
		}
	})

	t.Run("conflict", func(t *testing.T) {
		store := adminresource.NewStoreFromCapabilities(core.DefaultManifests())
		bindings := NewResourceAdminIdentityBindingStore(store, func() time.Time { return now })
		input := AdminIdentityProvisionInput{Provider: provider, Issuer: issuer, ProviderSubject: subject, Username: "admin", Now: now}
		created, err := bindings.ProvisionAdminIdentityBinding(context.Background(), input)
		if err != nil || created.RecordID == "" || !created.Created {
			t.Fatalf("initial ProvisionAdminIdentityBinding() error = %v", err)
		}
		replayed, err := bindings.ProvisionAdminIdentityBinding(context.Background(), input)
		if err != nil || replayed.RecordID != created.RecordID || replayed.Created {
			t.Fatalf("idempotent ProvisionAdminIdentityBinding() error = %v", err)
		}
		input.Username = "ops"
		conflict, err := bindings.ProvisionAdminIdentityBinding(context.Background(), input)
		if !errors.Is(err, ErrAdminIdentityBindingInvalid) || conflict.RecordID != created.RecordID || conflict.Username != "" || conflict.Created {
			t.Fatalf("conflicting ProvisionAdminIdentityBinding() error = %v, want rejection", err)
		}
		records, err := store.List(adminIdentitiesResource)
		if err != nil || len(records) != 1 {
			t.Fatalf("conflicting provision changed records: count = %d, error = %v", len(records), err)
		}
	})
}

func TestAdminIdentityBindingResolveDoesNotRestoreBindingDisabledByConcurrentCRUD(t *testing.T) {
	repository := newRevisionAwareIdentityRepository()
	seedAdminIdentityBinding(t, repository)
	resolveStore := newAdminIdentityRepositoryStore(t, repository)
	crudStore := newAdminIdentityRepositoryStore(t, repository)
	bindings := NewResourceAdminIdentityBindingStore(resolveStore, time.Now)
	before := adminIdentityRecordFromRepository(t, repository)
	reached, release := repository.pauseNextLoad()

	result := make(chan error, 1)
	go func() {
		_, err := bindings.ResolveAdminIdentityBinding(context.Background(), AdminIdentityBindingInput{
			Provider: adminOIDCProviderForTest(), Issuer: "https://id.example", ProviderSubject: "subject-123", Now: time.Date(2026, time.July, 11, 12, 0, 0, 0, time.UTC),
		})
		result <- err
	}()
	waitForRepositoryPause(t, reached)

	records, err := crudStore.List(adminIdentitiesResource)
	if err != nil || len(records) != 1 {
		t.Fatalf("List(admin-identities) = %d, %v", len(records), err)
	}
	if _, err := crudStore.UpdateInternal(adminIdentitiesResource, records[0].ID, adminresource.WriteInput{
		Code: records[0].Code, Name: records[0].Name, Status: "disabled", Description: records[0].Description, Values: records[0].Values,
	}); err != nil {
		t.Fatalf("Update(disable admin identity) error = %v", err)
	}
	close(release)

	if err := <-result; !errors.Is(err, ErrAdminIdentityBindingInvalid) {
		t.Fatalf("ResolveAdminIdentityBinding() error = %v, want disabled binding rejection", err)
	}
	after := adminIdentityRecordFromRepository(t, repository)
	if after.Status != "disabled" {
		t.Fatalf("binding status = %q, want concurrent disable preserved", after.Status)
	}
	if after.Values["platformUsername"] != before.Values["platformUsername"] || after.Values["lastLoginAt"] != before.Values["lastLoginAt"] {
		t.Fatalf("concurrent disable changed binding mapping or lastLoginAt")
	}
}

func TestAdminIdentityBindingResolveDoesNotOverwriteBindingReassignedByConcurrentCRUD(t *testing.T) {
	repository := newRevisionAwareIdentityRepository()
	seedAdminIdentityBinding(t, repository)
	resolveStore := newAdminIdentityRepositoryStore(t, repository)
	crudStore := newAdminIdentityRepositoryStore(t, repository)
	bindings := NewResourceAdminIdentityBindingStore(resolveStore, time.Now)
	reached, release := repository.pauseNextLoad()

	type resolveResult struct {
		binding AdminIdentityBinding
		err     error
	}
	result := make(chan resolveResult, 1)
	go func() {
		binding, err := bindings.ResolveAdminIdentityBinding(context.Background(), AdminIdentityBindingInput{
			Provider: adminOIDCProviderForTest(), Issuer: "https://id.example", ProviderSubject: "subject-123", Now: time.Date(2026, time.July, 11, 12, 30, 0, 0, time.UTC),
		})
		result <- resolveResult{binding: binding, err: err}
	}()
	waitForRepositoryPause(t, reached)

	records, err := crudStore.List(adminIdentitiesResource)
	if err != nil || len(records) != 1 {
		t.Fatalf("List(admin-identities) = %d, %v", len(records), err)
	}
	values := cloneAdminIdentityValues(records[0].Values)
	values["platformUsername"] = "ops"
	if _, err := crudStore.UpdateInternal(adminIdentitiesResource, records[0].ID, adminresource.WriteInput{
		Code: records[0].Code, Name: records[0].Name, Status: records[0].Status, Description: records[0].Description, Values: values,
	}); err != nil {
		t.Fatalf("Update(reassign admin identity) error = %v", err)
	}
	close(release)

	resolved := <-result
	if resolved.err != nil || resolved.binding.Username != "ops" {
		t.Fatalf("ResolveAdminIdentityBinding() = %+v, %v, want reassigned ops binding", resolved.binding, resolved.err)
	}
	after := adminIdentityRecordFromRepository(t, repository)
	if after.Values["platformUsername"] != "ops" {
		t.Fatalf("binding username = %q, want concurrent reassignment preserved", after.Values["platformUsername"])
	}
}

func TestAdminIdentityBindingResolveRetriesRevisionConflictAndFailsClosed(t *testing.T) {
	repository := newRevisionAwareIdentityRepository()
	seedAdminIdentityBinding(t, repository)
	resolveStore := newAdminIdentityRepositoryStore(t, repository)
	crudStore := newAdminIdentityRepositoryStore(t, repository)
	bindings := NewResourceAdminIdentityBindingStore(resolveStore, time.Now)
	reached, release := repository.pauseNextSave()

	result := make(chan error, 1)
	go func() {
		_, err := bindings.ResolveAdminIdentityBinding(context.Background(), AdminIdentityBindingInput{
			Provider: adminOIDCProviderForTest(), Issuer: "https://id.example", ProviderSubject: "subject-123", Now: time.Date(2026, time.July, 11, 13, 0, 0, 0, time.UTC),
		})
		result <- err
	}()
	waitForRepositoryPause(t, reached)

	records, err := crudStore.List(adminIdentitiesResource)
	if err != nil || len(records) != 1 {
		t.Fatalf("List(admin-identities) = %d, %v", len(records), err)
	}
	if _, err := crudStore.UpdateInternal(adminIdentitiesResource, records[0].ID, adminresource.WriteInput{
		Code: records[0].Code, Name: records[0].Name, Status: "disabled", Description: records[0].Description, Values: records[0].Values,
	}); err != nil {
		t.Fatalf("Update(disable admin identity) error = %v", err)
	}
	close(release)

	if err := <-result; !errors.Is(err, ErrAdminIdentityBindingInvalid) {
		t.Fatalf("ResolveAdminIdentityBinding() error = %v, want conflict retry followed by disabled rejection", err)
	}
	if record := adminIdentityRecordFromRepository(t, repository); record.Status != "disabled" {
		t.Fatalf("binding status = %q, want disabled after revision conflict", record.Status)
	}
}

func TestAdminIdentityBindingProvisionIsAtomicAcrossStoreWrappers(t *testing.T) {
	tests := []struct {
		name          string
		firstUsername string
		wantFirstErr  error
	}{
		{name: "same mapping is idempotent", firstUsername: "admin"},
		{name: "conflicting username is rejected", firstUsername: "ops", wantFirstErr: ErrAdminIdentityBindingInvalid},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := newRevisionAwareIdentityRepository()
			firstStore := newAdminIdentityRepositoryStore(t, repository)
			secondStore := newAdminIdentityRepositoryStore(t, repository)
			firstBindings := NewResourceAdminIdentityBindingStore(firstStore, time.Now)
			secondBindings := NewResourceAdminIdentityBindingStore(secondStore, time.Now)
			reached, release := repository.pauseNextLoad()

			result := make(chan error, 1)
			go func() {
				_, err := firstBindings.ProvisionAdminIdentityBinding(context.Background(), AdminIdentityProvisionInput{
					Provider: adminOIDCProviderForTest(), Issuer: "https://id.example", ProviderSubject: "subject-123", Username: test.firstUsername,
				})
				result <- err
			}()
			waitForRepositoryPause(t, reached)
			if _, err := secondBindings.ProvisionAdminIdentityBinding(context.Background(), AdminIdentityProvisionInput{
				Provider: adminOIDCProviderForTest(), Issuer: "https://id.example", ProviderSubject: "subject-123", Username: "admin",
			}); err != nil {
				t.Fatalf("second ProvisionAdminIdentityBinding() error = %v", err)
			}
			close(release)

			firstErr := <-result
			if test.wantFirstErr == nil && firstErr != nil {
				t.Fatalf("first ProvisionAdminIdentityBinding() error = %v", firstErr)
			}
			if test.wantFirstErr != nil && !errors.Is(firstErr, test.wantFirstErr) {
				t.Fatalf("first ProvisionAdminIdentityBinding() error = %v, want %v", firstErr, test.wantFirstErr)
			}
			snapshot, err := repository.Load(context.Background())
			if err != nil {
				t.Fatalf("repository.Load() error = %v", err)
			}
			records := snapshot.Resources[adminIdentitiesResource]
			if len(records) != 1 || records[0].Values["platformUsername"] != "admin" {
				t.Fatalf("persisted bindings = %+v, want one admin mapping", records)
			}
		})
	}
}

func TestAdminIdentityBindingResolveValidatesPrincipalBeforeLastLoginUpdate(t *testing.T) {
	repository := newRevisionAwareIdentityRepository()
	seedAdminIdentityBinding(t, repository)
	crudStore := newAdminIdentityRepositoryStore(t, repository)
	if _, err := crudStore.Update("users", "user-admin", adminresource.WriteInput{
		Name: "Platform Admin", Status: "disabled", Values: map[string]string{"roles": "super-admin", "tenantCode": "platform"},
	}); err != nil {
		t.Fatalf("Update(disable user-admin) error = %v", err)
	}
	before := adminIdentityRecordFromRepository(t, repository)
	resolveStore := newAdminIdentityRepositoryStore(t, repository)
	bindings := NewResourceAdminIdentityBindingStore(resolveStore, time.Now)

	_, err := bindings.ResolveAdminIdentityBinding(context.Background(), AdminIdentityBindingInput{
		Provider: adminOIDCProviderForTest(), Issuer: "https://id.example", ProviderSubject: "subject-123", Now: time.Date(2026, time.July, 11, 14, 0, 0, 0, time.UTC),
	})
	if !errors.Is(err, ErrAdminIdentityBindingInvalid) {
		t.Fatalf("ResolveAdminIdentityBinding() error = %v, want invalid principal rejection", err)
	}
	after := adminIdentityRecordFromRepository(t, repository)
	if after.Values["lastLoginAt"] != before.Values["lastLoginAt"] {
		t.Fatalf("lastLoginAt = %q, want unchanged %q", after.Values["lastLoginAt"], before.Values["lastLoginAt"])
	}
}

func TestAdminIdentityBindingResolveRollsBackRepositorySaveFailure(t *testing.T) {
	repository := newRevisionAwareIdentityRepository()
	seedAdminIdentityBinding(t, repository)
	store := newAdminIdentityRepositoryStore(t, repository)
	bindings := NewResourceAdminIdentityBindingStore(store, time.Now)
	before := adminIdentityRecordFromRepository(t, repository)
	wantErr := errors.New("injected admin identity save failure")
	repository.failNextSaveWith(wantErr)

	_, err := bindings.ResolveAdminIdentityBinding(context.Background(), AdminIdentityBindingInput{
		Provider: adminOIDCProviderForTest(), Issuer: "https://id.example", ProviderSubject: "subject-123", Now: time.Date(2026, time.July, 11, 15, 0, 0, 0, time.UTC),
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("ResolveAdminIdentityBinding() error = %v, want injected save failure", err)
	}
	after := adminIdentityRecordFromRepository(t, repository)
	if after.Values["lastLoginAt"] != before.Values["lastLoginAt"] {
		t.Fatalf("persisted lastLoginAt changed after failed save")
	}
	records, listErr := store.List(adminIdentitiesResource)
	if listErr != nil || len(records) != 1 || records[0].Values["lastLoginAt"] != before.Values["lastLoginAt"] {
		t.Fatalf("store snapshot was not rolled back after failed save: %+v, %v", records, listErr)
	}
}

func TestAdminIdentityBindingProvisionRollsBackRepositorySaveFailure(t *testing.T) {
	repository := newRevisionAwareIdentityRepository()
	store := newAdminIdentityRepositoryStore(t, repository)
	bindings := NewResourceAdminIdentityBindingStore(store, time.Now)
	provider := adminOIDCProviderForTest()
	issuer := "https://sensitive-id.example/provision"
	subject := "sensitive-provision-subject"
	username := "admin"
	before, err := repository.Load(context.Background())
	if err != nil {
		t.Fatalf("repository.Load(before) error = %v", err)
	}
	wantErr := errors.New("injected persistence failure")
	repository.failNextSaveWith(wantErr)

	_, err = bindings.ProvisionAdminIdentityBinding(context.Background(), AdminIdentityProvisionInput{
		Provider: provider, Issuer: issuer, ProviderSubject: subject, Username: username, Now: time.Date(2026, time.July, 11, 15, 30, 0, 0, time.UTC),
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("ProvisionAdminIdentityBinding() error = %v, want injected save failure", err)
	}
	assertAdminIdentityRedacted(t, err.Error(), issuer, subject, adminProviderSubjectHash(provider, issuer, subject), username)

	persisted, loadErr := repository.Load(context.Background())
	if loadErr != nil {
		t.Fatalf("repository.Load(after failure) error = %v", loadErr)
	}
	if len(persisted.Resources[adminIdentitiesResource]) != 0 || persisted.NextID != before.NextID {
		t.Fatalf("repository changed after failed provision: bindings = %+v, nextID = %d, want %d", persisted.Resources[adminIdentitiesResource], persisted.NextID, before.NextID)
	}
	records, listErr := store.List(adminIdentitiesResource)
	if listErr != nil || len(records) != 0 {
		t.Fatalf("store retained failed provision: bindings = %+v, error = %v", records, listErr)
	}

	if _, err := bindings.ProvisionAdminIdentityBinding(context.Background(), AdminIdentityProvisionInput{
		Provider: provider, Issuer: issuer, ProviderSubject: subject, Username: username, Now: time.Date(2026, time.July, 11, 15, 31, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("ProvisionAdminIdentityBinding(retry) error = %v", err)
	}
	persisted, loadErr = repository.Load(context.Background())
	if loadErr != nil {
		t.Fatalf("repository.Load(after retry) error = %v", loadErr)
	}
	records = persisted.Resources[adminIdentitiesResource]
	if len(records) != 1 || records[0].ID != "admin-identities-1001" || persisted.NextID != 1001 {
		t.Fatalf("retry persistence = %+v, nextID = %d, want first binding at 1001", records, persisted.NextID)
	}
}

func TestAdminAuthReadinessIsDataAwareAndRedacted(t *testing.T) {
	ctx := context.Background()
	provider := adminOIDCProviderForTest()
	manifest := capability.Manifest{ID: "admin-oidc", AuthProviders: []capability.AuthProvider{provider}}

	if err := ValidateAdminAuthReadiness(ctx, nil, nil, false); err != nil {
		t.Fatalf("ValidateAdminAuthReadiness() with demo enabled error = %v", err)
	}
	if err := ValidateAdminAuthReadiness(ctx, nil, nil, true); !errors.Is(err, ErrAdminAuthNotReady) {
		t.Fatalf("ValidateAdminAuthReadiness() without provider error = %v", err)
	}
	demoManifest := capability.Manifest{ID: "session", AuthProviders: []capability.AuthProvider{{
		ID: "demo", Kind: "demo", Enabled: true, Configured: true, Audiences: []capability.AuthProviderAudience{capability.AuthProviderAudienceAdmin},
	}}}
	if err := ValidateAdminAuthReadiness(ctx, []capability.Manifest{demoManifest}, nil, true); !errors.Is(err, ErrAdminAuthNotReady) {
		t.Fatalf("disabled demo provider satisfied readiness: %v", err)
	}

	store := adminresource.NewStoreFromCapabilities(core.DefaultManifests())
	bindings := NewResourceAdminIdentityBindingStore(store, time.Now)
	if err := ValidateAdminAuthReadiness(ctx, []capability.Manifest{manifest}, bindings, true); !errors.Is(err, ErrAdminAuthNotReady) {
		t.Fatalf("OIDC without binding readiness error = %v", err)
	}
	secretIssuer := "https://sensitive-id.example/tenant"
	secretSubject := "sensitive-subject-marker"
	secretUsername := "missing-sensitive-user"
	createAdminBindingRecord(t, store, provider, secretIssuer, secretSubject, secretUsername, "enabled", time.Now().UTC())
	err := ValidateAdminAuthReadiness(ctx, []capability.Manifest{manifest}, bindings, true)
	if !errors.Is(err, ErrAdminAuthNotReady) {
		t.Fatalf("OIDC invalid principal readiness error = %v", err)
	}
	assertAdminIdentityRedacted(t, err.Error(), secretIssuer, secretSubject, secretUsername, adminProviderSubjectHash(provider, secretIssuer, secretSubject))

	validStore := adminresource.NewStoreFromCapabilities(core.DefaultManifests())
	validBindings := NewResourceAdminIdentityBindingStore(validStore, time.Now)
	if _, err := validBindings.ProvisionAdminIdentityBinding(ctx, AdminIdentityProvisionInput{
		Provider: provider, Issuer: "https://id.example", ProviderSubject: "subject-123", Username: "admin",
	}); err != nil {
		t.Fatalf("ProvisionAdminIdentityBinding() error = %v", err)
	}
	if err := ValidateAdminAuthReadiness(ctx, []capability.Manifest{manifest}, validBindings, true); err != nil {
		t.Fatalf("ValidateAdminAuthReadiness() valid OIDC error = %v", err)
	}
}

func TestAdminAuthReadinessRejectsUnusableOIDCBindings(t *testing.T) {
	ctx := context.Background()
	oidcProvider := adminOIDCProviderForTest()
	manifest := capability.Manifest{ID: "admin-oidc", AuthProviders: []capability.AuthProvider{oidcProvider}}
	now := time.Date(2026, time.July, 11, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name  string
		setup func(*testing.T, *adminresource.Store)
	}{
		{
			name: "disabled binding",
			setup: func(t *testing.T, store *adminresource.Store) {
				createAdminBindingRecord(t, store, oidcProvider, "https://id.example", "subject-123", "admin", "disabled", now)
			},
		},
		{
			name: "disabled user",
			setup: func(t *testing.T, store *adminresource.Store) {
				createAdminBindingRecord(t, store, oidcProvider, "https://id.example", "subject-123", "admin", "enabled", now)
				if _, err := store.Update("users", "user-admin", adminresource.WriteInput{
					Name: "Platform Admin", Status: "disabled", Values: map[string]string{"roles": "super-admin", "tenantCode": "platform"},
				}); err != nil {
					t.Fatalf("Update(user-admin) error = %v", err)
				}
			},
		},
		{
			name: "no effective permission",
			setup: func(t *testing.T, store *adminresource.Store) {
				if _, err := store.Create("roles", adminresource.WriteInput{
					Code: "denied-role", Name: "Denied Role", Status: "enabled",
					Values: map[string]string{"dataScope": "all", "permissions": "admin:user:read", "denyPermissions": "admin:user:read"},
				}); err != nil {
					t.Fatalf("Create(denied-role) error = %v", err)
				}
				if _, err := store.Create("users", adminresource.WriteInput{
					Code: "denied", Name: "Denied User", Status: "enabled", Values: map[string]string{"roles": "denied-role", "tenantCode": "platform"},
				}); err != nil {
					t.Fatalf("Create(denied user) error = %v", err)
				}
				createAdminBindingRecord(t, store, oidcProvider, "https://id.example", "subject-123", "denied", "enabled", now)
			},
		},
		{
			name: "malformed hashes",
			setup: func(t *testing.T, store *adminresource.Store) {
				if _, err := store.CreateInternal(adminIdentitiesResource, adminresource.WriteInput{
					Code: "oidc-invalid", Name: "Admin identity binding", Status: "enabled",
					Values: map[string]string{
						"provider": "oidc", "providerKind": "oidc", "issuerHash": "not-a-sha256", "providerSubjectHash": "also-not-a-sha256",
						"platformUsername": "admin", "createdAt": now.Format(time.RFC3339), "lastLoginAt": now.Format(time.RFC3339),
					},
				}); err != nil {
					t.Fatalf("Create(malformed binding) error = %v", err)
				}
			},
		},
		{
			name: "duplicate tuple",
			setup: func(t *testing.T, store *adminresource.Store) {
				createAdminBindingRecord(t, store, oidcProvider, "https://id.example", "subject-123", "admin", "enabled", now)
				createAdminBindingRecord(t, store, oidcProvider, "https://id.example", "subject-123", "admin", "enabled", now)
			},
		},
		{
			name: "different oidc provider",
			setup: func(t *testing.T, store *adminresource.Store) {
				provider := oidcProvider
				provider.ID = "other-oidc"
				createAdminBindingRecord(t, store, provider, "https://id.example", "subject-123", "admin", "enabled", now)
			},
		},
		{
			name: "non oidc binding",
			setup: func(t *testing.T, store *adminresource.Store) {
				provider := oidcProvider
				provider.ID = "saml"
				provider.Kind = "saml"
				createAdminBindingRecord(t, store, provider, "https://id.example", "subject-123", "admin", "enabled", now)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := adminresource.NewStoreFromCapabilities(core.DefaultManifests())
			test.setup(t, store)
			bindings := NewResourceAdminIdentityBindingStore(store, func() time.Time { return now })
			if err := ValidateAdminAuthReadiness(ctx, []capability.Manifest{manifest}, bindings, true); !errors.Is(err, ErrAdminAuthNotReady) {
				t.Fatalf("ValidateAdminAuthReadiness() error = %v, want unusable OIDC binding rejection", err)
			}
		})
	}
}

func createAdminBindingRecord(t *testing.T, store *adminresource.Store, provider capability.AuthProvider, issuer string, subject string, username string, status string, now time.Time) {
	t.Helper()
	issuerSum := sha256.Sum256([]byte(strings.TrimSpace(issuer)))
	subjectHash := adminProviderSubjectHash(provider, issuer, subject)
	_, err := store.CreateInternal(adminIdentitiesResource, adminresource.WriteInput{
		Code:   strings.TrimSpace(provider.ID) + "-" + subjectHash[:12],
		Name:   "Admin identity binding",
		Status: status,
		Values: map[string]string{
			"provider":            strings.TrimSpace(provider.ID),
			"providerKind":        strings.TrimSpace(provider.Kind),
			"issuerHash":          hex.EncodeToString(issuerSum[:]),
			"providerSubjectHash": subjectHash,
			"platformUsername":    strings.TrimSpace(username),
			"createdAt":           now.UTC().Format(time.RFC3339),
			"lastLoginAt":         now.UTC().Format(time.RFC3339),
		},
	})
	if err != nil {
		t.Fatalf("Create(admin identity binding) error = %v", err)
	}
}

func adminOIDCProviderForTest() capability.AuthProvider {
	return capability.AuthProvider{
		ID:         "oidc",
		Kind:       "oidc",
		Enabled:    true,
		Configured: true,
		Audiences:  []capability.AuthProviderAudience{capability.AuthProviderAudienceAdmin},
	}
}

func sortedStringKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func assertAdminIdentityRedacted(t *testing.T, value string, sensitive ...string) {
	t.Helper()
	for _, item := range sensitive {
		if item != "" && strings.Contains(value, item) {
			t.Fatalf("value exposed sensitive admin identity data")
		}
	}
}

type repositoryPause struct {
	reached chan struct{}
	release chan struct{}
}

type revisionAwareIdentityRepository struct {
	mu           sync.Mutex
	snapshot     adminresource.ResourceSnapshot
	loadPause    *repositoryPause
	savePause    *repositoryPause
	failNextSave error
}

func newRevisionAwareIdentityRepository() *revisionAwareIdentityRepository {
	return &revisionAwareIdentityRepository{snapshot: adminresource.ResourceSnapshot{Resources: map[string][]adminresource.Record{}}}
}

func (r *revisionAwareIdentityRepository) Load(context.Context) (adminresource.ResourceSnapshot, error) {
	r.mu.Lock()
	pause := r.loadPause
	r.loadPause = nil
	r.mu.Unlock()
	if pause != nil {
		close(pause.reached)
		<-pause.release
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return cloneAdminResourceSnapshot(r.snapshot), nil
}

func (r *revisionAwareIdentityRepository) Save(_ context.Context, snapshot adminresource.ResourceSnapshot) (uint64, error) {
	r.mu.Lock()
	pause := r.savePause
	r.savePause = nil
	r.mu.Unlock()
	if pause != nil {
		close(pause.reached)
		<-pause.release
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failNextSave != nil {
		err := r.failNextSave
		r.failNextSave = nil
		return 0, err
	}
	if snapshot.Revision != r.snapshot.Revision {
		return 0, &adminresource.RevisionConflictError{Expected: snapshot.Revision, Actual: r.snapshot.Revision}
	}
	snapshot.Revision++
	r.snapshot = cloneAdminResourceSnapshot(snapshot)
	return snapshot.Revision, nil
}

func (r *revisionAwareIdentityRepository) pauseNextLoad() (<-chan struct{}, chan struct{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	pause := &repositoryPause{reached: make(chan struct{}), release: make(chan struct{})}
	r.loadPause = pause
	return pause.reached, pause.release
}

func (r *revisionAwareIdentityRepository) pauseNextSave() (<-chan struct{}, chan struct{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	pause := &repositoryPause{reached: make(chan struct{}), release: make(chan struct{})}
	r.savePause = pause
	return pause.reached, pause.release
}

func (r *revisionAwareIdentityRepository) failNextSaveWith(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failNextSave = err
}

func cloneAdminResourceSnapshot(snapshot adminresource.ResourceSnapshot) adminresource.ResourceSnapshot {
	cloned := adminresource.ResourceSnapshot{Revision: snapshot.Revision, NextID: snapshot.NextID, Resources: make(map[string][]adminresource.Record, len(snapshot.Resources))}
	for resource, records := range snapshot.Resources {
		items := make([]adminresource.Record, 0, len(records))
		for _, record := range records {
			values := make(map[string]string, len(record.Values))
			for key, value := range record.Values {
				values[key] = value
			}
			record.Values = values
			items = append(items, record)
		}
		cloned.Resources[resource] = items
	}
	return cloned
}

func cloneAdminIdentityValues(values map[string]string) map[string]string {
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func newAdminIdentityRepositoryStore(t *testing.T, repository adminresource.AdminResourceRepository) *adminresource.Store {
	t.Helper()
	store, err := adminresource.NewRepositoryBackedStoreFromCapabilitiesWithProtection(repository, core.DefaultManifests(), newHTTPTestDataProtectionRuntime())
	if err != nil {
		t.Fatalf("NewRepositoryBackedStoreFromCapabilitiesWithProtection() error = %v", err)
	}
	return store
}

func seedAdminIdentityBinding(t *testing.T, repository adminresource.AdminResourceRepository) {
	t.Helper()
	store := newAdminIdentityRepositoryStore(t, repository)
	bindings := NewResourceAdminIdentityBindingStore(store, time.Now)
	if _, err := bindings.ProvisionAdminIdentityBinding(context.Background(), AdminIdentityProvisionInput{
		Provider: adminOIDCProviderForTest(), Issuer: "https://id.example", ProviderSubject: "subject-123", Username: "admin", Now: time.Date(2026, time.July, 11, 10, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("seed ProvisionAdminIdentityBinding() error = %v", err)
	}
}

func adminIdentityRecordFromRepository(t *testing.T, repository adminresource.AdminResourceRepository) adminresource.Record {
	t.Helper()
	snapshot, err := repository.Load(context.Background())
	if err != nil {
		t.Fatalf("repository.Load() error = %v", err)
	}
	records := snapshot.Resources[adminIdentitiesResource]
	if len(records) != 1 {
		t.Fatalf("admin identity records = %+v, want one", records)
	}
	return records[0]
}

func waitForRepositoryPause(t *testing.T, reached <-chan struct{}) {
	t.Helper()
	select {
	case <-reached:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for repository pause")
	}
}
