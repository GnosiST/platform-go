package organizationrbac

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"gorm.io/gorm"
)

type principalTestArea struct {
	ID         string `gorm:"column:id;primaryKey"`
	Code       string `gorm:"column:code;uniqueIndex"`
	Status     string `gorm:"column:status"`
	ParentCode string `gorm:"column:parent_code"`
}

func (principalTestArea) TableName() string { return "platform_area_codes" }

func TestCompareAllActivePrincipals(t *testing.T) {
	t.Run("aggregate legacy cross-role deny differs from post-role candidate union", func(t *testing.T) {
		db, repository, manifest := seedPrincipalComparison(t)
		createPrincipalRole(t, db, "role-a", `{"denyPermissions":"admin:report:read","dataScope":"self"}`, "admin:user:read")
		createPrincipalRole(t, db, "role-b", `{"dataScope":"custom_orgs","dataScopeOrgCodes":"acme-hq"}`, "admin:report:read")
		createPrincipalUser(t, db, "alice", StatusEnabled, "role-a", "role-b")
		manifest.TenantUserOrganizationMap["alice"] = "acme-hq"

		state, err := loadPrincipalComparisonState(db, manifest, PrincipalComparisonCandidate)
		if err != nil {
			t.Fatal(err)
		}
		legacy, err := legacyPrincipalSnapshot(state, state.Users[0])
		if err != nil {
			t.Fatal(err)
		}
		target, err := targetPrincipalSnapshot(state, state.Users[0], PrincipalComparisonCandidate)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(legacy.MenuCodes, []string{"users"}) || !reflect.DeepEqual(target.MenuCodes, []string{"reports", "users"}) {
			t.Fatalf("legacy menus = %v target menus = %v", legacy.MenuCodes, target.MenuCodes)
		}
		report, err := repository.CompareAllActivePrincipals(context.Background(), manifest, PrincipalComparisonCandidate)
		if err != nil {
			t.Fatal(err)
		}
		if report.ActivePrincipals != 1 || len(report.Differences) != 1 || report.Differences[0].AddedMenus != 1 {
			t.Fatalf("comparison report = %+v", report)
		}
	})

	t.Run("explicit wildcard platform principal equals every enabled permission-backed page leaf", func(t *testing.T) {
		db, repository, manifest := seedPrincipalComparison(t)
		createPrincipalRole(t, db, "platform-role", `{"dataScope":"all"}`, "*")
		createPrincipalUser(t, db, "platform-admin", StatusEnabled, "platform-role")
		manifest.PlatformPrincipalAllowlist = []string{"platform-admin"}

		report, err := repository.CompareAllActivePrincipals(context.Background(), manifest, PrincipalComparisonCandidate)
		if err != nil {
			t.Fatal(err)
		}
		if report.ActivePrincipals != 1 || len(report.Differences) != 0 || report.Equivalent != 1 {
			t.Fatalf("comparison report = %+v", report)
		}
		state, err := loadPrincipalComparisonState(db, manifest, PrincipalComparisonCandidate)
		if err != nil {
			t.Fatal(err)
		}
		snapshot, err := targetPrincipalSnapshot(state, state.Users[0], PrincipalComparisonCandidate)
		if err != nil {
			t.Fatal(err)
		}
		if snapshot.ScopeType != ScopePlatform || !reflect.DeepEqual(snapshot.MenuCodes, []string{"reports", "users"}) {
			t.Fatalf("platform snapshot = %+v", snapshot)
		}
	})

	t.Run("wildcard deny expands against enabled permissions and excludes permissionless pages", func(t *testing.T) {
		db, repository, manifest := seedPrincipalComparison(t)
		createPrincipalRole(t, db, "deny-reports", `{"denyPermissions":"admin:report:*","dataScope":"all"}`, "*")
		createPrincipalUser(t, db, "alice", StatusEnabled, "deny-reports")
		manifest.TenantUserOrganizationMap["alice"] = "acme-hq"

		state, err := loadPrincipalComparisonState(db, manifest, PrincipalComparisonCandidate)
		if err != nil {
			t.Fatal(err)
		}
		legacy, err := legacyPrincipalSnapshot(state, state.Users[0])
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(legacy.MenuCodes, []string{"users"}) {
			t.Fatalf("legacy wildcard-deny menus = %v, want users only", legacy.MenuCodes)
		}
		if report, err := repository.CompareAllActivePrincipals(context.Background(), manifest, PrincipalComparisonCandidate); err != nil || len(report.Differences) != 0 {
			t.Fatalf("wildcard-deny comparison report = %+v, error = %v", report, err)
		}
	})

	t.Run("missing tenant organization without allowlist is blocking and never platform", func(t *testing.T) {
		_, repository, manifest := seedPrincipalComparison(t)
		db := repository.db
		createPrincipalUser(t, db, "orphan", StatusEnabled)

		report, err := repository.CompareAllActivePrincipals(context.Background(), manifest, PrincipalComparisonCandidate)
		if err != nil {
			t.Fatal(err)
		}
		if report.ActivePrincipals != 1 || len(report.Differences) != 1 || !reflect.DeepEqual(report.Differences[0].BlockingReasons, []string{"target-organization-required"}) {
			t.Fatalf("comparison report = %+v", report)
		}
	})

	t.Run("enabled zero-permission user contributes an empty snapshot", func(t *testing.T) {
		db, repository, manifest := seedPrincipalComparison(t)
		createPrincipalUser(t, db, "empty", StatusEnabled)
		manifest.TenantUserOrganizationMap["empty"] = "acme-hq"

		report, err := repository.CompareAllActivePrincipals(context.Background(), manifest, PrincipalComparisonCandidate)
		if err != nil {
			t.Fatal(err)
		}
		if report.ActivePrincipals != 1 || report.Equivalent != 1 || len(report.Differences) != 0 {
			t.Fatalf("comparison report = %+v", report)
		}
	})

	t.Run("candidate remediation replaces roles permissions data scope and menus", func(t *testing.T) {
		db, _, manifest := seedPrincipalComparison(t)
		createPrincipalRole(t, db, "legacy-role", `{"dataScope":"self"}`, "admin:user:read")
		createPrincipalRole(t, db, "replacement-role", `{"dataScope":"current_and_children"}`, "admin:report:read")
		createPrincipalUser(t, db, "alice", StatusEnabled, "legacy-role")
		manifest.TenantUserOrganizationMap["alice"] = "acme-hq"
		manifest.RolePoolConflictRemediations = []RoleAssignmentRemediation{{UserCode: "alice", RoleCode: "legacy-role", Action: "replace-role", ReplacementRoleCode: "replacement-role"}}

		state, err := loadPrincipalComparisonState(db, manifest, PrincipalComparisonCandidate)
		if err != nil {
			t.Fatal(err)
		}
		target, err := targetPrincipalSnapshot(state, state.Users[0], PrincipalComparisonCandidate)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(target.RoleCodes, []string{"replacement-role"}) || !reflect.DeepEqual(target.AllowPermissions, []string{"admin:report:read"}) ||
			!reflect.DeepEqual(target.DataScopeOrgCodes, []string{"acme-hq", "acme-ops"}) || !reflect.DeepEqual(target.MenuCodes, []string{"reports"}) {
			t.Fatalf("remediated target = %+v", target)
		}
		if err := db.Create(&gormRoleMenu{RoleCode: "legacy-role", MenuCode: "users"}).Error; err != nil {
			t.Fatal(err)
		}
		state, err = loadPrincipalComparisonState(db, manifest, PrincipalComparisonPersisted)
		if err != nil {
			t.Fatal(err)
		}
		persisted, err := targetPrincipalSnapshot(state, state.Users[0], PrincipalComparisonPersisted)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(persisted.RoleCodes, []string{"legacy-role"}) || !reflect.DeepEqual(persisted.MenuCodes, []string{"users"}) {
			t.Fatalf("persisted target reapplied remediation = %+v", persisted)
		}
	})

	t.Run("disabled and deleted users roles permissions and pages are excluded", func(t *testing.T) {
		db, repository, manifest := seedPrincipalComparison(t)
		createPrincipalRole(t, db, "disabled-role", `{"dataScope":"all"}`, "admin:disabled:read")
		if err := db.Model(&gormRole{}).Where("code = ?", "disabled-role").Update("status", "disabled").Error; err != nil {
			t.Fatal(err)
		}
		createPrincipalUser(t, db, "active", StatusEnabled, "disabled-role")
		createPrincipalUser(t, db, "disabled", "disabled")
		createPrincipalUser(t, db, "deleted", StatusEnabled)
		manifest.TenantUserOrganizationMap["active"] = "acme-hq"
		if err := db.Create(&gormResourceLifecycle{Resource: "users", RecordID: "user-deleted", DeletedAt: "2026-07-16T00:00:00Z"}).Error; err != nil {
			t.Fatal(err)
		}
		report, err := repository.CompareAllActivePrincipals(context.Background(), manifest, PrincipalComparisonCandidate)
		if err != nil {
			t.Fatal(err)
		}
		if report.ActivePrincipals != 1 || report.Equivalent != 1 {
			t.Fatalf("comparison report = %+v", report)
		}
	})
}

func TestCompareAllActivePrincipalsNormalizesEffectiveDataScope(t *testing.T) {
	db, _, manifest := seedPrincipalComparison(t)
	createPrincipalRole(t, db, "custom-self", `{"dataScope":"custom_orgs","dataScopeOrgCodes":"acme-ops"}`, "admin:user:read")
	createPrincipalRole(t, db, "self", `{"dataScope":"self"}`)
	createPrincipalUser(t, db, "alice", StatusEnabled, "custom-self", "self")
	manifest.TenantUserOrganizationMap["alice"] = "acme-hq"
	state, err := loadPrincipalComparisonState(db, manifest, PrincipalComparisonCandidate)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := targetPrincipalSnapshot(state, state.Users[0], PrincipalComparisonCandidate)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.DataScopeAll || !snapshot.DataScopeSelf || !reflect.DeepEqual(snapshot.DataScopeOrgCodes, []string{"acme-ops"}) {
		t.Fatalf("custom+self scope = %+v", snapshot)
	}

	if err := db.Model(&gormRole{}).Where("code = ?", "custom-self").Update("values_json", `{"dataScope":"current_and_children"}`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&gormRole{}).Where("code = ?", "self").Update("values_json", `{"dataScope":"current_and_children_areas"}`).Error; err != nil {
		t.Fatal(err)
	}
	state, err = loadPrincipalComparisonState(db, manifest, PrincipalComparisonCandidate)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err = targetPrincipalSnapshot(state, state.Users[0], PrincipalComparisonCandidate)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(snapshot.DataScopeOrgCodes, []string{"acme-hq", "acme-ops"}) || !reflect.DeepEqual(snapshot.DataScopeAreaCodes, []string{"110000", "110101"}) {
		t.Fatalf("expanded scope = %+v", snapshot)
	}
}

func seedPrincipalComparison(t *testing.T) (*gorm.DB, *GORMRepository, MigrationManifest) {
	t.Helper()
	db, repository := prepareOrganizationRBACTestRepository(t)
	if err := db.AutoMigrate(&gormOrgUnitMetadata{}, &principalTestArea{}); err != nil {
		t.Fatal(err)
	}
	rows := []any{
		&gormOrgUnitMetadata{ID: "org-acme-hq", Code: "acme-hq", Status: StatusEnabled, TenantCode: "acme", ParentCode: "", AreaCode: "110000", ValuesJSON: `{}`},
		&gormOrgUnitMetadata{ID: "org-acme-ops", Code: "acme-ops", Status: StatusEnabled, TenantCode: "acme", ParentCode: "acme-hq", AreaCode: "110101", ValuesJSON: `{}`},
		&principalTestArea{ID: "area-110000", Code: "110000", Status: StatusEnabled},
		&principalTestArea{ID: "area-110101", Code: "110101", Status: StatusEnabled, ParentCode: "110000"},
		&gormPermission{ID: "permission-user", Code: "admin:user:read", Status: StatusEnabled, ResourceType: PermissionResourceTypeAPI},
		&gormPermission{ID: "permission-report", Code: "admin:report:read", Status: StatusEnabled, ResourceType: PermissionResourceTypeAPI},
		&gormPermission{ID: "permission-all", Code: "*", Status: StatusEnabled, ResourceType: PermissionResourceTypeAPI},
		&gormPermission{ID: "permission-disabled", Code: "admin:disabled:read", Status: "disabled", ResourceType: PermissionResourceTypeAPI},
		&gormMenu{ID: "menu-users", Code: "users", Status: StatusEnabled, NodeType: string(MenuNodeTypePage), LegacyPermission: "admin:user:read"},
		&gormMenu{ID: "menu-reports", Code: "reports", Status: StatusEnabled, NodeType: string(MenuNodeTypePage), LegacyPermission: "admin:report:read"},
		&gormMenu{ID: "menu-permissionless", Code: "permissionless", Status: StatusEnabled, NodeType: string(MenuNodeTypePage)},
		&gormMenu{ID: "menu-disabled", Code: "disabled", Status: "disabled", NodeType: string(MenuNodeTypePage), LegacyPermission: "admin:user:read"},
		&gormMenu{ID: "menu-deleted", Code: "deleted", Status: StatusEnabled, NodeType: string(MenuNodeTypePage), LegacyPermission: "admin:user:read"},
		&gormResourceLifecycle{Resource: "menus", RecordID: "menu-deleted", DeletedAt: "2026-07-16T00:00:00Z"},
	}
	for _, row := range rows {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db, repository, MigrationManifest{
		Version: "principal-test", RoleGroupScopeTenantMap: map[string]RoleGroupPlacement{}, OrphanRoleGroupMap: map[string]string{},
		TenantUserOrganizationMap: map[string]string{}, OrganizationRoleGroupBindingMap: map[string][]string{},
		PlatformPrincipalAllowlist: []string{}, RolePoolConflictRemediations: []RoleAssignmentRemediation{},
	}
}

func createPrincipalRole(t *testing.T, db *gorm.DB, code, values string, permissions ...string) {
	t.Helper()
	if values == "" {
		values = `{}`
	}
	if err := db.Create(&gormRole{ID: "role-" + code, Code: code, Status: StatusEnabled, ValuesJSON: values}).Error; err != nil {
		t.Fatal(err)
	}
	for _, permission := range permissions {
		if err := db.Create(&gormRolePermission{RoleCode: code, Permission: permission}).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func createPrincipalUser(t *testing.T, db *gorm.DB, code, status string, roles ...string) {
	t.Helper()
	if err := db.Create(&gormUser{ID: "user-" + code, Code: code, Status: status, ValuesJSON: `{}`}).Error; err != nil {
		t.Fatal(err)
	}
	for _, role := range roles {
		if err := db.Create(&gormUserRole{UserID: "user-" + code, RoleCode: role}).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func TestRemediatedRoleCodesRejectsDuplicateOrMissingAssignments(t *testing.T) {
	user := gormUser{Code: "alice"}
	manifest := MigrationManifest{RolePoolConflictRemediations: []RoleAssignmentRemediation{{UserCode: "alice", RoleCode: "missing", Action: "remove-role"}}}
	if _, err := remediatedRoleCodes(user, []string{"operator"}, manifest); !errors.Is(err, ErrRolePoolViolation) {
		t.Fatalf("remediatedRoleCodes() error = %v, want ErrRolePoolViolation", err)
	}
}
