package datalifecycle

import (
	"context"
	"errors"

	"platform-go/internal/platform/adminresource"
	"platform-go/internal/platform/capability"
	"platform-go/internal/platform/dataprotection"

	"gorm.io/gorm"
)

type GORMAdminResourceApplier struct {
	db         *gorm.DB
	manifests  []capability.Manifest
	protection dataprotection.Runtime
}

func NewGORMAdminResourceApplier(db *gorm.DB, manifests []capability.Manifest, protection dataprotection.Runtime) *GORMAdminResourceApplier {
	return &GORMAdminResourceApplier{
		db: db, manifests: append([]capability.Manifest(nil), manifests...), protection: protection,
	}
}

func (a *GORMAdminResourceApplier) ApplyAndCheckpoint(ctx context.Context, request ApplyRequest) (ApplyResult, error) {
	if a == nil || a.db == nil || ctx == nil || request.DatasourceID != DefaultDatasourceID || len(request.Batch.Candidates) == 0 ||
		!validCheckpoint(request.PreviousCheckpoint, request.PreviousCheckpoint.Key) ||
		!validCheckpoint(request.Checkpoint, request.Checkpoint.Key) ||
		request.PreviousCheckpoint.Key != request.Checkpoint.Key || request.Checkpoint.Revision != request.PreviousCheckpoint.Revision+1 {
		return ApplyResult{}, ErrApplyFailed
	}
	resource := request.Batch.Candidates[0].Resource
	if resource == "" || resource == "files" || adminresource.RequiresGovernedLifecycleCommand(resource) {
		return ApplyResult{}, ErrApplyFailed
	}
	for _, candidate := range request.Batch.Candidates {
		if candidate.Resource != resource {
			return ApplyResult{}, ErrApplyFailed
		}
	}
	policy, ok := resourcePolicy(request.Policy, resource)
	if !ok || !policy.AutoPurge || policy.RetentionDays <= 0 || !capability.SupportsAdminAutoPurge(resource, policy.Mode) {
		return ApplyResult{}, ErrPolicyDrift
	}
	maintenancePolicy := adminresource.MaintenanceRetentionPolicy{
		Mode: policy.Mode, PolicyVersion: policy.PolicyVersion, RetentionDays: policy.RetentionDays, AutoPurge: policy.AutoPurge,
	}
	var result ApplyResult
	err := a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		lifecycleRepository, err := OpenGORMRepository(ctx, tx)
		if err != nil {
			return err
		}
		if persisted, found, err := lifecycleRepository.LoadCheckpoint(ctx, request.Checkpoint.Key); err != nil {
			return err
		} else if found {
			if persisted == request.Checkpoint {
				result = ApplyResult{Applied: len(request.Batch.Candidates), Checkpoint: persisted}
				return nil
			}
			if persisted != request.PreviousCheckpoint {
				return ErrPolicyDrift
			}
		} else if request.PreviousCheckpoint.Revision != 0 {
			return ErrPolicyDrift
		}

		adminRepository, err := adminresource.OpenGORMAdminResourceRepository(ctx, tx)
		if err != nil {
			return err
		}
		store, err := adminresource.NewRepositoryBackedStoreFromCapabilitiesWithProtection(adminRepository, a.manifests, a.protection)
		if err != nil {
			return err
		}
		for _, candidate := range request.Batch.Candidates {
			event := adminresource.AuditEvent{
				Actor: "retention-runner", Action: "admin_resource.purge", Result: "success", ReasonCode: "retention-elapsed",
			}
			var purgeErr error
			if resource == "api-tokens" {
				_, purgeErr = store.PurgeRevokedWithPolicyAndAudit(resource, candidate.RecordID, "revokedAt", maintenancePolicy, event)
			} else {
				_, purgeErr = store.PurgeWithPolicyAndAudit(resource, candidate.RecordID, maintenancePolicy, event)
			}
			if purgeErr != nil {
				return purgeErr
			}
		}
		if err := lifecycleRepository.SaveCheckpoint(ctx, request.Lease, request.Checkpoint); err != nil {
			return err
		}
		result = ApplyResult{Applied: len(request.Batch.Candidates), Checkpoint: request.Checkpoint}
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrPolicyDrift) || errors.Is(err, ErrLeaseLost) {
			return ApplyResult{}, err
		}
		return ApplyResult{}, ErrApplyFailed
	}
	return result, nil
}

type RoutedBatchApplier struct {
	files    BatchApplier
	database BatchApplier
}

func NewRoutedBatchApplier(files BatchApplier, database BatchApplier) *RoutedBatchApplier {
	return &RoutedBatchApplier{files: files, database: database}
}

func (a *RoutedBatchApplier) ApplyAndCheckpoint(ctx context.Context, request ApplyRequest) (ApplyResult, error) {
	if len(request.Batch.Candidates) == 0 {
		return ApplyResult{}, ErrApplyFailed
	}
	if request.Batch.Candidates[0].Resource == "files" {
		if a == nil || a.files == nil {
			return ApplyResult{}, ErrApplyFailed
		}
		return a.files.ApplyAndCheckpoint(ctx, request)
	}
	if a == nil || a.database == nil {
		return ApplyResult{}, ErrApplyFailed
	}
	return a.database.ApplyAndCheckpoint(ctx, request)
}
