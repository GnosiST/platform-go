package bootstrap

import (
	"strings"

	"platform-go/internal/platform/capability"
	"platform-go/internal/platform/config"
	"platform-go/internal/platform/core"
)

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
