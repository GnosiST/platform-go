package oidc

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestStateExpiresAfterFiveMinutes(t *testing.T) {
	now := time.Date(2026, time.July, 11, 10, 0, 0, 0, time.UTC)
	codec, err := newStateCodec([]byte("0123456789abcdef0123456789abcdef"), func() time.Time { return now })
	if err != nil {
		t.Fatalf("newStateCodec() error = %v", err)
	}

	signedState, issued, err := codec.issue("oidc", "challenge")
	if err != nil {
		t.Fatalf("issue() error = %v", err)
	}
	if got, want := issued.ExpiresAt, now.Add(5*time.Minute); !got.Equal(want) {
		t.Fatalf("ExpiresAt = %v, want %v", got, want)
	}

	now = now.Add(5*time.Minute - time.Second)
	if _, err := codec.verify(signedState, "oidc"); err != nil {
		t.Fatalf("verify() before expiry error = %v", err)
	}

	now = now.Add(time.Second)
	if _, err := codec.verify(signedState, "oidc"); !errors.Is(err, errInvalidState) {
		t.Fatalf("verify() at expiry error = %v, want invalid state", err)
	}
}

func TestStateRejectsSignatureCorruption(t *testing.T) {
	codec, err := newStateCodec([]byte("0123456789abcdef0123456789abcdef"), time.Now)
	if err != nil {
		t.Fatalf("newStateCodec() error = %v", err)
	}
	signedState, _, err := codec.issue("oidc", "challenge")
	if err != nil {
		t.Fatalf("issue() error = %v", err)
	}

	parts := strings.Split(signedState, ".")
	if len(parts) != 2 || len(parts[1]) == 0 {
		t.Fatalf("signed state has unexpected format")
	}
	replacement := byte('A')
	if parts[1][0] == replacement {
		replacement = 'B'
	}
	parts[1] = string(replacement) + parts[1][1:]

	if _, err := codec.verify(strings.Join(parts, "."), "oidc"); !errors.Is(err, errInvalidState) {
		t.Fatalf("verify() corrupted signature error = %v, want invalid state", err)
	}
}

func TestStateRejectsProviderMismatch(t *testing.T) {
	codec, err := newStateCodec([]byte("0123456789abcdef0123456789abcdef"), time.Now)
	if err != nil {
		t.Fatalf("newStateCodec() error = %v", err)
	}
	signedState, _, err := codec.issue("oidc", "challenge")
	if err != nil {
		t.Fatalf("issue() error = %v", err)
	}

	if _, err := codec.verify(signedState, "other-provider"); !errors.Is(err, errInvalidState) {
		t.Fatalf("verify() provider mismatch error = %v, want invalid state", err)
	}
}

func TestStateContainsOnlyTransactionClaims(t *testing.T) {
	codec, err := newStateCodec([]byte("0123456789abcdef0123456789abcdef"), time.Now)
	if err != nil {
		t.Fatalf("newStateCodec() error = %v", err)
	}
	signedState, issued, err := codec.issue("oidc", "challenge")
	if err != nil {
		t.Fatalf("issue() error = %v", err)
	}
	if issued.Nonce == "" || issued.TransactionID == "" {
		t.Fatalf("issued state is missing generated transaction claims")
	}

	payloadPart, _, ok := strings.Cut(signedState, ".")
	if !ok {
		t.Fatalf("signed state has unexpected format")
	}
	payload, err := base64.RawURLEncoding.DecodeString(payloadPart)
	if err != nil {
		t.Fatalf("DecodeString() error = %v", err)
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	for _, key := range []string{"flow", "provider_id", "nonce", "code_challenge", "transaction_id", "issued_at", "expires_at"} {
		if _, ok := claims[key]; !ok {
			t.Fatalf("state payload missing %q", key)
		}
	}
	if claims["flow"] != stateFlowLogin {
		t.Fatalf("state payload flow = %v, want %q", claims["flow"], stateFlowLogin)
	}
	if len(claims) != 7 {
		t.Fatalf("state payload keys = %d, want 7", len(claims))
	}
}

func TestStateSeparatesLoginAndStepUpFlows(t *testing.T) {
	codec, err := newStateCodec([]byte("0123456789abcdef0123456789abcdef"), time.Now)
	if err != nil {
		t.Fatalf("newStateCodec() error = %v", err)
	}
	loginState, loginClaims, err := codec.issue("oidc", "login-challenge")
	if err != nil {
		t.Fatalf("issue() error = %v", err)
	}
	stepUpState, stepUpClaims, err := codec.issueForFlow("oidc", "step-up-challenge", stateFlowStepUp)
	if err != nil {
		t.Fatalf("issueForFlow() error = %v", err)
	}
	if loginClaims.Flow != stateFlowLogin || stepUpClaims.Flow != stateFlowStepUp {
		t.Fatalf("issued flows = %q/%q", loginClaims.Flow, stepUpClaims.Flow)
	}
	if _, err := codec.verify(loginState, "oidc"); err != nil {
		t.Fatalf("verify(login) error = %v", err)
	}
	if _, err := codec.verifyForFlow(stepUpState, "oidc", stateFlowStepUp); err != nil {
		t.Fatalf("verifyForFlow(step-up) error = %v", err)
	}
	if _, err := codec.verifyForFlow(loginState, "oidc", stateFlowStepUp); !errors.Is(err, errInvalidState) {
		t.Fatalf("verify login state as step-up error = %v, want invalid state", err)
	}
	if _, err := codec.verify(stepUpState, "oidc"); !errors.Is(err, errInvalidState) {
		t.Fatalf("verify step-up state as login error = %v, want invalid state", err)
	}
}

func TestDeriveStateKeyUsesDedicatedContext(t *testing.T) {
	secret := "jwt-secret-value"
	first := DeriveStateKey(secret)
	second := DeriveStateKey(secret)
	if len(first) != 32 || string(first) == secret {
		t.Fatalf("DeriveStateKey() returned invalid derived key")
	}
	if string(first) != string(second) {
		t.Fatalf("DeriveStateKey() is not deterministic")
	}
}
