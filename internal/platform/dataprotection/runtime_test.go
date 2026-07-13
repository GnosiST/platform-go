package dataprotection

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestRuntimeProtectRevealUsesRandomNonceAndExactAAD(t *testing.T) {
	runtime := newTestRuntime(t, "enc-v1", "idx-v1")
	policy := FieldPolicy{
		Format:              FormatAES256GCMV1,
		Normalization:       NormalizationEmailV1,
		BlindIndexNamespace: "customer-contact-email",
	}
	fieldContext := FieldContext{
		TenantID: "tenant-a", Resource: "customer-profiles", RecordID: "customer-1001", FieldKey: "governmentReference", SchemaVersion: 1,
	}

	first, err := runtime.Protect(context.Background(), "CaseSensitive@Example.test", policy, fieldContext)
	if err != nil {
		t.Fatalf("Protect(first) error = %v", err)
	}
	second, err := runtime.Protect(context.Background(), "CaseSensitive@Example.test", policy, fieldContext)
	if err != nil {
		t.Fatalf("Protect(second) error = %v", err)
	}
	if first == second {
		t.Fatal("Protect() produced a deterministic envelope")
	}
	if strings.Contains(first, "CaseSensitive@Example.test") {
		t.Fatal("envelope contains plaintext")
	}

	revealed, err := runtime.Reveal(context.Background(), first, policy, fieldContext)
	if err != nil {
		t.Fatalf("Reveal() error = %v", err)
	}
	if revealed != "CaseSensitive@Example.test" {
		t.Fatalf("Reveal() = %q, want original presentation", revealed)
	}

	for name, mutate := range map[string]func(*FieldContext){
		"tenant":         func(value *FieldContext) { value.TenantID = "tenant-b" },
		"resource":       func(value *FieldContext) { value.Resource = "other-resource" },
		"record":         func(value *FieldContext) { value.RecordID = "customer-1002" },
		"field":          func(value *FieldContext) { value.FieldKey = "otherField" },
		"schema version": func(value *FieldContext) { value.SchemaVersion = 2 },
	} {
		t.Run(name, func(t *testing.T) {
			changed := fieldContext
			mutate(&changed)
			if _, err := runtime.Reveal(context.Background(), first, policy, changed); !errors.Is(err, ErrInvalidEnvelope) {
				t.Fatalf("Reveal() error = %v, want ErrInvalidEnvelope", err)
			}
		})
	}
}

func TestRuntimeRejectsEnvelopeAndPolicyTampering(t *testing.T) {
	runtime := newTestRuntime(t, "enc-v1", "idx-v1")
	policy := FieldPolicy{Format: FormatAES256GCMV1, Normalization: NormalizationTrimV1, BlindIndexNamespace: "custom-reference"}
	fieldContext := FieldContext{TenantID: GlobalTenantID, Resource: "custom-records", RecordID: "record-1", FieldKey: "customSecret", SchemaVersion: 7}
	envelope, err := runtime.Protect(context.Background(), "sensitive-marker", policy, fieldContext)
	if err != nil {
		t.Fatal(err)
	}

	for name, mutate := range map[string]func(*envelopeV1){
		"format":          func(value *envelopeV1) { value.Format = "aes-256-gcm-v2" },
		"algorithm":       func(value *envelopeV1) { value.Algorithm = "AES-CTR" },
		"key id":          func(value *envelopeV1) { value.EncryptionKeyID = "enc-v2" },
		"key fingerprint": func(value *envelopeV1) { value.EncryptionKeyFingerprint = "changed" },
		"nonce":           func(value *envelopeV1) { value.Nonce = mutateBase64(value.Nonce) },
		"ciphertext":      func(value *envelopeV1) { value.Ciphertext = mutateBase64(value.Ciphertext) },
		"normalization":   func(value *envelopeV1) { value.Normalization = NormalizationRawV1 },
		"namespace":       func(value *envelopeV1) { value.BlindIndex.Namespace = "other-namespace" },
		"index digest":    func(value *envelopeV1) { value.BlindIndex.Digest = mutateBase64(value.BlindIndex.Digest) },
		"schema version":  func(value *envelopeV1) { value.SchemaVersion++ },
	} {
		t.Run(name, func(t *testing.T) {
			decoded := mustDecodeEnvelope(t, envelope)
			mutate(&decoded)
			tampered := mustEncodeEnvelope(t, decoded)
			if _, err := runtime.Reveal(context.Background(), tampered, policy, fieldContext); !errors.Is(err, ErrInvalidEnvelope) {
				t.Fatalf("Reveal() error = %v, want ErrInvalidEnvelope", err)
			}
		})
	}

	changedPolicy := policy
	changedPolicy.Normalization = NormalizationRawV1
	if err := runtime.Validate(context.Background(), envelope, changedPolicy, fieldContext); !errors.Is(err, ErrPolicyMismatch) {
		t.Fatalf("Validate(policy drift) error = %v, want ErrPolicyMismatch", err)
	}

	for name, mutate := range map[string]func(*envelopeV1){
		"nonce":          func(value *envelopeV1) { value.Nonce = mutateBase64(value.Nonce) },
		"ciphertext":     func(value *envelopeV1) { value.Ciphertext = mutateBase64(value.Ciphertext) },
		"blind index":    func(value *envelopeV1) { value.BlindIndex.Digest = mutateBase64(value.BlindIndex.Digest) },
		"authentication": func(value *envelopeV1) { value.Authentication = mutateBase64(value.Authentication) },
	} {
		t.Run("validate "+name, func(t *testing.T) {
			decoded := mustDecodeEnvelope(t, envelope)
			mutate(&decoded)
			if err := runtime.Validate(context.Background(), mustEncodeEnvelope(t, decoded), policy, fieldContext); !errors.Is(err, ErrInvalidEnvelope) {
				t.Fatalf("Validate() error = %v, want ErrInvalidEnvelope", err)
			}
		})
	}
}

func TestRuntimeBlindIndexSupportsExactMatchAcrossKeyRotation(t *testing.T) {
	oldRuntime := newTestRuntime(t, "enc-v1", "idx-v1")
	policy := FieldPolicy{Format: FormatAES256GCMV1, Normalization: NormalizationEmailV1, BlindIndexNamespace: "customer-email"}
	fieldContext := FieldContext{TenantID: "tenant-a", Resource: "customers", RecordID: "customer-1", FieldKey: "contact", SchemaVersion: 1}
	oldEnvelope, err := oldRuntime.Protect(context.Background(), "Case@Example.test ", policy, fieldContext)
	if err != nil {
		t.Fatal(err)
	}

	rotatedProvider, err := NewStaticKeyProvider(StaticKeyProviderConfig{
		Kind:                  ProviderEnvAES256,
		ActiveEncryptionKeyID: "enc-v2",
		EncryptionKeys:        map[string][]byte{"enc-v1": keyBytes('e'), "enc-v2": keyBytes('n')},
		ActiveBlindIndexKeyID: "idx-v2",
		BlindIndexKeys:        map[string][]byte{"idx-v1": keyBytes('i'), "idx-v2": keyBytes('j')},
	})
	if err != nil {
		t.Fatal(err)
	}
	rotatedRuntime := NewRuntime(rotatedProvider)

	matched, err := rotatedRuntime.MatchExact(context.Background(), oldEnvelope, "case@example.test", policy, fieldContext)
	if err != nil {
		t.Fatalf("MatchExact(old envelope) error = %v", err)
	}
	if !matched {
		t.Fatal("MatchExact() = false, want true for historical blind-index key")
	}
	matched, err = rotatedRuntime.MatchExact(context.Background(), oldEnvelope, "other@example.test", policy, fieldContext)
	if err != nil {
		t.Fatal(err)
	}
	if matched {
		t.Fatal("MatchExact() = true for a different value")
	}

	newEnvelope, err := rotatedRuntime.Protect(context.Background(), "case@example.test", policy, fieldContext)
	if err != nil {
		t.Fatal(err)
	}
	oldDecoded := mustDecodeEnvelope(t, oldEnvelope)
	newDecoded := mustDecodeEnvelope(t, newEnvelope)
	if oldDecoded.EncryptionKeyID == newDecoded.EncryptionKeyID || oldDecoded.BlindIndex.KeyID == newDecoded.BlindIndex.KeyID {
		t.Fatal("rotation did not change active key IDs")
	}
	if oldDecoded.BlindIndex.Digest == newDecoded.BlindIndex.Digest {
		t.Fatal("blind index digest did not change across key versions")
	}
	if _, err := rotatedRuntime.Reveal(context.Background(), oldEnvelope, policy, fieldContext); err != nil {
		t.Fatalf("Reveal(old envelope after rotation) error = %v", err)
	}

	tampered := mustDecodeEnvelope(t, oldEnvelope)
	tampered.Ciphertext = mutateBase64(tampered.Ciphertext)
	if _, err := rotatedRuntime.MatchExact(context.Background(), mustEncodeEnvelope(t, tampered), "case@example.test", policy, fieldContext); !errors.Is(err, ErrInvalidEnvelope) {
		t.Fatalf("MatchExact(tampered envelope) error = %v, want ErrInvalidEnvelope", err)
	}
}

func TestRuntimeUsesDeclaredNormalizationInsteadOfFieldName(t *testing.T) {
	runtime := newTestRuntime(t, "enc-v1", "idx-v1")
	fieldContext := FieldContext{TenantID: GlobalTenantID, Resource: "custom", RecordID: "custom-1", FieldKey: "notAnEmailField", SchemaVersion: 1}
	emailPolicy := FieldPolicy{Format: FormatAES256GCMV1, Normalization: NormalizationEmailV1, BlindIndexNamespace: "custom-value"}
	rawPolicy := FieldPolicy{Format: FormatAES256GCMV1, Normalization: NormalizationRawV1, BlindIndexNamespace: "custom-value"}

	emailEnvelope, err := runtime.Protect(context.Background(), "Case@Example.test ", emailPolicy, fieldContext)
	if err != nil {
		t.Fatal(err)
	}
	rawEnvelope, err := runtime.Protect(context.Background(), "Case@Example.test ", rawPolicy, fieldContext)
	if err != nil {
		t.Fatal(err)
	}
	matched, err := runtime.MatchExact(context.Background(), emailEnvelope, "case@example.test", emailPolicy, fieldContext)
	if err != nil || !matched {
		t.Fatalf("email MatchExact() = %v, %v", matched, err)
	}
	matched, err = runtime.MatchExact(context.Background(), rawEnvelope, "case@example.test", rawPolicy, fieldContext)
	if err != nil {
		t.Fatal(err)
	}
	if matched {
		t.Fatal("raw normalizer unexpectedly performed email normalization")
	}

	otherContext := fieldContext
	otherContext.TenantID = "tenant-b"
	otherEnvelope, err := runtime.Protect(context.Background(), "Case@Example.test ", emailPolicy, otherContext)
	if err != nil {
		t.Fatal(err)
	}
	if mustDecodeEnvelope(t, emailEnvelope).BlindIndex.Digest == mustDecodeEnvelope(t, otherEnvelope).BlindIndex.Digest {
		t.Fatal("blind index digest was reused across tenant contexts")
	}
}

func TestRuntimeValidationDetectsMissingAndReplacedHistoricalKeys(t *testing.T) {
	runtime := newTestRuntime(t, "enc-v1", "idx-v1")
	policy := FieldPolicy{Format: FormatAES256GCMV1, Normalization: NormalizationTrimV1, BlindIndexNamespace: "reference"}
	fieldContext := FieldContext{TenantID: GlobalTenantID, Resource: "custom", RecordID: "custom-1", FieldKey: "reference", SchemaVersion: 1}
	envelope, err := runtime.Protect(context.Background(), "marker", policy, fieldContext)
	if err != nil {
		t.Fatal(err)
	}

	missingProvider, err := NewStaticKeyProvider(StaticKeyProviderConfig{
		Kind:                  ProviderEnvAES256,
		ActiveEncryptionKeyID: "enc-v2",
		EncryptionKeys:        map[string][]byte{"enc-v2": keyBytes('n')},
		ActiveBlindIndexKeyID: "idx-v2",
		BlindIndexKeys:        map[string][]byte{"idx-v2": keyBytes('j')},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := NewRuntime(missingProvider).Validate(context.Background(), envelope, policy, fieldContext); !errors.Is(err, ErrKeyUnavailable) {
		t.Fatalf("Validate(missing historical key) error = %v, want ErrKeyUnavailable", err)
	}

	replacedProvider, err := NewStaticKeyProvider(StaticKeyProviderConfig{
		Kind:                  ProviderEnvAES256,
		ActiveEncryptionKeyID: "enc-v1",
		EncryptionKeys:        map[string][]byte{"enc-v1": keyBytes('x')},
		ActiveBlindIndexKeyID: "idx-v1",
		BlindIndexKeys:        map[string][]byte{"idx-v1": keyBytes('y')},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := NewRuntime(replacedProvider).Validate(context.Background(), envelope, policy, fieldContext); !errors.Is(err, ErrKeyMismatch) {
		t.Fatalf("Validate(replaced key) error = %v, want ErrKeyMismatch", err)
	}
}

func TestRuntimeReadinessRequiresAvailableActiveKeys(t *testing.T) {
	if err := newTestRuntime(t, "enc-v1", "idx-v1").Ready(context.Background()); err != nil {
		t.Fatalf("Ready() error = %v", err)
	}
	if err := newTestRuntime(t, "enc-v1", "idx-v1").Ready(nil); !errors.Is(err, ErrKeyUnavailable) {
		t.Fatalf("Ready(nil) error = %v, want ErrKeyUnavailable", err)
	}

	var nilRuntime *Service
	if err := nilRuntime.Ready(context.Background()); !errors.Is(err, ErrKeyUnavailable) {
		t.Fatalf("nil Ready() error = %v, want ErrKeyUnavailable", err)
	}
	if err := NewRuntime(nil).Ready(context.Background()); !errors.Is(err, ErrKeyUnavailable) {
		t.Fatalf("provider-less Ready() error = %v, want ErrKeyUnavailable", err)
	}
}

func newTestRuntime(t *testing.T, encryptionKeyID string, indexKeyID string) *Service {
	t.Helper()
	provider, err := NewStaticKeyProvider(StaticKeyProviderConfig{
		Kind:                  ProviderEnvAES256,
		ActiveEncryptionKeyID: encryptionKeyID,
		EncryptionKeys:        map[string][]byte{encryptionKeyID: keyBytes('e')},
		ActiveBlindIndexKeyID: indexKeyID,
		BlindIndexKeys:        map[string][]byte{indexKeyID: keyBytes('i')},
	})
	if err != nil {
		t.Fatal(err)
	}
	return NewRuntime(provider)
}

func keyBytes(value byte) []byte {
	return []byte(strings.Repeat(string(value), 32))
}

func mustDecodeEnvelope(t *testing.T, raw string) envelopeV1 {
	t.Helper()
	payload := strings.TrimPrefix(raw, envelopePrefix)
	decoded, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		t.Fatal(err)
	}
	var envelope envelopeV1
	if err := json.Unmarshal(decoded, &envelope); err != nil {
		t.Fatal(err)
	}
	return envelope
}

func mustEncodeEnvelope(t *testing.T, envelope envelopeV1) string {
	t.Helper()
	encoded, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	return envelopePrefix + base64.RawURLEncoding.EncodeToString(encoded)
}

func mutateBase64(raw string) string {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(decoded) == 0 {
		return raw + "x"
	}
	decoded[len(decoded)-1] ^= 1
	return base64.RawURLEncoding.EncodeToString(decoded)
}
