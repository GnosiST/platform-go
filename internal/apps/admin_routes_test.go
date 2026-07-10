package apps

import (
	"testing"

	"platform-go/internal/platform/adminresource"
)

func TestDefaultAdminRoutesAreBusinessNeutral(t *testing.T) {
	registrations := DefaultAdminRoutes(adminresource.NewStore())
	if len(registrations) != 0 {
		t.Fatalf("DefaultAdminRoutes() = %+v, want no built-in business routes", registrations)
	}
}
