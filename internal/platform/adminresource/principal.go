package adminresource

import (
	"errors"
	"slices"
	"sort"
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
	for _, user := range s.resources["users"] {
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
	user := findRecordByCode(s.resources["users"], username)
	if user == nil {
		return rbac.Principal{User: rbac.User{Username: username}}
	}
	roles := roleValuesFromUser(*user)
	permissions := make([]string, 0)
	deniedPermissions := make([]string, 0)
	for _, role := range roles {
		roleRecord := findRecordByCode(s.resources["roles"], role)
		if roleRecord == nil || roleRecord.Status == "disabled" {
			continue
		}
		for _, permission := range rbac.ParsePermissionList(roleRecord.Values["permissions"]) {
			if !slices.Contains(permissions, permission) {
				permissions = append(permissions, permission)
			}
		}
		for _, permission := range rbac.ParsePermissionList(roleRecord.Values["denyPermissions"]) {
			if !slices.Contains(deniedPermissions, permission) {
				deniedPermissions = append(deniedPermissions, permission)
			}
		}
	}
	sort.Strings(permissions)
	sort.Strings(deniedPermissions)
	return rbac.Principal{
		User: rbac.User{
			ID:          user.ID,
			Username:    user.Code,
			Name:        user.Name,
			TenantCode:  strings.TrimSpace(user.Values["tenantCode"]),
			OrgUnitCode: strings.TrimSpace(user.Values["orgUnitCode"]),
			AreaCode:    strings.TrimSpace(user.Values["areaCode"]),
		},
		Roles:             roles,
		Permissions:       permissions,
		DeniedPermissions: deniedPermissions,
	}
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
