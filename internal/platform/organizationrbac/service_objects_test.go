package organizationrbac

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/GnosiST/platform-go/internal/platform/kernel"
	"github.com/GnosiST/platform-go/internal/platform/serviceobject"
)

func TestOrganizationRoleGroupPrepareImpactApplyIsOwnerScopedAndAtomic(t *testing.T) {
	db, repository := prepareOrganizationRBACTestRepository(t)
	seedOrganizationRBAC(t, db)
	if err := db.Create(&gormUser{ID: "user-alice", Code: "alice", ScopeType: string(ScopeTenant), TenantCode: "acme", OrgUnitCode: "acme-hq", Status: StatusEnabled}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&gormUserRole{UserID: "user-alice", RoleCode: "operator"}).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	executor, err := NewServiceObjectExecutor(repository, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	execution := kernel.ExecutionContext{Context: context.Background(), Actor: kernel.Actor{Username: "admin", Kind: kernel.ActorKindUser}}
	prepare, err := executor.ExecuteDomainCommand(context.Background(), serviceobject.DomainCommandPlan{
		Definition: domainDefinitionByID(t, OrganizationRoleGroupPrepareCommandID), Execution: execution,
		Arguments: serviceobject.ValidatedArguments{
			"orgUnitCode": "acme-hq", "roleGroupCodes": []string{"acme-audit"},
			"remediations": []serviceobject.RoleRemediation{{
				UserCode: "alice", RoleCode: "operator", Action: serviceobject.RoleRemediationReplace, ReplacementRoleCode: "auditor",
			}},
		},
	})
	if err != nil {
		t.Fatalf("prepare error = %v", err)
	}
	previewID := prepare.Values["previewId"].(string)
	impactHash := prepare.Values["impactHash"].(string)
	expectedRevision := prepare.Values["expectedRevision"].(int64)

	impactPlan := serviceobject.QueryPlan{
		Definition: queryDefinitionByID(t, OrganizationRoleGroupImpactQueryID), Execution: execution,
		AST: serviceobject.QueryAST{Predicates: []serviceobject.Predicate{{Field: "previewId", Operator: serviceobject.PredicateEqual, Value: previewID}}},
	}
	impact, err := executor.ExecuteQuery(context.Background(), impactPlan)
	if err != nil || len(impact.Items) != 1 || impact.Items[0]["conflictCount"] != int64(1) {
		t.Fatalf("impact = %+v, error = %v", impact, err)
	}
	conflicts, err := executor.ExecuteQuery(context.Background(), serviceobject.QueryPlan{
		Definition: queryDefinitionByID(t, OrganizationRoleGroupConflictsQueryID), Execution: execution, Page: 1, PageSize: 100,
		AST: serviceobject.QueryAST{Predicates: []serviceobject.Predicate{{Field: "previewId", Operator: serviceobject.PredicateEqual, Value: previewID}}},
	})
	if err != nil || len(conflicts.Items) != 1 || conflicts.Items[0]["userCode"] != "alice" || conflicts.Items[0]["roleCode"] != "operator" {
		t.Fatalf("conflicts = %+v, error = %v", conflicts, err)
	}
	impactPlan.Definition = queryDefinitionByID(t, UserOrganizationImpactQueryID)
	if _, err := executor.ExecuteQuery(context.Background(), impactPlan); !errors.Is(err, serviceobject.ErrObjectUnavailable) {
		t.Fatalf("cross-operation impact error = %v", err)
	}
	impactPlan.Definition = queryDefinitionByID(t, OrganizationRoleGroupImpactQueryID)
	impactPlan.Execution.Actor.Username = "other-admin"
	if _, err := executor.ExecuteQuery(context.Background(), impactPlan); !errors.Is(err, serviceobject.ErrObjectUnavailable) {
		t.Fatalf("cross-owner impact error = %v", err)
	}

	apply, err := executor.ExecuteDomainCommand(context.Background(), serviceobject.DomainCommandPlan{
		Definition: domainDefinitionByID(t, OrganizationRoleGroupsReplaceCommandID), Execution: execution,
		Arguments: serviceobject.ValidatedArguments{"previewId": previewID, "expectedRevision": expectedRevision, "impactHash": impactHash},
	})
	if err != nil || apply.Values["applied"] != true || apply.Values["revision"] != int64(1) {
		t.Fatalf("apply = %+v, error = %v", apply, err)
	}
	loaded, err := repository.LoadOrgUnitRoleGroups(context.Background(), "acme-hq")
	if err != nil || len(loaded.RoleGroupCodes) != 1 || loaded.RoleGroupCodes[0] != "acme-audit" {
		t.Fatalf("bindings = %+v, error = %v", loaded, err)
	}
	var roles []gormUserRole
	if err := db.Where("user_id = ?", "user-alice").Find(&roles).Error; err != nil || len(roles) != 1 || roles[0].RoleCode != "auditor" {
		t.Fatalf("user roles = %+v, error = %v", roles, err)
	}
	if revision, err := repository.CurrentGlobalRevision(context.Background()); err != nil || revision != 1 {
		t.Fatalf("global revision = %d, error = %v", revision, err)
	}
	var audits int64
	if err := db.Model(&gormOrganizationRBACAuditEvent{}).Where("preview_id = ?", previewID).Count(&audits).Error; err != nil || audits != 1 {
		t.Fatalf("audit count = %d, error = %v", audits, err)
	}
	if replay, err := executor.ExecuteDomainCommand(context.Background(), serviceobject.DomainCommandPlan{
		Definition: domainDefinitionByID(t, OrganizationRoleGroupsReplaceCommandID), Execution: execution,
		Arguments: serviceobject.ValidatedArguments{"previewId": previewID, "expectedRevision": expectedRevision, "impactHash": impactHash},
	}); err != nil || replay.Values["revision"] != int64(1) {
		t.Fatalf("apply replay = %+v, error = %v", replay, err)
	}
}

func TestUserOrganizationPrepareApplyUsesDerivedTenantAndGlobalRevision(t *testing.T) {
	db, repository := prepareOrganizationRBACTestRepository(t)
	seedOrganizationRBAC(t, db)
	now := time.Date(2026, 7, 15, 15, 0, 0, 0, time.UTC)
	if err := db.Create(&gormOrganization{ID: "org-acme-branch", Code: "acme-branch", TenantCode: "acme", Status: StatusEnabled}).Error; err != nil {
		t.Fatal(err)
	}
	for _, request := range []ReplaceOrgUnitRoleGroupsRequest{
		{OrgUnitCode: "acme-hq", RoleGroupCodes: []string{"acme-ops"}, ExpectedRevision: 0, ActorID: "admin", ChangedAt: now},
		{OrgUnitCode: "acme-branch", RoleGroupCodes: []string{"acme-audit"}, ExpectedRevision: 0, ActorID: "admin", ChangedAt: now},
	} {
		if _, err := repository.ReplaceOrgUnitRoleGroups(context.Background(), request); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&gormUser{ID: "user-alice", Code: "alice", ScopeType: string(ScopeTenant), TenantCode: "acme", OrgUnitCode: "acme-hq", Status: StatusEnabled}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&gormUserRole{UserID: "user-alice", RoleCode: "operator"}).Error; err != nil {
		t.Fatal(err)
	}
	executor, err := NewServiceObjectExecutor(repository, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	execution := kernel.ExecutionContext{Context: context.Background(), Actor: kernel.Actor{Username: "admin", Kind: kernel.ActorKindUser}}
	prepare, err := executor.ExecuteDomainCommand(context.Background(), serviceobject.DomainCommandPlan{
		Definition: domainDefinitionByID(t, UserOrganizationPrepareCommandID), Execution: execution,
		Arguments: serviceobject.ValidatedArguments{
			"userCode": "alice", "orgUnitCode": "acme-branch", "roleCodes": []string{"auditor"},
			"remediations": []serviceobject.RoleRemediation{{
				UserCode: "alice", RoleCode: "operator", Action: serviceobject.RoleRemediationReplace, ReplacementRoleCode: "auditor",
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	apply, err := executor.ExecuteDomainCommand(context.Background(), serviceobject.DomainCommandPlan{
		Definition: domainDefinitionByID(t, UserOrganizationChangeCommandID), Execution: execution,
		Arguments: serviceobject.ValidatedArguments{
			"previewId": prepare.Values["previewId"], "expectedRevision": prepare.Values["expectedRevision"], "impactHash": prepare.Values["impactHash"],
		},
	})
	if err != nil || apply.Values["revision"] != int64(3) {
		t.Fatalf("apply = %+v, error = %v", apply, err)
	}
	user, roles, err := loadUserWithRoles(db, "alice")
	if err != nil || user.TenantCode != "acme" || user.OrgUnitCode != "acme-branch" || len(roles) != 1 || roles[0] != "auditor" {
		t.Fatalf("user = %+v roles=%v error=%v", user, roles, err)
	}
	var audit gormOrganizationRBACAuditEvent
	if err := db.Where("preview_id = ?", prepare.Values["previewId"]).Take(&audit).Error; err != nil || audit.TenantCode != "acme" || audit.OrgUnitCode != "acme-branch" {
		t.Fatalf("audit = %+v error=%v", audit, err)
	}
}

func TestRoleMovePrepareApplyRequiresReviewedAssignmentRemediation(t *testing.T) {
	db, repository := prepareOrganizationRBACTestRepository(t)
	seedOrganizationRBAC(t, db)
	now := time.Date(2026, 7, 15, 16, 0, 0, 0, time.UTC)
	if err := db.Create(&gormRole{ID: "role-viewer", Code: "viewer", GroupCode: "acme-ops", Status: StatusEnabled}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repository.ReplaceOrgUnitRoleGroups(context.Background(), ReplaceOrgUnitRoleGroupsRequest{
		OrgUnitCode: "acme-hq", RoleGroupCodes: []string{"acme-ops"}, ExpectedRevision: 0, ActorID: "admin", ChangedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&gormUser{ID: "user-alice", Code: "alice", ScopeType: string(ScopeTenant), TenantCode: "acme", OrgUnitCode: "acme-hq", Status: StatusEnabled}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&gormUserRole{UserID: "user-alice", RoleCode: "operator"}).Error; err != nil {
		t.Fatal(err)
	}
	executor, err := NewServiceObjectExecutor(repository, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	execution := kernel.ExecutionContext{Context: context.Background(), Actor: kernel.Actor{Username: "admin", Kind: kernel.ActorKindUser}}
	prepare, err := executor.ExecuteDomainCommand(context.Background(), serviceobject.DomainCommandPlan{
		Definition: domainDefinitionByID(t, RoleStateOrGroupPrepareCommandID), Execution: execution,
		Arguments: serviceobject.ValidatedArguments{
			"roleCode": "operator", "operation": "move", "targetGroupCode": "acme-audit",
			"remediations": []serviceobject.RoleRemediation{{
				UserCode: "alice", RoleCode: "operator", Action: serviceobject.RoleRemediationReplace, ReplacementRoleCode: "viewer",
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	conflicts, err := executor.ExecuteQuery(context.Background(), serviceobject.QueryPlan{
		Definition: queryDefinitionByID(t, RoleStateOrGroupConflictsQueryID), Execution: execution,
		AST:  serviceobject.QueryAST{Predicates: []serviceobject.Predicate{{Field: "previewId", Operator: serviceobject.PredicateEqual, Value: prepare.Values["previewId"]}}},
		Page: 1, PageSize: 100,
	})
	if err != nil || len(conflicts.Items) != 1 || conflicts.Items[0]["userCode"] != "alice" || conflicts.Items[0]["roleCode"] != "operator" {
		t.Fatalf("role conflicts = %+v error=%v", conflicts, err)
	}
	apply, err := executor.ExecuteDomainCommand(context.Background(), serviceobject.DomainCommandPlan{
		Definition: domainDefinitionByID(t, RoleMoveCommandID), Execution: execution,
		Arguments: serviceobject.ValidatedArguments{
			"previewId": prepare.Values["previewId"], "expectedRevision": prepare.Values["expectedRevision"], "impactHash": prepare.Values["impactHash"],
		},
	})
	if err != nil || apply.Values["revision"] != int64(2) {
		t.Fatalf("apply = %+v error=%v", apply, err)
	}
	var role gormRole
	if err := db.Where("code = ?", "operator").Take(&role).Error; err != nil || role.GroupCode != "acme-audit" {
		t.Fatalf("role = %+v error=%v", role, err)
	}
	_, roles, err := loadUserWithRoles(db, "alice")
	if err != nil || len(roles) != 1 || roles[0] != "viewer" {
		t.Fatalf("roles = %v error=%v", roles, err)
	}
}

func TestRoleStateOrGroupConflictsQueryPaginatesAcrossThe1000RowBoundary(t *testing.T) {
	_, repository := prepareOrganizationRBACTestRepository(t)
	executor, err := NewServiceObjectExecutor(repository, func() time.Time {
		return time.Date(2026, 7, 15, 16, 0, 0, 0, time.UTC)
	})
	if err != nil {
		t.Fatal(err)
	}
	execution := kernel.ExecutionContext{
		Context: context.Background(), Actor: kernel.Actor{Username: "admin", Kind: kernel.ActorKindUser},
		TenantScope: kernel.TenantScope{PlatformWide: true}, PermissionIntent: kernel.PermissionIntent{Code: "admin:role:update", Action: "update"},
	}
	conflicts := make([]RoleAssignmentConflict, 2000)
	for index := range conflicts {
		conflicts[index] = RoleAssignmentConflict{UserCode: fmt.Sprintf("user-%04d", index+1), RoleCode: "operator"}
	}
	prepared, err := executor.storeDomainPreview(context.Background(), serviceobject.DomainCommandPlan{Execution: execution}, roleDisablePreviewOperation, 0,
		roleStateOrGroupChangeSet{RoleCode: "operator", Operation: roleOperationDisable},
		organizationRoleGroupImpactSummary{Severity: "high", AffectedUsers: len(conflicts), ConflictCount: len(conflicts), Conflicts: conflicts})
	if err != nil {
		t.Fatal(err)
	}
	registry, err := serviceobject.NewRegistry(OrganizationQueryDefinitions(), nil)
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := serviceobject.NewRuntime(registry, serviceobject.AuthorizerFunc(func(context.Context, kernel.ExecutionContext, string, string) bool { return true }), executor, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	request := serviceobject.QueryRequest{
		QueryID: RoleStateOrGroupConflictsQueryID, Version: ServiceObjectVersion,
		Arguments:  map[string]any{"previewId": prepared.Values["previewId"]},
		Pagination: serviceobject.Pagination{Page: 1, PageSize: 1000},
	}
	firstPage, err := runtime.ExecuteQuery(serviceobject.Invocation{Execution: execution}, request)
	if err != nil {
		t.Fatalf("first conflict page error = %v", err)
	}
	if len(firstPage.Items) != 1000 || firstPage.Items[999]["userCode"] != "user-1000" {
		t.Fatalf("first conflict page len=%d last=%v", len(firstPage.Items), firstPage.Items[999])
	}
	request.Pagination.Page = 2
	secondPage, err := runtime.ExecuteQuery(serviceobject.Invocation{Execution: execution}, request)
	if err != nil || len(secondPage.Items) != 1000 || secondPage.Items[0]["userCode"] != "user-1001" {
		t.Fatalf("second conflict page = %+v, error=%v", secondPage, err)
	}
	request.Pagination.Page = 3
	terminalPage, err := runtime.ExecuteQuery(serviceobject.Invocation{Execution: execution}, request)
	if err != nil || len(terminalPage.Items) != 0 {
		t.Fatalf("terminal conflict page = %+v, error=%v", terminalPage, err)
	}
}

func domainDefinitionByID(t *testing.T, id string) serviceobject.DomainCommandDefinition {
	t.Helper()
	for _, definition := range OrganizationDomainCommandDefinitions() {
		if definition.ID == id {
			return definition
		}
	}
	t.Fatalf("missing domain definition %s", id)
	return serviceobject.DomainCommandDefinition{}
}

func TestOrganizationDomainCommandDefinitionsHaveUniqueIDs(t *testing.T) {
	seen := make(map[string]struct{})
	for _, definition := range OrganizationDomainCommandDefinitions() {
		if _, exists := seen[definition.ID]; exists {
			t.Fatalf("duplicate organization domain command ID %q", definition.ID)
		}
		seen[definition.ID] = struct{}{}
	}
}

func queryDefinitionByID(t *testing.T, id string) serviceobject.QueryDefinition {
	t.Helper()
	for _, definition := range OrganizationQueryDefinitions() {
		if definition.ID == id {
			return definition
		}
	}
	t.Fatalf("missing query definition %s", id)
	return serviceobject.QueryDefinition{}
}
