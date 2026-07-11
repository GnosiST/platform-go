package httpapi

import (
	"strings"
	"testing"
)

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
