package bootstrap

import (
	"context"
	"database/sql"
	"errors"

	"platform-go/internal/platform/adminresource"
	"platform-go/internal/platform/capability"
	"platform-go/internal/platform/config"
	"platform-go/internal/platform/storage"
)

func AdminResourcesFromConfig(cfg config.Config, manifests []capability.Manifest) (*adminresource.Store, error) {
	if cfg.AdminResourceDriver != "" {
		if cfg.AdminResourceDSN == "" {
			return nil, errors.New("admin resource dsn is required")
		}
		if isGORMAdminResourceDriver(cfg.AdminResourceDriver) {
			db, err := storage.OpenGORM(storage.Config{Driver: cfg.AdminResourceDriver, DSN: cfg.AdminResourceDSN})
			if err != nil {
				return nil, err
			}
			repository, err := adminresource.NewGORMAdminResourceRepository(context.Background(), db)
			if err != nil {
				sqlDB, dbErr := db.DB()
				if dbErr == nil {
					_ = sqlDB.Close()
				}
				return nil, err
			}
			return adminresource.NewRepositoryBackedStoreFromCapabilities(repository, manifests)
		}
		db, err := sql.Open(cfg.AdminResourceDriver, cfg.AdminResourceDSN)
		if err != nil {
			return nil, err
		}
		repository, err := adminresource.NewSQLAdminResourceRepository(context.Background(), db)
		if err != nil {
			_ = db.Close()
			return nil, err
		}
		return adminresource.NewRepositoryBackedStoreFromCapabilities(repository, manifests)
	}
	if cfg.AdminResourceFile == "" {
		return adminresource.NewStoreFromCapabilities(manifests), nil
	}
	return adminresource.NewFileBackedStoreFromCapabilities(cfg.AdminResourceFile, manifests)
}

func isGORMAdminResourceDriver(driver string) bool {
	switch driver {
	case "mysql", "postgres", "sqlite":
		return true
	default:
		return false
	}
}
