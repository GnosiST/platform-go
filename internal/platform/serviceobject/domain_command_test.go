package serviceobject

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/GnosiST/platform-go/internal/platform/kernel"
)

const (
	referenceDomainCommandID = "platform.identity.role-assignments.replace"
	referenceDomainResource  = "role-assignments"
)

func TestRegistrySharesVersionKeysAcrossQueryGenericAndDomainCommands(t *testing.T) {
	query := ReferenceQueryDefinition()
	command := ReferenceCommandDefinition()
	command.ID = query.ID
	domain := referenceDomainCommandDefinition()
	domain.ID = query.ID

	for name, test := range map[string]struct {
		queries  []QueryDefinition
		commands []CommandDefinition
		domains  []DomainCommandDefinition
	}{
		"query and generic": {[]QueryDefinition{query}, []CommandDefinition{command}, nil},
		"query and domain":  {[]QueryDefinition{query}, nil, []DomainCommandDefinition{domain}},
		"generic and domain": {
			nil, []CommandDefinition{command}, []DomainCommandDefinition{domain},
		},
		"duplicate domain": {nil, nil, []DomainCommandDefinition{domain, domain}},
	} {
		if _, err := NewRegistryWithDomainCommands(test.queries, test.commands, test.domains); !errors.Is(err, ErrDefinitionConflict) {
			t.Fatalf("NewRegistryWithDomainCommands(%s) error = %v, want ErrDefinitionConflict", name, err)
		}
	}

	if _, err := NewRegistry([]QueryDefinition{ReferenceQueryDefinition()}, []CommandDefinition{ReferenceCommandDefinition()}); err != nil {
		t.Fatalf("NewRegistry() compatibility error = %v", err)
	}
}

func TestRegistryRequiresBoundedIdempotentDomainDefinitions(t *testing.T) {
	for name, mutate := range map[string]func(*DomainCommandDefinition){
		"idempotency": func(definition *DomainCommandDefinition) { definition.Idempotency = IdempotencyNone },
		"zero bound":  func(definition *DomainCommandDefinition) { definition.MaxAffectedRows = 0 },
		"large bound": func(definition *DomainCommandDefinition) { definition.MaxAffectedRows = maximumDomainCommandItems + 1 },
		"unbounded set": func(definition *DomainCommandDefinition) {
			definition.Arguments[0].MaxLength = 0
		},
	} {
		definition := referenceDomainCommandDefinition()
		mutate(&definition)
		if _, err := NewRegistryWithDomainCommands(nil, nil, []DomainCommandDefinition{definition}); !errors.Is(err, ErrDefinitionInvalid) {
			t.Fatalf("NewRegistryWithDomainCommands(%s) error = %v, want ErrDefinitionInvalid", name, err)
		}
	}
}

func TestRuntimeNormalizesAndDispatchesDomainCommandWithSharedGuards(t *testing.T) {
	executor := &domainCommandSpy{result: CommandResult{Values: map[string]any{"affected": int64(3), "internal": "hidden"}}}
	runtime := newDomainRuntime(t, referenceDomainCommandDefinition(), executor, NewMemoryIdempotencyStore())
	invocation := referenceDomainInvocation()
	request := CommandRequest{
		CommandID: referenceDomainCommandID, Version: ReferenceVersion, IdempotencyKey: "replace-role-assignments",
		Arguments: map[string]any{
			"roleIds": []any{" role-b ", "role-a", "role-b"},
			"remediations": []any{
				map[string]any{"userCode": " user-b ", "roleCode": "role-b", "action": "remove-role"},
				map[string]any{"userCode": "user-a", "roleCode": "role-c", "action": "replace-role", "replacementRoleCode": "role-d"},
				map[string]any{"userCode": "user-b", "roleCode": "role-b", "action": "remove-role"},
			},
		},
	}

	first, err := runtime.ExecuteCommand(invocation, request)
	if err != nil {
		t.Fatalf("ExecuteCommand(domain) error = %v", err)
	}
	if !reflect.DeepEqual(first.Values, map[string]any{"affected": int64(3)}) {
		t.Fatalf("ExecuteCommand(domain) values = %#v, want projected affected", first.Values)
	}
	request.Arguments["roleIds"] = []string{"role-a", "role-b"}
	request.Arguments["remediations"] = []RoleRemediation{
		{UserCode: "user-a", RoleCode: "role-c", Action: RoleRemediationReplace, ReplacementRoleCode: "role-d"},
		{UserCode: "user-b", RoleCode: "role-b", Action: RoleRemediationRemove},
	}
	second, err := runtime.ExecuteCommand(invocation, request)
	if err != nil || !reflect.DeepEqual(second, first) {
		t.Fatalf("ExecuteCommand(normalized replay) = %#v, %v, want %#v", second, err, first)
	}
	if executor.calls != 1 {
		t.Fatalf("domain executor calls = %d, want 1", executor.calls)
	}
	wantRoleIDs := []string{"role-a", "role-b"}
	wantRemediations := []RoleRemediation{
		{UserCode: "user-a", RoleCode: "role-c", Action: RoleRemediationReplace, ReplacementRoleCode: "role-d"},
		{UserCode: "user-b", RoleCode: "role-b", Action: RoleRemediationRemove},
	}
	if !reflect.DeepEqual(executor.plan.Arguments["roleIds"], wantRoleIDs) || !reflect.DeepEqual(executor.plan.Arguments["remediations"], wantRemediations) {
		t.Fatalf("normalized arguments = %#v, want roleIds=%#v remediations=%#v", executor.plan.Arguments, wantRoleIDs, wantRemediations)
	}
	if executor.plan.TenantID != "tenant-1" || !reflect.DeepEqual(executor.plan.Scope.OrgCodes, []string{"org-a"}) {
		t.Fatalf("trusted plan scope = tenant %q scope %#v", executor.plan.TenantID, executor.plan.Scope)
	}
}

func TestRuntimeRejectsInvalidOrUnguardedDomainCommandsBeforeDispatch(t *testing.T) {
	executor := &domainCommandSpy{}
	runtime := newDomainRuntime(t, referenceDomainCommandDefinition(), executor, NewMemoryIdempotencyStore())
	valid := CommandRequest{CommandID: referenceDomainCommandID, Version: ReferenceVersion, IdempotencyKey: "valid-key", Arguments: map[string]any{"roleIds": []string{"role-a"}}}

	requests := []CommandRequest{
		{CommandID: referenceDomainCommandID, Version: ReferenceVersion, Arguments: valid.Arguments},
		{CommandID: referenceDomainCommandID, Version: ReferenceVersion, IdempotencyKey: "closed", Arguments: map[string]any{
			"roleIds":      []string{"role-a"},
			"remediations": []any{map[string]any{"userCode": "user-a", "roleCode": "role-a", "action": "remove-role", "unexpected": true}},
		}},
		{CommandID: referenceDomainCommandID, Version: ReferenceVersion, IdempotencyKey: "wrong-type", Arguments: map[string]any{"roleIds": []any{"role-a", 17}}},
		{CommandID: referenceDomainCommandID, Version: ReferenceVersion, IdempotencyKey: "remove-replacement", Arguments: map[string]any{
			"roleIds": []string{"role-a"},
			"remediations": []any{map[string]any{
				"userCode": "user-a", "roleCode": "role-a", "action": "remove-role", "replacementRoleCode": "role-b",
			}},
		}},
		{CommandID: referenceDomainCommandID, Version: ReferenceVersion, IdempotencyKey: "replace-missing", Arguments: map[string]any{
			"roleIds":      []string{"role-a"},
			"remediations": []any{map[string]any{"userCode": "user-a", "roleCode": "role-a", "action": "replace-role"}},
		}},
		{CommandID: referenceDomainCommandID, Version: ReferenceVersion, IdempotencyKey: "conflict", Arguments: map[string]any{
			"roleIds": []string{"role-a"},
			"remediations": []any{
				map[string]any{"userCode": "user-a", "roleCode": "role-a", "action": "remove-role"},
				map[string]any{"userCode": "user-a", "roleCode": "role-a", "action": "replace-role", "replacementRoleCode": "role-b"},
			},
		}},
	}
	tooMany := make([]string, maximumDomainCommandItems+1)
	for index := range tooMany {
		tooMany[index] = fmt.Sprintf("role-%04d", index)
	}
	requests = append(requests, CommandRequest{CommandID: referenceDomainCommandID, Version: ReferenceVersion, IdempotencyKey: "too-many", Arguments: map[string]any{"roleIds": tooMany}})

	for _, request := range requests {
		if _, err := runtime.ExecuteCommand(referenceDomainInvocation(), request); !errors.Is(err, ErrRequestInvalid) {
			t.Fatalf("ExecuteCommand(%s) error = %v, want ErrRequestInvalid", request.IdempotencyKey, err)
		}
	}
	unauthorized := referenceDomainInvocation()
	unauthorized.Execution.PermissionIntent.Code = "admin:other:update"
	if _, err := runtime.ExecuteCommand(unauthorized, valid); !errors.Is(err, ErrObjectUnavailable) {
		t.Fatalf("ExecuteCommand(unauthorized) error = %v, want ErrObjectUnavailable", err)
	}
	wrongTenant := referenceDomainInvocation()
	wrongTenant.TenantID = ""
	if _, err := runtime.ExecuteCommand(wrongTenant, valid); !errors.Is(err, ErrObjectUnavailable) {
		t.Fatalf("ExecuteCommand(missing tenant) error = %v, want ErrObjectUnavailable", err)
	}
	if executor.calls != 0 {
		t.Fatalf("domain executor calls = %d, want 0", executor.calls)
	}
}

func TestRuntimeEnforcesDomainCostAndTimeout(t *testing.T) {
	definition := referenceDomainCommandDefinition()
	definition.Cost.Limit--
	runtime := newDomainRuntime(t, definition, &domainCommandSpy{}, NewMemoryIdempotencyStore())
	request := CommandRequest{CommandID: referenceDomainCommandID, Version: ReferenceVersion, IdempotencyKey: "cost", Arguments: map[string]any{"roleIds": []string{"role-a"}}}
	if _, err := runtime.ExecuteCommand(referenceDomainInvocation(), request); !errors.Is(err, ErrCostLimitExceeded) {
		t.Fatalf("ExecuteCommand(cost) error = %v, want ErrCostLimitExceeded", err)
	}

	definition = referenceDomainCommandDefinition()
	definition.Timeout = time.Millisecond
	timeoutExecutor := &domainCommandSpy{execute: func(ctx context.Context, _ DomainCommandPlan) (CommandResult, error) {
		<-ctx.Done()
		return CommandResult{}, fmt.Errorf("domain relation table leaked: %w", ctx.Err())
	}}
	runtime = newDomainRuntime(t, definition, timeoutExecutor, NewMemoryIdempotencyStore())
	request.IdempotencyKey = "timeout"
	_, err := runtime.ExecuteCommand(referenceDomainInvocation(), request)
	if !errors.Is(err, ErrExecutionFailed) || strings.Contains(err.Error(), "relation table") {
		t.Fatalf("ExecuteCommand(timeout) error = %v, want redacted ErrExecutionFailed", err)
	}
}

func referenceDomainCommandDefinition() DomainCommandDefinition {
	return DomainCommandDefinition{
		ID: referenceDomainCommandID, Version: ReferenceVersion, Resource: referenceDomainResource,
		Permission: "admin:role-assignments:update", Action: "update",
		TenantMode: TenantRequired, DataScope: "tenant",
		Arguments: []ArgumentDefinition{
			{Name: "roleIds", Type: ValueStringSet, Required: true, MaxLength: 64},
			{Name: "remediations", Type: ValueRoleRemediations, MaxLength: 64},
		},
		Cost: CostPolicy{BaseCost: 1, PerRowCost: 1, Limit: maximumDomainCommandItems + 1}, Timeout: 2 * time.Second,
		Idempotency: IdempotencyRequiredKey, MaxAffectedRows: maximumDomainCommandItems,
		ResultSchema: []ResultField{{Name: "affected", Type: ValueInteger}},
	}
}

func newDomainRuntime(t *testing.T, definition DomainCommandDefinition, executor DomainCommandExecutor, idempotency IdempotencyStore) *Runtime {
	t.Helper()
	registry, err := NewRegistryWithDomainCommands(nil, nil, []DomainCommandDefinition{definition})
	if err != nil {
		t.Fatalf("NewRegistryWithDomainCommands() error = %v", err)
	}
	authorizer := AuthorizerFunc(func(_ context.Context, execution kernel.ExecutionContext, permission string, action string) bool {
		return execution.PermissionIntent.Code == permission && execution.PermissionIntent.Action == action
	})
	runtime, err := NewRuntimeWithDomainCommands(registry, authorizer, nil, nil, executor, idempotency)
	if err != nil {
		t.Fatalf("NewRuntimeWithDomainCommands() error = %v", err)
	}
	return runtime
}

func referenceDomainInvocation() Invocation {
	invocation := referenceExecution("admin:role-assignments:update", "update", "tenant-1")
	invocation.Scope = ScopeConstraint{OrgCodes: []string{"org-a"}}
	return invocation
}

type domainCommandSpy struct {
	calls   int
	plan    DomainCommandPlan
	result  CommandResult
	execute func(context.Context, DomainCommandPlan) (CommandResult, error)
}

func (s *domainCommandSpy) ExecuteDomainCommand(ctx context.Context, plan DomainCommandPlan) (CommandResult, error) {
	s.calls++
	s.plan = plan
	if s.execute != nil {
		return s.execute(ctx, plan)
	}
	return s.result, nil
}
