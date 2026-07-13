package bootstrap

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"platform-go/internal/platform/capability"
	"platform-go/internal/platform/config"
	"platform-go/internal/platform/dataprotection"
	"platform-go/internal/platform/sensitivemigration"
	"platform-go/internal/platform/storage"
)

func TestSensitiveDataMigrationRejectsNonGORMAndIncompleteStorage(t *testing.T) {
	secretDSN := "secret-user:secret-password@tcp(database.internal)/platform"
	for _, tc := range []struct {
		name string
		cfg  config.Config
	}{
		{name: "memory", cfg: config.Config{}},
		{name: "file", cfg: config.Config{AdminResourceFile: filepath.Join(t.TempDir(), "resources.json")}},
		{name: "legacy sql", cfg: config.Config{AdminResourceDriver: "platform_admin_resource_test", AdminResourceDSN: secretDSN}},
		{name: "unsupported", cfg: config.Config{AdminResourceDriver: "oracle", AdminResourceDSN: secretDSN}},
		{name: "missing dsn", cfg: config.Config{AdminResourceDriver: "sqlite"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			migration, err := OpenSensitiveDataMigration(tc.cfg)
			if err == nil {
				if migration != nil {
					_ = migration.Close()
				}
				t.Fatal("OpenSensitiveDataMigration() error = nil")
			}
			if migration != nil {
				t.Fatalf("OpenSensitiveDataMigration() = %#v, want nil", migration)
			}
			if strings.Contains(err.Error(), secretDSN) || strings.Contains(err.Error(), "secret-password") {
				t.Fatalf("OpenSensitiveDataMigration() error exposed DSN: %q", err)
			}
			if !errors.Is(err, ErrSensitiveDataMigrationConfig) {
				t.Fatalf("OpenSensitiveDataMigration() error = %v, want configuration category", err)
			}
		})
	}
}

func TestSensitiveDataMigrationBuildsManifestPlanAndReadOnlyModesDoNotCreateJournal(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "sensitive-migration.db")
	db, err := storage.OpenGORM(storage.Config{Driver: "sqlite", DSN: dsn})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE platform_admin_resource_records (resource TEXT NOT NULL, id TEXT NOT NULL PRIMARY KEY, values_json TEXT NOT NULL)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO platform_admin_resource_records(resource, id, values_json) VALUES (?, ?, ?)`, "protected-bootstrap-records", "record-1", `{"governmentReference":"historical-plain-value"}`).Error; err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}

	cfg := dataProtectionConfigForTest(config.RuntimeEnvironmentTest, dataprotection.ProviderLocalTest)
	cfg.AdminResourceDriver = "sqlite"
	cfg.AdminResourceDSN = dsn
	cfg.Capabilities = []string{"protected-bootstrap-test"}
	manifest := sensitiveMigrationTestManifest()
	migration, err := OpenSensitiveDataMigration(cfg, manifest)
	if err != nil {
		t.Fatalf("OpenSensitiveDataMigration() error = %v", err)
	}
	if migration.PlanHash() == "" {
		t.Fatal("PlanHash() is empty")
	}

	for _, mode := range []sensitivemigration.Mode{sensitivemigration.ModeInventory, sensitivemigration.ModeDryRun} {
		report, runErr := migration.Runner().Run(context.Background(), sensitivemigration.Options{Mode: mode, BatchSize: 1})
		if runErr != nil {
			t.Fatalf("Run(%s) error = %v", mode, runErr)
		}
		if report.Status != sensitivemigration.StatusCompleted || report.Counts.Plaintext != 1 {
			t.Fatalf("Run(%s) report = %+v", mode, report)
		}
	}
	_, verifyErr := migration.Runner().Run(context.Background(), sensitivemigration.Options{
		Mode: sensitivemigration.ModeVerify,
		Request: sensitivemigration.RunRequest{
			RunID: "run-verify-1", PlanHash: migration.PlanHash(),
		},
	})
	if verifyErr == nil {
		t.Fatal("verify without prepared journal error = nil")
	}

	var journalTables int64
	if err := migration.db.Raw(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name LIKE 'platform_sensitive_migration_%'`).Scan(&journalTables).Error; err != nil {
		t.Fatal(err)
	}
	if journalTables != 0 {
		t.Fatalf("journal table count = %d, want 0", journalTables)
	}
	if err := migration.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestSensitiveDataMigrationSilencesStorageInitializationErrors(t *testing.T) {
	const (
		helperEnvironment = "PLATFORM_TEST_SENSITIVE_MIGRATION_OPEN_FAILURE"
		secretMarker      = "sensitive-bootstrap-dsn-marker"
	)
	if os.Getenv(helperEnvironment) == "1" {
		cfg := dataProtectionConfigForTest(config.RuntimeEnvironmentTest, dataprotection.ProviderLocalTest)
		cfg.AdminResourceDriver = "postgres"
		cfg.AdminResourceDSN = "postgres://" + secretMarker + ":password@127.0.0.1:1/platform?sslmode=disable"
		cfg.Capabilities = []string{"protected-bootstrap-test"}
		migration, err := OpenSensitiveDataMigration(cfg, sensitiveMigrationTestManifest())
		if migration != nil {
			_ = migration.Close()
			t.Fatal("OpenSensitiveDataMigration() returned a session for an invalid DSN")
		}
		if !errors.Is(err, ErrSensitiveDataMigrationStorage) {
			t.Fatalf("OpenSensitiveDataMigration() error = %v, want storage category", err)
		}
		if strings.Contains(err.Error(), secretMarker) {
			t.Fatalf("OpenSensitiveDataMigration() error exposed DSN marker: %q", err)
		}
		return
	}

	command := exec.Command(os.Args[0], "-test.run=^TestSensitiveDataMigrationSilencesStorageInitializationErrors$")
	command.Env = append(os.Environ(), helperEnvironment+"=1")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		t.Fatalf("helper test error = %v, stdout=%q stderr=%q", err, stdout.String(), stderr.String())
	}
	childOutput := strings.TrimPrefix(stdout.String(), "PASS\n")
	if childOutput != "" || stderr.Len() != 0 || strings.Contains(stdout.String()+stderr.String(), secretMarker) {
		t.Fatalf("bootstrap output stdout=%q stderr=%q, want silent initialization failure", stdout.String(), stderr.String())
	}
}

func TestSensitiveDataMigrationSQLiteEnvironmentPolicy(t *testing.T) {
	manifest := sensitiveMigrationTestManifest()
	for _, environment := range []string{config.RuntimeEnvironmentDevelopment, config.RuntimeEnvironmentTest} {
		t.Run(environment, func(t *testing.T) {
			cfg := dataProtectionConfigForTest(environment, dataprotection.ProviderLocalTest)
			cfg.AdminResourceDriver = "sqlite"
			cfg.AdminResourceDSN = filepath.Join(t.TempDir(), "sensitive-migration.db")
			cfg.Capabilities = []string{"protected-bootstrap-test"}
			migration, err := OpenSensitiveDataMigration(cfg, manifest)
			if err != nil {
				t.Fatalf("OpenSensitiveDataMigration() error = %v", err)
			}
			if err := migration.Close(); err != nil {
				t.Fatalf("Close() error = %v", err)
			}
		})
	}

	for _, environment := range []string{config.RuntimeEnvironmentStaging, config.RuntimeEnvironmentProduction} {
		t.Run(environment, func(t *testing.T) {
			cfg := dataProtectionConfigForTest(environment, dataprotection.ProviderEnvAES256)
			cfg.AdminResourceDriver = "sqlite"
			cfg.AdminResourceDSN = filepath.Join(t.TempDir(), "sensitive-migration.db")
			cfg.Capabilities = []string{"protected-bootstrap-test"}
			migration, err := OpenSensitiveDataMigration(cfg, manifest)
			if migration != nil {
				_ = migration.Close()
				t.Fatalf("OpenSensitiveDataMigration() returned a %s SQLite session", environment)
			}
			if !errors.Is(err, ErrSensitiveDataMigrationConfig) {
				t.Fatalf("OpenSensitiveDataMigration() error = %v, want configuration category", err)
			}
		})
	}
}

func sensitiveMigrationTestManifest() capability.Manifest {
	manifest := bootstrapProtectedManifests()[len(bootstrapProtectedManifests())-1]
	manifest.Name = "Protected Bootstrap Test"
	manifest.Version = "0.1.0"
	return manifest
}
