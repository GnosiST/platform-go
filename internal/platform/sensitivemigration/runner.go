package sensitivemigration

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"slices"
	"strings"

	"platform-go/internal/platform/dataprotection"
)

type Runner struct {
	plan     Plan
	planHash string
	runtime  dataprotection.Runtime
	store    ReadStore
}

func NewRunner(plan Plan, runtime dataprotection.Runtime, store ReadStore) *Runner {
	resources := append([]ResourcePlan(nil), plan.Resources...)
	for index := range resources {
		resources[index].Fields = append([]FieldPlan(nil), resources[index].Fields...)
		slices.SortFunc(resources[index].Fields, func(left FieldPlan, right FieldPlan) int {
			return strings.Compare(left.Key, right.Key)
		})
	}
	slices.SortFunc(resources, func(left ResourcePlan, right ResourcePlan) int {
		return strings.Compare(left.Resource, right.Resource)
	})
	canonicalPlan := Plan{Resources: resources}
	return &Runner{plan: canonicalPlan, planHash: PlanHash(canonicalPlan), runtime: runtime, store: store}
}

func (r *Runner) Run(ctx context.Context, options Options) (Report, error) {
	report := Report{Status: StatusFailed}
	batchSize, err := batchSizeForMode(options)
	if err != nil {
		return report, ErrInvalidOptions
	}
	report.Mode = options.Mode
	if r == nil || r.runtime == nil || r.store == nil || len(r.plan.Resources) == 0 {
		return report, ErrInvalidOptions
	}
	if nilInterface(r.store) {
		return report, ErrInvalidOptions
	}
	if nilInterface(r.runtime) {
		return report, ErrReadFailed
	}
	readiness, ok := r.runtime.(dataprotection.RuntimeReadiness)
	if !ok || readiness.Ready(ctx) != nil {
		return report, ErrReadFailed
	}

	switch options.Mode {
	case ModeInventory, ModeDryRun:
		return r.runReadOnly(ctx, options.Mode, batchSize, report)
	case ModePrepare, ModeApply, ModeVerify, ModeRehearseRestore, ModeRollback:
		return r.runPrepared(ctx, options, batchSize, report)
	default:
		return Report{Status: StatusFailed}, ErrInvalidOptions
	}
}

func (r *Runner) runReadOnly(ctx context.Context, mode Mode, batchSize int, report Report) (Report, error) {
	report.Mode = mode
	for _, resource := range r.plan.Resources {
		if ctx.Err() != nil {
			return report, ErrReadFailed
		}
		scopes, scopeErr := r.store.TenantScopes(ctx, resource)
		if scopeErr != nil || ctx.Err() != nil {
			return report, ErrReadFailed
		}
		slices.Sort(scopes)
		for _, tenant := range scopes {
			if tenant == "" {
				return report, ErrReadFailed
			}
			if err := r.runScope(ctx, resource, tenant, batchSize, &report); err != nil {
				return report, err
			}
		}
	}

	report.Status = StatusCompleted
	return report, nil
}

func (r *Runner) runPrepared(ctx context.Context, options Options, batchSize int, report Report) (Report, error) {
	store, ok := r.store.(MutatingStore)
	if !ok || nilInterface(store) {
		return report, ErrInvalidOptions
	}
	request := options.Request
	request.Mode = options.Mode
	request.Plan = r.plan
	report.Mode = options.Mode
	if request.PlanHash != r.planHash {
		return report, ErrInvalidOptions
	}
	switch options.Mode {
	case ModePrepare:
		if !validMutationRequest(request) {
			return report, ErrInvalidOptions
		}
		report.RunID = request.RunID
		state, err := store.Prepare(ctx, request)
		if err != nil {
			return report, ErrMutationFailed
		}
		report.Status = state.Status
		report.EventChainHead = state.EventChainHead
		return report, nil
	case ModeApply:
		if !validMutationRequest(request) {
			return report, ErrInvalidOptions
		}
		report.RunID = request.RunID
		state, err := store.StartOrResume(ctx, request)
		if err != nil {
			return report, ErrMutationFailed
		}
		return r.runApply(ctx, store, request, batchSize, state, report)
	case ModeVerify:
		if !validRunIdentity(request) {
			return report, ErrInvalidOptions
		}
		report.RunID = request.RunID
		state, err := store.StartOrResume(ctx, request)
		if err != nil {
			return report, ErrVerifyFailed
		}
		return r.runVerify(ctx, store, request, batchSize, state, report)
	case ModeRehearseRestore:
		if !validRunIdentity(request) {
			return report, ErrInvalidOptions
		}
		restoreStore, ok := store.(RestoreStore)
		if !ok || nilInterface(restoreStore) {
			return report, ErrInvalidOptions
		}
		report.RunID = request.RunID
		state, err := store.StartOrResume(ctx, request)
		if err != nil {
			return report, ErrVerifyFailed
		}
		return r.runRehearsal(ctx, restoreStore, request, state, report)
	case ModeRollback:
		if !validMutationRequest(request) {
			return report, ErrInvalidOptions
		}
		restoreStore, ok := store.(RestoreStore)
		if !ok || nilInterface(restoreStore) {
			return report, ErrInvalidOptions
		}
		report.RunID = request.RunID
		state, err := store.StartOrResume(ctx, request)
		if err != nil {
			return report, ErrMutationFailed
		}
		return r.runRollback(ctx, restoreStore, request, batchSize, state, report)
	default:
		return Report{Status: StatusFailed}, ErrInvalidOptions
	}
}

func (r *Runner) runApply(ctx context.Context, store MutatingStore, request RunRequest, batchSize int, state RunState, report Report) (Report, error) {
	if state.RunID != request.RunID || state.PlanHash != request.PlanHash || state.TargetCount < 0 {
		return report, ErrMutationFailed
	}
	report.Counts = state.Counts
	report.Checkpoints = checkpointBatches(state.Checkpoints)
	report.EventChainHead = state.EventChainHead
	if state.Status == StatusCompleted {
		if report.Counts.total() != state.TargetCount {
			return report, ErrMutationFailed
		}
		report.Status = StatusCompleted
		return report, nil
	}
	revision := state.ExpectedRevision
	for _, resource := range r.plan.Resources {
		scopes, err := store.TargetScopes(ctx, request.RunID, resource)
		if err != nil || ctx.Err() != nil {
			return report, ErrMutationFailed
		}
		slices.Sort(scopes)
		for _, tenant := range scopes {
			if tenant == "" {
				return report, ErrMutationFailed
			}
			after := checkpointCursor(state.Checkpoints, resource.Resource, tenant)
			for {
				rows, err := store.TargetRows(ctx, request.RunID, resource, tenant, after, batchSize)
				if err != nil || ctx.Err() != nil || len(rows) > batchSize {
					return report, ErrMutationFailed
				}
				if len(rows) == 0 {
					break
				}
				mutationRows, counts, err := r.protectRows(ctx, request.RunID, resource, tenant, after, rows)
				if err != nil {
					return report, err
				}
				lastRecordID := rows[len(rows)-1].RecordID
				commit, err := store.ApplyBatch(ctx, BatchMutation{
					RunID: request.RunID, Mode: ModeApply, Resource: resource, TenantID: tenant,
					ExpectedRevision: revision, LastRecordID: lastRecordID, Rows: mutationRows, Counts: counts,
				})
				if err != nil || commit.LastRecordID != lastRecordID || commit.EventHash == "" {
					return report, ErrMutationFailed
				}
				revision = commit.Revision
				after = commit.LastRecordID
				report.Counts = report.Counts.plus(counts)
				report.Checkpoints++
				report.EventChainHead = commit.EventHash
			}
		}
	}
	if report.Counts.total() != state.TargetCount {
		return report, ErrMutationFailed
	}
	if err := store.FinishRun(ctx, request.RunID, StatusCompleted); err != nil {
		return report, ErrMutationFailed
	}
	report.Status = StatusCompleted
	return report, nil
}

func (r *Runner) protectRows(ctx context.Context, runID string, resource ResourcePlan, tenant string, after string, rows []Row) ([]RowMutation, Counts, error) {
	mutations := make([]RowMutation, 0, len(rows))
	counts := Counts{}
	for _, row := range rows {
		if ctx.Err() != nil || row.RecordID == "" || row.RecordID <= after || row.Resource != "" && row.Resource != resource.Resource {
			return nil, Counts{}, ErrMutationFailed
		}
		values, err := DecodeUniqueObject(row.ValuesJSON)
		if err != nil {
			return nil, Counts{}, ErrMutationFailed
		}
		rowEscrow := make([]EscrowEntry, 0, len(resource.Fields))
		for _, field := range resource.Fields {
			raw, exists := values[field.Key]
			if !exists || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
				counts.add(ClassificationMissing)
				continue
			}
			var value string
			if err := json.Unmarshal(raw, &value); err != nil {
				return nil, Counts{}, ErrMutationFailed
			}
			fieldContext := dataprotection.FieldContext{
				TenantID: tenant, Resource: resource.Resource, RecordID: row.RecordID,
				FieldKey: field.Key, SchemaVersion: resource.SchemaVersion,
			}
			classification, err := classifyValue(ctx, r.runtime, value, field.Policy, fieldContext)
			if err != nil {
				return nil, Counts{}, ErrMutationFailed
			}
			if classification == ClassificationForeignEnvelope || classification == ClassificationMalformedEnvelope {
				return nil, Counts{}, ErrMutationFailed
			}
			counts.add(classification)
			if classification == ClassificationPlaintext {
				protected, err := r.runtime.Protect(ctx, value, field.Policy, fieldContext)
				if err != nil || ctx.Err() != nil {
					return nil, Counts{}, ErrMutationFailed
				}
				encoded, err := json.Marshal(protected)
				if err != nil {
					return nil, Counts{}, ErrMutationFailed
				}
				values[field.Key] = encoded
				escrowPolicy, escrowContext := EscrowContext(runID, tenant, resource.Resource, row.RecordID, field.Key)
				protectedOriginal, err := r.runtime.Protect(ctx, value, escrowPolicy, escrowContext)
				if err != nil || ctx.Err() != nil {
					return nil, Counts{}, ErrMutationFailed
				}
				rowEscrow = append(rowEscrow, EscrowEntry{
					RunID: runID, Resource: resource.Resource, RecordID: row.RecordID, FieldKey: field.Key,
					TenantID: tenant, ProtectedOriginal: protectedOriginal, MigratedValueHash: HashMigratedValue(protected),
				})
			}
		}
		updated, err := json.Marshal(values)
		if err != nil {
			return nil, Counts{}, ErrMutationFailed
		}
		mutations = append(mutations, RowMutation{RecordID: row.RecordID, OriginalValuesJSON: row.ValuesJSON, UpdatedValuesJSON: string(updated), Escrow: rowEscrow})
		after = row.RecordID
	}
	return mutations, counts, nil
}

func (r *Runner) runRehearsal(ctx context.Context, store RestoreStore, request RunRequest, state RunState, report Report) (Report, error) {
	if state.RunID != request.RunID || state.PlanHash != request.PlanHash || state.Status != StatusCompleted || state.EscrowCount < 0 {
		return report, ErrVerifyFailed
	}
	report.EventChainHead = state.EventChainHead
	report.Counts.Plaintext = state.EscrowCount
	if state.RestoreRehearsed {
		report.Status = StatusCompleted
		return report, nil
	}
	entries, err := store.EscrowEntries(ctx, request.RunID)
	if err != nil || ctx.Err() != nil || len(entries) != state.EscrowCount {
		return report, ErrVerifyFailed
	}
	for _, entry := range entries {
		if ctx.Err() != nil || !validEscrowEntry(entry, request.RunID) {
			return report, ErrVerifyFailed
		}
		policy, fieldContext := EscrowContext(entry.RunID, entry.TenantID, entry.Resource, entry.RecordID, entry.FieldKey)
		if _, err := r.runtime.Reveal(ctx, entry.ProtectedOriginal, policy, fieldContext); err != nil || ctx.Err() != nil {
			return report, ErrVerifyFailed
		}
	}
	commit, err := store.CommitRehearsal(ctx, request.RunID, len(entries))
	if err != nil || commit.Rows != len(entries) || commit.EventHash == "" {
		return report, ErrVerifyFailed
	}
	report.Checkpoints = 1
	report.EventChainHead = commit.EventHash
	report.Status = StatusCompleted
	return report, nil
}

func (r *Runner) runRollback(ctx context.Context, store RestoreStore, request RunRequest, batchSize int, state RunState, report Report) (Report, error) {
	if state.RunID != request.RunID || state.PlanHash != request.PlanHash || state.Status != StatusCompleted || !state.RestoreRehearsed || state.EscrowCount < 0 {
		return report, ErrMutationFailed
	}
	report.Counts = state.RollbackCounts
	report.Checkpoints = checkpointBatches(state.RollbackCheckpoints)
	report.EventChainHead = state.EventChainHead
	if state.RollbackStatus == StatusCompleted {
		if report.Counts.Plaintext != state.EscrowCount {
			return report, ErrMutationFailed
		}
		report.Status = StatusCompleted
		return report, nil
	}
	if state.RollbackStatus != StatusNone {
		return report, ErrMutationFailed
	}
	revision := state.ExpectedRevision
	for _, resource := range r.plan.Resources {
		scopes, err := store.RollbackScopes(ctx, request.RunID, resource)
		if err != nil || ctx.Err() != nil {
			return report, ErrMutationFailed
		}
		slices.Sort(scopes)
		for _, tenant := range scopes {
			if tenant == "" {
				return report, ErrMutationFailed
			}
			after := checkpointCursor(state.RollbackCheckpoints, resource.Resource, tenant)
			for {
				rows, err := store.RollbackRows(ctx, request.RunID, resource, tenant, after, batchSize)
				if err != nil || ctx.Err() != nil || len(rows) > batchSize {
					return report, ErrMutationFailed
				}
				if len(rows) == 0 {
					break
				}
				mutations, counts, err := r.restoreRows(ctx, request.RunID, resource, tenant, after, rows)
				if err != nil {
					return report, err
				}
				lastRecordID := rows[len(rows)-1].RecordID
				commit, err := store.RollbackBatch(ctx, BatchMutation{
					RunID: request.RunID, Mode: ModeRollback, Resource: resource, TenantID: tenant,
					ExpectedRevision: revision, LastRecordID: lastRecordID, Rows: mutations, Counts: counts,
				})
				if err != nil || commit.LastRecordID != lastRecordID || commit.EventHash == "" {
					return report, ErrMutationFailed
				}
				revision = commit.Revision
				after = commit.LastRecordID
				report.Counts = report.Counts.plus(counts)
				report.Checkpoints++
				report.EventChainHead = commit.EventHash
			}
		}
	}
	if report.Counts.Plaintext != state.EscrowCount || report.Counts.total() != report.Counts.Plaintext {
		return report, ErrMutationFailed
	}
	if err := store.FinishRollback(ctx, request.RunID); err != nil {
		return report, ErrMutationFailed
	}
	report.Status = StatusCompleted
	return report, nil
}

func (r *Runner) restoreRows(ctx context.Context, runID string, resource ResourcePlan, tenant string, after string, rows []RollbackRow) ([]RowMutation, Counts, error) {
	mutations := make([]RowMutation, 0, len(rows))
	counts := Counts{}
	for _, row := range rows {
		if ctx.Err() != nil || row.RecordID == "" || row.RecordID <= after || row.Resource != "" && row.Resource != resource.Resource || len(row.Escrow) == 0 {
			return nil, Counts{}, ErrMutationFailed
		}
		values, err := DecodeUniqueObject(row.ValuesJSON)
		if err != nil {
			return nil, Counts{}, ErrMutationFailed
		}
		slices.SortFunc(row.Escrow, func(left, right EscrowEntry) int { return strings.Compare(left.FieldKey, right.FieldKey) })
		previousField := ""
		for _, entry := range row.Escrow {
			if !validEscrowEntry(entry, runID) || entry.Resource != resource.Resource || entry.RecordID != row.RecordID || entry.TenantID != tenant || entry.FieldKey <= previousField {
				return nil, Counts{}, ErrMutationFailed
			}
			raw, ok := values[entry.FieldKey]
			if !ok {
				return nil, Counts{}, ErrMutationFailed
			}
			var migrated string
			if err := json.Unmarshal(raw, &migrated); err != nil || HashMigratedValue(migrated) != entry.MigratedValueHash {
				return nil, Counts{}, ErrMutationFailed
			}
			policy, fieldContext := EscrowContext(entry.RunID, entry.TenantID, entry.Resource, entry.RecordID, entry.FieldKey)
			original, err := r.runtime.Reveal(ctx, entry.ProtectedOriginal, policy, fieldContext)
			if err != nil || ctx.Err() != nil {
				return nil, Counts{}, ErrMutationFailed
			}
			encoded, err := json.Marshal(original)
			if err != nil {
				return nil, Counts{}, ErrMutationFailed
			}
			values[entry.FieldKey] = encoded
			counts.Plaintext++
			previousField = entry.FieldKey
		}
		updated, err := json.Marshal(values)
		if err != nil {
			return nil, Counts{}, ErrMutationFailed
		}
		mutations = append(mutations, RowMutation{
			RecordID: row.RecordID, OriginalValuesJSON: row.ValuesJSON, UpdatedValuesJSON: string(updated),
			Escrow: append([]EscrowEntry(nil), row.Escrow...),
		})
		after = row.RecordID
	}
	return mutations, counts, nil
}

func validEscrowEntry(entry EscrowEntry, runID string) bool {
	return entry.RunID == runID && entry.Resource != "" && entry.RecordID != "" && entry.FieldKey != "" && entry.TenantID != "" &&
		entry.ProtectedOriginal != "" && canonicalSHA256(entry.MigratedValueHash)
}

func (r *Runner) runVerify(ctx context.Context, store MutatingStore, request RunRequest, batchSize int, state RunState, report Report) (Report, error) {
	if state.RunID != request.RunID || state.PlanHash != request.PlanHash || state.Status != StatusCompleted || state.Counts.total() != state.TargetCount {
		return report, ErrVerifyFailed
	}
	report.EventChainHead = state.EventChainHead
	for _, resource := range r.plan.Resources {
		scopes, err := store.TargetScopes(ctx, request.RunID, resource)
		if err != nil || ctx.Err() != nil {
			return report, ErrVerifyFailed
		}
		slices.Sort(scopes)
		for _, tenant := range scopes {
			after := ""
			for {
				rows, err := store.TargetRows(ctx, request.RunID, resource, tenant, after, batchSize)
				if err != nil || ctx.Err() != nil || len(rows) > batchSize {
					return report, ErrVerifyFailed
				}
				if len(rows) == 0 {
					break
				}
				for _, row := range rows {
					before := report.Counts
					if err := r.classifyRow(ctx, resource, tenant, row, &report.Counts); err != nil {
						return report, ErrVerifyFailed
					}
					delta := Counts{
						Missing: report.Counts.Missing - before.Missing, Plaintext: report.Counts.Plaintext - before.Plaintext,
						TargetEnvelope:    report.Counts.TargetEnvelope - before.TargetEnvelope,
						ForeignEnvelope:   report.Counts.ForeignEnvelope - before.ForeignEnvelope,
						MalformedEnvelope: report.Counts.MalformedEnvelope - before.MalformedEnvelope,
					}
					if delta.Plaintext != 0 || delta.ForeignEnvelope != 0 || delta.MalformedEnvelope != 0 || row.RecordID == "" || row.RecordID <= after {
						return report, ErrVerifyFailed
					}
					after = row.RecordID
				}
				report.Checkpoints++
			}
		}
	}
	if report.Counts.total() != state.TargetCount {
		return report, ErrVerifyFailed
	}
	report.Status = StatusCompleted
	return report, nil
}

func checkpointCursor(checkpoints []CheckpointState, resource string, tenant string) string {
	for _, checkpoint := range checkpoints {
		if checkpoint.Resource == resource && checkpoint.TenantID == tenant {
			return checkpoint.LastRecordID
		}
	}
	return ""
}

func checkpointBatches(checkpoints []CheckpointState) int {
	total := 0
	for _, checkpoint := range checkpoints {
		total += checkpoint.Batches
	}
	return total
}

func nilInterface(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

func batchSizeForMode(options Options) (int, error) {
	if options.Mode != ModeInventory && options.Mode != ModeDryRun && options.Mode != ModePrepare && options.Mode != ModeApply && options.Mode != ModeVerify && options.Mode != ModeRehearseRestore && options.Mode != ModeRollback {
		return 0, ErrInvalidOptions
	}
	if options.BatchSize == 0 {
		return DefaultBatchSize, nil
	}
	if options.BatchSize < 1 || options.BatchSize > MaximumBatchSize {
		return 0, ErrInvalidOptions
	}
	return options.BatchSize, nil
}

func (r *Runner) runScope(ctx context.Context, resource ResourcePlan, tenant string, batchSize int, report *Report) error {
	after := ""
	for {
		if ctx.Err() != nil {
			return ErrReadFailed
		}
		rows, err := r.store.Rows(ctx, resource, tenant, after, batchSize)
		if err != nil || ctx.Err() != nil || len(rows) > batchSize {
			return ErrReadFailed
		}
		if len(rows) == 0 {
			return nil
		}
		for _, row := range rows {
			if ctx.Err() != nil || row.RecordID == "" || row.RecordID <= after || row.Resource != "" && row.Resource != resource.Resource {
				return ErrReadFailed
			}
			if err := r.classifyRow(ctx, resource, tenant, row, &report.Counts); err != nil {
				return err
			}
			after = row.RecordID
		}
		report.Checkpoints++
	}
}

func (r *Runner) classifyRow(ctx context.Context, resource ResourcePlan, tenant string, row Row, counts *Counts) error {
	if ctx.Err() != nil {
		return ErrReadFailed
	}
	values, err := DecodeUniqueObject(row.ValuesJSON)
	if err != nil {
		return ErrReadFailed
	}
	for _, field := range resource.Fields {
		if ctx.Err() != nil {
			return ErrReadFailed
		}
		raw, exists := values[field.Key]
		if !exists || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			counts.add(ClassificationMissing)
			continue
		}
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			counts.add(ClassificationMalformedEnvelope)
			continue
		}
		classification, err := classifyValue(ctx, r.runtime, value, field.Policy, dataprotection.FieldContext{
			TenantID: tenant, Resource: resource.Resource, RecordID: row.RecordID,
			FieldKey: field.Key, SchemaVersion: resource.SchemaVersion,
		})
		if err != nil {
			return err
		}
		counts.add(classification)
	}
	return nil
}
