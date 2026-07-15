package organizationrbac

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestGORMRepositoryComparesDeterministicLegacyRoleMenuCandidate(t *testing.T) {
	db, repository := prepareOrganizationRBACTestRepository(t)
	seedOrganizationRBAC(t, db)
	seedNativeMenus(t, db)
	if err := db.Model(&gormMenu{}).Where("code = ?", "users").Update("permission", "admin:user:read").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&gormMenu{}).Where("code = ?", "reports").Update("permission", "admin:report:read").Error; err != nil {
		t.Fatal(err)
	}
	permissions := []gormPermission{
		{ID: "permission-user-read", Code: "admin:user:read", Name: "Read users", Status: StatusEnabled, ResourceType: PermissionResourceTypeAPI, Resource: "users", Action: "read", ValuesJSON: "{}"},
		{ID: "permission-report-read", Code: "admin:report:read", Name: "Read reports", Status: StatusEnabled, ResourceType: PermissionResourceTypeAPI, Resource: "reports", Action: "read", ValuesJSON: "{}"},
		{ID: "permission-all", Code: "*", Name: "All", Status: StatusEnabled, ResourceType: PermissionResourceTypeAPI, Resource: "*", Action: "*", ValuesJSON: "{}"},
	}
	if err := db.Create(&permissions).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&[]gormRolePermission{{RoleCode: "operator", Permission: "admin:user:read"}, {RoleCode: "super-admin", Permission: "*"}}).Error; err != nil {
		t.Fatal(err)
	}

	comparison, err := repository.CompareRoleMenuCandidate(context.Background(), "operator")
	if err != nil || !reflect.DeepEqual(comparison.LegacyMenuCodes, []string{"users"}) || len(comparison.TargetMenuCodes) != 0 ||
		len(comparison.AddedMenuCodes) != 0 || !reflect.DeepEqual(comparison.RemovedMenuCodes, []string{"users"}) || comparison.PrincipalEquivalenceClaimed {
		t.Fatalf("CompareRoleMenuCandidate(operator) = %+v, %v", comparison, err)
	}
	if _, err := repository.ReplaceRoleMenus(context.Background(), ReplaceRoleMenusRequest{
		RoleCode: "operator", MenuCodes: []string{"users"}, ExpectedRevision: 0, ActorID: "admin", ChangedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	comparison, err = repository.CompareRoleMenuCandidate(context.Background(), "operator")
	if err != nil || comparison.TargetRevision != 1 || len(comparison.AddedMenuCodes) != 0 || len(comparison.RemovedMenuCodes) != 0 {
		t.Fatalf("CompareRoleMenuCandidate(matched) = %+v, %v", comparison, err)
	}

	wildcard, err := repository.CompareRoleMenuCandidate(context.Background(), "super-admin")
	if err != nil || !reflect.DeepEqual(wildcard.LegacyMenuCodes, []string{"reports", "users"}) {
		t.Fatalf("CompareRoleMenuCandidate(wildcard) = %+v, %v", wildcard, err)
	}
	if _, err := repository.CompareRoleMenuCandidate(context.Background(), "disabled-role"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("CompareRoleMenuCandidate(disabled) error = %v, want ErrInvalid", err)
	}
}

func TestGORMRepositoryResolvesEnabledRolePagesAndDirectoryAncestors(t *testing.T) {
	db, repository := prepareOrganizationRBACTestRepository(t)
	seedOrganizationRBAC(t, db)
	seedNativeMenus(t, db)
	if err := db.Create(&gormMenu{ID: "menu-root", Code: "root", Name: "Root", Status: StatusEnabled, NodeType: string(MenuNodeTypeDirectory), TitleZH: "根", TitleEN: "Root", ParametersJSON: "[]", ValuesJSON: "{}"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&gormMenu{}).Where("code = ?", "access").Updates(map[string]any{"parent_code": "root", "parent": "root"}).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	bindings := []gormRoleMenu{
		{RoleCode: "operator", MenuCode: "users", Revision: 1, ActorID: "admin", CreatedAt: now, UpdatedAt: now},
		{RoleCode: "auditor", MenuCode: "reports", Revision: 1, ActorID: "admin", CreatedAt: now, UpdatedAt: now},
		{RoleCode: "disabled-role", MenuCode: "reports", Revision: 1, ActorID: "admin", CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&bindings).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&gormPageButton{MenuCode: "users", ButtonKey: "export", LabelZH: "导出", LabelEN: "Export", Action: "export", Status: StatusEnabled, PermissionCode: "page:user:export"}).Error; err != nil {
		t.Fatal(err)
	}

	resolved, err := repository.ResolveRoleMenuNodes(context.Background(), []string{"auditor", "operator", "disabled-role"})
	if err != nil || !reflect.DeepEqual(menuNodeCodes(resolved.Nodes), []string{"access", "reports", "root", "users"}) {
		t.Fatalf("ResolveRoleMenuNodes() = %+v, %v", resolved, err)
	}
	if resolved.PrincipalEquivalenceClaimed {
		t.Fatalf("ResolveRoleMenuNodes() claimed principal equivalence: %+v", resolved)
	}
	if err := db.Model(&gormMenu{}).Where("code = ?", "reports").Update("status", "disabled").Error; err != nil {
		t.Fatal(err)
	}
	resolved, err = repository.ResolveRoleMenuNodes(context.Background(), []string{"auditor", "operator"})
	if err != nil || !reflect.DeepEqual(menuNodeCodes(resolved.Nodes), []string{"access", "root", "users"}) {
		t.Fatalf("ResolveRoleMenuNodes(disabled page) = %+v, %v", resolved, err)
	}
}

func menuNodeCodes(nodes []MenuNode) []string {
	codes := make([]string, 0, len(nodes))
	for _, node := range nodes {
		codes = append(codes, node.Code)
	}
	return codes
}
