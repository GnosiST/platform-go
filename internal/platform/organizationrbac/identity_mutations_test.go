package organizationrbac

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestChangeUserOrganizationDerivesTenantAndRequiresReviewedConflictRemediation(t *testing.T) {
	db, repository := prepareOrganizationRBACTestRepository(t)
	seedOrganizationRBAC(t, db)
	now := time.Date(2026, 7, 15, 13, 0, 0, 0, time.UTC)
	if err := db.Create(&gormOrganization{ID: "org-acme-branch", Code: "acme-branch", TenantCode: "acme", Status: StatusEnabled}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repository.ReplaceOrgUnitRoleGroups(context.Background(), ReplaceOrgUnitRoleGroupsRequest{
		OrgUnitCode: "acme-hq", RoleGroupCodes: []string{"acme-ops"}, ExpectedRevision: 0, ActorID: "admin", ChangedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.ReplaceOrgUnitRoleGroups(context.Background(), ReplaceOrgUnitRoleGroupsRequest{
		OrgUnitCode: "acme-branch", RoleGroupCodes: []string{"acme-audit"}, ExpectedRevision: 0, ActorID: "admin", ChangedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&gormUser{ID: "user-alice", Code: "alice", ScopeType: string(ScopeTenant), TenantCode: "acme", OrgUnitCode: "acme-hq", Status: StatusEnabled}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&gormUserRole{UserID: "user-alice", RoleCode: "operator"}).Error; err != nil {
		t.Fatal(err)
	}

	impact, err := repository.PreviewUserOrganizationChange(context.Background(), "alice", "acme-branch", []string{"auditor"})
	if err != nil {
		t.Fatal(err)
	}
	if impact.TargetTenantCode != "acme" || impact.ExpectedRevision != 2 || !reflect.DeepEqual(impact.Conflicts, []RoleAssignmentConflict{{UserCode: "alice", RoleCode: "operator"}}) {
		t.Fatalf("PreviewUserOrganizationChange() = %+v", impact)
	}
	request := ChangeUserOrganizationRequest{
		UserCode: "alice", OrgUnitCode: "acme-branch", RoleCodes: []string{"auditor"},
		ExpectedRevision: impact.ExpectedRevision, ActorID: "admin", ChangedAt: now.Add(time.Minute),
	}
	if _, err := repository.ChangeUserOrganization(context.Background(), request, nil); !errors.Is(err, ErrRolePoolViolation) {
		t.Fatalf("ChangeUserOrganization(without remediation) error = %v", err)
	}
	revision, err := repository.ChangeUserOrganization(context.Background(), request, []RoleAssignmentRemediation{{
		UserCode: "alice", RoleCode: "operator", Action: "replace-role", ReplacementRoleCode: "auditor",
	}})
	if err != nil || revision != 3 {
		t.Fatalf("ChangeUserOrganization() revision = %d, error = %v", revision, err)
	}
	user, roles, err := loadUserWithRoles(db, "alice")
	if err != nil || user.OrgUnitCode != "acme-branch" || user.TenantCode != "acme" || !reflect.DeepEqual(roles, []string{"auditor"}) {
		t.Fatalf("changed user = %+v roles=%v error=%v", user, roles, err)
	}
	if _, err := repository.ChangeUserOrganization(context.Background(), request, []RoleAssignmentRemediation{{
		UserCode: "alice", RoleCode: "operator", Action: "replace-role", ReplacementRoleCode: "auditor",
	}}); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("ChangeUserOrganization(stale) error = %v", err)
	}
}

func TestMoveAndDisableRoleRejectUnresolvedAssignments(t *testing.T) {
	db, repository := prepareOrganizationRBACTestRepository(t)
	seedOrganizationRBAC(t, db)
	now := time.Date(2026, 7, 15, 14, 0, 0, 0, time.UTC)
	if err := db.Create(&gormRole{ID: "role-viewer", Code: "viewer", GroupCode: "acme-ops", Status: StatusEnabled}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repository.ReplaceOrgUnitRoleGroups(context.Background(), ReplaceOrgUnitRoleGroupsRequest{
		OrgUnitCode: "acme-hq", RoleGroupCodes: []string{"acme-ops"}, ExpectedRevision: 0, ActorID: "admin", ChangedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&gormUser{ID: "user-alice", Code: "alice", ScopeType: string(ScopeTenant), TenantCode: "acme", OrgUnitCode: "acme-hq", Status: StatusEnabled}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&gormUserRole{UserID: "user-alice", RoleCode: "operator"}).Error; err != nil {
		t.Fatal(err)
	}

	impact, err := repository.PreviewRoleMove(context.Background(), "operator", "acme-audit")
	if err != nil || len(impact.Conflicts) != 1 || impact.ExpectedRevision != 1 {
		t.Fatalf("PreviewRoleMove() = %+v, %v", impact, err)
	}
	request := ChangeRoleRequest{RoleCode: "operator", TargetGroupCode: "acme-audit", ExpectedRevision: 1, ActorID: "admin", ChangedAt: now.Add(time.Minute)}
	if _, err := repository.MoveRole(context.Background(), request, nil); !errors.Is(err, ErrRolePoolViolation) {
		t.Fatalf("MoveRole(without remediation) error = %v", err)
	}
	revision, err := repository.MoveRole(context.Background(), request, []RoleAssignmentRemediation{{
		UserCode: "alice", RoleCode: "operator", Action: "replace-role", ReplacementRoleCode: "viewer",
	}})
	if err != nil || revision != 2 {
		t.Fatalf("MoveRole() revision=%d error=%v", revision, err)
	}
	var moved gormRole
	if err := db.Where("code = ?", "operator").Take(&moved).Error; err != nil || moved.GroupCode != "acme-audit" {
		t.Fatalf("moved role = %+v error=%v", moved, err)
	}
	_, roles, err := loadUserWithRoles(db, "alice")
	if err != nil || !reflect.DeepEqual(roles, []string{"viewer"}) {
		t.Fatalf("roles after move = %v error=%v", roles, err)
	}

	disableImpact, err := repository.PreviewRoleDisable(context.Background(), "viewer")
	if err != nil || len(disableImpact.Conflicts) != 1 || disableImpact.ExpectedRevision != 2 {
		t.Fatalf("PreviewRoleDisable() = %+v, %v", disableImpact, err)
	}
	revision, err = repository.DisableRole(context.Background(), ChangeRoleRequest{
		RoleCode: "viewer", ExpectedRevision: 2, ActorID: "admin", ChangedAt: now.Add(2 * time.Minute),
	}, []RoleAssignmentRemediation{{UserCode: "alice", RoleCode: "viewer", Action: "remove-role"}})
	if err != nil || revision != 3 {
		t.Fatalf("DisableRole() revision=%d error=%v", revision, err)
	}
	var disabled gormRole
	if err := db.Where("code = ?", "viewer").Take(&disabled).Error; err != nil || disabled.Status != "disabled" {
		t.Fatalf("disabled role = %+v error=%v", disabled, err)
	}
	_, roles, err = loadUserWithRoles(db, "alice")
	if err != nil || len(roles) != 0 {
		t.Fatalf("roles after disable = %v error=%v", roles, err)
	}
}
