package apps

import (
	"testing"

	"github.com/GnosiST/platform-go/internal/platform/adminresource"
)

func TestDefaultAppRoutesAreBusinessNeutral(t *testing.T) {
	registrations := DefaultAppRoutes(adminresource.NewStore())
	if len(registrations) != 0 {
		t.Fatalf("DefaultAppRoutes() = %+v, want no built-in business routes", registrations)
	}
}
