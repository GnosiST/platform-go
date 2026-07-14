package adminresource

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
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
		Revision: 0,
		NextID:   1042,
		Resources: map[string][]Record{
			"tenants": {
				{
					ID: "tenant-1042", Code: "acme", Name: "Acme Tenant", Status: "enabled", Description: "SQL tenant", UpdatedAt: "2026-07-04T00:00:00Z", Values: map[string]string{"isolation": "sandbox"},
					DeletedAt: "2026-07-14T00:00:00Z", DeletedBy: "user-admin", DeleteReason: "retired", PurgeAfter: "2026-08-13T00:00:00Z", DeletionPolicyVersion: 1,
				},
			},
			"roles": {
				{ID: "role-operator", Code: "operator", Name: "Operator", Status: "enabled", UpdatedAt: "2026-07-04T00:00:00Z", Values: map[string]string{"permissions": "admin:user:read"}},
			},
		},
	}

	committed, err := repository.Save(context.Background(), snapshot)
	if err != nil || committed != 1 {
		t.Fatalf("Save() = %d, %v; want revision 1", committed, err)
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

func TestSQLAdminResourceRepositoryRejectsStaleRevisionAndRollsBackFailures(t *testing.T) {
	db := openAdminResourceTestDB(t)
	repository, err := NewSQLAdminResourceRepository(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	baseline := ResourceSnapshot{Resources: map[string][]Record{"tenants": {{ID: "tenant-1", Code: "one", Name: "One"}}}}
	committed, err := repository.Save(context.Background(), baseline)
	if err != nil || committed != 1 {
		t.Fatalf("baseline Save() = %d, %v", committed, err)
	}
	stale := ResourceSnapshot{Revision: 0, Resources: map[string][]Record{"tenants": {{ID: "tenant-2", Code: "two", Name: "Two"}}}}
	if _, err := repository.Save(context.Background(), stale); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("stale Save() error = %v, want ErrRevisionConflict", err)
	}

	state := adminResourceTestStateForDB(t)
	state.mu.Lock()
	state.failQuery = "INSERT INTO " + adminResourceLifecycleTable
	state.mu.Unlock()
	failing := ResourceSnapshot{Revision: 1, Resources: map[string][]Record{"tenants": {{
		ID: "tenant-1", Code: "one", Name: "Changed", DeletedAt: "2026-07-14T00:00:00Z", DeletedBy: "admin", DeleteReason: "retired", PurgeAfter: "2026-08-13T00:00:00Z", DeletionPolicyVersion: 1,
	}}}}
	if _, err := repository.Save(context.Background(), failing); err == nil {
		t.Fatal("Save() with injected lifecycle failure succeeded")
	}
	loaded, err := repository.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Revision != 1 || len(loaded.Resources["tenants"]) != 1 || loaded.Resources["tenants"][0].Name != "One" || loaded.Resources["tenants"][0].DeletedAt != "" {
		t.Fatalf("rollback snapshot = %+v", loaded)
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
	mu         sync.Mutex
	nextID     string
	revision   string
	records    []adminResourceTestRecord
	lifecycles []adminResourceTestLifecycle
	failQuery  string
}

type adminResourceTestLifecycle struct {
	Resource              string
	RecordID              string
	DeletedAt             string
	DeletedBy             string
	DeleteReason          string
	PurgeAfter            string
	DeletionPolicyVersion int64
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
	tx    *adminResourceTestTx
}

type adminResourceTestData struct {
	nextID     string
	revision   string
	records    []adminResourceTestRecord
	lifecycles []adminResourceTestLifecycle
}

type adminResourceTestTx struct {
	conn     *adminResourceTestConn
	previous adminResourceTestData
	done     bool
}

func (c *adminResourceTestConn) Prepare(query string) (driver.Stmt, error) {
	return nil, driver.ErrSkip
}

func (c *adminResourceTestConn) Close() error {
	return nil
}

func (c *adminResourceTestConn) Begin() (driver.Tx, error) {
	if c.tx != nil {
		return nil, errors.New("transaction already active")
	}
	c.state.mu.Lock()
	c.tx = &adminResourceTestTx{conn: c, previous: c.state.dataLocked()}
	return c.tx, nil
}

func (c *adminResourceTestConn) Exec(query string, args []driver.Value) (driver.Result, error) {
	if c.tx == nil {
		c.state.mu.Lock()
		defer c.state.mu.Unlock()
	}
	if c.state.failQuery != "" && strings.Contains(query, c.state.failQuery) {
		c.state.failQuery = ""
		return nil, errors.New("injected SQL failure")
	}
	switch {
	case strings.Contains(query, "CREATE TABLE"):
		return driver.RowsAffected(0), nil
	case strings.Contains(query, "DELETE FROM platform_admin_resource_records"):
		c.state.records = nil
		return driver.RowsAffected(1), nil
	case strings.Contains(query, "DELETE FROM platform_admin_resource_lifecycle"):
		c.state.lifecycles = nil
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
	case strings.Contains(query, "UPDATE platform_admin_resource_state"):
		if len(args) != 3 || fmt.Sprint(args[1]) != "revision" || c.state.revision != fmt.Sprint(args[2]) {
			return driver.RowsAffected(0), nil
		}
		c.state.revision = fmt.Sprint(args[0])
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
	case strings.Contains(query, "INSERT INTO platform_admin_resource_lifecycle"):
		if len(args) != 7 {
			return nil, fmt.Errorf("unexpected lifecycle args: %v", args)
		}
		version, _ := args[6].(int64)
		c.state.lifecycles = append(c.state.lifecycles, adminResourceTestLifecycle{
			Resource: fmt.Sprint(args[0]), RecordID: fmt.Sprint(args[1]), DeletedAt: fmt.Sprint(args[2]), DeletedBy: fmt.Sprint(args[3]),
			DeleteReason: fmt.Sprint(args[4]), PurgeAfter: fmt.Sprint(args[5]), DeletionPolicyVersion: version,
		})
		return driver.RowsAffected(1), nil
	default:
		return driver.RowsAffected(0), nil
	}
}

func (c *adminResourceTestConn) Query(query string, args []driver.Value) (driver.Rows, error) {
	if c.tx == nil {
		c.state.mu.Lock()
		defer c.state.mu.Unlock()
	}
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
	if strings.Contains(query, "platform_admin_resource_lifecycle") {
		values := make([][]driver.Value, 0, len(c.state.lifecycles))
		for _, lifecycle := range c.state.lifecycles {
			values = append(values, []driver.Value{
				lifecycle.Resource, lifecycle.RecordID, lifecycle.DeletedAt, lifecycle.DeletedBy, lifecycle.DeleteReason, lifecycle.PurgeAfter, lifecycle.DeletionPolicyVersion,
			})
		}
		return &adminResourceTestRows{
			columns: []string{"resource", "record_id", "deleted_at", "deleted_by", "delete_reason", "purge_after", "deletion_policy_version"}, values: values,
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
	return &adminResourceTestRows{
		columns: []string{"resource", "id", "code", "name", "status", "description", "updated_at", "values_json"},
		values:  values,
	}, nil
}

func (tx *adminResourceTestTx) Commit() error {
	if tx.done {
		return errors.New("transaction already closed")
	}
	tx.done = true
	tx.conn.tx = nil
	tx.conn.state.mu.Unlock()
	return nil
}

func (tx *adminResourceTestTx) Rollback() error {
	if tx.done {
		return driver.ErrBadConn
	}
	tx.conn.state.restoreDataLocked(tx.previous)
	tx.done = true
	tx.conn.tx = nil
	tx.conn.state.mu.Unlock()
	return nil
}

func (state *adminResourceTestState) dataLocked() adminResourceTestData {
	return adminResourceTestData{
		nextID: state.nextID, revision: state.revision,
		records: append([]adminResourceTestRecord(nil), state.records...), lifecycles: append([]adminResourceTestLifecycle(nil), state.lifecycles...),
	}
}

func (state *adminResourceTestState) restoreDataLocked(data adminResourceTestData) {
	state.nextID = data.nextID
	state.revision = data.revision
	state.records = append([]adminResourceTestRecord(nil), data.records...)
	state.lifecycles = append([]adminResourceTestLifecycle(nil), data.lifecycles...)
}

func adminResourceTestStateForDB(t *testing.T) *adminResourceTestState {
	t.Helper()
	name := fmt.Sprintf("admin-resource-test-%s", t.Name())
	value, ok := adminResourceTestDriverState.Load(name)
	if !ok {
		t.Fatalf("missing test driver state %q", name)
	}
	return value.(*adminResourceTestState)
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
