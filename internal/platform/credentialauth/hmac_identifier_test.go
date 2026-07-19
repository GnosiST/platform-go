package credentialauth

import (
	"strings"
	"testing"
)

func TestHMACIdentifierHasherBindsSMSOTPToPhoneAndChallenge(t *testing.T) {
	hasher, err := NewHMACIdentifierHasher([]byte(strings.Repeat("i", 32)))
	if err != nil {
		t.Fatalf("NewHMACIdentifierHasher() error = %v", err)
	}
	first, err := hasher.HashSMSOTP("phone-hash-a", "challenge-a", "123456")
	if err != nil {
		t.Fatalf("HashSMSOTP() error = %v", err)
	}
	second, err := hasher.HashSMSOTP("phone-hash-a", "challenge-b", "123456")
	if err != nil {
		t.Fatalf("HashSMSOTP(other challenge) error = %v", err)
	}
	third, err := hasher.HashSMSOTP("phone-hash-b", "challenge-a", "123456")
	if err != nil {
		t.Fatalf("HashSMSOTP(other phone) error = %v", err)
	}
	if first == second || first == third || !strings.HasPrefix(first, smsOTPHashPrefix) {
		t.Fatalf("sms otp digests = %q, %q, %q; want distinct prefixed digests", first, second, third)
	}
}

func TestHMACIdentifierHasherRejectsInvalidSMSOTPInput(t *testing.T) {
	hasher, err := NewHMACIdentifierHasher([]byte(strings.Repeat("i", 32)))
	if err != nil {
		t.Fatalf("NewHMACIdentifierHasher() error = %v", err)
	}
	if _, err := hasher.HashSMSOTP("", "challenge", "123456"); err == nil {
		t.Fatal("HashSMSOTP(empty phone hash) error = nil")
	}
	if _, err := hasher.HashSMSOTP("phone-hash", "", "123456"); err == nil {
		t.Fatal("HashSMSOTP(empty challenge) error = nil")
	}
	if _, err := hasher.HashSMSOTP("phone-hash", "challenge", ""); err == nil {
		t.Fatal("HashSMSOTP(empty code) error = nil")
	}
}
