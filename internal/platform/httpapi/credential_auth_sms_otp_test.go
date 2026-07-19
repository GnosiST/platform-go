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

	"github.com/GnosiST/platform-go/internal/platform/credentialauth"
	"github.com/GnosiST/platform-go/internal/platform/kernel"
	"github.com/GnosiST/platform-go/internal/platform/notification"
	"github.com/GnosiST/platform-go/internal/platform/ratelimit"
)

func TestCredentialAuthSMSOTPStartUsesPhoneHashRateLimitAndDeliveryLedger(t *testing.T) {
	runtime, sender := credentialAuthRuntimeForTest(t)
	keyBuilder, err := ratelimit.NewKeyBuilder([]byte(strings.Repeat("r", 32)))
	if err != nil {
		t.Fatalf("NewKeyBuilder() error = %v", err)
	}
	limiter := &rateLimitTestStub{}
	server := newTestServer(ServerOptions{
		Capabilities:        configuredCredentialAuthManifestsForTest(t),
		CredentialAuth:      runtime,
		RateLimiter:         limiter,
		RateLimitKeyBuilder: keyBuilder,
	})
	normalized, err := credentialauth.NormalizeIdentifier(credentialauth.Identifier{Type: credentialauth.IdentifierTypePhone, Value: " +86 138-0013-8000 "})
	if err != nil {
		t.Fatalf("NormalizeIdentifier() error = %v", err)
	}
	phoneHash, err := runtime.IdentifierHasher.HashIdentifier(credentialauth.IdentifierTypePhone, normalized.Value)
	if err != nil {
		t.Fatalf("HashIdentifier(phone) error = %v", err)
	}
	wantRateLimitKey, err := keyBuilder.Build(ratelimit.OperationPhoneVerificationRequest, "127.0.0.1", "phone-sms-otp", phoneHash, credentialAuthLoginPurpose)
	if err != nil {
		t.Fatalf("Build(rate limit key) error = %v", err)
	}
	beforeDeliveries, err := server.resources.InternalRecordsContext(context.Background(), messageCenterDeliveries)
	if err != nil {
		t.Fatalf("InternalRecordsContext(%s) before error = %v", messageCenterDeliveries, err)
	}
	recorder := httptest.NewRecorder()
	request := newCredentialAuthHTTPTestRequest(http.MethodPost, "/api/auth/sms-otp/start?phone=%2B8613800138000&otp=123456", bytes.NewBufferString(`{"provider":"phone-sms-otp","identifier":{"type":"phone","value":" +86 138-0013-8000 "}}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer request-private-token")
	request.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("POST credential sms otp start status = %d body = %s, want 201", recorder.Code, recorder.Body.String())
	}
	var started credentialAuthSMSOTPStartTestPayload
	if err := json.Unmarshal(recorder.Body.Bytes(), &started); err != nil {
		t.Fatalf("decode sms otp start: %v body = %s", err, recorder.Body.String())
	}
	if started.Data.TransactionID == "" || started.Data.DebugCode != "123456" || started.Data.MaskedIdentifier != normalized.MaskedIdentifier {
		t.Fatalf("sms otp start = %+v, want transaction, debug code and masked identifier", started.Data)
	}
	if limiter.calls[ratelimit.OperationPhoneVerificationRequest] != 1 || limiter.keys[ratelimit.OperationPhoneVerificationRequest] != wantRateLimitKey {
		t.Fatalf("rate limit key = %q calls=%d, want normalized phone hash key %q", limiter.keys[ratelimit.OperationPhoneVerificationRequest], limiter.calls[ratelimit.OperationPhoneVerificationRequest], wantRateLimitKey)
	}
	if strings.Contains(limiter.keys[ratelimit.OperationPhoneVerificationRequest], "+8613800138000") || strings.Contains(limiter.keys[ratelimit.OperationPhoneVerificationRequest], "13800138000") {
		t.Fatalf("rate limit key leaked raw phone: %q", limiter.keys[ratelimit.OperationPhoneVerificationRequest])
	}
	sent := sender.Sent()
	if len(sent) != 1 ||
		sent[0].Recipient != normalized.Value ||
		sent[0].TemplateID != "login-template" ||
		sent[0].TemplateParams["code"] != "123456" ||
		sent[0].TraceID != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("sent sms = %+v, want normalized login OTP with trace id", sent)
	}
	afterDeliveries, err := server.resources.InternalRecordsContext(context.Background(), messageCenterDeliveries)
	if err != nil {
		t.Fatalf("InternalRecordsContext(%s) after error = %v", messageCenterDeliveries, err)
	}
	if len(afterDeliveries) != len(beforeDeliveries)+1 {
		t.Fatalf("notification deliveries = %d before=%d, want one new ledger", len(afterDeliveries), len(beforeDeliveries))
	}
	delivery := afterDeliveries[len(afterDeliveries)-1]
	correlation := kernel.Correlation{RequestID: delivery.Values["requestId"], TraceID: delivery.Values["traceId"]}
	if delivery.Status != "enabled" ||
		delivery.Values["deliveryStatus"] != notification.DeliveryStatusDelivered ||
		delivery.Values["target"] != "****8000" ||
		delivery.Values["provider"] != notification.SMSProviderMockLocal ||
		delivery.Values["providerStatus"] != notification.SMSDeliveryAccepted ||
		delivery.Values["requestId"] != recorder.Header().Get("X-Request-ID") ||
		delivery.Values["traceId"] != "4bf92f3577b34da6a3ce929d0e0e4736" ||
		!kernel.ValidCorrelation(correlation) {
		t.Fatalf("notification delivery ledger = %+v, want redacted correlated delivered SMS ledger", delivery)
	}
	assertCredentialAuthSMSOTPRecordDoesNotLeak(t, delivery,
		"request-private-token",
		"123456",
		"+8613800138000",
		"13800138000",
		" +86 138-0013-8000 ",
	)
}

func assertCredentialAuthSMSOTPRecordDoesNotLeak(t *testing.T, record any, markers ...string) {
	t.Helper()
	serialized := fmt.Sprintf("%+v", record)
	for _, marker := range markers {
		if strings.Contains(serialized, marker) {
			t.Fatalf("credential auth SMS OTP record leaked %q: %s", marker, serialized)
		}
	}
}
