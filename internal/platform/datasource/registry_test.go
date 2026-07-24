package datasource

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/GnosiST/platform-go/internal/platform/storage"
)

func TestOpenResolvesProtectedReferenceAndPinsCapabilityTransactions(t *testing.T) {
	ctx := context.Background()
	runtime, err := Open(ctx, Spec{
		Version:     "v1",
		Datasources: []Datasource{{ID: "primary", ConfigRef: "orders-primary"}},
		Groups:      []Group{{ID: "orders", PrimaryDatasourceID: "primary", Capabilities: []string{"orders"}}},
	}, ResolverFunc(func(_ context.Context, ref string) (storage.Config, error) {
		if ref != "orders-primary" {
			t.Fatalf("resolver reference = %q, want orders-primary", ref)
		}
		return storage.Config{Driver: "sqlite", DSN: filepath.Join(t.TempDir(), "orders.db")}, nil
	}))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = runtime.Close() })

	if got, err := runtime.DatasourceID("orders"); err != nil || got != "primary" {
		t.Fatalf("DatasourceID(orders) = %q, %v; want primary, nil", got, err)
	}
	if got, err := runtime.DatasourceIDForGroup("orders"); err != nil || got != "primary" {
		t.Fatalf("DatasourceIDForGroup(orders) = %q, %v; want primary, nil", got, err)
	}
	if db, err := runtime.DBForGroup("orders"); err != nil || db.DatasourceID() != "primary" {
		t.Fatalf("DBForGroup(orders) = %+v, %v; want primary database", db, err)
	}
	if err := runtime.Transaction(ctx, "orders", func(tx *DB) error {
		if tx.DatasourceID() != "primary" {
			t.Fatalf("transaction datasource = %q, want primary", tx.DatasourceID())
		}
		return nil
	}); err != nil {
		t.Fatalf("Transaction() error = %v", err)
	}
	if err := runtime.TransactionForGroup(ctx, "orders", func(tx *DB) error {
		if tx.DatasourceID() != "primary" {
			t.Fatalf("group transaction datasource = %q, want primary", tx.DatasourceID())
		}
		return nil
	}); err != nil {
		t.Fatalf("TransactionForGroup() error = %v", err)
	}
	if _, err := runtime.DatasourceID("unbound-capability"); !errors.Is(err, ErrCapabilityUnbound) {
		t.Fatalf("DatasourceID(unbound) error = %v, want ErrCapabilityUnbound", err)
	}
	for _, health := range runtime.Health() {
		if health.DatasourceID == "primary" && !health.Ready {
			t.Fatalf("primary health = %+v, want ready", health)
		}
	}
}

func TestOpenRejectsAmbiguousCapabilityBinding(t *testing.T) {
	_, err := Open(context.Background(), Spec{
		Version:     "v1",
		Datasources: []Datasource{{ID: "one", ConfigRef: "one"}, {ID: "two", ConfigRef: "two"}},
		Groups: []Group{
			{ID: "one", PrimaryDatasourceID: "one", Capabilities: []string{"orders"}},
			{ID: "two", PrimaryDatasourceID: "two", Capabilities: []string{"orders"}},
		},
	}, ResolverFunc(func(context.Context, string) (storage.Config, error) {
		return storage.Config{}, nil
	}))
	if !errors.Is(err, ErrInvalidSpec) {
		t.Fatalf("Open() error = %v, want ErrInvalidSpec", err)
	}
}
