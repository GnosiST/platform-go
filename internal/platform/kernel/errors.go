package kernel

import "fmt"

const (
	ErrCodeInvalidExecutionContext = "INVALID_EXECUTION_CONTEXT"
	ErrCodeValidation              = "VALIDATION_ERROR"
	ErrCodeNotFound                = "NOT_FOUND"
	ErrCodeConflict                = "CONFLICT"
	ErrCodeForbidden               = "FORBIDDEN"
	ErrCodeInternal                = "INTERNAL_ERROR"
)

type Error struct {
	Code    string
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func NewError(code string, message string) *Error {
	return &Error{Code: code, Message: message}
}

func WrapError(code string, message string, cause error) *Error {
	return &Error{Code: code, Message: message, Cause: cause}
}

func IsCode(err error, code string) bool {
	platformErr, ok := err.(*Error)
	return ok && platformErr.Code == code
}
