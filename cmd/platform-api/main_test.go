package main

import (
	"testing"

	"platform-go/internal/platform/config"
	"platform-go/internal/platform/httpapi"
)

func TestPhoneVerificationSenderFromConfigCreatesOnlyCanonicalDebugSender(t *testing.T) {
	debug := phoneVerificationSenderFromConfig(config.Config{PhoneVerificationProvider: httpapi.PhoneVerificationProviderDebug})
	if _, ok := debug.(*httpapi.DebugPhoneVerificationSender); !ok {
		t.Fatalf("phoneVerificationSenderFromConfig(debug) = %T, want built-in debug sender", debug)
	}
	for _, provider := range []string{"sms-vendor", " DEBUG ", "", "unknown"} {
		if sender := phoneVerificationSenderFromConfig(config.Config{PhoneVerificationProvider: provider}); sender != nil {
			t.Fatalf("phoneVerificationSenderFromConfig(%q) = %T, want nil", provider, sender)
		}
	}
}
