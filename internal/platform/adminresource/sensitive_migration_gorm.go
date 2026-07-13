package adminresource

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"hash"
	"slices"
	"strconv"
	"strings"
	"time"

	"platform-go/internal/platform/dataprotection"
	"platform-go/internal/platform/sensitivemigration"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

const (
	sensitiveMigrationRunsTable            = "platform_sensitive_migration_runs"
	sensitiveMigrationTargetsTable         = "platform_sensitive_migration_targets"
	sensitiveMigrationCheckpointsTable     = "platform_sensitive_migration_checkpoints"
	sensitiveMigrationEventsTable          = "platform_sensitive_migration_events"
	sensitiveMigrationEscrowTable          = "platform_sensitive_migration_escrow"
	sensitiveMigrationEscrowResource       = "migration-rollback"
	sensitiveMigrationTargetWriteBatchSize = 100
	sensitiveMigrationPendingTargetSetHash = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
)

var (
	ErrMigrationInvalidOptions    = errors.New("sensitive migration invalid options")
	ErrMigrationUnsupportedDriver = errors.New("sensitive migration unsupported driver")
	ErrMigrationPhysicalLayout    = errors.New("sensitive migration incompatible physical layout")
	ErrMigrationConflict          = errors.New("sensitive migration conflict")
	ErrMigrationStore             = errors.New("sensitive migration store failed")
)

type GORMProtectedValueMigrationStore struct {
	db     *gorm.DB
	driver string
}

type gormSensitiveMigrationRun struct {
	RunID              string `gorm:"column:run_id;size:64;primaryKey"`
	PlanHash           string `gorm:"column:plan_hash;size:71;not null"`
	TargetSetHash      string `gorm:"column:target_set_hash;size:71;not null"`
	ActorID            string `gorm:"column:actor_id;size:128;not null"`
	Reason             string `gorm:"column:reason;size:191;not null"`
	ApprovalRef        string `gorm:"column:approval_ref;size:191;not null"`
	BackupURI          string `gorm:"column:backup_uri;size:191;not null"`
	BackupHash         string `gorm:"column:backup_hash;size:71;not null"`
	RestoreEvidence    string `gorm:"column:restore_evidence;size:191;not null"`
	MaintenanceOK      bool   `gorm:"column:maintenance_confirmed;not null"`
	Status             string `gorm:"column:status;size:32;not null"`
	ExpectedRevision   uint64 `gorm:"column:expected_revision;not null"`
	TargetCount        int    `gorm:"column:target_count;not null"`
	RestoreRehearsed   bool   `gorm:"column:restore_rehearsed;not null;default:false"`
	RestoreRehearsedAt string `gorm:"column:restore_rehearsed_at;size:35;not null;default:''"`
	RollbackStatus     string `gorm:"column:rollback_status;size:32;not null;default:none"`
	CreatedAt          string `gorm:"column:created_at;size:35;not null"`
}

type gormSensitiveMigrationTarget struct {
	TargetID         string `gorm:"column:target_id;size:64;primaryKey;index:idx_sensitive_target_lookup,priority:5;index:idx_sensitive_target_cursor,priority:2"`
	RunID            string `gorm:"column:run_id;size:64;not null;index:idx_sensitive_target_lookup,priority:1;index:idx_sensitive_target_cursor,priority:1"`
	Resource         string `gorm:"column:resource;size:128;not null;index:idx_sensitive_target_lookup,priority:2"`
	TenantScope      string `gorm:"column:tenant_scope;size:191;not null"`
	TenantScopeHash  string `gorm:"column:tenant_scope_hash;size:64;not null;index:idx_sensitive_target_lookup,priority:3"`
	RecordID         string `gorm:"column:record_id;size:191;not null"`
	RecordIDHash     string `gorm:"column:record_id_hash;size:64;not null;index:idx_sensitive_target_lookup,priority:4"`
	FieldKey         string `gorm:"column:field_key;size:128;not null"`
	ResourcePlanHash string `gorm:"column:resource_plan_hash;size:71;not null"`
	SnapshotHash     string `gorm:"column:snapshot_hash;size:71;not null"`
}

type gormSensitiveMigrationCheckpoint struct {
	CheckpointID     string `gorm:"column:checkpoint_id;size:64;primaryKey;index:idx_sensitive_checkpoint_cursor,priority:2"`
	RunID            string `gorm:"column:run_id;size:64;not null;index:idx_sensitive_checkpoint_lookup,priority:1;index:idx_sensitive_checkpoint_cursor,priority:1"`
	Resource         string `gorm:"column:resource;size:128;not null;index:idx_sensitive_checkpoint_lookup,priority:2"`
	TenantScope      string `gorm:"column:tenant_scope;size:191;not null"`
	TenantScopeHash  string `gorm:"column:tenant_scope_hash;size:64;not null;index:idx_sensitive_checkpoint_lookup,priority:3"`
	Mode             string `gorm:"column:mode;size:32;not null;index:idx_sensitive_checkpoint_lookup,priority:4"`
	LastRecordID     string `gorm:"column:last_record_id;size:191;not null"`
	ExpectedRevision uint64 `gorm:"column:expected_revision;not null"`
	Rows             int    `gorm:"column:row_count;not null"`
	Missing          int    `gorm:"column:missing_count;not null"`
	Plaintext        int    `gorm:"column:plaintext_count;not null"`
	TargetEnvelope   int    `gorm:"column:target_envelope_count;not null"`
	ForeignEnvelope  int    `gorm:"column:foreign_envelope_count;not null"`
	Malformed        int    `gorm:"column:malformed_envelope_count;not null"`
	Batches          int    `gorm:"column:batch_count;not null"`
	Status           string `gorm:"column:status;size:32;not null"`
	EventSequence    uint64 `gorm:"column:event_sequence;not null"`
	EventHash        string `gorm:"column:event_hash;size:71;not null"`
	UpdatedAt        string `gorm:"column:updated_at;size:35;not null"`
}

type gormSensitiveMigrationEvent struct {
	EventID         string `gorm:"column:event_id;size:64;primaryKey"`
	RunID           string `gorm:"column:run_id;size:64;not null;uniqueIndex:idx_sensitive_event_sequence,priority:1;index:idx_sensitive_event_scope,priority:1"`
	Sequence        uint64 `gorm:"column:sequence;not null;uniqueIndex:idx_sensitive_event_sequence,priority:2;index:idx_sensitive_event_scope,priority:5"`
	Mode            string `gorm:"column:mode;size:32;not null;index:idx_sensitive_event_scope,priority:4"`
	Resource        string `gorm:"column:resource;size:128;not null;index:idx_sensitive_event_scope,priority:2"`
	TenantScopeHash string `gorm:"column:tenant_scope_hash;size:64;not null;index:idx_sensitive_event_scope,priority:3"`
	LastRecordID    string `gorm:"column:last_record_id;size:191;not null"`
	Rows            int    `gorm:"column:row_count;not null"`
	Missing         int    `gorm:"column:missing_count;not null"`
	Plaintext       int    `gorm:"column:plaintext_count;not null"`
	TargetEnvelope  int    `gorm:"column:target_envelope_count;not null"`
	ForeignEnvelope int    `gorm:"column:foreign_envelope_count;not null"`
	Malformed       int    `gorm:"column:malformed_envelope_count;not null"`
	EscrowSetHash   string `gorm:"column:escrow_set_hash;size:71;not null;default:''"`
	Revision        uint64 `gorm:"column:revision;not null"`
	PriorEventHash  string `gorm:"column:prior_event_hash;size:71;not null"`
	EventHash       string `gorm:"column:event_hash;size:71;not null"`
	CreatedAt       string `gorm:"column:created_at;size:35;not null"`
}

type gormSensitiveMigrationEscrow struct {
	EscrowID          string `gorm:"column:escrow_id;size:64;primaryKey;index:idx_sensitive_escrow_lookup,priority:5;index:idx_sensitive_escrow_cursor,priority:2"`
	RunID             string `gorm:"column:run_id;size:64;not null;index:idx_sensitive_escrow_lookup,priority:1;index:idx_sensitive_escrow_cursor,priority:1"`
	TargetID          string `gorm:"column:target_id;size:64;not null"`
	Resource          string `gorm:"column:resource;size:128;not null;index:idx_sensitive_escrow_lookup,priority:2"`
	TenantScopeHash   string `gorm:"column:tenant_scope_hash;size:64;not null;index:idx_sensitive_escrow_lookup,priority:3"`
	RecordID          string `gorm:"column:record_id;size:191;not null"`
	RecordIDHash      string `gorm:"column:record_id_hash;size:64;not null;index:idx_sensitive_escrow_lookup,priority:4"`
	FieldKey          string `gorm:"column:field_key;size:128;not null"`
	ProtectedOriginal string `gorm:"column:protected_original;type:text;not null"`
	MigratedValueHash string `gorm:"column:migrated_value_hash;size:71;not null"`
}

func (gormSensitiveMigrationRun) TableName() string        { return sensitiveMigrationRunsTable }
func (gormSensitiveMigrationTarget) TableName() string     { return sensitiveMigrationTargetsTable }
func (gormSensitiveMigrationCheckpoint) TableName() string { return sensitiveMigrationCheckpointsTable }
func (gormSensitiveMigrationEvent) TableName() string      { return sensitiveMigrationEventsTable }
func (gormSensitiveMigrationEscrow) TableName() string     { return sensitiveMigrationEscrowTable }

func NewGORMProtectedValueMigrationStore(db *gorm.DB, driver string) (*GORMProtectedValueMigrationStore, error) {
	driver = strings.TrimSpace(driver)
	if db == nil || db.Dialector == nil {
		return nil, ErrMigrationInvalidOptions
	}
	if driver != "mysql" && driver != "postgres" && driver != "sqlite" {
		return nil, ErrMigrationUnsupportedDriver
	}
	if db.Dialector.Name() != driver {
		return nil, ErrMigrationUnsupportedDriver
	}
	return &GORMProtectedValueMigrationStore{
		db: db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}), driver: driver,
	}, nil
}

func (s *GORMProtectedValueMigrationStore) Rows(ctx context.Context, plan sensitivemigration.ResourcePlan, after string, limit int) ([]sensitivemigration.Row, error) {
	layout, generic, err := migrationLayout(plan)
	if err != nil {
		return nil, err
	}
	if ctx == nil || ctx.Err() != nil || limit < 1 || limit > sensitivemigration.MaximumBatchSize {
		return nil, ErrMigrationInvalidOptions
	}
	return readMigrationRows(s.db.WithContext(ctx), layout, generic, plan, after, limit)
}

func readMigrationRows(db *gorm.DB, layout gormAdminResourceLayout, generic bool, plan sensitivemigration.ResourcePlan, after string, limit int) ([]sensitivemigration.Row, error) {
	physicalRows, err := readMigrationPhysicalRows(db, layout, generic, plan.Resource, after, limit)
	if err != nil {
		return nil, err
	}
	rows := make([]sensitivemigration.Row, 0, len(physicalRows))
	for _, row := range physicalRows {
		tenant := dataprotection.GlobalTenantID
		if plan.Scope == "tenant-field" {
			values, parseErr := migrationValues(row.ValuesJSON)
			if parseErr != nil {
				return nil, ErrMigrationStore
			}
			rowTenant, ok := values[plan.TenantField].(string)
			tenant = strings.TrimSpace(rowTenant)
			if !ok || tenant == "" {
				return nil, ErrMigrationStore
			}
		}
		rows = append(rows, sensitivemigration.Row{Resource: plan.Resource, TenantID: tenant, RecordID: row.ID, ValuesJSON: row.ValuesJSON})
	}
	return rows, nil
}

func (s *GORMProtectedValueMigrationStore) Prepare(ctx context.Context, request sensitivemigration.RunRequest) (sensitivemigration.RunState, error) {
	if s == nil || s.db == nil || ctx == nil || ctx.Err() != nil || !sensitivemigration.ValidMutationRequest(request) || len(request.Plan.Resources) == 0 || sensitivemigration.PlanHash(request.Plan) != request.PlanHash {
		return sensitivemigration.RunState{}, ErrMigrationInvalidOptions
	}
	for _, plan := range request.Plan.Resources {
		if _, _, err := migrationLayout(plan); err != nil {
			return sensitivemigration.RunState{}, err
		}
	}
	if err := s.db.WithContext(ctx).AutoMigrate(
		&gormSensitiveMigrationRun{}, &gormSensitiveMigrationTarget{}, &gormSensitiveMigrationCheckpoint{},
		&gormSensitiveMigrationEvent{}, &gormSensitiveMigrationEscrow{},
	); err != nil {
		return sensitivemigration.RunState{}, ErrMigrationStore
	}
	if existing, found, err := s.preparedRun(s.db.WithContext(ctx), request.RunID); err != nil {
		return sensitivemigration.RunState{}, err
	} else if found {
		if !runMatchesRequest(existing, request) {
			return sensitivemigration.RunState{}, ErrMigrationConflict
		}
		if err := validateMigrationTargetSet(s.db.WithContext(ctx), existing); err != nil {
			return sensitivemigration.RunState{}, err
		}
		return runState(existing), nil
	}

	revision, err := loadGORMRevision(s.db.WithContext(ctx))
	if err != nil {
		return sensitivemigration.RunState{}, ErrMigrationStore
	}
	var state sensitivemigration.RunState
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sealedRevision, err := s.lockedRevision(tx)
		if err != nil {
			return err
		}
		if sealedRevision != revision {
			return ErrMigrationConflict
		}
		run := gormSensitiveMigrationRun{
			RunID: request.RunID, PlanHash: request.PlanHash, TargetSetHash: sensitiveMigrationPendingTargetSetHash,
			ActorID: request.ActorID, Reason: request.Reason,
			ApprovalRef: request.ApprovalRef, BackupURI: request.BackupURI, BackupHash: request.BackupHash,
			RestoreEvidence: request.RestoreEvidence, MaintenanceOK: request.MaintenanceConfirmed, Status: sensitivemigration.StatusPrepared,
			ExpectedRevision: revision, TargetCount: 0, RollbackStatus: sensitivemigration.StatusNone, CreatedAt: migrationTimestamp(),
		}
		created := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "run_id"}}, DoNothing: true}).Create(&run)
		if created.Error != nil {
			return ErrMigrationStore
		}
		if created.RowsAffected == 0 {
			existing, found, err := s.preparedRun(tx, request.RunID)
			if err != nil || !found {
				return ErrMigrationStore
			}
			if !runMatchesRequest(existing, request) {
				return ErrMigrationConflict
			}
			if err := validateMigrationTargetSet(tx, existing); err != nil {
				return err
			}
			state = runState(existing)
			return nil
		}
		targetCount, targetSetHash, err := s.materializeMigrationTargets(ctx, tx, request)
		if err != nil {
			return err
		}
		updated := tx.Model(&gormSensitiveMigrationRun{}).
			Where("run_id = ? AND target_set_hash = ? AND target_count = ?", request.RunID, sensitiveMigrationPendingTargetSetHash, 0).
			Updates(map[string]any{"target_set_hash": targetSetHash, "target_count": targetCount})
		if updated.Error != nil || updated.RowsAffected != 1 {
			return ErrMigrationConflict
		}
		run.TargetCount = targetCount
		run.TargetSetHash = targetSetHash
		if err := validateMigrationTargetSet(tx, run); err != nil {
			return err
		}
		state = runState(run)
		return nil
	})
	if err != nil {
		return sensitivemigration.RunState{}, err
	}
	return state, nil
}

func (s *GORMProtectedValueMigrationStore) preparedRun(db *gorm.DB, runID string) (gormSensitiveMigrationRun, bool, error) {
	var run gormSensitiveMigrationRun
	err := db.Where("run_id = ? AND status = ?", runID, sensitivemigration.StatusPrepared).First(&run).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return gormSensitiveMigrationRun{}, false, nil
	}
	if err != nil {
		return gormSensitiveMigrationRun{}, false, ErrMigrationStore
	}
	return run, true, nil
}

func (s *GORMProtectedValueMigrationStore) StartOrResume(ctx context.Context, request sensitivemigration.RunRequest) (sensitivemigration.RunState, error) {
	if s == nil || s.db == nil || ctx == nil || ctx.Err() != nil || !sensitivemigration.ValidRunIdentity(request) {
		return sensitivemigration.RunState{}, ErrMigrationInvalidOptions
	}
	if (request.Mode == sensitivemigration.ModeApply || request.Mode == sensitivemigration.ModeRehearseRestore || request.Mode == sensitivemigration.ModeRollback) && !sensitivemigration.ValidMutationRequest(request) ||
		request.Mode != sensitivemigration.ModeApply && request.Mode != sensitivemigration.ModeVerify && request.Mode != sensitivemigration.ModeRehearseRestore && request.Mode != sensitivemigration.ModeRollback {
		return sensitivemigration.RunState{}, ErrMigrationInvalidOptions
	}
	if len(request.Plan.Resources) == 0 || sensitivemigration.PlanHash(request.Plan) != request.PlanHash {
		return sensitivemigration.RunState{}, ErrMigrationInvalidOptions
	}
	var state sensitivemigration.RunState
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var run gormSensitiveMigrationRun
		query := tx.Where("run_id = ?", request.RunID)
		if s.driver != "sqlite" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := query.First(&run).Error; err != nil {
			return ErrMigrationConflict
		}
		if run.PlanHash != request.PlanHash || run.Status != sensitivemigration.StatusPrepared && run.Status != sensitivemigration.StatusCompleted {
			return ErrMigrationConflict
		}
		if (request.Mode == sensitivemigration.ModeApply || request.Mode == sensitivemigration.ModeRehearseRestore || request.Mode == sensitivemigration.ModeRollback) && !runMatchesRequest(run, request) {
			return ErrMigrationConflict
		}
		if (request.Mode == sensitivemigration.ModeVerify || request.Mode == sensitivemigration.ModeRehearseRestore || request.Mode == sensitivemigration.ModeRollback) && run.Status != sensitivemigration.StatusCompleted {
			return ErrMigrationConflict
		}
		if err := validateMigrationTargetSet(tx, run); err != nil {
			return err
		}
		journal, err := verifyMigrationJournal(tx, request.RunID)
		if err != nil {
			return err
		}
		state = runState(run)
		state.EventChainHead = journal.Head
		state.Counts = journal.ApplyCounts
		state.CheckpointBatches = journal.ApplyBatches
		state.RollbackCounts = journal.RollbackCounts
		state.RollbackBatches = journal.RollbackBatches
		if request.Mode == sensitivemigration.ModeRehearseRestore || request.Mode == sensitivemigration.ModeRollback {
			escrowHash, escrowCount, err := migrationEscrowSetSummary(tx, request.RunID)
			if err != nil {
				return err
			}
			if escrowCount != journal.ApplyCounts.Plaintext {
				return ErrMigrationConflict
			}
			if run.RestoreRehearsed {
				if len(journal.RehearsalEvents) != 1 || journal.RehearsalEvents[0].Rows != escrowCount || journal.RehearsalEvents[0].EscrowSetHash != escrowHash {
					return ErrMigrationConflict
				}
			} else if len(journal.RehearsalEvents) != 0 {
				return ErrMigrationConflict
			}
			state.EscrowCount = escrowCount
		} else {
			state.EscrowCount = state.Counts.Plaintext
		}
		if migrationCountsTotal(state.Counts) > state.TargetCount {
			return ErrMigrationConflict
		}
		return nil
	})
	if err != nil {
		return sensitivemigration.RunState{}, err
	}
	return state, nil
}

func (s *GORMProtectedValueMigrationStore) Checkpoint(ctx context.Context, runID string, plan sensitivemigration.ResourcePlan, tenant string, mode sensitivemigration.Mode) (sensitivemigration.CheckpointState, bool, error) {
	if s == nil || s.db == nil || ctx == nil || ctx.Err() != nil || !canonicalMigrationRunID(runID) || strings.TrimSpace(tenant) == "" ||
		mode != sensitivemigration.ModeApply && mode != sensitivemigration.ModeRollback {
		return sensitivemigration.CheckpointState{}, false, ErrMigrationInvalidOptions
	}
	if _, _, err := migrationLayout(plan); err != nil {
		return sensitivemigration.CheckpointState{}, false, err
	}
	var checkpoint gormSensitiveMigrationCheckpoint
	err := s.db.WithContext(ctx).Where(
		"run_id = ? AND resource = ? AND tenant_scope_hash = ? AND mode = ?",
		runID, plan.Resource, sensitivemigration.TenantCursor(tenant), string(mode),
	).First(&checkpoint).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return sensitivemigration.CheckpointState{}, false, nil
	}
	if err != nil {
		return sensitivemigration.CheckpointState{}, false, ErrMigrationStore
	}
	if checkpoint.TenantScope != tenant || checkpoint.Status != sensitivemigration.StatusCompleted {
		return sensitivemigration.CheckpointState{}, false, ErrMigrationConflict
	}
	return sensitivemigration.CheckpointState{
		Resource: checkpoint.Resource, TenantID: checkpoint.TenantScope, LastRecordID: checkpoint.LastRecordID,
		Counts: checkpointCounts(checkpoint), Batches: checkpoint.Batches, EventHash: checkpoint.EventHash,
	}, true, nil
}

func (s *GORMProtectedValueMigrationStore) TargetScopes(ctx context.Context, runID string, plan sensitivemigration.ResourcePlan, after string, limit int) ([]string, error) {
	if s == nil || s.db == nil || ctx == nil || ctx.Err() != nil || !canonicalMigrationRunID(runID) || limit < 1 || limit > sensitivemigration.MaximumBatchSize {
		return nil, ErrMigrationInvalidOptions
	}
	if _, _, err := migrationLayout(plan); err != nil {
		return nil, err
	}
	var scopeHashes []string
	query := s.db.WithContext(ctx).Model(&gormSensitiveMigrationTarget{}).
		Where("run_id = ? AND resource = ?", runID, plan.Resource)
	if after != "" {
		query = query.Where("tenant_scope_hash > ?", sensitivemigration.TenantCursor(after))
	}
	if err := query.Distinct("tenant_scope_hash").Order("tenant_scope_hash").Limit(limit).Pluck("tenant_scope_hash", &scopeHashes).Error; err != nil {
		return nil, ErrMigrationStore
	}
	scopes := make([]string, 0, len(scopeHashes))
	for _, scopeHash := range scopeHashes {
		var matchingScopes []string
		if !canonicalMigrationCursor(scopeHash) || s.db.WithContext(ctx).Model(&gormSensitiveMigrationTarget{}).
			Where("run_id = ? AND resource = ? AND tenant_scope_hash = ?", runID, plan.Resource, scopeHash).
			Limit(1).Pluck("tenant_scope", &matchingScopes).Error != nil || len(matchingScopes) != 1 ||
			strings.TrimSpace(matchingScopes[0]) == "" || sensitivemigration.TenantCursor(matchingScopes[0]) != scopeHash {
			return nil, ErrMigrationConflict
		}
		scopes = append(scopes, matchingScopes[0])
	}
	return scopes, nil
}

func (s *GORMProtectedValueMigrationStore) TargetRows(ctx context.Context, runID string, plan sensitivemigration.ResourcePlan, tenant string, after string, limit int) ([]sensitivemigration.Row, error) {
	if s == nil || s.db == nil || ctx == nil || ctx.Err() != nil || !canonicalMigrationRunID(runID) || strings.TrimSpace(tenant) == "" || limit < 1 || limit > sensitivemigration.MaximumBatchSize {
		return nil, ErrMigrationInvalidOptions
	}
	layout, generic, err := migrationLayout(plan)
	if err != nil {
		return nil, err
	}
	tenantHash := sensitivemigration.TenantCursor(tenant)
	var recordHashes []string
	query := s.db.WithContext(ctx).Model(&gormSensitiveMigrationTarget{}).
		Where("run_id = ? AND resource = ? AND tenant_scope_hash = ?", runID, plan.Resource, tenantHash)
	if after != "" {
		query = query.Where("record_id_hash > ?", sensitivemigration.RecordCursor(after))
	}
	if err := query.Distinct("record_id_hash").Order("record_id_hash").Limit(limit).Pluck("record_id_hash", &recordHashes).Error; err != nil {
		return nil, ErrMigrationStore
	}
	rows := make([]sensitivemigration.Row, 0, len(recordHashes))
	for _, recordHash := range recordHashes {
		var targets []gormSensitiveMigrationTarget
		if err := s.db.WithContext(ctx).Where(
			"run_id = ? AND resource = ? AND tenant_scope_hash = ? AND record_id_hash = ?", runID, plan.Resource, tenantHash, recordHash,
		).Order("target_id").Limit(len(plan.Fields) + 1).Find(&targets).Error; err != nil || !targetsMatchPlan(targets, plan) {
			return nil, ErrMigrationConflict
		}
		recordID := targets[0].RecordID
		for _, target := range targets {
			if target.TenantScope != tenant || target.RecordID != recordID || target.RecordIDHash != recordHash || sensitivemigration.RecordCursor(recordID) != recordHash {
				return nil, ErrMigrationConflict
			}
		}
		var physical migrationPhysicalRow
		physicalQuery := s.db.WithContext(ctx).Table(layout.Table).Select("id, values_json").Where("id = ?", recordID)
		if generic {
			physicalQuery = physicalQuery.Where("resource = ?", plan.Resource)
		}
		if err := physicalQuery.Take(&physical).Error; err != nil || physical.ID != recordID || physical.ValuesJSON == "" {
			return nil, ErrMigrationConflict
		}
		rows = append(rows, sensitivemigration.Row{Resource: plan.Resource, TenantID: tenant, RecordID: recordID, ValuesJSON: physical.ValuesJSON})
	}
	return rows, nil
}

func (s *GORMProtectedValueMigrationStore) FinishRun(ctx context.Context, runID string, status string) error {
	if s == nil || s.db == nil || ctx == nil || ctx.Err() != nil || !canonicalMigrationRunID(runID) || status != sensitivemigration.StatusCompleted {
		return ErrMigrationInvalidOptions
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var run gormSensitiveMigrationRun
		if err := tx.Where("run_id = ?", runID).First(&run).Error; err != nil {
			return ErrMigrationConflict
		}
		if run.Status != sensitivemigration.StatusPrepared && run.Status != sensitivemigration.StatusCompleted {
			return ErrMigrationConflict
		}
		if err := validateMigrationTargetSet(tx, run); err != nil {
			return err
		}
		journal, err := verifyMigrationJournal(tx, runID)
		if err != nil {
			return err
		}
		if migrationCountsTotal(journal.ApplyCounts) != run.TargetCount {
			return ErrMigrationConflict
		}
		if run.Status == sensitivemigration.StatusCompleted {
			return nil
		}
		updated := tx.Model(&gormSensitiveMigrationRun{}).
			Where("run_id = ? AND status = ?", runID, sensitivemigration.StatusPrepared).
			Update("status", status)
		if updated.Error != nil || updated.RowsAffected != 1 {
			return ErrMigrationConflict
		}
		return nil
	})
}

func (s *GORMProtectedValueMigrationStore) ValidateRun(ctx context.Context, runID string) error {
	if s == nil || s.db == nil || ctx == nil || ctx.Err() != nil || !canonicalMigrationRunID(runID) {
		return ErrMigrationInvalidOptions
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var run gormSensitiveMigrationRun
		if err := tx.Where("run_id = ?", runID).First(&run).Error; err != nil {
			return ErrMigrationConflict
		}
		if err := validateMigrationTargetSet(tx, run); err != nil {
			return err
		}
		_, err := verifyMigrationJournal(tx, runID)
		return err
	})
}

func (s *GORMProtectedValueMigrationStore) EscrowEntries(ctx context.Context, runID string, after sensitivemigration.EscrowCursor, limit int) ([]sensitivemigration.EscrowEntry, error) {
	if s == nil || s.db == nil || ctx == nil || ctx.Err() != nil || !canonicalMigrationRunID(runID) || limit < 1 || limit > sensitivemigration.MaximumBatchSize ||
		after.RunID != "" && (after.RunID != runID || after.TenantID == "" || after.Resource == "" || after.RecordID == "" || after.FieldKey == "") {
		return nil, ErrMigrationInvalidOptions
	}
	var count int64
	if err := s.db.WithContext(ctx).Model(&gormSensitiveMigrationRun{}).Where("run_id = ? AND status = ?", runID, sensitivemigration.StatusCompleted).Count(&count).Error; err != nil {
		return nil, ErrMigrationStore
	}
	if count != 1 {
		return nil, ErrMigrationConflict
	}
	return loadMigrationEscrowPage(s.db.WithContext(ctx), runID, after, limit)
}

func (s *GORMProtectedValueMigrationStore) CommitRehearsal(ctx context.Context, runID string, count int, escrowHash string) (sensitivemigration.BatchCommit, error) {
	if s == nil || s.db == nil || ctx == nil || ctx.Err() != nil || !canonicalMigrationRunID(runID) || count < 0 || !canonicalMigrationHash(escrowHash) {
		return sensitivemigration.BatchCommit{}, ErrMigrationInvalidOptions
	}
	commit := sensitivemigration.BatchCommit{}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var run gormSensitiveMigrationRun
		query := tx.Where("run_id = ? AND status = ?", runID, sensitivemigration.StatusCompleted)
		if s.driver != "sqlite" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := query.First(&run).Error; err != nil || run.RollbackStatus != sensitivemigration.StatusNone && run.RollbackStatus != sensitivemigration.StatusCompleted {
			return ErrMigrationConflict
		}
		if err := validateMigrationTargetSet(tx, run); err != nil {
			return err
		}
		journal, err := verifyMigrationJournal(tx, runID)
		if err != nil {
			return err
		}
		actualHash, actualCount, err := migrationEscrowSetSummary(tx, runID)
		if err != nil {
			return err
		}
		if actualCount != journal.ApplyCounts.Plaintext || actualCount != count {
			return ErrMigrationConflict
		}
		if actualHash != escrowHash {
			return ErrMigrationConflict
		}
		if run.RestoreRehearsed {
			if len(journal.RehearsalEvents) != 1 || journal.RehearsalEvents[0].Rows != count || journal.RehearsalEvents[0].EscrowSetHash != escrowHash {
				return ErrMigrationConflict
			}
			event := journal.RehearsalEvents[0]
			commit = sensitivemigration.BatchCommit{Revision: run.ExpectedRevision, Rows: count, EventSequence: event.Sequence, EventHash: event.EventHash}
			return nil
		}
		if len(journal.RehearsalEvents) != 0 {
			return ErrMigrationConflict
		}
		sequence := journal.Sequence + 1
		now := migrationTimestamp()
		event := gormSensitiveMigrationEvent{
			EventID: migrationSurrogateID("event", runID, strconv.FormatUint(sequence, 10)), RunID: runID,
			Sequence: sequence, Mode: string(sensitivemigration.ModeRehearseRestore), Resource: sensitiveMigrationEscrowResource,
			TenantScopeHash: migrationHash("rehearsal-scope", runID), Rows: count, Plaintext: count, EscrowSetHash: escrowHash,
			Revision: run.ExpectedRevision, PriorEventHash: journal.Head, CreatedAt: now,
		}
		event.EventHash = migrationEventHash(event)
		if err := tx.Create(&event).Error; err != nil {
			return ErrMigrationStore
		}
		updated := tx.Model(&gormSensitiveMigrationRun{}).
			Where("run_id = ? AND restore_rehearsed = ?", runID, false).
			Updates(map[string]any{"restore_rehearsed": true, "restore_rehearsed_at": now})
		if updated.Error != nil || updated.RowsAffected != 1 {
			return ErrMigrationConflict
		}
		verified, err := verifyMigrationJournal(tx, runID)
		if err != nil || verified.Head != event.EventHash || len(verified.RehearsalEvents) != 1 || verified.RehearsalEvents[0].EscrowSetHash != escrowHash {
			return ErrMigrationConflict
		}
		commit = sensitivemigration.BatchCommit{Revision: run.ExpectedRevision, Rows: count, EventSequence: sequence, EventHash: event.EventHash}
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrMigrationInvalidOptions) || errors.Is(err, ErrMigrationConflict) {
			return sensitivemigration.BatchCommit{}, err
		}
		return sensitivemigration.BatchCommit{}, ErrMigrationStore
	}
	return commit, nil
}

func (s *GORMProtectedValueMigrationStore) RollbackScopes(ctx context.Context, runID string, plan sensitivemigration.ResourcePlan, after string, limit int) ([]string, error) {
	if s == nil || s.db == nil || ctx == nil || ctx.Err() != nil || !canonicalMigrationRunID(runID) || limit < 1 || limit > sensitivemigration.MaximumBatchSize {
		return nil, ErrMigrationInvalidOptions
	}
	if _, _, err := migrationLayout(plan); err != nil {
		return nil, err
	}
	var scopeHashes []string
	query := s.db.WithContext(ctx).Model(&gormSensitiveMigrationEscrow{}).
		Where(sensitiveMigrationEscrowTable+".run_id = ? AND "+sensitiveMigrationEscrowTable+".resource = ?", runID, plan.Resource)
	if after != "" {
		query = query.Where("tenant_scope_hash > ?", sensitivemigration.TenantCursor(after))
	}
	if err := query.Distinct("tenant_scope_hash").Order("tenant_scope_hash").Limit(limit).Pluck("tenant_scope_hash", &scopeHashes).Error; err != nil {
		return nil, ErrMigrationStore
	}
	scopes := make([]string, 0, len(scopeHashes))
	for _, scopeHash := range scopeHashes {
		var matchingScopes []string
		if !canonicalMigrationCursor(scopeHash) || s.db.WithContext(ctx).Model(&gormSensitiveMigrationTarget{}).
			Where("run_id = ? AND resource = ? AND tenant_scope_hash = ?", runID, plan.Resource, scopeHash).
			Limit(1).Pluck("tenant_scope", &matchingScopes).Error != nil || len(matchingScopes) != 1 ||
			strings.TrimSpace(matchingScopes[0]) == "" || sensitivemigration.TenantCursor(matchingScopes[0]) != scopeHash {
			return nil, ErrMigrationConflict
		}
		scopes = append(scopes, matchingScopes[0])
	}
	return scopes, nil
}

func (s *GORMProtectedValueMigrationStore) RollbackRows(ctx context.Context, runID string, plan sensitivemigration.ResourcePlan, tenant string, after string, limit int) ([]sensitivemigration.RollbackRow, error) {
	if ctx == nil || ctx.Err() != nil || strings.TrimSpace(tenant) == "" || limit < 1 || limit > sensitivemigration.MaximumBatchSize {
		return nil, ErrMigrationInvalidOptions
	}
	layout, generic, err := migrationLayout(plan)
	if err != nil {
		return nil, err
	}
	if !canonicalMigrationRunID(runID) {
		return nil, ErrMigrationInvalidOptions
	}
	tenantHash := sensitivemigration.TenantCursor(tenant)
	var recordHashes []string
	query := s.db.WithContext(ctx).Model(&gormSensitiveMigrationEscrow{}).
		Where("run_id = ? AND resource = ? AND tenant_scope_hash = ?", runID, plan.Resource, tenantHash)
	if after != "" {
		query = query.Where("record_id_hash > ?", sensitivemigration.RecordCursor(after))
	}
	if err := query.Distinct("record_id_hash").Order("record_id_hash").Limit(limit).Pluck("record_id_hash", &recordHashes).Error; err != nil {
		return nil, ErrMigrationStore
	}
	rows := make([]sensitivemigration.RollbackRow, 0, len(recordHashes))
	for _, recordHash := range recordHashes {
		entries, err := loadMigrationEscrowRecord(s.db.WithContext(ctx), runID, plan, tenant, recordHash)
		if err != nil || len(entries) == 0 {
			return nil, ErrMigrationConflict
		}
		recordID := entries[0].RecordID
		var physical migrationPhysicalRow
		query := s.db.WithContext(ctx).Table(layout.Table).Select("id, values_json").Where("id = ?", recordID)
		if generic {
			query = query.Where("resource = ?", plan.Resource)
		}
		if err := query.Take(&physical).Error; err != nil || physical.ID != recordID || physical.ValuesJSON == "" {
			return nil, ErrMigrationConflict
		}
		rows = append(rows, sensitivemigration.RollbackRow{
			Row:    sensitivemigration.Row{Resource: plan.Resource, TenantID: tenant, RecordID: recordID, ValuesJSON: physical.ValuesJSON},
			Escrow: entries,
		})
	}
	return rows, nil
}

type migrationEscrowJoinedRow struct {
	EscrowID          string `gorm:"column:escrow_id"`
	RunID             string `gorm:"column:run_id"`
	Resource          string `gorm:"column:resource"`
	TenantScopeHash   string `gorm:"column:tenant_scope_hash"`
	TenantScope       string `gorm:"column:tenant_scope"`
	RecordID          string `gorm:"column:record_id"`
	RecordIDHash      string `gorm:"column:record_id_hash"`
	FieldKey          string `gorm:"column:field_key"`
	ProtectedOriginal string `gorm:"column:protected_original"`
	MigratedValueHash string `gorm:"column:migrated_value_hash"`
	TargetID          string `gorm:"column:target_id"`
}

func migrationEscrowTargetJoin() string {
	return sensitiveMigrationTargetsTable + ".target_id = " + sensitiveMigrationEscrowTable + ".target_id"
}

func migrationEscrowJoinedQuery(tx *gorm.DB) *gorm.DB {
	return tx.Model(&gormSensitiveMigrationEscrow{}).
		Select(strings.Join([]string{
			sensitiveMigrationEscrowTable + ".escrow_id", sensitiveMigrationEscrowTable + ".run_id",
			sensitiveMigrationEscrowTable + ".resource", sensitiveMigrationEscrowTable + ".tenant_scope_hash",
			sensitiveMigrationTargetsTable + ".tenant_scope", sensitiveMigrationEscrowTable + ".record_id", sensitiveMigrationEscrowTable + ".record_id_hash",
			sensitiveMigrationEscrowTable + ".field_key", sensitiveMigrationEscrowTable + ".protected_original",
			sensitiveMigrationEscrowTable + ".migrated_value_hash", sensitiveMigrationTargetsTable + ".target_id",
		}, ", ")).
		Joins("JOIN " + sensitiveMigrationTargetsTable + " ON " + migrationEscrowTargetJoin())
}

func loadMigrationEscrowPage(tx *gorm.DB, runID string, after sensitivemigration.EscrowCursor, limit int) ([]sensitivemigration.EscrowEntry, error) {
	if tx == nil || !canonicalMigrationRunID(runID) || limit < 1 || limit > sensitivemigration.MaximumBatchSize ||
		after.RunID != "" && (after.RunID != runID || after.TenantID == "" || after.Resource == "" || after.RecordID == "" || after.FieldKey == "") {
		return nil, ErrMigrationInvalidOptions
	}
	query := migrationEscrowJoinedQuery(tx).Where(sensitiveMigrationEscrowTable+".run_id = ?", runID)
	if after.RunID != "" {
		query = query.Where(sensitiveMigrationEscrowTable+".escrow_id > ?", sensitivemigration.EscrowCursorKey(after))
	}
	var rows []migrationEscrowJoinedRow
	if err := query.Order(sensitiveMigrationEscrowTable + ".escrow_id").Limit(limit).Find(&rows).Error; err != nil {
		return nil, ErrMigrationStore
	}
	return migrationEscrowEntriesFromRows(runID, rows)
}

func loadMigrationEscrowRecord(tx *gorm.DB, runID string, plan sensitivemigration.ResourcePlan, tenant string, recordHash string) ([]sensitivemigration.EscrowEntry, error) {
	var rows []migrationEscrowJoinedRow
	limit := len(plan.Fields) + 1
	if err := migrationEscrowJoinedQuery(tx).
		Where(sensitiveMigrationEscrowTable+".run_id = ? AND "+sensitiveMigrationEscrowTable+".resource = ? AND "+sensitiveMigrationEscrowTable+".tenant_scope_hash = ? AND "+sensitiveMigrationEscrowTable+".record_id_hash = ?", runID, plan.Resource, sensitivemigration.TenantCursor(tenant), recordHash).
		Order(sensitiveMigrationEscrowTable + ".escrow_id").Limit(limit).Find(&rows).Error; err != nil {
		return nil, ErrMigrationStore
	}
	if len(rows) > len(plan.Fields) {
		return nil, ErrMigrationConflict
	}
	entries, err := migrationEscrowEntriesFromRows(runID, rows)
	if err != nil {
		return nil, err
	}
	fields := make(map[string]struct{}, len(plan.Fields))
	for _, field := range plan.Fields {
		fields[field.Key] = struct{}{}
	}
	for _, entry := range entries {
		if entry.TenantID != tenant || sensitivemigration.RecordCursor(entry.RecordID) != recordHash {
			return nil, ErrMigrationConflict
		}
		if _, ok := fields[entry.FieldKey]; !ok {
			return nil, ErrMigrationConflict
		}
	}
	return entries, nil
}

func migrationEscrowEntriesFromRows(runID string, rows []migrationEscrowJoinedRow) ([]sensitivemigration.EscrowEntry, error) {
	entries := make([]sensitivemigration.EscrowEntry, 0, len(rows))
	for _, row := range rows {
		entryCursor := sensitivemigration.EscrowCursor{RunID: runID, TenantID: row.TenantScope, Resource: row.Resource, RecordID: row.RecordID, FieldKey: row.FieldKey}
		if row.RunID != runID || row.TenantScopeHash != sensitivemigration.TenantCursor(row.TenantScope) || row.RecordIDHash != sensitivemigration.RecordCursor(row.RecordID) ||
			row.EscrowID != sensitivemigration.EscrowCursorKey(entryCursor) ||
			row.TargetID != migrationSurrogateID("target", runID, row.Resource, row.TenantScopeHash, row.RecordID, row.FieldKey) ||
			!dataprotection.IsEnvelope(row.ProtectedOriginal) || !canonicalMigrationHash(row.MigratedValueHash) {
			return nil, ErrMigrationConflict
		}
		entries = append(entries, sensitivemigration.EscrowEntry{
			RunID: runID, Resource: row.Resource, RecordID: row.RecordID, FieldKey: row.FieldKey, TenantID: row.TenantScope,
			ProtectedOriginal: row.ProtectedOriginal, MigratedValueHash: row.MigratedValueHash,
		})
	}
	hasher := sensitivemigration.NewEscrowSetHasher()
	if err := hasher.Add(entries...); err != nil {
		return nil, ErrMigrationConflict
	}
	return entries, nil
}

func migrationEscrowSetSummary(tx *gorm.DB, runID string) (string, int, error) {
	hasher := sensitivemigration.NewEscrowSetHasher()
	cursor := sensitivemigration.EscrowCursor{}
	for {
		entries, err := loadMigrationEscrowPage(tx, runID, cursor, sensitivemigration.MaximumBatchSize)
		if err != nil {
			return "", 0, err
		}
		if len(entries) == 0 {
			break
		}
		if err := hasher.Add(entries...); err != nil {
			return "", 0, ErrMigrationConflict
		}
		cursor = sensitivemigration.EscrowCursorFromEntry(entries[len(entries)-1])
	}
	value, count, err := hasher.Sum()
	if err != nil {
		return "", 0, ErrMigrationConflict
	}
	return value, count, nil
}

func (s *GORMProtectedValueMigrationStore) lockedRevision(tx *gorm.DB) (uint64, error) {
	query := tx.Where("key = ?", "revision")
	if s.driver != "sqlite" {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var state gormAdminResourceState
	if err := query.First(&state).Error; err != nil {
		return 0, ErrMigrationConflict
	}
	revision, err := strconv.ParseUint(state.Value, 10, 64)
	if err != nil {
		return 0, ErrMigrationConflict
	}
	return revision, nil
}

func (s *GORMProtectedValueMigrationStore) ApplyBatch(ctx context.Context, mutation sensitivemigration.BatchMutation) (sensitivemigration.BatchCommit, error) {
	if s == nil || s.db == nil || ctx == nil || ctx.Err() != nil || !canonicalMigrationRunID(mutation.RunID) || mutation.Mode != sensitivemigration.ModeApply || len(mutation.Rows) == 0 || mutation.ExpectedRevision == ^uint64(0) || migrationCountsTotal(mutation.Counts) == 0 {
		return sensitivemigration.BatchCommit{}, ErrMigrationInvalidOptions
	}
	layout, generic, err := migrationLayout(mutation.Resource)
	if err != nil {
		return sensitivemigration.BatchCommit{}, err
	}
	tenant := strings.TrimSpace(mutation.TenantID)
	if tenant == "" || strings.TrimSpace(mutation.LastRecordID) == "" {
		return sensitivemigration.BatchCommit{}, ErrMigrationInvalidOptions
	}
	previousRecordID := ""
	for _, row := range mutation.Rows {
		if strings.TrimSpace(row.RecordID) == "" || row.OriginalValuesJSON == "" || row.UpdatedValuesJSON == "" {
			return sensitivemigration.BatchCommit{}, ErrMigrationInvalidOptions
		}
		if previousRecordID != "" && sensitivemigration.RecordCursor(row.RecordID) <= sensitivemigration.RecordCursor(previousRecordID) {
			return sensitivemigration.BatchCommit{}, ErrMigrationConflict
		}
		previousRecordID = row.RecordID
	}
	if previousRecordID != mutation.LastRecordID {
		return sensitivemigration.BatchCommit{}, ErrMigrationConflict
	}
	tenantHash := sensitivemigration.TenantCursor(tenant)
	checkpointID := migrationSurrogateID("checkpoint", mutation.RunID, mutation.Resource.Resource, tenantHash, string(mutation.Mode))
	nextRevision := mutation.ExpectedRevision + 1
	commit := sensitivemigration.BatchCommit{}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var run gormSensitiveMigrationRun
		runQuery := tx.Where("run_id = ? AND status = ?", mutation.RunID, sensitivemigration.StatusPrepared)
		if s.driver != "sqlite" {
			runQuery = runQuery.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := runQuery.First(&run).Error; err != nil || run.ExpectedRevision != mutation.ExpectedRevision {
			return ErrMigrationConflict
		}
		if err := validateMigrationTargetSet(tx, run); err != nil {
			return err
		}
		actual, err := s.lockedRevision(tx)
		if err != nil || actual != mutation.ExpectedRevision {
			return ErrMigrationConflict
		}
		journal, err := verifyMigrationJournal(tx, mutation.RunID)
		if err != nil {
			return err
		}

		var checkpoint gormSensitiveMigrationCheckpoint
		checkpointQuery := tx.Where("checkpoint_id = ?", checkpointID)
		if s.driver != "sqlite" {
			checkpointQuery = checkpointQuery.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		checkpointErr := checkpointQuery.First(&checkpoint).Error
		checkpointFound := checkpointErr == nil
		if checkpointFound {
			if checkpoint.ExpectedRevision > mutation.ExpectedRevision || sensitivemigration.RecordCursor(mutation.Rows[0].RecordID) <= sensitivemigration.RecordCursor(checkpoint.LastRecordID) {
				return ErrMigrationConflict
			}
			var checkpointEvent gormSensitiveMigrationEvent
			if err := tx.Where("run_id = ? AND sequence = ?", mutation.RunID, checkpoint.EventSequence).First(&checkpointEvent).Error; err != nil {
				return ErrMigrationConflict
			}
			if checkpointEvent.Revision != checkpoint.ExpectedRevision ||
				checkpointEvent.Mode != string(mutation.Mode) || checkpointEvent.Resource != mutation.Resource.Resource ||
				checkpointEvent.TenantScopeHash != tenantHash || checkpointEvent.EventHash != checkpoint.EventHash {
				return ErrMigrationConflict
			}
		} else if !errors.Is(checkpointErr, gorm.ErrRecordNotFound) {
			return ErrMigrationStore
		}

		processedTargets := 0
		for _, row := range mutation.Rows {
			recordHash := sensitivemigration.RecordCursor(row.RecordID)
			var targets []gormSensitiveMigrationTarget
			if err := tx.Model(&gormSensitiveMigrationTarget{}).
				Where("run_id = ? AND resource = ? AND tenant_scope_hash = ? AND record_id_hash = ?", mutation.RunID, mutation.Resource.Resource, tenantHash, recordHash).
				Order("target_id").Limit(len(mutation.Resource.Fields) + 1).Find(&targets).Error; err != nil || !targetsMatchPlan(targets, mutation.Resource) {
				return ErrMigrationConflict
			}
			targetFields := make([]string, 0, len(targets))
			targetIDs := make(map[string]string, len(targets))
			snapshotHash := migrationHash("values-json", row.OriginalValuesJSON)
			for _, target := range targets {
				if target.TenantScope != tenant || target.RecordID != row.RecordID || target.RecordIDHash != recordHash ||
					target.SnapshotHash != snapshotHash || target.ResourcePlanHash != resourcePlanHash(mutation.Resource) {
					return ErrMigrationConflict
				}
				targetFields = append(targetFields, target.FieldKey)
				targetIDs[target.FieldKey] = target.TargetID
			}
			processedTargets += len(targets)
			changedFields, err := migrationChangedFields(row.OriginalValuesJSON, row.UpdatedValuesJSON)
			if err != nil || !migrationFieldsAreTargets(changedFields, targetFields) {
				return ErrMigrationConflict
			}
			escrowFields := make([]string, 0, len(row.Escrow))
			seenEscrowFields := map[string]struct{}{}
			for _, entry := range row.Escrow {
				if entry.RunID != mutation.RunID || entry.Resource != mutation.Resource.Resource || entry.RecordID != row.RecordID ||
					entry.TenantID != tenant || strings.TrimSpace(entry.FieldKey) == "" || !dataprotection.IsEnvelope(entry.ProtectedOriginal) ||
					!canonicalMigrationHash(entry.MigratedValueHash) {
					return ErrMigrationConflict
				}
				if _, duplicate := seenEscrowFields[entry.FieldKey]; duplicate {
					return ErrMigrationConflict
				}
				migrated, err := migrationStringField(row.UpdatedValuesJSON, entry.FieldKey)
				if err != nil || sensitivemigration.HashMigratedValue(migrated) != entry.MigratedValueHash {
					return ErrMigrationConflict
				}
				seenEscrowFields[entry.FieldKey] = struct{}{}
				escrowFields = append(escrowFields, entry.FieldKey)
			}
			slices.Sort(escrowFields)
			if !slices.Equal(changedFields, escrowFields) {
				return ErrMigrationConflict
			}
			if len(changedFields) == 0 {
				continue
			}
			for _, entry := range row.Escrow {
				cursor := sensitivemigration.EscrowCursor{RunID: mutation.RunID, TenantID: tenant, Resource: mutation.Resource.Resource, RecordID: row.RecordID, FieldKey: entry.FieldKey}
				escrow := gormSensitiveMigrationEscrow{
					EscrowID: sensitivemigration.EscrowCursorKey(cursor), TargetID: targetIDs[entry.FieldKey],
					RunID: mutation.RunID, Resource: mutation.Resource.Resource, TenantScopeHash: tenantHash,
					RecordID: row.RecordID, RecordIDHash: recordHash, FieldKey: entry.FieldKey, ProtectedOriginal: entry.ProtectedOriginal,
					MigratedValueHash: entry.MigratedValueHash,
				}
				if err := tx.Create(&escrow).Error; err != nil {
					return ErrMigrationStore
				}
			}
			valuesPredicate, err := migrationValuesJSONExactPredicate(s.driver)
			if err != nil {
				return err
			}
			query := tx.Table(layout.Table).Where("id = ?", row.RecordID).Where(valuesPredicate, row.OriginalValuesJSON)
			if generic {
				query = query.Where("resource = ?", mutation.Resource.Resource)
			}
			updated := query.Update("values_json", row.UpdatedValuesJSON)
			if updated.Error != nil {
				return ErrMigrationStore
			}
			if updated.RowsAffected != 1 {
				return ErrMigrationConflict
			}
		}
		if processedTargets != migrationCountsTotal(mutation.Counts) {
			return ErrMigrationConflict
		}

		priorHash := journal.Head
		sequence := journal.Sequence + 1
		now := migrationTimestamp()
		event := gormSensitiveMigrationEvent{
			EventID: migrationSurrogateID("event", mutation.RunID, strconv.FormatUint(sequence, 10)),
			RunID:   mutation.RunID, Sequence: sequence, Mode: string(mutation.Mode), Resource: mutation.Resource.Resource,
			TenantScopeHash: tenantHash, LastRecordID: mutation.LastRecordID,
			Rows: len(mutation.Rows), Missing: mutation.Counts.Missing,
			Plaintext: mutation.Counts.Plaintext, TargetEnvelope: mutation.Counts.TargetEnvelope,
			ForeignEnvelope: mutation.Counts.ForeignEnvelope, Malformed: mutation.Counts.MalformedEnvelope,
			Revision: nextRevision, PriorEventHash: priorHash, CreatedAt: now,
		}
		event.EventHash = migrationEventHash(event)
		if err := tx.Create(&event).Error; err != nil {
			return ErrMigrationStore
		}
		cumulativeRows := len(mutation.Rows)
		if checkpointFound {
			cumulativeRows += checkpoint.Rows
			cumulativeCounts := addMigrationCounts(checkpointCounts(checkpoint), mutation.Counts)
			updated := tx.Model(&gormSensitiveMigrationCheckpoint{}).
				Where("checkpoint_id = ? AND expected_revision = ? AND last_record_id = ? AND event_sequence = ?", checkpointID, checkpoint.ExpectedRevision, checkpoint.LastRecordID, checkpoint.EventSequence).
				Updates(map[string]any{
					"tenant_scope": tenant, "last_record_id": mutation.LastRecordID, "expected_revision": nextRevision,
					"row_count": cumulativeRows, "status": sensitivemigration.StatusCompleted,
					"missing_count": cumulativeCounts.Missing, "plaintext_count": cumulativeCounts.Plaintext,
					"target_envelope_count": cumulativeCounts.TargetEnvelope, "foreign_envelope_count": cumulativeCounts.ForeignEnvelope,
					"malformed_envelope_count": cumulativeCounts.MalformedEnvelope, "batch_count": checkpoint.Batches + 1,
					"event_sequence": sequence, "event_hash": event.EventHash, "updated_at": now,
				})
			if updated.Error != nil || updated.RowsAffected != 1 {
				return ErrMigrationConflict
			}
		} else {
			checkpoint = gormSensitiveMigrationCheckpoint{
				CheckpointID: checkpointID, RunID: mutation.RunID, Resource: mutation.Resource.Resource, TenantScope: tenant,
				TenantScopeHash: tenantHash, Mode: string(mutation.Mode), LastRecordID: mutation.LastRecordID,
				ExpectedRevision: nextRevision, Rows: cumulativeRows, Missing: mutation.Counts.Missing,
				Plaintext: mutation.Counts.Plaintext, TargetEnvelope: mutation.Counts.TargetEnvelope,
				ForeignEnvelope: mutation.Counts.ForeignEnvelope, Malformed: mutation.Counts.MalformedEnvelope,
				Batches: 1, Status: sensitivemigration.StatusCompleted,
				EventSequence: sequence, EventHash: event.EventHash, UpdatedAt: now,
			}
			if err := tx.Create(&checkpoint).Error; err != nil {
				return ErrMigrationStore
			}
		}
		verifiedJournal, err := verifyMigrationJournal(tx, mutation.RunID)
		if err != nil || verifiedJournal.Head != event.EventHash || verifiedJournal.Sequence != sequence {
			return ErrMigrationConflict
		}
		cas := tx.Model(&gormAdminResourceState{}).
			Where("key = ? AND value = ?", "revision", strconv.FormatUint(mutation.ExpectedRevision, 10)).
			Update("value", strconv.FormatUint(nextRevision, 10))
		if cas.Error != nil {
			return ErrMigrationStore
		}
		if cas.RowsAffected != 1 {
			return ErrMigrationConflict
		}
		runCAS := tx.Model(&gormSensitiveMigrationRun{}).
			Where("run_id = ? AND expected_revision = ?", mutation.RunID, mutation.ExpectedRevision).
			Update("expected_revision", nextRevision)
		if runCAS.Error != nil || runCAS.RowsAffected != 1 {
			return ErrMigrationConflict
		}
		commit = sensitivemigration.BatchCommit{Revision: nextRevision, Rows: len(mutation.Rows), LastRecordID: mutation.LastRecordID, EventSequence: sequence, EventHash: event.EventHash}
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrMigrationInvalidOptions) || errors.Is(err, ErrMigrationConflict) || errors.Is(err, ErrMigrationPhysicalLayout) {
			return sensitivemigration.BatchCommit{}, err
		}
		return sensitivemigration.BatchCommit{}, ErrMigrationStore
	}
	return commit, nil
}

func (s *GORMProtectedValueMigrationStore) RollbackBatch(ctx context.Context, mutation sensitivemigration.BatchMutation) (sensitivemigration.BatchCommit, error) {
	if s == nil || s.db == nil || ctx == nil || ctx.Err() != nil || !canonicalMigrationRunID(mutation.RunID) || mutation.Mode != sensitivemigration.ModeRollback ||
		len(mutation.Rows) == 0 || mutation.ExpectedRevision == ^uint64(0) || mutation.Counts.Plaintext < 1 || migrationCountsTotal(mutation.Counts) != mutation.Counts.Plaintext {
		return sensitivemigration.BatchCommit{}, ErrMigrationInvalidOptions
	}
	layout, generic, err := migrationLayout(mutation.Resource)
	if err != nil {
		return sensitivemigration.BatchCommit{}, err
	}
	tenant := strings.TrimSpace(mutation.TenantID)
	if tenant == "" || strings.TrimSpace(mutation.LastRecordID) == "" {
		return sensitivemigration.BatchCommit{}, ErrMigrationInvalidOptions
	}
	previousRecordID := ""
	for _, row := range mutation.Rows {
		if row.RecordID == "" || row.OriginalValuesJSON == "" || row.UpdatedValuesJSON == "" || len(row.Escrow) == 0 ||
			previousRecordID != "" && sensitivemigration.RecordCursor(row.RecordID) <= sensitivemigration.RecordCursor(previousRecordID) {
			return sensitivemigration.BatchCommit{}, ErrMigrationInvalidOptions
		}
		previousRecordID = row.RecordID
	}
	if previousRecordID != mutation.LastRecordID {
		return sensitivemigration.BatchCommit{}, ErrMigrationConflict
	}
	tenantHash := sensitivemigration.TenantCursor(tenant)
	checkpointID := migrationSurrogateID("checkpoint", mutation.RunID, mutation.Resource.Resource, tenantHash, string(mutation.Mode))
	nextRevision := mutation.ExpectedRevision + 1
	commit := sensitivemigration.BatchCommit{}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var run gormSensitiveMigrationRun
		runQuery := tx.Where("run_id = ? AND status = ? AND restore_rehearsed = ? AND rollback_status = ?", mutation.RunID, sensitivemigration.StatusCompleted, true, sensitivemigration.StatusNone)
		if s.driver != "sqlite" {
			runQuery = runQuery.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := runQuery.First(&run).Error; err != nil || run.ExpectedRevision != mutation.ExpectedRevision {
			return ErrMigrationConflict
		}
		if err := validateMigrationTargetSet(tx, run); err != nil {
			return err
		}
		actual, err := s.lockedRevision(tx)
		if err != nil || actual != mutation.ExpectedRevision {
			return ErrMigrationConflict
		}
		journal, err := verifyMigrationJournal(tx, mutation.RunID)
		if err != nil {
			return err
		}
		liveEscrowHash, liveEscrowCount, err := migrationEscrowSetSummary(tx, mutation.RunID)
		if err != nil {
			return err
		}
		if liveEscrowCount != journal.ApplyCounts.Plaintext || len(journal.RehearsalEvents) != 1 ||
			journal.RehearsalEvents[0].Rows != liveEscrowCount || journal.RehearsalEvents[0].EscrowSetHash != liveEscrowHash {
			return ErrMigrationConflict
		}

		var checkpoint gormSensitiveMigrationCheckpoint
		checkpointQuery := tx.Where("checkpoint_id = ?", checkpointID)
		if s.driver != "sqlite" {
			checkpointQuery = checkpointQuery.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		checkpointErr := checkpointQuery.First(&checkpoint).Error
		checkpointFound := checkpointErr == nil
		if checkpointFound {
			if checkpoint.ExpectedRevision > mutation.ExpectedRevision || sensitivemigration.RecordCursor(mutation.Rows[0].RecordID) <= sensitivemigration.RecordCursor(checkpoint.LastRecordID) {
				return ErrMigrationConflict
			}
			var checkpointEvent gormSensitiveMigrationEvent
			if err := tx.Where("run_id = ? AND sequence = ?", mutation.RunID, checkpoint.EventSequence).First(&checkpointEvent).Error; err != nil ||
				checkpointEvent.Revision != checkpoint.ExpectedRevision || checkpointEvent.Mode != string(mutation.Mode) ||
				checkpointEvent.Resource != mutation.Resource.Resource || checkpointEvent.TenantScopeHash != tenantHash || checkpointEvent.EventHash != checkpoint.EventHash {
				return ErrMigrationConflict
			}
		} else if !errors.Is(checkpointErr, gorm.ErrRecordNotFound) {
			return ErrMigrationStore
		}

		processed := 0
		for _, row := range mutation.Rows {
			persistedEntries, err := loadMigrationEscrowRecord(tx, mutation.RunID, mutation.Resource, tenant, sensitivemigration.RecordCursor(row.RecordID))
			if err != nil || len(persistedEntries) == 0 {
				return ErrMigrationConflict
			}
			escrowByCoordinate := make(map[string]sensitivemigration.EscrowEntry, len(persistedEntries))
			for _, entry := range persistedEntries {
				escrowByCoordinate[migrationSurrogateID("escrow-coordinate", entry.Resource, sensitivemigration.TenantCursor(entry.TenantID), entry.RecordID, entry.FieldKey)] = entry
			}
			changedFields, err := migrationChangedFields(row.OriginalValuesJSON, row.UpdatedValuesJSON)
			if err != nil {
				return ErrMigrationConflict
			}
			escrowFields := make([]string, 0, len(row.Escrow))
			seen := map[string]struct{}{}
			for _, supplied := range row.Escrow {
				if supplied.RunID != mutation.RunID || supplied.Resource != mutation.Resource.Resource || supplied.RecordID != row.RecordID || supplied.TenantID != tenant {
					return ErrMigrationConflict
				}
				coordinate := migrationSurrogateID("escrow-coordinate", supplied.Resource, tenantHash, supplied.RecordID, supplied.FieldKey)
				persisted, ok := escrowByCoordinate[coordinate]
				if !ok || persisted != supplied {
					return ErrMigrationConflict
				}
				if _, duplicate := seen[supplied.FieldKey]; duplicate {
					return ErrMigrationConflict
				}
				migrated, err := migrationStringField(row.OriginalValuesJSON, supplied.FieldKey)
				if err != nil || sensitivemigration.HashMigratedValue(migrated) != persisted.MigratedValueHash {
					return ErrMigrationConflict
				}
				if _, err := migrationStringField(row.UpdatedValuesJSON, supplied.FieldKey); err != nil {
					return ErrMigrationConflict
				}
				seen[supplied.FieldKey] = struct{}{}
				escrowFields = append(escrowFields, supplied.FieldKey)
			}
			slices.Sort(escrowFields)
			if !slices.Equal(changedFields, escrowFields) {
				return ErrMigrationConflict
			}
			valuesPredicate, err := migrationValuesJSONExactPredicate(s.driver)
			if err != nil {
				return err
			}
			query := tx.Table(layout.Table).Where("id = ?", row.RecordID).Where(valuesPredicate, row.OriginalValuesJSON)
			if generic {
				query = query.Where("resource = ?", mutation.Resource.Resource)
			}
			updated := query.Update("values_json", row.UpdatedValuesJSON)
			if updated.Error != nil {
				return ErrMigrationStore
			}
			if updated.RowsAffected != 1 {
				return ErrMigrationConflict
			}
			processed += len(row.Escrow)
		}
		if processed != mutation.Counts.Plaintext {
			return ErrMigrationConflict
		}

		sequence := journal.Sequence + 1
		now := migrationTimestamp()
		event := gormSensitiveMigrationEvent{
			EventID: migrationSurrogateID("event", mutation.RunID, strconv.FormatUint(sequence, 10)), RunID: mutation.RunID,
			Sequence: sequence, Mode: string(mutation.Mode), Resource: mutation.Resource.Resource, TenantScopeHash: tenantHash,
			LastRecordID: mutation.LastRecordID, Rows: len(mutation.Rows), Plaintext: mutation.Counts.Plaintext,
			Revision: nextRevision, PriorEventHash: journal.Head, CreatedAt: now,
		}
		event.EventHash = migrationEventHash(event)
		if err := tx.Create(&event).Error; err != nil {
			return ErrMigrationStore
		}
		cumulativeRows := len(mutation.Rows)
		if checkpointFound {
			cumulativeRows += checkpoint.Rows
			cumulative := addMigrationCounts(checkpointCounts(checkpoint), mutation.Counts)
			updated := tx.Model(&gormSensitiveMigrationCheckpoint{}).
				Where("checkpoint_id = ? AND expected_revision = ? AND last_record_id = ? AND event_sequence = ?", checkpointID, checkpoint.ExpectedRevision, checkpoint.LastRecordID, checkpoint.EventSequence).
				Updates(map[string]any{
					"tenant_scope": tenant, "last_record_id": mutation.LastRecordID, "expected_revision": nextRevision, "row_count": cumulativeRows,
					"status": sensitivemigration.StatusCompleted, "missing_count": cumulative.Missing, "plaintext_count": cumulative.Plaintext,
					"target_envelope_count": cumulative.TargetEnvelope, "foreign_envelope_count": cumulative.ForeignEnvelope,
					"malformed_envelope_count": cumulative.MalformedEnvelope, "batch_count": checkpoint.Batches + 1,
					"event_sequence": sequence, "event_hash": event.EventHash, "updated_at": now,
				})
			if updated.Error != nil || updated.RowsAffected != 1 {
				return ErrMigrationConflict
			}
		} else {
			checkpoint = gormSensitiveMigrationCheckpoint{
				CheckpointID: checkpointID, RunID: mutation.RunID, Resource: mutation.Resource.Resource, TenantScope: tenant,
				TenantScopeHash: tenantHash, Mode: string(mutation.Mode), LastRecordID: mutation.LastRecordID,
				ExpectedRevision: nextRevision, Rows: cumulativeRows, Plaintext: mutation.Counts.Plaintext, Batches: 1,
				Status: sensitivemigration.StatusCompleted, EventSequence: sequence, EventHash: event.EventHash, UpdatedAt: now,
			}
			if err := tx.Create(&checkpoint).Error; err != nil {
				return ErrMigrationStore
			}
		}
		verified, err := verifyMigrationJournal(tx, mutation.RunID)
		if err != nil || verified.Head != event.EventHash || verified.Sequence != sequence {
			return ErrMigrationConflict
		}
		cas := tx.Model(&gormAdminResourceState{}).Where("key = ? AND value = ?", "revision", strconv.FormatUint(mutation.ExpectedRevision, 10)).Update("value", strconv.FormatUint(nextRevision, 10))
		if cas.Error != nil {
			return ErrMigrationStore
		}
		if cas.RowsAffected != 1 {
			return ErrMigrationConflict
		}
		runCAS := tx.Model(&gormSensitiveMigrationRun{}).
			Where("run_id = ? AND expected_revision = ? AND rollback_status = ?", mutation.RunID, mutation.ExpectedRevision, sensitivemigration.StatusNone).
			Update("expected_revision", nextRevision)
		if runCAS.Error != nil || runCAS.RowsAffected != 1 {
			return ErrMigrationConflict
		}
		commit = sensitivemigration.BatchCommit{Revision: nextRevision, Rows: len(mutation.Rows), LastRecordID: mutation.LastRecordID, EventSequence: sequence, EventHash: event.EventHash}
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrMigrationInvalidOptions) || errors.Is(err, ErrMigrationConflict) || errors.Is(err, ErrMigrationPhysicalLayout) {
			return sensitivemigration.BatchCommit{}, err
		}
		return sensitivemigration.BatchCommit{}, ErrMigrationStore
	}
	return commit, nil
}

func (s *GORMProtectedValueMigrationStore) FinishRollback(ctx context.Context, runID string) error {
	if s == nil || s.db == nil || ctx == nil || ctx.Err() != nil || !canonicalMigrationRunID(runID) {
		return ErrMigrationInvalidOptions
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var run gormSensitiveMigrationRun
		query := tx.Where("run_id = ? AND status = ? AND restore_rehearsed = ?", runID, sensitivemigration.StatusCompleted, true)
		if s.driver != "sqlite" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := query.First(&run).Error; err != nil || run.RollbackStatus != sensitivemigration.StatusNone && run.RollbackStatus != sensitivemigration.StatusCompleted {
			return ErrMigrationConflict
		}
		if err := validateMigrationTargetSet(tx, run); err != nil {
			return err
		}
		journal, err := verifyMigrationJournal(tx, runID)
		if err != nil {
			return err
		}
		escrowHash, escrowCount, err := migrationEscrowSetSummary(tx, runID)
		if err != nil {
			return err
		}
		if escrowCount != journal.ApplyCounts.Plaintext || len(journal.RehearsalEvents) != 1 ||
			journal.RehearsalEvents[0].Rows != escrowCount || journal.RehearsalEvents[0].EscrowSetHash != escrowHash {
			return ErrMigrationConflict
		}
		if journal.RollbackCounts.Plaintext < 0 || migrationCountsTotal(journal.RollbackCounts) != journal.RollbackCounts.Plaintext ||
			journal.RollbackCounts.Plaintext != escrowCount {
			return ErrMigrationConflict
		}
		if run.RollbackStatus == sensitivemigration.StatusCompleted {
			return nil
		}
		updated := tx.Model(&gormSensitiveMigrationRun{}).
			Where("run_id = ? AND rollback_status = ?", runID, sensitivemigration.StatusNone).
			Update("rollback_status", sensitivemigration.StatusCompleted)
		if updated.Error != nil || updated.RowsAffected != 1 {
			return ErrMigrationConflict
		}
		return nil
	})
}

type migrationPhysicalRow struct {
	ID         string `gorm:"column:id"`
	ValuesJSON string `gorm:"column:values_json"`
}

func readMigrationPhysicalRows(db *gorm.DB, layout gormAdminResourceLayout, generic bool, resource string, after string, limit int) ([]migrationPhysicalRow, error) {
	query := db.Table(layout.Table).Select("id, values_json").Order("id").Limit(limit)
	if generic {
		query = query.Where("resource = ?", resource)
	}
	if after != "" {
		query = query.Where("id > ?", after)
	}
	var rows []migrationPhysicalRow
	if err := query.Scan(&rows).Error; err != nil {
		return nil, ErrMigrationStore
	}
	return rows, nil
}

func (s *GORMProtectedValueMigrationStore) materializeMigrationTargets(ctx context.Context, tx *gorm.DB, request sensitivemigration.RunRequest) (int, string, error) {
	resources := append([]sensitivemigration.ResourcePlan(nil), request.Plan.Resources...)
	for index := range resources {
		resources[index].Fields = append([]sensitivemigration.FieldPlan(nil), resources[index].Fields...)
		slices.SortFunc(resources[index].Fields, func(left, right sensitivemigration.FieldPlan) int {
			return strings.Compare(left.Key, right.Key)
		})
	}
	slices.SortFunc(resources, func(left, right sensitivemigration.ResourcePlan) int {
		return strings.Compare(left.Resource, right.Resource)
	})
	batch := make([]gormSensitiveMigrationTarget, 0, sensitiveMigrationTargetWriteBatchSize)
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		if err := tx.Create(&batch).Error; err != nil {
			return ErrMigrationStore
		}
		batch = batch[:0]
		return nil
	}
	for _, plan := range resources {
		if ctx.Err() != nil {
			return 0, "", ErrMigrationStore
		}
		layout, generic, err := migrationLayout(plan)
		if err != nil {
			return 0, "", err
		}
		planHash := resourcePlanHash(plan)
		after := ""
		for {
			if ctx.Err() != nil {
				return 0, "", ErrMigrationStore
			}
			rows, err := readMigrationRows(tx, layout, generic, plan, after, sensitivemigration.MaximumBatchSize)
			if err != nil {
				return 0, "", err
			}
			if len(rows) == 0 {
				break
			}
			for _, row := range rows {
				tenantHash := sensitivemigration.TenantCursor(row.TenantID)
				recordHash := sensitivemigration.RecordCursor(row.RecordID)
				for _, field := range plan.Fields {
					batch = append(batch, gormSensitiveMigrationTarget{
						TargetID: migrationSurrogateID("target", request.RunID, plan.Resource, tenantHash, row.RecordID, field.Key),
						RunID:    request.RunID, Resource: plan.Resource, TenantScope: row.TenantID, TenantScopeHash: tenantHash,
						RecordID: row.RecordID, RecordIDHash: recordHash, FieldKey: field.Key, ResourcePlanHash: planHash,
						SnapshotHash: migrationHash("values-json", row.ValuesJSON),
					})
					if len(batch) == cap(batch) {
						if err := flush(); err != nil {
							return 0, "", err
						}
					}
				}
				after = row.RecordID
			}
		}
	}
	if err := flush(); err != nil {
		return 0, "", err
	}
	return migrationTargetSetSummary(tx, request.RunID)
}

type migrationTargetSetHasher struct {
	hash             hash.Hash
	previousTargetID string
	count            int
}

func newMigrationTargetSetHasher() *migrationTargetSetHasher {
	digest := sha256.New()
	_, _ = digest.Write([]byte("platform-go:sensitive-migration:target-set:v1"))
	return &migrationTargetSetHasher{hash: digest}
}

func (h *migrationTargetSetHasher) Add(target gormSensitiveMigrationTarget) error {
	if h == nil || h.hash == nil || target.RunID == "" || target.Resource == "" || target.TenantScope == "" || target.RecordID == "" || target.FieldKey == "" ||
		target.TenantScopeHash != sensitivemigration.TenantCursor(target.TenantScope) ||
		target.RecordIDHash != sensitivemigration.RecordCursor(target.RecordID) ||
		target.TargetID != migrationSurrogateID("target", target.RunID, target.Resource, target.TenantScopeHash, target.RecordID, target.FieldKey) ||
		!canonicalMigrationHash(target.ResourcePlanHash) || !canonicalMigrationHash(target.SnapshotHash) {
		return ErrMigrationConflict
	}
	if h.count > 0 && target.TargetID <= h.previousTargetID {
		return ErrMigrationConflict
	}
	for _, value := range []string{
		target.RunID, target.Resource, target.TenantScope, target.TenantScopeHash, target.RecordID, target.FieldKey,
		target.ResourcePlanHash, target.SnapshotHash,
	} {
		writeMigrationHashPart(h.hash, value)
	}
	h.previousTargetID = target.TargetID
	h.count++
	return nil
}

func (h *migrationTargetSetHasher) Count() int { return h.count }

func (h *migrationTargetSetHasher) Sum() string {
	return "sha256:" + hex.EncodeToString(h.hash.Sum(nil))
}

func migrationTargetSetSummary(tx *gorm.DB, runID string) (int, string, error) {
	if tx == nil || runID == "" {
		return 0, "", ErrMigrationConflict
	}
	hasher := newMigrationTargetSetHasher()
	afterTargetID := ""
	for {
		var targets []gormSensitiveMigrationTarget
		query := tx.Where("run_id = ?", runID)
		if hasher.Count() > 0 {
			query = query.Where("target_id > ?", afterTargetID)
		}
		if err := query.Order("target_id").Limit(sensitivemigration.MaximumBatchSize).Find(&targets).Error; err != nil {
			return 0, "", ErrMigrationStore
		}
		if len(targets) == 0 {
			break
		}
		for _, target := range targets {
			if err := hasher.Add(target); err != nil {
				return 0, "", err
			}
			afterTargetID = target.TargetID
		}
	}
	return hasher.Count(), hasher.Sum(), nil
}

func validateMigrationTargetSet(tx *gorm.DB, run gormSensitiveMigrationRun) error {
	if tx == nil || run.RunID == "" || run.TargetCount < 0 || !canonicalMigrationHash(run.TargetSetHash) || run.TargetSetHash == sensitiveMigrationPendingTargetSetHash {
		return ErrMigrationConflict
	}
	count, targetSetHash, err := migrationTargetSetSummary(tx, run.RunID)
	if err != nil {
		return err
	}
	if count != run.TargetCount || targetSetHash != run.TargetSetHash {
		return ErrMigrationConflict
	}
	return nil
}

func writeMigrationHashPart(writer hash.Hash, value string) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	_, _ = writer.Write(length[:])
	_, _ = writer.Write([]byte(value))
}

func migrationLayout(plan sensitivemigration.ResourcePlan) (gormAdminResourceLayout, bool, error) {
	resource := strings.TrimSpace(plan.Resource)
	if resource == "" || resource != plan.Resource || len(plan.Fields) == 0 {
		return gormAdminResourceLayout{}, false, ErrMigrationInvalidOptions
	}
	if plan.Scope != "global" && plan.Scope != "tenant-field" || plan.Scope == "tenant-field" && strings.TrimSpace(plan.TenantField) == "" {
		return gormAdminResourceLayout{}, false, ErrMigrationInvalidOptions
	}
	layout, normalized := normalizedGORMResourceLayouts[resource]
	if !normalized {
		layout = gormAdminResourceLayout{Table: adminResourceRecordsTable}
	}
	seen := map[string]struct{}{}
	for _, field := range plan.Fields {
		key := strings.TrimSpace(field.Key)
		if key == "" || key != field.Key {
			return gormAdminResourceLayout{}, false, ErrMigrationInvalidOptions
		}
		if _, duplicate := seen[key]; duplicate {
			return gormAdminResourceLayout{}, false, ErrMigrationInvalidOptions
		}
		seen[key] = struct{}{}
		if _, duplicatedProjection := layout.ValueProjections[key]; duplicatedProjection {
			return gormAdminResourceLayout{}, false, ErrMigrationPhysicalLayout
		}
	}
	return layout, !normalized, nil
}

func migrationValues(raw string) (map[string]any, error) {
	rawValues, err := sensitivemigration.DecodeUniqueObject(raw)
	if err != nil {
		return nil, err
	}
	values := map[string]any{}
	for key, rawValue := range rawValues {
		var value any
		if err := json.Unmarshal(rawValue, &value); err != nil {
			return nil, err
		}
		values[key] = value
	}
	return values, nil
}

func migrationChangedFields(original string, updated string) ([]string, error) {
	originalValues, err := sensitivemigration.DecodeUniqueObject(original)
	if err != nil {
		return nil, err
	}
	updatedValues, err := sensitivemigration.DecodeUniqueObject(updated)
	if err != nil {
		return nil, err
	}
	keys := map[string]struct{}{}
	for key := range originalValues {
		keys[key] = struct{}{}
	}
	for key := range updatedValues {
		keys[key] = struct{}{}
	}
	changed := make([]string, 0)
	for key := range keys {
		originalValue, originalExists := originalValues[key]
		updatedValue, updatedExists := updatedValues[key]
		if originalExists != updatedExists || !bytes.Equal(bytes.TrimSpace(originalValue), bytes.TrimSpace(updatedValue)) {
			changed = append(changed, key)
		}
	}
	slices.Sort(changed)
	return changed, nil
}

func migrationStringField(valuesJSON string, field string) (string, error) {
	values, err := sensitivemigration.DecodeUniqueObject(valuesJSON)
	if err != nil {
		return "", err
	}
	raw, ok := values[field]
	if !ok {
		return "", ErrMigrationConflict
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", ErrMigrationConflict
	}
	return value, nil
}

func migrationValuesJSONExactPredicate(driver string) (string, error) {
	switch driver {
	case "mysql":
		return "BINARY values_json = BINARY ?", nil
	case "postgres":
		return "values_json = ?", nil
	case "sqlite":
		return "CAST(values_json AS BLOB) = CAST(? AS BLOB)", nil
	default:
		return "", ErrMigrationUnsupportedDriver
	}
}

func canonicalMigrationHash(value string) bool {
	if len(value) != 71 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	for _, character := range value[len("sha256:"):] {
		if character < '0' || character > '9' && character < 'a' || character > 'f' {
			return false
		}
	}
	return true
}

func canonicalMigrationCursor(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' && character < 'a' || character > 'f' {
			return false
		}
	}
	return true
}

func migrationFieldsAreTargets(changed []string, targets []string) bool {
	targetSet := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		targetSet[target] = struct{}{}
	}
	for _, field := range changed {
		if _, ok := targetSet[field]; !ok {
			return false
		}
	}
	return true
}

func migrationHash(domain string, values ...string) string {
	payload := "platform-go:sensitive-migration:v1\x00" + domain + "\x00" + strings.Join(values, "\x00")
	digest := sha256.Sum256([]byte(payload))
	return "sha256:" + hex.EncodeToString(digest[:])
}

func migrationSurrogateID(domain string, values ...string) string {
	hash := sha256.New()
	for _, value := range append([]string{"platform-go:sensitive-migration:id:v1", domain}, values...) {
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(len(value)))
		_, _ = hash.Write(size[:])
		_, _ = hash.Write([]byte(value))
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func migrationTimestamp() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

func runMatchesRequest(run gormSensitiveMigrationRun, request sensitivemigration.RunRequest) bool {
	return run.PlanHash == request.PlanHash && run.ActorID == request.ActorID && run.Reason == request.Reason &&
		run.ApprovalRef == request.ApprovalRef && run.BackupURI == request.BackupURI && run.BackupHash == request.BackupHash &&
		run.RestoreEvidence == request.RestoreEvidence && run.MaintenanceOK && request.MaintenanceConfirmed
}

func canonicalMigrationRunID(runID string) bool {
	return sensitivemigration.ValidRunIdentity(sensitivemigration.RunRequest{RunID: runID, PlanHash: "present"})
}

func targetsMatchPlan(targets []gormSensitiveMigrationTarget, plan sensitivemigration.ResourcePlan) bool {
	if len(targets) != len(plan.Fields) {
		return false
	}
	planHash := resourcePlanHash(plan)
	fields := make(map[string]struct{}, len(plan.Fields))
	for _, field := range plan.Fields {
		fields[field.Key] = struct{}{}
	}
	for _, target := range targets {
		if target.ResourcePlanHash != planHash {
			return false
		}
		if _, ok := fields[target.FieldKey]; !ok {
			return false
		}
		delete(fields, target.FieldKey)
	}
	return len(fields) == 0
}

func resourcePlanHash(plan sensitivemigration.ResourcePlan) string {
	return sensitivemigration.PlanHash(sensitivemigration.Plan{Resources: []sensitivemigration.ResourcePlan{plan}})
}

func checkpointCounts(checkpoint gormSensitiveMigrationCheckpoint) sensitivemigration.Counts {
	return sensitivemigration.Counts{
		Missing: checkpoint.Missing, Plaintext: checkpoint.Plaintext, TargetEnvelope: checkpoint.TargetEnvelope,
		ForeignEnvelope: checkpoint.ForeignEnvelope, MalformedEnvelope: checkpoint.Malformed,
	}
}

func addMigrationCounts(left sensitivemigration.Counts, right sensitivemigration.Counts) sensitivemigration.Counts {
	return sensitivemigration.Counts{
		Missing: left.Missing + right.Missing, Plaintext: left.Plaintext + right.Plaintext,
		TargetEnvelope:    left.TargetEnvelope + right.TargetEnvelope,
		ForeignEnvelope:   left.ForeignEnvelope + right.ForeignEnvelope,
		MalformedEnvelope: left.MalformedEnvelope + right.MalformedEnvelope,
	}
}

func migrationCountsTotal(counts sensitivemigration.Counts) int {
	return counts.Missing + counts.Plaintext + counts.TargetEnvelope + counts.ForeignEnvelope + counts.MalformedEnvelope
}

type migrationJournalVerification struct {
	Head                    string
	Sequence                uint64
	ApplyCounts             sensitivemigration.Counts
	ApplyBatches            int
	ApplyCheckpointCount    int
	RollbackCounts          sensitivemigration.Counts
	RollbackBatches         int
	RollbackCheckpointCount int
	RehearsalEvents         []gormSensitiveMigrationEvent
}

type migrationEventScopeState struct {
	LastRecordID     string
	ExpectedRevision uint64
	Rows             int
	Counts           sensitivemigration.Counts
	Batches          int
	EventSequence    uint64
	EventHash        string
}

func verifyMigrationJournal(tx *gorm.DB, runID string) (migrationJournalVerification, error) {
	verification := migrationJournalVerification{}
	prior := migrationHash("event-genesis", runID)
	applyEvents := 0
	rollbackEvents := 0
	for {
		var events []gormSensitiveMigrationEvent
		if err := tx.Where("run_id = ? AND sequence > ?", runID, verification.Sequence).
			Order("sequence").Limit(sensitivemigration.MaximumBatchSize).Find(&events).Error; err != nil {
			return migrationJournalVerification{}, ErrMigrationStore
		}
		if len(events) == 0 {
			break
		}
		for _, event := range events {
			if event.Sequence != verification.Sequence+1 || event.PriorEventHash != prior || event.EventHash == "" || event.EventHash != migrationEventHash(event) {
				return migrationJournalVerification{}, ErrMigrationConflict
			}
			switch event.Mode {
			case string(sensitivemigration.ModeApply), string(sensitivemigration.ModeRollback):
				if event.Resource == "" || event.TenantScopeHash == "" || event.LastRecordID == "" || event.Rows < 1 || event.EscrowSetHash != "" {
					return migrationJournalVerification{}, ErrMigrationConflict
				}
				if event.Mode == string(sensitivemigration.ModeRollback) && (event.Plaintext < 1 || event.Missing != 0 || event.TargetEnvelope != 0 || event.ForeignEnvelope != 0 || event.Malformed != 0) {
					return migrationJournalVerification{}, ErrMigrationConflict
				}
				if event.Mode == string(sensitivemigration.ModeApply) {
					applyEvents++
				} else {
					rollbackEvents++
				}
			case string(sensitivemigration.ModeRehearseRestore):
				if event.Resource != sensitiveMigrationEscrowResource || event.TenantScopeHash == "" || event.LastRecordID != "" || event.Rows < 0 || !canonicalMigrationHash(event.EscrowSetHash) ||
					event.Plaintext != event.Rows || event.Missing != 0 || event.TargetEnvelope != 0 || event.ForeignEnvelope != 0 || event.Malformed != 0 {
					return migrationJournalVerification{}, ErrMigrationConflict
				}
				verification.RehearsalEvents = append(verification.RehearsalEvents, event)
				if len(verification.RehearsalEvents) > 1 {
					return migrationJournalVerification{}, ErrMigrationConflict
				}
			default:
				return migrationJournalVerification{}, ErrMigrationConflict
			}
			prior = event.EventHash
			verification.Sequence = event.Sequence
		}
	}
	verification.Head = prior

	afterCheckpointID := ""
	for {
		var checkpoints []gormSensitiveMigrationCheckpoint
		query := tx.Where("run_id = ?", runID)
		if afterCheckpointID != "" {
			query = query.Where("checkpoint_id > ?", afterCheckpointID)
		}
		if err := query.Order("checkpoint_id").Limit(sensitivemigration.MaximumBatchSize).Find(&checkpoints).Error; err != nil {
			return migrationJournalVerification{}, ErrMigrationStore
		}
		if len(checkpoints) == 0 {
			break
		}
		for _, checkpoint := range checkpoints {
			if sensitivemigration.TenantCursor(checkpoint.TenantScope) != checkpoint.TenantScopeHash {
				return migrationJournalVerification{}, ErrMigrationConflict
			}
			state, err := verifyMigrationCheckpoint(tx, checkpoint)
			if err != nil || checkpoint.Status != sensitivemigration.StatusCompleted || checkpoint.LastRecordID != state.LastRecordID ||
				checkpoint.ExpectedRevision != state.ExpectedRevision || checkpoint.Rows != state.Rows ||
				checkpointCounts(checkpoint) != state.Counts || checkpoint.Batches != state.Batches ||
				checkpoint.EventSequence != state.EventSequence || checkpoint.EventHash != state.EventHash {
				return migrationJournalVerification{}, ErrMigrationConflict
			}
			switch checkpoint.Mode {
			case string(sensitivemigration.ModeApply):
				verification.ApplyCounts = addMigrationCounts(verification.ApplyCounts, state.Counts)
				verification.ApplyBatches += state.Batches
				verification.ApplyCheckpointCount++
			case string(sensitivemigration.ModeRollback):
				verification.RollbackCounts = addMigrationCounts(verification.RollbackCounts, state.Counts)
				verification.RollbackBatches += state.Batches
				verification.RollbackCheckpointCount++
			default:
				return migrationJournalVerification{}, ErrMigrationConflict
			}
			afterCheckpointID = checkpoint.CheckpointID
		}
	}
	if verification.ApplyBatches != applyEvents || verification.RollbackBatches != rollbackEvents {
		return migrationJournalVerification{}, ErrMigrationConflict
	}
	return verification, nil
}

func verifyMigrationCheckpoint(tx *gorm.DB, checkpoint gormSensitiveMigrationCheckpoint) (migrationEventScopeState, error) {
	state := migrationEventScopeState{}
	for {
		var events []gormSensitiveMigrationEvent
		if err := tx.Where(
			"run_id = ? AND resource = ? AND tenant_scope_hash = ? AND mode = ? AND sequence > ?",
			checkpoint.RunID, checkpoint.Resource, checkpoint.TenantScopeHash, checkpoint.Mode, state.EventSequence,
		).Order("sequence").Limit(sensitivemigration.MaximumBatchSize).Find(&events).Error; err != nil {
			return migrationEventScopeState{}, ErrMigrationStore
		}
		if len(events) == 0 {
			break
		}
		for _, event := range events {
			if state.LastRecordID != "" && sensitivemigration.RecordCursor(event.LastRecordID) <= sensitivemigration.RecordCursor(state.LastRecordID) {
				return migrationEventScopeState{}, ErrMigrationConflict
			}
			state.LastRecordID = event.LastRecordID
			state.ExpectedRevision = event.Revision
			state.Rows += event.Rows
			state.Counts = addMigrationCounts(state.Counts, eventCounts(event))
			state.Batches++
			state.EventSequence = event.Sequence
			state.EventHash = event.EventHash
		}
	}
	if state.Batches == 0 {
		return migrationEventScopeState{}, ErrMigrationConflict
	}
	return state, nil
}

func eventCounts(event gormSensitiveMigrationEvent) sensitivemigration.Counts {
	return sensitivemigration.Counts{
		Missing: event.Missing, Plaintext: event.Plaintext, TargetEnvelope: event.TargetEnvelope,
		ForeignEnvelope: event.ForeignEnvelope, MalformedEnvelope: event.Malformed,
	}
}

func migrationEventHash(event gormSensitiveMigrationEvent) string {
	return migrationHash(
		"event", event.RunID, strconv.FormatUint(event.Sequence, 10), event.Mode, event.Resource,
		event.TenantScopeHash, event.LastRecordID, strconv.Itoa(event.Rows), strconv.Itoa(event.Missing), strconv.Itoa(event.Plaintext),
		strconv.Itoa(event.TargetEnvelope), strconv.Itoa(event.ForeignEnvelope), strconv.Itoa(event.Malformed),
		event.EscrowSetHash, strconv.FormatUint(event.Revision, 10), event.PriorEventHash, event.CreatedAt,
	)
}

func runState(run gormSensitiveMigrationRun) sensitivemigration.RunState {
	return sensitivemigration.RunState{
		RunID: run.RunID, PlanHash: run.PlanHash, TargetSetHash: run.TargetSetHash, Status: run.Status,
		ExpectedRevision: run.ExpectedRevision, TargetCount: run.TargetCount,
		RestoreRehearsed: run.RestoreRehearsed, RollbackStatus: run.RollbackStatus,
	}
}

func sensitiveMigrationTableNames() []string {
	return []string{
		sensitiveMigrationRunsTable, sensitiveMigrationTargetsTable, sensitiveMigrationCheckpointsTable,
		sensitiveMigrationEventsTable, sensitiveMigrationEscrowTable,
	}
}
