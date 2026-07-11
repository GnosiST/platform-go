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

	AdminIdentityBindingAuditOutcomeBound    = "bound"
	AdminIdentityBindingAuditOutcomeConflict = "conflict"
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

type AdminIdentityBindingProvisionResult struct {
	RecordID         string
	PlatformUsername string
	Created          bool
}

type AdminIdentityBindingAuditInput struct {
	BindingRecordID string
	Provider        string
	Username        string
	Outcome         string
	Now             time.Time
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

func (s *Store) ProvisionAdminIdentityBinding(ctx context.Context, input AdminIdentityBindingProvisionInput) (AdminIdentityBindingProvisionResult, error) {
	if s == nil {
		return AdminIdentityBindingProvisionResult{}, ErrAdminIdentityBindingInvalid
	}
	if err := ctx.Err(); err != nil {
		return AdminIdentityBindingProvisionResult{}, err
	}
	key, ok := normalizeAdminIdentityBindingKey(input.Key)
	username := strings.TrimSpace(input.PlatformUsername)
	if !ok || username == "" {
		return AdminIdentityBindingProvisionResult{}, ErrAdminIdentityBindingInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for attempt := 0; attempt < adminIdentityMutationMaxAttempts; attempt++ {
		if err := s.reloadContextLocked(ctx); err != nil {
			if errors.Is(err, ErrRevisionConflict) {
				continue
			}
			return AdminIdentityBindingProvisionResult{}, err
		}
		matches, conflict, err := s.adminIdentityBindingMatchesLocked(key)
		if err != nil || conflict || len(matches) > 1 {
			return AdminIdentityBindingProvisionResult{}, ErrAdminIdentityBindingInvalid
		}
		if len(matches) == 1 {
			record := s.resources[adminIdentitiesResource][matches[0]]
			if strings.TrimSpace(record.Status) == adminIdentityBindingEnabledStatus && strings.TrimSpace(record.Values[adminIdentityBindingUsernameField]) == username {
				if _, err := s.validateAdminPrincipalLocked(username); err != nil {
					return AdminIdentityBindingProvisionResult{}, ErrAdminIdentityBindingInvalid
				}
				return AdminIdentityBindingProvisionResult{RecordID: record.ID, PlatformUsername: username}, nil
			}
			return AdminIdentityBindingProvisionResult{RecordID: record.ID}, ErrAdminIdentityBindingInvalid
		}
		if _, err := s.validateAdminPrincipalLocked(username); err != nil {
			return AdminIdentityBindingProvisionResult{}, ErrAdminIdentityBindingInvalid
		}
		previous := s.snapshotLocked()
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
		record, err := s.recordFromInputWithOrigin(adminIdentitiesResource, "", WriteInput{
			Code: key.Provider + "-" + key.ProviderSubjectHash[:12], Name: adminIdentityBindingManagedName, Status: adminIdentityBindingEnabledStatus, Values: values,
		}, WriteOriginInternal)
		if err != nil {
			return AdminIdentityBindingProvisionResult{}, err
		}
		s.nextID++
		record.ID = fmt.Sprintf("%s-%d", adminIdentitiesResource, s.nextID)
		s.resources[adminIdentitiesResource] = append(s.resources[adminIdentitiesResource], record)
		if err := s.persistContextLocked(ctx); err != nil {
			s.restoreSnapshotLocked(previous)
			if errors.Is(err, ErrRevisionConflict) {
				continue
			}
			return AdminIdentityBindingProvisionResult{}, err
		}
		return AdminIdentityBindingProvisionResult{RecordID: record.ID, PlatformUsername: username, Created: true}, nil
	}
	return AdminIdentityBindingProvisionResult{}, ErrRevisionConflict
}

func (s *Store) EnsureAdminIdentityBindingAudit(ctx context.Context, input AdminIdentityBindingAuditInput) (Record, error) {
	if s == nil {
		return Record{}, ErrInvalidRecord
	}
	if err := ctx.Err(); err != nil {
		return Record{}, err
	}
	input.BindingRecordID = strings.TrimSpace(input.BindingRecordID)
	input.Provider = strings.TrimSpace(input.Provider)
	input.Username = strings.TrimSpace(input.Username)
	input.Outcome = strings.TrimSpace(input.Outcome)
	if input.BindingRecordID == "" || input.Provider == "" || (input.Outcome != AdminIdentityBindingAuditOutcomeBound && input.Outcome != AdminIdentityBindingAuditOutcomeConflict) {
		return Record{}, ErrInvalidRecord
	}
	if input.Outcome == AdminIdentityBindingAuditOutcomeBound && input.Username == "" {
		return Record{}, ErrInvalidRecord
	}
	if input.Outcome == AdminIdentityBindingAuditOutcomeConflict && input.Username != "" {
		return Record{}, ErrInvalidRecord
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for attempt := 0; attempt < adminIdentityMutationMaxAttempts; attempt++ {
		if err := s.reloadContextLocked(ctx); err != nil {
			if errors.Is(err, ErrRevisionConflict) {
				continue
			}
			return Record{}, err
		}
		previous := s.snapshotLocked()
		binding, ok := findRecordByID(s.resources[adminIdentitiesResource], input.BindingRecordID)
		if !ok || strings.TrimSpace(binding.Status) != adminIdentityBindingEnabledStatus || strings.TrimSpace(binding.Values[adminIdentityBindingProviderField]) != input.Provider {
			return Record{}, ErrInvalidRecord
		}
		if input.Outcome == AdminIdentityBindingAuditOutcomeBound && strings.TrimSpace(binding.Values[adminIdentityBindingUsernameField]) != input.Username {
			return Record{}, ErrInvalidRecord
		}
		code := "admin_identity.bind." + input.Outcome + "." + input.BindingRecordID
		values := map[string]string{
			"action":    "admin_identity.bind",
			"resource":  adminIdentitiesResource,
			"targetId":  input.BindingRecordID,
			"provider":  input.Provider,
			"outcome":   input.Outcome,
			"createdAt": resolveAdminIdentityAuditNow(input.Now, s.now).Format(time.RFC3339),
		}
		name := "Admin OIDC Binding Conflict"
		if input.Outcome == AdminIdentityBindingAuditOutcomeBound {
			values["actor"] = input.Username
			name = "Admin OIDC Binding Provisioned"
		}
		audits, auditResourceAvailable := s.resources["audit-logs"]
		if !auditResourceAvailable {
			return Record{}, ErrUnknownResource
		}
		if existing, ok, err := matchingAdminIdentityAudit(audits, code, values); err != nil {
			return Record{}, err
		} else if ok {
			return cloneRecord(existing), nil
		}
		record, err := s.recordFromInputWithOrigin("audit-logs", "", WriteInput{
			Code: code, Name: name, Status: "recorded", Description: "Admin OIDC identity binding provisioning event.", Values: values,
		}, WriteOriginInternal)
		if err != nil {
			return Record{}, err
		}
		s.nextID++
		record.ID = fmt.Sprintf("audit-logs-%d", s.nextID)
		s.resources["audit-logs"] = append(s.resources["audit-logs"], record)
		if err := s.persistContextLocked(ctx); err != nil {
			s.restoreSnapshotLocked(previous)
			if errors.Is(err, ErrRevisionConflict) {
				continue
			}
			return Record{}, err
		}
		return cloneRecord(record), nil
	}
	return Record{}, ErrRevisionConflict
}

func findRecordByID(records []Record, id string) (Record, bool) {
	for _, record := range records {
		if record.ID == id {
			return record, true
		}
	}
	return Record{}, false
}

func resolveAdminIdentityAuditNow(value time.Time, now func() time.Time) time.Time {
	if !value.IsZero() {
		return value.UTC()
	}
	if now != nil {
		return now().UTC()
	}
	return time.Now().UTC()
}

func matchingAdminIdentityAudit(records []Record, code string, expected map[string]string) (Record, bool, error) {
	var matched *Record
	for index := range records {
		if records[index].Code != code {
			continue
		}
		if matched != nil {
			return Record{}, false, ErrInvalidRecord
		}
		matched = &records[index]
	}
	if matched == nil {
		return Record{}, false, nil
	}
	for _, key := range []string{"action", "resource", "targetId", "provider", "outcome", "actor"} {
		if strings.TrimSpace(matched.Values[key]) != strings.TrimSpace(expected[key]) {
			return Record{}, false, ErrInvalidRecord
		}
	}
	for key := range matched.Values {
		normalized := strings.ToLower(strings.TrimSpace(key))
		if strings.Contains(normalized, "issuer") || strings.Contains(normalized, "subject") || strings.Contains(normalized, "hash") {
			return Record{}, false, ErrInvalidRecord
		}
	}
	return *matched, true, nil
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
