package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/GnosiST/platform-go/internal/platform/adminresource"
	"github.com/GnosiST/platform-go/internal/platform/config"
	"github.com/GnosiST/platform-go/internal/platform/notification"
)

// RuntimeSettingsProjection is the non-secret configuration applied from the
// persisted admin-resource desired state at process startup.
type RuntimeSettingsProjection struct {
	Config          config.Config
	AppliedRevision uint64
}

// RuntimeSettingsFromAdminResources overlays enabled credential-auth and SMS
// settings onto base. Provider credentials remain encrypted in admin resources
// and are intentionally not part of this projection.
func RuntimeSettingsFromAdminResources(ctx context.Context, base config.Config, resources *adminresource.Store) (RuntimeSettingsProjection, error) {
	if resources == nil {
		return RuntimeSettingsProjection{}, errors.New("runtime settings require an admin resource store")
	}
	if ctx == nil {
		return RuntimeSettingsProjection{}, errors.New("runtime settings context is required")
	}
	projected := RuntimeSettingsProjection{Config: base}
	if !runtimeSettingsCapabilityEnabled(base, "credential-auth") {
		projected.AppliedRevision = resources.Revision()
		return projected, nil
	}
	credentialSettings, present, err := enabledRuntimeSettingsRecord(ctx, resources, "credential-auth-settings")
	if err != nil {
		return RuntimeSettingsProjection{}, err
	}
	if present {
		if err := applyCredentialAuthSettings(&projected.Config, credentialSettings); err != nil {
			return RuntimeSettingsProjection{}, err
		}
	}
	if !projected.Config.CredentialAuthPhoneSMSOTP {
		projected.AppliedRevision = resources.Revision()
		return projected, nil
	}
	if err := applyNotificationSMSSettings(ctx, &projected.Config, resources); err != nil {
		return RuntimeSettingsProjection{}, err
	}
	projected.AppliedRevision = resources.Revision()
	return projected, nil
}

func runtimeSettingsCapabilityEnabled(cfg config.Config, id string) bool {
	for _, capabilityID := range cfg.Capabilities {
		if strings.TrimSpace(capabilityID) == id {
			return true
		}
	}
	return false
}

func enabledRuntimeSettingsRecord(ctx context.Context, resources *adminresource.Store, resource string) (adminresource.Record, bool, error) {
	records, err := resources.InternalRecordsContext(ctx, resource)
	if errors.Is(err, adminresource.ErrUnknownResource) {
		return adminresource.Record{}, false, nil
	}
	if err != nil {
		return adminresource.Record{}, false, fmt.Errorf("load %s desired state: %w", resource, err)
	}
	enabled := make([]adminresource.Record, 0, 1)
	for _, record := range records {
		if strings.EqualFold(strings.TrimSpace(record.Status), "enabled") {
			enabled = append(enabled, record)
		}
	}
	if len(enabled) == 0 {
		return adminresource.Record{}, false, nil
	}
	if len(enabled) != 1 {
		return adminresource.Record{}, false, fmt.Errorf("runtime settings require exactly one enabled %s record", resource)
	}
	return enabled[0], true, nil
}

func applyCredentialAuthSettings(cfg *config.Config, record adminresource.Record) error {
	if cfg == nil {
		return errors.New("credential-auth runtime settings config is required")
	}
	usernamePassword, err := requiredRuntimeBool(record, "usernamePasswordEnabled")
	if err != nil {
		return err
	}
	phonePassword, err := requiredRuntimeBool(record, "phonePasswordEnabled")
	if err != nil {
		return err
	}
	emailPassword, err := requiredRuntimeBool(record, "emailPasswordEnabled")
	if err != nil {
		return err
	}
	phoneSMSOTP, err := requiredRuntimeBool(record, "phoneSMSOTPEnabled")
	if err != nil {
		return err
	}
	challengeMode := strings.TrimSpace(record.Values["challengeMode"])
	if challengeMode != "always" {
		return errors.New("credential-auth runtime settings require challengeMode always")
	}
	challengeKind := strings.TrimSpace(record.Values["challengeKind"])
	if challengeKind != "captcha" && challengeKind != "slider" {
		return errors.New("credential-auth runtime settings require a supported challengeKind")
	}
	if strings.TrimSpace(record.Values["secretTransport"]) != "ecdh-a256gcm-v1" {
		return errors.New("credential-auth runtime settings require ecdh-a256gcm-v1 secret transport")
	}
	if strings.TrimSpace(record.Values["passwordAlgorithm"]) != "argon2id" {
		return errors.New("credential-auth runtime settings require argon2id password algorithm")
	}
	maxPasswordAttempts, err := requiredRuntimePositiveInt(record, "passwordMaxAttempts")
	if err != nil {
		return err
	}
	lockSeconds, err := requiredRuntimePositiveInt(record, "lockSeconds")
	if err != nil {
		return err
	}
	smsOTPTTLSeconds, err := requiredRuntimePositiveInt(record, "smsOTPTTLSeconds")
	if err != nil {
		return err
	}
	smsOTPMaxAttempts, err := requiredRuntimePositiveInt(record, "smsOTPMaxAttempts")
	if err != nil {
		return err
	}
	cfg.CredentialAuthUsernamePassword = usernamePassword
	cfg.CredentialAuthPhonePassword = phonePassword
	cfg.CredentialAuthEmailPassword = emailPassword
	cfg.CredentialAuthPhoneSMSOTP = phoneSMSOTP
	cfg.CredentialAuthChallengeMode = challengeMode
	cfg.CredentialAuthChallengeKind = challengeKind
	cfg.CredentialAuthPasswordMaxAttempts = maxPasswordAttempts
	cfg.CredentialAuthPasswordLock = time.Duration(lockSeconds) * time.Second
	cfg.CredentialAuthSMSOTPTTL = time.Duration(smsOTPTTLSeconds) * time.Second
	cfg.CredentialAuthSMSOTPMaxAttempts = smsOTPMaxAttempts
	return nil
}

func applyNotificationSMSSettings(ctx context.Context, cfg *config.Config, resources *adminresource.Store) error {
	channels, err := resources.InternalRecordsContext(ctx, "notification-channels")
	if errors.Is(err, adminresource.ErrUnknownResource) {
		return errors.New("credential-auth SMS OTP requires notification SMS channel desired state")
	}
	if err != nil {
		return fmt.Errorf("load notification SMS channels: %w", err)
	}
	channel, err := enabledSMSChannel(channels)
	if err != nil {
		return err
	}
	providerCode := strings.TrimSpace(channel.Values["defaultProviderCode"])
	providers, err := resources.InternalRecordsContext(ctx, "notification-providers")
	if errors.Is(err, adminresource.ErrUnknownResource) {
		return errors.New("credential-auth SMS OTP requires notification SMS provider desired state")
	}
	if err != nil {
		return fmt.Errorf("load notification SMS providers: %w", err)
	}
	provider, err := enabledSMSProvider(providers, providerCode)
	if err != nil {
		return err
	}
	providerName := notification.CanonicalSMSProvider(provider.Values["provider"])
	if !notification.IsSupportedSMSProvider(providerName) {
		return errors.New("notification SMS desired state has an unsupported provider")
	}
	templates, err := resources.InternalRecordsContext(ctx, "notification-templates")
	if errors.Is(err, adminresource.ErrUnknownResource) {
		return errors.New("credential-auth SMS OTP requires notification SMS template desired state")
	}
	if err != nil {
		return fmt.Errorf("load notification SMS templates: %w", err)
	}
	template, err := enabledSMSLoginTemplate(templates)
	if err != nil {
		return err
	}
	cfg.NotificationSMSProvider = providerName
	cfg.NotificationSMSLoginTemplateID = template.Code
	return nil
}

// NotificationSMSSenderFromAdminResources constructs the selected SMS sender
// from protected provider credentials. Callers retain deployment-controlled
// dry-run/live-send flags in safety; provider identity and credentials come
// only from the persisted desired state.
func NotificationSMSSenderFromAdminResources(ctx context.Context, cfg config.Config, resources *adminresource.Store, safety notification.SMSProviderConfig) (notification.SMSSender, error) {
	providerName := notification.CanonicalSMSProvider(cfg.NotificationSMSProvider)
	if providerName == "" {
		return nil, nil
	}
	if resources == nil {
		return nil, errors.New("notification SMS runtime requires an admin resource store")
	}
	if providerName == notification.SMSProviderMockLocal {
		return notification.NewMockLocalSMSSender(), nil
	}
	providers, err := resources.InternalRecordsContext(ctx, "notification-providers")
	if err != nil {
		return nil, fmt.Errorf("load notification SMS providers: %w", err)
	}
	var selected *adminresource.Record
	for index := range providers {
		record := &providers[index]
		if strings.EqualFold(strings.TrimSpace(record.Status), "enabled") && strings.TrimSpace(record.Values["channel"]) == notification.ChannelSMS && notification.CanonicalSMSProvider(record.Values["provider"]) == providerName {
			if selected != nil {
				return nil, errors.New("notification runtime settings require exactly one enabled SMS provider")
			}
			selected = record
		}
	}
	if selected == nil {
		return nil, errors.New("notification runtime settings require an enabled SMS provider")
	}
	accessKey, err := runtimeProviderSecret(ctx, resources, selected.ID, "accessKey")
	if err != nil {
		return nil, err
	}
	accessSecret, err := runtimeProviderSecret(ctx, resources, selected.ID, "accessSecret")
	if err != nil {
		return nil, err
	}
	safety.Provider = providerName
	safety.SignName = strings.TrimSpace(selected.Values["senderId"])
	switch providerName {
	case notification.SMSProviderAliyun:
		safety.AliyunRegion = strings.TrimSpace(selected.Values["region"])
		safety.AliyunAccessKeyID = accessKey
		safety.AliyunSecretKey = accessSecret
	case notification.SMSProviderTencent:
		sdkAppID, secretErr := runtimeProviderSecret(ctx, resources, selected.ID, "appSecret")
		if secretErr != nil {
			return nil, secretErr
		}
		safety.TencentRegion = strings.TrimSpace(selected.Values["region"])
		safety.TencentSecretID = accessKey
		safety.TencentSecretKey = accessSecret
		safety.TencentSDKAppID = sdkAppID
	default:
		return nil, errors.New("notification runtime settings has an unsupported SMS provider")
	}
	sender, err := notification.NewVendorSMSSender(safety)
	if err != nil {
		return nil, fmt.Errorf("build notification SMS sender from desired state: %w", err)
	}
	return sender, nil
}

func runtimeProviderSecret(ctx context.Context, resources *adminresource.Store, recordID string, field string) (string, error) {
	value, err := resources.RevealProtectedField(ctx, adminresource.ProtectedFieldRevealRequest{
		Resource: "notification-providers", RecordID: recordID, Field: field, Purpose: adminresource.ProtectedFieldPurposeRuntimeBootstrap,
	})
	if err != nil || strings.TrimSpace(value) == "" {
		return "", errors.New("notification runtime settings require protected SMS provider credentials")
	}
	return value, nil
}

func enabledSMSChannel(records []adminresource.Record) (adminresource.Record, error) {
	matches := make([]adminresource.Record, 0, 1)
	for _, record := range records {
		if !strings.EqualFold(strings.TrimSpace(record.Status), "enabled") || !runtimeBool(record.Values["enabled"]) || strings.TrimSpace(record.Values["channel"]) != notification.ChannelSMS {
			continue
		}
		matches = append(matches, record)
	}
	if len(matches) != 1 {
		return adminresource.Record{}, errors.New("notification runtime settings require exactly one enabled SMS channel")
	}
	return matches[0], nil
}

func enabledSMSProvider(records []adminresource.Record, code string) (adminresource.Record, error) {
	code = strings.TrimSpace(code)
	for _, record := range records {
		if record.Code == code && strings.EqualFold(strings.TrimSpace(record.Status), "enabled") && strings.TrimSpace(record.Values["channel"]) == notification.ChannelSMS {
			return record, nil
		}
	}
	return adminresource.Record{}, errors.New("notification runtime settings require an enabled SMS default provider")
}

func enabledSMSLoginTemplate(records []adminresource.Record) (adminresource.Record, error) {
	matches := make([]adminresource.Record, 0, 1)
	for _, record := range records {
		if strings.EqualFold(strings.TrimSpace(record.Status), "enabled") && strings.TrimSpace(record.Values["channel"]) == notification.ChannelSMS && record.Code == "sms-login-code" {
			matches = append(matches, record)
		}
	}
	if len(matches) != 1 {
		return adminresource.Record{}, errors.New("notification runtime settings require exactly one enabled sms-login-code template")
	}
	return matches[0], nil
}

func requiredRuntimeBool(record adminresource.Record, key string) (bool, error) {
	value := strings.TrimSpace(record.Values[key])
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("credential-auth runtime settings %s must be a boolean", key)
	}
	return parsed, nil
}

func requiredRuntimePositiveInt(record adminresource.Record, key string) (int, error) {
	value := strings.TrimSpace(record.Values[key])
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("credential-auth runtime settings %s must be a positive integer", key)
	}
	return parsed, nil
}

func runtimeBool(value string) bool {
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	return err == nil && parsed
}
