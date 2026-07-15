package serviceobject

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
)

var (
	objectIDPattern              = regexp.MustCompile(`^[a-z][a-z0-9.-]*$`)
	versionPattern               = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)
	resourcePattern              = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	logicalNamePattern           = regexp.MustCompile(`^[a-z][A-Za-z0-9]*$`)
	permissionRequirementPattern = regexp.MustCompile(`^[A-Za-z0-9*][A-Za-z0-9._:*/-]*$`)
)

const maximumQueryOffset = 10000

var forbiddenClientNames = []string{"dsn", "datasource", "database", "schema", "shard", "field", "operator", "sql", "join"}
var forbiddenPhysicalPrefixes = []string{"dsn", "datasource", "database", "schema", "shard", "field", "operator", "sql", "join"}

type Registry struct {
	queries        map[string]QueryDefinition
	commands       map[string]CommandDefinition
	domainCommands map[string]DomainCommandDefinition
}

func NewRegistry(queries []QueryDefinition, commands []CommandDefinition) (*Registry, error) {
	return NewRegistryWithDomainCommands(queries, commands, nil)
}

func NewRegistryWithDomainCommands(queries []QueryDefinition, commands []CommandDefinition, domainCommands []DomainCommandDefinition) (*Registry, error) {
	registry := &Registry{
		queries:        make(map[string]QueryDefinition, len(queries)),
		commands:       make(map[string]CommandDefinition, len(commands)),
		domainCommands: make(map[string]DomainCommandDefinition, len(domainCommands)),
	}
	registered := make(map[string]string, len(queries)+len(commands)+len(domainCommands))
	for _, definition := range queries {
		if err := validateQueryDefinition(definition); err != nil {
			return nil, err
		}
		key := definitionKey(definition.ID, definition.Version)
		if kind, exists := registered[key]; exists {
			return nil, definitionConflict("query", definition.ID, definition.Version, kind)
		}
		registered[key] = "query"
		registry.queries[key] = cloneQueryDefinition(definition)
	}
	for _, definition := range commands {
		if err := validateCommandDefinition(definition); err != nil {
			return nil, err
		}
		key := definitionKey(definition.ID, definition.Version)
		if kind, exists := registered[key]; exists {
			return nil, definitionConflict("command", definition.ID, definition.Version, kind)
		}
		registered[key] = "command"
		registry.commands[key] = cloneCommandDefinition(definition)
	}
	for _, definition := range domainCommands {
		if err := validateDomainCommandDefinition(definition); err != nil {
			return nil, err
		}
		key := definitionKey(definition.ID, definition.Version)
		if kind, exists := registered[key]; exists {
			return nil, definitionConflict("domain command", definition.ID, definition.Version, kind)
		}
		registered[key] = "domain command"
		registry.domainCommands[key] = cloneDomainCommandDefinition(definition)
	}
	return registry, nil
}

func definitionConflict(kind string, id string, version string, existingKind string) error {
	return fmt.Errorf("%w: %s %s@%s conflicts with %s", ErrDefinitionConflict, kind, id, version, existingKind)
}

func (r *Registry) query(id string, version string) (QueryDefinition, bool) {
	if r == nil {
		return QueryDefinition{}, false
	}
	definition, ok := r.queries[definitionKey(id, version)]
	return cloneQueryDefinition(definition), ok
}

func (r *Registry) command(id string, version string) (CommandDefinition, bool) {
	if r == nil {
		return CommandDefinition{}, false
	}
	definition, ok := r.commands[definitionKey(id, version)]
	return cloneCommandDefinition(definition), ok
}

func (r *Registry) domainCommand(id string, version string) (DomainCommandDefinition, bool) {
	if r == nil {
		return DomainCommandDefinition{}, false
	}
	definition, ok := r.domainCommands[definitionKey(id, version)]
	return cloneDomainCommandDefinition(definition), ok
}

func definitionKey(id string, version string) string {
	return strings.TrimSpace(id) + "@" + strings.TrimSpace(version)
}

func validateQueryDefinition(definition QueryDefinition) error {
	if err := validateDefinitionBase(definition.ID, definition.Version, definition.Resource, definition.Permission, definition.Action, definition.AdditionalPermissions, definition.TenantMode, definition.DataScope, definition.Arguments, definition.Cost, definition.Timeout, definition.ResultSchema); err != nil {
		return fmt.Errorf("%w: query %s: %v", ErrDefinitionInvalid, definition.ID, err)
	}
	if definition.Build == nil {
		return fmt.Errorf("%w: query %s: builder is required", ErrDefinitionInvalid, definition.ID)
	}
	if definition.MaxPageSize < 1 || definition.MaxPageSize > 1000 {
		return fmt.Errorf("%w: query %s: max page size must be between 1 and 1000", ErrDefinitionInvalid, definition.ID)
	}
	if definition.Cost.MaxOffset < 0 || definition.Cost.MaxOffset > maximumQueryOffset {
		return fmt.Errorf("%w: query %s: max offset must be between 0 and %d", ErrDefinitionInvalid, definition.ID, maximumQueryOffset)
	}
	if definition.Cost.PerRowCost < 1 || definition.Cost.PredicateCost < 1 || definition.Cost.MaxOffset > 0 && definition.Cost.PerOffsetCost < 1 || len(definition.AllowedSort) > 0 && definition.Cost.SortCost < 1 || definition.ExposeTotal && definition.Cost.TotalCost < 1 {
		return fmt.Errorf("%w: query %s: enabled query work must have a positive cost", ErrDefinitionInvalid, definition.ID)
	}
	seen := map[string]struct{}{}
	for _, sorter := range definition.AllowedSort {
		if !logicalNamePattern.MatchString(sorter.Name) || forbiddenName(sorter.Name) || !logicalNamePattern.MatchString(sorter.Field) {
			return fmt.Errorf("%w: query %s: sort declaration is invalid", ErrDefinitionInvalid, definition.ID)
		}
		if _, exists := seen[sorter.Name]; exists {
			return fmt.Errorf("%w: query %s: sort %s is duplicated", ErrDefinitionInvalid, definition.ID, sorter.Name)
		}
		seen[sorter.Name] = struct{}{}
	}
	return nil
}

func validateCommandDefinition(definition CommandDefinition) error {
	if err := validateDefinitionBase(definition.ID, definition.Version, definition.Resource, definition.Permission, definition.Action, definition.AdditionalPermissions, definition.TenantMode, definition.DataScope, definition.Arguments, definition.Cost, definition.Timeout, definition.ResultSchema); err != nil {
		return fmt.Errorf("%w: command %s: %v", ErrDefinitionInvalid, definition.ID, err)
	}
	if definition.Build == nil {
		return fmt.Errorf("%w: command %s: builder is required", ErrDefinitionInvalid, definition.ID)
	}
	if !slices.Contains([]IdempotencyMode{IdempotencyNone, IdempotencyRequiredKey}, definition.Idempotency) {
		return fmt.Errorf("%w: command %s: idempotency mode is invalid", ErrDefinitionInvalid, definition.ID)
	}
	if definition.MaxAffectedRows < 1 || definition.MaxAffectedRows > 1000 {
		return fmt.Errorf("%w: command %s: max affected rows must be between 1 and 1000", ErrDefinitionInvalid, definition.ID)
	}
	if definition.Cost.PerRowCost < 1 || definition.Cost.PredicateCost < 1 {
		return fmt.Errorf("%w: command %s: row and predicate costs must be positive", ErrDefinitionInvalid, definition.ID)
	}
	return nil
}

func validateDomainCommandDefinition(definition DomainCommandDefinition) error {
	if err := validateDomainDefinitionBase(definition); err != nil {
		return fmt.Errorf("%w: domain command %s: %v", ErrDefinitionInvalid, definition.ID, err)
	}
	if definition.Idempotency != IdempotencyRequiredKey {
		return fmt.Errorf("%w: domain command %s: idempotency must be required-key", ErrDefinitionInvalid, definition.ID)
	}
	if definition.MaxAffectedRows < 1 || definition.MaxAffectedRows > maximumDomainCommandItems {
		return fmt.Errorf("%w: domain command %s: max affected rows must be between 1 and %d", ErrDefinitionInvalid, definition.ID, maximumDomainCommandItems)
	}
	if definition.Cost.PerRowCost < 1 {
		return fmt.Errorf("%w: domain command %s: row cost must be positive", ErrDefinitionInvalid, definition.ID)
	}
	return nil
}

func validateDomainDefinitionBase(definition DomainCommandDefinition) error {
	scalarArguments := make([]ArgumentDefinition, 0, len(definition.Arguments))
	for _, argument := range definition.Arguments {
		switch argument.Type {
		case ValueStringSet, ValueRoleRemediations:
			if !logicalNamePattern.MatchString(argument.Name) || forbiddenName(argument.Name) {
				return fmt.Errorf("argument %q is invalid or reserved", argument.Name)
			}
			if argument.MaxLength <= 0 {
				return fmt.Errorf("collection argument %q requires item max length", argument.Name)
			}
			scalarArguments = append(scalarArguments, ArgumentDefinition{Name: argument.Name, Type: ValueString, Required: argument.Required, MaxLength: argument.MaxLength})
		default:
			scalarArguments = append(scalarArguments, argument)
		}
	}
	return validateDefinitionBase(definition.ID, definition.Version, definition.Resource, definition.Permission, definition.Action, definition.AdditionalPermissions, definition.TenantMode, definition.DataScope, scalarArguments, definition.Cost, definition.Timeout, definition.ResultSchema)
}

func validateDefinitionBase(id string, version string, resource string, permission string, action string, additionalPermissions []PermissionRequirement, tenantMode TenantMode, dataScope string, arguments []ArgumentDefinition, cost CostPolicy, timeout time.Duration, result []ResultField) error {
	if !objectIDPattern.MatchString(id) || strings.TrimSpace(id) != id {
		return fmt.Errorf("id must be a stable lowercase identifier")
	}
	if !versionPattern.MatchString(version) {
		return fmt.Errorf("version must use numeric semver")
	}
	if !resourcePattern.MatchString(resource) {
		return fmt.Errorf("resource must be a logical identifier")
	}
	if strings.TrimSpace(permission) == "" || strings.TrimSpace(action) == "" {
		return fmt.Errorf("permission and action are required")
	}
	if err := validateAdditionalPermissions(permission, action, additionalPermissions); err != nil {
		return err
	}
	if tenantMode != TenantRequired && tenantMode != TenantPlatform {
		return fmt.Errorf("tenant mode is invalid")
	}
	if dataScope != "tenant" && dataScope != "platform" {
		return fmt.Errorf("data scope is invalid")
	}
	if tenantMode == TenantRequired && dataScope != "tenant" || tenantMode == TenantPlatform && dataScope != "platform" {
		return fmt.Errorf("tenant mode and data scope do not agree")
	}
	if cost.BaseCost < 0 || cost.PerRowCost < 0 || cost.PerOffsetCost < 0 || cost.PredicateCost < 0 || cost.SortCost < 0 || cost.TotalCost < 0 || cost.Limit < 1 || cost.BaseCost > cost.Limit {
		return fmt.Errorf("cost policy is invalid")
	}
	if timeout <= 0 || timeout > time.Minute {
		return fmt.Errorf("timeout must be between zero and one minute")
	}
	seen := map[string]struct{}{}
	for _, argument := range arguments {
		if !logicalNamePattern.MatchString(argument.Name) || forbiddenName(argument.Name) {
			return fmt.Errorf("argument %q is invalid or reserved", argument.Name)
		}
		if _, exists := seen[argument.Name]; exists {
			return fmt.Errorf("argument %q is duplicated", argument.Name)
		}
		seen[argument.Name] = struct{}{}
		if !slices.Contains([]ValueType{ValueString, ValueInteger, ValueBoolean, ValueMenuDefinition}, argument.Type) {
			return fmt.Errorf("argument %q type is invalid", argument.Name)
		}
		if argument.Type == ValueString && argument.MaxLength <= 0 {
			return fmt.Errorf("string argument %q requires max length", argument.Name)
		}
	}
	seen = map[string]struct{}{}
	for _, field := range result {
		if !logicalNamePattern.MatchString(field.Name) || forbiddenName(field.Name) {
			return fmt.Errorf("result field %q is invalid or reserved", field.Name)
		}
		if _, exists := seen[field.Name]; exists {
			return fmt.Errorf("result field %q is duplicated", field.Name)
		}
		seen[field.Name] = struct{}{}
		if !slices.Contains([]ValueType{ValueString, ValueInteger, ValueBoolean, ValueStringSet, ValueMenuDefinition}, field.Type) {
			return fmt.Errorf("result field %q type is invalid", field.Name)
		}
	}
	return nil
}

func validateAdditionalPermissions(primaryPermission string, primaryAction string, requirements []PermissionRequirement) error {
	seen := make(map[string]struct{}, len(requirements))
	primary := primaryPermission + "\x00" + primaryAction
	for _, requirement := range requirements {
		if requirement.Permission != strings.TrimSpace(requirement.Permission) || requirement.Action != strings.TrimSpace(requirement.Action) ||
			len(requirement.Permission) > 191 || len(requirement.Action) > 191 || !permissionRequirementPattern.MatchString(requirement.Permission) || !permissionRequirementPattern.MatchString(requirement.Action) {
			return fmt.Errorf("additional permission requirement is malformed")
		}
		key := requirement.Permission + "\x00" + requirement.Action
		if key == primary {
			return fmt.Errorf("additional permission requirement duplicates the primary requirement")
		}
		if _, duplicate := seen[key]; duplicate {
			return fmt.Errorf("additional permission requirement is duplicated")
		}
		seen[key] = struct{}{}
	}
	return nil
}

func forbiddenName(name string) bool {
	normalized := strings.ToLower(strings.TrimSpace(name))
	if slices.Contains(forbiddenClientNames, normalized) {
		return true
	}
	return slices.ContainsFunc(forbiddenPhysicalPrefixes, func(prefix string) bool {
		return strings.HasPrefix(normalized, prefix)
	})
}

func cloneQueryDefinition(definition QueryDefinition) QueryDefinition {
	definition.AdditionalPermissions = append([]PermissionRequirement(nil), definition.AdditionalPermissions...)
	definition.Arguments = append([]ArgumentDefinition(nil), definition.Arguments...)
	cloneArgumentBoundaries(definition.Arguments)
	definition.AllowedSort = append([]SortDefinition(nil), definition.AllowedSort...)
	definition.ResultSchema = append([]ResultField(nil), definition.ResultSchema...)
	return definition
}

func cloneCommandDefinition(definition CommandDefinition) CommandDefinition {
	definition.AdditionalPermissions = append([]PermissionRequirement(nil), definition.AdditionalPermissions...)
	definition.Arguments = append([]ArgumentDefinition(nil), definition.Arguments...)
	cloneArgumentBoundaries(definition.Arguments)
	definition.ResultSchema = append([]ResultField(nil), definition.ResultSchema...)
	return definition
}

func cloneDomainCommandDefinition(definition DomainCommandDefinition) DomainCommandDefinition {
	definition.AdditionalPermissions = append([]PermissionRequirement(nil), definition.AdditionalPermissions...)
	definition.Arguments = append([]ArgumentDefinition(nil), definition.Arguments...)
	cloneArgumentBoundaries(definition.Arguments)
	definition.ResultSchema = append([]ResultField(nil), definition.ResultSchema...)
	return definition
}

func cloneArgumentBoundaries(arguments []ArgumentDefinition) {
	for index := range arguments {
		if arguments[index].Minimum != nil {
			minimum := *arguments[index].Minimum
			arguments[index].Minimum = &minimum
		}
		if arguments[index].Maximum != nil {
			maximum := *arguments[index].Maximum
			arguments[index].Maximum = &maximum
		}
	}
}
