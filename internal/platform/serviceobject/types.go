package serviceobject

import (
	"context"
	"time"

	"platform-go/internal/platform/kernel"
)

type ValueType string

const (
	ValueString  ValueType = "string"
	ValueInteger ValueType = "integer"
	ValueBoolean ValueType = "boolean"
)

type TenantMode string

const (
	TenantRequired TenantMode = "required"
	TenantPlatform TenantMode = "platform"
)

type IdempotencyMode string

const (
	IdempotencyNone        IdempotencyMode = "none"
	IdempotencyRequiredKey IdempotencyMode = "required-key"
)

type ArgumentDefinition struct {
	Name      string
	Type      ValueType
	Required  bool
	MaxLength int
	Minimum   *int64
	Maximum   *int64
}

type ResultField struct {
	Name string
	Type ValueType
}

type SortDefinition struct {
	Name  string
	Field string
}

type CostPolicy struct {
	BaseCost      int
	PerRowCost    int
	PerOffsetCost int
	PredicateCost int
	SortCost      int
	TotalCost     int
	MaxOffset     int
	Limit         int
}

type QueryDefinition struct {
	ID           string
	Version      string
	Resource     string
	Permission   string
	Action       string
	TenantMode   TenantMode
	DataScope    string
	Arguments    []ArgumentDefinition
	AllowedSort  []SortDefinition
	Cost         CostPolicy
	Timeout      time.Duration
	MaxPageSize  int
	ExposeTotal  bool
	ResultSchema []ResultField
	Build        QueryBuilder
}

type CommandDefinition struct {
	ID              string
	Version         string
	Resource        string
	Permission      string
	Action          string
	TenantMode      TenantMode
	DataScope       string
	Arguments       []ArgumentDefinition
	Cost            CostPolicy
	Timeout         time.Duration
	Idempotency     IdempotencyMode
	MaxAffectedRows int64
	ResultSchema    []ResultField
	Build           CommandBuilder
}

type QueryRequest struct {
	QueryID    string         `json:"queryId"`
	Version    string         `json:"version"`
	Arguments  map[string]any `json:"arguments,omitempty"`
	Pagination Pagination     `json:"pagination,omitempty"`
	Sort       []SortInput    `json:"sort,omitempty"`
}

type CommandRequest struct {
	CommandID      string         `json:"commandId"`
	Version        string         `json:"version"`
	Arguments      map[string]any `json:"arguments,omitempty"`
	IdempotencyKey string         `json:"idempotencyKey,omitempty"`
}

type Pagination struct {
	Page     int `json:"page,omitempty"`
	PageSize int `json:"pageSize,omitempty"`
}

type SortInput struct {
	Name  string `json:"name"`
	Order string `json:"order"`
}

type ValidatedArguments map[string]any

type QueryBuilder func(ValidatedArguments) (QueryAST, error)

type CommandBuilder func(ValidatedArguments) (CommandAST, error)

type PredicateOperator string

const (
	PredicateEqual       PredicateOperator = "equal"
	PredicatePrefix      PredicateOperator = "prefix"
	PredicateLessThan    PredicateOperator = "less-than"
	PredicateGreaterThan PredicateOperator = "greater-than"
)

type Predicate struct {
	Field    string
	Operator PredicateOperator
	Value    any
}

type QueryAST struct {
	Resource   string
	Predicates []Predicate
}

type MutationKind string

const (
	MutationInsert MutationKind = "insert"
	MutationUpdate MutationKind = "update"
)

type CommandAST struct {
	Resource   string
	Kind       MutationKind
	Predicates []Predicate
	Values     map[string]any
}

type QueryPlan struct {
	Definition QueryDefinition
	AST        QueryAST
	TenantID   string
	Scope      ScopeConstraint
	Page       int
	PageSize   int
	Sort       []ResolvedSort
}

type CommandPlan struct {
	Definition CommandDefinition
	AST        CommandAST
	TenantID   string
	Scope      ScopeConstraint
}

type Invocation struct {
	Execution kernel.ExecutionContext
	TenantID  string
	Scope     ScopeConstraint
}

type ScopeConstraint struct {
	All              bool
	OrgCodes         []string
	AreaCodes        []string
	Self             bool
	ActorIdentifiers []string
}

type ResolvedSort struct {
	Field string
	Order string
}

type QueryResult struct {
	Items    []map[string]any `json:"items"`
	Total    *int64           `json:"total,omitempty"`
	Page     int              `json:"page"`
	PageSize int              `json:"pageSize"`
}

type CommandResult struct {
	Values map[string]any `json:"values"`
}

type Authorizer interface {
	Can(context.Context, kernel.ExecutionContext, string, string) bool
}

type AuthorizerFunc func(context.Context, kernel.ExecutionContext, string, string) bool

func (f AuthorizerFunc) Can(ctx context.Context, execution kernel.ExecutionContext, permission string, action string) bool {
	return f != nil && f(ctx, execution, permission, action)
}

type QueryExecutor interface {
	ExecuteQuery(context.Context, QueryPlan) (QueryResult, error)
}

type CommandExecutor interface {
	ExecuteCommand(context.Context, CommandPlan) (CommandResult, error)
}

type IdempotencyStore interface {
	Execute(context.Context, IdempotencyScope, string, func(context.Context) (CommandResult, error)) (CommandResult, error)
}

type IdempotencyScope struct {
	CommandID string
	Version   string
	Actor     string
	TenantID  string
	Key       string
}
