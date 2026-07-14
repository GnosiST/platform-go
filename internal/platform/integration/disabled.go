package integration

import "context"

type disabledMessageBus struct{}

func NewDisabledMessageBus() MessageBus {
	return disabledMessageBus{}
}

func (disabledMessageBus) Kind() string {
	return StateDisabled
}

func (disabledMessageBus) Health(context.Context) error {
	return ErrMessageBusDisabled
}

func (disabledMessageBus) Publish(context.Context, Message) error {
	return ErrMessageBusDisabled
}

type disabledSearchIndexer struct{}

func NewDisabledSearchIndexer() SearchIndexer {
	return disabledSearchIndexer{}
}

func (disabledSearchIndexer) Kind() string {
	return StateDisabled
}

func (disabledSearchIndexer) Health(context.Context) error {
	return ErrSearchDisabled
}

func (disabledSearchIndexer) Index(context.Context, SearchDocument) error {
	return ErrSearchDisabled
}

func (disabledSearchIndexer) Delete(context.Context, SearchDocumentRef) error {
	return ErrSearchDisabled
}

type disabledSearchReader struct{}

func NewDisabledSearchReader() SearchReader {
	return disabledSearchReader{}
}

func (disabledSearchReader) Kind() string {
	return StateDisabled
}

func (disabledSearchReader) Health(context.Context) error {
	return ErrSearchDisabled
}

func (disabledSearchReader) Search(context.Context, SearchRequest) (SearchResult, error) {
	return SearchResult{}, ErrSearchDisabled
}
