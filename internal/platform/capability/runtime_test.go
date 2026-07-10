package capability

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

func TestRuntimeDefaultExecutorsRunLifecycleSteps(t *testing.T) {
	var calls []string
	runtime := Runtime{}

	err := runtime.RunMigration(context.Background(), MigrationExecution{
		CapabilityID: "identity",
		Migration: Migration{
			ID:          "001_identity",
			Description: "create identity tables",
			Up:          appendCall(&calls, "migration"),
		},
	})
	if err != nil {
		t.Fatalf("RunMigration() error = %v", err)
	}
	err = runtime.RunSeed(context.Background(), SeedExecution{
		CapabilityID: "identity",
		Seed: Seed{
			ID:          "identity-default-users",
			Description: "seed default users",
			Run:         appendCall(&calls, "seed"),
		},
	})
	if err != nil {
		t.Fatalf("RunSeed() error = %v", err)
	}
	if !reflect.DeepEqual(calls, []string{"migration", "seed"}) {
		t.Fatalf("calls = %#v, want migration then seed", calls)
	}
}

func TestRuntimeUsesInjectedMigrationAndSeedExecutors(t *testing.T) {
	var calls []string
	var runtime Runtime
	runtime = Runtime{
		MigrationExecutor: MigrationExecutorFunc(func(ctx context.Context, exec MigrationExecution) error {
			calls = append(calls, string(exec.CapabilityID)+":"+exec.Migration.ID)
			return exec.Migration.Up(ctx, runtime)
		}),
		SeedExecutor: SeedExecutorFunc(func(ctx context.Context, exec SeedExecution) error {
			calls = append(calls, string(exec.CapabilityID)+":"+exec.Seed.ID)
			return exec.Seed.Run(ctx, runtime)
		}),
	}

	err := runtime.RunMigration(context.Background(), MigrationExecution{
		CapabilityID: "files",
		Migration:    Migration{ID: "001_files", Description: "create file tables", Up: appendCall(&calls, "migration.step")},
	})
	if err != nil {
		t.Fatalf("RunMigration() error = %v", err)
	}
	err = runtime.RunSeed(context.Background(), SeedExecution{
		CapabilityID: "files",
		Seed:         Seed{ID: "files-default-settings", Description: "seed file settings", Run: appendCall(&calls, "seed.step")},
	})
	if err != nil {
		t.Fatalf("RunSeed() error = %v", err)
	}

	want := []string{"files:001_files", "migration.step", "files:files-default-settings", "seed.step"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
}

func TestRunLifecycleUsesRuntimeExecutors(t *testing.T) {
	registry := NewRegistry()
	var calls []string
	var runtime Runtime
	runtime = Runtime{
		MigrationExecutor: MigrationExecutorFunc(func(ctx context.Context, exec MigrationExecution) error {
			calls = append(calls, "executor.migration:"+string(exec.CapabilityID)+":"+exec.Migration.ID)
			return exec.Migration.Up(ctx, runtime)
		}),
		SeedExecutor: SeedExecutorFunc(func(ctx context.Context, exec SeedExecution) error {
			calls = append(calls, "executor.seed:"+string(exec.CapabilityID)+":"+exec.Seed.ID)
			return exec.Seed.Run(ctx, runtime)
		}),
	}
	identity := testManifest("identity")
	identity.Migrations = []Migration{{ID: "001-identity", Description: "create identity tables", Up: appendCall(&calls, "identity.migration")}}
	identity.Seeds = []Seed{{ID: "identity-default-users", Description: "seed default users", Run: appendCall(&calls, "identity.seed")}}
	mustRegister(t, registry, identity)

	err := registry.RunLifecycle(context.Background(), []ID{"identity"}, runtime)
	if err != nil {
		t.Fatalf("RunLifecycle() error = %v", err)
	}

	want := []string{
		"executor.migration:identity:001-identity",
		"identity.migration",
		"executor.seed:identity:identity-default-users",
		"identity.seed",
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
}

func TestRunLifecycleWrapsExecutorFailure(t *testing.T) {
	registry := NewRegistry()
	runtime := Runtime{
		MigrationExecutor: MigrationExecutorFunc(func(context.Context, MigrationExecution) error {
			return errRuntimeTestFailure
		}),
	}
	files := testManifest("files")
	files.Migrations = []Migration{{ID: "001-files", Description: "create file tables", Up: noopLifecycleStep}}
	mustRegister(t, registry, files)

	err := registry.RunLifecycle(context.Background(), []ID{"files"}, runtime)
	if err == nil {
		t.Fatalf("RunLifecycle() error = nil, want executor failure")
	}
	if !strings.Contains(err.Error(), `capability "files" migration "001-files" failed`) {
		t.Fatalf("RunLifecycle() error = %v, want wrapped migration failure", err)
	}
}

var errRuntimeTestFailure = runtimeTestError{}

type runtimeTestError struct{}

func (runtimeTestError) Error() string {
	return "runtime test failure"
}
