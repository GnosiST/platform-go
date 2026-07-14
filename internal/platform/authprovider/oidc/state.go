package oidc

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const (
	stateKeyContext = "platform-admin-oidc-state-v1"
	stateLifetime   = 5 * time.Minute
	stateFlowLogin  = "login"
	stateFlowStepUp = "step-up"
)

var errInvalidState = errors.New("invalid oidc state")

type stateCodec struct {
	key []byte
	now func() time.Time
}

type stateClaims struct {
	Flow          string    `json:"flow"`
	ProviderID    string    `json:"provider_id"`
	Nonce         string    `json:"nonce"`
	CodeChallenge string    `json:"code_challenge"`
	TransactionID string    `json:"transaction_id"`
	IssuedAt      time.Time `json:"issued_at"`
	ExpiresAt     time.Time `json:"expires_at"`
}

func DeriveStateKey(jwtSecret string) []byte {
	mac := hmac.New(sha256.New, []byte(jwtSecret))
	_, _ = mac.Write([]byte(stateKeyContext))
	return mac.Sum(nil)
}

func newStateCodec(key []byte, now func() time.Time) (*stateCodec, error) {
	if len(key) == 0 {
		return nil, errInvalidState
	}
	if now == nil {
		now = time.Now
	}
	return &stateCodec{key: append([]byte(nil), key...), now: now}, nil
}

func (c *stateCodec) issue(providerID string, codeChallenge string) (string, stateClaims, error) {
	return c.issueForFlow(providerID, codeChallenge, stateFlowLogin)
}

func (c *stateCodec) issueForFlow(providerID string, codeChallenge string, flow string) (string, stateClaims, error) {
	providerID = strings.TrimSpace(providerID)
	codeChallenge = strings.TrimSpace(codeChallenge)
	flow = strings.TrimSpace(flow)
	if providerID == "" || codeChallenge == "" || (flow != stateFlowLogin && flow != stateFlowStepUp) {
		return "", stateClaims{}, errInvalidState
	}
	nonce, err := randomStateValue()
	if err != nil {
		return "", stateClaims{}, errInvalidState
	}
	transactionID, err := randomStateValue()
	if err != nil {
		return "", stateClaims{}, errInvalidState
	}
	now := c.now().UTC()
	claims := stateClaims{
		Flow:          flow,
		ProviderID:    providerID,
		Nonce:         nonce,
		CodeChallenge: codeChallenge,
		TransactionID: transactionID,
		IssuedAt:      now,
		ExpiresAt:     now.Add(stateLifetime),
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", stateClaims{}, errInvalidState
	}
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	signature := c.signature(encodedPayload)
	return encodedPayload + "." + base64.RawURLEncoding.EncodeToString(signature), claims, nil
}

func (c *stateCodec) verify(signedState string, providerID string) (stateClaims, error) {
	return c.verifyForFlow(signedState, providerID, stateFlowLogin)
}

func (c *stateCodec) verifyForFlow(signedState string, providerID string, flow string) (stateClaims, error) {
	payloadPart, signaturePart, ok := strings.Cut(strings.TrimSpace(signedState), ".")
	if !ok || payloadPart == "" || signaturePart == "" || strings.Contains(signaturePart, ".") {
		return stateClaims{}, errInvalidState
	}
	signature, err := base64.RawURLEncoding.DecodeString(signaturePart)
	if err != nil || !hmac.Equal(signature, c.signature(payloadPart)) {
		return stateClaims{}, errInvalidState
	}
	payload, err := base64.RawURLEncoding.DecodeString(payloadPart)
	if err != nil {
		return stateClaims{}, errInvalidState
	}
	var claims stateClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return stateClaims{}, errInvalidState
	}
	now := c.now().UTC()
	if claims.Flow != strings.TrimSpace(flow) || claims.ProviderID == "" || claims.Nonce == "" || claims.CodeChallenge == "" || claims.TransactionID == "" || claims.IssuedAt.IsZero() || claims.ExpiresAt.IsZero() ||
		!hmac.Equal([]byte(claims.ProviderID), []byte(strings.TrimSpace(providerID))) ||
		!claims.ExpiresAt.Equal(claims.IssuedAt.Add(stateLifetime)) || now.Before(claims.IssuedAt) || !now.Before(claims.ExpiresAt) {
		return stateClaims{}, errInvalidState
	}
	return claims, nil
}

func (c *stateCodec) signature(encodedPayload string) []byte {
	mac := hmac.New(sha256.New, c.key)
	_, _ = mac.Write([]byte(encodedPayload))
	return mac.Sum(nil)
}

func randomStateValue() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}
