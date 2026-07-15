package serviceobject

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"platform-go/internal/platform/kernel"
)

type Runtime struct {
	registry        *Registry
	authorizer      Authorizer
	queryExecutor   QueryExecutor
	commandExecutor CommandExecutor
	domainExecutor  DomainCommandExecutor
	idempotency     IdempotencyStore
}

func NewRuntime(registry *Registry, authorizer Authorizer, queryExecutor QueryExecutor, commandExecutor CommandExecutor, idempotency IdempotencyStore) (*Runtime, error) {
	return NewRuntimeWithDomainCommands(registry, authorizer, queryExecutor, commandExecutor, nil, idempotency)
}

func NewRuntimeWithDomainCommands(registry *Registry, authorizer Authorizer, queryExecutor QueryExecutor, commandExecutor CommandExecutor, domainExecutor DomainCommandExecutor, idempotency IdempotencyStore) (*Runtime, error) {
	if registry == nil || authorizer == nil {
		return nil, fmt.Errorf("%w: registry and authorizer are required", ErrDefinitionInvalid)
	}
	return &Runtime{
		registry: registry, authorizer: authorizer, queryExecutor: queryExecutor,
		commandExecutor: commandExecutor, domainExecutor: domainExecutor, idempotency: idempotency,
	}, nil
}

func (r *Runtime) WithAuthorizer(authorizer Authorizer) *Runtime {
	if r == nil || authorizer == nil {
		return nil
	}
	clone := *r
	clone.authorizer = authorizer
	return &clone
}

func (r *Runtime) ExecuteQuery(invocation Invocation, request QueryRequest) (QueryResult, error) {
	execution := invocation.Execution
	definition, ok := r.registry.query(request.QueryID, request.Version)
	if !ok || !r.allowed(execution, definition.Permission, definition.Action) || !r.allowedAdditional(execution, definition.AdditionalPermissions) {
		return QueryResult{}, ErrObjectUnavailable
	}
	if r.queryExecutor == nil || !validExecutionScope(invocation, definition.TenantMode) {
		return QueryResult{}, ErrObjectUnavailable
	}
	arguments, err := validateArguments(definition.Arguments, request.Arguments)
	if err != nil {
		return QueryResult{}, err
	}
	page, pageSize, err := validatePagination(request.Pagination, definition.MaxPageSize)
	if err != nil {
		return QueryResult{}, err
	}
	sort, err := resolveSort(request.Sort, definition.AllowedSort)
	if err != nil {
		return QueryResult{}, err
	}
	ast, err := definition.Build(arguments)
	if err != nil || validateQueryAST(definition, ast) != nil {
		return QueryResult{}, ErrRequestInvalid
	}
	if queryCostExceeded(definition, page, pageSize, len(ast.Predicates), len(sort)) {
		return QueryResult{}, ErrCostLimitExceeded
	}
	ctx, cancel := context.WithTimeout(execution.BaseContext(), definition.Timeout)
	defer cancel()
	result, err := r.queryExecutor.ExecuteQuery(ctx, QueryPlan{
		Definition: definition, AST: ast, Execution: execution, TenantID: strings.TrimSpace(invocation.TenantID),
		Scope: cloneScopeConstraint(invocation.Scope), Page: page, PageSize: pageSize, Sort: sort,
	})
	if err != nil {
		if errors.Is(err, ErrObjectUnavailable) || errors.Is(err, ErrCostLimitExceeded) || errors.Is(err, ErrConflict) || errors.Is(err, ErrValidation) {
			return QueryResult{}, err
		}
		return QueryResult{}, ErrExecutionFailed
	}
	items, err := projectItems(result.Items, definition.ResultSchema)
	if err != nil {
		return QueryResult{}, ErrExecutionFailed
	}
	result.Items = items
	result.Page = page
	result.PageSize = pageSize
	if !definition.ExposeTotal {
		result.Total = nil
	}
	return result, nil
}

func (r *Runtime) ExecuteCommand(invocation Invocation, request CommandRequest) (CommandResult, error) {
	execution := invocation.Execution
	definition, ok := r.registry.command(request.CommandID, request.Version)
	if ok {
		return r.executeGenericCommand(invocation, request, definition)
	}
	domainDefinition, ok := r.registry.domainCommand(request.CommandID, request.Version)
	if !ok || !r.allowed(execution, domainDefinition.Permission, domainDefinition.Action) || !r.allowedAdditional(execution, domainDefinition.AdditionalPermissions) {
		return CommandResult{}, ErrObjectUnavailable
	}
	return r.executeDomainCommand(invocation, request, domainDefinition)
}

func (r *Runtime) executeGenericCommand(invocation Invocation, request CommandRequest, definition CommandDefinition) (CommandResult, error) {
	execution := invocation.Execution
	if !r.allowed(execution, definition.Permission, definition.Action) || !r.allowedAdditional(execution, definition.AdditionalPermissions) {
		return CommandResult{}, ErrObjectUnavailable
	}
	if r.commandExecutor == nil || !validExecutionScope(invocation, definition.TenantMode) {
		return CommandResult{}, ErrObjectUnavailable
	}
	arguments, err := validateArguments(definition.Arguments, request.Arguments)
	if err != nil {
		return CommandResult{}, err
	}
	ast, err := definition.Build(arguments)
	if err != nil || validateCommandAST(definition, ast) != nil {
		return CommandResult{}, ErrRequestInvalid
	}
	if commandCostExceeded(definition, len(ast.Predicates)) {
		return CommandResult{}, ErrCostLimitExceeded
	}
	ctx, cancel := context.WithTimeout(execution.BaseContext(), definition.Timeout)
	defer cancel()
	execute := func(callContext context.Context) (CommandResult, error) {
		result, executeErr := r.commandExecutor.ExecuteCommand(callContext, CommandPlan{
			Definition: definition, AST: ast, Execution: execution, TenantID: strings.TrimSpace(invocation.TenantID),
			Scope: cloneScopeConstraint(invocation.Scope),
		})
		if executeErr != nil {
			if errors.Is(executeErr, ErrObjectUnavailable) || errors.Is(executeErr, ErrCostLimitExceeded) || errors.Is(executeErr, ErrConflict) || errors.Is(executeErr, ErrValidation) {
				return CommandResult{}, executeErr
			}
			return CommandResult{}, ErrExecutionFailed
		}
		projected, projectErr := projectValues(result.Values, definition.ResultSchema)
		if projectErr != nil {
			return CommandResult{}, ErrExecutionFailed
		}
		return CommandResult{Values: projected}, nil
	}
	if definition.Idempotency == IdempotencyNone {
		return execute(ctx)
	}
	key := strings.TrimSpace(request.IdempotencyKey)
	if key == "" || len(key) > 128 || r.idempotency == nil {
		return CommandResult{}, ErrRequestInvalid
	}
	fingerprint, err := commandFingerprint(definition, arguments)
	if err != nil {
		return CommandResult{}, ErrRequestInvalid
	}
	actor := execution.Actor.Username
	if actor == "" {
		actor = strconv.FormatInt(execution.Actor.ID, 10)
	}
	return r.idempotency.Execute(ctx, IdempotencyScope{
		CommandID: definition.ID, Version: definition.Version, Actor: actor,
		TenantID: strings.TrimSpace(invocation.TenantID), Key: key,
	}, fingerprint, execute)
}

func (r *Runtime) executeDomainCommand(invocation Invocation, request CommandRequest, definition DomainCommandDefinition) (CommandResult, error) {
	if r.domainExecutor == nil || !validExecutionScope(invocation, definition.TenantMode) {
		return CommandResult{}, ErrObjectUnavailable
	}
	arguments, err := validateArguments(definition.Arguments, request.Arguments)
	if err != nil {
		return CommandResult{}, err
	}
	if domainCommandCostExceeded(definition) {
		return CommandResult{}, ErrCostLimitExceeded
	}
	execution := invocation.Execution
	ctx, cancel := context.WithTimeout(execution.BaseContext(), definition.Timeout)
	defer cancel()
	execute := func(callContext context.Context) (CommandResult, error) {
		result, executeErr := r.domainExecutor.ExecuteDomainCommand(callContext, DomainCommandPlan{
			Definition: definition, Arguments: cloneValidatedArguments(arguments),
			Execution: execution,
			TenantID:  strings.TrimSpace(invocation.TenantID), Scope: cloneScopeConstraint(invocation.Scope),
		})
		if executeErr != nil {
			if errors.Is(executeErr, ErrObjectUnavailable) || errors.Is(executeErr, ErrCostLimitExceeded) || errors.Is(executeErr, ErrConflict) || errors.Is(executeErr, ErrValidation) {
				return CommandResult{}, executeErr
			}
			return CommandResult{}, ErrExecutionFailed
		}
		projected, projectErr := projectValues(result.Values, definition.ResultSchema)
		if projectErr != nil {
			return CommandResult{}, ErrExecutionFailed
		}
		return CommandResult{Values: projected}, nil
	}
	key := strings.TrimSpace(request.IdempotencyKey)
	if key == "" || len(key) > 128 || r.idempotency == nil {
		return CommandResult{}, ErrRequestInvalid
	}
	fingerprint, err := objectFingerprint(definition.ID, definition.Version, arguments)
	if err != nil {
		return CommandResult{}, ErrRequestInvalid
	}
	actor := execution.Actor.Username
	if actor == "" {
		actor = strconv.FormatInt(execution.Actor.ID, 10)
	}
	return r.idempotency.Execute(ctx, IdempotencyScope{
		CommandID: definition.ID, Version: definition.Version, Actor: actor,
		TenantID: strings.TrimSpace(invocation.TenantID), Key: key,
	}, fingerprint, execute)
}

func queryCostExceeded(definition QueryDefinition, page int, pageSize int, predicates int, sorts int) bool {
	offset := int64(page-1) * int64(pageSize)
	if offset > int64(definition.Cost.MaxOffset) {
		return true
	}
	cost := int64(definition.Cost.BaseCost) +
		int64(definition.Cost.PerRowCost)*int64(pageSize) +
		int64(definition.Cost.PerOffsetCost)*offset +
		int64(definition.Cost.PredicateCost)*int64(predicates) +
		int64(definition.Cost.SortCost)*int64(sorts)
	if definition.ExposeTotal {
		cost += int64(definition.Cost.TotalCost)
	}
	return cost > int64(definition.Cost.Limit)
}

func commandCostExceeded(definition CommandDefinition, predicates int) bool {
	cost := int64(definition.Cost.BaseCost) +
		int64(definition.Cost.PerRowCost)*definition.MaxAffectedRows +
		int64(definition.Cost.PredicateCost)*int64(predicates)
	return cost > int64(definition.Cost.Limit)
}

func domainCommandCostExceeded(definition DomainCommandDefinition) bool {
	cost := int64(definition.Cost.BaseCost) + int64(definition.Cost.PerRowCost)*definition.MaxAffectedRows
	return cost > int64(definition.Cost.Limit)
}

func cloneScopeConstraint(scope ScopeConstraint) ScopeConstraint {
	return ScopeConstraint{
		All: scope.All, Self: scope.Self,
		OrgCodes:         append([]string(nil), scope.OrgCodes...),
		AreaCodes:        append([]string(nil), scope.AreaCodes...),
		ActorIdentifiers: append([]string(nil), scope.ActorIdentifiers...),
	}
}

func (r *Runtime) allowed(execution kernel.ExecutionContext, permission string, action string) bool {
	if err := execution.ValidatePermissioned(); err != nil {
		return false
	}
	return r.authorizer.Can(execution.BaseContext(), execution, permission, action)
}

func (r *Runtime) allowedAdditional(execution kernel.ExecutionContext, requirements []PermissionRequirement) bool {
	for _, requirement := range requirements {
		if !r.authorizer.Can(execution.BaseContext(), execution, requirement.Permission, requirement.Action) {
			return false
		}
	}
	return true
}

func validExecutionScope(invocation Invocation, mode TenantMode) bool {
	execution := invocation.Execution
	switch mode {
	case TenantRequired:
		return strings.TrimSpace(invocation.TenantID) != "" && !execution.TenantScope.PlatformWide
	case TenantPlatform:
		return execution.TenantScope.PlatformWide
	default:
		return false
	}
}

func validateArguments(definitions []ArgumentDefinition, supplied map[string]any) (ValidatedArguments, error) {
	known := make(map[string]ArgumentDefinition, len(definitions))
	for _, definition := range definitions {
		known[definition.Name] = definition
	}
	for name := range supplied {
		if _, exists := known[name]; !exists || forbiddenName(name) {
			return nil, ErrRequestInvalid
		}
	}
	validated := make(ValidatedArguments, len(supplied))
	for _, definition := range definitions {
		value, exists := supplied[definition.Name]
		if !exists || value == nil {
			if definition.Required {
				return nil, ErrRequestInvalid
			}
			continue
		}
		normalized, err := validateValue(definition, value)
		if err != nil {
			return nil, ErrRequestInvalid
		}
		validated[definition.Name] = normalized
	}
	return validated, nil
}

func validateValue(definition ArgumentDefinition, value any) (any, error) {
	switch definition.Type {
	case ValueString:
		text, ok := value.(string)
		if !ok || len(text) > definition.MaxLength {
			return nil, ErrRequestInvalid
		}
		return text, nil
	case ValueBoolean:
		boolean, ok := value.(bool)
		if !ok {
			return nil, ErrRequestInvalid
		}
		return boolean, nil
	case ValueInteger:
		integer, ok := integerValue(value)
		if !ok || definition.Minimum != nil && integer < *definition.Minimum || definition.Maximum != nil && integer > *definition.Maximum {
			return nil, ErrRequestInvalid
		}
		return integer, nil
	case ValueStringSet:
		return normalizeStringSet(value, definition.MaxLength)
	case ValueRoleRemediations:
		return normalizeRoleRemediations(value, definition.MaxLength)
	case ValueMenuDefinition:
		return normalizeMenuDefinition(value)
	default:
		return nil, ErrRequestInvalid
	}
}

func integerValue(value any) (int64, bool) {
	switch typed := value.(type) {
	case json.Number:
		integer, err := typed.Int64()
		return integer, err == nil
	case int:
		return int64(typed), true
	case int64:
		return typed, true
	case int32:
		return int64(typed), true
	case float64:
		if math.Trunc(typed) != typed || typed < math.MinInt64 || typed > math.MaxInt64 {
			return 0, false
		}
		return int64(typed), true
	default:
		return 0, false
	}
}

func validatePagination(input Pagination, maxPageSize int) (int, int, error) {
	page := input.Page
	if page == 0 {
		page = 1
	}
	pageSize := input.PageSize
	if pageSize == 0 {
		pageSize = min(20, maxPageSize)
	}
	if page < 1 || page > 100000 || pageSize < 1 || pageSize > maxPageSize {
		return 0, 0, ErrRequestInvalid
	}
	return page, pageSize, nil
}

func resolveSort(input []SortInput, allowed []SortDefinition) ([]ResolvedSort, error) {
	if len(input) > 3 {
		return nil, ErrRequestInvalid
	}
	known := make(map[string]string, len(allowed))
	for _, sorter := range allowed {
		known[sorter.Name] = sorter.Field
	}
	seen := map[string]struct{}{}
	resolved := make([]ResolvedSort, 0, len(input))
	for _, sorter := range input {
		field, ok := known[sorter.Name]
		order := strings.ToLower(strings.TrimSpace(sorter.Order))
		if !ok || order != "asc" && order != "desc" {
			return nil, ErrRequestInvalid
		}
		if _, exists := seen[sorter.Name]; exists {
			return nil, ErrRequestInvalid
		}
		seen[sorter.Name] = struct{}{}
		resolved = append(resolved, ResolvedSort{Field: field, Order: order})
	}
	return resolved, nil
}

func validateQueryAST(definition QueryDefinition, ast QueryAST) error {
	if ast.Resource != definition.Resource {
		return ErrRequestInvalid
	}
	return validatePredicates(ast.Predicates)
}

func validateCommandAST(definition CommandDefinition, ast CommandAST) error {
	if ast.Resource != definition.Resource || ast.Kind != MutationInsert && ast.Kind != MutationUpdate || len(ast.Values) == 0 || ast.Kind == MutationUpdate && len(ast.Predicates) == 0 {
		return ErrRequestInvalid
	}
	for field := range ast.Values {
		if !logicalNamePattern.MatchString(field) {
			return ErrRequestInvalid
		}
	}
	return validatePredicates(ast.Predicates)
}

func validatePredicates(predicates []Predicate) error {
	if len(predicates) > 20 {
		return ErrRequestInvalid
	}
	for _, predicate := range predicates {
		if !logicalNamePattern.MatchString(predicate.Field) {
			return ErrRequestInvalid
		}
		switch predicate.Operator {
		case PredicateEqual, PredicatePrefix, PredicateLessThan, PredicateGreaterThan:
		default:
			return ErrRequestInvalid
		}
	}
	return nil
}

func projectItems(items []map[string]any, schema []ResultField) ([]map[string]any, error) {
	projected := make([]map[string]any, 0, len(items))
	for _, item := range items {
		values, err := projectValues(item, schema)
		if err != nil {
			return nil, err
		}
		projected = append(projected, values)
	}
	return projected, nil
}

func projectValues(values map[string]any, schema []ResultField) (map[string]any, error) {
	projected := make(map[string]any, len(schema))
	for _, field := range schema {
		value, exists := values[field.Name]
		if !exists || value == nil {
			continue
		}
		if !resultValueMatches(field.Type, value) {
			return nil, ErrExecutionFailed
		}
		projected[field.Name] = value
	}
	return projected, nil
}

func resultValueMatches(valueType ValueType, value any) bool {
	switch valueType {
	case ValueString:
		_, ok := value.(string)
		return ok
	case ValueBoolean:
		_, ok := value.(bool)
		return ok
	case ValueInteger:
		_, ok := integerValue(value)
		return ok
	case ValueStringSet:
		values, ok := value.([]string)
		if !ok || len(values) > maximumDomainCommandItems {
			return false
		}
		seen := make(map[string]struct{}, len(values))
		for _, item := range values {
			if _, duplicate := seen[item]; duplicate {
				return false
			}
			seen[item] = struct{}{}
		}
		return true
	case ValueMenuDefinition:
		definition, ok := value.(MenuDefinition)
		return ok && validateMenuDefinition(definition) == nil
	default:
		return false
	}
}

func commandFingerprint(definition CommandDefinition, arguments ValidatedArguments) (string, error) {
	return objectFingerprint(definition.ID, definition.Version, arguments)
}

func objectFingerprint(id string, version string, arguments ValidatedArguments) (string, error) {
	payload, err := json.Marshal(struct {
		ID        string             `json:"id"`
		Version   string             `json:"version"`
		Arguments ValidatedArguments `json:"arguments"`
	}{ID: id, Version: version, Arguments: arguments})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}
