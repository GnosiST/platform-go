package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/GnosiST/platform-go/internal/platform/adminresource"
	"github.com/GnosiST/platform-go/internal/platform/capability"
	"github.com/GnosiST/platform-go/internal/platform/credentialauth"
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
	if profile.Credentials.PasswordChange != "credential-auth-not-connected" {
		t.Fatalf("profile credential status = %+v, want credential auth disconnected without runtime", profile.Credentials)
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

func TestAdminProfilePasswordChangeUpdatesCredentialAuthPassword(t *testing.T) {
	server, _, runtime := newAdminProfileCredentialTestServer(t)
	login := loginForTest(t, server, "ops")
	body := adminProfilePasswordChangeBodyForTest(t, runtime, "current-password", "next-password")

	recorder := postAdminProfileForTest(server, login.Data.Token, "/api/admin/profile/current/password/change", body)

	if recorder.Code != http.StatusOK {
		t.Fatalf("POST password change status = %d body = %s, want 200", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Data struct {
			Credentials struct {
				PasswordChange string `json:"passwordChange"`
				PasswordReset  string `json:"passwordReset"`
			} `json:"credentials"`
			MustChange bool `json:"mustChange"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode password change response: %v", err)
	}
	if payload.Data.Credentials.PasswordChange != "credential-auth-ready" || payload.Data.Credentials.PasswordReset != "credential-auth-ready" || payload.Data.MustChange {
		t.Fatalf("password change response = %+v, want ready credential status and mustChange=false", payload.Data)
	}
	if _, err := runtime.Service.VerifyPassword(context.Background(), credentialauth.PasswordLoginInput{
		Identifier: credentialauth.Identifier{Type: credentialauth.IdentifierTypeUsername, Value: "ops"},
		Secret:     "next-password",
	}); err != nil {
		t.Fatalf("VerifyPassword(next-password) error = %v", err)
	}
}

func TestAdminProfilePasswordChangeRebindsBootstrapUsernamePrincipal(t *testing.T) {
	server, _, runtime := newAdminProfileCredentialTestServer(t)
	registerProfilePasswordCredentialForTest(t, runtime.Service, credentialauth.PrincipalRef{Type: credentialauth.PrincipalTypeAdmin, ID: "admin"}, "admin", "bootstrap-password", runtime.PasswordHashParams)
	login := loginForTest(t, server, "admin")
	body := adminProfilePasswordChangeBodyForTest(t, runtime, "bootstrap-password", "next-admin-password")

	recorder := postAdminProfileForTest(server, login.Data.Token, "/api/admin/profile/current/password/change", body)

	if recorder.Code != http.StatusOK {
		t.Fatalf("POST password change status = %d body = %s, want 200", recorder.Code, recorder.Body.String())
	}
	if _, err := runtime.Service.VerifyPassword(context.Background(), credentialauth.PasswordLoginInput{
		Identifier: credentialauth.Identifier{Type: credentialauth.IdentifierTypeUsername, Value: "admin"},
		Secret:     "next-admin-password",
	}); err != nil {
		t.Fatalf("VerifyPassword(next-admin-password) error = %v", err)
	}
	if _, err := runtime.Service.VerifyPassword(context.Background(), credentialauth.PasswordLoginInput{
		Identifier: credentialauth.Identifier{Type: credentialauth.IdentifierTypeUsername, Value: "admin"},
		Secret:     "bootstrap-password",
	}); err == nil {
		t.Fatalf("VerifyPassword(bootstrap-password) error = nil, want old bootstrap credential rejected")
	}
}

func TestAdminProfilePasswordChangeSupportsSubsequentCredentialLogin(t *testing.T) {
	server, runtime := newAdminProfileCredentialPlatformTestServer(t)
	login := loginForTest(t, server, "admin")
	body := adminProfilePasswordChangeBodyForTest(t, runtime, "admin-password", "next-admin-password")

	recorder := postAdminProfileForTest(server, login.Data.Token, "/api/admin/profile/current/password/change", body)

	if recorder.Code != http.StatusOK {
		t.Fatalf("POST password change status = %d body = %s, want 200", recorder.Code, recorder.Body.String())
	}
	newLoginRecorder := httptest.NewRecorder()
	newLoginRequest := newCredentialAuthHTTPTestRequest(http.MethodPost, "/api/auth/login", credentialAuthPasswordLoginBodyForTest(t, runtime, "next-admin-password"))
	newLoginRequest.Header.Set("Content-Type", "application/json")

	server.Router().ServeHTTP(newLoginRecorder, newLoginRequest)

	if newLoginRecorder.Code != http.StatusOK {
		t.Fatalf("POST credential login after password change status = %d body = %s, want 200", newLoginRecorder.Code, newLoginRecorder.Body.String())
	}
	oldLoginRecorder := httptest.NewRecorder()
	oldLoginRequest := newCredentialAuthHTTPTestRequest(http.MethodPost, "/api/auth/login", credentialAuthPasswordLoginBodyForTest(t, runtime, "admin-password"))
	oldLoginRequest.Header.Set("Content-Type", "application/json")

	server.Router().ServeHTTP(oldLoginRecorder, oldLoginRequest)

	if oldLoginRecorder.Code != http.StatusUnauthorized || !strings.Contains(oldLoginRecorder.Body.String(), "AUTH_INVALID_CREDENTIALS") {
		t.Fatalf("POST credential login with old password status = %d body = %s, want invalid credentials", oldLoginRecorder.Code, oldLoginRecorder.Body.String())
	}
}

func TestAdminProfilePasswordChangeRejectsWrongCurrentPasswordAndAuditsFailure(t *testing.T) {
	server, resources, runtime := newAdminProfileCredentialTestServer(t)
	login := loginForTest(t, server, "ops")
	body := adminProfilePasswordChangeBodyForTest(t, runtime, "wrong-password", "next-password")

	recorder := postAdminProfileForTest(server, login.Data.Token, "/api/admin/profile/current/password/change", body)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("POST password change with wrong current password status = %d body = %s, want 401", recorder.Code, recorder.Body.String())
	}
	if _, err := runtime.Service.VerifyPassword(context.Background(), credentialauth.PasswordLoginInput{
		Identifier: credentialauth.Identifier{Type: credentialauth.IdentifierTypeUsername, Value: "ops"},
		Secret:     "current-password",
	}); err != nil {
		t.Fatalf("VerifyPassword(current-password) error = %v", err)
	}
	if !adminProfileAuditExists(t, resources, "admin_profile.password.change", "failure", "current-password-rejected") {
		t.Fatalf("password change failure audit missing")
	}
}

func TestAdminProfilePasswordResetUpdatesTargetCredentialAndRequiresChange(t *testing.T) {
	server, _, runtime := newAdminProfileCredentialTestServer(t)
	server.authorizer = permissionSetAuthorizer{"admin:user:update": true}
	login := loginForTest(t, server, "ops")
	body := adminProfilePasswordResetBodyForTest(t, runtime, "reset-password")

	recorder := postAdminProfileForTest(server, login.Data.Token, "/api/admin/profile/user-admin/password/reset", body)

	if recorder.Code != http.StatusOK {
		t.Fatalf("POST password reset status = %d body = %s, want 200", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Data struct {
			Credentials struct {
				PasswordChange string `json:"passwordChange"`
				PasswordReset  string `json:"passwordReset"`
			} `json:"credentials"`
			MustChange bool `json:"mustChange"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode password reset response: %v", err)
	}
	if payload.Data.Credentials.PasswordChange != "credential-auth-ready" || payload.Data.Credentials.PasswordReset != "credential-auth-ready" || !payload.Data.MustChange {
		t.Fatalf("password reset response = %+v, want ready credential status and mustChange=true", payload.Data)
	}
	result, err := runtime.Service.VerifyPassword(context.Background(), credentialauth.PasswordLoginInput{
		Identifier: credentialauth.Identifier{Type: credentialauth.IdentifierTypeUsername, Value: "admin"},
		Secret:     "reset-password",
	})
	if err != nil {
		t.Fatalf("VerifyPassword(reset-password) error = %v", err)
	}
	if !result.MustChange {
		t.Fatalf("VerifyPassword(reset-password).MustChange = false, want true")
	}
}

func TestAdminProfilePasswordResetRebindsBootstrapUsernamePrincipal(t *testing.T) {
	server, _, runtime := newAdminProfileCredentialTestServer(t)
	server.authorizer = permissionSetAuthorizer{"admin:user:update": true}
	registerProfilePasswordCredentialForTest(t, runtime.Service, credentialauth.PrincipalRef{Type: credentialauth.PrincipalTypeAdmin, ID: "admin"}, "admin", "bootstrap-password", runtime.PasswordHashParams)
	login := loginForTest(t, server, "ops")
	body := adminProfilePasswordResetBodyForTest(t, runtime, "reset-admin-password")

	recorder := postAdminProfileForTest(server, login.Data.Token, "/api/admin/profile/user-admin/password/reset", body)

	if recorder.Code != http.StatusOK {
		t.Fatalf("POST password reset status = %d body = %s, want 200", recorder.Code, recorder.Body.String())
	}
	result, err := runtime.Service.VerifyPassword(context.Background(), credentialauth.PasswordLoginInput{
		Identifier: credentialauth.Identifier{Type: credentialauth.IdentifierTypeUsername, Value: "admin"},
		Secret:     "reset-admin-password",
	})
	if err != nil {
		t.Fatalf("VerifyPassword(reset-admin-password) error = %v", err)
	}
	if !result.MustChange {
		t.Fatalf("VerifyPassword(reset-admin-password).MustChange = false, want true")
	}
	if _, err := runtime.Service.VerifyPassword(context.Background(), credentialauth.PasswordLoginInput{
		Identifier: credentialauth.Identifier{Type: credentialauth.IdentifierTypeUsername, Value: "admin"},
		Secret:     "bootstrap-password",
	}); err == nil {
		t.Fatalf("VerifyPassword(bootstrap-password) error = nil, want old bootstrap credential rejected")
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

func newAdminProfileCredentialTestServer(t *testing.T) (*Server, *adminresource.Store, *CredentialAuthRuntime) {
	t.Helper()
	manifests := []capability.Manifest{authProviderTestManifest(), adminProfileTestManifest()}
	resources := adminresource.NewStoreFromCapabilities(manifests)
	runtime := credentialAuthRuntimeForProfileTest(t)
	server := newTestServer(ServerOptions{Capabilities: manifests, Resources: resources, CredentialAuth: runtime})
	server.RegisterAdminProfileRoutes()
	return server, resources, runtime
}

func newAdminProfileCredentialPlatformTestServer(t *testing.T) (*Server, *CredentialAuthRuntime) {
	t.Helper()
	manifests := configuredCredentialAuthManifestsForTest(t)
	resources := adminresource.NewStoreFromCapabilities(manifests)
	runtime := credentialAuthRuntimeForProfileTest(t)
	server := newTestServer(ServerOptions{Capabilities: manifests, Resources: resources, CredentialAuth: runtime})
	server.RegisterAdminProfileRoutes()
	return server, runtime
}

func credentialAuthRuntimeForProfileTest(t *testing.T) *CredentialAuthRuntime {
	t.Helper()
	hasher, err := credentialauth.NewHMACIdentifierHasher([]byte(strings.Repeat("k", 32)))
	if err != nil {
		t.Fatalf("NewHMACIdentifierHasher() error = %v", err)
	}
	params := credentialauth.Argon2idParams{MemoryKiB: 1024, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32}
	service, err := credentialauth.NewService(credentialauth.Options{
		Repository:           credentialauth.NewMemoryRepository(),
		IdentifierHasher:     hasher,
		PasswordVerifier:     credentialauth.NewArgon2idVerifier(params),
		ChallengeProofHasher: hasher,
		Now:                  func() time.Time { return time.Date(2026, 7, 19, 9, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	registerProfilePasswordCredentialForTest(t, service, credentialauth.PrincipalRef{Type: credentialauth.PrincipalTypeAdmin, ID: "user-ops"}, "ops", "current-password", params)
	registerProfilePasswordCredentialForTest(t, service, credentialauth.PrincipalRef{Type: credentialauth.PrincipalTypeAdmin, ID: "user-admin"}, "admin", "admin-password", params)
	secretTransport, err := credentialauth.NewSecretTransport(credentialauth.SecretTransportOptions{
		KeyID: "test-profile-auth-transport-v1",
		Now:   func() time.Time { return time.Date(2026, 7, 19, 9, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("NewSecretTransport() error = %v", err)
	}
	return &CredentialAuthRuntime{
		Service:                 service,
		IdentifierHasher:        hasher,
		SecretTransport:         secretTransport,
		ChallengeProofHasher:    hasher,
		Now:                     func() time.Time { return time.Date(2026, 7, 19, 9, 0, 0, 0, time.UTC) },
		PasswordHashParams:      params,
		PasswordParamsVersion:   "test-v1",
		RequireEncryptedSecrets: true,
	}
}

func registerProfilePasswordCredentialForTest(t *testing.T, service *credentialauth.Service, principal credentialauth.PrincipalRef, username string, password string, params credentialauth.Argon2idParams) {
	t.Helper()
	if _, err := service.RegisterIdentifier(context.Background(), credentialauth.RegisterIdentifierInput{
		Principal:  principal,
		Identifier: credentialauth.Identifier{Type: credentialauth.IdentifierTypeUsername, Value: username},
		Status:     credentialauth.StatusEnabled,
	}); err != nil {
		t.Fatalf("RegisterIdentifier(%s) error = %v", username, err)
	}
	hash, err := credentialauth.HashPasswordArgon2id(password, params)
	if err != nil {
		t.Fatalf("HashPasswordArgon2id(%s) error = %v", username, err)
	}
	if err := service.PutPasswordCredential(context.Background(), credentialauth.PasswordCredential{
		Principal:     principal,
		PasswordHash:  hash,
		Algorithm:     credentialauth.PasswordAlgorithmArgon2id,
		ParamsVersion: "test-v1",
		Status:        credentialauth.StatusEnabled,
	}); err != nil {
		t.Fatalf("PutPasswordCredential(%s) error = %v", username, err)
	}
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

func postAdminProfileForTest(server *Server, token string, path string, body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	request.RemoteAddr = "127.0.0.1:43110"
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	server.Router().ServeHTTP(recorder, request)
	return recorder
}

func adminProfilePasswordChangeBodyForTest(t *testing.T, runtime *CredentialAuthRuntime, currentPassword string, newPassword string) string {
	t.Helper()
	currentSecret := credentialAuthEncryptedSecretForTest(t, runtime, adminProfilePasswordChangeProvider, adminProfilePasswordCurrentSecretType, adminProfilePasswordSecretIdentifierType, currentPassword)
	newSecret := credentialAuthEncryptedSecretForTest(t, runtime, adminProfilePasswordChangeProvider, adminProfilePasswordNewSecretType, adminProfilePasswordSecretIdentifierType, newPassword)
	body, err := json.Marshal(map[string]any{
		"currentSecret": map[string]any{"type": adminProfilePasswordCurrentSecretType, "encrypted": currentSecret},
		"newSecret":     map[string]any{"type": adminProfilePasswordNewSecretType, "encrypted": newSecret},
	})
	if err != nil {
		t.Fatalf("marshal password change body: %v", err)
	}
	return string(body)
}

func adminProfilePasswordResetBodyForTest(t *testing.T, runtime *CredentialAuthRuntime, newPassword string) string {
	t.Helper()
	newSecret := credentialAuthEncryptedSecretForTest(t, runtime, adminProfilePasswordResetProvider, adminProfilePasswordNewSecretType, adminProfilePasswordSecretIdentifierType, newPassword)
	body, err := json.Marshal(map[string]any{
		"newSecret": map[string]any{"type": adminProfilePasswordNewSecretType, "encrypted": newSecret},
	})
	if err != nil {
		t.Fatalf("marshal password reset body: %v", err)
	}
	return string(body)
}

func adminProfileAuditExists(t *testing.T, resources *adminresource.Store, action string, result string, reason string) bool {
	t.Helper()
	records, err := resources.List("audit-logs")
	if err != nil {
		t.Fatalf("List(audit-logs) error = %v", err)
	}
	for _, record := range records {
		if record.Values["action"] == action && record.Values["outcome"] == result && record.Values["reasonCode"] == reason {
			return true
		}
	}
	return false
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
