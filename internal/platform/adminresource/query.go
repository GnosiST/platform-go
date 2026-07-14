package adminresource

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"platform-go/internal/platform/capability"
	"platform-go/internal/platform/rbac"
)

const (
	defaultQueryPageSize = 10
	maxQueryPageSize     = 100
	maxQueryValueLength  = 256
)

type QueryInput struct {
	Keywords   []string         `json:"keywords,omitempty"`
	Conditions []QueryCondition `json:"conditions,omitempty"`
	Sort       []QuerySort      `json:"sort,omitempty"`
	Page       int              `json:"page,omitempty"`
	PageSize   int              `json:"pageSize,omitempty"`
}

type QueryCondition struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

type QuerySort struct {
	Field string `json:"field"`
	Order string `json:"order"`
}

type QueryResult struct {
	Resource string   `json:"resource"`
	Items    []Record `json:"items"`
	Total    int      `json:"total"`
	Page     int      `json:"page"`
	PageSize int      `json:"pageSize"`
}

type QueryValidationError struct {
	Field  string
	Reason string
}

func (e QueryValidationError) Error() string {
	if e.Field == "" {
		return e.Reason
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Reason)
}

func (e QueryValidationError) Is(target error) bool {
	return target == ErrInvalidRecord
}

func (s *Store) Query(resource string, input QueryInput) (QueryResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	items, ok := s.resources[resource]
	if !ok {
		return QueryResult{}, ErrUnknownResource
	}
	return s.queryRecordsLocked(resource, input, visibleRecords(resource, items))
}

func (s *Store) QueryForPrincipal(resource string, input QueryInput, principal rbac.Principal) (QueryResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	items, ok := s.resources[resource]
	if !ok {
		return QueryResult{}, ErrUnknownResource
	}
	return s.queryRecordsLocked(resource, input, s.filterRecordsForPrincipalLocked(resource, visibleRecords(resource, items), principal))
}

func (s *Store) queryRecordsLocked(resource string, input QueryInput, items []Record) (QueryResult, error) {
	schema, ok := s.schemas[resource]
	if !ok {
		return QueryResult{}, ErrUnknownResource
	}
	plan, err := buildQueryPlanWithProtection(schema, input, s.protection != nil)
	if err != nil {
		return QueryResult{}, err
	}

	matched := make([]Record, 0, len(items))
	for _, item := range items {
		matches, matchErr := s.queryRecordMatchesProtected(context.Background(), resource, item, plan)
		if matchErr != nil {
			return QueryResult{}, matchErr
		}
		if matches {
			matched = append(matched, cloneRecord(item))
		}
	}
	sortRecords(matched, plan)

	total := len(matched)
	start := (plan.page - 1) * plan.pageSize
	if start > total {
		start = total
	}
	end := start + plan.pageSize
	if end > total {
		end = total
	}

	projected := make([]Record, 0, end-start)
	for _, record := range matched[start:end] {
		item, projectErr := s.projectRecordLocked(resource, record, ProjectionResponse)
		if projectErr != nil {
			return QueryResult{}, projectErr
		}
		projected = append(projected, item)
	}

	return QueryResult{
		Resource: resource,
		Items:    projected,
		Total:    total,
		Page:     plan.page,
		PageSize: plan.pageSize,
	}, nil
}

type queryPlan struct {
	fields       map[string]FieldDefinition
	searchFields []FieldDefinition
	keywords     []string
	conditions   []normalizedCondition
	sort         []normalizedSort
	page         int
	pageSize     int
}

type normalizedCondition struct {
	field    FieldDefinition
	operator string
	value    string
}

type normalizedSort struct {
	field FieldDefinition
	order string
}

func buildQueryPlan(schema Schema, input QueryInput) (queryPlan, error) {
	return buildQueryPlanWithProtection(schema, input, false)
}

func buildQueryPlanWithProtection(schema Schema, input QueryInput, allowEncrypted bool) (queryPlan, error) {
	fields := queryableFields(schema)
	plan := queryPlan{
		fields:       fields,
		searchFields: queryableSearchFields(schema, fields),
		page:         input.Page,
		pageSize:     input.PageSize,
	}
	if plan.page < 1 {
		plan.page = 1
	}
	if plan.pageSize <= 0 {
		plan.pageSize = defaultQueryPageSize
	}
	if plan.pageSize > maxQueryPageSize {
		plan.pageSize = maxQueryPageSize
	}

	for _, keyword := range input.Keywords {
		keyword = strings.TrimSpace(keyword)
		if keyword == "" {
			continue
		}
		if len(keyword) > maxQueryValueLength {
			return queryPlan{}, QueryValidationError{Field: "keywords", Reason: "query keyword is too long"}
		}
		plan.keywords = append(plan.keywords, keyword)
	}

	for _, condition := range input.Conditions {
		normalized, err := normalizeConditionWithProtection(condition, fields, allowEncrypted)
		if err != nil {
			return queryPlan{}, err
		}
		if normalized.value == "" {
			continue
		}
		plan.conditions = append(plan.conditions, normalized)
	}

	for _, sorter := range input.Sort {
		normalized, err := normalizeSort(sorter, fields)
		if err != nil {
			return queryPlan{}, err
		}
		if normalized.field.Key != "" {
			plan.sort = append(plan.sort, normalized)
		}
	}
	return plan, nil
}

func queryableFields(schema Schema) map[string]FieldDefinition {
	fields := map[string]FieldDefinition{
		"id": {Key: "id", Label: text("ID", "ID"), Type: "text", Source: "record", Searchable: true, Filterable: true, Sortable: true, InTable: true, InDetail: true},
	}
	for _, field := range schema.Fields {
		fields[field.Key] = field
	}
	return fields
}

func queryableSearchFields(schema Schema, fields map[string]FieldDefinition) []FieldDefinition {
	searchKeys := schema.SearchFields
	if len(searchKeys) == 0 {
		for _, field := range fields {
			if field.Searchable {
				searchKeys = append(searchKeys, field.Key)
			}
		}
	}
	result := make([]FieldDefinition, 0, len(searchKeys)+1)
	result = append(result, fields["id"])
	for _, key := range searchKeys {
		field, ok := fields[key]
		if !ok || field.StorageMode == capability.FieldStorageEncrypted {
			continue
		}
		result = append(result, field)
	}
	return result
}

func normalizeCondition(condition QueryCondition, fields map[string]FieldDefinition) (normalizedCondition, error) {
	return normalizeConditionWithProtection(condition, fields, false)
}

func normalizeConditionWithProtection(condition QueryCondition, fields map[string]FieldDefinition, allowEncrypted bool) (normalizedCondition, error) {
	fieldKey := strings.TrimSpace(condition.Field)
	if fieldKey == "" {
		return normalizedCondition{}, QueryValidationError{Field: "field", Reason: "query field is required"}
	}
	field, ok := fields[fieldKey]
	if !ok {
		return normalizedCondition{}, QueryValidationError{Field: fieldKey, Reason: "query field is not declared by resource schema"}
	}
	if field.StorageMode == capability.FieldStorageEncrypted {
		if !allowEncrypted {
			return normalizedCondition{}, QueryValidationError{Field: fieldKey, Reason: "encrypted field query requires the data protection runtime"}
		}
		if field.Protection == nil || field.Protection.BlindIndexNamespace == "" {
			return normalizedCondition{}, QueryValidationError{Field: fieldKey, Reason: "encrypted field exact-match query is disabled"}
		}
		operator, supported := normalizeOperator(condition.Operator)
		if !supported || operator != "=" {
			return normalizedCondition{}, QueryValidationError{Field: fieldKey, Reason: "encrypted field supports only exact-match queries"}
		}
		value := condition.Value
		if len(value) > maxQueryValueLength {
			return normalizedCondition{}, QueryValidationError{Field: fieldKey, Reason: "query value is too long"}
		}
		return normalizedCondition{field: field, operator: operator, value: value}, nil
	}
	if !field.Filterable && !field.Searchable {
		return normalizedCondition{}, QueryValidationError{Field: fieldKey, Reason: "query field is not filterable"}
	}
	operator, ok := normalizeOperator(condition.Operator)
	if !ok {
		return normalizedCondition{}, QueryValidationError{Field: fieldKey, Reason: "query operator is not supported"}
	}
	value := strings.TrimSpace(condition.Value)
	if len(value) > maxQueryValueLength {
		return normalizedCondition{}, QueryValidationError{Field: fieldKey, Reason: "query value is too long"}
	}
	return normalizedCondition{field: field, operator: operator, value: value}, nil
}

func normalizeSort(sorter QuerySort, fields map[string]FieldDefinition) (normalizedSort, error) {
	fieldKey := strings.TrimSpace(sorter.Field)
	if fieldKey == "" {
		return normalizedSort{}, nil
	}
	field, ok := fields[fieldKey]
	if !ok {
		return normalizedSort{}, QueryValidationError{Field: fieldKey, Reason: "sort field is not declared by resource schema"}
	}
	if field.StorageMode == capability.FieldStorageEncrypted {
		return normalizedSort{}, QueryValidationError{Field: fieldKey, Reason: "encrypted field cannot be sorted"}
	}
	if !field.Sortable {
		return normalizedSort{}, QueryValidationError{Field: fieldKey, Reason: "sort field is not sortable"}
	}
	order := strings.ToLower(strings.TrimSpace(sorter.Order))
	switch order {
	case "", "asc", "ascend":
		order = "asc"
	case "desc", "descend":
		order = "desc"
	default:
		return normalizedSort{}, QueryValidationError{Field: fieldKey, Reason: "sort order is not supported"}
	}
	return normalizedSort{field: field, order: order}, nil
}

func normalizeOperator(operator string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(operator)) {
	case ":", "~", "contains", "like":
		return "contains", true
	case "=", "eq":
		return "=", true
	case "!=", "<>", "ne":
		return "!=", true
	case ">":
		return ">", true
	case ">=", "gte":
		return ">=", true
	case "<":
		return "<", true
	case "<=", "lte":
		return "<=", true
	default:
		return "", false
	}
}

func queryRecordMatches(record Record, plan queryPlan) bool {
	for _, condition := range plan.conditions {
		if !conditionMatches(record, condition) {
			return false
		}
	}
	if len(plan.keywords) == 0 {
		return true
	}
	haystackValues := make([]string, 0, len(plan.searchFields))
	for _, field := range plan.searchFields {
		haystackValues = append(haystackValues, queryFieldValues(record, field)...)
	}
	haystack := strings.ToLower(strings.Join(haystackValues, " "))
	for _, keyword := range plan.keywords {
		if !strings.Contains(haystack, strings.ToLower(keyword)) {
			return false
		}
	}
	return true
}

func (s *Store) queryRecordMatchesProtected(ctx context.Context, resource string, record Record, plan queryPlan) (bool, error) {
	for _, condition := range plan.conditions {
		if condition.field.StorageMode != capability.FieldStorageEncrypted {
			if !conditionMatches(record, condition) {
				return false, nil
			}
			continue
		}
		if s.protection == nil {
			return false, QueryValidationError{Field: condition.field.Key, Reason: "encrypted field query requires the data protection runtime"}
		}
		envelope, exists := record.Values[condition.field.Key]
		if !exists || envelope == "" {
			return false, nil
		}
		schema := s.schemas[resource]
		policy, fieldContext, err := protectedPolicyAndContext(schema, resource, record, condition.field)
		if err != nil {
			return false, err
		}
		matched, err := s.protection.MatchExact(ctx, envelope, condition.value, policy, fieldContext)
		if err != nil {
			return false, fmt.Errorf("%w: encrypted field exact-match query failed", ErrInvalidRecord)
		}
		if !matched {
			return false, nil
		}
	}
	if len(plan.keywords) == 0 {
		return true, nil
	}
	haystackValues := make([]string, 0, len(plan.searchFields))
	for _, field := range plan.searchFields {
		haystackValues = append(haystackValues, queryFieldValues(record, field)...)
	}
	haystack := strings.ToLower(strings.Join(haystackValues, " "))
	for _, keyword := range plan.keywords {
		if !strings.Contains(haystack, strings.ToLower(keyword)) {
			return false, nil
		}
	}
	return true, nil
}

func conditionMatches(record Record, condition normalizedCondition) bool {
	values := queryFieldValues(record, condition.field)
	switch condition.operator {
	case "contains":
		return slices.ContainsFunc(values, func(value string) bool {
			return strings.Contains(strings.ToLower(value), strings.ToLower(condition.value))
		})
	case "=":
		return slices.ContainsFunc(values, func(value string) bool {
			return strings.EqualFold(value, condition.value)
		})
	case "!=":
		return !slices.ContainsFunc(values, func(value string) bool {
			return strings.EqualFold(value, condition.value)
		})
	case ">", ">=", "<", "<=":
		if len(values) == 0 {
			return false
		}
		comparison := compareQueryValues(values[0], condition.value, condition.field)
		switch condition.operator {
		case ">":
			return comparison > 0
		case ">=":
			return comparison >= 0
		case "<":
			return comparison < 0
		case "<=":
			return comparison <= 0
		}
	}
	return false
}

func queryFieldValues(record Record, field FieldDefinition) []string {
	value := queryFieldValue(record, field)
	if field.Localizable {
		return uniqueStrings(value, record.Values[field.Key+"Zh"], record.Values[field.Key+"En"])
	}
	if field.Type == "multiselect" {
		return splitQueryList(value)
	}
	return uniqueStrings(value)
}

func queryFieldValue(record Record, field FieldDefinition) string {
	if field.Source == "values" {
		return record.Values[field.Key]
	}
	switch field.Key {
	case "id":
		return record.ID
	case "code":
		return record.Code
	case "name":
		return record.Name
	case "status":
		return record.Status
	case "description":
		return record.Description
	case "updatedAt":
		return record.UpdatedAt
	default:
		return ""
	}
}

func sortRecords(records []Record, plan queryPlan) {
	if len(plan.sort) == 0 {
		return
	}
	sort.SliceStable(records, func(leftIndex, rightIndex int) bool {
		left := records[leftIndex]
		right := records[rightIndex]
		for _, sorter := range plan.sort {
			comparison := compareQueryValues(queryFieldValue(left, sorter.field), queryFieldValue(right, sorter.field), sorter.field)
			if comparison == 0 {
				continue
			}
			if sorter.order == "desc" {
				return comparison > 0
			}
			return comparison < 0
		}
		return false
	})
}

func compareQueryValues(left string, right string, field FieldDefinition) int {
	if field.Type == "number" {
		leftNumber, leftErr := strconv.ParseFloat(left, 64)
		rightNumber, rightErr := strconv.ParseFloat(right, 64)
		if leftErr == nil && rightErr == nil {
			switch {
			case leftNumber < rightNumber:
				return -1
			case leftNumber > rightNumber:
				return 1
			default:
				return 0
			}
		}
	}
	if field.Type == "datetime" {
		left = comparableDate(left)
		right = comparableDate(right)
	}
	return strings.Compare(strings.ToLower(left), strings.ToLower(right))
}

func comparableDate(value string) string {
	value = strings.TrimSpace(value)
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.UTC().Format(time.DateOnly)
	}
	if parsed, err := time.Parse(time.DateOnly, value); err == nil {
		return parsed.Format(time.DateOnly)
	}
	if len(value) >= len(time.DateOnly) {
		return value[:len(time.DateOnly)]
	}
	return value
}

func splitQueryList(value string) []string {
	return strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\t' || r == ' '
	})
}

func uniqueStrings(values ...string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, value)
	}
	return result
}
