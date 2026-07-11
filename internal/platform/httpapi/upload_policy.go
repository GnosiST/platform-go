package httpapi

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

const (
	defaultFileMaxUploadBytes int64 = 10 << 20
	maxFileUploadBytes        int64 = 100 << 20
	maxMultipartOverheadBytes int64 = 1 << 20
	maxUploadFileNameBytes          = 255
)

var defaultFileAllowedMediaTypes = []string{
	"application/pdf",
	"image/jpeg",
	"image/png",
	"text/plain",
}

type UploadPolicy struct {
	MaxBytes          int64
	AllowedMediaTypes map[string]struct{}
}

type validatedUpload struct {
	FileName    string
	ContentType string
	Reader      io.Reader
	Close       func() error
}

type uploadPolicyError struct {
	Status  int
	Code    string
	Message string
}

func (err *uploadPolicyError) Error() string {
	return err.Message
}

func NewUploadPolicy(maxBytes int64, allowedMediaTypes []string) (UploadPolicy, error) {
	if maxBytes <= 0 || maxBytes > maxFileUploadBytes {
		return UploadPolicy{}, fmt.Errorf("file upload max bytes must be between 1 and %d", maxFileUploadBytes)
	}
	allowed := make(map[string]struct{}, len(allowedMediaTypes))
	for _, raw := range allowedMediaTypes {
		value := strings.TrimSpace(raw)
		mediaType, params, err := mime.ParseMediaType(value)
		if err != nil || mediaType == "" || len(params) != 0 || value != strings.ToLower(mediaType) {
			return UploadPolicy{}, fmt.Errorf("invalid canonical file MIME type %q", raw)
		}
		allowed[mediaType] = struct{}{}
	}
	if len(allowed) == 0 {
		return UploadPolicy{}, errors.New("file MIME allowlist must not be empty")
	}
	return UploadPolicy{MaxBytes: maxBytes, AllowedMediaTypes: allowed}, nil
}

func DefaultUploadPolicy() UploadPolicy {
	policy, _ := NewUploadPolicy(defaultFileMaxUploadBytes, defaultFileAllowedMediaTypes)
	return policy
}

func normalizeUploadPolicy(policy UploadPolicy) UploadPolicy {
	if policy.MaxBytes == 0 && len(policy.AllowedMediaTypes) == 0 {
		return DefaultUploadPolicy()
	}
	return policy
}

func readValidatedUpload(ctx *gin.Context, policy UploadPolicy, codePrefix string) (validatedUpload, error) {
	policy = normalizeUploadPolicy(policy)
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, policy.MaxBytes+maxMultipartOverheadBytes)
	fileHeader, err := ctx.FormFile("file")
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			return validatedUpload{}, &uploadPolicyError{Status: http.StatusRequestEntityTooLarge, Code: codePrefix + "_TOO_LARGE", Message: "file exceeds the configured upload limit"}
		}
		return validatedUpload{}, &uploadPolicyError{Status: http.StatusBadRequest, Code: codePrefix + "_REQUIRED", Message: "file is required"}
	}
	if fileHeader.Size > policy.MaxBytes {
		_ = closeMultipartUpload(ctx, nil)
		return validatedUpload{}, &uploadPolicyError{Status: http.StatusRequestEntityTooLarge, Code: codePrefix + "_TOO_LARGE", Message: "file exceeds the configured upload limit"}
	}
	opened, err := fileHeader.Open()
	if err != nil {
		_ = closeMultipartUpload(ctx, nil)
		return validatedUpload{}, &uploadPolicyError{Status: http.StatusBadRequest, Code: codePrefix + "_OPEN_FAILED", Message: "open uploaded file failed"}
	}
	head, err := io.ReadAll(io.LimitReader(opened, 512))
	if err != nil {
		_ = closeMultipartUpload(ctx, opened)
		return validatedUpload{}, &uploadPolicyError{Status: http.StatusBadRequest, Code: codePrefix + "_READ_FAILED", Message: "read uploaded file failed"}
	}
	detected, _, err := mime.ParseMediaType(http.DetectContentType(head))
	if err != nil {
		_ = closeMultipartUpload(ctx, opened)
		return validatedUpload{}, &uploadPolicyError{Status: http.StatusUnsupportedMediaType, Code: codePrefix + "_MIME_INVALID", Message: "uploaded file MIME type is invalid"}
	}
	declared, _, err := mime.ParseMediaType(fileHeader.Header.Get("Content-Type"))
	if err != nil || declared == "" || !strings.EqualFold(declared, detected) {
		_ = closeMultipartUpload(ctx, opened)
		return validatedUpload{}, &uploadPolicyError{Status: http.StatusUnsupportedMediaType, Code: codePrefix + "_MIME_MISMATCH", Message: "declared and detected file MIME types do not match"}
	}
	detected = strings.ToLower(detected)
	if _, allowed := policy.AllowedMediaTypes[detected]; !allowed {
		_ = closeMultipartUpload(ctx, opened)
		return validatedUpload{}, &uploadPolicyError{Status: http.StatusUnsupportedMediaType, Code: codePrefix + "_MIME_NOT_ALLOWED", Message: "uploaded file MIME type is not allowed"}
	}
	return validatedUpload{
		FileName:    sanitizeUploadFileName(fileHeader.Filename),
		ContentType: detected,
		Reader:      io.MultiReader(bytes.NewReader(head), opened),
		Close: func() error {
			return closeMultipartUpload(ctx, opened)
		},
	}, nil
}

func closeMultipartUpload(ctx *gin.Context, opened io.Closer) error {
	var errs []error
	if opened != nil {
		errs = append(errs, opened.Close())
	}
	if ctx.Request.MultipartForm != nil {
		errs = append(errs, ctx.Request.MultipartForm.RemoveAll())
	}
	return errors.Join(errs...)
}

func sanitizeUploadFileName(raw string) string {
	name := path.Base(strings.ReplaceAll(strings.TrimSpace(raw), "\\", "/"))
	var builder strings.Builder
	lastUnderscore := false
	for _, char := range name {
		switch {
		case unicode.IsControl(char):
			continue
		case unicode.IsLetter(char), unicode.IsDigit(char), char == '.', char == '-', char == '_':
			builder.WriteRune(char)
			lastUnderscore = false
		case unicode.IsSpace(char):
			if !lastUnderscore {
				builder.WriteByte('_')
				lastUnderscore = true
			}
		default:
			if !lastUnderscore {
				builder.WriteByte('_')
				lastUnderscore = true
			}
		}
	}
	name = strings.Trim(builder.String(), "._")
	if name == "" {
		name = "file"
	}
	return truncateUploadFileName(name, maxUploadFileNameBytes)
}

func truncateUploadFileName(name string, maxBytes int) string {
	if len(name) <= maxBytes {
		return name
	}
	extension := path.Ext(name)
	if len(extension) > 32 {
		extension = ""
	}
	budget := maxBytes - len(extension)
	base := strings.TrimSuffix(name, extension)
	for len(base) > budget {
		_, size := utf8.DecodeLastRuneInString(base)
		base = base[:len(base)-size]
	}
	base = strings.TrimRight(base, "._-")
	if base == "" {
		base = "file"
	}
	return base + extension
}
