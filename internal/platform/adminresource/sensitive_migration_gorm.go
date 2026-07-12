package adminresource

import (
	"context"
	"crypto/sha256"
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
	RunID            string `gorm:"column:run_id;primaryKey"`
	PlanHash         string `gorm:"column:plan_hash;not null"`
	Status           string `gorm:"column:status;not null"`
	ExpectedRevision uint64 `gorm:"column:expected_revision;not null"`
	TargetCount      int    `gorm:"column:target_count;not null"`
	CreatedAt        string `gorm:"column:created_at;not null"`
}

type gormSensitiveMigrationTarget struct {
	RunID           string `gorm:"column:run_id;primaryKey"`
	Resource        string `gorm:"column:resource;primaryKey"`
	TenantScope     string `gorm:"column:tenant_scope;not null"`
	TenantScopeHash string `gorm:"column:tenant_scope_hash;primaryKey"`
	RecordID        string `gorm:"column:record_id;primaryKey"`
	FieldKey        string `gorm:"column:field_key;primaryKey"`
	SnapshotHash    string `gorm:"column:snapshot_hash;not null"`
}

type gormSensitiveMigrationCheckpoint struct {
	RunID            string `gorm:"column:run_id;primaryKey"`
	Resource         string `gorm:"column:resource;primaryKey"`
	TenantScope      string `gorm:"column:tenant_scope;not null"`
	TenantScopeHash  string `gorm:"column:tenant_scope_hash;primaryKey"`
	Mode             string `gorm:"column:mode;primaryKey"`
	LastRecordID     string `gorm:"column:last_record_id;not null"`
	ExpectedRevision uint64 `gorm:"column:expected_revision;not null"`
	Rows             int    `gorm:"column:row_count;not null"`
	Status           string `gorm:"column:status;not null"`
	EventSequence    uint64 `gorm:"column:event_sequence;not null"`
	UpdatedAt        string `gorm:"column:updated_at;not null"`
}

type gormSensitiveMigrationEvent struct {
	RunID           string `gorm:"column:run_id;primaryKey"`
	Sequence        uint64 `gorm:"column:sequence;primaryKey"`
	Mode            string `gorm:"column:mode;not null"`
	Resource        string `gorm:"column:resource;not null"`
	TenantScopeHash string `gorm:"column:tenant_scope_hash;not null"`
	Rows            int    `gorm:"column:row_count;not null"`
	PriorEventHash  string `gorm:"column:prior_event_hash;not null"`
	EventHash       string `gorm:"column:event_hash;not null"`
	CreatedAt       string `gorm:"column:created_at;not null"`
}

type gormSensitiveMigrationEscrow struct {
	RunID             string `gorm:"column:run_id;primaryKey"`
	Resource          string `gorm:"column:resource;primaryKey"`
	TenantScopeHash   string `gorm:"column:tenant_scope_hash;primaryKey"`
	RecordID          string `gorm:"column:record_id;primaryKey"`
	FieldKey          string `gorm:"column:field_key;primaryKey"`
	ProtectedOriginal string `gorm:"column:protected_original;not null"`
	MigratedValueHash string `gorm:"column:migrated_value_hash;not null"`
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
	revision, err := loadGORMRevision(s.db.WithContext(ctx))
	if err != nil {
		return sensitivemigration.RunState{}, ErrMigrationStore
	}
	targets, err := s.prepareTargets(ctx, request)
	if err != nil {
		return sensitivemigration.RunState{}, err
	}
	if err := s.db.WithContext(ctx).AutoMigrate(
		&gormSensitiveMigrationRun{}, &gormSensitiveMigrationTarget{}, &gormSensitiveMigrationCheckpoint{},
		&gormSensitiveMigrationEvent{}, &gormSensitiveMigrationEscrow{},
	); err != nil {
		return sensitivemigration.RunState{}, ErrMigrationStore
	}

	var state sensitivemigration.RunState
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing gormSensitiveMigrationRun
		findErr := tx.Where("run_id = ?", request.RunID).First(&existing).Error
		if findErr == nil {
			if existing.PlanHash != request.PlanHash {
				return ErrMigrationConflict
			}
			state = runState(existing)
			return nil
		}
		if !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return ErrMigrationStore
		}
		run := gormSensitiveMigrationRun{
			RunID: request.RunID, PlanHash: request.PlanHash, Status: sensitivemigration.StatusPrepared,
			ExpectedRevision: revision, TargetCount: len(targets), CreatedAt: migrationTimestamp(),
		}
		if err := tx.Create(&run).Error; err != nil {
			return ErrMigrationStore
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
	tenantHash := migrationHash("tenant-scope", tenant)
	nextRevision := mutation.ExpectedRevision + 1
	commit := sensitivemigration.BatchCommit{}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var run gormSensitiveMigrationRun
		if err := tx.Where("run_id = ? AND status = ?", mutation.RunID, sensitivemigration.StatusPrepared).First(&run).Error; err != nil {
			return ErrMigrationConflict
		}

		var revisionState gormAdminResourceState
		revisionQuery := tx.Where("key = ?", "revision")
		if s.driver != "sqlite" {
			revisionQuery = revisionQuery.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := revisionQuery.First(&revisionState).Error; err != nil {
			return ErrMigrationConflict
		}
		actual, parseErr := strconv.ParseUint(revisionState.Value, 10, 64)
		if parseErr != nil || actual != mutation.ExpectedRevision {
			return ErrMigrationConflict
		}

		for _, row := range mutation.Rows {
			if strings.TrimSpace(row.RecordID) == "" || row.OriginalValuesJSON == "" || row.UpdatedValuesJSON == "" {
				return ErrMigrationInvalidOptions
			}
			var targetFields []string
			if err := tx.Model(&gormSensitiveMigrationTarget{}).
				Where("run_id = ? AND resource = ? AND tenant_scope_hash = ? AND record_id = ?", mutation.RunID, mutation.Resource.Resource, tenantHash, row.RecordID).
				Pluck("field_key", &targetFields).Error; err != nil || len(targetFields) == 0 {
				return ErrMigrationConflict
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
			RunID: mutation.RunID, Sequence: sequence, Mode: string(mutation.Mode), Resource: mutation.Resource.Resource,
			TenantScopeHash: tenantHash, Rows: len(mutation.Rows), CreatedAt: now,
		}
		if err := tx.Create(&event).Error; err != nil {
			return ErrMigrationStore
		}
		checkpoint := gormSensitiveMigrationCheckpoint{
			RunID: mutation.RunID, Resource: mutation.Resource.Resource, TenantScope: tenant,
			TenantScopeHash: tenantHash, Mode: string(mutation.Mode), LastRecordID: mutation.LastRecordID,
			ExpectedRevision: nextRevision, Rows: len(mutation.Rows), Status: sensitivemigration.StatusCompleted,
			EventSequence: sequence, UpdatedAt: now,
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "run_id"}, {Name: "resource"}, {Name: "tenant_scope_hash"}, {Name: "mode"}},
			DoUpdates: clause.AssignmentColumns([]string{"tenant_scope", "last_record_id", "expected_revision", "row_count", "status", "event_sequence", "updated_at"}),
		}).Create(&checkpoint).Error; err != nil {
			return ErrMigrationStore
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
							RunID: request.RunID, Resource: plan.Resource, TenantScope: tenant, TenantScopeHash: tenantHash,
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
