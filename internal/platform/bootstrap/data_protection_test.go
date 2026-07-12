package bootstrap

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"platform-go/internal/platform/config"
	"platform-go/internal/platform/dataprotection"
)

func TestDataProtectionRuntimeFromConfigIsOptionalWhenUnconfiguredInDevelopment(t *testing.T) {
	runtime, err := DataProtectionRuntimeFromConfig(config.Config{RuntimeEnvironment: config.RuntimeEnvironmentDevelopment})
	if err != nil {
		t.Fatalf("DataProtectionRuntimeFromConfig() error = %v", err)
	}
	if runtime != nil {
		t.Fatalf("runtime = %T, want nil", runtime)
	}
}

func TestDataProtectionRuntimeFromConfigBuildsExplicitProvider(t *testing.T) {
	runtime, err := DataProtectionRuntimeFromConfig(dataProtectionConfigForTest(config.RuntimeEnvironmentProduction, dataprotection.ProviderEnvAES256))
	if err != nil {
		t.Fatalf("DataProtectionRuntimeFromConfig() error = %v", err)
	}
	policy := dataprotection.FieldPolicy{Format: dataprotection.FormatAES256GCMV1, Normalization: dataprotection.NormalizationTrimV1, BlindIndexNamespace: "custom-reference"}
	fieldContext := dataprotection.FieldContext{TenantID: dataprotection.GlobalTenantID, Resource: "custom", RecordID: "custom-1", FieldKey: "reference", SchemaVersion: 1}
	envelope, err := runtime.Protect(context.Background(), " marker ", policy, fieldContext)
	if err != nil {
		t.Fatalf("Protect() error = %v", err)
	}
	matched, err := runtime.MatchExact(context.Background(), envelope, "marker", policy, fieldContext)
	if err != nil || !matched {
		t.Fatalf("MatchExact() = %v, %v", matched, err)
	}
}

func TestDataProtectionRuntimeFromConfigRejectsLocalTestOutsideDevelopmentAndTest(t *testing.T) {
	for _, environment := range []string{config.RuntimeEnvironmentStaging, config.RuntimeEnvironmentProduction} {
		_, err := DataProtectionRuntimeFromConfig(dataProtectionConfigForTest(environment, dataprotection.ProviderLocalTest))
		if err == nil || !strings.Contains(err.Error(), "not allowed") {
			t.Fatalf("DataProtectionRuntimeFromConfig(%s) error = %v", environment, err)
		}
	}
}

func TestDataProtectionRuntimeFromConfigRedactsInvalidKeyring(t *testing.T) {
	cfg := dataProtectionConfigForTest(config.RuntimeEnvironmentProduction, dataprotection.ProviderEnvAES256)
	cfg.DataEncryptionKeyringJSON = "{\"enc-v1\":\"raw-keyring-secret-marker\"}"
	_, err := DataProtectionRuntimeFromConfig(cfg)
	if err == nil || strings.Contains(err.Error(), "raw-keyring-secret-marker") || strings.Contains(err.Error(), cfg.DataEncryptionKeyringJSON) {
		t.Fatalf("DataProtectionRuntimeFromConfig() error = %v", err)
	}
}

func TestDataProtectionRuntimeFromConfigRejectsKeysWithoutProvider(t *testing.T) {
	cfg := dataProtectionConfigForTest(config.RuntimeEnvironmentDevelopment, dataprotection.ProviderLocalTest)
	cfg.DataKeyProvider = ""
	if _, err := DataProtectionRuntimeFromConfig(cfg); err == nil || !strings.Contains(err.Error(), "require PLATFORM_DATA_KEY_PROVIDER") {
		t.Fatalf("DataProtectionRuntimeFromConfig() error = %v", err)
	}
}

func dataProtectionConfigForTest(environment string, provider string) config.Config {
	encodedEncryption := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("e", 32)))
	encodedIndex := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("i", 32)))
	return config.Config{
		RuntimeEnvironment:        environment,
		DataKeyProvider:           provider,
		DataEncryptionActiveKeyID: "enc-v1",
		DataEncryptionKeyringJSON: "{\"enc-v1\":\"" + encodedEncryption + "\"}",
		DataBlindIndexActiveKeyID: "idx-v1",
		DataBlindIndexKeyringJSON: "{\"idx-v1\":\"" + encodedIndex + "\"}",
	}
}
