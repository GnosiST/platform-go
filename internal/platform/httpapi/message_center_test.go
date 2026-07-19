package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/GnosiST/platform-go/internal/platform/errorcode"
	"github.com/GnosiST/platform-go/internal/platform/notification"
)

type adminMessageCenterTestSendPayload struct {
	Data struct {
		Notification adminResourceRecordTest `json:"notification"`
		Delivery     adminResourceRecordTest `json:"delivery"`
		Receipt      struct {
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
		payload.Data.Delivery.Values["providerMessageId"] == "" {
		t.Fatalf("delivery = %+v, want delivered SMS ledger linked to notification", payload.Data.Delivery)
	}
	sent := sender.Sent()
	if len(sent) != 1 || sent[0].Recipient != "+8613800000000" || sent[0].TemplateID != "SMS_TEST" || sent[0].TemplateParams["code"] != "123456" {
		t.Fatalf("sent messages = %+v, want one SMS with template params", sent)
	}
}
