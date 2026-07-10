package rbac

import (
	"slices"
	"strings"
)

type User struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	Name        string `json:"name"`
	TenantCode  string `json:"tenantCode,omitempty"`
	OrgUnitCode string `json:"orgUnitCode,omitempty"`
	AreaCode    string `json:"areaCode,omitempty"`
}

type Principal struct {
	User              User     `json:"user"`
	Roles             []string `json:"roles"`
	Permissions       []string `json:"permissions"`
	DeniedPermissions []string `json:"deniedPermissions,omitempty"`
}

type PolicySet struct {
	permissions       []string
	deniedPermissions []string
}

func NewPolicySet(permissions []string) PolicySet {
	return NewPolicySetWithDeny(permissions, nil)
}

func NewPolicySetWithDeny(permissions []string, deniedPermissions []string) PolicySet {
	normalized := make([]string, 0, len(permissions))
	for _, permission := range permissions {
		permission = strings.TrimSpace(permission)
		if permission == "" || slices.Contains(normalized, permission) {
			continue
		}
		normalized = append(normalized, permission)
	}
	denied := make([]string, 0, len(deniedPermissions))
	for _, permission := range deniedPermissions {
		permission = strings.TrimSpace(permission)
		if permission == "" || slices.Contains(denied, permission) {
			continue
		}
		denied = append(denied, permission)
	}
	return PolicySet{permissions: normalized, deniedPermissions: denied}
}

func (p PolicySet) Allows(permission string) bool {
	permission = strings.TrimSpace(permission)
	if permission == "" {
		return false
	}
	for _, denied := range p.deniedPermissions {
		if matches(denied, permission) {
			return false
		}
	}
	for _, granted := range p.permissions {
		if matches(granted, permission) {
			return true
		}
	}
	return false
}

func ParsePermissionList(value string) []string {
	return parseList(value)
}

func matches(granted string, permission string) bool {
	if granted == "*" || granted == permission {
		return true
	}
	if strings.HasSuffix(granted, ":*") {
		return strings.HasPrefix(permission, strings.TrimSuffix(granted, "*"))
	}
	return false
}

func parseList(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\t' || r == ' '
	})
	normalized := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || slices.Contains(normalized, part) {
			continue
		}
		normalized = append(normalized, part)
	}
	return normalized
}
