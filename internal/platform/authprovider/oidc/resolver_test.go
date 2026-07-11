package oidc

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"platform-go/internal/platform/capability"
	"platform-go/internal/platform/httpapi"
)

func TestResolverStartsAndResolvesAdminIdentity(t *testing.T) {
	providerFixture := newOIDCProviderFixture(t)
	defer providerFixture.Close()
	now := time.Date(2026, time.July, 11, 10, 0, 0, 0, time.UTC)
	resolver := newTestResolver(t, providerFixture, now)
	provider := configuredAdminOIDCProvider()
	verifier := "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ-._~"
	challenge := testCodeChallenge(verifier)

	start, err := resolver.StartAdminIdentity(context.Background(), httpapi.AdminIdentityStartInput{
		Provider:      provider,
		CodeChallenge: challenge,
	})
	if err != nil {
		t.Fatalf("StartAdminIdentity() error = %v", err)
	}
	if start.State == "" || start.AuthorizationURL == "" || !start.ExpiresAt.Equal(now.Add(5*time.Minute)) {
		t.Fatalf("StartAdminIdentity() returned incomplete transaction")
	}
	authorizationURL, err := url.Parse(start.AuthorizationURL)
	if err != nil {
		t.Fatalf("Parse() authorization URL error = %v", err)
	}
	query := authorizationURL.Query()
	if query.Get("state") != start.State || query.Get("code_challenge") != challenge || query.Get("code_challenge_method") != "S256" || query.Get("nonce") == "" {
		t.Fatalf("authorization URL is missing state, nonce, or S256 PKCE parameters")
	}
	providerFixture.SetTokenClaims(providerFixture.URL(), "platform-admin", query.Get("nonce"))

	identity, err := resolver.ResolveAdminIdentity(context.Background(), httpapi.AdminIdentityResolveInput{
		Provider:     provider,
		Code:         "single-use-code",
		State:        start.State,
		CodeVerifier: verifier,
	})
	if err != nil || identity.ProviderSubject != "subject-123" || identity.Issuer != providerFixture.URL() {
		t.Fatalf("identity = %+v error = %v", identity, err)
	}
	request := providerFixture.LastTokenRequest()
	if request.Code != "single-use-code" || request.CodeVerifier != verifier || request.ClientID != "platform-admin" || request.ClientSecret != "fixture-client-secret" {
		t.Fatalf("token request did not use the configured code, PKCE verifier, and client credentials")
	}
	serialized, err := json.Marshal(start)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	assertDoesNotContain(t, string(serialized), "fixture-client-secret", verifier, "single-use-code", "subject-123")
}

func TestResolverRejectsPKCEMismatchBeforeProviderExchange(t *testing.T) {
	providerFixture := newOIDCProviderFixture(t)
	defer providerFixture.Close()
	resolver := newTestResolver(t, providerFixture, time.Now().UTC())
	provider := configuredAdminOIDCProvider()

	start, err := resolver.StartAdminIdentity(context.Background(), httpapi.AdminIdentityStartInput{
		Provider:      provider,
		CodeChallenge: testCodeChallenge("expected-verifier"),
	})
	if err != nil {
		t.Fatalf("StartAdminIdentity() error = %v", err)
	}
	_, err = resolver.ResolveAdminIdentity(context.Background(), httpapi.AdminIdentityResolveInput{
		Provider:     provider,
		Code:         "single-use-code",
		State:        start.State,
		CodeVerifier: "wrong-verifier",
	})
	if !errors.Is(err, httpapi.ErrAdminIdentityTransaction) {
		t.Fatalf("ResolveAdminIdentity() error did not normalize to transaction failure")
	}
	if providerFixture.TokenRequestCount() != 0 {
		t.Fatalf("token endpoint was called for a PKCE mismatch")
	}
}

func TestResolverNormalizesIDTokenVerificationFailures(t *testing.T) {
	tests := []struct {
		name     string
		mutate   func(*oidcProviderFixture)
		redacted string
	}{
		{
			name: "nonce mismatch",
			mutate: func(provider *oidcProviderFixture) {
				provider.SetClaimNonce("raw-nonce-marker")
			},
			redacted: "raw-nonce-marker",
		},
		{
			name: "issuer mismatch",
			mutate: func(provider *oidcProviderFixture) {
				provider.SetClaimIssuer("https://raw-issuer-marker.invalid")
			},
			redacted: "raw-issuer-marker",
		},
		{
			name: "audience mismatch",
			mutate: func(provider *oidcProviderFixture) {
				provider.SetClaimAudience("raw-audience-marker")
			},
			redacted: "raw-audience-marker",
		},
		{
			name: "signature corruption",
			mutate: func(provider *oidcProviderFixture) {
				provider.SetCorruptIDTokenSignature()
			},
			redacted: "subject-123",
		},
		{
			name: "expired token",
			mutate: func(provider *oidcProviderFixture) {
				provider.SetExpiredIDToken()
			},
			redacted: "subject-123",
		},
		{
			name: "empty subject",
			mutate: func(provider *oidcProviderFixture) {
				provider.SetClaimSubject("")
			},
			redacted: "subject-123",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			providerFixture := newOIDCProviderFixture(t)
			defer providerFixture.Close()
			resolver := newTestResolver(t, providerFixture, time.Now().UTC())
			provider := configuredAdminOIDCProvider()
			verifier := "verification-secret-marker"
			start, err := resolver.StartAdminIdentity(context.Background(), httpapi.AdminIdentityStartInput{
				Provider:      provider,
				CodeChallenge: testCodeChallenge(verifier),
			})
			if err != nil {
				t.Fatalf("StartAdminIdentity() error = %v", err)
			}
			authorizationURL, err := url.Parse(start.AuthorizationURL)
			if err != nil {
				t.Fatalf("Parse() authorization URL error = %v", err)
			}
			providerFixture.SetTokenClaims(providerFixture.URL(), "platform-admin", authorizationURL.Query().Get("nonce"))
			test.mutate(providerFixture)

			_, err = resolver.ResolveAdminIdentity(context.Background(), httpapi.AdminIdentityResolveInput{
				Provider:     provider,
				Code:         "authorization-code-marker",
				State:        start.State,
				CodeVerifier: verifier,
			})
			if !errors.Is(err, httpapi.ErrAdminIdentityProviderExchange) {
				t.Fatalf("ResolveAdminIdentity() error did not normalize to provider exchange failure")
			}
			assertDoesNotContain(t, err.Error(), test.redacted, verifier, "authorization-code-marker", "fixture-client-secret", providerFixture.LastIDToken())
		})
	}
}

func TestResolverNormalizesAndRedactsUpstreamTokenError(t *testing.T) {
	providerFixture := newOIDCProviderFixture(t)
	defer providerFixture.Close()
	providerFixture.SetTokenError(http.StatusBadRequest, `{"error":"invalid_grant","error_description":"raw-provider-response-marker"}`)
	resolver := newTestResolver(t, providerFixture, time.Now().UTC())
	provider := configuredAdminOIDCProvider()
	verifier := "verification-secret-marker"
	start, err := resolver.StartAdminIdentity(context.Background(), httpapi.AdminIdentityStartInput{
		Provider:      provider,
		CodeChallenge: testCodeChallenge(verifier),
	})
	if err != nil {
		t.Fatalf("StartAdminIdentity() error = %v", err)
	}

	_, err = resolver.ResolveAdminIdentity(context.Background(), httpapi.AdminIdentityResolveInput{
		Provider:     provider,
		Code:         "authorization-code-marker",
		State:        start.State,
		CodeVerifier: verifier,
	})
	if !errors.Is(err, httpapi.ErrAdminIdentityProviderExchange) {
		t.Fatalf("ResolveAdminIdentity() error did not normalize to provider exchange failure")
	}
	assertDoesNotContain(t, err.Error(), "raw-provider-response-marker", verifier, "authorization-code-marker", "fixture-client-secret")
}

func TestResolverRequiresConfiguredAdminOIDCProviderAndS256Challenge(t *testing.T) {
	providerFixture := newOIDCProviderFixture(t)
	defer providerFixture.Close()
	resolver := newTestResolver(t, providerFixture, time.Now().UTC())
	validProvider := configuredAdminOIDCProvider()

	tests := []struct {
		name      string
		provider  capability.AuthProvider
		challenge string
	}{
		{name: "disabled", provider: func() capability.AuthProvider { value := validProvider; value.Enabled = false; return value }(), challenge: testCodeChallenge("verifier")},
		{name: "unconfigured", provider: func() capability.AuthProvider { value := validProvider; value.Configured = false; return value }(), challenge: testCodeChallenge("verifier")},
		{name: "wrong kind", provider: func() capability.AuthProvider { value := validProvider; value.Kind = "wechat"; return value }(), challenge: testCodeChallenge("verifier")},
		{name: "wrong audience", provider: func() capability.AuthProvider {
			value := validProvider
			value.Audiences = []capability.AuthProviderAudience{capability.AuthProviderAudienceApp}
			return value
		}(), challenge: testCodeChallenge("verifier")},
		{name: "invalid challenge", provider: validProvider, challenge: "plain-challenge"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := resolver.StartAdminIdentity(context.Background(), httpapi.AdminIdentityStartInput{
				Provider:      test.provider,
				CodeChallenge: test.challenge,
			})
			if !errors.Is(err, httpapi.ErrAdminIdentityInvalid) {
				t.Fatalf("StartAdminIdentity() error did not normalize to invalid identity")
			}
		})
	}
}

func newTestResolver(t *testing.T, provider *oidcProviderFixture, now time.Time) *Resolver {
	t.Helper()
	provider.SetNow(now)
	resolver, err := NewResolver(Config{
		IssuerURL:    provider.URL(),
		ClientID:     "platform-admin",
		ClientSecret: "fixture-client-secret",
		RedirectURL:  "https://admin.example/auth/oidc/callback",
		Scopes:       []string{"openid", "profile", "email"},
		StateKey:     []byte("0123456789abcdef0123456789abcdef"),
		Now:          func() time.Time { return now },
		HTTPClient:   provider.Client(),
	})
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}
	return resolver
}

func configuredAdminOIDCProvider() capability.AuthProvider {
	return capability.AuthProvider{
		ID:         "oidc",
		Kind:       "oidc",
		Enabled:    true,
		Configured: true,
		Audiences:  []capability.AuthProviderAudience{capability.AuthProviderAudienceAdmin},
	}
}

func testCodeChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func assertDoesNotContain(t *testing.T, value string, sensitiveValues ...string) {
	t.Helper()
	for _, sensitive := range sensitiveValues {
		if sensitive != "" && strings.Contains(value, sensitive) {
			t.Fatalf("value exposed sensitive fixture data")
		}
	}
}

type oidcProviderFixture struct {
	t             *testing.T
	server        *httptest.Server
	key           *rsa.PrivateKey
	mu            sync.Mutex
	claimIssuer   string
	claimAudience string
	claimNonce    string
	claimSubject  string
	now           time.Time
	expiredToken  bool
	corruptToken  bool
	tokenStatus   int
	tokenBody     string
	tokenRequests []oidcTokenRequest
	lastIDToken   string
}

type oidcTokenRequest struct {
	Code         string
	CodeVerifier string
	ClientID     string
	ClientSecret string
}

func newOIDCProviderFixture(t *testing.T) *oidcProviderFixture {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	fixture := &oidcProviderFixture{t: t, key: key, claimSubject: "subject-123"}
	fixture.server = httptest.NewServer(http.HandlerFunc(fixture.handle))
	return fixture
}

func (f *oidcProviderFixture) URL() string {
	return f.server.URL
}

func (f *oidcProviderFixture) Client() *http.Client {
	return f.server.Client()
}

func (f *oidcProviderFixture) Close() {
	f.server.Close()
}

func (f *oidcProviderFixture) SetTokenClaims(issuer string, audience string, nonce string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.claimIssuer = issuer
	f.claimAudience = audience
	f.claimNonce = nonce
}

func (f *oidcProviderFixture) SetNow(now time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = now
}

func (f *oidcProviderFixture) SetClaimIssuer(issuer string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.claimIssuer = issuer
}

func (f *oidcProviderFixture) SetClaimAudience(audience string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.claimAudience = audience
}

func (f *oidcProviderFixture) SetClaimNonce(nonce string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.claimNonce = nonce
}

func (f *oidcProviderFixture) SetClaimSubject(subject string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.claimSubject = subject
}

func (f *oidcProviderFixture) SetExpiredIDToken() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.expiredToken = true
}

func (f *oidcProviderFixture) SetCorruptIDTokenSignature() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.corruptToken = true
}

func (f *oidcProviderFixture) SetTokenError(status int, body string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tokenStatus = status
	f.tokenBody = body
}

func (f *oidcProviderFixture) TokenRequestCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.tokenRequests)
}

func (f *oidcProviderFixture) LastTokenRequest() oidcTokenRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.tokenRequests) == 0 {
		return oidcTokenRequest{}
	}
	return f.tokenRequests[len(f.tokenRequests)-1]
}

func (f *oidcProviderFixture) LastIDToken() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lastIDToken
}

func (f *oidcProviderFixture) handle(response http.ResponseWriter, request *http.Request) {
	switch request.URL.Path {
	case "/.well-known/openid-configuration":
		f.writeJSON(response, map[string]any{
			"issuer":                                f.URL(),
			"authorization_endpoint":                f.URL() + "/authorize",
			"token_endpoint":                        f.URL() + "/token",
			"jwks_uri":                              f.URL() + "/keys",
			"response_types_supported":              []string{"code"},
			"subject_types_supported":               []string{"public"},
			"id_token_signing_alg_values_supported": []string{"RS256"},
			"token_endpoint_auth_methods_supported": []string{"client_secret_basic"},
		})
	case "/keys":
		f.writeJSON(response, map[string]any{
			"keys": []map[string]string{{
				"kty": "RSA",
				"kid": "fixture-key",
				"use": "sig",
				"alg": "RS256",
				"n":   base64.RawURLEncoding.EncodeToString(f.key.PublicKey.N.Bytes()),
				"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(f.key.PublicKey.E)).Bytes()),
			}},
		})
	case "/token":
		f.handleToken(response, request)
	default:
		http.NotFound(response, request)
	}
}

func (f *oidcProviderFixture) handleToken(response http.ResponseWriter, request *http.Request) {
	if err := request.ParseForm(); err != nil {
		response.WriteHeader(http.StatusBadRequest)
		return
	}
	clientID, clientSecret, ok := request.BasicAuth()
	if !ok {
		clientID = request.Form.Get("client_id")
		clientSecret = request.Form.Get("client_secret")
	}
	f.mu.Lock()
	f.tokenRequests = append(f.tokenRequests, oidcTokenRequest{
		Code:         request.Form.Get("code"),
		CodeVerifier: request.Form.Get("code_verifier"),
		ClientID:     clientID,
		ClientSecret: clientSecret,
	})
	status := f.tokenStatus
	body := f.tokenBody
	issuer := f.claimIssuer
	audience := f.claimAudience
	nonce := f.claimNonce
	subject := f.claimSubject
	now := f.now
	expiredToken := f.expiredToken
	corruptToken := f.corruptToken
	f.mu.Unlock()
	if status != 0 {
		response.Header().Set("Content-Type", "application/json")
		response.WriteHeader(status)
		_, _ = response.Write([]byte(body))
		return
	}
	if clientID != "platform-admin" || clientSecret != "fixture-client-secret" {
		response.WriteHeader(http.StatusUnauthorized)
		return
	}
	idToken := f.signIDToken(issuer, audience, nonce, subject, now, expiredToken)
	if corruptToken {
		idToken = corruptIDTokenSignature(idToken)
	}
	f.mu.Lock()
	f.lastIDToken = idToken
	f.mu.Unlock()
	f.writeJSON(response, map[string]any{
		"access_token": "fixture-access-token",
		"token_type":   "Bearer",
		"id_token":     idToken,
	})
}

func (f *oidcProviderFixture) signIDToken(issuer string, audience string, nonce string, subject string, now time.Time, expired bool) string {
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "kid": "fixture-key", "typ": "JWT"})
	now = now.UTC()
	expiresAt := now.Add(5 * time.Minute)
	issuedAt := now.Add(-time.Second)
	if expired {
		expiresAt = now.Add(-time.Minute)
		issuedAt = now.Add(-2 * time.Minute)
	}
	claims, _ := json.Marshal(map[string]any{
		"iss":   issuer,
		"sub":   subject,
		"aud":   audience,
		"exp":   expiresAt.Unix(),
		"iat":   issuedAt.Unix(),
		"nonce": nonce,
	})
	signingInput := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claims)
	digest := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, f.key, crypto.SHA256, digest[:])
	if err != nil {
		f.t.Errorf("SignPKCS1v15() failed")
		return ""
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func corruptIDTokenSignature(idToken string) string {
	signingInput, signature, ok := strings.Cut(idToken, ".")
	if !ok {
		return idToken
	}
	payload, signature, ok := strings.Cut(signature, ".")
	if !ok || signature == "" {
		return idToken
	}
	replacement := byte('A')
	if signature[0] == replacement {
		replacement = 'B'
	}
	return signingInput + "." + payload + "." + string(replacement) + signature[1:]
}

func (f *oidcProviderFixture) writeJSON(response http.ResponseWriter, payload any) {
	response.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(response).Encode(payload); err != nil {
		f.t.Errorf("Encode() response failed")
	}
}
