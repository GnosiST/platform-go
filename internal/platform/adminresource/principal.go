package adminresource

import (
	"slices"
	"sort"
	"strings"

	"platform-go/internal/platform/rbac"
)

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
