package main

import (
	"testing"

	"platform-go/internal/apps"
	"platform-go/internal/platform/capability"
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

func TestPlatformRejectsLocalPasswordProviderWithoutInferringFieldSemanticsFromNames(t *testing.T) {
	if err := validateCredentialBoundary(apps.DefaultManifests()); err != nil {
		t.Fatalf("validateCredentialBoundary() error = %v", err)
	}
	customFieldManifest := capability.Manifest{
		ID: "custom-fields",
		Admin: capability.AdminSurface{Resources: []capability.AdminResource{{
			Resource: "custom-records",
			Fields: []capability.AdminField{{
				Key: "passwordHint", Source: "values", Sensitivity: capability.FieldSensitivityPublic,
				StorageMode: capability.FieldStoragePlain, ResponseMode: capability.FieldProjectionFull, ExportMode: capability.FieldProjectionFull,
			}},
		}}},
	}
	if err := validateCredentialBoundary([]capability.Manifest{customFieldManifest}); err != nil {
		t.Fatalf("validateCredentialBoundary(custom field) error = %v", err)
	}
	passwordlessProviderManifest := capability.Manifest{
		ID: "passwordless-auth", AuthProviders: []capability.AuthProvider{{ID: "passwordless-oidc", Kind: "oidc"}},
	}
	if err := validateCredentialBoundary([]capability.Manifest{passwordlessProviderManifest}); err != nil {
		t.Fatalf("validateCredentialBoundary(passwordless provider) error = %v", err)
	}
	passwordProviderManifest := capability.Manifest{
		ID: "local-password", AuthProviders: []capability.AuthProvider{{ID: "custom-login", Kind: " PASSWORD "}},
	}
	if err := validateCredentialBoundary([]capability.Manifest{passwordProviderManifest}); err == nil {
		t.Fatal("validateCredentialBoundary(password provider) error = nil")
	}
}
