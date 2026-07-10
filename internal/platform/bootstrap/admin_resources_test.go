package bootstrap

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"platform-go/internal/platform/adminresource"
	"platform-go/internal/platform/config"
	"platform-go/internal/platform/core"
)

func TestAdminResourcesFromConfigUsesMemoryStoreByDefault(t *testing.T) {
	store, err := AdminResourcesFromConfig(config.Config{}, core.DefaultManifests())
	if err != nil {
		t.Fatalf("AdminResourcesFromConfig() error = %v", err)
	}

	if _, err := store.List("tenants"); err != nil {
		t.Fatalf("List(tenants) error = %v", err)
	}
}

func TestAdminResourcesFromConfigUsesFileBackedStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "admin-resources.json")
	store, err := AdminResourcesFromConfig(config.Config{AdminResourceFile: path}, core.DefaultManifests())
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

	reloaded, err := AdminResourcesFromConfig(config.Config{AdminResourceFile: path}, core.DefaultManifests())
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
	store, err := AdminResourcesFromConfig(cfg, core.DefaultManifests())
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

	reloaded, err := AdminResourcesFromConfig(cfg, core.DefaultManifests())
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
	store, err := AdminResourcesFromConfig(cfg, core.DefaultManifests())
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

	reloaded, err := AdminResourcesFromConfig(cfg, core.DefaultManifests())
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
	_, err := AdminResourcesFromConfig(config.Config{AdminResourceDriver: "platform_admin_resource_test"}, core.DefaultManifests())
	if err == nil {
		t.Fatalf("AdminResourcesFromConfig() error = nil, want missing DSN")
	}
	if !strings.Contains(err.Error(), "admin resource dsn is required") {
		t.Fatalf("AdminResourcesFromConfig() error = %v, want missing DSN", err)
	}
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
	mu      sync.Mutex
	nextID  string
	records []bootstrapAdminResourceRecord
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
	return nil, driver.ErrSkip
}

func (c *bootstrapAdminResourceConn) Exec(query string, args []driver.Value) (driver.Result, error) {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	switch {
	case strings.Contains(query, "CREATE TABLE"):
		return driver.RowsAffected(0), nil
	case strings.Contains(query, "DELETE FROM platform_admin_resource_records"):
		c.state.records = nil
		return driver.RowsAffected(1), nil
	case strings.Contains(query, "DELETE FROM platform_admin_resource_state"):
		c.state.nextID = ""
		return driver.RowsAffected(1), nil
	case strings.Contains(query, "INSERT INTO platform_admin_resource_state"):
		c.state.nextID = fmt.Sprint(args[1])
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
		if c.state.nextID != "" {
			values = append(values, []driver.Value{c.state.nextID})
		}
		return &bootstrapAdminResourceRows{columns: []string{"value"}, values: values}, nil
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
