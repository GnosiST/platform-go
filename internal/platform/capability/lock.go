package capability

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type LockFile struct {
	Version      int  `json:"version"`
	Capabilities []ID `json:"capabilities"`
}

func LoadLockFile(path string) (LockFile, error) {
	normalizedPath := strings.TrimSpace(path)
	if normalizedPath == "" {
		return LockFile{}, fmt.Errorf("capability lock file path is required")
	}
	data, err := os.ReadFile(normalizedPath)
	if err != nil {
		return LockFile{}, fmt.Errorf("read capability lock file: %w", err)
	}
	var lock LockFile
	if err := json.Unmarshal(data, &lock); err != nil {
		return LockFile{}, fmt.Errorf("parse capability lock file: %w", err)
	}
	if lock.Version != 1 {
		return LockFile{}, fmt.Errorf("capability lock file version must be 1")
	}
	normalized, err := normalizeEnabledCapabilities(lock.Capabilities)
	if err != nil {
		return LockFile{}, err
	}
	lock.Capabilities = normalized
	return lock, nil
}

func IDsToStrings(ids []ID) []string {
	values := make([]string, 0, len(ids))
	for _, id := range ids {
		values = append(values, string(id))
	}
	return values
}
