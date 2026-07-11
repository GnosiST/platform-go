package adminresource

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

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
	for _, marker := range []string{"13800000000", "marker-secret", "marker-unknown"} {
		if strings.Contains(serialized, marker) {
			t.Fatalf("rewritten snapshot contains prohibited marker %q", marker)
		}
	}
}

type securityRecordingRepository struct {
	snapshot  ResourceSnapshot
	saveCount int
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
