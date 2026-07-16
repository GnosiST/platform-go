package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/GnosiST/platform-go/internal/platform/adminresource"
	"github.com/GnosiST/platform-go/internal/platform/capability"
	"github.com/GnosiST/platform-go/internal/platform/core"
	"github.com/GnosiST/platform-go/internal/platform/dataprotection"
	"github.com/GnosiST/platform-go/internal/platform/errorcode"
	"github.com/GnosiST/platform-go/internal/platform/ratelimit"
	"github.com/GnosiST/platform-go/internal/platform/sensitivereveal"
)

const (
	sensitiveRevealHTTPResource        = "sensitive-records"
	sensitiveRevealHTTPField           = "identityNumber"
	sensitiveRevealHTTPSecondaryField  = "secondarySecret"
	sensitiveRevealHTTPPolicy          = "admin-sensitive-any-v1"
	sensitiveRevealHTTPPurpose         = "support-case"
	sensitiveRevealHTTPOtherPurpose    = "incident-response"
	sensitiveRevealHTTPReadPermission  = "admin:sensitive-record:read"
	sensitiveRevealHTTPFieldPermission = "admin:sensitive-record:reveal"
)

func TestSensitiveRevealPolicyRequiresRealAdminBearerSession(t *testing.T) {
	harness := newSensitiveRevealHTTPHarness(t, sensitiveRevealHTTPHarnessOptions{})
	target := harness.fieldPath(harness.records["ops"], "/reveal-policy")

	recorder := harness.request(http.MethodGet, target, "", harness.token)
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET reveal policy with admin bearer status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("reveal policy Cache-Control = %q, want no-store", recorder.Header().Get("Cache-Control"))
	}

	apiToken, _ := createAdminAPITokenForTest(t, harness.server, sensitiveRevealHTTPReadPermission)
	for _, test := range []struct {
		name   string
		token  string
		header bool
		want   int
	}{
		{name: "missing bearer", want: http.StatusUnauthorized},
		{name: "admin api token", token: apiToken, want: http.StatusUnauthorized},
		{name: "insecure header auth", header: true, want: http.StatusUnauthorized},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, target, nil)
			if test.token != "" {
				request.Header.Set("Authorization", "Bearer "+test.token)
			}
			if test.header {
				request.Header.Set("X-Platform-User", "admin")
			}
			harness.server.Router().ServeHTTP(recorder, request)
			if recorder.Code != test.want {
				t.Fatalf("GET reveal policy status = %d body = %s, want %d", recorder.Code, recorder.Body.String(), test.want)
			}
		})
	}
}

func TestSensitiveRevealChecksReadRevealPermissionsAndDataScope(t *testing.T) {
	tests := []struct {
		name        string
		username    string
		permissions permissionSetAuthorizer
		recordKey   string
		want        int
	}{
		{
			name: "read without field reveal permission", username: "admin", recordKey: "ops", want: http.StatusForbidden,
			permissions: permissionSetAuthorizer{sensitiveRevealHTTPReadPermission: true},
		},
		{
			name: "field reveal without resource read permission", username: "admin", recordKey: "ops", want: http.StatusForbidden,
			permissions: permissionSetAuthorizer{sensitiveRevealHTTPFieldPermission: true},
		},
		{
			name: "in-scope operator", username: "ops", recordKey: "ops", want: http.StatusOK,
			permissions: permissionSetAuthorizer{sensitiveRevealHTTPReadPermission: true, sensitiveRevealHTTPFieldPermission: true},
		},
		{
			name: "out-of-scope operator", username: "ops", recordKey: "hq", want: http.StatusNotFound,
			permissions: permissionSetAuthorizer{sensitiveRevealHTTPReadPermission: true, sensitiveRevealHTTPFieldPermission: true},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			harness := newSensitiveRevealHTTPHarness(t, sensitiveRevealHTTPHarnessOptions{
				username: test.username, authorizer: test.permissions,
			})
			recorder := harness.request(http.MethodGet, harness.fieldPath(harness.records[test.recordKey], "/reveal-policy"), "", harness.token)
			if recorder.Code != test.want {
				t.Fatalf("GET reveal policy status = %d body = %s, want %d", recorder.Code, recorder.Body.String(), test.want)
			}
		})
	}
}

func TestSensitiveRevealChallengeURLMustMatchChallengeToken(t *testing.T) {
	harness := newSensitiveRevealHTTPHarness(t, sensitiveRevealHTTPHarnessOptions{})
	challenge := harness.beginChallenge(t, harness.records["ops"], sensitiveRevealHTTPPurpose, harness.token)
	body := `{"challengeToken":"` + challenge.ChallengeToken + `","purpose":"` + sensitiveRevealHTTPPurpose + `"}`
	recorder := harness.request(
		http.MethodPost,
		harness.fieldPath(harness.records["ops"], "/reveal/challenges/wrong-challenge/factors/sms/start"),
		body,
		harness.token,
	)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("POST SMS start with mismatched challenge URL status = %d body = %s, want 404", recorder.Code, recorder.Body.String())
	}
	if harness.sender.phone != "" {
		t.Fatalf("mismatched challenge sent an OTP to %q", harness.sender.phone)
	}
	recorder = harness.request(
		http.MethodPost,
		harness.fieldPath(harness.records["ops"], "/reveal/challenges/"+challenge.ChallengeID+"/factors/sms/start"),
		body,
		harness.token,
	)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("POST SMS start after mismatched URL status = %d body = %s, want 201", recorder.Code, recorder.Body.String())
	}
}

func TestSensitiveRevealAllOfChallengeUsesPublicFactorContract(t *testing.T) {
	harness := newSensitiveRevealHTTPHarness(t, sensitiveRevealHTTPHarnessOptions{policyMode: capability.AdminRevealModeAllOf})
	recordID := harness.records["ops"]
	policyRecorder := harness.request(http.MethodGet, harness.fieldPath(recordID, "/reveal-policy"), "", harness.token)
	if policyRecorder.Code != http.StatusOK {
		t.Fatalf("GET reveal policy status = %d body = %s", policyRecorder.Code, policyRecorder.Body.String())
	}
	policy := decodeSensitiveRevealData[adminSensitiveRevealPolicyResponse](t, policyRecorder)
	challenge := harness.beginChallenge(t, recordID, sensitiveRevealHTTPPurpose, harness.token)
	wantFactors := capability.AdminRevealFactorOIDCReauthentication + "," + capability.AdminRevealFactorSMSOTP
	policyFactors := make([]string, 0, len(policy.Factors))
	for _, factor := range policy.Factors {
		policyFactors = append(policyFactors, factor.Type)
	}
	if policy.Mode != capability.AdminRevealModeAllOf || challenge.Mode != capability.AdminRevealModeAllOf {
		t.Fatalf("policy/challenge modes = %q/%q, want allOf", policy.Mode, challenge.Mode)
	}
	if strings.Join(policyFactors, ",") != wantFactors || strings.Join(challenge.Factors, ",") != wantFactors {
		t.Fatalf("policy/challenge factors = %v/%v, want public factors %q", policyFactors, challenge.Factors, wantFactors)
	}
	for _, factor := range challenge.Factors {
		if factor == string(sensitivereveal.FactorOIDCReauthentication) || factor == string(sensitivereveal.FactorSMSOTP) {
			t.Fatalf("challenge exposed internal factor name %q", factor)
		}
	}
}

func TestSensitiveRevealVerificationFailurePreservesAdminSession(t *testing.T) {
	harness := newSensitiveRevealHTTPHarness(t, sensitiveRevealHTTPHarnessOptions{})
	recordID := harness.records["ops"]
	challenge := harness.beginChallenge(t, recordID, sensitiveRevealHTTPPurpose, harness.token)
	startBody := `{"challengeToken":"` + challenge.ChallengeToken + `","purpose":"` + sensitiveRevealHTTPPurpose + `"}`
	startRecorder := harness.request(
		http.MethodPost,
		harness.fieldPath(recordID, "/reveal/challenges/"+challenge.ChallengeID+"/factors/sms/start"),
		startBody,
		harness.token,
	)
	if startRecorder.Code != http.StatusCreated {
		t.Fatalf("POST SMS start status = %d body = %s", startRecorder.Code, startRecorder.Body.String())
	}
	start := decodeSensitiveRevealData[adminSensitiveRevealSMSStartResponse](t, startRecorder)
	completeBody := `{"challengeToken":"` + challenge.ChallengeToken + `","purpose":"` + sensitiveRevealHTTPPurpose + `","transactionToken":"` + start.TransactionToken + `","code":"000000"}`
	completeRecorder := harness.request(
		http.MethodPost,
		harness.fieldPath(recordID, "/reveal/challenges/"+challenge.ChallengeID+"/factors/sms/complete"),
		completeBody,
		harness.token,
	)
	if completeRecorder.Code != http.StatusUnprocessableEntity || !strings.Contains(completeRecorder.Body.String(), "ADMIN_SENSITIVE_REVEAL_VERIFICATION_FAILED") {
		t.Fatalf("POST SMS complete failure status = %d body = %s, want 422 reveal verification failure", completeRecorder.Code, completeRecorder.Body.String())
	}
	policyRecorder := harness.request(http.MethodGet, harness.fieldPath(recordID, "/reveal-policy"), "", harness.token)
	if policyRecorder.Code != http.StatusOK {
		t.Fatalf("GET reveal policy after failed verification status = %d body = %s, want session preserved", policyRecorder.Code, policyRecorder.Body.String())
	}
}

func TestSensitiveRevealSMSDeliveryFailureCancelsTransactionForRetry(t *testing.T) {
	sink := &recordingInternalErrorSink{}
	harness := newSensitiveRevealHTTPHarness(t, sensitiveRevealHTTPHarnessOptions{internalErrorSink: sink})
	recordID := harness.records["ops"]
	challenge := harness.beginChallenge(t, recordID, sensitiveRevealHTTPPurpose, harness.token)
	body := `{"challengeToken":"` + challenge.ChallengeToken + `","purpose":"` + sensitiveRevealHTTPPurpose + `"}`
	target := harness.fieldPath(recordID, "/reveal/challenges/"+challenge.ChallengeID+"/factors/sms/start")

	harness.sender.err = errors.New("delivery unavailable")
	failed := harness.request(http.MethodPost, target, body, harness.token)
	if failed.Code != http.StatusBadGateway || !strings.Contains(failed.Body.String(), "ADMIN_SENSITIVE_REVEAL_DELIVERY_FAILED") {
		t.Fatalf("POST SMS start delivery failure status = %d body = %s, want retryable 502", failed.Code, failed.Body.String())
	}
	if failed.Header().Get("Cache-Control") != "no-store" || len(sink.events) != 1 || sink.events[0].Code != "ADMIN_SENSITIVE_REVEAL_DELIVERY_FAILED" {
		t.Fatalf("delivery failure headers/events = %q/%+v, want no-store and one safe event", failed.Header().Get("Cache-Control"), sink.events)
	}

	harness.sender.err = nil
	retried := harness.request(http.MethodPost, target, body, harness.token)
	if retried.Code != http.StatusCreated {
		t.Fatalf("POST SMS start retry status = %d body = %s, want 201", retried.Code, retried.Body.String())
	}
	events, err := harness.runtime.AuditEvents(context.Background())
	if err != nil {
		t.Fatalf("AuditEvents() error = %v", err)
	}
	foundCancellation := false
	for _, event := range events {
		if event.Type == sensitivereveal.AuditFactorFailed && event.Outcome == "cancelled" && event.Reason == sensitivereveal.FactorCancelReasonDeliveryFailed {
			foundCancellation = true
			break
		}
	}
	if !foundCancellation {
		t.Fatalf("audit events = %+v, want delivery cancellation", events)
	}
}

func TestSensitiveRevealSMSUsesTrustedPhoneAndSingleUseGrant(t *testing.T) {
	harness := newSensitiveRevealHTTPHarness(t, sensitiveRevealHTTPHarnessOptions{})
	recordID := harness.records["ops"]
	challenge := harness.beginChallenge(t, recordID, sensitiveRevealHTTPPurpose, harness.token)
	clientPhoneBody := `{"challengeToken":"` + challenge.ChallengeToken + `","purpose":"` + sensitiveRevealHTTPPurpose + `","phone":"13999999999"}`
	startRecorder := harness.request(
		http.MethodPost,
		harness.fieldPath(recordID, "/reveal/challenges/"+challenge.ChallengeID+"/factors/sms/start"),
		clientPhoneBody,
		harness.token,
	)
	if startRecorder.Code != http.StatusBadRequest {
		t.Fatalf("POST SMS start with client phone status = %d body = %s, want 400", startRecorder.Code, startRecorder.Body.String())
	}
	if harness.sender.phone != "" {
		t.Fatalf("client phone request sent an OTP to %q", harness.sender.phone)
	}
	startBody := `{"challengeToken":"` + challenge.ChallengeToken + `","purpose":"` + sensitiveRevealHTTPPurpose + `"}`
	startRecorder = harness.request(
		http.MethodPost,
		harness.fieldPath(recordID, "/reveal/challenges/"+challenge.ChallengeID+"/factors/sms/start"),
		startBody,
		harness.token,
	)
	if startRecorder.Code != http.StatusCreated {
		t.Fatalf("POST SMS start status = %d body = %s", startRecorder.Code, startRecorder.Body.String())
	}
	start := decodeSensitiveRevealData[adminSensitiveRevealSMSStartResponse](t, startRecorder)
	if harness.phoneResolver.username != "admin" {
		t.Fatalf("trusted phone resolver username = %q, want admin", harness.phoneResolver.username)
	}
	if harness.sender.phone != "13800138000" || harness.sender.phone == "13999999999" {
		t.Fatalf("OTP destination = %q, want trusted server-side phone", harness.sender.phone)
	}
	if harness.sender.purpose != adminSensitiveRevealSMSPurpose || start.MaskedPhone != "138****8000" || start.DebugCode == "" {
		t.Fatalf("SMS start payload = %+v sender purpose = %q", start, harness.sender.purpose)
	}
	if strings.Contains(startRecorder.Body.String(), "13999999999") || strings.Contains(startRecorder.Body.String(), "13800138000") {
		t.Fatalf("SMS start response leaked client or trusted plaintext phone: %s", startRecorder.Body.String())
	}

	complete := harness.completeSMS(t, recordID, sensitiveRevealHTTPPurpose, challenge, start, start.DebugCode, harness.token)
	if complete.GrantToken == "" || !complete.PolicySatisfied {
		t.Fatalf("SMS complete payload = %+v, want a satisfied policy and grant", complete)
	}

	harness.protection.revealCalls = 0
	revealBody := `{"purpose":"` + sensitiveRevealHTTPPurpose + `","grantToken":"` + complete.GrantToken + `"}`
	revealRecorder := harness.request(http.MethodPost, harness.fieldPath(recordID, "/reveal"), revealBody, harness.token)
	if revealRecorder.Code != http.StatusOK {
		t.Fatalf("POST reveal status = %d body = %s", revealRecorder.Code, revealRecorder.Body.String())
	}
	revealed := decodeSensitiveRevealData[adminSensitiveRevealResponse](t, revealRecorder)
	if revealed.Field != sensitiveRevealHTTPField || revealed.Value != "170101199001011204" || !revealed.CopyAllowed {
		t.Fatalf("reveal payload = %+v", revealed)
	}
	if harness.protection.revealCalls != 1 || harness.protection.lastRevealField != sensitiveRevealHTTPField {
		t.Fatalf("protected field reveals = %d last field = %q, want only %q", harness.protection.revealCalls, harness.protection.lastRevealField, sensitiveRevealHTTPField)
	}
	if strings.Contains(revealRecorder.Body.String(), "secondary-private-value") || strings.Contains(revealRecorder.Body.String(), "tenantCode") {
		t.Fatalf("single-field response exposed another record field: %s", revealRecorder.Body.String())
	}
	assertSensitiveRevealDataKeys(t, revealRecorder, "copyAllowed", "field", "value")

	replay := harness.request(http.MethodPost, harness.fieldPath(recordID, "/reveal"), revealBody, harness.token)
	if replay.Code != http.StatusGone {
		t.Fatalf("POST replayed reveal status = %d body = %s, want 410", replay.Code, replay.Body.String())
	}
	if strings.Contains(replay.Body.String(), revealed.Value) {
		t.Fatalf("replayed reveal leaked plaintext: %s", replay.Body.String())
	}
}

func TestSensitiveRevealResponseWriteFailureRecordsAbortedAudit(t *testing.T) {
	store := &sensitiveRevealContextCheckingStore{Store: sensitivereveal.NewMemoryStore()}
	harness := newSensitiveRevealHTTPHarness(t, sensitiveRevealHTTPHarnessOptions{store: store})
	recordID := harness.records["ops"]
	_, _, grant := harness.issueSMSGrant(t, recordID, sensitiveRevealHTTPPurpose, harness.token)
	body := `{"purpose":"` + sensitiveRevealHTTPPurpose + `","grantToken":"` + grant.GrantToken + `"}`
	request := httptest.NewRequest(http.MethodPost, harness.fieldPath(recordID, "/reveal"), bytes.NewBufferString(body))
	requestCtx, cancelRequest := context.WithCancel(request.Context())
	request = request.WithContext(requestCtx)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+harness.token)
	writer := &sensitiveRevealFailingResponseWriter{header: make(http.Header), err: errors.New("client disconnected"), cancel: cancelRequest}
	harness.server.Router().ServeHTTP(writer, request)
	if store.sawCanceled || store.terminalCalls != 1 {
		t.Fatalf("terminal audit context canceled = %t calls = %d, want false and 1", store.sawCanceled, store.terminalCalls)
	}

	events, err := harness.runtime.AuditEvents(context.Background())
	if err != nil {
		t.Fatalf("AuditEvents() error = %v", err)
	}
	last := events[len(events)-1]
	if last.Type != sensitivereveal.AuditRevealFailed || last.Outcome != "failed" || last.Reason != sensitivereveal.RevealReasonResponseAborted {
		t.Fatalf("terminal reveal audit = %+v, want response_aborted", last)
	}
	replay := harness.request(http.MethodPost, harness.fieldPath(recordID, "/reveal"), body, harness.token)
	if replay.Code != http.StatusGone {
		t.Fatalf("POST reveal replay status = %d body = %s, want 410", replay.Code, replay.Body.String())
	}
}

func TestSensitiveRevealGrantRejectsPurposeRecordAndSessionScopeMismatch(t *testing.T) {
	harness := newSensitiveRevealHTTPHarness(t, sensitiveRevealHTTPHarnessOptions{})
	recordID := harness.records["ops"]
	challenge, start, grant := harness.issueSMSGrant(t, recordID, sensitiveRevealHTTPPurpose, harness.token)
	if challenge.ChallengeID == "" || start.TransactionToken == "" || grant.GrantToken == "" {
		t.Fatalf("grant setup incomplete: challenge=%+v start=%+v grant=%+v", challenge, start, grant)
	}

	tests := []struct {
		name     string
		recordID string
		purpose  string
		token    string
	}{
		{name: "purpose", recordID: recordID, purpose: sensitiveRevealHTTPOtherPurpose, token: harness.token},
		{name: "record", recordID: harness.records["hq"], purpose: sensitiveRevealHTTPPurpose, token: harness.token},
		{name: "session", recordID: recordID, purpose: sensitiveRevealHTTPPurpose, token: loginForTest(t, harness.server, "admin").Data.Token},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body := `{"purpose":"` + test.purpose + `","grantToken":"` + grant.GrantToken + `"}`
			recorder := harness.request(http.MethodPost, harness.fieldPath(test.recordID, "/reveal"), body, test.token)
			if recorder.Code != http.StatusForbidden {
				t.Fatalf("POST reveal with %s mismatch status = %d body = %s, want 403", test.name, recorder.Code, recorder.Body.String())
			}
		})
	}

	body := `{"purpose":"` + sensitiveRevealHTTPPurpose + `","grantToken":"` + grant.GrantToken + `"}`
	recorder := harness.request(http.MethodPost, harness.fieldPath(recordID, "/reveal"), body, harness.token)
	if recorder.Code != http.StatusOK {
		t.Fatalf("POST reveal after non-consuming scope mismatches status = %d body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestSensitiveRevealOIDCBindingMustMatchCurrentActor(t *testing.T) {
	const marker = "physical_table=identity_bindings email=private@example.test"
	for _, test := range []struct {
		name            string
		bindingUsername string
		bindingErr      error
		want            int
		wantCode        errorcode.Code
		wantEvents      int
		wantGrant       bool
	}{
		{name: "matching actor", bindingUsername: "admin", want: http.StatusOK, wantGrant: true},
		{name: "different actor", bindingUsername: "ops", want: http.StatusUnprocessableEntity, wantCode: errorcode.CodeAdminSensitiveRevealVerificationFailed},
		{name: "binding repository failure", bindingErr: errors.New(marker), want: http.StatusInternalServerError, wantCode: errorcode.CodeAdminSensitiveRevealFailed, wantEvents: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			resolver := &sensitiveRevealIdentityResolverStub{}
			bindings := &sensitiveRevealIdentityBindingStub{username: test.bindingUsername, err: test.bindingErr}
			sink := &recordingInternalErrorSink{}
			harness := newSensitiveRevealHTTPHarness(t, sensitiveRevealHTTPHarnessOptions{
				identityResolver:  resolver,
				identityBindings:  bindings,
				internalErrorSink: sink,
			})
			recordID := harness.records["ops"]
			challenge := harness.beginChallenge(t, recordID, sensitiveRevealHTTPPurpose, harness.token)
			startBody := `{"challengeToken":"` + challenge.ChallengeToken + `","purpose":"` + sensitiveRevealHTTPPurpose + `","provider":"oidc","codeChallenge":"challenge"}`
			startRecorder := harness.request(
				http.MethodPost,
				harness.fieldPath(recordID, "/reveal/challenges/"+challenge.ChallengeID+"/factors/oidc/start"),
				startBody,
				harness.token,
			)
			if startRecorder.Code != http.StatusCreated {
				t.Fatalf("POST OIDC start status = %d body = %s", startRecorder.Code, startRecorder.Body.String())
			}
			started := decodeSensitiveRevealData[adminSensitiveRevealOIDCStartResponse](t, startRecorder)
			completeBody := `{"challengeToken":"` + challenge.ChallengeToken + `","purpose":"` + sensitiveRevealHTTPPurpose + `","transactionToken":"` + started.TransactionToken + `","provider":"oidc","code":"code","state":"state","codeVerifier":"verifier"}`
			completeRecorder := harness.request(
				http.MethodPost,
				harness.fieldPath(recordID, "/reveal/challenges/"+challenge.ChallengeID+"/factors/oidc/complete"),
				completeBody,
				harness.token,
			)
			if completeRecorder.Code != test.want {
				t.Fatalf("POST OIDC complete status = %d body = %s, want %d", completeRecorder.Code, completeRecorder.Body.String(), test.want)
			}
			if completeRecorder.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("POST OIDC complete Cache-Control = %q, want no-store", completeRecorder.Header().Get("Cache-Control"))
			}
			if resolver.stepUpStarts != 1 || resolver.stepUpResolves != 1 || bindings.issuer != "https://id.example" || bindings.subject != "subject-123" {
				t.Fatalf("OIDC boundary calls = start:%d resolve:%d binding:%q/%q", resolver.stepUpStarts, resolver.stepUpResolves, bindings.issuer, bindings.subject)
			}
			if len(sink.events) != test.wantEvents {
				t.Fatalf("OIDC binding events = %+v, want %d", sink.events, test.wantEvents)
			}
			if test.wantGrant {
				completed := decodeSensitiveRevealData[adminSensitiveRevealFactorCompleteResponse](t, completeRecorder)
				if completed.GrantToken == "" || !completed.PolicySatisfied {
					t.Fatalf("matching OIDC binding did not issue grant: %+v", completed)
				}
			} else {
				var payload Response[any]
				if err := json.Unmarshal(completeRecorder.Body.Bytes(), &payload); err != nil || payload.Error == nil {
					t.Fatalf("decode OIDC error: %v body = %s", err, completeRecorder.Body.String())
				}
				definition, _ := errorcode.Lookup(test.wantCode)
				if payload.Error.Code != test.wantCode || payload.Error.Message != definition.PublicMessage || payload.Error.RequestID == "" || payload.Error.TraceID == "" {
					t.Fatalf("OIDC error = %+v, want registered %s with correlation", payload.Error, test.wantCode)
				}
				if strings.Contains(completeRecorder.Body.String(), "grantToken") || strings.Contains(completeRecorder.Body.String(), "physical_table") || strings.Contains(completeRecorder.Body.String(), "private@example.test") {
					t.Fatalf("OIDC binding error leaked grant or cause: %s", completeRecorder.Body.String())
				}
			}
		})
	}
}

func TestSensitiveRevealAppliesChallengeFactorAndConsumeRateLimits(t *testing.T) {
	t.Run("challenge", func(t *testing.T) {
		limiter := newSensitiveRevealRateLimiter(ratelimit.OperationSensitiveRevealChallenge)
		harness := newSensitiveRevealHTTPHarness(t, sensitiveRevealHTTPHarnessOptions{limiter: limiter})
		body := `{"purpose":"` + sensitiveRevealHTTPPurpose + `"}`
		recorder := harness.request(http.MethodPost, harness.fieldPath(harness.records["ops"], "/reveal/challenges"), body, harness.token)
		assertSensitiveRevealRateLimited(t, recorder, limiter, ratelimit.OperationSensitiveRevealChallenge)
	})

	t.Run("factor", func(t *testing.T) {
		limiter := newSensitiveRevealRateLimiter("")
		harness := newSensitiveRevealHTTPHarness(t, sensitiveRevealHTTPHarnessOptions{limiter: limiter})
		challenge := harness.beginChallenge(t, harness.records["ops"], sensitiveRevealHTTPPurpose, harness.token)
		limiter.deny = ratelimit.OperationSensitiveRevealFactor
		body := `{"challengeToken":"` + challenge.ChallengeToken + `","purpose":"` + sensitiveRevealHTTPPurpose + `"}`
		recorder := harness.request(
			http.MethodPost,
			harness.fieldPath(harness.records["ops"], "/reveal/challenges/"+challenge.ChallengeID+"/factors/sms/start"),
			body,
			harness.token,
		)
		assertSensitiveRevealRateLimited(t, recorder, limiter, ratelimit.OperationSensitiveRevealFactor)
	})

	t.Run("consume does not burn grant", func(t *testing.T) {
		limiter := newSensitiveRevealRateLimiter("")
		harness := newSensitiveRevealHTTPHarness(t, sensitiveRevealHTTPHarnessOptions{limiter: limiter})
		_, _, grant := harness.issueSMSGrant(t, harness.records["ops"], sensitiveRevealHTTPPurpose, harness.token)
		limiter.deny = ratelimit.OperationSensitiveRevealConsume
		body := `{"purpose":"` + sensitiveRevealHTTPPurpose + `","grantToken":"` + grant.GrantToken + `"}`
		recorder := harness.request(http.MethodPost, harness.fieldPath(harness.records["ops"], "/reveal"), body, harness.token)
		assertSensitiveRevealRateLimited(t, recorder, limiter, ratelimit.OperationSensitiveRevealConsume)
		limiter.deny = ""
		recorder = harness.request(http.MethodPost, harness.fieldPath(harness.records["ops"], "/reveal"), body, harness.token)
		if recorder.Code != http.StatusOK {
			t.Fatalf("POST reveal after rate limit status = %d body = %s, want unconsumed grant", recorder.Code, recorder.Body.String())
		}
	})
}

type sensitiveRevealHTTPHarnessOptions struct {
	username          string
	policyMode        string
	authorizer        Authorizer
	limiter           ratelimit.Limiter
	identityResolver  AdminIdentityResolver
	identityBindings  AdminIdentityBindingStore
	store             sensitivereveal.Store
	internalErrorSink InternalErrorSink
}

type sensitiveRevealHTTPHarness struct {
	server        *Server
	runtime       *sensitivereveal.Runtime
	token         string
	records       map[string]string
	phoneResolver *sensitiveRevealPhoneResolverStub
	sender        *sensitiveRevealPhoneSenderStub
	protection    *sensitiveRevealProtectionRuntime
}

func newSensitiveRevealHTTPHarness(t *testing.T, options sensitiveRevealHTTPHarnessOptions) *sensitiveRevealHTTPHarness {
	t.Helper()
	now := time.Date(2026, time.July, 14, 10, 0, 0, 0, time.UTC)
	capabilityMode := capability.AdminRevealModeAnyOf
	runtimeMode := sensitivereveal.PolicyAnyOf
	if options.policyMode == capability.AdminRevealModeAllOf {
		capabilityMode = capability.AdminRevealModeAllOf
		runtimeMode = sensitivereveal.PolicyAllOf
	}
	manifests := sensitiveRevealHTTPManifests(t, capabilityMode)
	protection := newSensitiveRevealProtectionRuntime(t)
	resources, err := adminresource.NewStoreFromCapabilitiesWithProtection(manifests, protection)
	if err != nil {
		t.Fatalf("NewStoreFromCapabilitiesWithProtection() error = %v", err)
	}
	records := map[string]string{}
	for _, record := range []struct {
		key     string
		code    string
		orgUnit string
		secret  string
	}{
		{key: "ops", code: "record-ops", orgUnit: "platform-ops", secret: "170101199001011204"},
		{key: "hq", code: "record-hq", orgUnit: "platform-hq", secret: "170101199001011212"},
	} {
		created, createErr := resources.CreateInternal(sensitiveRevealHTTPResource, adminresource.WriteInput{
			Code: record.code, Name: record.code, Status: "enabled",
			Values: map[string]string{
				"tenantCode":                      sensitiveRevealPlatformTenant,
				"orgUnitCode":                     record.orgUnit,
				sensitiveRevealHTTPField:          record.secret,
				sensitiveRevealHTTPSecondaryField: "secondary-private-value",
			},
		})
		if createErr != nil {
			t.Fatalf("CreateInternal(%s) error = %v", record.code, createErr)
		}
		records[record.key] = created.ID
	}
	revealStore := options.store
	if revealStore == nil {
		revealStore = sensitivereveal.NewMemoryStore()
	}
	revealRuntime, err := sensitivereveal.NewRuntime(sensitivereveal.RuntimeOptions{
		Store: revealStore,
		Policies: []sensitivereveal.Policy{{
			ID: sensitiveRevealHTTPPolicy, Mode: runtimeMode,
			Factors: []sensitivereveal.FactorRule{
				{Factor: sensitivereveal.FactorOIDCReauthentication, MaxAttempts: 3},
				{Factor: sensitivereveal.FactorSMSOTP, MaxAttempts: 3},
			},
			PurposeCodes: []string{sensitiveRevealHTTPPurpose, sensitiveRevealHTTPOtherPurpose},
			ChallengeTTL: 5 * time.Minute, GrantTTL: time.Minute,
		}},
		HashKey: []byte(strings.Repeat("r", 32)), Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("sensitivereveal.NewRuntime() error = %v", err)
	}
	phoneResolver := &sensitiveRevealPhoneResolverStub{phone: AdminStepUpPhone{Phone: "13800138000", MaskedPhone: "138****8000"}}
	sender := &sensitiveRevealPhoneSenderStub{}
	identityResolver := options.identityResolver
	if identityResolver == nil {
		identityResolver = &sensitiveRevealIdentityResolverStub{}
	}
	identityBindings := options.identityBindings
	if identityBindings == nil {
		identityBindings = &sensitiveRevealIdentityBindingStub{username: "admin"}
	}
	server := newTestServer(ServerOptions{
		Capabilities: manifests, Resources: resources, DataProtection: protection,
		SensitiveReveal: revealRuntime, AdminStepUpPhoneResolver: phoneResolver,
		PhoneVerificationSender: sender, DebugCodeEnabled: true,
		AdminIdentityResolver: identityResolver, AdminIdentityBindings: identityBindings,
		Authorizer: options.authorizer, RateLimiter: options.limiter,
		InternalErrorSink: options.internalErrorSink,
		Now:               func() time.Time { return now },
	})
	username := strings.TrimSpace(options.username)
	if username == "" {
		username = "admin"
	}
	return &sensitiveRevealHTTPHarness{
		server: server, runtime: revealRuntime, token: loginForTest(t, server, username).Data.Token, records: records,
		phoneResolver: phoneResolver, sender: sender, protection: protection,
	}
}

const sensitiveRevealPlatformTenant = "platform"

func sensitiveRevealHTTPManifests(t *testing.T, policyMode string) []capability.Manifest {
	t.Helper()
	manifests := core.DefaultManifests()
	for manifestIndex := range manifests {
		for providerIndex := range manifests[manifestIndex].AuthProviders {
			provider := &manifests[manifestIndex].AuthProviders[providerIndex]
			if provider.ID == "oidc" {
				provider.Configured = true
			}
		}
	}
	manifests = append(manifests, capability.Manifest{
		ID: "sensitive-reveal-http-test", Name: "Sensitive Reveal HTTP Test", Version: "0.1.0",
		Admin: capability.AdminSurface{
			RevealPolicies: []capability.AdminRevealPolicy{{
				ID: sensitiveRevealHTTPPolicy, Mode: policyMode,
				Factors: []string{capability.AdminRevealFactorOIDCReauthentication, capability.AdminRevealFactorSMSOTP},
				Purposes: []capability.AdminRevealPurpose{
					{Code: sensitiveRevealHTTPPurpose, Label: capability.Text("客户支持", "Customer Support")},
					{Code: sensitiveRevealHTTPOtherPurpose, Label: capability.Text("事件响应", "Incident Response")},
				},
				ChallengeTTLSeconds: 300, GrantTTLSeconds: 60,
			}},
			Resources: []capability.AdminResource{{
				Resource: sensitiveRevealHTTPResource,
				Title:    capability.Text("敏感记录", "Sensitive Records"), Description: capability.Text("HTTP 测试。", "HTTP tests."),
				PermissionPrefix: "admin:sensitive-record",
				Deletion:         &capability.AdminResourceDeletionPolicy{Mode: capability.AdminDeletionSoftDelete, PolicyVersion: 1},
				Protection:       &capability.AdminResourceProtection{SchemaVersion: 1, Scope: "tenant-field", TenantField: "tenantCode"},
				Fields: []capability.AdminField{
					{Key: "code", Label: capability.Text("编码", "Code"), Type: "text", Source: "record", Required: true, InTable: true, InDetail: true},
					{Key: "name", Label: capability.Text("名称", "Name"), Type: "text", Source: "record", Required: true, InTable: true, InDetail: true},
					{Key: "status", Label: capability.Text("状态", "Status"), Type: "text", Source: "record", InTable: true, InDetail: true},
					{Key: "tenantCode", Label: capability.Text("租户", "Tenant"), Type: "text", Source: "values", Required: true, InDetail: true},
					{Key: "orgUnitCode", Label: capability.Text("组织", "Organization"), Type: "text", Source: "values", Required: true, InDetail: true},
					sensitiveRevealHTTPAdminField(sensitiveRevealHTTPField, true),
					sensitiveRevealHTTPAdminField(sensitiveRevealHTTPSecondaryField, false),
				},
				SearchFields: []string{"code", "name"}, DefaultSortKey: "code",
			}},
		},
	})
	if err := capability.ValidateAdminSurface(manifests); err != nil {
		t.Fatalf("ValidateAdminSurface() error = %v", err)
	}
	return manifests
}

func sensitiveRevealHTTPAdminField(key string, reveal bool) capability.AdminField {
	field := capability.AdminField{
		Key: key, Label: capability.Text("敏感字段", "Sensitive Field"), Type: "text", Source: "values", InDetail: true,
		Sensitivity: capability.FieldSensitivitySensitive, StorageMode: capability.FieldStorageEncrypted,
		ResponseMode: capability.FieldProjectionPrivileged, ExportMode: capability.FieldProjectionOmitted,
		Protection: &capability.AdminFieldProtection{Format: dataprotection.FormatAES256GCMV1, Normalization: dataprotection.NormalizationRawV1},
	}
	if reveal {
		field.Reveal = &capability.AdminFieldReveal{
			PolicyID: sensitiveRevealHTTPPolicy, Permission: sensitiveRevealHTTPFieldPermission, CopyAllowed: true,
		}
	}
	return field
}

func (h *sensitiveRevealHTTPHarness) fieldPath(recordID string, suffix string) string {
	return "/api/admin/resources/" + sensitiveRevealHTTPResource + "/" + recordID + "/fields/" + sensitiveRevealHTTPField + suffix
}

func (h *sensitiveRevealHTTPHarness) request(method string, target string, body string, token string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	h.server.Router().ServeHTTP(recorder, request)
	return recorder
}

func (h *sensitiveRevealHTTPHarness) beginChallenge(t *testing.T, recordID string, purpose string, token string) adminSensitiveRevealChallengeResponse {
	t.Helper()
	body := `{"purpose":"` + purpose + `"}`
	recorder := h.request(http.MethodPost, h.fieldPath(recordID, "/reveal/challenges"), body, token)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("POST reveal challenge status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	return decodeSensitiveRevealData[adminSensitiveRevealChallengeResponse](t, recorder)
}

func (h *sensitiveRevealHTTPHarness) completeSMS(t *testing.T, recordID string, purpose string, challenge adminSensitiveRevealChallengeResponse, start adminSensitiveRevealSMSStartResponse, code string, token string) adminSensitiveRevealFactorCompleteResponse {
	t.Helper()
	body := `{"challengeToken":"` + challenge.ChallengeToken + `","purpose":"` + purpose + `","transactionToken":"` + start.TransactionToken + `","code":"` + code + `"}`
	recorder := h.request(
		http.MethodPost,
		h.fieldPath(recordID, "/reveal/challenges/"+challenge.ChallengeID+"/factors/sms/complete"),
		body,
		token,
	)
	if recorder.Code != http.StatusOK {
		t.Fatalf("POST SMS complete status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	return decodeSensitiveRevealData[adminSensitiveRevealFactorCompleteResponse](t, recorder)
}

func (h *sensitiveRevealHTTPHarness) issueSMSGrant(t *testing.T, recordID string, purpose string, token string) (adminSensitiveRevealChallengeResponse, adminSensitiveRevealSMSStartResponse, adminSensitiveRevealFactorCompleteResponse) {
	t.Helper()
	challenge := h.beginChallenge(t, recordID, purpose, token)
	body := `{"challengeToken":"` + challenge.ChallengeToken + `","purpose":"` + purpose + `"}`
	recorder := h.request(
		http.MethodPost,
		h.fieldPath(recordID, "/reveal/challenges/"+challenge.ChallengeID+"/factors/sms/start"),
		body,
		token,
	)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("POST SMS start status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	start := decodeSensitiveRevealData[adminSensitiveRevealSMSStartResponse](t, recorder)
	grant := h.completeSMS(t, recordID, purpose, challenge, start, start.DebugCode, token)
	return challenge, start, grant
}

func decodeSensitiveRevealData[T any](t *testing.T, recorder *httptest.ResponseRecorder) T {
	t.Helper()
	var payload Response[T]
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode sensitive reveal response: %v body = %s", err, recorder.Body.String())
	}
	return payload.Data
}

func assertSensitiveRevealDataKeys(t *testing.T, recorder *httptest.ResponseRecorder, want ...string) {
	t.Helper()
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode reveal payload: %v", err)
	}
	var data map[string]json.RawMessage
	if err := json.Unmarshal(payload["data"], &data); err != nil {
		t.Fatalf("decode reveal data: %v body = %s", err, recorder.Body.String())
	}
	if len(data) != len(want) {
		t.Fatalf("reveal data keys = %v, want %v", mapKeys(data), want)
	}
	for _, key := range want {
		if _, ok := data[key]; !ok {
			t.Fatalf("reveal data keys = %v, missing %q", mapKeys(data), key)
		}
	}
}

func mapKeys(values map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}

type sensitiveRevealPhoneResolverStub struct {
	phone    AdminStepUpPhone
	username string
	err      error
}

func (s *sensitiveRevealPhoneResolverStub) ResolveVerifiedAdminPhone(_ context.Context, username string) (AdminStepUpPhone, error) {
	s.username = username
	if s.err != nil {
		return AdminStepUpPhone{}, s.err
	}
	return s.phone, nil
}

type sensitiveRevealFailingResponseWriter struct {
	header http.Header
	status int
	err    error
	cancel context.CancelFunc
}

func (w *sensitiveRevealFailingResponseWriter) Header() http.Header { return w.header }

func (w *sensitiveRevealFailingResponseWriter) WriteHeader(status int) { w.status = status }

func (w *sensitiveRevealFailingResponseWriter) Write([]byte) (int, error) {
	if w.cancel != nil {
		w.cancel()
	}
	return 0, w.err
}

type sensitiveRevealContextCheckingStore struct {
	sensitivereveal.Store
	sawCanceled   bool
	terminalCalls int
}

func (s *sensitiveRevealContextCheckingStore) RecordRevealResult(ctx context.Context, command sensitivereveal.RecordRevealResultCommand) error {
	s.terminalCalls++
	if ctx.Err() != nil {
		s.sawCanceled = true
		return ctx.Err()
	}
	return s.Store.RecordRevealResult(ctx, command)
}

type sensitiveRevealPhoneSenderStub struct {
	phone   string
	purpose string
	code    string
	err     error
}

func (s *sensitiveRevealPhoneSenderStub) Send(_ context.Context, phone string, purpose string, code string) error {
	s.phone = phone
	s.purpose = purpose
	s.code = code
	return s.err
}

func (*sensitiveRevealPhoneSenderStub) Kind() string {
	return PhoneVerificationProviderDebug
}

type sensitiveRevealIdentityResolverStub struct {
	stepUpStarts   int
	stepUpResolves int
	startErr       error
	resolveErr     error
}

func (s *sensitiveRevealIdentityResolverStub) StartAdminIdentity(context.Context, AdminIdentityStartInput) (AdminIdentityStart, error) {
	return AdminIdentityStart{}, ErrAdminIdentityInvalid
}

func (s *sensitiveRevealIdentityResolverStub) ResolveAdminIdentity(context.Context, AdminIdentityResolveInput) (AdminIdentity, error) {
	return AdminIdentity{}, ErrAdminIdentityInvalid
}

func (s *sensitiveRevealIdentityResolverStub) StartAdminStepUpIdentity(_ context.Context, _ AdminIdentityStartInput) (AdminIdentityStart, error) {
	s.stepUpStarts++
	if s.startErr != nil {
		return AdminIdentityStart{}, s.startErr
	}
	return AdminIdentityStart{
		AuthorizationURL: "https://id.example/authorize?state=state", State: "state",
		ExpiresAt: time.Date(2026, time.July, 14, 10, 5, 0, 0, time.UTC),
	}, nil
}

func (s *sensitiveRevealIdentityResolverStub) ResolveAdminStepUpIdentity(_ context.Context, _ AdminIdentityResolveInput) (AdminStepUpIdentity, error) {
	s.stepUpResolves++
	if s.resolveErr != nil {
		return AdminStepUpIdentity{}, s.resolveErr
	}
	return AdminStepUpIdentity{
		AdminIdentity:   AdminIdentity{Issuer: "https://id.example", ProviderSubject: "subject-123"},
		AuthenticatedAt: time.Date(2026, time.July, 14, 10, 0, 0, 0, time.UTC), AuthenticationMethod: []string{"pwd"},
	}, nil
}

type sensitiveRevealIdentityBindingStub struct {
	username string
	issuer   string
	subject  string
	err      error
}

func (s *sensitiveRevealIdentityBindingStub) ResolveAdminIdentityBinding(_ context.Context, input AdminIdentityBindingInput) (AdminIdentityBinding, error) {
	s.issuer = input.Issuer
	s.subject = input.ProviderSubject
	if s.err != nil {
		return AdminIdentityBinding{}, s.err
	}
	return AdminIdentityBinding{Username: s.username}, nil
}

func (*sensitiveRevealIdentityBindingStub) ProvisionAdminIdentityBinding(context.Context, AdminIdentityProvisionInput) (AdminIdentityBinding, error) {
	return AdminIdentityBinding{}, ErrAdminIdentityBindingInvalid
}

func (*sensitiveRevealIdentityBindingStub) ValidateAdminIdentityBindingReadiness(context.Context, capability.AuthProvider) error {
	return nil
}

type sensitiveRevealProtectionRuntime struct {
	delegate        dataprotection.Runtime
	revealCalls     int
	lastRevealField string
}

func newSensitiveRevealProtectionRuntime(t *testing.T) *sensitiveRevealProtectionRuntime {
	t.Helper()
	provider, err := dataprotection.NewStaticKeyProvider(dataprotection.StaticKeyProviderConfig{
		Kind: dataprotection.ProviderEnvAES256, ActiveEncryptionKeyID: "enc-v1", ActiveBlindIndexKeyID: "idx-v1",
		EncryptionKeys: map[string][]byte{"enc-v1": []byte(strings.Repeat("e", 32))},
		BlindIndexKeys: map[string][]byte{"idx-v1": []byte(strings.Repeat("i", 32))},
	})
	if err != nil {
		t.Fatalf("NewStaticKeyProvider() error = %v", err)
	}
	return &sensitiveRevealProtectionRuntime{delegate: dataprotection.NewRuntime(provider)}
}

func (r *sensitiveRevealProtectionRuntime) Protect(ctx context.Context, value string, policy dataprotection.FieldPolicy, fieldContext dataprotection.FieldContext) (string, error) {
	return r.delegate.Protect(ctx, value, policy, fieldContext)
}

func (r *sensitiveRevealProtectionRuntime) Validate(ctx context.Context, value string, policy dataprotection.FieldPolicy, fieldContext dataprotection.FieldContext) error {
	return r.delegate.Validate(ctx, value, policy, fieldContext)
}

func (r *sensitiveRevealProtectionRuntime) Reveal(ctx context.Context, value string, policy dataprotection.FieldPolicy, fieldContext dataprotection.FieldContext) (string, error) {
	r.revealCalls++
	r.lastRevealField = fieldContext.FieldKey
	return r.delegate.Reveal(ctx, value, policy, fieldContext)
}

func (r *sensitiveRevealProtectionRuntime) MatchExact(ctx context.Context, envelope string, candidate string, policy dataprotection.FieldPolicy, fieldContext dataprotection.FieldContext) (bool, error) {
	return r.delegate.MatchExact(ctx, envelope, candidate, policy, fieldContext)
}

type sensitiveRevealRateLimiter struct {
	deny  ratelimit.Operation
	calls map[ratelimit.Operation]int
}

func newSensitiveRevealRateLimiter(deny ratelimit.Operation) *sensitiveRevealRateLimiter {
	return &sensitiveRevealRateLimiter{deny: deny, calls: map[ratelimit.Operation]int{}}
}

func (s *sensitiveRevealRateLimiter) Allow(_ context.Context, key string, _ int, _ time.Duration) (ratelimit.Decision, error) {
	for _, operation := range []ratelimit.Operation{
		ratelimit.OperationAdminLogin,
		ratelimit.OperationSensitiveRevealChallenge,
		ratelimit.OperationSensitiveRevealFactor,
		ratelimit.OperationSensitiveRevealConsume,
	} {
		if !strings.Contains(key, ":"+string(operation)+":") {
			continue
		}
		s.calls[operation]++
		if s.deny == operation {
			return ratelimit.Decision{RetryAfter: 7 * time.Second}, nil
		}
		return ratelimit.Decision{Allowed: true}, nil
	}
	return ratelimit.Decision{}, errors.New("unexpected rate limit operation")
}

func assertSensitiveRevealRateLimited(t *testing.T, recorder *httptest.ResponseRecorder, limiter *sensitiveRevealRateLimiter, operation ratelimit.Operation) {
	t.Helper()
	if recorder.Code != http.StatusTooManyRequests || recorder.Header().Get("Retry-After") != "7" {
		t.Fatalf("rate limited response status = %d retry-after = %q body = %s", recorder.Code, recorder.Header().Get("Retry-After"), recorder.Body.String())
	}
	if limiter.calls[operation] != 1 {
		t.Fatalf("rate limiter calls[%s] = %d, want 1", operation, limiter.calls[operation])
	}
}
