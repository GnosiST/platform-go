package session

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

const sessionsTable = "platform_sessions"

const sqlSessionTimeLayout = "2006-01-02T15:04:05.000000000Z"

type SQLRepository struct {
	db *sql.DB
}

func NewSQLRepository(ctx context.Context, db *sql.DB) (*SQLRepository, error) {
	repository := &SQLRepository{db: db}
	if err := repository.ensureSchema(ctx); err != nil {
		return nil, err
	}
	if err := repository.normalizeSessionTimes(ctx); err != nil {
		return nil, err
	}
	return repository, nil
}

func (r *SQLRepository) Load(ctx context.Context) (Snapshot, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT token_digest, username, issued_at, expires_at, revoked_at FROM `+sessionsTable+` ORDER BY token_digest`)
	if err != nil {
		return Snapshot{}, err
	}
	defer rows.Close()
	snapshot := Snapshot{Sessions: map[string]StoredSession{}}
	for rows.Next() {
		session, err := scanSQLSession(rows)
		if err != nil {
			return Snapshot{}, err
		}
		snapshot.Sessions[session.TokenDigest] = session
	}
	if err := rows.Err(); err != nil {
		return Snapshot{}, err
	}
	return snapshot, nil
}

func (r *SQLRepository) Create(ctx context.Context, session StoredSession) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO `+sessionsTable+` (token_digest, username, issued_at, expires_at, revoked_at) VALUES (?, ?, ?, ?, ?)`,
		session.TokenDigest,
		session.Username,
		formatSessionTime(session.IssuedAt),
		formatSessionTime(session.ExpiresAt),
		formatSessionTime(session.RevokedAt),
	)
	return err
}

func (r *SQLRepository) Resolve(ctx context.Context, tokenDigest string, now time.Time) (StoredSession, bool, error) {
	return querySQLSession(ctx, r.db, `SELECT token_digest, username, issued_at, expires_at, revoked_at FROM `+sessionsTable+` WHERE token_digest = ? AND (revoked_at IS NULL OR revoked_at = '') AND expires_at > ?`, tokenDigest, formatSessionTime(now))
}

func (r *SQLRepository) Renew(ctx context.Context, tokenDigest string, now time.Time, expiresAt time.Time) (StoredSession, bool, error) {
	return r.updateActive(ctx,
		`UPDATE `+sessionsTable+` SET expires_at = ? WHERE token_digest = ? AND (revoked_at IS NULL OR revoked_at = '') AND expires_at > ?`,
		[]any{formatSessionTime(expiresAt), tokenDigest, formatSessionTime(now)},
		tokenDigest,
	)
}

func (r *SQLRepository) Revoke(ctx context.Context, tokenDigest string, now time.Time) (StoredSession, bool, error) {
	return r.updateActive(ctx,
		`UPDATE `+sessionsTable+` SET revoked_at = ? WHERE token_digest = ? AND (revoked_at IS NULL OR revoked_at = '') AND expires_at > ?`,
		[]any{formatSessionTime(now), tokenDigest, formatSessionTime(now)},
		tokenDigest,
	)
}

func (r *SQLRepository) updateActive(ctx context.Context, statement string, args []any, tokenDigest string) (StoredSession, bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return StoredSession{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, statement, args...)
	if err != nil {
		return StoredSession{}, false, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return StoredSession{}, false, err
	}
	if rowsAffected == 0 {
		return StoredSession{}, false, nil
	}
	session, ok, err := querySQLSession(ctx, tx, `SELECT token_digest, username, issued_at, expires_at, revoked_at FROM `+sessionsTable+` WHERE token_digest = ?`, tokenDigest)
	if err != nil {
		return StoredSession{}, false, err
	}
	if !ok {
		return StoredSession{}, false, nil
	}
	if err := tx.Commit(); err != nil {
		return StoredSession{}, false, err
	}
	return session, true, nil
}

func (r *SQLRepository) ensureSchema(ctx context.Context) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, createSQLSessionsTable); err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `SELECT * FROM `+sessionsTable+` WHERE 1 = 0`)
	if err != nil {
		return err
	}
	columnTypes, err := rows.ColumnTypes()
	if closeErr := rows.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	hasDigest := false
	hasRawToken := false
	for _, columnType := range columnTypes {
		switch strings.ToLower(columnType.Name()) {
		case "token_digest":
			hasDigest = true
		case "token":
			hasRawToken = true
		}
	}
	if hasRawToken || !hasDigest {
		if _, err := tx.ExecContext(ctx, `DROP TABLE `+sessionsTable); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, createSQLSessionsTable); err != nil {
			return err
		}
	}
	return tx.Commit()
}

const createSQLSessionsTable = `CREATE TABLE IF NOT EXISTS ` + sessionsTable + ` (
token_digest TEXT NOT NULL PRIMARY KEY,
username TEXT NOT NULL,
issued_at TEXT NOT NULL,
expires_at TEXT NOT NULL,
revoked_at TEXT NOT NULL
)`

func (r *SQLRepository) normalizeSessionTimes(ctx context.Context) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.QueryContext(ctx, `SELECT token_digest, issued_at, expires_at, revoked_at FROM `+sessionsTable+` ORDER BY token_digest`)
	if err != nil {
		return err
	}
	type timeRecord struct {
		tokenDigest string
		issuedAt    string
		expiresAt   string
		revokedAt   sql.NullString
	}
	records := []timeRecord{}
	for rows.Next() {
		var record timeRecord
		if err := rows.Scan(&record.tokenDigest, &record.issuedAt, &record.expiresAt, &record.revokedAt); err != nil {
			_ = rows.Close()
			return err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, record := range records {
		normalizedIssuedAt, err := normalizeSessionTime(record.issuedAt)
		if err != nil {
			return err
		}
		normalizedExpiresAt, err := normalizeSessionTime(record.expiresAt)
		if err != nil {
			return err
		}
		normalizedRevokedAt, err := normalizeNullableSessionTime(record.revokedAt)
		if err != nil {
			return err
		}
		if record.issuedAt == normalizedIssuedAt && record.expiresAt == normalizedExpiresAt && nullableSessionTimeEqual(record.revokedAt, normalizedRevokedAt) {
			continue
		}
		statement := `UPDATE ` + sessionsTable + ` SET issued_at = ?, expires_at = ?, revoked_at = ? WHERE token_digest = ? AND issued_at = ? AND expires_at = ?`
		args := []any{normalizedIssuedAt, normalizedExpiresAt, normalizedRevokedAt, record.tokenDigest, record.issuedAt, record.expiresAt}
		if record.revokedAt.Valid {
			statement += ` AND revoked_at = ?`
			args = append(args, record.revokedAt.String)
		} else {
			statement += ` AND revoked_at IS NULL`
		}
		if _, err := tx.ExecContext(ctx, statement, args...); err != nil {
			return err
		}
	}
	return tx.Commit()
}

type sqlSessionQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type sqlSessionScanner interface {
	Scan(...any) error
}

func querySQLSession(ctx context.Context, queryer sqlSessionQueryer, query string, args ...any) (StoredSession, bool, error) {
	session, err := scanSQLSession(queryer.QueryRowContext(ctx, query, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return StoredSession{}, false, nil
	}
	if err != nil {
		return StoredSession{}, false, err
	}
	return session, true, nil
}

func scanSQLSession(scanner sqlSessionScanner) (StoredSession, error) {
	var session StoredSession
	var issuedAt string
	var expiresAt string
	var revokedAt sql.NullString
	if err := scanner.Scan(&session.TokenDigest, &session.Username, &issuedAt, &expiresAt, &revokedAt); err != nil {
		return StoredSession{}, err
	}
	var err error
	if session.IssuedAt, err = parseSessionTime(issuedAt); err != nil {
		return StoredSession{}, err
	}
	if session.ExpiresAt, err = parseSessionTime(expiresAt); err != nil {
		return StoredSession{}, err
	}
	if value := strings.TrimSpace(revokedAt.String); revokedAt.Valid && value != "" {
		if session.RevokedAt, err = parseSessionTime(value); err != nil {
			return StoredSession{}, err
		}
	}
	return session, nil
}

func parseSessionTime(value string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, value)
}

func formatSessionTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(sqlSessionTimeLayout)
}

func normalizeSessionTime(value string) (string, error) {
	parsed, err := parseSessionTime(value)
	if err != nil {
		return "", err
	}
	return formatSessionTime(parsed), nil
}

func normalizeNullableSessionTime(value sql.NullString) (any, error) {
	if !value.Valid {
		return nil, nil
	}
	trimmed := strings.TrimSpace(value.String)
	if trimmed == "" {
		return "", nil
	}
	return normalizeSessionTime(trimmed)
}

func nullableSessionTimeEqual(current sql.NullString, normalized any) bool {
	if !current.Valid {
		return normalized == nil
	}
	normalizedString, ok := normalized.(string)
	return ok && current.String == normalizedString
}
