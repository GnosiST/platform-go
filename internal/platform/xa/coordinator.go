package xa

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"sync"
	"time"
)

var (
	ErrDisabled      = errors.New("xa coordinator is disabled")
	ErrInvalidConfig = errors.New("invalid xa coordinator configuration")
	ErrPrepareFailed = errors.New("xa prepare failed")
	ErrHeuristic     = errors.New("xa heuristic recovery required")
)

var identifierPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)

type Status string

const (
	StatusCommitted  Status = "committed"
	StatusRolledBack Status = "rolled-back"
	StatusHeuristic  Status = "heuristic"
)

type Config struct {
	Enabled bool
	Timeout time.Duration
}

type ResourceManager interface {
	ID() string
	Prepare(context.Context, string) error
	Commit(context.Context, string) error
	Rollback(context.Context, string) error
}

type RecoveryRecord struct {
	TransactionID    string
	Operation        string
	Status           Status
	ResourceManagers []string
	OccurredAt       time.Time
}

type RecoveryLog interface {
	Save(context.Context, RecoveryRecord) error
}

type Coordinator struct {
	config   Config
	managers []ResourceManager
	log      RecoveryLog
}

func NewCoordinator(config Config, managers []ResourceManager, logs ...RecoveryLog) (*Coordinator, error) {
	if len(logs) > 1 {
		return nil, ErrInvalidConfig
	}
	coordinator := &Coordinator{config: config}
	if len(logs) == 1 {
		coordinator.log = logs[0]
	}
	if !config.Enabled {
		return coordinator, nil
	}
	if config.Timeout <= 0 || config.Timeout > time.Minute || coordinator.log == nil || len(managers) < 2 {
		return nil, ErrInvalidConfig
	}
	seen := map[string]struct{}{}
	for _, manager := range managers {
		if manager == nil || !identifierPattern.MatchString(manager.ID()) {
			return nil, ErrInvalidConfig
		}
		if _, found := seen[manager.ID()]; found {
			return nil, ErrInvalidConfig
		}
		seen[manager.ID()] = struct{}{}
		coordinator.managers = append(coordinator.managers, manager)
	}
	return coordinator, nil
}

func (c *Coordinator) Execute(ctx context.Context, operation string) error {
	if c == nil || !c.config.Enabled {
		return ErrDisabled
	}
	if ctx == nil || !identifierPattern.MatchString(operation) {
		return ErrInvalidConfig
	}
	transactionID, err := newTransactionID()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()
	prepared := make([]ResourceManager, 0, len(c.managers))
	for _, manager := range c.managers {
		if err := manager.Prepare(ctx, transactionID); err != nil {
			for index := len(prepared) - 1; index >= 0; index-- {
				_ = prepared[index].Rollback(context.Background(), transactionID)
			}
			_ = c.save(context.Background(), RecoveryRecord{TransactionID: transactionID, Operation: operation, Status: StatusRolledBack, ResourceManagers: managerIDs(c.managers), OccurredAt: time.Now().UTC()})
			return ErrPrepareFailed
		}
		prepared = append(prepared, manager)
	}
	for _, manager := range c.managers {
		if err := manager.Commit(ctx, transactionID); err != nil {
			_ = c.save(context.Background(), RecoveryRecord{TransactionID: transactionID, Operation: operation, Status: StatusHeuristic, ResourceManagers: managerIDs(c.managers), OccurredAt: time.Now().UTC()})
			return ErrHeuristic
		}
	}
	if err := c.save(context.Background(), RecoveryRecord{TransactionID: transactionID, Operation: operation, Status: StatusCommitted, ResourceManagers: managerIDs(c.managers), OccurredAt: time.Now().UTC()}); err != nil {
		return fmt.Errorf("record xa recovery state: %w", err)
	}
	return nil
}

func (c *Coordinator) save(ctx context.Context, record RecoveryRecord) error {
	if c.log == nil {
		return ErrInvalidConfig
	}
	return c.log.Save(ctx, record)
}

type MemoryRecoveryLog struct {
	mu      sync.RWMutex
	records []RecoveryRecord
}

func NewMemoryRecoveryLog() *MemoryRecoveryLog { return &MemoryRecoveryLog{} }

func (l *MemoryRecoveryLog) Save(_ context.Context, record RecoveryRecord) error {
	if l == nil {
		return ErrInvalidConfig
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	record.ResourceManagers = append([]string(nil), record.ResourceManagers...)
	l.records = append(l.records, record)
	return nil
}

func (l *MemoryRecoveryLog) Records() []RecoveryRecord {
	if l == nil {
		return nil
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	records := append([]RecoveryRecord(nil), l.records...)
	for index := range records {
		records[index].ResourceManagers = append([]string(nil), records[index].ResourceManagers...)
	}
	return records
}

func managerIDs(managers []ResourceManager) []string {
	ids := make([]string, 0, len(managers))
	for _, manager := range managers {
		ids = append(ids, manager.ID())
	}
	return ids
}

func newTransactionID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate xa transaction id: %w", err)
	}
	return "xa_" + hex.EncodeToString(bytes), nil
}
