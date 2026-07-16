package datalifecycle

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/GnosiST/platform-go/internal/platform/storage"

	"gorm.io/gorm"
)

func TestOpenGORMRepositoryDoesNotCreateSchema(t *testing.T) {
	db := openLifecycleTestDB(t)

	if _, err := OpenGORMRepository(context.Background(), db); !errors.Is(err, ErrRepositoryFailed) {
		t.Fatalf("OpenGORMRepository() error = %v, want ErrRepositoryFailed", err)
	}
	for _, table := range lifecycleRepositoryTableNames() {
		if db.Migrator().HasTable(table) {
			t.Fatalf("OpenGORMRepository() created %q", table)
		}
	}
}

func TestPrepareGORMRepositoryCreatesDedicatedSchemaAndOpenReusesIt(t *testing.T) {
	db := openLifecycleTestDB(t)
	repository, err := PrepareGORMRepository(context.Background(), db)
	if err != nil {
		t.Fatalf("PrepareGORMRepository() error = %v", err)
	}
	if !repository.Persistent() {
		t.Fatal("Persistent() = false")
	}
	for _, table := range lifecycleRepositoryTableNames() {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("PrepareGORMRepository() did not create %q", table)
		}
	}
	if db.Migrator().HasTable("platform_admin_resource_records") {
		t.Fatal("PrepareGORMRepository() migrated ordinary Admin resource tables")
	}
	if _, err := OpenGORMRepository(context.Background(), db); err != nil {
		t.Fatalf("OpenGORMRepository(prepared) error = %v", err)
	}
}

func TestGORMRepositoryLeaseUsesExpiryAndFencingToken(t *testing.T) {
	repository := prepareLifecycleTestRepository(t)
	now := time.Now().UTC().Truncate(time.Millisecond)
	request := LeaseRequest{
		DatasourceID: DefaultDatasourceID, Key: "data-lifecycle:default", OwnerID: "worker-1",
		PolicyFingerprint: testLifecycleDigest("policy-1"), Now: now, TTL: time.Minute,
	}
	unsupported := request
	unsupported.DatasourceID = "secondary"
	if _, err := repository.AcquireLease(context.Background(), unsupported); !errors.Is(err, ErrRepositoryFailed) {
		t.Fatalf("AcquireLease(non-default datasource) error = %v, want ErrRepositoryFailed", err)
	}
	first, err := repository.AcquireLease(context.Background(), request)
	if err != nil {
		t.Fatalf("AcquireLease(first) error = %v", err)
	}
	if first.Token == "" || !first.ExpiresAt.Equal(now.Add(time.Minute)) {
		t.Fatalf("AcquireLease(first) = %+v", first)
	}
	if _, err := repository.AcquireLease(context.Background(), request); !errors.Is(err, ErrLeaseHeld) {
		t.Fatalf("AcquireLease(active) error = %v, want ErrLeaseHeld", err)
	}
	if _, err := repository.HeartbeatLease(context.Background(), first, first.ExpiresAt, time.Minute); !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("HeartbeatLease(expired) error = %v, want ErrLeaseLost", err)
	}
	if _, err := repository.HeartbeatLease(context.Background(), first, first.HeartbeatAt.Add(-time.Second), time.Minute); !errors.Is(err, ErrRepositoryFailed) {
		t.Fatalf("HeartbeatLease(backward time) error = %v, want ErrRepositoryFailed", err)
	}

	request.OwnerID = "worker-2"
	request.Now = first.ExpiresAt
	second, err := repository.AcquireLease(context.Background(), request)
	if err != nil {
		t.Fatalf("AcquireLease(expired) error = %v", err)
	}
	if second.Token == first.Token || second.OwnerID != "worker-2" {
		t.Fatalf("replacement lease = %+v, first = %+v", second, first)
	}
	if err := repository.ReleaseLease(context.Background(), first); !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("ReleaseLease(stale) error = %v, want ErrLeaseLost", err)
	}
	checkpoint := lifecycleTestCheckpoint("dry-run-fenced", ModeDryRun, 1, "record-1", false)
	if err := repository.SaveCheckpoint(context.Background(), first, checkpoint); !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("SaveCheckpoint(stale lease) error = %v, want ErrLeaseLost", err)
	}
	if _, err := repository.HeartbeatLease(context.Background(), second, second.HeartbeatAt.Add(time.Second), time.Minute); err != nil {
		t.Fatalf("HeartbeatLease(current) error = %v", err)
	}
}

func TestGORMRepositoryCheckpointUsesIntegrityRevisionCASAndRestart(t *testing.T) {
	db := openLifecycleTestDB(t)
	repository, err := PrepareGORMRepository(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	lease := acquireLifecycleTestLease(t, repository)
	checkpoint := lifecycleTestCheckpoint("dry-run-1", ModeDryRun, 1, "record-1", false)

	if err := repository.SaveCheckpoint(context.Background(), lease, checkpoint); err != nil {
		t.Fatalf("SaveCheckpoint(first) error = %v", err)
	}
	loaded, found, err := repository.LoadCheckpoint(context.Background(), checkpoint.Key)
	if err != nil || !found || loaded != checkpoint {
		t.Fatalf("LoadCheckpoint() = %+v, %t, %v", loaded, found, err)
	}

	tampered := checkpoint
	tampered.Cursor.RecordID = "record-tampered"
	if err := repository.SaveCheckpoint(context.Background(), lease, tampered); !errors.Is(err, ErrRepositoryFailed) {
		t.Fatalf("SaveCheckpoint(tampered) error = %v, want ErrRepositoryFailed", err)
	}
	conflict := lifecycleTestCheckpoint("dry-run-1", ModeDryRun, 1, "record-2", false)
	if err := repository.SaveCheckpoint(context.Background(), lease, conflict); !errors.Is(err, ErrRepositoryFailed) {
		t.Fatalf("SaveCheckpoint(stale revision) error = %v, want ErrRepositoryFailed", err)
	}

	next := lifecycleTestCheckpoint("dry-run-1", ModeDryRun, 2, "record-2", true)
	next.Counts = Counts{Planned: 2, Batches: 2}
	next.LastBatchID = testLifecycleDigest("batch-2")
	next = sealCheckpoint(next)
	if err := repository.SaveCheckpoint(context.Background(), lease, next); err != nil {
		t.Fatalf("SaveCheckpoint(next) error = %v", err)
	}
	if err := repository.SaveCheckpoint(context.Background(), lease, next); err != nil {
		t.Fatalf("SaveCheckpoint(idempotent) error = %v", err)
	}

	reopened, err := OpenGORMRepository(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	restarted, found, err := reopened.LoadCheckpoint(context.Background(), next.Key)
	if err != nil || !found || restarted != next {
		t.Fatalf("restarted checkpoint = %+v, %t, %v", restarted, found, err)
	}
}

func TestGORMRepositorySavesImpactReportAndCheckpointAtomically(t *testing.T) {
	db := openLifecycleTestDB(t)
	repository, err := PrepareGORMRepository(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	lease := acquireLifecycleTestLease(t, repository)
	checkpoint := lifecycleTestCheckpoint("impact-1", ModeImpact, 1, "record-1", true)
	checkpoint.Counts = Counts{Eligible: 3, Batches: 1}
	checkpoint = sealCheckpoint(checkpoint)
	impact := lifecycleTestImpact(checkpoint)

	invalid := impact
	invalid.ReportHash = testLifecycleDigest("wrong-report")
	if err := repository.SaveImpactReportAndCheckpoint(context.Background(), lease, invalid, checkpoint); !errors.Is(err, ErrRepositoryFailed) {
		t.Fatalf("SaveImpactReportAndCheckpoint(invalid) error = %v, want ErrRepositoryFailed", err)
	}
	if _, found, err := repository.LoadCheckpoint(context.Background(), checkpoint.Key); err != nil || found {
		t.Fatalf("checkpoint after failed atomic save found=%t error=%v", found, err)
	}
	if _, found, err := repository.LoadImpactReport(context.Background(), DefaultDatasourceID, invalid.ReportHash); err != nil || found {
		t.Fatalf("impact after failed atomic save found=%t error=%v", found, err)
	}
	if err := db.Exec(`CREATE TRIGGER fail_lifecycle_impact_insert BEFORE INSERT ON platform_data_lifecycle_impact_reports BEGIN SELECT RAISE(FAIL, 'forced impact failure'); END`).Error; err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}
	if err := repository.SaveImpactReportAndCheckpoint(context.Background(), lease, impact, checkpoint); !errors.Is(err, ErrRepositoryFailed) {
		t.Fatalf("SaveImpactReportAndCheckpoint(storage failure) error = %v, want ErrRepositoryFailed", err)
	}
	if _, found, err := repository.LoadCheckpoint(context.Background(), checkpoint.Key); err != nil || found {
		t.Fatalf("checkpoint after transaction rollback found=%t error=%v", found, err)
	}
	if err := db.Exec(`DROP TRIGGER fail_lifecycle_impact_insert`).Error; err != nil {
		t.Fatalf("drop failure trigger: %v", err)
	}

	if err := repository.SaveImpactReportAndCheckpoint(context.Background(), lease, impact, checkpoint); err != nil {
		t.Fatalf("SaveImpactReportAndCheckpoint() error = %v", err)
	}
	replayedImpact := impact
	replayedImpact.GeneratedAt = replayedImpact.GeneratedAt.Add(time.Minute)
	if err := repository.SaveImpactReportAndCheckpoint(context.Background(), lease, replayedImpact, checkpoint); err != nil {
		t.Fatalf("SaveImpactReportAndCheckpoint(idempotent) error = %v", err)
	}

	reopened, err := OpenGORMRepository(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	loadedImpact, found, err := reopened.LoadImpactReport(context.Background(), DefaultDatasourceID, impact.ReportHash)
	if err != nil || !found || loadedImpact != impact {
		t.Fatalf("restarted impact = %+v, %t, %v", loadedImpact, found, err)
	}
	loadedCheckpoint, found, err := reopened.LoadCheckpoint(context.Background(), checkpoint.Key)
	if err != nil || !found || loadedCheckpoint != checkpoint {
		t.Fatalf("restarted checkpoint = %+v, %t, %v", loadedCheckpoint, found, err)
	}
}

func TestGORMRepositoryPromotionSaveIsExactAndIdempotent(t *testing.T) {
	repository := prepareLifecycleTestRepository(t)
	promotion := Promotion{
		DatasourceID: DefaultDatasourceID, CurrentFingerprint: testLifecycleDigest("current"),
		PromotedFingerprint: testLifecycleDigest("promoted"), ImpactReportHash: testLifecycleDigest("impact"),
		ActorID: "security-admin", Reason: "approved-retention-change", ApprovalRef: "CAB-2026-0714",
		PromotedAt: time.Now().UTC().Truncate(time.Millisecond),
	}
	if err := repository.SavePromotion(context.Background(), promotion); err != nil {
		t.Fatalf("SavePromotion() error = %v", err)
	}
	replayedPromotion := promotion
	replayedPromotion.PromotedAt = replayedPromotion.PromotedAt.Add(time.Minute)
	if err := repository.SavePromotion(context.Background(), replayedPromotion); err != nil {
		t.Fatalf("SavePromotion(idempotent) error = %v", err)
	}
	conflict := promotion
	conflict.ActorID = "other-admin"
	if err := repository.SavePromotion(context.Background(), conflict); !errors.Is(err, ErrRepositoryFailed) {
		t.Fatalf("SavePromotion(conflict) error = %v, want ErrRepositoryFailed", err)
	}
	loaded, found, err := repository.LoadPromotion(context.Background(), DefaultDatasourceID, promotion.PromotedFingerprint)
	if err != nil || !found || loaded != promotion {
		t.Fatalf("LoadPromotion() = %+v, %t, %v", loaded, found, err)
	}
}

func openLifecycleTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := storage.OpenGORM(storage.Config{Driver: "sqlite", DSN: filepath.Join(t.TempDir(), "data-lifecycle.db")})
	if err != nil {
		t.Fatalf("OpenGORM() error = %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func prepareLifecycleTestRepository(t *testing.T) *GORMRepository {
	t.Helper()
	repository, err := PrepareGORMRepository(context.Background(), openLifecycleTestDB(t))
	if err != nil {
		t.Fatalf("PrepareGORMRepository() error = %v", err)
	}
	return repository
}

func acquireLifecycleTestLease(t *testing.T, repository *GORMRepository) Lease {
	t.Helper()
	now := time.Now().UTC()
	lease, err := repository.AcquireLease(context.Background(), LeaseRequest{
		DatasourceID: DefaultDatasourceID, Key: "data-lifecycle:default", OwnerID: "worker-1",
		PolicyFingerprint: testLifecycleDigest("policy"), Now: now, TTL: time.Hour,
	})
	if err != nil {
		t.Fatalf("AcquireLease() error = %v", err)
	}
	return lease
}

func lifecycleTestCheckpoint(runID string, mode Mode, revision uint64, recordID string, complete bool) Checkpoint {
	fingerprint := testLifecycleDigest("policy")
	checkpoint := Checkpoint{
		Key:          CheckpointKey{DatasourceID: DefaultDatasourceID, RunID: runID, Mode: mode, PolicyFingerprint: fingerprint},
		DatasourceID: DefaultDatasourceID,
		Cursor:       Cursor{DatasourceID: DefaultDatasourceID, Resource: "files", EligibleAt: "2026-06-01T00:00:00Z", RecordID: recordID},
		Counts:       Counts{Planned: int(revision), Batches: int(revision)}, EvidenceHash: testLifecycleDigest("evidence"),
		LastBatchID: testLifecycleDigest("batch"), Revision: revision, Complete: complete,
		UpdatedAt: time.Now().UTC().Truncate(time.Millisecond),
	}
	return sealCheckpoint(checkpoint)
}

func lifecycleTestImpact(checkpoint Checkpoint) ImpactReport {
	impact := ImpactReport{
		DatasourceID: checkpoint.DatasourceID, RunID: checkpoint.Key.RunID,
		PolicyFingerprint: checkpoint.Key.PolicyFingerprint, Counts: checkpoint.Counts,
		Cursor: sanitizedCursor(checkpoint.Cursor), EvidenceHash: checkpoint.EvidenceHash,
		GeneratedAt: time.Now().UTC().Truncate(time.Millisecond),
	}
	impact.ReportHash = impactReportDigest(impact)
	return impact
}

func testLifecycleDigest(value string) string {
	return stableDigest("test", value)
}
