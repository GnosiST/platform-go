package dataprotection

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
)

type Runtime interface {
	Protect(context.Context, string, FieldPolicy, FieldContext) (string, error)
	Validate(context.Context, string, FieldPolicy, FieldContext) error
	Reveal(context.Context, string, FieldPolicy, FieldContext) (string, error)
	MatchExact(context.Context, string, string, FieldPolicy, FieldContext) (bool, error)
}

type RuntimeReadiness interface {
	Ready(context.Context) error
}

type Service struct {
	provider KeyProvider
}

func NewRuntime(provider KeyProvider) *Service {
	return &Service{provider: provider}
}

func (s *Service) Ready(ctx context.Context) error {
	if s == nil || s.provider == nil || ctx.Err() != nil {
		return ErrKeyUnavailable
	}
	for _, purpose := range []KeyPurpose{KeyPurposeEncryption, KeyPurposeBlindIndex} {
		if _, err := s.provider.ActiveKey(ctx, purpose); err != nil {
			return ErrKeyUnavailable
		}
	}
	return nil
}

func (s *Service) Protect(ctx context.Context, plaintext string, policy FieldPolicy, fieldContext FieldContext) (string, error) {
	if err := policy.validate(); err != nil {
		return "", err
	}
	if err := fieldContext.validate(); err != nil {
		return "", err
	}
	if s == nil || s.provider == nil {
		return "", ErrKeyUnavailable
	}
	encryptionKey, err := s.provider.ActiveKey(ctx, KeyPurposeEncryption)
	if err != nil {
		return "", ErrKeyUnavailable
	}
	envelope := envelopeV1{
		Version: envelopeVersion, Format: policy.Format, Algorithm: algorithmAESGCM,
		EncryptionKeyID: encryptionKey.ID, EncryptionKeyFingerprint: encryptionKey.Fingerprint,
		SchemaVersion: fieldContext.SchemaVersion, Normalization: policy.Normalization,
	}
	if policy.BlindIndexNamespace != "" {
		indexKey, keyErr := s.provider.ActiveKey(ctx, KeyPurposeBlindIndex)
		if keyErr != nil {
			return "", ErrKeyUnavailable
		}
		digest, digestErr := blindIndexDigest(indexKey.Material, policy, fieldContext, plaintext)
		if digestErr != nil {
			return "", digestErr
		}
		envelope.BlindIndex = blindIndexEnvelopeV1{
			Version: blindIndexV1, KeyID: indexKey.ID, KeyFingerprint: indexKey.Fingerprint,
			Namespace: policy.BlindIndexNamespace, Digest: base64.RawURLEncoding.EncodeToString(digest),
		}
	}
	aead, err := newAEAD(encryptionKey.Material)
	if err != nil {
		return "", ErrKeyUnavailable
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("%w: nonce generation failed", ErrInvalidEnvelope)
	}
	envelope.Nonce = base64.RawURLEncoding.EncodeToString(nonce)
	aad, err := envelopeAAD(envelope, fieldContext)
	if err != nil {
		return "", err
	}
	envelope.Ciphertext = base64.RawURLEncoding.EncodeToString(aead.Seal(nil, nonce, []byte(plaintext), aad))
	authentication, err := envelopeAuthentication(encryptionKey.Material, envelope, fieldContext)
	if err != nil {
		return "", err
	}
	envelope.Authentication = base64.RawURLEncoding.EncodeToString(authentication)
	return encodeEnvelope(envelope)
}

func (s *Service) Validate(ctx context.Context, raw string, policy FieldPolicy, fieldContext FieldContext) error {
	if err := policy.validate(); err != nil {
		return err
	}
	if err := fieldContext.validate(); err != nil {
		return err
	}
	envelope, err := decodeEnvelope(raw)
	if err != nil {
		return err
	}
	return s.validateDecodedEnvelope(ctx, envelope, policy, fieldContext)
}

func (s *Service) Reveal(ctx context.Context, raw string, policy FieldPolicy, fieldContext FieldContext) (string, error) {
	envelope, err := decodeEnvelope(raw)
	if err != nil {
		return "", ErrInvalidEnvelope
	}
	if err := s.validateDecodedEnvelope(ctx, envelope, policy, fieldContext); err != nil {
		return "", ErrInvalidEnvelope
	}
	key, err := s.provider.Key(ctx, KeyPurposeEncryption, envelope.EncryptionKeyID)
	if err != nil {
		return "", ErrInvalidEnvelope
	}
	aead, err := newAEAD(key.Material)
	if err != nil {
		return "", ErrInvalidEnvelope
	}
	nonce, err := base64.RawURLEncoding.Strict().DecodeString(envelope.Nonce)
	if err != nil || len(nonce) != aead.NonceSize() {
		return "", ErrInvalidEnvelope
	}
	ciphertext, err := base64.RawURLEncoding.Strict().DecodeString(envelope.Ciphertext)
	if err != nil || len(ciphertext) < aead.Overhead() {
		return "", ErrInvalidEnvelope
	}
	aad, err := envelopeAAD(envelope, fieldContext)
	if err != nil {
		return "", ErrInvalidEnvelope
	}
	plaintext, err := aead.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return "", ErrInvalidEnvelope
	}
	return string(plaintext), nil
}

func (s *Service) MatchExact(ctx context.Context, raw string, candidate string, policy FieldPolicy, fieldContext FieldContext) (bool, error) {
	envelope, err := decodeEnvelope(raw)
	if err != nil {
		return false, ErrInvalidEnvelope
	}
	if err := s.validateDecodedEnvelope(ctx, envelope, policy, fieldContext); err != nil {
		return false, ErrInvalidEnvelope
	}
	if policy.BlindIndexNamespace == "" {
		return false, fmt.Errorf("%w: exact-match index is disabled", ErrInvalidPolicy)
	}
	key, err := s.provider.Key(ctx, KeyPurposeBlindIndex, envelope.BlindIndex.KeyID)
	if err != nil || subtle.ConstantTimeCompare([]byte(key.Fingerprint), []byte(envelope.BlindIndex.KeyFingerprint)) != 1 {
		return false, ErrInvalidEnvelope
	}
	digest, err := blindIndexDigest(key.Material, policy, fieldContext, candidate)
	if err != nil {
		return false, err
	}
	stored, err := base64.RawURLEncoding.Strict().DecodeString(envelope.BlindIndex.Digest)
	if err != nil || len(stored) != sha256.Size {
		return false, ErrInvalidEnvelope
	}
	return hmac.Equal(stored, digest), nil
}

func (s *Service) validateDecodedEnvelope(ctx context.Context, envelope envelopeV1, policy FieldPolicy, fieldContext FieldContext) error {
	if s == nil || s.provider == nil {
		return ErrKeyUnavailable
	}
	if envelope.Version != envelopeVersion || envelope.Format != policy.Format || envelope.Algorithm != algorithmAESGCM || envelope.Normalization != policy.Normalization || envelope.SchemaVersion != fieldContext.SchemaVersion {
		return ErrPolicyMismatch
	}
	if policy.BlindIndexNamespace == "" {
		if envelope.BlindIndex != (blindIndexEnvelopeV1{}) {
			return ErrPolicyMismatch
		}
	} else if envelope.BlindIndex.Version != blindIndexV1 || envelope.BlindIndex.Namespace != policy.BlindIndexNamespace || !canonicalIdentifier(envelope.BlindIndex.KeyID) || envelope.BlindIndex.KeyFingerprint == "" {
		return ErrPolicyMismatch
	}
	if !canonicalIdentifier(envelope.EncryptionKeyID) || envelope.EncryptionKeyFingerprint == "" {
		return ErrInvalidEnvelope
	}
	encryptionKey, err := s.provider.Key(ctx, KeyPurposeEncryption, envelope.EncryptionKeyID)
	if err != nil {
		return ErrKeyUnavailable
	}
	if subtle.ConstantTimeCompare([]byte(encryptionKey.Fingerprint), []byte(envelope.EncryptionKeyFingerprint)) != 1 {
		return ErrKeyMismatch
	}
	aead, err := newAEAD(encryptionKey.Material)
	if err != nil {
		return ErrKeyUnavailable
	}
	nonce, err := base64.RawURLEncoding.Strict().DecodeString(envelope.Nonce)
	if err != nil || len(nonce) != aead.NonceSize() {
		return ErrInvalidEnvelope
	}
	ciphertext, err := base64.RawURLEncoding.Strict().DecodeString(envelope.Ciphertext)
	if err != nil || len(ciphertext) < aead.Overhead() {
		return ErrInvalidEnvelope
	}
	if policy.BlindIndexNamespace != "" {
		indexKey, keyErr := s.provider.Key(ctx, KeyPurposeBlindIndex, envelope.BlindIndex.KeyID)
		if keyErr != nil {
			return ErrKeyUnavailable
		}
		if subtle.ConstantTimeCompare([]byte(indexKey.Fingerprint), []byte(envelope.BlindIndex.KeyFingerprint)) != 1 {
			return ErrKeyMismatch
		}
		digest, decodeErr := base64.RawURLEncoding.Strict().DecodeString(envelope.BlindIndex.Digest)
		if decodeErr != nil || len(digest) != sha256.Size {
			return ErrInvalidEnvelope
		}
	}
	authentication, err := base64.RawURLEncoding.Strict().DecodeString(envelope.Authentication)
	if err != nil || len(authentication) != sha256.Size {
		return ErrInvalidEnvelope
	}
	expectedAuthentication, err := envelopeAuthentication(encryptionKey.Material, envelope, fieldContext)
	if err != nil || !hmac.Equal(authentication, expectedAuthentication) {
		return ErrInvalidEnvelope
	}
	return nil
}

func newAEAD(material []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(material)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

type aadV1 struct {
	Version                  int                  `json:"version"`
	Format                   string               `json:"format"`
	Algorithm                string               `json:"algorithm"`
	EncryptionKeyID          string               `json:"encryptionKeyId"`
	EncryptionKeyFingerprint string               `json:"encryptionKeyFingerprint"`
	TenantID                 string               `json:"tenantId"`
	Resource                 string               `json:"resource"`
	RecordID                 string               `json:"recordId"`
	FieldKey                 string               `json:"fieldKey"`
	SchemaVersion            uint32               `json:"schemaVersion"`
	Normalization            string               `json:"normalization"`
	BlindIndex               blindIndexEnvelopeV1 `json:"blindIndex,omitempty"`
}

func envelopeAAD(envelope envelopeV1, fieldContext FieldContext) ([]byte, error) {
	value, err := envelopeAADValue(envelope, fieldContext)
	if err != nil {
		return nil, err
	}
	return json.Marshal(value)
}

type envelopeAuthenticationInputV1 struct {
	AAD        aadV1  `json:"aad"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

func envelopeAuthentication(material []byte, envelope envelopeV1, fieldContext FieldContext) ([]byte, error) {
	aad, err := envelopeAADValue(envelope, fieldContext)
	if err != nil {
		return nil, err
	}
	input, err := json.Marshal(envelopeAuthenticationInputV1{AAD: aad, Nonce: envelope.Nonce, Ciphertext: envelope.Ciphertext})
	if err != nil {
		return nil, fmt.Errorf("%w: authentication encoding failed", ErrInvalidEnvelope)
	}
	derivation := hmac.New(sha256.New, material)
	_, _ = derivation.Write([]byte("platform-envelope-authentication-key-v1"))
	authentication := hmac.New(sha256.New, derivation.Sum(nil))
	_, _ = authentication.Write([]byte("platform-envelope-authentication-v1\x00"))
	_, _ = authentication.Write(input)
	return authentication.Sum(nil), nil
}

func envelopeAADValue(envelope envelopeV1, fieldContext FieldContext) (aadV1, error) {
	if err := fieldContext.validate(); err != nil {
		return aadV1{}, err
	}
	return aadV1{
		Version: envelope.Version, Format: envelope.Format, Algorithm: envelope.Algorithm,
		EncryptionKeyID: envelope.EncryptionKeyID, EncryptionKeyFingerprint: envelope.EncryptionKeyFingerprint,
		TenantID: fieldContext.TenantID, Resource: fieldContext.Resource, RecordID: fieldContext.RecordID, FieldKey: fieldContext.FieldKey,
		SchemaVersion: envelope.SchemaVersion, Normalization: envelope.Normalization, BlindIndex: envelope.BlindIndex,
	}, nil
}

type blindIndexInputV1 struct {
	Version       string `json:"version"`
	Namespace     string `json:"namespace"`
	Normalization string `json:"normalization"`
	TenantID      string `json:"tenantId"`
	Resource      string `json:"resource"`
	FieldKey      string `json:"fieldKey"`
	SchemaVersion uint32 `json:"schemaVersion"`
	Value         string `json:"value"`
}

func blindIndexDigest(material []byte, policy FieldPolicy, fieldContext FieldContext, value string) ([]byte, error) {
	normalized, err := normalizeValue(value, policy.Normalization)
	if err != nil {
		return nil, err
	}
	input, err := json.Marshal(blindIndexInputV1{
		Version: blindIndexV1, Namespace: policy.BlindIndexNamespace, Normalization: policy.Normalization,
		TenantID: fieldContext.TenantID, Resource: fieldContext.Resource, FieldKey: fieldContext.FieldKey, SchemaVersion: fieldContext.SchemaVersion,
		Value: normalized,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: blind-index encoding failed", ErrInvalidPolicy)
	}
	mac := hmac.New(sha256.New, material)
	_, _ = mac.Write([]byte("platform-blind-index\x00"))
	_, _ = mac.Write(input)
	return mac.Sum(nil), nil
}

func IsEnvelope(value string) bool {
	_, err := decodeEnvelope(value)
	return err == nil
}
