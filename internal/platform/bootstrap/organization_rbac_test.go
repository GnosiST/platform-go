package bootstrap

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"platform-go/internal/platform/adminresource"
	"platform-go/internal/platform/config"
	"platform-go/internal/platform/core"
	"platform-go/internal/platform/organizationrbac"
	"platform-go/internal/platform/storage"
)

func TestPrepareAndOpenOrganizationRBAC(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "organization-rbac.db")
	cfg := config.Config{
		AdminResourceDriver: "sqlite", AdminResourceDSN: dsn,
		OrganizationRBACMode: config.OrganizationRBACModeTarget,
	}
	db, err := storage.OpenGORM(storage.Config{Driver: "sqlite", DSN: dsn})
	if err != nil {
		t.Fatal(err)
	}
	repository, err := adminresource.NewGORMAdminResourceRepository(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := adminresource.NewRepositoryBackedStoreFromCapabilities(repository, core.DefaultManifests()); err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	_ = sqlDB.Close()

	if _, err := OpenOrganizationRBAC(context.Background(), cfg); err == nil {
		t.Fatal("OpenOrganizationRBAC(unprepared) error = nil")
	}
	if err := PrepareOrganizationRBAC(context.Background(), cfg); err != nil {
		t.Fatalf("PrepareOrganizationRBAC() error = %v", err)
	}
	db, err = storage.OpenGORM(storage.Config{Driver: "sqlite", DSN: dsn})
	if err != nil {
		t.Fatal(err)
	}
	organizationRepository, err := organizationrbac.OpenGORMRepository(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := organizationRepository.RunMigration(context.Background(), organizationrbac.MigrationApply, defaultPlatformOrganizationRBACManifest(), organizationrbac.MigrationEvidence{
		RunID: "bootstrap-org-rbac", ActorID: "test-admin", Reason: "test cutover", ApprovalRef: "test-approval",
		BackupURI: "file:///test-backup", BackupSHA256: strings.Repeat("a", 64), CheckpointRef: "test-checkpoint", AppliedAt: time.Now(),
	}); err != nil {
		t.Fatalf("RunMigration() error = %v", err)
	}
	sqlDB, _ = db.DB()
	_ = sqlDB.Close()
	runtime, err := OpenOrganizationRBAC(context.Background(), cfg)
	if err != nil {
		t.Fatalf("OpenOrganizationRBAC() error = %v", err)
	}
	if runtime.Repository == nil || runtime.ServiceObjects == nil {
		t.Fatalf("runtime = %+v", runtime)
	}
	if err := runtime.Close(); err != nil {
		t.Fatal(err)
	}
}

func defaultPlatformOrganizationRBACManifest() organizationrbac.MigrationManifest {
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
