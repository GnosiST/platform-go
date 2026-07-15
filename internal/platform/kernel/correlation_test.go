package kernel

import (
	"context"
	"testing"
)

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
	} {
		if got, ok := CorrelationFromContext(WithCorrelation(context.Background(), correlation)); ok {
			t.Fatalf("CorrelationFromContext() = %+v, true for incomplete correlation %+v", got, correlation)
		}
	}
}
