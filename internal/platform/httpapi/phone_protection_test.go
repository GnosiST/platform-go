package httpapi

import (
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"platform-go/internal/platform/adminresource"
)

func TestHMACPhoneProtectorSeparatesPhoneAndCodeDomains(t *testing.T) {
	protector := NewHMACPhoneProtector([]byte(strings.Repeat("p", 32)), []byte(strings.Repeat("c", 32)))
	phoneDigest, err := protector.PhoneDigest("+8613800138000")
	if err != nil {
		t.Fatalf("PhoneDigest() error = %v", err)
	}
	codeDigest, err := protector.CodeDigest(phoneDigest, "bind", "123456")
	if err != nil {
		t.Fatalf("CodeDigest() error = %v", err)
	}
	if phoneDigest == codeDigest {
		t.Fatal("digest domains collapsed")
	}
	if !strings.HasPrefix(phoneDigest, "v1:hmac-sha256:phone:") {
		t.Fatalf("phone digest = %q, want versioned phone HMAC", phoneDigest)
	}
	if !strings.HasPrefix(codeDigest, "v1:hmac-sha256:code:") {
		t.Fatalf("code digest = %q, want versioned code HMAC", codeDigest)
	}
	if strings.Contains(phoneDigest, "+8613800138000") || strings.Contains(codeDigest, "123456") {
		t.Fatalf("digests leaked source values: phone=%q code=%q", phoneDigest, codeDigest)
	}
	keyIDs, err := protector.KeyIDs()
	if err != nil {
		t.Fatalf("KeyIDs() error = %v", err)
	}
	for label, keyID := range map[string]string{"phone": keyIDs.Phone, "code": keyIDs.Code} {
		decoded, err := hex.DecodeString(keyID)
		if err != nil || len(decoded) != 32 {
			t.Fatalf("%s key ID = %q, want full SHA-256 hex", label, keyID)
		}
	}
	if !strings.HasPrefix(phoneDigest, "v1:hmac-sha256:phone:"+keyIDs.Phone+":") {
		t.Fatalf("phone digest = %q, want embedded phone key ID %q", phoneDigest, keyIDs.Phone)
	}
	if !strings.HasPrefix(codeDigest, "v1:hmac-sha256:code:"+keyIDs.Code+":") {
		t.Fatalf("code digest = %q, want embedded code key ID %q", codeDigest, keyIDs.Code)
	}
}

func TestHMACPhoneProtectorUsesNormalizedPhoneValue(t *testing.T) {
	formatted, ok := normalizeAppPhone("+86 (138) 0013-8000")
	if !ok {
		t.Fatal("normalizeAppPhone(formatted) = false")
	}
	compact, ok := normalizeAppPhone("+8613800138000")
	if !ok || formatted != compact {
		t.Fatalf("normalized values = %q and %q, want equal", formatted, compact)
	}
	protector := NewHMACPhoneProtector([]byte(strings.Repeat("p", 32)), []byte(strings.Repeat("c", 32)))
	formattedDigest, err := protector.PhoneDigest(formatted)
	if err != nil {
		t.Fatalf("PhoneDigest(formatted) error = %v", err)
	}
	compactDigest, err := protector.PhoneDigest(compact)
	if err != nil {
		t.Fatalf("PhoneDigest(compact) error = %v", err)
	}
	if formattedDigest != compactDigest {
		t.Fatalf("digests = %q and %q, want equal", formattedDigest, compactDigest)
	}
}

func TestHMACPhoneProtectorRejectsMissingKeysAndInputs(t *testing.T) {
	tests := []struct {
		name      string
		protector *HMACPhoneProtector
		phone     string
		purpose   string
		code      string
	}{
		{name: "missing phone key", protector: NewHMACPhoneProtector(nil, []byte(strings.Repeat("c", 32))), phone: "+8613800138000"},
		{name: "missing code key", protector: NewHMACPhoneProtector([]byte(strings.Repeat("p", 32)), nil), phone: "+8613800138000", purpose: "bind", code: "123456"},
		{name: "missing phone", protector: NewHMACPhoneProtector([]byte(strings.Repeat("p", 32)), []byte(strings.Repeat("c", 32)))},
		{name: "missing purpose", protector: NewHMACPhoneProtector([]byte(strings.Repeat("p", 32)), []byte(strings.Repeat("c", 32))), phone: "+8613800138000", code: "123456"},
		{name: "missing code", protector: NewHMACPhoneProtector([]byte(strings.Repeat("p", 32)), []byte(strings.Repeat("c", 32))), phone: "+8613800138000", purpose: "bind"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.purpose == "" && tt.code == "" {
				if _, err := tt.protector.PhoneDigest(tt.phone); err == nil {
					t.Fatal("PhoneDigest() error = nil, want invalid input")
				}
				return
			}
			phoneDigest, _ := tt.protector.PhoneDigest(tt.phone)
			if _, err := tt.protector.CodeDigest(phoneDigest, tt.purpose, tt.code); err == nil {
				t.Fatal("CodeDigest() error = nil, want invalid input")
			}
		})
	}
}

func TestHMACPhoneProtectorRejectsShortAndEqualKeys(t *testing.T) {
	for _, tt := range []struct {
		name     string
		phoneKey []byte
		codeKey  []byte
	}{
		{name: "short phone key", phoneKey: []byte("short"), codeKey: []byte(strings.Repeat("c", 32))},
		{name: "short code key", phoneKey: []byte(strings.Repeat("p", 32)), codeKey: []byte("short")},
		{name: "equal keys", phoneKey: []byte(strings.Repeat("k", 32)), codeKey: []byte(strings.Repeat("k", 32))},
	} {
		t.Run(tt.name, func(t *testing.T) {
			protector := NewHMACPhoneProtector(tt.phoneKey, tt.codeKey)
			if _, err := protector.PhoneDigest("13800138000"); err == nil {
				t.Fatal("PhoneDigest() error = nil, want unsafe key rejection")
			}
		})
	}
}

func TestHMACPhoneProtectorChangesDigestAcrossInputsAndKeys(t *testing.T) {
	first := NewHMACPhoneProtector([]byte(strings.Repeat("p", 32)), []byte(strings.Repeat("c", 32)))
	second := NewHMACPhoneProtector([]byte(strings.Repeat("q", 32)), []byte(strings.Repeat("d", 32)))
	phoneDigest, _ := first.PhoneDigest("13800138000")
	otherPhoneDigest, _ := first.PhoneDigest("13900139000")
	otherKeyDigest, _ := second.PhoneDigest("13800138000")
	if phoneDigest == otherPhoneDigest || phoneDigest == otherKeyDigest {
		t.Fatalf("phone digests did not separate input/key: %q %q %q", phoneDigest, otherPhoneDigest, otherKeyDigest)
	}
	codeDigest, _ := first.CodeDigest(phoneDigest, "bind", "123456")
	otherPurposeDigest, _ := first.CodeDigest(phoneDigest, "login", "123456")
	otherCodeDigest, _ := first.CodeDigest(phoneDigest, "bind", "654321")
	otherCodeKeyDigest, _ := second.CodeDigest(phoneDigest, "bind", "123456")
	if codeDigest == otherPurposeDigest || codeDigest == otherCodeDigest || codeDigest == otherCodeKeyDigest {
		t.Fatalf("code digests did not separate purpose/code/key: %q %q %q %q", codeDigest, otherPurposeDigest, otherCodeDigest, otherCodeKeyDigest)
	}
}

func TestValidatePhoneProtectionHistoryAcceptsNoRowsAndMatchingKeyIDs(t *testing.T) {
	protector := NewHMACPhoneProtector([]byte(strings.Repeat("p", 32)), []byte(strings.Repeat("c", 32)))
	empty := appPhoneHistoryStoreForTest(t)
	if err := ValidatePhoneProtectionHistory(empty, protector); err != nil {
		t.Fatalf("ValidatePhoneProtectionHistory(empty) error = %v", err)
	}
	withHistory := appPhoneHistoryStoreForTest(t)
	seedAppPhoneHistoryForTest(t, withHistory, protector)
	if err := ValidatePhoneProtectionHistory(withHistory, protector); err != nil {
		t.Fatalf("ValidatePhoneProtectionHistory(matching) error = %v", err)
	}
}

func TestValidatePhoneProtectionHistoryRejectsKeyChanges(t *testing.T) {
	original := NewHMACPhoneProtector([]byte(strings.Repeat("p", 32)), []byte(strings.Repeat("c", 32)))
	store := appPhoneHistoryStoreForTest(t)
	seedAppPhoneHistoryForTest(t, store, original)
	for _, tt := range []struct {
		name      string
		protector *HMACPhoneProtector
	}{
		{name: "phone key", protector: NewHMACPhoneProtector([]byte(strings.Repeat("q", 32)), []byte(strings.Repeat("c", 32)))},
		{name: "code key", protector: NewHMACPhoneProtector([]byte(strings.Repeat("p", 32)), []byte(strings.Repeat("d", 32)))},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePhoneProtectionHistory(store, tt.protector)
			if err == nil || !strings.Contains(err.Error(), "key ID") {
				t.Fatalf("ValidatePhoneProtectionHistory(changed %s) error = %v, want startup rejection", tt.name, err)
			}
		})
	}
}

func TestValidatePhoneProtectionHistoryRejectsLegacyDigestFormat(t *testing.T) {
	protector := NewHMACPhoneProtector([]byte(strings.Repeat("p", 32)), []byte(strings.Repeat("c", 32)))
	keyIDs, err := protector.KeyIDs()
	if err != nil {
		t.Fatalf("KeyIDs() error = %v", err)
	}
	phoneDigest, err := protector.PhoneDigest("13800138000")
	if err != nil {
		t.Fatalf("PhoneDigest() error = %v", err)
	}
	for _, tt := range []struct {
		name      string
		phoneHash string
	}{
		{name: "legacy without key ID", phoneHash: "v1:hmac-sha256:phone:" + strings.Repeat("a", 64)},
		{name: "padded current digest", phoneHash: " " + phoneDigest + " "},
		{name: "malformed key ID", phoneHash: phoneDigestPrefix + keyIDs.Phone[:63] + ":" + strings.Repeat("a", 64)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			store := appPhoneHistoryStoreForTest(t)
			_, err := store.CreateInternal(appPhoneBindingsResource, adminresource.WriteInput{
				Code: "phone-binding-legacy", Name: "Legacy Phone Binding", Status: "enabled",
				Values: map[string]string{
					"appUsername": "guest-alpha",
					"maskedPhone": "138****8000",
					"phoneHash":   tt.phoneHash,
					"boundAt":     time.Date(2026, 7, 12, 10, 0, 0, 0, time.UTC).Format(time.RFC3339),
				},
			})
			if err != nil {
				t.Fatalf("seed legacy binding: %v", err)
			}
			if err := ValidatePhoneProtectionHistory(store, protector); err == nil || !strings.Contains(err.Error(), "legacy or malformed") {
				t.Fatalf("ValidatePhoneProtectionHistory(%s) error = %v, want format rejection", tt.name, err)
			}
		})
	}
}

func appPhoneHistoryStoreForTest(t *testing.T) *adminresource.Store {
	t.Helper()
	return adminresource.NewStoreFromCapabilities(capabilitiesFromConfigForTest(t, []string{
		"dictionary", "tenant", "identity", "session", "rbac", "audit", "app-phone",
	}))
}

func seedAppPhoneHistoryForTest(t *testing.T, store *adminresource.Store, protector PhoneProtector) {
	t.Helper()
	phoneDigest, err := protector.PhoneDigest("13800138000")
	if err != nil {
		t.Fatalf("PhoneDigest() error = %v", err)
	}
	codeDigest, err := protector.CodeDigest(phoneDigest, appPhoneVerificationPurpose, "123456")
	if err != nil {
		t.Fatalf("CodeDigest() error = %v", err)
	}
	now := time.Date(2026, 7, 12, 10, 0, 0, 0, time.UTC)
	verification, err := store.CreateInternal(appPhoneVerificationsResource, adminresource.WriteInput{
		Code: "phone-verification-history", Name: "Phone Verification History", Status: "verified",
		Values: map[string]string{
			"appUsername": "guest-alpha",
			"maskedPhone": "138****8000",
			"phoneHash":   phoneDigest,
			"purpose":     appPhoneVerificationPurpose,
			"codeHash":    codeDigest,
			"requestedAt": now.Format(time.RFC3339),
			"expiresAt":   now.Add(appPhoneVerificationTTL).Format(time.RFC3339),
			"verifiedAt":  now.Add(time.Minute).Format(time.RFC3339),
		},
	})
	if err != nil {
		t.Fatalf("seed verification history: %v", err)
	}
	_, err = store.CreateInternal(appPhoneBindingsResource, adminresource.WriteInput{
		Code: "phone-binding-history", Name: "Phone Binding History", Status: "enabled",
		Values: map[string]string{
			"appUsername":    "guest-alpha",
			"maskedPhone":    "138****8000",
			"phoneHash":      phoneDigest,
			"boundAt":        now.Add(time.Minute).Format(time.RFC3339),
			"verificationId": verification.ID,
		},
	})
	if err != nil {
		t.Fatalf("seed binding history: %v", err)
	}
}
