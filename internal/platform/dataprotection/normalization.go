package dataprotection

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

func normalizeValue(value string, version string) (string, error) {
	if !utf8.ValidString(value) {
		return "", fmt.Errorf("%w: value must be valid UTF-8", ErrInvalidPolicy)
	}
	switch version {
	case NormalizationRawV1:
		return value, nil
	case NormalizationTrimV1:
		return strings.TrimSpace(value), nil
	case NormalizationEmailV1:
		return strings.ToLower(strings.TrimSpace(value)), nil
	case NormalizationPhoneCNV1:
		return normalizeChinesePhone(value)
	case NormalizationIdentityCNV1:
		return normalizeChineseIdentity(value)
	default:
		return "", fmt.Errorf("%w: unsupported normalization", ErrInvalidPolicy)
	}
}

func normalizeChinesePhone(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	var digits strings.Builder
	for _, char := range trimmed {
		switch {
		case char >= '0' && char <= '9':
			digits.WriteRune(char)
		case char == '+' || unicode.IsSpace(char) || char == '-' || char == '(' || char == ')':
		default:
			return "", fmt.Errorf("%w: phone value is invalid", ErrInvalidPolicy)
		}
	}
	normalized := digits.String()
	if strings.HasPrefix(normalized, "86") && len(normalized) == 13 {
		normalized = normalized[2:]
	}
	if len(normalized) != 11 || normalized[0] != '1' {
		return "", fmt.Errorf("%w: phone value is invalid", ErrInvalidPolicy)
	}
	return "+86" + normalized, nil
}

func normalizeChineseIdentity(value string) (string, error) {
	normalized := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(value), " ", ""))
	if len(normalized) != 15 && len(normalized) != 18 {
		return "", fmt.Errorf("%w: identity value is invalid", ErrInvalidPolicy)
	}
	for index, char := range normalized {
		if char >= '0' && char <= '9' {
			continue
		}
		if len(normalized) == 18 && index == 17 && char == 'X' {
			continue
		}
		return "", fmt.Errorf("%w: identity value is invalid", ErrInvalidPolicy)
	}
	return normalized, nil
}
