package adminresource

import (
	"errors"
	"testing"
	"time"

	"platform-go/internal/platform/capability"
	"platform-go/internal/platform/rbac"
)

func TestRequiresGovernedLifecycleCommandCoversAuthorizationEntities(t *testing.T) {
	for _, resource := range []string{"org-units", "role-groups", "roles", "users", "menus", "permissions"} {
		if !RequiresGovernedLifecycleCommand(resource) {
			t.Fatalf("RequiresGovernedLifecycleCommand(%q) = false, want true", resource)
		}
	}
	for _, resource := range []string{"", "tenants", "files", " users"} {
		if RequiresGovernedLifecycleCommand(resource) {
			t.Fatalf("RequiresGovernedLifecycleCommand(%q) = true, want false", resource)
		}
	}
}

func TestSoftDeleteHidesRecordsUntilRestore(t *testing.T) {
	now := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	store := lifecycleTestStore(capability.AdminResourceDeletionPolicy{
		Mode: capability.AdminDeletionSoftDelete, PolicyVersion: 3, RetentionDays: 7,
	})
	store.now = func() time.Time { return now }
	record, err := store.Create("lifecycle-records", WriteInput{Code: "record-1", Name: "Record 1", Status: "enabled"})
	if err != nil {
		t.Fatal(err)
	}

	result, err := store.DeleteWithAudit("lifecycle-records", record.ID, AuditEvent{
		Actor: "user-admin", Action: "admin_resource.delete", Result: "success", ReasonCode: "operator-request",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Record.DeletedAt != now.Format(time.RFC3339) || result.Record.DeletedBy != "user-admin" || result.Record.DeleteReason != "operator-request" {
		t.Fatalf("deleted lifecycle = %+v", result.Record)
	}
	if result.Record.DeletionPolicyVersion != 3 || result.Record.PurgeAfter != now.Add(7*24*time.Hour).Format(time.RFC3339) {
		t.Fatalf("deleted retention = %+v", result.Record)
	}
	if items, _ := store.List("lifecycle-records"); len(items) != 0 {
		t.Fatalf("List() returned soft-deleted records: %+v", items)
	}
	if query, queryErr := store.Query("lifecycle-records", QueryInput{}); queryErr != nil || query.Total != 0 {
		t.Fatalf("Query() = %+v, %v", query, queryErr)
	}
	principal := rbac.Principal{User: rbac.User{Username: "admin", TenantCode: "platform"}}
	if items, _ := store.ListForPrincipal("lifecycle-records", principal); len(items) != 0 {
		t.Fatalf("ListForPrincipal() returned soft-deleted records: %+v", items)
	}
	if _, updateErr := store.Update("lifecycle-records", record.ID, WriteInput{Name: "Changed"}); !errors.Is(updateErr, ErrRecordDeleted) {
		t.Fatalf("Update() error = %v, want ErrRecordDeleted", updateErr)
	}
	repeated, err := store.DeleteWithAudit("lifecycle-records", record.ID, AuditEvent{
		Actor: "user-admin", Action: "admin_resource.delete", Result: "success", ReasonCode: "operator-request",
	})
	if err != nil {
		t.Fatalf("repeated DeleteWithAudit() error = %v", err)
	}
	if repeated.Record.DeletedAt != result.Record.DeletedAt || repeated.Record.PurgeAfter != result.Record.PurgeAfter {
		t.Fatalf("repeated delete changed lifecycle window: first=%+v repeated=%+v", result.Record, repeated.Record)
	}

	restored, err := store.RestoreWithAudit("lifecycle-records", record.ID, AuditEvent{
		Actor: "user-admin", Action: "admin_resource.restore", Result: "success", ReasonCode: "approved",
	})
	if err != nil {
		t.Fatal(err)
	}
	if restored.Record.DeletedAt != "" || restored.Record.DeletedBy != "" || restored.Record.DeleteReason != "" || restored.Record.PurgeAfter != "" || restored.Record.DeletionPolicyVersion != 0 {
		t.Fatalf("restored lifecycle metadata not cleared: %+v", restored.Record)
	}
	if items, _ := store.List("lifecycle-records"); len(items) != 1 || items[0].ID != record.ID {
		t.Fatalf("List() after restore = %+v", items)
	}
}

func TestRestoreRejectsExpiredWindow(t *testing.T) {
	now := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	store := lifecycleTestStore(capability.AdminResourceDeletionPolicy{
		Mode: capability.AdminDeletionSoftDelete, PolicyVersion: 1, RetentionDays: 1,
	})
	store.now = func() time.Time { return now }
	record, err := store.Create("lifecycle-records", WriteInput{Code: "record-1", Name: "Record 1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.DeleteWithAudit("lifecycle-records", record.ID, AuditEvent{Actor: "admin", Action: "admin_resource.delete", Result: "success", ReasonCode: "deleted"}); err != nil {
		t.Fatal(err)
	}
	now = now.Add(24 * time.Hour)
	if _, err := store.RestoreWithAudit("lifecycle-records", record.ID, AuditEvent{Actor: "admin", Action: "admin_resource.restore", Result: "success", ReasonCode: "approved"}); !errors.Is(err, ErrRestoreWindowExpired) {
		t.Fatalf("RestoreWithAudit(expired) error = %v, want ErrRestoreWindowExpired", err)
	}
}

func TestRestoreProjectsOmittedInternalFields(t *testing.T) {
	store := lifecycleFileTestStore()
	record, err := store.CreateInternal("files", WriteInput{
		Code: "file-restore-projection", Name: "File Restore Projection", Status: "enabled",
		Values: map[string]string{"storageKey": "objects/private-file"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.TombstoneFileWithAudit(record.ID, AuditEvent{
		Actor: "file-admin", Action: "file.delete.request", Result: "success", ReasonCode: "operator-request",
	}); err != nil {
		t.Fatal(err)
	}
	restored, err := store.RestoreWithAudit("files", record.ID, AuditEvent{
		Actor: "file-admin", Action: "file.restore", Result: "success", ReasonCode: "approved",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, exposed := restored.Record.Values["storageKey"]; exposed {
		t.Fatalf("restore response exposed omitted storageKey: %+v", restored.Record)
	}
	internal, err := store.InternalRecord("files", record.ID)
	if err != nil || internal.Values["storageKey"] != "objects/private-file" {
		t.Fatalf("internal restored record = %+v, %v", internal, err)
	}
}

func TestPurgeRequiresDeletedRecordAndElapsedRetention(t *testing.T) {
	now := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	store := lifecycleTestStore(capability.AdminResourceDeletionPolicy{
		Mode: capability.AdminDeletionSoftDelete, PolicyVersion: 1, RetentionDays: 1, AutoPurge: true,
	})
	store.now = func() time.Time { return now }
	record, err := store.Create("lifecycle-records", WriteInput{Code: "record-1", Name: "Record 1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.PurgeWithAudit("lifecycle-records", record.ID, AuditEvent{Actor: "system", Action: "admin_resource.purge", Result: "success", ReasonCode: "retention-elapsed"}); !errors.Is(err, ErrRecordNotDeleted) {
		t.Fatalf("PurgeWithAudit(active) error = %v, want ErrRecordNotDeleted", err)
	}
	if _, err := store.DeleteWithAudit("lifecycle-records", record.ID, AuditEvent{Actor: "user-admin", Action: "admin_resource.delete", Result: "success", ReasonCode: "deleted"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.PurgeWithAudit("lifecycle-records", record.ID, AuditEvent{Actor: "system", Action: "admin_resource.purge", Result: "success", ReasonCode: "retention-elapsed"}); !errors.Is(err, ErrRetentionNotElapsed) {
		t.Fatalf("PurgeWithAudit(before cutoff) error = %v, want ErrRetentionNotElapsed", err)
	}
	now = now.Add(24 * time.Hour)
	if _, err := store.PurgeWithAudit("lifecycle-records", record.ID, AuditEvent{Actor: "system", Action: "admin_resource.purge", Result: "success", ReasonCode: "retention-elapsed"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.InternalRecord("lifecycle-records", record.ID); !errors.Is(err, ErrRecordNotFound) {
		t.Fatalf("InternalRecord() error = %v, want ErrRecordNotFound", err)
	}
}

func TestFileCleanupStateMachineGuardsRestoreAndPurgeOrdering(t *testing.T) {
	now := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	store := lifecycleFileTestStore()
	store.now = func() time.Time { return now }
	record, err := store.CreateInternal("files", WriteInput{
		Code: "file-1", Name: "File 1", Status: "enabled",
		Values: map[string]string{"storageKey": "objects/file-1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.TombstoneFileWithAudit(record.ID, AuditEvent{
		Actor: "file-admin", Action: "file.delete.request", Result: "success", ReasonCode: "cleanup-pending",
	}); err != nil {
		t.Fatal(err)
	}
	internal, err := store.InternalRecord("files", record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if state := fileDeletionState(internal); state != fileDeletionPending {
		t.Fatalf("tombstone state = %q, want %q", state, fileDeletionPending)
	}
	if _, err := store.PurgeTombstonedFileWithAudit(record.ID, AuditEvent{
		Actor: "retention-runner", Action: "file.purge", Result: "success", ReasonCode: "retention-elapsed",
	}); !errors.Is(err, ErrDeletionCleanupStarted) {
		t.Fatalf("PurgeTombstonedFileWithAudit(before cleanup) error = %v, want ErrDeletionCleanupStarted", err)
	}

	now = now.Add(24 * time.Hour)
	items := store.resources["files"]
	items[0].PurgeAfter = now.Add(29 * 24 * time.Hour).Format(time.RFC3339)
	store.resources["files"] = items
	if _, err := store.ClaimTombstonedFileCleanupWithPolicyAndAudit(record.ID, MaintenanceRetentionPolicy{
		Mode: capability.AdminDeletionTombstone, PolicyVersion: 2, RetentionDays: 1, AutoPurge: true,
	}, AuditEvent{Actor: "retention-runner", Action: "file.cleanup.claim", Result: "success", ReasonCode: "retention-elapsed"}); !errors.Is(err, ErrRetentionPolicyMismatch) {
		t.Fatalf("ClaimTombstonedFileCleanupWithPolicyAndAudit(mismatch) error = %v, want ErrRetentionPolicyMismatch", err)
	}
	claimed, err := store.ClaimTombstonedFileCleanupWithPolicyAndAudit(record.ID, MaintenanceRetentionPolicy{
		Mode: capability.AdminDeletionTombstone, PolicyVersion: 1, RetentionDays: 1, AutoPurge: true,
	}, AuditEvent{
		Actor: "retention-runner", Action: "file.cleanup.claim", Result: "success", ReasonCode: "retention-elapsed",
	})
	if err != nil {
		t.Fatal(err)
	}
	if state := fileDeletionState(claimed.Record); state != fileDeletionCleanupStarted {
		t.Fatalf("claimed state = %q, want %q", state, fileDeletionCleanupStarted)
	}
	if _, err := store.RestoreWithAudit("files", record.ID, AuditEvent{
		Actor: "file-admin", Action: "file.restore", Result: "success", ReasonCode: "approved",
	}); !errors.Is(err, ErrDeletionCleanupStarted) {
		t.Fatalf("RestoreWithAudit(after claim) error = %v, want ErrDeletionCleanupStarted", err)
	}
	repeatedClaim, err := store.ClaimTombstonedFileCleanupWithAudit(record.ID, AuditEvent{
		Actor: "retention-runner", Action: "file.cleanup.claim", Result: "success", ReasonCode: "retry",
	})
	if err != nil || fileDeletionState(repeatedClaim.Record) != fileDeletionCleanupStarted {
		t.Fatalf("repeated claim = %+v, %v", repeatedClaim, err)
	}

	completed, err := store.CompleteTombstonedFileCleanupWithAudit(record.ID, AuditEvent{
		Actor: "retention-runner", Action: "file.cleanup.complete", Result: "success", ReasonCode: "object-deleted",
	})
	if err != nil {
		t.Fatal(err)
	}
	if state := fileDeletionState(completed.Record); state != fileDeletionObjectDeleted {
		t.Fatalf("completed state = %q, want %q", state, fileDeletionObjectDeleted)
	}
	repeatedComplete, err := store.CompleteTombstonedFileCleanupWithAudit(record.ID, AuditEvent{
		Actor: "retention-runner", Action: "file.cleanup.complete", Result: "success", ReasonCode: "retry",
	})
	if err != nil || fileDeletionState(repeatedComplete.Record) != fileDeletionObjectDeleted {
		t.Fatalf("repeated complete = %+v, %v", repeatedComplete, err)
	}
	if _, err := store.PurgeTombstonedFileWithPolicyAndAudit(record.ID, MaintenanceRetentionPolicy{
		Mode: capability.AdminDeletionTombstone, PolicyVersion: 1, RetentionDays: 1, AutoPurge: true,
	}, AuditEvent{
		Actor: "retention-runner", Action: "file.purge", Result: "success", ReasonCode: "retention-elapsed",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.InternalRecord("files", record.ID); !errors.Is(err, ErrRecordNotFound) {
		t.Fatalf("InternalRecord(purged file) error = %v, want ErrRecordNotFound", err)
	}
}

func TestRevokedRetentionPurgeOnlyAcceptsAuthoritativeTerminalRecords(t *testing.T) {
	now := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	store := lifecycleRevokedTestStore()
	store.now = func() time.Time { return now }
	active, err := store.CreateInternal("api-tokens", WriteInput{
		Code: "active", Name: "Active", Status: "active", Values: map[string]string{"revokedAt": "2026-07-01T00:00:00Z"},
	})
	if err != nil {
		t.Fatal(err)
	}
	revoked, err := store.CreateInternal("api-tokens", WriteInput{
		Code: "revoked", Name: "Revoked", Status: "revoked", Values: map[string]string{"revokedAt": "2026-07-01T00:00:00Z"},
	})
	if err != nil {
		t.Fatal(err)
	}
	policy := MaintenanceRetentionPolicy{Mode: capability.AdminDeletionRevoke, PolicyVersion: 1, RetentionDays: 7, AutoPurge: true}
	event := AuditEvent{Actor: "retention-runner", Action: "admin_resource.purge", Result: "success", ReasonCode: "retention-elapsed"}
	if _, err := store.PurgeRevokedWithPolicyAndAudit("api-tokens", active.ID, "revokedAt", policy, event); !errors.Is(err, ErrRecordNotDeleted) {
		t.Fatalf("PurgeRevokedWithPolicyAndAudit(active) error = %v, want ErrRecordNotDeleted", err)
	}
	if _, err := store.PurgeRevokedWithPolicyAndAudit("api-tokens", revoked.ID, "revokedAt", policy, event); err != nil {
		t.Fatal(err)
	}
	if _, err := store.InternalRecord("api-tokens", revoked.ID); !errors.Is(err, ErrRecordNotFound) {
		t.Fatalf("InternalRecord(revoked token) error = %v, want ErrRecordNotFound", err)
	}
	if _, err := store.InternalRecord("api-tokens", active.ID); err != nil {
		t.Fatalf("active token was purged: %v", err)
	}
}

func TestMaintenanceRetentionPolicyRejectsModeDriftAndOversizedRetention(t *testing.T) {
	now := time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC)
	store := lifecycleRevokedTestStore()
	store.now = func() time.Time { return now }
	revoked, err := store.CreateInternal("api-tokens", WriteInput{
		Code: "revoked", Name: "Revoked", Status: "revoked", Values: map[string]string{"revokedAt": "2026-07-01T00:00:00Z"},
	})
	if err != nil {
		t.Fatal(err)
	}
	event := AuditEvent{Actor: "retention-runner", Action: "admin_resource.purge", Result: "success", ReasonCode: "retention-elapsed"}
	for _, policy := range []MaintenanceRetentionPolicy{
		{Mode: capability.AdminDeletionSoftDelete, PolicyVersion: 1, RetentionDays: 7, AutoPurge: true},
		{Mode: capability.AdminDeletionRevoke, PolicyVersion: 1, RetentionDays: capability.MaximumAdminRetentionDays + 1, AutoPurge: true},
	} {
		if _, err := store.PurgeRevokedWithPolicyAndAudit("api-tokens", revoked.ID, "revokedAt", policy, event); !errors.Is(err, ErrRetentionPolicyMismatch) {
			t.Fatalf("PurgeRevokedWithPolicyAndAudit(%+v) error = %v, want ErrRetentionPolicyMismatch", policy, err)
		}
	}
}

func TestDeletionPolicyFailsClosedAndRestrictsReferences(t *testing.T) {
	store := lifecycleReferenceTestStore()
	parent, err := store.Create("parents", WriteInput{Code: "parent-1", Name: "Parent"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create("children", WriteInput{Code: "child-1", Name: "Child", Values: map[string]string{"parentCode": parent.Code}}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DeleteWithAudit("parents", parent.ID, AuditEvent{Actor: "admin", Action: "admin_resource.delete", Result: "success", ReasonCode: "deleted"}); !errors.Is(err, ErrRecordReferenced) {
		t.Fatalf("DeleteWithAudit(referenced) error = %v, want ErrRecordReferenced", err)
	}
	child := store.resources["children"][0]
	child.DeletedAt = time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC).Format(time.RFC3339)
	child.PurgeAfter = time.Date(2026, 7, 21, 9, 0, 0, 0, time.UTC).Format(time.RFC3339)
	store.resources["children"][0] = child
	if _, err := store.DeleteWithAudit("parents", parent.ID, AuditEvent{Actor: "admin", Action: "admin_resource.delete", Result: "success", ReasonCode: "deleted"}); !errors.Is(err, ErrRecordReferenced) {
		t.Fatalf("DeleteWithAudit(recoverable reference) error = %v, want ErrRecordReferenced", err)
	}

	missingPolicy := lifecycleTestManifests(capability.AdminResourceDeletionPolicy{Mode: capability.AdminDeletionSoftDelete, PolicyVersion: 1})
	missingPolicy[0].Admin.Resources[0].Deletion = nil
	missingStore := NewStoreFromCapabilities(missingPolicy)
	record, err := missingStore.Create("lifecycle-records", WriteInput{Code: "record-1", Name: "Record 1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := missingStore.DeleteWithAudit("lifecycle-records", record.ID, AuditEvent{Actor: "admin", Action: "admin_resource.delete", Result: "success", ReasonCode: "deleted"}); !errors.Is(err, ErrDeletionPolicyMissing) {
		t.Fatalf("DeleteWithAudit(missing policy) error = %v, want ErrDeletionPolicyMissing", err)
	}
}

func TestDeletedAuthorizationRecordsAreInactive(t *testing.T) {
	newStore := func() *Store {
		policy := capability.AdminResourceDeletionPolicy{Mode: capability.AdminDeletionSoftDelete, PolicyVersion: 1, RetentionDays: 7}
		return NewStoreFromCapabilities([]capability.Manifest{{ID: "lifecycle-auth", Admin: capability.AdminSurface{Resources: []capability.AdminResource{
			lifecycleTestResource("users", "admin:user", policy, nil),
			lifecycleTestResource("roles", "admin:role", policy, nil),
			lifecycleTestResource("menus", "admin:menu", policy, nil),
			lifecycleTestResource("audit-logs", "admin:audit-log", capability.AdminResourceDeletionPolicy{Mode: capability.AdminDeletionAppendOnly, PolicyVersion: 1}, nil),
		}}}})
	}
	seed := func(t *testing.T, store *Store) (Record, Record, Record) {
		t.Helper()
		role, err := store.Create("roles", WriteInput{Code: "lifecycle-operator", Name: "Lifecycle Operator", Status: "enabled", Values: map[string]string{"permissions": "admin:tenant:read", "dataScope": "all"}})
		if err != nil {
			t.Fatal(err)
		}
		user, err := store.Create("users", WriteInput{Code: "lifecycle-ops", Name: "Lifecycle Ops", Status: "enabled", Values: map[string]string{"tenantCode": "platform", "roles": "lifecycle-operator"}})
		if err != nil {
			t.Fatal(err)
		}
		menu, err := store.Create("menus", WriteInput{Code: "tenants", Name: "Tenants", Status: "enabled", Values: map[string]string{
			"route": "/tenants", "permission": "admin:tenant:read", "group": "foundation", "icon": "tenants", "titleZh": "租户", "titleEn": "Tenants",
		}})
		if err != nil {
			t.Fatal(err)
		}
		return user, role, menu
	}

	t.Run("deleted user", func(t *testing.T) {
		store := newStore()
		user, _, _ := seed(t, store)
		if _, err := store.DeleteWithAudit("users", user.ID, AuditEvent{Actor: "admin", Action: "admin_resource.delete", Result: "success", ReasonCode: "deleted"}); err != nil {
			t.Fatal(err)
		}
		if _, err := ValidateAdminPrincipal(store, "lifecycle-ops"); !errors.Is(err, ErrAdminPrincipalInvalid) {
			t.Fatalf("ValidateAdminPrincipal(deleted user) error = %v", err)
		}
		authorizer, err := store.CasbinAuthorizer()
		if err != nil {
			t.Fatal(err)
		}
		if authorizer.Can("lifecycle-ops", "platform", "admin:tenant:read", "read") {
			t.Fatal("deleted user remains in Casbin role bindings")
		}
	})

	t.Run("deleted role and menu", func(t *testing.T) {
		store := newStore()
		_, role, menu := seed(t, store)
		if _, err := store.DeleteWithAudit("roles", role.ID, AuditEvent{Actor: "admin", Action: "admin_resource.delete", Result: "success", ReasonCode: "deleted"}); err != nil {
			t.Fatal(err)
		}
		principal := store.CurrentPrincipal("lifecycle-ops")
		if len(principal.Permissions) != 0 {
			t.Fatalf("deleted role permissions remain active: %+v", principal)
		}
		if _, err := store.DeleteWithAudit("menus", menu.ID, AuditEvent{Actor: "admin", Action: "admin_resource.delete", Result: "success", ReasonCode: "deleted"}); err != nil {
			t.Fatal(err)
		}
		if items := store.MenuItemsForPrincipal(rbac.Principal{Permissions: []string{"admin:tenant:read"}}); len(items) != 0 {
			t.Fatalf("deleted menu remains visible: %+v", items)
		}
	})
}

func lifecycleTestStore(policy capability.AdminResourceDeletionPolicy) *Store {
	return NewStoreFromCapabilities(lifecycleTestManifests(policy))
}

func lifecycleTestManifests(policy capability.AdminResourceDeletionPolicy) []capability.Manifest {
	return []capability.Manifest{{ID: "lifecycle-test", Admin: capability.AdminSurface{Resources: []capability.AdminResource{
		lifecycleTestResource("lifecycle-records", "admin:lifecycle-record", policy, nil),
		lifecycleTestResource("audit-logs", "admin:audit-log", capability.AdminResourceDeletionPolicy{Mode: capability.AdminDeletionAppendOnly, PolicyVersion: 1}, nil),
	}}}}
}

func lifecycleReferenceTestStore() *Store {
	softRestricted := capability.AdminResourceDeletionPolicy{Mode: capability.AdminDeletionSoftDelete, PolicyVersion: 1, RestrictReferences: true}
	return NewStoreFromCapabilities([]capability.Manifest{{ID: "lifecycle-test", Admin: capability.AdminSurface{Resources: []capability.AdminResource{
		lifecycleTestResource("parents", "admin:parent", softRestricted, nil),
		lifecycleTestResource("children", "admin:child", capability.AdminResourceDeletionPolicy{Mode: capability.AdminDeletionSoftDelete, PolicyVersion: 1}, []capability.AdminField{{
			Key: "parentCode", Label: capability.Text("上级", "Parent"), Type: "select", Source: "values",
			Relation: &capability.AdminFieldRelation{Resource: "parents", ValueField: "code", LabelField: "name"},
		}}),
		lifecycleTestResource("audit-logs", "admin:audit-log", capability.AdminResourceDeletionPolicy{Mode: capability.AdminDeletionAppendOnly, PolicyVersion: 1}, nil),
	}}}})
}

func lifecycleFileTestStore() *Store {
	policy := capability.AdminResourceDeletionPolicy{
		Mode: capability.AdminDeletionTombstone, PolicyVersion: 1, RetentionDays: 1, AutoPurge: true,
	}
	fields := []capability.AdminField{
		{Key: "storageKey", Label: capability.Text("对象键", "Object Key"), Type: "text", Source: "values", ReadOnly: true, ResponseMode: capability.FieldProjectionOmitted},
		{Key: fileDeletionStateField, Label: capability.Text("删除状态", "Deletion State"), Type: "text", Source: "values", ReadOnly: true},
		{Key: fileDeletionRequestedAtField, Label: capability.Text("删除请求时间", "Deletion Requested At"), Type: "datetime", Source: "values", ReadOnly: true},
	}
	return NewStoreFromCapabilities([]capability.Manifest{{ID: "lifecycle-file-test", Admin: capability.AdminSurface{Resources: []capability.AdminResource{
		lifecycleTestResource("files", "admin:file", policy, fields),
		lifecycleTestResource("audit-logs", "admin:audit-log", capability.AdminResourceDeletionPolicy{Mode: capability.AdminDeletionAppendOnly, PolicyVersion: 1}, nil),
	}}}})
}

func lifecycleRevokedTestStore() *Store {
	policy := capability.AdminResourceDeletionPolicy{
		Mode: capability.AdminDeletionRevoke, PolicyVersion: 1, RetentionDays: 7, AutoPurge: true,
	}
	fields := []capability.AdminField{
		{Key: "revokedAt", Label: capability.Text("撤销时间", "Revoked At"), Type: "datetime", Source: "values", ReadOnly: true},
	}
	return NewStoreFromCapabilities([]capability.Manifest{{ID: "lifecycle-revoked-test", Admin: capability.AdminSurface{Resources: []capability.AdminResource{
		lifecycleTestResource("api-tokens", "admin:api-token", policy, fields),
		lifecycleTestResource("audit-logs", "admin:audit-log", capability.AdminResourceDeletionPolicy{Mode: capability.AdminDeletionAppendOnly, PolicyVersion: 1}, nil),
	}}}})
}

func lifecycleTestResource(resource string, prefix string, policy capability.AdminResourceDeletionPolicy, fields []capability.AdminField) capability.AdminResource {
	if fields != nil {
		fields = append([]capability.AdminField{
			{Key: "code", Label: capability.Text("编码", "Code"), Type: "text", Source: "record", InTable: true, InForm: true, InDetail: true},
			{Key: "name", Label: capability.Text("名称", "Name"), Type: "text", Source: "record", Required: true, InTable: true, InForm: true, InDetail: true},
			{Key: "status", Label: capability.Text("状态", "Status"), Type: "text", Source: "record", InTable: true, InForm: true, InDetail: true},
			{Key: "updatedAt", Label: capability.Text("更新时间", "Updated At"), Type: "datetime", Source: "record", ReadOnly: true, InDetail: true},
		}, fields...)
	}
	return capability.AdminResource{
		Resource: resource, Title: capability.Text("测试", "Test"), Description: capability.Text("测试资源。", "Test resource."),
		PermissionPrefix: prefix, Deletion: &policy, Menu: capability.AdminMenu{Route: "/" + resource, Group: "test", Icon: "test", Order: 1},
		Fields: fields,
	}
}
