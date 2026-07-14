package datalifecycle

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	dataLifecycleLeasesTable        = "platform_data_lifecycle_leases"
	dataLifecycleCheckpointsTable   = "platform_data_lifecycle_checkpoints"
	dataLifecycleImpactReportsTable = "platform_data_lifecycle_impact_reports"
	dataLifecyclePromotionsTable    = "platform_data_lifecycle_promotions"
)

type GORMRepository struct {
	db *gorm.DB
}

type gormLifecycleLease struct {
	LeaseID           string    `gorm:"column:lease_id;size:71;primaryKey"`
	DatasourceID      string    `gorm:"column:datasource_id;size:64;not null"`
	LeaseKey          string    `gorm:"column:lease_key;size:128;not null"`
	OwnerID           string    `gorm:"column:owner_id;size:128;not null"`
	Token             string    `gorm:"column:token;size:64;not null"`
	PolicyFingerprint string    `gorm:"column:policy_fingerprint;size:71;not null"`
	AcquiredAt        time.Time `gorm:"column:acquired_at;not null"`
	HeartbeatAt       time.Time `gorm:"column:heartbeat_at;not null"`
	ExpiresAt         time.Time `gorm:"column:expires_at;index;not null"`
}

type gormLifecycleCheckpoint struct {
	CheckpointID      string    `gorm:"column:checkpoint_id;size:71;primaryKey"`
	DatasourceID      string    `gorm:"column:datasource_id;size:64;not null"`
	RunID             string    `gorm:"column:run_id;size:128;not null"`
	Mode              string    `gorm:"column:mode;size:32;not null"`
	PolicyFingerprint string    `gorm:"column:policy_fingerprint;size:71;not null"`
	CursorDatasource  string    `gorm:"column:cursor_datasource_id;size:64;not null"`
	CursorResource    string    `gorm:"column:cursor_resource;size:128;not null"`
	CursorEligibleAt  string    `gorm:"column:cursor_eligible_at;size:64;not null"`
	CursorRecordID    string    `gorm:"column:cursor_record_id;size:191;not null"`
	Eligible          int       `gorm:"column:eligible_count;not null"`
	Planned           int       `gorm:"column:planned_count;not null"`
	Applied           int       `gorm:"column:applied_count;not null"`
	Batches           int       `gorm:"column:batch_count;not null"`
	Retries           int       `gorm:"column:retry_count;not null"`
	EvidenceHash      string    `gorm:"column:evidence_hash;size:71;not null"`
	IntegrityHash     string    `gorm:"column:integrity_hash;size:71;not null"`
	LastBatchID       string    `gorm:"column:last_batch_id;size:71;not null"`
	Revision          uint64    `gorm:"column:revision;not null"`
	Complete          bool      `gorm:"column:complete;not null"`
	UpdatedAt         time.Time `gorm:"column:updated_at;not null"`
}

type gormLifecycleImpactReport struct {
	ReportHash        string    `gorm:"column:report_hash;size:71;primaryKey"`
	DatasourceID      string    `gorm:"column:datasource_id;size:64;not null"`
	RunID             string    `gorm:"column:run_id;size:128;not null"`
	PolicyFingerprint string    `gorm:"column:policy_fingerprint;size:71;not null"`
	Eligible          int       `gorm:"column:eligible_count;not null"`
	Planned           int       `gorm:"column:planned_count;not null"`
	Applied           int       `gorm:"column:applied_count;not null"`
	Batches           int       `gorm:"column:batch_count;not null"`
	Retries           int       `gorm:"column:retry_count;not null"`
	CursorDatasource  string    `gorm:"column:cursor_datasource_id;size:64;not null"`
	CursorResource    string    `gorm:"column:cursor_resource;size:128;not null"`
	CursorEligibleAt  string    `gorm:"column:cursor_eligible_at;size:64;not null"`
	CursorRecordID    string    `gorm:"column:cursor_record_id;size:191;not null"`
	EvidenceHash      string    `gorm:"column:evidence_hash;size:71;not null"`
	GeneratedAt       time.Time `gorm:"column:generated_at;not null"`
}

type gormLifecyclePromotion struct {
	PromotedFingerprint string    `gorm:"column:promoted_fingerprint;size:71;primaryKey"`
	DatasourceID        string    `gorm:"column:datasource_id;size:64;not null"`
	CurrentFingerprint  string    `gorm:"column:current_fingerprint;size:71;not null"`
	ImpactReportHash    string    `gorm:"column:impact_report_hash;size:71;not null"`
	ActorID             string    `gorm:"column:actor_id;size:128;not null"`
	Reason              string    `gorm:"column:reason;size:191;not null"`
	ApprovalRef         string    `gorm:"column:approval_ref;size:191;not null"`
	PromotedAt          time.Time `gorm:"column:promoted_at;not null"`
}

func (gormLifecycleLease) TableName() string        { return dataLifecycleLeasesTable }
func (gormLifecycleCheckpoint) TableName() string   { return dataLifecycleCheckpointsTable }
func (gormLifecycleImpactReport) TableName() string { return dataLifecycleImpactReportsTable }
func (gormLifecyclePromotion) TableName() string    { return dataLifecyclePromotionsTable }

func PrepareGORMRepository(ctx context.Context, db *gorm.DB) (*GORMRepository, error) {
	if ctx == nil || db == nil {
		return nil, ErrRepositoryFailed
	}
	if err := db.WithContext(ctx).AutoMigrate(
		&gormLifecycleLease{},
		&gormLifecycleCheckpoint{},
		&gormLifecycleImpactReport{},
		&gormLifecyclePromotion{},
	); err != nil {
		return nil, ErrRepositoryFailed
	}
	return OpenGORMRepository(ctx, db)
}

func OpenGORMRepository(ctx context.Context, db *gorm.DB) (*GORMRepository, error) {
	if ctx == nil || db == nil {
		return nil, ErrRepositoryFailed
	}
	for _, model := range lifecycleRepositoryModels() {
		if !db.WithContext(ctx).Migrator().HasTable(model) {
			return nil, ErrRepositoryFailed
		}
	}
	return &GORMRepository{db: db}, nil
}

func (r *GORMRepository) Persistent() bool {
	return r != nil && r.db != nil
}

func (r *GORMRepository) AcquireLease(ctx context.Context, request LeaseRequest) (Lease, error) {
	request.DatasourceID = normalizedLifecycleDatasource(request.DatasourceID)
	if !r.ready(ctx) || !validLeaseRequest(request) {
		return Lease{}, ErrRepositoryFailed
	}
	token, err := newLeaseToken()
	if err != nil {
		return Lease{}, ErrRepositoryFailed
	}
	next := Lease{
		DatasourceID: request.DatasourceID, Key: request.Key, OwnerID: request.OwnerID,
		Token: token, PolicyFingerprint: request.PolicyFingerprint,
		AcquiredAt: request.Now.UTC(), HeartbeatAt: request.Now.UTC(), ExpiresAt: request.Now.UTC().Add(request.TTL),
	}
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		id := lifecycleLeaseID(request.DatasourceID, request.Key)
		var current gormLifecycleLease
		err := tx.Where("lease_id = ?", id).Take(&current).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			row := gormLeaseFromLease(next)
			created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
			if created.Error != nil {
				return ErrRepositoryFailed
			}
			if created.RowsAffected != 1 {
				return ErrLeaseHeld
			}
			return nil
		}
		if err != nil {
			return ErrRepositoryFailed
		}
		if current.ExpiresAt.After(request.Now.UTC()) {
			return ErrLeaseHeld
		}
		result := tx.Model(&gormLifecycleLease{}).
			Where("lease_id = ? AND token = ? AND expires_at <= ?", id, current.Token, request.Now.UTC()).
			Updates(gormLeaseUpdate(next))
		if result.Error != nil {
			return ErrRepositoryFailed
		}
		if result.RowsAffected != 1 {
			return ErrLeaseHeld
		}
		return nil
	})
	if err != nil {
		return Lease{}, err
	}
	return next, nil
}

func (r *GORMRepository) HeartbeatLease(ctx context.Context, lease Lease, now time.Time, ttl time.Duration) (Lease, error) {
	lease.DatasourceID = normalizedLifecycleDatasource(lease.DatasourceID)
	if !r.ready(ctx) || !validStoredLease(lease) || now.IsZero() || now.Before(lease.HeartbeatAt) || ttl <= 0 {
		return Lease{}, ErrRepositoryFailed
	}
	now = now.UTC()
	next := lease
	next.HeartbeatAt = now
	next.ExpiresAt = now.Add(ttl)
	result := r.db.WithContext(ctx).Model(&gormLifecycleLease{}).
		Where("lease_id = ? AND datasource_id = ? AND lease_key = ? AND owner_id = ? AND token = ? AND policy_fingerprint = ? AND expires_at > ?",
			lifecycleLeaseID(lease.DatasourceID, lease.Key), lease.DatasourceID, lease.Key, lease.OwnerID, lease.Token, lease.PolicyFingerprint, now).
		Updates(map[string]any{"heartbeat_at": next.HeartbeatAt, "expires_at": next.ExpiresAt})
	if result.Error != nil {
		return Lease{}, ErrRepositoryFailed
	}
	if result.RowsAffected != 1 {
		return Lease{}, ErrLeaseLost
	}
	return next, nil
}

func (r *GORMRepository) ReleaseLease(ctx context.Context, lease Lease) error {
	lease.DatasourceID = normalizedLifecycleDatasource(lease.DatasourceID)
	if !r.ready(ctx) || !validStoredLease(lease) {
		return ErrRepositoryFailed
	}
	result := r.db.WithContext(ctx).
		Where("lease_id = ? AND datasource_id = ? AND lease_key = ? AND owner_id = ? AND token = ? AND policy_fingerprint = ?",
			lifecycleLeaseID(lease.DatasourceID, lease.Key), lease.DatasourceID, lease.Key, lease.OwnerID, lease.Token, lease.PolicyFingerprint).
		Delete(&gormLifecycleLease{})
	if result.Error != nil {
		return ErrRepositoryFailed
	}
	if result.RowsAffected != 1 {
		return ErrLeaseLost
	}
	return nil
}

func (r *GORMRepository) LoadCheckpoint(ctx context.Context, key CheckpointKey) (Checkpoint, bool, error) {
	key.DatasourceID = normalizedLifecycleDatasource(key.DatasourceID)
	if !r.ready(ctx) || !validCheckpointKey(key) {
		return Checkpoint{}, false, ErrRepositoryFailed
	}
	var row gormLifecycleCheckpoint
	err := r.db.WithContext(ctx).Where("checkpoint_id = ?", lifecycleCheckpointID(key)).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Checkpoint{}, false, nil
	}
	if err != nil {
		return Checkpoint{}, false, ErrRepositoryFailed
	}
	checkpoint := row.checkpoint()
	if !validPersistedCheckpoint(checkpoint) || checkpoint.Key != key {
		return Checkpoint{}, false, ErrRepositoryFailed
	}
	return checkpoint, true, nil
}

func (r *GORMRepository) SaveCheckpoint(ctx context.Context, lease Lease, checkpoint Checkpoint) error {
	if !r.ready(ctx) || !validPersistedCheckpoint(checkpoint) {
		return ErrRepositoryFailed
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := validateLeaseFence(tx, lease, time.Now().UTC()); err != nil {
			return err
		}
		return saveCheckpoint(tx, checkpoint)
	})
}

func (r *GORMRepository) SaveImpactReportAndCheckpoint(ctx context.Context, lease Lease, report ImpactReport, checkpoint Checkpoint) error {
	if !r.ready(ctx) || !validImpactCompletion(report, checkpoint) {
		return ErrRepositoryFailed
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := validateLeaseFence(tx, lease, time.Now().UTC()); err != nil {
			return err
		}
		if err := saveCheckpoint(tx, checkpoint); err != nil {
			return err
		}
		var existing gormLifecycleImpactReport
		err := tx.Where("report_hash = ?", report.ReportHash).Take(&existing).Error
		if err == nil {
			if !sameImpactReport(existing.impactReport(), report) {
				return ErrRepositoryFailed
			}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrRepositoryFailed
		}
		row := gormImpactReportFromReport(report)
		if err := tx.Create(&row).Error; err != nil {
			return ErrRepositoryFailed
		}
		return nil
	})
}

func (r *GORMRepository) LoadImpactReport(ctx context.Context, datasourceID, reportHash string) (ImpactReport, bool, error) {
	datasourceID = normalizedLifecycleDatasource(datasourceID)
	if !r.ready(ctx) || datasourceID != DefaultDatasourceID || !canonicalDigest(reportHash) {
		return ImpactReport{}, false, ErrRepositoryFailed
	}
	var row gormLifecycleImpactReport
	err := r.db.WithContext(ctx).Where("report_hash = ? AND datasource_id = ?", reportHash, datasourceID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ImpactReport{}, false, nil
	}
	if err != nil {
		return ImpactReport{}, false, ErrRepositoryFailed
	}
	report := row.impactReport()
	if !validStoredImpactReport(report) || report.ReportHash != reportHash {
		return ImpactReport{}, false, ErrRepositoryFailed
	}
	return report, true, nil
}

func (r *GORMRepository) SavePromotion(ctx context.Context, promotion Promotion) error {
	if !r.ready(ctx) || !validStoredPromotion(promotion) {
		return ErrRepositoryFailed
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing gormLifecyclePromotion
		err := tx.Where("promoted_fingerprint = ?", promotion.PromotedFingerprint).Take(&existing).Error
		if err == nil {
			if !samePromotion(existing.promotion(), promotion) {
				return ErrRepositoryFailed
			}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrRepositoryFailed
		}
		row := gormPromotionFromPromotion(promotion)
		if err := tx.Create(&row).Error; err != nil {
			return ErrRepositoryFailed
		}
		return nil
	})
}

func (r *GORMRepository) LoadPromotion(ctx context.Context, datasourceID, fingerprint string) (Promotion, bool, error) {
	datasourceID = normalizedLifecycleDatasource(datasourceID)
	if !r.ready(ctx) || datasourceID != DefaultDatasourceID || !canonicalDigest(fingerprint) {
		return Promotion{}, false, ErrRepositoryFailed
	}
	var row gormLifecyclePromotion
	err := r.db.WithContext(ctx).Where("promoted_fingerprint = ? AND datasource_id = ?", fingerprint, datasourceID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Promotion{}, false, nil
	}
	if err != nil {
		return Promotion{}, false, ErrRepositoryFailed
	}
	promotion := row.promotion()
	if !validStoredPromotion(promotion) || promotion.PromotedFingerprint != fingerprint {
		return Promotion{}, false, ErrRepositoryFailed
	}
	return promotion, true, nil
}

func lifecycleRepositoryModels() []any {
	return []any{&gormLifecycleLease{}, &gormLifecycleCheckpoint{}, &gormLifecycleImpactReport{}, &gormLifecyclePromotion{}}
}

func lifecycleRepositoryTableNames() []string {
	return []string{dataLifecycleLeasesTable, dataLifecycleCheckpointsTable, dataLifecycleImpactReportsTable, dataLifecyclePromotionsTable}
}

func (r *GORMRepository) ready(ctx context.Context) bool {
	return r != nil && r.db != nil && ctx != nil && ctx.Err() == nil
}

func normalizedLifecycleDatasource(datasourceID string) string {
	if datasourceID == "" {
		return DefaultDatasourceID
	}
	return datasourceID
}

func validLeaseRequest(request LeaseRequest) bool {
	return request.DatasourceID == DefaultDatasourceID && validStorageKey(request.Key) && validIdentifier(request.OwnerID) &&
		canonicalDigest(request.PolicyFingerprint) && !request.Now.IsZero() && request.TTL > 0
}

func validStoredLease(lease Lease) bool {
	return lease.DatasourceID == DefaultDatasourceID && validStorageKey(lease.Key) && validIdentifier(lease.OwnerID) &&
		validLeaseToken(lease.Token) && canonicalDigest(lease.PolicyFingerprint) && !lease.AcquiredAt.IsZero() &&
		!lease.HeartbeatAt.Before(lease.AcquiredAt) && lease.ExpiresAt.After(lease.HeartbeatAt)
}

func validStorageKey(value string) bool {
	trimmed := strings.TrimSpace(value)
	return trimmed != "" && trimmed == value && len(value) <= 128
}

func validLeaseToken(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func newLeaseToken() (string, error) {
	var token [32]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(token[:]), nil
}

func lifecycleLeaseID(datasourceID, key string) string {
	return stableDigest("lease-id", datasourceID, key)
}

func lifecycleCheckpointID(key CheckpointKey) string {
	return stableDigest("checkpoint-id", key.DatasourceID, key.RunID, string(key.Mode), key.PolicyFingerprint)
}

func validCheckpointKey(key CheckpointKey) bool {
	return key.DatasourceID == DefaultDatasourceID && validIdentifier(key.RunID) &&
		(key.Mode == ModeImpact || key.Mode == ModeDryRun || key.Mode == ModeApply) && canonicalDigest(key.PolicyFingerprint)
}

func validPersistedCheckpoint(checkpoint Checkpoint) bool {
	return validCheckpointKey(checkpoint.Key) && checkpoint.DatasourceID == DefaultDatasourceID &&
		validCheckpoint(checkpoint, checkpoint.Key) && checkpoint.Revision > 0 && canonicalDigest(checkpoint.LastBatchID) &&
		validLifecycleCursor(checkpoint.Cursor) && !checkpoint.UpdatedAt.IsZero()
}

func validateLeaseFence(tx *gorm.DB, lease Lease, now time.Time) error {
	lease.DatasourceID = normalizedLifecycleDatasource(lease.DatasourceID)
	if tx == nil || !validStoredLease(lease) {
		return ErrLeaseLost
	}
	var row gormLifecycleLease
	err := tx.Where("lease_id = ? AND datasource_id = ? AND lease_key = ? AND owner_id = ? AND token = ? AND policy_fingerprint = ? AND expires_at > ?",
		lifecycleLeaseID(lease.DatasourceID, lease.Key), lease.DatasourceID, lease.Key, lease.OwnerID, lease.Token, lease.PolicyFingerprint, now.UTC()).Take(&row).Error
	if err != nil {
		return ErrLeaseLost
	}
	return nil
}

func saveCheckpoint(tx *gorm.DB, checkpoint Checkpoint) error {
	id := lifecycleCheckpointID(checkpoint.Key)
	var existing gormLifecycleCheckpoint
	err := tx.Where("checkpoint_id = ?", id).Take(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if checkpoint.Revision != 1 {
			return ErrRepositoryFailed
		}
		row := gormCheckpointFromCheckpoint(checkpoint)
		if err := tx.Create(&row).Error; err != nil {
			return ErrRepositoryFailed
		}
		return nil
	}
	if err != nil {
		return ErrRepositoryFailed
	}
	current := existing.checkpoint()
	if !validPersistedCheckpoint(current) {
		return ErrRepositoryFailed
	}
	if current == checkpoint {
		return nil
	}
	if current.Complete || checkpoint.Revision != current.Revision+1 || checkpoint.UpdatedAt.Before(current.UpdatedAt) ||
		checkpoint.LastBatchID == current.LastBatchID || !checkpointProgresses(current, checkpoint) {
		return ErrRepositoryFailed
	}
	next := gormCheckpointFromCheckpoint(checkpoint)
	result := tx.Model(&gormLifecycleCheckpoint{}).
		Where("checkpoint_id = ? AND revision = ? AND integrity_hash = ?", id, current.Revision, current.IntegrityHash).
		Updates(checkpointUpdate(next))
	if result.Error != nil || result.RowsAffected != 1 {
		return ErrRepositoryFailed
	}
	return nil
}

func checkpointProgresses(current, next Checkpoint) bool {
	if current.Key != next.Key || current.DatasourceID != next.DatasourceID || current.Cursor.DatasourceID != next.Cursor.DatasourceID {
		return false
	}
	if next.Counts.Eligible < current.Counts.Eligible || next.Counts.Planned < current.Counts.Planned ||
		next.Counts.Applied < current.Counts.Applied || next.Counts.Batches < current.Counts.Batches || next.Counts.Retries < current.Counts.Retries {
		return false
	}
	return lifecycleCursorValue(next.Cursor) >= lifecycleCursorValue(current.Cursor)
}

func lifecycleCursorValue(cursor Cursor) string {
	return cursor.Resource + "\x00" + cursor.EligibleAt + "\x00" + cursor.RecordID
}

func validImpactCompletion(report ImpactReport, checkpoint Checkpoint) bool {
	return validPersistedCheckpoint(checkpoint) && checkpoint.Key.Mode == ModeImpact && checkpoint.Complete &&
		validStoredImpactReport(report) && report.DatasourceID == checkpoint.DatasourceID && report.RunID == checkpoint.Key.RunID &&
		report.PolicyFingerprint == checkpoint.Key.PolicyFingerprint && report.Counts == checkpoint.Counts &&
		report.Cursor == sanitizedCursor(checkpoint.Cursor) && report.EvidenceHash == checkpoint.EvidenceHash
}

func validStoredImpactReport(report ImpactReport) bool {
	return report.DatasourceID == DefaultDatasourceID && validIdentifier(report.RunID) && canonicalDigest(report.PolicyFingerprint) &&
		canonicalDigest(report.EvidenceHash) && canonicalDigest(report.ReportHash) && report.ReportHash == impactReportDigest(report) &&
		report.Counts.Eligible >= 0 && report.Counts.Planned >= 0 && report.Counts.Applied >= 0 && report.Counts.Batches >= 0 &&
		report.Counts.Retries >= 0 && validLifecycleCursor(report.Cursor) && !report.GeneratedAt.IsZero()
}

func validLifecycleCursor(cursor Cursor) bool {
	if cursor.DatasourceID != DefaultDatasourceID {
		return false
	}
	if cursor.Resource == "" && cursor.EligibleAt == "" && cursor.RecordID == "" {
		return true
	}
	return cursor.Resource != "" && cursor.Resource == strings.TrimSpace(cursor.Resource) && len(cursor.Resource) <= 128 &&
		cursor.EligibleAt != "" && cursor.EligibleAt == strings.TrimSpace(cursor.EligibleAt) && len(cursor.EligibleAt) <= 64 &&
		cursor.RecordID != "" && cursor.RecordID == strings.TrimSpace(cursor.RecordID) && len(cursor.RecordID) <= 191
}

func sameImpactReport(left, right ImpactReport) bool {
	left.GeneratedAt = time.Time{}
	right.GeneratedAt = time.Time{}
	return left == right
}

func validStoredPromotion(promotion Promotion) bool {
	return promotion.DatasourceID == DefaultDatasourceID && canonicalDigest(promotion.CurrentFingerprint) &&
		canonicalDigest(promotion.PromotedFingerprint) && canonicalDigest(promotion.ImpactReportHash) && validIdentifier(promotion.ActorID) &&
		promotion.Reason != "" && promotion.Reason == strings.TrimSpace(promotion.Reason) && len(promotion.Reason) <= 191 &&
		promotion.ApprovalRef != "" && promotion.ApprovalRef == strings.TrimSpace(promotion.ApprovalRef) && len(promotion.ApprovalRef) <= 191 &&
		!promotion.PromotedAt.IsZero()
}

func samePromotion(left, right Promotion) bool {
	left.PromotedAt = time.Time{}
	right.PromotedAt = time.Time{}
	return left == right
}

func gormLeaseFromLease(lease Lease) gormLifecycleLease {
	return gormLifecycleLease{
		LeaseID: lifecycleLeaseID(lease.DatasourceID, lease.Key), DatasourceID: lease.DatasourceID, LeaseKey: lease.Key,
		OwnerID: lease.OwnerID, Token: lease.Token, PolicyFingerprint: lease.PolicyFingerprint,
		AcquiredAt: lease.AcquiredAt, HeartbeatAt: lease.HeartbeatAt, ExpiresAt: lease.ExpiresAt,
	}
}

func gormLeaseUpdate(lease Lease) map[string]any {
	return map[string]any{
		"datasource_id": lease.DatasourceID, "lease_key": lease.Key, "owner_id": lease.OwnerID, "token": lease.Token,
		"policy_fingerprint": lease.PolicyFingerprint, "acquired_at": lease.AcquiredAt,
		"heartbeat_at": lease.HeartbeatAt, "expires_at": lease.ExpiresAt,
	}
}

func gormCheckpointFromCheckpoint(checkpoint Checkpoint) gormLifecycleCheckpoint {
	return gormLifecycleCheckpoint{
		CheckpointID: lifecycleCheckpointID(checkpoint.Key), DatasourceID: checkpoint.Key.DatasourceID,
		RunID: checkpoint.Key.RunID, Mode: string(checkpoint.Key.Mode), PolicyFingerprint: checkpoint.Key.PolicyFingerprint,
		CursorDatasource: checkpoint.Cursor.DatasourceID, CursorResource: checkpoint.Cursor.Resource,
		CursorEligibleAt: checkpoint.Cursor.EligibleAt, CursorRecordID: checkpoint.Cursor.RecordID,
		Eligible: checkpoint.Counts.Eligible, Planned: checkpoint.Counts.Planned, Applied: checkpoint.Counts.Applied,
		Batches: checkpoint.Counts.Batches, Retries: checkpoint.Counts.Retries,
		EvidenceHash: checkpoint.EvidenceHash, IntegrityHash: checkpoint.IntegrityHash, LastBatchID: checkpoint.LastBatchID,
		Revision: checkpoint.Revision, Complete: checkpoint.Complete, UpdatedAt: checkpoint.UpdatedAt,
	}
}

func checkpointUpdate(row gormLifecycleCheckpoint) map[string]any {
	return map[string]any{
		"cursor_datasource_id": row.CursorDatasource, "cursor_resource": row.CursorResource,
		"cursor_eligible_at": row.CursorEligibleAt, "cursor_record_id": row.CursorRecordID,
		"eligible_count": row.Eligible, "planned_count": row.Planned, "applied_count": row.Applied,
		"batch_count": row.Batches, "retry_count": row.Retries, "evidence_hash": row.EvidenceHash,
		"integrity_hash": row.IntegrityHash, "last_batch_id": row.LastBatchID, "revision": row.Revision,
		"complete": row.Complete, "updated_at": row.UpdatedAt,
	}
}

func (row gormLifecycleCheckpoint) checkpoint() Checkpoint {
	return Checkpoint{
		Key:          CheckpointKey{DatasourceID: row.DatasourceID, RunID: row.RunID, Mode: Mode(row.Mode), PolicyFingerprint: row.PolicyFingerprint},
		DatasourceID: row.DatasourceID,
		Cursor:       Cursor{DatasourceID: row.CursorDatasource, Resource: row.CursorResource, EligibleAt: row.CursorEligibleAt, RecordID: row.CursorRecordID},
		Counts:       Counts{Eligible: row.Eligible, Planned: row.Planned, Applied: row.Applied, Batches: row.Batches, Retries: row.Retries},
		EvidenceHash: row.EvidenceHash, IntegrityHash: row.IntegrityHash, LastBatchID: row.LastBatchID,
		Revision: row.Revision, Complete: row.Complete, UpdatedAt: row.UpdatedAt,
	}
}

func gormImpactReportFromReport(report ImpactReport) gormLifecycleImpactReport {
	return gormLifecycleImpactReport{
		ReportHash: report.ReportHash, DatasourceID: report.DatasourceID, RunID: report.RunID,
		PolicyFingerprint: report.PolicyFingerprint, Eligible: report.Counts.Eligible, Planned: report.Counts.Planned,
		Applied: report.Counts.Applied, Batches: report.Counts.Batches, Retries: report.Counts.Retries,
		CursorDatasource: report.Cursor.DatasourceID, CursorResource: report.Cursor.Resource,
		CursorEligibleAt: report.Cursor.EligibleAt, CursorRecordID: report.Cursor.RecordID,
		EvidenceHash: report.EvidenceHash, GeneratedAt: report.GeneratedAt,
	}
}

func (row gormLifecycleImpactReport) impactReport() ImpactReport {
	return ImpactReport{
		DatasourceID: row.DatasourceID, RunID: row.RunID, PolicyFingerprint: row.PolicyFingerprint,
		Counts:       Counts{Eligible: row.Eligible, Planned: row.Planned, Applied: row.Applied, Batches: row.Batches, Retries: row.Retries},
		Cursor:       Cursor{DatasourceID: row.CursorDatasource, Resource: row.CursorResource, EligibleAt: row.CursorEligibleAt, RecordID: row.CursorRecordID},
		EvidenceHash: row.EvidenceHash, ReportHash: row.ReportHash, GeneratedAt: row.GeneratedAt,
	}
}

func gormPromotionFromPromotion(promotion Promotion) gormLifecyclePromotion {
	return gormLifecyclePromotion{
		PromotedFingerprint: promotion.PromotedFingerprint, DatasourceID: promotion.DatasourceID,
		CurrentFingerprint: promotion.CurrentFingerprint, ImpactReportHash: promotion.ImpactReportHash,
		ActorID: promotion.ActorID, Reason: promotion.Reason, ApprovalRef: promotion.ApprovalRef, PromotedAt: promotion.PromotedAt,
	}
}

func (row gormLifecyclePromotion) promotion() Promotion {
	return Promotion{
		DatasourceID: row.DatasourceID, CurrentFingerprint: row.CurrentFingerprint,
		PromotedFingerprint: row.PromotedFingerprint, ImpactReportHash: row.ImpactReportHash,
		ActorID: row.ActorID, Reason: row.Reason, ApprovalRef: row.ApprovalRef, PromotedAt: row.PromotedAt,
	}
}
