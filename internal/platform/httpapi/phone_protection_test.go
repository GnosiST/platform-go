package httpapi

import (
	"strings"
	"testing"
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
