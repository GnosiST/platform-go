package organizationrbac

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"platform-go/internal/platform/kernel"
	"platform-go/internal/platform/serviceobject"

	"gorm.io/gorm"
)

func TestNavigationServiceObjectDefinitionsAreTypedAndPermissionSeparated(t *testing.T) {
	queryCases := []struct {
		id                   string
		permission           string
		action               string
		additionalPermission []serviceobject.PermissionRequirement
		arguments            []serviceobject.ArgumentDefinition
		result               []serviceobject.ResultField
	}{
		{
			id: MenuDefinitionGetQueryID, permission: "admin:menu:read", action: "read",
			arguments: []serviceobject.ArgumentDefinition{{Name: "menuCode", Type: serviceobject.ValueString, Required: true, MaxLength: 191}},
			result:    []serviceobject.ResultField{{Name: "definition", Type: serviceobject.ValueMenuDefinition}, {Name: "revision", Type: serviceobject.ValueInteger}},
		},
		{
			id: RoleMenusGetQueryID, permission: "admin:role:read", action: "read",
			additionalPermission: []serviceobject.PermissionRequirement{{Permission: "admin:menu:read", Action: "read"}},
			arguments:            []serviceobject.ArgumentDefinition{{Name: "roleCode", Type: serviceobject.ValueString, Required: true, MaxLength: 191}},
			result: []serviceobject.ResultField{
				{Name: "roleCode", Type: serviceobject.ValueString}, {Name: "menuCodes", Type: serviceobject.ValueStringSet}, {Name: "revision", Type: serviceobject.ValueInteger},
			},
		},
		{
			id: RoleMenuImpactQueryID, permission: "admin:role:update", action: "update",
			additionalPermission: []serviceobject.PermissionRequirement{{Permission: "admin:menu:read", Action: "read"}},
			arguments:            []serviceobject.ArgumentDefinition{{Name: "previewId", Type: serviceobject.ValueString, Required: true, MaxLength: 64}},
			result: []serviceobject.ResultField{
				{Name: "previewId", Type: serviceobject.ValueString}, {Name: "changed", Type: serviceobject.ValueBoolean},
				{Name: "currentMenuCodes", Type: serviceobject.ValueStringSet}, {Name: "proposedMenuCodes", Type: serviceobject.ValueStringSet},
				{Name: "expectedRevision", Type: serviceobject.ValueInteger}, {Name: "impactHash", Type: serviceobject.ValueString},
				{Name: "expiresAt", Type: serviceobject.ValueString},
			},
		},
		{
			id: RoleMenuMigrationCompareQueryID, permission: "admin:role:read", action: "read",
			additionalPermission: []serviceobject.PermissionRequirement{{Permission: "admin:menu:read", Action: "read"}},
			arguments:            []serviceobject.ArgumentDefinition{{Name: "roleCode", Type: serviceobject.ValueString, Required: true, MaxLength: 191}},
			result: []serviceobject.ResultField{
				{Name: "roleCode", Type: serviceobject.ValueString}, {Name: "legacyMenuCodes", Type: serviceobject.ValueStringSet},
				{Name: "targetMenuCodes", Type: serviceobject.ValueStringSet}, {Name: "addedMenuCodes", Type: serviceobject.ValueStringSet},
				{Name: "removedMenuCodes", Type: serviceobject.ValueStringSet}, {Name: "targetRevision", Type: serviceobject.ValueInteger},
				{Name: "principalEquivalenceClaimed", Type: serviceobject.ValueBoolean},
			},
		},
	}
	for _, testCase := range queryCases {
		definition := queryDefinitionByID(t, testCase.id)
		if definition.Version != ServiceObjectVersion || definition.Resource != navigationResource || definition.Permission != testCase.permission || definition.Action != testCase.action ||
			definition.TenantMode != serviceobject.TenantPlatform || definition.DataScope != "platform" ||
			!reflect.DeepEqual(definition.AdditionalPermissions, testCase.additionalPermission) || !reflect.DeepEqual(definition.Arguments, testCase.arguments) || !reflect.DeepEqual(definition.ResultSchema, testCase.result) {
			t.Fatalf("query %s = %+v", testCase.id, definition)
		}
	}

	commandCases := []struct {
		id                   string
		permission           string
		action               string
		additionalPermission []serviceobject.PermissionRequirement
		arguments            []serviceobject.ArgumentDefinition
		result               []serviceobject.ResultField
	}{
		{
			id: MenuDefinitionCreateCommandID, permission: "admin:menu:create", action: "create",
			arguments: menuDefinitionMutationArguments(), result: menuDefinitionMutationResultSchema(),
		},
		{
			id: MenuDefinitionReplaceCommandID, permission: "admin:menu:update", action: "update",
			arguments: menuDefinitionMutationArguments(), result: menuDefinitionMutationResultSchema(),
		},
		{
			id: RoleMenuPrepareCommandID, permission: "admin:role:update", action: "update",
			additionalPermission: []serviceobject.PermissionRequirement{{Permission: "admin:menu:read", Action: "read"}},
			arguments: []serviceobject.ArgumentDefinition{
				{Name: "roleCode", Type: serviceobject.ValueString, Required: true, MaxLength: 191},
				{Name: "menuCodes", Type: serviceobject.ValueStringSet, Required: true, MaxLength: 191},
			},
			result: previewResultSchema(),
		},
		{
			id: RoleMenusReplaceCommandID, permission: "admin:role:update", action: "update",
			additionalPermission: []serviceobject.PermissionRequirement{{Permission: "admin:menu:read", Action: "read"}},
			arguments: []serviceobject.ArgumentDefinition{
				{Name: "previewId", Type: serviceobject.ValueString, Required: true, MaxLength: 64},
				{Name: "expectedRevision", Type: serviceobject.ValueInteger, Required: true},
				{Name: "impactHash", Type: serviceobject.ValueString, Required: true, MaxLength: 64},
			},
			result: []serviceobject.ResultField{
				{Name: "applied", Type: serviceobject.ValueBoolean}, {Name: "revision", Type: serviceobject.ValueInteger}, {Name: "previewId", Type: serviceobject.ValueString},
			},
		},
	}
	for _, testCase := range commandCases {
		definition := domainDefinitionByID(t, testCase.id)
		if definition.Version != ServiceObjectVersion || definition.Resource != navigationResource || definition.Permission != testCase.permission || definition.Action != testCase.action ||
			definition.TenantMode != serviceobject.TenantPlatform || definition.DataScope != "platform" || definition.Idempotency != serviceobject.IdempotencyRequiredKey ||
			definition.MaxAffectedRows != 2000 || !reflect.DeepEqual(definition.AdditionalPermissions, testCase.additionalPermission) ||
			!reflect.DeepEqual(definition.Arguments, testCase.arguments) || !reflect.DeepEqual(definition.ResultSchema, testCase.result) {
			t.Fatalf("domain command %s = %+v", testCase.id, definition)
		}
	}
}

func TestMenuDefinitionServiceObjectsCreateReplaceReadAndReplayTypedDefinitions(t *testing.T) {
	db, repository := prepareOrganizationRBACTestRepository(t)
	seedOrganizationRBAC(t, db)
	seedNativeMenus(t, db)
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	runtime := newNavigationRuntime(t, repository, ServiceObjectExecutorOptions{}, func() time.Time { return now })
	invocation := navigationInvocation()

	missing := navigationMenuDefinition("missing", "Missing")
	_, err := runtime.ExecuteCommand(invocation, serviceobject.CommandRequest{
		CommandID: MenuDefinitionReplaceCommandID, Version: ServiceObjectVersion, IdempotencyKey: "replace-missing",
		Arguments: map[string]any{"definition": missing, "expectedRevision": int64(0)},
	})
	if !errors.Is(err, serviceobject.ErrObjectUnavailable) {
		t.Fatalf("replace missing error = %v, want ErrObjectUnavailable", err)
	}

	definition := navigationMenuDefinition("settings", "Settings")
	definition.UpdatedAt = "1999-01-01T00:00:00Z"
	createRequest := serviceobject.CommandRequest{
		CommandID: MenuDefinitionCreateCommandID, Version: ServiceObjectVersion, IdempotencyKey: "create-settings",
		Arguments: map[string]any{"definition": definition, "expectedRevision": int64(0)},
	}
	created, err := runtime.ExecuteCommand(invocation, createRequest)
	if err != nil || created.Values["applied"] != true || created.Values["revision"] != int64(1) {
		t.Fatalf("create = %+v, error = %v", created, err)
	}
	createdDefinition, err := runtime.ExecuteQuery(invocation, serviceobject.QueryRequest{
		QueryID: MenuDefinitionGetQueryID, Version: ServiceObjectVersion, Arguments: map[string]any{"menuCode": "settings"},
	})
	if err != nil || createdDefinition.Items[0]["definition"].(serviceobject.MenuDefinition).UpdatedAt != now.Format(time.RFC3339) {
		t.Fatalf("created definition accepted client updatedAt: %+v, error = %v", createdDefinition, err)
	}
	replay, err := runtime.ExecuteCommand(invocation, createRequest)
	if err != nil || !reflect.DeepEqual(replay, created) {
		t.Fatalf("create replay = %+v, error = %v", replay, err)
	}

	existing := navigationMenuDefinition("users", "Existing")
	existing.ID = "menu-users"
	_, err = runtime.ExecuteCommand(invocation, serviceobject.CommandRequest{
		CommandID: MenuDefinitionCreateCommandID, Version: ServiceObjectVersion, IdempotencyKey: "create-existing",
		Arguments: map[string]any{"definition": existing, "expectedRevision": int64(1)},
	})
	if !errors.Is(err, serviceobject.ErrConflict) {
		t.Fatalf("create existing error = %v, want ErrConflict", err)
	}
	existingID := navigationMenuDefinition("new-code", "Existing ID")
	existingID.ID = "menu-users"
	_, err = runtime.ExecuteCommand(invocation, serviceobject.CommandRequest{
		CommandID: MenuDefinitionCreateCommandID, Version: ServiceObjectVersion, IdempotencyKey: "create-existing-id",
		Arguments: map[string]any{"definition": existingID, "expectedRevision": int64(1)},
	})
	if !errors.Is(err, serviceobject.ErrConflict) {
		t.Fatalf("create existing ID error = %v, want ErrConflict", err)
	}

	definition.Node.TitleEN = "Platform settings"
	definition.UpdatedAt = "2000-01-01T00:00:00Z"
	replaced, err := runtime.ExecuteCommand(invocation, serviceobject.CommandRequest{
		CommandID: MenuDefinitionReplaceCommandID, Version: ServiceObjectVersion, IdempotencyKey: "replace-settings",
		Arguments: map[string]any{"definition": definition, "expectedRevision": int64(1)},
	})
	if err != nil || replaced.Values["applied"] != true || replaced.Values["revision"] != int64(2) {
		t.Fatalf("replace = %+v, error = %v", replaced, err)
	}
	definition.UpdatedAt = now.Format(time.RFC3339)
	loaded, err := runtime.ExecuteQuery(invocation, serviceobject.QueryRequest{
		QueryID: MenuDefinitionGetQueryID, Version: ServiceObjectVersion, Arguments: map[string]any{"menuCode": "settings"},
	})
	if err != nil || len(loaded.Items) != 1 || loaded.Items[0]["revision"] != int64(2) || !reflect.DeepEqual(loaded.Items[0]["definition"], definition) {
		t.Fatalf("menu get = %+v, error = %v", loaded, err)
	}
}

func TestRoleMenuWriteGateDefaultsClosedWhileReadAndCompareRemainAvailable(t *testing.T) {
	db, repository := prepareOrganizationRBACTestRepository(t)
	seedOrganizationRBAC(t, db)
	seedNativeMenus(t, db)
	runtime := newNavigationRuntime(t, repository, ServiceObjectExecutorOptions{}, nil)
	invocation := navigationInvocation()

	roleMenus, err := runtime.ExecuteQuery(invocation, serviceobject.QueryRequest{
		QueryID: RoleMenusGetQueryID, Version: ServiceObjectVersion, Arguments: map[string]any{"roleCode": "operator"},
	})
	if err != nil || len(roleMenus.Items) != 1 || roleMenus.Items[0]["roleCode"] != "operator" ||
		!reflect.DeepEqual(roleMenus.Items[0]["menuCodes"], []string{}) || roleMenus.Items[0]["revision"] != int64(0) {
		t.Fatalf("role menus = %+v, error = %v", roleMenus, err)
	}
	comparison, err := runtime.ExecuteQuery(invocation, serviceobject.QueryRequest{
		QueryID: RoleMenuMigrationCompareQueryID, Version: ServiceObjectVersion, Arguments: map[string]any{"roleCode": "operator"},
	})
	if err != nil || len(comparison.Items) != 1 || comparison.Items[0]["roleCode"] != "operator" || comparison.Items[0]["principalEquivalenceClaimed"] != false {
		t.Fatalf("comparison = %+v, error = %v", comparison, err)
	}

	_, err = runtime.ExecuteCommand(invocation, serviceobject.CommandRequest{
		CommandID: RoleMenuPrepareCommandID, Version: ServiceObjectVersion, IdempotencyKey: "closed-prepare",
		Arguments: map[string]any{"roleCode": "operator", "menuCodes": []string{"users"}},
	})
	if !errors.Is(err, serviceobject.ErrObjectUnavailable) {
		t.Fatalf("closed prepare error = %v, want ErrObjectUnavailable", err)
	}
	_, err = runtime.ExecuteCommand(invocation, serviceobject.CommandRequest{
		CommandID: RoleMenusReplaceCommandID, Version: ServiceObjectVersion, IdempotencyKey: "closed-apply",
		Arguments: map[string]any{"previewId": "0123456789abcdef", "expectedRevision": int64(0), "impactHash": strings.Repeat("a", 64)},
	})
	if !errors.Is(err, serviceobject.ErrObjectUnavailable) {
		t.Fatalf("closed apply error = %v, want ErrObjectUnavailable", err)
	}
}

func TestRoleMenuPrepareImpactApplyReplacesCompleteSetAndReplays(t *testing.T) {
	db, repository := prepareOrganizationRBACTestRepository(t)
	seedOrganizationRBAC(t, db)
	seedNativeMenus(t, db)
	now := time.Date(2026, 7, 15, 14, 0, 0, 0, time.UTC)
	if _, err := repository.ReplaceRoleMenus(context.Background(), ReplaceRoleMenusRequest{
		RoleCode: "operator", MenuCodes: []string{"users"}, ExpectedRevision: 0, ActorID: "seed", ChangedAt: now.Add(-time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	runtime := newNavigationRuntime(t, repository, ServiceObjectExecutorOptions{RoleMenuWriteEnabled: true}, func() time.Time { return now })
	invocation := navigationInvocation()

	prepared, err := runtime.ExecuteCommand(invocation, serviceobject.CommandRequest{
		CommandID: RoleMenuPrepareCommandID, Version: ServiceObjectVersion, IdempotencyKey: "prepare-role-menus",
		Arguments: map[string]any{"roleCode": "operator", "menuCodes": []string{"reports", "reports"}},
	})
	if err != nil || prepared.Values["expectedRevision"] != int64(1) || prepared.Values["previewId"] == "" || prepared.Values["impactHash"] == "" {
		t.Fatalf("prepare = %+v, error = %v", prepared, err)
	}
	previewID := prepared.Values["previewId"].(string)
	impact, err := runtime.ExecuteQuery(invocation, serviceobject.QueryRequest{
		QueryID: RoleMenuImpactQueryID, Version: ServiceObjectVersion, Arguments: map[string]any{"previewId": previewID},
	})
	if err != nil || len(impact.Items) != 1 || impact.Items[0]["changed"] != true ||
		!reflect.DeepEqual(impact.Items[0]["currentMenuCodes"], []string{"users"}) ||
		!reflect.DeepEqual(impact.Items[0]["proposedMenuCodes"], []string{"reports"}) ||
		impact.Items[0]["expectedRevision"] != int64(1) {
		t.Fatalf("impact = %+v, error = %v", impact, err)
	}

	applyRequest := serviceobject.CommandRequest{
		CommandID: RoleMenusReplaceCommandID, Version: ServiceObjectVersion, IdempotencyKey: "apply-role-menus",
		Arguments: map[string]any{
			"previewId": previewID, "expectedRevision": prepared.Values["expectedRevision"], "impactHash": prepared.Values["impactHash"],
		},
	}
	applied, err := runtime.ExecuteCommand(invocation, applyRequest)
	if err != nil || applied.Values["applied"] != true || applied.Values["revision"] != int64(2) || applied.Values["previewId"] != previewID {
		t.Fatalf("apply = %+v, error = %v", applied, err)
	}
	roleMenus, err := repository.LoadRoleMenus(context.Background(), "operator")
	if err != nil || roleMenus.Revision != 2 || !reflect.DeepEqual(roleMenus.MenuCodes, []string{"reports"}) {
		t.Fatalf("stored role menus = %+v, error = %v", roleMenus, err)
	}
	if replay, err := runtime.ExecuteCommand(invocation, applyRequest); err != nil || !reflect.DeepEqual(replay, applied) {
		t.Fatalf("idempotency replay = %+v, error = %v", replay, err)
	}
	applyRequest.IdempotencyKey = "apply-role-menus-again"
	if replay, err := runtime.ExecuteCommand(invocation, applyRequest); err != nil || !reflect.DeepEqual(replay, applied) {
		t.Fatalf("preview replay = %+v, error = %v", replay, err)
	}
	var audits []gormOrganizationRBACAuditEvent
	if err := db.Where("preview_id = ?", previewID).Find(&audits).Error; err != nil || len(audits) != 1 || audits[0].Action != RoleMenusReplaceCommandID || audits[0].TenantCode != "acme" || audits[0].ConflictCount != 0 {
		t.Fatalf("audits = %+v, error = %v", audits, err)
	}
}

func TestRoleMenuPreviewRejectsCrossOwnerStaleExpiredAndHashMismatch(t *testing.T) {
	t.Run("cross owner and hash mismatch", func(t *testing.T) {
		db, repository := prepareOrganizationRBACTestRepository(t)
		seedOrganizationRBAC(t, db)
		seedNativeMenus(t, db)
		now := time.Date(2026, 7, 15, 15, 0, 0, 0, time.UTC)
		runtime := newNavigationRuntime(t, repository, ServiceObjectExecutorOptions{RoleMenuWriteEnabled: true}, func() time.Time { return now })
		invocation := navigationInvocation()
		prepared := prepareRoleMenuPreview(t, runtime, invocation, "preview-guards", "operator", []string{"users"})
		previewID := prepared.Values["previewId"].(string)

		other := invocation
		other.Execution.Actor.Username = "other-admin"
		if _, err := runtime.ExecuteQuery(other, serviceobject.QueryRequest{
			QueryID: RoleMenuImpactQueryID, Version: ServiceObjectVersion, Arguments: map[string]any{"previewId": previewID},
		}); !errors.Is(err, serviceobject.ErrObjectUnavailable) {
			t.Fatalf("cross-owner impact error = %v, want ErrObjectUnavailable", err)
		}
		if _, err := runtime.ExecuteCommand(invocation, serviceobject.CommandRequest{
			CommandID: RoleMenusReplaceCommandID, Version: ServiceObjectVersion, IdempotencyKey: "wrong-impact-hash",
			Arguments: map[string]any{
				"previewId": previewID, "expectedRevision": prepared.Values["expectedRevision"], "impactHash": strings.Repeat("f", 64),
			},
		}); !errors.Is(err, serviceobject.ErrConflict) {
			t.Fatalf("wrong hash error = %v, want ErrConflict", err)
		}
	})

	t.Run("stale revision", func(t *testing.T) {
		db, repository := prepareOrganizationRBACTestRepository(t)
		seedOrganizationRBAC(t, db)
		seedNativeMenus(t, db)
		now := time.Date(2026, 7, 15, 16, 0, 0, 0, time.UTC)
		runtime := newNavigationRuntime(t, repository, ServiceObjectExecutorOptions{RoleMenuWriteEnabled: true}, func() time.Time { return now })
		invocation := navigationInvocation()
		prepared := prepareRoleMenuPreview(t, runtime, invocation, "stale-preview", "operator", []string{"reports"})
		if _, err := repository.ReplaceRoleMenus(context.Background(), ReplaceRoleMenusRequest{
			RoleCode: "operator", MenuCodes: []string{"users"}, ExpectedRevision: 0, ActorID: "concurrent-admin", ChangedAt: now,
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := runtime.ExecuteCommand(invocation, serviceobject.CommandRequest{
			CommandID: RoleMenusReplaceCommandID, Version: ServiceObjectVersion, IdempotencyKey: "apply-stale-preview",
			Arguments: map[string]any{
				"previewId": prepared.Values["previewId"], "expectedRevision": prepared.Values["expectedRevision"], "impactHash": prepared.Values["impactHash"],
			},
		}); !errors.Is(err, serviceobject.ErrConflict) {
			t.Fatalf("stale apply error = %v, want ErrConflict", err)
		}
		current, err := repository.LoadRoleMenus(context.Background(), "operator")
		if err != nil || current.Revision != 1 || !reflect.DeepEqual(current.MenuCodes, []string{"users"}) {
			t.Fatalf("role menus after stale apply = %+v, error = %v", current, err)
		}
	})

	t.Run("expired preview", func(t *testing.T) {
		db, repository := prepareOrganizationRBACTestRepository(t)
		seedOrganizationRBAC(t, db)
		seedNativeMenus(t, db)
		now := time.Date(2026, 7, 15, 17, 0, 0, 0, time.UTC)
		runtime := newNavigationRuntime(t, repository, ServiceObjectExecutorOptions{RoleMenuWriteEnabled: true}, func() time.Time { return now })
		invocation := navigationInvocation()
		prepared := prepareRoleMenuPreview(t, runtime, invocation, "expiring-preview", "operator", []string{"users"})
		now = now.Add(defaultOrganizationRoleGroupPreviewDuration + time.Second)
		if _, err := runtime.ExecuteQuery(invocation, serviceobject.QueryRequest{
			QueryID: RoleMenuImpactQueryID, Version: ServiceObjectVersion, Arguments: map[string]any{"previewId": prepared.Values["previewId"]},
		}); !errors.Is(err, serviceobject.ErrConflict) {
			t.Fatalf("expired impact error = %v, want ErrConflict", err)
		}
		if _, err := runtime.ExecuteCommand(invocation, serviceobject.CommandRequest{
			CommandID: RoleMenusReplaceCommandID, Version: ServiceObjectVersion, IdempotencyKey: "apply-expired-preview",
			Arguments: map[string]any{
				"previewId": prepared.Values["previewId"], "expectedRevision": prepared.Values["expectedRevision"], "impactHash": prepared.Values["impactHash"],
			},
		}); !errors.Is(err, serviceobject.ErrConflict) {
			t.Fatalf("expired apply error = %v, want ErrConflict", err)
		}
	})
}

func TestRoleMenuPrepareRejectsDisabledRolesAndNonPageOrDisabledMenus(t *testing.T) {
	tests := []struct {
		name      string
		roleCode  string
		menuCode  string
		disableDB func(*testing.T, *gorm.DB)
	}{
		{name: "disabled role", roleCode: "disabled-role", menuCode: "users"},
		{name: "directory", roleCode: "operator", menuCode: "access"},
		{name: "disabled page", roleCode: "operator", menuCode: "reports", disableDB: func(t *testing.T, db *gorm.DB) {
			if err := db.Model(&gormMenu{}).Where("code = ?", "reports").Update("status", "disabled").Error; err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			db, repository := prepareOrganizationRBACTestRepository(t)
			seedOrganizationRBAC(t, db)
			seedNativeMenus(t, db)
			if testCase.disableDB != nil {
				testCase.disableDB(t, db)
			}
			runtime := newNavigationRuntime(t, repository, ServiceObjectExecutorOptions{RoleMenuWriteEnabled: true}, nil)
			_, err := runtime.ExecuteCommand(navigationInvocation(), serviceobject.CommandRequest{
				CommandID: RoleMenuPrepareCommandID, Version: ServiceObjectVersion, IdempotencyKey: "invalid-" + strings.ReplaceAll(testCase.name, " ", "-"),
				Arguments: map[string]any{"roleCode": testCase.roleCode, "menuCodes": []string{testCase.menuCode}},
			})
			if !errors.Is(err, serviceobject.ErrValidation) {
				t.Fatalf("prepare error = %v, want ErrValidation", err)
			}
		})
	}
}

func newNavigationRuntime(t *testing.T, repository *GORMRepository, options ServiceObjectExecutorOptions, now func() time.Time) *serviceobject.Runtime {
	t.Helper()
	executor, err := NewServiceObjectExecutorWithOptions(repository, now, options)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := serviceobject.NewRegistryWithDomainCommands(OrganizationQueryDefinitions(), nil, OrganizationDomainCommandDefinitions())
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := serviceobject.NewRuntimeWithDomainCommands(
		registry,
		serviceobject.AuthorizerFunc(func(context.Context, kernel.ExecutionContext, string, string) bool { return true }),
		executor,
		nil,
		executor,
		serviceobject.NewMemoryIdempotencyStore(),
	)
	if err != nil {
		t.Fatal(err)
	}
	return runtime
}

func prepareRoleMenuPreview(t *testing.T, runtime *serviceobject.Runtime, invocation serviceobject.Invocation, idempotencyKey, roleCode string, menuCodes []string) serviceobject.CommandResult {
	t.Helper()
	prepared, err := runtime.ExecuteCommand(invocation, serviceobject.CommandRequest{
		CommandID: RoleMenuPrepareCommandID, Version: ServiceObjectVersion, IdempotencyKey: idempotencyKey,
		Arguments: map[string]any{"roleCode": roleCode, "menuCodes": menuCodes},
	})
	if err != nil {
		t.Fatalf("prepare role menus error = %v", err)
	}
	return prepared
}

func navigationInvocation() serviceobject.Invocation {
	return serviceobject.Invocation{
		Execution: kernel.ExecutionContext{
			Context: context.Background(), Actor: kernel.Actor{Username: "admin", Kind: kernel.ActorKindUser},
			TenantScope: kernel.TenantScope{PlatformWide: true}, PermissionIntent: kernel.PermissionIntent{Code: "navigation", Action: "execute"},
		},
		Scope: serviceobject.ScopeConstraint{All: true},
	}
}

func navigationMenuDefinition(code, title string) serviceobject.MenuDefinition {
	return serviceobject.MenuDefinition{
		ID: "menu-" + code, Name: title, Buttons: []serviceobject.PageButton{},
		Node: serviceobject.MenuNode{
			Code: code, ParentCode: "access", NodeType: serviceobject.MenuNodeTypePage,
			TitleZH: title, TitleEN: title, Status: StatusEnabled,
			Route: "/" + code, ComponentKey: code, Parameters: []serviceobject.MenuParameter{}, CacheEnabled: true, BreadcrumbVisible: true,
		},
	}
}
