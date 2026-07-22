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

	"github.com/GnosiST/platform-go/internal/platform/credentialauth"
	"github.com/GnosiST/platform-go/internal/platform/kernel"
	"github.com/GnosiST/platform-go/internal/platform/notification"
	"github.com/GnosiST/platform-go/internal/platform/ratelimit"
)

func TestCredentialAuthSMSOTPStartUsesPhoneHashRateLimitAndDeliveryLedger(t *testing.T) {
	runtime, sender := credentialAuthRuntimeForTest(t)
	runtime.RequireLoginChallenge = true
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
	challengeID := createCredentialAuthChallengeForSMSOTPTest(t, runtime, "sms-start-client-1")
	recorder := httptest.NewRecorder()
	request := newCredentialAuthHTTPTestRequest(http.MethodPost, "/api/auth/sms-otp/start?phone=%2B8613800138000&otp=123456", bytes.NewBufferString(`{"provider":"phone-sms-otp","identifier":{"type":"phone","value":" +86 138-0013-8000 "},"challenge":{"id":"`+challengeID+`","kind":"captcha","proof":"`+credentialAuthChallengeProofForTest+`","clientFingerprint":"sms-start-client-1"}}`))
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

func TestCredentialAuthSMSOTPStartAppliesCoarseRateLimitBeforeIdentifierResolution(t *testing.T) {
	runtime, sender := credentialAuthRuntimeForTest(t)
	limiter := &rateLimitTestStub{deny: ratelimit.OperationAdminLogin, retryAfter: 90}
	server := newTestServer(ServerOptions{
		Capabilities:   configuredCredentialAuthManifestsForTest(t),
		CredentialAuth: runtime,
		RateLimiter:    limiter,
	})
	recorder := httptest.NewRecorder()
	request := newCredentialAuthHTTPTestRequest(
		http.MethodPost,
		"/api/auth/sms-otp/start",
		bytes.NewBufferString(`{"provider":"phone-sms-otp","identifier":{"type":"phone","value":"+8613999999999"}}`),
	)
	request.Header.Set("Content-Type", "application/json")

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusTooManyRequests || !strings.Contains(recorder.Body.String(), "RATE_LIMITED") {
		t.Fatalf("POST sms otp start with coarse limit status = %d body = %s, want rate limited", recorder.Code, recorder.Body.String())
	}
	if limiter.calls[ratelimit.OperationAdminLogin] != 1 || limiter.calls[ratelimit.OperationPhoneVerificationRequest] != 0 {
		t.Fatalf("rate limiter calls = %+v, want coarse admin-login only before identifier resolution", limiter.calls)
	}
	if sent := sender.Sent(); len(sent) != 0 {
		t.Fatalf("sent sms = %+v, want no send when coarse rate limit denies request", sent)
	}
}

func TestCredentialAuthSMSOTPStartRequiresChallengeBeforeSending(t *testing.T) {
	runtime, sender := credentialAuthRuntimeForTest(t)
	runtime.RequireLoginChallenge = true
	server := newTestServer(ServerOptions{
		Capabilities:   configuredCredentialAuthManifestsForTest(t),
		CredentialAuth: runtime,
	})
	recorder := httptest.NewRecorder()
	request := newCredentialAuthHTTPTestRequest(
		http.MethodPost,
		"/api/auth/sms-otp/start",
		bytes.NewBufferString(`{"provider":"phone-sms-otp","identifier":{"type":"phone","value":"+8613800138000"}}`),
	)
	request.Header.Set("Content-Type", "application/json")

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "AUTH_INVALID_REQUEST") {
		t.Fatalf("POST sms otp start without challenge status = %d body = %s, want invalid request", recorder.Code, recorder.Body.String())
	}
	if sent := sender.Sent(); len(sent) != 0 {
		t.Fatalf("sent sms = %+v, want no send when challenge is missing", sent)
	}
}

func TestCredentialAuthSMSOTPStartRejectsInvalidChallengeBeforeSending(t *testing.T) {
	runtime, sender := credentialAuthRuntimeForTest(t)
	runtime.RequireLoginChallenge = true
	server := newTestServer(ServerOptions{
		Capabilities:   configuredCredentialAuthManifestsForTest(t),
		CredentialAuth: runtime,
	})
	challengeID := createCredentialAuthChallengeForSMSOTPTest(t, runtime, "sms-start-client-bad")
	recorder := httptest.NewRecorder()
	request := newCredentialAuthHTTPTestRequest(
		http.MethodPost,
		"/api/auth/sms-otp/start",
		bytes.NewBufferString(`{"provider":"phone-sms-otp","identifier":{"type":"phone","value":"+8613800138000"},"challenge":{"id":"`+challengeID+`","kind":"captcha","proof":"wrong-proof","clientFingerprint":"sms-start-client-bad"}}`),
	)
	request.Header.Set("Content-Type", "application/json")

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized || !strings.Contains(recorder.Body.String(), "AUTH_INVALID_CREDENTIALS") {
		t.Fatalf("POST sms otp start with invalid challenge status = %d body = %s, want invalid credentials", recorder.Code, recorder.Body.String())
	}
	if sent := sender.Sent(); len(sent) != 0 {
		t.Fatalf("sent sms = %+v, want no send when challenge proof is invalid", sent)
	}
	deliveries, err := server.resources.InternalRecordsContext(context.Background(), messageCenterDeliveries)
	if err != nil {
		t.Fatalf("InternalRecordsContext(%s) error = %v", messageCenterDeliveries, err)
	}
	if len(deliveries) != 0 {
		t.Fatalf("notification deliveries = %+v, want none when challenge is invalid", deliveries)
	}
}

func TestCredentialAuthSMSOTPStartConsumesChallengeOnce(t *testing.T) {
	runtime, sender := credentialAuthRuntimeForTest(t)
	runtime.RequireLoginChallenge = true
	server := newTestServer(ServerOptions{
		Capabilities:   configuredCredentialAuthManifestsForTest(t),
		CredentialAuth: runtime,
	})
	challengeID := createCredentialAuthChallengeForSMSOTPTest(t, runtime, "sms-start-client-reuse")
	body := `{"provider":"phone-sms-otp","identifier":{"type":"phone","value":"+8613800138000"},"challenge":{"id":"` + challengeID + `","kind":"captcha","proof":"` + credentialAuthChallengeProofForTest + `","clientFingerprint":"sms-start-client-reuse"}}`

	firstRecorder := httptest.NewRecorder()
	firstRequest := newCredentialAuthHTTPTestRequest(http.MethodPost, "/api/auth/sms-otp/start", bytes.NewBufferString(body))
	firstRequest.Header.Set("Content-Type", "application/json")
	server.Router().ServeHTTP(firstRecorder, firstRequest)

	if firstRecorder.Code != http.StatusCreated {
		t.Fatalf("first POST sms otp start status = %d body = %s, want 201", firstRecorder.Code, firstRecorder.Body.String())
	}

	secondRecorder := httptest.NewRecorder()
	secondRequest := newCredentialAuthHTTPTestRequest(http.MethodPost, "/api/auth/sms-otp/start", bytes.NewBufferString(body))
	secondRequest.Header.Set("Content-Type", "application/json")
	server.Router().ServeHTTP(secondRecorder, secondRequest)

	if secondRecorder.Code != http.StatusUnauthorized || !strings.Contains(secondRecorder.Body.String(), "AUTH_INVALID_CREDENTIALS") {
		t.Fatalf("reused challenge sms otp start status = %d body = %s, want invalid credentials", secondRecorder.Code, secondRecorder.Body.String())
	}
	if sent := sender.Sent(); len(sent) != 1 {
		t.Fatalf("sent sms = %+v, want exactly one send after challenge reuse attempt", sent)
	}
	deliveries, err := server.resources.InternalRecordsContext(context.Background(), messageCenterDeliveries)
	if err != nil {
		t.Fatalf("InternalRecordsContext(%s) error = %v", messageCenterDeliveries, err)
	}
	if len(deliveries) != 1 || deliveries[0].Values["deliveryStatus"] != notification.DeliveryStatusDelivered {
		t.Fatalf("notification deliveries = %+v, want one delivered ledger after challenge reuse attempt", deliveries)
	}
}

func TestCredentialAuthSMSOTPStartDoesNotSendWhenChallengePersistenceFails(t *testing.T) {
	repository := &smsOTPUpsertFailRepository{Repository: credentialauth.NewMemoryRepository()}
	runtime, sender := credentialAuthSMSOTPPersistenceRuntimeForTest(t, repository, notification.NewMockLocalSMSSender())
	runtime.RequireLoginChallenge = true
	server := newTestServer(ServerOptions{
		Capabilities:   configuredCredentialAuthManifestsForTest(t),
		CredentialAuth: runtime,
	})
	challengeID := createCredentialAuthChallengeForSMSOTPTest(t, runtime, "sms-start-persist-fails")
	recorder := httptest.NewRecorder()
	request := newCredentialAuthHTTPTestRequest(
		http.MethodPost,
		"/api/auth/sms-otp/start",
		bytes.NewBufferString(`{"provider":"phone-sms-otp","identifier":{"type":"phone","value":"+8613800138000"},"challenge":{"id":"`+challengeID+`","kind":"captcha","proof":"`+credentialAuthChallengeProofForTest+`","clientFingerprint":"sms-start-persist-fails"}}`),
	)
	request.Header.Set("Content-Type", "application/json")

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadGateway || !strings.Contains(recorder.Body.String(), "AUTH_PROVIDER_RESOLVE_FAILED") {
		t.Fatalf("POST sms otp start with persistence failure status = %d body = %s, want provider resolve failure", recorder.Code, recorder.Body.String())
	}
	if repository.attempts != 1 {
		t.Fatalf("sms otp persistence attempts = %d, want exactly one failed write", repository.attempts)
	}
	if sent := sender.Sent(); len(sent) != 0 {
		t.Fatalf("sent sms = %+v, want no send before OTP challenge is persisted", sent)
	}
	deliveries, err := server.resources.InternalRecordsContext(context.Background(), messageCenterDeliveries)
	if err != nil {
		t.Fatalf("InternalRecordsContext(%s) error = %v", messageCenterDeliveries, err)
	}
	if len(deliveries) != 0 {
		t.Fatalf("notification deliveries = %+v, want no delivery ledger before OTP challenge is persisted", deliveries)
	}
}

func TestCredentialAuthSMSOTPStartDisablesChallengeWhenSendFails(t *testing.T) {
	repository := &smsOTPRecordingRepository{Repository: credentialauth.NewMemoryRepository()}
	runtime, _ := credentialAuthSMSOTPPersistenceRuntimeForTest(t, repository, failingCredentialAuthSMSSender{})
	runtime.RequireLoginChallenge = true
	server := newTestServer(ServerOptions{
		Capabilities:   configuredCredentialAuthManifestsForTest(t),
		CredentialAuth: runtime,
	})
	challengeID := createCredentialAuthChallengeForSMSOTPTest(t, runtime, "sms-start-send-fails")
	recorder := httptest.NewRecorder()
	request := newCredentialAuthHTTPTestRequest(
		http.MethodPost,
		"/api/auth/sms-otp/start",
		bytes.NewBufferString(`{"provider":"phone-sms-otp","identifier":{"type":"phone","value":"+8613800138000"},"challenge":{"id":"`+challengeID+`","kind":"captcha","proof":"`+credentialAuthChallengeProofForTest+`","clientFingerprint":"sms-start-send-fails"}}`),
	)
	request.Header.Set("Content-Type", "application/json")

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadGateway || !strings.Contains(recorder.Body.String(), "AUTH_PROVIDER_RESOLVE_FAILED") {
		t.Fatalf("POST sms otp start with send failure status = %d body = %s, want provider resolve failure", recorder.Code, recorder.Body.String())
	}
	if len(repository.challenges) != 2 {
		t.Fatalf("sms otp challenge writes = %+v, want initial enabled challenge and disabled cleanup", repository.challenges)
	}
	if first := repository.challenges[0]; first.Status != credentialauth.StatusEnabled || first.MessageID != "" {
		t.Fatalf("first sms otp challenge write = %+v, want enabled without provider message id before send", first)
	}
	if last := repository.challenges[len(repository.challenges)-1]; last.Status != credentialauth.StatusDisabled || last.MessageID != "" {
		t.Fatalf("last sms otp challenge write = %+v, want disabled challenge after send failure", last)
	}
	deliveries, err := server.resources.InternalRecordsContext(context.Background(), messageCenterDeliveries)
	if err != nil {
		t.Fatalf("InternalRecordsContext(%s) error = %v", messageCenterDeliveries, err)
	}
	if len(deliveries) != 1 || deliveries[0].Values["deliveryStatus"] != notification.DeliveryStatusFailed || deliveries[0].Values["target"] != "****8000" {
		t.Fatalf("notification deliveries = %+v, want one redacted failed ledger after send failure", deliveries)
	}
	assertCredentialAuthSMSOTPRecordDoesNotLeak(t, deliveries[0], "123456", "+8613800138000", "13800138000")
}

func createCredentialAuthChallengeForSMSOTPTest(t *testing.T, runtime *CredentialAuthRuntime, clientFingerprint string) string {
	t.Helper()
	if runtime == nil || runtime.Service == nil {
		t.Fatal("credential auth runtime missing service")
	}
	created, err := runtime.Service.CreateCredentialChallenge(context.Background(), credentialauth.CreateCredentialChallengeInput{
		Kind:                  credentialauth.ChallengeKindCaptcha,
		Purpose:               credentialauth.ChallengePurposeLogin,
		ClientFingerprintHash: credentialAuthClientFingerprintHash(clientFingerprint),
		TTL:                   runtime.ChallengeTTL,
	})
	if err != nil {
		t.Fatalf("CreateCredentialChallenge() error = %v", err)
	}
	if created.ChallengeID == "" {
		t.Fatalf("credential challenge id is empty: %+v", created)
	}
	return created.ChallengeID
}

func credentialAuthSMSOTPPersistenceRuntimeForTest(t *testing.T, repository credentialauth.Repository, sender notification.SMSSender) (*CredentialAuthRuntime, *notification.MockLocalSMSSender) {
	t.Helper()
	hasher, err := credentialauth.NewHMACIdentifierHasher([]byte(strings.Repeat("k", 32)))
	if err != nil {
		t.Fatalf("NewHMACIdentifierHasher() error = %v", err)
	}
	service, err := credentialauth.NewService(credentialauth.Options{
		Repository:           repository,
		IdentifierHasher:     hasher,
		ChallengeProofHasher: hasher,
		ChallengeMaterialGenerator: func(kind credentialauth.ChallengeKind) (credentialauth.ChallengeMaterial, error) {
			return credentialauth.ChallengeMaterial{
				Prompt:     "Complete the test challenge.",
				Parameters: map[string]string{"tileX": "5", "tileY": "12", "unit": "px"},
				Proof:      credentialAuthChallengeProofForTest,
			}, nil
		},
		Now: func() time.Time { return time.Date(2026, 7, 19, 9, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if _, err := service.RegisterIdentifier(context.Background(), credentialauth.RegisterIdentifierInput{
		Principal:  credentialauth.PrincipalRef{Type: credentialauth.PrincipalTypeAdmin, ID: "admin"},
		Identifier: credentialauth.Identifier{Type: credentialauth.IdentifierTypePhone, Value: "+8613800138000"},
		Status:     credentialauth.StatusEnabled,
	}); err != nil {
		t.Fatalf("RegisterIdentifier(phone) error = %v", err)
	}
	mockSender, _ := sender.(*notification.MockLocalSMSSender)
	return &CredentialAuthRuntime{
		Service:              service,
		IdentifierHasher:     hasher,
		SMSOTPHasher:         hasher,
		ChallengeProofHasher: hasher,
		SMSSender:            sender,
		LoginTemplateID:      "login-template",
		DebugCodeEnabled:     true,
		CodeGenerator:        func() (string, error) { return "123456", nil },
		Now:                  func() time.Time { return time.Date(2026, 7, 19, 9, 0, 0, 0, time.UTC) },
		SMSOTPTTL:            5 * time.Minute,
		MaxSMSOTPAttempts:    credentialauth.DefaultMaxSMSOTPAttempts,
		MaxChallengeAttempts: credentialauth.DefaultMaxChallengeAttempts,
		ChallengeTTL:         credentialauth.DefaultCredentialChallengeTTL,
	}, mockSender
}

type smsOTPUpsertFailRepository struct {
	credentialauth.Repository
	attempts int
}

func (r *smsOTPUpsertFailRepository) UpsertSMSOTPChallenge(context.Context, credentialauth.SMSOTPChallenge) error {
	r.attempts++
	return fmt.Errorf("injected sms otp persistence failure")
}

type smsOTPRecordingRepository struct {
	credentialauth.Repository
	challenges []credentialauth.SMSOTPChallenge
}

func (r *smsOTPRecordingRepository) UpsertSMSOTPChallenge(ctx context.Context, challenge credentialauth.SMSOTPChallenge) error {
	r.challenges = append(r.challenges, challenge)
	return r.Repository.UpsertSMSOTPChallenge(ctx, challenge)
}

type failingCredentialAuthSMSSender struct{}

func (failingCredentialAuthSMSSender) Kind() string {
	return notification.SMSProviderMockLocal
}

func (failingCredentialAuthSMSSender) SendSMS(_ context.Context, message notification.SMSMessage) (notification.SMSDeliveryReceipt, error) {
	return notification.SMSDeliveryReceipt{
		Provider:       notification.SMSProviderMockLocal,
		Status:         "failed",
		RedactedTarget: notification.RedactSMSTarget(message.Recipient),
	}, fmt.Errorf("injected sms send failure")
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
