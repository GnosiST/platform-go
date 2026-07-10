package capability

import (
	"fmt"
	"strings"
)

func ValidateAuthProviderDeclarations(manifests []Manifest) error {
	providers := map[string]ID{}
	for _, manifest := range manifests {
		for _, provider := range manifest.AuthProviders {
			providerID := strings.TrimSpace(provider.ID)
			kind := strings.TrimSpace(provider.Kind)
			if providerID == "" {
				return fmt.Errorf("capability %q auth provider id is required", manifest.ID)
			}
			if !validAuthProviderSegment(providerID) {
				return fmt.Errorf("capability %q auth provider %q id must use lowercase letters, numbers or hyphens", manifest.ID, provider.ID)
			}
			if kind == "" {
				return fmt.Errorf("capability %q auth provider %q kind is required", manifest.ID, providerID)
			}
			if !validAuthProviderSegment(kind) {
				return fmt.Errorf("capability %q auth provider %q kind must use lowercase letters, numbers or hyphens", manifest.ID, providerID)
			}
			if !hasLocalizedText(provider.Title) {
				return fmt.Errorf("capability %q auth provider %q title is required", manifest.ID, providerID)
			}
			if !hasLocalizedText(provider.Description) {
				return fmt.Errorf("capability %q auth provider %q description is required", manifest.ID, providerID)
			}
			if err := validateAuthProviderConfigKeys(manifest.ID, providerID, provider.ConfigKeys); err != nil {
				return err
			}
			if owner, exists := providers[providerID]; exists {
				return fmt.Errorf("capability %q auth provider %q already registered by capability %q", manifest.ID, providerID, owner)
			}
			providers[providerID] = manifest.ID
		}
	}
	return nil
}

func validAuthProviderSegment(segment string) bool {
	if segment == "" {
		return false
	}
	for _, char := range segment {
		if char >= 'a' && char <= 'z' {
			continue
		}
		if char >= '0' && char <= '9' {
			continue
		}
		if char == '-' {
			continue
		}
		return false
	}
	return true
}

func validateAuthProviderConfigKeys(owner ID, providerID string, configKeys []string) error {
	seen := map[string]struct{}{}
	for _, configKey := range configKeys {
		key := strings.TrimSpace(configKey)
		if key == "" {
			return fmt.Errorf("capability %q auth provider %q config key is required", owner, providerID)
		}
		if !validAuthProviderConfigKey(key) {
			return fmt.Errorf("capability %q auth provider %q config key %q must use uppercase letters, numbers or underscores and start with a letter", owner, providerID, configKey)
		}
		if _, exists := seen[key]; exists {
			return fmt.Errorf("capability %q auth provider %q duplicate config key %q", owner, providerID, key)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func validAuthProviderConfigKey(key string) bool {
	if key == "" {
		return false
	}
	for index, char := range key {
		if char >= 'A' && char <= 'Z' {
			continue
		}
		if index > 0 && char >= '0' && char <= '9' {
			continue
		}
		if index > 0 && char == '_' {
			continue
		}
		return false
	}
	return true
}
