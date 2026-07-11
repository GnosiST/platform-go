package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type fileSnapshot struct {
	Version  int                      `json:"version"`
	Sessions map[string]StoredSession `json:"sessions"`
}

type FileRepository struct {
	mu       sync.Mutex
	path     string
	sessions map[string]StoredSession
}

func NewFileRepository(path string) *FileRepository {
	return &FileRepository{path: strings.TrimSpace(path), sessions: map[string]StoredSession{}}
}

func (r *FileRepository) Load(context.Context) (Snapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.loadLocked()
}

func (r *FileRepository) Create(_ context.Context, session StoredSession) error {
	if err := validateStoredSessionForKey(session.TokenDigest, session); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	snapshot, err := r.loadLocked()
	if err != nil {
		return err
	}
	if _, exists := snapshot.Sessions[session.TokenDigest]; exists {
		return fmt.Errorf("session digest already exists")
	}
	snapshot.Sessions[session.TokenDigest] = session
	return r.writeLocked(snapshot)
}

func (r *FileRepository) Resolve(_ context.Context, tokenDigest string, now time.Time) (StoredSession, bool, error) {
	if err := validateTokenDigest(tokenDigest); err != nil {
		return StoredSession{}, false, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	snapshot, err := r.loadLocked()
	if err != nil {
		return StoredSession{}, false, err
	}
	return activeSession(snapshot, tokenDigest, now)
}

func (r *FileRepository) Renew(_ context.Context, tokenDigest string, now time.Time, expiresAt time.Time) (StoredSession, bool, error) {
	if err := validateTokenDigest(tokenDigest); err != nil {
		return StoredSession{}, false, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	snapshot, err := r.loadLocked()
	if err != nil {
		return StoredSession{}, false, err
	}
	session, ok, err := activeSession(snapshot, tokenDigest, now)
	if err != nil {
		return StoredSession{}, false, err
	}
	if !ok {
		return StoredSession{}, false, nil
	}
	session.ExpiresAt = expiresAt.UTC()
	snapshot.Sessions[tokenDigest] = session
	if err := r.writeLocked(snapshot); err != nil {
		return StoredSession{}, false, err
	}
	return session, true, nil
}

func (r *FileRepository) Revoke(_ context.Context, tokenDigest string, now time.Time) (StoredSession, bool, error) {
	if err := validateTokenDigest(tokenDigest); err != nil {
		return StoredSession{}, false, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	snapshot, err := r.loadLocked()
	if err != nil {
		return StoredSession{}, false, err
	}
	session, ok, err := activeSession(snapshot, tokenDigest, now)
	if err != nil {
		return StoredSession{}, false, err
	}
	if !ok {
		return StoredSession{}, false, nil
	}
	session.RevokedAt = now.UTC()
	snapshot.Sessions[tokenDigest] = session
	if err := r.writeLocked(snapshot); err != nil {
		return StoredSession{}, false, err
	}
	return session, true, nil
}

func (r *FileRepository) loadLocked() (Snapshot, error) {
	if r.path == "" {
		snapshot := Snapshot{Sessions: cloneStoredSessions(r.sessions)}
		if err := validateSnapshot(snapshot); err != nil {
			return Snapshot{}, err
		}
		return snapshot, nil
	}
	content, err := os.ReadFile(r.path)
	if errors.Is(err, os.ErrNotExist) {
		return Snapshot{Sessions: map[string]StoredSession{}}, nil
	}
	if err != nil {
		return Snapshot{}, err
	}
	var snapshot fileSnapshot
	if err := json.Unmarshal(content, &snapshot); err != nil {
		return Snapshot{}, err
	}
	if snapshot.Version == 1 {
		empty := Snapshot{Sessions: map[string]StoredSession{}}
		if err := r.writeLocked(empty); err != nil {
			return Snapshot{}, err
		}
		return empty, nil
	}
	if snapshot.Version != 2 {
		return Snapshot{}, fmt.Errorf("unsupported session snapshot version %d", snapshot.Version)
	}
	if snapshot.Sessions == nil {
		snapshot.Sessions = map[string]StoredSession{}
	}
	loaded := Snapshot{Sessions: cloneStoredSessions(snapshot.Sessions)}
	if err := validateSnapshot(loaded); err != nil {
		return Snapshot{}, err
	}
	return loaded, nil
}

func (r *FileRepository) writeLocked(snapshot Snapshot) error {
	if err := validateSnapshot(snapshot); err != nil {
		return err
	}
	if r.path == "" {
		r.sessions = cloneStoredSessions(snapshot.Sessions)
		return nil
	}
	directory := filepath.Dir(r.path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}
	content, err := json.MarshalIndent(fileSnapshot{Version: 2, Sessions: cloneStoredSessions(snapshot.Sessions)}, "", "  ")
	if err != nil {
		return err
	}
	tempFile, err := os.CreateTemp(directory, "."+filepath.Base(r.path)+".tmp-*")
	if err != nil {
		return err
	}
	tempPath := tempFile.Name()
	defer func() { _ = os.Remove(tempPath) }()
	if err := tempFile.Chmod(0o600); err != nil {
		_ = tempFile.Close()
		return err
	}
	if _, err := tempFile.Write(content); err != nil {
		_ = tempFile.Close()
		return err
	}
	if err := tempFile.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, r.path)
}

func activeSession(snapshot Snapshot, tokenDigest string, now time.Time) (StoredSession, bool, error) {
	session, ok := snapshot.Sessions[tokenDigest]
	if !ok || !session.RevokedAt.IsZero() || !now.UTC().Before(session.ExpiresAt) {
		return StoredSession{}, false, nil
	}
	if err := validateStoredSessionForKey(tokenDigest, session); err != nil {
		return StoredSession{}, false, err
	}
	return session, true, nil
}
