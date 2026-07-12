package adminresource

import (
	"context"
	"errors"
	"fmt"

	"platform-go/internal/platform/capability"
)

var ErrRevisionConflict = errors.New("admin resource revision conflict")

type RevisionConflictError struct {
	Expected uint64
	Actual   uint64
}

func (e *RevisionConflictError) Error() string {
	return fmt.Sprintf("%s: expected %d, actual %d", ErrRevisionConflict, e.Expected, e.Actual)
}

func (e *RevisionConflictError) Unwrap() error {
	return ErrRevisionConflict
}

type ResourceSnapshot struct {
	Revision  uint64
	NextID    int
	Resources map[string][]Record
}

type AdminResourceRepository interface {
	Load(context.Context) (ResourceSnapshot, error)
	Save(context.Context, ResourceSnapshot) (uint64, error)
}

type AdminResourceRevisionReader interface {
	CurrentRevision(context.Context) (uint64, error)
}

func NewRepositoryBackedStoreFromCapabilities(repository AdminResourceRepository, manifests []capability.Manifest) (*Store, error) {
	baseResources := seedResourcesFromCapabilities(manifests)
	schemas := seedResourceSchemasFromCapabilities(manifests)
	store := newStore(baseResources, schemas)
	store.repository = repository
	if err := store.reloadContextLocked(context.Background()); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *Store) snapshotLocked() ResourceSnapshot {
	return ResourceSnapshot{
		Revision:  s.revision,
		NextID:    s.nextID,
		Resources: cloneResourceMap(s.resources),
	}
}

func (s *Store) restoreSnapshotLocked(snapshot ResourceSnapshot) {
	s.revision = snapshot.Revision
	s.nextID = snapshot.NextID
	s.resources = cloneResourceMap(snapshot.Resources)
}

func (s *Store) installSnapshotLocked(snapshot ResourceSnapshot) {
	resources := mergePersistedResources(s.seedResources, snapshot.Resources, s.schemas)
	s.revision = snapshot.Revision
	s.nextID = max(1000, snapshot.NextID, nextIDFromResources(resources))
	s.resources = resources
}
