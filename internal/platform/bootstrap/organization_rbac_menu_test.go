package bootstrap

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/GnosiST/platform-go/internal/platform/adminresource"
	"github.com/GnosiST/platform-go/internal/platform/organizationrbac"
)

func TestAdminMenuItemsFromOrganizationNodesPreservesNavigationMetadata(t *testing.T) {
	nodes := []organizationrbac.MenuNode{
		{
			Code: "access", NodeType: organizationrbac.MenuNodeTypeDirectory, TitleZH: "访问控制", TitleEN: "Access",
			DescriptionZH: "目录", DescriptionEN: "Directory", Status: organizationrbac.StatusEnabled, Icon: "shield", SortOrder: 10,
		},
		{
			Code: "users", ParentCode: "access", NodeType: organizationrbac.MenuNodeTypePage, TitleZH: "用户", TitleEN: "Users",
			DescriptionZH: "用户管理", DescriptionEN: "User management", Status: organizationrbac.StatusEnabled, Icon: "users", Group: "foundation", SortOrder: 20,
			Route: "/users", ComponentKey: "users", ResourceCode: "users", CacheEnabled: true, Hidden: true,
			ActiveMenuCode: "profiles", BreadcrumbVisible: true, LegacyPermission: "admin:user:read",
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
		Description: adminresource.LocalizedText{ZH: "用户管理", EN: "User management"},
		Permission:  "admin:user:read", Group: "foundation", Icon: "users", Order: 20,
	}
	if !reflect.DeepEqual(items[1], want) {
		t.Fatalf("page item = %+v, want %+v", items[1], want)
	}
	var parameters []map[string]any
	if err := json.Unmarshal([]byte(items[1].Parameters), &parameters); err != nil || len(parameters) != 1 {
		t.Fatalf("parameters = %q, error = %v", items[1].Parameters, err)
	}
}

func TestAdminMenuPageItemsFromOrganizationNodesSkipsDirectoriesForShell(t *testing.T) {
	items, err := adminMenuPageItemsFromOrganizationNodes([]organizationrbac.MenuNode{
		{
			Code: "access", NodeType: organizationrbac.MenuNodeTypeDirectory, TitleZH: "访问控制", TitleEN: "Access",
			Status: organizationrbac.StatusEnabled,
		},
		{
			Code: "users", ParentCode: "access", NodeType: organizationrbac.MenuNodeTypePage, TitleZH: "用户", TitleEN: "Users",
			Status: organizationrbac.StatusEnabled, Group: "foundation", Route: "/users", ComponentKey: "users", ResourceCode: "users", CacheEnabled: true,
			LegacyPermission: "admin:user:read",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Name != "users" || items[0].Route != "/users" {
		t.Fatalf("page shell items = %+v", items)
	}
	if items[0].Permission != "admin:user:read" || items[0].Group != "foundation" {
		t.Fatalf("page shell item authorization metadata = %+v", items[0])
	}
}
