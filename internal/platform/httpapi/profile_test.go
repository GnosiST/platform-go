package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/GnosiST/platform-go/internal/platform/adminresource"
	"github.com/GnosiST/platform-go/internal/platform/capability"
)

type adminProfileTestPayload struct {
	Data struct {
		Profile struct {
			ID          string `json:"id"`
			Username    string `json:"username"`
			Name        string `json:"name"`
			AvatarURL   string `json:"avatarUrl"`
			Phone       string `json:"phone"`
			MaskedPhone string `json:"maskedPhone"`
			Email       string `json:"email"`
			MaskedEmail string `json:"maskedEmail"`
			Address     string `json:"address"`
			TenantCode  string `json:"tenantCode"`
			OrgUnitCode string `json:"orgUnitCode"`
			AreaCode    string `json:"areaCode"`
			Credentials struct {
				PasswordChange string `json:"passwordChange"`
			} `json:"credentials"`
		} `json:"profile"`
	} `json:"data"`
}

func TestAdminProfileRequiresAuthenticatedAdminSession(t *testing.T) {
	server, _ := newAdminProfileTestServer(t)

	recorder := putAdminProfileForTest(server, "", "/api/admin/profile/current", `{"name":"Ops Prime"}`)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("PUT profile without token status = %d body = %s, want 401", recorder.Code, recorder.Body.String())
	}
}

func TestAdminProfileRejectsUpdatingAnotherUser(t *testing.T) {
	server, resources := newAdminProfileTestServer(t)
	login := loginForTest(t, server, "ops")

	recorder := putAdminProfileForTest(server, login.Data.Token, "/api/admin/profile/user-admin", `{"name":"Hijacked"}`)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("PUT profile for another user status = %d body = %s, want 403", recorder.Code, recorder.Body.String())
	}
	admin, err := resources.InternalRecord("users", "user-admin")
	if err != nil {
		t.Fatalf("read admin user: %v", err)
	}
	if admin.Name == "Hijacked" {
		t.Fatalf("another user record was updated: %+v", admin)
	}
}

func TestAdminProfileRejectsInvalidEmailAndPhone(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "invalid email", body: `{"email":"not-an-email"}`},
		{name: "invalid phone", body: `{"phone":"13x800138000"}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server, _ := newAdminProfileTestServer(t)
			login := loginForTest(t, server, "ops")

			recorder := putAdminProfileForTest(server, login.Data.Token, "/api/admin/profile/current", test.body)

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("PUT profile status = %d body = %s, want 400", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestAdminProfileRejectsFieldsOutsideProfileWhitelist(t *testing.T) {
	server, resources := newAdminProfileTestServer(t)
	login := loginForTest(t, server, "ops")

	recorder := putAdminProfileForTest(server, login.Data.Token, "/api/admin/profile/current", `{"roles":"super-admin","tenantCode":"demo"}`)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("PUT profile with forbidden fields status = %d body = %s, want 400", recorder.Code, recorder.Body.String())
	}
	stored, err := resources.InternalRecord("users", "user-ops")
	if err != nil {
		t.Fatalf("read user after rejected profile update: %v", err)
	}
	if stored.Values["roles"] != "operator" || stored.Values["tenantCode"] != "platform" {
		t.Fatalf("forbidden profile fields changed user record: %+v", stored.Values)
	}
}

func TestAdminProfileUpdatesCurrentUserResourceRecord(t *testing.T) {
	server, resources := newAdminProfileTestServer(t)
	login := loginForTest(t, server, "ops")

	recorder := putAdminProfileForTest(server, login.Data.Token, "/api/admin/profile/current", `{
		"avatarUrl":"https://cdn.example.test/ops.png",
		"nickname":" Ops Prime ",
		"phone":"+86 (138) 0013-8000",
		"email":"OPS@EXAMPLE.TEST",
		"address":" 北京市朝阳区建国路 88 号 "
	}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("PUT profile status = %d body = %s, want 200", recorder.Code, recorder.Body.String())
	}
	var payload adminProfileTestPayload
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode profile response: %v body = %s", err, recorder.Body.String())
	}
	profile := payload.Data.Profile
	if profile.ID != "user-ops" || profile.Username != "ops" || profile.Name != "Ops Prime" {
		t.Fatalf("profile identity = %+v, want normalized current ops profile", profile)
	}
	if profile.AvatarURL != "https://cdn.example.test/ops.png" || profile.Phone != "+8613800138000" || profile.Email != "ops@example.test" || profile.Address != "北京市朝阳区建国路 88 号" {
		t.Fatalf("profile contact fields = %+v, want normalized values", profile)
	}
	if profile.MaskedPhone == "" || profile.MaskedEmail == "" {
		t.Fatalf("profile masked contact fields are empty: %+v", profile)
	}
	if profile.TenantCode != "platform" || profile.OrgUnitCode != "platform-ops" || profile.AreaCode != "110000" {
		t.Fatalf("profile context = %+v, want read-only user context preserved", profile)
	}
	if profile.Credentials.PasswordChange == "" {
		t.Fatalf("profile credential status is empty: %+v", profile.Credentials)
	}
	stored, err := resources.InternalRecord("users", "user-ops")
	if err != nil {
		t.Fatalf("read updated user: %v", err)
	}
	if stored.Name != "Ops Prime" || stored.Code != "ops" || stored.Status != "enabled" {
		t.Fatalf("stored user record = %+v, want only profile-safe record fields changed", stored)
	}
	for key, want := range map[string]string{
		"avatarUrl":   "https://cdn.example.test/ops.png",
		"phone":       "+8613800138000",
		"email":       "ops@example.test",
		"address":     "北京市朝阳区建国路 88 号",
		"roles":       "operator",
		"tenantCode":  "platform",
		"orgUnitCode": "platform-ops",
		"areaCode":    "110000",
	} {
		if got := stored.Values[key]; got != want {
			t.Fatalf("stored user value %s = %q, want %q; values = %+v", key, got, want, stored.Values)
		}
	}
}

func newAdminProfileTestServer(t *testing.T) (*Server, *adminresource.Store) {
	t.Helper()
	manifests := []capability.Manifest{authProviderTestManifest(), adminProfileTestManifest()}
	resources := adminresource.NewStoreFromCapabilities(manifests)
	server := newTestServer(ServerOptions{Capabilities: manifests, Resources: resources})
	server.RegisterAdminProfileRoutes()
	return server, resources
}

func putAdminProfileForTest(server *Server, token string, path string, body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	server.Router().ServeHTTP(recorder, request)
	return recorder
}

func adminProfileTestManifest() capability.Manifest {
	return capability.Manifest{
		ID: "profile-test",
		Admin: capability.AdminSurface{Resources: []capability.AdminResource{
			{
				Resource:         "users",
				Title:            capability.Text("用户", "Users"),
				Description:      capability.Text("后台用户。", "Admin users."),
				PermissionPrefix: "admin:user",
				Fields: []capability.AdminField{
					profileTestField("avatarUrl", "头像", "Avatar URL", "text"),
					profileTestField("phone", "手机号码", "Phone", "text"),
					profileTestField("email", "邮箱", "Email", "text"),
					profileTestField("address", "地址", "Address", "textarea"),
				},
				SearchFields: []string{"avatarUrl", "phone", "email", "address"},
				Deletion:     &capability.AdminResourceDeletionPolicy{Mode: capability.AdminDeletionSoftDelete, PolicyVersion: 1},
			},
			{
				Resource:         "audit-logs",
				Title:            capability.Text("审计日志", "Audit Logs"),
				Description:      capability.Text("平台操作审计。", "Platform operation audit."),
				PermissionPrefix: "admin:audit-log",
				Deletion:         &capability.AdminResourceDeletionPolicy{Mode: capability.AdminDeletionAppendOnly, PolicyVersion: 1},
			},
			{Resource: "overview", Title: capability.Text("概览", "Overview"), Description: capability.Text("平台运行概览。", "Platform runtime overview."), PermissionPrefix: "admin:overview"},
			{Resource: "capabilities", Title: capability.Text("能力清单", "Capabilities"), Description: capability.Text("能力清单。", "Capabilities."), PermissionPrefix: "admin:capability"},
			{Resource: "tenants", Title: capability.Text("租户", "Tenants"), Description: capability.Text("租户。", "Tenants."), PermissionPrefix: "admin:tenant"},
			{Resource: "monitoring", Title: capability.Text("监控", "Monitoring"), Description: capability.Text("监控。", "Monitoring."), PermissionPrefix: "admin:monitoring"},
		}},
	}
}

func profileTestField(key string, labelZH string, labelEN string, fieldType string) capability.AdminField {
	return capability.AdminField{
		Key:          key,
		Label:        capability.Text(labelZH, labelEN),
		Type:         fieldType,
		Source:       "values",
		InForm:       true,
		InDetail:     true,
		Sensitivity:  capability.FieldSensitivityPublic,
		StorageMode:  capability.FieldStoragePlain,
		ResponseMode: capability.FieldProjectionFull,
		ExportMode:   capability.FieldProjectionFull,
	}
}
