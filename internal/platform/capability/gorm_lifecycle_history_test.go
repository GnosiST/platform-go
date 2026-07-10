package capability

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"platform-go/internal/platform/storage"

	"gorm.io/gorm"
)

func TestGORMLifecycleHistoryRecordsMigrationsAndSeeds(t *testing.T) {
	db := openGORMLifecycleTestDB(t)
	history, err := NewGORMLifecycleHistory(context.Background(), db)
	if err != nil {
		t.Fatalf("NewGORMLifecycleHistory() error = %v", err)
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

func TestGORMLifecycleHistoryDoesNotDuplicateRecords(t *testing.T) {
	db := openGORMLifecycleTestDB(t)
	history, err := NewGORMLifecycleHistory(context.Background(), db)
	if err != nil {
		t.Fatalf("NewGORMLifecycleHistory() error = %v", err)
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

func TestRecordedLifecycleExecutorSkipsStepsWithGORMHistory(t *testing.T) {
	db := openGORMLifecycleTestDB(t)
	history, err := NewGORMLifecycleHistory(context.Background(), db)
	if err != nil {
		t.Fatalf("NewGORMLifecycleHistory() error = %v", err)
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
		t.Fatalf("calls = %#v, want each GORM-backed step once", calls)
	}
	if got := len(history.Records(context.Background())); got != 2 {
		t.Fatalf("record count = %d, want 2", got)
	}
}

func openGORMLifecycleTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := storage.OpenGORM(storage.Config{
		Driver: "sqlite",
		DSN:    filepath.Join(t.TempDir(), "lifecycle-history.db"),
	})
	if err != nil {
		t.Fatalf("OpenGORM(sqlite) error = %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}
