package organizationrbac

import (
	"context"
	"encoding/json"
	"strings"

	"platform-go/internal/platform/serviceobject"

	"gorm.io/gorm"
)

type rolePermissionChangeSet struct {
	RoleCode                     string   `json:"roleCode"`
	TenantCode                   string   `json:"tenantCode"`
	CurrentAllowPermissionCodes  []string `json:"currentAllowPermissionCodes"`
	ProposedAllowPermissionCodes []string `json:"proposedAllowPermissionCodes"`
	CurrentDenyPermissionCodes   []string `json:"currentDenyPermissionCodes"`
	ProposedDenyPermissionCodes  []string `json:"proposedDenyPermissionCodes"`
	CurrentDataScope             string   `json:"currentDataScope"`
	ProposedDataScope            string   `json:"proposedDataScope"`
	CurrentDataScopeOrgCodes     []string `json:"currentDataScopeOrgCodes"`
	ProposedDataScopeOrgCodes    []string `json:"proposedDataScopeOrgCodes"`
	CurrentDataScopeAreaCodes    []string `json:"currentDataScopeAreaCodes"`
	ProposedDataScopeAreaCodes   []string `json:"proposedDataScopeAreaCodes"`
}

func (e *ServiceObjectExecutor) prepareRolePermissions(ctx context.Context, plan serviceobject.DomainCommandPlan) (serviceobject.CommandResult, error) {
	actor := strings.TrimSpace(plan.Execution.Actor.Username)
	roleCode, _ := plan.Arguments["roleCode"].(string)
	allowPermissionCodes, _ := plan.Arguments["allowPermissionCodes"].([]string)
	denyPermissionCodes, _ := plan.Arguments["denyPermissionCodes"].([]string)
	dataScope, _ := plan.Arguments["dataScope"].(string)
	dataScopeOrgCodes, _ := plan.Arguments["dataScopeOrgCodes"].([]string)
	dataScopeAreaCodes, _ := plan.Arguments["dataScopeAreaCodes"].([]string)
	if actor == "" || !validCode(roleCode) {
		return serviceobject.CommandResult{}, serviceobject.ErrValidation
	}
	impact, err := e.repository.PreviewRolePermissions(ctx, roleCode, allowPermissionCodes, denyPermissionCodes, dataScope, dataScopeOrgCodes, dataScopeAreaCodes)
	if err != nil {
		return serviceobject.CommandResult{}, mapServiceObjectError(err)
	}
	severity := "low"
	if impact.Changed {
		severity = "medium"
	}
	return e.storeDomainPreview(ctx, plan, rolePermissionsPreviewOperation, impact.ExpectedRevision, rolePermissionChangeSet{
		RoleCode: roleCode, TenantCode: impact.TenantCode,
		CurrentAllowPermissionCodes: impact.CurrentAllowPermissionCodes, ProposedAllowPermissionCodes: impact.ProposedAllowPermissionCodes,
		CurrentDenyPermissionCodes: impact.CurrentDenyPermissionCodes, ProposedDenyPermissionCodes: impact.ProposedDenyPermissionCodes,
		CurrentDataScope: impact.CurrentDataScope, ProposedDataScope: impact.ProposedDataScope,
		CurrentDataScopeOrgCodes: impact.CurrentDataScopeOrgCodes, ProposedDataScopeOrgCodes: impact.ProposedDataScopeOrgCodes,
		CurrentDataScopeAreaCodes: impact.CurrentDataScopeAreaCodes, ProposedDataScopeAreaCodes: impact.ProposedDataScopeAreaCodes,
	}, organizationRoleGroupImpactSummary{
		Severity:          severity,
		AffectedUsers:     impact.AffectedUsers,
		CurrentGroupCount: len(impact.CurrentAllowPermissionCodes) + len(impact.CurrentDenyPermissionCodes),
		TargetGroupCount:  len(impact.ProposedAllowPermissionCodes) + len(impact.ProposedDenyPermissionCodes),
	})
}

func (e *ServiceObjectExecutor) applyRolePermissions(ctx context.Context, plan serviceobject.DomainCommandPlan) (serviceobject.CommandResult, error) {
	return e.applyIdentityPreview(ctx, plan, rolePermissionsPreviewOperation, func(tx *gorm.DB, preview gormOrganizationRBACPreview, actor string) (uint64, string, string, error) {
		var changeSet rolePermissionChangeSet
		if err := json.Unmarshal([]byte(preview.ChangeSetJSON), &changeSet); err != nil {
			return 0, "", "", serviceobject.ErrExecutionFailed
		}
		current, err := (&GORMRepository{db: tx}).PreviewRolePermissions(ctx, changeSet.RoleCode,
			changeSet.ProposedAllowPermissionCodes, changeSet.ProposedDenyPermissionCodes, changeSet.ProposedDataScope,
			changeSet.ProposedDataScopeOrgCodes, changeSet.ProposedDataScopeAreaCodes)
		if err != nil || current.ExpectedRevision != preview.ExpectedRevision ||
			!equalStrings(current.CurrentAllowPermissionCodes, changeSet.CurrentAllowPermissionCodes) ||
			!equalStrings(current.ProposedAllowPermissionCodes, changeSet.ProposedAllowPermissionCodes) ||
			!equalStrings(current.CurrentDenyPermissionCodes, changeSet.CurrentDenyPermissionCodes) ||
			!equalStrings(current.ProposedDenyPermissionCodes, changeSet.ProposedDenyPermissionCodes) ||
			current.CurrentDataScope != changeSet.CurrentDataScope || current.ProposedDataScope != changeSet.ProposedDataScope ||
			!equalStrings(current.CurrentDataScopeOrgCodes, changeSet.CurrentDataScopeOrgCodes) ||
			!equalStrings(current.ProposedDataScopeOrgCodes, changeSet.ProposedDataScopeOrgCodes) ||
			!equalStrings(current.CurrentDataScopeAreaCodes, changeSet.CurrentDataScopeAreaCodes) ||
			!equalStrings(current.ProposedDataScopeAreaCodes, changeSet.ProposedDataScopeAreaCodes) {
			return 0, "", "", serviceobject.ErrConflict
		}
		revision, tenantCode, err := (&GORMRepository{db: tx}).ReplaceRolePermissions(ctx, ReplaceRolePermissionsRequest{
			RoleCode: changeSet.RoleCode, AllowPermissionCodes: changeSet.ProposedAllowPermissionCodes,
			DenyPermissionCodes: changeSet.ProposedDenyPermissionCodes, DataScope: changeSet.ProposedDataScope,
			DataScopeOrgCodes: changeSet.ProposedDataScopeOrgCodes, DataScopeAreaCodes: changeSet.ProposedDataScopeAreaCodes,
			ExpectedRevision: preview.ExpectedRevision, ActorID: actor, ChangedAt: e.now().UTC(),
		})
		return revision, tenantCode, "", mapServiceObjectError(err)
	})
}
