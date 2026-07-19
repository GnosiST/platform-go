package credentialauth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	identifierHashPrefix      = "v1:hmac-sha256:identifier:"
	smsOTPHashPrefix          = "v1:hmac-sha256:sms-otp:"
	identifierHashKeyMinBytes = 32
)

type IdentifierHasher interface {
	HashIdentifier(IdentifierType, string) (string, error)
}

type HMACIdentifierHasher struct {
	key []byte
}

func NewHMACIdentifierHasher(key []byte) (*HMACIdentifierHasher, error) {
	if len(key) < identifierHashKeyMinBytes {
		return nil, fmt.Errorf("%w: identifier hash key is too short", ErrInvalidInput)
	}
	return &HMACIdentifierHasher{key: append([]byte(nil), key...)}, nil
}

func (h *HMACIdentifierHasher) HashIdentifier(identifierType IdentifierType, normalizedValue string) (string, error) {
	if h == nil || len(h.key) < identifierHashKeyMinBytes {
		return "", fmt.Errorf("%w: identifier hasher is not configured", ErrInvalidInput)
	}
	identifierType = IdentifierType(strings.TrimSpace(string(identifierType)))
	normalizedValue = strings.TrimSpace(normalizedValue)
	if !validIdentifierType(identifierType) || normalizedValue == "" || !utf8.ValidString(normalizedValue) || strings.IndexFunc(normalizedValue, unicode.IsControl) >= 0 {
		return "", fmt.Errorf("%w: identifier hash input is invalid", ErrInvalidInput)
	}
	keyID := sha256Hex("platform-credential-auth-identifier-key-id\x00v1\x00", h.key)
	mac := hmac.New(sha256.New, h.key)
	_, _ = mac.Write([]byte("platform-credential-auth-identifier\x00v1\x00"))
	_, _ = mac.Write([]byte(identifierType))
	_, _ = fmt.Fprintf(mac, "\x00%d:", len(normalizedValue))
	_, _ = mac.Write([]byte(normalizedValue))
	return identifierHashPrefix + keyID + ":" + hex.EncodeToString(mac.Sum(nil)), nil
}

func (h *HMACIdentifierHasher) HashSMSOTP(phoneHash string, challengeID string, code string) (string, error) {
	if h == nil || len(h.key) < identifierHashKeyMinBytes {
		return "", fmt.Errorf("%w: sms otp hasher is not configured", ErrInvalidInput)
	}
	phoneHash = strings.TrimSpace(phoneHash)
	challengeID = strings.TrimSpace(challengeID)
	code = strings.TrimSpace(code)
	if phoneHash == "" || challengeID == "" || code == "" || !utf8.ValidString(code) || strings.IndexFunc(code, unicode.IsControl) >= 0 {
		return "", fmt.Errorf("%w: sms otp hash input is invalid", ErrInvalidInput)
	}
	keyID := sha256Hex("platform-credential-auth-sms-otp-key-id\x00v1\x00", h.key)
	mac := hmac.New(sha256.New, h.key)
	_, _ = mac.Write([]byte("platform-credential-auth-sms-otp\x00v1\x00"))
	_, _ = mac.Write([]byte(phoneHash))
	_, _ = fmt.Fprintf(mac, "\x00%d:", len(challengeID))
	_, _ = mac.Write([]byte(challengeID))
	_, _ = fmt.Fprintf(mac, "\x00%d:", len(code))
	_, _ = mac.Write([]byte(code))
	return smsOTPHashPrefix + keyID + ":" + hex.EncodeToString(mac.Sum(nil)), nil
}

func sha256Hex(domain string, key []byte) string {
	digest := sha256.New()
	_, _ = digest.Write([]byte(domain))
	_, _ = digest.Write(key)
	return hex.EncodeToString(digest.Sum(nil))
}
