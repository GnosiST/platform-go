package bootstrap

import (
	"context"
	"strings"
	"testing"

	"github.com/GnosiST/platform-go/internal/platform/config"
	"github.com/GnosiST/platform-go/internal/platform/integration"
)

func TestIntegrationsFromConfigDefaultsToExplicitDisabledPorts(t *testing.T) {
	runtime, err := IntegrationsFromConfig(config.Config{}, IntegrationAdapters{})
	if err != nil {
		t.Fatalf("IntegrationsFromConfig() error = %v", err)
	}
	for _, status := range runtime.Status(context.Background()) {
		if status.Enabled || status.State != integration.StateDisabled {
			t.Fatalf("status = %+v, want disabled", status)
		}
	}
}

func TestIntegrationsFromConfigRejectsEnabledIntegrationWithoutRegisteredAdapter(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.Config
		want string
	}{
		{name: "message bus", cfg: config.Config{MessageBusEnabled: true, MessageBusAdapter: "nats"}, want: `message bus adapter "nats" is enabled but not registered`},
		{name: "search indexer", cfg: config.Config{SearchEnabled: true, SearchAdapter: "elasticsearch"}, want: `search indexer adapter "elasticsearch" is enabled but not registered`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := IntegrationsFromConfig(tt.cfg, IntegrationAdapters{})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("IntegrationsFromConfig() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestIntegrationsFromConfigRequiresCompleteMatchingSearchAdapter(t *testing.T) {
	cfg := config.Config{SearchEnabled: true, SearchAdapter: "elasticsearch"}
	_, err := IntegrationsFromConfig(cfg, IntegrationAdapters{SearchIndexer: &integrationAdapterStub{kind: "elasticsearch"}})
	if err == nil || !strings.Contains(err.Error(), "search reader") {
		t.Fatalf("IntegrationsFromConfig(indexer only) error = %v", err)
	}

	_, err = IntegrationsFromConfig(cfg, IntegrationAdapters{
		SearchIndexer: &integrationAdapterStub{kind: "elasticsearch"},
		SearchReader:  &integrationAdapterStub{kind: "opensearch"},
	})
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("IntegrationsFromConfig(mismatched reader) error = %v", err)
	}
}

func TestIntegrationsFromConfigDoesNotLetCapabilityProfilesImplicitlyEnableExternalAdapters(t *testing.T) {
	adapter := &integrationAdapterStub{kind: "test-adapter"}
	runtime, err := IntegrationsFromConfig(config.Config{
		Capabilities: []string{"notification", "job"},
	}, IntegrationAdapters{MessageBus: adapter, SearchIndexer: adapter, SearchReader: adapter})
	if err != nil {
		t.Fatalf("IntegrationsFromConfig(profile only) error = %v", err)
	}
	for _, status := range runtime.Status(context.Background()) {
		if status.Enabled || status.State != integration.StateDisabled {
			t.Fatalf("profile-only status = %+v, want disabled", status)
		}
	}
	if adapter.healthCalls != 0 || adapter.publishCalls != 0 || adapter.indexCalls != 0 || adapter.searchCalls != 0 {
		t.Fatalf("profile-only adapter calls = health:%d publish:%d index:%d search:%d, want zero", adapter.healthCalls, adapter.publishCalls, adapter.indexCalls, adapter.searchCalls)
	}
}

func TestIntegrationsFromConfigComposesExplicitAdaptersAndHealth(t *testing.T) {
	adapter := &integrationAdapterStub{kind: "test-adapter"}
	runtime, err := IntegrationsFromConfig(config.Config{
		MessageBusEnabled: true, MessageBusAdapter: "test-adapter",
		SearchEnabled: true, SearchAdapter: "test-adapter",
	}, IntegrationAdapters{MessageBus: adapter, SearchIndexer: adapter, SearchReader: adapter})
	if err != nil {
		t.Fatalf("IntegrationsFromConfig() error = %v", err)
	}
	for _, status := range runtime.Status(context.Background()) {
		if !status.Enabled || status.State != integration.StateReady || status.Adapter != "test-adapter" {
			t.Fatalf("enabled status = %+v, want ready test-adapter", status)
		}
	}
}

type integrationAdapterStub struct {
	kind         string
	healthCalls  int
	publishCalls int
	indexCalls   int
	searchCalls  int
}

func (s *integrationAdapterStub) Kind() string { return s.kind }
func (s *integrationAdapterStub) Health(context.Context) error {
	s.healthCalls++
	return nil
}
func (s *integrationAdapterStub) Publish(context.Context, integration.Message) error {
	s.publishCalls++
	return nil
}
func (s *integrationAdapterStub) Index(context.Context, integration.SearchDocument) error {
	s.indexCalls++
	return nil
}
func (s *integrationAdapterStub) Delete(context.Context, integration.SearchDocumentRef) error {
	return nil
}
func (s *integrationAdapterStub) Search(context.Context, integration.SearchRequest) (integration.SearchResult, error) {
	s.searchCalls++
	return integration.SearchResult{}, nil
}
