package bootstrap

import (
	"context"
	"database/sql"
	"errors"

	"platform-go/internal/platform/adminresource"
	"platform-go/internal/platform/capability"
	"platform-go/internal/platform/config"
	"platform-go/internal/platform/dataprotection"
	"platform-go/internal/platform/organizationrbac"
	"platform-go/internal/platform/storage"
)

func AdminResourcesFromConfig(cfg config.Config, manifests []capability.Manifest, protection dataprotection.Runtime) (*adminresource.Store, error) {
	if cfg.AdminResourceDriver != "" {
		if cfg.AdminResourceDSN == "" {
			return nil, errors.New("admin resource dsn is required")
		}
		if isGORMAdminResourceDriver(cfg.AdminResourceDriver) {
			db, err := storage.OpenGORM(storage.Config{Driver: cfg.AdminResourceDriver, DSN: cfg.AdminResourceDSN})
			if err != nil {
				return nil, err
			}
			var repository *adminresource.GORMAdminResourceRepository
			if cfg.OrganizationRBACMode == config.OrganizationRBACModeTarget {
				repository, err = adminresource.OpenGORMAdminResourceRepository(context.Background(), db)
				if err == nil {
					repository = repository.
						WithOrganizationRBACOwnership(organizationrbac.NewAdminUserSnapshotWriter()).
						WithOrganizationRBACOrgUnitWriter(organizationrbac.NewAdminOrgUnitSnapshotWriter()).
						WithOrganizationRBACRoleWriters(organizationrbac.NewAdminRoleSnapshotWriter(), organizationrbac.NewAdminRoleSnapshotWriter())
				}
			} else {
				repository, err = adminresource.NewGORMAdminResourceRepository(context.Background(), db)
			}
			if err != nil {
				sqlDB, dbErr := db.DB()
				if dbErr == nil {
					_ = sqlDB.Close()
				}
				return nil, err
			}
			store, err := adminresource.NewRepositoryBackedStoreFromCapabilitiesWithProtection(repository, manifests, protection)
			if err != nil {
				return nil, err
			}
			if cfg.OrganizationRBACMode == config.OrganizationRBACModeTarget {
				store.EnableOrganizationRBACRoleGovernanceWrites()
			}
			return store, nil
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
		return adminresource.NewRepositoryBackedStoreFromCapabilitiesWithProtection(repository, manifests, protection)
	}
	if cfg.AdminResourceFile == "" {
		return adminresource.NewStoreFromCapabilitiesWithProtection(manifests, protection)
	}
	return adminresource.NewRepositoryBackedStoreFromCapabilitiesWithProtection(adminresource.NewFileAdminResourceRepository(cfg.AdminResourceFile), manifests, protection)
}

func isGORMAdminResourceDriver(driver string) bool {
	switch driver {
	case "mysql", "postgres", "sqlite":
		return true
	default:
		return false
	}
}
