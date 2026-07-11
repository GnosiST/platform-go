package bootstrap

import (
	"testing"

	"platform-go/internal/apps"
	"platform-go/internal/platform/capability"
	"platform-go/internal/platform/config"
)

func TestCapabilitiesFromConfigResolvesOnlyConfiguredCapabilities(t *testing.T) {
	manifests, err := CapabilitiesFromConfig(config.Config{
		Capabilities: []string{"dictionary", "tenant", "identity", "session", "rbac", "menu", "admin-shell"},
	})
	if err != nil {
		t.Fatalf("CapabilitiesFromConfig() error = %v", err)
	}
	ids := capabilityIDs(manifests)
	want := []capability.ID{"dictionary", "tenant", "identity", "session", "rbac", "menu", "admin-shell"}
	if !sameCapabilityIDs(ids, want) {
		t.Fatalf("capabilities = %+v, want %+v", ids, want)
	}
	if containsCapabilityID(ids, "wechat-login") || containsCapabilityID(ids, "demo-data") || containsCapabilityID(ids, "system-admin") {
		t.Fatalf("capabilities = %+v, want optional capabilities disabled", ids)
	}
}

func TestCapabilitiesFromConfigDoesNotEnableBusinessManifestsByDefault(t *testing.T) {
	t.Setenv("PLATFORM_CAPABILITIES", "")

	manifests, err := CapabilitiesFromConfig(config.Load(), apps.DefaultManifests()...)
	if err != nil {
		t.Fatalf("CapabilitiesFromConfig(default config) error = %v", err)
	}
	ids := capabilityIDs(manifests)
	if containsCapabilityID(ids, "external-business-capability") {
		t.Fatalf("capabilities = %+v, want no external business capability by default", ids)
	}
}

func TestCapabilitiesFromConfigRejectsMissingDependencies(t *testing.T) {
	_, err := CapabilitiesFromConfig(config.Config{Capabilities: []string{"session"}})
	if err == nil {
		t.Fatalf("CapabilitiesFromConfig() error = nil, want missing dependency")
	}
}

func TestCapabilitiesFromConfigMarksWechatProviderConfiguredWhenMiniAppCredentialsExist(t *testing.T) {
	manifests, err := CapabilitiesFromConfig(config.Config{
		Capabilities:        []string{"dictionary", "tenant", "identity", "session", "rbac", "audit", "wechat-login"},
		WechatMiniAppID:     "wx-app",
		WechatMiniAppSecret: "wx-secret",
	})
	if err != nil {
		t.Fatalf("CapabilitiesFromConfig() error = %v", err)
	}
	provider, ok := authProviderByID(manifests, "wechat")
	if !ok {
		t.Fatalf("wechat provider not found in manifests: %+v", manifests)
	}
	if !provider.Configured {
		t.Fatalf("wechat provider Configured = false, want true when miniapp credentials exist")
	}
	if !sameStrings(provider.ConfigKeys, []string{"PLATFORM_WECHAT_MINIAPP_APP_ID", "PLATFORM_WECHAT_MINIAPP_SECRET", "PLATFORM_WECHAT_MINIAPP_CODE2SESSION_ENDPOINT"}) {
		t.Fatalf("wechat provider ConfigKeys = %+v, want miniapp env keys", provider.ConfigKeys)
	}
}

func TestCapabilitiesFromConfigMarksAdminOIDCProviderConfigured(t *testing.T) {
	manifests, err := CapabilitiesFromConfig(config.Config{
		Capabilities:          []string{"dictionary", "tenant", "identity", "session", "rbac", "audit", "admin-oidc"},
		AdminOIDCIssuerURL:    "https://id.example/realms/platform",
		AdminOIDCClientID:     "platform-admin",
		AdminOIDCClientSecret: "client-secret",
		AdminOIDCRedirectURL:  "https://admin.example/login",
		AdminOIDCScopes:       []string{"openid", "profile", "email"},
	})
	if err != nil {
		t.Fatalf("CapabilitiesFromConfig() error = %v", err)
	}
	provider, ok := authProviderByID(manifests, "oidc")
	if !ok {
		t.Fatalf("oidc provider not found in manifests: %+v", manifests)
	}
	if !provider.Configured {
		t.Fatalf("oidc provider Configured = false, want true when admin OIDC config is complete")
	}
	if !provider.SupportsAudience(capability.AuthProviderAudienceAdmin) || provider.SupportsAudience(capability.AuthProviderAudienceApp) {
		t.Fatalf("oidc provider audiences = %+v, want admin only", provider.Audiences)
	}
}

func TestCapabilitiesFromConfigRejectsUnregisteredBusinessCapability(t *testing.T) {
	_, err := CapabilitiesFromConfig(config.Config{
		Capabilities: []string{
			"tenant",
			"identity",
			"session",
			"rbac",
			"menu",
			"audit",
			"dictionary",
			"admin-shell",
			"external-ordering",
		},
	})
	if err == nil {
		t.Fatalf("CapabilitiesFromConfig() error = nil, want unknown capability")
	}
}

func capabilityIDs(manifests []capability.Manifest) []capability.ID {
	ids := make([]capability.ID, 0, len(manifests))
	for _, manifest := range manifests {
		ids = append(ids, manifest.ID)
	}
	return ids
}

func sameCapabilityIDs(got []capability.ID, want []capability.ID) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range got {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}

func containsCapabilityID(ids []capability.ID, target capability.ID) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}

func authProviderByID(manifests []capability.Manifest, id string) (capability.AuthProvider, bool) {
	for _, manifest := range manifests {
		for _, provider := range manifest.AuthProviders {
			if provider.ID == id {
				return provider, true
			}
		}
	}
	return capability.AuthProvider{}, false
}

func sameStrings(got []string, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range got {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}
