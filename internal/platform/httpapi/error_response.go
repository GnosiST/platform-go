package httpapi

import (
	"context"

	"github.com/gin-gonic/gin"

	"platform-go/internal/platform/errorcode"
	"platform-go/internal/platform/kernel"
)

func writePlatformError(ctx *gin.Context, code errorcode.Code) {
	definition := registeredErrorDefinition(code)
	writeRegisteredError(ctx, definition)
}

func writePlatformErrorWithCause(ctx *gin.Context, sink InternalErrorSink, code errorcode.Code, _ error) {
	definition := registeredErrorDefinition(code)
	correlation := correlationFromGinContext(ctx)
	publicErr := errorcode.New(definition.Code)
	event := InternalErrorEvent{
		Code:           string(definition.Code),
		CauseClass:     internalErrorCauseClass(string(definition.Code)),
		EventID:        internalErrorEventID(ctx),
		Err:            publicErr,
		Owner:          definition.Owner,
		Category:       definition.Category,
		RetryPolicy:    definition.RetryPolicy,
		RedactionClass: definition.RedactionClass,
		RequestID:      correlation.RequestID,
		TraceID:        correlation.TraceID,
	}
	_ = ctx.Error(publicErr).SetMeta(event)
	if sink != nil {
		recordContext := context.Background()
		if ctx.Request != nil {
			recordContext = ctx.Request.Context()
		}
		sink.Record(recordContext, event)
	}
	writeRegisteredError(ctx, definition)
}

func recoveryMiddleware(sink InternalErrorSink) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		defer func() {
			if recover() != nil {
				writePlatformErrorWithCause(ctx, sink, errorcode.CodeInternal, nil)
			}
		}()
		ctx.Next()
	}
}

func registeredErrorDefinition(code errorcode.Code) errorcode.Definition {
	if definition, ok := errorcode.Lookup(code); ok {
		return definition
	}
	definition, _ := errorcode.Lookup(errorcode.CodeInternal)
	return definition
}

func writeRegisteredError(ctx *gin.Context, definition errorcode.Definition) {
	correlation := correlationFromGinContext(ctx)
	ctx.AbortWithStatusJSON(definition.HTTPStatus, Response[any]{Error: &ErrorBody{
		Code:      definition.Code,
		Message:   definition.PublicMessage,
		RequestID: correlation.RequestID,
		TraceID:   correlation.TraceID,
	}})
}

func legacyErrorBody(ctx *gin.Context, code string, message string) *ErrorBody {
	correlation := correlationFromGinContext(ctx)
	return &ErrorBody{
		Code:      errorcode.Code(code),
		Message:   message,
		RequestID: correlation.RequestID,
		TraceID:   correlation.TraceID,
	}
}

func correlationFromGinContext(ctx *gin.Context) kernel.Correlation {
	if ctx == nil || ctx.Request == nil {
		return kernel.Correlation{}
	}
	correlation, _ := kernel.CorrelationFromContext(ctx.Request.Context())
	return correlation
}
