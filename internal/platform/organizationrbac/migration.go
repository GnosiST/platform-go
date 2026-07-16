package organizationrbac

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type MigrationMode string

const (
	MigrationInventory MigrationMode = "inventory"
	MigrationVerify    MigrationMode = "verify"
	MigrationApply     MigrationMode = "apply"
	MigrationRollback  MigrationMode = "rollback"
)

type RoleGroupPlacement struct {
	ScopeType  ScopeType `json:"scopeType"`
	TenantCode string    `json:"tenantCode,omitempty"`
}

type MigrationManifest struct {
	Version                         string                        `json:"version"`
	RoleGroupScopeTenantMap         map[string]RoleGroupPlacement `json:"roleGroupScopeTenantMap"`
	OrphanRoleGroupMap              map[string]string             `json:"orphanRoleGroupMap"`
	TenantUserOrganizationMap       map[string]string             `json:"tenantUserOrganizationMap"`
	OrganizationRoleGroupBindingMap map[string][]string           `json:"organizationRoleGroupBindingMap"`
	PlatformPrincipalAllowlist      []string                      `json:"platformPrincipalAllowlist"`
	RolePoolConflictRemediations    []RoleAssignmentRemediation   `json:"rolePoolConflictRemediations"`
}

type MigrationEvidence struct {
	RunID         string
	ActorID       string
	Reason        string
	ApprovalRef   string
	BackupURI     string
	BackupSHA256  string
	CheckpointRef string
	AppliedAt     time.Time
}

type MigrationConflict struct {
	Kind   string `json:"kind"`
	Code   string `json:"code"`
	Detail string `json:"detail"`
}

type MigrationReport struct {
	Mode         MigrationMode       `json:"mode"`
	Status       string              `json:"status"`
	ManifestHash string              `json:"manifestHash"`
	RunID        string              `json:"runId,omitempty"`
	Conflicts    []MigrationConflict `json:"conflicts,omitempty"`
	Cutover      *CutoverReport      `json:"cutover,omitempty"`
}

type gormOrganizationRBACMigrationRun struct {
	RunID         string    `gorm:"column:run_id;size:191;primaryKey"`
	ManifestHash  string    `gorm:"column:manifest_hash;size:64;index;not null"`
	Status        string    `gorm:"column:status;size:32;index;not null"`
	ActorID       string    `gorm:"column:actor_id;size:191;not null"`
	Reason        string    `gorm:"column:reason;size:512;not null"`
	ApprovalRef   string    `gorm:"column:approval_ref;size:512;not null"`
	BackupURI     string    `gorm:"column:backup_uri;size:1024;not null"`
	BackupSHA256  string    `gorm:"column:backup_sha256;size:64;not null"`
	CheckpointRef string    `gorm:"column:checkpoint_ref;size:1024;not null"`
	CreatedAt     time.Time `gorm:"column:created_at;not null"`
	UpdatedAt     time.Time `gorm:"column:updated_at;not null"`
}

type gormOrganizationRBACMigrationConflict struct {
	RunID    string `gorm:"column:run_id;size:191;primaryKey"`
	Sequence int    `gorm:"column:sequence;primaryKey"`
	Kind     string `gorm:"column:kind;size:64;index;not null"`
	Code     string `gorm:"column:code;size:191;not null"`
	Detail   string `gorm:"column:detail;size:1024;not null"`
}

func (gormOrganizationRBACMigrationRun) TableName() string {
	return "platform_organization_rbac_migration_runs"
}
func (gormOrganizationRBACMigrationConflict) TableName() string {
	return "platform_organization_rbac_migration_conflicts"
}

func (r *GORMRepository) RunMigration(ctx context.Context, mode MigrationMode, manifest MigrationManifest, evidence MigrationEvidence) (MigrationReport, error) {
	manifestHash, err := validateMigrationManifest(manifest)
	if err != nil {
		return MigrationReport{}, err
	}
	// Resolve a replay by run ID before inventory or candidate comparison. A valid
	// run ID is enough to inspect the immutable record; malformed evidence must
	// fail against an existing applied run without re-entering migration logic.
	if mode == MigrationApply && validCode(evidence.RunID) {
		if report, replayed, err := r.replayAppliedMigration(ctx, manifestHash, evidence); replayed || err != nil {
			return report, err
		}
	}
	conflicts, err := r.inventoryMigration(ctx, manifest)
	if err != nil {
		return MigrationReport{}, err
	}
	conflictRunID := migrationConflictRunID(mode, manifestHash, evidence.RunID)
	if err := r.persistMigrationConflicts(ctx, conflictRunID, conflicts); err != nil {
		return MigrationReport{}, err
	}
	report := MigrationReport{Mode: mode, ManifestHash: manifestHash, RunID: conflictRunID, Conflicts: conflicts}
	switch mode {
	case MigrationInventory:
		report.Status = "inventoried"
		return report, nil
	case MigrationVerify:
		if len(conflicts) > 0 {
			report.Status = "blocked"
			return report, ErrRolePoolViolation
		}
		comparison, compareErr := r.CompareAllActivePrincipals(ctx, manifest, PrincipalComparisonCandidate)
		if compareErr != nil || len(comparison.Differences) != 0 {
			report.Status = "blocked"
			if compareErr != nil {
				return report, compareErr
			}
			return report, ErrRolePoolViolation
		}
		report.Status = "verified"
		return report, nil
	case MigrationApply:
		if len(conflicts) > 0 {
			report.Status = "blocked"
			return report, ErrRolePoolViolation
		}
		if !validMigrationEvidence(evidence) {
			return MigrationReport{}, &ValidationError{Field: "migrationEvidence", Reason: "approval, backup and checkpoint evidence are required"}
		}
		cutover, err := r.applyMigration(ctx, manifest, manifestHash, evidence)
		if err != nil {
			return MigrationReport{}, err
		}
		report.Status = "applied"
		report.Cutover = &cutover
		return report, nil
	case MigrationRollback:
		if !validMigrationEvidence(evidence) {
			return MigrationReport{}, &ValidationError{Field: "migrationEvidence", Reason: "approval, backup and checkpoint evidence are required"}
		}
		if err := r.recordRollbackRequired(ctx, manifestHash, evidence); err != nil {
			return MigrationReport{}, err
		}
		report.Status = "external-checkpoint-restore-required"
		return report, nil
	default:
		return MigrationReport{}, &ValidationError{Field: "mode", Reason: "must be inventory, verify, apply or rollback"}
	}
}

func (r *GORMRepository) replayAppliedMigration(ctx context.Context, manifestHash string, evidence MigrationEvidence) (MigrationReport, bool, error) {
	if !r.ready(ctx) {
		return MigrationReport{}, false, ErrRepositoryFailed
	}
	var existing gormOrganizationRBACMigrationRun
	err := r.db.WithContext(ctx).Where("run_id = ?", evidence.RunID).Take(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return MigrationReport{}, false, nil
	}
	if err != nil {
		return MigrationReport{}, false, repositoryError(err)
	}
	if existing.Status != "applied" || !migrationRunMatches(existing, manifestHash, evidence) {
		return MigrationReport{}, true, &ValidationError{Field: "migrationEvidence.runId", Reason: "already belongs to a different immutable migration execution"}
	}
	cutover, err := r.ValidateCutover(ctx)
	if err != nil {
		return MigrationReport{}, true, err
	}
	return MigrationReport{
		Mode: MigrationApply, Status: "applied", ManifestHash: manifestHash, RunID: evidence.RunID, Cutover: &cutover,
	}, true, nil
}

func validateMigrationManifest(manifest MigrationManifest) (string, error) {
	if strings.TrimSpace(manifest.Version) == "" || manifest.RoleGroupScopeTenantMap == nil || manifest.OrphanRoleGroupMap == nil ||
		manifest.TenantUserOrganizationMap == nil || manifest.OrganizationRoleGroupBindingMap == nil || manifest.PlatformPrincipalAllowlist == nil ||
		manifest.RolePoolConflictRemediations == nil {
		return "", &ValidationError{Field: "manifest", Reason: "all required migration mappings must be present"}
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		return "", ErrInvalid
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func validMigrationEvidence(evidence MigrationEvidence) bool {
	return validCode(evidence.RunID) && validCode(evidence.ActorID) && strings.TrimSpace(evidence.Reason) != "" &&
		strings.TrimSpace(evidence.ApprovalRef) != "" && strings.TrimSpace(evidence.BackupURI) != "" &&
		len(strings.TrimSpace(evidence.BackupSHA256)) == 64 && strings.TrimSpace(evidence.CheckpointRef) != "" && !evidence.AppliedAt.IsZero()
}

func (r *GORMRepository) inventoryMigration(ctx context.Context, manifest MigrationManifest) ([]MigrationConflict, error) {
	if !r.ready(ctx) {
		return nil, ErrRepositoryFailed
	}
	db := r.db.WithContext(ctx)
	conflicts := make([]MigrationConflict, 0)
	var groups []gormRoleGroup
	var roles []gormRole
	var users []gormUser
	var organizations []gormOrganization
	var assignments []gormUserRole
	var lifecycle []gormResourceLifecycle
	if err := db.Find(&groups).Error; err != nil {
		return nil, repositoryError(err)
	}
	if err := db.Find(&roles).Error; err != nil {
		return nil, repositoryError(err)
	}
	if err := db.Find(&users).Error; err != nil {
		return nil, repositoryError(err)
	}
	if err := db.Find(&organizations).Error; err != nil {
		return nil, repositoryError(err)
	}
	if err := db.Find(&assignments).Error; err != nil {
		return nil, repositoryError(err)
	}
	if err := db.Find(&lifecycle).Error; err != nil {
		return nil, repositoryError(err)
	}
	deleted := make(map[string]struct{}, len(lifecycle))
	for _, row := range lifecycle {
		if strings.TrimSpace(row.DeletedAt) != "" {
			deleted[row.Resource+"\x00"+row.RecordID] = struct{}{}
		}
	}
	organizationTenants := map[string]string{}
	organizationMap := make(map[string]Organization, len(organizations))
	for _, organization := range organizations {
		organizationTenants[organization.Code] = organization.TenantCode
		_, isDeleted := deleted["org-units\x00"+organization.ID]
		organizationMap[organization.Code] = organizationFromGORM(organization, isDeleted)
	}
	groupCodes := map[string]struct{}{}
	groupMap := make(map[string]RoleGroup, len(groups))
	for _, group := range groups {
		groupCodes[group.Code] = struct{}{}
		placement, exists := manifest.RoleGroupScopeTenantMap[group.Code]
		if !exists || ValidateRoleGroup(RoleGroup{Code: group.Code, ScopeType: placement.ScopeType, TenantCode: placement.TenantCode}) != nil {
			conflicts = append(conflicts, MigrationConflict{Kind: "role-group-placement", Code: group.Code, Detail: "explicit valid scope and tenant mapping is required"})
		}
		_, isDeleted := deleted["role-groups\x00"+group.ID]
		groupMap[group.Code] = RoleGroup{
			Code: group.Code, Name: group.Name, ScopeType: placement.ScopeType, TenantCode: placement.TenantCode,
			Status: group.Status, Deleted: isDeleted, Revision: group.Revision,
		}
	}
	roleMap := make(map[string]Role, len(roles))
	for _, role := range roles {
		groupCode := strings.TrimSpace(role.GroupCode)
		if mapped := strings.TrimSpace(manifest.OrphanRoleGroupMap[role.Code]); mapped != "" {
			groupCode = mapped
		}
		if _, exists := groupCodes[groupCode]; !exists {
			conflicts = append(conflicts, MigrationConflict{Kind: "orphan-role", Code: role.Code, Detail: "exactly one existing role group is required"})
		}
		_, isDeleted := deleted["roles\x00"+role.ID]
		roleMap[role.Code] = Role{Code: role.Code, GroupCode: groupCode, Status: role.Status, Deleted: isDeleted}
	}
	platformUsers := map[string]struct{}{}
	for _, code := range manifest.PlatformPrincipalAllowlist {
		platformUsers[strings.TrimSpace(code)] = struct{}{}
	}
	for _, user := range users {
		if _, platform := platformUsers[user.Code]; platform {
			continue
		}
		orgCode := strings.TrimSpace(manifest.TenantUserOrganizationMap[user.Code])
		if _, exists := organizationTenants[orgCode]; !exists {
			conflicts = append(conflicts, MigrationConflict{Kind: "user-organization", Code: user.Code, Detail: "explicit existing primary organization is required"})
		}
	}
	for orgCode, bindingCodes := range manifest.OrganizationRoleGroupBindingMap {
		tenant, exists := organizationTenants[orgCode]
		if !exists {
			conflicts = append(conflicts, MigrationConflict{Kind: "organization-binding", Code: orgCode, Detail: "organization does not exist"})
			continue
		}
		for _, groupCode := range bindingCodes {
			placement, exists := manifest.RoleGroupScopeTenantMap[groupCode]
			if !exists || placement.ScopeType != ScopeTenant || placement.TenantCode != tenant {
				conflicts = append(conflicts, MigrationConflict{Kind: "organization-binding", Code: orgCode + ":" + groupCode, Detail: "binding must reference a same-tenant tenant role group"})
			}
		}
	}
	conflicts = append(conflicts, migrationRolePoolConflicts(users, assignments, organizationMap, groupMap, roleMap, manifest)...)
	sort.Slice(conflicts, func(i, j int) bool {
		if conflicts[i].Kind != conflicts[j].Kind {
			return conflicts[i].Kind < conflicts[j].Kind
		}
		return conflicts[i].Code < conflicts[j].Code
	})
	return conflicts, nil
}

func migrationRolePoolConflicts(
	users []gormUser,
	assignments []gormUserRole,
	organizations map[string]Organization,
	groups map[string]RoleGroup,
	roles map[string]Role,
	manifest MigrationManifest,
) []MigrationConflict {
	platformUsers := make(map[string]struct{}, len(manifest.PlatformPrincipalAllowlist))
	for _, code := range manifest.PlatformPrincipalAllowlist {
		platformUsers[strings.TrimSpace(code)] = struct{}{}
	}
	assignmentsByUser := make(map[string][]string)
	for _, assignment := range assignments {
		assignmentsByUser[assignment.UserID] = append(assignmentsByUser[assignment.UserID], assignment.RoleCode)
	}
	raw := make(map[string]MigrationConflict)
	allowedByUser := make(map[string]map[string]struct{}, len(users))
	remediationEligible := make(map[string]bool, len(users))
	for _, user := range users {
		allowed := make(map[string]struct{})
		if _, platform := platformUsers[user.Code]; platform {
			remediationEligible[user.Code] = true
			for roleCode, role := range roles {
				group, exists := groups[role.GroupCode]
				if exists && role.Enabled() && group.Enabled() && group.ScopeType == ScopePlatform && ValidateRoleGroup(group) == nil {
					allowed[roleCode] = struct{}{}
				}
			}
		} else {
			organizationCode := strings.TrimSpace(manifest.TenantUserOrganizationMap[user.Code])
			organization := organizations[organizationCode]
			_, remediationEligible[user.Code] = manifest.OrganizationRoleGroupBindingMap[organizationCode]
			if organization.Enabled() {
				eligibleGroups := make(map[string]struct{})
				for _, groupCode := range manifest.OrganizationRoleGroupBindingMap[organization.Code] {
					group, exists := groups[groupCode]
					if exists && group.Enabled() && group.ScopeType == ScopeTenant && group.TenantCode == organization.TenantCode && ValidateRoleGroup(group) == nil {
						eligibleGroups[groupCode] = struct{}{}
					}
				}
				for roleCode, role := range roles {
					if _, eligible := eligibleGroups[role.GroupCode]; eligible && role.Enabled() {
						allowed[roleCode] = struct{}{}
					}
				}
			}
		}
		allowedByUser[user.Code] = allowed
		for _, roleCode := range assignmentsByUser[user.ID] {
			if _, allowed := allowed[roleCode]; !allowed {
				key := user.Code + "\x00" + roleCode
				raw[key] = MigrationConflict{Kind: "role-pool-assignment", Code: user.Code + ":" + roleCode, Detail: "assigned role is outside the target organization or platform role pool"}
			}
		}
	}
	resolved := make(map[string]struct{}, len(manifest.RolePoolConflictRemediations))
	seenRemediations := make(map[string]struct{}, len(manifest.RolePoolConflictRemediations))
	conflicts := make([]MigrationConflict, 0, len(raw))
	for _, remediation := range manifest.RolePoolConflictRemediations {
		key := remediation.UserCode + "\x00" + remediation.RoleCode
		_, targetExists := raw[key]
		_, duplicate := seenRemediations[key]
		seenRemediations[key] = struct{}{}
		valid := targetExists && remediationEligible[remediation.UserCode] &&
			validCode(remediation.UserCode) && validCode(remediation.RoleCode) && !duplicate
		switch remediation.Action {
		case "remove-role":
			valid = valid && remediation.ReplacementRoleCode == ""
		case "replace-role":
			_, replacementAllowed := allowedByUser[remediation.UserCode][remediation.ReplacementRoleCode]
			valid = valid && validCode(remediation.ReplacementRoleCode) && replacementAllowed
		default:
			valid = false
		}
		if valid {
			resolved[key] = struct{}{}
			continue
		}
		conflicts = append(conflicts, MigrationConflict{
			Kind: "role-pool-remediation", Code: remediation.UserCode + ":" + remediation.RoleCode,
			Detail: "remediation must target one role-pool conflict and remove or replace it with an allowed role",
		})
	}
	for key, conflict := range raw {
		if _, ok := resolved[key]; !ok {
			conflicts = append(conflicts, conflict)
		}
	}
	return conflicts
}

func migrationConflictRunID(mode MigrationMode, manifestHash, requested string) string {
	if validCode(requested) {
		return requested
	}
	return string(mode) + "-" + manifestHash
}

func (r *GORMRepository) persistMigrationConflicts(ctx context.Context, runID string, conflicts []MigrationConflict) error {
	if !r.ready(ctx) || !validCode(runID) {
		return ErrRepositoryFailed
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("run_id = ?", runID).Delete(&gormOrganizationRBACMigrationConflict{}).Error; err != nil {
			return err
		}
		for index, conflict := range conflicts {
			if err := tx.Create(&gormOrganizationRBACMigrationConflict{
				RunID: runID, Sequence: index + 1, Kind: conflict.Kind, Code: conflict.Code, Detail: conflict.Detail,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	return repositoryError(err)
}

func (r *GORMRepository) applyMigration(ctx context.Context, manifest MigrationManifest, manifestHash string, evidence MigrationEvidence) (CutoverReport, error) {
	var cutover CutoverReport
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txRepository := &GORMRepository{db: tx}
		candidate, err := txRepository.CompareAllActivePrincipals(ctx, manifest, PrincipalComparisonCandidate)
		if err != nil {
			return err
		}
		if len(candidate.Differences) != 0 {
			return ErrRolePoolViolation
		}
		run := gormOrganizationRBACMigrationRun{
			RunID: evidence.RunID, ManifestHash: manifestHash, Status: "applying", ActorID: evidence.ActorID,
			Reason: evidence.Reason, ApprovalRef: evidence.ApprovalRef, BackupURI: evidence.BackupURI,
			BackupSHA256: evidence.BackupSHA256, CheckpointRef: evidence.CheckpointRef,
			CreatedAt: evidence.AppliedAt.UTC(), UpdatedAt: evidence.AppliedAt.UTC(),
		}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&run)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			var existing gormOrganizationRBACMigrationRun
			if err := tx.Where("run_id = ?", evidence.RunID).Take(&existing).Error; err != nil {
				return err
			}
			if existing.Status != "applied" || !migrationRunMatches(existing, manifestHash, evidence) {
				return &ValidationError{Field: "migrationEvidence.runId", Reason: "already belongs to a different immutable migration execution"}
			}
			var err error
			cutover, err = txRepository.ValidateCutover(ctx)
			return err
		}
		var groups []gormRoleGroup
		if err := tx.Find(&groups).Error; err != nil {
			return err
		}
		for _, group := range groups {
			placement := manifest.RoleGroupScopeTenantMap[group.Code]
			values, err := mergeValuesJSON(group.ValuesJSON, map[string]string{"scopeType": string(placement.ScopeType), "tenantCode": placement.TenantCode, "parentCode": ""})
			if err != nil {
				return err
			}
			if err := tx.Model(&gormRoleGroup{}).Where("id = ?", group.ID).Updates(map[string]any{
				"scope_type": string(placement.ScopeType), "tenant_code": placement.TenantCode, "parent_code": "", "values_json": values,
			}).Error; err != nil {
				return err
			}
		}
		var roles []gormRole
		if err := tx.Find(&roles).Error; err != nil {
			return err
		}
		for _, role := range roles {
			groupCode := role.GroupCode
			if mapped := strings.TrimSpace(manifest.OrphanRoleGroupMap[role.Code]); mapped != "" {
				groupCode = mapped
			}
			values, err := mergeValuesJSON(role.ValuesJSON, map[string]string{"groupCode": groupCode})
			if err != nil {
				return err
			}
			if err := tx.Model(&gormRole{}).Where("id = ?", role.ID).Updates(map[string]any{"group_code": groupCode, "values_json": values}).Error; err != nil {
				return err
			}
		}
		platformUsers := map[string]struct{}{}
		for _, code := range manifest.PlatformPrincipalAllowlist {
			platformUsers[strings.TrimSpace(code)] = struct{}{}
		}
		var users []gormUser
		if err := tx.Find(&users).Error; err != nil {
			return err
		}
		for _, user := range users {
			scopeType, tenantCode, orgUnitCode := ScopeTenant, "", strings.TrimSpace(manifest.TenantUserOrganizationMap[user.Code])
			if _, platform := platformUsers[user.Code]; platform {
				scopeType, orgUnitCode = ScopePlatform, ""
			} else {
				var org gormOrganization
				if err := tx.Where("code = ?", orgUnitCode).Take(&org).Error; err != nil {
					return err
				}
				tenantCode = org.TenantCode
			}
			values, err := mergeValuesJSON(user.ValuesJSON, map[string]string{"scopeType": string(scopeType), "tenantCode": tenantCode, "orgUnitCode": orgUnitCode})
			if err != nil {
				return err
			}
			if err := tx.Model(&gormUser{}).Where("id = ?", user.ID).Updates(map[string]any{
				"scope_type": string(scopeType), "tenant_code": tenantCode, "org_unit_code": orgUnitCode, "values_json": values,
			}).Error; err != nil {
				return err
			}
		}
		var existingBindings []gormOrgUnitRoleGroup
		if err := tx.Find(&existingBindings).Error; err != nil {
			return err
		}
		changedOrganizations := make(map[string]struct{}, len(existingBindings)+len(manifest.OrganizationRoleGroupBindingMap))
		for _, binding := range existingBindings {
			changedOrganizations[binding.OrgUnitCode] = struct{}{}
		}
		for orgCode := range manifest.OrganizationRoleGroupBindingMap {
			changedOrganizations[orgCode] = struct{}{}
		}
		organizationCodes := make([]string, 0, len(changedOrganizations))
		for orgCode := range changedOrganizations {
			organizationCodes = append(organizationCodes, orgCode)
		}
		sort.Strings(organizationCodes)
		bindingRevisions := make(map[string]uint64, len(organizationCodes))
		for _, orgCode := range organizationCodes {
			current, err := loadOrgUnitRoleGroupRevision(tx, orgCode)
			if err != nil {
				return err
			}
			if current == ^uint64(0) {
				return &ValidationError{Field: "organizationRoleGroupBinding.revision", Reason: "cannot advance beyond maximum revision"}
			}
			next := current + 1
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "org_unit_code"}},
				DoUpdates: clause.Assignments(map[string]any{"revision": next, "updated_at": evidence.AppliedAt.UTC()}),
			}).Create(&gormOrgUnitRoleGroupRevision{OrgUnitCode: orgCode, Revision: next, UpdatedAt: evidence.AppliedAt.UTC()}).Error; err != nil {
				return err
			}
			bindingRevisions[orgCode] = next
		}
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&gormOrgUnitRoleGroup{}).Error; err != nil {
			return err
		}
		for orgCode, groupCodes := range manifest.OrganizationRoleGroupBindingMap {
			codes, err := canonicalCodes(groupCodes)
			if err != nil {
				return err
			}
			for _, groupCode := range codes {
				if err := tx.Create(&gormOrgUnitRoleGroup{OrgUnitCode: orgCode, RoleGroupCode: groupCode, Revision: bindingRevisions[orgCode], ActorID: evidence.ActorID, CreatedAt: evidence.AppliedAt.UTC(), UpdatedAt: evidence.AppliedAt.UTC()}).Error; err != nil {
					return err
				}
			}
		}
		for orgCode := range manifest.OrganizationRoleGroupBindingMap {
			organization, groups, roles, bindings, err := (&GORMRepository{db: tx}).loadOrganizationState(tx, orgCode, nil)
			if err != nil {
				return err
			}
			pool, err := EffectiveRolePool(organization, groups, roles, bindings)
			if err != nil {
				return err
			}
			orgRemediations := make([]RoleAssignmentRemediation, 0)
			for _, remediation := range manifest.RolePoolConflictRemediations {
				var user gormUser
				if err := tx.Where("code = ?", remediation.UserCode).Take(&user).Error; err == nil && user.OrgUnitCode == orgCode {
					orgRemediations = append(orgRemediations, remediation)
				}
			}
			if err := applyRoleAssignmentRemediations(tx, organization, pool, orgRemediations); err != nil {
				return err
			}
		}
		if err := applyPlatformMigrationRemediations(tx, platformUsers, manifest.RolePoolConflictRemediations); err != nil {
			return err
		}
		state, err := loadPrincipalComparisonState(tx, manifest, PrincipalComparisonCandidate)
		if err != nil {
			return err
		}
		for _, roleCode := range sortedComparisonRoleCodes(state) {
			if _, err := replaceRoleMenusTx(tx, ReplaceRoleMenusRequest{
				RoleCode: roleCode, MenuCodes: candidatePageLeavesForRoles(state, []string{roleCode}),
				ExpectedRevision: state.RoleMenuRevisionByRole[roleCode], ActorID: evidence.ActorID, ChangedAt: evidence.AppliedAt,
			}, false); err != nil {
				return err
			}
		}
		persisted, err := txRepository.CompareAllActivePrincipals(ctx, manifest, PrincipalComparisonPersisted)
		if err != nil {
			return err
		}
		if len(persisted.Differences) != 0 {
			return ErrRolePoolViolation
		}
		cutover, err = txRepository.ValidateCutover(ctx)
		if err != nil {
			return err
		}
		if _, err := bumpGlobalRevision(tx); err != nil {
			return err
		}
		result = tx.Model(&gormOrganizationRBACMigrationRun{}).Where("run_id = ? AND status = ?", evidence.RunID, "applying").Updates(map[string]any{"status": "applied", "updated_at": evidence.AppliedAt.UTC()})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrRepositoryFailed
		}
		return nil
	})
	if err != nil {
		return CutoverReport{}, repositoryError(err)
	}
	return cutover, nil
}

func (r *GORMRepository) recordRollbackRequired(ctx context.Context, manifestHash string, evidence MigrationEvidence) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		run := gormOrganizationRBACMigrationRun{
			RunID: evidence.RunID, ManifestHash: manifestHash, Status: "external-checkpoint-restore-required", ActorID: evidence.ActorID,
			Reason: evidence.Reason, ApprovalRef: evidence.ApprovalRef, BackupURI: evidence.BackupURI, BackupSHA256: evidence.BackupSHA256,
			CheckpointRef: evidence.CheckpointRef, CreatedAt: evidence.AppliedAt.UTC(), UpdatedAt: evidence.AppliedAt.UTC(),
		}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&run)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 1 {
			return nil
		}
		var existing gormOrganizationRBACMigrationRun
		if err := tx.Where("run_id = ?", evidence.RunID).Take(&existing).Error; err != nil {
			return err
		}
		if existing.Status != run.Status || !migrationRunMatches(existing, manifestHash, evidence) {
			return &ValidationError{Field: "migrationEvidence.runId", Reason: "already belongs to a different immutable migration execution"}
		}
		return nil
	})
	return repositoryError(err)
}

func migrationRunMatches(existing gormOrganizationRBACMigrationRun, manifestHash string, evidence MigrationEvidence) bool {
	return existing.ManifestHash == manifestHash && existing.ActorID == evidence.ActorID && existing.Reason == evidence.Reason &&
		existing.ApprovalRef == evidence.ApprovalRef && existing.BackupURI == evidence.BackupURI &&
		existing.BackupSHA256 == evidence.BackupSHA256 && existing.CheckpointRef == evidence.CheckpointRef
}

func applyPlatformMigrationRemediations(tx *gorm.DB, platformUsers map[string]struct{}, remediations []RoleAssignmentRemediation) error {
	for _, remediation := range remediations {
		if _, platform := platformUsers[remediation.UserCode]; !platform {
			continue
		}
		var user gormUser
		if err := tx.Where("code = ? AND scope_type = ? AND tenant_code = '' AND org_unit_code = ''", remediation.UserCode, string(ScopePlatform)).Take(&user).Error; err != nil {
			return repositoryError(err)
		}
		switch remediation.Action {
		case "remove-role":
			if remediation.ReplacementRoleCode != "" {
				return ErrInvalid
			}
		case "replace-role":
			if !validCode(remediation.ReplacementRoleCode) || remediation.ReplacementRoleCode == remediation.RoleCode {
				return ErrInvalid
			}
		default:
			return ErrInvalid
		}
		result := tx.Where("user_id = ? AND role_code = ?", user.ID, remediation.RoleCode).Delete(&gormUserRole{})
		if result.Error != nil {
			return repositoryError(result.Error)
		}
		if result.RowsAffected != 1 {
			return ErrRolePoolViolation
		}
		if remediation.Action == "replace-role" {
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&gormUserRole{UserID: user.ID, RoleCode: remediation.ReplacementRoleCode}).Error; err != nil {
				return repositoryError(err)
			}
		}
	}
	return nil
}

func mergeValuesJSON(raw string, updates map[string]string) (string, error) {
	values := map[string]string{}
	if strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &values); err != nil || values == nil {
			return "", &ValidationError{Field: "valuesJSON", Reason: "must be a JSON object containing only string values"}
		}
	}
	for key, value := range updates {
		values[key] = value
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return "", ErrInvalid
	}
	return string(encoded), nil
}

func migrationError(err error) error {
	if err == nil || errors.Is(err, ErrInvalid) || errors.Is(err, ErrRolePoolViolation) {
		return err
	}
	return fmt.Errorf("%w: migration", ErrRepositoryFailed)
}
