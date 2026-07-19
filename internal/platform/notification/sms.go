package notification

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

const (
	ChannelSMS = "sms"

	SMSProviderAliyun    = "aliyun"
	SMSProviderTencent   = "tencent"
	SMSProviderMockLocal = "mock-local"

	SMSDeliveryAccepted       = "accepted"
	SMSDeliveryDryRunAccepted = "dry-run-accepted"

	EnvNotificationSMSDryRun          = "PLATFORM_NOTIFICATION_SMS_DRY_RUN"
	EnvNotificationSMSLiveSendEnabled = "PLATFORM_NOTIFICATION_SMS_LIVE_SEND_ENABLED"
	EnvNotificationSMSSignName        = "PLATFORM_NOTIFICATION_SMS_SIGN_NAME"

	EnvNotificationSMSAliyunRegion      = "PLATFORM_NOTIFICATION_SMS_ALIYUN_REGION"
	EnvNotificationSMSAliyunAccessKeyID = "PLATFORM_NOTIFICATION_SMS_ALIYUN_ACCESS_KEY_ID"
	EnvNotificationSMSAliyunSecretKey   = "PLATFORM_NOTIFICATION_SMS_ALIYUN_ACCESS_KEY_SECRET"

	EnvNotificationSMSTencentRegion    = "PLATFORM_NOTIFICATION_SMS_TENCENT_REGION"
	EnvNotificationSMSTencentSecretID  = "PLATFORM_NOTIFICATION_SMS_TENCENT_SECRET_ID"
	EnvNotificationSMSTencentSecretKey = "PLATFORM_NOTIFICATION_SMS_TENCENT_SECRET_KEY"
	EnvNotificationSMSTencentSDKAppID  = "PLATFORM_NOTIFICATION_SMS_TENCENT_SDK_APP_ID"
)

type SMSMessage struct {
	TenantCode     string
	Recipient      string
	TemplateID     string
	TemplateParams map[string]string
	Purpose        string
	TraceID        string
}

type SMSDeliveryReceipt struct {
	Provider       string
	MessageID      string
	Status         string
	RedactedTarget string
}

type SMSSender interface {
	SendSMS(context.Context, SMSMessage) (SMSDeliveryReceipt, error)
	Kind() string
}

type SMSProviderConfig struct {
	Provider          string
	DryRun            bool
	LiveSendEnabled   bool
	SignName          string
	AliyunRegion      string
	AliyunAccessKeyID string
	AliyunSecretKey   string
	TencentRegion     string
	TencentSecretID   string
	TencentSecretKey  string
	TencentSDKAppID   string
}

type DryRunSMSSender struct {
	mu       sync.Mutex
	counter  int
	provider string
}

func IsSupportedSMSProvider(provider string) bool {
	switch provider {
	case SMSProviderAliyun, SMSProviderTencent, SMSProviderMockLocal:
		return true
	default:
		return false
	}
}

func CanonicalSMSProvider(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func NewVendorSMSSender(config SMSProviderConfig) (SMSSender, error) {
	normalized, err := validateSMSProviderConfig(config)
	if err != nil {
		return nil, err
	}
	if normalized.LiveSendEnabled && !normalized.DryRun {
		return nil, fmt.Errorf("notification SMS provider %q live sending is not implemented", normalized.Provider)
	}
	return NewDryRunSMSSender(normalized)
}

func NewDryRunSMSSender(config SMSProviderConfig) (*DryRunSMSSender, error) {
	normalized, err := validateSMSProviderConfig(config)
	if err != nil {
		return nil, err
	}
	return &DryRunSMSSender{provider: normalized.Provider}, nil
}

func (s *DryRunSMSSender) Kind() string {
	if s == nil {
		return ""
	}
	return s.provider
}

func (*DryRunSMSSender) DryRun() bool {
	return true
}

func (s *DryRunSMSSender) SendSMS(_ context.Context, message SMSMessage) (SMSDeliveryReceipt, error) {
	if err := validateSMSMessage(message); err != nil {
		return SMSDeliveryReceipt{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counter++
	return SMSDeliveryReceipt{
		Provider:       s.provider,
		MessageID:      fmt.Sprintf("%s-dry-run-%06d", s.provider, s.counter),
		Status:         SMSDeliveryDryRunAccepted,
		RedactedTarget: RedactSMSTarget(message.Recipient),
	}, nil
}

type MockLocalSMSSender struct {
	mu      sync.Mutex
	counter int
	sent    []SMSMessage
}

func NewMockLocalSMSSender() *MockLocalSMSSender {
	return &MockLocalSMSSender{}
}

func (*MockLocalSMSSender) Kind() string {
	return SMSProviderMockLocal
}

func (s *MockLocalSMSSender) SendSMS(_ context.Context, message SMSMessage) (SMSDeliveryReceipt, error) {
	if err := validateSMSMessage(message); err != nil {
		return SMSDeliveryReceipt{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counter++
	s.sent = append(s.sent, cloneSMSMessage(message))
	return SMSDeliveryReceipt{
		Provider:       SMSProviderMockLocal,
		MessageID:      fmt.Sprintf("mock-local-%06d", s.counter),
		Status:         SMSDeliveryAccepted,
		RedactedTarget: RedactSMSTarget(message.Recipient),
	}, nil
}

func (s *MockLocalSMSSender) Sent() []SMSMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]SMSMessage, 0, len(s.sent))
	for _, message := range s.sent {
		result = append(result, cloneSMSMessage(message))
	}
	return result
}

func validateSMSProviderConfig(config SMSProviderConfig) (SMSProviderConfig, error) {
	normalized := config
	normalized.Provider = CanonicalSMSProvider(config.Provider)
	var errs []error
	if config.Provider != normalized.Provider {
		errs = append(errs, errors.New("notification SMS provider must be canonical trimmed lowercase"))
	}
	switch normalized.Provider {
	case SMSProviderAliyun:
		errs = append(errs, validateSMSConfigField(EnvNotificationSMSAliyunRegion, config.AliyunRegion, &normalized.AliyunRegion)...)
		errs = append(errs, validateSMSConfigField(EnvNotificationSMSAliyunAccessKeyID, config.AliyunAccessKeyID, &normalized.AliyunAccessKeyID)...)
		errs = append(errs, validateSMSConfigField(EnvNotificationSMSAliyunSecretKey, config.AliyunSecretKey, &normalized.AliyunSecretKey)...)
		errs = append(errs, validateSMSConfigField(EnvNotificationSMSSignName, config.SignName, &normalized.SignName)...)
	case SMSProviderTencent:
		errs = append(errs, validateSMSConfigField(EnvNotificationSMSTencentRegion, config.TencentRegion, &normalized.TencentRegion)...)
		errs = append(errs, validateSMSConfigField(EnvNotificationSMSTencentSecretID, config.TencentSecretID, &normalized.TencentSecretID)...)
		errs = append(errs, validateSMSConfigField(EnvNotificationSMSTencentSecretKey, config.TencentSecretKey, &normalized.TencentSecretKey)...)
		errs = append(errs, validateSMSConfigField(EnvNotificationSMSTencentSDKAppID, config.TencentSDKAppID, &normalized.TencentSDKAppID)...)
		errs = append(errs, validateSMSConfigField(EnvNotificationSMSSignName, config.SignName, &normalized.SignName)...)
	case "":
		errs = append(errs, errors.New("notification SMS provider is required"))
	case SMSProviderMockLocal:
		errs = append(errs, errors.New("notification SMS vendor sender does not support mock-local"))
	default:
		errs = append(errs, fmt.Errorf("unsupported notification SMS provider %q", normalized.Provider))
	}
	if config.DryRun && config.LiveSendEnabled {
		errs = append(errs, fmt.Errorf("%s and %s cannot both be true", EnvNotificationSMSDryRun, EnvNotificationSMSLiveSendEnabled))
	}
	return normalized, errors.Join(errs...)
}

func validateSMSConfigField(envKey string, value string, normalized *string) []error {
	trimmed := strings.TrimSpace(value)
	*normalized = trimmed
	var errs []error
	if value != trimmed {
		errs = append(errs, fmt.Errorf("%s must be trimmed", envKey))
	}
	if trimmed == "" {
		errs = append(errs, fmt.Errorf("%s is required", envKey))
	}
	return errs
}

func validateSMSMessage(message SMSMessage) error {
	if strings.TrimSpace(message.Recipient) == "" {
		return fmt.Errorf("sms recipient is required")
	}
	if strings.TrimSpace(message.TemplateID) == "" {
		return fmt.Errorf("sms template id is required")
	}
	return nil
}

func cloneSMSMessage(message SMSMessage) SMSMessage {
	clone := message
	if message.TemplateParams != nil {
		clone.TemplateParams = make(map[string]string, len(message.TemplateParams))
		for key, value := range message.TemplateParams {
			clone.TemplateParams[key] = value
		}
	}
	return clone
}

func RedactSMSTarget(value string) string {
	trimmed := strings.TrimSpace(value)
	runes := []rune(trimmed)
	if len(runes) <= 4 {
		return "****"
	}
	return "****" + string(runes[len(runes)-4:])
}
