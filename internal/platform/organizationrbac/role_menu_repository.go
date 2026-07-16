package organizationrbac

import (
	"context"
	"errors"
	"reflect"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *GORMRepository) LoadRoleMenus(ctx context.Context, roleCode string) (RoleMenuSet, error) {
	if !r.ready(ctx) || !validCode(roleCode) {
		return RoleMenuSet{}, ErrInvalid
	}
	return loadRoleMenus(r.db.WithContext(ctx), roleCode)
}

func (r *GORMRepository) PreviewRoleMenus(ctx context.Context, roleCode string, menuCodes []string) (RoleMenuImpact, error) {
	if !r.ready(ctx) || !validCode(roleCode) {
		return RoleMenuImpact{}, ErrInvalid
	}
	codes, err := canonicalRoleMenuCodes(menuCodes)
	if err != nil {
		return RoleMenuImpact{}, err
	}
	db := r.db.WithContext(ctx)
	tenantCode, err := validateRoleMenuTarget(db, roleCode, codes)
	if err != nil {
		return RoleMenuImpact{}, err
	}
	current, err := loadRoleMenus(db, roleCode)
	if err != nil {
		return RoleMenuImpact{}, err
	}
	return RoleMenuImpact{RoleCode: roleCode, TenantCode: tenantCode, CurrentMenuCodes: current.MenuCodes, ProposedMenuCodes: codes, ExpectedRevision: current.Revision, Changed: !reflect.DeepEqual(current.MenuCodes, codes)}, nil
}

func (r *GORMRepository) ReplaceRoleMenus(ctx context.Context, request ReplaceRoleMenusRequest) (RoleMenuSet, error) {
	if !r.ready(ctx) {
		return RoleMenuSet{}, ErrInvalid
	}
	var committed RoleMenuSet
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		committed, err = replaceRoleMenusTx(tx, request, true)
		return err
	})
	return committed, repositoryError(err)
}

func replaceRoleMenusTx(tx *gorm.DB, request ReplaceRoleMenusRequest, bumpGlobal bool) (RoleMenuSet, error) {
	if tx == nil || !validCode(request.RoleCode) || !validCode(request.ActorID) || request.ChangedAt.IsZero() {
		return RoleMenuSet{}, ErrInvalid
	}
	codes, err := canonicalRoleMenuCodes(request.MenuCodes)
	if err != nil {
		return RoleMenuSet{}, err
	}
	request.ChangedAt = request.ChangedAt.UTC()
	if _, err := validateRoleMenuTarget(tx, request.RoleCode, codes); err != nil {
		return RoleMenuSet{}, err
	}
	current, err := loadRoleMenus(tx, request.RoleCode)
	if err != nil {
		return RoleMenuSet{}, err
	}
	if current.Revision != request.ExpectedRevision {
		return RoleMenuSet{}, &RevisionConflictError{Expected: request.ExpectedRevision, Actual: current.Revision}
	}
	if reflect.DeepEqual(current.MenuCodes, codes) {
		return current, nil
	}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&gormRoleMenuRevision{RoleCode: request.RoleCode, Revision: 0, UpdatedAt: request.ChangedAt}).Error; err != nil {
		return RoleMenuSet{}, repositoryError(err)
	}
	next := current.Revision + 1
	result := tx.Model(&gormRoleMenuRevision{}).Where("role_code = ? AND revision = ?", request.RoleCode, current.Revision).
		Updates(map[string]any{"revision": next, "updated_at": request.ChangedAt})
	if result.Error != nil {
		return RoleMenuSet{}, repositoryError(result.Error)
	}
	if result.RowsAffected != 1 {
		actual, loadErr := loadRoleMenuRevision(tx, request.RoleCode)
		if loadErr != nil {
			return RoleMenuSet{}, loadErr
		}
		return RoleMenuSet{}, &RevisionConflictError{Expected: current.Revision, Actual: actual}
	}
	currentSet := stringSet(current.MenuCodes)
	targetSet := stringSet(codes)
	for _, code := range current.MenuCodes {
		if _, keep := targetSet[code]; keep {
			continue
		}
		if err := tx.Where("role_code = ? AND menu_code = ?", request.RoleCode, code).Delete(&gormRoleMenu{}).Error; err != nil {
			return RoleMenuSet{}, repositoryError(err)
		}
	}
	for _, code := range codes {
		if _, exists := currentSet[code]; exists {
			if err := tx.Model(&gormRoleMenu{}).Where("role_code = ? AND menu_code = ?", request.RoleCode, code).
				Updates(map[string]any{"revision": next, "actor_id": request.ActorID, "updated_at": request.ChangedAt}).Error; err != nil {
				return RoleMenuSet{}, repositoryError(err)
			}
			continue
		}
		if err := tx.Create(&gormRoleMenu{RoleCode: request.RoleCode, MenuCode: code, Revision: next, ActorID: request.ActorID, CreatedAt: request.ChangedAt, UpdatedAt: request.ChangedAt}).Error; err != nil {
			return RoleMenuSet{}, repositoryError(err)
		}
	}
	if bumpGlobal {
		if _, err := bumpGlobalRevision(tx); err != nil {
			return RoleMenuSet{}, err
		}
	}
	return RoleMenuSet{RoleCode: request.RoleCode, MenuCodes: codes, Revision: next}, nil
}

func loadRoleMenus(db *gorm.DB, roleCode string) (RoleMenuSet, error) {
	var rows []gormRoleMenu
	if err := db.Where("role_code = ?", roleCode).Order("menu_code").Find(&rows).Error; err != nil {
		return RoleMenuSet{}, repositoryError(err)
	}
	revision, err := loadRoleMenuRevision(db, roleCode)
	if err != nil {
		return RoleMenuSet{}, err
	}
	result := RoleMenuSet{RoleCode: roleCode, Revision: revision, MenuCodes: make([]string, 0, len(rows))}
	for _, row := range rows {
		result.MenuCodes = append(result.MenuCodes, row.MenuCode)
	}
	return result, nil
}

func loadRoleMenuRevision(db *gorm.DB, roleCode string) (uint64, error) {
	var row gormRoleMenuRevision
	err := db.Where("role_code = ?", roleCode).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, repositoryError(err)
	}
	return row.Revision, nil
}

func validateRoleMenuTarget(db *gorm.DB, roleCode string, menuCodes []string) (string, error) {
	var role gormRole
	if err := db.Where("code = ?", roleCode).Take(&role).Error; err != nil {
		return "", repositoryError(err)
	}
	deleted, err := isLifecycleDeleted(db, "roles", role.ID)
	if err != nil || deleted || role.Status != StatusEnabled {
		return "", ErrInvalid
	}
	var group gormRoleGroup
	if err := db.Where("code = ?", role.GroupCode).Take(&group).Error; err != nil {
		return "", repositoryError(err)
	}
	if err := ValidateRoleGroup(roleGroupFromGORM(group, false)); err != nil {
		return "", err
	}
	for _, code := range menuCodes {
		var menu gormMenu
		if err := db.Where("code = ?", code).Take(&menu).Error; err != nil {
			return "", repositoryError(err)
		}
		deleted, err := isLifecycleDeleted(db, "menus", menu.ID)
		if err != nil || deleted || menu.Status != StatusEnabled || MenuNodeType(menu.NodeType) != MenuNodeTypePage {
			return "", ErrInvalid
		}
	}
	return group.TenantCode, nil
}

func canonicalRoleMenuCodes(menuCodes []string) ([]string, error) {
	codes, err := canonicalCodes(menuCodes)
	if err != nil || len(codes) > MaximumRoleMenuSelections {
		return nil, ErrInvalid
	}
	return codes, nil
}

func stringSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}
