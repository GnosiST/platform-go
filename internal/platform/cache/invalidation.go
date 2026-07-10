package cache

import (
	"context"
	"strings"
	"sync"
)

type InvalidationEvent struct {
	Resource string `json:"resource"`
}

type InvalidationHandler func(context.Context, InvalidationEvent)

type InvalidationBus interface {
	PublishInvalidation(context.Context, InvalidationEvent) error
	SubscribeInvalidations(context.Context, InvalidationHandler) error
}

type NoopInvalidationBus struct{}

func NewNoopInvalidationBus() NoopInvalidationBus {
	return NoopInvalidationBus{}
}

func (NoopInvalidationBus) PublishInvalidation(context.Context, InvalidationEvent) error {
	return nil
}

func (NoopInvalidationBus) SubscribeInvalidations(context.Context, InvalidationHandler) error {
	return nil
}

type MemoryInvalidationBus struct {
	mu       sync.Mutex
	handlers []InvalidationHandler
}

func NewMemoryInvalidationBus() *MemoryInvalidationBus {
	return &MemoryInvalidationBus{}
}

func (b *MemoryInvalidationBus) PublishInvalidation(ctx context.Context, event InvalidationEvent) error {
	if strings.TrimSpace(event.Resource) == "" {
		return nil
	}
	b.mu.Lock()
	handlers := append([]InvalidationHandler(nil), b.handlers...)
	b.mu.Unlock()
	for _, handler := range handlers {
		handler(ctx, event)
	}
	return nil
}

func (b *MemoryInvalidationBus) SubscribeInvalidations(_ context.Context, handler InvalidationHandler) error {
	if handler == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers = append(b.handlers, handler)
	return nil
}
