package bootstrap

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/GnosiST/platform-go/internal/platform/config"
	"github.com/GnosiST/platform-go/internal/platform/integration"
)

type IntegrationAdapters struct {
	MessageBus    integration.MessageBus
	SearchIndexer integration.SearchIndexer
	SearchReader  integration.SearchReader
}

func IntegrationsFromConfig(cfg config.Config, adapters IntegrationAdapters) (integration.Runtime, error) {
	messageBus := integration.NewDisabledMessageBus()
	searchIndexer := integration.NewDisabledSearchIndexer()
	searchReader := integration.NewDisabledSearchReader()

	if cfg.MessageBusEnabled {
		if err := validateIntegrationAdapter("message bus", cfg.MessageBusAdapter, adapters.MessageBus); err != nil {
			return integration.Runtime{}, err
		}
		messageBus = adapters.MessageBus
	}
	if cfg.SearchEnabled {
		if err := validateIntegrationAdapter("search indexer", cfg.SearchAdapter, adapters.SearchIndexer); err != nil {
			return integration.Runtime{}, err
		}
		if err := validateIntegrationAdapter("search reader", cfg.SearchAdapter, adapters.SearchReader); err != nil {
			return integration.Runtime{}, err
		}
		searchIndexer = adapters.SearchIndexer
		searchReader = adapters.SearchReader
	}

	return integration.NewRuntime(messageBus, searchIndexer, searchReader), nil
}

type namedIntegrationAdapter interface {
	Kind() string
}

func validateIntegrationAdapter(label string, configured string, adapter namedIntegrationAdapter) error {
	kind := strings.ToLower(strings.TrimSpace(configured))
	if kind == "" {
		return fmt.Errorf("%s integration is enabled without a configured adapter", label)
	}
	if interfaceNil(adapter) {
		return fmt.Errorf("%s adapter %q is enabled but not registered", label, kind)
	}
	actual := adapter.Kind()
	if actual != strings.ToLower(strings.TrimSpace(actual)) || actual == "" {
		return fmt.Errorf("%s registered adapter kind must be canonical trimmed lowercase", label)
	}
	if actual != kind {
		return fmt.Errorf("%s registered adapter %q does not match configured adapter %q", label, actual, kind)
	}
	return nil
}

func interfaceNil(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
