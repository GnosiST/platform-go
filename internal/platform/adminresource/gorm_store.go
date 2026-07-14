package adminresource

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GORMAdminResourceRepository struct {
	db *gorm.DB
}

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
		Table:            adminRolesTable,
		ValueProjections: map[string][]string{"permissions": {adminRolePermissionsTable + ".permission"}},
	},
	"role-groups": {
		Table: adminRoleGroupsTable,
		ValueProjections: map[string][]string{
			"parentCode": {adminRoleGroupsTable + ".parent_code"}, "sortOrder": {adminRoleGroupsTable + ".sort_order"},
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
			"route": {adminMenusTable + ".route"}, "parent": {adminMenusTable + ".parent"},
			"isExternal": {adminMenusTable + ".is_external"}, "cacheEnabled": {adminMenusTable + ".cache_enabled"},
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
			"path": {adminAreaCodesTable + ".path"}, "sortOrder": {adminAreaCodesTable + ".sort_order"},
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
	ValuesJSON  string `gorm:"column:values_json;not null"`
}

type gormAdminUserRole struct {
	UserID   string `gorm:"column:user_id;primaryKey"`
	RoleCode string `gorm:"column:role_code;primaryKey"`
}

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
	ValuesJSON  string `gorm:"column:values_json;not null"`
}

type gormAdminRoleGroup struct {
	ID          string `gorm:"column:id;primaryKey"`
	Code        string `gorm:"column:code;uniqueIndex;not null"`
	Name        string `gorm:"column:name;not null"`
	Status      string `gorm:"column:status;not null"`
	Description string `gorm:"column:description;not null"`
	UpdatedAt   string `gorm:"column:updated_at;not null"`
	ParentCode  string `gorm:"column:parent_code;index;not null"`
	SortOrder   int    `gorm:"column:sort_order;index;not null"`
	ValuesJSON  string `gorm:"column:values_json;not null"`
}

type gormAdminRolePermission struct {
	RoleCode   string `gorm:"column:role_code;primaryKey"`
	Permission string `gorm:"column:permission;primaryKey"`
}

type gormAdminPermission struct {
	ID          string `gorm:"column:id;primaryKey"`
	Code        string `gorm:"column:code;uniqueIndex;not null"`
	Name        string `gorm:"column:name;not null"`
	Status      string `gorm:"column:status;not null"`
	Description string `gorm:"column:description;not null"`
	UpdatedAt   string `gorm:"column:updated_at;not null"`
	Capability  string `gorm:"column:capability;not null"`
	Resource    string `gorm:"column:resource;not null"`
	Action      string `gorm:"column:action;not null"`
	Prefix      string `gorm:"column:prefix;not null"`
	ValuesJSON  string `gorm:"column:values_json;not null"`
}

type gormAdminMenu struct {
	ID            string `gorm:"column:id;primaryKey"`
	Code          string `gorm:"column:code;uniqueIndex;not null"`
	Name          string `gorm:"column:name;not null"`
	Status        string `gorm:"column:status;not null"`
	Description   string `gorm:"column:description;not null"`
	UpdatedAt     string `gorm:"column:updated_at;not null"`
	Route         string `gorm:"column:route;not null"`
	Parent        string `gorm:"column:parent;not null"`
	IsExternal    bool   `gorm:"column:is_external;not null"`
	CacheEnabled  bool   `gorm:"column:cache_enabled;not null"`
	Resource      string `gorm:"column:resource;not null"`
	Permission    string `gorm:"column:permission;not null"`
	Group         string `gorm:"column:group_name;not null"`
	Icon          string `gorm:"column:icon;not null"`
	Order         int    `gorm:"column:sort_order;not null"`
	TitleZH       string `gorm:"column:title_zh;not null"`
	TitleEN       string `gorm:"column:title_en;not null"`
	DescriptionZH string `gorm:"column:description_zh;not null"`
	DescriptionEN string `gorm:"column:description_en;not null"`
	ValuesJSON    string `gorm:"column:values_json;not null"`
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
	Path        string `gorm:"column:path;index;not null"`
	SortOrder   int    `gorm:"column:sort_order;index;not null"`
	ValuesJSON  string `gorm:"column:values_json;not null"`
}

type gormAdminAuditLog struct {
	ID          string `gorm:"column:id;primaryKey"`
	Code        string `gorm:"column:code;uniqueIndex;not null"`
	Name        string `gorm:"column:name;not null"`
	Status      string `gorm:"column:status;not null"`
	Description string `gorm:"column:description;not null"`
	UpdatedAt   string `gorm:"column:updated_at;not null"`
	Actor       string `gorm:"column:actor;index;not null"`
	Action      string `gorm:"column:action;index;not null"`
	Resource    string `gorm:"column:resource;index;not null"`
	CreatedAt   string `gorm:"column:created_at;index;not null"`
	ValuesJSON  string `gorm:"column:values_json;not null"`
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
	return repository, nil
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
	return &GORMAdminResourceRepository{db: db}, nil
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
		for _, model := range []interface{}{
			&gormAdminResourceRecord{},
			&gormAdminResourceLifecycle{},
			&gormAdminUserRole{},
			&gormAdminRolePermission{},
			&gormAdminUser{},
			&gormAdminTenant{},
			&gormAdminOrgUnit{},
			&gormAdminRole{},
			&gormAdminRoleGroup{},
			&gormAdminPermission{},
			&gormAdminMenu{},
			&gormAdminAreaCode{},
			&gormAdminAuditLog{},
			&gormAdminLoginLog{},
			&gormAdminErrorLog{},
			&gormAdminVersion{},
		} {
			if err := deleteAll.Delete(model).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("key = ?", "next_id").Delete(&gormAdminResourceState{}).Error; err != nil {
			return err
		}
		if err := tx.Create(&gormAdminResourceState{Key: "next_id", Value: strconv.Itoa(snapshot.NextID)}).Error; err != nil {
			return err
		}
		if err := saveNormalizedResources(tx, snapshot); err != nil {
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
		return saveLifecycleRows(tx, snapshot)
	})
	if err != nil {
		return 0, err
	}
	return committed, nil
}

func (r *GORMAdminResourceRepository) loadLifecycle(ctx context.Context, snapshot *ResourceSnapshot) error {
	var rows []gormAdminResourceLifecycle
	if err := r.db.WithContext(ctx).Order("resource, record_id").Find(&rows).Error; err != nil {
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

func saveLifecycleRows(tx *gorm.DB, snapshot ResourceSnapshot) error {
	rows := make([]gormAdminResourceLifecycle, 0)
	for resource, records := range snapshot.Resources {
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
		values["roles"] = roleValue
		if strings.TrimSpace(values["role"]) == "" {
			values["role"] = roleValue
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
	records := make([]Record, 0, len(rows))
	for _, row := range rows {
		values, err := valuesFromJSON(row.ValuesJSON)
		if err != nil {
			return nil, err
		}
		values["route"] = row.Route
		values["parent"] = row.Parent
		if _, ok := values["isExternal"]; ok || row.IsExternal {
			values["isExternal"] = boolString(row.IsExternal)
		}
		if _, ok := values["cacheEnabled"]; ok || !row.CacheEnabled {
			values["cacheEnabled"] = boolString(row.CacheEnabled)
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
		values["actor"] = row.Actor
		values["action"] = row.Action
		values["resource"] = row.Resource
		values["createdAt"] = row.CreatedAt
		records = append(records, recordFromNormalized(row.ID, row.Code, row.Name, row.Status, row.Description, row.UpdatedAt, values))
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

func saveNormalizedResources(tx *gorm.DB, snapshot ResourceSnapshot) error {
	if err := saveUsers(tx, snapshot.Resources["users"]); err != nil {
		return err
	}
	if err := saveTenants(tx, snapshot.Resources["tenants"]); err != nil {
		return err
	}
	if err := saveOrgUnits(tx, snapshot.Resources["org-units"]); err != nil {
		return err
	}
	if err := saveRoles(tx, snapshot.Resources["roles"]); err != nil {
		return err
	}
	if err := saveRoleGroups(tx, snapshot.Resources["role-groups"]); err != nil {
		return err
	}
	if err := savePermissions(tx, snapshot.Resources["permissions"]); err != nil {
		return err
	}
	if err := saveMenus(tx, snapshot.Resources["menus"]); err != nil {
		return err
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
		rows = append(rows, gormAdminPermission{
			ID:          record.ID,
			Code:        record.Code,
			Name:        record.Name,
			Status:      record.Status,
			Description: record.Description,
			UpdatedAt:   record.UpdatedAt,
			Capability:  record.Values["capability"],
			Resource:    record.Values["resource"],
			Action:      record.Values["action"],
			Prefix:      record.Values["prefix"],
			ValuesJSON:  valuesJSON,
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
			ID:            record.ID,
			Code:          record.Code,
			Name:          record.Name,
			Status:        record.Status,
			Description:   record.Description,
			UpdatedAt:     record.UpdatedAt,
			Route:         record.Values["route"],
			Parent:        record.Values["parent"],
			IsExternal:    parseBool(record.Values["isExternal"]),
			CacheEnabled:  parseBoolDefault(record.Values["cacheEnabled"], true),
			Resource:      record.Values["resource"],
			Permission:    record.Values["permission"],
			Group:         record.Values["group"],
			Icon:          record.Values["icon"],
			Order:         parseOrder(record.Values["order"]),
			TitleZH:       valueWithFallback(record.Values["titleZh"], record.Values["nameZh"]),
			TitleEN:       valueWithFallback(record.Values["titleEn"], record.Values["nameEn"]),
			DescriptionZH: valueWithFallback(record.Values["descriptionZh"], record.Description),
			DescriptionEN: valueWithFallback(record.Values["descriptionEn"], record.Description),
			ValuesJSON:    valuesJSON,
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
		valuesJSON, err := marshalRecordValues(record)
		if err != nil {
			return err
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
