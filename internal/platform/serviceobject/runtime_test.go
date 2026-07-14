package serviceobject

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"platform-go/internal/platform/kernel"
	"platform-go/internal/platform/storage"

	"gorm.io/gorm"
)

const referenceTable = "platform_service_object_reference_records"

type referenceRecord struct {
	ID       int64  `gorm:"column:id;primaryKey;autoIncrement"`
	TenantID string `gorm:"column:tenant_id;not null;index"`
	OrgCode  string `gorm:"column:org_code;index"`
	AreaCode string `gorm:"column:area_code;index"`
	Owner    string `gorm:"column:owner_code;index"`
	Code     string `gorm:"column:code;not null;index"`
	Name     string `gorm:"column:name;not null"`
	Status   string `gorm:"column:status;not null"`
}

func TestGORMRuntimeAppliesTrustedPrincipalDataScopeToQueriesAndCommands(t *testing.T) {
	runtime, db := newReferenceRuntime(t, nil)
	if err := db.Model(&referenceRecord{}).Where("tenant_id = ? AND code = ?", "tenant-1", "A-001").Updates(map[string]any{"org_code": "org-a", "area_code": "area-a", "owner_code": "admin"}).Error; err != nil {
		t.Fatalf("scope tenant 1 A: %v", err)
	}
	if err := db.Model(&referenceRecord{}).Where("tenant_id = ? AND code = ?", "tenant-1", "B-001").Updates(map[string]any{"org_code": "org-b", "area_code": "area-b", "owner_code": "other"}).Error; err != nil {
		t.Fatalf("scope tenant 1 B: %v", err)
	}

	invocation := referenceExecution("admin:reference-records:read", "read", "tenant-1")
	for name, scope := range map[string]ScopeConstraint{
		"org":  {OrgCodes: []string{"org-a"}},
		"area": {AreaCodes: []string{"area-a"}},
		"self": {Self: true, ActorIdentifiers: []string{"admin", "user-admin"}},
	} {
		invocation.Scope = scope
		result, err := runtime.ExecuteQuery(invocation, QueryRequest{QueryID: ReferenceQueryID, Version: ReferenceVersion, Pagination: Pagination{Page: 1, PageSize: 10}})
		if err != nil || len(result.Items) != 1 || result.Items[0]["code"] != "A-001" {
			t.Fatalf("ExecuteQuery(%s scoped) = %+v, %v, want only A-001", name, result, err)
		}
	}
	if err := db.Model(&referenceRecord{}).Where("tenant_id = ? AND code = ?", "tenant-1", "B-001").Update("owner_code", "admin").Error; err != nil {
		t.Fatalf("assign cross-org self owner: %v", err)
	}
	invocation.Scope = ScopeConstraint{OrgCodes: []string{"org-a"}, Self: true, ActorIdentifiers: []string{"admin"}}
	result, err := runtime.ExecuteQuery(invocation, QueryRequest{QueryID: ReferenceQueryID, Version: ReferenceVersion, Pagination: Pagination{Page: 1, PageSize: 10}})
	if err != nil || len(result.Items) != 1 || result.Items[0]["code"] != "A-001" {
		t.Fatalf("ExecuteQuery(org+self) = %+v, %v, want org scope to remain authoritative", result, err)
	}

	commandInvocation := invocation
	commandInvocation.Execution.PermissionIntent = kernel.PermissionIntent{Code: "admin:reference-records:update", Action: "update"}
	commandInvocation.Scope = ScopeConstraint{OrgCodes: []string{"org-b"}}
	command, err := runtime.ExecuteCommand(commandInvocation, CommandRequest{
		CommandID: ReferenceCommandID, Version: ReferenceVersion,
		Arguments: map[string]any{"code": "A-001", "name": "Out of scope"}, IdempotencyKey: "scope-denied-a",
	})
	if err != nil || command.Values["affected"] != int64(0) {
		t.Fatalf("ExecuteCommand(out of scope) = %+v, %v, want affected=0", command, err)
	}
	var unchanged referenceRecord
	if err := db.Where("tenant_id = ? AND code = ?", "tenant-1", "A-001").First(&unchanged).Error; err != nil || unchanged.Name != "Alpha" {
		t.Fatalf("out-of-scope row = %+v, %v, want unchanged", unchanged, err)
	}

	movingCommand := ReferenceCommandDefinition()
	movingCommand.Arguments = append(movingCommand.Arguments, ArgumentDefinition{Name: "targetOrg", Type: ValueString, Required: true, MaxLength: 64})
	movingCommand.Build = func(arguments ValidatedArguments) (CommandAST, error) {
		return CommandAST{
			Resource: "reference-records", Kind: MutationUpdate,
			Predicates: []Predicate{{Field: "code", Operator: PredicateEqual, Value: arguments["code"]}},
			Values:     map[string]any{"name": arguments["name"], "orgCode": arguments["targetOrg"]},
		}, nil
	}
	runtime.registry, err = NewRegistry([]QueryDefinition{ReferenceQueryDefinition()}, []CommandDefinition{movingCommand})
	if err != nil {
		t.Fatalf("NewRegistry(moving command) error = %v", err)
	}
	commandInvocation.Scope = ScopeConstraint{OrgCodes: []string{"org-a"}}
	_, err = runtime.ExecuteCommand(commandInvocation, CommandRequest{
		CommandID: ReferenceCommandID, Version: ReferenceVersion,
		Arguments: map[string]any{"code": "A-001", "name": "Moved", "targetOrg": "org-b"}, IdempotencyKey: "move-out-of-scope",
	})
	if !errors.Is(err, ErrObjectUnavailable) {
		t.Fatalf("ExecuteCommand(move out of scope) error = %v, want ErrObjectUnavailable", err)
	}
	if err := db.Where("tenant_id = ? AND code = ?", "tenant-1", "A-001").First(&unchanged).Error; err != nil || unchanged.Name != "Alpha" || unchanged.OrgCode != "org-a" {
		t.Fatalf("scope-move row = %+v, %v, want unchanged org-a Alpha", unchanged, err)
	}
}

func TestGORMRuntimeFailsClosedWhenBindingCannotApplyPrincipalScope(t *testing.T) {
	runtime, _ := newReferenceRuntime(t, nil)
	invocation := referenceExecution("admin:reference-records:read", "read", "tenant-1")
	invocation.Scope = ScopeConstraint{OrgCodes: []string{"org-a"}}
	runtime.queryExecutor.(*GORMExecutor).resources["reference-records"] = GORMResourceBinding{
		Table: referenceTable, TenantColumn: "tenant_id",
		PredicateColumns: map[string]string{"id": "id", "code": "code", "name": "name", "status": "status"},
		ValueColumns:     map[string]string{"name": "name"},
		ResultColumns:    map[string]string{"id": "id", "code": "code", "name": "name", "status": "status"},
	}
	if _, err := runtime.ExecuteQuery(invocation, QueryRequest{QueryID: ReferenceQueryID, Version: ReferenceVersion}); !errors.Is(err, ErrObjectUnavailable) {
		t.Fatalf("ExecuteQuery(missing scope binding) error = %v, want ErrObjectUnavailable", err)
	}
}

func TestGORMCommandRollsBackWhenAffectedRowsExceedDefinitionLimit(t *testing.T) {
	command := ReferenceCommandDefinition()
	command.Build = func(arguments ValidatedArguments) (CommandAST, error) {
		return CommandAST{Resource: "reference-records", Kind: MutationUpdate,
			Predicates: []Predicate{{Field: "status", Operator: PredicateEqual, Value: "enabled"}},
			Values:     map[string]any{"name": arguments["name"]}}, nil
	}
	runtime, db := newReferenceRuntime(t, nil)
	runtime.registry, _ = NewRegistry([]QueryDefinition{ReferenceQueryDefinition()}, []CommandDefinition{command})
	if err := db.Create(&referenceRecord{TenantID: "tenant-1", Code: "A-002", Name: "Second", Status: "enabled"}).Error; err != nil {
		t.Fatalf("seed second affected row: %v", err)
	}
	invocation := referenceExecution("admin:reference-records:update", "update", "tenant-1")
	_, err := runtime.ExecuteCommand(invocation, CommandRequest{CommandID: ReferenceCommandID, Version: ReferenceVersion,
		Arguments: map[string]any{"code": "ignored", "name": "Too broad"}, IdempotencyKey: "too-broad"})
	if !errors.Is(err, ErrCostLimitExceeded) {
		t.Fatalf("ExecuteCommand(too broad) error = %v, want ErrCostLimitExceeded", err)
	}
	var renamed int64
	if err := db.Model(&referenceRecord{}).Where("tenant_id = ? AND name = ?", "tenant-1", "Too broad").Count(&renamed).Error; err != nil || renamed != 0 {
		t.Fatalf("rolled-back renamed count = %d, %v, want 0", renamed, err)
	}
}

func (referenceRecord) TableName() string { return referenceTable }

func TestGORMRuntimeExecutesPersistedQueryWithTenantIsolationAndParameters(t *testing.T) {
	runtime, db := newReferenceRuntime(t, nil)
	execution := referenceExecution("admin:reference-records:read", "read", "tenant-1")
	result, err := runtime.ExecuteQuery(execution, QueryRequest{
		QueryID: ReferenceQueryID, Version: ReferenceVersion,
		Arguments:  map[string]any{"status": "enabled", "codePrefix": "A"},
		Pagination: Pagination{Page: 1, PageSize: 10},
		Sort:       []SortInput{{Name: "code", Order: "asc"}},
	})
	if err != nil {
		t.Fatalf("ExecuteQuery() error = %v", err)
	}
	if len(result.Items) != 1 || result.Items[0]["code"] != "A-001" || result.Total != nil {
		t.Fatalf("ExecuteQuery() result = %+v, want tenant 1 A-001 without total", result)
	}
	if _, leaked := result.Items[0]["tenant_id"]; leaked {
		t.Fatalf("ExecuteQuery() leaked physical tenant column: %+v", result.Items[0])
	}

	injection, err := runtime.ExecuteQuery(execution, QueryRequest{
		QueryID: ReferenceQueryID, Version: ReferenceVersion,
		Arguments:  map[string]any{"codePrefix": `' OR 1=1 --`},
		Pagination: Pagination{Page: 1, PageSize: 10},
	})
	if err != nil {
		t.Fatalf("ExecuteQuery(injection) error = %v", err)
	}
	if len(injection.Items) != 0 {
		t.Fatalf("ExecuteQuery(injection) returned %+v, want no rows", injection.Items)
	}
	var count int64
	if err := db.Table(referenceTable).Count(&count).Error; err != nil || count != 4 {
		t.Fatalf("reference table count = %d, %v, want 4", count, err)
	}
}

func TestRuntimeRejectsTamperingBeforeExecutor(t *testing.T) {
	executor := &querySpy{}
	runtime := newRuntimeWithExecutors(t, []QueryDefinition{ReferenceQueryDefinition()}, nil, executor, nil, nil)
	execution := referenceExecution("admin:reference-records:read", "read", "tenant-1")
	tampered := []QueryRequest{
		{QueryID: ReferenceQueryID, Version: ReferenceVersion, Arguments: map[string]any{"field": "status"}},
		{QueryID: ReferenceQueryID, Version: ReferenceVersion, Arguments: map[string]any{"status": 17}},
		{QueryID: ReferenceQueryID, Version: ReferenceVersion, Arguments: map[string]any{"status": strings.Repeat("x", 33)}},
		{QueryID: ReferenceQueryID, Version: ReferenceVersion, Sort: []SortInput{{Name: "database", Order: "asc"}}},
	}
	for _, request := range tampered {
		if _, err := runtime.ExecuteQuery(execution, request); !errors.Is(err, ErrRequestInvalid) {
			t.Fatalf("ExecuteQuery(%+v) error = %v, want ErrRequestInvalid", request, err)
		}
	}
	if executor.calls != 0 {
		t.Fatalf("query executor calls = %d, want 0", executor.calls)
	}
}

func TestRuntimeMakesUnknownAndUnauthorizedQueriesIndistinguishable(t *testing.T) {
	executor := &querySpy{}
	runtime := newRuntimeWithExecutors(t, []QueryDefinition{ReferenceQueryDefinition()}, nil, executor, nil, nil)
	unauthorized := referenceExecution("other.permission", "read", "tenant-1")
	_, forbiddenErr := runtime.ExecuteQuery(unauthorized, QueryRequest{QueryID: ReferenceQueryID, Version: ReferenceVersion})
	_, missingErr := runtime.ExecuteQuery(unauthorized, QueryRequest{QueryID: "platform.missing.list", Version: ReferenceVersion})
	if !errors.Is(forbiddenErr, ErrObjectUnavailable) || !errors.Is(missingErr, ErrObjectUnavailable) || forbiddenErr.Error() != missingErr.Error() {
		t.Fatalf("forbidden=%v missing=%v, want indistinguishable unavailable errors", forbiddenErr, missingErr)
	}
	if executor.calls != 0 {
		t.Fatalf("query executor calls = %d, want 0", executor.calls)
	}
}

func TestRuntimeEnforcesCostAndTimeoutWithoutLeakingExecutorDetails(t *testing.T) {
	definition := ReferenceQueryDefinition()
	definition.Cost.BaseCost = 1
	definition.Cost.PerRowCost = 2
	definition.Cost.Limit = 10
	runtime := newRuntimeWithExecutors(t, []QueryDefinition{definition}, nil, &querySpy{}, nil, nil)
	execution := referenceExecution("admin:reference-records:read", "read", "tenant-1")
	if _, err := runtime.ExecuteQuery(execution, QueryRequest{
		QueryID: ReferenceQueryID, Version: ReferenceVersion, Pagination: Pagination{Page: 1, PageSize: 5},
	}); !errors.Is(err, ErrCostLimitExceeded) {
		t.Fatalf("ExecuteQuery(expensive) error = %v, want ErrCostLimitExceeded", err)
	}

	definition = ReferenceQueryDefinition()
	definition.Cost.MaxOffset = 50
	runtime = newRuntimeWithExecutors(t, []QueryDefinition{definition}, nil, &querySpy{}, nil, nil)
	if _, err := runtime.ExecuteQuery(execution, QueryRequest{
		QueryID: ReferenceQueryID, Version: ReferenceVersion, Pagination: Pagination{Page: 7, PageSize: 10},
	}); !errors.Is(err, ErrCostLimitExceeded) {
		t.Fatalf("ExecuteQuery(deep offset) error = %v, want ErrCostLimitExceeded", err)
	}

	definition = ReferenceQueryDefinition()
	definition.Timeout = time.Millisecond
	runtime = newRuntimeWithExecutors(t, []QueryDefinition{definition}, nil, queryExecutorFunc(func(ctx context.Context, _ QueryPlan) (QueryResult, error) {
		<-ctx.Done()
		return QueryResult{}, fmt.Errorf("physical table secret_records: %w", ctx.Err())
	}), nil, nil)
	_, err := runtime.ExecuteQuery(execution, QueryRequest{QueryID: ReferenceQueryID, Version: ReferenceVersion})
	if !errors.Is(err, ErrExecutionFailed) || strings.Contains(err.Error(), "secret_records") {
		t.Fatalf("ExecuteQuery(timeout) error = %v, want redacted ErrExecutionFailed", err)
	}
}

func TestRuntimeProjectsDeclaredResultSchemaAndScopesCountsByTenant(t *testing.T) {
	definition := ReferenceQueryDefinition()
	definition.ExposeTotal = true
	runtime, _ := newReferenceRuntime(t, &definition)
	result, err := runtime.ExecuteQuery(referenceExecution("admin:reference-records:read", "read", "tenant-1"), QueryRequest{
		QueryID: ReferenceQueryID, Version: ReferenceVersion, Pagination: Pagination{Page: 1, PageSize: 10},
	})
	if err != nil {
		t.Fatalf("ExecuteQuery() error = %v", err)
	}
	if result.Total == nil || *result.Total != 2 {
		t.Fatalf("ExecuteQuery() total = %v, want tenant-scoped 2", result.Total)
	}

	projecting := newRuntimeWithExecutors(t, []QueryDefinition{ReferenceQueryDefinition()}, nil, queryExecutorFunc(func(context.Context, QueryPlan) (QueryResult, error) {
		return QueryResult{Items: []map[string]any{{"id": int64(1), "code": "A", "name": "Alpha", "status": "enabled", "secretTable": "platform_secret"}}}, nil
	}), nil, nil)
	projected, err := projecting.ExecuteQuery(referenceExecution("admin:reference-records:read", "read", "tenant-1"), QueryRequest{QueryID: ReferenceQueryID, Version: ReferenceVersion})
	if err != nil {
		t.Fatalf("ExecuteQuery(projecting) error = %v", err)
	}
	if _, leaked := projected.Items[0]["secretTable"]; leaked {
		t.Fatalf("ExecuteQuery(projecting) leaked undeclared result: %+v", projected.Items[0])
	}
}

func TestGORMCommandIsTenantScopedAndIdempotent(t *testing.T) {
	counting := &countingCommandExecutor{}
	runtime, db := newReferenceRuntimeWithCommandWrapper(t, counting)
	execution := referenceExecution("admin:reference-records:update", "update", "tenant-1")
	request := CommandRequest{
		CommandID: ReferenceCommandID, Version: ReferenceVersion,
		Arguments: map[string]any{"code": "A-001", "name": "Renamed"}, IdempotencyKey: "rename-a-001",
	}
	first, err := runtime.ExecuteCommand(execution, request)
	if err != nil {
		t.Fatalf("ExecuteCommand() error = %v", err)
	}
	second, err := runtime.ExecuteCommand(execution, request)
	if err != nil || first.Values["affected"] != second.Values["affected"] {
		t.Fatalf("ExecuteCommand(replay) = %+v, %v, want cached %+v", second, err, first)
	}
	if counting.calls != 1 {
		t.Fatalf("command executor calls = %d, want 1", counting.calls)
	}

	request.Arguments = map[string]any{"code": "A-001", "name": "Different"}
	if _, err := runtime.ExecuteCommand(execution, request); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("ExecuteCommand(conflicting replay) error = %v, want ErrIdempotencyConflict", err)
	}
	var rows []referenceRecord
	if err := db.Where("code = ?", "A-001").Order("tenant_id").Find(&rows).Error; err != nil {
		t.Fatalf("load renamed records: %v", err)
	}
	if len(rows) != 2 || rows[0].TenantID != "tenant-1" || rows[0].Name != "Renamed" || rows[1].TenantID != "tenant-2" || rows[1].Name != "Tenant Two A" {
		t.Fatalf("renamed rows = %+v, want only tenant 1 changed", rows)
	}
}

func newReferenceRuntime(t *testing.T, override *QueryDefinition) (*Runtime, *gorm.DB) {
	return newReferenceRuntimeWithCommandWrapper(t, nil, override)
}

func newReferenceRuntimeWithCommandWrapper(t *testing.T, wrapper *countingCommandExecutor, overrides ...*QueryDefinition) (*Runtime, *gorm.DB) {
	t.Helper()
	db, err := storage.OpenGORM(storage.Config{Driver: "sqlite", DSN: filepath.Join(t.TempDir(), "service-objects.db")})
	if err != nil {
		t.Fatalf("OpenGORM() error = %v", err)
	}
	if err := db.AutoMigrate(&referenceRecord{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	rows := []referenceRecord{
		{TenantID: "tenant-1", Code: "A-001", Name: "Alpha", Status: "enabled"},
		{TenantID: "tenant-1", Code: "B-001", Name: "Beta", Status: "disabled"},
		{TenantID: "tenant-2", Code: "A-001", Name: "Tenant Two A", Status: "enabled"},
		{TenantID: "tenant-2", Code: "C-001", Name: "Tenant Two C", Status: "enabled"},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed reference records: %v", err)
	}
	executor, err := NewGORMExecutor(db, map[string]GORMResourceBinding{"reference-records": ReferenceGORMBinding(referenceTable)})
	if err != nil {
		t.Fatalf("NewGORMExecutor() error = %v", err)
	}
	queryDefinition := ReferenceQueryDefinition()
	if len(overrides) > 0 && overrides[0] != nil {
		queryDefinition = *overrides[0]
	}
	var commandExecutor CommandExecutor = executor
	if wrapper != nil {
		wrapper.next = executor
		commandExecutor = wrapper
	}
	runtime := newRuntimeWithExecutors(t, []QueryDefinition{queryDefinition}, []CommandDefinition{ReferenceCommandDefinition()}, executor, commandExecutor, NewMemoryIdempotencyStore())
	return runtime, db
}

func newRuntimeWithExecutors(t *testing.T, queries []QueryDefinition, commands []CommandDefinition, queryExecutor QueryExecutor, commandExecutor CommandExecutor, idempotency IdempotencyStore) *Runtime {
	t.Helper()
	registry, err := NewRegistry(queries, commands)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	authorizer := AuthorizerFunc(func(_ context.Context, execution kernel.ExecutionContext, permission string, action string) bool {
		return execution.PermissionIntent.Code == permission && execution.PermissionIntent.Action == action
	})
	runtime, err := NewRuntime(registry, authorizer, queryExecutor, commandExecutor, idempotency)
	if err != nil {
		t.Fatalf("NewRuntime() error = %v", err)
	}
	return runtime
}

func referenceExecution(permission string, action string, tenantID string) Invocation {
	return Invocation{Execution: kernel.ExecutionContext{
		Context: context.Background(), Actor: kernel.Actor{ID: 1, Username: "admin", Kind: kernel.ActorKindUser},
		TenantScope:      kernel.TenantScope{TenantID: 1},
		PermissionIntent: kernel.PermissionIntent{Code: permission, Action: action},
	}, TenantID: tenantID, Scope: ScopeConstraint{All: true}}
}

type querySpy struct{ calls int }

func (s *querySpy) ExecuteQuery(context.Context, QueryPlan) (QueryResult, error) {
	s.calls++
	return QueryResult{}, nil
}

type queryExecutorFunc func(context.Context, QueryPlan) (QueryResult, error)

func (f queryExecutorFunc) ExecuteQuery(ctx context.Context, plan QueryPlan) (QueryResult, error) {
	return f(ctx, plan)
}

type countingCommandExecutor struct {
	mu    sync.Mutex
	calls int
	next  CommandExecutor
}

func (e *countingCommandExecutor) ExecuteCommand(ctx context.Context, plan CommandPlan) (CommandResult, error) {
	e.mu.Lock()
	e.calls++
	e.mu.Unlock()
	return e.next.ExecuteCommand(ctx, plan)
}
