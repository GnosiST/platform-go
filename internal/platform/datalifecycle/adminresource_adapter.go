package datalifecycle

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"platform-go/internal/platform/adminresource"
	"platform-go/internal/platform/capability"
	"platform-go/internal/platform/storage"
)

func PolicySnapshotFromManifests(manifests []capability.Manifest) (PolicySnapshot, error) {
	policy := PolicySnapshot{Version: 1}
	seen := map[string]struct{}{}
	for _, manifest := range manifests {
		for _, resource := range manifest.Admin.Resources {
			name := strings.TrimSpace(resource.Resource)
			if name == "" || name != resource.Resource || resource.Deletion == nil {
				return PolicySnapshot{}, ErrInvalidPolicy
			}
			if _, exists := seen[name]; exists {
				return PolicySnapshot{}, ErrInvalidPolicy
			}
			seen[name] = struct{}{}
			policy.Resources = append(policy.Resources, ResourcePolicy{
				Resource: name, Mode: resource.Deletion.Mode, PolicyVersion: resource.Deletion.PolicyVersion,
				RetentionDays: resource.Deletion.RetentionDays, AutoPurge: resource.Deletion.AutoPurge,
			})
		}
	}
	if _, err := PolicyFingerprint(policy); err != nil {
		return PolicySnapshot{}, err
	}
	return policy, nil
}

type AdminResourceLifecycleStore interface {
	InternalRecordsContext(context.Context, string) ([]adminresource.Record, error)
	InternalRecord(string, string) (adminresource.Record, error)
	ClaimTombstonedFileCleanupWithPolicyAndAudit(string, adminresource.MaintenanceRetentionPolicy, adminresource.AuditEvent) (adminresource.MutationResult, error)
	CompleteTombstonedFileCleanupWithAudit(string, adminresource.AuditEvent) (adminresource.MutationResult, error)
	PurgeTombstonedFileWithPolicyAndAudit(string, adminresource.MaintenanceRetentionPolicy, adminresource.AuditEvent) (adminresource.MutationResult, error)
}

type AdminResourcePlanner struct {
	store AdminResourceLifecycleStore
	clock Clock
}

func NewAdminResourcePlanner(store AdminResourceLifecycleStore, clock Clock) *AdminResourcePlanner {
	return &AdminResourcePlanner{store: store, clock: clock}
}

func (p *AdminResourcePlanner) Plan(ctx context.Context, request PlanRequest) (Batch, error) {
	if p == nil || p.store == nil || p.clock == nil || request.DatasourceID != DefaultDatasourceID || request.Limit < 1 {
		return Batch{}, ErrPlanningFailed
	}
	if _, err := PolicyFingerprint(request.Policy); err != nil {
		return Batch{}, ErrPlanningFailed
	}
	policies := append([]ResourcePolicy(nil), request.Policy.Resources...)
	slices.SortFunc(policies, func(left, right ResourcePolicy) int {
		return strings.Compare(left.Resource, right.Resource)
	})
	now := p.clock.Now().UTC()
	candidates := make([]Candidate, 0)
	for _, policy := range policies {
		if !policy.AutoPurge || policy.RetentionDays <= 0 {
			continue
		}
		retention, ok := capability.AdminRetentionDuration(policy.RetentionDays)
		if !ok || retention <= 0 || !capability.SupportsAdminAutoPurge(policy.Resource, policy.Mode) {
			return Batch{}, ErrPlanningFailed
		}
		records, err := p.store.InternalRecordsContext(ctx, policy.Resource)
		if err != nil {
			return Batch{}, err
		}
		for _, record := range records {
			anchor, ok := retentionAnchor(policy.Resource, record)
			if !ok {
				continue
			}
			deletedAt, err := time.Parse(time.RFC3339, anchor)
			if err != nil {
				return Batch{}, ErrPlanningFailed
			}
			eligibleAt := deletedAt.UTC().Add(retention)
			if now.Before(eligibleAt) {
				continue
			}
			candidate := Candidate{
				Resource: policy.Resource, EligibleAt: eligibleAt.Format(time.RFC3339), RecordID: record.ID,
			}
			if !cursorAfter(Cursor{
				DatasourceID: request.DatasourceID, Resource: candidate.Resource,
				EligibleAt: candidate.EligibleAt, RecordID: candidate.RecordID,
			}, request.Cursor) {
				continue
			}
			candidates = append(candidates, candidate)
		}
	}
	slices.SortFunc(candidates, func(left, right Candidate) int {
		leftKey := left.Resource + "\x00" + left.EligibleAt + "\x00" + left.RecordID
		rightKey := right.Resource + "\x00" + right.EligibleAt + "\x00" + right.RecordID
		return strings.Compare(leftKey, rightKey)
	})
	if len(candidates) == 0 {
		return Batch{DatasourceID: request.DatasourceID, Done: true}, nil
	}
	limit := min(request.Limit, len(candidates))
	resource := candidates[0].Resource
	if resource == "files" {
		limit = 1
	} else {
		for index := 1; index < limit; index++ {
			if candidates[index].Resource != resource {
				limit = index
				break
			}
		}
	}
	selected := append([]Candidate(nil), candidates[:limit]...)
	last := selected[len(selected)-1]
	return Batch{
		DatasourceID: request.DatasourceID,
		Candidates:   selected,
		NextCursor: Cursor{
			DatasourceID: request.DatasourceID, Resource: last.Resource,
			EligibleAt: last.EligibleAt, RecordID: last.RecordID,
		},
		Done: limit == len(candidates),
	}, nil
}

func retentionAnchor(resource string, record adminresource.Record) (string, bool) {
	if resource == "api-tokens" {
		value := strings.TrimSpace(record.Values["revokedAt"])
		return value, record.Status == "revoked" && value != ""
	}
	value := strings.TrimSpace(record.DeletedAt)
	return value, value != ""
}

type FileCleanupApplier struct {
	store        AdminResourceLifecycleStore
	repository   Repository
	objectStores map[string]storage.ObjectStore
}

func NewFileCleanupApplier(store AdminResourceLifecycleStore, repository Repository, objectStores map[string]storage.ObjectStore) *FileCleanupApplier {
	stores := make(map[string]storage.ObjectStore, len(objectStores))
	for driver, objectStore := range objectStores {
		stores[strings.TrimSpace(driver)] = objectStore
	}
	return &FileCleanupApplier{store: store, repository: repository, objectStores: stores}
}

func (a *FileCleanupApplier) ApplyAndCheckpoint(ctx context.Context, request ApplyRequest) (ApplyResult, error) {
	if a == nil || a.store == nil || a.repository == nil || !a.repository.Persistent() ||
		request.DatasourceID != DefaultDatasourceID || len(request.Batch.Candidates) != 1 ||
		request.Batch.Candidates[0].Resource != "files" || !validCheckpoint(request.Checkpoint, request.Checkpoint.Key) ||
		!validCheckpoint(request.PreviousCheckpoint, request.PreviousCheckpoint.Key) ||
		request.PreviousCheckpoint.Key != request.Checkpoint.Key || request.Checkpoint.Revision != request.PreviousCheckpoint.Revision+1 {
		return ApplyResult{}, ErrApplyFailed
	}
	if persisted, found, err := a.repository.LoadCheckpoint(ctx, request.Checkpoint.Key); err != nil {
		return ApplyResult{}, err
	} else if found {
		if persisted == request.Checkpoint {
			return ApplyResult{Applied: 1, Checkpoint: persisted}, nil
		}
		if persisted != request.PreviousCheckpoint {
			return ApplyResult{}, ErrPolicyDrift
		}
	} else if request.PreviousCheckpoint.Revision != 0 {
		return ApplyResult{}, ErrPolicyDrift
	}
	policy, ok := resourcePolicy(request.Policy, "files")
	if !ok || !policy.AutoPurge || policy.RetentionDays <= 0 || policy.Mode != capability.AdminDeletionTombstone ||
		!capability.SupportsAdminAutoPurge("files", policy.Mode) {
		return ApplyResult{}, ErrPolicyDrift
	}
	maintenancePolicy := adminresource.MaintenanceRetentionPolicy{
		Mode: policy.Mode, PolicyVersion: policy.PolicyVersion, RetentionDays: policy.RetentionDays, AutoPurge: policy.AutoPurge,
	}
	candidate := request.Batch.Candidates[0]
	record, err := a.store.InternalRecord("files", candidate.RecordID)
	if errors.Is(err, adminresource.ErrRecordNotFound) {
		return a.commitMissingFileCheckpoint(ctx, request)
	}
	if err != nil {
		return ApplyResult{}, err
	}
	objectKey := adminresource.FileObjectKey(record)
	driver := adminresource.FileStorageDriver(record)
	objectStore := a.objectStores[driver]
	if objectKey == "" || driver == "" || objectStore == nil {
		return ApplyResult{}, ErrApplyFailed
	}
	state := adminresource.FileDeletionState(record)
	if state == adminresource.FileDeletionPending {
		if err := renewFileCleanupLease(ctx, a.repository, &request.Lease); err != nil {
			return ApplyResult{}, err
		}
		claimed, err := a.store.ClaimTombstonedFileCleanupWithPolicyAndAudit(candidate.RecordID, maintenancePolicy, fileCleanupAudit("file.cleanup.claim", "retention-elapsed"))
		if err != nil {
			return ApplyResult{}, err
		}
		record = claimed.Record
		state = adminresource.FileDeletionState(record)
	}
	if state == adminresource.FileDeletionCleanupStarted {
		if err := renewFileCleanupLease(ctx, a.repository, &request.Lease); err != nil {
			return ApplyResult{}, err
		}
		if err := objectStore.Delete(ctx, objectKey); err != nil && !errors.Is(err, storage.ErrObjectNotFound) {
			return ApplyResult{}, err
		}
		if err := renewFileCleanupLease(ctx, a.repository, &request.Lease); err != nil {
			return ApplyResult{}, err
		}
		completed, err := a.store.CompleteTombstonedFileCleanupWithAudit(candidate.RecordID, fileCleanupAudit("file.cleanup.complete", "object-deleted"))
		if err != nil {
			return ApplyResult{}, err
		}
		state = adminresource.FileDeletionState(completed.Record)
	}
	if state != adminresource.FileDeletionObjectDeleted {
		return ApplyResult{}, ErrApplyFailed
	}
	if err := renewFileCleanupLease(ctx, a.repository, &request.Lease); err != nil {
		return ApplyResult{}, err
	}
	if _, err := a.store.PurgeTombstonedFileWithPolicyAndAudit(candidate.RecordID, maintenancePolicy, fileCleanupAudit("file.purge", "retention-elapsed")); err != nil && !errors.Is(err, adminresource.ErrRecordNotFound) {
		return ApplyResult{}, err
	}
	if err := a.repository.SaveCheckpoint(ctx, request.Lease, request.Checkpoint); err != nil {
		return ApplyResult{}, err
	}
	return ApplyResult{Applied: 1, Checkpoint: request.Checkpoint}, nil
}

func renewFileCleanupLease(ctx context.Context, repository Repository, lease *Lease) error {
	if repository == nil || lease == nil {
		return ErrLeaseLost
	}
	ttl := lease.ExpiresAt.Sub(lease.HeartbeatAt)
	if ttl <= 0 {
		return ErrLeaseLost
	}
	now := time.Now().UTC()
	renewed, err := repository.HeartbeatLease(ctx, *lease, now, ttl)
	if err != nil {
		return err
	}
	if renewed.DatasourceID != lease.DatasourceID || renewed.Key != lease.Key || renewed.OwnerID != lease.OwnerID ||
		renewed.Token != lease.Token || renewed.PolicyFingerprint != lease.PolicyFingerprint || !renewed.Active(now) {
		return ErrLeaseLost
	}
	*lease = renewed
	return nil
}

func (a *FileCleanupApplier) commitMissingFileCheckpoint(ctx context.Context, request ApplyRequest) (ApplyResult, error) {
	if err := a.repository.SaveCheckpoint(ctx, request.Lease, request.Checkpoint); err != nil {
		return ApplyResult{}, err
	}
	return ApplyResult{Applied: 1, Checkpoint: request.Checkpoint}, nil
}

func resourcePolicy(policy PolicySnapshot, resource string) (ResourcePolicy, bool) {
	for _, candidate := range policy.Resources {
		if candidate.Resource == resource {
			return candidate, true
		}
	}
	return ResourcePolicy{}, false
}

func fileCleanupAudit(action, reason string) adminresource.AuditEvent {
	return adminresource.AuditEvent{
		Actor: "retention-runner", Action: action, Result: "success", ReasonCode: reason,
	}
}
