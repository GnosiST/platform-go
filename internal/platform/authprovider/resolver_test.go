package authprovider

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestAdminIdentityResolverFromConfigReturnsOIDCResolverWhenConfigured(t *testing.T) {
	var providerServer *httptest.Server
	providerServer = httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/.well-known/openid-configuration" {
			http.NotFound(response, request)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]any{
			"issuer":                                providerServer.URL,
			"authorization_endpoint":                providerServer.URL + "/authorize",
			"token_endpoint":                        providerServer.URL + "/token",
			"jwks_uri":                              providerServer.URL + "/keys",
			"response_types_supported":              []string{"code"},
			"subject_types_supported":               []string{"public"},
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	}))
	defer providerServer.Close()

	resolver, err := AdminIdentityResolverFromConfig(config.Config{
		JWTSecret:             "0123456789abcdef0123456789abcdef",
		AdminOIDCIssuerURL:    providerServer.URL,
		AdminOIDCClientID:     "platform-admin",
		AdminOIDCClientSecret: "fixture-client-secret",
		AdminOIDCRedirectURL:  "https://admin.example/auth/oidc/callback",
		AdminOIDCScopes:       []string{"openid", "profile", "email"},
	})
	if err != nil {
		t.Fatalf("AdminIdentityResolverFromConfig() error = %v", err)
	}
	if resolver == nil {
		t.Fatalf("AdminIdentityResolverFromConfig() resolver = nil, want configured resolver")
	}
}

func TestAdminIdentityResolverFromConfigReturnsNilWhenOIDCConfigMissing(t *testing.T) {
	resolver, err := AdminIdentityResolverFromConfig(config.Config{
		JWTSecret:          "0123456789abcdef0123456789abcdef",
		AdminOIDCIssuerURL: "https://issuer.example",
	})
	if err != nil {
		t.Fatalf("AdminIdentityResolverFromConfig() error = %v, want nil resolver without complete config", err)
	}
	if resolver != nil {
		t.Fatalf("AdminIdentityResolverFromConfig() resolver = %T, want nil without complete config", resolver)
	}
}

func TestAdminIdentityResolverFromConfigNormalizesAndRedactsDiscoveryFailure(t *testing.T) {
	providerServer := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusBadGateway)
		_, _ = response.Write([]byte("raw-discovery-response-marker"))
	}))
	defer providerServer.Close()

	_, err := AdminIdentityResolverFromConfig(config.Config{
		JWTSecret:             "0123456789abcdef0123456789abcdef",
		AdminOIDCIssuerURL:    providerServer.URL,
		AdminOIDCClientID:     "platform-admin",
		AdminOIDCClientSecret: "fixture-client-secret",
		AdminOIDCRedirectURL:  "https://admin.example/auth/oidc/callback",
		AdminOIDCScopes:       []string{"openid"},
	})
	if !errors.Is(err, httpapi.ErrAdminIdentityProviderExchange) {
		t.Fatalf("AdminIdentityResolverFromConfig() error did not normalize to provider exchange failure")
	}
	if strings.Contains(err.Error(), "raw-discovery-response-marker") || strings.Contains(err.Error(), "fixture-client-secret") {
		t.Fatalf("AdminIdentityResolverFromConfig() error exposed sensitive fixture data")
	}
}
