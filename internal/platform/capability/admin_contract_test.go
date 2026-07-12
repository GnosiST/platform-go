package capability

import (
	"strings"
	"testing"
)

func TestValidateAdminSurfaceAcceptsUniqueEnabledResources(t *testing.T) {
	manifests := []Manifest{
		{
			ID: "feedback",
			Admin: AdminSurface{
				Resources: []AdminResource{
					{
						Resource:         "feedback-tickets",
						Title:            Text("反馈工单", "Feedback Tickets"),
						Description:      Text("用户反馈与处理记录。", "User feedback and handling records."),
						PermissionPrefix: "admin:feedback-ticket",
						Menu:             AdminMenu{Route: "/feedback-tickets", Group: "operations", Icon: "audit", Order: 250},
					},
				},
			},
		},
	}

	if err := ValidateAdminSurface(manifests); err != nil {
		t.Fatalf("ValidateAdminSurface() error = %v", err)
	}
}

func TestValidateAdminSurfaceAcceptsExternalMenuURL(t *testing.T) {
	manifests := []Manifest{
		{
			ID: "docs",
			Admin: AdminSurface{
				Resources: []AdminResource{
					{
						Resource:         "external-docs",
						Title:            Text("外部文档", "External Docs"),
						Description:      Text("外部文档入口。", "External documentation entry."),
						PermissionPrefix: "admin:external-doc",
						Menu: AdminMenu{
							Route:    "https://docs.example.com/platform",
							Group:    "foundation",
							Icon:     "overview",
							Order:    10,
							External: true,
						},
					},
				},
			},
		},
	}

	if err := ValidateAdminSurface(manifests); err != nil {
		t.Fatalf("ValidateAdminSurface() error = %v", err)
	}
}

func TestValidateAdminSurfaceRejectsNonExternalURLRoute(t *testing.T) {
	manifests := []Manifest{
		{
			ID: "docs",
			Admin: AdminSurface{
				Resources: []AdminResource{
					validAdminResource("external-docs", "https://docs.example.com/platform", "admin:external-doc"),
				},
			},
		},
	}

	err := ValidateAdminSurface(manifests)
	if err == nil {
		t.Fatalf("ValidateAdminSurface() error = nil, want invalid route")
	}
	if !strings.Contains(err.Error(), "menu route must start with / or be an http(s) URL when external") {
		t.Fatalf("ValidateAdminSurface() error = %v, want invalid route", err)
	}
}

func TestValidateAdminSurfaceRejectsMissingRequiredFields(t *testing.T) {
	manifests := []Manifest{
		{
			ID: "broken",
			Admin: AdminSurface{
				Resources: []AdminResource{
					{
						Resource:         "broken-resource",
						PermissionPrefix: "admin:broken-resource",
						Menu:             AdminMenu{Route: "/broken-resource", Group: "foundation", Icon: "overview", Order: 10},
					},
				},
			},
		},
	}

	err := ValidateAdminSurface(manifests)
	if err == nil {
		t.Fatalf("ValidateAdminSurface() error = nil, want missing title")
	}
	if !strings.Contains(err.Error(), "title is required") {
		t.Fatalf("ValidateAdminSurface() error = %v, want title is required", err)
	}
}

func TestValidateAdminSurfaceAcceptsSupportedFormLayout(t *testing.T) {
	resource := validAdminResource("demo-resources", "/demo-resources", "admin:demo")
	resource.FormLayout = "side-detail-preview"

	if err := ValidateAdminSurface([]Manifest{{ID: "demo", Admin: AdminSurface{Resources: []AdminResource{resource}}}}); err != nil {
		t.Fatalf("ValidateAdminSurface() error = %v", err)
	}
}

func TestValidateAdminSurfaceRejectsUnsupportedFormLayout(t *testing.T) {
	resource := validAdminResource("demo-resources", "/demo-resources", "admin:demo")
	resource.FormLayout = "runtime-component-path"

	err := ValidateAdminSurface([]Manifest{{ID: "demo", Admin: AdminSurface{Resources: []AdminResource{resource}}}})
	if err == nil {
		t.Fatalf("ValidateAdminSurface() error = nil, want invalid form layout")
	}
	if !strings.Contains(err.Error(), "form layout must be one of") {
		t.Fatalf("ValidateAdminSurface() error = %v, want invalid form layout", err)
	}
}

func TestValidateAdminSurfaceRejectsInvalidPermissionPrefix(t *testing.T) {
	resource := validAdminResource("demo-resources", "/demo-resources", "demo:permission")

	err := ValidateAdminSurface([]Manifest{{ID: "demo", Admin: AdminSurface{Resources: []AdminResource{resource}}}})
	if err == nil {
		t.Fatalf("ValidateAdminSurface() error = nil, want invalid permission prefix")
	}
	if !strings.Contains(err.Error(), "permission prefix must match admin:<resource>") {
		t.Fatalf("ValidateAdminSurface() error = %v, want invalid permission prefix", err)
	}
}

func TestValidateAdminSurfaceRejectsCustomSurfacePermissionsOutsideResourcePrefix(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*AdminResource)
		wantErr string
	}{
		{
			name: "action",
			mutate: func(resource *AdminResource) {
				resource.Actions = []AdminResourceAction{
					{
						Key:        "approve",
						Label:      Text("通过", "Approve"),
						Kind:       "row",
						Permission: "admin:other:update",
					},
				}
			},
			wantErr: "action approve permission must start with \"admin:demo:\"",
		},
		{
			name: "panel",
			mutate: func(resource *AdminResource) {
				resource.Panels = []AdminResourcePanel{
					{
						Key:        "audit",
						Label:      Text("审计", "Audit"),
						Kind:       "audit",
						Permission: "admin:other:read",
					},
				}
			},
			wantErr: "panel audit permission must start with \"admin:demo:\"",
		},
		{
			name: "runtime slot",
			mutate: func(resource *AdminResource) {
				resource.RuntimeSlots = []AdminRuntimeSlot{
					{
						SlotID:      "platform.record-summary",
						Region:      "side.preview",
						Label:       Text("记录摘要", "Record Summary"),
						Description: Text("展示当前记录关键字段。", "Shows key fields for the current record."),
						Permission:  "admin:other:read",
						DataBinding: AdminRuntimeSlotDataBinding{Mode: "record", Fields: []string{"status"}},
					},
				}
			},
			wantErr: "runtime slot platform.record-summary permission must start with \"admin:demo:\"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource := validAdminResource("demo-resources", "/demo-resources", "admin:demo")
			tt.mutate(&resource)

			err := ValidateAdminSurface([]Manifest{{ID: "demo", Admin: AdminSurface{Resources: []AdminResource{resource}}}})
			if err == nil {
				t.Fatalf("ValidateAdminSurface() error = nil, want invalid permission")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateAdminSurface() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateAdminSurfaceAcceptsSchemaFieldsFormGroupsSearchAndSort(t *testing.T) {
	resource := validAdminResource("demo-resources", "/demo-resources", "admin:demo")
	resource.FormGroups = []AdminFormGroup{
		{
			Key:         "basic",
			Label:       Text("基础信息", "Basic"),
			Description: Text("常用字段。", "Common fields."),
		},
	}
	resource.Fields = []AdminField{
		{
			Key:    "category",
			Label:  Text("分类", "Category"),
			Type:   "select",
			Source: "values",
			Group:  "basic",
			Help:   Text("选择资源分类。", "Choose the resource category."),
			Options: []AdminFieldOption{
				{Value: "system", Label: Text("系统", "System")},
				{Value: "business", Label: Text("业务", "Business")},
			},
			Searchable: true,
			Sortable:   true,
			InTable:    true,
			InForm:     true,
		},
	}
	resource.SearchFields = []string{"name", "category"}
	resource.DefaultSortKey = "category"

	if err := ValidateAdminSurface([]Manifest{{ID: "demo", Admin: AdminSurface{Resources: []AdminResource{resource}}}}); err != nil {
		t.Fatalf("ValidateAdminSurface() error = %v", err)
	}
}

func TestValidateAdminSurfaceValidatesFieldSecurityPolicies(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		field   AdminField
		wantErr string
	}{
		{name: "unsupported sensitivity", field: AdminField{Sensitivity: "classified"}, wantErr: "sensitivity is unsupported"},
		{name: "unsupported storage", field: AdminField{StorageMode: "digest"}, wantErr: "storageMode is unsupported"},
		{name: "unsupported response", field: AdminField{ResponseMode: "redacted"}, wantErr: "responseMode is unsupported"},
		{name: "unsupported export", field: AdminField{ExportMode: "redacted"}, wantErr: "exportMode is unsupported"},
		{name: "sensitive record field", field: AdminField{Source: "record", Sensitivity: FieldSensitivitySensitive, StorageMode: FieldStorageHashed, ResponseMode: FieldProjectionOmitted, ExportMode: FieldProjectionOmitted}, wantErr: "cannot use record storage"},
		{name: "plain personal", field: AdminField{Sensitivity: FieldSensitivityPersonal, StorageMode: FieldStoragePlain}, wantErr: "personal values require masked or protected storage"},
		{name: "plain secret", field: AdminField{Sensitivity: FieldSensitivitySecret, StorageMode: FieldStoragePlain}, wantErr: "require protected storage"},
		{name: "masked public", field: AdminField{Sensitivity: FieldSensitivityPublic, StorageMode: FieldStorageMasked, ResponseMode: FieldProjectionMasked, ExportMode: FieldProjectionMasked}, wantErr: "masked storage requires personal sensitivity"},
		{name: "masked full response", field: AdminField{Sensitivity: FieldSensitivityPersonal, StorageMode: FieldStorageMasked, ResponseMode: FieldProjectionFull, ExportMode: FieldProjectionMasked}, wantErr: "masked storage must use masked or omitted response and export"},
		{name: "masked privileged export", field: AdminField{Sensitivity: FieldSensitivityPersonal, StorageMode: FieldStorageMasked, ResponseMode: FieldProjectionMasked, ExportMode: FieldProjectionPrivileged}, wantErr: "masked storage must use masked or omitted response and export"},
		{name: "hashed response", field: AdminField{Sensitivity: FieldSensitivitySecret, StorageMode: FieldStorageHashed, ResponseMode: FieldProjectionFull, ExportMode: FieldProjectionOmitted}, wantErr: "must be omitted from response and export"},
		{name: "encrypted full response", field: AdminField{
			Sensitivity: FieldSensitivitySensitive, StorageMode: FieldStorageEncrypted, ResponseMode: FieldProjectionFull, ExportMode: FieldProjectionOmitted,
			Protection: &AdminFieldProtection{Format: "aes-256-gcm-v1", Normalization: "raw-v1"},
		}, wantErr: "must use privileged or omitted response and export"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource := validAdminResource("demo-resources", "/demo-resources", "admin:demo")
			field := tt.field
			field.Key = tt.key
			if field.Key == "" {
				field.Key = "protectedValue"
			}
			field.Label = Text("保护值", "Protected Value")
			field.Type = "text"
			if field.Source == "" {
				field.Source = "values"
			}
			resource.Fields = []AdminField{field}

			err := ValidateAdminSurface([]Manifest{{ID: "demo", Admin: AdminSurface{Resources: []AdminResource{resource}}}})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateAdminSurface() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateAdminSurfaceUsesExplicitPolicyForSecurityLikeFieldNames(t *testing.T) {
	resource := validAdminResource("contact-records", "/contact-records", "admin:contact-record")
	for _, key := range []string{"contactPhone", "email", "address", "apiToken"} {
		resource.Fields = append(resource.Fields, AdminField{
			Key: key, Label: Text("公开字段", "Public Field"), Type: "text", Source: "values",
			Sensitivity: FieldSensitivityPublic, StorageMode: FieldStoragePlain,
			ResponseMode: FieldProjectionFull, ExportMode: FieldProjectionFull,
			Searchable: true, Filterable: true, Sortable: true,
		})
	}

	if err := ValidateAdminSurface([]Manifest{{ID: "contacts", Admin: AdminSurface{Resources: []AdminResource{resource}}}}); err != nil {
		t.Fatalf("ValidateAdminSurface() error = %v", err)
	}
}

func TestValidateAdminSurfaceAcceptsDefaultAndProtectedFieldSecurityPolicies(t *testing.T) {
	resource := validAdminResource("demo-resources", "/demo-resources", "admin:demo")
	resource.Fields = []AdminField{
		{Key: "title", Label: Text("标题", "Title"), Type: "text", Source: "values"},
		{Key: "maskedPhone", Label: Text("脱敏手机号", "Masked Phone"), Type: "text", Source: "values", Sensitivity: FieldSensitivityPersonal, StorageMode: FieldStorageMasked, ResponseMode: FieldProjectionMasked, ExportMode: FieldProjectionMasked},
		{Key: "maskedToken", Label: Text("脱敏令牌", "Masked Token"), Type: "text", Source: "values", Sensitivity: FieldSensitivityPersonal, StorageMode: FieldStorageMasked, ResponseMode: FieldProjectionMasked, ExportMode: FieldProjectionOmitted},
		{Key: "tokenHash", Label: Text("令牌哈希", "Token Hash"), Type: "text", Source: "values", Sensitivity: FieldSensitivitySecret, StorageMode: FieldStorageHashed, ResponseMode: FieldProjectionOmitted, ExportMode: FieldProjectionOmitted},
		{Key: "tokenPrefix", Label: Text("令牌前缀", "Token Prefix"), Type: "text", Source: "values"},
		{Key: "tokenType", Label: Text("令牌类型", "Token Type"), Type: "text", Source: "values"},
		{Key: "sessionType", Label: Text("会话类型", "Session Type"), Type: "text", Source: "values"},
		{Key: "sessionStatus", Label: Text("会话状态", "Session Status"), Type: "text", Source: "values"},
	}

	if err := ValidateAdminSurface([]Manifest{{ID: "demo", Admin: AdminSurface{Resources: []AdminResource{resource}}}}); err != nil {
		t.Fatalf("ValidateAdminSurface() error = %v", err)
	}
}

func TestValidateAdminSurfaceAcceptsCustomSensitiveEncryptedFieldProtection(t *testing.T) {
	resource := validAdminResource("custom-records", "/custom-records", "admin:custom-record")
	resource.Protection = &AdminResourceProtection{SchemaVersion: 1, Scope: "global"}
	resource.Fields = []AdminField{
		{
			Key: "governmentReference", Label: Text("政府引用", "Government Reference"), Type: "text", Source: "values",
			Sensitivity: FieldSensitivitySensitive, StorageMode: FieldStorageEncrypted,
			ResponseMode: FieldProjectionPrivileged, ExportMode: FieldProjectionOmitted, Filterable: true,
			Protection: &AdminFieldProtection{
				Format: "aes-256-gcm-v1", Normalization: "trim-v1", BlindIndexNamespace: "custom-government-reference",
			},
		},
	}

	if err := ValidateAdminSurface([]Manifest{{ID: "custom", Admin: AdminSurface{Resources: []AdminResource{resource}}}}); err != nil {
		t.Fatalf("ValidateAdminSurface() error = %v", err)
	}
}

func TestValidateAdminSurfaceRejectsIncompleteEncryptedProtection(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*AdminResource, *AdminField)
		wantErr string
	}{
		{name: "missing field protection", mutate: func(_ *AdminResource, field *AdminField) { field.Protection = nil }, wantErr: "encrypted storage requires protection metadata"},
		{name: "missing format", mutate: func(_ *AdminResource, field *AdminField) { field.Protection.Format = "" }, wantErr: "protection format is required"},
		{name: "unsupported format", mutate: func(_ *AdminResource, field *AdminField) { field.Protection.Format = "aes-cbc-v1" }, wantErr: "protection format is unsupported"},
		{name: "missing normalization", mutate: func(_ *AdminResource, field *AdminField) { field.Protection.Normalization = "" }, wantErr: "protection normalization is required"},
		{name: "unsupported normalization", mutate: func(_ *AdminResource, field *AdminField) { field.Protection.Normalization = "email" }, wantErr: "protection normalization is unsupported"},
		{name: "missing resource protection", mutate: func(resource *AdminResource, _ *AdminField) { resource.Protection = nil }, wantErr: "encrypted fields require resource protection metadata"},
		{name: "missing schema version", mutate: func(resource *AdminResource, _ *AdminField) { resource.Protection.SchemaVersion = 0 }, wantErr: "protection schemaVersion is required"},
		{name: "missing scope", mutate: func(resource *AdminResource, _ *AdminField) { resource.Protection.Scope = "" }, wantErr: "protection scope is required"},
		{name: "unsupported scope", mutate: func(resource *AdminResource, _ *AdminField) { resource.Protection.Scope = "user" }, wantErr: "protection scope is unsupported"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource := validEncryptedAdminResource()
			tt.mutate(&resource, &resource.Fields[1])
			err := ValidateAdminSurface([]Manifest{{ID: "custom", Admin: AdminSurface{Resources: []AdminResource{resource}}}})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateAdminSurface() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateAdminSurfaceRejectsInvalidTenantFieldProtectionScope(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*AdminResource)
		wantErr string
	}{
		{name: "missing tenant field", mutate: func(resource *AdminResource) { resource.Protection.TenantField = "" }, wantErr: "tenantField is required"},
		{name: "undeclared tenant field", mutate: func(resource *AdminResource) { resource.Protection.TenantField = "accountCode" }, wantErr: "tenantField \"accountCode\" is not declared"},
		{name: "protected tenant field", mutate: func(resource *AdminResource) {
			resource.Fields[0].StorageMode = FieldStorageHashed
			resource.Fields[0].ResponseMode = FieldProjectionOmitted
			resource.Fields[0].ExportMode = FieldProjectionOmitted
		}, wantErr: "tenantField \"tenantCode\" must use plain storage"},
		{name: "optional tenant field", mutate: func(resource *AdminResource) { resource.Fields[0].Required = false }, wantErr: "tenantField \"tenantCode\" must be required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource := validEncryptedAdminResource()
			tt.mutate(&resource)
			err := ValidateAdminSurface([]Manifest{{ID: "custom", Admin: AdminSurface{Resources: []AdminResource{resource}}}})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateAdminSurface() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateAdminSurfaceRejectsInvalidBlindIndexNamespace(t *testing.T) {
	t.Run("non canonical", func(t *testing.T) {
		resource := validEncryptedAdminResource()
		resource.Fields[1].Protection.BlindIndexNamespace = "Custom Government Reference"
		err := ValidateAdminSurface([]Manifest{{ID: "custom", Admin: AdminSurface{Resources: []AdminResource{resource}}}})
		if err == nil || !strings.Contains(err.Error(), "blindIndexNamespace must be canonical") {
			t.Fatalf("ValidateAdminSurface() error = %v, want canonical namespace error", err)
		}
	})

	t.Run("duplicate", func(t *testing.T) {
		resource := validEncryptedAdminResource()
		resource.Fields = append(resource.Fields, AdminField{
			Key: "secondaryReference", Label: Text("次级引用", "Secondary Reference"), Type: "text", Source: "values",
			Sensitivity: FieldSensitivitySensitive, StorageMode: FieldStorageEncrypted,
			ResponseMode: FieldProjectionOmitted, ExportMode: FieldProjectionOmitted,
			Protection: &AdminFieldProtection{Format: "aes-256-gcm-v1", Normalization: "raw-v1", BlindIndexNamespace: "custom-government-reference"},
		})
		err := ValidateAdminSurface([]Manifest{{ID: "custom", Admin: AdminSurface{Resources: []AdminResource{resource}}}})
		if err == nil || !strings.Contains(err.Error(), "duplicate blindIndexNamespace") {
			t.Fatalf("ValidateAdminSurface() error = %v, want duplicate namespace error", err)
		}
	})
}

func TestValidateAdminSurfaceRejectsEncryptedKeywordRangeAndSortConfiguration(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*AdminResource)
		wantErr string
	}{
		{name: "keyword field", mutate: func(resource *AdminResource) { resource.Fields[1].Searchable = true }, wantErr: "encrypted fields cannot use keyword search"},
		{name: "keyword search list", mutate: func(resource *AdminResource) { resource.SearchFields = []string{"governmentReference"} }, wantErr: "encrypted fields cannot use keyword search"},
		{name: "range filtering without blind index", mutate: func(resource *AdminResource) { resource.Fields[1].Protection.BlindIndexNamespace = "" }, wantErr: "encrypted filtering requires a blindIndexNamespace"},
		{name: "sortable field", mutate: func(resource *AdminResource) { resource.Fields[1].Sortable = true }, wantErr: "encrypted fields cannot be sorted"},
		{name: "default sort", mutate: func(resource *AdminResource) { resource.DefaultSortKey = "governmentReference" }, wantErr: "encrypted fields cannot be sorted"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource := validEncryptedAdminResource()
			tt.mutate(&resource)
			err := ValidateAdminSurface([]Manifest{{ID: "custom", Admin: AdminSurface{Resources: []AdminResource{resource}}}})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateAdminSurface() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func validEncryptedAdminResource() AdminResource {
	resource := validAdminResource("custom-records", "/custom-records", "admin:custom-record")
	resource.Protection = &AdminResourceProtection{SchemaVersion: 1, Scope: "tenant-field", TenantField: "tenantCode"}
	resource.Fields = []AdminField{
		{Key: "tenantCode", Label: Text("租户", "Tenant"), Type: "text", Source: "values", Required: true},
		{
			Key: "governmentReference", Label: Text("政府引用", "Government Reference"), Type: "text", Source: "values",
			Sensitivity: FieldSensitivitySensitive, StorageMode: FieldStorageEncrypted,
			ResponseMode: FieldProjectionPrivileged, ExportMode: FieldProjectionOmitted, Filterable: true,
			Protection: &AdminFieldProtection{Format: "aes-256-gcm-v1", Normalization: "trim-v1", BlindIndexNamespace: "custom-government-reference"},
		},
	}
	return resource
}

func TestValidateAdminSurfaceRejectsDuplicateFieldKeys(t *testing.T) {
	resource := validAdminResource("demo-resources", "/demo-resources", "admin:demo")
	resource.Fields = []AdminField{
		{Key: "title", Label: Text("标题", "Title"), Type: "text", Source: "values"},
		{Key: "title", Label: Text("再次标题", "Title Again"), Type: "text", Source: "values"},
	}

	err := ValidateAdminSurface([]Manifest{{ID: "demo", Admin: AdminSurface{Resources: []AdminResource{resource}}}})
	if err == nil {
		t.Fatalf("ValidateAdminSurface() error = nil, want duplicate field key")
	}
	if !strings.Contains(err.Error(), "duplicate field key") {
		t.Fatalf("ValidateAdminSurface() error = %v, want duplicate field key", err)
	}
}

func TestValidateAdminSurfaceRejectsInvalidFormGroupMetadata(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*AdminResource)
		wantErr string
	}{
		{
			name: "missing key",
			mutate: func(resource *AdminResource) {
				resource.FormGroups = []AdminFormGroup{{Label: Text("基础信息", "Basic")}}
			},
			wantErr: "form group key is required",
		},
		{
			name: "missing localized label",
			mutate: func(resource *AdminResource) {
				resource.FormGroups = []AdminFormGroup{{Key: "basic", Label: Text("基础信息", "")}}
			},
			wantErr: "form group basic label is required",
		},
		{
			name: "missing localized description",
			mutate: func(resource *AdminResource) {
				resource.FormGroups = []AdminFormGroup{{Key: "basic", Label: Text("基础信息", "Basic"), Description: Text("说明", "")}}
			},
			wantErr: "form group basic description must declare zh/en text",
		},
		{
			name: "duplicate key",
			mutate: func(resource *AdminResource) {
				resource.FormGroups = []AdminFormGroup{
					{Key: "basic", Label: Text("基础信息", "Basic")},
					{Key: "basic", Label: Text("再次基础", "Basic Again")},
				}
			},
			wantErr: "duplicate form group key",
		},
		{
			name: "field references unknown group",
			mutate: func(resource *AdminResource) {
				resource.FormGroups = []AdminFormGroup{{Key: "basic", Label: Text("基础信息", "Basic")}}
				resource.Fields = []AdminField{{Key: "title", Label: Text("标题", "Title"), Type: "text", Source: "values", Group: "advanced"}}
			},
			wantErr: "field title references unknown form group advanced",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource := validAdminResource("demo-resources", "/demo-resources", "admin:demo")
			tt.mutate(&resource)

			err := ValidateAdminSurface([]Manifest{{ID: "demo", Admin: AdminSurface{Resources: []AdminResource{resource}}}})
			if err == nil {
				t.Fatalf("ValidateAdminSurface() error = nil, want invalid form group")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateAdminSurface() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateAdminSurfaceRejectsInvalidFieldOptionAndHelpMetadata(t *testing.T) {
	tests := []struct {
		name    string
		field   AdminField
		wantErr string
	}{
		{
			name: "missing option value",
			field: AdminField{
				Key:     "category",
				Label:   Text("分类", "Category"),
				Type:    "select",
				Source:  "values",
				Options: []AdminFieldOption{{Label: Text("系统", "System")}},
			},
			wantErr: "field category option value is required",
		},
		{
			name: "missing option label",
			field: AdminField{
				Key:     "category",
				Label:   Text("分类", "Category"),
				Type:    "select",
				Source:  "values",
				Options: []AdminFieldOption{{Value: "system", Label: Text("系统", "")}},
			},
			wantErr: "field category option system label is required",
		},
		{
			name: "missing help localization",
			field: AdminField{
				Key:    "title",
				Label:  Text("标题", "Title"),
				Type:   "text",
				Source: "values",
				Help:   Text("标题说明", ""),
			},
			wantErr: "field title help must declare zh/en text",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource := validAdminResource("demo-resources", "/demo-resources", "admin:demo")
			resource.Fields = []AdminField{tt.field}

			err := ValidateAdminSurface([]Manifest{{ID: "demo", Admin: AdminSurface{Resources: []AdminResource{resource}}}})
			if err == nil {
				t.Fatalf("ValidateAdminSurface() error = nil, want invalid field metadata")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateAdminSurface() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateAdminSurfaceRejectsSearchAndSortUnknownFields(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*AdminResource)
		wantErr string
	}{
		{
			name:    "search field",
			mutate:  func(resource *AdminResource) { resource.SearchFields = []string{"missing"} },
			wantErr: "search field missing is not declared",
		},
		{
			name:    "default sort key",
			mutate:  func(resource *AdminResource) { resource.DefaultSortKey = "missing" },
			wantErr: "default sort key missing is not declared",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource := validAdminResource("demo-resources", "/demo-resources", "admin:demo")
			resource.Fields = []AdminField{{Key: "title", Label: Text("标题", "Title"), Type: "text", Source: "values"}}
			tt.mutate(&resource)

			err := ValidateAdminSurface([]Manifest{{ID: "demo", Admin: AdminSurface{Resources: []AdminResource{resource}}}})
			if err == nil {
				t.Fatalf("ValidateAdminSurface() error = nil, want invalid field reference")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateAdminSurface() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateAdminSurfaceRejectsDuplicateResourceKeys(t *testing.T) {
	manifests := []Manifest{
		{ID: "a", Admin: AdminSurface{Resources: []AdminResource{validAdminResource("shared", "/a", "admin:a")}}},
		{ID: "b", Admin: AdminSurface{Resources: []AdminResource{validAdminResource("shared", "/b", "admin:b")}}},
	}

	err := ValidateAdminSurface(manifests)
	if err == nil {
		t.Fatalf("ValidateAdminSurface() error = nil, want duplicate resource")
	}
	if !strings.Contains(err.Error(), `admin resource "shared" already registered`) {
		t.Fatalf("ValidateAdminSurface() error = %v, want duplicate resource", err)
	}
}

func TestValidateAdminSurfaceRejectsDuplicateMenuRoutes(t *testing.T) {
	manifests := []Manifest{
		{ID: "a", Admin: AdminSurface{Resources: []AdminResource{validAdminResource("a", "/shared", "admin:a")}}},
		{ID: "b", Admin: AdminSurface{Resources: []AdminResource{validAdminResource("b", "/shared", "admin:b")}}},
	}

	err := ValidateAdminSurface(manifests)
	if err == nil {
		t.Fatalf("ValidateAdminSurface() error = nil, want duplicate route")
	}
	if !strings.Contains(err.Error(), `admin menu route "/shared" already registered`) {
		t.Fatalf("ValidateAdminSurface() error = %v, want duplicate route", err)
	}
}

func TestValidateAdminSurfaceRejectsDuplicatePermissionPrefixes(t *testing.T) {
	manifests := []Manifest{
		{ID: "a", Admin: AdminSurface{Resources: []AdminResource{validAdminResource("a", "/a", "admin:shared")}}},
		{ID: "b", Admin: AdminSurface{Resources: []AdminResource{validAdminResource("b", "/b", "admin:shared")}}},
	}

	err := ValidateAdminSurface(manifests)
	if err == nil {
		t.Fatalf("ValidateAdminSurface() error = nil, want duplicate permission prefix")
	}
	if !strings.Contains(err.Error(), `admin permission prefix "admin:shared" already registered`) {
		t.Fatalf("ValidateAdminSurface() error = %v, want duplicate permission prefix", err)
	}
}

func TestValidateAdminSurfaceRejectsDuplicateResourceActionKeys(t *testing.T) {
	resource := validAdminResource("demo-resources", "/demo-resources", "admin:demo")
	resource.Actions = []AdminResourceAction{
		{
			Key:        "approve",
			Label:      Text("通过", "Approve"),
			Kind:       "row",
			Permission: "admin:demo:update",
			Method:     "POST",
			Route:      "/api/admin/demo-resources/:id/approve",
		},
		{
			Key:        "approve",
			Label:      Text("再次通过", "Approve Again"),
			Kind:       "row",
			Permission: "admin:demo:update",
			Method:     "POST",
			Route:      "/api/admin/demo-resources/:id/approve-again",
		},
	}

	err := ValidateAdminSurface([]Manifest{{ID: "demo", Admin: AdminSurface{Resources: []AdminResource{resource}}}})
	if err == nil {
		t.Fatalf("ValidateAdminSurface() error = nil, want duplicate action key")
	}
	if !strings.Contains(err.Error(), "duplicate action key") {
		t.Fatalf("ValidateAdminSurface() error = %v, want duplicate action key", err)
	}
}

func TestValidateAdminSurfaceRejectsDangerActionWithoutConfirm(t *testing.T) {
	resource := validAdminResource("demo-resources", "/demo-resources", "admin:demo")
	resource.Actions = []AdminResourceAction{
		{
			Key:        "close",
			Label:      Text("关闭", "Close"),
			Kind:       "row",
			Tone:       "danger",
			Permission: "admin:demo:update",
			Method:     "POST",
			Route:      "/api/admin/demo-resources/:id/close",
		},
	}

	err := ValidateAdminSurface([]Manifest{{ID: "demo", Admin: AdminSurface{Resources: []AdminResource{resource}}}})
	if err == nil {
		t.Fatalf("ValidateAdminSurface() error = nil, want confirmation error")
	}
	if !strings.Contains(err.Error(), "danger action requires confirmation") {
		t.Fatalf("ValidateAdminSurface() error = %v, want confirmation error", err)
	}
}

func TestValidateAdminSurfaceAcceptsResourcePanels(t *testing.T) {
	resource := validAdminResource("demo-resources", "/demo-resources", "admin:demo")
	resource.Panels = []AdminResourcePanel{
		{
			Key:        "audit",
			Label:      Text("审计", "Audit"),
			Kind:       "audit",
			Permission: "admin:demo:read",
			Component:  "audit-timeline",
			Order:      30,
		},
	}

	if err := ValidateAdminSurface([]Manifest{{ID: "demo", Admin: AdminSurface{Resources: []AdminResource{resource}}}}); err != nil {
		t.Fatalf("ValidateAdminSurface() error = %v", err)
	}
}

func TestValidateAdminSurfaceAcceptsRuntimeSlots(t *testing.T) {
	resource := validAdminResource("demo-resources", "/demo-resources", "admin:demo")
	resource.Fields = []AdminField{
		{Key: "title", Label: Text("标题", "Title"), Type: "text", Source: "values", InTable: true, InForm: true, InDetail: true},
	}
	resource.RuntimeSlots = []AdminRuntimeSlot{
		{
			SlotID:      "platform.record-summary",
			Region:      "side.preview",
			Label:       Text("记录摘要", "Record Summary"),
			Description: Text("展示当前记录关键字段。", "Shows key fields for the current record."),
			Permission:  "admin:demo:read",
			DataBinding: AdminRuntimeSlotDataBinding{Mode: "record", Fields: []string{"title", "status"}},
			Variant:     "preview",
			Order:       10,
		},
	}

	if err := ValidateAdminSurface([]Manifest{{ID: "demo", Admin: AdminSurface{Resources: []AdminResource{resource}}}}); err != nil {
		t.Fatalf("ValidateAdminSurface() error = %v", err)
	}
}

func TestValidateAdminSurfaceRejectsInvalidRuntimeSlots(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*AdminRuntimeSlot)
		wantErr string
	}{
		{
			name:    "missing slot id",
			mutate:  func(slot *AdminRuntimeSlot) { slot.SlotID = "" },
			wantErr: "runtime slot id is required",
		},
		{
			name:    "invalid region",
			mutate:  func(slot *AdminRuntimeSlot) { slot.Region = "unsafe.region" },
			wantErr: "runtime slot platform.record-summary region must be one of",
		},
		{
			name:    "missing localized label",
			mutate:  func(slot *AdminRuntimeSlot) { slot.Label.EN = "" },
			wantErr: "runtime slot platform.record-summary label is required",
		},
		{
			name:    "missing permission",
			mutate:  func(slot *AdminRuntimeSlot) { slot.Permission = "" },
			wantErr: "runtime slot platform.record-summary permission is required",
		},
		{
			name:    "unknown bound field",
			mutate:  func(slot *AdminRuntimeSlot) { slot.DataBinding.Fields = []string{"missing"} },
			wantErr: "runtime slot platform.record-summary data binding field \"missing\" is not declared",
		},
		{
			name:    "field control without target field",
			mutate:  func(slot *AdminRuntimeSlot) { slot.Region = "field.control" },
			wantErr: "runtime slot platform.record-summary targetField is required for field.control",
		},
		{
			name:    "section slot without target section",
			mutate:  func(slot *AdminRuntimeSlot) { slot.Region = "form.section.before" },
			wantErr: "runtime slot platform.record-summary targetSection is required for section slots",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource := validAdminResource("demo-resources", "/demo-resources", "admin:demo")
			resource.Fields = []AdminField{
				{Key: "title", Label: Text("标题", "Title"), Type: "text", Source: "values", InTable: true, InForm: true, InDetail: true},
			}
			slot := AdminRuntimeSlot{
				SlotID:      "platform.record-summary",
				Region:      "side.preview",
				Label:       Text("记录摘要", "Record Summary"),
				Description: Text("展示当前记录关键字段。", "Shows key fields for the current record."),
				Permission:  "admin:demo:read",
				DataBinding: AdminRuntimeSlotDataBinding{Mode: "record", Fields: []string{"title"}},
				Variant:     "preview",
				Order:       10,
			}
			tt.mutate(&slot)
			resource.RuntimeSlots = []AdminRuntimeSlot{slot}

			err := ValidateAdminSurface([]Manifest{{ID: "demo", Admin: AdminSurface{Resources: []AdminResource{resource}}}})
			if err == nil {
				t.Fatalf("ValidateAdminSurface() error = nil, want %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateAdminSurface() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestResolveEnabledValidatesAdminSurface(t *testing.T) {
	registry := NewRegistry()
	a := testManifest("a")
	a.Admin = AdminSurface{Resources: []AdminResource{validAdminResource("a", "/shared", "admin:a")}}
	mustRegister(t, registry, a)
	b := testManifest("b")
	b.Admin = AdminSurface{Resources: []AdminResource{validAdminResource("b", "/shared", "admin:b")}}
	mustRegister(t, registry, b)

	_, err := registry.ResolveEnabled([]ID{"a", "b"})
	if err == nil {
		t.Fatalf("ResolveEnabled() error = nil, want duplicate route")
	}
	if !strings.Contains(err.Error(), `admin menu route "/shared" already registered`) {
		t.Fatalf("ResolveEnabled() error = %v, want duplicate route", err)
	}
}

func TestResolveEnabledRejectsAdminRelationToDisabledResource(t *testing.T) {
	users := validAdminResource("users", "/users", "admin:user")
	users.Fields = []AdminField{
		{
			Key:      "tenantCode",
			Label:    Text("租户", "Tenant"),
			Type:     "select",
			Source:   "values",
			InForm:   true,
			Relation: &AdminFieldRelation{Resource: "tenants", ValueField: "code", LabelField: "name"},
		},
	}

	registry := NewRegistry()
	tenant := testManifest("tenant")
	tenant.Admin = AdminSurface{Resources: []AdminResource{validAdminResource("tenants", "/tenants", "admin:tenant")}}
	mustRegister(t, registry, tenant)
	identity := testManifest("identity")
	identity.Admin = AdminSurface{Resources: []AdminResource{users}}
	mustRegister(t, registry, identity)

	_, err := registry.ResolveEnabled([]ID{"identity"})
	if err == nil {
		t.Fatalf("ResolveEnabled() error = nil, want missing relation target")
	}
	if !strings.Contains(err.Error(), `relation target resource "tenants" is not enabled`) {
		t.Fatalf("ResolveEnabled() error = %v, want missing relation target", err)
	}
}

func TestValidateAdminSurfaceAcceptsRelationSelectWithoutStaticOptions(t *testing.T) {
	tenants := validAdminResource("tenants", "/tenants", "admin:tenant")
	orgUnits := validAdminResource("org-units", "/org-units", "admin:org-unit")
	orgUnits.Fields = []AdminField{
		{
			Key:      "parentCode",
			Label:    Text("上级机构", "Parent"),
			Type:     "text",
			Source:   "values",
			InForm:   true,
			InDetail: true,
		},
	}
	roles := validAdminResource("roles", "/roles", "admin:role")
	resource := validAdminResource("users", "/users", "admin:user")
	resource.Fields = []AdminField{
		{
			Key:      "tenantCode",
			Label:    Text("租户", "Tenant"),
			Type:     "select",
			Source:   "values",
			InForm:   true,
			Relation: &AdminFieldRelation{Resource: "tenants", ValueField: "code", LabelField: "name", SortField: "name", SortOrder: "asc", Display: "select"},
		},
		{
			Key:      "orgUnitCode",
			Label:    Text("机构", "Org Unit"),
			Type:     "select",
			Source:   "values",
			InForm:   true,
			Relation: &AdminFieldRelation{Resource: "org-units", ValueField: "code", LabelField: "name", SortField: "name", SortOrder: "asc", Display: "tree", ParentField: "parentCode"},
		},
		{
			Key:      "roles",
			Label:    Text("角色", "Roles"),
			Type:     "multiselect",
			Source:   "values",
			InForm:   true,
			Relation: &AdminFieldRelation{Resource: "roles", ValueField: "code", LabelField: "name", Multiple: true},
		},
	}

	if err := ValidateAdminSurface([]Manifest{
		{ID: "tenant", Admin: AdminSurface{Resources: []AdminResource{tenants}}},
		{ID: "identity", Admin: AdminSurface{Resources: []AdminResource{orgUnits, resource}}},
		{ID: "rbac", Admin: AdminSurface{Resources: []AdminResource{roles}}},
	}); err != nil {
		t.Fatalf("ValidateAdminSurface() error = %v", err)
	}
}

func TestValidateAdminSurfaceRejectsRelationTargetMissingDeclaredField(t *testing.T) {
	tenants := validAdminResource("tenants", "/tenants", "admin:tenant")
	users := validAdminResource("users", "/users", "admin:user")
	users.Fields = []AdminField{
		{
			Key:      "tenantCode",
			Label:    Text("租户", "Tenant"),
			Type:     "select",
			Source:   "values",
			InForm:   true,
			Relation: &AdminFieldRelation{Resource: "tenants", ValueField: "code", LabelField: "displayName"},
		},
	}

	err := ValidateAdminSurface([]Manifest{
		{ID: "tenant", Admin: AdminSurface{Resources: []AdminResource{tenants}}},
		{ID: "identity", Admin: AdminSurface{Resources: []AdminResource{users}}},
	})
	if err == nil {
		t.Fatalf("ValidateAdminSurface() error = nil, want missing relation target field")
	}
	if !strings.Contains(err.Error(), `relation label field "displayName" is not declared by target resource "tenants"`) {
		t.Fatalf("ValidateAdminSurface() error = %v, want missing relation label field", err)
	}
}

func TestValidateAdminSurfaceRejectsRelationTargetMissingMetadataFields(t *testing.T) {
	for _, tt := range []struct {
		name     string
		relation AdminFieldRelation
		want     string
	}{
		{
			name:     "sort field",
			relation: AdminFieldRelation{Resource: "tenants", ValueField: "code", LabelField: "name", SortField: "displayOrder"},
			want:     `relation sort field "displayOrder" is not declared by target resource "tenants"`,
		},
		{
			name:     "filter field",
			relation: AdminFieldRelation{Resource: "tenants", ValueField: "code", LabelField: "name", Filters: []AdminFieldRelationFilter{{Field: "visibility", Operator: "="}}},
			want:     `relation filter field "visibility" is not declared by target resource "tenants"`,
		},
		{
			name:     "parent field",
			relation: AdminFieldRelation{Resource: "tenants", ValueField: "code", LabelField: "name", Display: "tree", ParentField: "parentCode"},
			want:     `relation parent field "parentCode" is not declared by target resource "tenants"`,
		},
		{
			name:     "path field",
			relation: AdminFieldRelation{Resource: "tenants", ValueField: "code", LabelField: "name", PathField: "path"},
			want:     `relation path field "path" is not declared by target resource "tenants"`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tenants := validAdminResource("tenants", "/tenants", "admin:tenant")
			users := validAdminResource("users", "/users", "admin:user")
			users.Fields = []AdminField{
				{
					Key:      "tenantCode",
					Label:    Text("租户", "Tenant"),
					Type:     "select",
					Source:   "values",
					InForm:   true,
					Relation: &tt.relation,
				},
			}

			err := ValidateAdminSurface([]Manifest{
				{ID: "tenant", Admin: AdminSurface{Resources: []AdminResource{tenants}}},
				{ID: "identity", Admin: AdminSurface{Resources: []AdminResource{users}}},
			})
			if err == nil {
				t.Fatalf("ValidateAdminSurface() error = nil, want missing relation target field")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ValidateAdminSurface() error = %v, want %s", err, tt.want)
			}
		})
	}
}

func TestValidateAdminSurfaceRejectsUnsupportedRelationFilterOperator(t *testing.T) {
	tenants := validAdminResource("tenants", "/tenants", "admin:tenant")
	users := validAdminResource("users", "/users", "admin:user")
	users.Fields = []AdminField{
		{
			Key:    "tenantCode",
			Label:  Text("租户", "Tenant"),
			Type:   "select",
			Source: "values",
			Relation: &AdminFieldRelation{
				Resource:   "tenants",
				ValueField: "code",
				LabelField: "name",
				Filters: []AdminFieldRelationFilter{
					{Field: "status", Operator: "LIKE", Value: "enabled"},
				},
			},
		},
	}

	err := ValidateAdminSurface([]Manifest{
		{ID: "tenant", Admin: AdminSurface{Resources: []AdminResource{tenants}}},
		{ID: "identity", Admin: AdminSurface{Resources: []AdminResource{users}}},
	})
	if err == nil {
		t.Fatalf("ValidateAdminSurface() error = nil, want invalid relation filter operator")
	}
	if !strings.Contains(err.Error(), "relation filter operator must be one of") {
		t.Fatalf("ValidateAdminSurface() error = %v, want invalid relation filter operator", err)
	}
}

func TestValidateAdminSurfaceRejectsInvalidRelationField(t *testing.T) {
	resource := validAdminResource("users", "/users", "admin:user")
	resource.Fields = []AdminField{
		{
			Key:      "roles",
			Label:    Text("角色", "Roles"),
			Type:     "select",
			Source:   "values",
			Relation: &AdminFieldRelation{Resource: "roles", ValueField: "code", LabelField: "name", Multiple: true},
		},
	}

	err := ValidateAdminSurface([]Manifest{{ID: "identity", Admin: AdminSurface{Resources: []AdminResource{resource}}}})
	if err == nil {
		t.Fatalf("ValidateAdminSurface() error = nil, want invalid relation")
	}
	if !strings.Contains(err.Error(), "multiple relation requires multiselect type") {
		t.Fatalf("ValidateAdminSurface() error = %v, want multiple relation error", err)
	}
}

func TestValidateAdminSurfaceRejectsTreeRelationWithoutParentField(t *testing.T) {
	resource := validAdminResource("users", "/users", "admin:user")
	resource.Fields = []AdminField{
		{
			Key:      "orgUnitCode",
			Label:    Text("机构", "Org Unit"),
			Type:     "select",
			Source:   "values",
			Relation: &AdminFieldRelation{Resource: "org-units", ValueField: "code", LabelField: "name", Display: "tree"},
		},
	}

	err := ValidateAdminSurface([]Manifest{{ID: "identity", Admin: AdminSurface{Resources: []AdminResource{resource}}}})
	if err == nil {
		t.Fatalf("ValidateAdminSurface() error = nil, want invalid tree relation")
	}
	if !strings.Contains(err.Error(), "tree relation parent field is required") {
		t.Fatalf("ValidateAdminSurface() error = %v, want tree parent field error", err)
	}
}

func validAdminResource(resource string, route string, permissionPrefix string) AdminResource {
	return AdminResource{
		Resource:         resource,
		Title:            Text("标题", "Title"),
		Description:      Text("描述", "Description"),
		PermissionPrefix: permissionPrefix,
		Menu:             AdminMenu{Route: route, Group: "foundation", Icon: "overview", Order: 10},
	}
}
