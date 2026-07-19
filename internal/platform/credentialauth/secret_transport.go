package credentialauth

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/hkdf"
)

const (
	SecretTransportVersion   = "pgo-auth-secret-v1"
	SecretTransportAlgorithm = "ECDH-P256-HKDF-SHA256+A256GCM"

	defaultSecretTransportTTL       = 10 * time.Minute
	defaultSecretTransportReplayTTL = 10 * time.Minute
	secretTransportInfoPrefix       = "platform-go credential-auth secret v1"
)

type SecretTransportPublicKey struct {
	Version   string    `json:"version"`
	Algorithm string    `json:"algorithm"`
	KeyID     string    `json:"keyId"`
	PublicKey string    `json:"publicKey"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type SecretEnvelope struct {
	Version         string `json:"version"`
	Algorithm       string `json:"algorithm"`
	KeyID           string `json:"keyId"`
	ClientPublicKey string `json:"clientPublicKey"`
	Salt            string `json:"salt"`
	Nonce           string `json:"nonce"`
	Ciphertext      string `json:"ciphertext"`
}

type SecretTransportOptions struct {
	KeyID      string
	PrivateKey []byte
	Now        func() time.Time
	TTL        time.Duration
	ReplayTTL  time.Duration
}

type SecretTransport struct {
	mu         sync.Mutex
	keyID      string
	privateKey *ecdh.PrivateKey
	now        func() time.Time
	ttl        time.Duration
	replayTTL  time.Duration
	seen       map[string]time.Time
}

func NewSecretTransport(options SecretTransportOptions) (*SecretTransport, error) {
	keyID := strings.TrimSpace(options.KeyID)
	if keyID == "" {
		keyID = "development-ephemeral"
	}
	privateBytes := append([]byte(nil), options.PrivateKey...)
	if len(privateBytes) == 0 {
		generated, err := ecdh.P256().GenerateKey(rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("%w: generate secret transport key", ErrInvalidSecret)
		}
		privateBytes = generated.Bytes()
	}
	privateKey, err := ecdh.P256().NewPrivateKey(privateBytes)
	if err != nil {
		return nil, fmt.Errorf("%w: secret transport private key is invalid", ErrInvalidSecret)
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	ttl := options.TTL
	if ttl <= 0 {
		ttl = defaultSecretTransportTTL
	}
	replayTTL := options.ReplayTTL
	if replayTTL <= 0 {
		replayTTL = defaultSecretTransportReplayTTL
	}
	return &SecretTransport{
		keyID:      keyID,
		privateKey: privateKey,
		now:        now,
		ttl:        ttl,
		replayTTL:  replayTTL,
		seen:       map[string]time.Time{},
	}, nil
}

func DecodeSecretTransportPrivateKey(value string) ([]byte, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, fmt.Errorf("%w: secret transport private key is required", ErrInvalidSecret)
	}
	decoded, err := base64.RawURLEncoding.DecodeString(trimmed)
	if err != nil {
		decoded, err = base64.StdEncoding.DecodeString(trimmed)
	}
	if err != nil || len(decoded) != 32 {
		return nil, fmt.Errorf("%w: secret transport private key must be a base64-encoded 32-byte P-256 scalar", ErrInvalidSecret)
	}
	if _, err := ecdh.P256().NewPrivateKey(decoded); err != nil {
		return nil, fmt.Errorf("%w: secret transport private key is invalid", ErrInvalidSecret)
	}
	return decoded, nil
}

func (t *SecretTransport) PublicKey() SecretTransportPublicKey {
	now := t.now().UTC()
	return SecretTransportPublicKey{
		Version:   SecretTransportVersion,
		Algorithm: SecretTransportAlgorithm,
		KeyID:     t.keyID,
		PublicKey: base64.RawURLEncoding.EncodeToString(t.privateKey.PublicKey().Bytes()),
		ExpiresAt: now.Add(t.ttl),
	}
}

func (t *SecretTransport) Decrypt(ctx context.Context, envelope SecretEnvelope, aad string) (string, error) {
	if err := checkContext(ctx); err != nil {
		return "", err
	}
	if t == nil || t.privateKey == nil {
		return "", fmt.Errorf("%w: secret transport is unavailable", ErrInvalidSecret)
	}
	envelope = normalizedSecretEnvelope(envelope)
	if envelope.Version != SecretTransportVersion || envelope.Algorithm != SecretTransportAlgorithm || envelope.KeyID != t.keyID {
		return "", ErrInvalidSecret
	}
	clientPublicKey, err := decodeBase64URL(envelope.ClientPublicKey)
	if err != nil {
		return "", ErrInvalidSecret
	}
	salt, err := decodeBase64URL(envelope.Salt)
	if err != nil || len(salt) < 16 {
		return "", ErrInvalidSecret
	}
	nonce, err := decodeBase64URL(envelope.Nonce)
	if err != nil || len(nonce) != 12 {
		return "", ErrInvalidSecret
	}
	ciphertext, err := decodeBase64URL(envelope.Ciphertext)
	if err != nil || len(ciphertext) == 0 {
		return "", ErrInvalidSecret
	}
	replayKey := secretEnvelopeReplayKey(envelope)
	if t.replayed(replayKey) {
		return "", ErrInvalidSecret
	}
	peer, err := ecdh.P256().NewPublicKey(clientPublicKey)
	if err != nil {
		return "", ErrInvalidSecret
	}
	shared, err := t.privateKey.ECDH(peer)
	if err != nil {
		return "", ErrInvalidSecret
	}
	aead, err := secretEnvelopeAEAD(shared, salt, t.keyID, aad)
	if err != nil {
		return "", err
	}
	plaintext, err := aead.Open(nil, nonce, ciphertext, []byte(aad))
	if err != nil {
		return "", ErrInvalidSecret
	}
	if len(plaintext) == 0 || len(plaintext) > 1024 {
		return "", ErrInvalidSecret
	}
	t.markSeen(replayKey)
	return string(plaintext), nil
}

func normalizedSecretEnvelope(envelope SecretEnvelope) SecretEnvelope {
	envelope.Version = strings.TrimSpace(envelope.Version)
	envelope.Algorithm = strings.TrimSpace(envelope.Algorithm)
	envelope.KeyID = strings.TrimSpace(envelope.KeyID)
	envelope.ClientPublicKey = strings.TrimSpace(envelope.ClientPublicKey)
	envelope.Salt = strings.TrimSpace(envelope.Salt)
	envelope.Nonce = strings.TrimSpace(envelope.Nonce)
	envelope.Ciphertext = strings.TrimSpace(envelope.Ciphertext)
	return envelope
}

func secretEnvelopeAEAD(sharedSecret []byte, salt []byte, keyID string, aad string) (cipher.AEAD, error) {
	reader := hkdf.New(sha256.New, sharedSecret, salt, []byte(secretTransportInfoPrefix+"\x00"+keyID+"\x00"+aad))
	key := make([]byte, 32)
	if _, err := io.ReadFull(reader, key); err != nil {
		return nil, fmt.Errorf("%w: derive secret transport key", ErrInvalidSecret)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("%w: build secret transport cipher", ErrInvalidSecret)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("%w: build secret transport aead", ErrInvalidSecret)
	}
	return aead, nil
}

func secretEnvelopeReplayKey(envelope SecretEnvelope) string {
	digest := sha256.Sum256([]byte(envelope.KeyID + "\x00" + envelope.ClientPublicKey + "\x00" + envelope.Nonce + "\x00" + envelope.Ciphertext))
	return base64.RawURLEncoding.EncodeToString(digest[:])
}

func (t *SecretTransport) replayed(key string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.now().UTC()
	for replayKey, expiresAt := range t.seen {
		if !expiresAt.After(now) {
			delete(t.seen, replayKey)
		}
	}
	_, ok := t.seen[key]
	return ok
}

func (t *SecretTransport) markSeen(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.seen[key] = t.now().UTC().Add(t.replayTTL)
}

func decodeBase64URL(value string) ([]byte, error) {
	return base64.RawURLEncoding.Strict().DecodeString(value)
}
