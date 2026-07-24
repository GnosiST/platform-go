// Package searchprojection applies registered outbox events to an optional
// search adapter. It has no HTTP surface and never treats search as a source
// of truth.
package searchprojection

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/GnosiST/platform-go/internal/platform/integration"
	"github.com/GnosiST/platform-go/internal/platform/outbox"
	"gorm.io/gorm"
)

var (
	ErrInvalidMapping = errors.New("invalid search projection mapping")
	fieldPattern      = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,63}$`)
	namePattern       = regexp.MustCompile(`^[a-z][a-z0-9._-]{0,127}$`)
	documentIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,139}$`)
)

const maxRebuildDocuments = 1000

// Mapping is registered by a composition root. Its allowlisted fields are the
// only payload values that may enter a search document.
type Mapping struct {
	ID              string
	ConsumerID      string
	EventType       string
	Collection      string
	DocumentIDField string
	DeletedField    string
	Fields          []string
}

type RebuildOptions struct {
	Limit int
}

type RebuildResult struct {
	Indexed int
	Deleted int
}

type GORMStore struct {
	db     *gorm.DB
	now    func() time.Time
	ledger *outbox.ConsumerLedger
}

type gormDocument struct {
	MappingID  string `gorm:"primaryKey;size:140"`
	TenantCode string `gorm:"primaryKey;size:64"`
	DocumentID string `gorm:"primaryKey;size:140"`
	Collection string `gorm:"index;size:140;not null"`
	Body       []byte `gorm:"not null"`
	DeletedAt  *time.Time
	CreatedAt  time.Time `gorm:"not null"`
	UpdatedAt  time.Time `gorm:"not null"`
}

func (gormDocument) TableName() string { return "platform_search_projection_documents" }

type gormCheckpoint struct {
	MappingID   string    `gorm:"primaryKey;size:140"`
	ConsumerID  string    `gorm:"size:140;not null"`
	LastEventID string    `gorm:"size:140;not null"`
	EventType   string    `gorm:"size:140;not null"`
	TenantCode  string    `gorm:"size:64;not null"`
	UpdatedAt   time.Time `gorm:"not null"`
}

func (gormCheckpoint) TableName() string { return "platform_search_projection_checkpoints" }

func PrepareGORMStore(ctx context.Context, db *gorm.DB) error {
	if ctx == nil || db == nil {
		return ErrInvalidMapping
	}
	if err := outbox.PrepareGORMStore(ctx, db); err != nil {
		return err
	}
	return db.WithContext(ctx).AutoMigrate(&gormDocument{}, &gormCheckpoint{})
}

func OpenGORMStore(ctx context.Context, db *gorm.DB, now func() time.Time) (*GORMStore, error) {
	if ctx == nil || db == nil || now == nil || !db.WithContext(ctx).Migrator().HasTable(&gormDocument{}) || !db.WithContext(ctx).Migrator().HasTable(&gormCheckpoint{}) {
		return nil, ErrInvalidMapping
	}
	ledger, err := outbox.OpenConsumerLedger(ctx, db, now)
	if err != nil {
		return nil, err
	}
	return &GORMStore{db: db, now: now, ledger: ledger}, nil
}

// Apply consumes one validated outbox envelope. The consumer receipt, local
// projection document and checkpoint share one transaction. Search adapters
// must make Index and Delete idempotent because an external call can succeed
// before the database transaction's final commit.
func (s *GORMStore) Apply(ctx context.Context, mapping Mapping, envelope outbox.Envelope, indexer integration.SearchIndexer) (bool, error) {
	if s == nil || s.db == nil || s.ledger == nil || ctx == nil || indexer == nil || !validMapping(mapping) {
		return false, ErrInvalidMapping
	}
	validated, err := outbox.ValidateEnvelope(envelope)
	if err != nil {
		return false, err
	}
	if validated.EventType != mapping.EventType {
		return false, ErrInvalidMapping
	}
	document, deleted, err := documentFromEnvelope(mapping, validated)
	if err != nil {
		return false, err
	}
	return s.ledger.Apply(ctx, mapping.ConsumerID, validated, func(tx *gorm.DB) error {
		now := s.now().UTC()
		row := gormDocument{MappingID: mapping.ID, TenantCode: validated.TenantCode, DocumentID: document.ID, Collection: mapping.Collection, Body: append([]byte(nil), document.Body...), CreatedAt: now, UpdatedAt: now}
		if deleted {
			row.DeletedAt = &now
		}
		if err := tx.Save(&row).Error; err != nil {
			return err
		}
		if deleted {
			if err := indexer.Delete(ctx, integration.SearchDocumentRef{Collection: mapping.Collection, ID: document.ID, TenantID: validated.TenantCode}); err != nil {
				return err
			}
		} else if err := indexer.Index(ctx, document); err != nil {
			return err
		}
		return tx.Save(&gormCheckpoint{MappingID: mapping.ID, ConsumerID: mapping.ConsumerID, LastEventID: validated.EventID, EventType: validated.EventType, TenantCode: validated.TenantCode, UpdatedAt: now}).Error
	})
}

// Rebuild replays the local, bounded projection cache to the registered
// adapter. It can restore the adapter without querying arbitrary business
// tables and includes tombstones to keep deletions synchronized.
func (s *GORMStore) Rebuild(ctx context.Context, mapping Mapping, indexer integration.SearchIndexer, options RebuildOptions) (RebuildResult, error) {
	if s == nil || s.db == nil || ctx == nil || indexer == nil || !validMapping(mapping) || options.Limit <= 0 || options.Limit > maxRebuildDocuments {
		return RebuildResult{}, ErrInvalidMapping
	}
	var rows []gormDocument
	if err := s.db.WithContext(ctx).Where("mapping_id = ? AND collection = ?", mapping.ID, mapping.Collection).Order("updated_at ASC, document_id ASC").Limit(options.Limit).Find(&rows).Error; err != nil {
		return RebuildResult{}, err
	}
	result := RebuildResult{}
	for _, row := range rows {
		if row.DeletedAt != nil {
			if err := indexer.Delete(ctx, integration.SearchDocumentRef{Collection: row.Collection, ID: row.DocumentID, TenantID: row.TenantCode}); err != nil {
				return result, err
			}
			result.Deleted++
			continue
		}
		if err := indexer.Index(ctx, integration.SearchDocument{Collection: row.Collection, ID: row.DocumentID, TenantID: row.TenantCode, Body: append([]byte(nil), row.Body...)}); err != nil {
			return result, err
		}
		result.Indexed++
	}
	return result, nil
}

func validMapping(mapping Mapping) bool {
	if !namePattern.MatchString(strings.TrimSpace(mapping.ID)) || !namePattern.MatchString(strings.TrimSpace(mapping.ConsumerID)) || !namePattern.MatchString(strings.TrimSpace(mapping.EventType)) || !namePattern.MatchString(strings.TrimSpace(mapping.Collection)) || !fieldPattern.MatchString(strings.TrimSpace(mapping.DocumentIDField)) || !fieldPattern.MatchString(strings.TrimSpace(mapping.DeletedField)) || len(mapping.Fields) == 0 || len(mapping.Fields) > 32 {
		return false
	}
	seen := make(map[string]struct{}, len(mapping.Fields))
	for _, field := range mapping.Fields {
		field = strings.TrimSpace(field)
		if !fieldPattern.MatchString(field) || field == mapping.DocumentIDField || field == mapping.DeletedField {
			return false
		}
		if _, exists := seen[field]; exists {
			return false
		}
		seen[field] = struct{}{}
	}
	return true
}

func documentFromEnvelope(mapping Mapping, envelope outbox.Envelope) (integration.SearchDocument, bool, error) {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return integration.SearchDocument{}, false, ErrInvalidMapping
	}
	var id string
	if raw, exists := payload[mapping.DocumentIDField]; !exists || json.Unmarshal(raw, &id) != nil || !documentIDPattern.MatchString(strings.TrimSpace(id)) {
		return integration.SearchDocument{}, false, ErrInvalidMapping
	}
	deleted := false
	if raw, exists := payload[mapping.DeletedField]; exists && json.Unmarshal(raw, &deleted) != nil {
		return integration.SearchDocument{}, false, ErrInvalidMapping
	}
	body := make(map[string]json.RawMessage, len(mapping.Fields))
	for _, field := range mapping.Fields {
		if raw, exists := payload[field]; exists {
			body[field] = append(json.RawMessage(nil), raw...)
		}
	}
	keys := make([]string, 0, len(body))
	for key := range body {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	ordered := make(map[string]json.RawMessage, len(keys))
	for _, key := range keys {
		ordered[key] = body[key]
	}
	encoded, err := json.Marshal(ordered)
	if err != nil {
		return integration.SearchDocument{}, false, ErrInvalidMapping
	}
	return integration.SearchDocument{Collection: mapping.Collection, ID: strings.TrimSpace(id), TenantID: envelope.TenantCode, Body: encoded}, deleted, nil
}
