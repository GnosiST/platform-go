package adminresource

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
)

const (
	adminResourceRecordsTable = "platform_admin_resource_records"
	adminResourceStateTable   = "platform_admin_resource_state"
)

type SQLAdminResourceRepository struct {
	db *sql.DB
}

func NewSQLAdminResourceRepository(ctx context.Context, db *sql.DB) (*SQLAdminResourceRepository, error) {
	repository := &SQLAdminResourceRepository{db: db}
	if err := repository.ensureSchema(ctx); err != nil {
		return nil, err
	}
	return repository, nil
}

func (r *SQLAdminResourceRepository) Load(ctx context.Context) (ResourceSnapshot, error) {
	snapshot := ResourceSnapshot{Resources: map[string][]Record{}}
	revision, err := r.loadStateValue(ctx, "revision")
	if err != nil {
		return ResourceSnapshot{}, err
	}
	if revision != "" {
		parsed, parseErr := strconv.ParseUint(revision, 10, 64)
		if parseErr != nil {
			return ResourceSnapshot{}, parseErr
		}
		snapshot.Revision = parsed
	}
	nextID, err := r.loadStateValue(ctx, "next_id")
	if err != nil {
		return ResourceSnapshot{}, err
	}
	if nextID != "" {
		if parsed, parseErr := strconv.Atoi(nextID); parseErr == nil {
			snapshot.NextID = parsed
		}
	}

	recordRows, err := r.db.QueryContext(ctx, `SELECT resource, id, code, name, status, description, updated_at, values_json FROM `+adminResourceRecordsTable+` ORDER BY resource, id`)
	if err != nil {
		return ResourceSnapshot{}, err
	}
	defer recordRows.Close()
	for recordRows.Next() {
		var resource string
		var valuesJSON string
		var record Record
		if err := recordRows.Scan(&resource, &record.ID, &record.Code, &record.Name, &record.Status, &record.Description, &record.UpdatedAt, &valuesJSON); err != nil {
			return ResourceSnapshot{}, err
		}
		if valuesJSON != "" {
			if err := json.Unmarshal([]byte(valuesJSON), &record.Values); err != nil {
				return ResourceSnapshot{}, err
			}
		}
		snapshot.Resources[resource] = append(snapshot.Resources[resource], record)
	}
	return snapshot, nil
}

func (r *SQLAdminResourceRepository) Save(ctx context.Context, snapshot ResourceSnapshot) (uint64, error) {
	nextRevision := snapshot.Revision + 1
	if _, err := r.db.ExecContext(ctx, `DELETE FROM `+adminResourceRecordsTable); err != nil {
		return 0, err
	}
	for _, key := range []string{"next_id", "revision"} {
		if _, err := r.db.ExecContext(ctx, `DELETE FROM `+adminResourceStateTable+` WHERE key = ?`, key); err != nil {
			return 0, err
		}
	}
	if _, err := r.db.ExecContext(ctx, `INSERT INTO `+adminResourceStateTable+` (key, value) VALUES (?, ?)`, "next_id", strconv.Itoa(snapshot.NextID)); err != nil {
		return 0, err
	}
	if _, err := r.db.ExecContext(ctx, `INSERT INTO `+adminResourceStateTable+` (key, value) VALUES (?, ?)`, "revision", strconv.FormatUint(nextRevision, 10)); err != nil {
		return 0, err
	}
	for resource, records := range snapshot.Resources {
		for _, record := range records {
			valuesJSON, err := json.Marshal(cloneValues(record.Values))
			if err != nil {
				return 0, err
			}
			if _, err := r.db.ExecContext(ctx, `INSERT INTO `+adminResourceRecordsTable+` (resource, id, code, name, status, description, updated_at, values_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
				resource,
				record.ID,
				record.Code,
				record.Name,
				record.Status,
				record.Description,
				record.UpdatedAt,
				string(valuesJSON),
			); err != nil {
				return 0, err
			}
		}
	}
	return nextRevision, nil
}

func (r *SQLAdminResourceRepository) loadStateValue(ctx context.Context, key string) (string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT value FROM `+adminResourceStateTable+` WHERE key = ?`, key)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	if !rows.Next() {
		return "", rows.Err()
	}
	var value string
	if err := rows.Scan(&value); err != nil {
		return "", err
	}
	return value, rows.Err()
}

func (r *SQLAdminResourceRepository) ensureSchema(ctx context.Context) error {
	if _, err := r.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS `+adminResourceRecordsTable+` (
resource TEXT NOT NULL,
id TEXT NOT NULL,
code TEXT NOT NULL,
name TEXT NOT NULL,
status TEXT NOT NULL,
description TEXT NOT NULL,
updated_at TEXT NOT NULL,
values_json TEXT NOT NULL,
PRIMARY KEY (resource, id)
)`); err != nil {
		return err
	}
	_, err := r.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS `+adminResourceStateTable+` (
key TEXT NOT NULL PRIMARY KEY,
value TEXT NOT NULL
)`)
	return err
}
