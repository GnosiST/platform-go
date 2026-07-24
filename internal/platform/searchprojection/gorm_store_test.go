package searchprojection

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/GnosiST/platform-go/internal/platform/integration"
	"github.com/GnosiST/platform-go/internal/platform/outbox"
	"github.com/GnosiST/platform-go/internal/platform/storage"
	"gorm.io/gorm"
)

func TestGORMStoreAppliesAllowlistedDocumentOnceAndPersistsCheckpoint(t *testing.T) {
	ctx := context.Background()
	store, db := newTestStore(t, "search-projection-apply")
	mapping := testMapping()
	indexer := &testIndexer{}
	envelope := testEnvelope(t, "evt-search-apply", `{"id":"order-1","title":"Order one","internal":"omit"}`)
	processed, err := store.Apply(ctx, mapping, envelope, indexer)
	if err != nil || !processed {
		t.Fatalf("first Apply() = %t, %v", processed, err)
	}
	processed, err = store.Apply(ctx, mapping, envelope, indexer)
	if err != nil || processed {
		t.Fatalf("duplicate Apply() = %t, %v", processed, err)
	}
	if len(indexer.indexed) != 1 || string(indexer.indexed[0].Body) != `{"title":"Order one"}` || indexer.indexed[0].TenantID != "tenant-a" {
		t.Fatalf("indexed documents = %+v", indexer.indexed)
	}
	var checkpoint gormCheckpoint
	if err := db.Where("mapping_id = ?", mapping.ID).Take(&checkpoint).Error; err != nil {
		t.Fatalf("load checkpoint: %v", err)
	}
	if checkpoint.LastEventID != envelope.EventID || checkpoint.TenantCode != envelope.TenantCode {
		t.Fatalf("checkpoint = %+v", checkpoint)
	}
}

func TestGORMStoreRollsBackReceiptAfterIndexerFailureAndSynchronizesTombstones(t *testing.T) {
	ctx := context.Background()
	store, _ := newTestStore(t, "search-projection-delete")
	mapping := testMapping()
	indexer := &testIndexer{indexErr: errors.New("index unavailable")}
	envelope := testEnvelope(t, "evt-search-retry", `{"id":"order-2","title":"Order two"}`)
	if processed, err := store.Apply(ctx, mapping, envelope, indexer); processed || !errors.Is(err, indexer.indexErr) {
		t.Fatalf("failed Apply() = %t, %v", processed, err)
	}
	indexer.indexErr = nil
	if processed, err := store.Apply(ctx, mapping, envelope, indexer); err != nil || !processed {
		t.Fatalf("retry Apply() = %t, %v", processed, err)
	}
	deleted := testEnvelope(t, "evt-search-delete", `{"id":"order-2","deleted":true}`)
	if processed, err := store.Apply(ctx, mapping, deleted, indexer); err != nil || !processed {
		t.Fatalf("delete Apply() = %t, %v", processed, err)
	}
	if len(indexer.deleted) != 1 || indexer.deleted[0].ID != "order-2" || indexer.deleted[0].TenantID != "tenant-a" {
		t.Fatalf("deleted documents = %+v", indexer.deleted)
	}
	result, err := store.Rebuild(ctx, mapping, indexer, RebuildOptions{Limit: 10})
	if err != nil || result.Deleted != 1 || result.Indexed != 0 {
		t.Fatalf("Rebuild() = %+v, %v", result, err)
	}
}

func newTestStore(t *testing.T, name string) (*GORMStore, *gorm.DB) {
	t.Helper()
	db, err := storage.OpenGORM(storage.Config{Driver: "sqlite", DSN: "file:" + name + "?mode=memory&cache=shared"})
	if err != nil {
		t.Fatal(err)
	}
	if err := PrepareGORMStore(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	store, err := OpenGORMStore(context.Background(), db, func() time.Time { return time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	return store, db
}

func testMapping() Mapping {
	return Mapping{ID: "orders-search", ConsumerID: "orders-search", EventType: "platform.order.changed.v1", Collection: "orders", DocumentIDField: "id", DeletedField: "deleted", Fields: []string{"title"}}
}

func testEnvelope(t *testing.T, eventID, payload string) outbox.Envelope {
	t.Helper()
	envelope, err := outbox.ValidateEnvelope(outbox.Envelope{EventID: eventID, EventType: "platform.order.changed.v1", Version: outbox.EnvelopeVersion, OccurredAt: time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC), Producer: "platform-test", TenantCode: "tenant-a", TraceParent: "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01", Payload: []byte(payload)})
	if err != nil {
		t.Fatal(err)
	}
	return envelope
}

type testIndexer struct {
	indexed  []integration.SearchDocument
	deleted  []integration.SearchDocumentRef
	indexErr error
}

func (i *testIndexer) Kind() string                 { return "test" }
func (i *testIndexer) Health(context.Context) error { return nil }
func (i *testIndexer) Index(_ context.Context, document integration.SearchDocument) error {
	if i.indexErr != nil {
		return i.indexErr
	}
	i.indexed = append(i.indexed, document)
	return nil
}
func (i *testIndexer) Delete(_ context.Context, document integration.SearchDocumentRef) error {
	i.deleted = append(i.deleted, document)
	return nil
}
