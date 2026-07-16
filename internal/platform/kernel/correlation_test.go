package kernel

import (
	"context"
	"regexp"
	"testing"
)

func TestGenerateCorrelationReturnsOpaqueValidPair(t *testing.T) {
	requestPattern := regexp.MustCompile(`^req_[0-9a-f]{32}$`)
	tracePattern := regexp.MustCompile(`^[0-9a-f]{32}$`)

	first := GenerateCorrelation()
	second := GenerateCorrelation()
	for _, correlation := range []Correlation{first, second} {
		if !requestPattern.MatchString(correlation.RequestID) || !tracePattern.MatchString(correlation.TraceID) {
			t.Fatalf("GenerateCorrelation() = %+v, want opaque request/trace pair", correlation)
		}
		if !ValidCorrelation(correlation) {
			t.Fatalf("ValidCorrelation(%+v) = false", correlation)
		}
	}
	if first.RequestID == second.RequestID || first.TraceID == second.TraceID {
		t.Fatalf("GenerateCorrelation() repeated identifiers: first=%+v second=%+v", first, second)
	}
}

func TestCorrelationContextRoundTripsNormalizedIdentifiers(t *testing.T) {
	want := Correlation{
		RequestID:   "req_0123456789abcdef0123456789abcdef",
		TraceID:     "4bf92f3577b34da6a3ce929d0e0e4736",
		TraceParent: "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
	}

	got, ok := CorrelationFromContext(WithCorrelation(context.Background(), want))
	if !ok || got != want {
		t.Fatalf("CorrelationFromContext() = %+v, %t, want %+v, true", got, ok, want)
	}
}

func TestCorrelationContextRejectsEmptyRequestOrTraceID(t *testing.T) {
	for _, correlation := range []Correlation{
		{TraceID: "4bf92f3577b34da6a3ce929d0e0e4736"},
		{RequestID: "req_0123456789abcdef0123456789abcdef"},
		{RequestID: "client@example.test/private", TraceID: "4bf92f3577b34da6a3ce929d0e0e4736"},
		{RequestID: "req_0123456789abcdef0123456789abcdef", TraceID: "legacy-trace"},
	} {
		if got, ok := CorrelationFromContext(WithCorrelation(context.Background(), correlation)); ok {
			t.Fatalf("CorrelationFromContext() = %+v, true for incomplete correlation %+v", got, correlation)
		}
	}
}
