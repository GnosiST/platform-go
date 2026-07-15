package organizationrbac

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	PermissionResourceTypeAPI        = "api"
	PermissionResourceTypePageButton = "page-button"
)

type RolePermissionImpact struct {
	RoleCode                string
	TenantCode              string
	CurrentPermissionCodes  []string
	ProposedPermissionCodes []string
	AddedCount              int
	RemovedCount            int
	ExpectedRevision        uint64
}

type ReplaceRolePermissionsRequest struct {
	RoleCode         string
	PermissionCodes  []string
	ExpectedRevision uint64
	ActorID          string
	ChangedAt        time.Time
}

func (r *GORMRepository) PreviewRolePermissions(ctx context.Context, roleCode string, permissionCodes []string) (RolePermissionImpact, error) {
	if !r.ready(ctx) || !validCode(roleCode) {
		return RolePermissionImpact{}, ErrInvalid
	}
	target, err := canonicalCodes(permissionCodes)
	if err != nil {
		return RolePermissionImpact{}, err
	}
	return previewRolePermissions(r.db.WithContext(ctx), roleCode, target)
}

func (r *GORMRepository) ReplaceRolePermissions(ctx context.Context, request ReplaceRolePermissionsRequest) (uint64, string, error) {
	if !r.ready(ctx) || !validCode(request.RoleCode) || !validCode(request.ActorID) || request.ChangedAt.IsZero() {
		return 0, "", ErrInvalid
	}
	target, err := canonicalCodes(request.PermissionCodes)
	if err != nil {
		return 0, "", err
	}
	var revision uint64
	var tenantCode string
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		impact, err := previewRolePermissions(tx, request.RoleCode, target)
		if err != nil {
			return err
		}
		if impact.ExpectedRevision != request.ExpectedRevision {
			return &RevisionConflictError{Expected: request.ExpectedRevision, Actual: impact.ExpectedRevision}
		}
		if err := tx.Where("role_code = ?", request.RoleCode).Delete(&gormRolePermission{}).Error; err != nil {
			return repositoryError(err)
		}
		for _, permissionCode := range target {
			if err := tx.Create(&gormRolePermission{RoleCode: request.RoleCode, Permission: permissionCode}).Error; err != nil {
				return repositoryError(err)
			}
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

func previewRolePermissions(db *gorm.DB, roleCode string, target []string) (RolePermissionImpact, error) {
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
	for _, permissionCode := range target {
		var permission gormPermission
		if err := db.Where("code = ?", permissionCode).Take(&permission).Error; err != nil {
			return RolePermissionImpact{}, repositoryError(err)
		}
		permissionDeleted, err := isLifecycleDeleted(db, "permissions", permission.ID)
		if err != nil {
			return RolePermissionImpact{}, err
		}
		if permissionDeleted || permission.Status != StatusEnabled || !validPermissionResourceType(permission.ResourceType) {
			return RolePermissionImpact{}, ErrInvalid
		}
	}
	var rows []gormRolePermission
	if err := db.Where("role_code = ?", roleCode).Order("permission").Find(&rows).Error; err != nil {
		return RolePermissionImpact{}, repositoryError(err)
	}
	current := make([]string, 0, len(rows))
	for _, row := range rows {
		current = append(current, row.Permission)
	}
	current, err = canonicalCodes(current)
	if err != nil {
		return RolePermissionImpact{}, err
	}
	revision, err := loadGlobalRevision(db)
	if err != nil {
		return RolePermissionImpact{}, err
	}
	added, removed := permissionDiffCounts(current, target)
	return RolePermissionImpact{
		RoleCode: roleCode, TenantCode: group.TenantCode, CurrentPermissionCodes: current, ProposedPermissionCodes: target,
		AddedCount: added, RemovedCount: removed, ExpectedRevision: revision,
	}, nil
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
