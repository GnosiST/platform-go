package federatedreport

import (
	"context"
	"regexp"
	"strings"

	"gorm.io/gorm"
)

var gormIdentifierPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)

type GORMSourceBinding struct {
	ID                string
	DatasourceGroupID string
	Table             string
	Fields            []string
	DB                *gorm.DB
}

type gormSourceExecutor struct{ binding GORMSourceBinding }

func NewGORMSourceExecutors(bindings []GORMSourceBinding) (map[string]SourceExecutor, error) {
	executors := make(map[string]SourceExecutor, len(bindings))
	for _, binding := range bindings {
		if !validIdentifier(binding.ID) || !validIdentifier(binding.DatasourceGroupID) || !validGORMIdentifier(binding.Table) || binding.DB == nil || len(binding.Fields) == 0 {
			return nil, ErrInvalidDefinition
		}
		fields := make([]string, 0, len(binding.Fields))
		seen := map[string]struct{}{}
		for _, field := range binding.Fields {
			if !validGORMIdentifier(field) || field == "tenant_code" {
				return nil, ErrInvalidDefinition
			}
			if _, found := seen[field]; found {
				return nil, ErrInvalidDefinition
			}
			seen[field] = struct{}{}
			fields = append(fields, field)
		}
		if _, found := executors[binding.ID]; found {
			return nil, ErrInvalidDefinition
		}
		binding.Fields = fields
		executors[binding.ID] = gormSourceExecutor{binding: binding}
	}
	return executors, nil
}

func validGORMIdentifier(value string) bool {
	return value == strings.TrimSpace(value) && gormIdentifierPattern.MatchString(value)
}

func (e gormSourceExecutor) Execute(ctx context.Context, request SourceRequest) ([]map[string]any, error) {
	if ctx == nil || request.DatasourceGroupID != e.binding.DatasourceGroupID || !validIdentifier(request.TenantCode) || request.MaximumRows < 1 {
		return nil, ErrExecutionFailed
	}
	rows := []map[string]any{}
	if err := e.binding.DB.WithContext(ctx).Table(e.binding.Table).Select(strings.Join(e.binding.Fields, ", ")).Where("tenant_code = ?", request.TenantCode).Limit(request.MaximumRows).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
