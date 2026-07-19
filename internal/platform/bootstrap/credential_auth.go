package bootstrap

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/GnosiST/platform-go/internal/platform/config"
	"github.com/GnosiST/platform-go/internal/platform/credentialauth"
	"github.com/GnosiST/platform-go/internal/platform/httpapi"
)

func CredentialAuthRuntimeFromConfig(ctx context.Context, cfg config.Config, sms NotificationSMSRuntime) (*httpapi.CredentialAuthRuntime, error) {
	if !cfg.CredentialAuthConfigured() {
		return nil, nil
	}
	environment := strings.ToLower(strings.TrimSpace(cfg.RuntimeEnvironment))
	if environment == "" {
		environment = config.RuntimeEnvironmentDevelopment
	}
	if environment == config.RuntimeEnvironmentProduction {
		return nil, fmt.Errorf("production credential-auth requires a persistent credential repository")
	}
	hasher, err := credentialauth.NewHMACIdentifierHasher([]byte(cfg.CredentialAuthIdentifierHMACKey))
	if err != nil {
		return nil, fmt.Errorf("build credential identifier hasher: %w", err)
	}
	repository := credentialauth.NewMemoryRepository()
	service, err := credentialauth.NewService(credentialauth.Options{
		Repository:       repository,
		IdentifierHasher: hasher,
		PasswordVerifier: credentialauth.NewArgon2idVerifier(credentialauth.DefaultArgon2idParams()),
		Now:              time.Now,
	})
	if err != nil {
		return nil, fmt.Errorf("build credential auth service: %w", err)
	}
	if err := seedCredentialAuthBootstrapAdmin(ctx, cfg, service); err != nil {
		return nil, err
	}
	if cfg.CredentialAuthPhoneSMSOTP && sms.Sender == nil {
		return nil, fmt.Errorf("credential-auth phone SMS OTP requires notification SMS sender")
	}
	return &httpapi.CredentialAuthRuntime{
		Service:           service,
		IdentifierHasher:  hasher,
		SMSOTPHasher:      hasher,
		SMSSender:         sms.Sender,
		LoginTemplateID:   sms.LoginTemplateID,
		DebugCodeEnabled:  sms.MockLocalEnabled && (environment == config.RuntimeEnvironmentDevelopment || environment == config.RuntimeEnvironmentTest),
		Now:               time.Now,
		SMSOTPTTL:         5 * time.Minute,
		MaxSMSOTPAttempts: credentialauth.DefaultMaxSMSOTPAttempts,
	}, nil
}

func seedCredentialAuthBootstrapAdmin(ctx context.Context, cfg config.Config, service *credentialauth.Service) error {
	username := strings.TrimSpace(cfg.CredentialAuthBootstrapAdminUsername)
	if username == "" {
		return nil
	}
	principal := credentialauth.PrincipalRef{Type: credentialauth.PrincipalTypeAdmin, ID: username}
	if cfg.CredentialAuthUsernamePassword {
		if _, err := service.RegisterIdentifier(ctx, credentialauth.RegisterIdentifierInput{
			Principal:  principal,
			Identifier: credentialauth.Identifier{Type: credentialauth.IdentifierTypeUsername, Value: username},
			Status:     credentialauth.StatusEnabled,
		}); err != nil {
			return fmt.Errorf("seed credential username identifier: %w", err)
		}
	}
	phone := strings.TrimSpace(cfg.CredentialAuthBootstrapAdminPhone)
	if cfg.CredentialAuthPhonePassword || cfg.CredentialAuthPhoneSMSOTP {
		if _, err := service.RegisterIdentifier(ctx, credentialauth.RegisterIdentifierInput{
			Principal:  principal,
			Identifier: credentialauth.Identifier{Type: credentialauth.IdentifierTypePhone, Value: phone},
			Status:     credentialauth.StatusEnabled,
		}); err != nil {
			return fmt.Errorf("seed credential phone identifier: %w", err)
		}
	}
	if cfg.CredentialAuthEmailPassword {
		if _, err := service.RegisterIdentifier(ctx, credentialauth.RegisterIdentifierInput{
			Principal:  principal,
			Identifier: credentialauth.Identifier{Type: credentialauth.IdentifierTypeEmail, Value: cfg.CredentialAuthBootstrapAdminEmail},
			Status:     credentialauth.StatusEnabled,
		}); err != nil {
			return fmt.Errorf("seed credential email identifier: %w", err)
		}
	}
	if cfg.CredentialAuthUsernamePassword || cfg.CredentialAuthPhonePassword || cfg.CredentialAuthEmailPassword {
		hash, err := credentialauth.HashPasswordArgon2id(cfg.CredentialAuthBootstrapAdminPassword, credentialauth.DefaultArgon2idParams())
		if err != nil {
			return fmt.Errorf("seed credential password hash: %w", err)
		}
		if err := service.PutPasswordCredential(ctx, credentialauth.PasswordCredential{
			Principal:     principal,
			PasswordHash:  hash,
			Algorithm:     credentialauth.PasswordAlgorithmArgon2id,
			ParamsVersion: "argon2id-default",
			Status:        credentialauth.StatusEnabled,
		}); err != nil {
			return fmt.Errorf("seed credential password: %w", err)
		}
	}
	return nil
}
