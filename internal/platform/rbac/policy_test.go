package rbac

import "testing"

func TestPolicyAllowsExactAndWildcardPermissions(t *testing.T) {
	policy := NewPolicySet([]string{"admin:tenant:read", "admin:role:*", "admin:menu:*"})

	tests := []struct {
		name       string
		permission string
		want       bool
	}{
		{name: "exact permission", permission: "admin:tenant:read", want: true},
		{name: "same resource different action denied", permission: "admin:tenant:create", want: false},
		{name: "resource wildcard", permission: "admin:role:update", want: true},
		{name: "different resource denied", permission: "admin:user:read", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := policy.Allows(test.permission); got != test.want {
				t.Fatalf("Allows(%q) = %v, want %v", test.permission, got, test.want)
			}
		})
	}
}

func TestPolicyAllowsGlobalWildcard(t *testing.T) {
	policy := NewPolicySet([]string{"*"})

	if !policy.Allows("admin:user:delete") {
		t.Fatalf("Allows(admin:user:delete) = false, want true")
	}
}
