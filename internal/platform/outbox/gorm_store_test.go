package outbox

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/GnosiST/platform-go/internal/platform/integration"
	"github.com/GnosiST/platform-go/internal/platform/storage"
	"gorm.io/gorm"
)

func TestGORMStoreAppendSharesCallerTransaction(t *testing.T) {
	ctx := context.Background()
	db, err := storage.OpenGORM(storage.Config{Driver: "sqlite", DSN: "file:outbox-atomic?mode=memory&cache=shared"})
	if err != nil {
		t.Fatal(err)
	}
	if err := PrepareGORMStore(ctx, db); err != nil {
		t.Fatal(err)
	}
	store, err := OpenGORMStore(ctx, db, fixedOutboxClock)
	if err != nil {
		t.Fatal(err)
	}
	envelope := testEnvelope(t, "evt-atomic")
	rollback := errors.New("rollback")
	if err := db.Transaction(func(tx *gorm.DB) error {
		if _, err := store.AppendInTransaction(tx, envelope); err != nil {
			return err
		}
		return rollback
	}); !errors.Is(err, rollback) {
		t.Fatalf("rollback transaction error = %v", err)
	}
	if records, err := store.List(ctx); err != nil || len(records) != 0 {
		t.Fatalf("records after rollback = %+v, %v", records, err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		_, err := store.AppendInTransaction(tx, envelope)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if records, err := store.List(ctx); err != nil || len(records) != 1 || records[0].Envelope.EventID != envelope.EventID {
		t.Fatalf("records after commit = %+v, %v", records, err)
	}
}

func TestDispatcherRetriesDeadLettersAndReplaysWithOriginalEventID(t *testing.T) {
	ctx := context.Background()
	db, err := storage.OpenGORM(storage.Config{Driver: "sqlite", DSN: "file:outbox-dispatch?mode=memory&cache=shared"})
	if err != nil {
		t.Fatal(err)
	}
	if err := PrepareGORMStore(ctx, db); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC)
	store, err := OpenGORMStore(ctx, db, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	record, err := store.Append(ctx, testEnvelope(t, "evt-retry"))
	if err != nil {
		t.Fatal(err)
	}
	publisher := &outboxPublisher{err: errors.New("broker private error")}
	dispatcher := NewDispatcher(store, publisher, DispatcherOptions{MaxAttempts: 2, RetryDelay: time.Second, LeaseDuration: time.Minute})
	if result, err := dispatcher.RunOnce(ctx, "worker-a", 1); err != nil || result.Retried != 1 {
		t.Fatalf("first dispatch = %+v, %v", result, err)
	}
	now = now.Add(time.Second)
	if result, err := dispatcher.RunOnce(ctx, "worker-a", 1); err != nil || result.DeadLettered != 1 {
		t.Fatalf("second dispatch = %+v, %v", result, err)
	}
	dead, err := store.Get(ctx, record.ID)
	if err != nil || dead.Status != StatusDeadLetter || dead.LastError != SafeDeliveryError {
		t.Fatalf("dead letter = %+v, %v", dead, err)
	}
	replay, err := store.Replay(ctx, record.ID, "operator")
	if err != nil {
		t.Fatal(err)
	}
	if replay.ID == record.ID || replay.Envelope.EventID != record.Envelope.EventID || replay.ReplayOf != record.ID {
		t.Fatalf("replay = %+v, original = %+v", replay, record)
	}
	publisher.err = nil
	if result, err := dispatcher.RunOnce(ctx, "worker-b", 1); err != nil || result.Published != 1 {
		t.Fatalf("replay dispatch = %+v, %v", result, err)
	}
	if len(publisher.messages) != 1 || publisher.messages[0].Headers["event-id"] != record.Envelope.EventID {
		t.Fatalf("published messages = %+v", publisher.messages)
	}
}

func TestEnvelopeRejectsSensitivePayloadAndInvalidTrace(t *testing.T) {
	envelope := testEnvelope(t, "evt-sensitive")
	envelope.Payload = []byte(`{"otp":"123456"}`)
	if _, err := ValidateEnvelope(envelope); !errors.Is(err, ErrInvalidEnvelope) {
		t.Fatalf("ValidateEnvelope(sensitive) error = %v", err)
	}
	envelope = testEnvelope(t, "evt-trace")
	envelope.TraceParent = "invalid"
	if _, err := ValidateEnvelope(envelope); !errors.Is(err, ErrInvalidEnvelope) {
		t.Fatalf("ValidateEnvelope(trace) error = %v", err)
	}
}

func fixedOutboxClock() time.Time {
	return time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC)
}

func testEnvelope(t *testing.T, eventID string) Envelope {
	t.Helper()
	envelope, err := ValidateEnvelope(Envelope{
		EventID:     eventID,
		EventType:   "platform.record.changed.v1",
		Version:     EnvelopeVersion,
		OccurredAt:  fixedOutboxClock(),
		Producer:    "platform-test",
		TenantCode:  "tenant-a",
		TraceParent: "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
		Payload:     []byte(`{"recordId":"record-1"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	return envelope
}

type outboxPublisher struct {
	err      error
	messages []integration.Message
}

func (p *outboxPublisher) Publish(_ context.Context, message integration.Message) error {
	if p.err != nil {
		return p.err
	}
	p.messages = append(p.messages, message)
	return nil
}
