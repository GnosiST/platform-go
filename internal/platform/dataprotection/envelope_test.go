package dataprotection

import "testing"

func TestEnvelopeShapeClassificationDoesNotDecodeValues(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  EnvelopeShape
	}{
		{name: "plaintext", value: "fixture-sensitive-value", want: EnvelopeShapeNone},
		{name: "current valid shape", value: "pgo:enc:v1:payload", want: EnvelopeShapeCurrent},
		{name: "current malformed payload", value: "pgo:enc:v1:", want: EnvelopeShapeCurrent},
		{name: "foreign version", value: "pgo:enc:v2:payload", want: EnvelopeShapeForeign},
		{name: "malformed version token", value: "pgo:enc:foreign:payload", want: EnvelopeShapeMalformed},
		{name: "missing version delimiter", value: "pgo:enc:v1", want: EnvelopeShapeMalformed},
		{name: "missing version", value: "pgo:enc:", want: EnvelopeShapeMalformed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClassifyEnvelopeShape(tt.value); got != tt.want {
				t.Fatalf("ClassifyEnvelopeShape() = %q, want %q", got, tt.want)
			}
		})
	}
}
