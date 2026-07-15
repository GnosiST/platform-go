package organizationrbac

import (
	"fmt"
	"sort"
)

func ValidateRoleGroup(group RoleGroup) error {
	if !validCode(group.Code) {
		return &ValidationError{Field: "roleGroup.code", Reason: "is required and must be canonical"}
	}
	switch group.ScopeType {
	case ScopePlatform:
		if group.TenantCode != "" {
			return &ValidationError{Field: "roleGroup.tenantCode", Reason: "must be empty for platform scope"}
		}
	case ScopeTenant:
		if !validCode(group.TenantCode) {
			return &ValidationError{Field: "roleGroup.tenantCode", Reason: "is required for tenant scope"}
		}
	default:
		return &ValidationError{Field: "roleGroup.scopeType", Reason: "must be platform or tenant"}
	}
	return nil
}

func ValidateRole(role Role, groups map[string]RoleGroup) error {
	if !validCode(role.Code) {
		return &ValidationError{Field: "role.code", Reason: "is required and must be canonical"}
	}
	if !validCode(role.GroupCode) {
		return &ValidationError{Field: "role.groupCode", Reason: "exactly one role group is required"}
	}
	group, exists := groups[role.GroupCode]
	if !exists {
		return fmt.Errorf("%w: role group %q", ErrNotFound, role.GroupCode)
	}
	return ValidateRoleGroup(group)
}

func EffectiveRolePool(
	organization Organization,
	groups map[string]RoleGroup,
	roles map[string]Role,
	bindings []OrgUnitRoleGroupBinding,
) ([]RolePoolEntry, error) {
	if !validCode(organization.Code) || !validCode(organization.TenantCode) {
		return nil, &ValidationError{Field: "organization", Reason: "code and tenantCode are required"}
	}
	if !organization.Enabled() {
		return nil, &ValidationError{Field: "organization.status", Reason: "organization must be enabled"}
	}

	eligibleGroups := make(map[string]RoleGroup)
	for _, binding := range bindings {
		if binding.OrgUnitCode != organization.Code {
			continue
		}
		group, exists := groups[binding.RoleGroupCode]
		if !exists {
			return nil, fmt.Errorf("%w: role group %q", ErrNotFound, binding.RoleGroupCode)
		}
		if err := validateBoundRoleGroup(organization, group); err != nil {
			return nil, err
		}
		eligibleGroups[group.Code] = group
	}

	pool := make([]RolePoolEntry, 0)
	for _, roleCode := range sortedRoleCodes(roles) {
		role := roles[roleCode]
		if !role.Enabled() {
			continue
		}
		group, eligible := eligibleGroups[role.GroupCode]
		if !eligible {
			continue
		}
		if err := ValidateRole(role, groups); err != nil {
			return nil, err
		}
		pool = append(pool, RolePoolEntry{
			RoleCode: role.Code, RoleGroupCode: group.Code, RoleGroupName: group.Name, TenantCode: organization.TenantCode, Status: role.Status,
		})
	}
	return pool, nil
}

func DeriveAndValidateUser(
	user User,
	roleCodes []string,
	organizations map[string]Organization,
	groups map[string]RoleGroup,
	roles map[string]Role,
	bindings []OrgUnitRoleGroupBinding,
) (User, error) {
	rolesRequested, err := canonicalCodes(roleCodes)
	if err != nil {
		return User{}, err
	}
	switch user.ScopeType {
	case ScopePlatform:
		if user.OrgUnitCode != "" || user.TenantCode != "" {
			return User{}, &ValidationError{Field: "user.scopeType", Reason: "platform users cannot have organization or tenant ownership"}
		}
		for _, roleCode := range rolesRequested {
			role, exists := roles[roleCode]
			if !exists || !role.Enabled() {
				return User{}, fmt.Errorf("%w: role %q", ErrRolePoolViolation, roleCode)
			}
			group, exists := groups[role.GroupCode]
			if !exists || ValidateRoleGroup(group) != nil || !group.Enabled() || group.ScopeType != ScopePlatform {
				return User{}, fmt.Errorf("%w: role %q is not an enabled platform role", ErrRolePoolViolation, roleCode)
			}
		}
		return user, nil
	case ScopeTenant:
		if !validCode(user.OrgUnitCode) {
			return User{}, &ValidationError{Field: "user.orgUnitCode", Reason: "one primary organization is required for tenant users"}
		}
		organization, exists := organizations[user.OrgUnitCode]
		if !exists {
			return User{}, fmt.Errorf("%w: organization %q", ErrNotFound, user.OrgUnitCode)
		}
		pool, err := EffectiveRolePool(organization, groups, roles, bindings)
		if err != nil {
			return User{}, err
		}
		allowed := make(map[string]struct{}, len(pool))
		for _, entry := range pool {
			allowed[entry.RoleCode] = struct{}{}
		}
		for _, roleCode := range rolesRequested {
			if _, exists := allowed[roleCode]; !exists {
				return User{}, fmt.Errorf("%w: role %q is outside organization %q", ErrRolePoolViolation, roleCode, user.OrgUnitCode)
			}
		}
		user.TenantCode = organization.TenantCode
		return user, nil
	default:
		return User{}, &ValidationError{Field: "user.scopeType", Reason: "must be platform or tenant"}
	}
}

func validateBoundRoleGroup(organization Organization, group RoleGroup) error {
	if err := ValidateRoleGroup(group); err != nil {
		return err
	}
	if !group.Enabled() {
		return &ValidationError{Field: "roleGroup.status", Reason: "bound role group must be enabled"}
	}
	if group.ScopeType != ScopeTenant {
		return &ValidationError{Field: "roleGroup.scopeType", Reason: "platform role groups cannot be bound to organizations"}
	}
	if group.TenantCode != organization.TenantCode {
		return &ValidationError{Field: "roleGroup.tenantCode", Reason: "must match the organization tenant"}
	}
	return nil
}

func sortedRoleCodes(roles map[string]Role) []string {
	codes := make([]string, 0, len(roles))
	for code := range roles {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes
}
