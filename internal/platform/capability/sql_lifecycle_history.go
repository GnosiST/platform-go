package capability

import (
	"context"
	"database/sql"
)

const lifecycleHistoryTable = "platform_lifecycle_history"

type SQLLifecycleHistory struct {
	db *sql.DB
}

func NewSQLLifecycleHistory(ctx context.Context, db *sql.DB) (*SQLLifecycleHistory, error) {
	history := &SQLLifecycleHistory{db: db}
	if err := history.ensureSchema(ctx); err != nil {
		return nil, err
	}
	return history, nil
}

func (h *SQLLifecycleHistory) HasMigration(ctx context.Context, capabilityID ID, migrationID string) bool {
	return h.has(ctx, LifecycleKindMigration, capabilityID, migrationID)
}

func (h *SQLLifecycleHistory) RecordMigration(ctx context.Context, record LifecycleRecord) error {
	record.Kind = LifecycleKindMigration
	return h.record(ctx, record)
}

func (h *SQLLifecycleHistory) HasSeed(ctx context.Context, capabilityID ID, seedID string) bool {
	return h.has(ctx, LifecycleKindSeed, capabilityID, seedID)
}

func (h *SQLLifecycleHistory) RecordSeed(ctx context.Context, record LifecycleRecord) error {
	record.Kind = LifecycleKindSeed
	return h.record(ctx, record)
}

func (h *SQLLifecycleHistory) Records(ctx context.Context) []LifecycleRecord {
	rows, err := h.db.QueryContext(ctx, `SELECT kind, capability_id, step_id, description FROM `+lifecycleHistoryTable+` ORDER BY kind, capability_id, step_id`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	records := []LifecycleRecord{}
	for rows.Next() {
		var record LifecycleRecord
		var kind string
		var capabilityID string
		if err := rows.Scan(&kind, &capabilityID, &record.StepID, &record.Description); err != nil {
			return nil
		}
		record.Kind = LifecycleKind(kind)
		record.CapabilityID = ID(capabilityID)
		records = append(records, record)
	}
	return records
}

func (h *SQLLifecycleHistory) ensureSchema(ctx context.Context) error {
	_, err := h.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS `+lifecycleHistoryTable+` (
kind TEXT NOT NULL,
capability_id TEXT NOT NULL,
step_id TEXT NOT NULL,
description TEXT NOT NULL,
PRIMARY KEY (kind, capability_id, step_id)
)`)
	return err
}

func (h *SQLLifecycleHistory) has(ctx context.Context, kind LifecycleKind, capabilityID ID, stepID string) bool {
	rows, err := h.db.QueryContext(ctx, `SELECT 1 FROM `+lifecycleHistoryTable+` WHERE kind = ? AND capability_id = ? AND step_id = ?`, string(kind), string(capabilityID), stepID)
	if err != nil {
		return false
	}
	defer rows.Close()
	return rows.Next()
}

func (h *SQLLifecycleHistory) record(ctx context.Context, record LifecycleRecord) error {
	if h.has(ctx, record.Kind, record.CapabilityID, record.StepID) {
		return nil
	}
	_, err := h.db.ExecContext(ctx, `INSERT INTO `+lifecycleHistoryTable+` (kind, capability_id, step_id, description) VALUES (?, ?, ?, ?)`, string(record.Kind), string(record.CapabilityID), record.StepID, record.Description)
	return err
}
