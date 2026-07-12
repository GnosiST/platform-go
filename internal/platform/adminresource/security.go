package adminresource

import (
	"fmt"
	"strings"

	"platform-go/internal/platform/capability"
)

type WriteOrigin string

const (
	WriteOriginExternal WriteOrigin = "external"
	WriteOriginInternal WriteOrigin = "internal"
)

type ProjectionPurpose string

const (
	ProjectionResponse ProjectionPurpose = "response"
	ProjectionExport   ProjectionPurpose = "export"
)

func defaultFieldPolicy(field FieldDefinition) FieldDefinition {
	if field.Sensitivity == "" {
		field.Sensitivity = capability.FieldSensitivityPublic
	}
	if field.StorageMode == "" {
		field.StorageMode = capability.FieldStoragePlain
	}
	if field.ResponseMode == "" {
		field.ResponseMode = capability.FieldProjectionFull
	}
	if field.ExportMode == "" {
		field.ExportMode = capability.FieldProjectionFull
	}
	return field
}

func (s *Store) validateWriteValues(resource string, values map[string]string, origin WriteOrigin) error {
	schema, ok := s.schemas[resource]
	if !ok {
		return ErrUnknownResource
	}
	fields := make(map[string]FieldDefinition, len(schema.Fields))
	for _, field := range schema.Fields {
		if field.Source == "values" {
			fields[field.Key] = defaultFieldPolicy(field)
		}
	}
	for key, value := range values {
		field, declared := fields[key]
		if !declared {
			return invalidSecurityField(key, "is not declared by the active schema")
		}
		if err := validateFieldWrite(key, value, field, origin); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) validateWriteInput(resource string, input WriteInput, origin WriteOrigin) error {
	if err := s.validateWriteValues(resource, input.Values, origin); err != nil {
		return err
	}
	schema, ok := s.schemas[resource]
	if !ok {
		return ErrUnknownResource
	}
	fields := make(map[string]FieldDefinition, len(schema.Fields))
	for _, field := range schema.Fields {
		if field.Source == "record" {
			fields[field.Key] = defaultFieldPolicy(field)
		}
	}
	for key, value := range map[string]string{
		"code": input.Code, "name": input.Name, "status": input.Status, "description": input.Description,
	} {
		if strings.TrimSpace(value) == "" {
			continue
		}
		field, declared := fields[key]
		if !declared {
			return invalidSecurityField(key, "is not declared by the active schema")
		}
		if err := validateFieldWrite(key, value, field, origin); err != nil {
			return err
		}
	}
	return nil
}

func validateFieldWrite(key string, value string, field FieldDefinition, origin WriteOrigin) error {
	if !validFieldSensitivity(field.Sensitivity) {
		return invalidSecurityField(key, "has unsupported sensitivity")
	}
	if !validFieldStorageMode(field.StorageMode) {
		return invalidSecurityField(key, "has unsupported storage mode")
	}
	if !validFieldProjectionMode(field.ResponseMode) {
		return invalidSecurityField(key, "has unsupported response mode")
	}
	if !validFieldProjectionMode(field.ExportMode) {
		return invalidSecurityField(key, "has unsupported export mode")
	}
	if origin == WriteOriginExternal && field.ReadOnly {
		return invalidSecurityField(key, "is read-only")
	}
	if origin == WriteOriginExternal && field.Sensitivity != capability.FieldSensitivityPublic {
		return invalidSecurityField(key, "is not externally writable")
	}
	protectedSensitivity := field.Sensitivity == capability.FieldSensitivityPersonal ||
		field.Sensitivity == capability.FieldSensitivitySensitive ||
		field.Sensitivity == capability.FieldSensitivitySecret
	if (field.Sensitivity == capability.FieldSensitivitySensitive || field.Sensitivity == capability.FieldSensitivitySecret) && field.Source == "record" {
		return invalidSecurityField(key, "sensitive or secret values cannot use record storage")
	}
	if protectedSensitivity && field.StorageMode == capability.FieldStoragePlain {
		return invalidSecurityField(key, "requires protected storage")
	}
	if field.StorageMode == capability.FieldStorageMasked {
		if field.Sensitivity != capability.FieldSensitivityPersonal {
			return invalidSecurityField(key, "masked storage requires personal sensitivity")
		}
		if !allowsMaskedProjection(field.ResponseMode) || !allowsMaskedProjection(field.ExportMode) {
			return invalidSecurityField(key, "masked storage requires masked or omitted response and export")
		}
		if strings.TrimSpace(value) != "" && !strings.Contains(value, "*") {
			return invalidSecurityField(key, "masked storage requires an actually masked value")
		}
	}
	if field.StorageMode == capability.FieldStorageHashed &&
		(field.ResponseMode != capability.FieldProjectionOmitted || field.ExportMode != capability.FieldProjectionOmitted) {
		return invalidSecurityField(key, "hashed storage must be omitted from response and export")
	}
	if field.StorageMode == capability.FieldStorageEncrypted &&
		(!allowsEncryptedProjection(field.ResponseMode) || !allowsEncryptedProjection(field.ExportMode)) {
		return invalidSecurityField(key, "encrypted storage must use privileged or omitted response and export")
	}
	if err := validateFieldProtection(key, field); err != nil {
		return err
	}
	if prohibitedRawField(key) && !allowsProtectedField(field) {
		return invalidSecurityField(key, "is a prohibited raw credential or personal field")
	}
	return nil
}

func validateFieldProtection(key string, field FieldDefinition) error {
	if field.StorageMode != capability.FieldStorageEncrypted {
		if field.Protection != nil {
			return invalidSecurityField(key, "protection metadata requires encrypted storage")
		}
		return nil
	}
	if field.Protection == nil {
		return invalidSecurityField(key, "encrypted storage requires protection metadata")
	}
	if field.Protection.Format != "aes-256-gcm-v1" {
		return invalidSecurityField(key, "has unsupported protection format")
	}
	switch field.Protection.Normalization {
	case "raw-v1", "trim-v1", "email-v1", "phone-e164-cn-v1", "identity-cn-v1":
	default:
		return invalidSecurityField(key, "has unsupported protection normalization")
	}
	return nil
}

func invalidSecurityField(field string, reason string) error {
	return fmt.Errorf("%w: field %s %s", ErrInvalidRecord, field, reason)
}

func validFieldSensitivity(value string) bool {
	switch value {
	case capability.FieldSensitivityPublic, capability.FieldSensitivityInternal, capability.FieldSensitivityPersonal, capability.FieldSensitivitySensitive, capability.FieldSensitivitySecret:
		return true
	default:
		return false
	}
}

func validFieldStorageMode(value string) bool {
	switch value {
	case capability.FieldStoragePlain, capability.FieldStorageMasked, capability.FieldStorageHashed, capability.FieldStorageEncrypted:
		return true
	default:
		return false
	}
}

func validFieldProjectionMode(value string) bool {
	switch value {
	case capability.FieldProjectionFull, capability.FieldProjectionMasked, capability.FieldProjectionPrivileged, capability.FieldProjectionOmitted:
		return true
	default:
		return false
	}
}

func prohibitedRawField(key string) bool {
	normalized := strings.ToLower(strings.NewReplacer("_", "", "-", "", ".", "").Replace(strings.TrimSpace(key)))
	base := normalized
	for {
		previous := base
		for _, suffix := range []string{"hash", "digest"} {
			base = strings.TrimSuffix(base, suffix)
		}
		if base == previous {
			break
		}
	}
	if (base == "code" || base == "session") && base != normalized {
		return true
	}
	normalized = base
	switch base {
	case "verificationcode", "debugcode", "providersubject", "phone", "phonenumber", "identitynumber", "idnumber", "email", "address", "detailedaddress", "sessionid", "sessionhandle", "sessiontoken":
		return true
	}
	for _, marker := range []string{"password", "passwd", "token", "secret", "credential", "credentials", "sessionid", "sessionhandle", "sessiontoken", "session"} {
		if protectedNameMatch(base, marker) {
			return true
		}
	}
	return strings.HasSuffix(base, "email") ||
		strings.HasSuffix(base, "phone") ||
		strings.HasSuffix(base, "phonenumber") ||
		strings.HasSuffix(base, "address") ||
		strings.HasSuffix(base, "identitynumber") ||
		strings.HasSuffix(base, "idnumber") ||
		strings.HasSuffix(base, "providersubject")
}

func protectedNameMatch(normalized string, marker string) bool {
	if normalized == marker || strings.HasSuffix(normalized, marker) {
		return true
	}
	if !strings.HasPrefix(normalized, marker) {
		return false
	}
	for _, suffix := range []string{"prefix", "type", "count", "status", "expiresat", "issuedat", "createdat", "updatedat", "revokedat", "lastusedat"} {
		if strings.HasSuffix(normalized, suffix) {
			return false
		}
	}
	return true
}

func allowsMaskedProjection(mode string) bool {
	return mode == capability.FieldProjectionMasked || mode == capability.FieldProjectionOmitted
}

func allowsEncryptedProjection(mode string) bool {
	return mode == capability.FieldProjectionPrivileged || mode == capability.FieldProjectionOmitted
}

func allowsProtectedField(field FieldDefinition) bool {
	switch field.StorageMode {
	case capability.FieldStorageMasked:
		return field.Sensitivity == capability.FieldSensitivityPersonal &&
			allowsMaskedProjection(field.ResponseMode) && allowsMaskedProjection(field.ExportMode)
	case capability.FieldStorageHashed:
		return field.Sensitivity != capability.FieldSensitivityPublic &&
			field.ResponseMode == capability.FieldProjectionOmitted && field.ExportMode == capability.FieldProjectionOmitted
	case capability.FieldStorageEncrypted:
		return field.Sensitivity != capability.FieldSensitivityPublic &&
			allowsEncryptedProjection(field.ResponseMode) && allowsEncryptedProjection(field.ExportMode)
	default:
		return false
	}
}

func (s *Store) validateSnapshot(snapshot ResourceSnapshot) error {
	for resource, records := range snapshot.Resources {
		if _, ok := s.schemas[resource]; !ok {
			return fmt.Errorf("%w: snapshot contains unknown resource %s", ErrInvalidRecord, resource)
		}
		for _, record := range records {
			if err := s.validateWriteValues(resource, record.Values, WriteOriginInternal); err != nil {
				return fmt.Errorf("%w: resource %s record %s", err, resource, record.ID)
			}
			if err := s.validateStoredRecordFields(resource, record); err != nil {
				return fmt.Errorf("%w: resource %s record %s", err, resource, record.ID)
			}
		}
	}
	return nil
}

func (s *Store) validateStoredRecordFields(resource string, record Record) error {
	schema := s.schemas[resource]
	fields := make(map[string]FieldDefinition, len(schema.Fields))
	for _, field := range schema.Fields {
		if field.Source == "record" {
			fields[field.Key] = defaultFieldPolicy(field)
		}
	}
	for key, value := range map[string]string{
		"code": record.Code, "name": record.Name, "status": record.Status, "description": record.Description, "updatedAt": record.UpdatedAt,
	} {
		if strings.TrimSpace(value) == "" {
			continue
		}
		field, declared := fields[key]
		if !declared {
			return invalidSecurityField(key, "is not declared by the active schema")
		}
		if err := validateFieldWrite(key, value, field, WriteOriginInternal); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) scrubSnapshot(snapshot ResourceSnapshot) (ResourceSnapshot, bool, error) {
	clean := ResourceSnapshot{Revision: snapshot.Revision, NextID: snapshot.NextID, Resources: map[string][]Record{}}
	changed := false
	for resource, records := range snapshot.Resources {
		schema, ok := s.schemas[resource]
		if !ok {
			changed = true
			continue
		}
		fields := make(map[string]FieldDefinition, len(schema.Fields))
		for _, field := range schema.Fields {
			if field.Source == "values" {
				fields[field.Key] = defaultFieldPolicy(field)
			}
		}
		cleanRecords := make([]Record, 0, len(records))
		for _, record := range records {
			cleanRecord := cloneRecord(record)
			cleanRecord.Values = map[string]string{}
			for key, value := range record.Values {
				field, declared := fields[key]
				if !declared || validateFieldWrite(key, value, field, WriteOriginInternal) != nil {
					changed = true
					continue
				}
				cleanRecord.Values[key] = value
			}
			if len(cleanRecord.Values) == 0 {
				cleanRecord.Values = nil
			}
			cleanRecords = append(cleanRecords, cleanRecord)
		}
		clean.Resources[resource] = cleanRecords
	}
	if err := s.validateSnapshot(clean); err != nil {
		return ResourceSnapshot{}, false, err
	}
	return clean, changed, nil
}

func (s *Store) ProjectRecord(resource string, record Record, purpose ProjectionPurpose) (Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.projectRecordLocked(resource, record, purpose)
}

func (s *Store) projectRecordLocked(resource string, record Record, purpose ProjectionPurpose) (Record, error) {
	schema, ok := s.schemas[resource]
	if !ok {
		return Record{}, ErrUnknownResource
	}
	projected := Record{ID: record.ID}
	projected.Values = map[string]string{}
	for _, rawField := range schema.Fields {
		field := defaultFieldPolicy(rawField)
		mode, err := projectionMode(field, purpose)
		if err != nil {
			return Record{}, err
		}
		if mode == capability.FieldProjectionOmitted || mode == capability.FieldProjectionPrivileged {
			continue
		}
		if field.Source == "values" {
			if value, exists := record.Values[field.Key]; exists {
				projected.Values[field.Key] = value
			}
			continue
		}
		if field.Source == "record" {
			applyProjectedRecordField(&projected, record, field.Key)
		}
	}
	if len(projected.Values) == 0 {
		projected.Values = nil
	}
	return projected, nil
}

func projectionMode(field FieldDefinition, purpose ProjectionPurpose) (string, error) {
	mode := field.ResponseMode
	if purpose == ProjectionExport {
		mode = field.ExportMode
	} else if purpose != ProjectionResponse {
		return "", fmt.Errorf("%w: unsupported projection purpose %s", ErrInvalidRecord, purpose)
	}
	switch mode {
	case capability.FieldProjectionFull, capability.FieldProjectionMasked, capability.FieldProjectionPrivileged, capability.FieldProjectionOmitted:
		return mode, nil
	default:
		return "", fmt.Errorf("%w: field %s has unsupported projection mode", ErrInvalidRecord, field.Key)
	}
}

func applyProjectedRecordField(projected *Record, record Record, key string) {
	switch key {
	case "code":
		projected.Code = record.Code
	case "name":
		projected.Name = record.Name
	case "status":
		projected.Status = record.Status
	case "description":
		projected.Description = record.Description
	case "updatedAt":
		projected.UpdatedAt = record.UpdatedAt
	}
}
