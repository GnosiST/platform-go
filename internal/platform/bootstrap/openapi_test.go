package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"platform-go/internal/platform/config"
)

func TestOpenAPIDocumentFromConfigLoadsJSONDocument(t *testing.T) {
	path := filepath.Join(t.TempDir(), "openapi.json")
	document := []byte(`{"openapi":"3.1.0","info":{"title":"Platform Admin API","version":"0.1.0"},"paths":{}}`)
	if err := os.WriteFile(path, document, 0o600); err != nil {
		t.Fatalf("write openapi fixture: %v", err)
	}

	loaded, err := OpenAPIDocumentFromConfig(config.Config{OpenAPIFile: path})

	if err != nil {
		t.Fatalf("OpenAPIDocumentFromConfig() error = %v", err)
	}
	if string(loaded) != string(document) {
		t.Fatalf("loaded document = %s, want %s", string(loaded), string(document))
	}
}

func TestOpenAPIDocumentFromConfigAllowsDisabledDocument(t *testing.T) {
	loaded, err := OpenAPIDocumentFromConfig(config.Config{})

	if err != nil {
		t.Fatalf("OpenAPIDocumentFromConfig() error = %v", err)
	}
	if loaded != nil {
		t.Fatalf("loaded document = %s, want nil", string(loaded))
	}
}

func TestOpenAPIDocumentFromConfigRejectsInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "openapi.json")
	if err := os.WriteFile(path, []byte(`{"openapi":`), 0o600); err != nil {
		t.Fatalf("write openapi fixture: %v", err)
	}

	_, err := OpenAPIDocumentFromConfig(config.Config{OpenAPIFile: path})

	if err == nil {
		t.Fatalf("OpenAPIDocumentFromConfig() error = nil, want invalid JSON")
	}
	if !strings.Contains(err.Error(), "openapi document must be valid json") {
		t.Fatalf("OpenAPIDocumentFromConfig() error = %v", err)
	}
}
