package httpapi

import (
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/GnosiST/platform-go/internal/platform/errorcode"
)

func TestUploadErrorCodesAreCompleteRegisteredAndSeparateUploadOpenFromObjectOpen(t *testing.T) {
	tests := []struct {
		name       string
		codes      UploadErrorCodes
		want       []errorcode.Code
		objectOpen errorcode.Code
	}{
		{name: "admin", codes: adminUploadErrorCodes, want: []errorcode.Code{
			errorcode.CodeAdminFileRequired,
			errorcode.CodeAdminFileTooLarge,
			errorcode.CodeAdminFileUploadOpenFailed,
			errorcode.CodeAdminFileReadFailed,
			errorcode.CodeAdminFileMIMEInvalid,
			errorcode.CodeAdminFileMIMEMismatch,
			errorcode.CodeAdminFileMIMENotAllowed,
		}, objectOpen: errorcode.CodeAdminFileOpenFailed},
		{name: "app", codes: appUploadErrorCodes, want: []errorcode.Code{
			errorcode.CodeAppFileRequired,
			errorcode.CodeAppFileTooLarge,
			errorcode.CodeAppFileUploadOpenFailed,
			errorcode.CodeAppFileReadFailed,
			errorcode.CodeAppFileMIMEInvalid,
			errorcode.CodeAppFileMIMEMismatch,
			errorcode.CodeAppFileMIMENotAllowed,
		}, objectOpen: errorcode.CodeAppFileOpenFailed},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			codes := []errorcode.Code{
				test.codes.Required,
				test.codes.TooLarge,
				test.codes.OpenFailed,
				test.codes.ReadFailed,
				test.codes.MIMEInvalid,
				test.codes.MIMEMismatch,
				test.codes.MIMENotAllowed,
			}
			for index, code := range codes {
				if code != test.want[index] {
					t.Fatalf("upload error code[%d] = %q, want %q", index, code, test.want[index])
				}
			}
			for _, code := range codes {
				if code == "" {
					t.Fatal("upload error code set contains an empty code")
				}
				if _, ok := errorcode.Lookup(code); !ok {
					t.Fatalf("upload error code %q is not registered", code)
				}
			}
			if test.codes.OpenFailed == test.objectOpen {
				t.Fatalf("multipart open code %q collides with stored-object open code", test.codes.OpenFailed)
			}
			uploadDefinition, _ := errorcode.Lookup(test.codes.OpenFailed)
			objectDefinition, _ := errorcode.Lookup(test.objectOpen)
			if uploadDefinition.HTTPStatus != http.StatusBadRequest || objectDefinition.HTTPStatus != http.StatusInternalServerError {
				t.Fatalf("upload/object open statuses = %d/%d, want 400/500", uploadDefinition.HTTPStatus, objectDefinition.HTTPStatus)
			}
		})
	}
}

func TestReadValidatedUploadUsesMultipartOpenCode(t *testing.T) {
	tests := []struct {
		name  string
		codes UploadErrorCodes
		want  errorcode.Code
	}{
		{name: "admin", codes: adminUploadErrorCodes, want: errorcode.CodeAdminFileUploadOpenFailed},
		{name: "app", codes: appUploadErrorCodes, want: errorcode.CodeAppFileUploadOpenFailed},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader(""))
			ctx.Request.MultipartForm = &multipart.Form{File: map[string][]*multipart.FileHeader{
				"file": {{Filename: "private.txt", Header: textproto.MIMEHeader{"Content-Type": {"text/plain"}}}},
			}}
			_, err := readValidatedUpload(ctx, DefaultUploadPolicy(), test.codes)
			var policyErr *uploadPolicyError
			if !errors.As(err, &policyErr) || policyErr.Code != test.want {
				t.Fatalf("readValidatedUpload() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestUploadPolicyErrorCodeUsesRegistryAndUnknownFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{name: "registered", err: &uploadPolicyError{Code: errorcode.CodeAppFileUploadOpenFailed}, wantStatus: http.StatusBadRequest, wantCode: "APP_FILE_UPLOAD_OPEN_FAILED"},
		{name: "unregistered policy code", err: &uploadPolicyError{Code: errorcode.Code("PRIVATE_UPLOAD_FAILURE")}, wantStatus: http.StatusBadRequest, wantCode: "FILE_UPLOAD_INVALID"},
		{name: "unknown", err: errors.New("private multipart parser detail"), wantStatus: http.StatusBadRequest, wantCode: "FILE_UPLOAD_INVALID"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/upload", nil)
			writePlatformError(ctx, uploadPolicyErrorCode(test.err))
			if recorder.Code != test.wantStatus || !strings.Contains(recorder.Body.String(), `"code":"`+test.wantCode+`"`) {
				t.Fatalf("response = %d/%s, want code %s", recorder.Code, recorder.Body.String(), test.wantCode)
			}
		})
	}
}

func TestNewUploadPolicyCanonicalizesAllowedMediaTypes(t *testing.T) {
	policy, err := NewUploadPolicy(1024, []string{" image/png ", "text/plain", "image/png"})
	if err != nil {
		t.Fatalf("NewUploadPolicy() error = %v", err)
	}
	if policy.MaxBytes != 1024 || len(policy.AllowedMediaTypes) != 2 {
		t.Fatalf("policy = %+v", policy)
	}
	for _, mediaType := range []string{"image/png", "text/plain"} {
		if _, ok := policy.AllowedMediaTypes[mediaType]; !ok {
			t.Fatalf("policy missing %q: %+v", mediaType, policy)
		}
	}
}

func TestNewUploadPolicyRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		maxBytes int64
		allowed  []string
	}{
		{maxBytes: 0, allowed: []string{"text/plain"}},
		{maxBytes: 1024, allowed: nil},
		{maxBytes: 1024, allowed: []string{"text/plain; charset=utf-8"}},
	}
	for _, tt := range tests {
		if _, err := NewUploadPolicy(tt.maxBytes, tt.allowed); err == nil {
			t.Fatalf("NewUploadPolicy(%d, %#v) succeeded, want error", tt.maxBytes, tt.allowed)
		}
	}
}

func TestSanitizeUploadFileName(t *testing.T) {
	name := sanitizeUploadFileName("..\\..\\unsafe\x00\n report.txt")
	if name != "unsafe_report.txt" {
		t.Fatalf("sanitizeUploadFileName() = %q", name)
	}
	long := sanitizeUploadFileName(strings.Repeat("a", 400) + ".txt")
	if len(long) > maxUploadFileNameBytes || !strings.HasSuffix(long, ".txt") {
		t.Fatalf("long sanitized filename length/suffix = %d/%q", len(long), long)
	}
}
