package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"platform-go/internal/platform/adminresource"
	"platform-go/internal/platform/capability"
)

var ErrAppIdentityInvalid = errors.New("invalid app identity")

type AppIdentityResolver interface {
	ResolveAppIdentity(context.Context, AppIdentityResolveInput) (AppIdentity, error)
}

type AppIdentityResolveInput struct {
	Provider     capability.AuthProvider
	Code         string
	UsernameHint string
}

type AppIdentity struct {
	Username             string
	DisplayName          string
	ProviderSubject      string
	ProviderUnionSubject string
}

type AppIdentityBindingStore interface {
	ResolveAppIdentityBinding(context.Context, AppIdentityBindingInput) (AppIdentityBinding, error)
}

type AppIdentityBindingInput struct {
	Provider        capability.AuthProvider
	ProviderSubject string
	UsernameHint    string
	Now             time.Time
}

type AppIdentityBinding struct {
	Username string
}

const appIdentitiesResource = "app-identities"

type resourceAppIdentityBindingStore struct {
	resources *adminresource.Store
	now       func() time.Time
}

func newResourceAppIdentityBindingStore(resources *adminresource.Store, now func() time.Time) AppIdentityBindingStore {
	return resourceAppIdentityBindingStore{resources: resources, now: now}
}

func (s resourceAppIdentityBindingStore) ResolveAppIdentityBinding(ctx context.Context, input AppIdentityBindingInput) (AppIdentityBinding, error) {
	if err := ctx.Err(); err != nil {
		return AppIdentityBinding{}, err
	}
	providerSubject := strings.TrimSpace(input.ProviderSubject)
	if providerSubject == "" {
		return AppIdentityBinding{}, ErrAppIdentityInvalid
	}
	now := input.Now
	if now.IsZero() {
		if s.now != nil {
			now = s.now()
		} else {
			now = time.Now()
		}
	}
	subjectHash := appProviderSubjectHash(input.Provider, providerSubject)
	username := appBindingUsername(input.Provider, subjectHash, input.UsernameHint)
	if username == "" {
		return AppIdentityBinding{}, ErrAppIdentityInvalid
	}

	records, err := s.resources.List(appIdentitiesResource)
	if errors.Is(err, adminresource.ErrUnknownResource) {
		return AppIdentityBinding{Username: username}, nil
	}
	if err != nil {
		return AppIdentityBinding{}, err
	}
	for _, record := range records {
		values := record.Values
		if record.Status == "disabled" || values["provider"] != input.Provider.ID || values["providerSubjectHash"] != subjectHash {
			continue
		}
		storedUsername := appUsername(values["appUsername"])
		if strings.TrimSpace(values["appUsername"]) == "" {
			return AppIdentityBinding{}, ErrAppIdentityInvalid
		}
		nextValues := cloneStringMap(values)
		nextValues["lastLoginAt"] = now.UTC().Format(time.RFC3339)
		_, err := s.resources.UpdateInternal(appIdentitiesResource, record.ID, adminresource.WriteInput{
			Code:        record.Code,
			Name:        record.Name,
			Status:      record.Status,
			Description: record.Description,
			Values:      nextValues,
		})
		if errors.Is(err, adminresource.ErrUnknownResource) {
			return AppIdentityBinding{Username: storedUsername}, nil
		}
		if err != nil {
			return AppIdentityBinding{}, err
		}
		return AppIdentityBinding{Username: storedUsername}, nil
	}

	values := map[string]string{
		"provider":            input.Provider.ID,
		"providerKind":        input.Provider.Kind,
		"providerScope":       input.Provider.ID,
		"providerSubjectHash": subjectHash,
		"maskedSubject":       maskProviderSubject(providerSubject),
		"appUsername":         username,
		"createdAt":           now.UTC().Format(time.RFC3339),
		"lastLoginAt":         now.UTC().Format(time.RFC3339),
	}
	_, err = s.resources.CreateInternal(appIdentitiesResource, adminresource.WriteInput{
		Code:        input.Provider.ID + "-" + subjectHash[:12],
		Name:        input.Provider.ID + " / " + username,
		Status:      "enabled",
		Description: "App identity binding managed by platform auth.",
		Values:      values,
	})
	if errors.Is(err, adminresource.ErrUnknownResource) {
		return AppIdentityBinding{Username: username}, nil
	}
	if err != nil {
		return AppIdentityBinding{}, err
	}
	return AppIdentityBinding{Username: username}, nil
}

func appProviderSubjectHash(provider capability.AuthProvider, providerSubject string) string {
	normalized := strings.Join([]string{
		strings.TrimSpace(provider.ID),
		strings.TrimSpace(provider.Kind),
		strings.TrimSpace(providerSubject),
	}, "\x00")
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}

func appBindingUsername(provider capability.AuthProvider, subjectHash string, usernameHint string) string {
	if strings.TrimSpace(usernameHint) != "" {
		return appUsername(usernameHint)
	}
	providerID := strings.TrimSpace(provider.ID)
	if providerID == "" {
		providerID = "provider"
	}
	hashPrefix := subjectHash
	if len(hashPrefix) > 10 {
		hashPrefix = hashPrefix[:10]
	}
	return "guest-" + providerID + "-" + hashPrefix
}

func maskProviderSubject(providerSubject string) string {
	value := []rune(strings.TrimSpace(providerSubject))
	if len(value) == 0 {
		return ""
	}
	if len(value) <= 8 {
		return string(value[:1]) + "***" + string(value[len(value)-1:])
	}
	return string(value[:3]) + "***" + string(value[len(value)-4:])
}
