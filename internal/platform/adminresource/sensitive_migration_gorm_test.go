package adminresource

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"platform-go/internal/platform/dataprotection"
	"platform-go/internal/platform/sensitivemigration"

	"gorm.io/gorm"
)

func TestGORMProtectedValueMigrationReadPathsDoNotCreateTables(t *testing.T) {
	db := openAdminResourceGORMDB(t)
	store, err := NewGORMProtectedValueMigrationStore(db, "sqlite")
	if err != nil {
		t.Fatalf("NewGORMProtectedValueMigrationStore() error = %v", err)
	}
	assertMigrationTableNames(t, db, nil)

	if err := db.Migrator().CreateTable(&gormAdminResourceRecord{}); err != nil {
		t.Fatalf("CreateTable(generic resource) error = %v", err)
	}
	if err := db.Create(&gormAdminResourceRecord{
		Resource: "customer-records", ID: "record-1", Code: "record-1", Name: "Record 1",
		Status: "enabled", Description: "", UpdatedAt: "2026-07-12T00:00:00Z",
		ValuesJSON: `{"secretNote":"plain-value"}`,
	}).Error; err != nil {
		t.Fatalf("Create(generic row) error = %v", err)
	}
	before := migrationTableNames(t, db)
	plan := migrationResourcePlan("customer-records", "global", "", "secretNote")
	scopes, err := store.TenantScopes(context.Background(), plan)
	if err != nil {
		t.Fatalf("TenantScopes() error = %v", err)
	}
	if !slices.Equal(scopes, []string{dataprotection.GlobalTenantID}) {
		t.Fatalf("TenantScopes() = %v, want global scope", scopes)
	}
	rows, err := store.Rows(context.Background(), plan, dataprotection.GlobalTenantID, "", 10)
	if err != nil {
		t.Fatalf("Rows() error = %v", err)
	}
	if len(rows) != 1 || rows[0].RecordID != "record-1" || rows[0].ValuesJSON == "" {
		t.Fatalf("Rows() = %+v, want generic row", rows)
	}
	assertMigrationTableNames(t, db, before)
}

func TestGORMProtectedValueMigrationUsesWhitelistedPhysicalLayouts(t *testing.T) {
	db := openAdminResourceGORMDB(t)
	if err := db.AutoMigrate(&gormAdminResourceRecord{}, &gormAdminLoginLog{}); err != nil {
		t.Fatalf("AutoMigrate(test tables) error = %v", err)
	}
	if err := db.Create(&gormAdminResourceRecord{
		Resource: "custom-contacts", ID: "generic-1", Code: "generic-1", Name: "Generic",
		Status: "enabled", Description: "", UpdatedAt: "2026-07-12T00:00:00Z",
		ValuesJSON: `{"phone":"plain-generic"}`,
	}).Error; err != nil {
		t.Fatalf("Create(generic row) error = %v", err)
	}
	if err := db.Create(&gormAdminLoginLog{
		ID: "login-1", Code: "login-1", Name: "Login", Status: "success", Description: "",
		UpdatedAt: "2026-07-12T00:00:00Z", Username: "projected-user", Provider: "demo",
		IP: "127.0.0.1", CreatedAt: "2026-07-12T00:00:00Z",
		ValuesJSON: `{"customSecret":"plain-normalized","phone":"plain-custom"}`,
	}).Error; err != nil {
		t.Fatalf("Create(normalized row) error = %v", err)
	}
	store, err := NewGORMProtectedValueMigrationStore(db, "sqlite")
	if err != nil {
		t.Fatalf("NewGORMProtectedValueMigrationStore() error = %v", err)
	}

	for _, testCase := range []struct {
		name string
		plan sensitivemigration.ResourcePlan
		want string
	}{
		{name: "generic custom field", plan: migrationResourcePlan("custom-contacts", "global", "", "phone"), want: "generic-1"},
		{name: "normalized custom field", plan: migrationResourcePlan("login-logs", "global", "", "customSecret"), want: "login-1"},
		{name: "normalized name is not a sensitivity heuristic", plan: migrationResourcePlan("login-logs", "global", "", "phone"), want: "login-1"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			rows, readErr := store.Rows(context.Background(), testCase.plan, dataprotection.GlobalTenantID, "", 10)
			if readErr != nil {
				t.Fatalf("Rows() error = %v", readErr)
			}
			if len(rows) != 1 || rows[0].RecordID != testCase.want {
				t.Fatalf("Rows() = %+v, want %q", rows, testCase.want)
			}
		})
	}

	for _, testCase := range []struct {
		resource string
		field    string
	}{
		{resource: "tenants", field: "areaCode"},
		{resource: "org-units", field: "tenantCode"},
		{resource: "login-logs", field: "username"},
		{resource: "login-logs", field: "ip"},
		{resource: "users", field: "roles"},
		{resource: "roles", field: "permissions"},
	} {
		plan := migrationResourcePlan(testCase.resource, "global", "", testCase.field)
		if _, err := store.Rows(context.Background(), plan, dataprotection.GlobalTenantID, "", 10); !errors.Is(err, ErrMigrationPhysicalLayout) {
			t.Fatalf("Rows(%s.%s) error = %v, want ErrMigrationPhysicalLayout", testCase.resource, testCase.field, err)
		}
	}
}

func TestGORMProtectedValueMigrationRowsUseBoundedPhysicalKeysetPages(t *testing.T) {
	db := openAdminResourceGORMDB(t)
	if err := db.AutoMigrate(&gormAdminResourceRecord{}); err != nil {
		t.Fatalf("AutoMigrate(test generic table) error = %v", err)
	}
	for index, valuesJSON := range []string{
		`{"tenantCode":"tenant-b","secretNote":"b-1"}`,
		`{"tenantCode":"tenant-a","secretNote":"a-1"}`,
		`{"tenantCode":"tenant-b","secretNote":"b-2"}`,
		`{"tenantCode":"tenant-a","secretNote":"a-2"}`,
		`{"tenantCode":"tenant-b","secretNote":"b-3"}`,
	} {
		id := "record-" + string(rune('1'+index))
		if err := db.Create(&gormAdminResourceRecord{
			Resource: "customer-records", ID: id, Code: id, Name: id, Status: "enabled",
			Description: "", UpdatedAt: "2026-07-12T00:00:00Z", ValuesJSON: valuesJSON,
		}).Error; err != nil {
			t.Fatalf("Create(%s) error = %v", id, err)
		}
	}
	callbackName := "test:sensitive_migration_requires_limit"
	sawPhysicalQuery := false
	sawUnboundedQuery := false
	if err := db.Callback().Row().Before("gorm:row").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == adminResourceRecordsTable {
			sawPhysicalQuery = true
			if _, ok := tx.Statement.Clauses["LIMIT"]; !ok {
				sawUnboundedQuery = true
			}
		}
	}); err != nil {
		t.Fatalf("register query callback error = %v", err)
	}
	t.Cleanup(func() { _ = db.Callback().Row().Remove(callbackName) })
	store, err := NewGORMProtectedValueMigrationStore(db, "sqlite")
	if err != nil {
		t.Fatalf("NewGORMProtectedValueMigrationStore() error = %v", err)
	}

	rows, err := store.Rows(context.Background(), migrationResourcePlan("customer-records", "tenant-field", "tenantCode", "secretNote"), "tenant-a", "", 2)
	if err != nil {
		t.Fatalf("Rows() error = %v", err)
	}
	if len(rows) != 2 || rows[0].RecordID != "record-2" || rows[1].RecordID != "record-4" {
		t.Fatalf("Rows() = %+v, want tenant-a keyset page", rows)
	}
	if !sawPhysicalQuery || sawUnboundedQuery {
		t.Fatalf("physical queries bounded = %v, unbounded = %v; want bounded SQL pages", sawPhysicalQuery, sawUnboundedQuery)
	}
}

func TestGORMProtectedValueMigrationPrepareCreatesOnlyValueFreeJournalTables(t *testing.T) {
	db := openAdminResourceGORMDB(t)
	if err := db.AutoMigrate(&gormAdminResourceRecord{}, &gormAdminResourceState{}); err != nil {
		t.Fatalf("AutoMigrate(test ordinary tables) error = %v", err)
	}
	if err := db.Create(&gormAdminResourceState{Key: "revision", Value: "7"}).Error; err != nil {
		t.Fatalf("Create(revision) error = %v", err)
	}
	const protectedFixture = "historical-plaintext-marker"
	if err := db.Create(&gormAdminResourceRecord{
		Resource: "customer-records", ID: "record-1", Code: "record-1", Name: "Record 1",
		Status: "enabled", Description: "", UpdatedAt: "2026-07-12T00:00:00Z",
		ValuesJSON: `{"secretNote":"` + protectedFixture + `"}`,
	}).Error; err != nil {
		t.Fatalf("Create(resource row) error = %v", err)
	}
	store, err := NewGORMProtectedValueMigrationStore(db, "sqlite")
	if err != nil {
		t.Fatalf("NewGORMProtectedValueMigrationStore() error = %v", err)
	}
	before := migrationTableNames(t, db)
	state, err := store.Prepare(context.Background(), sensitivemigration.RunRequest{
		RunID: "run-prepare", PlanHash: "sha256:prepare-plan",
		Plan: sensitivemigration.Plan{Resources: []sensitivemigration.ResourcePlan{
			migrationResourcePlan("customer-records", "global", "", "secretNote"),
		}},
	})
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if state.RunID != "run-prepare" || state.PlanHash != "sha256:prepare-plan" || state.ExpectedRevision != 7 || state.TargetCount != 1 {
		t.Fatalf("Prepare() state = %+v", state)
	}

	want := append(append([]string(nil), before...), sensitiveMigrationTableNames()...)
	slices.Sort(want)
	assertMigrationTableNames(t, db, want)
	if db.Migrator().HasTable(&gormAdminUser{}) {
		t.Fatal("Prepare() created ordinary normalized table")
	}

	var run gormSensitiveMigrationRun
	if err := db.First(&run, "run_id = ?", "run-prepare").Error; err != nil {
		t.Fatalf("read prepared run error = %v", err)
	}
	if run.PlanHash != "sha256:prepare-plan" {
		t.Fatalf("prepared plan hash = %q", run.PlanHash)
	}
	var targets []gormSensitiveMigrationTarget
	if err := db.Order("record_id, field_key").Find(&targets).Error; err != nil {
		t.Fatalf("read targets error = %v", err)
	}
	if len(targets) != 1 || targets[0].RecordID != "record-1" || targets[0].FieldKey != "secretNote" {
		t.Fatalf("prepared targets = %+v", targets)
	}
	serialized := strings.Join([]string{run.RunID, run.PlanHash, targets[0].RunID, targets[0].Resource, targets[0].TenantScope, targets[0].TenantScopeHash, targets[0].RecordID, targets[0].FieldKey, targets[0].SnapshotHash}, "|")
	if strings.Contains(serialized, protectedFixture) {
		t.Fatal("Prepare() persisted a protected business value")
	}
	for _, column := range migrationTableColumns(t, db, sensitiveMigrationTargetsTable) {
		switch column {
		case "values_json", "value", "protected_value", "ciphertext":
			t.Fatalf("target table contains protected-value column %q", column)
		}
	}
}

func TestGORMProtectedValueMigrationApplyBatchUsesSnapshotAndRevisionCAS(t *testing.T) {
	db, store := preparedMigrationStore(t, map[string]string{
		"record-1": `{"secretNote":"plain-one"}`,
	})
	mutation := sensitivemigration.BatchMutation{
		RunID: "run-apply", Mode: sensitivemigration.ModeApply,
		Resource: migrationResourcePlan("customer-records", "global", "", "secretNote"),
		TenantID: dataprotection.GlobalTenantID, ExpectedRevision: 7, LastRecordID: "record-1",
		Rows: []sensitivemigration.RowMutation{{
			RecordID: "record-1", OriginalValuesJSON: `{"secretNote":"plain-one"}`,
			UpdatedValuesJSON: `{"secretNote":"pgo:enc:v1:migrated"}`,
		}},
	}
	commit, err := store.ApplyBatch(context.Background(), mutation)
	if err != nil {
		t.Fatalf("ApplyBatch() error = %v", err)
	}
	if commit.Revision != 8 || commit.Rows != 1 || commit.LastRecordID != "record-1" {
		t.Fatalf("ApplyBatch() commit = %+v", commit)
	}
	assertMigrationRecordJSON(t, db, "record-1", `{"secretNote":"pgo:enc:v1:migrated"}`)
	if revision, err := loadGORMRevision(db); err != nil || revision != 8 {
		t.Fatalf("revision = %d, %v; want 8", revision, err)
	}
	assertGORMCount(t, db, &gormSensitiveMigrationCheckpoint{}, 1)
	assertGORMCount(t, db, &gormSensitiveMigrationEvent{}, 1)
	assertGORMCount(t, db, &gormSensitiveMigrationEscrow{}, 0)
}

func TestGORMProtectedValueMigrationApplyBatchRollsBackJournalOnRowConflict(t *testing.T) {
	db, store := preparedMigrationStore(t, map[string]string{
		"record-1": `{"secretNote":"plain-one"}`,
		"record-2": `{"secretNote":"plain-two"}`,
	})
	_, err := store.ApplyBatch(context.Background(), sensitivemigration.BatchMutation{
		RunID: "run-apply", Mode: sensitivemigration.ModeApply,
		Resource: migrationResourcePlan("customer-records", "global", "", "secretNote"),
		TenantID: dataprotection.GlobalTenantID, ExpectedRevision: 7, LastRecordID: "record-2",
		Rows: []sensitivemigration.RowMutation{
			{RecordID: "record-1", OriginalValuesJSON: `{"secretNote":"plain-one"}`, UpdatedValuesJSON: `{"secretNote":"pgo:enc:v1:one"}`},
			{RecordID: "record-2", OriginalValuesJSON: `{"secretNote":"stale"}`, UpdatedValuesJSON: `{"secretNote":"pgo:enc:v1:two"}`},
		},
	})
	if !errors.Is(err, ErrMigrationConflict) {
		t.Fatalf("ApplyBatch() error = %v, want ErrMigrationConflict", err)
	}
	assertMigrationRecordJSON(t, db, "record-1", `{"secretNote":"plain-one"}`)
	assertMigrationRecordJSON(t, db, "record-2", `{"secretNote":"plain-two"}`)
	if revision, revisionErr := loadGORMRevision(db); revisionErr != nil || revision != 7 {
		t.Fatalf("revision = %d, %v; want 7", revision, revisionErr)
	}
	assertGORMCount(t, db, &gormSensitiveMigrationCheckpoint{}, 0)
	assertGORMCount(t, db, &gormSensitiveMigrationEvent{}, 0)
	assertGORMCount(t, db, &gormSensitiveMigrationEscrow{}, 0)
}

func TestGORMProtectedValueMigrationApplyBatchRejectsNonTargetJSONChanges(t *testing.T) {
	db, store := preparedMigrationStore(t, map[string]string{
		"record-1": `{"displayName":"Original","secretNote":"plain-one"}`,
	})
	_, err := store.ApplyBatch(context.Background(), sensitivemigration.BatchMutation{
		RunID: "run-apply", Mode: sensitivemigration.ModeApply,
		Resource: migrationResourcePlan("customer-records", "global", "", "secretNote"),
		TenantID: dataprotection.GlobalTenantID, ExpectedRevision: 7, LastRecordID: "record-1",
		Rows: []sensitivemigration.RowMutation{{
			RecordID: "record-1", OriginalValuesJSON: `{"displayName":"Original","secretNote":"plain-one"}`,
			UpdatedValuesJSON: `{"displayName":"Changed","secretNote":"pgo:enc:v1:one"}`,
		}},
	})
	if !errors.Is(err, ErrMigrationConflict) {
		t.Fatalf("ApplyBatch() error = %v, want ErrMigrationConflict", err)
	}
	assertMigrationRecordJSON(t, db, "record-1", `{"displayName":"Original","secretNote":"plain-one"}`)
	if revision, revisionErr := loadGORMRevision(db); revisionErr != nil || revision != 7 {
		t.Fatalf("revision = %d, %v; want 7", revision, revisionErr)
	}
	assertGORMCount(t, db, &gormSensitiveMigrationCheckpoint{}, 0)
	assertGORMCount(t, db, &gormSensitiveMigrationEvent{}, 0)
}

func migrationResourcePlan(resource string, scope string, tenantField string, fields ...string) sensitivemigration.ResourcePlan {
	plan := sensitivemigration.ResourcePlan{Resource: resource, Scope: scope, TenantField: tenantField, SchemaVersion: 1}
	for _, field := range fields {
		plan.Fields = append(plan.Fields, sensitivemigration.FieldPlan{Key: field})
	}
	return plan
}

func preparedMigrationStore(t *testing.T, rows map[string]string) (*gorm.DB, *GORMProtectedValueMigrationStore) {
	t.Helper()
	db := openAdminResourceGORMDB(t)
	if err := db.AutoMigrate(&gormAdminResourceRecord{}, &gormAdminResourceState{}); err != nil {
		t.Fatalf("AutoMigrate(test ordinary tables) error = %v", err)
	}
	if err := db.Create(&gormAdminResourceState{Key: "revision", Value: "7"}).Error; err != nil {
		t.Fatalf("Create(revision) error = %v", err)
	}
	for id, valuesJSON := range rows {
		if err := db.Create(&gormAdminResourceRecord{
			Resource: "customer-records", ID: id, Code: id, Name: id, Status: "enabled",
			Description: "", UpdatedAt: "2026-07-12T00:00:00Z", ValuesJSON: valuesJSON,
		}).Error; err != nil {
			t.Fatalf("Create(%s) error = %v", id, err)
		}
	}
	store, err := NewGORMProtectedValueMigrationStore(db, "sqlite")
	if err != nil {
		t.Fatalf("NewGORMProtectedValueMigrationStore() error = %v", err)
	}
	if _, err := store.Prepare(context.Background(), sensitivemigration.RunRequest{
		RunID: "run-apply", PlanHash: "sha256:apply-plan",
		Plan: sensitivemigration.Plan{Resources: []sensitivemigration.ResourcePlan{
			migrationResourcePlan("customer-records", "global", "", "secretNote"),
		}},
	}); err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	return db, store
}

func migrationTableNames(t *testing.T, db *gorm.DB) []string {
	t.Helper()
	tables, err := db.Migrator().GetTables()
	if err != nil {
		t.Fatalf("GetTables() error = %v", err)
	}
	slices.Sort(tables)
	return tables
}

func assertMigrationTableNames(t *testing.T, db *gorm.DB, want []string) {
	t.Helper()
	got := migrationTableNames(t, db)
	want = append([]string(nil), want...)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("database tables = %v, want %v", got, want)
	}
}

func migrationTableColumns(t *testing.T, db *gorm.DB, table string) []string {
	t.Helper()
	type columnRow struct {
		Name string `gorm:"column:name"`
	}
	var rows []columnRow
	if err := db.Raw("PRAGMA table_info(" + table + ")").Scan(&rows).Error; err != nil {
		t.Fatalf("table_info(%s) error = %v", table, err)
	}
	columns := make([]string, 0, len(rows))
	for _, row := range rows {
		columns = append(columns, row.Name)
	}
	return columns
}

func assertMigrationRecordJSON(t *testing.T, db *gorm.DB, id string, want string) {
	t.Helper()
	var row gormAdminResourceRecord
	if err := db.Where("resource = ? AND id = ?", "customer-records", id).First(&row).Error; err != nil {
		t.Fatalf("read resource row %s error = %v", id, err)
	}
	if row.ValuesJSON != want {
		t.Fatalf("resource row %s values_json = %q, want %q", id, row.ValuesJSON, want)
	}
}
