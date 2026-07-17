package bootstrap

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/GnosiST/platform-go/internal/platform/adminresource"
	"github.com/GnosiST/platform-go/internal/platform/config"
	"github.com/GnosiST/platform-go/internal/platform/core"
	"github.com/GnosiST/platform-go/internal/platform/organizationrbac"
	"github.com/GnosiST/platform-go/internal/platform/storage"
)

func TestBootstrapDevelopmentOrganizationRBAC(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC)
	cfg := developmentOrganizationRBACConfig(t)

	report, err := BootstrapDevelopmentOrganizationRBAC(ctx, cfg, core.DefaultManifests(), nil, now)
	if err != nil {
		t.Fatalf("BootstrapDevelopmentOrganizationRBAC() error = %v", err)
	}
	if !report.SeedMaterialized || report.SeedRevision == 0 || report.MigrationRunID == "" ||
		report.MigrationStatus != "applied" || report.PromotionPhase != organizationrbac.PromotionTargetWrite {
		t.Fatalf("bootstrap report = %+v", report)
	}
	assertDevelopmentOrganizationRBACRuntime(t, cfg)

	replayed, err := BootstrapDevelopmentOrganizationRBAC(ctx, cfg, core.DefaultManifests(), nil, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("BootstrapDevelopmentOrganizationRBAC(replay) error = %v", err)
	}
	if replayed.SeedMaterialized || replayed.MigrationRunID != report.MigrationRunID ||
		replayed.MigrationStatus != "applied" || replayed.PromotionPhase != organizationrbac.PromotionTargetWrite {
		t.Fatalf("replayed report = %+v, want idempotent target-write", replayed)
	}
	assertDevelopmentOrganizationRBACRuntime(t, cfg)
}

func TestBootstrapDevelopmentOrganizationRBACRejectsUnsafeInputs(t *testing.T) {
	now := time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name      string
		configure func(*config.Config)
	}{
		{name: "production", configure: func(cfg *config.Config) { cfg.RuntimeEnvironment = config.RuntimeEnvironmentProduction }},
		{name: "test", configure: func(cfg *config.Config) { cfg.RuntimeEnvironment = config.RuntimeEnvironmentTest }},
		{name: "mysql", configure: func(cfg *config.Config) { cfg.AdminResourceDriver = "mysql" }},
		{name: "missing dsn", configure: func(cfg *config.Config) { cfg.AdminResourceDSN = "" }},
		{name: "file store", configure: func(cfg *config.Config) { cfg.AdminResourceFile = filepath.Join(t.TempDir(), "admin.json") }},
		{name: "legacy organization mode", configure: func(cfg *config.Config) { cfg.OrganizationRBACMode = config.OrganizationRBACModeLegacy }},
		{name: "dual-read menu mode", configure: func(cfg *config.Config) { cfg.AdminMenuServingMode = config.AdminMenuServingModeDualRead }},
		{name: "role menu writes disabled", configure: func(cfg *config.Config) { cfg.AdminRoleMenuWriteEnabled = false }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := developmentOrganizationRBACConfig(t)
			tc.configure(&cfg)
			if report, err := BootstrapDevelopmentOrganizationRBAC(context.Background(), cfg, core.DefaultManifests(), nil, now); err == nil {
				t.Fatalf("BootstrapDevelopmentOrganizationRBAC() = %+v, nil error; want rejection", report)
			}
		})
	}
}

func TestBootstrapDevelopmentOrganizationRBACRejectsNonEmptyUnbootstrappedDatabase(t *testing.T) {
	ctx := context.Background()
	cfg := developmentOrganizationRBACConfig(t)
	db, err := storage.OpenGORM(storage.Config{Driver: "sqlite", DSN: cfg.AdminResourceDSN})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	repository, err := adminresource.NewGORMAdminResourceRepository(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Save(ctx, adminresource.ResourceSnapshot{
		NextID: 1,
		Resources: map[string][]adminresource.Record{
			"settings": {{ID: "setting-existing", Code: "existing", Name: "Existing", Status: "enabled", UpdatedAt: "2026-07-17T00:00:00Z"}},
		},
	}); err != nil {
		t.Fatal(err)
	}

	_, err = BootstrapDevelopmentOrganizationRBAC(ctx, cfg, core.DefaultManifests(), nil, time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC))
	if err == nil || !strings.Contains(err.Error(), "empty development database") {
		t.Fatalf("BootstrapDevelopmentOrganizationRBAC(non-empty) error = %v, want empty database rejection", err)
	}
}

func TestBootstrapDevelopmentOrganizationRBACRejectsPartialBootstrapState(t *testing.T) {
	ctx := context.Background()
	cfg := developmentOrganizationRBACConfig(t)
	db, err := storage.OpenGORM(storage.Config{Driver: "sqlite", DSN: cfg.AdminResourceDSN})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	repository, err := adminresource.NewGORMAdminResourceRepository(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	store, err := adminresource.NewRepositoryBackedStoreFromCapabilitiesWithProtection(repository, core.DefaultManifests(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if materialized, err := store.MaterializeCapabilitySeeds(ctx); err != nil || !materialized {
		t.Fatalf("MaterializeCapabilitySeeds() = %v, %v", materialized, err)
	}
	if err := PrepareOrganizationRBAC(ctx, cfg); err != nil {
		t.Fatal(err)
	}

	_, err = BootstrapDevelopmentOrganizationRBAC(ctx, cfg, core.DefaultManifests(), nil, time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC))
	if err == nil || !strings.Contains(err.Error(), "completed bootstrap state") {
		t.Fatalf("BootstrapDevelopmentOrganizationRBAC(partial) error = %v, want completed state rejection", err)
	}
}

func developmentOrganizationRBACConfig(t *testing.T) config.Config {
	t.Helper()
	return config.Config{
		RuntimeEnvironment:        config.RuntimeEnvironmentDevelopment,
		AdminResourceDriver:       "sqlite",
		AdminResourceDSN:          filepath.Join(t.TempDir(), "organization-rbac-development.db"),
		OrganizationRBACMode:      config.OrganizationRBACModeTarget,
		AdminMenuServingMode:      config.AdminMenuServingModeTarget,
		AdminRoleMenuWriteEnabled: true,
	}
}

func assertDevelopmentOrganizationRBACRuntime(t *testing.T, cfg config.Config) {
	t.Helper()
	runtime, err := OpenOrganizationRBAC(context.Background(), cfg)
	if err != nil {
		t.Fatalf("OpenOrganizationRBAC() error = %v", err)
	}
	if _, err := runtime.Repository.ValidateMenuPromotion(context.Background(), config.AdminMenuServingModeTarget, true); err != nil {
		t.Fatalf("ValidateMenuPromotion(target write) error = %v", err)
	}
	if err := runtime.Close(); err != nil {
		t.Fatal(err)
	}
}
