package organizationrbac

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/GnosiST/platform-go/internal/platform/httpapi"
	"gorm.io/gorm"
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

func TestPromoteMenuWrites(t *testing.T) {
	request := MenuWritePromotionRequest{
		RunID: "promotion-write-1", ExpectedPhase: PromotionTargetRead,
		ActorID: "migration-admin", Reason: "approved target writes", ApprovalRef: "change-write-1",
		ObservedAt: time.Date(2026, 7, 17, 9, 0, 0, 0, time.UTC),
	}

	t.Run("advances a complete target-read state and replays idempotently", func(t *testing.T) {
		db, repository := prepareOrganizationRBACTestRepository(t)
		state := completeTargetReadPromotionState(t, repository, request.RunID)
		if err := repository.RecordMenuPromotionState(context.Background(), state); err != nil {
			t.Fatal(err)
		}

		promoted, err := repository.PromoteMenuWrites(context.Background(), request)
		if err != nil {
			t.Fatalf("PromoteMenuWrites() error = %v", err)
		}
		if promoted.Phase != PromotionTargetWrite || promoted.RunID != request.RunID || !promoted.ObservedAt.Equal(request.ObservedAt) {
			t.Fatalf("promoted state = %+v", promoted)
		}
		assertMenuPromotionEventCount(t, db, request.RunID, 2)
		var event gormOrganizationRBACPromotionEvent
		if err := db.Where("run_id = ? AND sequence = ?", request.RunID, 2).Take(&event).Error; err != nil {
			t.Fatal(err)
		}
		if event.FromPhase != PromotionTargetRead || event.ToPhase != PromotionTargetWrite || event.ActorID != request.ActorID || event.Reason != request.Reason || event.ApprovalRef != request.ApprovalRef || !event.ObservedAt.Equal(request.ObservedAt) {
			t.Fatalf("promotion event = %+v", event)
		}

		replayed, err := repository.PromoteMenuWrites(context.Background(), request)
		if err != nil {
			t.Fatalf("PromoteMenuWrites(replay) error = %v", err)
		}
		if replayed != promoted {
			t.Fatalf("replayed state = %+v, want %+v", replayed, promoted)
		}
		assertMenuPromotionEventCount(t, db, request.RunID, 2)
	})

	for _, tc := range []struct {
		name    string
		state   func(*testing.T, *GORMRepository) MenuPromotionState
		request func(MenuWritePromotionRequest) MenuWritePromotionRequest
		prepare func(*testing.T, *gorm.DB)
	}{
		{
			name: "wrong current phase",
			state: func(t *testing.T, repository *GORMRepository) MenuPromotionState {
				state := completeTargetReadPromotionState(t, repository, request.RunID)
				state.Phase = PromotionDualRead
				return state
			},
		},
		{
			name: "wrong expected phase",
			state: func(t *testing.T, repository *GORMRepository) MenuPromotionState {
				return completeTargetReadPromotionState(t, repository, request.RunID)
			},
			request: func(request MenuWritePromotionRequest) MenuWritePromotionRequest {
				request.ExpectedPhase = PromotionDualRead
				return request
			},
		},
		{
			name: "revision drift",
			state: func(t *testing.T, repository *GORMRepository) MenuPromotionState {
				return completeTargetReadPromotionState(t, repository, request.RunID)
			},
			prepare: func(t *testing.T, db *gorm.DB) {
				if _, err := bumpGlobalRevision(db); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "incomplete principal equivalence",
			state: func(t *testing.T, repository *GORMRepository) MenuPromotionState {
				state := completeTargetReadPromotionState(t, repository, request.RunID)
				state.ActivePrincipals = 2
				state.Equivalent = 1
				return state
			},
		},
		{
			name: "missing audit fields",
			state: func(t *testing.T, repository *GORMRepository) MenuPromotionState {
				return completeTargetReadPromotionState(t, repository, request.RunID)
			},
			request: func(request MenuWritePromotionRequest) MenuWritePromotionRequest {
				request.ActorID = ""
				return request
			},
		},
		{
			name: "different run ID",
			state: func(t *testing.T, repository *GORMRepository) MenuPromotionState {
				return completeTargetReadPromotionState(t, repository, request.RunID)
			},
			request: func(request MenuWritePromotionRequest) MenuWritePromotionRequest {
				request.RunID = "promotion-write-other"
				return request
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, repository := prepareOrganizationRBACTestRepository(t)
			state := tc.state(t, repository)
			if err := repository.RecordMenuPromotionState(context.Background(), state); err != nil {
				t.Fatal(err)
			}
			if tc.prepare != nil {
				tc.prepare(t, db)
			}
			failedRequest := request
			if tc.request != nil {
				failedRequest = tc.request(failedRequest)
			}
			if _, err := repository.PromoteMenuWrites(context.Background(), failedRequest); err == nil {
				t.Fatal("PromoteMenuWrites() error = nil")
			}
			var current gormOrganizationRBACPromotion
			if err := db.Where("run_id = ?", state.RunID).Take(&current).Error; err != nil {
				t.Fatal(err)
			}
			if current.Phase != state.Phase || current.FrozenRevision != state.FrozenRevision || current.ActivePrincipals != state.ActivePrincipals || current.Equivalent != state.Equivalent || !current.ObservedAt.Equal(state.ObservedAt) {
				t.Fatalf("state changed after rejected promotion = %+v, want %+v", current, state)
			}
			assertMenuPromotionEventCount(t, db, state.RunID, 1)
		})
	}
}

func completeTargetReadPromotionState(t *testing.T, repository *GORMRepository, runID string) MenuPromotionState {
	t.Helper()
	revision, err := repository.CurrentGlobalRevision(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	digest := strings.Repeat("e", 64)
	return MenuPromotionState{
		RunID: runID, ManifestHash: digest, Phase: PromotionTargetRead, FrozenRevision: revision,
		ActivePrincipals: 1, Equivalent: 1, ComparisonHash: digest, LegacySnapshotHash: digest,
		CheckpointRef: "checkpoint-write-1", ObservedAt: time.Date(2026, 7, 17, 8, 0, 0, 0, time.UTC),
	}
}

func assertMenuPromotionEventCount(t *testing.T, db *gorm.DB, runID string, want int64) {
	t.Helper()
	var got int64
	if err := db.Model(&gormOrganizationRBACPromotionEvent{}).Where("run_id = ?", runID).Count(&got).Error; err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("promotion event count = %d, want %d", got, want)
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
