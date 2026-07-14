package httpapi

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"
	"time"

	"platform-go/internal/platform/adminresource"
	"platform-go/internal/platform/capability"
)

var ErrAdminStepUpPhoneUnavailable = errors.New("admin step-up phone is unavailable")
var ErrAdminStepUpPhoneConfiguration = errors.New("admin step-up phone source is invalid")

type AdminStepUpPhone struct {
	Phone       string
	MaskedPhone string
}

type AdminStepUpPhoneResolver interface {
	ResolveVerifiedAdminPhone(context.Context, string) (AdminStepUpPhone, error)
}

type AdminStepUpPhoneSource struct {
	Resource                 string
	ActorField               string
	PhoneField               string
	VerifiedAtField          string
	VerifiedPhoneDigestField string
}

type resourceAdminStepUpPhoneResolver struct {
	resources *adminresource.Store
	protector PhoneProtector
	source    AdminStepUpPhoneSource
	now       func() time.Time
}

func NewResourceAdminStepUpPhoneResolver(resources *adminresource.Store, protector PhoneProtector, source AdminStepUpPhoneSource, now func() time.Time) (AdminStepUpPhoneResolver, error) {
	source.Resource = strings.TrimSpace(source.Resource)
	source.ActorField = strings.TrimSpace(source.ActorField)
	source.PhoneField = strings.TrimSpace(source.PhoneField)
	source.VerifiedAtField = strings.TrimSpace(source.VerifiedAtField)
	source.VerifiedPhoneDigestField = strings.TrimSpace(source.VerifiedPhoneDigestField)
	if resources == nil || protector == nil {
		return nil, fmt.Errorf("%w: resource store and phone protector are required", ErrAdminStepUpPhoneConfiguration)
	}
	if source.Resource == "" || source.ActorField == "" || source.PhoneField == "" || source.VerifiedAtField == "" || source.VerifiedPhoneDigestField == "" {
		return nil, fmt.Errorf("%w: resource and field names are required", ErrAdminStepUpPhoneConfiguration)
	}
	fields := []string{source.ActorField, source.PhoneField, source.VerifiedAtField, source.VerifiedPhoneDigestField}
	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		if _, exists := seen[field]; exists {
			return nil, fmt.Errorf("%w: source fields must be distinct", ErrAdminStepUpPhoneConfiguration)
		}
		seen[field] = struct{}{}
	}
	schema, err := resources.Schema(source.Resource)
	if err != nil {
		return nil, fmt.Errorf("%w: resource %q is unavailable", ErrAdminStepUpPhoneConfiguration, source.Resource)
	}
	fieldByKey := make(map[string]adminresource.FieldDefinition, len(schema.Fields))
	for _, field := range schema.Fields {
		fieldByKey[field.Key] = field
	}
	checks := []struct {
		name        string
		key         string
		fieldType   string
		storageMode string
		mustProtect bool
	}{
		{name: "actor", key: source.ActorField, fieldType: "text", storageMode: capability.FieldStoragePlain},
		{name: "phone", key: source.PhoneField, fieldType: "text", storageMode: capability.FieldStorageEncrypted, mustProtect: true},
		{name: "verified timestamp", key: source.VerifiedAtField, fieldType: "datetime", storageMode: capability.FieldStoragePlain},
		{name: "verified phone digest", key: source.VerifiedPhoneDigestField, fieldType: "text", storageMode: capability.FieldStoragePlain},
	}
	for _, check := range checks {
		field, exists := fieldByKey[check.key]
		if !exists {
			return nil, fmt.Errorf("%w: %s field %q is not declared", ErrAdminStepUpPhoneConfiguration, check.name, check.key)
		}
		if field.Source != "values" || field.Type != check.fieldType || field.StorageMode != check.storageMode || (check.mustProtect && field.Protection == nil) {
			return nil, fmt.Errorf("%w: %s field %q has incompatible schema semantics", ErrAdminStepUpPhoneConfiguration, check.name, check.key)
		}
	}
	if now == nil {
		now = time.Now
	}
	return &resourceAdminStepUpPhoneResolver{resources: resources, protector: protector, source: source, now: now}, nil
}

func (r *resourceAdminStepUpPhoneResolver) ResolveVerifiedAdminPhone(ctx context.Context, username string) (AdminStepUpPhone, error) {
	if err := ctx.Err(); err != nil {
		return AdminStepUpPhone{}, err
	}
	username = strings.TrimSpace(username)
	source := r.source
	if r.resources == nil || r.protector == nil || username == "" || source.Resource == "" || source.ActorField == "" || source.PhoneField == "" || source.VerifiedAtField == "" || source.VerifiedPhoneDigestField == "" {
		return AdminStepUpPhone{}, ErrAdminStepUpPhoneUnavailable
	}
	records, err := r.resources.List(source.Resource)
	if err != nil {
		return AdminStepUpPhone{}, ErrAdminStepUpPhoneUnavailable
	}
	var match *adminresource.Record
	for index := range records {
		record := records[index]
		if record.Status == "disabled" || strings.TrimSpace(record.Values[source.ActorField]) != username {
			continue
		}
		verifiedAt, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(record.Values[source.VerifiedAtField]))
		if parseErr != nil || verifiedAt.After(r.now().UTC()) {
			continue
		}
		if match != nil {
			return AdminStepUpPhone{}, ErrAdminStepUpPhoneUnavailable
		}
		copy := record
		match = &copy
	}
	if match == nil {
		return AdminStepUpPhone{}, ErrAdminStepUpPhoneUnavailable
	}
	phone, err := r.resources.RevealProtectedField(ctx, adminresource.ProtectedFieldRevealRequest{
		Resource: source.Resource, RecordID: match.ID, Field: source.PhoneField, Purpose: adminresource.ProtectedFieldPurposeStepUpDelivery,
	})
	if err != nil {
		return AdminStepUpPhone{}, ErrAdminStepUpPhoneUnavailable
	}
	phone, ok := normalizeAppPhone(phone)
	if !ok {
		return AdminStepUpPhone{}, ErrAdminStepUpPhoneUnavailable
	}
	phoneDigest, err := r.protector.PhoneDigest(phone)
	if err != nil || subtle.ConstantTimeCompare([]byte(phoneDigest), []byte(strings.TrimSpace(match.Values[source.VerifiedPhoneDigestField]))) != 1 {
		return AdminStepUpPhone{}, ErrAdminStepUpPhoneUnavailable
	}
	return AdminStepUpPhone{Phone: phone, MaskedPhone: maskAppPhone(phone)}, nil
}
