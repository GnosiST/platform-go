package bootstrap

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/GnosiST/platform-go/internal/platform/config"
	"github.com/GnosiST/platform-go/internal/platform/credentialauth"
	"github.com/GnosiST/platform-go/internal/platform/httpapi"
	"github.com/GnosiST/platform-go/internal/platform/storage"
)

func CredentialAuthRuntimeFromConfig(ctx context.Context, cfg config.Config, sms NotificationSMSRuntime) (*httpapi.CredentialAuthRuntime, error) {
	if !cfg.CredentialAuthConfigured() {
		return nil, nil
	}
	environment := strings.ToLower(strings.TrimSpace(cfg.RuntimeEnvironment))
	if environment == "" {
		environment = config.RuntimeEnvironmentDevelopment
	}
	hasher, err := credentialauth.NewHMACIdentifierHasher([]byte(cfg.CredentialAuthIdentifierHMACKey))
	if err != nil {
		return nil, fmt.Errorf("build credential identifier hasher: %w", err)
	}
	repository, err := credentialAuthRepositoryFromConfig(ctx, cfg, environment)
	if err != nil {
		return nil, err
	}
	secretTransport, err := credentialAuthSecretTransportFromConfig(cfg, environment)
	if err != nil {
		return nil, err
	}
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
		Service:                 service,
		IdentifierHasher:        hasher,
		SecretTransport:         secretTransport,
		SMSOTPHasher:            hasher,
		SMSSender:               sms.Sender,
		LoginTemplateID:         sms.LoginTemplateID,
		DebugCodeEnabled:        sms.MockLocalEnabled && (environment == config.RuntimeEnvironmentDevelopment || environment == config.RuntimeEnvironmentTest),
		Now:                     time.Now,
		SMSOTPTTL:               5 * time.Minute,
		MaxSMSOTPAttempts:       credentialauth.DefaultMaxSMSOTPAttempts,
		RequireEncryptedSecrets: true,
	}, nil
}

func credentialAuthRepositoryFromConfig(ctx context.Context, cfg config.Config, environment string) (credentialauth.Repository, error) {
	driver := strings.TrimSpace(cfg.CredentialAuthRepositoryDriver)
	dsn := strings.TrimSpace(cfg.CredentialAuthRepositoryDSN)
	if driver == "" && dsn == "" {
		if environment == config.RuntimeEnvironmentProduction {
			return nil, fmt.Errorf("production credential-auth requires a persistent credential repository")
		}
		return credentialauth.NewMemoryRepository(), nil
	}
	if driver == "" || dsn == "" {
		return nil, fmt.Errorf("credential-auth repository driver and dsn are required together")
	}
	db, err := storage.OpenGORM(storage.Config{Driver: driver, DSN: dsn})
	if err != nil {
		return nil, fmt.Errorf("open credential-auth repository database: %w", err)
	}
	repository, err := credentialauth.NewGORMRepository(ctx, db)
	if err != nil {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
		return nil, fmt.Errorf("open credential-auth repository: %w", err)
	}
	return repository, nil
}

func credentialAuthSecretTransportFromConfig(cfg config.Config, environment string) (*credentialauth.SecretTransport, error) {
	keyID := strings.TrimSpace(cfg.CredentialAuthSecretTransportKeyID)
	privateKeyValue := strings.TrimSpace(cfg.CredentialAuthSecretTransportKey)
	if keyID == "" {
		if environment == config.RuntimeEnvironmentProduction {
			return nil, fmt.Errorf("production credential-auth requires application-layer secret transport key id")
		}
		keyID = "development-ephemeral"
	}
	var privateKey []byte
	if privateKeyValue != "" {
		decoded, err := credentialauth.DecodeSecretTransportPrivateKey(privateKeyValue)
		if err != nil {
			return nil, fmt.Errorf("decode credential-auth secret transport private key: %w", err)
		}
		privateKey = decoded
	} else if environment == config.RuntimeEnvironmentProduction {
		return nil, fmt.Errorf("production credential-auth requires application-layer secret transport private key")
	}
	transport, err := credentialauth.NewSecretTransport(credentialauth.SecretTransportOptions{
		KeyID:      keyID,
		PrivateKey: privateKey,
		Now:        time.Now,
	})
	if err != nil {
		return nil, fmt.Errorf("build credential-auth secret transport: %w", err)
	}
	return transport, nil
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
