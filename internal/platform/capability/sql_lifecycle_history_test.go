package capability

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"reflect"
	"sync"
	"testing"
)

func TestSQLLifecycleHistoryRecordsMigrationsAndSeeds(t *testing.T) {
	db := openLifecycleTestDB(t)
	history, err := NewSQLLifecycleHistory(context.Background(), db)
	if err != nil {
		t.Fatalf("NewSQLLifecycleHistory() error = %v", err)
	}

	migration := LifecycleRecord{CapabilityID: "tenant", StepID: "core-tenant-0001", Description: "tenant migration"}
	seed := LifecycleRecord{CapabilityID: "tenant", StepID: "core-tenant-seed-0001", Description: "tenant seed"}
	if history.HasMigration(context.Background(), migration.CapabilityID, migration.StepID) {
		t.Fatalf("HasMigration() = true before record")
	}
	if err := history.RecordMigration(context.Background(), migration); err != nil {
		t.Fatalf("RecordMigration() error = %v", err)
	}
	if err := history.RecordSeed(context.Background(), seed); err != nil {
		t.Fatalf("RecordSeed() error = %v", err)
	}
	if !history.HasMigration(context.Background(), migration.CapabilityID, migration.StepID) {
		t.Fatalf("HasMigration() = false after record")
	}
	if !history.HasSeed(context.Background(), seed.CapabilityID, seed.StepID) {
		t.Fatalf("HasSeed() = false after record")
	}

	want := []LifecycleRecord{
		{Kind: LifecycleKindMigration, CapabilityID: "tenant", StepID: "core-tenant-0001", Description: "tenant migration"},
		{Kind: LifecycleKindSeed, CapabilityID: "tenant", StepID: "core-tenant-seed-0001", Description: "tenant seed"},
	}
	if !reflect.DeepEqual(history.Records(context.Background()), want) {
		t.Fatalf("Records() = %#v, want %#v", history.Records(context.Background()), want)
	}
}

func TestSQLLifecycleHistoryDoesNotDuplicateRecords(t *testing.T) {
	db := openLifecycleTestDB(t)
	history, err := NewSQLLifecycleHistory(context.Background(), db)
	if err != nil {
		t.Fatalf("NewSQLLifecycleHistory() error = %v", err)
	}
	record := LifecycleRecord{CapabilityID: "audit", StepID: "core-audit-0001", Description: "audit migration"}

	if err := history.RecordMigration(context.Background(), record); err != nil {
		t.Fatalf("RecordMigration(first) error = %v", err)
	}
	if err := history.RecordMigration(context.Background(), record); err != nil {
		t.Fatalf("RecordMigration(second) error = %v", err)
	}
	if got := len(history.Records(context.Background())); got != 1 {
		t.Fatalf("record count = %d, want 1", got)
	}
}

func TestRecordedLifecycleExecutorSkipsStepsWithSQLHistory(t *testing.T) {
	db := openLifecycleTestDB(t)
	history, err := NewSQLLifecycleHistory(context.Background(), db)
	if err != nil {
		t.Fatalf("NewSQLLifecycleHistory() error = %v", err)
	}
	executor := NewRecordedLifecycleExecutor(history)
	runtime := Runtime{MigrationExecutor: executor, SeedExecutor: executor}
	var calls []string

	for i := 0; i < 2; i++ {
		if err := runtime.RunMigration(context.Background(), MigrationExecution{
			CapabilityID: "menu",
			Migration:    Migration{ID: "core-menu-0001", Description: "menu migration", Up: appendCall(&calls, "migration")},
		}); err != nil {
			t.Fatalf("RunMigration(%d) error = %v", i, err)
		}
		if err := runtime.RunSeed(context.Background(), SeedExecution{
			CapabilityID: "menu",
			Seed:         Seed{ID: "core-menu-seed-0001", Description: "menu seed", Run: appendCall(&calls, "seed")},
		}); err != nil {
			t.Fatalf("RunSeed(%d) error = %v", i, err)
		}
	}

	if !reflect.DeepEqual(calls, []string{"migration", "seed"}) {
		t.Fatalf("calls = %#v, want each SQL-backed step once", calls)
	}
	if got := len(history.Records(context.Background())); got != 2 {
		t.Fatalf("record count = %d, want 2", got)
	}
}

func openLifecycleTestDB(t *testing.T) *sql.DB {
	t.Helper()
	name := fmt.Sprintf("lifecycle-test-%s", t.Name())
	db, err := sql.Open("platform_lifecycle_test", name)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		lifecycleTestDriverState.Delete(name)
	})
	return db
}

var lifecycleTestDriverState sync.Map

func init() {
	sql.Register("platform_lifecycle_test", lifecycleTestDriver{})
}

type lifecycleTestDriver struct{}

func (lifecycleTestDriver) Open(name string) (driver.Conn, error) {
	stateValue, _ := lifecycleTestDriverState.LoadOrStore(name, &lifecycleTestState{})
	return &lifecycleTestConn{state: stateValue.(*lifecycleTestState)}, nil
}

type lifecycleTestState struct {
	mu      sync.Mutex
	records []LifecycleRecord
}

type lifecycleTestConn struct {
	state *lifecycleTestState
}

func (c *lifecycleTestConn) Prepare(query string) (driver.Stmt, error) {
	return nil, driver.ErrSkip
}

func (c *lifecycleTestConn) Close() error {
	return nil
}

func (c *lifecycleTestConn) Begin() (driver.Tx, error) {
	return nil, driver.ErrSkip
}

func (c *lifecycleTestConn) Exec(query string, args []driver.Value) (driver.Result, error) {
	if len(args) == 0 {
		return driver.RowsAffected(0), nil
	}
	if len(args) != 4 {
		return nil, fmt.Errorf("unexpected exec args: %v", args)
	}
	record := LifecycleRecord{
		Kind:         LifecycleKind(fmt.Sprint(args[0])),
		CapabilityID: ID(fmt.Sprint(args[1])),
		StepID:       fmt.Sprint(args[2]),
		Description:  fmt.Sprint(args[3]),
	}
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	if hasLifecycleRecord(c.state.records, record.Kind, record.CapabilityID, record.StepID) {
		return driver.RowsAffected(0), nil
	}
	c.state.records = append(c.state.records, record)
	return driver.RowsAffected(1), nil
}

func (c *lifecycleTestConn) Query(query string, args []driver.Value) (driver.Rows, error) {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	if len(args) == 3 {
		kind := LifecycleKind(fmt.Sprint(args[0]))
		capabilityID := ID(fmt.Sprint(args[1]))
		stepID := fmt.Sprint(args[2])
		if hasLifecycleRecord(c.state.records, kind, capabilityID, stepID) {
			return &lifecycleTestRows{columns: []string{"exists"}, values: [][]driver.Value{{int64(1)}}}, nil
		}
		return &lifecycleTestRows{columns: []string{"exists"}}, nil
	}
	values := make([][]driver.Value, 0, len(c.state.records))
	for _, record := range c.state.records {
		values = append(values, []driver.Value{string(record.Kind), string(record.CapabilityID), record.StepID, record.Description})
	}
	return &lifecycleTestRows{columns: []string{"kind", "capability_id", "step_id", "description"}, values: values}, nil
}

type lifecycleTestRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *lifecycleTestRows) Columns() []string {
	return r.columns
}

func (r *lifecycleTestRows) Close() error {
	return nil
}

func (r *lifecycleTestRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}
