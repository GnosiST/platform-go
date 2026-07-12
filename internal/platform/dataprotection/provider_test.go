package dataprotection

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func TestStaticKeyProviderRejectsInvalidAndReusedKeyMaterial(t *testing.T) {
	valid := StaticKeyProviderConfig{
		Kind:                  ProviderEnvAES256,
		ActiveEncryptionKeyID: "enc-v1",
		EncryptionKeys:        map[string][]byte{"enc-v1": keyBytes('e')},
		ActiveBlindIndexKeyID: "idx-v1",
		BlindIndexKeys:        map[string][]byte{"idx-v1": keyBytes('i')},
	}
	for name, mutate := range map[string]func(*StaticKeyProviderConfig){
		"missing kind":         func(config *StaticKeyProviderConfig) { config.Kind = "" },
		"non-canonical kind":   func(config *StaticKeyProviderConfig) { config.Kind = " Env-AES256 " },
		"invalid active id":    func(config *StaticKeyProviderConfig) { config.ActiveEncryptionKeyID = "Enc V1" },
		"missing active key":   func(config *StaticKeyProviderConfig) { config.ActiveEncryptionKeyID = "enc-v2" },
		"short encryption key": func(config *StaticKeyProviderConfig) { config.EncryptionKeys["enc-v1"] = []byte("short") },
		"short index key":      func(config *StaticKeyProviderConfig) { config.BlindIndexKeys["idx-v1"] = []byte("short") },
		"reused material":      func(config *StaticKeyProviderConfig) { config.BlindIndexKeys["idx-v1"] = keyBytes('e') },
	} {
		t.Run(name, func(t *testing.T) {
			config := cloneProviderConfig(valid)
			mutate(&config)
			if _, err := NewStaticKeyProvider(config); !errors.Is(err, ErrInvalidKeyConfig) {
				t.Fatalf("NewStaticKeyProvider() error = %v, want ErrInvalidKeyConfig", err)
			}
		})
	}
}

func TestParseEncodedKeyringRejectsMalformedInputWithoutEchoingSecret(t *testing.T) {
	secretMarker := "raw-secret-marker"
	for name, raw := range map[string]string{
		"invalid json":   "{\"enc-v1\":\"raw-secret-marker\"",
		"invalid base64": "{\"enc-v1\":\"raw-secret-marker\"}",
		"wrong length":   "{\"enc-v1\":\"" + base64.StdEncoding.EncodeToString([]byte("short")) + "\"}",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseEncodedKeyring(raw)
			if !errors.Is(err, ErrInvalidKeyConfig) {
				t.Fatalf("ParseEncodedKeyring() error = %v, want ErrInvalidKeyConfig", err)
			}
			if strings.Contains(err.Error(), secretMarker) || strings.Contains(err.Error(), raw) {
				t.Fatalf("error exposed key configuration: %v", err)
			}
		})
	}
}

func TestStaticKeyProviderReturnsClonedKeys(t *testing.T) {
	provider, err := NewStaticKeyProvider(StaticKeyProviderConfig{
		Kind:                  ProviderEnvAES256,
		ActiveEncryptionKeyID: "enc-v1",
		EncryptionKeys:        map[string][]byte{"enc-v1": keyBytes('e')},
		ActiveBlindIndexKeyID: "idx-v1",
		BlindIndexKeys:        map[string][]byte{"idx-v1": keyBytes('i')},
	})
	if err != nil {
		t.Fatal(err)
	}
	first, err := provider.Key(context.Background(), KeyPurposeEncryption, "enc-v1")
	if err != nil {
		t.Fatal(err)
	}
	first.Material[0] ^= 1
	second, err := provider.Key(context.Background(), KeyPurposeEncryption, "enc-v1")
	if err != nil {
		t.Fatal(err)
	}
	if first.Material[0] == second.Material[0] {
		t.Fatal("provider returned mutable shared key material")
	}
}

func cloneProviderConfig(config StaticKeyProviderConfig) StaticKeyProviderConfig {
	config.EncryptionKeys = cloneKeyMap(config.EncryptionKeys)
	config.BlindIndexKeys = cloneKeyMap(config.BlindIndexKeys)
	return config
}

func cloneKeyMap(source map[string][]byte) map[string][]byte {
	cloned := make(map[string][]byte, len(source))
	for key, value := range source {
		cloned[key] = append([]byte(nil), value...)
	}
	return cloned
}
