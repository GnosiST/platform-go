package httpapi

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"

	"platform-go/internal/platform/kernel"
)

var (
	traceParentPattern       = regexp.MustCompile(`^00-([0-9a-f]{32})-([0-9a-f]{16})-([0-9a-f]{2})$`)
	correlationFallbackCount atomic.Uint64
)

func requestCorrelation() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		requestID := "req_" + correlationRandomHex(16)
		traceID, flags := incomingTraceContext(ctx)
		if traceID == "" {
			traceID = correlationNonZeroRandomHex(16)
			flags = "00"
		}
		traceParent := "00-" + traceID + "-" + correlationNonZeroRandomHex(8) + "-" + flags
		correlation := kernel.Correlation{RequestID: requestID, TraceID: traceID, TraceParent: traceParent}

		ctx.Header("X-Request-ID", requestID)
		ctx.Header("traceparent", traceParent)
		ctx.Request = ctx.Request.WithContext(kernel.WithCorrelation(ctx.Request.Context(), correlation))
		ctx.Next()
	}
}

func incomingTraceContext(ctx *gin.Context) (string, string) {
	values := ctx.Request.Header.Values("traceparent")
	if len(values) != 1 {
		return "", ""
	}
	matches := traceParentPattern.FindStringSubmatch(values[0])
	if len(matches) != 4 || allZeroHex(matches[1]) || allZeroHex(matches[2]) {
		return "", ""
	}
	return matches[1], matches[3]
}

func allZeroHex(value string) bool {
	return strings.Trim(value, "0") == ""
}

func correlationRandomHex(size int) string {
	value := make([]byte, size)
	if _, err := rand.Read(value); err == nil {
		return hex.EncodeToString(value)
	}
	seed := strconv.FormatInt(time.Now().UnixNano(), 10) + ":" + strconv.FormatUint(correlationFallbackCount.Add(1), 10)
	digest := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(digest[:size])
}

func correlationNonZeroRandomHex(size int) string {
	value := correlationRandomHex(size)
	if allZeroHex(value) {
		return value[:len(value)-1] + "1"
	}
	return value
}
