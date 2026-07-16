package apps

import (
	"github.com/GnosiST/platform-go/internal/platform/adminresource"
	"github.com/GnosiST/platform-go/internal/platform/adminroute"
)

func DefaultAdminRoutes(resources *adminresource.Store) []adminroute.Registration {
	return []adminroute.Registration{}
}
