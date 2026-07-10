package session

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

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
			session := testSession(fmt.Sprintf("token-%02d", index), "admin", now, now.Add(time.Hour), time.Time{})
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
	Create(context.Context, Session) error
	Resolve(context.Context, string, time.Time) (Session, bool, error)
	Renew(context.Context, string, time.Time, time.Time) (Session, bool, error)
	Revoke(context.Context, string, time.Time) (Session, bool, error)
}

func assertRecordScopedLifecyclePreservesUnrelatedSessions(t *testing.T, repository recordScopedRepository, now time.Time) {
	t.Helper()
	first := testSession("first", "admin", now, now.Add(time.Hour), time.Time{})
	second := testSession("second", "ops", now, now.Add(2*time.Hour), time.Time{})
	for _, session := range []Session{first, second} {
		if err := repository.Create(context.Background(), session); err != nil {
			t.Fatalf("Create(%s) error = %v", session.Token, err)
		}
	}

	resolved, ok, err := repository.Resolve(context.Background(), first.Token, now)
	if err != nil || !ok || resolved.Username != first.Username {
		t.Fatalf("Resolve(first) = %+v, %v, %v; want active admin session", resolved, ok, err)
	}

	renewedExpiry := now.Add(3 * time.Hour)
	renewed, ok, err := repository.Renew(context.Background(), first.Token, now, renewedExpiry)
	if err != nil || !ok || !renewed.ExpiresAt.Equal(renewedExpiry) {
		t.Fatalf("Renew(first) = %+v, %v, %v; want expiry %s", renewed, ok, err, renewedExpiry)
	}
	assertSessionUnchanged(t, repository, second)

	revoked, ok, err := repository.Revoke(context.Background(), first.Token, now)
	if err != nil || !ok || !revoked.RevokedAt.Equal(now) {
		t.Fatalf("Revoke(first) = %+v, %v, %v; want revokedAt %s", revoked, ok, err, now)
	}
	assertSessionUnchanged(t, repository, second)
}

func assertInactiveSessionsRejectRenewAndRevoke(t *testing.T, repository recordScopedRepository, now time.Time) {
	t.Helper()
	expired := testSession("expired", "admin", now.Add(-2*time.Hour), now.Add(-time.Hour), time.Time{})
	revoked := testSession("revoked", "ops", now.Add(-time.Hour), now.Add(time.Hour), now.Add(-time.Minute))
	for _, session := range []Session{expired, revoked} {
		if err := repository.Create(context.Background(), session); err != nil {
			t.Fatalf("Create(%s) error = %v", session.Token, err)
		}
		if renewed, ok, err := repository.Renew(context.Background(), session.Token, now, now.Add(2*time.Hour)); err != nil || ok {
			t.Fatalf("Renew(%s) = %+v, %v, %v; want inactive false", session.Token, renewed, ok, err)
		}
		if revokedSession, ok, err := repository.Revoke(context.Background(), session.Token, now); err != nil || ok {
			t.Fatalf("Revoke(%s) = %+v, %v, %v; want inactive false", session.Token, revokedSession, ok, err)
		}
	}
}

func assertSessionUnchanged(t *testing.T, repository recordScopedRepository, want Session) {
	t.Helper()
	snapshot, err := repository.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	got, ok := snapshot.Sessions[want.Token]
	if !ok || got != want {
		t.Fatalf("unrelated session = %+v, %v; want unchanged %+v", got, ok, want)
	}
}
