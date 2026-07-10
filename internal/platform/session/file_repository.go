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
	Version  int                `json:"version"`
	Sessions map[string]Session `json:"sessions"`
}

type FileRepository struct {
	mu       sync.Mutex
	path     string
	sessions map[string]Session
}

func NewFileRepository(path string) *FileRepository {
	return &FileRepository{path: strings.TrimSpace(path), sessions: map[string]Session{}}
}

func (r *FileRepository) Load(context.Context) (Snapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.loadLocked()
}

func (r *FileRepository) Create(_ context.Context, session Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	snapshot, err := r.loadLocked()
	if err != nil {
		return err
	}
	if _, exists := snapshot.Sessions[session.Token]; exists {
		return fmt.Errorf("session token %q already exists", session.Token)
	}
	snapshot.Sessions[session.Token] = session
	return r.writeLocked(snapshot)
}

func (r *FileRepository) Resolve(_ context.Context, token string, now time.Time) (Session, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	snapshot, err := r.loadLocked()
	if err != nil {
		return Session{}, false, err
	}
	session, ok := activeSession(snapshot, token, now)
	return session, ok, nil
}

func (r *FileRepository) Renew(_ context.Context, token string, now time.Time, expiresAt time.Time) (Session, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	snapshot, err := r.loadLocked()
	if err != nil {
		return Session{}, false, err
	}
	session, ok := activeSession(snapshot, token, now)
	if !ok {
		return Session{}, false, nil
	}
	session.ExpiresAt = expiresAt.UTC()
	snapshot.Sessions[token] = session
	if err := r.writeLocked(snapshot); err != nil {
		return Session{}, false, err
	}
	return session, true, nil
}

func (r *FileRepository) Revoke(_ context.Context, token string, now time.Time) (Session, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	snapshot, err := r.loadLocked()
	if err != nil {
		return Session{}, false, err
	}
	session, ok := activeSession(snapshot, token, now)
	if !ok {
		return Session{}, false, nil
	}
	session.RevokedAt = now.UTC()
	snapshot.Sessions[token] = session
	if err := r.writeLocked(snapshot); err != nil {
		return Session{}, false, err
	}
	return session, true, nil
}

func (r *FileRepository) loadLocked() (Snapshot, error) {
	if r.path == "" {
		return Snapshot{Sessions: cloneSessions(r.sessions)}, nil
	}
	content, err := os.ReadFile(r.path)
	if errors.Is(err, os.ErrNotExist) {
		return Snapshot{Sessions: map[string]Session{}}, nil
	}
	if err != nil {
		return Snapshot{}, err
	}
	var snapshot fileSnapshot
	if err := json.Unmarshal(content, &snapshot); err != nil {
		return Snapshot{}, err
	}
	if snapshot.Sessions == nil {
		snapshot.Sessions = map[string]Session{}
	}
	return Snapshot{Sessions: cloneSessions(snapshot.Sessions)}, nil
}

func (r *FileRepository) writeLocked(snapshot Snapshot) error {
	if r.path == "" {
		r.sessions = cloneSessions(snapshot.Sessions)
		return nil
	}
	directory := filepath.Dir(r.path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}
	content, err := json.MarshalIndent(fileSnapshot{Version: 1, Sessions: cloneSessions(snapshot.Sessions)}, "", "  ")
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

func activeSession(snapshot Snapshot, token string, now time.Time) (Session, bool) {
	session, ok := snapshot.Sessions[token]
	if !ok || !session.RevokedAt.IsZero() || !now.UTC().Before(session.ExpiresAt) {
		return Session{}, false
	}
	return session, true
}
