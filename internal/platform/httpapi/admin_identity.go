package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"

	"platform-go/internal/platform/adminresource"
	"platform-go/internal/platform/capability"
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
	mu        sync.Mutex
}

func NewResourceAdminIdentityBindingStore(resources *adminresource.Store, now func() time.Time) AdminIdentityBindingStore {
	return &resourceAdminIdentityBindingStore{resources: resources, now: now}
}

func (s *resourceAdminIdentityBindingStore) ResolveAdminIdentityBinding(ctx context.Context, input AdminIdentityBindingInput) (AdminIdentityBinding, error) {
	if err := ctx.Err(); err != nil {
		return AdminIdentityBinding{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	provider, issuer, subject, ok := normalizedAdminIdentityTuple(input.Provider, input.Issuer, input.ProviderSubject)
	if !ok || s.resources == nil {
		return AdminIdentityBinding{}, ErrAdminIdentityBindingInvalid
	}
	issuerHash := adminIssuerHash(issuer)
	subjectHash := adminProviderSubjectHash(provider, issuer, subject)
	records, err := s.resources.List(adminIdentitiesResource)
	if err != nil {
		return AdminIdentityBinding{}, normalizeAdminIdentityBindingStoreError(err)
	}
	match := matchingAdminIdentityRecords(records, provider, issuerHash, subjectHash)
	if match.conflict || len(match.records) != 1 || strings.TrimSpace(match.records[0].Status) != "enabled" {
		return AdminIdentityBinding{}, ErrAdminIdentityBindingInvalid
	}
	record := match.records[0]
	username := strings.TrimSpace(record.Values["platformUsername"])
	if username == "" {
		return AdminIdentityBinding{}, ErrAdminIdentityBindingInvalid
	}
	now := s.resolveNow(input.Now)
	values := cloneStringMap(record.Values)
	values["lastLoginAt"] = now.UTC().Format(time.RFC3339)
	if _, err := s.resources.Update(adminIdentitiesResource, record.ID, adminresource.WriteInput{
		Code: record.Code, Name: record.Name, Status: record.Status, Description: record.Description, Values: values,
	}); err != nil {
		return AdminIdentityBinding{}, normalizeAdminIdentityBindingStoreError(err)
	}
	return AdminIdentityBinding{Username: username}, nil
}

func (s *resourceAdminIdentityBindingStore) ProvisionAdminIdentityBinding(ctx context.Context, input AdminIdentityProvisionInput) (AdminIdentityBinding, error) {
	if err := ctx.Err(); err != nil {
		return AdminIdentityBinding{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	provider, issuer, subject, ok := normalizedAdminIdentityTuple(input.Provider, input.Issuer, input.ProviderSubject)
	username := strings.TrimSpace(input.Username)
	if !ok || username == "" || s.resources == nil {
		return AdminIdentityBinding{}, ErrAdminIdentityBindingInvalid
	}
	if _, err := adminresource.ValidateAdminPrincipal(s.resources, username); err != nil {
		return AdminIdentityBinding{}, ErrAdminIdentityBindingInvalid
	}
	issuerHash := adminIssuerHash(issuer)
	subjectHash := adminProviderSubjectHash(provider, issuer, subject)
	records, err := s.resources.List(adminIdentitiesResource)
	if err != nil {
		return AdminIdentityBinding{}, normalizeAdminIdentityBindingStoreError(err)
	}
	match := matchingAdminIdentityRecords(records, provider, issuerHash, subjectHash)
	if match.conflict {
		return AdminIdentityBinding{}, ErrAdminIdentityBindingInvalid
	}
	if len(match.records) > 0 {
		if len(match.records) == 1 && strings.TrimSpace(match.records[0].Status) == "enabled" && strings.TrimSpace(match.records[0].Values["platformUsername"]) == username {
			return AdminIdentityBinding{Username: username}, nil
		}
		return AdminIdentityBinding{}, ErrAdminIdentityBindingInvalid
	}
	now := s.resolveNow(input.Now).UTC()
	values := map[string]string{
		"provider":            provider.ID,
		"providerKind":        provider.Kind,
		"issuerHash":          issuerHash,
		"providerSubjectHash": subjectHash,
		"platformUsername":    username,
		"createdAt":           now.Format(time.RFC3339),
		"lastLoginAt":         now.Format(time.RFC3339),
	}
	if _, err := s.resources.Create(adminIdentitiesResource, adminresource.WriteInput{
		Code: provider.ID + "-" + subjectHash[:12], Name: "Admin identity binding", Status: "enabled", Values: values,
	}); err != nil {
		return AdminIdentityBinding{}, normalizeAdminIdentityBindingStoreError(err)
	}
	return AdminIdentityBinding{Username: username}, nil
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
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.resources == nil {
		return ErrAdminIdentityBindingInvalid
	}
	records, err := s.resources.List(adminIdentitiesResource)
	if err != nil {
		return normalizeAdminIdentityBindingStoreError(err)
	}
	seenTuples := make(map[string]struct{})
	ready := false
	for _, record := range records {
		if strings.TrimSpace(record.Status) != "enabled" {
			continue
		}
		values := record.Values
		recordProvider := strings.TrimSpace(values["provider"])
		recordProviderKind := strings.TrimSpace(values["providerKind"])
		issuerHash := strings.TrimSpace(values["issuerHash"])
		subjectHash := strings.TrimSpace(values["providerSubjectHash"])
		if recordProvider != provider.ID || recordProviderKind != provider.Kind {
			continue
		}
		if !validAdminIdentityHash(issuerHash) || !validAdminIdentityHash(subjectHash) {
			continue
		}
		tupleKey := issuerHash + "\x00" + subjectHash
		if _, exists := seenTuples[tupleKey]; exists {
			return ErrAdminIdentityBindingInvalid
		}
		seenTuples[tupleKey] = struct{}{}
		if _, err := adminresource.ValidateAdminPrincipal(s.resources, values["platformUsername"]); err == nil {
			ready = true
		}
	}
	if ready {
		return nil
	}
	return ErrAdminIdentityBindingInvalid
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

type adminIdentityRecordMatch struct {
	records  []adminresource.Record
	conflict bool
}

func matchingAdminIdentityRecords(records []adminresource.Record, provider capability.AuthProvider, issuerHash string, subjectHash string) adminIdentityRecordMatch {
	match := adminIdentityRecordMatch{records: make([]adminresource.Record, 0, 1)}
	for _, record := range records {
		values := record.Values
		if values["issuerHash"] != issuerHash || values["providerSubjectHash"] != subjectHash {
			continue
		}
		if strings.TrimSpace(values["provider"]) != provider.ID || strings.TrimSpace(values["providerKind"]) != provider.Kind {
			match.conflict = true
			continue
		}
		match.records = append(match.records, record)
	}
	return match
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

func validAdminIdentityHash(value string) bool {
	value = strings.TrimSpace(value)
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size && hex.EncodeToString(decoded) == value
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
	if errors.Is(err, adminresource.ErrUnknownResource) || errors.Is(err, adminresource.ErrRecordNotFound) || errors.Is(err, adminresource.ErrInvalidRecord) {
		return ErrAdminIdentityBindingInvalid
	}
	return err
}
