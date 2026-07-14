package bootstrap

import (
	"database/sql"
	"database/sql/driver"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"platform-go/internal/platform/adminresource"
	"platform-go/internal/platform/capability"
	"platform-go/internal/platform/config"
	"platform-go/internal/platform/core"
	"platform-go/internal/platform/dataprotection"
)

func TestAdminResourcesFromConfigUsesMemoryStoreByDefault(t *testing.T) {
	store, err := AdminResourcesFromConfig(config.Config{}, core.DefaultManifests(), nil)
	if err != nil {
		t.Fatalf("AdminResourcesFromConfig() error = %v", err)
	}

	if _, err := store.List("tenants"); err != nil {
		t.Fatalf("List(tenants) error = %v", err)
	}
}

func TestAdminResourcesFromConfigUsesFileBackedStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "admin-resources.json")
	store, err := AdminResourcesFromConfig(config.Config{AdminResourceFile: path}, core.DefaultManifests(), nil)
	if err != nil {
		t.Fatalf("AdminResourcesFromConfig() error = %v", err)
	}
	created, err := store.Create("tenants", adminresource.WriteInput{
		Code:   "persisted",
		Name:   "Persisted Tenant",
		Status: "enabled",
		Values: map[string]string{"isolation": "sandbox"},
	})
	if err != nil {
		t.Fatalf("Create(tenants) error = %v", err)
	}

	reloaded, err := AdminResourcesFromConfig(config.Config{AdminResourceFile: path}, core.DefaultManifests(), nil)
	if err != nil {
		t.Fatalf("reload AdminResourcesFromConfig() error = %v", err)
	}
	tenants, err := reloaded.List("tenants")
	if err != nil {
		t.Fatalf("List(tenants) error = %v", err)
	}
	for _, tenant := range tenants {
		if tenant.ID == created.ID {
			return
		}
	}
	t.Fatalf("reloaded tenants missing %q: %+v", created.ID, tenants)
}

func TestAdminResourcesFromConfigUsesSQLStore(t *testing.T) {
	cfg := config.Config{
		AdminResourceDriver: "platform_admin_resource_test",
		AdminResourceDSN:    "bootstrap-admin-resources",
	}
	store, err := AdminResourcesFromConfig(cfg, core.DefaultManifests(), nil)
	if err != nil {
		t.Fatalf("AdminResourcesFromConfig() error = %v", err)
	}
	if _, err := store.Update("roles", "role-operator", adminresource.WriteInput{
		Name:   "Operator",
		Status: "enabled",
		Values: map[string]string{"groupCode": "operations", "dataScope": "current_org", "permissions": "admin:user:read"},
	}); err != nil {
		t.Fatalf("Update(role-operator) error = %v", err)
	}

	reloaded, err := AdminResourcesFromConfig(cfg, core.DefaultManifests(), nil)
	if err != nil {
		t.Fatalf("reload AdminResourcesFromConfig() error = %v", err)
	}
	menus := reloaded.MenuItemsForPrincipal(reloaded.CurrentPrincipal("ops"))
	for _, menu := range menus {
		if menu.Route == "/users" {
			return
		}
	}
	t.Fatalf("menus after SQL reload = %+v, want /users", menus)
}

func TestAdminResourcesFromConfigUsesGORMStoreForSupportedDrivers(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "admin-resources.db")
	cfg := config.Config{
		AdminResourceDriver: "sqlite",
		AdminResourceDSN:    dsn,
	}
	store, err := AdminResourcesFromConfig(cfg, core.DefaultManifests(), nil)
	if err != nil {
		t.Fatalf("AdminResourcesFromConfig() error = %v", err)
	}
	if _, err := store.Update("roles", "role-operator", adminresource.WriteInput{
		Name:   "Operator",
		Status: "enabled",
		Values: map[string]string{"groupCode": "operations", "dataScope": "current_org", "permissions": "admin:user:read"},
	}); err != nil {
		t.Fatalf("Update(role-operator) error = %v", err)
	}

	reloaded, err := AdminResourcesFromConfig(cfg, core.DefaultManifests(), nil)
	if err != nil {
		t.Fatalf("reload AdminResourcesFromConfig() error = %v", err)
	}
	menus := reloaded.MenuItemsForPrincipal(reloaded.CurrentPrincipal("ops"))
	for _, menu := range menus {
		if menu.Route == "/users" {
			return
		}
	}
	t.Fatalf("menus after GORM reload = %+v, want /users", menus)
}

func TestAdminResourcesFromConfigRejectsSQLStoreWithoutDSN(t *testing.T) {
	_, err := AdminResourcesFromConfig(config.Config{AdminResourceDriver: "platform_admin_resource_test"}, core.DefaultManifests(), nil)
	if err == nil {
		t.Fatalf("AdminResourcesFromConfig() error = nil, want missing DSN")
	}
	if !strings.Contains(err.Error(), "admin resource dsn is required") {
		t.Fatalf("AdminResourcesFromConfig() error = %v, want missing DSN", err)
	}
}

func TestAdminResourcesFromConfigInjectsProtectionBeforePersistentLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "protected-admin-resources.json")
	cfg := config.Config{AdminResourceFile: path}
	manifests := bootstrapProtectedManifests()
	runtime, err := DataProtectionRuntimeFromConfig(dataProtectionConfigForTest(config.RuntimeEnvironmentTest, dataprotection.ProviderLocalTest))
	if err != nil {
		t.Fatal(err)
	}
	store, err := AdminResourcesFromConfig(cfg, manifests, runtime)
	if err != nil {
		t.Fatalf("AdminResourcesFromConfig() error = %v", err)
	}
	if _, err := store.Create("protected-bootstrap-records", adminresource.WriteInput{
		Code: "protected-1", Name: "Protected", Values: map[string]string{"governmentReference": "bootstrap-secret-marker"},
	}); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), "bootstrap-secret-marker") || !strings.Contains(string(content), "pgo:enc:v1:") {
		t.Fatalf("protected repository content = %s", content)
	}
	if _, err := AdminResourcesFromConfig(cfg, manifests, runtime); err != nil {
		t.Fatalf("reload with historical keys error = %v", err)
	}
	replaced, err := DataProtectionRuntimeFromConfig(replacedDataProtectionConfigForTest())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AdminResourcesFromConfig(cfg, manifests, replaced); err == nil {
		t.Fatal("reload with replaced historical key error = nil")
	}
}

func bootstrapProtectedManifests() []capability.Manifest {
	return append(core.DefaultManifests(), capability.Manifest{
		ID: "protected-bootstrap-test",
		Admin: capability.AdminSurface{Resources: []capability.AdminResource{{
			Resource: "protected-bootstrap-records", Title: capability.Text("受保护记录", "Protected Records"), Description: capability.Text("测试。", "Test."),
			PermissionPrefix: "admin:protected-bootstrap", Deletion: &capability.AdminResourceDeletionPolicy{Mode: capability.AdminDeletionSoftDelete, PolicyVersion: 1},
			Protection: &capability.AdminResourceProtection{SchemaVersion: 1, Scope: "global"},
			Fields: []capability.AdminField{
				{Key: "code", Label: capability.Text("编码", "Code"), Type: "text", Source: "record", Required: true, InForm: true},
				{Key: "name", Label: capability.Text("名称", "Name"), Type: "text", Source: "record", Required: true, InForm: true},
				{Key: "governmentReference", Label: capability.Text("政府引用", "Government Reference"), Type: "text", Source: "values", InForm: true,
					Sensitivity: capability.FieldSensitivitySensitive, StorageMode: capability.FieldStorageEncrypted,
					ResponseMode: capability.FieldProjectionPrivileged, ExportMode: capability.FieldProjectionOmitted,
					Protection: &capability.AdminFieldProtection{Format: dataprotection.FormatAES256GCMV1, Normalization: dataprotection.NormalizationRawV1}},
			},
		}}},
	})
}

func replacedDataProtectionConfigForTest() config.Config {
	cfg := dataProtectionConfigForTest(config.RuntimeEnvironmentTest, dataprotection.ProviderLocalTest)
	encoded := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("x", 32)))
	cfg.DataEncryptionKeyringJSON = "{\"enc-v1\":\"" + encoded + "\"}"
	return cfg
}

var bootstrapAdminResourceDriverState sync.Map

func init() {
	sql.Register("platform_admin_resource_test", bootstrapAdminResourceDriver{})
}

type bootstrapAdminResourceDriver struct{}

func (bootstrapAdminResourceDriver) Open(name string) (driver.Conn, error) {
	stateValue, _ := bootstrapAdminResourceDriverState.LoadOrStore(name, &bootstrapAdminResourceState{})
	return &bootstrapAdminResourceConn{state: stateValue.(*bootstrapAdminResourceState)}, nil
}

type bootstrapAdminResourceState struct {
	mu       sync.Mutex
	nextID   string
	revision string
	records  []bootstrapAdminResourceRecord
}

type bootstrapAdminResourceRecord struct {
	Resource    string
	ID          string
	Code        string
	Name        string
	Status      string
	Description string
	UpdatedAt   string
	ValuesJSON  string
}

type bootstrapAdminResourceConn struct {
	state *bootstrapAdminResourceState
}

func (c *bootstrapAdminResourceConn) Prepare(query string) (driver.Stmt, error) {
	return nil, driver.ErrSkip
}

func (c *bootstrapAdminResourceConn) Close() error {
	return nil
}

func (c *bootstrapAdminResourceConn) Begin() (driver.Tx, error) {
	return bootstrapAdminResourceTx{}, nil
}

type bootstrapAdminResourceTx struct{}

func (bootstrapAdminResourceTx) Commit() error   { return nil }
func (bootstrapAdminResourceTx) Rollback() error { return nil }

func (c *bootstrapAdminResourceConn) Exec(query string, args []driver.Value) (driver.Result, error) {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	switch {
	case strings.Contains(query, "CREATE TABLE"):
		return driver.RowsAffected(0), nil
	case strings.Contains(query, "DELETE FROM platform_admin_resource_records"):
		c.state.records = nil
		return driver.RowsAffected(1), nil
	case strings.Contains(query, "DELETE FROM platform_admin_resource_lifecycle"):
		return driver.RowsAffected(0), nil
	case strings.Contains(query, "DELETE FROM platform_admin_resource_state"):
		if len(args) > 0 {
			switch fmt.Sprint(args[0]) {
			case "next_id":
				c.state.nextID = ""
			case "revision":
				c.state.revision = ""
			}
		}
		return driver.RowsAffected(1), nil
	case strings.Contains(query, "UPDATE platform_admin_resource_state"):
		if len(args) != 3 || fmt.Sprint(args[1]) != "revision" || c.state.revision != fmt.Sprint(args[2]) {
			return driver.RowsAffected(0), nil
		}
		c.state.revision = fmt.Sprint(args[0])
		return driver.RowsAffected(1), nil
	case strings.Contains(query, "INSERT INTO platform_admin_resource_state"):
		switch fmt.Sprint(args[0]) {
		case "next_id":
			c.state.nextID = fmt.Sprint(args[1])
		case "revision":
			c.state.revision = fmt.Sprint(args[1])
		}
		return driver.RowsAffected(1), nil
	case strings.Contains(query, "INSERT INTO platform_admin_resource_records"):
		c.state.records = append(c.state.records, bootstrapAdminResourceRecord{
			Resource:    fmt.Sprint(args[0]),
			ID:          fmt.Sprint(args[1]),
			Code:        fmt.Sprint(args[2]),
			Name:        fmt.Sprint(args[3]),
			Status:      fmt.Sprint(args[4]),
			Description: fmt.Sprint(args[5]),
			UpdatedAt:   fmt.Sprint(args[6]),
			ValuesJSON:  fmt.Sprint(args[7]),
		})
		return driver.RowsAffected(1), nil
	default:
		return driver.RowsAffected(0), nil
	}
}

func (c *bootstrapAdminResourceConn) Query(query string, args []driver.Value) (driver.Rows, error) {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	if strings.Contains(query, "platform_admin_resource_state") {
		values := [][]driver.Value{}
		if len(args) > 0 {
			var value string
			switch fmt.Sprint(args[0]) {
			case "next_id":
				value = c.state.nextID
			case "revision":
				value = c.state.revision
			}
			if value != "" {
				values = append(values, []driver.Value{value})
			}
		}
		return &bootstrapAdminResourceRows{columns: []string{"value"}, values: values}, nil
	}
	if strings.Contains(query, "platform_admin_resource_lifecycle") {
		return &bootstrapAdminResourceRows{
			columns: []string{"resource", "record_id", "deleted_at", "deleted_by", "delete_reason", "purge_after", "deletion_policy_version"},
		}, nil
	}
	values := make([][]driver.Value, 0, len(c.state.records))
	for _, record := range c.state.records {
		values = append(values, []driver.Value{
			record.Resource,
			record.ID,
			record.Code,
			record.Name,
			record.Status,
			record.Description,
			record.UpdatedAt,
			record.ValuesJSON,
		})
	}
	return &bootstrapAdminResourceRows{
		columns: []string{"resource", "id", "code", "name", "status", "description", "updated_at", "values_json"},
		values:  values,
	}, nil
}

type bootstrapAdminResourceRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *bootstrapAdminResourceRows) Columns() []string {
	return r.columns
}

func (r *bootstrapAdminResourceRows) Close() error {
	return nil
}

func (r *bootstrapAdminResourceRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}
