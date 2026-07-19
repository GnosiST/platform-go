package credentialauth

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"testing"
	"time"
)

func TestSecretTransportDecryptsHybridEnvelopeAndRejectsReplay(t *testing.T) {
	now := time.Date(2026, time.July, 19, 10, 0, 0, 0, time.UTC)
	transport, err := NewSecretTransport(SecretTransportOptions{
		KeyID: "auth-key-v1",
		Now:   func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewSecretTransport() error = %v", err)
	}
	envelope := encryptSecretForTransportTest(t, transport.PublicKey(), "username-password\x00password\x00username", "correct-password")

	plaintext, err := transport.Decrypt(context.Background(), envelope, "username-password\x00password\x00username")
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if plaintext != "correct-password" {
		t.Fatalf("Decrypt() = %q, want exact secret", plaintext)
	}
	if _, err := transport.Decrypt(context.Background(), envelope, "username-password\x00password\x00username"); err == nil {
		t.Fatal("Decrypt(replay) error = nil, want replay rejection")
	}
}

func TestSecretTransportRejectsAADMismatch(t *testing.T) {
	transport, err := NewSecretTransport(SecretTransportOptions{KeyID: "auth-key-v1"})
	if err != nil {
		t.Fatalf("NewSecretTransport() error = %v", err)
	}
	envelope := encryptSecretForTransportTest(t, transport.PublicKey(), "username-password\x00password\x00username", "correct-password")

	if _, err := transport.Decrypt(context.Background(), envelope, "email-password\x00password\x00email"); err == nil {
		t.Fatal("Decrypt(aad mismatch) error = nil, want rejection")
	}
}

func TestDecodeSecretTransportPrivateKeyValidatesP256Scalar(t *testing.T) {
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	encoded := base64.RawURLEncoding.EncodeToString(key.Bytes())
	decoded, err := DecodeSecretTransportPrivateKey(encoded)
	if err != nil {
		t.Fatalf("DecodeSecretTransportPrivateKey() error = %v", err)
	}
	if string(decoded) != string(key.Bytes()) {
		t.Fatal("DecodeSecretTransportPrivateKey() returned different key bytes")
	}
	if _, err := DecodeSecretTransportPrivateKey(base64.RawURLEncoding.EncodeToString([]byte("short"))); err == nil {
		t.Fatal("DecodeSecretTransportPrivateKey(short) error = nil, want rejection")
	}
}

func encryptSecretForTransportTest(t *testing.T, publicKey SecretTransportPublicKey, aad string, plaintext string) SecretEnvelope {
	t.Helper()
	serverPublic, err := base64.RawURLEncoding.Strict().DecodeString(publicKey.PublicKey)
	if err != nil {
		t.Fatalf("decode server public key: %v", err)
	}
	peer, err := ecdh.P256().NewPublicKey(serverPublic)
	if err != nil {
		t.Fatalf("server public key: %v", err)
	}
	clientPrivate, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("client private key: %v", err)
	}
	shared, err := clientPrivate.ECDH(peer)
	if err != nil {
		t.Fatalf("ECDH() error = %v", err)
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		t.Fatalf("rand salt: %v", err)
	}
	aead, err := secretEnvelopeAEAD(shared, salt, publicKey.KeyID, aad)
	if err != nil {
		t.Fatalf("secretEnvelopeAEAD() error = %v", err)
	}
	nonce := make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatalf("rand nonce: %v", err)
	}
	ciphertext := aead.Seal(nil, nonce, []byte(plaintext), []byte(aad))
	return SecretEnvelope{
		Version:         publicKey.Version,
		Algorithm:       publicKey.Algorithm,
		KeyID:           publicKey.KeyID,
		ClientPublicKey: base64.RawURLEncoding.EncodeToString(clientPrivate.PublicKey().Bytes()),
		Salt:            base64.RawURLEncoding.EncodeToString(salt),
		Nonce:           base64.RawURLEncoding.EncodeToString(nonce),
		Ciphertext:      base64.RawURLEncoding.EncodeToString(ciphertext),
	}
}
