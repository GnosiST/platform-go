package sensitivemigration

import (
	"strings"
	"testing"
)

func TestDecodeUniqueObjectPreservesRawValues(t *testing.T) {
	object, err := DecodeUniqueObject(`{"large":9007199254740993,"nested":{"value":1},"text":"kept"}`)
	if err != nil {
		t.Fatalf("DecodeUniqueObject() error = %v", err)
	}
	if string(object["large"]) != "9007199254740993" || string(object["nested"]) != `{"value":1}` || string(object["text"]) != `"kept"` {
		t.Fatalf("DecodeUniqueObject() = %+v", object)
	}
}

func TestDecodeUniqueObjectRejectsDuplicateTrailingAndNonObjectJSON(t *testing.T) {
	for _, testCase := range []struct {
		name string
		raw  string
	}{
		{name: "duplicate", raw: `{"field":1,"field":2}`},
		{name: "trailing", raw: `{"field":1} {"other":2}`},
		{name: "array", raw: `[{"field":1}]`},
		{name: "scalar", raw: `"value"`},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := DecodeUniqueObject(testCase.raw); err == nil || strings.Contains(err.Error(), testCase.raw) {
				t.Fatalf("DecodeUniqueObject(%s) error = %v, want sanitized rejection", testCase.name, err)
			}
		})
	}
}
