package organizationrbac

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"strings"

	"github.com/GnosiST/platform-go/internal/platform/adminresource"
	"github.com/GnosiST/platform-go/internal/platform/rbac"

	"gorm.io/gorm"
)

type AdminUserSnapshotWriter struct{}

func NewAdminUserSnapshotWriter() AdminUserSnapshotWriter { return AdminUserSnapshotWriter{} }

func (AdminUserSnapshotWriter) ApplyUserSnapshot(ctx context.Context, tx *gorm.DB, current, proposed []adminresource.Record) error {
	if ctx == nil || tx == nil {
		return adminresource.ErrInvalidRecord
	}
	currentByID := make(map[string]adminresource.Record, len(current))
	for _, record := range current {
		currentByID[record.ID] = record
	}
	proposedIDs := make(map[string]struct{}, len(proposed))
	proposedCodes := make(map[string]struct{}, len(proposed))
	for _, record := range proposed {
		if strings.TrimSpace(record.ID) == "" || !validCode(record.Code) {
			return adminresource.ErrInvalidRecord
		}
		if _, duplicate := proposedIDs[record.ID]; duplicate {
			return adminresource.ErrInvalidRecord
		}
		if _, duplicate := proposedCodes[record.Code]; duplicate {
			return adminresource.ErrInvalidRecord
		}
		proposedIDs[record.ID] = struct{}{}
		proposedCodes[record.Code] = struct{}{}
		previous, exists := currentByID[record.ID]
		if exists && previous.Code != record.Code {
			return adminresource.ErrDomainOwnedMutation
		}
		if exists && reflect.DeepEqual(previous, record) {
			continue
		}
		if err := applyAdminUserRecord(ctx, tx, previous, record, exists); err != nil {
			return err
		}
	}
	for _, record := range current {
		if _, exists := proposedIDs[record.ID]; !exists {
			return adminresource.ErrDomainOwnedMutation
		}
	}
	return nil
}

func applyAdminUserRecord(ctx context.Context, tx *gorm.DB, previous, proposed adminresource.Record, exists bool) error {
	roles := adminUserRoleCodes(proposed)
	scopeType := ScopeType(strings.TrimSpace(proposed.Values["scopeType"]))
	if scopeType == "" {
		scopeType = ScopeTenant
	}
	if exists && authorizationUserProjectionChanged(previous, proposed) {
		return adminresource.ErrDomainOwnedMutation
	}
	if !exists && scopeType != ScopeTenant {
		return adminresource.ErrDomainOwnedMutation
	}
	user := User{
		ID: proposed.ID, Code: proposed.Code, ScopeType: scopeType,
		TenantCode: strings.TrimSpace(proposed.Values["tenantCode"]), OrgUnitCode: strings.TrimSpace(proposed.Values["orgUnitCode"]),
		Status: proposed.Status,
	}
	derived, err := (&GORMRepository{db: tx}).DeriveAndValidateUser(ctx, user, roles)
	if err != nil {
		if errors.Is(err, ErrInvalid) || errors.Is(err, ErrNotFound) || errors.Is(err, ErrRolePoolViolation) {
			return adminresource.ErrInvalidRecord
		}
		return err
	}
	if user.TenantCode != "" && user.TenantCode != derived.TenantCode {
		return adminresource.ErrInvalidRecord
	}
	values := make(map[string]string, len(proposed.Values)+3)
	for key, value := range proposed.Values {
		values[key] = value
	}
	values["scopeType"] = string(derived.ScopeType)
	values["tenantCode"] = derived.TenantCode
	values["orgUnitCode"] = derived.OrgUnitCode
	encodedValues, err := json.Marshal(values)
	if err != nil {
		return err
	}
	row := gormUser{
		ID: proposed.ID, Code: proposed.Code, Name: proposed.Name, Description: proposed.Description,
		ScopeType: string(derived.ScopeType), TenantCode: derived.TenantCode, OrgUnitCode: derived.OrgUnitCode,
		Status: proposed.Status, UpdatedAt: proposed.UpdatedAt, ValuesJSON: string(encodedValues),
	}
	if exists {
		result := tx.Model(&gormUser{}).Where("id = ?", proposed.ID).Updates(map[string]any{
			"name": row.Name, "description": row.Description, "scope_type": row.ScopeType,
			"tenant_code": row.TenantCode, "org_unit_code": row.OrgUnitCode, "status": row.Status,
			"updated_at": row.UpdatedAt, "values_json": row.ValuesJSON,
		})
		if result.Error != nil || result.RowsAffected != 1 {
			return adminresource.ErrInvalidRecord
		}
		return nil
	}
	if err := tx.Create(&row).Error; err != nil {
		return err
	}
	assignments := make([]gormUserRole, 0, len(roles))
	for _, roleCode := range roles {
		assignments = append(assignments, gormUserRole{UserID: proposed.ID, RoleCode: roleCode})
	}
	if len(assignments) > 0 {
		return tx.Create(&assignments).Error
	}
	return nil
}

func authorizationUserProjectionChanged(current, proposed adminresource.Record) bool {
	if current.Status != proposed.Status {
		return true
	}
	for _, key := range []string{"scopeType", "tenantCode", "orgUnitCode"} {
		if strings.TrimSpace(current.Values[key]) != strings.TrimSpace(proposed.Values[key]) {
			return true
		}
	}
	return !reflect.DeepEqual(adminUserRoleCodes(current), adminUserRoleCodes(proposed))
}

func adminUserRoleCodes(record adminresource.Record) []string {
	roles := rbac.ParsePermissionList(record.Values["roles"])
	if len(roles) == 0 {
		roles = rbac.ParsePermissionList(record.Values["role"])
	}
	sort.Strings(roles)
	return roles
}
