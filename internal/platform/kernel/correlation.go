package kernel

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"regexp"
	"strings"
)

var (
	requestIDPattern = regexp.MustCompile(`^req_[0-9a-f]{32}$`)
	traceIDPattern   = regexp.MustCompile(`^[0-9a-f]{32}$`)
)

type Correlation struct {
	RequestID   string
	TraceID     string
	TraceParent string
}

type correlationContextKey struct{}

func GenerateCorrelation() (Correlation, error) {
	return generateCorrelation(rand.Reader)
}

func ValidCorrelation(correlation Correlation) bool {
	return requestIDPattern.MatchString(correlation.RequestID) && traceIDPattern.MatchString(correlation.TraceID)
}

func generateCorrelation(random io.Reader) (Correlation, error) {
	var value [32]byte
	if _, err := io.ReadFull(random, value[:]); err != nil {
		return Correlation{}, fmt.Errorf("generate correlation: %w", err)
	}
	return Correlation{
		RequestID: "req_" + hex.EncodeToString(value[:16]),
		TraceID:   hex.EncodeToString(value[16:]),
	}, nil
}

func WithCorrelation(ctx context.Context, correlation Correlation) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	correlation.RequestID = strings.TrimSpace(correlation.RequestID)
	correlation.TraceID = strings.TrimSpace(correlation.TraceID)
	if !ValidCorrelation(correlation) {
		return ctx
	}
	return context.WithValue(ctx, correlationContextKey{}, correlation)
}

func CorrelationFromContext(ctx context.Context) (Correlation, bool) {
	if ctx == nil {
		return Correlation{}, false
	}
	correlation, ok := ctx.Value(correlationContextKey{}).(Correlation)
	if !ok || !ValidCorrelation(correlation) {
		return Correlation{}, false
	}
	return correlation, true
}
