package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"platform-go/internal/platform/capability"
	"platform-go/internal/platform/kernel"
	"platform-go/internal/platform/ratelimit"
	"platform-go/internal/platform/serviceobject"
)

func TestAdminServiceObjectTransportReturnsUnavailableWhenRuntimeIsNotComposed(t *testing.T) {
	server := New(ServerOptions{Capabilities: []capability.Manifest{authProviderTestManifest()}})
	token := loginServiceObjectAdmin(t, server)
	response := serviceObjectRequest(server, token, "/api/admin/service-objects/query", `{"queryId":"platform.reference-records.list","version":"1.0.0"}`)
	if response.Code != http.StatusNotFound || !bytes.Contains(response.Body.Bytes(), []byte(`"code":"SERVICE_OBJECT_UNAVAILABLE"`)) {
		t.Fatalf("uncomposed runtime status = %d body = %s, want stable 404 unavailable", response.Code, response.Body.String())
	}
}

func TestAdminServiceObjectTransportUsesBearerAndServerAuthorization(t *testing.T) {
	query := serviceobject.ReferenceQueryDefinition()
	query.TenantMode = serviceobject.TenantPlatform
	query.DataScope = "platform"
	command := serviceobject.ReferenceCommandDefinition()
	command.TenantMode = serviceobject.TenantPlatform
	command.DataScope = "platform"
	registry, err := serviceobject.NewRegistry([]serviceobject.QueryDefinition{query}, []serviceobject.CommandDefinition{command})
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	runtime, err := serviceobject.NewRuntime(
		registry,
		serviceobject.AuthorizerFunc(func(context.Context, kernel.ExecutionContext, string, string) bool { return false }),
		httpQueryExecutor{}, httpCommandExecutor{}, serviceobject.NewMemoryIdempotencyStore(),
	)
	if err != nil {
		t.Fatalf("NewRuntime() error = %v", err)
	}
	server := New(ServerOptions{
		Capabilities:   []capability.Manifest{authProviderTestManifest()},
		ServiceObjects: runtime,
		Authorizer: permissionSetAuthorizer{
			"admin:reference-records:read":   true,
			"admin:reference-records:update": true,
		},
	})

	unauthenticated := serviceObjectRequest(server, "", "/api/admin/service-objects/query", `{"queryId":"platform.reference-records.list","version":"1.0.0"}`)
	if unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated query status = %d body = %s, want 401", unauthenticated.Code, unauthenticated.Body.String())
	}

	token := loginServiceObjectAdmin(t, server)
	queryResponse := serviceObjectRequest(server, token, "/api/admin/service-objects/query", `{"queryId":"platform.reference-records.list","version":"1.0.0","arguments":{"status":"enabled"},"pagination":{"page":1,"pageSize":10}}`)
	if queryResponse.Code != http.StatusOK || !bytes.Contains(queryResponse.Body.Bytes(), []byte(`"code":"A-001"`)) {
		t.Fatalf("authorized query status = %d body = %s, want 200 reference data", queryResponse.Code, queryResponse.Body.String())
	}
	commandResponse := serviceObjectRequest(server, token, "/api/admin/service-objects/command", `{"commandId":"platform.reference-records.rename","version":"1.0.0","arguments":{"code":"A-001","name":"Renamed"},"idempotencyKey":"rename-1"}`)
	if commandResponse.Code != http.StatusOK || !bytes.Contains(commandResponse.Body.Bytes(), []byte(`"affected":1`)) {
		t.Fatalf("authorized command status = %d body = %s, want 200 affected=1", commandResponse.Code, commandResponse.Body.String())
	}

	tampered := serviceObjectRequest(server, token, "/api/admin/service-objects/query", `{"queryId":"platform.reference-records.list","version":"1.0.0","field":"status"}`)
	if tampered.Code != http.StatusBadRequest {
		t.Fatalf("tampered query status = %d body = %s, want 400", tampered.Code, tampered.Body.String())
	}
}

func TestAdminServiceObjectTransportDoesNotEnumerateDeniedDefinitions(t *testing.T) {
	query := serviceobject.ReferenceQueryDefinition()
	query.TenantMode = serviceobject.TenantPlatform
	query.DataScope = "platform"
	registry, err := serviceobject.NewRegistry([]serviceobject.QueryDefinition{query}, nil)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	runtime, err := serviceobject.NewRuntime(registry, serviceobject.AuthorizerFunc(func(context.Context, kernel.ExecutionContext, string, string) bool { return false }), httpQueryExecutor{}, nil, nil)
	if err != nil {
		t.Fatalf("NewRuntime() error = %v", err)
	}
	server := New(ServerOptions{Capabilities: []capability.Manifest{authProviderTestManifest()}, ServiceObjects: runtime, Authorizer: denyAllAuthorizer{}})
	token := loginServiceObjectAdmin(t, server)
	known := serviceObjectRequest(server, token, "/api/admin/service-objects/query", `{"queryId":"platform.reference-records.list","version":"1.0.0"}`)
	missing := serviceObjectRequest(server, token, "/api/admin/service-objects/query", `{"queryId":"platform.missing.list","version":"1.0.0"}`)
	var knownBody Response[any]
	var missingBody Response[any]
	knownDecodeErr := json.Unmarshal(known.Body.Bytes(), &knownBody)
	missingDecodeErr := json.Unmarshal(missing.Body.Bytes(), &missingBody)
	if known.Code != http.StatusNotFound || missing.Code != http.StatusNotFound || knownDecodeErr != nil || missingDecodeErr != nil ||
		knownBody.Error == nil || missingBody.Error == nil || knownBody.Error.Code != missingBody.Error.Code || knownBody.Error.Message != missingBody.Error.Message ||
		knownBody.Error.RequestID == "" || knownBody.Error.TraceID == "" || missingBody.Error.RequestID == "" || missingBody.Error.TraceID == "" {
		t.Fatalf("known denied=%d %s missing=%d %s, want indistinguishable 404", known.Code, known.Body.String(), missing.Code, missing.Body.String())
	}
}

func TestAdminServiceObjectTransportAppliesDedicatedRateLimit(t *testing.T) {
	query := serviceobject.ReferenceQueryDefinition()
	query.TenantMode = serviceobject.TenantPlatform
	query.DataScope = "platform"
	registry, err := serviceobject.NewRegistry([]serviceobject.QueryDefinition{query}, nil)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	runtime, err := serviceobject.NewRuntime(registry,
		serviceobject.AuthorizerFunc(func(context.Context, kernel.ExecutionContext, string, string) bool { return false }),
		httpQueryExecutor{}, nil, nil)
	if err != nil {
		t.Fatalf("NewRuntime() error = %v", err)
	}
	limiter := &rateLimitTestStub{deny: ratelimit.OperationAdminServiceObjectQuery, retryAfter: time.Minute}
	server := New(ServerOptions{
		Capabilities: []capability.Manifest{authProviderTestManifest()}, ServiceObjects: runtime,
		Authorizer: permissionSetAuthorizer{"admin:reference-records:read": true}, RateLimiter: limiter,
	})
	token := loginServiceObjectAdmin(t, server)
	response := serviceObjectRequest(server, token, "/api/admin/service-objects/query", `{"queryId":"platform.reference-records.list","version":"1.0.0"}`)
	if response.Code != http.StatusTooManyRequests || limiter.calls[ratelimit.OperationAdminServiceObjectQuery] != 1 {
		t.Fatalf("rate-limited query status = %d calls = %d body = %s, want 429", response.Code, limiter.calls[ratelimit.OperationAdminServiceObjectQuery], response.Body.String())
	}
}

func TestAdminServiceObjectRateLimitCannotBeBypassedByChangingObjectID(t *testing.T) {
	query := serviceobject.ReferenceQueryDefinition()
	query.TenantMode = serviceobject.TenantPlatform
	query.DataScope = "platform"
	registry, err := serviceobject.NewRegistry([]serviceobject.QueryDefinition{query}, nil)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	runtime, err := serviceobject.NewRuntime(registry,
		serviceobject.AuthorizerFunc(func(context.Context, kernel.ExecutionContext, string, string) bool { return false }),
		httpQueryExecutor{}, nil, nil)
	if err != nil {
		t.Fatalf("NewRuntime() error = %v", err)
	}
	limiter := &rateLimitTestStub{}
	server := New(ServerOptions{Capabilities: []capability.Manifest{authProviderTestManifest()}, ServiceObjects: runtime, RateLimiter: limiter})
	token := loginServiceObjectAdmin(t, server)
	for _, queryID := range []string{"platform.missing.one", "platform.missing.two"} {
		response := serviceObjectRequest(server, token, "/api/admin/service-objects/query", `{"queryId":"`+queryID+`","version":"1.0.0"}`)
		if response.Code != http.StatusNotFound {
			t.Fatalf("unknown query %s status = %d body = %s, want 404", queryID, response.Code, response.Body.String())
		}
	}
	keys := limiter.keyHistory[ratelimit.OperationAdminServiceObjectQuery]
	if len(keys) != 2 || keys[0] != keys[1] {
		t.Fatalf("service object rate-limit keys = %+v, want one endpoint bucket per actor and IP", keys)
	}
}

func TestAuthorizationServiceObjectCommandsUseOneInvalidationChannel(t *testing.T) {
	for _, commandID := range []string{
		"platform.identity.organization-role-groups.replace",
		"platform.authorization.role-permissions.replace",
		"platform.navigation.role-menus.replace",
	} {
		if !isAuthorizationServiceObjectCommand(commandID) {
			t.Fatalf("isAuthorizationServiceObjectCommand(%q) = false", commandID)
		}
	}
	for _, commandID := range []string{
		"platform.identity.organization-role-group-change.prepare",
		"platform.reference-records.rename",
		"platform.integration.replay",
	} {
		if isAuthorizationServiceObjectCommand(commandID) {
			t.Fatalf("isAuthorizationServiceObjectCommand(%q) = true", commandID)
		}
	}
}

func loginServiceObjectAdmin(t *testing.T, server *Server) string {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"provider":"demo","username":"ops"}`))
	request.Header.Set("Content-Type", "application/json")
	server.Router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("login status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil || payload.Data.Token == "" {
		t.Fatalf("decode login token: %v body = %s", err, recorder.Body.String())
	}
	return payload.Data.Token
}

func serviceObjectRequest(server *Server, token string, path string, body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	server.Router().ServeHTTP(recorder, request)
	return recorder
}

type httpQueryExecutor struct{}

func (httpQueryExecutor) ExecuteQuery(context.Context, serviceobject.QueryPlan) (serviceobject.QueryResult, error) {
	return serviceobject.QueryResult{Items: []map[string]any{{"id": int64(1), "code": "A-001", "name": "Alpha", "status": "enabled"}}}, nil
}

type httpCommandExecutor struct{}

func (httpCommandExecutor) ExecuteCommand(context.Context, serviceobject.CommandPlan) (serviceobject.CommandResult, error) {
	return serviceobject.CommandResult{Values: map[string]any{"affected": int64(1)}}, nil
}
