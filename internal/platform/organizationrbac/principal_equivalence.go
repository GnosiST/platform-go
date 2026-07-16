package organizationrbac

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"

	"platform-go/internal/platform/rbac"

	"gorm.io/gorm"
)

type PrincipalComparisonMode string

const (
	PrincipalComparisonCandidate PrincipalComparisonMode = "candidate"
	PrincipalComparisonPersisted PrincipalComparisonMode = "persisted"
)

type PrincipalAuthorizationSnapshot struct {
	UserCode           string
	ScopeType          ScopeType
	TenantCode         string
	OrgUnitCode        string
	RoleCodes          []string
	AllowPermissions   []string
	DeniedPermissions  []string
	DataScopeAll       bool
	DataScopeOrgCodes  []string
	DataScopeAreaCodes []string
	DataScopeSelf      bool
	MenuCodes          []string
}

type PrincipalAuthorizationDifference struct {
	UserCode        string
	LegacyHash      string
	TargetHash      string
	ChangedFields   []string
	BlockingReasons []string
	AddedMenus      int
	RemovedMenus    int
}

type PrincipalEquivalenceReport struct {
	ActivePrincipals int
	Equivalent       int
	Differences      []PrincipalAuthorizationDifference
	ComparisonHash   string
}

type principalComparisonState struct {
	Manifest               MigrationManifest
	Users                  []gormUser
	AssignmentsByUserID    map[string][]string
	RolesByCode            map[string]gormRole
	RoleValuesByCode       map[string]map[string]string
	AllowByRoleCode        map[string][]string
	EnabledPermissions     []string
	EnabledPolicyCodes     map[string]struct{}
	EnabledPages           []gormMenu
	PersistedLeavesByRole  map[string][]string
	RoleMenuRevisionByRole map[string]uint64
	OrganizationsByCode    map[string]gormOrganization
	OrgParentByCode        map[string]string
	OrgAreaByCode          map[string]string
	AreaParentByCode       map[string]string
	UserAreaByID           map[string]string
}

type intendedPrincipalIdentity struct {
	ScopeType   ScopeType
	TenantCode  string
	OrgUnitCode string
	AreaCode    string
}

type principalOrgScopeRow struct {
	Code       string `gorm:"column:code"`
	ParentCode string `gorm:"column:parent_code"`
	AreaCode   string `gorm:"column:area_code"`
}

type principalAreaScopeRow struct {
	Code       string `gorm:"column:code"`
	ParentCode string `gorm:"column:parent_code"`
}

func (r *GORMRepository) CompareAllActivePrincipals(ctx context.Context, manifest MigrationManifest, mode PrincipalComparisonMode) (PrincipalEquivalenceReport, error) {
	if !r.ready(ctx) {
		return PrincipalEquivalenceReport{}, ErrRepositoryFailed
	}
	state, err := loadPrincipalComparisonState(r.db.WithContext(ctx), manifest, mode)
	if err != nil {
		return PrincipalEquivalenceReport{}, err
	}
	return compareLoadedPrincipals(state, mode)
}

func loadPrincipalComparisonState(db *gorm.DB, manifest MigrationManifest, mode PrincipalComparisonMode) (principalComparisonState, error) {
	if db == nil || mode != PrincipalComparisonCandidate && mode != PrincipalComparisonPersisted {
		return principalComparisonState{}, ErrInvalid
	}
	state := principalComparisonState{
		Manifest: manifest, AssignmentsByUserID: map[string][]string{}, RolesByCode: map[string]gormRole{},
		RoleValuesByCode: map[string]map[string]string{}, AllowByRoleCode: map[string][]string{},
		EnabledPolicyCodes:    map[string]struct{}{},
		PersistedLeavesByRole: map[string][]string{}, RoleMenuRevisionByRole: map[string]uint64{},
		OrganizationsByCode: map[string]gormOrganization{}, OrgParentByCode: map[string]string{},
		OrgAreaByCode: map[string]string{}, AreaParentByCode: map[string]string{}, UserAreaByID: map[string]string{},
	}
	deletedUsers, err := deletedRecordIDs(db, "users")
	if err != nil {
		return principalComparisonState{}, err
	}
	var users []gormUser
	if err := db.Where("status = ?", StatusEnabled).Order("code").Find(&users).Error; err != nil {
		return principalComparisonState{}, repositoryError(err)
	}
	for _, user := range users {
		if _, deleted := deletedUsers[user.ID]; deleted {
			continue
		}
		state.Users = append(state.Users, user)
		var values map[string]string
		if json.Unmarshal([]byte(user.ValuesJSON), &values) == nil {
			state.UserAreaByID[user.ID] = strings.TrimSpace(values["areaCode"])
		}
	}
	var assignments []gormUserRole
	if err := db.Order("user_id, role_code").Find(&assignments).Error; err != nil {
		return principalComparisonState{}, repositoryError(err)
	}
	for _, assignment := range assignments {
		state.AssignmentsByUserID[assignment.UserID] = append(state.AssignmentsByUserID[assignment.UserID], assignment.RoleCode)
	}
	deletedRoles, err := deletedRecordIDs(db, "roles")
	if err != nil {
		return principalComparisonState{}, err
	}
	var roles []gormRole
	if err := db.Where("status = ?", StatusEnabled).Order("code").Find(&roles).Error; err != nil {
		return principalComparisonState{}, repositoryError(err)
	}
	for _, role := range roles {
		if _, deleted := deletedRoles[role.ID]; deleted {
			continue
		}
		values := map[string]string{}
		if strings.TrimSpace(role.ValuesJSON) != "" {
			if err := json.Unmarshal([]byte(role.ValuesJSON), &values); err != nil || values == nil {
				return principalComparisonState{}, ErrInvalid
			}
		}
		state.RolesByCode[role.Code] = role
		state.RoleValuesByCode[role.Code] = values
	}
	deletedPermissions, err := deletedRecordIDs(db, "permissions")
	if err != nil {
		return principalComparisonState{}, err
	}
	var permissions []gormPermission
	if err := db.Where("status = ?", StatusEnabled).Order("code").Find(&permissions).Error; err != nil {
		return principalComparisonState{}, repositoryError(err)
	}
	for _, permission := range permissions {
		if _, deleted := deletedPermissions[permission.ID]; deleted || !validPermissionResourceType(permission.ResourceType) {
			continue
		}
		state.EnabledPolicyCodes[permission.Code] = struct{}{}
		if !strings.Contains(permission.Code, "*") {
			state.EnabledPermissions = append(state.EnabledPermissions, permission.Code)
		}
	}
	var allowRows []gormRolePermission
	if err := db.Order("role_code, permission").Find(&allowRows).Error; err != nil {
		return principalComparisonState{}, repositoryError(err)
	}
	for _, row := range allowRows {
		_, activeRole := state.RolesByCode[row.RoleCode]
		_, enabledPermission := state.EnabledPolicyCodes[strings.TrimSpace(row.Permission)]
		if activeRole && enabledPermission {
			state.AllowByRoleCode[row.RoleCode] = append(state.AllowByRoleCode[row.RoleCode], strings.TrimSpace(row.Permission))
		}
	}
	deletedMenus, err := deletedRecordIDs(db, "menus")
	if err != nil {
		return principalComparisonState{}, err
	}
	var menus []gormMenu
	if err := db.Where("status = ? AND node_type = ?", StatusEnabled, MenuNodeTypePage).Order("code").Find(&menus).Error; err != nil {
		return principalComparisonState{}, repositoryError(err)
	}
	for _, menu := range menus {
		if _, deleted := deletedMenus[menu.ID]; !deleted {
			state.EnabledPages = append(state.EnabledPages, menu)
		}
	}
	var roleMenus []gormRoleMenu
	if err := db.Order("role_code, menu_code").Find(&roleMenus).Error; err != nil {
		return principalComparisonState{}, repositoryError(err)
	}
	enabledPageCodes := stringSet(menuCodes(state.EnabledPages))
	for _, row := range roleMenus {
		if _, active := state.RolesByCode[row.RoleCode]; !active {
			continue
		}
		if _, page := enabledPageCodes[row.MenuCode]; page {
			state.PersistedLeavesByRole[row.RoleCode] = append(state.PersistedLeavesByRole[row.RoleCode], row.MenuCode)
		}
	}
	var revisions []gormRoleMenuRevision
	if err := db.Find(&revisions).Error; err != nil {
		return principalComparisonState{}, repositoryError(err)
	}
	for _, revision := range revisions {
		state.RoleMenuRevisionByRole[revision.RoleCode] = revision.Revision
	}
	deletedOrganizations, err := deletedRecordIDs(db, "org-units")
	if err != nil {
		return principalComparisonState{}, err
	}
	var organizations []gormOrganization
	if err := db.Order("code").Find(&organizations).Error; err != nil {
		return principalComparisonState{}, repositoryError(err)
	}
	for _, organization := range organizations {
		if _, deleted := deletedOrganizations[organization.ID]; !deleted && organization.Status == StatusEnabled {
			state.OrganizationsByCode[organization.Code] = organization
		}
	}
	if db.Migrator().HasColumn(organizationsTable, "parent_code") && db.Migrator().HasColumn(organizationsTable, "area_code") {
		var rows []principalOrgScopeRow
		if err := db.Table(organizationsTable).Select("code", "parent_code", "area_code").Order("code").Find(&rows).Error; err != nil {
			return principalComparisonState{}, repositoryError(err)
		}
		for _, row := range rows {
			state.OrgParentByCode[row.Code] = strings.TrimSpace(row.ParentCode)
			state.OrgAreaByCode[row.Code] = strings.TrimSpace(row.AreaCode)
		}
	}
	if db.Migrator().HasTable("platform_area_codes") {
		var rows []principalAreaScopeRow
		if err := db.Table("platform_area_codes").Select("code", "parent_code").Where("status = ?", StatusEnabled).Order("code").Find(&rows).Error; err != nil {
			return principalComparisonState{}, repositoryError(err)
		}
		for _, row := range rows {
			state.AreaParentByCode[row.Code] = strings.TrimSpace(row.ParentCode)
		}
	}
	return state, nil
}

func intendedIdentityForPrincipal(state principalComparisonState, user gormUser) (intendedPrincipalIdentity, string) {
	for _, code := range state.Manifest.PlatformPrincipalAllowlist {
		if strings.TrimSpace(code) == user.Code {
			return intendedPrincipalIdentity{ScopeType: ScopePlatform, AreaCode: state.UserAreaByID[user.ID]}, ""
		}
	}
	orgCode := strings.TrimSpace(state.Manifest.TenantUserOrganizationMap[user.Code])
	organization, exists := state.OrganizationsByCode[orgCode]
	if orgCode == "" || !exists || organization.TenantCode == "" {
		return intendedPrincipalIdentity{}, "target-organization-required"
	}
	areaCode := strings.TrimSpace(state.OrgAreaByCode[orgCode])
	if areaCode == "" {
		areaCode = strings.TrimSpace(state.UserAreaByID[user.ID])
	}
	return intendedPrincipalIdentity{ScopeType: ScopeTenant, TenantCode: organization.TenantCode, OrgUnitCode: orgCode, AreaCode: areaCode}, ""
}

func legacyPrincipalSnapshot(state principalComparisonState, user gormUser) (PrincipalAuthorizationSnapshot, error) {
	identity, blocking := intendedIdentityForPrincipal(state, user)
	if blocking != "" {
		return PrincipalAuthorizationSnapshot{}, ErrRolePoolViolation
	}
	return legacyPrincipalSnapshotWithIdentity(state, user, identity), nil
}

func legacyPrincipalSnapshotWithIdentity(state principalComparisonState, user gormUser, identity intendedPrincipalIdentity) PrincipalAuthorizationSnapshot {
	roles := activeComparisonRoleCodes(state, state.AssignmentsByUserID[user.ID])
	return principalSnapshotForRoles(state, user, identity, roles, aggregateLegacyPrincipalPageLeaves(state, roles))
}

func targetPrincipalSnapshot(state principalComparisonState, user gormUser, mode PrincipalComparisonMode) (PrincipalAuthorizationSnapshot, error) {
	identity, blocking := intendedIdentityForPrincipal(state, user)
	if blocking != "" {
		return PrincipalAuthorizationSnapshot{}, ErrRolePoolViolation
	}
	return targetPrincipalSnapshotWithIdentity(state, user, identity, mode)
}

func targetPrincipalSnapshotWithIdentity(state principalComparisonState, user gormUser, identity intendedPrincipalIdentity, mode PrincipalComparisonMode) (PrincipalAuthorizationSnapshot, error) {
	assigned := state.AssignmentsByUserID[user.ID]
	if mode == PrincipalComparisonCandidate {
		var err error
		assigned, err = remediatedRoleCodes(user, assigned, state.Manifest)
		if err != nil {
			return PrincipalAuthorizationSnapshot{}, err
		}
	} else if mode != PrincipalComparisonPersisted {
		return PrincipalAuthorizationSnapshot{}, ErrInvalid
	}
	roles := activeComparisonRoleCodes(state, assigned)
	menus := candidatePageLeavesForRoles(state, roles)
	if mode == PrincipalComparisonPersisted {
		menus = persistedPageLeavesForRoles(state, roles)
	}
	return principalSnapshotForRoles(state, user, identity, roles, menus), nil
}

func principalSnapshotForRoles(state principalComparisonState, user gormUser, identity intendedPrincipalIdentity, roleCodes, menus []string) PrincipalAuthorizationSnapshot {
	allow, deny := effectivePrincipalPermissions(state, roleCodes)
	all, orgCodes, areaCodes, self := effectivePrincipalDataScope(state, identity, roleCodes)
	return canonicalPrincipalSnapshot(PrincipalAuthorizationSnapshot{
		UserCode: user.Code, ScopeType: identity.ScopeType, TenantCode: identity.TenantCode, OrgUnitCode: identity.OrgUnitCode,
		RoleCodes: roleCodes, AllowPermissions: allow, DeniedPermissions: deny, DataScopeAll: all,
		DataScopeOrgCodes: orgCodes, DataScopeAreaCodes: areaCodes, DataScopeSelf: self, MenuCodes: menus,
	})
}

func aggregateLegacyPrincipalPageLeaves(state principalComparisonState, roleCodes []string) []string {
	allow, deny := comparisonPolicyPatterns(state, roleCodes)
	policy := rbac.NewPolicySetWithDeny(allow, deny)
	return visiblePermissionBackedPages(state, policy)
}

func candidatePageLeavesForRoles(state principalComparisonState, roleCodes []string) []string {
	set := map[string]struct{}{}
	for _, roleCode := range roleCodes {
		allow, deny := comparisonPolicyPatterns(state, []string{roleCode})
		for _, code := range visiblePermissionBackedPages(state, rbac.NewPolicySetWithDeny(allow, deny)) {
			set[code] = struct{}{}
		}
	}
	return sortedSet(set)
}

func persistedPageLeavesForRoles(state principalComparisonState, roleCodes []string) []string {
	set := map[string]struct{}{}
	for _, roleCode := range roleCodes {
		for _, code := range state.PersistedLeavesByRole[roleCode] {
			set[code] = struct{}{}
		}
	}
	return sortedSet(set)
}

func remediatedRoleCodes(user gormUser, assignedRoleCodes []string, manifest MigrationManifest) ([]string, error) {
	roles := stringSet(assignedRoleCodes)
	seen := map[string]struct{}{}
	for _, remediation := range manifest.RolePoolConflictRemediations {
		if remediation.UserCode != user.Code {
			continue
		}
		if _, duplicate := seen[remediation.RoleCode]; duplicate {
			return nil, ErrRolePoolViolation
		}
		seen[remediation.RoleCode] = struct{}{}
		if _, assigned := roles[remediation.RoleCode]; !assigned {
			return nil, ErrRolePoolViolation
		}
		delete(roles, remediation.RoleCode)
		switch remediation.Action {
		case "remove-role":
			if remediation.ReplacementRoleCode != "" {
				return nil, ErrRolePoolViolation
			}
		case "replace-role":
			if !validCode(remediation.ReplacementRoleCode) || remediation.ReplacementRoleCode == remediation.RoleCode {
				return nil, ErrRolePoolViolation
			}
			roles[remediation.ReplacementRoleCode] = struct{}{}
		default:
			return nil, ErrRolePoolViolation
		}
	}
	return sortedSet(roles), nil
}

func compareLoadedPrincipals(state principalComparisonState, mode PrincipalComparisonMode) (PrincipalEquivalenceReport, error) {
	report := PrincipalEquivalenceReport{ActivePrincipals: len(state.Users), Differences: []PrincipalAuthorizationDifference{}}
	hashParts := make([]string, 0, len(state.Users))
	for _, user := range state.Users {
		identity, blocking := intendedIdentityForPrincipal(state, user)
		if blocking != "" {
			difference := PrincipalAuthorizationDifference{UserCode: user.Code, BlockingReasons: []string{blocking}}
			report.Differences = append(report.Differences, difference)
			hashParts = append(hashParts, user.Code+"|blocking|"+blocking)
			continue
		}
		legacy := legacyPrincipalSnapshotWithIdentity(state, user, identity)
		target, err := targetPrincipalSnapshotWithIdentity(state, user, identity, mode)
		if err != nil {
			return PrincipalEquivalenceReport{}, err
		}
		legacyHash := principalAuthorizationHash(legacy)
		targetHash := principalAuthorizationHash(target)
		hashParts = append(hashParts, user.Code+"|"+legacyHash+"|"+targetHash)
		if legacyHash == targetHash {
			report.Equivalent++
			continue
		}
		added, removed := setDifferenceCounts(legacy.MenuCodes, target.MenuCodes)
		report.Differences = append(report.Differences, PrincipalAuthorizationDifference{
			UserCode: user.Code, LegacyHash: legacyHash, TargetHash: targetHash,
			ChangedFields: changedPrincipalFields(legacy, target), AddedMenus: added, RemovedMenus: removed,
		})
	}
	sort.Strings(hashParts)
	digest := sha256.Sum256([]byte(strings.Join(hashParts, "\n")))
	report.ComparisonHash = hex.EncodeToString(digest[:])
	return report, nil
}

func canonicalPrincipalSnapshot(snapshot PrincipalAuthorizationSnapshot) PrincipalAuthorizationSnapshot {
	snapshot.UserCode = strings.TrimSpace(snapshot.UserCode)
	snapshot.TenantCode = strings.TrimSpace(snapshot.TenantCode)
	snapshot.OrgUnitCode = strings.TrimSpace(snapshot.OrgUnitCode)
	snapshot.RoleCodes = canonicalComparisonStrings(snapshot.RoleCodes)
	snapshot.AllowPermissions = canonicalComparisonStrings(snapshot.AllowPermissions)
	snapshot.DeniedPermissions = canonicalComparisonStrings(snapshot.DeniedPermissions)
	snapshot.DataScopeOrgCodes = canonicalComparisonStrings(snapshot.DataScopeOrgCodes)
	snapshot.DataScopeAreaCodes = canonicalComparisonStrings(snapshot.DataScopeAreaCodes)
	snapshot.MenuCodes = canonicalComparisonStrings(snapshot.MenuCodes)
	if snapshot.DataScopeAll {
		snapshot.DataScopeOrgCodes = []string{}
		snapshot.DataScopeAreaCodes = []string{}
		snapshot.DataScopeSelf = false
	}
	return snapshot
}

func principalAuthorizationHash(snapshot PrincipalAuthorizationSnapshot) string {
	encoded, _ := json.Marshal(canonicalPrincipalSnapshot(snapshot))
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func effectivePrincipalPermissions(state principalComparisonState, roleCodes []string) ([]string, []string) {
	allowPatterns, denyPatterns := comparisonPolicyPatterns(state, roleCodes)
	allowPolicy := rbac.NewPolicySet(allowPatterns)
	denyPolicy := rbac.NewPolicySet(denyPatterns)
	allow := make([]string, 0)
	deny := make([]string, 0)
	for _, permission := range state.EnabledPermissions {
		if denyPolicy.Allows(permission) {
			deny = append(deny, permission)
			continue
		}
		if allowPolicy.Allows(permission) {
			allow = append(allow, permission)
		}
	}
	return allow, deny
}

func comparisonPolicyPatterns(state principalComparisonState, roleCodes []string) ([]string, []string) {
	allow := make([]string, 0)
	deny := make([]string, 0)
	for _, roleCode := range roleCodes {
		allow = append(allow, state.AllowByRoleCode[roleCode]...)
		for _, permission := range rbac.ParsePermissionList(state.RoleValuesByCode[roleCode]["denyPermissions"]) {
			if policyPatternMatchesEnabledPermission(permission, state.EnabledPermissions) {
				deny = append(deny, permission)
			}
		}
	}
	return canonicalComparisonStrings(allow), canonicalComparisonStrings(deny)
}

func policyPatternMatchesEnabledPermission(pattern string, enabledPermissions []string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return false
	}
	policy := rbac.NewPolicySet([]string{pattern})
	for _, permission := range enabledPermissions {
		if policy.Allows(permission) {
			return true
		}
	}
	return false
}

func visiblePermissionBackedPages(state principalComparisonState, policy rbac.PolicySet) []string {
	enabled := stringSet(state.EnabledPermissions)
	result := make([]string, 0)
	for _, page := range state.EnabledPages {
		permission := strings.TrimSpace(page.LegacyPermission)
		if permission == "" {
			continue
		}
		if _, exists := enabled[permission]; exists && policy.Allows(permission) {
			result = append(result, page.Code)
		}
	}
	return canonicalComparisonStrings(result)
}

func effectivePrincipalDataScope(state principalComparisonState, identity intendedPrincipalIdentity, roleCodes []string) (bool, []string, []string, bool) {
	orgs := map[string]struct{}{}
	areas := map[string]struct{}{}
	self := false
	for _, roleCode := range roleCodes {
		values := state.RoleValuesByCode[roleCode]
		switch scope := strings.TrimSpace(values["dataScope"]); scope {
		case "", "all":
			return true, []string{}, []string{}, false
		case "current_org":
			addComparisonCode(orgs, identity.OrgUnitCode)
		case "current_and_children":
			addComparisonCodes(orgs, hierarchyCodes(identity.OrgUnitCode, state.OrgParentByCode)...)
		case "custom_orgs":
			addComparisonCodes(orgs, rbac.ParsePermissionList(values["dataScopeOrgCodes"])...)
		case "current_area":
			addComparisonCode(areas, identity.AreaCode)
		case "current_and_children_areas":
			addComparisonCodes(areas, hierarchyCodes(identity.AreaCode, state.AreaParentByCode)...)
		case "custom_areas":
			addComparisonCodes(areas, rbac.ParsePermissionList(values["dataScopeAreaCodes"])...)
		case "self":
			self = true
		}
	}
	return false, sortedSet(orgs), sortedSet(areas), self
}

func hierarchyCodes(root string, parentByCode map[string]string) []string {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil
	}
	seen := map[string]struct{}{root: {}}
	for {
		changed := false
		for code, parent := range parentByCode {
			if _, parentIncluded := seen[parent]; !parentIncluded {
				continue
			}
			if _, included := seen[code]; included {
				continue
			}
			seen[code] = struct{}{}
			changed = true
		}
		if !changed {
			return sortedSet(seen)
		}
	}
}

func activeComparisonRoleCodes(state principalComparisonState, roleCodes []string) []string {
	active := map[string]struct{}{}
	for _, roleCode := range roleCodes {
		if _, exists := state.RolesByCode[roleCode]; exists {
			active[roleCode] = struct{}{}
		}
	}
	return sortedSet(active)
}

func sortedComparisonRoleCodes(state principalComparisonState) []string {
	roles := make(map[string]struct{}, len(state.RolesByCode))
	for roleCode := range state.RolesByCode {
		roles[roleCode] = struct{}{}
	}
	return sortedSet(roles)
}

func changedPrincipalFields(left, right PrincipalAuthorizationSnapshot) []string {
	fields := make([]string, 0)
	if left.ScopeType != right.ScopeType {
		fields = append(fields, "scopeType")
	}
	if left.TenantCode != right.TenantCode {
		fields = append(fields, "tenantCode")
	}
	if left.OrgUnitCode != right.OrgUnitCode {
		fields = append(fields, "orgUnitCode")
	}
	if !equalStrings(left.RoleCodes, right.RoleCodes) {
		fields = append(fields, "roleCodes")
	}
	if !equalStrings(left.AllowPermissions, right.AllowPermissions) {
		fields = append(fields, "allowPermissions")
	}
	if !equalStrings(left.DeniedPermissions, right.DeniedPermissions) {
		fields = append(fields, "deniedPermissions")
	}
	if left.DataScopeAll != right.DataScopeAll || !equalStrings(left.DataScopeOrgCodes, right.DataScopeOrgCodes) ||
		!equalStrings(left.DataScopeAreaCodes, right.DataScopeAreaCodes) || left.DataScopeSelf != right.DataScopeSelf {
		fields = append(fields, "dataScope")
	}
	if !equalStrings(left.MenuCodes, right.MenuCodes) {
		fields = append(fields, "menuCodes")
	}
	return fields
}

func setDifferenceCounts(current, target []string) (int, int) {
	currentSet := stringSet(current)
	targetSet := stringSet(target)
	added := 0
	removed := 0
	for code := range targetSet {
		if _, exists := currentSet[code]; !exists {
			added++
		}
	}
	for code := range currentSet {
		if _, exists := targetSet[code]; !exists {
			removed++
		}
	}
	return added, removed
}

func menuCodes(menus []gormMenu) []string {
	result := make([]string, 0, len(menus))
	for _, menu := range menus {
		result = append(result, menu.Code)
	}
	return result
}

func canonicalComparisonStrings(values []string) []string {
	set := map[string]struct{}{}
	for _, value := range values {
		addComparisonCode(set, value)
	}
	return sortedSet(set)
}

func addComparisonCodes(set map[string]struct{}, values ...string) {
	for _, value := range values {
		addComparisonCode(set, value)
	}
}

func addComparisonCode(set map[string]struct{}, value string) {
	value = strings.TrimSpace(value)
	if value != "" {
		set[value] = struct{}{}
	}
}

func sortedSet(set map[string]struct{}) []string {
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
