package xa

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

type gormRecoveryRecord struct {
	ID                   uint64    `gorm:"primaryKey;autoIncrement"`
	TransactionID        string    `gorm:"size:64;not null;index"`
	Operation            string    `gorm:"size:64;not null"`
	Status               string    `gorm:"size:32;not null"`
	ResourceManagersJSON string    `gorm:"type:text;not null"`
	OccurredAt           time.Time `gorm:"not null"`
}

func (gormRecoveryRecord) TableName() string { return "platform_xa_recovery_records" }

type GORMRecoveryLog struct{ db *gorm.DB }

func PrepareGORMRecoveryLog(ctx context.Context, db *gorm.DB) error {
	if ctx == nil || db == nil {
		return ErrInvalidConfig
	}
	return db.WithContext(ctx).AutoMigrate(&gormRecoveryRecord{})
}

func OpenGORMRecoveryLog(ctx context.Context, db *gorm.DB) (*GORMRecoveryLog, error) {
	if ctx == nil || db == nil || !db.WithContext(ctx).Migrator().HasTable(&gormRecoveryRecord{}) {
		return nil, ErrInvalidConfig
	}
	return &GORMRecoveryLog{db: db}, nil
}

func (l *GORMRecoveryLog) Save(ctx context.Context, record RecoveryRecord) error {
	if l == nil || l.db == nil || ctx == nil || !validRecord(record) {
		return ErrInvalidConfig
	}
	managers, err := json.Marshal(record.ResourceManagers)
	if err != nil {
		return ErrInvalidConfig
	}
	row := gormRecoveryRecord{TransactionID: record.TransactionID, Operation: record.Operation, Status: string(record.Status), ResourceManagersJSON: string(managers), OccurredAt: record.OccurredAt.UTC()}
	if err := l.db.WithContext(ctx).Create(&row).Error; err != nil {
		return fmt.Errorf("save xa recovery record: %w", err)
	}
	return nil
}

func (l *GORMRecoveryLog) Records(ctx context.Context) ([]RecoveryRecord, error) {
	if l == nil || l.db == nil || ctx == nil {
		return nil, ErrInvalidConfig
	}
	var rows []gormRecoveryRecord
	if err := l.db.WithContext(ctx).Order("id ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list xa recovery records: %w", err)
	}
	records := make([]RecoveryRecord, 0, len(rows))
	for _, row := range rows {
		var managers []string
		if err := json.Unmarshal([]byte(row.ResourceManagersJSON), &managers); err != nil {
			return nil, ErrInvalidConfig
		}
		records = append(records, RecoveryRecord{TransactionID: row.TransactionID, Operation: row.Operation, Status: Status(row.Status), ResourceManagers: managers, OccurredAt: row.OccurredAt})
	}
	return records, nil
}

func validRecord(record RecoveryRecord) bool {
	if !strings.HasPrefix(record.TransactionID, "xa_") || len(record.TransactionID) != 35 || !identifierPattern.MatchString(record.Operation) || len(record.ResourceManagers) < 2 || record.OccurredAt.IsZero() || (record.Status != StatusCommitted && record.Status != StatusRolledBack && record.Status != StatusHeuristic) {
		return false
	}
	seen := map[string]struct{}{}
	for _, manager := range record.ResourceManagers {
		if !identifierPattern.MatchString(manager) {
			return false
		}
		if _, found := seen[manager]; found {
			return false
		}
		seen[manager] = struct{}{}
	}
	return true
}
