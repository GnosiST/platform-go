package session

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestStoreIssuesOpaqueSessionToken(t *testing.T) {
	now := time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC)
	store := NewStore(Options{TTL: time.Hour, Now: func() time.Time { return now }})

	issued, err := store.Issue("ops")

	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if issued.Token == "" || strings.Contains(issued.Token, "ops") || strings.HasPrefix(issued.Token, "demo.") {
		t.Fatalf("Issue() token = %q, want opaque token", issued.Token)
	}
	if issued.Username != "ops" {
		t.Fatalf("Issue() username = %q, want ops", issued.Username)
	}
	if !issued.ExpiresAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("Issue() expiresAt = %s, want %s", issued.ExpiresAt, now.Add(time.Hour))
	}
}

func TestStoreRejectsExpiredAndRevokedSessions(t *testing.T) {
	now := time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC)
	store := NewStore(Options{TTL: time.Hour, Now: func() time.Time { return now }})
	issued, err := store.Issue("ops")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	now = now.Add(2 * time.Hour)
	if _, ok := store.Resolve(issued.Token); ok {
		t.Fatalf("Resolve() ok = true for expired session")
	}

	now = time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC)
	issued, err = store.Issue("admin")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if !store.Revoke(issued.Token) {
		t.Fatalf("Revoke() = false, want true")
	}
	if _, ok := store.Resolve(issued.Token); ok {
		t.Fatalf("Resolve() ok = true for revoked session")
	}
}

func TestStoreRenewsActiveSession(t *testing.T) {
	now := time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC)
	store := NewStore(Options{TTL: time.Hour, Now: func() time.Time { return now }})
	issued, err := store.Issue("ops")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	now = now.Add(45 * time.Minute)
	renewed, ok := store.Renew(issued.Token)
	if !ok {
		t.Fatalf("Renew() ok = false, want true")
	}
	if renewed.Token != issued.Token || renewed.Username != issued.Username {
		t.Fatalf("Renew() = %+v, want same token and username as %+v", renewed, issued)
	}
	if !renewed.IssuedAt.Equal(issued.IssuedAt) {
		t.Fatalf("Renew() issuedAt = %s, want original %s", renewed.IssuedAt, issued.IssuedAt)
	}
	wantExpiresAt := now.Add(time.Hour)
	if !renewed.ExpiresAt.Equal(wantExpiresAt) {
		t.Fatalf("Renew() expiresAt = %s, want %s", renewed.ExpiresAt, wantExpiresAt)
	}

	now = issued.ExpiresAt.Add(time.Minute)
	if resolved, ok := store.Resolve(issued.Token); !ok || !resolved.ExpiresAt.Equal(wantExpiresAt) {
		t.Fatalf("Resolve() after original expiry = %+v, %v; want renewed active session", resolved, ok)
	}
}

func TestStoreRejectsExpiredAndRevokedSessionRenewal(t *testing.T) {
	now := time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC)
	store := NewStore(Options{TTL: time.Hour, Now: func() time.Time { return now }})
	expired, err := store.Issue("ops")
	if err != nil {
		t.Fatalf("Issue(expired) error = %v", err)
	}
	now = now.Add(2 * time.Hour)
	if renewed, ok := store.Renew(expired.Token); ok {
		t.Fatalf("Renew(expired) = %+v, true; want false", renewed)
	}

	now = time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC)
	revoked, err := store.Issue("admin")
	if err != nil {
		t.Fatalf("Issue(revoked) error = %v", err)
	}
	if !store.Revoke(revoked.Token) {
		t.Fatalf("Revoke() = false, want true")
	}
	if renewed, ok := store.Renew(revoked.Token); ok {
		t.Fatalf("Renew(revoked) = %+v, true; want false", renewed)
	}
}

func TestRepositoryBackedStoreSeesExternalSessionChangesWithoutReload(t *testing.T) {
	now := time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC)
	path := filepath.Join(t.TempDir(), "sessions.json")
	repository := NewFileRepository(path)
	writer, err := NewRepositoryBackedStore(Options{TTL: time.Hour, Now: func() time.Time { return now }}, repository)
	if err != nil {
		t.Fatalf("NewRepositoryBackedStore(writer) error = %v", err)
	}
	reader, err := NewRepositoryBackedStore(Options{TTL: time.Hour, Now: func() time.Time { return now }}, repository)
	if err != nil {
		t.Fatalf("NewRepositoryBackedStore(reader) error = %v", err)
	}

	issued, err := writer.Issue("ops")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if resolved, ok := reader.Resolve(issued.Token); !ok || resolved.Username != "ops" {
		t.Fatalf("reader Resolve() without reload = %+v, %v; want ops session", resolved, ok)
	}

	if !writer.Revoke(issued.Token) {
		t.Fatalf("writer Revoke() = false, want true")
	}
	if _, ok := reader.Resolve(issued.Token); ok {
		t.Fatalf("reader Resolve() after peer revoke ok = true, want false without reload")
	}
}

func TestFileRepositoryPersistsIssuedAndRevokedSessions(t *testing.T) {
	now := time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC)
	path := filepath.Join(t.TempDir(), "sessions.json")
	repository := NewFileRepository(path)
	store, err := NewRepositoryBackedStore(Options{TTL: time.Hour, Now: func() time.Time { return now }}, repository)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	issued, err := store.Issue("ops")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	reloaded, err := NewRepositoryBackedStore(Options{TTL: time.Hour, Now: func() time.Time { return now }}, NewFileRepository(path))
	if err != nil {
		t.Fatalf("NewStore() after issue error = %v", err)
	}
	if resolved, ok := reloaded.Resolve(issued.Token); !ok || resolved.Username != "ops" {
		t.Fatalf("Resolve() after reload = %+v, %v; want ops session", resolved, ok)
	}
	if !reloaded.Revoke(issued.Token) {
		t.Fatalf("Revoke() after reload = false, want true")
	}
	revokedReload, err := NewRepositoryBackedStore(Options{TTL: time.Hour, Now: func() time.Time { return now }}, NewFileRepository(path))
	if err != nil {
		t.Fatalf("NewStore() after revoke error = %v", err)
	}
	if _, ok := revokedReload.Resolve(issued.Token); ok {
		t.Fatalf("Resolve() after revoked reload ok = true, want false")
	}
}

func TestStoreDoesNotAddLocalSessionWhenRepositoryCreateFails(t *testing.T) {
	now := time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC)
	wantErr := errors.New("create failed")
	repository := &failingSessionRepository{createErr: wantErr}
	store, err := NewRepositoryBackedStore(Options{TTL: time.Hour, Now: func() time.Time { return now }}, repository)
	if err != nil {
		t.Fatalf("NewRepositoryBackedStore() error = %v", err)
	}

	if _, err := store.Issue("admin"); !errors.Is(err, wantErr) {
		t.Fatalf("Issue() error = %v, want %v", err, wantErr)
	}
	if len(store.sessions) != 0 {
		t.Fatalf("local sessions = %d after Create error, want 0", len(store.sessions))
	}
}

func TestStoreDoesNotReplaceLocalSessionWhenRepositoryRenewFails(t *testing.T) {
	now := time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC)
	wantErr := errors.New("renew failed")
	existing := testSession("active", "admin", now, now.Add(time.Hour), time.Time{})
	repository := &failingSessionRepository{
		snapshot: Snapshot{Sessions: map[string]Session{existing.Token: existing}},
		renewErr: wantErr,
	}
	store, err := NewRepositoryBackedStore(Options{TTL: time.Hour, Now: func() time.Time { return now }}, repository)
	if err != nil {
		t.Fatalf("NewRepositoryBackedStore() error = %v", err)
	}

	if _, ok, err := store.RenewContext(context.Background(), existing.Token); ok || !errors.Is(err, wantErr) {
		t.Fatalf("RenewContext() = _, %v, %v; want false, %v", ok, err, wantErr)
	}
	if got := store.sessions[existing.Token]; !got.ExpiresAt.Equal(existing.ExpiresAt) {
		t.Fatalf("local expiresAt = %s after Renew error, want %s", got.ExpiresAt, existing.ExpiresAt)
	}
}

func TestStoreDoesNotReplaceLocalSessionWhenRepositoryRevokeFails(t *testing.T) {
	now := time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC)
	wantErr := errors.New("revoke failed")
	existing := testSession("active", "admin", now, now.Add(time.Hour), time.Time{})
	repository := &failingSessionRepository{
		snapshot:  Snapshot{Sessions: map[string]Session{existing.Token: existing}},
		revokeErr: wantErr,
	}
	store, err := NewRepositoryBackedStore(Options{TTL: time.Hour, Now: func() time.Time { return now }}, repository)
	if err != nil {
		t.Fatalf("NewRepositoryBackedStore() error = %v", err)
	}

	if ok, err := store.RevokeContext(context.Background(), existing.Token); ok || !errors.Is(err, wantErr) {
		t.Fatalf("RevokeContext() = %v, %v; want false, %v", ok, err, wantErr)
	}
	if got := store.sessions[existing.Token]; !got.RevokedAt.IsZero() {
		t.Fatalf("local revokedAt = %s after Revoke error, want zero", got.RevokedAt)
	}
}

func TestStoreCompatibilityWrappersFoldRepositoryErrors(t *testing.T) {
	now := time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC)
	repository := &failingSessionRepository{
		resolveErr: errors.New("resolve failed"),
		renewErr:   errors.New("renew failed"),
		revokeErr:  errors.New("revoke failed"),
	}
	store, err := NewRepositoryBackedStore(Options{TTL: time.Hour, Now: func() time.Time { return now }}, repository)
	if err != nil {
		t.Fatalf("NewRepositoryBackedStore() error = %v", err)
	}

	if _, ok := store.Resolve("token"); ok {
		t.Fatalf("Resolve() ok = true on repository error, want false")
	}
	if _, ok := store.Renew("token"); ok {
		t.Fatalf("Renew() ok = true on repository error, want false")
	}
	if store.Revoke("token") {
		t.Fatalf("Revoke() = true on repository error, want false")
	}
}

func TestStoreReloadCannotOverwriteConcurrentSuccessfulIssue(t *testing.T) {
	now := time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC)
	repository := newBlockingReloadRepository()
	store, err := NewRepositoryBackedStore(Options{TTL: time.Hour, Now: func() time.Time { return now }}, repository)
	if err != nil {
		t.Fatalf("NewRepositoryBackedStore() error = %v", err)
	}
	repository.blockNextLoad()

	reloadDone := make(chan error, 1)
	go func() { reloadDone <- store.Reload() }()
	<-repository.loadStarted

	type issueResult struct {
		session Session
		err     error
	}
	issueDone := make(chan issueResult, 1)
	go func() {
		issued, issueErr := store.Issue("admin")
		issueDone <- issueResult{session: issued, err: issueErr}
	}()

	select {
	case <-repository.createCalled:
		repository.allowCreateReturn()
		result := <-issueDone
		if result.err != nil {
			t.Fatalf("Issue() error = %v", result.err)
		}
		repository.releaseLoad()
		if err := <-reloadDone; err != nil {
			t.Fatalf("Reload() error = %v", err)
		}
		if _, ok := store.sessions[result.session.Token]; !ok {
			t.Fatalf("Reload() overwrote a successful concurrent Issue()")
		}
	case <-time.After(250 * time.Millisecond):
		repository.releaseLoad()
		if err := <-reloadDone; err != nil {
			t.Fatalf("Reload() error = %v", err)
		}
		repository.allowCreateReturn()
		result := <-issueDone
		if result.err != nil {
			t.Fatalf("Issue() error = %v", result.err)
		}
		if _, ok := store.sessions[result.session.Token]; !ok {
			t.Fatalf("local session missing after serialized Reload() and Issue()")
		}
	}
}

func testSession(token string, username string, issuedAt time.Time, expiresAt time.Time, revokedAt time.Time) Session {
	return Session{
		Token:     token,
		Username:  username,
		IssuedAt:  issuedAt,
		ExpiresAt: expiresAt,
		RevokedAt: revokedAt,
	}
}

type failingSessionRepository struct {
	snapshot   Snapshot
	createErr  error
	resolveErr error
	renewErr   error
	revokeErr  error
}

func (r *failingSessionRepository) Load(context.Context) (Snapshot, error) {
	return Snapshot{Sessions: cloneSessions(r.snapshot.Sessions)}, nil
}

func (r *failingSessionRepository) Create(context.Context, Session) error {
	return r.createErr
}

func (r *failingSessionRepository) Resolve(context.Context, string, time.Time) (Session, bool, error) {
	return Session{}, false, r.resolveErr
}

func (r *failingSessionRepository) Renew(context.Context, string, time.Time, time.Time) (Session, bool, error) {
	return Session{}, false, r.renewErr
}

func (r *failingSessionRepository) Revoke(context.Context, string, time.Time) (Session, bool, error) {
	return Session{}, false, r.revokeErr
}

type blockingReloadRepository struct {
	mu           sync.Mutex
	sessions     map[string]Session
	blockLoad    bool
	loadStarted  chan struct{}
	loadRelease  chan struct{}
	createCalled chan struct{}
	createReturn chan struct{}
}

func newBlockingReloadRepository() *blockingReloadRepository {
	return &blockingReloadRepository{sessions: map[string]Session{}}
}

func (r *blockingReloadRepository) blockNextLoad() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.blockLoad = true
	r.loadStarted = make(chan struct{})
	r.loadRelease = make(chan struct{})
	r.createCalled = make(chan struct{})
	r.createReturn = make(chan struct{})
}

func (r *blockingReloadRepository) releaseLoad() {
	close(r.loadRelease)
}

func (r *blockingReloadRepository) allowCreateReturn() {
	close(r.createReturn)
}

func (r *blockingReloadRepository) Load(context.Context) (Snapshot, error) {
	r.mu.Lock()
	snapshot := Snapshot{Sessions: cloneSessions(r.sessions)}
	if !r.blockLoad {
		r.mu.Unlock()
		return snapshot, nil
	}
	r.blockLoad = false
	loadStarted := r.loadStarted
	loadRelease := r.loadRelease
	r.mu.Unlock()
	close(loadStarted)
	<-loadRelease
	return snapshot, nil
}

func (r *blockingReloadRepository) Create(_ context.Context, session Session) error {
	close(r.createCalled)
	<-r.createReturn
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[session.Token] = session
	return nil
}

func (r *blockingReloadRepository) Resolve(_ context.Context, token string, now time.Time) (Session, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	session, ok := r.sessions[token]
	if !ok || !session.RevokedAt.IsZero() || !now.Before(session.ExpiresAt) {
		return Session{}, false, nil
	}
	return session, true, nil
}

func (r *blockingReloadRepository) Renew(_ context.Context, token string, now time.Time, expiresAt time.Time) (Session, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	session, ok := r.sessions[token]
	if !ok || !session.RevokedAt.IsZero() || !now.Before(session.ExpiresAt) {
		return Session{}, false, nil
	}
	session.ExpiresAt = expiresAt
	r.sessions[token] = session
	return session, true, nil
}

func (r *blockingReloadRepository) Revoke(_ context.Context, token string, now time.Time) (Session, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	session, ok := r.sessions[token]
	if !ok || !session.RevokedAt.IsZero() || !now.Before(session.ExpiresAt) {
		return Session{}, false, nil
	}
	session.RevokedAt = now
	r.sessions[token] = session
	return session, true, nil
}
