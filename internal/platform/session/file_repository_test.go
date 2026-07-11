package session

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestFileRepositoryDoesNotPersistRawSessionToken(t *testing.T) {
	now := time.Date(2026, 7, 12, 9, 0, 0, 0, time.UTC)
	path := filepath.Join(t.TempDir(), "sessions.json")
	store, err := NewRepositoryBackedStore(Options{TTL: time.Hour, Now: func() time.Time { return now }}, NewFileRepository(path))
	if err != nil {
		t.Fatalf("NewRepositoryBackedStore() error = %v", err)
	}
	issued, err := store.Issue("ops")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	serialized := string(content)
	if strings.Contains(serialized, issued.Token) {
		t.Fatalf("session snapshot contains raw token %q: %s", issued.Token, serialized)
	}
	if digest := DigestToken(issued.Token); !strings.Contains(serialized, digest) {
		t.Fatalf("session snapshot does not contain digest %q: %s", digest, serialized)
	}
	if !strings.Contains(serialized, `"version": 2`) {
		t.Fatalf("session snapshot = %s, want v2", serialized)
	}
}

func TestFileRepositoryReplacesLegacyV1SnapshotAndRevokesSessions(t *testing.T) {
	now := time.Date(2026, 7, 12, 9, 0, 0, 0, time.UTC)
	path := filepath.Join(t.TempDir(), "sessions.json")
	raw := "raw-session-marker"
	legacy := fmt.Sprintf(`{
  "version": 1,
  "sessions": {
    %q: {
      "token": %q,
      "username": "ops",
      "issuedAt": %q,
      "expiresAt": %q
    }
  }
}`, raw, raw, now.Add(-time.Hour).Format(time.RFC3339), now.Add(time.Hour).Format(time.RFC3339))
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	store, err := NewRepositoryBackedStore(Options{TTL: time.Hour, Now: func() time.Time { return now }}, NewFileRepository(path))
	if err != nil {
		t.Fatalf("NewRepositoryBackedStore() error = %v", err)
	}
	if _, ok := store.Resolve(raw); ok {
		t.Fatal("Resolve(legacy raw token) = true, want revoked during v2 migration")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(content), raw) {
		t.Fatalf("migrated snapshot still contains raw marker: %s", content)
	}
	if !strings.Contains(string(content), `"version": 2`) {
		t.Fatalf("migrated snapshot = %s, want v2", content)
	}
}

func TestFileRepositoryRecordScopedLifecyclePreservesUnrelatedSessions(t *testing.T) {
	now := time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC)
	repository := NewFileRepository(filepath.Join(t.TempDir(), "sessions.json"))
	assertRecordScopedLifecyclePreservesUnrelatedSessions(t, repository, now)
}

func TestFileRepositoryRejectsInactiveRenewAndRevoke(t *testing.T) {
	now := time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC)
	repository := NewFileRepository(filepath.Join(t.TempDir(), "sessions.json"))
	assertInactiveSessionsRejectRenewAndRevoke(t, repository, now)
}

func TestFileRepositorySerializesConcurrentCreates(t *testing.T) {
	now := time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC)
	repository := NewFileRepository(filepath.Join(t.TempDir(), "sessions.json"))
	const count = 16

	var waitGroup sync.WaitGroup
	errors := make(chan error, count)
	for index := 0; index < count; index++ {
		waitGroup.Add(1)
		go func(index int) {
			defer waitGroup.Done()
			session := testStoredSession(fmt.Sprintf("token-%02d", index), "admin", now, now.Add(time.Hour), time.Time{})
			if err := repository.Create(context.Background(), session); err != nil {
				errors <- err
			}
		}(index)
	}
	waitGroup.Wait()
	close(errors)
	for err := range errors {
		t.Fatalf("Create() error = %v", err)
	}

	snapshot, err := repository.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(snapshot.Sessions) != count {
		t.Fatalf("Load() sessions = %d, want %d concurrent creates", len(snapshot.Sessions), count)
	}
}

type recordScopedRepository interface {
	Load(context.Context) (Snapshot, error)
	Create(context.Context, StoredSession) error
	Resolve(context.Context, string, time.Time) (StoredSession, bool, error)
	Renew(context.Context, string, time.Time, time.Time) (StoredSession, bool, error)
	Revoke(context.Context, string, time.Time) (StoredSession, bool, error)
}

func assertRecordScopedLifecyclePreservesUnrelatedSessions(t *testing.T, repository recordScopedRepository, now time.Time) {
	t.Helper()
	first := testStoredSession("first", "admin", now, now.Add(time.Hour), time.Time{})
	second := testStoredSession("second", "ops", now, now.Add(2*time.Hour), time.Time{})
	for _, session := range []StoredSession{first, second} {
		if err := repository.Create(context.Background(), session); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}

	resolved, ok, err := repository.Resolve(context.Background(), first.TokenDigest, now)
	if err != nil || !ok || resolved.Username != first.Username {
		t.Fatalf("Resolve(first) = %+v, %v, %v; want active admin session", resolved, ok, err)
	}

	renewedExpiry := now.Add(3 * time.Hour)
	renewed, ok, err := repository.Renew(context.Background(), first.TokenDigest, now, renewedExpiry)
	if err != nil || !ok || !renewed.ExpiresAt.Equal(renewedExpiry) {
		t.Fatalf("Renew(first) = %+v, %v, %v; want expiry %s", renewed, ok, err, renewedExpiry)
	}
	assertSessionUnchanged(t, repository, second)

	revoked, ok, err := repository.Revoke(context.Background(), first.TokenDigest, now)
	if err != nil || !ok || !revoked.RevokedAt.Equal(now) {
		t.Fatalf("Revoke(first) = %+v, %v, %v; want revokedAt %s", revoked, ok, err, now)
	}
	assertSessionUnchanged(t, repository, second)
}

func assertInactiveSessionsRejectRenewAndRevoke(t *testing.T, repository recordScopedRepository, now time.Time) {
	t.Helper()
	expired := testStoredSession("expired", "admin", now.Add(-2*time.Hour), now.Add(-time.Hour), time.Time{})
	revoked := testStoredSession("revoked", "ops", now.Add(-time.Hour), now.Add(time.Hour), now.Add(-time.Minute))
	for _, session := range []StoredSession{expired, revoked} {
		if err := repository.Create(context.Background(), session); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if renewed, ok, err := repository.Renew(context.Background(), session.TokenDigest, now, now.Add(2*time.Hour)); err != nil || ok {
			t.Fatalf("Renew() = %+v, %v, %v; want inactive false", renewed, ok, err)
		}
		if revokedSession, ok, err := repository.Revoke(context.Background(), session.TokenDigest, now); err != nil || ok {
			t.Fatalf("Revoke() = %+v, %v, %v; want inactive false", revokedSession, ok, err)
		}
	}
}

func assertSessionUnchanged(t *testing.T, repository recordScopedRepository, want StoredSession) {
	t.Helper()
	snapshot, err := repository.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	got, ok := snapshot.Sessions[want.TokenDigest]
	if !ok || got != want {
		t.Fatalf("unrelated session = %+v, %v; want unchanged %+v", got, ok, want)
	}
}
