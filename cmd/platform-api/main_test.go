package main

import (
	"context"
	"net/http"
	"testing"

	"platform-go/internal/apps"
	"platform-go/internal/platform/bootstrap"
	"platform-go/internal/platform/capability"
	"platform-go/internal/platform/config"
	"platform-go/internal/platform/httpapi"
)

func TestStartRetentionRuntimeDisabledHasZeroSideEffects(t *testing.T) {
	called := false
	open := func(config.Config, ...capability.Manifest) (*bootstrap.DataLifecycle, error) {
		called = true
		return nil, nil
	}
	lifecycle, scheduler, err := startRetentionRuntime(context.Background(), config.Config{}, nil, open)
	if err != nil || lifecycle != nil || scheduler != nil {
		t.Fatalf("startRetentionRuntime(disabled) = %#v, %#v, %v", lifecycle, scheduler, err)
	}
	if called {
		t.Fatal("disabled retention runtime opened lifecycle storage")
	}
}

func TestRunHTTPServerStopsOnContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := runHTTPServer(ctx, "127.0.0.1:0", http.NewServeMux()); err != nil {
		t.Fatalf("runHTTPServer() error = %v", err)
	}
}

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
