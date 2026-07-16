package organizationrbac

import (
	"context"
	"errors"
	"testing"

	"github.com/GnosiST/platform-go/internal/platform/adminresource"
)

func TestAdminRoleSnapshotWriterAllowsMetadataAndBlocksAuthorizationChanges(t *testing.T) {
	db, _ := prepareOrganizationRBACTestRepository(t)
	if err := db.AutoMigrate(&gormRoleMetadata{}, &gormRoleGroupMetadata{}, &gormOrgUnitTenant{}); err != nil {
		t.Fatal(err)
	}
	seedOrganizationRBAC(t, db)
	writer := NewAdminRoleSnapshotWriter()
	current := []adminresource.Record{{
		ID: "role-operator", Code: "operator", Name: "Operator", Status: StatusEnabled,
		Values: map[string]string{"groupCode": "acme-ops", "permissions": "admin:user:read", "denyPermissions": "admin:user:delete", "dataScope": "current_org"},
	}}
	proposed := []adminresource.Record{{
		ID: "role-operator", Code: "operator", Name: "Tenant Operator", Status: StatusEnabled, Description: "metadata only", UpdatedAt: "2026-07-15T18:00:00Z",
		Values: map[string]string{"groupCode": "acme-ops", "permissions": "admin:user:read", "denyPermissions": "admin:user:delete", "dataScope": "current_org"},
	}}
	if err := writer.ApplyRoleSnapshot(context.Background(), db, current, proposed); err != nil {
		t.Fatalf("ApplyRoleSnapshot(metadata) error = %v", err)
	}
	var row gormRoleMetadata
	if err := db.Where("code = ?", "operator").Take(&row).Error; err != nil || row.Name != "Tenant Operator" || row.Description != "metadata only" {
		t.Fatalf("role metadata = %+v, error = %v", row, err)
	}
	for _, mutation := range []func(*adminresource.Record){
		func(record *adminresource.Record) { record.Status = "disabled" },
		func(record *adminresource.Record) { record.Values["groupCode"] = "acme-audit" },
		func(record *adminresource.Record) { record.Values["permissions"] = "admin:*" },
		func(record *adminresource.Record) { record.Values["denyPermissions"] = "" },
		func(record *adminresource.Record) { record.Values["dataScope"] = "all" },
	} {
		changed := proposed[0]
		changed.Values = cloneTestValues(proposed[0].Values)
		mutation(&changed)
		if err := writer.ApplyRoleSnapshot(context.Background(), db, proposed, []adminresource.Record{changed}); !errors.Is(err, adminresource.ErrDomainOwnedMutation) {
			t.Fatalf("ApplyRoleSnapshot(authorization mutation) error = %v, want ErrDomainOwnedMutation", err)
		}
	}
}

func TestAdminRoleSnapshotWriterCreatesStrictRoleGroupAndRole(t *testing.T) {
	db, _ := prepareOrganizationRBACTestRepository(t)
	if err := db.AutoMigrate(&gormRoleMetadata{}, &gormRoleGroupMetadata{}, &gormOrgUnitTenant{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&gormOrgUnitTenant{Code: "acme", Status: StatusEnabled}).Error; err != nil {
		t.Fatal(err)
	}
	writer := NewAdminRoleSnapshotWriter()
	group := adminresource.Record{
		ID: "group-acme-support", Code: "acme-support", Name: "Support", Status: StatusEnabled, UpdatedAt: "2026-07-15T18:00:00Z",
		Values: map[string]string{"scopeType": "tenant", "tenantCode": "acme", "sortOrder": "30"},
	}
	if err := writer.ApplyRoleGroupSnapshot(context.Background(), db, nil, []adminresource.Record{group}); err != nil {
		t.Fatalf("ApplyRoleGroupSnapshot(create) error = %v", err)
	}
	role := adminresource.Record{
		ID: "role-support", Code: "support", Name: "Support", Status: StatusEnabled, UpdatedAt: "2026-07-15T18:01:00Z",
		Values: map[string]string{"groupCode": "acme-support"},
	}
	if err := writer.ApplyRoleSnapshot(context.Background(), db, nil, []adminresource.Record{role}); err != nil {
		t.Fatalf("ApplyRoleSnapshot(create) error = %v", err)
	}
	groupWithParent := group
	groupWithParent.Values = cloneTestValues(group.Values)
	groupWithParent.Values["parentCode"] = "acme-ops"
	if err := writer.ApplyRoleGroupSnapshot(context.Background(), db, []adminresource.Record{group}, []adminresource.Record{groupWithParent}); !errors.Is(err, adminresource.ErrDomainOwnedMutation) {
		t.Fatalf("ApplyRoleGroupSnapshot(parentCode) error = %v, want ErrDomainOwnedMutation", err)
	}
	role.Values["permissions"] = "admin:user:read"
	if err := writer.ApplyRoleSnapshot(context.Background(), db, nil, []adminresource.Record{role}); !errors.Is(err, adminresource.ErrDomainOwnedMutation) {
		t.Fatalf("ApplyRoleSnapshot(create with permissions) error = %v, want ErrDomainOwnedMutation", err)
	}
}

func cloneTestValues(source map[string]string) map[string]string {
	result := make(map[string]string, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
