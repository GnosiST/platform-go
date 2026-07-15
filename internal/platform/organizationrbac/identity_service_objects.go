package organizationrbac

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"platform-go/internal/platform/serviceobject"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type userOrganizationChangeSet struct {
	UserCode     string                      `json:"userCode"`
	OrgUnitCode  string                      `json:"orgUnitCode"`
	TenantCode   string                      `json:"tenantCode"`
	RoleCodes    []string                    `json:"roleCodes"`
	Remediations []RoleAssignmentRemediation `json:"remediations,omitempty"`
}

type roleStateOrGroupChangeSet struct {
	RoleCode        string                      `json:"roleCode"`
	Operation       string                      `json:"operation"`
	TargetGroupCode string                      `json:"targetGroupCode,omitempty"`
	TenantCode      string                      `json:"tenantCode"`
	Remediations    []RoleAssignmentRemediation `json:"remediations,omitempty"`
}

func (e *ServiceObjectExecutor) prepareUserOrganizationChange(ctx context.Context, plan serviceobject.DomainCommandPlan) (serviceobject.CommandResult, error) {
	actor := strings.TrimSpace(plan.Execution.Actor.Username)
	userCode, _ := plan.Arguments["userCode"].(string)
	orgUnitCode, _ := plan.Arguments["orgUnitCode"].(string)
	roleCodes, _ := plan.Arguments["roleCodes"].([]string)
	remediations := domainRemediations(plan.Arguments["remediations"])
	if actor == "" || userCode == "" || orgUnitCode == "" {
		return serviceobject.CommandResult{}, serviceobject.ErrValidation
	}
	impact, err := e.repository.PreviewUserOrganizationChange(ctx, userCode, orgUnitCode, roleCodes)
	if err != nil {
		return serviceobject.CommandResult{}, mapServiceObjectError(err)
	}
	if err := validateUserChangeRemediations(impact, remediations); err != nil {
		return serviceobject.CommandResult{}, mapServiceObjectError(err)
	}
	severity := "low"
	if len(impact.Conflicts) > 0 {
		severity = "high"
	} else if impact.CurrentOrgUnitCode != impact.TargetOrgUnitCode || !equalStrings(impact.CurrentRoleCodes, impact.TargetRoleCodes) {
		severity = "medium"
	}
	changeSet := userOrganizationChangeSet{
		UserCode: impact.UserCode, OrgUnitCode: impact.TargetOrgUnitCode, TenantCode: impact.TargetTenantCode,
		RoleCodes: impact.TargetRoleCodes, Remediations: remediations,
	}
	summary := organizationRoleGroupImpactSummary{
		Severity: severity, AffectedUsers: 1, ConflictCount: len(impact.Conflicts),
		CurrentGroupCount: len(impact.CurrentRoleCodes), TargetGroupCount: len(impact.TargetRoleCodes),
		Conflicts: impact.Conflicts,
	}
	return e.storeDomainPreview(ctx, plan, userOrganizationPreviewOperation, impact.ExpectedRevision, changeSet, summary)
}

func (e *ServiceObjectExecutor) prepareRoleStateOrGroupChange(ctx context.Context, plan serviceobject.DomainCommandPlan) (serviceobject.CommandResult, error) {
	actor := strings.TrimSpace(plan.Execution.Actor.Username)
	roleCode, _ := plan.Arguments["roleCode"].(string)
	operation, _ := plan.Arguments["operation"].(string)
	targetGroupCode, _ := plan.Arguments["targetGroupCode"].(string)
	remediations := domainRemediations(plan.Arguments["remediations"])
	if actor == "" || roleCode == "" || operation != roleOperationMove && operation != roleOperationDisable || operation == roleOperationMove && targetGroupCode == "" || operation == roleOperationDisable && targetGroupCode != "" {
		return serviceobject.CommandResult{}, serviceobject.ErrValidation
	}
	var (
		impact RoleStateOrGroupImpact
		err    error
	)
	if operation == roleOperationMove {
		impact, err = e.repository.PreviewRoleMove(ctx, roleCode, targetGroupCode)
	} else {
		impact, err = e.repository.PreviewRoleDisable(ctx, roleCode)
	}
	if err != nil {
		return serviceobject.CommandResult{}, mapServiceObjectError(err)
	}
	if err := validateConflictRemediationPlan(e.repository.db.WithContext(ctx), impact.Conflicts, remediations); err != nil {
		return serviceobject.CommandResult{}, mapServiceObjectError(err)
	}
	operationKey := roleDisablePreviewOperation
	if operation == roleOperationMove {
		operationKey = roleMovePreviewOperation
	}
	severity := "medium"
	if len(impact.Conflicts) > 0 {
		severity = "high"
	}
	changeSet := roleStateOrGroupChangeSet{
		RoleCode: impact.RoleCode, Operation: impact.Operation, TargetGroupCode: impact.TargetGroupCode,
		TenantCode: impact.TenantCode, Remediations: remediations,
	}
	summary := organizationRoleGroupImpactSummary{
		Severity: severity, AffectedUsers: impact.AffectedUsers, ConflictCount: len(impact.Conflicts),
		CurrentGroupCount: 1, TargetGroupCount: 1,
	}
	return e.storeDomainPreview(ctx, plan, operationKey, impact.ExpectedRevision, changeSet, summary)
}

func (e *ServiceObjectExecutor) applyUserOrganizationChange(ctx context.Context, plan serviceobject.DomainCommandPlan) (serviceobject.CommandResult, error) {
	return e.applyIdentityPreview(ctx, plan, userOrganizationPreviewOperation, func(tx *gorm.DB, preview gormOrganizationRBACPreview, actor string) (uint64, string, string, error) {
		var changeSet userOrganizationChangeSet
		if err := json.Unmarshal([]byte(preview.ChangeSetJSON), &changeSet); err != nil {
			return 0, "", "", serviceobject.ErrExecutionFailed
		}
		revision, err := (&GORMRepository{db: tx}).ChangeUserOrganization(ctx, ChangeUserOrganizationRequest{
			UserCode: changeSet.UserCode, OrgUnitCode: changeSet.OrgUnitCode, RoleCodes: changeSet.RoleCodes,
			ExpectedRevision: preview.ExpectedRevision, ActorID: actor, ChangedAt: e.now().UTC(),
		}, changeSet.Remediations)
		return revision, changeSet.TenantCode, changeSet.OrgUnitCode, mapServiceObjectError(err)
	})
}

func (e *ServiceObjectExecutor) applyRoleMove(ctx context.Context, plan serviceobject.DomainCommandPlan) (serviceobject.CommandResult, error) {
	return e.applyRolePreview(ctx, plan, roleMovePreviewOperation, roleOperationMove)
}

func (e *ServiceObjectExecutor) applyRoleDisable(ctx context.Context, plan serviceobject.DomainCommandPlan) (serviceobject.CommandResult, error) {
	return e.applyRolePreview(ctx, plan, roleDisablePreviewOperation, roleOperationDisable)
}

func (e *ServiceObjectExecutor) applyRolePreview(ctx context.Context, plan serviceobject.DomainCommandPlan, previewOperation, operation string) (serviceobject.CommandResult, error) {
	return e.applyIdentityPreview(ctx, plan, previewOperation, func(tx *gorm.DB, preview gormOrganizationRBACPreview, actor string) (uint64, string, string, error) {
		var changeSet roleStateOrGroupChangeSet
		if err := json.Unmarshal([]byte(preview.ChangeSetJSON), &changeSet); err != nil || changeSet.Operation != operation {
			return 0, "", "", serviceobject.ErrExecutionFailed
		}
		request := ChangeRoleRequest{
			RoleCode: changeSet.RoleCode, TargetGroupCode: changeSet.TargetGroupCode,
			ExpectedRevision: preview.ExpectedRevision, ActorID: actor, ChangedAt: e.now().UTC(),
		}
		var (
			revision uint64
			err      error
		)
		if operation == roleOperationMove {
			revision, err = (&GORMRepository{db: tx}).MoveRole(ctx, request, changeSet.Remediations)
		} else {
			revision, err = (&GORMRepository{db: tx}).DisableRole(ctx, request, changeSet.Remediations)
		}
		return revision, changeSet.TenantCode, "", mapServiceObjectError(err)
	})
}

type identityPreviewApply func(*gorm.DB, gormOrganizationRBACPreview, string) (revision uint64, tenantCode string, orgUnitCode string, err error)

func (e *ServiceObjectExecutor) applyIdentityPreview(ctx context.Context, plan serviceobject.DomainCommandPlan, operation string, apply identityPreviewApply) (serviceobject.CommandResult, error) {
	actor := strings.TrimSpace(plan.Execution.Actor.Username)
	previewID, _ := plan.Arguments["previewId"].(string)
	expectedRevision, ok := plan.Arguments["expectedRevision"].(int64)
	impactHash, _ := plan.Arguments["impactHash"].(string)
	if actor == "" || previewID == "" || !ok || expectedRevision < 0 || impactHash == "" {
		return serviceobject.CommandResult{}, serviceobject.ErrValidation
	}
	var result serviceobject.CommandResult
	err := e.repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		preview, storedResult, replay, err := e.lockIdentityPreview(tx, plan, operation, previewID, uint64(expectedRevision), impactHash)
		if err != nil {
			return err
		}
		if replay {
			result = storedResult
			return nil
		}
		revision, tenantCode, orgUnitCode, err := apply(tx, preview, actor)
		if err != nil {
			return err
		}
		var summary organizationRoleGroupImpactSummary
		if err := json.Unmarshal([]byte(preview.SummaryJSON), &summary); err != nil {
			return serviceobject.ErrExecutionFailed
		}
		if err := e.writeIdentityAudit(tx, plan.Definition.ID, actor, tenantCode, orgUnitCode, preview, revision, summary.ConflictCount); err != nil {
			return err
		}
		result = serviceobject.CommandResult{Values: map[string]any{"applied": true, "revision": int64(revision), "previewId": preview.ID}}
		return e.completeIdentityPreview(tx, preview.ID, revision)
	})
	if err != nil {
		return serviceobject.CommandResult{}, err
	}
	return result, nil
}

func (e *ServiceObjectExecutor) storeDomainPreview(ctx context.Context, plan serviceobject.DomainCommandPlan, operation string, expectedRevision uint64, changeSet any, summary organizationRoleGroupImpactSummary) (serviceobject.CommandResult, error) {
	actor := strings.TrimSpace(plan.Execution.Actor.Username)
	changeSetJSON, changeSetHash, err := canonicalHash(changeSet)
	if err != nil {
		return serviceobject.CommandResult{}, serviceobject.ErrExecutionFailed
	}
	scopeHash, err := serviceObjectScopeHash(actor, plan.TenantID, plan.Scope)
	if err != nil {
		return serviceobject.CommandResult{}, serviceobject.ErrExecutionFailed
	}
	impactHash := hashText(fmt.Sprintf("%d\n%s\n%s", expectedRevision, changeSetHash, scopeHash))
	summaryJSON, err := json.Marshal(summary)
	if err != nil {
		return serviceobject.CommandResult{}, serviceobject.ErrExecutionFailed
	}
	now := e.now().UTC()
	preview := gormOrganizationRBACPreview{
		ID: newOpaqueID(), Operation: operation, OwnerActor: actor, TenantCode: strings.TrimSpace(plan.TenantID),
		ScopeHash: scopeHash, ExpectedRevision: expectedRevision, ImpactHash: impactHash, ChangeSetHash: changeSetHash,
		ChangeSetJSON: string(changeSetJSON), SummaryJSON: string(summaryJSON), Status: organizationRoleGroupPreviewStatusPrepared,
		ExpiresAt: now.Add(e.ttl), CreatedAt: now, UpdatedAt: now,
	}
	if preview.ID == "" || e.repository.db.WithContext(ctx).Create(&preview).Error != nil {
		return serviceobject.CommandResult{}, serviceobject.ErrExecutionFailed
	}
	return serviceobject.CommandResult{Values: map[string]any{
		"previewId": preview.ID, "expectedRevision": int64(preview.ExpectedRevision),
		"impactHash": preview.ImpactHash, "expiresAt": preview.ExpiresAt.Format(time.RFC3339),
	}}, nil
}

func (e *ServiceObjectExecutor) lockIdentityPreview(tx *gorm.DB, plan serviceobject.DomainCommandPlan, operation, previewID string, expectedRevision uint64, impactHash string) (gormOrganizationRBACPreview, serviceobject.CommandResult, bool, error) {
	var preview gormOrganizationRBACPreview
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", previewID).Take(&preview).Error; err != nil {
		return gormOrganizationRBACPreview{}, serviceobject.CommandResult{}, false, serviceobject.ErrObjectUnavailable
	}
	actor := strings.TrimSpace(plan.Execution.Actor.Username)
	scopeHash, err := serviceObjectScopeHash(actor, plan.TenantID, plan.Scope)
	if err != nil || preview.OwnerActor != actor || preview.TenantCode != strings.TrimSpace(plan.TenantID) || preview.ScopeHash != scopeHash || preview.Operation != operation {
		return gormOrganizationRBACPreview{}, serviceobject.CommandResult{}, false, serviceobject.ErrObjectUnavailable
	}
	if preview.Status == organizationRoleGroupPreviewStatusApplied {
		var stored organizationRoleGroupAppliedResult
		if err := json.Unmarshal([]byte(preview.AppliedResultJSON), &stored); err != nil {
			return gormOrganizationRBACPreview{}, serviceobject.CommandResult{}, false, serviceobject.ErrExecutionFailed
		}
		return preview, serviceobject.CommandResult{Values: map[string]any{"applied": stored.Applied, "revision": stored.Revision, "previewId": stored.PreviewID}}, true, nil
	}
	if e.now().UTC().After(preview.ExpiresAt) || preview.ExpectedRevision != expectedRevision || preview.ImpactHash != impactHash {
		return gormOrganizationRBACPreview{}, serviceobject.CommandResult{}, false, serviceobject.ErrConflict
	}
	return preview, serviceobject.CommandResult{}, false, nil
}

func (e *ServiceObjectExecutor) writeIdentityAudit(tx *gorm.DB, action, actor, tenantCode, orgUnitCode string, preview gormOrganizationRBACPreview, revision uint64, conflictCount int) error {
	auditID := newOpaqueID()
	if auditID == "" {
		return serviceobject.ErrExecutionFailed
	}
	if err := tx.Create(&gormOrganizationRBACAuditEvent{
		ID: auditID, ActorType: "user", ActorCode: actor, TenantCode: tenantCode, OrgUnitCode: orgUnitCode,
		Action: action, BeforeRevision: preview.ExpectedRevision, AfterRevision: revision, ImpactHash: preview.ImpactHash,
		PreviewID: preview.ID, ConflictCount: conflictCount, CreatedAt: e.now().UTC(),
	}).Error; err != nil {
		return serviceobject.ErrExecutionFailed
	}
	return nil
}

func (e *ServiceObjectExecutor) completeIdentityPreview(tx *gorm.DB, previewID string, revision uint64) error {
	encoded, err := json.Marshal(organizationRoleGroupAppliedResult{Applied: true, Revision: int64(revision), PreviewID: previewID})
	if err != nil {
		return serviceobject.ErrExecutionFailed
	}
	update := tx.Model(&gormOrganizationRBACPreview{}).Where("id = ? AND status = ?", previewID, organizationRoleGroupPreviewStatusPrepared).
		Updates(map[string]any{"status": organizationRoleGroupPreviewStatusApplied, "applied_result_json": string(encoded), "updated_at": e.now().UTC()})
	if update.Error != nil || update.RowsAffected != 1 {
		return serviceobject.ErrConflict
	}
	return nil
}

func domainRemediations(value any) []RoleAssignmentRemediation {
	serviceRemediations, _ := value.([]serviceobject.RoleRemediation)
	result := make([]RoleAssignmentRemediation, 0, len(serviceRemediations))
	for _, remediation := range serviceRemediations {
		result = append(result, RoleAssignmentRemediation{
			UserCode: remediation.UserCode, RoleCode: remediation.RoleCode, Action: string(remediation.Action),
			ReplacementRoleCode: remediation.ReplacementRoleCode,
		})
	}
	return result
}
