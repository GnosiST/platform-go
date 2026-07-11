package session

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"platform-go/internal/platform/storage"
)

func TestSQLRepositoryDoesNotPersistRawSessionToken(t *testing.T) {
	now := time.Date(2026, 7, 12, 9, 0, 0, 0, time.UTC)
	db := openSessionTestDB(t)
	repository, err := NewSQLRepository(context.Background(), db)
	if err != nil {
		t.Fatalf("NewSQLRepository() error = %v", err)
	}
	store, err := NewRepositoryBackedStore(Options{TTL: time.Hour, Now: func() time.Time { return now }}, repository)
	if err != nil {
		t.Fatalf("NewRepositoryBackedStore() error = %v", err)
	}
	issued, err := store.Issue("ops")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	var digest string
	if err := db.QueryRow(`SELECT token_digest FROM platform_sessions`).Scan(&digest); err != nil {
		t.Fatalf("query token_digest error = %v", err)
	}
	if digest != DigestToken(issued.Token) {
		t.Fatalf("persisted digest = %q, want %q", digest, DigestToken(issued.Token))
	}
	assertSQLSessionTableHasNoRawTokenColumn(t, db)
}

func TestSQLRepositoryReplacesLegacyRawTokenTable(t *testing.T) {
	ctx := context.Background()
	db := openSessionTestDB(t)
	createLegacySQLSessionTable(t, db, true)
	raw := "raw-session-marker"
	now := time.Date(2026, 7, 12, 9, 0, 0, 0, time.UTC)
	insertLegacySQLSession(t, db, testSession(raw, "ops", now.Add(-time.Hour), now.Add(time.Hour), time.Time{}), nil)

	repository, err := NewSQLRepository(ctx, db)
	if err != nil {
		t.Fatalf("NewSQLRepository() error = %v", err)
	}
	snapshot, err := repository.Load(ctx)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(snapshot.Sessions) != 0 {
		t.Fatalf("legacy sessions = %d, want revoked and removed", len(snapshot.Sessions))
	}
	assertSQLSessionTableHasNoRawTokenColumn(t, db)
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM platform_sessions WHERE token_digest = ?`, raw).Scan(&count); err != nil {
		t.Fatalf("query migrated table error = %v", err)
	}
	if count != 0 {
		t.Fatalf("raw marker rows = %d, want 0", count)
	}
}

func assertSQLSessionTableHasNoRawTokenColumn(t *testing.T, db *sql.DB) {
	t.Helper()
	rows, err := db.Query(`PRAGMA table_info(platform_sessions)`)
	if err != nil {
		t.Fatalf("PRAGMA table_info error = %v", err)
	}
	defer rows.Close()
	columns := []string{}
	for rows.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue sql.NullString
		var primaryKey int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatalf("scan table info error = %v", err)
		}
		columns = append(columns, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("table info rows error = %v", err)
	}
	joined := strings.Join(columns, ",")
	if strings.Contains(joined, "token,") || strings.HasSuffix(joined, ",token") || joined == "token" {
		t.Fatalf("session columns = %v, raw token column must be absent", columns)
	}
	if !strings.Contains(joined, "token_digest") {
		t.Fatalf("session columns = %v, token_digest column is required", columns)
	}
}

func TestSQLRepositoryPersistsIssuedAndRevokedSessions(t *testing.T) {
	now := time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC)
	db := openSessionTestDB(t)
	repository, err := NewSQLRepository(context.Background(), db)
	if err != nil {
		t.Fatalf("NewSQLRepository() error = %v", err)
	}
	store, err := NewRepositoryBackedStore(Options{TTL: time.Hour, Now: func() time.Time { return now }}, repository)
	if err != nil {
		t.Fatalf("NewRepositoryBackedStore() error = %v", err)
	}
	issued, err := store.Issue("ops")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	reloaded, err := NewRepositoryBackedStore(Options{TTL: time.Hour, Now: func() time.Time { return now }}, repository)
	if err != nil {
		t.Fatalf("reload NewRepositoryBackedStore() error = %v", err)
	}
	if resolved, ok := reloaded.Resolve(issued.Token); !ok || resolved.Username != "ops" {
		t.Fatalf("Resolve() after SQL reload = %+v, %v; want ops session", resolved, ok)
	}
	if !reloaded.Revoke(issued.Token) {
		t.Fatalf("Revoke() after SQL reload = false, want true")
	}
	revokedReload, err := NewRepositoryBackedStore(Options{TTL: time.Hour, Now: func() time.Time { return now }}, repository)
	if err != nil {
		t.Fatalf("revoked reload NewRepositoryBackedStore() error = %v", err)
	}
	if _, ok := revokedReload.Resolve(issued.Token); ok {
		t.Fatalf("Resolve() after SQL revoke reload ok = true, want false")
	}
}

func TestSQLRepositoryRecordScopedLifecyclePreservesUnrelatedSessions(t *testing.T) {
	now := time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC)
	repository := openSQLSessionRepository(t)
	assertRecordScopedLifecyclePreservesUnrelatedSessions(t, repository, now)
}

func TestSQLRepositoryRejectsInactiveRenewAndRevoke(t *testing.T) {
	now := time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC)
	repository := openSQLSessionRepository(t)
	assertInactiveSessionsRejectRenewAndRevoke(t, repository, now)
}

func TestNewSQLRepositoryNormalizesCurrentSessionTimes(t *testing.T) {
	ctx := context.Background()
	base := time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC)
	db := openSessionTestDB(t)
	createCurrentSQLSessionTable(t, db, true)
	session := testStoredSession("nullable-active", "admin", base.Add(-1500*time.Millisecond), base.Add(100*time.Millisecond), time.Time{})
	insertCurrentSQLSession(t, db, session, nil)

	if _, err := NewSQLRepository(ctx, db); err != nil {
		t.Fatalf("NewSQLRepository() error = %v", err)
	}
	var issuedAt string
	var expiresAt string
	var revokedAt sql.NullString
	if err := db.QueryRowContext(ctx, `SELECT issued_at, expires_at, revoked_at FROM platform_sessions WHERE token_digest = ?`, session.TokenDigest).Scan(&issuedAt, &expiresAt, &revokedAt); err != nil {
		t.Fatalf("query normalized session error = %v", err)
	}
	if want := fixedSQLSessionTime(session.IssuedAt); issuedAt != want {
		t.Fatalf("normalized issued_at = %q, want %q", issuedAt, want)
	}
	if want := fixedSQLSessionTime(session.ExpiresAt); expiresAt != want {
		t.Fatalf("normalized expires_at = %q, want %q", expiresAt, want)
	}
	if revokedAt.Valid {
		t.Fatalf("normalized revoked_at = %q, want preserved NULL", revokedAt.String)
	}
}

func TestSQLRepositoryCreateUsesSortableFixedWidthTimes(t *testing.T) {
	ctx := context.Background()
	base := time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC)
	db := openSessionTestDB(t)
	repository, err := NewSQLRepository(ctx, db)
	if err != nil {
		t.Fatalf("NewSQLRepository() error = %v", err)
	}
	session := testStoredSession("new-fractional", "admin", base, base.Add(100*time.Millisecond), time.Time{})
	if err := repository.Create(ctx, session); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	var issuedAt string
	var expiresAt string
	if err := db.QueryRowContext(ctx, `SELECT issued_at, expires_at FROM platform_sessions WHERE token_digest = ?`, session.TokenDigest).Scan(&issuedAt, &expiresAt); err != nil {
		t.Fatalf("query created session error = %v", err)
	}
	if want := fixedSQLSessionTime(session.IssuedAt); issuedAt != want {
		t.Fatalf("created issued_at = %q, want %q", issuedAt, want)
	}
	if want := fixedSQLSessionTime(session.ExpiresAt); expiresAt != want {
		t.Fatalf("created expires_at = %q, want %q", expiresAt, want)
	}
	if resolved, ok, err := repository.Resolve(ctx, session.TokenDigest, base); err != nil || !ok || resolved != session {
		t.Fatalf("Resolve(created fractional) = %+v, %v, %v; want active %+v", resolved, ok, err, session)
	}
}

func TestSQLRepositoryCurrentFractionalSecondRecordSemantics(t *testing.T) {
	ctx := context.Background()
	base := time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC)
	db := openSessionTestDB(t)
	createCurrentSQLSessionTable(t, db, true)
	activeResolve := testStoredSession("active-resolve", "admin", base.Add(-time.Hour), base.Add(100*time.Millisecond), time.Time{})
	activeRenew := testStoredSession("active-renew", "admin", base.Add(-time.Hour), base.Add(100*time.Millisecond), time.Time{})
	activeRevoke := testStoredSession("active-revoke", "admin", base.Add(-time.Hour), base.Add(100*time.Millisecond), time.Time{})
	expiredResolve := testStoredSession("expired-resolve", "ops", base.Add(-time.Hour), base, time.Time{})
	expiredRenew := testStoredSession("expired-renew", "ops", base.Add(-time.Hour), base, time.Time{})
	expiredRevoke := testStoredSession("expired-revoke", "ops", base.Add(-time.Hour), base, time.Time{})
	unrelated := testStoredSession("unrelated", "auditor", base.Add(-time.Hour), base.Add(time.Hour), time.Time{})
	for index, session := range []StoredSession{activeResolve, activeRenew, activeRevoke, expiredResolve, expiredRenew, expiredRevoke, unrelated} {
		var revokedAt any = ""
		if index == 0 {
			revokedAt = nil
		}
		insertCurrentSQLSession(t, db, session, revokedAt)
	}

	repository, err := NewSQLRepository(ctx, db)
	if err != nil {
		t.Fatalf("NewSQLRepository() error = %v", err)
	}
	if resolved, ok, err := repository.Resolve(ctx, activeResolve.TokenDigest, base); err != nil || !ok || resolved != activeResolve {
		t.Fatalf("Resolve(active legacy) = %+v, %v, %v; want active %+v", resolved, ok, err, activeResolve)
	}
	expiredNow := base.Add(100 * time.Millisecond)
	if resolved, ok, err := repository.Resolve(ctx, expiredResolve.TokenDigest, expiredNow); err != nil || ok {
		t.Fatalf("Resolve(expired legacy) = %+v, %v, %v; want inactive", resolved, ok, err)
	}

	renewedExpiry := base.Add(2 * time.Hour)
	if renewed, ok, err := repository.Renew(ctx, activeRenew.TokenDigest, base, renewedExpiry); err != nil || !ok || !renewed.ExpiresAt.Equal(renewedExpiry) {
		t.Fatalf("Renew(active legacy) = %+v, %v, %v; want expiry %s", renewed, ok, err, renewedExpiry)
	}
	if renewed, ok, err := repository.Renew(ctx, expiredRenew.TokenDigest, expiredNow, renewedExpiry); err != nil || ok {
		t.Fatalf("Renew(expired legacy) = %+v, %v, %v; want inactive", renewed, ok, err)
	}
	assertSessionUnchanged(t, repository, expiredRenew)

	if revoked, ok, err := repository.Revoke(ctx, activeRevoke.TokenDigest, base); err != nil || !ok || !revoked.RevokedAt.Equal(base) {
		t.Fatalf("Revoke(active legacy) = %+v, %v, %v; want revokedAt %s", revoked, ok, err, base)
	}
	if revoked, ok, err := repository.Revoke(ctx, expiredRevoke.TokenDigest, expiredNow); err != nil || ok {
		t.Fatalf("Revoke(expired legacy) = %+v, %v, %v; want inactive", revoked, ok, err)
	}
	assertSessionUnchanged(t, repository, expiredRevoke)
	assertSessionUnchanged(t, repository, unrelated)
}

func openSessionTestDB(t *testing.T) *sql.DB {
	t.Helper()
	gormDB, err := storage.OpenGORM(storage.Config{
		Driver: "sqlite",
		DSN:    filepath.Join(t.TempDir(), "sessions.db"),
	})
	if err != nil {
		t.Fatalf("OpenGORM() error = %v", err)
	}
	db, err := gormDB.DB()
	if err != nil {
		t.Fatalf("db.DB() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

func openSQLSessionRepository(t *testing.T) *SQLRepository {
	t.Helper()
	repository, err := NewSQLRepository(context.Background(), openSessionTestDB(t))
	if err != nil {
		t.Fatalf("NewSQLRepository() error = %v", err)
	}
	return repository
}

func createLegacySQLSessionTable(t *testing.T, db *sql.DB, nullableRevokedAt bool) {
	t.Helper()
	revokedAtColumn := "revoked_at TEXT NOT NULL"
	if nullableRevokedAt {
		revokedAtColumn = "revoked_at TEXT"
	}
	if _, err := db.Exec(`CREATE TABLE platform_sessions (
token TEXT NOT NULL PRIMARY KEY,
username TEXT NOT NULL,
issued_at TEXT NOT NULL,
expires_at TEXT NOT NULL,
` + revokedAtColumn + `
)`); err != nil {
		t.Fatalf("create legacy schema error = %v", err)
	}
}

func createCurrentSQLSessionTable(t *testing.T, db *sql.DB, nullableRevokedAt bool) {
	t.Helper()
	revokedAtColumn := "revoked_at TEXT NOT NULL"
	if nullableRevokedAt {
		revokedAtColumn = "revoked_at TEXT"
	}
	if _, err := db.Exec(`CREATE TABLE platform_sessions (
	token_digest TEXT NOT NULL PRIMARY KEY,
	username TEXT NOT NULL,
	issued_at TEXT NOT NULL,
	expires_at TEXT NOT NULL,
	` + revokedAtColumn + `
	)`); err != nil {
		t.Fatalf("create current schema error = %v", err)
	}
}

func insertLegacySQLSession(t *testing.T, db *sql.DB, session Session, revokedAt any) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO platform_sessions (token, username, issued_at, expires_at, revoked_at) VALUES (?, ?, ?, ?, ?)`,
		session.Token,
		session.Username,
		legacySQLSessionTime(session.IssuedAt),
		legacySQLSessionTime(session.ExpiresAt),
		revokedAt,
	); err != nil {
		t.Fatalf("insert legacy session %q error = %v", session.Token, err)
	}
}

func insertCurrentSQLSession(t *testing.T, db *sql.DB, session StoredSession, revokedAt any) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO platform_sessions (token_digest, username, issued_at, expires_at, revoked_at) VALUES (?, ?, ?, ?, ?)`,
		session.TokenDigest,
		session.Username,
		legacySQLSessionTime(session.IssuedAt),
		legacySQLSessionTime(session.ExpiresAt),
		revokedAt,
	); err != nil {
		t.Fatalf("insert current session error = %v", err)
	}
}

func legacySQLSessionTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func fixedSQLSessionTime(value time.Time) string {
	return value.UTC().Format("2006-01-02T15:04:05.000000000Z")
}
