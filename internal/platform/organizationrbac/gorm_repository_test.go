package organizationrbac

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestOpenGORMRepositoryDoesNotCreateSchema(t *testing.T) {
	db := openOrganizationRBACTestDB(t)
	if _, err := OpenGORMRepository(context.Background(), db); !errors.Is(err, ErrRepositoryFailed) {
		t.Fatalf("OpenGORMRepository() error = %v, want ErrRepositoryFailed", err)
	}
	if db.Migrator().HasTable(orgUnitRoleGroupsTable) {
		t.Fatalf("OpenGORMRepository() created %q", orgUnitRoleGroupsTable)
	}
}

func TestPrepareGORMRepositoryIsAdditiveAndOpenReusesSchema(t *testing.T) {
	db := openOrganizationRBACTestDB(t)
	type legacyRoleGroup struct {
		ID     string `gorm:"column:id;primaryKey"`
		Code   string `gorm:"column:code;uniqueIndex"`
		Name   string `gorm:"column:name"`
		Status string `gorm:"column:status"`
	}
	type legacyPermission struct {
		ID     string `gorm:"column:id;primaryKey"`
		Code   string `gorm:"column:code;uniqueIndex"`
		Status string `gorm:"column:status"`
	}
	funcTable := roleGroupsTable
	if err := db.Table(funcTable).AutoMigrate(&legacyRoleGroup{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Table(funcTable).Create(&legacyRoleGroup{ID: "group-1", Code: "legacy", Name: "Legacy", Status: StatusEnabled}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table(permissionsTable).AutoMigrate(&legacyPermission{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Table(permissionsTable).Create(&legacyPermission{ID: "permission-legacy", Code: "admin:legacy:read", Status: StatusEnabled}).Error; err != nil {
		t.Fatal(err)
	}

	repository, err := PrepareGORMRepository(context.Background(), db)
	if err != nil {
		t.Fatalf("PrepareGORMRepository() error = %v", err)
	}
	if !repository.Persistent() {
		t.Fatal("Persistent() = false")
	}
	for _, table := range []string{organizationsTable, roleGroupsTable, rolesTable, usersTable, userRolesTable, permissionsTable, rolePermissionsTable, resourceLifecycleTable, orgUnitRoleGroupsTable, orgUnitRoleGroupRevisionsTable} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("PrepareGORMRepository() did not create %q", table)
		}
	}
	if !db.Migrator().HasColumn(&gormPermission{}, "ResourceType") {
		t.Fatal("PrepareGORMRepository() did not add permission resource_type")
	}
	for _, column := range []string{"scope_type", "tenant_code", "revision"} {
		if !db.Migrator().HasColumn(&gormRoleGroup{}, column) {
			t.Fatalf("PrepareGORMRepository() did not add %q", column)
		}
	}
	var count int64
	if err := db.Table(roleGroupsTable).Where("code = ?", "legacy").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("legacy row count = %d, error = %v", count, err)
	}
	var permission gormPermission
	if err := db.Where("code = ?", "admin:legacy:read").Take(&permission).Error; err != nil || permission.ResourceType != PermissionResourceTypeAPI {
		t.Fatalf("legacy permission = %+v, error = %v", permission, err)
	}
	if _, err := OpenGORMRepository(context.Background(), db); err != nil {
		t.Fatalf("OpenGORMRepository(prepared) error = %v", err)
	}
}

func TestGORMRepositoryReplaceUsesRolePoolAndRevisionCAS(t *testing.T) {
	db, repository := prepareOrganizationRBACTestRepository(t)
	seedOrganizationRBAC(t, db)
	now := time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)

	first, err := repository.ReplaceOrgUnitRoleGroups(context.Background(), ReplaceOrgUnitRoleGroupsRequest{
		OrgUnitCode: "acme-hq", RoleGroupCodes: []string{"acme-ops", "acme-ops"}, ExpectedRevision: 0, ActorID: "admin", ChangedAt: now,
	})
	if err != nil {
		t.Fatalf("ReplaceOrgUnitRoleGroups(first) error = %v", err)
	}
	if first.Revision != 1 || !reflect.DeepEqual(first.RoleGroupCodes, []string{"acme-ops"}) {
		t.Fatalf("ReplaceOrgUnitRoleGroups(first) = %+v", first)
	}
	pool, err := repository.EffectiveRolePool(context.Background(), "acme-hq")
	if err != nil || len(pool) != 1 || pool[0].RoleCode != "operator" {
		t.Fatalf("EffectiveRolePool() = %+v, %v", pool, err)
	}

	if err := db.Create(&gormUser{ID: "user-alice", Code: "alice", ScopeType: string(ScopeTenant), TenantCode: "acme", OrgUnitCode: "acme-hq", Status: StatusEnabled}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&gormUserRole{UserID: "user-alice", RoleCode: "operator"}).Error; err != nil {
		t.Fatal(err)
	}
	_, err = repository.ReplaceOrgUnitRoleGroups(context.Background(), ReplaceOrgUnitRoleGroupsRequest{
		OrgUnitCode: "acme-hq", RoleGroupCodes: []string{"acme-audit"}, ExpectedRevision: 1, ActorID: "admin", ChangedAt: now.Add(time.Minute),
	})
	if !errors.Is(err, ErrRolePoolViolation) {
		t.Fatalf("ReplaceOrgUnitRoleGroups(role pool violation) error = %v", err)
	}
	unchanged, err := repository.LoadOrgUnitRoleGroups(context.Background(), "acme-hq")
	if err != nil || unchanged.Revision != 1 || !reflect.DeepEqual(unchanged.RoleGroupCodes, []string{"acme-ops"}) {
		t.Fatalf("LoadOrgUnitRoleGroups(after rejected change) = %+v, %v", unchanged, err)
	}

	_, err = repository.ReplaceOrgUnitRoleGroups(context.Background(), ReplaceOrgUnitRoleGroupsRequest{
		OrgUnitCode: "acme-hq", RoleGroupCodes: []string{"acme-ops"}, ExpectedRevision: 0, ActorID: "admin", ChangedAt: now.Add(2 * time.Minute),
	})
	if !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("ReplaceOrgUnitRoleGroups(stale) error = %v, want ErrRevisionConflict", err)
	}
	var conflict *RevisionConflictError
	if !errors.As(err, &conflict) || conflict.Actual != 1 {
		t.Fatalf("revision conflict = %#v", conflict)
	}
}

func TestGORMRepositoryReplaceRollsBackRevisionAndRelationsTogether(t *testing.T) {
	db, repository := prepareOrganizationRBACTestRepository(t)
	seedOrganizationRBAC(t, db)
	now := time.Date(2026, 7, 15, 11, 0, 0, 0, time.UTC)
	if _, err := repository.ReplaceOrgUnitRoleGroups(context.Background(), ReplaceOrgUnitRoleGroupsRequest{
		OrgUnitCode: "acme-hq", RoleGroupCodes: []string{"acme-ops"}, ExpectedRevision: 0, ActorID: "admin", ChangedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	trigger := fmt.Sprintf(`CREATE TRIGGER fail_audit_binding BEFORE INSERT ON %s WHEN NEW.role_group_code = 'acme-audit' BEGIN SELECT RAISE(FAIL, 'blocked'); END`, orgUnitRoleGroupsTable)
	if err := db.Exec(trigger).Error; err != nil {
		t.Fatal(err)
	}

	_, err := repository.ReplaceOrgUnitRoleGroups(context.Background(), ReplaceOrgUnitRoleGroupsRequest{
		OrgUnitCode: "acme-hq", RoleGroupCodes: []string{"acme-audit"}, ExpectedRevision: 1, ActorID: "admin", ChangedAt: now.Add(time.Minute),
	})
	if !errors.Is(err, ErrRepositoryFailed) {
		t.Fatalf("ReplaceOrgUnitRoleGroups(failed insert) error = %v, want ErrRepositoryFailed", err)
	}
	loaded, err := repository.LoadOrgUnitRoleGroups(context.Background(), "acme-hq")
	if err != nil || loaded.Revision != 1 || !reflect.DeepEqual(loaded.RoleGroupCodes, []string{"acme-ops"}) {
		t.Fatalf("LoadOrgUnitRoleGroups(after rollback) = %+v, %v", loaded, err)
	}
}

func TestGORMRepositoryDerivesTenantAndRejectsCrossScopeRoles(t *testing.T) {
	db, repository := prepareOrganizationRBACTestRepository(t)
	seedOrganizationRBAC(t, db)
	if _, err := repository.ReplaceOrgUnitRoleGroups(context.Background(), ReplaceOrgUnitRoleGroupsRequest{
		OrgUnitCode: "acme-hq", RoleGroupCodes: []string{"acme-ops"}, ExpectedRevision: 0, ActorID: "admin", ChangedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	derived, err := repository.DeriveAndValidateUser(context.Background(), User{Code: "alice", ScopeType: ScopeTenant, TenantCode: "client-value", OrgUnitCode: "acme-hq"}, []string{"operator"})
	if err != nil || derived.TenantCode != "acme" {
		t.Fatalf("DeriveAndValidateUser() = %+v, %v", derived, err)
	}
	if _, err := repository.DeriveAndValidateUser(context.Background(), derived, []string{"super-admin"}); !errors.Is(err, ErrRolePoolViolation) {
		t.Fatalf("DeriveAndValidateUser(cross scope) error = %v, want ErrRolePoolViolation", err)
	}
}

func prepareOrganizationRBACTestRepository(t *testing.T) (*gorm.DB, *GORMRepository) {
	t.Helper()
	db := openOrganizationRBACTestDB(t)
	repository, err := PrepareGORMRepository(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	return db, repository
}

func openOrganizationRBACTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func seedOrganizationRBAC(t *testing.T, db *gorm.DB) {
	t.Helper()
	rows := []any{
		&gormOrganization{ID: "org-acme-hq", Code: "acme-hq", TenantCode: "acme", Status: StatusEnabled},
		&gormRoleGroup{ID: "group-acme-ops", Code: "acme-ops", Name: "Operations", ScopeType: string(ScopeTenant), TenantCode: "acme", Status: StatusEnabled},
		&gormRoleGroup{ID: "group-acme-audit", Code: "acme-audit", Name: "Audit", ScopeType: string(ScopeTenant), TenantCode: "acme", Status: StatusEnabled},
		&gormRoleGroup{ID: "group-platform", Code: "platform-admin", Name: "Platform", ScopeType: string(ScopePlatform), Status: StatusEnabled},
		&gormRole{ID: "role-operator", Code: "operator", GroupCode: "acme-ops", Status: StatusEnabled},
		&gormRole{ID: "role-auditor", Code: "auditor", GroupCode: "acme-audit", Status: StatusEnabled},
		&gormRole{ID: "role-disabled", Code: "disabled-role", GroupCode: "acme-ops", Status: "disabled"},
		&gormRole{ID: "role-super-admin", Code: "super-admin", GroupCode: "platform-admin", Status: StatusEnabled},
	}
	for _, row := range rows {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
}
