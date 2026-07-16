package adminresource

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"platform-go/internal/platform/capability"
	"platform-go/internal/platform/dataprotection"
	"platform-go/internal/platform/kernel"
	"platform-go/internal/platform/masking"
)

var (
	ErrUnknownResource = errors.New("unknown admin resource")
	ErrRecordNotFound  = errors.New("admin resource record not found")
	ErrInvalidRecord   = errors.New("invalid admin resource record")
)

type ValidationError struct {
	Field string
}

func (e ValidationError) Error() string {
	return e.Field + " is required"
}

func (e ValidationError) Is(target error) bool {
	return target == ErrInvalidRecord
}

type Record struct {
	ID                    string            `json:"id"`
	Code                  string            `json:"code"`
	Name                  string            `json:"name"`
	Status                string            `json:"status"`
	Description           string            `json:"description,omitempty"`
	UpdatedAt             string            `json:"updatedAt"`
	Values                map[string]string `json:"values,omitempty"`
	DeletedAt             string            `json:"deletedAt,omitempty"`
	DeletedBy             string            `json:"deletedBy,omitempty"`
	DeleteReason          string            `json:"deleteReason,omitempty"`
	PurgeAfter            string            `json:"purgeAfter,omitempty"`
	DeletionPolicyVersion uint32            `json:"deletionPolicyVersion,omitempty"`
}

type WriteInput struct {
	Code        string            `json:"code"`
	Name        string            `json:"name"`
	Status      string            `json:"status"`
	Description string            `json:"description"`
	Values      map[string]string `json:"values"`
}

type Store struct {
	mu            sync.Mutex
	seedResources map[string][]Record
	resources     map[string][]Record
	schemas       map[string]Schema
	nextID        int
	revision      uint64
	now           func() time.Time
	correlationFn func() (kernel.Correlation, error)
	repository    AdminResourceRepository
	protection    dataprotection.Runtime
	masking       masking.Runtime
}

func NewStore() *Store {
	return newStore(seedResources(), seedResourceSchemas())
}

func (s *Store) persistLocked() error {
	return s.persistContextLocked(context.Background())
}

func (s *Store) persistContextLocked(ctx context.Context) error {
	snapshot := s.snapshotLocked()
	if err := s.validateSnapshot(snapshot); err != nil {
		return err
	}
	if s.repository == nil {
		return nil
	}
	committed, err := s.repository.Save(ctx, snapshot)
	if err != nil {
		return err
	}
	s.revision = committed
	return nil
}

func NewStoreFromCapabilities(manifests []capability.Manifest) *Store {
	return newStore(seedResourcesFromCapabilities(manifests), seedResourceSchemasFromCapabilities(manifests))
}

func NewStoreFromCapabilitiesWithProtection(manifests []capability.Manifest, runtime dataprotection.Runtime) (*Store, error) {
	store := newStore(seedResourcesFromCapabilities(manifests), seedResourceSchemasFromCapabilities(manifests))
	store.protection = runtime
	if err := store.validateProtectionRuntime(); err != nil {
		return nil, err
	}
	if err := store.protectSeedResources(context.Background()); err != nil {
		return nil, err
	}
	return store, nil
}

func newStore(resources map[string][]Record, schemas map[string]Schema) *Store {
	store := &Store{
		seedResources: cloneResourceMap(resources),
		resources:     cloneResourceMap(resources),
		schemas:       schemas,
		nextID:        1000,
		now:           time.Now,
		correlationFn: kernel.GenerateCorrelation,
		masking:       masking.NewRuntime(),
	}
	return store
}

func (s *Store) Reload() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reloadLocked()
}

func (s *Store) RefreshContext(ctx context.Context) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.repository == nil {
		return false, nil
	}
	revisionReader, revisionAware := s.repository.(AdminResourceRevisionReader)
	if revisionAware {
		current, err := revisionReader.CurrentRevision(ctx)
		if err != nil {
			return false, err
		}
		if current == s.revision {
			return false, nil
		}
	}
	previousRevision := s.revision
	if err := s.reloadContextLocked(ctx); err != nil {
		return false, err
	}
	if !revisionAware {
		return true, nil
	}
	return s.revision != previousRevision, nil
}

func (s *Store) RepositoryBacked() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.repository != nil
}

func (s *Store) reloadLocked() error {
	return s.reloadContextLocked(context.Background())
}

func (s *Store) reloadContextLocked(ctx context.Context) error {
	if s.repository == nil {
		return nil
	}
	snapshot, err := s.repository.Load(ctx)
	if err != nil {
		return err
	}
	snapshot, changed, err := s.scrubSnapshot(snapshot)
	if err != nil {
		return err
	}
	if changed {
		committed, saveErr := s.repository.Save(ctx, snapshot)
		if saveErr != nil {
			return saveErr
		}
		snapshot.Revision = committed
	}
	s.installSnapshotLocked(snapshot)
	return nil
}

func (s *Store) prepareMutationLocked() (ResourceSnapshot, error) {
	if err := s.reloadLocked(); err != nil {
		return ResourceSnapshot{}, err
	}
	return s.snapshotLocked(), nil
}

func (s *Store) List(resource string) ([]Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items, ok := s.resources[resource]
	if !ok {
		return nil, ErrUnknownResource
	}
	return cloneRecords(visibleRecords(resource, items)), nil
}

func (s *Store) Create(resource string, input WriteInput) (Record, error) {
	return s.create(resource, input, WriteOriginExternal)
}

func (s *Store) CreateInternal(resource string, input WriteInput) (Record, error) {
	return s.create(resource, input, WriteOriginInternal)
}

func (s *Store) create(resource string, input WriteInput, origin WriteOrigin) (Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	previous, err := s.prepareMutationLocked()
	if err != nil {
		return Record{}, err
	}
	items, ok := s.resources[resource]
	if !ok {
		return Record{}, ErrUnknownResource
	}
	nextID := s.nextID + 1
	record, err := s.recordFromInputWithOrigin(resource, fmt.Sprintf("%s-%d", resource, nextID), input, origin)
	if err != nil {
		return Record{}, err
	}
	if err := s.protectRecordForStorage(context.Background(), resource, &record, nil); err != nil {
		return Record{}, err
	}
	result, err := s.mutationRecordResultLocked(resource, record, origin)
	if err != nil {
		return Record{}, err
	}
	s.nextID = nextID
	s.resources[resource] = append(items, record)
	if err := s.persistLocked(); err != nil {
		s.restoreSnapshotLocked(previous)
		return Record{}, err
	}
	return result, nil
}

func (s *Store) Update(resource string, id string, input WriteInput) (Record, error) {
	return s.update(resource, id, input, WriteOriginExternal)
}

func (s *Store) UpdateInternal(resource string, id string, input WriteInput) (Record, error) {
	return s.update(resource, id, input, WriteOriginInternal)
}

func (s *Store) update(resource string, id string, input WriteInput, origin WriteOrigin) (Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	previous, err := s.prepareMutationLocked()
	if err != nil {
		return Record{}, err
	}
	items, ok := s.resources[resource]
	if !ok {
		return Record{}, ErrUnknownResource
	}
	index := slices.IndexFunc(items, func(record Record) bool {
		return record.ID == id
	})
	if index < 0 {
		return Record{}, ErrRecordNotFound
	}
	if isLifecycleDeleted(items[index]) {
		return Record{}, ErrRecordDeleted
	}
	if strings.TrimSpace(input.Code) == "" {
		input.Code = items[index].Code
	}
	record, err := s.recordFromInputWithOriginExisting(resource, id, input, origin, &items[index])
	if err != nil {
		return Record{}, err
	}
	if err := s.protectRecordForStorage(context.Background(), resource, &record, &items[index]); err != nil {
		return Record{}, err
	}
	result, err := s.mutationRecordResultLocked(resource, record, origin)
	if err != nil {
		return Record{}, err
	}
	items[index] = record
	s.resources[resource] = items
	if err := s.persistLocked(); err != nil {
		s.restoreSnapshotLocked(previous)
		return Record{}, err
	}
	return result, nil
}

func (s *Store) Delete(resource string, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	previous, err := s.prepareMutationLocked()
	if err != nil {
		return err
	}
	_, nextItems, err := s.deletionMutationLocked(resource, id, "system", "deleted")
	if err != nil {
		return err
	}
	s.resources[resource] = nextItems
	if err := s.persistLocked(); err != nil {
		s.restoreSnapshotLocked(previous)
		return err
	}
	return nil
}

func (s *Store) recordFromInput(resource string, id string, input WriteInput) (Record, error) {
	return s.recordFromInputWithOrigin(resource, id, input, WriteOriginExternal)
}

func (s *Store) recordFromInputWithOrigin(resource string, id string, input WriteInput, origin WriteOrigin) (Record, error) {
	return s.recordFromInputWithOriginExisting(resource, id, input, origin, nil)
}

func (s *Store) recordFromInputWithOriginExisting(resource string, id string, input WriteInput, origin WriteOrigin, existing *Record) (Record, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return Record{}, ValidationError{Field: "name"}
	}
	if err := s.validateRequiredFieldsExisting(resource, input, existing); err != nil {
		return Record{}, err
	}
	if err := s.validateWriteInput(resource, input, origin); err != nil {
		return Record{}, err
	}
	status := strings.TrimSpace(input.Status)
	if status == "" {
		status = "enabled"
	}
	code := strings.TrimSpace(input.Code)
	values := cloneValues(input.Values)
	if existing != nil {
		values = s.mergeExistingUpdateValues(resource, values, *existing)
	}
	return Record{
		ID:          id,
		Code:        code,
		Name:        name,
		Status:      status,
		Description: strings.TrimSpace(input.Description),
		UpdatedAt:   s.now().UTC().Format(time.RFC3339),
		Values:      values,
	}, nil
}

func (s *Store) mergeExistingUpdateValues(resource string, values map[string]string, existing Record) map[string]string {
	schema := s.schemas[resource]
	for _, rawField := range schema.Fields {
		field := defaultFieldPolicy(rawField)
		if field.Source != "values" || field.StorageMode == capability.FieldStorageEncrypted || !preserveUnsubmittedUpdateField(field) {
			continue
		}
		if _, submitted := values[field.Key]; submitted {
			continue
		}
		if value, exists := existing.Values[field.Key]; exists {
			if values == nil {
				values = map[string]string{}
			}
			values[field.Key] = value
		}
	}
	return values
}

func preserveUnsubmittedUpdateField(field FieldDefinition) bool {
	return field.ReadOnly || !field.InForm || field.Sensitivity != capability.FieldSensitivityPublic ||
		field.StorageMode == capability.FieldStorageHashed ||
		field.ResponseMode == capability.FieldProjectionOmitted || field.ResponseMode == capability.FieldProjectionPrivileged
}

func (s *Store) validateRequiredFields(resource string, input WriteInput) error {
	return s.validateRequiredFieldsExisting(resource, input, nil)
}

func (s *Store) validateRequiredFieldsExisting(resource string, input WriteInput, existing *Record) error {
	schema, ok := s.schemas[resource]
	if !ok {
		return ErrUnknownResource
	}
	for _, field := range schema.Fields {
		if !field.Required {
			continue
		}
		var value string
		switch field.Source {
		case "record":
			value = recordInputValue(input, field.Key)
		case "values":
			var submitted bool
			value, submitted = input.Values[field.Key]
			preserveMissing := !submitted && (field.StorageMode == capability.FieldStorageEncrypted || preserveUnsubmittedUpdateField(field))
			if existing != nil && (preserveMissing || (field.StorageMode == capability.FieldStorageEncrypted && strings.TrimSpace(value) == "")) {
				value = existing.Values[field.Key]
			}
		}
		if strings.TrimSpace(value) == "" {
			return ValidationError{Field: field.Key}
		}
	}
	return nil
}

func (s *Store) mutationRecordResultLocked(resource string, record Record, origin WriteOrigin) (Record, error) {
	if origin == WriteOriginInternal {
		result := cloneRecord(record)
		schema, ok := s.schemas[resource]
		if !ok {
			return Record{}, ErrUnknownResource
		}
		for _, field := range schema.Fields {
			if field.StorageMode == capability.FieldStorageEncrypted && field.Source == "values" {
				delete(result.Values, field.Key)
			}
		}
		if len(result.Values) == 0 {
			result.Values = nil
		}
		return result, nil
	}
	return s.projectRecordLocked(resource, record, ProjectionResponse)
}

func (s *Store) protectSeedResources(ctx context.Context) error {
	protected := cloneResourceMap(s.seedResources)
	for resource, records := range protected {
		for index := range records {
			if err := s.protectRecordForStorage(ctx, resource, &records[index], nil); err != nil {
				return fmt.Errorf("protect seed resource %s record %s: %w", resource, records[index].ID, err)
			}
		}
		protected[resource] = records
	}
	s.seedResources = protected
	s.resources = cloneResourceMap(protected)
	return nil
}

func recordInputValue(input WriteInput, key string) string {
	switch key {
	case "code":
		return input.Code
	case "name":
		return input.Name
	case "status":
		return input.Status
	case "description":
		return input.Description
	default:
		return ""
	}
}

func cloneRecords(records []Record) []Record {
	cloned := make([]Record, 0, len(records))
	for _, record := range records {
		cloned = append(cloned, cloneRecord(record))
	}
	return cloned
}

func cloneRecord(record Record) Record {
	record.Values = cloneValues(record.Values)
	return record
}

func cloneValues(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func seedResources() map[string][]Record {
	updatedAt := "2026-07-04T00:00:00Z"
	resources := map[string][]Record{
		"overview": {
			seedLocalized("overview-platform", "platform", "平台运行", "Platform Runtime", "healthy", "平台运行概览。", "Platform runtime overview.", updatedAt, map[string]string{"domain": "foundation"}),
		},
		"tenants": {
			seedLocalized("tenant-platform", "platform", "平台租户", "Platform Tenant", "enabled", "平台管理默认租户。", "Default tenant for platform administration.", updatedAt, map[string]string{"isolation": "shared"}),
			seedLocalized("tenant-demo", "demo", "演示租户", "Demo Tenant", "enabled", "用于演示和测试数据的可复用租户。", "Reusable tenant for demos and fixtures.", updatedAt, map[string]string{"isolation": "sandbox"}),
		},
		"users": {
			seedLocalized("user-admin", "admin", "平台管理员", "Platform Admin", "enabled", "默认管理员账号。", "Default administrator account.", updatedAt, map[string]string{"roles": "super-admin", "tenantCode": "platform", "orgUnitCode": "platform-hq", "areaCode": "110000"}),
			seedLocalized("user-ops", "ops", "运维用户", "Operations User", "enabled", "用于监控任务的运维账号。", "Operations account for monitoring tasks.", updatedAt, map[string]string{"roles": "operator", "tenantCode": "platform", "orgUnitCode": "platform-ops", "areaCode": "110000"}),
		},
		"roles": {
			seedLocalized("role-super-admin", "super-admin", "超级管理员", "Super Admin", "enabled", "拥有完整平台管理权限的角色。", "Full platform administration role.", updatedAt, map[string]string{"groupCode": "system-admin", "dataScope": "all", "permissions": "*"}),
			seedLocalized("role-platform-admin", "platform-admin", "平台管理员", "Platform Admin", "enabled", "标准平台资源管理角色。", "Standard platform resource management role.", updatedAt, map[string]string{"groupCode": "system-admin", "dataScope": "all", "permissions": "admin:*"}),
			seedLocalized("role-operator", "operator", "运维人员", "Operator", "enabled", "用于平台导航演示的只读运维角色。", "Read-only operations role for platform navigation demos.", updatedAt, map[string]string{"groupCode": "operations", "dataScope": "current_org", "permissions": "admin:overview:read,admin:capability:read,admin:tenant:read,admin:monitoring:read"}),
		},
		"org-units":   seedRowsForResource("org-units", updatedAt),
		"role-groups": seedRowsForResource("role-groups", updatedAt),
		"area-codes":  seedRowsForResource("area-codes", updatedAt),
		"permissions": {
			seedLocalized("permission-all", "*", "全部权限", "All Permissions", "enabled", "全部平台权限。", "All platform permissions.", updatedAt, map[string]string{"capability": "platform", "resource": "*", "action": "*", "prefix": "*"}),
			seedLocalized("permission-admin-all", "admin:*", "全部后台权限", "All Admin Permissions", "enabled", "全部后台管理权限。", "All admin permissions.", updatedAt, map[string]string{"capability": "platform", "resource": "admin", "action": "*", "prefix": "admin"}),
		},
		"menus": {
			seedMenu("menu-overview", "overview", "Overview", "Platform runtime overview entry.", updatedAt, "/overview", "overview", "admin:overview:read", "foundation", "overview", "10", "概览", "Overview", "平台底座运行概览。", "Platform foundation overview."),
			seedMenu("menu-capabilities", "capabilities", "Capabilities", "Capability management entry.", updatedAt, "/capabilities", "capabilities", "admin:capability:read", "foundation", "capabilities", "20", "能力清单", "Capabilities", "查看当前平台启用的能力包。", "View enabled platform capability packages."),
			seedMenu("menu-tenants", "tenants", "Tenants", "Tenant management entry.", updatedAt, "/tenants", "tenants", "admin:tenant:read", "foundation", "tenants", "30", "租户", "Tenants", "租户空间与隔离边界。", "Tenant spaces and isolation boundaries."),
			seedMenu("menu-users", "users", "Users", "User management entry.", updatedAt, "/users", "users", "admin:user:read", "foundation", "users", "40", "用户", "Users", "用户、账号和身份档案。", "Users, accounts, and identity profiles."),
			seedMenu("menu-org-units", "org-units", "Org Units", "Organization management entry.", updatedAt, "/org-units", "org-units", "admin:org-unit:read", "foundation", "cluster", "45", "组织机构", "Org Units", "租户下的集团、公司、分支、部门、团队和门店层级。", "Group, company, branch, department, team, and store hierarchy under tenants.", "identity"),
			seedMenu("menu-role-groups", "role-groups", "Role Groups", "Role group management entry.", updatedAt, "/role-groups", "role-groups", "admin:role-group:read", "foundation", "roles", "48", "角色组", "Role Groups", "角色分类、治理和授权维护。", "Role classification, governance, and authorization maintenance.", "access"),
			seedMenu("menu-roles", "roles", "Roles", "Role management entry.", updatedAt, "/roles", "roles", "admin:role:read", "foundation", "roles", "50", "角色", "Roles", "角色、权限和授权策略。", "Roles, permissions, and authorization policies."),
			seedMenu("menu-menus", "menus", "Menus", "Menu management entry.", updatedAt, "/menus", "menus", "admin:menu:read", "foundation", "menus", "60", "菜单", "Menus", "后台菜单和资源入口。", "Admin menus and resource entries."),
			seedMenu("menu-audit-logs", "audit-logs", "Audit Logs", "Audit logs entry.", updatedAt, "/audit-logs", "audit-logs", "admin:audit-log:read", "governance", "audit", "110", "审计日志", "Audit Logs", "操作审计和日志留痕。", "Operation audit and activity trails."),
			seedMenu("menu-api-resources", "api-resources", "API Resources", "API resource management entry.", updatedAt, "/api-resources", "api-resources", "admin:api-resource:read", "governance", "apiResources", "120", "API 资源", "API Resources", "接口资源、权限码和调用边界。", "API resources, permission codes, and invocation boundaries."),
			seedMenu("menu-dictionaries", "dictionaries", "Dictionaries", "Dictionary management entry.", updatedAt, "/dictionaries", "dictionaries", "admin:dictionary:read", "governance", "dictParams", "128", "字典管理", "Dictionaries", "字典目录、枚举分类和业务选项分组。", "Dictionary catalogs, enum categories, and business option groups."),
			seedMenu("menu-dictionary-parameters", "dictionary-parameters", "Dict & Params", "Dictionary and parameter entry.", updatedAt, "/dictionary-parameters", "dictionary-parameters", "admin:dictionary-parameter:read", "governance", "dictParams", "130", "字典参数", "Dict & Params", "字典、参数和配置项。", "Dictionaries, parameters, and configuration items."),
			seedMenu("menu-parameters", "parameters", "Parameters", "Parameter management entry.", updatedAt, "/parameters", "parameters", "admin:parameter:read", "governance", "dictParams", "132", "参数管理", "Parameters", "平台参数、运行开关和能力配置键值。", "Platform parameters, runtime switches, and capability configuration key-values."),
			seedMenu("menu-area-codes", "area-codes", "Area Codes", "Area code management entry.", updatedAt, "/area-codes", "area-codes", "admin:area-code:read", "governance", "cluster", "135", "地址码", "Area Codes", "通用行政区划或业务区域编码。", "Common administrative or business area codes.", "configuration"),
			seedMenu("menu-monitoring", "monitoring", "Monitoring", "Monitoring entry.", updatedAt, "/monitoring", "monitoring", "admin:monitoring:read", "operations", "monitoring", "210", "监控", "Monitoring", "实例、健康与告警。", "Instances, health, and alerts."),
			seedMenu("menu-branding", "branding", "Branding", "Branding entry.", updatedAt, "/branding", "branding", "admin:branding:read", "foundation", "settings", "308", "品牌配置", "Branding", "平台品牌、主题和登录展示配置。", "Platform branding, theme, and login presentation configuration."),
			seedMenu("menu-settings", "settings", "Settings", "Settings entry.", updatedAt, "/settings", "settings", "admin:settings:read", "security", "settings", "310", "设置", "Settings", "平台配置和品牌设置。", "Platform configuration and branding."),
		},
		"api-resources": {
			seedLocalized("api-capabilities", "GET:/api/capabilities", "能力清单接口", "Capability List API", "enabled", "能力自省接口。", "Capability introspection endpoint.", updatedAt, map[string]string{"method": "GET"}),
			seedLocalized("api-admin-resources", "GET:/api/admin/resources/:resource", "后台资源接口", "Admin Resource API", "enabled", "通用后台资源接口。", "Generic admin resource endpoint.", updatedAt, map[string]string{"method": "GET"}),
		},
		"dictionary-parameters": {
			seedLocalized("dict-capability-kind", "capability-kind", "能力类型", "Capability Kind", "enabled", "核心、插件、可选、停用枚举。", "Core/plugin/optional/disabled enum.", updatedAt, map[string]string{"scope": "platform"}),
			seedLocalized("param-brand-name", "brand.name", "品牌名称", "Brand Name", "enabled", "展示用产品名称。", "Displayed product name.", updatedAt, map[string]string{"scope": "branding"}),
		},
		"dictionaries": {
			seedLocalized("dictionary-platform", "platform", "平台字典", "Platform Dictionary", "enabled", "平台通用枚举和字典分组。", "Shared platform enum and dictionary groups.", updatedAt, map[string]string{"itemCount": "2"}),
		},
		"parameters": {
			seedLocalized("parameter-brand-name", "brand.name", "品牌名称", "Brand Name", "enabled", "展示用产品名称。", "Displayed product name.", updatedAt, map[string]string{"value": "Platform Go", "group": "branding"}),
		},
		"branding": {
			seedLocalized("branding-platform", "branding", "品牌配置", "Branding Settings", "enabled", "平台品牌、主题和登录展示配置。", "Platform branding, theme, and login presentation configuration.", updatedAt, brandingResourceSeedValues()),
		},
		"audit-logs": {
			seed("audit-bootstrap", "platform.bootstrap", "Platform Bootstrap", "recorded", "Initial platform bootstrap event.", updatedAt, map[string]string{"actor": "system", "action": "platform.bootstrap", "resource": "platform", "createdAt": updatedAt}),
		},
		"monitoring": {
			seedLocalized("monitor-api", "platform-api", "平台 API", "Platform API", "healthy", "核心 API 进程健康状态。", "Core API process health.", updatedAt, map[string]string{"targetType": "service"}),
		},
		"settings": {
			seedLocalized("setting-branding", "branding", "品牌设置", "Branding Settings", "enabled", "产品名称、Logo 和主题设置。", "Product name, logo and theme settings.", updatedAt, brandingSeedValues()),
		},
	}
	permissionCodes := []string{
		"admin:api-docs:read",
		"admin:capability:read",
		"admin:demo-data:apply",
		"admin:demo-data:read",
		"admin:monitoring:read",
		"admin:policy-review:export",
	}
	for _, menu := range resources["menus"] {
		permissionCodes = append(permissionCodes, menu.Values["permission"])
	}
	resources["permissions"] = appendPermissionCatalogCodes(permissionCatalogFromSchemas(seedResourceSchemas(), updatedAt), permissionCodes, updatedAt)
	return resources
}

func seedResourcesFromCapabilities(manifests []capability.Manifest) map[string][]Record {
	resources := seedPrincipalResources()
	updatedAt := "2026-07-04T00:00:00Z"
	resources["menus"] = []Record{}
	resources["permissions"] = permissionCatalogFromCapabilities(manifests, updatedAt)
	registered := false
	for _, manifest := range manifests {
		for _, resource := range manifest.Admin.Resources {
			if resource.Resource == "" {
				continue
			}
			registered = true
			if resource.Resource != "users" && resource.Resource != "roles" && resource.Resource != "menus" && resource.Resource != "permissions" {
				resources[resource.Resource] = seedRowsForResource(resource.Resource, updatedAt)
			}
			if menu, ok := menuRecordFromCapability(resource, updatedAt); ok {
				resources["menus"] = append(resources["menus"], menu)
			}
		}
	}
	if !registered {
		return seedResources()
	}
	return resources
}

func seedPrincipalResources() map[string][]Record {
	updatedAt := "2026-07-04T00:00:00Z"
	return map[string][]Record{
		"users": {
			seedLocalized("user-admin", "admin", "平台管理员", "Platform Admin", "enabled", "默认管理员账号。", "Default administrator account.", updatedAt, map[string]string{"roles": "super-admin", "tenantCode": "platform", "orgUnitCode": "platform-hq", "areaCode": "110000"}),
			seedLocalized("user-ops", "ops", "运维用户", "Operations User", "enabled", "用于监控任务的运维账号。", "Operations account for monitoring tasks.", updatedAt, map[string]string{"roles": "operator", "tenantCode": "platform", "orgUnitCode": "platform-ops", "areaCode": "110000"}),
		},
		"roles": {
			seedLocalized("role-super-admin", "super-admin", "超级管理员", "Super Admin", "enabled", "拥有完整平台管理权限的角色。", "Full platform administration role.", updatedAt, map[string]string{"groupCode": "system-admin", "dataScope": "all", "permissions": "*"}),
			seedLocalized("role-platform-admin", "platform-admin", "平台管理员", "Platform Admin", "enabled", "标准平台资源管理角色。", "Standard platform resource management role.", updatedAt, map[string]string{"groupCode": "system-admin", "dataScope": "all", "permissions": "admin:*"}),
			seedLocalized("role-operator", "operator", "运维人员", "Operator", "enabled", "用于平台导航演示的只读运维角色。", "Read-only operations role for platform navigation demos.", updatedAt, map[string]string{"groupCode": "operations", "dataScope": "current_org", "permissions": "admin:overview:read,admin:capability:read,admin:tenant:read,admin:monitoring:read"}),
		},
		"permissions": {
			seedLocalized("permission-all", "*", "全部权限", "All Permissions", "enabled", "全部平台权限。", "All platform permissions.", updatedAt, map[string]string{"capability": "platform", "resource": "*", "action": "*", "prefix": "*"}),
			seedLocalized("permission-admin-all", "admin:*", "全部后台权限", "All Admin Permissions", "enabled", "全部后台管理权限。", "All admin permissions.", updatedAt, map[string]string{"capability": "platform", "resource": "admin", "action": "*", "prefix": "admin"}),
		},
	}
}

func seedRowsForResource(resource string, updatedAt string) []Record {
	switch resource {
	case "overview":
		return []Record{
			seedLocalized("overview-platform", "platform", "平台运行", "Platform Runtime", "healthy", "平台运行概览。", "Platform runtime overview.", updatedAt, map[string]string{"domain": "foundation"}),
		}
	case "tenants":
		return []Record{
			seedLocalized("tenant-platform", "platform", "平台租户", "Platform Tenant", "enabled", "平台管理默认租户。", "Default tenant for platform administration.", updatedAt, map[string]string{"isolation": "shared", "areaCode": "110000"}),
			seedLocalized("tenant-demo", "demo", "演示租户", "Demo Tenant", "enabled", "用于演示和测试数据的可复用租户。", "Reusable tenant for demos and fixtures.", updatedAt, map[string]string{"isolation": "sandbox", "areaCode": "110000"}),
		}
	case "org-units":
		return []Record{
			seedLocalized("org-platform-hq", "platform-hq", "平台总部", "Platform HQ", "enabled", "平台默认顶级机构。", "Default top-level platform organization.", updatedAt, map[string]string{"type": "organization", "tenantCode": "platform", "parentCode": "", "areaCode": "110000", "sortOrder": "10"}),
			seedLocalized("org-platform-ops", "platform-ops", "运维部门", "Operations Department", "enabled", "平台默认运维部门。", "Default platform operations department.", updatedAt, map[string]string{"type": "department", "tenantCode": "platform", "parentCode": "platform-hq", "areaCode": "110000", "sortOrder": "20"}),
		}
	case "role-groups":
		return []Record{
			seedLocalized("role-group-system-admin", "system-admin", "系统管理", "System Admin", "enabled", "系统管理类角色分组。", "Role group for system administration roles.", updatedAt, map[string]string{"sortOrder": "10"}),
			seedLocalized("role-group-operations", "operations", "运维管理", "Operations", "enabled", "运维和监控类角色分组。", "Role group for operations and monitoring roles.", updatedAt, map[string]string{"sortOrder": "20"}),
		}
	case "area-codes":
		return []Record{
			seedLocalized("area-cn", "CN", "中国", "China", "enabled", "国家级区域根节点。", "Country-level area root.", updatedAt, map[string]string{"parentCode": "", "level": "country", "path": "CN", "sortOrder": "10"}),
			seedLocalized("area-beijing", "110000", "北京市", "Beijing", "enabled", "示例省/直辖市地址码。", "Sample province/municipality area code.", updatedAt, map[string]string{"parentCode": "CN", "level": "province", "path": "CN/110000", "sortOrder": "20"}),
		}
	case "api-resources":
		return []Record{
			seedLocalized("api-capabilities", "GET:/api/capabilities", "能力清单接口", "Capability List API", "enabled", "能力自省接口。", "Capability introspection endpoint.", updatedAt, map[string]string{"method": "GET"}),
			seedLocalized("api-admin-resources", "GET:/api/admin/resources/:resource", "后台资源接口", "Admin Resource API", "enabled", "通用后台资源接口。", "Generic admin resource endpoint.", updatedAt, map[string]string{"method": "GET"}),
		}
	case "dictionary-parameters":
		return []Record{
			seedLocalized("dict-capability-kind", "capability-kind", "能力类型", "Capability Kind", "enabled", "核心、插件、可选、停用枚举。", "Core/plugin/optional/disabled enum.", updatedAt, map[string]string{"scope": "platform"}),
			seedLocalized("param-brand-name", "brand.name", "品牌名称", "Brand Name", "enabled", "展示用产品名称。", "Displayed product name.", updatedAt, map[string]string{"scope": "branding"}),
		}
	case "dictionaries":
		return []Record{
			seedLocalized("dictionary-platform", "platform", "平台字典", "Platform Dictionary", "enabled", "平台通用枚举和字典分组。", "Shared platform enum and dictionary groups.", updatedAt, map[string]string{"itemCount": "2"}),
		}
	case "parameters":
		return []Record{
			seedLocalized("parameter-brand-name", "brand.name", "品牌名称", "Brand Name", "enabled", "展示用产品名称。", "Displayed product name.", updatedAt, map[string]string{"value": "Platform Go", "group": "branding"}),
		}
	case "branding":
		return []Record{
			seedLocalized("branding-platform", "branding", "品牌配置", "Branding Settings", "enabled", "平台品牌、主题和登录展示配置。", "Platform branding, theme, and login presentation configuration.", updatedAt, brandingResourceSeedValues()),
		}
	case "audit-logs":
		return []Record{
			seed("audit-bootstrap", "platform.bootstrap", "Platform Bootstrap", "recorded", "Initial platform bootstrap event.", updatedAt, map[string]string{"actor": "system", "action": "platform.bootstrap", "resource": "platform", "createdAt": updatedAt}),
		}
	case "monitoring":
		return []Record{
			seedLocalized("monitor-api", "platform-api", "平台 API", "Platform API", "healthy", "核心 API 进程健康状态。", "Core API process health.", updatedAt, map[string]string{"targetType": "service"}),
		}
	case "settings":
		return []Record{
			seedLocalized("setting-branding", "branding", "品牌设置", "Branding Settings", "enabled", "产品名称、Logo 和主题设置。", "Product name, logo and theme settings.", updatedAt, brandingSeedValues()),
		}
	case "users", "roles":
		return nil
	default:
		return []Record{}
	}
}

func brandingSeedValues() map[string]string {
	return map[string]string{
		"capability":    "branding",
		"productName":   "Platform Go",
		"shortName":     "Platform",
		"logoUrl":       "",
		"faviconUrl":    "",
		"primaryColor":  "#1677ff",
		"defaultTheme":  "tech",
		"loginTitle":    "Platform Go",
		"loginSubtitle": "Reusable operations platform foundation.",
	}
}

func brandingResourceSeedValues() map[string]string {
	values := brandingSeedValues()
	delete(values, "capability")
	return values
}

func seed(id string, code string, name string, status string, description string, updatedAt string, values map[string]string) Record {
	return Record{ID: id, Code: code, Name: name, Status: status, Description: description, UpdatedAt: updatedAt, Values: values}
}

func seedLocalized(id string, code string, nameZH string, nameEN string, status string, descriptionZH string, descriptionEN string, updatedAt string, values map[string]string) Record {
	return seed(id, code, nameEN, status, descriptionEN, updatedAt, withLocalizedValues(values, nameZH, nameEN, descriptionZH, descriptionEN))
}

func withLocalizedValues(values map[string]string, nameZH string, nameEN string, descriptionZH string, descriptionEN string) map[string]string {
	cloned := map[string]string{}
	for key, value := range values {
		cloned[key] = value
	}
	cloned["nameZh"] = nameZH
	cloned["nameEn"] = nameEN
	cloned["descriptionZh"] = descriptionZH
	cloned["descriptionEn"] = descriptionEN
	return cloned
}

func seedMenu(id string, code string, name string, description string, updatedAt string, route string, args ...string) Record {
	resource := valueAt(args, 0)
	permission := valueAt(args, 1)
	group := valueAt(args, 2)
	icon := valueAt(args, 3)
	order := valueAt(args, 4)
	titleZH := valueAt(args, 5)
	titleEN := valueAt(args, 6)
	descriptionZH := valueAt(args, 7)
	descriptionEN := valueAt(args, 8)
	parent := valueAt(args, 9)
	isExternal := valueAt(args, 10)
	cacheEnabled := valueAt(args, 11)
	if parent == "" {
		parent = defaultMenuParent(resource, group)
	}
	if isExternal == "" {
		isExternal = "false"
	}
	if cacheEnabled == "" {
		cacheEnabled = "true"
	}
	return seed(id, code, name, "enabled", description, updatedAt, map[string]string{
		"route":         route,
		"parent":        parent,
		"isExternal":    isExternal,
		"cacheEnabled":  cacheEnabled,
		"resource":      resource,
		"permission":    permission,
		"group":         group,
		"icon":          icon,
		"order":         order,
		"titleZh":       titleZH,
		"titleEn":       titleEN,
		"nameZh":        titleZH,
		"nameEn":        titleEN,
		"descriptionZh": descriptionZH,
		"descriptionEn": descriptionEN,
	})
}

func valueAt(values []string, index int) string {
	if index >= len(values) {
		return ""
	}
	return values[index]
}

func defaultMenuParent(resource string, group string) string {
	switch resource {
	case "overview", "capabilities":
		return "runtime"
	case "tenants", "users":
		return "identity"
	case "roles", "permissions", "menus":
		return "access"
	case "api-resources", "dictionary-parameters":
		return "resources"
	case "audit-logs":
		return "audit"
	case "monitoring", "demo-data":
		return "runtime"
	case "settings":
		return "configuration"
	}
	return group
}
