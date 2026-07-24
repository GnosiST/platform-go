package outbox

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

type Status string

const (
	StatusPending    Status = "pending"
	StatusRetry      Status = "retry"
	StatusLeased     Status = "leased"
	StatusPublished  Status = "published"
	StatusDeadLetter Status = "dead-letter"

	SafeDeliveryError = "outbox delivery failed"
)

var (
	ErrRecordNotFound = errors.New("outbox record not found")
	ErrInvalidLease   = errors.New("invalid outbox lease")
	ErrNotReplayable  = errors.New("outbox record is not replayable")
)

type Record struct {
	ID          string
	Envelope    Envelope
	Status      Status
	Attempts    int
	AvailableAt time.Time
	LeaseOwner  string
	LeaseUntil  time.Time
	PublishedAt time.Time
	ReplayOf    string
	LastError   string
	CreatedAt   time.Time
}

type AuditEntry struct {
	RecordID   string
	Kind       string
	Actor      string
	OccurredAt time.Time
}

type gormRecord struct {
	ID          string    `gorm:"primaryKey;size:40"`
	EventID     string    `gorm:"index;size:140;not null"`
	EventType   string    `gorm:"index;size:140;not null"`
	Version     string    `gorm:"size:16;not null"`
	OccurredAt  time.Time `gorm:"not null"`
	Producer    string    `gorm:"size:140;not null"`
	TenantCode  string    `gorm:"index;size:64;not null"`
	TraceParent string    `gorm:"size:64;not null"`
	TraceState  string    `gorm:"size:512"`
	Payload     []byte    `gorm:"not null"`
	Status      string    `gorm:"index;size:16;not null"`
	Attempts    int       `gorm:"not null"`
	AvailableAt time.Time `gorm:"index;not null"`
	LeaseOwner  string    `gorm:"size:128"`
	LeaseUntil  *time.Time
	PublishedAt *time.Time
	ReplayOf    string `gorm:"index;size:40"`
	LastError   string `gorm:"size:128"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (gormRecord) TableName() string { return "platform_outbox_records" }

type gormAudit struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement"`
	RecordID   string    `gorm:"index;size:40;not null"`
	Kind       string    `gorm:"size:32;not null"`
	Actor      string    `gorm:"size:128;not null"`
	OccurredAt time.Time `gorm:"not null"`
}

func (gormAudit) TableName() string { return "platform_outbox_audits" }

type GORMStore struct {
	db  *gorm.DB
	now func() time.Time
}

func PrepareGORMStore(ctx context.Context, db *gorm.DB) error {
	if ctx == nil || db == nil {
		return ErrInvalidEnvelope
	}
	return db.WithContext(ctx).AutoMigrate(&gormRecord{}, &gormAudit{}, &gormConsumerReceipt{}, &gormSagaRun{}, &gormSagaStep{})
}

func OpenGORMStore(ctx context.Context, db *gorm.DB, now func() time.Time) (*GORMStore, error) {
	if ctx == nil || db == nil || now == nil || !db.WithContext(ctx).Migrator().HasTable(&gormRecord{}) || !db.WithContext(ctx).Migrator().HasTable(&gormAudit{}) || !db.WithContext(ctx).Migrator().HasTable(&gormConsumerReceipt{}) {
		return nil, ErrInvalidEnvelope
	}
	return &GORMStore{db: db, now: now}, nil
}

func (s *GORMStore) Append(ctx context.Context, envelope Envelope) (Record, error) {
	if s == nil || s.db == nil || ctx == nil {
		return Record{}, ErrInvalidEnvelope
	}
	var record Record
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		created, err := s.AppendInTransaction(tx, envelope)
		record = created
		return err
	})
	return record, err
}

// AppendInTransaction lets a business mutation and its event share the exact caller-owned transaction.
func (s *GORMStore) AppendInTransaction(tx *gorm.DB, envelope Envelope) (Record, error) {
	if s == nil || tx == nil {
		return Record{}, ErrInvalidEnvelope
	}
	validated, err := ValidateEnvelope(envelope)
	if err != nil {
		return Record{}, err
	}
	id, err := newRecordID()
	if err != nil {
		return Record{}, err
	}
	now := s.now().UTC()
	row := gormRecord{ID: id, EventID: validated.EventID, EventType: validated.EventType, Version: validated.Version, OccurredAt: validated.OccurredAt, Producer: validated.Producer, TenantCode: validated.TenantCode, TraceParent: validated.TraceParent, TraceState: validated.TraceState, Payload: validated.Payload, Status: string(StatusPending), AvailableAt: now}
	if err := tx.Create(&row).Error; err != nil {
		return Record{}, fmt.Errorf("append outbox record: %w", err)
	}
	if err := tx.Create(&gormAudit{RecordID: id, Kind: "appended", Actor: "transaction", OccurredAt: now}).Error; err != nil {
		return Record{}, fmt.Errorf("audit outbox append: %w", err)
	}
	return fromGORMRecord(row), nil
}

func (s *GORMStore) List(ctx context.Context) ([]Record, error) {
	if s == nil || s.db == nil || ctx == nil {
		return nil, ErrInvalidEnvelope
	}
	var rows []gormRecord
	if err := s.db.WithContext(ctx).Order("created_at ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return recordsFromRows(rows), nil
}

func (s *GORMStore) Get(ctx context.Context, id string) (Record, error) {
	if s == nil || s.db == nil || ctx == nil {
		return Record{}, ErrRecordNotFound
	}
	var row gormRecord
	err := s.db.WithContext(ctx).Where("id = ?", strings.TrimSpace(id)).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Record{}, ErrRecordNotFound
	}
	if err != nil {
		return Record{}, err
	}
	return fromGORMRecord(row), nil
}

func (s *GORMStore) Claim(ctx context.Context, owner string, limit int, leaseDuration time.Duration) ([]Record, error) {
	if s == nil || s.db == nil || ctx == nil || !eventIdentifierPattern.MatchString(strings.TrimSpace(owner)) || limit <= 0 || leaseDuration <= 0 {
		return nil, ErrInvalidLease
	}
	now := s.now().UTC()
	leaseUntil := now.Add(leaseDuration)
	claimed := make([]Record, 0, limit)
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var candidates []gormRecord
		if err := tx.Where("(status IN ? AND available_at <= ?) OR (status = ? AND lease_until <= ?)", []string{string(StatusPending), string(StatusRetry)}, now, string(StatusLeased), now).Order("available_at ASC, id ASC").Limit(limit).Find(&candidates).Error; err != nil {
			return err
		}
		for _, candidate := range candidates {
			result := tx.Model(&gormRecord{}).Where("id = ? AND ((status IN ? AND available_at <= ?) OR (status = ? AND lease_until <= ?))", candidate.ID, []string{string(StatusPending), string(StatusRetry)}, now, string(StatusLeased), now).Updates(map[string]any{"status": string(StatusLeased), "lease_owner": owner, "lease_until": leaseUntil})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				continue
			}
			candidate.Status, candidate.LeaseOwner, candidate.LeaseUntil = string(StatusLeased), owner, &leaseUntil
			claimed = append(claimed, fromGORMRecord(candidate))
			if err := tx.Create(&gormAudit{RecordID: candidate.ID, Kind: "leased", Actor: owner, OccurredAt: now}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	return claimed, err
}

func (s *GORMStore) MarkPublished(ctx context.Context, id, owner string) error {
	return s.completeLease(ctx, id, owner, func(tx *gorm.DB, row gormRecord, now time.Time) error {
		result := tx.Model(&gormRecord{}).Where("id = ? AND status = ? AND lease_owner = ?", row.ID, string(StatusLeased), owner).Updates(map[string]any{"status": string(StatusPublished), "published_at": now, "lease_owner": "", "lease_until": nil, "last_error": ""})
		if result.Error != nil || result.RowsAffected != 1 {
			return ErrInvalidLease
		}
		return tx.Create(&gormAudit{RecordID: row.ID, Kind: "published", Actor: owner, OccurredAt: now}).Error
	})
}

func (s *GORMStore) MarkFailure(ctx context.Context, id, owner string, maxAttempts int, retryDelay time.Duration) (Status, error) {
	if maxAttempts <= 0 || retryDelay <= 0 {
		return "", ErrInvalidLease
	}
	var status Status
	err := s.completeLease(ctx, id, owner, func(tx *gorm.DB, row gormRecord, now time.Time) error {
		attempts := row.Attempts + 1
		status = StatusRetry
		updates := map[string]any{"attempts": attempts, "status": string(StatusRetry), "available_at": now.Add(retryDelay), "lease_owner": "", "lease_until": nil, "last_error": SafeDeliveryError}
		kind := "retried"
		if attempts >= maxAttempts {
			status, kind = StatusDeadLetter, "dead-lettered"
			updates["status"] = string(status)
			updates["available_at"] = now
		}
		result := tx.Model(&gormRecord{}).Where("id = ? AND status = ? AND lease_owner = ?", row.ID, string(StatusLeased), owner).Updates(updates)
		if result.Error != nil || result.RowsAffected != 1 {
			return ErrInvalidLease
		}
		return tx.Create(&gormAudit{RecordID: row.ID, Kind: kind, Actor: owner, OccurredAt: now}).Error
	})
	return status, err
}

func (s *GORMStore) Replay(ctx context.Context, id, actor string) (Record, error) {
	if s == nil || s.db == nil || ctx == nil || !eventIdentifierPattern.MatchString(strings.TrimSpace(actor)) {
		return Record{}, ErrNotReplayable
	}
	var replay Record
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var original gormRecord
		if err := tx.Where("id = ?", strings.TrimSpace(id)).Take(&original).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrRecordNotFound
			}
			return err
		}
		if Status(original.Status) != StatusDeadLetter {
			return ErrNotReplayable
		}
		newID, err := newRecordID()
		if err != nil {
			return err
		}
		now := s.now().UTC()
		row := original
		row.ID, row.Status, row.Attempts, row.AvailableAt, row.LeaseOwner, row.LeaseUntil, row.PublishedAt, row.ReplayOf, row.LastError, row.CreatedAt, row.UpdatedAt = newID, string(StatusPending), 0, now, "", nil, nil, original.ID, "", now, now
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		if err := tx.Create(&gormAudit{RecordID: newID, Kind: "replayed", Actor: actor, OccurredAt: now}).Error; err != nil {
			return err
		}
		replay = fromGORMRecord(row)
		return nil
	})
	return replay, err
}

func (s *GORMStore) completeLease(ctx context.Context, id, owner string, operation func(*gorm.DB, gormRecord, time.Time) error) error {
	if s == nil || s.db == nil || ctx == nil || !eventIdentifierPattern.MatchString(strings.TrimSpace(owner)) {
		return ErrInvalidLease
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row gormRecord
		if err := tx.Where("id = ? AND status = ? AND lease_owner = ?", strings.TrimSpace(id), string(StatusLeased), owner).Take(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrInvalidLease
			}
			return err
		}
		return operation(tx, row, s.now().UTC())
	})
}

func newRecordID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return "out-" + hex.EncodeToString(value), nil
}

func fromGORMRecord(row gormRecord) Record {
	record := Record{ID: row.ID, Envelope: Envelope{EventID: row.EventID, EventType: row.EventType, Version: row.Version, OccurredAt: row.OccurredAt.UTC(), Producer: row.Producer, TenantCode: row.TenantCode, TraceParent: row.TraceParent, TraceState: row.TraceState, Payload: append([]byte(nil), row.Payload...)}, Status: Status(row.Status), Attempts: row.Attempts, AvailableAt: row.AvailableAt.UTC(), LeaseOwner: row.LeaseOwner, ReplayOf: row.ReplayOf, LastError: row.LastError, CreatedAt: row.CreatedAt.UTC()}
	if row.LeaseUntil != nil {
		record.LeaseUntil = row.LeaseUntil.UTC()
	}
	if row.PublishedAt != nil {
		record.PublishedAt = row.PublishedAt.UTC()
	}
	return record
}

func recordsFromRows(rows []gormRecord) []Record {
	records := make([]Record, 0, len(rows))
	for _, row := range rows {
		records = append(records, fromGORMRecord(row))
	}
	return records
}
