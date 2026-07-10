package wechat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"platform-go/internal/platform/capability"
	"platform-go/internal/platform/httpapi"
)

func TestResolverExchangesCodeForTrustedWechatIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		query := request.URL.Query()
		if query.Get("appid") != "wx-app" || query.Get("secret") != "wx-secret" || query.Get("js_code") != "wx-code" {
			t.Fatalf("code2session query = %s, want configured credentials and code", request.URL.RawQuery)
		}
		if query.Get("grant_type") != "authorization_code" {
			t.Fatalf("grant_type = %q, want authorization_code", query.Get("grant_type"))
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"openid":"openid-from-wechat","unionid":"union-from-wechat","session_key":"secret-session"}`))
	}))
	defer server.Close()
	resolver, err := NewResolver(Config{
		AppID:    "wx-app",
		Secret:   "wx-secret",
		Endpoint: server.URL,
	})
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}

	identity, err := resolver.ResolveAppIdentity(context.Background(), httpapi.AppIdentityResolveInput{
		Provider:     wechatProviderForTest(),
		Code:         " wx-code ",
		UsernameHint: "client-supplied-name",
	})

	if err != nil {
		t.Fatalf("ResolveAppIdentity() error = %v", err)
	}
	if identity.ProviderSubject != "openid-from-wechat" || identity.ProviderUnionSubject != "union-from-wechat" {
		t.Fatalf("identity = %+v, want trusted openid/unionid", identity)
	}
	if identity.Username != "" {
		t.Fatalf("identity.Username = %q, want adapter to leave business username mapping to binding layer", identity.Username)
	}
}

func TestResolverReturnsInvalidIdentityForEmptyOpenID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"session_key":"secret-session"}`))
	}))
	defer server.Close()
	resolver, err := NewResolver(Config{AppID: "wx-app", Secret: "wx-secret", Endpoint: server.URL})
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}

	_, err = resolver.ResolveAppIdentity(context.Background(), httpapi.AppIdentityResolveInput{
		Provider: wechatProviderForTest(),
		Code:     "wx-code",
	})

	if !errors.Is(err, httpapi.ErrAppIdentityInvalid) {
		t.Fatalf("ResolveAppIdentity() error = %v, want ErrAppIdentityInvalid", err)
	}
}

func TestResolverNormalizesProviderErrorForWechatErrorCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"errcode":40029,"errmsg":"invalid code openid-secret wx-secret bad-code"}`))
	}))
	defer server.Close()
	resolver, err := NewResolver(Config{AppID: "wx-app", Secret: "wx-secret", Endpoint: server.URL})
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}

	_, err = resolver.ResolveAppIdentity(context.Background(), httpapi.AppIdentityResolveInput{
		Provider: wechatProviderForTest(),
		Code:     "bad-code",
	})

	if !errors.Is(err, ErrProviderResolveFailed) {
		t.Fatalf("ResolveAppIdentity() error = %v, want ErrProviderResolveFailed", err)
	}
	for _, leaked := range []string{"invalid code", "openid-secret", "wx-secret", "bad-code"} {
		if strings.Contains(err.Error(), leaked) {
			t.Fatalf("ResolveAppIdentity() error leaked %q: %v", leaked, err)
		}
	}
	if errors.Is(err, httpapi.ErrAppIdentityInvalid) {
		t.Fatalf("ResolveAppIdentity() error = %v, want provider exchange error, not invalid identity", err)
	}
}

func TestNewResolverRequiresMiniAppCredentials(t *testing.T) {
	_, err := NewResolver(Config{AppID: "wx-app"})
	if err == nil {
		t.Fatalf("NewResolver() error = nil, want missing credentials error")
	}
}

func TestResolverBacksConfiguredHTTPAppLoginWithoutLeakingProviderSubjects(t *testing.T) {
	codeServer := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"openid":"openid-secret-from-wechat","unionid":"union-secret-from-wechat"}`))
	}))
	defer codeServer.Close()
	resolver, err := NewResolver(Config{AppID: "wx-app", Secret: "wx-secret", Endpoint: codeServer.URL})
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}
	server := httpapi.New(httpapi.ServerOptions{
		Capabilities:        []capability.Manifest{wechatLoginManifestForTest()},
		AppIdentityResolver: resolver,
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/app/auth/login", bytes.NewBufferString(`{"provider":"wechat","code":"wx-code"}`))
	request.Header.Set("Content-Type", "application/json")

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("POST app auth login status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if strings.Contains(body, "openid-secret-from-wechat") || strings.Contains(body, "union-secret-from-wechat") {
		t.Fatalf("app auth login response leaked provider subject: %s", body)
	}
	var payload struct {
		Data struct {
			Session struct {
				Username string `json:"username"`
				TenantID string `json:"tenantId"`
			} `json:"session"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode app auth login: %v body = %s", err, body)
	}
	if !strings.HasPrefix(payload.Data.Session.Username, "guest-wechat-") || payload.Data.Session.TenantID != "app" {
		t.Fatalf("app auth session = %+v, want provider-bound app identity", payload.Data.Session)
	}
}

func wechatProviderForTest() capability.AuthProvider {
	return capability.AuthProvider{
		ID:         "wechat",
		Kind:       "wechat",
		Enabled:    true,
		Configured: true,
	}
}

func wechatLoginManifestForTest() capability.Manifest {
	return capability.Manifest{
		ID: "wechat-login-test",
		App: capability.AppSurface{Routes: []capability.AppRoute{
			{
				Method:      http.MethodPost,
				Path:        "/api/app/auth/login",
				Auth:        capability.AppRouteAuthPublic,
				Description: capability.Text("App 登录。", "App login."),
			},
		}},
		AuthProviders: []capability.AuthProvider{wechatProviderForTest()},
	}
}
