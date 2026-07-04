package kernel

import (
	"context"
	"testing"
)

func TestExecutionContextRequiresActorTenantAndPermissionForPermissionedCall(t *testing.T) {
	exec := ExecutionContext{
		Context: context.Background(),
		Actor: Actor{
			ID:       1001,
			Username: "admin",
			Kind:     ActorKindUser,
		},
		TenantScope: TenantScope{TenantID: 1, PlatformWide: true},
		PermissionIntent: PermissionIntent{
			Code:   "admin:tenant:read",
			Action: "read",
		},
	}

	if err := exec.ValidatePermissioned(); err != nil {
		t.Fatalf("ValidatePermissioned() error = %v", err)
	}
}

func TestExecutionContextRejectsMissingActor(t *testing.T) {
	exec := ExecutionContext{
		Context:          context.Background(),
		TenantScope:      TenantScope{TenantID: 1},
		PermissionIntent: PermissionIntent{Code: "admin:user:read", Action: "read"},
	}

	err := exec.ValidatePermissioned()
	if err == nil {
		t.Fatalf("ValidatePermissioned() error = nil, want missing actor")
	}
	if !IsCode(err, ErrCodeInvalidExecutionContext) {
		t.Fatalf("ValidatePermissioned() error = %v, want %s", err, ErrCodeInvalidExecutionContext)
	}
}

func TestExecutionContextRejectsMissingTenantScope(t *testing.T) {
	exec := ExecutionContext{
		Context: context.Background(),
		Actor:   Actor{ID: 1001, Username: "admin", Kind: ActorKindUser},
		PermissionIntent: PermissionIntent{
			Code:   "admin:user:read",
			Action: "read",
		},
	}

	err := exec.ValidatePermissioned()
	if err == nil {
		t.Fatalf("ValidatePermissioned() error = nil, want missing tenant")
	}
	if !IsCode(err, ErrCodeInvalidExecutionContext) {
		t.Fatalf("ValidatePermissioned() error = %v, want %s", err, ErrCodeInvalidExecutionContext)
	}
}
