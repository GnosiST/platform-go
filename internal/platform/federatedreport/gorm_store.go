package federatedreport

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"
)

type gormDefinition struct {
	ID          string `gorm:"primaryKey;size:64;not null"`
	Version     uint64 `gorm:"primaryKey;not null"`
	Sources     string `gorm:"type:text;not null"`
	Fields      string `gorm:"type:text;not null"`
	MaximumRows int    `gorm:"not null"`
	MaximumCost int    `gorm:"not null"`
	TimeoutNS   int64  `gorm:"not null"`
}

func (gormDefinition) TableName() string { return "platform_federated_report_definitions" }

type GORMDefinitionStore struct{ db *gorm.DB }

func PrepareGORMDefinitionStore(ctx context.Context, db *gorm.DB) error {
	if ctx == nil || db == nil {
		return ErrInvalidDefinition
	}
	return db.WithContext(ctx).AutoMigrate(&gormDefinition{})
}

func OpenGORMDefinitionStore(ctx context.Context, db *gorm.DB) (*GORMDefinitionStore, error) {
	if ctx == nil || db == nil || !db.WithContext(ctx).Migrator().HasTable(&gormDefinition{}) {
		return nil, ErrInvalidDefinition
	}
	return &GORMDefinitionStore{db: db}, nil
}

func (s *GORMDefinitionStore) Save(ctx context.Context, definition Definition) error {
	if s == nil || s.db == nil || ctx == nil || !validStoredDefinition(definition) {
		return ErrInvalidDefinition
	}
	sources, err := json.Marshal(definition.Sources)
	if err != nil {
		return ErrInvalidDefinition
	}
	fields, err := json.Marshal(definition.AllowedFields)
	if err != nil {
		return ErrInvalidDefinition
	}
	if err := s.db.WithContext(ctx).Create(&gormDefinition{ID: definition.ID, Version: definition.Version, Sources: string(sources), Fields: string(fields), MaximumRows: definition.MaximumRows, MaximumCost: definition.MaximumCost, TimeoutNS: int64(definition.Timeout)}).Error; err != nil {
		return fmt.Errorf("save federated report definition: %w", err)
	}
	return nil
}

func (s *GORMDefinitionStore) Runtime(executors map[string]SourceExecutor) (*Runtime, error) {
	if s == nil || s.db == nil {
		return nil, ErrInvalidDefinition
	}
	var rows []gormDefinition
	if err := s.db.Order("id ASC, version ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("load federated report definitions: %w", err)
	}
	definitions := make([]Definition, 0, len(rows))
	for _, row := range rows {
		var sources []Source
		var fields []string
		if json.Unmarshal([]byte(row.Sources), &sources) != nil || json.Unmarshal([]byte(row.Fields), &fields) != nil {
			return nil, ErrInvalidDefinition
		}
		definitions = append(definitions, Definition{ID: row.ID, Version: row.Version, Sources: sources, AllowedFields: fields, MaximumRows: row.MaximumRows, MaximumCost: row.MaximumCost, Timeout: time.Duration(row.TimeoutNS)})
	}
	return NewRuntime(definitions, executors)
}

func validStoredDefinition(definition Definition) bool {
	if !validIdentifier(definition.ID) || definition.Version == 0 || len(definition.Sources) == 0 || len(definition.AllowedFields) == 0 || definition.MaximumRows < 1 || definition.MaximumRows > 10000 || definition.MaximumCost < 1 || definition.Timeout <= 0 || definition.Timeout > time.Minute {
		return false
	}
	for _, source := range definition.Sources {
		if !validIdentifier(source.ID) || !validIdentifier(source.DatasourceGroupID) {
			return false
		}
	}
	for _, field := range definition.AllowedFields {
		if !validIdentifier(field) {
			return false
		}
	}
	return true
}
