package serviceobject

import (
	"sort"
	"strings"
)

const maximumDomainCommandItems = 2000

func normalizeStringSet(value any, maxLength int) ([]string, error) {
	values, ok := stringSlice(value)
	if !ok || len(values) > maximumDomainCommandItems {
		return nil, ErrRequestInvalid
	}
	unique := make(map[string]struct{}, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" || len(normalized) > maxLength {
			return nil, ErrRequestInvalid
		}
		unique[normalized] = struct{}{}
	}
	normalized := make([]string, 0, len(unique))
	for value := range unique {
		normalized = append(normalized, value)
	}
	sort.Strings(normalized)
	return normalized, nil
}

func normalizeRoleRemediations(value any, maxLength int) ([]RoleRemediation, error) {
	entries, ok := remediationSlice(value)
	if !ok || len(entries) > maximumDomainCommandItems {
		return nil, ErrRequestInvalid
	}
	normalizedByTarget := make(map[string]RoleRemediation, len(entries))
	for _, entry := range entries {
		normalized, err := normalizeRoleRemediation(entry, maxLength)
		if err != nil {
			return nil, ErrRequestInvalid
		}
		key := normalized.UserCode + "\x00" + normalized.RoleCode
		if existing, exists := normalizedByTarget[key]; exists {
			if existing != normalized {
				return nil, ErrRequestInvalid
			}
			continue
		}
		normalizedByTarget[key] = normalized
	}
	normalized := make([]RoleRemediation, 0, len(normalizedByTarget))
	for _, entry := range normalizedByTarget {
		normalized = append(normalized, entry)
	}
	sort.Slice(normalized, func(left int, right int) bool {
		if normalized[left].UserCode != normalized[right].UserCode {
			return normalized[left].UserCode < normalized[right].UserCode
		}
		return normalized[left].RoleCode < normalized[right].RoleCode
	})
	return normalized, nil
}

func normalizeRoleRemediation(entry RoleRemediation, maxLength int) (RoleRemediation, error) {
	entry.UserCode = strings.TrimSpace(entry.UserCode)
	entry.RoleCode = strings.TrimSpace(entry.RoleCode)
	entry.ReplacementRoleCode = strings.TrimSpace(entry.ReplacementRoleCode)
	if entry.UserCode == "" || entry.RoleCode == "" || len(entry.UserCode) > maxLength || len(entry.RoleCode) > maxLength || len(entry.ReplacementRoleCode) > maxLength {
		return RoleRemediation{}, ErrRequestInvalid
	}
	switch entry.Action {
	case RoleRemediationRemove:
		if entry.ReplacementRoleCode != "" {
			return RoleRemediation{}, ErrRequestInvalid
		}
	case RoleRemediationReplace:
		if entry.ReplacementRoleCode == "" {
			return RoleRemediation{}, ErrRequestInvalid
		}
	default:
		return RoleRemediation{}, ErrRequestInvalid
	}
	return entry, nil
}

func stringSlice(value any) ([]string, bool) {
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...), true
	case []any:
		values := make([]string, len(typed))
		for index, item := range typed {
			text, ok := item.(string)
			if !ok {
				return nil, false
			}
			values[index] = text
		}
		return values, true
	default:
		return nil, false
	}
}

func remediationSlice(value any) ([]RoleRemediation, bool) {
	switch typed := value.(type) {
	case []RoleRemediation:
		return cloneRoleRemediations(typed), true
	case []any:
		entries := make([]RoleRemediation, len(typed))
		for index, item := range typed {
			entry, ok := item.(map[string]any)
			if !ok {
				return nil, false
			}
			userCode, userOK := entry["userCode"].(string)
			roleCode, roleOK := entry["roleCode"].(string)
			action, actionOK := entry["action"].(string)
			if !userOK || !roleOK || !actionOK {
				return nil, false
			}
			replacement := ""
			expectedFields := 3
			if raw, exists := entry["replacementRoleCode"]; exists {
				var replacementOK bool
				replacement, replacementOK = raw.(string)
				if !replacementOK {
					return nil, false
				}
				expectedFields++
			}
			if len(entry) != expectedFields || action == string(RoleRemediationRemove) && expectedFields != 3 || action == string(RoleRemediationReplace) && expectedFields != 4 {
				return nil, false
			}
			entries[index] = RoleRemediation{
				UserCode: userCode, RoleCode: roleCode, Action: RoleRemediationAction(action), ReplacementRoleCode: replacement,
			}
		}
		return entries, true
	default:
		return nil, false
	}
}

func cloneValidatedArguments(arguments ValidatedArguments) ValidatedArguments {
	clone := make(ValidatedArguments, len(arguments))
	for name, value := range arguments {
		switch typed := value.(type) {
		case []string:
			clone[name] = append([]string(nil), typed...)
		case []RoleRemediation:
			clone[name] = cloneRoleRemediations(typed)
		case MenuDefinition:
			clone[name] = cloneMenuDefinition(typed)
		default:
			clone[name] = value
		}
	}
	return clone
}

func cloneRoleRemediations(remediations []RoleRemediation) []RoleRemediation {
	return append([]RoleRemediation(nil), remediations...)
}
