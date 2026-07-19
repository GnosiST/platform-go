package notification

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

const (
	ChannelSMS = "sms"

	SMSProviderAliyun    = "aliyun"
	SMSProviderTencent   = "tencent"
	SMSProviderMockLocal = "mock-local"

	SMSDeliveryAccepted = "accepted"
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
	if strings.TrimSpace(message.Recipient) == "" {
		return SMSDeliveryReceipt{}, fmt.Errorf("sms recipient is required")
	}
	if strings.TrimSpace(message.TemplateID) == "" {
		return SMSDeliveryReceipt{}, fmt.Errorf("sms template id is required")
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
