package httpapi

import (
	"errors"

	"github.com/gin-gonic/gin"

	"github.com/GnosiST/platform-go/internal/platform/adminresource"
	"github.com/GnosiST/platform-go/internal/platform/errorcode"
	"github.com/GnosiST/platform-go/internal/platform/sensitivereveal"
)

func sensitiveRevealErrorCode(err error) errorcode.Code {
	switch {
	case errors.Is(err, adminresource.ErrRecordNotFound),
		errors.Is(err, sensitivereveal.ErrPolicyNotFound),
		errors.Is(err, sensitivereveal.ErrChallengeNotFound),
		errors.Is(err, sensitivereveal.ErrFactorTransactionNotFound),
		errors.Is(err, sensitivereveal.ErrGrantNotFound):
		return errorcode.CodeAdminSensitiveRevealNotFound
	case errors.Is(err, sensitivereveal.ErrChallengeExpired),
		errors.Is(err, sensitivereveal.ErrChallengeClosed),
		errors.Is(err, sensitivereveal.ErrGrantExpired),
		errors.Is(err, sensitivereveal.ErrGrantConsumed),
		errors.Is(err, sensitivereveal.ErrFactorLocked):
		return errorcode.CodeAdminSensitiveRevealExpired
	case errors.Is(err, sensitivereveal.ErrFactorAlreadyStarted),
		errors.Is(err, sensitivereveal.ErrFactorAlreadyCompleted),
		errors.Is(err, sensitivereveal.ErrRevealResultRecorded):
		return errorcode.CodeAdminSensitiveRevealConflict
	case errors.Is(err, sensitivereveal.ErrVerificationFailed),
		errors.Is(err, ErrAdminIdentityInvalid),
		errors.Is(err, ErrAdminIdentityTransaction),
		errors.Is(err, ErrAdminIdentityBindingInvalid):
		return errorcode.CodeAdminSensitiveRevealVerificationFailed
	case errors.Is(err, sensitivereveal.ErrScopeMismatch),
		errors.Is(err, sensitivereveal.ErrPurposeNotAllowed),
		errors.Is(err, sensitivereveal.ErrFactorNotAllowed):
		return errorcode.CodeAdminForbidden
	case errors.Is(err, ErrAdminIdentityProviderExchange):
		return errorcode.CodeAdminSensitiveRevealProviderFailed
	case errors.Is(err, adminresource.ErrProtectedFieldUnavailable),
		errors.Is(err, adminresource.ErrProtectedFieldDecryptionFailed):
		return errorcode.CodeAdminSensitiveRevealUnavailable
	case errors.Is(err, sensitivereveal.ErrInvalidScope),
		errors.Is(err, adminresource.ErrInvalidRecord):
		return errorcode.CodeAdminSensitiveRevealInvalid
	default:
		return errorcode.CodeAdminSensitiveRevealFailed
	}
}

func writeSensitiveRevealError(ctx *gin.Context, sink InternalErrorSink, err error) {
	noStoreSensitiveReveal(ctx)
	code := sensitiveRevealErrorCode(err)
	switch code {
	case errorcode.CodeAdminSensitiveRevealProviderFailed,
		errorcode.CodeAdminSensitiveRevealUnavailable,
		errorcode.CodeAdminSensitiveRevealFailed:
		writePlatformErrorWithCause(ctx, sink, code, err)
	default:
		writePlatformError(ctx, code)
	}
}

func writeSensitiveRevealUnavailable(ctx *gin.Context, sink InternalErrorSink, cause error) {
	noStoreSensitiveReveal(ctx)
	writePlatformErrorWithCause(ctx, sink, errorcode.CodeAdminSensitiveRevealUnavailable, cause)
}
