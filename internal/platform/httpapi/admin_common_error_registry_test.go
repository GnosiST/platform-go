package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/GnosiST/platform-go/internal/platform/adminresource"
	"github.com/GnosiST/platform-go/internal/platform/authjwt"
	"github.com/GnosiST/platform-go/internal/platform/capability"
	"github.com/GnosiST/platform-go/internal/platform/errorcode"
	"github.com/GnosiST/platform-go/internal/platform/ratelimit"
	"github.com/GnosiST/platform-go/internal/platform/session"
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

func TestRateLimitDependencyFailuresRespectPlaneSinkBoundary(t *testing.T) {
	const marker = "private-plane-rate-dependency"
	tests := []struct {
		name       string
		operation  ratelimit.Operation
		wantEvents int
		build      func(*testing.T, ratelimit.Limiter, *recordingInternalErrorSink) (*Server, *http.Request)
	}{
		{
			name: "app login", operation: ratelimit.OperationAppLogin,
			build: func(_ *testing.T, limiter ratelimit.Limiter, sink *recordingInternalErrorSink) (*Server, *http.Request) {
				server := newTestServer(ServerOptions{Capabilities: []capability.Manifest{authProviderTestManifest()}, RateLimiter: limiter, InternalErrorSink: sink})
				request := httptest.NewRequest(http.MethodPost, "/api/app/auth/login", bytes.NewBufferString(`{"username":"app-user"}`))
				request.Header.Set("Content-Type", "application/json")
				return server, request
			},
		},
		{
			name: "app upload", operation: ratelimit.OperationAppUpload,
			build: func(t *testing.T, limiter ratelimit.Limiter, sink *recordingInternalErrorSink) (*Server, *http.Request) {
				capabilities := capabilitiesFromConfigForTest(t, []string{"dictionary", "tenant", "identity", "session", "rbac", "menu", "audit", "parameter", "file-storage", "admin-shell"})
				server := newTestServer(ServerOptions{Capabilities: capabilities, RateLimiter: limiter, InternalErrorSink: sink})
				login := appLoginForTest(t, server, "upload-user")
				body, contentType := multipartUploadBody(t, "avatar.png", "content")
				request := httptest.NewRequest(http.MethodPost, "/api/app/files", body)
				request.Header.Set("Content-Type", contentType)
				request.Header.Set("Authorization", "Bearer "+login.Data.Token)
				return server, request
			},
		},
		{
			name: "phone verification", operation: ratelimit.OperationPhoneVerificationRequest,
			build: func(t *testing.T, limiter ratelimit.Limiter, sink *recordingInternalErrorSink) (*Server, *http.Request) {
				capabilities := capabilitiesFromConfigForTest(t, []string{"dictionary", "tenant", "identity", "session", "rbac", "audit", "app-phone"})
				server := newTestServer(ServerOptions{Capabilities: capabilities, RateLimiter: limiter, InternalErrorSink: sink})
				login := appLoginForTest(t, server, "phone-user")
				request := httptest.NewRequest(http.MethodPost, "/api/app/identity/phone-verifications", bytes.NewBufferString(`{"phone":"13800138000","purpose":"bind"}`))
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("Authorization", "Bearer "+login.Data.Token)
				return server, request
			},
		},
		{
			name: "phone binding", operation: ratelimit.OperationPhoneBindingVerification,
			build: func(t *testing.T, limiter ratelimit.Limiter, sink *recordingInternalErrorSink) (*Server, *http.Request) {
				capabilities := capabilitiesFromConfigForTest(t, []string{"dictionary", "tenant", "identity", "session", "rbac", "audit", "app-phone"})
				server := newTestServer(ServerOptions{Capabilities: capabilities, RateLimiter: limiter, InternalErrorSink: sink})
				login := appLoginForTest(t, server, "binding-user")
				request := httptest.NewRequest(http.MethodPost, "/api/app/identity/phone-bindings", bytes.NewBufferString(`{"phone":"13800138000","code":"123456"}`))
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("Authorization", "Bearer "+login.Data.Token)
				return server, request
			},
		},
		{
			name: "admin login", operation: ratelimit.OperationAdminLogin, wantEvents: 1,
			build: func(_ *testing.T, limiter ratelimit.Limiter, sink *recordingInternalErrorSink) (*Server, *http.Request) {
				server := newTestServer(ServerOptions{Capabilities: []capability.Manifest{authProviderTestManifest()}, RateLimiter: limiter, InternalErrorSink: sink})
				request := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"provider":"demo","username":"admin"}`))
				request.Header.Set("Content-Type", "application/json")
				return server, request
			},
		},
		{
			name: "sensitive reveal", operation: ratelimit.OperationSensitiveRevealChallenge, wantEvents: 1,
			build: func(t *testing.T, limiter ratelimit.Limiter, sink *recordingInternalErrorSink) (*Server, *http.Request) {
				harness := newSensitiveRevealHTTPHarness(t, sensitiveRevealHTTPHarnessOptions{limiter: limiter, internalErrorSink: sink})
				request := httptest.NewRequest(http.MethodPost, harness.fieldPath(harness.records["ops"], "/reveal/challenges"), bytes.NewBufferString(`{"purpose":"`+sensitiveRevealHTTPPurpose+`"}`))
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("Authorization", "Bearer "+harness.token)
				return harness.server, request
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sink := &recordingInternalErrorSink{}
			limiter := selectiveRateErrorLimiter{operation: test.operation, err: errors.New(marker)}
			server, request := test.build(t, limiter, sink)
			recorder := httptest.NewRecorder()

			server.Router().ServeHTTP(recorder, request)

			body := decodePlatformErrorResponse(t, recorder)
			definition, _ := errorcode.Lookup(errorcode.CodeRateLimitUnavailable)
			if recorder.Code != definition.HTTPStatus || body.Error.Code != definition.Code || body.Error.Message != definition.PublicMessage {
				t.Fatalf("status = %d error = %+v, want registered rate dependency", recorder.Code, body.Error)
			}
			if len(sink.events) != test.wantEvents {
				t.Fatalf("events = %+v, want %d", sink.events, test.wantEvents)
			}
			if test.wantEvents == 1 {
				assertRegisteredCauseEvent(t, sink, errorcode.CodeRateLimitUnavailable, body.Error)
			}
			if strings.Contains(recorder.Body.String(), marker) || strings.Contains(fmt.Sprintf("%+v", sink.events), marker) {
				t.Fatalf("rate dependency surface leaked cause: body=%s events=%+v", recorder.Body.String(), sink.events)
			}
		})
	}
}

type selectiveRateErrorLimiter struct {
	operation ratelimit.Operation
	err       error
}

func (l selectiveRateErrorLimiter) Allow(_ context.Context, key string, _ int, _ time.Duration) (ratelimit.Decision, error) {
	if strings.Contains(key, ":"+string(l.operation)+":") {
		return ratelimit.Decision{}, l.err
	}
	return ratelimit.Decision{Allowed: true}, nil
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

func TestAdminUnexpectedFailureMatrixUsesRegistryCauseWriterOnce(t *testing.T) {
	tests := []struct {
		name string
		code errorcode.Code
		run  func(*testing.T, *recordingInternalErrorSink, string) *httptest.ResponseRecorder
	}{
		{name: "provider start", code: errorcode.CodeAuthProviderStartFailed, run: adminProviderStartFailureRequest},
		{name: "provider resolve", code: errorcode.CodeAuthProviderResolveFailed, run: adminProviderResolveFailureRequest},
		{name: "identity binding", code: errorcode.CodeAuthIdentityBindingFailed, run: adminIdentityBindingFailureRequest},
		{name: "session issue", code: errorcode.CodeAuthSessionIssueFailed, run: adminSessionIssueFailureRequest},
		{name: "token sign", code: errorcode.CodeAuthTokenSignFailed, run: adminTokenSignFailureRequest},
		{name: "audit", code: errorcode.CodeAuthAuditFailed, run: adminAuditFailureRequest},
		{name: "renew", code: errorcode.CodeAuthSessionRenewFailed, run: adminRenewFailureRequest},
		{name: "revoke", code: errorcode.CodeAuthSessionRevokeFailed, run: adminRevokeFailureRequest},
		{name: "auth state refresh", code: errorcode.CodeAuthStateRefreshFailed, run: adminAuthStateRefreshFailureRequest},
		{name: "authorization state refresh", code: errorcode.CodeAdminAuthStateRefreshFailed, run: adminAuthorizationStateRefreshFailureRequest},
		{name: "menu resolution", code: errorcode.CodeAdminMenuResolutionFailed, run: adminMenuResolutionFailureRequest},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			marker := "private-" + strings.ReplaceAll(test.name, " ", "-") + "-cause"
			sink := &recordingInternalErrorSink{}
			recorder := test.run(t, sink, marker)
			assertRegisteredFailure(t, recorder, sink, test.code, marker, 1)
		})
	}
}

func TestAdminKnownAuthFailuresDoNotRecordInternalEvents(t *testing.T) {
	tests := []struct {
		name    string
		code    errorcode.Code
		request func(*testing.T, *Server) *http.Request
	}{
		{
			name: "invalid login request", code: errorcode.CodeAuthInvalidRequest,
			request: func(_ *testing.T, _ *Server) *http.Request {
				request := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader("{"))
				request.Header.Set("Content-Type", "application/json")
				return request
			},
		},
		{
			name: "invalid credentials", code: errorcode.CodeAuthInvalidCredentials,
			request: func(_ *testing.T, _ *Server) *http.Request {
				request := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"provider":"demo","username":"missing"}`))
				request.Header.Set("Content-Type", "application/json")
				return request
			},
		},
		{
			name: "unauthorized refresh", code: errorcode.CodeAuthUnauthorized,
			request: func(_ *testing.T, _ *Server) *http.Request {
				return httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sink := &recordingInternalErrorSink{}
			server := newTestServer(ServerOptions{Capabilities: []capability.Manifest{authProviderTestManifest()}, InternalErrorSink: sink})
			recorder := httptest.NewRecorder()
			server.Router().ServeHTTP(recorder, test.request(t, server))
			assertRegisteredFailure(t, recorder, sink, test.code, "", 0)
		})
	}
}

func TestAdminRateLimitDependencyFailureMatrix(t *testing.T) {
	t.Run("key builder", func(t *testing.T) {
		const marker = "private-rate\n-dimension"
		sink := &recordingInternalErrorSink{}
		server := newTestServer(ServerOptions{InternalErrorSink: sink})
		server.Router().GET("/rate-key-builder-test", func(ctx *gin.Context) {
			server.enforceAdminRateLimit(ctx, ratelimit.OperationAdminLogin, marker)
		})
		recorder := httptest.NewRecorder()
		server.Router().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/rate-key-builder-test", nil))
		assertRegisteredFailure(t, recorder, sink, errorcode.CodeRateLimitUnavailable, "private-rate", 1)
	})

	t.Run("limiter", func(t *testing.T) {
		const marker = "private-admin-limiter-cause"
		sink := &recordingInternalErrorSink{}
		builder, err := ratelimit.NewKeyBuilder([]byte(strings.Repeat("r", 32)))
		if err != nil {
			t.Fatal(err)
		}
		server := newTestServer(ServerOptions{
			Capabilities: []capability.Manifest{authProviderTestManifest()}, InternalErrorSink: sink,
			RateLimitKeyBuilder: builder, RateLimiter: selectiveRateErrorLimiter{operation: ratelimit.OperationAdminLogin, err: errors.New(marker)},
		})
		request := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"provider":"demo","username":"admin"}`))
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		server.Router().ServeHTTP(recorder, request)
		assertRegisteredFailure(t, recorder, sink, errorcode.CodeRateLimitUnavailable, marker, 1)
	})
}

func TestAdminRateLimitedRetryAfterRoundsUpAndHasMinimum(t *testing.T) {
	tests := []struct {
		name       string
		retryAfter time.Duration
		wantHeader string
	}{
		{name: "exact 90 seconds", retryAfter: 90 * time.Second, wantHeader: "90"},
		{name: "rounds upward", retryAfter: 1500 * time.Millisecond, wantHeader: "2"},
		{name: "minimum one", retryAfter: 0, wantHeader: "1"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sink := &recordingInternalErrorSink{}
			server := newTestServer(ServerOptions{
				Capabilities: []capability.Manifest{authProviderTestManifest()}, InternalErrorSink: sink,
				RateLimiter: &rateLimitTestStub{deny: ratelimit.OperationAdminLogin, retryAfter: test.retryAfter},
			})
			request := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"provider":"demo","username":"admin"}`))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			server.Router().ServeHTTP(recorder, request)
			assertRegisteredFailure(t, recorder, sink, errorcode.CodeRateLimited, "", 0)
			if recorder.Header().Get("Retry-After") != test.wantHeader {
				t.Fatalf("Retry-After = %q, want %q", recorder.Header().Get("Retry-After"), test.wantHeader)
			}
		})
	}
}

type authTokenServiceStub struct {
	delegate *authjwt.Service
	signErr  error
}

func (s authTokenServiceStub) Sign(authjwt.Subject, time.Duration) (string, time.Time, error) {
	return "", time.Time{}, s.signErr
}

func (s authTokenServiceStub) Parse(token string) (authjwt.Claims, error) {
	return s.delegate.Parse(token)
}

func adminProviderStartFailureRequest(t *testing.T, sink *recordingInternalErrorSink, marker string) *httptest.ResponseRecorder {
	t.Helper()
	server := newTestServer(ServerOptions{
		Capabilities: []capability.Manifest{adminOIDCProviderManifest(true, true)}, InternalErrorSink: sink,
		AdminIdentityResolver: adminIdentityResolverFunc{
			start: func(context.Context, AdminIdentityStartInput) (AdminIdentityStart, error) {
				return AdminIdentityStart{}, errors.New(marker)
			},
			resolve: func(context.Context, AdminIdentityResolveInput) (AdminIdentity, error) { return AdminIdentity{}, nil },
		},
	})
	request := httptest.NewRequest(http.MethodPost, "/api/auth/providers/oidc/start", bytes.NewBufferString(`{"codeChallenge":"47DEQpj8HBSa-_TImW-5JCeuQeRkm5NMpJWZG3hSuFU"}`))
	request.Header.Set("Content-Type", "application/json")
	return serveTestRequest(server, request)
}

func adminProviderResolveFailureRequest(t *testing.T, sink *recordingInternalErrorSink, marker string) *httptest.ResponseRecorder {
	t.Helper()
	server := newTestServer(ServerOptions{
		Capabilities: []capability.Manifest{adminOIDCProviderManifest(true, true)}, InternalErrorSink: sink,
		AdminIdentityResolver: adminIdentityResolverFunc{
			start: func(context.Context, AdminIdentityStartInput) (AdminIdentityStart, error) {
				return AdminIdentityStart{}, nil
			},
			resolve: func(context.Context, AdminIdentityResolveInput) (AdminIdentity, error) {
				return AdminIdentity{}, errors.New(marker)
			},
		},
	})
	return serveTestRequest(server, adminOIDCLoginRequest())
}

func adminIdentityBindingFailureRequest(t *testing.T, sink *recordingInternalErrorSink, marker string) *httptest.ResponseRecorder {
	t.Helper()
	server := newTestServer(ServerOptions{
		Capabilities: []capability.Manifest{adminOIDCProviderManifest(true, true)}, InternalErrorSink: sink,
		AdminIdentityResolver: adminIdentityResolverFunc{
			start: func(context.Context, AdminIdentityStartInput) (AdminIdentityStart, error) {
				return AdminIdentityStart{}, nil
			},
			resolve: func(context.Context, AdminIdentityResolveInput) (AdminIdentity, error) {
				return AdminIdentity{Issuer: "issuer", ProviderSubject: "subject"}, nil
			},
		},
		AdminIdentityBindings: adminIdentityBindingStoreFunc{resolve: func(context.Context, AdminIdentityBindingInput) (AdminIdentityBinding, error) {
			return AdminIdentityBinding{}, errors.New(marker)
		}},
	})
	return serveTestRequest(server, adminOIDCLoginRequest())
}

func adminSessionIssueFailureRequest(t *testing.T, sink *recordingInternalErrorSink, marker string) *httptest.ResponseRecorder {
	t.Helper()
	repository := newControllableSessionRepository()
	repository.createErr = errors.New(marker)
	sessions, err := session.NewRepositoryBackedStore(session.Options{TTL: time.Hour}, repository)
	if err != nil {
		t.Fatal(err)
	}
	server := newTestServer(ServerOptions{Capabilities: []capability.Manifest{authProviderTestManifest()}, Sessions: sessions, InternalErrorSink: sink})
	return serveTestRequest(server, adminDemoLoginRequest())
}

func adminTokenSignFailureRequest(t *testing.T, sink *recordingInternalErrorSink, marker string) *httptest.ResponseRecorder {
	t.Helper()
	delegate := authjwt.NewService("test-secret", time.Now)
	server := newTestServer(ServerOptions{
		Capabilities: []capability.Manifest{authProviderTestManifest()}, InternalErrorSink: sink,
		tokenService: authTokenServiceStub{delegate: delegate, signErr: errors.New(marker)},
	})
	return serveTestRequest(server, adminDemoLoginRequest())
}

func adminAuditFailureRequest(t *testing.T, sink *recordingInternalErrorSink, marker string) *httptest.ResponseRecorder {
	t.Helper()
	repository, resources := newRepositoryBackedAuthResources(t)
	server := newTestServer(ServerOptions{Capabilities: authResourceCapabilities(t), Resources: resources, InternalErrorSink: sink})
	repository.saveErr = errors.New(marker)
	return serveTestRequest(server, adminDemoLoginRequest())
}

func adminRenewFailureRequest(t *testing.T, sink *recordingInternalErrorSink, marker string) *httptest.ResponseRecorder {
	t.Helper()
	server, repository, token := newAdminSessionFailureServer(t, sink)
	repository.renewErr = errors.New(marker)
	request := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	return serveTestRequest(server, request)
}

func adminRevokeFailureRequest(t *testing.T, sink *recordingInternalErrorSink, marker string) *httptest.ResponseRecorder {
	t.Helper()
	server, repository, token := newAdminSessionFailureServer(t, sink)
	repository.revokeErr = errors.New(marker)
	request := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	return serveTestRequest(server, request)
}

func adminAuthStateRefreshFailureRequest(t *testing.T, sink *recordingInternalErrorSink, marker string) *httptest.ResponseRecorder {
	t.Helper()
	repository, resources := newRepositoryBackedAuthResources(t)
	server := newTestServer(ServerOptions{Capabilities: authResourceCapabilities(t), Resources: resources, InternalErrorSink: sink})
	repository.loadErr = errors.New(marker)
	return serveTestRequest(server, adminDemoLoginRequest())
}

func adminAuthorizationStateRefreshFailureRequest(t *testing.T, sink *recordingInternalErrorSink, marker string) *httptest.ResponseRecorder {
	t.Helper()
	repository, resources := newRepositoryBackedAuthResources(t)
	server := newTestServer(ServerOptions{Capabilities: authResourceCapabilities(t), Resources: resources, InternalErrorSink: sink})
	repository.loadErr = errors.New(marker)
	request := httptest.NewRequest(http.MethodGet, "/api/openapi.json", nil)
	request.Header.Set("X-Platform-User", "admin")
	return serveTestRequest(server, request)
}

func adminMenuResolutionFailureRequest(t *testing.T, sink *recordingInternalErrorSink, marker string) *httptest.ResponseRecorder {
	t.Helper()
	server := newTestServer(ServerOptions{
		Capabilities: []capability.Manifest{{ID: "tenant"}}, InternalErrorSink: sink,
		AdminMenuServingMode: AdminMenuServingModeTarget, AdminMenuResolver: &adminMenuResolverStub{revisionErr: errors.New(marker)},
	})
	request := httptest.NewRequest(http.MethodGet, "/api/admin/menus", nil)
	request.Header.Set("X-Platform-User", "ops")
	return serveTestRequest(server, request)
}

func adminOIDCLoginRequest() *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"provider":"oidc","code":"code","state":"state","codeVerifier":"verifier"}`))
	request.Header.Set("Content-Type", "application/json")
	return request
}

func adminDemoLoginRequest() *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"provider":"demo","username":"admin"}`))
	request.Header.Set("Content-Type", "application/json")
	return request
}

func authResourceCapabilities(t *testing.T) []capability.Manifest {
	t.Helper()
	return capabilitiesFromConfigForTest(t, []string{"dictionary", "tenant", "identity", "session", "rbac", "audit"})
}

func newRepositoryBackedAuthResources(t *testing.T) (*controllableAdminResourceRepository, *adminresource.Store) {
	t.Helper()
	repository := &controllableAdminResourceRepository{}
	resources, err := adminresource.NewRepositoryBackedStoreFromCapabilities(repository, authResourceCapabilities(t))
	if err != nil {
		t.Fatal(err)
	}
	return repository, resources
}

func newAdminSessionFailureServer(t *testing.T, sink *recordingInternalErrorSink) (*Server, *controllableSessionRepository, string) {
	t.Helper()
	repository := newControllableSessionRepository()
	sessions, err := session.NewRepositoryBackedStore(session.Options{TTL: time.Hour}, repository)
	if err != nil {
		t.Fatal(err)
	}
	server := newTestServer(ServerOptions{Capabilities: []capability.Manifest{authProviderTestManifest()}, Sessions: sessions, InternalErrorSink: sink})
	token := loginForTest(t, server, "ops").Data.Token
	return server, repository, token
}

func serveTestRequest(server *Server, request *http.Request) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	server.Router().ServeHTTP(recorder, request)
	return recorder
}

func assertRegisteredFailure(t *testing.T, recorder *httptest.ResponseRecorder, sink *recordingInternalErrorSink, code errorcode.Code, marker string, wantEvents int) {
	t.Helper()
	body := decodePlatformErrorResponse(t, recorder)
	definition, ok := errorcode.Lookup(code)
	if !ok {
		t.Fatalf("error code %s is not registered", code)
	}
	if recorder.Code != definition.HTTPStatus || body.Error.Code != code || body.Error.Message != definition.PublicMessage {
		t.Fatalf("status = %d error = %+v events = %+v, want %d/%s/%q", recorder.Code, body.Error, sink.events, definition.HTTPStatus, code, definition.PublicMessage)
	}
	if len(sink.events) != wantEvents {
		t.Fatalf("events = %+v, want %d", sink.events, wantEvents)
	}
	if wantEvents == 1 {
		assertRegisteredCauseEvent(t, sink, code, body.Error)
	}
	if marker != "" && (strings.Contains(recorder.Body.String(), marker) || strings.Contains(fmt.Sprintf("%+v", sink.events), marker)) {
		t.Fatalf("error surface leaked cause marker %q: body=%s events=%+v", marker, recorder.Body.String(), sink.events)
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
	if event.Code != code || event.RequestID != body.RequestID || event.TraceID != body.TraceID || event.Owner == "" {
		t.Fatalf("event = %+v body = %+v, want matching registered correlation", event, body)
	}
}
