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
	rows, err := r.db.QueryContext(ctx, `SELECT token, username, issued_at, expires_at, revoked_at FROM `+sessionsTable+` ORDER BY token`)
	if err != nil {
		return Snapshot{}, err
	}
	defer rows.Close()
	snapshot := Snapshot{Sessions: map[string]Session{}}
	for rows.Next() {
		session, err := scanSQLSession(rows)
		if err != nil {
			return Snapshot{}, err
		}
		snapshot.Sessions[session.Token] = session
	}
	if err := rows.Err(); err != nil {
		return Snapshot{}, err
	}
	return snapshot, nil
}

func (r *SQLRepository) Create(ctx context.Context, session Session) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO `+sessionsTable+` (token, username, issued_at, expires_at, revoked_at) VALUES (?, ?, ?, ?, ?)`,
		session.Token,
		session.Username,
		formatSessionTime(session.IssuedAt),
		formatSessionTime(session.ExpiresAt),
		formatSessionTime(session.RevokedAt),
	)
	return err
}

func (r *SQLRepository) Resolve(ctx context.Context, token string, now time.Time) (Session, bool, error) {
	return querySQLSession(ctx, r.db, `SELECT token, username, issued_at, expires_at, revoked_at FROM `+sessionsTable+` WHERE token = ? AND (revoked_at IS NULL OR revoked_at = '') AND expires_at > ?`, token, formatSessionTime(now))
}

func (r *SQLRepository) Renew(ctx context.Context, token string, now time.Time, expiresAt time.Time) (Session, bool, error) {
	return r.updateActive(ctx,
		`UPDATE `+sessionsTable+` SET expires_at = ? WHERE token = ? AND (revoked_at IS NULL OR revoked_at = '') AND expires_at > ?`,
		[]any{formatSessionTime(expiresAt), token, formatSessionTime(now)},
		token,
	)
}

func (r *SQLRepository) Revoke(ctx context.Context, token string, now time.Time) (Session, bool, error) {
	return r.updateActive(ctx,
		`UPDATE `+sessionsTable+` SET revoked_at = ? WHERE token = ? AND (revoked_at IS NULL OR revoked_at = '') AND expires_at > ?`,
		[]any{formatSessionTime(now), token, formatSessionTime(now)},
		token,
	)
}

func (r *SQLRepository) updateActive(ctx context.Context, statement string, args []any, token string) (Session, bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Session{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, statement, args...)
	if err != nil {
		return Session{}, false, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return Session{}, false, err
	}
	if rowsAffected == 0 {
		return Session{}, false, nil
	}
	session, ok, err := querySQLSession(ctx, tx, `SELECT token, username, issued_at, expires_at, revoked_at FROM `+sessionsTable+` WHERE token = ?`, token)
	if err != nil {
		return Session{}, false, err
	}
	if !ok {
		return Session{}, false, nil
	}
	if err := tx.Commit(); err != nil {
		return Session{}, false, err
	}
	return session, true, nil
}

func (r *SQLRepository) ensureSchema(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS `+sessionsTable+` (
token TEXT NOT NULL PRIMARY KEY,
username TEXT NOT NULL,
issued_at TEXT NOT NULL,
expires_at TEXT NOT NULL,
revoked_at TEXT NOT NULL
)`)
	return err
}

func (r *SQLRepository) normalizeSessionTimes(ctx context.Context) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.QueryContext(ctx, `SELECT token, issued_at, expires_at, revoked_at FROM `+sessionsTable+` ORDER BY token`)
	if err != nil {
		return err
	}
	type timeRecord struct {
		token     string
		issuedAt  string
		expiresAt string
		revokedAt sql.NullString
	}
	records := []timeRecord{}
	for rows.Next() {
		var record timeRecord
		if err := rows.Scan(&record.token, &record.issuedAt, &record.expiresAt, &record.revokedAt); err != nil {
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
		statement := `UPDATE ` + sessionsTable + ` SET issued_at = ?, expires_at = ?, revoked_at = ? WHERE token = ? AND issued_at = ? AND expires_at = ?`
		args := []any{normalizedIssuedAt, normalizedExpiresAt, normalizedRevokedAt, record.token, record.issuedAt, record.expiresAt}
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

func querySQLSession(ctx context.Context, queryer sqlSessionQueryer, query string, args ...any) (Session, bool, error) {
	session, err := scanSQLSession(queryer.QueryRowContext(ctx, query, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, false, nil
	}
	if err != nil {
		return Session{}, false, err
	}
	return session, true, nil
}

func scanSQLSession(scanner sqlSessionScanner) (Session, error) {
	var session Session
	var issuedAt string
	var expiresAt string
	var revokedAt sql.NullString
	if err := scanner.Scan(&session.Token, &session.Username, &issuedAt, &expiresAt, &revokedAt); err != nil {
		return Session{}, err
	}
	var err error
	if session.IssuedAt, err = parseSessionTime(issuedAt); err != nil {
		return Session{}, err
	}
	if session.ExpiresAt, err = parseSessionTime(expiresAt); err != nil {
		return Session{}, err
	}
	if value := strings.TrimSpace(revokedAt.String); revokedAt.Valid && value != "" {
		if session.RevokedAt, err = parseSessionTime(value); err != nil {
			return Session{}, err
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
