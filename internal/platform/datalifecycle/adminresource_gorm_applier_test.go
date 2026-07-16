package datalifecycle

import (
	"context"
	"errors"
	"testing"
	"time"

	"platform-go/internal/platform/adminresource"
	"platform-go/internal/platform/capability"

	"gorm.io/gorm"
)

func TestGORMAdminResourceApplierCommitsPurgeAuditAndCheckpointTogether(t *testing.T) {
	db := openLifecycleTestDB(t)
	adminRepository, lifecycleRepository, manifests := prepareAdminLifecycleApplyTest(t, db)
	request := databaseApplyRequest(t, lifecycleRepository)
	applier := NewGORMAdminResourceApplier(db, manifests, nil)

	result, err := applier.ApplyAndCheckpoint(context.Background(), request)
	if err != nil {
		t.Fatalf("ApplyAndCheckpoint() error = %v", err)
	}
	if result.Applied != 1 || result.Checkpoint != request.Checkpoint {
		t.Fatalf("ApplyAndCheckpoint() result = %+v", result)
	}
	snapshot, err := adminRepository.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if records := snapshot.Resources["lifecycle-records"]; len(records) != 0 {
		t.Fatalf("purged records = %+v", records)
	}
	if audits := snapshot.Resources["audit-logs"]; countAuditAction(audits, "admin_resource.purge") != 1 {
		t.Fatalf("purge audits = %+v", audits)
	}
	checkpoint, found, err := lifecycleRepository.LoadCheckpoint(context.Background(), request.Checkpoint.Key)
	if err != nil || !found || checkpoint != request.Checkpoint {
		t.Fatalf("checkpoint = %+v, %t, %v", checkpoint, found, err)
	}
}

func TestGORMAdminResourceApplierRollsBackPurgeWhenCheckpointFails(t *testing.T) {
	db := openLifecycleTestDB(t)
	adminRepository, lifecycleRepository, manifests := prepareAdminLifecycleApplyTest(t, db)
	request := databaseApplyRequest(t, lifecycleRepository)
	if err := db.Exec(`CREATE TRIGGER fail_lifecycle_checkpoint_insert BEFORE INSERT ON platform_data_lifecycle_checkpoints BEGIN SELECT RAISE(FAIL, 'forced checkpoint failure'); END`).Error; err != nil {
		t.Fatal(err)
	}
	applier := NewGORMAdminResourceApplier(db, manifests, nil)

	if _, err := applier.ApplyAndCheckpoint(context.Background(), request); !errors.Is(err, ErrApplyFailed) {
		t.Fatalf("ApplyAndCheckpoint() error = %v, want ErrApplyFailed", err)
	}
	snapshot, err := adminRepository.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if records := snapshot.Resources["lifecycle-records"]; len(records) != 1 || records[0].ID != "record-1" {
		t.Fatalf("record purge escaped rollback: %+v", records)
	}
	if audits := snapshot.Resources["audit-logs"]; countAuditAction(audits, "admin_resource.purge") != 0 {
		t.Fatalf("audit escaped rollback: %+v", audits)
	}
	if _, found, err := lifecycleRepository.LoadCheckpoint(context.Background(), request.Checkpoint.Key); err != nil || found {
		t.Fatalf("checkpoint after rollback found=%t error=%v", found, err)
	}
}

func TestGORMAdminResourceApplierRejectsDeletionModeDrift(t *testing.T) {
	db := openLifecycleTestDB(t)
	_, lifecycleRepository, manifests := prepareAdminLifecycleApplyTest(t, db)
	request := databaseApplyRequest(t, lifecycleRepository)
	request.Policy.Resources[0].Mode = capability.AdminDeletionTombstone
	applier := NewGORMAdminResourceApplier(db, manifests, nil)

	if _, err := applier.ApplyAndCheckpoint(context.Background(), request); !errors.Is(err, ErrPolicyDrift) {
		t.Fatalf("ApplyAndCheckpoint(mode drift) error = %v, want ErrPolicyDrift", err)
	}
}

func TestGORMAdminResourceApplierRejectsAuthorizationLifecycleWithoutCheckpoint(t *testing.T) {
	for _, resource := range []string{"org-units", "role-groups", "roles", "users", "menus", "permissions"} {
		t.Run(resource, func(t *testing.T) {
			db := openLifecycleTestDB(t)
			adminRepository, lifecycleRepository, manifests := prepareAdminLifecycleApplyTest(t, db)
			snapshot, err := adminRepository.Load(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			recordID := resource + "-1"
			snapshot.Resources[resource] = []adminresource.Record{{
				ID: recordID, Code: recordID, Name: "Authorization record", Status: "disabled", UpdatedAt: "2020-01-01T00:00:00Z",
				DeletedAt: "2020-01-01T00:00:00Z", DeletedBy: "admin", DeleteReason: "deleted",
				PurgeAfter: "2020-01-31T00:00:00Z", DeletionPolicyVersion: 1,
			}}
			if _, err := adminRepository.Save(context.Background(), snapshot); err != nil {
				t.Fatal(err)
			}
			manifests[0].Admin.Resources = append(manifests[0].Admin.Resources, capability.AdminResource{
				Resource: resource, Title: capability.Text("授权资源", "Authorization resource"), Description: capability.Text("授权资源。", "Authorization resource."),
				PermissionPrefix: "admin:authorization", Deletion: &capability.AdminResourceDeletionPolicy{
					Mode: capability.AdminDeletionSoftDelete, PolicyVersion: 1, RetentionDays: 30, AutoPurge: true,
				},
				Menu: capability.AdminMenu{Route: "/" + resource, Group: "test", Icon: "shield", Order: 3},
			})
			request := databaseApplyRequestForResource(t, lifecycleRepository, resource, recordID, "2020-01-31T00:00:00Z")
			applier := NewGORMAdminResourceApplier(db, manifests, nil)

			if _, err := applier.ApplyAndCheckpoint(context.Background(), request); !errors.Is(err, ErrApplyFailed) {
				t.Fatalf("ApplyAndCheckpoint(%s lifecycle) error = %v, want ErrApplyFailed", resource, err)
			}
			if _, found, err := lifecycleRepository.LoadCheckpoint(context.Background(), request.Checkpoint.Key); err != nil || found {
				t.Fatalf("%s lifecycle checkpoint found=%t error=%v, want no checkpoint", resource, found, err)
			}
			persisted, err := adminRepository.Load(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if records := persisted.Resources[resource]; len(records) != 1 || records[0].ID != recordID {
				t.Fatalf("%s lifecycle record changed after rejection: %+v", resource, records)
			}
			if audits := persisted.Resources["audit-logs"]; countAuditAction(audits, "admin_resource.purge") != 0 {
				t.Fatalf("%s lifecycle audit escaped rejection: %+v", resource, audits)
			}
		})
	}
}

func TestGORMAdminResourceApplierPurgesOnlyRevokedAPITokenMetadata(t *testing.T) {
	db := openLifecycleTestDB(t)
	adminRepository, err := adminresource.NewGORMAdminResourceRepository(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	lifecycleRepository, err := PrepareGORMRepository(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	manifests := apiTokenLifecycleTestManifests()
	if _, err := adminRepository.Save(context.Background(), adminresource.ResourceSnapshot{
		NextID: 1000,
		Resources: map[string][]adminresource.Record{
			"api-tokens": {
				{ID: "token-revoked", Name: "Revoked", Status: "revoked", UpdatedAt: "2020-01-01T00:00:00Z", Values: map[string]string{"revokedAt": "2020-01-01T00:00:00Z"}},
				{ID: "token-active", Name: "Active", Status: "active", UpdatedAt: "2020-01-01T00:00:00Z", Values: map[string]string{"revokedAt": "2020-01-01T00:00:00Z"}},
			},
			"audit-logs": {},
		},
	}); err != nil {
		t.Fatal(err)
	}
	request := databaseApplyRequestForResource(t, lifecycleRepository, "api-tokens", "token-revoked", "2020-01-31T00:00:00Z")
	applier := NewGORMAdminResourceApplier(db, manifests, nil)

	result, err := applier.ApplyAndCheckpoint(context.Background(), request)
	if err != nil {
		t.Fatalf("ApplyAndCheckpoint(api token) error = %v", err)
	}
	if result.Applied != 1 || result.Checkpoint != request.Checkpoint {
		t.Fatalf("ApplyAndCheckpoint(api token) result = %+v", result)
	}
	snapshot, err := adminRepository.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if records := snapshot.Resources["api-tokens"]; len(records) != 1 || records[0].ID != "token-active" {
		t.Fatalf("remaining api tokens = %+v", records)
	}
	if audits := snapshot.Resources["audit-logs"]; countAuditAction(audits, "admin_resource.purge") != 1 {
		t.Fatalf("api token purge audits = %+v", audits)
	}
	checkpoint, found, err := lifecycleRepository.LoadCheckpoint(context.Background(), request.Checkpoint.Key)
	if err != nil || !found || checkpoint != request.Checkpoint {
		t.Fatalf("api token checkpoint = %+v, %t, %v", checkpoint, found, err)
	}
}

func prepareAdminLifecycleApplyTest(t *testing.T, db *gorm.DB) (*adminresource.GORMAdminResourceRepository, *GORMRepository, []capability.Manifest) {
	t.Helper()
	adminRepository, err := adminresource.NewGORMAdminResourceRepository(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	lifecycleRepository, err := PrepareGORMRepository(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	manifests := databaseLifecycleTestManifests()
	if _, err := adminRepository.Save(context.Background(), adminresource.ResourceSnapshot{
		NextID: 1000,
		Resources: map[string][]adminresource.Record{
			"lifecycle-records": {{
				ID: "record-1", Code: "record-1", Name: "Record 1", Status: "enabled", UpdatedAt: "2020-01-01T00:00:00Z",
				DeletedAt: "2020-01-01T00:00:00Z", DeletedBy: "admin", DeleteReason: "deleted",
				PurgeAfter: "2030-01-01T00:00:00Z", DeletionPolicyVersion: 1,
			}},
			"audit-logs": {},
		},
	}); err != nil {
		t.Fatal(err)
	}
	return adminRepository, lifecycleRepository, manifests
}

func databaseApplyRequest(t *testing.T, repository *GORMRepository) ApplyRequest {
	t.Helper()
	return databaseApplyRequestForResource(t, repository, "lifecycle-records", "record-1", "2020-01-31T00:00:00Z")
}

func databaseApplyRequestForResource(t *testing.T, repository *GORMRepository, resource string, recordID string, eligibleAt string) ApplyRequest {
	t.Helper()
	policy := PolicySnapshot{Version: 1, Resources: []ResourcePolicy{{
		Resource: resource, Mode: deletionModeForResource(resource), PolicyVersion: 1, RetentionDays: 30, AutoPurge: true,
	}}}
	fingerprint, err := PolicyFingerprint(policy)
	if err != nil {
		t.Fatal(err)
	}
	lease, err := repository.AcquireLease(context.Background(), LeaseRequest{
		DatasourceID: DefaultDatasourceID, Key: "data-lifecycle:default", OwnerID: "maintenance-1",
		PolicyFingerprint: fingerprint, Now: time.Now().UTC(), TTL: time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	key := CheckpointKey{DatasourceID: DefaultDatasourceID, RunID: "apply-" + resource, Mode: ModeApply, PolicyFingerprint: fingerprint}
	previous := sealCheckpoint(Checkpoint{
		Key: key, DatasourceID: DefaultDatasourceID, Cursor: Cursor{DatasourceID: DefaultDatasourceID},
		EvidenceHash: stableDigest("evidence-empty", DefaultDatasourceID, fingerprint),
	})
	candidate := Candidate{Resource: resource, EligibleAt: eligibleAt, RecordID: recordID}
	next := sealCheckpoint(Checkpoint{
		Key: key, DatasourceID: DefaultDatasourceID,
		Cursor: Cursor{DatasourceID: DefaultDatasourceID, Resource: candidate.Resource, EligibleAt: candidate.EligibleAt, RecordID: candidate.RecordID},
		Counts: Counts{Applied: 1, Batches: 1}, EvidenceHash: stableDigest("evidence", "record-1"),
		LastBatchID: stableDigest("batch", "record-1"), Revision: 1, Complete: true, UpdatedAt: time.Now().UTC(),
	})
	return ApplyRequest{
		DatasourceID: DefaultDatasourceID, RunID: key.RunID, BatchID: next.LastBatchID,
		Policy: policy, PolicyFingerprint: fingerprint, Lease: lease,
		Batch: Batch{
			DatasourceID: DefaultDatasourceID, Candidates: []Candidate{candidate}, NextCursor: next.Cursor, Done: true,
		},
		PreviousCheckpoint: previous, Checkpoint: next,
	}
}

func deletionModeForResource(resource string) string {
	if resource == "api-tokens" {
		return capability.AdminDeletionRevoke
	}
	return capability.AdminDeletionSoftDelete
}

func apiTokenLifecycleTestManifests() []capability.Manifest {
	revoke := capability.AdminResourceDeletionPolicy{
		Mode: capability.AdminDeletionRevoke, PolicyVersion: 1, RetentionDays: 30, AutoPurge: true,
	}
	appendOnly := capability.AdminResourceDeletionPolicy{Mode: capability.AdminDeletionAppendOnly, PolicyVersion: 1}
	return []capability.Manifest{{ID: "api-token-lifecycle-test", Admin: capability.AdminSurface{Resources: []capability.AdminResource{
		{
			Resource: "api-tokens", Title: capability.Text("API Token", "API Tokens"),
			Description: capability.Text("API Token。", "API tokens."), PermissionPrefix: "admin:api-token",
			Deletion: &revoke, Menu: capability.AdminMenu{Route: "/api-tokens", Group: "test", Icon: "key", Order: 1},
			Fields: []capability.AdminField{
				{Key: "status", Label: capability.Text("状态", "Status"), Type: "text", Source: "record", Searchable: true, InTable: true, InDetail: true},
				{Key: "revokedAt", Label: capability.Text("撤销时间", "Revoked At"), Type: "datetime", Source: "values", ReadOnly: true, Searchable: true, InTable: true, InDetail: true},
			},
		},
		{
			Resource: "audit-logs", Title: capability.Text("审计日志", "Audit Logs"),
			Description: capability.Text("审计日志。", "Audit logs."), PermissionPrefix: "admin:audit-log",
			Deletion: &appendOnly, Menu: capability.AdminMenu{Route: "/audit-logs", Group: "test", Icon: "audit", Order: 2},
		},
	}}}}
}

func databaseLifecycleTestManifests() []capability.Manifest {
	softDelete := capability.AdminResourceDeletionPolicy{
		Mode: capability.AdminDeletionSoftDelete, PolicyVersion: 1, RetentionDays: 30, AutoPurge: true,
	}
	appendOnly := capability.AdminResourceDeletionPolicy{Mode: capability.AdminDeletionAppendOnly, PolicyVersion: 1}
	return []capability.Manifest{{ID: "lifecycle-database-test", Admin: capability.AdminSurface{Resources: []capability.AdminResource{
		{
			Resource: "lifecycle-records", Title: capability.Text("生命周期记录", "Lifecycle Records"),
			Description: capability.Text("生命周期记录。", "Lifecycle records."), PermissionPrefix: "admin:lifecycle-record",
			Deletion: &softDelete, Menu: capability.AdminMenu{Route: "/lifecycle-records", Group: "test", Icon: "test", Order: 1},
		},
		{
			Resource: "audit-logs", Title: capability.Text("审计日志", "Audit Logs"),
			Description: capability.Text("审计日志。", "Audit logs."), PermissionPrefix: "admin:audit-log",
			Deletion: &appendOnly, Menu: capability.AdminMenu{Route: "/audit-logs", Group: "test", Icon: "audit", Order: 2},
		},
	}}}}
}

func countAuditAction(records []adminresource.Record, action string) int {
	count := 0
	for _, record := range records {
		if record.Values["action"] == action {
			count++
		}
	}
	return count
}
