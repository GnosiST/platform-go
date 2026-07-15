package bootstrap

import (
	"context"
	"errors"

	"platform-go/internal/platform/config"
	"platform-go/internal/platform/kernel"
	"platform-go/internal/platform/organizationrbac"
	"platform-go/internal/platform/serviceobject"
	"platform-go/internal/platform/storage"

	"gorm.io/gorm"
)

type OrganizationRBAC struct {
	Repository     *organizationrbac.GORMRepository
	ServiceObjects *serviceobject.Runtime
	close          func() error
}

type OrganizationRBACMigration struct {
	Repository *organizationrbac.GORMRepository
	close      func() error
}

func PrepareOrganizationRBAC(ctx context.Context, cfg config.Config) error {
	db, closeDB, err := openOrganizationRBACDB(cfg)
	if err != nil {
		return err
	}
	defer func() { _ = closeDB() }()
	if _, err := organizationrbac.PrepareGORMRepository(ctx, db); err != nil {
		return err
	}
	_, err = serviceobject.NewGORMIdempotencyStore(ctx, db, serviceobject.GORMIdempotencyStoreOptions{})
	return err
}

func OpenOrganizationRBAC(ctx context.Context, cfg config.Config) (*OrganizationRBAC, error) {
	if cfg.OrganizationRBACMode != config.OrganizationRBACModeTarget {
		return nil, errors.New("organization rbac target mode is required")
	}
	db, closeDB, err := openOrganizationRBACDB(cfg)
	if err != nil {
		return nil, err
	}
	repository, err := organizationrbac.OpenGORMRepository(ctx, db)
	if err != nil {
		_ = closeDB()
		return nil, err
	}
	if _, err := repository.ValidateCutover(ctx); err != nil {
		_ = closeDB()
		return nil, err
	}
	idempotency, err := serviceobject.OpenGORMIdempotencyStore(ctx, db, serviceobject.GORMIdempotencyStoreOptions{})
	if err != nil {
		_ = closeDB()
		return nil, err
	}
	executor, err := organizationrbac.NewServiceObjectExecutor(repository, nil)
	if err != nil {
		_ = closeDB()
		return nil, err
	}
	registry, err := serviceobject.NewRegistryWithDomainCommands(
		organizationrbac.OrganizationQueryDefinitions(), nil, organizationrbac.OrganizationDomainCommandDefinitions(),
	)
	if err != nil {
		_ = closeDB()
		return nil, err
	}
	runtime, err := serviceobject.NewRuntimeWithDomainCommands(
		registry,
		serviceobject.AuthorizerFunc(func(context.Context, kernel.ExecutionContext, string, string) bool { return false }),
		executor, nil, executor, idempotency,
	)
	if err != nil {
		_ = closeDB()
		return nil, err
	}
	return &OrganizationRBAC{Repository: repository, ServiceObjects: runtime, close: closeDB}, nil
}

func OpenOrganizationRBACMigration(ctx context.Context, cfg config.Config) (*OrganizationRBACMigration, error) {
	db, closeDB, err := openOrganizationRBACDB(cfg)
	if err != nil {
		return nil, err
	}
	repository, err := organizationrbac.OpenGORMRepository(ctx, db)
	if err != nil {
		_ = closeDB()
		return nil, err
	}
	return &OrganizationRBACMigration{Repository: repository, close: closeDB}, nil
}

func (r *OrganizationRBAC) Close() error {
	if r == nil || r.close == nil {
		return nil
	}
	return r.close()
}

func (r *OrganizationRBACMigration) Close() error {
	if r == nil || r.close == nil {
		return nil
	}
	return r.close()
}

func openOrganizationRBACDB(cfg config.Config) (*gorm.DB, func() error, error) {
	if !isGORMAdminResourceDriver(cfg.AdminResourceDriver) || cfg.AdminResourceDSN == "" || cfg.AdminResourceFile != "" {
		return nil, nil, errors.New("organization rbac requires persistent GORM Admin resource storage")
	}
	db, err := storage.OpenGORM(storage.Config{Driver: cfg.AdminResourceDriver, DSN: cfg.AdminResourceDSN})
	if err != nil {
		return nil, nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, nil, err
	}
	return db, sqlDB.Close, nil
}
