package authz

import "testing"

func TestCasbinAuthorizerAllowsRolePolicy(t *testing.T) {
	authorizer, err := NewCasbinAuthorizer([]RolePolicy{
		{RoleCode: "admin", Tenant: "platform", Permission: "admin:user:read", Action: "read"},
	}, []UserRole{
		{User: "user:1", RoleCode: "admin", Tenant: "platform"},
	})
	if err != nil {
		t.Fatalf("NewCasbinAuthorizer() error = %v", err)
	}
	if !authorizer.Can("user:1", "platform", "admin:user:read", "read") {
		t.Fatalf("Can() = false, want true")
	}
	if authorizer.Can("user:1", "platform", "admin:user:write", "write") {
		t.Fatalf("Can(write) = true, want false")
	}
}

func TestCasbinAuthorizerAllowsWildcardRolePolicy(t *testing.T) {
	authorizer, err := NewCasbinAuthorizer([]RolePolicy{
		{RoleCode: "platform-admin", Tenant: "platform", Permission: "admin:*", Action: "*"},
	}, []UserRole{
		{User: "user:1", RoleCode: "platform-admin", Tenant: "platform"},
	})
	if err != nil {
		t.Fatalf("NewCasbinAuthorizer() error = %v", err)
	}
	if !authorizer.Can("user:1", "platform", "admin:tenant:read", "read") {
		t.Fatalf("Can(read through wildcard) = false, want true")
	}
	if !authorizer.Can("user:1", "platform", "admin:user:update", "update") {
		t.Fatalf("Can(update through wildcard) = false, want true")
	}
	if authorizer.Can("user:1", "other", "admin:tenant:read", "read") {
		t.Fatalf("Can(other tenant) = true, want false")
	}
}

func TestCasbinAuthorizerDenyOverridesWildcardAllow(t *testing.T) {
	authorizer, err := NewCasbinAuthorizer([]RolePolicy{
		{RoleCode: "platform-admin", Tenant: "platform", Permission: "admin:*", Action: "*"},
		{RoleCode: "platform-admin", Tenant: "platform", Permission: "admin:tenant:read", Action: "read", Effect: PolicyEffectDeny},
	}, []UserRole{
		{User: "user:1", RoleCode: "platform-admin", Tenant: "platform"},
	})
	if err != nil {
		t.Fatalf("NewCasbinAuthorizer() error = %v", err)
	}
	if authorizer.Can("user:1", "platform", "admin:tenant:read", "read") {
		t.Fatalf("Can(explicit deny) = true, want false")
	}
	if !authorizer.Can("user:1", "platform", "admin:user:read", "read") {
		t.Fatalf("Can(other wildcard permission) = false, want true")
	}
	if !authorizer.Can("user:1", "platform", "admin:tenant:update", "update") {
		t.Fatalf("Can(other tenant action) = false, want true")
	}
}

func TestCasbinAuthorizerInactivePermissionOverridesWildcardAllow(t *testing.T) {
	authorizer, err := NewCasbinAuthorizerWithInactivePermissions([]RolePolicy{
		{RoleCode: "platform-admin", Tenant: "platform", Permission: "*", Action: "*"},
	}, []UserRole{
		{User: "user:1", RoleCode: "platform-admin", Tenant: "platform"},
	}, []string{"admin:user:read"})
	if err != nil {
		t.Fatalf("NewCasbinAuthorizerWithInactivePermissions() error = %v", err)
	}
	if authorizer.Can("user:1", "platform", "admin:user:read", "read") {
		t.Fatal("inactive admin:user:read allowed through global wildcard")
	}
	if !authorizer.Can("user:1", "platform", "admin:tenant:read", "read") {
		t.Fatal("unrelated permission rejected through global wildcard")
	}
}
