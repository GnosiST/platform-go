package bootstrap

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GnosiST/platform-go/internal/platform/config"
	"github.com/GnosiST/platform-go/internal/platform/credentialauth"
	"github.com/GnosiST/platform-go/internal/platform/notification"
)

func TestCredentialAuthRuntimeFromConfigDisabledByDefault(t *testing.T) {
	runtime, err := CredentialAuthRuntimeFromConfig(context.Background(), config.Load(), NotificationSMSRuntime{})
	if err != nil {
		t.Fatalf("CredentialAuthRuntimeFromConfig() error = %v", err)
	}
	if runtime != nil {
		t.Fatalf("CredentialAuthRuntimeFromConfig() = %+v, want nil when no provider is enabled", runtime)
	}
}

func TestCredentialAuthRuntimeFromConfigSeedsBootstrapAdmin(t *testing.T) {
	cfg := config.Load()
	cfg.Capabilities = append(cfg.Capabilities, "notification", "credential-auth")
	cfg.CredentialAuthUsernamePassword = true
	cfg.CredentialAuthPhoneSMSOTP = true
	cfg.CredentialAuthIdentifierHMACKey = strings.Repeat("i", 32)
	cfg.CredentialAuthBootstrapAdminUsername = "admin"
	cfg.CredentialAuthBootstrapAdminPassword = "correct-password"
	cfg.CredentialAuthBootstrapAdminPhone = "+8613800138000"
	cfg.NotificationSMSProvider = notification.SMSProviderMockLocal
	cfg.NotificationSMSLoginTemplateID = "login-template"
	sms := NotificationSMSRuntime{Sender: notification.NewMockLocalSMSSender(), LoginTemplateID: "login-template", MockLocalEnabled: true}

	runtime, err := CredentialAuthRuntimeFromConfig(context.Background(), cfg, sms)
	if err != nil {
		t.Fatalf("CredentialAuthRuntimeFromConfig() error = %v", err)
	}
	if runtime == nil || runtime.Service == nil || runtime.SecretTransport == nil || runtime.SMSSender == nil || !runtime.DebugCodeEnabled || !runtime.RequireEncryptedSecrets {
		t.Fatalf("credential auth runtime = %+v, want configured service, encrypted secret transport and debug mock sender", runtime)
	}
	login, err := runtime.Service.VerifyPassword(context.Background(), credentialauth.PasswordLoginInput{
		Identifier: credentialauth.Identifier{Type: credentialauth.IdentifierTypeUsername, Value: "admin"},
		Secret:     "correct-password",
	})
	if err != nil {
		t.Fatalf("VerifyPassword() error = %v", err)
	}
	if login.Principal.Type != credentialauth.PrincipalTypeAdmin || login.Principal.ID != "admin" {
		t.Fatalf("VerifyPassword principal = %+v, want admin bootstrap principal", login.Principal)
	}
}

func TestCredentialAuthRuntimeFromConfigUsesPersistentRepository(t *testing.T) {
	cfg := config.Load()
	cfg.Capabilities = append(cfg.Capabilities, "notification", "credential-auth")
	cfg.CredentialAuthUsernamePassword = true
	cfg.CredentialAuthRepositoryDriver = "sqlite"
	cfg.CredentialAuthRepositoryDSN = filepath.Join(t.TempDir(), "credential-auth.db")
	cfg.CredentialAuthIdentifierHMACKey = strings.Repeat("i", 32)
	cfg.CredentialAuthBootstrapAdminUsername = "admin"
	cfg.CredentialAuthBootstrapAdminPassword = "correct-password"
	sms := NotificationSMSRuntime{}

	runtime, err := CredentialAuthRuntimeFromConfig(context.Background(), cfg, sms)
	if err != nil {
		t.Fatalf("CredentialAuthRuntimeFromConfig() error = %v", err)
	}
	if runtime == nil || runtime.Service == nil || runtime.SecretTransport == nil || !runtime.RequireEncryptedSecrets {
		t.Fatalf("credential auth runtime = %+v, want encrypted persistent runtime", runtime)
	}

	reopened, err := CredentialAuthRuntimeFromConfig(context.Background(), cfg, sms)
	if err != nil {
		t.Fatalf("CredentialAuthRuntimeFromConfig(reopen) error = %v", err)
	}
	if _, err := reopened.Service.VerifyPassword(context.Background(), credentialauth.PasswordLoginInput{
		Identifier: credentialauth.Identifier{Type: credentialauth.IdentifierTypeUsername, Value: "admin"},
		Secret:     "correct-password",
	}); err != nil {
		t.Fatalf("VerifyPassword(reopened) error = %v", err)
	}
}
