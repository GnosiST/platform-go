package organizationrbac

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"platform-go/internal/platform/adminresource"

	"gorm.io/gorm"
)

func TestPrepareGORMRepositoryAddsNativeMenuSchema(t *testing.T) {
	db := openOrganizationRBACTestDB(t)
	type legacyMenu struct {
		ID         string `gorm:"column:id;primaryKey"`
		Code       string `gorm:"column:code;uniqueIndex"`
		Status     string `gorm:"column:status"`
		Route      string `gorm:"column:route"`
		Parent     string `gorm:"column:parent"`
		Permission string `gorm:"column:permission"`
	}
	if err := db.Table(menusTable).AutoMigrate(&legacyMenu{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Table(menusTable).Create(&legacyMenu{ID: "menu-users", Code: "users", Status: StatusEnabled, Route: "/users", Parent: "access", Permission: "admin:user:read"}).Error; err != nil {
		t.Fatal(err)
	}

	if _, err := PrepareGORMRepository(context.Background(), db); err != nil {
		t.Fatalf("PrepareGORMRepository() error = %v", err)
	}
	for _, table := range []string{menusTable, roleMenusTable, roleMenuRevisionsTable, pageButtonsTable} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("missing table %q", table)
		}
	}
	for _, column := range []string{"node_type", "parent_code", "component_key", "resource_code", "external_url", "parameters_json", "breadcrumb_visible"} {
		if !db.Migrator().HasColumn(&gormMenu{}, column) {
			t.Fatalf("menus missing column %q", column)
		}
	}
	var row gormMenu
	if err := db.Where("code = ?", "users").Take(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Route != "/users" || row.LegacyPermission != "admin:user:read" {
		t.Fatalf("legacy menu = %+v", row)
	}
	if _, err := OpenGORMRepository(context.Background(), db); err != nil {
		t.Fatalf("OpenGORMRepository() error = %v", err)
	}
}

func TestValidateMenuSnapshotRejectsInvalidTrees(t *testing.T) {
	valid := []MenuNode{
		{Code: "access", NodeType: MenuNodeTypeDirectory, TitleZH: "权限", TitleEN: "Access", Status: StatusEnabled},
		{Code: "users", ParentCode: "access", NodeType: MenuNodeTypePage, TitleZH: "用户", TitleEN: "Users", Status: StatusEnabled, Route: "/users", ComponentKey: "users", BreadcrumbVisible: true},
		{Code: "user-detail", ParentCode: "access", NodeType: MenuNodeTypePage, TitleZH: "用户详情", TitleEN: "User Detail", Status: StatusEnabled, Route: "/user-detail", ComponentKey: "user-detail", Hidden: true, ActiveMenuCode: "users", BreadcrumbVisible: true},
	}
	if err := ValidateMenuSnapshot(valid, nil, nil); err != nil {
		t.Fatalf("ValidateMenuSnapshot(valid) error = %v", err)
	}
	tests := []struct {
		name  string
		nodes []MenuNode
	}{
		{name: "directory navigates", nodes: []MenuNode{{Code: "access", NodeType: MenuNodeTypeDirectory, TitleZH: "权限", TitleEN: "Access", Status: StatusEnabled, Route: "/access"}}},
		{name: "page parent", nodes: []MenuNode{{Code: "parent", NodeType: MenuNodeTypePage, TitleZH: "P", TitleEN: "P", Status: StatusEnabled, Route: "/parent", ComponentKey: "parent"}, {Code: "child", ParentCode: "parent", NodeType: MenuNodeTypePage, TitleZH: "C", TitleEN: "C", Status: StatusEnabled, Route: "/child", ComponentKey: "child"}}},
		{name: "cycle", nodes: []MenuNode{{Code: "one", ParentCode: "two", NodeType: MenuNodeTypeDirectory, TitleZH: "一", TitleEN: "One", Status: StatusEnabled}, {Code: "two", ParentCode: "one", NodeType: MenuNodeTypeDirectory, TitleZH: "二", TitleEN: "Two", Status: StatusEnabled}}},
		{name: "external http", nodes: []MenuNode{{Code: "docs", NodeType: MenuNodeTypePage, TitleZH: "文档", TitleEN: "Docs", Status: StatusEnabled, External: true, ExternalURL: "http://example.com", OpenMode: MenuOpenModeNewTab}}},
		{name: "active menu missing", nodes: []MenuNode{{Code: "detail", NodeType: MenuNodeTypePage, TitleZH: "详情", TitleEN: "Detail", Status: StatusEnabled, Route: "/detail", ComponentKey: "detail", ActiveMenuCode: "missing"}}},
		{name: "active menu directory", nodes: []MenuNode{{Code: "access", NodeType: MenuNodeTypeDirectory, TitleZH: "权限", TitleEN: "Access", Status: StatusEnabled}, {Code: "detail", ParentCode: "access", NodeType: MenuNodeTypePage, TitleZH: "详情", TitleEN: "Detail", Status: StatusEnabled, Route: "/detail", ComponentKey: "detail", ActiveMenuCode: "access"}}},
		{name: "active menu disabled", nodes: []MenuNode{{Code: "users", NodeType: MenuNodeTypePage, TitleZH: "用户", TitleEN: "Users", Status: "disabled", Route: "/users", ComponentKey: "users"}, {Code: "detail", NodeType: MenuNodeTypePage, TitleZH: "详情", TitleEN: "Detail", Status: StatusEnabled, Route: "/detail", ComponentKey: "detail", ActiveMenuCode: "users"}}},
		{name: "active menu self", nodes: []MenuNode{{Code: "detail", NodeType: MenuNodeTypePage, TitleZH: "详情", TitleEN: "Detail", Status: StatusEnabled, Route: "/detail", ComponentKey: "detail", ActiveMenuCode: "detail"}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := ValidateMenuSnapshot(test.nodes, nil, nil); !errors.Is(err, ErrInvalid) {
				t.Fatalf("ValidateMenuSnapshot() error = %v, want ErrInvalid", err)
			}
		})
	}
}

func TestValidateMenuSnapshotRequiresMatchingPageButtonPermission(t *testing.T) {
	nodes := []MenuNode{{Code: "users", NodeType: MenuNodeTypePage, TitleZH: "用户", TitleEN: "Users", Status: StatusEnabled, Route: "/users", ComponentKey: "users"}}
	buttons := []PageButton{{MenuCode: "users", ButtonKey: "export", LabelZH: "导出", LabelEN: "Export", Action: "export", SortOrder: 10, Status: StatusEnabled, PermissionCode: "page:user:export"}}
	permissions := []MenuPermission{{Code: "page:user:export", Status: StatusEnabled, ResourceType: PermissionResourceTypePageButton, MenuCode: "users", ButtonKey: "export", Action: "export"}}
	if err := ValidateMenuSnapshot(nodes, buttons, permissions); err != nil {
		t.Fatalf("ValidateMenuSnapshot(valid button) error = %v", err)
	}
	permissions[0].ButtonKey = "download"
	if err := ValidateMenuSnapshot(nodes, buttons, permissions); !errors.Is(err, ErrInvalid) {
		t.Fatalf("ValidateMenuSnapshot(mismatch) error = %v, want ErrInvalid", err)
	}
}

func TestValidateMenuSnapshotRejectsExecutableRoutesAndPhysicalParameters(t *testing.T) {
	base := MenuNode{Code: "users", NodeType: MenuNodeTypePage, TitleZH: "用户", TitleEN: "Users", Status: StatusEnabled, Route: "/users", ComponentKey: "users"}
	for _, route := range []string{"/users/:id", "/users/{id}", "/users/*"} {
		node := base
		node.Route = route
		if err := ValidateMenuSnapshot([]MenuNode{node}, nil, nil); !errors.Is(err, ErrInvalid) {
			t.Fatalf("route %q error = %v, want ErrInvalid", route, err)
		}
	}
	for _, key := range []string{"datasource", "shard", "database", "schema", "sql", "script", "expression", "route-template", "physical-routing"} {
		node := base
		node.Parameters = []MenuParameter{{Key: key, Type: MenuParameterTypeString, Value: "unsafe"}}
		if err := ValidateMenuSnapshot([]MenuNode{node}, nil, nil); !errors.Is(err, ErrInvalid) {
			t.Fatalf("parameter %q error = %v, want ErrInvalid", key, err)
		}
	}
}

func TestValidateMenuSnapshotRejectsExecutableAndPhysicalStringParameterValues(t *testing.T) {
	base := MenuNode{Code: "users", NodeType: MenuNodeTypePage, TitleZH: "用户", TitleEN: "Users", Status: StatusEnabled, Route: "/users", ComponentKey: "users"}
	unsafeValues := map[string]string{
		"script tag":          `<script>alert("x")</script>`,
		"script URI":          `javascript:alert("x")`,
		"script keyword":      `script`,
		"expression":          `${tenant.id}`,
		"expression keyword":  `expression`,
		"template expression": `{{ currentUser.id }}`,
		"SQL":                 `SELECT * FROM platform_admin_users`,
		"SQL keyword":         `sql`,
		"route parameter":     `/users/:id`,
		"route expression":    `/users/{id}`,
		"route wildcard":      `/users/*`,
		"datasource routing":  `datasource=primary`,
		"datasource keyword":  `datasource`,
		"shard routing":       `shard:tenant-42`,
		"shard keyword":       `shard`,
		"database routing":    `database=platform`,
		"database keyword":    `database`,
		"schema routing":      `{"schema":"public"}`,
		"schema keyword":      `schema`,
	}
	for name, value := range unsafeValues {
		t.Run(name, func(t *testing.T) {
			node := base
			node.Parameters = []MenuParameter{{Key: "tab", Type: MenuParameterTypeString, Value: value}}
			if err := ValidateMenuSnapshot([]MenuNode{node}, nil, nil); !errors.Is(err, ErrInvalid) {
				t.Fatalf("ValidateMenuSnapshot(%q) error = %v, want ErrInvalid", value, err)
			}
		})
	}

	for name, value := range map[string]string{
		"ordinary value": "active",
		"word substring": "selection",
		"camel-case key": "schemaVersion",
		"static path":    "/users/profile",
	} {
		t.Run("allows "+name, func(t *testing.T) {
			node := base
			node.Parameters = []MenuParameter{{Key: "tab", Type: MenuParameterTypeString, Value: value}}
			if err := ValidateMenuSnapshot([]MenuNode{node}, nil, nil); err != nil {
				t.Fatalf("ValidateMenuSnapshot(%q) error = %v", value, err)
			}
		})
	}
}

func TestReplaceMenuDefinitionAtomicallyOwnsPageButtonPermissions(t *testing.T) {
	db, repository := prepareOrganizationRBACTestRepository(t)
	seedOrganizationRBAC(t, db)
	if err := db.Create(&gormPermission{ID: "permission-user-read", Code: "admin:user:read", Name: "Read users", Status: StatusEnabled, ResourceType: PermissionResourceTypeAPI, Resource: "users", Action: "read", ValuesJSON: "{}"}).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 15, 19, 0, 0, 0, time.UTC)
	definition := MenuDefinition{ID: "menu-users", Name: "Users", Node: MenuNode{Code: "users", NodeType: MenuNodeTypePage, TitleZH: "用户", TitleEN: "Users", Status: StatusEnabled, Route: "/users", ComponentKey: "users", BreadcrumbVisible: true}, Buttons: []PageButton{{MenuCode: "users", ButtonKey: "export", LabelZH: "导出", LabelEN: "Export", Action: "export", Status: StatusEnabled, PermissionCode: "page:user:export"}}}
	revision, err := repository.ReplaceMenuDefinition(context.Background(), ReplaceMenuDefinitionRequest{Definition: definition, ExpectedRevision: 0, ActorID: "admin", ChangedAt: now})
	if err != nil || revision != 1 {
		t.Fatalf("ReplaceMenuDefinition(create) = %d, %v", revision, err)
	}
	invalidActiveMenu := definition
	invalidActiveMenu.Node.ActiveMenuCode = "missing"
	if _, err := repository.ReplaceMenuDefinition(context.Background(), ReplaceMenuDefinitionRequest{Definition: invalidActiveMenu, ExpectedRevision: 1, ActorID: "admin", ChangedAt: now.Add(5 * time.Second)}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("ReplaceMenuDefinition(missing active menu) error = %v, want ErrInvalid", err)
	}
	deletedTarget := gormMenu{ID: "menu-dashboard", Code: "dashboard", Name: "Dashboard", Status: StatusEnabled, NodeType: string(MenuNodeTypePage), Route: "/dashboard", ComponentKey: "dashboard", TitleZH: "看板", TitleEN: "Dashboard", ParametersJSON: "[]", ValuesJSON: "{}"}
	if err := db.Create(&deletedTarget).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&gormResourceLifecycle{Resource: "menus", RecordID: deletedTarget.ID, DeletedAt: now.Format(time.RFC3339)}).Error; err != nil {
		t.Fatal(err)
	}
	invalidActiveMenu.Node.ActiveMenuCode = deletedTarget.Code
	if _, err := repository.ReplaceMenuDefinition(context.Background(), ReplaceMenuDefinitionRequest{Definition: invalidActiveMenu, ExpectedRevision: 1, ActorID: "admin", ChangedAt: now.Add(6 * time.Second)}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("ReplaceMenuDefinition(deleted active menu) error = %v, want ErrInvalid", err)
	}
	assertPageButtonPermission(t, db, "page:user:export", "users", "export", "export")
	conflictingID := definition
	conflictingID.ID = "menu-users-other"
	if _, err := repository.ReplaceMenuDefinition(context.Background(), ReplaceMenuDefinitionRequest{Definition: conflictingID, ExpectedRevision: 1, ActorID: "admin", ChangedAt: now.Add(10 * time.Second)}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("ReplaceMenuDefinition(conflicting id) error = %v, want ErrInvalid", err)
	}
	conflictingCode := definition
	conflictingCode.Buttons = append([]PageButton(nil), definition.Buttons...)
	conflictingCode.Node.Code = "users-other"
	conflictingCode.Buttons[0].MenuCode = "users-other"
	if _, err := repository.ReplaceMenuDefinition(context.Background(), ReplaceMenuDefinitionRequest{Definition: conflictingCode, ExpectedRevision: 1, ActorID: "admin", ChangedAt: now.Add(20 * time.Second)}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("ReplaceMenuDefinition(conflicting code) error = %v, want ErrInvalid", err)
	}
	loaded, loadedRevision, err := repository.LoadMenuDefinition(context.Background(), "users")
	if err != nil || loadedRevision != 1 || loaded.ID != definition.ID || !reflect.DeepEqual(loaded.Node, definition.Node) || !reflect.DeepEqual(loaded.Buttons, definition.Buttons) {
		t.Fatalf("LoadMenuDefinition() = %+v, revision = %d, error = %v", loaded, loadedRevision, err)
	}
	noOpRevision, err := repository.ReplaceMenuDefinition(context.Background(), ReplaceMenuDefinitionRequest{Definition: definition, ExpectedRevision: 1, ActorID: "admin", ChangedAt: now.Add(30 * time.Second)})
	if err != nil || noOpRevision != 1 {
		t.Fatalf("ReplaceMenuDefinition(noop) = %d, %v", noOpRevision, err)
	}

	definition.Buttons[0].Action = "download"
	definition.Buttons[0].LabelEN = "Download"
	revision, err = repository.ReplaceMenuDefinition(context.Background(), ReplaceMenuDefinitionRequest{Definition: definition, ExpectedRevision: 1, ActorID: "admin", ChangedAt: now.Add(time.Minute)})
	if err != nil || revision != 2 {
		t.Fatalf("ReplaceMenuDefinition(update) = %d, %v", revision, err)
	}
	assertPageButtonPermission(t, db, "page:user:export", "users", "export", "download")

	definition.Buttons = nil
	revision, err = repository.ReplaceMenuDefinition(context.Background(), ReplaceMenuDefinitionRequest{Definition: definition, ExpectedRevision: 2, ActorID: "admin", ChangedAt: now.Add(2 * time.Minute)})
	if err != nil || revision != 3 {
		t.Fatalf("ReplaceMenuDefinition(delete button) = %d, %v", revision, err)
	}
	var pagePermissionCount, apiPermissionCount int64
	if err := db.Model(&gormPermission{}).Where("code = ?", "page:user:export").Count(&pagePermissionCount).Error; err != nil || pagePermissionCount != 0 {
		t.Fatalf("page permission count = %d, error = %v", pagePermissionCount, err)
	}
	if err := db.Model(&gormPermission{}).Where("code = ?", "admin:user:read").Count(&apiPermissionCount).Error; err != nil || apiPermissionCount != 1 {
		t.Fatalf("api permission count = %d, error = %v", apiPermissionCount, err)
	}
}

func TestReplaceMenuDefinitionRollsBackMenuButtonPermissionAndRevision(t *testing.T) {
	db, repository := prepareOrganizationRBACTestRepository(t)
	seedOrganizationRBAC(t, db)
	trigger := `CREATE TRIGGER fail_page_permission BEFORE INSERT ON platform_admin_permissions WHEN NEW.resource_type = 'page-button' BEGIN SELECT RAISE(FAIL, 'blocked'); END`
	if err := db.Exec(trigger).Error; err != nil {
		t.Fatal(err)
	}
	definition := MenuDefinition{ID: "menu-users", Name: "Users", Node: MenuNode{Code: "users", NodeType: MenuNodeTypePage, TitleZH: "用户", TitleEN: "Users", Status: StatusEnabled, Route: "/users", ComponentKey: "users"}, Buttons: []PageButton{{MenuCode: "users", ButtonKey: "export", LabelZH: "导出", LabelEN: "Export", Action: "export", Status: StatusEnabled, PermissionCode: "page:user:export"}}}
	if _, err := repository.ReplaceMenuDefinition(context.Background(), ReplaceMenuDefinitionRequest{Definition: definition, ExpectedRevision: 0, ActorID: "admin", ChangedAt: time.Now().UTC()}); !errors.Is(err, ErrRepositoryFailed) {
		t.Fatalf("ReplaceMenuDefinition() error = %v, want ErrRepositoryFailed", err)
	}
	for _, model := range []any{&gormMenu{}, &gormPageButton{}, &gormPermission{}} {
		var count int64
		if err := db.Model(model).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("%T count = %d, error = %v", model, count, err)
		}
	}
	if revision, err := repository.CurrentGlobalRevision(context.Background()); err != nil || revision != 0 {
		t.Fatalf("global revision = %d, error = %v", revision, err)
	}
}

func TestAdminMenuPermissionSnapshotWriterFreezesLegacyPermission(t *testing.T) {
	db, _ := prepareOrganizationRBACTestRepository(t)
	writer := NewAdminMenuPermissionSnapshotWriter()
	current := menuRecord("menu-users", "users", "page", "", "/users", "users")
	current.Values["permission"] = "admin:user:read"
	proposed := current
	proposed.Values = map[string]string{}
	for key, value := range current.Values {
		proposed.Values[key] = value
	}
	proposed.Values["permission"] = "admin:user:update"
	if err := writer.ApplyMenuPermissionSnapshot(context.Background(), db, []adminresource.Record{current}, []adminresource.Record{proposed}, nil, nil); !errors.Is(err, adminresource.ErrDomainOwnedMutation) {
		t.Fatalf("ApplyMenuPermissionSnapshot(legacy permission) error = %v, want ErrDomainOwnedMutation", err)
	}
}

func TestAdminMenuPermissionSnapshotWriterRejectsLegacyParentDrift(t *testing.T) {
	db, _ := prepareOrganizationRBACTestRepository(t)
	writer := NewAdminMenuPermissionSnapshotWriter()
	current := menuRecord("menu-users", "users", "page", "access", "/users", "users")
	current.Values["parent"] = "access"
	proposed := current
	proposed.Values = map[string]string{}
	for key, value := range current.Values {
		proposed.Values[key] = value
	}
	proposed.Values["parent"] = "identity"
	if err := writer.ApplyMenuPermissionSnapshot(context.Background(), db, []adminresource.Record{current}, []adminresource.Record{proposed}, nil, nil); !errors.Is(err, adminresource.ErrDomainOwnedMutation) {
		t.Fatalf("ApplyMenuPermissionSnapshot(parent drift) error = %v, want ErrDomainOwnedMutation", err)
	}
}

func TestMenuAndPermissionLifecycleIncludesNativeReferences(t *testing.T) {
	db, repository := prepareOrganizationRBACTestRepository(t)
	seedOrganizationRBAC(t, db)
	seedNativeMenus(t, db)
	now := time.Date(2026, 7, 15, 20, 0, 0, 0, time.UTC)
	if err := db.Create(&gormRoleMenu{RoleCode: "operator", MenuCode: "users", Revision: 1, ActorID: "admin", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&gormPermission{ID: "permission-user-export", Code: "page:user:export", Name: "Export", Status: StatusEnabled, ResourceType: PermissionResourceTypePageButton, Resource: "users", Action: "export", ValuesJSON: `{"menuCode":"users","buttonKey":"export","action":"export"}`}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&gormPageButton{MenuCode: "users", ButtonKey: "export", LabelZH: "导出", LabelEN: "Export", Action: "export", Status: StatusEnabled, PermissionCode: "page:user:export"}).Error; err != nil {
		t.Fatal(err)
	}

	menuImpact, err := repository.PreviewResourceLifecycle(context.Background(), "menus", "users", LifecycleOperationDelete, now)
	if err != nil || menuImpact.ReferenceCount != 2 || !hasLifecycleReference(menuImpact.References, "role-menu", "operator") || !hasLifecycleReference(menuImpact.References, "page-button", "export") {
		t.Fatalf("menu impact = %+v, error = %v", menuImpact, err)
	}
	permissionImpact, err := repository.PreviewResourceLifecycle(context.Background(), "permissions", "page:user:export", LifecycleOperationDelete, now)
	if err != nil || permissionImpact.ReferenceCount != 1 || !hasLifecycleReference(permissionImpact.References, "page-button", "users:export") {
		t.Fatalf("permission impact = %+v, error = %v", permissionImpact, err)
	}
	roleImpact, err := repository.PreviewResourceLifecycle(context.Background(), "roles", "operator", LifecycleOperationDelete, now)
	if err != nil || !hasLifecycleReference(roleImpact.References, "role-menu", "users") {
		t.Fatalf("role impact = %+v, error = %v", roleImpact, err)
	}
	if _, err := repository.ApplyResourceLifecycle(context.Background(), ResourceLifecycleRequest{Resource: "menus", ResourceCode: "users", Operation: LifecycleOperationDelete, ExpectedRevision: menuImpact.ExpectedRevision, ActorID: "admin", ChangedAt: now, RetentionDays: 30, PolicyVersion: 1}, nil); !errors.Is(err, ErrRolePoolViolation) {
		t.Fatalf("ApplyResourceLifecycle(menu with references) error = %v, want ErrRolePoolViolation", err)
	}
}

func TestDeepMenuRestoreLoadsAncestorClosureAndPermissionPurgesWithoutReferences(t *testing.T) {
	db, repository := prepareOrganizationRBACTestRepository(t)
	seedOrganizationRBAC(t, db)
	rows := []gormMenu{
		{ID: "menu-root", Code: "root", Name: "Root", Status: StatusEnabled, NodeType: string(MenuNodeTypeDirectory), TitleZH: "根", TitleEN: "Root", ParametersJSON: "[]", ValuesJSON: "{}"},
		{ID: "menu-nested", Code: "nested", Name: "Nested", Status: StatusEnabled, NodeType: string(MenuNodeTypeDirectory), ParentCode: "root", TitleZH: "嵌套", TitleEN: "Nested", ParametersJSON: "[]", ValuesJSON: "{}"},
		{ID: "menu-page", Code: "page", Name: "Page", Status: StatusEnabled, NodeType: string(MenuNodeTypePage), ParentCode: "nested", Route: "/page", ComponentKey: "page", TitleZH: "页面", TitleEN: "Page", ParametersJSON: "[]", ValuesJSON: "{}"},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 15, 21, 0, 0, 0, time.UTC)
	deleteImpact, err := repository.PreviewResourceLifecycle(context.Background(), "menus", "page", LifecycleOperationDelete, now)
	if err != nil {
		t.Fatal(err)
	}
	revision, err := repository.ApplyResourceLifecycle(context.Background(), ResourceLifecycleRequest{Resource: "menus", ResourceCode: "page", Operation: LifecycleOperationDelete, ExpectedRevision: deleteImpact.ExpectedRevision, ActorID: "admin", ChangedAt: now, RetentionDays: 1, PolicyVersion: 1}, nil)
	if err != nil {
		t.Fatalf("delete deep page error = %v", err)
	}
	restoreImpact, err := repository.PreviewResourceLifecycle(context.Background(), "menus", "page", LifecycleOperationRestore, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if revision, err = repository.ApplyResourceLifecycle(context.Background(), ResourceLifecycleRequest{Resource: "menus", ResourceCode: "page", Operation: LifecycleOperationRestore, ExpectedRevision: restoreImpact.ExpectedRevision, ActorID: "admin", ChangedAt: now.Add(time.Hour)}, nil); err != nil {
		t.Fatalf("restore deep page error = %v", err)
	}
	secondMenuDelete, err := repository.PreviewResourceLifecycle(context.Background(), "menus", "page", LifecycleOperationDelete, now.Add(2*time.Hour))
	if err != nil || secondMenuDelete.ExpectedRevision != revision {
		t.Fatalf("second menu delete impact = %+v, error = %v", secondMenuDelete, err)
	}
	if revision, err = repository.ApplyResourceLifecycle(context.Background(), ResourceLifecycleRequest{Resource: "menus", ResourceCode: "page", Operation: LifecycleOperationDelete, ExpectedRevision: secondMenuDelete.ExpectedRevision, ActorID: "admin", ChangedAt: now.Add(2 * time.Hour), RetentionDays: 1, PolicyVersion: 1}, nil); err != nil {
		t.Fatalf("second delete deep page error = %v", err)
	}
	menuPurge, err := repository.PreviewResourceLifecycle(context.Background(), "menus", "page", LifecycleOperationPurge, now.Add(28*time.Hour))
	if err != nil || menuPurge.ExpectedRevision != revision || !menuPurge.RetentionElapsed {
		t.Fatalf("menu purge impact = %+v, error = %v", menuPurge, err)
	}
	if revision, err = repository.ApplyResourceLifecycle(context.Background(), ResourceLifecycleRequest{Resource: "menus", ResourceCode: "page", Operation: LifecycleOperationPurge, ExpectedRevision: menuPurge.ExpectedRevision, ActorID: "admin", ChangedAt: now.Add(28 * time.Hour)}, nil); err != nil {
		t.Fatalf("purge deep page error = %v", err)
	}

	permission := gormPermission{ID: "permission-orphan", Code: "admin:orphan:read", Name: "Orphan", Status: StatusEnabled, ResourceType: PermissionResourceTypeAPI, Resource: "orphan", Action: "read", ValuesJSON: "{}"}
	if err := db.Create(&permission).Error; err != nil {
		t.Fatal(err)
	}
	permissionDelete, err := repository.PreviewResourceLifecycle(context.Background(), "permissions", permission.Code, LifecycleOperationDelete, now.Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	revision, err = repository.ApplyResourceLifecycle(context.Background(), ResourceLifecycleRequest{Resource: "permissions", ResourceCode: permission.Code, Operation: LifecycleOperationDelete, ExpectedRevision: permissionDelete.ExpectedRevision, ActorID: "admin", ChangedAt: now.Add(2 * time.Hour), RetentionDays: 1, PolicyVersion: 1}, nil)
	if err != nil {
		t.Fatalf("delete permission error = %v", err)
	}
	permissionRestore, err := repository.PreviewResourceLifecycle(context.Background(), "permissions", permission.Code, LifecycleOperationRestore, now.Add(3*time.Hour))
	if err != nil || permissionRestore.ExpectedRevision != revision {
		t.Fatalf("permission restore impact = %+v, error = %v", permissionRestore, err)
	}
	if revision, err = repository.ApplyResourceLifecycle(context.Background(), ResourceLifecycleRequest{Resource: "permissions", ResourceCode: permission.Code, Operation: LifecycleOperationRestore, ExpectedRevision: permissionRestore.ExpectedRevision, ActorID: "admin", ChangedAt: now.Add(3 * time.Hour)}, nil); err != nil {
		t.Fatalf("restore permission error = %v", err)
	}
	permissionDelete, err = repository.PreviewResourceLifecycle(context.Background(), "permissions", permission.Code, LifecycleOperationDelete, now.Add(4*time.Hour))
	if err != nil || permissionDelete.ExpectedRevision != revision {
		t.Fatalf("second permission delete impact = %+v, error = %v", permissionDelete, err)
	}
	if revision, err = repository.ApplyResourceLifecycle(context.Background(), ResourceLifecycleRequest{Resource: "permissions", ResourceCode: permission.Code, Operation: LifecycleOperationDelete, ExpectedRevision: permissionDelete.ExpectedRevision, ActorID: "admin", ChangedAt: now.Add(4 * time.Hour), RetentionDays: 1, PolicyVersion: 1}, nil); err != nil {
		t.Fatalf("second delete permission error = %v", err)
	}
	permissionPurge, err := repository.PreviewResourceLifecycle(context.Background(), "permissions", permission.Code, LifecycleOperationPurge, now.Add(30*time.Hour))
	if err != nil || permissionPurge.ExpectedRevision != revision || !permissionPurge.RetentionElapsed {
		t.Fatalf("permission purge impact = %+v, error = %v", permissionPurge, err)
	}
	if _, err := repository.ApplyResourceLifecycle(context.Background(), ResourceLifecycleRequest{Resource: "permissions", ResourceCode: permission.Code, Operation: LifecycleOperationPurge, ExpectedRevision: permissionPurge.ExpectedRevision, ActorID: "admin", ChangedAt: now.Add(30 * time.Hour)}, nil); err != nil {
		t.Fatalf("purge permission error = %v", err)
	}
	var count int64
	if err := db.Model(&gormPermission{}).Where("code = ?", permission.Code).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("purged permission count = %d, error = %v", count, err)
	}
}

func TestGORMRepositoryReplaceRoleMenusUsesPerRoleRevisionAndNoOp(t *testing.T) {
	db, repository := prepareOrganizationRBACTestRepository(t)
	seedOrganizationRBAC(t, db)
	seedNativeMenus(t, db)
	now := time.Date(2026, 7, 15, 18, 0, 0, 0, time.UTC)

	first, err := repository.ReplaceRoleMenus(context.Background(), ReplaceRoleMenusRequest{RoleCode: "operator", MenuCodes: []string{"users", "users"}, ExpectedRevision: 0, ActorID: "admin", ChangedAt: now})
	if err != nil {
		t.Fatalf("ReplaceRoleMenus(first) error = %v", err)
	}
	if first.Revision != 1 || !reflect.DeepEqual(first.MenuCodes, []string{"users"}) {
		t.Fatalf("first = %+v", first)
	}
	preview, err := repository.PreviewRoleMenus(context.Background(), "operator", []string{"users", "users"})
	if err != nil || preview.ExpectedRevision != 1 || preview.Changed || !reflect.DeepEqual(preview.CurrentMenuCodes, []string{"users"}) || !reflect.DeepEqual(preview.ProposedMenuCodes, []string{"users"}) {
		t.Fatalf("PreviewRoleMenus(noop) = %+v, %v", preview, err)
	}
	changedPreview, err := repository.PreviewRoleMenus(context.Background(), "operator", []string{"reports", "users"})
	if err != nil || changedPreview.ExpectedRevision != 1 || !changedPreview.Changed || !reflect.DeepEqual(changedPreview.ProposedMenuCodes, []string{"reports", "users"}) {
		t.Fatalf("PreviewRoleMenus(changed) = %+v, %v", changedPreview, err)
	}
	globalAfterFirst, err := repository.CurrentGlobalRevision(context.Background())
	if err != nil || globalAfterFirst != 1 {
		t.Fatalf("global revision = %d, error = %v", globalAfterFirst, err)
	}

	other, err := repository.ReplaceRoleMenus(context.Background(), ReplaceRoleMenusRequest{RoleCode: "auditor", MenuCodes: []string{"reports"}, ExpectedRevision: 0, ActorID: "admin", ChangedAt: now.Add(time.Minute)})
	if err != nil || other.Revision != 1 {
		t.Fatalf("other = %+v, error = %v", other, err)
	}
	loaded, err := repository.LoadRoleMenus(context.Background(), "operator")
	if err != nil || loaded.Revision != 1 {
		t.Fatalf("operator after other role change = %+v, %v", loaded, err)
	}

	noop, err := repository.ReplaceRoleMenus(context.Background(), ReplaceRoleMenusRequest{RoleCode: "operator", MenuCodes: []string{"users"}, ExpectedRevision: 1, ActorID: "admin", ChangedAt: now.Add(2 * time.Minute)})
	if err != nil || noop.Revision != 1 {
		t.Fatalf("noop = %+v, error = %v", noop, err)
	}
	globalAfterNoop, err := repository.CurrentGlobalRevision(context.Background())
	if err != nil || globalAfterNoop != 2 {
		t.Fatalf("global revision after noop = %d, error = %v", globalAfterNoop, err)
	}

	_, err = repository.ReplaceRoleMenus(context.Background(), ReplaceRoleMenusRequest{RoleCode: "operator", MenuCodes: []string{"reports"}, ExpectedRevision: 0, ActorID: "admin", ChangedAt: now.Add(3 * time.Minute)})
	if !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("stale replace error = %v, want ErrRevisionConflict", err)
	}
	_, err = repository.ReplaceRoleMenus(context.Background(), ReplaceRoleMenusRequest{RoleCode: "operator", MenuCodes: []string{"access"}, ExpectedRevision: 1, ActorID: "admin", ChangedAt: now.Add(4 * time.Minute)})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("directory replace error = %v, want ErrInvalid", err)
	}
}

func TestGORMRepositoryRoleMenusRejectsSelectionsAboveContractLimit(t *testing.T) {
	_, repository := prepareOrganizationRBACTestRepository(t)
	menuCodes := make([]string, MaximumRoleMenuSelections+1)
	for index := range menuCodes {
		menuCodes[index] = fmt.Sprintf("menu-%04d", index)
	}

	if _, err := repository.PreviewRoleMenus(context.Background(), "operator", menuCodes); !errors.Is(err, ErrInvalid) {
		t.Fatalf("PreviewRoleMenus(over limit) error = %v, want ErrInvalid", err)
	}
	if _, err := repository.ReplaceRoleMenus(context.Background(), ReplaceRoleMenusRequest{
		RoleCode: "operator", MenuCodes: menuCodes, ActorID: "admin", ChangedAt: time.Now().UTC(),
	}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("ReplaceRoleMenus(over limit) error = %v, want ErrInvalid", err)
	}
}

func TestAdminMenuPermissionSnapshotWriterPreservesNativeRelations(t *testing.T) {
	db, _ := prepareOrganizationRBACTestRepository(t)
	seedOrganizationRBAC(t, db)
	seedNativeMenus(t, db)
	if err := db.Create(&gormRoleMenuRevision{RoleCode: "operator", Revision: 1, UpdatedAt: time.Now().UTC()}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&gormRoleMenu{RoleCode: "operator", MenuCode: "users", Revision: 1, ActorID: "admin", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}).Error; err != nil {
		t.Fatal(err)
	}
	writer := NewAdminMenuPermissionSnapshotWriter()
	currentMenus := []adminresource.Record{menuRecord("menu-users", "users", "page", "", "/users", "users")}
	proposedMenus := append([]adminresource.Record{}, currentMenus...)
	proposedMenus[0].Name = "User management"
	currentPermissions := []adminresource.Record{{ID: "permission-user-read", Code: "admin:user:read", Name: "Read users", Status: StatusEnabled, Values: map[string]string{"resourceType": PermissionResourceTypeAPI, "capability": "core", "resource": "users", "action": "read"}}}
	proposedPermissions := append([]adminresource.Record{}, currentPermissions...)
	proposedPermissions[0].Description = "Updated"
	if err := writer.ApplyMenuPermissionSnapshot(context.Background(), db, currentMenus, proposedMenus, currentPermissions, proposedPermissions); err != nil {
		t.Fatalf("ApplyMenuPermissionSnapshot() error = %v", err)
	}
	var bindingCount int64
	if err := db.Model(&gormRoleMenu{}).Where("role_code = ? AND menu_code = ?", "operator", "users").Count(&bindingCount).Error; err != nil || bindingCount != 1 {
		t.Fatalf("role menu count = %d, error = %v", bindingCount, err)
	}
}

func seedNativeMenus(t *testing.T, db *gorm.DB) {
	t.Helper()
	rows := []any{
		&gormMenu{ID: "menu-access", Code: "access", Name: "Access", Status: StatusEnabled, NodeType: string(MenuNodeTypeDirectory), TitleZH: "权限", TitleEN: "Access", BreadcrumbVisible: true, ValuesJSON: "{}"},
		&gormMenu{ID: "menu-users", Code: "users", Name: "Users", Status: StatusEnabled, NodeType: string(MenuNodeTypePage), ParentCode: "access", Parent: "access", Route: "/users", ComponentKey: "users", TitleZH: "用户", TitleEN: "Users", BreadcrumbVisible: true, ValuesJSON: "{}"},
		&gormMenu{ID: "menu-reports", Code: "reports", Name: "Reports", Status: StatusEnabled, NodeType: string(MenuNodeTypePage), ParentCode: "access", Parent: "access", Route: "/reports", ComponentKey: "reports", TitleZH: "报表", TitleEN: "Reports", BreadcrumbVisible: true, ValuesJSON: "{}"},
	}
	for _, row := range rows {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func menuRecord(id, code, nodeType, parentCode, route, componentKey string) adminresource.Record {
	return adminresource.Record{ID: id, Code: code, Name: code, Status: StatusEnabled, Values: map[string]string{
		"nodeType": nodeType, "parentCode": parentCode, "parent": parentCode, "route": route, "componentKey": componentKey,
		"titleZh": code, "titleEn": code, "breadcrumbVisible": "true",
	}}
}

func assertPageButtonPermission(t *testing.T, db *gorm.DB, code, menuCode, buttonKey, action string) {
	t.Helper()
	var permission gormPermission
	if err := db.Where("code = ?", code).Take(&permission).Error; err != nil {
		t.Fatal(err)
	}
	var metadata map[string]string
	if err := json.Unmarshal([]byte(permission.ValuesJSON), &metadata); err != nil {
		t.Fatal(err)
	}
	if permission.ResourceType != PermissionResourceTypePageButton || metadata["menuCode"] != menuCode || metadata["buttonKey"] != buttonKey || metadata["action"] != action {
		t.Fatalf("page permission = %+v, metadata = %+v", permission, metadata)
	}
}

func hasLifecycleReference(references []ResourceLifecycleReference, kind, code string) bool {
	for _, reference := range references {
		if reference.Kind == kind && reference.Code == code {
			return true
		}
	}
	return false
}
