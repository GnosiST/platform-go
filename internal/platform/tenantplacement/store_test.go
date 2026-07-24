package tenantplacement

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/GnosiST/platform-go/internal/platform/kernel"
	"github.com/GnosiST/platform-go/internal/platform/storage"
)

func TestStoreResolvesTrustedTenantContextAndAuditsPlacementTransition(t *testing.T) {
	store, err := NewStore([]Placement{{
		TenantCode: "tenant-a", CapabilityID: "orders", DatasourceGroupID: "orders-primary", Version: 1, Status: StatusActive,
	}}, AuthorizerFunc(func(_ kernel.ExecutionContext, permission string) bool {
		return permission == PermissionManage
	}), func() time.Time { return time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	execution := tenantExecution("tenant-a")
	resolved, err := store.Resolve(execution, "orders", Override{})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if resolved.DatasourceGroupID != "orders-primary" || resolved.Version != 1 {
		t.Fatalf("Resolve() = %+v, want initial placement", resolved)
	}

	updated, err := store.Transition(execution, Transition{
		TenantCode: "tenant-a", CapabilityID: "orders", DatasourceGroupID: "orders-secondary", Version: 2,
		Status: StatusMigrating, Reason: "capacity-rebalance",
	})
	if err != nil {
		t.Fatalf("Transition() error = %v", err)
	}
	if updated.Version != 2 || updated.Status != StatusMigrating {
		t.Fatalf("Transition() = %+v", updated)
	}
	audit := store.Audit()
	if len(audit) != 1 || audit[0].Actor != "ops" || audit[0].PreviousDatasourceGroupID != "orders-primary" || audit[0].DatasourceGroupID != "orders-secondary" {
		t.Fatalf("Audit() = %+v, want redacted transition audit", audit)
	}
}

func TestGORMStorePersistsPlacementTransitionsAndAudit(t *testing.T) {
	db, err := storage.OpenGORM(storage.Config{Driver: "sqlite", DSN: t.TempDir() + "/placements.db"})
	if err != nil {
		t.Fatal(err)
	}
	if err := PrepareGORMStore(context.Background(), db); err != nil {
		t.Fatalf("PrepareGORMStore() error = %v", err)
	}
	store, err := OpenGORMStore(context.Background(), db, AuthorizerFunc(func(_ kernel.ExecutionContext, permission string) bool { return permission == PermissionManage }), time.Now)
	if err != nil {
		t.Fatalf("OpenGORMStore() error = %v", err)
	}
	execution := tenantExecution("tenant-a")
	if _, err := store.Create(execution, Placement{TenantCode: "tenant-a", CapabilityID: "orders", DatasourceGroupID: "orders-primary", Version: 1, Status: StatusActive}, "initial-placement"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := store.Transition(execution, Transition{TenantCode: "tenant-a", CapabilityID: "orders", DatasourceGroupID: "orders-secondary", Version: 2, Status: StatusMigrating, Reason: "capacity-rebalance"}); err != nil {
		t.Fatalf("Transition() error = %v", err)
	}
	reopened, err := OpenGORMStore(context.Background(), db, AuthorizerFunc(func(_ kernel.ExecutionContext, permission string) bool { return permission == PermissionManage }), time.Now)
	if err != nil {
		t.Fatalf("reopen GORM store: %v", err)
	}
	resolved, err := reopened.Resolve(execution, "orders", Override{})
	if err != nil || resolved.DatasourceGroupID != "orders-secondary" || resolved.Version != 2 {
		t.Fatalf("reopened Resolve() = %+v, %v", resolved, err)
	}
	if audit, err := reopened.Audit(context.Background()); err != nil || len(audit) != 2 || audit[1].Kind != "transition" {
		t.Fatalf("reopened Audit() = %+v, %v", audit, err)
	}
}

func TestStoreRejectsUntrustedTenantAndUnscopedOverride(t *testing.T) {
	store, err := NewStore([]Placement{{
		TenantCode: "tenant-a", CapabilityID: "orders", DatasourceGroupID: "orders-primary", Version: 1, Status: StatusActive,
	}}, AuthorizerFunc(func(_ kernel.ExecutionContext, permission string) bool { return permission == PermissionOverride }), time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Resolve(tenantExecution("tenant-b"), "orders", Override{}); !errors.Is(err, ErrPlacementNotFound) {
		t.Fatalf("Resolve(untrusted tenant) error = %v, want ErrPlacementNotFound", err)
	}
	if _, err := store.Resolve(tenantExecution("tenant-a"), "orders", Override{DatasourceGroupID: "orders-secondary"}); !errors.Is(err, ErrInvalidOverride) {
		t.Fatalf("Resolve(unscoped override) error = %v, want ErrInvalidOverride", err)
	}
}

func TestStoreAppliesOnlyAuthorizedBoundedOverride(t *testing.T) {
	now := time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC)
	store, err := NewStore([]Placement{{
		TenantCode: "tenant-a", CapabilityID: "orders", DatasourceGroupID: "orders-primary", Version: 1, Status: StatusActive,
	}}, AuthorizerFunc(func(_ kernel.ExecutionContext, permission string) bool { return permission == PermissionOverride }), func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := store.Resolve(tenantExecution("tenant-a"), "orders", Override{
		TenantCode: "tenant-a", CapabilityID: "orders", DatasourceGroupID: "orders-recovery",
		ExpiresAt: now.Add(10 * time.Minute), Reason: "approved-incident-123",
	})
	if err != nil {
		t.Fatalf("Resolve(override) error = %v", err)
	}
	if resolved.DatasourceGroupID != "orders-recovery" || resolved.Version != 1 {
		t.Fatalf("Resolve(override) = %+v", resolved)
	}
	if len(store.Audit()) != 1 || store.Audit()[0].Kind != "override" {
		t.Fatalf("Audit() = %+v, want override audit", store.Audit())
	}
	if _, err := store.Resolve(tenantExecution("tenant-a"), "orders", Override{
		TenantCode: "tenant-a", CapabilityID: "orders", DatasourceGroupID: "orders-recovery",
		ExpiresAt: now.Add(16 * time.Minute), Reason: "too-long",
	}); !errors.Is(err, ErrInvalidOverride) {
		t.Fatalf("Resolve(long override) error = %v, want ErrInvalidOverride", err)
	}
}

func tenantExecution(tenant string) kernel.ExecutionContext {
	return kernel.ExecutionContext{
		Context: context.Background(), Actor: kernel.Actor{Username: "ops", Kind: kernel.ActorKindUser},
		TenantScope: kernel.TenantScope{TenantID: 1, TenantCode: tenant}, PermissionIntent: kernel.PermissionIntent{Code: "service-object", Action: "execute"},
	}
}
