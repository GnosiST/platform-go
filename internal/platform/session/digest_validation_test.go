package session

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRepositoriesRejectNonCanonicalSessionDigests(t *testing.T) {
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	malformed := []struct {
		name  string
		value string
	}{
		{name: "raw", value: "raw-session-marker"},
		{name: "short", value: "sha256:v1:" + strings.Repeat("a", 63)},
		{name: "long", value: "sha256:v1:" + strings.Repeat("a", 65)},
		{name: "uppercase-hex", value: "sha256:v1:" + strings.Repeat("A", 64)},
		{name: "uppercase-prefix", value: "SHA256:v1:" + strings.Repeat("a", 64)},
		{name: "unknown-version", value: "sha256:v2:" + strings.Repeat("a", 64)},
		{name: "non-hex", value: "sha256:v1:" + strings.Repeat("g", 64)},
		{name: "trailing-newline", value: "sha256:v1:" + strings.Repeat("a", 64) + "\n"},
	}

	repositories := []struct {
		name       string
		repository recordScopedRepository
	}{
		{name: "file", repository: NewFileRepository(filepath.Join(t.TempDir(), "sessions.json"))},
		{name: "sql", repository: openSQLSessionRepository(t)},
		{name: "gorm", repository: openGormSessionRepository(t)},
	}

	for _, item := range repositories {
		for _, invalid := range malformed {
			t.Run(item.name+"/"+invalid.name, func(t *testing.T) {
				tokenDigest := invalid.value
				session := StoredSession{
					TokenDigest: tokenDigest,
					Username:    "admin",
					IssuedAt:    now,
					ExpiresAt:   now.Add(time.Hour),
				}
				assertDigestErrorDoesNotEcho(t, tokenDigest, item.repository.Create(context.Background(), session))
				if _, _, err := item.repository.Resolve(context.Background(), tokenDigest, now); err == nil {
					t.Fatal("Resolve() error = nil, want malformed digest rejection")
				} else {
					assertDigestErrorDoesNotEcho(t, tokenDigest, err)
				}
				if _, _, err := item.repository.Renew(context.Background(), tokenDigest, now, now.Add(2*time.Hour)); err == nil {
					t.Fatal("Renew() error = nil, want malformed digest rejection")
				} else {
					assertDigestErrorDoesNotEcho(t, tokenDigest, err)
				}
				if _, _, err := item.repository.Revoke(context.Background(), tokenDigest, now); err == nil {
					t.Fatal("Revoke() error = nil, want malformed digest rejection")
				} else {
					assertDigestErrorDoesNotEcho(t, tokenDigest, err)
				}
			})
		}
	}
}

func TestFileRepositoryRejectsMalformedV2Snapshots(t *testing.T) {
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	first := testStoredSession("first", "admin", now, now.Add(time.Hour), time.Time{})
	second := testStoredSession("second", "admin", now, now.Add(time.Hour), time.Time{})
	tests := []struct {
		name    string
		content string
		marker  string
	}{
		{
			name: "raw digest",
			content: fmt.Sprintf(`{"version":2,"sessions":{"raw-session-marker":{"tokenDigest":"raw-session-marker","username":"admin","issuedAt":%q,"expiresAt":%q}}}`,
				now.Format(time.RFC3339), now.Add(time.Hour).Format(time.RFC3339)),
			marker: "raw-session-marker",
		},
		{
			name: "map key mismatch",
			content: fmt.Sprintf(`{"version":2,"sessions":{%q:{"tokenDigest":%q,"username":"admin","issuedAt":%q,"expiresAt":%q}}}`,
				first.TokenDigest, second.TokenDigest, now.Format(time.RFC3339), now.Add(time.Hour).Format(time.RFC3339)),
			marker: first.TokenDigest,
		},
		{name: "unknown version", content: `{"version":99,"sessions":{}}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "sessions.json")
			if err := os.WriteFile(path, []byte(test.content), 0o600); err != nil {
				t.Fatalf("WriteFile() error = %v", err)
			}
			_, err := NewFileRepository(path).Load(context.Background())
			if err == nil {
				t.Fatal("Load() error = nil, want malformed snapshot rejection")
			}
			if test.marker != "" {
				assertDigestErrorDoesNotEcho(t, test.marker, err)
			}
		})
	}
}

func TestSQLRepositoryRejectsMalformedPersistedDigest(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	db := openSessionTestDB(t)
	repository, err := NewSQLRepository(ctx, db)
	if err != nil {
		t.Fatalf("NewSQLRepository() error = %v", err)
	}
	marker := "raw-sql-session-marker"
	if _, err := db.ExecContext(ctx, `INSERT INTO platform_sessions (token_digest, username, issued_at, expires_at, revoked_at) VALUES (?, ?, ?, ?, ?)`, marker, "admin", formatSessionTime(now), formatSessionTime(now.Add(time.Hour)), ""); err != nil {
		t.Fatalf("insert malformed session error = %v", err)
	}
	_, err = repository.Load(ctx)
	assertDigestErrorDoesNotEcho(t, marker, err)
}

func TestGORMRepositoryRejectsMalformedPersistedDigest(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	repository, db := openGormSessionRepositoryAndDB(t)
	marker := "raw-gorm-session-marker"
	if err := db.Exec(`INSERT INTO platform_sessions (token_digest, username, issued_at, expires_at, revoked_at) VALUES (?, ?, ?, ?, NULL)`, marker, "admin", now, now.Add(time.Hour)).Error; err != nil {
		t.Fatalf("insert malformed session error = %v", err)
	}
	_, err := repository.Load(ctx)
	assertDigestErrorDoesNotEcho(t, marker, err)
}

func TestRepositoryBackedStoreRejectsInvalidSnapshots(t *testing.T) {
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	marker := "raw-snapshot-session-marker"
	invalid := StoredSession{TokenDigest: marker, Username: "admin", IssuedAt: now, ExpiresAt: now.Add(time.Hour)}
	repository := &malformedSessionRepository{snapshot: Snapshot{Sessions: map[string]StoredSession{marker: invalid}}}
	if _, err := NewRepositoryBackedStore(Options{TTL: time.Hour, Now: func() time.Time { return now }}, repository); err == nil {
		t.Fatal("NewRepositoryBackedStore() error = nil, want invalid snapshot rejection")
	} else {
		assertDigestErrorDoesNotEcho(t, marker, err)
	}

	valid := testStoredSession("valid", "admin", now, now.Add(time.Hour), time.Time{})
	repository.snapshot = Snapshot{Sessions: map[string]StoredSession{valid.TokenDigest: valid}}
	store, err := NewRepositoryBackedStore(Options{TTL: time.Hour, Now: func() time.Time { return now }}, repository)
	if err != nil {
		t.Fatalf("NewRepositoryBackedStore(valid) error = %v", err)
	}
	repository.snapshot = Snapshot{Sessions: map[string]StoredSession{marker: invalid}}
	if err := store.Reload(); err == nil {
		t.Fatal("Reload() error = nil, want invalid snapshot rejection")
	} else {
		assertDigestErrorDoesNotEcho(t, marker, err)
	}
	if got := store.sessions[valid.TokenDigest]; got != valid {
		t.Fatalf("Reload() replaced last valid snapshot: got %+v, want %+v", got, valid)
	}
}

func TestRepositoryBackedStoreRejectsInvalidOperationResults(t *testing.T) {
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	rawToken := "valid-raw-token"
	valid := testStoredSession(rawToken, "admin", now, now.Add(time.Hour), time.Time{})
	marker := "raw-operation-result-marker"
	invalid := valid
	invalid.TokenDigest = marker

	tests := []struct {
		name string
		run  func(*Store) error
	}{
		{name: "resolve", run: func(store *Store) error {
			_, _, err := store.ResolveContext(context.Background(), rawToken)
			return err
		}},
		{name: "renew", run: func(store *Store) error { _, _, err := store.RenewContext(context.Background(), rawToken); return err }},
		{name: "revoke", run: func(store *Store) error { _, err := store.RevokeContext(context.Background(), rawToken); return err }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &malformedSessionRepository{
				snapshot: Snapshot{Sessions: map[string]StoredSession{valid.TokenDigest: valid}},
				result:   invalid,
			}
			store, err := NewRepositoryBackedStore(Options{TTL: time.Hour, Now: func() time.Time { return now }}, repository)
			if err != nil {
				t.Fatalf("NewRepositoryBackedStore() error = %v", err)
			}
			err = test.run(store)
			assertDigestErrorDoesNotEcho(t, marker, err)
			if got := store.sessions[valid.TokenDigest]; got != valid {
				t.Fatalf("invalid repository result changed cache: got %+v, want %+v", got, valid)
			}
			if _, exists := store.sessions[marker]; exists {
				t.Fatal("invalid repository result was cached")
			}
		})
	}
}

func assertDigestErrorDoesNotEcho(t *testing.T, marker string, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("error = nil, want malformed digest rejection")
	}
	if marker != "" && strings.Contains(err.Error(), marker) {
		t.Fatalf("error %q echoes protected digest input", err)
	}
}

type malformedSessionRepository struct {
	snapshot Snapshot
	result   StoredSession
}

func (r *malformedSessionRepository) Load(context.Context) (Snapshot, error) {
	return Snapshot{Sessions: cloneStoredSessions(r.snapshot.Sessions)}, nil
}

func (r *malformedSessionRepository) Create(context.Context, StoredSession) error {
	return nil
}

func (r *malformedSessionRepository) Resolve(context.Context, string, time.Time) (StoredSession, bool, error) {
	return r.result, true, nil
}

func (r *malformedSessionRepository) Renew(context.Context, string, time.Time, time.Time) (StoredSession, bool, error) {
	return r.result, true, nil
}

func (r *malformedSessionRepository) Revoke(context.Context, string, time.Time) (StoredSession, bool, error) {
	return r.result, true, nil
}
