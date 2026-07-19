package adminresource

import (
	"testing"

	"github.com/GnosiST/platform-go/internal/platform/core"
)

func newRepositoryBackedStoreFromDefaultCapabilitiesForTest(t *testing.T, repository AdminResourceRepository) (*Store, error) {
	t.Helper()
	return NewRepositoryBackedStoreFromCapabilitiesWithProtection(repository, core.DefaultManifests(), newTrackingProtectionRuntime(t, 'e', 'i'))
}

func newFileBackedStoreFromDefaultCapabilitiesForTest(t *testing.T, path string) (*Store, error) {
	t.Helper()
	return NewRepositoryBackedStoreFromCapabilitiesWithProtection(NewFileAdminResourceRepository(path), core.DefaultManifests(), newTrackingProtectionRuntime(t, 'e', 'i'))
}
