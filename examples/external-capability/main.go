package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/GnosiST/platform-go/pkg/platform/capability"
)

type ContractPreview struct {
	CapabilityID           string                 `json:"capabilityId"`
	Version                string                 `json:"version"`
	AdminResources         []string               `json:"adminResources"`
	AdminResourceContracts []AdminResourcePreview `json:"adminResourceContracts"`
	PermissionPrefixes     []string               `json:"permissionPrefixes"`
	ConfigResources        []string               `json:"configResources"`
	ConfigKeys             []string               `json:"configKeys"`
	AppRoutes              []AppRoutePreview      `json:"appRoutes"`
	DemoDataSets           []string               `json:"demoDataSets"`
	Migrations             []string               `json:"migrations"`
	Seeds                  []string               `json:"seeds"`
	ServiceContractHash    string                 `json:"serviceContractHash"`
	ServiceCount           int                    `json:"serviceCount"`
}

type AdminResourcePreview struct {
	Resource         string `json:"resource"`
	PermissionPrefix string `json:"permissionPrefix"`
	MenuRoute        string `json:"menuRoute,omitempty"`
	MenuParent       string `json:"menuParent,omitempty"`
}

type AppRoutePreview struct {
	Method      string                   `json:"method"`
	Path        string                   `json:"path"`
	Auth        capability.AppRouteAuth  `json:"auth"`
	Permission  string                   `json:"permission,omitempty"`
	Description capability.LocalizedText `json:"description"`
}

func main() {
	preview, err := BuildContractPreview()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(preview); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func BuildContractPreview() (ContractPreview, error) {
	manifests, err := ResolveExampleManifests()
	if err != nil {
		return ContractPreview{}, err
	}
	routes, err := capability.AppRouteContracts(manifests)
	if err != nil {
		return ContractPreview{}, err
	}
	serviceDocument, err := capability.ServiceContractDocumentFromManifests(manifests)
	if err != nil {
		return ContractPreview{}, err
	}

	manifest := manifests[0]
	return ContractPreview{
		CapabilityID:           string(manifest.ID),
		Version:                manifest.Version,
		AdminResources:         adminResourceIDs(manifest.Admin.Resources),
		AdminResourceContracts: adminResourcePreviews(manifest.Admin.Resources),
		PermissionPrefixes:     permissionPrefixes(manifest.Admin.Resources),
		ConfigResources:        configResourceIDs(manifest.Admin.Resources),
		ConfigKeys:             authProviderConfigKeys(manifest.AuthProviders),
		AppRoutes:              appRoutePreviews(routes),
		DemoDataSets:           demoDataSetIDs(manifest.DemoData),
		Migrations:             migrationIDs(manifest.Migrations),
		Seeds:                  seedIDs(manifest.Seeds),
		ServiceContractHash:    serviceDocument.ContractHash,
		ServiceCount:           len(serviceDocument.Services),
	}, nil
}

func ResolveExampleManifests() ([]capability.Manifest, error) {
	manifests := Manifests()
	registry := capability.NewRegistry()
	enabled := make([]capability.ID, 0, len(manifests))
	for _, manifest := range manifests {
		if err := registry.Register(manifest); err != nil {
			return nil, err
		}
		enabled = append(enabled, manifest.ID)
	}
	return registry.ResolveEnabled(enabled)
}

func Manifests() []capability.Manifest {
	return []capability.Manifest{CatalogManifest()}
}

func CatalogManifest() capability.Manifest {
	return capability.Manifest{
		ID:      "example-catalog",
		Name:    "Example Catalog",
		Version: "0.1.0",
		Admin: capability.AdminSurface{
			Resources: []capability.AdminResource{catalogItemResource(), catalogSettingsResource()},
		},
		App: capability.AppSurface{
			Routes: []capability.AppRoute{
				{
					Method:      "GET",
					Path:        "/api/app/catalog/items",
					Auth:        capability.AppRouteAuthSession,
					Permission:  "app:catalog-item:read",
					Description: capability.Text("读取目录项列表。", "Read catalog item list."),
				},
			},
		},
		Service: catalogServiceSurface(),
		AuthProviders: []capability.AuthProvider{
			{
				ID:          "catalog-partner",
				Kind:        "oauth2",
				Title:       capability.Text("目录伙伴登录", "Catalog Partner Login"),
				Description: capability.Text("示例业务能力的外部伙伴登录边界。", "External partner login boundary for the example business capability."),
				Enabled:     false,
				ConfigKeys:  []string{"CATALOG_PARTNER_CLIENT_ID", "CATALOG_PARTNER_CLIENT_SECRET"},
				Audiences:   []capability.AuthProviderAudience{capability.AuthProviderAudienceApp},
			},
		},
		Migrations: []capability.Migration{
			{
				ID:          "catalog-0001",
				Description: "Create downstream catalog tables.",
				Up:          noopLifecycleStep,
			},
		},
		Seeds: []capability.Seed{
			{
				ID:          "catalog-seed-0001",
				Description: "Seed downstream catalog dictionary values.",
				Run:         noopLifecycleStep,
			},
		},
		DemoData: []capability.DemoDataSet{
			{
				ID:          "catalog-demo-items",
				Title:       capability.Text("目录演示项", "Catalog Demo Items"),
				Description: capability.Text("用于验证外部能力声明式资源和演示数据合同。", "Demo records for validating external capability resource and demo-data contracts."),
				Resource:    "catalog-items",
				Records: []capability.DemoRecord{
					{
						ID:          "catalog-item-demo",
						Code:        "demo-catalog-item",
						Name:        "Demo Catalog Item",
						Status:      "enabled",
						Description: "Standalone demo record owned by the example capability.",
						Values:      map[string]string{"category": "digital", "tenantCode": "demo-tenant"},
					},
				},
			},
		},
	}
}

func catalogSettingsResource() capability.AdminResource {
	return capability.AdminResource{
		Resource:         "catalog-settings",
		Title:            capability.Text("目录配置", "Catalog Settings"),
		Description:      capability.Text("外部目录能力的配置入口，由 /settings 动态聚合。", "Configuration entry for the external catalog capability, dynamically aggregated by /settings."),
		PermissionPrefix: "admin:catalog-setting",
		Deletion:         &capability.AdminResourceDeletionPolicy{Mode: capability.AdminDeletionDisabled, PolicyVersion: 1},
		Menu: capability.AdminMenu{
			Route:  "/catalog-settings",
			Parent: "configuration",
			Group:  "foundation",
			Icon:   "settings",
			Order:  320,
			Cache:  true,
		},
		FormLayout: "single-column",
		FormGroups: []capability.AdminFormGroup{
			{Key: "runtime", Label: capability.Text("运行配置", "Runtime")},
			{Key: "integration", Label: capability.Text("集成配置", "Integration")},
		},
		Fields: []capability.AdminField{
			catalogField("code", "配置编码", "Setting Code", "text", "record", "runtime", true, false, true, true, true, true, 160, nil),
			catalogField("name", "配置名称", "Setting Name", "text", "record", "runtime", true, false, true, true, true, true, 180, nil),
			catalogField("tenantCode", "租户编码", "Tenant Code", "text", "values", "runtime", false, false, true, true, true, true, 160, nil),
			catalogField("defaultVisibility", "默认可见性", "Default Visibility", "select", "values", "runtime", true, false, true, true, true, true, 160, []capability.AdminFieldOption{
				{Value: "internal", Label: capability.Text("内部可见", "Internal")},
				{Value: "public", Label: capability.Text("公开可见", "Public")},
			}),
			catalogField("approvalRequired", "发布需审批", "Approval Required", "switch", "values", "runtime", false, false, false, true, true, true, 140, nil),
			catalogField("partnerProvider", "伙伴登录提供方", "Partner Provider", "select", "values", "integration", false, false, true, true, true, true, 180, []capability.AdminFieldOption{
				{Value: "disabled", Label: capability.Text("未启用", "Disabled")},
				{Value: "catalog-partner", Label: capability.Text("目录伙伴", "Catalog Partner")},
			}),
			catalogField("updatedAt", "更新时间", "Updated At", "datetime", "record", "runtime", false, true, false, true, false, true, 180, nil),
		},
		SearchFields:   []string{"code", "name", "tenantCode", "defaultVisibility", "partnerProvider"},
		DefaultSortKey: "updatedAt",
	}
}

func catalogItemResource() capability.AdminResource {
	return capability.AdminResource{
		Resource:         "catalog-items",
		Title:            capability.Text("目录项", "Catalog Items"),
		Description:      capability.Text("外部业务目录项示例，不进入平台默认能力。", "External business catalog example that stays outside platform defaults."),
		PermissionPrefix: "admin:catalog-item",
		Deletion:         &capability.AdminResourceDeletionPolicy{Mode: capability.AdminDeletionSoftDelete, PolicyVersion: 1, RetentionDays: 30, RestrictReferences: true},
		Menu: capability.AdminMenu{
			Route:  "/catalog-items",
			Parent: "operations",
			Group:  "business",
			Icon:   "appstore",
			Order:  100,
			Cache:  true,
		},
		FormLayout: "grouped-sections",
		FormGroups: []capability.AdminFormGroup{
			{Key: "identity", Label: capability.Text("基础信息", "Identity")},
			{Key: "ownership", Label: capability.Text("归属", "Ownership")},
			{Key: "details", Label: capability.Text("详情", "Details")},
		},
		Fields: []capability.AdminField{
			catalogField("code", "目录编码", "Catalog Code", "text", "record", "identity", true, false, true, true, true, true, 160, nil),
			catalogField("name", "目录名称", "Catalog Name", "text", "record", "identity", true, false, true, true, true, true, 180, nil),
			catalogField("tenantCode", "租户编码", "Tenant Code", "text", "values", "ownership", true, false, true, true, true, true, 160, nil),
			catalogField("category", "目录分类", "Category", "select", "values", "details", true, false, true, true, true, true, 140, []capability.AdminFieldOption{
				{Value: "digital", Label: capability.Text("数字内容", "Digital")},
				{Value: "physical", Label: capability.Text("实物", "Physical")},
			}),
			catalogField("status", "状态", "Status", "select", "record", "details", false, false, true, true, true, true, 120, []capability.AdminFieldOption{
				{Value: "enabled", Label: capability.Text("已启用", "Enabled")},
				{Value: "disabled", Label: capability.Text("已停用", "Disabled")},
			}),
			catalogField("price", "价格", "Price", "number", "values", "details", false, false, false, true, true, true, 120, nil),
			catalogField("description", "说明", "Description", "textarea", "record", "details", false, false, false, false, true, true, 220, nil),
			catalogField("updatedAt", "更新时间", "Updated At", "datetime", "record", "details", false, true, false, true, false, true, 180, nil),
		},
		Actions: []capability.AdminResourceAction{
			{
				Key:         "publish",
				Label:       capability.Text("发布", "Publish"),
				Kind:        "row",
				Tone:        "primary",
				Icon:        "upload",
				Permission:  "admin:catalog-item:publish",
				Route:       "/api/admin/catalog-items/:id/publish",
				Method:      "POST",
				AuditAction: "catalog-item.publish",
				Refresh:     true,
			},
		},
		Panels: []capability.AdminResourcePanel{
			{Key: "audit", Label: capability.Text("审计", "Audit"), Kind: "audit", Permission: "admin:catalog-item:read", Order: 20},
		},
		RuntimeSlots: []capability.AdminRuntimeSlot{
			{
				SlotID:        "catalog-validation-hint",
				Region:        "form.section.after",
				Label:         capability.Text("校验提示", "Validation Hint"),
				Description:   capability.Text("示例业务包提供的表单扩展槽位。", "Form extension slot provided by the example business package."),
				Permission:    "admin:catalog-item:read",
				TargetSection: "details",
				DataBinding:   capability.AdminRuntimeSlotDataBinding{Mode: "formValues", Fields: []string{"code", "category"}},
				Variant:       "info",
				Order:         10,
			},
		},
		SearchFields:   []string{"name", "code", "tenantCode", "category", "status"},
		DefaultSortKey: "updatedAt",
	}
}

func catalogServiceSurface() capability.ServiceSurface {
	reliability := capability.ServiceReliability{
		Idempotency:           "none",
		OptimisticConcurrency: "version",
		TimeoutMilliseconds:   2000,
		MaxRetries:            1,
		RateLimitPerMinute:    120,
		CostLimit:             10,
	}
	compatibility := capability.ServiceCompatibility{Mode: "semver"}
	return capability.ServiceSurface{
		ID:            "catalog-service",
		Owner:         "example-catalog",
		Audiences:     []capability.ServiceAudience{capability.ServiceAudienceInternal},
		Stability:     capability.ServiceStabilityExperimental,
		Version:       "0.1.0",
		IdentityModes: []capability.ServiceIdentityMode{capability.ServiceIdentityWorkload},
		AuthModes:     []capability.ServiceAuthMode{capability.ServiceAuthWorkloadJWT},
		TenantContext: capability.DefaultTrustedTenantContext(),
		Operations: []capability.ServiceOperation{
			{
				ID:            "catalog-list-items",
				Kind:          capability.ServiceOperationQuery,
				Plane:         capability.ServicePlaneData,
				RuntimeStatus: capability.ServiceRuntimeContractOnly,
				IdentityMode:  capability.ServiceIdentityWorkload,
				AuthModes:     []capability.ServiceAuthMode{capability.ServiceAuthWorkloadJWT},
				TenantMode:    capability.ServiceTenantRequired,
				DataScopes:    []string{"tenant"},
				Permissions:   []string{"app:catalog-item:read"},
				RequestSchema: capability.ServicePayloadSchema{
					Ref: "#/schemas/CatalogItemQuery",
					PII: capability.ServicePIINone,
				},
				ResponseSchema: capability.ServicePayloadSchema{
					Ref:            "#/schemas/CatalogItemPage",
					RequiredFields: []string{"items"},
					PII:            capability.ServicePIINone,
				},
				Reliability:   reliability,
				Compatibility: compatibility,
				Description:   capability.Text("按可信租户上下文查询目录项。", "Query catalog items under trusted tenant context."),
			},
		},
		Events: []capability.ServiceEvent{
			{
				ID:              "catalog-item-published",
				Name:            "example.catalog.item-published.v1",
				Version:         1,
				Direction:       capability.ServiceEventPublish,
				RuntimeStatus:   capability.ServiceRuntimeContractOnly,
				TenantMode:      capability.ServiceTenantRequired,
				DataScopes:      []string{"tenant"},
				Permissions:     []string{"admin:catalog-item:publish"},
				PayloadSchema:   capability.ServicePayloadSchema{Ref: "#/schemas/CatalogItemPublished", RequiredFields: []string{"code"}, PII: capability.ServicePIINone},
				EnvelopeVersion: "1.0",
				TraceContext:    []string{"traceparent", "tracestate"},
				Compatibility:   compatibility,
				Description:     capability.Text("目录项发布事件。", "Catalog item published event."),
			},
		},
		SLA:           capability.ServiceSLA{AvailabilityTarget: "99.5%", LatencyP95MS: 500},
		Compatibility: compatibility,
		RuntimeBoundary: capability.ServiceRuntimeBoundary{
			ContractExecution:    "contract-only example; downstream package owns handlers and persistence",
			IdentityProtocols:    "workload-jwt declaration only",
			EventDelivery:        "contract-only; downstream package owns broker/outbox binding",
			DatasourceRouting:    "trusted tenant context only; client physical routing is forbidden",
			RuntimeSourceWriting: "disabled",
		},
	}
}

func catalogField(key string, labelZH string, labelEN string, fieldType string, source string, group string, required bool, readOnly bool, searchable bool, inTable bool, inForm bool, inDetail bool, width int, options []capability.AdminFieldOption) capability.AdminField {
	return capability.AdminField{
		Key:          key,
		Label:        capability.Text(labelZH, labelEN),
		Type:         fieldType,
		Source:       source,
		Group:        group,
		Required:     required,
		ReadOnly:     readOnly,
		Searchable:   searchable,
		Filterable:   searchable,
		Sortable:     fieldType == "text" || fieldType == "datetime" || fieldType == "number",
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

func noopLifecycleStep(context.Context, capability.Runtime) error {
	return nil
}

func appRoutePreviews(routes []capability.AppRouteContract) []AppRoutePreview {
	previews := make([]AppRoutePreview, 0, len(routes))
	for _, route := range routes {
		previews = append(previews, AppRoutePreview{
			Method:      route.Method,
			Path:        route.Path,
			Auth:        route.Auth,
			Permission:  route.Permission,
			Description: route.Description,
		})
	}
	return previews
}

func adminResourceIDs(resources []capability.AdminResource) []string {
	ids := make([]string, 0, len(resources))
	for _, resource := range resources {
		ids = append(ids, resource.Resource)
	}
	return ids
}

func adminResourcePreviews(resources []capability.AdminResource) []AdminResourcePreview {
	previews := make([]AdminResourcePreview, 0, len(resources))
	for _, resource := range resources {
		previews = append(previews, AdminResourcePreview{
			Resource:         resource.Resource,
			PermissionPrefix: resource.PermissionPrefix,
			MenuRoute:        resource.Menu.Route,
			MenuParent:       resource.Menu.Parent,
		})
	}
	return previews
}

func permissionPrefixes(resources []capability.AdminResource) []string {
	prefixes := make([]string, 0, len(resources))
	for _, resource := range resources {
		prefixes = append(prefixes, resource.PermissionPrefix)
	}
	return prefixes
}

func configResourceIDs(resources []capability.AdminResource) []string {
	ids := make([]string, 0, len(resources))
	for _, resource := range resources {
		if resource.Menu.Parent == "configuration" || resource.Resource == "settings" {
			ids = append(ids, resource.Resource)
		}
	}
	return ids
}

func authProviderConfigKeys(providers []capability.AuthProvider) []string {
	keys := []string{}
	for _, provider := range providers {
		keys = append(keys, provider.ConfigKeys...)
	}
	return keys
}

func demoDataSetIDs(datasets []capability.DemoDataSet) []string {
	ids := make([]string, 0, len(datasets))
	for _, dataset := range datasets {
		ids = append(ids, dataset.ID)
	}
	return ids
}

func migrationIDs(migrations []capability.Migration) []string {
	ids := make([]string, 0, len(migrations))
	for _, migration := range migrations {
		ids = append(ids, migration.ID)
	}
	return ids
}

func seedIDs(seeds []capability.Seed) []string {
	ids := make([]string, 0, len(seeds))
	for _, seed := range seeds {
		ids = append(ids, seed.ID)
	}
	return ids
}
