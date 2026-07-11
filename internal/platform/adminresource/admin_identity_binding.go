package adminresource

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrAdminIdentityBindingInvalid = errors.New("invalid admin identity binding")

const (
	adminIdentitiesResource              = "admin-identities"
	adminIdentityMutationMaxAttempts     = 3
	adminIdentityBindingManagedName      = "Admin identity binding"
	adminIdentityBindingEnabledStatus    = "enabled"
	adminIdentityBindingProviderField    = "provider"
	adminIdentityBindingKindField        = "providerKind"
	adminIdentityBindingIssuerHashField  = "issuerHash"
	adminIdentityBindingSubjectHashField = "providerSubjectHash"
	adminIdentityBindingUsernameField    = "platformUsername"
	adminIdentityBindingCreatedAtField   = "createdAt"
	adminIdentityBindingLastLoginAtField = "lastLoginAt"
)

type AdminIdentityBindingKey struct {
	Provider            string
	ProviderKind        string
	IssuerHash          string
	ProviderSubjectHash string
}

type AdminIdentityBindingResolveInput struct {
	Key AdminIdentityBindingKey
	Now time.Time
}

type AdminIdentityBindingProvisionInput struct {
	Key              AdminIdentityBindingKey
	PlatformUsername string
	Now              time.Time
}

func (s *Store) ResolveAdminIdentityBinding(ctx context.Context, input AdminIdentityBindingResolveInput) (string, error) {
	if s == nil {
		return "", ErrAdminIdentityBindingInvalid
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	key, ok := normalizeAdminIdentityBindingKey(input.Key)
	if !ok {
		return "", ErrAdminIdentityBindingInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for attempt := 0; attempt < adminIdentityMutationMaxAttempts; attempt++ {
		if err := s.reloadContextLocked(ctx); err != nil {
			if errors.Is(err, ErrRevisionConflict) {
				continue
			}
			return "", err
		}
		previous := s.snapshotLocked()
		index, err := s.adminIdentityBindingIndexLocked(key)
		if err != nil {
			return "", err
		}
		record := s.resources[adminIdentitiesResource][index]
		username := strings.TrimSpace(record.Values[adminIdentityBindingUsernameField])
		if _, err := s.validateAdminPrincipalLocked(username); err != nil {
			return "", ErrAdminIdentityBindingInvalid
		}
		now := input.Now
		if now.IsZero() {
			now = s.now()
		}
		values := cloneValues(record.Values)
		values[adminIdentityBindingLastLoginAtField] = now.UTC().Format(time.RFC3339)
		record.Values = values
		record.UpdatedAt = s.now().UTC().Format(time.RFC3339)
		s.resources[adminIdentitiesResource][index] = record
		if err := s.persistContextLocked(ctx); err != nil {
			s.restoreSnapshotLocked(previous)
			if errors.Is(err, ErrRevisionConflict) {
				continue
			}
			return "", err
		}
		return username, nil
	}
	return "", ErrRevisionConflict
}

func (s *Store) ProvisionAdminIdentityBinding(ctx context.Context, input AdminIdentityBindingProvisionInput) (string, error) {
	if s == nil {
		return "", ErrAdminIdentityBindingInvalid
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	key, ok := normalizeAdminIdentityBindingKey(input.Key)
	username := strings.TrimSpace(input.PlatformUsername)
	if !ok || username == "" {
		return "", ErrAdminIdentityBindingInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for attempt := 0; attempt < adminIdentityMutationMaxAttempts; attempt++ {
		if err := s.reloadContextLocked(ctx); err != nil {
			if errors.Is(err, ErrRevisionConflict) {
				continue
			}
			return "", err
		}
		previous := s.snapshotLocked()
		if _, err := s.validateAdminPrincipalLocked(username); err != nil {
			return "", ErrAdminIdentityBindingInvalid
		}
		matches, conflict, err := s.adminIdentityBindingMatchesLocked(key)
		if err != nil || conflict || len(matches) > 1 {
			return "", ErrAdminIdentityBindingInvalid
		}
		if len(matches) == 1 {
			record := s.resources[adminIdentitiesResource][matches[0]]
			if strings.TrimSpace(record.Status) == adminIdentityBindingEnabledStatus && strings.TrimSpace(record.Values[adminIdentityBindingUsernameField]) == username {
				return username, nil
			}
			return "", ErrAdminIdentityBindingInvalid
		}
		now := input.Now
		if now.IsZero() {
			now = s.now()
		}
		now = now.UTC()
		values := map[string]string{
			adminIdentityBindingProviderField:    key.Provider,
			adminIdentityBindingKindField:        key.ProviderKind,
			adminIdentityBindingIssuerHashField:  key.IssuerHash,
			adminIdentityBindingSubjectHashField: key.ProviderSubjectHash,
			adminIdentityBindingUsernameField:    username,
			adminIdentityBindingCreatedAtField:   now.Format(time.RFC3339),
			adminIdentityBindingLastLoginAtField: now.Format(time.RFC3339),
		}
		record, err := s.recordFromInput(adminIdentitiesResource, "", WriteInput{
			Code: key.Provider + "-" + key.ProviderSubjectHash[:12], Name: adminIdentityBindingManagedName, Status: adminIdentityBindingEnabledStatus, Values: values,
		})
		if err != nil {
			return "", err
		}
		s.nextID++
		record.ID = fmt.Sprintf("%s-%d", adminIdentitiesResource, s.nextID)
		s.resources[adminIdentitiesResource] = append(s.resources[adminIdentitiesResource], record)
		if err := s.persistContextLocked(ctx); err != nil {
			s.restoreSnapshotLocked(previous)
			if errors.Is(err, ErrRevisionConflict) {
				continue
			}
			return "", err
		}
		return username, nil
	}
	return "", ErrRevisionConflict
}

func (s *Store) ValidateAdminIdentityBindingReadiness(ctx context.Context, provider string, providerKind string) error {
	if s == nil {
		return ErrAdminIdentityBindingInvalid
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	provider = strings.TrimSpace(provider)
	providerKind = strings.TrimSpace(providerKind)
	if provider == "" || providerKind == "" {
		return ErrAdminIdentityBindingInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for attempt := 0; attempt < adminIdentityMutationMaxAttempts; attempt++ {
		if err := s.reloadContextLocked(ctx); err != nil {
			if errors.Is(err, ErrRevisionConflict) {
				continue
			}
			return err
		}
		seenTuples := make(map[string]struct{})
		ready := false
		for _, record := range s.resources[adminIdentitiesResource] {
			if strings.TrimSpace(record.Status) != adminIdentityBindingEnabledStatus {
				continue
			}
			values := record.Values
			if strings.TrimSpace(values[adminIdentityBindingProviderField]) != provider || strings.TrimSpace(values[adminIdentityBindingKindField]) != providerKind {
				continue
			}
			issuerHash := strings.TrimSpace(values[adminIdentityBindingIssuerHashField])
			subjectHash := strings.TrimSpace(values[adminIdentityBindingSubjectHashField])
			if !validAdminIdentityBindingHash(issuerHash) || !validAdminIdentityBindingHash(subjectHash) {
				continue
			}
			tupleKey := issuerHash + "\x00" + subjectHash
			if _, exists := seenTuples[tupleKey]; exists {
				return ErrAdminIdentityBindingInvalid
			}
			seenTuples[tupleKey] = struct{}{}
			if _, err := s.validateAdminPrincipalLocked(values[adminIdentityBindingUsernameField]); err == nil {
				ready = true
			}
		}
		if ready {
			return nil
		}
		return ErrAdminIdentityBindingInvalid
	}
	return ErrRevisionConflict
}

func (s *Store) adminIdentityBindingIndexLocked(key AdminIdentityBindingKey) (int, error) {
	matches, conflict, err := s.adminIdentityBindingMatchesLocked(key)
	if err != nil || conflict || len(matches) != 1 {
		return -1, ErrAdminIdentityBindingInvalid
	}
	record := s.resources[adminIdentitiesResource][matches[0]]
	if strings.TrimSpace(record.Status) != adminIdentityBindingEnabledStatus {
		return -1, ErrAdminIdentityBindingInvalid
	}
	return matches[0], nil
}

func (s *Store) adminIdentityBindingMatchesLocked(key AdminIdentityBindingKey) ([]int, bool, error) {
	records, ok := s.resources[adminIdentitiesResource]
	if !ok {
		return nil, false, ErrUnknownResource
	}
	matches := make([]int, 0, 1)
	conflict := false
	for index, record := range records {
		values := record.Values
		if strings.TrimSpace(values[adminIdentityBindingIssuerHashField]) != key.IssuerHash || strings.TrimSpace(values[adminIdentityBindingSubjectHashField]) != key.ProviderSubjectHash {
			continue
		}
		if strings.TrimSpace(values[adminIdentityBindingProviderField]) != key.Provider || strings.TrimSpace(values[adminIdentityBindingKindField]) != key.ProviderKind {
			conflict = true
			continue
		}
		matches = append(matches, index)
	}
	return matches, conflict, nil
}

func normalizeAdminIdentityBindingKey(key AdminIdentityBindingKey) (AdminIdentityBindingKey, bool) {
	key.Provider = strings.TrimSpace(key.Provider)
	key.ProviderKind = strings.TrimSpace(key.ProviderKind)
	key.IssuerHash = strings.TrimSpace(key.IssuerHash)
	key.ProviderSubjectHash = strings.TrimSpace(key.ProviderSubjectHash)
	valid := key.Provider != "" && key.ProviderKind != "" && validAdminIdentityBindingHash(key.IssuerHash) && validAdminIdentityBindingHash(key.ProviderSubjectHash)
	return key, valid
}

func validAdminIdentityBindingHash(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size && hex.EncodeToString(decoded) == value
}
