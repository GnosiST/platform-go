package authprovider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"platform-go/internal/platform/capability"
	"platform-go/internal/platform/config"
	"platform-go/internal/platform/httpapi"
)

func TestAppIdentityResolverFromConfigReturnsWechatResolverWhenConfigured(t *testing.T) {
	codeServer := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"openid":"openid-from-factory"}`))
	}))
	defer codeServer.Close()
	resolver, err := AppIdentityResolverFromConfig(config.Config{
		WechatMiniAppID:                   "wx-app",
		WechatMiniAppSecret:               "wx-secret",
		WechatMiniAppCode2SessionEndpoint: codeServer.URL,
	})
	if err != nil {
		t.Fatalf("AppIdentityResolverFromConfig() error = %v", err)
	}
	if resolver == nil {
		t.Fatalf("AppIdentityResolverFromConfig() resolver = nil, want configured resolver")
	}
	identity, err := resolver.ResolveAppIdentity(context.Background(), httpapi.AppIdentityResolveInput{
		Provider: capability.AuthProvider{ID: "wechat", Kind: "wechat", Enabled: true, Configured: true},
		Code:     "wx-code",
	})
	if err != nil {
		t.Fatalf("ResolveAppIdentity() error = %v", err)
	}
	if identity.ProviderSubject != "openid-from-factory" {
		t.Fatalf("ProviderSubject = %q, want factory-backed openid", identity.ProviderSubject)
	}
}

func TestAppIdentityResolverFromConfigReturnsNilWhenWechatCredentialsMissing(t *testing.T) {
	resolver, err := AppIdentityResolverFromConfig(config.Config{WechatMiniAppID: "wx-app"})
	if err != nil {
		t.Fatalf("AppIdentityResolverFromConfig() error = %v, want nil resolver without credentials", err)
	}
	if resolver != nil {
		t.Fatalf("AppIdentityResolverFromConfig() resolver = %T, want nil without complete credentials", resolver)
	}
}
