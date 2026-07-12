package sensitivemigration

import (
	"context"
	"encoding/json"
	"reflect"
	"slices"
	"strings"

	"platform-go/internal/platform/dataprotection"
)

type Runner struct {
	plan    Plan
	runtime dataprotection.Runtime
	store   ReadStore
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
	return &Runner{plan: Plan{Resources: resources}, runtime: runtime, store: store}
}

func (r *Runner) Run(ctx context.Context, options Options) (Report, error) {
	report := Report{Status: StatusFailed}
	batchSize, err := readOnlyBatchSize(options)
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

func readOnlyBatchSize(options Options) (int, error) {
	if options.Mode != ModeInventory && options.Mode != ModeDryRun {
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
	values := map[string]any{}
	if err := json.Unmarshal([]byte(row.ValuesJSON), &values); err != nil {
		return ErrReadFailed
	}
	for _, field := range resource.Fields {
		if ctx.Err() != nil {
			return ErrReadFailed
		}
		raw, exists := values[field.Key]
		if !exists || raw == nil {
			counts.add(ClassificationMissing)
			continue
		}
		value, ok := raw.(string)
		if !ok {
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
