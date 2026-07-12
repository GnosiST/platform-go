package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"platform-go/internal/apps"
	"platform-go/internal/platform/authprovider"
	"platform-go/internal/platform/bootstrap"
	"platform-go/internal/platform/capability"
	"platform-go/internal/platform/config"
	"platform-go/internal/platform/httpapi"
)

func main() {
	cfg := config.Load()
	if err := cfg.ValidateRuntime(); err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}
	ordered, err := bootstrap.CapabilitiesFromConfig(cfg, apps.DefaultManifests()...)
	if err != nil {
		log.Fatalf("resolve capabilities: %v", err)
	}
	if err := validateCredentialBoundary(ordered); err != nil {
		log.Fatalf("validate credential boundary: %v", err)
	}
	runtime, err := bootstrap.RuntimeFromConfig(cfg)
	if err != nil {
		log.Fatalf("build platform runtime: %v", err)
	}
	if err := capability.RunLifecycle(context.Background(), ordered, runtime); err != nil {
		log.Fatalf("run capability lifecycle: %v", err)
	}
	dataProtection, err := bootstrap.DataProtectionRuntimeFromConfig(cfg)
	if err != nil {
		log.Fatalf("build data protection runtime: %v", err)
	}
	resources, err := bootstrap.AdminResourcesFromConfig(cfg, ordered, dataProtection)
	if err != nil {
		log.Fatalf("build admin resources: %v", err)
	}
	if err := resources.ValidateProtectedData(context.Background()); err != nil {
		log.Fatalf("validate protected admin resources: %v", err)
	}
	adminIdentityResolver, err := authprovider.AdminIdentityResolverFromConfig(cfg)
	if err != nil {
		log.Fatalf("build admin identity resolver: %v", err)
	}
	adminIdentityBindings := httpapi.NewResourceAdminIdentityBindingStore(resources, time.Now)
	if err := httpapi.ValidateAdminAuthReadiness(context.Background(), ordered, adminIdentityBindings, cfg.DisableDemoAuthProvider); err != nil {
		log.Fatalf("validate admin auth readiness: %v", err)
	}
	sessions, err := bootstrap.SessionsFromConfig(cfg)
	if err != nil {
		log.Fatalf("build sessions: %v", err)
	}
	cacheStore, err := bootstrap.CacheFromConfig(cfg)
	if err != nil {
		log.Fatalf("build cache: %v", err)
	}
	invalidationBus := bootstrap.CacheInvalidationBusFromConfig(cfg)
	fileStorage, err := bootstrap.FileStorageFromConfig(cfg)
	if err != nil {
		log.Fatalf("build file storage: %v", err)
	}
	uploadPolicy, err := httpapi.NewUploadPolicy(cfg.FileMaxUploadBytes, cfg.FileAllowedMIMETypes)
	if err != nil {
		log.Fatalf("build upload policy: %v", err)
	}
	appIdentityResolver, err := authprovider.AppIdentityResolverFromConfig(cfg)
	if err != nil {
		log.Fatalf("build app identity resolver: %v", err)
	}
	openAPIDocument, err := bootstrap.OpenAPIDocumentFromConfig(cfg)
	if err != nil {
		log.Fatalf("load openapi document: %v", err)
	}
	phoneVerificationSender := phoneVerificationSenderFromConfig(cfg)
	phoneVerification, err := bootstrap.PhoneVerificationRuntimeFromConfig(cfg, phoneVerificationSender)
	if err != nil {
		log.Fatalf("build phone verification runtime: %v", err)
	}
	if err := httpapi.ValidatePhoneProtectionHistory(resources, phoneVerification.Protector); err != nil {
		log.Fatalf("validate phone protection history: %v", err)
	}
	rateLimitRuntime, err := bootstrap.RateLimitRuntimeFromConfig(cfg)
	if err != nil {
		log.Fatalf("build rate limit runtime: %v", err)
	}
	server := httpapi.New(httpapi.ServerOptions{
		Capabilities:            ordered,
		Resources:               resources,
		Sessions:                sessions,
		Cache:                   cacheStore,
		InvalidationBus:         invalidationBus,
		CacheTTL:                cfg.CacheDefaultTTL,
		FileStorage:             fileStorage,
		UploadPolicy:            uploadPolicy,
		AdminRoutes:             apps.DefaultAdminRoutes(resources),
		AppRoutes:               apps.DefaultAppRoutes(resources),
		AdminIdentityResolver:   adminIdentityResolver,
		AdminIdentityBindings:   adminIdentityBindings,
		AppIdentityResolver:     appIdentityResolver,
		PhoneProtector:          phoneVerification.Protector,
		PhoneVerificationSender: phoneVerification.Sender,
		DebugCodeEnabled:        phoneVerification.DebugCodeEnabled,
		JWTSecret:               cfg.JWTSecret,
		OpenAPIDocument:         openAPIDocument,
		DisableDemoAuthProvider: cfg.DisableDemoAuthProvider,
		Security:                securityOptionsFromConfig(cfg),
		RateLimiter:             rateLimitRuntime.Limiter,
		RateLimitKeyBuilder:     rateLimitRuntime.KeyBuilder,
	})
	if err := server.Run(cfg.HTTPAddr); err != nil {
		log.Fatalf("run platform api: %v", err)
	}
}

func validateCredentialBoundary(manifests []capability.Manifest) error {
	for _, manifest := range manifests {
		for _, provider := range manifest.AuthProviders {
			kind := strings.ToLower(strings.TrimSpace(provider.Kind))
			id := strings.ToLower(strings.TrimSpace(provider.ID))
			if strings.Contains(kind, "password") || strings.Contains(id, "password") {
				return fmt.Errorf("local password provider requires a separately approved Argon2id capability")
			}
		}
		for _, resource := range manifest.Admin.Resources {
			for _, field := range resource.Fields {
				key := strings.ToLower(strings.TrimSpace(field.Key))
				if strings.Contains(key, "password") || strings.Contains(key, "passwd") {
					return fmt.Errorf("password fields cannot use generic admin resource persistence")
				}
			}
		}
	}
	return nil
}

func securityOptionsFromConfig(cfg config.Config) httpapi.SecurityOptions {
	return httpapi.SecurityOptions{
		RequireHTTPS:   cfg.RuntimeEnvironment == config.RuntimeEnvironmentProduction,
		PublicBaseURL:  cfg.PublicBaseURL,
		TrustedProxies: append([]string(nil), cfg.TrustedProxies...),
		MaxBodyBytes:   cfg.HTTPMaxBodyBytes,
	}
}

func phoneVerificationSenderFromConfig(cfg config.Config) httpapi.PhoneVerificationSender {
	if cfg.PhoneVerificationProvider == httpapi.PhoneVerificationProviderDebug {
		return httpapi.NewDebugPhoneVerificationSender()
	}
	return nil
}
