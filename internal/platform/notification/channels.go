package notification

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

const (
	ChannelInApp          = "in_app"
	ChannelEmail          = "email"
	ChannelWeChatOfficial = "wechat_official"
	ChannelWeChatMiniapp  = "wechat_miniapp"

	EmailProviderSMTP          = "smtp"
	WeChatProviderOfficial     = "wechat-official"
	WeChatProviderMiniapp      = "wechat-miniapp"
	InAppProviderLocal         = "local"
	GenericDeliveryDryRun      = "dry-run"
	GenericDeliveryDryRunState = "dry-run-accepted"
)

type Message struct {
	TenantCode     string
	Channel        string
	Recipient      string
	TemplateID     string
	TemplateParams map[string]string
	Title          string
	Body           string
	Purpose        string
	TraceID        string
}

type DeliveryReceipt struct {
	Channel        string `json:"channel"`
	Provider       string `json:"provider"`
	MessageID      string `json:"messageId"`
	Status         string `json:"status"`
	RedactedTarget string `json:"redactedTarget"`
}

type MessageSender interface {
	SendMessage(context.Context, Message) (DeliveryReceipt, error)
	Channel() string
	Kind() string
}

type DryRunMessageSender struct {
	mu       sync.Mutex
	counter  int
	channel  string
	provider string
}

func CanonicalChannel(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func CanonicalProvider(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func IsSupportedChannel(channel string) bool {
	switch CanonicalChannel(channel) {
	case ChannelInApp, ChannelSMS, ChannelEmail, ChannelWeChatOfficial, ChannelWeChatMiniapp:
		return true
	default:
		return false
	}
}

func DefaultProviderForChannel(channel string) string {
	switch CanonicalChannel(channel) {
	case ChannelInApp:
		return InAppProviderLocal
	case ChannelSMS:
		return SMSProviderMockLocal
	case ChannelEmail:
		return EmailProviderSMTP
	case ChannelWeChatOfficial:
		return WeChatProviderOfficial
	case ChannelWeChatMiniapp:
		return WeChatProviderMiniapp
	default:
		return ""
	}
}

func IsSupportedProviderForChannel(channel string, provider string) bool {
	channel = CanonicalChannel(channel)
	provider = CanonicalProvider(provider)
	switch channel {
	case ChannelInApp:
		return provider == "" || provider == InAppProviderLocal
	case ChannelSMS:
		return IsSupportedSMSProvider(provider)
	case ChannelEmail:
		return provider == EmailProviderSMTP
	case ChannelWeChatOfficial:
		return provider == WeChatProviderOfficial
	case ChannelWeChatMiniapp:
		return provider == WeChatProviderMiniapp
	default:
		return false
	}
}

func NewDryRunMessageSender(channel string, provider string) (*DryRunMessageSender, error) {
	channel = CanonicalChannel(channel)
	if !IsSupportedChannel(channel) {
		return nil, fmt.Errorf("notification channel %q is unsupported", channel)
	}
	provider = CanonicalProvider(provider)
	if provider == "" {
		provider = DefaultProviderForChannel(channel)
	}
	if !IsSupportedProviderForChannel(channel, provider) {
		return nil, fmt.Errorf("notification provider %q is unsupported for channel %q", provider, channel)
	}
	return &DryRunMessageSender{channel: channel, provider: provider}, nil
}

func (s *DryRunMessageSender) Channel() string {
	if s == nil {
		return ""
	}
	return s.channel
}

func (s *DryRunMessageSender) Kind() string {
	if s == nil {
		return ""
	}
	return s.provider
}

func (s *DryRunMessageSender) SendMessage(_ context.Context, message Message) (DeliveryReceipt, error) {
	if err := validateMessage(message); err != nil {
		return DeliveryReceipt{}, err
	}
	if s == nil {
		return DeliveryReceipt{}, errors.New("notification sender is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counter++
	return DeliveryReceipt{
		Channel:        s.channel,
		Provider:       s.provider,
		MessageID:      fmt.Sprintf("%s-%s-%06d", s.channel, GenericDeliveryDryRun, s.counter),
		Status:         GenericDeliveryDryRunState,
		RedactedTarget: RedactMessageTarget(s.channel, message.Recipient),
	}, nil
}

func DeliveryReceiptFromSMS(receipt SMSDeliveryReceipt) DeliveryReceipt {
	return DeliveryReceipt{
		Channel:        ChannelSMS,
		Provider:       receipt.Provider,
		MessageID:      receipt.MessageID,
		Status:         receipt.Status,
		RedactedTarget: receipt.RedactedTarget,
	}
}

func SMSMessageFromMessage(message Message) SMSMessage {
	return SMSMessage{
		TenantCode:     message.TenantCode,
		Recipient:      message.Recipient,
		TemplateID:     message.TemplateID,
		TemplateParams: cloneWorkerStringMap(message.TemplateParams),
		Purpose:        message.Purpose,
		TraceID:        message.TraceID,
	}
}

func RedactMessageTarget(channel string, value string) string {
	switch CanonicalChannel(channel) {
	case ChannelSMS:
		return RedactSMSTarget(value)
	case ChannelEmail:
		return redactEmailTarget(value)
	default:
		return redactOpaqueTarget(value)
	}
}

func validateMessage(message Message) error {
	channel := CanonicalChannel(message.Channel)
	if !IsSupportedChannel(channel) {
		return fmt.Errorf("notification channel %q is unsupported", channel)
	}
	if strings.TrimSpace(message.Recipient) == "" {
		return fmt.Errorf("notification recipient is required")
	}
	if strings.TrimSpace(message.TemplateID) == "" {
		return fmt.Errorf("notification template id is required")
	}
	return nil
}

func senderKey(channel string, provider string) string {
	return CanonicalChannel(channel) + ":" + CanonicalProvider(provider)
}

func redactEmailTarget(value string) string {
	trimmed := strings.TrimSpace(value)
	at := strings.LastIndex(trimmed, "@")
	if at <= 0 || at == len(trimmed)-1 {
		return redactOpaqueTarget(trimmed)
	}
	local := []rune(trimmed[:at])
	if len(local) <= 2 {
		return string(local[:1]) + "***" + trimmed[at:]
	}
	return string(local[:2]) + "***" + trimmed[at:]
}

func redactOpaqueTarget(value string) string {
	trimmed := strings.TrimSpace(value)
	runes := []rune(trimmed)
	if len(runes) == 0 {
		return "****"
	}
	if len(runes) <= 4 {
		return "****"
	}
	return "****" + string(runes[len(runes)-4:])
}
