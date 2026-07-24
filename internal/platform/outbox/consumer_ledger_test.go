package outbox

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/GnosiST/platform-go/internal/platform/storage"
	"gorm.io/gorm"
)

func TestConsumerLedgerSkipsCompletedDuplicateAndPersistsItsReceipt(t *testing.T) {
	ctx := context.Background()
	db, err := storage.OpenGORM(storage.Config{Driver: "sqlite", DSN: "file:outbox-consumer-ledger?mode=memory&cache=shared"})
	if err != nil {
		t.Fatal(err)
	}
	if err := PrepareGORMStore(ctx, db); err != nil {
		t.Fatal(err)
	}
	ledger, err := OpenConsumerLedger(ctx, db, fixedOutboxClock)
	if err != nil {
		t.Fatal(err)
	}
	envelope := testEnvelope(t, "evt-consumer-duplicate")
	var calls atomic.Int32
	apply := func(*gorm.DB) error {
		calls.Add(1)
		return nil
	}
	processed, err := ledger.Apply(ctx, "search-projection", envelope, apply)
	if err != nil || !processed {
		t.Fatalf("first Apply() = %t, %v", processed, err)
	}
	processed, err = ledger.Apply(ctx, "search-projection", envelope, apply)
	if err != nil || processed {
		t.Fatalf("duplicate Apply() = %t, %v", processed, err)
	}
	if calls.Load() != 1 {
		t.Fatalf("apply calls = %d, want 1", calls.Load())
	}
	var receipt gormConsumerReceipt
	if err := db.Where("consumer_id = ? AND event_id = ?", "search-projection", envelope.EventID).Take(&receipt).Error; err != nil {
		t.Fatalf("load consumer receipt: %v", err)
	}
	if receipt.TenantCode != envelope.TenantCode || receipt.EventType != envelope.EventType || receipt.EventVersion != envelope.Version || receipt.ProcessedAt.IsZero() {
		t.Fatalf("consumer receipt = %+v", receipt)
	}
}

func TestConsumerLedgerRollsBackFailedApplySoDeliveryCanRetry(t *testing.T) {
	ctx := context.Background()
	db, err := storage.OpenGORM(storage.Config{Driver: "sqlite", DSN: "file:outbox-consumer-retry?mode=memory&cache=shared"})
	if err != nil {
		t.Fatal(err)
	}
	if err := PrepareGORMStore(ctx, db); err != nil {
		t.Fatal(err)
	}
	ledger, err := OpenConsumerLedger(ctx, db, fixedOutboxClock)
	if err != nil {
		t.Fatal(err)
	}
	envelope := testEnvelope(t, "evt-consumer-retry")
	failed := errors.New("projection unavailable")
	if processed, err := ledger.Apply(ctx, "search-projection", envelope, func(*gorm.DB) error { return failed }); processed || !errors.Is(err, failed) {
		t.Fatalf("failed Apply() = %t, %v", processed, err)
	}
	var count int64
	if err := db.Model(&gormConsumerReceipt{}).Where("consumer_id = ? AND event_id = ?", "search-projection", envelope.EventID).Count(&count).Error; err != nil {
		t.Fatalf("count consumer receipts: %v", err)
	}
	if count != 0 {
		t.Fatalf("receipts after failed apply = %d, want 0", count)
	}
	var calls atomic.Int32
	processed, err := ledger.Apply(ctx, "search-projection", envelope, func(*gorm.DB) error {
		calls.Add(1)
		return nil
	})
	if err != nil || !processed || calls.Load() != 1 {
		t.Fatalf("retry Apply() = %t, %v; calls = %d", processed, err, calls.Load())
	}
}
