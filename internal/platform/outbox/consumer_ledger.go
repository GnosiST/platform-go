package outbox

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ConsumerLedger records completed consumers in the same transaction as the
// consumer's local projection, making replayed outbox deliveries idempotent.
type ConsumerLedger struct {
	db  *gorm.DB
	now func() time.Time
}

type gormConsumerReceipt struct {
	ConsumerID   string    `gorm:"primaryKey;size:140"`
	EventID      string    `gorm:"primaryKey;size:140"`
	EventType    string    `gorm:"size:140;not null"`
	EventVersion string    `gorm:"size:16;not null"`
	TenantCode   string    `gorm:"index;size:64;not null"`
	ProcessedAt  time.Time `gorm:"not null"`
	CreatedAt    time.Time
}

func (gormConsumerReceipt) TableName() string { return "platform_outbox_consumer_receipts" }

func OpenConsumerLedger(ctx context.Context, db *gorm.DB, now func() time.Time) (*ConsumerLedger, error) {
	if ctx == nil || db == nil || now == nil || !db.WithContext(ctx).Migrator().HasTable(&gormConsumerReceipt{}) {
		return nil, ErrInvalidEnvelope
	}
	return &ConsumerLedger{db: db, now: now}, nil
}

// Apply executes a consumer's local mutation and its receipt in one database
// transaction. A previously completed event is skipped; a failed apply rolls
// the receipt back so the broker can retry it.
func (l *ConsumerLedger) Apply(ctx context.Context, consumerID string, envelope Envelope, apply func(*gorm.DB) error) (bool, error) {
	if l == nil || l.db == nil || ctx == nil || apply == nil || !eventIdentifierPattern.MatchString(strings.TrimSpace(consumerID)) {
		return false, ErrInvalidEnvelope
	}
	validated, err := ValidateEnvelope(envelope)
	if err != nil {
		return false, err
	}
	processed := false
	err = l.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := l.now().UTC()
		receipt := gormConsumerReceipt{
			ConsumerID: strings.TrimSpace(consumerID), EventID: validated.EventID, EventType: validated.EventType,
			EventVersion: validated.Version, TenantCode: validated.TenantCode, ProcessedAt: now,
		}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&receipt)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		if err := apply(tx); err != nil {
			return err
		}
		processed = true
		return nil
	})
	return processed, err
}
