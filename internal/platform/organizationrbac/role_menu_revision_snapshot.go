package organizationrbac

import "context"

type RoleMenuRevision struct {
	RoleCode string
	Revision uint64
}

type RoleMenuRevisionSnapshot struct {
	GlobalRevision uint64
	RoleRevisions  []RoleMenuRevision
}

func (r *GORMRepository) LoadRoleMenuRevisionSnapshot(ctx context.Context, roleCodes []string) (RoleMenuRevisionSnapshot, error) {
	if !r.ready(ctx) {
		return RoleMenuRevisionSnapshot{}, ErrRepositoryFailed
	}
	codes, err := canonicalCodes(roleCodes)
	if err != nil {
		return RoleMenuRevisionSnapshot{}, err
	}
	db := r.db.WithContext(ctx)
	globalRevision, err := loadGlobalRevision(db)
	if err != nil {
		return RoleMenuRevisionSnapshot{}, err
	}
	snapshot := RoleMenuRevisionSnapshot{GlobalRevision: globalRevision, RoleRevisions: make([]RoleMenuRevision, 0, len(codes))}
	if len(codes) == 0 {
		return snapshot, nil
	}
	deletedRoles, err := deletedRecordIDs(db, "roles")
	if err != nil {
		return RoleMenuRevisionSnapshot{}, err
	}
	var roles []gormRole
	if err := db.Where("code IN ? AND status = ?", codes, StatusEnabled).Order("code").Find(&roles).Error; err != nil {
		return RoleMenuRevisionSnapshot{}, repositoryError(err)
	}
	activeCodes := make([]string, 0, len(roles))
	for _, role := range roles {
		if _, deleted := deletedRoles[role.ID]; !deleted {
			activeCodes = append(activeCodes, role.Code)
		}
	}
	if len(activeCodes) == 0 {
		return snapshot, nil
	}
	var rows []gormRoleMenuRevision
	if err := db.Where("role_code IN ?", activeCodes).Find(&rows).Error; err != nil {
		return RoleMenuRevisionSnapshot{}, repositoryError(err)
	}
	revisions := make(map[string]uint64, len(rows))
	for _, row := range rows {
		revisions[row.RoleCode] = row.Revision
	}
	for _, roleCode := range activeCodes {
		snapshot.RoleRevisions = append(snapshot.RoleRevisions, RoleMenuRevision{RoleCode: roleCode, Revision: revisions[roleCode]})
	}
	return snapshot, nil
}
