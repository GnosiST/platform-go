package tenantplacement

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/GnosiST/platform-go/internal/platform/kernel"
)

const (
	PermissionManage   = "platform:tenant-placement:manage"
	PermissionOverride = "platform:tenant-placement:override"
	maximumOverrideTTL = 15 * time.Minute
)

type Status string

const (
	StatusActive    Status = "active"
	StatusMigrating Status = "migrating"
)

var (
	ErrInvalidPlacement  = errors.New("invalid tenant placement")
	ErrPlacementNotFound = errors.New("tenant placement not found")
	ErrInvalidOverride   = errors.New("invalid tenant placement override")
	ErrUnauthorized      = errors.New("tenant placement authorization failed")
)

var placementIdentifier = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)

type Placement struct {
	TenantCode        string `json:"tenantCode"`
	CapabilityID      string `json:"capabilityId"`
	DatasourceGroupID string `json:"datasourceGroupId"`
	Version           uint64 `json:"version"`
	Status            Status `json:"status"`
}

type Transition struct {
	TenantCode        string `json:"tenantCode"`
	CapabilityID      string `json:"capabilityId"`
	DatasourceGroupID string `json:"datasourceGroupId"`
	Version           uint64 `json:"version"`
	Status            Status `json:"status"`
	Reason            string `json:"reason"`
}

type Override struct {
	TenantCode        string    `json:"tenantCode"`
	CapabilityID      string    `json:"capabilityId"`
	DatasourceGroupID string    `json:"datasourceGroupId"`
	ExpiresAt         time.Time `json:"expiresAt"`
	Reason            string    `json:"reason"`
}

type AuditEntry struct {
	Kind                      string    `json:"kind"`
	Actor                     string    `json:"actor"`
	TenantCode                string    `json:"tenantCode"`
	CapabilityID              string    `json:"capabilityId"`
	PreviousDatasourceGroupID string    `json:"previousDatasourceGroupId,omitempty"`
	DatasourceGroupID         string    `json:"datasourceGroupId"`
	PreviousVersion           uint64    `json:"previousVersion,omitempty"`
	Version                   uint64    `json:"version"`
	Status                    Status    `json:"status"`
	Reason                    string    `json:"reason"`
	OccurredAt                time.Time `json:"occurredAt"`
}

type Authorizer interface {
	Can(kernel.ExecutionContext, string) bool
}

type AuthorizerFunc func(kernel.ExecutionContext, string) bool

func (f AuthorizerFunc) Can(execution kernel.ExecutionContext, permission string) bool {
	return f(execution, permission)
}

type Store struct {
	mu         sync.RWMutex
	placements map[string]Placement
	audit      []AuditEntry
	authorizer Authorizer
	now        func() time.Time
}

func NewStore(initial []Placement, authorizer Authorizer, now func() time.Time) (*Store, error) {
	if authorizer == nil || now == nil {
		return nil, ErrInvalidPlacement
	}
	store := &Store{placements: make(map[string]Placement, len(initial)), authorizer: authorizer, now: now}
	for _, placement := range initial {
		if err := validatePlacement(placement); err != nil {
			return nil, err
		}
		key := placementKey(placement.TenantCode, placement.CapabilityID)
		if _, exists := store.placements[key]; exists {
			return nil, fmt.Errorf("%w: duplicate placement", ErrInvalidPlacement)
		}
		store.placements[key] = placement
	}
	return store, nil
}

func (s *Store) Resolve(execution kernel.ExecutionContext, capabilityID string, override Override) (Placement, error) {
	if err := validateExecution(execution, capabilityID); err != nil {
		return Placement{}, err
	}
	tenantCode := execution.TenantScope.TenantCode
	s.mu.Lock()
	defer s.mu.Unlock()
	placement, found := s.placements[placementKey(tenantCode, capabilityID)]
	if !found {
		return Placement{}, fmt.Errorf("%w: tenant and capability have no placement", ErrPlacementNotFound)
	}
	if isEmptyOverride(override) {
		return placement, nil
	}
	if err := s.validateOverride(execution, placement, override); err != nil {
		return Placement{}, err
	}
	placement.DatasourceGroupID = override.DatasourceGroupID
	s.audit = append(s.audit, AuditEntry{
		Kind: "override", Actor: execution.Actor.Username, TenantCode: placement.TenantCode, CapabilityID: placement.CapabilityID,
		PreviousDatasourceGroupID: s.placements[placementKey(tenantCode, capabilityID)].DatasourceGroupID,
		DatasourceGroupID:         placement.DatasourceGroupID, PreviousVersion: placement.Version, Version: placement.Version,
		Status: placement.Status, Reason: override.Reason, OccurredAt: s.now().UTC(),
	})
	return placement, nil
}

func (s *Store) Transition(execution kernel.ExecutionContext, transition Transition) (Placement, error) {
	if err := validateExecution(execution, transition.CapabilityID); err != nil || execution.TenantScope.TenantCode != transition.TenantCode {
		return Placement{}, ErrUnauthorized
	}
	if s == nil || s.authorizer == nil || !s.authorizer.Can(execution, PermissionManage) {
		return Placement{}, ErrUnauthorized
	}
	if !validIdentifier(transition.TenantCode) || !validIdentifier(transition.CapabilityID) || !validIdentifier(transition.DatasourceGroupID) ||
		(transition.Status != StatusActive && transition.Status != StatusMigrating) || !validReason(transition.Reason) {
		return Placement{}, ErrInvalidPlacement
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := placementKey(transition.TenantCode, transition.CapabilityID)
	previous, found := s.placements[key]
	if !found {
		return Placement{}, ErrPlacementNotFound
	}
	if transition.Version != previous.Version+1 {
		return Placement{}, ErrInvalidPlacement
	}
	updated := Placement{TenantCode: transition.TenantCode, CapabilityID: transition.CapabilityID, DatasourceGroupID: transition.DatasourceGroupID, Version: transition.Version, Status: transition.Status}
	s.placements[key] = updated
	s.audit = append(s.audit, AuditEntry{
		Kind: "transition", Actor: execution.Actor.Username, TenantCode: updated.TenantCode, CapabilityID: updated.CapabilityID,
		PreviousDatasourceGroupID: previous.DatasourceGroupID, DatasourceGroupID: updated.DatasourceGroupID,
		PreviousVersion: previous.Version, Version: updated.Version, Status: updated.Status, Reason: transition.Reason, OccurredAt: s.now().UTC(),
	})
	return updated, nil
}

func (s *Store) Audit() []AuditEntry {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	entries := append([]AuditEntry(nil), s.audit...)
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].OccurredAt.Before(entries[j].OccurredAt) })
	return entries
}

func (s *Store) validateOverride(execution kernel.ExecutionContext, placement Placement, override Override) error {
	if s == nil || s.authorizer == nil || !s.authorizer.Can(execution, PermissionOverride) || override.TenantCode != placement.TenantCode ||
		override.CapabilityID != placement.CapabilityID || !validIdentifier(override.DatasourceGroupID) || !validReason(override.Reason) {
		return ErrInvalidOverride
	}
	now := s.now().UTC()
	if override.ExpiresAt.UTC().Before(now) || override.ExpiresAt.UTC().After(now.Add(maximumOverrideTTL)) {
		return ErrInvalidOverride
	}
	return nil
}

func validatePlacement(placement Placement) error {
	if !validIdentifier(placement.TenantCode) || !validIdentifier(placement.CapabilityID) || !validIdentifier(placement.DatasourceGroupID) || placement.Version == 0 ||
		(placement.Status != StatusActive && placement.Status != StatusMigrating) {
		return ErrInvalidPlacement
	}
	return nil
}

func validateExecution(execution kernel.ExecutionContext, capabilityID string) error {
	if execution.ValidatePermissioned() != nil || execution.TenantScope.PlatformWide || !validIdentifier(execution.TenantScope.TenantCode) || !validIdentifier(capabilityID) {
		return ErrPlacementNotFound
	}
	return nil
}

func validIdentifier(value string) bool {
	return value == strings.TrimSpace(value) && placementIdentifier.MatchString(value)
}

func validReason(value string) bool {
	return value == strings.TrimSpace(value) && value != "" && len(value) <= 256
}

func placementKey(tenantCode, capabilityID string) string {
	return tenantCode + "\x00" + capabilityID
}

func isEmptyOverride(override Override) bool {
	return override == (Override{})
}
