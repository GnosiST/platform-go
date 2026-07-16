package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/GnosiST/platform-go/internal/platform/errorcode"
	"github.com/GnosiST/platform-go/internal/platform/serviceobject"
)

func TestServiceObjectErrorCodeMatrix(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code errorcode.Code
	}{
		{name: "unavailable", err: serviceobject.ErrObjectUnavailable, code: errorcode.CodeServiceObjectUnavailable},
		{name: "wrapped invalid request", err: fmt.Errorf("%w: physical_column=secret", serviceobject.ErrRequestInvalid), code: errorcode.CodeServiceObjectRequestInvalid},
		{name: "cost limit", err: serviceobject.ErrCostLimitExceeded, code: errorcode.CodeServiceObjectCostLimit},
		{name: "idempotency conflict", err: serviceobject.ErrIdempotencyConflict, code: errorcode.CodeServiceObjectIdempotencyConflict},
		{name: "state conflict", err: serviceobject.ErrConflict, code: errorcode.CodeServiceObjectStateConflict},
		{name: "domain validation", err: serviceobject.ErrValidation, code: errorcode.CodeServiceObjectDomainValidation},
		{name: "execution failed", err: serviceobject.ErrExecutionFailed, code: errorcode.CodeServiceObjectExecutionFailed},
		{name: "unknown failure", err: errors.New("repository failed"), code: errorcode.CodeServiceObjectExecutionFailed},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := serviceObjectErrorCode(test.err); got != test.code {
				t.Fatalf("serviceObjectErrorCode(%v) = %s, want %s", test.err, got, test.code)
			}
		})
	}
}

func TestServiceObjectErrorAdapterUsesRegistryAndNeverLeaksCause(t *testing.T) {
	const marker = "physical_table=secret_records email=marker@example.test"
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   errorcode.Code
		wantEvents int
	}{
		{name: "request invalid", err: fmt.Errorf("%w: %s", serviceobject.ErrRequestInvalid, marker), wantStatus: http.StatusBadRequest, wantCode: errorcode.CodeServiceObjectRequestInvalid},
		{name: "execution failed", err: fmt.Errorf("%w: %s", serviceobject.ErrExecutionFailed, marker), wantStatus: http.StatusInternalServerError, wantCode: errorcode.CodeServiceObjectExecutionFailed, wantEvents: 1},
		{name: "unknown execution failure", err: errors.New(marker), wantStatus: http.StatusInternalServerError, wantCode: errorcode.CodeServiceObjectExecutionFailed, wantEvents: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sink := &errorResponseSink{}
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/api/admin/service-objects/query", nil)

			writeServiceObjectError(ctx, sink, test.err)

			if recorder.Code != test.wantStatus || !strings.Contains(recorder.Body.String(), `"code":"`+string(test.wantCode)+`"`) {
				t.Fatalf("status/body = %d/%s, want %d/%s", recorder.Code, recorder.Body.String(), test.wantStatus, test.wantCode)
			}
			if strings.Contains(recorder.Body.String(), "physical_table") || strings.Contains(recorder.Body.String(), "marker@example.test") {
				t.Fatalf("response leaked cause: %s", recorder.Body.String())
			}
			if len(sink.events) != test.wantEvents {
				t.Fatalf("events = %+v, want %d", sink.events, test.wantEvents)
			}
		})
	}
}
