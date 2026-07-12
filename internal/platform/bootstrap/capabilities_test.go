package bootstrap

import (
	"context"
	"errors"
	"strings"
	"testing"

	"platform-go/internal/apps"
	"platform-go/internal/platform/capability"
	"platform-go/internal/platform/config"
	"platform-go/internal/platform/httpapi"
	"platform-go/internal/platform/ratelimit"
)

type phoneVerificationSenderStub struct {
	kind string
	err  error
}

func (s phoneVerificationSenderStub) Send(context.Context, string, string, string) error {
	return s.err
}

func (s phoneVerificationSenderStub) Kind() string {
	return s.kind
}

type pointerPhoneVerificationSenderStub struct {
	kind string
}

func (*pointerPhoneVerificationSenderStub) Send(context.Context, string, string, string) error {
	return nil
}

func (s *pointerPhoneVerificationSenderStub) Kind() string {
	if s == nil {
		return "sms-vendor"
	}
	return s.kind
}

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

func TestRateLimitRuntimeFromConfigUsesRedisInProduction(t *testing.T) {
	runtime, err := RateLimitRuntimeFromConfig(config.Config{
		RuntimeEnvironment: config.RuntimeEnvironmentProduction,
		CacheDriver:        "redis",
		RedisAddr:          "127.0.0.1:6379",
		RateLimitHMACKey:   strings.Repeat("r", 32),
	})
	if err != nil {
		t.Fatalf("RateLimitRuntimeFromConfig() error = %v", err)
	}
	if _, ok := runtime.Limiter.(*ratelimit.RedisLimiter); !ok || runtime.KeyBuilder == nil {
		t.Fatalf("rate limit runtime = %+v, want Redis limiter and key builder", runtime)
	}
}

func TestRateLimitRuntimeFromConfigRejectsProcessLocalProductionLimiter(t *testing.T) {
	for _, driver := range []string{"", "memory"} {
		t.Run(driver, func(t *testing.T) {
			_, err := RateLimitRuntimeFromConfig(config.Config{
				RuntimeEnvironment: config.RuntimeEnvironmentProduction,
				CacheDriver:        driver,
				RateLimitHMACKey:   strings.Repeat("r", 32),
			})
			if err == nil || !strings.Contains(err.Error(), "production rate limiting requires Redis") {
				t.Fatalf("RateLimitRuntimeFromConfig(%q) error = %v, want Redis requirement", driver, err)
			}
		})
	}
}

func TestRateLimitRuntimeFromConfigRejectsUnsafeProductionHMACKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{name: "missing", key: ""},
		{name: "short", key: "short"},
		{name: "phone duplicate", key: strings.Repeat("p", 32)},
		{name: "code duplicate", key: strings.Repeat("c", 32)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := RateLimitRuntimeFromConfig(config.Config{
				RuntimeEnvironment: config.RuntimeEnvironmentProduction,
				CacheDriver:        "redis",
				RateLimitHMACKey:   tt.key,
				PhoneHMACKey:       strings.Repeat("p", 32),
				PhoneCodeHMACKey:   strings.Repeat("c", 32),
			})
			if err == nil {
				t.Fatal("RateLimitRuntimeFromConfig() error = nil, want unsafe production key rejection")
			}
		})
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

func TestPhoneVerificationRuntimeFromConfigComposesExplicitDevelopmentDebugSender(t *testing.T) {
	runtime, err := PhoneVerificationRuntimeFromConfig(config.Config{
		RuntimeEnvironment:        config.RuntimeEnvironmentDevelopment,
		Capabilities:              []string{"app-phone"},
		PhoneHMACKey:              strings.Repeat("p", 32),
		PhoneCodeHMACKey:          strings.Repeat("c", 32),
		PhoneVerificationProvider: "debug",
	}, httpapi.NewDebugPhoneVerificationSender())
	if err != nil {
		t.Fatalf("PhoneVerificationRuntimeFromConfig() error = %v", err)
	}
	if runtime.Protector == nil || runtime.Sender == nil || runtime.Sender.Kind() != httpapi.PhoneVerificationProviderDebug || !runtime.DebugCodeEnabled {
		t.Fatalf("phone verification runtime = %+v, want protector, debug sender, and immutable debug disclosure", runtime)
	}
}

func TestPhoneVerificationRuntimeFromConfigDoesNotImplicitlyCreateDebugSender(t *testing.T) {
	_, err := PhoneVerificationRuntimeFromConfig(config.Config{
		RuntimeEnvironment:        config.RuntimeEnvironmentDevelopment,
		Capabilities:              []string{"app-phone"},
		PhoneHMACKey:              strings.Repeat("p", 32),
		PhoneCodeHMACKey:          strings.Repeat("c", 32),
		PhoneVerificationProvider: "debug",
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "no sender is registered") {
		t.Fatalf("PhoneVerificationRuntimeFromConfig() error = %v, want explicit debug sender requirement", err)
	}
}

func TestPhoneVerificationRuntimeFromConfigRejectsUnsupportedConfiguredProvider(t *testing.T) {
	for _, environment := range []string{config.RuntimeEnvironmentDevelopment, config.RuntimeEnvironmentProduction} {
		t.Run(environment, func(t *testing.T) {
			_, err := PhoneVerificationRuntimeFromConfig(config.Config{
				RuntimeEnvironment:        environment,
				Capabilities:              []string{"app-phone"},
				PhoneHMACKey:              strings.Repeat("p", 32),
				PhoneCodeHMACKey:          strings.Repeat("c", 32),
				PhoneVerificationProvider: "sms-vendor",
			}, nil)
			if err == nil || !strings.Contains(err.Error(), "unsupported phone verification provider") {
				t.Fatalf("PhoneVerificationRuntimeFromConfig() error = %v, want unsupported provider", err)
			}
		})
	}
}

func TestPhoneVerificationRuntimeFromConfigRequiresMatchingInjectedSender(t *testing.T) {
	cfg := config.Config{
		RuntimeEnvironment:        config.RuntimeEnvironmentProduction,
		Capabilities:              []string{"app-phone"},
		PhoneHMACKey:              strings.Repeat("p", 32),
		PhoneCodeHMACKey:          strings.Repeat("c", 32),
		PhoneVerificationProvider: "sms-vendor",
	}
	if _, err := PhoneVerificationRuntimeFromConfig(cfg, phoneVerificationSenderStub{kind: "other-vendor"}); err == nil || !strings.Contains(err.Error(), "does not match configured provider") {
		t.Fatalf("PhoneVerificationRuntimeFromConfig(mismatch) error = %v", err)
	}
	if _, err := PhoneVerificationRuntimeFromConfig(cfg, phoneVerificationSenderStub{kind: " SMS-VENDOR "}); err == nil || !strings.Contains(err.Error(), "does not match configured provider") {
		t.Fatalf("PhoneVerificationRuntimeFromConfig(non-canonical sender) error = %v, want exact provider match", err)
	}
	runtime, err := PhoneVerificationRuntimeFromConfig(cfg, phoneVerificationSenderStub{kind: "sms-vendor"})
	if err != nil || runtime.DebugCodeEnabled {
		t.Fatalf("PhoneVerificationRuntimeFromConfig(match) runtime=%+v error=%v, want non-debug injected runtime", runtime, err)
	}
}

func TestPhoneVerificationRuntimeFromConfigRejectsNonCanonicalProviderAndTypedNilSender(t *testing.T) {
	base := config.Config{
		RuntimeEnvironment:        config.RuntimeEnvironmentProduction,
		Capabilities:              []string{"app-phone"},
		PhoneHMACKey:              strings.Repeat("p", 32),
		PhoneCodeHMACKey:          strings.Repeat("c", 32),
		PhoneVerificationProvider: " SMS-VENDOR ",
	}
	if _, err := PhoneVerificationRuntimeFromConfig(base, phoneVerificationSenderStub{kind: "sms-vendor"}); err == nil || !strings.Contains(err.Error(), "canonical trimmed lowercase") {
		t.Fatalf("PhoneVerificationRuntimeFromConfig(non-canonical provider) error = %v", err)
	}
	base.PhoneVerificationProvider = "sms-vendor"
	var sender *pointerPhoneVerificationSenderStub
	if _, err := PhoneVerificationRuntimeFromConfig(base, sender); err == nil || !strings.Contains(err.Error(), "no sender is registered") {
		t.Fatalf("PhoneVerificationRuntimeFromConfig(typed nil) error = %v", err)
	}
	base.PhoneVerificationProvider = "unknown"
	if _, err := PhoneVerificationRuntimeFromConfig(base, phoneVerificationSenderStub{kind: "unknown"}); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("PhoneVerificationRuntimeFromConfig(unknown provider) error = %v", err)
	}
}

func TestPhoneVerificationRuntimeFromConfigRejectsFailingSenderKind(t *testing.T) {
	_, err := PhoneVerificationRuntimeFromConfig(config.Config{
		RuntimeEnvironment:        config.RuntimeEnvironmentProduction,
		Capabilities:              []string{"app-phone"},
		PhoneHMACKey:              strings.Repeat("p", 32),
		PhoneCodeHMACKey:          strings.Repeat("c", 32),
		PhoneVerificationProvider: "sms-vendor",
	}, phoneVerificationSenderStub{kind: "sms-vendor", err: errors.New("delivery unavailable")})
	if err != nil {
		t.Fatalf("PhoneVerificationRuntimeFromConfig() error = %v, sender delivery is not a bootstrap probe", err)
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
