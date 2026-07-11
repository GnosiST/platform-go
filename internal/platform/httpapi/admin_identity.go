package httpapi

import (
	"context"
	"errors"
	"time"

	"platform-go/internal/platform/capability"
)

var (
	ErrAdminIdentityInvalid          = errors.New("invalid admin identity")
	ErrAdminIdentityTransaction      = errors.New("invalid admin identity transaction")
	ErrAdminIdentityProviderExchange = errors.New("admin identity provider exchange failed")
)

type AdminIdentityResolver interface {
	StartAdminIdentity(context.Context, AdminIdentityStartInput) (AdminIdentityStart, error)
	ResolveAdminIdentity(context.Context, AdminIdentityResolveInput) (AdminIdentity, error)
}

type AdminIdentityStartInput struct {
	Provider      capability.AuthProvider
	CodeChallenge string
}

type AdminIdentityStart struct {
	AuthorizationURL string
	State            string
	ExpiresAt        time.Time
}

type AdminIdentityResolveInput struct {
	Provider     capability.AuthProvider
	Code         string
	State        string
	CodeVerifier string
}

type AdminIdentity struct {
	Issuer          string
	ProviderSubject string
}
