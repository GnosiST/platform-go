package datalifecycle

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/GnosiST/platform-go/internal/platform/adminresource"
	"github.com/GnosiST/platform-go/internal/platform/capability"
	"github.com/GnosiST/platform-go/internal/platform/core"
	"github.com/GnosiST/platform-go/internal/platform/storage"
)

func TestPolicySnapshotFromDefaultManifestsUsesDeclaredRetention(t *testing.T) {
	manifests := core.DefaultManifests()
	policy, err := PolicySnapshotFromManifests(manifests)
	if err != nil {
		t.Fatalf("PolicySnapshotFromManifests(DefaultManifests) error = %v", err)
	}
	assertPolicy := func(resource string, mode string, retentionDays int, autoPurge bool) {
		t.Helper()
		for _, declared := range policy.Resources {
			if declared.Resource == resource {
				if declared.Mode != mode || declared.PolicyVersion != 1 || declared.RetentionDays != retentionDays || declared.AutoPurge != autoPurge {
					t.Fatalf("policy %q = %+v", resource, declared)
				}
				return
			}
		}
		t.Fatalf("policy %q is missing", resource)
	}
	assertPolicy("api-tokens", capability.AdminDeletionRevoke, 90, true)
	assertPolicy("files", capability.AdminDeletionTombstone, 30, true)
	assertPolicy("audit-logs", capability.AdminDeletionAppendOnly, 0, false)

	fingerprint, err := PolicyFingerprint(policy)
	if err != nil {
		t.Fatalf("PolicyFingerprint(default policy) error = %v", err)
	}
	for left, right := 0, len(manifests)-1; left < right; left, right = left+1, right-1 {
		manifests[left], manifests[right] = manifests[right], manifests[left]
	}
	reordered, err := PolicySnapshotFromManifests(manifests)
	if err != nil {
		t.Fatalf("PolicySnapshotFromManifests(reordered defaults) error = %v", err)
	}
	reorderedFingerprint, err := PolicyFingerprint(reordered)
	if err != nil || reorderedFingerprint != fingerprint {
		t.Fatalf("reordered fingerprint = %q, %v; want %q", reorderedFingerprint, err, fingerprint)
	}
}

func TestAdminResourcePlannerUsesProposedRetentionAndBoundsFileBatches(t *testing.T) {
	store := &fakeLifecycleStore{records: map[string][]adminresource.Record{
		"files": {
			{ID: "file-1", DeletedAt: "2026-06-01T00:00:00Z"},
			{ID: "file-2", DeletedAt: "2026-06-02T00:00:00Z"},
		},
	}}
	planner := NewAdminResourcePlanner(store, &testClock{now: time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC)})
	policy := PolicySnapshot{Version: 1, Resources: []ResourcePolicy{{
		Resource: "files", Mode: capability.AdminDeletionTombstone, PolicyVersion: 2, RetentionDays: 30, AutoPurge: true,
	}}}

	first, err := planner.Plan(context.Background(), PlanRequest{
		DatasourceID: DefaultDatasourceID, Mode: ModeImpact, RunID: "impact-files",
		Policy: policy, Cursor: Cursor{DatasourceID: DefaultDatasourceID}, Limit: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Candidates) != 1 || first.Candidates[0].RecordID != "file-1" || first.Done {
		t.Fatalf("first batch = %+v", first)
	}
	if first.Candidates[0].EligibleAt != "2026-07-01T00:00:00Z" {
		t.Fatalf("eligibleAt = %q, want proposed retention boundary", first.Candidates[0].EligibleAt)
	}
	second, err := planner.Plan(context.Background(), PlanRequest{
		DatasourceID: DefaultDatasourceID, Mode: ModeImpact, RunID: "impact-files",
		Policy: policy, Cursor: first.NextCursor, Limit: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Candidates) != 1 || second.Candidates[0].RecordID != "file-2" || !second.Done {
		t.Fatalf("second batch = %+v", second)
	}
}

func TestAdminResourcePlannerOnlySelectsRevokedAPITokenMetadata(t *testing.T) {
	store := &fakeLifecycleStore{records: map[string][]adminresource.Record{
		"api-tokens": {
			{ID: "token-active", Status: "active", Values: map[string]string{"revokedAt": "2026-06-01T00:00:00Z"}},
			{ID: "token-revoked", Status: "revoked", Values: map[string]string{"revokedAt": "2026-06-01T00:00:00Z"}},
			{ID: "token-invalid", Status: "revoked"},
		},
	}}
	planner := NewAdminResourcePlanner(store, &testClock{now: time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC)})

	batch, err := planner.Plan(context.Background(), PlanRequest{
		DatasourceID: DefaultDatasourceID, Mode: ModeDryRun, RunID: "dry-run-tokens", Limit: 100,
		Policy: PolicySnapshot{Version: 1, Resources: []ResourcePolicy{{
			Resource: "api-tokens", Mode: capability.AdminDeletionRevoke, PolicyVersion: 1, RetentionDays: 30, AutoPurge: true,
		}}},
		Cursor: Cursor{DatasourceID: DefaultDatasourceID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Candidates) != 1 || batch.Candidates[0].RecordID != "token-revoked" || !batch.Done {
		t.Fatalf("token retention batch = %+v", batch)
	}
}

func TestAdminResourcePlannerRejectsUnsupportedAutomaticPurgeRoute(t *testing.T) {
	planner := NewAdminResourcePlanner(&fakeLifecycleStore{}, &testClock{now: time.Now().UTC()})
	_, err := planner.Plan(context.Background(), PlanRequest{
		DatasourceID: DefaultDatasourceID, Mode: ModeImpact, RunID: "impact-files", Limit: 100,
		Policy: PolicySnapshot{Version: 1, Resources: []ResourcePolicy{{
			Resource: "files", Mode: capability.AdminDeletionSoftDelete, PolicyVersion: 1, RetentionDays: 30, AutoPurge: true,
		}}},
		Cursor: Cursor{DatasourceID: DefaultDatasourceID},
	})
	if !errors.Is(err, ErrPlanningFailed) {
		t.Fatalf("Plan(unsupported route) error = %v, want ErrPlanningFailed", err)
	}
}

func TestFileCleanupApplierTreatsObjectNotFoundAsSuccess(t *testing.T) {
	request := fileApplyRequest(t)
	store := newFakeFileCleanupStore(request.Batch.Candidates[0].RecordID)
	objects := &fakeObjectStore{deleteErr: storage.ErrObjectNotFound}
	repository := newTestRepository(true)
	applier := NewFileCleanupApplier(store, repository, map[string]storage.ObjectStore{"local": objects})

	result, err := applier.ApplyAndCheckpoint(context.Background(), request)
	if err != nil {
		t.Fatalf("ApplyAndCheckpoint() error = %v", err)
	}
	if result.Applied != 1 || result.Checkpoint != request.Checkpoint {
		t.Fatalf("ApplyAndCheckpoint() result = %+v", result)
	}
	if strings.Join(store.calls, ",") != "claim,complete,purge" || objects.deleteCalls != 1 {
		t.Fatalf("cleanup calls=%v object deletes=%d", store.calls, objects.deleteCalls)
	}
	if persisted := repository.checkpoints[request.Checkpoint.Key]; persisted != request.Checkpoint {
		t.Fatalf("persisted checkpoint = %+v", persisted)
	}
}

func TestFileCleanupApplierRetriesFromDurableStates(t *testing.T) {
	t.Run("delete failure keeps cleanup started", func(t *testing.T) {
		request := fileApplyRequest(t)
		store := newFakeFileCleanupStore(request.Batch.Candidates[0].RecordID)
		objects := &fakeObjectStore{deleteErr: errors.New("private object dependency detail")}
		repository := newTestRepository(true)
		applier := NewFileCleanupApplier(store, repository, map[string]storage.ObjectStore{"local": objects})

		if _, err := applier.ApplyAndCheckpoint(context.Background(), request); err == nil {
			t.Fatal("ApplyAndCheckpoint() error = nil, want delete failure")
		}
		if strings.Join(store.calls, ",") != "claim" || store.state != adminresource.FileDeletionCleanupStarted || len(repository.checkpoints) != 0 {
			t.Fatalf("failed cleanup state=%q calls=%v checkpoints=%+v", store.state, store.calls, repository.checkpoints)
		}
	})

	t.Run("purged metadata repairs missing checkpoint", func(t *testing.T) {
		request := fileApplyRequest(t)
		store := newFakeFileCleanupStore(request.Batch.Candidates[0].RecordID)
		repository := newTestRepository(true)
		repository.saveCheckpointErr = errors.New("temporary checkpoint failure")
		applier := NewFileCleanupApplier(store, repository, map[string]storage.ObjectStore{"local": &fakeObjectStore{}})

		if _, err := applier.ApplyAndCheckpoint(context.Background(), request); err == nil {
			t.Fatal("ApplyAndCheckpoint() error = nil, want checkpoint failure")
		}
		if !store.purged {
			t.Fatal("metadata purge did not complete before injected checkpoint failure")
		}
		repository.saveCheckpointErr = nil
		result, err := applier.ApplyAndCheckpoint(context.Background(), request)
		if err != nil || result.Checkpoint != request.Checkpoint {
			t.Fatalf("ApplyAndCheckpoint(replay) result=%+v error=%v", result, err)
		}
		if repository.checkpoints[request.Checkpoint.Key] != request.Checkpoint {
			t.Fatalf("checkpoint was not repaired: %+v", repository.checkpoints)
		}
	})

	t.Run("lost lease after object delete resumes from cleanup started", func(t *testing.T) {
		request := fileApplyRequest(t)
		store := newFakeFileCleanupStore(request.Batch.Candidates[0].RecordID)
		objects := &fakeObjectStore{}
		lostRepository := &leaseFenceTestRepository{testRepository: newTestRepository(true), failHeartbeat: 3}
		applier := NewFileCleanupApplier(store, lostRepository, map[string]storage.ObjectStore{"local": objects})

		if _, err := applier.ApplyAndCheckpoint(context.Background(), request); !errors.Is(err, ErrLeaseLost) {
			t.Fatalf("ApplyAndCheckpoint(first) error = %v, want ErrLeaseLost", err)
		}
		if store.state != adminresource.FileDeletionCleanupStarted || objects.deleteCalls != 1 {
			t.Fatalf("first attempt state=%q deletes=%d", store.state, objects.deleteCalls)
		}

		objects.deleteErr = storage.ErrObjectNotFound
		repository := newTestRepository(true)
		applier = NewFileCleanupApplier(store, repository, map[string]storage.ObjectStore{"local": objects})
		result, err := applier.ApplyAndCheckpoint(context.Background(), request)
		if err != nil || result.Checkpoint != request.Checkpoint {
			t.Fatalf("ApplyAndCheckpoint(retry) result=%+v error=%v", result, err)
		}
		if got := strings.Join(store.calls, ","); got != "claim,complete,purge" || objects.deleteCalls != 2 || !store.purged {
			t.Fatalf("retry calls=%q deletes=%d purged=%t", got, objects.deleteCalls, store.purged)
		}
	})
}

func TestFileCleanupApplierFailsClosedWhenLeaseIsLost(t *testing.T) {
	tests := []struct {
		name           string
		failHeartbeat  int
		wantCalls      string
		wantState      string
		wantDeletes    int
		wantHeartbeats int
	}{
		{
			name: "before claim", failHeartbeat: 1,
			wantState: adminresource.FileDeletionPending, wantHeartbeats: 1,
		},
		{
			name: "before object delete", failHeartbeat: 2,
			wantCalls: "claim", wantState: adminresource.FileDeletionCleanupStarted, wantHeartbeats: 2,
		},
		{
			name: "after object delete", failHeartbeat: 3,
			wantCalls: "claim", wantState: adminresource.FileDeletionCleanupStarted, wantDeletes: 1, wantHeartbeats: 3,
		},
		{
			name: "before metadata purge", failHeartbeat: 4,
			wantCalls: "claim,complete", wantState: adminresource.FileDeletionObjectDeleted, wantDeletes: 1, wantHeartbeats: 4,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := fileApplyRequest(t)
			store := newFakeFileCleanupStore(request.Batch.Candidates[0].RecordID)
			objects := &fakeObjectStore{}
			repository := &leaseFenceTestRepository{testRepository: newTestRepository(true), failHeartbeat: test.failHeartbeat}
			applier := NewFileCleanupApplier(store, repository, map[string]storage.ObjectStore{"local": objects})

			if _, err := applier.ApplyAndCheckpoint(context.Background(), request); !errors.Is(err, ErrLeaseLost) {
				t.Fatalf("ApplyAndCheckpoint() error = %v, want ErrLeaseLost", err)
			}
			if got := strings.Join(store.calls, ","); got != test.wantCalls {
				t.Fatalf("store calls = %q, want %q", got, test.wantCalls)
			}
			if store.state != test.wantState || objects.deleteCalls != test.wantDeletes || repository.heartbeatCalls != test.wantHeartbeats {
				t.Fatalf("state=%q deletes=%d heartbeats=%d", store.state, objects.deleteCalls, repository.heartbeatCalls)
			}
			if store.purged || len(repository.checkpoints) != 0 {
				t.Fatalf("lost lease committed cleanup: purged=%t checkpoints=%+v", store.purged, repository.checkpoints)
			}
		})
	}
}

func TestFileCleanupApplierFailsClosedBeforeClaimForMissingStorageBinding(t *testing.T) {
	request := fileApplyRequest(t)
	store := newFakeFileCleanupStore(request.Batch.Candidates[0].RecordID)
	store.record.Values["storageKey"] = ""
	store.record.Code = ""
	applier := NewFileCleanupApplier(store, newTestRepository(true), map[string]storage.ObjectStore{"local": &fakeObjectStore{}})

	if _, err := applier.ApplyAndCheckpoint(context.Background(), request); !errors.Is(err, ErrApplyFailed) {
		t.Fatalf("ApplyAndCheckpoint(empty key) error = %v, want ErrApplyFailed", err)
	}
	if len(store.calls) != 0 {
		t.Fatalf("empty key cleanup changed state: calls=%v", store.calls)
	}
}

func TestFileCleanupApplierRejectsDeletionModeDriftBeforeClaim(t *testing.T) {
	request := fileApplyRequest(t)
	request.Policy.Resources[0].Mode = capability.AdminDeletionSoftDelete
	store := newFakeFileCleanupStore(request.Batch.Candidates[0].RecordID)
	applier := NewFileCleanupApplier(store, newTestRepository(true), map[string]storage.ObjectStore{"local": &fakeObjectStore{}})

	if _, err := applier.ApplyAndCheckpoint(context.Background(), request); !errors.Is(err, ErrPolicyDrift) {
		t.Fatalf("ApplyAndCheckpoint(mode drift) error = %v, want ErrPolicyDrift", err)
	}
	if len(store.calls) != 0 {
		t.Fatalf("mode drift changed state: calls=%v", store.calls)
	}
}

func fileApplyRequest(t *testing.T) ApplyRequest {
	t.Helper()
	now := time.Now().UTC()
	policy := testPolicy(30)
	fingerprint, err := PolicyFingerprint(policy)
	if err != nil {
		t.Fatal(err)
	}
	key := CheckpointKey{DatasourceID: DefaultDatasourceID, RunID: "apply-files", Mode: ModeApply, PolicyFingerprint: fingerprint}
	previous := sealCheckpoint(Checkpoint{
		Key: key, DatasourceID: DefaultDatasourceID, Cursor: Cursor{DatasourceID: DefaultDatasourceID},
		EvidenceHash: stableDigest("evidence-empty", DefaultDatasourceID, fingerprint),
	})
	checkpoint := sealCheckpoint(Checkpoint{
		Key: key, DatasourceID: DefaultDatasourceID,
		Cursor: Cursor{DatasourceID: DefaultDatasourceID, Resource: "files", EligibleAt: "2026-07-01T00:00:00Z", RecordID: "file-1"},
		Counts: Counts{Applied: 1, Batches: 1}, EvidenceHash: stableDigest("evidence", "file-1"),
		LastBatchID: stableDigest("batch", "file-1"), Revision: 1, Complete: true, UpdatedAt: time.Now().UTC(),
	})
	return ApplyRequest{
		DatasourceID: DefaultDatasourceID, RunID: "apply-files", BatchID: checkpoint.LastBatchID,
		Policy: policy, PolicyFingerprint: fingerprint,
		Lease: Lease{
			DatasourceID: DefaultDatasourceID, Key: "data-lifecycle:default", OwnerID: "maintenance-1",
			Token: "lease-token", PolicyFingerprint: fingerprint,
			AcquiredAt: now, HeartbeatAt: now, ExpiresAt: now.Add(time.Minute),
		},
		Batch: Batch{
			DatasourceID: DefaultDatasourceID,
			Candidates:   []Candidate{{Resource: "files", EligibleAt: "2026-07-01T00:00:00Z", RecordID: "file-1"}},
			NextCursor:   checkpoint.Cursor, Done: true,
		},
		PreviousCheckpoint: previous,
		Checkpoint:         checkpoint,
	}
}

type leaseFenceTestRepository struct {
	*testRepository
	heartbeatCalls int
	failHeartbeat  int
}

func (r *leaseFenceTestRepository) HeartbeatLease(ctx context.Context, lease Lease, now time.Time, ttl time.Duration) (Lease, error) {
	r.heartbeatCalls++
	if r.heartbeatCalls == r.failHeartbeat {
		return Lease{}, ErrLeaseLost
	}
	return r.testRepository.HeartbeatLease(ctx, lease, now, ttl)
}

type fakeLifecycleStore struct {
	records map[string][]adminresource.Record
}

func (s *fakeLifecycleStore) InternalRecordsContext(_ context.Context, resource string) ([]adminresource.Record, error) {
	return append([]adminresource.Record(nil), s.records[resource]...), nil
}

func (s *fakeLifecycleStore) InternalRecord(string, string) (adminresource.Record, error) {
	return adminresource.Record{}, adminresource.ErrRecordNotFound
}

func (s *fakeLifecycleStore) ClaimTombstonedFileCleanupWithPolicyAndAudit(string, adminresource.MaintenanceRetentionPolicy, adminresource.AuditEvent) (adminresource.MutationResult, error) {
	return adminresource.MutationResult{}, errors.New("unexpected claim")
}

func (s *fakeLifecycleStore) CompleteTombstonedFileCleanupWithAudit(string, adminresource.AuditEvent) (adminresource.MutationResult, error) {
	return adminresource.MutationResult{}, errors.New("unexpected complete")
}

func (s *fakeLifecycleStore) PurgeTombstonedFileWithPolicyAndAudit(string, adminresource.MaintenanceRetentionPolicy, adminresource.AuditEvent) (adminresource.MutationResult, error) {
	return adminresource.MutationResult{}, errors.New("unexpected purge")
}

type fakeFileCleanupStore struct {
	record adminresource.Record
	state  string
	calls  []string
	purged bool
}

func newFakeFileCleanupStore(recordID string) *fakeFileCleanupStore {
	state := adminresource.FileDeletionPending
	return &fakeFileCleanupStore{
		state: state,
		record: adminresource.Record{
			ID: recordID, Code: "legacy-file-key", DeletedAt: "2026-06-01T00:00:00Z",
			Values: map[string]string{"storageKey": "objects/file-1", "storageDriver": "local", "deletionState": state},
		},
	}
}

func (s *fakeFileCleanupStore) InternalRecordsContext(context.Context, string) ([]adminresource.Record, error) {
	if s.purged {
		return nil, nil
	}
	return []adminresource.Record{s.record}, nil
}

func (s *fakeFileCleanupStore) InternalRecord(_, id string) (adminresource.Record, error) {
	if s.purged || id != s.record.ID {
		return adminresource.Record{}, adminresource.ErrRecordNotFound
	}
	return s.record, nil
}

func (s *fakeFileCleanupStore) ClaimTombstonedFileCleanupWithPolicyAndAudit(_ string, _ adminresource.MaintenanceRetentionPolicy, _ adminresource.AuditEvent) (adminresource.MutationResult, error) {
	s.calls = append(s.calls, "claim")
	s.state = adminresource.FileDeletionCleanupStarted
	s.record.Values["deletionState"] = s.state
	return adminresource.MutationResult{Record: s.record}, nil
}

func (s *fakeFileCleanupStore) CompleteTombstonedFileCleanupWithAudit(_ string, _ adminresource.AuditEvent) (adminresource.MutationResult, error) {
	s.calls = append(s.calls, "complete")
	s.state = adminresource.FileDeletionObjectDeleted
	s.record.Values["deletionState"] = s.state
	return adminresource.MutationResult{Record: s.record}, nil
}

func (s *fakeFileCleanupStore) PurgeTombstonedFileWithPolicyAndAudit(_ string, _ adminresource.MaintenanceRetentionPolicy, _ adminresource.AuditEvent) (adminresource.MutationResult, error) {
	s.calls = append(s.calls, "purge")
	s.purged = true
	return adminresource.MutationResult{Record: s.record}, nil
}

type fakeObjectStore struct {
	deleteCalls int
	deleteErr   error
}

func (s *fakeObjectStore) Save(context.Context, storage.ObjectSaveInput) (storage.ObjectMetadata, error) {
	return storage.ObjectMetadata{}, errors.New("unexpected save")
}

func (s *fakeObjectStore) Open(context.Context, string) (io.ReadCloser, error) {
	return nil, errors.New("unexpected open")
}

func (s *fakeObjectStore) Delete(context.Context, string) error {
	s.deleteCalls++
	return s.deleteErr
}
