package refreshtoken

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"
)

func TestStoreIssuesHashedRefreshTokenFamily(t *testing.T) {
	now := time.Date(2026, 7, 7, 10, 0, 0, 0, time.UTC)
	store, err := NewStore(Options{TTL: time.Hour, HashKey: []byte("test-refresh-hash-key"), Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	issued, err := store.Issue(context.Background(), IssueInput{
		SessionID: "session-1",
		Username:  "ops",
		TenantID:  "platform",
		TokenType: "admin",
	})
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if issued.RefreshToken == "" {
		t.Fatalf("Issue() refresh token is empty")
	}
	if issued.Record.TokenHash == "" || issued.Record.TokenHash == issued.RefreshToken {
		t.Fatalf("Issue() token hash = %q, raw = %q; want hashed value only", issued.Record.TokenHash, issued.RefreshToken)
	}
	records := store.Family(issued.Record.FamilyID)
	if len(records) != 1 {
		t.Fatalf("Family() records = %d, want 1", len(records))
	}
	if records[0].TokenHash != "" || records[0].Status != StatusActive {
		t.Fatalf("Family() record = %+v, want active public record without token hash", records[0])
	}
}

func TestStoreRotatesRefreshTokenAndDetectsReuse(t *testing.T) {
	now := time.Date(2026, 7, 7, 10, 0, 0, 0, time.UTC)
	store, err := NewStore(Options{TTL: time.Hour, HashKey: []byte("test-refresh-hash-key"), Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	issued, err := store.Issue(context.Background(), IssueInput{
		SessionID: "session-1",
		Username:  "ops",
		TenantID:  "platform",
		TokenType: "admin",
	})
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	effects := &recordingEffects{}

	now = now.Add(15 * time.Minute)
	rotated, err := store.Rotate(context.Background(), issued.RefreshToken, Effects{
		RenewSession:        effects.renewSession,
		RevokeSession:       effects.revokeSession,
		PublishInvalidation: effects.publishInvalidation,
		Audit:               effects.audit,
	})
	if err != nil {
		t.Fatalf("Rotate() error = %v", err)
	}
	if rotated.RefreshToken == "" || rotated.RefreshToken == issued.RefreshToken {
		t.Fatalf("Rotate() refresh token = %q, want a new token", rotated.RefreshToken)
	}
	if rotated.Record.ParentTokenID != issued.Record.TokenID {
		t.Fatalf("Rotate() parent = %q, want %q", rotated.Record.ParentTokenID, issued.Record.TokenID)
	}
	if !slices.Contains(effects.renewedSessions, "session-1") {
		t.Fatalf("renewed sessions = %+v, want session-1", effects.renewedSessions)
	}
	if !slices.Contains(effects.auditActions, "auth.refresh.rotate") {
		t.Fatalf("audit actions = %+v, want auth.refresh.rotate", effects.auditActions)
	}
	if effects.auditRawValues {
		t.Fatalf("audit contained raw token values")
	}

	_, err = store.Rotate(context.Background(), issued.RefreshToken, Effects{
		RevokeSession:       effects.revokeSession,
		PublishInvalidation: effects.publishInvalidation,
		Audit:               effects.audit,
	})
	if !errors.Is(err, ErrRefreshTokenReuseDetected) {
		t.Fatalf("Rotate(reused) error = %v, want ErrRefreshTokenReuseDetected", err)
	}
	if !slices.Contains(effects.revokedSessions, "session-1") {
		t.Fatalf("revoked sessions = %+v, want session-1", effects.revokedSessions)
	}
	if !slices.Contains(effects.auditActions, "auth.refresh.reuse_detected") {
		t.Fatalf("audit actions = %+v, want auth.refresh.reuse_detected", effects.auditActions)
	}
	records := store.Family(issued.Record.FamilyID)
	for _, record := range records {
		if record.Status != StatusReused && record.Status != StatusRevoked {
			t.Fatalf("family record after reuse = %+v, want reused or revoked", record)
		}
	}
}

func TestStoreRejectsExpiredRefreshTokenAsReuse(t *testing.T) {
	now := time.Date(2026, 7, 7, 10, 0, 0, 0, time.UTC)
	store, err := NewStore(Options{TTL: time.Hour, HashKey: []byte("test-refresh-hash-key"), Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	issued, err := store.Issue(context.Background(), IssueInput{
		SessionID: "session-1",
		Username:  "ops",
		TenantID:  "platform",
		TokenType: "admin",
	})
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	effects := &recordingEffects{}
	now = now.Add(2 * time.Hour)
	_, err = store.Rotate(context.Background(), issued.RefreshToken, Effects{
		RevokeSession:       effects.revokeSession,
		PublishInvalidation: effects.publishInvalidation,
		Audit:               effects.audit,
	})
	if !errors.Is(err, ErrRefreshTokenReuseDetected) {
		t.Fatalf("Rotate(expired) error = %v, want ErrRefreshTokenReuseDetected", err)
	}
	if len(effects.revokedSessions) != 1 || effects.revokedSessions[0] != "session-1" {
		t.Fatalf("revoked sessions = %+v, want session-1", effects.revokedSessions)
	}
}

type recordingEffects struct {
	renewedSessions []string
	revokedSessions []string
	auditActions    []string
	invalidation    int
	auditRawValues  bool
}

func (r *recordingEffects) renewSession(_ context.Context, sessionID string) (time.Time, error) {
	r.renewedSessions = append(r.renewedSessions, sessionID)
	return time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC), nil
}

func (r *recordingEffects) revokeSession(_ context.Context, sessionID string) error {
	r.revokedSessions = append(r.revokedSessions, sessionID)
	return nil
}

func (r *recordingEffects) publishInvalidation(context.Context) error {
	r.invalidation++
	return nil
}

func (r *recordingEffects) audit(_ context.Context, event AuditEvent) error {
	r.auditActions = append(r.auditActions, event.Action)
	if event.RefreshToken != "" || event.TokenHash != "" {
		r.auditRawValues = true
	}
	return nil
}
