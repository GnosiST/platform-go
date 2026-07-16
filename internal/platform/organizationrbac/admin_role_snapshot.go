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

type AdminRoleSnapshotWriter struct{}

func NewAdminRoleSnapshotWriter() AdminRoleSnapshotWriter { return AdminRoleSnapshotWriter{} }

type gormRoleMetadata struct {
	ID          string `gorm:"column:id;primaryKey"`
	Code        string `gorm:"column:code;uniqueIndex;not null"`
	Name        string `gorm:"column:name;not null"`
	Status      string `gorm:"column:status;not null"`
	Description string `gorm:"column:description;not null;default:''"`
	UpdatedAt   string `gorm:"column:updated_at;not null;default:''"`
	GroupCode   string `gorm:"column:group_code;index;not null"`
	ValuesJSON  string `gorm:"column:values_json;not null;default:'{}'"`
}

type gormRoleGroupMetadata struct {
	ID          string `gorm:"column:id;primaryKey"`
	Code        string `gorm:"column:code;uniqueIndex;not null"`
	Name        string `gorm:"column:name;not null"`
	Status      string `gorm:"column:status;not null"`
	Description string `gorm:"column:description;not null;default:''"`
	UpdatedAt   string `gorm:"column:updated_at;not null;default:''"`
	ScopeType   string `gorm:"column:scope_type;index;not null"`
	TenantCode  string `gorm:"column:tenant_code;index;not null"`
	Revision    uint64 `gorm:"column:revision;not null;default:0"`
	ParentCode  string `gorm:"column:parent_code;index;not null;default:''"`
	SortOrder   int    `gorm:"column:sort_order;index;not null;default:0"`
	ValuesJSON  string `gorm:"column:values_json;not null;default:'{}'"`
}

func (gormRoleMetadata) TableName() string      { return rolesTable }
func (gormRoleGroupMetadata) TableName() string { return roleGroupsTable }

func (AdminRoleSnapshotWriter) ApplyRoleSnapshot(ctx context.Context, tx *gorm.DB, current, proposed []adminresource.Record) error {
	if ctx == nil || tx == nil {
		return adminresource.ErrInvalidRecord
	}
	currentByID, proposedIDs, err := validateRoleSnapshotIdentity(current, proposed)
	if err != nil {
		return err
	}
	for _, record := range current {
		if _, exists := proposedIDs[record.ID]; !exists {
			return adminresource.ErrDomainOwnedMutation
		}
	}
	for _, record := range proposed {
		previous, exists := currentByID[record.ID]
		if exists && reflect.DeepEqual(previous, record) {
			continue
		}
		if err := validateAdminRoleRecord(tx.WithContext(ctx), previous, record, exists); err != nil {
			return err
		}
		if err := applyAdminRoleMetadata(tx, record, exists); err != nil {
			return err
		}
	}
	return nil
}

func (AdminRoleSnapshotWriter) ApplyRoleGroupSnapshot(ctx context.Context, tx *gorm.DB, current, proposed []adminresource.Record) error {
	if ctx == nil || tx == nil {
		return adminresource.ErrInvalidRecord
	}
	currentByID, proposedIDs, err := validateRoleSnapshotIdentity(current, proposed)
	if err != nil {
		return err
	}
	for _, record := range current {
		if _, exists := proposedIDs[record.ID]; !exists {
			return adminresource.ErrDomainOwnedMutation
		}
	}
	for _, record := range proposed {
		previous, exists := currentByID[record.ID]
		if exists && reflect.DeepEqual(previous, record) {
			continue
		}
		if err := validateAdminRoleGroupRecord(tx.WithContext(ctx), previous, record, exists); err != nil {
			return err
		}
		if err := applyAdminRoleGroupMetadata(tx, record, exists); err != nil {
			return err
		}
	}
	return nil
}

func validateRoleSnapshotIdentity(current, proposed []adminresource.Record) (map[string]adminresource.Record, map[string]struct{}, error) {
	currentByID := make(map[string]adminresource.Record, len(current))
	for _, record := range current {
		currentByID[record.ID] = record
	}
	proposedIDs := make(map[string]struct{}, len(proposed))
	proposedCodes := make(map[string]struct{}, len(proposed))
	for _, record := range proposed {
		if strings.TrimSpace(record.ID) == "" || !validCode(record.Code) || strings.TrimSpace(record.Name) == "" {
			return nil, nil, adminresource.ErrInvalidRecord
		}
		if _, duplicate := proposedIDs[record.ID]; duplicate {
			return nil, nil, adminresource.ErrInvalidRecord
		}
		if _, duplicate := proposedCodes[record.Code]; duplicate {
			return nil, nil, adminresource.ErrInvalidRecord
		}
		if previous, exists := currentByID[record.ID]; exists && previous.Code != record.Code {
			return nil, nil, adminresource.ErrDomainOwnedMutation
		}
		proposedIDs[record.ID] = struct{}{}
		proposedCodes[record.Code] = struct{}{}
	}
	return currentByID, proposedIDs, nil
}

func validateAdminRoleRecord(db *gorm.DB, previous, proposed adminresource.Record, exists bool) error {
	if authorizationLifecycleProjectionChanged(previous, proposed, exists) {
		return adminresource.ErrDomainOwnedMutation
	}
	groupCode := strings.TrimSpace(proposed.Values["groupCode"])
	if !validCode(groupCode) {
		return adminresource.ErrInvalidRecord
	}
	if exists && (previous.Status != proposed.Status || strings.TrimSpace(previous.Values["groupCode"]) != groupCode || roleAuthorizationProjectionChanged(previous, proposed)) {
		return adminresource.ErrDomainOwnedMutation
	}
	var group gormRoleGroup
	if err := db.Where("code = ?", groupCode).Take(&group).Error; err != nil {
		return adminresource.ErrInvalidRecord
	}
	deleted, err := isLifecycleDeleted(db, "role-groups", group.ID)
	if err != nil || deleted || group.Status != StatusEnabled || ValidateRoleGroup(roleGroupFromGORM(group, false)) != nil {
		return adminresource.ErrInvalidRecord
	}
	if !exists {
		if proposed.Status != StatusEnabled || roleAuthorizationValuesPresent(proposed) {
			return adminresource.ErrDomainOwnedMutation
		}
		return nil
	}
	return nil
}

func validateAdminRoleGroupRecord(db *gorm.DB, previous, proposed adminresource.Record, exists bool) error {
	if authorizationLifecycleProjectionChanged(previous, proposed, exists) || strings.TrimSpace(proposed.Values["parentCode"]) != "" {
		return adminresource.ErrDomainOwnedMutation
	}
	group := RoleGroup{
		Code: proposed.Code, Name: proposed.Name, ScopeType: ScopeType(strings.TrimSpace(proposed.Values["scopeType"])),
		TenantCode: strings.TrimSpace(proposed.Values["tenantCode"]), Status: proposed.Status,
	}
	if ValidateRoleGroup(group) != nil {
		return adminresource.ErrInvalidRecord
	}
	if !exists {
		if proposed.Status != StatusEnabled {
			return adminresource.ErrDomainOwnedMutation
		}
		if group.ScopeType == ScopeTenant {
			var count int64
			if err := db.Table("platform_admin_tenants").Where("code = ? AND status = ?", group.TenantCode, StatusEnabled).Count(&count).Error; err != nil || count != 1 {
				return adminresource.ErrInvalidRecord
			}
		}
		return nil
	}
	if previous.Status != proposed.Status || strings.TrimSpace(previous.Values["scopeType"]) != string(group.ScopeType) || strings.TrimSpace(previous.Values["tenantCode"]) != group.TenantCode {
		return adminresource.ErrDomainOwnedMutation
	}
	return nil
}

func authorizationLifecycleProjectionChanged(previous, proposed adminresource.Record, exists bool) bool {
	if proposed.DeletedAt != "" || proposed.DeletedBy != "" || proposed.DeleteReason != "" || proposed.PurgeAfter != "" || proposed.DeletionPolicyVersion != 0 {
		return true
	}
	if !exists {
		return false
	}
	return previous.DeletedAt != proposed.DeletedAt || previous.DeletedBy != proposed.DeletedBy ||
		previous.DeleteReason != proposed.DeleteReason || previous.PurgeAfter != proposed.PurgeAfter ||
		previous.DeletionPolicyVersion != proposed.DeletionPolicyVersion
}

func roleAuthorizationValuesPresent(record adminresource.Record) bool {
	for _, key := range []string{"permissions", "denyPermissions", "dataScope", "dataScopeOrgCodes", "dataScopeAreaCodes"} {
		if strings.TrimSpace(record.Values[key]) != "" {
			return true
		}
	}
	return false
}

func roleAuthorizationProjectionChanged(current, proposed adminresource.Record) bool {
	for _, key := range []string{"permissions", "denyPermissions", "dataScope", "dataScopeOrgCodes", "dataScopeAreaCodes"} {
		if key == "permissions" || key == "denyPermissions" || key == "dataScopeOrgCodes" || key == "dataScopeAreaCodes" {
			left := rbac.ParsePermissionList(current.Values[key])
			right := rbac.ParsePermissionList(proposed.Values[key])
			sort.Strings(left)
			sort.Strings(right)
			if !reflect.DeepEqual(left, right) {
				return true
			}
			continue
		}
		if strings.TrimSpace(current.Values[key]) != strings.TrimSpace(proposed.Values[key]) {
			return true
		}
	}
	return false
}

func applyAdminRoleMetadata(tx *gorm.DB, record adminresource.Record, exists bool) error {
	values, err := metadataValues(record.Values, "permissions")
	if err != nil {
		return err
	}
	row := gormRoleMetadata{
		ID: record.ID, Code: record.Code, Name: record.Name, Status: record.Status, Description: record.Description,
		UpdatedAt: record.UpdatedAt, GroupCode: strings.TrimSpace(record.Values["groupCode"]), ValuesJSON: values,
	}
	if !exists {
		return tx.Create(&row).Error
	}
	result := tx.Model(&gormRoleMetadata{}).Where("id = ?", row.ID).Updates(map[string]any{
		"name": row.Name, "description": row.Description, "updated_at": row.UpdatedAt, "values_json": row.ValuesJSON,
	})
	if result.Error != nil || result.RowsAffected != 1 {
		return adminresource.ErrInvalidRecord
	}
	return nil
}

func applyAdminRoleGroupMetadata(tx *gorm.DB, record adminresource.Record, exists bool) error {
	values, err := metadataValues(record.Values, "parentCode")
	if err != nil {
		return err
	}
	sortOrder, _ := strconv.Atoi(strings.TrimSpace(record.Values["sortOrder"]))
	row := gormRoleGroupMetadata{
		ID: record.ID, Code: record.Code, Name: record.Name, Status: record.Status, Description: record.Description,
		UpdatedAt: record.UpdatedAt, ScopeType: strings.TrimSpace(record.Values["scopeType"]), TenantCode: strings.TrimSpace(record.Values["tenantCode"]),
		ParentCode: "", SortOrder: sortOrder, ValuesJSON: values,
	}
	if !exists {
		return tx.Create(&row).Error
	}
	result := tx.Model(&gormRoleGroupMetadata{}).Where("id = ?", row.ID).Updates(map[string]any{
		"name": row.Name, "description": row.Description, "updated_at": row.UpdatedAt, "sort_order": row.SortOrder, "values_json": row.ValuesJSON,
	})
	if result.Error != nil || result.RowsAffected != 1 {
		return adminresource.ErrInvalidRecord
	}
	return nil
}

func metadataValues(source map[string]string, omit ...string) (string, error) {
	values := make(map[string]string, len(source))
	for key, value := range source {
		values[key] = value
	}
	for _, key := range omit {
		delete(values, key)
	}
	encoded, err := json.Marshal(values)
	return string(encoded), err
}
