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
	sensitiveReveal, err := bootstrap.SensitiveRevealRuntimeFromConfig(cfg, ordered, phoneVerification)
	if err != nil {
		log.Fatalf("build sensitive reveal runtime: %v", err)
	}
	rateLimitRuntime, err := bootstrap.RateLimitRuntimeFromConfig(cfg)
	if err != nil {
		log.Fatalf("build rate limit runtime: %v", err)
	}
	var adminStepUpPhoneResolver httpapi.AdminStepUpPhoneResolver
	if cfg.AdminStepUpPhoneSourceConfigured() {
		adminStepUpPhoneResolver, err = httpapi.NewResourceAdminStepUpPhoneResolver(resources, phoneVerification.Protector, httpapi.AdminStepUpPhoneSource{
			Resource: cfg.AdminStepUpPhoneResource, ActorField: cfg.AdminStepUpPhoneActorField,
			PhoneField: cfg.AdminStepUpPhoneField, VerifiedAtField: cfg.AdminStepUpPhoneVerifiedAtField,
			VerifiedPhoneDigestField: cfg.AdminStepUpPhoneVerifiedDigestField,
		}, time.Now)
		if err != nil {
			log.Fatalf("build admin step-up phone resolver: %v", err)
		}
	}
	server := httpapi.New(httpapi.ServerOptions{
		Capabilities:             ordered,
		Resources:                resources,
		Sessions:                 sessions,
		Cache:                    cacheStore,
		InvalidationBus:          invalidationBus,
		CacheTTL:                 cfg.CacheDefaultTTL,
		FileStorage:              fileStorage,
		UploadPolicy:             uploadPolicy,
		AdminRoutes:              apps.DefaultAdminRoutes(resources),
		AppRoutes:                apps.DefaultAppRoutes(resources),
		AdminIdentityResolver:    adminIdentityResolver,
		AdminIdentityBindings:    adminIdentityBindings,
		AppIdentityResolver:      appIdentityResolver,
		PhoneProtector:           phoneVerification.Protector,
		PhoneVerificationSender:  phoneVerification.Sender,
		AdminStepUpPhoneResolver: adminStepUpPhoneResolver,
		SensitiveReveal:          sensitiveReveal,
		DebugCodeEnabled:         phoneVerification.DebugCodeEnabled,
		JWTSecret:                cfg.JWTSecret,
		OpenAPIDocument:          openAPIDocument,
		DisableDemoAuthProvider:  cfg.DisableDemoAuthProvider,
		Security:                 securityOptionsFromConfig(cfg),
		RateLimiter:              rateLimitRuntime.Limiter,
		RateLimitKeyBuilder:      rateLimitRuntime.KeyBuilder,
	})
	if err := server.Run(cfg.HTTPAddr); err != nil {
		log.Fatalf("run platform api: %v", err)
	}
}

func validateCredentialBoundary(manifests []capability.Manifest) error {
	for _, manifest := range manifests {
		for _, provider := range manifest.AuthProviders {
			kind := strings.ToLower(strings.TrimSpace(provider.Kind))
			if kind == "password" {
				return fmt.Errorf("local password provider requires a separately approved Argon2id capability")
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
