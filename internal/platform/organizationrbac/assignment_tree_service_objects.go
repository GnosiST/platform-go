package organizationrbac

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/GnosiST/platform-go/internal/platform/serviceobject"
)

const (
	MenuAssignmentTreeSearchQueryID        = "platform.navigation.menu-assignment-tree.search"
	MenuAssignmentTreeHydrateQueryID       = "platform.navigation.menu-assignment-tree.hydrate"
	PermissionAssignmentTreeSearchQueryID  = "platform.authorization.permission-assignment-tree.search"
	PermissionAssignmentTreeHydrateQueryID = "platform.authorization.permission-assignment-tree.hydrate"
)

func assignmentTreeResultSchema(menu bool) []serviceobject.ResultField {
	result := []serviceobject.ResultField{{Name: "code", Type: serviceobject.ValueString}, {Name: "name", Type: serviceobject.ValueString}, {Name: "parentCode", Type: serviceobject.ValueString}, {Name: "status", Type: serviceobject.ValueString}, {Name: "selected", Type: serviceobject.ValueBoolean}, {Name: "disabledReason", Type: serviceobject.ValueString}}
	if menu {
		return append(result, serviceobject.ResultField{Name: "nodeType", Type: serviceobject.ValueString})
	}
	return append(result, serviceobject.ResultField{Name: "resourceType", Type: serviceobject.ValueString})
}

func assignmentTreeQueryDefinitions() []serviceobject.QueryDefinition {
	menuArgs := []serviceobject.ArgumentDefinition{{Name: "roleCode", Type: serviceobject.ValueString, Required: true, MaxLength: 191}, {Name: "query", Type: serviceobject.ValueString, MaxLength: 191}}
	permissionArgs := append([]serviceobject.ArgumentDefinition(nil), menuArgs...)
	return []serviceobject.QueryDefinition{
		assignmentTreeQueryDefinition(MenuAssignmentTreeSearchQueryID, "menus", menuArgs, 100, true, false),
		assignmentTreeQueryDefinition(MenuAssignmentTreeHydrateQueryID, "menus", menuArgs[:1], 1000, true, true),
		assignmentTreeQueryDefinition(PermissionAssignmentTreeSearchQueryID, "permissions", permissionArgs, 100, false, false),
		assignmentTreeQueryDefinition(PermissionAssignmentTreeHydrateQueryID, "permissions", permissionArgs[:1], 1000, false, true),
	}
}

func assignmentTreeQueryDefinition(id, resource string, args []serviceobject.ArgumentDefinition, maxPage int, menu, hydrate bool) serviceobject.QueryDefinition {
	return serviceobject.QueryDefinition{ID: id, Version: ServiceObjectVersion, Resource: resource, Permission: "admin:role:read", Action: "read", TenantMode: serviceobject.TenantPlatform, DataScope: "platform", Arguments: args, Cost: serviceobject.CostPolicy{BaseCost: 2, PerRowCost: 1, PerOffsetCost: 1, PredicateCost: 1, MaxOffset: 2000, Limit: 2005}, Timeout: 3 * time.Second, MaxPageSize: maxPage, ResultSchema: assignmentTreeResultSchema(menu), Build: func(arguments serviceobject.ValidatedArguments) (serviceobject.QueryAST, error) {
		predicates := []serviceobject.Predicate{{Field: "roleCode", Operator: serviceobject.PredicateEqual, Value: arguments["roleCode"]}}
		if !hydrate {
			if query, ok := arguments["query"].(string); ok && strings.TrimSpace(query) != "" {
				predicates = append(predicates, serviceobject.Predicate{Field: "query", Operator: serviceobject.PredicateEqual, Value: strings.TrimSpace(query)})
			}
		}
		return serviceobject.QueryAST{Resource: resource, Predicates: predicates}, nil
	}}
}

func (e *ServiceObjectExecutor) executeAssignmentTreeQuery(ctx context.Context, plan serviceobject.QueryPlan) (serviceobject.QueryResult, error) {
	roleCode, ok := assignmentPredicateString(plan.AST, "roleCode")
	if !ok {
		return serviceobject.QueryResult{}, serviceobject.ErrValidation
	}
	if err := e.validateAssignmentRole(ctx, roleCode, plan); err != nil {
		return serviceobject.QueryResult{}, err
	}
	query, _ := assignmentPredicateString(plan.AST, "query")
	hydrate := strings.HasSuffix(plan.Definition.ID, ".hydrate")
	menu := plan.Definition.Resource == "menus"
	var items []map[string]any
	var err error
	if menu {
		items, err = e.repository.assignmentMenuItems(ctx, roleCode, query, hydrate)
	} else {
		items, err = e.repository.assignmentPermissionItems(ctx, roleCode, query, hydrate)
	}
	if err != nil {
		return serviceobject.QueryResult{}, mapServiceObjectError(err)
	}
	start := (plan.Page - 1) * plan.PageSize
	if start > len(items) {
		start = len(items)
	}
	end := start + plan.PageSize
	if end > len(items) {
		end = len(items)
	}
	return serviceobject.QueryResult{Items: items[start:end]}, nil
}

func (e *ServiceObjectExecutor) validateAssignmentRole(ctx context.Context, roleCode string, plan serviceobject.QueryPlan) error {
	var role gormRole
	if err := e.repository.db.WithContext(ctx).Where("code = ? AND status = ?", roleCode, StatusEnabled).Take(&role).Error; err != nil {
		return serviceobject.ErrObjectUnavailable
	}
	var group gormRoleGroup
	if err := e.repository.db.WithContext(ctx).Where("code = ? AND status = ?", role.GroupCode, StatusEnabled).Take(&group).Error; err != nil {
		return serviceobject.ErrObjectUnavailable
	}
	if plan.Execution.TenantScope.PlatformWide {
		return nil
	}
	tenant := strings.TrimSpace(plan.TenantID)
	if tenant == "" || group.ScopeType == string(ScopePlatform) || strings.TrimSpace(group.TenantCode) != tenant {
		return serviceobject.ErrObjectUnavailable
	}
	return nil
}

func assignmentPredicateString(ast serviceobject.QueryAST, field string) (string, bool) {
	for _, predicate := range ast.Predicates {
		if predicate.Field == field && predicate.Operator == serviceobject.PredicateEqual {
			value, ok := predicate.Value.(string)
			value = strings.TrimSpace(value)
			return value, ok && value != ""
		}
	}
	return "", false
}

func (r *GORMRepository) assignmentMenuItems(ctx context.Context, roleCode, query string, hydrate bool) ([]map[string]any, error) {
	var menus []gormMenu
	if err := r.db.WithContext(ctx).Order("sort_order, code").Find(&menus).Error; err != nil {
		return nil, repositoryError(err)
	}
	var bindings []gormRoleMenu
	if err := r.db.WithContext(ctx).Where("role_code = ?", roleCode).Find(&bindings).Error; err != nil {
		return nil, repositoryError(err)
	}
	selected := make(map[string]bool, len(bindings))
	for _, binding := range bindings {
		selected[binding.MenuCode] = true
	}
	byCode := make(map[string]gormMenu, len(menus))
	for _, menu := range menus {
		byCode[menu.Code] = menu
	}
	include := make(map[string]bool)
	if hydrate {
		for code := range selected {
			include[code] = true
			for parent := byCode[code].ParentCode; parent != ""; parent = byCode[parent].ParentCode {
				include[parent] = true
			}
		}
	} else {
		needle := strings.ToLower(strings.TrimSpace(query))
		for _, menu := range menus {
			if menu.NodeType != string(MenuNodeTypePage) || needle != "" && !strings.Contains(strings.ToLower(menu.Code+" "+menu.Name+" "+menu.TitleZH+" "+menu.TitleEN), needle) {
				continue
			}
			include[menu.Code] = true
			for parent := menu.ParentCode; parent != ""; parent = byCode[parent].ParentCode {
				include[parent] = true
			}
		}
	}
	for code := range selected {
		if _, exists := byCode[code]; !exists {
			include[code] = true
		}
	}
	items := make([]map[string]any, 0, len(include))
	for _, menu := range menus {
		if !include[menu.Code] {
			continue
		}
		items = append(items, map[string]any{"code": menu.Code, "name": firstNonEmpty(menu.Name, menu.TitleZH, menu.TitleEN), "parentCode": menu.ParentCode, "status": menu.Status, "selected": selected[menu.Code], "disabledReason": assignmentDisabledReason(menu.Status, false), "nodeType": menu.NodeType})
	}
	for code := range include {
		if _, exists := byCode[code]; !exists && selected[code] {
			items = append(items, map[string]any{"code": code, "name": code, "parentCode": "", "status": "disabled", "selected": true, "disabledReason": "historical selection unavailable", "nodeType": string(MenuNodeTypePage)})
		}
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i]["code"].(string) < items[j]["code"].(string) })
	return items, nil
}

func (r *GORMRepository) assignmentPermissionItems(ctx context.Context, roleCode, query string, hydrate bool) ([]map[string]any, error) {
	var permissions []gormPermission
	if err := r.db.WithContext(ctx).Order("code").Find(&permissions).Error; err != nil {
		return nil, repositoryError(err)
	}
	var bindings []gormRolePermission
	if err := r.db.WithContext(ctx).Where("role_code = ?", roleCode).Find(&bindings).Error; err != nil {
		return nil, repositoryError(err)
	}
	selected := make(map[string]bool, len(bindings))
	for _, binding := range bindings {
		selected[binding.Permission] = true
	}
	needle := strings.ToLower(strings.TrimSpace(query))
	items := make([]map[string]any, 0)
	byCode := make(map[string]gormPermission, len(permissions))
	for _, permission := range permissions {
		byCode[permission.Code] = permission
	}
	for _, permission := range permissions {
		if !hydrate && needle != "" && !strings.Contains(strings.ToLower(permission.Code+" "+permission.Name+" "+permission.Resource), needle) {
			continue
		}
		if !hydrate && permission.Status != StatusEnabled {
			continue
		}
		if hydrate && !selected[permission.Code] {
			continue
		}
		items = append(items, map[string]any{"code": permission.Code, "name": firstNonEmpty(permission.Name, permission.Code), "parentCode": permission.Resource, "status": permission.Status, "selected": selected[permission.Code], "disabledReason": assignmentDisabledReason(permission.Status, selected[permission.Code]), "resourceType": permission.ResourceType})
	}
	for code := range selected {
		if _, exists := byCode[code]; !exists {
			items = append(items, map[string]any{"code": code, "name": code, "parentCode": "", "status": "disabled", "selected": true, "disabledReason": "historical selection unavailable", "resourceType": PermissionResourceTypeAPI})
		}
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i]["code"].(string) < items[j]["code"].(string) })
	return items, nil
}

func assignmentDisabledReason(status string, selected bool) string {
	if selected && status != StatusEnabled {
		return "historical selection is disabled"
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
