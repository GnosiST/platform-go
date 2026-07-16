package adminresource

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/GnosiST/platform-go/internal/platform/capability"
	"github.com/GnosiST/platform-go/internal/platform/dataprotection"
	"github.com/GnosiST/platform-go/internal/platform/kernel"
	"github.com/GnosiST/platform-go/internal/platform/masking"
)

var (
	ErrProtectedFieldUnavailable      = errors.New("protected field is unavailable")
	ErrProtectedFieldDecryptionFailed = errors.New("protected field decryption failed")
)

type WriteOrigin string

const (
	WriteOriginExternal WriteOrigin = "external"
	WriteOriginInternal WriteOrigin = "internal"
)

type ProtectedFieldAuthorizer interface {
	AuthorizeProtectedField(context.Context, string, string, string, ProjectionPurpose) error
}

type ProtectedFieldPurpose string

const (
	ProtectedFieldPurposeSensitiveReveal ProtectedFieldPurpose = "sensitive-reveal"
	ProtectedFieldPurposeStepUpDelivery  ProtectedFieldPurpose = "step-up-delivery"
)

type ProtectedFieldRevealRequest struct {
	Resource string
	RecordID string
	Field    string
	Purpose  ProtectedFieldPurpose
}

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
		if field.StorageMode == capability.FieldStorageEncrypted && origin == WriteOriginExternal {
			if s.protection == nil {
				return invalidSecurityField(key, "requires the data protection runtime")
			}
			if field.ReadOnly {
				return invalidSecurityField(key, "is read-only")
			}
			if dataprotection.IsEnvelope(value) {
				return invalidSecurityField(key, "does not accept client-supplied envelopes")
			}
			if err := validateFieldWrite(key, value, field, WriteOriginInternal); err != nil {
				return err
			}
			continue
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
	if (field.ResponseMode == capability.FieldProjectionMasked || field.ExportMode == capability.FieldProjectionMasked) &&
		field.StorageMode != capability.FieldStorageMasked && field.StorageMode != capability.FieldStorageEncrypted {
		return invalidSecurityField(key, "masked projection requires masked or encrypted storage")
	}
	if field.StorageMode == capability.FieldStorageEncrypted &&
		(!allowsEncryptedProjection(field.ResponseMode) || !allowsEncryptedProjection(field.ExportMode)) {
		return invalidSecurityField(key, "encrypted storage must use masked, privileged or omitted response and export")
	}
	if err := validateFieldProtection(key, field); err != nil {
		return err
	}
	if err := validateFieldMasking(key, field); err != nil {
		return err
	}
	return nil
}

func validateFieldMasking(key string, field FieldDefinition) error {
	maskedProjection := field.ResponseMode == capability.FieldProjectionMasked || field.ExportMode == capability.FieldProjectionMasked
	if field.Masking == nil {
		if field.StorageMode == capability.FieldStorageEncrypted && maskedProjection {
			return invalidSecurityField(key, "encrypted masked projection requires masking metadata")
		}
		return nil
	}
	if field.StorageMode != capability.FieldStorageEncrypted {
		return invalidSecurityField(key, "masking metadata requires encrypted storage")
	}
	if !maskedProjection {
		return invalidSecurityField(key, "masking metadata requires a masked response or export")
	}
	if err := masking.NewRuntime().Validate(maskingPolicy(field)); err != nil {
		return invalidSecurityField(key, err.Error())
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

func allowsMaskedProjection(mode string) bool {
	return mode == capability.FieldProjectionMasked || mode == capability.FieldProjectionOmitted
}

func allowsEncryptedProjection(mode string) bool {
	return mode == capability.FieldProjectionMasked || mode == capability.FieldProjectionPrivileged || mode == capability.FieldProjectionOmitted
}

func (s *Store) validateSnapshot(snapshot ResourceSnapshot) error {
	for resource, records := range snapshot.Resources {
		if _, ok := s.schemas[resource]; !ok {
			return fmt.Errorf("%w: snapshot contains unknown resource %s", ErrInvalidRecord, resource)
		}
		for _, record := range records {
			if err := validateLifecycleRecord(record); err != nil {
				return fmt.Errorf("%w: resource %s record %s", err, resource, record.ID)
			}
			if err := s.validateWriteValues(resource, record.Values, WriteOriginInternal); err != nil {
				return fmt.Errorf("%w: resource %s record %s", err, resource, record.ID)
			}
			if err := s.validateStoredRecordFields(resource, record); err != nil {
				return fmt.Errorf("%w: resource %s record %s", err, resource, record.ID)
			}
			if err := s.validateProtectedRecord(context.Background(), resource, record); err != nil {
				return fmt.Errorf("%w: resource %s record %s", err, resource, record.ID)
			}
		}
	}
	return nil
}

func validateLifecycleRecord(record Record) error {
	deletedAt := strings.TrimSpace(record.DeletedAt)
	if deletedAt == "" {
		if record.DeletedBy != "" || record.DeleteReason != "" || record.PurgeAfter != "" || record.DeletionPolicyVersion != 0 {
			return fmt.Errorf("%w: active record contains lifecycle metadata", ErrInvalidRecord)
		}
		return nil
	}
	deletedTime, err := time.Parse(time.RFC3339, deletedAt)
	if err != nil {
		return fmt.Errorf("%w: deletedAt must be RFC3339", ErrInvalidRecord)
	}
	if strings.TrimSpace(record.DeletedBy) == "" || strings.TrimSpace(record.DeleteReason) == "" || record.DeletionPolicyVersion == 0 {
		return fmt.Errorf("%w: deleted record lifecycle metadata is incomplete", ErrInvalidRecord)
	}
	if strings.TrimSpace(record.PurgeAfter) == "" {
		return nil
	}
	purgeAfter, err := time.Parse(time.RFC3339, record.PurgeAfter)
	if err != nil || purgeAfter.Before(deletedTime) {
		return fmt.Errorf("%w: purgeAfter must be RFC3339 and not precede deletedAt", ErrInvalidRecord)
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
			if resource == "audit-logs" {
				var normalized bool
				record, normalized = normalizeAuditCorrelationRecord(record)
				changed = changed || normalized
			}
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

func normalizeAuditCorrelationRecord(record Record) (Record, bool) {
	normalized := cloneRecord(record)
	requestID := strings.TrimSpace(normalized.Values["requestId"])
	traceID := strings.TrimSpace(normalized.Values["traceId"])
	if kernel.ValidCorrelation(kernel.Correlation{RequestID: requestID, TraceID: traceID}) {
		changed := normalized.Values["requestId"] != requestID || normalized.Values["traceId"] != traceID
		normalized.Values["requestId"] = requestID
		normalized.Values["traceId"] = traceID
		return normalized, changed
	}

	legacyTraceID := strings.TrimSpace(normalized.Values["legacyTraceId"])
	if traceID != "" && legacyTraceID == "" {
		legacyTraceID = traceID
	}
	_, hadRequestID := normalized.Values["requestId"]
	_, hadTraceID := normalized.Values["traceId"]
	delete(normalized.Values, "requestId")
	delete(normalized.Values, "traceId")
	if legacyTraceID != "" {
		normalized.Values["legacyTraceId"] = legacyTraceID
	}
	if len(normalized.Values) == 0 {
		normalized.Values = nil
	}
	return normalized, hadRequestID || hadTraceID || normalized.Values["legacyTraceId"] != record.Values["legacyTraceId"]
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
	return projectRecordWithSchema(context.Background(), schema, resource, record, purpose, s.protection, s.masking)
}

func projectRecordWithSchema(ctx context.Context, schema Schema, resource string, record Record, purpose ProjectionPurpose, protection dataprotection.Runtime, maskingRuntime masking.Runtime) (Record, error) {
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
				projectedValue, projectErr := projectStoredValue(ctx, schema, resource, record, field, mode, value, protection, maskingRuntime)
				if projectErr != nil {
					return Record{}, projectErr
				}
				projected.Values[field.Key] = projectedValue
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

func projectStoredValue(ctx context.Context, schema Schema, resource string, record Record, field FieldDefinition, mode string, value string, protection dataprotection.Runtime, maskingRuntime masking.Runtime) (string, error) {
	if mode != capability.FieldProjectionMasked || field.StorageMode == capability.FieldStorageMasked {
		return value, nil
	}
	if field.StorageMode != capability.FieldStorageEncrypted || field.Masking == nil {
		return "", invalidSecurityField(field.Key, "masked projection is not configured")
	}
	if protection == nil || maskingRuntime == nil || !dataprotection.IsEnvelope(value) {
		return "", invalidSecurityField(field.Key, "masked projection runtime is unavailable")
	}
	policy, fieldContext, err := protectedPolicyAndContext(schema, resource, record, field)
	if err != nil {
		return "", err
	}
	plaintext, err := protection.Reveal(ctx, value, policy, fieldContext)
	if err != nil {
		return "", invalidSecurityField(field.Key, "masked projection decryption failed")
	}
	masked, err := maskingRuntime.Mask(ctx, maskingPolicy(field), plaintext)
	if err != nil {
		return "", invalidSecurityField(field.Key, "masked projection failed")
	}
	return masked, nil
}

func maskingPolicy(field FieldDefinition) masking.Policy {
	if field.Masking == nil {
		return masking.Policy{}
	}
	return masking.Policy{
		Strategy: field.Masking.Strategy, PreservePrefix: field.Masking.PreservePrefix, PreserveSuffix: field.Masking.PreserveSuffix,
		MaskLength: field.Masking.MaskLength, Replacement: field.Masking.Replacement,
	}
}

func (s *Store) ProjectRecordPrivileged(ctx context.Context, resource string, record Record, purpose ProjectionPurpose, authorizer ProtectedFieldAuthorizer) (Record, error) {
	s.mu.Lock()
	schema, ok := s.schemas[resource]
	runtime := s.protection
	s.mu.Unlock()
	if !ok {
		return Record{}, ErrUnknownResource
	}
	if authorizer == nil {
		return Record{}, fmt.Errorf("%w: protected field authorizer is required", ErrInvalidRecord)
	}
	projected, err := projectRecordWithSchema(ctx, schema, resource, record, purpose, runtime, s.masking)
	if err != nil {
		return Record{}, err
	}
	for _, rawField := range schema.Fields {
		field := defaultFieldPolicy(rawField)
		mode, modeErr := projectionMode(field, purpose)
		if modeErr != nil {
			return Record{}, modeErr
		}
		if mode != capability.FieldProjectionPrivileged || field.StorageMode != capability.FieldStorageEncrypted {
			continue
		}
		envelope, exists := record.Values[field.Key]
		if !exists {
			continue
		}
		if err := authorizer.AuthorizeProtectedField(ctx, resource, record.ID, field.Key, purpose); err != nil {
			return Record{}, err
		}
		if runtime == nil {
			return Record{}, invalidSecurityField(field.Key, "requires the data protection runtime")
		}
		policy, fieldContext, contextErr := protectedPolicyAndContext(schema, resource, record, field)
		if contextErr != nil {
			return Record{}, contextErr
		}
		value, revealErr := runtime.Reveal(ctx, envelope, policy, fieldContext)
		if revealErr != nil {
			return Record{}, fmt.Errorf("%w: protected field reveal failed", ErrInvalidRecord)
		}
		if projected.Values == nil {
			projected.Values = map[string]string{}
		}
		projected.Values[field.Key] = value
	}
	return projected, nil
}

func (s *Store) RevealProtectedField(ctx context.Context, request ProtectedFieldRevealRequest) (string, error) {
	request.Resource = strings.TrimSpace(request.Resource)
	request.RecordID = strings.TrimSpace(request.RecordID)
	request.Field = strings.TrimSpace(request.Field)
	if request.Resource == "" || request.RecordID == "" || request.Field == "" ||
		(request.Purpose != ProtectedFieldPurposeSensitiveReveal && request.Purpose != ProtectedFieldPurposeStepUpDelivery) {
		return "", errors.Join(ErrInvalidRecord, ErrProtectedFieldUnavailable)
	}
	s.mu.Lock()
	schema, ok := s.schemas[request.Resource]
	if !ok {
		s.mu.Unlock()
		return "", ErrUnknownResource
	}
	items := s.resources[request.Resource]
	index := recordIndexByID(items, request.RecordID)
	if index < 0 {
		s.mu.Unlock()
		return "", ErrRecordNotFound
	}
	record := cloneRecord(items[index])
	runtime := s.protection
	s.mu.Unlock()
	field, ok := schemaFieldByKey(schema, request.Field)
	if !ok {
		return "", errors.Join(ErrInvalidRecord, ErrProtectedFieldUnavailable)
	}
	field = defaultFieldPolicy(field)
	if field.StorageMode != capability.FieldStorageEncrypted || field.Protection == nil {
		return "", errors.Join(ErrInvalidRecord, ErrProtectedFieldUnavailable)
	}
	envelope, exists := record.Values[field.Key]
	if !exists || strings.TrimSpace(envelope) == "" || runtime == nil {
		return "", errors.Join(ErrInvalidRecord, ErrProtectedFieldUnavailable)
	}
	policy, fieldContext, err := protectedPolicyAndContext(schema, request.Resource, record, field)
	if err != nil {
		return "", errors.Join(ErrInvalidRecord, fmt.Errorf("%w: protected field context is unavailable", ErrProtectedFieldUnavailable))
	}
	value, err := runtime.Reveal(ctx, envelope, policy, fieldContext)
	if err != nil {
		return "", errors.Join(ErrInvalidRecord, fmt.Errorf("%w: protected field reveal failed", ErrProtectedFieldDecryptionFailed))
	}
	return value, nil
}

func (s *Store) validateProtectionRuntime() error {
	for _, schema := range s.schemas {
		if schemaHasEncryptedFields(schema) && s.protection == nil {
			return fmt.Errorf("%w: encrypted resources require the data protection runtime", ErrInvalidRecord)
		}
	}
	return nil
}

func (s *Store) protectRecordForStorage(ctx context.Context, resource string, record *Record, existing *Record) error {
	schema, ok := s.schemas[resource]
	if !ok {
		return ErrUnknownResource
	}
	if !schemaHasEncryptedFields(schema) {
		return nil
	}
	if s.protection == nil {
		return fmt.Errorf("%w: encrypted resources require the data protection runtime", ErrInvalidRecord)
	}
	if err := validateProtectedTenantImmutable(schema, *record, existing); err != nil {
		return err
	}
	if record.Values == nil {
		record.Values = map[string]string{}
	}
	for _, rawField := range schema.Fields {
		field := defaultFieldPolicy(rawField)
		if field.StorageMode != capability.FieldStorageEncrypted {
			continue
		}
		value, submitted := record.Values[field.Key]
		if existing != nil && (!submitted || strings.TrimSpace(value) == "") {
			if envelope, exists := existing.Values[field.Key]; exists {
				record.Values[field.Key] = envelope
			}
			continue
		}
		if !submitted {
			continue
		}
		if dataprotection.IsEnvelope(value) {
			return invalidSecurityField(field.Key, "does not accept client-supplied envelopes")
		}
		policy, fieldContext, err := protectedPolicyAndContext(schema, resource, *record, field)
		if err != nil {
			return err
		}
		envelope, err := s.protection.Protect(ctx, value, policy, fieldContext)
		if err != nil {
			return fmt.Errorf("%w: field %s protection failed", ErrInvalidRecord, field.Key)
		}
		record.Values[field.Key] = envelope
	}
	if len(record.Values) == 0 {
		record.Values = nil
	}
	return nil
}

func (s *Store) ValidateProtectedData(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.validateProtectedDataLocked(ctx)
}

func (s *Store) validateProtectedDataLocked(ctx context.Context) error {
	if err := s.validateProtectionRuntime(); err != nil {
		return err
	}
	for resource, records := range s.resources {
		for _, record := range records {
			if err := s.validateProtectedRecord(ctx, resource, record); err != nil {
				return fmt.Errorf("%w: resource %s record %s", err, resource, record.ID)
			}
		}
	}
	return nil
}

func (s *Store) validateProtectedRecord(ctx context.Context, resource string, record Record) error {
	schema, ok := s.schemas[resource]
	if !ok || !schemaHasEncryptedFields(schema) {
		return nil
	}
	if s.protection == nil {
		return fmt.Errorf("%w: encrypted resources require the data protection runtime", ErrInvalidRecord)
	}
	if _, err := protectedTenantID(schema, record); err != nil {
		return err
	}
	for _, rawField := range schema.Fields {
		field := defaultFieldPolicy(rawField)
		if field.StorageMode != capability.FieldStorageEncrypted {
			continue
		}
		envelope, exists := record.Values[field.Key]
		if !exists {
			continue
		}
		if !dataprotection.IsEnvelope(envelope) {
			return invalidSecurityField(field.Key, "does not contain a valid envelope")
		}
		policy, fieldContext, err := protectedPolicyAndContext(schema, resource, record, field)
		if err != nil {
			return err
		}
		if err := s.protection.Validate(ctx, envelope, policy, fieldContext); err != nil {
			return fmt.Errorf("%w: field %s envelope validation failed", ErrInvalidRecord, field.Key)
		}
	}
	return nil
}

func protectedPolicyAndContext(schema Schema, resource string, record Record, field FieldDefinition) (dataprotection.FieldPolicy, dataprotection.FieldContext, error) {
	if field.Protection == nil || schema.Protection == nil {
		return dataprotection.FieldPolicy{}, dataprotection.FieldContext{}, invalidSecurityField(field.Key, "is missing protection context")
	}
	tenantID, err := protectedTenantID(schema, record)
	if err != nil {
		return dataprotection.FieldPolicy{}, dataprotection.FieldContext{}, err
	}
	return dataprotection.FieldPolicy{
			Format: field.Protection.Format, Normalization: field.Protection.Normalization, BlindIndexNamespace: field.Protection.BlindIndexNamespace,
		}, dataprotection.FieldContext{
			TenantID: tenantID, Resource: resource, RecordID: record.ID, FieldKey: field.Key, SchemaVersion: schema.Protection.SchemaVersion,
		}, nil
}

func protectedTenantID(schema Schema, record Record) (string, error) {
	if schema.Protection == nil {
		return "", fmt.Errorf("%w: protected resource context is missing", ErrInvalidRecord)
	}
	switch schema.Protection.Scope {
	case "global":
		return dataprotection.GlobalTenantID, nil
	case "tenant-field":
		field, ok := schemaFieldByKey(schema, schema.Protection.TenantField)
		if !ok {
			return "", fmt.Errorf("%w: protected tenant field is missing", ErrInvalidRecord)
		}
		tenantID := strings.TrimSpace(storedFieldValue(record, field))
		if tenantID == "" {
			return "", invalidSecurityField(field.Key, "is required for protected data")
		}
		return tenantID, nil
	default:
		return "", fmt.Errorf("%w: protected resource scope is unsupported", ErrInvalidRecord)
	}
}

func validateProtectedTenantImmutable(schema Schema, record Record, existing *Record) error {
	if existing == nil || schema.Protection == nil || schema.Protection.Scope != "tenant-field" {
		return nil
	}
	field, ok := schemaFieldByKey(schema, schema.Protection.TenantField)
	if !ok {
		return fmt.Errorf("%w: protected tenant field is missing", ErrInvalidRecord)
	}
	if strings.TrimSpace(storedFieldValue(*existing, field)) != strings.TrimSpace(storedFieldValue(record, field)) {
		return invalidSecurityField(field.Key, "is immutable for protected data")
	}
	return nil
}

func schemaHasEncryptedFields(schema Schema) bool {
	for _, field := range schema.Fields {
		if field.StorageMode == capability.FieldStorageEncrypted {
			return true
		}
	}
	return false
}

func schemaFieldByKey(schema Schema, key string) (FieldDefinition, bool) {
	for _, field := range schema.Fields {
		if field.Key == key {
			return field, true
		}
	}
	return FieldDefinition{}, false
}

func storedFieldValue(record Record, field FieldDefinition) string {
	if field.Source == "values" {
		return record.Values[field.Key]
	}
	switch field.Key {
	case "code":
		return record.Code
	case "name":
		return record.Name
	case "status":
		return record.Status
	case "description":
		return record.Description
	case "updatedAt":
		return record.UpdatedAt
	default:
		return ""
	}
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
