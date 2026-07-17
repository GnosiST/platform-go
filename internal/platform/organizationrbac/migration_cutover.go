package organizationrbac

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/GnosiST/platform-go/internal/platform/httpapi"
	"github.com/GnosiST/platform-go/internal/platform/rbac"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	PromotionPrepared         = "prepared"
	PromotionDualRead         = "dual-read"
	PromotionTargetRead       = "target-read"
	PromotionTargetWrite      = "target-write"
	PromotionRollbackRequired = "rollback-required"
	PromotionRolledBack       = "rolled-back-verified"
)

type MenuPromotionState struct {
	RunID              string
	ManifestHash       string
	Phase              string
	FrozenRevision     uint64
	ActivePrincipals   int
	Equivalent         int
	ComparisonHash     string
	LegacySnapshotHash string
	CheckpointRef      string
	ObservedAt         time.Time
}

type MenuWritePromotionRequest struct {
	RunID         string
	ExpectedPhase string
	ActorID       string
	Reason        string
	ApprovalRef   string
	ObservedAt    time.Time
}

type gormOrganizationRBACPromotion struct {
	RunID              string    `gorm:"column:run_id;size:191;primaryKey"`
	ManifestHash       string    `gorm:"column:manifest_hash;size:64;not null"`
	Phase              string    `gorm:"column:phase;size:32;index;not null"`
	FrozenRevision     uint64    `gorm:"column:frozen_revision;not null"`
	ActivePrincipals   int       `gorm:"column:active_principals;not null"`
	Equivalent         int       `gorm:"column:equivalent;not null"`
	ComparisonHash     string    `gorm:"column:comparison_hash;size:64;not null"`
	LegacySnapshotHash string    `gorm:"column:legacy_snapshot_hash;size:64;not null"`
	CheckpointRef      string    `gorm:"column:checkpoint_ref;size:1024;not null"`
	ObservedAt         time.Time `gorm:"column:observed_at;not null"`
	UpdatedAt          time.Time `gorm:"column:updated_at;not null"`
}

type gormOrganizationRBACPromotionEvent struct {
	RunID          string    `gorm:"column:run_id;size:191;primaryKey"`
	Sequence       int       `gorm:"column:sequence;primaryKey"`
	FromPhase      string    `gorm:"column:from_phase;size:32;not null"`
	ToPhase        string    `gorm:"column:to_phase;size:32;not null"`
	FrozenRevision uint64    `gorm:"column:frozen_revision;not null"`
	ActorID        string    `gorm:"column:actor_id;size:191"`
	Reason         string    `gorm:"column:reason;size:1024"`
	ApprovalRef    string    `gorm:"column:approval_ref;size:1024"`
	ObservedAt     time.Time `gorm:"column:observed_at;not null"`
}

type gormOrganizationRBACPromotionObservation struct {
	RunID          string    `gorm:"column:run_id;size:191;primaryKey"`
	PrincipalKey   string    `gorm:"column:principal_key;size:64;primaryKey"`
	GlobalRevision uint64    `gorm:"column:global_revision;primaryKey"`
	Equal          bool      `gorm:"column:equal;not null"`
	AddedCount     int       `gorm:"column:added_count;not null"`
	RemovedCount   int       `gorm:"column:removed_count;not null"`
	ObservedAt     time.Time `gorm:"column:observed_at;not null"`
}

func (gormOrganizationRBACPromotion) TableName() string {
	return "platform_organization_rbac_promotions"
}
func (gormOrganizationRBACPromotionEvent) TableName() string {
	return "platform_organization_rbac_promotion_events"
}
func (gormOrganizationRBACPromotionObservation) TableName() string {
	return "platform_organization_rbac_promotion_observations"
}

func (s MenuPromotionState) valid() bool {
	return validCode(s.RunID) && canonicalDigest(s.ManifestHash) && validPromotionPhase(s.Phase) &&
		canonicalDigest(s.ComparisonHash) && canonicalDigest(s.LegacySnapshotHash) && strings.TrimSpace(s.CheckpointRef) != "" &&
		!s.ObservedAt.IsZero() && s.ActivePrincipals >= 0 && s.Equivalent >= 0 && s.Equivalent <= s.ActivePrincipals
}

func validPromotionPhase(phase string) bool {
	switch phase {
	case PromotionPrepared, PromotionDualRead, PromotionTargetRead, PromotionTargetWrite, PromotionRollbackRequired, PromotionRolledBack:
		return true
	default:
		return false
	}
}

func (r *GORMRepository) RecordMenuPromotionState(ctx context.Context, state MenuPromotionState) error {
	if !r.ready(ctx) || !state.valid() {
		return ErrInvalid
	}
	return repositoryError(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current gormOrganizationRBACPromotion
		err := tx.Where("run_id = ?", state.RunID).Take(&current).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := tx.Create(gormPromotionFromState(state)).Error; err != nil {
				return err
			}
			return tx.Create(&gormOrganizationRBACPromotionEvent{RunID: state.RunID, Sequence: 1, ToPhase: state.Phase, FrozenRevision: state.FrozenRevision, ObservedAt: state.ObservedAt.UTC()}).Error
		}
		if err != nil {
			return err
		}
		if current.ManifestHash != state.ManifestHash || current.CheckpointRef != state.CheckpointRef || !promotionTransitionAllowed(current.Phase, state.Phase) {
			return &ValidationError{Field: "promotion.phase", Reason: "immutable promotion evidence or invalid phase transition"}
		}
		var count int64
		if err := tx.Model(&gormOrganizationRBACPromotionEvent{}).Where("run_id = ?", state.RunID).Count(&count).Error; err != nil {
			return err
		}
		if err := tx.Model(&gormOrganizationRBACPromotion{}).Where("run_id = ?", state.RunID).Updates(map[string]any{
			"phase": state.Phase, "frozen_revision": state.FrozenRevision, "active_principals": state.ActivePrincipals,
			"equivalent": state.Equivalent, "comparison_hash": state.ComparisonHash, "legacy_snapshot_hash": state.LegacySnapshotHash,
			"observed_at": state.ObservedAt.UTC(), "updated_at": state.ObservedAt.UTC(),
		}).Error; err != nil {
			return err
		}
		if err := tx.Create(&gormOrganizationRBACPromotionEvent{RunID: state.RunID, Sequence: int(count) + 1, FromPhase: current.Phase, ToPhase: state.Phase, FrozenRevision: state.FrozenRevision, ObservedAt: state.ObservedAt.UTC()}).Error; err != nil {
			return err
		}
		return nil
	}))
}

func (r *GORMRepository) PromoteMenuWrites(ctx context.Context, request MenuWritePromotionRequest) (MenuPromotionState, error) {
	request, valid := normalizedMenuWritePromotionRequest(request)
	if !r.ready(ctx) || !valid {
		return MenuPromotionState{}, ErrInvalid
	}
	var promoted MenuPromotionState
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current gormOrganizationRBACPromotion
		if err := lockedMenuPromotion(tx, request.RunID, &current); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return &ValidationError{Field: "promotion.runId", Reason: "target-read promotion run was not found"}
			}
			return err
		}
		if current.Phase == PromotionTargetWrite {
			promoted = stateFromGORMPromotion(current)
			return nil
		}
		if current.Phase != PromotionTargetRead {
			return &ValidationError{Field: "promotion.phase", Reason: "target-read promotion is required"}
		}
		currentRevision, err := lockedGlobalRevision(tx)
		if err != nil {
			return err
		}
		if current.FrozenRevision != currentRevision {
			return &ValidationError{Field: "promotion.frozenRevision", Reason: "global revision changed; a new comparison is required"}
		}
		if current.ActivePrincipals != current.Equivalent {
			return &ValidationError{Field: "promotion.observation", Reason: "all-principal observation is incomplete"}
		}
		var count int64
		if err := tx.Model(&gormOrganizationRBACPromotionEvent{}).Where("run_id = ?", current.RunID).Count(&count).Error; err != nil {
			return err
		}
		observedAt := request.ObservedAt.UTC()
		result := tx.Model(&gormOrganizationRBACPromotion{}).
			Where("run_id = ? AND phase = ? AND frozen_revision = ? AND active_principals = equivalent", current.RunID, PromotionTargetRead, current.FrozenRevision).
			Where("(SELECT value FROM "+adminResourceStateTable+" WHERE key = ?) = ?", "revision", strconv.FormatUint(current.FrozenRevision, 10)).
			Updates(map[string]any{
				"phase": PromotionTargetWrite, "observed_at": observedAt, "updated_at": observedAt,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return &ValidationError{Field: "promotion", Reason: "promotion state changed during target-write approval"}
		}
		if err := tx.Create(&gormOrganizationRBACPromotionEvent{
			RunID: current.RunID, Sequence: int(count) + 1, FromPhase: PromotionTargetRead, ToPhase: PromotionTargetWrite,
			FrozenRevision: current.FrozenRevision, ActorID: request.ActorID, Reason: request.Reason, ApprovalRef: request.ApprovalRef,
			ObservedAt: observedAt,
		}).Error; err != nil {
			return err
		}
		current.Phase = PromotionTargetWrite
		current.ObservedAt = observedAt
		current.UpdatedAt = observedAt
		promoted = stateFromGORMPromotion(current)
		return nil
	})
	if err != nil {
		return MenuPromotionState{}, repositoryError(err)
	}
	return promoted, nil
}

func lockedMenuPromotion(tx *gorm.DB, runID string, current *gormOrganizationRBACPromotion) error {
	query := tx.Where("run_id = ?", runID)
	if tx.Dialector.Name() != "sqlite" {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	return query.Take(current).Error
}

func lockedGlobalRevision(tx *gorm.DB) (uint64, error) {
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&gormAdminResourceState{Key: "revision", Value: "0"}).Error; err != nil {
		return 0, err
	}
	query := tx.Where("key = ?", "revision")
	if tx.Dialector.Name() != "sqlite" {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var state gormAdminResourceState
	if err := query.Take(&state).Error; err != nil {
		return 0, err
	}
	revision, err := strconv.ParseUint(state.Value, 10, 64)
	if err != nil {
		return 0, ErrRepositoryFailed
	}
	return revision, nil
}

func normalizedMenuWritePromotionRequest(request MenuWritePromotionRequest) (MenuWritePromotionRequest, bool) {
	request.ActorID = strings.TrimSpace(request.ActorID)
	request.Reason = strings.TrimSpace(request.Reason)
	request.ApprovalRef = strings.TrimSpace(request.ApprovalRef)
	valid := validCode(request.RunID) && request.ExpectedPhase == PromotionTargetRead && validCode(request.ActorID) &&
		len(request.ActorID) <= 191 && len(request.Reason) > 0 && len(request.Reason) <= 1024 &&
		len(request.ApprovalRef) > 0 && len(request.ApprovalRef) <= 1024 && !request.ObservedAt.IsZero()
	return request, valid
}

func promotionTransitionAllowed(from, to string) bool {
	if from == to {
		return true
	}
	allowed := map[string]map[string]bool{
		PromotionPrepared:         {PromotionDualRead: true, PromotionRollbackRequired: true},
		PromotionDualRead:         {PromotionTargetRead: true, PromotionRollbackRequired: true},
		PromotionTargetRead:       {PromotionTargetWrite: true, PromotionRollbackRequired: true},
		PromotionTargetWrite:      {PromotionRollbackRequired: true},
		PromotionRollbackRequired: {PromotionRolledBack: true},
	}
	return allowed[from][to]
}

func gormPromotionFromState(state MenuPromotionState) gormOrganizationRBACPromotion {
	return gormOrganizationRBACPromotion{RunID: state.RunID, ManifestHash: state.ManifestHash, Phase: state.Phase, FrozenRevision: state.FrozenRevision, ActivePrincipals: state.ActivePrincipals, Equivalent: state.Equivalent, ComparisonHash: state.ComparisonHash, LegacySnapshotHash: state.LegacySnapshotHash, CheckpointRef: state.CheckpointRef, ObservedAt: state.ObservedAt.UTC(), UpdatedAt: state.ObservedAt.UTC()}
}

func stateFromGORMPromotion(row gormOrganizationRBACPromotion) MenuPromotionState {
	return MenuPromotionState{RunID: row.RunID, ManifestHash: row.ManifestHash, Phase: row.Phase, FrozenRevision: row.FrozenRevision, ActivePrincipals: row.ActivePrincipals, Equivalent: row.Equivalent, ComparisonHash: row.ComparisonHash, LegacySnapshotHash: row.LegacySnapshotHash, CheckpointRef: row.CheckpointRef, ObservedAt: row.ObservedAt}
}

func (r *GORMRepository) ValidateMenuPromotion(ctx context.Context, mode string, roleMenuWriteEnabled bool) (MenuPromotionState, error) {
	if !r.ready(ctx) {
		return MenuPromotionState{}, ErrRepositoryFailed
	}
	mode = strings.TrimSpace(mode)
	if mode != string(httpapi.AdminMenuServingModeLegacy) && mode != string(httpapi.AdminMenuServingModeDualRead) && mode != string(httpapi.AdminMenuServingModeTarget) {
		return MenuPromotionState{}, &ValidationError{Field: "menuServingMode", Reason: "must be legacy, dual-read, or target"}
	}
	var row gormOrganizationRBACPromotion
	err := r.db.WithContext(ctx).Order("observed_at DESC").Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if mode == string(httpapi.AdminMenuServingModeLegacy) && !roleMenuWriteEnabled {
			return MenuPromotionState{}, nil
		}
		return MenuPromotionState{}, &ValidationError{Field: "promotion", Reason: "promotion evidence is required"}
	}
	if err != nil {
		return MenuPromotionState{}, repositoryError(err)
	}
	state := stateFromGORMPromotion(row)
	currentRevision, err := r.CurrentGlobalRevision(ctx)
	if err != nil {
		return MenuPromotionState{}, err
	}
	if state.Phase == PromotionRolledBack {
		if mode == string(httpapi.AdminMenuServingModeLegacy) && !roleMenuWriteEnabled {
			return state, nil
		}
		return MenuPromotionState{}, &ValidationError{Field: "promotion.phase", Reason: "rolled-back evidence only permits legacy read mode"}
	}
	if mode == string(httpapi.AdminMenuServingModeLegacy) {
		return MenuPromotionState{}, &ValidationError{Field: "promotion.phase", Reason: "legacy serving is blocked after target promotion"}
	}
	if state.FrozenRevision != currentRevision && state.Phase != PromotionTargetWrite {
		return MenuPromotionState{}, &ValidationError{Field: "promotion.frozenRevision", Reason: "global revision changed; a new comparison is required"}
	}
	if state.ActivePrincipals != state.Equivalent {
		return MenuPromotionState{}, &ValidationError{Field: "promotion.observation", Reason: "all-principal observation is incomplete"}
	}
	switch mode {
	case string(httpapi.AdminMenuServingModeDualRead):
		if state.Phase != PromotionDualRead {
			return MenuPromotionState{}, &ValidationError{Field: "promotion.phase", Reason: "dual-read promotion is not open"}
		}
		if roleMenuWriteEnabled {
			return MenuPromotionState{}, &ValidationError{Field: "roleMenuWriteEnabled", Reason: "target write promotion is required"}
		}
	case string(httpapi.AdminMenuServingModeTarget):
		if state.Phase != PromotionTargetRead && state.Phase != PromotionTargetWrite {
			return MenuPromotionState{}, &ValidationError{Field: "promotion.phase", Reason: "target promotion is not approved"}
		}
		if roleMenuWriteEnabled && state.Phase != PromotionTargetWrite {
			return MenuPromotionState{}, &ValidationError{Field: "roleMenuWriteEnabled", Reason: "target write promotion is not approved"}
		}
	}
	return state, nil
}

func (r *GORMRepository) RecordMenuDualReadComparison(ctx context.Context, principalKey string, comparison httpapi.AdminMenuComparison) error {
	if !r.ready(ctx) || strings.TrimSpace(principalKey) == "" {
		return ErrInvalid
	}
	keyDigest := sha256.Sum256([]byte(strings.TrimSpace(principalKey)))
	key := hex.EncodeToString(keyDigest[:])
	return repositoryError(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var promotion gormOrganizationRBACPromotion
		if err := tx.Where("phase = ?", PromotionDualRead).Order("observed_at DESC").Take(&promotion).Error; err != nil {
			return err
		}
		row := gormOrganizationRBACPromotionObservation{RunID: promotion.RunID, PrincipalKey: key, GlobalRevision: comparison.GlobalRevision, Equal: comparison.Equal, AddedCount: comparison.AddedCount, RemovedCount: comparison.RemovedCount, ObservedAt: time.Now().UTC()}
		var existing gormOrganizationRBACPromotionObservation
		err := tx.Where("run_id = ? AND principal_key = ? AND global_revision = ?", row.RunID, row.PrincipalKey, row.GlobalRevision).Take(&existing).Error
		if err == nil {
			if existing.Equal != row.Equal || existing.AddedCount != row.AddedCount || existing.RemovedCount != row.RemovedCount {
				return &ValidationError{Field: "promotion.observation", Reason: "duplicate principal observation mismatch"}
			}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		return tx.Create(&row).Error
	}))
}

func (r *GORMRepository) LegacySnapshotHash(ctx context.Context, manifest MigrationManifest) (string, error) {
	if !r.ready(ctx) {
		return "", ErrRepositoryFailed
	}
	state, err := loadPrincipalComparisonState(r.db.WithContext(ctx), manifest, PrincipalComparisonPersisted)
	if err != nil {
		return "", err
	}
	return legacySnapshotHash(state)
}

func legacySnapshotHash(state principalComparisonState) (string, error) {
	values := make([]PrincipalAuthorizationSnapshot, 0, len(state.Users))
	for _, user := range state.Users {
		snapshot, err := legacyPrincipalSnapshot(state, user)
		if err != nil {
			return "", err
		}
		values = append(values, snapshot)
	}
	sort.Slice(values, func(i, j int) bool { return values[i].UserCode < values[j].UserCode })
	encoded, err := json.Marshal(values)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func canonicalDigest(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 64 {
		return false
	}
	for _, char := range value {
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f')) {
			return false
		}
	}
	return true
}

func principalComparisonKey(principal rbac.Principal) string {
	return strings.TrimSpace(principal.User.Username)
}
