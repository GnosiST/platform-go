package adminresource

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
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

func (r *SQLAdminResourceRepository) Close() error {
	if r == nil || r.db == nil {
		return nil
	}
	return r.db.Close()
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
	if err := recordRows.Err(); err != nil {
		return ResourceSnapshot{}, err
	}
	lifecycleRows, err := r.db.QueryContext(ctx, `SELECT resource, record_id, deleted_at, deleted_by, delete_reason, purge_after, deletion_policy_version FROM `+adminResourceLifecycleTable+` ORDER BY resource, record_id`)
	if err != nil {
		return ResourceSnapshot{}, err
	}
	defer lifecycleRows.Close()
	for lifecycleRows.Next() {
		var resource string
		var recordID string
		var deletedAt string
		var deletedBy string
		var deleteReason string
		var purgeAfter string
		var policyVersion uint32
		if err := lifecycleRows.Scan(&resource, &recordID, &deletedAt, &deletedBy, &deleteReason, &purgeAfter, &policyVersion); err != nil {
			return ResourceSnapshot{}, err
		}
		records := snapshot.Resources[resource]
		index := recordIndexByID(records, recordID)
		if index < 0 {
			return ResourceSnapshot{}, fmt.Errorf("lifecycle metadata references missing record %s/%s", resource, recordID)
		}
		records[index].DeletedAt = deletedAt
		records[index].DeletedBy = deletedBy
		records[index].DeleteReason = deleteReason
		records[index].PurgeAfter = purgeAfter
		records[index].DeletionPolicyVersion = policyVersion
		snapshot.Resources[resource] = records
	}
	if err := lifecycleRows.Err(); err != nil {
		return ResourceSnapshot{}, err
	}
	return snapshot, nil
}

func (r *SQLAdminResourceRepository) Save(ctx context.Context, snapshot ResourceSnapshot) (uint64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	actualRevision, revisionExists, err := loadSQLRevision(ctx, tx)
	if err != nil {
		return 0, err
	}
	if actualRevision != snapshot.Revision {
		return 0, &RevisionConflictError{Expected: snapshot.Revision, Actual: actualRevision}
	}
	nextRevision := snapshot.Revision + 1
	if revisionExists {
		result, err := tx.ExecContext(ctx, `UPDATE `+adminResourceStateTable+` SET value = ? WHERE key = ? AND value = ?`, strconv.FormatUint(nextRevision, 10), "revision", strconv.FormatUint(snapshot.Revision, 10))
		if err != nil {
			return 0, err
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		if rows != 1 {
			latest, _, loadErr := loadSQLRevision(ctx, tx)
			if loadErr != nil {
				return 0, loadErr
			}
			return 0, &RevisionConflictError{Expected: snapshot.Revision, Actual: latest}
		}
	} else {
		if _, err := tx.ExecContext(ctx, `INSERT INTO `+adminResourceStateTable+` (key, value) VALUES (?, ?)`, "revision", strconv.FormatUint(nextRevision, 10)); err != nil {
			return 0, err
		}
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM `+adminResourceRecordsTable); err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM `+adminResourceLifecycleTable); err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM `+adminResourceStateTable+` WHERE key = ?`, "next_id"); err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO `+adminResourceStateTable+` (key, value) VALUES (?, ?)`, "next_id", strconv.Itoa(snapshot.NextID)); err != nil {
		return 0, err
	}
	resources := make([]string, 0, len(snapshot.Resources))
	for resource := range snapshot.Resources {
		resources = append(resources, resource)
	}
	sort.Strings(resources)
	for _, resource := range resources {
		records := snapshot.Resources[resource]
		for _, record := range records {
			valuesJSON, err := json.Marshal(cloneValues(record.Values))
			if err != nil {
				return 0, err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO `+adminResourceRecordsTable+` (resource, id, code, name, status, description, updated_at, values_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
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
			if isLifecycleDeleted(record) {
				if _, err := tx.ExecContext(ctx, `INSERT INTO `+adminResourceLifecycleTable+` (resource, record_id, deleted_at, deleted_by, delete_reason, purge_after, deletion_policy_version) VALUES (?, ?, ?, ?, ?, ?, ?)`,
					resource, record.ID, record.DeletedAt, record.DeletedBy, record.DeleteReason, record.PurgeAfter, record.DeletionPolicyVersion,
				); err != nil {
					return 0, err
				}
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return nextRevision, nil
}

func (r *SQLAdminResourceRepository) CurrentRevision(ctx context.Context) (uint64, error) {
	revision, _, err := loadSQLRevision(ctx, r.db)
	return revision, err
}

type sqlStateQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func loadSQLRevision(ctx context.Context, queryer sqlStateQueryer) (uint64, bool, error) {
	rows, err := queryer.QueryContext(ctx, `SELECT value FROM `+adminResourceStateTable+` WHERE key = ?`, "revision")
	if err != nil {
		return 0, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return 0, false, rows.Err()
	}
	var value string
	if err := rows.Scan(&value); err != nil {
		return 0, false, err
	}
	revision, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, false, err
	}
	return revision, true, rows.Err()
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
	if _, err := r.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS `+adminResourceLifecycleTable+` (
resource TEXT NOT NULL,
record_id TEXT NOT NULL,
deleted_at TEXT NOT NULL,
deleted_by TEXT NOT NULL,
delete_reason TEXT NOT NULL,
purge_after TEXT NOT NULL,
deletion_policy_version INTEGER NOT NULL,
PRIMARY KEY (resource, record_id)
)`); err != nil {
		return err
	}
	_, err := r.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS `+adminResourceStateTable+` (
key TEXT NOT NULL PRIMARY KEY,
value TEXT NOT NULL
)`)
	return err
}
