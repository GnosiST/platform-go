package apps

import (
	"platform-go/internal/platform/adminresource"
	"platform-go/internal/platform/adminroute"
)

func DefaultAdminRoutes(resources *adminresource.Store) []adminroute.Registration {
	return []adminroute.Registration{}
}
