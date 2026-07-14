package oidc

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	coreoidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"platform-go/internal/platform/capability"
	"platform-go/internal/platform/httpapi"
)

type Config struct {
	IssuerURL    string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
	StateKey     []byte
	Now          func() time.Time
	HTTPClient   *http.Client
}

type Resolver struct {
	provider   *coreoidc.Provider
	oauth2     oauth2.Config
	clientID   string
	state      *stateCodec
	now        func() time.Time
	httpClient *http.Client
}

var _ httpapi.AdminIdentityResolver = (*Resolver)(nil)
var _ httpapi.AdminStepUpIdentityResolver = (*Resolver)(nil)

func NewResolver(config Config) (*Resolver, error) {
	issuerURL := strings.TrimSpace(config.IssuerURL)
	clientID := strings.TrimSpace(config.ClientID)
	clientSecret := strings.TrimSpace(config.ClientSecret)
	redirectURL := strings.TrimSpace(config.RedirectURL)
	if issuerURL == "" || clientID == "" || clientSecret == "" || redirectURL == "" || len(config.StateKey) == 0 {
		return nil, httpapi.ErrAdminIdentityInvalid
	}
	client := config.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	state, err := newStateCodec(config.StateKey, now)
	if err != nil {
		return nil, httpapi.ErrAdminIdentityInvalid
	}
	discoveryContext := coreoidc.ClientContext(context.Background(), client)
	provider, err := coreoidc.NewProvider(discoveryContext, issuerURL)
	if err != nil {
		return nil, fmt.Errorf("%w: discovery failed", httpapi.ErrAdminIdentityProviderExchange)
	}
	scopes := append([]string(nil), config.Scopes...)
	return &Resolver{
		provider: provider,
		oauth2: oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Endpoint:     provider.Endpoint(),
			RedirectURL:  redirectURL,
			Scopes:       scopes,
		},
		clientID:   clientID,
		state:      state,
		now:        now,
		httpClient: client,
	}, nil
}

func (r *Resolver) StartAdminIdentity(_ context.Context, input httpapi.AdminIdentityStartInput) (httpapi.AdminIdentityStart, error) {
	if !validAdminOIDCProvider(input.Provider) || !validS256Challenge(input.CodeChallenge) {
		return httpapi.AdminIdentityStart{}, httpapi.ErrAdminIdentityInvalid
	}
	signedState, claims, err := r.state.issue(input.Provider.ID, input.CodeChallenge)
	if err != nil {
		return httpapi.AdminIdentityStart{}, httpapi.ErrAdminIdentityTransaction
	}
	authorizationURL := r.oauth2.AuthCodeURL(
		signedState,
		oauth2.SetAuthURLParam("nonce", claims.Nonce),
		oauth2.SetAuthURLParam("code_challenge", claims.CodeChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)
	return httpapi.AdminIdentityStart{
		AuthorizationURL: authorizationURL,
		State:            signedState,
		ExpiresAt:        claims.ExpiresAt,
	}, nil
}

func (r *Resolver) StartAdminStepUpIdentity(_ context.Context, input httpapi.AdminIdentityStartInput) (httpapi.AdminIdentityStart, error) {
	if !validAdminOIDCProvider(input.Provider) || !validS256Challenge(input.CodeChallenge) {
		return httpapi.AdminIdentityStart{}, httpapi.ErrAdminIdentityInvalid
	}
	signedState, claims, err := r.state.issueForFlow(input.Provider.ID, input.CodeChallenge, stateFlowStepUp)
	if err != nil {
		return httpapi.AdminIdentityStart{}, httpapi.ErrAdminIdentityTransaction
	}
	authorizationURL := r.oauth2.AuthCodeURL(
		signedState,
		oauth2.SetAuthURLParam("nonce", claims.Nonce),
		oauth2.SetAuthURLParam("code_challenge", claims.CodeChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
		oauth2.SetAuthURLParam("prompt", "login"),
		oauth2.SetAuthURLParam("max_age", "0"),
	)
	return httpapi.AdminIdentityStart{AuthorizationURL: authorizationURL, State: signedState, ExpiresAt: claims.ExpiresAt}, nil
}

func (r *Resolver) ResolveAdminIdentity(ctx context.Context, input httpapi.AdminIdentityResolveInput) (httpapi.AdminIdentity, error) {
	identity, err := r.resolveAdminIdentity(ctx, input, stateFlowLogin, false)
	return identity.AdminIdentity, err
}

func (r *Resolver) ResolveAdminStepUpIdentity(ctx context.Context, input httpapi.AdminIdentityResolveInput) (httpapi.AdminStepUpIdentity, error) {
	return r.resolveAdminIdentity(ctx, input, stateFlowStepUp, true)
}

func (r *Resolver) resolveAdminIdentity(ctx context.Context, input httpapi.AdminIdentityResolveInput, flow string, requireFresh bool) (httpapi.AdminStepUpIdentity, error) {
	if !validAdminOIDCProvider(input.Provider) || strings.TrimSpace(input.Code) == "" || strings.TrimSpace(input.State) == "" || strings.TrimSpace(input.CodeVerifier) == "" {
		return httpapi.AdminStepUpIdentity{}, httpapi.ErrAdminIdentityInvalid
	}
	claims, err := r.state.verifyForFlow(input.State, input.Provider.ID, flow)
	if err != nil || !matchesS256Challenge(input.CodeVerifier, claims.CodeChallenge) {
		return httpapi.AdminStepUpIdentity{}, httpapi.ErrAdminIdentityTransaction
	}
	providerContext := coreoidc.ClientContext(ctx, r.httpClient)
	token, err := r.oauth2.Exchange(
		providerContext,
		strings.TrimSpace(input.Code),
		oauth2.SetAuthURLParam("code_verifier", input.CodeVerifier),
	)
	if err != nil {
		return httpapi.AdminStepUpIdentity{}, fmt.Errorf("%w: token exchange failed", httpapi.ErrAdminIdentityProviderExchange)
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || strings.TrimSpace(rawIDToken) == "" {
		return httpapi.AdminStepUpIdentity{}, fmt.Errorf("%w: id token missing", httpapi.ErrAdminIdentityProviderExchange)
	}
	verified, err := r.provider.VerifierContext(providerContext, &coreoidc.Config{
		ClientID: r.clientID,
		Now:      r.now,
	}).Verify(providerContext, rawIDToken)
	if err != nil {
		return httpapi.AdminStepUpIdentity{}, fmt.Errorf("%w: id token invalid", httpapi.ErrAdminIdentityProviderExchange)
	}
	if subtle.ConstantTimeCompare([]byte(verified.Nonce), []byte(claims.Nonce)) != 1 || strings.TrimSpace(verified.Subject) == "" {
		return httpapi.AdminStepUpIdentity{}, fmt.Errorf("%w: id token claims invalid", httpapi.ErrAdminIdentityProviderExchange)
	}
	var authentication struct {
		AuthTime int64    `json:"auth_time"`
		AMR      []string `json:"amr"`
	}
	if err := verified.Claims(&authentication); err != nil {
		return httpapi.AdminStepUpIdentity{}, fmt.Errorf("%w: id token claims invalid", httpapi.ErrAdminIdentityProviderExchange)
	}
	authenticatedAt := time.Time{}
	if authentication.AuthTime > 0 {
		authenticatedAt = time.Unix(authentication.AuthTime, 0).UTC()
	}
	if requireFresh {
		const clockSkew = 30 * time.Second
		now := r.now().UTC()
		if authenticatedAt.IsZero() || authenticatedAt.Before(claims.IssuedAt.Add(-clockSkew)) || authenticatedAt.After(now.Add(clockSkew)) {
			return httpapi.AdminStepUpIdentity{}, fmt.Errorf("%w: fresh authentication is required", httpapi.ErrAdminIdentityProviderExchange)
		}
	}
	return httpapi.AdminStepUpIdentity{
		AdminIdentity:   httpapi.AdminIdentity{Issuer: verified.Issuer, ProviderSubject: strings.TrimSpace(verified.Subject)},
		AuthenticatedAt: authenticatedAt, AuthenticationMethod: append([]string(nil), authentication.AMR...),
	}, nil
}

func validAdminOIDCProvider(provider capability.AuthProvider) bool {
	return strings.TrimSpace(provider.ID) != "" && provider.Kind == "oidc" && provider.Enabled && provider.Configured && provider.SupportsAudience(capability.AuthProviderAudienceAdmin)
}

func validS256Challenge(challenge string) bool {
	challenge = strings.TrimSpace(challenge)
	decoded, err := base64.RawURLEncoding.DecodeString(challenge)
	return err == nil && len(decoded) == sha256.Size && base64.RawURLEncoding.EncodeToString(decoded) == challenge
}

func matchesS256Challenge(verifier string, expected string) bool {
	sum := sha256.Sum256([]byte(verifier))
	actual := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) == 1
}
