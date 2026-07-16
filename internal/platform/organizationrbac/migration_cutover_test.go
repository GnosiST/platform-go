package organizationrbac

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/GnosiST/platform-go/internal/platform/httpapi"
)

func TestMenuPromotionValidationTransitionTable(t *testing.T) {
	db, repository := prepareOrganizationRBACTestRepository(t)
	manifest := testOrganizationRBACMigrationManifest()
	digest := strings.Repeat("a", 64)
	checkpoint := "checkpoint-cutover-1"
	if _, err := repository.ValidateMenuPromotion(context.Background(), string(httpapi.AdminMenuServingModeLegacy), false); err != nil {
		t.Fatalf("legacy without evidence = %v", err)
	}
	now := time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC)
	state := MenuPromotionState{RunID: "promotion-1", ManifestHash: digest, Phase: PromotionPrepared, FrozenRevision: 0, ActivePrincipals: 1, Equivalent: 1, ComparisonHash: digest, LegacySnapshotHash: digest, CheckpointRef: checkpoint, ObservedAt: now}
	if err := repository.RecordMenuPromotionState(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.ValidateMenuPromotion(context.Background(), string(httpapi.AdminMenuServingModeDualRead), false); err == nil {
		t.Fatal("prepared dual-read unexpectedly allowed")
	}
	state.Phase = PromotionDualRead
	if err := repository.RecordMenuPromotionState(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.ValidateMenuPromotion(context.Background(), string(httpapi.AdminMenuServingModeDualRead), false); err != nil {
		t.Fatalf("equal dual-read = %v", err)
	}
	state.Phase = PromotionTargetRead
	if err := repository.RecordMenuPromotionState(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.ValidateMenuPromotion(context.Background(), string(httpapi.AdminMenuServingModeTarget), false); err != nil {
		t.Fatalf("target read = %v", err)
	}
	if _, err := repository.ValidateMenuPromotion(context.Background(), string(httpapi.AdminMenuServingModeTarget), true); err == nil {
		t.Fatal("target writes allowed before target-write phase")
	}
	state.Phase = PromotionTargetWrite
	if err := repository.RecordMenuPromotionState(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.ValidateMenuPromotion(context.Background(), string(httpapi.AdminMenuServingModeTarget), true); err != nil {
		t.Fatalf("target write = %v", err)
	}
	if _, err := repository.ValidateMenuPromotion(context.Background(), string(httpapi.AdminMenuServingModeLegacy), false); err == nil {
		t.Fatal("legacy rollback allowed before verified restore")
	}
	_ = db
	_ = manifest
}

func TestMenuPromotionRevisionDriftRequiresNewComparison(t *testing.T) {
	db, repository := prepareOrganizationRBACTestRepository(t)
	digest := strings.Repeat("b", 64)
	revision, err := repository.CurrentGlobalRevision(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	state := MenuPromotionState{RunID: "promotion-drift-1", ManifestHash: digest, Phase: PromotionDualRead, FrozenRevision: revision, ActivePrincipals: 1, Equivalent: 1, ComparisonHash: digest, LegacySnapshotHash: digest, CheckpointRef: "checkpoint-drift-1", ObservedAt: time.Now().UTC()}
	if err := repository.RecordMenuPromotionState(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	if _, err := bumpGlobalRevision(db); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.ValidateMenuPromotion(context.Background(), string(httpapi.AdminMenuServingModeDualRead), false); err == nil {
		t.Fatal("revision drift unexpectedly allowed")
	}
}

func TestRecordMenuDualReadComparisonIsIdempotentAndMismatchFails(t *testing.T) {
	_, repository := prepareOrganizationRBACTestRepository(t)
	digest := strings.Repeat("c", 64)
	state := MenuPromotionState{RunID: "promotion-observation-1", ManifestHash: digest, Phase: PromotionDualRead, FrozenRevision: 0, ActivePrincipals: 1, Equivalent: 1, ComparisonHash: digest, LegacySnapshotHash: digest, CheckpointRef: "checkpoint-observation-1", ObservedAt: time.Now().UTC()}
	if err := repository.RecordMenuPromotionState(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	comparison := httpapi.AdminMenuComparison{Equal: true, GlobalRevision: 0}
	if err := repository.RecordMenuDualReadComparison(context.Background(), "principal-a", comparison); err != nil {
		t.Fatal(err)
	}
	if err := repository.RecordMenuDualReadComparison(context.Background(), "principal-a", comparison); err != nil {
		t.Fatal(err)
	}
	comparison.Equal = false
	if err := repository.RecordMenuDualReadComparison(context.Background(), "principal-a", comparison); err == nil || !errors.Is(err, ErrInvalid) && !strings.Contains(err.Error(), "observation") {
		t.Fatalf("mismatched observation error = %v", err)
	}
}

func TestRollbackUsesExternalSnapshotEvidenceAfterCheckpointRestore(t *testing.T) {
	_, repository := prepareOrganizationRBACTestRepository(t)
	seedOrganizationRBAC(t, repository.db)
	manifest := testOrganizationRBACMigrationManifest()
	snapshotHash, err := repository.LegacySnapshotHash(context.Background(), manifest)
	if err != nil {
		t.Fatal(err)
	}
	evidence := MigrationEvidence{
		RunID: "restored-checkpoint-run", ActorID: "admin", Reason: "checkpoint restore",
		ApprovalRef: "approval-restored", BackupURI: "file:///backup", BackupSHA256: strings.Repeat("d", 64),
		CheckpointRef: "checkpoint-restored", LegacySnapshotHash: snapshotHash, AppliedAt: time.Now().UTC(),
	}
	report, err := repository.RunMigration(context.Background(), MigrationRollback, manifest, evidence)
	if err != nil {
		t.Fatalf("rollback after restore = %v", err)
	}
	if report.Status != "rolled-back-verified" {
		t.Fatalf("rollback status = %q", report.Status)
	}
}
