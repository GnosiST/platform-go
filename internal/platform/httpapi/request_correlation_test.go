package httpapi

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"platform-go/internal/platform/kernel"
)

var requestIDPatternForTest = regexp.MustCompile(`^req_[0-9a-f]{32}$`)

type saltThenFailCorrelationReader struct {
	salt byte
	read bool
}

func (reader *saltThenFailCorrelationReader) Read(value []byte) (int, error) {
	if reader.read {
		return 0, errors.New("injected random failure")
	}
	reader.read = true
	for index := range value {
		value[index] = reader.salt
	}
	return len(value), nil
}

func TestRequestCorrelationGeneratesServerOwnedRequestID(t *testing.T) {
	const clientRequestID = "email@example.test/private/path"
	var correlation kernel.Correlation
	router := gin.New()
	router.Use(requestCorrelation())
	router.GET("/test", func(ctx *gin.Context) {
		correlation, _ = kernel.CorrelationFromContext(ctx.Request.Context())
		ctx.Status(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodGet, "/test", nil)
	request.Header.Set("X-Request-ID", clientRequestID)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if !requestIDPatternForTest.MatchString(correlation.RequestID) {
		t.Fatalf("request ID = %q, want req_ plus 32 lowercase hex", correlation.RequestID)
	}
	if correlation.RequestID == clientRequestID || strings.Contains(recorder.Header().Get("X-Request-ID"), "email@example.test") {
		t.Fatalf("client request ID was reflected: correlation=%+v headers=%v", correlation, recorder.Header())
	}
	if got := recorder.Header().Get("X-Request-ID"); got != correlation.RequestID {
		t.Fatalf("X-Request-ID = %q, want %q", got, correlation.RequestID)
	}
}

func TestRequestCorrelationPreservesValidTraceIDAndFlagsWithNewSpan(t *testing.T) {
	const incoming = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	var correlation kernel.Correlation
	router := gin.New()
	router.Use(requestCorrelation())
	router.GET("/test", func(ctx *gin.Context) {
		correlation, _ = kernel.CorrelationFromContext(ctx.Request.Context())
		ctx.Status(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodGet, "/test", nil)
	request.Header.Set("traceparent", incoming)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	parts := strings.Split(correlation.TraceParent, "-")
	if len(parts) != 4 || parts[0] != "00" || parts[1] != "4bf92f3577b34da6a3ce929d0e0e4736" || parts[3] != "01" {
		t.Fatalf("traceparent = %q, want preserved version/trace/flags", correlation.TraceParent)
	}
	if parts[2] == "00f067aa0ba902b7" || parts[2] == "0000000000000000" {
		t.Fatalf("server span ID = %q, want new non-zero span", parts[2])
	}
	if correlation.TraceID != parts[1] || recorder.Header().Get("traceparent") != correlation.TraceParent {
		t.Fatalf("correlation/header mismatch: %+v headers=%v", correlation, recorder.Header())
	}
}

func TestRequestCorrelationRejectsInvalidOrZeroTraceparent(t *testing.T) {
	for _, incoming := range []string{
		"00-00000000000000000000000000000000-00f067aa0ba902b7-01",
		"00-4bf92f3577b34da6a3ce929d0e0e4736-0000000000000000-01",
		"01-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
		"00-4BF92F3577B34DA6A3CE929D0E0E4736-00f067aa0ba902b7-01",
	} {
		t.Run(incoming, func(t *testing.T) {
			var correlation kernel.Correlation
			router := gin.New()
			router.Use(requestCorrelation())
			router.GET("/test", func(ctx *gin.Context) {
				correlation, _ = kernel.CorrelationFromContext(ctx.Request.Context())
				ctx.Status(http.StatusNoContent)
			})
			request := httptest.NewRequest(http.MethodGet, "/test", nil)
			request.Header.Set("traceparent", incoming)
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, request)

			if correlation.TraceID == "" || strings.Contains(correlation.TraceParent, strings.Split(incoming, "-")[1]) {
				t.Fatalf("invalid traceparent %q was retained as %+v", incoming, correlation)
			}
		})
	}
}

func TestRequestCorrelationRejectsDuplicateTraceparentHeaders(t *testing.T) {
	const incoming = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	var correlation kernel.Correlation
	router := gin.New()
	router.Use(requestCorrelation())
	router.GET("/test", func(ctx *gin.Context) {
		correlation, _ = kernel.CorrelationFromContext(ctx.Request.Context())
		ctx.Status(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodGet, "/test", nil)
	request.Header.Add("traceparent", incoming)
	request.Header.Add("traceparent", incoming)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if correlation.TraceID == "" || correlation.TraceID == "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("duplicate traceparent was retained as %+v", correlation)
	}
}

func TestCorrelationGeneratorFallbackIsFormattedUniqueAndSalted(t *testing.T) {
	first := newCorrelationIDGenerator(&saltThenFailCorrelationReader{salt: 0x11})
	second := newCorrelationIDGenerator(&saltThenFailCorrelationReader{salt: 0x22})
	seen := make(map[string]struct{})
	for index := 0; index < 4; index++ {
		requestID := "req_" + first.randomHex(16)
		if !requestIDPatternForTest.MatchString(requestID) {
			t.Fatalf("fallback request ID = %q, want req_ plus 32 lowercase hex", requestID)
		}
		if _, exists := seen[requestID]; exists {
			t.Fatalf("duplicate fallback request ID %q", requestID)
		}
		seen[requestID] = struct{}{}
	}
	if first.randomHex(16) == second.randomHex(16) {
		t.Fatal("fallback output did not incorporate process salt")
	}
}
