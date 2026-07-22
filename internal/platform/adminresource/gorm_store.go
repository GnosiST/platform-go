package adminresource

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/GnosiST/platform-go/internal/platform/kernel"
)

type GORMAdminResourceRepository struct {
	db                       *gorm.DB
	organizationRBACOwned    bool
	organizationRBACUsers    OrganizationRBACUserSnapshotWriter
	organizationRBACOrgUnits OrganizationRBACOrgUnitSnapshotWriter
	organizationRBACRoles    OrganizationRBACRoleSnapshotWriter
	organizationRBACGroups   OrganizationRBACRoleGroupSnapshotWriter
	organizationRBACMenus    OrganizationRBACMenuPermissionSnapshotWriter
}

type OrganizationRBACUserSnapshotWriter interface {
	ApplyUserSnapshot(context.Context, *gorm.DB, []Record, []Record) error
}

type OrganizationRBACOrgUnitSnapshotWriter interface {
	ApplyOrgUnitSnapshot(context.Context, *gorm.DB, []Record, []Record) error
}

type OrganizationRBACRoleSnapshotWriter interface {
	ApplyRoleSnapshot(context.Context, *gorm.DB, []Record, []Record) error
}

type OrganizationRBACRoleGroupSnapshotWriter interface {
	ApplyRoleGroupSnapshot(context.Context, *gorm.DB, []Record, []Record) error
}

type OrganizationRBACMenuPermissionSnapshotWriter interface {
	ApplyMenuPermissionSnapshot(context.Context, *gorm.DB, []Record, []Record, []Record, []Record) error
}

var organizationRBACCoreOwnedResources = []string{"users", "org-units", "roles", "role-groups"}
var organizationRBACOwnedResources = organizationRBACCoreOwnedResources
var organizationRBACSnapshotResources = append(append([]string{}, organizationRBACCoreOwnedResources...), "menus", "permissions")

const (
	adminUsersTable             = "platform_admin_users"
	adminUserRolesTable         = "platform_admin_user_roles"
	adminTenantsTable           = "platform_admin_tenants"
	adminOrgUnitsTable          = "platform_admin_org_units"
	adminRolesTable             = "platform_admin_roles"
	adminRoleGroupsTable        = "platform_admin_role_groups"
	adminRolePermissionsTable   = "platform_admin_role_permissions"
	adminPermissionsTable       = "platform_admin_permissions"
	adminMenusTable             = "platform_admin_menus"
	adminAreaCodesTable         = "platform_area_codes"
	adminAuditLogsTable         = "platform_audit_logs"
	adminLoginLogsTable         = "platform_login_logs"
	adminErrorLogsTable         = "platform_error_logs"
	adminVersionsTable          = "platform_versions"
	adminResourceLifecycleTable = "platform_admin_resource_lifecycle"
)

type gormAdminResourceLayout struct {
	Table            string
	ValueProjections map[string][]string
}

var normalizedGORMResourceLayouts = map[string]gormAdminResourceLayout{
	"users": {
		Table: adminUsersTable,
		ValueProjections: map[string][]string{
			"role": {adminUserRolesTable + ".role_code"}, "roles": {adminUserRolesTable + ".role_code"},
			"scopeType": {adminUsersTable + ".scope_type"}, "tenantCode": {adminUsersTable + ".tenant_code"},
			"orgUnitCode": {adminUsersTable + ".org_unit_code"},
		},
	},
	"tenants": {
		Table:            adminTenantsTable,
		ValueProjections: map[string][]string{"areaCode": {adminTenantsTable + ".area_code"}},
	},
	"org-units": {
		Table: adminOrgUnitsTable,
		ValueProjections: map[string][]string{
			"type": {adminOrgUnitsTable + ".type"}, "tenantCode": {adminOrgUnitsTable + ".tenant_code"},
			"parentCode": {adminOrgUnitsTable + ".parent_code"}, "areaCode": {adminOrgUnitsTable + ".area_code"},
			"sortOrder": {adminOrgUnitsTable + ".sort_order"},
		},
	},
	"roles": {
		Table: adminRolesTable,
		ValueProjections: map[string][]string{
			"groupCode": {adminRolesTable + ".group_code"}, "permissions": {adminRolePermissionsTable + ".permission"},
		},
	},
	"role-groups": {
		Table: adminRoleGroupsTable,
		ValueProjections: map[string][]string{
			"parentCode": {adminRoleGroupsTable + ".parent_code"}, "sortOrder": {adminRoleGroupsTable + ".sort_order"},
			"scopeType": {adminRoleGroupsTable + ".scope_type"}, "tenantCode": {adminRoleGroupsTable + ".tenant_code"},
		},
	},
	"permissions": {
		Table: adminPermissionsTable,
		ValueProjections: map[string][]string{
			"capability": {adminPermissionsTable + ".capability"}, "resource": {adminPermissionsTable + ".resource"},
			"action": {adminPermissionsTable + ".action"}, "prefix": {adminPermissionsTable + ".prefix"},
		},
	},
	"menus": {
		Table: adminMenusTable,
		ValueProjections: map[string][]string{
			"nodeType": {adminMenusTable + ".node_type"}, "parentCode": {adminMenusTable + ".parent_code"},
			"route": {adminMenusTable + ".route"}, "parent": {adminMenusTable + ".parent"},
			"componentKey": {adminMenusTable + ".component_key"}, "resourceCode": {adminMenusTable + ".resource_code"},
			"isExternal": {adminMenusTable + ".is_external"}, "cacheEnabled": {adminMenusTable + ".cache_enabled"},
			"externalUrl": {adminMenusTable + ".external_url"}, "openMode": {adminMenusTable + ".open_mode"},
			"parameters": {adminMenusTable + ".parameters_json"}, "hidden": {adminMenusTable + ".hidden"},
			"activeMenuCode": {adminMenusTable + ".active_menu_code"}, "breadcrumbVisible": {adminMenusTable + ".breadcrumb_visible"},
			"resource": {adminMenusTable + ".resource"}, "permission": {adminMenusTable + ".permission"},
			"group": {adminMenusTable + ".group_name"}, "icon": {adminMenusTable + ".icon"},
			"order": {adminMenusTable + ".sort_order"}, "titleZh": {adminMenusTable + ".title_zh"},
			"nameZh": {adminMenusTable + ".title_zh"}, "titleEn": {adminMenusTable + ".title_en"},
			"nameEn": {adminMenusTable + ".title_en"}, "descriptionZh": {adminMenusTable + ".description_zh"},
			"descriptionEn": {adminMenusTable + ".description_en"},
		},
	},
	"area-codes": {
		Table: adminAreaCodesTable,
		ValueProjections: map[string][]string{
			"parentCode": {adminAreaCodesTable + ".parent_code"}, "level": {adminAreaCodesTable + ".level"},
			"depth": {adminAreaCodesTable + ".depth"}, "path": {adminAreaCodesTable + ".path"},
			"sortOrder": {adminAreaCodesTable + ".sort_order"},
		},
	},
	"audit-logs": {
		Table: adminAuditLogsTable,
		ValueProjections: map[string][]string{
			"actor": {adminAuditLogsTable + ".actor"}, "action": {adminAuditLogsTable + ".action"},
			"resource": {adminAuditLogsTable + ".resource"}, "createdAt": {adminAuditLogsTable + ".created_at"},
		},
	},
	"login-logs": {
		Table: adminLoginLogsTable,
		ValueProjections: map[string][]string{
			"username": {adminLoginLogsTable + ".username"}, "provider": {adminLoginLogsTable + ".provider"},
			"ip": {adminLoginLogsTable + ".ip"}, "createdAt": {adminLoginLogsTable + ".created_at"},
		},
	},
	"error-logs": {
		Table: adminErrorLogsTable,
		ValueProjections: map[string][]string{
			"level": {adminErrorLogsTable + ".level"}, "message": {adminErrorLogsTable + ".message"},
			"traceId": {adminErrorLogsTable + ".trace_id"}, "createdAt": {adminErrorLogsTable + ".created_at"},
		},
	},
	"versions": {
		Table: adminVersionsTable,
		ValueProjections: map[string][]string{
			"version": {adminVersionsTable + ".version"}, "channel": {adminVersionsTable + ".channel"},
			"releasedAt": {adminVersionsTable + ".released_at"},
		},
	},
}

type gormAdminResourceRecord struct {
	Resource    string `gorm:"column:resource;primaryKey"`
	ID          string `gorm:"column:id;primaryKey"`
	Code        string `gorm:"column:code;not null"`
	Name        string `gorm:"column:name;not null"`
	Status      string `gorm:"column:status;not null"`
	Description string `gorm:"column:description;not null"`
	UpdatedAt   string `gorm:"column:updated_at;not null"`
	ValuesJSON  string `gorm:"column:values_json;not null"`
}

type gormAdminResourceState struct {
	Key   string `gorm:"column:key;primaryKey"`
	Value string `gorm:"column:value;not null"`
}

type gormAdminResourceLifecycle struct {
	Resource              string `gorm:"column:resource;primaryKey"`
	RecordID              string `gorm:"column:record_id;primaryKey"`
	DeletedAt             string `gorm:"column:deleted_at;index;not null"`
	DeletedBy             string `gorm:"column:deleted_by;not null"`
	DeleteReason          string `gorm:"column:delete_reason;not null"`
	PurgeAfter            string `gorm:"column:purge_after;index;not null"`
	DeletionPolicyVersion uint32 `gorm:"column:deletion_policy_version;not null"`
}

type gormAdminUser struct {
	ID          string `gorm:"column:id;primaryKey"`
	Code        string `gorm:"column:code;uniqueIndex;not null"`
	Name        string `gorm:"column:name;not null"`
	Status      string `gorm:"column:status;not null"`
	Description string `gorm:"column:description;not null"`
	UpdatedAt   string `gorm:"column:updated_at;not null"`
	ScopeType   string `gorm:"column:scope_type;index;not null"`
	TenantCode  string `gorm:"column:tenant_code;index;not null"`
	OrgUnitCode string `gorm:"column:org_unit_code;index;not null"`
	ValuesJSON  string `gorm:"column:values_json;not null"`
}

type gormAdminUserRole struct {
	UserID   string `gorm:"column:user_id;primaryKey"`
	RoleCode string `gorm:"column:role_code;primaryKey"`
}

type gormAdminOrgUnitRoleGroup struct {
	OrgUnitCode   string `gorm:"column:org_unit_code;primaryKey"`
	RoleGroupCode string `gorm:"column:role_group_code;primaryKey"`
}

func (gormAdminOrgUnitRoleGroup) TableName() string { return "platform_admin_org_unit_role_groups" }

type gormAdminTenant struct {
	ID          string `gorm:"column:id;primaryKey"`
	Code        string `gorm:"column:code;uniqueIndex;not null"`
	Name        string `gorm:"column:name;not null"`
	Status      string `gorm:"column:status;not null"`
	Description string `gorm:"column:description;not null"`
	UpdatedAt   string `gorm:"column:updated_at;not null"`
	AreaCode    string `gorm:"column:area_code;index;not null"`
	ValuesJSON  string `gorm:"column:values_json;not null"`
}

type gormAdminOrgUnit struct {
	ID          string `gorm:"column:id;primaryKey"`
	Code        string `gorm:"column:code;uniqueIndex;not null"`
	Name        string `gorm:"column:name;not null"`
	Status      string `gorm:"column:status;not null"`
	Description string `gorm:"column:description;not null"`
	UpdatedAt   string `gorm:"column:updated_at;not null"`
	Type        string `gorm:"column:type;index;not null"`
	TenantCode  string `gorm:"column:tenant_code;index;not null"`
	ParentCode  string `gorm:"column:parent_code;index;not null"`
	AreaCode    string `gorm:"column:area_code;index;not null"`
	SortOrder   int    `gorm:"column:sort_order;index;not null"`
	ValuesJSON  string `gorm:"column:values_json;not null"`
}

type gormAdminRole struct {
	ID          string `gorm:"column:id;primaryKey"`
	Code        string `gorm:"column:code;uniqueIndex;not null"`
	Name        string `gorm:"column:name;not null"`
	Status      string `gorm:"column:status;not null"`
	Description string `gorm:"column:description;not null"`
	UpdatedAt   string `gorm:"column:updated_at;not null"`
	GroupCode   string `gorm:"column:group_code;index;not null"`
	ValuesJSON  string `gorm:"column:values_json;not null"`
}

type gormAdminRoleGroup struct {
	ID          string `gorm:"column:id;primaryKey"`
	Code        string `gorm:"column:code;uniqueIndex;not null"`
	Name        string `gorm:"column:name;not null"`
	Status      string `gorm:"column:status;not null"`
	Description string `gorm:"column:description;not null"`
	UpdatedAt   string `gorm:"column:updated_at;not null"`
	ScopeType   string `gorm:"column:scope_type;index;not null"`
	TenantCode  string `gorm:"column:tenant_code;index;not null"`
	Revision    uint64 `gorm:"column:revision;not null"`
	ParentCode  string `gorm:"column:parent_code;index;not null"`
	SortOrder   int    `gorm:"column:sort_order;index;not null"`
	ValuesJSON  string `gorm:"column:values_json;not null"`
}

type gormAdminRolePermission struct {
	RoleCode   string `gorm:"column:role_code;primaryKey"`
	Permission string `gorm:"column:permission;primaryKey"`
}

type gormAdminPermission struct {
	ID           string `gorm:"column:id;primaryKey"`
	Code         string `gorm:"column:code;uniqueIndex;not null"`
	Name         string `gorm:"column:name;not null"`
	Status       string `gorm:"column:status;not null"`
	Description  string `gorm:"column:description;not null"`
	UpdatedAt    string `gorm:"column:updated_at;not null"`
	Capability   string `gorm:"column:capability;not null"`
	Resource     string `gorm:"column:resource;not null"`
	Action       string `gorm:"column:action;not null"`
	Prefix       string `gorm:"column:prefix;not null"`
	ResourceType string `gorm:"column:resource_type;size:32;not null;default:api"`
	ValuesJSON   string `gorm:"column:values_json;not null"`
}

type gormAdminMenu struct {
	ID                string `gorm:"column:id;primaryKey"`
	Code              string `gorm:"column:code;uniqueIndex;not null"`
	Name              string `gorm:"column:name;not null"`
	Status            string `gorm:"column:status;not null"`
	Description       string `gorm:"column:description;not null"`
	UpdatedAt         string `gorm:"column:updated_at;not null"`
	NodeType          string `gorm:"column:node_type;size:32;index;not null;default:page"`
	ParentCode        string `gorm:"column:parent_code;index;not null;default:''"`
	Route             string `gorm:"column:route;not null"`
	ComponentKey      string `gorm:"column:component_key;not null;default:''"`
	ResourceCode      string `gorm:"column:resource_code;not null;default:''"`
	Parent            string `gorm:"column:parent;not null"`
	IsExternal        bool   `gorm:"column:is_external;not null"`
	ExternalURL       string `gorm:"column:external_url;not null;default:''"`
	OpenMode          string `gorm:"column:open_mode;size:32;not null;default:''"`
	ParametersJSON    string `gorm:"column:parameters_json;type:text;not null;default:'[]'"`
	CacheEnabled      bool   `gorm:"column:cache_enabled;not null"`
	Hidden            bool   `gorm:"column:hidden;not null;default:false"`
	ActiveMenuCode    string `gorm:"column:active_menu_code;not null;default:''"`
	BreadcrumbVisible bool   `gorm:"column:breadcrumb_visible;not null;default:true"`
	Resource          string `gorm:"column:resource;not null"`
	Permission        string `gorm:"column:permission;not null"`
	Group             string `gorm:"column:group_name;not null"`
	Icon              string `gorm:"column:icon;not null"`
	Order             int    `gorm:"column:sort_order;not null"`
	TitleZH           string `gorm:"column:title_zh;not null"`
	TitleEN           string `gorm:"column:title_en;not null"`
	DescriptionZH     string `gorm:"column:description_zh;not null"`
	DescriptionEN     string `gorm:"column:description_en;not null"`
	ValuesJSON        string `gorm:"column:values_json;not null"`
}

type gormAdminPageButton struct {
	MenuCode       string `gorm:"column:menu_code;primaryKey" json:"menuCode"`
	ButtonKey      string `gorm:"column:button_key;primaryKey" json:"buttonKey"`
	LabelZH        string `gorm:"column:label_zh;not null" json:"labelZh"`
	LabelEN        string `gorm:"column:label_en;not null" json:"labelEn"`
	Action         string `gorm:"column:action;not null" json:"action"`
	SortOrder      int    `gorm:"column:sort_order;not null" json:"sortOrder"`
	Status         string `gorm:"column:status;not null" json:"status"`
	PermissionCode string `gorm:"column:permission_code;not null" json:"permissionCode"`
}

type gormAdminAreaCode struct {
	ID          string `gorm:"column:id;primaryKey"`
	Code        string `gorm:"column:code;uniqueIndex;not null"`
	Name        string `gorm:"column:name;not null"`
	Status      string `gorm:"column:status;not null"`
	Description string `gorm:"column:description;not null"`
	UpdatedAt   string `gorm:"column:updated_at;not null"`
	ParentCode  string `gorm:"column:parent_code;index;not null"`
	Level       string `gorm:"column:level;index;not null"`
	Depth       int    `gorm:"column:depth;index;not null;default:0"`
	Path        string `gorm:"column:path;index;not null"`
	SortOrder   int    `gorm:"column:sort_order;index;not null"`
	ValuesJSON  string `gorm:"column:values_json;not null"`
}

type gormAdminAuditLog struct {
	ID          string  `gorm:"column:id;primaryKey"`
	Code        string  `gorm:"column:code;uniqueIndex;not null"`
	Name        string  `gorm:"column:name;not null"`
	Status      string  `gorm:"column:status;not null"`
	Description string  `gorm:"column:description;not null"`
	UpdatedAt   string  `gorm:"column:updated_at;not null"`
	Actor       string  `gorm:"column:actor;index;not null"`
	Action      string  `gorm:"column:action;index;not null"`
	Resource    string  `gorm:"column:resource;index;not null"`
	CreatedAt   string  `gorm:"column:created_at;index;not null"`
	RequestID   *string `gorm:"column:request_id;index"`
	TraceID     *string `gorm:"column:trace_id;index"`
	ValuesJSON  string  `gorm:"column:values_json;not null"`
}

type gormAdminLoginLog struct {
	ID          string `gorm:"column:id;primaryKey"`
	Code        string `gorm:"column:code;uniqueIndex;not null"`
	Name        string `gorm:"column:name;not null"`
	Status      string `gorm:"column:status;not null"`
	Description string `gorm:"column:description;not null"`
	UpdatedAt   string `gorm:"column:updated_at;not null"`
	Username    string `gorm:"column:username;index;not null"`
	Provider    string `gorm:"column:provider;index;not null"`
	IP          string `gorm:"column:ip;index;not null"`
	CreatedAt   string `gorm:"column:created_at;index;not null"`
	ValuesJSON  string `gorm:"column:values_json;not null"`
}

type gormAdminErrorLog struct {
	ID          string `gorm:"column:id;primaryKey"`
	Code        string `gorm:"column:code;uniqueIndex;not null"`
	Name        string `gorm:"column:name;not null"`
	Status      string `gorm:"column:status;not null"`
	Description string `gorm:"column:description;not null"`
	UpdatedAt   string `gorm:"column:updated_at;not null"`
	Level       string `gorm:"column:level;index;not null"`
	Message     string `gorm:"column:message;not null"`
	TraceID     string `gorm:"column:trace_id;index;not null"`
	CreatedAt   string `gorm:"column:created_at;index;not null"`
	ValuesJSON  string `gorm:"column:values_json;not null"`
}

type gormAdminVersion struct {
	ID          string `gorm:"column:id;primaryKey"`
	Code        string `gorm:"column:code;uniqueIndex;not null"`
	Name        string `gorm:"column:name;not null"`
	Status      string `gorm:"column:status;index;not null"`
	Description string `gorm:"column:description;not null"`
	UpdatedAt   string `gorm:"column:updated_at;not null"`
	Version     string `gorm:"column:version;index;not null"`
	Channel     string `gorm:"column:channel;index;not null"`
	ReleasedAt  string `gorm:"column:released_at;index;not null"`
	ValuesJSON  string `gorm:"column:values_json;not null"`
}

func NewGORMAdminResourceRepository(ctx context.Context, db *gorm.DB) (*GORMAdminResourceRepository, error) {
	if ctx == nil || db == nil {
		return nil, errors.New("gorm admin resource repository is unavailable")
	}
	repository := &GORMAdminResourceRepository{db: db}
	if err := db.WithContext(ctx).AutoMigrate(gormAdminResourceModels()...); err != nil {
		return nil, err
	}
	if err := db.WithContext(ctx).Model(&gormAdminPermission{}).
		Where("resource_type = '' OR resource_type IS NULL").Update("resource_type", "api").Error; err != nil {
		return nil, err
	}
	return repository, nil
}

func (r *GORMAdminResourceRepository) Close() error {
	if r == nil || r.db == nil {
		return nil
	}
	db, err := r.db.DB()
	if err != nil {
		return err
	}
	return db.Close()
}

func OpenGORMAdminResourceRepository(ctx context.Context, db *gorm.DB) (*GORMAdminResourceRepository, error) {
	if ctx == nil || db == nil {
		return nil, errors.New("gorm admin resource repository is unavailable")
	}
	for _, model := range gormAdminResourceModels() {
		if !db.WithContext(ctx).Migrator().HasTable(model) {
			return nil, errors.New("gorm admin resource schema is not prepared")
		}
	}
	if !db.WithContext(ctx).Migrator().HasColumn(&gormAdminPermission{}, "ResourceType") {
		return nil, errors.New("gorm admin resource schema is not prepared")
	}
	if !db.WithContext(ctx).Migrator().HasColumn(&gormAdminAreaCode{}, "Depth") {
		return nil, errors.New("gorm admin resource schema is not prepared")
	}
	return &GORMAdminResourceRepository{db: db}, nil
}

func (r *GORMAdminResourceRepository) WithOrganizationRBACOwnership(userWriter ...OrganizationRBACUserSnapshotWriter) *GORMAdminResourceRepository {
	if r == nil {
		return nil
	}
	clone := *r
	clone.organizationRBACOwned = true
	if len(userWriter) > 0 {
		clone.organizationRBACUsers = userWriter[0]
	}
	return &clone
}

func (r *GORMAdminResourceRepository) WithOrganizationRBACOrgUnitWriter(writer OrganizationRBACOrgUnitSnapshotWriter) *GORMAdminResourceRepository {
	if r == nil {
		return nil
	}
	clone := *r
	clone.organizationRBACOrgUnits = writer
	return &clone
}

func (r *GORMAdminResourceRepository) WithOrganizationRBACRoleWriters(roleWriter OrganizationRBACRoleSnapshotWriter, groupWriter OrganizationRBACRoleGroupSnapshotWriter) *GORMAdminResourceRepository {
	if r == nil {
		return nil
	}
	clone := *r
	clone.organizationRBACRoles = roleWriter
	clone.organizationRBACGroups = groupWriter
	return &clone
}

func (r *GORMAdminResourceRepository) WithOrganizationRBACMenuPermissionWriter(writer OrganizationRBACMenuPermissionSnapshotWriter) *GORMAdminResourceRepository {
	if r == nil {
		return nil
	}
	clone := *r
	clone.organizationRBACMenus = writer
	return &clone
}

func (r *GORMAdminResourceRepository) ExcludeCapabilitySeed(resource string) bool {
	return r != nil && r.organizationRBACOwned && slices.Contains(organizationRBACCoreOwnedResources, resource)
}

func gormAdminResourceModels() []any {
	return []any{
		&gormAdminResourceRecord{},
		&gormAdminResourceState{},
		&gormAdminResourceLifecycle{},
		&gormAdminUser{},
		&gormAdminUserRole{},
		&gormAdminTenant{},
		&gormAdminOrgUnit{},
		&gormAdminRole{},
		&gormAdminRoleGroup{},
		&gormAdminRolePermission{},
		&gormAdminPermission{},
		&gormAdminMenu{},
		&gormAdminAreaCode{},
		&gormAdminAuditLog{},
		&gormAdminLoginLog{},
		&gormAdminErrorLog{},
		&gormAdminVersion{},
	}
}

func (gormAdminResourceRecord) TableName() string {
	return adminResourceRecordsTable
}

func (gormAdminResourceState) TableName() string {
	return adminResourceStateTable
}

func (gormAdminResourceLifecycle) TableName() string {
	return adminResourceLifecycleTable
}

func (gormAdminUser) TableName() string {
	return adminUsersTable
}

func (gormAdminUserRole) TableName() string {
	return adminUserRolesTable
}

func (gormAdminTenant) TableName() string {
	return adminTenantsTable
}

func (gormAdminOrgUnit) TableName() string {
	return adminOrgUnitsTable
}

func (gormAdminRole) TableName() string {
	return adminRolesTable
}

func (gormAdminRoleGroup) TableName() string {
	return adminRoleGroupsTable
}

func (gormAdminRolePermission) TableName() string {
	return adminRolePermissionsTable
}

func (gormAdminPermission) TableName() string {
	return adminPermissionsTable
}

func (gormAdminMenu) TableName() string {
	return adminMenusTable
}

func (gormAdminPageButton) TableName() string { return "platform_admin_page_buttons" }

func (gormAdminAreaCode) TableName() string {
	return adminAreaCodesTable
}

func (gormAdminAuditLog) TableName() string {
	return adminAuditLogsTable
}

func (gormAdminLoginLog) TableName() string {
	return adminLoginLogsTable
}

func (gormAdminErrorLog) TableName() string {
	return adminErrorLogsTable
}

func (gormAdminVersion) TableName() string {
	return adminVersionsTable
}

func (r *GORMAdminResourceRepository) Load(ctx context.Context) (ResourceSnapshot, error) {
	revision, err := loadGORMRevision(r.db.WithContext(ctx))
	if err != nil {
		return ResourceSnapshot{}, err
	}
	snapshot := ResourceSnapshot{Revision: revision, Resources: map[string][]Record{}}
	var state gormAdminResourceState
	err = r.db.WithContext(ctx).Where("key = ?", "next_id").First(&state).Error
	if err == nil {
		if nextID, parseErr := strconv.Atoi(state.Value); parseErr == nil {
			snapshot.NextID = nextID
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return ResourceSnapshot{}, err
	}

	var rows []gormAdminResourceRecord
	if err := r.db.WithContext(ctx).Order("resource, id").Find(&rows).Error; err != nil {
		return ResourceSnapshot{}, err
	}
	for _, row := range rows {
		record := Record{
			ID:          row.ID,
			Code:        row.Code,
			Name:        row.Name,
			Status:      row.Status,
			Description: row.Description,
			UpdatedAt:   row.UpdatedAt,
		}
		if row.ValuesJSON != "" {
			if err := json.Unmarshal([]byte(row.ValuesJSON), &record.Values); err != nil {
				return ResourceSnapshot{}, err
			}
		}
		snapshot.Resources[row.Resource] = append(snapshot.Resources[row.Resource], record)
	}
	if err := r.loadNormalizedResources(ctx, &snapshot); err != nil {
		return ResourceSnapshot{}, err
	}
	if err := r.loadLifecycle(ctx, &snapshot); err != nil {
		return ResourceSnapshot{}, err
	}
	actual, err := loadGORMRevision(r.db.WithContext(ctx))
	if err != nil {
		return ResourceSnapshot{}, err
	}
	if actual != revision {
		return ResourceSnapshot{}, &RevisionConflictError{Expected: revision, Actual: actual}
	}
	return snapshot, nil
}

func (r *GORMAdminResourceRepository) CurrentRevision(ctx context.Context) (uint64, error) {
	return loadGORMRevision(r.db.WithContext(ctx))
}

func (r *GORMAdminResourceRepository) Save(ctx context.Context, snapshot ResourceSnapshot) (uint64, error) {
	var committed uint64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var organizationRBACUsersChanged bool
		var organizationRBACOrgUnitsChanged bool
		var organizationRBACRolesChanged bool
		var organizationRBACGroupsChanged bool
		var organizationRBACMenusChanged bool
		var organizationRBACPermissionsChanged bool
		if r.organizationRBACOwned {
			current := ResourceSnapshot{Resources: map[string][]Record{}}
			txRepository := &GORMAdminResourceRepository{db: tx, organizationRBACOwned: true}
			if err := txRepository.loadOrganizationRBACOwnedResources(ctx, &current); err != nil {
				return err
			}
			if err := txRepository.loadLifecycleForResources(ctx, &current, organizationRBACSnapshotResources); err != nil {
				return err
			}
			for _, resource := range organizationRBACSnapshotResources {
				if !equalOrganizationRBACProjection(resource, current.Resources[resource], snapshot.Resources[resource]) {
					switch resource {
					case "users":
						if r.organizationRBACUsers == nil {
							return fmt.Errorf("%w: %s", ErrDomainOwnedMutation, resource)
						}
						organizationRBACUsersChanged = true
					case "org-units":
						if r.organizationRBACOrgUnits == nil {
							return fmt.Errorf("%w: %s", ErrDomainOwnedMutation, resource)
						}
						organizationRBACOrgUnitsChanged = true
					case "roles":
						if r.organizationRBACRoles == nil {
							return fmt.Errorf("%w: %s", ErrDomainOwnedMutation, resource)
						}
						organizationRBACRolesChanged = true
					case "role-groups":
						if r.organizationRBACGroups == nil {
							return fmt.Errorf("%w: %s", ErrDomainOwnedMutation, resource)
						}
						organizationRBACGroupsChanged = true
					case "menus":
						if r.organizationRBACMenus == nil {
							return fmt.Errorf("%w: %s", ErrDomainOwnedMutation, resource)
						}
						organizationRBACMenusChanged = true
					case "permissions":
						if r.organizationRBACMenus == nil {
							return fmt.Errorf("%w: %s", ErrDomainOwnedMutation, resource)
						}
						organizationRBACPermissionsChanged = true
					default:
						return fmt.Errorf("%w: %s", ErrDomainOwnedMutation, resource)
					}
				}
			}
			if organizationRBACUsersChanged {
				if err := r.organizationRBACUsers.ApplyUserSnapshot(ctx, tx, current.Resources["users"], snapshot.Resources["users"]); err != nil {
					return err
				}
			}
			if organizationRBACOrgUnitsChanged {
				if err := r.organizationRBACOrgUnits.ApplyOrgUnitSnapshot(ctx, tx, current.Resources["org-units"], snapshot.Resources["org-units"]); err != nil {
					return err
				}
			}
			if organizationRBACGroupsChanged {
				if err := r.organizationRBACGroups.ApplyRoleGroupSnapshot(ctx, tx, current.Resources["role-groups"], snapshot.Resources["role-groups"]); err != nil {
					return err
				}
			}
			if organizationRBACRolesChanged {
				if err := r.organizationRBACRoles.ApplyRoleSnapshot(ctx, tx, current.Resources["roles"], snapshot.Resources["roles"]); err != nil {
					return err
				}
			}
			if organizationRBACMenusChanged || organizationRBACPermissionsChanged {
				if err := r.organizationRBACMenus.ApplyMenuPermissionSnapshot(
					ctx, tx,
					current.Resources["menus"], snapshot.Resources["menus"],
					current.Resources["permissions"], snapshot.Resources["permissions"],
				); err != nil {
					return err
				}
			}
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&gormAdminResourceState{Key: "revision", Value: "0"}).Error; err != nil {
			return err
		}
		expected := strconv.FormatUint(snapshot.Revision, 10)
		nextRevision := snapshot.Revision + 1
		result := tx.Model(&gormAdminResourceState{}).
			Where("key = ? AND value = ?", "revision", expected).
			Update("value", strconv.FormatUint(nextRevision, 10))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			actual, err := loadGORMRevision(tx)
			if err != nil {
				return err
			}
			return &RevisionConflictError{Expected: snapshot.Revision, Actual: actual}
		}
		committed = nextRevision

		deleteAll := tx.Session(&gorm.Session{AllowGlobalUpdate: true})
		deleteModels := []interface{}{
			&gormAdminResourceRecord{},
			&gormAdminTenant{},
			&gormAdminAreaCode{},
			&gormAdminAuditLog{},
			&gormAdminLoginLog{},
			&gormAdminErrorLog{},
			&gormAdminVersion{},
		}
		if !r.organizationRBACOwned {
			deleteModels = append(deleteModels,
				&gormAdminUserRole{}, &gormAdminRolePermission{}, &gormAdminUser{},
				&gormAdminOrgUnit{}, &gormAdminRole{}, &gormAdminRoleGroup{}, &gormAdminPermission{}, &gormAdminMenu{},
			)
		}
		for _, model := range deleteModels {
			if err := deleteAll.Delete(model).Error; err != nil {
				return err
			}
		}
		lifecycleDelete := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Model(&gormAdminResourceLifecycle{})
		if r.organizationRBACOwned {
			lifecycleDelete = lifecycleDelete.Where("resource NOT IN ?", organizationRBACSnapshotResources)
		}
		if err := lifecycleDelete.Delete(&gormAdminResourceLifecycle{}).Error; err != nil {
			return err
		}
		if err := tx.Where("key = ?", "next_id").Delete(&gormAdminResourceState{}).Error; err != nil {
			return err
		}
		if err := tx.Create(&gormAdminResourceState{Key: "next_id", Value: strconv.Itoa(snapshot.NextID)}).Error; err != nil {
			return err
		}
		if err := saveNormalizedResources(tx, snapshot, r.organizationRBACOwned); err != nil {
			return err
		}
		rows := make([]gormAdminResourceRecord, 0)
		for resource, records := range snapshot.Resources {
			if isNormalizedResource(resource) {
				continue
			}
			for _, record := range records {
				valuesJSON, err := json.Marshal(cloneValues(record.Values))
				if err != nil {
					return err
				}
				rows = append(rows, gormAdminResourceRecord{
					Resource:    resource,
					ID:          record.ID,
					Code:        record.Code,
					Name:        record.Name,
					Status:      record.Status,
					Description: record.Description,
					UpdatedAt:   record.UpdatedAt,
					ValuesJSON:  string(valuesJSON),
				})
			}
		}
		if len(rows) > 0 {
			if err := tx.Create(&rows).Error; err != nil {
				return err
			}
		}
		return saveLifecycleRows(tx, snapshot, r.organizationRBACOwned)
	})
	if err != nil {
		return 0, err
	}
	return committed, nil
}

func equalOrganizationRBACProjection(resource string, current []Record, proposed []Record) bool {
	return reflect.DeepEqual(canonicalOrganizationRBACRecords(resource, current), canonicalOrganizationRBACRecords(resource, proposed))
}

func canonicalOrganizationRBACRecords(resource string, records []Record) []Record {
	canonical := cloneRecords(records)
	for index := range canonical {
		values := canonical[index].Values
		for key, value := range values {
			if strings.TrimSpace(value) == "" {
				delete(values, key)
			}
		}
		if resource == "users" {
			delete(values, "role")
		}
		canonical[index].Values = emptyValuesToNil(values)
	}
	sort.Slice(canonical, func(left int, right int) bool { return canonical[left].ID < canonical[right].ID })
	return canonical
}

func (r *GORMAdminResourceRepository) loadOrganizationRBACOwnedResources(ctx context.Context, snapshot *ResourceSnapshot) error {
	loaders := []struct {
		resource string
		load     func(context.Context) ([]Record, error)
	}{
		{resource: "users", load: r.loadUsers},
		{resource: "org-units", load: r.loadOrgUnits},
		{resource: "roles", load: r.loadRoles},
		{resource: "role-groups", load: r.loadRoleGroups},
		{resource: "menus", load: r.loadMenus},
		{resource: "permissions", load: r.loadPermissions},
	}
	for _, loader := range loaders {
		records, err := loader.load(ctx)
		if err != nil {
			return err
		}
		if records != nil {
			snapshot.Resources[loader.resource] = records
		}
	}
	return nil
}

func (r *GORMAdminResourceRepository) loadLifecycle(ctx context.Context, snapshot *ResourceSnapshot) error {
	return r.loadLifecycleForResources(ctx, snapshot, nil)
}

func (r *GORMAdminResourceRepository) loadLifecycleForResources(ctx context.Context, snapshot *ResourceSnapshot, resources []string) error {
	var rows []gormAdminResourceLifecycle
	query := r.db.WithContext(ctx).Order("resource, record_id")
	if len(resources) > 0 {
		query = query.Where("resource IN ?", resources)
	}
	if err := query.Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		records := snapshot.Resources[row.Resource]
		index := recordIndexByID(records, row.RecordID)
		if index < 0 {
			return fmt.Errorf("lifecycle metadata references missing record %s/%s", row.Resource, row.RecordID)
		}
		records[index].DeletedAt = row.DeletedAt
		records[index].DeletedBy = row.DeletedBy
		records[index].DeleteReason = row.DeleteReason
		records[index].PurgeAfter = row.PurgeAfter
		records[index].DeletionPolicyVersion = row.DeletionPolicyVersion
		snapshot.Resources[row.Resource] = records
	}
	return nil
}

func saveLifecycleRows(tx *gorm.DB, snapshot ResourceSnapshot, organizationRBACOwned bool) error {
	rows := make([]gormAdminResourceLifecycle, 0)
	for resource, records := range snapshot.Resources {
		if organizationRBACOwned && slices.Contains(organizationRBACSnapshotResources, resource) {
			continue
		}
		for _, record := range records {
			if !isLifecycleDeleted(record) {
				continue
			}
			rows = append(rows, gormAdminResourceLifecycle{
				Resource: resource, RecordID: record.ID, DeletedAt: record.DeletedAt, DeletedBy: record.DeletedBy,
				DeleteReason: record.DeleteReason, PurgeAfter: record.PurgeAfter, DeletionPolicyVersion: record.DeletionPolicyVersion,
			})
		}
	}
	if len(rows) == 0 {
		return nil
	}
	return tx.Create(&rows).Error
}

func loadGORMRevision(db *gorm.DB) (uint64, error) {
	var state gormAdminResourceState
	err := db.Where("key = ?", "revision").First(&state).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	revision, err := strconv.ParseUint(state.Value, 10, 64)
	if err != nil {
		return 0, err
	}
	return revision, nil
}

func (r *GORMAdminResourceRepository) loadNormalizedResources(ctx context.Context, snapshot *ResourceSnapshot) error {
	users, err := r.loadUsers(ctx)
	if err != nil {
		return err
	}
	if users != nil {
		snapshot.Resources["users"] = users
	}
	tenants, err := r.loadTenants(ctx)
	if err != nil {
		return err
	}
	if tenants != nil {
		snapshot.Resources["tenants"] = tenants
	}
	orgUnits, err := r.loadOrgUnits(ctx)
	if err != nil {
		return err
	}
	if orgUnits != nil {
		snapshot.Resources["org-units"] = orgUnits
	}
	roles, err := r.loadRoles(ctx)
	if err != nil {
		return err
	}
	if roles != nil {
		snapshot.Resources["roles"] = roles
	}
	roleGroups, err := r.loadRoleGroups(ctx)
	if err != nil {
		return err
	}
	if roleGroups != nil {
		snapshot.Resources["role-groups"] = roleGroups
	}
	permissions, err := r.loadPermissions(ctx)
	if err != nil {
		return err
	}
	if permissions != nil {
		snapshot.Resources["permissions"] = permissions
	}
	menus, err := r.loadMenus(ctx)
	if err != nil {
		return err
	}
	if menus != nil {
		snapshot.Resources["menus"] = menus
	}
	areaCodes, err := r.loadAreaCodes(ctx)
	if err != nil {
		return err
	}
	if areaCodes != nil {
		snapshot.Resources["area-codes"] = areaCodes
	}
	auditLogs, err := r.loadAuditLogs(ctx)
	if err != nil {
		return err
	}
	if auditLogs != nil {
		snapshot.Resources["audit-logs"] = auditLogs
	}
	loginLogs, err := r.loadLoginLogs(ctx)
	if err != nil {
		return err
	}
	if loginLogs != nil {
		snapshot.Resources["login-logs"] = loginLogs
	}
	errorLogs, err := r.loadErrorLogs(ctx)
	if err != nil {
		return err
	}
	if errorLogs != nil {
		snapshot.Resources["error-logs"] = errorLogs
	}
	versions, err := r.loadVersions(ctx)
	if err != nil {
		return err
	}
	if versions != nil {
		snapshot.Resources["versions"] = versions
	}
	return nil
}

func (r *GORMAdminResourceRepository) loadUsers(ctx context.Context) ([]Record, error) {
	var rows []gormAdminUser
	if err := r.db.WithContext(ctx).Order("id").Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	var roleRows []gormAdminUserRole
	if err := r.db.WithContext(ctx).Order("user_id, role_code").Find(&roleRows).Error; err != nil {
		return nil, err
	}
	rolesByUser := map[string][]string{}
	for _, row := range roleRows {
		rolesByUser[row.UserID] = append(rolesByUser[row.UserID], row.RoleCode)
	}
	records := make([]Record, 0, len(rows))
	for _, row := range rows {
		values, err := valuesFromJSON(row.ValuesJSON)
		if err != nil {
			return nil, err
		}
		roleValue := strings.Join(rolesByUser[row.ID], ",")
		values["scopeType"] = row.ScopeType
		values["tenantCode"] = row.TenantCode
		values["orgUnitCode"] = row.OrgUnitCode
		values["roles"] = roleValue
		records = append(records, Record{
			ID:          row.ID,
			Code:        row.Code,
			Name:        row.Name,
			Status:      row.Status,
			Description: row.Description,
			UpdatedAt:   row.UpdatedAt,
			Values:      emptyValuesToNil(values),
		})
	}
	return records, nil
}

func (r *GORMAdminResourceRepository) loadTenants(ctx context.Context) ([]Record, error) {
	var rows []gormAdminTenant
	if err := r.db.WithContext(ctx).Order("id").Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	records := make([]Record, 0, len(rows))
	for _, row := range rows {
		values, err := valuesFromJSON(row.ValuesJSON)
		if err != nil {
			return nil, err
		}
		values["areaCode"] = row.AreaCode
		records = append(records, recordFromNormalized(row.ID, row.Code, row.Name, row.Status, row.Description, row.UpdatedAt, values))
	}
	return records, nil
}

func (r *GORMAdminResourceRepository) loadOrgUnits(ctx context.Context) ([]Record, error) {
	var rows []gormAdminOrgUnit
	if err := r.db.WithContext(ctx).Order("sort_order, id").Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	groupsByOrganization := map[string][]string{}
	if r.db.WithContext(ctx).Migrator().HasTable(&gormAdminOrgUnitRoleGroup{}) {
		var bindings []gormAdminOrgUnitRoleGroup
		if err := r.db.WithContext(ctx).Order("org_unit_code, role_group_code").Find(&bindings).Error; err != nil {
			return nil, err
		}
		for _, binding := range bindings {
			groupsByOrganization[binding.OrgUnitCode] = append(groupsByOrganization[binding.OrgUnitCode], binding.RoleGroupCode)
		}
	}
	var groupRows []gormAdminRoleGroup
	if err := r.db.WithContext(ctx).Find(&groupRows).Error; err != nil {
		return nil, err
	}
	var roleRows []gormAdminRole
	if err := r.db.WithContext(ctx).Find(&roleRows).Error; err != nil {
		return nil, err
	}
	var lifecycleRows []gormAdminResourceLifecycle
	if err := r.db.WithContext(ctx).
		Where("resource IN ?", []string{"org-units", "role-groups", "roles"}).
		Find(&lifecycleRows).Error; err != nil {
		return nil, err
	}
	deleted := make(map[string]map[string]struct{})
	for _, lifecycle := range lifecycleRows {
		if deleted[lifecycle.Resource] == nil {
			deleted[lifecycle.Resource] = map[string]struct{}{}
		}
		deleted[lifecycle.Resource][lifecycle.RecordID] = struct{}{}
	}
	groupsByCode := make(map[string]gormAdminRoleGroup, len(groupRows))
	for _, group := range groupRows {
		groupsByCode[group.Code] = group
	}
	rolesByGroup := make(map[string][]gormAdminRole)
	for _, role := range roleRows {
		rolesByGroup[role.GroupCode] = append(rolesByGroup[role.GroupCode], role)
	}
	records := make([]Record, 0, len(rows))
	for _, row := range rows {
		values, err := valuesFromJSON(row.ValuesJSON)
		if err != nil {
			return nil, err
		}
		values["type"] = row.Type
		values["tenantCode"] = row.TenantCode
		values["parentCode"] = row.ParentCode
		values["areaCode"] = row.AreaCode
		values["sortOrder"] = strconv.Itoa(row.SortOrder)
		if roleGroups := groupsByOrganization[row.Code]; len(roleGroups) > 0 {
			values["roleGroupCodes"] = strings.Join(roleGroups, ",")
		}
		roleGroupCodes := groupsByOrganization[row.Code]
		values["roleGroupCount"] = strconv.Itoa(len(roleGroupCodes))
		effectiveRoles := map[string]struct{}{}
		if row.Status == "enabled" {
			if _, orgDeleted := deleted["org-units"][row.ID]; !orgDeleted {
				for _, groupCode := range roleGroupCodes {
					group, exists := groupsByCode[groupCode]
					if !exists || group.Status != "enabled" || group.ScopeType != "tenant" || group.TenantCode != row.TenantCode {
						continue
					}
					if _, groupDeleted := deleted["role-groups"][group.ID]; groupDeleted {
						continue
					}
					for _, role := range rolesByGroup[groupCode] {
						if role.Status != "enabled" {
							continue
						}
						if _, roleDeleted := deleted["roles"][role.ID]; !roleDeleted {
							effectiveRoles[role.Code] = struct{}{}
						}
					}
				}
			}
		}
		values["effectiveRoleCount"] = strconv.Itoa(len(effectiveRoles))
		records = append(records, recordFromNormalized(row.ID, row.Code, row.Name, row.Status, row.Description, row.UpdatedAt, values))
	}
	return records, nil
}

func (r *GORMAdminResourceRepository) loadRoles(ctx context.Context) ([]Record, error) {
	var rows []gormAdminRole
	if err := r.db.WithContext(ctx).Order("id").Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	var permissionRows []gormAdminRolePermission
	if err := r.db.WithContext(ctx).Order("role_code, permission").Find(&permissionRows).Error; err != nil {
		return nil, err
	}
	permissionsByRole := map[string][]string{}
	for _, row := range permissionRows {
		permissionsByRole[row.RoleCode] = append(permissionsByRole[row.RoleCode], row.Permission)
	}
	records := make([]Record, 0, len(rows))
	for _, row := range rows {
		values, err := valuesFromJSON(row.ValuesJSON)
		if err != nil {
			return nil, err
		}
		values["permissions"] = strings.Join(permissionsByRole[row.Code], ",")
		values["groupCode"] = row.GroupCode
		records = append(records, Record{
			ID:          row.ID,
			Code:        row.Code,
			Name:        row.Name,
			Status:      row.Status,
			Description: row.Description,
			UpdatedAt:   row.UpdatedAt,
			Values:      emptyValuesToNil(values),
		})
	}
	return records, nil
}

func (r *GORMAdminResourceRepository) loadRoleGroups(ctx context.Context) ([]Record, error) {
	var rows []gormAdminRoleGroup
	if err := r.db.WithContext(ctx).Order("sort_order, id").Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	records := make([]Record, 0, len(rows))
	for _, row := range rows {
		values, err := valuesFromJSON(row.ValuesJSON)
		if err != nil {
			return nil, err
		}
		values["parentCode"] = row.ParentCode
		values["sortOrder"] = strconv.Itoa(row.SortOrder)
		values["scopeType"] = row.ScopeType
		values["tenantCode"] = row.TenantCode
		records = append(records, recordFromNormalized(row.ID, row.Code, row.Name, row.Status, row.Description, row.UpdatedAt, values))
	}
	return records, nil
}

func (r *GORMAdminResourceRepository) loadPermissions(ctx context.Context) ([]Record, error) {
	var rows []gormAdminPermission
	if err := r.db.WithContext(ctx).Order("id").Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	records := make([]Record, 0, len(rows))
	for _, row := range rows {
		values, err := valuesFromJSON(row.ValuesJSON)
		if err != nil {
			return nil, err
		}
		values["capability"] = row.Capability
		values["resource"] = row.Resource
		values["action"] = row.Action
		values["prefix"] = row.Prefix
		values["resourceType"] = row.ResourceType
		records = append(records, Record{
			ID:          row.ID,
			Code:        row.Code,
			Name:        row.Name,
			Status:      row.Status,
			Description: row.Description,
			UpdatedAt:   row.UpdatedAt,
			Values:      emptyValuesToNil(values),
		})
	}
	return records, nil
}

func (r *GORMAdminResourceRepository) loadMenus(ctx context.Context) ([]Record, error) {
	var rows []gormAdminMenu
	if err := r.db.WithContext(ctx).Order("sort_order, id").Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	buttonsByMenu := map[string][]gormAdminPageButton{}
	if r.db.WithContext(ctx).Migrator().HasTable(&gormAdminPageButton{}) {
		var buttons []gormAdminPageButton
		if err := r.db.WithContext(ctx).Order("menu_code, sort_order, button_key").Find(&buttons).Error; err != nil {
			return nil, err
		}
		for _, button := range buttons {
			buttonsByMenu[button.MenuCode] = append(buttonsByMenu[button.MenuCode], button)
		}
	}
	records := make([]Record, 0, len(rows))
	for _, row := range rows {
		values, err := valuesFromJSON(row.ValuesJSON)
		if err != nil {
			return nil, err
		}
		if _, ok := values["nodeType"]; ok || row.NodeType != "page" || r.organizationRBACOwned {
			values["nodeType"] = row.NodeType
		}
		if _, ok := values["parentCode"]; ok || r.organizationRBACOwned {
			values["parentCode"] = row.ParentCode
		}
		values["route"] = row.Route
		values["parent"] = row.Parent
		if _, ok := values["componentKey"]; ok || r.organizationRBACOwned {
			values["componentKey"] = row.ComponentKey
		}
		if _, ok := values["resourceCode"]; ok || r.organizationRBACOwned {
			values["resourceCode"] = row.ResourceCode
		}
		if _, ok := values["isExternal"]; ok || row.IsExternal || r.organizationRBACOwned {
			values["isExternal"] = boolString(row.IsExternal)
		}
		if _, ok := values["cacheEnabled"]; ok || !row.CacheEnabled || r.organizationRBACOwned {
			values["cacheEnabled"] = boolString(row.CacheEnabled)
		}
		if _, ok := values["externalUrl"]; ok || row.ExternalURL != "" || r.organizationRBACOwned {
			values["externalUrl"] = row.ExternalURL
		}
		if _, ok := values["openMode"]; ok || row.OpenMode != "" || r.organizationRBACOwned {
			values["openMode"] = row.OpenMode
		}
		if _, ok := values["parameters"]; ok || row.ParametersJSON != "[]" || r.organizationRBACOwned {
			values["parameters"] = row.ParametersJSON
		}
		if _, ok := values["hidden"]; ok || row.Hidden || r.organizationRBACOwned {
			values["hidden"] = boolString(row.Hidden)
		}
		if _, ok := values["activeMenuCode"]; ok || row.ActiveMenuCode != "" || r.organizationRBACOwned {
			values["activeMenuCode"] = row.ActiveMenuCode
		}
		if _, ok := values["breadcrumbVisible"]; ok || !row.BreadcrumbVisible || r.organizationRBACOwned {
			values["breadcrumbVisible"] = boolString(row.BreadcrumbVisible)
		}
		values["resource"] = row.Resource
		values["permission"] = row.Permission
		values["group"] = row.Group
		values["icon"] = row.Icon
		values["order"] = strconv.Itoa(row.Order)
		values["titleZh"] = row.TitleZH
		values["titleEn"] = row.TitleEN
		values["descriptionZh"] = row.DescriptionZH
		values["descriptionEn"] = row.DescriptionEN
		if buttons := buttonsByMenu[row.Code]; len(buttons) > 0 {
			encoded, err := json.Marshal(buttons)
			if err != nil {
				return nil, err
			}
			values["pageButtons"] = string(encoded)
		}
		records = append(records, Record{
			ID:          row.ID,
			Code:        row.Code,
			Name:        row.Name,
			Status:      row.Status,
			Description: row.Description,
			UpdatedAt:   row.UpdatedAt,
			Values:      emptyValuesToNil(values),
		})
	}
	return records, nil
}

func (r *GORMAdminResourceRepository) loadAreaCodes(ctx context.Context) ([]Record, error) {
	var rows []gormAdminAreaCode
	if err := r.db.WithContext(ctx).Order("sort_order, id").Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	records := make([]Record, 0, len(rows))
	for _, row := range rows {
		values, err := valuesFromJSON(row.ValuesJSON)
		if err != nil {
			return nil, err
		}
		values["parentCode"] = row.ParentCode
		values["level"] = row.Level
		values["depth"] = strconv.Itoa(areaDepthWithFallback(row.Depth, row.Path))
		values["path"] = row.Path
		values["sortOrder"] = strconv.Itoa(row.SortOrder)
		records = append(records, recordFromNormalized(row.ID, row.Code, row.Name, row.Status, row.Description, row.UpdatedAt, values))
	}
	return records, nil
}

func (r *GORMAdminResourceRepository) loadAuditLogs(ctx context.Context) ([]Record, error) {
	var rows []gormAdminAuditLog
	if err := r.db.WithContext(ctx).Order("created_at DESC, id").Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	records := make([]Record, 0, len(rows))
	for _, row := range rows {
		values, err := valuesFromJSON(row.ValuesJSON)
		if err != nil {
			return nil, err
		}
		jsonTraceID := strings.TrimSpace(values["traceId"])
		if row.RequestID != nil {
			values["requestId"] = strings.TrimSpace(*row.RequestID)
		}
		if row.TraceID != nil {
			canonicalTraceID := strings.TrimSpace(*row.TraceID)
			if jsonTraceID != "" && jsonTraceID != canonicalTraceID && strings.TrimSpace(values["legacyTraceId"]) == "" {
				values["legacyTraceId"] = jsonTraceID
			}
			values["traceId"] = canonicalTraceID
		}
		values["actor"] = row.Actor
		values["action"] = row.Action
		values["resource"] = row.Resource
		values["createdAt"] = row.CreatedAt
		record, _ := normalizeAuditCorrelationRecord(recordFromNormalized(row.ID, row.Code, row.Name, row.Status, row.Description, row.UpdatedAt, values))
		records = append(records, record)
	}
	return records, nil
}

func (r *GORMAdminResourceRepository) loadLoginLogs(ctx context.Context) ([]Record, error) {
	var rows []gormAdminLoginLog
	if err := r.db.WithContext(ctx).Order("created_at DESC, id").Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	records := make([]Record, 0, len(rows))
	for _, row := range rows {
		values, err := valuesFromJSON(row.ValuesJSON)
		if err != nil {
			return nil, err
		}
		values["username"] = row.Username
		values["provider"] = row.Provider
		values["ip"] = row.IP
		values["createdAt"] = row.CreatedAt
		records = append(records, recordFromNormalized(row.ID, row.Code, row.Name, row.Status, row.Description, row.UpdatedAt, values))
	}
	return records, nil
}

func (r *GORMAdminResourceRepository) loadErrorLogs(ctx context.Context) ([]Record, error) {
	var rows []gormAdminErrorLog
	if err := r.db.WithContext(ctx).Order("created_at DESC, id").Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	records := make([]Record, 0, len(rows))
	for _, row := range rows {
		values, err := valuesFromJSON(row.ValuesJSON)
		if err != nil {
			return nil, err
		}
		values["level"] = row.Level
		values["message"] = row.Message
		values["traceId"] = row.TraceID
		values["createdAt"] = row.CreatedAt
		records = append(records, recordFromNormalized(row.ID, row.Code, row.Name, row.Status, row.Description, row.UpdatedAt, values))
	}
	return records, nil
}

func (r *GORMAdminResourceRepository) loadVersions(ctx context.Context) ([]Record, error) {
	var rows []gormAdminVersion
	if err := r.db.WithContext(ctx).Order("released_at DESC, id").Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	records := make([]Record, 0, len(rows))
	for _, row := range rows {
		values, err := valuesFromJSON(row.ValuesJSON)
		if err != nil {
			return nil, err
		}
		values["version"] = row.Version
		values["channel"] = row.Channel
		values["releasedAt"] = row.ReleasedAt
		records = append(records, recordFromNormalized(row.ID, row.Code, row.Name, row.Status, row.Description, row.UpdatedAt, values))
	}
	return records, nil
}

func saveNormalizedResources(tx *gorm.DB, snapshot ResourceSnapshot, organizationRBACOwned bool) error {
	if !organizationRBACOwned {
		if err := saveUsers(tx, snapshot.Resources["users"]); err != nil {
			return err
		}
	}
	if err := saveTenants(tx, snapshot.Resources["tenants"]); err != nil {
		return err
	}
	if !organizationRBACOwned {
		if err := saveOrgUnits(tx, snapshot.Resources["org-units"]); err != nil {
			return err
		}
		if err := saveRoles(tx, snapshot.Resources["roles"]); err != nil {
			return err
		}
		if err := saveRoleGroups(tx, snapshot.Resources["role-groups"]); err != nil {
			return err
		}
	}
	if !organizationRBACOwned {
		if err := savePermissions(tx, snapshot.Resources["permissions"]); err != nil {
			return err
		}
		if err := saveMenus(tx, snapshot.Resources["menus"]); err != nil {
			return err
		}
	}
	if err := saveAreaCodes(tx, snapshot.Resources["area-codes"]); err != nil {
		return err
	}
	if err := saveAuditLogs(tx, snapshot.Resources["audit-logs"]); err != nil {
		return err
	}
	if err := saveLoginLogs(tx, snapshot.Resources["login-logs"]); err != nil {
		return err
	}
	if err := saveErrorLogs(tx, snapshot.Resources["error-logs"]); err != nil {
		return err
	}
	return saveVersions(tx, snapshot.Resources["versions"])
}

func saveUsers(tx *gorm.DB, records []Record) error {
	rows := make([]gormAdminUser, 0, len(records))
	roleRows := make([]gormAdminUserRole, 0)
	for _, record := range records {
		valuesJSON, err := marshalRecordValues(record)
		if err != nil {
			return err
		}
		rows = append(rows, gormAdminUser{
			ID:          record.ID,
			Code:        record.Code,
			Name:        record.Name,
			Status:      record.Status,
			Description: record.Description,
			UpdatedAt:   record.UpdatedAt,
			ScopeType:   record.Values["scopeType"],
			TenantCode:  record.Values["tenantCode"],
			OrgUnitCode: record.Values["orgUnitCode"],
			ValuesJSON:  valuesJSON,
		})
		for _, role := range roleValuesFromUser(record) {
			roleRows = append(roleRows, gormAdminUserRole{UserID: record.ID, RoleCode: role})
		}
	}
	if len(rows) > 0 {
		if err := tx.Create(&rows).Error; err != nil {
			return err
		}
	}
	if len(roleRows) > 0 {
		return tx.Create(&roleRows).Error
	}
	return nil
}

func saveTenants(tx *gorm.DB, records []Record) error {
	rows := make([]gormAdminTenant, 0, len(records))
	for _, record := range records {
		valuesJSON, err := marshalRecordValues(record)
		if err != nil {
			return err
		}
		rows = append(rows, gormAdminTenant{
			ID:          record.ID,
			Code:        record.Code,
			Name:        record.Name,
			Status:      record.Status,
			Description: record.Description,
			UpdatedAt:   record.UpdatedAt,
			AreaCode:    record.Values["areaCode"],
			ValuesJSON:  valuesJSON,
		})
	}
	if len(rows) == 0 {
		return nil
	}
	return tx.Create(&rows).Error
}

func saveOrgUnits(tx *gorm.DB, records []Record) error {
	rows := make([]gormAdminOrgUnit, 0, len(records))
	for _, record := range records {
		valuesJSON, err := marshalRecordValues(record)
		if err != nil {
			return err
		}
		rows = append(rows, gormAdminOrgUnit{
			ID:          record.ID,
			Code:        record.Code,
			Name:        record.Name,
			Status:      record.Status,
			Description: record.Description,
			UpdatedAt:   record.UpdatedAt,
			Type:        record.Values["type"],
			TenantCode:  record.Values["tenantCode"],
			ParentCode:  record.Values["parentCode"],
			AreaCode:    record.Values["areaCode"],
			SortOrder:   parseOrder(record.Values["sortOrder"]),
			ValuesJSON:  valuesJSON,
		})
	}
	if len(rows) == 0 {
		return nil
	}
	return tx.Create(&rows).Error
}

func saveRoles(tx *gorm.DB, records []Record) error {
	rows := make([]gormAdminRole, 0, len(records))
	permissionRows := make([]gormAdminRolePermission, 0)
	for _, record := range records {
		valuesJSON, err := marshalRecordValues(record)
		if err != nil {
			return err
		}
		rows = append(rows, gormAdminRole{
			ID:          record.ID,
			Code:        record.Code,
			Name:        record.Name,
			Status:      record.Status,
			Description: record.Description,
			UpdatedAt:   record.UpdatedAt,
			GroupCode:   record.Values["groupCode"],
			ValuesJSON:  valuesJSON,
		})
		for _, permission := range parseCSV(record.Values["permissions"]) {
			permissionRows = append(permissionRows, gormAdminRolePermission{RoleCode: record.Code, Permission: permission})
		}
	}
	if len(rows) > 0 {
		if err := tx.Create(&rows).Error; err != nil {
			return err
		}
	}
	if len(permissionRows) > 0 {
		return tx.Create(&permissionRows).Error
	}
	return nil
}

func saveRoleGroups(tx *gorm.DB, records []Record) error {
	rows := make([]gormAdminRoleGroup, 0, len(records))
	for _, record := range records {
		valuesJSON, err := marshalRecordValues(record)
		if err != nil {
			return err
		}
		rows = append(rows, gormAdminRoleGroup{
			ID:          record.ID,
			Code:        record.Code,
			Name:        record.Name,
			Status:      record.Status,
			Description: record.Description,
			UpdatedAt:   record.UpdatedAt,
			ScopeType:   record.Values["scopeType"],
			TenantCode:  record.Values["tenantCode"],
			ParentCode:  record.Values["parentCode"],
			SortOrder:   parseOrder(record.Values["sortOrder"]),
			ValuesJSON:  valuesJSON,
		})
	}
	if len(rows) == 0 {
		return nil
	}
	return tx.Create(&rows).Error
}

func savePermissions(tx *gorm.DB, records []Record) error {
	rows := make([]gormAdminPermission, 0, len(records))
	for _, record := range records {
		valuesJSON, err := marshalRecordValues(record)
		if err != nil {
			return err
		}
		resourceType := strings.TrimSpace(record.Values["resourceType"])
		if resourceType == "" {
			resourceType = "api"
		}
		if resourceType != "api" && resourceType != "page-button" {
			return errors.New("permission resource type is invalid")
		}
		rows = append(rows, gormAdminPermission{
			ID:           record.ID,
			Code:         record.Code,
			Name:         record.Name,
			Status:       record.Status,
			Description:  record.Description,
			UpdatedAt:    record.UpdatedAt,
			Capability:   record.Values["capability"],
			Resource:     record.Values["resource"],
			Action:       record.Values["action"],
			Prefix:       record.Values["prefix"],
			ResourceType: resourceType,
			ValuesJSON:   valuesJSON,
		})
	}
	if len(rows) == 0 {
		return nil
	}
	return tx.Create(&rows).Error
}

func saveMenus(tx *gorm.DB, records []Record) error {
	rows := make([]gormAdminMenu, 0, len(records))
	for _, record := range records {
		valuesJSON, err := marshalRecordValues(record)
		if err != nil {
			return err
		}
		rows = append(rows, gormAdminMenu{
			ID:                record.ID,
			Code:              record.Code,
			Name:              record.Name,
			Status:            record.Status,
			Description:       record.Description,
			UpdatedAt:         record.UpdatedAt,
			NodeType:          valueWithFallback(record.Values["nodeType"], "page"),
			ParentCode:        valueWithFallback(record.Values["parentCode"], record.Values["parent"]),
			Route:             record.Values["route"],
			ComponentKey:      valueWithFallback(record.Values["componentKey"], record.Values["resource"]),
			ResourceCode:      valueWithFallback(record.Values["resourceCode"], record.Values["resource"]),
			Parent:            record.Values["parent"],
			IsExternal:        parseBool(record.Values["isExternal"]),
			ExternalURL:       record.Values["externalUrl"],
			OpenMode:          record.Values["openMode"],
			ParametersJSON:    valueWithFallback(record.Values["parameters"], "[]"),
			CacheEnabled:      parseBoolDefault(record.Values["cacheEnabled"], true),
			Hidden:            parseBool(record.Values["hidden"]),
			ActiveMenuCode:    record.Values["activeMenuCode"],
			BreadcrumbVisible: parseBoolDefault(record.Values["breadcrumbVisible"], true),
			Resource:          record.Values["resource"],
			Permission:        record.Values["permission"],
			Group:             record.Values["group"],
			Icon:              record.Values["icon"],
			Order:             parseOrder(record.Values["order"]),
			TitleZH:           valueWithFallback(record.Values["titleZh"], record.Values["nameZh"]),
			TitleEN:           valueWithFallback(record.Values["titleEn"], record.Values["nameEn"]),
			DescriptionZH:     valueWithFallback(record.Values["descriptionZh"], record.Description),
			DescriptionEN:     valueWithFallback(record.Values["descriptionEn"], record.Description),
			ValuesJSON:        valuesJSON,
		})
	}
	if len(rows) == 0 {
		return nil
	}
	return tx.Create(&rows).Error
}

func saveAreaCodes(tx *gorm.DB, records []Record) error {
	rows := make([]gormAdminAreaCode, 0, len(records))
	for _, record := range records {
		valuesJSON, err := marshalRecordValues(record)
		if err != nil {
			return err
		}
		rows = append(rows, gormAdminAreaCode{
			ID:          record.ID,
			Code:        record.Code,
			Name:        record.Name,
			Status:      record.Status,
			Description: record.Description,
			UpdatedAt:   record.UpdatedAt,
			ParentCode:  record.Values["parentCode"],
			Level:       record.Values["level"],
			Depth:       parseAreaDepth(record.Values["depth"], record.Values["path"]),
			Path:        record.Values["path"],
			SortOrder:   parseOrder(record.Values["sortOrder"]),
			ValuesJSON:  valuesJSON,
		})
	}
	if len(rows) == 0 {
		return nil
	}
	return tx.Create(&rows).Error
}

func saveAuditLogs(tx *gorm.DB, records []Record) error {
	rows := make([]gormAdminAuditLog, 0, len(records))
	for _, record := range records {
		record, _ = normalizeAuditCorrelationRecord(record)
		valuesJSON, err := marshalRecordValues(record)
		if err != nil {
			return err
		}
		var requestID *string
		var traceID *string
		correlation := kernel.Correlation{RequestID: record.Values["requestId"], TraceID: record.Values["traceId"]}
		if kernel.ValidCorrelation(correlation) {
			requestID = &correlation.RequestID
			traceID = &correlation.TraceID
		}
		rows = append(rows, gormAdminAuditLog{
			ID:          record.ID,
			Code:        record.Code,
			Name:        record.Name,
			Status:      record.Status,
			Description: record.Description,
			UpdatedAt:   record.UpdatedAt,
			Actor:       record.Values["actor"],
			Action:      record.Values["action"],
			Resource:    record.Values["resource"],
			CreatedAt:   record.Values["createdAt"],
			RequestID:   requestID,
			TraceID:     traceID,
			ValuesJSON:  valuesJSON,
		})
	}
	if len(rows) == 0 {
		return nil
	}
	return tx.Create(&rows).Error
}

func saveLoginLogs(tx *gorm.DB, records []Record) error {
	rows := make([]gormAdminLoginLog, 0, len(records))
	for _, record := range records {
		valuesJSON, err := marshalRecordValues(record)
		if err != nil {
			return err
		}
		rows = append(rows, gormAdminLoginLog{
			ID:          record.ID,
			Code:        record.Code,
			Name:        record.Name,
			Status:      record.Status,
			Description: record.Description,
			UpdatedAt:   record.UpdatedAt,
			Username:    record.Values["username"],
			Provider:    record.Values["provider"],
			IP:          record.Values["ip"],
			CreatedAt:   record.Values["createdAt"],
			ValuesJSON:  valuesJSON,
		})
	}
	if len(rows) == 0 {
		return nil
	}
	return tx.Create(&rows).Error
}

func saveErrorLogs(tx *gorm.DB, records []Record) error {
	rows := make([]gormAdminErrorLog, 0, len(records))
	for _, record := range records {
		valuesJSON, err := marshalRecordValues(record)
		if err != nil {
			return err
		}
		rows = append(rows, gormAdminErrorLog{
			ID:          record.ID,
			Code:        record.Code,
			Name:        record.Name,
			Status:      record.Status,
			Description: record.Description,
			UpdatedAt:   record.UpdatedAt,
			Level:       record.Values["level"],
			Message:     record.Values["message"],
			TraceID:     record.Values["traceId"],
			CreatedAt:   record.Values["createdAt"],
			ValuesJSON:  valuesJSON,
		})
	}
	if len(rows) == 0 {
		return nil
	}
	return tx.Create(&rows).Error
}

func saveVersions(tx *gorm.DB, records []Record) error {
	rows := make([]gormAdminVersion, 0, len(records))
	for _, record := range records {
		valuesJSON, err := marshalRecordValues(record)
		if err != nil {
			return err
		}
		rows = append(rows, gormAdminVersion{
			ID:          record.ID,
			Code:        record.Code,
			Name:        record.Name,
			Status:      record.Status,
			Description: record.Description,
			UpdatedAt:   record.UpdatedAt,
			Version:     record.Values["version"],
			Channel:     record.Values["channel"],
			ReleasedAt:  record.Values["releasedAt"],
			ValuesJSON:  valuesJSON,
		})
	}
	if len(rows) == 0 {
		return nil
	}
	return tx.Create(&rows).Error
}

func recordFromNormalized(id string, code string, name string, status string, description string, updatedAt string, values map[string]string) Record {
	return Record{
		ID:          id,
		Code:        code,
		Name:        name,
		Status:      status,
		Description: description,
		UpdatedAt:   updatedAt,
		Values:      emptyValuesToNil(values),
	}
}

func marshalRecordValues(record Record) (string, error) {
	valuesJSON, err := json.Marshal(cloneValues(record.Values))
	if err != nil {
		return "", err
	}
	return string(valuesJSON), nil
}

func valuesFromJSON(raw string) (map[string]string, error) {
	values := map[string]string{}
	if strings.TrimSpace(raw) == "" {
		return values, nil
	}
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil, err
	}
	if values == nil {
		values = map[string]string{}
	}
	return values, nil
}

func parseAreaDepth(value string, path string) int {
	if depth, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && depth >= 0 {
		return depth
	}
	return areaDepthFromPath(path)
}

func areaDepthWithFallback(depth int, path string) int {
	if depth > 0 {
		return depth
	}
	return areaDepthFromPath(path)
}

func areaDepthFromPath(path string) int {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	depth := 0
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			depth++
		}
	}
	return depth
}

func emptyValuesToNil(values map[string]string) map[string]string {
	for key, value := range values {
		if strings.TrimSpace(value) == "" {
			delete(values, key)
		}
	}
	if len(values) == 0 {
		return nil
	}
	return values
}

func isNormalizedResource(resource string) bool {
	_, ok := normalizedGORMResourceLayouts[resource]
	return ok
}

func parseCSV(value string) []string {
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			items = append(items, item)
		}
	}
	return items
}

func valueWithFallback(value string, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}
