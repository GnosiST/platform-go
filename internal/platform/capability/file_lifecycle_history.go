package capability

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

type FileLifecycleHistory struct {
	mu      sync.Mutex
	path    string
	records []LifecycleRecord
}

func NewFileLifecycleHistory(path string) (*FileLifecycleHistory, error) {
	history := &FileLifecycleHistory{path: path}
	if err := history.load(); err != nil {
		return nil, err
	}
	return history, nil
}

func (h *FileLifecycleHistory) HasMigration(ctx context.Context, capabilityID ID, migrationID string) bool {
	return h.has(LifecycleKindMigration, capabilityID, migrationID)
}

func (h *FileLifecycleHistory) RecordMigration(ctx context.Context, record LifecycleRecord) error {
	record.Kind = LifecycleKindMigration
	return h.record(record)
}

func (h *FileLifecycleHistory) HasSeed(ctx context.Context, capabilityID ID, seedID string) bool {
	return h.has(LifecycleKindSeed, capabilityID, seedID)
}

func (h *FileLifecycleHistory) RecordSeed(ctx context.Context, record LifecycleRecord) error {
	record.Kind = LifecycleKindSeed
	return h.record(record)
}

func (h *FileLifecycleHistory) Records() []LifecycleRecord {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]LifecycleRecord(nil), h.records...)
}

func (h *FileLifecycleHistory) load() error {
	if h.path == "" {
		return errors.New("lifecycle history path is required")
	}
	data, err := os.ReadFile(h.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, &h.records)
}

func (h *FileLifecycleHistory) has(kind LifecycleKind, capabilityID ID, stepID string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return hasLifecycleRecord(h.records, kind, capabilityID, stepID)
}

func (h *FileLifecycleHistory) record(record LifecycleRecord) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if hasLifecycleRecord(h.records, record.Kind, record.CapabilityID, record.StepID) {
		return nil
	}
	h.records = append(h.records, record)
	return h.flushLocked()
}

func (h *FileLifecycleHistory) flushLocked() error {
	if err := os.MkdirAll(filepath.Dir(h.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(h.records, "", "  ")
	if err != nil {
		return err
	}
	tmpPath := h.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmpPath, h.path)
}

func hasLifecycleRecord(records []LifecycleRecord, kind LifecycleKind, capabilityID ID, stepID string) bool {
	for _, record := range records {
		if record.Kind == kind && record.CapabilityID == capabilityID && record.StepID == stepID {
			return true
		}
	}
	return false
}
