package adminresource

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"sync"
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
	request := migrationRunRequest("run-prepare")
	state, err := store.Prepare(context.Background(), request)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if state.RunID != "run-prepare" || state.PlanHash != request.PlanHash || state.ExpectedRevision != 7 || state.TargetCount != 1 {
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
	if run.PlanHash != request.PlanHash {
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
	if state.RunID != "run-apply" || state.PlanHash != migrationRunRequest("run-apply").PlanHash || state.ExpectedRevision != 7 {
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
	planHash := migrationRunRequest("run-race").PlanHash
	injected := false
	callbackName := "test:sensitive_migration_prepare_winner"
	if err := db.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if injected || tx.Statement == nil || tx.Statement.Table != sensitiveMigrationRunsTable {
			return
		}
		injected = true
		result := tx.Exec("INSERT INTO "+sensitiveMigrationRunsTable+" (run_id, plan_hash, actor_id, reason, approval_ref, backup_uri, backup_hash, restore_evidence, maintenance_confirmed, status, expected_revision, target_count, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			"run-race", planHash, "operator-1", "approved maintenance", "approval-1", "s3://backup/location", "sha256:"+strings.Repeat("b", 64), "restore-evidence-1", true, sensitivemigration.StatusPrepared, 7, 1, "2026-07-12T00:00:00Z")
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
	if state.RunID != "run-race" || state.PlanHash != planHash || state.ExpectedRevision != 7 {
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
		result := tx.Exec("INSERT INTO "+sensitiveMigrationRunsTable+" (run_id, plan_hash, actor_id, reason, approval_ref, backup_uri, backup_hash, restore_evidence, maintenance_confirmed, status, expected_revision, target_count, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			"run-race-conflict", "sha256:other-plan", "operator-1", "approved maintenance", "approval-1", "s3://backup/location", "sha256:"+strings.Repeat("b", 64), "restore-evidence-1", true, sensitivemigration.StatusPrepared, 7, 1, "2026-07-12T00:00:00Z")
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

func TestGORMProtectedValueMigrationPrepareRequiresMutationApprovals(t *testing.T) {
	db := migrationOrdinaryDB(t, map[string]string{"record-1": `{"secretNote":"plain-one"}`})
	store, err := NewGORMProtectedValueMigrationStore(db, "sqlite")
	if err != nil {
		t.Fatalf("NewGORMProtectedValueMigrationStore() error = %v", err)
	}

	request := migrationRunRequest("run-without-approvals")
	request.ActorID = ""
	if _, err := store.Prepare(context.Background(), request); !errors.Is(err, ErrMigrationInvalidOptions) {
		t.Fatalf("Prepare(without approvals) error = %v, want ErrMigrationInvalidOptions", err)
	}
}

func TestGORMProtectedValueMigrationApplyRefusesMissingPreparedRun(t *testing.T) {
	_, store := preparedMigrationStore(t, map[string]string{"record-1": `{"secretNote":"plain-one"}`})
	mutation := migrationBatch("record-1", `{"secretNote":"plain-one"}`, `{"secretNote":"pgo:enc:v1:one"}`, 7)
	mutation.RunID = "missing-prepared-run"

	if _, err := store.ApplyBatch(context.Background(), mutation); !errors.Is(err, ErrMigrationConflict) {
		t.Fatalf("ApplyBatch(missing prepared run) error = %v, want ErrMigrationConflict", err)
	}
}

func TestGORMProtectedValueMigrationEventChainPersistsCanonicalHashes(t *testing.T) {
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

	var events []gormSensitiveMigrationEvent
	if err := db.Order("sequence").Find(&events).Error; err != nil {
		t.Fatalf("read events error = %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("event count = %d, want 2", len(events))
	}
	for index, event := range events {
		if !strings.HasPrefix(event.PriorEventHash, "sha256:") || len(event.PriorEventHash) != 71 {
			t.Fatalf("event %d prior hash = %q, want canonical SHA-256", index+1, event.PriorEventHash)
		}
		if !strings.HasPrefix(event.EventHash, "sha256:") || len(event.EventHash) != 71 {
			t.Fatalf("event %d hash = %q, want canonical SHA-256", index+1, event.EventHash)
		}
	}
	if events[1].PriorEventHash != events[0].EventHash {
		t.Fatalf("event chain prior hash does not reference previous event")
	}
}

func TestGORMProtectedValueMigrationResumeRejectsPreparedPlanHashMismatch(t *testing.T) {
	_, store := preparedMigrationStore(t, map[string]string{"record-1": `{"secretNote":"plain-one"}`})
	request := migrationRunRequest("run-apply")
	request.Mode = sensitivemigration.ModeApply
	request.PlanHash = "sha256:different-plan"

	if _, err := store.StartOrResume(context.Background(), request); !errors.Is(err, ErrMigrationInvalidOptions) {
		t.Fatalf("StartOrResume(plan mismatch) error = %v, want ErrMigrationInvalidOptions", err)
	}
}

func TestGORMProtectedValueMigrationPrepareRecomputesCanonicalPlanHash(t *testing.T) {
	db := migrationOrdinaryDB(t, map[string]string{"record-1": `{"secretNote":"plain-one"}`})
	store, err := NewGORMProtectedValueMigrationStore(db, "sqlite")
	if err != nil {
		t.Fatalf("NewGORMProtectedValueMigrationStore() error = %v", err)
	}
	request := migrationRunRequest("run-plan-hash-mismatch")
	request.PlanHash = "sha256:" + strings.Repeat("f", 64)

	if _, err := store.Prepare(context.Background(), request); !errors.Is(err, ErrMigrationInvalidOptions) {
		t.Fatalf("Prepare(caller plan hash) error = %v, want ErrMigrationInvalidOptions", err)
	}
}

func TestGORMProtectedValueMigrationResumeRecomputesPolicyBoundPlanHash(t *testing.T) {
	_, store := preparedMigrationStore(t, map[string]string{"record-1": `{"secretNote":"plain-one"}`})
	request := migrationRunRequest("run-apply")
	request.Mode = sensitivemigration.ModeApply
	request.Plan.Resources[0].Fields[0].Policy.Normalization = dataprotection.NormalizationTrimV1

	if _, err := store.StartOrResume(context.Background(), request); !errors.Is(err, ErrMigrationInvalidOptions) {
		t.Fatalf("StartOrResume(changed policy) error = %v, want ErrMigrationInvalidOptions", err)
	}
}

func TestGORMProtectedValueMigrationTargetRowsRejectPolicyChangedAfterPrepare(t *testing.T) {
	_, store := preparedMigrationStore(t, map[string]string{"record-1": `{"secretNote":"plain-one"}`})
	plan := migrationResourcePlan("customer-records", "global", "", "secretNote")
	plan.Fields[0].Policy.Normalization = dataprotection.NormalizationTrimV1

	if _, err := store.TargetRows(context.Background(), "run-apply", plan, dataprotection.GlobalTenantID, "", 10); !errors.Is(err, ErrMigrationConflict) {
		t.Fatalf("TargetRows(changed policy) error = %v, want ErrMigrationConflict", err)
	}
}

func TestGORMProtectedValueMigrationTargetRowsUseSealedPreparedCoordinates(t *testing.T) {
	db, store := preparedMigrationStore(t, map[string]string{"record-1": `{"secretNote":"plain-one"}`})
	if err := db.Create(&gormAdminResourceRecord{
		Resource: "customer-records", ID: "record-2", Code: "record-2", Name: "record-2", Status: "enabled",
		UpdatedAt: "2026-07-12T00:00:00Z", ValuesJSON: `{"secretNote":"added-after-prepare"}`,
	}).Error; err != nil {
		t.Fatalf("create post-prepare row error = %v", err)
	}

	rows, err := store.TargetRows(context.Background(), "run-apply", migrationResourcePlan("customer-records", "global", "", "secretNote"), dataprotection.GlobalTenantID, "", 10)
	if err != nil {
		t.Fatalf("TargetRows() error = %v", err)
	}
	if len(rows) != 1 || rows[0].RecordID != "record-1" {
		t.Fatalf("TargetRows() = %+v, want only sealed record-1", rows)
	}
}

func TestGORMProtectedValueMigrationApplyTargetEnvelopeNoOpCommitsCursor(t *testing.T) {
	const valuesJSON = `{"displayName":"kept","secretNote":"pgo:enc:v1:existing"}`
	db, store := preparedMigrationStore(t, map[string]string{"record-1": valuesJSON})
	mutation := migrationBatch("record-1", valuesJSON, valuesJSON, 7)
	mutation.Counts = sensitivemigration.Counts{TargetEnvelope: 1}

	commit, err := store.ApplyBatch(context.Background(), mutation)
	if err != nil {
		t.Fatalf("ApplyBatch(target no-op) error = %v", err)
	}
	if commit.LastRecordID != "record-1" || commit.EventHash == "" {
		t.Fatalf("ApplyBatch(target no-op) commit = %+v", commit)
	}
	assertMigrationRecordJSON(t, db, "record-1", valuesJSON)
	var checkpoint gormSensitiveMigrationCheckpoint
	if err := db.Where("run_id = ?", "run-apply").First(&checkpoint).Error; err != nil {
		t.Fatalf("read checkpoint error = %v", err)
	}
	if checkpoint.LastRecordID != "record-1" || checkpoint.TargetEnvelope != 1 || checkpoint.Batches != 1 || checkpoint.EventHash != commit.EventHash {
		t.Fatalf("target no-op checkpoint = %+v", checkpoint)
	}
}

func TestGORMProtectedValueMigrationResumeLoadsCommittedCursorRevisionAndChain(t *testing.T) {
	_, store := preparedMigrationStore(t, map[string]string{
		"record-1": `{"secretNote":"plain-one"}`,
		"record-2": `{"secretNote":"plain-two"}`,
	})
	commit, err := store.ApplyBatch(context.Background(), migrationBatch("record-1", `{"secretNote":"plain-one"}`, `{"secretNote":"pgo:enc:v1:one"}`, 7))
	if err != nil {
		t.Fatalf("ApplyBatch(first) error = %v", err)
	}
	request := migrationRunRequest("run-apply")
	request.Mode = sensitivemigration.ModeApply
	state, err := store.StartOrResume(context.Background(), request)
	if err != nil {
		t.Fatalf("StartOrResume() error = %v", err)
	}
	if state.ExpectedRevision != 8 || state.Counts.Plaintext != 1 || len(state.Checkpoints) != 1 {
		t.Fatalf("resumed state = %+v", state)
	}
	checkpoint := state.Checkpoints[0]
	if checkpoint.LastRecordID != "record-1" || checkpoint.Batches != 1 || checkpoint.EventHash != commit.EventHash || state.EventChainHead != commit.EventHash {
		t.Fatalf("resumed checkpoint = %+v state head = %q", checkpoint, state.EventChainHead)
	}
}

func TestGORMProtectedValueMigrationEventChainTamperRollsBackNextBatch(t *testing.T) {
	db, store := preparedMigrationStore(t, map[string]string{
		"record-1": `{"secretNote":"plain-one"}`,
		"record-2": `{"secretNote":"plain-two"}`,
	})
	if _, err := store.ApplyBatch(context.Background(), migrationBatch("record-1", `{"secretNote":"plain-one"}`, `{"secretNote":"pgo:enc:v1:one"}`, 7)); err != nil {
		t.Fatalf("ApplyBatch(first) error = %v", err)
	}
	tampered := "sha256:" + strings.Repeat("d", 64)
	if result := db.Model(&gormSensitiveMigrationEvent{}).Where("run_id = ? AND sequence = ?", "run-apply", 1).Update("event_hash", tampered); result.Error != nil || result.RowsAffected != 1 {
		t.Fatalf("tamper event hash = %d, %v", result.RowsAffected, result.Error)
	}
	if result := db.Model(&gormSensitiveMigrationCheckpoint{}).Where("run_id = ?", "run-apply").Update("event_hash", tampered); result.Error != nil || result.RowsAffected != 1 {
		t.Fatalf("tamper checkpoint hash = %d, %v", result.RowsAffected, result.Error)
	}

	if _, err := store.ApplyBatch(context.Background(), migrationBatch("record-2", `{"secretNote":"plain-two"}`, `{"secretNote":"pgo:enc:v1:two"}`, 8)); !errors.Is(err, ErrMigrationConflict) {
		t.Fatalf("ApplyBatch(after chain tamper) error = %v, want ErrMigrationConflict", err)
	}
	assertMigrationRecordJSON(t, db, "record-2", `{"secretNote":"plain-two"}`)
	if revision, err := loadGORMRevision(db); err != nil || revision != 8 {
		t.Fatalf("revision after rejected tampered batch = %d, %v; want 8", revision, err)
	}
}

func TestGORMProtectedValueMigrationEventPersistsCommittedCursor(t *testing.T) {
	db, store := preparedMigrationStore(t, map[string]string{"record-1": `{"secretNote":"plain-one"}`})
	if _, err := store.ApplyBatch(context.Background(), migrationBatch("record-1", `{"secretNote":"plain-one"}`, `{"secretNote":"pgo:enc:v1:one"}`, 7)); err != nil {
		t.Fatalf("ApplyBatch() error = %v", err)
	}
	if !slices.Contains(migrationTableColumns(t, db, sensitiveMigrationEventsTable), "last_record_id") {
		t.Fatal("event journal does not persist the committed last record ID")
	}
	var cursor string
	if err := db.Table(sensitiveMigrationEventsTable).Select("last_record_id").Where("run_id = ? AND sequence = ?", "run-apply", 1).Scan(&cursor).Error; err != nil {
		t.Fatalf("read event cursor error = %v", err)
	}
	if cursor != "record-1" {
		t.Fatalf("event cursor = %q, want record-1", cursor)
	}
}

func TestGORMProtectedValueMigrationResumeReconstructsCheckpointExactlyFromEvents(t *testing.T) {
	tests := []struct {
		name   string
		tamper func(*testing.T, *gorm.DB)
	}{
		{name: "cursor", tamper: func(t *testing.T, db *gorm.DB) {
			result := db.Model(&gormSensitiveMigrationCheckpoint{}).Where("run_id = ?", "run-apply").Update("last_record_id", "record-9")
			if result.Error != nil || result.RowsAffected != 1 {
				t.Fatalf("tamper cursor = %d, %v", result.RowsAffected, result.Error)
			}
		}},
		{name: "counts", tamper: func(t *testing.T, db *gorm.DB) {
			result := db.Model(&gormSensitiveMigrationCheckpoint{}).Where("run_id = ?", "run-apply").Updates(map[string]any{"plaintext_count": 0, "target_envelope_count": 1})
			if result.Error != nil || result.RowsAffected != 1 {
				t.Fatalf("tamper counts = %d, %v", result.RowsAffected, result.Error)
			}
		}},
		{name: "event hash", tamper: func(t *testing.T, db *gorm.DB) {
			result := db.Model(&gormSensitiveMigrationCheckpoint{}).Where("run_id = ?", "run-apply").Update("event_hash", "sha256:"+strings.Repeat("e", 64))
			if result.Error != nil || result.RowsAffected != 1 {
				t.Fatalf("tamper checkpoint hash = %d, %v", result.RowsAffected, result.Error)
			}
		}},
		{name: "missing checkpoint", tamper: func(t *testing.T, db *gorm.DB) {
			result := db.Where("run_id = ?", "run-apply").Delete(&gormSensitiveMigrationCheckpoint{})
			if result.Error != nil || result.RowsAffected != 1 {
				t.Fatalf("delete checkpoint = %d, %v", result.RowsAffected, result.Error)
			}
		}},
		{name: "extra checkpoint", tamper: func(t *testing.T, db *gorm.DB) {
			var event gormSensitiveMigrationEvent
			if err := db.Where("run_id = ? AND sequence = ?", "run-apply", 1).First(&event).Error; err != nil {
				t.Fatalf("read event error = %v", err)
			}
			tenant := "other-tenant"
			tenantHash := migrationHash("tenant-scope", tenant)
			extra := gormSensitiveMigrationCheckpoint{
				CheckpointID: migrationSurrogateID("checkpoint", "run-apply", "customer-records", tenantHash, string(sensitivemigration.ModeApply)),
				RunID:        "run-apply", Resource: "customer-records", TenantScope: tenant, TenantScopeHash: tenantHash,
				Mode: string(sensitivemigration.ModeApply), LastRecordID: "record-1", ExpectedRevision: event.Revision,
				Status: sensitivemigration.StatusCompleted, EventSequence: event.Sequence, EventHash: event.EventHash,
				UpdatedAt: "2026-07-12T00:00:00Z",
			}
			if err := db.Create(&extra).Error; err != nil {
				t.Fatalf("create extra checkpoint error = %v", err)
			}
		}},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			db, store := preparedMigrationStore(t, map[string]string{"record-1": `{"secretNote":"plain-one"}`})
			if _, err := store.ApplyBatch(context.Background(), migrationBatch("record-1", `{"secretNote":"plain-one"}`, `{"secretNote":"pgo:enc:v1:one"}`, 7)); err != nil {
				t.Fatalf("ApplyBatch() error = %v", err)
			}
			testCase.tamper(t, db)
			request := migrationRunRequest("run-apply")
			request.Mode = sensitivemigration.ModeApply
			if _, err := store.StartOrResume(context.Background(), request); !errors.Is(err, ErrMigrationConflict) {
				t.Fatalf("StartOrResume(tampered %s) error = %v, want ErrMigrationConflict", testCase.name, err)
			}
		})
	}
}

func TestGORMProtectedValueMigrationFinishRejectsCheckpointForgedPastEvents(t *testing.T) {
	db, store := preparedMigrationStore(t, map[string]string{
		"record-1": `{"secretNote":"plain-one"}`,
		"record-2": `{"secretNote":"plain-two"}`,
	})
	if _, err := store.ApplyBatch(context.Background(), migrationBatch("record-1", `{"secretNote":"plain-one"}`, `{"secretNote":"pgo:enc:v1:one"}`, 7)); err != nil {
		t.Fatalf("ApplyBatch(first) error = %v", err)
	}
	result := db.Model(&gormSensitiveMigrationCheckpoint{}).Where("run_id = ?", "run-apply").Updates(map[string]any{
		"last_record_id": "record-2", "row_count": 2, "plaintext_count": 2,
	})
	if result.Error != nil || result.RowsAffected != 1 {
		t.Fatalf("forge checkpoint = %d, %v", result.RowsAffected, result.Error)
	}
	if err := store.FinishRun(context.Background(), "run-apply", sensitivemigration.StatusCompleted); !errors.Is(err, ErrMigrationConflict) {
		t.Fatalf("FinishRun(forged checkpoint) error = %v, want ErrMigrationConflict", err)
	}
	var run gormSensitiveMigrationRun
	if err := db.Where("run_id = ?", "run-apply").First(&run).Error; err != nil {
		t.Fatalf("read run error = %v", err)
	}
	if run.Status != sensitivemigration.StatusPrepared {
		t.Fatalf("run status = %q, want prepared", run.Status)
	}
}

func TestGORMProtectedValueMigrationFinishCompletedRunStillVerifiesJournal(t *testing.T) {
	db, store := preparedMigrationStore(t, map[string]string{"record-1": `{"secretNote":"plain-one"}`})
	if _, err := store.ApplyBatch(context.Background(), migrationBatch("record-1", `{"secretNote":"plain-one"}`, `{"secretNote":"pgo:enc:v1:one"}`, 7)); err != nil {
		t.Fatalf("ApplyBatch() error = %v", err)
	}
	if err := store.FinishRun(context.Background(), "run-apply", sensitivemigration.StatusCompleted); err != nil {
		t.Fatalf("FinishRun(first) error = %v", err)
	}
	result := db.Model(&gormSensitiveMigrationCheckpoint{}).Where("run_id = ?", "run-apply").Update("last_record_id", "record-9")
	if result.Error != nil || result.RowsAffected != 1 {
		t.Fatalf("tamper completed checkpoint = %d, %v", result.RowsAffected, result.Error)
	}
	if err := store.FinishRun(context.Background(), "run-apply", sensitivemigration.StatusCompleted); !errors.Is(err, ErrMigrationConflict) {
		t.Fatalf("FinishRun(completed tampered run) error = %v, want ErrMigrationConflict", err)
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
		Counts: sensitivemigration.Counts{Plaintext: 1},
		Rows: []sensitivemigration.RowMutation{{
			RecordID: "record-1", OriginalValuesJSON: `{"secretNote":"plain-one"}`,
			UpdatedValuesJSON: `{"secretNote":"pgo:enc:v1:migrated"}`,
			Escrow: []sensitivemigration.EscrowEntry{{
				RunID: "run-apply", Resource: "customer-records", RecordID: "record-1", FieldKey: "secretNote",
				TenantID: dataprotection.GlobalTenantID, ProtectedOriginal: migrationEscrowEnvelope(),
				MigratedValueHash: sensitivemigration.HashMigratedValue("pgo:enc:v1:migrated"),
			}},
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
	assertGORMCount(t, db, &gormSensitiveMigrationEscrow{}, 1)
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
		Counts: sensitivemigration.Counts{Plaintext: 2},
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
		Counts: sensitivemigration.Counts{Plaintext: 1},
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

func TestGORMProtectedValueMigrationApplyBatchRejectsLargeIntegerNonTargetChange(t *testing.T) {
	const original = `{"largeNumber":9007199254740992,"secretNote":"plain-one"}`
	const updated = `{"largeNumber":9007199254740993,"secretNote":"pgo:enc:v1:one"}`
	db, store := preparedMigrationStore(t, map[string]string{"record-1": original})
	mutation := migrationBatch("record-1", original, updated, 7)

	if _, err := store.ApplyBatch(context.Background(), mutation); !errors.Is(err, ErrMigrationConflict) {
		t.Fatalf("ApplyBatch(large non-target mutation) error = %v, want ErrMigrationConflict", err)
	}
	assertMigrationRecordJSON(t, db, "record-1", original)
}

func TestGORMProtectedValueMigrationApplyBatchRejectsDuplicateNonTargetKey(t *testing.T) {
	const original = `{"displayName":"first","displayName":"second","secretNote":"plain-one"}`
	const updated = `{"displayName":"first","displayName":"second","secretNote":"pgo:enc:v1:one"}`
	db, store := preparedMigrationStore(t, map[string]string{"record-1": original})
	mutation := migrationBatch("record-1", original, updated, 7)

	if _, err := store.ApplyBatch(context.Background(), mutation); !errors.Is(err, ErrMigrationConflict) {
		t.Fatalf("ApplyBatch(duplicate non-target key) error = %v, want ErrMigrationConflict", err)
	}
	assertMigrationRecordJSON(t, db, "record-1", original)
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
	request := migrationRunRequest("run-interleaved")
	request.Plan = sensitivemigration.Plan{Resources: []sensitivemigration.ResourcePlan{plan}}
	request.PlanHash = sensitivemigration.PlanHash(request.Plan)
	if _, err := store.Prepare(context.Background(), request); err != nil {
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
			Counts: sensitivemigration.Counts{Plaintext: 2},
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

func TestGORMProtectedValueMigrationEscrowCommitsAtomicallyWithApply(t *testing.T) {
	db, store := preparedMigrationStore(t, map[string]string{"record-1": `{"displayName":"kept","secretNote":"plain-one"}`})
	mutation := migrationBatch("record-1", `{"displayName":"kept","secretNote":"plain-one"}`, `{"displayName":"kept","secretNote":"pgo:enc:v1:migrated"}`, 7)
	mutation.Rows[0].Escrow = []sensitivemigration.EscrowEntry{{
		RunID: "run-apply", Resource: "customer-records", RecordID: "record-1", FieldKey: "secretNote",
		TenantID: dataprotection.GlobalTenantID, ProtectedOriginal: migrationEscrowEnvelope(),
		MigratedValueHash: sensitivemigration.HashMigratedValue("pgo:enc:v1:migrated"),
	}}
	if _, err := store.ApplyBatch(context.Background(), mutation); err != nil {
		t.Fatalf("ApplyBatch() error = %v", err)
	}
	var escrow gormSensitiveMigrationEscrow
	if err := db.Where("run_id = ?", "run-apply").First(&escrow).Error; err != nil {
		t.Fatalf("read escrow error = %v", err)
	}
	if escrow.ProtectedOriginal != migrationEscrowEnvelope() || escrow.MigratedValueHash != sensitivemigration.HashMigratedValue("pgo:enc:v1:migrated") {
		t.Fatalf("persisted escrow = %+v", escrow)
	}
	if strings.Contains(escrow.ProtectedOriginal, "plain-one") {
		t.Fatal("escrow persisted plaintext")
	}
}

func TestGORMProtectedValueMigrationEscrowRollsBackWithRowConflict(t *testing.T) {
	db, store := preparedMigrationStore(t, map[string]string{"record-1": `{"secretNote":"plain-one"}`})
	mutation := migrationBatch("record-1", `{"secretNote":"stale"}`, `{"secretNote":"pgo:enc:v1:migrated"}`, 7)
	mutation.Rows[0].Escrow = []sensitivemigration.EscrowEntry{{
		RunID: "run-apply", Resource: "customer-records", RecordID: "record-1", FieldKey: "secretNote",
		TenantID: dataprotection.GlobalTenantID, ProtectedOriginal: migrationEscrowEnvelope(),
		MigratedValueHash: sensitivemigration.HashMigratedValue("pgo:enc:v1:migrated"),
	}}
	if _, err := store.ApplyBatch(context.Background(), mutation); !errors.Is(err, ErrMigrationConflict) {
		t.Fatalf("ApplyBatch(stale) error = %v, want ErrMigrationConflict", err)
	}
	assertGORMCount(t, db, &gormSensitiveMigrationEscrow{}, 0)
	assertGORMCount(t, db, &gormSensitiveMigrationEvent{}, 0)
	assertGORMCount(t, db, &gormSensitiveMigrationCheckpoint{}, 0)
}

func TestGORMProtectedValueMigrationEscrowRejectsPlaintextOriginal(t *testing.T) {
	db, store := preparedMigrationStore(t, map[string]string{"record-1": `{"secretNote":"plain-one"}`})
	mutation := migrationBatch("record-1", `{"secretNote":"plain-one"}`, `{"secretNote":"pgo:enc:v1:migrated"}`, 7)
	mutation.Rows[0].Escrow[0].ProtectedOriginal = "plain-one"
	if _, err := store.ApplyBatch(context.Background(), mutation); !errors.Is(err, ErrMigrationConflict) {
		t.Fatalf("ApplyBatch(plaintext escrow) error = %v, want ErrMigrationConflict", err)
	}
	assertMigrationRecordJSON(t, db, "record-1", `{"secretNote":"plain-one"}`)
	assertGORMCount(t, db, &gormSensitiveMigrationEscrow{}, 0)
}

func TestGORMProtectedValueMigrationRollbackRefusesPostMigrationEdit(t *testing.T) {
	db, store := appliedEscrowMigrationStore(t)
	if result := db.Model(&gormAdminResourceRecord{}).Where("resource = ? AND id = ?", "customer-records", "record-1").
		Update("values_json", `{"displayName":"kept","secretNote":"pgo:enc:v1:post-migration-edit"}`); result.Error != nil || result.RowsAffected != 1 {
		t.Fatalf("edit migrated row = %d, %v", result.RowsAffected, result.Error)
	}
	mutation := migrationRollbackBatch(`{"displayName":"kept","secretNote":"pgo:enc:v1:post-migration-edit"}`, `{"displayName":"kept","secretNote":"plain-one"}`, 8)
	if _, err := store.RollbackBatch(context.Background(), mutation); !errors.Is(err, ErrMigrationConflict) {
		t.Fatalf("RollbackBatch(post-edit) error = %v, want ErrMigrationConflict", err)
	}
	assertMigrationRecordJSON(t, db, "record-1", `{"displayName":"kept","secretNote":"pgo:enc:v1:post-migration-edit"}`)
	assertGORMCount(t, db, &gormSensitiveMigrationCheckpoint{}, 1)
	assertGORMCount(t, db, &gormSensitiveMigrationEvent{}, 2)
}

func TestGORMProtectedValueMigrationRollbackRestoresOnlyTargetAndCommitsJournalAtomically(t *testing.T) {
	db, store := appliedEscrowMigrationStore(t)
	mutation := migrationRollbackBatch(`{"displayName":"kept","secretNote":"pgo:enc:v1:migrated"}`, `{"displayName":"kept","secretNote":"plain-one"}`, 8)
	commit, err := store.RollbackBatch(context.Background(), mutation)
	if err != nil {
		t.Fatalf("RollbackBatch() error = %v", err)
	}
	if commit.Revision != 9 || commit.EventHash == "" || commit.LastRecordID != "record-1" {
		t.Fatalf("RollbackBatch() commit = %+v", commit)
	}
	assertMigrationRecordJSON(t, db, "record-1", `{"displayName":"kept","secretNote":"plain-one"}`)
	if err := store.FinishRollback(context.Background(), "run-apply"); err != nil {
		t.Fatalf("FinishRollback() error = %v", err)
	}
	var run gormSensitiveMigrationRun
	if err := db.Where("run_id = ?", "run-apply").First(&run).Error; err != nil {
		t.Fatal(err)
	}
	if run.RollbackStatus != sensitivemigration.StatusCompleted || run.ExpectedRevision != 9 {
		t.Fatalf("rollback run = %+v", run)
	}
	var rollbackCheckpoint gormSensitiveMigrationCheckpoint
	if err := db.Where("run_id = ? AND mode = ?", "run-apply", sensitivemigration.ModeRollback).First(&rollbackCheckpoint).Error; err != nil {
		t.Fatal(err)
	}
	if rollbackCheckpoint.Rows != 1 || rollbackCheckpoint.EventHash != commit.EventHash {
		t.Fatalf("rollback checkpoint = %+v", rollbackCheckpoint)
	}
}

func TestGORMProtectedValueMigrationRunnerRehearsesAndRollsBackIdempotently(t *testing.T) {
	const original = `{"displayName":"kept","secretNote":"plain-one"}`
	db := migrationOrdinaryDB(t, map[string]string{"record-1": original})
	store, err := NewGORMProtectedValueMigrationStore(db, "sqlite")
	if err != nil {
		t.Fatal(err)
	}
	request := migrationRunRequest("run-apply")
	runner := sensitivemigration.NewRunner(request.Plan, migrationStoreRuntime(), store)
	if _, err := runner.Run(context.Background(), sensitivemigration.Options{Mode: sensitivemigration.ModePrepare, Request: request}); err != nil {
		t.Fatalf("Run(prepare) error = %v", err)
	}
	if _, err := runner.Run(context.Background(), sensitivemigration.Options{Mode: sensitivemigration.ModeApply, BatchSize: 1, Request: request}); err != nil {
		t.Fatalf("Run(apply) error = %v", err)
	}
	var migrated gormAdminResourceRecord
	if err := db.Where("resource = ? AND id = ?", "customer-records", "record-1").First(&migrated).Error; err != nil {
		t.Fatal(err)
	}
	if strings.Contains(migrated.ValuesJSON, "plain-one") || !strings.Contains(migrated.ValuesJSON, "pgo:enc:v1:") {
		t.Fatal("apply did not replace plaintext with target envelope")
	}

	rehearsal, err := runner.Run(context.Background(), sensitivemigration.Options{Mode: sensitivemigration.ModeRehearseRestore, Request: request})
	if err != nil {
		t.Fatalf("Run(rehearse) error = %v", err)
	}
	serialized, err := json.Marshal(rehearsal)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(serialized), "plain-one") || strings.Contains(string(serialized), "pgo:enc:") || strings.Contains(string(serialized), "record-1") {
		t.Fatal("rehearsal report exposed protected data or coordinates")
	}
	if rehearsal.Status != sensitivemigration.StatusCompleted || rehearsal.Counts.Plaintext != 1 {
		t.Fatalf("rehearsal report = %+v", rehearsal)
	}

	rollback, err := runner.Run(context.Background(), sensitivemigration.Options{Mode: sensitivemigration.ModeRollback, BatchSize: 1, Request: request})
	if err != nil {
		t.Fatalf("Run(rollback) error = %v", err)
	}
	if rollback.Status != sensitivemigration.StatusCompleted || rollback.Counts.Plaintext != 1 {
		t.Fatalf("rollback report = %+v", rollback)
	}
	assertMigrationRecordJSON(t, db, "record-1", original)
	if revision, err := loadGORMRevision(db); err != nil || revision != 9 {
		t.Fatalf("revision = %d, %v; want 9", revision, err)
	}
	assertGORMCount(t, db, &gormSensitiveMigrationEvent{}, 3)
	assertGORMCount(t, db, &gormSensitiveMigrationEscrow{}, 1)

	second, err := runner.Run(context.Background(), sensitivemigration.Options{Mode: sensitivemigration.ModeRollback, BatchSize: 1, Request: request})
	if err != nil || second.Status != sensitivemigration.StatusCompleted {
		t.Fatalf("Run(rollback resume) report = %+v error = %v", second, err)
	}
	if revision, err := loadGORMRevision(db); err != nil || revision != 9 {
		t.Fatalf("revision after idempotent rollback = %d, %v; want 9", revision, err)
	}
	assertGORMCount(t, db, &gormSensitiveMigrationEvent{}, 3)
}

func appliedEscrowMigrationStore(t *testing.T) (*gorm.DB, *GORMProtectedValueMigrationStore) {
	t.Helper()
	db, store := preparedMigrationStore(t, map[string]string{"record-1": `{"displayName":"kept","secretNote":"plain-one"}`})
	apply := migrationBatch("record-1", `{"displayName":"kept","secretNote":"plain-one"}`, `{"displayName":"kept","secretNote":"pgo:enc:v1:migrated"}`, 7)
	apply.Rows[0].Escrow = []sensitivemigration.EscrowEntry{{
		RunID: "run-apply", Resource: "customer-records", RecordID: "record-1", FieldKey: "secretNote",
		TenantID: dataprotection.GlobalTenantID, ProtectedOriginal: migrationEscrowEnvelope(),
		MigratedValueHash: sensitivemigration.HashMigratedValue("pgo:enc:v1:migrated"),
	}}
	if _, err := store.ApplyBatch(context.Background(), apply); err != nil {
		t.Fatalf("ApplyBatch() error = %v", err)
	}
	if err := store.FinishRun(context.Background(), "run-apply", sensitivemigration.StatusCompleted); err != nil {
		t.Fatalf("FinishRun() error = %v", err)
	}
	if _, err := store.CommitRehearsal(context.Background(), "run-apply", 1); err != nil {
		t.Fatalf("CommitRehearsal() error = %v", err)
	}
	return db, store
}

func migrationRollbackBatch(original string, updated string, revision uint64) sensitivemigration.BatchMutation {
	mutation := migrationBatch("record-1", original, updated, revision)
	mutation.Mode = sensitivemigration.ModeRollback
	mutation.Counts = sensitivemigration.Counts{Plaintext: 1}
	mutation.Rows[0].Escrow = []sensitivemigration.EscrowEntry{{
		RunID: "run-apply", Resource: "customer-records", RecordID: "record-1", FieldKey: "secretNote",
		TenantID: dataprotection.GlobalTenantID, ProtectedOriginal: migrationEscrowEnvelope(),
		MigratedValueHash: sensitivemigration.HashMigratedValue("pgo:enc:v1:migrated"),
	}}
	return mutation
}

func migrationResourcePlan(resource string, scope string, tenantField string, fields ...string) sensitivemigration.ResourcePlan {
	plan := sensitivemigration.ResourcePlan{Resource: resource, Scope: scope, TenantField: tenantField, SchemaVersion: 1}
	for _, field := range fields {
		plan.Fields = append(plan.Fields, sensitivemigration.FieldPlan{
			Key: field, Policy: dataprotection.FieldPolicy{
				Format: dataprotection.FormatAES256GCMV1, Normalization: dataprotection.NormalizationRawV1,
			},
		})
	}
	return plan
}

func migrationRunRequest(runID string) sensitivemigration.RunRequest {
	plan := sensitivemigration.Plan{Resources: []sensitivemigration.ResourcePlan{
		migrationResourcePlan("customer-records", "global", "", "secretNote"),
	}}
	return sensitivemigration.RunRequest{
		RunID: runID, PlanHash: sensitivemigration.PlanHash(plan),
		ActorID: "operator-1", Reason: "approved maintenance", ApprovalRef: "approval-1",
		BackupURI: "s3://backup/location", BackupHash: "sha256:" + strings.Repeat("b", 64),
		RestoreEvidence: "restore-evidence-1", MaintenanceConfirmed: true,
		Plan: plan,
	}
}

func migrationBatch(recordID string, original string, updated string, revision uint64) sensitivemigration.BatchMutation {
	mutation := sensitivemigration.BatchMutation{
		RunID: "run-apply", Mode: sensitivemigration.ModeApply,
		Resource: migrationResourcePlan("customer-records", "global", "", "secretNote"),
		TenantID: dataprotection.GlobalTenantID, ExpectedRevision: revision, LastRecordID: recordID,
		Counts: sensitivemigration.Counts{Plaintext: 1},
		Rows:   []sensitivemigration.RowMutation{{RecordID: recordID, OriginalValuesJSON: original, UpdatedValuesJSON: updated}},
	}
	if original != updated {
		migrated, err := migrationStringField(updated, "secretNote")
		if err == nil {
			mutation.Rows[0].Escrow = []sensitivemigration.EscrowEntry{{
				RunID: mutation.RunID, Resource: mutation.Resource.Resource, RecordID: recordID, FieldKey: "secretNote",
				TenantID: mutation.TenantID, ProtectedOriginal: migrationEscrowEnvelope(),
				MigratedValueHash: sensitivemigration.HashMigratedValue(migrated),
			}}
		}
	}
	return mutation
}

func tenantMigrationBatch(plan sensitivemigration.ResourcePlan, tenant string, recordID string, original string, updated string, revision uint64) sensitivemigration.BatchMutation {
	mutation := sensitivemigration.BatchMutation{
		RunID: "run-interleaved", Mode: sensitivemigration.ModeApply, Resource: plan,
		TenantID: tenant, ExpectedRevision: revision, LastRecordID: recordID,
		Counts: sensitivemigration.Counts{Plaintext: 1},
		Rows:   []sensitivemigration.RowMutation{{RecordID: recordID, OriginalValuesJSON: original, UpdatedValuesJSON: updated}},
	}
	if original != updated {
		migrated, err := migrationStringField(updated, "secretNote")
		if err == nil {
			mutation.Rows[0].Escrow = []sensitivemigration.EscrowEntry{{
				RunID: mutation.RunID, Resource: plan.Resource, RecordID: recordID, FieldKey: "secretNote", TenantID: tenant,
				ProtectedOriginal: migrationEscrowEnvelope(), MigratedValueHash: sensitivemigration.HashMigratedValue(migrated),
			}}
		}
	}
	return mutation
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

var (
	migrationEscrowEnvelopeOnce  sync.Once
	migrationEscrowEnvelopeValue string
)

func migrationEscrowEnvelope() string {
	migrationEscrowEnvelopeOnce.Do(func() {
		provider, err := dataprotection.NewStaticKeyProvider(dataprotection.StaticKeyProviderConfig{
			Kind:                  dataprotection.ProviderEnvAES256,
			ActiveEncryptionKeyID: "migration-escrow-v1",
			EncryptionKeys:        map[string][]byte{"migration-escrow-v1": []byte(strings.Repeat("e", 32))},
			ActiveBlindIndexKeyID: "migration-index-v1",
			BlindIndexKeys:        map[string][]byte{"migration-index-v1": []byte(strings.Repeat("i", 32))},
		})
		if err != nil {
			panic(err)
		}
		policy, fieldContext := sensitivemigration.EscrowContext(
			"run-apply", dataprotection.GlobalTenantID, "customer-records", "record-1", "secretNote",
		)
		migrationEscrowEnvelopeValue, err = dataprotection.NewRuntime(provider).Protect(context.Background(), "protected-fixture", policy, fieldContext)
		if err != nil {
			panic(err)
		}
	})
	return migrationEscrowEnvelopeValue
}

func migrationStoreRuntime() *dataprotection.Service {
	provider, err := dataprotection.NewStaticKeyProvider(dataprotection.StaticKeyProviderConfig{
		Kind:                  dataprotection.ProviderEnvAES256,
		ActiveEncryptionKeyID: "migration-runtime-v1",
		EncryptionKeys:        map[string][]byte{"migration-runtime-v1": []byte(strings.Repeat("r", 32))},
		ActiveBlindIndexKeyID: "migration-runtime-index-v1",
		BlindIndexKeys:        map[string][]byte{"migration-runtime-index-v1": []byte(strings.Repeat("j", 32))},
	})
	if err != nil {
		panic(err)
	}
	return dataprotection.NewRuntime(provider)
}
