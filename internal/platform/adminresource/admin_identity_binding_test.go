package adminresource

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"platform-go/internal/platform/core"
)

func TestProvisionAdminIdentityBindingReturnsStableRecordIdentity(t *testing.T) {
	store := NewStoreFromCapabilities(core.DefaultManifests())
	input := testAdminIdentityProvisionInput("admin")

	created, err := store.ProvisionAdminIdentityBinding(context.Background(), input)
	if err != nil {
		t.Fatalf("ProvisionAdminIdentityBinding(create) error = %v", err)
	}
	if created.RecordID == "" || !created.Created || created.PlatformUsername != "admin" {
		t.Fatalf("created result = %+v, want stable record identity", created)
	}

	replayed, err := store.ProvisionAdminIdentityBinding(context.Background(), input)
	if err != nil {
		t.Fatalf("ProvisionAdminIdentityBinding(replay) error = %v", err)
	}
	if replayed.RecordID != created.RecordID || replayed.Created || replayed.PlatformUsername != "admin" {
		t.Fatalf("replayed result = %+v, want existing record %q", replayed, created.RecordID)
	}

	conflict, err := store.ProvisionAdminIdentityBinding(context.Background(), testAdminIdentityProvisionInput("ops"))
	if !errors.Is(err, ErrAdminIdentityBindingInvalid) {
		t.Fatalf("ProvisionAdminIdentityBinding(conflict) error = %v, want invalid binding", err)
	}
	if conflict.RecordID != created.RecordID || conflict.PlatformUsername != "" || conflict.Created {
		t.Fatalf("conflict result = %+v, want record ID only", conflict)
	}

	missingPrincipalConflict, err := store.ProvisionAdminIdentityBinding(context.Background(), testAdminIdentityProvisionInput("missing-user"))
	if !errors.Is(err, ErrAdminIdentityBindingInvalid) || missingPrincipalConflict.RecordID != created.RecordID {
		t.Fatalf("missing principal conflict = %+v, %v, want existing record ID", missingPrincipalConflict, err)
	}
}

func TestEnsureAdminIdentityBindingAuditIsIdempotentAcrossStoreWrappers(t *testing.T) {
	repository := newAtomicIdentityAuditRepository()
	firstStore := newIdentityAuditStore(t, repository)
	created, err := firstStore.ProvisionAdminIdentityBinding(context.Background(), testAdminIdentityProvisionInput("admin"))
	if err != nil {
		t.Fatalf("ProvisionAdminIdentityBinding() error = %v", err)
	}
	now := time.Date(2026, time.July, 11, 18, 0, 0, 0, time.UTC)
	input := AdminIdentityBindingAuditInput{
		BindingRecordID: created.RecordID,
		Provider:        "oidc",
		Username:        "admin",
		Outcome:         AdminIdentityBindingAuditOutcomeBound,
		Now:             now,
	}
	if _, err := firstStore.EnsureAdminIdentityBindingAudit(context.Background(), input); err != nil {
		t.Fatalf("EnsureAdminIdentityBindingAudit(first) error = %v", err)
	}

	secondStore := newIdentityAuditStore(t, repository)
	replayed, err := secondStore.ProvisionAdminIdentityBinding(context.Background(), testAdminIdentityProvisionInput("admin"))
	if err != nil || replayed.RecordID != created.RecordID || replayed.Created {
		t.Fatalf("ProvisionAdminIdentityBinding(replay) = %+v, %v", replayed, err)
	}
	if _, err := secondStore.EnsureAdminIdentityBindingAudit(context.Background(), input); err != nil {
		t.Fatalf("EnsureAdminIdentityBindingAudit(replay) error = %v", err)
	}

	audits, err := secondStore.List("audit-logs")
	if err != nil {
		t.Fatalf("List(audit-logs) error = %v", err)
	}
	matching := matchingAdminIdentityAudits(audits, AdminIdentityBindingAuditOutcomeBound)
	if len(matching) != 1 {
		t.Fatalf("bound audits = %+v, want one", matching)
	}
	assertIdentityAuditRedacted(t, matching[0], input)
	if matching[0].Values["actor"] != "user-admin" {
		t.Fatalf("bound audit actor = %q, want stable user ID", matching[0].Values["actor"])
	}
}

func TestEnsureAdminIdentityBindingAuditRetriesAfterPersistenceFailure(t *testing.T) {
	repository := newAtomicIdentityAuditRepository()
	store := newIdentityAuditStore(t, repository)
	created, err := store.ProvisionAdminIdentityBinding(context.Background(), testAdminIdentityProvisionInput("admin"))
	if err != nil {
		t.Fatalf("ProvisionAdminIdentityBinding() error = %v", err)
	}
	input := AdminIdentityBindingAuditInput{
		BindingRecordID: created.RecordID,
		Provider:        "oidc",
		Username:        "admin",
		Outcome:         AdminIdentityBindingAuditOutcomeBound,
		Now:             time.Date(2026, time.July, 11, 18, 30, 0, 0, time.UTC),
	}
	wantErr := errors.New("injected audit persistence failure")
	repository.failNextSaveWith(wantErr)
	if _, err := store.EnsureAdminIdentityBindingAudit(context.Background(), input); !errors.Is(err, wantErr) {
		t.Fatalf("EnsureAdminIdentityBindingAudit(failure) error = %v, want injected error", err)
	}

	retryStore := newIdentityAuditStore(t, repository)
	if _, err := retryStore.EnsureAdminIdentityBindingAudit(context.Background(), input); err != nil {
		t.Fatalf("EnsureAdminIdentityBindingAudit(retry) error = %v", err)
	}
	if _, err := retryStore.EnsureAdminIdentityBindingAudit(context.Background(), input); err != nil {
		t.Fatalf("EnsureAdminIdentityBindingAudit(second retry) error = %v", err)
	}
	audits, err := retryStore.List("audit-logs")
	if err != nil {
		t.Fatalf("List(audit-logs) error = %v", err)
	}
	matching := matchingAdminIdentityAudits(audits, AdminIdentityBindingAuditOutcomeBound)
	if len(matching) != 1 {
		t.Fatalf("bound audits after retry = %+v, want one", matching)
	}
}

func TestEnsureAdminIdentityBindingConflictAuditIsRedactedAndIdempotent(t *testing.T) {
	repository := newAtomicIdentityAuditRepository()
	store := newIdentityAuditStore(t, repository)
	created, err := store.ProvisionAdminIdentityBinding(context.Background(), testAdminIdentityProvisionInput("admin"))
	if err != nil {
		t.Fatalf("ProvisionAdminIdentityBinding() error = %v", err)
	}
	conflict, err := store.ProvisionAdminIdentityBinding(context.Background(), testAdminIdentityProvisionInput("ops"))
	if !errors.Is(err, ErrAdminIdentityBindingInvalid) || conflict.RecordID != created.RecordID {
		t.Fatalf("ProvisionAdminIdentityBinding(conflict) = %+v, %v", conflict, err)
	}
	input := AdminIdentityBindingAuditInput{
		BindingRecordID: conflict.RecordID,
		Provider:        "oidc",
		Outcome:         AdminIdentityBindingAuditOutcomeConflict,
		Now:             time.Date(2026, time.July, 11, 19, 0, 0, 0, time.UTC),
	}
	if _, err := store.EnsureAdminIdentityBindingAudit(context.Background(), input); err != nil {
		t.Fatalf("EnsureAdminIdentityBindingAudit(conflict) error = %v", err)
	}
	if _, err := store.EnsureAdminIdentityBindingAudit(context.Background(), input); err != nil {
		t.Fatalf("EnsureAdminIdentityBindingAudit(conflict replay) error = %v", err)
	}
	audits, err := store.List("audit-logs")
	if err != nil {
		t.Fatalf("List(audit-logs) error = %v", err)
	}
	matching := matchingAdminIdentityAudits(audits, AdminIdentityBindingAuditOutcomeConflict)
	if len(matching) != 1 {
		t.Fatalf("conflict audits = %+v, want one", matching)
	}
	assertIdentityAuditRedacted(t, matching[0], input)
	if matching[0].Values["actor"] != "system:platform" {
		t.Fatalf("conflict audit actor = %q, want explicit system ID", matching[0].Values["actor"])
	}
	for key, value := range matching[0].Values {
		if key == "platformUsername" || value == "admin" || value == "ops" {
			t.Fatalf("conflict audit values = %+v, want no username", matching[0].Values)
		}
	}
}

func testAdminIdentityProvisionInput(username string) AdminIdentityBindingProvisionInput {
	return AdminIdentityBindingProvisionInput{
		Key: AdminIdentityBindingKey{
			Provider:            "oidc",
			ProviderKind:        "oidc",
			IssuerHash:          strings.Repeat("a", 64),
			ProviderSubjectHash: strings.Repeat("b", 64),
		},
		PlatformUsername: username,
		Now:              time.Date(2026, time.July, 11, 17, 0, 0, 0, time.UTC),
	}
}

func matchingAdminIdentityAudits(records []Record, outcome string) []Record {
	matching := make([]Record, 0)
	for _, record := range records {
		if record.Values["action"] == "admin_identity.bind" && record.Values["outcome"] == outcome {
			matching = append(matching, record)
		}
	}
	return matching
}

func assertIdentityAuditRedacted(t *testing.T, record Record, input AdminIdentityBindingAuditInput, forbidden ...string) {
	t.Helper()
	serialized := record.Code + strings.Join([]string{
		record.Values["actor"], record.Values["outcome"], record.Values["resource"], record.Values["createdAt"],
	}, "\x00")
	for _, value := range append(forbidden, strings.Repeat("a", 64), strings.Repeat("b", 64)) {
		if value != "" && strings.Contains(serialized, value) {
			t.Fatalf("audit exposed forbidden identity value")
		}
	}
	if strings.Contains(record.Code, "aaaa") || strings.Contains(record.Code, "bbbb") {
		t.Fatalf("audit code = %q, want no identity hash", record.Code)
	}
	if record.Values["provider"] != "" || record.Values["outcome"] != input.Outcome {
		t.Fatalf("audit values = %+v, want redacted structured outcome", record.Values)
	}
	for _, key := range []string{"actor", "action", "resource", "targetId", "outcome", "eventId", "reasonCode", "createdAt"} {
		if strings.TrimSpace(record.Values[key]) == "" {
			t.Fatalf("audit values = %+v, missing %s", record.Values, key)
		}
	}
}

type atomicIdentityAuditRepository struct {
	mu           sync.Mutex
	snapshot     ResourceSnapshot
	failNextSave error
}

func newAtomicIdentityAuditRepository() *atomicIdentityAuditRepository {
	return &atomicIdentityAuditRepository{snapshot: ResourceSnapshot{Resources: map[string][]Record{}}}
}

func (r *atomicIdentityAuditRepository) Load(context.Context) (ResourceSnapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return cloneIdentityAuditSnapshot(r.snapshot), nil
}

func (r *atomicIdentityAuditRepository) Save(_ context.Context, snapshot ResourceSnapshot) (uint64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failNextSave != nil {
		err := r.failNextSave
		r.failNextSave = nil
		return 0, err
	}
	if snapshot.Revision != r.snapshot.Revision {
		return 0, &RevisionConflictError{Expected: snapshot.Revision, Actual: r.snapshot.Revision}
	}
	snapshot.Revision++
	r.snapshot = cloneIdentityAuditSnapshot(snapshot)
	return snapshot.Revision, nil
}

func (r *atomicIdentityAuditRepository) failNextSaveWith(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failNextSave = err
}

func cloneIdentityAuditSnapshot(snapshot ResourceSnapshot) ResourceSnapshot {
	return ResourceSnapshot{Revision: snapshot.Revision, NextID: snapshot.NextID, Resources: cloneResourceMap(snapshot.Resources)}
}

func newIdentityAuditStore(t *testing.T, repository AdminResourceRepository) *Store {
	t.Helper()
	store, err := NewRepositoryBackedStoreFromCapabilities(repository, core.DefaultManifests())
	if err != nil {
		t.Fatalf("NewRepositoryBackedStoreFromCapabilities() error = %v", err)
	}
	return store
}
