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

	"github.com/GnosiST/platform-go/internal/platform/adminresource"
	"github.com/GnosiST/platform-go/internal/platform/authjwt"
	"github.com/GnosiST/platform-go/internal/platform/capability"
	"github.com/GnosiST/platform-go/internal/platform/errorcode"
	"github.com/GnosiST/platform-go/internal/platform/session"
)

func TestAppRegistryOwnedFunctionsDoNotUseLegacyErrorWriters(t *testing.T) {
	targetsByFile := map[string]map[string]bool{
		"app_routes.go": {"withAppRoutePolicy": true},
		"app_phone.go": {
			"appPhoneCreateVerification": true, "appPhoneCreateBinding": true,
			"appPhoneBindingExists": true, "validAppPhoneVerification": true,
		},
		"server.go": {
			"appAuthLogin": true, "resolveAppLoginIdentity": true, "appAuthLogout": true,
			"appCurrentSession": true, "restoreAdminFile": true, "adminFileUpload": true,
			"adminFileContent": true, "appFileUpload": true, "appFileContent": true,
			"appSessionFromBearer": true,
		},
	}
	legacy := map[string]bool{
		"writeAuthError": true, "writeForbidden": true, "writeUnauthorized": true,
		"writeFileError": true, "writeUploadPolicyError": true, "legacyErrorBody": true,
		"recordInternalError": true, "refreshResourceStateLegacy": true,
	}
	for filename, targets := range targetsByFile {
		file, err := parser.ParseFile(token.NewFileSet(), filename, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		visited := map[string]bool{}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || !targets[function.Name.Name] {
				continue
			}
			visited[function.Name.Name] = true
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
		for target := range targets {
			if !visited[target] {
				t.Errorf("%s configured target %s was not visited", filename, target)
			}
		}
	}
}

func TestAppAuthDependencyFailureMatrixUsesRegistryCauseWriterOnce(t *testing.T) {
	tests := []struct {
		name string
		code errorcode.Code
		run  func(*testing.T, *recordingInternalErrorSink, string) *httptest.ResponseRecorder
	}{
		{name: "provider resolve", code: errorcode.CodeAppAuthProviderResolveFailed, run: appProviderResolveFailureRequest},
		{name: "identity binding", code: errorcode.CodeAppAuthIdentityBindingFailed, run: appIdentityBindingFailureRequest},
		{name: "session issue", code: errorcode.CodeAppAuthSessionIssueFailed, run: appSessionIssueFailureRequest},
		{name: "token sign", code: errorcode.CodeAppAuthTokenSignFailed, run: appTokenSignFailureRequest},
		{name: "issued session cleanup", code: errorcode.CodeAppAuthSessionCleanupFailed, run: appIssuedSessionCleanupFailureRequest},
		{name: "login audit", code: errorcode.CodeAppAuthAuditFailed, run: appLoginAuditFailureRequest},
		{name: "login audit cleanup", code: errorcode.CodeAppAuthSessionCleanupFailed, run: appLoginAuditCleanupFailureRequest},
		{name: "logout resolve", code: errorcode.CodeAppAuthSessionRevokeFailed, run: appLogoutResolveFailureRequest},
		{name: "logout audit", code: errorcode.CodeAppAuthAuditFailed, run: appLogoutAuditFailureRequest},
		{name: "logout revoke", code: errorcode.CodeAppAuthSessionRevokeFailed, run: appLogoutRevokeFailureRequest},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			marker := "private-app-auth-" + strings.ReplaceAll(test.name, " ", "-")
			sink := &recordingInternalErrorSink{}
			recorder := test.run(t, sink, marker)
			assertRegisteredFailure(t, recorder, sink, test.code, marker, 1)
		})
	}
}

func TestAppPhoneDependencyFailureMatrixUsesRegistryCauseWriterOnce(t *testing.T) {
	tests := []struct {
		name        string
		code        errorcode.Code
		causeMarker string
		run         func(*testing.T, *recordingInternalErrorSink, string) *httptest.ResponseRecorder
	}{
		{name: "verification dependencies", code: errorcode.CodeAppPhoneVerificationUnavailable, causeMarker: "app phone verification dependency is not configured", run: appPhoneVerificationDependenciesMissingRequest},
		{name: "verification phone digest", code: errorcode.CodeAppPhoneVerificationUnavailable, run: appPhoneDigestFailureRequest},
		{name: "verification code digest", code: errorcode.CodeAppPhoneVerificationUnavailable, run: appCodeDigestFailureRequest},
		{name: "random generation", code: errorcode.CodeAppPhoneCodeGenerationFailed, run: appPhoneCodeGenerationFailureRequest},
		{name: "delivery", code: errorcode.CodeAppPhoneVerificationDeliveryFailed, run: appPhoneDeliveryFailureRequest},
		{name: "verification create", code: errorcode.CodeAppPhoneVerificationCreateFailed, run: appPhoneVerificationCreateFailureRequest},
		{name: "binding protector", code: errorcode.CodeAppPhoneVerificationUnavailable, causeMarker: "app phone verification protector is not configured", run: appPhoneBindingProtectorMissingRequest},
		{name: "binding phone digest", code: errorcode.CodeAppPhoneVerificationUnavailable, run: appPhoneBindingDigestFailureRequest},
		{name: "binding code digest", code: errorcode.CodeAppPhoneVerificationUnavailable, run: appPhoneBindingCodeDigestFailureRequest},
		{name: "binding lookup", code: errorcode.CodeAppPhoneVerificationUnavailable, run: appPhoneBindingLookupFailureRequest},
		{name: "verification lookup", code: errorcode.CodeAppPhoneVerificationUnavailable, run: appPhoneVerificationLookupFailureRequest},
		{name: "binding create", code: errorcode.CodeAppPhoneBindingCreateFailed, run: appPhoneBindingCreateFailureRequest},
		{name: "verification update", code: errorcode.CodeAppPhoneVerificationUpdateFailed, run: appPhoneVerificationUpdateFailureRequest},
		{name: "audit", code: errorcode.CodeAppPhoneAuditFailed, run: appPhoneAuditFailureRequest},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			marker := "private-app-phone-" + strings.ReplaceAll(test.name, " ", "-")
			if test.causeMarker != "" {
				marker = test.causeMarker
			}
			sink := &recordingInternalErrorSink{}
			recorder := test.run(t, sink, marker)
			assertRegisteredFailure(t, recorder, sink, test.code, marker, 1)
		})
	}
}

func TestAppPhoneInvalidJSONUsesCanonicalRegistryMessage(t *testing.T) {
	capabilities := capabilitiesFromConfigForTest(t, []string{"dictionary", "tenant", "identity", "session", "rbac", "audit", "app-phone"})
	server := newTestServer(ServerOptions{Capabilities: capabilities})
	login := appLoginForTest(t, server, "phone-user")
	for _, path := range []string{"/api/app/identity/phone-verifications", "/api/app/identity/phone-bindings"} {
		request := httptest.NewRequest(http.MethodPost, path, strings.NewReader("{"))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Authorization", "Bearer "+login.Data.Token)
		recorder := serveTestRequest(server, request)
		body := decodePlatformErrorResponse(t, recorder)
		if recorder.Code != http.StatusBadRequest || body.Error.Code != errorcode.CodeAppPhoneInvalidRequest || body.Error.Message != "invalid app phone request" {
			t.Fatalf("POST %s status/error = %d/%+v, want canonical APP_PHONE_INVALID_REQUEST", path, recorder.Code, body.Error)
		}
	}
}

func appProviderResolveFailureRequest(t *testing.T, sink *recordingInternalErrorSink, marker string) *httptest.ResponseRecorder {
	t.Helper()
	server := newTestServer(ServerOptions{
		Capabilities: []capability.Manifest{configuredWechatAuthProviderManifest()}, InternalErrorSink: sink,
		AppIdentityResolver: appIdentityResolverFunc(func(context.Context, AppIdentityResolveInput) (AppIdentity, error) {
			return AppIdentity{}, errors.New(marker)
		}),
	})
	request := httptest.NewRequest(http.MethodPost, "/api/app/auth/login", bytes.NewBufferString(`{"provider":"wechat","code":"code"}`))
	request.Header.Set("Content-Type", "application/json")
	return serveTestRequest(server, request)
}

func appSessionIssueFailureRequest(t *testing.T, sink *recordingInternalErrorSink, marker string) *httptest.ResponseRecorder {
	t.Helper()
	repository := newControllableSessionRepository()
	repository.createErr = errors.New(marker)
	sessions, err := session.NewRepositoryBackedStore(session.Options{TTL: time.Hour}, repository)
	if err != nil {
		t.Fatal(err)
	}
	server := newTestServer(ServerOptions{Capabilities: []capability.Manifest{authProviderTestManifest()}, Sessions: sessions, InternalErrorSink: sink})
	request := httptest.NewRequest(http.MethodPost, "/api/app/auth/login", bytes.NewBufferString(`{"username":"buyer"}`))
	request.Header.Set("Content-Type", "application/json")
	return serveTestRequest(server, request)
}

type appIdentityBindingStoreFunc func(context.Context, AppIdentityBindingInput) (AppIdentityBinding, error)

func (f appIdentityBindingStoreFunc) ResolveAppIdentityBinding(ctx context.Context, input AppIdentityBindingInput) (AppIdentityBinding, error) {
	return f(ctx, input)
}

func appIdentityBindingFailureRequest(t *testing.T, sink *recordingInternalErrorSink, marker string) *httptest.ResponseRecorder {
	t.Helper()
	server := newTestServer(ServerOptions{
		Capabilities: []capability.Manifest{configuredWechatAuthProviderManifest()}, InternalErrorSink: sink,
		AppIdentityResolver: appIdentityResolverFunc(func(context.Context, AppIdentityResolveInput) (AppIdentity, error) {
			return AppIdentity{ProviderSubject: "provider-subject"}, nil
		}),
		AppIdentityBindings: appIdentityBindingStoreFunc(func(context.Context, AppIdentityBindingInput) (AppIdentityBinding, error) {
			return AppIdentityBinding{}, errors.New(marker)
		}),
	})
	request := httptest.NewRequest(http.MethodPost, "/api/app/auth/login", bytes.NewBufferString(`{"provider":"wechat","code":"code"}`))
	request.Header.Set("Content-Type", "application/json")
	return serveTestRequest(server, request)
}

func appTokenSignFailureRequest(t *testing.T, sink *recordingInternalErrorSink, marker string) *httptest.ResponseRecorder {
	t.Helper()
	server := newTestServer(ServerOptions{
		Capabilities: []capability.Manifest{authProviderTestManifest()}, InternalErrorSink: sink,
		tokenService: authTokenServiceStub{delegate: authjwt.NewService("test-secret", time.Now), signErr: errors.New(marker)},
	})
	return serveTestRequest(server, appDemoLoginRequest())
}

func appIssuedSessionCleanupFailureRequest(t *testing.T, sink *recordingInternalErrorSink, marker string) *httptest.ResponseRecorder {
	t.Helper()
	repository := newControllableSessionRepository()
	repository.revokeErr = errors.New(marker)
	sessions, err := session.NewRepositoryBackedStore(session.Options{TTL: time.Hour}, repository)
	if err != nil {
		t.Fatal(err)
	}
	server := newTestServer(ServerOptions{
		Capabilities: []capability.Manifest{authProviderTestManifest()}, Sessions: sessions, InternalErrorSink: sink,
		tokenService: authTokenServiceStub{delegate: authjwt.NewService("test-secret", time.Now), signErr: errors.New("private-token-sign-cause")},
	})
	recorder := serveTestRequest(server, appDemoLoginRequest())
	if strings.Contains(recorder.Body.String(), "private-token-sign-cause") || strings.Contains(fmt.Sprintf("%+v", sink.events), "private-token-sign-cause") {
		t.Fatalf("issued-session cleanup surface leaked original sign cause: body=%s events=%+v", recorder.Body.String(), sink.events)
	}
	return recorder
}

func appLoginAuditFailureRequest(t *testing.T, sink *recordingInternalErrorSink, marker string) *httptest.ResponseRecorder {
	t.Helper()
	repository, resources := newRepositoryBackedAuthResources(t)
	server := newTestServer(ServerOptions{Capabilities: authResourceCapabilities(t), Resources: resources, InternalErrorSink: sink})
	repository.saveErr = errors.New(marker)
	return serveTestRequest(server, appDemoLoginRequest())
}

func appLoginAuditCleanupFailureRequest(t *testing.T, sink *recordingInternalErrorSink, marker string) *httptest.ResponseRecorder {
	t.Helper()
	const auditMarker = "private-app-login-audit-cause"
	repository, resources := newRepositoryBackedAuthResources(t)
	sessionRepository := newControllableSessionRepository()
	sessionRepository.revokeErr = errors.New(marker)
	sessions, err := session.NewRepositoryBackedStore(session.Options{TTL: time.Hour}, sessionRepository)
	if err != nil {
		t.Fatal(err)
	}
	server := newTestServer(ServerOptions{Capabilities: authResourceCapabilities(t), Resources: resources, Sessions: sessions, InternalErrorSink: sink})
	repository.saveErr = errors.New(auditMarker)
	recorder := serveTestRequest(server, appDemoLoginRequest())
	if strings.Contains(recorder.Body.String(), auditMarker) || strings.Contains(fmt.Sprintf("%+v", sink.events), auditMarker) {
		t.Fatalf("login audit cleanup surface leaked original audit cause: body=%s events=%+v", recorder.Body.String(), sink.events)
	}
	return recorder
}

func appLogoutResolveFailureRequest(t *testing.T, sink *recordingInternalErrorSink, marker string) *httptest.ResponseRecorder {
	t.Helper()
	repository := newControllableSessionRepository()
	sessions, err := session.NewRepositoryBackedStore(session.Options{TTL: time.Hour}, repository)
	if err != nil {
		t.Fatal(err)
	}
	server := newTestServer(ServerOptions{Capabilities: []capability.Manifest{authProviderTestManifest()}, Sessions: sessions, InternalErrorSink: sink})
	token := appLoginForTest(t, server, "buyer").Data.Token
	repository.resolveErr = errors.New(marker)
	return performBearerRequest(server, http.MethodPost, "/api/app/auth/logout", token)
}

func appLogoutAuditFailureRequest(t *testing.T, sink *recordingInternalErrorSink, marker string) *httptest.ResponseRecorder {
	t.Helper()
	repository, resources := newRepositoryBackedAuthResources(t)
	server := newTestServer(ServerOptions{Capabilities: authResourceCapabilities(t), Resources: resources, InternalErrorSink: sink})
	token := appLoginForTest(t, server, "buyer").Data.Token
	repository.saveErr = errors.New(marker)
	return performBearerRequest(server, http.MethodPost, "/api/app/auth/logout", token)
}

func appLogoutRevokeFailureRequest(t *testing.T, sink *recordingInternalErrorSink, marker string) *httptest.ResponseRecorder {
	t.Helper()
	repository := newControllableSessionRepository()
	sessions, err := session.NewRepositoryBackedStore(session.Options{TTL: time.Hour}, repository)
	if err != nil {
		t.Fatal(err)
	}
	server := newTestServer(ServerOptions{Capabilities: []capability.Manifest{authProviderTestManifest()}, Sessions: sessions, InternalErrorSink: sink})
	token := appLoginForTest(t, server, "buyer").Data.Token
	repository.revokeErr = errors.New(marker)
	return performBearerRequest(server, http.MethodPost, "/api/app/auth/logout", token)
}

func appDemoLoginRequest() *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/api/app/auth/login", bytes.NewBufferString(`{"username":"buyer"}`))
	request.Header.Set("Content-Type", "application/json")
	return request
}

func appPhoneVerificationDependenciesMissingRequest(t *testing.T, sink *recordingInternalErrorSink, _ string) *httptest.ResponseRecorder {
	t.Helper()
	server, token := newAppPhoneFailureServerWithoutDefaults(t, sink, ServerOptions{})
	return serveTestRequest(server, newAppPhoneVerificationHTTPRequest(token))
}

func appPhoneDigestFailureRequest(t *testing.T, sink *recordingInternalErrorSink, marker string) *httptest.ResponseRecorder {
	t.Helper()
	server, token := newAppPhoneFailureServer(t, sink, ServerOptions{
		PhoneProtector: phoneProtectorTestStub{
			phoneDigest: func(string) (string, error) { return "", errors.New(marker) },
			codeDigest:  func(string, string, string) (string, error) { return "", errors.New("unexpected code digest call") },
		},
	})
	return serveTestRequest(server, newAppPhoneVerificationHTTPRequest(token))
}

func appCodeDigestFailureRequest(t *testing.T, sink *recordingInternalErrorSink, marker string) *httptest.ResponseRecorder {
	t.Helper()
	server, token := newAppPhoneFailureServer(t, sink, ServerOptions{
		PhoneProtector: phoneProtectorTestStub{
			phoneDigest: func(string) (string, error) { return "phone-digest", nil },
			codeDigest:  func(string, string, string) (string, error) { return "", errors.New(marker) },
		},
	})
	return serveTestRequest(server, newAppPhoneVerificationHTTPRequest(token))
}

func appPhoneCodeGenerationFailureRequest(t *testing.T, sink *recordingInternalErrorSink, marker string) *httptest.ResponseRecorder {
	t.Helper()
	server, token := newAppPhoneFailureServer(t, sink, ServerOptions{
		appPhoneCodeGenerator: func() (string, error) { return "", errors.New(marker) },
	})
	return serveTestRequest(server, newAppPhoneVerificationHTTPRequest(token))
}

func appPhoneDeliveryFailureRequest(t *testing.T, sink *recordingInternalErrorSink, marker string) *httptest.ResponseRecorder {
	t.Helper()
	server, token := newAppPhoneFailureServer(t, sink, ServerOptions{
		PhoneVerificationSender: phoneVerificationSenderTestStub{
			kind: "sms-vendor",
			send: func(context.Context, string, string, string) error { return errors.New(marker) },
		},
	})
	return serveTestRequest(server, newAppPhoneVerificationHTTPRequest(token))
}

func appPhoneVerificationCreateFailureRequest(t *testing.T, sink *recordingInternalErrorSink, marker string) *httptest.ResponseRecorder {
	t.Helper()
	repository, server, token := newRepositoryBackedAppPhoneFailureServer(t, sink)
	repository.armSave(errors.New(marker), 1)
	return serveTestRequest(server, newAppPhoneVerificationHTTPRequest(token))
}

func appPhoneBindingProtectorMissingRequest(t *testing.T, sink *recordingInternalErrorSink, _ string) *httptest.ResponseRecorder {
	t.Helper()
	server, token := newAppPhoneFailureServer(t, sink, ServerOptions{})
	verification := requestAppPhoneVerificationForTest(t, server, token)
	server.phoneProtector = nil
	return serveTestRequest(server, newAppPhoneBindingHTTPRequest(token, verification.Data.DebugCode))
}

func appPhoneBindingDigestFailureRequest(t *testing.T, sink *recordingInternalErrorSink, marker string) *httptest.ResponseRecorder {
	t.Helper()
	server, token := newAppPhoneFailureServer(t, sink, ServerOptions{})
	verification := requestAppPhoneVerificationForTest(t, server, token)
	server.phoneProtector = phoneProtectorTestStub{
		phoneDigest: func(string) (string, error) { return "", errors.New(marker) },
		codeDigest:  func(string, string, string) (string, error) { return "", errors.New("unexpected code digest call") },
	}
	return serveTestRequest(server, newAppPhoneBindingHTTPRequest(token, verification.Data.DebugCode))
}

func appPhoneBindingCodeDigestFailureRequest(t *testing.T, sink *recordingInternalErrorSink, marker string) *httptest.ResponseRecorder {
	t.Helper()
	server, token := newAppPhoneFailureServer(t, sink, ServerOptions{})
	verification := requestAppPhoneVerificationForTest(t, server, token)
	server.phoneProtector = phoneProtectorTestStub{
		phoneDigest: func(string) (string, error) { return "phone-digest", nil },
		codeDigest:  func(string, string, string) (string, error) { return "", errors.New(marker) },
	}
	return serveTestRequest(server, newAppPhoneBindingHTTPRequest(token, verification.Data.DebugCode))
}

func appPhoneBindingLookupFailureRequest(t *testing.T, sink *recordingInternalErrorSink, marker string) *httptest.ResponseRecorder {
	return appPhoneRepositoryLookupFailureRequest(t, sink, marker, 1)
}

func appPhoneVerificationLookupFailureRequest(t *testing.T, sink *recordingInternalErrorSink, marker string) *httptest.ResponseRecorder {
	return appPhoneRepositoryLookupFailureRequest(t, sink, marker, 2)
}

func appPhoneRepositoryLookupFailureRequest(t *testing.T, sink *recordingInternalErrorSink, marker string, failAt int) *httptest.ResponseRecorder {
	t.Helper()
	repository, server, token := newRepositoryBackedAppPhoneFailureServer(t, sink)
	verification := requestAppPhoneVerificationForTest(t, server, token)
	repository.arm(errors.New(marker), failAt)
	return serveTestRequest(server, newAppPhoneBindingHTTPRequest(token, verification.Data.DebugCode))
}

func appPhoneBindingCreateFailureRequest(t *testing.T, sink *recordingInternalErrorSink, marker string) *httptest.ResponseRecorder {
	return appPhoneBindingMutationFailureRequest(t, sink, marker, 1)
}

func appPhoneVerificationUpdateFailureRequest(t *testing.T, sink *recordingInternalErrorSink, marker string) *httptest.ResponseRecorder {
	return appPhoneBindingMutationFailureRequest(t, sink, marker, 2)
}

func appPhoneAuditFailureRequest(t *testing.T, sink *recordingInternalErrorSink, marker string) *httptest.ResponseRecorder {
	return appPhoneBindingMutationFailureRequest(t, sink, marker, 3)
}

func appPhoneBindingMutationFailureRequest(t *testing.T, sink *recordingInternalErrorSink, marker string, failAt int) *httptest.ResponseRecorder {
	t.Helper()
	repository, server, token := newRepositoryBackedAppPhoneFailureServer(t, sink)
	verification := requestAppPhoneVerificationForTest(t, server, token)
	repository.armSave(errors.New(marker), failAt)
	request := httptest.NewRequest(http.MethodPost, "/api/app/identity/phone-bindings", bytes.NewBufferString(`{"phone":"13800138000","code":"`+verification.Data.DebugCode+`"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	return serveTestRequest(server, request)
}

func newAppPhoneFailureServer(t *testing.T, sink *recordingInternalErrorSink, overrides ServerOptions) (*Server, string) {
	t.Helper()
	overrides.Capabilities = appPhoneFailureCapabilities(t)
	overrides.InternalErrorSink = sink
	server := newTestServer(overrides)
	return server, appLoginForTest(t, server, "phone-user").Data.Token
}

func newAppPhoneFailureServerWithoutDefaults(t *testing.T, sink *recordingInternalErrorSink, options ServerOptions) (*Server, string) {
	t.Helper()
	options.Capabilities = appPhoneFailureCapabilities(t)
	options.InternalErrorSink = sink
	options.AllowInsecureHeaderAuth = true
	server := New(options)
	return server, appLoginForTest(t, server, "phone-user").Data.Token
}

func newRepositoryBackedAppPhoneFailureServer(t *testing.T, sink *recordingInternalErrorSink) (*armedAdminResourceRepository, *Server, string) {
	t.Helper()
	capabilities := appPhoneFailureCapabilities(t)
	repository := &armedAdminResourceRepository{}
	resources, err := adminresource.NewRepositoryBackedStoreFromCapabilities(repository, capabilities)
	if err != nil {
		t.Fatal(err)
	}
	server := newTestServer(ServerOptions{Capabilities: capabilities, Resources: resources, InternalErrorSink: sink, DebugCodeEnabled: true})
	return repository, server, appLoginForTest(t, server, "phone-user").Data.Token
}

func appPhoneFailureCapabilities(t *testing.T) []capability.Manifest {
	t.Helper()
	return capabilitiesFromConfigForTest(t, []string{"dictionary", "tenant", "identity", "session", "rbac", "audit", "app-phone"})
}

func newAppPhoneVerificationHTTPRequest(token string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/api/app/identity/phone-verifications", bytes.NewBufferString(`{"phone":"13800138000","purpose":"bind"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	return request
}

func newAppPhoneBindingHTTPRequest(token string, code string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/api/app/identity/phone-bindings", bytes.NewBufferString(`{"phone":"13800138000","code":"`+code+`"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	return request
}

type armedAdminResourceRepository struct {
	controllableAdminResourceRepository
	armed       bool
	loadCalls   int
	failAt      int
	loadFailure error
	saveArmed   bool
	saveCalls   int
	saveFailAt  int
	saveFailure error
}

func (r *armedAdminResourceRepository) Save(ctx context.Context, snapshot adminresource.ResourceSnapshot) (uint64, error) {
	if r.saveArmed {
		r.saveCalls++
		if r.saveCalls == r.saveFailAt {
			return 0, r.saveFailure
		}
	}
	return r.controllableAdminResourceRepository.Save(ctx, snapshot)
}

func (r *armedAdminResourceRepository) Load(ctx context.Context) (adminresource.ResourceSnapshot, error) {
	if r.armed {
		r.loadCalls++
		if r.loadCalls == r.failAt {
			return adminresource.ResourceSnapshot{}, r.loadFailure
		}
	}
	return r.controllableAdminResourceRepository.Load(ctx)
}

func (r *armedAdminResourceRepository) arm(err error, failAt int) {
	r.armed = true
	r.loadCalls = 0
	r.failAt = failAt
	r.loadFailure = err
}

func (r *armedAdminResourceRepository) armSave(err error, failAt int) {
	r.saveArmed = true
	r.saveCalls = 0
	r.saveFailAt = failAt
	r.saveFailure = err
}

func requestAppPhoneVerificationForTest(t *testing.T, server *Server, token string) appPhoneVerificationTestPayload {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/app/identity/phone-verifications", bytes.NewBufferString(`{"phone":"13800138000","purpose":"bind"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := serveTestRequest(server, request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("create phone verification status/body = %d/%s", recorder.Code, recorder.Body.String())
	}
	var payload appPhoneVerificationTestPayload
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	return payload
}
