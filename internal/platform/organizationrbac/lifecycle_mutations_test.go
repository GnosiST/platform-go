package organizationrbac

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestUserLifecycleDeleteRestoreAndPurgeRequiresReviewedRemediation(t *testing.T) {
	db, repository := prepareOrganizationRBACTestRepository(t)
	seedOrganizationRBAC(t, db)
	now := time.Date(2026, 7, 15, 16, 0, 0, 0, time.UTC)
	if err := db.Create(&gormUser{ID: "user-alice", Code: "alice", ScopeType: string(ScopeTenant), TenantCode: "acme", OrgUnitCode: "acme-hq", Status: StatusEnabled}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&gormUserRole{UserID: "user-alice", RoleCode: "operator"}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repository.ReplaceOrgUnitRoleGroups(context.Background(), ReplaceOrgUnitRoleGroupsRequest{
		OrgUnitCode: "acme-hq", RoleGroupCodes: []string{"acme-ops"}, ExpectedRevision: 0, ActorID: "admin", ChangedAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	impact, err := repository.PreviewResourceLifecycle(context.Background(), "users", "alice", LifecycleOperationDelete, now)
	if err != nil || impact.ExpectedRevision != 1 || len(impact.Conflicts) != 1 || impact.ReferenceCount != 1 {
		t.Fatalf("delete impact = %+v, error = %v", impact, err)
	}
	request := ResourceLifecycleRequest{
		Resource: "users", ResourceCode: "alice", Operation: LifecycleOperationDelete,
		RetentionDays: 30, PolicyVersion: 1, ExpectedRevision: 1, ActorID: "admin", ChangedAt: now,
	}
	if _, err := repository.ApplyResourceLifecycle(context.Background(), request, nil); !errors.Is(err, ErrRolePoolViolation) {
		t.Fatalf("delete without remediation error = %v, want ErrRolePoolViolation", err)
	}
	revision, err := repository.ApplyResourceLifecycle(context.Background(), request, []RoleAssignmentRemediation{{
		UserCode: "alice", RoleCode: "operator", Action: "remove-role",
	}})
	if err != nil || revision != 2 {
		t.Fatalf("delete revision = %d, error = %v", revision, err)
	}
	var lifecycle gormResourceLifecycle
	if err := db.Where("resource = ? AND record_id = ?", "users", "user-alice").Take(&lifecycle).Error; err != nil || lifecycle.PurgeAfter != now.Add(30*24*time.Hour).Format(time.RFC3339) {
		t.Fatalf("lifecycle = %+v, error = %v", lifecycle, err)
	}
	var assignments int64
	if err := db.Model(&gormUserRole{}).Where("user_id = ?", "user-alice").Count(&assignments).Error; err != nil || assignments != 0 {
		t.Fatalf("user-role assignments = %d, error = %v", assignments, err)
	}

	restoreImpact, err := repository.PreviewResourceLifecycle(context.Background(), "users", "alice", LifecycleOperationRestore, now)
	if err != nil || restoreImpact.ExpectedRevision != 2 {
		t.Fatalf("restore impact = %+v, error = %v", restoreImpact, err)
	}
	revision, err = repository.ApplyResourceLifecycle(context.Background(), ResourceLifecycleRequest{
		Resource: "users", ResourceCode: "alice", Operation: LifecycleOperationRestore,
		ExpectedRevision: 2, ActorID: "admin", ChangedAt: now.Add(time.Minute),
	}, nil)
	if err != nil || revision != 3 {
		t.Fatalf("restore revision = %d, error = %v", revision, err)
	}
	var restored gormUser
	if err := db.Where("code = ?", "alice").Take(&restored).Error; err != nil || restored.Status != "disabled" {
		t.Fatalf("restored user = %+v, error = %v", restored, err)
	}
	if deleted, err := isLifecycleDeleted(db, "users", "user-alice"); err != nil || deleted {
		t.Fatalf("restored lifecycle deleted=%t error=%v", deleted, err)
	}
}

func TestRolePurgeRejectsPermissionReferencesUntilRetentionAndReferencesClear(t *testing.T) {
	db, repository := prepareOrganizationRBACTestRepository(t)
	seedOrganizationRBAC(t, db)
	now := time.Date(2026, 7, 15, 17, 0, 0, 0, time.UTC)
	if err := db.Create(&gormResourceLifecycle{
		Resource: "roles", RecordID: "role-operator", DeletedAt: now.Add(-31 * 24 * time.Hour).Format(time.RFC3339),
		DeletedBy: "admin", DeleteReason: LifecycleOperationDelete, PurgeAfter: now.Add(-24 * time.Hour).Format(time.RFC3339), DeletionPolicyVersion: 1,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&gormRolePermission{RoleCode: "operator", Permission: "admin:report:read"}).Error; err != nil {
		t.Fatal(err)
	}
	impact, err := repository.PreviewResourceLifecycle(context.Background(), "roles", "operator", LifecycleOperationPurge, now)
	if err != nil || impact.RetentionElapsed != true || impact.ReferenceCount != 1 {
		t.Fatalf("purge impact = %+v, error = %v", impact, err)
	}
	request := ResourceLifecycleRequest{
		Resource: "roles", ResourceCode: "operator", Operation: LifecycleOperationPurge,
		ExpectedRevision: 0, ActorID: "admin", ChangedAt: now,
	}
	if _, err := repository.ApplyResourceLifecycle(context.Background(), request, nil); !errors.Is(err, ErrRolePoolViolation) {
		t.Fatalf("purge with permission reference error = %v, want ErrRolePoolViolation", err)
	}
	if err := db.Where("role_code = ?", "operator").Delete(&gormRolePermission{}).Error; err != nil {
		t.Fatal(err)
	}
	revision, err := repository.ApplyResourceLifecycle(context.Background(), request, nil)
	if err != nil || revision != 1 {
		t.Fatalf("purge revision = %d, error = %v", revision, err)
	}
	var roles int64
	if err := db.Model(&gormRole{}).Where("code = ?", "operator").Count(&roles).Error; err != nil || roles != 0 {
		t.Fatalf("purged role count = %d, error = %v", roles, err)
	}
}

func TestPurgeRejectsBeforeRetentionElapses(t *testing.T) {
	db, repository := prepareOrganizationRBACTestRepository(t)
	seedOrganizationRBAC(t, db)
	now := time.Date(2026, 7, 15, 17, 30, 0, 0, time.UTC)
	if err := db.Create(&gormResourceLifecycle{
		Resource: "roles", RecordID: "role-disabled", DeletedAt: now.Add(-24 * time.Hour).Format(time.RFC3339),
		DeletedBy: "admin", DeleteReason: LifecycleOperationDelete, PurgeAfter: now.Add(29 * 24 * time.Hour).Format(time.RFC3339), DeletionPolicyVersion: 1,
	}).Error; err != nil {
		t.Fatal(err)
	}
	impact, err := repository.PreviewResourceLifecycle(context.Background(), "roles", "disabled-role", LifecycleOperationPurge, now)
	if err != nil || impact.RetentionElapsed || impact.ReferenceCount != 0 {
		t.Fatalf("purge impact = %+v, error = %v", impact, err)
	}
	if _, err := repository.ApplyResourceLifecycle(context.Background(), ResourceLifecycleRequest{
		Resource: "roles", ResourceCode: "disabled-role", Operation: LifecycleOperationPurge,
		ExpectedRevision: 0, ActorID: "admin", ChangedAt: now,
	}, nil); !errors.Is(err, ErrRolePoolViolation) {
		t.Fatalf("early purge error = %v, want ErrRolePoolViolation", err)
	}
}

func TestOrganizationPurgeClearsBindingRevisionForRecreatedCode(t *testing.T) {
	db, repository := prepareOrganizationRBACTestRepository(t)
	seedOrganizationRBAC(t, db)
	now := time.Date(2026, 7, 15, 17, 40, 0, 0, time.UTC)
	if _, err := repository.ReplaceOrgUnitRoleGroups(context.Background(), ReplaceOrgUnitRoleGroupsRequest{
		OrgUnitCode: "acme-hq", ExpectedRevision: 0, ActorID: "admin", ChangedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&gormOrganization{}).Where("code = ?", "acme-hq").Update("status", "disabled").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&gormResourceLifecycle{
		Resource: "org-units", RecordID: "org-acme-hq", DeletedAt: now.Add(-31 * 24 * time.Hour).Format(time.RFC3339),
		DeletedBy: "admin", DeleteReason: LifecycleOperationDelete, PurgeAfter: now.Add(-24 * time.Hour).Format(time.RFC3339), DeletionPolicyVersion: 1,
	}).Error; err != nil {
		t.Fatal(err)
	}

	impact, err := repository.PreviewResourceLifecycle(context.Background(), "org-units", "acme-hq", LifecycleOperationPurge, now)
	if err != nil || !impact.RetentionElapsed || impact.ReferenceCount != 0 || impact.ExpectedRevision != 1 {
		t.Fatalf("purge impact = %+v, error = %v", impact, err)
	}
	if revision, err := repository.ApplyResourceLifecycle(context.Background(), ResourceLifecycleRequest{
		Resource: "org-units", ResourceCode: "acme-hq", Operation: LifecycleOperationPurge,
		ExpectedRevision: impact.ExpectedRevision, ActorID: "retention-runner", ChangedAt: now,
	}, nil); err != nil || revision != 2 {
		t.Fatalf("purge revision = %d, error = %v", revision, err)
	}

	if err := db.Create(&gormOrganization{ID: "org-acme-hq-recreated", Code: "acme-hq", TenantCode: "acme", Status: StatusEnabled}).Error; err != nil {
		t.Fatal(err)
	}
	bindings, err := repository.ReplaceOrgUnitRoleGroups(context.Background(), ReplaceOrgUnitRoleGroupsRequest{
		OrgUnitCode: "acme-hq", RoleGroupCodes: []string{"acme-audit"}, ExpectedRevision: 0, ActorID: "admin", ChangedAt: now.Add(time.Minute),
	})
	if err != nil || bindings.Revision != 1 {
		t.Fatalf("recreated organization bindings = %+v, error = %v", bindings, err)
	}
}

func TestResourceLifecycleRestoreAlwaysLeavesEntityDisabled(t *testing.T) {
	now := time.Date(2026, 7, 15, 17, 45, 0, 0, time.UTC)
	tests := []struct {
		name        string
		resource    string
		code        string
		recordID    string
		prepare     func(*gorm.DB) error
		statusModel any
	}{
		{
			name: "organization", resource: "org-units", code: "acme-hq", recordID: "org-acme-hq",
			prepare: func(db *gorm.DB) error {
				return db.Model(&gormOrganization{}).Where("code = ?", "acme-hq").Update("status", "disabled").Error
			},
			statusModel: &gormOrganization{},
		},
		{
			name: "role group", resource: "role-groups", code: "acme-empty", recordID: "group-acme-empty",
			prepare: func(db *gorm.DB) error {
				return db.Create(&gormRoleGroup{
					ID: "group-acme-empty", Code: "acme-empty", Name: "Empty", ScopeType: string(ScopeTenant), TenantCode: "acme", Status: "disabled",
				}).Error
			},
			statusModel: &gormRoleGroup{},
		},
		{
			name: "role", resource: "roles", code: "operator", recordID: "role-operator",
			prepare: func(db *gorm.DB) error {
				return db.Model(&gormRole{}).Where("code = ?", "operator").Update("status", "disabled").Error
			},
			statusModel: &gormRole{},
		},
		{
			name: "user", resource: "users", code: "alice", recordID: "user-alice",
			prepare: func(db *gorm.DB) error {
				return db.Create(&gormUser{
					ID: "user-alice", Code: "alice", ScopeType: string(ScopeTenant), TenantCode: "acme", OrgUnitCode: "acme-hq", Status: "disabled",
				}).Error
			},
			statusModel: &gormUser{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, repository := prepareOrganizationRBACTestRepository(t)
			seedOrganizationRBAC(t, db)
			if err := tt.prepare(db); err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&gormResourceLifecycle{
				Resource: tt.resource, RecordID: tt.recordID, DeletedAt: now.Add(-24 * time.Hour).Format(time.RFC3339),
				DeletedBy: "admin", DeleteReason: LifecycleOperationDelete, PurgeAfter: now.Add(29 * 24 * time.Hour).Format(time.RFC3339), DeletionPolicyVersion: 1,
			}).Error; err != nil {
				t.Fatal(err)
			}

			impact, err := repository.PreviewResourceLifecycle(context.Background(), tt.resource, tt.code, LifecycleOperationRestore, now)
			if err != nil || !impact.Deleted || impact.ReferenceCount != 0 || impact.ExpectedRevision != 0 {
				t.Fatalf("restore impact = %+v, error = %v", impact, err)
			}
			revision, err := repository.ApplyResourceLifecycle(context.Background(), ResourceLifecycleRequest{
				Resource: tt.resource, ResourceCode: tt.code, Operation: LifecycleOperationRestore,
				ExpectedRevision: impact.ExpectedRevision, ActorID: "admin", ChangedAt: now,
			}, nil)
			if err != nil || revision != 1 {
				t.Fatalf("restore revision = %d, error = %v", revision, err)
			}
			var restored struct{ Status string }
			if err := db.Model(tt.statusModel).Select("status").Where("code = ?", tt.code).Take(&restored).Error; err != nil || restored.Status != "disabled" {
				t.Fatalf("restored status = %q, error = %v", restored.Status, err)
			}
			if deleted, err := isLifecycleDeleted(db, tt.resource, tt.recordID); err != nil || deleted {
				t.Fatalf("restored lifecycle deleted=%t error=%v", deleted, err)
			}
		})
	}
}

func TestResourceLifecyclePurgeRejectsLiveDomainReferences(t *testing.T) {
	now := time.Date(2026, 7, 15, 17, 50, 0, 0, time.UTC)
	tests := []struct {
		name     string
		resource string
		code     string
		recordID string
		prepare  func(*gorm.DB) error
	}{
		{
			name: "organization binding", resource: "org-units", code: "acme-hq", recordID: "org-acme-hq",
			prepare: func(db *gorm.DB) error {
				return db.Create(&gormOrgUnitRoleGroup{OrgUnitCode: "acme-hq", RoleGroupCode: "acme-ops", Revision: 1, ActorID: "admin", CreatedAt: now, UpdatedAt: now}).Error
			},
		},
		{
			name: "role group role", resource: "role-groups", code: "acme-ops", recordID: "group-acme-ops",
			prepare: func(*gorm.DB) error { return nil },
		},
		{
			name: "role permission", resource: "roles", code: "operator", recordID: "role-operator",
			prepare: func(db *gorm.DB) error {
				return db.Create(&gormRolePermission{RoleCode: "operator", Permission: "admin:report:read"}).Error
			},
		},
		{
			name: "user role", resource: "users", code: "alice", recordID: "user-alice",
			prepare: func(db *gorm.DB) error {
				if err := db.Create(&gormUser{
					ID: "user-alice", Code: "alice", ScopeType: string(ScopeTenant), TenantCode: "acme", OrgUnitCode: "acme-hq", Status: "disabled",
				}).Error; err != nil {
					return err
				}
				return db.Create(&gormUserRole{UserID: "user-alice", RoleCode: "operator"}).Error
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, repository := prepareOrganizationRBACTestRepository(t)
			seedOrganizationRBAC(t, db)
			if err := tt.prepare(db); err != nil {
				t.Fatal(err)
			}
			if tt.resource != "users" {
				var model any
				switch tt.resource {
				case "org-units":
					model = &gormOrganization{}
				case "role-groups":
					model = &gormRoleGroup{}
				case "roles":
					model = &gormRole{}
				}
				if err := db.Model(model).Where("code = ?", tt.code).Update("status", "disabled").Error; err != nil {
					t.Fatal(err)
				}
			}
			if err := db.Create(&gormResourceLifecycle{
				Resource: tt.resource, RecordID: tt.recordID, DeletedAt: now.Add(-31 * 24 * time.Hour).Format(time.RFC3339),
				DeletedBy: "admin", DeleteReason: LifecycleOperationDelete, PurgeAfter: now.Add(-24 * time.Hour).Format(time.RFC3339), DeletionPolicyVersion: 1,
			}).Error; err != nil {
				t.Fatal(err)
			}

			impact, err := repository.PreviewResourceLifecycle(context.Background(), tt.resource, tt.code, LifecycleOperationPurge, now)
			if err != nil || !impact.RetentionElapsed || impact.ReferenceCount == 0 {
				t.Fatalf("purge impact = %+v, error = %v", impact, err)
			}
			if _, err := repository.ApplyResourceLifecycle(context.Background(), ResourceLifecycleRequest{
				Resource: tt.resource, ResourceCode: tt.code, Operation: LifecycleOperationPurge,
				ExpectedRevision: impact.ExpectedRevision, ActorID: "retention-runner", ChangedAt: now,
			}, nil); !errors.Is(err, ErrRolePoolViolation) {
				t.Fatalf("purge error = %v, want ErrRolePoolViolation", err)
			}
			if deleted, err := isLifecycleDeleted(db, tt.resource, tt.recordID); err != nil || !deleted {
				t.Fatalf("rejected purge lifecycle deleted=%t error=%v", deleted, err)
			}
			if revision, err := loadGlobalRevision(db); err != nil || revision != 0 {
				t.Fatalf("rejected purge revision = %d, error = %v", revision, err)
			}
		})
	}
}

func TestRoleGroupLifecycleDisableOnlyReducesAccess(t *testing.T) {
	db, repository := prepareOrganizationRBACTestRepository(t)
	seedOrganizationRBAC(t, db)
	now := time.Date(2026, 7, 15, 18, 0, 0, 0, time.UTC)
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
	impact, err := repository.PreviewResourceLifecycle(context.Background(), "role-groups", "acme-ops", LifecycleOperationDisable, now)
	if err != nil || impact.ReferenceCount == 0 || len(impact.Conflicts) != 1 || impact.ExpectedRevision != 1 {
		t.Fatalf("disable impact = %+v, error = %v", impact, err)
	}
	revision, err := repository.ApplyResourceLifecycle(context.Background(), ResourceLifecycleRequest{
		Resource: "role-groups", ResourceCode: "acme-ops", Operation: LifecycleOperationDisable,
		ExpectedRevision: 1, ActorID: "admin", ChangedAt: now.Add(time.Minute),
	}, []RoleAssignmentRemediation{{UserCode: "alice", RoleCode: "operator", Action: "remove-role"}})
	if err != nil || revision != 2 {
		t.Fatalf("disable revision = %d, error = %v", revision, err)
	}
	var group gormRoleGroup
	if err := db.Where("code = ?", "acme-ops").Take(&group).Error; err != nil || group.Status != "disabled" {
		t.Fatalf("disabled group = %+v, error = %v", group, err)
	}
	var roles int64
	if err := db.Model(&gormRole{}).Where("group_code = ?", "acme-ops").Count(&roles).Error; err != nil || roles == 0 {
		t.Fatalf("role references were removed: count=%d error=%v", roles, err)
	}
	var assignments, bindings int64
	if err := db.Model(&gormUserRole{}).Where("user_id = ?", "user-alice").Count(&assignments).Error; err != nil || assignments != 0 {
		t.Fatalf("user role assignments = %d, error = %v", assignments, err)
	}
	if err := db.Model(&gormOrgUnitRoleGroup{}).Where("role_group_code = ?", "acme-ops").Count(&bindings).Error; err != nil || bindings != 0 {
		t.Fatalf("organization bindings = %d, error = %v", bindings, err)
	}
	if _, err := repository.ValidateCutover(context.Background()); err != nil {
		t.Fatalf("ValidateCutover(after role-group disable) error = %v", err)
	}
}

func TestRoleGroupLifecycleDisableBumpsAffectedOrganizationBindingRevisions(t *testing.T) {
	db, repository := prepareOrganizationRBACTestRepository(t)
	seedOrganizationRBAC(t, db)
	now := time.Date(2026, 7, 15, 18, 15, 0, 0, time.UTC)
	if err := db.Create(&gormOrganization{ID: "org-acme-branch", Code: "acme-branch", TenantCode: "acme", Status: StatusEnabled}).Error; err != nil {
		t.Fatal(err)
	}
	for _, orgUnitCode := range []string{"acme-hq", "acme-branch"} {
		if _, err := repository.ReplaceOrgUnitRoleGroups(context.Background(), ReplaceOrgUnitRoleGroupsRequest{
			OrgUnitCode: orgUnitCode, RoleGroupCodes: []string{"acme-ops"}, ExpectedRevision: 0, ActorID: "admin", ChangedAt: now,
		}); err != nil {
			t.Fatal(err)
		}
	}

	impact, err := repository.PreviewResourceLifecycle(context.Background(), "role-groups", "acme-ops", LifecycleOperationDisable, now)
	if err != nil || impact.ExpectedRevision != 2 {
		t.Fatalf("disable impact = %+v, error = %v", impact, err)
	}
	if revision, err := repository.ApplyResourceLifecycle(context.Background(), ResourceLifecycleRequest{
		Resource: "role-groups", ResourceCode: "acme-ops", Operation: LifecycleOperationDisable,
		ExpectedRevision: impact.ExpectedRevision, ActorID: "admin", ChangedAt: now.Add(time.Minute),
	}, nil); err != nil || revision != 3 {
		t.Fatalf("disable revision = %d, error = %v", revision, err)
	}

	for _, orgUnitCode := range []string{"acme-hq", "acme-branch"} {
		revision, err := loadOrgUnitRoleGroupRevision(db, orgUnitCode)
		if err != nil || revision != 2 {
			t.Fatalf("%s binding revision = %d, error = %v", orgUnitCode, revision, err)
		}
		if _, err := repository.ReplaceOrgUnitRoleGroups(context.Background(), ReplaceOrgUnitRoleGroupsRequest{
			OrgUnitCode: orgUnitCode, RoleGroupCodes: []string{"acme-audit"}, ExpectedRevision: 1, ActorID: "admin", ChangedAt: now.Add(2 * time.Minute),
		}); !errors.Is(err, ErrRevisionConflict) {
			t.Fatalf("%s stale binding update error = %v, want ErrRevisionConflict", orgUnitCode, err)
		}
	}
}

func TestOrganizationAndRoleGroupDeleteRejectLiveReferences(t *testing.T) {
	db, repository := prepareOrganizationRBACTestRepository(t)
	seedOrganizationRBAC(t, db)
	now := time.Date(2026, 7, 15, 18, 30, 0, 0, time.UTC)
	if err := db.Create(&gormUser{ID: "user-alice", Code: "alice", ScopeType: string(ScopeTenant), TenantCode: "acme", OrgUnitCode: "acme-hq", Status: StatusEnabled}).Error; err != nil {
		t.Fatal(err)
	}

	for _, target := range []struct {
		resource string
		code     string
	}{
		{resource: "org-units", code: "acme-hq"},
		{resource: "role-groups", code: "acme-ops"},
	} {
		impact, err := repository.PreviewResourceLifecycle(context.Background(), target.resource, target.code, LifecycleOperationDelete, now)
		if err != nil || impact.ReferenceCount == 0 {
			t.Fatalf("%s delete impact = %+v, error = %v", target.resource, impact, err)
		}
		if _, err := repository.ApplyResourceLifecycle(context.Background(), ResourceLifecycleRequest{
			Resource: target.resource, ResourceCode: target.code, Operation: LifecycleOperationDelete,
			RetentionDays: 30, PolicyVersion: 1, ExpectedRevision: impact.ExpectedRevision, ActorID: "admin", ChangedAt: now,
		}, nil); !errors.Is(err, ErrRolePoolViolation) {
			t.Fatalf("%s delete error = %v, want ErrRolePoolViolation", target.resource, err)
		}
		var lifecycle int64
		if err := db.Model(&gormResourceLifecycle{}).Where("resource = ? AND record_id = ?", target.resource, impact.RecordID).Count(&lifecycle).Error; err != nil || lifecycle != 0 {
			t.Fatalf("%s lifecycle count = %d, error = %v", target.resource, lifecycle, err)
		}
	}
}
