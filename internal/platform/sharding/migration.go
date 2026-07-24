package sharding

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/GnosiST/platform-go/internal/platform/kernel"
)

const PermissionManage = "platform:shard-migration:manage"

type Status string

const (
	StatusPrepared    Status = "prepared"
	StatusRunning     Status = "running"
	StatusCompensated Status = "compensated"
)

var (
	ErrInvalidMigration  = errors.New("invalid shard migration")
	ErrMigrationNotFound = errors.New("shard migration not found")
	ErrUnauthorized      = errors.New("shard migration authorization failed")
)

var identifierPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)

type Checkpoint struct {
	Sequence uint64 `json:"sequence"`
	Cursor   string `json:"cursor"`
}

type Migration struct {
	ID             string     `json:"id"`
	GlobalID       string     `json:"globalId"`
	TenantCode     string     `json:"tenantCode"`
	CapabilityID   string     `json:"capabilityId"`
	SourceShardID  string     `json:"sourceShardId"`
	TargetShardID  string     `json:"targetShardId"`
	RoutingVersion uint64     `json:"routingVersion"`
	Checkpoint     Checkpoint `json:"checkpoint"`
	Status         Status     `json:"status"`
	Reason         string     `json:"reason"`
}

type AuditEntry struct {
	Kind        string     `json:"kind"`
	MigrationID string     `json:"migrationId"`
	Actor       string     `json:"actor"`
	Checkpoint  Checkpoint `json:"checkpoint"`
	Status      Status     `json:"status"`
	Reason      string     `json:"reason"`
	OccurredAt  time.Time  `json:"occurredAt"`
}

type Authorizer interface {
	Can(kernel.ExecutionContext, string) bool
}

type AuthorizerFunc func(kernel.ExecutionContext, string) bool

func (f AuthorizerFunc) Can(execution kernel.ExecutionContext, permission string) bool {
	return f(execution, permission)
}

type Controller struct {
	mu         sync.RWMutex
	migrations map[string]Migration
	audit      []AuditEntry
	authorizer Authorizer
	now        func() time.Time
}

func NewController(authorizer Authorizer, now func() time.Time) (*Controller, error) {
	if authorizer == nil || now == nil {
		return nil, ErrInvalidMigration
	}
	return &Controller{migrations: map[string]Migration{}, authorizer: authorizer, now: now}, nil
}

func (c *Controller) Create(execution kernel.ExecutionContext, migration Migration) (Migration, error) {
	if c == nil || c.authorizer == nil || !authorized(execution, migration.TenantCode, c.authorizer) {
		return Migration{}, ErrUnauthorized
	}
	if err := validateNewMigration(migration); err != nil {
		return Migration{}, err
	}
	globalID, err := newGlobalID()
	if err != nil {
		return Migration{}, err
	}
	migration.GlobalID = globalID
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, found := c.migrations[migration.ID]; found {
		return Migration{}, ErrInvalidMigration
	}
	c.migrations[migration.ID] = migration
	c.audit = append(c.audit, AuditEntry{Kind: "create", MigrationID: migration.ID, Actor: execution.Actor.Username, Checkpoint: migration.Checkpoint, Status: migration.Status, Reason: migration.Reason, OccurredAt: c.now().UTC()})
	return migration, nil
}

func (c *Controller) Checkpoint(execution kernel.ExecutionContext, migrationID string, checkpoint Checkpoint) (Migration, error) {
	if c == nil || !validIdentifier(migrationID) || !validCheckpoint(checkpoint) {
		return Migration{}, ErrInvalidMigration
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	migration, found := c.migrations[migrationID]
	if !found {
		return Migration{}, ErrMigrationNotFound
	}
	if !authorized(execution, migration.TenantCode, c.authorizer) {
		return Migration{}, ErrUnauthorized
	}
	if migration.Status != StatusPrepared && migration.Status != StatusRunning || checkpoint.Sequence <= migration.Checkpoint.Sequence {
		return Migration{}, ErrInvalidMigration
	}
	migration.Checkpoint = checkpoint
	migration.Status = StatusRunning
	c.migrations[migrationID] = migration
	c.audit = append(c.audit, AuditEntry{Kind: "checkpoint", MigrationID: migrationID, Actor: execution.Actor.Username, Checkpoint: checkpoint, Status: migration.Status, Reason: migration.Reason, OccurredAt: c.now().UTC()})
	return migration, nil
}

func (c *Controller) Compensate(execution kernel.ExecutionContext, migrationID, reason string) (Migration, error) {
	if c == nil || !validIdentifier(migrationID) || !validReason(reason) {
		return Migration{}, ErrInvalidMigration
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	migration, found := c.migrations[migrationID]
	if !found {
		return Migration{}, ErrMigrationNotFound
	}
	if !authorized(execution, migration.TenantCode, c.authorizer) {
		return Migration{}, ErrUnauthorized
	}
	if migration.Status != StatusPrepared && migration.Status != StatusRunning {
		return Migration{}, ErrInvalidMigration
	}
	migration.Status = StatusCompensated
	c.migrations[migrationID] = migration
	c.audit = append(c.audit, AuditEntry{Kind: "compensate", MigrationID: migrationID, Actor: execution.Actor.Username, Checkpoint: migration.Checkpoint, Status: migration.Status, Reason: reason, OccurredAt: c.now().UTC()})
	return migration, nil
}

func (c *Controller) Audit() []AuditEntry {
	if c == nil {
		return nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return append([]AuditEntry(nil), c.audit...)
}

func authorized(execution kernel.ExecutionContext, tenantCode string, authorizer Authorizer) bool {
	return authorizer != nil && execution.ValidatePermissioned() == nil && !execution.TenantScope.PlatformWide && execution.TenantScope.TenantCode == tenantCode && validIdentifier(tenantCode) && authorizer.Can(execution, PermissionManage)
}

func validateNewMigration(migration Migration) error {
	if !validIdentifier(migration.ID) || migration.GlobalID != "" || !validIdentifier(migration.TenantCode) || !validIdentifier(migration.CapabilityID) || !validIdentifier(migration.SourceShardID) || !validIdentifier(migration.TargetShardID) || migration.SourceShardID == migration.TargetShardID || migration.RoutingVersion == 0 || migration.Status != StatusPrepared || !validCheckpoint(migration.Checkpoint) || !validReason(migration.Reason) {
		return ErrInvalidMigration
	}
	return nil
}

func validIdentifier(value string) bool {
	return value == strings.TrimSpace(value) && identifierPattern.MatchString(value)
}

func validCheckpoint(checkpoint Checkpoint) bool {
	return checkpoint.Sequence > 0 && checkpoint.Cursor == strings.TrimSpace(checkpoint.Cursor) && checkpoint.Cursor != "" && len(checkpoint.Cursor) <= 256
}

func validReason(value string) bool {
	return value == strings.TrimSpace(value) && value != "" && len(value) <= 256
}

func newGlobalID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate global identifier: %w", err)
	}
	return "gid_" + hex.EncodeToString(bytes), nil
}
