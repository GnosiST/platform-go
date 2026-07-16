package adminresource

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/GnosiST/platform-go/internal/platform/capability"
)

type fileStoreSnapshot struct {
	Version   int                 `json:"version"`
	Revision  uint64              `json:"revision,omitempty"`
	NextID    int                 `json:"nextId"`
	Resources map[string][]Record `json:"resources"`
}

type FileAdminResourceRepository struct {
	path string
}

func NewFileAdminResourceRepository(path string) *FileAdminResourceRepository {
	return &FileAdminResourceRepository{path: strings.TrimSpace(path)}
}

func NewFileBackedStoreFromCapabilities(path string, manifests []capability.Manifest) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		return NewStoreFromCapabilities(manifests), nil
	}
	return NewRepositoryBackedStoreFromCapabilities(NewFileAdminResourceRepository(path), manifests)
}

func (r *FileAdminResourceRepository) Load(context.Context) (ResourceSnapshot, error) {
	content, err := os.ReadFile(r.path)
	if errors.Is(err, os.ErrNotExist) {
		return ResourceSnapshot{Resources: map[string][]Record{}}, nil
	}
	if err != nil {
		return ResourceSnapshot{}, err
	}
	var snapshot fileStoreSnapshot
	if err := json.Unmarshal(content, &snapshot); err != nil {
		return ResourceSnapshot{}, err
	}
	if snapshot.Resources == nil {
		snapshot.Resources = map[string][]Record{}
	}
	return ResourceSnapshot{Revision: snapshot.Revision, NextID: snapshot.NextID, Resources: snapshot.Resources}, nil
}

func (r *FileAdminResourceRepository) Save(_ context.Context, snapshot ResourceSnapshot) (uint64, error) {
	nextRevision := snapshot.Revision + 1
	if r.path == "" {
		return nextRevision, nil
	}
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return 0, err
	}
	fileSnapshot := fileStoreSnapshot{
		Version:   1,
		Revision:  nextRevision,
		NextID:    snapshot.NextID,
		Resources: cloneResourceMap(snapshot.Resources),
	}
	content, err := json.MarshalIndent(fileSnapshot, "", "  ")
	if err != nil {
		return 0, err
	}
	tempPath := r.path + ".tmp"
	if err := os.WriteFile(tempPath, content, 0o600); err != nil {
		return 0, err
	}
	if err := os.Rename(tempPath, r.path); err != nil {
		return 0, err
	}
	if err := os.Chmod(r.path, 0o600); err != nil {
		return 0, err
	}
	return nextRevision, nil
}

func mergePersistedResources(base map[string][]Record, persisted map[string][]Record, schemas map[string]Schema) map[string][]Record {
	merged := cloneResourceMap(base)
	for resource, records := range persisted {
		if _, ok := schemas[resource]; !ok {
			continue
		}
		merged[resource] = cloneRecords(records)
	}
	addMissingRecordsByID(merged, "menus", base["menus"])
	addMissingRecordsByID(merged, "permissions", base["permissions"])
	return merged
}

func addMissingRecordsByID(resources map[string][]Record, resource string, records []Record) {
	existing := map[string]struct{}{}
	for _, record := range resources[resource] {
		existing[record.ID] = struct{}{}
	}
	for _, record := range records {
		if _, ok := existing[record.ID]; ok {
			continue
		}
		resources[resource] = append(resources[resource], cloneRecord(record))
	}
}

func cloneResourceMap(resources map[string][]Record) map[string][]Record {
	cloned := make(map[string][]Record, len(resources))
	for resource, records := range resources {
		cloned[resource] = cloneRecords(records)
	}
	return cloned
}

func nextIDFromResources(resources map[string][]Record) int {
	nextID := 1000
	for resource, records := range resources {
		prefix := resource + "-"
		for _, record := range records {
			if !strings.HasPrefix(record.ID, prefix) {
				continue
			}
			value, err := strconv.Atoi(strings.TrimPrefix(record.ID, prefix))
			if err == nil && value > nextID {
				nextID = value
			}
		}
	}
	return nextID
}
