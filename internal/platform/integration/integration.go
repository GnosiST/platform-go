package integration

import (
	"context"
	"errors"
)

const (
	CapabilityMessageBus    = "message-bus"
	CapabilitySearchIndexer = "search-indexer"
	CapabilitySearchReader  = "search-reader"

	StateDisabled    = "disabled"
	StateReady       = "ready"
	StateUnavailable = "unavailable"
)

var (
	ErrMessageBusDisabled = errors.New("message bus integration is disabled")
	ErrSearchDisabled     = errors.New("search integration is disabled")
)

type Message struct {
	Topic   string
	Key     string
	Payload []byte
	Headers map[string]string
}

type SearchDocument struct {
	Collection string
	ID         string
	TenantID   string
	Body       []byte
}

type SearchDocumentRef struct {
	Collection string
	ID         string
	TenantID   string
}

type SearchRequest struct {
	QueryID   string
	Version   string
	TenantID  string
	Arguments map[string]any
	Offset    int
	Limit     int
}

type SearchHit struct {
	ID   string
	Body []byte
}

type SearchResult struct {
	Hits  []SearchHit
	Total int64
}

type MessageBus interface {
	Kind() string
	Health(context.Context) error
	Publish(context.Context, Message) error
}

type SearchIndexer interface {
	Kind() string
	Health(context.Context) error
	Index(context.Context, SearchDocument) error
	Delete(context.Context, SearchDocumentRef) error
}

type SearchReader interface {
	Kind() string
	Health(context.Context) error
	Search(context.Context, SearchRequest) (SearchResult, error)
}

type CapabilityStatus struct {
	Capability string `json:"capability"`
	Enabled    bool   `json:"enabled"`
	State      string `json:"state"`
	Adapter    string `json:"adapter"`
}

type Runtime struct {
	MessageBus    MessageBus
	SearchIndexer SearchIndexer
	SearchReader  SearchReader
}

func NewRuntime(messageBus MessageBus, searchIndexer SearchIndexer, searchReader SearchReader) Runtime {
	if messageBus == nil {
		messageBus = NewDisabledMessageBus()
	}
	if searchIndexer == nil {
		searchIndexer = NewDisabledSearchIndexer()
	}
	if searchReader == nil {
		searchReader = NewDisabledSearchReader()
	}
	return Runtime{MessageBus: messageBus, SearchIndexer: searchIndexer, SearchReader: searchReader}
}

func (r Runtime) Status(ctx context.Context) []CapabilityStatus {
	return []CapabilityStatus{
		status(ctx, CapabilityMessageBus, r.MessageBus),
		status(ctx, CapabilitySearchIndexer, r.SearchIndexer),
		status(ctx, CapabilitySearchReader, r.SearchReader),
	}
}

type healthAdapter interface {
	Kind() string
	Health(context.Context) error
}

func status(ctx context.Context, capability string, adapter healthAdapter) CapabilityStatus {
	if adapter == nil {
		return CapabilityStatus{Capability: capability, State: StateUnavailable}
	}
	result := CapabilityStatus{Capability: capability, Enabled: true, State: StateReady, Adapter: adapter.Kind()}
	if err := adapter.Health(ctx); err != nil {
		result.State = StateUnavailable
		if errors.Is(err, ErrMessageBusDisabled) || errors.Is(err, ErrSearchDisabled) {
			result.Enabled = false
			result.State = StateDisabled
		}
	}
	return result
}
