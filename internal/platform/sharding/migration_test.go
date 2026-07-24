package sharding

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/GnosiST/platform-go/internal/platform/kernel"
	"github.com/GnosiST/platform-go/internal/platform/storage"
)

func TestMigrationTracksVersionedShardPlacementCheckpointsAndCompensation(t *testing.T) {
	now := time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC)
	controller, err := NewController(AuthorizerFunc(func(_ kernel.ExecutionContext, permission string) bool { return permission == PermissionManage }), func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	execution := migrationExecution("tenant-a")
	migration, err := controller.Create(execution, Migration{
		ID: "tenant-a-orders-rebalance-1", TenantCode: "tenant-a", CapabilityID: "orders", SourceShardID: "orders-a", TargetShardID: "orders-b",
		RoutingVersion: 2, Checkpoint: Checkpoint{Sequence: 1, Cursor: "initial"}, Status: StatusPrepared, Reason: "capacity-rebalance",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if migration.GlobalID == "" || migration.Status != StatusPrepared {
		t.Fatalf("Create() = %+v", migration)
	}
	migration, err = controller.Checkpoint(execution, migration.ID, Checkpoint{Sequence: 2, Cursor: "records-100"})
	if err != nil || migration.Checkpoint.Sequence != 2 || migration.Status != StatusRunning {
		t.Fatalf("Checkpoint() = %+v, %v", migration, err)
	}
	migration, err = controller.Compensate(execution, migration.ID, "target-validation-failed")
	if err != nil || migration.Status != StatusCompensated {
		t.Fatalf("Compensate() = %+v, %v", migration, err)
	}
	if audit := controller.Audit(); len(audit) != 3 || audit[2].Kind != "compensate" {
		t.Fatalf("Audit() = %+v", audit)
	}
}

func TestMigrationRejectsClientShardInputAndNonMonotonicCheckpoints(t *testing.T) {
	controller, err := NewController(AuthorizerFunc(func(_ kernel.ExecutionContext, permission string) bool { return permission == PermissionManage }), time.Now)
	if err != nil {
		t.Fatal(err)
	}
	execution := migrationExecution("tenant-a")
	migration, err := controller.Create(execution, Migration{ID: "tenant-a-orders-rebalance-1", TenantCode: "tenant-a", CapabilityID: "orders", SourceShardID: "orders-a", TargetShardID: "orders-b", RoutingVersion: 2, Checkpoint: Checkpoint{Sequence: 1, Cursor: "initial"}, Status: StatusPrepared, Reason: "capacity-rebalance"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := controller.Checkpoint(execution, migration.ID, Checkpoint{Sequence: 1, Cursor: "repeat"}); !errors.Is(err, ErrInvalidMigration) {
		t.Fatalf("Checkpoint(non-monotonic) error = %v, want ErrInvalidMigration", err)
	}
	if _, err := controller.Create(migrationExecution("tenant-b"), Migration{ID: "tenant-a-orders-rebalance-2", TenantCode: "tenant-a", CapabilityID: "orders", SourceShardID: "orders-a", TargetShardID: "orders-b", RoutingVersion: 2, Checkpoint: Checkpoint{Sequence: 1, Cursor: "initial"}, Status: StatusPrepared, Reason: "capacity-rebalance"}); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("Create(cross-tenant) error = %v, want ErrUnauthorized", err)
	}
}

func TestGORMControllerPersistsMigrationCheckpointsAndCompensationAudit(t *testing.T) {
	db, err := storage.OpenGORM(storage.Config{Driver: "sqlite", DSN: t.TempDir() + "/sharding.db"})
	if err != nil {
		t.Fatal(err)
	}
	if err := PrepareGORMController(context.Background(), db); err != nil {
		t.Fatalf("PrepareGORMController() error = %v", err)
	}
	authorizer := AuthorizerFunc(func(_ kernel.ExecutionContext, permission string) bool { return permission == PermissionManage })
	controller, err := OpenGORMController(context.Background(), db, authorizer, time.Now)
	if err != nil {
		t.Fatalf("OpenGORMController() error = %v", err)
	}
	execution := migrationExecution("tenant-a")
	migration, err := controller.Create(execution, Migration{
		ID: "tenant-a-orders-rebalance-1", TenantCode: "tenant-a", CapabilityID: "orders", SourceShardID: "orders-a", TargetShardID: "orders-b",
		RoutingVersion: 2, Checkpoint: Checkpoint{Sequence: 1, Cursor: "initial"}, Status: StatusPrepared, Reason: "capacity-rebalance",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := controller.Checkpoint(execution, migration.ID, Checkpoint{Sequence: 2, Cursor: "records-100"}); err != nil {
		t.Fatalf("Checkpoint() error = %v", err)
	}
	if _, err := controller.Compensate(execution, migration.ID, "target-validation-failed"); err != nil {
		t.Fatalf("Compensate() error = %v", err)
	}
	reopened, err := OpenGORMController(context.Background(), db, authorizer, time.Now)
	if err != nil {
		t.Fatalf("reopen GORM controller: %v", err)
	}
	audit, err := reopened.Audit(context.Background())
	if err != nil || len(audit) != 3 || audit[1].Checkpoint.Sequence != 2 || audit[2].Status != StatusCompensated {
		t.Fatalf("reopened Audit() = %+v, %v", audit, err)
	}
}

func TestRouterResolvesOnlyTrustedTenantAndCapabilityRoute(t *testing.T) {
	router, err := NewRouter([]Route{{TenantCode: "tenant-a", CapabilityID: "orders", ShardID: "orders-a", DatasourceGroupID: "orders-primary", Version: 2}})
	if err != nil {
		t.Fatal(err)
	}
	route, err := router.Resolve(migrationExecution("tenant-a"), "orders")
	if err != nil || route.ShardID != "orders-a" || route.Version != 2 {
		t.Fatalf("Resolve() = %+v, %v", route, err)
	}
	if _, err := router.Resolve(migrationExecution("tenant-b"), "orders"); !errors.Is(err, ErrRouteNotFound) {
		t.Fatalf("Resolve(unbound tenant) error = %v, want ErrRouteNotFound", err)
	}
}

func migrationExecution(tenant string) kernel.ExecutionContext {
	return kernel.ExecutionContext{Context: context.Background(), Actor: kernel.Actor{Username: "ops", Kind: kernel.ActorKindUser}, TenantScope: kernel.TenantScope{TenantID: 1, TenantCode: tenant}, PermissionIntent: kernel.PermissionIntent{Code: "service-object", Action: "execute"}}
}
