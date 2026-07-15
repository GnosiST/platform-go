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
	"platform-go/internal/platform/sensitivereveal"
)

func TestSensitiveRevealErrorCodeMatrix(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code errorcode.Code
	}{
		{name: "record not found", err: adminresource.ErrRecordNotFound, code: errorcode.CodeAdminSensitiveRevealNotFound},
		{name: "policy not found", err: sensitivereveal.ErrPolicyNotFound, code: errorcode.CodeAdminSensitiveRevealNotFound},
		{name: "challenge not found", err: sensitivereveal.ErrChallengeNotFound, code: errorcode.CodeAdminSensitiveRevealNotFound},
		{name: "factor transaction not found", err: sensitivereveal.ErrFactorTransactionNotFound, code: errorcode.CodeAdminSensitiveRevealNotFound},
		{name: "grant not found", err: sensitivereveal.ErrGrantNotFound, code: errorcode.CodeAdminSensitiveRevealNotFound},
		{name: "challenge expired", err: sensitivereveal.ErrChallengeExpired, code: errorcode.CodeAdminSensitiveRevealExpired},
		{name: "challenge closed", err: sensitivereveal.ErrChallengeClosed, code: errorcode.CodeAdminSensitiveRevealExpired},
		{name: "grant expired", err: sensitivereveal.ErrGrantExpired, code: errorcode.CodeAdminSensitiveRevealExpired},
		{name: "grant consumed", err: sensitivereveal.ErrGrantConsumed, code: errorcode.CodeAdminSensitiveRevealExpired},
		{name: "factor locked", err: sensitivereveal.ErrFactorLocked, code: errorcode.CodeAdminSensitiveRevealExpired},
		{name: "factor already started", err: sensitivereveal.ErrFactorAlreadyStarted, code: errorcode.CodeAdminSensitiveRevealConflict},
		{name: "factor already completed", err: sensitivereveal.ErrFactorAlreadyCompleted, code: errorcode.CodeAdminSensitiveRevealConflict},
		{name: "reveal result recorded", err: sensitivereveal.ErrRevealResultRecorded, code: errorcode.CodeAdminSensitiveRevealConflict},
		{name: "verification failed", err: sensitivereveal.ErrVerificationFailed, code: errorcode.CodeAdminSensitiveRevealVerificationFailed},
		{name: "admin identity invalid", err: ErrAdminIdentityInvalid, code: errorcode.CodeAdminSensitiveRevealVerificationFailed},
		{name: "admin identity transaction invalid", err: ErrAdminIdentityTransaction, code: errorcode.CodeAdminSensitiveRevealVerificationFailed},
		{name: "admin identity binding invalid", err: ErrAdminIdentityBindingInvalid, code: errorcode.CodeAdminSensitiveRevealVerificationFailed},
		{name: "scope mismatch", err: sensitivereveal.ErrScopeMismatch, code: errorcode.CodeAdminForbidden},
		{name: "purpose not allowed", err: sensitivereveal.ErrPurposeNotAllowed, code: errorcode.CodeAdminForbidden},
		{name: "factor not allowed", err: sensitivereveal.ErrFactorNotAllowed, code: errorcode.CodeAdminForbidden},
		{name: "provider exchange", err: ErrAdminIdentityProviderExchange, code: errorcode.CodeAdminSensitiveRevealProviderFailed},
		{name: "protected field unavailable", err: adminresource.ErrProtectedFieldUnavailable, code: errorcode.CodeAdminSensitiveRevealUnavailable},
		{name: "protected field decryption failed", err: adminresource.ErrProtectedFieldDecryptionFailed, code: errorcode.CodeAdminSensitiveRevealUnavailable},
		{name: "invalid scope", err: sensitivereveal.ErrInvalidScope, code: errorcode.CodeAdminSensitiveRevealInvalid},
		{name: "invalid record", err: adminresource.ErrInvalidRecord, code: errorcode.CodeAdminSensitiveRevealInvalid},
		{name: "unknown failure", err: sensitivereveal.ErrInvalidConfiguration, code: errorcode.CodeAdminSensitiveRevealFailed},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := sensitiveRevealErrorCode(fmt.Errorf("wrapped: %w", test.err)); got != test.code {
				t.Fatalf("sensitiveRevealErrorCode(%v) = %s, want %s", test.err, got, test.code)
			}
		})
	}
}

func TestSensitiveRevealErrorAdapterUsesRegistryNoStoreAndNeverLeaksCause(t *testing.T) {
	const marker = "physical_table=users identityNumber=110101199001010000 email=marker@example.test"
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   errorcode.Code
		wantEvents int
	}{
		{name: "not found", err: fmt.Errorf("%w: %s", sensitivereveal.ErrChallengeNotFound, marker), wantStatus: http.StatusNotFound, wantCode: errorcode.CodeAdminSensitiveRevealNotFound},
		{name: "expired", err: sensitivereveal.ErrChallengeExpired, wantStatus: http.StatusGone, wantCode: errorcode.CodeAdminSensitiveRevealExpired},
		{name: "conflict", err: sensitivereveal.ErrFactorAlreadyStarted, wantStatus: http.StatusConflict, wantCode: errorcode.CodeAdminSensitiveRevealConflict},
		{name: "verification", err: ErrAdminIdentityInvalid, wantStatus: http.StatusUnprocessableEntity, wantCode: errorcode.CodeAdminSensitiveRevealVerificationFailed},
		{name: "forbidden", err: sensitivereveal.ErrScopeMismatch, wantStatus: http.StatusForbidden, wantCode: errorcode.CodeAdminForbidden},
		{name: "provider failure", err: fmt.Errorf("%w: %s", ErrAdminIdentityProviderExchange, marker), wantStatus: http.StatusBadGateway, wantCode: errorcode.CodeAdminSensitiveRevealProviderFailed, wantEvents: 1},
		{name: "protected field unavailable", err: fmt.Errorf("%w: %s", adminresource.ErrProtectedFieldUnavailable, marker), wantStatus: http.StatusServiceUnavailable, wantCode: errorcode.CodeAdminSensitiveRevealUnavailable, wantEvents: 1},
		{name: "invalid request", err: fmt.Errorf("%w: %s", adminresource.ErrInvalidRecord, marker), wantStatus: http.StatusBadRequest, wantCode: errorcode.CodeAdminSensitiveRevealInvalid},
		{name: "unknown failure", err: errors.New(marker), wantStatus: http.StatusInternalServerError, wantCode: errorcode.CodeAdminSensitiveRevealFailed, wantEvents: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sink := &errorResponseSink{}
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/api/admin/resources/users/1/fields/identityNumber/reveal", nil)

			writeSensitiveRevealError(ctx, sink, test.err)

			if recorder.Code != test.wantStatus || !strings.Contains(recorder.Body.String(), `"code":"`+string(test.wantCode)+`"`) {
				t.Fatalf("status/body = %d/%s, want %d/%s", recorder.Code, recorder.Body.String(), test.wantStatus, test.wantCode)
			}
			if recorder.Header().Get("Cache-Control") != "no-store" || recorder.Header().Get("Pragma") != "no-cache" {
				t.Fatalf("cache headers = %q/%q, want no-store/no-cache", recorder.Header().Get("Cache-Control"), recorder.Header().Get("Pragma"))
			}
			if strings.Contains(recorder.Body.String(), "physical_table") || strings.Contains(recorder.Body.String(), "110101199001010000") || strings.Contains(recorder.Body.String(), "marker@example.test") {
				t.Fatalf("response leaked cause: %s", recorder.Body.String())
			}
			if len(sink.events) != test.wantEvents {
				t.Fatalf("events = %+v, want %d", sink.events, test.wantEvents)
			}
		})
	}
}

func TestSensitiveRevealStateRefreshFailureUsesRegistryNoStoreAndRecordsOnce(t *testing.T) {
	const marker = "physical_table=users password=private"
	repository := &controllableAdminResourceRepository{snapshot: adminresource.ResourceSnapshot{Revision: 1, NextID: 1, Resources: map[string][]adminresource.Record{}}}
	resources, err := adminresource.NewRepositoryBackedStoreFromCapabilities(repository, nil)
	if err != nil {
		t.Fatalf("NewRepositoryBackedStoreFromCapabilities() error = %v", err)
	}
	repository.loadErr = errors.New(marker)
	sink := &errorResponseSink{}
	server := &Server{resources: resources, internalErrorSink: sink}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/admin/resources/users/1/fields/identityNumber/reveal-policy", nil)

	if server.refreshSensitiveRevealState(ctx) {
		t.Fatal("refreshSensitiveRevealState() = true, want false")
	}

	if recorder.Code != http.StatusServiceUnavailable || !strings.Contains(recorder.Body.String(), `"code":"ADMIN_SENSITIVE_REVEAL_STATE_REFRESH_FAILED"`) {
		t.Fatalf("status/body = %d/%s, want registered 503", recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get("Cache-Control") != "no-store" || len(sink.events) != 1 || sink.events[0].Code != string(errorcode.CodeAdminSensitiveRevealStateRefreshFailed) {
		t.Fatalf("headers/events = %q/%+v, want no-store and one safe event", recorder.Header().Get("Cache-Control"), sink.events)
	}
	if strings.Contains(recorder.Body.String(), "physical_table") || strings.Contains(recorder.Body.String(), "password") {
		t.Fatalf("response leaked refresh cause: %s", recorder.Body.String())
	}
}
