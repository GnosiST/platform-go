// Package capability exposes stable business-neutral capability contracts for
// downstream modules. Runtime implementation remains under internal/.
package capability

import internal "github.com/GnosiST/platform-go/internal/platform/capability"

type (
	ID                  = internal.ID
	Manifest            = internal.Manifest
	LocalizedText       = internal.LocalizedText
	AdminSurface        = internal.AdminSurface
	AdminResource       = internal.AdminResource
	AdminMenu           = internal.AdminMenu
	AdminField          = internal.AdminField
	AdminFormGroup      = internal.AdminFormGroup
	AdminResourceAction = internal.AdminResourceAction
)
