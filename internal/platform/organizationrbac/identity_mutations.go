package organizationrbac

import (
	"context"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	roleOperationMove    = "move"
	roleOperationDisable = "disable"
)

func (r *GORMRepository) PreviewUserOrganizationChange(ctx context.Context, userCode, orgUnitCode string, roleCodes []string) (UserOrganizationChangeImpact, error) {
	if !r.ready(ctx) || !validCode(userCode) || !validCode(orgUnitCode) {
		return UserOrganizationChangeImpact{}, ErrInvalid
	}
	targetRoles, err := canonicalCodes(roleCodes)
	if err != nil {
		return UserOrganizationChangeImpact{}, err
	}
	userRow, currentRoles, err := loadUserWithRoles(r.db.WithContext(ctx), userCode)
	if err != nil {
		return UserOrganizationChangeImpact{}, err
	}
	deleted, err := isLifecycleDeleted(r.db.WithContext(ctx), "users", userRow.ID)
	if err != nil {
		return UserOrganizationChangeImpact{}, err
	}
	organization, groups, roles, bindings, err := r.loadOrganizationState(r.db.WithContext(ctx), orgUnitCode, nil)
	if err != nil {
		return UserOrganizationChangeImpact{}, err
	}
	derived, err := DeriveAndValidateUser(User{
		ID: userRow.ID, Code: userRow.Code, ScopeType: ScopeTenant, TenantCode: userRow.TenantCode,
		OrgUnitCode: orgUnitCode, Status: userRow.Status, Deleted: deleted,
	}, targetRoles, map[string]Organization{organization.Code: organization}, groups, roles, bindings)
	if err != nil {
		return UserOrganizationChangeImpact{}, err
	}
	pool, err := EffectiveRolePool(organization, groups, roles, bindings)
	if err != nil {
		return UserOrganizationChangeImpact{}, err
	}
	allowed := make(map[string]struct{}, len(pool))
	for _, entry := range pool {
		allowed[entry.RoleCode] = struct{}{}
	}
	conflicts := make([]RoleAssignmentConflict, 0)
	for _, roleCode := range currentRoles {
		if _, ok := allowed[roleCode]; !ok {
			conflicts = append(conflicts, RoleAssignmentConflict{UserCode: userCode, RoleCode: roleCode})
		}
	}
	revision, err := loadGlobalRevision(r.db.WithContext(ctx))
	if err != nil {
		return UserOrganizationChangeImpact{}, err
	}
	return UserOrganizationChangeImpact{
		UserCode: userCode, CurrentOrgUnitCode: userRow.OrgUnitCode, TargetOrgUnitCode: orgUnitCode,
		TargetTenantCode: derived.TenantCode, CurrentRoleCodes: currentRoles, TargetRoleCodes: targetRoles,
		Conflicts: conflicts, ExpectedRevision: revision,
	}, nil
}

func (r *GORMRepository) ChangeUserOrganization(ctx context.Context, request ChangeUserOrganizationRequest, remediations []RoleAssignmentRemediation) (uint64, error) {
	if !r.ready(ctx) || !validCode(request.UserCode) || !validCode(request.OrgUnitCode) || !validCode(request.ActorID) || request.ChangedAt.IsZero() {
		return 0, ErrInvalid
	}
	request.ChangedAt = request.ChangedAt.UTC()
	var committedRevision uint64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repository := &GORMRepository{db: tx}
		impact, err := repository.PreviewUserOrganizationChange(ctx, request.UserCode, request.OrgUnitCode, request.RoleCodes)
		if err != nil {
			return err
		}
		if impact.ExpectedRevision != request.ExpectedRevision {
			return &RevisionConflictError{Expected: request.ExpectedRevision, Actual: impact.ExpectedRevision}
		}
		if err := validateUserChangeRemediations(impact, remediations); err != nil {
			return err
		}
		var user gormUser
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("code = ?", request.UserCode).Take(&user).Error; err != nil {
			return repositoryError(err)
		}
		roles, err := canonicalCodes(request.RoleCodes)
		if err != nil {
			return err
		}
		if err := tx.Model(&gormUser{}).Where("id = ?", user.ID).Updates(map[string]any{
			"scope_type": string(ScopeTenant), "tenant_code": impact.TargetTenantCode,
			"org_unit_code": request.OrgUnitCode, "updated_at": request.ChangedAt.Format(time.RFC3339),
		}).Error; err != nil {
			return repositoryError(err)
		}
		if err := tx.Where("user_id = ?", user.ID).Delete(&gormUserRole{}).Error; err != nil {
			return repositoryError(err)
		}
		assignments := make([]gormUserRole, 0, len(roles))
		for _, roleCode := range roles {
			assignments = append(assignments, gormUserRole{UserID: user.ID, RoleCode: roleCode})
		}
		if len(assignments) > 0 {
			if err := tx.Create(&assignments).Error; err != nil {
				return repositoryError(err)
			}
		}
		committedRevision, err = advanceGlobalRevision(tx, request.ExpectedRevision)
		return err
	})
	if err != nil {
		return 0, repositoryError(err)
	}
	return committedRevision, nil
}

func (r *GORMRepository) PreviewRoleMove(ctx context.Context, roleCode, targetGroupCode string) (RoleStateOrGroupImpact, error) {
	return r.previewRoleStateOrGroupChange(ctx, roleOperationMove, roleCode, targetGroupCode)
}

func (r *GORMRepository) PreviewRoleDisable(ctx context.Context, roleCode string) (RoleStateOrGroupImpact, error) {
	return r.previewRoleStateOrGroupChange(ctx, roleOperationDisable, roleCode, "")
}

func (r *GORMRepository) MoveRole(ctx context.Context, request ChangeRoleRequest, remediations []RoleAssignmentRemediation) (uint64, error) {
	return r.applyRoleStateOrGroupChange(ctx, roleOperationMove, request, remediations)
}

func (r *GORMRepository) DisableRole(ctx context.Context, request ChangeRoleRequest, remediations []RoleAssignmentRemediation) (uint64, error) {
	return r.applyRoleStateOrGroupChange(ctx, roleOperationDisable, request, remediations)
}

func (r *GORMRepository) previewRoleStateOrGroupChange(ctx context.Context, operation, roleCode, targetGroupCode string) (RoleStateOrGroupImpact, error) {
	if !r.ready(ctx) || !validCode(roleCode) || operation == roleOperationMove && !validCode(targetGroupCode) {
		return RoleStateOrGroupImpact{}, ErrInvalid
	}
	db := r.db.WithContext(ctx)
	var role gormRole
	if err := db.Where("code = ?", roleCode).Take(&role).Error; err != nil {
		return RoleStateOrGroupImpact{}, repositoryError(err)
	}
	deleted, err := isLifecycleDeleted(db, "roles", role.ID)
	if err != nil {
		return RoleStateOrGroupImpact{}, err
	}
	if deleted || role.Status != StatusEnabled {
		return RoleStateOrGroupImpact{}, ErrInvalid
	}
	var currentGroup gormRoleGroup
	if err := db.Where("code = ?", role.GroupCode).Take(&currentGroup).Error; err != nil {
		return RoleStateOrGroupImpact{}, repositoryError(err)
	}
	targetGroup := currentGroup
	if operation == roleOperationMove {
		if targetGroupCode == role.GroupCode {
			return RoleStateOrGroupImpact{}, ErrInvalid
		}
		targetGroup = gormRoleGroup{}
		if err := db.Where("code = ?", targetGroupCode).Take(&targetGroup).Error; err != nil {
			return RoleStateOrGroupImpact{}, repositoryError(err)
		}
		targetDeleted, err := isLifecycleDeleted(db, "role-groups", targetGroup.ID)
		if err != nil {
			return RoleStateOrGroupImpact{}, err
		}
		if targetDeleted || targetGroup.Status != StatusEnabled || targetGroup.ScopeType != currentGroup.ScopeType || targetGroup.TenantCode != currentGroup.TenantCode {
			return RoleStateOrGroupImpact{}, ErrRolePoolViolation
		}
	}
	conflicts, affected, err := roleAssignmentImpact(db, operation, roleCode, targetGroup)
	if err != nil {
		return RoleStateOrGroupImpact{}, err
	}
	revision, err := loadGlobalRevision(db)
	if err != nil {
		return RoleStateOrGroupImpact{}, err
	}
	return RoleStateOrGroupImpact{
		RoleCode: roleCode, Operation: operation, CurrentGroupCode: role.GroupCode, TargetGroupCode: targetGroupCode,
		TenantCode: currentGroup.TenantCode, Conflicts: conflicts, AffectedUsers: affected, ExpectedRevision: revision,
	}, nil
}

func (r *GORMRepository) applyRoleStateOrGroupChange(ctx context.Context, operation string, request ChangeRoleRequest, remediations []RoleAssignmentRemediation) (uint64, error) {
	if !r.ready(ctx) || !validCode(request.RoleCode) || !validCode(request.ActorID) || request.ChangedAt.IsZero() || operation == roleOperationMove && !validCode(request.TargetGroupCode) {
		return 0, ErrInvalid
	}
	var committedRevision uint64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repository := &GORMRepository{db: tx}
		var impact RoleStateOrGroupImpact
		var err error
		if operation == roleOperationMove {
			impact, err = repository.PreviewRoleMove(ctx, request.RoleCode, request.TargetGroupCode)
		} else {
			impact, err = repository.PreviewRoleDisable(ctx, request.RoleCode)
		}
		if err != nil {
			return err
		}
		if impact.ExpectedRevision != request.ExpectedRevision {
			return &RevisionConflictError{Expected: request.ExpectedRevision, Actual: impact.ExpectedRevision}
		}
		if err := applyConflictRemediations(tx, impact.Conflicts, remediations); err != nil {
			return err
		}
		if operation == roleOperationMove {
			remaining, _, err := roleAssignmentImpact(tx, operation, request.RoleCode, gormRoleGroup{Code: request.TargetGroupCode, ScopeType: scopeFromTenant(impact.TenantCode), TenantCode: impact.TenantCode, Status: StatusEnabled})
			if err != nil || len(remaining) != 0 {
				return ErrRolePoolViolation
			}
			if err := tx.Model(&gormRole{}).Where("code = ?", request.RoleCode).Updates(map[string]any{
				"group_code": request.TargetGroupCode, "updated_at": request.ChangedAt.Format(time.RFC3339),
			}).Error; err != nil {
				return repositoryError(err)
			}
		} else {
			var count int64
			if err := tx.Model(&gormUserRole{}).Where("role_code = ?", request.RoleCode).Count(&count).Error; err != nil || count != 0 {
				return ErrRolePoolViolation
			}
			if err := tx.Model(&gormRole{}).Where("code = ?", request.RoleCode).Updates(map[string]any{
				"status": "disabled", "updated_at": request.ChangedAt.Format(time.RFC3339),
			}).Error; err != nil {
				return repositoryError(err)
			}
		}
		committedRevision, err = advanceGlobalRevision(tx, request.ExpectedRevision)
		return err
	})
	if err != nil {
		return 0, repositoryError(err)
	}
	return committedRevision, nil
}

func loadUserWithRoles(db *gorm.DB, userCode string) (gormUser, []string, error) {
	var user gormUser
	if err := db.Where("code = ?", userCode).Take(&user).Error; err != nil {
		return gormUser{}, nil, repositoryError(err)
	}
	var rows []gormUserRole
	if err := db.Where("user_id = ?", user.ID).Order("role_code").Find(&rows).Error; err != nil {
		return gormUser{}, nil, repositoryError(err)
	}
	roles := make([]string, 0, len(rows))
	for _, row := range rows {
		roles = append(roles, row.RoleCode)
	}
	return user, roles, nil
}

func validateUserChangeRemediations(impact UserOrganizationChangeImpact, remediations []RoleAssignmentRemediation) error {
	conflicts := conflictSet(impact.Conflicts)
	seen := make(map[string]struct{}, len(remediations))
	target := make(map[string]struct{}, len(impact.TargetRoleCodes))
	for _, code := range impact.TargetRoleCodes {
		target[code] = struct{}{}
	}
	for _, remediation := range remediations {
		key := remediation.UserCode + "\x00" + remediation.RoleCode
		if _, ok := conflicts[key]; !ok || remediation.UserCode != impact.UserCode {
			return ErrInvalid
		}
		if _, duplicate := seen[key]; duplicate {
			return ErrInvalid
		}
		seen[key] = struct{}{}
		if _, retained := target[remediation.RoleCode]; retained {
			return ErrRolePoolViolation
		}
		switch remediation.Action {
		case "remove-role":
			if remediation.ReplacementRoleCode != "" {
				return ErrInvalid
			}
		case "replace-role":
			if _, ok := target[remediation.ReplacementRoleCode]; !ok {
				return ErrRolePoolViolation
			}
		default:
			return ErrInvalid
		}
	}
	if len(seen) != len(conflicts) {
		return ErrRolePoolViolation
	}
	return nil
}

func roleAssignmentImpact(db *gorm.DB, operation, roleCode string, targetGroup gormRoleGroup) ([]RoleAssignmentConflict, int, error) {
	var assignments []gormUserRole
	if err := db.Where("role_code = ?", roleCode).Find(&assignments).Error; err != nil {
		return nil, 0, repositoryError(err)
	}
	if len(assignments) == 0 {
		return nil, 0, nil
	}
	userIDs := make([]string, 0, len(assignments))
	for _, assignment := range assignments {
		userIDs = append(userIDs, assignment.UserID)
	}
	var users []gormUser
	if err := db.Where("id IN ?", userIDs).Find(&users).Error; err != nil {
		return nil, 0, repositoryError(err)
	}
	byID := make(map[string]gormUser, len(users))
	for _, user := range users {
		byID[user.ID] = user
	}
	conflicts := make([]RoleAssignmentConflict, 0)
	affected := make(map[string]struct{}, len(assignments))
	for _, assignment := range assignments {
		user, ok := byID[assignment.UserID]
		if !ok {
			return nil, 0, ErrRolePoolViolation
		}
		affected[user.Code] = struct{}{}
		conflict := operation == roleOperationDisable
		if operation == roleOperationMove && !conflict {
			switch ScopeType(user.ScopeType) {
			case ScopePlatform:
				conflict = ScopeType(targetGroup.ScopeType) != ScopePlatform
			case ScopeTenant:
				if ScopeType(targetGroup.ScopeType) != ScopeTenant || targetGroup.TenantCode != user.TenantCode {
					conflict = true
				} else {
					var count int64
					if err := db.Model(&gormOrgUnitRoleGroup{}).Where("org_unit_code = ? AND role_group_code = ?", user.OrgUnitCode, targetGroup.Code).Count(&count).Error; err != nil {
						return nil, 0, repositoryError(err)
					}
					conflict = count != 1
				}
			default:
				conflict = true
			}
		}
		if conflict {
			conflicts = append(conflicts, RoleAssignmentConflict{UserCode: user.Code, RoleCode: roleCode})
		}
	}
	sort.Slice(conflicts, func(i, j int) bool { return conflicts[i].UserCode < conflicts[j].UserCode })
	return conflicts, len(affected), nil
}

func applyConflictRemediations(db *gorm.DB, conflicts []RoleAssignmentConflict, remediations []RoleAssignmentRemediation) error {
	if err := validateConflictRemediationPlan(db, conflicts, remediations); err != nil {
		return err
	}
	for _, remediation := range remediations {
		var user gormUser
		if err := db.Where("code = ?", remediation.UserCode).Take(&user).Error; err != nil {
			return repositoryError(err)
		}
		if err := db.Where("user_id = ? AND role_code = ?", user.ID, remediation.RoleCode).Delete(&gormUserRole{}).Error; err != nil {
			return repositoryError(err)
		}
		if remediation.Action == "replace-role" {
			if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&gormUserRole{UserID: user.ID, RoleCode: remediation.ReplacementRoleCode}).Error; err != nil {
				return repositoryError(err)
			}
		}
	}
	return nil
}

func validateConflictRemediationPlan(db *gorm.DB, conflicts []RoleAssignmentConflict, remediations []RoleAssignmentRemediation) error {
	targets := conflictSet(conflicts)
	seen := make(map[string]struct{}, len(remediations))
	for _, remediation := range remediations {
		key := remediation.UserCode + "\x00" + remediation.RoleCode
		if _, ok := targets[key]; !ok {
			return ErrInvalid
		}
		if _, duplicate := seen[key]; duplicate {
			return ErrInvalid
		}
		seen[key] = struct{}{}
		user, roles, err := loadUserWithRoles(db, remediation.UserCode)
		if err != nil {
			return err
		}
		resulting := make([]string, 0, len(roles))
		found := false
		for _, roleCode := range roles {
			if roleCode == remediation.RoleCode {
				found = true
				continue
			}
			resulting = append(resulting, roleCode)
		}
		if !found {
			return ErrRolePoolViolation
		}
		switch remediation.Action {
		case "remove-role":
			if remediation.ReplacementRoleCode != "" {
				return ErrInvalid
			}
		case "replace-role":
			if !validCode(remediation.ReplacementRoleCode) || remediation.ReplacementRoleCode == remediation.RoleCode {
				return ErrInvalid
			}
			resulting = append(resulting, remediation.ReplacementRoleCode)
		default:
			return ErrInvalid
		}
		resulting, err = canonicalCodes(resulting)
		if err != nil {
			return err
		}
		userDeleted, err := isLifecycleDeleted(db, "users", user.ID)
		if err != nil {
			return err
		}
		if _, err := (&GORMRepository{db: db}).DeriveAndValidateUser(context.Background(), User{
			ID: user.ID, Code: user.Code, ScopeType: ScopeType(user.ScopeType), TenantCode: user.TenantCode,
			OrgUnitCode: user.OrgUnitCode, Status: user.Status, Deleted: userDeleted,
		}, resulting); err != nil {
			return err
		}
	}
	if len(seen) != len(targets) {
		return ErrRolePoolViolation
	}
	return nil
}

func conflictSet(conflicts []RoleAssignmentConflict) map[string]struct{} {
	result := make(map[string]struct{}, len(conflicts))
	for _, conflict := range conflicts {
		result[conflict.UserCode+"\x00"+conflict.RoleCode] = struct{}{}
	}
	return result
}

func scopeFromTenant(tenant string) string {
	if strings.TrimSpace(tenant) == "" {
		return string(ScopePlatform)
	}
	return string(ScopeTenant)
}
