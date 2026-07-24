package federatedreport

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/GnosiST/platform-go/internal/platform/kernel"
	"github.com/GnosiST/platform-go/internal/platform/storage"
)

func TestRuntimeExecutesRegisteredReadOnlyReportWithTenantProjectionAndLimits(t *testing.T) {
	runtime, err := NewRuntime([]Definition{{
		ID: "orders-summary", Version: 1, Sources: []Source{{ID: "orders", DatasourceGroupID: "orders-primary"}},
		AllowedFields: []string{"count", "status"}, MaximumRows: 10, MaximumCost: 10, Timeout: time.Second,
	}}, map[string]SourceExecutor{"orders": SourceExecutorFunc(func(_ context.Context, request SourceRequest) ([]map[string]any, error) {
		if request.TenantCode != "tenant-a" || request.DatasourceGroupID != "orders-primary" {
			t.Fatalf("source request = %+v", request)
		}
		return []map[string]any{{"count": 3, "status": "open", "internal": "must-not-leak"}}, nil
	})})
	if err != nil {
		t.Fatal(err)
	}
	result, err := runtime.Execute(execution("tenant-a"), Request{ReportID: "orders-summary", Version: 1})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(result.Rows) != 1 || result.Rows[0]["count"] != 3 || result.Rows[0]["internal"] != nil {
		t.Fatalf("Execute() = %+v", result)
	}
}

func TestGORMSourceExecutorReadsOnlyRegisteredTenantScopedSources(t *testing.T) {
	ordersDB, err := storage.OpenGORM(storage.Config{Driver: "sqlite", DSN: t.TempDir() + "/orders.db"})
	if err != nil {
		t.Fatal(err)
	}
	profilesDB, err := storage.OpenGORM(storage.Config{Driver: "sqlite", DSN: t.TempDir() + "/profiles.db"})
	if err != nil {
		t.Fatal(err)
	}
	if err := ordersDB.Exec("CREATE TABLE report_orders (tenant_code TEXT, count INTEGER, internal TEXT)").Error; err != nil {
		t.Fatal(err)
	}
	if err := profilesDB.Exec("CREATE TABLE report_profiles (tenant_code TEXT, status TEXT, internal TEXT)").Error; err != nil {
		t.Fatal(err)
	}
	if err := ordersDB.Exec("INSERT INTO report_orders VALUES ('tenant-a', 3, 'secret'), ('tenant-b', 9, 'other')").Error; err != nil {
		t.Fatal(err)
	}
	if err := profilesDB.Exec("INSERT INTO report_profiles VALUES ('tenant-a', 'open', 'secret'), ('tenant-b', 'closed', 'other')").Error; err != nil {
		t.Fatal(err)
	}
	executors, err := NewGORMSourceExecutors([]GORMSourceBinding{{ID: "orders", DatasourceGroupID: "orders-primary", Table: "report_orders", Fields: []string{"count"}, DB: ordersDB}, {ID: "profiles", DatasourceGroupID: "profiles-primary", Table: "report_profiles", Fields: []string{"status"}, DB: profilesDB}})
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := NewRuntime([]Definition{{ID: "summary", Version: 1, Sources: []Source{{ID: "orders", DatasourceGroupID: "orders-primary"}, {ID: "profiles", DatasourceGroupID: "profiles-primary"}}, AllowedFields: []string{"count", "status"}, MaximumRows: 4, MaximumCost: 4, Timeout: time.Second}}, executors)
	if err != nil {
		t.Fatal(err)
	}
	result, err := runtime.Execute(execution("tenant-a"), Request{ReportID: "summary", Version: 1})
	if err != nil || len(result.Rows) != 2 || result.Rows[0]["count"] != int64(3) || result.Rows[1]["status"] != "open" {
		t.Fatalf("Execute() = %+v, %v", result, err)
	}
}

func TestGORMDefinitionStoreRestoresVersionedRegisteredReports(t *testing.T) {
	db, err := storage.OpenGORM(storage.Config{Driver: "sqlite", DSN: t.TempDir() + "/reports.db"})
	if err != nil {
		t.Fatal(err)
	}
	if err := PrepareGORMDefinitionStore(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	store, err := OpenGORMDefinitionStore(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	definition := Definition{ID: "orders-summary", Version: 1, Sources: []Source{{ID: "orders", DatasourceGroupID: "orders-primary"}}, AllowedFields: []string{"count"}, MaximumRows: 10, MaximumCost: 10, Timeout: time.Second}
	if err := store.Save(context.Background(), definition); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenGORMDefinitionStore(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := reopened.Runtime(map[string]SourceExecutor{"orders": SourceExecutorFunc(func(context.Context, SourceRequest) ([]map[string]any, error) {
		return []map[string]any{{"count": 3}}, nil
	})})
	if err != nil {
		t.Fatal(err)
	}
	result, err := runtime.Execute(execution("tenant-a"), Request{ReportID: "orders-summary", Version: 1})
	if err != nil || len(result.Rows) != 1 || result.Rows[0]["count"] != 3 {
		t.Fatalf("Execute() = %+v, %v", result, err)
	}
}

func TestRuntimeFailsClosedForUntrustedTenantAndTimeout(t *testing.T) {
	runtime, err := NewRuntime([]Definition{{
		ID: "orders-summary", Version: 1, Sources: []Source{{ID: "orders", DatasourceGroupID: "orders-primary"}},
		AllowedFields: []string{"count"}, MaximumRows: 10, MaximumCost: 10, Timeout: time.Millisecond,
	}}, map[string]SourceExecutor{"orders": SourceExecutorFunc(func(ctx context.Context, _ SourceRequest) ([]map[string]any, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.Execute(execution(""), Request{ReportID: "orders-summary", Version: 1}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Execute(untrusted) error = %v, want ErrInvalidRequest", err)
	}
	if _, err := runtime.Execute(execution("tenant-a"), Request{ReportID: "orders-summary", Version: 1}); !errors.Is(err, ErrExecutionFailed) {
		t.Fatalf("Execute(timeout) error = %v, want ErrExecutionFailed", err)
	}
}

func execution(tenant string) kernel.ExecutionContext {
	return kernel.ExecutionContext{Context: context.Background(), Actor: kernel.Actor{Username: "ops", Kind: kernel.ActorKindUser}, TenantScope: kernel.TenantScope{TenantID: 1, TenantCode: tenant}, PermissionIntent: kernel.PermissionIntent{Code: "report", Action: "read"}}
}
