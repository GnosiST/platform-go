package organizationrbac

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	organizationsTable             = "platform_admin_org_units"
	roleGroupsTable                = "platform_admin_role_groups"
	rolesTable                     = "platform_admin_roles"
	usersTable                     = "platform_admin_users"
	userRolesTable                 = "platform_admin_user_roles"
	permissionsTable               = "platform_admin_permissions"
	rolePermissionsTable           = "platform_admin_role_permissions"
	resourceLifecycleTable         = "platform_admin_resource_lifecycle"
	orgUnitRoleGroupsTable         = "platform_admin_org_unit_role_groups"
	orgUnitRoleGroupRevisionsTable = "platform_admin_org_unit_role_group_revisions"
	adminResourceStateTable        = "platform_admin_resource_state"
)

type GORMRepository struct {
	db *gorm.DB
}

type gormOrganization struct {
	ID         string `gorm:"column:id;size:191;primaryKey"`
	Code       string `gorm:"column:code;size:191;uniqueIndex"`
	TenantCode string `gorm:"column:tenant_code;size:191;index"`
	Status     string `gorm:"column:status;size:32;index"`
	ValuesJSON string `gorm:"column:values_json;type:text"`
}

type gormRoleGroup struct {
	ID         string `gorm:"column:id;size:191;primaryKey"`
	Code       string `gorm:"column:code;size:191;uniqueIndex"`
	Name       string `gorm:"column:name;size:191"`
	ScopeType  string `gorm:"column:scope_type;size:32;index"`
	TenantCode string `gorm:"column:tenant_code;size:191;index"`
	Status     string `gorm:"column:status;size:32;index"`
	Revision   uint64 `gorm:"column:revision"`
	ParentCode string `gorm:"column:parent_code"`
	ValuesJSON string `gorm:"column:values_json;type:text"`
}

type gormRole struct {
	ID         string `gorm:"column:id;size:191;primaryKey"`
	Code       string `gorm:"column:code;size:191;uniqueIndex"`
	Name       string `gorm:"column:name;size:191"`
	GroupCode  string `gorm:"column:group_code;size:191;index"`
	Status     string `gorm:"column:status;size:32;index"`
	UpdatedAt  string `gorm:"column:updated_at;size:35"`
	ValuesJSON string `gorm:"column:values_json;type:text"`
}

type gormUser struct {
	ID          string `gorm:"column:id;size:191;primaryKey"`
	Code        string `gorm:"column:code;size:191;uniqueIndex"`
	Name        string `gorm:"column:name;size:191"`
	Description string `gorm:"column:description;type:text"`
	ScopeType   string `gorm:"column:scope_type;size:32;index"`
	TenantCode  string `gorm:"column:tenant_code;size:191;index"`
	OrgUnitCode string `gorm:"column:org_unit_code;size:191;index"`
	Status      string `gorm:"column:status;size:32;index"`
	UpdatedAt   string `gorm:"column:updated_at;size:35"`
	ValuesJSON  string `gorm:"column:values_json;type:text"`
}

type gormUserRole struct {
	UserID   string `gorm:"column:user_id;size:191;primaryKey"`
	RoleCode string `gorm:"column:role_code;size:191;primaryKey"`
}

type gormResourceLifecycle struct {
	Resource              string `gorm:"column:resource;size:128;primaryKey"`
	RecordID              string `gorm:"column:record_id;size:191;primaryKey"`
	DeletedAt             string `gorm:"column:deleted_at;size:64;index;not null"`
	DeletedBy             string `gorm:"column:deleted_by;size:191;not null"`
	DeleteReason          string `gorm:"column:delete_reason;size:191;not null"`
	PurgeAfter            string `gorm:"column:purge_after;size:64;index;not null"`
	DeletionPolicyVersion uint32 `gorm:"column:deletion_policy_version;not null"`
}

type gormRolePermission struct {
	RoleCode   string `gorm:"column:role_code;size:191;primaryKey"`
	Permission string `gorm:"column:permission;size:191;primaryKey"`
}

type gormPermission struct {
	ID           string `gorm:"column:id;size:191;primaryKey"`
	Code         string `gorm:"column:code;size:191;uniqueIndex"`
	Name         string `gorm:"column:name;size:191"`
	Status       string `gorm:"column:status;size:32;index"`
	Description  string `gorm:"column:description;type:text"`
	UpdatedAt    string `gorm:"column:updated_at;size:35"`
	Capability   string `gorm:"column:capability;size:191"`
	Resource     string `gorm:"column:resource;size:191"`
	Action       string `gorm:"column:action;size:64"`
	Prefix       string `gorm:"column:prefix;size:191"`
	ResourceType string `gorm:"column:resource_type;size:32;not null;default:api"`
	ValuesJSON   string `gorm:"column:values_json;type:text"`
}

type gormOrgUnitRoleGroup struct {
	OrgUnitCode   string    `gorm:"column:org_unit_code;size:191;primaryKey"`
	RoleGroupCode string    `gorm:"column:role_group_code;size:191;primaryKey"`
	Revision      uint64    `gorm:"column:revision;not null"`
	ActorID       string    `gorm:"column:actor_id;size:191;not null"`
	CreatedAt     time.Time `gorm:"column:created_at;not null"`
	UpdatedAt     time.Time `gorm:"column:updated_at;not null"`
}

type gormOrgUnitRoleGroupRevision struct {
	OrgUnitCode string    `gorm:"column:org_unit_code;size:191;primaryKey"`
	Revision    uint64    `gorm:"column:revision;not null"`
	UpdatedAt   time.Time `gorm:"column:updated_at;not null"`
}

type gormAdminResourceState struct {
	Key   string `gorm:"column:key;primaryKey"`
	Value string `gorm:"column:value;not null"`
}

func (gormOrganization) TableName() string             { return organizationsTable }
func (gormRoleGroup) TableName() string                { return roleGroupsTable }
func (gormRole) TableName() string                     { return rolesTable }
func (gormUser) TableName() string                     { return usersTable }
func (gormUserRole) TableName() string                 { return userRolesTable }
func (gormPermission) TableName() string               { return permissionsTable }
func (gormRolePermission) TableName() string           { return rolePermissionsTable }
func (gormResourceLifecycle) TableName() string        { return resourceLifecycleTable }
func (gormOrgUnitRoleGroup) TableName() string         { return orgUnitRoleGroupsTable }
func (gormOrgUnitRoleGroupRevision) TableName() string { return orgUnitRoleGroupRevisionsTable }
func (gormAdminResourceState) TableName() string       { return adminResourceStateTable }

func PrepareGORMRepository(ctx context.Context, db *gorm.DB) (*GORMRepository, error) {
	if ctx == nil || db == nil {
		return nil, ErrRepositoryFailed
	}
	if err := db.WithContext(ctx).AutoMigrate(repositoryModels()...); err != nil {
		return nil, fmt.Errorf("%w: prepare schema", ErrRepositoryFailed)
	}
	if err := db.WithContext(ctx).Model(&gormPermission{}).
		Where("resource_type = '' OR resource_type IS NULL").Update("resource_type", PermissionResourceTypeAPI).Error; err != nil {
		return nil, fmt.Errorf("%w: backfill permission resource type", ErrRepositoryFailed)
	}
	if err := db.WithContext(ctx).Model(&gormMenu{}).
		Where("parent_code = '' AND parent <> ''").Update("parent_code", gorm.Expr("parent")).Error; err != nil {
		return nil, fmt.Errorf("%w: backfill menu parent code", ErrRepositoryFailed)
	}
	if err := db.WithContext(ctx).Model(&gormMenu{}).
		Where("resource_code = '' AND resource <> ''").Update("resource_code", gorm.Expr("resource")).Error; err != nil {
		return nil, fmt.Errorf("%w: backfill menu resource code", ErrRepositoryFailed)
	}
	return OpenGORMRepository(ctx, db)
}

func OpenGORMRepository(ctx context.Context, db *gorm.DB) (*GORMRepository, error) {
	if ctx == nil || db == nil {
		return nil, ErrRepositoryFailed
	}
	migrator := db.WithContext(ctx).Migrator()
	for _, model := range repositoryModels() {
		if !migrator.HasTable(model) {
			return nil, ErrRepositoryFailed
		}
	}
	checks := []struct {
		model  any
		fields []string
	}{
		{model: gormOrganization{}, fields: []string{"Code", "TenantCode", "Status"}},
		{model: gormRoleGroup{}, fields: []string{"Code", "ScopeType", "TenantCode", "Status", "Revision"}},
		{model: gormRole{}, fields: []string{"Code", "Name", "GroupCode", "Status"}},
		{model: gormUser{}, fields: []string{"ID", "ScopeType", "TenantCode", "OrgUnitCode"}},
		{model: gormUserRole{}, fields: []string{"UserID", "RoleCode"}},
		{model: gormPermission{}, fields: []string{"ID", "Code", "Status", "ResourceType"}},
		{model: gormRolePermission{}, fields: []string{"RoleCode", "Permission"}},
		{model: gormMenu{}, fields: []string{"ID", "Code", "Status", "NodeType", "ParentCode", "Route", "ComponentKey", "ResourceCode", "ExternalURL", "ParametersJSON", "BreadcrumbVisible", "LegacyPermission"}},
		{model: gormRoleMenu{}, fields: []string{"RoleCode", "MenuCode", "Revision", "ActorID", "CreatedAt", "UpdatedAt"}},
		{model: gormRoleMenuRevision{}, fields: []string{"RoleCode", "Revision", "UpdatedAt"}},
		{model: gormPageButton{}, fields: []string{"MenuCode", "ButtonKey", "Action", "Status", "PermissionCode"}},
		{model: gormOrgUnitRoleGroup{}, fields: []string{"OrgUnitCode", "RoleGroupCode", "Revision", "ActorID", "CreatedAt", "UpdatedAt"}},
		{model: gormOrgUnitRoleGroupRevision{}, fields: []string{"OrgUnitCode", "Revision", "UpdatedAt"}},
	}
	for _, check := range checks {
		for _, field := range check.fields {
			if !migrator.HasColumn(check.model, field) {
				return nil, ErrRepositoryFailed
			}
		}
	}
	return &GORMRepository{db: db}, nil
}

func (r *GORMRepository) Persistent() bool {
	return r != nil && r.db != nil
}

func (r *GORMRepository) LoadOrgUnitRoleGroups(ctx context.Context, orgUnitCode string) (OrgUnitRoleGroupSet, error) {
	if !r.ready(ctx) || !validCode(orgUnitCode) {
		return OrgUnitRoleGroupSet{}, ErrRepositoryFailed
	}
	var rows []gormOrgUnitRoleGroup
	if err := r.db.WithContext(ctx).Where("org_unit_code = ?", orgUnitCode).Order("role_group_code").Find(&rows).Error; err != nil {
		return OrgUnitRoleGroupSet{}, repositoryError(err)
	}
	revision, err := loadOrgUnitRoleGroupRevision(r.db.WithContext(ctx), orgUnitCode)
	if err != nil {
		return OrgUnitRoleGroupSet{}, repositoryError(err)
	}
	result := OrgUnitRoleGroupSet{OrgUnitCode: orgUnitCode, Revision: revision, RoleGroupCodes: make([]string, 0, len(rows))}
	for _, row := range rows {
		if row.Revision != revision {
			return OrgUnitRoleGroupSet{}, ErrRepositoryFailed
		}
		result.RoleGroupCodes = append(result.RoleGroupCodes, row.RoleGroupCode)
	}
	return result, nil
}

func (r *GORMRepository) EffectiveRolePool(ctx context.Context, orgUnitCode string) ([]RolePoolEntry, error) {
	if !r.ready(ctx) || !validCode(orgUnitCode) {
		return nil, ErrRepositoryFailed
	}
	organization, groups, roles, bindings, err := r.loadOrganizationState(r.db.WithContext(ctx), orgUnitCode, nil)
	if err != nil {
		return nil, err
	}
	return EffectiveRolePool(organization, groups, roles, bindings)
}

func (r *GORMRepository) PreviewOrgUnitRoleGroups(ctx context.Context, orgUnitCode string, roleGroupCodes []string) (OrgUnitRoleGroupImpact, error) {
	if !r.ready(ctx) || !validCode(orgUnitCode) {
		return OrgUnitRoleGroupImpact{}, ErrRepositoryFailed
	}
	codes, err := canonicalCodes(roleGroupCodes)
	if err != nil {
		return OrgUnitRoleGroupImpact{}, err
	}
	current, err := r.LoadOrgUnitRoleGroups(ctx, orgUnitCode)
	if err != nil {
		return OrgUnitRoleGroupImpact{}, err
	}
	organization, groups, roles, bindings, err := r.loadOrganizationState(r.db.WithContext(ctx), orgUnitCode, codes)
	if err != nil {
		return OrgUnitRoleGroupImpact{}, err
	}
	pool, err := EffectiveRolePool(organization, groups, roles, bindings)
	if err != nil {
		return OrgUnitRoleGroupImpact{}, err
	}
	conflicts, err := persistedUserAssignmentConflicts(r.db.WithContext(ctx), organization, pool)
	if err != nil {
		return OrgUnitRoleGroupImpact{}, err
	}
	affected := map[string]struct{}{}
	for _, conflict := range conflicts {
		affected[conflict.UserCode] = struct{}{}
	}
	return OrgUnitRoleGroupImpact{
		TenantCode:            organization.TenantCode,
		CurrentRoleGroupCodes: current.RoleGroupCodes, ProposedRoleGroupCodes: codes,
		Conflicts: conflicts, AffectedUsers: len(affected), ExpectedRevision: current.Revision,
	}, nil
}

func (r *GORMRepository) DeriveAndValidateUser(ctx context.Context, user User, roleCodes []string) (User, error) {
	if !r.ready(ctx) {
		return User{}, ErrRepositoryFailed
	}
	if user.ScopeType == ScopeTenant && validCode(user.OrgUnitCode) {
		organization, groups, roles, bindings, err := r.loadOrganizationState(r.db.WithContext(ctx), user.OrgUnitCode, nil)
		if err != nil {
			return User{}, err
		}
		return DeriveAndValidateUser(user, roleCodes, map[string]Organization{organization.Code: organization}, groups, roles, bindings)
	}
	groups, roles, err := r.loadAllRoleState(r.db.WithContext(ctx))
	if err != nil {
		return User{}, err
	}
	return DeriveAndValidateUser(user, roleCodes, nil, groups, roles, nil)
}

func (r *GORMRepository) ReplaceOrgUnitRoleGroups(ctx context.Context, request ReplaceOrgUnitRoleGroupsRequest) (OrgUnitRoleGroupSet, error) {
	return r.ReplaceOrgUnitRoleGroupsWithRemediations(ctx, request, nil)
}

func (r *GORMRepository) ReplaceOrgUnitRoleGroupsWithRemediations(ctx context.Context, request ReplaceOrgUnitRoleGroupsRequest, remediations []RoleAssignmentRemediation) (OrgUnitRoleGroupSet, error) {
	if !r.ready(ctx) || !validCode(request.OrgUnitCode) || !validCode(request.ActorID) || request.ChangedAt.IsZero() || request.ExpectedRevision == ^uint64(0) {
		return OrgUnitRoleGroupSet{}, ErrInvalid
	}
	codes, err := canonicalCodes(request.RoleGroupCodes)
	if err != nil {
		return OrgUnitRoleGroupSet{}, err
	}
	request.ChangedAt = request.ChangedAt.UTC()
	var committed OrgUnitRoleGroupSet
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		organization, groups, roles, bindings, err := r.loadOrganizationState(tx, request.OrgUnitCode, codes)
		if err != nil {
			return err
		}
		pool, err := EffectiveRolePool(organization, groups, roles, bindings)
		if err != nil {
			return err
		}
		if err := applyRoleAssignmentRemediations(tx, organization, pool, remediations); err != nil {
			return err
		}
		if err := validatePersistedUserAssignments(tx, organization, pool); err != nil {
			return err
		}

		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&gormOrgUnitRoleGroupRevision{
			OrgUnitCode: request.OrgUnitCode, Revision: 0, UpdatedAt: request.ChangedAt,
		}).Error; err != nil {
			return err
		}
		nextRevision := request.ExpectedRevision + 1
		result := tx.Model(&gormOrgUnitRoleGroupRevision{}).
			Where("org_unit_code = ? AND revision = ?", request.OrgUnitCode, request.ExpectedRevision).
			Updates(map[string]any{"revision": nextRevision, "updated_at": request.ChangedAt})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			actual, err := loadOrgUnitRoleGroupRevision(tx, request.OrgUnitCode)
			if err != nil {
				return err
			}
			return &RevisionConflictError{Expected: request.ExpectedRevision, Actual: actual}
		}

		var previous []gormOrgUnitRoleGroup
		if err := tx.Where("org_unit_code = ?", request.OrgUnitCode).Find(&previous).Error; err != nil {
			return err
		}
		createdAt := make(map[string]time.Time, len(previous))
		for _, row := range previous {
			createdAt[row.RoleGroupCode] = row.CreatedAt
		}
		if err := tx.Where("org_unit_code = ?", request.OrgUnitCode).Delete(&gormOrgUnitRoleGroup{}).Error; err != nil {
			return err
		}
		rows := make([]gormOrgUnitRoleGroup, 0, len(codes))
		for _, code := range codes {
			created := createdAt[code]
			if created.IsZero() {
				created = request.ChangedAt
			}
			rows = append(rows, gormOrgUnitRoleGroup{
				OrgUnitCode: request.OrgUnitCode, RoleGroupCode: code, Revision: nextRevision,
				ActorID: request.ActorID, CreatedAt: created, UpdatedAt: request.ChangedAt,
			})
		}
		if len(rows) > 0 {
			if err := tx.Create(&rows).Error; err != nil {
				return err
			}
		}
		if _, err := bumpGlobalRevision(tx); err != nil {
			return err
		}
		committed = OrgUnitRoleGroupSet{OrgUnitCode: request.OrgUnitCode, RoleGroupCodes: codes, Revision: nextRevision}
		return nil
	})
	if err != nil {
		return OrgUnitRoleGroupSet{}, repositoryError(err)
	}
	return committed, nil
}

func (r *GORMRepository) CurrentGlobalRevision(ctx context.Context) (uint64, error) {
	if !r.ready(ctx) {
		return 0, ErrRepositoryFailed
	}
	return loadGlobalRevision(r.db.WithContext(ctx))
}

func (r *GORMRepository) ValidateCutover(ctx context.Context) (CutoverReport, error) {
	if !r.ready(ctx) {
		return CutoverReport{}, ErrRepositoryFailed
	}
	db := r.db.WithContext(ctx)
	var organizations []gormOrganization
	var groups []gormRoleGroup
	var roles []gormRole
	var users []gormUser
	var bindings []gormOrgUnitRoleGroup
	if err := db.Find(&organizations).Error; err != nil {
		return CutoverReport{}, repositoryError(err)
	}
	if err := db.Find(&groups).Error; err != nil {
		return CutoverReport{}, repositoryError(err)
	}
	if err := db.Find(&roles).Error; err != nil {
		return CutoverReport{}, repositoryError(err)
	}
	if err := db.Find(&users).Error; err != nil {
		return CutoverReport{}, repositoryError(err)
	}
	if err := db.Find(&bindings).Error; err != nil {
		return CutoverReport{}, repositoryError(err)
	}
	organizationMap := make(map[string]Organization, len(organizations))
	for _, row := range organizations {
		deleted, err := isLifecycleDeleted(db, "org-units", row.ID)
		if err != nil {
			return CutoverReport{}, err
		}
		organizationMap[row.Code] = organizationFromGORM(row, deleted)
	}
	groupMap := make(map[string]RoleGroup, len(groups))
	for _, row := range groups {
		if strings.TrimSpace(row.ParentCode) != "" {
			return CutoverReport{}, &ValidationError{Field: "roleGroup.parentCode", Reason: "nested role groups must be flattened before cutover"}
		}
		deleted, err := isLifecycleDeleted(db, "role-groups", row.ID)
		if err != nil {
			return CutoverReport{}, err
		}
		group := roleGroupFromGORM(row, deleted)
		if err := ValidateRoleGroup(group); err != nil {
			return CutoverReport{}, err
		}
		groupMap[group.Code] = group
	}
	roleMap := make(map[string]Role, len(roles))
	for _, row := range roles {
		deleted, err := isLifecycleDeleted(db, "roles", row.ID)
		if err != nil {
			return CutoverReport{}, err
		}
		role := Role{Code: row.Code, Name: row.Name, GroupCode: row.GroupCode, Status: row.Status, Deleted: deleted}
		if err := ValidateRole(role, groupMap); err != nil {
			return CutoverReport{}, err
		}
		roleMap[role.Code] = role
	}
	domainBindings := make([]OrgUnitRoleGroupBinding, 0, len(bindings))
	for _, binding := range bindings {
		organization, exists := organizationMap[binding.OrgUnitCode]
		if !exists {
			return CutoverReport{}, fmt.Errorf("%w: organization %q", ErrNotFound, binding.OrgUnitCode)
		}
		group, exists := groupMap[binding.RoleGroupCode]
		if !exists {
			return CutoverReport{}, fmt.Errorf("%w: role group %q", ErrNotFound, binding.RoleGroupCode)
		}
		if err := validateBoundRoleGroup(organization, group); err != nil {
			return CutoverReport{}, err
		}
		domainBindings = append(domainBindings, OrgUnitRoleGroupBinding{OrgUnitCode: binding.OrgUnitCode, RoleGroupCode: binding.RoleGroupCode})
	}
	for _, row := range users {
		deleted, err := isLifecycleDeleted(db, "users", row.ID)
		if err != nil {
			return CutoverReport{}, err
		}
		var assignments []gormUserRole
		if err := db.Where("user_id = ?", row.ID).Order("role_code").Find(&assignments).Error; err != nil {
			return CutoverReport{}, repositoryError(err)
		}
		roleCodes := make([]string, 0, len(assignments))
		for _, assignment := range assignments {
			roleCodes = append(roleCodes, assignment.RoleCode)
		}
		_, err = DeriveAndValidateUser(User{
			ID: row.ID, Code: row.Code, ScopeType: ScopeType(row.ScopeType), TenantCode: row.TenantCode,
			OrgUnitCode: row.OrgUnitCode, Status: row.Status, Deleted: deleted,
		}, roleCodes, organizationMap, groupMap, roleMap, domainBindings)
		if err != nil {
			return CutoverReport{}, err
		}
	}
	return CutoverReport{Organizations: len(organizations), RoleGroups: len(groups), Roles: len(roles), Users: len(users), Bindings: len(bindings)}, nil
}

func loadGlobalRevision(db *gorm.DB) (uint64, error) {
	var row gormAdminResourceState
	err := db.Where("key = ?", "revision").Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, repositoryError(err)
	}
	revision, err := strconv.ParseUint(row.Value, 10, 64)
	if err != nil {
		return 0, ErrRepositoryFailed
	}
	return revision, nil
}

func bumpGlobalRevision(db *gorm.DB) (uint64, error) {
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&gormAdminResourceState{Key: "revision", Value: "0"}).Error; err != nil {
		return 0, repositoryError(err)
	}
	current, err := loadGlobalRevision(db)
	if err != nil {
		return 0, err
	}
	return advanceGlobalRevision(db, current)
}

func advanceGlobalRevision(db *gorm.DB, expected uint64) (uint64, error) {
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&gormAdminResourceState{Key: "revision", Value: "0"}).Error; err != nil {
		return 0, repositoryError(err)
	}
	next := expected + 1
	result := db.Model(&gormAdminResourceState{}).Where("key = ? AND value = ?", "revision", strconv.FormatUint(expected, 10)).Update("value", strconv.FormatUint(next, 10))
	if result.Error != nil {
		return 0, repositoryError(result.Error)
	}
	if result.RowsAffected != 1 {
		actual, err := loadGlobalRevision(db)
		if err != nil {
			return 0, err
		}
		return 0, &RevisionConflictError{Expected: expected, Actual: actual}
	}
	return next, nil
}

func (r *GORMRepository) loadOrganizationState(db *gorm.DB, orgUnitCode string, requestedGroupCodes []string) (Organization, map[string]RoleGroup, map[string]Role, []OrgUnitRoleGroupBinding, error) {
	organizationRow, err := loadOrganization(db, orgUnitCode)
	if err != nil {
		return Organization{}, nil, nil, nil, err
	}
	organizationDeleted, err := isLifecycleDeleted(db, "org-units", organizationRow.ID)
	if err != nil {
		return Organization{}, nil, nil, nil, err
	}
	organization := organizationFromGORM(organizationRow, organizationDeleted)

	groupCodes := requestedGroupCodes
	if groupCodes == nil {
		var rows []gormOrgUnitRoleGroup
		if err := db.Where("org_unit_code = ?", orgUnitCode).Order("role_group_code").Find(&rows).Error; err != nil {
			return Organization{}, nil, nil, nil, repositoryError(err)
		}
		groupCodes = make([]string, 0, len(rows))
		for _, row := range rows {
			groupCodes = append(groupCodes, row.RoleGroupCode)
		}
	}
	groups, err := loadRoleGroups(db, groupCodes)
	if err != nil {
		return Organization{}, nil, nil, nil, err
	}
	bindings := make([]OrgUnitRoleGroupBinding, 0, len(groupCodes))
	for _, code := range groupCodes {
		bindings = append(bindings, OrgUnitRoleGroupBinding{OrgUnitCode: orgUnitCode, RoleGroupCode: code})
	}
	roles, err := loadRolesForGroups(db, groupCodes)
	if err != nil {
		return Organization{}, nil, nil, nil, err
	}
	return organization, groups, roles, bindings, nil
}

func (r *GORMRepository) loadAllRoleState(db *gorm.DB) (map[string]RoleGroup, map[string]Role, error) {
	var groupRows []gormRoleGroup
	if err := db.Order("code").Find(&groupRows).Error; err != nil {
		return nil, nil, repositoryError(err)
	}
	groups := make(map[string]RoleGroup, len(groupRows))
	groupCodes := make([]string, 0, len(groupRows))
	for _, row := range groupRows {
		deleted, err := isLifecycleDeleted(db, "role-groups", row.ID)
		if err != nil {
			return nil, nil, err
		}
		groups[row.Code] = roleGroupFromGORM(row, deleted)
		groupCodes = append(groupCodes, row.Code)
	}
	roles, err := loadRolesForGroups(db, groupCodes)
	return groups, roles, err
}

func loadOrganization(db *gorm.DB, code string) (gormOrganization, error) {
	var row gormOrganization
	err := db.Where("code = ?", code).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return gormOrganization{}, fmt.Errorf("%w: organization %q", ErrNotFound, code)
	}
	if err != nil {
		return gormOrganization{}, repositoryError(err)
	}
	return row, nil
}

func loadRoleGroups(db *gorm.DB, codes []string) (map[string]RoleGroup, error) {
	if len(codes) == 0 {
		return map[string]RoleGroup{}, nil
	}
	var rows []gormRoleGroup
	if err := db.Where("code IN ?", codes).Find(&rows).Error; err != nil {
		return nil, repositoryError(err)
	}
	if len(rows) != len(codes) {
		return nil, fmt.Errorf("%w: one or more role groups", ErrNotFound)
	}
	groups := make(map[string]RoleGroup, len(rows))
	for _, row := range rows {
		deleted, err := isLifecycleDeleted(db, "role-groups", row.ID)
		if err != nil {
			return nil, err
		}
		group := roleGroupFromGORM(row, deleted)
		if err := ValidateRoleGroup(group); err != nil {
			return nil, err
		}
		groups[group.Code] = group
	}
	return groups, nil
}

func loadRolesForGroups(db *gorm.DB, groupCodes []string) (map[string]Role, error) {
	if len(groupCodes) == 0 {
		return map[string]Role{}, nil
	}
	var rows []gormRole
	if err := db.Where("group_code IN ?", groupCodes).Order("code").Find(&rows).Error; err != nil {
		return nil, repositoryError(err)
	}
	roles := make(map[string]Role, len(rows))
	for _, row := range rows {
		deleted, err := isLifecycleDeleted(db, "roles", row.ID)
		if err != nil {
			return nil, err
		}
		roles[row.Code] = Role{Code: row.Code, Name: row.Name, GroupCode: row.GroupCode, Status: row.Status, Deleted: deleted}
	}
	return roles, nil
}

func validatePersistedUserAssignments(db *gorm.DB, organization Organization, pool []RolePoolEntry) error {
	conflicts, err := persistedUserAssignmentConflicts(db, organization, pool)
	if err != nil {
		return err
	}
	if len(conflicts) > 0 {
		return fmt.Errorf("%w: assigned role %q for user %q would fall outside organization %q", ErrRolePoolViolation, conflicts[0].RoleCode, conflicts[0].UserCode, organization.Code)
	}
	return nil
}

func persistedUserAssignmentConflicts(db *gorm.DB, organization Organization, pool []RolePoolEntry) ([]RoleAssignmentConflict, error) {
	allowed := make(map[string]struct{}, len(pool))
	for _, entry := range pool {
		allowed[entry.RoleCode] = struct{}{}
	}
	var users []gormUser
	if err := db.Where("scope_type = ? AND org_unit_code = ?", string(ScopeTenant), organization.Code).Find(&users).Error; err != nil {
		return nil, repositoryError(err)
	}
	if len(users) == 0 {
		return nil, nil
	}
	userIDs := make([]string, 0, len(users))
	userCodes := make(map[string]string, len(users))
	for _, user := range users {
		if user.TenantCode != organization.TenantCode {
			return nil, fmt.Errorf("%w: user %q tenant does not match organization", ErrRolePoolViolation, user.Code)
		}
		userIDs = append(userIDs, user.ID)
		userCodes[user.ID] = user.Code
	}
	var assignments []gormUserRole
	if err := db.Where("user_id IN ?", userIDs).Find(&assignments).Error; err != nil {
		return nil, repositoryError(err)
	}
	conflicts := make([]RoleAssignmentConflict, 0)
	for _, assignment := range assignments {
		if _, exists := allowed[assignment.RoleCode]; !exists {
			conflicts = append(conflicts, RoleAssignmentConflict{UserCode: userCodes[assignment.UserID], RoleCode: assignment.RoleCode})
		}
	}
	sort.Slice(conflicts, func(left, right int) bool {
		if conflicts[left].UserCode != conflicts[right].UserCode {
			return conflicts[left].UserCode < conflicts[right].UserCode
		}
		return conflicts[left].RoleCode < conflicts[right].RoleCode
	})
	return conflicts, nil
}

func applyRoleAssignmentRemediations(db *gorm.DB, organization Organization, pool []RolePoolEntry, remediations []RoleAssignmentRemediation) error {
	if len(remediations) == 0 {
		return nil
	}
	conflicts, err := persistedUserAssignmentConflicts(db, organization, pool)
	if err != nil {
		return err
	}
	conflictTargets := make(map[string]struct{}, len(conflicts))
	for _, conflict := range conflicts {
		conflictTargets[conflict.UserCode+"\x00"+conflict.RoleCode] = struct{}{}
	}
	allowed := make(map[string]struct{}, len(pool))
	for _, entry := range pool {
		allowed[entry.RoleCode] = struct{}{}
	}
	for _, remediation := range remediations {
		if !validCode(remediation.UserCode) || !validCode(remediation.RoleCode) {
			return ErrInvalid
		}
		if _, exists := conflictTargets[remediation.UserCode+"\x00"+remediation.RoleCode]; !exists {
			return ErrInvalid
		}
		var user gormUser
		if err := db.Where("code = ? AND scope_type = ? AND org_unit_code = ?", remediation.UserCode, string(ScopeTenant), organization.Code).Take(&user).Error; err != nil {
			return repositoryError(err)
		}
		assignment := gormUserRole{UserID: user.ID, RoleCode: remediation.RoleCode}
		var count int64
		if err := db.Model(&gormUserRole{}).Where("user_id = ? AND role_code = ?", assignment.UserID, assignment.RoleCode).Count(&count).Error; err != nil || count != 1 {
			return ErrRolePoolViolation
		}
		switch remediation.Action {
		case "remove-role":
			if remediation.ReplacementRoleCode != "" {
				return ErrInvalid
			}
		case "replace-role":
			if _, exists := allowed[remediation.ReplacementRoleCode]; !exists {
				return ErrRolePoolViolation
			}
		default:
			return ErrInvalid
		}
		if err := db.Where("user_id = ? AND role_code = ?", assignment.UserID, assignment.RoleCode).Delete(&gormUserRole{}).Error; err != nil {
			return repositoryError(err)
		}
		if remediation.Action == "replace-role" {
			if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&gormUserRole{UserID: user.ID, RoleCode: remediation.ReplacementRoleCode}).Error; err != nil {
				return repositoryError(err)
			}
		}
	}
	return nil
}

func loadOrgUnitRoleGroupRevision(db *gorm.DB, orgUnitCode string) (uint64, error) {
	var row gormOrgUnitRoleGroupRevision
	err := db.Where("org_unit_code = ?", orgUnitCode).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return row.Revision, nil
}

func isLifecycleDeleted(db *gorm.DB, resource, recordID string) (bool, error) {
	if recordID == "" {
		return false, nil
	}
	var count int64
	if err := db.Model(&gormResourceLifecycle{}).
		Where("resource = ? AND record_id = ? AND deleted_at <> ''", resource, recordID).
		Count(&count).Error; err != nil {
		return false, repositoryError(err)
	}
	return count > 0, nil
}

func organizationFromGORM(row gormOrganization, deleted bool) Organization {
	return Organization{Code: row.Code, TenantCode: row.TenantCode, Status: row.Status, Deleted: deleted}
}

func roleGroupFromGORM(row gormRoleGroup, deleted bool) RoleGroup {
	return RoleGroup{
		Code: row.Code, Name: row.Name, ScopeType: ScopeType(row.ScopeType), TenantCode: row.TenantCode,
		Status: row.Status, Deleted: deleted, Revision: row.Revision,
	}
}

func repositoryModels() []any {
	return []any{
		&gormOrganization{}, &gormRoleGroup{}, &gormRole{}, &gormUser{}, &gormUserRole{}, &gormResourceLifecycle{},
		&gormPermission{}, &gormRolePermission{},
		&gormMenu{}, &gormRoleMenu{}, &gormRoleMenuRevision{}, &gormPageButton{},
		&gormOrgUnitRoleGroup{}, &gormOrgUnitRoleGroupRevision{}, &gormAdminResourceState{},
		&gormOrganizationRBACPreview{}, &gormOrganizationRBACAuditEvent{},
		&gormOrganizationRBACMigrationRun{}, &gormOrganizationRBACMigrationConflict{},
	}
}

func (r *GORMRepository) ready(ctx context.Context) bool {
	return r != nil && r.db != nil && ctx != nil && ctx.Err() == nil
}

func repositoryError(err error) error {
	if err == nil || errors.Is(err, ErrInvalid) || errors.Is(err, ErrNotFound) || errors.Is(err, ErrRevisionConflict) || errors.Is(err, ErrRolePoolViolation) {
		return err
	}
	return fmt.Errorf("%w: %v", ErrRepositoryFailed, err)
}
