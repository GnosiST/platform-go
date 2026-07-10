package capability

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFileLifecycleHistoryPersistsRecordsAcrossInstances(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lifecycle-history.json")
	ctx := context.Background()

	first, err := NewFileLifecycleHistory(path)
	if err != nil {
		t.Fatalf("NewFileLifecycleHistory() error = %v", err)
	}
	if err := first.RecordMigration(ctx, LifecycleRecord{CapabilityID: "identity", StepID: "001_identity", Description: "create identity tables"}); err != nil {
		t.Fatalf("RecordMigration() error = %v", err)
	}
	if err := first.RecordSeed(ctx, LifecycleRecord{CapabilityID: "identity", StepID: "identity-default-users", Description: "seed default users"}); err != nil {
		t.Fatalf("RecordSeed() error = %v", err)
	}

	second, err := NewFileLifecycleHistory(path)
	if err != nil {
		t.Fatalf("NewFileLifecycleHistory(second) error = %v", err)
	}
	if !second.HasMigration(ctx, "identity", "001_identity") {
		t.Fatalf("HasMigration() = false after reopening file history")
	}
	if !second.HasSeed(ctx, "identity", "identity-default-users") {
		t.Fatalf("HasSeed() = false after reopening file history")
	}

	want := []LifecycleRecord{
		{CapabilityID: "identity", StepID: "001_identity", Description: "create identity tables", Kind: LifecycleKindMigration},
		{CapabilityID: "identity", StepID: "identity-default-users", Description: "seed default users", Kind: LifecycleKindSeed},
	}
	if !reflect.DeepEqual(second.Records(), want) {
		t.Fatalf("Records() = %#v, want %#v", second.Records(), want)
	}
}

func TestFileLifecycleHistoryDoesNotDuplicateRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lifecycle-history.json")
	history, err := NewFileLifecycleHistory(path)
	if err != nil {
		t.Fatalf("NewFileLifecycleHistory() error = %v", err)
	}
	record := LifecycleRecord{CapabilityID: "files", StepID: "001_files", Description: "create file tables"}

	if err := history.RecordMigration(context.Background(), record); err != nil {
		t.Fatalf("RecordMigration(first) error = %v", err)
	}
	if err := history.RecordMigration(context.Background(), record); err != nil {
		t.Fatalf("RecordMigration(second) error = %v", err)
	}
	if got := len(history.Records()); got != 1 {
		t.Fatalf("record count = %d, want 1", got)
	}
}

func TestRecordedLifecycleExecutorSkipsStepsWithFileHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lifecycle-history.json")
	history, err := NewFileLifecycleHistory(path)
	if err != nil {
		t.Fatalf("NewFileLifecycleHistory() error = %v", err)
	}
	executor := NewRecordedLifecycleExecutor(history)
	runtime := Runtime{MigrationExecutor: executor, SeedExecutor: executor}
	var calls []string

	for i := 0; i < 2; i++ {
		if err := runtime.RunMigration(context.Background(), MigrationExecution{
			CapabilityID: "dictionary",
			Migration:    Migration{ID: "001_dictionary", Description: "create dictionary tables", Up: appendCall(&calls, "migration")},
		}); err != nil {
			t.Fatalf("RunMigration(%d) error = %v", i, err)
		}
		if err := runtime.RunSeed(context.Background(), SeedExecution{
			CapabilityID: "dictionary",
			Seed:         Seed{ID: "dictionary-default-values", Description: "seed dictionary values", Run: appendCall(&calls, "seed")},
		}); err != nil {
			t.Fatalf("RunSeed(%d) error = %v", i, err)
		}
	}

	if !reflect.DeepEqual(calls, []string{"migration", "seed"}) {
		t.Fatalf("calls = %#v, want file-backed history to run each step once", calls)
	}
	reopened, err := NewFileLifecycleHistory(path)
	if err != nil {
		t.Fatalf("NewFileLifecycleHistory(reopened) error = %v", err)
	}
	if got := len(reopened.Records()); got != 2 {
		t.Fatalf("persisted record count = %d, want 2", got)
	}
}
