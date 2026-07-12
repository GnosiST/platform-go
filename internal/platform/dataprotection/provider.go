package dataprotection

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
)

const (
	ProviderEnvAES256 = "env-aes256"
	ProviderLocalTest = "local-test"
)

type KeyPurpose string

const (
	KeyPurposeEncryption KeyPurpose = "encryption"
	KeyPurposeBlindIndex KeyPurpose = "blind-index"
)

var (
	ErrInvalidKeyConfig = errors.New("invalid data protection key configuration")
	ErrKeyUnavailable   = errors.New("data protection key unavailable")
	ErrKeyMismatch      = errors.New("data protection key mismatch")
)

type Key struct {
	ID          string
	Material    []byte
	Fingerprint string
}

type KeyProvider interface {
	Kind() string
	ActiveKey(context.Context, KeyPurpose) (Key, error)
	Key(context.Context, KeyPurpose, string) (Key, error)
	KeyIDs(KeyPurpose) []string
}

type StaticKeyProviderConfig struct {
	Kind                  string
	ActiveEncryptionKeyID string
	EncryptionKeys        map[string][]byte
	ActiveBlindIndexKeyID string
	BlindIndexKeys        map[string][]byte
}

type StaticKeyProvider struct {
	kind     string
	active   map[KeyPurpose]string
	keyrings map[KeyPurpose]map[string]Key
}

func NewStaticKeyProvider(config StaticKeyProviderConfig) (*StaticKeyProvider, error) {
	if !canonicalIdentifier(config.Kind) {
		return nil, fmt.Errorf("%w: provider kind must be canonical", ErrInvalidKeyConfig)
	}
	encryption, fingerprints, err := validateKeyring(config.EncryptionKeys, nil)
	if err != nil {
		return nil, err
	}
	blindIndexes, _, err := validateKeyring(config.BlindIndexKeys, fingerprints)
	if err != nil {
		return nil, err
	}
	if !canonicalIdentifier(config.ActiveEncryptionKeyID) || !canonicalIdentifier(config.ActiveBlindIndexKeyID) {
		return nil, fmt.Errorf("%w: active key IDs must be canonical", ErrInvalidKeyConfig)
	}
	if _, ok := encryption[config.ActiveEncryptionKeyID]; !ok {
		return nil, fmt.Errorf("%w: active encryption key is unavailable", ErrInvalidKeyConfig)
	}
	if _, ok := blindIndexes[config.ActiveBlindIndexKeyID]; !ok {
		return nil, fmt.Errorf("%w: active blind-index key is unavailable", ErrInvalidKeyConfig)
	}
	return &StaticKeyProvider{
		kind: config.Kind,
		active: map[KeyPurpose]string{
			KeyPurposeEncryption: config.ActiveEncryptionKeyID,
			KeyPurposeBlindIndex: config.ActiveBlindIndexKeyID,
		},
		keyrings: map[KeyPurpose]map[string]Key{
			KeyPurposeEncryption: encryption,
			KeyPurposeBlindIndex: blindIndexes,
		},
	}, nil
}

func validateKeyring(source map[string][]byte, reserved map[string]struct{}) (map[string]Key, map[string]struct{}, error) {
	if len(source) == 0 {
		return nil, nil, fmt.Errorf("%w: keyring is required", ErrInvalidKeyConfig)
	}
	fingerprints := make(map[string]struct{}, len(source)+len(reserved))
	for value := range reserved {
		fingerprints[value] = struct{}{}
	}
	result := make(map[string]Key, len(source))
	for id, material := range source {
		if !canonicalIdentifier(id) {
			return nil, nil, fmt.Errorf("%w: key ID must be canonical", ErrInvalidKeyConfig)
		}
		if len(material) != 32 {
			return nil, nil, fmt.Errorf("%w: keys must contain 32 bytes", ErrInvalidKeyConfig)
		}
		fingerprint := keyFingerprint(material)
		if _, exists := fingerprints[fingerprint]; exists {
			return nil, nil, fmt.Errorf("%w: key material must not be reused", ErrInvalidKeyConfig)
		}
		fingerprints[fingerprint] = struct{}{}
		result[id] = Key{ID: id, Material: append([]byte(nil), material...), Fingerprint: fingerprint}
	}
	return result, fingerprints, nil
}

func ParseEncodedKeyring(raw string) (map[string][]byte, error) {
	decoder := json.NewDecoder(strings.NewReader(raw))
	var encoded map[string]string
	if err := decoder.Decode(&encoded); err != nil {
		return nil, fmt.Errorf("%w: keyring JSON is invalid", ErrInvalidKeyConfig)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, fmt.Errorf("%w: keyring JSON is invalid", ErrInvalidKeyConfig)
	}
	if len(encoded) == 0 {
		return nil, fmt.Errorf("%w: keyring is required", ErrInvalidKeyConfig)
	}
	decoded := make(map[string][]byte, len(encoded))
	for id, value := range encoded {
		if !canonicalIdentifier(id) {
			return nil, fmt.Errorf("%w: key ID must be canonical", ErrInvalidKeyConfig)
		}
		material, err := base64.StdEncoding.Strict().DecodeString(value)
		if err != nil || len(material) != 32 {
			return nil, fmt.Errorf("%w: key material must be base64-encoded 32-byte values", ErrInvalidKeyConfig)
		}
		decoded[id] = material
	}
	return decoded, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("trailing JSON value")
	}
	return nil
}

func (p *StaticKeyProvider) Kind() string {
	if p == nil {
		return ""
	}
	return p.kind
}

func (p *StaticKeyProvider) ActiveKey(ctx context.Context, purpose KeyPurpose) (Key, error) {
	if p == nil {
		return Key{}, ErrKeyUnavailable
	}
	id, ok := p.active[purpose]
	if !ok {
		return Key{}, ErrKeyUnavailable
	}
	return p.Key(ctx, purpose, id)
}

func (p *StaticKeyProvider) Key(_ context.Context, purpose KeyPurpose, id string) (Key, error) {
	if p == nil {
		return Key{}, ErrKeyUnavailable
	}
	keyring, ok := p.keyrings[purpose]
	if !ok {
		return Key{}, ErrKeyUnavailable
	}
	key, ok := keyring[id]
	if !ok {
		return Key{}, ErrKeyUnavailable
	}
	key.Material = bytes.Clone(key.Material)
	return key, nil
}

func (p *StaticKeyProvider) KeyIDs(purpose KeyPurpose) []string {
	if p == nil {
		return nil
	}
	keyring := p.keyrings[purpose]
	ids := make([]string, 0, len(keyring))
	for id := range keyring {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

func keyFingerprint(material []byte) string {
	sum := sha256.Sum256(append([]byte("platform-data-key-fingerprint\x00"), material...))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
