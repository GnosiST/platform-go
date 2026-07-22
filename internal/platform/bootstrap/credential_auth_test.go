package bootstrap

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/GnosiST/platform-go/internal/platform/adminresource"
	"github.com/GnosiST/platform-go/internal/platform/config"
	"github.com/GnosiST/platform-go/internal/platform/core"
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
	resources := adminresource.NewStoreFromCapabilities(core.DefaultManifests())

	runtime, err := CredentialAuthRuntimeFromConfig(context.Background(), cfg, sms, resources)
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
	if login.Principal.Type != credentialauth.PrincipalTypeAdmin || login.Principal.ID != "user-admin" {
		t.Fatalf("VerifyPassword principal = %+v, want admin user principal", login.Principal)
	}
}

func TestCredentialAuthRuntimeFromConfigKeepsUsernamePrincipalWithoutResources(t *testing.T) {
	cfg := config.Load()
	cfg.Capabilities = append(cfg.Capabilities, "notification", "credential-auth")
	cfg.CredentialAuthUsernamePassword = true
	cfg.CredentialAuthIdentifierHMACKey = strings.Repeat("i", 32)
	cfg.CredentialAuthBootstrapAdminUsername = "admin"
	cfg.CredentialAuthBootstrapAdminPassword = "correct-password"
	sms := NotificationSMSRuntime{}

	runtime, err := CredentialAuthRuntimeFromConfig(context.Background(), cfg, sms)
	if err != nil {
		t.Fatalf("CredentialAuthRuntimeFromConfig() error = %v", err)
	}
	login, err := runtime.Service.VerifyPassword(context.Background(), credentialauth.PasswordLoginInput{
		Identifier: credentialauth.Identifier{Type: credentialauth.IdentifierTypeUsername, Value: "admin"},
		Secret:     "correct-password",
	})
	if err != nil {
		t.Fatalf("VerifyPassword() error = %v", err)
	}
	if login.Principal.Type != credentialauth.PrincipalTypeAdmin || login.Principal.ID != "admin" {
		t.Fatalf("VerifyPassword principal = %+v, want username fallback principal", login.Principal)
	}
}

func TestCredentialAuthRuntimeFromConfigAppliesDesiredStateLimits(t *testing.T) {
	cfg := config.Load()
	cfg.Capabilities = append(cfg.Capabilities, "notification", "credential-auth")
	cfg.CredentialAuthUsernamePassword = true
	cfg.CredentialAuthIdentifierHMACKey = strings.Repeat("i", 32)
	cfg.CredentialAuthBootstrapAdminUsername = "admin"
	cfg.CredentialAuthBootstrapAdminPassword = "correct-password"
	cfg.CredentialAuthPasswordMaxAttempts = 2
	cfg.CredentialAuthPasswordLock = 90 * time.Second
	cfg.CredentialAuthSMSOTPTTL = 2 * time.Minute
	cfg.CredentialAuthSMSOTPMaxAttempts = 3
	cfg.CredentialAuthChallengeKind = "slider"

	runtime, err := CredentialAuthRuntimeFromConfig(context.Background(), cfg, NotificationSMSRuntime{})
	if err != nil {
		t.Fatalf("CredentialAuthRuntimeFromConfig() error = %v", err)
	}
	if runtime.SMSOTPTTL != 2*time.Minute || runtime.MaxSMSOTPAttempts != 3 || runtime.LoginChallengeKind != credentialauth.ChallengeKindSlider {
		t.Fatalf("credential auth runtime policy = %+v, want desired SMS and challenge settings", runtime)
	}
	identifier := credentialauth.Identifier{Type: credentialauth.IdentifierTypeUsername, Value: "admin"}
	for attempt := 0; attempt < 2; attempt++ {
		if _, err := runtime.Service.VerifyPassword(context.Background(), credentialauth.PasswordLoginInput{Identifier: identifier, Secret: "wrong"}); !errors.Is(err, credentialauth.ErrCredentialRejected) {
			t.Fatalf("failed attempt %d error = %v, want credential rejection", attempt+1, err)
		}
	}
	if _, err := runtime.Service.VerifyPassword(context.Background(), credentialauth.PasswordLoginInput{Identifier: identifier, Secret: "correct-password"}); !errors.Is(err, credentialauth.ErrCredentialLocked) {
		t.Fatalf("locked credential error = %v, want configured lock", err)
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
