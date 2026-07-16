package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/GnosiST/platform-go/internal/platform/adminresource"
	"github.com/GnosiST/platform-go/internal/platform/capability"
)

var (
	ErrAdminIdentityInvalid          = errors.New("invalid admin identity")
	ErrAdminIdentityTransaction      = errors.New("invalid admin identity transaction")
	ErrAdminIdentityProviderExchange = errors.New("admin identity provider exchange failed")
	ErrAdminIdentityBindingInvalid   = errors.New("invalid admin identity binding")
	ErrAdminAuthNotReady             = errors.New("admin authentication is not ready")
)

type AdminIdentityResolver interface {
	StartAdminIdentity(context.Context, AdminIdentityStartInput) (AdminIdentityStart, error)
	ResolveAdminIdentity(context.Context, AdminIdentityResolveInput) (AdminIdentity, error)
}

type AdminStepUpIdentityResolver interface {
	StartAdminStepUpIdentity(context.Context, AdminIdentityStartInput) (AdminIdentityStart, error)
	ResolveAdminStepUpIdentity(context.Context, AdminIdentityResolveInput) (AdminStepUpIdentity, error)
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

type AdminStepUpIdentity struct {
	AdminIdentity
	AuthenticatedAt      time.Time
	AuthenticationMethod []string
}

type AdminIdentityBindingStore interface {
	ResolveAdminIdentityBinding(context.Context, AdminIdentityBindingInput) (AdminIdentityBinding, error)
	ProvisionAdminIdentityBinding(context.Context, AdminIdentityProvisionInput) (AdminIdentityBinding, error)
	ValidateAdminIdentityBindingReadiness(context.Context, capability.AuthProvider) error
}

type AdminIdentityBindingInput struct {
	Provider        capability.AuthProvider
	Issuer          string
	ProviderSubject string
	Now             time.Time
}

type AdminIdentityBinding struct {
	Username string
	RecordID string
	Created  bool
}

type AdminIdentityProvisionInput struct {
	Provider        capability.AuthProvider
	Issuer          string
	ProviderSubject string
	Username        string
	Now             time.Time
}

const adminIdentitiesResource = "admin-identities"

type resourceAdminIdentityBindingStore struct {
	resources *adminresource.Store
	now       func() time.Time
}

func NewResourceAdminIdentityBindingStore(resources *adminresource.Store, now func() time.Time) AdminIdentityBindingStore {
	return &resourceAdminIdentityBindingStore{resources: resources, now: now}
}

func (s *resourceAdminIdentityBindingStore) ResolveAdminIdentityBinding(ctx context.Context, input AdminIdentityBindingInput) (AdminIdentityBinding, error) {
	if err := ctx.Err(); err != nil {
		return AdminIdentityBinding{}, err
	}
	provider, issuer, subject, ok := normalizedAdminIdentityTuple(input.Provider, input.Issuer, input.ProviderSubject)
	if !ok || s.resources == nil {
		return AdminIdentityBinding{}, ErrAdminIdentityBindingInvalid
	}
	username, err := s.resources.ResolveAdminIdentityBinding(ctx, adminresource.AdminIdentityBindingResolveInput{
		Key: adminresource.AdminIdentityBindingKey{
			Provider: provider.ID, ProviderKind: provider.Kind, IssuerHash: adminIssuerHash(issuer), ProviderSubjectHash: adminProviderSubjectHash(provider, issuer, subject),
		},
		Now: s.resolveNow(input.Now),
	})
	if err != nil {
		return AdminIdentityBinding{}, normalizeAdminIdentityBindingStoreError(err)
	}
	return AdminIdentityBinding{Username: username}, nil
}

func (s *resourceAdminIdentityBindingStore) ProvisionAdminIdentityBinding(ctx context.Context, input AdminIdentityProvisionInput) (AdminIdentityBinding, error) {
	if err := ctx.Err(); err != nil {
		return AdminIdentityBinding{}, err
	}
	provider, issuer, subject, ok := normalizedAdminIdentityTuple(input.Provider, input.Issuer, input.ProviderSubject)
	username := strings.TrimSpace(input.Username)
	if !ok || username == "" || s.resources == nil {
		return AdminIdentityBinding{}, ErrAdminIdentityBindingInvalid
	}
	result, err := s.resources.ProvisionAdminIdentityBinding(ctx, adminresource.AdminIdentityBindingProvisionInput{
		Key: adminresource.AdminIdentityBindingKey{
			Provider: provider.ID, ProviderKind: provider.Kind, IssuerHash: adminIssuerHash(issuer), ProviderSubjectHash: adminProviderSubjectHash(provider, issuer, subject),
		},
		PlatformUsername: username,
		Now:              s.resolveNow(input.Now),
	})
	if err != nil {
		return AdminIdentityBinding{RecordID: result.RecordID}, normalizeAdminIdentityBindingStoreError(err)
	}
	return AdminIdentityBinding{Username: result.PlatformUsername, RecordID: result.RecordID, Created: result.Created}, nil
}

func (s *resourceAdminIdentityBindingStore) ValidateAdminIdentityBindingReadiness(ctx context.Context, provider capability.AuthProvider) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	provider.ID = strings.TrimSpace(provider.ID)
	provider.Kind = strings.TrimSpace(provider.Kind)
	if provider.ID == "" || provider.Kind != "oidc" || !provider.Enabled || !provider.Configured || !provider.SupportsAudience(capability.AuthProviderAudienceAdmin) {
		return ErrAdminIdentityBindingInvalid
	}
	if s.resources == nil {
		return ErrAdminIdentityBindingInvalid
	}
	return normalizeAdminIdentityBindingStoreError(s.resources.ValidateAdminIdentityBindingReadiness(ctx, provider.ID, provider.Kind))
}

func ValidateAdminAuthReadiness(ctx context.Context, manifests []capability.Manifest, bindings AdminIdentityBindingStore, disableDemo bool) error {
	if !disableDemo {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return ErrAdminAuthNotReady
	}
	hasAdminProvider := false
	oidcProviders := make([]capability.AuthProvider, 0)
	for _, manifest := range manifests {
		for _, provider := range manifest.AuthProviders {
			if !provider.Enabled || !provider.Configured || !provider.SupportsAudience(capability.AuthProviderAudienceAdmin) || strings.TrimSpace(provider.Kind) == "demo" {
				continue
			}
			hasAdminProvider = true
			if strings.TrimSpace(provider.Kind) == "oidc" {
				oidcProviders = append(oidcProviders, provider)
			}
		}
	}
	if !hasAdminProvider {
		return ErrAdminAuthNotReady
	}
	for _, provider := range oidcProviders {
		if bindings == nil || bindings.ValidateAdminIdentityBindingReadiness(ctx, provider) != nil {
			return ErrAdminAuthNotReady
		}
	}
	return nil
}

func normalizedAdminIdentityTuple(provider capability.AuthProvider, issuer string, subject string) (capability.AuthProvider, string, string, bool) {
	provider.ID = strings.TrimSpace(provider.ID)
	provider.Kind = strings.TrimSpace(provider.Kind)
	issuer = strings.TrimSpace(issuer)
	subject = strings.TrimSpace(subject)
	valid := provider.ID != "" && provider.Kind != "" && issuer != "" && subject != "" && provider.Enabled && provider.Configured && provider.SupportsAudience(capability.AuthProviderAudienceAdmin)
	return provider, issuer, subject, valid
}

func adminIssuerHash(issuer string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(issuer)))
	return hex.EncodeToString(sum[:])
}

func adminProviderSubjectHash(provider capability.AuthProvider, issuer string, subject string) string {
	normalized := strings.Join([]string{
		strings.TrimSpace(provider.ID),
		strings.TrimSpace(provider.Kind),
		strings.TrimSpace(issuer),
		strings.TrimSpace(subject),
	}, "\x00")
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}

func (s *resourceAdminIdentityBindingStore) resolveNow(value time.Time) time.Time {
	if !value.IsZero() {
		return value
	}
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

func normalizeAdminIdentityBindingStoreError(err error) error {
	if errors.Is(err, adminresource.ErrAdminIdentityBindingInvalid) || errors.Is(err, adminresource.ErrUnknownResource) || errors.Is(err, adminresource.ErrRecordNotFound) || errors.Is(err, adminresource.ErrInvalidRecord) {
		return ErrAdminIdentityBindingInvalid
	}
	return err
}
