package main

import (
	"context"
	"log"
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
	runtime, err := bootstrap.RuntimeFromConfig(cfg)
	if err != nil {
		log.Fatalf("build platform runtime: %v", err)
	}
	if err := capability.RunLifecycle(context.Background(), ordered, runtime); err != nil {
		log.Fatalf("run capability lifecycle: %v", err)
	}
	resources, err := bootstrap.AdminResourcesFromConfig(cfg, ordered)
	if err != nil {
		log.Fatalf("build admin resources: %v", err)
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
	})
	if err := server.Run(cfg.HTTPAddr); err != nil {
		log.Fatalf("run platform api: %v", err)
	}
}

func securityOptionsFromConfig(cfg config.Config) httpapi.SecurityOptions {
	return httpapi.SecurityOptions{
		RequireHTTPS:     cfg.RuntimeEnvironment == config.RuntimeEnvironmentProduction,
		PublicBaseURL:    cfg.PublicBaseURL,
		TrustedProxies:   append([]string(nil), cfg.TrustedProxies...),
		MaxJSONBodyBytes: cfg.HTTPMaxBodyBytes,
	}
}

func phoneVerificationSenderFromConfig(cfg config.Config) httpapi.PhoneVerificationSender {
	if cfg.PhoneVerificationProvider == httpapi.PhoneVerificationProviderDebug {
		return httpapi.NewDebugPhoneVerificationSender()
	}
	return nil
}
