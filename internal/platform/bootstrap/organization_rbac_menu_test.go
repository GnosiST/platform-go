package bootstrap

import (
	"encoding/json"
	"reflect"
	"testing"

	"platform-go/internal/platform/adminresource"
	"platform-go/internal/platform/organizationrbac"
)

func TestAdminMenuItemsFromOrganizationNodesPreservesNavigationMetadata(t *testing.T) {
	nodes := []organizationrbac.MenuNode{
		{
			Code: "access", NodeType: organizationrbac.MenuNodeTypeDirectory, TitleZH: "访问控制", TitleEN: "Access",
			DescriptionZH: "目录", DescriptionEN: "Directory", Status: organizationrbac.StatusEnabled, Icon: "shield", SortOrder: 10,
		},
		{
			Code: "users", ParentCode: "access", NodeType: organizationrbac.MenuNodeTypePage, TitleZH: "用户", TitleEN: "Users",
			DescriptionZH: "用户管理", DescriptionEN: "User management", Status: organizationrbac.StatusEnabled, Icon: "users", SortOrder: 20,
			Route: "/users", ComponentKey: "users", ResourceCode: "users", CacheEnabled: true, Hidden: true,
			ActiveMenuCode: "profiles", BreadcrumbVisible: true,
			Parameters: []organizationrbac.MenuParameter{{Key: "tab", Type: organizationrbac.MenuParameterTypeString, Value: "active"}},
		},
	}

	items, err := adminMenuItemsFromOrganizationNodes(nodes)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].Name != "access" || items[0].NodeType != "directory" || items[0].Parameters != "[]" {
		t.Fatalf("directory item = %+v", items)
	}
	want := adminresource.MenuItem{
		Name: "users", NodeType: "page", Route: "/users", Parent: "access", ParentCode: "access",
		ComponentKey: "users", ResourceCode: "users", Parameters: `[{"key":"tab","type":"string","value":"active"}]`,
		CacheEnabled: true, Hidden: true, ActiveMenuCode: "profiles", BreadcrumbVisible: true, Resource: "users",
		Title:       adminresource.LocalizedText{ZH: "用户", EN: "Users"},
		Description: adminresource.LocalizedText{ZH: "用户管理", EN: "User management"}, Icon: "users", Order: 20,
	}
	if !reflect.DeepEqual(items[1], want) {
		t.Fatalf("page item = %+v, want %+v", items[1], want)
	}
	var parameters []map[string]any
	if err := json.Unmarshal([]byte(items[1].Parameters), &parameters); err != nil || len(parameters) != 1 {
		t.Fatalf("parameters = %q, error = %v", items[1].Parameters, err)
	}
}
