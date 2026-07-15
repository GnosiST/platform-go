package serviceobject

import "errors"

var (
	ErrDefinitionInvalid   = errors.New("service object definition is invalid")
	ErrDefinitionConflict  = errors.New("service object definition conflicts with an existing version")
	ErrObjectUnavailable   = errors.New("service object is unavailable")
	ErrRequestInvalid      = errors.New("service object request is invalid")
	ErrCostLimitExceeded   = errors.New("service object cost limit exceeded")
	ErrExecutionFailed     = errors.New("service object execution failed")
	ErrIdempotencyConflict = errors.New("service object idempotency conflict")
	ErrConflict            = errors.New("service object state conflict")
	ErrValidation          = errors.New("service object domain validation failed")
)
