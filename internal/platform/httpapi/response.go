package httpapi

import "platform-go/internal/platform/errorcode"

type Response[T any] struct {
	Data  T          `json:"data,omitempty"`
	Error *ErrorBody `json:"error,omitempty"`
}

type ErrorBody struct {
	Code      errorcode.Code `json:"code"`
	Message   string         `json:"message"`
	RequestID string         `json:"requestId"`
	TraceID   string         `json:"traceId"`
}
