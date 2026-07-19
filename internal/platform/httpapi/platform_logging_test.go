package httpapi

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/GnosiST/platform-go/internal/platform/errorcode"
	"github.com/GnosiST/platform-go/internal/platform/kernel"
)

func TestDefaultRequestLoggingSinkPersistsSafeAuditLog(t *testing.T) {
	clock := monotonicClockForPlatformLoggingTest(time.Date(2026, 7, 19, 10, 0, 0, 0, time.UTC))
	server := newTestServer(ServerOptions{
		Capabilities: capabilitiesFromConfigForTest(t, []string{"dictionary", "tenant", "identity", "session", "rbac", "audit"}),
		Now:          clock,
	})
	body := bytes.NewBufferString(`{"provider":"demo","username":"admin","password":"correct-password","otp":"123456","phone":"+8613800138000","email":"owner@example.test"}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login?password=query-secret&otp=654321&phone=%2B8613800138000&email=owner@example.test", body)
	request.Header.Set("Authorization", "Bearer request-private-token")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	request.Header.Set("X-Forwarded-For", "198.51.100.99")
	request.RemoteAddr = "203.0.113.10:43110"

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("POST auth login status = %d body = %s, want 200", recorder.Code, recorder.Body.String())
	}
	logs, err := server.resources.InternalRecordsContext(context.Background(), requestLogResource)
	if err != nil {
		t.Fatalf("InternalRecordsContext(%s) error = %v", requestLogResource, err)
	}
	if len(logs) != 1 {
		t.Fatalf("request logs = %d, want 1", len(logs))
	}
	log := logs[0]
	values := log.Values
	if log.Status != "success" ||
		values["method"] != http.MethodPost ||
		values["route"] != "/api/auth/login" ||
		values["statusCode"] != strconv.Itoa(http.StatusOK) ||
		values["domain"] != "auth" {
		t.Fatalf("request log = %+v, want auth POST success log", log)
	}
	latency, err := strconv.Atoi(values["latencyMs"])
	if err != nil || latency <= 0 {
		t.Fatalf("latencyMs = %q, want positive integer", values["latencyMs"])
	}
	correlation := kernel.Correlation{RequestID: values["requestId"], TraceID: values["traceId"]}
	if !kernel.ValidCorrelation(correlation) ||
		values["requestId"] != recorder.Header().Get("X-Request-ID") ||
		values["traceId"] != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("request log correlation = %+v headers=%v", correlation, recorder.Header())
	}
	if !strings.HasPrefix(values["clientIpHash"], "v1:sha256:client-ip:") {
		t.Fatalf("clientIpHash = %q, want versioned hash", values["clientIpHash"])
	}
	assertPlatformLogRecordDoesNotLeak(t, log,
		"Authorization",
		"request-private-token",
		"query-secret",
		"correct-password",
		"123456",
		"654321",
		"+8613800138000",
		"13800138000",
		"owner@example.test",
		"203.0.113.10",
		"198.51.100.99",
	)
}

func TestDefaultInternalErrorSinkPersistsErrorLogInAuditCenter(t *testing.T) {
	clock := monotonicClockForPlatformLoggingTest(time.Date(2026, 7, 19, 10, 30, 0, 0, time.UTC))
	server := newTestServer(ServerOptions{
		Capabilities: capabilitiesFromConfigForTest(t, []string{"dictionary", "tenant", "identity", "session", "rbac", "audit"}),
		Now:          clock,
		AdminRoutes: []AdminRouteRegistration{{
			Method: http.MethodGet,
			Path:   "/api/admin/test-panic",
			Handler: func(*gin.Context) {
				panic("private-panic-marker password=correct-password otp=123456 phone=+8613800138000 email=owner@example.test")
			},
		}},
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/admin/test-panic?password=query-secret&otp=654321", strings.NewReader("request-private-body"))
	request.Header.Set("Authorization", "Bearer request-private-token")
	request.RemoteAddr = "203.0.113.10:43110"

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("GET panic route status = %d body = %s, want 500", recorder.Code, recorder.Body.String())
	}
	errorLogs, err := server.resources.InternalRecordsContext(context.Background(), errorLogResource)
	if err != nil {
		t.Fatalf("InternalRecordsContext(%s) error = %v", errorLogResource, err)
	}
	requestLogs, err := server.resources.InternalRecordsContext(context.Background(), requestLogResource)
	if err != nil {
		t.Fatalf("InternalRecordsContext(%s) error = %v", requestLogResource, err)
	}
	if len(errorLogs) != 1 || len(requestLogs) != 1 {
		t.Fatalf("error logs = %d request logs = %d, want one each in audit resources", len(errorLogs), len(requestLogs))
	}
	errorLog := errorLogs[0]
	requestLog := requestLogs[0]
	if errorLog.Status != "open" ||
		!strings.Contains(errorLog.Name, string(errorcode.CodeInternal)) ||
		errorLog.Description != "Platform internal error recorded with public-safe diagnostics." {
		t.Fatalf("error log = %+v request log = %+v, want default audit error log", errorLog, requestLog)
	}
	if requestLog.Status != "error" ||
		requestLog.Values["method"] != http.MethodGet ||
		requestLog.Values["route"] != "/api/admin/test-panic" ||
		requestLog.Values["statusCode"] != strconv.Itoa(http.StatusInternalServerError) {
		t.Fatalf("request log = %+v, want failed request log next to error log", requestLog)
	}
	assertPlatformLogRecordDoesNotLeak(t, errorLog,
		"private-panic-marker",
		"request-private-token",
		"request-private-body",
		"query-secret",
		"correct-password",
		"123456",
		"654321",
		"+8613800138000",
		"13800138000",
		"owner@example.test",
		"203.0.113.10",
	)
	assertPlatformLogRecordDoesNotLeak(t, requestLog,
		"request-private-token",
		"request-private-body",
		"query-secret",
		"correct-password",
		"123456",
		"654321",
		"+8613800138000",
		"13800138000",
		"owner@example.test",
		"203.0.113.10",
	)
}

func monotonicClockForPlatformLoggingTest(start time.Time) func() time.Time {
	current := start.Add(-25 * time.Millisecond)
	return func() time.Time {
		current = current.Add(25 * time.Millisecond)
		return current
	}
}

func assertPlatformLogRecordDoesNotLeak(t *testing.T, record any, markers ...string) {
	t.Helper()
	serialized := fmt.Sprintf("%+v", record)
	for _, marker := range markers {
		if strings.Contains(serialized, marker) {
			t.Fatalf("platform log leaked %q: %s", marker, serialized)
		}
	}
}
