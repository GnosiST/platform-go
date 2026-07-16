package organizationrbac

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"platform-go/internal/platform/serviceobject"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	ServiceObjectVersion                        = "1.0.0"
	OrganizationRolePoolQueryID                 = "platform.identity.organization-role-pool.get"
	OrganizationRoleGroupImpactQueryID          = "platform.identity.organization-role-group-change.impact"
	OrganizationRoleGroupConflictsQueryID       = "platform.identity.organization-role-group-change.conflicts.list"
	UserOrganizationImpactQueryID               = "platform.identity.user-organization-change.impact"
	RoleStateOrGroupImpactQueryID               = "platform.identity.role-state-or-group-change.impact"
	RoleStateOrGroupConflictsQueryID            = "platform.identity.role-state-or-group-change.conflicts.list"
	RolePermissionImpactQueryID                 = "platform.authorization.role-permission-change.impact"
	OrganizationRoleGroupPrepareCommandID       = "platform.identity.organization-role-group-change.prepare"
	UserOrganizationPrepareCommandID            = "platform.identity.user-organization-change.prepare"
	RoleStateOrGroupPrepareCommandID            = "platform.identity.role-state-or-group-change.prepare"
	RolePermissionPrepareCommandID              = "platform.authorization.role-permission-change.prepare"
	OrganizationRoleGroupsReplaceCommandID      = "platform.identity.organization-role-groups.replace"
	UserOrganizationChangeCommandID             = "platform.identity.user-organization.change"
	RoleMoveCommandID                           = "platform.identity.role.move"
	RoleDisableCommandID                        = "platform.identity.role.disable"
	RolePermissionsReplaceCommandID             = "platform.authorization.role-permissions.replace"
	organizationRoleGroupPreviewOperation       = "organization-role-groups.replace"
	userOrganizationPreviewOperation            = "user-organization.change"
	roleMovePreviewOperation                    = "role.move"
	roleDisablePreviewOperation                 = "role.disable"
	rolePermissionsPreviewOperation             = "role-permissions.replace"
	organizationRoleGroupPreviewStatusPrepared  = "prepared"
	organizationRoleGroupPreviewStatusApplied   = "applied"
	defaultOrganizationRoleGroupPreviewDuration = 15 * time.Minute
)

type ServiceObjectExecutor struct {
	repository           *GORMRepository
	now                  func() time.Time
	ttl                  time.Duration
	roleMenuWriteEnabled bool
}

type organizationRoleGroupChangeSet struct {
	OrgUnitCode    string                      `json:"orgUnitCode"`
	TenantCode     string                      `json:"tenantCode"`
	RoleGroupCodes []string                    `json:"roleGroupCodes"`
	Remediations   []RoleAssignmentRemediation `json:"remediations,omitempty"`
}

type organizationRoleGroupImpactSummary struct {
	Severity          string                   `json:"severity"`
	AffectedUsers     int                      `json:"affectedUsers"`
	ConflictCount     int                      `json:"conflictCount"`
	CurrentGroupCount int                      `json:"currentGroupCount"`
	TargetGroupCount  int                      `json:"targetGroupCount"`
	Resource          string                   `json:"resource,omitempty"`
	ResourceCode      string                   `json:"resourceCode,omitempty"`
	Operation         string                   `json:"operation,omitempty"`
	ReferenceCount    int                      `json:"referenceCount,omitempty"`
	RetentionElapsed  bool                     `json:"retentionElapsed,omitempty"`
	Conflicts         []RoleAssignmentConflict `json:"conflicts,omitempty"`
}

type organizationRoleGroupAppliedResult struct {
	Applied   bool   `json:"applied"`
	Revision  int64  `json:"revision"`
	PreviewID string `json:"previewId"`
}

type gormOrganizationRBACPreview struct {
	ID                string    `gorm:"column:id;size:64;primaryKey"`
	Operation         string    `gorm:"column:operation;size:128;index;not null"`
	OwnerActor        string    `gorm:"column:owner_actor;size:191;index;not null"`
	TenantCode        string    `gorm:"column:tenant_code;size:191;index;not null"`
	ScopeHash         string    `gorm:"column:scope_hash;size:64;not null"`
	ExpectedRevision  uint64    `gorm:"column:expected_revision;not null"`
	ImpactHash        string    `gorm:"column:impact_hash;size:64;not null"`
	ChangeSetHash     string    `gorm:"column:change_set_hash;size:64;not null"`
	ChangeSetJSON     string    `gorm:"column:change_set_json;type:text;not null"`
	SummaryJSON       string    `gorm:"column:summary_json;type:text;not null"`
	Status            string    `gorm:"column:status;size:32;index;not null"`
	ExpiresAt         time.Time `gorm:"column:expires_at;index;not null"`
	AppliedResultJSON string    `gorm:"column:applied_result_json;type:text;not null"`
	CreatedAt         time.Time `gorm:"column:created_at;not null"`
	UpdatedAt         time.Time `gorm:"column:updated_at;not null"`
}

type gormOrganizationRBACAuditEvent struct {
	ID             string    `gorm:"column:id;size:64;primaryKey"`
	ActorType      string    `gorm:"column:actor_type;size:32;index;not null"`
	ActorCode      string    `gorm:"column:actor_code;size:191;index;not null"`
	TenantCode     string    `gorm:"column:tenant_code;size:191;index;not null"`
	OrgUnitCode    string    `gorm:"column:org_unit_code;size:191;index;not null"`
	Action         string    `gorm:"column:action;size:191;index;not null"`
	BeforeRevision uint64    `gorm:"column:before_revision;not null"`
	AfterRevision  uint64    `gorm:"column:after_revision;not null"`
	ImpactHash     string    `gorm:"column:impact_hash;size:64;not null"`
	PreviewID      string    `gorm:"column:preview_id;size:64;index;not null"`
	ConflictCount  int       `gorm:"column:conflict_count;not null"`
	CreatedAt      time.Time `gorm:"column:created_at;index;not null"`
}

func (gormOrganizationRBACPreview) TableName() string { return "platform_organization_rbac_previews" }
func (gormOrganizationRBACAuditEvent) TableName() string {
	return "platform_organization_rbac_audit_events"
}

func NewServiceObjectExecutor(repository *GORMRepository, now func() time.Time) (*ServiceObjectExecutor, error) {
	return NewServiceObjectExecutorWithOptions(repository, now, ServiceObjectExecutorOptions{})
}

func NewServiceObjectExecutorWithOptions(repository *GORMRepository, now func() time.Time, options ServiceObjectExecutorOptions) (*ServiceObjectExecutor, error) {
	if repository == nil || !repository.Persistent() {
		return nil, ErrRepositoryFailed
	}
	if now == nil {
		now = time.Now
	}
	return &ServiceObjectExecutor{
		repository: repository, now: now, ttl: defaultOrganizationRoleGroupPreviewDuration,
		roleMenuWriteEnabled: options.RoleMenuWriteEnabled,
	}, nil
}

func OrganizationQueryDefinitions() []serviceobject.QueryDefinition {
	definitions := []serviceobject.QueryDefinition{
		{
			ID: OrganizationRolePoolQueryID, Version: ServiceObjectVersion, Resource: "org-units",
			Permission: "admin:org-unit:read", Action: "read", TenantMode: serviceobject.TenantPlatform, DataScope: "platform",
			Arguments: []serviceobject.ArgumentDefinition{{Name: "orgUnitCode", Type: serviceobject.ValueString, Required: true, MaxLength: 191}},
			Cost:      serviceobject.CostPolicy{BaseCost: 2, PerRowCost: 1, PredicateCost: 1, Limit: 2050}, Timeout: 3 * time.Second,
			MaxPageSize: 1000,
			ResultSchema: []serviceobject.ResultField{
				{Name: "roleCode", Type: serviceobject.ValueString}, {Name: "roleName", Type: serviceobject.ValueString}, {Name: "roleGroupCode", Type: serviceobject.ValueString},
				{Name: "roleGroupName", Type: serviceobject.ValueString}, {Name: "tenantCode", Type: serviceobject.ValueString},
				{Name: "status", Type: serviceobject.ValueString},
			},
			Build: func(arguments serviceobject.ValidatedArguments) (serviceobject.QueryAST, error) {
				return serviceobject.QueryAST{Resource: "org-units", Predicates: []serviceobject.Predicate{{Field: "orgUnitCode", Operator: serviceobject.PredicateEqual, Value: arguments["orgUnitCode"]}}}, nil
			},
		},
		{
			ID: OrganizationRoleGroupImpactQueryID, Version: ServiceObjectVersion, Resource: "org-units",
			Permission: "admin:org-unit:update", Action: "update", TenantMode: serviceobject.TenantPlatform, DataScope: "platform",
			Arguments: []serviceobject.ArgumentDefinition{{Name: "previewId", Type: serviceobject.ValueString, Required: true, MaxLength: 64}},
			Cost:      serviceobject.CostPolicy{BaseCost: 1, PerRowCost: 1, PredicateCost: 1, Limit: 8}, Timeout: 2 * time.Second,
			MaxPageSize: 1,
			ResultSchema: []serviceobject.ResultField{
				{Name: "previewId", Type: serviceobject.ValueString}, {Name: "severity", Type: serviceobject.ValueString},
				{Name: "affectedUsers", Type: serviceobject.ValueInteger}, {Name: "conflictCount", Type: serviceobject.ValueInteger},
				{Name: "expectedRevision", Type: serviceobject.ValueInteger}, {Name: "impactHash", Type: serviceobject.ValueString},
				{Name: "expiresAt", Type: serviceobject.ValueString},
			},
			Build: func(arguments serviceobject.ValidatedArguments) (serviceobject.QueryAST, error) {
				return serviceobject.QueryAST{Resource: "org-units", Predicates: []serviceobject.Predicate{{Field: "previewId", Operator: serviceobject.PredicateEqual, Value: arguments["previewId"]}}}, nil
			},
		},
		{
			ID: OrganizationRoleGroupConflictsQueryID, Version: ServiceObjectVersion, Resource: "org-units",
			Permission: "admin:org-unit:update", Action: "update", TenantMode: serviceobject.TenantPlatform, DataScope: "platform",
			Arguments: []serviceobject.ArgumentDefinition{{Name: "previewId", Type: serviceobject.ValueString, Required: true, MaxLength: 64}},
			Cost:      serviceobject.CostPolicy{BaseCost: 2, PerRowCost: 1, PredicateCost: 1, Limit: 2005}, Timeout: 3 * time.Second,
			MaxPageSize: 1000,
			ResultSchema: []serviceobject.ResultField{
				{Name: "userCode", Type: serviceobject.ValueString}, {Name: "roleCode", Type: serviceobject.ValueString},
			},
			Build: func(arguments serviceobject.ValidatedArguments) (serviceobject.QueryAST, error) {
				return serviceobject.QueryAST{Resource: "org-units", Predicates: []serviceobject.Predicate{{Field: "previewId", Operator: serviceobject.PredicateEqual, Value: arguments["previewId"]}}}, nil
			},
		},
		impactQueryDefinition(UserOrganizationImpactQueryID, "users", "admin:user:update"),
		impactQueryDefinition(RoleStateOrGroupImpactQueryID, "roles", "admin:role:update"),
		conflictQueryDefinition(RoleStateOrGroupConflictsQueryID, "roles", "admin:role:update"),
		impactQueryDefinition(RolePermissionImpactQueryID, "roles", "admin:role:update"),
		resourceLifecycleImpactQueryDefinition(),
	}
	definitions = append(definitions, navigationQueryDefinitions()...)
	return append(definitions, assignmentTreeQueryDefinitions()...)
}

func OrganizationDomainCommandDefinitions() []serviceobject.DomainCommandDefinition {
	baseCost := serviceobject.CostPolicy{BaseCost: 5, PerRowCost: 1, Limit: 2005}
	definitions := []serviceobject.DomainCommandDefinition{
		{
			ID: OrganizationRoleGroupPrepareCommandID, Version: ServiceObjectVersion, Resource: "org-units",
			Permission: "admin:org-unit:update", Action: "update", TenantMode: serviceobject.TenantPlatform, DataScope: "platform",
			Arguments: []serviceobject.ArgumentDefinition{
				{Name: "orgUnitCode", Type: serviceobject.ValueString, Required: true, MaxLength: 191},
				{Name: "roleGroupCodes", Type: serviceobject.ValueStringSet, Required: true, MaxLength: 191},
				{Name: "remediations", Type: serviceobject.ValueRoleRemediations, MaxLength: 191},
			},
			Cost: baseCost, Timeout: 5 * time.Second, Idempotency: serviceobject.IdempotencyRequiredKey, MaxAffectedRows: 2000,
			ResultSchema: previewResultSchema(),
		},
		{
			ID: OrganizationRoleGroupsReplaceCommandID, Version: ServiceObjectVersion, Resource: "org-units",
			Permission: "admin:org-unit:update", Action: "update", TenantMode: serviceobject.TenantPlatform, DataScope: "platform",
			Arguments: []serviceobject.ArgumentDefinition{
				{Name: "previewId", Type: serviceobject.ValueString, Required: true, MaxLength: 64},
				{Name: "expectedRevision", Type: serviceobject.ValueInteger, Required: true},
				{Name: "impactHash", Type: serviceobject.ValueString, Required: true, MaxLength: 64},
			},
			Cost: baseCost, Timeout: 10 * time.Second, Idempotency: serviceobject.IdempotencyRequiredKey, MaxAffectedRows: 2000,
			ResultSchema: []serviceobject.ResultField{
				{Name: "applied", Type: serviceobject.ValueBoolean}, {Name: "revision", Type: serviceobject.ValueInteger},
				{Name: "previewId", Type: serviceobject.ValueString},
			},
		},
		{
			ID: UserOrganizationPrepareCommandID, Version: ServiceObjectVersion, Resource: "users",
			Permission: "admin:user:update", Action: "update", TenantMode: serviceobject.TenantPlatform, DataScope: "platform",
			Arguments: []serviceobject.ArgumentDefinition{
				{Name: "userCode", Type: serviceobject.ValueString, Required: true, MaxLength: 191},
				{Name: "orgUnitCode", Type: serviceobject.ValueString, Required: true, MaxLength: 191},
				{Name: "roleCodes", Type: serviceobject.ValueStringSet, Required: true, MaxLength: 191},
				{Name: "remediations", Type: serviceobject.ValueRoleRemediations, MaxLength: 191},
			},
			Cost: baseCost, Timeout: 5 * time.Second, Idempotency: serviceobject.IdempotencyRequiredKey, MaxAffectedRows: 2000,
			ResultSchema: previewResultSchema(),
		},
		{
			ID: RoleStateOrGroupPrepareCommandID, Version: ServiceObjectVersion, Resource: "roles",
			Permission: "admin:role:update", Action: "update", TenantMode: serviceobject.TenantPlatform, DataScope: "platform",
			Arguments: []serviceobject.ArgumentDefinition{
				{Name: "roleCode", Type: serviceobject.ValueString, Required: true, MaxLength: 191},
				{Name: "operation", Type: serviceobject.ValueString, Required: true, MaxLength: 32},
				{Name: "targetGroupCode", Type: serviceobject.ValueString, MaxLength: 191},
				{Name: "remediations", Type: serviceobject.ValueRoleRemediations, MaxLength: 191},
			},
			Cost: baseCost, Timeout: 5 * time.Second, Idempotency: serviceobject.IdempotencyRequiredKey, MaxAffectedRows: 2000,
			ResultSchema: previewResultSchema(),
		},
		{
			ID: RolePermissionPrepareCommandID, Version: ServiceObjectVersion, Resource: "roles",
			Permission: "admin:role:update", Action: "update", TenantMode: serviceobject.TenantPlatform, DataScope: "platform",
			Arguments: []serviceobject.ArgumentDefinition{
				{Name: "roleCode", Type: serviceobject.ValueString, Required: true, MaxLength: 191},
				{Name: "allowPermissionCodes", Type: serviceobject.ValueStringSet, Required: true, MaxLength: 191},
				{Name: "denyPermissionCodes", Type: serviceobject.ValueStringSet, Required: true, MaxLength: 191},
				{Name: "dataScope", Type: serviceobject.ValueString, Required: true, MaxLength: 64},
				{Name: "dataScopeOrgCodes", Type: serviceobject.ValueStringSet, MaxLength: 191},
				{Name: "dataScopeAreaCodes", Type: serviceobject.ValueStringSet, MaxLength: 191},
			},
			Cost: baseCost, Timeout: 5 * time.Second, Idempotency: serviceobject.IdempotencyRequiredKey, MaxAffectedRows: 2000,
			ResultSchema: previewResultSchema(),
		},
		applyDomainDefinition(UserOrganizationChangeCommandID, "users", "admin:user:update", baseCost),
		applyDomainDefinition(RoleMoveCommandID, "roles", "admin:role:update", baseCost),
		applyDomainDefinition(RoleDisableCommandID, "roles", "admin:role:update", baseCost),
		applyDomainDefinition(RolePermissionsReplaceCommandID, "roles", "admin:role:update", baseCost),
		resourceLifecyclePrepareDefinition(baseCost),
		resourceLifecycleApplyDefinition(baseCost),
	}
	return append(definitions, navigationDomainCommandDefinitions()...)
}

func impactQueryDefinition(id, resource, permission string) serviceobject.QueryDefinition {
	return serviceobject.QueryDefinition{
		ID: id, Version: ServiceObjectVersion, Resource: resource, Permission: permission, Action: "update",
		TenantMode: serviceobject.TenantPlatform, DataScope: "platform",
		Arguments: []serviceobject.ArgumentDefinition{{Name: "previewId", Type: serviceobject.ValueString, Required: true, MaxLength: 64}},
		Cost:      serviceobject.CostPolicy{BaseCost: 1, PerRowCost: 1, PredicateCost: 1, Limit: 8}, Timeout: 2 * time.Second,
		MaxPageSize: 1,
		ResultSchema: []serviceobject.ResultField{
			{Name: "previewId", Type: serviceobject.ValueString}, {Name: "severity", Type: serviceobject.ValueString},
			{Name: "affectedUsers", Type: serviceobject.ValueInteger}, {Name: "conflictCount", Type: serviceobject.ValueInteger},
			{Name: "expectedRevision", Type: serviceobject.ValueInteger}, {Name: "impactHash", Type: serviceobject.ValueString},
			{Name: "expiresAt", Type: serviceobject.ValueString},
		},
		Build: func(arguments serviceobject.ValidatedArguments) (serviceobject.QueryAST, error) {
			return serviceobject.QueryAST{Resource: resource, Predicates: []serviceobject.Predicate{{Field: "previewId", Operator: serviceobject.PredicateEqual, Value: arguments["previewId"]}}}, nil
		},
	}
}

func conflictQueryDefinition(id, resource, permission string) serviceobject.QueryDefinition {
	return serviceobject.QueryDefinition{
		ID: id, Version: ServiceObjectVersion, Resource: resource, Permission: permission, Action: "update",
		TenantMode: serviceobject.TenantPlatform, DataScope: "platform",
		Arguments: []serviceobject.ArgumentDefinition{{Name: "previewId", Type: serviceobject.ValueString, Required: true, MaxLength: 64}},
		Cost: serviceobject.CostPolicy{
			BaseCost: 2, PerRowCost: 1, PerOffsetCost: 1, PredicateCost: 1, MaxOffset: 2000, Limit: 3005,
		}, Timeout: 3 * time.Second,
		MaxPageSize: 1000,
		ResultSchema: []serviceobject.ResultField{
			{Name: "userCode", Type: serviceobject.ValueString}, {Name: "roleCode", Type: serviceobject.ValueString},
		},
		Build: func(arguments serviceobject.ValidatedArguments) (serviceobject.QueryAST, error) {
			return serviceobject.QueryAST{Resource: resource, Predicates: []serviceobject.Predicate{{Field: "previewId", Operator: serviceobject.PredicateEqual, Value: arguments["previewId"]}}}, nil
		},
	}
}

func applyDomainDefinition(id, resource, permission string, cost serviceobject.CostPolicy) serviceobject.DomainCommandDefinition {
	return serviceobject.DomainCommandDefinition{
		ID: id, Version: ServiceObjectVersion, Resource: resource, Permission: permission, Action: "update",
		TenantMode: serviceobject.TenantPlatform, DataScope: "platform",
		Arguments: []serviceobject.ArgumentDefinition{
			{Name: "previewId", Type: serviceobject.ValueString, Required: true, MaxLength: 64},
			{Name: "expectedRevision", Type: serviceobject.ValueInteger, Required: true},
			{Name: "impactHash", Type: serviceobject.ValueString, Required: true, MaxLength: 64},
		},
		Cost: cost, Timeout: 10 * time.Second, Idempotency: serviceobject.IdempotencyRequiredKey, MaxAffectedRows: 2000,
		ResultSchema: []serviceobject.ResultField{
			{Name: "applied", Type: serviceobject.ValueBoolean}, {Name: "revision", Type: serviceobject.ValueInteger},
			{Name: "previewId", Type: serviceobject.ValueString},
		},
	}
}

func previewResultSchema() []serviceobject.ResultField {
	return []serviceobject.ResultField{
		{Name: "previewId", Type: serviceobject.ValueString}, {Name: "expectedRevision", Type: serviceobject.ValueInteger},
		{Name: "impactHash", Type: serviceobject.ValueString}, {Name: "expiresAt", Type: serviceobject.ValueString},
	}
}

func (e *ServiceObjectExecutor) ExecuteQuery(ctx context.Context, plan serviceobject.QueryPlan) (serviceobject.QueryResult, error) {
	switch plan.Definition.ID {
	case MenuDefinitionGetQueryID:
		return e.getMenuDefinition(ctx, plan)
	case RoleMenusGetQueryID:
		return e.getRoleMenus(ctx, plan)
	case RoleMenuImpactQueryID:
		return e.getRoleMenuImpact(ctx, plan)
	case RoleMenuMigrationCompareQueryID:
		return e.compareRoleMenuMigration(ctx, plan)
	case MenuAssignmentTreeSearchQueryID, MenuAssignmentTreeHydrateQueryID, PermissionAssignmentTreeSearchQueryID, PermissionAssignmentTreeHydrateQueryID:
		return e.executeAssignmentTreeQuery(ctx, plan)
	case OrganizationRolePoolQueryID:
		orgUnitCode, ok := predicateString(plan.AST, "orgUnitCode")
		if !ok {
			return serviceobject.QueryResult{}, serviceobject.ErrValidation
		}
		pool, err := e.repository.EffectiveRolePool(ctx, orgUnitCode)
		if err != nil {
			return serviceobject.QueryResult{}, mapServiceObjectError(err)
		}
		items := make([]map[string]any, 0, len(pool))
		for _, entry := range pool {
			items = append(items, map[string]any{
				"roleCode": entry.RoleCode, "roleName": entry.RoleName, "roleGroupCode": entry.RoleGroupCode, "roleGroupName": entry.RoleGroupName,
				"tenantCode": entry.TenantCode, "status": entry.Status,
			})
		}
		return serviceobject.QueryResult{Items: items}, nil
	case OrganizationRoleGroupImpactQueryID, UserOrganizationImpactQueryID, RoleStateOrGroupImpactQueryID, RolePermissionImpactQueryID, ResourceLifecycleImpactQueryID:
		previewID, ok := predicateString(plan.AST, "previewId")
		if !ok {
			return serviceobject.QueryResult{}, serviceobject.ErrValidation
		}
		preview, summary, err := e.loadPreviewForPlan(ctx, plan.Execution.Actor.Username, plan.TenantID, plan.Scope, previewID)
		if err != nil {
			return serviceobject.QueryResult{}, err
		}
		if !previewMatchesImpactQuery(plan.Definition.ID, preview.Operation) {
			return serviceobject.QueryResult{}, serviceobject.ErrObjectUnavailable
		}
		item := map[string]any{
			"previewId": preview.ID, "severity": summary.Severity, "affectedUsers": int64(summary.AffectedUsers),
			"conflictCount": int64(summary.ConflictCount), "expectedRevision": int64(preview.ExpectedRevision),
			"impactHash": preview.ImpactHash, "expiresAt": preview.ExpiresAt.UTC().Format(time.RFC3339),
		}
		if plan.Definition.ID == ResourceLifecycleImpactQueryID {
			item["resource"] = summary.Resource
			item["resourceCode"] = summary.ResourceCode
			item["operation"] = summary.Operation
			item["referenceCount"] = int64(summary.ReferenceCount)
			item["retentionElapsed"] = summary.RetentionElapsed
		}
		return serviceobject.QueryResult{Items: []map[string]any{item}}, nil
	case OrganizationRoleGroupConflictsQueryID, RoleStateOrGroupConflictsQueryID:
		previewID, ok := predicateString(plan.AST, "previewId")
		if !ok {
			return serviceobject.QueryResult{}, serviceobject.ErrValidation
		}
		preview, summary, err := e.loadPreviewForPlan(ctx, plan.Execution.Actor.Username, plan.TenantID, plan.Scope, previewID)
		if err != nil {
			return serviceobject.QueryResult{}, err
		}
		if plan.Definition.ID == OrganizationRoleGroupConflictsQueryID && preview.Operation != organizationRoleGroupPreviewOperation ||
			plan.Definition.ID == RoleStateOrGroupConflictsQueryID && preview.Operation != roleMovePreviewOperation && preview.Operation != roleDisablePreviewOperation {
			return serviceobject.QueryResult{}, serviceobject.ErrObjectUnavailable
		}
		start := (plan.Page - 1) * plan.PageSize
		if start < 0 {
			start = 0
		}
		if start > len(summary.Conflicts) {
			start = len(summary.Conflicts)
		}
		end := start + plan.PageSize
		if end > len(summary.Conflicts) {
			end = len(summary.Conflicts)
		}
		items := make([]map[string]any, 0, end-start)
		for _, conflict := range summary.Conflicts[start:end] {
			items = append(items, map[string]any{"userCode": conflict.UserCode, "roleCode": conflict.RoleCode})
		}
		return serviceobject.QueryResult{Items: items}, nil
	default:
		return serviceobject.QueryResult{}, serviceobject.ErrObjectUnavailable
	}
}

func (e *ServiceObjectExecutor) ExecuteDomainCommand(ctx context.Context, plan serviceobject.DomainCommandPlan) (serviceobject.CommandResult, error) {
	switch plan.Definition.ID {
	case MenuDefinitionCreateCommandID:
		return e.createMenuDefinition(ctx, plan)
	case MenuDefinitionReplaceCommandID:
		return e.replaceMenuDefinition(ctx, plan)
	case RoleMenuPrepareCommandID:
		return e.prepareRoleMenus(ctx, plan)
	case RoleMenusReplaceCommandID:
		return e.applyRoleMenus(ctx, plan)
	case OrganizationRoleGroupPrepareCommandID:
		return e.prepareOrganizationRoleGroups(ctx, plan)
	case OrganizationRoleGroupsReplaceCommandID:
		return e.applyOrganizationRoleGroups(ctx, plan)
	case UserOrganizationPrepareCommandID:
		return e.prepareUserOrganizationChange(ctx, plan)
	case RoleStateOrGroupPrepareCommandID:
		return e.prepareRoleStateOrGroupChange(ctx, plan)
	case UserOrganizationChangeCommandID:
		return e.applyUserOrganizationChange(ctx, plan)
	case RoleMoveCommandID:
		return e.applyRoleMove(ctx, plan)
	case RoleDisableCommandID:
		return e.applyRoleDisable(ctx, plan)
	case RolePermissionPrepareCommandID:
		return e.prepareRolePermissions(ctx, plan)
	case RolePermissionsReplaceCommandID:
		return e.applyRolePermissions(ctx, plan)
	case ResourceLifecyclePrepareCommandID:
		return e.prepareResourceLifecycle(ctx, plan)
	case ResourceLifecycleApplyCommandID:
		return e.applyResourceLifecycle(ctx, plan)
	default:
		return serviceobject.CommandResult{}, serviceobject.ErrObjectUnavailable
	}
}

func previewMatchesImpactQuery(queryID, operation string) bool {
	switch queryID {
	case OrganizationRoleGroupImpactQueryID:
		return operation == organizationRoleGroupPreviewOperation
	case UserOrganizationImpactQueryID:
		return operation == userOrganizationPreviewOperation
	case RoleStateOrGroupImpactQueryID:
		return operation == roleMovePreviewOperation || operation == roleDisablePreviewOperation
	case RolePermissionImpactQueryID:
		return operation == rolePermissionsPreviewOperation
	case ResourceLifecycleImpactQueryID:
		return operation == resourceLifecyclePreviewOperation
	default:
		return false
	}
}

func (e *ServiceObjectExecutor) prepareOrganizationRoleGroups(ctx context.Context, plan serviceobject.DomainCommandPlan) (serviceobject.CommandResult, error) {
	actor := strings.TrimSpace(plan.Execution.Actor.Username)
	orgUnitCode, _ := plan.Arguments["orgUnitCode"].(string)
	roleGroupCodes, _ := plan.Arguments["roleGroupCodes"].([]string)
	serviceRemediations, _ := plan.Arguments["remediations"].([]serviceobject.RoleRemediation)
	if actor == "" || orgUnitCode == "" {
		return serviceobject.CommandResult{}, serviceobject.ErrValidation
	}
	impact, err := e.repository.PreviewOrgUnitRoleGroups(ctx, orgUnitCode, roleGroupCodes)
	if err != nil {
		return serviceobject.CommandResult{}, mapServiceObjectError(err)
	}
	remediations := make([]RoleAssignmentRemediation, 0, len(serviceRemediations))
	for _, remediation := range serviceRemediations {
		remediations = append(remediations, RoleAssignmentRemediation{
			UserCode: remediation.UserCode, RoleCode: remediation.RoleCode, Action: string(remediation.Action), ReplacementRoleCode: remediation.ReplacementRoleCode,
		})
	}
	changeSet := organizationRoleGroupChangeSet{OrgUnitCode: orgUnitCode, TenantCode: impact.TenantCode, RoleGroupCodes: impact.ProposedRoleGroupCodes, Remediations: remediations}
	changeSetJSON, changeSetHash, err := canonicalHash(changeSet)
	if err != nil {
		return serviceobject.CommandResult{}, serviceobject.ErrExecutionFailed
	}
	scopeHash, err := serviceObjectScopeHash(actor, plan.TenantID, plan.Scope)
	if err != nil {
		return serviceobject.CommandResult{}, serviceobject.ErrExecutionFailed
	}
	impactHash := hashText(fmt.Sprintf("%d\n%s\n%s", impact.ExpectedRevision, changeSetHash, scopeHash))
	severity := "low"
	if len(impact.Conflicts) > 0 {
		severity = "high"
	} else if !equalStrings(impact.CurrentRoleGroupCodes, impact.ProposedRoleGroupCodes) {
		severity = "medium"
	}
	summary := organizationRoleGroupImpactSummary{
		Severity: severity, AffectedUsers: impact.AffectedUsers, ConflictCount: len(impact.Conflicts),
		CurrentGroupCount: len(impact.CurrentRoleGroupCodes), TargetGroupCount: len(impact.ProposedRoleGroupCodes),
		Conflicts: impact.Conflicts,
	}
	summaryJSON, err := json.Marshal(summary)
	if err != nil {
		return serviceobject.CommandResult{}, serviceobject.ErrExecutionFailed
	}
	now := e.now().UTC()
	preview := gormOrganizationRBACPreview{
		ID: newOpaqueID(), Operation: organizationRoleGroupPreviewOperation, OwnerActor: actor,
		TenantCode: strings.TrimSpace(plan.TenantID), ScopeHash: scopeHash, ExpectedRevision: impact.ExpectedRevision,
		ImpactHash: impactHash, ChangeSetHash: changeSetHash, ChangeSetJSON: string(changeSetJSON), SummaryJSON: string(summaryJSON),
		Status: organizationRoleGroupPreviewStatusPrepared, ExpiresAt: now.Add(e.ttl), CreatedAt: now, UpdatedAt: now,
	}
	if preview.ID == "" || e.repository.db.WithContext(ctx).Create(&preview).Error != nil {
		return serviceobject.CommandResult{}, serviceobject.ErrExecutionFailed
	}
	return serviceobject.CommandResult{Values: map[string]any{
		"previewId": preview.ID, "expectedRevision": int64(preview.ExpectedRevision),
		"impactHash": preview.ImpactHash, "expiresAt": preview.ExpiresAt.Format(time.RFC3339),
	}}, nil
}

func (e *ServiceObjectExecutor) applyOrganizationRoleGroups(ctx context.Context, plan serviceobject.DomainCommandPlan) (serviceobject.CommandResult, error) {
	actor := strings.TrimSpace(plan.Execution.Actor.Username)
	previewID, _ := plan.Arguments["previewId"].(string)
	expectedRevision, ok := plan.Arguments["expectedRevision"].(int64)
	impactHash, _ := plan.Arguments["impactHash"].(string)
	if actor == "" || previewID == "" || !ok || expectedRevision < 0 || impactHash == "" {
		return serviceobject.CommandResult{}, serviceobject.ErrValidation
	}
	var result serviceobject.CommandResult
	err := e.repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var preview gormOrganizationRBACPreview
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", previewID).Take(&preview).Error; err != nil {
			return serviceobject.ErrObjectUnavailable
		}
		scopeHash, err := serviceObjectScopeHash(actor, plan.TenantID, plan.Scope)
		if err != nil || preview.OwnerActor != actor || preview.TenantCode != strings.TrimSpace(plan.TenantID) || preview.ScopeHash != scopeHash {
			return serviceobject.ErrObjectUnavailable
		}
		if preview.Status == organizationRoleGroupPreviewStatusApplied {
			var stored organizationRoleGroupAppliedResult
			if err := json.Unmarshal([]byte(preview.AppliedResultJSON), &stored); err != nil {
				return serviceobject.ErrExecutionFailed
			}
			result = serviceobject.CommandResult{Values: map[string]any{"applied": stored.Applied, "revision": stored.Revision, "previewId": stored.PreviewID}}
			return nil
		}
		if e.now().UTC().After(preview.ExpiresAt) || preview.ExpectedRevision != uint64(expectedRevision) || preview.ImpactHash != impactHash {
			return serviceobject.ErrConflict
		}
		var changeSet organizationRoleGroupChangeSet
		if err := json.Unmarshal([]byte(preview.ChangeSetJSON), &changeSet); err != nil {
			return serviceobject.ErrExecutionFailed
		}
		current, err := (&GORMRepository{db: tx}).LoadOrgUnitRoleGroups(ctx, changeSet.OrgUnitCode)
		if err != nil || current.Revision != preview.ExpectedRevision {
			return serviceobject.ErrConflict
		}
		committed, err := (&GORMRepository{db: tx}).ReplaceOrgUnitRoleGroupsWithRemediations(ctx, ReplaceOrgUnitRoleGroupsRequest{
			OrgUnitCode: changeSet.OrgUnitCode, RoleGroupCodes: changeSet.RoleGroupCodes,
			ExpectedRevision: preview.ExpectedRevision, ActorID: actor, ChangedAt: e.now().UTC(),
		}, changeSet.Remediations)
		if err != nil {
			return mapServiceObjectError(err)
		}
		var summary organizationRoleGroupImpactSummary
		_ = json.Unmarshal([]byte(preview.SummaryJSON), &summary)
		auditID := newOpaqueID()
		if auditID == "" {
			return serviceobject.ErrExecutionFailed
		}
		if err := tx.Create(&gormOrganizationRBACAuditEvent{
			ID: auditID, ActorType: "user", ActorCode: actor, TenantCode: changeSet.TenantCode, OrgUnitCode: changeSet.OrgUnitCode,
			Action: OrganizationRoleGroupsReplaceCommandID, BeforeRevision: preview.ExpectedRevision, AfterRevision: committed.Revision,
			ImpactHash: preview.ImpactHash, PreviewID: preview.ID, ConflictCount: summary.ConflictCount, CreatedAt: e.now().UTC(),
		}).Error; err != nil {
			return serviceobject.ErrExecutionFailed
		}
		result = serviceobject.CommandResult{Values: map[string]any{"applied": true, "revision": int64(committed.Revision), "previewId": preview.ID}}
		encodedResult, err := json.Marshal(organizationRoleGroupAppliedResult{Applied: true, Revision: int64(committed.Revision), PreviewID: preview.ID})
		if err != nil {
			return serviceobject.ErrExecutionFailed
		}
		update := tx.Model(&gormOrganizationRBACPreview{}).
			Where("id = ? AND status = ?", preview.ID, organizationRoleGroupPreviewStatusPrepared).
			Updates(map[string]any{"status": organizationRoleGroupPreviewStatusApplied, "applied_result_json": string(encodedResult), "updated_at": e.now().UTC()})
		if update.Error != nil || update.RowsAffected != 1 {
			return serviceobject.ErrConflict
		}
		return nil
	})
	if err != nil {
		return serviceobject.CommandResult{}, err
	}
	return result, nil
}

func (e *ServiceObjectExecutor) loadPreviewForPlan(ctx context.Context, actor, tenant string, scope serviceobject.ScopeConstraint, previewID string) (gormOrganizationRBACPreview, organizationRoleGroupImpactSummary, error) {
	if strings.TrimSpace(actor) == "" || strings.TrimSpace(previewID) == "" {
		return gormOrganizationRBACPreview{}, organizationRoleGroupImpactSummary{}, serviceobject.ErrObjectUnavailable
	}
	scopeHash, err := serviceObjectScopeHash(actor, tenant, scope)
	if err != nil {
		return gormOrganizationRBACPreview{}, organizationRoleGroupImpactSummary{}, serviceobject.ErrExecutionFailed
	}
	var preview gormOrganizationRBACPreview
	if err := e.repository.db.WithContext(ctx).
		Where("id = ? AND owner_actor = ? AND tenant_code = ? AND scope_hash = ?", previewID, actor, strings.TrimSpace(tenant), scopeHash).
		Take(&preview).Error; err != nil {
		return gormOrganizationRBACPreview{}, organizationRoleGroupImpactSummary{}, serviceobject.ErrObjectUnavailable
	}
	if e.now().UTC().After(preview.ExpiresAt) {
		return gormOrganizationRBACPreview{}, organizationRoleGroupImpactSummary{}, serviceobject.ErrConflict
	}
	var summary organizationRoleGroupImpactSummary
	if err := json.Unmarshal([]byte(preview.SummaryJSON), &summary); err != nil {
		return gormOrganizationRBACPreview{}, organizationRoleGroupImpactSummary{}, serviceobject.ErrExecutionFailed
	}
	return preview, summary, nil
}

func predicateString(ast serviceobject.QueryAST, field string) (string, bool) {
	for _, predicate := range ast.Predicates {
		if predicate.Field == field && predicate.Operator == serviceobject.PredicateEqual {
			value, ok := predicate.Value.(string)
			return strings.TrimSpace(value), ok && strings.TrimSpace(value) != ""
		}
	}
	return "", false
}

func serviceObjectScopeHash(actor, tenant string, scope serviceobject.ScopeConstraint) (string, error) {
	sort.Strings(scope.OrgCodes)
	sort.Strings(scope.AreaCodes)
	sort.Strings(scope.ActorIdentifiers)
	encoded, err := json.Marshal(struct {
		Actor  string                        `json:"actor"`
		Tenant string                        `json:"tenant"`
		Scope  serviceobject.ScopeConstraint `json:"scope"`
	}{Actor: strings.TrimSpace(actor), Tenant: strings.TrimSpace(tenant), Scope: scope})
	if err != nil {
		return "", err
	}
	return hashText(string(encoded)), nil
}

func canonicalHash(value any) ([]byte, string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, "", err
	}
	return encoded, hashText(string(encoded)), nil
}

func hashText(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func newOpaqueID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return ""
	}
	return hex.EncodeToString(bytes)
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func mapServiceObjectError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrNotFound):
		return serviceobject.ErrObjectUnavailable
	case errors.Is(err, ErrRevisionConflict), errors.Is(err, ErrRolePoolViolation):
		return serviceobject.ErrConflict
	case errors.Is(err, ErrInvalid):
		return serviceobject.ErrValidation
	default:
		return serviceobject.ErrExecutionFailed
	}
}
