package datalifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"platform-go/internal/platform/capability"
)

func TestRunnerIsDisabledByDefault(t *testing.T) {
	runner := NewRunner(nil, nil, nil, nil)

	report, err := runner.Run(context.Background(), Options{})

	if !errors.Is(err, ErrDisabled) {
		t.Fatalf("Run() error = %v, want ErrDisabled", err)
	}
	if report.Status != StatusDisabled || report.DatasourceID != DefaultDatasourceID {
		t.Fatalf("Run() report = %+v", report)
	}
}

func TestPolicyFingerprintIsStableAndSensitiveToRetention(t *testing.T) {
	forward := PolicySnapshot{Version: 3, Resources: []ResourcePolicy{
		{Resource: "files", Mode: capability.AdminDeletionTombstone, PolicyVersion: 2, RetentionDays: 30, AutoPurge: true},
		{Resource: "users", Mode: capability.AdminDeletionSoftDelete, PolicyVersion: 4, RetentionDays: 90, AutoPurge: false},
	}}
	reverse := PolicySnapshot{Version: 3, Resources: []ResourcePolicy{
		{Resource: "users", Mode: capability.AdminDeletionSoftDelete, PolicyVersion: 4, RetentionDays: 90, AutoPurge: false},
		{Resource: "files", Mode: capability.AdminDeletionTombstone, PolicyVersion: 2, RetentionDays: 30, AutoPurge: true},
	}}

	forwardFingerprint, err := PolicyFingerprint(forward)
	if err != nil {
		t.Fatalf("PolicyFingerprint() error = %v", err)
	}
	reverseFingerprint, err := PolicyFingerprint(reverse)
	if err != nil {
		t.Fatalf("PolicyFingerprint() error = %v", err)
	}
	if forwardFingerprint != reverseFingerprint {
		t.Fatalf("fingerprints differ: %q != %q", forwardFingerprint, reverseFingerprint)
	}

	reverse.Resources[1].RetentionDays = 29
	changedFingerprint, err := PolicyFingerprint(reverse)
	if err != nil {
		t.Fatalf("PolicyFingerprint(changed) error = %v", err)
	}
	if changedFingerprint == forwardFingerprint {
		t.Fatal("retention change did not change policy fingerprint")
	}

	reverse.Resources[1].RetentionDays = 30
	reverse.Resources[0].Mode = capability.AdminDeletionAppendOnly
	changedFingerprint, err = PolicyFingerprint(reverse)
	if err != nil {
		t.Fatalf("PolicyFingerprint(mode changed) error = %v", err)
	}
	if changedFingerprint == forwardFingerprint {
		t.Fatal("deletion mode change did not change policy fingerprint")
	}
}

func TestPolicyFingerprintRejectsMissingModeAndOversizedRetention(t *testing.T) {
	for _, policy := range []ResourcePolicy{
		{Resource: "files", PolicyVersion: 1, RetentionDays: 30, AutoPurge: true},
		{Resource: "files", Mode: capability.AdminDeletionTombstone, PolicyVersion: 1, RetentionDays: capability.MaximumAdminRetentionDays + 1, AutoPurge: true},
	} {
		if _, err := PolicyFingerprint(PolicySnapshot{Version: 1, Resources: []ResourcePolicy{policy}}); !errors.Is(err, ErrInvalidPolicy) {
			t.Fatalf("PolicyFingerprint(%+v) error = %v, want ErrInvalidPolicy", policy, err)
		}
	}
}

func TestImpactRunUsesPersistentLeaseCheckpointAndSanitizedReport(t *testing.T) {
	now := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)
	clock := &testClock{now: now}
	repository := newTestRepository(true)
	planner := &scriptedPlanner{batches: []Batch{
		{
			DatasourceID: DefaultDatasourceID,
			Candidates: []Candidate{
				{Resource: "files", EligibleAt: "2026-06-01T00:00:00Z", RecordID: "record-secret-1"},
				{Resource: "files", EligibleAt: "2026-06-01T00:00:00Z", RecordID: "object-key-secret"},
			},
			NextCursor: Cursor{DatasourceID: DefaultDatasourceID, Resource: "files", EligibleAt: "2026-06-01T00:00:00Z", RecordID: "record-secret-1"},
		},
		{
			DatasourceID: DefaultDatasourceID,
			Candidates:   []Candidate{{Resource: "files", EligibleAt: "2026-06-02T00:00:00Z", RecordID: "record-secret-2"}},
			NextCursor:   Cursor{DatasourceID: DefaultDatasourceID, Resource: "files", EligibleAt: "2026-06-02T00:00:00Z", RecordID: "record-secret-2"},
			Done:         true,
		},
	}}
	policy := testPolicy(30)
	fingerprint, err := PolicyFingerprint(policy)
	if err != nil {
		t.Fatal(err)
	}
	runner := NewRunner(repository, planner, &recordingApplier{}, clock)

	report, err := runner.Run(context.Background(), Options{
		Enabled: true, Mode: ModeImpact, RunID: "impact-1", OwnerID: "maintenance-1",
		DatasourceID: DefaultDatasourceID, BatchSize: 2, MaxRetries: 1,
		LeaseTTL: time.Minute, HeartbeatInterval: 10 * time.Second,
		Policy: policy, PolicyFingerprint: fingerprint,
	})

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Status != StatusCompleted || report.Mode != ModeImpact || report.DatasourceID != DefaultDatasourceID {
		t.Fatalf("Run() report = %+v", report)
	}
	if report.Counts != (Counts{Eligible: 3, Batches: 2}) {
		t.Fatalf("Run() counts = %+v", report.Counts)
	}
	if report.PolicyFingerprint != fingerprint || !canonicalDigest(report.ImpactReportHash) || !canonicalDigest(report.EvidenceHash) {
		t.Fatalf("Run() report hashes = %+v", report)
	}
	if report.Cursor.DatasourceID != DefaultDatasourceID || !canonicalDigest(report.Cursor.RecordID) {
		t.Fatalf("Run() cursor = %+v", report.Cursor)
	}
	payload, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"record-secret-1", "record-secret-2", "object-key-secret"} {
		if strings.Contains(string(payload), secret) {
			t.Fatalf("report leaked %q: %s", secret, payload)
		}
	}
	checkpoint, found := repository.checkpoints[checkpointKey(Options{
		Mode: ModeImpact, RunID: "impact-1", DatasourceID: DefaultDatasourceID,
		PolicyFingerprint: fingerprint,
	})]
	if !found || !checkpoint.Complete || checkpoint.Cursor.RecordID != "record-secret-2" {
		t.Fatalf("persisted checkpoint = %+v, found = %t", checkpoint, found)
	}
	if repository.impactReports[report.ImpactReportHash].DatasourceID != DefaultDatasourceID {
		t.Fatalf("impact report not persisted: %+v", repository.impactReports)
	}
	if repository.acquireCalls != 1 || repository.releaseCalls != 1 {
		t.Fatalf("lease calls acquire=%d release=%d", repository.acquireCalls, repository.releaseCalls)
	}
}

func TestRunnerRequiresPersistentRepository(t *testing.T) {
	policy := testPolicy(30)
	fingerprint, err := PolicyFingerprint(policy)
	if err != nil {
		t.Fatal(err)
	}
	runner := NewRunner(newTestRepository(false), &scriptedPlanner{}, &recordingApplier{}, &testClock{now: time.Now()})

	_, err = runner.Run(context.Background(), Options{
		Enabled: true, Mode: ModeImpact, RunID: "impact-1", OwnerID: "maintenance-1",
		DatasourceID: DefaultDatasourceID, Policy: policy, PolicyFingerprint: fingerprint,
	})

	if !errors.Is(err, ErrPersistentRepositoryRequired) {
		t.Fatalf("Run() error = %v, want ErrPersistentRepositoryRequired", err)
	}
}

func TestPromoteShorterRetentionRequiresExactImpactAndApproval(t *testing.T) {
	now := time.Date(2026, 7, 14, 11, 0, 0, 0, time.UTC)
	repository := newTestRepository(true)
	current := testPolicy(90)
	proposed := testPolicy(30)
	currentFingerprint, err := PolicyFingerprint(current)
	if err != nil {
		t.Fatal(err)
	}
	proposedFingerprint, err := PolicyFingerprint(proposed)
	if err != nil {
		t.Fatal(err)
	}
	impact := ImpactReport{
		DatasourceID: DefaultDatasourceID, RunID: "impact-shorter-1",
		PolicyFingerprint: proposedFingerprint, Counts: Counts{Eligible: 12, Batches: 2},
		Cursor:       Cursor{DatasourceID: DefaultDatasourceID, Resource: "files", RecordID: stableDigest("cursor", "last")},
		EvidenceHash: stableDigest("evidence", "targets"), GeneratedAt: now,
	}
	impact.ReportHash = impactReportDigest(impact)
	repository.impactReports[impact.ReportHash] = impact
	clock := &testClock{now: now}
	runner := NewRunner(repository, &scriptedPlanner{}, &recordingApplier{}, clock)
	request := PromotionRequest{
		Enabled: true, DatasourceID: DefaultDatasourceID,
		CurrentPolicy: current, ProposedPolicy: proposed, ImpactReportHash: impact.ReportHash,
		ActorID: "security-admin", Reason: "retention-policy-approval",
		ApprovalRef: "CAB-2026-0714", PromotedFingerprint: proposedFingerprint,
	}

	missingActor := request
	missingActor.ActorID = ""
	if _, err := runner.Promote(context.Background(), missingActor); !errors.Is(err, ErrPromotionRequired) {
		t.Fatalf("Promote(missing actor) error = %v", err)
	}
	mismatchedImpact := request
	mismatchedImpact.ImpactReportHash = stableDigest("wrong", "report")
	if _, err := runner.Promote(context.Background(), mismatchedImpact); !errors.Is(err, ErrPromotionRequired) {
		t.Fatalf("Promote(mismatched impact) error = %v", err)
	}

	promotion, err := runner.Promote(context.Background(), request)
	if err != nil {
		t.Fatalf("Promote() error = %v", err)
	}
	if promotion.DatasourceID != DefaultDatasourceID || promotion.CurrentFingerprint != currentFingerprint ||
		promotion.PromotedFingerprint != proposedFingerprint || promotion.ImpactReportHash != impact.ReportHash ||
		promotion.ActorID != request.ActorID || promotion.Reason != request.Reason || promotion.ApprovalRef != request.ApprovalRef {
		t.Fatalf("Promote() = %+v", promotion)
	}
	if stored := repository.promotions[proposedFingerprint]; stored != promotion {
		t.Fatalf("stored promotion = %+v, want %+v", stored, promotion)
	}
	clock.now = now.Add(time.Hour)
	replayed, err := runner.Promote(context.Background(), request)
	if err != nil {
		t.Fatalf("Promote(replay) error = %v", err)
	}
	if replayed != promotion {
		t.Fatalf("Promote(replay) = %+v, want persisted %+v", replayed, promotion)
	}
}

func TestApplyShorterRetentionRequiresPersistedExactPromotion(t *testing.T) {
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	repository := newTestRepository(true)
	current := testPolicy(90)
	proposed := testPolicy(30)
	fingerprint, err := PolicyFingerprint(proposed)
	if err != nil {
		t.Fatal(err)
	}
	approval := PromotionApproval{
		ImpactReportHash: stableDigest("impact", "shorter"), ActorID: "security-admin",
		Reason: "retention-policy-approval", ApprovalRef: "CAB-2026-0714", PromotedFingerprint: fingerprint,
		DryRunID: "dry-run-shorter-1",
	}
	planner := &scriptedPlanner{batches: []Batch{{
		DatasourceID: DefaultDatasourceID,
		Candidates:   []Candidate{{Resource: "files", EligibleAt: "2026-06-01T00:00:00Z", RecordID: "file-1"}},
		NextCursor:   Cursor{DatasourceID: DefaultDatasourceID, Resource: "files", EligibleAt: "2026-06-01T00:00:00Z", RecordID: "file-1"},
		Done:         true,
	}}}
	applier := &recordingApplier{}
	runner := NewRunner(repository, planner, applier, &testClock{now: now})
	options := Options{
		Enabled: true, Mode: ModeApply, RunID: "apply-shorter-1", OwnerID: "maintenance-1",
		Policy: proposed, PolicyFingerprint: fingerprint, Promotion: approval,
	}

	if _, err := runner.Run(context.Background(), options); !errors.Is(err, ErrPromotionRequired) {
		t.Fatalf("Run(without stored promotion) error = %v, want ErrPromotionRequired", err)
	}
	if len(applier.requests) != 0 || repository.acquireCalls != 0 {
		t.Fatalf("unsafe apply started: requests=%d acquire=%d", len(applier.requests), repository.acquireCalls)
	}
	currentFingerprint, err := PolicyFingerprint(current)
	if err != nil {
		t.Fatal(err)
	}
	approval.CurrentFingerprint = currentFingerprint
	options.Promotion = approval
	repository.promotions[fingerprint] = Promotion{
		DatasourceID: DefaultDatasourceID, CurrentFingerprint: currentFingerprint,
		PromotedFingerprint: fingerprint, ImpactReportHash: approval.ImpactReportHash,
		ActorID: approval.ActorID, Reason: approval.Reason, ApprovalRef: approval.ApprovalRef, PromotedAt: now,
	}
	if _, err := runner.Run(context.Background(), options); !errors.Is(err, ErrPromotionRequired) {
		t.Fatalf("Run(without completed dry-run) error = %v, want ErrPromotionRequired", err)
	}
	storeCompletedDryRun(repository, approval.DryRunID, fingerprint, now)

	report, err := runner.Run(context.Background(), options)
	if err != nil {
		t.Fatalf("Run(promoted) error = %v", err)
	}
	if report.Status != StatusCompleted || report.Counts.Applied != 1 || len(applier.requests) != 1 {
		t.Fatalf("Run(promoted) report=%+v requests=%d", report, len(applier.requests))
	}
	if repository.saveCheckpointCalls != 0 {
		t.Fatalf("apply checkpoint was saved outside applier: calls=%d", repository.saveCheckpointCalls)
	}
}

func TestImpactCompletionDoesNotPersistCheckpointWithoutImpactReport(t *testing.T) {
	now := time.Date(2026, 7, 14, 13, 0, 0, 0, time.UTC)
	repository := newTestRepository(true)
	repository.impactSaveErr = errors.New("temporary persistence failure")
	policy := testPolicy(30)
	fingerprint, err := PolicyFingerprint(policy)
	if err != nil {
		t.Fatal(err)
	}
	batch := Batch{
		DatasourceID: DefaultDatasourceID,
		Candidates:   []Candidate{{Resource: "files", EligibleAt: "2026-06-01T00:00:00Z", RecordID: "file-1"}},
		NextCursor:   Cursor{DatasourceID: DefaultDatasourceID, Resource: "files", EligibleAt: "2026-06-01T00:00:00Z", RecordID: "file-1"},
		Done:         true,
	}
	options := Options{
		Enabled: true, Mode: ModeImpact, RunID: "impact-atomic-1", OwnerID: "maintenance-1",
		Policy: policy, PolicyFingerprint: fingerprint,
	}
	runner := NewRunner(repository, &scriptedPlanner{batches: []Batch{batch}}, &recordingApplier{}, &testClock{now: now})

	if _, err := runner.Run(context.Background(), options); !errors.Is(err, ErrRepositoryFailed) {
		t.Fatalf("Run(failed impact save) error = %v, want ErrRepositoryFailed", err)
	}
	if len(repository.checkpoints) != 0 || len(repository.impactReports) != 0 {
		t.Fatalf("partial impact completion persisted: checkpoints=%+v reports=%+v", repository.checkpoints, repository.impactReports)
	}

	repository.impactSaveErr = nil
	runner = NewRunner(repository, &scriptedPlanner{batches: []Batch{batch}}, &recordingApplier{}, &testClock{now: now})
	report, err := runner.Run(context.Background(), options)
	if err != nil || report.Status != StatusCompleted || !canonicalDigest(report.ImpactReportHash) {
		t.Fatalf("Run(retry) report=%+v error=%v", report, err)
	}
}

func TestRunnerRejectsTamperedCheckpoint(t *testing.T) {
	policy := testPolicy(30)
	fingerprint, err := PolicyFingerprint(policy)
	if err != nil {
		t.Fatal(err)
	}
	options := Options{
		Enabled: true, Mode: ModeDryRun, RunID: "dry-run-tampered", OwnerID: "maintenance-1",
		Policy: policy, PolicyFingerprint: fingerprint,
	}
	checkpoint := sealCheckpoint(Checkpoint{
		Key: checkpointKey(options), DatasourceID: DefaultDatasourceID,
		Cursor:       Cursor{DatasourceID: DefaultDatasourceID, Resource: "files", EligibleAt: "2026-06-01T00:00:00Z", RecordID: "file-1"},
		EvidenceHash: stableDigest("evidence", "checkpoint"), Revision: 1,
	})
	checkpoint.Cursor.RecordID = "file-999"
	repository := newTestRepository(true)
	repository.checkpoints[checkpoint.Key] = checkpoint
	runner := NewRunner(repository, &scriptedPlanner{}, &recordingApplier{}, &testClock{now: time.Now()})

	if _, err := runner.Run(context.Background(), options); !errors.Is(err, ErrPolicyDrift) {
		t.Fatalf("Run(tampered checkpoint) error = %v, want ErrPolicyDrift", err)
	}
}

func TestApplyRejectsApplierWithoutCommittedCheckpoint(t *testing.T) {
	policy := testPolicy(30)
	current := testPolicy(90)
	fingerprint, err := PolicyFingerprint(policy)
	if err != nil {
		t.Fatal(err)
	}
	planner := &scriptedPlanner{batches: []Batch{{
		DatasourceID: DefaultDatasourceID,
		Candidates:   []Candidate{{Resource: "files", EligibleAt: "2026-06-01T00:00:00Z", RecordID: "file-1"}},
		NextCursor:   Cursor{DatasourceID: DefaultDatasourceID, Resource: "files", EligibleAt: "2026-06-01T00:00:00Z", RecordID: "file-1"}, Done: true,
	}}}
	applier := batchApplierFunc(func(context.Context, ApplyRequest) (ApplyResult, error) {
		return ApplyResult{Applied: 1}, nil
	})
	repository := newTestRepository(true)
	approval := storeTestPromotion(t, repository, current, policy)
	runner := NewRunner(repository, planner, applier, &testClock{now: time.Now()})

	_, err = runner.Run(context.Background(), Options{
		Enabled: true, Mode: ModeApply, RunID: "apply-uncommitted", OwnerID: "maintenance-1",
		Policy: policy, PolicyFingerprint: fingerprint, Promotion: approval,
	})
	if !errors.Is(err, ErrApplyFailed) {
		t.Fatalf("Run(uncommitted checkpoint) error = %v, want ErrApplyFailed", err)
	}
}

func storeTestPromotion(t *testing.T, repository *testRepository, current, proposed PolicySnapshot) PromotionApproval {
	t.Helper()
	currentFingerprint, err := PolicyFingerprint(current)
	if err != nil {
		t.Fatal(err)
	}
	proposedFingerprint, err := PolicyFingerprint(proposed)
	if err != nil {
		t.Fatal(err)
	}
	approval := PromotionApproval{
		ImpactReportHash: stableDigest("impact", proposedFingerprint), ActorID: "security-admin",
		Reason: "retention-policy-approval", ApprovalRef: "CAB-2026-0714", CurrentFingerprint: currentFingerprint,
		PromotedFingerprint: proposedFingerprint, DryRunID: "dry-run-approved",
	}
	repository.promotions[proposedFingerprint] = Promotion{
		DatasourceID: DefaultDatasourceID, CurrentFingerprint: currentFingerprint,
		PromotedFingerprint: proposedFingerprint, ImpactReportHash: approval.ImpactReportHash,
		ActorID: approval.ActorID, Reason: approval.Reason, ApprovalRef: approval.ApprovalRef, PromotedAt: time.Now().UTC(),
	}
	storeCompletedDryRun(repository, approval.DryRunID, proposedFingerprint, time.Now().UTC())
	return approval
}

func storeCompletedDryRun(repository *testRepository, runID, fingerprint string, now time.Time) {
	key := CheckpointKey{
		DatasourceID: DefaultDatasourceID, RunID: runID, Mode: ModeDryRun,
		PolicyFingerprint: fingerprint,
	}
	repository.checkpoints[key] = sealCheckpoint(Checkpoint{
		Key: key, DatasourceID: DefaultDatasourceID, Cursor: Cursor{DatasourceID: DefaultDatasourceID},
		Counts: Counts{Planned: 1, Batches: 1}, EvidenceHash: stableDigest("dry-run", runID),
		LastBatchID: stableDigest("dry-run-batch", runID), Revision: 1, Complete: true, UpdatedAt: now,
	})
}

func TestApplyRetriesPreserveCheckpointIntegrity(t *testing.T) {
	policy := testPolicy(30)
	current := testPolicy(90)
	fingerprint, err := PolicyFingerprint(policy)
	if err != nil {
		t.Fatal(err)
	}
	planner := &scriptedPlanner{
		failures: []error{errors.New("temporary planner failure")},
		batches: []Batch{{
			DatasourceID: DefaultDatasourceID,
			Candidates:   []Candidate{{Resource: "files", EligibleAt: "2026-06-01T00:00:00Z", RecordID: "file-1"}},
			NextCursor:   Cursor{DatasourceID: DefaultDatasourceID, Resource: "files", EligibleAt: "2026-06-01T00:00:00Z", RecordID: "file-1"},
			Done:         true,
		}},
	}
	applyCalls := 0
	applier := batchApplierFunc(func(_ context.Context, request ApplyRequest) (ApplyResult, error) {
		applyCalls++
		if applyCalls == 1 {
			return ApplyResult{}, errors.New("temporary apply failure")
		}
		return ApplyResult{Applied: 1, Checkpoint: request.Checkpoint}, nil
	})
	repository := newTestRepository(true)
	approval := storeTestPromotion(t, repository, current, policy)
	runner := NewRunner(repository, planner, applier, &testClock{now: time.Now().UTC()})

	report, err := runner.Run(context.Background(), Options{
		Enabled: true, Mode: ModeApply, RunID: "apply-retry", OwnerID: "maintenance-1",
		Policy: policy, PolicyFingerprint: fingerprint, Promotion: approval, MaxRetries: 1,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Status != StatusCompleted || report.Counts != (Counts{Applied: 1, Batches: 1, Retries: 2}) {
		t.Fatalf("Run() report = %+v", report)
	}
	if applyCalls != 2 {
		t.Fatalf("ApplyAndCheckpoint() calls = %d, want 2", applyCalls)
	}
}

func testPolicy(days int) PolicySnapshot {
	return PolicySnapshot{Version: 1, Resources: []ResourcePolicy{{
		Resource: "files", Mode: capability.AdminDeletionTombstone, PolicyVersion: 1, RetentionDays: days, AutoPurge: true,
	}}}
}

type testClock struct {
	now time.Time
}

func (c *testClock) Now() time.Time { return c.now }

type scriptedPlanner struct {
	batches  []Batch
	calls    []PlanRequest
	failures []error
}

func (p *scriptedPlanner) Plan(_ context.Context, request PlanRequest) (Batch, error) {
	p.calls = append(p.calls, request)
	if len(p.failures) > 0 {
		err := p.failures[0]
		p.failures = p.failures[1:]
		return Batch{}, err
	}
	if len(p.batches) == 0 {
		return Batch{DatasourceID: request.DatasourceID, Done: true}, nil
	}
	batch := p.batches[0]
	p.batches = p.batches[1:]
	return batch, nil
}

type recordingApplier struct {
	requests []ApplyRequest
}

func (a *recordingApplier) ApplyAndCheckpoint(_ context.Context, request ApplyRequest) (ApplyResult, error) {
	a.requests = append(a.requests, request)
	return ApplyResult{Applied: len(request.Batch.Candidates), Checkpoint: request.Checkpoint}, nil
}

type batchApplierFunc func(context.Context, ApplyRequest) (ApplyResult, error)

func (fn batchApplierFunc) ApplyAndCheckpoint(ctx context.Context, request ApplyRequest) (ApplyResult, error) {
	return fn(ctx, request)
}

type testRepository struct {
	persistent          bool
	lease               Lease
	checkpoints         map[CheckpointKey]Checkpoint
	impactReports       map[string]ImpactReport
	promotions          map[string]Promotion
	acquireCalls        int
	releaseCalls        int
	saveCheckpointCalls int
	saveImpactCalls     int
	saveCheckpointErr   error
	impactSaveErr       error
}

func newTestRepository(persistent bool) *testRepository {
	return &testRepository{
		persistent: persistent, checkpoints: map[CheckpointKey]Checkpoint{},
		impactReports: map[string]ImpactReport{}, promotions: map[string]Promotion{},
	}
}

func (r *testRepository) Persistent() bool { return r.persistent }

func (r *testRepository) AcquireLease(_ context.Context, request LeaseRequest) (Lease, error) {
	r.acquireCalls++
	r.lease = Lease{
		DatasourceID: request.DatasourceID, Key: request.Key, OwnerID: request.OwnerID,
		Token: "lease-token", PolicyFingerprint: request.PolicyFingerprint,
		AcquiredAt: request.Now, HeartbeatAt: request.Now, ExpiresAt: request.Now.Add(request.TTL),
	}
	return r.lease, nil
}

func (r *testRepository) HeartbeatLease(_ context.Context, lease Lease, now time.Time, ttl time.Duration) (Lease, error) {
	lease.HeartbeatAt = now
	lease.ExpiresAt = now.Add(ttl)
	r.lease = lease
	return lease, nil
}

func (r *testRepository) ReleaseLease(_ context.Context, _ Lease) error {
	r.releaseCalls++
	return nil
}

func (r *testRepository) LoadCheckpoint(_ context.Context, key CheckpointKey) (Checkpoint, bool, error) {
	checkpoint, found := r.checkpoints[key]
	return checkpoint, found, nil
}

func (r *testRepository) SaveCheckpoint(_ context.Context, _ Lease, checkpoint Checkpoint) error {
	r.saveCheckpointCalls++
	if r.saveCheckpointErr != nil {
		return r.saveCheckpointErr
	}
	r.checkpoints[checkpoint.Key] = checkpoint
	return nil
}

func (r *testRepository) SaveImpactReportAndCheckpoint(_ context.Context, _ Lease, report ImpactReport, checkpoint Checkpoint) error {
	r.saveImpactCalls++
	if r.impactSaveErr != nil {
		return r.impactSaveErr
	}
	r.impactReports[report.ReportHash] = report
	r.checkpoints[checkpoint.Key] = checkpoint
	return nil
}

func (r *testRepository) LoadImpactReport(_ context.Context, datasourceID, reportHash string) (ImpactReport, bool, error) {
	report, found := r.impactReports[reportHash]
	return report, found && report.DatasourceID == datasourceID, nil
}

func (r *testRepository) SavePromotion(_ context.Context, promotion Promotion) error {
	if existing, found := r.promotions[promotion.PromotedFingerprint]; found {
		if !samePromotion(existing, promotion) {
			return ErrRepositoryFailed
		}
		return nil
	}
	r.promotions[promotion.PromotedFingerprint] = promotion
	return nil
}

func (r *testRepository) LoadPromotion(_ context.Context, datasourceID, fingerprint string) (Promotion, bool, error) {
	promotion, found := r.promotions[fingerprint]
	return promotion, found && promotion.DatasourceID == datasourceID, nil
}
