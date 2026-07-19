package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/GnosiST/platform-go/internal/platform/adminresource"
)

const (
	NotificationResource         = "notifications"
	NotificationDeliveryResource = "notification-deliveries"

	DeliveryStatusPending   = "pending"
	DeliveryStatusDelivered = "delivered"
	DeliveryStatusFailed    = "failed"

	deliveryWorkerDefaultBatch = 20
	deliveryWorkerActor        = "notification-delivery-worker"
	deliveryWorkerAction       = "notification.delivery.attempt"
)

type DeliveryStore interface {
	InternalRecordsContext(context.Context, string) ([]adminresource.Record, error)
	UpdateInternalWithAudit(string, string, adminresource.WriteInput, adminresource.AuditEvent) (adminresource.MutationResult, error)
}

type DeliveryPolicyGate interface {
	AllowDelivery(context.Context, DeliveryPolicyInput) (bool, error)
}

type DeliveryPolicyInput struct {
	TenantCode   string
	Channel      string
	Provider     string
	TemplateCode string
	TemplateID   string
	Purpose      string
}

type DeliveryWorkerOptions struct {
	SMSSenders          map[string]SMSSender
	MessageSenders      map[string]MessageSender
	DefaultSMSProvider  string
	DefaultProviders    map[string]string
	PolicyGate          DeliveryPolicyGate
	MaxBatch            int
	Now                 func() time.Time
	Actor               string
	AllowDryRunFallback bool
}

type DeliveryWorker struct {
	store               DeliveryStore
	smsSenders          map[string]SMSSender
	messageSenders      map[string]MessageSender
	defaultSMSProvider  string
	defaultProviders    map[string]string
	policyGate          DeliveryPolicyGate
	maxBatch            int
	now                 func() time.Time
	actor               string
	allowDryRunFallback bool
}

type DeliveryWorkerResult struct {
	Scanned   int `json:"scanned"`
	Attempted int `json:"attempted"`
	Delivered int `json:"delivered"`
	Failed    int `json:"failed"`
	Skipped   int `json:"skipped"`
}

type notificationPayload struct {
	Channel        string            `json:"channel"`
	TemplateID     string            `json:"templateId"`
	TemplateParams map[string]string `json:"templateParams"`
	Purpose        string            `json:"purpose"`
}

func NewDeliveryWorker(store DeliveryStore, options DeliveryWorkerOptions) *DeliveryWorker {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	maxBatch := options.MaxBatch
	if maxBatch <= 0 {
		maxBatch = deliveryWorkerDefaultBatch
	}
	actor := strings.TrimSpace(options.Actor)
	if actor == "" {
		actor = deliveryWorkerActor
	}
	senders := make(map[string]SMSSender, len(options.SMSSenders))
	for provider, sender := range options.SMSSenders {
		provider = CanonicalSMSProvider(provider)
		if provider == "" || smsSenderNil(sender) {
			continue
		}
		senders[provider] = sender
	}
	defaultProvider := CanonicalSMSProvider(options.DefaultSMSProvider)
	messageSenders := make(map[string]MessageSender, len(options.MessageSenders))
	for key, sender := range options.MessageSenders {
		if messageSenderNil(sender) {
			continue
		}
		channel := CanonicalChannel(sender.Channel())
		provider := CanonicalProvider(sender.Kind())
		if channel == "" || provider == "" {
			parts := strings.SplitN(key, ":", 2)
			if len(parts) == 2 {
				channel = firstNonBlank(channel, CanonicalChannel(parts[0]))
				provider = firstNonBlank(provider, CanonicalProvider(parts[1]))
			}
		}
		if channel == "" || provider == "" {
			continue
		}
		messageSenders[senderKey(channel, provider)] = sender
	}
	defaultProviders := make(map[string]string, len(options.DefaultProviders)+1)
	for channel, provider := range options.DefaultProviders {
		channel = CanonicalChannel(channel)
		provider = CanonicalProvider(provider)
		if channel == "" || provider == "" {
			continue
		}
		defaultProviders[channel] = provider
	}
	if defaultProvider != "" {
		defaultProviders[ChannelSMS] = defaultProvider
	}
	return &DeliveryWorker{
		store:               store,
		smsSenders:          senders,
		messageSenders:      messageSenders,
		defaultSMSProvider:  defaultProvider,
		defaultProviders:    defaultProviders,
		policyGate:          options.PolicyGate,
		maxBatch:            maxBatch,
		now:                 now,
		actor:               actor,
		allowDryRunFallback: options.AllowDryRunFallback,
	}
}

func (w *DeliveryWorker) RunOnce(ctx context.Context) (DeliveryWorkerResult, error) {
	if w == nil || w.store == nil {
		return DeliveryWorkerResult{}, fmt.Errorf("notification delivery worker store is required")
	}
	deliveries, err := w.store.InternalRecordsContext(ctx, NotificationDeliveryResource)
	if err != nil {
		return DeliveryWorkerResult{}, err
	}
	notifications, err := w.store.InternalRecordsContext(ctx, NotificationResource)
	if err != nil {
		return DeliveryWorkerResult{}, err
	}
	noticeByCode := make(map[string]adminresource.Record, len(notifications))
	for _, record := range notifications {
		noticeByCode[record.Code] = record
	}
	sort.SliceStable(deliveries, func(i, j int) bool {
		left := deliveries[i].UpdatedAt + deliveries[i].Code
		right := deliveries[j].UpdatedAt + deliveries[j].Code
		return left < right
	})
	result := DeliveryWorkerResult{Scanned: len(deliveries)}
	for _, delivery := range deliveries {
		if result.Attempted >= w.maxBatch {
			break
		}
		channel, ok := pendingNotificationDelivery(delivery)
		if !ok {
			result.Skipped++
			continue
		}
		notice, ok := noticeByCode[strings.TrimSpace(delivery.Values["notificationCode"])]
		if !ok {
			result.Attempted++
			if err := w.markDeliveryFailed(delivery, channel, nil, "notification record not found"); err != nil {
				return result, err
			}
			result.Failed++
			continue
		}
		if w.policyGate != nil {
			allowed, err := w.policyGate.AllowDelivery(ctx, deliveryPolicyInputFromRecords(channel, delivery, notice))
			if err != nil {
				return result, err
			}
			if !allowed {
				result.Skipped++
				continue
			}
		}
		result.Attempted++
		delivered, err := w.deliver(ctx, channel, delivery, notice)
		if err != nil {
			return result, err
		}
		if !delivered {
			result.Failed++
			continue
		}
		result.Delivered++
	}
	return result, nil
}

func pendingSMSDelivery(record adminresource.Record) bool {
	channel, ok := pendingNotificationDelivery(record)
	return ok && channel == ChannelSMS
}

func pendingNotificationDelivery(record adminresource.Record) (string, bool) {
	if record.Status == "disabled" {
		return "", false
	}
	values := record.Values
	channel := CanonicalChannel(values["channel"])
	return channel, IsSupportedChannel(channel) &&
		strings.EqualFold(strings.TrimSpace(values["deliveryStatus"]), DeliveryStatusPending)
}

func deliveryPolicyInputFromRecords(channel string, delivery adminresource.Record, notice adminresource.Record) DeliveryPolicyInput {
	payload := parseNotificationPayload(notice.Values["payload"])
	return DeliveryPolicyInput{
		TenantCode:   firstNonBlank(delivery.Values["tenantCode"], notice.Values["tenantCode"]),
		Channel:      channel,
		Provider:     delivery.Values["provider"],
		TemplateCode: firstNonBlank(delivery.Values["templateCode"], notice.Values["templateCode"]),
		TemplateID:   firstNonBlank(delivery.Values["templateId"], notice.Values["templateId"], payload.TemplateID),
		Purpose:      firstNonBlank(notice.Values["category"], payload.Purpose, "notification"),
	}
}

func (w *DeliveryWorker) deliver(ctx context.Context, channel string, delivery adminresource.Record, notice adminresource.Record) (bool, error) {
	if channel == ChannelSMS {
		return w.deliverSMS(ctx, delivery, notice)
	}
	return w.deliverMessage(ctx, channel, delivery, notice)
}

func (w *DeliveryWorker) deliverSMS(ctx context.Context, delivery adminresource.Record, notice adminresource.Record) (bool, error) {
	message, provider, prepareErr := w.smsMessageFromRecords(delivery, notice)
	if prepareErr != nil {
		return false, w.markDeliveryFailed(delivery, ChannelSMS, &provider, "notification delivery failed")
	}
	sender, err := w.smsSender(provider)
	if err != nil {
		return false, w.markDeliveryFailed(delivery, ChannelSMS, &provider, "notification delivery sender unavailable")
	}
	receipt, sendErr := sender.SendSMS(ctx, message)
	if sendErr != nil {
		if receipt.Provider == "" {
			receipt.Provider = sender.Kind()
		}
		if receipt.RedactedTarget == "" {
			receipt.RedactedTarget = RedactSMSTarget(message.Recipient)
		}
		return false, w.markDeliveryFailed(delivery, ChannelSMS, &receipt.Provider, "notification delivery failed")
	}
	if receipt.Provider == "" {
		receipt.Provider = sender.Kind()
	}
	if receipt.RedactedTarget == "" {
		receipt.RedactedTarget = RedactSMSTarget(message.Recipient)
	}
	return w.markDeliveryDelivered(delivery, DeliveryReceiptFromSMS(receipt))
}

func (w *DeliveryWorker) deliverMessage(ctx context.Context, channel string, delivery adminresource.Record, notice adminresource.Record) (bool, error) {
	message, provider, prepareErr := w.messageFromRecords(channel, delivery, notice)
	if prepareErr != nil {
		return false, w.markDeliveryFailed(delivery, channel, &provider, "notification delivery failed")
	}
	sender, err := w.messageSender(channel, provider)
	if err != nil {
		return false, w.markDeliveryFailed(delivery, channel, &provider, "notification delivery sender unavailable")
	}
	receipt, sendErr := sender.SendMessage(ctx, message)
	if sendErr != nil {
		if receipt.Channel == "" {
			receipt.Channel = channel
		}
		if receipt.Provider == "" {
			receipt.Provider = sender.Kind()
		}
		if receipt.RedactedTarget == "" {
			receipt.RedactedTarget = RedactMessageTarget(channel, message.Recipient)
		}
		return false, w.markDeliveryFailed(delivery, channel, &receipt.Provider, "notification delivery failed")
	}
	if receipt.Channel == "" {
		receipt.Channel = channel
	}
	if receipt.Provider == "" {
		receipt.Provider = sender.Kind()
	}
	if receipt.RedactedTarget == "" {
		receipt.RedactedTarget = RedactMessageTarget(channel, message.Recipient)
	}
	return w.markDeliveryDelivered(delivery, receipt)
}

func (w *DeliveryWorker) markDeliveryDelivered(delivery adminresource.Record, receipt DeliveryReceipt) (bool, error) {
	now := w.now().UTC()
	values := BuildDeliveryLedgerValues(DeliveryLedgerInput{
		BaseValues:     delivery.Values,
		Channel:        receipt.Channel,
		Target:         delivery.Values["target"],
		Receipt:        receipt,
		DeliveryStatus: DeliveryStatusDelivered,
		AttemptedAt:    now,
		DeliveredAt:    now,
	})
	_, err := w.store.UpdateInternalWithAudit(NotificationDeliveryResource, delivery.ID, adminresource.WriteInput{
		Code:        delivery.Code,
		Name:        delivery.Name,
		Status:      delivery.Status,
		Description: delivery.Description,
		Values:      values,
	}, adminresource.AuditEvent{Actor: w.actor, Action: deliveryWorkerAction, Result: "success", ReasonCode: DeliveryStatusDelivered})
	return true, err
}

func (w *DeliveryWorker) markDeliveryFailed(delivery adminresource.Record, channel string, provider *string, errorMessage string) error {
	providerValue := ""
	if provider != nil {
		providerValue = *provider
	}
	values := BuildDeliveryLedgerValues(DeliveryLedgerInput{
		BaseValues:     delivery.Values,
		Channel:        channel,
		Target:         delivery.Values["target"],
		Receipt:        DeliveryReceipt{Channel: channel, Provider: providerValue, Status: DeliveryStatusFailed},
		DeliveryStatus: DeliveryStatusFailed,
		ErrorMessage:   errorMessage,
		AttemptedAt:    w.now().UTC(),
	})
	_, err := w.store.UpdateInternalWithAudit(NotificationDeliveryResource, delivery.ID, adminresource.WriteInput{
		Code:        delivery.Code,
		Name:        delivery.Name,
		Status:      delivery.Status,
		Description: delivery.Description,
		Values:      values,
	}, adminresource.AuditEvent{Actor: w.actor, Action: deliveryWorkerAction, Result: "failed", ReasonCode: DeliveryStatusFailed})
	return err
}

func (w *DeliveryWorker) messageFromRecords(channel string, delivery adminresource.Record, notice adminresource.Record) (Message, string, error) {
	payload := parseNotificationPayload(notice.Values["payload"])
	provider := CanonicalProvider(delivery.Values["provider"])
	if provider == "" {
		provider = w.defaultProvider(channel)
	}
	templateID := firstNonBlank(delivery.Values["templateId"], notice.Values["templateId"], payload.TemplateID)
	recipient := strings.TrimSpace(delivery.Values["target"])
	if recipient == "" || strings.HasPrefix(recipient, "****") {
		return Message{}, provider, fmt.Errorf("notification delivery target is required")
	}
	params := firstTemplateParams(delivery.Values["templateParams"], notice.Values["templateParams"], payload.TemplateParams)
	return Message{
		TenantCode:     firstNonBlank(delivery.Values["tenantCode"], notice.Values["tenantCode"]),
		Channel:        channel,
		Recipient:      recipient,
		TemplateID:     templateID,
		TemplateParams: params,
		Title:          notice.Name,
		Body:           notice.Description,
		Purpose:        firstNonBlank(notice.Values["category"], payload.Purpose, "notification"),
		TraceID:        firstNonBlank(delivery.Values["traceId"], notice.Values["traceId"]),
	}, provider, nil
}

func (w *DeliveryWorker) smsMessageFromRecords(delivery adminresource.Record, notice adminresource.Record) (SMSMessage, string, error) {
	payload := parseNotificationPayload(notice.Values["payload"])
	provider := CanonicalSMSProvider(delivery.Values["provider"])
	if provider == "" {
		provider = w.defaultSMSProvider
	}
	templateID := firstNonBlank(delivery.Values["templateId"], notice.Values["templateId"], payload.TemplateID)
	recipient := strings.TrimSpace(delivery.Values["target"])
	if recipient == "" || strings.HasPrefix(recipient, "****") {
		return SMSMessage{}, provider, fmt.Errorf("sms delivery target is required")
	}
	params := firstTemplateParams(delivery.Values["templateParams"], notice.Values["templateParams"], payload.TemplateParams)
	return SMSMessage{
		TenantCode:     firstNonBlank(delivery.Values["tenantCode"], notice.Values["tenantCode"]),
		Recipient:      recipient,
		TemplateID:     templateID,
		TemplateParams: params,
		Purpose:        firstNonBlank(notice.Values["category"], payload.Purpose, "notification"),
		TraceID:        firstNonBlank(delivery.Values["traceId"], notice.Values["traceId"]),
	}, provider, nil
}

func (w *DeliveryWorker) smsSender(provider string) (SMSSender, error) {
	provider = CanonicalSMSProvider(provider)
	if provider == "" && len(w.smsSenders) == 1 {
		for _, sender := range w.smsSenders {
			return sender, nil
		}
	}
	if provider == "" {
		provider = SMSProviderMockLocal
	}
	if sender := w.smsSenders[provider]; !smsSenderNil(sender) {
		return sender, nil
	}
	if !w.allowDryRunFallback {
		return nil, fmt.Errorf("sms sender %q is not configured", provider)
	}
	return NewDryRunSMSSenderForProvider(provider)
}

func (w *DeliveryWorker) messageSender(channel string, provider string) (MessageSender, error) {
	channel = CanonicalChannel(channel)
	provider = CanonicalProvider(provider)
	if provider == "" && len(w.messageSenders) == 1 {
		for _, sender := range w.messageSenders {
			if CanonicalChannel(sender.Channel()) == channel {
				return sender, nil
			}
		}
	}
	if provider == "" {
		provider = w.defaultProvider(channel)
	}
	if provider == "" {
		provider = DefaultProviderForChannel(channel)
	}
	if sender := w.messageSenders[senderKey(channel, provider)]; !messageSenderNil(sender) {
		return sender, nil
	}
	if !w.allowDryRunFallback {
		return nil, fmt.Errorf("notification sender %q for channel %q is not configured", provider, channel)
	}
	return NewDryRunMessageSender(channel, provider)
}

func (w *DeliveryWorker) defaultProvider(channel string) string {
	channel = CanonicalChannel(channel)
	if channel == ChannelSMS && w.defaultSMSProvider != "" {
		return w.defaultSMSProvider
	}
	if provider := w.defaultProviders[channel]; provider != "" {
		return provider
	}
	return ""
}

func parseNotificationPayload(raw string) notificationPayload {
	var payload notificationPayload
	if strings.TrimSpace(raw) == "" {
		return payload
	}
	_ = json.Unmarshal([]byte(raw), &payload)
	return payload
}

func firstTemplateParams(rawValues ...any) map[string]string {
	for _, raw := range rawValues {
		switch value := raw.(type) {
		case string:
			if params := decodeTemplateParams(value); len(params) > 0 {
				return params
			}
		case map[string]string:
			if len(value) > 0 {
				return cloneWorkerStringMap(value)
			}
		}
	}
	return nil
}

func decodeTemplateParams(raw string) map[string]string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var params map[string]string
	if err := json.Unmarshal([]byte(raw), &params); err != nil {
		return nil
	}
	return cloneWorkerStringMap(params)
}

func deliveryAttempts(record adminresource.Record) int {
	attempts, err := strconv.Atoi(strings.TrimSpace(record.Values["attempts"]))
	if err != nil || attempts < 0 {
		return 0
	}
	return attempts
}

func cloneSMSDeliveryValues(values map[string]string) map[string]string {
	cloned := make(map[string]string, len(values)+4)
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func cloneWorkerStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func smsSenderNil(sender SMSSender) bool {
	if sender == nil {
		return true
	}
	return false
}

func messageSenderNil(sender MessageSender) bool {
	if sender == nil {
		return true
	}
	return false
}
