package bootstrap

import (
	"fmt"
	"strings"

	"github.com/GnosiST/platform-go/internal/platform/config"
	"github.com/GnosiST/platform-go/internal/platform/dataprotection"
)

func DataProtectionRuntimeFromConfig(cfg config.Config) (dataprotection.Runtime, error) {
	environment := strings.ToLower(strings.TrimSpace(cfg.RuntimeEnvironment))
	if environment == "" {
		environment = config.RuntimeEnvironmentDevelopment
	}
	rawProvider := cfg.DataKeyProvider
	providerKind := strings.ToLower(strings.TrimSpace(rawProvider))
	if providerKind == "" {
		if environment == config.RuntimeEnvironmentProduction {
			return nil, fmt.Errorf("production runtime requires PLATFORM_DATA_KEY_PROVIDER=env-aes256")
		}
		if strings.TrimSpace(cfg.DataEncryptionActiveKeyID) != "" || strings.TrimSpace(cfg.DataEncryptionKeyringJSON) != "" ||
			strings.TrimSpace(cfg.DataBlindIndexActiveKeyID) != "" || strings.TrimSpace(cfg.DataBlindIndexKeyringJSON) != "" {
			return nil, fmt.Errorf("data protection keys require PLATFORM_DATA_KEY_PROVIDER")
		}
		return nil, nil
	}
	if rawProvider != providerKind {
		return nil, fmt.Errorf("data key provider must be canonical trimmed lowercase")
	}
	switch providerKind {
	case dataprotection.ProviderEnvAES256:
	case dataprotection.ProviderLocalTest:
		if environment != config.RuntimeEnvironmentDevelopment && environment != config.RuntimeEnvironmentTest {
			return nil, fmt.Errorf("data key provider local-test is not allowed in %s", environment)
		}
	default:
		return nil, fmt.Errorf("unsupported data key provider %q", providerKind)
	}
	if environment == config.RuntimeEnvironmentProduction && providerKind != dataprotection.ProviderEnvAES256 {
		return nil, fmt.Errorf("production runtime requires PLATFORM_DATA_KEY_PROVIDER=env-aes256")
	}
	encryptionKeys, err := dataprotection.ParseEncodedKeyring(cfg.DataEncryptionKeyringJSON)
	if err != nil {
		return nil, fmt.Errorf("data encryption keyring: %w", err)
	}
	blindIndexKeys, err := dataprotection.ParseEncodedKeyring(cfg.DataBlindIndexKeyringJSON)
	if err != nil {
		return nil, fmt.Errorf("data blind-index keyring: %w", err)
	}
	provider, err := dataprotection.NewStaticKeyProvider(dataprotection.StaticKeyProviderConfig{
		Kind:                  providerKind,
		ActiveEncryptionKeyID: cfg.DataEncryptionActiveKeyID, EncryptionKeys: encryptionKeys,
		ActiveBlindIndexKeyID: cfg.DataBlindIndexActiveKeyID, BlindIndexKeys: blindIndexKeys,
	})
	if err != nil {
		return nil, fmt.Errorf("data protection provider: %w", err)
	}
	return dataprotection.NewRuntime(provider), nil
}
