package httpapi

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/GnosiST/platform-go/internal/platform/kernel"
)

var (
	traceParentPattern          = regexp.MustCompile(`^00-([0-9a-f]{32})-([0-9a-f]{16})-([0-9a-f]{2})$`)
	correlationProcessStartedAt = time.Now().UnixNano()
	defaultCorrelationGenerator = newCorrelationIDGenerator(rand.Reader)
)

type correlationIDGenerator struct {
	random  io.Reader
	salt    [sha256.Size]byte
	readMu  sync.Mutex
	counter atomic.Uint64
}

func newCorrelationIDGenerator(random io.Reader) *correlationIDGenerator {
	generator := &correlationIDGenerator{random: random}
	if generator.readRandom(generator.salt[:]) && !allZeroHex(hex.EncodeToString(generator.salt[:])) {
		return generator
	}
	generator.salt = processFallbackCorrelationSalt()
	return generator
}

func requestCorrelation() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		requestID := "req_" + defaultCorrelationGenerator.randomHex(16)
		traceID, flags := incomingTraceContext(ctx)
		if traceID == "" {
			traceID = defaultCorrelationGenerator.nonZeroRandomHex(16)
			flags = "00"
		}
		traceParent := "00-" + traceID + "-" + defaultCorrelationGenerator.nonZeroRandomHex(8) + "-" + flags
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

func (generator *correlationIDGenerator) randomHex(size int) string {
	value := make([]byte, size)
	if generator.readRandom(value) {
		return hex.EncodeToString(value)
	}
	var counter [8]byte
	binary.BigEndian.PutUint64(counter[:], generator.counter.Add(1))
	material := make([]byte, 0, len("platform-go:correlation-fallback:v1\x00")+len(generator.salt)+len(counter))
	material = append(material, "platform-go:correlation-fallback:v1\x00"...)
	material = append(material, generator.salt[:]...)
	material = append(material, counter[:]...)
	digest := sha256.Sum256(material)
	return hex.EncodeToString(digest[:size])
}

func (generator *correlationIDGenerator) nonZeroRandomHex(size int) string {
	value := generator.randomHex(size)
	if allZeroHex(value) {
		return value[:len(value)-1] + "1"
	}
	return value
}

func (generator *correlationIDGenerator) readRandom(value []byte) bool {
	if generator.random == nil {
		return false
	}
	generator.readMu.Lock()
	defer generator.readMu.Unlock()
	_, err := io.ReadFull(generator.random, value)
	return err == nil
}

func processFallbackCorrelationSalt() [sha256.Size]byte {
	hostname, _ := os.Hostname()
	addressMarker := new(byte)
	seed := fmt.Sprintf("%d\x00%d\x00%s\x00%p", os.Getpid(), correlationProcessStartedAt, hostname, addressMarker)
	return sha256.Sum256([]byte(seed))
}
