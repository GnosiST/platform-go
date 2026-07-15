package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"platform-go/internal/platform/errorcode"
)

type errorResponseSink struct {
	events []InternalErrorEvent
}

func (sink *errorResponseSink) Record(_ context.Context, event InternalErrorEvent) {
	sink.events = append(sink.events, event)
}

func TestPlatformErrorWriterUsesRegisteredEnvelopeWithoutData(t *testing.T) {
	router := gin.New()
	router.Use(requestCorrelation())
	router.GET("/test", func(ctx *gin.Context) {
		writePlatformError(ctx, errorcode.CodeRequestBodyTooLarge)
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/test", nil))

	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d body = %s, want 413", recorder.Code, recorder.Body.String())
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(recorder.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode response: %v body=%s", err, recorder.Body.String())
	}
	if _, exists := raw["data"]; exists {
		t.Fatalf("error response contains data: %s", recorder.Body.String())
	}
	var body Response[any]
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode typed response: %v", err)
	}
	if body.Error == nil || body.Error.Code != errorcode.CodeRequestBodyTooLarge || body.Error.Message != "request body exceeds configured limit" || body.Error.RequestID == "" || body.Error.TraceID == "" {
		t.Fatalf("error body = %+v", body.Error)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw["error"], &fields); err != nil || len(fields) != 4 {
		t.Fatalf("error fields = %v, err=%v, want exactly four fields", fields, err)
	}
}

func TestPlatformErrorWriterFallsBackToRegisteredInternalError(t *testing.T) {
	router := gin.New()
	router.Use(requestCorrelation())
	router.GET("/test", func(ctx *gin.Context) {
		writePlatformError(ctx, errorcode.Code("NOT_REGISTERED"))
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/test", nil))

	var body Response[any]
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusInternalServerError || body.Error == nil || body.Error.Code != errorcode.CodeInternal || body.Error.Message != "internal server error" {
		t.Fatalf("status = %d body = %+v", recorder.Code, body)
	}
}

func TestPlatformErrorWriterWithCauseRecordsOnlySafeRegistryMetadata(t *testing.T) {
	const marker = "password=private-marker physical_table=users"
	const clientRequestID = "email@example.test/private/path"
	sink := &errorResponseSink{}
	router := gin.New()
	router.Use(requestCorrelation())
	router.POST("/test", func(ctx *gin.Context) {
		writePlatformErrorWithCause(ctx, sink, errorcode.CodeInternal, errors.New(marker))
	})
	request := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(marker))
	request.Header.Set("Authorization", "Bearer private-token")
	request.Header.Set("X-Request-ID", clientRequestID)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if len(sink.events) != 1 {
		t.Fatalf("events = %+v, want one", sink.events)
	}
	event := sink.events[0]
	if event.Code != string(errorcode.CodeInternal) || event.Owner != "platform.kernel" || event.Category != errorcode.CategoryInternal || event.RequestID == "" || event.TraceID == "" {
		t.Fatalf("event = %+v, want registry metadata and correlation", event)
	}
	clientDigest := sha256.Sum256([]byte("platform-go:request-correlation:v1\x00" + clientRequestID))
	if event.EventID == "request:v1:"+hex.EncodeToString(clientDigest[:]) || event.RequestID == clientRequestID {
		t.Fatalf("event correlation used client request ID: %+v", event)
	}
	serialized := fmt.Sprintf("%+v", event)
	for _, forbidden := range []string{marker, "private-token", "physical_table", "users"} {
		if strings.Contains(serialized, forbidden) || strings.Contains(recorder.Body.String(), forbidden) {
			t.Fatalf("error surface leaked %q: event=%s body=%s", forbidden, serialized, recorder.Body.String())
		}
	}
}
