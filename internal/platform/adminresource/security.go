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
	for key := range values {
		field, declared := fields[key]
		if !declared {
			return invalidSecurityField(key, "is not declared by the active schema")
		}
		if err := validateFieldWrite(key, field, origin); err != nil {
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
		if err := validateFieldWrite(key, field, origin); err != nil {
			return err
		}
	}
	return nil
}

func validateFieldWrite(key string, field FieldDefinition, origin WriteOrigin) error {
	if origin == WriteOriginExternal && field.ReadOnly {
		return invalidSecurityField(key, "is read-only")
	}
	if origin == WriteOriginExternal && field.Sensitivity != capability.FieldSensitivityPublic {
		return invalidSecurityField(key, "is not externally writable")
	}
	if prohibitedRawField(key) && !allowsDerivedProtectedValue(field) {
		return invalidSecurityField(key, "is a prohibited raw credential or personal field")
	}
	if (field.Sensitivity == capability.FieldSensitivitySensitive || field.Sensitivity == capability.FieldSensitivitySecret) && field.StorageMode == capability.FieldStoragePlain {
		return invalidSecurityField(key, "requires protected storage")
	}
	return nil
}

func invalidSecurityField(field string, reason string) error {
	return fmt.Errorf("%w: field %s %s", ErrInvalidRecord, field, reason)
}

func prohibitedRawField(key string) bool {
	normalized := strings.ToLower(strings.NewReplacer("_", "", "-", "", ".", "").Replace(strings.TrimSpace(key)))
	if strings.HasPrefix(normalized, "masked") {
		return false
	}
	base := normalized
	for _, suffix := range []string{"hash", "digest"} {
		base = strings.TrimSuffix(base, suffix)
	}
	if (base == "code" || base == "session") && base != normalized {
		return true
	}
	normalized = base
	switch normalized {
	case "password", "passwd", "token", "accesstoken", "refreshtoken", "secret", "clientsecret", "credential", "credentials", "verificationcode", "debugcode", "providersubject", "phone", "phonenumber", "identitynumber", "idnumber", "email", "address", "detailedaddress", "sessionid", "sessionhandle", "sessiontoken":
		return true
	default:
		return strings.HasSuffix(normalized, "email") ||
			strings.HasSuffix(normalized, "phone") ||
			strings.HasSuffix(normalized, "phonenumber") ||
			strings.HasSuffix(normalized, "address") ||
			strings.HasSuffix(normalized, "identitynumber") ||
			strings.HasSuffix(normalized, "idnumber")
	}
}

func allowsDerivedProtectedValue(field FieldDefinition) bool {
	if field.StorageMode != capability.FieldStorageHashed && field.StorageMode != capability.FieldStorageEncrypted {
		return false
	}
	return field.ResponseMode == capability.FieldProjectionOmitted && field.ExportMode == capability.FieldProjectionOmitted
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
		if err := validateFieldWrite(key, field, WriteOriginInternal); err != nil {
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
				if !declared || (prohibitedRawField(key) && !allowsDerivedProtectedValue(field)) || ((field.Sensitivity == capability.FieldSensitivitySensitive || field.Sensitivity == capability.FieldSensitivitySecret) && field.StorageMode == capability.FieldStoragePlain) {
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
