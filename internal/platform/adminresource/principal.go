package adminresource

import (
	"errors"
	"slices"
	"strings"

	"platform-go/internal/platform/rbac"
)

var ErrAdminPrincipalInvalid = errors.New("invalid admin principal")

func ValidateAdminPrincipal(store *Store, username string) (rbac.Principal, error) {
	if store == nil {
		return rbac.Principal{}, ErrAdminPrincipalInvalid
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.validateAdminPrincipalLocked(username)
}

func (s *Store) validateAdminPrincipalLocked(username string) (rbac.Principal, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return rbac.Principal{}, ErrAdminPrincipalInvalid
	}
	if !s.hasSingleEnabledUserLocked(username) {
		return rbac.Principal{}, ErrAdminPrincipalInvalid
	}
	principal := s.currentPrincipalLocked(username)
	if strings.TrimSpace(principal.User.ID) == "" || principal.User.Username != username || !hasEffectivePermission(principal) {
		return rbac.Principal{}, ErrAdminPrincipalInvalid
	}
	return principal, nil
}

func (s *Store) hasSingleEnabledUserLocked(username string) bool {
	matches := 0
	for _, user := range visibleRecords("users", s.resources["users"]) {
		if user.Code != username {
			continue
		}
		matches++
		if strings.TrimSpace(user.Status) != "enabled" {
			return false
		}
	}
	return matches == 1
}

func hasEffectivePermission(principal rbac.Principal) bool {
	policy := rbac.NewPolicySetWithDeny(principal.Permissions, principal.DeniedPermissions)
	for _, permission := range principal.Permissions {
		if policy.Allows(permission) {
			return true
		}
	}
	return false
}

func (s *Store) CurrentPrincipal(username string) rbac.Principal {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.currentPrincipalLocked(username)
}

func (s *Store) currentPrincipalLocked(username string) rbac.Principal {
	username = strings.TrimSpace(username)
	if username == "" {
		username = "admin"
	}
	user := findRecordByCode(visibleRecords("users", s.resources["users"]), username)
	if user == nil {
		return rbac.Principal{User: rbac.User{Username: username}}
	}
	roles := s.effectiveRoleCodesLocked(*user)
	permissionPolicies := make([]string, 0)
	deniedPermissionPolicies := make([]string, 0)
	for _, role := range roles {
		roleRecord := findRecordByCode(visibleRecords("roles", s.resources["roles"]), role)
		if roleRecord == nil || roleRecord.Status == "disabled" {
			continue
		}
		for _, permission := range rbac.ParsePermissionList(roleRecord.Values["permissions"]) {
			if !slices.Contains(permissionPolicies, permission) {
				permissionPolicies = append(permissionPolicies, permission)
			}
		}
		for _, permission := range rbac.ParsePermissionList(roleRecord.Values["denyPermissions"]) {
			if !slices.Contains(deniedPermissionPolicies, permission) {
				deniedPermissionPolicies = append(deniedPermissionPolicies, permission)
			}
		}
	}
	permissions := s.expandPermissionPoliciesLocked(permissionPolicies)
	deniedPermissions := s.expandPermissionPoliciesLocked(deniedPermissionPolicies)
	return rbac.Principal{
		User: rbac.User{
			ID:          user.ID,
			Username:    user.Code,
			Name:        user.Name,
			ScopeType:   principalScopeType(*user),
			TenantCode:  strings.TrimSpace(user.Values["tenantCode"]),
			OrgUnitCode: strings.TrimSpace(user.Values["orgUnitCode"]),
			AreaCode:    strings.TrimSpace(user.Values["areaCode"]),
		},
		Roles:             roles,
		Permissions:       permissions,
		DeniedPermissions: deniedPermissions,
	}
}

func principalScopeType(user Record) string {
	scopeType := strings.TrimSpace(user.Values["scopeType"])
	if scopeType != "" {
		return scopeType
	}
	if strings.TrimSpace(user.Values["tenantCode"]) == platformTenant {
		return "platform"
	}
	if strings.TrimSpace(user.Values["orgUnitCode"]) != "" {
		return "tenant"
	}
	return ""
}

func (s *Store) effectiveRoleCodesLocked(user Record) []string {
	requested := roleValuesFromUser(user)
	scopeType := principalScopeType(user)
	tenantCode := strings.TrimSpace(user.Values["tenantCode"])
	orgUnitCode := strings.TrimSpace(user.Values["orgUnitCode"])
	eligibleGroups := map[string]struct{}{}
	switch scopeType {
	case "platform":
		for _, group := range visibleRecords("role-groups", s.resources["role-groups"]) {
			groupScope := strings.TrimSpace(group.Values["scopeType"])
			if groupScope == "" {
				groupScope = "platform"
			}
			if group.Status == "enabled" && groupScope == "platform" && strings.TrimSpace(group.Values["tenantCode"]) == "" {
				eligibleGroups[group.Code] = struct{}{}
			}
		}
	case "tenant":
		organization := findRecordByCode(visibleRecords("org-units", s.resources["org-units"]), orgUnitCode)
		if organization == nil || organization.Status != "enabled" || strings.TrimSpace(organization.Values["tenantCode"]) != tenantCode {
			return nil
		}
		for _, groupCode := range rbac.ParsePermissionList(organization.Values["roleGroupCodes"]) {
			group := findRecordByCode(visibleRecords("role-groups", s.resources["role-groups"]), groupCode)
			if group != nil && group.Status == "enabled" && strings.TrimSpace(group.Values["scopeType"]) == "tenant" && strings.TrimSpace(group.Values["tenantCode"]) == tenantCode {
				eligibleGroups[groupCode] = struct{}{}
			}
		}
	default:
		return nil
	}
	roles := make([]string, 0, len(requested))
	for _, roleCode := range requested {
		role := findRecordByCode(visibleRecords("roles", s.resources["roles"]), roleCode)
		if role == nil || role.Status != "enabled" {
			continue
		}
		groupCode := strings.TrimSpace(role.Values["groupCode"])
		if scopeType == "platform" && (groupCode == "" || findRecordByCode(visibleRecords("role-groups", s.resources["role-groups"]), groupCode) == nil) {
			roles = append(roles, roleCode)
			continue
		}
		if _, eligible := eligibleGroups[groupCode]; eligible {
			roles = append(roles, roleCode)
		}
	}
	return roles
}

func roleValuesFromUser(user Record) []string {
	values := user.Values
	if values == nil {
		return nil
	}
	if roles := rbac.ParsePermissionList(values["roles"]); len(roles) > 0 {
		return roles
	}
	return rbac.ParsePermissionList(values["role"])
}

func findRecordByCode(records []Record, code string) *Record {
	for index := range records {
		if records[index].Code == code {
			return &records[index]
		}
	}
	return nil
}
