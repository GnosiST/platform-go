package bootstrap

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"

	"platform-go/internal/platform/adminresource"
	"platform-go/internal/platform/capability"
	"platform-go/internal/platform/config"
	"platform-go/internal/platform/sensitivemigration"
	"platform-go/internal/platform/storage"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	ErrSensitiveDataMigrationConfig  = errors.New("sensitive data migration configuration invalid")
	ErrSensitiveDataMigrationRuntime = errors.New("sensitive data migration runtime unavailable")
	ErrSensitiveDataMigrationStorage = errors.New("sensitive data migration storage unavailable")
)

type SensitiveDataMigration struct {
	db        *gorm.DB
	sqlDB     *sql.DB
	runner    *sensitivemigration.Runner
	planHash  string
	closeOnce sync.Once
	closeErr  error
}

func OpenSensitiveDataMigration(cfg config.Config, additionalManifests ...capability.Manifest) (*SensitiveDataMigration, error) {
	driver := strings.TrimSpace(cfg.AdminResourceDriver)
	dsn := strings.TrimSpace(cfg.AdminResourceDSN)
	environment := strings.ToLower(strings.TrimSpace(cfg.RuntimeEnvironment))
	if environment == "" {
		environment = config.RuntimeEnvironmentDevelopment
	}
	if cfg.AdminResourceDriver != driver || cfg.AdminResourceDSN != dsn || strings.TrimSpace(cfg.AdminResourceFile) != "" || dsn == "" ||
		!sensitiveMigrationGORMDriver(driver) || driver == "sqlite" && !sensitiveMigrationLocalEnvironment(environment) {
		return nil, ErrSensitiveDataMigrationConfig
	}

	manifests, err := CapabilitiesFromConfig(cfg, additionalManifests...)
	if err != nil {
		return nil, ErrSensitiveDataMigrationRuntime
	}
	protection, err := DataProtectionRuntimeFromConfig(cfg)
	if err != nil || protection == nil {
		return nil, ErrSensitiveDataMigrationRuntime
	}
	plan, err := sensitivemigration.PlanFromManifests(manifests)
	if err != nil {
		return nil, ErrSensitiveDataMigrationRuntime
	}

	db, err := storage.OpenGORM(storage.Config{Driver: driver, DSN: dsn}, &gorm.Config{Logger: logger.Discard})
	if err != nil {
		return nil, ErrSensitiveDataMigrationStorage
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, ErrSensitiveDataMigrationStorage
	}
	store, err := adminresource.NewGORMProtectedValueMigrationStore(db, driver)
	if err != nil {
		_ = sqlDB.Close()
		return nil, ErrSensitiveDataMigrationStorage
	}
	return &SensitiveDataMigration{
		db: db, sqlDB: sqlDB, runner: sensitivemigration.NewRunner(plan, protection, store), planHash: sensitivemigration.PlanHash(plan),
	}, nil
}

func (m *SensitiveDataMigration) Runner() *sensitivemigration.Runner {
	if m == nil {
		return nil
	}
	return m.runner
}

func (m *SensitiveDataMigration) PlanHash() string {
	if m == nil {
		return ""
	}
	return m.planHash
}

func (m *SensitiveDataMigration) Run(ctx context.Context, options sensitivemigration.Options) (sensitivemigration.Report, error) {
	if m == nil || m.runner == nil {
		return sensitivemigration.Report{Status: sensitivemigration.StatusFailed}, sensitivemigration.ErrInvalidOptions
	}
	return m.runner.Run(ctx, options)
}

func (m *SensitiveDataMigration) Close() error {
	if m == nil {
		return nil
	}
	m.closeOnce.Do(func() {
		if m.sqlDB != nil {
			m.closeErr = m.sqlDB.Close()
		}
	})
	return m.closeErr
}

func sensitiveMigrationGORMDriver(driver string) bool {
	switch driver {
	case "mysql", "postgres", "sqlite":
		return true
	default:
		return false
	}
}

func sensitiveMigrationLocalEnvironment(environment string) bool {
	return environment == config.RuntimeEnvironmentDevelopment || environment == config.RuntimeEnvironmentTest
}
