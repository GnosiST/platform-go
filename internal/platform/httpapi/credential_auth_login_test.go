package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/GnosiST/platform-go/internal/platform/adminresource"
)

func TestCredentialAuthPasswordLoginAuditsRejectedCredentials(t *testing.T) {
	runtime, _ := credentialAuthRuntimeForTest(t)
	server := newTestServer(ServerOptions{Capabilities: configuredCredentialAuthManifestsForTest(t), CredentialAuth: runtime})
	loginRecorder := httptest.NewRecorder()
	loginRequest := newCredentialAuthHTTPTestRequest(
		http.MethodPost,
		"/api/auth/login",
		credentialAuthPasswordLoginBodyForTest(t, runtime, "wrong-password"),
	)
	loginRequest.Header.Set("Content-Type", "application/json")

	server.Router().ServeHTTP(loginRecorder, loginRequest)

	if loginRecorder.Code != http.StatusUnauthorized || !strings.Contains(loginRecorder.Body.String(), "AUTH_INVALID_CREDENTIALS") {
		t.Fatalf("POST credential password login with wrong password = %d body = %s, want invalid credentials", loginRecorder.Code, loginRecorder.Body.String())
	}
	audits, err := server.resources.List("audit-logs")
	if err != nil {
		t.Fatalf("List(audit-logs) error = %v", err)
	}
	audit := credentialAuthFailureAuditForTest(audits, "credential-auth:username-password", "credential-rejected")
	if audit == nil {
		t.Fatalf("credential auth failure audit missing: %+v", audits)
	}
	serialized := fmt.Sprintf("%+v", *audit)
	for _, marker := range []string{"wrong-password", "correct-password"} {
		if strings.Contains(serialized, marker) {
			t.Fatalf("credential auth failure audit leaked %q: %s", marker, serialized)
		}
	}
}

func TestCredentialAuthPasswordLoginAuditsRejectedChallenge(t *testing.T) {
	runtime, _ := credentialAuthRuntimeForTest(t)
	server := newTestServer(ServerOptions{Capabilities: configuredCredentialAuthManifestsForTest(t), CredentialAuth: runtime})
	startRecorder := httptest.NewRecorder()
	startRequest := newCredentialAuthHTTPTestRequest(http.MethodPost, "/api/auth/challenges", strings.NewReader(`{"kind":"captcha","purpose":"login"}`))
	startRequest.Header.Set("Content-Type", "application/json")
	server.Router().ServeHTTP(startRecorder, startRequest)
	if startRecorder.Code != http.StatusCreated {
		t.Fatalf("POST credential challenge start status = %d body = %s", startRecorder.Code, startRecorder.Body.String())
	}
	var started credentialAuthChallengeStartTestPayload
	if err := json.Unmarshal(startRecorder.Body.Bytes(), &started); err != nil {
		t.Fatalf("decode credential challenge start: %v body = %s", err, startRecorder.Body.String())
	}

	loginRecorder := httptest.NewRecorder()
	loginRequest := newCredentialAuthHTTPTestRequest(
		http.MethodPost,
		"/api/auth/login",
		credentialAuthPasswordLoginBodyWithChallengeForTest(t, runtime, "correct-password", started.Data.ID, "captcha", "wrong-proof", ""),
	)
	loginRequest.Header.Set("Content-Type", "application/json")

	server.Router().ServeHTTP(loginRecorder, loginRequest)

	if loginRecorder.Code != http.StatusUnauthorized || !strings.Contains(loginRecorder.Body.String(), "AUTH_INVALID_CREDENTIALS") {
		t.Fatalf("POST credential password login wrong challenge = %d body = %s, want invalid credentials", loginRecorder.Code, loginRecorder.Body.String())
	}
	audits, err := server.resources.List("audit-logs")
	if err != nil {
		t.Fatalf("List(audit-logs) error = %v", err)
	}
	if audit := credentialAuthFailureAuditForTest(audits, "credential-auth:username-password", "challenge-rejected"); audit == nil {
		t.Fatalf("credential auth challenge failure audit missing: %+v", audits)
	}
}

func credentialAuthFailureAuditForTest(audits []adminresource.Record, targetID string, reasonCode string) *adminresource.Record {
	for index := range audits {
		audit := audits[index]
		if audit.Values["action"] == "auth.login" &&
			audit.Values["outcome"] == "failure" &&
			audit.Values["targetId"] == targetID &&
			audit.Values["reasonCode"] == reasonCode {
			return &audit
		}
	}
	return nil
}
