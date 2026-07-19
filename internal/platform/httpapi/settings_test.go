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

	body := bytes.NewBufferString(`{"values":{"region":"ap-shanghai"}}`)
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
	after, err := server.resources.InternalRecord("notification-providers", recordID)
	if err != nil {
		t.Fatalf("read internal notification provider after update: %v", err)
	}
	if after.Values["apiSecret"] != beforeSecret || !dataprotection.IsEnvelope(after.Values["apiSecret"]) {
		t.Fatalf("stored apiSecret after update = %q, want preserved envelope %q", after.Values["apiSecret"], beforeSecret)
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
				{Key: "region", Label: capability.Text("区域", "Region"), Type: "text", Source: "values", InTable: true, InForm: true},
				{
					Key:          "apiSecret",
					Label:        capability.Text("密钥", "API Secret"),
					Type:         "text",
					Source:       "values",
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
			"provider":  "aliyun",
			"region":    "cn-hangzhou",
			"apiSecret": "provider-secret",
		},
	}); err != nil {
		t.Fatalf("seed notification provider: %v", err)
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

func settingsRuntimeField(schema adminresource.Schema, key string) *adminresource.FieldDefinition {
	for index := range schema.Fields {
		if schema.Fields[index].Key == key {
			return &schema.Fields[index]
		}
	}
	return nil
}
