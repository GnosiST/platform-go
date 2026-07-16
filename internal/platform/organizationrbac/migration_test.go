package organizationrbac

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestOrganizationRBACMigrationInventoryPersistsUnresolvedRolePoolConflicts(t *testing.T) {
	db, repository := prepareOrganizationRBACTestRepository(t)
	seedOrganizationRBAC(t, db)
	if err := db.Create(&gormUser{ID: "user-alice", Code: "alice", TenantCode: "legacy-client", Status: StatusEnabled, ValuesJSON: `{}`}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&gormUserRole{UserID: "user-alice", RoleCode: "auditor"}).Error; err != nil {
		t.Fatal(err)
	}
	manifest := testOrganizationRBACMigrationManifest()

	report, err := repository.RunMigration(context.Background(), MigrationInventory, manifest, MigrationEvidence{RunID: "inventory-role-pool-1"})
	if err != nil {
		t.Fatalf("RunMigration(inventory) error = %v", err)
	}
	if report.Status != "inventoried" || len(report.Conflicts) != 1 || report.Conflicts[0].Kind != "role-pool-assignment" || report.Conflicts[0].Code != "alice:auditor" {
		t.Fatalf("inventory report = %+v", report)
	}
	var persisted []gormOrganizationRBACMigrationConflict
	if err := db.Where("run_id = ?", "inventory-role-pool-1").Order("sequence").Find(&persisted).Error; err != nil {
		t.Fatal(err)
	}
	if len(persisted) != 1 || persisted[0].Kind != report.Conflicts[0].Kind || persisted[0].Code != report.Conflicts[0].Code {
		t.Fatalf("persisted conflicts = %+v", persisted)
	}

	manifest.RolePoolConflictRemediations = []RoleAssignmentRemediation{{UserCode: "alice", RoleCode: "auditor", Action: "remove-role"}}
	report, err = repository.RunMigration(context.Background(), MigrationInventory, manifest, MigrationEvidence{RunID: "inventory-role-pool-1"})
	if err != nil {
		t.Fatalf("RunMigration(remediated inventory) error = %v", err)
	}
	if len(report.Conflicts) != 0 {
		t.Fatalf("remediated inventory conflicts = %+v", report.Conflicts)
	}
	var count int64
	if err := db.Model(&gormOrganizationRBACMigrationConflict{}).Where("run_id = ?", "inventory-role-pool-1").Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("persisted remediated conflict count = %d, error = %v", count, err)
	}
	missingBinding := manifest
	missingBinding.OrganizationRoleGroupBindingMap = map[string][]string{}
	missingBindingReport, err := repository.RunMigration(context.Background(), MigrationInventory, missingBinding, MigrationEvidence{RunID: "inventory-missing-binding-1"})
	if err != nil {
		t.Fatalf("RunMigration(missing binding inventory) error = %v", err)
	}
	if !hasMigrationConflictKind(missingBindingReport.Conflicts, "role-pool-remediation") {
		t.Fatalf("missing binding conflicts = %+v", missingBindingReport.Conflicts)
	}
	now := time.Date(2026, 7, 15, 12, 30, 0, 0, time.UTC)
	evidence := MigrationEvidence{
		RunID: "remediated-role-pool-apply-1", ActorID: "migration-admin", Reason: "approved role cleanup", ApprovalRef: "change-remediation-1",
		BackupURI: "s3://backups/remediated-role-pool-apply-1", BackupSHA256: strings.Repeat("c", 64), CheckpointRef: "restore-rehearsal-remediation-1", AppliedAt: now,
	}
	if report, err := repository.RunMigration(context.Background(), MigrationApply, manifest, evidence); !errors.Is(err, ErrRolePoolViolation) || report.Status != "" {
		t.Fatalf("RunMigration(non-equivalent remediation) report = %+v, error = %v", report, err)
	}
	if err := db.Model(&gormUserRole{}).Where("user_id = ? AND role_code = ?", "user-alice", "auditor").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("assignment count after blocked remediation = %d, error = %v", count, err)
	}
	if err := db.Model(&gormOrganizationRBACMigrationRun{}).Where("run_id = ?", evidence.RunID).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("migration run count after blocked remediation = %d, error = %v", count, err)
	}
}

func TestOrganizationRBACMigrationRequiresExplicitMappingsAndAppliesCutover(t *testing.T) {
	db, repository := prepareOrganizationRBACTestRepository(t)
	seedOrganizationRBAC(t, db)
	if err := db.Create(&gormUser{ID: "user-alice", Code: "alice", TenantCode: "legacy-client", OrgUnitCode: "", Status: StatusEnabled, ValuesJSON: `{}`}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&gormUserRole{UserID: "user-alice", RoleCode: "operator"}).Error; err != nil {
		t.Fatal(err)
	}
	manifest := testOrganizationRBACMigrationManifest()
	bad := manifest
	bad.RoleGroupScopeTenantMap = map[string]RoleGroupPlacement{}
	if report, err := repository.RunMigration(context.Background(), MigrationVerify, bad, MigrationEvidence{}); err == nil || len(report.Conflicts) == 0 {
		t.Fatalf("verify missing mappings report = %+v, error = %v", report, err)
	}
	now := time.Date(2026, 7, 15, 13, 0, 0, 0, time.UTC)
	report, err := repository.RunMigration(context.Background(), MigrationApply, manifest, MigrationEvidence{
		RunID: "org-rbac-run-1", ActorID: "migration-admin", Reason: "approved cutover", ApprovalRef: "change-123",
		BackupURI: "s3://backups/org-rbac-run-1", BackupSHA256: strings.Repeat("a", 64), CheckpointRef: "restore-rehearsal-123", AppliedAt: now,
	})
	if err != nil {
		t.Fatalf("RunMigration(apply) error = %v", err)
	}
	if report.Status != "applied" || report.Cutover == nil || report.Cutover.Users != 1 || report.Cutover.Bindings != 1 {
		t.Fatalf("migration report = %+v", report)
	}
	var user gormUser
	if err := db.Where("code = ?", "alice").Take(&user).Error; err != nil {
		t.Fatal(err)
	}
	if user.ScopeType != string(ScopeTenant) || user.TenantCode != "acme" || user.OrgUnitCode != "acme-hq" {
		t.Fatalf("migrated user = %+v", user)
	}
	if _, err := repository.ValidateCutover(context.Background()); err != nil {
		t.Fatalf("ValidateCutover() error = %v", err)
	}
	revision, err := repository.CurrentGlobalRevision(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := repository.RunMigration(context.Background(), MigrationApply, manifest, MigrationEvidence{
		RunID: "org-rbac-run-1", ActorID: "migration-admin", Reason: "approved cutover", ApprovalRef: "change-123",
		BackupURI: "s3://backups/org-rbac-run-1", BackupSHA256: strings.Repeat("a", 64), CheckpointRef: "restore-rehearsal-123", AppliedAt: now.Add(time.Minute),
	})
	if err != nil || repeated.Status != "applied" {
		t.Fatalf("RunMigration(repeated apply) report = %+v, error = %v", repeated, err)
	}
	if after, err := repository.CurrentGlobalRevision(context.Background()); err != nil || after != revision {
		t.Fatalf("global revision after repeated apply = %d, want %d, error = %v", after, revision, err)
	}
	var runCount int64
	if err := db.Model(&gormOrganizationRBACMigrationRun{}).Where("run_id = ?", "org-rbac-run-1").Count(&runCount).Error; err != nil || runCount != 1 {
		t.Fatalf("migration run count = %d, error = %v", runCount, err)
	}

	changedManifest := manifest
	changedManifest.Version = "1.0.1"
	if _, err := repository.RunMigration(context.Background(), MigrationApply, changedManifest, MigrationEvidence{
		RunID: "org-rbac-run-1", ActorID: "migration-admin", Reason: "approved cutover", ApprovalRef: "change-123",
		BackupURI: "s3://backups/org-rbac-run-1", BackupSHA256: strings.Repeat("a", 64), CheckpointRef: "restore-rehearsal-123", AppliedAt: now.Add(2 * time.Minute),
	}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("RunMigration(reused run ID) error = %v, want ErrInvalid", err)
	}
}

func TestOrganizationRBACMigrationReplaysAfterSourceRoleAssignmentWasRemoved(t *testing.T) {
	db, repository := prepareOrganizationRBACTestRepository(t)
	seedOrganizationRBAC(t, db)
	if err := db.Create(&gormUser{ID: "user-alice", Code: "alice", TenantCode: "legacy-client", Status: StatusEnabled, ValuesJSON: `{}`}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&gormUserRole{UserID: "user-alice", RoleCode: "operator"}).Error; err != nil {
		t.Fatal(err)
	}
	manifest := testOrganizationRBACMigrationManifest()
	evidence := MigrationEvidence{
		RunID: "org-rbac-replay-after-remediation-1", ActorID: "migration-admin", Reason: "approved role remediation", ApprovalRef: "change-replay-remediation-1",
		BackupURI: "s3://backups/org-rbac-replay-after-remediation-1", BackupSHA256: strings.Repeat("7", 64), CheckpointRef: "restore-rehearsal-replay-remediation-1",
		AppliedAt: time.Date(2026, 7, 15, 13, 5, 0, 0, time.UTC),
	}
	if report, err := repository.RunMigration(context.Background(), MigrationApply, manifest, evidence); err != nil || report.Status != "applied" {
		t.Fatalf("RunMigration(initial apply) report = %+v, error = %v", report, err)
	}
	if err := db.Where("user_id = ? AND role_code = ?", "user-alice", "operator").Delete(&gormUserRole{}).Error; err != nil {
		t.Fatal(err)
	}
	replayed, err := repository.RunMigration(context.Background(), MigrationApply, manifest, evidence)
	if err != nil || replayed.Status != "applied" {
		t.Fatalf("RunMigration(replay after source-role removal) report = %+v, error = %v", replayed, err)
	}
	if revision, err := repository.CurrentGlobalRevision(context.Background()); err != nil || revision != 1 {
		t.Fatalf("global revision after replay = %d, error = %v", revision, err)
	}
}

func TestOrganizationRBACMigrationPersistsPageLeavesWithOneGlobalRevisionBump(t *testing.T) {
	db, repository := prepareOrganizationRBACTestRepository(t)
	seedOrganizationRBAC(t, db)
	if err := db.Create(&gormUser{ID: "user-alice", Code: "alice", TenantCode: "legacy", Status: StatusEnabled, ValuesJSON: `{}`}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&gormUserRole{UserID: "user-alice", RoleCode: "operator"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&gormPermission{ID: "permission-user-read", Code: "admin:user:read", Status: StatusEnabled, ResourceType: PermissionResourceTypeAPI}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&gormRolePermission{RoleCode: "operator", Permission: "admin:user:read"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&gormMenu{ID: "menu-users", Code: "users", Status: StatusEnabled, NodeType: string(MenuNodeTypePage), LegacyPermission: "admin:user:read"}).Error; err != nil {
		t.Fatal(err)
	}
	evidence := MigrationEvidence{
		RunID: "page-leaf-backfill-1", ActorID: "migration-admin", Reason: "persist reviewed page leaves", ApprovalRef: "change-page-leaves",
		BackupURI: "s3://backups/page-leaf-backfill-1", BackupSHA256: strings.Repeat("9", 64), CheckpointRef: "restore-page-leaf-backfill-1",
		AppliedAt: time.Date(2026, 7, 15, 13, 15, 0, 0, time.UTC),
	}
	if report, err := repository.RunMigration(context.Background(), MigrationApply, testOrganizationRBACMigrationManifest(), evidence); err != nil || report.Status != "applied" {
		t.Fatalf("RunMigration(page-leaf apply) report = %+v, error = %v", report, err)
	}
	menus, err := repository.LoadRoleMenus(context.Background(), "operator")
	if err != nil || !reflect.DeepEqual(menus.MenuCodes, []string{"users"}) || menus.Revision != 1 {
		t.Fatalf("persisted role menus = %+v, error = %v", menus, err)
	}
	if revision, err := repository.CurrentGlobalRevision(context.Background()); err != nil || revision != 1 {
		t.Fatalf("global revision = %d, error = %v", revision, err)
	}
}

func TestOrganizationRBACMigrationRollsBackWhenPersistedPrincipalComparisonDiffers(t *testing.T) {
	db, repository := prepareOrganizationRBACTestRepository(t)
	seedOrganizationRBAC(t, db)
	if err := db.Create(&gormUser{ID: "user-alice", Code: "alice", TenantCode: "legacy", Status: StatusEnabled, ValuesJSON: `{}`}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&gormUserRole{UserID: "user-alice", RoleCode: "operator"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&gormPermission{ID: "permission-user-read", Code: "admin:user:read", Status: StatusEnabled, ResourceType: PermissionResourceTypeAPI}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&gormRolePermission{RoleCode: "operator", Permission: "admin:user:read"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&gormMenu{ID: "menu-users", Code: "users", Status: StatusEnabled, NodeType: string(MenuNodeTypePage), LegacyPermission: "admin:user:read"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TRIGGER discard_migrated_role_menu AFTER INSERT ON platform_admin_role_menus BEGIN DELETE FROM platform_admin_role_menus WHERE role_code = NEW.role_code AND menu_code = NEW.menu_code; END`).Error; err != nil {
		t.Fatal(err)
	}
	evidence := MigrationEvidence{
		RunID: "persisted-mismatch-1", ActorID: "migration-admin", Reason: "verify persisted comparison rollback", ApprovalRef: "change-persisted-mismatch",
		BackupURI: "s3://backups/persisted-mismatch-1", BackupSHA256: strings.Repeat("8", 64), CheckpointRef: "restore-persisted-mismatch-1",
		AppliedAt: time.Date(2026, 7, 15, 13, 20, 0, 0, time.UTC),
	}
	if _, err := repository.RunMigration(context.Background(), MigrationApply, testOrganizationRBACMigrationManifest(), evidence); !errors.Is(err, ErrRolePoolViolation) {
		t.Fatalf("RunMigration(persisted mismatch) error = %v, want ErrRolePoolViolation", err)
	}
	var user gormUser
	if err := db.Where("code = ?", "alice").Take(&user).Error; err != nil {
		t.Fatal(err)
	}
	if user.ScopeType != "" || user.TenantCode != "legacy" || user.OrgUnitCode != "" {
		t.Fatalf("user after rolled back persisted mismatch = %+v", user)
	}
	var count int64
	if err := db.Model(&gormOrganizationRBACMigrationRun{}).Where("run_id = ?", evidence.RunID).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("migration run count after rollback = %d, error = %v", count, err)
	}
	if revision, err := repository.CurrentGlobalRevision(context.Background()); err != nil || revision != 0 {
		t.Fatalf("global revision after rollback = %d, error = %v", revision, err)
	}
}

func TestOrganizationRBACMigrationBlocksNonEquivalentPlatformPrincipalRemediation(t *testing.T) {
	db, repository := prepareOrganizationRBACTestRepository(t)
	seedOrganizationRBAC(t, db)
	if err := db.Create(&gormUser{ID: "user-root", Code: "root", TenantCode: "legacy-client", Status: StatusEnabled, ValuesJSON: `{}`}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&gormUserRole{UserID: "user-root", RoleCode: "operator"}).Error; err != nil {
		t.Fatal(err)
	}
	manifest := testOrganizationRBACMigrationManifest()
	manifest.PlatformPrincipalAllowlist = []string{"root"}
	manifest.RolePoolConflictRemediations = []RoleAssignmentRemediation{{
		UserCode: "root", RoleCode: "operator", Action: "replace-role", ReplacementRoleCode: "super-admin",
	}}

	inventory, err := repository.RunMigration(context.Background(), MigrationInventory, manifest, MigrationEvidence{RunID: "inventory-platform-principal-1"})
	if err != nil || len(inventory.Conflicts) != 0 {
		t.Fatalf("RunMigration(platform inventory) report = %+v, error = %v", inventory, err)
	}
	evidence := MigrationEvidence{
		RunID: "platform-principal-apply-1", ActorID: "migration-admin", Reason: "approved platform principal cleanup", ApprovalRef: "change-platform-1",
		BackupURI: "s3://backups/platform-principal-apply-1", BackupSHA256: strings.Repeat("d", 64), CheckpointRef: "restore-rehearsal-platform-1",
		AppliedAt: time.Date(2026, 7, 15, 13, 30, 0, 0, time.UTC),
	}
	if report, err := repository.RunMigration(context.Background(), MigrationApply, manifest, evidence); !errors.Is(err, ErrRolePoolViolation) || report.Status != "" {
		t.Fatalf("RunMigration(non-equivalent platform apply) report = %+v, error = %v", report, err)
	}
	var user gormUser
	if err := db.Where("code = ?", "root").Take(&user).Error; err != nil {
		t.Fatal(err)
	}
	if user.ScopeType != "" || user.TenantCode != "legacy-client" || user.OrgUnitCode != "" {
		t.Fatalf("platform user mutated by blocked migration = %+v", user)
	}
	var assignments []gormUserRole
	if err := db.Where("user_id = ?", user.ID).Order("role_code").Find(&assignments).Error; err != nil {
		t.Fatal(err)
	}
	if len(assignments) != 1 || assignments[0].RoleCode != "operator" {
		t.Fatalf("platform assignments after blocked migration = %+v", assignments)
	}
}

func TestOrganizationRBACMigrationNewRunKeepsOrganizationRevisionMonotonic(t *testing.T) {
	db, repository := prepareOrganizationRBACTestRepository(t)
	seedOrganizationRBAC(t, db)
	manifest := testOrganizationRBACMigrationManifest()
	now := time.Date(2026, 7, 15, 13, 45, 0, 0, time.UTC)
	firstEvidence := MigrationEvidence{
		RunID: "org-rbac-monotonic-1", ActorID: "migration-admin", Reason: "approved initial cutover", ApprovalRef: "change-monotonic-1",
		BackupURI: "s3://backups/org-rbac-monotonic-1", BackupSHA256: strings.Repeat("e", 64), CheckpointRef: "restore-rehearsal-monotonic-1", AppliedAt: now,
	}
	if _, err := repository.RunMigration(context.Background(), MigrationApply, manifest, firstEvidence); err != nil {
		t.Fatalf("RunMigration(first apply) error = %v", err)
	}
	if _, err := repository.ReplaceOrgUnitRoleGroups(context.Background(), ReplaceOrgUnitRoleGroupsRequest{
		OrgUnitCode: "acme-hq", RoleGroupCodes: []string{"acme-audit", "acme-ops"}, ExpectedRevision: 1,
		ActorID: "admin", ChangedAt: now.Add(time.Minute),
	}); err != nil {
		t.Fatalf("ReplaceOrgUnitRoleGroups() error = %v", err)
	}

	secondEvidence := MigrationEvidence{
		RunID: "org-rbac-monotonic-2", ActorID: "migration-admin", Reason: "approved repeated cutover", ApprovalRef: "change-monotonic-2",
		BackupURI: "s3://backups/org-rbac-monotonic-2", BackupSHA256: strings.Repeat("f", 64), CheckpointRef: "restore-rehearsal-monotonic-2", AppliedAt: now.Add(2 * time.Minute),
	}
	if _, err := repository.RunMigration(context.Background(), MigrationApply, manifest, secondEvidence); err != nil {
		t.Fatalf("RunMigration(second apply) error = %v", err)
	}
	bindings, err := repository.LoadOrgUnitRoleGroups(context.Background(), "acme-hq")
	if err != nil {
		t.Fatal(err)
	}
	if bindings.Revision != 3 || len(bindings.RoleGroupCodes) != 1 || bindings.RoleGroupCodes[0] != "acme-ops" {
		t.Fatalf("bindings after repeated migration = %+v", bindings)
	}
}

func TestOrganizationRBACMigrationRejectsInvalidValuesJSONWithoutMutation(t *testing.T) {
	for _, tt := range []struct {
		name   string
		values string
	}{
		{name: "malformed", values: `{"legacy":`},
		{name: "non string value", values: `{"legacy":1}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			db, repository := prepareOrganizationRBACTestRepository(t)
			seedOrganizationRBAC(t, db)
			if err := db.Create(&gormUser{ID: "user-alice", Code: "alice", TenantCode: "legacy-client", Status: StatusEnabled, ValuesJSON: tt.values}).Error; err != nil {
				t.Fatal(err)
			}
			manifest := testOrganizationRBACMigrationManifest()
			evidence := MigrationEvidence{
				RunID: "invalid-values-" + strings.ReplaceAll(tt.name, " ", "-"), ActorID: "migration-admin", Reason: "validate legacy values", ApprovalRef: "change-invalid-values",
				BackupURI: "s3://backups/invalid-values", BackupSHA256: strings.Repeat("1", 64), CheckpointRef: "restore-rehearsal-invalid-values",
				AppliedAt: time.Date(2026, 7, 15, 13, 50, 0, 0, time.UTC),
			}
			if _, err := repository.RunMigration(context.Background(), MigrationApply, manifest, evidence); err == nil {
				t.Fatal("RunMigration(invalid values) error = nil")
			}
			var user gormUser
			if err := db.Where("code = ?", "alice").Take(&user).Error; err != nil {
				t.Fatal(err)
			}
			if user.ScopeType != "" || user.TenantCode != "legacy-client" || user.OrgUnitCode != "" || user.ValuesJSON != tt.values {
				t.Fatalf("user mutated after rejected migration = %+v", user)
			}
			var runCount int64
			if err := db.Model(&gormOrganizationRBACMigrationRun{}).Where("run_id = ?", evidence.RunID).Count(&runCount).Error; err != nil || runCount != 0 {
				t.Fatalf("migration run count = %d, error = %v", runCount, err)
			}
		})
	}
}

func TestOrganizationRBACMigrationRollbackRecordsCheckpointWithoutMutatingData(t *testing.T) {
	db, repository := prepareOrganizationRBACTestRepository(t)
	seedOrganizationRBAC(t, db)
	manifest := testOrganizationRBACMigrationManifest()
	now := time.Date(2026, 7, 15, 14, 0, 0, 0, time.UTC)
	evidence := MigrationEvidence{
		RunID: "org-rbac-rollback-1", ActorID: "migration-admin", Reason: "restore approved checkpoint", ApprovalRef: "change-rollback-123",
		BackupURI: "s3://backups/org-rbac-run-1", BackupSHA256: strings.Repeat("b", 64), CheckpointRef: "restore-rehearsal-123", AppliedAt: now,
	}

	report, err := repository.RunMigration(context.Background(), MigrationRollback, manifest, evidence)
	if err != nil || report.Status != "external-checkpoint-restore-required" {
		t.Fatalf("RunMigration(rollback) report = %+v, error = %v", report, err)
	}
	var group gormRoleGroup
	if err := db.Where("code = ?", "acme-ops").Take(&group).Error; err != nil {
		t.Fatal(err)
	}
	if group.ScopeType != string(ScopeTenant) || group.TenantCode != "acme" {
		t.Fatalf("rollback mutated role group = %+v", group)
	}
	var run gormOrganizationRBACMigrationRun
	if err := db.Where("run_id = ?", evidence.RunID).Take(&run).Error; err != nil {
		t.Fatal(err)
	}
	if run.Status != "external-checkpoint-restore-required" || run.CheckpointRef != evidence.CheckpointRef {
		t.Fatalf("rollback run = %+v", run)
	}
	if repeated, err := repository.RunMigration(context.Background(), MigrationRollback, manifest, evidence); err != nil || repeated.Status != report.Status {
		t.Fatalf("RunMigration(repeated rollback) report = %+v, error = %v", repeated, err)
	}
	var runCount int64
	if err := db.Model(&gormOrganizationRBACMigrationRun{}).Where("run_id = ?", evidence.RunID).Count(&runCount).Error; err != nil || runCount != 1 {
		t.Fatalf("rollback run count = %d, error = %v", runCount, err)
	}
	evidence.CheckpointRef = "different-checkpoint"
	if _, err := repository.RunMigration(context.Background(), MigrationRollback, manifest, evidence); !errors.Is(err, ErrInvalid) {
		t.Fatalf("RunMigration(reused rollback run ID) error = %v, want ErrInvalid", err)
	}
}

func testOrganizationRBACMigrationManifest() MigrationManifest {
	return MigrationManifest{
		Version: "1.0.0",
		RoleGroupScopeTenantMap: map[string]RoleGroupPlacement{
			"acme-ops":       {ScopeType: ScopeTenant, TenantCode: "acme"},
			"acme-audit":     {ScopeType: ScopeTenant, TenantCode: "acme"},
			"platform-admin": {ScopeType: ScopePlatform},
		},
		OrphanRoleGroupMap:              map[string]string{},
		TenantUserOrganizationMap:       map[string]string{"alice": "acme-hq"},
		OrganizationRoleGroupBindingMap: map[string][]string{"acme-hq": {"acme-ops"}},
		PlatformPrincipalAllowlist:      []string{},
		RolePoolConflictRemediations:    []RoleAssignmentRemediation{},
	}
}

func hasMigrationConflictKind(conflicts []MigrationConflict, kind string) bool {
	for _, conflict := range conflicts {
		if conflict.Kind == kind {
			return true
		}
	}
	return false
}
