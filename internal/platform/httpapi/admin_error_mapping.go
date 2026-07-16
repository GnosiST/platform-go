package httpapi

import (
	"errors"

	"github.com/gin-gonic/gin"

	"github.com/GnosiST/platform-go/internal/platform/adminresource"
	"github.com/GnosiST/platform-go/internal/platform/errorcode"
)

func adminResourceErrorCode(err error) errorcode.Code {
	switch {
	case errors.Is(err, adminresource.ErrUnknownResource):
		return errorcode.CodeAdminResourceNotFound
	case errors.Is(err, adminresource.ErrRecordNotFound):
		return errorcode.CodeAdminResourceRecordNotFound
	case errors.Is(err, adminresource.ErrRevisionConflict):
		return errorcode.CodeAdminResourceRevisionConflict
	case errors.Is(err, adminresource.ErrDomainOwnedMutation):
		return errorcode.CodeAdminResourceDomainOwnedMutation
	case errors.Is(err, adminresource.ErrDeletionDisabled),
		errors.Is(err, adminresource.ErrDeletionRequiresAdapter),
		errors.Is(err, adminresource.ErrDeletionCleanupStarted),
		errors.Is(err, adminresource.ErrRecordDeleted),
		errors.Is(err, adminresource.ErrRecordNotDeleted),
		errors.Is(err, adminresource.ErrRecordReferenced),
		errors.Is(err, adminresource.ErrRestoreWindowExpired),
		errors.Is(err, adminresource.ErrRetentionNotConfigured),
		errors.Is(err, adminresource.ErrRetentionNotElapsed):
		return errorcode.CodeAdminResourceLifecycleConflict
	case errors.Is(err, adminresource.ErrInvalidRecord):
		return errorcode.CodeAdminResourceInvalidRecord
	default:
		return errorcode.CodeAdminResourceError
	}
}

func writeAdminResourceError(ctx *gin.Context, sink InternalErrorSink, err error) {
	code := adminResourceErrorCode(err)
	if code == errorcode.CodeAdminResourceError {
		writePlatformErrorWithCause(ctx, sink, code, err)
		return
	}
	writePlatformError(ctx, code)
}
