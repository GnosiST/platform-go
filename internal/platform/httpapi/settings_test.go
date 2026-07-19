package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/GnosiST/platform-go/internal/platform/adminresource"
	"github.com/GnosiST/platform-go/internal/platform/capability"
	"github.com/GnosiST/platform-go/internal/platform/core"
	"github.com/GnosiST/platform-go/internal/platform/dataprotection"
)

func TestAdminSettingsRuntimeAggregatesEnabledCapabilityConfigResources(t *testing.T) {
	manifests := []capability.Manifest{
		settingsRuntimeTestManifest(),
		{
			ID:      "operations",
			Name:    "Operations",
			Version: "0.1.0",
			Admin: capability.AdminSurface{Resources: []capability.AdminResource{{
				Resource:         "operations-dashboard",
				Title:            capability.Text("运行面板", "Operations Dashboard"),
				Description:      capability.Text("运行视图。", "Operations view."),
				PermissionPrefix: "admin:operations-dashboard",
				Menu:             capability.AdminMenu{Route: "/operations-dashboard", Parent: "operations", Group: "operations"},
				Fields: []capability.AdminField{
					{Key: "enabled", Label: capability.Text("启用", "Enabled"), Type: "switch", Source: "values", InTable: true, InForm: true},
				},
			}}},
		},
	}
	server := newSettingsRuntimeTestServer(t, manifests)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/admin/settings", nil)
	request.Header.Set("X-Platform-User", "admin")
	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("GET settings runtime status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	var payload Response[adminSettingsRuntimeResponse]
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode settings runtime: %v body = %s", err, recorder.Body.String())
	}
	if payload.Data.Metrics.Resources != 1 || payload.Data.Metrics.Capabilities != 1 {
		t.Fatalf("settings metrics = %+v, want one config resource from one capability", payload.Data.Metrics)
	}
	if len(payload.Data.Items) != 1 {
		t.Fatalf("settings items = %+v, want one config resource", payload.Data.Items)
	}
	item := payload.Data.Items[0]
	if item.CapabilityID != "notification" || item.Resource != "notification-providers" || item.Route != "/notification-providers" {
		t.Fatalf("settings item = %+v, want notification provider config", item)
	}
	if item.RecordCount != 1 || len(item.Records) != 1 {
		t.Fatalf("settings records = %d %+v, want one current record", item.RecordCount, item.Records)
	}
	if item.Records[0].Values["apiSecret"] != "" || strings.Contains(recorder.Body.String(), "provider-secret") {
		t.Fatalf("settings runtime leaked provider secret: %s", recorder.Body.String())
	}
	if item.Records[0].Values["region"] != "cn-hangzhou" {
		t.Fatalf("projected public region = %q, want cn-hangzhou", item.Records[0].Values["region"])
	}
	if field := settingsRuntimeField(item.Schema, "apiSecret"); field == nil || field.ResponseMode != capability.FieldProjectionOmitted {
		t.Fatalf("schema apiSecret field = %+v, want omitted secret contract", field)
	}
	if !item.RestartRequired || item.PendingRestart || item.RuntimeApplyMode != adminSettingsApplyModeRestartRequired {
		t.Fatalf("settings item restart state = %+v, want restart-required without pending changes", item)
	}
	if item.ValidationEndpoint == "" || item.TestConnectionEndpoint == "" {
		t.Fatalf("settings item endpoints = validate %q test %q, want both endpoints", item.ValidationEndpoint, item.TestConnectionEndpoint)
	}
}

func TestAdminSettingsRuntimeIncludesCredentialAuthSecurityConfig(t *testing.T) {
	server := newSettingsRuntimeDefaultServer(t, core.DefaultManifests())

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/admin/settings", nil)
	request.Header.Set("X-Platform-User", "admin")
	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("GET settings runtime status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	var payload Response[adminSettingsRuntimeResponse]
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode settings runtime: %v body = %s", err, recorder.Body.String())
	}
	for _, resource := range []string{"notification-channels", "notification-providers", "notification-send-policies", "notification-templates", "credential-auth-settings"} {
		item := settingsRuntimeItem(payload.Data.Items, resource)
		if item == nil {
			t.Fatalf("settings runtime missing %q: %+v", resource, payload.Data.Items)
		}
		if item.ValidationEndpoint == "" {
			t.Fatalf("settings runtime item %q missing validation endpoint: %+v", resource, item)
		}
	}
	credentialItem := settingsRuntimeItem(payload.Data.Items, "credential-auth-settings")
	if credentialItem.CapabilityID != "credential-auth" || credentialItem.Group != "security" || !credentialItem.RestartRequired {
		t.Fatalf("credential-auth settings item = %+v, want security restart-required resource", credentialItem)
	}
	if credentialItem.RecordCount != 1 || credentialItem.Records[0].Values["secretTransport"] != "ecdh-a256gcm-v1" || credentialItem.Records[0].Values["passwordAlgorithm"] != "argon2id" {
		t.Fatalf("credential-auth settings record = %+v, want seeded security defaults", credentialItem.Records)
	}
}

func TestAdminSettingsRuntimeUpdatesWritableConfigRecord(t *testing.T) {
	manifests := []capability.Manifest{settingsRuntimeTestManifest()}
	server := newSettingsRuntimeTestServer(t, manifests)
	recordID := settingsRuntimeProviderRecordID(t, server)
	before, err := server.resources.InternalRecord("notification-providers", recordID)
	if err != nil {
		t.Fatalf("read internal notification provider before update: %v", err)
	}
	beforeSecret := before.Values["apiSecret"]
	if !dataprotection.IsEnvelope(beforeSecret) || strings.Contains(beforeSecret, "provider-secret") {
		t.Fatalf("stored apiSecret before update = %q, want protected envelope", beforeSecret)
	}

	body := bytes.NewBufferString(`{"values":{"region":"ap-shanghai","apiSecret":"new-provider-secret"}}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/admin/settings/notification-providers/"+recordID, body)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Platform-User", "admin")
	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("PUT settings runtime status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	var payload Response[adminSettingsMutationResponse]
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode settings update: %v body = %s", err, recorder.Body.String())
	}
	if payload.Data.Record.Values["region"] != "ap-shanghai" {
		t.Fatalf("updated region = %q, want ap-shanghai", payload.Data.Record.Values["region"])
	}
	if payload.Data.Record.Values["apiSecret"] != "" || strings.Contains(recorder.Body.String(), "provider-secret") {
		t.Fatalf("settings update leaked provider secret: %s", recorder.Body.String())
	}
	if !payload.Data.RestartRequired || !payload.Data.PendingRestart {
		t.Fatalf("settings update restart state = %+v, want pending restart", payload.Data)
	}
	after, err := server.resources.InternalRecord("notification-providers", recordID)
	if err != nil {
		t.Fatalf("read internal notification provider after update: %v", err)
	}
	if after.Values["apiSecret"] == beforeSecret || !dataprotection.IsEnvelope(after.Values["apiSecret"]) || strings.Contains(after.Values["apiSecret"], "new-provider-secret") {
		t.Fatalf("stored apiSecret after update = %q, want new protected envelope", after.Values["apiSecret"])
	}
}

func TestAdminSettingsRuntimeValidatesAndDryRunTestsConfigWithoutLeakingSecrets(t *testing.T) {
	server := newSettingsRuntimeTestServer(t, []capability.Manifest{settingsRuntimeTestManifest()})
	recordID := settingsRuntimeProviderRecordID(t, server)

	validateRecorder := httptest.NewRecorder()
	validateRequest := httptest.NewRequest(http.MethodPost, "/api/admin/settings/notification-providers/"+recordID+"/validate-config", nil)
	validateRequest.Header.Set("X-Platform-User", "admin")
	server.Router().ServeHTTP(validateRecorder, validateRequest)
	if validateRecorder.Code != http.StatusOK {
		t.Fatalf("POST validate-config status = %d body = %s", validateRecorder.Code, validateRecorder.Body.String())
	}
	var validation Response[adminSettingsValidationResponse]
	if err := json.Unmarshal(validateRecorder.Body.Bytes(), &validation); err != nil {
		t.Fatalf("decode validate-config: %v body = %s", err, validateRecorder.Body.String())
	}
	if !validation.Data.Valid || validation.Data.Status != adminSettingsStatusValid || !validation.Data.RestartRequired {
		t.Fatalf("validate-config data = %+v, want valid restart-required config", validation.Data)
	}
	if strings.Contains(validateRecorder.Body.String(), "provider-secret") {
		t.Fatalf("validate-config leaked provider secret: %s", validateRecorder.Body.String())
	}

	testRecorder := httptest.NewRecorder()
	testRequest := httptest.NewRequest(http.MethodPost, "/api/admin/settings/notification-providers/"+recordID+"/test-connect", nil)
	testRequest.Header.Set("X-Platform-User", "admin")
	server.Router().ServeHTTP(testRecorder, testRequest)
	if testRecorder.Code != http.StatusOK {
		t.Fatalf("POST test-connect status = %d body = %s", testRecorder.Code, testRecorder.Body.String())
	}
	var connection Response[adminSettingsTestConnectionResponse]
	if err := json.Unmarshal(testRecorder.Body.Bytes(), &connection); err != nil {
		t.Fatalf("decode test-connect: %v body = %s", err, testRecorder.Body.String())
	}
	if !connection.Data.Supported || !connection.Data.Connected || connection.Data.Status != adminSettingsStatusDryRun || connection.Data.Mode != "dry-run" {
		t.Fatalf("test-connect data = %+v, want SMS dry-run support", connection.Data)
	}
	if strings.Contains(testRecorder.Body.String(), "provider-secret") {
		t.Fatalf("test-connect leaked provider secret: %s", testRecorder.Body.String())
	}
}

func TestAdminSettingsRuntimeRejectsNonConfigResourceUpdate(t *testing.T) {
	server := newSettingsRuntimeTestServer(t, []capability.Manifest{
		settingsRuntimeTestManifest(),
		{
			ID:      "tenant",
			Name:    "Tenant",
			Version: "0.1.0",
			Admin: capability.AdminSurface{Resources: []capability.AdminResource{{
				Resource:         "tenants",
				Title:            capability.Text("租户", "Tenants"),
				Description:      capability.Text("租户。", "Tenants."),
				PermissionPrefix: "admin:tenant",
				Menu:             capability.AdminMenu{Route: "/tenants", Parent: "foundation", Group: "foundation"},
				Fields: []capability.AdminField{
					{Key: "isolation", Label: capability.Text("隔离", "Isolation"), Type: "text", Source: "values", InTable: true, InForm: true},
				},
			}}},
		},
	})

	body := bytes.NewBufferString(`{"name":"Bad Tenant","values":{"isolation":"shared"}}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/admin/settings/tenants/tenant-platform", body)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Platform-User", "admin")
	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("PUT non-config settings status = %d body = %s, want 404", recorder.Code, recorder.Body.String())
	}
}

func settingsRuntimeTestManifest() capability.Manifest {
	return capability.Manifest{
		ID:      "notification",
		Name:    "Notification",
		Version: "0.1.0",
		Admin: capability.AdminSurface{Resources: []capability.AdminResource{{
			Resource:         "notification-providers",
			Title:            capability.Text("消息供应商", "Notification Providers"),
			Description:      capability.Text("短信、邮箱和微信供应商配置。", "SMS, email, and WeChat provider configuration."),
			PermissionPrefix: "admin:notification-provider",
			Menu:             capability.AdminMenu{Route: "/notification-providers", Parent: "configuration", Group: "message", Icon: "api", Order: 220},
			Protection:       &capability.AdminResourceProtection{SchemaVersion: 1, Scope: "global"},
			Fields: []capability.AdminField{
				{Key: "provider", Label: capability.Text("供应商", "Provider"), Type: "select", Source: "values", Required: true, InTable: true, InForm: true},
				{Key: "channel", Label: capability.Text("渠道", "Channel"), Type: "select", Source: "values", Required: true, InTable: true, InForm: true},
				{Key: "region", Label: capability.Text("区域", "Region"), Type: "text", Source: "values", InTable: true, InForm: true},
				{
					Key:          "apiSecret",
					Label:        capability.Text("密钥", "API Secret"),
					Type:         "text",
					Source:       "values",
					InForm:       true,
					Sensitivity:  capability.FieldSensitivitySecret,
					StorageMode:  capability.FieldStorageEncrypted,
					ResponseMode: capability.FieldProjectionOmitted,
					ExportMode:   capability.FieldProjectionOmitted,
					Protection:   &capability.AdminFieldProtection{Format: "aes-256-gcm-v1", Normalization: "raw-v1"},
				},
			},
		}}},
	}
}

func newSettingsRuntimeTestServer(t *testing.T, manifests []capability.Manifest) *Server {
	t.Helper()
	manifests = withSettingsRuntimeAuditManifest(manifests)
	protection := newHTTPTestDataProtectionRuntime()
	resources, err := adminresource.NewStoreFromCapabilitiesWithProtection(manifests, protection)
	if err != nil {
		t.Fatalf("build settings runtime resource store: %v", err)
	}
	if _, err := resources.CreateInternal("notification-providers", adminresource.WriteInput{
		Code:        "aliyun",
		Name:        "Aliyun SMS",
		Status:      "enabled",
		Description: "SMS provider account.",
		Values: map[string]string{
			"provider":  "mock-local",
			"channel":   "sms",
			"region":    "cn-hangzhou",
			"apiSecret": "provider-secret",
		},
	}); err != nil {
		t.Fatalf("seed notification provider: %v", err)
	}
	return newTestServer(ServerOptions{Capabilities: manifests, Resources: resources, DataProtection: protection})
}

func newSettingsRuntimeDefaultServer(t *testing.T, manifests []capability.Manifest) *Server {
	t.Helper()
	protection := newHTTPTestDataProtectionRuntime()
	resources, err := adminresource.NewStoreFromCapabilitiesWithProtection(manifests, protection)
	if err != nil {
		t.Fatalf("build default settings runtime resource store: %v", err)
	}
	return newTestServer(ServerOptions{Capabilities: manifests, Resources: resources, DataProtection: protection})
}

func withSettingsRuntimeAuditManifest(manifests []capability.Manifest) []capability.Manifest {
	for _, manifest := range manifests {
		if manifest.ID == "audit" {
			return manifests
		}
	}
	for _, manifest := range core.DefaultManifests() {
		if manifest.ID == "audit" {
			return append(append([]capability.Manifest(nil), manifests...), manifest)
		}
	}
	return manifests
}

func settingsRuntimeProviderRecordID(t *testing.T, server *Server) string {
	t.Helper()
	records, err := server.resources.List("notification-providers")
	if err != nil {
		t.Fatalf("list notification providers: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("notification provider records = %+v, want one", records)
	}
	return records[0].ID
}

func settingsRuntimeItem(items []adminSettingsResourceItem, resource string) *adminSettingsResourceItem {
	for index := range items {
		if items[index].Resource == resource {
			return &items[index]
		}
	}
	return nil
}

func settingsRuntimeField(schema adminresource.Schema, key string) *adminresource.FieldDefinition {
	for index := range schema.Fields {
		if schema.Fields[index].Key == key {
			return &schema.Fields[index]
		}
	}
	return nil
}
