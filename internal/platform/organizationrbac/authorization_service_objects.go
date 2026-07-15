package organizationrbac

import (
	"context"
	"encoding/json"
	"strings"

	"platform-go/internal/platform/serviceobject"

	"gorm.io/gorm"
)

type rolePermissionChangeSet struct {
	RoleCode                string   `json:"roleCode"`
	TenantCode              string   `json:"tenantCode"`
	CurrentPermissionCodes  []string `json:"currentPermissionCodes"`
	ProposedPermissionCodes []string `json:"proposedPermissionCodes"`
}

func (e *ServiceObjectExecutor) prepareRolePermissions(ctx context.Context, plan serviceobject.DomainCommandPlan) (serviceobject.CommandResult, error) {
	actor := strings.TrimSpace(plan.Execution.Actor.Username)
	roleCode, _ := plan.Arguments["roleCode"].(string)
	permissionCodes, _ := plan.Arguments["permissionCodes"].([]string)
	if actor == "" || !validCode(roleCode) {
		return serviceobject.CommandResult{}, serviceobject.ErrValidation
	}
	impact, err := e.repository.PreviewRolePermissions(ctx, roleCode, permissionCodes)
	if err != nil {
		return serviceobject.CommandResult{}, mapServiceObjectError(err)
	}
	severity := "low"
	if impact.AddedCount != 0 || impact.RemovedCount != 0 {
		severity = "medium"
	}
	return e.storeDomainPreview(ctx, plan, rolePermissionsPreviewOperation, impact.ExpectedRevision, rolePermissionChangeSet{
		RoleCode: roleCode, TenantCode: impact.TenantCode,
		CurrentPermissionCodes: impact.CurrentPermissionCodes, ProposedPermissionCodes: impact.ProposedPermissionCodes,
	}, organizationRoleGroupImpactSummary{
		Severity: severity, CurrentGroupCount: len(impact.CurrentPermissionCodes), TargetGroupCount: len(impact.ProposedPermissionCodes),
	})
}

func (e *ServiceObjectExecutor) applyRolePermissions(ctx context.Context, plan serviceobject.DomainCommandPlan) (serviceobject.CommandResult, error) {
	return e.applyIdentityPreview(ctx, plan, rolePermissionsPreviewOperation, func(tx *gorm.DB, preview gormOrganizationRBACPreview, actor string) (uint64, string, string, error) {
		var changeSet rolePermissionChangeSet
		if err := json.Unmarshal([]byte(preview.ChangeSetJSON), &changeSet); err != nil {
			return 0, "", "", serviceobject.ErrExecutionFailed
		}
		current, err := (&GORMRepository{db: tx}).PreviewRolePermissions(ctx, changeSet.RoleCode, changeSet.ProposedPermissionCodes)
		if err != nil || current.ExpectedRevision != preview.ExpectedRevision ||
			!equalStrings(current.CurrentPermissionCodes, changeSet.CurrentPermissionCodes) ||
			!equalStrings(current.ProposedPermissionCodes, changeSet.ProposedPermissionCodes) {
			return 0, "", "", serviceobject.ErrConflict
		}
		revision, tenantCode, err := (&GORMRepository{db: tx}).ReplaceRolePermissions(ctx, ReplaceRolePermissionsRequest{
			RoleCode: changeSet.RoleCode, PermissionCodes: changeSet.ProposedPermissionCodes,
			ExpectedRevision: preview.ExpectedRevision, ActorID: actor, ChangedAt: e.now().UTC(),
		})
		return revision, tenantCode, "", mapServiceObjectError(err)
	})
}
