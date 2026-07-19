package main

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/GnosiST/platform-go/internal/apps"
	"github.com/GnosiST/platform-go/internal/platform/bootstrap"
	"github.com/GnosiST/platform-go/internal/platform/capability"
	"github.com/GnosiST/platform-go/internal/platform/config"
	"github.com/GnosiST/platform-go/internal/platform/httpapi"
	"github.com/GnosiST/platform-go/internal/platform/notification"
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

func TestNotificationSMSSenderFromConfigCreatesOnlyCanonicalMockLocalSender(t *testing.T) {
	mock, err := notificationSMSSenderFromConfig(config.Config{NotificationSMSProvider: notification.SMSProviderMockLocal})
	if err != nil {
		t.Fatalf("notificationSMSSenderFromConfig(mock-local) error = %v", err)
	}
	if _, ok := mock.(*notification.MockLocalSMSSender); !ok {
		t.Fatalf("notificationSMSSenderFromConfig(mock-local) = %T, want built-in mock sender", mock)
	}
	for _, provider := range []string{" MOCK-LOCAL ", "", "debug"} {
		sender, err := notificationSMSSenderFromConfig(config.Config{NotificationSMSProvider: provider})
		if err != nil {
			t.Fatalf("notificationSMSSenderFromConfig(%q) error = %v", provider, err)
		}
		if sender != nil {
			t.Fatalf("notificationSMSSenderFromConfig(%q) = %T, want nil", provider, sender)
		}
	}
}

func TestNotificationSMSSenderFromConfigCreatesVendorDryRunSender(t *testing.T) {
	tests := []struct {
		name        string
		provider    string
		setProvider func(*testing.T)
	}{
		{
			name:     "aliyun",
			provider: notification.SMSProviderAliyun,
			setProvider: func(t *testing.T) {
				t.Setenv(notification.EnvNotificationSMSAliyunRegion, "cn-hangzhou")
				t.Setenv(notification.EnvNotificationSMSAliyunAccessKeyID, "test-access-key")
				t.Setenv(notification.EnvNotificationSMSAliyunSecretKey, "test-secret-key")
				t.Setenv(notification.EnvNotificationSMSSignName, "Platform")
			},
		},
		{
			name:     "tencent",
			provider: notification.SMSProviderTencent,
			setProvider: func(t *testing.T) {
				t.Setenv(notification.EnvNotificationSMSTencentRegion, "ap-guangzhou")
				t.Setenv(notification.EnvNotificationSMSTencentSecretID, "test-secret-id")
				t.Setenv(notification.EnvNotificationSMSTencentSecretKey, "test-secret-key")
				t.Setenv(notification.EnvNotificationSMSTencentSDKAppID, "1400000000")
				t.Setenv(notification.EnvNotificationSMSSignName, "Platform")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(notification.EnvNotificationSMSDryRun, "true")
			tt.setProvider(t)

			sender, err := notificationSMSSenderFromConfig(config.Config{NotificationSMSProvider: tt.provider})
			if err != nil {
				t.Fatalf("notificationSMSSenderFromConfig() error = %v", err)
			}
			if sender == nil || sender.Kind() != tt.provider {
				t.Fatalf("notificationSMSSenderFromConfig() = %T/%v, want %s dry-run sender", sender, sender, tt.provider)
			}
			receipt, err := sender.SendSMS(context.Background(), notification.SMSMessage{
				Recipient:  "+8613800000000",
				TemplateID: "login-template",
				Purpose:    "login",
			})
			if err != nil {
				t.Fatalf("SendSMS() error = %v", err)
			}
			if receipt.Status != notification.SMSDeliveryDryRunAccepted || receipt.Provider != tt.provider {
				t.Fatalf("receipt = %+v, want vendor dry-run receipt", receipt)
			}
		})
	}
}

func TestNotificationSMSSenderFromConfigFailsClosedForLiveSendWithoutImplementation(t *testing.T) {
	t.Setenv(notification.EnvNotificationSMSLiveSendEnabled, "true")
	t.Setenv(notification.EnvNotificationSMSAliyunRegion, "cn-hangzhou")
	t.Setenv(notification.EnvNotificationSMSAliyunAccessKeyID, "test-access-key")
	t.Setenv(notification.EnvNotificationSMSAliyunSecretKey, "test-secret-key")
	t.Setenv(notification.EnvNotificationSMSSignName, "Platform")

	_, err := notificationSMSSenderFromConfig(config.Config{
		RuntimeEnvironment:      config.RuntimeEnvironmentProduction,
		NotificationSMSProvider: notification.SMSProviderAliyun,
	})
	if err == nil || !strings.Contains(err.Error(), "live sending is not implemented") {
		t.Fatalf("notificationSMSSenderFromConfig() error = %v, want live-send implementation failure", err)
	}
}

func TestNotificationSMSSenderFromConfigRejectsMissingVendorConfigWithoutSecretLeak(t *testing.T) {
	secret := "do-not-print-this-secret"
	t.Setenv(notification.EnvNotificationSMSDryRun, "true")
	t.Setenv(notification.EnvNotificationSMSAliyunRegion, "cn-hangzhou")
	t.Setenv(notification.EnvNotificationSMSAliyunSecretKey, secret)
	t.Setenv(notification.EnvNotificationSMSSignName, "Platform")

	_, err := notificationSMSSenderFromConfig(config.Config{NotificationSMSProvider: notification.SMSProviderAliyun})
	if err == nil || !strings.Contains(err.Error(), notification.EnvNotificationSMSAliyunAccessKeyID) {
		t.Fatalf("notificationSMSSenderFromConfig() error = %v, want missing access key", err)
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("notificationSMSSenderFromConfig() leaked secret in error: %v", err)
	}
}

func TestNotificationSMSSenderFromConfigRejectsInvalidSMSRuntimeBoolean(t *testing.T) {
	t.Setenv(notification.EnvNotificationSMSDryRun, "sometimes")

	_, err := notificationSMSSenderFromConfig(config.Config{NotificationSMSProvider: notification.SMSProviderAliyun})
	if err == nil || !strings.Contains(err.Error(), notification.EnvNotificationSMSDryRun+" must be a boolean") {
		t.Fatalf("notificationSMSSenderFromConfig() error = %v, want dry-run boolean rejection", err)
	}
}

func TestNotificationSMSSenderFromConfigTreatsLiveDisabledAsDryRun(t *testing.T) {
	t.Setenv(notification.EnvNotificationSMSLiveSendEnabled, "false")
	t.Setenv(notification.EnvNotificationSMSDryRun, "false")
	t.Setenv(notification.EnvNotificationSMSTencentRegion, "ap-guangzhou")
	t.Setenv(notification.EnvNotificationSMSTencentSecretID, "test-secret-id")
	t.Setenv(notification.EnvNotificationSMSTencentSecretKey, "test-secret-key")
	t.Setenv(notification.EnvNotificationSMSTencentSDKAppID, "1400000000")
	t.Setenv(notification.EnvNotificationSMSSignName, "Platform")

	sender, err := notificationSMSSenderFromConfig(config.Config{NotificationSMSProvider: notification.SMSProviderTencent})
	if err != nil {
		t.Fatalf("notificationSMSSenderFromConfig() error = %v", err)
	}
	receipt, err := sender.SendSMS(context.Background(), notification.SMSMessage{
		Recipient:  "+8613800000000",
		TemplateID: "login-template",
		Purpose:    "login",
	})
	if err != nil {
		t.Fatalf("SendSMS() error = %v", err)
	}
	if receipt.Status != notification.SMSDeliveryDryRunAccepted {
		t.Fatalf("receipt = %+v, want dry-run when live send is disabled", receipt)
	}
}

func TestNotificationSMSSenderFromConfigRejectsConflictingDryRunAndLiveSend(t *testing.T) {
	t.Setenv(notification.EnvNotificationSMSDryRun, "true")
	t.Setenv(notification.EnvNotificationSMSLiveSendEnabled, "true")

	_, err := notificationSMSSenderFromConfig(config.Config{NotificationSMSProvider: notification.SMSProviderTencent})
	if err == nil || !strings.Contains(err.Error(), "cannot both be true") {
		t.Fatalf("notificationSMSSenderFromConfig() error = %v, want conflict rejection", err)
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
