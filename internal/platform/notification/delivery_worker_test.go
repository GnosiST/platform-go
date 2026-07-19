package notification

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/GnosiST/platform-go/internal/platform/adminresource"
	"github.com/GnosiST/platform-go/internal/platform/core"
)

func TestDeliveryWorkerDeliversPendingSMSWithMockLocal(t *testing.T) {
	store := notificationWorkerTestStore(t)
	notice := createNotificationWorkerNotice(t, store, map[string]string{
		"tenantCode": "platform",
		"category":   "login",
		"payload":    `{"templateId":"SMS_LOGIN","templateParams":{"code":"123456"}}`,
	})
	delivery := createNotificationWorkerDelivery(t, store, map[string]string{
		"tenantCode":       "platform",
		"notificationCode": notice.Code,
		"channel":          ChannelSMS,
		"deliveryStatus":   "pending",
		"target":           "+8613800000000",
		"provider":         SMSProviderMockLocal,
		"attempts":         "0",
	})
	sender := NewMockLocalSMSSender()
	now := time.Date(2026, 7, 19, 10, 30, 0, 0, time.UTC)
	worker := NewDeliveryWorker(store, DeliveryWorkerOptions{
		SMSSenders: map[string]SMSSender{SMSProviderMockLocal: sender},
		Now:        func() time.Time { return now },
	})

	result, err := worker.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if result.Scanned == 0 || result.Attempted != 1 || result.Delivered != 1 || result.Failed != 0 {
		t.Fatalf("RunOnce() result = %+v, want one delivered attempt", result)
	}
	sent := sender.Sent()
	if len(sent) != 1 {
		t.Fatalf("sent messages = %+v, want one SMS", sent)
	}
	if sent[0].TenantCode != "platform" || sent[0].Recipient != "+8613800000000" || sent[0].TemplateID != "SMS_LOGIN" || sent[0].TemplateParams["code"] != "123456" || sent[0].Purpose != "login" {
		t.Fatalf("sent message = %+v, want hydrated message from notification and delivery records", sent[0])
	}
	updated, err := store.InternalRecord("notification-deliveries", delivery.ID)
	if err != nil {
		t.Fatalf("InternalRecord(notification-deliveries) error = %v", err)
	}
	if updated.Values["deliveryStatus"] != "delivered" ||
		updated.Values["attempts"] != "1" ||
		updated.Values["target"] != "****0000" ||
		updated.Values["provider"] != SMSProviderMockLocal ||
		!strings.HasPrefix(updated.Values["providerMessageId"], "mock-local-") ||
		updated.Values["lastAttemptAt"] != now.Format(time.RFC3339) ||
		updated.Values["deliveredAt"] != now.Format(time.RFC3339) ||
		updated.Values["errorMessage"] != "" {
		t.Fatalf("updated delivery values = %+v, want delivered redacted ledger", updated.Values)
	}
}

func TestDeliveryWorkerUsesVendorDryRunSender(t *testing.T) {
	store := notificationWorkerTestStore(t)
	notice := createNotificationWorkerNotice(t, store, map[string]string{
		"tenantCode": "platform",
		"category":   "marketing",
		"payload":    `{"templateId":"SMS_PROMO"}`,
	})
	delivery := createNotificationWorkerDelivery(t, store, map[string]string{
		"tenantCode":       "platform",
		"notificationCode": notice.Code,
		"channel":          ChannelSMS,
		"deliveryStatus":   "pending",
		"target":           "+8613900000000",
		"provider":         SMSProviderAliyun,
		"attempts":         "2",
	})
	worker := NewDeliveryWorker(store, DeliveryWorkerOptions{AllowDryRunFallback: true})

	result, err := worker.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if result.Attempted != 1 || result.Delivered != 1 || result.Failed != 0 {
		t.Fatalf("RunOnce() result = %+v, want one delivered dry-run attempt", result)
	}
	updated, err := store.InternalRecord("notification-deliveries", delivery.ID)
	if err != nil {
		t.Fatalf("InternalRecord(notification-deliveries) error = %v", err)
	}
	if updated.Values["deliveryStatus"] != "delivered" ||
		updated.Values["attempts"] != "3" ||
		updated.Values["provider"] != SMSProviderAliyun ||
		!strings.HasPrefix(updated.Values["providerMessageId"], "aliyun-dry-run-") ||
		updated.Values["target"] != "****0000" {
		t.Fatalf("updated delivery values = %+v, want aliyun dry-run ledger", updated.Values)
	}
}

func TestDeliveryWorkerRequiresConfiguredSenderUnlessDryRunFallbackEnabled(t *testing.T) {
	store := notificationWorkerTestStore(t)
	notice := createNotificationWorkerNotice(t, store, map[string]string{
		"tenantCode": "platform",
		"category":   "marketing",
		"payload":    `{"templateId":"SMS_PROMO"}`,
	})
	delivery := createNotificationWorkerDelivery(t, store, map[string]string{
		"tenantCode":       "platform",
		"notificationCode": notice.Code,
		"channel":          ChannelSMS,
		"deliveryStatus":   "pending",
		"target":           "+8613900000000",
		"provider":         SMSProviderAliyun,
		"attempts":         "0",
	})

	result, err := NewDeliveryWorker(store, DeliveryWorkerOptions{}).RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if result.Attempted != 1 || result.Delivered != 0 || result.Failed != 1 {
		t.Fatalf("RunOnce() result = %+v, want one failed unconfigured sender attempt", result)
	}
	updated, err := store.InternalRecord("notification-deliveries", delivery.ID)
	if err != nil {
		t.Fatalf("InternalRecord(notification-deliveries) error = %v", err)
	}
	if updated.Values["deliveryStatus"] != "failed" ||
		updated.Values["provider"] != SMSProviderAliyun ||
		updated.Values["target"] != "****0000" ||
		updated.Values["errorMessage"] != "notification delivery sender unavailable" {
		t.Fatalf("updated delivery values = %+v, want failed unconfigured sender ledger", updated.Values)
	}
}

func TestDeliveryWorkerMarksFailedDeliveryWithRedactedTarget(t *testing.T) {
	store := notificationWorkerTestStore(t)
	notice := createNotificationWorkerNotice(t, store, map[string]string{
		"tenantCode": "platform",
		"category":   "login",
		"payload":    `{"templateId":"SMS_LOGIN"}`,
	})
	delivery := createNotificationWorkerDelivery(t, store, map[string]string{
		"tenantCode":       "platform",
		"notificationCode": notice.Code,
		"channel":          ChannelSMS,
		"deliveryStatus":   "pending",
		"target":           "+8613800000000",
		"provider":         SMSProviderTencent,
		"attempts":         "0",
	})
	worker := NewDeliveryWorker(store, DeliveryWorkerOptions{
		SMSSenders: map[string]SMSSender{SMSProviderTencent: failingSMSSender{provider: SMSProviderTencent}},
	})

	result, err := worker.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if result.Attempted != 1 || result.Delivered != 0 || result.Failed != 1 {
		t.Fatalf("RunOnce() result = %+v, want one failed attempt", result)
	}
	updated, err := store.InternalRecord("notification-deliveries", delivery.ID)
	if err != nil {
		t.Fatalf("InternalRecord(notification-deliveries) error = %v", err)
	}
	if updated.Values["deliveryStatus"] != "failed" ||
		updated.Values["attempts"] != "1" ||
		updated.Values["target"] != "****0000" ||
		updated.Values["provider"] != SMSProviderTencent ||
		updated.Values["providerMessageId"] != "" ||
		updated.Values["errorMessage"] != "notification delivery failed" {
		t.Fatalf("updated delivery values = %+v, want failed redacted ledger", updated.Values)
	}
	if strings.Contains(updated.Values["errorMessage"], "+8613800000000") {
		t.Fatalf("errorMessage leaked recipient: %+v", updated.Values)
	}
}

type failingSMSSender struct {
	provider string
}

func (s failingSMSSender) Kind() string {
	return s.provider
}

func (s failingSMSSender) SendSMS(context.Context, SMSMessage) (SMSDeliveryReceipt, error) {
	return SMSDeliveryReceipt{}, errors.New("provider rejected +8613800000000")
}

func notificationWorkerTestStore(t *testing.T) *adminresource.Store {
	t.Helper()
	return adminresource.NewStoreFromCapabilities(core.DefaultManifests())
}

func createNotificationWorkerNotice(t *testing.T, store *adminresource.Store, values map[string]string) adminresource.Record {
	t.Helper()
	result, err := store.CreateInternalWithAudit("notifications", adminresource.WriteInput{
		Code:        "notice-worker-" + values["category"],
		Name:        "Worker Notice",
		Status:      "sent",
		Description: "Worker notice body.",
		Values:      values,
	}, adminresource.AuditEvent{Actor: "test", Action: "notification.worker.test.seed", Result: "success", ReasonCode: "seeded"})
	if err != nil {
		t.Fatalf("CreateInternalWithAudit(notifications) error = %v", err)
	}
	return result.Record
}

func createNotificationWorkerDelivery(t *testing.T, store *adminresource.Store, values map[string]string) adminresource.Record {
	t.Helper()
	result, err := store.CreateInternalWithAudit("notification-deliveries", adminresource.WriteInput{
		Code:        "delivery-worker-" + values["provider"],
		Name:        "Worker Delivery",
		Status:      "enabled",
		Description: "Worker delivery ledger.",
		Values:      values,
	}, adminresource.AuditEvent{Actor: "test", Action: "notification.worker.test.seed", Result: "success", ReasonCode: "seeded"})
	if err != nil {
		t.Fatalf("CreateInternalWithAudit(notification-deliveries) error = %v", err)
	}
	return result.Record
}
