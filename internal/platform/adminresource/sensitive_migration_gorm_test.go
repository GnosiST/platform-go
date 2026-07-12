package adminresource

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"platform-go/internal/platform/dataprotection"
	"platform-go/internal/platform/sensitivemigration"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestGORMProtectedValueMigrationJournalSchemaUsesMySQLSafeSurrogateKeys(t *testing.T) {
	sqliteDB := openAdminResourceGORMDB(t)
	sqlDB, err := sqliteDB.DB()
	if err != nil {
		t.Fatalf("sqlite DB() error = %v", err)
	}
	mysqlDB, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{DryRun: true})
	if err != nil {
		t.Fatalf("open MySQL dry-run DB error = %v", err)
	}

	for _, model := range []any{
		&gormSensitiveMigrationRun{}, &gormSensitiveMigrationTarget{}, &gormSensitiveMigrationCheckpoint{},
		&gormSensitiveMigrationEvent{}, &gormSensitiveMigrationEscrow{},
	} {
		statement := &gorm.Statement{DB: mysqlDB}
		if err := statement.Parse(model); err != nil {
			t.Fatalf("parse %T schema error = %v", model, err)
		}
		if len(statement.Schema.PrimaryFields) != 1 {
			t.Fatalf("%T primary fields = %d, want one surrogate key", model, len(statement.Schema.PrimaryFields))
		}
		primary := statement.Schema.PrimaryFields[0]
		if primary.DataType != "string" || primary.Size != 64 {
			t.Fatalf("%T primary field = %s size %d, want string(64)", model, primary.DBName, primary.Size)
		}
		if fullType := mysqlDB.Migrator().FullDataTypeOf(primary).SQL; !strings.Contains(strings.ToLower(fullType), "varchar(64)") {
			t.Fatalf("%T MySQL primary type = %q, want varchar(64)", model, fullType)
		}
		for _, field := range statement.Schema.Fields {
			if field.DataType != "string" || field.DBName == "protected_original" {
				continue
			}
			if field.Size < 1 || field.Size > 191 {
				t.Fatalf("%T identifier column %s size = %d, want explicit bounded size", model, field.DBName, field.Size)
			}
		}
		for _, index := range statement.Schema.ParseIndexes() {
			bytes := 0
			for _, option := range index.Fields {
				if option.Field.DataType == "string" {
					bytes += option.Field.Size * 4
				}
			}
			if bytes > 3072 {
				t.Fatalf("%T MySQL index %s uses %d bytes, want <= 3072", model, index.Name, bytes)
			}
		}
	}
}

func TestGORMProtectedValueMigrationSurrogateIDsUseUnambiguousCoordinates(t *testing.T) {
	left := migrationSurrogateID("target", "a\x00b", "c")
	right := migrationSurrogateID("target", "a", "b\x00c")
	if left == right {
		t.Fatal("surrogate IDs collide for distinct coordinate sequences")
	}
	if len(left) != 64 || left != migrationSurrogateID("target", "a\x00b", "c") {
		t.Fatalf("surrogate ID = %q, want deterministic 64-character SHA-256 hex", left)
	}
}

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

func TestGORMProtectedValueMigrationPrepareReturnsExistingRunBeforeRescanning(t *testing.T) {
	db, store := preparedMigrationStore(t, map[string]string{"record-1": `{"secretNote":"plain-one"}`})
	if err := db.Migrator().DropTable(&gormAdminResourceRecord{}); err != nil {
		t.Fatalf("DropTable(resource records) error = %v", err)
	}
	state, err := store.Prepare(context.Background(), migrationRunRequest("run-apply"))
	if err != nil {
		t.Fatalf("Prepare(existing same-plan run) error = %v", err)
	}
	if state.RunID != "run-apply" || state.PlanHash != "sha256:apply-plan" || state.ExpectedRevision != 7 {
		t.Fatalf("Prepare(existing) state = %+v", state)
	}
}

func TestGORMProtectedValueMigrationPrepareRejectsRevisionDriftBeforeSealing(t *testing.T) {
	db := migrationOrdinaryDB(t, map[string]string{"record-1": `{"secretNote":"plain-one"}`})
	changed := false
	callbackName := "test:sensitive_migration_prepare_revision_drift"
	if err := db.Callback().Query().After("gorm:after_query").Register(callbackName, func(tx *gorm.DB) {
		if changed || !gormQueryReadsRevision(tx) {
			return
		}
		changed = true
		if result := db.Model(&gormAdminResourceState{}).Where("key = ? AND value = ?", "revision", "7").Update("value", "8"); result.Error != nil || result.RowsAffected != 1 {
			t.Errorf("inject revision drift = %d, %v", result.RowsAffected, result.Error)
		}
	}); err != nil {
		t.Fatalf("register revision callback error = %v", err)
	}
	t.Cleanup(func() { _ = db.Callback().Query().Remove(callbackName) })
	store, err := NewGORMProtectedValueMigrationStore(db, "sqlite")
	if err != nil {
		t.Fatalf("NewGORMProtectedValueMigrationStore() error = %v", err)
	}
	if _, err := store.Prepare(context.Background(), migrationRunRequest("run-drift")); !errors.Is(err, ErrMigrationConflict) {
		t.Fatalf("Prepare(revision drift) error = %v, want ErrMigrationConflict", err)
	}
	assertGORMCount(t, db, &gormSensitiveMigrationRun{}, 0)
	assertGORMCount(t, db, &gormSensitiveMigrationTarget{}, 0)
}

func TestGORMProtectedValueMigrationPrepareReloadsConcurrentIdenticalWinner(t *testing.T) {
	db := migrationOrdinaryDB(t, map[string]string{"record-1": `{"secretNote":"plain-one"}`})
	injected := false
	callbackName := "test:sensitive_migration_prepare_winner"
	if err := db.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if injected || tx.Statement == nil || tx.Statement.Table != sensitiveMigrationRunsTable {
			return
		}
		injected = true
		result := tx.Exec("INSERT INTO "+sensitiveMigrationRunsTable+" (run_id, plan_hash, status, expected_revision, target_count, created_at) VALUES (?, ?, ?, ?, ?, ?)",
			"run-race", "sha256:apply-plan", sensitivemigration.StatusPrepared, 7, 1, "2026-07-12T00:00:00Z")
		if result.Error != nil {
			t.Errorf("inject prepare winner error = %v", result.Error)
		}
	}); err != nil {
		t.Fatalf("register create callback error = %v", err)
	}
	t.Cleanup(func() { _ = db.Callback().Create().Remove(callbackName) })
	store, err := NewGORMProtectedValueMigrationStore(db, "sqlite")
	if err != nil {
		t.Fatalf("NewGORMProtectedValueMigrationStore() error = %v", err)
	}
	state, err := store.Prepare(context.Background(), migrationRunRequest("run-race"))
	if err != nil {
		t.Fatalf("Prepare(concurrent identical winner) error = %v", err)
	}
	if state.RunID != "run-race" || state.PlanHash != "sha256:apply-plan" || state.ExpectedRevision != 7 {
		t.Fatalf("Prepare(concurrent winner) state = %+v", state)
	}
}

func TestGORMProtectedValueMigrationPrepareRejectsConcurrentConflictingWinner(t *testing.T) {
	db := migrationOrdinaryDB(t, map[string]string{"record-1": `{"secretNote":"plain-one"}`})
	injected := false
	callbackName := "test:sensitive_migration_prepare_conflicting_winner"
	if err := db.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if injected || tx.Statement == nil || tx.Statement.Table != sensitiveMigrationRunsTable {
			return
		}
		injected = true
		result := tx.Exec("INSERT INTO "+sensitiveMigrationRunsTable+" (run_id, plan_hash, status, expected_revision, target_count, created_at) VALUES (?, ?, ?, ?, ?, ?)",
			"run-race-conflict", "sha256:other-plan", sensitivemigration.StatusPrepared, 7, 1, "2026-07-12T00:00:00Z")
		if result.Error != nil {
			t.Errorf("inject conflicting prepare winner error = %v", result.Error)
		}
	}); err != nil {
		t.Fatalf("register create callback error = %v", err)
	}
	t.Cleanup(func() { _ = db.Callback().Create().Remove(callbackName) })
	store, err := NewGORMProtectedValueMigrationStore(db, "sqlite")
	if err != nil {
		t.Fatalf("NewGORMProtectedValueMigrationStore() error = %v", err)
	}
	if _, err := store.Prepare(context.Background(), migrationRunRequest("run-race-conflict")); !errors.Is(err, ErrMigrationConflict) {
		t.Fatalf("Prepare(concurrent conflicting winner) error = %v, want ErrMigrationConflict", err)
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
	var run gormSensitiveMigrationRun
	if err := db.Where("run_id = ?", "run-apply").First(&run).Error; err != nil {
		t.Fatalf("read run error = %v", err)
	}
	if run.ExpectedRevision != 8 {
		t.Fatalf("run expected revision = %d, want 8", run.ExpectedRevision)
	}
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

func TestGORMProtectedValueMigrationApplyBatchRequiresSealedRunRevision(t *testing.T) {
	db, store := preparedMigrationStore(t, map[string]string{"record-1": `{"secretNote":"plain-one"}`})
	if result := db.Model(&gormAdminResourceState{}).Where("key = ?", "revision").Update("value", "8"); result.Error != nil || result.RowsAffected != 1 {
		t.Fatalf("advance global revision = %d, %v", result.RowsAffected, result.Error)
	}
	mutation := migrationBatch("record-1", `{"secretNote":"plain-one"}`, `{"secretNote":"pgo:enc:v1:one"}`, 8)
	if _, err := store.ApplyBatch(context.Background(), mutation); !errors.Is(err, ErrMigrationConflict) {
		t.Fatalf("ApplyBatch(stale run) error = %v, want ErrMigrationConflict", err)
	}
	assertMigrationRecordJSON(t, db, "record-1", `{"secretNote":"plain-one"}`)
}

func TestGORMProtectedValueMigrationApplyBatchRequiresPreparedSnapshotHash(t *testing.T) {
	db, store := preparedMigrationStore(t, map[string]string{"record-1": `{"secretNote":"plain-one"}`})
	if result := db.Model(&gormAdminResourceRecord{}).
		Where("resource = ? AND id = ?", "customer-records", "record-1").
		Update("values_json", `{"secretNote":"changed-after-prepare"}`); result.Error != nil || result.RowsAffected != 1 {
		t.Fatalf("change prepared row = %d, %v", result.RowsAffected, result.Error)
	}
	mutation := migrationBatch("record-1", `{"secretNote":"changed-after-prepare"}`, `{"secretNote":"pgo:enc:v1:changed"}`, 7)
	if _, err := store.ApplyBatch(context.Background(), mutation); !errors.Is(err, ErrMigrationConflict) {
		t.Fatalf("ApplyBatch(snapshot drift) error = %v, want ErrMigrationConflict", err)
	}
	assertMigrationRecordJSON(t, db, "record-1", `{"secretNote":"changed-after-prepare"}`)
}

func TestGORMProtectedValueMigrationCheckpointIsMonotonicAndCumulative(t *testing.T) {
	db, store := preparedMigrationStore(t, map[string]string{
		"record-1": `{"secretNote":"plain-one"}`,
		"record-2": `{"secretNote":"plain-two"}`,
	})
	if _, err := store.ApplyBatch(context.Background(), migrationBatch("record-1", `{"secretNote":"plain-one"}`, `{"secretNote":"pgo:enc:v1:one"}`, 7)); err != nil {
		t.Fatalf("ApplyBatch(first) error = %v", err)
	}
	if _, err := store.ApplyBatch(context.Background(), migrationBatch("record-2", `{"secretNote":"plain-two"}`, `{"secretNote":"pgo:enc:v1:two"}`, 8)); err != nil {
		t.Fatalf("ApplyBatch(second) error = %v", err)
	}
	var checkpoint gormSensitiveMigrationCheckpoint
	if err := db.Where("run_id = ?", "run-apply").First(&checkpoint).Error; err != nil {
		t.Fatalf("read checkpoint error = %v", err)
	}
	if checkpoint.LastRecordID != "record-2" || checkpoint.ExpectedRevision != 9 || checkpoint.Rows != 2 {
		t.Fatalf("checkpoint = %+v, want cumulative row count and revision 9", checkpoint)
	}
}

func TestGORMProtectedValueMigrationCheckpointAllowsInterleavedTenantScopes(t *testing.T) {
	db := migrationOrdinaryDB(t, map[string]string{
		"a-1": `{"tenantCode":"tenant-a","secretNote":"plain-a1"}`,
		"a-2": `{"tenantCode":"tenant-a","secretNote":"plain-a2"}`,
		"b-1": `{"tenantCode":"tenant-b","secretNote":"plain-b1"}`,
	})
	store, err := NewGORMProtectedValueMigrationStore(db, "sqlite")
	if err != nil {
		t.Fatalf("NewGORMProtectedValueMigrationStore() error = %v", err)
	}
	plan := migrationResourcePlan("customer-records", "tenant-field", "tenantCode", "secretNote")
	if _, err := store.Prepare(context.Background(), sensitivemigration.RunRequest{
		RunID: "run-interleaved", PlanHash: "sha256:interleaved-plan",
		Plan: sensitivemigration.Plan{Resources: []sensitivemigration.ResourcePlan{plan}},
	}); err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}

	for _, mutation := range []sensitivemigration.BatchMutation{
		tenantMigrationBatch(plan, "tenant-a", "a-1", `{"tenantCode":"tenant-a","secretNote":"plain-a1"}`, `{"tenantCode":"tenant-a","secretNote":"pgo:enc:v1:a1"}`, 7),
		tenantMigrationBatch(plan, "tenant-b", "b-1", `{"tenantCode":"tenant-b","secretNote":"plain-b1"}`, `{"tenantCode":"tenant-b","secretNote":"pgo:enc:v1:b1"}`, 8),
		tenantMigrationBatch(plan, "tenant-a", "a-2", `{"tenantCode":"tenant-a","secretNote":"plain-a2"}`, `{"tenantCode":"tenant-a","secretNote":"pgo:enc:v1:a2"}`, 9),
	} {
		if _, err := store.ApplyBatch(context.Background(), mutation); err != nil {
			t.Fatalf("ApplyBatch(%s %s) error = %v", mutation.TenantID, mutation.LastRecordID, err)
		}
	}

	var checkpoints []gormSensitiveMigrationCheckpoint
	if err := db.Order("tenant_scope").Find(&checkpoints).Error; err != nil {
		t.Fatalf("read checkpoints error = %v", err)
	}
	if len(checkpoints) != 2 {
		t.Fatalf("checkpoint count = %d, want 2", len(checkpoints))
	}
	if checkpoints[0].TenantScope != "tenant-a" || checkpoints[0].LastRecordID != "a-2" || checkpoints[0].ExpectedRevision != 10 || checkpoints[0].Rows != 2 {
		t.Fatalf("tenant-a checkpoint = %+v", checkpoints[0])
	}
	if checkpoints[1].TenantScope != "tenant-b" || checkpoints[1].LastRecordID != "b-1" || checkpoints[1].ExpectedRevision != 9 || checkpoints[1].Rows != 1 {
		t.Fatalf("tenant-b checkpoint = %+v", checkpoints[1])
	}
	type eventRevision struct {
		Sequence uint64 `gorm:"column:sequence"`
		Revision uint64 `gorm:"column:revision"`
	}
	var events []eventRevision
	if err := db.Table(sensitiveMigrationEventsTable).Select("sequence, revision").Order("sequence").Scan(&events).Error; err != nil {
		t.Fatalf("read events error = %v", err)
	}
	if len(events) != 3 || events[0].Revision != 8 || events[1].Revision != 9 || events[2].Revision != 10 {
		t.Fatalf("event revisions = %+v, want 8, 9, 10", events)
	}
	if revision, err := loadGORMRevision(db); err != nil || revision != 10 {
		t.Fatalf("global revision = %d, %v; want 10", revision, err)
	}
	var run gormSensitiveMigrationRun
	if err := db.Where("run_id = ?", "run-interleaved").First(&run).Error; err != nil || run.ExpectedRevision != 10 {
		t.Fatalf("run revision = %d, %v; want 10", run.ExpectedRevision, err)
	}
}

func TestGORMProtectedValueMigrationCheckpointRejectsOrderReplayAndCASConflicts(t *testing.T) {
	t.Run("rows must be strictly ascending", func(t *testing.T) {
		_, store := preparedMigrationStore(t, map[string]string{
			"record-1": `{"secretNote":"plain-one"}`, "record-2": `{"secretNote":"plain-two"}`,
		})
		mutation := sensitivemigration.BatchMutation{
			RunID: "run-apply", Mode: sensitivemigration.ModeApply,
			Resource: migrationResourcePlan("customer-records", "global", "", "secretNote"),
			TenantID: dataprotection.GlobalTenantID, ExpectedRevision: 7, LastRecordID: "record-1",
			Rows: []sensitivemigration.RowMutation{
				{RecordID: "record-2", OriginalValuesJSON: `{"secretNote":"plain-two"}`, UpdatedValuesJSON: `{"secretNote":"pgo:enc:v1:two"}`},
				{RecordID: "record-1", OriginalValuesJSON: `{"secretNote":"plain-one"}`, UpdatedValuesJSON: `{"secretNote":"pgo:enc:v1:one"}`},
			},
		}
		if _, err := store.ApplyBatch(context.Background(), mutation); !errors.Is(err, ErrMigrationConflict) {
			t.Fatalf("ApplyBatch(out of order) error = %v, want ErrMigrationConflict", err)
		}
	})

	t.Run("last record must match final row", func(t *testing.T) {
		_, store := preparedMigrationStore(t, map[string]string{"record-1": `{"secretNote":"plain-one"}`})
		mutation := migrationBatch("record-1", `{"secretNote":"plain-one"}`, `{"secretNote":"pgo:enc:v1:one"}`, 7)
		mutation.LastRecordID = "record-2"
		if _, err := store.ApplyBatch(context.Background(), mutation); !errors.Is(err, ErrMigrationConflict) {
			t.Fatalf("ApplyBatch(last mismatch) error = %v, want ErrMigrationConflict", err)
		}
	})

	t.Run("committed record cannot replay", func(t *testing.T) {
		_, store := preparedMigrationStore(t, map[string]string{"record-1": `{"secretNote":"plain-one"}`})
		if _, err := store.ApplyBatch(context.Background(), migrationBatch("record-1", `{"secretNote":"plain-one"}`, `{"secretNote":"pgo:enc:v1:one"}`, 7)); err != nil {
			t.Fatalf("ApplyBatch(first) error = %v", err)
		}
		if _, err := store.ApplyBatch(context.Background(), migrationBatch("record-1", `{"secretNote":"pgo:enc:v1:one"}`, `{"secretNote":"pgo:enc:v1:two"}`, 8)); !errors.Is(err, ErrMigrationConflict) {
			t.Fatalf("ApplyBatch(replay) error = %v, want ErrMigrationConflict", err)
		}
	})

	t.Run("checkpoint referenced event revision is verified", func(t *testing.T) {
		db, store := preparedMigrationStore(t, map[string]string{
			"record-1": `{"secretNote":"plain-one"}`, "record-2": `{"secretNote":"plain-two"}`,
		})
		if _, err := store.ApplyBatch(context.Background(), migrationBatch("record-1", `{"secretNote":"plain-one"}`, `{"secretNote":"pgo:enc:v1:one"}`, 7)); err != nil {
			t.Fatalf("ApplyBatch(first) error = %v", err)
		}
		if result := db.Model(&gormSensitiveMigrationEvent{}).Where("run_id = ? AND sequence = ?", "run-apply", 1).Update("revision", 7); result.Error != nil || result.RowsAffected != 1 {
			t.Fatalf("corrupt checkpoint event = %d, %v", result.RowsAffected, result.Error)
		}
		if _, err := store.ApplyBatch(context.Background(), migrationBatch("record-2", `{"secretNote":"plain-two"}`, `{"secretNote":"pgo:enc:v1:two"}`, 8)); !errors.Is(err, ErrMigrationConflict) {
			t.Fatalf("ApplyBatch(stale checkpoint) error = %v, want ErrMigrationConflict", err)
		}
	})
}

func migrationResourcePlan(resource string, scope string, tenantField string, fields ...string) sensitivemigration.ResourcePlan {
	plan := sensitivemigration.ResourcePlan{Resource: resource, Scope: scope, TenantField: tenantField, SchemaVersion: 1}
	for _, field := range fields {
		plan.Fields = append(plan.Fields, sensitivemigration.FieldPlan{Key: field})
	}
	return plan
}

func migrationRunRequest(runID string) sensitivemigration.RunRequest {
	return sensitivemigration.RunRequest{
		RunID: runID, PlanHash: "sha256:apply-plan",
		Plan: sensitivemigration.Plan{Resources: []sensitivemigration.ResourcePlan{
			migrationResourcePlan("customer-records", "global", "", "secretNote"),
		}},
	}
}

func migrationBatch(recordID string, original string, updated string, revision uint64) sensitivemigration.BatchMutation {
	return sensitivemigration.BatchMutation{
		RunID: "run-apply", Mode: sensitivemigration.ModeApply,
		Resource: migrationResourcePlan("customer-records", "global", "", "secretNote"),
		TenantID: dataprotection.GlobalTenantID, ExpectedRevision: revision, LastRecordID: recordID,
		Rows: []sensitivemigration.RowMutation{{RecordID: recordID, OriginalValuesJSON: original, UpdatedValuesJSON: updated}},
	}
}

func tenantMigrationBatch(plan sensitivemigration.ResourcePlan, tenant string, recordID string, original string, updated string, revision uint64) sensitivemigration.BatchMutation {
	return sensitivemigration.BatchMutation{
		RunID: "run-interleaved", Mode: sensitivemigration.ModeApply, Resource: plan,
		TenantID: tenant, ExpectedRevision: revision, LastRecordID: recordID,
		Rows: []sensitivemigration.RowMutation{{RecordID: recordID, OriginalValuesJSON: original, UpdatedValuesJSON: updated}},
	}
}

func migrationOrdinaryDB(t *testing.T, rows map[string]string) *gorm.DB {
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
	return db
}

func preparedMigrationStore(t *testing.T, rows map[string]string) (*gorm.DB, *GORMProtectedValueMigrationStore) {
	t.Helper()
	db := migrationOrdinaryDB(t, rows)
	store, err := NewGORMProtectedValueMigrationStore(db, "sqlite")
	if err != nil {
		t.Fatalf("NewGORMProtectedValueMigrationStore() error = %v", err)
	}
	if _, err := store.Prepare(context.Background(), migrationRunRequest("run-apply")); err != nil {
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
