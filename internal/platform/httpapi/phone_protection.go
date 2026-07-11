package httpapi

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
)

const (
	PhoneVerificationProviderDebug = "debug"
	phoneDigestPrefix              = "v1:hmac-sha256:phone:"
	phoneCodeDigestPrefix          = "v1:hmac-sha256:code:"
	phoneProtectionKeyMinBytes     = 32
)

var ErrPhoneProtectionInvalid = errors.New("phone protection configuration or input is invalid")

type PhoneProtector interface {
	PhoneDigest(phone string) (string, error)
	CodeDigest(phoneDigest string, purpose string, code string) (string, error)
}

type PhoneVerificationSender interface {
	Send(context.Context, string, string, string) error
	Kind() string
}

type HMACPhoneProtector struct {
	phoneKey []byte
	codeKey  []byte
}

func NewHMACPhoneProtector(phoneKey []byte, codeKey []byte) *HMACPhoneProtector {
	return &HMACPhoneProtector{
		phoneKey: append([]byte(nil), phoneKey...),
		codeKey:  append([]byte(nil), codeKey...),
	}
}

func (p *HMACPhoneProtector) PhoneDigest(phone string) (string, error) {
	if err := p.validateKeys(); err != nil {
		return "", err
	}
	phone = strings.TrimSpace(phone)
	normalized, ok := normalizeAppPhone(phone)
	if !ok || normalized != phone {
		return "", ErrPhoneProtectionInvalid
	}
	return phoneDigestPrefix + hmacHex(p.phoneKey, "platform-phone-digest\x00v1\x00"+phone), nil
}

func (p *HMACPhoneProtector) CodeDigest(phoneDigest string, purpose string, code string) (string, error) {
	if err := p.validateKeys(); err != nil {
		return "", err
	}
	phoneDigest = strings.TrimSpace(phoneDigest)
	purpose = strings.TrimSpace(purpose)
	code = strings.TrimSpace(code)
	if !strings.HasPrefix(phoneDigest, phoneDigestPrefix) || purpose == "" || code == "" {
		return "", ErrPhoneProtectionInvalid
	}
	payload := strings.Join([]string{"platform-phone-code-digest", "v1", phoneDigest, purpose, code}, "\x00")
	return phoneCodeDigestPrefix + hmacHex(p.codeKey, payload), nil
}

func (p *HMACPhoneProtector) validateKeys() error {
	if p == nil || len(p.phoneKey) < phoneProtectionKeyMinBytes || len(p.codeKey) < phoneProtectionKeyMinBytes || bytes.Equal(p.phoneKey, p.codeKey) {
		return ErrPhoneProtectionInvalid
	}
	return nil
}

func hmacHex(key []byte, payload string) string {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

type DebugPhoneVerificationSender struct{}

func NewDebugPhoneVerificationSender() *DebugPhoneVerificationSender {
	return &DebugPhoneVerificationSender{}
}

func (*DebugPhoneVerificationSender) Send(_ context.Context, phone string, purpose string, code string) error {
	if strings.TrimSpace(phone) == "" || strings.TrimSpace(purpose) == "" || strings.TrimSpace(code) == "" {
		return ErrPhoneProtectionInvalid
	}
	return nil
}

func (*DebugPhoneVerificationSender) Kind() string {
	return PhoneVerificationProviderDebug
}
