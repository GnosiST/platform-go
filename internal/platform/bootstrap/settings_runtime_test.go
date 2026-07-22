package bootstrap

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/GnosiST/platform-go/internal/platform/adminresource"
	"github.com/GnosiST/platform-go/internal/platform/config"
	"github.com/GnosiST/platform-go/internal/platform/core"
	"github.com/GnosiST/platform-go/internal/platform/notification"
)

func TestRuntimeSettingsFromAdminResourcesAppliesCredentialAndSMSDesiredState(t *testing.T) {
	resources := runtimeSettingsStore(t)
	updateRuntimeSettingsRecord(t, resources, "credential-auth-settings", "credential-auth-setting-default", adminresource.WriteInput{
		Code: "default", Name: "Default Credential Auth Settings", Status: "enabled", Description: "Runtime settings.",
		Values: map[string]string{
			"usernamePasswordEnabled": "false", "phonePasswordEnabled": "false", "emailPasswordEnabled": "false", "phoneSMSOTPEnabled": "true",
			"challengeMode": "always", "challengeKind": "slider", "passwordMaxAttempts": "3", "lockSeconds": "120",
			"smsOTPTTLSeconds": "180", "smsOTPMaxAttempts": "4", "secretTransport": "ecdh-a256gcm-v1",
			"passwordAlgorithm": "argon2id", "argon2ParamsVersion": "v1",
		},
	})
	updateRuntimeSettingsRecord(t, resources, "notification-channels", "notification-channel-sms", adminresource.WriteInput{
		Code: "sms", Name: "SMS", Status: "enabled", Description: "SMS channel.",
		Values: map[string]string{"tenantCode": "platform", "channel": "sms", "defaultProviderCode": "sms-aliyun", "enabled": "true", "rateLimitPerMinute": "60", "dailyQuota": "2000"},
	})
	updateRuntimeSettingsRecord(t, resources, "notification-providers", "notification-provider-sms-aliyun", adminresource.WriteInput{
		Code: "sms-aliyun", Name: "Aliyun SMS", Status: "enabled", Description: "Aliyun SMS.",
		Values: map[string]string{"tenantCode": "platform", "channel": "sms", "provider": "aliyun", "accountName": "Aliyun", "endpoint": "dysmsapi.aliyuncs.com", "region": "cn-hangzhou", "senderId": "Platform", "templateNamespace": "aliyun", "credentialStatus": "configured", "accessKey": "access-key", "accessSecret": "provider-secret"},
	})

	base := config.Config{RuntimeEnvironment: config.RuntimeEnvironmentDevelopment, Capabilities: []string{"credential-auth", "notification"}}
	projection, err := RuntimeSettingsFromAdminResources(context.Background(), base, resources)
	if err != nil {
		t.Fatalf("RuntimeSettingsFromAdminResources() error = %v", err)
	}
	got := projection.Config
	if got.CredentialAuthUsernamePassword || got.CredentialAuthPhonePassword || got.CredentialAuthEmailPassword || !got.CredentialAuthPhoneSMSOTP {
		t.Fatalf("credential auth providers = username:%t phone:%t email:%t sms:%t, want SMS only", got.CredentialAuthUsernamePassword, got.CredentialAuthPhonePassword, got.CredentialAuthEmailPassword, got.CredentialAuthPhoneSMSOTP)
	}
	if got.CredentialAuthChallengeMode != "always" || got.CredentialAuthChallengeKind != "slider" || got.CredentialAuthPasswordMaxAttempts != 3 || got.CredentialAuthPasswordLock != 2*time.Minute || got.CredentialAuthSMSOTPTTL != 3*time.Minute || got.CredentialAuthSMSOTPMaxAttempts != 4 {
		t.Fatalf("credential auth desired state = %+v, want projected policy", got)
	}
	if got.NotificationSMSProvider != "aliyun" || got.NotificationSMSLoginTemplateID != "sms-login-code" {
		t.Fatalf("notification SMS desired state = provider %q template %q, want aliyun/sms-login-code", got.NotificationSMSProvider, got.NotificationSMSLoginTemplateID)
	}
	if projection.AppliedRevision == 0 || projection.AppliedRevision != resources.Revision() {
		t.Fatalf("applied revision = %d store revision = %d, want current persisted revision", projection.AppliedRevision, resources.Revision())
	}
	if strings.Contains(fmt.Sprintf("%+v", projection), "provider-secret") || strings.Contains(fmt.Sprintf("%+v", projection), "access-key") {
		t.Fatalf("runtime settings projection leaked provider secret: %+v", projection)
	}
}

func TestNotificationSMSSenderFromAdminResourcesUsesProtectedProviderCredentials(t *testing.T) {
	resources := runtimeSettingsStore(t)
	updateRuntimeSettingsRecord(t, resources, "notification-providers", "notification-provider-sms-aliyun", adminresource.WriteInput{
		Code: "sms-aliyun", Name: "Aliyun SMS", Status: "enabled", Description: "Aliyun SMS.",
		Values: map[string]string{"tenantCode": "platform", "channel": "sms", "provider": "aliyun", "accountName": "Aliyun", "endpoint": "dysmsapi.aliyuncs.com", "region": "cn-hangzhou", "senderId": "Platform", "templateNamespace": "aliyun", "credentialStatus": "configured", "accessKey": "access-key", "accessSecret": "provider-secret"},
	})
	sender, err := NotificationSMSSenderFromAdminResources(context.Background(), config.Config{NotificationSMSProvider: "aliyun"}, resources, notification.SMSProviderConfig{DryRun: true})
	if err != nil {
		t.Fatalf("NotificationSMSSenderFromAdminResources() error = %v", err)
	}
	if sender == nil || sender.Kind() != notification.SMSProviderAliyun {
		t.Fatalf("notification sender = %+v, want aliyun dry-run sender", sender)
	}
}

func TestRuntimeSettingsFromAdminResourcesRejectsAmbiguousOrUnsafeCredentialSettings(t *testing.T) {
	resources := runtimeSettingsStore(t)
	_, err := resources.CreateInternal("credential-auth-settings", adminresource.WriteInput{
		Code: "second", Name: "Second", Status: "enabled", Description: "Second settings.",
		Values: map[string]string{
			"usernamePasswordEnabled": "true", "phonePasswordEnabled": "false", "emailPasswordEnabled": "false", "phoneSMSOTPEnabled": "false",
			"challengeMode": "always", "challengeKind": "captcha", "passwordMaxAttempts": "5", "lockSeconds": "900",
			"smsOTPTTLSeconds": "300", "smsOTPMaxAttempts": "5", "secretTransport": "ecdh-a256gcm-v1", "passwordAlgorithm": "argon2id", "argon2ParamsVersion": "v1",
		},
	})
	if err != nil {
		t.Fatalf("create second credential settings: %v", err)
	}
	_, err = RuntimeSettingsFromAdminResources(context.Background(), config.Config{Capabilities: []string{"credential-auth", "notification"}}, resources)
	if err == nil || !strings.Contains(err.Error(), "exactly one enabled credential-auth-settings") {
		t.Fatalf("RuntimeSettingsFromAdminResources(ambiguous) error = %v, want deterministic rejection", err)
	}
}

func TestRuntimeSettingsFromAdminResourcesIgnoresDisabledCredentialCapability(t *testing.T) {
	resources := runtimeSettingsStore(t)
	projection, err := RuntimeSettingsFromAdminResources(context.Background(), config.Config{Capabilities: []string{"notification"}}, resources)
	if err != nil {
		t.Fatalf("RuntimeSettingsFromAdminResources() error = %v", err)
	}
	if projection.Config.CredentialAuthConfigured() {
		t.Fatalf("disabled credential capability projected auth providers: %+v", projection.Config)
	}
}

func runtimeSettingsStore(t *testing.T) *adminresource.Store {
	t.Helper()
	resources, err := adminresource.NewStoreFromCapabilitiesWithProtection(core.DefaultManifests(), testDataProtectionRuntime(t))
	if err != nil {
		t.Fatalf("build runtime settings store: %v", err)
	}
	return resources
}

func updateRuntimeSettingsRecord(t *testing.T, resources *adminresource.Store, resource string, id string, input adminresource.WriteInput) {
	t.Helper()
	if _, err := resources.UpdateInternal(resource, id, input); err != nil {
		t.Fatalf("update %s/%s: %v", resource, id, err)
	}
}
