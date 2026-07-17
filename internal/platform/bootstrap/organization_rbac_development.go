package bootstrap

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/GnosiST/platform-go/internal/platform/adminresource"
	"github.com/GnosiST/platform-go/internal/platform/capability"
	"github.com/GnosiST/platform-go/internal/platform/config"
	"github.com/GnosiST/platform-go/internal/platform/dataprotection"
	"github.com/GnosiST/platform-go/internal/platform/organizationrbac"
	"github.com/GnosiST/platform-go/internal/platform/storage"
	"gorm.io/gorm"
)

const developmentOrganizationRBACRunID = "development-role-menu-bootstrap"

type DevelopmentOrganizationRBACReport struct {
	SeedMaterialized bool   `json:"seedMaterialized"`
	SeedRevision     uint64 `json:"seedRevision"`
	MigrationRunID   string `json:"migrationRunId"`
	MigrationStatus  string `json:"migrationStatus"`
	PromotionPhase   string `json:"promotionPhase"`
}

func BootstrapDevelopmentOrganizationRBAC(ctx context.Context, cfg config.Config, manifests []capability.Manifest, protection dataprotection.Runtime, observedAt time.Time) (DevelopmentOrganizationRBACReport, error) {
	if err := validateDevelopmentOrganizationRBACConfig(cfg); err != nil {
		return DevelopmentOrganizationRBACReport{}, err
	}
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}
	db, err := storage.OpenGORM(storage.Config{Driver: strings.TrimSpace(cfg.AdminResourceDriver), DSN: strings.TrimSpace(cfg.AdminResourceDSN)})
	if err != nil {
		return DevelopmentOrganizationRBACReport{}, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return DevelopmentOrganizationRBACReport{}, err
	}
	defer func() { _ = sqlDB.Close() }()

	hasTables, err := sqliteHasUserTables(ctx, db)
	if err != nil {
		return DevelopmentOrganizationRBACReport{}, err
	}
	var adminRepository *adminresource.GORMAdminResourceRepository
	seedMaterialized := false
	if hasTables {
		complete, err := developmentOrganizationRBACBootstrapComplete(ctx, db)
		if err != nil {
			return DevelopmentOrganizationRBACReport{}, err
		}
		if !complete {
			return DevelopmentOrganizationRBACReport{}, errors.New("development organization rbac bootstrap requires an empty development database or completed bootstrap state")
		}
		adminRepository, err = adminresource.OpenGORMAdminResourceRepository(ctx, db)
		if err != nil {
			return DevelopmentOrganizationRBACReport{}, err
		}
	} else {
		adminRepository, err = adminresource.NewGORMAdminResourceRepository(ctx, db)
		if err != nil {
			return DevelopmentOrganizationRBACReport{}, err
		}
		store, err := adminresource.NewRepositoryBackedStoreFromCapabilitiesWithProtection(adminRepository, manifests, protection)
		if err != nil {
			return DevelopmentOrganizationRBACReport{}, err
		}
		seedMaterialized, err = store.MaterializeCapabilitySeeds(ctx)
		if err != nil {
			return DevelopmentOrganizationRBACReport{}, err
		}
		if !seedMaterialized {
			return DevelopmentOrganizationRBACReport{}, errors.New("development organization rbac bootstrap requires an empty development database")
		}
		if err := PrepareOrganizationRBAC(ctx, cfg); err != nil {
			return DevelopmentOrganizationRBACReport{}, err
		}
		if _, err := AdminResourcesFromConfig(cfg, manifests, protection); err != nil {
			return DevelopmentOrganizationRBACReport{}, err
		}
	}
	seedSnapshot, err := adminRepository.Load(ctx)
	if err != nil {
		return DevelopmentOrganizationRBACReport{}, err
	}
	repository, err := organizationrbac.OpenGORMRepository(ctx, db)
	if err != nil {
		return DevelopmentOrganizationRBACReport{}, err
	}
	evidence := developmentOrganizationRBACEvidence(observedAt)
	migration, err := repository.RunMigration(ctx, organizationrbac.MigrationApply, defaultDevelopmentOrganizationRBACManifest(), evidence)
	if err != nil {
		return DevelopmentOrganizationRBACReport{}, err
	}
	promotion, err := repository.PromoteMenuWrites(ctx, organizationrbac.MenuWritePromotionRequest{
		RunID: developmentOrganizationRBACRunID, ExpectedPhase: organizationrbac.PromotionTargetRead,
		ActorID: evidence.ActorID, Reason: evidence.Reason, ApprovalRef: evidence.ApprovalRef, ObservedAt: observedAt.UTC(),
	})
	if err != nil {
		return DevelopmentOrganizationRBACReport{}, err
	}
	runtime, err := OpenOrganizationRBAC(ctx, cfg)
	if err != nil {
		return DevelopmentOrganizationRBACReport{}, err
	}
	if err := runtime.Close(); err != nil {
		return DevelopmentOrganizationRBACReport{}, err
	}
	return DevelopmentOrganizationRBACReport{
		SeedMaterialized: seedMaterialized,
		SeedRevision:     seedSnapshot.Revision,
		MigrationRunID:   migration.RunID,
		MigrationStatus:  migration.Status,
		PromotionPhase:   promotion.Phase,
	}, nil
}

func validateDevelopmentOrganizationRBACConfig(cfg config.Config) error {
	if strings.ToLower(strings.TrimSpace(cfg.RuntimeEnvironment)) != config.RuntimeEnvironmentDevelopment {
		return errors.New("development organization rbac bootstrap requires explicit development runtime")
	}
	if strings.TrimSpace(cfg.AdminResourceDriver) != "sqlite" || strings.TrimSpace(cfg.AdminResourceDSN) == "" || strings.TrimSpace(cfg.AdminResourceFile) != "" {
		return errors.New("development organization rbac bootstrap requires SQLite Admin resource storage")
	}
	if strings.TrimSpace(cfg.OrganizationRBACMode) != config.OrganizationRBACModeTarget ||
		strings.TrimSpace(cfg.AdminMenuServingMode) != config.AdminMenuServingModeTarget || !cfg.AdminRoleMenuWriteEnabled {
		return errors.New("development organization rbac bootstrap requires explicit target menu write configuration")
	}
	return nil
}

func sqliteHasUserTables(ctx context.Context, db *gorm.DB) (bool, error) {
	var count int64
	if err := db.WithContext(ctx).Raw("SELECT COUNT(*) FROM sqlite_master WHERE type = ? AND name NOT LIKE ?", "table", "sqlite_%").Scan(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func developmentOrganizationRBACBootstrapComplete(ctx context.Context, db *gorm.DB) (bool, error) {
	repository, err := organizationrbac.OpenGORMRepository(ctx, db)
	if err != nil {
		if errors.Is(err, organizationrbac.ErrRepositoryFailed) {
			return false, nil
		}
		return false, err
	}
	state, err := repository.ValidateMenuPromotion(ctx, config.AdminMenuServingModeTarget, true)
	if err != nil {
		if errors.Is(err, organizationrbac.ErrInvalid) || errors.Is(err, organizationrbac.ErrRepositoryFailed) {
			return false, nil
		}
		return false, err
	}
	if state.Phase != organizationrbac.PromotionTargetWrite {
		return false, nil
	}
	return true, nil
}

func developmentOrganizationRBACEvidence(observedAt time.Time) organizationrbac.MigrationEvidence {
	return organizationrbac.MigrationEvidence{
		RunID: developmentOrganizationRBACRunID, ActorID: "development-bootstrap",
		Reason: "development role menu target bootstrap", ApprovalRef: "development-only",
		BackupURI: "file:///development-organization-rbac-bootstrap", BackupSHA256: strings.Repeat("d", 64),
		CheckpointRef: "development-empty-sqlite-bootstrap", AppliedAt: observedAt.UTC(),
	}
}

func defaultDevelopmentOrganizationRBACManifest() organizationrbac.MigrationManifest {
	return organizationrbac.MigrationManifest{
		Version: "1.0.0",
		RoleGroupScopeTenantMap: map[string]organizationrbac.RoleGroupPlacement{
			"system-admin": {ScopeType: organizationrbac.ScopePlatform},
			"operations":   {ScopeType: organizationrbac.ScopePlatform},
		},
		OrphanRoleGroupMap:              map[string]string{},
		TenantUserOrganizationMap:       map[string]string{},
		OrganizationRoleGroupBindingMap: map[string][]string{},
		PlatformPrincipalAllowlist:      []string{"admin", "ops"},
		RolePoolConflictRemediations:    []organizationrbac.RoleAssignmentRemediation{},
	}
}
