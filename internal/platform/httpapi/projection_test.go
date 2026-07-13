package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"platform-go/internal/platform/adminresource"
	"platform-go/internal/platform/capability"
	"platform-go/internal/platform/core"
	"platform-go/internal/platform/dataprotection"
	"platform-go/internal/platform/masking"
)

const (
	protectedHTTPResource = "protected-http-records"
	protectedHTTPField    = "phoneNumber"
)

func TestAdminResourceAPIsProjectEncryptedMaskedValuesOnce(t *testing.T) {
	manifests := protectedHTTPProjectionManifests(t)
	runtime := newCountingHTTPProtectionRuntime(t)
	resources, err := adminresource.NewStoreFromCapabilitiesWithProtection(manifests, runtime)
	if err != nil {
		t.Fatal(err)
	}
	server := newTestServer(ServerOptions{
		Capabilities: manifests,
		Resources:    resources,
		Authorizer:   allowAllHTTPProjectionAuthorizer{},
	})

	create := httptest.NewRecorder()
	createRequest := newAdminProjectionRequest(http.MethodPost, "/api/admin/resources/"+protectedHTTPResource,
		`{"code":"protected-http-1","name":"Protected HTTP","status":"enabled","values":{"phoneNumber":"13800138000"}}`)
	runtime.revealCalls = 0
	server.Router().ServeHTTP(create, createRequest)
	assertSingleHTTPProjection(t, create, runtime, http.StatusCreated, "138****8000")

	var created adminResourceRecordTestPayload
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v body = %s", err, create.Body.String())
	}
	recordID := created.Data.Record.ID

	query := httptest.NewRecorder()
	queryRequest := newAdminProjectionRequest(http.MethodPost, "/api/admin/resources/"+protectedHTTPResource+"/query", `{"page":1,"pageSize":10}`)
	runtime.revealCalls = 0
	server.Router().ServeHTTP(query, queryRequest)
	assertSingleHTTPProjection(t, query, runtime, http.StatusOK, "138****8000")

	update := httptest.NewRecorder()
	updateRequest := newAdminProjectionRequest(http.MethodPut, "/api/admin/resources/"+protectedHTTPResource+"/"+recordID,
		`{"code":"protected-http-1","name":"Protected HTTP","status":"enabled","values":{"phoneNumber":"13900139000"}}`)
	runtime.revealCalls = 0
	server.Router().ServeHTTP(update, updateRequest)
	assertSingleHTTPProjection(t, update, runtime, http.StatusOK, "139****9000")
}

func TestPolicyReviewExportProjectsEncryptedMaskedValuesOnce(t *testing.T) {
	manifests := protectedHTTPProjectionManifests(t)
	runtime := newCountingHTTPProtectionRuntime(t)
	resources, err := adminresource.NewStoreFromCapabilitiesWithProtection(manifests, runtime)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := resources.CreateInternal("policy-reviews", adminresource.WriteInput{
		Code: "PR-PROJECTION-1", Name: "Projection review", Status: "enabled",
		Values: map[string]string{
			"policyType": "deny_permission", "requestedAction": "update", "reviewStatus": "draft",
			"rejectionReason": "private-review-reason",
		},
	}); err != nil {
		t.Fatal(err)
	}
	server := newTestServer(ServerOptions{
		Capabilities: manifests,
		Resources:    resources,
		Authorizer:   allowAllHTTPProjectionAuthorizer{},
	})

	recorder := httptest.NewRecorder()
	request := newAdminProjectionRequest(http.MethodGet, "/api/admin/policy-reviews/export", "")
	runtime.revealCalls = 0
	server.Router().ServeHTTP(recorder, request)
	assertSingleHTTPProjection(t, recorder, runtime, http.StatusOK, "pr****")
}

func protectedHTTPProjectionManifests(t *testing.T) []capability.Manifest {
	t.Helper()
	manifests := core.DefaultManifests()
	policyReviewConfigured := false
	for manifestIndex := range manifests {
		for resourceIndex := range manifests[manifestIndex].Admin.Resources {
			resource := &manifests[manifestIndex].Admin.Resources[resourceIndex]
			if resource.Resource != "policy-reviews" {
				continue
			}
			resource.Protection = &capability.AdminResourceProtection{SchemaVersion: 1, Scope: "global"}
			for fieldIndex := range resource.Fields {
				field := &resource.Fields[fieldIndex]
				if field.Key != "rejectionReason" {
					continue
				}
				configureEncryptedMaskedHTTPField(field, masking.StrategyPartialV1)
				field.Masking.PreservePrefix = 2
				field.Masking.MaskLength = 4
				policyReviewConfigured = true
			}
		}
	}
	if !policyReviewConfigured {
		t.Fatal("policy-reviews.rejectionReason field was not found")
	}

	return append(manifests, capability.Manifest{
		ID: "protected-http-test",
		Admin: capability.AdminSurface{Resources: []capability.AdminResource{{
			Resource: protectedHTTPResource, Title: capability.Text("HTTP 投影测试", "HTTP Projection Test"),
			Description: capability.Text("测试 HTTP 投影次数。", "Tests HTTP projection count."), PermissionPrefix: "admin:protected-http",
			Protection: &capability.AdminResourceProtection{SchemaVersion: 1, Scope: "global"},
			Fields: []capability.AdminField{
				{Key: "code", Label: capability.Text("编码", "Code"), Type: "text", Source: "record", Required: true, InForm: true, InTable: true, InDetail: true},
				{Key: "name", Label: capability.Text("名称", "Name"), Type: "text", Source: "record", Required: true, InForm: true, InTable: true, InDetail: true},
				{Key: "status", Label: capability.Text("状态", "Status"), Type: "text", Source: "record", InForm: true, InTable: true, InDetail: true},
				protectedHTTPAdminField(),
			},
			SearchFields: []string{"code", "name"}, DefaultSortKey: "code",
		}}},
	})
}

func protectedHTTPAdminField() capability.AdminField {
	field := capability.AdminField{
		Key: protectedHTTPField, Label: capability.Text("手机号", "Phone Number"), Type: "text", Source: "values",
		Required: true, InForm: true, InTable: true, InDetail: true,
	}
	configureEncryptedMaskedHTTPField(&field, masking.StrategyPhoneV1)
	return field
}

func configureEncryptedMaskedHTTPField(field *capability.AdminField, strategy string) {
	field.Sensitivity = capability.FieldSensitivitySensitive
	field.StorageMode = capability.FieldStorageEncrypted
	field.ResponseMode = capability.FieldProjectionMasked
	field.ExportMode = capability.FieldProjectionMasked
	field.Protection = &capability.AdminFieldProtection{
		Format: dataprotection.FormatAES256GCMV1, Normalization: dataprotection.NormalizationRawV1,
	}
	field.Masking = &capability.AdminFieldMasking{Strategy: strategy}
}

func newAdminProjectionRequest(method string, target string, body string) *http.Request {
	request := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	request.Header.Set("X-Platform-User", "admin")
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	return request
}

func assertSingleHTTPProjection(t *testing.T, recorder *httptest.ResponseRecorder, runtime *countingHTTPProtectionRuntime, status int, masked string) {
	t.Helper()
	if recorder.Code != status {
		t.Fatalf("status = %d body = %s, want %d", recorder.Code, recorder.Body.String(), status)
	}
	if runtime.revealCalls != 1 {
		t.Fatalf("Reveal calls = %d, want one API projection", runtime.revealCalls)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, masked) || strings.Contains(body, "13800138000") || strings.Contains(body, "13900139000") || strings.Contains(body, "private-review-reason") {
		t.Fatalf("response projection = %s, want masked value %q without plaintext", body, masked)
	}
}

type allowAllHTTPProjectionAuthorizer struct{}

func (allowAllHTTPProjectionAuthorizer) Can(string, string, string, string) bool { return true }

type countingHTTPProtectionRuntime struct {
	delegate    dataprotection.Runtime
	revealCalls int
}

func newCountingHTTPProtectionRuntime(t *testing.T) *countingHTTPProtectionRuntime {
	t.Helper()
	provider, err := dataprotection.NewStaticKeyProvider(dataprotection.StaticKeyProviderConfig{
		Kind: dataprotection.ProviderEnvAES256, ActiveEncryptionKeyID: "enc-v1", ActiveBlindIndexKeyID: "idx-v1",
		EncryptionKeys: map[string][]byte{"enc-v1": []byte(strings.Repeat("e", 32))},
		BlindIndexKeys: map[string][]byte{"idx-v1": []byte(strings.Repeat("i", 32))},
	})
	if err != nil {
		t.Fatal(err)
	}
	return &countingHTTPProtectionRuntime{delegate: dataprotection.NewRuntime(provider)}
}

func (r *countingHTTPProtectionRuntime) Protect(ctx context.Context, value string, policy dataprotection.FieldPolicy, fieldContext dataprotection.FieldContext) (string, error) {
	return r.delegate.Protect(ctx, value, policy, fieldContext)
}

func (r *countingHTTPProtectionRuntime) Validate(ctx context.Context, value string, policy dataprotection.FieldPolicy, fieldContext dataprotection.FieldContext) error {
	return r.delegate.Validate(ctx, value, policy, fieldContext)
}

func (r *countingHTTPProtectionRuntime) Reveal(ctx context.Context, value string, policy dataprotection.FieldPolicy, fieldContext dataprotection.FieldContext) (string, error) {
	r.revealCalls++
	return r.delegate.Reveal(ctx, value, policy, fieldContext)
}

func (r *countingHTTPProtectionRuntime) MatchExact(ctx context.Context, envelope string, candidate string, policy dataprotection.FieldPolicy, fieldContext dataprotection.FieldContext) (bool, error) {
	return r.delegate.MatchExact(ctx, envelope, candidate, policy, fieldContext)
}
