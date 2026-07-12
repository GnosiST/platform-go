package main

import (
	"strings"
	"testing"

	"platform-go/internal/apps"
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

func TestPlatformHasNoLocalPasswordProviderOrPasswordPersistencePath(t *testing.T) {
	if err := validateCredentialBoundary(apps.DefaultManifests()); err != nil {
		t.Fatalf("validateCredentialBoundary() error = %v", err)
	}
	for _, manifest := range apps.DefaultManifests() {
		for _, provider := range manifest.AuthProviders {
			if strings.Contains(strings.ToLower(provider.Kind), "password") || strings.Contains(strings.ToLower(provider.ID), "password") {
				t.Fatalf("default manifest %q exposes local password provider %+v", manifest.ID, provider)
			}
		}
		for _, resource := range manifest.Admin.Resources {
			for _, field := range resource.Fields {
				if strings.Contains(strings.ToLower(field.Key), "password") {
					t.Fatalf("default resource %q persists password-like field %q", resource.Resource, field.Key)
				}
			}
		}
	}
}
