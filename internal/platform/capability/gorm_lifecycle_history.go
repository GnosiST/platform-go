package capability

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GORMLifecycleHistory struct {
	db *gorm.DB
}

type gormLifecycleRecord struct {
	Kind         string `gorm:"primaryKey;size:64;column:kind"`
	CapabilityID string `gorm:"primaryKey;size:128;column:capability_id"`
	StepID       string `gorm:"primaryKey;size:256;column:step_id"`
	Description  string `gorm:"not null;column:description"`
}

func (gormLifecycleRecord) TableName() string {
	return lifecycleHistoryTable
}

func NewGORMLifecycleHistory(ctx context.Context, db *gorm.DB) (*GORMLifecycleHistory, error) {
	history := &GORMLifecycleHistory{db: db}
	if err := db.WithContext(ctx).AutoMigrate(&gormLifecycleRecord{}); err != nil {
		return nil, err
	}
	return history, nil
}

func (h *GORMLifecycleHistory) HasMigration(ctx context.Context, capabilityID ID, migrationID string) bool {
	return h.has(ctx, LifecycleKindMigration, capabilityID, migrationID)
}

func (h *GORMLifecycleHistory) RecordMigration(ctx context.Context, record LifecycleRecord) error {
	record.Kind = LifecycleKindMigration
	return h.record(ctx, record)
}

func (h *GORMLifecycleHistory) HasSeed(ctx context.Context, capabilityID ID, seedID string) bool {
	return h.has(ctx, LifecycleKindSeed, capabilityID, seedID)
}

func (h *GORMLifecycleHistory) RecordSeed(ctx context.Context, record LifecycleRecord) error {
	record.Kind = LifecycleKindSeed
	return h.record(ctx, record)
}

func (h *GORMLifecycleHistory) Records(ctx context.Context) []LifecycleRecord {
	var records []gormLifecycleRecord
	if err := h.db.WithContext(ctx).Order("kind ASC, capability_id ASC, step_id ASC").Find(&records).Error; err != nil {
		return nil
	}
	lifecycleRecords := make([]LifecycleRecord, 0, len(records))
	for _, record := range records {
		lifecycleRecords = append(lifecycleRecords, record.toLifecycleRecord())
	}
	return lifecycleRecords
}

func (h *GORMLifecycleHistory) has(ctx context.Context, kind LifecycleKind, capabilityID ID, stepID string) bool {
	var count int64
	err := h.db.WithContext(ctx).Model(&gormLifecycleRecord{}).
		Where("kind = ? AND capability_id = ? AND step_id = ?", string(kind), string(capabilityID), stepID).
		Count(&count).Error
	return err == nil && count > 0
}

func (h *GORMLifecycleHistory) record(ctx context.Context, record LifecycleRecord) error {
	return h.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(gormLifecycleRecordFromRecord(record)).
		Error
}

func gormLifecycleRecordFromRecord(record LifecycleRecord) *gormLifecycleRecord {
	return &gormLifecycleRecord{
		Kind:         string(record.Kind),
		CapabilityID: string(record.CapabilityID),
		StepID:       record.StepID,
		Description:  record.Description,
	}
}

func (r gormLifecycleRecord) toLifecycleRecord() LifecycleRecord {
	return LifecycleRecord{
		Kind:         LifecycleKind(r.Kind),
		CapabilityID: ID(r.CapabilityID),
		StepID:       r.StepID,
		Description:  r.Description,
	}
}
