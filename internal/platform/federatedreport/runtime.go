package federatedreport

import (
	"context"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/GnosiST/platform-go/internal/platform/kernel"
)

var (
	ErrInvalidDefinition = errors.New("invalid federated report definition")
	ErrInvalidRequest    = errors.New("invalid federated report request")
	ErrExecutionFailed   = errors.New("federated report execution failed")
	ErrCostExceeded      = errors.New("federated report cost exceeded")
)

var identifierPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)

type Definition struct {
	ID            string        `json:"id"`
	Version       uint64        `json:"version"`
	Sources       []Source      `json:"sources"`
	AllowedFields []string      `json:"allowedFields"`
	MaximumRows   int           `json:"maximumRows"`
	MaximumCost   int           `json:"maximumCost"`
	Timeout       time.Duration `json:"timeout"`
}

type Source struct {
	ID                string `json:"id"`
	DatasourceGroupID string `json:"datasourceGroupId"`
}

type Request struct {
	ReportID string `json:"reportId"`
	Version  uint64 `json:"version"`
}

type SourceRequest struct {
	ReportID          string
	Version           uint64
	TenantCode        string
	DatasourceGroupID string
	MaximumRows       int
}

type SourceExecutor interface {
	Execute(context.Context, SourceRequest) ([]map[string]any, error)
}

type SourceExecutorFunc func(context.Context, SourceRequest) ([]map[string]any, error)

func (f SourceExecutorFunc) Execute(ctx context.Context, request SourceRequest) ([]map[string]any, error) {
	return f(ctx, request)
}

type Result struct {
	Rows []map[string]any `json:"rows"`
}

type Runtime struct {
	definitions map[string]Definition
	executors   map[string]SourceExecutor
}

func NewRuntime(definitions []Definition, executors map[string]SourceExecutor) (*Runtime, error) {
	runtime := &Runtime{definitions: make(map[string]Definition, len(definitions)), executors: make(map[string]SourceExecutor, len(executors))}
	for id, executor := range executors {
		if !validIdentifier(id) || executor == nil {
			return nil, ErrInvalidDefinition
		}
		runtime.executors[id] = executor
	}
	for _, definition := range definitions {
		if err := validateDefinition(definition, runtime.executors); err != nil {
			return nil, err
		}
		key := definitionKey(definition.ID, definition.Version)
		if _, found := runtime.definitions[key]; found {
			return nil, ErrInvalidDefinition
		}
		runtime.definitions[key] = cloneDefinition(definition)
	}
	return runtime, nil
}

func (r *Runtime) Execute(execution kernel.ExecutionContext, request Request) (Result, error) {
	if r == nil || execution.ValidatePermissioned() != nil || execution.TenantScope.PlatformWide || !validIdentifier(execution.TenantScope.TenantCode) || !validIdentifier(request.ReportID) || request.Version == 0 {
		return Result{}, ErrInvalidRequest
	}
	definition, found := r.definitions[definitionKey(request.ReportID, request.Version)]
	if !found {
		return Result{}, ErrInvalidRequest
	}
	ctx, cancel := context.WithTimeout(execution.BaseContext(), definition.Timeout)
	defer cancel()
	result := Result{Rows: make([]map[string]any, 0, definition.MaximumRows)}
	cost := 0
	for _, source := range definition.Sources {
		executor := r.executors[source.ID]
		rows, err := executor.Execute(ctx, SourceRequest{ReportID: definition.ID, Version: definition.Version, TenantCode: execution.TenantScope.TenantCode, DatasourceGroupID: source.DatasourceGroupID, MaximumRows: definition.MaximumRows - len(result.Rows)})
		if err != nil {
			return Result{}, ErrExecutionFailed
		}
		if len(rows)+len(result.Rows) > definition.MaximumRows {
			return Result{}, ErrCostExceeded
		}
		for _, row := range rows {
			projected := make(map[string]any, len(definition.AllowedFields))
			for _, field := range definition.AllowedFields {
				if value, found := row[field]; found {
					projected[field] = value
				}
			}
			cost += len(projected)
			if cost > definition.MaximumCost {
				return Result{}, ErrCostExceeded
			}
			result.Rows = append(result.Rows, projected)
		}
	}
	return result, nil
}

func validateDefinition(definition Definition, executors map[string]SourceExecutor) error {
	if !validIdentifier(definition.ID) || definition.Version == 0 || len(definition.Sources) == 0 || len(definition.AllowedFields) == 0 || definition.MaximumRows < 1 || definition.MaximumRows > 10000 || definition.MaximumCost < 1 || definition.Timeout <= 0 || definition.Timeout > time.Minute {
		return ErrInvalidDefinition
	}
	seenSources := map[string]struct{}{}
	for _, source := range definition.Sources {
		if !validIdentifier(source.ID) || !validIdentifier(source.DatasourceGroupID) || executors[source.ID] == nil {
			return ErrInvalidDefinition
		}
		if _, found := seenSources[source.ID]; found {
			return ErrInvalidDefinition
		}
		seenSources[source.ID] = struct{}{}
	}
	seenFields := map[string]struct{}{}
	for _, field := range definition.AllowedFields {
		if !validIdentifier(field) {
			return ErrInvalidDefinition
		}
		if _, found := seenFields[field]; found {
			return ErrInvalidDefinition
		}
		seenFields[field] = struct{}{}
	}
	return nil
}

func cloneDefinition(definition Definition) Definition {
	definition.Sources = append([]Source(nil), definition.Sources...)
	definition.AllowedFields = append([]string(nil), definition.AllowedFields...)
	return definition
}

func definitionKey(id string, version uint64) string {
	return id + "@" + strconv.FormatUint(version, 10)
}

func validIdentifier(value string) bool {
	return value == strings.TrimSpace(value) && identifierPattern.MatchString(value)
}
