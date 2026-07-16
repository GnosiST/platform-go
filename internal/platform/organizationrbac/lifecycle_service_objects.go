package organizationrbac

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/GnosiST/platform-go/internal/platform/serviceobject"

	"gorm.io/gorm"
)

const (
	ResourceLifecycleImpactQueryID      = "platform.authorization.resource-lifecycle.impact"
	ResourceLifecyclePrepareCommandID   = "platform.authorization.resource-lifecycle.prepare"
	ResourceLifecycleApplyCommandID     = "platform.authorization.resource-lifecycle.apply"
	resourceLifecyclePreviewOperation   = "resource-lifecycle"
	resourceLifecyclePermission         = "admin:authorization-lifecycle:update"
	resourceLifecycleDefinitionResource = "authorization-lifecycle"
)

type resourceLifecycleChangeSet struct {
	Resource      string                      `json:"resource"`
	ResourceCode  string                      `json:"resourceCode"`
	Operation     string                      `json:"operation"`
	TenantCode    string                      `json:"tenantCode"`
	OrgUnitCode   string                      `json:"orgUnitCode,omitempty"`
	RetentionDays int                         `json:"retentionDays,omitempty"`
	PolicyVersion uint32                      `json:"policyVersion,omitempty"`
	Remediations  []RoleAssignmentRemediation `json:"remediations,omitempty"`
}

func resourceLifecycleImpactQueryDefinition() serviceobject.QueryDefinition {
	return serviceobject.QueryDefinition{
		ID: ResourceLifecycleImpactQueryID, Version: ServiceObjectVersion, Resource: resourceLifecycleDefinitionResource,
		Permission: resourceLifecyclePermission, Action: "update", TenantMode: serviceobject.TenantPlatform, DataScope: "platform",
		Arguments: []serviceobject.ArgumentDefinition{{Name: "previewId", Type: serviceobject.ValueString, Required: true, MaxLength: 64}},
		Cost:      serviceobject.CostPolicy{BaseCost: 1, PerRowCost: 1, PredicateCost: 1, Limit: 8}, Timeout: 2 * time.Second, MaxPageSize: 1,
		ResultSchema: []serviceobject.ResultField{
			{Name: "previewId", Type: serviceobject.ValueString}, {Name: "severity", Type: serviceobject.ValueString},
			{Name: "affectedUsers", Type: serviceobject.ValueInteger}, {Name: "conflictCount", Type: serviceobject.ValueInteger},
			{Name: "expectedRevision", Type: serviceobject.ValueInteger}, {Name: "impactHash", Type: serviceobject.ValueString},
			{Name: "expiresAt", Type: serviceobject.ValueString}, {Name: "resource", Type: serviceobject.ValueString},
			{Name: "resourceCode", Type: serviceobject.ValueString}, {Name: "operation", Type: serviceobject.ValueString},
			{Name: "referenceCount", Type: serviceobject.ValueInteger}, {Name: "retentionElapsed", Type: serviceobject.ValueBoolean},
		},
		Build: func(arguments serviceobject.ValidatedArguments) (serviceobject.QueryAST, error) {
			return serviceobject.QueryAST{Resource: resourceLifecycleDefinitionResource, Predicates: []serviceobject.Predicate{{
				Field: "previewId", Operator: serviceobject.PredicateEqual, Value: arguments["previewId"],
			}}}, nil
		},
	}
}

func resourceLifecyclePrepareDefinition(cost serviceobject.CostPolicy) serviceobject.DomainCommandDefinition {
	minimumRetention, maximumRetention := int64(1), int64(36500)
	minimumPolicy, maximumPolicy := int64(1), int64(^uint32(0))
	return serviceobject.DomainCommandDefinition{
		ID: ResourceLifecyclePrepareCommandID, Version: ServiceObjectVersion, Resource: resourceLifecycleDefinitionResource,
		Permission: resourceLifecyclePermission, Action: "update", TenantMode: serviceobject.TenantPlatform, DataScope: "platform",
		Arguments: []serviceobject.ArgumentDefinition{
			{Name: "resource", Type: serviceobject.ValueString, Required: true, MaxLength: 32},
			{Name: "resourceCode", Type: serviceobject.ValueString, Required: true, MaxLength: 191},
			{Name: "operation", Type: serviceobject.ValueString, Required: true, MaxLength: 16},
			{Name: "retentionDays", Type: serviceobject.ValueInteger, Minimum: &minimumRetention, Maximum: &maximumRetention},
			{Name: "policyVersion", Type: serviceobject.ValueInteger, Minimum: &minimumPolicy, Maximum: &maximumPolicy},
			{Name: "remediations", Type: serviceobject.ValueRoleRemediations, MaxLength: 191},
		},
		Cost: cost, Timeout: 5 * time.Second, Idempotency: serviceobject.IdempotencyRequiredKey, MaxAffectedRows: 2000,
		ResultSchema: previewResultSchema(),
	}
}

func resourceLifecycleApplyDefinition(cost serviceobject.CostPolicy) serviceobject.DomainCommandDefinition {
	return applyDomainDefinition(ResourceLifecycleApplyCommandID, resourceLifecycleDefinitionResource, resourceLifecyclePermission, cost)
}

func (e *ServiceObjectExecutor) prepareResourceLifecycle(ctx context.Context, plan serviceobject.DomainCommandPlan) (serviceobject.CommandResult, error) {
	actor := strings.TrimSpace(plan.Execution.Actor.Username)
	resource, _ := plan.Arguments["resource"].(string)
	resourceCode, _ := plan.Arguments["resourceCode"].(string)
	operation, _ := plan.Arguments["operation"].(string)
	retentionDays, hasRetention := plan.Arguments["retentionDays"].(int64)
	policyVersion, hasPolicy := plan.Arguments["policyVersion"].(int64)
	remediations := domainRemediations(plan.Arguments["remediations"])
	if actor == "" || !validLifecycleResource(resource) || !validCode(resourceCode) || !validLifecycleOperation(operation) {
		return serviceobject.CommandResult{}, serviceobject.ErrValidation
	}
	if operation == LifecycleOperationDelete {
		if !hasRetention || !hasPolicy || retentionDays < 1 || retentionDays > 36500 || policyVersion < 1 || policyVersion > int64(^uint32(0)) {
			return serviceobject.CommandResult{}, serviceobject.ErrValidation
		}
	} else if hasRetention || hasPolicy {
		return serviceobject.CommandResult{}, serviceobject.ErrValidation
	}
	impact, err := e.repository.PreviewResourceLifecycle(ctx, resource, resourceCode, operation, e.now().UTC())
	if err != nil {
		return serviceobject.CommandResult{}, mapServiceObjectError(err)
	}
	if operation == LifecycleOperationDelete {
		if err := validateLifecycleRemediationPlan(e.repository.db.WithContext(ctx), impact, remediations); err != nil {
			return serviceobject.CommandResult{}, mapServiceObjectError(err)
		}
	} else if operation == LifecycleOperationDisable {
		if err := validateLifecycleRemediationPlan(e.repository.db.WithContext(ctx), impact, remediations); err != nil {
			return serviceobject.CommandResult{}, mapServiceObjectError(err)
		}
	} else if len(remediations) != 0 {
		return serviceobject.CommandResult{}, serviceobject.ErrValidation
	}
	severity := "low"
	if impact.ReferenceCount > 0 || operation == LifecycleOperationPurge && !impact.RetentionElapsed {
		severity = "high"
	} else if operation != LifecycleOperationRestore {
		severity = "medium"
	}
	changeSet := resourceLifecycleChangeSet{
		Resource: resource, ResourceCode: resourceCode, Operation: operation,
		TenantCode: impact.TenantCode, OrgUnitCode: impact.OrgUnitCode, Remediations: remediations,
	}
	if operation == LifecycleOperationDelete {
		changeSet.RetentionDays = int(retentionDays)
		changeSet.PolicyVersion = uint32(policyVersion)
	}
	summary := organizationRoleGroupImpactSummary{
		Severity: severity, AffectedUsers: impact.AffectedUsers, ConflictCount: impact.ReferenceCount,
		Resource: resource, ResourceCode: resourceCode, Operation: operation,
		ReferenceCount: impact.ReferenceCount, RetentionElapsed: impact.RetentionElapsed,
	}
	return e.storeDomainPreview(ctx, plan, resourceLifecyclePreviewOperation, impact.ExpectedRevision, changeSet, summary)
}

func (e *ServiceObjectExecutor) applyResourceLifecycle(ctx context.Context, plan serviceobject.DomainCommandPlan) (serviceobject.CommandResult, error) {
	return e.applyIdentityPreview(ctx, plan, resourceLifecyclePreviewOperation, func(tx *gorm.DB, preview gormOrganizationRBACPreview, actor string) (uint64, string, string, error) {
		var changeSet resourceLifecycleChangeSet
		if err := json.Unmarshal([]byte(preview.ChangeSetJSON), &changeSet); err != nil {
			return 0, "", "", serviceobject.ErrExecutionFailed
		}
		revision, err := (&GORMRepository{db: tx}).ApplyResourceLifecycle(ctx, ResourceLifecycleRequest{
			Resource: changeSet.Resource, ResourceCode: changeSet.ResourceCode, Operation: changeSet.Operation,
			RetentionDays: changeSet.RetentionDays, PolicyVersion: changeSet.PolicyVersion,
			ExpectedRevision: preview.ExpectedRevision, ActorID: actor, ChangedAt: e.now().UTC(),
		}, changeSet.Remediations)
		return revision, changeSet.TenantCode, changeSet.OrgUnitCode, mapServiceObjectError(err)
	})
}
