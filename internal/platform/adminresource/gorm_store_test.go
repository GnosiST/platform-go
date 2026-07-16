package adminresource

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"gorm.io/gorm"
	"platform-go/internal/platform/core"
	"platform-go/internal/platform/storage"
)

type preTask6GORMAdminAuditLog struct {
	ID          string `gorm:"column:id;primaryKey"`
	Code        string `gorm:"column:code;uniqueIndex;not null"`
	Name        string `gorm:"column:name;not null"`
	Status      string `gorm:"column:status;not null"`
	Description string `gorm:"column:description;not null"`
	UpdatedAt   string `gorm:"column:updated_at;not null"`
	Actor       string `gorm:"column:actor;index;not null"`
	Action      string `gorm:"column:action;index;not null"`
	Resource    string `gorm:"column:resource;index;not null"`
	CreatedAt   string `gorm:"column:created_at;index;not null"`
	ValuesJSON  string `gorm:"column:values_json;not null"`
}

func (preTask6GORMAdminAuditLog) TableName() string {
	return adminAuditLogsTable
}

func TestOpenGORMAdminResourceRepositoryDoesNotCreateSchema(t *testing.T) {
	db, err := storage.OpenGORM(storage.Config{Driver: "sqlite", DSN: filepath.Join(t.TempDir(), "admin-open.db")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := OpenGORMAdminResourceRepository(context.Background(), db); err == nil {
		t.Fatal("OpenGORMAdminResourceRepository() error = nil, want missing schema failure")
	}
	if db.Migrator().HasTable(&gormAdminResourceRecord{}) {
		t.Fatal("OpenGORMAdminResourceRepository() created schema")
	}
}

func TestGORMAdminResourceRepositoryPersistsSnapshots(t *testing.T) {
	db := openAdminResourceGORMDB(t)
	repository, err := NewGORMAdminResourceRepository(context.Background(), db)
	if err != nil {
		t.Fatalf("NewGORMAdminResourceRepository() error = %v", err)
	}
	snapshot := ResourceSnapshot{
		NextID: 1042,
		Resources: map[string][]Record{
			"tenants": {
				{
					ID: "tenant-1042", Code: "acme", Name: "Acme Tenant", Status: "enabled", Description: "GORM tenant", UpdatedAt: "2026-07-04T00:00:00Z", Values: map[string]string{"isolation": "sandbox"},
					DeletedAt: "2026-07-14T00:00:00Z", DeletedBy: "user-admin", DeleteReason: "retired", PurgeAfter: "2026-08-13T00:00:00Z", DeletionPolicyVersion: 1,
				},
			},
			"roles": {
				{ID: "role-operator", Code: "operator", Name: "Operator", Status: "enabled", UpdatedAt: "2026-07-04T00:00:00Z", Values: map[string]string{"permissions": "admin:user:read"}},
			},
		},
	}

	if _, err := repository.Save(context.Background(), snapshot); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	loaded, err := repository.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded.NextID != snapshot.NextID {
		t.Fatalf("NextID = %d, want %d", loaded.NextID, snapshot.NextID)
	}
	if !reflect.DeepEqual(loaded.Resources, snapshot.Resources) {
		t.Fatalf("Resources = %#v, want %#v", loaded.Resources, snapshot.Resources)
	}
}

func TestGORMAdminResourceRepositoryRejectsStaleRevision(t *testing.T) {
	repository, err := NewGORMAdminResourceRepository(context.Background(), openAdminResourceGORMDB(t))
	if err != nil {
		t.Fatalf("NewGORMAdminResourceRepository() error = %v", err)
	}
	first, err := repository.Load(context.Background())
	if err != nil {
		t.Fatalf("Load(first) error = %v", err)
	}
	stale := first
	first.Resources["tenants"] = []Record{{ID: "tenant-a", Code: "a", Name: "A", Status: "enabled"}}
	committed, err := repository.Save(context.Background(), first)
	if err != nil || committed != 1 {
		t.Fatalf("Save(first) = %d, %v; want revision 1", committed, err)
	}
	stale.Resources["tenants"] = []Record{{ID: "tenant-b", Code: "b", Name: "B", Status: "enabled"}}
	if _, err := repository.Save(context.Background(), stale); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("Save(stale) error = %v, want ErrRevisionConflict", err)
	}

	loaded, err := repository.Load(context.Background())
	if err != nil {
		t.Fatalf("Load(after stale save) error = %v", err)
	}
	if !hasRecordID(loaded.Resources["tenants"], "tenant-a") {
		t.Fatalf("tenants after stale save = %+v, want tenant-a", loaded.Resources["tenants"])
	}
	if hasRecordID(loaded.Resources["tenants"], "tenant-b") {
		t.Fatalf("tenants after stale save = %+v, tenant-b must not be written", loaded.Resources["tenants"])
	}
}

func TestGORMAdminResourceRepositoryRejectsMixedRevisionLoad(t *testing.T) {
	ctx := context.Background()
	dsn := filepath.Join(t.TempDir(), "admin-resources.db")
	readerDB := openAdminResourceGORMDBAt(t, dsn)
	writerDB := openAdminResourceGORMDBAt(t, dsn)
	reader, err := NewGORMAdminResourceRepository(ctx, readerDB)
	if err != nil {
		t.Fatalf("NewGORMAdminResourceRepository(reader) error = %v", err)
	}
	writer, err := NewGORMAdminResourceRepository(ctx, writerDB)
	if err != nil {
		t.Fatalf("NewGORMAdminResourceRepository(writer) error = %v", err)
	}
	writerSnapshot, err := writer.Load(ctx)
	if err != nil {
		t.Fatalf("writer.Load() error = %v", err)
	}

	callbackName := "test:commit_revision_after_first_read"
	committed := false
	var commitErr error
	if err := readerDB.Callback().Query().After("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if committed || !gormQueryReadsRevision(tx) {
			return
		}
		committed = true
		writerSnapshot.Resources["tenants"] = []Record{{ID: "tenant-new", Code: "new", Name: "New", Status: "enabled"}}
		_, commitErr = writer.Save(ctx, writerSnapshot)
	}); err != nil {
		t.Fatalf("register query callback error = %v", err)
	}

	loaded, err := reader.Load(ctx)
	if commitErr != nil {
		t.Fatalf("writer.Save() error = %v", commitErr)
	}
	if !committed {
		t.Fatal("writer Save was not triggered after the first revision read")
	}
	if !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("reader.Load() error = %v, want ErrRevisionConflict", err)
	}
	var conflict *RevisionConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("reader.Load() error = %T, want *RevisionConflictError", err)
	}
	if conflict.Expected != 0 || conflict.Actual != 1 {
		t.Fatalf("revision conflict = %+v, want expected 0 actual 1", conflict)
	}
	if !reflect.DeepEqual(loaded, ResourceSnapshot{}) {
		t.Fatalf("reader.Load() snapshot = %#v, want zero snapshot on mixed revision", loaded)
	}
}

func TestGORMAdminResourceRepositoryNormalizesSystemResources(t *testing.T) {
	db := openAdminResourceGORMDB(t)
	repository, err := NewGORMAdminResourceRepository(context.Background(), db)
	if err != nil {
		t.Fatalf("NewGORMAdminResourceRepository() error = %v", err)
	}
	for _, model := range []interface{}{
		&gormAdminUser{},
		&gormAdminUserRole{},
		&gormAdminTenant{},
		&gormAdminRole{},
		&gormAdminRolePermission{},
		&gormAdminPermission{},
		&gormAdminMenu{},
	} {
		if !db.Migrator().HasTable(model) {
			t.Fatalf("expected normalized table for %T", model)
		}
	}
	snapshot := ResourceSnapshot{
		NextID: 1088,
		Resources: map[string][]Record{
			"users": {
				{ID: "user-auditor", Code: "auditor", Name: "Auditor", Status: "enabled", Description: "Audit user", UpdatedAt: "2026-07-04T00:00:00Z", Values: map[string]string{"role": "auditor-role", "roles": "auditor-role", "nameZh": "审计员"}},
			},
			"roles": {
				{ID: "role-auditor", Code: "auditor-role", Name: "Auditor Role", Status: "enabled", Description: "Audit role", UpdatedAt: "2026-07-04T00:00:00Z", Values: map[string]string{"permissions": "admin:audit-log:read,admin:menu:read", "nameZh": "审计角色"}},
			},
			"permissions": {
				{ID: "permission-audit-log-read", Code: "admin:audit-log:read", Name: "Audit Log Read", Status: "enabled", Description: "Audit log read permission", UpdatedAt: "2026-07-04T00:00:00Z", Values: map[string]string{"capability": "audit", "resource": "audit-logs", "action": "read", "prefix": "admin:audit-log", "resourceType": "api", "nameZh": "审计日志读取"}},
			},
			"menus": {
				{ID: "menu-audit-logs", Code: "audit-logs", Name: "Audit Logs", Status: "enabled", Description: "Audit logs menu", UpdatedAt: "2026-07-04T00:00:00Z", Values: map[string]string{"route": "/audit-logs", "parent": "audit", "resource": "audit-logs", "permission": "admin:audit-log:read", "group": "governance", "icon": "audit", "order": "110", "titleZh": "审计日志", "titleEn": "Audit Logs", "descriptionZh": "审计记录", "descriptionEn": "Audit records", "cacheEnabled": "true"}},
			},
			"tenants": {
				{ID: "tenant-acme", Code: "acme", Name: "Acme", Status: "enabled", Description: "Generic tenant", UpdatedAt: "2026-07-04T00:00:00Z", Values: map[string]string{"areaCode": "110000", "isolation": "shared"}},
			},
		},
	}

	if _, err := repository.Save(context.Background(), snapshot); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	assertGORMCount(t, db, &gormAdminUser{}, 1)
	assertGORMCount(t, db, &gormAdminUserRole{}, 1)
	assertGORMCount(t, db, &gormAdminTenant{}, 1)
	assertGORMCount(t, db, &gormAdminRole{}, 1)
	assertGORMCount(t, db, &gormAdminRolePermission{}, 2)
	assertGORMCount(t, db, &gormAdminPermission{}, 1)
	assertGORMCount(t, db, &gormAdminMenu{}, 1)
	var genericSystemRows int64
	if err := db.Model(&gormAdminResourceRecord{}).Where("resource IN ?", []string{"users", "tenants", "roles", "permissions", "menus"}).Count(&genericSystemRows).Error; err != nil {
		t.Fatalf("count generic system rows error = %v", err)
	}
	if genericSystemRows != 0 {
		t.Fatalf("generic system rows = %d, want 0", genericSystemRows)
	}
	assertGORMCount(t, db, &gormAdminResourceRecord{}, 0)

	loaded, err := repository.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !reflect.DeepEqual(loaded.Resources, snapshot.Resources) {
		t.Fatalf("Resources = %#v, want %#v", loaded.Resources, snapshot.Resources)
	}
}

func TestGORMAdminResourceRepositoryNormalizesGovernanceResources(t *testing.T) {
	db := openAdminResourceGORMDB(t)
	repository, err := NewGORMAdminResourceRepository(context.Background(), db)
	if err != nil {
		t.Fatalf("NewGORMAdminResourceRepository() error = %v", err)
	}
	for _, model := range []interface{}{
		&gormAdminOrgUnit{},
		&gormAdminRoleGroup{},
		&gormAdminAreaCode{},
	} {
		if !db.Migrator().HasTable(model) {
			t.Fatalf("expected normalized governance table for %T", model)
		}
	}
	snapshot := ResourceSnapshot{
		NextID: 1090,
		Resources: map[string][]Record{
			"org-units": {
				{ID: "org-platform-hq", Code: "platform-hq", Name: "Platform HQ", Status: "enabled", Description: "Top org", UpdatedAt: "2026-07-04T00:00:00Z", Values: map[string]string{"type": "organization", "tenantCode": "platform", "areaCode": "110000", "sortOrder": "10", "roleGroupCount": "0", "effectiveRoleCount": "0"}},
			},
			"role-groups": {
				{ID: "role-group-system-admin", Code: "system-admin", Name: "System Admin", Status: "enabled", Description: "System role group", UpdatedAt: "2026-07-04T00:00:00Z", Values: map[string]string{"sortOrder": "10"}},
			},
			"area-codes": {
				{ID: "area-beijing", Code: "110000", Name: "Beijing", Status: "enabled", Description: "Area", UpdatedAt: "2026-07-04T00:00:00Z", Values: map[string]string{"parentCode": "CN", "level": "province", "path": "CN/110000", "sortOrder": "20"}},
			},
		},
	}

	if _, err := repository.Save(context.Background(), snapshot); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	assertGORMCount(t, db, &gormAdminOrgUnit{}, 1)
	assertGORMCount(t, db, &gormAdminRoleGroup{}, 1)
	assertGORMCount(t, db, &gormAdminAreaCode{}, 1)
	var genericGovernanceRows int64
	if err := db.Model(&gormAdminResourceRecord{}).Where("resource IN ?", []string{"org-units", "role-groups", "area-codes"}).Count(&genericGovernanceRows).Error; err != nil {
		t.Fatalf("count generic governance rows error = %v", err)
	}
	if genericGovernanceRows != 0 {
		t.Fatalf("generic governance rows = %d, want 0", genericGovernanceRows)
	}

	loaded, err := repository.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !reflect.DeepEqual(loaded.Resources, snapshot.Resources) {
		t.Fatalf("Resources = %#v, want %#v", loaded.Resources, snapshot.Resources)
	}
}

func TestGORMAdminResourceRepositoryProjectsOrganizationRoleCounts(t *testing.T) {
	ctx := context.Background()
	db := openAdminResourceGORMDB(t)
	repository, err := NewGORMAdminResourceRepository(ctx, db)
	if err != nil {
		t.Fatalf("NewGORMAdminResourceRepository() error = %v", err)
	}
	deletedAt := "2026-07-15T00:00:00Z"
	snapshot := ResourceSnapshot{
		NextID: 20,
		Resources: map[string][]Record{
			"org-units": {
				{ID: "org-acme-hq", Code: "acme-hq", Name: "Acme HQ", Status: "enabled", Values: map[string]string{"type": "organization", "tenantCode": "acme"}},
			},
			"role-groups": {
				{ID: "group-valid", Code: "valid", Name: "Valid", Status: "enabled", Values: map[string]string{"scopeType": "tenant", "tenantCode": "acme"}},
				{ID: "group-disabled", Code: "disabled", Name: "Disabled", Status: "disabled", Values: map[string]string{"scopeType": "tenant", "tenantCode": "acme"}},
				{ID: "group-deleted", Code: "deleted", Name: "Deleted", Status: "enabled", DeletedAt: deletedAt, Values: map[string]string{"scopeType": "tenant", "tenantCode": "acme"}},
				{ID: "group-cross-tenant", Code: "cross-tenant", Name: "Cross Tenant", Status: "enabled", Values: map[string]string{"scopeType": "tenant", "tenantCode": "other"}},
			},
			"roles": {
				{ID: "role-valid", Code: "valid-role", Name: "Valid Role", Status: "enabled", Values: map[string]string{"groupCode": "valid"}},
				{ID: "role-disabled", Code: "disabled-role", Name: "Disabled Role", Status: "disabled", Values: map[string]string{"groupCode": "valid"}},
				{ID: "role-deleted", Code: "deleted-role", Name: "Deleted Role", Status: "enabled", DeletedAt: deletedAt, Values: map[string]string{"groupCode": "valid"}},
				{ID: "role-disabled-group", Code: "disabled-group-role", Name: "Disabled Group Role", Status: "enabled", Values: map[string]string{"groupCode": "disabled"}},
				{ID: "role-deleted-group", Code: "deleted-group-role", Name: "Deleted Group Role", Status: "enabled", Values: map[string]string{"groupCode": "deleted"}},
				{ID: "role-cross-tenant", Code: "cross-tenant-role", Name: "Cross Tenant Role", Status: "enabled", Values: map[string]string{"groupCode": "cross-tenant"}},
			},
		},
	}
	if _, err := repository.Save(ctx, snapshot); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := db.AutoMigrate(&gormAdminOrgUnitRoleGroup{}); err != nil {
		t.Fatalf("AutoMigrate(org unit role groups) error = %v", err)
	}
	bindings := []gormAdminOrgUnitRoleGroup{
		{OrgUnitCode: "acme-hq", RoleGroupCode: "valid"},
		{OrgUnitCode: "acme-hq", RoleGroupCode: "disabled"},
		{OrgUnitCode: "acme-hq", RoleGroupCode: "deleted"},
		{OrgUnitCode: "acme-hq", RoleGroupCode: "cross-tenant"},
	}
	if err := db.Create(&bindings).Error; err != nil {
		t.Fatalf("Create(bindings) error = %v", err)
	}

	loaded, err := repository.Load(ctx)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	organizations := loaded.Resources["org-units"]
	if len(organizations) != 1 {
		t.Fatalf("org-units = %+v, want one organization", organizations)
	}
	if got := organizations[0].Values["roleGroupCount"]; got != "4" {
		t.Fatalf("roleGroupCount = %q, want 4 bound groups", got)
	}
	if got := organizations[0].Values["effectiveRoleCount"]; got != "1" {
		t.Fatalf("effectiveRoleCount = %q, want only the enabled same-tenant live role", got)
	}
}

func TestGORMAdminResourceRepositoryNormalizesOperationsResources(t *testing.T) {
	db := openAdminResourceGORMDB(t)
	repository, err := NewGORMAdminResourceRepository(context.Background(), db)
	if err != nil {
		t.Fatalf("NewGORMAdminResourceRepository() error = %v", err)
	}
	for _, model := range []interface{}{
		&gormAdminAuditLog{},
		&gormAdminLoginLog{},
		&gormAdminErrorLog{},
		&gormAdminVersion{},
	} {
		if !db.Migrator().HasTable(model) {
			t.Fatalf("expected normalized operations table for %T", model)
		}
	}
	snapshot := ResourceSnapshot{
		NextID: 1099,
		Resources: map[string][]Record{
			"audit-logs": {
				{ID: "audit-1", Code: "audit-1", Name: "Role update", Status: "recorded", Description: "Role was updated", UpdatedAt: "2026-07-05T00:00:00Z", Values: map[string]string{"actor": "admin", "action": "role.update", "resource": "roles", "createdAt": "2026-07-05T00:00:00Z", "requestId": "req_0123456789abcdef0123456789abcdef", "traceId": "4bf92f3577b34da6a3ce929d0e0e4736", "legacyTraceId": "trace-audit"}},
			},
			"login-logs": {
				{ID: "login-1", Code: "login-1", Name: "Admin login", Status: "recorded", Description: "Admin logged in", UpdatedAt: "2026-07-05T00:01:00Z", Values: map[string]string{"username": "admin", "provider": "demo", "ip": "127.0.0.1", "createdAt": "2026-07-05T00:01:00Z"}},
			},
			"error-logs": {
				{ID: "error-1", Code: "error-1", Name: "Route error", Status: "recorded", Description: "Route returned an error", UpdatedAt: "2026-07-05T00:02:00Z", Values: map[string]string{"level": "error", "message": "route failed", "traceId": "trace-error", "createdAt": "2026-07-05T00:02:00Z"}},
			},
			"versions": {
				{ID: "version-1", Code: "2026.7.5", Name: "2026.7.5", Status: "released", Description: "Initial platform release", UpdatedAt: "2026-07-05T00:03:00Z", Values: map[string]string{"version": "2026.7.5", "channel": "stable", "releasedAt": "2026-07-05T00:03:00Z"}},
			},
			"tenants": {
				{ID: "tenant-acme", Code: "acme", Name: "Acme", Status: "enabled", Description: "Generic tenant", UpdatedAt: "2026-07-05T00:04:00Z", Values: map[string]string{"isolation": "shared"}},
			},
		},
	}

	if _, err := repository.Save(context.Background(), snapshot); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	assertGORMCount(t, db, &gormAdminAuditLog{}, 1)
	assertGORMCount(t, db, &gormAdminLoginLog{}, 1)
	assertGORMCount(t, db, &gormAdminErrorLog{}, 1)
	assertGORMCount(t, db, &gormAdminVersion{}, 1)
	var genericOperationsRows int64
	if err := db.Model(&gormAdminResourceRecord{}).Where("resource IN ?", []string{"audit-logs", "login-logs", "error-logs", "versions"}).Count(&genericOperationsRows).Error; err != nil {
		t.Fatalf("count generic operations rows error = %v", err)
	}
	if genericOperationsRows != 0 {
		t.Fatalf("generic operations rows = %d, want 0", genericOperationsRows)
	}
	assertGORMCount(t, db, &gormAdminResourceRecord{}, 0)

	loaded, err := repository.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !reflect.DeepEqual(loaded.Resources, snapshot.Resources) {
		t.Fatalf("Resources = %#v, want %#v", loaded.Resources, snapshot.Resources)
	}
}

func TestGORMAdminAuditMigrationPreservesHistoricalRowWithoutCanonicalBackfill(t *testing.T) {
	db := openAdminResourceGORMDB(t)
	if err := db.AutoMigrate(&preTask6GORMAdminAuditLog{}); err != nil {
		t.Fatalf("AutoMigrate(pre-Task6 audit log) error = %v", err)
	}
	const legacyTrace = "legacy-secret-email@example.test"
	historical := preTask6GORMAdminAuditLog{
		ID: "audit-historical", Code: "audit-historical", Name: "Historical", Status: "recorded", Description: "pre-Task6 row",
		UpdatedAt: "2026-07-15T00:00:00Z", Actor: "admin", Action: "legacy.action", Resource: "roles", CreatedAt: "2026-07-15T00:00:00Z",
		ValuesJSON: `{"targetId":"role-1","outcome":"success","eventId":"legacy-event","reasonCode":"completed","traceId":"` + legacyTrace + `"}`,
	}
	if err := db.Create(&historical).Error; err != nil {
		t.Fatalf("Create(historical audit) error = %v", err)
	}

	repository, err := NewGORMAdminResourceRepository(context.Background(), db)
	if err != nil {
		t.Fatalf("NewGORMAdminResourceRepository() error = %v", err)
	}
	for _, column := range []string{"request_id", "trace_id"} {
		if !db.Migrator().HasColumn(&gormAdminAuditLog{}, column) {
			t.Fatalf("audit migration missing %s", column)
		}
	}
	var migrated gormAdminAuditLog
	if err := db.First(&migrated, "id = ?", historical.ID).Error; err != nil {
		t.Fatalf("load migrated audit row error = %v", err)
	}
	if migrated.ID != historical.ID || migrated.Code != historical.Code || migrated.ValuesJSON != historical.ValuesJSON || migrated.RequestID != nil || migrated.TraceID != nil {
		t.Fatalf("migrated audit row changed history or backfilled correlation: %+v", migrated)
	}

	loaded, err := repository.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	audits := loaded.Resources["audit-logs"]
	if len(audits) != 1 {
		t.Fatalf("loaded audits = %+v, want historical row", audits)
	}
	if values := audits[0].Values; values["legacyTraceId"] != legacyTrace || values["requestId"] != "" || values["traceId"] != "" {
		t.Fatalf("loaded historical correlation = %+v, want hidden legacy only", values)
	}
}

func TestGORMAdminAuditCorrelationRoundTripPreservesCanonicalAndLegacyValues(t *testing.T) {
	repository, err := NewGORMAdminResourceRepository(context.Background(), openAdminResourceGORMDB(t))
	if err != nil {
		t.Fatalf("NewGORMAdminResourceRepository() error = %v", err)
	}
	snapshot := ResourceSnapshot{NextID: 1001, Resources: map[string][]Record{
		"audit-logs": {{
			ID: "audit-canonical", Code: "audit-canonical", Name: "Canonical", Status: "recorded", Description: "Task6 row", UpdatedAt: "2026-07-16T00:00:00Z",
			Values: map[string]string{
				"actor": "admin", "action": "settings.update", "resource": "settings", "targetId": "setting-1", "outcome": "success",
				"eventId": "domain-event", "reasonCode": "completed", "createdAt": "2026-07-16T00:00:00Z",
				"requestId": "req_0123456789abcdef0123456789abcdef", "traceId": "4bf92f3577b34da6a3ce929d0e0e4736", "legacyTraceId": "historical-marker",
			},
		}},
	}}
	if _, err := repository.Save(context.Background(), snapshot); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	loaded, err := repository.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !reflect.DeepEqual(loaded.Resources["audit-logs"], snapshot.Resources["audit-logs"]) {
		t.Fatalf("audit round trip = %#v, want %#v", loaded.Resources["audit-logs"], snapshot.Resources["audit-logs"])
	}
}

func TestGORMBackedStorePersistsRolePermissionsForDynamicMenus(t *testing.T) {
	db := openAdminResourceGORMDB(t)
	repository, err := NewGORMAdminResourceRepository(context.Background(), db)
	if err != nil {
		t.Fatalf("NewGORMAdminResourceRepository() error = %v", err)
	}
	store, err := NewRepositoryBackedStoreFromCapabilities(repository, core.DefaultManifests())
	if err != nil {
		t.Fatalf("NewRepositoryBackedStoreFromCapabilities() error = %v", err)
	}
	if _, err := store.Update("roles", "role-operator", WriteInput{
		Name:        "Operator",
		Status:      "enabled",
		Description: "GORM operator",
		Values:      map[string]string{"groupCode": "operations", "dataScope": "current_org", "permissions": "admin:user:read"},
	}); err != nil {
		t.Fatalf("Update(role-operator) error = %v", err)
	}

	reloaded, err := NewRepositoryBackedStoreFromCapabilities(repository, core.DefaultManifests())
	if err != nil {
		t.Fatalf("reload store error = %v", err)
	}
	permissions, err := reloaded.List("permissions")
	if err != nil {
		t.Fatalf("List(permissions) error = %v", err)
	}
	for _, policyPrimitive := range []string{"*", "admin:*"} {
		if findRecordByCode(permissions, policyPrimitive) == nil {
			t.Fatalf("reloaded permission catalog missing policy primitive %q", policyPrimitive)
		}
	}
	menus := reloaded.MenuItemsForPrincipal(reloaded.CurrentPrincipal("ops"))
	if !hasMenuRoute(menus, "/users") {
		t.Fatalf("menus after GORM reload = %+v, want /users", menus)
	}
	if hasMenuRoute(menus, "/tenants") {
		t.Fatalf("menus after GORM reload = %+v, want /tenants removed", menus)
	}
}

func TestGORMAdminResourceRepositoryProtectsOrganizationRBACOwnedResources(t *testing.T) {
	ctx := context.Background()
	repository, err := NewGORMAdminResourceRepository(ctx, openAdminResourceGORMDB(t))
	if err != nil {
		t.Fatalf("NewGORMAdminResourceRepository() error = %v", err)
	}
	snapshot := ResourceSnapshot{
		NextID: 10,
		Resources: map[string][]Record{
			"users":       {{ID: "user-admin", Code: "admin", Name: "Admin", Status: "enabled", Values: map[string]string{"roles": "super-admin"}}},
			"org-units":   {{ID: "org-hq", Code: "hq", Name: "HQ", Status: "enabled", Values: map[string]string{"type": "organization", "tenantCode": "tenant-a"}}},
			"roles":       {{ID: "role-super-admin", Code: "super-admin", Name: "Super Admin", Status: "enabled", Values: map[string]string{"groupCode": "system", "permissions": "*"}}},
			"role-groups": {{ID: "group-system", Code: "system", Name: "System", Status: "enabled", Values: map[string]string{"sortOrder": "10"}}},
			"settings":    {{ID: "setting-brand", Code: "brand", Name: "Brand", Status: "enabled"}},
		},
	}
	committed, err := repository.Save(ctx, snapshot)
	if err != nil {
		t.Fatalf("Save(seed) error = %v", err)
	}
	snapshot, err = repository.Load(ctx)
	if err != nil {
		t.Fatalf("Load(seed) error = %v", err)
	}
	if snapshot.Revision != committed {
		t.Fatalf("seed revision = %d, want %d", snapshot.Revision, committed)
	}
	owned := repository.WithOrganizationRBACOwnership()

	snapshot.Resources["settings"][0].Name = "Updated Brand"
	committed, err = owned.Save(ctx, snapshot)
	if err != nil {
		t.Fatalf("Save(unrelated) error = %v", err)
	}
	snapshot.Revision = committed

	mutated := snapshot
	mutated.Resources = cloneResourceMap(snapshot.Resources)
	mutated.Resources["users"][0].Name = "Changed Outside Domain"
	if _, err := owned.Save(ctx, mutated); !errors.Is(err, ErrDomainOwnedMutation) {
		t.Fatalf("Save(domain-owned mutation) error = %v, want ErrDomainOwnedMutation", err)
	}
	loaded, err := owned.Load(ctx)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got := loaded.Resources["users"][0].Name; got != "Admin" {
		t.Fatalf("owned user name = %q, want Admin", got)
	}
	if loaded.Revision != committed {
		t.Fatalf("revision = %d, want %d", loaded.Revision, committed)
	}
}

func TestGORMOrganizationRBACRepositoryExcludesGenericCapabilitySeeds(t *testing.T) {
	repository, err := NewGORMAdminResourceRepository(context.Background(), openAdminResourceGORMDB(t))
	if err != nil {
		t.Fatalf("NewGORMAdminResourceRepository() error = %v", err)
	}
	store, err := NewRepositoryBackedStoreFromCapabilities(repository.WithOrganizationRBACOwnership(), core.DefaultManifests())
	if err != nil {
		t.Fatalf("NewRepositoryBackedStoreFromCapabilities() error = %v", err)
	}
	for _, resource := range organizationRBACOwnedResources {
		records, err := store.List(resource)
		if err != nil {
			t.Fatalf("List(%s) error = %v", resource, err)
		}
		if len(records) != 0 {
			t.Fatalf("List(%s) = %+v, want no generic capability seeds", resource, records)
		}
	}
	permissions, err := store.List("permissions")
	if err != nil || len(permissions) == 0 {
		t.Fatalf("List(permissions) = %+v, %v; want non-domain capability seeds", permissions, err)
	}
}

func openAdminResourceGORMDB(t *testing.T) *gorm.DB {
	t.Helper()
	return openAdminResourceGORMDBAt(t, filepath.Join(t.TempDir(), "admin-resources.db"))
}

func openAdminResourceGORMDBAt(t *testing.T, dsn string) *gorm.DB {
	t.Helper()
	db, err := storage.OpenGORM(storage.Config{
		Driver: "sqlite",
		DSN:    dsn,
	})
	if err != nil {
		t.Fatalf("OpenGORM(sqlite) error = %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func gormQueryReadsRevision(tx *gorm.DB) bool {
	if tx.Statement == nil || tx.Statement.Table != adminResourceStateTable {
		return false
	}
	for _, value := range tx.Statement.Vars {
		if value == "revision" {
			return true
		}
	}
	return false
}

func assertGORMCount(t *testing.T, db *gorm.DB, model interface{}, want int64) {
	t.Helper()
	var got int64
	if err := db.Model(model).Count(&got).Error; err != nil {
		t.Fatalf("count %T error = %v", model, err)
	}
	if got != want {
		t.Fatalf("count %T = %d, want %d", model, got, want)
	}
}
