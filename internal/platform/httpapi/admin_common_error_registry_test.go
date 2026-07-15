package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"platform-go/internal/platform/capability"
	"platform-go/internal/platform/errorcode"
	"platform-go/internal/platform/ratelimit"
)

func TestAdminInvalidCredentialsUseCanonicalRegistryMessage(t *testing.T) {
	server := newTestServer(ServerOptions{Capabilities: []capability.Manifest{authProviderTestManifest()}})
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"provider":"demo","username":"missing-user"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	server.Router().ServeHTTP(recorder, request)

	body := decodePlatformErrorResponse(t, recorder)
	if recorder.Code != http.StatusUnauthorized || body.Error.Code != errorcode.CodeAuthInvalidCredentials || body.Error.Message != "invalid credentials" {
		t.Fatalf("status = %d error = %+v, want canonical invalid credentials", recorder.Code, body.Error)
	}
}

func TestAdminProviderFailureUsesRegistryCauseWriterOnce(t *testing.T) {
	const marker = "private-provider-failure"
	sink := &recordingInternalErrorSink{}
	server := newTestServer(ServerOptions{
		Capabilities: []capability.Manifest{adminOIDCProviderManifest(true, true)},
		AdminIdentityResolver: adminIdentityResolverFunc{
			start: func(_ context.Context, _ AdminIdentityStartInput) (AdminIdentityStart, error) {
				return AdminIdentityStart{}, errors.New(marker)
			},
			resolve: func(_ context.Context, _ AdminIdentityResolveInput) (AdminIdentity, error) {
				return AdminIdentity{}, nil
			},
		},
		InternalErrorSink: sink,
	})
	request := httptest.NewRequest(http.MethodPost, "/api/auth/providers/oidc/start", bytes.NewBufferString(`{"codeChallenge":"47DEQpj8HBSa-_TImW-5JCeuQeRkm5NMpJWZG3hSuFU"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	server.Router().ServeHTTP(recorder, request)

	body := decodePlatformErrorResponse(t, recorder)
	assertRegisteredCauseEvent(t, sink, errorcode.CodeAuthProviderStartFailed, body.Error)
	if recorder.Code != http.StatusBadGateway || strings.Contains(recorder.Body.String(), marker) {
		t.Fatalf("status = %d body = %s, want safe provider failure", recorder.Code, recorder.Body.String())
	}
}

func TestAdminRateLimitFailureUsesRegistryCauseWriterOnce(t *testing.T) {
	const marker = "private-rate-limit-backend"
	builder, err := ratelimit.NewKeyBuilder([]byte(strings.Repeat("r", 32)))
	if err != nil {
		t.Fatal(err)
	}
	sink := &recordingInternalErrorSink{}
	server := newTestServer(ServerOptions{
		Capabilities:        []capability.Manifest{authProviderTestManifest()},
		RateLimiter:         &rateLimitTestStub{deny: ratelimit.OperationAdminLogin, err: errors.New(marker)},
		RateLimitKeyBuilder: builder,
		InternalErrorSink:   sink,
	})
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"provider":"demo","username":"admin"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	server.Router().ServeHTTP(recorder, request)

	body := decodePlatformErrorResponse(t, recorder)
	assertRegisteredCauseEvent(t, sink, errorcode.CodeRateLimitUnavailable, body.Error)
	if recorder.Code != http.StatusServiceUnavailable || strings.Contains(recorder.Body.String(), marker) {
		t.Fatalf("status = %d body = %s, want safe rate dependency failure", recorder.Code, recorder.Body.String())
	}
}

func TestAdminRegistryOwnedFunctionsDoNotUseLegacyErrorWriters(t *testing.T) {
	targetsByFile := map[string]map[string]bool{
		"server.go": {
			"enforceRateLimit": true, "authProviderStart": true, "authLogin": true,
			"issueAdminLogin": true, "authRefresh": true, "authLogout": true,
			"adminServiceObjectInvocation": true, "adminCurrentSession": true, "adminMenus": true,
			"adminPolicyReviewApprove": true, "adminPolicyReviewRequest": true,
			"adminPolicyReviewReject": true, "adminPolicyReviewExport": true,
			"authorize": true, "authorizeAdminResourcePrincipal": true,
		},
		"sensitive_reveal.go": {"resolveAdminSensitiveRevealTarget": true},
	}
	legacy := map[string]bool{
		"writeAuthError": true, "writeUnauthorized": true, "writeForbidden": true,
		"recordInternalError": true,
	}
	for filename, targets := range targetsByFile {
		file, err := parser.ParseFile(token.NewFileSet(), filename, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || !targets[function.Name.Name] {
				continue
			}
			ast.Inspect(function.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				var name string
				switch expression := call.Fun.(type) {
				case *ast.Ident:
					name = expression.Name
				case *ast.SelectorExpr:
					name = expression.Sel.Name
				}
				if legacy[name] {
					t.Errorf("%s:%s still calls legacy error surface %s", filename, function.Name.Name, name)
				}
				return true
			})
		}
	}
}

func decodePlatformErrorResponse(t *testing.T, recorder *httptest.ResponseRecorder) Response[any] {
	t.Helper()
	var body Response[any]
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error response: %v body=%s", err, recorder.Body.String())
	}
	if body.Error == nil || body.Error.RequestID == "" || body.Error.TraceID == "" {
		t.Fatalf("error response lacks correlation: %+v", body.Error)
	}
	return body
}

func assertRegisteredCauseEvent(t *testing.T, sink *recordingInternalErrorSink, code errorcode.Code, body *ErrorBody) {
	t.Helper()
	if len(sink.events) != 1 {
		t.Fatalf("events = %+v, want exactly one", sink.events)
	}
	event := sink.events[0]
	if event.Code != string(code) || event.RequestID != body.RequestID || event.TraceID != body.TraceID || event.Owner == "" {
		t.Fatalf("event = %+v body = %+v, want matching registered correlation", event, body)
	}
}
