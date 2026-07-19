package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/GnosiST/platform-go/internal/apps"
	"github.com/GnosiST/platform-go/internal/platform/authprovider"
	"github.com/GnosiST/platform-go/internal/platform/bootstrap"
	"github.com/GnosiST/platform-go/internal/platform/capability"
	"github.com/GnosiST/platform-go/internal/platform/config"
	"github.com/GnosiST/platform-go/internal/platform/httpapi"
	"github.com/GnosiST/platform-go/internal/platform/notification"
	"github.com/GnosiST/platform-go/internal/platform/rbac"
	"github.com/GnosiST/platform-go/internal/platform/serviceobject"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
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
	integrationRuntime, err := bootstrap.IntegrationsFromConfig(cfg, bootstrap.IntegrationAdapters{})
	if err != nil {
		log.Fatalf("build optional integrations: %v", err)
	}
	for _, status := range integrationRuntime.Status(ctx) {
		log.Printf("optional integration capability=%s enabled=%t state=%s adapter=%s", status.Capability, status.Enabled, status.State, status.Adapter)
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
	var organizationRBAC *bootstrap.OrganizationRBAC
	if cfg.OrganizationRBACMode == config.OrganizationRBACModeTarget {
		organizationRBAC, err = bootstrap.OpenOrganizationRBAC(ctx, cfg)
		if err != nil {
			log.Fatalf("open organization rbac runtime: %v", err)
		}
		defer func() { _ = organizationRBAC.Close() }()
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
	notificationSMSSender, err := notificationSMSSenderFromConfig(cfg)
	if err != nil {
		log.Fatalf("build notification SMS sender: %v", err)
	}
	notificationSMS, err := bootstrap.NotificationSMSRuntimeFromConfig(cfg, notificationSMSSender)
	if err != nil {
		log.Fatalf("build notification SMS runtime: %v", err)
	}
	if notificationSMS.Sender != nil {
		log.Printf("notification sms provider=%s mockLocal=%t loginTemplateConfigured=%t", notificationSMS.Sender.Kind(), notificationSMS.MockLocalEnabled, notificationSMS.LoginTemplateID != "")
	}
	credentialAuth, err := bootstrap.CredentialAuthRuntimeFromConfig(ctx, cfg, notificationSMS)
	if err != nil {
		log.Fatalf("build credential auth runtime: %v", err)
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
	var serviceObjects *serviceobject.Runtime
	var adminMenuResolver httpapi.AdminMenuResolver
	var adminMenuComparisonSink any = adminMenuComparisonLogSink{}
	if organizationRBAC != nil {
		serviceObjects = organizationRBAC.ServiceObjects
		adminMenuResolver = organizationRBAC.AdminMenus
		adminMenuComparisonSink = organizationRBAC.MenuComparisonSink
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
		CredentialAuth:           credentialAuth,
		AppIdentityResolver:      appIdentityResolver,
		PhoneProtector:           phoneVerification.Protector,
		PhoneVerificationSender:  phoneVerification.Sender,
		NotificationSMSSender:    notificationSMS.Sender,
		AdminStepUpPhoneResolver: adminStepUpPhoneResolver,
		SensitiveReveal:          sensitiveReveal,
		ServiceObjects:           serviceObjects,
		DebugCodeEnabled:         phoneVerification.DebugCodeEnabled,
		JWTSecret:                cfg.JWTSecret,
		OpenAPIDocument:          openAPIDocument,
		CapabilityLockFile:       cfg.CapabilityLockFile,
		CapabilityConfigSource:   cfg.CapabilityConfigSource(),
		DisableDemoAuthProvider:  cfg.DisableDemoAuthProvider,
		Security:                 securityOptionsFromConfig(cfg),
		RateLimiter:              rateLimitRuntime.Limiter,
		RateLimitKeyBuilder:      rateLimitRuntime.KeyBuilder,
		AdminMenuServingMode:     httpapi.AdminMenuServingMode(cfg.AdminMenuServingMode),
		AdminMenuResolver:        adminMenuResolver,
		AdminMenuComparisonSink:  adminMenuComparisonSink,
	})
	dataLifecycle, scheduler, err := startRetentionRuntime(ctx, cfg, apps.DefaultManifests(), bootstrap.OpenDataLifecycle)
	if err != nil {
		log.Fatalf("start retention runtime: %v", err)
	}
	if dataLifecycle != nil {
		defer func() { _ = dataLifecycle.Close() }()
	}
	if scheduler != nil {
		defer func() { _ = scheduler.Close() }()
	}
	if err := runHTTPServer(ctx, cfg.HTTPAddr, server.Router()); err != nil {
		if scheduler != nil {
			_ = scheduler.Close()
		}
		if dataLifecycle != nil {
			_ = dataLifecycle.Close()
		}
		log.Fatalf("run platform api: %v", err)
	}
}

type adminMenuComparisonLogSink struct{}

func (adminMenuComparisonLogSink) Record(_ context.Context, _ rbac.Principal, comparison httpapi.AdminMenuComparison) {
	log.Printf(
		"admin menu dual-read equal=%t addedCount=%d removedCount=%d globalRevision=%d",
		comparison.Equal, comparison.AddedCount, comparison.RemovedCount, comparison.GlobalRevision,
	)
}

type openDataLifecycleFunc func(config.Config, ...capability.Manifest) (*bootstrap.DataLifecycle, error)

func startRetentionRuntime(ctx context.Context, cfg config.Config, manifests []capability.Manifest, open openDataLifecycleFunc) (*bootstrap.DataLifecycle, *bootstrap.DataLifecycleScheduler, error) {
	if !cfg.RetentionRunnerEnabled {
		return nil, nil, nil
	}
	lifecycle, err := open(cfg, manifests...)
	if err != nil {
		return nil, nil, err
	}
	scheduler, err := bootstrap.StartDataLifecycleScheduler(ctx, lifecycle, bootstrap.DataLifecycleScheduleOptions{
		Interval: cfg.RetentionRunnerInterval, BatchSize: cfg.RetentionRunnerBatchSize, MaxRetries: cfg.RetentionRunnerMaxRetries,
	})
	if err != nil {
		_ = lifecycle.Close()
		return nil, nil, err
	}
	return lifecycle, scheduler, nil
}

func runHTTPServer(ctx context.Context, addr string, handler http.Handler) error {
	server := &http.Server{Addr: addr, Handler: handler}
	errCh := make(chan error, 1)
	go func() { errCh <- server.ListenAndServe() }()
	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		err := <-errCh
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
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

func notificationSMSSenderFromConfig(cfg config.Config) (notification.SMSSender, error) {
	rawProvider := cfg.NotificationSMSProvider
	provider := notification.CanonicalSMSProvider(rawProvider)
	if rawProvider != provider {
		return nil, nil
	}
	switch provider {
	case "":
		return nil, nil
	case notification.SMSProviderMockLocal:
		return notification.NewMockLocalSMSSender(), nil
	case notification.SMSProviderAliyun, notification.SMSProviderTencent:
		config, err := notificationSMSProviderConfigFromEnv(provider)
		if err != nil {
			return nil, err
		}
		return notification.NewVendorSMSSender(config)
	default:
		return nil, nil
	}
}

func notificationSMSProviderConfigFromEnv(provider string) (notification.SMSProviderConfig, error) {
	dryRun, dryRunConfigured, dryRunErr := notificationSMSBoolEnv(notification.EnvNotificationSMSDryRun)
	liveSendEnabled, _, liveSendErr := notificationSMSBoolEnv(notification.EnvNotificationSMSLiveSendEnabled)
	if dryRunErr != nil {
		return notification.SMSProviderConfig{}, dryRunErr
	}
	if liveSendErr != nil {
		return notification.SMSProviderConfig{}, liveSendErr
	}
	config := notification.SMSProviderConfig{
		Provider:          provider,
		DryRun:            dryRunConfigured && dryRun,
		LiveSendEnabled:   liveSendEnabled,
		SignName:          os.Getenv(notification.EnvNotificationSMSSignName),
		AliyunRegion:      os.Getenv(notification.EnvNotificationSMSAliyunRegion),
		AliyunAccessKeyID: os.Getenv(notification.EnvNotificationSMSAliyunAccessKeyID),
		AliyunSecretKey:   os.Getenv(notification.EnvNotificationSMSAliyunSecretKey),
		TencentRegion:     os.Getenv(notification.EnvNotificationSMSTencentRegion),
		TencentSecretID:   os.Getenv(notification.EnvNotificationSMSTencentSecretID),
		TencentSecretKey:  os.Getenv(notification.EnvNotificationSMSTencentSecretKey),
		TencentSDKAppID:   os.Getenv(notification.EnvNotificationSMSTencentSDKAppID),
	}
	if dryRunConfigured && dryRun && liveSendEnabled {
		return notification.SMSProviderConfig{}, fmt.Errorf("%s and %s cannot both be true", notification.EnvNotificationSMSDryRun, notification.EnvNotificationSMSLiveSendEnabled)
	}
	if !liveSendEnabled {
		config.DryRun = true
	}
	return config, nil
}

func notificationSMSBoolEnv(key string) (bool, bool, error) {
	raw, ok := os.LookupEnv(key)
	if !ok {
		return false, false, nil
	}
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "true", "1", "yes", "y":
		return true, true, nil
	case "false", "0", "no", "n":
		return false, true, nil
	default:
		return false, true, fmt.Errorf("%s must be a boolean", key)
	}
}
