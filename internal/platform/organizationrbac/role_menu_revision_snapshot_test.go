package organizationrbac

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func TestGORMRepositoryLoadsDeterministicRoleMenuRevisionSnapshot(t *testing.T) {
	db, repository := prepareOrganizationRBACTestRepository(t)
	seedOrganizationRBAC(t, db)
	seedNativeMenus(t, db)
	now := time.Date(2026, 7, 15, 18, 0, 0, 0, time.UTC)
	for _, request := range []ReplaceRoleMenusRequest{
		{RoleCode: "operator", MenuCodes: []string{"users"}, ExpectedRevision: 0, ActorID: "admin", ChangedAt: now},
		{RoleCode: "auditor", MenuCodes: []string{"reports"}, ExpectedRevision: 0, ActorID: "admin", ChangedAt: now.Add(time.Minute)},
		{RoleCode: "operator", MenuCodes: []string{"reports"}, ExpectedRevision: 1, ActorID: "admin", ChangedAt: now.Add(2 * time.Minute)},
	} {
		if _, err := repository.ReplaceRoleMenus(context.Background(), request); err != nil {
			t.Fatal(err)
		}
	}

	snapshot, err := repository.LoadRoleMenuRevisionSnapshot(context.Background(), []string{
		"operator", "missing-role", "super-admin", "auditor", "operator",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := RoleMenuRevisionSnapshot{
		GlobalRevision: 3,
		RoleRevisions: []RoleMenuRevision{
			{RoleCode: "auditor", Revision: 1},
			{RoleCode: "operator", Revision: 2},
			{RoleCode: "super-admin", Revision: 0},
		},
	}
	if !reflect.DeepEqual(snapshot, want) {
		t.Fatalf("LoadRoleMenuRevisionSnapshot() = %+v, want %+v", snapshot, want)
	}
}

func TestGORMRepositoryRoleMenuRevisionSnapshotIgnoresInactiveRoles(t *testing.T) {
	db, repository := prepareOrganizationRBACTestRepository(t)
	seedOrganizationRBAC(t, db)
	now := time.Date(2026, 7, 15, 19, 0, 0, 0, time.UTC)
	if err := db.Create(&[]gormRoleMenuRevision{
		{RoleCode: "disabled-role", Revision: 7, UpdatedAt: now},
		{RoleCode: "auditor", Revision: 4, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&gormResourceLifecycle{
		Resource: "roles", RecordID: "role-auditor", DeletedAt: now.Format(time.RFC3339), DeletedBy: "admin",
		DeleteReason: LifecycleOperationDelete, PurgeAfter: now.Add(30 * 24 * time.Hour).Format(time.RFC3339), DeletionPolicyVersion: 1,
	}).Error; err != nil {
		t.Fatal(err)
	}

	snapshot, err := repository.LoadRoleMenuRevisionSnapshot(context.Background(), []string{"disabled-role", "auditor", "operator", "missing-role"})
	if err != nil {
		t.Fatal(err)
	}
	want := RoleMenuRevisionSnapshot{GlobalRevision: 0, RoleRevisions: []RoleMenuRevision{{RoleCode: "operator", Revision: 0}}}
	if !reflect.DeepEqual(snapshot, want) {
		t.Fatalf("LoadRoleMenuRevisionSnapshot(inactive) = %+v, want %+v", snapshot, want)
	}
}

func TestGORMRepositoryRoleMenuRevisionSnapshotAllowsEmptyRoles(t *testing.T) {
	_, repository := prepareOrganizationRBACTestRepository(t)
	snapshot, err := repository.LoadRoleMenuRevisionSnapshot(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	want := RoleMenuRevisionSnapshot{RoleRevisions: []RoleMenuRevision{}}
	if !reflect.DeepEqual(snapshot, want) {
		t.Fatalf("LoadRoleMenuRevisionSnapshot(empty) = %+v, want %+v", snapshot, want)
	}
}
