package organizationrbac

import (
	"reflect"
	"testing"

	"platform-go/internal/platform/kernel"
	"platform-go/internal/platform/serviceobject"
)

func TestAssignmentTreeQueryDefinitionsAreStableAndClosed(t *testing.T) {
	want := map[string]struct {
		permission string
		resource   string
		args       []serviceobject.ArgumentDefinition
		result     []serviceobject.ResultField
		maxPage    int
	}{
		MenuAssignmentTreeSearchQueryID:        {"admin:role:read", "menus", []serviceobject.ArgumentDefinition{{Name: "roleCode", Type: serviceobject.ValueString, Required: true, MaxLength: 191}, {Name: "query", Type: serviceobject.ValueString, MaxLength: 191}}, assignmentTreeResultSchema(true), 100},
		MenuAssignmentTreeHydrateQueryID:       {"admin:role:read", "menus", []serviceobject.ArgumentDefinition{{Name: "roleCode", Type: serviceobject.ValueString, Required: true, MaxLength: 191}}, assignmentTreeResultSchema(true), 1000},
		PermissionAssignmentTreeSearchQueryID:  {"admin:role:read", "permissions", []serviceobject.ArgumentDefinition{{Name: "roleCode", Type: serviceobject.ValueString, Required: true, MaxLength: 191}, {Name: "query", Type: serviceobject.ValueString, MaxLength: 191}}, assignmentTreeResultSchema(false), 100},
		PermissionAssignmentTreeHydrateQueryID: {"admin:role:read", "permissions", []serviceobject.ArgumentDefinition{{Name: "roleCode", Type: serviceobject.ValueString, Required: true, MaxLength: 191}}, assignmentTreeResultSchema(false), 1000},
	}
	definitions := OrganizationQueryDefinitions()
	for id, expected := range want {
		definition := queryDefinitionByID(t, id)
		if definition.ID != id || definition.Version != ServiceObjectVersion || definition.Permission != expected.permission || definition.Resource != expected.resource || definition.Action != "read" || definition.TenantMode != serviceobject.TenantPlatform || definition.DataScope != "platform" || definition.MaxPageSize != expected.maxPage || !reflect.DeepEqual(definition.Arguments, expected.args) || !reflect.DeepEqual(definition.ResultSchema, expected.result) {
			t.Fatalf("definition %s = %+v", id, definition)
		}
	}
	if len(definitions) < len(want) {
		t.Fatalf("query definitions = %d", len(definitions))
	}
}

func TestAssignmentTreeSearchAndHydrateQueries(t *testing.T) {
	db, repository := prepareOrganizationRBACTestRepository(t)
	seedOrganizationRBAC(t, db)
	seedNativeMenus(t, db)
	if err := db.Create(&gormRoleMenu{RoleCode: "operator", MenuCode: "users", Revision: 1, ActorID: "seed"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&gormPermission{ID: "perm-users", Code: "admin:user:read", Name: "Users", Status: StatusEnabled, ResourceType: PermissionResourceTypeAPI}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&gormRolePermission{RoleCode: "operator", Permission: "admin:user:read"}).Error; err != nil {
		t.Fatal(err)
	}
	runtime := newNavigationRuntime(t, repository, ServiceObjectExecutorOptions{}, nil)
	invocation := navigationInvocation()
	search, err := runtime.ExecuteQuery(invocation, serviceobject.QueryRequest{QueryID: MenuAssignmentTreeSearchQueryID, Version: ServiceObjectVersion, Arguments: map[string]any{"roleCode": "operator", "query": "User"}, Pagination: serviceobject.Pagination{Page: 1, PageSize: 10}})
	if err != nil {
		t.Fatalf("menu search error = %v", err)
	}
	if len(search.Items) != 2 || search.Items[0]["code"] != "access" || search.Items[1]["code"] != "users" {
		t.Fatalf("menu search = %+v", search.Items)
	}
	hydrate, err := runtime.ExecuteQuery(invocation, serviceobject.QueryRequest{QueryID: MenuAssignmentTreeHydrateQueryID, Version: ServiceObjectVersion, Arguments: map[string]any{"roleCode": "operator"}, Pagination: serviceobject.Pagination{Page: 1, PageSize: 10}})
	if err != nil || len(hydrate.Items) != 2 || hydrate.Items[0]["code"] != "access" || hydrate.Items[1]["selected"] != true {
		t.Fatalf("menu hydrate = %+v, error = %v", hydrate, err)
	}
	permissionSearch, err := runtime.ExecuteQuery(invocation, serviceobject.QueryRequest{QueryID: PermissionAssignmentTreeSearchQueryID, Version: ServiceObjectVersion, Arguments: map[string]any{"roleCode": "operator", "query": "user"}, Pagination: serviceobject.Pagination{Page: 1, PageSize: 10}})
	if err != nil || len(permissionSearch.Items) != 1 || permissionSearch.Items[0]["code"] != "admin:user:read" {
		t.Fatalf("permission search = %+v, error = %v", permissionSearch, err)
	}
	permissionHydrate, err := runtime.ExecuteQuery(invocation, serviceobject.QueryRequest{QueryID: PermissionAssignmentTreeHydrateQueryID, Version: ServiceObjectVersion, Arguments: map[string]any{"roleCode": "operator"}, Pagination: serviceobject.Pagination{Page: 1, PageSize: 10}})
	if err != nil || len(permissionHydrate.Items) != 1 || permissionHydrate.Items[0]["selected"] != true {
		t.Fatalf("permission hydrate = %+v, error = %v", permissionHydrate, err)
	}
}

func TestAssignmentTreeHydrateKeepsDisabledHistoricalSelections(t *testing.T) {
	db, repository := prepareOrganizationRBACTestRepository(t)
	seedOrganizationRBAC(t, db)
	if err := db.Create(&gormPermission{ID: "perm-old", Code: "admin:old:read", Name: "Old", Status: "disabled", ResourceType: PermissionResourceTypeAPI}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&gormRolePermission{RoleCode: "operator", Permission: "admin:old:read"}).Error; err != nil {
		t.Fatal(err)
	}
	runtime := newNavigationRuntime(t, repository, ServiceObjectExecutorOptions{}, nil)
	result, err := runtime.ExecuteQuery(navigationInvocation(), serviceobject.QueryRequest{QueryID: PermissionAssignmentTreeHydrateQueryID, Version: ServiceObjectVersion, Arguments: map[string]any{"roleCode": "operator"}, Pagination: serviceobject.Pagination{Page: 1, PageSize: 10}})
	if err != nil || len(result.Items) != 1 || result.Items[0]["code"] != "admin:old:read" || result.Items[0]["selected"] != true || result.Items[0]["disabledReason"] == "" {
		t.Fatalf("historical hydrate = %+v, error = %v", result, err)
	}
}

func TestAssignmentTreeRejectsClientSelectedCodesAndCrossTenantRole(t *testing.T) {
	db, repository := prepareOrganizationRBACTestRepository(t)
	seedOrganizationRBAC(t, db)
	if err := db.Create(&gormRoleGroup{ID: "group-other", Code: "other-ops", Name: "Other", ScopeType: string(ScopeTenant), TenantCode: "other", Status: StatusEnabled}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&gormRole{ID: "role-other", Code: "other-role", Name: "Other", GroupCode: "other-ops", Status: StatusEnabled}).Error; err != nil {
		t.Fatal(err)
	}
	runtime := newNavigationRuntime(t, repository, ServiceObjectExecutorOptions{}, nil)
	request := serviceobject.QueryRequest{QueryID: MenuAssignmentTreeSearchQueryID, Version: ServiceObjectVersion, Arguments: map[string]any{"roleCode": "other-role", "query": "", "selectedCodes": []string{"users"}}, Pagination: serviceobject.Pagination{Page: 1, PageSize: 10}}
	if _, err := runtime.ExecuteQuery(navigationInvocation(), request); err == nil {
		t.Fatal("selectedCodes unexpectedly accepted")
	}
	invocation := navigationInvocation()
	invocation.Execution.TenantScope = kernel.TenantScope{TenantCode: "acme"}
	invocation.TenantID = "acme"
	if _, err := runtime.ExecuteQuery(invocation, serviceobject.QueryRequest{QueryID: MenuAssignmentTreeSearchQueryID, Version: ServiceObjectVersion, Arguments: map[string]any{"roleCode": "other-role", "query": ""}, Pagination: serviceobject.Pagination{Page: 1, PageSize: 10}}); err == nil {
		t.Fatal("cross-tenant role unexpectedly visible")
	}
}
