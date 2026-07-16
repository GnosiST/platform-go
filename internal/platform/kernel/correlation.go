package kernel

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

var (
	requestIDPattern = regexp.MustCompile(`^req_[0-9a-f]{32}$`)
	traceIDPattern   = regexp.MustCompile(`^[0-9a-f]{32}$`)
	fallbackSalt     = sha256.Sum256([]byte(strconv.FormatInt(time.Now().UnixNano(), 10) + ":" + strconv.Itoa(os.Getpid())))
	fallbackSequence atomic.Uint64
)

type Correlation struct {
	RequestID   string
	TraceID     string
	TraceParent string
}

type correlationContextKey struct{}

func GenerateCorrelation() Correlation {
	return Correlation{
		RequestID: "req_" + opaqueCorrelationHex(),
		TraceID:   opaqueCorrelationHex(),
	}
}

func ValidCorrelation(correlation Correlation) bool {
	return requestIDPattern.MatchString(correlation.RequestID) && traceIDPattern.MatchString(correlation.TraceID)
}

func opaqueCorrelationHex() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err == nil {
		return hex.EncodeToString(value[:])
	}
	sequence := fallbackSequence.Add(1)
	digest := sha256.Sum256([]byte(hex.EncodeToString(fallbackSalt[:]) + ":" + strconv.FormatUint(sequence, 10)))
	return hex.EncodeToString(digest[:16])
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
