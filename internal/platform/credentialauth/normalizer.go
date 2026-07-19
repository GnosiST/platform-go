package credentialauth

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

func NormalizeIdentifier(identifier Identifier) (NormalizedIdentifier, error) {
	identifier.Type = IdentifierType(strings.TrimSpace(string(identifier.Type)))
	if !validIdentifierType(identifier.Type) {
		return NormalizedIdentifier{}, fmt.Errorf("%w: identifier type is unsupported", ErrInvalidInput)
	}
	if !utf8.ValidString(identifier.Value) {
		return NormalizedIdentifier{}, fmt.Errorf("%w: identifier must be valid UTF-8", ErrInvalidInput)
	}
	switch identifier.Type {
	case IdentifierTypeUsername:
		return normalizeUsernameIdentifier(identifier.Value)
	case IdentifierTypeEmail:
		return normalizeEmailIdentifier(identifier.Value)
	case IdentifierTypePhone:
		return normalizePhoneIdentifier(identifier.Value)
	default:
		return NormalizedIdentifier{}, fmt.Errorf("%w: identifier type is unsupported", ErrInvalidInput)
	}
}

func normalizeUsernameIdentifier(value string) (NormalizedIdentifier, error) {
	normalized := strings.TrimSpace(value)
	if normalized == "" || len(normalized) > 128 || strings.IndexFunc(normalized, invalidIdentifierRune) >= 0 {
		return NormalizedIdentifier{}, fmt.Errorf("%w: username identifier is invalid", ErrInvalidInput)
	}
	return NormalizedIdentifier{
		Type:             IdentifierTypeUsername,
		Value:            normalized,
		MaskedIdentifier: maskEdges(normalized, 2, 1, 3),
	}, nil
}

func normalizeEmailIdentifier(value string) (NormalizedIdentifier, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" || len(normalized) > 254 || strings.IndexFunc(normalized, invalidEmailRune) >= 0 {
		return NormalizedIdentifier{}, fmt.Errorf("%w: email identifier is invalid", ErrInvalidInput)
	}
	at := strings.LastIndex(normalized, "@")
	if at <= 0 || at != strings.Index(normalized, "@") || at == len(normalized)-1 {
		return NormalizedIdentifier{}, fmt.Errorf("%w: email identifier is invalid", ErrInvalidInput)
	}
	return NormalizedIdentifier{
		Type:             IdentifierTypeEmail,
		Value:            normalized,
		MaskedIdentifier: maskEmail(normalized),
	}, nil
}

func normalizePhoneIdentifier(value string) (NormalizedIdentifier, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return NormalizedIdentifier{}, fmt.Errorf("%w: phone identifier is invalid", ErrInvalidInput)
	}
	var builder strings.Builder
	for index, r := range raw {
		switch {
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '+' && index == 0:
			builder.WriteRune(r)
		case r == ' ' || r == '-' || r == '(' || r == ')':
			continue
		default:
			return NormalizedIdentifier{}, fmt.Errorf("%w: phone identifier is invalid", ErrInvalidInput)
		}
	}
	normalized := builder.String()
	digits := strings.TrimPrefix(normalized, "+")
	if len(digits) < 6 || len(digits) > 18 || strings.Contains(normalized[1:], "+") {
		return NormalizedIdentifier{}, fmt.Errorf("%w: phone identifier is invalid", ErrInvalidInput)
	}
	return NormalizedIdentifier{
		Type:             IdentifierTypePhone,
		Value:            normalized,
		MaskedIdentifier: maskPhone(normalized),
	}, nil
}

func invalidIdentifierRune(r rune) bool {
	return unicode.IsSpace(r) || unicode.IsControl(r)
}

func invalidEmailRune(r rune) bool {
	return unicode.IsSpace(r) || unicode.IsControl(r)
}

func maskEmail(value string) string {
	at := strings.LastIndex(value, "@")
	if at <= 0 || at == len(value)-1 {
		return maskEdges(value, 2, 0, 3)
	}
	return maskEdges(value[:at], 2, 0, 3) + value[at:]
}

func maskPhone(value string) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) == 0 {
		return ""
	}
	if len(runes) <= 7 {
		return string(runes[:1]) + "***" + string(runes[len(runes)-1:])
	}
	return string(runes[:3]) + "****" + string(runes[len(runes)-4:])
}

func maskEdges(value string, preservePrefix int, preserveSuffix int, maskLength int) string {
	runes := []rune(value)
	if len(runes) == 0 {
		return ""
	}
	prefix := min(preservePrefix, len(runes)-1)
	remaining := len(runes) - prefix
	suffix := min(preserveSuffix, max(remaining-1, 0))
	return string(runes[:prefix]) + strings.Repeat("*", maskLength) + string(runes[len(runes)-suffix:])
}
