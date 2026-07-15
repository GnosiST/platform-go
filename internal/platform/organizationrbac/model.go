package organizationrbac

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

type ScopeType string

const (
	ScopePlatform ScopeType = "platform"
	ScopeTenant   ScopeType = "tenant"
	StatusEnabled           = "enabled"

	LifecycleOperationDelete  = "delete"
	LifecycleOperationRestore = "restore"
	LifecycleOperationPurge   = "purge"
	LifecycleOperationDisable = "disable"
)

var (
	ErrInvalid           = errors.New("organization rbac model is invalid")
	ErrNotFound          = errors.New("organization rbac record was not found")
	ErrRevisionConflict  = errors.New("organization rbac revision conflict")
	ErrRolePoolViolation = errors.New("organization role pool violation")
	ErrRepositoryFailed  = errors.New("organization rbac repository failed")
)

type ValidationError struct {
	Field  string
	Reason string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Reason)
}

func (e *ValidationError) Unwrap() error { return ErrInvalid }

type RevisionConflictError struct {
	Expected uint64
	Actual   uint64
}

func (e *RevisionConflictError) Error() string {
	return fmt.Sprintf("organization rbac revision conflict: expected %d, actual %d", e.Expected, e.Actual)
}

func (e *RevisionConflictError) Unwrap() error { return ErrRevisionConflict }

type Organization struct {
	Code       string
	TenantCode string
	Status     string
	Deleted    bool
}

func (organization Organization) Enabled() bool {
	return organization.Status == StatusEnabled && !organization.Deleted
}

type RoleGroup struct {
	Code       string
	Name       string
	ScopeType  ScopeType
	TenantCode string
	Status     string
	Deleted    bool
	Revision   uint64
}

func (group RoleGroup) Enabled() bool {
	return group.Status == StatusEnabled && !group.Deleted
}

type Role struct {
	Code      string
	GroupCode string
	Status    string
	Deleted   bool
}

func (role Role) Enabled() bool {
	return role.Status == StatusEnabled && !role.Deleted
}

type User struct {
	ID          string
	Code        string
	ScopeType   ScopeType
	TenantCode  string
	OrgUnitCode string
	Status      string
	Deleted     bool
}

type OrgUnitRoleGroupBinding struct {
	OrgUnitCode   string
	RoleGroupCode string
}

type RolePoolEntry struct {
	RoleCode      string
	RoleGroupCode string
	RoleGroupName string
	TenantCode    string
	Status        string
}

type OrgUnitRoleGroupSet struct {
	OrgUnitCode    string
	RoleGroupCodes []string
	Revision       uint64
}

type ReplaceOrgUnitRoleGroupsRequest struct {
	OrgUnitCode      string
	RoleGroupCodes   []string
	ExpectedRevision uint64
	ActorID          string
	ChangedAt        time.Time
}

type RoleAssignmentRemediation struct {
	UserCode            string
	RoleCode            string
	Action              string
	ReplacementRoleCode string
}

type RoleAssignmentConflict struct {
	UserCode string
	RoleCode string
}

type OrgUnitRoleGroupImpact struct {
	TenantCode             string
	CurrentRoleGroupCodes  []string
	ProposedRoleGroupCodes []string
	Conflicts              []RoleAssignmentConflict
	AffectedUsers          int
	ExpectedRevision       uint64
}

type UserOrganizationChangeImpact struct {
	UserCode           string
	CurrentOrgUnitCode string
	TargetOrgUnitCode  string
	TargetTenantCode   string
	CurrentRoleCodes   []string
	TargetRoleCodes    []string
	Conflicts          []RoleAssignmentConflict
	ExpectedRevision   uint64
}

type ChangeUserOrganizationRequest struct {
	UserCode         string
	OrgUnitCode      string
	RoleCodes        []string
	ExpectedRevision uint64
	ActorID          string
	ChangedAt        time.Time
}

type RoleStateOrGroupImpact struct {
	RoleCode         string
	Operation        string
	CurrentGroupCode string
	TargetGroupCode  string
	TenantCode       string
	Conflicts        []RoleAssignmentConflict
	AffectedUsers    int
	ExpectedRevision uint64
}

type ChangeRoleRequest struct {
	RoleCode         string
	TargetGroupCode  string
	ExpectedRevision uint64
	ActorID          string
	ChangedAt        time.Time
}

type ResourceLifecycleReference struct {
	Kind string
	Code string
}

type ResourceLifecycleImpact struct {
	Resource         string
	ResourceCode     string
	RecordID         string
	Operation        string
	TenantCode       string
	OrgUnitCode      string
	Status           string
	Deleted          bool
	RetentionElapsed bool
	References       []ResourceLifecycleReference
	Conflicts        []RoleAssignmentConflict
	ReferenceCount   int
	AffectedUsers    int
	ExpectedRevision uint64
}

type ResourceLifecycleRequest struct {
	Resource         string
	ResourceCode     string
	Operation        string
	RetentionDays    int
	PolicyVersion    uint32
	ExpectedRevision uint64
	ActorID          string
	ChangedAt        time.Time
}

type CutoverReport struct {
	Organizations int
	RoleGroups    int
	Roles         int
	Users         int
	Bindings      int
}

func canonicalCodes(values []string) ([]string, error) {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if !validCode(value) {
			return nil, &ValidationError{Field: "codes", Reason: "contains an invalid code"}
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result, nil
}

func validCode(value string) bool {
	return value != "" && value == strings.TrimSpace(value) && len(value) <= 191
}
