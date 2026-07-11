package session

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"gorm.io/gorm"
	"platform-go/internal/platform/storage"
)

func TestGORMRepositoryDoesNotPersistRawSessionToken(t *testing.T) {
	now := time.Date(2026, 7, 12, 9, 0, 0, 0, time.UTC)
	repository, db := openGormSessionRepositoryAndDB(t)
	store, err := NewRepositoryBackedStore(Options{TTL: time.Hour, Now: func() time.Time { return now }}, repository)
	if err != nil {
		t.Fatalf("NewRepositoryBackedStore() error = %v", err)
	}
	issued, err := store.Issue("ops")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	var digest string
	if err := db.Raw(`SELECT token_digest FROM platform_sessions`).Scan(&digest).Error; err != nil {
		t.Fatalf("query token_digest error = %v", err)
	}
	if digest != DigestToken(issued.Token) {
		t.Fatalf("persisted digest = %q, want %q", digest, DigestToken(issued.Token))
	}
	if db.Migrator().HasColumn(&gormSessionRecord{}, "token") {
		t.Fatal("platform_sessions still has raw token column")
	}
}

func TestGORMRepositoryReplacesLegacyRawTokenTable(t *testing.T) {
	ctx := context.Background()
	db, err := storage.OpenGORM(storage.Config{Driver: "sqlite", DSN: filepath.Join(t.TempDir(), "sessions.db")})
	if err != nil {
		t.Fatalf("OpenGORM() error = %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	raw := "raw-session-marker"
	if err := db.Exec(`CREATE TABLE platform_sessions (token TEXT PRIMARY KEY, username TEXT NOT NULL, issued_at datetime NOT NULL, expires_at datetime NOT NULL, revoked_at datetime)`).Error; err != nil {
		t.Fatalf("create legacy table error = %v", err)
	}
	if err := db.Exec(`INSERT INTO platform_sessions (token, username, issued_at, expires_at) VALUES (?, ?, ?, ?)`, raw, "ops", time.Now().Add(-time.Hour), time.Now().Add(time.Hour)).Error; err != nil {
		t.Fatalf("insert legacy row error = %v", err)
	}

	repository, err := NewGORMRepository(ctx, db)
	if err != nil {
		t.Fatalf("NewGORMRepository() error = %v", err)
	}
	snapshot, err := repository.Load(ctx)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(snapshot.Sessions) != 0 {
		t.Fatalf("legacy sessions = %d, want revoked and removed", len(snapshot.Sessions))
	}
	if db.Migrator().HasColumn(&gormSessionRecord{}, "token") {
		t.Fatal("platform_sessions still has raw token column")
	}
	if !db.Migrator().HasColumn(&gormSessionRecord{}, "token_digest") {
		t.Fatal("platform_sessions does not have token_digest column")
	}
}

func TestGORMRepositoryPersistsIssuedAndRevokedSessions(t *testing.T) {
	now := time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC)
	repository := openGormSessionRepository(t)
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
		t.Fatalf("Resolve() after GORM reload = %+v, %v; want ops session", resolved, ok)
	}
	if !reloaded.Revoke(issued.Token) {
		t.Fatalf("Revoke() after GORM reload = false, want true")
	}
	revokedReload, err := NewRepositoryBackedStore(Options{TTL: time.Hour, Now: func() time.Time { return now }}, repository)
	if err != nil {
		t.Fatalf("revoked reload NewRepositoryBackedStore() error = %v", err)
	}
	if _, ok := revokedReload.Resolve(issued.Token); ok {
		t.Fatalf("Resolve() after GORM revoke reload ok = true, want false")
	}
}

func TestGORMRepositoryCanLoadEmptySnapshot(t *testing.T) {
	repository := openGormSessionRepository(t)
	snapshot, err := repository.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(snapshot.Sessions) != 0 {
		t.Fatalf("Load() sessions = %d, want 0", len(snapshot.Sessions))
	}
}

func TestIndependentGORMStoresIssueWithoutLosingSessions(t *testing.T) {
	now := time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC)
	path := filepath.Join(t.TempDir(), "sessions.db")
	writer := openSharedGORMStore(t, path, func() time.Time { return now })
	reader := openSharedGORMStore(t, path, func() time.Time { return now })

	first, err := writer.Issue("admin")
	if err != nil {
		t.Fatalf("Issue(admin) error = %v", err)
	}
	second, err := reader.Issue("ops")
	if err != nil {
		t.Fatalf("Issue(ops) error = %v", err)
	}

	verifier := openSharedGORMStore(t, path, func() time.Time { return now })
	for _, token := range []string{first.Token, second.Token} {
		if _, ok := verifier.Resolve(token); !ok {
			t.Fatalf("Resolve(%q) = false, want both independent sessions", token)
		}
	}
}

func TestGORMStoreResolvesPeerIssueWithoutReload(t *testing.T) {
	now := time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC)
	path := filepath.Join(t.TempDir(), "sessions.db")
	writer := openSharedGORMStore(t, path, func() time.Time { return now })
	reader := openSharedGORMStore(t, path, func() time.Time { return now })

	issued, err := writer.Issue("admin")
	if err != nil {
		t.Fatalf("Issue(admin) error = %v", err)
	}
	resolved, ok := reader.Resolve(issued.Token)
	if !ok || resolved.Username != "admin" {
		t.Fatalf("peer Resolve() = %+v, %v; want admin session without Reload", resolved, ok)
	}
}

func TestGORMStoreRenewsPeerIssueWithoutReload(t *testing.T) {
	now := time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC)
	path := filepath.Join(t.TempDir(), "sessions.db")
	writer := openSharedGORMStore(t, path, func() time.Time { return now })
	reader := openSharedGORMStore(t, path, func() time.Time { return now })

	issued, err := writer.Issue("admin")
	if err != nil {
		t.Fatalf("Issue(admin) error = %v", err)
	}
	now = now.Add(30 * time.Minute)
	renewed, ok := reader.Renew(issued.Token)
	if !ok {
		t.Fatalf("peer Renew() ok = false, want true without Reload")
	}
	if want := now.Add(time.Hour); !renewed.ExpiresAt.Equal(want) {
		t.Fatalf("peer Renew() expiresAt = %s, want %s", renewed.ExpiresAt, want)
	}

	verifier := openSharedGORMStore(t, path, func() time.Time { return now })
	resolved, ok := verifier.Resolve(issued.Token)
	if !ok || !resolved.ExpiresAt.Equal(renewed.ExpiresAt) {
		t.Fatalf("Resolve() after peer renewal = %+v, %v; want renewed session", resolved, ok)
	}
}

func TestGORMStoreRevokesPeerIssueWithoutReload(t *testing.T) {
	now := time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC)
	path := filepath.Join(t.TempDir(), "sessions.db")
	writer := openSharedGORMStore(t, path, func() time.Time { return now })
	reader := openSharedGORMStore(t, path, func() time.Time { return now })

	issued, err := writer.Issue("admin")
	if err != nil {
		t.Fatalf("Issue(admin) error = %v", err)
	}
	if !reader.Revoke(issued.Token) {
		t.Fatalf("peer Revoke() = false, want true without Reload")
	}
	if _, ok := writer.Resolve(issued.Token); ok {
		t.Fatalf("writer Resolve() after peer revoke ok = true, want false")
	}
}

func TestGORMRepositoryOperationsNeverExecuteGlobalDelete(t *testing.T) {
	now := time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC)
	path := filepath.Join(t.TempDir(), "sessions.db")
	store, db := openSharedGORMStoreAndDB(t, path, func() time.Time { return now })
	if err := db.Exec(`CREATE TRIGGER block_session_delete BEFORE DELETE ON platform_sessions BEGIN SELECT RAISE(ABORT, 'session delete blocked'); END`).Error; err != nil {
		t.Fatalf("create delete-blocking trigger error = %v", err)
	}

	issued, err := store.Issue("admin")
	if err != nil {
		t.Fatalf("Issue(admin) error = %v", err)
	}
	if _, ok := store.Resolve(issued.Token); !ok {
		t.Fatalf("Resolve() ok = false, want true")
	}
	now = now.Add(30 * time.Minute)
	if _, ok := store.Renew(issued.Token); !ok {
		t.Fatalf("Renew() ok = false, want true")
	}
	if !store.Revoke(issued.Token) {
		t.Fatalf("Revoke() = false, want true")
	}
}

func TestGORMRepositoryRenewTreatsActiveNoOpUpdateAsSuccess(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC)
	repository, db := openGormSessionRepositoryAndDB(t)
	session := testStoredSession("active", "admin", now.Add(-time.Hour), now.Add(time.Hour), time.Time{})
	if err := repository.Create(ctx, session); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	callbackName := "test:force_mysql_noop_update_rows_affected"
	forcedNoOp := false
	if err := db.Callback().Update().After("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement == nil || tx.Statement.Schema == nil || tx.Statement.Schema.Table != sessionsTable {
			return
		}
		forcedNoOp = true
		tx.RowsAffected = 0
	}); err != nil {
		t.Fatalf("register update callback error = %v", err)
	}

	renewed, ok, err := repository.Renew(ctx, session.TokenDigest, now, session.ExpiresAt)
	if !forcedNoOp {
		t.Fatal("test callback did not force RowsAffected to 0")
	}
	if err != nil || !ok || renewed != session {
		t.Fatalf("Renew(active no-op) = %+v, %v, %v; want authoritative active %+v", renewed, ok, err, session)
	}
}

func openGormSessionRepository(t *testing.T) *GORMRepository {
	t.Helper()
	repository, _ := openGormSessionRepositoryAndDB(t)
	return repository
}

func openGormSessionRepositoryAndDB(t *testing.T) (*GORMRepository, *gorm.DB) {
	t.Helper()
	db, err := storage.OpenGORM(storage.Config{
		Driver: "sqlite",
		DSN:    filepath.Join(t.TempDir(), "sessions.db"),
	})
	if err != nil {
		t.Fatalf("OpenGORM() error = %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	repository, err := NewGORMRepository(context.Background(), db)
	if err != nil {
		t.Fatalf("NewGORMRepository() error = %v", err)
	}
	return repository, db
}

func openSharedGORMStore(t *testing.T, path string, now func() time.Time) *Store {
	t.Helper()
	store, _ := openSharedGORMStoreAndDB(t, path, now)
	return store
}

func openSharedGORMStoreAndDB(t *testing.T, path string, now func() time.Time) (*Store, *gorm.DB) {
	t.Helper()
	db, err := storage.OpenGORM(storage.Config{Driver: "sqlite", DSN: path})
	if err != nil {
		t.Fatalf("OpenGORM() error = %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	repository, err := NewGORMRepository(context.Background(), db)
	if err != nil {
		t.Fatalf("NewGORMRepository() error = %v", err)
	}
	store, err := NewRepositoryBackedStore(Options{TTL: time.Hour, Now: now}, repository)
	if err != nil {
		t.Fatalf("NewRepositoryBackedStore() error = %v", err)
	}
	return store, db
}
