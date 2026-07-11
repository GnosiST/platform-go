package session

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestSessionIntegrationDatabaseNameGuard(t *testing.T) {
	for _, tt := range []struct {
		name string
		safe bool
	}{
		{name: "platform_session_integration_test", safe: true},
		{name: "platform_session_integration_ci", safe: true},
		{name: "platform_test", safe: false},
		{name: "platform_production", safe: false},
		{name: "", safe: false},
	} {
		if got := safeSessionIntegrationDatabase(tt.name); got != tt.safe {
			t.Fatalf("safeSessionIntegrationDatabase(%q) = %t, want %t", tt.name, got, tt.safe)
		}
	}
}

func TestGORMRepositoryMigratesLegacySchemaOnMySQL(t *testing.T) {
	dsn := os.Getenv("PLATFORM_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("PLATFORM_TEST_MYSQL_DSN is not set")
	}
	db := openSessionIntegrationDB(t, mysql.Open(dsn))
	t.Cleanup(func() { resetSessionIntegrationTables(t, db) })
	for _, tt := range []struct {
		name  string
		setup func(*testing.T, *gorm.DB)
	}{
		{name: "legacy current table", setup: func(t *testing.T, db *gorm.DB) { createMySQLLegacySessionTable(t, db, sessionsTable) }},
		{name: "replacement only", setup: func(t *testing.T, db *gorm.DB) { createMySQLDigestSessionTable(t, db, mysqlSessionReplacementTable) }},
		{name: "legacy only", setup: func(t *testing.T, db *gorm.DB) { createMySQLLegacySessionTable(t, db, mysqlSessionLegacyTable) }},
		{name: "digest current with leftovers", setup: func(t *testing.T, db *gorm.DB) {
			createMySQLDigestSessionTable(t, db, sessionsTable)
			createMySQLDigestSessionTable(t, db, mysqlSessionReplacementTable)
			createMySQLLegacySessionTable(t, db, mysqlSessionLegacyTable)
		}},
		{name: "legacy current with replacement", setup: func(t *testing.T, db *gorm.DB) {
			createMySQLLegacySessionTable(t, db, sessionsTable)
			createMySQLDigestSessionTable(t, db, mysqlSessionReplacementTable)
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			resetSessionIntegrationTables(t, db)
			tt.setup(t, db)
			if _, err := NewGORMRepository(context.Background(), db); err != nil {
				t.Fatalf("NewGORMRepository() error = %v", err)
			}
			assertSessionIntegrationSchema(t, db)
		})
	}
}

func TestGORMRepositoryMigratesLegacySchemaOnPostgres(t *testing.T) {
	dsn := os.Getenv("PLATFORM_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("PLATFORM_TEST_POSTGRES_DSN is not set")
	}
	db := openSessionIntegrationDB(t, postgres.Open(dsn))
	resetSessionIntegrationTables(t, db)
	t.Cleanup(func() { resetSessionIntegrationTables(t, db) })
	raw := "raw-session-marker"
	if err := db.Exec(`CREATE TABLE platform_sessions (token text PRIMARY KEY, username text NOT NULL, issued_at timestamptz NOT NULL, expires_at timestamptz NOT NULL, revoked_at timestamptz)`).Error; err != nil {
		t.Fatalf("create legacy table error = %v", err)
	}
	if err := db.Exec(`INSERT INTO platform_sessions (token, username, issued_at, expires_at) VALUES (?, ?, ?, ?)`, raw, "ops", time.Now().Add(-time.Hour), time.Now().Add(time.Hour)).Error; err != nil {
		t.Fatalf("insert legacy row error = %v", err)
	}
	if _, err := NewGORMRepository(context.Background(), db); err != nil {
		t.Fatalf("NewGORMRepository() error = %v", err)
	}
	assertSessionIntegrationSchema(t, db)
}

func openSessionIntegrationDB(t *testing.T, dialector gorm.Dialector) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(dialector, &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB() error = %v", err)
	}
	if err := sqlDB.PingContext(context.Background()); err != nil {
		t.Fatalf("database ping error = %v", err)
	}
	requireSafeSessionIntegrationDatabase(t, db)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func requireSafeSessionIntegrationDatabase(t *testing.T, db *gorm.DB) {
	t.Helper()
	query := "SELECT current_database()"
	if db.Dialector.Name() == "mysql" {
		query = "SELECT DATABASE()"
	}
	var name string
	if err := db.Raw(query).Scan(&name).Error; err != nil {
		t.Fatalf("read integration database name error = %v", err)
	}
	if !safeSessionIntegrationDatabase(name) {
		t.Fatalf("refusing destructive session integration test against database %q", name)
	}
}

func safeSessionIntegrationDatabase(name string) bool {
	return strings.HasPrefix(strings.TrimSpace(name), "platform_session_integration_")
}

func resetSessionIntegrationTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, table := range []string{sessionsTable, mysqlSessionReplacementTable, mysqlSessionLegacyTable} {
		if err := db.Exec("DROP TABLE IF EXISTS " + table).Error; err != nil {
			t.Fatalf("drop table %s error = %v", table, err)
		}
	}
}

func createMySQLLegacySessionTable(t *testing.T, db *gorm.DB, table string) {
	t.Helper()
	statement := fmt.Sprintf("CREATE TABLE %s (token varchar(256) PRIMARY KEY, username varchar(256) NOT NULL, issued_at datetime(3) NOT NULL, expires_at datetime(3) NOT NULL, revoked_at datetime(3) NULL)", table)
	if err := db.Exec(statement).Error; err != nil {
		t.Fatalf("create legacy table %s error = %v", table, err)
	}
	insert := fmt.Sprintf("INSERT INTO %s (token, username, issued_at, expires_at) VALUES (?, ?, ?, ?)", table)
	if err := db.Exec(insert, "raw-session-marker", "ops", time.Now().Add(-time.Hour), time.Now().Add(time.Hour)).Error; err != nil {
		t.Fatalf("insert legacy row into %s error = %v", table, err)
	}
}

func createMySQLDigestSessionTable(t *testing.T, db *gorm.DB, table string) {
	t.Helper()
	statement := fmt.Sprintf("CREATE TABLE %s (token_digest varchar(80) PRIMARY KEY, username varchar(256) NOT NULL, issued_at datetime(3) NOT NULL, expires_at datetime(3) NOT NULL, revoked_at datetime(3) NULL, INDEX idx_platform_sessions_expires_at (expires_at))", table)
	if err := db.Exec(statement).Error; err != nil {
		t.Fatalf("create digest table %s error = %v", table, err)
	}
}

func assertSessionIntegrationSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	migrator := db.Migrator()
	if !migrator.HasTable(sessionsTable) || !migrator.HasColumn(&gormSessionRecord{}, "token_digest") {
		t.Fatal("session digest table is missing token_digest")
	}
	if migrator.HasColumn(&gormSessionRecord{}, "token") {
		t.Fatal("session digest table still contains legacy token column")
	}
	if migrator.HasTable(mysqlSessionReplacementTable) || migrator.HasTable(mysqlSessionLegacyTable) {
		t.Fatal("session migration left replacement or legacy tables behind")
	}
	var count int64
	if err := db.Table(sessionsTable).Count(&count).Error; err != nil {
		t.Fatalf("count migrated sessions error = %v", err)
	}
	if count != 0 {
		t.Fatalf("migrated session count = %d, want 0", count)
	}
}
