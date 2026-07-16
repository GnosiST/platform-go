package organizationrbac

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/GnosiST/platform-go/internal/platform/adminresource"
)

func TestAdminOrgUnitSnapshotWriterCreatesAndUpdatesMetadata(t *testing.T) {
	db, _ := prepareOrganizationRBACTestRepository(t)
	if err := db.AutoMigrate(&gormOrgUnitMetadata{}, &gormOrgUnitTenant{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&gormOrgUnitTenant{Code: "acme", Status: StatusEnabled}).Error; err != nil {
		t.Fatal(err)
	}
	writer := NewAdminOrgUnitSnapshotWriter()
	created := adminresource.Record{
		ID: "org-acme-hq", Code: "acme-hq", Name: "Acme HQ", Status: StatusEnabled, UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		Values: map[string]string{"type": "organization", "tenantCode": "acme", "sortOrder": "10"},
	}
	if err := writer.ApplyOrgUnitSnapshot(context.Background(), db, nil, []adminresource.Record{created}); err != nil {
		t.Fatalf("ApplyOrgUnitSnapshot(create) error = %v", err)
	}
	updated := created
	updated.Name = "Acme Headquarters"
	updated.Values = map[string]string{"type": "company", "tenantCode": "acme", "areaCode": "110000", "sortOrder": "20"}
	if err := writer.ApplyOrgUnitSnapshot(context.Background(), db, []adminresource.Record{created}, []adminresource.Record{updated}); err != nil {
		t.Fatalf("ApplyOrgUnitSnapshot(update) error = %v", err)
	}
	var row gormOrgUnitMetadata
	if err := db.Where("code = ?", "acme-hq").Take(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Name != updated.Name || row.Type != "company" || row.AreaCode != "110000" || row.SortOrder != 20 || row.TenantCode != "acme" {
		t.Fatalf("row = %+v", row)
	}
}

func TestAdminOrgUnitSnapshotWriterProtectsOwnedChangesAndHierarchy(t *testing.T) {
	db, _ := prepareOrganizationRBACTestRepository(t)
	if err := db.AutoMigrate(&gormOrgUnitMetadata{}, &gormOrgUnitTenant{}); err != nil {
		t.Fatal(err)
	}
	for _, tenant := range []gormOrgUnitTenant{{Code: "acme", Status: StatusEnabled}, {Code: "other", Status: StatusEnabled}} {
		if err := db.Create(&tenant).Error; err != nil {
			t.Fatal(err)
		}
	}
	writer := NewAdminOrgUnitSnapshotWriter()
	root := adminresource.Record{ID: "org-acme", Code: "acme", Name: "Acme", Status: StatusEnabled, Values: map[string]string{"type": "organization", "tenantCode": "acme"}}
	child := adminresource.Record{ID: "org-acme-team", Code: "acme-team", Name: "Team", Status: StatusEnabled, Values: map[string]string{"type": "team", "tenantCode": "acme", "parentCode": "acme"}}
	if err := writer.ApplyOrgUnitSnapshot(context.Background(), db, nil, []adminresource.Record{root, child}); err != nil {
		t.Fatal(err)
	}

	mutations := []adminresource.Record{
		func() adminresource.Record { next := root; next.Status = "disabled"; return next }(),
		func() adminresource.Record {
			next := root
			next.Values = map[string]string{"type": "organization", "tenantCode": "other"}
			return next
		}(),
		func() adminresource.Record {
			next := root
			next.Values = map[string]string{"type": "organization", "tenantCode": "acme", "roleGroupCodes": "acme-ops"}
			return next
		}(),
	}
	for _, changed := range mutations {
		if err := writer.ApplyOrgUnitSnapshot(context.Background(), db, []adminresource.Record{root, child}, []adminresource.Record{changed, child}); !errors.Is(err, adminresource.ErrDomainOwnedMutation) {
			t.Fatalf("ApplyOrgUnitSnapshot(owned mutation) error = %v", err)
		}
	}

	cycleRoot := root
	cycleRoot.Values = map[string]string{"type": "organization", "tenantCode": "acme", "parentCode": "acme-team"}
	if err := writer.ApplyOrgUnitSnapshot(context.Background(), db, []adminresource.Record{root, child}, []adminresource.Record{cycleRoot, child}); !errors.Is(err, adminresource.ErrInvalidRecord) {
		t.Fatalf("ApplyOrgUnitSnapshot(cycle) error = %v", err)
	}
	crossTenantChild := adminresource.Record{ID: "org-other-team", Code: "other-team", Name: "Other Team", Status: StatusEnabled, Values: map[string]string{"type": "team", "tenantCode": "other", "parentCode": "acme"}}
	if err := writer.ApplyOrgUnitSnapshot(context.Background(), db, []adminresource.Record{root, child}, []adminresource.Record{root, child, crossTenantChild}); !errors.Is(err, adminresource.ErrInvalidRecord) {
		t.Fatalf("ApplyOrgUnitSnapshot(cross tenant parent) error = %v", err)
	}
}

func TestAdminOrgUnitSnapshotWriterKeepsUnchangedDeletedRecords(t *testing.T) {
	db, _ := prepareOrganizationRBACTestRepository(t)
	if err := db.AutoMigrate(&gormOrgUnitMetadata{}, &gormOrgUnitTenant{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&gormOrgUnitTenant{Code: "acme", Status: StatusEnabled}).Error; err != nil {
		t.Fatal(err)
	}
	writer := NewAdminOrgUnitSnapshotWriter()
	deleted := adminresource.Record{
		ID: "org-deleted", Code: "deleted", Name: "Deleted", Status: StatusEnabled,
		Values: map[string]string{"type": "team", "tenantCode": "acme"},
	}
	live := adminresource.Record{
		ID: "org-live", Code: "live", Name: "Live", Status: StatusEnabled,
		Values: map[string]string{"type": "team", "tenantCode": "acme"},
	}
	if err := writer.ApplyOrgUnitSnapshot(context.Background(), db, nil, []adminresource.Record{deleted, live}); err != nil {
		t.Fatal(err)
	}
	deleted.DeletedAt = time.Now().UTC().Format(time.RFC3339)
	deleted.DeletedBy = "admin"
	deleted.DeleteReason = "retired"
	deleted.PurgeAfter = time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339)
	deleted.DeletionPolicyVersion = 1
	updated := live
	updated.Name = "Live Updated"

	if err := writer.ApplyOrgUnitSnapshot(
		context.Background(),
		db,
		[]adminresource.Record{deleted, live},
		[]adminresource.Record{deleted, updated},
	); err != nil {
		t.Fatalf("ApplyOrgUnitSnapshot(unchanged deleted record) error = %v", err)
	}
	var row gormOrgUnitMetadata
	if err := db.Where("code = ?", live.Code).Take(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Name != updated.Name {
		t.Fatalf("row.Name = %q, want %q", row.Name, updated.Name)
	}
}
