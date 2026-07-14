package serviceobject

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRegistryRejectsReservedPhysicalClientArgumentsAndDuplicateVersions(t *testing.T) {
	for _, name := range []string{"datasource", "datasourceId", "shardKey", "schemaVersion", "fieldName", "operatorType", "joinTarget"} {
		definition := ReferenceQueryDefinition()
		definition.Arguments = append(definition.Arguments, ArgumentDefinition{Name: name, Type: ValueString, MaxLength: 20})
		if _, err := NewRegistry([]QueryDefinition{definition}, nil); !errors.Is(err, ErrDefinitionInvalid) {
			t.Fatalf("NewRegistry(reserved argument %q) error = %v, want ErrDefinitionInvalid", name, err)
		}
	}

	definition := ReferenceQueryDefinition()
	if _, err := NewRegistry([]QueryDefinition{definition, definition}, nil); !errors.Is(err, ErrDefinitionConflict) {
		t.Fatalf("NewRegistry(duplicate) error = %v, want ErrDefinitionConflict", err)
	}
}

func TestRegistryClonesDefinitionSlicesOnRegistrationAndLookup(t *testing.T) {
	definition := ReferenceQueryDefinition()
	minimum, maximum := int64(1), int64(10)
	definition.Arguments = append(definition.Arguments, ArgumentDefinition{Name: "priority", Type: ValueInteger, Minimum: &minimum, Maximum: &maximum})
	registry, err := NewRegistry([]QueryDefinition{definition}, nil)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	definition.Arguments[0].Name = "mutatedInput"
	minimum = 5
	maximum = 6
	stored, ok := registry.query(ReferenceQueryID, ReferenceVersion)
	if !ok || stored.Arguments[0].Name != "status" || *stored.Arguments[2].Minimum != 1 || *stored.Arguments[2].Maximum != 10 {
		t.Fatalf("registered definition = %+v, want immutable status argument", stored.Arguments)
	}
	stored.Arguments[0].Name = "mutatedLookup"
	*stored.Arguments[2].Minimum = 9
	again, _ := registry.query(ReferenceQueryID, ReferenceVersion)
	if again.Arguments[0].Name != "status" || *again.Arguments[2].Minimum != 1 {
		t.Fatalf("lookup mutation changed registry: %+v", again.Arguments)
	}
}

func TestRegistryRejectsUnpricedOrUnboundedQueryWork(t *testing.T) {
	for name, mutate := range map[string]func(*QueryDefinition){
		"row":       func(definition *QueryDefinition) { definition.Cost.PerRowCost = 0 },
		"predicate": func(definition *QueryDefinition) { definition.Cost.PredicateCost = 0 },
		"offset":    func(definition *QueryDefinition) { definition.Cost.PerOffsetCost = 0 },
		"sort":      func(definition *QueryDefinition) { definition.Cost.SortCost = 0 },
		"total": func(definition *QueryDefinition) {
			definition.ExposeTotal = true
			definition.Cost.TotalCost = 0
		},
		"hard offset": func(definition *QueryDefinition) { definition.Cost.MaxOffset = maximumQueryOffset + 1 },
	} {
		definition := ReferenceQueryDefinition()
		mutate(&definition)
		if _, err := NewRegistry([]QueryDefinition{definition}, nil); !errors.Is(err, ErrDefinitionInvalid) {
			t.Fatalf("NewRegistry(unpriced %s) error = %v, want ErrDefinitionInvalid", name, err)
		}
	}
}

func TestStrictRequestDecoderRejectsComposableAndPhysicalQueryFields(t *testing.T) {
	malicious := []string{
		`{"queryId":"platform.reference-records.list","version":"1.0.0","field":"secret"}`,
		`{"queryId":"platform.reference-records.list","version":"1.0.0","operator":"eq"}`,
		`{"queryId":"platform.reference-records.list","version":"1.0.0","datasource":"primary"}`,
		`{"queryId":"platform.reference-records.list","version":"1.0.0","arguments":{}} {}`,
	}
	for _, payload := range malicious {
		if _, err := DecodeQueryRequest(strings.NewReader(payload)); !errors.Is(err, ErrRequestInvalid) {
			t.Fatalf("DecodeQueryRequest(%s) error = %v, want ErrRequestInvalid", payload, err)
		}
	}
}

func TestRegistryRequiresBoundedTypedContracts(t *testing.T) {
	definition := ReferenceQueryDefinition()
	definition.Timeout = 0
	if _, err := NewRegistry([]QueryDefinition{definition}, nil); !errors.Is(err, ErrDefinitionInvalid) {
		t.Fatalf("NewRegistry(timeout=0) error = %v, want ErrDefinitionInvalid", err)
	}

	definition = ReferenceQueryDefinition()
	definition.Arguments[0].MaxLength = 0
	if _, err := NewRegistry([]QueryDefinition{definition}, nil); !errors.Is(err, ErrDefinitionInvalid) {
		t.Fatalf("NewRegistry(unbounded string) error = %v, want ErrDefinitionInvalid", err)
	}

	definition = ReferenceQueryDefinition()
	definition.Timeout = 61 * time.Second
	if _, err := NewRegistry([]QueryDefinition{definition}, nil); !errors.Is(err, ErrDefinitionInvalid) {
		t.Fatalf("NewRegistry(long timeout) error = %v, want ErrDefinitionInvalid", err)
	}
}
