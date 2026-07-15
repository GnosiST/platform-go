package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"platform-go/internal/platform/adminresource"
	"platform-go/internal/platform/errorcode"
)

func TestAdminResourceErrorCodeMatrix(t *testing.T) {
	lifecycleErrors := []error{
		adminresource.ErrDeletionDisabled,
		adminresource.ErrDeletionRequiresAdapter,
		adminresource.ErrDeletionCleanupStarted,
		adminresource.ErrRecordDeleted,
		adminresource.ErrRecordNotDeleted,
		adminresource.ErrRecordReferenced,
		adminresource.ErrRestoreWindowExpired,
		adminresource.ErrRetentionNotConfigured,
		adminresource.ErrRetentionNotElapsed,
	}
	tests := []struct {
		name string
		err  error
		code errorcode.Code
	}{
		{name: "unknown resource", err: adminresource.ErrUnknownResource, code: errorcode.CodeAdminResourceNotFound},
		{name: "record not found", err: adminresource.ErrRecordNotFound, code: errorcode.CodeAdminResourceRecordNotFound},
		{name: "revision conflict", err: &adminresource.RevisionConflictError{Expected: 3, Actual: 4}, code: errorcode.CodeAdminResourceRevisionConflict},
		{name: "domain-owned mutation", err: fmt.Errorf("%w: users", adminresource.ErrDomainOwnedMutation), code: errorcode.CodeAdminResourceDomainOwnedMutation},
		{name: "invalid record", err: fmt.Errorf("%w: physical_table=users", adminresource.ErrInvalidRecord), code: errorcode.CodeAdminResourceInvalidRecord},
		{name: "deletion policy missing remains generic", err: adminresource.ErrDeletionPolicyMissing, code: errorcode.CodeAdminResourceError},
		{name: "retention policy mismatch remains generic", err: adminresource.ErrRetentionPolicyMismatch, code: errorcode.CodeAdminResourceError},
		{name: "unknown failure", err: errors.New("driver unavailable"), code: errorcode.CodeAdminResourceError},
	}
	for _, err := range lifecycleErrors {
		tests = append(tests, struct {
			name string
			err  error
			code errorcode.Code
		}{name: err.Error(), err: fmt.Errorf("%w: internal detail", err), code: errorcode.CodeAdminResourceLifecycleConflict})
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := adminResourceErrorCode(test.err); got != test.code {
				t.Fatalf("adminResourceErrorCode(%v) = %s, want %s", test.err, got, test.code)
			}
		})
	}
}

func TestAdminResourceErrorAdapterUsesRegistryAndNeverLeaksCause(t *testing.T) {
	const marker = "physical_table=users email=marker@example.test driver=postgres address=private"
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   errorcode.Code
		wantEvents int
	}{
		{name: "wrapped invalid record", err: fmt.Errorf("%w: %s", adminresource.ErrInvalidRecord, marker), wantStatus: http.StatusBadRequest, wantCode: errorcode.CodeAdminResourceInvalidRecord},
		{name: "wrapped lifecycle conflict", err: fmt.Errorf("%w: %s", adminresource.ErrRecordReferenced, marker), wantStatus: http.StatusConflict, wantCode: errorcode.CodeAdminResourceLifecycleConflict},
		{name: "unexpected storage failure", err: errors.New(marker), wantStatus: http.StatusInternalServerError, wantCode: errorcode.CodeAdminResourceError, wantEvents: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sink := &errorResponseSink{}
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/api/admin/resources/users", nil)

			writeAdminResourceError(ctx, sink, test.err)

			if recorder.Code != test.wantStatus || !strings.Contains(recorder.Body.String(), `"code":"`+string(test.wantCode)+`"`) {
				t.Fatalf("status/body = %d/%s, want %d/%s", recorder.Code, recorder.Body.String(), test.wantStatus, test.wantCode)
			}
			if strings.Contains(recorder.Body.String(), "physical_table") || strings.Contains(recorder.Body.String(), "marker@example.test") || strings.Contains(recorder.Body.String(), "driver=postgres") || strings.Contains(recorder.Body.String(), "address=private") {
				t.Fatalf("response leaked cause: %s", recorder.Body.String())
			}
			if len(sink.events) != test.wantEvents {
				t.Fatalf("events = %+v, want %d", sink.events, test.wantEvents)
			}
		})
	}
}
