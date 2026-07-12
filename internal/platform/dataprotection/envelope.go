package dataprotection

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	envelopePrefix  = "pgo:enc:v1:"
	envelopeVersion = 1
	algorithmAESGCM = "AES-256-GCM"
	blindIndexV1    = "hmac-sha256-v1"
)

var ErrInvalidEnvelope = errors.New("invalid data protection envelope")

type blindIndexEnvelopeV1 struct {
	Version        string `json:"version"`
	KeyID          string `json:"keyId"`
	KeyFingerprint string `json:"keyFingerprint"`
	Namespace      string `json:"namespace"`
	Digest         string `json:"digest"`
}

type envelopeV1 struct {
	Version                  int                  `json:"version"`
	Format                   string               `json:"format"`
	Algorithm                string               `json:"algorithm"`
	EncryptionKeyID          string               `json:"encryptionKeyId"`
	EncryptionKeyFingerprint string               `json:"encryptionKeyFingerprint"`
	SchemaVersion            uint32               `json:"schemaVersion"`
	Normalization            string               `json:"normalization"`
	BlindIndex               blindIndexEnvelopeV1 `json:"blindIndex,omitempty"`
	Nonce                    string               `json:"nonce"`
	Ciphertext               string               `json:"ciphertext"`
	Authentication           string               `json:"authentication"`
}

func encodeEnvelope(envelope envelopeV1) (string, error) {
	payload, err := json.Marshal(envelope)
	if err != nil {
		return "", fmt.Errorf("%w: encoding failed", ErrInvalidEnvelope)
	}
	return envelopePrefix + base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodeEnvelope(raw string) (envelopeV1, error) {
	if !strings.HasPrefix(raw, envelopePrefix) || len(raw) <= len(envelopePrefix) {
		return envelopeV1{}, ErrInvalidEnvelope
	}
	payload, err := base64.RawURLEncoding.Strict().DecodeString(strings.TrimPrefix(raw, envelopePrefix))
	if err != nil {
		return envelopeV1{}, ErrInvalidEnvelope
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var envelope envelopeV1
	if err := decoder.Decode(&envelope); err != nil {
		return envelopeV1{}, ErrInvalidEnvelope
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return envelopeV1{}, ErrInvalidEnvelope
	}
	return envelope, nil
}
