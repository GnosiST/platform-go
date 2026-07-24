package sharding

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/GnosiST/platform-go/internal/platform/kernel"
	"gorm.io/gorm"
)

type gormMigration struct {
	ID             string `gorm:"primaryKey;size:64;not null"`
	GlobalID       string `gorm:"uniqueIndex;size:40;not null"`
	TenantCode     string `gorm:"size:64;not null"`
	CapabilityID   string `gorm:"size:64;not null"`
	SourceShardID  string `gorm:"size:64;not null"`
	TargetShardID  string `gorm:"size:64;not null"`
	RoutingVersion uint64 `gorm:"not null"`
	Sequence       uint64 `gorm:"not null"`
	Cursor         string `gorm:"size:256;not null"`
	Status         string `gorm:"size:16;not null"`
	Reason         string `gorm:"size:256;not null"`
}

func (gormMigration) TableName() string { return "platform_shard_migrations" }

type gormMigrationAudit struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement"`
	Kind        string    `gorm:"size:16;not null"`
	MigrationID string    `gorm:"size:64;not null"`
	Actor       string    `gorm:"size:128;not null"`
	Sequence    uint64    `gorm:"not null"`
	Cursor      string    `gorm:"size:256;not null"`
	Status      string    `gorm:"size:16;not null"`
	Reason      string    `gorm:"size:256;not null"`
	OccurredAt  time.Time `gorm:"not null"`
}

func (gormMigrationAudit) TableName() string { return "platform_shard_migration_audits" }

// GORMController persists the same migration state machine as Controller.
type GORMController struct {
	db         *gorm.DB
	authorizer Authorizer
	now        func() time.Time
}

func PrepareGORMController(ctx context.Context, db *gorm.DB) error {
	if ctx == nil || db == nil {
		return ErrInvalidMigration
	}
	return db.WithContext(ctx).AutoMigrate(&gormMigration{}, &gormMigrationAudit{})
}

func OpenGORMController(ctx context.Context, db *gorm.DB, authorizer Authorizer, now func() time.Time) (*GORMController, error) {
	if ctx == nil || db == nil || authorizer == nil || now == nil || !db.WithContext(ctx).Migrator().HasTable(&gormMigration{}) || !db.WithContext(ctx).Migrator().HasTable(&gormMigrationAudit{}) {
		return nil, ErrInvalidMigration
	}
	return &GORMController{db: db, authorizer: authorizer, now: now}, nil
}

func (c *GORMController) Create(execution kernel.ExecutionContext, migration Migration) (Migration, error) {
	if c == nil || c.db == nil || c.authorizer == nil || !authorized(execution, migration.TenantCode, c.authorizer) {
		return Migration{}, ErrUnauthorized
	}
	if err := validateNewMigration(migration); err != nil {
		return Migration{}, err
	}
	globalID, err := newGlobalID()
	if err != nil {
		return Migration{}, err
	}
	migration.GlobalID = globalID
	err = c.db.WithContext(execution.BaseContext()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(toGORMMigration(migration)).Error; err != nil {
			return err
		}
		return tx.Create(toGORMAudit(AuditEntry{Kind: "create", MigrationID: migration.ID, Actor: execution.Actor.Username, Checkpoint: migration.Checkpoint, Status: migration.Status, Reason: migration.Reason, OccurredAt: c.now().UTC()})).Error
	})
	if err != nil {
		return Migration{}, fmt.Errorf("create shard migration: %w", err)
	}
	return migration, nil
}

func (c *GORMController) Checkpoint(execution kernel.ExecutionContext, migrationID string, checkpoint Checkpoint) (Migration, error) {
	if c == nil || c.db == nil || !validIdentifier(migrationID) || !validCheckpoint(checkpoint) {
		return Migration{}, ErrInvalidMigration
	}
	var updated Migration
	err := c.db.WithContext(execution.BaseContext()).Transaction(func(tx *gorm.DB) error {
		var row gormMigration
		if err := tx.Where("id = ?", migrationID).Take(&row).Error; err != nil {
			return err
		}
		migration := fromGORMMigration(row)
		if !authorized(execution, migration.TenantCode, c.authorizer) {
			return ErrUnauthorized
		}
		if (migration.Status != StatusPrepared && migration.Status != StatusRunning) || checkpoint.Sequence <= migration.Checkpoint.Sequence {
			return ErrInvalidMigration
		}
		result := tx.Model(&gormMigration{}).Where("id = ? AND sequence = ?", migrationID, migration.Checkpoint.Sequence).Updates(map[string]any{"sequence": checkpoint.Sequence, "cursor": checkpoint.Cursor, "status": string(StatusRunning)})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrInvalidMigration
		}
		migration.Checkpoint = checkpoint
		migration.Status = StatusRunning
		updated = migration
		return tx.Create(toGORMAudit(AuditEntry{Kind: "checkpoint", MigrationID: migration.ID, Actor: execution.Actor.Username, Checkpoint: checkpoint, Status: migration.Status, Reason: migration.Reason, OccurredAt: c.now().UTC()})).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Migration{}, ErrMigrationNotFound
	}
	if err != nil {
		return Migration{}, err
	}
	return updated, nil
}

func (c *GORMController) Compensate(execution kernel.ExecutionContext, migrationID, reason string) (Migration, error) {
	if c == nil || c.db == nil || !validIdentifier(migrationID) || !validReason(reason) {
		return Migration{}, ErrInvalidMigration
	}
	var updated Migration
	err := c.db.WithContext(execution.BaseContext()).Transaction(func(tx *gorm.DB) error {
		var row gormMigration
		if err := tx.Where("id = ?", migrationID).Take(&row).Error; err != nil {
			return err
		}
		migration := fromGORMMigration(row)
		if !authorized(execution, migration.TenantCode, c.authorizer) {
			return ErrUnauthorized
		}
		if migration.Status != StatusPrepared && migration.Status != StatusRunning {
			return ErrInvalidMigration
		}
		result := tx.Model(&gormMigration{}).Where("id = ? AND status IN ?", migrationID, []string{string(StatusPrepared), string(StatusRunning)}).Update("status", string(StatusCompensated))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrInvalidMigration
		}
		migration.Status = StatusCompensated
		updated = migration
		return tx.Create(toGORMAudit(AuditEntry{Kind: "compensate", MigrationID: migration.ID, Actor: execution.Actor.Username, Checkpoint: migration.Checkpoint, Status: migration.Status, Reason: reason, OccurredAt: c.now().UTC()})).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Migration{}, ErrMigrationNotFound
	}
	if err != nil {
		return Migration{}, err
	}
	return updated, nil
}

func (c *GORMController) Audit(ctx context.Context) ([]AuditEntry, error) {
	if c == nil || c.db == nil || ctx == nil {
		return nil, ErrInvalidMigration
	}
	var rows []gormMigrationAudit
	if err := c.db.WithContext(ctx).Order("id ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list shard migration audit: %w", err)
	}
	entries := make([]AuditEntry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, AuditEntry{Kind: row.Kind, MigrationID: row.MigrationID, Actor: row.Actor, Checkpoint: Checkpoint{Sequence: row.Sequence, Cursor: row.Cursor}, Status: Status(row.Status), Reason: row.Reason, OccurredAt: row.OccurredAt})
	}
	return entries, nil
}

func toGORMMigration(migration Migration) *gormMigration {
	return &gormMigration{ID: migration.ID, GlobalID: migration.GlobalID, TenantCode: migration.TenantCode, CapabilityID: migration.CapabilityID, SourceShardID: migration.SourceShardID, TargetShardID: migration.TargetShardID, RoutingVersion: migration.RoutingVersion, Sequence: migration.Checkpoint.Sequence, Cursor: migration.Checkpoint.Cursor, Status: string(migration.Status), Reason: migration.Reason}
}

func fromGORMMigration(row gormMigration) Migration {
	return Migration{ID: row.ID, GlobalID: row.GlobalID, TenantCode: row.TenantCode, CapabilityID: row.CapabilityID, SourceShardID: row.SourceShardID, TargetShardID: row.TargetShardID, RoutingVersion: row.RoutingVersion, Checkpoint: Checkpoint{Sequence: row.Sequence, Cursor: row.Cursor}, Status: Status(row.Status), Reason: row.Reason}
}

func toGORMAudit(entry AuditEntry) *gormMigrationAudit {
	return &gormMigrationAudit{Kind: entry.Kind, MigrationID: entry.MigrationID, Actor: entry.Actor, Sequence: entry.Checkpoint.Sequence, Cursor: entry.Checkpoint.Cursor, Status: string(entry.Status), Reason: entry.Reason, OccurredAt: entry.OccurredAt}
}
