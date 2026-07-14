package adminresource

import (
	"strings"

	"platform-go/internal/platform/authz"
	"platform-go/internal/platform/rbac"
)

const platformTenant = "platform"

func (s *Store) CasbinAuthorizer() (*authz.CasbinAuthorizer, error) {
	policies, roles := s.casbinPolicySnapshot()
	return authz.NewCasbinAuthorizer(policies, roles)
}

func (s *Store) casbinPolicySnapshot() ([]authz.RolePolicy, []authz.UserRole) {
	s.mu.Lock()
	defer s.mu.Unlock()

	policies := make([]authz.RolePolicy, 0)
	for _, role := range visibleRecords("roles", s.resources["roles"]) {
		if role.Status == "disabled" {
			continue
		}
		for _, permission := range rbac.ParsePermissionList(role.Values["permissions"]) {
			policies = append(policies, authz.RolePolicy{
				RoleCode:   role.Code,
				Tenant:     platformTenant,
				Permission: permission,
				Action:     actionFromPermission(permission),
				Effect:     authz.PolicyEffectAllow,
			})
		}
		for _, permission := range rbac.ParsePermissionList(role.Values["denyPermissions"]) {
			policies = append(policies, authz.RolePolicy{
				RoleCode:   role.Code,
				Tenant:     platformTenant,
				Permission: permission,
				Action:     actionFromPermission(permission),
				Effect:     authz.PolicyEffectDeny,
			})
		}
	}

	roles := make([]authz.UserRole, 0)
	for _, user := range visibleRecords("users", s.resources["users"]) {
		if user.Status == "disabled" {
			continue
		}
		for _, role := range roleValuesFromUser(user) {
			roles = append(roles, authz.UserRole{
				User:     user.Code,
				RoleCode: role,
				Tenant:   platformTenant,
			})
		}
	}
	return policies, roles
}

func actionFromPermission(permission string) string {
	permission = strings.TrimSpace(permission)
	if permission == "" || permission == "*" || strings.HasSuffix(permission, "*") {
		return "*"
	}
	index := strings.LastIndex(permission, ":")
	if index < 0 || index == len(permission)-1 {
		return "*"
	}
	return permission[index+1:]
}
