package bootstrap

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"platform-go/internal/platform/capability"
	"platform-go/internal/platform/config"
)

func TestRuntimeFromConfigUsesZeroValueRuntimeWithoutHistoryFile(t *testing.T) {
	runtime, err := RuntimeFromConfig(config.Config{})
	if err != nil {
		t.Fatalf("RuntimeFromConfig() error = %v", err)
	}
	var calls []string
	if err := runtime.RunMigration(context.Background(), capability.MigrationExecution{
		CapabilityID: "identity",
		Migration:    capability.Migration{ID: "001_identity", Description: "create identity tables", Up: appendCall(&calls, "migration")},
	}); err != nil {
		t.Fatalf("RunMigration() error = %v", err)
	}
	if !reflect.DeepEqual(calls, []string{"migration"}) {
		t.Fatalf("calls = %#v, want direct runtime execution", calls)
	}
}

func TestRuntimeFromConfigUsesFileLifecycleHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lifecycle-history.json")
	cfg := config.Config{LifecycleHistoryFile: path}
	var calls []string

	first, err := RuntimeFromConfig(cfg)
	if err != nil {
		t.Fatalf("RuntimeFromConfig(first) error = %v", err)
	}
	runMigration(t, first, &calls)

	second, err := RuntimeFromConfig(cfg)
	if err != nil {
		t.Fatalf("RuntimeFromConfig(second) error = %v", err)
	}
	runMigration(t, second, &calls)

	if !reflect.DeepEqual(calls, []string{"migration"}) {
		t.Fatalf("calls = %#v, want second runtime to skip recorded migration", calls)
	}
}

func TestRuntimeFromConfigUsesGORMLifecycleHistory(t *testing.T) {
	cfg := config.Config{
		LifecycleHistoryDriver: "sqlite",
		LifecycleHistoryDSN:    filepath.Join(t.TempDir(), "bootstrap-lifecycle.db"),
	}
	var calls []string

	first, err := RuntimeFromConfig(cfg)
	if err != nil {
		t.Fatalf("RuntimeFromConfig(first) error = %v", err)
	}
	runMigration(t, first, &calls)

	second, err := RuntimeFromConfig(cfg)
	if err != nil {
		t.Fatalf("RuntimeFromConfig(second) error = %v", err)
	}
	runMigration(t, second, &calls)

	if !reflect.DeepEqual(calls, []string{"migration"}) {
		t.Fatalf("calls = %#v, want GORM-backed runtime to skip recorded migration", calls)
	}
}

func TestRuntimeFromConfigRejectsGORMHistoryWithoutDSN(t *testing.T) {
	_, err := RuntimeFromConfig(config.Config{LifecycleHistoryDriver: "sqlite"})
	if err == nil {
		t.Fatalf("RuntimeFromConfig() error = nil, want missing DSN")
	}
	if !strings.Contains(err.Error(), "lifecycle history dsn is required") {
		t.Fatalf("RuntimeFromConfig() error = %v, want missing DSN", err)
	}
}

func runMigration(t *testing.T, runtime capability.Runtime, calls *[]string) {
	t.Helper()
	if err := runtime.RunMigration(context.Background(), capability.MigrationExecution{
		CapabilityID: "identity",
		Migration:    capability.Migration{ID: "001_identity", Description: "create identity tables", Up: appendCall(calls, "migration")},
	}); err != nil {
		t.Fatalf("RunMigration() error = %v", err)
	}
}

func appendCall(calls *[]string, value string) capability.LifecycleStep {
	return func(context.Context, capability.Runtime) error {
		*calls = append(*calls, value)
		return nil
	}
}
