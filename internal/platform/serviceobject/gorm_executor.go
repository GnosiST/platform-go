package serviceobject

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var sqlIdentifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type GORMResourceBinding struct {
	Table            string
	TenantColumn     string
	OrgColumn        string
	AreaColumn       string
	OwnerColumns     []string
	PredicateColumns map[string]string
	ValueColumns     map[string]string
	ResultColumns    map[string]string
}

type GORMExecutor struct {
	db        *gorm.DB
	resources map[string]GORMResourceBinding
}

func NewGORMExecutor(db *gorm.DB, resources map[string]GORMResourceBinding) (*GORMExecutor, error) {
	if db == nil || len(resources) == 0 {
		return nil, fmt.Errorf("%w: gorm database and resource bindings are required", ErrDefinitionInvalid)
	}
	bindings := make(map[string]GORMResourceBinding, len(resources))
	for resource, binding := range resources {
		if !resourcePattern.MatchString(resource) || !safeSQLIdentifier(binding.Table) || !safeOptionalSQLIdentifier(binding.TenantColumn) || !safeOptionalSQLIdentifier(binding.OrgColumn) || !safeOptionalSQLIdentifier(binding.AreaColumn) || !safeSQLIdentifiers(binding.OwnerColumns) {
			return nil, fmt.Errorf("%w: gorm resource binding is invalid", ErrDefinitionInvalid)
		}
		if !safeColumnMap(binding.PredicateColumns) || !safeColumnMap(binding.ValueColumns) || !safeColumnMap(binding.ResultColumns) {
			return nil, fmt.Errorf("%w: gorm resource column binding is invalid", ErrDefinitionInvalid)
		}
		bindings[resource] = cloneBinding(binding)
	}
	return &GORMExecutor{db: db, resources: bindings}, nil
}

func (e *GORMExecutor) ExecuteQuery(ctx context.Context, plan QueryPlan) (QueryResult, error) {
	binding, ok := e.resources[plan.Definition.Resource]
	if !ok || plan.AST.Resource != plan.Definition.Resource {
		return QueryResult{}, ErrExecutionFailed
	}
	query := e.db.WithContext(ctx).Table(binding.Table)
	query, err := applyTenant(query, binding, plan.Definition.TenantMode, plan.TenantID)
	if err != nil {
		return QueryResult{}, err
	}
	query, err = applyScope(query, binding, plan.Scope)
	if err != nil {
		return QueryResult{}, err
	}
	query, err = applyPredicates(query, binding.PredicateColumns, plan.AST.Predicates)
	if err != nil {
		return QueryResult{}, err
	}
	var total *int64
	if plan.Definition.ExposeTotal {
		count := int64(0)
		if err := query.Session(&gorm.Session{}).Count(&count).Error; err != nil {
			return QueryResult{}, err
		}
		total = &count
	}
	selects, err := resultSelects(plan.Definition.ResultSchema, binding.ResultColumns)
	if err != nil {
		return QueryResult{}, err
	}
	query = query.Select(selects)
	for _, sorter := range plan.Sort {
		column, exists := binding.PredicateColumns[sorter.Field]
		if !exists {
			return QueryResult{}, ErrRequestInvalid
		}
		query = query.Order(clause.OrderByColumn{Column: clause.Column{Name: column}, Desc: sorter.Order == "desc"})
	}
	if len(plan.Sort) == 0 {
		if column, exists := binding.PredicateColumns["id"]; exists {
			query = query.Order(clause.OrderByColumn{Column: clause.Column{Name: column}})
		}
	}
	items := make([]map[string]any, 0, plan.PageSize)
	if err := query.Offset((plan.Page - 1) * plan.PageSize).Limit(plan.PageSize).Scan(&items).Error; err != nil {
		return QueryResult{}, err
	}
	return QueryResult{Items: items, Total: total, Page: plan.Page, PageSize: plan.PageSize}, nil
}

func (e *GORMExecutor) ExecuteCommand(ctx context.Context, plan CommandPlan) (CommandResult, error) {
	binding, ok := e.resources[plan.Definition.Resource]
	if !ok || plan.AST.Resource != plan.Definition.Resource {
		return CommandResult{}, ErrExecutionFailed
	}
	values, err := mappedValues(plan.AST.Values, binding.ValueColumns)
	if err != nil {
		return CommandResult{}, err
	}
	if plan.Definition.TenantMode == TenantRequired {
		if binding.TenantColumn == "" || strings.TrimSpace(plan.TenantID) == "" {
			return CommandResult{}, ErrRequestInvalid
		}
		values[binding.TenantColumn] = plan.TenantID
	}
	query := e.db.WithContext(ctx).Table(binding.Table)
	switch plan.AST.Kind {
	case MutationInsert:
		if err := validateInsertScope(values, binding, plan.Scope); err != nil {
			return CommandResult{}, err
		}
		result := query.Create(values)
		if result.Error != nil {
			return CommandResult{}, result.Error
		}
		if result.RowsAffected > plan.Definition.MaxAffectedRows {
			return CommandResult{}, ErrCostLimitExceeded
		}
		return CommandResult{Values: map[string]any{"affected": result.RowsAffected}}, nil
	case MutationUpdate:
		if err := validateUpdatedScopeValues(values, binding, plan.Scope); err != nil {
			return CommandResult{}, err
		}
		var affected int64
		err = e.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			scoped := tx.Table(binding.Table)
			scoped, err = applyTenant(scoped, binding, plan.Definition.TenantMode, plan.TenantID)
			if err != nil {
				return err
			}
			scoped, err = applyScope(scoped, binding, plan.Scope)
			if err != nil {
				return err
			}
			scoped, err = applyPredicates(scoped, binding.PredicateColumns, plan.AST.Predicates)
			if err != nil {
				return err
			}
			result := scoped.Updates(values)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected > plan.Definition.MaxAffectedRows {
				return ErrCostLimitExceeded
			}
			affected = result.RowsAffected
			return nil
		})
		if err != nil {
			return CommandResult{}, err
		}
		return CommandResult{Values: map[string]any{"affected": affected}}, nil
	default:
		return CommandResult{}, ErrRequestInvalid
	}
}

func applyScope(query *gorm.DB, binding GORMResourceBinding, scope ScopeConstraint) (*gorm.DB, error) {
	expression, err := scopeExpression(binding, scope)
	if err != nil {
		return nil, err
	}
	if expression == nil {
		return query, nil
	}
	return query.Where(expression), nil
}

func scopeExpression(binding GORMResourceBinding, scope ScopeConstraint) (clause.Expression, error) {
	if scope.All {
		return nil, nil
	}
	orgArea := make([]clause.Expression, 0, 2)
	if len(scope.OrgCodes) > 0 {
		if binding.OrgColumn == "" {
			return nil, ErrObjectUnavailable
		}
		orgArea = append(orgArea, stringSetExpression(binding.OrgColumn, scope.OrgCodes))
	}
	if len(scope.AreaCodes) > 0 {
		if binding.AreaColumn == "" {
			return nil, ErrObjectUnavailable
		}
		orgArea = append(orgArea, stringSetExpression(binding.AreaColumn, scope.AreaCodes))
	}
	self := make([]clause.Expression, 0, len(binding.OwnerColumns))
	if scope.Self {
		if len(binding.OwnerColumns) == 0 || len(scope.ActorIdentifiers) == 0 {
			return nil, ErrObjectUnavailable
		}
		for _, column := range binding.OwnerColumns {
			self = append(self, stringSetExpression(column, scope.ActorIdentifiers))
		}
	}
	if len(orgArea) == 0 && len(self) == 0 {
		return nil, ErrObjectUnavailable
	}
	if len(orgArea) == 0 {
		return clause.Or(self...), nil
	}
	return clause.And(orgArea...), nil
}

func stringSetExpression(column string, values []string) clause.Expression {
	set := make([]any, 0, len(values))
	for _, value := range values {
		set = append(set, value)
	}
	return clause.IN{Column: clause.Column{Name: column}, Values: set}
}

func validateInsertScope(values map[string]any, binding GORMResourceBinding, scope ScopeConstraint) error {
	if scope.All {
		return nil
	}
	orgAreaConfigured := false
	orgAreaAllowed := true
	if len(scope.OrgCodes) > 0 {
		if binding.OrgColumn == "" {
			return ErrObjectUnavailable
		}
		orgAreaConfigured = true
		orgAreaAllowed = orgAreaAllowed && valueInScope(values[binding.OrgColumn], scope.OrgCodes)
	}
	if len(scope.AreaCodes) > 0 {
		if binding.AreaColumn == "" {
			return ErrObjectUnavailable
		}
		orgAreaConfigured = true
		orgAreaAllowed = orgAreaAllowed && valueInScope(values[binding.AreaColumn], scope.AreaCodes)
	}
	if scope.Self && !orgAreaConfigured {
		if len(binding.OwnerColumns) == 0 || len(scope.ActorIdentifiers) == 0 {
			return ErrObjectUnavailable
		}
		for _, column := range binding.OwnerColumns {
			if valueInScope(values[column], scope.ActorIdentifiers) {
				return nil
			}
		}
	}
	if orgAreaConfigured && orgAreaAllowed {
		return nil
	}
	return ErrObjectUnavailable
}

func validateUpdatedScopeValues(values map[string]any, binding GORMResourceBinding, scope ScopeConstraint) error {
	if scope.All {
		return nil
	}
	orgAreaConfigured := len(scope.OrgCodes) > 0 || len(scope.AreaCodes) > 0
	if value, changed := values[binding.OrgColumn]; changed && len(scope.OrgCodes) > 0 && !valueInScope(value, scope.OrgCodes) {
		return ErrObjectUnavailable
	}
	if value, changed := values[binding.AreaColumn]; changed && len(scope.AreaCodes) > 0 && !valueInScope(value, scope.AreaCodes) {
		return ErrObjectUnavailable
	}
	if scope.Self && !orgAreaConfigured {
		for _, column := range binding.OwnerColumns {
			if value, changed := values[column]; changed && !valueInScope(value, scope.ActorIdentifiers) {
				return ErrObjectUnavailable
			}
		}
	}
	return nil
}

func valueInScope(value any, allowed []string) bool {
	text, ok := value.(string)
	if !ok {
		return false
	}
	for _, candidate := range allowed {
		if strings.EqualFold(strings.TrimSpace(text), strings.TrimSpace(candidate)) {
			return true
		}
	}
	return false
}

func applyTenant(query *gorm.DB, binding GORMResourceBinding, mode TenantMode, tenantID string) (*gorm.DB, error) {
	switch mode {
	case TenantRequired:
		if binding.TenantColumn == "" || strings.TrimSpace(tenantID) == "" {
			return nil, ErrRequestInvalid
		}
		return query.Where(clause.Eq{Column: clause.Column{Name: binding.TenantColumn}, Value: tenantID}), nil
	case TenantPlatform:
		return query, nil
	default:
		return nil, ErrRequestInvalid
	}
}

func applyPredicates(query *gorm.DB, columns map[string]string, predicates []Predicate) (*gorm.DB, error) {
	for _, predicate := range predicates {
		column, ok := columns[predicate.Field]
		if !ok {
			return nil, ErrRequestInvalid
		}
		switch predicate.Operator {
		case PredicateEqual:
			query = query.Where(clause.Eq{Column: clause.Column{Name: column}, Value: predicate.Value})
		case PredicatePrefix:
			value, ok := predicate.Value.(string)
			if !ok {
				return nil, ErrRequestInvalid
			}
			query = query.Where(clause.Like{Column: clause.Column{Name: column}, Value: value + "%"})
		case PredicateLessThan:
			query = query.Where(clause.Lt{Column: clause.Column{Name: column}, Value: predicate.Value})
		case PredicateGreaterThan:
			query = query.Where(clause.Gt{Column: clause.Column{Name: column}, Value: predicate.Value})
		default:
			return nil, ErrRequestInvalid
		}
	}
	return query, nil
}

func resultSelects(schema []ResultField, columns map[string]string) ([]string, error) {
	selects := make([]string, 0, len(schema))
	for _, field := range schema {
		column, ok := columns[field.Name]
		if !ok {
			return nil, ErrDefinitionInvalid
		}
		selects = append(selects, column+" AS "+field.Name)
	}
	return selects, nil
}

func mappedValues(values map[string]any, columns map[string]string) (map[string]any, error) {
	mapped := make(map[string]any, len(values))
	for field, value := range values {
		column, ok := columns[field]
		if !ok {
			return nil, ErrRequestInvalid
		}
		mapped[column] = value
	}
	return mapped, nil
}

func safeColumnMap(columns map[string]string) bool {
	for logical, physical := range columns {
		if !logicalNamePattern.MatchString(logical) || !safeSQLIdentifier(physical) {
			return false
		}
	}
	return true
}

func safeSQLIdentifier(identifier string) bool {
	return sqlIdentifierPattern.MatchString(strings.TrimSpace(identifier))
}

func safeOptionalSQLIdentifier(identifier string) bool {
	return strings.TrimSpace(identifier) == "" || safeSQLIdentifier(identifier)
}

func safeSQLIdentifiers(identifiers []string) bool {
	for _, identifier := range identifiers {
		if !safeSQLIdentifier(identifier) {
			return false
		}
	}
	return true
}

func cloneBinding(binding GORMResourceBinding) GORMResourceBinding {
	return GORMResourceBinding{
		Table: binding.Table, TenantColumn: binding.TenantColumn,
		OrgColumn: binding.OrgColumn, AreaColumn: binding.AreaColumn,
		OwnerColumns:     append([]string(nil), binding.OwnerColumns...),
		PredicateColumns: cloneStringMap(binding.PredicateColumns),
		ValueColumns:     cloneStringMap(binding.ValueColumns),
		ResultColumns:    cloneStringMap(binding.ResultColumns),
	}
}

func cloneStringMap(input map[string]string) map[string]string {
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make(map[string]string, len(input))
	for _, key := range keys {
		result[key] = input[key]
	}
	return result
}
