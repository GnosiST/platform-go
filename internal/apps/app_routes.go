package apps

import (
	"github.com/GnosiST/platform-go/internal/platform/adminresource"
	"github.com/GnosiST/platform-go/internal/platform/approute"
)

func DefaultAppRoutes(resources *adminresource.Store) []approute.Registration {
	return []approute.Registration{}
}
