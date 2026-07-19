package notification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	aliyunsms "github.com/alibabacloud-go/dysmsapi-20170525/v5/client"
	"github.com/alibabacloud-go/tea/tea"
	tencentcommon "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	tencentprofile "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	tencentsms "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/sms/v20210111"
)

const (
	ChannelSMS = "sms"

	SMSProviderAliyun    = "aliyun"
	SMSProviderTencent   = "tencent"
	SMSProviderMockLocal = "mock-local"

	SMSDeliveryAccepted       = "accepted"
	SMSDeliveryDryRunAccepted = "dry-run-accepted"

	SMSProviderTemplateParamOrderKey = "_order"

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

type aliyunSMSClient interface {
	SendSms(*aliyunsms.SendSmsRequest) (*aliyunsms.SendSmsResponse, error)
}

type tencentSMSClient interface {
	SendSms(*tencentsms.SendSmsRequest) (*tencentsms.SendSmsResponse, error)
}

type AliyunSMSSender struct {
	client aliyunSMSClient
	config SMSProviderConfig
}

type TencentSMSSender struct {
	client tencentSMSClient
	config SMSProviderConfig
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
	if normalized.DryRun || !normalized.LiveSendEnabled {
		return NewDryRunSMSSender(normalized)
	}
	switch normalized.Provider {
	case SMSProviderAliyun:
		return newAliyunSMSSender(normalized)
	case SMSProviderTencent:
		return newTencentSMSSender(normalized)
	default:
		return nil, fmt.Errorf("unsupported notification SMS provider %q", normalized.Provider)
	}
}

func NewDryRunSMSSender(config SMSProviderConfig) (*DryRunSMSSender, error) {
	normalized, err := validateSMSProviderConfig(config)
	if err != nil {
		return nil, err
	}
	return &DryRunSMSSender{provider: normalized.Provider}, nil
}

func NewDryRunSMSSenderForProvider(provider string) (SMSSender, error) {
	normalized := CanonicalSMSProvider(provider)
	switch normalized {
	case SMSProviderMockLocal:
		return NewMockLocalSMSSender(), nil
	case SMSProviderAliyun, SMSProviderTencent:
		return &DryRunSMSSender{provider: normalized}, nil
	case "":
		return nil, errors.New("notification SMS provider is required")
	default:
		return nil, fmt.Errorf("unsupported notification SMS provider %q", normalized)
	}
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

func newAliyunSMSSender(config SMSProviderConfig) (*AliyunSMSSender, error) {
	client, err := newAliyunSMSClient(config)
	if err != nil {
		return nil, err
	}
	return newAliyunSMSSenderWithClient(config, client)
}

func newAliyunSMSClient(config SMSProviderConfig) (aliyunSMSClient, error) {
	client, err := aliyunsms.NewClient(&openapi.Config{
		AccessKeyId:     tea.String(config.AliyunAccessKeyID),
		AccessKeySecret: tea.String(config.AliyunSecretKey),
		RegionId:        tea.String(config.AliyunRegion),
		Endpoint:        tea.String("dysmsapi.aliyuncs.com"),
	})
	if err != nil {
		return nil, fmt.Errorf("notification SMS provider %q client initialization failed", SMSProviderAliyun)
	}
	return client, nil
}

func newAliyunSMSSenderWithClient(config SMSProviderConfig, client aliyunSMSClient) (*AliyunSMSSender, error) {
	normalized, err := validateSMSProviderConfig(config)
	if err != nil {
		return nil, err
	}
	if normalized.Provider != SMSProviderAliyun {
		return nil, fmt.Errorf("notification SMS provider %q is not aliyun", normalized.Provider)
	}
	if client == nil {
		return nil, fmt.Errorf("notification SMS provider %q client is required", SMSProviderAliyun)
	}
	return &AliyunSMSSender{client: client, config: normalized}, nil
}

func (*AliyunSMSSender) Kind() string {
	return SMSProviderAliyun
}

func (s *AliyunSMSSender) SendSMS(ctx context.Context, message SMSMessage) (SMSDeliveryReceipt, error) {
	if err := validateSMSMessage(message); err != nil {
		return SMSDeliveryReceipt{}, err
	}
	if s == nil || s.client == nil {
		return SMSDeliveryReceipt{}, fmt.Errorf("notification SMS provider %q client is required", SMSProviderAliyun)
	}
	if err := ctx.Err(); err != nil {
		return SMSDeliveryReceipt{}, err
	}
	request, err := s.aliyunSendSMSRequest(message)
	if err != nil {
		return SMSDeliveryReceipt{}, err
	}
	response, err := s.client.SendSms(request)
	if err != nil {
		return SMSDeliveryReceipt{
			Provider:       SMSProviderAliyun,
			Status:         "failed",
			RedactedTarget: RedactSMSTarget(message.Recipient),
		}, normalizeSMSProviderError(SMSProviderAliyun, "")
	}
	if err := ctx.Err(); err != nil {
		return SMSDeliveryReceipt{}, err
	}
	var body *aliyunsms.SendSmsResponseBody
	if response != nil {
		body = response.GetBody()
	}
	code := ""
	if body != nil {
		code = stringPtrValue(body.Code)
	}
	receipt := SMSDeliveryReceipt{
		Provider:       SMSProviderAliyun,
		MessageID:      aliyunMessageID(body),
		Status:         SMSDeliveryAccepted,
		RedactedTarget: RedactSMSTarget(message.Recipient),
	}
	if !strings.EqualFold(code, "OK") {
		receipt.Status = "provider-rejected"
		return receipt, normalizeSMSProviderError(SMSProviderAliyun, code)
	}
	return receipt, nil
}

func (s *AliyunSMSSender) aliyunSendSMSRequest(message SMSMessage) (*aliyunsms.SendSmsRequest, error) {
	templateParam, err := aliyunTemplateParamJSON(message.TemplateParams)
	if err != nil {
		return nil, err
	}
	request := &aliyunsms.SendSmsRequest{
		PhoneNumbers: tea.String(strings.TrimSpace(message.Recipient)),
		SignName:     tea.String(s.config.SignName),
		TemplateCode: tea.String(strings.TrimSpace(message.TemplateID)),
	}
	if templateParam != "" {
		request.TemplateParam = tea.String(templateParam)
	}
	if outID := boundedSMSContext(message.TraceID, 64); outID != "" {
		request.OutId = tea.String(outID)
	}
	return request, nil
}

func newTencentSMSSender(config SMSProviderConfig) (*TencentSMSSender, error) {
	client, err := newTencentSMSClient(config)
	if err != nil {
		return nil, err
	}
	return newTencentSMSSenderWithClient(config, client)
}

func newTencentSMSClient(config SMSProviderConfig) (tencentSMSClient, error) {
	clientProfile := tencentprofile.NewClientProfile()
	clientProfile.HttpProfile.Endpoint = "sms.tencentcloudapi.com"
	client, err := tencentsms.NewClient(
		tencentcommon.NewCredential(config.TencentSecretID, config.TencentSecretKey),
		config.TencentRegion,
		clientProfile,
	)
	if err != nil {
		return nil, fmt.Errorf("notification SMS provider %q client initialization failed", SMSProviderTencent)
	}
	return client, nil
}

func newTencentSMSSenderWithClient(config SMSProviderConfig, client tencentSMSClient) (*TencentSMSSender, error) {
	normalized, err := validateSMSProviderConfig(config)
	if err != nil {
		return nil, err
	}
	if normalized.Provider != SMSProviderTencent {
		return nil, fmt.Errorf("notification SMS provider %q is not tencent", normalized.Provider)
	}
	if client == nil {
		return nil, fmt.Errorf("notification SMS provider %q client is required", SMSProviderTencent)
	}
	return &TencentSMSSender{client: client, config: normalized}, nil
}

func (*TencentSMSSender) Kind() string {
	return SMSProviderTencent
}

func (s *TencentSMSSender) SendSMS(ctx context.Context, message SMSMessage) (SMSDeliveryReceipt, error) {
	if err := validateSMSMessage(message); err != nil {
		return SMSDeliveryReceipt{}, err
	}
	if s == nil || s.client == nil {
		return SMSDeliveryReceipt{}, fmt.Errorf("notification SMS provider %q client is required", SMSProviderTencent)
	}
	if err := ctx.Err(); err != nil {
		return SMSDeliveryReceipt{}, err
	}
	response, err := s.client.SendSms(s.tencentSendSMSRequest(message))
	if err != nil {
		return SMSDeliveryReceipt{
			Provider:       SMSProviderTencent,
			Status:         "failed",
			RedactedTarget: RedactSMSTarget(message.Recipient),
		}, normalizeSMSProviderError(SMSProviderTencent, "")
	}
	if err := ctx.Err(); err != nil {
		return SMSDeliveryReceipt{}, err
	}
	status := firstTencentSendStatus(response)
	code := ""
	if status != nil {
		code = stringPtrValue(status.Code)
	}
	receipt := SMSDeliveryReceipt{
		Provider:       SMSProviderTencent,
		MessageID:      tencentMessageID(response, status),
		Status:         SMSDeliveryAccepted,
		RedactedTarget: RedactSMSTarget(message.Recipient),
	}
	if !strings.EqualFold(code, "Ok") {
		receipt.Status = "provider-rejected"
		return receipt, normalizeSMSProviderError(SMSProviderTencent, code)
	}
	return receipt, nil
}

func (s *TencentSMSSender) tencentSendSMSRequest(message SMSMessage) *tencentsms.SendSmsRequest {
	request := tencentsms.NewSendSmsRequest()
	request.PhoneNumberSet = []*string{tencentcommon.StringPtr(strings.TrimSpace(message.Recipient))}
	request.SmsSdkAppId = tencentcommon.StringPtr(s.config.TencentSDKAppID)
	request.TemplateId = tencentcommon.StringPtr(strings.TrimSpace(message.TemplateID))
	request.SignName = tencentcommon.StringPtr(s.config.SignName)
	request.TemplateParamSet = tencentStringPtrs(orderedSMSTemplateParamValues(message.TemplateParams))
	if contextValue := boundedSMSContext(message.TraceID, 512); contextValue != "" {
		request.SessionContext = tencentcommon.StringPtr(contextValue)
	}
	return request
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

func aliyunTemplateParamJSON(params map[string]string) (string, error) {
	normalized := normalizedSMSTemplateParams(params)
	if len(normalized) == 0 {
		return "", nil
	}
	delete(normalized, SMSProviderTemplateParamOrderKey)
	if len(normalized) == 0 {
		return "", nil
	}
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return "", fmt.Errorf("sms template params are invalid")
	}
	return string(encoded), nil
}

func orderedSMSTemplateParamValues(params map[string]string) []string {
	normalized := normalizedSMSTemplateParams(params)
	if len(normalized) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	var result []string
	for _, key := range strings.Split(normalized[SMSProviderTemplateParamOrderKey], ",") {
		key = strings.TrimSpace(key)
		if key == "" || key == SMSProviderTemplateParamOrderKey {
			continue
		}
		if _, ok := normalized[key]; !ok {
			continue
		}
		result = append(result, normalized[key])
		seen[key] = struct{}{}
	}
	var remaining []string
	for key := range normalized {
		if key == SMSProviderTemplateParamOrderKey {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		remaining = append(remaining, key)
	}
	sortStrings(remaining)
	for _, key := range remaining {
		result = append(result, normalized[key])
	}
	return result
}

func normalizedSMSTemplateParams(params map[string]string) map[string]string {
	if len(params) == 0 {
		return nil
	}
	normalized := make(map[string]string, len(params))
	for key, value := range params {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		normalized[key] = value
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func firstTencentSendStatus(response *tencentsms.SendSmsResponse) *tencentsms.SendStatus {
	if response == nil || response.Response == nil || len(response.Response.SendStatusSet) == 0 {
		return nil
	}
	return response.Response.SendStatusSet[0]
}

func aliyunMessageID(body *aliyunsms.SendSmsResponseBody) string {
	if body == nil {
		return ""
	}
	if bizID := strings.TrimSpace(stringPtrValue(body.BizId)); bizID != "" {
		return bizID
	}
	return strings.TrimSpace(stringPtrValue(body.RequestId))
}

func tencentMessageID(response *tencentsms.SendSmsResponse, status *tencentsms.SendStatus) string {
	if status != nil {
		if serial := strings.TrimSpace(stringPtrValue(status.SerialNo)); serial != "" {
			return serial
		}
	}
	if response == nil || response.Response == nil {
		return ""
	}
	return strings.TrimSpace(stringPtrValue(response.Response.RequestId))
}

func tencentStringPtrs(values []string) []*string {
	if len(values) == 0 {
		return nil
	}
	result := make([]*string, 0, len(values))
	for _, value := range values {
		result = append(result, tencentcommon.StringPtr(value))
	}
	return result
}

func boundedSMSContext(value string, maxBytes int) string {
	value = strings.TrimSpace(value)
	if value == "" || maxBytes <= 0 {
		return ""
	}
	if len([]byte(value)) <= maxBytes {
		return value
	}
	for len([]byte(value)) > maxBytes {
		runes := []rune(value)
		if len(runes) == 0 {
			return ""
		}
		value = string(runes[:len(runes)-1])
	}
	return value
}

func normalizeSMSProviderError(provider string, code string) error {
	provider = CanonicalSMSProvider(provider)
	code = strings.TrimSpace(code)
	if code == "" {
		return fmt.Errorf("notification SMS provider %q request failed", provider)
	}
	return fmt.Errorf("notification SMS provider %q rejected request with code %q", provider, code)
}

func stringPtrValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}

func RedactSMSTarget(value string) string {
	trimmed := strings.TrimSpace(value)
	runes := []rune(trimmed)
	if len(runes) <= 4 {
		return "****"
	}
	return "****" + string(runes[len(runes)-4:])
}
