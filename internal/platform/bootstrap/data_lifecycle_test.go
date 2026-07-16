package bootstrap

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/GnosiST/platform-go/internal/platform/adminresource"
	"github.com/GnosiST/platform-go/internal/platform/config"
	"github.com/GnosiST/platform-go/internal/platform/dataprotection"
	"github.com/GnosiST/platform-go/internal/platform/storage"
)

func TestPrepareDataLifecycleCreatesOnlyLifecycleTables(t *testing.T) {
	cfg := dataLifecycleConfigForTest(t, config.RuntimeEnvironmentTest)
	prepareAdminResourceSchema(t, cfg)
	before := sqliteTableNames(t, cfg.AdminResourceDSN)

	if err := PrepareDataLifecycle(context.Background(), cfg); err != nil {
		t.Fatalf("PrepareDataLifecycle() error = %v", err)
	}
	after := sqliteTableNames(t, cfg.AdminResourceDSN)
	added := tableDifference(after, before)
	want := []string{
		"platform_data_lifecycle_checkpoints",
		"platform_data_lifecycle_impact_reports",
		"platform_data_lifecycle_leases",
		"platform_data_lifecycle_promotions",
	}
	if !slices.Equal(added, want) {
		t.Fatalf("PrepareDataLifecycle() added tables = %v, want %v", added, want)
	}
}

func TestOpenDataLifecycleRequiresPreparedSchema(t *testing.T) {
	cfg := dataLifecycleConfigForTest(t, config.RuntimeEnvironmentTest)
	prepareAdminResourceSchema(t, cfg)

	lifecycle, err := OpenDataLifecycle(cfg)
	if lifecycle != nil {
		_ = lifecycle.Close()
		t.Fatalf("OpenDataLifecycle() = %#v, want nil", lifecycle)
	}
	if !errors.Is(err, ErrDataLifecycleStorage) {
		t.Fatalf("OpenDataLifecycle() error = %v, want storage category", err)
	}
	for _, table := range sqliteTableNames(t, cfg.AdminResourceDSN) {
		if strings.HasPrefix(table, "platform_data_lifecycle_") {
			t.Fatalf("OpenDataLifecycle() created lifecycle table %q", table)
		}
	}
}

func TestDataLifecycleRejectsNonPersistentAdminStorage(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  config.Config
	}{
		{name: "memory", cfg: config.Config{}},
		{name: "file", cfg: config.Config{AdminResourceDriver: "sqlite", AdminResourceDSN: filepath.Join(t.TempDir(), "admin.db"), AdminResourceFile: filepath.Join(t.TempDir(), "admin.json")}},
		{name: "legacy sql", cfg: config.Config{AdminResourceDriver: "platform_admin_resource_test", AdminResourceDSN: "lifecycle-test"}},
		{name: "unsupported", cfg: config.Config{AdminResourceDriver: "oracle", AdminResourceDSN: "lifecycle-test"}},
		{name: "missing dsn", cfg: config.Config{AdminResourceDriver: "sqlite"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := PrepareDataLifecycle(context.Background(), tc.cfg); !errors.Is(err, ErrDataLifecycleConfig) {
				t.Fatalf("PrepareDataLifecycle() error = %v, want configuration category", err)
			}
			lifecycle, err := OpenDataLifecycle(tc.cfg)
			if lifecycle != nil {
				_ = lifecycle.Close()
				t.Fatalf("OpenDataLifecycle() = %#v, want nil", lifecycle)
			}
			if !errors.Is(err, ErrDataLifecycleConfig) {
				t.Fatalf("OpenDataLifecycle() error = %v, want configuration category", err)
			}
		})
	}
}

func TestDataLifecycleSQLiteEnvironmentPolicy(t *testing.T) {
	for _, environment := range []string{config.RuntimeEnvironmentDevelopment, config.RuntimeEnvironmentTest} {
		t.Run(environment, func(t *testing.T) {
			cfg := dataLifecycleConfigForTest(t, environment)
			prepareAdminResourceSchema(t, cfg)
			if err := PrepareDataLifecycle(context.Background(), cfg); err != nil {
				t.Fatalf("PrepareDataLifecycle() error = %v", err)
			}
		})
	}
	for _, environment := range []string{config.RuntimeEnvironmentStaging, config.RuntimeEnvironmentProduction} {
		t.Run(environment, func(t *testing.T) {
			cfg := dataLifecycleConfigForTest(t, environment)
			if err := PrepareDataLifecycle(context.Background(), cfg); !errors.Is(err, ErrDataLifecycleConfig) {
				t.Fatalf("PrepareDataLifecycle() error = %v, want configuration category", err)
			}
			lifecycle, err := OpenDataLifecycle(cfg)
			if lifecycle != nil {
				_ = lifecycle.Close()
				t.Fatalf("OpenDataLifecycle() = %#v, want nil", lifecycle)
			}
			if !errors.Is(err, ErrDataLifecycleConfig) {
				t.Fatalf("OpenDataLifecycle() error = %v, want configuration category", err)
			}
		})
	}
}

func TestDataLifecycleCloseIsIdempotent(t *testing.T) {
	cfg := dataLifecycleConfigForTest(t, config.RuntimeEnvironmentTest)
	prepareAdminResourceSchema(t, cfg)
	if err := PrepareDataLifecycle(context.Background(), cfg); err != nil {
		t.Fatalf("PrepareDataLifecycle() error = %v", err)
	}
	lifecycle, err := OpenDataLifecycle(cfg)
	if err != nil {
		t.Fatalf("OpenDataLifecycle() error = %v", err)
	}
	if err := lifecycle.Close(); err != nil {
		t.Fatalf("Close() first error = %v", err)
	}
	if err := lifecycle.Close(); err != nil {
		t.Fatalf("Close() second error = %v", err)
	}
	if err := (*DataLifecycle)(nil).Close(); err != nil {
		t.Fatalf("nil Close() error = %v", err)
	}
}

func dataLifecycleConfigForTest(t *testing.T, environment string) config.Config {
	t.Helper()
	cfg := dataProtectionConfigForTest(environment, dataprotection.ProviderLocalTest)
	cfg.AdminResourceDriver = "sqlite"
	cfg.AdminResourceDSN = filepath.Join(t.TempDir(), "admin.db")
	cfg.Capabilities = []string{"dictionary"}
	cfg.FileStorageDriver = "local"
	cfg.FileStorageLocalDir = t.TempDir()
	return cfg
}

func prepareAdminResourceSchema(t *testing.T, cfg config.Config) {
	t.Helper()
	db, err := storage.OpenGORM(storage.Config{Driver: cfg.AdminResourceDriver, DSN: cfg.AdminResourceDSN})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := adminresource.NewGORMAdminResourceRepository(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}
}

func sqliteTableNames(t *testing.T, dsn string) []string {
	t.Helper()
	db, err := storage.OpenGORM(storage.Config{Driver: "sqlite", DSN: dsn})
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	if err := db.Raw(`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`).Scan(&names).Error; err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}
	return names
}

func tableDifference(after, before []string) []string {
	seen := make(map[string]struct{}, len(before))
	for _, table := range before {
		seen[table] = struct{}{}
	}
	var added []string
	for _, table := range after {
		if _, ok := seen[table]; !ok {
			added = append(added, table)
		}
	}
	sort.Strings(added)
	return added
}
