package httpapi

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"platform-go/internal/platform/adminresource"
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
	KeyIDs() (PhoneProtectionKeyIDs, error)
}

type PhoneProtectionKeyIDs struct {
	Phone string
	Code  string
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
	keyIDs, err := p.KeyIDs()
	if err != nil {
		return "", err
	}
	phone = strings.TrimSpace(phone)
	normalized, ok := normalizeAppPhone(phone)
	if !ok || normalized != phone {
		return "", ErrPhoneProtectionInvalid
	}
	return phoneDigestPrefix + keyIDs.Phone + ":" + hmacHex(p.phoneKey, "platform-phone-digest\x00v1\x00"+phone), nil
}

func (p *HMACPhoneProtector) CodeDigest(phoneDigest string, purpose string, code string) (string, error) {
	keyIDs, err := p.KeyIDs()
	if err != nil {
		return "", err
	}
	phoneDigest = strings.TrimSpace(phoneDigest)
	purpose = strings.TrimSpace(purpose)
	code = strings.TrimSpace(code)
	phoneKeyID, _, ok := parsePhoneProtectionDigest(phoneDigest, phoneDigestPrefix)
	if !ok || phoneKeyID != keyIDs.Phone || purpose == "" || code == "" {
		return "", ErrPhoneProtectionInvalid
	}
	payload := strings.Join([]string{"platform-phone-code-digest", "v1", phoneDigest, purpose, code}, "\x00")
	return phoneCodeDigestPrefix + keyIDs.Code + ":" + hmacHex(p.codeKey, payload), nil
}

func (p *HMACPhoneProtector) KeyIDs() (PhoneProtectionKeyIDs, error) {
	if err := p.validateKeys(); err != nil {
		return PhoneProtectionKeyIDs{}, err
	}
	return PhoneProtectionKeyIDs{
		Phone: sha256Hex("platform-phone-protection-key-id\x00v1\x00phone\x00", p.phoneKey),
		Code:  sha256Hex("platform-phone-protection-key-id\x00v1\x00code\x00", p.codeKey),
	}, nil
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

func sha256Hex(domain string, key []byte) string {
	digest := sha256.New()
	_, _ = digest.Write([]byte(domain))
	_, _ = digest.Write(key)
	return hex.EncodeToString(digest.Sum(nil))
}

func parsePhoneProtectionDigest(value string, prefix string) (string, string, bool) {
	if value != strings.TrimSpace(value) || !strings.HasPrefix(value, prefix) {
		return "", "", false
	}
	remainder := strings.TrimPrefix(value, prefix)
	parts := strings.Split(remainder, ":")
	if len(parts) != 2 || !fullSHA256Hex(parts[0]) || !fullSHA256Hex(parts[1]) {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func fullSHA256Hex(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}

func ValidatePhoneProtectionHistory(resources *adminresource.Store, protector PhoneProtector) error {
	if resources == nil {
		return errors.New("phone protection history requires admin resources")
	}
	type protectedField struct {
		resource string
		field    string
		prefix   string
		keyID    func(PhoneProtectionKeyIDs) string
	}
	fields := []protectedField{
		{resource: appPhoneVerificationsResource, field: "phoneHash", prefix: phoneDigestPrefix, keyID: func(ids PhoneProtectionKeyIDs) string { return ids.Phone }},
		{resource: appPhoneVerificationsResource, field: "codeHash", prefix: phoneCodeDigestPrefix, keyID: func(ids PhoneProtectionKeyIDs) string { return ids.Code }},
		{resource: appPhoneBindingsResource, field: "phoneHash", prefix: phoneDigestPrefix, keyID: func(ids PhoneProtectionKeyIDs) string { return ids.Phone }},
	}
	recordsByResource := make(map[string][]adminresource.Record, 2)
	for _, resource := range []string{appPhoneVerificationsResource, appPhoneBindingsResource} {
		records, err := resources.List(resource)
		if errors.Is(err, adminresource.ErrUnknownResource) {
			continue
		}
		if err != nil {
			return fmt.Errorf("load phone protection history for %s: %w", resource, err)
		}
		recordsByResource[resource] = records
	}
	if len(recordsByResource) == 0 {
		return nil
	}
	if protector == nil {
		return errors.New("phone protection history requires a configured protector")
	}
	keyIDs, err := protector.KeyIDs()
	if err != nil {
		return fmt.Errorf("resolve phone protection key IDs: %w", err)
	}
	for _, field := range fields {
		for _, record := range recordsByResource[field.resource] {
			keyID, _, ok := parsePhoneProtectionDigest(record.Values[field.field], field.prefix)
			if !ok {
				return fmt.Errorf("phone protection history %s record %s field %s uses a legacy or malformed digest", field.resource, record.ID, field.field)
			}
			if keyID != field.keyID(keyIDs) {
				return fmt.Errorf("phone protection history %s record %s field %s key ID does not match current configuration", field.resource, record.ID, field.field)
			}
		}
	}
	return nil
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
