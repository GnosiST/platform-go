package apps

import (
	"platform-go/internal/platform/adminresource"
	"platform-go/internal/platform/approute"
)

func DefaultAppRoutes(resources *adminresource.Store) []approute.Registration {
	return []approute.Registration{}
}
