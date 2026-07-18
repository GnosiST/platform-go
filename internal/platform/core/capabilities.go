package core

import (
	"context"

	"github.com/GnosiST/platform-go/internal/platform/capability"
)

func DefaultManifests() []capability.Manifest {
	return []capability.Manifest{
		{ID: "tenant", Name: "Tenant", Version: "0.1.0", Dependencies: []capability.ID{"dictionary"}, Admin: adminSurface(tenantAdminResource()), Migrations: lifecycleMigrations("tenant"), Seeds: lifecycleSeeds("tenant")},
		{ID: "identity", Name: "Identity", Version: "0.1.0", Dependencies: []capability.ID{"tenant"}, Admin: adminSurface(userAdminResource(), appIdentityAdminResource(), orgUnitAdminResource()), Migrations: lifecycleMigrations("identity"), Seeds: lifecycleSeeds("identity")},
		{ID: "session", Name: "Session", Version: "0.1.0", Dependencies: []capability.ID{"identity"}, Admin: adminSurface(adminResource("sessions", "在线会话", "Sessions", "后台和 App 会话、有效期与撤销状态。", "Admin and app sessions, expiration and revocation state.", "admin:session", "/sessions", "operations", "wifi", 350)), App: appSurface(appRoute("POST", "/api/app/auth/login", capability.AppRouteAuthPublic, "", "App 登录。", "App login."), appRoute("GET", "/api/app/session/current", capability.AppRouteAuthSession, "", "读取 App 当前会话。", "Read current app session."), appRoute("POST", "/api/app/auth/logout", capability.AppRouteAuthSession, "", "退出 App 会话。", "Log out app session.")), AuthProviders: []capability.AuthProvider{authProvider("demo", "demo", "演示登录", "Demo Login", "本地开发演示账号登录。", "Local demo account login.", true, []capability.AuthProviderAudience{capability.AuthProviderAudienceAdmin, capability.AuthProviderAudienceApp})}, Migrations: lifecycleMigrations("session"), Seeds: lifecycleSeeds("session")},
		{ID: "rbac", Name: "RBAC", Version: "0.1.0", Dependencies: []capability.ID{"tenant", "identity"}, Admin: adminSurface(roleGroupAdminResource(), roleAdminResource(), permissionAdminResource()), Migrations: lifecycleMigrations("rbac"), Seeds: lifecycleSeeds("rbac")},
		{ID: "menu", Name: "Menu", Version: "0.1.0", Dependencies: []capability.ID{"rbac"}, Admin: adminSurface(adminResource("menus", "菜单", "Menus", "后台菜单和资源入口。", "Admin menus and resource entries.", "admin:menu", "/menus", "foundation", "menus", 60)), Migrations: lifecycleMigrations("menu"), Seeds: lifecycleSeeds("menu")},
		{ID: "api-resource", Name: "API Resource", Version: "0.1.0", Dependencies: []capability.ID{"rbac"}, Admin: adminSurface(adminResource("api-resources", "API 资源", "API Resources", "接口资源、权限码和调用边界。", "API resources, permission codes, and invocation boundaries.", "admin:api-resource", "/api-resources", "governance", "apiResources", 120), adminResource("api-docs", "API 文档", "API Docs", "OpenAPI 文档、权限码和接口契约。", "OpenAPI docs, permission codes, and API contracts.", "admin:api-docs", "/api-docs", "governance", "book", 125)), Migrations: lifecycleMigrations("api-resource"), Seeds: lifecycleSeeds("api-resource")},
		{ID: "audit", Name: "Audit", Version: "0.1.0", Dependencies: []capability.ID{"tenant", "identity"}, Admin: adminSurface(adminResource("audit-logs", "审计日志", "Audit Logs", "操作审计和日志留痕。", "Operation audit and activity trails.", "admin:audit-log", "/audit-logs", "governance", "audit", 110), adminResource("login-logs", "登录日志", "Login Logs", "登录认证记录和安全追踪。", "Login authentication records and security tracing.", "admin:login-log", "/login-logs", "operations", "desktop", 320), adminResource("error-logs", "错误日志", "Error Logs", "运行错误、异常和排查记录。", "Runtime errors, exceptions and troubleshooting records.", "admin:error-log", "/error-logs", "operations", "warning", 330)), Migrations: lifecycleMigrations("audit"), Seeds: lifecycleSeeds("audit")},
		{ID: "policy-review", Name: "Policy Review", Version: "0.1.0", Dependencies: []capability.ID{"rbac", "audit", "identity"}, Admin: adminSurface(policyReviewAdminResource()), Migrations: lifecycleMigrations("policy-review"), Seeds: lifecycleSeeds("policy-review")},
		{ID: "wechat-login", Name: "WeChat Login", Version: "0.1.0", Dependencies: []capability.ID{"identity", "session", "audit"}, AuthProviders: []capability.AuthProvider{authProvider("wechat", "wechat", "微信登录", "WeChat Login", "微信 code 换取登录态。", "WeChat code exchange login.", false, []capability.AuthProviderAudience{capability.AuthProviderAudienceApp}, "PLATFORM_WECHAT_MINIAPP_APP_ID", "PLATFORM_WECHAT_MINIAPP_SECRET", "PLATFORM_WECHAT_MINIAPP_CODE2SESSION_ENDPOINT")}, Migrations: lifecycleMigrations("wechat-login"), Seeds: lifecycleSeeds("wechat-login")},
		{ID: "admin-oidc", Name: "Admin OIDC", Version: "0.1.0", Dependencies: []capability.ID{"identity", "session", "audit"}, Admin: adminSurface(adminIdentityAdminResource()), AuthProviders: []capability.AuthProvider{authProvider("oidc", "oidc", "企业单点登录", "Enterprise SSO", "通过 OpenID Connect 登录管理台。", "Sign in to Admin through OpenID Connect.", false, []capability.AuthProviderAudience{capability.AuthProviderAudienceAdmin}, "PLATFORM_ADMIN_OIDC_ISSUER_URL", "PLATFORM_ADMIN_OIDC_CLIENT_ID", "PLATFORM_ADMIN_OIDC_CLIENT_SECRET", "PLATFORM_ADMIN_OIDC_REDIRECT_URL")}, Migrations: lifecycleMigrations("admin-oidc"), Seeds: lifecycleSeeds("admin-oidc")},
		{ID: "app-phone", Name: "App Phone", Version: "0.1.0", Dependencies: []capability.ID{"identity", "session", "audit"}, Admin: adminSurface(appPhoneVerificationAdminResource(), appPhoneBindingAdminResource()), App: appSurface(appRoute("POST", "/api/app/identity/phone-verifications", capability.AppRouteAuthSession, "", "创建 App 手机验证码。", "Create app phone verification."), appRoute("POST", "/api/app/identity/phone-bindings", capability.AppRouteAuthSession, "", "绑定 App 手机号。", "Bind app phone number.")), Migrations: lifecycleMigrations("app-phone"), Seeds: lifecycleSeeds("app-phone")},
		{ID: "notification", Name: "Notification", Version: "0.1.0", Dependencies: []capability.ID{"tenant", "identity", "audit"}, Admin: adminSurface(notificationTemplateAdminResource(), notificationAdminResource(), notificationDeliveryAdminResource()), Migrations: lifecycleMigrations("notification"), Seeds: lifecycleSeeds("notification")},
		{ID: "job", Name: "Job", Version: "0.1.0", Dependencies: []capability.ID{"tenant", "identity", "audit"}, Admin: adminSurface(jobDefinitionAdminResource(), jobRunAdminResource(), jobRunAttemptAdminResource()), Migrations: lifecycleMigrations("job"), Seeds: lifecycleSeeds("job")},
		{ID: "personnel", Name: "Personnel", Version: "0.1.0", Dependencies: []capability.ID{"tenant", "identity", "dictionary"}, Admin: adminSurface(personnelProfileAdminResource(), positionAdminResource(), positionAssignmentAdminResource()), Migrations: lifecycleMigrations("personnel"), Seeds: lifecycleSeeds("personnel")},
		{ID: "dictionary", Name: "Dictionary", Version: "0.1.0", Admin: adminSurface(dictionaryAdminResource(), dictionaryParameterAdminResource(), areaCodeAdminResource()), Migrations: lifecycleMigrations("dictionary"), Seeds: lifecycleSeeds("dictionary")},
		{ID: "parameter", Name: "Parameter", Version: "0.1.0", Dependencies: []capability.ID{"tenant", "audit"}, Admin: adminSurface(parameterAdminResource(), brandingAdminResource(), adminResource("settings", "设置", "Settings", "平台配置和品牌设置。", "Platform configuration and branding.", "admin:settings", "/settings", "security", "settings", 310)), Migrations: lifecycleMigrations("parameter"), Seeds: lifecycleSeeds("parameter")},
		{ID: "file-storage", Name: "File Storage", Version: "0.1.0", Dependencies: []capability.ID{"tenant", "identity", "parameter", "audit"}, Admin: adminSurface(fileStorageAdminResource()), App: appSurface(appRoute("POST", "/api/app/files", capability.AppRouteAuthSession, "", "上传 App 文件。", "Upload app file."), appRoute("GET", "/api/app/files/:id/content", capability.AppRouteAuthSession, "", "读取 App 文件内容。", "Read app file content.")), Service: fileStorageServiceSurface(), Migrations: lifecycleMigrations("file-storage"), Seeds: lifecycleSeeds("file-storage")},
		{ID: "admin-shell", Name: "Admin Shell", Version: "0.1.0", Dependencies: []capability.ID{"identity", "session", "rbac", "menu"}, Admin: adminSurface(adminResource("capabilities", "能力清单", "Capabilities", "查看当前平台启用的能力包。", "View enabled platform capability packages.", "admin:capability", "/capabilities", "foundation", "capabilities", 20), adminResource("overview", "概览", "Overview", "平台底座运行概览。", "Platform foundation overview.", "admin:overview", "/overview", "foundation", "overview", 10)), Migrations: lifecycleMigrations("admin-shell"), Seeds: lifecycleSeeds("admin-shell")},
		{ID: "demo-data", Name: "Demo Data", Version: "0.1.0", Dependencies: []capability.ID{"admin-shell", "tenant"}, Admin: adminSurface(demoDataAdminResource()), Migrations: lifecycleMigrations("demo-data"), Seeds: lifecycleSeeds("demo-data"), DemoData: platformDemoDataSets()},
		{ID: "system-admin", Name: "System Admin", Version: "0.1.0", Dependencies: []capability.ID{"admin-shell", "api-resource", "dictionary", "parameter", "audit"}, Admin: adminSurface(monitoringAdminResource(), adminResource("versions", "版本管理", "Versions", "平台版本、发布记录和运行基线。", "Platform versions, release records and runtime baselines.", "admin:version", "/versions", "operations", "cluster", 360), apiTokenAdminResource()), Migrations: lifecycleMigrations("system-admin"), Seeds: lifecycleSeeds("system-admin")},
	}
}

func adminSurface(resources ...capability.AdminResource) capability.AdminSurface {
	for index := range resources {
		resources[index].Deletion = coreDeletionPolicy(resources[index].Resource)
	}
	return capability.AdminSurface{Resources: resources}
}

func coreDeletionPolicy(resource string) *capability.AdminResourceDeletionPolicy {
	policy := &capability.AdminResourceDeletionPolicy{PolicyVersion: 1}
	switch resource {
	case "api-docs", "branding", "capabilities", "demo-data", "overview", "settings":
		policy.Mode = capability.AdminDeletionDisabled
	case "app-phone-verifications", "audit-logs", "error-logs", "job-run-attempts", "job-runs", "login-logs", "notification-deliveries", "policy-reviews", "versions":
		policy.Mode = capability.AdminDeletionAppendOnly
	case "api-resources", "area-codes", "dictionaries", "permissions":
		policy.Mode = capability.AdminDeletionRestrict
		policy.RestrictReferences = true
	case "dictionary-parameters", "job-definitions", "menus", "monitoring", "notification-templates", "notifications", "org-units", "parameters", "personnel-profiles", "position-assignments", "positions", "role-groups", "roles", "tenants", "users":
		policy.Mode = capability.AdminDeletionSoftDelete
		policy.RestrictReferences = true
	case "admin-identities", "app-identities", "app-phone-bindings", "sessions":
		policy.Mode = capability.AdminDeletionDisabled
	case "api-tokens":
		policy.Mode = capability.AdminDeletionRevoke
		policy.RetentionDays = 90
		policy.AutoPurge = true
	case "files":
		policy.Mode = capability.AdminDeletionTombstone
		policy.RetentionDays = 30
		policy.AutoPurge = true
	default:
		panic("core admin resource missing deletion policy: " + resource)
	}
	return policy
}

func appSurface(routes ...capability.AppRoute) capability.AppSurface {
	return capability.AppSurface{Routes: routes}
}

func appRoute(method string, path string, auth capability.AppRouteAuth, permission string, descriptionZH string, descriptionEN string) capability.AppRoute {
	return capability.AppRoute{
		Method:      method,
		Path:        path,
		Auth:        auth,
		Permission:  permission,
		Description: capability.Text(descriptionZH, descriptionEN),
	}
}

func fileStorageServiceSurface() capability.ServiceSurface {
	compatibility := capability.ServiceCompatibility{Mode: "semver"}
	defaultReliability := capability.ServiceReliability{
		Idempotency:           "none",
		OptimisticConcurrency: "none",
		TimeoutMilliseconds:   5000,
		MaxRetries:            0,
		RateLimitPerMinute:    120,
		CostLimit:             10,
	}
	tenantScopes := []string{"tenant"}
	return capability.ServiceSurface{
		ID:            "file-storage",
		Owner:         "platform-core",
		Audiences:     []capability.ServiceAudience{capability.ServiceAudienceOperator, capability.ServiceAudienceInternal, capability.ServiceAudiencePartner},
		Stability:     capability.ServiceStabilityStable,
		Version:       "1.0.0",
		IdentityModes: []capability.ServiceIdentityMode{capability.ServiceIdentityManagementUser, capability.ServiceIdentityWorkload},
		AuthModes:     []capability.ServiceAuthMode{capability.ServiceAuthAdminSession, capability.ServiceAuthAppSession, capability.ServiceAuthWorkloadJWT},
		TenantContext: capability.DefaultTrustedTenantContext(),
		Operations: []capability.ServiceOperation{
			{
				ID: "list-files", Kind: capability.ServiceOperationQuery, Plane: capability.ServicePlaneAdmin, RuntimeStatus: capability.ServiceRuntimeContractOnly,
				IdentityMode: capability.ServiceIdentityManagementUser, AuthModes: []capability.ServiceAuthMode{capability.ServiceAuthAdminSession}, TenantMode: capability.ServiceTenantRequired,
				Permissions: []string{"admin:file:read"}, DataScopes: tenantScopes,
				RequestSchema:  capability.ServicePayloadSchema{Ref: "#/schemas/ListFilesRequest", PII: capability.ServicePIINone},
				ResponseSchema: capability.ServicePayloadSchema{Ref: "#/schemas/FileRecordPage", PII: capability.ServicePIIPersonal},
				Reliability:    defaultReliability, Compatibility: compatibility, Description: capability.Text("按租户查询文件记录。", "Query tenant file records."),
			},
			{
				ID: "store-file", Kind: capability.ServiceOperationCommand, Plane: capability.ServicePlaneData, RuntimeStatus: capability.ServiceRuntimeContractOnly,
				IdentityMode: capability.ServiceIdentityWorkload, AuthModes: []capability.ServiceAuthMode{capability.ServiceAuthWorkloadJWT}, TenantMode: capability.ServiceTenantRequired,
				DataScopes:     tenantScopes,
				RequestSchema:  capability.ServicePayloadSchema{Ref: "#/schemas/StoreFileRequest", RequiredFields: []string{"fileName", "content"}, PII: capability.ServicePIISensitive},
				ResponseSchema: capability.ServicePayloadSchema{Ref: "#/schemas/FileRecord", RequiredFields: []string{"id"}, PII: capability.ServicePIIPersonal},
				Reliability:    capability.ServiceReliability{Idempotency: "required-key", OptimisticConcurrency: "none", TimeoutMilliseconds: 5000, MaxRetries: 0, RateLimitPerMinute: 120, CostLimit: 20}, Compatibility: compatibility,
				Description: capability.Text("为可信服务存储租户文件。", "Store a tenant file for a trusted service."),
			},
			{
				ID: "read-file", Kind: capability.ServiceOperationQuery, Plane: capability.ServicePlaneData, RuntimeStatus: capability.ServiceRuntimeContractOnly,
				IdentityMode: capability.ServiceIdentityWorkload, AuthModes: []capability.ServiceAuthMode{capability.ServiceAuthWorkloadJWT}, TenantMode: capability.ServiceTenantRequired,
				DataScopes:     tenantScopes,
				RequestSchema:  capability.ServicePayloadSchema{Ref: "#/schemas/ReadFileRequest", RequiredFields: []string{"id"}, PII: capability.ServicePIINone},
				ResponseSchema: capability.ServicePayloadSchema{Ref: "#/schemas/FileContent", RequiredFields: []string{"content"}, PII: capability.ServicePIISensitive},
				Reliability:    defaultReliability, Compatibility: compatibility, Description: capability.Text("为可信服务读取租户文件。", "Read a tenant file for a trusted service."),
			},
			{
				ID: "inspect-storage", Kind: capability.ServiceOperationQuery, Plane: capability.ServicePlaneControl, RuntimeStatus: capability.ServiceRuntimeContractOnly,
				IdentityMode: capability.ServiceIdentityWorkload, AuthModes: []capability.ServiceAuthMode{capability.ServiceAuthWorkloadJWT}, TenantMode: capability.ServiceTenantPlatform,
				RequestSchema:  capability.ServicePayloadSchema{Ref: "#/schemas/InspectStorageRequest", PII: capability.ServicePIINone},
				ResponseSchema: capability.ServicePayloadSchema{Ref: "#/schemas/StorageStatus", RequiredFields: []string{"status"}, PII: capability.ServicePIINone},
				Reliability:    defaultReliability, Compatibility: compatibility, Description: capability.Text("检查文件存储运行状态。", "Inspect file storage runtime status."),
			},
			{
				ID: "upload-file", Kind: capability.ServiceOperationCommand, Plane: capability.ServicePlaneExternal, RuntimeStatus: capability.ServiceRuntimeBound,
				IdentityMode: capability.ServiceIdentityManagementUser, AuthModes: []capability.ServiceAuthMode{capability.ServiceAuthAppSession}, TenantMode: capability.ServiceTenantRequired,
				DataScopes: tenantScopes, Method: "POST", Path: "/api/app/files", RequestMediaType: "multipart/form-data", ResponseMediaType: "application/json", SuccessStatus: 201,
				RequestSchema:  capability.ServicePayloadSchema{Ref: "#/schemas/AppFileUploadRequest", RequiredFields: []string{"file"}, PII: capability.ServicePIISensitive},
				ResponseSchema: capability.ServicePayloadSchema{Ref: "#/schemas/AppFileUploadResponse", RequiredFields: []string{"data"}, PII: capability.ServicePIIPersonal},
				Reliability:    defaultReliability, Compatibility: compatibility, Description: capability.Text("上传 App 文件。", "Upload an app file."),
			},
			{
				ID: "read-file-content", Kind: capability.ServiceOperationQuery, Plane: capability.ServicePlaneExternal, RuntimeStatus: capability.ServiceRuntimeBound,
				IdentityMode: capability.ServiceIdentityManagementUser, AuthModes: []capability.ServiceAuthMode{capability.ServiceAuthAppSession}, TenantMode: capability.ServiceTenantRequired,
				DataScopes: tenantScopes, Method: "GET", Path: "/api/app/files/:id/content", ResponseMediaType: "application/octet-stream", SuccessStatus: 200,
				RequestSchema:  capability.ServicePayloadSchema{Ref: "#/schemas/AppFileContentRequest", RequiredFields: []string{"id"}, PII: capability.ServicePIINone},
				ResponseSchema: capability.ServicePayloadSchema{Ref: "#/schemas/FileContent", RequiredFields: []string{"content"}, PII: capability.ServicePIISensitive},
				Reliability:    defaultReliability, Compatibility: compatibility, Description: capability.Text("读取 App 文件内容。", "Read app file content."),
			},
		},
		Events: []capability.ServiceEvent{
			{
				ID: "file-stored", Name: "platform.file-storage.file-stored.v1", Version: 1, Direction: capability.ServiceEventPublish, RuntimeStatus: capability.ServiceRuntimeContractOnly,
				TenantMode: capability.ServiceTenantRequired, DataScopes: tenantScopes,
				PayloadSchema:   capability.ServicePayloadSchema{Ref: "#/schemas/FileStoredEvent", RequiredFields: []string{"fileId"}, PII: capability.ServicePIIPersonal},
				EnvelopeVersion: "1.0", TraceContext: []string{"traceparent", "tracestate"}, Compatibility: compatibility,
				Description: capability.Text("文件已存储合同事件。", "File stored contract event."),
			},
		},
		SLA:           capability.ServiceSLA{AvailabilityTarget: "99.9%", LatencyP95MS: 1000},
		Compatibility: compatibility,
		RuntimeBoundary: capability.ServiceRuntimeBoundary{
			ContractExecution:    "external upload and content read are HTTP-bound; admin, service, and control operations are contract-only",
			IdentityProtocols:    "app-session is bound for external operations; admin-session and workload-jwt are declaration-only",
			EventDelivery:        "contract-only; no event broker or outbox delivery is implemented",
			DatasourceRouting:    "single configured datasource; client physical routing is forbidden",
			RuntimeSourceWriting: "disabled",
		},
	}
}

func authProvider(id string, kind string, titleZH string, titleEN string, descriptionZH string, descriptionEN string, configured bool, audiences []capability.AuthProviderAudience, configKeys ...string) capability.AuthProvider {
	return capability.AuthProvider{
		ID:          id,
		Kind:        kind,
		Title:       capability.Text(titleZH, titleEN),
		Description: capability.Text(descriptionZH, descriptionEN),
		Enabled:     true,
		Configured:  configured,
		ConfigKeys:  append([]string(nil), configKeys...),
		Audiences:   append([]capability.AuthProviderAudience(nil), audiences...),
	}
}

func lifecycleMigrations(id capability.ID) []capability.Migration {
	return []capability.Migration{
		{
			ID:          "core-" + string(id) + "-0001",
			Description: "Register core " + string(id) + " migration boundary.",
			Up:          noopLifecycleStep,
		},
	}
}

func lifecycleSeeds(id capability.ID) []capability.Seed {
	return []capability.Seed{
		{
			ID:          "core-" + string(id) + "-seed-0001",
			Description: "Register core " + string(id) + " seed boundary.",
			Run:         noopLifecycleStep,
		},
	}
}

func noopLifecycleStep(context.Context, capability.Runtime) error {
	return nil
}

func platformDemoDataSets() []capability.DemoDataSet {
	return []capability.DemoDataSet{
		{
			ID:          "platform-demo-tenants",
			Title:       capability.Text("平台演示租户", "Platform Demo Tenants"),
			Description: capability.Text("用于本地演示和底座验证的租户数据。", "Tenant data for local demos and platform validation."),
			Resource:    "tenants",
			Records: []capability.DemoRecord{
				{
					ID:          "tenant-demo-acme",
					Code:        "demo-acme",
					Name:        "Demo Acme Tenant",
					Status:      "enabled",
					Description: "Reusable tenant for platform demos.",
					Values:      map[string]string{"isolation": "sandbox"},
				},
			},
		},
	}
}

func demoDataAdminResource() capability.AdminResource {
	resource := adminResource("demo-data", "演示数据", "Demo Data", "声明式演示数据集。", "Declarative demo data sets.", "admin:demo-data", "/demo-data", "operations", "dictParams", 205)
	resource.Actions = []capability.AdminResourceAction{
		{
			Key: "apply", Label: capability.Text("执行", "Apply"), Kind: "resource", Tone: "primary", Icon: "database",
			Permission: "admin:demo-data:apply", Route: "/api/admin/demo-data/:capabilityId/:datasetId/apply", Method: "POST", AuditAction: "demo-data.apply", Refresh: true,
		},
	}
	return resource
}

func adminResource(resource string, titleZH string, titleEN string, descriptionZH string, descriptionEN string, permissionPrefix string, route string, group string, icon string, order int) capability.AdminResource {
	return capability.AdminResource{
		Resource:         resource,
		Title:            capability.Text(titleZH, titleEN),
		Description:      capability.Text(descriptionZH, descriptionEN),
		PermissionPrefix: permissionPrefix,
		Menu: capability.AdminMenu{
			Route: route,
			Group: group,
			Icon:  icon,
			Order: order,
		},
	}
}

func tenantAdminResource() capability.AdminResource {
	return capability.AdminResource{
		Resource:         "tenants",
		Title:            capability.Text("租户", "Tenants"),
		Description:      capability.Text("租户空间、隔离边界和区域归属。", "Tenant spaces, isolation boundaries, and regional ownership."),
		PermissionPrefix: "admin:tenant",
		Menu:             capability.AdminMenu{Route: "/tenants", Parent: "identity", Group: "foundation", Icon: "tenants", Order: 30, Cache: true},
		Fields: []capability.AdminField{
			adminField("code", "租户编码", "Tenant Code", "text", "record", true, false, true, true, true, true, 160, nil),
			adminField("name", "租户名称", "Tenant Name", "text", "record", true, false, true, true, true, true, 180, nil),
			adminField("isolation", "隔离模式", "Isolation", "select", "values", false, false, true, true, true, true, 140, []capability.AdminFieldOption{
				adminFieldOption("shared", "共享", "Shared"),
				adminFieldOption("sandbox", "沙箱", "Sandbox"),
			}),
			relationAdminField(adminField("areaCode", "地址码", "Area Code", "select", "values", false, false, true, true, true, true, 140, nil), areaCodeAdminFieldRelation(enabledAdminRelationFilter())),
			adminField("status", "状态", "Status", "select", "record", false, false, true, true, true, true, 120, enabledDisabledOptions()),
			adminField("description", "描述", "Description", "textarea", "record", false, false, false, false, true, true, 220, nil),
			adminField("updatedAt", "更新时间", "Updated At", "datetime", "record", false, true, false, true, false, true, 180, nil),
		},
		SearchFields:   []string{"name", "code", "status", "description", "isolation", "areaCode"},
		DefaultSortKey: "updatedAt",
	}
}

func userAdminResource() capability.AdminResource {
	return capability.AdminResource{
		Resource:         "users",
		Title:            capability.Text("用户", "Users"),
		Description:      capability.Text("后台用户、账号、组织归属和角色绑定。", "Admin users, accounts, organization ownership, and role bindings."),
		PermissionPrefix: "admin:user",
		Menu:             capability.AdminMenu{Route: "/users", Parent: "identity", Group: "foundation", Icon: "users", Order: 40, Cache: true},
		Fields: []capability.AdminField{
			adminField("code", "用户名", "Username", "text", "record", true, false, true, true, true, true, 160, nil),
			adminField("name", "姓名", "Name", "text", "record", true, false, true, true, true, true, 160, nil),
			relationAdminField(adminField("tenantCode", "租户", "Tenant", "select", "values", true, false, true, true, true, true, 150, nil), adminFieldRelation("tenants", "code", "name", false, enabledAdminRelationFilter())),
			relationAdminField(adminField("orgUnitCode", "机构", "Org Unit", "select", "values", false, false, true, true, true, true, 160, nil), treeAdminFieldRelation("org-units", "code", "name", "parentCode", enabledAdminRelationFilter())),
			relationAdminField(adminField("areaCode", "地址码", "Area Code", "select", "values", false, false, true, true, true, true, 140, nil), areaCodeAdminFieldRelation(enabledAdminRelationFilter())),
			relationAdminField(adminField("roles", "角色", "Roles", "multiselect", "values", false, false, true, true, true, true, 220, nil), adminFieldRelation("roles", "code", "name", true, enabledAdminRelationFilter())),
			adminField("status", "状态", "Status", "select", "record", false, false, true, true, true, true, 120, enabledDisabledOptions()),
			adminField("description", "描述", "Description", "textarea", "record", false, false, false, false, true, true, 220, nil),
			adminField("updatedAt", "更新时间", "Updated At", "datetime", "record", false, true, false, true, false, true, 180, nil),
		},
		SearchFields:   []string{"name", "code", "status", "description", "tenantCode", "orgUnitCode", "areaCode", "roles"},
		DefaultSortKey: "updatedAt",
	}
}

func orgUnitAdminResource() capability.AdminResource {
	return capability.AdminResource{
		Resource:         "org-units",
		Title:            capability.Text("组织机构", "Org Units"),
		Description:      capability.Text("租户下的集团、公司、分支、部门、团队和门店层级。", "Group, company, branch, department, team, and store hierarchy under tenants."),
		PermissionPrefix: "admin:org-unit",
		Menu:             capability.AdminMenu{Route: "/org-units", Parent: "identity", Group: "foundation", Icon: "cluster", Order: 45, Cache: true},
		Fields: []capability.AdminField{
			adminField("code", "机构编码", "Org Unit Code", "text", "record", true, false, true, true, true, true, 160, nil),
			adminField("name", "机构名称", "Org Unit Name", "text", "record", true, false, true, true, true, true, 180, nil),
			adminField("type", "类型", "Type", "select", "values", true, false, true, true, true, true, 130, orgUnitTypeOptions()),
			relationAdminField(adminField("tenantCode", "租户", "Tenant", "select", "values", true, false, true, true, true, true, 150, nil), adminFieldRelation("tenants", "code", "name", false, enabledAdminRelationFilter())),
			relationAdminField(adminField("parentCode", "上级机构", "Parent", "select", "values", false, false, true, true, true, true, 160, nil), treeAdminFieldRelation("org-units", "code", "name", "parentCode", enabledAdminRelationFilter())),
			relationAdminField(adminField("areaCode", "地址码", "Area Code", "select", "values", false, false, true, true, true, true, 140, nil), areaCodeAdminFieldRelation(enabledAdminRelationFilter())),
			adminField("sortOrder", "排序", "Sort Order", "number", "values", false, false, false, true, true, true, 110, nil),
			adminField("status", "状态", "Status", "select", "record", false, false, true, true, true, true, 120, enabledDisabledOptions()),
			adminField("description", "描述", "Description", "textarea", "record", false, false, false, false, true, true, 220, nil),
			adminField("updatedAt", "更新时间", "Updated At", "datetime", "record", false, true, false, true, false, true, 180, nil),
		},
		SearchFields:   []string{"name", "code", "status", "description", "type", "tenantCode", "parentCode", "areaCode"},
		DefaultSortKey: "sortOrder",
	}
}

func roleGroupAdminResource() capability.AdminResource {
	return capability.AdminResource{
		Resource:         "role-groups",
		Title:            capability.Text("角色组", "Role Groups"),
		Description:      capability.Text("用于角色分类、治理和授权维护，不直接授予权限。", "Classifies roles for governance and authorization maintenance without granting permissions directly."),
		PermissionPrefix: "admin:role-group",
		Menu:             capability.AdminMenu{Route: "/role-groups", Parent: "access", Group: "foundation", Icon: "roles", Order: 48, Cache: true},
		Fields: []capability.AdminField{
			adminField("code", "分组编码", "Group Code", "text", "record", true, false, true, true, true, true, 160, nil),
			adminField("name", "分组名称", "Group Name", "text", "record", true, false, true, true, true, true, 180, nil),
			relationAdminField(adminField("parentCode", "上级角色组", "Parent Group", "select", "values", false, false, true, true, true, true, 160, nil), treeAdminFieldRelation("role-groups", "code", "name", "parentCode", enabledAdminRelationFilter())),
			adminField("sortOrder", "排序", "Sort Order", "number", "values", false, false, false, true, true, true, 110, nil),
			adminField("status", "状态", "Status", "select", "record", false, false, true, true, true, true, 120, enabledDisabledOptions()),
			adminField("description", "说明", "Description", "textarea", "record", false, false, false, false, true, true, 220, nil),
		},
		SearchFields:   []string{"name", "code", "status", "description", "parentCode"},
		DefaultSortKey: "sortOrder",
	}
}

func roleAdminResource() capability.AdminResource {
	return capability.AdminResource{
		Resource:         "roles",
		Title:            capability.Text("角色", "Roles"),
		Description:      capability.Text("角色、权限和授权策略。", "Roles, permissions, and authorization policies."),
		PermissionPrefix: "admin:role",
		Menu:             capability.AdminMenu{Route: "/roles", Parent: "access", Group: "foundation", Icon: "roles", Order: 50, Cache: true},
		Fields: []capability.AdminField{
			adminField("code", "角色编码", "Code", "text", "record", true, false, true, true, true, true, 160, nil),
			adminField("name", "角色名称", "Name", "text", "record", true, false, true, true, true, true, 180, nil),
			relationAdminField(adminField("groupCode", "角色组", "Role Group", "select", "values", false, false, true, true, true, true, 150, nil), treeAdminFieldRelation("role-groups", "code", "name", "parentCode", enabledAdminRelationFilter())),
			adminField("dataScope", "数据范围", "Data Scope", "select", "values", true, false, true, true, true, true, 190, dataScopeOptions()),
			relationAdminField(adminField("dataScopeOrgCodes", "数据机构", "Data Orgs", "multiselect", "values", false, false, true, false, true, true, 220, nil), treeMultiAdminFieldRelation("org-units", "code", "name", "parentCode", enabledAdminRelationFilter())),
			relationAdminField(adminField("dataScopeAreaCodes", "数据区域", "Data Areas", "multiselect", "values", false, false, true, false, true, true, 220, nil), areaCodeMultiAdminFieldRelation(enabledAdminRelationFilter())),
			relationAdminField(adminField("permissions", "权限码", "Permissions", "multiselect", "values", false, false, true, false, true, true, 260, nil), adminFieldRelation("permissions", "code", "name", true, enabledAdminRelationFilter())),
			relationAdminField(adminField("denyPermissions", "拒绝权限", "Deny Permissions", "multiselect", "values", false, false, true, false, true, true, 260, nil), adminFieldRelation("permissions", "code", "name", true, enabledAdminRelationFilter())),
			adminField("status", "状态", "Status", "select", "record", false, false, true, true, true, true, 120, enabledDisabledOptions()),
		},
		SearchFields:   []string{"name", "code", "status", "groupCode", "dataScope", "dataScopeOrgCodes", "dataScopeAreaCodes", "denyPermissions"},
		DefaultSortKey: "updatedAt",
	}
}

func permissionAdminResource() capability.AdminResource {
	return capability.AdminResource{
		Resource:         "permissions",
		Title:            capability.Text("权限", "Permissions"),
		Description:      capability.Text("平台级 API 与页面按钮权限资源目录。", "Platform-wide API and page-button permission resource catalog."),
		PermissionPrefix: "admin:permission",
		Menu:             capability.AdminMenu{Route: "/permissions", Parent: "access", Group: "foundation", Icon: "roles", Order: 55, Cache: true},
		Fields: []capability.AdminField{
			adminField("code", "权限编码", "Permission Code", "text", "record", true, false, true, true, true, true, 220, nil),
			adminField("name", "权限名称", "Permission Name", "text", "record", true, false, true, true, true, true, 180, nil),
			adminField("resourceType", "资源类型", "Resource Type", "select", "values", true, false, true, true, true, true, 130, []capability.AdminFieldOption{
				adminFieldOption("api", "接口权限", "API"),
				adminFieldOption("page-button", "页面按钮", "Page Button"),
			}),
			adminField("capability", "能力", "Capability", "text", "values", true, false, true, true, true, true, 150, nil),
			adminField("resource", "资源", "Resource", "text", "values", true, false, true, true, true, true, 160, nil),
			adminField("action", "动作", "Action", "text", "values", true, false, true, true, true, true, 120, nil),
			adminField("menuCode", "菜单编码", "Menu Code", "text", "values", false, true, false, false, false, true, 180, nil),
			adminField("buttonKey", "按钮标识", "Button Key", "text", "values", false, true, false, false, false, true, 160, nil),
			adminField("prefix", "前缀", "Prefix", "text", "values", true, false, true, true, true, true, 180, nil),
			adminField("status", "状态", "Status", "select", "record", false, false, true, true, true, true, 120, enabledDisabledOptions()),
			adminField("description", "说明", "Description", "textarea", "record", false, false, false, false, true, true, 220, nil),
		},
		SearchFields:   []string{"name", "code", "status", "description", "resourceType", "capability", "resource", "action", "prefix"},
		DefaultSortKey: "updatedAt",
	}
}

func policyReviewAdminResource() capability.AdminResource {
	return capability.AdminResource{
		Resource:         "policy-reviews",
		Title:            capability.Text("策略评审", "Policy Reviews"),
		Description:      capability.Text("角色、权限和数据范围变更的可选审批台账。", "Optional approval ledger for role, permission, and data-scope changes."),
		PermissionPrefix: "admin:policy-review",
		Menu:             capability.AdminMenu{Route: "/policy-reviews", Parent: "access", Group: "governance", Icon: "audit", Order: 115, Cache: false},
		Fields: []capability.AdminField{
			adminField("code", "评审单号", "Review Code", "text", "record", true, false, true, true, true, true, 170, nil),
			adminField("name", "标题", "Title", "text", "record", true, false, true, true, true, true, 180, nil),
			adminField("status", "状态", "Status", "select", "record", false, false, true, true, true, true, 120, enabledDisabledOptions()),
			adminField("policyType", "策略类型", "Policy Type", "select", "values", true, false, true, true, true, true, 140, []capability.AdminFieldOption{
				adminFieldOption("role_permission", "角色权限", "Role Permission"),
				adminFieldOption("deny_permission", "拒绝权限", "Deny Permission"),
				adminFieldOption("data_scope", "数据范围", "Data Scope"),
				adminFieldOption("menu_visibility", "菜单可见性", "Menu Visibility"),
			}),
			adminField("requestedAction", "申请动作", "Requested Action", "select", "values", true, false, true, true, true, true, 140, []capability.AdminFieldOption{
				adminFieldOption("create", "新增", "Create"),
				adminFieldOption("update", "更新", "Update"),
				adminFieldOption("delete", "删除", "Delete"),
				adminFieldOption("enable", "启用", "Enable"),
				adminFieldOption("disable", "停用", "Disable"),
			}),
			adminField("reviewStatus", "评审状态", "Review Status", "select", "values", true, false, true, true, true, true, 140, []capability.AdminFieldOption{
				adminFieldOption("draft", "草稿", "Draft"),
				adminFieldOption("pending", "待评审", "Pending"),
				adminFieldOption("approved", "已通过", "Approved"),
				adminFieldOption("rejected", "已拒绝", "Rejected"),
				adminFieldOption("cancelled", "已取消", "Cancelled"),
			}),
			relationAdminField(adminField("roleCode", "目标角色", "Target Role", "select", "values", false, false, true, true, true, true, 160, nil), adminFieldRelation("roles", "code", "name", false, enabledAdminRelationFilter())),
			relationAdminField(adminField("permissionCodes", "权限码", "Permission Codes", "multiselect", "values", false, false, true, true, true, true, 240, nil), adminFieldRelation("permissions", "code", "name", true, enabledAdminRelationFilter())),
			relationAdminField(adminField("requestedBy", "申请人", "Requested By", "select", "values", false, false, true, true, true, true, 150, nil), adminFieldRelation("users", "code", "name", false, enabledAdminRelationFilter())),
			relationAdminField(adminField("reviewedBy", "评审人", "Reviewed By", "select", "values", false, false, true, true, true, true, 150, nil), adminFieldRelation("users", "code", "name", false, enabledAdminRelationFilter())),
			adminField("submittedAt", "提交时间", "Submitted At", "datetime", "values", false, false, true, true, true, true, 180, nil),
			adminField("reviewedAt", "评审时间", "Reviewed At", "datetime", "values", false, false, true, true, true, true, 180, nil),
			adminField("rejectionReason", "拒绝原因", "Rejection Reason", "textarea", "values", false, true, false, false, false, true, 260, nil),
			adminField("description", "评审说明", "Review Notes", "textarea", "record", false, false, false, false, true, true, 260, nil),
			adminField("updatedAt", "更新时间", "Updated At", "datetime", "record", false, true, false, true, false, true, 180, nil),
		},
		Actions: []capability.AdminResourceAction{
			{
				Key: "export", Label: capability.Text("导出证据", "Export Evidence"), Kind: "resource", Tone: "default", Icon: "download",
				Permission: "admin:policy-review:export", Route: "/api/admin/policy-reviews/export", Method: "GET", AuditAction: "policy-review.export",
			},
		},
		SearchFields:   []string{"name", "code", "policyType", "requestedAction", "reviewStatus", "roleCode", "permissionCodes", "requestedBy", "reviewedBy"},
		DefaultSortKey: "submittedAt",
	}
}

func personnelProfileAdminResource() capability.AdminResource {
	return capability.AdminResource{
		Resource:         "personnel-profiles",
		Title:            capability.Text("人员档案", "Personnel Profiles"),
		Description:      capability.Text("可选人员档案能力，复用租户、机构、地址码和平台账号边界。", "Optional personnel records reusing tenant, org unit, area code, and platform account boundaries."),
		PermissionPrefix: "admin:personnel-profile",
		Menu:             capability.AdminMenu{Route: "/personnel-profiles", Parent: "identity", Group: "foundation", Icon: "users", Order: 52, Cache: true},
		Fields: []capability.AdminField{
			adminField("code", "人员编码", "Personnel Code", "text", "record", true, false, true, true, true, true, 160, nil),
			adminField("name", "姓名", "Name", "text", "record", true, false, true, true, true, true, 160, nil),
			relationAdminField(adminField("tenantCode", "租户", "Tenant", "select", "values", false, false, true, true, true, true, 150, nil), adminFieldRelation("tenants", "code", "name", false, enabledAdminRelationFilter())),
			relationAdminField(adminField("orgUnitCode", "机构", "Org Unit", "select", "values", false, false, true, true, true, true, 160, nil), treeAdminFieldRelation("org-units", "code", "name", "parentCode", enabledAdminRelationFilter())),
			relationAdminField(adminField("areaCode", "地址码", "Area Code", "select", "values", false, false, true, true, true, true, 140, nil), areaCodeAdminFieldRelation(enabledAdminRelationFilter())),
			relationAdminField(adminField("userCode", "绑定账号", "Linked User", "select", "values", false, false, true, true, true, true, 160, nil), adminFieldRelation("users", "code", "name", false, enabledAdminRelationFilter())),
			adminField("personnelType", "人员类型", "Personnel Type", "select", "values", false, false, true, true, true, true, 130, personnelTypeOptions()),
			adminField("employmentStatus", "任职状态", "Employment Status", "select", "values", false, false, true, true, true, true, 140, employmentStatusOptions()),
			adminField("status", "状态", "Status", "select", "record", false, false, true, true, true, true, 120, enabledDisabledOptions()),
			adminField("description", "说明", "Description", "textarea", "record", false, false, false, false, true, true, 220, nil),
			adminField("updatedAt", "更新时间", "Updated At", "datetime", "record", false, true, false, true, false, true, 180, nil),
		},
		SearchFields:   []string{"name", "code", "status", "description", "tenantCode", "orgUnitCode", "areaCode", "userCode", "personnelType", "employmentStatus"},
		DefaultSortKey: "updatedAt",
	}
}

func positionAdminResource() capability.AdminResource {
	return capability.AdminResource{
		Resource:         "positions",
		Title:            capability.Text("岗位", "Positions"),
		Description:      capability.Text("可选岗位目录，支持租户和机构范围内的岗位治理。", "Optional position catalog scoped by tenant and org unit."),
		PermissionPrefix: "admin:position",
		Menu:             capability.AdminMenu{Route: "/positions", Parent: "identity", Group: "foundation", Icon: "roles", Order: 53, Cache: true},
		Fields: []capability.AdminField{
			adminField("code", "岗位编码", "Position Code", "text", "record", true, false, true, true, true, true, 160, nil),
			adminField("name", "岗位名称", "Position Name", "text", "record", true, false, true, true, true, true, 180, nil),
			relationAdminField(adminField("tenantCode", "租户", "Tenant", "select", "values", false, false, true, true, true, true, 150, nil), adminFieldRelation("tenants", "code", "name", false, enabledAdminRelationFilter())),
			relationAdminField(adminField("orgUnitCode", "机构", "Org Unit", "select", "values", false, false, true, true, true, true, 160, nil), treeAdminFieldRelation("org-units", "code", "name", "parentCode", enabledAdminRelationFilter())),
			adminField("positionLevel", "岗位级别", "Position Level", "text", "values", false, false, true, true, true, true, 130, nil),
			adminField("sortOrder", "排序", "Sort Order", "number", "values", false, false, false, true, true, true, 110, nil),
			adminField("status", "状态", "Status", "select", "record", false, false, true, true, true, true, 120, enabledDisabledOptions()),
			adminField("description", "说明", "Description", "textarea", "record", false, false, false, false, true, true, 220, nil),
			adminField("updatedAt", "更新时间", "Updated At", "datetime", "record", false, true, false, true, false, true, 180, nil),
		},
		SearchFields:   []string{"name", "code", "status", "description", "tenantCode", "orgUnitCode", "positionLevel"},
		DefaultSortKey: "sortOrder",
	}
}

func positionAssignmentAdminResource() capability.AdminResource {
	return capability.AdminResource{
		Resource:         "position-assignments",
		Title:            capability.Text("任职关系", "Position Assignments"),
		Description:      capability.Text("人员与岗位的任职关系，支持主岗、兼职和任期记录。", "Personnel-to-position assignments with primary, part-time, and term records."),
		PermissionPrefix: "admin:position-assignment",
		Menu:             capability.AdminMenu{Route: "/position-assignments", Parent: "identity", Group: "foundation", Icon: "cluster", Order: 54, Cache: true},
		Fields: []capability.AdminField{
			adminField("code", "任职编码", "Assignment Code", "text", "record", true, false, true, true, true, true, 170, nil),
			adminField("name", "任职名称", "Assignment Name", "text", "record", true, false, true, true, true, true, 180, nil),
			relationAdminField(adminField("personnelCode", "人员", "Personnel", "select", "values", true, false, true, true, true, true, 160, nil), adminFieldRelation("personnel-profiles", "code", "name", false, enabledAdminRelationFilter())),
			relationAdminField(adminField("positionCode", "岗位", "Position", "select", "values", true, false, true, true, true, true, 160, nil), adminFieldRelation("positions", "code", "name", false, enabledAdminRelationFilter())),
			relationAdminField(adminField("tenantCode", "租户", "Tenant", "select", "values", false, false, true, true, true, true, 150, nil), adminFieldRelation("tenants", "code", "name", false, enabledAdminRelationFilter())),
			relationAdminField(adminField("orgUnitCode", "机构", "Org Unit", "select", "values", false, false, true, true, true, true, 160, nil), treeAdminFieldRelation("org-units", "code", "name", "parentCode", enabledAdminRelationFilter())),
			adminField("assignmentType", "任职类型", "Assignment Type", "select", "values", false, false, true, true, true, true, 130, assignmentTypeOptions()),
			adminField("primary", "主岗", "Primary", "switch", "values", false, false, true, true, true, true, 110, nil),
			adminField("startedAt", "开始时间", "Started At", "datetime", "values", false, false, true, true, true, true, 180, nil),
			adminField("endedAt", "结束时间", "Ended At", "datetime", "values", false, false, true, true, true, true, 180, nil),
			adminField("status", "状态", "Status", "select", "record", false, false, true, true, true, true, 120, enabledDisabledOptions()),
			adminField("description", "说明", "Description", "textarea", "record", false, false, false, false, true, true, 220, nil),
			adminField("updatedAt", "更新时间", "Updated At", "datetime", "record", false, true, false, true, false, true, 180, nil),
		},
		SearchFields:   []string{"name", "code", "status", "description", "personnelCode", "positionCode", "tenantCode", "orgUnitCode", "assignmentType", "primary", "startedAt", "endedAt"},
		DefaultSortKey: "startedAt",
	}
}

func notificationTemplateAdminResource() capability.AdminResource {
	return capability.AdminResource{
		Resource:         "notification-templates",
		Title:            capability.Text("通知模板", "Notification Templates"),
		Description:      capability.Text("站内通知模板，供平台能力和业务能力复用。", "In-app notification templates reused by platform and business capabilities."),
		PermissionPrefix: "admin:notification-template",
		Menu:             capability.AdminMenu{Route: "/notification-templates", Parent: "operations", Group: "operations", Icon: "audit", Order: 380, Cache: true},
		Fields: []capability.AdminField{
			adminField("code", "模板编码", "Template Code", "text", "record", true, false, true, true, true, true, 180, nil),
			adminField("name", "模板名称", "Template Name", "text", "record", true, false, true, true, true, true, 180, nil),
			relationAdminField(adminField("tenantCode", "租户", "Tenant", "select", "values", false, false, true, true, true, true, 150, nil), adminFieldRelation("tenants", "code", "name", false, enabledAdminRelationFilter())),
			adminField("channel", "通知渠道", "Channel", "select", "values", true, false, true, true, true, true, 130, notificationChannelOptions()),
			adminField("titleTemplate", "标题模板", "Title Template", "text", "values", true, false, true, true, true, true, 220, nil),
			adminField("bodyTemplate", "内容模板", "Body Template", "textarea", "values", true, false, true, true, true, true, 280, nil),
			adminField("status", "状态", "Status", "select", "record", false, false, true, true, true, true, 120, enabledDisabledOptions()),
			adminField("description", "说明", "Description", "textarea", "record", false, false, false, false, true, true, 220, nil),
			adminField("updatedAt", "更新时间", "Updated At", "datetime", "record", false, true, false, true, false, true, 180, nil),
		},
		SearchFields:   []string{"name", "code", "status", "description", "tenantCode", "channel", "titleTemplate", "bodyTemplate"},
		DefaultSortKey: "updatedAt",
	}
}

func notificationAdminResource() capability.AdminResource {
	return capability.AdminResource{
		Resource:         "notifications",
		Title:            capability.Text("站内通知", "Notifications"),
		Description:      capability.Text("面向用户的站内通知记录，不绑定具体业务领域。", "User-facing in-app notifications without binding to a concrete business domain."),
		PermissionPrefix: "admin:notification",
		Menu:             capability.AdminMenu{Route: "/notifications", Parent: "operations", Group: "operations", Icon: "warning", Order: 381, Cache: true},
		Fields: []capability.AdminField{
			adminField("code", "通知编码", "Notification Code", "text", "record", true, false, true, true, true, true, 180, nil),
			adminField("name", "通知标题", "Notification Title", "text", "record", true, false, true, true, true, true, 220, nil),
			relationAdminField(adminField("tenantCode", "租户", "Tenant", "select", "values", false, false, true, true, true, true, 150, nil), adminFieldRelation("tenants", "code", "name", false, enabledAdminRelationFilter())),
			relationAdminField(adminField("templateCode", "通知模板", "Template", "select", "values", false, false, true, true, true, true, 180, nil), adminFieldRelation("notification-templates", "code", "name", false, enabledAdminRelationFilter())),
			relationAdminField(adminField("recipientUserCode", "接收用户", "Recipient User", "select", "values", false, false, true, true, true, true, 160, nil), adminFieldRelation("users", "code", "name", false, enabledAdminRelationFilter())),
			adminField("category", "通知分类", "Category", "text", "values", false, false, true, true, true, true, 140, nil),
			adminField("priority", "优先级", "Priority", "select", "values", false, false, true, true, true, true, 120, notificationPriorityOptions()),
			adminField("readStatus", "阅读状态", "Read Status", "select", "values", false, false, true, true, true, true, 130, notificationReadStatusOptions()),
			adminField("payload", "负载", "Payload", "textarea", "values", false, false, true, false, true, true, 260, nil),
			adminField("sentAt", "发送时间", "Sent At", "datetime", "values", false, false, true, true, true, true, 180, nil),
			adminField("readAt", "阅读时间", "Read At", "datetime", "values", false, false, true, true, true, true, 180, nil),
			adminField("status", "状态", "Status", "select", "record", false, false, true, true, true, true, 120, notificationStatusOptions()),
			adminField("description", "内容", "Body", "textarea", "record", false, false, false, false, true, true, 280, nil),
			adminField("updatedAt", "更新时间", "Updated At", "datetime", "record", false, true, false, true, false, true, 180, nil),
		},
		SearchFields:   []string{"name", "code", "status", "description", "tenantCode", "templateCode", "recipientUserCode", "category", "priority", "readStatus", "sentAt", "readAt"},
		DefaultSortKey: "sentAt",
	}
}

func notificationDeliveryAdminResource() capability.AdminResource {
	return capability.AdminResource{
		Resource:         "notification-deliveries",
		Title:            capability.Text("通知投递", "Notification Deliveries"),
		Description:      capability.Text("通知投递尝试、结果和错误记录，为外部通道适配预留边界。", "Notification delivery attempts, results, and errors with room for external channel adapters."),
		PermissionPrefix: "admin:notification-delivery",
		Menu:             capability.AdminMenu{Route: "/notification-deliveries", Parent: "operations", Group: "operations", Icon: "desktop", Order: 382, Cache: true},
		Fields: []capability.AdminField{
			adminField("code", "投递编码", "Delivery Code", "text", "record", true, false, true, true, true, true, 180, nil),
			adminField("name", "投递名称", "Delivery Name", "text", "record", true, false, true, true, true, true, 180, nil),
			relationAdminField(adminField("tenantCode", "租户", "Tenant", "select", "values", false, false, true, true, true, true, 150, nil), adminFieldRelation("tenants", "code", "name", false, enabledAdminRelationFilter())),
			relationAdminField(adminField("notificationCode", "通知", "Notification", "select", "values", true, false, true, true, true, true, 180, nil), adminFieldRelation("notifications", "code", "name", false, enabledAdminRelationFilter())),
			relationAdminField(adminField("recipientUserCode", "接收用户", "Recipient User", "select", "values", false, false, true, true, true, true, 160, nil), adminFieldRelation("users", "code", "name", false, enabledAdminRelationFilter())),
			adminField("channel", "通知渠道", "Channel", "select", "values", true, false, true, true, true, true, 130, notificationChannelOptions()),
			adminField("deliveryStatus", "投递状态", "Delivery Status", "select", "values", true, false, true, true, true, true, 130, notificationDeliveryStatusOptions()),
			adminField("attempts", "尝试次数", "Attempts", "number", "values", false, false, true, true, true, true, 110, nil),
			adminField("lastAttemptAt", "最近尝试", "Last Attempt", "datetime", "values", false, false, true, true, true, true, 180, nil),
			adminField("deliveredAt", "投递时间", "Delivered At", "datetime", "values", false, false, true, true, true, true, 180, nil),
			adminField("errorMessage", "错误信息", "Error Message", "textarea", "values", false, false, true, false, true, true, 260, nil),
			adminField("status", "状态", "Status", "select", "record", false, false, true, true, true, true, 120, enabledDisabledOptions()),
			adminField("description", "说明", "Description", "textarea", "record", false, false, false, false, true, true, 220, nil),
			adminField("updatedAt", "更新时间", "Updated At", "datetime", "record", false, true, false, true, false, true, 180, nil),
		},
		SearchFields:   []string{"name", "code", "status", "description", "tenantCode", "notificationCode", "recipientUserCode", "channel", "deliveryStatus", "lastAttemptAt", "deliveredAt", "errorMessage"},
		DefaultSortKey: "lastAttemptAt",
	}
}

func jobDefinitionAdminResource() capability.AdminResource {
	return capability.AdminResource{
		Resource:         "job-definitions",
		Title:            capability.Text("任务定义", "Job Definitions"),
		Description:      capability.Text("可选调度任务定义，声明触发方式、负责人和执行边界。", "Optional scheduled job definitions declaring trigger mode, owner, and execution boundary."),
		PermissionPrefix: "admin:job-definition",
		Menu:             capability.AdminMenu{Route: "/job-definitions", Parent: "operations", Group: "operations", Icon: "cluster", Order: 390, Cache: true},
		Fields: []capability.AdminField{
			adminField("code", "任务编码", "Job Code", "text", "record", true, false, true, true, true, true, 180, nil),
			adminField("name", "任务名称", "Job Name", "text", "record", true, false, true, true, true, true, 200, nil),
			relationAdminField(adminField("tenantCode", "租户", "Tenant", "select", "values", false, false, true, true, true, true, 150, nil), adminFieldRelation("tenants", "code", "name", false, enabledAdminRelationFilter())),
			relationAdminField(adminField("ownerUserCode", "负责人", "Owner", "select", "values", false, false, true, true, true, true, 160, nil), adminFieldRelation("users", "code", "name", false, enabledAdminRelationFilter())),
			adminField("triggerType", "触发方式", "Trigger Type", "select", "values", true, false, true, true, true, true, 130, jobTriggerTypeOptions()),
			adminField("cronExpression", "Cron 表达式", "Cron Expression", "text", "values", false, false, true, true, true, true, 180, nil),
			adminField("timezone", "时区", "Timezone", "text", "values", false, false, true, true, true, true, 130, nil),
			adminField("maxAttempts", "最大尝试", "Max Attempts", "number", "values", false, false, true, true, true, true, 110, nil),
			adminField("timeoutSeconds", "超时秒数", "Timeout Seconds", "number", "values", false, false, true, true, true, true, 120, nil),
			adminField("payloadSchema", "负载 Schema", "Payload Schema", "textarea", "values", false, false, true, false, true, true, 260, nil),
			adminField("status", "状态", "Status", "select", "record", false, false, true, true, true, true, 120, enabledDisabledOptions()),
			adminField("description", "说明", "Description", "textarea", "record", false, false, false, false, true, true, 240, nil),
			adminField("updatedAt", "更新时间", "Updated At", "datetime", "record", false, true, false, true, false, true, 180, nil),
		},
		SearchFields:   []string{"name", "code", "status", "description", "tenantCode", "ownerUserCode", "triggerType", "cronExpression", "timezone"},
		DefaultSortKey: "updatedAt",
	}
}

func jobRunAdminResource() capability.AdminResource {
	return capability.AdminResource{
		Resource:         "job-runs",
		Title:            capability.Text("任务运行", "Job Runs"),
		Description:      capability.Text("调度任务运行记录，保留触发来源、状态和耗时。", "Scheduled job run records with trigger source, status, and duration."),
		PermissionPrefix: "admin:job-run",
		Menu:             capability.AdminMenu{Route: "/job-runs", Parent: "operations", Group: "operations", Icon: "monitoring", Order: 391, Cache: true},
		Fields: []capability.AdminField{
			adminField("code", "运行编码", "Run Code", "text", "record", true, false, true, true, true, true, 180, nil),
			adminField("name", "运行名称", "Run Name", "text", "record", true, false, true, true, true, true, 200, nil),
			relationAdminField(adminField("tenantCode", "租户", "Tenant", "select", "values", false, false, true, true, true, true, 150, nil), adminFieldRelation("tenants", "code", "name", false, enabledAdminRelationFilter())),
			relationAdminField(adminField("jobCode", "任务定义", "Job", "select", "values", true, false, true, true, true, true, 180, nil), adminFieldRelation("job-definitions", "code", "name", false, enabledAdminRelationFilter())),
			relationAdminField(adminField("triggeredBy", "触发用户", "Triggered By", "select", "values", false, false, true, true, true, true, 160, nil), adminFieldRelation("users", "code", "name", false, enabledAdminRelationFilter())),
			adminField("triggerSource", "触发来源", "Trigger Source", "select", "values", true, false, true, true, true, true, 130, jobTriggerSourceOptions()),
			adminField("runStatus", "运行状态", "Run Status", "select", "values", true, false, true, true, true, true, 130, jobRunStatusOptions()),
			adminField("startedAt", "开始时间", "Started At", "datetime", "values", false, false, true, true, true, true, 180, nil),
			adminField("finishedAt", "完成时间", "Finished At", "datetime", "values", false, false, true, true, true, true, 180, nil),
			adminField("durationMs", "耗时毫秒", "Duration Ms", "number", "values", false, false, true, true, true, true, 120, nil),
			adminField("payload", "运行负载", "Payload", "textarea", "values", false, false, true, false, true, true, 260, nil),
			adminField("result", "运行结果", "Result", "textarea", "values", false, false, true, false, true, true, 260, nil),
			adminField("status", "状态", "Status", "select", "record", false, false, true, true, true, true, 120, enabledDisabledOptions()),
			adminField("description", "说明", "Description", "textarea", "record", false, false, false, false, true, true, 240, nil),
			adminField("updatedAt", "更新时间", "Updated At", "datetime", "record", false, true, false, true, false, true, 180, nil),
		},
		SearchFields:   []string{"name", "code", "status", "description", "tenantCode", "jobCode", "triggeredBy", "triggerSource", "runStatus", "startedAt", "finishedAt"},
		DefaultSortKey: "startedAt",
	}
}

func jobRunAttemptAdminResource() capability.AdminResource {
	return capability.AdminResource{
		Resource:         "job-run-attempts",
		Title:            capability.Text("任务尝试", "Job Run Attempts"),
		Description:      capability.Text("任务运行的单次尝试、错误和重试记录。", "Single attempt, error, and retry records for job runs."),
		PermissionPrefix: "admin:job-run-attempt",
		Menu:             capability.AdminMenu{Route: "/job-run-attempts", Parent: "operations", Group: "operations", Icon: "warning", Order: 392, Cache: true},
		Fields: []capability.AdminField{
			adminField("code", "尝试编码", "Attempt Code", "text", "record", true, false, true, true, true, true, 180, nil),
			adminField("name", "尝试名称", "Attempt Name", "text", "record", true, false, true, true, true, true, 200, nil),
			relationAdminField(adminField("tenantCode", "租户", "Tenant", "select", "values", false, false, true, true, true, true, 150, nil), adminFieldRelation("tenants", "code", "name", false, enabledAdminRelationFilter())),
			relationAdminField(adminField("runCode", "运行记录", "Run", "select", "values", true, false, true, true, true, true, 180, nil), adminFieldRelation("job-runs", "code", "name", false, enabledAdminRelationFilter())),
			adminField("attemptNo", "尝试序号", "Attempt No", "number", "values", true, false, true, true, true, true, 110, nil),
			adminField("attemptStatus", "尝试状态", "Attempt Status", "select", "values", true, false, true, true, true, true, 130, jobAttemptStatusOptions()),
			adminField("startedAt", "开始时间", "Started At", "datetime", "values", false, false, true, true, true, true, 180, nil),
			adminField("finishedAt", "完成时间", "Finished At", "datetime", "values", false, false, true, true, true, true, 180, nil),
			adminField("workerId", "执行节点", "Worker ID", "text", "values", false, false, true, true, true, true, 160, nil),
			adminField("errorMessage", "错误信息", "Error Message", "textarea", "values", false, false, true, false, true, true, 260, nil),
			adminField("status", "状态", "Status", "select", "record", false, false, true, true, true, true, 120, enabledDisabledOptions()),
			adminField("description", "说明", "Description", "textarea", "record", false, false, false, false, true, true, 240, nil),
			adminField("updatedAt", "更新时间", "Updated At", "datetime", "record", false, true, false, true, false, true, 180, nil),
		},
		SearchFields:   []string{"name", "code", "status", "description", "tenantCode", "runCode", "attemptNo", "attemptStatus", "startedAt", "finishedAt", "workerId", "errorMessage"},
		DefaultSortKey: "startedAt",
	}
}

func areaCodeAdminResource() capability.AdminResource {
	return capability.AdminResource{
		Resource:         "area-codes",
		Title:            capability.Text("地址码", "Area Codes"),
		Description:      capability.Text("通用行政区划或业务区域编码，供租户、机构和人员引用。", "Common administrative or business area codes referenced by tenants, org units, and users."),
		PermissionPrefix: "admin:area-code",
		Menu:             capability.AdminMenu{Route: "/area-codes", Parent: "configuration", Group: "governance", Icon: "cluster", Order: 135, Cache: true},
		Fields: []capability.AdminField{
			adminField("code", "地址码", "Area Code", "text", "record", true, false, true, true, true, true, 150, nil),
			adminField("name", "区域名称", "Area Name", "text", "record", true, false, true, true, true, true, 180, nil),
			relationAdminField(adminField("parentCode", "上级地址码", "Parent Code", "select", "values", false, false, true, true, true, true, 150, nil), areaCodeAdminFieldRelation(enabledAdminRelationFilter())),
			adminField("level", "层级", "Level", "select", "values", false, false, true, true, true, true, 130, areaLevelOptions()),
			adminField("path", "层级路径", "Path", "text", "values", false, false, true, true, true, true, 220, nil),
			adminField("sortOrder", "排序", "Sort Order", "number", "values", false, false, false, true, true, true, 110, nil),
			adminField("status", "状态", "Status", "select", "record", false, false, true, true, true, true, 120, enabledDisabledOptions()),
			adminField("description", "说明", "Description", "textarea", "record", false, false, false, false, true, true, 220, nil),
			adminField("updatedAt", "更新时间", "Updated At", "datetime", "record", false, true, false, true, false, true, 180, nil),
		},
		SearchFields:   []string{"name", "code", "status", "description", "parentCode", "level", "path"},
		DefaultSortKey: "sortOrder",
	}
}

func dictionaryAdminResource() capability.AdminResource {
	return capability.AdminResource{
		Resource:         "dictionaries",
		Title:            capability.Text("字典管理", "Dictionaries"),
		Description:      capability.Text("字典目录、枚举分类和业务选项分组。", "Dictionary catalogs, enum categories, and business option groups."),
		PermissionPrefix: "admin:dictionary",
		Menu:             capability.AdminMenu{Route: "/dictionaries", Parent: "configuration", Group: "governance", Icon: "dictParams", Order: 128, Cache: true},
		Fields: []capability.AdminField{
			adminField("code", "字典编码", "Dictionary Code", "text", "record", true, false, true, true, true, true, 160, nil),
			adminField("name", "字典名称", "Dictionary Name", "text", "record", true, false, true, true, true, true, 180, nil),
			adminField("itemCount", "条目数", "Items", "number", "values", false, true, false, true, false, true, 110, nil),
			adminField("status", "状态", "Status", "select", "record", false, false, true, true, true, true, 120, enabledDisabledOptions()),
			adminField("description", "说明", "Description", "textarea", "record", false, false, false, false, true, true, 220, nil),
			adminField("updatedAt", "更新时间", "Updated At", "datetime", "record", false, true, false, true, false, true, 180, nil),
		},
		SearchFields:   []string{"name", "code", "status", "description"},
		DefaultSortKey: "updatedAt",
	}
}

func dictionaryParameterAdminResource() capability.AdminResource {
	return capability.AdminResource{
		Resource:         "dictionary-parameters",
		Title:            capability.Text("字典参数", "Dictionary Parameters"),
		Description:      capability.Text("字典项、枚举值和可复用配置选项。", "Dictionary entries, enum values, and reusable configuration options."),
		PermissionPrefix: "admin:dictionary-parameter",
		Menu:             capability.AdminMenu{Route: "/dictionary-parameters", Parent: "configuration", Group: "governance", Icon: "dictParams", Order: 130, Cache: true},
		Fields: []capability.AdminField{
			adminField("code", "参数编码", "Parameter Code", "text", "record", true, false, true, true, true, true, 170, nil),
			adminField("name", "参数名称", "Parameter Name", "text", "record", true, false, true, true, true, true, 180, nil),
			relationAdminField(adminField("dictionaryCode", "所属字典", "Dictionary", "select", "values", true, false, true, true, true, true, 160, nil), adminFieldRelation("dictionaries", "code", "name", false, enabledAdminRelationFilter())),
			adminField("value", "参数值", "Value", "text", "values", true, false, true, true, true, true, 180, nil),
			adminField("valueType", "值类型", "Value Type", "select", "values", false, false, true, true, true, true, 120, valueTypeOptions()),
			adminField("sortOrder", "排序", "Sort Order", "number", "values", false, false, false, true, true, true, 110, nil),
			adminField("status", "状态", "Status", "select", "record", false, false, true, true, true, true, 120, enabledDisabledOptions()),
			adminField("description", "说明", "Description", "textarea", "record", false, false, false, false, true, true, 220, nil),
			adminField("updatedAt", "更新时间", "Updated At", "datetime", "record", false, true, false, true, false, true, 180, nil),
		},
		SearchFields:   []string{"name", "code", "status", "description", "dictionaryCode", "value", "valueType"},
		DefaultSortKey: "sortOrder",
	}
}

func parameterAdminResource() capability.AdminResource {
	return capability.AdminResource{
		Resource:         "parameters",
		Title:            capability.Text("参数管理", "Parameters"),
		Description:      capability.Text("平台参数、运行开关和能力配置键值。", "Platform parameters, runtime switches, and capability configuration key-values."),
		PermissionPrefix: "admin:parameter",
		Menu:             capability.AdminMenu{Route: "/parameters", Parent: "configuration", Group: "governance", Icon: "dictParams", Order: 132, Cache: true},
		Fields: []capability.AdminField{
			adminField("code", "参数键", "Parameter Key", "text", "record", true, false, true, true, true, true, 180, nil),
			adminField("name", "参数名称", "Parameter Name", "text", "record", true, false, true, true, true, true, 180, nil),
			adminField("value", "参数值", "Value", "text", "values", true, false, true, true, true, true, 220, nil),
			adminField("group", "分组", "Group", "text", "values", false, false, true, true, true, true, 140, nil),
			adminField("status", "状态", "Status", "select", "record", false, false, true, true, true, true, 120, enabledDisabledOptions()),
			adminField("description", "说明", "Description", "textarea", "record", false, false, false, false, true, true, 220, nil),
			adminField("updatedAt", "更新时间", "Updated At", "datetime", "record", false, true, false, true, false, true, 180, nil),
		},
		SearchFields:   []string{"name", "code", "status", "description", "value", "group"},
		DefaultSortKey: "updatedAt",
	}
}

func brandingAdminResource() capability.AdminResource {
	return capability.AdminResource{
		Resource:         "branding",
		Title:            capability.Text("品牌配置", "Branding"),
		Description:      capability.Text("平台品牌、主题和登录展示配置。", "Platform branding, theme, and login presentation configuration."),
		PermissionPrefix: "admin:branding",
		Menu:             capability.AdminMenu{Route: "/branding", Parent: "configuration", Group: "foundation", Icon: "settings", Order: 308, Cache: true},
		Fields: []capability.AdminField{
			adminField("name", "配置名称", "Config Name", "text", "record", true, false, true, true, true, true, 180, nil),
			adminField("code", "配置编码", "Config Code", "text", "record", true, false, true, false, false, true, 160, nil),
			adminField("productName", "产品名称", "Product Name", "text", "values", true, false, true, true, true, true, 180, nil),
			adminField("shortName", "简称", "Short Name", "text", "values", false, false, true, true, true, true, 120, nil),
			adminField("logoUrl", "Logo URL", "Logo URL", "text", "values", false, false, false, false, true, true, 220, nil),
			adminField("faviconUrl", "Favicon URL", "Favicon URL", "text", "values", false, false, false, false, true, true, 220, nil),
			adminField("primaryColor", "主色", "Primary Color", "color", "values", false, false, true, true, true, true, 120, nil),
			adminField("defaultTheme", "默认主题", "Default Theme", "select", "values", true, false, true, true, true, true, 140, []capability.AdminFieldOption{
				adminFieldOption("tech", "科技风", "Tech"),
				adminFieldOption("white", "高级白", "Premium White"),
				adminFieldOption("black", "炫酷黑", "Cool Black"),
				adminFieldOption("warm", "温暖黄", "Warm Yellow"),
			}),
			adminField("loginTitle", "登录标题", "Login Title", "text", "values", false, false, true, false, true, true, 220, nil),
			adminField("loginSubtitle", "登录副标题", "Login Subtitle", "textarea", "values", false, false, true, false, true, true, 260, nil),
			adminField("updatedAt", "更新时间", "Updated At", "datetime", "record", false, true, false, true, false, true, 180, nil),
		},
		SearchFields:   []string{"name", "code", "productName", "shortName", "defaultTheme", "loginTitle"},
		DefaultSortKey: "updatedAt",
	}
}

func fileStorageAdminResource() capability.AdminResource {
	return capability.AdminResource{
		Resource:         "files",
		Title:            capability.Text("文件管理", "Files"),
		Description:      capability.Text("文件对象、存储位置和下载治理。", "File objects, storage locations, and download governance."),
		PermissionPrefix: "admin:file",
		Menu: capability.AdminMenu{
			Route:  "/files",
			Parent: "storage",
			Group:  "operations",
			Icon:   "upload",
			Order:  340,
			Cache:  true,
		},
		Fields: []capability.AdminField{
			adminField("name", "文件名", "Filename", "text", "record", true, false, true, true, true, true, 220, nil),
			adminField("code", "文件编码", "File Code", "text", "record", false, true, true, false, false, true, 240, nil),
			adminField("status", "状态", "Status", "select", "record", false, false, true, true, true, true, 120, []capability.AdminFieldOption{
				adminFieldOption("enabled", "已启用", "Enabled"),
				adminFieldOption("disabled", "已停用", "Disabled"),
			}),
			adminField("description", "描述", "Description", "textarea", "record", false, false, true, false, true, true, 220, nil),
			adminField("mimeType", "类型", "MIME Type", "text", "values", false, false, true, true, true, true, 160, nil),
			adminField("size", "大小", "Size", "number", "values", false, true, true, true, false, true, 120, nil),
			adminField("storageDriver", "存储驱动", "Storage Driver", "text", "values", false, true, true, false, false, true, 140, nil),
			secureAdminField(adminField("storageKey", "对象键", "Object Key", "text", "values", false, true, false, false, false, false, 260, nil), capability.FieldSensitivityInternal, capability.FieldStoragePlain, capability.FieldProjectionOmitted, capability.FieldProjectionOmitted),
			secureAdminField(adminField("tenantId", "租户 ID", "Tenant ID", "text", "values", false, true, false, false, false, false, 180, nil), capability.FieldSensitivityInternal, capability.FieldStoragePlain, capability.FieldProjectionOmitted, capability.FieldProjectionOmitted),
			secureAdminField(adminField("ownerId", "所有者 ID", "Owner ID", "text", "values", false, true, false, false, false, false, 180, nil), capability.FieldSensitivityInternal, capability.FieldStoragePlain, capability.FieldProjectionOmitted, capability.FieldProjectionOmitted),
			secureAdminField(adminField("uploadedBy", "上传人", "Uploaded By", "text", "values", false, true, false, false, false, true, 180, nil), capability.FieldSensitivityInternal, capability.FieldStoragePlain, capability.FieldProjectionFull, capability.FieldProjectionFull),
			secureAdminField(adminField("deletionState", "删除状态", "Deletion State", "text", "values", false, true, false, false, false, false, 140, nil), capability.FieldSensitivityInternal, capability.FieldStoragePlain, capability.FieldProjectionOmitted, capability.FieldProjectionOmitted),
			secureAdminField(adminField("deletionRequestedAt", "删除请求时间", "Deletion Requested At", "datetime", "values", false, true, false, false, false, false, 180, nil), capability.FieldSensitivityInternal, capability.FieldStoragePlain, capability.FieldProjectionOmitted, capability.FieldProjectionOmitted),
			adminField("createdAt", "上传时间", "Created At", "datetime", "values", false, true, true, true, false, true, 180, nil),
			adminField("updatedAt", "更新时间", "Updated At", "datetime", "record", false, true, false, true, false, true, 180, nil),
		},
		SearchFields:   []string{"name", "code", "description", "mimeType", "storageDriver", "createdAt"},
		DefaultSortKey: "createdAt",
	}
}

func appIdentityAdminResource() capability.AdminResource {
	return capability.AdminResource{
		Resource:         "app-identities",
		Title:            capability.Text("App 身份绑定", "App Identities"),
		Description:      capability.Text("App 登录 provider 与平台 App 用户的安全绑定。", "Safe bindings between app login providers and platform app users."),
		PermissionPrefix: "admin:app-identity",
		Menu: capability.AdminMenu{
			Route:  "/app-identities",
			Parent: "identity",
			Group:  "foundation",
			Icon:   "user",
			Order:  45,
			Cache:  true,
		},
		Fields: []capability.AdminField{
			adminField("name", "名称", "Name", "text", "record", true, false, true, true, false, true, 180, nil),
			adminField("code", "绑定编码", "Binding Code", "text", "record", true, true, true, false, false, true, 190, nil),
			adminField("status", "状态", "Status", "select", "record", false, false, true, true, false, true, 110, []capability.AdminFieldOption{
				adminFieldOption("enabled", "已启用", "Enabled"),
				adminFieldOption("disabled", "已停用", "Disabled"),
			}),
			adminField("provider", "登录方式", "Provider", "text", "values", true, true, true, true, false, true, 120, nil),
			adminField("providerKind", "方式类型", "Provider Kind", "text", "values", true, true, true, true, false, true, 120, nil),
			adminField("providerScope", "作用域", "Scope", "text", "values", true, true, true, true, false, true, 140, nil),
			secureAdminField(adminField("maskedSubject", "脱敏标识", "Masked Subject", "text", "values", true, true, true, true, false, true, 180, nil), capability.FieldSensitivityPersonal, capability.FieldStorageMasked, capability.FieldProjectionMasked, capability.FieldProjectionMasked),
			secureAdminField(adminField("providerSubjectHash", "标识哈希", "Subject Hash", "text", "values", true, true, false, false, false, true, 260, nil), capability.FieldSensitivitySecret, capability.FieldStorageHashed, capability.FieldProjectionOmitted, capability.FieldProjectionOmitted),
			adminField("appUsername", "App 用户", "App User", "text", "values", true, true, true, true, false, true, 180, nil),
			adminField("createdAt", "创建时间", "Created At", "datetime", "values", false, true, true, true, false, true, 180, nil),
			adminField("lastLoginAt", "最近登录", "Last Login", "datetime", "values", false, true, true, true, false, true, 180, nil),
			adminField("description", "描述", "Description", "textarea", "record", false, false, false, false, false, true, 220, nil),
		},
		SearchFields:   []string{"name", "code", "status", "provider", "providerKind", "providerScope", "maskedSubject", "appUsername", "createdAt", "lastLoginAt"},
		DefaultSortKey: "lastLoginAt",
	}
}

func adminIdentityAdminResource() capability.AdminResource {
	return capability.AdminResource{
		Resource:         "admin-identities",
		Title:            capability.Text("后台身份绑定", "Admin Identities"),
		Description:      capability.Text("后台登录 provider 与平台管理员的脱敏安全绑定。", "Privacy-safe bindings between Admin login providers and platform administrators."),
		PermissionPrefix: "admin:admin-identity",
		Menu: capability.AdminMenu{
			Route:  "/admin-identities",
			Parent: "identity",
			Group:  "foundation",
			Icon:   "user",
			Order:  47,
			Cache:  true,
		},
		Fields: []capability.AdminField{
			adminField("provider", "登录方式", "Provider", "text", "values", true, true, true, true, false, true, 120, nil),
			adminField("providerKind", "方式类型", "Provider Kind", "text", "values", true, true, true, true, false, true, 120, nil),
			secureAdminField(adminField("issuerHash", "签发方哈希", "Issuer Hash", "text", "values", true, true, false, false, false, false, 260, nil), capability.FieldSensitivitySecret, capability.FieldStorageHashed, capability.FieldProjectionOmitted, capability.FieldProjectionOmitted),
			secureAdminField(adminField("providerSubjectHash", "标识哈希", "Subject Hash", "text", "values", true, true, false, false, false, true, 260, nil), capability.FieldSensitivitySecret, capability.FieldStorageHashed, capability.FieldProjectionOmitted, capability.FieldProjectionOmitted),
			adminField("platformUsername", "平台管理员", "Platform Administrator", "text", "values", true, true, true, true, false, true, 180, nil),
			adminField("createdAt", "创建时间", "Created At", "datetime", "values", false, true, true, true, false, true, 180, nil),
			adminField("lastLoginAt", "最近登录", "Last Login", "datetime", "values", false, true, true, true, false, true, 180, nil),
		},
		SearchFields:   []string{"provider", "providerKind", "platformUsername", "createdAt", "lastLoginAt"},
		DefaultSortKey: "lastLoginAt",
	}
}

func appPhoneVerificationAdminResource() capability.AdminResource {
	return capability.AdminResource{
		Resource:         "app-phone-verifications",
		Title:            capability.Text("App 手机验证", "App Phone Verifications"),
		Description:      capability.Text("App 手机验证码请求记录，只保存脱敏手机号、手机号哈希和验证码哈希。", "App phone verification requests storing only masked phone, phone hash, and code hash."),
		PermissionPrefix: "admin:app-phone-verification",
		Menu: capability.AdminMenu{
			Route:  "/app-phone-verifications",
			Parent: "identity",
			Group:  "foundation",
			Icon:   "key",
			Order:  46,
			Cache:  true,
		},
		Fields: []capability.AdminField{
			adminField("code", "验证编码", "Verification Code", "text", "record", true, true, true, false, false, true, 190, nil),
			adminField("name", "名称", "Name", "text", "record", true, false, true, true, false, true, 180, nil),
			adminField("status", "状态", "Status", "select", "record", false, false, true, true, false, true, 110, []capability.AdminFieldOption{
				adminFieldOption("pending", "待验证", "Pending"),
				adminFieldOption("verified", "已验证", "Verified"),
				adminFieldOption("expired", "已过期", "Expired"),
			}),
			adminField("appUsername", "App 用户", "App User", "text", "values", true, true, true, true, false, true, 180, nil),
			secureAdminField(adminField("maskedPhone", "脱敏手机号", "Masked Phone", "text", "values", true, true, true, true, false, true, 150, nil), capability.FieldSensitivityPersonal, capability.FieldStorageMasked, capability.FieldProjectionMasked, capability.FieldProjectionMasked),
			secureAdminField(adminField("phoneHash", "手机号哈希", "Phone Hash", "text", "values", true, true, false, false, false, true, 260, nil), capability.FieldSensitivitySensitive, capability.FieldStorageHashed, capability.FieldProjectionOmitted, capability.FieldProjectionOmitted),
			adminField("purpose", "用途", "Purpose", "select", "values", true, true, true, true, false, true, 120, []capability.AdminFieldOption{
				adminFieldOption("bind", "绑定", "Bind"),
			}),
			secureAdminField(adminField("codeHash", "验证码哈希", "Code Hash", "text", "values", true, true, false, false, false, true, 260, nil), capability.FieldSensitivitySecret, capability.FieldStorageHashed, capability.FieldProjectionOmitted, capability.FieldProjectionOmitted),
			adminField("requestedAt", "请求时间", "Requested At", "datetime", "values", false, true, true, true, false, true, 180, nil),
			adminField("expiresAt", "过期时间", "Expires At", "datetime", "values", false, true, true, true, false, true, 180, nil),
			adminField("verifiedAt", "验证时间", "Verified At", "datetime", "values", false, true, true, false, false, true, 180, nil),
		},
		SearchFields:   []string{"name", "code", "status", "appUsername", "maskedPhone", "purpose", "requestedAt", "expiresAt", "verifiedAt"},
		DefaultSortKey: "requestedAt",
	}
}

func appPhoneBindingAdminResource() capability.AdminResource {
	return capability.AdminResource{
		Resource:         "app-phone-bindings",
		Title:            capability.Text("App 手机绑定", "App Phone Bindings"),
		Description:      capability.Text("App 用户与手机号的安全绑定，只保存脱敏手机号和手机号哈希。", "Safe bindings between app users and phone numbers storing only masked phone and phone hash."),
		PermissionPrefix: "admin:app-phone-binding",
		Menu: capability.AdminMenu{
			Route:  "/app-phone-bindings",
			Parent: "identity",
			Group:  "foundation",
			Icon:   "user",
			Order:  47,
			Cache:  true,
		},
		Fields: []capability.AdminField{
			adminField("code", "绑定编码", "Binding Code", "text", "record", true, true, true, false, false, true, 190, nil),
			adminField("name", "名称", "Name", "text", "record", true, false, true, true, false, true, 180, nil),
			adminField("status", "状态", "Status", "select", "record", false, false, true, true, false, true, 110, []capability.AdminFieldOption{
				adminFieldOption("enabled", "已启用", "Enabled"),
				adminFieldOption("disabled", "已停用", "Disabled"),
			}),
			adminField("appUsername", "App 用户", "App User", "text", "values", true, true, true, true, false, true, 180, nil),
			secureAdminField(adminField("maskedPhone", "脱敏手机号", "Masked Phone", "text", "values", true, true, true, true, false, true, 150, nil), capability.FieldSensitivityPersonal, capability.FieldStorageMasked, capability.FieldProjectionMasked, capability.FieldProjectionMasked),
			secureAdminField(adminField("phoneHash", "手机号哈希", "Phone Hash", "text", "values", true, true, false, false, false, true, 260, nil), capability.FieldSensitivitySensitive, capability.FieldStorageHashed, capability.FieldProjectionOmitted, capability.FieldProjectionOmitted),
			adminField("boundAt", "绑定时间", "Bound At", "datetime", "values", false, true, true, true, false, true, 180, nil),
			adminField("verificationId", "验证记录", "Verification", "text", "values", false, true, false, false, false, true, 180, nil),
		},
		SearchFields:   []string{"name", "code", "status", "appUsername", "maskedPhone", "boundAt"},
		DefaultSortKey: "boundAt",
	}
}

func monitoringAdminResource() capability.AdminResource {
	return capability.AdminResource{
		Resource:         "monitoring",
		Title:            capability.Text("监控", "Monitoring"),
		Description:      capability.Text("实例、服务、健康状态和告警摘要。", "Instances, services, health state, and alert summaries."),
		PermissionPrefix: "admin:monitoring",
		Menu:             capability.AdminMenu{Route: "/monitoring", Parent: "runtime", Group: "operations", Icon: "monitoring", Order: 210, Cache: true},
		Fields: []capability.AdminField{
			adminField("code", "监控编码", "Monitor Code", "text", "record", true, false, true, true, true, true, 170, nil),
			adminField("name", "监控名称", "Monitor Name", "text", "record", true, false, true, true, true, true, 180, nil),
			adminField("targetType", "目标类型", "Target Type", "select", "values", true, false, true, true, true, true, 130, monitoringTargetTypeOptions()),
			adminField("health", "健康状态", "Health", "select", "values", false, true, true, true, false, true, 120, healthStatusOptions()),
			adminField("endpoint", "端点", "Endpoint", "text", "values", false, false, true, true, true, true, 220, nil),
			adminField("lastSeenAt", "最近上报", "Last Seen", "datetime", "values", false, true, true, true, false, true, 180, nil),
			adminField("alertCount", "告警数", "Alerts", "number", "values", false, true, false, true, false, true, 110, nil),
			adminField("status", "状态", "Status", "select", "record", false, false, true, true, true, true, 120, enabledDisabledOptions()),
			adminField("description", "说明", "Description", "textarea", "record", false, false, false, false, true, true, 220, nil),
			adminField("updatedAt", "更新时间", "Updated At", "datetime", "record", false, true, false, true, false, true, 180, nil),
		},
		SearchFields:   []string{"name", "code", "status", "description", "targetType", "health", "endpoint", "lastSeenAt"},
		DefaultSortKey: "lastSeenAt",
	}
}

func apiTokenAdminResource() capability.AdminResource {
	return capability.AdminResource{
		Resource:         "api-tokens",
		Title:            capability.Text("API Token", "API Tokens"),
		Description:      capability.Text("平台集成调用令牌，只在签发时返回明文。", "Platform integration tokens; secret is returned once when issued."),
		PermissionPrefix: "admin:api-token",
		Menu: capability.AdminMenu{
			Route:  "/api-tokens",
			Parent: "security",
			Group:  "operations",
			Icon:   "key",
			Order:  370,
			Cache:  true,
		},
		Fields: []capability.AdminField{
			adminField("name", "名称", "Name", "text", "record", true, false, true, true, true, true, 180, nil),
			adminField("status", "状态", "Status", "select", "record", false, false, true, true, false, true, 110, []capability.AdminFieldOption{
				adminFieldOption("active", "有效", "Active"),
				adminFieldOption("revoked", "已撤销", "Revoked"),
				adminFieldOption("expired", "已过期", "Expired"),
			}),
			adminField("scope", "作用域", "Scope", "text", "values", true, false, true, true, true, true, 260, nil),
			adminField("tokenPrefix", "令牌前缀", "Token Prefix", "text", "values", false, true, true, true, false, true, 140, nil),
			secureAdminField(adminField("tokenHash", "令牌哈希", "Token Hash", "text", "values", false, true, false, false, false, false, 260, nil), capability.FieldSensitivitySecret, capability.FieldStorageHashed, capability.FieldProjectionOmitted, capability.FieldProjectionOmitted),
			adminField("expiresAt", "过期时间", "Expires At", "datetime", "values", false, false, true, true, true, true, 180, nil),
			adminField("createdAt", "签发时间", "Created At", "datetime", "values", false, true, true, true, false, true, 180, nil),
			adminField("revokedAt", "撤销时间", "Revoked At", "datetime", "values", false, true, true, false, false, true, 180, nil),
		},
		SearchFields:   []string{"name", "status", "scope", "tokenPrefix", "expiresAt"},
		DefaultSortKey: "createdAt",
	}
}

func adminField(key string, labelZH string, labelEN string, fieldType string, source string, required bool, readOnly bool, searchable bool, inTable bool, inForm bool, inDetail bool, width int, options []capability.AdminFieldOption) capability.AdminField {
	return capability.AdminField{
		Key:          key,
		Label:        capability.Text(labelZH, labelEN),
		Type:         fieldType,
		Source:       source,
		Required:     required,
		ReadOnly:     readOnly,
		Searchable:   searchable,
		InTable:      inTable,
		InForm:       inForm,
		InDetail:     inDetail,
		Width:        width,
		Options:      options,
		Sensitivity:  capability.FieldSensitivityPublic,
		StorageMode:  capability.FieldStoragePlain,
		ResponseMode: capability.FieldProjectionFull,
		ExportMode:   capability.FieldProjectionFull,
	}
}

func secureAdminField(field capability.AdminField, sensitivity string, storageMode string, responseMode string, exportMode string) capability.AdminField {
	field.Sensitivity = sensitivity
	field.StorageMode = storageMode
	field.ResponseMode = responseMode
	field.ExportMode = exportMode
	return field
}

func relationAdminField(field capability.AdminField, relation capability.AdminFieldRelation) capability.AdminField {
	field.Relation = &relation
	return field
}

func adminFieldRelation(resource string, valueField string, labelField string, multiple bool, filters ...capability.AdminFieldRelationFilter) capability.AdminFieldRelation {
	return capability.AdminFieldRelation{
		Resource:   resource,
		ValueField: valueField,
		LabelField: labelField,
		Multiple:   multiple,
		Filters:    filters,
		SortField:  labelField,
		SortOrder:  "asc",
	}
}

func treeAdminFieldRelation(resource string, valueField string, labelField string, parentField string, filters ...capability.AdminFieldRelationFilter) capability.AdminFieldRelation {
	relation := adminFieldRelation(resource, valueField, labelField, false, filters...)
	relation.Display = "tree"
	relation.ParentField = parentField
	return relation
}

func treeMultiAdminFieldRelation(resource string, valueField string, labelField string, parentField string, filters ...capability.AdminFieldRelationFilter) capability.AdminFieldRelation {
	relation := adminFieldRelation(resource, valueField, labelField, true, filters...)
	relation.Display = "tree"
	relation.ParentField = parentField
	return relation
}

func areaCodeAdminFieldRelation(filters ...capability.AdminFieldRelationFilter) capability.AdminFieldRelation {
	relation := treeAdminFieldRelation("area-codes", "code", "name", "parentCode", filters...)
	relation.PathField = "path"
	return relation
}

func areaCodeMultiAdminFieldRelation(filters ...capability.AdminFieldRelationFilter) capability.AdminFieldRelation {
	relation := treeMultiAdminFieldRelation("area-codes", "code", "name", "parentCode", filters...)
	relation.PathField = "path"
	return relation
}

func enabledAdminRelationFilter() capability.AdminFieldRelationFilter {
	return capability.AdminFieldRelationFilter{Field: "status", Operator: "=", Value: "enabled"}
}

func adminFieldOption(value string, labelZH string, labelEN string) capability.AdminFieldOption {
	return capability.AdminFieldOption{Value: value, Label: capability.Text(labelZH, labelEN)}
}

func enabledDisabledOptions() []capability.AdminFieldOption {
	return []capability.AdminFieldOption{
		adminFieldOption("enabled", "已启用", "Enabled"),
		adminFieldOption("disabled", "已停用", "Disabled"),
	}
}

func orgUnitTypeOptions() []capability.AdminFieldOption {
	return []capability.AdminFieldOption{
		adminFieldOption("group", "集团", "Group"),
		adminFieldOption("company", "公司", "Company"),
		adminFieldOption("branch", "分支机构", "Branch"),
		adminFieldOption("organization", "机构", "Organization"),
		adminFieldOption("department", "部门", "Department"),
		adminFieldOption("team", "团队", "Team"),
		adminFieldOption("store", "门店", "Store"),
		adminFieldOption("custom", "自定义", "Custom"),
	}
}

func areaLevelOptions() []capability.AdminFieldOption {
	return []capability.AdminFieldOption{
		adminFieldOption("continent", "洲/大区", "Continent / Region"),
		adminFieldOption("country", "国家", "Country"),
		adminFieldOption("subdivision", "一级行政区", "Subdivision"),
		adminFieldOption("state", "州/邦", "State"),
		adminFieldOption("province", "省/直辖市", "Province"),
		adminFieldOption("city", "城市", "City"),
		adminFieldOption("district", "区县", "District"),
		adminFieldOption("street", "街道", "Street"),
		adminFieldOption("custom", "自定义", "Custom"),
	}
}

func valueTypeOptions() []capability.AdminFieldOption {
	return []capability.AdminFieldOption{
		adminFieldOption("text", "文本", "Text"),
		adminFieldOption("number", "数字", "Number"),
		adminFieldOption("boolean", "布尔", "Boolean"),
		adminFieldOption("json", "JSON", "JSON"),
	}
}

func monitoringTargetTypeOptions() []capability.AdminFieldOption {
	return []capability.AdminFieldOption{
		adminFieldOption("instance", "实例", "Instance"),
		adminFieldOption("service", "服务", "Service"),
		adminFieldOption("endpoint", "端点", "Endpoint"),
		adminFieldOption("job", "任务", "Job"),
		adminFieldOption("queue", "队列", "Queue"),
		adminFieldOption("custom", "自定义", "Custom"),
	}
}

func healthStatusOptions() []capability.AdminFieldOption {
	return []capability.AdminFieldOption{
		adminFieldOption("healthy", "健康", "Healthy"),
		adminFieldOption("warning", "预警", "Warning"),
		adminFieldOption("error", "异常", "Error"),
		adminFieldOption("unknown", "未知", "Unknown"),
	}
}

func dataScopeOptions() []capability.AdminFieldOption {
	return []capability.AdminFieldOption{
		adminFieldOption("all", "全部数据", "All Data"),
		adminFieldOption("current_org", "当前机构", "Current Org"),
		adminFieldOption("current_and_children", "当前及下级机构", "Current And Children"),
		adminFieldOption("custom_orgs", "自定义机构", "Custom Orgs"),
		adminFieldOption("current_area", "当前区域", "Current Area"),
		adminFieldOption("current_and_children_areas", "当前及下级区域", "Current And Children Areas"),
		adminFieldOption("custom_areas", "自定义区域", "Custom Areas"),
		adminFieldOption("self", "仅本人", "Self"),
	}
}

func personnelTypeOptions() []capability.AdminFieldOption {
	return []capability.AdminFieldOption{
		adminFieldOption("employee", "员工", "Employee"),
		adminFieldOption("contractor", "外协", "Contractor"),
		adminFieldOption("partner", "合作人员", "Partner"),
	}
}

func employmentStatusOptions() []capability.AdminFieldOption {
	return []capability.AdminFieldOption{
		adminFieldOption("active", "在职", "Active"),
		adminFieldOption("inactive", "停用", "Inactive"),
		adminFieldOption("left", "离职", "Left"),
	}
}

func assignmentTypeOptions() []capability.AdminFieldOption {
	return []capability.AdminFieldOption{
		adminFieldOption("primary", "主岗", "Primary"),
		adminFieldOption("part_time", "兼职", "Part-time"),
		adminFieldOption("temporary", "临时", "Temporary"),
	}
}

func notificationChannelOptions() []capability.AdminFieldOption {
	return []capability.AdminFieldOption{
		adminFieldOption("in_app", "站内", "In-App"),
	}
}

func notificationPriorityOptions() []capability.AdminFieldOption {
	return []capability.AdminFieldOption{
		adminFieldOption("low", "低", "Low"),
		adminFieldOption("normal", "普通", "Normal"),
		adminFieldOption("high", "高", "High"),
	}
}

func notificationReadStatusOptions() []capability.AdminFieldOption {
	return []capability.AdminFieldOption{
		adminFieldOption("unread", "未读", "Unread"),
		adminFieldOption("read", "已读", "Read"),
	}
}

func notificationStatusOptions() []capability.AdminFieldOption {
	return []capability.AdminFieldOption{
		adminFieldOption("draft", "草稿", "Draft"),
		adminFieldOption("sent", "已发送", "Sent"),
		adminFieldOption("archived", "已归档", "Archived"),
	}
}

func notificationDeliveryStatusOptions() []capability.AdminFieldOption {
	return []capability.AdminFieldOption{
		adminFieldOption("pending", "待投递", "Pending"),
		adminFieldOption("delivered", "已投递", "Delivered"),
		adminFieldOption("failed", "失败", "Failed"),
	}
}

func jobTriggerTypeOptions() []capability.AdminFieldOption {
	return []capability.AdminFieldOption{
		adminFieldOption("manual", "手动", "Manual"),
		adminFieldOption("cron", "Cron", "Cron"),
		adminFieldOption("event", "事件", "Event"),
	}
}

func jobTriggerSourceOptions() []capability.AdminFieldOption {
	return []capability.AdminFieldOption{
		adminFieldOption("manual", "手动", "Manual"),
		adminFieldOption("scheduler", "调度器", "Scheduler"),
		adminFieldOption("api", "API", "API"),
		adminFieldOption("event", "事件", "Event"),
	}
}

func jobRunStatusOptions() []capability.AdminFieldOption {
	return []capability.AdminFieldOption{
		adminFieldOption("queued", "排队中", "Queued"),
		adminFieldOption("running", "运行中", "Running"),
		adminFieldOption("succeeded", "已成功", "Succeeded"),
		adminFieldOption("failed", "已失败", "Failed"),
		adminFieldOption("cancelled", "已取消", "Cancelled"),
	}
}

func jobAttemptStatusOptions() []capability.AdminFieldOption {
	return []capability.AdminFieldOption{
		adminFieldOption("running", "运行中", "Running"),
		adminFieldOption("succeeded", "已成功", "Succeeded"),
		adminFieldOption("failed", "已失败", "Failed"),
		adminFieldOption("skipped", "已跳过", "Skipped"),
	}
}
