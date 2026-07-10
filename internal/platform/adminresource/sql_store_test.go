package adminresource

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync"
	"testing"

	"platform-go/internal/platform/core"
)

func TestSQLAdminResourceRepositoryPersistsSnapshots(t *testing.T) {
	db := openAdminResourceTestDB(t)
	repository, err := NewSQLAdminResourceRepository(context.Background(), db)
	if err != nil {
		t.Fatalf("NewSQLAdminResourceRepository() error = %v", err)
	}
	snapshot := ResourceSnapshot{
		Revision: 4,
		NextID:   1042,
		Resources: map[string][]Record{
			"tenants": {
				{ID: "tenant-1042", Code: "acme", Name: "Acme Tenant", Status: "enabled", Description: "SQL tenant", UpdatedAt: "2026-07-04T00:00:00Z", Values: map[string]string{"isolation": "sandbox"}},
			},
			"roles": {
				{ID: "role-operator", Code: "operator", Name: "Operator", Status: "enabled", UpdatedAt: "2026-07-04T00:00:00Z", Values: map[string]string{"permissions": "admin:user:read"}},
			},
		},
	}

	committed, err := repository.Save(context.Background(), snapshot)
	if err != nil || committed != 5 {
		t.Fatalf("Save() = %d, %v; want revision 5", committed, err)
	}
	loaded, err := repository.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded.NextID != snapshot.NextID {
		t.Fatalf("NextID = %d, want %d", loaded.NextID, snapshot.NextID)
	}
	if loaded.Revision != committed {
		t.Fatalf("Revision = %d, want %d", loaded.Revision, committed)
	}
	if !reflect.DeepEqual(loaded.Resources, snapshot.Resources) {
		t.Fatalf("Resources = %#v, want %#v", loaded.Resources, snapshot.Resources)
	}
}

func TestSQLBackedStorePersistsRolePermissionsForDynamicMenus(t *testing.T) {
	db := openAdminResourceTestDB(t)
	repository, err := NewSQLAdminResourceRepository(context.Background(), db)
	if err != nil {
		t.Fatalf("NewSQLAdminResourceRepository() error = %v", err)
	}
	store, err := NewRepositoryBackedStoreFromCapabilities(repository, core.DefaultManifests())
	if err != nil {
		t.Fatalf("NewRepositoryBackedStoreFromCapabilities() error = %v", err)
	}
	if _, err := store.Update("roles", "role-operator", WriteInput{
		Name:        "Operator",
		Status:      "enabled",
		Description: "SQL operator",
		Values:      map[string]string{"groupCode": "operations", "dataScope": "current_org", "permissions": "admin:user:read"},
	}); err != nil {
		t.Fatalf("Update(role-operator) error = %v", err)
	}

	reloaded, err := NewRepositoryBackedStoreFromCapabilities(repository, core.DefaultManifests())
	if err != nil {
		t.Fatalf("reload store error = %v", err)
	}
	menus := reloaded.MenuItemsForPrincipal(reloaded.CurrentPrincipal("ops"))
	if !hasMenuRoute(menus, "/users") {
		t.Fatalf("menus after SQL reload = %+v, want /users", menus)
	}
	if hasMenuRoute(menus, "/tenants") {
		t.Fatalf("menus after SQL reload = %+v, want /tenants removed", menus)
	}
}

func openAdminResourceTestDB(t *testing.T) *sql.DB {
	t.Helper()
	name := fmt.Sprintf("admin-resource-test-%s", t.Name())
	db, err := sql.Open("platform_admin_resource_test", name)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		adminResourceTestDriverState.Delete(name)
	})
	return db
}

var adminResourceTestDriverState sync.Map

func init() {
	sql.Register("platform_admin_resource_test", adminResourceTestDriver{})
}

type adminResourceTestDriver struct{}

func (adminResourceTestDriver) Open(name string) (driver.Conn, error) {
	stateValue, _ := adminResourceTestDriverState.LoadOrStore(name, &adminResourceTestState{})
	return &adminResourceTestConn{state: stateValue.(*adminResourceTestState)}, nil
}

type adminResourceTestState struct {
	mu       sync.Mutex
	nextID   string
	revision string
	records  []adminResourceTestRecord
}

type adminResourceTestRecord struct {
	Resource    string
	ID          string
	Code        string
	Name        string
	Status      string
	Description string
	UpdatedAt   string
	ValuesJSON  string
}

type adminResourceTestConn struct {
	state *adminResourceTestState
}

func (c *adminResourceTestConn) Prepare(query string) (driver.Stmt, error) {
	return nil, driver.ErrSkip
}

func (c *adminResourceTestConn) Close() error {
	return nil
}

func (c *adminResourceTestConn) Begin() (driver.Tx, error) {
	return nil, driver.ErrSkip
}

func (c *adminResourceTestConn) Exec(query string, args []driver.Value) (driver.Result, error) {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	switch {
	case strings.Contains(query, "CREATE TABLE"):
		return driver.RowsAffected(0), nil
	case strings.Contains(query, "DELETE FROM platform_admin_resource_records"):
		c.state.records = nil
		return driver.RowsAffected(1), nil
	case strings.Contains(query, "DELETE FROM platform_admin_resource_state"):
		switch fmt.Sprint(args[0]) {
		case "next_id":
			c.state.nextID = ""
		case "revision":
			c.state.revision = ""
		}
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
		if len(args) != 8 {
			return nil, fmt.Errorf("unexpected record args: %v", args)
		}
		c.state.records = append(c.state.records, adminResourceTestRecord{
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

func (c *adminResourceTestConn) Query(query string, args []driver.Value) (driver.Rows, error) {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	if strings.Contains(query, "platform_admin_resource_state") {
		values := [][]driver.Value{}
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
		return &adminResourceTestRows{columns: []string{"value"}, values: values}, nil
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
	return &adminResourceTestRows{
		columns: []string{"resource", "id", "code", "name", "status", "description", "updated_at", "values_json"},
		values:  values,
	}, nil
}

type adminResourceTestRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *adminResourceTestRows) Columns() []string {
	return r.columns
}

func (r *adminResourceTestRows) Close() error {
	return nil
}

func (r *adminResourceTestRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}
