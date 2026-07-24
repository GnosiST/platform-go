package tenantplacement

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/GnosiST/platform-go/internal/platform/kernel"
	"gorm.io/gorm"
)

type gormPlacement struct {
	TenantCode        string `gorm:"primaryKey;size:64;not null"`
	CapabilityID      string `gorm:"primaryKey;size:64;not null"`
	DatasourceGroupID string `gorm:"size:64;not null"`
	Version           uint64 `gorm:"not null"`
	Status            string `gorm:"size:16;not null"`
}

func (gormPlacement) TableName() string { return "platform_tenant_placements" }

type gormAuditEntry struct {
	ID                        uint64    `gorm:"primaryKey;autoIncrement"`
	Kind                      string    `gorm:"size:16;not null"`
	Actor                     string    `gorm:"size:128;not null"`
	TenantCode                string    `gorm:"size:64;not null"`
	CapabilityID              string    `gorm:"size:64;not null"`
	PreviousDatasourceGroupID string    `gorm:"size:64"`
	DatasourceGroupID         string    `gorm:"size:64;not null"`
	PreviousVersion           uint64    `gorm:"not null"`
	Version                   uint64    `gorm:"not null"`
	Status                    string    `gorm:"size:16;not null"`
	Reason                    string    `gorm:"size:256;not null"`
	OccurredAt                time.Time `gorm:"not null"`
}

func (gormAuditEntry) TableName() string { return "platform_tenant_placement_audits" }

type GORMStore struct {
	db         *gorm.DB
	authorizer Authorizer
	now        func() time.Time
}

func PrepareGORMStore(ctx context.Context, db *gorm.DB) error {
	if ctx == nil || db == nil {
		return ErrInvalidPlacement
	}
	return db.WithContext(ctx).AutoMigrate(&gormPlacement{}, &gormAuditEntry{})
}

func OpenGORMStore(ctx context.Context, db *gorm.DB, authorizer Authorizer, now func() time.Time) (*GORMStore, error) {
	if ctx == nil || db == nil || authorizer == nil || now == nil || !db.WithContext(ctx).Migrator().HasTable(&gormPlacement{}) || !db.WithContext(ctx).Migrator().HasTable(&gormAuditEntry{}) {
		return nil, ErrInvalidPlacement
	}
	return &GORMStore{db: db, authorizer: authorizer, now: now}, nil
}

func (s *GORMStore) Create(execution kernel.ExecutionContext, placement Placement, reason string) (Placement, error) {
	if err := validateExecution(execution, placement.CapabilityID); err != nil || execution.TenantScope.TenantCode != placement.TenantCode || s == nil || s.authorizer == nil || !s.authorizer.Can(execution, PermissionManage) {
		return Placement{}, ErrUnauthorized
	}
	if err := validatePlacement(placement); err != nil || !validReason(reason) {
		return Placement{}, ErrInvalidPlacement
	}
	err := s.db.WithContext(execution.BaseContext()).Transaction(func(tx *gorm.DB) error {
		placementRow := toGORMPlacement(placement)
		if err := tx.Create(&placementRow).Error; err != nil {
			return err
		}
		auditRow := toGORMAudit(AuditEntry{Kind: "create", Actor: execution.Actor.Username, TenantCode: placement.TenantCode, CapabilityID: placement.CapabilityID, DatasourceGroupID: placement.DatasourceGroupID, Version: placement.Version, Status: placement.Status, Reason: reason, OccurredAt: s.now().UTC()})
		return tx.Create(&auditRow).Error
	})
	if err != nil {
		return Placement{}, fmt.Errorf("create tenant placement: %w", err)
	}
	return placement, nil
}

func (s *GORMStore) Resolve(execution kernel.ExecutionContext, capabilityID string, override Override) (Placement, error) {
	if s == nil || s.db == nil {
		return Placement{}, ErrPlacementNotFound
	}
	if err := validateExecution(execution, capabilityID); err != nil {
		return Placement{}, err
	}
	var row gormPlacement
	err := s.db.WithContext(execution.BaseContext()).Where("tenant_code = ? AND capability_id = ?", execution.TenantScope.TenantCode, capabilityID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Placement{}, ErrPlacementNotFound
	}
	if err != nil {
		return Placement{}, fmt.Errorf("load tenant placement: %w", err)
	}
	placement := fromGORMPlacement(row)
	if isEmptyOverride(override) {
		return placement, nil
	}
	if err := s.validateOverride(execution, placement, override); err != nil {
		return Placement{}, err
	}
	placement.DatasourceGroupID = override.DatasourceGroupID
	auditRow := toGORMAudit(AuditEntry{Kind: "override", Actor: execution.Actor.Username, TenantCode: placement.TenantCode, CapabilityID: placement.CapabilityID, PreviousDatasourceGroupID: row.DatasourceGroupID, DatasourceGroupID: placement.DatasourceGroupID, PreviousVersion: placement.Version, Version: placement.Version, Status: placement.Status, Reason: override.Reason, OccurredAt: s.now().UTC()})
	if err := s.db.WithContext(execution.BaseContext()).Create(&auditRow).Error; err != nil {
		return Placement{}, fmt.Errorf("audit tenant placement override: %w", err)
	}
	return placement, nil
}

func (s *GORMStore) Transition(execution kernel.ExecutionContext, transition Transition) (Placement, error) {
	if s == nil || s.db == nil || s.authorizer == nil || !s.authorizer.Can(execution, PermissionManage) || execution.TenantScope.TenantCode != transition.TenantCode {
		return Placement{}, ErrUnauthorized
	}
	if err := validateExecution(execution, transition.CapabilityID); err != nil || !validIdentifier(transition.TenantCode) || !validIdentifier(transition.DatasourceGroupID) || !validReason(transition.Reason) || (transition.Status != StatusActive && transition.Status != StatusMigrating) {
		return Placement{}, ErrInvalidPlacement
	}
	var updated Placement
	err := s.db.WithContext(execution.BaseContext()).Transaction(func(tx *gorm.DB) error {
		var row gormPlacement
		if err := tx.Where("tenant_code = ? AND capability_id = ?", transition.TenantCode, transition.CapabilityID).Take(&row).Error; err != nil {
			return err
		}
		previous := fromGORMPlacement(row)
		if transition.Version != previous.Version+1 {
			return ErrInvalidPlacement
		}
		result := tx.Model(&gormPlacement{}).Where("tenant_code = ? AND capability_id = ? AND version = ?", previous.TenantCode, previous.CapabilityID, previous.Version).Updates(map[string]any{"datasource_group_id": transition.DatasourceGroupID, "version": transition.Version, "status": string(transition.Status)})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrInvalidPlacement
		}
		updated = Placement{TenantCode: transition.TenantCode, CapabilityID: transition.CapabilityID, DatasourceGroupID: transition.DatasourceGroupID, Version: transition.Version, Status: transition.Status}
		auditRow := toGORMAudit(AuditEntry{Kind: "transition", Actor: execution.Actor.Username, TenantCode: updated.TenantCode, CapabilityID: updated.CapabilityID, PreviousDatasourceGroupID: previous.DatasourceGroupID, DatasourceGroupID: updated.DatasourceGroupID, PreviousVersion: previous.Version, Version: updated.Version, Status: updated.Status, Reason: transition.Reason, OccurredAt: s.now().UTC()})
		return tx.Create(&auditRow).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Placement{}, ErrPlacementNotFound
	}
	if err != nil {
		return Placement{}, err
	}
	return updated, nil
}

func (s *GORMStore) Audit(ctx context.Context) ([]AuditEntry, error) {
	if s == nil || s.db == nil || ctx == nil {
		return nil, ErrInvalidPlacement
	}
	var rows []gormAuditEntry
	if err := s.db.WithContext(ctx).Order("id ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list tenant placement audit: %w", err)
	}
	entries := make([]AuditEntry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, fromGORMAudit(row))
	}
	return entries, nil
}

func (s *GORMStore) validateOverride(execution kernel.ExecutionContext, placement Placement, override Override) error {
	if s.authorizer == nil || !s.authorizer.Can(execution, PermissionOverride) || override.TenantCode != placement.TenantCode || override.CapabilityID != placement.CapabilityID || !validIdentifier(override.DatasourceGroupID) || !validReason(override.Reason) {
		return ErrInvalidOverride
	}
	now := s.now().UTC()
	if override.ExpiresAt.UTC().Before(now) || override.ExpiresAt.UTC().After(now.Add(maximumOverrideTTL)) {
		return ErrInvalidOverride
	}
	return nil
}

func toGORMPlacement(placement Placement) gormPlacement {
	return gormPlacement{TenantCode: placement.TenantCode, CapabilityID: placement.CapabilityID, DatasourceGroupID: placement.DatasourceGroupID, Version: placement.Version, Status: string(placement.Status)}
}

func fromGORMPlacement(row gormPlacement) Placement {
	return Placement{TenantCode: row.TenantCode, CapabilityID: row.CapabilityID, DatasourceGroupID: row.DatasourceGroupID, Version: row.Version, Status: Status(row.Status)}
}

func toGORMAudit(entry AuditEntry) gormAuditEntry {
	return gormAuditEntry{Kind: entry.Kind, Actor: entry.Actor, TenantCode: entry.TenantCode, CapabilityID: entry.CapabilityID, PreviousDatasourceGroupID: entry.PreviousDatasourceGroupID, DatasourceGroupID: entry.DatasourceGroupID, PreviousVersion: entry.PreviousVersion, Version: entry.Version, Status: string(entry.Status), Reason: entry.Reason, OccurredAt: entry.OccurredAt}
}

func fromGORMAudit(row gormAuditEntry) AuditEntry {
	return AuditEntry{Kind: row.Kind, Actor: row.Actor, TenantCode: row.TenantCode, CapabilityID: row.CapabilityID, PreviousDatasourceGroupID: row.PreviousDatasourceGroupID, DatasourceGroupID: row.DatasourceGroupID, PreviousVersion: row.PreviousVersion, Version: row.Version, Status: Status(row.Status), Reason: row.Reason, OccurredAt: row.OccurredAt}
}
