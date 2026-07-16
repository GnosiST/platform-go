package adminresource

import (
	"context"
	"errors"
	"fmt"

	"github.com/GnosiST/platform-go/internal/platform/capability"
	"github.com/GnosiST/platform-go/internal/platform/dataprotection"
)

var ErrRevisionConflict = errors.New("admin resource revision conflict")
var ErrDomainOwnedMutation = errors.New("admin resource is owned by a domain repository")

func RequiresGovernedLifecycleCommand(resource string) bool {
	switch resource {
	case "org-units", "role-groups", "roles", "users", "menus", "permissions":
		return true
	default:
		return false
	}
}

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

type AdminResourceCapabilitySeedPolicy interface {
	ExcludeCapabilitySeed(resource string) bool
}

func NewRepositoryBackedStoreFromCapabilities(repository AdminResourceRepository, manifests []capability.Manifest) (*Store, error) {
	baseResources := repositoryCapabilitySeeds(repository, manifests)
	schemas := seedResourceSchemasFromCapabilities(manifests)
	store := newStore(baseResources, schemas)
	store.repository = repository
	if err := store.validateProtectionRuntime(); err != nil {
		return nil, err
	}
	if err := store.reloadContextLocked(context.Background()); err != nil {
		return nil, err
	}
	return store, nil
}

func NewRepositoryBackedStoreFromCapabilitiesWithProtection(repository AdminResourceRepository, manifests []capability.Manifest, runtime dataprotection.Runtime) (*Store, error) {
	baseResources := repositoryCapabilitySeeds(repository, manifests)
	schemas := seedResourceSchemasFromCapabilities(manifests)
	store := newStore(baseResources, schemas)
	store.repository = repository
	store.protection = runtime
	if err := store.validateProtectionRuntime(); err != nil {
		return nil, err
	}
	if err := store.protectSeedResources(context.Background()); err != nil {
		return nil, err
	}
	if err := store.reloadContextLocked(context.Background()); err != nil {
		return nil, err
	}
	if err := store.validateProtectedDataLocked(context.Background()); err != nil {
		return nil, err
	}
	return store, nil
}

func repositoryCapabilitySeeds(repository AdminResourceRepository, manifests []capability.Manifest) map[string][]Record {
	resources := seedResourcesFromCapabilities(manifests)
	policy, ok := repository.(AdminResourceCapabilitySeedPolicy)
	if !ok {
		return resources
	}
	for resource := range resources {
		if policy.ExcludeCapabilitySeed(resource) {
			resources[resource] = nil
		}
	}
	return resources
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
