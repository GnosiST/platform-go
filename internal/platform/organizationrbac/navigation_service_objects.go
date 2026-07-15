package organizationrbac

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"platform-go/internal/platform/serviceobject"

	"gorm.io/gorm"
)

type ServiceObjectExecutorOptions struct {
	RoleMenuWriteEnabled bool
}

type roleMenuChangeSet struct {
	RoleCode          string   `json:"roleCode"`
	TenantCode        string   `json:"tenantCode"`
	CurrentMenuCodes  []string `json:"currentMenuCodes"`
	ProposedMenuCodes []string `json:"proposedMenuCodes"`
	Changed           bool     `json:"changed"`
}

const (
	navigationResource              = "menus"
	MenuDefinitionGetQueryID        = "platform.navigation.menu-definition.get"
	RoleMenusGetQueryID             = "platform.navigation.role-menus.get"
	RoleMenuImpactQueryID           = "platform.navigation.role-menu-change.impact"
	RoleMenuMigrationCompareQueryID = "platform.navigation.role-menu-migration.compare"
	MenuDefinitionCreateCommandID   = "platform.navigation.menu-definition.create"
	MenuDefinitionReplaceCommandID  = "platform.navigation.menu-definition.replace"
	RoleMenuPrepareCommandID        = "platform.navigation.role-menu-change.prepare"
	RoleMenusReplaceCommandID       = "platform.navigation.role-menus.replace"
	roleMenusPreviewOperation       = "role-menus.replace"
)

func navigationQueryDefinitions() []serviceobject.QueryDefinition {
	readMenu := []serviceobject.PermissionRequirement{{Permission: "admin:menu:read", Action: "read"}}
	return []serviceobject.QueryDefinition{
		{
			ID: MenuDefinitionGetQueryID, Version: ServiceObjectVersion, Resource: navigationResource,
			Permission: "admin:menu:read", Action: "read", TenantMode: serviceobject.TenantPlatform, DataScope: "platform",
			Arguments: []serviceobject.ArgumentDefinition{{Name: "menuCode", Type: serviceobject.ValueString, Required: true, MaxLength: 191}},
			Cost:      serviceobject.CostPolicy{BaseCost: 1, PerRowCost: 1, PredicateCost: 1, Limit: 8}, Timeout: 2 * time.Second, MaxPageSize: 1,
			ResultSchema: []serviceobject.ResultField{{Name: "definition", Type: serviceobject.ValueMenuDefinition}, {Name: "revision", Type: serviceobject.ValueInteger}},
			Build:        navigationScalarQueryBuilder("menuCode"),
		},
		{
			ID: RoleMenusGetQueryID, Version: ServiceObjectVersion, Resource: navigationResource,
			Permission: "admin:role:read", Action: "read", AdditionalPermissions: readMenu,
			TenantMode: serviceobject.TenantPlatform, DataScope: "platform",
			Arguments: []serviceobject.ArgumentDefinition{{Name: "roleCode", Type: serviceobject.ValueString, Required: true, MaxLength: 191}},
			Cost:      serviceobject.CostPolicy{BaseCost: 1, PerRowCost: 1, PredicateCost: 1, Limit: 8}, Timeout: 2 * time.Second, MaxPageSize: 1,
			ResultSchema: []serviceobject.ResultField{
				{Name: "roleCode", Type: serviceobject.ValueString}, {Name: "menuCodes", Type: serviceobject.ValueStringSet}, {Name: "revision", Type: serviceobject.ValueInteger},
			},
			Build: navigationScalarQueryBuilder("roleCode"),
		},
		{
			ID: RoleMenuImpactQueryID, Version: ServiceObjectVersion, Resource: navigationResource,
			Permission: "admin:role:update", Action: "update", AdditionalPermissions: readMenu,
			TenantMode: serviceobject.TenantPlatform, DataScope: "platform",
			Arguments: []serviceobject.ArgumentDefinition{{Name: "previewId", Type: serviceobject.ValueString, Required: true, MaxLength: 64}},
			Cost:      serviceobject.CostPolicy{BaseCost: 1, PerRowCost: 1, PredicateCost: 1, Limit: 8}, Timeout: 2 * time.Second, MaxPageSize: 1,
			ResultSchema: []serviceobject.ResultField{
				{Name: "previewId", Type: serviceobject.ValueString}, {Name: "changed", Type: serviceobject.ValueBoolean},
				{Name: "currentMenuCodes", Type: serviceobject.ValueStringSet}, {Name: "proposedMenuCodes", Type: serviceobject.ValueStringSet},
				{Name: "expectedRevision", Type: serviceobject.ValueInteger}, {Name: "impactHash", Type: serviceobject.ValueString},
				{Name: "expiresAt", Type: serviceobject.ValueString},
			},
			Build: navigationScalarQueryBuilder("previewId"),
		},
		{
			ID: RoleMenuMigrationCompareQueryID, Version: ServiceObjectVersion, Resource: navigationResource,
			Permission: "admin:role:read", Action: "read", AdditionalPermissions: readMenu,
			TenantMode: serviceobject.TenantPlatform, DataScope: "platform",
			Arguments: []serviceobject.ArgumentDefinition{{Name: "roleCode", Type: serviceobject.ValueString, Required: true, MaxLength: 191}},
			Cost:      serviceobject.CostPolicy{BaseCost: 1, PerRowCost: 1, PredicateCost: 1, Limit: 8}, Timeout: 3 * time.Second, MaxPageSize: 1,
			ResultSchema: []serviceobject.ResultField{
				{Name: "roleCode", Type: serviceobject.ValueString}, {Name: "legacyMenuCodes", Type: serviceobject.ValueStringSet},
				{Name: "targetMenuCodes", Type: serviceobject.ValueStringSet}, {Name: "addedMenuCodes", Type: serviceobject.ValueStringSet},
				{Name: "removedMenuCodes", Type: serviceobject.ValueStringSet}, {Name: "targetRevision", Type: serviceobject.ValueInteger},
				{Name: "principalEquivalenceClaimed", Type: serviceobject.ValueBoolean},
			},
			Build: navigationScalarQueryBuilder("roleCode"),
		},
	}
}

func navigationDomainCommandDefinitions() []serviceobject.DomainCommandDefinition {
	baseCost := serviceobject.CostPolicy{BaseCost: 5, PerRowCost: 1, Limit: 2005}
	readMenu := []serviceobject.PermissionRequirement{{Permission: "admin:menu:read", Action: "read"}}
	return []serviceobject.DomainCommandDefinition{
		{
			ID: MenuDefinitionCreateCommandID, Version: ServiceObjectVersion, Resource: navigationResource,
			Permission: "admin:menu:create", Action: "create", TenantMode: serviceobject.TenantPlatform, DataScope: "platform",
			Arguments: menuDefinitionMutationArguments(), Cost: baseCost, Timeout: 10 * time.Second,
			Idempotency: serviceobject.IdempotencyRequiredKey, MaxAffectedRows: 2000, ResultSchema: menuDefinitionMutationResultSchema(),
		},
		{
			ID: MenuDefinitionReplaceCommandID, Version: ServiceObjectVersion, Resource: navigationResource,
			Permission: "admin:menu:update", Action: "update", TenantMode: serviceobject.TenantPlatform, DataScope: "platform",
			Arguments: menuDefinitionMutationArguments(), Cost: baseCost, Timeout: 10 * time.Second,
			Idempotency: serviceobject.IdempotencyRequiredKey, MaxAffectedRows: 2000, ResultSchema: menuDefinitionMutationResultSchema(),
		},
		{
			ID: RoleMenuPrepareCommandID, Version: ServiceObjectVersion, Resource: navigationResource,
			Permission: "admin:role:update", Action: "update", AdditionalPermissions: readMenu,
			TenantMode: serviceobject.TenantPlatform, DataScope: "platform",
			Arguments: []serviceobject.ArgumentDefinition{
				{Name: "roleCode", Type: serviceobject.ValueString, Required: true, MaxLength: 191},
				{Name: "menuCodes", Type: serviceobject.ValueStringSet, Required: true, MaxLength: 191},
			},
			Cost: baseCost, Timeout: 5 * time.Second, Idempotency: serviceobject.IdempotencyRequiredKey, MaxAffectedRows: 2000,
			ResultSchema: previewResultSchema(),
		},
		{
			ID: RoleMenusReplaceCommandID, Version: ServiceObjectVersion, Resource: navigationResource,
			Permission: "admin:role:update", Action: "update", AdditionalPermissions: readMenu,
			TenantMode: serviceobject.TenantPlatform, DataScope: "platform",
			Arguments: []serviceobject.ArgumentDefinition{
				{Name: "previewId", Type: serviceobject.ValueString, Required: true, MaxLength: 64},
				{Name: "expectedRevision", Type: serviceobject.ValueInteger, Required: true},
				{Name: "impactHash", Type: serviceobject.ValueString, Required: true, MaxLength: 64},
			},
			Cost: baseCost, Timeout: 10 * time.Second, Idempotency: serviceobject.IdempotencyRequiredKey, MaxAffectedRows: 2000,
			ResultSchema: []serviceobject.ResultField{
				{Name: "applied", Type: serviceobject.ValueBoolean}, {Name: "revision", Type: serviceobject.ValueInteger}, {Name: "previewId", Type: serviceobject.ValueString},
			},
		},
	}
}

func navigationScalarQueryBuilder(argument string) serviceobject.QueryBuilder {
	return func(arguments serviceobject.ValidatedArguments) (serviceobject.QueryAST, error) {
		return serviceobject.QueryAST{Resource: navigationResource, Predicates: []serviceobject.Predicate{{Field: argument, Operator: serviceobject.PredicateEqual, Value: arguments[argument]}}}, nil
	}
}

func menuDefinitionMutationArguments() []serviceobject.ArgumentDefinition {
	return []serviceobject.ArgumentDefinition{
		{Name: "definition", Type: serviceobject.ValueMenuDefinition, Required: true},
		{Name: "expectedRevision", Type: serviceobject.ValueInteger, Required: true},
	}
}

func menuDefinitionMutationResultSchema() []serviceobject.ResultField {
	return []serviceobject.ResultField{{Name: "applied", Type: serviceobject.ValueBoolean}, {Name: "revision", Type: serviceobject.ValueInteger}}
}

func (e *ServiceObjectExecutor) getMenuDefinition(ctx context.Context, plan serviceobject.QueryPlan) (serviceobject.QueryResult, error) {
	menuCode, ok := predicateString(plan.AST, "menuCode")
	if !ok {
		return serviceobject.QueryResult{}, serviceobject.ErrValidation
	}
	definition, revision, err := e.repository.LoadMenuDefinition(ctx, menuCode)
	if err != nil {
		return serviceobject.QueryResult{}, mapServiceObjectError(err)
	}
	return serviceobject.QueryResult{Items: []map[string]any{{
		"definition": serviceMenuDefinition(definition), "revision": int64(revision),
	}}}, nil
}

func (e *ServiceObjectExecutor) getRoleMenus(ctx context.Context, plan serviceobject.QueryPlan) (serviceobject.QueryResult, error) {
	roleCode, ok := predicateString(plan.AST, "roleCode")
	if !ok {
		return serviceobject.QueryResult{}, serviceobject.ErrValidation
	}
	current, err := e.repository.LoadRoleMenus(ctx, roleCode)
	if err != nil {
		return serviceobject.QueryResult{}, mapServiceObjectError(err)
	}
	if _, err := e.repository.PreviewRoleMenus(ctx, roleCode, current.MenuCodes); err != nil {
		return serviceobject.QueryResult{}, mapServiceObjectError(err)
	}
	return serviceobject.QueryResult{Items: []map[string]any{{
		"roleCode": current.RoleCode, "menuCodes": current.MenuCodes, "revision": int64(current.Revision),
	}}}, nil
}

func (e *ServiceObjectExecutor) getRoleMenuImpact(ctx context.Context, plan serviceobject.QueryPlan) (serviceobject.QueryResult, error) {
	previewID, ok := predicateString(plan.AST, "previewId")
	if !ok {
		return serviceobject.QueryResult{}, serviceobject.ErrValidation
	}
	preview, _, err := e.loadPreviewForPlan(ctx, plan.Execution.Actor.Username, plan.TenantID, plan.Scope, previewID)
	if err != nil {
		return serviceobject.QueryResult{}, err
	}
	if preview.Operation != roleMenusPreviewOperation {
		return serviceobject.QueryResult{}, serviceobject.ErrObjectUnavailable
	}
	var changeSet roleMenuChangeSet
	if err := json.Unmarshal([]byte(preview.ChangeSetJSON), &changeSet); err != nil {
		return serviceobject.QueryResult{}, serviceobject.ErrExecutionFailed
	}
	return serviceobject.QueryResult{Items: []map[string]any{{
		"previewId": preview.ID, "changed": changeSet.Changed,
		"currentMenuCodes": changeSet.CurrentMenuCodes, "proposedMenuCodes": changeSet.ProposedMenuCodes,
		"expectedRevision": int64(preview.ExpectedRevision), "impactHash": preview.ImpactHash,
		"expiresAt": preview.ExpiresAt.UTC().Format(time.RFC3339),
	}}}, nil
}

func (e *ServiceObjectExecutor) compareRoleMenuMigration(ctx context.Context, plan serviceobject.QueryPlan) (serviceobject.QueryResult, error) {
	roleCode, ok := predicateString(plan.AST, "roleCode")
	if !ok {
		return serviceobject.QueryResult{}, serviceobject.ErrValidation
	}
	comparison, err := e.repository.CompareRoleMenuCandidate(ctx, roleCode)
	if err != nil {
		return serviceobject.QueryResult{}, mapServiceObjectError(err)
	}
	return serviceobject.QueryResult{Items: []map[string]any{{
		"roleCode": comparison.RoleCode, "legacyMenuCodes": comparison.LegacyMenuCodes,
		"targetMenuCodes": comparison.TargetMenuCodes, "addedMenuCodes": comparison.AddedMenuCodes,
		"removedMenuCodes": comparison.RemovedMenuCodes, "targetRevision": int64(comparison.TargetRevision),
		"principalEquivalenceClaimed": false,
	}}}, nil
}

func (e *ServiceObjectExecutor) createMenuDefinition(ctx context.Context, plan serviceobject.DomainCommandPlan) (serviceobject.CommandResult, error) {
	definition, expectedRevision, actor, ok := menuDefinitionMutationPlan(plan)
	if !ok {
		return serviceobject.CommandResult{}, serviceobject.ErrValidation
	}
	var existing int64
	if err := e.repository.db.WithContext(ctx).Model(&gormMenu{}).
		Where("code = ? OR id = ?", definition.Node.Code, definition.ID).Count(&existing).Error; err != nil {
		return serviceobject.CommandResult{}, serviceobject.ErrExecutionFailed
	}
	if existing != 0 {
		return serviceobject.CommandResult{}, serviceobject.ErrConflict
	}
	return e.commitMenuDefinition(ctx, definition, expectedRevision, actor)
}

func (e *ServiceObjectExecutor) replaceMenuDefinition(ctx context.Context, plan serviceobject.DomainCommandPlan) (serviceobject.CommandResult, error) {
	definition, expectedRevision, actor, ok := menuDefinitionMutationPlan(plan)
	if !ok {
		return serviceobject.CommandResult{}, serviceobject.ErrValidation
	}
	if _, _, err := e.repository.LoadMenuDefinition(ctx, definition.Node.Code); err != nil {
		return serviceobject.CommandResult{}, mapServiceObjectError(err)
	}
	return e.commitMenuDefinition(ctx, definition, expectedRevision, actor)
}

func (e *ServiceObjectExecutor) prepareRoleMenus(ctx context.Context, plan serviceobject.DomainCommandPlan) (serviceobject.CommandResult, error) {
	if !e.roleMenuWriteEnabled {
		return serviceobject.CommandResult{}, serviceobject.ErrObjectUnavailable
	}
	actor := strings.TrimSpace(plan.Execution.Actor.Username)
	roleCode, _ := plan.Arguments["roleCode"].(string)
	menuCodes, _ := plan.Arguments["menuCodes"].([]string)
	if actor == "" || !validCode(roleCode) {
		return serviceobject.CommandResult{}, serviceobject.ErrValidation
	}
	impact, err := e.repository.PreviewRoleMenus(ctx, roleCode, menuCodes)
	if err != nil {
		return serviceobject.CommandResult{}, mapServiceObjectError(err)
	}
	severity := "low"
	if impact.Changed {
		severity = "medium"
	}
	return e.storeDomainPreview(ctx, plan, roleMenusPreviewOperation, impact.ExpectedRevision, roleMenuChangeSet{
		RoleCode: impact.RoleCode, TenantCode: impact.TenantCode, CurrentMenuCodes: impact.CurrentMenuCodes,
		ProposedMenuCodes: impact.ProposedMenuCodes, Changed: impact.Changed,
	}, organizationRoleGroupImpactSummary{
		Severity: severity, CurrentGroupCount: len(impact.CurrentMenuCodes), TargetGroupCount: len(impact.ProposedMenuCodes),
	})
}

func (e *ServiceObjectExecutor) applyRoleMenus(ctx context.Context, plan serviceobject.DomainCommandPlan) (serviceobject.CommandResult, error) {
	if !e.roleMenuWriteEnabled {
		return serviceobject.CommandResult{}, serviceobject.ErrObjectUnavailable
	}
	return e.applyIdentityPreview(ctx, plan, roleMenusPreviewOperation, func(tx *gorm.DB, preview gormOrganizationRBACPreview, actor string) (uint64, string, string, error) {
		var changeSet roleMenuChangeSet
		if err := json.Unmarshal([]byte(preview.ChangeSetJSON), &changeSet); err != nil {
			return 0, "", "", serviceobject.ErrExecutionFailed
		}
		transactionRepository := &GORMRepository{db: tx}
		current, err := transactionRepository.PreviewRoleMenus(ctx, changeSet.RoleCode, changeSet.ProposedMenuCodes)
		if err != nil {
			return 0, "", "", mapServiceObjectError(err)
		}
		if current.ExpectedRevision != preview.ExpectedRevision ||
			!equalStrings(current.CurrentMenuCodes, changeSet.CurrentMenuCodes) ||
			!equalStrings(current.ProposedMenuCodes, changeSet.ProposedMenuCodes) || current.Changed != changeSet.Changed {
			return 0, "", "", serviceobject.ErrConflict
		}
		committed, err := transactionRepository.ReplaceRoleMenus(ctx, ReplaceRoleMenusRequest{
			RoleCode: changeSet.RoleCode, MenuCodes: changeSet.ProposedMenuCodes,
			ExpectedRevision: preview.ExpectedRevision, ActorID: actor, ChangedAt: e.now().UTC(),
		})
		return committed.Revision, changeSet.TenantCode, "", mapServiceObjectError(err)
	})
}

func (e *ServiceObjectExecutor) commitMenuDefinition(ctx context.Context, definition serviceobject.MenuDefinition, expectedRevision uint64, actor string) (serviceobject.CommandResult, error) {
	revision, err := e.repository.ReplaceMenuDefinition(ctx, ReplaceMenuDefinitionRequest{
		Definition: organizationMenuDefinition(definition), ExpectedRevision: expectedRevision,
		ActorID: actor, ChangedAt: e.now().UTC(),
	})
	if err != nil {
		return serviceobject.CommandResult{}, mapServiceObjectError(err)
	}
	return serviceobject.CommandResult{Values: map[string]any{"applied": true, "revision": int64(revision)}}, nil
}

func menuDefinitionMutationPlan(plan serviceobject.DomainCommandPlan) (serviceobject.MenuDefinition, uint64, string, bool) {
	definition, definitionOK := plan.Arguments["definition"].(serviceobject.MenuDefinition)
	expectedRevision, revisionOK := plan.Arguments["expectedRevision"].(int64)
	actor := strings.TrimSpace(plan.Execution.Actor.Username)
	if !definitionOK || !revisionOK || expectedRevision < 0 || actor == "" {
		return serviceobject.MenuDefinition{}, 0, "", false
	}
	return definition, uint64(expectedRevision), actor, true
}

func organizationMenuDefinition(definition serviceobject.MenuDefinition) MenuDefinition {
	parameters := make([]MenuParameter, 0, len(definition.Node.Parameters))
	for _, parameter := range definition.Node.Parameters {
		parameters = append(parameters, MenuParameter{Key: parameter.Key, Type: MenuParameterType(parameter.Type), Value: parameter.Value})
	}
	buttons := make([]PageButton, 0, len(definition.Buttons))
	for _, button := range definition.Buttons {
		buttons = append(buttons, PageButton{
			MenuCode: button.MenuCode, ButtonKey: button.ButtonKey, LabelZH: button.LabelZH, LabelEN: button.LabelEN,
			Action: button.Action, SortOrder: button.SortOrder, Status: button.Status, PermissionCode: button.PermissionCode,
		})
	}
	node := definition.Node
	return MenuDefinition{
		ID: definition.ID, Name: definition.Name, Description: definition.Description,
		Node: MenuNode{
			Code: node.Code, ParentCode: node.ParentCode, NodeType: MenuNodeType(node.NodeType), TitleZH: node.TitleZH, TitleEN: node.TitleEN,
			DescriptionZH: node.DescriptionZH, DescriptionEN: node.DescriptionEN, Status: node.Status, Icon: node.Icon, SortOrder: node.SortOrder,
			Route: node.Route, ComponentKey: node.ComponentKey, ResourceCode: node.ResourceCode, External: node.External, ExternalURL: node.ExternalURL,
			OpenMode: MenuOpenMode(node.OpenMode), Parameters: parameters, CacheEnabled: node.CacheEnabled, Hidden: node.Hidden,
			ActiveMenuCode: node.ActiveMenuCode, BreadcrumbVisible: node.BreadcrumbVisible,
		},
		Buttons: buttons,
	}
}

func serviceMenuDefinition(definition MenuDefinition) serviceobject.MenuDefinition {
	parameters := make([]serviceobject.MenuParameter, 0, len(definition.Node.Parameters))
	for _, parameter := range definition.Node.Parameters {
		parameters = append(parameters, serviceobject.MenuParameter{Key: parameter.Key, Type: serviceobject.MenuParameterType(parameter.Type), Value: parameter.Value})
	}
	buttons := make([]serviceobject.PageButton, 0, len(definition.Buttons))
	for _, button := range definition.Buttons {
		buttons = append(buttons, serviceobject.PageButton{
			MenuCode: button.MenuCode, ButtonKey: button.ButtonKey, LabelZH: button.LabelZH, LabelEN: button.LabelEN,
			Action: button.Action, SortOrder: button.SortOrder, Status: button.Status, PermissionCode: button.PermissionCode,
		})
	}
	node := definition.Node
	return serviceobject.MenuDefinition{
		ID: definition.ID, Name: definition.Name, Description: definition.Description, UpdatedAt: definition.UpdatedAt,
		Node: serviceobject.MenuNode{
			Code: node.Code, ParentCode: node.ParentCode, NodeType: serviceobject.MenuNodeType(node.NodeType), TitleZH: node.TitleZH, TitleEN: node.TitleEN,
			DescriptionZH: node.DescriptionZH, DescriptionEN: node.DescriptionEN, Status: node.Status, Icon: node.Icon, SortOrder: node.SortOrder,
			Route: node.Route, ComponentKey: node.ComponentKey, ResourceCode: node.ResourceCode, External: node.External, ExternalURL: node.ExternalURL,
			OpenMode: serviceobject.MenuOpenMode(node.OpenMode), Parameters: parameters, CacheEnabled: node.CacheEnabled, Hidden: node.Hidden,
			ActiveMenuCode: node.ActiveMenuCode, BreadcrumbVisible: node.BreadcrumbVisible,
		},
		Buttons: buttons,
	}
}
