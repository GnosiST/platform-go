package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
