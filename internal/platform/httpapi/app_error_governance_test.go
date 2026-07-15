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

	"platform-go/internal/platform/adminresource"
	"platform-go/internal/platform/capability"
	"platform-go/internal/platform/errorcode"
	"platform-go/internal/platform/session"
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

func TestAppAuthDependencyFailureMatrixUsesRegistryCauseWriterOnce(t *testing.T) {
	tests := []struct {
		name string
		code errorcode.Code
		run  func(*testing.T, *recordingInternalErrorSink, string) *httptest.ResponseRecorder
	}{
		{name: "provider resolve", code: errorcode.CodeAppAuthProviderResolveFailed, run: appProviderResolveFailureRequest},
		{name: "session issue", code: errorcode.CodeAppAuthSessionIssueFailed, run: appSessionIssueFailureRequest},
		{name: "logout resolve", code: errorcode.CodeAppAuthSessionRevokeFailed, run: appLogoutResolveFailureRequest},
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

func TestAppPhoneRepositoryLookupFailuresUseRegisteredUnavailableCause(t *testing.T) {
	for _, failAt := range []int{1, 2} {
		t.Run(fmt.Sprintf("lookup_%d", failAt), func(t *testing.T) {
			const marker = "private-phone-repository-marker"
			capabilities := capabilitiesFromConfigForTest(t, []string{"dictionary", "tenant", "identity", "session", "rbac", "audit", "app-phone"})
			repository := &armedAdminResourceRepository{}
			resources, err := adminresource.NewRepositoryBackedStoreFromCapabilities(repository, capabilities)
			if err != nil {
				t.Fatal(err)
			}
			sink := &recordingInternalErrorSink{}
			server := newTestServer(ServerOptions{Capabilities: capabilities, Resources: resources, InternalErrorSink: sink, DebugCodeEnabled: true})
			login := appLoginForTest(t, server, "phone-user")
			verification := requestAppPhoneVerificationForTest(t, server, login.Data.Token)
			repository.arm(errors.New(marker), failAt)
			request := httptest.NewRequest(http.MethodPost, "/api/app/identity/phone-bindings", bytes.NewBufferString(`{"phone":"13800138000","code":"`+verification.Data.DebugCode+`"}`))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Authorization", "Bearer "+login.Data.Token)
			recorder := serveTestRequest(server, request)
			assertRegisteredFailure(t, recorder, sink, errorcode.CodeAppPhoneVerificationUnavailable, marker, 1)
		})
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

type armedAdminResourceRepository struct {
	controllableAdminResourceRepository
	armed       bool
	loadCalls   int
	failAt      int
	loadFailure error
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
