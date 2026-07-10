package capability

import (
	"context"
	"sync"
)

type LifecycleKind string

const (
	LifecycleKindMigration LifecycleKind = "migration"
	LifecycleKindSeed      LifecycleKind = "seed"
)

type LifecycleRecord struct {
	CapabilityID ID
	StepID       string
	Description  string
	Kind         LifecycleKind
}

type LifecycleHistory interface {
	HasMigration(ctx context.Context, capabilityID ID, migrationID string) bool
	RecordMigration(ctx context.Context, record LifecycleRecord) error
	HasSeed(ctx context.Context, capabilityID ID, seedID string) bool
	RecordSeed(ctx context.Context, record LifecycleRecord) error
}

type MemoryLifecycleHistory struct {
	mu      sync.Mutex
	records []LifecycleRecord
}

func NewMemoryLifecycleHistory() *MemoryLifecycleHistory {
	return &MemoryLifecycleHistory{}
}

func (h *MemoryLifecycleHistory) HasMigration(ctx context.Context, capabilityID ID, migrationID string) bool {
	return h.has(LifecycleKindMigration, capabilityID, migrationID)
}

func (h *MemoryLifecycleHistory) RecordMigration(ctx context.Context, record LifecycleRecord) error {
	record.Kind = LifecycleKindMigration
	h.record(record)
	return nil
}

func (h *MemoryLifecycleHistory) HasSeed(ctx context.Context, capabilityID ID, seedID string) bool {
	return h.has(LifecycleKindSeed, capabilityID, seedID)
}

func (h *MemoryLifecycleHistory) RecordSeed(ctx context.Context, record LifecycleRecord) error {
	record.Kind = LifecycleKindSeed
	h.record(record)
	return nil
}

func (h *MemoryLifecycleHistory) Records() []LifecycleRecord {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]LifecycleRecord(nil), h.records...)
}

func (h *MemoryLifecycleHistory) has(kind LifecycleKind, capabilityID ID, stepID string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return hasLifecycleRecord(h.records, kind, capabilityID, stepID)
}

func (h *MemoryLifecycleHistory) record(record LifecycleRecord) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if hasLifecycleRecord(h.records, record.Kind, record.CapabilityID, record.StepID) {
		return
	}
	h.records = append(h.records, record)
}

type RecordedLifecycleExecutor struct {
	history LifecycleHistory
}

func NewRecordedLifecycleExecutor(history LifecycleHistory) RecordedLifecycleExecutor {
	return RecordedLifecycleExecutor{history: history}
}

func (e RecordedLifecycleExecutor) RunMigration(ctx context.Context, exec MigrationExecution) error {
	if e.history != nil && e.history.HasMigration(ctx, exec.CapabilityID, exec.Migration.ID) {
		return nil
	}
	if err := exec.Migration.Up(ctx, exec.Runtime); err != nil {
		return err
	}
	if e.history == nil {
		return nil
	}
	return e.history.RecordMigration(ctx, LifecycleRecord{
		CapabilityID: exec.CapabilityID,
		StepID:       exec.Migration.ID,
		Description:  exec.Migration.Description,
	})
}

func (e RecordedLifecycleExecutor) RunSeed(ctx context.Context, exec SeedExecution) error {
	if e.history != nil && e.history.HasSeed(ctx, exec.CapabilityID, exec.Seed.ID) {
		return nil
	}
	if err := exec.Seed.Run(ctx, exec.Runtime); err != nil {
		return err
	}
	if e.history == nil {
		return nil
	}
	return e.history.RecordSeed(ctx, LifecycleRecord{
		CapabilityID: exec.CapabilityID,
		StepID:       exec.Seed.ID,
		Description:  exec.Seed.Description,
	})
}
