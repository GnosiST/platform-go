package organizationrbac

import (
	"errors"
	"reflect"
	"testing"
)

func TestValidateRoleGroupRequiresExplicitScopeOwnership(t *testing.T) {
	tests := []struct {
		name  string
		group RoleGroup
		valid bool
	}{
		{name: "platform", group: RoleGroup{Code: "platform-admin", ScopeType: ScopePlatform}, valid: true},
		{name: "tenant", group: RoleGroup{Code: "tenant-admin", ScopeType: ScopeTenant, TenantCode: "acme"}, valid: true},
		{name: "platform tenant forbidden", group: RoleGroup{Code: "platform-admin", ScopeType: ScopePlatform, TenantCode: "acme"}},
		{name: "tenant owner required", group: RoleGroup{Code: "tenant-admin", ScopeType: ScopeTenant}},
		{name: "implicit scope forbidden", group: RoleGroup{Code: "tenant-admin", TenantCode: "acme"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateRoleGroup(test.group)
			if test.valid && err != nil {
				t.Fatalf("ValidateRoleGroup() error = %v", err)
			}
			if !test.valid && !errors.Is(err, ErrInvalid) {
				t.Fatalf("ValidateRoleGroup() error = %v, want ErrInvalid", err)
			}
		})
	}
}

func TestEffectiveRolePoolUsesDirectEnabledSameTenantGroups(t *testing.T) {
	organization := Organization{Code: "acme-hq", TenantCode: "acme", Status: StatusEnabled}
	groups := map[string]RoleGroup{
		"acme-ops": {Code: "acme-ops", Name: "Operations", ScopeType: ScopeTenant, TenantCode: "acme", Status: StatusEnabled},
		"acme-off": {Code: "acme-off", ScopeType: ScopeTenant, TenantCode: "acme", Status: "disabled"},
		"platform": {Code: "platform", ScopeType: ScopePlatform, Status: StatusEnabled},
	}
	roles := map[string]Role{
		"operator":      {Code: "operator", GroupCode: "acme-ops", Status: StatusEnabled},
		"disabled-role": {Code: "disabled-role", GroupCode: "acme-ops", Status: "disabled"},
		"unbound-role":  {Code: "unbound-role", GroupCode: "acme-off", Status: StatusEnabled},
	}
	pool, err := EffectiveRolePool(organization, groups, roles, []OrgUnitRoleGroupBinding{{OrgUnitCode: "acme-hq", RoleGroupCode: "acme-ops"}})
	if err != nil {
		t.Fatalf("EffectiveRolePool() error = %v", err)
	}
	want := []RolePoolEntry{{RoleCode: "operator", RoleGroupCode: "acme-ops", RoleGroupName: "Operations", TenantCode: "acme", Status: StatusEnabled}}
	if !reflect.DeepEqual(pool, want) {
		t.Fatalf("EffectiveRolePool() = %+v, want %+v", pool, want)
	}

	_, err = EffectiveRolePool(organization, groups, roles, []OrgUnitRoleGroupBinding{{OrgUnitCode: "acme-hq", RoleGroupCode: "platform"}})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("EffectiveRolePool(platform group) error = %v, want ErrInvalid", err)
	}
}

func TestDeriveAndValidateUserEnforcesTenantRolePoolAndPlatformException(t *testing.T) {
	organizations := map[string]Organization{
		"acme-hq": {Code: "acme-hq", TenantCode: "acme", Status: StatusEnabled},
	}
	groups := map[string]RoleGroup{
		"acme-ops":       {Code: "acme-ops", ScopeType: ScopeTenant, TenantCode: "acme", Status: StatusEnabled},
		"platform-admin": {Code: "platform-admin", ScopeType: ScopePlatform, Status: StatusEnabled},
	}
	roles := map[string]Role{
		"operator":    {Code: "operator", GroupCode: "acme-ops", Status: StatusEnabled},
		"super-admin": {Code: "super-admin", GroupCode: "platform-admin", Status: StatusEnabled},
	}
	bindings := []OrgUnitRoleGroupBinding{{OrgUnitCode: "acme-hq", RoleGroupCode: "acme-ops"}}

	tenantUser, err := DeriveAndValidateUser(User{Code: "alice", ScopeType: ScopeTenant, TenantCode: "untrusted", OrgUnitCode: "acme-hq"}, []string{"operator"}, organizations, groups, roles, bindings)
	if err != nil {
		t.Fatalf("DeriveAndValidateUser(tenant) error = %v", err)
	}
	if tenantUser.TenantCode != "acme" {
		t.Fatalf("TenantCode = %q, want server-derived acme", tenantUser.TenantCode)
	}
	if _, err := DeriveAndValidateUser(tenantUser, []string{"super-admin"}, organizations, groups, roles, bindings); !errors.Is(err, ErrRolePoolViolation) {
		t.Fatalf("DeriveAndValidateUser(outside pool) error = %v, want ErrRolePoolViolation", err)
	}
	platformUser, err := DeriveAndValidateUser(User{Code: "root", ScopeType: ScopePlatform}, []string{"super-admin"}, organizations, groups, roles, bindings)
	if err != nil || platformUser.TenantCode != "" || platformUser.OrgUnitCode != "" {
		t.Fatalf("DeriveAndValidateUser(platform) = %+v, %v", platformUser, err)
	}
	if _, err := DeriveAndValidateUser(User{Code: "implicit", ScopeType: ScopePlatform, OrgUnitCode: "acme-hq"}, nil, organizations, groups, roles, bindings); !errors.Is(err, ErrInvalid) {
		t.Fatalf("DeriveAndValidateUser(platform organization) error = %v, want ErrInvalid", err)
	}
}
