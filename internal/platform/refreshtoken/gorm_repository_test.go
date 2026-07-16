package refreshtoken

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/GnosiST/platform-go/internal/platform/storage"
)

func TestGORMRepositoryPersistsRefreshTokenFamilyLineage(t *testing.T) {
	now := time.Date(2026, 7, 7, 10, 0, 0, 0, time.UTC)
	repository := openGORMRepository(t)
	store, err := NewRepositoryBackedStore(Options{TTL: time.Hour, HashKey: []byte("test-refresh-hash-key"), Now: func() time.Time { return now }}, repository)
	if err != nil {
		t.Fatalf("NewRepositoryBackedStore() error = %v", err)
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
	now = now.Add(10 * time.Minute)
	rotated, err := store.Rotate(context.Background(), issued.RefreshToken, Effects{})
	if err != nil {
		t.Fatalf("Rotate() error = %v", err)
	}

	reloaded, err := NewRepositoryBackedStore(Options{TTL: time.Hour, HashKey: []byte("test-refresh-hash-key"), Now: func() time.Time { return now }}, repository)
	if err != nil {
		t.Fatalf("reload NewRepositoryBackedStore() error = %v", err)
	}
	records := reloaded.Family(issued.Record.FamilyID)
	if len(records) != 2 {
		t.Fatalf("reloaded family records = %d, want 2", len(records))
	}
	if records[0].Status != StatusRotated || records[1].Status != StatusActive {
		t.Fatalf("reloaded records = %+v, want rotated then active", records)
	}
	if records[1].ParentTokenID != issued.Record.TokenID || records[1].TokenID != rotated.Record.TokenID {
		t.Fatalf("reloaded lineage = %+v, want parent %s and token %s", records[1], issued.Record.TokenID, rotated.Record.TokenID)
	}
}

func openGORMRepository(t *testing.T) *GORMRepository {
	t.Helper()
	db, err := storage.OpenGORM(storage.Config{
		Driver: "sqlite",
		DSN:    filepath.Join(t.TempDir(), "refresh-token-families.db"),
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
	return repository
}
