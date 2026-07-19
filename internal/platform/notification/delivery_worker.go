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

type DeliveryWorkerOptions struct {
	SMSSenders          map[string]SMSSender
	DefaultSMSProvider  string
	MaxBatch            int
	Now                 func() time.Time
	Actor               string
	AllowDryRunFallback bool
}

type DeliveryWorker struct {
	store               DeliveryStore
	smsSenders          map[string]SMSSender
	defaultSMSProvider  string
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
	return &DeliveryWorker{
		store:               store,
		smsSenders:          senders,
		defaultSMSProvider:  defaultProvider,
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
		if !pendingSMSDelivery(delivery) {
			result.Skipped++
			continue
		}
		result.Attempted++
		notice, ok := noticeByCode[strings.TrimSpace(delivery.Values["notificationCode"])]
		if !ok {
			if err := w.markDeliveryFailed(delivery, nil, "notification record not found"); err != nil {
				return result, err
			}
			result.Failed++
			continue
		}
		delivered, err := w.deliverSMS(ctx, delivery, notice)
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
	if record.Status == "disabled" {
		return false
	}
	values := record.Values
	return strings.EqualFold(strings.TrimSpace(values["channel"]), ChannelSMS) &&
		strings.EqualFold(strings.TrimSpace(values["deliveryStatus"]), DeliveryStatusPending)
}

func (w *DeliveryWorker) deliverSMS(ctx context.Context, delivery adminresource.Record, notice adminresource.Record) (bool, error) {
	message, provider, prepareErr := w.smsMessageFromRecords(delivery, notice)
	if prepareErr != nil {
		return false, w.markDeliveryFailed(delivery, &provider, "notification delivery failed")
	}
	sender, err := w.smsSender(provider)
	if err != nil {
		return false, w.markDeliveryFailed(delivery, &provider, "notification delivery sender unavailable")
	}
	receipt, sendErr := sender.SendSMS(ctx, message)
	if sendErr != nil {
		if receipt.Provider == "" {
			receipt.Provider = sender.Kind()
		}
		if receipt.RedactedTarget == "" {
			receipt.RedactedTarget = RedactSMSTarget(message.Recipient)
		}
		return false, w.markDeliveryFailed(delivery, &receipt.Provider, "notification delivery failed")
	}
	if receipt.Provider == "" {
		receipt.Provider = sender.Kind()
	}
	if receipt.RedactedTarget == "" {
		receipt.RedactedTarget = RedactSMSTarget(message.Recipient)
	}
	now := w.now().UTC().Format(time.RFC3339)
	values := cloneSMSDeliveryValues(delivery.Values)
	values["deliveryStatus"] = DeliveryStatusDelivered
	values["attempts"] = strconv.Itoa(deliveryAttempts(delivery) + 1)
	values["lastAttemptAt"] = now
	values["deliveredAt"] = now
	values["target"] = receipt.RedactedTarget
	values["provider"] = receipt.Provider
	values["providerMessageId"] = receipt.MessageID
	delete(values, "errorMessage")
	_, err = w.store.UpdateInternalWithAudit(NotificationDeliveryResource, delivery.ID, adminresource.WriteInput{
		Code:        delivery.Code,
		Name:        delivery.Name,
		Status:      delivery.Status,
		Description: delivery.Description,
		Values:      values,
	}, adminresource.AuditEvent{Actor: w.actor, Action: deliveryWorkerAction, Result: "success", ReasonCode: DeliveryStatusDelivered})
	return true, err
}

func (w *DeliveryWorker) markDeliveryFailed(delivery adminresource.Record, provider *string, errorMessage string) error {
	now := w.now().UTC().Format(time.RFC3339)
	values := cloneSMSDeliveryValues(delivery.Values)
	values["deliveryStatus"] = DeliveryStatusFailed
	values["attempts"] = strconv.Itoa(deliveryAttempts(delivery) + 1)
	values["lastAttemptAt"] = now
	values["target"] = RedactSMSTarget(values["target"])
	if provider != nil && strings.TrimSpace(*provider) != "" {
		values["provider"] = CanonicalSMSProvider(*provider)
	}
	delete(values, "providerMessageId")
	values["errorMessage"] = strings.TrimSpace(errorMessage)
	if values["errorMessage"] == "" {
		values["errorMessage"] = "notification delivery failed"
	}
	_, err := w.store.UpdateInternalWithAudit(NotificationDeliveryResource, delivery.ID, adminresource.WriteInput{
		Code:        delivery.Code,
		Name:        delivery.Name,
		Status:      delivery.Status,
		Description: delivery.Description,
		Values:      values,
	}, adminresource.AuditEvent{Actor: w.actor, Action: deliveryWorkerAction, Result: "failed", ReasonCode: DeliveryStatusFailed})
	return err
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
