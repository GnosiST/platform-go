package storage

import "testing"

func TestOpenGORMRejectsUnknownDriver(t *testing.T) {
	_, err := OpenGORM(Config{Driver: "mongo", DSN: "memory"})
	if err == nil {
		t.Fatalf("OpenGORM() error = nil, want unknown driver")
	}
}

func TestOpenGORMOpensSQLiteMemory(t *testing.T) {
	db, err := OpenGORM(Config{Driver: "sqlite", DSN: ":memory:"})
	if err != nil {
		t.Fatalf("OpenGORM(sqlite) error = %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := sqlDB.Ping(); err != nil {
		t.Fatalf("sqlite ping error = %v", err)
	}
}
