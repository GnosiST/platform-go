package integration

import (
	"context"
	"errors"
	"testing"
)

func TestDisabledIntegrationsRejectOperationsWithoutSideEffects(t *testing.T) {
	ctx := context.Background()
	bus := NewDisabledMessageBus()
	indexer := NewDisabledSearchIndexer()
	reader := NewDisabledSearchReader()

	payload := []byte("unchanged")
	document := []byte("unchanged")
	if err := bus.Publish(ctx, Message{Topic: "orders", Payload: payload}); !errors.Is(err, ErrMessageBusDisabled) {
		t.Fatalf("Publish() error = %v, want ErrMessageBusDisabled", err)
	}
	if err := indexer.Index(ctx, SearchDocument{Collection: "orders", ID: "1", Body: document}); !errors.Is(err, ErrSearchDisabled) {
		t.Fatalf("Index() error = %v, want ErrSearchDisabled", err)
	}
	if err := indexer.Delete(ctx, SearchDocumentRef{Collection: "orders", ID: "1"}); !errors.Is(err, ErrSearchDisabled) {
		t.Fatalf("Delete() error = %v, want ErrSearchDisabled", err)
	}
	if result, err := reader.Search(ctx, SearchRequest{QueryID: "orders.list", Limit: 10}); !errors.Is(err, ErrSearchDisabled) || len(result.Hits) != 0 || result.Total != 0 {
		t.Fatalf("Search() = %+v, %v, want empty result and ErrSearchDisabled", result, err)
	}
	if string(payload) != "unchanged" || string(document) != "unchanged" {
		t.Fatal("disabled integrations mutated caller-owned payloads")
	}
}

func TestRuntimeReportsUnifiedDisabledStatus(t *testing.T) {
	runtime := NewRuntime(nil, nil, nil)
	statuses := runtime.Status(context.Background())
	if len(statuses) != 3 {
		t.Fatalf("Status() count = %d, want 3", len(statuses))
	}
	for _, status := range statuses {
		if status.Enabled || status.State != StateDisabled || status.Adapter != StateDisabled {
			t.Fatalf("status = %+v, want explicitly disabled", status)
		}
	}
}

func TestRuntimeRedactsAdapterHealthErrors(t *testing.T) {
	runtime := NewRuntime(failingMessageBus{}, nil, nil)
	status := runtime.Status(context.Background())[0]
	if !status.Enabled || status.State != StateUnavailable || status.Adapter != "test-bus" {
		t.Fatalf("message bus status = %+v, want unavailable test-bus", status)
	}
}

type failingMessageBus struct{}

func (failingMessageBus) Kind() string { return "test-bus" }
func (failingMessageBus) Health(context.Context) error {
	return errors.New("broker.internal password=secret")
}
func (failingMessageBus) Publish(context.Context, Message) error { return nil }
