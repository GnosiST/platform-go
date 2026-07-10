package capability

import (
	"context"
	"reflect"
	"testing"
)

func TestMemoryLifecycleHistoryRecordsMigrationsAndSeeds(t *testing.T) {
	history := NewMemoryLifecycleHistory()
	ctx := context.Background()

	migration := LifecycleRecord{CapabilityID: "identity", StepID: "001_identity", Description: "create identity tables"}
	seed := LifecycleRecord{CapabilityID: "identity", StepID: "identity-default-users", Description: "seed default users"}

	if history.HasMigration(ctx, migration.CapabilityID, migration.StepID) {
		t.Fatalf("HasMigration() = true before record")
	}
	if err := history.RecordMigration(ctx, migration); err != nil {
		t.Fatalf("RecordMigration() error = %v", err)
	}
	if !history.HasMigration(ctx, migration.CapabilityID, migration.StepID) {
		t.Fatalf("HasMigration() = false after record")
	}
	if err := history.RecordSeed(ctx, seed); err != nil {
		t.Fatalf("RecordSeed() error = %v", err)
	}
	if !history.HasSeed(ctx, seed.CapabilityID, seed.StepID) {
		t.Fatalf("HasSeed() = false after record")
	}

	records := history.Records()
	want := []LifecycleRecord{
		{CapabilityID: "identity", StepID: "001_identity", Description: "create identity tables", Kind: LifecycleKindMigration},
		{CapabilityID: "identity", StepID: "identity-default-users", Description: "seed default users", Kind: LifecycleKindSeed},
	}
	if !reflect.DeepEqual(records, want) {
		t.Fatalf("Records() = %#v, want %#v", records, want)
	}
}

func TestRecordedLifecycleExecutorSkipsAlreadyAppliedSteps(t *testing.T) {
	history := NewMemoryLifecycleHistory()
	var calls []string
	runtime := Runtime{}
	executor := NewRecordedLifecycleExecutor(history)
	runtime.MigrationExecutor = executor
	runtime.SeedExecutor = executor

	migration := MigrationExecution{
		CapabilityID: "identity",
		Migration:    Migration{ID: "001_identity", Description: "create identity tables", Up: appendCall(&calls, "migration")},
	}
	seed := SeedExecution{
		CapabilityID: "identity",
		Seed:         Seed{ID: "identity-default-users", Description: "seed default users", Run: appendCall(&calls, "seed")},
	}

	for i := 0; i < 2; i++ {
		if err := runtime.RunMigration(context.Background(), migration); err != nil {
			t.Fatalf("RunMigration(%d) error = %v", i, err)
		}
		if err := runtime.RunSeed(context.Background(), seed); err != nil {
			t.Fatalf("RunSeed(%d) error = %v", i, err)
		}
	}

	if !reflect.DeepEqual(calls, []string{"migration", "seed"}) {
		t.Fatalf("calls = %#v, want each step once", calls)
	}
	records := history.Records()
	if len(records) != 2 {
		t.Fatalf("record count = %d, want 2 records: %#v", len(records), records)
	}
}

func TestRecordedLifecycleExecutorDoesNotRecordFailedStep(t *testing.T) {
	history := NewMemoryLifecycleHistory()
	runtime := Runtime{}
	executor := NewRecordedLifecycleExecutor(history)
	runtime.MigrationExecutor = executor

	err := runtime.RunMigration(context.Background(), MigrationExecution{
		CapabilityID: "files",
		Migration: Migration{
			ID:          "001_files",
			Description: "create file tables",
			Up: func(context.Context, Runtime) error {
				return errRuntimeTestFailure
			},
		},
	})
	if err == nil {
		t.Fatalf("RunMigration() error = nil, want failure")
	}
	if history.HasMigration(context.Background(), "files", "001_files") {
		t.Fatalf("failed migration was recorded")
	}
}
