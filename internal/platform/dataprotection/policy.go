package dataprotection

import (
	"errors"
	"fmt"
	"strings"
)

const (
	FormatAES256GCMV1 = "aes-256-gcm-v1"

	NormalizationRawV1        = "raw-v1"
	NormalizationTrimV1       = "trim-v1"
	NormalizationEmailV1      = "email-v1"
	NormalizationPhoneCNV1    = "phone-e164-cn-v1"
	NormalizationIdentityCNV1 = "identity-cn-v1"

	GlobalTenantID = "platform:global"
)

var (
	ErrInvalidPolicy  = errors.New("invalid data protection policy")
	ErrInvalidContext = errors.New("invalid data protection context")
	ErrPolicyMismatch = errors.New("data protection policy mismatch")
)

type FieldPolicy struct {
	Format              string
	Normalization       string
	BlindIndexNamespace string
}

type FieldContext struct {
	TenantID      string
	Resource      string
	RecordID      string
	FieldKey      string
	SchemaVersion uint32
}

func (p FieldPolicy) validate() error {
	if p.Format != FormatAES256GCMV1 {
		return fmt.Errorf("%w: unsupported format", ErrInvalidPolicy)
	}
	switch p.Normalization {
	case NormalizationRawV1, NormalizationTrimV1, NormalizationEmailV1, NormalizationPhoneCNV1, NormalizationIdentityCNV1:
	default:
		return fmt.Errorf("%w: unsupported normalization", ErrInvalidPolicy)
	}
	if p.BlindIndexNamespace != "" && !canonicalIdentifier(p.BlindIndexNamespace) {
		return fmt.Errorf("%w: blind-index namespace must be canonical", ErrInvalidPolicy)
	}
	return nil
}

func (c FieldContext) validate() error {
	if strings.TrimSpace(c.TenantID) == "" || strings.TrimSpace(c.Resource) == "" || strings.TrimSpace(c.RecordID) == "" || strings.TrimSpace(c.FieldKey) == "" || c.SchemaVersion == 0 {
		return ErrInvalidContext
	}
	if c.TenantID != strings.TrimSpace(c.TenantID) || c.Resource != strings.TrimSpace(c.Resource) || c.RecordID != strings.TrimSpace(c.RecordID) || c.FieldKey != strings.TrimSpace(c.FieldKey) {
		return ErrInvalidContext
	}
	return nil
}

func canonicalIdentifier(value string) bool {
	if value == "" || value != strings.ToLower(strings.TrimSpace(value)) {
		return false
	}
	for index, char := range value {
		if char >= 'a' && char <= 'z' || char >= '0' && char <= '9' {
			continue
		}
		if (char == '-' || char == '.' || char == ':') && index > 0 && index < len(value)-1 {
			continue
		}
		return false
	}
	return true
}
