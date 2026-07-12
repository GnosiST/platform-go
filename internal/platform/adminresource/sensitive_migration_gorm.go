package adminresource

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
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
	sensitiveMigrationRunsTable        = "platform_sensitive_migration_runs"
	sensitiveMigrationTargetsTable     = "platform_sensitive_migration_targets"
	sensitiveMigrationCheckpointsTable = "platform_sensitive_migration_checkpoints"
	sensitiveMigrationEventsTable      = "platform_sensitive_migration_events"
	sensitiveMigrationEscrowTable      = "platform_sensitive_migration_escrow"
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
	RunID            string `gorm:"column:run_id;size:64;primaryKey"`
	PlanHash         string `gorm:"column:plan_hash;size:71;not null"`
	Status           string `gorm:"column:status;size:32;not null"`
	ExpectedRevision uint64 `gorm:"column:expected_revision;not null"`
	TargetCount      int    `gorm:"column:target_count;not null"`
	CreatedAt        string `gorm:"column:created_at;size:35;not null"`
}

type gormSensitiveMigrationTarget struct {
	TargetID        string `gorm:"column:target_id;size:64;primaryKey"`
	RunID           string `gorm:"column:run_id;size:64;not null;index:idx_sensitive_target_lookup,priority:1"`
	Resource        string `gorm:"column:resource;size:128;not null;index:idx_sensitive_target_lookup,priority:2"`
	TenantScope     string `gorm:"column:tenant_scope;size:191;not null"`
	TenantScopeHash string `gorm:"column:tenant_scope_hash;size:71;not null;index:idx_sensitive_target_lookup,priority:3"`
	RecordID        string `gorm:"column:record_id;size:191;not null;index:idx_sensitive_target_lookup,priority:4"`
	FieldKey        string `gorm:"column:field_key;size:128;not null"`
	SnapshotHash    string `gorm:"column:snapshot_hash;size:71;not null"`
}

type gormSensitiveMigrationCheckpoint struct {
	CheckpointID     string `gorm:"column:checkpoint_id;size:64;primaryKey"`
	RunID            string `gorm:"column:run_id;size:64;not null;index:idx_sensitive_checkpoint_lookup,priority:1"`
	Resource         string `gorm:"column:resource;size:128;not null;index:idx_sensitive_checkpoint_lookup,priority:2"`
	TenantScope      string `gorm:"column:tenant_scope;size:191;not null"`
	TenantScopeHash  string `gorm:"column:tenant_scope_hash;size:71;not null;index:idx_sensitive_checkpoint_lookup,priority:3"`
	Mode             string `gorm:"column:mode;size:32;not null;index:idx_sensitive_checkpoint_lookup,priority:4"`
	LastRecordID     string `gorm:"column:last_record_id;size:191;not null"`
	ExpectedRevision uint64 `gorm:"column:expected_revision;not null"`
	Rows             int    `gorm:"column:row_count;not null"`
	Status           string `gorm:"column:status;size:32;not null"`
	EventSequence    uint64 `gorm:"column:event_sequence;not null"`
	UpdatedAt        string `gorm:"column:updated_at;size:35;not null"`
}

type gormSensitiveMigrationEvent struct {
	EventID         string `gorm:"column:event_id;size:64;primaryKey"`
	RunID           string `gorm:"column:run_id;size:64;not null;uniqueIndex:idx_sensitive_event_sequence,priority:1"`
	Sequence        uint64 `gorm:"column:sequence;not null;uniqueIndex:idx_sensitive_event_sequence,priority:2"`
	Mode            string `gorm:"column:mode;size:32;not null"`
	Resource        string `gorm:"column:resource;size:128;not null"`
	TenantScopeHash string `gorm:"column:tenant_scope_hash;size:71;not null"`
	Rows            int    `gorm:"column:row_count;not null"`
	PriorEventHash  string `gorm:"column:prior_event_hash;size:71;not null"`
	EventHash       string `gorm:"column:event_hash;size:71;not null"`
	CreatedAt       string `gorm:"column:created_at;size:35;not null"`
}

type gormSensitiveMigrationEscrow struct {
	EscrowID          string `gorm:"column:escrow_id;size:64;primaryKey"`
	RunID             string `gorm:"column:run_id;size:64;not null;index:idx_sensitive_escrow_lookup,priority:1"`
	Resource          string `gorm:"column:resource;size:128;not null;index:idx_sensitive_escrow_lookup,priority:2"`
	TenantScopeHash   string `gorm:"column:tenant_scope_hash;size:71;not null;index:idx_sensitive_escrow_lookup,priority:3"`
	RecordID          string `gorm:"column:record_id;size:191;not null;index:idx_sensitive_escrow_lookup,priority:4"`
	FieldKey          string `gorm:"column:field_key;size:128;not null;index:idx_sensitive_escrow_lookup,priority:5"`
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

func (s *GORMProtectedValueMigrationStore) TenantScopes(ctx context.Context, plan sensitivemigration.ResourcePlan) ([]string, error) {
	layout, generic, err := migrationLayout(plan)
	if err != nil {
		return nil, err
	}
	if ctx == nil || ctx.Err() != nil {
		return nil, ErrMigrationStore
	}
	if plan.Scope == "global" {
		return []string{dataprotection.GlobalTenantID}, nil
	}

	scopes := map[string]struct{}{}
	after := ""
	for {
		rows, err := s.readPhysicalRows(ctx, layout, generic, plan.Resource, after, sensitivemigration.MaximumBatchSize)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			break
		}
		for _, row := range rows {
			values, parseErr := migrationValues(row.ValuesJSON)
			if parseErr != nil {
				return nil, ErrMigrationStore
			}
			tenant, ok := values[plan.TenantField].(string)
			tenant = strings.TrimSpace(tenant)
			if !ok || tenant == "" {
				return nil, ErrMigrationStore
			}
			scopes[tenant] = struct{}{}
			after = row.ID
		}
	}
	result := make([]string, 0, len(scopes))
	for scope := range scopes {
		result = append(result, scope)
	}
	slices.Sort(result)
	return result, nil
}

func (s *GORMProtectedValueMigrationStore) Rows(ctx context.Context, plan sensitivemigration.ResourcePlan, tenant string, after string, limit int) ([]sensitivemigration.Row, error) {
	layout, generic, err := migrationLayout(plan)
	if err != nil {
		return nil, err
	}
	if ctx == nil || ctx.Err() != nil || limit < 1 || limit > sensitivemigration.MaximumBatchSize {
		return nil, ErrMigrationInvalidOptions
	}
	tenant = strings.TrimSpace(tenant)
	if plan.Scope == "global" && tenant != dataprotection.GlobalTenantID || plan.Scope == "tenant-field" && tenant == "" {
		return nil, ErrMigrationInvalidOptions
	}
	result := make([]sensitivemigration.Row, 0, limit)
	scanAfter := after
	for len(result) < limit {
		rows, err := s.readPhysicalRows(ctx, layout, generic, plan.Resource, scanAfter, limit)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			break
		}
		for _, row := range rows {
			if plan.Scope == "tenant-field" {
				values, parseErr := migrationValues(row.ValuesJSON)
				if parseErr != nil {
					return nil, ErrMigrationStore
				}
				rowTenant, ok := values[plan.TenantField].(string)
				if !ok || strings.TrimSpace(rowTenant) != tenant {
					scanAfter = row.ID
					continue
				}
			}
			result = append(result, sensitivemigration.Row{Resource: plan.Resource, RecordID: row.ID, ValuesJSON: row.ValuesJSON})
			scanAfter = row.ID
			if len(result) == limit {
				break
			}
		}
	}
	return result, nil
}

func (s *GORMProtectedValueMigrationStore) Prepare(ctx context.Context, request sensitivemigration.RunRequest) (sensitivemigration.RunState, error) {
	if s == nil || s.db == nil || ctx == nil || ctx.Err() != nil || strings.TrimSpace(request.RunID) == "" || strings.TrimSpace(request.PlanHash) == "" || len(request.Plan.Resources) == 0 {
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
		if existing.PlanHash != request.PlanHash {
			return sensitivemigration.RunState{}, ErrMigrationConflict
		}
		return runState(existing), nil
	}

	revision, err := loadGORMRevision(s.db.WithContext(ctx))
	if err != nil {
		return sensitivemigration.RunState{}, ErrMigrationStore
	}
	targets, err := s.prepareTargets(ctx, request)
	if err != nil {
		return sensitivemigration.RunState{}, err
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
			RunID: request.RunID, PlanHash: request.PlanHash, Status: sensitivemigration.StatusPrepared,
			ExpectedRevision: revision, TargetCount: len(targets), CreatedAt: migrationTimestamp(),
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
			if existing.PlanHash != request.PlanHash {
				return ErrMigrationConflict
			}
			state = runState(existing)
			return nil
		}
		if len(targets) > 0 {
			if err := tx.Create(&targets).Error; err != nil {
				return ErrMigrationStore
			}
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
	if s == nil || s.db == nil || ctx == nil || ctx.Err() != nil || strings.TrimSpace(mutation.RunID) == "" || mutation.Mode != sensitivemigration.ModeApply || len(mutation.Rows) == 0 || mutation.ExpectedRevision == ^uint64(0) {
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
		if previousRecordID != "" && row.RecordID <= previousRecordID {
			return sensitivemigration.BatchCommit{}, ErrMigrationConflict
		}
		previousRecordID = row.RecordID
	}
	if previousRecordID != mutation.LastRecordID {
		return sensitivemigration.BatchCommit{}, ErrMigrationConflict
	}
	tenantHash := migrationHash("tenant-scope", tenant)
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
		actual, err := s.lockedRevision(tx)
		if err != nil || actual != mutation.ExpectedRevision {
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
			if checkpoint.ExpectedRevision != mutation.ExpectedRevision || mutation.Rows[0].RecordID <= checkpoint.LastRecordID {
				return ErrMigrationConflict
			}
		} else if !errors.Is(checkpointErr, gorm.ErrRecordNotFound) {
			return ErrMigrationStore
		}

		for _, row := range mutation.Rows {
			var targets []gormSensitiveMigrationTarget
			if err := tx.Model(&gormSensitiveMigrationTarget{}).
				Where("run_id = ? AND resource = ? AND tenant_scope = ? AND tenant_scope_hash = ? AND record_id = ?", mutation.RunID, mutation.Resource.Resource, tenant, tenantHash, row.RecordID).
				Find(&targets).Error; err != nil || len(targets) == 0 {
				return ErrMigrationConflict
			}
			targetFields := make([]string, 0, len(targets))
			snapshotHash := migrationHash("values-json", row.OriginalValuesJSON)
			for _, target := range targets {
				if target.SnapshotHash != snapshotHash {
					return ErrMigrationConflict
				}
				targetFields = append(targetFields, target.FieldKey)
			}
			changedFields, err := migrationChangedFields(row.OriginalValuesJSON, row.UpdatedValuesJSON)
			if err != nil || !migrationFieldsAreTargets(changedFields, targetFields) {
				return ErrMigrationConflict
			}
			query := tx.Table(layout.Table).Where("id = ? AND values_json = ?", row.RecordID, row.OriginalValuesJSON)
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

		var sequence uint64
		if err := tx.Model(&gormSensitiveMigrationEvent{}).Where("run_id = ?", mutation.RunID).
			Select("COALESCE(MAX(sequence), 0)").Scan(&sequence).Error; err != nil {
			return ErrMigrationStore
		}
		sequence++
		now := migrationTimestamp()
		event := gormSensitiveMigrationEvent{
			EventID: migrationSurrogateID("event", mutation.RunID, strconv.FormatUint(sequence, 10)),
			RunID:   mutation.RunID, Sequence: sequence, Mode: string(mutation.Mode), Resource: mutation.Resource.Resource,
			TenantScopeHash: tenantHash, Rows: len(mutation.Rows), CreatedAt: now,
		}
		if err := tx.Create(&event).Error; err != nil {
			return ErrMigrationStore
		}
		cumulativeRows := len(mutation.Rows)
		if checkpointFound {
			cumulativeRows += checkpoint.Rows
			updated := tx.Model(&gormSensitiveMigrationCheckpoint{}).
				Where("checkpoint_id = ? AND expected_revision = ? AND last_record_id = ?", checkpointID, mutation.ExpectedRevision, checkpoint.LastRecordID).
				Updates(map[string]any{
					"tenant_scope": tenant, "last_record_id": mutation.LastRecordID, "expected_revision": nextRevision,
					"row_count": cumulativeRows, "status": sensitivemigration.StatusCompleted,
					"event_sequence": sequence, "updated_at": now,
				})
			if updated.Error != nil || updated.RowsAffected != 1 {
				return ErrMigrationConflict
			}
		} else {
			checkpoint = gormSensitiveMigrationCheckpoint{
				CheckpointID: checkpointID, RunID: mutation.RunID, Resource: mutation.Resource.Resource, TenantScope: tenant,
				TenantScopeHash: tenantHash, Mode: string(mutation.Mode), LastRecordID: mutation.LastRecordID,
				ExpectedRevision: nextRevision, Rows: cumulativeRows, Status: sensitivemigration.StatusCompleted,
				EventSequence: sequence, UpdatedAt: now,
			}
			if err := tx.Create(&checkpoint).Error; err != nil {
				return ErrMigrationStore
			}
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
		commit = sensitivemigration.BatchCommit{Revision: nextRevision, Rows: len(mutation.Rows), LastRecordID: mutation.LastRecordID, EventSequence: sequence}
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

type migrationPhysicalRow struct {
	ID         string `gorm:"column:id"`
	ValuesJSON string `gorm:"column:values_json"`
}

func (s *GORMProtectedValueMigrationStore) readPhysicalRows(ctx context.Context, layout gormAdminResourceLayout, generic bool, resource string, after string, limit int) ([]migrationPhysicalRow, error) {
	query := s.db.WithContext(ctx).Table(layout.Table).Select("id, values_json").Order("id").Limit(limit)
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

func (s *GORMProtectedValueMigrationStore) prepareTargets(ctx context.Context, request sensitivemigration.RunRequest) ([]gormSensitiveMigrationTarget, error) {
	targets := make([]gormSensitiveMigrationTarget, 0)
	for _, plan := range request.Plan.Resources {
		scopes, err := s.TenantScopes(ctx, plan)
		if err != nil {
			return nil, err
		}
		for _, tenant := range scopes {
			after := ""
			for {
				rows, err := s.Rows(ctx, plan, tenant, after, sensitivemigration.MaximumBatchSize)
				if err != nil {
					return nil, err
				}
				if len(rows) == 0 {
					break
				}
				tenantHash := migrationHash("tenant-scope", tenant)
				for _, row := range rows {
					for _, field := range plan.Fields {
						targets = append(targets, gormSensitiveMigrationTarget{
							TargetID: migrationSurrogateID("target", request.RunID, plan.Resource, tenantHash, row.RecordID, field.Key),
							RunID:    request.RunID, Resource: plan.Resource, TenantScope: tenant, TenantScopeHash: tenantHash,
							RecordID: row.RecordID, FieldKey: field.Key, SnapshotHash: migrationHash("values-json", row.ValuesJSON),
						})
					}
					after = row.RecordID
				}
			}
		}
	}
	return targets, nil
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
	values := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil, err
	}
	return values, nil
}

func migrationChangedFields(original string, updated string) ([]string, error) {
	originalValues, err := migrationValues(original)
	if err != nil {
		return nil, err
	}
	updatedValues, err := migrationValues(updated)
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
		if !reflect.DeepEqual(originalValues[key], updatedValues[key]) {
			changed = append(changed, key)
		}
	}
	slices.Sort(changed)
	if len(changed) == 0 {
		return nil, ErrMigrationConflict
	}
	return changed, nil
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

func runState(run gormSensitiveMigrationRun) sensitivemigration.RunState {
	return sensitivemigration.RunState{
		RunID: run.RunID, PlanHash: run.PlanHash, Status: run.Status,
		ExpectedRevision: run.ExpectedRevision, TargetCount: run.TargetCount,
	}
}

func sensitiveMigrationTableNames() []string {
	return []string{
		sensitiveMigrationRunsTable, sensitiveMigrationTargetsTable, sensitiveMigrationCheckpointsTable,
		sensitiveMigrationEventsTable, sensitiveMigrationEscrowTable,
	}
}
