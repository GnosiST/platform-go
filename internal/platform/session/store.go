package session

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
	"sync"
	"time"
)

const DefaultTTL = 8 * time.Hour

var ErrInvalidSession = errors.New("invalid session")

type Options struct {
	TTL time.Duration
	Now func() time.Time
}

type Snapshot struct {
	Sessions map[string]Session `json:"sessions"`
}

type Repository interface {
	Load(context.Context) (Snapshot, error)
	Create(context.Context, Session) error
	Resolve(context.Context, string, time.Time) (Session, bool, error)
	Renew(context.Context, string, time.Time, time.Time) (Session, bool, error)
	Revoke(context.Context, string, time.Time) (Session, bool, error)
}

type Session struct {
	Token     string    `json:"token"`
	Username  string    `json:"username"`
	IssuedAt  time.Time `json:"issuedAt"`
	ExpiresAt time.Time `json:"expiresAt"`
	RevokedAt time.Time `json:"revokedAt,omitempty"`
}

type Store struct {
	mu         sync.Mutex
	sessions   map[string]Session
	ttl        time.Duration
	now        func() time.Time
	repository Repository
}

func NewStore(options Options) *Store {
	ttl := options.TTL
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &Store{
		sessions: map[string]Session{},
		ttl:      ttl,
		now:      now,
	}
}

func NewRepositoryBackedStore(options Options, repository Repository) (*Store, error) {
	store := NewStore(options)
	if repository == nil {
		return store, nil
	}
	snapshot, err := repository.Load(context.Background())
	if err != nil {
		return nil, err
	}
	store.sessions = cloneSessions(snapshot.Sessions)
	store.repository = repository
	return store, nil
}

func (s *Store) Reload() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.repository == nil {
		return nil
	}
	snapshot, err := s.repository.Load(context.Background())
	if err != nil {
		return err
	}
	s.sessions = cloneSessions(snapshot.Sessions)
	return nil
}

func (s *Store) Issue(username string) (Session, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return Session{}, ErrInvalidSession
	}
	token, err := newToken()
	if err != nil {
		return Session{}, err
	}
	issuedAt := s.now().UTC()
	session := Session{
		Token:     token,
		Username:  username,
		IssuedAt:  issuedAt,
		ExpiresAt: issuedAt.Add(s.ttl),
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.repository != nil {
		if err := s.repository.Create(context.Background(), session); err != nil {
			return Session{}, err
		}
	}
	s.sessions[token] = session
	return session, nil
}

func (s *Store) ResolveContext(ctx context.Context, token string) (Session, bool, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return Session{}, false, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now().UTC()
	if s.repository != nil {
		session, ok, err := s.repository.Resolve(ctx, token, now)
		if err != nil || !ok {
			return Session{}, false, err
		}
		s.sessions[token] = session
		return session, true, nil
	}
	session, ok := s.sessions[token]
	if !ok || !session.RevokedAt.IsZero() || !now.Before(session.ExpiresAt) {
		return Session{}, false, nil
	}
	return session, true, nil
}

func (s *Store) Resolve(token string) (Session, bool) {
	session, ok, err := s.ResolveContext(context.Background(), token)
	if err != nil {
		return Session{}, false
	}
	return session, ok
}

func (s *Store) RenewContext(ctx context.Context, token string) (Session, bool, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return Session{}, false, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now().UTC()
	expiresAt := now.Add(s.ttl)
	if s.repository != nil {
		session, ok, err := s.repository.Renew(ctx, token, now, expiresAt)
		if err != nil || !ok {
			return Session{}, false, err
		}
		s.sessions[token] = session
		return session, true, nil
	}
	session, ok := s.sessions[token]
	if !ok || !session.RevokedAt.IsZero() || !now.Before(session.ExpiresAt) {
		return Session{}, false, nil
	}
	session.ExpiresAt = expiresAt
	s.sessions[token] = session
	return session, true, nil
}

func (s *Store) Renew(token string) (Session, bool) {
	session, ok, err := s.RenewContext(context.Background(), token)
	if err != nil {
		return Session{}, false
	}
	return session, ok
}

func (s *Store) RevokeContext(ctx context.Context, token string) (bool, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return false, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now().UTC()
	if s.repository != nil {
		session, ok, err := s.repository.Revoke(ctx, token, now)
		if err != nil || !ok {
			return false, err
		}
		s.sessions[token] = session
		return true, nil
	}
	session, ok := s.sessions[token]
	if !ok || !session.RevokedAt.IsZero() || !now.Before(session.ExpiresAt) {
		return false, nil
	}
	session.RevokedAt = now
	s.sessions[token] = session
	return true, nil
}

func (s *Store) Revoke(token string) bool {
	ok, err := s.RevokeContext(context.Background(), token)
	if err != nil {
		return false
	}
	return ok
}

func cloneSessions(sessions map[string]Session) map[string]Session {
	cloned := make(map[string]Session, len(sessions))
	for token, session := range sessions {
		cloned[token] = session
	}
	return cloned
}

func newToken() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}
