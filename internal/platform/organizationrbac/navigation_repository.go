package organizationrbac

import (
	"context"
	"sort"
	"strings"

	"platform-go/internal/platform/rbac"

	"gorm.io/gorm"
)

type RoleMenuCandidateComparison struct {
	RoleCode                    string
	LegacyMenuCodes             []string
	TargetMenuCodes             []string
	AddedMenuCodes              []string
	RemovedMenuCodes            []string
	TargetRevision              uint64
	PrincipalEquivalenceClaimed bool
}

type ResolvedRoleMenus struct {
	Nodes                       []MenuNode
	Revision                    uint64
	PrincipalEquivalenceClaimed bool
}

func (r *GORMRepository) CompareRoleMenuCandidate(ctx context.Context, roleCode string) (RoleMenuCandidateComparison, error) {
	if !r.ready(ctx) || !validCode(roleCode) {
		return RoleMenuCandidateComparison{}, ErrInvalid
	}
	db := r.db.WithContext(ctx)
	legacy, err := legacyRoleMenuCandidate(db, roleCode)
	if err != nil {
		return RoleMenuCandidateComparison{}, err
	}
	target, err := loadRoleMenus(db, roleCode)
	if err != nil {
		return RoleMenuCandidateComparison{}, err
	}
	added, removed := menuCodeDiff(legacy, target.MenuCodes)
	return RoleMenuCandidateComparison{
		RoleCode: roleCode, LegacyMenuCodes: legacy, TargetMenuCodes: target.MenuCodes,
		AddedMenuCodes: added, RemovedMenuCodes: removed, TargetRevision: target.Revision,
		PrincipalEquivalenceClaimed: false,
	}, nil
}

func legacyRoleMenuCandidate(db *gorm.DB, roleCode string) ([]string, error) {
	var role gormRole
	if err := db.Where("code = ?", roleCode).Take(&role).Error; err != nil {
		return nil, repositoryError(err)
	}
	deleted, err := isLifecycleDeleted(db, "roles", role.ID)
	if err != nil || deleted || role.Status != StatusEnabled {
		return nil, ErrInvalid
	}
	var allowRows []gormRolePermission
	if err := db.Where("role_code = ?", roleCode).Order("permission").Find(&allowRows).Error; err != nil {
		return nil, repositoryError(err)
	}
	allow := make([]string, 0, len(allowRows))
	for _, row := range allowRows {
		allow = append(allow, row.Permission)
	}
	values, err := loadRolePolicyValues(db, roleCode)
	if err != nil {
		return nil, err
	}
	policy := rbac.NewPolicySetWithDeny(allow, rbac.ParsePermissionList(values["denyPermissions"]))
	wildcard := policy.Allows("*")
	deletedMenus, err := deletedRecordIDs(db, "menus")
	if err != nil {
		return nil, err
	}
	deletedPermissions, err := deletedRecordIDs(db, "permissions")
	if err != nil {
		return nil, err
	}
	var permissions []gormPermission
	if err := db.Where("status = ?", StatusEnabled).Find(&permissions).Error; err != nil {
		return nil, repositoryError(err)
	}
	enabledPermissions := make(map[string]struct{}, len(permissions))
	for _, permission := range permissions {
		if _, deleted := deletedPermissions[permission.ID]; !deleted {
			enabledPermissions[permission.Code] = struct{}{}
		}
	}
	var menus []gormMenu
	if err := db.Where("status = ? AND node_type = ?", StatusEnabled, MenuNodeTypePage).Order("code").Find(&menus).Error; err != nil {
		return nil, repositoryError(err)
	}
	result := make([]string, 0, len(menus))
	for _, menu := range menus {
		if _, deleted := deletedMenus[menu.ID]; deleted {
			continue
		}
		permission := strings.TrimSpace(menu.LegacyPermission)
		if wildcard {
			if permission == "" || policy.Allows(permission) {
				result = append(result, menu.Code)
			}
			continue
		}
		if permission == "" || !policy.Allows(permission) {
			continue
		}
		if _, enabled := enabledPermissions[permission]; enabled {
			result = append(result, menu.Code)
		}
	}
	return result, nil
}

func (r *GORMRepository) ResolveRoleMenuNodes(ctx context.Context, roleCodes []string) (ResolvedRoleMenus, error) {
	if !r.ready(ctx) {
		return ResolvedRoleMenus{}, ErrInvalid
	}
	codes, err := canonicalCodes(roleCodes)
	if err != nil || len(codes) > MaximumRoleMenuSelections {
		return ResolvedRoleMenus{}, ErrInvalid
	}
	db := r.db.WithContext(ctx)
	deletedRoles, err := deletedRecordIDs(db, "roles")
	if err != nil {
		return ResolvedRoleMenus{}, err
	}
	var roles []gormRole
	if len(codes) > 0 {
		if err := db.Where("code IN ? AND status = ?", codes, StatusEnabled).Find(&roles).Error; err != nil {
			return ResolvedRoleMenus{}, repositoryError(err)
		}
	}
	activeRoles := make([]string, 0, len(roles))
	for _, role := range roles {
		if _, deleted := deletedRoles[role.ID]; !deleted {
			activeRoles = append(activeRoles, role.Code)
		}
	}
	var bindings []gormRoleMenu
	if len(activeRoles) > 0 {
		if err := db.Where("role_code IN ?", activeRoles).Order("menu_code").Find(&bindings).Error; err != nil {
			return ResolvedRoleMenus{}, repositoryError(err)
		}
	}
	var rows []gormMenu
	if err := db.Order("code").Find(&rows).Error; err != nil {
		return ResolvedRoleMenus{}, repositoryError(err)
	}
	deletedMenus, err := deletedRecordIDs(db, "menus")
	if err != nil {
		return ResolvedRoleMenus{}, err
	}
	byCode := make(map[string]gormMenu, len(rows))
	for _, row := range rows {
		if _, deleted := deletedMenus[row.ID]; !deleted && row.Status == StatusEnabled {
			byCode[row.Code] = row
		}
	}
	selected := make(map[string]struct{})
	for _, binding := range bindings {
		page, exists := byCode[binding.MenuCode]
		if !exists || MenuNodeType(page.NodeType) != MenuNodeTypePage {
			continue
		}
		closure, ok := activeMenuClosure(byCode, page)
		if !ok {
			continue
		}
		for _, code := range closure {
			selected[code] = struct{}{}
		}
	}
	selectedCodes := make([]string, 0, len(selected))
	for code := range selected {
		selectedCodes = append(selectedCodes, code)
	}
	sort.Slice(selectedCodes, func(i, j int) bool {
		left, right := byCode[selectedCodes[i]], byCode[selectedCodes[j]]
		if left.SortOrder != right.SortOrder {
			return left.SortOrder < right.SortOrder
		}
		return selectedCodes[i] < selectedCodes[j]
	})
	nodes := make([]MenuNode, 0, len(selectedCodes))
	for _, code := range selectedCodes {
		node, err := menuNodeFromGORM(byCode[code])
		if err != nil {
			return ResolvedRoleMenus{}, err
		}
		nodes = append(nodes, node)
	}
	revision, err := loadGlobalRevision(db)
	if err != nil {
		return ResolvedRoleMenus{}, err
	}
	return ResolvedRoleMenus{Nodes: nodes, Revision: revision, PrincipalEquivalenceClaimed: false}, nil
}

func activeMenuClosure(byCode map[string]gormMenu, page gormMenu) ([]string, bool) {
	result := []string{page.Code}
	seen := map[string]struct{}{page.Code: {}}
	parentCode := strings.TrimSpace(page.ParentCode)
	for parentCode != "" {
		if _, duplicate := seen[parentCode]; duplicate {
			return nil, false
		}
		parent, exists := byCode[parentCode]
		if !exists || MenuNodeType(parent.NodeType) != MenuNodeTypeDirectory {
			return nil, false
		}
		seen[parentCode] = struct{}{}
		result = append(result, parentCode)
		parentCode = strings.TrimSpace(parent.ParentCode)
	}
	return result, true
}

func deletedRecordIDs(db *gorm.DB, resource string) (map[string]struct{}, error) {
	var ids []string
	if err := db.Model(&gormResourceLifecycle{}).
		Where("resource = ? AND deleted_at <> ''", resource).
		Pluck("record_id", &ids).Error; err != nil {
		return nil, repositoryError(err)
	}
	return stringSet(ids), nil
}

func menuCodeDiff(legacy, target []string) ([]string, []string) {
	legacySet := stringSet(legacy)
	targetSet := stringSet(target)
	added := make([]string, 0)
	removed := make([]string, 0)
	for _, code := range target {
		if _, exists := legacySet[code]; !exists {
			added = append(added, code)
		}
	}
	for _, code := range legacy {
		if _, exists := targetSet[code]; !exists {
			removed = append(removed, code)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	return added, removed
}
