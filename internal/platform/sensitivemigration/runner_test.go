package sensitivemigration

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"

	"platform-go/internal/platform/dataprotection"
)

const (
	fixturePlaintext = "fixture-sensitive-value"
	fixtureTenant    = "fixture-tenant-value"
	fixtureRecord    = "fixture-record-value"
)

func TestValueClassificationCoversMissingPlaintextTargetForeignAndMalformed(t *testing.T) {
	ctx := context.Background()
	service := migrationTestRuntime(t)
	runtime := &trackingRuntime{Runtime: service}
	policy := dataprotection.FieldPolicy{
		Format:              dataprotection.FormatAES256GCMV1,
		Normalization:       dataprotection.NormalizationRawV1,
		BlindIndexNamespace: "fixture-index-namespace",
	}
	plan := Plan{Resources: []ResourcePlan{{
		Resource: "fixture-resources", Scope: "tenant-field", TenantField: "tenantCode", SchemaVersion: 3,
		Fields: []FieldPlan{
			{Key: "emptyValue", Policy: policy},
			{Key: "foreignPolicy", Policy: policy},
			{Key: "foreignVersion", Policy: policy},
			{Key: "invalidVersion", Policy: policy},
			{Key: "malformedValue", Policy: policy},
			{Key: "missingValue", Policy: policy},
			{Key: "plainValue", Policy: policy},
			{Key: "targetValue", Policy: policy},
		},
	}}}
	targetContext := dataprotection.FieldContext{
		TenantID: fixtureTenant, Resource: "fixture-resources", RecordID: fixtureRecord,
		FieldKey: "targetValue", SchemaVersion: 3,
	}
	targetEnvelope, err := service.Protect(ctx, fixturePlaintext, policy, targetContext)
	if err != nil {
		t.Fatal(err)
	}
	foreignContext := targetContext
	foreignContext.FieldKey = "foreignPolicy"
	foreignPolicy := policy
	foreignPolicy.Normalization = dataprotection.NormalizationTrimV1
	foreignEnvelope, err := service.Protect(ctx, fixturePlaintext, foreignPolicy, foreignContext)
	if err != nil {
		t.Fatal(err)
	}
	values, err := json.Marshal(map[string]any{
		"emptyValue":     "",
		"foreignPolicy":  foreignEnvelope,
		"foreignVersion": "pgo:enc:v2:fixture-envelope-payload",
		"invalidVersion": "pgo:enc:foreign:fixture-envelope-payload",
		"malformedValue": "pgo:enc:v1:fixture-envelope-payload",
		"plainValue":     fixturePlaintext,
		"targetValue":    targetEnvelope,
	})
	if err != nil {
		t.Fatal(err)
	}
	store := newMemoryReadStore(map[string]map[string][]Row{
		"fixture-resources": {fixtureTenant: {{Resource: "fixture-resources", RecordID: fixtureRecord, ValuesJSON: string(values)}}},
	})

	report, err := NewRunner(plan, runtime, store).Run(ctx, Options{Mode: ModeInventory, BatchSize: 10})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	want := Counts{Missing: 2, Plaintext: 1, TargetEnvelope: 1, ForeignEnvelope: 2, MalformedEnvelope: 2}
	if report.Counts != want {
		t.Fatalf("Run() counts = %+v, want %+v", report.Counts, want)
	}
	if runtime.revealCalls != 0 {
		t.Fatalf("classification called Reveal() %d times", runtime.revealCalls)
	}
}

func TestInventoryTraversesTenantScopesInBoundedBatchesWithDeterministicCounts(t *testing.T) {
	policy := dataprotection.FieldPolicy{Format: dataprotection.FormatAES256GCMV1, Normalization: dataprotection.NormalizationRawV1}
	plan := Plan{Resources: []ResourcePlan{
		{Resource: "zeta-resources", Scope: "global", SchemaVersion: 1, Fields: []FieldPlan{{Key: "secretValue", Policy: policy}}},
		{Resource: "alpha-resources", Scope: "tenant-field", TenantField: "tenantCode", SchemaVersion: 1, Fields: []FieldPlan{{Key: "secretValue", Policy: policy}}},
	}}
	row := func(resource string, recordID string, value string) Row {
		encoded, err := json.Marshal(map[string]string{"secretValue": value})
		if err != nil {
			t.Fatal(err)
		}
		return Row{Resource: resource, RecordID: recordID, ValuesJSON: string(encoded)}
	}
	store := newMemoryReadStore(map[string]map[string][]Row{
		"alpha-resources": {
			"tenant-b": {row("alpha-resources", "b-1", fixturePlaintext)},
			"tenant-a": {
				row("alpha-resources", "a-1", fixturePlaintext),
				row("alpha-resources", "a-2", ""),
				row("alpha-resources", "a-3", fixturePlaintext),
			},
		},
		"zeta-resources": {
			dataprotection.GlobalTenantID: {row("zeta-resources", "g-1", fixturePlaintext)},
		},
	})
	store.scopeOrder["alpha-resources"] = []string{"tenant-b", "tenant-a"}

	report, err := NewRunner(plan, migrationTestRuntime(t), store).Run(context.Background(), Options{Mode: ModeInventory, BatchSize: 2})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Status != StatusCompleted || report.Mode != ModeInventory {
		t.Fatalf("Run() report = %+v, want completed inventory", report)
	}
	if report.Counts != (Counts{Missing: 1, Plaintext: 4}) {
		t.Fatalf("Run() counts = %+v, want deterministic aggregate counts", report.Counts)
	}
	if report.Checkpoints != 4 {
		t.Fatalf("Run() checkpoints = %d, want 4 completed batches", report.Checkpoints)
	}
	for _, call := range store.rowCalls {
		if call.limit != 2 {
			t.Fatalf("Rows() limit = %d, want 2", call.limit)
		}
	}
	wantFirstCalls := []readCall{
		{resource: "alpha-resources", tenant: "tenant-a", after: "", limit: 2},
		{resource: "alpha-resources", tenant: "tenant-a", after: "a-2", limit: 2},
	}
	if len(store.rowCalls) < len(wantFirstCalls) || !reflect.DeepEqual(store.rowCalls[:len(wantFirstCalls)], wantFirstCalls) {
		t.Fatalf("Rows() first calls = %+v, want %+v", store.rowCalls, wantFirstCalls)
	}
}

func TestInventoryContinuesAfterShortPagesUntilEmpty(t *testing.T) {
	policy := dataprotection.FieldPolicy{Format: dataprotection.FormatAES256GCMV1, Normalization: dataprotection.NormalizationRawV1}
	row := func(recordID string) Row {
		return Row{Resource: "fixture-resources", RecordID: recordID, ValuesJSON: `{"secretValue":"` + fixturePlaintext + `"}`}
	}
	store := &scriptedReadStore{pages: [][]Row{{row("record-1")}, {row("record-2")}, {}}}
	plan := Plan{Resources: []ResourcePlan{{
		Resource: "fixture-resources", Scope: "global", SchemaVersion: 1,
		Fields: []FieldPlan{{Key: "secretValue", Policy: policy}},
	}}}

	report, err := NewRunner(plan, migrationTestRuntime(t), store).Run(context.Background(), Options{Mode: ModeInventory, BatchSize: 2})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Counts != (Counts{Plaintext: 2}) || report.Checkpoints != 2 {
		t.Fatalf("Run() report = %+v, want both short pages counted", report)
	}
	if store.calls != 3 {
		t.Fatalf("Rows() calls = %d, want two short pages plus empty page", store.calls)
	}
}

func TestInventoryRejectsOutOfRangeBatchSizesWithoutReading(t *testing.T) {
	store := newMemoryReadStore(nil)
	runner := NewRunner(Plan{Resources: []ResourcePlan{{Resource: "fixture-resources"}}}, migrationTestRuntime(t), store)
	for _, batchSize := range []int{-1, 1001} {
		report, err := runner.Run(context.Background(), Options{Mode: ModeInventory, BatchSize: batchSize})
		if err == nil {
			t.Fatalf("Run(BatchSize=%d) error = nil, want sanitized validation failure", batchSize)
		}
		if report.Status != StatusFailed || len(store.rowCalls) != 0 {
			t.Fatalf("Run(BatchSize=%d) report = %+v calls = %+v", batchSize, report, store.rowCalls)
		}
	}
}

func TestSanitizedInvalidModeIsNotCopiedIntoFailedReport(t *testing.T) {
	mode := Mode(fixturePlaintext)
	report, err := NewRunner(Plan{}, migrationTestRuntime(t), newMemoryReadStore(nil)).Run(context.Background(), Options{Mode: mode})
	if !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("Run() error = %v, want ErrInvalidOptions", err)
	}
	if report.Mode != "" || report.Status != StatusFailed {
		t.Fatalf("Run() report = %+v, want failed report with empty mode", report)
	}
	encoded, marshalErr := json.Marshal(report)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	assertSanitized(t, string(encoded), err.Error())
}

func TestInventoryPropagatesRuntimeValidationFailuresAsSanitizedReadErrors(t *testing.T) {
	service := migrationTestRuntime(t)
	policy := dataprotection.FieldPolicy{Format: dataprotection.FormatAES256GCMV1, Normalization: dataprotection.NormalizationRawV1}
	fieldContext := dataprotection.FieldContext{
		TenantID: dataprotection.GlobalTenantID, Resource: "fixture-resources", RecordID: fixtureRecord,
		FieldKey: "secretValue", SchemaVersion: 1,
	}
	envelope, err := service.Protect(context.Background(), fixturePlaintext, policy, fieldContext)
	if err != nil {
		t.Fatal(err)
	}
	values, err := json.Marshal(map[string]string{"secretValue": envelope})
	if err != nil {
		t.Fatal(err)
	}
	plan := Plan{Resources: []ResourcePlan{{
		Resource: "fixture-resources", Scope: "global", SchemaVersion: 1,
		Fields: []FieldPlan{{Key: "secretValue", Policy: policy}},
	}}}

	for _, tt := range []struct {
		name        string
		validateErr error
	}{
		{name: "unexpected runtime failure", validateErr: errors.New("runtime failed: " + fixturePlaintext + " " + fixtureRecord)},
		{name: "key unavailable", validateErr: dataprotection.ErrKeyUnavailable},
	} {
		t.Run(tt.name, func(t *testing.T) {
			store := newMemoryReadStore(map[string]map[string][]Row{
				"fixture-resources": {dataprotection.GlobalTenantID: {{Resource: "fixture-resources", RecordID: fixtureRecord, ValuesJSON: string(values)}}},
			})
			runtime := &validationRuntime{Service: service, validateErr: tt.validateErr}
			report, runErr := NewRunner(plan, runtime, store).Run(context.Background(), Options{Mode: ModeInventory})
			if !errors.Is(runErr, ErrReadFailed) {
				t.Fatalf("Run() error = %v, want ErrReadFailed", runErr)
			}
			if report.Counts != (Counts{}) {
				t.Fatalf("Run() counts = %+v, want no malformed classification", report.Counts)
			}
			encoded, marshalErr := json.Marshal(report)
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			assertSanitized(t, string(encoded), runErr.Error())
		})
	}
}

func TestInventoryStopsOnCancellationDuringRowAndFieldTraversal(t *testing.T) {
	policy := dataprotection.FieldPolicy{Format: dataprotection.FormatAES256GCMV1, Normalization: dataprotection.NormalizationRawV1}
	plan := Plan{Resources: []ResourcePlan{{
		Resource: "fixture-resources", Scope: "global", SchemaVersion: 1,
		Fields: []FieldPlan{{Key: "alphaSecret", Policy: policy}, {Key: "zetaSecret", Policy: policy}},
	}}}

	t.Run("rows", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		store := &scriptedReadStore{
			pages:  [][]Row{{{Resource: "fixture-resources", RecordID: fixtureRecord, ValuesJSON: `{"alphaSecret":"` + fixturePlaintext + `"}`}}},
			onRows: cancel,
		}
		report, err := NewRunner(plan, migrationTestRuntime(t), store).Run(ctx, Options{Mode: ModeInventory})
		if !errors.Is(err, ErrReadFailed) || report.Counts != (Counts{}) {
			t.Fatalf("Run() report = %+v error = %v, want canceled read failure before row classification", report, err)
		}
	})

	t.Run("fields", func(t *testing.T) {
		service := migrationTestRuntime(t)
		ctx, cancel := context.WithCancel(context.Background())
		fieldContext := dataprotection.FieldContext{
			TenantID: dataprotection.GlobalTenantID, Resource: "fixture-resources", RecordID: fixtureRecord,
			FieldKey: "alphaSecret", SchemaVersion: 1,
		}
		envelope, err := service.Protect(ctx, fixturePlaintext, policy, fieldContext)
		if err != nil {
			t.Fatal(err)
		}
		values, err := json.Marshal(map[string]string{"alphaSecret": envelope, "zetaSecret": fixturePlaintext})
		if err != nil {
			t.Fatal(err)
		}
		store := newMemoryReadStore(map[string]map[string][]Row{
			"fixture-resources": {dataprotection.GlobalTenantID: {{Resource: "fixture-resources", RecordID: fixtureRecord, ValuesJSON: string(values)}}},
		})
		runtime := &validationRuntime{Service: service, cancel: cancel}
		report, runErr := NewRunner(plan, runtime, store).Run(ctx, Options{Mode: ModeInventory})
		if !errors.Is(runErr, ErrReadFailed) || report.Counts != (Counts{}) {
			t.Fatalf("Run() report = %+v error = %v, want canceled read failure before later fields", report, runErr)
		}
	})
}

func TestInventoryRequiresReadyRuntimeBeforePlaintextTraversal(t *testing.T) {
	policy := dataprotection.FieldPolicy{Format: dataprotection.FormatAES256GCMV1, Normalization: dataprotection.NormalizationRawV1}
	plan := Plan{Resources: []ResourcePlan{{
		Resource: "fixture-resources", Scope: "global", SchemaVersion: 1,
		Fields: []FieldPlan{{Key: "secretValue", Policy: policy}},
	}}}
	rows := map[string]map[string][]Row{
		"fixture-resources": {dataprotection.GlobalTenantID: {{Resource: "fixture-resources", RecordID: fixtureRecord, ValuesJSON: `{"secretValue":"` + fixturePlaintext + `"}`}}},
	}
	var typedNil *dataprotection.Service
	for _, tt := range []struct {
		name    string
		runtime dataprotection.Runtime
	}{
		{name: "typed nil runtime", runtime: typedNil},
		{name: "provider unavailable", runtime: dataprotection.NewRuntime(nil)},
		{name: "readiness contract missing", runtime: runtimeWithoutReadiness{Runtime: migrationTestRuntime(t)}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			store := newMemoryReadStore(rows)
			report, err := NewRunner(plan, tt.runtime, store).Run(context.Background(), Options{Mode: ModeInventory})
			if !errors.Is(err, ErrReadFailed) {
				t.Fatalf("Run() error = %v, want ErrReadFailed", err)
			}
			if report.Status != StatusFailed || report.Counts != (Counts{}) || store.scopeCalls != 0 {
				t.Fatalf("Run() report = %+v scope calls = %d, want readiness failure before reads", report, store.scopeCalls)
			}
		})
	}
}

func TestInventoryRejectsGenericTypedNilDependenciesBeforeDispatch(t *testing.T) {
	plan := Plan{Resources: []ResourcePlan{{Resource: "fixture-resources", Scope: "global", SchemaVersion: 1}}}

	t.Run("read store", func(t *testing.T) {
		var store *memoryReadStore
		report, err := NewRunner(plan, migrationTestRuntime(t), store).Run(context.Background(), Options{Mode: ModeInventory})
		if !errors.Is(err, ErrInvalidOptions) || report.Status != StatusFailed {
			t.Fatalf("Run() report = %+v error = %v, want sanitized typed-nil store failure", report, err)
		}
	})

	t.Run("runtime readiness wrapper", func(t *testing.T) {
		var runtime *typedNilReadyRuntime
		store := newMemoryReadStore(nil)
		report, err := NewRunner(plan, runtime, store).Run(context.Background(), Options{Mode: ModeInventory})
		if !errors.Is(err, ErrReadFailed) || report.Status != StatusFailed || store.scopeCalls != 0 {
			t.Fatalf("Run() report = %+v error = %v scope calls = %d, want failure before runtime/store dispatch", report, err, store.scopeCalls)
		}
	})
}

func TestInventoryRejectsCancellationReturnedWithEmptyTenantScopes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	store := &cancelingScopeStore{cancel: cancel}
	plan := Plan{Resources: []ResourcePlan{{Resource: "fixture-resources", Scope: "global", SchemaVersion: 1}}}

	report, err := NewRunner(plan, migrationTestRuntime(t), store).Run(ctx, Options{Mode: ModeInventory})
	if !errors.Is(err, ErrReadFailed) || report.Status != StatusFailed || report.Counts != (Counts{}) {
		t.Fatalf("Run() report = %+v error = %v, want canceled scope read failure", report, err)
	}
}

func TestDryRunIsIdempotentForTargetEnvelopesAndNeverProtectsOrReveals(t *testing.T) {
	service := migrationTestRuntime(t)
	runtime := &trackingRuntime{Runtime: service}
	policy := dataprotection.FieldPolicy{Format: dataprotection.FormatAES256GCMV1, Normalization: dataprotection.NormalizationRawV1}
	fieldContext := dataprotection.FieldContext{
		TenantID: dataprotection.GlobalTenantID, Resource: "global-resources", RecordID: "record-1", FieldKey: "secretValue", SchemaVersion: 1,
	}
	envelope, err := service.Protect(context.Background(), fixturePlaintext, policy, fieldContext)
	if err != nil {
		t.Fatal(err)
	}
	values, err := json.Marshal(map[string]string{"secretValue": envelope})
	if err != nil {
		t.Fatal(err)
	}
	plan := Plan{Resources: []ResourcePlan{{
		Resource: "global-resources", Scope: "global", SchemaVersion: 1,
		Fields: []FieldPlan{{Key: "secretValue", Policy: policy}},
	}}}
	store := newMemoryReadStore(map[string]map[string][]Row{
		"global-resources": {dataprotection.GlobalTenantID: {{Resource: "global-resources", RecordID: "record-1", ValuesJSON: string(values)}}},
	})

	for run := 0; run < 2; run++ {
		report, runErr := NewRunner(plan, runtime, store).Run(context.Background(), Options{Mode: ModeDryRun})
		if runErr != nil {
			t.Fatalf("Run(%d) error = %v", run, runErr)
		}
		if report.Counts != (Counts{TargetEnvelope: 1}) {
			t.Fatalf("Run(%d) counts = %+v, want one target envelope", run, report.Counts)
		}
	}
	if runtime.protectCalls != 0 || runtime.revealCalls != 0 {
		t.Fatalf("dry-run calls: Protect=%d Reveal=%d, want zero", runtime.protectCalls, runtime.revealCalls)
	}
}

func TestSanitizedRunnerErrorsAndReportsContainNoStoredValuesOrCoordinates(t *testing.T) {
	policy := dataprotection.FieldPolicy{Format: dataprotection.FormatAES256GCMV1, Normalization: dataprotection.NormalizationRawV1}
	plan := Plan{Resources: []ResourcePlan{{
		Resource: "fixture-resources", Scope: "tenant-field", TenantField: "tenantCode", SchemaVersion: 1,
		Fields: []FieldPlan{{Key: "secretValue", Policy: policy}},
	}}}
	for _, tt := range []struct {
		name  string
		store *memoryReadStore
	}{
		{
			name:  "tenant scope failure",
			store: &memoryReadStore{tenantErr: errors.New("read failed: " + fixturePlaintext + " " + fixtureTenant + " " + fixtureRecord + " pgo:enc:v1:fixture")},
		},
		{
			name: "row failure",
			store: &memoryReadStore{
				scopeOrder: map[string][]string{"fixture-resources": {fixtureTenant}},
				rowsErr:    errors.New("read failed: " + fixturePlaintext + " " + fixtureTenant + " " + fixtureRecord + " pgo:enc:v1:fixture"),
			},
		},
		{
			name: "malformed values json",
			store: newMemoryReadStore(map[string]map[string][]Row{
				"fixture-resources": {fixtureTenant: {{Resource: "fixture-resources", RecordID: fixtureRecord, ValuesJSON: `{"secretValue":"` + fixturePlaintext}}},
			}),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			report, err := NewRunner(plan, migrationTestRuntime(t), tt.store).Run(context.Background(), Options{Mode: ModeInventory})
			if err == nil {
				t.Fatal("Run() error = nil, want sanitized failure")
			}
			encoded, marshalErr := json.Marshal(report)
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			assertSanitized(t, string(encoded), err.Error())
		})
	}
}

func TestSanitizedSuccessfulReportContainsCountsOnly(t *testing.T) {
	policy := dataprotection.FieldPolicy{Format: dataprotection.FormatAES256GCMV1, Normalization: dataprotection.NormalizationRawV1, BlindIndexNamespace: "fixture-blind-index"}
	service := migrationTestRuntime(t)
	fieldContext := dataprotection.FieldContext{TenantID: fixtureTenant, Resource: "fixture-resources", RecordID: fixtureRecord, FieldKey: "secretValue", SchemaVersion: 1}
	envelope, err := service.Protect(context.Background(), fixturePlaintext, policy, fieldContext)
	if err != nil {
		t.Fatal(err)
	}
	values, err := json.Marshal(map[string]string{"secretValue": envelope, "tenantCode": fixtureTenant})
	if err != nil {
		t.Fatal(err)
	}
	plan := Plan{Resources: []ResourcePlan{{
		Resource: "fixture-resources", Scope: "tenant-field", TenantField: "tenantCode", SchemaVersion: 1,
		Fields: []FieldPlan{{Key: "secretValue", Policy: policy}},
	}}}
	store := newMemoryReadStore(map[string]map[string][]Row{
		"fixture-resources": {fixtureTenant: {{Resource: "fixture-resources", RecordID: fixtureRecord, ValuesJSON: string(values)}}},
	})
	report, err := NewRunner(plan, service, store).Run(context.Background(), Options{Mode: ModeInventory})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	assertSanitized(t, string(encoded))
}

func assertSanitized(t *testing.T, values ...string) {
	t.Helper()
	for _, value := range values {
		for _, forbidden := range []string{fixturePlaintext, fixtureTenant, fixtureRecord, "pgo:enc:", "fixture-envelope-payload", "fixture-blind-index"} {
			if strings.Contains(value, forbidden) {
				t.Fatalf("output contains forbidden fixture marker %q", forbidden)
			}
		}
	}
}

type trackingRuntime struct {
	dataprotection.Runtime
	protectCalls int
	revealCalls  int
}

func (r *trackingRuntime) Ready(ctx context.Context) error {
	readiness, ok := r.Runtime.(dataprotection.RuntimeReadiness)
	if !ok {
		return dataprotection.ErrKeyUnavailable
	}
	return readiness.Ready(ctx)
}

type validationRuntime struct {
	*dataprotection.Service
	validateErr error
	cancel      context.CancelFunc
}

type runtimeWithoutReadiness struct {
	dataprotection.Runtime
}

type typedNilReadyRuntime struct {
	dataprotection.Runtime
}

func (*typedNilReadyRuntime) Ready(context.Context) error {
	panic("typed-nil runtime readiness was dispatched")
}

func (r *validationRuntime) Validate(context.Context, string, dataprotection.FieldPolicy, dataprotection.FieldContext) error {
	if r.cancel != nil {
		r.cancel()
	}
	return r.validateErr
}

func (r *trackingRuntime) Protect(ctx context.Context, value string, policy dataprotection.FieldPolicy, fieldContext dataprotection.FieldContext) (string, error) {
	r.protectCalls++
	return r.Runtime.Protect(ctx, value, policy, fieldContext)
}

func (r *trackingRuntime) Reveal(ctx context.Context, value string, policy dataprotection.FieldPolicy, fieldContext dataprotection.FieldContext) (string, error) {
	r.revealCalls++
	return "", errors.New("Reveal called with " + value + " " + fieldContext.RecordID)
}

type readCall struct {
	resource string
	tenant   string
	after    string
	limit    int
}

type memoryReadStore struct {
	rows       map[string]map[string][]Row
	scopeOrder map[string][]string
	tenantErr  error
	rowsErr    error
	scopeCalls int
	rowCalls   []readCall
}

func newMemoryReadStore(rows map[string]map[string][]Row) *memoryReadStore {
	store := &memoryReadStore{rows: rows, scopeOrder: map[string][]string{}}
	for resource, scopes := range rows {
		for tenant := range scopes {
			store.scopeOrder[resource] = append(store.scopeOrder[resource], tenant)
		}
		sort.Sort(sort.Reverse(sort.StringSlice(store.scopeOrder[resource])))
	}
	return store
}

func (s *memoryReadStore) TenantScopes(_ context.Context, plan ResourcePlan) ([]string, error) {
	s.scopeCalls++
	if s.tenantErr != nil {
		return nil, s.tenantErr
	}
	return append([]string(nil), s.scopeOrder[plan.Resource]...), nil
}

type scriptedReadStore struct {
	pages  [][]Row
	calls  int
	onRows func()
}

type cancelingScopeStore struct {
	cancel context.CancelFunc
}

func (s *cancelingScopeStore) TenantScopes(context.Context, ResourcePlan) ([]string, error) {
	s.cancel()
	return nil, nil
}

func (*cancelingScopeStore) Rows(context.Context, ResourcePlan, string, string, int) ([]Row, error) {
	panic("Rows called after tenant-scope cancellation")
}

func (s *scriptedReadStore) TenantScopes(context.Context, ResourcePlan) ([]string, error) {
	return []string{dataprotection.GlobalTenantID}, nil
}

func (s *scriptedReadStore) Rows(context.Context, ResourcePlan, string, string, int) ([]Row, error) {
	if s.onRows != nil {
		s.onRows()
	}
	if s.calls >= len(s.pages) {
		s.calls++
		return nil, nil
	}
	page := append([]Row(nil), s.pages[s.calls]...)
	s.calls++
	return page, nil
}

func (s *memoryReadStore) Rows(_ context.Context, plan ResourcePlan, tenant string, after string, limit int) ([]Row, error) {
	s.rowCalls = append(s.rowCalls, readCall{resource: plan.Resource, tenant: tenant, after: after, limit: limit})
	if s.rowsErr != nil {
		return nil, s.rowsErr
	}
	rows := s.rows[plan.Resource][tenant]
	start := sort.Search(len(rows), func(index int) bool { return rows[index].RecordID > after })
	end := min(start+limit, len(rows))
	return append([]Row(nil), rows[start:end]...), nil
}

func migrationTestRuntime(t *testing.T) *dataprotection.Service {
	t.Helper()
	provider, err := dataprotection.NewStaticKeyProvider(dataprotection.StaticKeyProviderConfig{
		Kind:                  dataprotection.ProviderEnvAES256,
		ActiveEncryptionKeyID: "migration-enc-v1",
		EncryptionKeys:        map[string][]byte{"migration-enc-v1": []byte(strings.Repeat("e", 32))},
		ActiveBlindIndexKeyID: "migration-idx-v1",
		BlindIndexKeys:        map[string][]byte{"migration-idx-v1": []byte(strings.Repeat("i", 32))},
	})
	if err != nil {
		t.Fatal(err)
	}
	return dataprotection.NewRuntime(provider)
}
