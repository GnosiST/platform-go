package httpapi

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"platform-go/internal/platform/adminresource"
	"platform-go/internal/platform/capability"
	"platform-go/internal/platform/dataprotection"
)

func TestResourceAdminStepUpPhoneResolverBindsVerificationToCurrentPhone(t *testing.T) {
	now := time.Date(2026, time.July, 13, 10, 0, 0, 0, time.UTC)
	protector := NewHMACPhoneProtector([]byte(strings.Repeat("p", 32)), []byte(strings.Repeat("c", 32)))
	verifiedPhone := "13800138000"
	verifiedDigest, err := protector.PhoneDigest(verifiedPhone)
	if err != nil {
		t.Fatalf("PhoneDigest() error = %v", err)
	}
	resources := newResourceAdminStepUpPhoneTestStore(t, nil)
	values := map[string]string{
		"accountCode": "admin", "mobile": verifiedPhone,
		"mobileVerifiedAt": now.Add(-time.Hour).Format(time.RFC3339), "mobileVerifiedDigest": verifiedDigest,
	}
	record, err := resources.CreateInternal("staff-profiles", adminresource.WriteInput{
		Code: "staff-admin", Name: "Admin", Status: "enabled", Values: values,
	})
	if err != nil {
		t.Fatalf("CreateInternal() error = %v", err)
	}
	resolver, err := NewResourceAdminStepUpPhoneResolver(resources, protector, AdminStepUpPhoneSource{
		Resource: "staff-profiles", ActorField: "accountCode", PhoneField: "mobile",
		VerifiedAtField: "mobileVerifiedAt", VerifiedPhoneDigestField: "mobileVerifiedDigest",
	}, func() time.Time { return now })
	if err != nil {
		t.Fatalf("NewResourceAdminStepUpPhoneResolver() error = %v", err)
	}
	resolved, err := resolver.ResolveVerifiedAdminPhone(context.Background(), "admin")
	if err != nil || resolved.Phone != verifiedPhone {
		t.Fatalf("ResolveVerifiedAdminPhone() = %+v, %v", resolved, err)
	}

	values["mobile"] = "13900139000"
	if _, err := resources.UpdateInternal("staff-profiles", record.ID, adminresource.WriteInput{
		Code: record.Code, Name: record.Name, Status: record.Status, Values: values,
	}); err != nil {
		t.Fatalf("UpdateInternal() error = %v", err)
	}
	if _, err := resolver.ResolveVerifiedAdminPhone(context.Background(), "admin"); !errors.Is(err, ErrAdminStepUpPhoneUnavailable) {
		t.Fatalf("ResolveVerifiedAdminPhone() after phone change error = %v, want unavailable", err)
	}
}

func TestResourceAdminStepUpPhoneResolverRejectsInvalidSourceSchema(t *testing.T) {
	protector := NewHMACPhoneProtector([]byte(strings.Repeat("p", 32)), []byte(strings.Repeat("c", 32)))
	validSource := AdminStepUpPhoneSource{
		Resource: "staff-profiles", ActorField: "accountCode", PhoneField: "mobile",
		VerifiedAtField: "mobileVerifiedAt", VerifiedPhoneDigestField: "mobileVerifiedDigest",
	}
	tests := []struct {
		name   string
		source func(AdminStepUpPhoneSource) AdminStepUpPhoneSource
		mutate func(*capability.AdminResource)
	}{
		{name: "unknown resource", source: func(source AdminStepUpPhoneSource) AdminStepUpPhoneSource { source.Resource = "missing"; return source }},
		{name: "missing actor", source: func(source AdminStepUpPhoneSource) AdminStepUpPhoneSource {
			source.ActorField = "missing"
			return source
		}},
		{name: "missing phone", source: func(source AdminStepUpPhoneSource) AdminStepUpPhoneSource {
			source.PhoneField = "missing"
			return source
		}},
		{name: "missing verified timestamp", source: func(source AdminStepUpPhoneSource) AdminStepUpPhoneSource {
			source.VerifiedAtField = "missing"
			return source
		}},
		{name: "missing digest", source: func(source AdminStepUpPhoneSource) AdminStepUpPhoneSource {
			source.VerifiedPhoneDigestField = "missing"
			return source
		}},
		{name: "duplicate actor and digest", source: func(source AdminStepUpPhoneSource) AdminStepUpPhoneSource {
			source.VerifiedPhoneDigestField = source.ActorField
			return source
		}},
		{name: "actor record source", source: func(source AdminStepUpPhoneSource) AdminStepUpPhoneSource { source.ActorField = "code"; return source }},
		{name: "phone record source", source: func(source AdminStepUpPhoneSource) AdminStepUpPhoneSource { source.PhoneField = "code"; return source }},
		{name: "digest record source", source: func(source AdminStepUpPhoneSource) AdminStepUpPhoneSource {
			source.VerifiedPhoneDigestField = "code"
			return source
		}},
		{name: "actor wrong type", mutate: func(resource *capability.AdminResource) {
			mutateAdminStepUpPhoneField(resource, "accountCode", func(field *capability.AdminField) { field.Type = "datetime" })
		}},
		{name: "verified timestamp wrong type", mutate: func(resource *capability.AdminResource) {
			mutateAdminStepUpPhoneField(resource, "mobileVerifiedAt", func(field *capability.AdminField) { field.Type = "text" })
		}},
		{name: "phone not encrypted", mutate: func(resource *capability.AdminResource) {
			mutateAdminStepUpPhoneField(resource, "mobile", func(field *capability.AdminField) {
				field.Sensitivity = capability.FieldSensitivityPublic
				field.StorageMode = capability.FieldStoragePlain
				field.Protection = nil
			})
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resources := newResourceAdminStepUpPhoneTestStore(t, test.mutate)
			source := validSource
			if test.source != nil {
				source = test.source(source)
			}
			if _, err := NewResourceAdminStepUpPhoneResolver(resources, protector, source, time.Now); !errors.Is(err, ErrAdminStepUpPhoneConfiguration) {
				t.Fatalf("NewResourceAdminStepUpPhoneResolver() error = %v, want configuration error", err)
			}
		})
	}

	resources := newResourceAdminStepUpPhoneTestStore(t, nil)
	if _, err := NewResourceAdminStepUpPhoneResolver(nil, protector, validSource, time.Now); !errors.Is(err, ErrAdminStepUpPhoneConfiguration) {
		t.Fatalf("nil resource store error = %v, want configuration error", err)
	}
	if _, err := NewResourceAdminStepUpPhoneResolver(resources, nil, validSource, time.Now); !errors.Is(err, ErrAdminStepUpPhoneConfiguration) {
		t.Fatalf("nil protector error = %v, want configuration error", err)
	}
}

func newResourceAdminStepUpPhoneTestStore(t *testing.T, mutate func(*capability.AdminResource)) *adminresource.Store {
	t.Helper()
	resource := capability.AdminResource{
		Resource: "staff-profiles", Title: capability.Text("员工资料", "Staff Profiles"), PermissionPrefix: "admin:staff-profile",
		Protection: &capability.AdminResourceProtection{SchemaVersion: 1, Scope: "global"},
		Fields: []capability.AdminField{
			{Key: "code", Label: capability.Text("编码", "Code"), Type: "text", Source: "record", Required: true, InTable: true, InDetail: true},
			{Key: "name", Label: capability.Text("名称", "Name"), Type: "text", Source: "record", Required: true, InTable: true, InDetail: true},
			{Key: "status", Label: capability.Text("状态", "Status"), Type: "text", Source: "record", InTable: true, InDetail: true},
			{Key: "accountCode", Label: capability.Text("账号", "Account"), Type: "text", Source: "values", Required: true, InDetail: true},
			{
				Key: "mobile", Label: capability.Text("手机号", "Mobile"), Type: "text", Source: "values", Required: true,
				Sensitivity: capability.FieldSensitivitySensitive, StorageMode: capability.FieldStorageEncrypted,
				ResponseMode: capability.FieldProjectionOmitted, ExportMode: capability.FieldProjectionOmitted,
				Protection: &capability.AdminFieldProtection{Format: dataprotection.FormatAES256GCMV1, Normalization: dataprotection.NormalizationRawV1},
			},
			{Key: "mobileVerifiedAt", Label: capability.Text("手机号验证时间", "Mobile Verified At"), Type: "datetime", Source: "values", Required: true},
			{Key: "mobileVerifiedDigest", Label: capability.Text("手机号验证摘要", "Mobile Verified Digest"), Type: "text", Source: "values", Required: true},
		},
	}
	if mutate != nil {
		mutate(&resource)
	}
	provider, err := dataprotection.NewStaticKeyProvider(dataprotection.StaticKeyProviderConfig{
		Kind: dataprotection.ProviderEnvAES256, ActiveEncryptionKeyID: "enc-v1", ActiveBlindIndexKeyID: "idx-v1",
		EncryptionKeys: map[string][]byte{"enc-v1": []byte(strings.Repeat("e", 32))},
		BlindIndexKeys: map[string][]byte{"idx-v1": []byte(strings.Repeat("i", 32))},
	})
	if err != nil {
		t.Fatalf("NewStaticKeyProvider() error = %v", err)
	}
	resources, err := adminresource.NewStoreFromCapabilitiesWithProtection([]capability.Manifest{{
		ID: "step-up-phone-test", Name: "Step Up Phone Test", Version: "0.1.0",
		Admin: capability.AdminSurface{Resources: []capability.AdminResource{resource}},
	}}, dataprotection.NewRuntime(provider))
	if err != nil {
		t.Fatalf("NewStoreFromCapabilitiesWithProtection() error = %v", err)
	}
	return resources
}

func mutateAdminStepUpPhoneField(resource *capability.AdminResource, key string, mutate func(*capability.AdminField)) {
	for index := range resource.Fields {
		if resource.Fields[index].Key == key {
			mutate(&resource.Fields[index])
			return
		}
	}
}
