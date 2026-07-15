package organizationrbac

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

type resourceLifecycleState struct {
	impact    ResourceLifecycleImpact
	lifecycle gormResourceLifecycle
}

func (r *GORMRepository) PreviewResourceLifecycle(ctx context.Context, resource, resourceCode, operation string, now time.Time) (ResourceLifecycleImpact, error) {
	if !r.ready(ctx) || !validLifecycleResource(resource) || !validCode(resourceCode) || !validLifecycleOperation(operation) || now.IsZero() {
		return ResourceLifecycleImpact{}, ErrInvalid
	}
	state, err := loadResourceLifecycleState(r.db.WithContext(ctx), resource, resourceCode, now.UTC())
	if err != nil {
		return ResourceLifecycleImpact{}, err
	}
	switch operation {
	case LifecycleOperationDelete:
		if state.impact.Deleted {
			return ResourceLifecycleImpact{}, ErrInvalid
		}
	case LifecycleOperationRestore, LifecycleOperationPurge:
		if !state.impact.Deleted {
			return ResourceLifecycleImpact{}, ErrInvalid
		}
	case LifecycleOperationDisable:
		if resource != "role-groups" || state.impact.Deleted || state.impact.Status != StatusEnabled {
			return ResourceLifecycleImpact{}, ErrInvalid
		}
	}
	state.impact.Operation = operation
	revision, err := loadGlobalRevision(r.db.WithContext(ctx))
	if err != nil {
		return ResourceLifecycleImpact{}, err
	}
	state.impact.ExpectedRevision = revision
	return state.impact, nil
}

func (r *GORMRepository) ApplyResourceLifecycle(ctx context.Context, request ResourceLifecycleRequest, remediations []RoleAssignmentRemediation) (uint64, error) {
	if !r.ready(ctx) || !validLifecycleResource(request.Resource) || !validCode(request.ResourceCode) ||
		!validLifecycleOperation(request.Operation) || !validCode(request.ActorID) || request.ChangedAt.IsZero() {
		return 0, ErrInvalid
	}
	if request.Operation == LifecycleOperationDelete && (request.RetentionDays < 1 || request.RetentionDays > 36500 || request.PolicyVersion == 0) {
		return 0, ErrInvalid
	}
	if request.Operation != LifecycleOperationDelete && (request.RetentionDays != 0 || request.PolicyVersion != 0) {
		return 0, ErrInvalid
	}
	var committed uint64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repository := &GORMRepository{db: tx}
		impact, err := repository.PreviewResourceLifecycle(ctx, request.Resource, request.ResourceCode, request.Operation, request.ChangedAt)
		if err != nil {
			return err
		}
		if impact.ExpectedRevision != request.ExpectedRevision {
			return &RevisionConflictError{Expected: request.ExpectedRevision, Actual: impact.ExpectedRevision}
		}
		switch request.Operation {
		case LifecycleOperationDelete:
			if err := applyLifecycleDeleteRemediations(tx, impact, remediations); err != nil {
				return err
			}
			remaining, err := loadResourceLifecycleState(tx, request.Resource, request.ResourceCode, request.ChangedAt.UTC())
			if err != nil {
				return err
			}
			if remaining.impact.ReferenceCount != 0 {
				return ErrRolePoolViolation
			}
			if err := setLifecycleResourceStatus(tx, request.Resource, request.ResourceCode, "disabled", request.ChangedAt); err != nil {
				return err
			}
			lifecycle := gormResourceLifecycle{
				Resource: request.Resource, RecordID: impact.RecordID, DeletedAt: request.ChangedAt.UTC().Format(time.RFC3339),
				DeletedBy: request.ActorID, DeleteReason: LifecycleOperationDelete,
				PurgeAfter:            request.ChangedAt.UTC().Add(time.Duration(request.RetentionDays) * 24 * time.Hour).Format(time.RFC3339),
				DeletionPolicyVersion: request.PolicyVersion,
			}
			if err := tx.Create(&lifecycle).Error; err != nil {
				return repositoryError(err)
			}
		case LifecycleOperationRestore:
			if len(remediations) != 0 || impact.ReferenceCount != 0 {
				return ErrRolePoolViolation
			}
			if err := validateLifecycleRestore(tx, impact); err != nil {
				return err
			}
			if err := setLifecycleResourceStatus(tx, request.Resource, request.ResourceCode, "disabled", request.ChangedAt); err != nil {
				return err
			}
			result := tx.Where("resource = ? AND record_id = ?", request.Resource, impact.RecordID).Delete(&gormResourceLifecycle{})
			if result.Error != nil || result.RowsAffected != 1 {
				return ErrRepositoryFailed
			}
		case LifecycleOperationPurge:
			if len(remediations) != 0 || !impact.RetentionElapsed || impact.ReferenceCount != 0 {
				return ErrRolePoolViolation
			}
			if err := purgeLifecycleResource(tx, request.Resource, request.ResourceCode); err != nil {
				return err
			}
			result := tx.Where("resource = ? AND record_id = ?", request.Resource, impact.RecordID).Delete(&gormResourceLifecycle{})
			if result.Error != nil || result.RowsAffected != 1 {
				return ErrRepositoryFailed
			}
		case LifecycleOperationDisable:
			if err := validateLifecycleRemediationPlan(tx, impact, remediations); err != nil {
				return err
			}
			if err := applyConflictRemediations(tx, impact.Conflicts, remediations); err != nil {
				return err
			}
			var bindings []gormOrgUnitRoleGroup
			if err := tx.Where("role_group_code = ?", request.ResourceCode).Find(&bindings).Error; err != nil {
				return repositoryError(err)
			}
			if len(bindings) > 0 {
				orgUnitCodes := make([]string, 0, len(bindings))
				for _, binding := range bindings {
					orgUnitCodes = append(orgUnitCodes, binding.OrgUnitCode)
				}
				result := tx.Model(&gormOrgUnitRoleGroupRevision{}).
					Where("org_unit_code IN ?", orgUnitCodes).
					Updates(map[string]any{"revision": gorm.Expr("revision + ?", 1), "updated_at": request.ChangedAt.UTC()})
				if result.Error != nil {
					return repositoryError(result.Error)
				}
				if result.RowsAffected != int64(len(orgUnitCodes)) {
					return ErrRepositoryFailed
				}
			}
			if err := tx.Where("role_group_code = ?", request.ResourceCode).Delete(&gormOrgUnitRoleGroup{}).Error; err != nil {
				return repositoryError(err)
			}
			if err := setLifecycleResourceStatus(tx, request.Resource, request.ResourceCode, "disabled", request.ChangedAt); err != nil {
				return err
			}
		}
		committed, err = bumpGlobalRevision(tx)
		return err
	})
	if err != nil {
		return 0, err
	}
	return committed, nil
}

func loadResourceLifecycleState(db *gorm.DB, resource, code string, now time.Time) (resourceLifecycleState, error) {
	impact := ResourceLifecycleImpact{Resource: resource, ResourceCode: code}
	switch resource {
	case "org-units":
		var row gormOrganization
		if err := db.Where("code = ?", code).Take(&row).Error; err != nil {
			return resourceLifecycleState{}, lifecycleLookupError(err)
		}
		impact.RecordID, impact.TenantCode, impact.OrgUnitCode, impact.Status = row.ID, row.TenantCode, row.Code, row.Status
	case "role-groups":
		var row gormRoleGroup
		if err := db.Where("code = ?", code).Take(&row).Error; err != nil {
			return resourceLifecycleState{}, lifecycleLookupError(err)
		}
		impact.RecordID, impact.TenantCode, impact.Status = row.ID, row.TenantCode, row.Status
	case "roles":
		var row gormRole
		if err := db.Where("code = ?", code).Take(&row).Error; err != nil {
			return resourceLifecycleState{}, lifecycleLookupError(err)
		}
		impact.RecordID, impact.Status = row.ID, row.Status
		var group gormRoleGroup
		if err := db.Where("code = ?", row.GroupCode).Take(&group).Error; err != nil {
			return resourceLifecycleState{}, lifecycleLookupError(err)
		}
		impact.TenantCode = group.TenantCode
	case "users":
		var row gormUser
		if err := db.Where("code = ?", code).Take(&row).Error; err != nil {
			return resourceLifecycleState{}, lifecycleLookupError(err)
		}
		impact.RecordID, impact.TenantCode, impact.OrgUnitCode, impact.Status = row.ID, row.TenantCode, row.OrgUnitCode, row.Status
	}
	var lifecycle gormResourceLifecycle
	err := db.Where("resource = ? AND record_id = ?", resource, impact.RecordID).Take(&lifecycle).Error
	switch {
	case err == nil:
		impact.Deleted = strings.TrimSpace(lifecycle.DeletedAt) != ""
		if impact.Deleted {
			purgeAfter, parseErr := time.Parse(time.RFC3339, lifecycle.PurgeAfter)
			if parseErr != nil {
				return resourceLifecycleState{}, ErrInvalid
			}
			impact.RetentionElapsed = !now.Before(purgeAfter.UTC())
		}
	case errors.Is(err, gorm.ErrRecordNotFound):
		lifecycle = gormResourceLifecycle{}
	default:
		return resourceLifecycleState{}, repositoryError(err)
	}
	references, conflicts, affected, err := lifecycleReferences(db, resource, code, impact.RecordID)
	if err != nil {
		return resourceLifecycleState{}, err
	}
	impact.References, impact.Conflicts, impact.AffectedUsers = references, conflicts, affected
	impact.ReferenceCount = len(references)
	return resourceLifecycleState{impact: impact, lifecycle: lifecycle}, nil
}

func lifecycleReferences(db *gorm.DB, resource, code, recordID string) ([]ResourceLifecycleReference, []RoleAssignmentConflict, int, error) {
	references := make([]ResourceLifecycleReference, 0)
	conflicts := make([]RoleAssignmentConflict, 0)
	affected := map[string]struct{}{}
	switch resource {
	case "users":
		var rows []gormUserRole
		if err := db.Where("user_id = ?", recordID).Order("role_code").Find(&rows).Error; err != nil {
			return nil, nil, 0, repositoryError(err)
		}
		for _, row := range rows {
			references = append(references, ResourceLifecycleReference{Kind: "user-role", Code: row.RoleCode})
			conflicts = append(conflicts, RoleAssignmentConflict{UserCode: code, RoleCode: row.RoleCode})
		}
		if len(rows) > 0 {
			affected[code] = struct{}{}
		}
	case "roles":
		var assignments []gormUserRole
		if err := db.Where("role_code = ?", code).Order("user_id").Find(&assignments).Error; err != nil {
			return nil, nil, 0, repositoryError(err)
		}
		for _, assignment := range assignments {
			var user gormUser
			if err := db.Where("id = ?", assignment.UserID).Take(&user).Error; err != nil {
				return nil, nil, 0, lifecycleLookupError(err)
			}
			references = append(references, ResourceLifecycleReference{Kind: "user-role", Code: user.Code})
			conflicts = append(conflicts, RoleAssignmentConflict{UserCode: user.Code, RoleCode: code})
			affected[user.Code] = struct{}{}
		}
		var permissions []gormRolePermission
		if err := db.Where("role_code = ?", code).Order("permission").Find(&permissions).Error; err != nil {
			return nil, nil, 0, repositoryError(err)
		}
		for _, permission := range permissions {
			references = append(references, ResourceLifecycleReference{Kind: "role-permission", Code: permission.Permission})
		}
	case "role-groups":
		var roles []gormRole
		if err := db.Where("group_code = ?", code).Order("code").Find(&roles).Error; err != nil {
			return nil, nil, 0, repositoryError(err)
		}
		for _, role := range roles {
			references = append(references, ResourceLifecycleReference{Kind: "group-role", Code: role.Code})
		}
		if len(roles) > 0 {
			roleCodes := make([]string, 0, len(roles))
			for _, role := range roles {
				roleCodes = append(roleCodes, role.Code)
			}
			var assignments []gormUserRole
			if err := db.Where("role_code IN ?", roleCodes).Order("user_id, role_code").Find(&assignments).Error; err != nil {
				return nil, nil, 0, repositoryError(err)
			}
			for _, assignment := range assignments {
				var user gormUser
				if err := db.Where("id = ?", assignment.UserID).Take(&user).Error; err != nil {
					return nil, nil, 0, lifecycleLookupError(err)
				}
				references = append(references, ResourceLifecycleReference{Kind: "user-role", Code: user.Code + ":" + assignment.RoleCode})
				conflicts = append(conflicts, RoleAssignmentConflict{UserCode: user.Code, RoleCode: assignment.RoleCode})
				affected[user.Code] = struct{}{}
			}
		}
		var bindings []gormOrgUnitRoleGroup
		if err := db.Where("role_group_code = ?", code).Order("org_unit_code").Find(&bindings).Error; err != nil {
			return nil, nil, 0, repositoryError(err)
		}
		for _, binding := range bindings {
			references = append(references, ResourceLifecycleReference{Kind: "organization-role-group", Code: binding.OrgUnitCode})
		}
	case "org-units":
		var users []gormUser
		if err := db.Where("org_unit_code = ?", code).Order("code").Find(&users).Error; err != nil {
			return nil, nil, 0, repositoryError(err)
		}
		for _, user := range users {
			references = append(references, ResourceLifecycleReference{Kind: "organization-user", Code: user.Code})
			affected[user.Code] = struct{}{}
		}
		var bindings []gormOrgUnitRoleGroup
		if err := db.Where("org_unit_code = ?", code).Order("role_group_code").Find(&bindings).Error; err != nil {
			return nil, nil, 0, repositoryError(err)
		}
		for _, binding := range bindings {
			references = append(references, ResourceLifecycleReference{Kind: "organization-role-group", Code: binding.RoleGroupCode})
		}
	}
	sort.Slice(references, func(i, j int) bool {
		if references[i].Kind != references[j].Kind {
			return references[i].Kind < references[j].Kind
		}
		return references[i].Code < references[j].Code
	})
	sort.Slice(conflicts, func(i, j int) bool {
		if conflicts[i].UserCode != conflicts[j].UserCode {
			return conflicts[i].UserCode < conflicts[j].UserCode
		}
		return conflicts[i].RoleCode < conflicts[j].RoleCode
	})
	return references, conflicts, len(affected), nil
}

func applyLifecycleDeleteRemediations(db *gorm.DB, impact ResourceLifecycleImpact, remediations []RoleAssignmentRemediation) error {
	if err := validateLifecycleRemediationPlan(db, impact, remediations); err != nil {
		return err
	}
	switch impact.Resource {
	case "users":
		for _, remediation := range remediations {
			if err := db.Where("user_id = ? AND role_code = ?", impact.RecordID, remediation.RoleCode).Delete(&gormUserRole{}).Error; err != nil {
				return repositoryError(err)
			}
		}
	case "roles":
		return applyConflictRemediations(db, impact.Conflicts, remediations)
	}
	return nil
}

func validateLifecycleRemediationPlan(db *gorm.DB, impact ResourceLifecycleImpact, remediations []RoleAssignmentRemediation) error {
	switch impact.Resource {
	case "users":
		if len(remediations) != len(impact.Conflicts) {
			return ErrRolePoolViolation
		}
		targets := conflictSet(impact.Conflicts)
		seen := map[string]struct{}{}
		for _, remediation := range remediations {
			key := remediation.UserCode + "\x00" + remediation.RoleCode
			if _, ok := targets[key]; !ok || remediation.Action != "remove-role" || remediation.ReplacementRoleCode != "" {
				return ErrInvalid
			}
			if _, duplicate := seen[key]; duplicate {
				return ErrInvalid
			}
			seen[key] = struct{}{}
		}
		return nil
	case "roles":
		return validateConflictRemediationPlan(db, impact.Conflicts, remediations)
	case "role-groups":
		if impact.Operation == LifecycleOperationDisable {
			return validateConflictRemediationPlan(db, impact.Conflicts, remediations)
		}
		if len(remediations) != 0 {
			return ErrInvalid
		}
		return nil
	default:
		if len(remediations) != 0 {
			return ErrInvalid
		}
		return nil
	}
}

func validateLifecycleRestore(db *gorm.DB, impact ResourceLifecycleImpact) error {
	switch impact.Resource {
	case "org-units":
		if !validCode(impact.TenantCode) {
			return ErrInvalid
		}
		return nil
	case "role-groups":
		var row gormRoleGroup
		if err := db.Where("code = ?", impact.ResourceCode).Take(&row).Error; err != nil {
			return lifecycleLookupError(err)
		}
		return ValidateRoleGroup(roleGroupFromGORM(row, false))
	case "roles":
		var role gormRole
		if err := db.Where("code = ?", impact.ResourceCode).Take(&role).Error; err != nil {
			return lifecycleLookupError(err)
		}
		var group gormRoleGroup
		if err := db.Where("code = ?", role.GroupCode).Take(&group).Error; err != nil {
			return lifecycleLookupError(err)
		}
		deleted, err := isLifecycleDeleted(db, "role-groups", group.ID)
		if err != nil || deleted {
			return ErrRolePoolViolation
		}
		return ValidateRole(Role{Code: role.Code, GroupCode: role.GroupCode, Status: "disabled"}, map[string]RoleGroup{group.Code: roleGroupFromGORM(group, false)})
	case "users":
		var user gormUser
		if err := db.Where("code = ?", impact.ResourceCode).Take(&user).Error; err != nil {
			return lifecycleLookupError(err)
		}
		_, err := (&GORMRepository{db: db}).DeriveAndValidateUser(context.Background(), User{
			ID: user.ID, Code: user.Code, ScopeType: ScopeType(user.ScopeType), TenantCode: user.TenantCode,
			OrgUnitCode: user.OrgUnitCode, Status: "disabled",
		}, nil)
		return err
	}
	return ErrInvalid
}

func setLifecycleResourceStatus(db *gorm.DB, resource, code, status string, changedAt time.Time) error {
	updates := map[string]any{"status": status}
	if resource == "roles" || resource == "users" {
		updates["updated_at"] = changedAt.UTC().Format(time.RFC3339)
	}
	var model any
	switch resource {
	case "org-units":
		model = &gormOrganization{}
	case "role-groups":
		model = &gormRoleGroup{}
	case "roles":
		model = &gormRole{}
	case "users":
		model = &gormUser{}
	default:
		return ErrInvalid
	}
	result := db.Model(model).Where("code = ?", code).Updates(updates)
	if result.Error != nil {
		return repositoryError(result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrNotFound
	}
	return nil
}

func purgeLifecycleResource(db *gorm.DB, resource, code string) error {
	var model any
	switch resource {
	case "org-units":
		if err := db.Where("org_unit_code = ?", code).Delete(&gormOrgUnitRoleGroupRevision{}).Error; err != nil {
			return repositoryError(err)
		}
		model = &gormOrganization{}
	case "role-groups":
		model = &gormRoleGroup{}
	case "roles":
		model = &gormRole{}
	case "users":
		model = &gormUser{}
	default:
		return ErrInvalid
	}
	result := db.Where("code = ?", code).Delete(model)
	if result.Error != nil {
		return repositoryError(result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrNotFound
	}
	return nil
}

func validLifecycleResource(resource string) bool {
	switch resource {
	case "org-units", "role-groups", "roles", "users":
		return true
	default:
		return false
	}
}

func validLifecycleOperation(operation string) bool {
	switch operation {
	case LifecycleOperationDelete, LifecycleOperationRestore, LifecycleOperationPurge, LifecycleOperationDisable:
		return true
	default:
		return false
	}
}

func lifecycleLookupError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("%w: lifecycle resource", ErrNotFound)
	}
	return repositoryError(err)
}
