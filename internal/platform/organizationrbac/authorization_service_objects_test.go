package organizationrbac

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"platform-go/internal/platform/kernel"
	"platform-go/internal/platform/serviceobject"

	"gorm.io/gorm"
)

func TestRolePermissionReplaceRollsBackRelationAndRevisionTogether(t *testing.T) {
	db, repository := prepareOrganizationRBACTestRepository(t)
	seedOrganizationRBAC(t, db)
	seedRolePermissionCatalog(t, db)
	if err := db.Create(&gormRolePermission{RoleCode: "operator", Permission: "admin:user:read"}).Error; err != nil {
		t.Fatal(err)
	}
	impact, err := repository.PreviewRolePermissions(context.Background(), "operator", []string{"admin:user:update", "page:user:export"})
	if err != nil {
		t.Fatal(err)
	}
	trigger := fmt.Sprintf(`CREATE TRIGGER fail_role_permission BEFORE INSERT ON %s WHEN NEW.permission = 'page:user:export' BEGIN SELECT RAISE(FAIL, 'blocked'); END`, rolePermissionsTable)
	if err := db.Exec(trigger).Error; err != nil {
		t.Fatal(err)
	}
	_, _, err = repository.ReplaceRolePermissions(context.Background(), ReplaceRolePermissionsRequest{
		RoleCode: "operator", PermissionCodes: impact.ProposedPermissionCodes, ExpectedRevision: impact.ExpectedRevision,
		ActorID: "admin", ChangedAt: time.Date(2026, 7, 15, 15, 30, 0, 0, time.UTC),
	})
	if !errors.Is(err, ErrRepositoryFailed) {
		t.Fatalf("ReplaceRolePermissions() error = %v, want ErrRepositoryFailed", err)
	}
	var rows []gormRolePermission
	if err := db.Where("role_code = ?", "operator").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Permission != "admin:user:read" {
		t.Fatalf("role permissions after rollback = %+v", rows)
	}
	if revision, err := repository.CurrentGlobalRevision(context.Background()); err != nil || revision != 0 {
		t.Fatalf("global revision after rollback = %d, error = %v", revision, err)
	}
}

func TestRolePermissionPrepareImpactReplaceIsAtomicOwnerScopedAndReplayable(t *testing.T) {
	db, repository := prepareOrganizationRBACTestRepository(t)
	seedOrganizationRBAC(t, db)
	seedRolePermissionCatalog(t, db)
	if err := db.Create(&gormRolePermission{RoleCode: "operator", Permission: "admin:user:read"}).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 15, 16, 0, 0, 0, time.UTC)
	executor, err := NewServiceObjectExecutor(repository, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	execution := kernel.ExecutionContext{Context: context.Background(), Actor: kernel.Actor{Username: "admin", Kind: kernel.ActorKindUser}}
	prepare, err := executor.ExecuteDomainCommand(context.Background(), serviceobject.DomainCommandPlan{
		Definition: domainDefinitionByID(t, RolePermissionPrepareCommandID), Execution: execution,
		Arguments: serviceobject.ValidatedArguments{
			"roleCode": "operator", "permissionCodes": []string{"admin:user:update", "page:user:export", "admin:user:update"},
		},
	})
	if err != nil {
		t.Fatalf("prepare error = %v", err)
	}
	previewID := prepare.Values["previewId"].(string)
	impactPlan := serviceobject.QueryPlan{
		Definition: queryDefinitionByID(t, RolePermissionImpactQueryID), Execution: execution,
		AST: serviceobject.QueryAST{Predicates: []serviceobject.Predicate{{Field: "previewId", Operator: serviceobject.PredicateEqual, Value: previewID}}},
	}
	impact, err := executor.ExecuteQuery(context.Background(), impactPlan)
	if err != nil || len(impact.Items) != 1 || impact.Items[0]["severity"] != "medium" || impact.Items[0]["expectedRevision"] != int64(0) {
		t.Fatalf("impact = %+v, error = %v", impact, err)
	}
	impactPlan.Execution.Actor.Username = "other-admin"
	if _, err := executor.ExecuteQuery(context.Background(), impactPlan); !errors.Is(err, serviceobject.ErrObjectUnavailable) {
		t.Fatalf("cross-owner impact error = %v", err)
	}

	applyPlan := serviceobject.DomainCommandPlan{
		Definition: domainDefinitionByID(t, RolePermissionsReplaceCommandID), Execution: execution,
		Arguments: serviceobject.ValidatedArguments{
			"previewId": previewID, "expectedRevision": prepare.Values["expectedRevision"], "impactHash": prepare.Values["impactHash"],
		},
	}
	applied, err := executor.ExecuteDomainCommand(context.Background(), applyPlan)
	if err != nil || applied.Values["revision"] != int64(1) {
		t.Fatalf("apply = %+v, error = %v", applied, err)
	}
	var rows []gormRolePermission
	if err := db.Where("role_code = ?", "operator").Order("permission").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	permissions := make([]string, 0, len(rows))
	for _, row := range rows {
		permissions = append(permissions, row.Permission)
	}
	if want := []string{"admin:user:update", "page:user:export"}; !reflect.DeepEqual(permissions, want) {
		t.Fatalf("role permissions = %v, want %v", permissions, want)
	}
	var audit gormOrganizationRBACAuditEvent
	if err := db.Where("preview_id = ?", previewID).Take(&audit).Error; err != nil {
		t.Fatal(err)
	}
	if audit.Action != RolePermissionsReplaceCommandID || audit.TenantCode != "acme" || audit.OrgUnitCode != "" || audit.ConflictCount != 0 {
		t.Fatalf("audit = %+v", audit)
	}
	if replay, err := executor.ExecuteDomainCommand(context.Background(), applyPlan); err != nil || replay.Values["revision"] != int64(1) {
		t.Fatalf("replay = %+v, error = %v", replay, err)
	}
	var auditCount int64
	if err := db.Model(&gormOrganizationRBACAuditEvent{}).Where("preview_id = ?", previewID).Count(&auditCount).Error; err != nil || auditCount != 1 {
		t.Fatalf("audit count = %d, error = %v", auditCount, err)
	}
}

func TestRolePermissionPrepareRejectsIneligibleRoleOrPermission(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, *gorm.DB)
		code   string
	}{
		{name: "disabled permission", code: "admin:user:read", mutate: func(t *testing.T, db *gorm.DB) {
			if err := db.Model(&gormPermission{}).Where("code = ?", "admin:user:read").Update("status", "disabled").Error; err != nil {
				t.Fatal(err)
			}
		}},
		{name: "deleted permission", code: "admin:user:read", mutate: func(t *testing.T, db *gorm.DB) {
			if err := db.Create(&gormResourceLifecycle{Resource: "permissions", RecordID: "permission-user-read", DeletedAt: "2026-07-15T16:00:00Z"}).Error; err != nil {
				t.Fatal(err)
			}
		}},
		{name: "invalid resource type", code: "invalid:permission", mutate: func(t *testing.T, db *gorm.DB) {}},
		{name: "disabled role", code: "admin:user:read", mutate: func(t *testing.T, db *gorm.DB) {
			if err := db.Model(&gormRole{}).Where("code = ?", "operator").Update("status", "disabled").Error; err != nil {
				t.Fatal(err)
			}
		}},
		{name: "deleted role", code: "admin:user:read", mutate: func(t *testing.T, db *gorm.DB) {
			if err := db.Create(&gormResourceLifecycle{Resource: "roles", RecordID: "role-operator", DeletedAt: "2026-07-15T16:00:00Z"}).Error; err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db, repository := prepareOrganizationRBACTestRepository(t)
			seedOrganizationRBAC(t, db)
			seedRolePermissionCatalog(t, db)
			tc.mutate(t, db)
			executor, err := NewServiceObjectExecutor(repository, nil)
			if err != nil {
				t.Fatal(err)
			}
			_, err = executor.ExecuteDomainCommand(context.Background(), serviceobject.DomainCommandPlan{
				Definition: domainDefinitionByID(t, RolePermissionPrepareCommandID),
				Execution:  kernel.ExecutionContext{Context: context.Background(), Actor: kernel.Actor{Username: "admin", Kind: kernel.ActorKindUser}},
				Arguments:  serviceobject.ValidatedArguments{"roleCode": "operator", "permissionCodes": []string{tc.code}},
			})
			if !errors.Is(err, serviceobject.ErrValidation) {
				t.Fatalf("prepare error = %v, want ErrValidation", err)
			}
		})
	}
}

func TestRolePermissionApplyRejectsExpiredOrStalePreview(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*testing.T, *gorm.DB, *time.Time)
	}{
		{name: "expired", mutate: func(_ *testing.T, _ *gorm.DB, now *time.Time) {
			*now = now.Add(defaultOrganizationRoleGroupPreviewDuration + time.Second)
		}},
		{name: "stale revision", mutate: func(t *testing.T, db *gorm.DB, _ *time.Time) {
			if _, err := advanceGlobalRevision(db, 0); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, repository := prepareOrganizationRBACTestRepository(t)
			seedOrganizationRBAC(t, db)
			seedRolePermissionCatalog(t, db)
			now := time.Date(2026, 7, 15, 17, 0, 0, 0, time.UTC)
			executor, err := NewServiceObjectExecutor(repository, func() time.Time { return now })
			if err != nil {
				t.Fatal(err)
			}
			execution := kernel.ExecutionContext{Context: context.Background(), Actor: kernel.Actor{Username: "admin", Kind: kernel.ActorKindUser}}
			prepare, err := executor.ExecuteDomainCommand(context.Background(), serviceobject.DomainCommandPlan{
				Definition: domainDefinitionByID(t, RolePermissionPrepareCommandID), Execution: execution,
				Arguments: serviceobject.ValidatedArguments{"roleCode": "operator", "permissionCodes": []string{"admin:user:update"}},
			})
			if err != nil {
				t.Fatal(err)
			}
			tc.mutate(t, db, &now)
			_, err = executor.ExecuteDomainCommand(context.Background(), serviceobject.DomainCommandPlan{
				Definition: domainDefinitionByID(t, RolePermissionsReplaceCommandID), Execution: execution,
				Arguments: serviceobject.ValidatedArguments{
					"previewId": prepare.Values["previewId"], "expectedRevision": prepare.Values["expectedRevision"], "impactHash": prepare.Values["impactHash"],
				},
			})
			if !errors.Is(err, serviceobject.ErrConflict) {
				t.Fatalf("apply error = %v, want ErrConflict", err)
			}
		})
	}
}

func TestRolePermissionServiceObjectInputsExcludeTenantAndPhysicalFields(t *testing.T) {
	for _, definition := range []serviceobject.DomainCommandDefinition{
		domainDefinitionByID(t, RolePermissionPrepareCommandID), domainDefinitionByID(t, RolePermissionsReplaceCommandID),
	} {
		for _, argument := range definition.Arguments {
			switch argument.Name {
			case "tenant", "tenantCode", "datasource", "database", "schema", "shard", "field", "operator":
				t.Fatalf("domain command %s exposes forbidden argument %q", definition.ID, argument.Name)
			}
		}
	}
}

func seedRolePermissionCatalog(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, permission := range []gormPermission{
		{ID: "permission-user-read", Code: "admin:user:read", Status: StatusEnabled, ResourceType: PermissionResourceTypeAPI},
		{ID: "permission-user-update", Code: "admin:user:update", Status: StatusEnabled, ResourceType: PermissionResourceTypeAPI},
		{ID: "permission-page-user-export", Code: "page:user:export", Status: StatusEnabled, ResourceType: PermissionResourceTypePageButton},
		{ID: "permission-invalid", Code: "invalid:permission", Status: StatusEnabled, ResourceType: "unknown"},
	} {
		if err := db.Create(&permission).Error; err != nil {
			t.Fatal(err)
		}
	}
}
