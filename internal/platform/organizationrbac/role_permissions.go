package organizationrbac

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"platform-go/internal/platform/rbac"

	"gorm.io/gorm"
)

const (
	PermissionResourceTypeAPI        = "api"
	PermissionResourceTypePageButton = "page-button"
)

type RolePermissionImpact struct {
	RoleCode                     string
	TenantCode                   string
	CurrentAllowPermissionCodes  []string
	ProposedAllowPermissionCodes []string
	CurrentDenyPermissionCodes   []string
	ProposedDenyPermissionCodes  []string
	CurrentDataScope             string
	ProposedDataScope            string
	CurrentDataScopeOrgCodes     []string
	ProposedDataScopeOrgCodes    []string
	CurrentDataScopeAreaCodes    []string
	ProposedDataScopeAreaCodes   []string
	AddedCount                   int
	RemovedCount                 int
	Changed                      bool
	AffectedUsers                int
	ExpectedRevision             uint64
}

type ReplaceRolePermissionsRequest struct {
	RoleCode             string
	AllowPermissionCodes []string
	DenyPermissionCodes  []string
	DataScope            string
	DataScopeOrgCodes    []string
	DataScopeAreaCodes   []string
	ExpectedRevision     uint64
	ActorID              string
	ChangedAt            time.Time
}

func (r *GORMRepository) PreviewRolePermissions(ctx context.Context, roleCode string, allowPermissionCodes, denyPermissionCodes []string, dataScope string, dataScopeOrgCodes, dataScopeAreaCodes []string) (RolePermissionImpact, error) {
	if !r.ready(ctx) || !validCode(roleCode) {
		return RolePermissionImpact{}, ErrInvalid
	}
	allow, err := canonicalCodes(allowPermissionCodes)
	if err != nil {
		return RolePermissionImpact{}, err
	}
	deny, err := canonicalCodes(denyPermissionCodes)
	if err != nil {
		return RolePermissionImpact{}, err
	}
	orgCodes, err := canonicalCodes(dataScopeOrgCodes)
	if err != nil {
		return RolePermissionImpact{}, err
	}
	areaCodes, err := canonicalCodes(dataScopeAreaCodes)
	if err != nil {
		return RolePermissionImpact{}, err
	}
	return previewRolePermissions(r.db.WithContext(ctx), roleCode, allow, deny, strings.TrimSpace(dataScope), orgCodes, areaCodes)
}

func (r *GORMRepository) ReplaceRolePermissions(ctx context.Context, request ReplaceRolePermissionsRequest) (uint64, string, error) {
	if !r.ready(ctx) || !validCode(request.RoleCode) || !validCode(request.ActorID) || request.ChangedAt.IsZero() {
		return 0, "", ErrInvalid
	}
	allow, err := canonicalCodes(request.AllowPermissionCodes)
	if err != nil {
		return 0, "", err
	}
	deny, err := canonicalCodes(request.DenyPermissionCodes)
	if err != nil {
		return 0, "", err
	}
	orgCodes, err := canonicalCodes(request.DataScopeOrgCodes)
	if err != nil {
		return 0, "", err
	}
	areaCodes, err := canonicalCodes(request.DataScopeAreaCodes)
	if err != nil {
		return 0, "", err
	}
	var revision uint64
	var tenantCode string
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		impact, err := previewRolePermissions(tx, request.RoleCode, allow, deny, strings.TrimSpace(request.DataScope), orgCodes, areaCodes)
		if err != nil {
			return err
		}
		if impact.ExpectedRevision != request.ExpectedRevision {
			return &RevisionConflictError{Expected: request.ExpectedRevision, Actual: impact.ExpectedRevision}
		}
		if !impact.Changed {
			revision = impact.ExpectedRevision
			tenantCode = impact.TenantCode
			return nil
		}
		if err := tx.Where("role_code = ?", request.RoleCode).Delete(&gormRolePermission{}).Error; err != nil {
			return repositoryError(err)
		}
		for _, permissionCode := range allow {
			if err := tx.Create(&gormRolePermission{RoleCode: request.RoleCode, Permission: permissionCode}).Error; err != nil {
				return repositoryError(err)
			}
		}
		values, err := loadRolePolicyValues(tx, request.RoleCode)
		if err != nil {
			return err
		}
		delete(values, "permissions")
		values["denyPermissions"] = strings.Join(deny, ",")
		values["dataScope"] = strings.TrimSpace(request.DataScope)
		values["dataScopeOrgCodes"] = strings.Join(orgCodes, ",")
		values["dataScopeAreaCodes"] = strings.Join(areaCodes, ",")
		encoded, err := json.Marshal(values)
		if err != nil {
			return ErrInvalid
		}
		update := tx.Model(&gormRole{}).Where("code = ?", request.RoleCode).Update("values_json", string(encoded))
		if update.Error != nil || update.RowsAffected != 1 {
			return ErrRepositoryFailed
		}
		revision, err = advanceGlobalRevision(tx, request.ExpectedRevision)
		tenantCode = impact.TenantCode
		return err
	})
	if err != nil {
		return 0, "", repositoryError(err)
	}
	return revision, tenantCode, nil
}

func previewRolePermissions(db *gorm.DB, roleCode string, allow, deny []string, dataScope string, orgCodes, areaCodes []string) (RolePermissionImpact, error) {
	var role gormRole
	if err := db.Where("code = ?", roleCode).Take(&role).Error; err != nil {
		return RolePermissionImpact{}, repositoryError(err)
	}
	deleted, err := isLifecycleDeleted(db, "roles", role.ID)
	if err != nil {
		return RolePermissionImpact{}, err
	}
	if deleted || role.Status != StatusEnabled {
		return RolePermissionImpact{}, ErrInvalid
	}
	var group gormRoleGroup
	if err := db.Where("code = ?", role.GroupCode).Take(&group).Error; err != nil {
		return RolePermissionImpact{}, repositoryError(err)
	}
	if err := ValidateRoleGroup(roleGroupFromGORM(group, false)); err != nil {
		return RolePermissionImpact{}, err
	}
	if err := validateRolePolicy(db, group, allow, deny, dataScope, orgCodes, areaCodes); err != nil {
		return RolePermissionImpact{}, err
	}
	for _, permissionCode := range append(append([]string{}, allow...), deny...) {
		if err := validatePermissionPolicyCode(db, permissionCode); err != nil {
			return RolePermissionImpact{}, err
		}
	}
	var rows []gormRolePermission
	if err := db.Where("role_code = ?", roleCode).Order("permission").Find(&rows).Error; err != nil {
		return RolePermissionImpact{}, repositoryError(err)
	}
	currentAllow := make([]string, 0, len(rows))
	for _, row := range rows {
		currentAllow = append(currentAllow, row.Permission)
	}
	currentAllow, err = canonicalCodes(currentAllow)
	if err != nil {
		return RolePermissionImpact{}, err
	}
	values, err := loadRolePolicyValues(db, roleCode)
	if err != nil {
		return RolePermissionImpact{}, err
	}
	currentDeny, err := canonicalCodes(rbac.ParsePermissionList(values["denyPermissions"]))
	if err != nil {
		return RolePermissionImpact{}, err
	}
	currentOrgCodes, err := canonicalCodes(rbac.ParsePermissionList(values["dataScopeOrgCodes"]))
	if err != nil {
		return RolePermissionImpact{}, err
	}
	currentAreaCodes, err := canonicalCodes(rbac.ParsePermissionList(values["dataScopeAreaCodes"]))
	if err != nil {
		return RolePermissionImpact{}, err
	}
	currentDataScope := strings.TrimSpace(values["dataScope"])
	if currentDataScope == "" {
		currentDataScope = "all"
	}
	revision, err := loadGlobalRevision(db)
	if err != nil {
		return RolePermissionImpact{}, err
	}
	var affectedUsers int64
	if err := db.Model(&gormUserRole{}).Where("role_code = ?", roleCode).Count(&affectedUsers).Error; err != nil {
		return RolePermissionImpact{}, repositoryError(err)
	}
	allowAdded, allowRemoved := permissionDiffCounts(currentAllow, allow)
	denyAdded, denyRemoved := permissionDiffCounts(currentDeny, deny)
	changed := allowAdded+allowRemoved+denyAdded+denyRemoved != 0 || currentDataScope != dataScope ||
		!equalStrings(currentOrgCodes, orgCodes) || !equalStrings(currentAreaCodes, areaCodes)
	return RolePermissionImpact{
		RoleCode: roleCode, TenantCode: group.TenantCode,
		CurrentAllowPermissionCodes: currentAllow, ProposedAllowPermissionCodes: allow,
		CurrentDenyPermissionCodes: currentDeny, ProposedDenyPermissionCodes: deny,
		CurrentDataScope: currentDataScope, ProposedDataScope: dataScope,
		CurrentDataScopeOrgCodes: currentOrgCodes, ProposedDataScopeOrgCodes: orgCodes,
		CurrentDataScopeAreaCodes: currentAreaCodes, ProposedDataScopeAreaCodes: areaCodes,
		AddedCount: allowAdded + denyAdded, RemovedCount: allowRemoved + denyRemoved, Changed: changed,
		AffectedUsers: int(affectedUsers), ExpectedRevision: revision,
	}, nil
}

func validatePermissionPolicyCode(db *gorm.DB, code string) error {
	code = strings.TrimSpace(code)
	if code == "" {
		return ErrInvalid
	}
	if code == "*" || strings.HasSuffix(code, ":*") && strings.Count(code, "*") == 1 {
		var wildcard gormPermission
		if err := db.Where("code = ?", code).Take(&wildcard).Error; err == nil {
			deleted, lifecycleErr := isLifecycleDeleted(db, "permissions", wildcard.ID)
			if lifecycleErr != nil {
				return lifecycleErr
			}
			if deleted || wildcard.Status != StatusEnabled || !validPermissionResourceType(wildcard.ResourceType) {
				return ErrInvalid
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return repositoryError(err)
		}
		prefix := ""
		if code != "*" {
			prefix = strings.TrimSuffix(code, "*")
		}
		var permissions []gormPermission
		if err := db.Where("status = ?", StatusEnabled).Order("code").Find(&permissions).Error; err != nil {
			return repositoryError(err)
		}
		for _, permission := range permissions {
			if strings.Contains(permission.Code, "*") || !strings.HasPrefix(permission.Code, prefix) || !validPermissionResourceType(permission.ResourceType) {
				continue
			}
			deleted, err := isLifecycleDeleted(db, "permissions", permission.ID)
			if err != nil {
				return err
			}
			if !deleted {
				return nil
			}
		}
		return ErrInvalid
	}
	if strings.Contains(code, "*") {
		return ErrInvalid
	}
	var permission gormPermission
	if err := db.Where("code = ?", code).Take(&permission).Error; err != nil {
		return repositoryError(err)
	}
	deleted, err := isLifecycleDeleted(db, "permissions", permission.ID)
	if err != nil {
		return err
	}
	if deleted || permission.Status != StatusEnabled || !validPermissionResourceType(permission.ResourceType) {
		return ErrInvalid
	}
	return nil
}

func validateRolePolicy(db *gorm.DB, group gormRoleGroup, allow, deny []string, dataScope string, orgCodes, areaCodes []string) error {
	allowed := make(map[string]struct{}, len(allow))
	for _, code := range allow {
		allowed[code] = struct{}{}
	}
	for _, code := range deny {
		if _, overlap := allowed[code]; overlap {
			return &ValidationError{Field: "role.permissions", Reason: "allow and deny permissions must not overlap"}
		}
	}
	switch dataScope {
	case "all", "current_org", "current_and_children", "current_area", "current_and_children_areas", "self":
		if len(orgCodes) != 0 || len(areaCodes) != 0 {
			return &ValidationError{Field: "role.dataScope", Reason: "selected data scope does not accept explicit codes"}
		}
	case "custom_orgs":
		if len(orgCodes) == 0 || len(areaCodes) != 0 {
			return &ValidationError{Field: "role.dataScopeOrgCodes", Reason: "custom organization scope requires only organization codes"}
		}
		for _, code := range orgCodes {
			var organization gormOrganization
			if err := db.Where("code = ?", code).Take(&organization).Error; err != nil {
				return repositoryError(err)
			}
			deleted, err := isLifecycleDeleted(db, "org-units", organization.ID)
			if err != nil || deleted || organization.Status != StatusEnabled || group.TenantCode != "" && organization.TenantCode != group.TenantCode {
				return ErrInvalid
			}
		}
	case "custom_areas":
		if len(areaCodes) == 0 || len(orgCodes) != 0 {
			return &ValidationError{Field: "role.dataScopeAreaCodes", Reason: "custom area scope requires only area codes"}
		}
		for _, code := range areaCodes {
			var count int64
			if err := db.Table("platform_area_codes").Where("code = ? AND status = ?", code, StatusEnabled).Count(&count).Error; err != nil || count != 1 {
				return ErrInvalid
			}
		}
	default:
		return &ValidationError{Field: "role.dataScope", Reason: "is invalid"}
	}
	return nil
}

func loadRolePolicyValues(db *gorm.DB, roleCode string) (map[string]string, error) {
	var role gormRole
	if err := db.Select("values_json").Where("code = ?", roleCode).Take(&role).Error; err != nil {
		return nil, repositoryError(err)
	}
	if strings.TrimSpace(role.ValuesJSON) == "" {
		return map[string]string{}, nil
	}
	var values map[string]string
	if err := json.Unmarshal([]byte(role.ValuesJSON), &values); err != nil || values == nil {
		return nil, ErrInvalid
	}
	return values, nil
}

func validPermissionResourceType(resourceType string) bool {
	switch strings.TrimSpace(resourceType) {
	case PermissionResourceTypeAPI, PermissionResourceTypePageButton:
		return true
	default:
		return false
	}
}

func permissionDiffCounts(current, target []string) (int, int) {
	currentSet := make(map[string]struct{}, len(current))
	targetSet := make(map[string]struct{}, len(target))
	for _, code := range current {
		currentSet[code] = struct{}{}
	}
	for _, code := range target {
		targetSet[code] = struct{}{}
	}
	added := 0
	for code := range targetSet {
		if _, exists := currentSet[code]; !exists {
			added++
		}
	}
	removed := 0
	for code := range currentSet {
		if _, exists := targetSet[code]; !exists {
			removed++
		}
	}
	return added, removed
}
