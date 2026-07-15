package kernel

import (
	"context"
	"strings"
)

type Correlation struct {
	RequestID   string
	TraceID     string
	TraceParent string
}

type correlationContextKey struct{}

func WithCorrelation(ctx context.Context, correlation Correlation) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(correlation.RequestID) == "" || strings.TrimSpace(correlation.TraceID) == "" {
		return ctx
	}
	return context.WithValue(ctx, correlationContextKey{}, correlation)
}

func CorrelationFromContext(ctx context.Context) (Correlation, bool) {
	if ctx == nil {
		return Correlation{}, false
	}
	correlation, ok := ctx.Value(correlationContextKey{}).(Correlation)
	if !ok || strings.TrimSpace(correlation.RequestID) == "" || strings.TrimSpace(correlation.TraceID) == "" {
		return Correlation{}, false
	}
	return correlation, true
}
