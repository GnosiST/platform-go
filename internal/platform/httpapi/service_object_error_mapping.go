package httpapi

import (
	"errors"

	"github.com/gin-gonic/gin"

	"github.com/GnosiST/platform-go/internal/platform/errorcode"
	"github.com/GnosiST/platform-go/internal/platform/serviceobject"
)

func serviceObjectErrorCode(err error) errorcode.Code {
	switch {
	case errors.Is(err, serviceobject.ErrObjectUnavailable):
		return errorcode.CodeServiceObjectUnavailable
	case errors.Is(err, serviceobject.ErrRequestInvalid):
		return errorcode.CodeServiceObjectRequestInvalid
	case errors.Is(err, serviceobject.ErrCostLimitExceeded):
		return errorcode.CodeServiceObjectCostLimit
	case errors.Is(err, serviceobject.ErrIdempotencyConflict):
		return errorcode.CodeServiceObjectIdempotencyConflict
	case errors.Is(err, serviceobject.ErrConflict):
		return errorcode.CodeServiceObjectStateConflict
	case errors.Is(err, serviceobject.ErrValidation):
		return errorcode.CodeServiceObjectDomainValidation
	default:
		return errorcode.CodeServiceObjectExecutionFailed
	}
}

func writeServiceObjectError(ctx *gin.Context, sink InternalErrorSink, err error) {
	code := serviceObjectErrorCode(err)
	if code == errorcode.CodeServiceObjectExecutionFailed {
		writePlatformErrorWithCause(ctx, sink, code, err)
		return
	}
	writePlatformError(ctx, code)
}
