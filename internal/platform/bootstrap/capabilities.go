package bootstrap

import (
	"fmt"
	"reflect"
	"strings"

	"platform-go/internal/platform/capability"
	"platform-go/internal/platform/config"
	"platform-go/internal/platform/core"
	"platform-go/internal/platform/httpapi"
	"platform-go/internal/platform/ratelimit"
)

type PhoneVerificationRuntime struct {
	Protector        httpapi.PhoneProtector
	Sender           httpapi.PhoneVerificationSender
	DebugCodeEnabled bool
}

type RateLimitRuntime struct {
	Limiter    ratelimit.Limiter
	KeyBuilder *ratelimit.KeyBuilder
}

const developmentRateLimitHMACKey = "dev-platform-rate-limit-key-00001"

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

func RateLimitRuntimeFromConfig(cfg config.Config) (RateLimitRuntime, error) {
	environment := strings.ToLower(strings.TrimSpace(cfg.RuntimeEnvironment))
	if environment == "" {
		environment = config.RuntimeEnvironmentDevelopment
	}
	key := cfg.RateLimitHMACKey
	if key == "" && (environment == config.RuntimeEnvironmentDevelopment || environment == config.RuntimeEnvironmentTest) {
		key = developmentRateLimitHMACKey
	}
	if environment == config.RuntimeEnvironmentProduction {
		if len([]byte(key)) < 32 {
			return RateLimitRuntime{}, fmt.Errorf("production rate limit HMAC key must be at least 32 bytes")
		}
		if key == cfg.PhoneHMACKey || key == cfg.PhoneCodeHMACKey {
			return RateLimitRuntime{}, fmt.Errorf("production rate limit HMAC key must be distinct from phone and code HMAC keys")
		}
	}
	keyBuilder, err := ratelimit.NewKeyBuilder([]byte(key))
	if err != nil {
		return RateLimitRuntime{}, fmt.Errorf("build rate limit key builder: %w", err)
	}
	switch cfg.CacheDriver {
	case "redis":
		options := redisOptionsFromConfig(cfg)
		return RateLimitRuntime{
			Limiter: ratelimit.NewRedisLimiter(ratelimit.RedisOptions{
				Addr:     options.Addr,
				Password: options.Password,
				DB:       options.DB,
			}),
			KeyBuilder: keyBuilder,
		}, nil
	case "", "memory":
		if environment == config.RuntimeEnvironmentProduction {
			return RateLimitRuntime{}, fmt.Errorf("production rate limiting requires Redis")
		}
		return RateLimitRuntime{Limiter: ratelimit.NewMemoryLimiter(ratelimit.MemoryOptions{}), KeyBuilder: keyBuilder}, nil
	default:
		return RateLimitRuntime{}, fmt.Errorf("unsupported rate limit backend %q", cfg.CacheDriver)
	}
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
	rawProvider := cfg.PhoneVerificationProvider
	provider := strings.ToLower(strings.TrimSpace(rawProvider))
	if rawProvider != provider {
		return PhoneVerificationRuntime{}, fmt.Errorf("phone verification provider must be canonical trimmed lowercase")
	}
	if provider == "" {
		return PhoneVerificationRuntime{}, fmt.Errorf("phone verification provider is required")
	}
	if provider == "unknown" {
		return PhoneVerificationRuntime{}, fmt.Errorf("unsupported phone verification provider %q", provider)
	}
	environment := strings.ToLower(strings.TrimSpace(cfg.RuntimeEnvironment))
	if environment == "" {
		environment = config.RuntimeEnvironmentDevelopment
	}
	if phoneVerificationSenderNil(sender) {
		return PhoneVerificationRuntime{}, fmt.Errorf("unsupported phone verification provider %q: no sender is registered", provider)
	}
	debugCodeEnabled := false
	if provider == httpapi.PhoneVerificationProviderDebug {
		if environment != config.RuntimeEnvironmentDevelopment && environment != config.RuntimeEnvironmentTest {
			return PhoneVerificationRuntime{}, fmt.Errorf("phone verification debug provider is not allowed in %s", environment)
		}
		debugSender, ok := sender.(*httpapi.DebugPhoneVerificationSender)
		if !ok || debugSender == nil {
			return PhoneVerificationRuntime{}, fmt.Errorf("phone verification debug provider requires the built-in debug sender")
		}
		debugCodeEnabled = true
	}
	actualProvider := sender.Kind()
	if actualProvider != provider {
		return PhoneVerificationRuntime{}, fmt.Errorf("phone verification sender %q does not match configured provider %q", actualProvider, provider)
	}
	return PhoneVerificationRuntime{Protector: protector, Sender: sender, DebugCodeEnabled: debugCodeEnabled}, nil
}

func phoneVerificationSenderNil(sender httpapi.PhoneVerificationSender) bool {
	if sender == nil {
		return true
	}
	value := reflect.ValueOf(sender)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func configuredCapability(capabilities []string, target string) bool {
	for _, id := range capabilities {
		if strings.TrimSpace(id) == target {
			return true
		}
	}
	return false
}
