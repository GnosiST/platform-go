package adminresource

import (
	"slices"
	"strings"

	"github.com/GnosiST/platform-go/internal/platform/capability"
)

type LocalizedText struct {
	ZH string `json:"zh"`
	EN string `json:"en"`
}

type FieldOption struct {
	Value string        `json:"value"`
	Label LocalizedText `json:"label"`
}

type FormGroupDefinition struct {
	Key         string        `json:"key"`
	Label       LocalizedText `json:"label"`
	Description LocalizedText `json:"description,omitempty"`
}

type FieldValidation struct {
	MinLength int      `json:"minLength,omitempty"`
	MaxLength int      `json:"maxLength,omitempty"`
	Min       *float64 `json:"min,omitempty"`
	Max       *float64 `json:"max,omitempty"`
	Pattern   string   `json:"pattern,omitempty"`
}

type FieldRelationFilter struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

type FieldRelation struct {
	Resource    string                `json:"resource"`
	ValueField  string                `json:"valueField"`
	LabelField  string                `json:"labelField"`
	Multiple    bool                  `json:"multiple,omitempty"`
	Filters     []FieldRelationFilter `json:"filters,omitempty"`
	SortField   string                `json:"sortField,omitempty"`
	SortOrder   string                `json:"sortOrder,omitempty"`
	Display     string                `json:"display,omitempty"`
	ParentField string                `json:"parentField,omitempty"`
	PathField   string                `json:"pathField,omitempty"`
	RootValue   string                `json:"rootValue,omitempty"`
}

type FieldDefinition struct {
	Key          string           `json:"key"`
	Label        LocalizedText    `json:"label"`
	Type         string           `json:"type"`
	Source       string           `json:"source"`
	Group        string           `json:"group,omitempty"`
	Help         *LocalizedText   `json:"help,omitempty"`
	Required     bool             `json:"required,omitempty"`
	ReadOnly     bool             `json:"readOnly,omitempty"`
	Searchable   bool             `json:"searchable,omitempty"`
	Filterable   bool             `json:"filterable,omitempty"`
	Sortable     bool             `json:"sortable,omitempty"`
	Localizable  bool             `json:"localizable,omitempty"`
	InTable      bool             `json:"inTable,omitempty"`
	InForm       bool             `json:"inForm,omitempty"`
	InDetail     bool             `json:"inDetail,omitempty"`
	Width        int              `json:"width,omitempty"`
	Options      []FieldOption    `json:"options,omitempty"`
	Relation     *FieldRelation   `json:"relation,omitempty"`
	Validation   *FieldValidation `json:"validation,omitempty"`
	Sensitivity  string           `json:"sensitivity"`
	StorageMode  string           `json:"storageMode"`
	ResponseMode string           `json:"responseMode"`
	ExportMode   string           `json:"exportMode"`
	Protection   *FieldProtection `json:"protection,omitempty"`
	Masking      *FieldMasking    `json:"masking,omitempty"`
	Reveal       *FieldReveal     `json:"reveal,omitempty"`
}

type FieldProtection struct {
	Format              string `json:"format"`
	Normalization       string `json:"normalization"`
	BlindIndexNamespace string `json:"blindIndexNamespace,omitempty"`
}

type FieldMasking struct {
	Strategy       string `json:"strategy"`
	PreservePrefix int    `json:"preservePrefix,omitempty"`
	PreserveSuffix int    `json:"preserveSuffix,omitempty"`
	MaskLength     int    `json:"maskLength,omitempty"`
	Replacement    string `json:"replacement,omitempty"`
}

type FieldReveal struct {
	PolicyID    string `json:"policyId"`
	Permission  string `json:"permission"`
	CopyAllowed bool   `json:"copyAllowed,omitempty"`
}

type ResourceProtection struct {
	SchemaVersion uint32 `json:"schemaVersion"`
	Scope         string `json:"scope"`
	TenantField   string `json:"tenantField,omitempty"`
}

type ResourceDeletionPolicy struct {
	Mode               string `json:"mode"`
	PolicyVersion      uint32 `json:"policyVersion"`
	RetentionDays      int    `json:"retentionDays,omitempty"`
	AutoPurge          bool   `json:"autoPurge,omitempty"`
	RestrictReferences bool   `json:"restrictReferences,omitempty"`
}

type ActionPermissions struct {
	Read    string `json:"read"`
	Create  string `json:"create"`
	Update  string `json:"update"`
	Delete  string `json:"delete"`
	Restore string `json:"restore,omitempty"`
	Purge   string `json:"purge,omitempty"`
}

type ResourceActionConfirm struct {
	Title       LocalizedText `json:"title"`
	Description LocalizedText `json:"description,omitempty"`
	OkText      LocalizedText `json:"okText,omitempty"`
}

type ResourceActionDefinition struct {
	Key         string                 `json:"key"`
	Label       LocalizedText          `json:"label"`
	Kind        string                 `json:"kind"`
	Tone        string                 `json:"tone,omitempty"`
	Icon        string                 `json:"icon,omitempty"`
	Permission  string                 `json:"permission"`
	Route       string                 `json:"route,omitempty"`
	Method      string                 `json:"method,omitempty"`
	Confirm     *ResourceActionConfirm `json:"confirm,omitempty"`
	AuditAction string                 `json:"auditAction,omitempty"`
	Refresh     bool                   `json:"refresh,omitempty"`
}

type ResourcePanelDefinition struct {
	Key        string        `json:"key"`
	Label      LocalizedText `json:"label"`
	Kind       string        `json:"kind"`
	Permission string        `json:"permission,omitempty"`
	Component  string        `json:"component,omitempty"`
	Order      int           `json:"order,omitempty"`
	Empty      LocalizedText `json:"empty,omitempty"`
}

type RuntimeSlotDataBinding struct {
	Mode   string   `json:"mode,omitempty"`
	Fields []string `json:"fields,omitempty"`
}

type RuntimeSlotDefinition struct {
	SlotID        string                 `json:"slotId"`
	Region        string                 `json:"region"`
	Label         LocalizedText          `json:"label"`
	Description   LocalizedText          `json:"description"`
	Permission    string                 `json:"permission,omitempty"`
	VisibleWhen   string                 `json:"visibleWhen,omitempty"`
	TargetSection string                 `json:"targetSection,omitempty"`
	TargetField   string                 `json:"targetField,omitempty"`
	DataBinding   RuntimeSlotDataBinding `json:"dataBinding,omitempty"`
	Variant       string                 `json:"variant,omitempty"`
	Order         int                    `json:"order,omitempty"`
}

type Schema struct {
	Resource       string                     `json:"resource"`
	Title          LocalizedText              `json:"title"`
	Description    LocalizedText              `json:"description"`
	Permissions    ActionPermissions          `json:"permissions"`
	FormGroups     []FormGroupDefinition      `json:"formGroups,omitempty"`
	FormLayout     string                     `json:"formLayout,omitempty"`
	Fields         []FieldDefinition          `json:"fields"`
	Actions        []ResourceActionDefinition `json:"actions,omitempty"`
	Panels         []ResourcePanelDefinition  `json:"panels,omitempty"`
	RuntimeSlots   []RuntimeSlotDefinition    `json:"runtimeSlots,omitempty"`
	SearchFields   []string                   `json:"searchFields"`
	DefaultSortKey string                     `json:"defaultSortKey,omitempty"`
	Protection     *ResourceProtection        `json:"protection,omitempty"`
	Deletion       *ResourceDeletionPolicy    `json:"deletion"`
}

func (s *Store) Schema(resource string) (Schema, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	schema, ok := s.schemas[resource]
	if !ok {
		return Schema{}, ErrUnknownResource
	}
	return cloneSchema(schema), nil
}

// EnableOrganizationRBACRoleGovernanceWrites opens only the ownership fields
// required to create strict role groups in target organization-RBAC mode.
// Domain snapshot writers remain authoritative for immutability and validation.
func (s *Store) EnableOrganizationRBACRoleGovernanceWrites() {
	s.mu.Lock()
	defer s.mu.Unlock()
	schema, ok := s.schemas["role-groups"]
	if !ok {
		return
	}
	for index := range schema.Fields {
		switch schema.Fields[index].Key {
		case "scopeType":
			schema.Fields[index].Required = true
			schema.Fields[index].ReadOnly = false
			schema.Fields[index].InForm = true
		case "tenantCode":
			schema.Fields[index].ReadOnly = false
			schema.Fields[index].InForm = true
		case "parentCode":
			schema.Fields[index].ReadOnly = true
			schema.Fields[index].InForm = false
		}
	}
	s.schemas["role-groups"] = schema
	roleSchema, ok := s.schemas["roles"]
	if !ok {
		return
	}
	for index := range roleSchema.Fields {
		switch roleSchema.Fields[index].Key {
		case "permissions", "denyPermissions", "dataScope", "dataScopeOrgCodes", "dataScopeAreaCodes":
			roleSchema.Fields[index].Required = false
			roleSchema.Fields[index].ReadOnly = true
			roleSchema.Fields[index].InForm = false
		}
	}
	s.schemas["roles"] = roleSchema
}

// EnableOrganizationRBACMenuGovernanceWrites exposes target menu authoring
// while keeping legacy routing and permission fields readable for migration.
func (s *Store) EnableOrganizationRBACMenuGovernanceWrites() {
	s.mu.Lock()
	defer s.mu.Unlock()
	schema, ok := s.schemas["menus"]
	if !ok {
		return
	}
	for index := range schema.Fields {
		switch schema.Fields[index].Key {
		case "nodeType":
			schema.Fields[index].Required = true
		case "parent", "permission":
			schema.Fields[index].Required = false
			schema.Fields[index].ReadOnly = true
			schema.Fields[index].InForm = false
		}
	}
	s.schemas["menus"] = schema
}

func cloneSchema(schema Schema) Schema {
	schema.FormGroups = append([]FormGroupDefinition(nil), schema.FormGroups...)
	schema.Fields = append([]FieldDefinition(nil), schema.Fields...)
	schema.Actions = append([]ResourceActionDefinition(nil), schema.Actions...)
	schema.Panels = append([]ResourcePanelDefinition(nil), schema.Panels...)
	schema.RuntimeSlots = append([]RuntimeSlotDefinition(nil), schema.RuntimeSlots...)
	schema.SearchFields = append([]string(nil), schema.SearchFields...)
	if schema.Protection != nil {
		protection := *schema.Protection
		schema.Protection = &protection
	}
	if schema.Deletion != nil {
		deletion := *schema.Deletion
		schema.Deletion = &deletion
	}
	for index := range schema.Actions {
		if schema.Actions[index].Confirm != nil {
			confirm := *schema.Actions[index].Confirm
			schema.Actions[index].Confirm = &confirm
		}
	}
	for index := range schema.Fields {
		schema.Fields[index].Options = append([]FieldOption(nil), schema.Fields[index].Options...)
		if schema.Fields[index].Help != nil {
			help := *schema.Fields[index].Help
			schema.Fields[index].Help = &help
		}
		if schema.Fields[index].Validation != nil {
			validation := *schema.Fields[index].Validation
			schema.Fields[index].Validation = &validation
		}
		if schema.Fields[index].Relation != nil {
			relation := *schema.Fields[index].Relation
			relation.Filters = append([]FieldRelationFilter(nil), schema.Fields[index].Relation.Filters...)
			schema.Fields[index].Relation = &relation
		}
		if schema.Fields[index].Protection != nil {
			protection := *schema.Fields[index].Protection
			schema.Fields[index].Protection = &protection
		}
		if schema.Fields[index].Masking != nil {
			masking := *schema.Fields[index].Masking
			schema.Fields[index].Masking = &masking
		}
		if schema.Fields[index].Reveal != nil {
			reveal := *schema.Fields[index].Reveal
			schema.Fields[index].Reveal = &reveal
		}
	}
	for index := range schema.RuntimeSlots {
		schema.RuntimeSlots[index].DataBinding.Fields = append([]string(nil), schema.RuntimeSlots[index].DataBinding.Fields...)
	}
	return schema
}

func seedResourceSchemas() map[string]Schema {
	permissionOptions := []FieldOption{
		option("*", "全部权限", "All Permissions"),
		option("admin:*", "后台全部权限", "All Admin Permissions"),
	}
	schemas := map[string]Schema{
		"overview":              overviewResourceSchema(),
		"tenants":               tenantResourceSchema(),
		"users":                 userResourceSchema(),
		"org-units":             orgUnitResourceSchema(),
		"roles":                 roleResourceSchema(permissionOptions),
		"role-groups":           roleGroupResourceSchema(),
		"menus":                 menuResourceSchema(),
		"permissions":           permissionResourceSchema(),
		"api-resources":         apiResourceSchema(),
		"dictionaries":          dictionaryResourceSchema(),
		"parameters":            parameterResourceSchema(),
		"dictionary-parameters": dictionaryParameterSchema(),
		"area-codes":            areaCodeResourceSchema(),
		"audit-logs":            auditLogResourceSchema(),
		"monitoring":            monitoringResourceSchema(),
		"branding":              brandingResourceSchema(),
		"settings":              settingsResourceSchema(),
	}
	for resource, schema := range schemas {
		schema.Deletion = seedDeletionPolicy(resource)
		schema.Permissions = permissionsForDeletion(schema.Permissions, strings.TrimSuffix(schema.Permissions.Read, ":read"), deletionPolicyToCapability(schema.Deletion))
		schemas[resource] = schema
	}
	return schemas
}

func seedResourceSchemasFromCapabilities(manifests []capability.Manifest) map[string]Schema {
	permissionOptions := permissionOptionsFromCapabilities(manifests)
	schemas := map[string]Schema{
		"users":       userResourceSchema(),
		"roles":       roleResourceSchema(permissionOptions),
		"menus":       menuResourceSchema(),
		"permissions": permissionResourceSchema(),
	}
	for resource, schema := range schemas {
		schema.Deletion = seedDeletionPolicy(resource)
		schema.Permissions = permissionsForDeletion(schema.Permissions, strings.TrimSuffix(schema.Permissions.Read, ":read"), deletionPolicyToCapability(schema.Deletion))
		schemas[resource] = schema
	}
	registered := false
	for _, manifest := range manifests {
		for _, resource := range manifest.Admin.Resources {
			if resource.Resource == "" {
				continue
			}
			registered = true
			if resource.Resource == "roles" {
				schemas[resource.Resource] = mergeCapabilityDeletion(mergeCapabilityProtection(roleResourceSchema(permissionOptions), resource), resource)
				continue
			}
			schemas[resource.Resource] = schemaFromCapabilityResource(resource)
		}
	}
	if !registered {
		return seedResourceSchemas()
	}
	return schemas
}

func seedDeletionPolicy(resource string) *ResourceDeletionPolicy {
	policy := &ResourceDeletionPolicy{PolicyVersion: 1}
	switch resource {
	case "overview", "branding", "settings":
		policy.Mode = capability.AdminDeletionDisabled
	case "audit-logs":
		policy.Mode = capability.AdminDeletionAppendOnly
	case "api-resources", "area-codes", "dictionaries", "permissions":
		policy.Mode = capability.AdminDeletionRestrict
		policy.RestrictReferences = true
	default:
		policy.Mode = capability.AdminDeletionSoftDelete
		policy.RestrictReferences = true
	}
	return policy
}

func deletionPolicyToCapability(policy *ResourceDeletionPolicy) *capability.AdminResourceDeletionPolicy {
	if policy == nil {
		return nil
	}
	return &capability.AdminResourceDeletionPolicy{
		Mode: policy.Mode, PolicyVersion: policy.PolicyVersion, RetentionDays: policy.RetentionDays,
		AutoPurge: policy.AutoPurge, RestrictReferences: policy.RestrictReferences,
	}
}

func schemaFromCapabilityResource(resource capability.AdminResource) Schema {
	var specialized Schema
	switch resource.Resource {
	case "tenants":
		specialized = tenantResourceSchema()
	case "users":
		specialized = userResourceSchema()
	case "org-units":
		specialized = orgUnitResourceSchema()
	case "role-groups":
		specialized = roleGroupResourceSchema()
	case "area-codes":
		specialized = areaCodeResourceSchema()
	case "api-resources":
		specialized = apiResourceSchema()
	case "dictionaries":
		specialized = dictionaryResourceSchema()
	case "parameters":
		specialized = parameterResourceSchema()
	case "dictionary-parameters":
		specialized = dictionaryParameterSchema()
	case "menus":
		specialized = menuResourceSchema()
	case "permissions":
		specialized = permissionResourceSchema()
	case "settings":
		specialized = settingsResourceSchema()
	case "audit-logs":
		specialized = auditLogResourceSchema()
	case "branding":
		specialized = brandingResourceSchema()
	case "monitoring":
		specialized = monitoringResourceSchema()
	case "overview":
		specialized = overviewResourceSchema()
	}
	if specialized.Resource != "" {
		return mergeCapabilityDeletion(mergeCapabilityProtection(specialized, resource), resource)
	}
	if len(resource.Fields) == 0 {
		schema := defaultSchema(
			resource.Resource,
			localizedTextFromCapability(resource.Title),
			localizedTextFromCapability(resource.Description),
			resource.PermissionPrefix,
		)
		schema.Actions = actionsFromCapability(resource.Actions)
		schema.Panels = panelsFromCapability(resource.Panels)
		schema.RuntimeSlots = runtimeSlotsFromCapability(resource.RuntimeSlots)
		schema.FormLayout = formLayoutFromCapability(resource.FormLayout, schema.Fields)
		schema.Protection = resourceProtectionFromCapability(resource.Protection)
		schema.Deletion = resourceDeletionFromCapability(resource.Deletion)
		schema.Permissions = permissionsForDeletion(schema.Permissions, resource.PermissionPrefix, resource.Deletion)
		return schema
	}
	fields := fieldsFromCapability(resource.Fields)
	schema := Schema{
		Resource:       resource.Resource,
		Title:          localizedTextFromCapability(resource.Title),
		Description:    localizedTextFromCapability(resource.Description),
		Permissions:    permissions(resource.PermissionPrefix),
		FormGroups:     formGroupsFromCapability(resource.FormGroups, fields),
		FormLayout:     formLayoutFromCapability(resource.FormLayout, fields),
		Fields:         fields,
		Actions:        actionsFromCapability(resource.Actions),
		Panels:         panelsFromCapability(resource.Panels),
		RuntimeSlots:   runtimeSlotsFromCapability(resource.RuntimeSlots),
		SearchFields:   append([]string(nil), resource.SearchFields...),
		DefaultSortKey: resource.DefaultSortKey,
		Protection:     resourceProtectionFromCapability(resource.Protection),
		Deletion:       resourceDeletionFromCapability(resource.Deletion),
	}
	schema.Permissions = permissionsForDeletion(schema.Permissions, resource.PermissionPrefix, resource.Deletion)
	return schema
}

func mergeCapabilityDeletion(schema Schema, resource capability.AdminResource) Schema {
	schema.Deletion = resourceDeletionFromCapability(resource.Deletion)
	schema.Permissions = permissionsForDeletion(schema.Permissions, resource.PermissionPrefix, resource.Deletion)
	return schema
}

func resourceDeletionFromCapability(policy *capability.AdminResourceDeletionPolicy) *ResourceDeletionPolicy {
	if policy == nil {
		return nil
	}
	return &ResourceDeletionPolicy{
		Mode: policy.Mode, PolicyVersion: policy.PolicyVersion, RetentionDays: policy.RetentionDays,
		AutoPurge: policy.AutoPurge, RestrictReferences: policy.RestrictReferences,
	}
}

func permissionsForDeletion(current ActionPermissions, prefix string, policy *capability.AdminResourceDeletionPolicy) ActionPermissions {
	current.Restore = ""
	current.Purge = ""
	if policy == nil {
		current.Delete = ""
		return current
	}
	switch policy.Mode {
	case capability.AdminDeletionDisabled, capability.AdminDeletionAppendOnly:
		current.Delete = ""
	case capability.AdminDeletionSoftDelete, capability.AdminDeletionTombstone:
		current.Delete = prefix + ":delete"
		current.Restore = prefix + ":restore"
	case capability.AdminDeletionRevoke:
		current.Delete = prefix + ":delete"
	default:
		current.Delete = prefix + ":delete"
	}
	return current
}

func mergeCapabilityProtection(schema Schema, resource capability.AdminResource) Schema {
	schema.Protection = resourceProtectionFromCapability(resource.Protection)
	declaredFields := fieldsFromCapability(resource.Fields)
	declaredByKey := make(map[string]FieldDefinition, len(declaredFields))
	for _, field := range declaredFields {
		declaredByKey[field.Key] = field
	}
	existing := make(map[string]struct{}, len(schema.Fields))
	for index := range schema.Fields {
		key := schema.Fields[index].Key
		existing[key] = struct{}{}
		declared, ok := declaredByKey[key]
		if !ok || declared.StorageMode != capability.FieldStorageEncrypted {
			continue
		}
		schema.Fields[index].Required = declared.Required
		schema.Fields[index].ReadOnly = declared.ReadOnly
		schema.Fields[index].Searchable = declared.Searchable
		schema.Fields[index].Filterable = declared.Filterable
		schema.Fields[index].Sortable = declared.Sortable
		schema.Fields[index].Sensitivity = declared.Sensitivity
		schema.Fields[index].StorageMode = declared.StorageMode
		schema.Fields[index].ResponseMode = declared.ResponseMode
		schema.Fields[index].ExportMode = declared.ExportMode
		schema.Fields[index].Protection = declared.Protection
		schema.Fields[index].Masking = declared.Masking
		schema.Fields[index].Reveal = declared.Reveal
	}
	for _, declared := range resource.Fields {
		if _, ok := existing[declared.Key]; ok {
			continue
		}
		field, ok := declaredByKey[declared.Key]
		if !ok {
			continue
		}
		schema.Fields = append(schema.Fields, field)
		existing[declared.Key] = struct{}{}
	}
	for _, key := range resource.SearchFields {
		if !slices.Contains(schema.SearchFields, key) {
			schema.SearchFields = append(schema.SearchFields, key)
		}
	}
	if resource.DefaultSortKey != "" {
		schema.DefaultSortKey = resource.DefaultSortKey
	}
	return schema
}

func resourceProtectionFromCapability(protection *capability.AdminResourceProtection) *ResourceProtection {
	if protection == nil {
		return nil
	}
	return &ResourceProtection{SchemaVersion: protection.SchemaVersion, Scope: protection.Scope, TenantField: protection.TenantField}
}

func fieldProtectionFromCapability(protection *capability.AdminFieldProtection) *FieldProtection {
	if protection == nil {
		return nil
	}
	return &FieldProtection{Format: protection.Format, Normalization: protection.Normalization, BlindIndexNamespace: protection.BlindIndexNamespace}
}

func fieldMaskingFromCapability(masking *capability.AdminFieldMasking) *FieldMasking {
	if masking == nil {
		return nil
	}
	return &FieldMasking{
		Strategy: masking.Strategy, PreservePrefix: masking.PreservePrefix, PreserveSuffix: masking.PreserveSuffix,
		MaskLength: masking.MaskLength, Replacement: masking.Replacement,
	}
}

func fieldRevealFromCapability(reveal *capability.AdminFieldReveal) *FieldReveal {
	if reveal == nil {
		return nil
	}
	return &FieldReveal{PolicyID: reveal.PolicyID, Permission: reveal.Permission, CopyAllowed: reveal.CopyAllowed}
}

func actionsFromCapability(actions []capability.AdminResourceAction) []ResourceActionDefinition {
	if len(actions) == 0 {
		return nil
	}
	result := make([]ResourceActionDefinition, 0, len(actions))
	for _, action := range actions {
		item := ResourceActionDefinition{
			Key:         action.Key,
			Label:       localizedTextFromCapability(action.Label),
			Kind:        defaultString(action.Kind, "row"),
			Tone:        defaultString(action.Tone, "default"),
			Icon:        action.Icon,
			Permission:  action.Permission,
			Route:       action.Route,
			Method:      action.Method,
			AuditAction: action.AuditAction,
			Refresh:     action.Refresh,
		}
		if action.Confirm != nil {
			item.Confirm = &ResourceActionConfirm{
				Title:       localizedTextFromCapability(action.Confirm.Title),
				Description: localizedTextFromCapability(action.Confirm.Description),
				OkText:      localizedTextFromCapability(action.Confirm.OkText),
			}
		}
		result = append(result, item)
	}
	return result
}

func panelsFromCapability(panels []capability.AdminResourcePanel) []ResourcePanelDefinition {
	if len(panels) == 0 {
		return nil
	}
	result := make([]ResourcePanelDefinition, 0, len(panels))
	for _, panel := range panels {
		result = append(result, ResourcePanelDefinition{
			Key:        panel.Key,
			Label:      localizedTextFromCapability(panel.Label),
			Kind:       defaultString(panel.Kind, "custom"),
			Permission: panel.Permission,
			Component:  panel.Component,
			Order:      panel.Order,
			Empty:      localizedTextFromCapability(panel.Empty),
		})
	}
	return result
}

func runtimeSlotsFromCapability(slots []capability.AdminRuntimeSlot) []RuntimeSlotDefinition {
	if len(slots) == 0 {
		return nil
	}
	result := make([]RuntimeSlotDefinition, 0, len(slots))
	for _, slot := range slots {
		result = append(result, RuntimeSlotDefinition{
			SlotID:        slot.SlotID,
			Region:        slot.Region,
			Label:         localizedTextFromCapability(slot.Label),
			Description:   localizedTextFromCapability(slot.Description),
			Permission:    slot.Permission,
			VisibleWhen:   slot.VisibleWhen,
			TargetSection: slot.TargetSection,
			TargetField:   slot.TargetField,
			DataBinding: RuntimeSlotDataBinding{
				Mode:   slot.DataBinding.Mode,
				Fields: append([]string(nil), slot.DataBinding.Fields...),
			},
			Variant: slot.Variant,
			Order:   slot.Order,
		})
	}
	return result
}

func defaultString(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func defaultSchema(resource string, title LocalizedText, description LocalizedText, permissionPrefix string) Schema {
	return Schema{
		Resource:    resource,
		Title:       title,
		Description: description,
		Permissions: permissions(permissionPrefix),
		FormGroups:  standardFormGroups(),
		FormLayout:  "two-column-density",
		Fields: []FieldDefinition{
			recordField("name", text("名称", "Name"), "text", true, true, true, true, true, 180, nil),
			recordField("code", text("编码", "Code"), "text", true, true, true, true, true, 180, nil),
			recordField("status", text("状态", "Status"), "select", false, true, true, true, true, 120, statusOptions()),
			recordField("description", text("描述", "Description"), "textarea", false, true, true, true, true, 280, nil),
			valueField("nameZh", text("中文名称", "Chinese Name"), "text", false, true, false, true, false, 180, nil),
			valueField("nameEn", text("英文名称", "English Name"), "text", false, true, false, true, false, 180, nil),
			valueField("descriptionZh", text("中文描述", "Chinese Description"), "textarea", false, true, false, true, false, 240, nil),
			valueField("descriptionEn", text("英文描述", "English Description"), "textarea", false, true, false, true, false, 240, nil),
			recordField("updatedAt", text("更新时间", "Updated At"), "datetime", false, false, false, false, true, 180, nil),
		},
		SearchFields:   []string{"name", "code", "status", "description"},
		DefaultSortKey: "updatedAt",
	}
}

func tenantResourceSchema() Schema {
	schema := defaultSchema(
		"tenants",
		text("租户", "Tenants"),
		text("租户空间、隔离边界和区域归属。", "Tenant spaces, isolation boundaries, and regional ownership."),
		"admin:tenant",
	)
	schema.Fields = append(schema.Fields,
		valueField("isolation", text("隔离模式", "Isolation"), "select", false, true, true, true, true, 140, []FieldOption{
			option("shared", "共享", "Shared"),
			option("sandbox", "沙箱", "Sandbox"),
		}),
		withRelation(valueField("areaCode", text("地址码", "Area Code"), "select", false, true, true, true, true, 140, nil), areaCodeFieldRelation(enabledRelationFilter())),
	)
	schema.SearchFields = []string{"name", "code", "status", "description", "isolation", "areaCode"}
	return schema
}

func overviewResourceSchema() Schema {
	schema := defaultSchema(
		"overview",
		text("概览", "Overview"),
		text("平台运行概览资源。", "Platform runtime overview resource."),
		"admin:overview",
	)
	schema.Fields = append(schema.Fields, valueField("domain", text("领域", "Domain"), "text", false, true, true, false, true, 160, nil))
	schema.SearchFields = append(schema.SearchFields, "domain")
	return schema
}

func userResourceSchema() Schema {
	schema := defaultSchema(
		"users",
		text("用户", "Users"),
		text("后台用户、账号、组织归属和角色绑定。", "Admin users, accounts, organization ownership, and role bindings."),
		"admin:user",
	)
	schema.Fields = append(schema.Fields,
		readOnlyValueField("scopeType", text("账号范围", "Account Scope"), "select", false, false, true, 130, []FieldOption{
			option("platform", "平台", "Platform"),
			option("tenant", "租户", "Tenant"),
		}),
		withRelation(valueField("tenantCode", text("租户", "Tenant"), "select", true, true, true, true, true, 150, nil), fieldRelation("tenants", "code", "name", false, enabledRelationFilter())),
		withRelation(valueField("orgUnitCode", text("机构", "Org Unit"), "select", false, true, true, true, true, 160, nil), treeFieldRelation("org-units", "code", "name", "parentCode", enabledRelationFilter())),
		withRelation(valueField("areaCode", text("地址码", "Area Code"), "select", false, true, true, true, true, 140, nil), areaCodeFieldRelation(enabledRelationFilter())),
		withRelation(valueField("roles", text("角色", "Roles"), "multiselect", false, true, true, true, true, 220, nil), fieldRelation("roles", "code", "name", true, enabledRelationFilter())),
	)
	schema.SearchFields = []string{"name", "code", "status", "description", "tenantCode", "orgUnitCode", "areaCode", "roles"}
	return schema
}

func orgUnitResourceSchema() Schema {
	schema := defaultSchema(
		"org-units",
		text("组织机构", "Org Units"),
		text("租户下的集团、公司、分支、部门、团队和门店层级。", "Group, company, branch, department, team, and store hierarchy under tenants."),
		"admin:org-unit",
	)
	schema.Fields = append(schema.Fields,
		valueField("type", text("类型", "Type"), "select", true, true, true, true, true, 130, orgUnitTypeOptions()),
		withRelation(valueField("tenantCode", text("租户", "Tenant"), "select", true, true, true, true, true, 150, nil), fieldRelation("tenants", "code", "name", false, enabledRelationFilter())),
		withRelation(valueField("parentCode", text("上级机构", "Parent"), "select", false, true, true, true, true, 160, nil), treeFieldRelation("org-units", "code", "name", "parentCode", enabledRelationFilter())),
		withRelation(valueField("areaCode", text("地址码", "Area Code"), "select", false, true, true, true, true, 140, nil), areaCodeFieldRelation(enabledRelationFilter())),
		withRelation(readOnlyValueField("roleGroupCodes", text("角色组", "Role Groups"), "multiselect", false, false, true, 220, nil), fieldRelation("role-groups", "code", "name", true, enabledRelationFilter())),
		readOnlyValueField("roleGroupCount", text("绑定角色组数", "Bound Role Groups"), "number", false, true, true, 130, nil),
		readOnlyValueField("effectiveRoleCount", text("有效角色数", "Effective Roles"), "number", false, true, true, 130, nil),
		valueField("sortOrder", text("排序", "Sort Order"), "number", false, false, false, true, true, 110, nil),
	)
	schema.SearchFields = []string{"name", "code", "status", "description", "type", "tenantCode", "parentCode", "areaCode"}
	schema.DefaultSortKey = "sortOrder"
	return schema
}

func roleGroupResourceSchema() Schema {
	schema := defaultSchema(
		"role-groups",
		text("角色组", "Role Groups"),
		text("用于角色分类、治理和授权维护，不直接授予权限。", "Classifies roles for governance and authorization maintenance without granting permissions directly."),
		"admin:role-group",
	)
	schema.Fields = append(schema.Fields,
		readOnlyValueField("scopeType", text("归属范围", "Ownership Scope"), "select", false, false, true, 130, []FieldOption{
			option("platform", "平台", "Platform"),
			option("tenant", "租户", "Tenant"),
		}),
		withRelation(readOnlyValueField("tenantCode", text("租户", "Tenant"), "select", false, false, true, 150, nil), fieldRelation("tenants", "code", "name", false, enabledRelationFilter())),
		withRelation(valueField("parentCode", text("上级角色组", "Parent Group"), "select", false, true, true, true, true, 160, nil), treeFieldRelation("role-groups", "code", "name", "parentCode", enabledRelationFilter())),
		valueField("sortOrder", text("排序", "Sort Order"), "number", false, false, false, true, true, 110, nil),
	)
	schema.SearchFields = []string{"name", "code", "status", "description", "parentCode"}
	schema.DefaultSortKey = "sortOrder"
	return schema
}

func areaCodeResourceSchema() Schema {
	schema := defaultSchema(
		"area-codes",
		text("地址码", "Area Codes"),
		text("通用行政区划或业务区域编码，供租户、机构和人员引用。", "Common administrative or business area codes referenced by tenants, org units, and users."),
		"admin:area-code",
	)
	schema.Fields = append(schema.Fields,
		withRelation(valueField("parentCode", text("上级地址码", "Parent Code"), "select", false, true, true, true, true, 150, nil), areaCodeFieldRelation(enabledRelationFilter())),
		valueField("level", text("层级", "Level"), "select", false, true, true, true, true, 130, areaLevelOptions()),
		valueField("path", text("层级路径", "Path"), "text", false, true, true, true, true, 220, nil),
		valueField("sortOrder", text("排序", "Sort Order"), "number", false, false, false, true, true, 110, nil),
	)
	schema.SearchFields = []string{"name", "code", "status", "description", "parentCode", "level", "path"}
	schema.DefaultSortKey = "sortOrder"
	return schema
}

func auditLogResourceSchema() Schema {
	legacyTargetCode := secureFieldDefinition(auditLogField("targetCode", text("目标编码", "Target Code"), "text", false, false, 180), capability.FieldSensitivityInternal, capability.FieldStoragePlain, capability.FieldProjectionOmitted, capability.FieldProjectionOmitted)
	legacyTraceID := secureFieldDefinition(auditLogField("legacyTraceId", text("旧链路 ID", "Legacy Trace ID"), "text", false, false, 180), capability.FieldSensitivityInternal, capability.FieldStoragePlain, capability.FieldProjectionOmitted, capability.FieldProjectionOmitted)
	requestID := secureFieldDefinition(auditLogField("requestId", text("请求 ID", "Request ID"), "text", true, false, 300), capability.FieldSensitivityInternal, capability.FieldStoragePlain, capability.FieldProjectionFull, capability.FieldProjectionOmitted)
	requestID.Validation = &FieldValidation{MinLength: 36, MaxLength: 36, Pattern: `^req_[0-9a-f]{32}$`}
	traceID := secureFieldDefinition(auditLogField("traceId", text("链路 ID", "Trace ID"), "text", true, false, 280), capability.FieldSensitivityInternal, capability.FieldStoragePlain, capability.FieldProjectionFull, capability.FieldProjectionOmitted)
	traceID.Validation = &FieldValidation{MinLength: 32, MaxLength: 32, Pattern: `^[0-9a-f]{32}$`}
	fields := withStandardRecordFields([]FieldDefinition{
		auditLogField("actor", text("操作人", "Actor"), "text", true, true, 140),
		auditLogField("action", text("动作", "Action"), "text", true, true, 180),
		auditLogField("resource", text("资源", "Resource"), "text", true, true, 160),
		auditLogField("targetId", text("目标 ID", "Target ID"), "text", true, false, 180),
		legacyTargetCode,
		auditLogField("provider", text("提供方", "Provider"), "text", true, true, 130),
		auditLogField("outcome", text("结果", "Outcome"), "text", true, true, 130),
		auditLogField("eventId", text("事件 ID", "Event ID"), "text", true, false, 180),
		auditLogField("reasonCode", text("原因码", "Reason Code"), "text", true, true, 160),
		auditLogField("createdAt", text("发生时间", "Created At"), "datetime", true, true, 180),
		legacyTraceID,
		requestID,
		traceID,
	})
	return Schema{
		Resource:    "audit-logs",
		Title:       text("审计日志", "Audit Logs"),
		Description: text("平台操作审计。", "Platform operation audit."),
		Permissions: ActionPermissions{
			Read: "admin:audit-log:read",
		},
		Fields: fields,
		SearchFields: []string{
			"actor",
			"action",
			"resource",
			"provider",
			"outcome",
			"eventId",
			"reasonCode",
			"requestId",
			"traceId",
		},
		DefaultSortKey: "createdAt",
	}
}

func apiResourceSchema() Schema {
	schema := defaultSchema(
		"api-resources",
		text("API 资源", "API Resources"),
		text("接口资源、权限码和调用边界。", "API resources, permission codes, and invocation boundaries."),
		"admin:api-resource",
	)
	schema.Fields = append(schema.Fields, valueField("method", text("请求方法", "Method"), "select", true, true, true, true, true, 110, []FieldOption{
		option("GET", "GET", "GET"),
		option("POST", "POST", "POST"),
		option("PUT", "PUT", "PUT"),
		option("PATCH", "PATCH", "PATCH"),
		option("DELETE", "DELETE", "DELETE"),
	}))
	schema.SearchFields = []string{"name", "code", "status", "description"}
	return schema
}

func dictionaryParameterSchema() Schema {
	schema := defaultSchema(
		"dictionary-parameters",
		text("字典参数", "Dict & Params"),
		text("字典枚举、配置参数和平台常量。", "Dictionary enums, configuration parameters, and platform constants."),
		"admin:dictionary-parameter",
	)
	schema.Fields = append(schema.Fields, valueField("scope", text("作用域", "Scope"), "text", false, true, true, true, true, 140, nil))
	schema.SearchFields = []string{"name", "code", "status", "description", "scope"}
	return schema
}

func dictionaryResourceSchema() Schema {
	schema := defaultSchema(
		"dictionaries",
		text("字典管理", "Dictionaries"),
		text("字典目录、枚举分类和业务选项分组。", "Dictionary catalogs, enum categories, and business option groups."),
		"admin:dictionary",
	)
	schema.Fields = append(schema.Fields,
		valueField("itemCount", text("条目数", "Items"), "number", false, false, true, false, true, 110, nil),
	)
	schema.SearchFields = []string{"name", "code", "status", "description"}
	return schema
}

func parameterResourceSchema() Schema {
	schema := defaultSchema(
		"parameters",
		text("参数管理", "Parameters"),
		text("平台参数、运行开关和能力配置键值。", "Platform parameters, runtime switches, and capability configuration key-values."),
		"admin:parameter",
	)
	schema.Fields = append(schema.Fields,
		valueField("value", text("参数值", "Value"), "text", true, true, true, true, true, 220, nil),
		valueField("group", text("分组", "Group"), "text", false, true, true, true, true, 140, nil),
	)
	schema.SearchFields = []string{"name", "code", "status", "description", "value", "group"}
	return schema
}

func brandingResourceSchema() Schema {
	schema := defaultSchema(
		"branding",
		text("品牌配置", "Branding"),
		text("平台品牌、主题和登录展示配置。", "Platform branding, theme, and login presentation configuration."),
		"admin:branding",
	)
	schema.Permissions.Create = ""
	schema.Permissions.Delete = ""
	schema.Fields = append(schema.Fields,
		valueField("productName", text("产品名称", "Product Name"), "text", true, true, true, true, true, 180, nil),
		valueField("shortName", text("简称", "Short Name"), "text", false, true, true, true, true, 120, nil),
		valueField("logoUrl", text("Logo URL", "Logo URL"), "text", false, false, false, true, true, 220, nil),
		valueField("faviconUrl", text("Favicon URL", "Favicon URL"), "text", false, false, false, true, true, 220, nil),
		valueField("primaryColor", text("主色", "Primary Color"), "color", false, true, true, true, true, 120, nil),
		valueField("defaultTheme", text("默认主题", "Default Theme"), "select", true, true, true, true, true, 140, []FieldOption{
			option("tech", "科技风", "Tech"),
			option("white", "高级白", "Premium White"),
			option("black", "炫酷黑", "Cool Black"),
			option("warm", "温暖黄", "Warm Yellow"),
		}),
		valueField("loginTitle", text("登录标题", "Login Title"), "text", false, true, false, true, true, 220, nil),
		valueField("loginSubtitle", text("登录副标题", "Login Subtitle"), "textarea", false, true, false, true, true, 260, nil),
	)
	schema.SearchFields = []string{"name", "code", "productName", "shortName", "defaultTheme", "loginTitle"}
	return schema
}

func settingsResourceSchema() Schema {
	schema := defaultSchema(
		"settings",
		text("设置", "Settings"),
		text("平台设置、品牌和运行体验配置。", "Platform settings, branding, and runtime experience configuration."),
		"admin:settings",
	)
	schema.Permissions.Create = ""
	schema.Permissions.Delete = ""
	schema.Fields = append(schema.Fields,
		valueField("capability", text("能力", "Capability"), "text", true, true, true, true, true, 140, nil),
		valueField("productName", text("产品名称", "Product Name"), "text", true, true, true, true, true, 180, nil),
		valueField("shortName", text("短名称", "Short Name"), "text", false, true, true, true, true, 120, nil),
		valueField("logoUrl", text("Logo URL", "Logo URL"), "text", false, false, false, true, true, 220, nil),
		valueField("faviconUrl", text("Favicon URL", "Favicon URL"), "text", false, false, false, true, true, 220, nil),
		valueField("primaryColor", text("主色", "Primary Color"), "text", false, true, true, true, true, 120, nil),
		valueField("defaultTheme", text("默认主题", "Default Theme"), "select", true, true, true, true, true, 140, []FieldOption{
			option("tech", "科技风", "Tech"),
			option("white", "高级白", "Premium White"),
			option("black", "炫酷黑", "Cool Black"),
			option("warm", "温暖黄", "Warm Yellow"),
		}),
		valueField("loginTitle", text("登录标题", "Login Title"), "text", false, true, false, true, true, 220, nil),
		valueField("loginSubtitle", text("登录副标题", "Login Subtitle"), "textarea", false, true, false, true, true, 260, nil),
	)
	schema.SearchFields = []string{"name", "code", "status", "description", "capability", "productName", "shortName", "defaultTheme", "loginTitle"}
	return schema
}

func monitoringResourceSchema() Schema {
	schema := defaultSchema(
		"monitoring",
		text("监控", "Monitoring"),
		text("实例、服务、健康状态和告警摘要。", "Instances, services, health state, and alert summaries."),
		"admin:monitoring",
	)
	schema.Fields = append(schema.Fields,
		valueField("targetType", text("目标类型", "Target Type"), "select", true, true, true, true, true, 130, []FieldOption{
			option("instance", "实例", "Instance"),
			option("service", "服务", "Service"),
			option("endpoint", "端点", "Endpoint"),
			option("job", "任务", "Job"),
			option("queue", "队列", "Queue"),
			option("custom", "自定义", "Custom"),
		}),
		valueField("health", text("健康状态", "Health"), "select", false, true, true, true, true, 120, []FieldOption{
			option("healthy", "健康", "Healthy"),
			option("warning", "预警", "Warning"),
			option("error", "异常", "Error"),
			option("unknown", "未知", "Unknown"),
		}),
		valueField("endpoint", text("端点", "Endpoint"), "text", false, true, true, true, true, 220, nil),
		valueField("lastSeenAt", text("最近上报", "Last Seen"), "datetime", false, true, true, true, true, 180, nil),
		valueField("alertCount", text("告警数", "Alerts"), "number", false, true, true, true, true, 110, nil),
	)
	schema.SearchFields = []string{"name", "code", "status", "description", "targetType", "health", "endpoint", "lastSeenAt"}
	schema.DefaultSortKey = "lastSeenAt"
	return schema
}

func roleResourceSchema(permissionOptions []FieldOption) Schema {
	schema := defaultSchema(
		"roles",
		text("角色", "Roles"),
		text("角色与权限集合。", "Roles and permission sets."),
		"admin:role",
	)
	schema.Fields = append(schema.Fields,
		withRelation(valueField("groupCode", text("角色组", "Role Group"), "select", false, true, true, true, true, 150, nil), treeFieldRelation("role-groups", "code", "name", "parentCode", enabledRelationFilter())),
		valueField("dataScope", text("数据范围", "Data Scope"), "select", true, true, true, true, true, 160, dataScopeOptions()),
		withRelation(valueField("dataScopeOrgCodes", text("数据机构", "Data Orgs"), "multiselect", false, true, false, true, true, 220, nil), multipleTreeFieldRelation("org-units", "code", "name", "parentCode", enabledRelationFilter())),
		withRelation(valueField("dataScopeAreaCodes", text("数据区域", "Data Areas"), "multiselect", false, true, false, true, true, 220, nil), multipleAreaCodeFieldRelation(enabledRelationFilter())),
		withRelation(valueField("permissions", text("权限", "Permissions"), "multiselect", true, true, true, true, true, 260, permissionOptions), fieldRelation("permissions", "code", "name", true, enabledRelationFilter())),
		withRelation(valueField("denyPermissions", text("拒绝权限", "Deny Permissions"), "multiselect", false, true, false, true, true, 260, permissionOptions), fieldRelation("permissions", "code", "name", true, enabledRelationFilter())),
	)
	schema.SearchFields = []string{"name", "code", "status", "description", "groupCode", "dataScope", "dataScopeOrgCodes", "dataScopeAreaCodes", "permissions", "denyPermissions"}
	return schema
}

func permissionResourceSchema() Schema {
	schema := defaultSchema(
		"permissions",
		text("权限", "Permissions"),
		text("能力、资源、菜单和接口共用的权限码目录。", "Permission catalog shared by capabilities, resources, menus, and APIs."),
		"admin:permission",
	)
	schema.Fields = append(schema.Fields,
		valueField("resourceType", text("资源类型", "Resource Type"), "select", true, true, true, true, true, 130, []FieldOption{
			option("api", "接口权限", "API"),
			option("page-button", "页面按钮", "Page Button"),
		}),
		valueField("capability", text("能力", "Capability"), "text", true, true, true, true, true, 150, nil),
		valueField("resource", text("资源", "Resource"), "text", true, true, true, true, true, 160, nil),
		valueField("action", text("动作", "Action"), "select", true, true, true, true, true, 120, []FieldOption{
			option("read", "读取", "Read"),
			option("create", "创建", "Create"),
			option("update", "更新", "Update"),
			option("delete", "删除", "Delete"),
		}),
		valueField("prefix", text("前缀", "Prefix"), "text", true, true, true, true, true, 180, nil),
	)
	schema.SearchFields = []string{"name", "code", "status", "description", "resourceType", "capability", "resource", "action", "prefix"}
	return schema
}

func menuResourceSchema() Schema {
	schema := defaultSchema(
		"menus",
		text("菜单", "Menus"),
		text("后台菜单和资源入口。", "Admin menus and resource entries."),
		"admin:menu",
	)
	schema.Fields = append(schema.Fields,
		valueField("nodeType", text("节点类型", "Node Type"), "select", false, true, true, true, true, 120, []FieldOption{
			option("directory", "目录", "Directory"),
			option("page", "页面", "Page"),
		}),
		withRelation(valueField("parentCode", text("上级菜单", "Parent Menu"), "select", false, true, true, true, true, 160, nil), treeFieldRelation("menus", "code", "name", "parentCode", enabledRelationFilter())),
		valueField("route", text("路由", "Route"), "text", false, true, true, true, true, 180, nil),
		valueField("componentKey", text("组件键", "Component Key"), "text", false, true, true, true, true, 180, nil),
		valueField("resourceCode", text("资源编码", "Resource Code"), "text", false, true, true, true, true, 160, nil),
		valueField("isExternal", text("是否外链", "External Link"), "switch", false, true, true, true, true, 110, nil),
		valueField("externalUrl", text("外链地址", "External URL"), "text", false, true, false, true, true, 240, nil),
		valueField("openMode", text("打开方式", "Open Mode"), "select", false, true, true, true, true, 130, []FieldOption{
			option("same-tab", "当前页", "Same Tab"),
			option("new-tab", "新标签页", "New Tab"),
		}),
		valueField("parameters", text("页面参数", "Page Parameters"), "textarea", false, false, false, true, true, 280, nil),
		valueField("cacheEnabled", text("启用缓存", "Cache Enabled"), "switch", false, true, true, true, true, 110, nil),
		valueField("hidden", text("隐藏菜单", "Hidden"), "switch", false, true, true, true, true, 110, nil),
		withRelation(valueField("activeMenuCode", text("激活菜单", "Active Menu"), "select", false, true, false, true, true, 160, nil), treeFieldRelation("menus", "code", "name", "parentCode", enabledRelationFilter())),
		valueField("breadcrumbVisible", text("显示面包屑", "Breadcrumb Visible"), "switch", false, true, true, true, true, 130, nil),
		readOnlyValueField("pageButtons", text("页面按钮", "Page Buttons"), "textarea", false, false, true, 280, nil),
		valueField("parent", text("上级菜单", "Parent Menu"), "text", false, true, true, true, true, 160, nil),
		valueField("resource", text("资源", "Resource"), "text", false, true, true, true, true, 160, nil),
		valueField("permission", text("旧权限码", "Legacy Permission"), "text", false, true, true, true, true, 220, nil),
		valueField("group", text("分组", "Group"), "select", false, true, true, true, true, 140, []FieldOption{
			option("foundation", "基础", "Foundation"),
			option("governance", "治理", "Governance"),
			option("operations", "运维", "Operations"),
			option("security", "安全", "Security"),
		}),
		valueField("icon", text("图标", "Icon"), "text", false, false, true, true, true, 140, nil),
		valueField("order", text("排序", "Order"), "text", false, false, true, true, true, 90, nil),
		valueField("titleZh", text("中文标题", "Chinese Title"), "text", true, true, false, true, true, 160, nil),
		valueField("titleEn", text("英文标题", "English Title"), "text", true, true, false, true, true, 160, nil),
	)
	schema.Actions = []ResourceActionDefinition{
		{
			Key:        "copy-config",
			Label:      text("复制配置", "Copy Config"),
			Kind:       "row",
			Tone:       "default",
			Icon:       "copy",
			Permission: "admin:menu:read",
		},
	}
	schema.Panels = []ResourcePanelDefinition{
		{
			Key:        "audit",
			Label:      text("审计", "Audit"),
			Kind:       "audit",
			Permission: "admin:menu:read",
			Component:  "audit-timeline",
			Order:      30,
			Empty:      text("暂无审计记录", "No audit records"),
		},
	}
	schema.FormLayout = "side-detail-preview"
	schema.RuntimeSlots = []RuntimeSlotDefinition{
		{
			SlotID:      "platform.record-summary",
			Region:      "side.preview",
			Label:       text("记录摘要", "Record Summary"),
			Description: text("展示当前菜单的路由、权限和状态。", "Shows the current menu route, permission and status."),
			Permission:  "admin:menu:read",
			DataBinding: RuntimeSlotDataBinding{Mode: "record", Fields: []string{"code", "route", "permission", "status"}},
			Variant:     "preview",
			Order:       10,
		},
		{
			SlotID:      "platform.permission-summary",
			Region:      "side.preview",
			Label:       text("权限摘要", "Permission Summary"),
			Description: text("展示该资源的标准操作权限码。", "Shows standard action permission codes for this resource."),
			Permission:  "admin:menu:read",
			DataBinding: RuntimeSlotDataBinding{Mode: "resource"},
			Variant:     "compact",
			Order:       20,
		},
		{
			SlotID:      "platform.localized-preview",
			Region:      "side.preview",
			Label:       text("多语言预览", "Localized Preview"),
			Description: text("预览菜单标题的中英文展示值。", "Previews Chinese and English menu title values."),
			Permission:  "admin:menu:read",
			DataBinding: RuntimeSlotDataBinding{Mode: "record", Fields: []string{"titleZh", "titleEn"}},
			Variant:     "inline",
			Order:       30,
		},
	}
	schema.SearchFields = []string{
		"name", "code", "status", "description", "nodeType", "parentCode", "route", "componentKey", "resourceCode",
		"isExternal", "externalUrl", "openMode", "cacheEnabled", "hidden", "activeMenuCode", "breadcrumbVisible",
		"parent", "resource", "permission", "group", "titleZh", "titleEn",
	}
	return schema
}

func recordField(key string, label LocalizedText, fieldType string, required bool, searchable bool, inTable bool, inForm bool, inDetail bool, width int, options []FieldOption) FieldDefinition {
	return field(key, label, fieldType, "record", required, searchable, inTable, inForm, inDetail, width, options)
}

func valueField(key string, label LocalizedText, fieldType string, required bool, searchable bool, inTable bool, inForm bool, inDetail bool, width int, options []FieldOption) FieldDefinition {
	return field(key, label, fieldType, "values", required, searchable, inTable, inForm, inDetail, width, options)
}

func readOnlyValueField(key string, label LocalizedText, fieldType string, searchable bool, inTable bool, inDetail bool, width int, options []FieldOption) FieldDefinition {
	field := valueField(key, label, fieldType, false, searchable, inTable, false, inDetail, width, options)
	field.ReadOnly = true
	return field
}

func auditLogField(key string, label LocalizedText, fieldType string, searchable bool, inTable bool, width int) FieldDefinition {
	field := valueField(key, label, fieldType, false, searchable, inTable, false, true, width, nil)
	field.ReadOnly = true
	return field
}

func withRelation(field FieldDefinition, relation FieldRelation) FieldDefinition {
	field.Relation = &relation
	return field
}

func fieldRelation(resource string, valueField string, labelField string, multiple bool, filters ...FieldRelationFilter) FieldRelation {
	return FieldRelation{
		Resource:   resource,
		ValueField: valueField,
		LabelField: labelField,
		Multiple:   multiple,
		Filters:    filters,
		SortField:  labelField,
		SortOrder:  "asc",
	}
}

func treeFieldRelation(resource string, valueField string, labelField string, parentField string, filters ...FieldRelationFilter) FieldRelation {
	relation := fieldRelation(resource, valueField, labelField, false, filters...)
	relation.Display = "tree"
	relation.ParentField = parentField
	return relation
}

func multipleTreeFieldRelation(resource string, valueField string, labelField string, parentField string, filters ...FieldRelationFilter) FieldRelation {
	relation := treeFieldRelation(resource, valueField, labelField, parentField, filters...)
	relation.Multiple = true
	return relation
}

func areaCodeFieldRelation(filters ...FieldRelationFilter) FieldRelation {
	relation := treeFieldRelation("area-codes", "code", "name", "parentCode", filters...)
	relation.PathField = "path"
	return relation
}

func multipleAreaCodeFieldRelation(filters ...FieldRelationFilter) FieldRelation {
	relation := areaCodeFieldRelation(filters...)
	relation.Multiple = true
	return relation
}

func enabledRelationFilter() FieldRelationFilter {
	return FieldRelationFilter{Field: "status", Operator: "=", Value: "enabled"}
}

func field(key string, label LocalizedText, fieldType string, source string, required bool, searchable bool, inTable bool, inForm bool, inDetail bool, width int, options []FieldOption) FieldDefinition {
	return FieldDefinition{
		Key:          key,
		Label:        label,
		Type:         fieldType,
		Source:       source,
		Group:        defaultFormGroupForField(key),
		Required:     required,
		Searchable:   searchable,
		Filterable:   defaultFilterableField(key, fieldType, searchable),
		Sortable:     defaultSortableField(key, fieldType, inTable),
		Localizable:  defaultLocalizableField(key),
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

func secureFieldDefinition(field FieldDefinition, sensitivity string, storageMode string, responseMode string, exportMode string) FieldDefinition {
	field.Sensitivity = sensitivity
	field.StorageMode = storageMode
	field.ResponseMode = responseMode
	field.ExportMode = exportMode
	return field
}

func defaultFilterableField(key string, fieldType string, searchable bool) bool {
	return searchable || key == "status" || fieldType == "select" || fieldType == "multiselect" || fieldType == "switch" || fieldType == "datetime" || fieldType == "number"
}

func defaultSortableField(key string, fieldType string, inTable bool) bool {
	if key == "id" || key == "updatedAt" || fieldType == "datetime" || fieldType == "number" {
		return true
	}
	return inTable && fieldType != "textarea" && fieldType != "multiselect"
}

func defaultLocalizableField(key string) bool {
	return key == "name" || key == "description"
}

func text(zh string, en string) LocalizedText {
	return LocalizedText{ZH: zh, EN: en}
}

func localizedTextFromCapability(value capability.LocalizedText) LocalizedText {
	return LocalizedText{ZH: value.ZH, EN: value.EN}
}

func localizedTextPointerFromCapability(value capability.LocalizedText) *LocalizedText {
	if value.ZH == "" && value.EN == "" {
		return nil
	}
	text := localizedTextFromCapability(value)
	return &text
}

func fieldsFromCapability(fields []capability.AdminField) []FieldDefinition {
	definitions := make([]FieldDefinition, 0, len(fields))
	for _, field := range fields {
		options := make([]FieldOption, 0, len(field.Options))
		for _, option := range field.Options {
			options = append(options, FieldOption{
				Value: option.Value,
				Label: localizedTextFromCapability(option.Label),
			})
		}
		definitions = append(definitions, FieldDefinition{
			Key:          field.Key,
			Label:        localizedTextFromCapability(field.Label),
			Type:         field.Type,
			Source:       field.Source,
			Group:        field.Group,
			Help:         localizedTextPointerFromCapability(field.Help),
			Required:     field.Required,
			ReadOnly:     field.ReadOnly,
			Searchable:   field.Searchable,
			Filterable:   field.Filterable || defaultFilterableField(field.Key, field.Type, field.Searchable),
			Sortable:     field.Sortable || defaultSortableField(field.Key, field.Type, field.InTable),
			Localizable:  field.Localizable || defaultLocalizableField(field.Key),
			InTable:      field.InTable,
			InForm:       field.InForm,
			InDetail:     field.InDetail,
			Width:        field.Width,
			Options:      options,
			Relation:     relationFromCapability(field.Relation),
			Validation:   validationFromCapability(field.Validation),
			Sensitivity:  defaultString(field.Sensitivity, capability.FieldSensitivityPublic),
			StorageMode:  defaultString(field.StorageMode, capability.FieldStoragePlain),
			ResponseMode: defaultString(field.ResponseMode, capability.FieldProjectionFull),
			ExportMode:   defaultString(field.ExportMode, capability.FieldProjectionFull),
			Protection:   fieldProtectionFromCapability(field.Protection),
			Masking:      fieldMaskingFromCapability(field.Masking),
			Reveal:       fieldRevealFromCapability(field.Reveal),
		})
	}
	return withLocalizedValueFields(withStandardRecordFields(definitions))
}

func withStandardRecordFields(fields []FieldDefinition) []FieldDefinition {
	declared := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		declared[field.Key] = struct{}{}
	}
	standard := []FieldDefinition{
		recordField("name", text("名称", "Name"), "text", false, true, false, false, false, 180, nil),
		recordField("code", text("编码", "Code"), "text", false, true, false, false, false, 180, nil),
		recordField("status", text("状态", "Status"), "select", false, true, false, false, false, 120, statusOptions()),
		recordField("description", text("描述", "Description"), "textarea", false, false, false, false, false, 280, nil),
		recordField("updatedAt", text("更新时间", "Updated At"), "datetime", false, false, false, false, false, 180, nil),
	}
	for _, field := range standard {
		if _, exists := declared[field.Key]; exists {
			continue
		}
		field.ReadOnly = true
		fields = append(fields, field)
	}
	return fields
}

func withLocalizedValueFields(fields []FieldDefinition) []FieldDefinition {
	declared := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		declared[field.Key] = struct{}{}
	}
	localized := []FieldDefinition{
		valueField("nameZh", text("中文名称", "Chinese Name"), "text", false, true, false, true, false, 180, nil),
		valueField("nameEn", text("英文名称", "English Name"), "text", false, true, false, true, false, 180, nil),
		valueField("descriptionZh", text("中文描述", "Chinese Description"), "textarea", false, true, false, true, false, 240, nil),
		valueField("descriptionEn", text("英文描述", "English Description"), "textarea", false, true, false, true, false, 240, nil),
	}
	for _, field := range fields {
		if field.Source != "record" || !field.Localizable {
			continue
		}
		prefix := field.Key
		for _, candidate := range localized {
			if !strings.HasPrefix(candidate.Key, prefix) {
				continue
			}
			if _, exists := declared[candidate.Key]; exists {
				continue
			}
			fields = append(fields, candidate)
			declared[candidate.Key] = struct{}{}
		}
	}
	return fields
}

func relationFromCapability(value *capability.AdminFieldRelation) *FieldRelation {
	if value == nil {
		return nil
	}
	filters := make([]FieldRelationFilter, 0, len(value.Filters))
	for _, filter := range value.Filters {
		filters = append(filters, FieldRelationFilter{
			Field:    filter.Field,
			Operator: filter.Operator,
			Value:    filter.Value,
		})
	}
	return &FieldRelation{
		Resource:    value.Resource,
		ValueField:  value.ValueField,
		LabelField:  value.LabelField,
		Multiple:    value.Multiple,
		Filters:     filters,
		SortField:   value.SortField,
		SortOrder:   value.SortOrder,
		Display:     value.Display,
		ParentField: value.ParentField,
		PathField:   value.PathField,
		RootValue:   value.RootValue,
	}
}

func formGroupsFromCapability(groups []capability.AdminFormGroup, fields []FieldDefinition) []FormGroupDefinition {
	if len(groups) == 0 {
		return inferredFormGroups(fields)
	}
	definitions := make([]FormGroupDefinition, 0, len(groups))
	for _, group := range groups {
		if group.Key == "" {
			continue
		}
		definitions = append(definitions, FormGroupDefinition{
			Key:         group.Key,
			Label:       localizedTextFromCapability(group.Label),
			Description: localizedTextFromCapability(group.Description),
		})
	}
	return definitions
}

func formLayoutFromCapability(layout string, fields []FieldDefinition) string {
	switch layout {
	case "single-column", "grouped-sections", "two-column-density", "side-detail-preview":
		return layout
	}
	formFieldCount := 0
	for _, field := range fields {
		if field.InForm && !field.ReadOnly {
			formFieldCount++
		}
	}
	if formFieldCount >= 6 {
		return "two-column-density"
	}
	return "grouped-sections"
}

func inferredFormGroups(fields []FieldDefinition) []FormGroupDefinition {
	seen := map[string]bool{}
	var groups []FormGroupDefinition
	for _, field := range fields {
		if field.Group == "" || seen[field.Group] {
			continue
		}
		seen[field.Group] = true
		if group, ok := standardFormGroup(field.Group); ok {
			groups = append(groups, group)
		}
	}
	return groups
}

func standardFormGroups() []FormGroupDefinition {
	return []FormGroupDefinition{
		mustStandardFormGroup("basic"),
		mustStandardFormGroup("localization"),
	}
}

func standardFormGroup(key string) (FormGroupDefinition, bool) {
	switch key {
	case "basic":
		return FormGroupDefinition{
			Key:         "basic",
			Label:       text("基础信息", "Basic Info"),
			Description: text("资源的名称、编码、状态和说明。", "Name, code, status, and description for the resource."),
		}, true
	case "localization":
		return FormGroupDefinition{
			Key:         "localization",
			Label:       text("多语言内容", "Localized Content"),
			Description: text("可按需填写中英文展示内容。", "Optional Chinese and English display content."),
		}, true
	default:
		return FormGroupDefinition{}, false
	}
}

func mustStandardFormGroup(key string) FormGroupDefinition {
	group, ok := standardFormGroup(key)
	if !ok {
		panic("unknown standard form group: " + key)
	}
	return group
}

func defaultFormGroupForField(key string) string {
	switch key {
	case "name", "code", "status", "description":
		return "basic"
	case "nameZh", "nameEn", "descriptionZh", "descriptionEn", "titleZh", "titleEn":
		return "localization"
	default:
		return ""
	}
}

func validationFromCapability(value capability.AdminFieldValidation) *FieldValidation {
	if value.MinLength == 0 && value.MaxLength == 0 && value.Min == nil && value.Max == nil && value.Pattern == "" {
		return nil
	}
	return &FieldValidation{
		MinLength: value.MinLength,
		MaxLength: value.MaxLength,
		Min:       value.Min,
		Max:       value.Max,
		Pattern:   value.Pattern,
	}
}

func option(value string, zh string, en string) FieldOption {
	return FieldOption{Value: value, Label: text(zh, en)}
}

func statusOptions() []FieldOption {
	return []FieldOption{
		option("enabled", "已启用", "Enabled"),
		option("disabled", "已停用", "Disabled"),
		option("healthy", "健康", "Healthy"),
		option("recorded", "已记录", "Recorded"),
	}
}

func dataScopeOptions() []FieldOption {
	return []FieldOption{
		option("all", "全部数据", "All Data"),
		option("current_org", "当前机构", "Current Org"),
		option("current_and_children", "当前及下级机构", "Current And Children"),
		option("custom_orgs", "自定义机构", "Custom Orgs"),
		option("current_area", "当前区域", "Current Area"),
		option("current_and_children_areas", "当前及下级区域", "Current And Children Areas"),
		option("custom_areas", "自定义区域", "Custom Areas"),
		option("self", "本人数据", "Self"),
	}
}

func orgUnitTypeOptions() []FieldOption {
	return []FieldOption{
		option("group", "集团", "Group"),
		option("company", "公司", "Company"),
		option("branch", "分支机构", "Branch"),
		option("organization", "机构", "Organization"),
		option("department", "部门", "Department"),
		option("team", "团队", "Team"),
		option("store", "门店", "Store"),
		option("custom", "自定义", "Custom"),
	}
}

func areaLevelOptions() []FieldOption {
	return []FieldOption{
		option("continent", "洲/大区", "Continent / Region"),
		option("country", "国家", "Country"),
		option("subdivision", "一级行政区", "Subdivision"),
		option("state", "州/邦", "State"),
		option("province", "省/直辖市", "Province"),
		option("city", "城市", "City"),
		option("district", "区县", "District"),
		option("street", "街道", "Street"),
		option("custom", "自定义区域", "Custom Area"),
	}
}

func boolOptions() []FieldOption {
	return []FieldOption{
		option("true", "是", "Yes"),
		option("false", "否", "No"),
	}
}

func permissions(prefix string) ActionPermissions {
	return ActionPermissions{
		Read:   prefix + ":read",
		Create: prefix + ":create",
		Update: prefix + ":update",
		Delete: prefix + ":delete",
	}
}
