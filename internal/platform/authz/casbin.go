package authz

import (
	"strings"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
)

const (
	PolicyEffectAllow = "allow"
	PolicyEffectDeny  = "deny"
)

type RolePolicy struct {
	RoleCode   string
	Tenant     string
	Permission string
	Action     string
	Effect     string
}

type UserRole struct {
	User     string
	RoleCode string
	Tenant   string
}

type CasbinAuthorizer struct {
	enforcer *casbin.Enforcer
}

func NewCasbinAuthorizer(policies []RolePolicy, roles []UserRole) (*CasbinAuthorizer, error) {
	m, err := model.NewModelFromString(`
[request_definition]
r = sub, tenant, obj, act

[policy_definition]
p = role, tenant, obj, act, eft

[role_definition]
g = _, _, _

[policy_effect]
e = some(where (p.eft == allow)) && !some(where (p.eft == deny))

[matchers]
m = g(r.sub, p.role, r.tenant) && r.tenant == p.tenant && permissionMatch(r.obj, p.obj) && (p.act == "*" || r.act == p.act)
`)
	if err != nil {
		return nil, err
	}
	enforcer, err := casbin.NewEnforcer(m)
	if err != nil {
		return nil, err
	}
	enforcer.AddFunction("permissionMatch", permissionMatch)
	for _, policy := range policies {
		if _, err := enforcer.AddPolicy(policy.RoleCode, policy.Tenant, policy.Permission, policy.Action, policyEffect(policy.Effect)); err != nil {
			return nil, err
		}
	}
	for _, role := range roles {
		if _, err := enforcer.AddGroupingPolicy(role.User, role.RoleCode, role.Tenant); err != nil {
			return nil, err
		}
	}
	return &CasbinAuthorizer{enforcer: enforcer}, nil
}

func policyEffect(effect string) string {
	switch strings.TrimSpace(effect) {
	case PolicyEffectDeny:
		return PolicyEffectDeny
	default:
		return PolicyEffectAllow
	}
}

func (a *CasbinAuthorizer) Can(user string, tenant string, permission string, action string) bool {
	if a == nil || a.enforcer == nil {
		return false
	}
	ok, err := a.enforcer.Enforce(user, tenant, permission, action)
	return err == nil && ok
}

func permissionMatch(args ...interface{}) (interface{}, error) {
	if len(args) != 2 {
		return false, nil
	}
	requested := expressionString(args[0])
	granted := expressionString(args[1])
	if requested == "" || granted == "" {
		return false, nil
	}
	if granted == "*" || granted == requested {
		return true, nil
	}
	if strings.HasSuffix(granted, "*") {
		return strings.HasPrefix(requested, strings.TrimSuffix(granted, "*")), nil
	}
	return false, nil
}

func expressionString(value interface{}) string {
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}
