package datasource

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/GnosiST/platform-go/internal/platform/storage"
)

func TestRouterPinsWritesAndTransactionsToPrimaryAndFallsBackFromReplica(t *testing.T) {
	now := time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC)
	runtime, err := Open(context.Background(), Spec{
		Version:     "v1",
		Datasources: []Datasource{{ID: "primary", ConfigRef: "primary"}, {ID: "replica", ConfigRef: "replica"}},
		Groups:      []Group{{ID: "orders", PrimaryDatasourceID: "primary", ReplicaDatasourceIDs: []string{"replica"}, Capabilities: []string{"orders"}}},
	}, ResolverFunc(func(_ context.Context, ref string) (storage.Config, error) {
		return storage.Config{Driver: "sqlite", DSN: filepath.Join(t.TempDir(), ref+".db")}, nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = runtime.Close() })
	health := replicaHealth{"replica": {Healthy: true, Lag: time.Second}}
	router, err := NewRouter(runtime, RoutingPolicy{StickyWindow: time.Minute, MaximumReplicaLag: 5 * time.Second}, health, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	read, err := router.Route(context.Background(), RouteRequest{TenantCode: "tenant-a", GroupID: "orders", Operation: OperationRead})
	if err != nil || read.DatasourceID() != "replica" {
		t.Fatalf("Route(read) = %+v, %v; want replica", read, err)
	}
	write, err := router.Route(context.Background(), RouteRequest{TenantCode: "tenant-a", GroupID: "orders", Operation: OperationWrite})
	if err != nil || write.DatasourceID() != "primary" {
		t.Fatalf("Route(write) = %+v, %v; want primary", write, err)
	}
	router.RecordWrite("tenant-a", "orders")
	stickyRead, err := router.Route(context.Background(), RouteRequest{TenantCode: "tenant-a", GroupID: "orders", Operation: OperationRead})
	if err != nil || stickyRead.DatasourceID() != "primary" {
		t.Fatalf("Route(sticky read) = %+v, %v; want primary", stickyRead, err)
	}
	now = now.Add(2 * time.Minute)
	health["replica"] = ReplicaHealth{Healthy: false}
	fallback, err := router.Route(context.Background(), RouteRequest{TenantCode: "tenant-a", GroupID: "orders", Operation: OperationRead})
	if err != nil || fallback.DatasourceID() != "primary" {
		t.Fatalf("Route(unhealthy replica) = %+v, %v; want primary fallback", fallback, err)
	}
	transaction, err := router.Route(context.Background(), RouteRequest{TenantCode: "tenant-a", GroupID: "orders", Operation: OperationRead, Transaction: true})
	if err != nil || transaction.DatasourceID() != "primary" {
		t.Fatalf("Route(transaction) = %+v, %v; want primary", transaction, err)
	}
}

type replicaHealth map[string]ReplicaHealth

func (h replicaHealth) Health(_ context.Context, datasourceID string) (ReplicaHealth, error) {
	return h[datasourceID], nil
}
