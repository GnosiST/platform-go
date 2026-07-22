package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/GnosiST/platform-go/internal/platform/adminresource"
	"github.com/GnosiST/platform-go/internal/platform/errorcode"
	"github.com/GnosiST/platform-go/internal/platform/kernel"
	"github.com/GnosiST/platform-go/internal/platform/notification"
	"github.com/GnosiST/platform-go/internal/platform/ratelimit"
)

type adminMessageCenterTestSendPayload struct {
	Data struct {
		Notification adminResourceRecordTest `json:"notification"`
		Delivery     adminResourceRecordTest `json:"delivery"`
		Receipt      struct {
			Channel        string `json:"channel"`
			Provider       string `json:"provider"`
			MessageID      string `json:"messageId"`
			Status         string `json:"status"`
			RedactedTarget string `json:"redactedTarget"`
		} `json:"receipt"`
	} `json:"data"`
	Error *ErrorBody `json:"error"`
}

func TestAdminMessageCenterTestSendRequiresConfiguredSMSRuntime(t *testing.T) {
	server := newTestServer(ServerOptions{
		Capabilities: capabilitiesFromConfigForTest(t, []string{"dictionary", "tenant", "identity", "session", "rbac", "audit", "notification"}),
	})
	login := loginForTest(t, server, "admin")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/admin/message-center/test-send", bytes.NewBufferString(`{
		"channel": "sms",
		"recipient": "+8613800000000",
		"templateId": "SMS_TEST"
	}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+login.Data.Token)
	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("POST message-center test-send status = %d body = %s, want 503", recorder.Code, recorder.Body.String())
	}
	var payload adminMessageCenterTestSendPayload
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v body = %s", err, recorder.Body.String())
	}
	if payload.Error == nil || payload.Error.Code != errorcode.CodeAdminMessageCenterUnavailable {
		t.Fatalf("error = %+v, want %s", payload.Error, errorcode.CodeAdminMessageCenterUnavailable)
	}
}

func TestAdminMessageCenterTestSendSupportsEmailDryRun(t *testing.T) {
	server := newTestServer(ServerOptions{
		Capabilities: capabilitiesFromConfigForTest(t, []string{"dictionary", "tenant", "identity", "session", "rbac", "audit", "notification"}),
	})
	login := loginForTest(t, server, "admin")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/admin/message-center/test-send", bytes.NewBufferString(`{
		"channel": "email",
		"tenantCode": "platform",
		"recipient": "owner@example.com",
		"templateId": "EMAIL_TEST",
		"templateParams": {"name": "Admin"},
		"title": "邮箱试发送",
		"body": "这是一条消息中心邮箱 dry-run"
	}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+login.Data.Token)
	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("POST message-center email test-send status = %d body = %s, want 201", recorder.Code, recorder.Body.String())
	}
	var payload adminMessageCenterTestSendPayload
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v body = %s", err, recorder.Body.String())
	}
	if payload.Data.Receipt.Channel != notification.ChannelEmail ||
		payload.Data.Receipt.Provider != notification.EmailProviderSMTP ||
		payload.Data.Receipt.Status != notification.GenericDeliveryDryRunState ||
		payload.Data.Receipt.RedactedTarget != "ow***@example.com" {
		t.Fatalf("receipt = %+v, want email dry-run receipt", payload.Data.Receipt)
	}
	if payload.Data.Delivery.Values["notificationCode"] != payload.Data.Notification.Code ||
		payload.Data.Delivery.Values["channel"] != notification.ChannelEmail ||
		payload.Data.Delivery.Values["deliveryStatus"] != "delivered" ||
		payload.Data.Delivery.Values["target"] != "ow***@example.com" ||
		payload.Data.Delivery.Values["provider"] != notification.EmailProviderSMTP ||
		payload.Data.Delivery.Values["providerMessageId"] == "" {
		t.Fatalf("delivery = %+v, want delivered email dry-run ledger linked to notification", payload.Data.Delivery)
	}
}

func TestAdminMessageCenterTestSendCreatesNotificationAndDelivery(t *testing.T) {
	sender := notification.NewMockLocalSMSSender()
	server := newTestServer(ServerOptions{
		Capabilities:          capabilitiesFromConfigForTest(t, []string{"dictionary", "tenant", "identity", "session", "rbac", "audit", "notification"}),
		NotificationSMSSender: sender,
	})
	login := loginForTest(t, server, "admin")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/admin/message-center/test-send", bytes.NewBufferString(`{
		"channel": "sms",
		"tenantCode": "platform",
		"recipient": "+8613800000000",
		"templateId": "SMS_TEST",
		"templateParams": {"code": "123456"},
		"title": "登录验证码测试",
		"body": "这是一条消息中心试发送"
	}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+login.Data.Token)
	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("POST message-center test-send status = %d body = %s, want 201", recorder.Code, recorder.Body.String())
	}
	var payload adminMessageCenterTestSendPayload
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v body = %s", err, recorder.Body.String())
	}
	if payload.Data.Receipt.Provider != notification.SMSProviderMockLocal || payload.Data.Receipt.RedactedTarget != "****0000" {
		t.Fatalf("receipt = %+v, want mock-local redacted target", payload.Data.Receipt)
	}
	if payload.Data.Notification.Values["category"] != "message_center_test" || payload.Data.Notification.Status != "sent" {
		t.Fatalf("notification = %+v, want sent message-center test record", payload.Data.Notification)
	}
	if payload.Data.Delivery.Values["notificationCode"] != payload.Data.Notification.Code ||
		payload.Data.Delivery.Values["channel"] != notification.ChannelSMS ||
		payload.Data.Delivery.Values["deliveryStatus"] != "delivered" ||
		payload.Data.Delivery.Values["target"] != "****0000" ||
		payload.Data.Delivery.Values["provider"] != notification.SMSProviderMockLocal ||
		payload.Data.Delivery.Values["providerMessageId"] == "" ||
		payload.Data.Delivery.Values["providerStatus"] != notification.SMSDeliveryAccepted {
		t.Fatalf("delivery = %+v, want delivered SMS ledger linked to notification", payload.Data.Delivery)
	}
	correlation := kernel.Correlation{
		RequestID: payload.Data.Delivery.Values["requestId"],
		TraceID:   payload.Data.Delivery.Values["traceId"],
	}
	if !kernel.ValidCorrelation(correlation) {
		t.Fatalf("delivery correlation = %+v, want canonical request/trace ids", payload.Data.Delivery.Values)
	}
	for _, marker := range []string{"+8613800000000", "123456"} {
		if strings.Contains(recorder.Body.String(), marker) {
			t.Fatalf("message-center response leaked %q: %s", marker, recorder.Body.String())
		}
	}
	sent := sender.Sent()
	if len(sent) != 1 || sent[0].Recipient != "+8613800000000" || sent[0].TemplateID != "SMS_TEST" || sent[0].TemplateParams["code"] != "123456" {
		t.Fatalf("sent messages = %+v, want one SMS with template params", sent)
	}
}

func TestAdminMessageCenterResourceQueriesRedactPayloadAndDeliveryTarget(t *testing.T) {
	server := newTestServer(ServerOptions{
		Capabilities: capabilitiesFromConfigForTest(t, []string{"dictionary", "tenant", "identity", "session", "rbac", "audit", "notification"}),
	})
	login := loginForTest(t, server, "admin")
	notice := createHTTPMessageCenterWorkerNotice(t, server.resources, map[string]string{
		"tenantCode": "platform",
		"category":   "response-redaction",
		"payload":    `{"channel":"sms","redactedTarget":"+8613800000000","templateId":"SMS_SECRET","templateParams":{"code":"123456"}}`,
	})
	createHTTPMessageCenterWorkerDelivery(t, server.resources, map[string]string{
		"tenantCode":       "platform",
		"notificationCode": notice.Code,
		"channel":          notification.ChannelSMS,
		"deliveryStatus":   notification.DeliveryStatusPending,
		"target":           "+8613800000000",
		"provider":         notification.SMSProviderMockLocal,
		"attempts":         "0",
		"errorMessage":     "vendor rejected +8613800000000 with code 123456",
	})

	notificationRecorder := httptest.NewRecorder()
	notificationRequest := httptest.NewRequest(http.MethodPost, "/api/admin/resources/notifications/query", bytes.NewBufferString(`{"page":1,"pageSize":10}`))
	notificationRequest.Header.Set("Authorization", "Bearer "+login.Data.Token)
	notificationRequest.Header.Set("Content-Type", "application/json")
	server.Router().ServeHTTP(notificationRecorder, notificationRequest)

	if notificationRecorder.Code != http.StatusOK {
		t.Fatalf("POST notifications query status = %d body = %s, want 200", notificationRecorder.Code, notificationRecorder.Body.String())
	}
	var notificationPayload adminResourceQueryTestPayload
	if err := json.Unmarshal(notificationRecorder.Body.Bytes(), &notificationPayload); err != nil {
		t.Fatalf("decode notifications query: %v body = %s", err, notificationRecorder.Body.String())
	}
	if len(notificationPayload.Data.Items) != 1 {
		t.Fatalf("notifications query items = %+v, want one notice", notificationPayload.Data.Items)
	}
	safePayload := notificationPayload.Data.Items[0].Values["payload"]
	if !strings.Contains(safePayload, `"templateParamKeys":["code"]`) || strings.Contains(safePayload, "templateParams") || strings.Contains(notificationRecorder.Body.String(), "123456") || strings.Contains(notificationRecorder.Body.String(), "+8613800000000") {
		t.Fatalf("notifications query leaked sensitive payload: record=%+v body=%s", notificationPayload.Data.Items[0], notificationRecorder.Body.String())
	}

	deliveryRecorder := httptest.NewRecorder()
	deliveryRequest := httptest.NewRequest(http.MethodPost, "/api/admin/resources/notification-deliveries/query", bytes.NewBufferString(`{"page":1,"pageSize":10}`))
	deliveryRequest.Header.Set("Authorization", "Bearer "+login.Data.Token)
	deliveryRequest.Header.Set("Content-Type", "application/json")
	server.Router().ServeHTTP(deliveryRecorder, deliveryRequest)

	if deliveryRecorder.Code != http.StatusOK {
		t.Fatalf("POST notification-deliveries query status = %d body = %s, want 200", deliveryRecorder.Code, deliveryRecorder.Body.String())
	}
	var deliveryPayload adminResourceQueryTestPayload
	if err := json.Unmarshal(deliveryRecorder.Body.Bytes(), &deliveryPayload); err != nil {
		t.Fatalf("decode deliveries query: %v body = %s", err, deliveryRecorder.Body.String())
	}
	if len(deliveryPayload.Data.Items) != 1 ||
		deliveryPayload.Data.Items[0].Values["target"] != "****0000" ||
		deliveryPayload.Data.Items[0].Values["errorMessage"] != notification.DeliveryLedgerErrorFailed ||
		strings.Contains(deliveryRecorder.Body.String(), "+8613800000000") ||
		strings.Contains(deliveryRecorder.Body.String(), "123456") {
		t.Fatalf("deliveries query leaked raw target: payload=%+v body=%s", deliveryPayload, deliveryRecorder.Body.String())
	}
}

func TestAdminMessageCenterTestSendAppliesRuntimeSendPolicyLimit(t *testing.T) {
	builder, err := ratelimit.NewKeyBuilder([]byte(strings.Repeat("r", 32)))
	if err != nil {
		t.Fatalf("NewKeyBuilder() error = %v", err)
	}
	limiter := &nthMessageCenterRateLimiter{denyAt: 1, retryAfter: 45 * time.Second}
	sender := notification.NewMockLocalSMSSender()
	server := newTestServer(ServerOptions{
		Capabilities:          capabilitiesFromConfigForTest(t, []string{"dictionary", "tenant", "identity", "session", "rbac", "audit", "notification"}),
		NotificationSMSSender: sender,
		RateLimiter:           limiter,
		RateLimitKeyBuilder:   builder,
	})
	login := loginForTest(t, server, "admin")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/admin/message-center/test-send", bytes.NewBufferString(`{
		"channel": "sms",
		"tenantCode": "platform",
		"recipient": "+8613800000000",
		"templateId": "SMS_TEST",
		"templateParams": {"code": "123456"},
		"title": "登录验证码测试",
		"body": "这是一条消息中心试发送"
	}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+login.Data.Token)
	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusTooManyRequests || recorder.Header().Get("Retry-After") != "45" {
		t.Fatalf("POST message-center test-send rate limited status = %d Retry-After=%q body=%s", recorder.Code, recorder.Header().Get("Retry-After"), recorder.Body.String())
	}
	var payload adminMessageCenterTestSendPayload
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v body = %s", err, recorder.Body.String())
	}
	if payload.Error == nil || payload.Error.Code != errorcode.CodeRateLimited {
		t.Fatalf("error = %+v, want %s", payload.Error, errorcode.CodeRateLimited)
	}
	if len(sender.Sent()) != 0 {
		t.Fatalf("sent messages = %+v, want no SMS after runtime send-policy limit", sender.Sent())
	}
	if limiter.calls != 1 || len(limiter.keys) != 1 {
		t.Fatalf("runtime limiter calls=%d keys=%+v, want one dynamic policy check", limiter.calls, limiter.keys)
	}
	for _, marker := range []string{"+8613800000000", "13800000000", "123456"} {
		if strings.Contains(limiter.keys[0], marker) || strings.Contains(recorder.Body.String(), marker) {
			t.Fatalf("message-center rate limit leaked marker %q in key=%q body=%s", marker, limiter.keys[0], recorder.Body.String())
		}
	}
}

func TestAdminMessageCenterDeliveryRunSkipsPendingWhenRuntimePolicyLimited(t *testing.T) {
	builder, err := ratelimit.NewKeyBuilder([]byte(strings.Repeat("r", 32)))
	if err != nil {
		t.Fatalf("NewKeyBuilder() error = %v", err)
	}
	limiter := &nthMessageCenterRateLimiter{denyAt: 2, retryAfter: time.Minute}
	sender := notification.NewMockLocalSMSSender()
	server := newTestServer(ServerOptions{
		Capabilities:          capabilitiesFromConfigForTest(t, []string{"dictionary", "tenant", "identity", "session", "rbac", "audit", "notification"}),
		NotificationSMSSender: sender,
		RateLimiter:           limiter,
		RateLimitKeyBuilder:   builder,
	})
	login := loginForTest(t, server, "admin")
	notice := createHTTPMessageCenterWorkerNotice(t, server.resources, map[string]string{
		"tenantCode": "platform",
		"category":   "ops",
		"payload":    `{"templateId":"SMS_OPS","templateParams":{"ticket":"T-100"}}`,
	})
	delivery := createHTTPMessageCenterWorkerDelivery(t, server.resources, map[string]string{
		"tenantCode":       "platform",
		"notificationCode": notice.Code,
		"channel":          notification.ChannelSMS,
		"deliveryStatus":   notification.DeliveryStatusPending,
		"target":           "+8613800000000",
		"provider":         notification.SMSProviderMockLocal,
		"attempts":         "0",
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/admin/message-center/deliveries/run", nil)
	request.Header.Set("Authorization", "Bearer "+login.Data.Token)
	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("POST message-center deliveries run status = %d body = %s, want 200", recorder.Code, recorder.Body.String())
	}
	var payload adminMessageCenterDeliveryRunTestPayload
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode delivery run response: %v body = %s", err, recorder.Body.String())
	}
	if payload.Data.Attempted != 0 || payload.Data.Delivered != 0 || payload.Data.Failed != 0 || payload.Data.Skipped != 1 {
		t.Fatalf("delivery run payload = %+v, want one skipped pending delivery", payload.Data)
	}
	if len(sender.Sent()) != 0 {
		t.Fatalf("sent messages = %+v, want no SMS when runtime policy limit denies worker delivery", sender.Sent())
	}
	updated, err := server.resources.InternalRecord("notification-deliveries", delivery.ID)
	if err != nil {
		t.Fatalf("InternalRecord(notification-deliveries) error = %v", err)
	}
	if updated.Values["deliveryStatus"] != notification.DeliveryStatusPending ||
		updated.Values["attempts"] != "0" ||
		updated.Values["target"] != "+8613800000000" {
		t.Fatalf("updated delivery values = %+v, want untouched pending delivery after policy skip", updated.Values)
	}
	if limiter.calls != 2 {
		t.Fatalf("runtime limiter calls = %d keys=%+v, want static run limit plus one dynamic policy check", limiter.calls, limiter.keys)
	}
	for _, marker := range []string{"+8613800000000", "13800000000", "T-100"} {
		for _, key := range limiter.keys {
			if strings.Contains(key, marker) {
				t.Fatalf("message-center worker rate limit leaked marker %q in key=%q", marker, key)
			}
		}
	}
}

func TestAdminMessageCenterDeliveryRunSupportsEmailDryRunWithoutConfiguredSender(t *testing.T) {
	now := time.Date(2026, 7, 19, 11, 35, 0, 0, time.UTC)
	server := newTestServer(ServerOptions{
		Capabilities: capabilitiesFromConfigForTest(t, []string{"dictionary", "tenant", "identity", "session", "rbac", "audit", "notification"}),
		Now:          func() time.Time { return now },
	})
	login := loginForTest(t, server, "admin")
	notice := createHTTPMessageCenterWorkerNotice(t, server.resources, map[string]string{
		"tenantCode": "platform",
		"category":   "email-dry-run",
		"payload":    `{"templateId":"EMAIL_OPS","templateParams":{"ticket":"T-EMAIL-100"}}`,
	})
	delivery := createHTTPMessageCenterWorkerDelivery(t, server.resources, map[string]string{
		"tenantCode":       "platform",
		"notificationCode": notice.Code,
		"channel":          notification.ChannelEmail,
		"deliveryStatus":   notification.DeliveryStatusPending,
		"target":           "owner@example.com",
		"provider":         notification.EmailProviderSMTP,
		"attempts":         "0",
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/admin/message-center/deliveries/run", nil)
	request.Header.Set("Authorization", "Bearer "+login.Data.Token)
	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("POST message-center deliveries run email dry-run status = %d body = %s, want 200", recorder.Code, recorder.Body.String())
	}
	var payload adminMessageCenterDeliveryRunTestPayload
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode delivery run response: %v body = %s", err, recorder.Body.String())
	}
	if payload.Data.Attempted != 1 || payload.Data.Delivered != 1 || payload.Data.Failed != 0 {
		t.Fatalf("delivery run payload = %+v, want one delivered email dry-run attempt", payload.Data)
	}
	updated, err := server.resources.InternalRecord("notification-deliveries", delivery.ID)
	if err != nil {
		t.Fatalf("InternalRecord(notification-deliveries) error = %v", err)
	}
	if updated.Values["deliveryStatus"] != notification.DeliveryStatusDelivered ||
		updated.Values["attempts"] != "1" ||
		updated.Values["target"] != "ow***@example.com" ||
		updated.Values["provider"] != notification.EmailProviderSMTP ||
		!strings.HasPrefix(updated.Values["providerMessageId"], "email-dry-run-") ||
		updated.Values["providerStatus"] != notification.GenericDeliveryDryRunState ||
		updated.Values["lastAttemptAt"] != now.Format(time.RFC3339) ||
		updated.Values["deliveredAt"] != now.Format(time.RFC3339) {
		t.Fatalf("updated delivery values = %+v, want delivered email dry-run ledger", updated.Values)
	}
	if strings.Contains(fmt.Sprintf("%+v", updated), "owner@example.com") || strings.Contains(fmt.Sprintf("%+v", updated), "T-EMAIL-100") {
		t.Fatalf("email dry-run delivery leaked sensitive values: %+v", updated)
	}
}

func TestAdminMessageCenterDeliveryRunSupportsInAppDryRunWithoutConfiguredSender(t *testing.T) {
	now := time.Date(2026, 7, 19, 11, 40, 0, 0, time.UTC)
	server := newTestServer(ServerOptions{
		Capabilities: capabilitiesFromConfigForTest(t, []string{"dictionary", "tenant", "identity", "session", "rbac", "audit", "notification"}),
		Now:          func() time.Time { return now },
	})
	login := loginForTest(t, server, "admin")
	notice := createHTTPMessageCenterWorkerNotice(t, server.resources, map[string]string{
		"tenantCode": "platform",
		"category":   "in-app-dry-run",
		"payload":    `{"templateId":"IN_APP_OPS","templateParams":{"ticket":"T-INAPP-100"}}`,
	})
	delivery := createHTTPMessageCenterWorkerDelivery(t, server.resources, map[string]string{
		"tenantCode":       "platform",
		"notificationCode": notice.Code,
		"channel":          notification.ChannelInApp,
		"deliveryStatus":   notification.DeliveryStatusPending,
		"target":           "admin-user-1",
		"provider":         notification.InAppProviderLocal,
		"attempts":         "0",
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/admin/message-center/deliveries/run", nil)
	request.Header.Set("Authorization", "Bearer "+login.Data.Token)
	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("POST message-center deliveries run in-app dry-run status = %d body = %s, want 200", recorder.Code, recorder.Body.String())
	}
	var payload adminMessageCenterDeliveryRunTestPayload
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode delivery run response: %v body = %s", err, recorder.Body.String())
	}
	if payload.Data.Attempted != 1 || payload.Data.Delivered != 1 || payload.Data.Failed != 0 {
		t.Fatalf("delivery run payload = %+v, want one delivered in-app dry-run attempt", payload.Data)
	}
	updated, err := server.resources.InternalRecord("notification-deliveries", delivery.ID)
	if err != nil {
		t.Fatalf("InternalRecord(notification-deliveries) error = %v", err)
	}
	if updated.Values["deliveryStatus"] != notification.DeliveryStatusDelivered ||
		updated.Values["attempts"] != "1" ||
		updated.Values["target"] != "****er-1" ||
		updated.Values["provider"] != notification.InAppProviderLocal ||
		!strings.HasPrefix(updated.Values["providerMessageId"], "in_app-dry-run-") ||
		updated.Values["providerStatus"] != notification.GenericDeliveryDryRunState ||
		updated.Values["lastAttemptAt"] != now.Format(time.RFC3339) ||
		updated.Values["deliveredAt"] != now.Format(time.RFC3339) {
		t.Fatalf("updated delivery values = %+v, want delivered in-app dry-run ledger", updated.Values)
	}
	if strings.Contains(fmt.Sprintf("%+v", updated), "admin-user-1") || strings.Contains(fmt.Sprintf("%+v", updated), "T-INAPP-100") {
		t.Fatalf("in-app dry-run delivery leaked sensitive values: %+v", updated)
	}
}

func TestAdminMessageCenterDeliveryRetryQueuesFailedDeliveryWithWriteOnlyRecipient(t *testing.T) {
	now := time.Date(2026, 7, 19, 11, 45, 0, 0, time.UTC)
	sender := notification.NewMockLocalSMSSender()
	server := newTestServer(ServerOptions{
		Capabilities:          capabilitiesFromConfigForTest(t, []string{"dictionary", "tenant", "identity", "session", "rbac", "audit", "notification"}),
		NotificationSMSSender: sender,
		Now:                   func() time.Time { return now },
	})
	login := loginForTest(t, server, "admin")
	notice := createHTTPMessageCenterWorkerNotice(t, server.resources, map[string]string{
		"tenantCode": "platform",
		"category":   "retry",
		"payload":    `{"templateId":"SMS_RETRY","templateParams":{"ticket":"T-200"}}`,
	})
	delivery := createHTTPMessageCenterWorkerDelivery(t, server.resources, map[string]string{
		"tenantCode":          "platform",
		"notificationCode":    notice.Code,
		"channel":             notification.ChannelSMS,
		"deliveryStatus":      notification.DeliveryStatusFailed,
		"target":              "****0000",
		"provider":            notification.SMSProviderMockLocal,
		"attempts":            "1",
		"errorMessage":        notification.DeliveryLedgerErrorFailed,
		"nextRetryAt":         now.Add(time.Minute).Format(time.RFC3339),
		"retryBackoffSeconds": "60",
		"providerStatus":      notification.DeliveryStatusFailed,
		"providerMessageId":   "failed-message",
	})

	retryRecorder := httptest.NewRecorder()
	retryRequest := httptest.NewRequest(http.MethodPost, "/api/admin/message-center/deliveries/"+delivery.ID+"/retry", bytes.NewBufferString(`{"recipient":"+8613800000000"}`))
	retryRequest.Header.Set("Authorization", "Bearer "+login.Data.Token)
	retryRequest.Header.Set("Content-Type", "application/json")
	server.Router().ServeHTTP(retryRecorder, retryRequest)

	if retryRecorder.Code != http.StatusOK {
		t.Fatalf("POST message-center delivery retry status = %d body = %s, want 200", retryRecorder.Code, retryRecorder.Body.String())
	}
	var retryPayload Response[adminMessageCenterDeliveryRetryResponse]
	if err := json.Unmarshal(retryRecorder.Body.Bytes(), &retryPayload); err != nil {
		t.Fatalf("decode retry response: %v body = %s", err, retryRecorder.Body.String())
	}
	if retryPayload.Data.Delivery.Values["deliveryStatus"] != notification.DeliveryStatusPending ||
		retryPayload.Data.Delivery.Values["target"] != "****0000" ||
		retryPayload.Data.Delivery.Values["errorMessage"] != "" ||
		strings.Contains(retryRecorder.Body.String(), "+8613800000000") {
		t.Fatalf("retry response = %+v body=%s, want pending redacted delivery without raw recipient", retryPayload.Data.Delivery, retryRecorder.Body.String())
	}
	queued, err := server.resources.InternalRecord("notification-deliveries", delivery.ID)
	if err != nil {
		t.Fatalf("InternalRecord(notification-deliveries) after retry error = %v", err)
	}
	if queued.Values["deliveryStatus"] != notification.DeliveryStatusPending ||
		queued.Values["target"] != "****0000" ||
		queued.Values["attempts"] != "1" ||
		queued.Values["retryRequestedAt"] != now.Format(time.RFC3339) ||
		queued.Values["nextRetryAt"] != "" ||
		queued.Values["providerMessageId"] != "" ||
		queued.Values["providerStatus"] != "" ||
		queued.Values["retryBackoffSeconds"] != "" {
		t.Fatalf("queued delivery values = %+v, want retry-ready redacted internal delivery", queued.Values)
	}
	if strings.Contains(fmt.Sprintf("%+v", queued), "+8613800000000") {
		t.Fatalf("queued delivery leaked raw retry recipient: %+v", queued)
	}

	runRecorder := httptest.NewRecorder()
	runRequest := httptest.NewRequest(http.MethodPost, "/api/admin/message-center/deliveries/run", nil)
	runRequest.Header.Set("Authorization", "Bearer "+login.Data.Token)
	server.Router().ServeHTTP(runRecorder, runRequest)

	if runRecorder.Code != http.StatusOK {
		t.Fatalf("POST message-center deliveries run after retry status = %d body = %s, want 200", runRecorder.Code, runRecorder.Body.String())
	}
	var runPayload adminMessageCenterDeliveryRunTestPayload
	if err := json.Unmarshal(runRecorder.Body.Bytes(), &runPayload); err != nil {
		t.Fatalf("decode run response: %v body = %s", err, runRecorder.Body.String())
	}
	if runPayload.Data.Attempted != 1 || runPayload.Data.Delivered != 1 || runPayload.Data.Failed != 0 {
		t.Fatalf("delivery run after retry = %+v, want one delivered retry", runPayload.Data)
	}
	if sent := sender.Sent(); len(sent) != 1 || sent[0].Recipient != "+8613800000000" || sent[0].TemplateID != "SMS_RETRY" {
		t.Fatalf("sent after retry = %+v, want one rehydrated SMS", sent)
	}
	delivered, err := server.resources.InternalRecord("notification-deliveries", delivery.ID)
	if err != nil {
		t.Fatalf("InternalRecord(notification-deliveries) after run error = %v", err)
	}
	if delivered.Values["deliveryStatus"] != notification.DeliveryStatusDelivered ||
		delivered.Values["target"] != "****0000" ||
		strings.Contains(fmt.Sprintf("%+v", delivered), "+8613800000000") {
		t.Fatalf("delivered retry values = %+v, want delivered redacted ledger without raw recipient", delivered.Values)
	}
}

func TestAdminMessageCenterDeliveryRetryRequiresRecipientForRedactedTarget(t *testing.T) {
	now := time.Date(2026, 7, 19, 11, 50, 0, 0, time.UTC)
	server := newTestServer(ServerOptions{
		Capabilities: capabilitiesFromConfigForTest(t, []string{"dictionary", "tenant", "identity", "session", "rbac", "audit", "notification"}),
		Now:          func() time.Time { return now },
	})
	login := loginForTest(t, server, "admin")
	notice := createHTTPMessageCenterWorkerNotice(t, server.resources, map[string]string{
		"tenantCode": "platform",
		"category":   "retry-required",
		"payload":    `{"templateId":"SMS_RETRY"}`,
	})
	delivery := createHTTPMessageCenterWorkerDelivery(t, server.resources, map[string]string{
		"tenantCode":       "platform",
		"notificationCode": notice.Code,
		"channel":          notification.ChannelSMS,
		"deliveryStatus":   notification.DeliveryStatusFailed,
		"target":           "****0000",
		"provider":         notification.SMSProviderMockLocal,
		"attempts":         "1",
		"errorMessage":     notification.DeliveryLedgerErrorFailed,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/admin/message-center/deliveries/"+delivery.ID+"/retry", bytes.NewBufferString(`{}`))
	request.Header.Set("Authorization", "Bearer "+login.Data.Token)
	request.Header.Set("Content-Type", "application/json")
	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("POST message-center delivery retry without recipient status = %d body = %s, want 400", recorder.Code, recorder.Body.String())
	}
	unchanged, err := server.resources.InternalRecord("notification-deliveries", delivery.ID)
	if err != nil {
		t.Fatalf("InternalRecord(notification-deliveries) error = %v", err)
	}
	if unchanged.Values["deliveryStatus"] != notification.DeliveryStatusFailed || unchanged.Values["target"] != "****0000" || unchanged.Values["errorMessage"] == "" {
		t.Fatalf("delivery values = %+v, want failed record unchanged", unchanged.Values)
	}
}

func TestMessageCenterRetryTargetSurvivesTransientResolutionUntilDeliveryCompletes(t *testing.T) {
	server := newTestServer(ServerOptions{Now: func() time.Time { return time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC) }})
	delivery := adminresource.Record{ID: "delivery-retry-target"}
	server.storeMessageCenterRetryTarget(delivery.ID, "+8613800000000")

	first, ok, err := server.ResolveDeliveryTarget(context.Background(), delivery)
	if err != nil || !ok || first != "+8613800000000" {
		t.Fatalf("first ResolveDeliveryTarget() = %q, %t, %v; want unredacted target", first, ok, err)
	}
	second, ok, err := server.ResolveDeliveryTarget(context.Background(), delivery)
	if err != nil || !ok || second != "+8613800000000" {
		t.Fatalf("second ResolveDeliveryTarget() = %q, %t, %v; want target retained for a transient retry", second, ok, err)
	}
	server.DeliveryTargetDelivered(context.Background(), delivery)
	if target, ok, err := server.ResolveDeliveryTarget(context.Background(), delivery); err != nil || ok || target != "" {
		t.Fatalf("ResolveDeliveryTarget(after delivery) = %q, %t, %v; want released target", target, ok, err)
	}
}

type nthMessageCenterRateLimiter struct {
	denyAt     int
	retryAfter time.Duration
	calls      int
	keys       []string
}

func (limiter *nthMessageCenterRateLimiter) Allow(_ context.Context, key string, _ int, _ time.Duration) (ratelimit.Decision, error) {
	if strings.Contains(key, ":"+string(ratelimit.OperationMessageCenterDelivery)+":") {
		limiter.calls++
		limiter.keys = append(limiter.keys, key)
		if limiter.calls == limiter.denyAt {
			return ratelimit.Decision{RetryAfter: limiter.retryAfter}, nil
		}
	}
	return ratelimit.Decision{Allowed: true}, nil
}
