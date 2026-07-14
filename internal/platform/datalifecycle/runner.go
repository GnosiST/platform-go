package datalifecycle

import (
	"context"
	"strconv"
	"strings"
	"time"
)

type Runner struct {
	repository Repository
	planner    BatchPlanner
	applier    BatchApplier
	clock      Clock
}

func NewRunner(repository Repository, planner BatchPlanner, applier BatchApplier, clock Clock) *Runner {
	return &Runner{repository: repository, planner: planner, applier: applier, clock: clock}
}

func (r *Runner) Promote(ctx context.Context, request PromotionRequest) (Promotion, error) {
	if !request.Enabled {
		return Promotion{}, ErrDisabled
	}
	if r == nil || r.repository == nil || !r.repository.Persistent() || r.clock == nil || ctx == nil {
		return Promotion{}, ErrPersistentRepositoryRequired
	}
	datasourceID := request.DatasourceID
	if datasourceID == "" {
		datasourceID = DefaultDatasourceID
	}
	if datasourceID != DefaultDatasourceID || !validIdentifier(request.ActorID) ||
		strings.TrimSpace(request.Reason) == "" || request.Reason != strings.TrimSpace(request.Reason) ||
		strings.TrimSpace(request.ApprovalRef) == "" || request.ApprovalRef != strings.TrimSpace(request.ApprovalRef) ||
		!canonicalDigest(request.ImpactReportHash) {
		return Promotion{}, ErrPromotionRequired
	}
	currentFingerprint, err := PolicyFingerprint(request.CurrentPolicy)
	if err != nil {
		return Promotion{}, ErrInvalidPolicy
	}
	proposedFingerprint, err := PolicyFingerprint(request.ProposedPolicy)
	if err != nil || proposedFingerprint != request.PromotedFingerprint || !retentionShortened(request.CurrentPolicy, request.ProposedPolicy) {
		return Promotion{}, ErrPromotionRequired
	}
	impact, found, err := r.repository.LoadImpactReport(ctx, datasourceID, request.ImpactReportHash)
	if err != nil {
		return Promotion{}, ErrRepositoryFailed
	}
	if !found || impact.ReportHash != request.ImpactReportHash || impact.ReportHash != impactReportDigest(impact) ||
		impact.PolicyFingerprint != proposedFingerprint || impact.DatasourceID != datasourceID || !canonicalDigest(impact.EvidenceHash) {
		return Promotion{}, ErrPromotionRequired
	}
	promotion := Promotion{
		DatasourceID: datasourceID, CurrentFingerprint: currentFingerprint,
		PromotedFingerprint: proposedFingerprint, ImpactReportHash: request.ImpactReportHash,
		ActorID: request.ActorID, Reason: request.Reason, ApprovalRef: request.ApprovalRef,
		PromotedAt: r.clock.Now().UTC(),
	}
	if err := r.repository.SavePromotion(ctx, promotion); err != nil {
		return Promotion{}, ErrRepositoryFailed
	}
	stored, found, err := r.repository.LoadPromotion(ctx, datasourceID, proposedFingerprint)
	if err != nil || !found || !samePromotion(stored, promotion) {
		return Promotion{}, ErrRepositoryFailed
	}
	return stored, nil
}

func (r *Runner) Run(ctx context.Context, options Options) (Report, error) {
	datasourceID := options.DatasourceID
	if datasourceID == "" {
		datasourceID = DefaultDatasourceID
	}
	report := Report{DatasourceID: datasourceID, RunID: options.RunID, Mode: options.Mode, Status: StatusFailed}
	if !options.Enabled {
		report.Status = StatusDisabled
		return report, ErrDisabled
	}
	if r == nil || r.repository == nil || !r.repository.Persistent() {
		return report, ErrPersistentRepositoryRequired
	}
	if ctx == nil || r.planner == nil || r.clock == nil || datasourceID != DefaultDatasourceID {
		return report, ErrInvalidOptions
	}
	if options.Mode != ModeImpact && options.Mode != ModeDryRun && options.Mode != ModeApply {
		return report, ErrInvalidOptions
	}
	fingerprint, err := PolicyFingerprint(options.Policy)
	if err != nil || options.PolicyFingerprint != fingerprint {
		return report, ErrPolicyDrift
	}
	report.PolicyFingerprint = fingerprint
	if !validIdentifier(options.RunID) || !validIdentifier(options.OwnerID) {
		return report, ErrInvalidOptions
	}
	batchSize := options.BatchSize
	if batchSize == 0 {
		batchSize = DefaultBatchSize
	}
	if batchSize < 1 || batchSize > MaximumBatchSize || options.MaxRetries < 0 || options.MaxRetries > MaximumRetries {
		return report, ErrInvalidOptions
	}
	leaseTTL := options.LeaseTTL
	if leaseTTL == 0 {
		leaseTTL = time.Minute
	}
	heartbeatInterval := options.HeartbeatInterval
	if heartbeatInterval == 0 {
		heartbeatInterval = leaseTTL / 3
	}
	if leaseTTL <= 0 || heartbeatInterval <= 0 || heartbeatInterval >= leaseTTL {
		return report, ErrInvalidOptions
	}
	operationTimeout := options.OperationTimeout
	if operationTimeout == 0 {
		operationTimeout = heartbeatInterval
	}
	if operationTimeout <= 0 || operationTimeout > heartbeatInterval {
		return report, ErrInvalidOptions
	}
	if options.Mode == ModeApply {
		if err := r.validatePromotion(ctx, datasourceID, fingerprint, options.Promotion); err != nil {
			return reportWithFailure(report, "promotion"), err
		}
	}

	now := r.clock.Now().UTC()
	lease, err := r.repository.AcquireLease(ctx, LeaseRequest{
		DatasourceID: datasourceID, Key: "data-lifecycle:" + datasourceID,
		OwnerID: options.OwnerID, PolicyFingerprint: fingerprint, Now: now, TTL: leaseTTL,
	})
	if err != nil {
		if err == ErrLeaseHeld {
			return report, ErrLeaseHeld
		}
		return reportWithFailure(report, "repository"), ErrRepositoryFailed
	}
	if !validLease(lease, datasourceID, options.OwnerID, fingerprint, now) {
		return reportWithFailure(report, "lease"), ErrLeaseLost
	}
	defer func() {
		releaseCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = r.repository.ReleaseLease(releaseCtx, lease)
	}()

	key := checkpointKey(options)
	checkpoint, found, err := r.repository.LoadCheckpoint(ctx, key)
	if err != nil {
		return reportWithFailure(report, "repository"), ErrRepositoryFailed
	}
	if !found {
		checkpoint = sealCheckpoint(Checkpoint{
			Key: key, DatasourceID: datasourceID,
			Cursor:       Cursor{DatasourceID: datasourceID},
			EvidenceHash: stableDigest("evidence-empty", datasourceID, fingerprint),
		})
	} else if !validCheckpoint(checkpoint, key) {
		return reportWithFailure(report, "checkpoint"), ErrPolicyDrift
	}
	if checkpoint.Complete {
		return r.finish(ctx, lease, report, checkpoint)
	}

	for {
		if ctx.Err() != nil {
			return reportWithFailure(report, "cancelled"), ctx.Err()
		}
		now = r.clock.Now().UTC()
		if !lease.Active(now) {
			return reportWithFailure(report, "lease"), ErrLeaseLost
		}
		if !now.Before(lease.HeartbeatAt.Add(heartbeatInterval)) {
			lease, err = r.repository.HeartbeatLease(ctx, lease, now, leaseTTL)
			if err != nil || !validLease(lease, datasourceID, options.OwnerID, fingerprint, now) {
				return reportWithFailure(report, "lease"), ErrLeaseLost
			}
		}
		operationCtx, cancelOperation, ok := boundedOperationContext(ctx, operationTimeout, lease.ExpiresAt, now)
		if !ok {
			return reportWithFailure(r.reportFromCheckpoint(report, checkpoint), "lease"), ErrLeaseLost
		}
		batch, retries, err := r.planBatch(operationCtx, PlanRequest{
			DatasourceID: datasourceID, Mode: options.Mode, RunID: options.RunID,
			Policy: options.Policy, PolicyFingerprint: fingerprint,
			Cursor: checkpoint.Cursor, Limit: batchSize,
		}, options.MaxRetries)
		cancelOperation()
		if err != nil {
			report = r.reportFromCheckpoint(report, checkpoint)
			report.Counts.Retries += retries
			return reportWithFailure(report, "planner"), ErrPlanningFailed
		}
		if err := validateBatch(batch, datasourceID, checkpoint.Cursor, batchSize); err != nil {
			report = r.reportFromCheckpoint(report, checkpoint)
			report.Counts.Retries += retries
			return reportWithFailure(report, "planner"), ErrPlanningFailed
		}
		if len(batch.Candidates) == 0 && batch.Done {
			next := checkpoint
			next.Counts.Retries += retries
			next.Complete = true
			next.LastBatchID = stableDigest("empty-batch", datasourceID, fingerprint, checkpoint.Cursor.Resource, checkpoint.Cursor.EligibleAt, checkpoint.Cursor.RecordID)
			next.Revision++
			next.UpdatedAt = now
			next = sealCheckpoint(next)
			return r.commitCheckpoint(ctx, lease, report, next)
		}

		batchID := batchDigest(datasourceID, fingerprint, checkpoint.Cursor, batch)
		batchCounts := Counts{Batches: 1}
		switch options.Mode {
		case ModeImpact:
			batchCounts.Eligible = len(batch.Candidates)
		case ModeDryRun:
			batchCounts.Planned = len(batch.Candidates)
		case ModeApply:
			if r.applier == nil {
				return reportWithFailure(r.reportFromCheckpoint(report, checkpoint), "applier"), ErrApplyFailed
			}
			batchCounts.Applied = len(batch.Candidates)
		}

		next := checkpoint
		next.Cursor = batch.NextCursor
		next.Counts = next.Counts.plus(batchCounts)
		next.Counts.Retries += retries
		next.EvidenceHash = stableDigest("evidence-chain", checkpoint.EvidenceHash, batchID)
		next.LastBatchID = batchID
		next.Revision++
		next.Complete = batch.Done
		next.UpdatedAt = now
		next = sealCheckpoint(next)
		if options.Mode == ModeApply {
			operationCtx, cancelOperation, ok := boundedOperationContext(ctx, operationTimeout, lease.ExpiresAt, r.clock.Now().UTC())
			if !ok {
				return reportWithFailure(r.reportFromCheckpoint(report, checkpoint), "lease"), ErrLeaseLost
			}
			result, applyRetries, applyErr := r.applyBatch(operationCtx, ApplyRequest{
				DatasourceID: datasourceID, RunID: options.RunID, BatchID: batchID,
				Policy: options.Policy, PolicyFingerprint: fingerprint, Lease: lease, Batch: batch, Checkpoint: next,
				PreviousCheckpoint: checkpoint,
			}, options.MaxRetries)
			cancelOperation()
			expected := next
			expected.Counts.Retries += applyRetries
			expected = sealCheckpoint(expected)
			if applyErr != nil || result.Applied != len(batch.Candidates) || result.Checkpoint != expected {
				failed := r.reportFromCheckpoint(report, checkpoint)
				failed.Counts.Retries += retries + applyRetries
				return reportWithFailure(failed, "applier"), ErrApplyFailed
			}
			checkpoint = result.Checkpoint
		} else {
			checkpoint = next
			if checkpoint.Complete {
				return r.commitCheckpoint(ctx, lease, report, checkpoint)
			}
			if err := r.repository.SaveCheckpoint(ctx, lease, checkpoint); err != nil {
				return reportWithFailure(r.reportFromCheckpoint(report, checkpoint), "repository"), ErrRepositoryFailed
			}
		}
		if checkpoint.Complete {
			return r.finish(ctx, lease, report, checkpoint)
		}
	}
}

func (r *Runner) planBatch(ctx context.Context, request PlanRequest, maxRetries int) (Batch, int, error) {
	for attempt := 0; ; attempt++ {
		batch, err := r.planner.Plan(ctx, request)
		if err == nil {
			return batch, attempt, nil
		}
		if attempt >= maxRetries || ctx.Err() != nil {
			return Batch{}, attempt, err
		}
	}
}

func (r *Runner) applyBatch(ctx context.Context, request ApplyRequest, maxRetries int) (ApplyResult, int, error) {
	for attempt := 0; ; attempt++ {
		attemptRequest := request
		attemptRequest.Checkpoint.Counts.Retries += attempt
		attemptRequest.Checkpoint = sealCheckpoint(attemptRequest.Checkpoint)
		result, err := r.applier.ApplyAndCheckpoint(ctx, attemptRequest)
		if err == nil {
			return result, attempt, nil
		}
		if attempt >= maxRetries || ctx.Err() != nil {
			return ApplyResult{}, attempt, err
		}
	}
}

func (r *Runner) finish(ctx context.Context, lease Lease, report Report, checkpoint Checkpoint) (Report, error) {
	report = r.completedReport(report, checkpoint)
	if report.Mode == ModeImpact {
		impact := impactReportFrom(report, r.clock.Now().UTC())
		if err := r.repository.SaveImpactReportAndCheckpoint(ctx, lease, impact, checkpoint); err != nil {
			return reportWithFailure(report, "repository"), ErrRepositoryFailed
		}
	}
	return report, nil
}

func (r *Runner) commitCheckpoint(ctx context.Context, lease Lease, report Report, checkpoint Checkpoint) (Report, error) {
	if report.Mode == ModeImpact {
		return r.finish(ctx, lease, report, checkpoint)
	}
	if err := r.repository.SaveCheckpoint(ctx, lease, checkpoint); err != nil {
		return reportWithFailure(r.reportFromCheckpoint(report, checkpoint), "repository"), ErrRepositoryFailed
	}
	return r.finish(ctx, lease, report, checkpoint)
}

func impactReportFrom(report Report, generatedAt time.Time) ImpactReport {
	impact := ImpactReport{
		DatasourceID: report.DatasourceID, RunID: report.RunID,
		PolicyFingerprint: report.PolicyFingerprint, Counts: report.Counts,
		Cursor: report.Cursor, EvidenceHash: report.EvidenceHash, GeneratedAt: generatedAt,
	}
	impact.ReportHash = impactReportDigest(impact)
	return impact
}

func (r *Runner) completedReport(report Report, checkpoint Checkpoint) Report {
	report.Status = StatusCompleted
	report.Counts = checkpoint.Counts
	report.Cursor = sanitizedCursor(checkpoint.Cursor)
	report.EvidenceHash = checkpoint.EvidenceHash
	if report.Mode == ModeImpact {
		impact := ImpactReport{
			DatasourceID: report.DatasourceID, RunID: report.RunID,
			PolicyFingerprint: report.PolicyFingerprint, Counts: report.Counts,
			Cursor: report.Cursor, EvidenceHash: report.EvidenceHash,
		}
		report.ImpactReportHash = impactReportDigest(impact)
	}
	return report
}

func (r *Runner) reportFromCheckpoint(report Report, checkpoint Checkpoint) Report {
	report.Counts = checkpoint.Counts
	report.Cursor = sanitizedCursor(checkpoint.Cursor)
	report.EvidenceHash = checkpoint.EvidenceHash
	return report
}

func reportWithFailure(report Report, category string) Report {
	report.Status = StatusFailed
	report.Failures = []ReportFailure{{Category: category, Count: 1}}
	return report
}

func validLease(lease Lease, datasourceID, ownerID, fingerprint string, now time.Time) bool {
	return lease.DatasourceID == datasourceID && lease.OwnerID == ownerID &&
		lease.PolicyFingerprint == fingerprint && lease.Active(now)
}

func validCheckpoint(checkpoint Checkpoint, key CheckpointKey) bool {
	return checkpoint.Key == key && checkpoint.DatasourceID == key.DatasourceID &&
		checkpoint.Cursor.DatasourceID == key.DatasourceID && canonicalDigest(checkpoint.EvidenceHash) &&
		canonicalDigest(checkpoint.IntegrityHash) && checkpoint.IntegrityHash == checkpointDigest(checkpoint) &&
		checkpoint.Counts.Eligible >= 0 && checkpoint.Counts.Planned >= 0 && checkpoint.Counts.Applied >= 0 &&
		checkpoint.Counts.Batches >= 0 && checkpoint.Counts.Retries >= 0
}

func validateBatch(batch Batch, datasourceID string, previous Cursor, limit int) error {
	if batch.DatasourceID != datasourceID || len(batch.Candidates) > limit || len(batch.Candidates) == 0 && !batch.Done {
		return ErrPlanningFailed
	}
	if len(batch.Candidates) > 0 {
		if batch.NextCursor.DatasourceID != datasourceID || !cursorAfter(batch.NextCursor, previous) {
			return ErrPlanningFailed
		}
		seen := make(map[string]struct{}, len(batch.Candidates))
		for _, candidate := range batch.Candidates {
			if strings.TrimSpace(candidate.Resource) == "" || candidate.Resource != strings.TrimSpace(candidate.Resource) ||
				strings.TrimSpace(candidate.EligibleAt) == "" || candidate.EligibleAt != strings.TrimSpace(candidate.EligibleAt) ||
				strings.TrimSpace(candidate.RecordID) == "" || candidate.RecordID != strings.TrimSpace(candidate.RecordID) {
				return ErrPlanningFailed
			}
			key := candidate.Resource + "\x00" + candidate.EligibleAt + "\x00" + candidate.RecordID
			if _, duplicate := seen[key]; duplicate {
				return ErrPlanningFailed
			}
			seen[key] = struct{}{}
		}
	}
	return nil
}

func cursorAfter(next, previous Cursor) bool {
	if previous.Resource == "" && previous.EligibleAt == "" && previous.RecordID == "" {
		return next.Resource != "" && next.EligibleAt != "" && next.RecordID != ""
	}
	left := next.Resource + "\x00" + next.EligibleAt + "\x00" + next.RecordID
	right := previous.Resource + "\x00" + previous.EligibleAt + "\x00" + previous.RecordID
	return left > right
}

func batchDigest(datasourceID, fingerprint string, previous Cursor, batch Batch) string {
	values := []string{
		datasourceID, fingerprint, previous.Resource, previous.EligibleAt, previous.RecordID,
		batch.NextCursor.Resource, batch.NextCursor.EligibleAt, batch.NextCursor.RecordID,
		strconv.FormatBool(batch.Done),
	}
	for _, candidate := range batch.Candidates {
		values = append(values, candidate.Resource, candidate.EligibleAt, candidate.RecordID)
	}
	return stableDigest("batch", values...)
}

func sanitizedCursor(cursor Cursor) Cursor {
	result := cursor
	if result.RecordID != "" {
		result.RecordID = stableDigest("report-cursor", cursor.DatasourceID, cursor.Resource, cursor.EligibleAt, cursor.RecordID)
	}
	return result
}

func impactReportDigest(report ImpactReport) string {
	return stableDigest(
		"impact-report", report.DatasourceID, report.RunID, report.PolicyFingerprint,
		strconv.Itoa(report.Counts.Eligible), strconv.Itoa(report.Counts.Batches),
		report.Cursor.Resource, report.Cursor.EligibleAt, report.Cursor.RecordID, report.EvidenceHash,
	)
}

func validIdentifier(value string) bool {
	if value == "" || len(value) > 128 || value != strings.TrimSpace(value) {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' || character == '-' || character == '_' || character == '.' {
			continue
		}
		return false
	}
	return true
}

func retentionShortened(current, proposed PolicySnapshot) bool {
	currentByResource := make(map[string]ResourcePolicy, len(current.Resources))
	for _, resource := range current.Resources {
		currentByResource[resource.Resource] = resource
	}
	for _, resource := range proposed.Resources {
		previous, ok := currentByResource[resource.Resource]
		if ok && (resource.RetentionDays < previous.RetentionDays || !previous.AutoPurge && resource.AutoPurge) {
			return true
		}
		if !ok && resource.AutoPurge {
			return true
		}
	}
	return false
}

func (r *Runner) validatePromotion(ctx context.Context, datasourceID, proposedFingerprint string, approval PromotionApproval) error {
	if approval.PromotedFingerprint != proposedFingerprint || !canonicalDigest(approval.ImpactReportHash) ||
		!canonicalDigest(approval.CurrentFingerprint) || !validIdentifier(approval.DryRunID) ||
		!validIdentifier(approval.ActorID) || strings.TrimSpace(approval.Reason) == "" || approval.Reason != strings.TrimSpace(approval.Reason) ||
		strings.TrimSpace(approval.ApprovalRef) == "" || approval.ApprovalRef != strings.TrimSpace(approval.ApprovalRef) {
		return ErrPromotionRequired
	}
	promotion, found, err := r.repository.LoadPromotion(ctx, datasourceID, proposedFingerprint)
	if err != nil {
		return ErrRepositoryFailed
	}
	if !found || promotion.DatasourceID != datasourceID ||
		promotion.CurrentFingerprint != approval.CurrentFingerprint || promotion.PromotedFingerprint != proposedFingerprint || promotion.ImpactReportHash != approval.ImpactReportHash ||
		promotion.ActorID != approval.ActorID || promotion.Reason != approval.Reason || promotion.ApprovalRef != approval.ApprovalRef {
		return ErrPromotionRequired
	}
	dryRunKey := CheckpointKey{
		DatasourceID: datasourceID, RunID: approval.DryRunID, Mode: ModeDryRun,
		PolicyFingerprint: proposedFingerprint,
	}
	dryRun, found, err := r.repository.LoadCheckpoint(ctx, dryRunKey)
	if err != nil {
		return ErrRepositoryFailed
	}
	if !found || !dryRun.Complete || !validCheckpoint(dryRun, dryRunKey) {
		return ErrPromotionRequired
	}
	return nil
}

func boundedOperationContext(parent context.Context, timeout time.Duration, leaseExpiresAt, now time.Time) (context.Context, context.CancelFunc, bool) {
	remaining := leaseExpiresAt.Sub(now)
	if remaining <= 0 {
		return nil, func() {}, false
	}
	if timeout > remaining {
		timeout = remaining
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	return ctx, cancel, true
}
