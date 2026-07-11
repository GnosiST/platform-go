package bootstrap

import (
	"fmt"
	"strings"

	"platform-go/internal/platform/capability"
	"platform-go/internal/platform/config"
	"platform-go/internal/platform/core"
	"platform-go/internal/platform/httpapi"
)

type PhoneVerificationRuntime struct {
	Protector httpapi.PhoneProtector
	Sender    httpapi.PhoneVerificationSender
}

func CapabilitiesFromConfig(cfg config.Config, additionalManifests ...capability.Manifest) ([]capability.Manifest, error) {
	registry := capability.NewRegistry()
	for _, manifest := range core.DefaultManifests() {
		manifest = configureAuthProvidersFromConfig(manifest, cfg)
		if err := registry.Register(manifest); err != nil {
			return nil, err
		}
	}
	for _, manifest := range additionalManifests {
		if err := registry.Register(manifest); err != nil {
			return nil, err
		}
	}
	enabled := make([]capability.ID, 0, len(cfg.Capabilities))
	for _, id := range cfg.Capabilities {
		enabled = append(enabled, capability.ID(id))
	}
	return registry.ResolveEnabled(enabled)
}

func configureAuthProvidersFromConfig(manifest capability.Manifest, cfg config.Config) capability.Manifest {
	for index := range manifest.AuthProviders {
		provider := &manifest.AuthProviders[index]
		if provider.ID == "wechat" && wechatMiniAppConfigured(cfg) {
			provider.Configured = true
		}
		if provider.ID == "oidc" && cfg.AdminOIDCConfigured() {
			provider.Configured = true
		}
	}
	return manifest
}

func wechatMiniAppConfigured(cfg config.Config) bool {
	return strings.TrimSpace(cfg.WechatMiniAppID) != "" && strings.TrimSpace(cfg.WechatMiniAppSecret) != ""
}

func PhoneVerificationRuntimeFromConfig(cfg config.Config, sender httpapi.PhoneVerificationSender) (PhoneVerificationRuntime, error) {
	if !configuredCapability(cfg.Capabilities, "app-phone") {
		return PhoneVerificationRuntime{}, nil
	}
	protector := httpapi.NewHMACPhoneProtector([]byte(cfg.PhoneHMACKey), []byte(cfg.PhoneCodeHMACKey))
	phoneDigest, err := protector.PhoneDigest("000000")
	if err != nil {
		return PhoneVerificationRuntime{}, fmt.Errorf("build phone protector: %w", err)
	}
	if _, err := protector.CodeDigest(phoneDigest, "bootstrap", "000000"); err != nil {
		return PhoneVerificationRuntime{}, fmt.Errorf("build phone protector: %w", err)
	}
	provider := strings.ToLower(strings.TrimSpace(cfg.PhoneVerificationProvider))
	environment := strings.ToLower(strings.TrimSpace(cfg.RuntimeEnvironment))
	if environment == "" {
		environment = config.RuntimeEnvironmentDevelopment
	}
	if provider == httpapi.PhoneVerificationProviderDebug {
		if environment != config.RuntimeEnvironmentDevelopment && environment != config.RuntimeEnvironmentTest {
			return PhoneVerificationRuntime{}, fmt.Errorf("phone verification debug provider is not allowed in %s", environment)
		}
		if sender == nil {
			sender = httpapi.NewDebugPhoneVerificationSender()
		}
	} else if sender == nil {
		return PhoneVerificationRuntime{}, fmt.Errorf("unsupported phone verification provider %q: no sender is registered", provider)
	}
	if provider == "" {
		return PhoneVerificationRuntime{}, fmt.Errorf("phone verification provider is required")
	}
	actualProvider := strings.ToLower(strings.TrimSpace(sender.Kind()))
	if actualProvider != provider {
		return PhoneVerificationRuntime{}, fmt.Errorf("phone verification sender %q does not match configured provider %q", actualProvider, provider)
	}
	return PhoneVerificationRuntime{Protector: protector, Sender: sender}, nil
}

func configuredCapability(capabilities []string, target string) bool {
	for _, id := range capabilities {
		if strings.TrimSpace(id) == target {
			return true
		}
	}
	return false
}
