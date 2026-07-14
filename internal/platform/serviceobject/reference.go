package serviceobject

import "time"

const (
	ReferenceQueryID   = "platform.reference-records.list"
	ReferenceCommandID = "platform.reference-records.rename"
	ReferenceVersion   = "1.0.0"
)

func ReferenceQueryDefinition() QueryDefinition {
	return QueryDefinition{
		ID: ReferenceQueryID, Version: ReferenceVersion, Resource: "reference-records",
		Permission: "admin:reference-records:read", Action: "read",
		TenantMode: TenantRequired, DataScope: "tenant",
		Arguments: []ArgumentDefinition{
			{Name: "status", Type: ValueString, MaxLength: 32},
			{Name: "codePrefix", Type: ValueString, MaxLength: 64},
		},
		AllowedSort: []SortDefinition{{Name: "code", Field: "code"}, {Name: "name", Field: "name"}},
		Cost: CostPolicy{
			BaseCost: 2, PerRowCost: 1, PerOffsetCost: 1,
			PredicateCost: 1, SortCost: 1, TotalCost: 20, MaxOffset: 1000, Limit: 128,
		}, Timeout: 2 * time.Second,
		MaxPageSize: 100, ExposeTotal: false,
		ResultSchema: []ResultField{
			{Name: "id", Type: ValueInteger}, {Name: "code", Type: ValueString},
			{Name: "name", Type: ValueString}, {Name: "status", Type: ValueString},
		},
		Build: func(arguments ValidatedArguments) (QueryAST, error) {
			predicates := make([]Predicate, 0, 2)
			if status, ok := arguments["status"].(string); ok && status != "" {
				predicates = append(predicates, Predicate{Field: "status", Operator: PredicateEqual, Value: status})
			}
			if prefix, ok := arguments["codePrefix"].(string); ok && prefix != "" {
				predicates = append(predicates, Predicate{Field: "code", Operator: PredicatePrefix, Value: prefix})
			}
			return QueryAST{Resource: "reference-records", Predicates: predicates}, nil
		},
	}
}

func ReferenceCommandDefinition() CommandDefinition {
	return CommandDefinition{
		ID: ReferenceCommandID, Version: ReferenceVersion, Resource: "reference-records",
		Permission: "admin:reference-records:update", Action: "update",
		TenantMode: TenantRequired, DataScope: "tenant",
		Arguments: []ArgumentDefinition{
			{Name: "code", Type: ValueString, Required: true, MaxLength: 64},
			{Name: "name", Type: ValueString, Required: true, MaxLength: 128},
		},
		Cost: CostPolicy{BaseCost: 5, PerRowCost: 1, PredicateCost: 1, Limit: 7}, Timeout: 2 * time.Second,
		Idempotency: IdempotencyRequiredKey, MaxAffectedRows: 1,
		ResultSchema: []ResultField{{Name: "affected", Type: ValueInteger}},
		Build: func(arguments ValidatedArguments) (CommandAST, error) {
			return CommandAST{
				Resource: "reference-records", Kind: MutationUpdate,
				Predicates: []Predicate{{Field: "code", Operator: PredicateEqual, Value: arguments["code"]}},
				Values:     map[string]any{"name": arguments["name"]},
			}, nil
		},
	}
}

func ReferenceGORMBinding(table string) GORMResourceBinding {
	return GORMResourceBinding{
		Table: table, TenantColumn: "tenant_id", OrgColumn: "org_code", AreaColumn: "area_code",
		OwnerColumns:     []string{"owner_code"},
		PredicateColumns: map[string]string{"id": "id", "code": "code", "name": "name", "status": "status"},
		ValueColumns: map[string]string{
			"code": "code", "name": "name", "status": "status",
			"orgCode": "org_code", "areaCode": "area_code", "ownerCode": "owner_code",
		},
		ResultColumns: map[string]string{"id": "id", "code": "code", "name": "name", "status": "status"},
	}
}
