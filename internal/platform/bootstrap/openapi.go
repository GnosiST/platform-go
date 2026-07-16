package bootstrap

import (
	"encoding/json"
	"errors"
	"os"
	"strings"

	"github.com/GnosiST/platform-go/internal/platform/config"
)

func OpenAPIDocumentFromConfig(cfg config.Config) ([]byte, error) {
	path := strings.TrimSpace(cfg.OpenAPIFile)
	if path == "" {
		return nil, nil
	}
	document, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var decoded map[string]any
	if err := json.Unmarshal(document, &decoded); err != nil {
		return nil, errors.New("openapi document must be valid json")
	}
	return document, nil
}
