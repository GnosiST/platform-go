package organizationrbac

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/GnosiST/platform-go/internal/platform/adminresource"
)

func TestAdminUserSnapshotWriterCreatesValidatedTenantUserAndProtectsAuthorizationUpdates(t *testing.T) {
	db, repository := prepareOrganizationRBACTestRepository(t)
	seedOrganizationRBAC(t, db)
	if _, err := repository.ReplaceOrgUnitRoleGroups(context.Background(), ReplaceOrgUnitRoleGroupsRequest{
		OrgUnitCode: "acme-hq", RoleGroupCodes: []string{"acme-ops"}, ExpectedRevision: 0,
		ActorID: "admin", ChangedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	writer := NewAdminUserSnapshotWriter()
	created := adminresource.Record{
		ID: "user-alice", Code: "alice", Name: "Alice", Status: StatusEnabled, UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		Values: map[string]string{
			"scopeType": "tenant", "tenantCode": "acme", "orgUnitCode": "acme-hq", "roles": "operator",
		},
	}
	if err := writer.ApplyUserSnapshot(context.Background(), db, nil, []adminresource.Record{created}); err != nil {
		t.Fatalf("ApplyUserSnapshot(create) error = %v", err)
	}
	user, roles, err := loadUserWithRoles(db, "alice")
	if err != nil || user.TenantCode != "acme" || user.OrgUnitCode != "acme-hq" || len(roles) != 1 || roles[0] != "operator" {
		t.Fatalf("user = %+v roles=%v error=%v", user, roles, err)
	}

	metadataUpdate := created
	metadataUpdate.Name = "Alice Updated"
	if err := writer.ApplyUserSnapshot(context.Background(), db, []adminresource.Record{created}, []adminresource.Record{metadataUpdate}); err != nil {
		t.Fatalf("ApplyUserSnapshot(metadata update) error = %v", err)
	}
	authorizationUpdate := metadataUpdate
	authorizationUpdate.Values = map[string]string{
		"scopeType": "tenant", "tenantCode": "acme", "orgUnitCode": "acme-hq", "roles": "auditor",
	}
	if err := writer.ApplyUserSnapshot(context.Background(), db, []adminresource.Record{metadataUpdate}, []adminresource.Record{authorizationUpdate}); !errors.Is(err, adminresource.ErrDomainOwnedMutation) {
		t.Fatalf("ApplyUserSnapshot(auth update) error = %v", err)
	}
}

func TestAdminUserSnapshotWriterRejectsClientTenantMismatchAndPlatformCreation(t *testing.T) {
	db, repository := prepareOrganizationRBACTestRepository(t)
	seedOrganizationRBAC(t, db)
	if _, err := repository.ReplaceOrgUnitRoleGroups(context.Background(), ReplaceOrgUnitRoleGroupsRequest{
		OrgUnitCode: "acme-hq", RoleGroupCodes: []string{"acme-ops"}, ExpectedRevision: 0,
		ActorID: "admin", ChangedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	writer := NewAdminUserSnapshotWriter()
	for _, record := range []adminresource.Record{
		{ID: "user-wrong-tenant", Code: "wrong-tenant", Status: StatusEnabled, Values: map[string]string{
			"scopeType": "tenant", "tenantCode": "other", "orgUnitCode": "acme-hq", "roles": "operator",
		}},
		{ID: "user-platform", Code: "platform-user", Status: StatusEnabled, Values: map[string]string{
			"scopeType": "platform", "roles": "super-admin",
		}},
	} {
		if err := writer.ApplyUserSnapshot(context.Background(), db, nil, []adminresource.Record{record}); !errors.Is(err, adminresource.ErrInvalidRecord) && !errors.Is(err, adminresource.ErrDomainOwnedMutation) {
			t.Fatalf("ApplyUserSnapshot(%s) error = %v", record.Code, err)
		}
	}
}
