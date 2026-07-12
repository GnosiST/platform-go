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
	envelopeFamilyPrefix = "pgo:enc:"
	envelopePrefix       = "pgo:enc:v1:"
	envelopeVersion      = 1
	algorithmAESGCM      = "AES-256-GCM"
	blindIndexV1         = "hmac-sha256-v1"
)

var ErrInvalidEnvelope = errors.New("invalid data protection envelope")

type EnvelopeShape string

const (
	EnvelopeShapeNone      EnvelopeShape = "none"
	EnvelopeShapeCurrent   EnvelopeShape = "current"
	EnvelopeShapeForeign   EnvelopeShape = "foreign"
	EnvelopeShapeMalformed EnvelopeShape = "malformed"
)

func ClassifyEnvelopeShape(value string) EnvelopeShape {
	if !strings.HasPrefix(value, envelopeFamilyPrefix) {
		return EnvelopeShapeNone
	}
	version, _, found := strings.Cut(strings.TrimPrefix(value, envelopeFamilyPrefix), ":")
	if !found || version == "" {
		return EnvelopeShapeMalformed
	}
	if version == "v1" {
		return EnvelopeShapeCurrent
	}
	if len(version) > 1 && version[0] == 'v' {
		for _, digit := range version[1:] {
			if digit < '0' || digit > '9' {
				return EnvelopeShapeMalformed
			}
		}
		return EnvelopeShapeForeign
	}
	return EnvelopeShapeMalformed
}

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
