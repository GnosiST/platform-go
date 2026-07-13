package masking

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	StrategyPartialV1    = "partial-v1"
	StrategyPhoneV1      = "phone-v1"
	StrategyEmailV1      = "email-v1"
	StrategyIdentityCNV1 = "identity-cn-v1"
	StrategyAddressCNV1  = "address-cn-v1"
)

var ErrInvalidPolicy = errors.New("invalid masking policy")

type Policy struct {
	Strategy       string
	PreservePrefix int
	PreserveSuffix int
	MaskLength     int
	Replacement    string
}

type Runtime interface {
	Validate(Policy) error
	Mask(context.Context, Policy, string) (string, error)
}

type Service struct{}

func NewRuntime() *Service {
	return &Service{}
}

func (s *Service) Validate(policy Policy) error {
	if _, err := normalizedPolicy(policy); err != nil {
		return err
	}
	return nil
}

func (s *Service) Mask(ctx context.Context, policy Policy, value string) (string, error) {
	if ctx == nil {
		return "", context.Canceled
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	policy, err := normalizedPolicy(policy)
	if err != nil {
		return "", err
	}
	if value == "" {
		return "", nil
	}
	switch policy.Strategy {
	case StrategyPartialV1:
		return maskEdges(value, policy.PreservePrefix, policy.PreserveSuffix, policy.MaskLength, policy.Replacement), nil
	case StrategyPhoneV1:
		return maskEdges(value, 3, 4, 4, policy.Replacement), nil
	case StrategyEmailV1:
		return maskEmail(value, policy.Replacement), nil
	case StrategyIdentityCNV1:
		return maskEdges(value, 2, 2, 6, policy.Replacement), nil
	case StrategyAddressCNV1:
		return maskEdges(value, 6, 0, 6, policy.Replacement), nil
	default:
		return "", fmt.Errorf("%w: masking strategy is unsupported", ErrInvalidPolicy)
	}
}

func normalizedPolicy(policy Policy) (Policy, error) {
	policy.Strategy = strings.TrimSpace(policy.Strategy)
	if policy.Replacement == "" {
		policy.Replacement = "*"
	}
	if utf8.RuneCountInString(policy.Replacement) != 1 {
		return Policy{}, fmt.Errorf("%w: masking replacement must be one rune", ErrInvalidPolicy)
	}
	replacement, _ := utf8.DecodeRuneInString(policy.Replacement)
	if !unicode.IsLetter(replacement) && !unicode.IsNumber(replacement) && !unicode.IsPunct(replacement) && !unicode.IsSymbol(replacement) {
		return Policy{}, fmt.Errorf("%w: masking replacement must be visible", ErrInvalidPolicy)
	}
	if policy.PreservePrefix < 0 || policy.PreservePrefix > 64 || policy.PreserveSuffix < 0 || policy.PreserveSuffix > 64 || policy.MaskLength < 0 || policy.MaskLength > 64 {
		return Policy{}, fmt.Errorf("%w: masking counts must be between zero and 64", ErrInvalidPolicy)
	}
	switch policy.Strategy {
	case StrategyPartialV1:
		if policy.MaskLength == 0 {
			return Policy{}, fmt.Errorf("%w: partial-v1 masking requires maskLength", ErrInvalidPolicy)
		}
	case StrategyPhoneV1, StrategyEmailV1, StrategyIdentityCNV1, StrategyAddressCNV1:
		if policy.PreservePrefix != 0 || policy.PreserveSuffix != 0 || policy.MaskLength != 0 {
			return Policy{}, fmt.Errorf("%w: preset masking strategy does not accept custom counts", ErrInvalidPolicy)
		}
	default:
		return Policy{}, fmt.Errorf("%w: masking strategy is unsupported", ErrInvalidPolicy)
	}
	return policy, nil
}

func maskEmail(value string, replacement string) string {
	at := strings.LastIndex(value, "@")
	if at <= 0 || at == len(value)-1 {
		return maskEdges(value, 2, 0, 3, replacement)
	}
	return maskEdges(value[:at], 2, 0, 3, replacement) + value[at:]
}

func maskEdges(value string, preservePrefix int, preserveSuffix int, maskLength int, replacement string) string {
	runes := []rune(value)
	if len(runes) == 0 {
		return ""
	}
	prefix := min(preservePrefix, len(runes)-1)
	remaining := len(runes) - prefix
	suffix := min(preserveSuffix, max(remaining-1, 0))
	return string(runes[:prefix]) + strings.Repeat(replacement, maskLength) + string(runes[len(runes)-suffix:])
}
