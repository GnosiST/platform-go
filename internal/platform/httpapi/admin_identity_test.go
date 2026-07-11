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
	"testing"
	"time"

	"platform-go/internal/platform/adminresource"
	"platform-go/internal/platform/capability"
	"platform-go/internal/platform/core"
)

func TestAdminIdentityBindingProvisionResolveAndPersistHashesOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "admin-resources.json")
	store, err := adminresource.NewFileBackedStoreFromCapabilities(path, core.DefaultManifests())
	if err != nil {
		t.Fatalf("NewFileBackedStoreFromCapabilities() error = %v", err)
	}
	createdAt := time.Date(2026, time.July, 11, 8, 0, 0, 0, time.UTC)
	lastLoginAt := createdAt.Add(time.Hour)
	bindings := NewResourceAdminIdentityBindingStore(store, func() time.Time { return createdAt })
	provider := adminOIDCProviderForTest()

	provisioned, err := bindings.ProvisionAdminIdentityBinding(context.Background(), AdminIdentityProvisionInput{
		Provider: provider, Issuer: " https://id.example/realms/platform ", ProviderSubject: " subject-123 ", Username: " admin ", Now: createdAt,
	})
	if err != nil || provisioned.Username != "admin" {
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
		if _, err := bindings.ProvisionAdminIdentityBinding(context.Background(), input); err != nil {
			t.Fatalf("initial ProvisionAdminIdentityBinding() error = %v", err)
		}
		if _, err := bindings.ProvisionAdminIdentityBinding(context.Background(), input); err != nil {
			t.Fatalf("idempotent ProvisionAdminIdentityBinding() error = %v", err)
		}
		input.Username = "ops"
		if _, err := bindings.ProvisionAdminIdentityBinding(context.Background(), input); !errors.Is(err, ErrAdminIdentityBindingInvalid) {
			t.Fatalf("conflicting ProvisionAdminIdentityBinding() error = %v, want rejection", err)
		}
		records, err := store.List(adminIdentitiesResource)
		if err != nil || len(records) != 1 {
			t.Fatalf("conflicting provision changed records: count = %d, error = %v", len(records), err)
		}
	})
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
				if _, err := store.Create(adminIdentitiesResource, adminresource.WriteInput{
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
	_, err := store.Create(adminIdentitiesResource, adminresource.WriteInput{
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
