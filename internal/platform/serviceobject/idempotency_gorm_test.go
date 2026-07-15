package serviceobject

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestGORMIdempotencyStorePersistsCompletedReplay(t *testing.T) {
	db := openGORMIdempotencyTestDB(t)
	store := newGORMIdempotencyTestStore(t, db, GORMIdempotencyStoreOptions{})
	scope := testIdempotencyScope("persisted")
	var calls atomic.Int32
	result, err := store.Execute(context.Background(), scope, "fingerprint", func(context.Context) (CommandResult, error) {
		calls.Add(1)
		return CommandResult{Values: map[string]any{"name": "updated"}}, nil
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Values["name"] != "updated" {
		t.Fatalf("Execute() result = %#v", result)
	}

	reopened := newGORMIdempotencyTestStore(t, db, GORMIdempotencyStoreOptions{})
	replayed, err := reopened.Execute(context.Background(), scope, "fingerprint", func(context.Context) (CommandResult, error) {
		calls.Add(1)
		return CommandResult{}, nil
	})
	if err != nil {
		t.Fatalf("replay Execute() error = %v", err)
	}
	if replayed.Values["name"] != "updated" {
		t.Fatalf("replay result = %#v", replayed)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("execute calls = %d, want 1", got)
	}

	var record gormIdempotencyRecord
	if err := db.Where("scope_digest = ?", idempotencyScopeDigest(scope)).Take(&record).Error; err != nil {
		t.Fatalf("load persisted record: %v", err)
	}
	if record.Status != idempotencyStatusCompleted || record.Fingerprint != "fingerprint" || len(record.ResultJSON) == 0 || record.ExpiresAt.IsZero() {
		t.Fatalf("persisted record = %#v", record)
	}
	if _, err := reopened.Execute(context.Background(), scope, "different", func(context.Context) (CommandResult, error) {
		return CommandResult{}, nil
	}); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("conflicting replay error = %v, want ErrIdempotencyConflict", err)
	}
}

func TestOpenGORMIdempotencyStoreRequiresPreparedSchema(t *testing.T) {
	db := openGORMIdempotencyTestDB(t)
	if _, err := OpenGORMIdempotencyStore(context.Background(), db, GORMIdempotencyStoreOptions{}); !errors.Is(err, ErrObjectUnavailable) {
		t.Fatalf("OpenGORMIdempotencyStore(unprepared) error = %v", err)
	}
	if db.Migrator().HasTable(&gormIdempotencyRecord{}) {
		t.Fatal("OpenGORMIdempotencyStore created schema")
	}
	if _, err := NewGORMIdempotencyStore(context.Background(), db, GORMIdempotencyStoreOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenGORMIdempotencyStore(context.Background(), db, GORMIdempotencyStoreOptions{}); err != nil {
		t.Fatalf("OpenGORMIdempotencyStore(prepared) error = %v", err)
	}
}

func TestGORMIdempotencyStoreCoordinatesConcurrentClaimAcrossStores(t *testing.T) {
	db := openGORMIdempotencyTestDB(t)
	firstStore := newGORMIdempotencyTestStore(t, db, GORMIdempotencyStoreOptions{})
	secondStore := newGORMIdempotencyTestStore(t, db, GORMIdempotencyStoreOptions{})
	scope := testIdempotencyScope("concurrent")
	entered := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	execute := func(context.Context) (CommandResult, error) {
		if calls.Add(1) == 1 {
			close(entered)
		}
		<-release
		return CommandResult{Values: map[string]any{"state": "done"}}, nil
	}

	type response struct {
		result CommandResult
		err    error
	}
	responses := make(chan response, 2)
	go func() {
		result, err := firstStore.Execute(context.Background(), scope, "fingerprint", execute)
		responses <- response{result: result, err: err}
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("first command did not start")
	}
	go func() {
		result, err := secondStore.Execute(context.Background(), scope, "fingerprint", execute)
		responses <- response{result: result, err: err}
	}()
	time.Sleep(25 * time.Millisecond)
	close(release)
	for range 2 {
		response := <-responses
		if response.err != nil {
			t.Fatalf("Execute() error = %v", response.err)
		}
		if response.result.Values["state"] != "done" {
			t.Fatalf("Execute() result = %#v", response.result)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("execute calls = %d, want 1", got)
	}
}

func TestGORMIdempotencyStoreReleasesFailureAndReclaimsExpiredRecords(t *testing.T) {
	db := openGORMIdempotencyTestDB(t)
	now := time.Date(2026, time.July, 15, 10, 0, 0, 0, time.UTC)
	options := GORMIdempotencyStoreOptions{
		TTL: time.Minute, LeaseDuration: time.Second, PollInterval: time.Millisecond,
		CleanupInterval: time.Hour, Now: func() time.Time { return now },
	}
	store := newGORMIdempotencyTestStore(t, db, options)
	scope := testIdempotencyScope("failure-and-expiry")
	commandErr := errors.New("command rejected")
	if _, err := store.Execute(context.Background(), scope, "first", func(context.Context) (CommandResult, error) {
		return CommandResult{}, commandErr
	}); !errors.Is(err, commandErr) {
		t.Fatalf("failed Execute() error = %v, want command error", err)
	}
	var count int64
	if err := db.Model(&gormIdempotencyRecord{}).Where("scope_digest = ?", idempotencyScopeDigest(scope)).Count(&count).Error; err != nil {
		t.Fatalf("count failed claim: %v", err)
	}
	if count != 0 {
		t.Fatalf("failed claim count = %d, want 0", count)
	}
	if _, err := store.Execute(context.Background(), scope, "first", func(context.Context) (CommandResult, error) {
		return CommandResult{Values: map[string]any{"attempt": "retry"}}, nil
	}); err != nil {
		t.Fatalf("retry Execute() error = %v", err)
	}
	now = now.Add(time.Minute)
	var calls atomic.Int32
	if _, err := store.Execute(context.Background(), scope, "replacement", func(context.Context) (CommandResult, error) {
		calls.Add(1)
		return CommandResult{Values: map[string]any{"attempt": "replacement"}}, nil
	}); err != nil {
		t.Fatalf("expired Execute() error = %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("replacement execute calls = %d, want 1", calls.Load())
	}
	var record gormIdempotencyRecord
	if err := db.Where("scope_digest = ?", idempotencyScopeDigest(scope)).Take(&record).Error; err != nil {
		t.Fatalf("load replacement record: %v", err)
	}
	if record.Fingerprint != "replacement" || record.Status != idempotencyStatusCompleted {
		t.Fatalf("replacement record = %#v", record)
	}
}

func TestGORMIdempotencyStoreReclaimsAbandonedLease(t *testing.T) {
	db := openGORMIdempotencyTestDB(t)
	now := time.Date(2026, time.July, 15, 10, 0, 0, 0, time.UTC)
	store := newGORMIdempotencyTestStore(t, db, GORMIdempotencyStoreOptions{
		TTL: time.Hour, LeaseDuration: time.Minute, PollInterval: time.Millisecond,
		CleanupInterval: time.Hour, Now: func() time.Time { return now },
	})
	scope := testIdempotencyScope("abandoned")
	record := newGORMIdempotencyRecord(scope, idempotencyScopeDigest(scope), "fingerprint", "abandoned-owner", now.Add(-time.Hour), now.Add(-time.Minute), time.Hour)
	record.ExpiresAt = now.Add(time.Hour)
	if err := db.Create(&record).Error; err != nil {
		t.Fatalf("create abandoned record: %v", err)
	}
	var calls atomic.Int32
	if _, err := store.Execute(context.Background(), scope, "fingerprint", func(context.Context) (CommandResult, error) {
		calls.Add(1)
		return CommandResult{Values: map[string]any{"reclaimed": true}}, nil
	}); err != nil {
		t.Fatalf("reclaim Execute() error = %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("reclaimed execute calls = %d, want 1", calls.Load())
	}
}

func TestGORMIdempotencyStoreFinalizesAfterRequestCancellation(t *testing.T) {
	db := openGORMIdempotencyTestDB(t)
	store := newGORMIdempotencyTestStore(t, db, GORMIdempotencyStoreOptions{})
	scope := testIdempotencyScope("cancel-after-command")
	ctx, cancel := context.WithCancel(context.Background())
	result, err := store.Execute(ctx, scope, "fingerprint", func(context.Context) (CommandResult, error) {
		cancel()
		return CommandResult{Values: map[string]any{"saved": true}}, nil
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Values["saved"] != true {
		t.Fatalf("Execute() result = %#v", result)
	}
	replayed, err := store.Execute(context.Background(), scope, "fingerprint", func(context.Context) (CommandResult, error) {
		t.Fatal("persisted cancellation replay executed command")
		return CommandResult{}, nil
	})
	if err != nil {
		t.Fatalf("replay Execute() error = %v", err)
	}
	if replayed.Values["saved"] != true {
		t.Fatalf("replay result = %#v", replayed)
	}
}

func TestGORMIdempotencyStoreSanitizesDatabaseErrors(t *testing.T) {
	db := openGORMIdempotencyTestDB(t)
	store := newGORMIdempotencyTestStore(t, db, GORMIdempotencyStoreOptions{})
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB() error = %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close database: %v", err)
	}
	_, err = store.Execute(context.Background(), testIdempotencyScope("database-error"), "fingerprint", func(context.Context) (CommandResult, error) {
		return CommandResult{}, nil
	})
	if !errors.Is(err, ErrExecutionFailed) {
		t.Fatalf("Execute() error = %v, want ErrExecutionFailed", err)
	}
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "database") {
		t.Fatalf("Execute() leaked database detail: %v", err)
	}
}

func openGORMIdempotencyTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:" + filepath.ToSlash(filepath.Join(t.TempDir(), "idempotency.db")) + "?_busy_timeout=5000&_journal_mode=WAL"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB() error = %v", err)
	}
	sqlDB.SetMaxOpenConns(8)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func newGORMIdempotencyTestStore(t *testing.T, db *gorm.DB, options GORMIdempotencyStoreOptions) *GORMIdempotencyStore {
	t.Helper()
	store, err := NewGORMIdempotencyStore(context.Background(), db, options)
	if err != nil {
		t.Fatalf("NewGORMIdempotencyStore() error = %v", err)
	}
	return store
}
