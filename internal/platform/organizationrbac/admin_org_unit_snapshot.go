package organizationrbac

import (
	"context"
	"encoding/json"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/GnosiST/platform-go/internal/platform/adminresource"
	"github.com/GnosiST/platform-go/internal/platform/rbac"

	"gorm.io/gorm"
)

type AdminOrgUnitSnapshotWriter struct{}

func NewAdminOrgUnitSnapshotWriter() AdminOrgUnitSnapshotWriter { return AdminOrgUnitSnapshotWriter{} }

type gormOrgUnitMetadata struct {
	ID          string `gorm:"column:id;primaryKey"`
	Code        string `gorm:"column:code;uniqueIndex;not null"`
	Name        string `gorm:"column:name;not null"`
	Status      string `gorm:"column:status;not null"`
	Description string `gorm:"column:description;not null"`
	UpdatedAt   string `gorm:"column:updated_at;not null"`
	Type        string `gorm:"column:type;index;not null"`
	TenantCode  string `gorm:"column:tenant_code;index;not null"`
	ParentCode  string `gorm:"column:parent_code;index;not null"`
	AreaCode    string `gorm:"column:area_code;index;not null"`
	SortOrder   int    `gorm:"column:sort_order;index;not null"`
	ValuesJSON  string `gorm:"column:values_json;not null"`
}

type gormOrgUnitTenant struct {
	Code   string `gorm:"column:code;primaryKey"`
	Status string `gorm:"column:status;not null"`
}

func (gormOrgUnitMetadata) TableName() string { return organizationsTable }
func (gormOrgUnitTenant) TableName() string   { return "platform_admin_tenants" }

func (AdminOrgUnitSnapshotWriter) ApplyOrgUnitSnapshot(ctx context.Context, tx *gorm.DB, current, proposed []adminresource.Record) error {
	if ctx == nil || tx == nil {
		return adminresource.ErrInvalidRecord
	}
	currentByID := make(map[string]adminresource.Record, len(current))
	for _, record := range current {
		currentByID[record.ID] = record
	}
	proposedByCode := make(map[string]adminresource.Record, len(proposed))
	proposedIDs := make(map[string]struct{}, len(proposed))
	for _, record := range proposed {
		if _, duplicate := proposedIDs[record.ID]; duplicate {
			return adminresource.ErrInvalidRecord
		}
		if _, duplicate := proposedByCode[record.Code]; duplicate {
			return adminresource.ErrInvalidRecord
		}
		previous, exists := currentByID[record.ID]
		if !exists || !reflect.DeepEqual(previous, record) {
			if err := validateAdminOrgUnitRecord(record); err != nil {
				return err
			}
		}
		proposedIDs[record.ID] = struct{}{}
		proposedByCode[record.Code] = record
	}
	for _, record := range current {
		if _, exists := proposedIDs[record.ID]; !exists {
			return adminresource.ErrDomainOwnedMutation
		}
	}
	for _, record := range proposed {
		if previous, exists := currentByID[record.ID]; exists && adminOrgUnitOwnedProjectionChanged(previous, record) {
			return adminresource.ErrDomainOwnedMutation
		}
	}
	if err := validateAdminOrgUnitHierarchy(proposedByCode); err != nil {
		return err
	}
	if err := validateAdminOrgUnitTenants(ctx, tx, proposed); err != nil {
		return err
	}
	for _, record := range proposed {
		previous, exists := currentByID[record.ID]
		if exists && reflect.DeepEqual(previous, record) {
			continue
		}
		if !exists && len(adminOrgUnitRoleGroupCodes(record)) > 0 {
			return adminresource.ErrDomainOwnedMutation
		}
		if err := applyAdminOrgUnitRecord(tx, record, exists); err != nil {
			return err
		}
	}
	return nil
}

func validateAdminOrgUnitRecord(record adminresource.Record) error {
	if strings.TrimSpace(record.ID) == "" || !validCode(record.Code) || strings.TrimSpace(record.Name) == "" || strings.TrimSpace(record.Status) == "" {
		return adminresource.ErrInvalidRecord
	}
	if !validCode(strings.TrimSpace(record.Values["tenantCode"])) || strings.TrimSpace(record.Values["type"]) == "" {
		return adminresource.ErrInvalidRecord
	}
	if record.DeletedAt != "" || record.DeletedBy != "" || record.DeleteReason != "" || record.PurgeAfter != "" || record.DeletionPolicyVersion != 0 {
		return adminresource.ErrDomainOwnedMutation
	}
	return nil
}

func validateAdminOrgUnitHierarchy(records map[string]adminresource.Record) error {
	for code, record := range records {
		if record.DeletedAt != "" {
			continue
		}
		parentCode := strings.TrimSpace(record.Values["parentCode"])
		if parentCode == "" {
			continue
		}
		parent, exists := records[parentCode]
		if !exists || parentCode == code || parent.DeletedAt != "" || strings.TrimSpace(parent.Values["tenantCode"]) != strings.TrimSpace(record.Values["tenantCode"]) {
			return adminresource.ErrInvalidRecord
		}
		seen := map[string]struct{}{code: {}}
		cursor := parentCode
		for cursor != "" {
			if _, cycle := seen[cursor]; cycle {
				return adminresource.ErrInvalidRecord
			}
			seen[cursor] = struct{}{}
			next, exists := records[cursor]
			if !exists {
				return adminresource.ErrInvalidRecord
			}
			cursor = strings.TrimSpace(next.Values["parentCode"])
		}
	}
	return nil
}

func validateAdminOrgUnitTenants(ctx context.Context, tx *gorm.DB, records []adminresource.Record) error {
	requested := make(map[string]struct{})
	for _, record := range records {
		if record.DeletedAt != "" {
			continue
		}
		requested[strings.TrimSpace(record.Values["tenantCode"])] = struct{}{}
	}
	codes := make([]string, 0, len(requested))
	for code := range requested {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	var rows []gormOrgUnitTenant
	if err := tx.WithContext(ctx).Where("code IN ?", codes).Find(&rows).Error; err != nil {
		return err
	}
	enabled := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		if row.Status == StatusEnabled {
			enabled[row.Code] = struct{}{}
		}
	}
	for _, code := range codes {
		if _, ok := enabled[code]; !ok {
			return adminresource.ErrInvalidRecord
		}
	}
	return nil
}

func adminOrgUnitOwnedProjectionChanged(current, proposed adminresource.Record) bool {
	if current.Code != proposed.Code || current.Status != proposed.Status || strings.TrimSpace(current.Values["tenantCode"]) != strings.TrimSpace(proposed.Values["tenantCode"]) {
		return true
	}
	if current.DeletedAt != proposed.DeletedAt || current.DeletedBy != proposed.DeletedBy || current.DeleteReason != proposed.DeleteReason || current.PurgeAfter != proposed.PurgeAfter || current.DeletionPolicyVersion != proposed.DeletionPolicyVersion {
		return true
	}
	return !reflect.DeepEqual(adminOrgUnitRoleGroupCodes(current), adminOrgUnitRoleGroupCodes(proposed))
}

func adminOrgUnitRoleGroupCodes(record adminresource.Record) []string {
	codes := rbac.ParsePermissionList(record.Values["roleGroupCodes"])
	sort.Strings(codes)
	return codes
}

func applyAdminOrgUnitRecord(tx *gorm.DB, record adminresource.Record, exists bool) error {
	values := make(map[string]string, len(record.Values))
	for key, value := range record.Values {
		values[key] = value
	}
	delete(values, "roleGroupCodes")
	delete(values, "roleGroupCount")
	delete(values, "effectiveRoleCount")
	encodedValues, err := json.Marshal(values)
	if err != nil {
		return err
	}
	sortOrder, _ := strconv.Atoi(strings.TrimSpace(record.Values["sortOrder"]))
	row := gormOrgUnitMetadata{
		ID: record.ID, Code: record.Code, Name: record.Name, Status: record.Status, Description: record.Description,
		UpdatedAt: record.UpdatedAt, Type: strings.TrimSpace(record.Values["type"]), TenantCode: strings.TrimSpace(record.Values["tenantCode"]),
		ParentCode: strings.TrimSpace(record.Values["parentCode"]), AreaCode: strings.TrimSpace(record.Values["areaCode"]), SortOrder: sortOrder,
		ValuesJSON: string(encodedValues),
	}
	if !exists {
		return tx.Create(&row).Error
	}
	result := tx.Model(&gormOrgUnitMetadata{}).Where("id = ?", record.ID).Updates(map[string]any{
		"name": row.Name, "description": row.Description, "updated_at": row.UpdatedAt, "type": row.Type,
		"parent_code": row.ParentCode, "area_code": row.AreaCode, "sort_order": row.SortOrder, "values_json": row.ValuesJSON,
	})
	if result.Error != nil || result.RowsAffected != 1 {
		return adminresource.ErrInvalidRecord
	}
	return nil
}
