package adminresource

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"platform-go/internal/platform/capability"
	"platform-go/internal/platform/core"
)

func TestStoreRegistersAdminResourcesFromEnabledCapabilities(t *testing.T) {
	store := NewStoreFromCapabilities([]capability.Manifest{
		{
			ID: "feedback",
			Admin: capability.AdminSurface{
				Resources: []capability.AdminResource{
					{
						Resource:         "feedback-tickets",
						Title:            capability.Text("反馈工单", "Feedback Tickets"),
						Description:      capability.Text("用户反馈与处理记录。", "User feedback and handling records."),
						PermissionPrefix: "admin:feedback-ticket",
						Menu: capability.AdminMenu{
							Route:    "/feedback-tickets",
							Parent:   "support/workbench",
							Group:    "operations",
							Icon:     "audit",
							Order:    250,
							External: true,
							Cache:    false,
						},
					},
				},
			},
		},
	})

	schema, err := store.Schema("feedback-tickets")
	if err != nil {
		t.Fatalf("Schema(feedback-tickets) error = %v", err)
	}
	if schema.Permissions.Read != "admin:feedback-ticket:read" {
		t.Fatalf("schema read permission = %q, want admin:feedback-ticket:read", schema.Permissions.Read)
	}
	if schema.Title.ZH != "反馈工单" || len(schema.Fields) == 0 {
		t.Fatalf("schema not populated from manifest: %+v", schema)
	}

	principal := store.CurrentPrincipal("admin")
	menus := store.MenuItemsForPrincipal(principal)
	if !hasMenuRoute(menus, "/feedback-tickets") {
		t.Fatalf("menus = %+v, want feedback menu", menus)
	}
	if parent := menuParentForRoute(menus, "/feedback-tickets"); parent != "support/workbench" {
		t.Fatalf("feedback menu parent = %q, want support/workbench", parent)
	}
	if item := menuForRoute(menus, "/feedback-tickets"); item == nil || !item.IsExternal || item.CacheEnabled {
		t.Fatalf("feedback menu external/cache = %+v, want external without cache", item)
	}
}

func TestStoreSchemaExposesFormMetadataFromCapabilityFields(t *testing.T) {
	store := NewStoreFromCapabilities([]capability.Manifest{
		{
			ID: "ticketing",
			Admin: capability.AdminSurface{
				Resources: []capability.AdminResource{
					{
						Resource:         "support-tickets",
						Title:            capability.Text("支持工单", "Support Tickets"),
						Description:      capability.Text("支持工单资源。", "Support ticket resource."),
						PermissionPrefix: "admin:support-ticket",
						Menu:             capability.AdminMenu{Route: "/support-tickets", Group: "operations", Icon: "audit", Order: 260},
						FormLayout:       "two-column-density",
						FormGroups: []capability.AdminFormGroup{
							{
								Key:         "basic",
								Label:       capability.Text("基础信息", "Basic Info"),
								Description: capability.Text("工单的核心识别信息。", "Core ticket identity fields."),
							},
						},
						Fields: []capability.AdminField{
							{
								Key:        "name",
								Label:      capability.Text("工单名称", "Ticket Name"),
								Type:       "text",
								Source:     "record",
								Required:   true,
								Searchable: true,
								InTable:    true,
								InForm:     true,
								InDetail:   true,
								Width:      180,
								Group:      "basic",
								Help:       capability.Text("用于列表、搜索和通知。", "Used for lists, search, and notifications."),
								Validation: capability.AdminFieldValidation{
									MinLength: 3,
									MaxLength: 64,
									Pattern:   "^TK-[0-9]+$",
								},
							},
						},
						SearchFields:   []string{"name"},
						DefaultSortKey: "name",
					},
				},
			},
		},
	})

	schema, err := store.Schema("support-tickets")
	if err != nil {
		t.Fatalf("Schema(support-tickets) error = %v", err)
	}
	if len(schema.FormGroups) != 1 || schema.FormGroups[0].Key != "basic" || schema.FormGroups[0].Label.ZH != "基础信息" {
		t.Fatalf("schema form groups = %+v, want localized basic group", schema.FormGroups)
	}
	if schema.FormLayout != "two-column-density" {
		t.Fatalf("schema form layout = %q, want two-column-density", schema.FormLayout)
	}
	field := fieldByKey(schema.Fields, "name")
	if field == nil {
		t.Fatalf("schema fields = %+v, want name field", schema.Fields)
	}
	if field.Group != "basic" {
		t.Fatalf("field group = %q, want basic", field.Group)
	}
	if field.Help == nil || field.Help.ZH != "用于列表、搜索和通知。" || field.Help.EN == "" {
		t.Fatalf("field help = %+v, want localized help", field.Help)
	}
	if field.Validation == nil || field.Validation.MinLength != 3 || field.Validation.MaxLength != 64 || field.Validation.Pattern != "^TK-[0-9]+$" {
		t.Fatalf("field validation = %+v, want min/max/pattern", field.Validation)
	}
}

func TestStoreSchemaExposesRuntimeSlotsFromCapabilityResources(t *testing.T) {
	store := NewStoreFromCapabilities([]capability.Manifest{
		{
			ID: "ticketing",
			Admin: capability.AdminSurface{
				Resources: []capability.AdminResource{
					{
						Resource:         "support-tickets",
						Title:            capability.Text("支持工单", "Support Tickets"),
						Description:      capability.Text("支持工单资源。", "Support ticket resource."),
						PermissionPrefix: "admin:support-ticket",
						Menu:             capability.AdminMenu{Route: "/support-tickets", Group: "operations", Icon: "audit", Order: 260},
						FormLayout:       "side-detail-preview",
						Fields: []capability.AdminField{
							{
								Key:        "title",
								Label:      capability.Text("标题", "Title"),
								Type:       "text",
								Source:     "values",
								Searchable: true,
								InTable:    true,
								InForm:     true,
								InDetail:   true,
							},
						},
						RuntimeSlots: []capability.AdminRuntimeSlot{
							{
								SlotID:      "platform.record-summary",
								Region:      "side.preview",
								Label:       capability.Text("记录摘要", "Record Summary"),
								Description: capability.Text("展示当前记录关键字段。", "Shows key fields for the current record."),
								Permission:  "admin:support-ticket:read",
								DataBinding: capability.AdminRuntimeSlotDataBinding{
									Mode:   "record",
									Fields: []string{"title", "status"},
								},
								Variant: "preview",
								Order:   10,
							},
						},
					},
				},
			},
		},
	})

	schema, err := store.Schema("support-tickets")
	if err != nil {
		t.Fatalf("Schema(support-tickets) error = %v", err)
	}
	if len(schema.RuntimeSlots) != 1 {
		t.Fatalf("schema runtime slots = %+v, want one slot", schema.RuntimeSlots)
	}
	if schema.FormLayout != "side-detail-preview" {
		t.Fatalf("schema form layout = %q, want side-detail-preview", schema.FormLayout)
	}
	slot := schema.RuntimeSlots[0]
	if slot.SlotID != "platform.record-summary" || slot.Region != "side.preview" || slot.DataBinding.Mode != "record" {
		t.Fatalf("schema runtime slot = %+v, want record summary side preview slot", slot)
	}
	if len(slot.DataBinding.Fields) != 2 || slot.DataBinding.Fields[0] != "title" || slot.DataBinding.Fields[1] != "status" {
		t.Fatalf("schema runtime slot fields = %+v, want title/status", slot.DataBinding.Fields)
	}

	schema.RuntimeSlots[0].DataBinding.Fields[0] = "mutated"
	freshSchema, err := store.Schema("support-tickets")
	if err != nil {
		t.Fatalf("Schema(support-tickets) second read error = %v", err)
	}
	if freshSchema.RuntimeSlots[0].DataBinding.Fields[0] != "title" {
		t.Fatalf("schema runtime slot fields were not cloned, got %+v", freshSchema.RuntimeSlots[0].DataBinding.Fields)
	}
}

func TestStoreSchemaExposesActionAndPanelMetadataFromCapabilityResources(t *testing.T) {
	store := NewStoreFromCapabilities([]capability.Manifest{
		{
			ID: "ticketing",
			Admin: capability.AdminSurface{
				Resources: []capability.AdminResource{
					{
						Resource:         "support-tickets",
						Title:            capability.Text("支持工单", "Support Tickets"),
						Description:      capability.Text("支持工单资源。", "Support ticket resource."),
						PermissionPrefix: "admin:support-ticket",
						Menu:             capability.AdminMenu{Route: "/support-tickets", Group: "operations", Icon: "audit", Order: 260},
						Actions: []capability.AdminResourceAction{
							{
								Key:        "approve",
								Label:      capability.Text("通过", "Approve"),
								Kind:       "row",
								Tone:       "primary",
								Icon:       "check",
								Permission: "admin:support-ticket:update",
								Route:      "/api/admin/support-tickets/:id/approve",
								Method:     "POST",
								Confirm: &capability.AdminActionConfirm{
									Title:       capability.Text("确认通过", "Confirm Approval"),
									Description: capability.Text("确认通过该记录。", "Approve this record."),
									OkText:      capability.Text("通过", "Approve"),
								},
								AuditAction: "support_ticket.approve",
								Refresh:     true,
							},
						},
						Panels: []capability.AdminResourcePanel{
							{
								Key:        "audit",
								Label:      capability.Text("审计", "Audit"),
								Kind:       "audit",
								Permission: "admin:support-ticket:read",
								Component:  "audit-timeline",
								Order:      30,
								Empty:      capability.Text("暂无审计记录", "No audit records"),
							},
						},
					},
				},
			},
		},
	})

	schema, err := store.Schema("support-tickets")
	if err != nil {
		t.Fatalf("Schema(support-tickets) error = %v", err)
	}
	if len(schema.Actions) != 1 || schema.Actions[0].Key != "approve" || schema.Actions[0].Confirm == nil {
		t.Fatalf("schema actions = %+v, want approve action with confirm", schema.Actions)
	}
	if schema.Actions[0].Confirm.Title.ZH != "确认通过" || schema.Actions[0].AuditAction != "support_ticket.approve" {
		t.Fatalf("schema action = %+v, want localized confirm and audit action", schema.Actions[0])
	}
	if len(schema.Panels) != 1 || schema.Panels[0].Key != "audit" || schema.Panels[0].Component != "audit-timeline" {
		t.Fatalf("schema panels = %+v, want audit panel", schema.Panels)
	}
}

func TestStoreSchemaExposesStaticMenuActionAndPanelMetadata(t *testing.T) {
	store := NewStoreFromCapabilities(core.DefaultManifests())

	schema, err := store.Schema("menus")
	if err != nil {
		t.Fatalf("Schema(menus) error = %v", err)
	}
	if len(schema.Actions) != 1 || schema.Actions[0].Key != "copy-config" || schema.Actions[0].Permission != "admin:menu:read" {
		t.Fatalf("schema actions = %+v, want copy-config action", schema.Actions)
	}
	if len(schema.Panels) != 1 || schema.Panels[0].Key != "audit" || schema.Panels[0].Component != "audit-timeline" {
		t.Fatalf("schema panels = %+v, want audit timeline panel", schema.Panels)
	}
	if schema.FormLayout != "side-detail-preview" {
		t.Fatalf("schema form layout = %q, want side-detail-preview", schema.FormLayout)
	}
	if len(schema.RuntimeSlots) != 3 {
		t.Fatalf("schema runtime slots = %+v, want menu preview slots", schema.RuntimeSlots)
	}
}

func TestCoreRuntimeSchemasHaveUniqueFieldKeys(t *testing.T) {
	store := NewStoreFromCapabilities(core.DefaultManifests())

	for resource, schema := range store.schemas {
		seen := map[string]struct{}{}
		for _, field := range schema.Fields {
			if _, ok := seen[field.Key]; ok {
				t.Fatalf("schema %s has duplicate field key %q", resource, field.Key)
			}
			seen[field.Key] = struct{}{}
		}
	}
}

func TestCoreRuntimeSchemasExposeGovernanceTopology(t *testing.T) {
	store := NewStoreFromCapabilities(core.DefaultManifests())

	for _, resource := range []string{"tenants", "org-units", "users", "roles", "role-groups", "area-codes"} {
		if _, err := store.Schema(resource); err != nil {
			t.Fatalf("Schema(%s) error = %v", resource, err)
		}
	}

	orgUnits, err := store.Schema("org-units")
	if err != nil {
		t.Fatalf("Schema(org-units) error = %v", err)
	}
	requireRelationField(t, orgUnits, "tenantCode", "tenants", "", false, true)
	requireRelationField(t, orgUnits, "parentCode", "org-units", "tree", false, false)
	requireRelationField(t, orgUnits, "areaCode", "area-codes", "tree", false, false)

	roleGroups, err := store.Schema("role-groups")
	if err != nil {
		t.Fatalf("Schema(role-groups) error = %v", err)
	}
	requireRelationField(t, roleGroups, "parentCode", "role-groups", "tree", false, false)
	for _, field := range roleGroups.Fields {
		key := strings.ToLower(field.Key)
		for _, forbidden := range []string{"permission", "datascope", "scope", "inherit", "membership", "membercodes", "usercodes"} {
			if strings.Contains(key, forbidden) {
				t.Fatalf("role-groups field %q adds policy semantics; role groups must remain classification-only", field.Key)
			}
		}
	}

	roles, err := store.Schema("roles")
	if err != nil {
		t.Fatalf("Schema(roles) error = %v", err)
	}
	requireRelationField(t, roles, "groupCode", "role-groups", "tree", false, false)
	requireRelationField(t, roles, "dataScopeOrgCodes", "org-units", "tree", true, false)
	requireRelationField(t, roles, "dataScopeAreaCodes", "area-codes", "tree", true, false)

	areaCodes, err := store.Schema("area-codes")
	if err != nil {
		t.Fatalf("Schema(area-codes) error = %v", err)
	}
	requireRelationField(t, areaCodes, "parentCode", "area-codes", "tree", false, false)
	pathField := fieldByKey(areaCodes.Fields, "path")
	if pathField == nil {
		t.Fatalf("area-codes path field is missing")
	}
}

func TestStoreDoesNotRegisterDisabledCapabilityAdminResources(t *testing.T) {
	store := NewStoreFromCapabilities([]capability.Manifest{
		{
			ID: "enabled",
			Admin: capability.AdminSurface{
				Resources: []capability.AdminResource{
					{
						Resource:         "enabled-resource",
						Title:            capability.Text("启用资源", "Enabled Resource"),
						Description:      capability.Text("启用能力资源。", "Enabled capability resource."),
						PermissionPrefix: "admin:enabled-resource",
						Menu:             capability.AdminMenu{Route: "/enabled-resource", Group: "foundation", Icon: "overview", Order: 10},
					},
				},
			},
		},
	})

	if _, err := store.Schema("enabled-resource"); err != nil {
		t.Fatalf("Schema(enabled-resource) error = %v", err)
	}
	if _, err := store.Schema("disabled-resource"); !errors.Is(err, ErrUnknownResource) {
		t.Fatalf("Schema(disabled-resource) error = %v, want ErrUnknownResource", err)
	}
}

func TestStoreRegistersCoreManifestMenus(t *testing.T) {
	store := NewStoreFromCapabilities(core.DefaultManifests())
	principal := store.CurrentPrincipal("admin")
	menus := store.MenuItemsForPrincipal(principal)

	for _, route := range []string{"/overview", "/capabilities", "/tenants", "/users", "/roles", "/menus", "/api-docs", "/api-tokens"} {
		if !hasMenuRoute(menus, route) {
			t.Fatalf("core manifest menus missing %s: %+v", route, menus)
		}
	}
}

func TestMenuItemsForPrincipalKeepsDeepParentsAndAppliesRoleVisibility(t *testing.T) {
	manifests := append(core.DefaultManifests(), capability.Manifest{
		ID: "support",
		Admin: capability.AdminSurface{
			Resources: []capability.AdminResource{
				{
					Resource:         "feedback-tickets",
					Title:            capability.Text("反馈工单", "Feedback Tickets"),
					Description:      capability.Text("反馈工单资源。", "Feedback ticket resource."),
					PermissionPrefix: "admin:feedback-ticket",
					Menu: capability.AdminMenu{
						Route:  "/feedback-tickets",
						Parent: "support/workbench/tickets",
						Group:  "operations",
						Icon:   "audit",
						Order:  250,
						Cache:  true,
					},
				},
				{
					Resource:         "private-audits",
					Title:            capability.Text("私有审计", "Private Audits"),
					Description:      capability.Text("私有审计资源。", "Private audit resource."),
					PermissionPrefix: "admin:private-audit",
					Menu: capability.AdminMenu{
						Route:  "/private-audits",
						Parent: "support/workbench/audits",
						Group:  "operations",
						Icon:   "audit",
						Order:  260,
						Cache:  true,
					},
				},
			},
		},
	})
	store := NewStoreFromCapabilities(manifests)
	if _, err := store.Update("roles", "role-operator", WriteInput{
		Name:        "Operator",
		Status:      "enabled",
		Description: "Support operator",
		Values: map[string]string{
			"groupCode":       "operations",
			"dataScope":       "current_org",
			"permissions":     "admin:feedback-ticket:read,admin:private-audit:read",
			"denyPermissions": "admin:private-audit:read",
		},
	}); err != nil {
		t.Fatalf("Update(role-operator) error = %v", err)
	}

	menus := store.MenuItemsForPrincipal(store.CurrentPrincipal("ops"))
	if !hasMenuRoute(menus, "/feedback-tickets") {
		t.Fatalf("menus = %+v, want permitted feedback menu", menus)
	}
	if parent := menuParentForRoute(menus, "/feedback-tickets"); parent != "support/workbench/tickets" {
		t.Fatalf("feedback menu parent = %q, want support/workbench/tickets", parent)
	}
	if item := menuForRoute(menus, "/feedback-tickets"); item == nil || item.IsExternal || !item.CacheEnabled {
		t.Fatalf("feedback menu external/cache = %+v, want internal cached menu", item)
	}
	if hasMenuRoute(menus, "/private-audits") {
		t.Fatalf("menus = %+v, want denied private audit menu hidden", menus)
	}
}

func TestStoreBuildsPermissionCatalogFromCapabilityAdminResources(t *testing.T) {
	store := NewStoreFromCapabilities(core.DefaultManifests())

	schema, err := store.Schema("permissions")
	if err != nil {
		t.Fatalf("Schema(permissions) error = %v", err)
	}
	if schema.Permissions.Read != "admin:permission:read" {
		t.Fatalf("permission schema read permission = %q, want admin:permission:read", schema.Permissions.Read)
	}

	permissions, err := store.List("permissions")
	if err != nil {
		t.Fatalf("List(permissions) error = %v", err)
	}
	if !hasRecordCode(permissions, "admin:tenant:read") || !hasRecordCode(permissions, "admin:role:update") {
		t.Fatalf("permission catalog missing core permissions: %+v", permissions)
	}
	permission := recordByCode(permissions, "admin:tenant:read")
	if permission == nil {
		t.Fatalf("permission catalog missing admin:tenant:read")
	}
	if permission.Values["nameZh"] != "租户读取" || permission.Values["descriptionZh"] != "租户读取权限。" {
		t.Fatalf("permission localized values = %+v, want zh name and description", permission.Values)
	}
}

func TestStoreQueryFiltersSortsAndPaginatesRecords(t *testing.T) {
	now := time.Date(2026, 7, 4, 10, 0, 0, 0, time.UTC)
	store := newStore(map[string][]Record{
		"dictionary-parameters": {
			{
				ID:          "dict-1",
				Code:        "billing-status",
				Name:        "Billing Status",
				Status:      "enabled",
				Description: "Billing dictionary",
				UpdatedAt:   now.Add(-2 * time.Hour).Format(time.RFC3339),
				Values:      map[string]string{"scope": "finance"},
			},
			{
				ID:          "dict-2",
				Code:        "order-status",
				Name:        "Order Status",
				Status:      "enabled",
				Description: "Order dictionary",
				UpdatedAt:   now.Format(time.RFC3339),
				Values:      map[string]string{"scope": "global"},
			},
			{
				ID:          "dict-3",
				Code:        "hidden-status",
				Name:        "Hidden Status",
				Status:      "disabled",
				Description: "Hidden dictionary",
				UpdatedAt:   now.Add(-1 * time.Hour).Format(time.RFC3339),
				Values:      map[string]string{"scope": "global"},
			},
		},
	}, map[string]Schema{"dictionary-parameters": dictionaryParameterSchema()})

	result, err := store.Query("dictionary-parameters", QueryInput{
		Keywords: []string{"status"},
		Conditions: []QueryCondition{
			{Field: "status", Operator: "=", Value: "enabled"},
			{Field: "scope", Operator: "contains", Value: "global"},
		},
		Sort:     []QuerySort{{Field: "updatedAt", Order: "desc"}},
		Page:     1,
		PageSize: 1,
	})
	if err != nil {
		t.Fatalf("Query(dictionary-parameters) error = %v", err)
	}
	if result.Total != 1 || result.Page != 1 || result.PageSize != 1 {
		t.Fatalf("query metadata = total %d page %d pageSize %d, want 1/1/1", result.Total, result.Page, result.PageSize)
	}
	if len(result.Items) != 1 || result.Items[0].ID != "dict-2" {
		t.Fatalf("query items = %+v, want newest enabled global status", result.Items)
	}
}

func TestStoreQuerySupportsDateRangeConditions(t *testing.T) {
	store := newStore(map[string][]Record{
		"tenants": {
			{ID: "tenant-1", Code: "a", Name: "A", Status: "enabled", UpdatedAt: "2026-07-01T00:00:00Z"},
			{ID: "tenant-2", Code: "b", Name: "B", Status: "enabled", UpdatedAt: "2026-07-02T00:00:00Z"},
			{ID: "tenant-3", Code: "c", Name: "C", Status: "enabled", UpdatedAt: "2026-07-03T00:00:00Z"},
		},
	}, map[string]Schema{"tenants": defaultSchema("tenants", text("租户", "Tenants"), text("租户", "Tenants"), "admin:tenant")})

	result, err := store.Query("tenants", QueryInput{
		Conditions: []QueryCondition{
			{Field: "updatedAt", Operator: ">=", Value: "2026-07-02"},
			{Field: "updatedAt", Operator: "<", Value: "2026-07-03"},
		},
		Page:     1,
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("Query(tenants) date range error = %v", err)
	}
	if result.Total != 1 || len(result.Items) != 1 || result.Items[0].ID != "tenant-2" {
		t.Fatalf("date range query = total %d items %+v, want tenant-2", result.Total, result.Items)
	}
}

func TestStoreQuerySearchesLocalizableValues(t *testing.T) {
	store := newStore(map[string][]Record{
		"tenants": {
			{
				ID:        "tenant-platform",
				Code:      "platform",
				Name:      "Platform Tenant",
				Status:    "enabled",
				UpdatedAt: "2026-07-02T00:00:00Z",
				Values:    map[string]string{"nameZh": "平台租户"},
			},
		},
	}, map[string]Schema{"tenants": defaultSchema("tenants", text("租户", "Tenants"), text("租户", "Tenants"), "admin:tenant")})

	result, err := store.Query("tenants", QueryInput{Keywords: []string{"平台"}, Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("Query(tenants) localized value error = %v", err)
	}
	if result.Total != 1 || len(result.Items) != 1 || result.Items[0].ID != "tenant-platform" {
		t.Fatalf("localized query = total %d items %+v, want tenant-platform", result.Total, result.Items)
	}
}

func TestStoreQueryRejectsUnsafeConditions(t *testing.T) {
	store := NewStoreFromCapabilities(core.DefaultManifests())
	tests := []struct {
		name  string
		input QueryInput
	}{
		{
			name:  "unknown field",
			input: QueryInput{Conditions: []QueryCondition{{Field: "missing", Operator: "=", Value: "x"}}},
		},
		{
			name:  "sensitive field",
			input: QueryInput{Conditions: []QueryCondition{{Field: "password", Operator: "=", Value: "x"}}},
		},
		{
			name:  "invalid operator",
			input: QueryInput{Conditions: []QueryCondition{{Field: "status", Operator: "DROP TABLE", Value: "enabled"}}},
		},
		{
			name:  "long value",
			input: QueryInput{Conditions: []QueryCondition{{Field: "status", Operator: "=", Value: strings.Repeat("a", 257)}}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := store.Query("tenants", tt.input); !errors.Is(err, ErrInvalidRecord) {
				t.Fatalf("Query unsafe error = %v, want ErrInvalidRecord", err)
			}
		})
	}
}

func TestStoreQueryRejectsUndeclaredFieldCapabilities(t *testing.T) {
	schema := defaultSchema("tenants", text("租户", "Tenants"), text("租户", "Tenants"), "admin:tenant")
	schema.Fields = append(schema.Fields,
		valueField("displayOnly", text("展示字段", "Display Only"), "text", false, false, false, false, true, 160, nil),
		valueField("displayNote", text("展示备注", "Display Note"), "textarea", false, false, true, false, true, 220, nil),
	)
	store := newStore(map[string][]Record{
		"tenants": {
			{
				ID:     "tenant-platform",
				Code:   "platform",
				Name:   "Platform Tenant",
				Status: "enabled",
				Values: map[string]string{
					"displayOnly": "internal",
					"displayNote": "manual only",
				},
			},
		},
	}, map[string]Schema{"tenants": schema})

	if _, err := store.Query("tenants", QueryInput{Conditions: []QueryCondition{{Field: "displayOnly", Operator: "=", Value: "internal"}}}); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("Query non-filterable field error = %v, want ErrInvalidRecord", err)
	}
	if _, err := store.Query("tenants", QueryInput{Sort: []QuerySort{{Field: "displayNote", Order: "asc"}}}); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("Query non-sortable field error = %v, want ErrInvalidRecord", err)
	}
}

func TestStoreQueryTreatsSQLLikeValuesAsLiterals(t *testing.T) {
	store := newStore(map[string][]Record{
		"tenants": {
			{
				ID:          "tenant-platform",
				Code:        "tenant-platform",
				Name:        "Platform Tenant",
				Status:      "enabled",
				Description: "Default platform tenant",
			},
			{
				ID:          "tenant-demo",
				Code:        "tenant-demo",
				Name:        "Demo Tenant",
				Status:      "disabled",
				Description: "Reusable demo tenant",
			},
		},
	}, map[string]Schema{"tenants": defaultSchema("tenants", text("租户", "Tenants"), text("租户", "Tenants"), "admin:tenant")})

	result, err := store.Query("tenants", QueryInput{
		Conditions: []QueryCondition{
			{Field: "status", Operator: "=", Value: "enabled' OR '1'='1"},
		},
		Page:     1,
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("Query(sql-like value) error = %v", err)
	}
	if result.Total != 0 || len(result.Items) != 0 {
		t.Fatalf("sql-like value matched records = total %d items %+v, want no literal match", result.Total, result.Items)
	}
}

func TestStoreRoleSchemaExposesPermissionCatalogOptions(t *testing.T) {
	store := NewStoreFromCapabilities(core.DefaultManifests())

	schema, err := store.Schema("roles")
	if err != nil {
		t.Fatalf("Schema(roles) error = %v", err)
	}
	var groupCodeField FieldDefinition
	var dataScopeField FieldDefinition
	var dataScopeOrgCodesField FieldDefinition
	var dataScopeAreaCodesField FieldDefinition
	var permissionsField FieldDefinition
	var denyPermissionsField FieldDefinition
	for _, field := range schema.Fields {
		if field.Key == "groupCode" {
			groupCodeField = field
		}
		if field.Key == "dataScope" {
			dataScopeField = field
		}
		if field.Key == "dataScopeOrgCodes" {
			dataScopeOrgCodesField = field
		}
		if field.Key == "dataScopeAreaCodes" {
			dataScopeAreaCodesField = field
		}
		if field.Key == "permissions" {
			permissionsField = field
		}
		if field.Key == "denyPermissions" {
			denyPermissionsField = field
		}
	}
	if groupCodeField.Key == "" {
		t.Fatalf("roles schema missing groupCode field: %+v", schema.Fields)
	}
	if permissionsField.Key == "" {
		t.Fatalf("roles schema missing permissions field: %+v", schema.Fields)
	}
	if dataScopeField.Key == "" || dataScopeField.Type != "select" || !dataScopeField.Required {
		t.Fatalf("roles schema dataScope field = %+v, want required select", dataScopeField)
	}
	if dataScopeOrgCodesField.Key == "" || dataScopeOrgCodesField.Type != "multiselect" {
		t.Fatalf("roles schema dataScopeOrgCodes field = %+v, want multiselect", dataScopeOrgCodesField)
	}
	if dataScopeAreaCodesField.Key == "" || dataScopeAreaCodesField.Type != "multiselect" {
		t.Fatalf("roles schema dataScopeAreaCodes field = %+v, want multiselect", dataScopeAreaCodesField)
	}
	if permissionsField.Type != "multiselect" {
		t.Fatalf("roles permissions field type = %q, want multiselect", permissionsField.Type)
	}
	if denyPermissionsField.Key == "" || denyPermissionsField.Type != "multiselect" {
		t.Fatalf("roles denyPermissions field = %+v, want multiselect", denyPermissionsField)
	}
	if groupCodeField.Relation == nil || groupCodeField.Relation.Resource != "role-groups" || groupCodeField.Relation.ValueField != "code" {
		t.Fatalf("roles groupCode relation = %+v, want role-groups code relation", groupCodeField.Relation)
	}
	if groupCodeField.Relation.Display != "tree" || groupCodeField.Relation.ParentField != "parentCode" {
		t.Fatalf("roles groupCode relation display = %+v, want tree parentCode", groupCodeField.Relation)
	}
	if permissionsField.Relation == nil || permissionsField.Relation.Resource != "permissions" || !permissionsField.Relation.Multiple {
		t.Fatalf("roles permissions relation = %+v, want multiple permissions relation", permissionsField.Relation)
	}
	if denyPermissionsField.Relation == nil || denyPermissionsField.Relation.Resource != "permissions" || !denyPermissionsField.Relation.Multiple {
		t.Fatalf("roles denyPermissions relation = %+v, want multiple permissions relation", denyPermissionsField.Relation)
	}
	if dataScopeOrgCodesField.Relation == nil || dataScopeOrgCodesField.Relation.Resource != "org-units" || dataScopeOrgCodesField.Relation.Display != "tree" || !dataScopeOrgCodesField.Relation.Multiple {
		t.Fatalf("roles dataScopeOrgCodes relation = %+v, want org-units tree relation", dataScopeOrgCodesField.Relation)
	}
	if dataScopeAreaCodesField.Relation == nil || dataScopeAreaCodesField.Relation.Resource != "area-codes" || dataScopeAreaCodesField.Relation.Display != "tree" || !dataScopeAreaCodesField.Relation.Multiple || dataScopeAreaCodesField.Relation.PathField != "path" {
		t.Fatalf("roles dataScopeAreaCodes relation = %+v, want area-codes tree relation with path", dataScopeAreaCodesField.Relation)
	}
	if !hasFieldOption(permissionsField.Options, "admin:tenant:read") || !hasFieldOption(permissionsField.Options, "admin:menu:read") {
		t.Fatalf("roles permissions options missing catalog entries: %+v", permissionsField.Options)
	}
	if !hasFieldOption(denyPermissionsField.Options, "admin:tenant:read") || !hasFieldOption(denyPermissionsField.Options, "admin:menu:read") {
		t.Fatalf("roles denyPermissions options missing catalog entries: %+v", denyPermissionsField.Options)
	}
	if !hasFieldOption(dataScopeField.Options, "all") || !hasFieldOption(dataScopeField.Options, "current_org") || !hasFieldOption(dataScopeField.Options, "current_area") || !hasFieldOption(dataScopeField.Options, "custom_areas") {
		t.Fatalf("roles dataScope options missing expected entries: %+v", dataScopeField.Options)
	}

	roles, err := store.List("roles")
	if err != nil {
		t.Fatalf("List(roles) error = %v", err)
	}
	superAdmin := findRecordByCode(roles, "super-admin")
	operator := findRecordByCode(roles, "operator")
	if superAdmin == nil || superAdmin.Values["dataScope"] != "all" {
		t.Fatalf("super-admin dataScope = %+v, want all", superAdmin)
	}
	if operator == nil || operator.Values["dataScope"] != "current_org" {
		t.Fatalf("operator dataScope = %+v, want current_org", operator)
	}
}

func TestDefaultStoreIncludesOrganizationRoleGroupAndAreaResources(t *testing.T) {
	store := NewStore()

	for _, resource := range []string{"org-units", "role-groups", "area-codes"} {
		schema, err := store.Schema(resource)
		if err != nil {
			t.Fatalf("Schema(%s) error = %v", resource, err)
		}
		if schema.Resource != resource {
			t.Fatalf("Schema(%s).Resource = %q", resource, schema.Resource)
		}
		records, err := store.List(resource)
		if err != nil {
			t.Fatalf("List(%s) error = %v", resource, err)
		}
		if len(records) == 0 {
			t.Fatalf("List(%s) returned no seed records", resource)
		}
	}

	userSchema, err := store.Schema("users")
	if err != nil {
		t.Fatalf("Schema(users) error = %v", err)
	}
	userTenantField := fieldByKey(userSchema.Fields, "tenantCode")
	if userTenantField == nil || !userTenantField.Required || userTenantField.Relation == nil || userTenantField.Relation.Resource != "tenants" {
		t.Fatalf("users.tenantCode field = %+v, want required relation to tenants", userTenantField)
	}
	if orgUnitField := fieldByKey(userSchema.Fields, "orgUnitCode"); orgUnitField == nil || orgUnitField.Required {
		t.Fatalf("users.orgUnitCode field = %+v, want optional org unit relation", orgUnitField)
	}
	if areaField := fieldByKey(userSchema.Fields, "areaCode"); areaField == nil || areaField.Required {
		t.Fatalf("users.areaCode field = %+v, want optional area relation", areaField)
	}

	orgSchema, err := store.Schema("org-units")
	if err != nil {
		t.Fatalf("Schema(org-units) error = %v", err)
	}
	orgTenantField := fieldByKey(orgSchema.Fields, "tenantCode")
	if orgTenantField == nil || !orgTenantField.Required || orgTenantField.Relation == nil || orgTenantField.Relation.Resource != "tenants" {
		t.Fatalf("org-units.tenantCode field = %+v, want required relation to tenants", orgTenantField)
	}
	if parentField := fieldByKey(orgSchema.Fields, "parentCode"); parentField == nil || parentField.Required {
		t.Fatalf("org-units.parentCode field = %+v, want optional tree parent", parentField)
	}
	if areaField := fieldByKey(orgSchema.Fields, "areaCode"); areaField == nil || areaField.Required {
		t.Fatalf("org-units.areaCode field = %+v, want optional area relation", areaField)
	}

	areaSchema, err := store.Schema("area-codes")
	if err != nil {
		t.Fatalf("Schema(area-codes) error = %v", err)
	}
	levelField := fieldByKey(areaSchema.Fields, "level")
	if levelField == nil || !hasFieldOption(levelField.Options, "street") {
		t.Fatalf("default area-codes.level options = %+v, want street level", levelField)
	}

	menus := store.MenuItemsForPrincipal(store.CurrentPrincipal("admin"))
	for _, route := range []string{"/org-units", "/role-groups", "/area-codes"} {
		if !hasMenuRoute(menus, route) {
			t.Fatalf("default store menus missing %s: %+v", route, menus)
		}
	}
}

func TestPersonnelCapabilityReusesGovernanceRelations(t *testing.T) {
	store := NewStoreFromCapabilities(core.DefaultManifests())

	profileSchema, err := store.Schema("personnel-profiles")
	if err != nil {
		t.Fatalf("Schema(personnel-profiles) error = %v", err)
	}
	for _, item := range []struct {
		key      string
		resource string
		display  string
		path     string
	}{
		{key: "tenantCode", resource: "tenants"},
		{key: "orgUnitCode", resource: "org-units", display: "tree"},
		{key: "areaCode", resource: "area-codes", display: "tree", path: "path"},
		{key: "userCode", resource: "users"},
	} {
		field := fieldByKey(profileSchema.Fields, item.key)
		if field == nil || field.Relation == nil || field.Relation.Resource != item.resource {
			t.Fatalf("personnel-profiles.%s relation = %+v, want %s", item.key, field, item.resource)
		}
		if item.display != "" && (field.Relation.Display != item.display || field.Relation.ParentField != "parentCode") {
			t.Fatalf("personnel-profiles.%s relation display = %+v, want %s parentCode", item.key, field.Relation, item.display)
		}
		if item.path != "" && field.Relation.PathField != item.path {
			t.Fatalf("personnel-profiles.%s relation path field = %+v, want %s", item.key, field.Relation, item.path)
		}
		if !slices.Contains(profileSchema.SearchFields, item.key) {
			t.Fatalf("personnel-profiles search fields = %+v, want %s", profileSchema.SearchFields, item.key)
		}
	}

	positionSchema, err := store.Schema("positions")
	if err != nil {
		t.Fatalf("Schema(positions) error = %v", err)
	}
	if field := fieldByKey(positionSchema.Fields, "tenantCode"); field == nil || field.Relation == nil || field.Relation.Resource != "tenants" {
		t.Fatalf("positions.tenantCode relation = %+v, want tenants", field)
	}
	if field := fieldByKey(positionSchema.Fields, "orgUnitCode"); field == nil || field.Relation == nil || field.Relation.Resource != "org-units" || field.Relation.Display != "tree" {
		t.Fatalf("positions.orgUnitCode relation = %+v, want org-units tree", field)
	}

	assignmentSchema, err := store.Schema("position-assignments")
	if err != nil {
		t.Fatalf("Schema(position-assignments) error = %v", err)
	}
	for _, item := range []struct {
		key      string
		resource string
		display  string
	}{
		{key: "personnelCode", resource: "personnel-profiles"},
		{key: "positionCode", resource: "positions"},
		{key: "tenantCode", resource: "tenants"},
		{key: "orgUnitCode", resource: "org-units", display: "tree"},
	} {
		field := fieldByKey(assignmentSchema.Fields, item.key)
		if field == nil || field.Relation == nil || field.Relation.Resource != item.resource {
			t.Fatalf("position-assignments.%s relation = %+v, want %s", item.key, field, item.resource)
		}
		if item.display != "" && (field.Relation.Display != item.display || field.Relation.ParentField != "parentCode") {
			t.Fatalf("position-assignments.%s relation display = %+v, want %s parentCode", item.key, field.Relation, item.display)
		}
	}
}

func TestStoreIncludesConfigurationResourceSchemas(t *testing.T) {
	store := NewStoreFromCapabilities(core.DefaultManifests())

	for _, resource := range []string{"dictionaries", "parameters", "branding", "settings"} {
		schema, err := store.Schema(resource)
		if err != nil {
			t.Fatalf("Schema(%s) error = %v", resource, err)
		}
		if schema.Resource != resource {
			t.Fatalf("Schema(%s).Resource = %q", resource, schema.Resource)
		}
		records, err := store.List(resource)
		if err != nil {
			t.Fatalf("List(%s) error = %v", resource, err)
		}
		if len(records) == 0 {
			t.Fatalf("List(%s) returned no seed records", resource)
		}
	}

	settingsSchema, err := store.Schema("settings")
	if err != nil {
		t.Fatalf("Schema(settings) error = %v", err)
	}
	if settingsSchema.Permissions.Read != "admin:settings:read" || settingsSchema.Permissions.Update != "admin:settings:update" || settingsSchema.Permissions.Create != "" || settingsSchema.Permissions.Delete != "" {
		t.Fatalf("settings permissions = %+v, want read/update admin:settings only", settingsSchema.Permissions)
	}

	brandingSchema, err := store.Schema("branding")
	if err != nil {
		t.Fatalf("Schema(branding) error = %v", err)
	}
	if brandingSchema.Permissions.Read != "admin:branding:read" || brandingSchema.Permissions.Update != "admin:branding:update" || brandingSchema.Permissions.Create != "" || brandingSchema.Permissions.Delete != "" {
		t.Fatalf("branding permissions = %+v, want read/update admin:branding only", brandingSchema.Permissions)
	}
	if field := fieldByKey(brandingSchema.Fields, "defaultTheme"); field == nil || field.Type != "select" || !hasFieldOption(field.Options, "tech") || !hasFieldOption(field.Options, "warm") {
		t.Fatalf("branding.defaultTheme = %+v, want theme select options", field)
	}

	parameterSchema, err := store.Schema("parameters")
	if err != nil {
		t.Fatalf("Schema(parameters) error = %v", err)
	}
	if field := fieldByKey(parameterSchema.Fields, "value"); field == nil || field.Source != "values" || !field.Required {
		t.Fatalf("parameters.value = %+v, want required values field", field)
	}
}

func TestStoreAuditLogsSchemaExposesStructuredReadOnlyFields(t *testing.T) {
	store := NewStoreFromCapabilities(core.DefaultManifests())

	schema, err := store.Schema("audit-logs")
	if err != nil {
		t.Fatalf("Schema(audit-logs) error = %v", err)
	}
	if schema.DefaultSortKey != "createdAt" {
		t.Fatalf("audit-logs default sort = %q, want createdAt", schema.DefaultSortKey)
	}
	for _, key := range []string{"actor", "action", "resource", "targetId", "targetCode", "provider", "outcome", "createdAt", "traceId"} {
		field := fieldByKey(schema.Fields, key)
		if field == nil {
			t.Fatalf("audit-logs schema missing %s field: %+v", key, schema.Fields)
		}
		if field.Source != "values" || field.InForm || !field.InDetail || !field.ReadOnly {
			t.Fatalf("audit-logs.%s = %+v, want read-only values detail field outside forms", key, *field)
		}
	}
	for _, key := range []string{"actor", "action", "resource", "targetCode", "provider", "outcome", "traceId"} {
		if !slices.Contains(schema.SearchFields, key) {
			t.Fatalf("audit-logs search fields = %+v, want %s", schema.SearchFields, key)
		}
	}
	for _, key := range []string{"actor", "action", "resource", "targetCode", "provider", "outcome", "createdAt"} {
		field := fieldByKey(schema.Fields, key)
		if field == nil || !field.InTable || !field.Searchable || !field.Sortable {
			t.Fatalf("audit-logs.%s = %+v, want searchable sortable table field", key, field)
		}
	}
}

func TestCoreSchemaExposesOrganizationAndAreaRelations(t *testing.T) {
	store := NewStoreFromCapabilities(core.DefaultManifests())

	userSchema, err := store.Schema("users")
	if err != nil {
		t.Fatalf("Schema(users) error = %v", err)
	}
	for _, item := range []struct {
		key      string
		resource string
		multiple bool
		display  string
	}{
		{key: "tenantCode", resource: "tenants"},
		{key: "orgUnitCode", resource: "org-units", display: "tree"},
		{key: "areaCode", resource: "area-codes", display: "tree"},
		{key: "roles", resource: "roles", multiple: true},
	} {
		field := fieldByKey(userSchema.Fields, item.key)
		if field == nil || field.Relation == nil || field.Relation.Resource != item.resource || field.Relation.Multiple != item.multiple {
			t.Fatalf("users.%s relation = %+v, want %s multiple=%v", item.key, field, item.resource, item.multiple)
		}
		if item.display != "" && (field.Relation.Display != item.display || field.Relation.ParentField != "parentCode") {
			t.Fatalf("users.%s relation display = %+v, want %s parentCode", item.key, field.Relation, item.display)
		}
		if item.key == "areaCode" && field.Relation.PathField != "path" {
			t.Fatalf("users.%s relation path field = %+v, want path", item.key, field.Relation)
		}
	}

	orgSchema, err := store.Schema("org-units")
	if err != nil {
		t.Fatalf("Schema(org-units) error = %v", err)
	}
	typeField := fieldByKey(orgSchema.Fields, "type")
	for _, required := range []string{"group", "company", "branch", "organization", "department", "team", "store", "custom"} {
		if typeField == nil || !hasFieldOption(typeField.Options, required) {
			t.Fatalf("org-units.type options = %+v, missing %s", typeField, required)
		}
	}
	for _, item := range []struct {
		key      string
		resource string
		display  string
	}{
		{key: "tenantCode", resource: "tenants"},
		{key: "parentCode", resource: "org-units", display: "tree"},
		{key: "areaCode", resource: "area-codes", display: "tree"},
	} {
		field := fieldByKey(orgSchema.Fields, item.key)
		if field == nil || field.Relation == nil || field.Relation.Resource != item.resource {
			t.Fatalf("org-units.%s relation = %+v, want %s", item.key, field, item.resource)
		}
		if item.display != "" && (field.Relation.Display != item.display || field.Relation.ParentField != "parentCode") {
			t.Fatalf("org-units.%s relation display = %+v, want %s parentCode", item.key, field.Relation, item.display)
		}
		if item.key == "areaCode" && field.Relation.PathField != "path" {
			t.Fatalf("org-units.%s relation path field = %+v, want path", item.key, field.Relation)
		}
	}

	areaSchema, err := store.Schema("area-codes")
	if err != nil {
		t.Fatalf("Schema(area-codes) error = %v", err)
	}
	parentField := fieldByKey(areaSchema.Fields, "parentCode")
	if parentField == nil || parentField.Relation == nil || parentField.Relation.Resource != "area-codes" || parentField.Relation.Display != "tree" || parentField.Relation.ParentField != "parentCode" {
		t.Fatalf("area-codes.parentCode relation = %+v, want area-codes tree relation", parentField)
	}
	if parentField.Relation.PathField != "path" {
		t.Fatalf("area-codes.parentCode relation path field = %+v, want path", parentField.Relation)
	}
	levelField := fieldByKey(areaSchema.Fields, "level")
	if levelField == nil || !hasFieldOption(levelField.Options, "country") || !hasFieldOption(levelField.Options, "province") || !hasFieldOption(levelField.Options, "street") || !hasFieldOption(levelField.Options, "custom") {
		t.Fatalf("area-codes.level options = %+v, want country/province/street/custom", levelField)
	}
}

func TestCurrentPrincipalReadsRolesFieldWithLegacyRoleFallback(t *testing.T) {
	store := NewStoreFromCapabilities(core.DefaultManifests())

	if _, err := store.Update("users", "user-ops", WriteInput{
		Name:   "Operations User",
		Status: "enabled",
		Values: map[string]string{
			"roles":       "operator",
			"tenantCode":  "platform",
			"orgUnitCode": "platform-ops",
			"areaCode":    "110000",
		},
	}); err != nil {
		t.Fatalf("Update(user-ops) error = %v", err)
	}

	principal := store.CurrentPrincipal("ops")
	if principal.User.TenantCode != "platform" || principal.User.OrgUnitCode != "platform-ops" || principal.User.AreaCode != "110000" {
		t.Fatalf("CurrentPrincipal(ops).User scope = %+v, want platform/platform-ops/110000", principal.User)
	}
	if !slices.Contains(principal.Roles, "operator") {
		t.Fatalf("CurrentPrincipal(ops).Roles = %+v, want operator from roles field", principal.Roles)
	}
	if !slices.Contains(principal.Permissions, "admin:tenant:read") {
		t.Fatalf("CurrentPrincipal(ops).Permissions = %+v, want operator permissions", principal.Permissions)
	}
}

func TestValidateAdminPrincipalRequiresEnabledExistingUserWithEffectivePermission(t *testing.T) {
	t.Run("missing user", func(t *testing.T) {
		store := NewStoreFromCapabilities(core.DefaultManifests())
		if _, err := ValidateAdminPrincipal(store, "missing"); !errors.Is(err, ErrAdminPrincipalInvalid) {
			t.Fatalf("ValidateAdminPrincipal() error = %v, want missing user rejection", err)
		}
	})

	t.Run("disabled user", func(t *testing.T) {
		store := NewStoreFromCapabilities(core.DefaultManifests())
		if _, err := store.Update("users", "user-admin", WriteInput{
			Name: "Platform Admin", Status: "disabled", Values: map[string]string{"roles": "super-admin", "tenantCode": "platform"},
		}); err != nil {
			t.Fatalf("Update(user-admin) error = %v", err)
		}
		if _, err := ValidateAdminPrincipal(store, "admin"); !errors.Is(err, ErrAdminPrincipalInvalid) {
			t.Fatalf("ValidateAdminPrincipal() error = %v, want disabled user rejection", err)
		}
	})

	t.Run("no effective permission", func(t *testing.T) {
		store := NewStoreFromCapabilities(core.DefaultManifests())
		role, err := store.Create("roles", WriteInput{
			Code: "denied-role", Name: "Denied Role", Status: "enabled",
			Values: map[string]string{"dataScope": "all", "permissions": "admin:user:read", "denyPermissions": "admin:user:read"},
		})
		if err != nil {
			t.Fatalf("Create(denied-role) error = %v", err)
		}
		if _, err := store.Create("users", WriteInput{
			Code: "denied", Name: "Denied User", Status: "enabled", Values: map[string]string{"roles": role.Code, "tenantCode": "platform"},
		}); err != nil {
			t.Fatalf("Create(denied user) error = %v", err)
		}
		if _, err := ValidateAdminPrincipal(store, "denied"); !errors.Is(err, ErrAdminPrincipalInvalid) {
			t.Fatalf("ValidateAdminPrincipal() error = %v, want no effective permission rejection", err)
		}
	})

	t.Run("valid", func(t *testing.T) {
		store := NewStoreFromCapabilities(core.DefaultManifests())
		principal, err := ValidateAdminPrincipal(store, " admin ")
		if err != nil || principal.User.ID != "user-admin" || len(principal.Permissions) == 0 {
			t.Fatalf("ValidateAdminPrincipal() = %+v, %v", principal, err)
		}
	})
}

func TestQueryForPrincipalAppliesCurrentOrgDataScope(t *testing.T) {
	store := NewStoreFromCapabilities(core.DefaultManifests())
	principal := store.CurrentPrincipal("ops")

	users, err := store.QueryForPrincipal("users", QueryInput{Page: 1, PageSize: 10}, principal)
	if err != nil {
		t.Fatalf("QueryForPrincipal(users) error = %v", err)
	}
	if users.Total != 1 || !hasRecordCode(users.Items, "ops") || hasRecordCode(users.Items, "admin") {
		t.Fatalf("current_org users = total %d items %+v, want only ops", users.Total, users.Items)
	}

	orgs, err := store.QueryForPrincipal("org-units", QueryInput{Page: 1, PageSize: 10}, principal)
	if err != nil {
		t.Fatalf("QueryForPrincipal(org-units) error = %v", err)
	}
	if orgs.Total != 1 || !hasRecordCode(orgs.Items, "platform-ops") || hasRecordCode(orgs.Items, "platform-hq") {
		t.Fatalf("current_org orgs = total %d items %+v, want only platform-ops", orgs.Total, orgs.Items)
	}
}

func TestQueryForPrincipalUnionsChildOrgAndCustomOrgScopes(t *testing.T) {
	store := NewStoreFromCapabilities(core.DefaultManifests())
	if _, err := store.Update("users", "user-ops", WriteInput{
		Name:   "Operations User",
		Status: "enabled",
		Values: map[string]string{
			"roles":       "operator",
			"tenantCode":  "platform",
			"orgUnitCode": "platform-hq",
			"areaCode":    "110000",
		},
	}); err != nil {
		t.Fatalf("Update(user-ops) error = %v", err)
	}
	if _, err := store.Update("roles", "role-operator", WriteInput{
		Name:        "Operator",
		Status:      "enabled",
		Description: "Operator",
		Values: map[string]string{
			"groupCode":   "operations",
			"dataScope":   "current_and_children",
			"permissions": "admin:user:read",
		},
	}); err != nil {
		t.Fatalf("Update(role-operator current_and_children) error = %v", err)
	}

	childrenPrincipal := store.CurrentPrincipal("ops")
	orgs, err := store.QueryForPrincipal("org-units", QueryInput{Page: 1, PageSize: 10}, childrenPrincipal)
	if err != nil {
		t.Fatalf("QueryForPrincipal(org-units current_and_children) error = %v", err)
	}
	if orgs.Total != 2 || !hasRecordCode(orgs.Items, "platform-hq") || !hasRecordCode(orgs.Items, "platform-ops") {
		t.Fatalf("current_and_children orgs = total %d items %+v, want hq and ops", orgs.Total, orgs.Items)
	}

	if _, err := store.Update("roles", "role-operator", WriteInput{
		Name:        "Operator",
		Status:      "enabled",
		Description: "Operator",
		Values: map[string]string{
			"groupCode":         "operations",
			"dataScope":         "custom_orgs",
			"dataScopeOrgCodes": "platform-ops",
			"permissions":       "admin:user:read",
		},
	}); err != nil {
		t.Fatalf("Update(role-operator custom_orgs) error = %v", err)
	}

	customPrincipal := store.CurrentPrincipal("ops")
	customOrgs, err := store.QueryForPrincipal("org-units", QueryInput{Page: 1, PageSize: 10}, customPrincipal)
	if err != nil {
		t.Fatalf("QueryForPrincipal(org-units custom_orgs) error = %v", err)
	}
	if customOrgs.Total != 1 || !hasRecordCode(customOrgs.Items, "platform-ops") || hasRecordCode(customOrgs.Items, "platform-hq") {
		t.Fatalf("custom_orgs orgs = total %d items %+v, want only platform-ops", customOrgs.Total, customOrgs.Items)
	}
}

func TestQueryForPrincipalAppliesAreaDataScope(t *testing.T) {
	store := NewStoreFromCapabilities(core.DefaultManifests())
	if _, err := store.Create("area-codes", WriteInput{
		Code:        "110101",
		Name:        "Dongcheng",
		Status:      "enabled",
		Description: "Beijing district",
		Values:      map[string]string{"parentCode": "110000", "level": "district", "path": "CN/110000/110101", "sortOrder": "30"},
	}); err != nil {
		t.Fatalf("Create(area 110101) error = %v", err)
	}
	if _, err := store.Create("area-codes", WriteInput{
		Code:        "310000",
		Name:        "Shanghai",
		Status:      "enabled",
		Description: "Other municipality",
		Values:      map[string]string{"parentCode": "CN", "level": "province", "path": "CN/310000", "sortOrder": "40"},
	}); err != nil {
		t.Fatalf("Create(area 310000) error = %v", err)
	}
	if _, err := store.Create("org-units", WriteInput{
		Code:        "platform-field",
		Name:        "Field Team",
		Status:      "enabled",
		Description: "Field team",
		Values:      map[string]string{"type": "team", "tenantCode": "platform", "parentCode": "platform-hq", "areaCode": "110101", "sortOrder": "30"},
	}); err != nil {
		t.Fatalf("Create(org platform-field) error = %v", err)
	}
	if _, err := store.Create("org-units", WriteInput{
		Code:        "platform-shanghai",
		Name:        "Shanghai Team",
		Status:      "enabled",
		Description: "Shanghai team",
		Values:      map[string]string{"type": "team", "tenantCode": "platform", "parentCode": "platform-hq", "areaCode": "310000", "sortOrder": "40"},
	}); err != nil {
		t.Fatalf("Create(org platform-shanghai) error = %v", err)
	}
	if _, err := store.Create("users", WriteInput{
		Code:        "field",
		Name:        "Field User",
		Status:      "enabled",
		Description: "Field user",
		Values:      map[string]string{"roles": "operator", "tenantCode": "platform", "orgUnitCode": "platform-field", "areaCode": "110101"},
	}); err != nil {
		t.Fatalf("Create(user field) error = %v", err)
	}
	if _, err := store.Create("users", WriteInput{
		Code:        "shanghai",
		Name:        "Shanghai User",
		Status:      "enabled",
		Description: "Shanghai user",
		Values:      map[string]string{"roles": "operator", "tenantCode": "platform", "orgUnitCode": "platform-shanghai", "areaCode": "310000"},
	}); err != nil {
		t.Fatalf("Create(user shanghai) error = %v", err)
	}
	if _, err := store.Update("roles", "role-operator", WriteInput{
		Name:        "Operator",
		Status:      "enabled",
		Description: "Area-scoped operator",
		Values:      map[string]string{"groupCode": "operations", "dataScope": "current_and_children_areas", "permissions": "admin:user:read"},
	}); err != nil {
		t.Fatalf("Update(role-operator current_and_children_areas) error = %v", err)
	}

	areaPrincipal := store.CurrentPrincipal("ops")
	areas, err := store.QueryForPrincipal("area-codes", QueryInput{Page: 1, PageSize: 10}, areaPrincipal)
	if err != nil {
		t.Fatalf("QueryForPrincipal(area-codes current_and_children_areas) error = %v", err)
	}
	if areas.Total != 2 || !hasRecordCode(areas.Items, "110000") || !hasRecordCode(areas.Items, "110101") || hasRecordCode(areas.Items, "310000") || hasRecordCode(areas.Items, "CN") {
		t.Fatalf("current_and_children_areas areas = total %d items %+v, want 110000 and 110101 only", areas.Total, areas.Items)
	}

	orgs, err := store.QueryForPrincipal("org-units", QueryInput{Page: 1, PageSize: 10}, areaPrincipal)
	if err != nil {
		t.Fatalf("QueryForPrincipal(org-units current_and_children_areas) error = %v", err)
	}
	if orgs.Total != 3 || !hasRecordCode(orgs.Items, "platform-hq") || !hasRecordCode(orgs.Items, "platform-ops") || !hasRecordCode(orgs.Items, "platform-field") || hasRecordCode(orgs.Items, "platform-shanghai") {
		t.Fatalf("current_and_children_areas orgs = total %d items %+v, want Beijing orgs only", orgs.Total, orgs.Items)
	}

	if _, err := store.Update("roles", "role-operator", WriteInput{
		Name:        "Operator",
		Status:      "enabled",
		Description: "Custom area operator",
		Values:      map[string]string{"groupCode": "operations", "dataScope": "custom_areas", "dataScopeAreaCodes": "310000", "permissions": "admin:user:read"},
	}); err != nil {
		t.Fatalf("Update(role-operator custom_areas) error = %v", err)
	}

	customPrincipal := store.CurrentPrincipal("ops")
	customOrgs, err := store.QueryForPrincipal("org-units", QueryInput{Page: 1, PageSize: 10}, customPrincipal)
	if err != nil {
		t.Fatalf("QueryForPrincipal(org-units custom_areas) error = %v", err)
	}
	if customOrgs.Total != 1 || !hasRecordCode(customOrgs.Items, "platform-shanghai") || hasRecordCode(customOrgs.Items, "platform-field") {
		t.Fatalf("custom_areas orgs = total %d items %+v, want only platform-shanghai", customOrgs.Total, customOrgs.Items)
	}
}

func TestStoreBuildsCasbinAuthorizerFromRoleResources(t *testing.T) {
	store := NewStoreFromCapabilities(core.DefaultManifests())

	authorizer, err := store.CasbinAuthorizer()
	if err != nil {
		t.Fatalf("CasbinAuthorizer() error = %v", err)
	}
	if !authorizer.Can("ops", "platform", "admin:tenant:read", "read") {
		t.Fatalf("ops cannot read tenants through role-backed Casbin policy")
	}
	if authorizer.Can("ops", "platform", "admin:user:read", "read") {
		t.Fatalf("ops can read users before role update, want false")
	}

	if _, err := store.Update("roles", "role-operator", WriteInput{
		Name:        "Operator",
		Status:      "enabled",
		Description: "Updated operator",
		Values:      map[string]string{"groupCode": "operations", "dataScope": "current_org", "permissions": "admin:*", "denyPermissions": "admin:tenant:read"},
	}); err != nil {
		t.Fatalf("Update(role-operator) error = %v", err)
	}
	authorizer, err = store.CasbinAuthorizer()
	if err != nil {
		t.Fatalf("CasbinAuthorizer() after update error = %v", err)
	}
	if !authorizer.Can("ops", "platform", "admin:user:read", "read") {
		t.Fatalf("ops cannot read users after role update, want true")
	}
	if authorizer.Can("ops", "platform", "admin:tenant:read", "read") {
		t.Fatalf("ops can read explicitly denied tenant read after wildcard allow, want false")
	}
	if !authorizer.Can("ops", "platform", "admin:tenant:update", "update") {
		t.Fatalf("ops cannot update tenants after wildcard allow with read-only deny, want true")
	}
}

func TestFileBackedStorePersistsResourceMutations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "admin-resources.json")
	store, err := NewFileBackedStoreFromCapabilities(path, core.DefaultManifests())
	if err != nil {
		t.Fatalf("NewFileBackedStoreFromCapabilities() error = %v", err)
	}

	created, err := store.Create("tenants", WriteInput{
		Code:        "acme",
		Name:        "Acme Tenant",
		Status:      "enabled",
		Description: "Persisted tenant",
		Values:      map[string]string{"isolation": "sandbox"},
	})
	if err != nil {
		t.Fatalf("Create(tenants) error = %v", err)
	}

	reloaded, err := NewFileBackedStoreFromCapabilities(path, core.DefaultManifests())
	if err != nil {
		t.Fatalf("reload file-backed store error = %v", err)
	}
	tenants, err := reloaded.List("tenants")
	if err != nil {
		t.Fatalf("List(tenants) error = %v", err)
	}
	if !hasRecordID(tenants, created.ID) {
		t.Fatalf("reloaded tenants missing created record %q: %+v", created.ID, tenants)
	}
}

func TestFileBackedStorePersistsRolePermissionsForDynamicMenus(t *testing.T) {
	path := filepath.Join(t.TempDir(), "admin-resources.json")
	store, err := NewFileBackedStoreFromCapabilities(path, core.DefaultManifests())
	if err != nil {
		t.Fatalf("NewFileBackedStoreFromCapabilities() error = %v", err)
	}
	if _, err := store.Update("roles", "role-operator", WriteInput{
		Name:        "Operator",
		Status:      "enabled",
		Description: "Updated operator",
		Values:      map[string]string{"groupCode": "operations", "dataScope": "current_org", "permissions": "admin:user:read"},
	}); err != nil {
		t.Fatalf("Update(role-operator) error = %v", err)
	}

	reloaded, err := NewFileBackedStoreFromCapabilities(path, core.DefaultManifests())
	if err != nil {
		t.Fatalf("reload file-backed store error = %v", err)
	}
	menus := reloaded.MenuItemsForPrincipal(reloaded.CurrentPrincipal("ops"))
	if !hasMenuRoute(menus, "/users") {
		t.Fatalf("menus after reload = %+v, want /users from persisted role permissions", menus)
	}
	if hasMenuRoute(menus, "/tenants") {
		t.Fatalf("menus after reload = %+v, want /tenants removed by persisted role permissions", menus)
	}
}

func TestStorePersistsThroughRepositoryPort(t *testing.T) {
	repository := &recordingRepository{}
	store, err := NewRepositoryBackedStoreFromCapabilities(repository, core.DefaultManifests())
	if err != nil {
		t.Fatalf("NewRepositoryBackedStoreFromCapabilities() error = %v", err)
	}

	created, err := store.Create("tenants", WriteInput{
		Code:   "repo",
		Name:   "Repository Tenant",
		Status: "enabled",
		Values: map[string]string{"isolation": "sandbox"},
	})
	if err != nil {
		t.Fatalf("Create(tenants) error = %v", err)
	}

	if repository.saveCount != 1 {
		t.Fatalf("saveCount = %d, want 1", repository.saveCount)
	}
	if !hasRecordID(repository.snapshot.Resources["tenants"], created.ID) {
		t.Fatalf("repository snapshot missing created tenant %q: %+v", created.ID, repository.snapshot.Resources["tenants"])
	}
}

func TestRepositoryBackedStoreRollsBackFullSnapshotOnConflict(t *testing.T) {
	repository := &conflictingRepository{snapshot: ResourceSnapshot{
		Revision: 9,
		NextID:   2077,
		Resources: map[string][]Record{
			"tenants": {{ID: "tenant-a", Code: "a", Name: "A", Status: "enabled"}},
		},
	}}
	store, err := NewRepositoryBackedStoreFromCapabilities(repository, core.DefaultManifests())
	if err != nil {
		t.Fatalf("NewRepositoryBackedStoreFromCapabilities() error = %v", err)
	}
	want := store.snapshotLocked()

	if _, err := store.Create("tenants", WriteInput{
		Code:   "b",
		Name:   "B",
		Status: "enabled",
		Values: map[string]string{"isolation": "sandbox"},
	}); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("Create(tenants) error = %v, want ErrRevisionConflict", err)
	}
	got := store.snapshotLocked()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("snapshot after conflict = %#v, want rollback to %#v", got, want)
	}
}

func TestRepositoryBackedStoreReloadsBeforeMutationAndPreservesConcurrentRecord(t *testing.T) {
	repository := &revisionMemoryRepository{snapshot: ResourceSnapshot{Resources: map[string][]Record{}}}
	first, err := NewRepositoryBackedStoreFromCapabilities(repository, core.DefaultManifests())
	if err != nil {
		t.Fatalf("NewRepositoryBackedStoreFromCapabilities(first) error = %v", err)
	}
	second, err := NewRepositoryBackedStoreFromCapabilities(repository, core.DefaultManifests())
	if err != nil {
		t.Fatalf("NewRepositoryBackedStoreFromCapabilities(second) error = %v", err)
	}

	createdFirst, err := first.Create("tenants", WriteInput{
		Code: "first", Name: "First", Status: "enabled", Values: map[string]string{"isolation": "sandbox"},
	})
	if err != nil {
		t.Fatalf("first.Create(tenants) error = %v", err)
	}
	createdSecond, err := second.Create("tenants", WriteInput{
		Code: "second", Name: "Second", Status: "enabled", Values: map[string]string{"isolation": "sandbox"},
	})
	if err != nil {
		t.Fatalf("second.Create(tenants) error = %v", err)
	}

	loaded, err := repository.Load(context.Background())
	if err != nil {
		t.Fatalf("repository.Load() error = %v", err)
	}
	if !hasRecordID(loaded.Resources["tenants"], createdFirst.ID) || !hasRecordID(loaded.Resources["tenants"], createdSecond.ID) {
		t.Fatalf("persisted tenants = %+v, want both concurrent records", loaded.Resources["tenants"])
	}
}

type recordingRepository struct {
	snapshot  ResourceSnapshot
	saveCount int
}

func (r *recordingRepository) Load(context.Context) (ResourceSnapshot, error) {
	return r.snapshot, nil
}

func (r *recordingRepository) Save(_ context.Context, snapshot ResourceSnapshot) (uint64, error) {
	r.snapshot = snapshot
	r.saveCount++
	r.snapshot.Revision++
	return r.snapshot.Revision, nil
}

type conflictingRepository struct {
	snapshot ResourceSnapshot
}

func (r *conflictingRepository) Load(context.Context) (ResourceSnapshot, error) {
	return ResourceSnapshot{
		Revision:  r.snapshot.Revision,
		NextID:    r.snapshot.NextID,
		Resources: cloneResourceMap(r.snapshot.Resources),
	}, nil
}

func (r *conflictingRepository) Save(context.Context, ResourceSnapshot) (uint64, error) {
	return 0, &RevisionConflictError{Expected: r.snapshot.Revision, Actual: r.snapshot.Revision + 1}
}

type revisionMemoryRepository struct {
	snapshot ResourceSnapshot
}

func (r *revisionMemoryRepository) Load(context.Context) (ResourceSnapshot, error) {
	return ResourceSnapshot{
		Revision:  r.snapshot.Revision,
		NextID:    r.snapshot.NextID,
		Resources: cloneResourceMap(r.snapshot.Resources),
	}, nil
}

func (r *revisionMemoryRepository) Save(_ context.Context, snapshot ResourceSnapshot) (uint64, error) {
	if snapshot.Revision != r.snapshot.Revision {
		return 0, &RevisionConflictError{Expected: snapshot.Revision, Actual: r.snapshot.Revision}
	}
	snapshot.Revision++
	snapshot.Resources = cloneResourceMap(snapshot.Resources)
	r.snapshot = snapshot
	return snapshot.Revision, nil
}

func hasMenuRoute(menus []MenuItem, route string) bool {
	for _, menu := range menus {
		if menu.Route == route {
			return true
		}
	}
	return false
}

func menuParentForRoute(menus []MenuItem, route string) string {
	item := menuForRoute(menus, route)
	if item == nil {
		return ""
	}
	return item.Parent
}

func menuForRoute(menus []MenuItem, route string) *MenuItem {
	for _, menu := range menus {
		if menu.Route == route {
			return &menu
		}
	}
	return nil
}

func hasRecordID(records []Record, id string) bool {
	for _, record := range records {
		if record.ID == id {
			return true
		}
	}
	return false
}

func hasRecordCode(records []Record, code string) bool {
	for _, record := range records {
		if record.Code == code {
			return true
		}
	}
	return false
}

func recordByCode(records []Record, code string) *Record {
	for _, record := range records {
		if record.Code == code {
			return &record
		}
	}
	return nil
}

func fieldByKey(fields []FieldDefinition, key string) *FieldDefinition {
	for _, field := range fields {
		if field.Key == key {
			return &field
		}
	}
	return nil
}

func requireRelationField(t *testing.T, schema Schema, key string, resource string, display string, multiple bool, required bool) {
	t.Helper()

	field := fieldByKey(schema.Fields, key)
	if field == nil {
		t.Fatalf("%s.%s field is missing", schema.Resource, key)
	}
	if field.Required != required {
		t.Fatalf("%s.%s required = %v, want %v", schema.Resource, key, field.Required, required)
	}
	if field.Relation == nil {
		t.Fatalf("%s.%s relation is missing", schema.Resource, key)
	}
	if field.Relation.Resource != resource {
		t.Fatalf("%s.%s relation resource = %q, want %q", schema.Resource, key, field.Relation.Resource, resource)
	}
	if field.Relation.Multiple != multiple {
		t.Fatalf("%s.%s relation multiple = %v, want %v", schema.Resource, key, field.Relation.Multiple, multiple)
	}
	if display != "" && field.Relation.Display != display {
		t.Fatalf("%s.%s relation display = %q, want %q", schema.Resource, key, field.Relation.Display, display)
	}
	if display == "tree" && field.Relation.ParentField != "parentCode" {
		t.Fatalf("%s.%s relation parentField = %q, want parentCode", schema.Resource, key, field.Relation.ParentField)
	}
}

func hasFieldOption(options []FieldOption, value string) bool {
	for _, option := range options {
		if option.Value == value {
			return true
		}
	}
	return false
}
