package core

import (
	"context"
	"path/filepath"
	"slices"
	"testing"

	"github.com/GnosiST/platform-go/internal/platform/capability"
)

func TestDefaultManifestsResolve(t *testing.T) {
	registry := capability.NewRegistry()
	for _, manifest := range DefaultManifests() {
		if err := registry.Register(manifest); err != nil {
			t.Fatalf("Register(%q) error = %v", manifest.ID, err)
		}
	}

	enabled := make([]capability.ID, 0, len(DefaultManifests()))
	for _, manifest := range DefaultManifests() {
		enabled = append(enabled, manifest.ID)
	}

	ordered, err := registry.ResolveEnabled(enabled)
	if err != nil {
		t.Fatalf("ResolveEnabled() error = %v", err)
	}
	if len(ordered) != len(DefaultManifests()) {
		t.Fatalf("ResolveEnabled() returned %d manifests, want %d", len(ordered), len(DefaultManifests()))
	}
	if ordered[0].ID != "dictionary" {
		t.Fatalf("first core capability = %q, want dictionary", ordered[0].ID)
	}
}

func TestDefaultManifestsExposeAdminSurface(t *testing.T) {
	manifests := DefaultManifests()
	var tenantFound, orgUnitFound, roleGroupFound, areaCodeFound, dictionaryFound, parameterFound, brandingFound, settingsFound, appPhoneVerificationFound, appPhoneBindingFound, capabilityFound, apiDocsFound, filesFound, sessionFound, loginLogFound, errorLogFound, versionFound, apiTokenFound, policyReviewFound bool
	for _, manifest := range manifests {
		for _, resource := range manifest.Admin.Resources {
			if resource.Resource == "tenants" && resource.Menu.Route == "/tenants" && resource.PermissionPrefix == "admin:tenant" {
				tenantFound = true
			}
			if resource.Resource == "org-units" && resource.Menu.Route == "/org-units" && resource.PermissionPrefix == "admin:org-unit" {
				orgUnitFound = true
			}
			if resource.Resource == "role-groups" && resource.Menu.Route == "/role-groups" && resource.PermissionPrefix == "admin:role-group" {
				roleGroupFound = true
			}
			if resource.Resource == "area-codes" && resource.Menu.Route == "/area-codes" && resource.PermissionPrefix == "admin:area-code" {
				areaCodeFound = true
			}
			if manifest.ID == "dictionary" && resource.Resource == "dictionaries" && resource.Menu.Route == "/dictionaries" && resource.PermissionPrefix == "admin:dictionary" {
				dictionaryFound = true
			}
			if manifest.ID == "parameter" && resource.Resource == "parameters" && resource.Menu.Route == "/parameters" && resource.PermissionPrefix == "admin:parameter" {
				parameterFound = true
			}
			if manifest.ID == "parameter" && resource.Resource == "branding" && resource.Menu.Route == "/branding" && resource.PermissionPrefix == "admin:branding" {
				brandingFound = true
			}
			if manifest.ID == "parameter" && resource.Resource == "settings" && resource.Menu.Route == "/settings" && resource.PermissionPrefix == "admin:settings" {
				settingsFound = true
			}
			if resource.Resource == "app-phone-verifications" && resource.Menu.Route == "/app-phone-verifications" && resource.PermissionPrefix == "admin:app-phone-verification" {
				appPhoneVerificationFound = true
			}
			if resource.Resource == "app-phone-bindings" && resource.Menu.Route == "/app-phone-bindings" && resource.PermissionPrefix == "admin:app-phone-binding" {
				appPhoneBindingFound = true
			}
			if resource.Resource == "capabilities" && resource.Menu.Route == "/capabilities" && resource.PermissionPrefix == "admin:capability" {
				capabilityFound = true
			}
			if resource.Resource == "api-docs" && resource.Menu.Route == "/api-docs" && resource.PermissionPrefix == "admin:api-docs" {
				apiDocsFound = true
			}
			if resource.Resource == "files" && resource.Menu.Route == "/files" && resource.Menu.Parent == "storage" && resource.PermissionPrefix == "admin:file" {
				filesFound = true
			}
			if manifest.ID == "session" && resource.Resource == "sessions" && resource.Menu.Route == "/sessions" && resource.PermissionPrefix == "admin:session" {
				sessionFound = true
			}
			if manifest.ID == "audit" && resource.Resource == "login-logs" && resource.Menu.Route == "/login-logs" && resource.PermissionPrefix == "admin:login-log" {
				loginLogFound = true
			}
			if manifest.ID == "audit" && resource.Resource == "error-logs" && resource.Menu.Route == "/error-logs" && resource.PermissionPrefix == "admin:error-log" {
				errorLogFound = true
			}
			if manifest.ID == "system-admin" && resource.Resource == "versions" && resource.Menu.Route == "/versions" && resource.PermissionPrefix == "admin:version" {
				versionFound = true
			}
			if resource.Resource == "api-tokens" && resource.Menu.Route == "/api-tokens" && resource.Menu.Parent == "security" && resource.PermissionPrefix == "admin:api-token" {
				apiTokenFound = true
			}
			if manifest.ID == "policy-review" && resource.Resource == "policy-reviews" && resource.Menu.Route == "/policy-reviews" && resource.Menu.Parent == "access" && resource.PermissionPrefix == "admin:policy-review" {
				policyReviewFound = true
			}
		}
	}
	if !tenantFound {
		t.Fatalf("DefaultManifests() missing tenant admin surface")
	}
	if !orgUnitFound {
		t.Fatalf("DefaultManifests() missing org unit admin surface")
	}
	if !roleGroupFound {
		t.Fatalf("DefaultManifests() missing role group admin surface")
	}
	if !areaCodeFound {
		t.Fatalf("DefaultManifests() missing area code admin surface")
	}
	if !dictionaryFound {
		t.Fatalf("DefaultManifests() missing dictionary admin surface")
	}
	if !parameterFound {
		t.Fatalf("DefaultManifests() missing parameter admin surface")
	}
	if !brandingFound {
		t.Fatalf("DefaultManifests() missing branding admin surface")
	}
	if !settingsFound {
		t.Fatalf("DefaultManifests() missing settings admin surface")
	}
	if !appPhoneVerificationFound {
		t.Fatalf("DefaultManifests() missing app phone verification admin surface")
	}
	if !appPhoneBindingFound {
		t.Fatalf("DefaultManifests() missing app phone binding admin surface")
	}
	if !capabilityFound {
		t.Fatalf("DefaultManifests() missing capability admin surface")
	}
	if !apiDocsFound {
		t.Fatalf("DefaultManifests() missing api docs admin surface")
	}
	if !filesFound {
		t.Fatalf("DefaultManifests() missing file storage admin surface")
	}
	if !sessionFound {
		t.Fatalf("DefaultManifests() missing session admin surface")
	}
	if !loginLogFound {
		t.Fatalf("DefaultManifests() missing login log admin surface")
	}
	if !errorLogFound {
		t.Fatalf("DefaultManifests() missing error log admin surface")
	}
	if !versionFound {
		t.Fatalf("DefaultManifests() missing version admin surface")
	}
	if !apiTokenFound {
		t.Fatalf("DefaultManifests() missing api token admin surface")
	}
	if !policyReviewFound {
		t.Fatalf("DefaultManifests() missing optional policy review admin surface")
	}
}

func TestDefaultDeletionPoliciesDoNotAdvertiseMissingAuthoritativeAdapters(t *testing.T) {
	expectedDisabled := map[string]bool{
		"admin-identities":   true,
		"app-identities":     true,
		"app-phone-bindings": true,
		"sessions":           true,
	}
	for _, manifest := range DefaultManifests() {
		for _, resource := range manifest.Admin.Resources {
			if expectedDisabled[resource.Resource] {
				if resource.Deletion == nil || resource.Deletion.Mode != capability.AdminDeletionDisabled {
					t.Fatalf("%s deletion policy = %+v, want disabled until an authoritative adapter exists", resource.Resource, resource.Deletion)
				}
			}
			if resource.Resource == "api-tokens" {
				if resource.Deletion == nil || resource.Deletion.Mode != capability.AdminDeletionRevoke || resource.Deletion.RetentionDays != 90 {
					t.Fatalf("api-tokens deletion policy = %+v, want authoritative revoke with retention", resource.Deletion)
				}
			}
		}
	}
}

func TestPolicyReviewManifestIsOptionalAndKeepsRoleGroupsClassificationOnly(t *testing.T) {
	manifests := DefaultManifests()
	var policyReview capability.Manifest
	var roleGroups capability.AdminResource

	for _, manifest := range manifests {
		if manifest.ID == "policy-review" {
			policyReview = manifest
		}
		for _, resource := range manifest.Admin.Resources {
			if resource.Resource == "role-groups" {
				roleGroups = resource
			}
		}
	}
	if policyReview.ID == "" {
		t.Fatalf("DefaultManifests() missing policy-review capability")
	}
	if len(policyReview.Dependencies) == 0 {
		t.Fatalf("policy-review dependencies are empty")
	}
	if !slices.Contains(policyReview.Dependencies, capability.ID("rbac")) || !slices.Contains(policyReview.Dependencies, capability.ID("audit")) {
		t.Fatalf("policy-review dependencies = %+v, want rbac and audit", policyReview.Dependencies)
	}
	if len(policyReview.Admin.Resources) != 1 || policyReview.Admin.Resources[0].Resource != "policy-reviews" {
		t.Fatalf("policy-review resources = %+v, want policy-reviews only", policyReview.Admin.Resources)
	}

	requireFieldRelation(t, roleGroups, "parentCode", "role-groups")
	for _, field := range roleGroups.Fields {
		switch field.Key {
		case "permissions", "denyPermissions", "dataScope", "approvalStatus", "reviewStatus", "inheritedRoleCodes", "roleCodes":
			t.Fatalf("role-groups field %q adds policy semantics; keep policy review isolated", field.Key)
		}
	}
}

func TestOrgUnitManifestSupportsCommonInstitutionLevels(t *testing.T) {
	manifests := DefaultManifests()
	var orgUnits capability.AdminResource

	for _, manifest := range manifests {
		for _, resource := range manifest.Admin.Resources {
			if resource.Resource == "org-units" {
				orgUnits = resource
			}
		}
	}
	if orgUnits.Resource == "" {
		t.Fatalf("DefaultManifests() missing org-units resource")
	}
	var typeField capability.AdminField
	for _, field := range orgUnits.Fields {
		if field.Key == "type" {
			typeField = field
			break
		}
	}
	if typeField.Key == "" || typeField.Type != "select" || !typeField.Required {
		t.Fatalf("org-units.type = %+v, want required select", typeField)
	}
	options := map[string]struct{}{}
	for _, option := range typeField.Options {
		if option.Label.ZH == "" || option.Label.EN == "" {
			t.Fatalf("org-units.type option %q must declare zh/en labels", option.Value)
		}
		options[option.Value] = struct{}{}
	}
	for _, required := range []string{"group", "company", "branch", "organization", "department", "team", "store", "custom"} {
		if _, ok := options[required]; !ok {
			t.Fatalf("org-units.type options missing %q: %+v", required, typeField.Options)
		}
	}
}

func TestPersonnelManifestIsOptionalAndReusesOrganizationBoundaries(t *testing.T) {
	manifests := DefaultManifests()
	var personnel capability.Manifest

	for _, manifest := range manifests {
		if manifest.ID == "personnel" {
			personnel = manifest
			break
		}
	}
	if personnel.ID == "" {
		t.Fatalf("DefaultManifests() missing optional personnel capability")
	}
	for _, required := range []capability.ID{"tenant", "identity", "dictionary"} {
		if !slices.Contains(personnel.Dependencies, required) {
			t.Fatalf("personnel dependencies = %+v, want %q", personnel.Dependencies, required)
		}
	}

	resources := map[string]capability.AdminResource{}
	for _, resource := range personnel.Admin.Resources {
		resources[resource.Resource] = resource
	}
	for _, resourceID := range []string{"personnel-profiles", "positions", "position-assignments"} {
		if _, ok := resources[resourceID]; !ok {
			t.Fatalf("personnel resources missing %q: %+v", resourceID, personnel.Admin.Resources)
		}
	}

	requireFieldRelation(t, resources["personnel-profiles"], "tenantCode", "tenants")
	requireFieldRelation(t, resources["personnel-profiles"], "orgUnitCode", "org-units")
	requireFieldRelation(t, resources["personnel-profiles"], "areaCode", "area-codes")
	requireFieldRelation(t, resources["personnel-profiles"], "userCode", "users")
	requireFieldRelation(t, resources["positions"], "tenantCode", "tenants")
	requireFieldRelation(t, resources["positions"], "orgUnitCode", "org-units")
	requireFieldRelation(t, resources["position-assignments"], "personnelCode", "personnel-profiles")
	requireFieldRelation(t, resources["position-assignments"], "positionCode", "positions")
	requireFieldRelation(t, resources["position-assignments"], "tenantCode", "tenants")
	requireFieldRelation(t, resources["position-assignments"], "orgUnitCode", "org-units")
}

func TestNotificationManifestIsOptionalAndBusinessNeutral(t *testing.T) {
	manifests := DefaultManifests()
	var notification capability.Manifest

	for _, manifest := range manifests {
		if manifest.ID == "notification" {
			notification = manifest
			break
		}
	}
	if notification.ID == "" {
		t.Fatalf("DefaultManifests() missing optional notification capability")
	}
	for _, required := range []capability.ID{"tenant", "identity", "audit"} {
		if !slices.Contains(notification.Dependencies, required) {
			t.Fatalf("notification dependencies = %+v, want %q", notification.Dependencies, required)
		}
	}

	resources := map[string]capability.AdminResource{}
	for _, resource := range notification.Admin.Resources {
		resources[resource.Resource] = resource
	}
	for _, resourceID := range []string{"notification-templates", "notifications", "notification-deliveries"} {
		if _, ok := resources[resourceID]; !ok {
			t.Fatalf("notification resources missing %q: %+v", resourceID, notification.Admin.Resources)
		}
	}

	requireFieldRelation(t, resources["notification-templates"], "tenantCode", "tenants")
	requireFieldRelation(t, resources["notifications"], "tenantCode", "tenants")
	requireFieldRelation(t, resources["notifications"], "templateCode", "notification-templates")
	requireFieldRelation(t, resources["notifications"], "recipientUserCode", "users")
	requireFieldRelation(t, resources["notification-deliveries"], "tenantCode", "tenants")
	requireFieldRelation(t, resources["notification-deliveries"], "notificationCode", "notifications")
	requireFieldRelation(t, resources["notification-deliveries"], "recipientUserCode", "users")
}

func TestJobManifestIsOptionalAndBusinessNeutral(t *testing.T) {
	manifests := DefaultManifests()
	var job capability.Manifest

	for _, manifest := range manifests {
		if manifest.ID == "job" {
			job = manifest
			break
		}
	}
	if job.ID == "" {
		t.Fatalf("DefaultManifests() missing optional job capability")
	}
	for _, required := range []capability.ID{"tenant", "identity", "audit"} {
		if !slices.Contains(job.Dependencies, required) {
			t.Fatalf("job dependencies = %+v, want %q", job.Dependencies, required)
		}
	}

	resources := map[string]capability.AdminResource{}
	for _, resource := range job.Admin.Resources {
		resources[resource.Resource] = resource
	}
	for _, resourceID := range []string{"job-definitions", "job-runs", "job-run-attempts"} {
		if _, ok := resources[resourceID]; !ok {
			t.Fatalf("job resources missing %q: %+v", resourceID, job.Admin.Resources)
		}
	}

	requireFieldRelation(t, resources["job-definitions"], "tenantCode", "tenants")
	requireFieldRelation(t, resources["job-definitions"], "ownerUserCode", "users")
	requireFieldRelation(t, resources["job-runs"], "tenantCode", "tenants")
	requireFieldRelation(t, resources["job-runs"], "jobCode", "job-definitions")
	requireFieldRelation(t, resources["job-runs"], "triggeredBy", "users")
	requireFieldRelation(t, resources["job-run-attempts"], "tenantCode", "tenants")
	requireFieldRelation(t, resources["job-run-attempts"], "runCode", "job-runs")
}

func TestDefaultManifestsExposeLifecycleDeclarations(t *testing.T) {
	manifests := DefaultManifests()
	var migrationCount, seedCount int
	for _, manifest := range manifests {
		migrationCount += len(manifest.Migrations)
		seedCount += len(manifest.Seeds)
		for _, migration := range manifest.Migrations {
			if migration.ID == "" || migration.Description == "" || migration.Up == nil {
				t.Fatalf("manifest %q has incomplete migration: %+v", manifest.ID, migration)
			}
		}
		for _, seed := range manifest.Seeds {
			if seed.ID == "" || seed.Description == "" || seed.Run == nil {
				t.Fatalf("manifest %q has incomplete seed: %+v", manifest.ID, seed)
			}
		}
	}
	if migrationCount == 0 {
		t.Fatalf("DefaultManifests() expose no migrations")
	}
	if seedCount == 0 {
		t.Fatalf("DefaultManifests() expose no seeds")
	}
}

func requireFieldRelation(t *testing.T, resource capability.AdminResource, fieldKey string, relationResource string) {
	t.Helper()
	if resource.Resource == "" {
		t.Fatalf("resource is empty while checking field %q relation %q", fieldKey, relationResource)
	}
	for _, field := range resource.Fields {
		if field.Key != fieldKey {
			continue
		}
		if field.Relation == nil {
			t.Fatalf("resource %q field %q has no relation", resource.Resource, fieldKey)
		}
		if field.Relation.Resource != relationResource {
			t.Fatalf("resource %q field %q relation resource = %q, want %q", resource.Resource, fieldKey, field.Relation.Resource, relationResource)
		}
		if field.Label.ZH == "" || field.Label.EN == "" {
			t.Fatalf("resource %q field %q must declare zh/en labels", resource.Resource, fieldKey)
		}
		return
	}
	t.Fatalf("resource %q missing field %q", resource.Resource, fieldKey)
}

func TestDefaultManifestsExposeDemoDataDeclarations(t *testing.T) {
	manifests := DefaultManifests()
	var found bool
	for _, manifest := range manifests {
		for _, dataset := range manifest.DemoData {
			if dataset.ID == "platform-demo-tenants" && dataset.Resource == "tenants" && len(dataset.Records) > 0 {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("DefaultManifests() missing platform demo tenant data")
	}
}

func TestDefaultManifestsExposeAuthProviderDeclarations(t *testing.T) {
	manifests := DefaultManifests()
	var demoFound, wechatFound, oidcFound bool
	for _, manifest := range manifests {
		for _, provider := range manifest.AuthProviders {
			if provider.ID == "demo" && provider.Kind == "demo" && provider.Configured && provider.SupportsAudience(capability.AuthProviderAudienceAdmin) && provider.SupportsAudience(capability.AuthProviderAudienceApp) {
				demoFound = true
			}
			if provider.ID == "wechat" && provider.Kind == "wechat" && !provider.Configured && provider.SupportsAudience(capability.AuthProviderAudienceApp) && !provider.SupportsAudience(capability.AuthProviderAudienceAdmin) {
				wechatFound = true
			}
			if manifest.ID == "admin-oidc" && provider.ID == "oidc" && provider.Kind == "oidc" && !provider.Configured && provider.SupportsAudience(capability.AuthProviderAudienceAdmin) && !provider.SupportsAudience(capability.AuthProviderAudienceApp) && slices.Equal(provider.ConfigKeys, []string{"PLATFORM_ADMIN_OIDC_ISSUER_URL", "PLATFORM_ADMIN_OIDC_CLIENT_ID", "PLATFORM_ADMIN_OIDC_CLIENT_SECRET", "PLATFORM_ADMIN_OIDC_REDIRECT_URL"}) {
				oidcFound = true
			}
		}
	}
	if !demoFound {
		t.Fatalf("DefaultManifests() missing configured demo auth provider")
	}
	if !wechatFound {
		t.Fatalf("DefaultManifests() missing unconfigured wechat auth provider")
	}
	if !oidcFound {
		t.Fatalf("DefaultManifests() missing unconfigured admin oidc auth provider")
	}
}

func TestDefaultManifestsExposeAdminOIDCIdentityResourceWithoutRawIdentifiers(t *testing.T) {
	var identityResource capability.AdminResource
	for _, manifest := range DefaultManifests() {
		if manifest.ID != "admin-oidc" {
			continue
		}
		for _, resource := range manifest.Admin.Resources {
			if resource.Resource == "admin-identities" {
				identityResource = resource
			}
		}
	}
	if identityResource.Resource == "" {
		t.Fatalf("DefaultManifests() missing admin-identities resource")
	}
	fields := map[string]struct{}{}
	for _, field := range identityResource.Fields {
		fields[field.Key] = struct{}{}
	}
	for _, required := range []string{"provider", "providerKind", "issuerHash", "providerSubjectHash", "platformUsername", "createdAt", "lastLoginAt"} {
		if _, ok := fields[required]; !ok {
			t.Fatalf("admin-identities fields missing %q: %+v", required, identityResource.Fields)
		}
	}
	for _, forbidden := range []string{"issuer", "subject", "providerSubject"} {
		if _, ok := fields[forbidden]; ok {
			t.Fatalf("admin-identities exposes raw identifier field %q", forbidden)
		}
	}
}

func TestDefaultManifestsExposeAppRouteDeclarations(t *testing.T) {
	contracts, err := capability.AppRouteContracts(DefaultManifests())
	if err != nil {
		t.Fatalf("AppRouteContracts(DefaultManifests()) error = %v", err)
	}
	var loginFound, currentSessionFound, logoutFound, phoneVerificationFound, phoneBindingFound, appFileUploadFound, appFileContentFound bool
	for _, route := range contracts {
		switch {
		case route.CapabilityID == "session" && route.Method == "POST" && route.Path == "/api/app/auth/login" && route.Auth == capability.AppRouteAuthPublic:
			loginFound = true
		case route.CapabilityID == "session" && route.Method == "GET" && route.Path == "/api/app/session/current" && route.Auth == capability.AppRouteAuthSession:
			currentSessionFound = true
		case route.CapabilityID == "session" && route.Method == "POST" && route.Path == "/api/app/auth/logout" && route.Auth == capability.AppRouteAuthSession:
			logoutFound = true
		case route.CapabilityID == "app-phone" && route.Method == "POST" && route.Path == "/api/app/identity/phone-verifications" && route.Auth == capability.AppRouteAuthSession:
			phoneVerificationFound = true
		case route.CapabilityID == "app-phone" && route.Method == "POST" && route.Path == "/api/app/identity/phone-bindings" && route.Auth == capability.AppRouteAuthSession:
			phoneBindingFound = true
		case route.CapabilityID == "file-storage" && route.Method == "POST" && route.Path == "/api/app/files" && route.Auth == capability.AppRouteAuthSession:
			appFileUploadFound = true
		case route.CapabilityID == "file-storage" && route.Method == "GET" && route.Path == "/api/app/files/:id/content" && route.Auth == capability.AppRouteAuthSession:
			appFileContentFound = true
		}
	}
	if !loginFound {
		t.Fatalf("DefaultManifests() missing app login route declaration")
	}
	if !currentSessionFound {
		t.Fatalf("DefaultManifests() missing app current-session route declaration")
	}
	if !logoutFound {
		t.Fatalf("DefaultManifests() missing app logout route declaration")
	}
	if !phoneVerificationFound {
		t.Fatalf("DefaultManifests() missing app phone verification route declaration")
	}
	if !phoneBindingFound {
		t.Fatalf("DefaultManifests() missing app phone binding route declaration")
	}
	if !appFileUploadFound {
		t.Fatalf("DefaultManifests() missing app file upload route declaration")
	}
	if !appFileContentFound {
		t.Fatalf("DefaultManifests() missing app file content route declaration")
	}
}

func TestFileStorageServiceContractMatchesBoundAppRoutes(t *testing.T) {
	manifests := DefaultManifests()
	if err := capability.ValidateServiceContracts(manifests); err != nil {
		t.Fatalf("ValidateServiceContracts(DefaultManifests()) error = %v", err)
	}

	var fileStorage capability.Manifest
	for _, manifest := range manifests {
		if manifest.ID == "file-storage" {
			fileStorage = manifest
			break
		}
	}
	if fileStorage.Service.ID != "file-storage" {
		t.Fatalf("file-storage service = %+v, want canonical service surface", fileStorage.Service)
	}
	if fileStorage.Service.Stability != capability.ServiceStabilityStable || fileStorage.Service.Version != "1.0.0" {
		t.Fatalf("file-storage service baseline = %q/%q, want stable 1.0.0", fileStorage.Service.Stability, fileStorage.Service.Version)
	}

	wantBoundRoutes := map[string]bool{}
	for _, route := range fileStorage.App.Routes {
		wantBoundRoutes[route.Method+" "+route.Path] = true
	}
	gotBoundRoutes := map[string]bool{}
	boundSuccessStatuses := map[string]int{}
	planes := map[capability.ServicePlane]bool{}
	for _, operation := range fileStorage.Service.Operations {
		planes[operation.Plane] = true
		if operation.RuntimeStatus != capability.ServiceRuntimeBound {
			if operation.Method != "" || operation.Path != "" {
				t.Fatalf("contract-only operation %q claims HTTP binding %s %s", operation.ID, operation.Method, operation.Path)
			}
			continue
		}
		if operation.Plane != capability.ServicePlaneExternal {
			t.Fatalf("bound operation %q plane = %q, want external", operation.ID, operation.Plane)
		}
		gotBoundRoutes[operation.Method+" "+operation.Path] = true
		boundSuccessStatuses[operation.ID] = operation.SuccessStatus
		if operation.ID == "upload-file" {
			if !slices.Equal(operation.ResponseSchema.RequiredFields, []string{"data"}) {
				t.Fatalf("upload-file response required fields = %+v, want data envelope", operation.ResponseSchema.RequiredFields)
			}
		}
	}
	if len(gotBoundRoutes) != len(wantBoundRoutes) {
		t.Fatalf("bound service routes = %+v, app routes = %+v", gotBoundRoutes, wantBoundRoutes)
	}
	for route := range wantBoundRoutes {
		if !gotBoundRoutes[route] {
			t.Fatalf("bound service routes = %+v, missing app route %q", gotBoundRoutes, route)
		}
	}
	if boundSuccessStatuses["upload-file"] != 201 {
		t.Fatalf("upload-file success status = %d, want 201", boundSuccessStatuses["upload-file"])
	}
	if boundSuccessStatuses["read-file-content"] != 200 {
		t.Fatalf("read-file-content success status = %d, want 200", boundSuccessStatuses["read-file-content"])
	}
	for _, plane := range []capability.ServicePlane{capability.ServicePlaneAdmin, capability.ServicePlaneData, capability.ServicePlaneControl, capability.ServicePlaneExternal} {
		if !planes[plane] {
			t.Fatalf("file-storage service planes = %+v, missing %q", planes, plane)
		}
	}
	if len(fileStorage.Service.Events) == 0 {
		t.Fatalf("file-storage service events are empty")
	}
	for _, event := range fileStorage.Service.Events {
		if event.RuntimeStatus != capability.ServiceRuntimeContractOnly {
			t.Fatalf("event %q runtime status = %q, want contract-only", event.ID, event.RuntimeStatus)
		}
	}
}

func TestDefaultManifestsRecordLifecycleHistory(t *testing.T) {
	registry := capability.NewRegistry()
	for _, manifest := range DefaultManifests() {
		if err := registry.Register(manifest); err != nil {
			t.Fatalf("Register(%q) error = %v", manifest.ID, err)
		}
	}
	enabled := make([]capability.ID, 0, len(DefaultManifests()))
	for _, manifest := range DefaultManifests() {
		enabled = append(enabled, manifest.ID)
	}
	ordered, err := registry.ResolveEnabled(enabled)
	if err != nil {
		t.Fatalf("ResolveEnabled() error = %v", err)
	}

	path := filepath.Join(t.TempDir(), "lifecycle-history.json")
	history, err := capability.NewFileLifecycleHistory(path)
	if err != nil {
		t.Fatalf("NewFileLifecycleHistory() error = %v", err)
	}
	executor := capability.NewRecordedLifecycleExecutor(history)
	runtime := capability.Runtime{MigrationExecutor: executor, SeedExecutor: executor}

	if err := capability.RunLifecycle(context.Background(), ordered, runtime); err != nil {
		t.Fatalf("RunLifecycle(first) error = %v", err)
	}
	firstCount := len(history.Records())
	if firstCount == 0 {
		t.Fatalf("lifecycle history has no records after first run")
	}
	reopened, err := capability.NewFileLifecycleHistory(path)
	if err != nil {
		t.Fatalf("NewFileLifecycleHistory(reopened) error = %v", err)
	}
	runtime = capability.Runtime{MigrationExecutor: capability.NewRecordedLifecycleExecutor(reopened), SeedExecutor: capability.NewRecordedLifecycleExecutor(reopened)}
	if err := capability.RunLifecycle(context.Background(), ordered, runtime); err != nil {
		t.Fatalf("RunLifecycle(second) error = %v", err)
	}
	if got := len(reopened.Records()); got != firstCount {
		t.Fatalf("record count after second run = %d, want %d", got, firstCount)
	}
}
