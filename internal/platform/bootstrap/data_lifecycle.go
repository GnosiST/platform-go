package bootstrap

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/GnosiST/platform-go/internal/platform/adminresource"
	"github.com/GnosiST/platform-go/internal/platform/capability"
	"github.com/GnosiST/platform-go/internal/platform/config"
	"github.com/GnosiST/platform-go/internal/platform/datalifecycle"
	"github.com/GnosiST/platform-go/internal/platform/storage"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	ErrDataLifecycleConfig  = errors.New("data lifecycle configuration invalid")
	ErrDataLifecycleRuntime = errors.New("data lifecycle runtime unavailable")
	ErrDataLifecycleStorage = errors.New("data lifecycle storage unavailable")
)

type DataLifecycle struct {
	db                *gorm.DB
	sqlDB             *sql.DB
	runner            *datalifecycle.Runner
	repository        datalifecycle.Repository
	policy            datalifecycle.PolicySnapshot
	policyFingerprint string
	closeOnce         sync.Once
	closeErr          error
}

func PrepareDataLifecycle(ctx context.Context, cfg config.Config) error {
	db, sqlDB, err := openDataLifecycleDB(cfg)
	if err != nil {
		return err
	}
	defer func() { _ = sqlDB.Close() }()
	if _, err := adminresource.OpenGORMAdminResourceRepository(ctx, db); err != nil {
		return ErrDataLifecycleStorage
	}
	if _, err := datalifecycle.PrepareGORMRepository(ctx, db); err != nil {
		return ErrDataLifecycleStorage
	}
	return nil
}

func OpenDataLifecycle(cfg config.Config, additionalManifests ...capability.Manifest) (*DataLifecycle, error) {
	db, sqlDB, err := openDataLifecycleDB(cfg)
	if err != nil {
		return nil, err
	}
	closeOnError := func() { _ = sqlDB.Close() }
	manifests, err := CapabilitiesFromConfig(cfg, additionalManifests...)
	if err != nil {
		closeOnError()
		return nil, ErrDataLifecycleRuntime
	}
	policy, err := datalifecycle.PolicySnapshotFromManifests(manifests)
	if err != nil {
		closeOnError()
		return nil, ErrDataLifecycleRuntime
	}
	policyFingerprint, err := datalifecycle.PolicyFingerprint(policy)
	if err != nil {
		closeOnError()
		return nil, ErrDataLifecycleRuntime
	}
	protection, err := DataProtectionRuntimeFromConfig(cfg)
	if err != nil {
		closeOnError()
		return nil, ErrDataLifecycleRuntime
	}
	lifecycleRepository, err := datalifecycle.OpenGORMRepository(context.Background(), db)
	if err != nil {
		closeOnError()
		return nil, ErrDataLifecycleStorage
	}
	adminRepository, err := adminresource.OpenGORMAdminResourceRepository(context.Background(), db)
	if err != nil {
		closeOnError()
		return nil, ErrDataLifecycleStorage
	}
	store, err := adminresource.NewRepositoryBackedStoreFromCapabilitiesWithProtection(adminRepository, manifests, protection)
	if err != nil {
		closeOnError()
		return nil, ErrDataLifecycleRuntime
	}
	objectStore, err := FileStorageFromConfig(cfg)
	if err != nil {
		closeOnError()
		return nil, ErrDataLifecycleStorage
	}
	clock := systemClock{}
	planner := datalifecycle.NewAdminResourcePlanner(store, clock)
	fileApplier := datalifecycle.NewFileCleanupApplier(store, lifecycleRepository, map[string]storage.ObjectStore{
		strings.TrimSpace(cfg.FileStorageDriver): objectStore,
	})
	databaseApplier := datalifecycle.NewGORMAdminResourceApplier(db, manifests, protection)
	runner := datalifecycle.NewRunner(lifecycleRepository, planner, datalifecycle.NewRoutedBatchApplier(fileApplier, databaseApplier), clock)
	return &DataLifecycle{
		db: db, sqlDB: sqlDB, runner: runner, repository: lifecycleRepository, policy: policy, policyFingerprint: policyFingerprint,
	}, nil
}

func (l *DataLifecycle) Policy() datalifecycle.PolicySnapshot {
	if l == nil {
		return datalifecycle.PolicySnapshot{}
	}
	return l.policy
}

func (l *DataLifecycle) PolicyFingerprint() string {
	if l == nil {
		return ""
	}
	return l.policyFingerprint
}

func (l *DataLifecycle) Run(ctx context.Context, options datalifecycle.Options) (datalifecycle.Report, error) {
	if l == nil || l.runner == nil {
		return datalifecycle.Report{Status: datalifecycle.StatusFailed}, datalifecycle.ErrInvalidOptions
	}
	options.Policy = l.policy
	options.PolicyFingerprint = l.policyFingerprint
	return l.runner.Run(ctx, options)
}

func (l *DataLifecycle) LoadPromotion(ctx context.Context) (datalifecycle.Promotion, bool, error) {
	if l == nil || l.repository == nil || ctx == nil {
		return datalifecycle.Promotion{}, false, datalifecycle.ErrInvalidOptions
	}
	promotion, found, err := l.repository.LoadPromotion(ctx, datalifecycle.DefaultDatasourceID, l.policyFingerprint)
	if err != nil {
		return datalifecycle.Promotion{}, false, datalifecycle.ErrRepositoryFailed
	}
	return promotion, found, nil
}

func (l *DataLifecycle) Promote(ctx context.Context, request datalifecycle.PromotionRequest) (datalifecycle.Promotion, error) {
	if l == nil || l.runner == nil {
		return datalifecycle.Promotion{}, datalifecycle.ErrInvalidOptions
	}
	request.ProposedPolicy = l.policy
	request.PromotedFingerprint = l.policyFingerprint
	return l.runner.Promote(ctx, request)
}

func (l *DataLifecycle) Close() error {
	if l == nil {
		return nil
	}
	l.closeOnce.Do(func() {
		if l.sqlDB != nil {
			l.closeErr = l.sqlDB.Close()
		}
	})
	return l.closeErr
}

func openDataLifecycleDB(cfg config.Config) (*gorm.DB, *sql.DB, error) {
	driver := strings.TrimSpace(cfg.AdminResourceDriver)
	dsn := strings.TrimSpace(cfg.AdminResourceDSN)
	environment := strings.ToLower(strings.TrimSpace(cfg.RuntimeEnvironment))
	if environment == "" {
		environment = config.RuntimeEnvironmentDevelopment
	}
	if cfg.AdminResourceDriver != driver || cfg.AdminResourceDSN != dsn || strings.TrimSpace(cfg.AdminResourceFile) != "" || dsn == "" ||
		!dataLifecycleGORMDriver(driver) || driver == "sqlite" && !dataLifecycleLocalEnvironment(environment) {
		return nil, nil, ErrDataLifecycleConfig
	}
	db, err := storage.OpenGORM(storage.Config{Driver: driver, DSN: dsn}, &gorm.Config{Logger: logger.Discard})
	if err != nil {
		return nil, nil, ErrDataLifecycleStorage
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, nil, ErrDataLifecycleStorage
	}
	return db, sqlDB, nil
}

func dataLifecycleGORMDriver(driver string) bool {
	switch driver {
	case "mysql", "postgres", "sqlite":
		return true
	default:
		return false
	}
}

func dataLifecycleLocalEnvironment(environment string) bool {
	return environment == config.RuntimeEnvironmentDevelopment || environment == config.RuntimeEnvironmentTest
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }
