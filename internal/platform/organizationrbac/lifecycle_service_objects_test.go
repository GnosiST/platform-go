package organizationrbac

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"platform-go/internal/platform/kernel"
	"platform-go/internal/platform/serviceobject"

	"gorm.io/gorm"
)

func TestResourceLifecyclePrepareImpactApplyIsOwnerScopedAndAudited(t *testing.T) {
	db, repository := prepareOrganizationRBACTestRepository(t)
	seedOrganizationRBAC(t, db)
	now := time.Date(2026, 7, 15, 19, 0, 0, 0, time.UTC)
	if err := db.Create(&gormUser{ID: "user-alice", Code: "alice", ScopeType: string(ScopeTenant), TenantCode: "acme", OrgUnitCode: "acme-hq", Status: StatusEnabled}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&gormUserRole{UserID: "user-alice", RoleCode: "operator"}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repository.ReplaceOrgUnitRoleGroups(context.Background(), ReplaceOrgUnitRoleGroupsRequest{
		OrgUnitCode: "acme-hq", RoleGroupCodes: []string{"acme-ops"}, ExpectedRevision: 0, ActorID: "admin", ChangedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	executor, err := NewServiceObjectExecutor(repository, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	execution := kernel.ExecutionContext{Context: context.Background(), Actor: kernel.Actor{Username: "admin", Kind: kernel.ActorKindUser}}
	prepare, err := executor.ExecuteDomainCommand(context.Background(), serviceobject.DomainCommandPlan{
		Definition: domainDefinitionByID(t, ResourceLifecyclePrepareCommandID), Execution: execution,
		Arguments: serviceobject.ValidatedArguments{
			"resource": "users", "resourceCode": "alice", "operation": LifecycleOperationDelete,
			"retentionDays": int64(30), "policyVersion": int64(1),
			"remediations": []serviceobject.RoleRemediation{{UserCode: "alice", RoleCode: "operator", Action: serviceobject.RoleRemediationRemove}},
		},
	})
	if err != nil {
		t.Fatalf("prepare error = %v", err)
	}
	previewID := prepare.Values["previewId"].(string)
	impactPlan := serviceobject.QueryPlan{
		Definition: queryDefinitionByID(t, ResourceLifecycleImpactQueryID), Execution: execution,
		AST: serviceobject.QueryAST{Predicates: []serviceobject.Predicate{{Field: "previewId", Operator: serviceobject.PredicateEqual, Value: previewID}}},
	}
	impact, err := executor.ExecuteQuery(context.Background(), impactPlan)
	if err != nil || len(impact.Items) != 1 || impact.Items[0]["operation"] != LifecycleOperationDelete || impact.Items[0]["referenceCount"] != int64(1) {
		t.Fatalf("impact = %+v, error = %v", impact, err)
	}
	impactPlan.Execution.Actor.Username = "other-admin"
	if _, err := executor.ExecuteQuery(context.Background(), impactPlan); !errors.Is(err, serviceobject.ErrObjectUnavailable) {
		t.Fatalf("cross-owner impact error = %v", err)
	}
	applyArguments := serviceobject.ValidatedArguments{
		"previewId": previewID, "expectedRevision": prepare.Values["expectedRevision"], "impactHash": prepare.Values["impactHash"],
	}
	if _, err := executor.ExecuteDomainCommand(context.Background(), serviceobject.DomainCommandPlan{
		Definition: domainDefinitionByID(t, ResourceLifecycleApplyCommandID),
		Execution:  kernel.ExecutionContext{Context: context.Background(), Actor: kernel.Actor{Username: "other-admin", Kind: kernel.ActorKindUser}},
		Arguments:  applyArguments,
	}); !errors.Is(err, serviceobject.ErrObjectUnavailable) {
		t.Fatalf("cross-owner apply error = %v, want ErrObjectUnavailable", err)
	}
	tamperedArguments := serviceobject.ValidatedArguments{
		"previewId": previewID, "expectedRevision": prepare.Values["expectedRevision"], "impactHash": "mismatched-impact-hash",
	}
	if _, err := executor.ExecuteDomainCommand(context.Background(), serviceobject.DomainCommandPlan{
		Definition: domainDefinitionByID(t, ResourceLifecycleApplyCommandID), Execution: execution, Arguments: tamperedArguments,
	}); !errors.Is(err, serviceobject.ErrConflict) {
		t.Fatalf("mismatched-hash apply error = %v, want ErrConflict", err)
	}
	var unchanged gormUser
	if err := db.Where("code = ?", "alice").Take(&unchanged).Error; err != nil || unchanged.Status != StatusEnabled {
		t.Fatalf("rejected apply changed user = %+v error=%v", unchanged, err)
	}
	apply, err := executor.ExecuteDomainCommand(context.Background(), serviceobject.DomainCommandPlan{
		Definition: domainDefinitionByID(t, ResourceLifecycleApplyCommandID), Execution: execution,
		Arguments: applyArguments,
	})
	if err != nil || apply.Values["revision"] != int64(2) {
		t.Fatalf("apply = %+v, error = %v", apply, err)
	}
	var audit gormOrganizationRBACAuditEvent
	if err := db.Where("preview_id = ?", previewID).Take(&audit).Error; err != nil || audit.Action != ResourceLifecycleApplyCommandID || audit.TenantCode != "acme" || audit.OrgUnitCode != "acme-hq" {
		t.Fatalf("audit = %+v error=%v", audit, err)
	}
	if replay, err := executor.ExecuteDomainCommand(context.Background(), serviceobject.DomainCommandPlan{
		Definition: domainDefinitionByID(t, ResourceLifecycleApplyCommandID), Execution: execution,
		Arguments: serviceobject.ValidatedArguments{
			"previewId": previewID, "expectedRevision": prepare.Values["expectedRevision"], "impactHash": prepare.Values["impactHash"],
		},
	}); err != nil || replay.Values["revision"] != int64(2) {
		t.Fatalf("replay = %+v, error = %v", replay, err)
	}
}

func TestResourceLifecycleApplyRejectsExpiredPreview(t *testing.T) {
	db, repository := prepareOrganizationRBACTestRepository(t)
	seedOrganizationRBAC(t, db)
	now := time.Date(2026, 7, 15, 20, 0, 0, 0, time.UTC)
	executor, err := NewServiceObjectExecutor(repository, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	execution := kernel.ExecutionContext{Context: context.Background(), Actor: kernel.Actor{Username: "admin", Kind: kernel.ActorKindUser}}
	prepare, err := executor.ExecuteDomainCommand(context.Background(), serviceobject.DomainCommandPlan{
		Definition: domainDefinitionByID(t, ResourceLifecyclePrepareCommandID), Execution: execution,
		Arguments: serviceobject.ValidatedArguments{"resource": "role-groups", "resourceCode": "acme-ops", "operation": LifecycleOperationDisable},
	})
	if err != nil {
		t.Fatal(err)
	}
	executor.now = func() time.Time { return now.Add(defaultOrganizationRoleGroupPreviewDuration + time.Second) }
	if _, err := executor.ExecuteDomainCommand(context.Background(), serviceobject.DomainCommandPlan{
		Definition: domainDefinitionByID(t, ResourceLifecycleApplyCommandID), Execution: execution,
		Arguments: serviceobject.ValidatedArguments{
			"previewId": prepare.Values["previewId"], "expectedRevision": prepare.Values["expectedRevision"], "impactHash": prepare.Values["impactHash"],
		},
	}); !errors.Is(err, serviceobject.ErrConflict) {
		t.Fatalf("expired apply error = %v, want ErrConflict", err)
	}
	var group gormRoleGroup
	if err := db.Where("code = ?", "acme-ops").Take(&group).Error; err != nil || group.Status != StatusEnabled {
		t.Fatalf("expired preview changed group = %+v error=%v", group, err)
	}
}

func TestResourceLifecycleApplyRejectsStaleRevisionWithoutMutation(t *testing.T) {
	db, repository := prepareOrganizationRBACTestRepository(t)
	seedOrganizationRBAC(t, db)
	if err := db.Create(&gormPermission{
		ID: "permission-user-read", Code: "admin:user:read", Name: "Read users", Status: StatusEnabled,
		ResourceType: PermissionResourceTypeAPI, Resource: "users", Action: "read", ValuesJSON: "{}",
	}).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 15, 20, 30, 0, 0, time.UTC)
	executor, err := NewServiceObjectExecutor(repository, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	execution := kernel.ExecutionContext{Context: context.Background(), Actor: kernel.Actor{Username: "admin", Kind: kernel.ActorKindUser}}
	prepare, err := executor.ExecuteDomainCommand(context.Background(), serviceobject.DomainCommandPlan{
		Definition: domainDefinitionByID(t, ResourceLifecyclePrepareCommandID), Execution: execution,
		Arguments: serviceobject.ValidatedArguments{
			"resource": "permissions", "resourceCode": "admin:user:read", "operation": LifecycleOperationDelete,
			"retentionDays": int64(30), "policyVersion": int64(1),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.ReplaceOrgUnitRoleGroups(context.Background(), ReplaceOrgUnitRoleGroupsRequest{
		OrgUnitCode: "acme-hq", RoleGroupCodes: []string{"acme-ops"}, ExpectedRevision: 0, ActorID: "admin", ChangedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := executor.ExecuteDomainCommand(context.Background(), serviceobject.DomainCommandPlan{
		Definition: domainDefinitionByID(t, ResourceLifecycleApplyCommandID), Execution: execution,
		Arguments: serviceobject.ValidatedArguments{
			"previewId": prepare.Values["previewId"], "expectedRevision": prepare.Values["expectedRevision"], "impactHash": prepare.Values["impactHash"],
		},
	}); !errors.Is(err, serviceobject.ErrConflict) {
		t.Fatalf("stale apply error = %v, want ErrConflict", err)
	}
	var permission gormPermission
	if err := db.Where("code = ?", "admin:user:read").Take(&permission).Error; err != nil || permission.Status != StatusEnabled {
		t.Fatalf("stale apply changed permission = %+v error=%v", permission, err)
	}
}

func TestResourceLifecycleServiceObjectMatrixAllGovernedResources(t *testing.T) {
	resources := []struct {
		name string
		code string
	}{
		{name: "org-units", code: "acme-hq"},
		{name: "role-groups", code: "lifecycle-group"},
		{name: "roles", code: "auditor"},
		{name: "users", code: "lifecycle-user"},
		{name: "menus", code: "lifecycle-menu"},
		{name: "permissions", code: "admin:lifecycle:read"},
	}
	for _, resource := range resources {
		resource := resource
		t.Run(resource.name+"/success", func(t *testing.T) {
			_, _, executor, execution, prepare := prepareLifecycleServiceObjectCase(t, resource.name, resource.code)
			apply, err := executor.ExecuteDomainCommand(context.Background(), serviceobject.DomainCommandPlan{
				Definition: domainDefinitionByID(t, ResourceLifecycleApplyCommandID), Execution: execution,
				Arguments: serviceobject.ValidatedArguments{
					"previewId": prepare.Values["previewId"], "expectedRevision": prepare.Values["expectedRevision"], "impactHash": prepare.Values["impactHash"],
				},
			})
			if err != nil || apply.Values["revision"] != int64(1) {
				t.Fatalf("apply = %+v, error = %v", apply, err)
			}
		})
		for _, scenario := range []string{"cross-owner", "hash-mismatch", "stale-revision", "expired"} {
			scenario := scenario
			t.Run(resource.name+"/"+scenario, func(t *testing.T) {
				db, repository, executor, execution, prepare := prepareLifecycleServiceObjectCase(t, resource.name, resource.code)
				beforeRevision := currentOrganizationRBACRevision(t, db)
				beforeAuditCount := countOrganizationRBACAudits(t, db)
				applyExecution := execution
				arguments := serviceobject.ValidatedArguments{
					"previewId": prepare.Values["previewId"], "expectedRevision": prepare.Values["expectedRevision"], "impactHash": prepare.Values["impactHash"],
				}
				switch scenario {
				case "cross-owner":
					applyExecution.Actor.Username = "other-admin"
				case "hash-mismatch":
					arguments["impactHash"] = "mismatched-impact-hash"
				case "stale-revision":
					if _, err := repository.ReplaceOrgUnitRoleGroups(context.Background(), ReplaceOrgUnitRoleGroupsRequest{
						OrgUnitCode: "acme-hq", RoleGroupCodes: []string{"acme-ops"}, ExpectedRevision: 0, ActorID: "admin", ChangedAt: time.Date(2026, 7, 15, 21, 0, 0, 0, time.UTC),
					}); err != nil {
						t.Fatal(err)
					}
				case "expired":
					executor.now = func() time.Time {
						return time.Date(2026, 7, 15, 21, 0, 0, 0, time.UTC).Add(defaultOrganizationRoleGroupPreviewDuration + time.Second)
					}
				}
				if _, err := executor.ExecuteDomainCommand(context.Background(), serviceobject.DomainCommandPlan{
					Definition: domainDefinitionByID(t, ResourceLifecycleApplyCommandID), Execution: applyExecution, Arguments: arguments,
				}); !errors.Is(err, serviceobject.ErrConflict) && !errors.Is(err, serviceobject.ErrObjectUnavailable) {
					t.Fatalf("%s apply error = %v, want conflict/unavailable", scenario, err)
				}
				if got := currentOrganizationRBACRevision(t, db); scenario != "stale-revision" && got != beforeRevision {
					t.Fatalf("%s changed revision from %d to %d", scenario, beforeRevision, got)
				}
				if got := countOrganizationRBACAudits(t, db); got != beforeAuditCount {
					t.Fatalf("%s changed audit count from %d to %d", scenario, beforeAuditCount, got)
				}
			})
		}
	}
}

func prepareLifecycleServiceObjectCase(t *testing.T, resource, code string) (*gorm.DB, *GORMRepository, *ServiceObjectExecutor, kernel.ExecutionContext, serviceobject.CommandResult) {
	t.Helper()
	db, repository := prepareOrganizationRBACTestRepository(t)
	seedOrganizationRBAC(t, db)
	rows := map[string]any{
		"org-units":   nil,
		"role-groups": &gormRoleGroup{ID: "group-lifecycle", Code: code, Name: "Lifecycle group", ScopeType: string(ScopePlatform), Status: StatusEnabled},
		"roles":       nil,
		"users":       &gormUser{ID: "user-lifecycle", Code: code, ScopeType: string(ScopeTenant), TenantCode: "acme", OrgUnitCode: "acme-hq", Status: StatusEnabled},
		"menus":       &gormMenu{ID: "menu-lifecycle", Code: code, Name: "Lifecycle menu", Status: StatusEnabled, NodeType: "page", Route: "/lifecycle", ComponentKey: "lifecycle"},
		"permissions": &gormPermission{ID: "permission-lifecycle", Code: code, Name: "Lifecycle permission", Status: StatusEnabled, ResourceType: PermissionResourceTypeAPI, Resource: "lifecycle", Action: "read"},
	}
	if rows[resource] != nil {
		if err := db.Create(rows[resource]).Error; err != nil {
			t.Fatal(err)
		}
	}
	now := time.Date(2026, 7, 15, 21, 0, 0, 0, time.UTC)
	executor, err := NewServiceObjectExecutor(repository, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	definitionArgs := serviceobject.ValidatedArguments{"resource": resource, "resourceCode": code, "operation": LifecycleOperationDelete, "retentionDays": int64(30), "policyVersion": int64(1)}
	execution := kernel.ExecutionContext{Context: context.Background(), Actor: kernel.Actor{Username: "admin", Kind: kernel.ActorKindUser}}
	prepare, err := executor.ExecuteDomainCommand(context.Background(), serviceobject.DomainCommandPlan{
		Definition: domainDefinitionByID(t, ResourceLifecyclePrepareCommandID), Execution: execution, Arguments: definitionArgs,
	})
	if err != nil {
		t.Fatalf("prepare %s/%s error = %v", resource, code, err)
	}
	return db, repository, executor, execution, prepare
}

func currentOrganizationRBACRevision(t *testing.T, db *gorm.DB) uint64 {
	t.Helper()
	var state gormAdminResourceState
	if err := db.Where("key = ?", "revision").Take(&state).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return 0
	} else if err != nil {
		t.Fatal(err)
	}
	parsed, err := strconv.ParseUint(state.Value, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func countOrganizationRBACAudits(t *testing.T, db *gorm.DB) int {
	t.Helper()
	var count int64
	if err := db.Model(&gormOrganizationRBACAuditEvent{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	return int(count)
}
