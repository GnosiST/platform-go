package organizationrbac

import (
	"context"
	"errors"
	"testing"
	"time"

	"platform-go/internal/platform/kernel"
	"platform-go/internal/platform/serviceobject"
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
	apply, err := executor.ExecuteDomainCommand(context.Background(), serviceobject.DomainCommandPlan{
		Definition: domainDefinitionByID(t, ResourceLifecycleApplyCommandID), Execution: execution,
		Arguments: serviceobject.ValidatedArguments{
			"previewId": previewID, "expectedRevision": prepare.Values["expectedRevision"], "impactHash": prepare.Values["impactHash"],
		},
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
