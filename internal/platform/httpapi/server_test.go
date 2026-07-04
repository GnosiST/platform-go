package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"platform-go/internal/platform/capability"
)

func TestHealthEndpoint(t *testing.T) {
	server := New(ServerOptions{Capabilities: []capability.Manifest{{ID: "tenant"}}})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /api/health status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"ok":true`) {
		t.Fatalf("GET /api/health body = %s", recorder.Body.String())
	}
}

func TestCapabilitiesEndpoint(t *testing.T) {
	server := New(ServerOptions{Capabilities: []capability.Manifest{{ID: "tenant"}, {ID: "identity"}}})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/capabilities", nil)

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /api/capabilities status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"id":"tenant"`) || !strings.Contains(body, `"id":"identity"`) {
		t.Fatalf("GET /api/capabilities body = %s", body)
	}
}
