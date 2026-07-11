package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"platform-go/internal/apps"
	"platform-go/internal/platform/adminresource"
	"platform-go/internal/platform/authjwt"
	"platform-go/internal/platform/cache"
	"platform-go/internal/platform/capability"
	"platform-go/internal/platform/core"
	"platform-go/internal/platform/session"
	"platform-go/internal/platform/storage"
)

type adminResourceListTestPayload struct {
	Data struct {
		Resource string                    `json:"resource"`
		Items    []adminResourceRecordTest `json:"items"`
	} `json:"data"`
}

type adminResourceQueryTestPayload struct {
	Data struct {
		Resource string                    `json:"resource"`
		Items    []adminResourceRecordTest `json:"items"`
		Total    int                       `json:"total"`
		Page     int                       `json:"page"`
		PageSize int                       `json:"pageSize"`
	} `json:"data"`
}

type adminResourceRecordTestPayload struct {
	Data struct {
		Record adminResourceRecordTest `json:"record"`
		Token  string                  `json:"token"`
	} `json:"data"`
}

type policyReviewApproveTestPayload struct {
	Data struct {
		Review adminResourceRecordTest `json:"review"`
		Role   adminResourceRecordTest `json:"role"`
		Audit  adminResourceRecordTest `json:"audit"`
	} `json:"data"`
}

type policyReviewActionTestPayload struct {
	Data struct {
		Review adminResourceRecordTest `json:"review"`
		Audit  adminResourceRecordTest `json:"audit"`
	} `json:"data"`
}

type policyReviewExportTestPayload struct {
	Data struct {
		ExportedBy string                    `json:"exportedBy"`
		ExportedAt string                    `json:"exportedAt"`
		Reviews    []adminResourceRecordTest `json:"reviews"`
		Audits     []adminResourceRecordTest `json:"audits"`
	} `json:"data"`
}

type adminResourceSchemaTestPayload struct {
	Data struct {
		Resource string `json:"resource"`
		Title    struct {
			ZH string `json:"zh"`
			EN string `json:"en"`
		} `json:"title"`
		Permissions struct {
			Read   string `json:"read"`
			Create string `json:"create"`
			Update string `json:"update"`
			Delete string `json:"delete"`
		} `json:"permissions"`
		FormLayout string `json:"formLayout"`
		Fields     []struct {
			Key        string `json:"key"`
			Type       string `json:"type"`
			Source     string `json:"source"`
			Required   bool   `json:"required"`
			Searchable bool   `json:"searchable"`
			InTable    bool   `json:"inTable"`
			InForm     bool   `json:"inForm"`
			InDetail   bool   `json:"inDetail"`
			Options    []struct {
				Value string `json:"value"`
			} `json:"options"`
		} `json:"fields"`
		SearchFields []string `json:"searchFields"`
	} `json:"data"`
}

type currentSessionTestPayload struct {
	Data struct {
		User struct {
			Username    string `json:"username"`
			TenantCode  string `json:"tenantCode"`
			OrgUnitCode string `json:"orgUnitCode"`
			AreaCode    string `json:"areaCode"`
		} `json:"user"`
		Roles             []string `json:"roles"`
		Permissions       []string `json:"permissions"`
		DeniedPermissions []string `json:"deniedPermissions"`
	} `json:"data"`
}

type authProviderListTestPayload struct {
	Data struct {
		Items []struct {
			ID         string `json:"id"`
			Kind       string `json:"kind"`
			Configured bool   `json:"configured"`
			Enabled    bool   `json:"enabled"`
			Title      struct {
				ZH string `json:"zh"`
				EN string `json:"en"`
			} `json:"title"`
		} `json:"items"`
	} `json:"data"`
}

type authLoginTestPayload struct {
	Data struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expiresAt"`
		Principal struct {
			User struct {
				Username string `json:"username"`
			} `json:"user"`
			Roles       []string `json:"roles"`
			Permissions []string `json:"permissions"`
		} `json:"principal"`
	} `json:"data"`
}

type adminIdentityStartTestPayload struct {
	Data struct {
		AuthorizationURL string    `json:"authorizationUrl"`
		State            string    `json:"state"`
		ExpiresAt        time.Time `json:"expiresAt"`
	} `json:"data"`
}

type appLoginTestPayload struct {
	Data struct {
		Token     string             `json:"token"`
		ExpiresAt time.Time          `json:"expiresAt"`
		Session   appSessionTestData `json:"session"`
	} `json:"data"`
}

type appSessionTestPayload struct {
	Data appSessionTestData `json:"data"`
}

type appSessionTestData struct {
	UserID    string `json:"userId"`
	Username  string `json:"username"`
	TenantID  string `json:"tenantId"`
	SessionID string `json:"sessionId"`
}

type appPhoneVerificationTestPayload struct {
	Data struct {
		ID          string    `json:"id"`
		MaskedPhone string    `json:"maskedPhone"`
		PhoneHash   string    `json:"phoneHash"`
		Purpose     string    `json:"purpose"`
		ExpiresAt   time.Time `json:"expiresAt"`
		DebugCode   string    `json:"debugCode"`
	} `json:"data"`
}

type appPhoneBindingTestPayload struct {
	Data struct {
		ID          string    `json:"id"`
		AppUsername string    `json:"appUsername"`
		MaskedPhone string    `json:"maskedPhone"`
		PhoneHash   string    `json:"phoneHash"`
		BoundAt     time.Time `json:"boundAt"`
	} `json:"data"`
}

type appIdentityResolverFunc func(context.Context, AppIdentityResolveInput) (AppIdentity, error)

func (f appIdentityResolverFunc) ResolveAppIdentity(ctx context.Context, input AppIdentityResolveInput) (AppIdentity, error) {
	return f(ctx, input)
}

type adminIdentityResolverFunc struct {
	start   func(context.Context, AdminIdentityStartInput) (AdminIdentityStart, error)
	resolve func(context.Context, AdminIdentityResolveInput) (AdminIdentity, error)
}

type phoneVerificationSenderTestStub struct {
	kind string
	send func(context.Context, string, string, string) error
}

type mutablePhoneVerificationSenderTestStub struct {
	kind string
}

type phoneProtectorTestStub struct {
	phoneDigest func(string) (string, error)
	codeDigest  func(string, string, string) (string, error)
}

func (p phoneProtectorTestStub) PhoneDigest(phone string) (string, error) {
	return p.phoneDigest(phone)
}

func (p phoneProtectorTestStub) CodeDigest(phoneDigest string, purpose string, code string) (string, error) {
	return p.codeDigest(phoneDigest, purpose, code)
}

func (p phoneProtectorTestStub) KeyIDs() (PhoneProtectionKeyIDs, error) {
	return PhoneProtectionKeyIDs{Phone: strings.Repeat("a", 64), Code: strings.Repeat("b", 64)}, nil
}

func (s phoneVerificationSenderTestStub) Send(ctx context.Context, phone string, purpose string, code string) error {
	if s.send == nil {
		return nil
	}
	return s.send(ctx, phone, purpose, code)
}

func (s phoneVerificationSenderTestStub) Kind() string {
	return s.kind
}

func (s *mutablePhoneVerificationSenderTestStub) Send(context.Context, string, string, string) error {
	s.kind = PhoneVerificationProviderDebug
	return nil
}

func (s *mutablePhoneVerificationSenderTestStub) Kind() string {
	return s.kind
}

func (f adminIdentityResolverFunc) StartAdminIdentity(ctx context.Context, input AdminIdentityStartInput) (AdminIdentityStart, error) {
	return f.start(ctx, input)
}

func (f adminIdentityResolverFunc) ResolveAdminIdentity(ctx context.Context, input AdminIdentityResolveInput) (AdminIdentity, error) {
	return f.resolve(ctx, input)
}

type adminIdentityBindingStoreFunc struct {
	resolve func(context.Context, AdminIdentityBindingInput) (AdminIdentityBinding, error)
}

func (f adminIdentityBindingStoreFunc) ResolveAdminIdentityBinding(ctx context.Context, input AdminIdentityBindingInput) (AdminIdentityBinding, error) {
	return f.resolve(ctx, input)
}

func (adminIdentityBindingStoreFunc) ProvisionAdminIdentityBinding(context.Context, AdminIdentityProvisionInput) (AdminIdentityBinding, error) {
	return AdminIdentityBinding{}, ErrAdminIdentityBindingInvalid
}

func (adminIdentityBindingStoreFunc) ValidateAdminIdentityBindingReadiness(context.Context, capability.AuthProvider) error {
	return nil
}

func newTestServer(options ServerOptions) *Server {
	options.AllowInsecureHeaderAuth = true
	if options.PhoneProtector == nil {
		options.PhoneProtector = NewHMACPhoneProtector([]byte(strings.Repeat("p", 32)), []byte(strings.Repeat("c", 32)))
	}
	if options.PhoneVerificationSender == nil {
		options.PhoneVerificationSender = NewDebugPhoneVerificationSender()
		options.DebugCodeEnabled = true
	}
	return New(options)
}

type adminMenuListTestPayload struct {
	Data struct {
		Items []struct {
			Name         string `json:"name"`
			Route        string `json:"route"`
			Parent       string `json:"parent"`
			IsExternal   bool   `json:"isExternal"`
			CacheEnabled bool   `json:"cacheEnabled"`
			Permission   string `json:"permission"`
			Group        string `json:"group"`
		} `json:"items"`
	} `json:"data"`
}

type brandingConfigTestPayload struct {
	Data struct {
		ProductName   string `json:"productName"`
		ShortName     string `json:"shortName"`
		LogoURL       string `json:"logoUrl"`
		FaviconURL    string `json:"faviconUrl"`
		PrimaryColor  string `json:"primaryColor"`
		DefaultTheme  string `json:"defaultTheme"`
		LoginTitle    string `json:"loginTitle"`
		LoginSubtitle string `json:"loginSubtitle"`
		SupportEmail  string `json:"supportEmail"`
	} `json:"data"`
}

type demoDataListTestPayload struct {
	Data struct {
		Items []struct {
			ID           string `json:"id"`
			CapabilityID string `json:"capabilityId"`
			Resource     string `json:"resource"`
			Title        struct {
				ZH string `json:"zh"`
				EN string `json:"en"`
			} `json:"title"`
			Records int `json:"records"`
		} `json:"items"`
	} `json:"data"`
}

type demoDataApplyTestPayload struct {
	Data struct {
		ID           string `json:"id"`
		CapabilityID string `json:"capabilityId"`
		Resource     string `json:"resource"`
		Applied      int    `json:"applied"`
	} `json:"data"`
}

type cacheStatsTestPayload struct {
	Data struct {
		Driver         string `json:"driver"`
		Hits           uint64 `json:"hits"`
		Misses         uint64 `json:"misses"`
		Sets           uint64 `json:"sets"`
		Deletes        uint64 `json:"deletes"`
		DeletePrefixes uint64 `json:"deletePrefixes"`
		Errors         uint64 `json:"errors"`
	} `json:"data"`
}

type adminResourceRecordTest struct {
	ID          string            `json:"id"`
	Code        string            `json:"code"`
	Name        string            `json:"name"`
	Status      string            `json:"status"`
	Description string            `json:"description"`
	Values      map[string]string `json:"values"`
}

type denyAllAuthorizer struct{}

func (denyAllAuthorizer) Can(user string, tenant string, permission string, action string) bool {
	return false
}

type allowAppPermissionAuthorizer struct {
	user       string
	tenant     string
	permission string
	action     string
}

func (a allowAppPermissionAuthorizer) Can(user string, tenant string, permission string, action string) bool {
	return user == a.user && tenant == a.tenant && permission == a.permission && action == a.action
}

func TestHealthEndpoint(t *testing.T) {
	server := newTestServer(ServerOptions{Capabilities: []capability.Manifest{{ID: "tenant"}}})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /api/health status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"ok":true`) {
		t.Fatalf("GET /api/health body = %s", recorder.Body.String())
	}
}

func TestCapabilitiesEndpoint(t *testing.T) {
	server := newTestServer(ServerOptions{Capabilities: []capability.Manifest{{ID: "tenant"}, {ID: "identity"}}})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/capabilities", nil)

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /api/capabilities status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"id":"tenant"`) || !strings.Contains(body, `"id":"identity"`) {
		t.Fatalf("GET /api/capabilities body = %s", body)
	}
}

func TestAdminEndpointsRequireBearerByDefault(t *testing.T) {
	server := New(ServerOptions{Capabilities: []capability.Manifest{{ID: "tenant"}, {ID: "identity"}}})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/admin/resources/tenants/query", bytes.NewBufferString(`{"page":1,"pageSize":10}`))
	request.Header.Set("Content-Type", "application/json")

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("POST tenant query without bearer status = %d body = %s, want 401", recorder.Code, recorder.Body.String())
	}
}

func TestOpenAPIEndpointReturnsConfiguredDocument(t *testing.T) {
	document := []byte(`{"openapi":"3.1.0","info":{"title":"Platform Admin API","version":"0.1.0"},"paths":{}}`)
	server := newTestServer(ServerOptions{
		Capabilities:    []capability.Manifest{{ID: "api-docs"}},
		OpenAPIDocument: document,
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/openapi.json", nil)

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /api/openapi.json status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	if contentType := recorder.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", contentType)
	}
	if recorder.Body.String() != string(document) {
		t.Fatalf("openapi body = %s, want %s", recorder.Body.String(), string(document))
	}
}

func TestOpenAPIEndpointReportsMissingDocument(t *testing.T) {
	server := newTestServer(ServerOptions{Capabilities: []capability.Manifest{{ID: "api-docs"}}})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/openapi.json", nil)

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("GET /api/openapi.json status = %d body = %s, want 404", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "OPENAPI_NOT_CONFIGURED") {
		t.Fatalf("missing openapi body = %s", recorder.Body.String())
	}
}

func TestBrandingEndpointReturnsDefaultBrandingConfig(t *testing.T) {
	server := newTestServer(ServerOptions{Capabilities: []capability.Manifest{{ID: "parameter"}}})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/platform/branding", nil)

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("GET branding status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	var payload brandingConfigTestPayload
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode branding: %v body = %s", err, recorder.Body.String())
	}
	if payload.Data.ProductName != "Platform Go" {
		t.Fatalf("ProductName = %q, want Platform Go", payload.Data.ProductName)
	}
	if payload.Data.DefaultTheme != "tech" || payload.Data.PrimaryColor == "" {
		t.Fatalf("branding theme/color mismatch: %+v", payload.Data)
	}
}

func TestBrandingEndpointReflectsAdminSettingsUpdate(t *testing.T) {
	cacheStore := cache.NewMemoryStore(cache.MemoryStoreOptions{})
	server := newTestServer(ServerOptions{
		Capabilities: []capability.Manifest{{ID: "parameter"}},
		Cache:        cacheStore,
	})
	primeRecorder := httptest.NewRecorder()
	primeRequest := httptest.NewRequest(http.MethodGet, "/api/platform/branding", nil)
	server.Router().ServeHTTP(primeRecorder, primeRequest)
	if primeRecorder.Code != http.StatusOK {
		t.Fatalf("prime branding status = %d body = %s", primeRecorder.Code, primeRecorder.Body.String())
	}
	if _, ok, err := cacheStore.Get(primeRequest.Context(), "admin:branding"); err != nil || !ok {
		t.Fatalf("branding cache after prime ok = %v err = %v, want cached config", ok, err)
	}

	updateBody := bytes.NewBufferString(`{"name":"Branding Settings","status":"enabled","description":"Updated branding","values":{"capability":"branding","productName":"Acme Ops","shortName":"Acme","logoUrl":"https://cdn.example.test/logo.png","faviconUrl":"https://cdn.example.test/favicon.ico","primaryColor":"#1677ff","defaultTheme":"white","loginTitle":"Welcome to Acme","loginSubtitle":"Operate with confidence"}}`)
	updateRecorder := httptest.NewRecorder()
	updateRequest := httptest.NewRequest(http.MethodPut, "/api/admin/resources/settings/setting-branding", updateBody)
	updateRequest.Header.Set("Content-Type", "application/json")

	server.Router().ServeHTTP(updateRecorder, updateRequest)

	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("PUT branding settings status = %d body = %s", updateRecorder.Code, updateRecorder.Body.String())
	}
	if _, ok, err := cacheStore.Get(updateRequest.Context(), "admin:branding"); err != nil || ok {
		t.Fatalf("branding cache after settings update ok = %v err = %v, want invalidated cache", ok, err)
	}

	brandRecorder := httptest.NewRecorder()
	brandRequest := httptest.NewRequest(http.MethodGet, "/api/platform/branding", nil)
	server.Router().ServeHTTP(brandRecorder, brandRequest)

	if brandRecorder.Code != http.StatusOK {
		t.Fatalf("GET branding status = %d body = %s", brandRecorder.Code, brandRecorder.Body.String())
	}
	if _, ok, err := cacheStore.Get(brandRequest.Context(), "admin:branding"); err != nil || !ok {
		t.Fatalf("branding cache after reload ok = %v err = %v, want refreshed cache", ok, err)
	}
	var payload brandingConfigTestPayload
	if err := json.Unmarshal(brandRecorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode branding: %v body = %s", err, brandRecorder.Body.String())
	}
	if payload.Data.ProductName != "Acme Ops" || payload.Data.ShortName != "Acme" || payload.Data.DefaultTheme != "white" {
		t.Fatalf("branding after update mismatch: %+v", payload.Data)
	}
	if payload.Data.SupportEmail != "" {
		t.Fatalf("SupportEmail = %q, want disabled until encrypted storage is available", payload.Data.SupportEmail)
	}
}

func TestAdminDemoDataEndpointListsCapabilityDeclaredDatasets(t *testing.T) {
	server := newTestServer(ServerOptions{Capabilities: []capability.Manifest{demoDataTestManifest()}})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/admin/demo-data", nil)

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("GET demo data status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	var payload demoDataListTestPayload
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode demo data: %v body = %s", err, recorder.Body.String())
	}
	if len(payload.Data.Items) != 1 {
		t.Fatalf("demo data item count = %d, want 1", len(payload.Data.Items))
	}
	item := payload.Data.Items[0]
	if item.ID != "demo-tenants" || item.CapabilityID != "demo-seed" || item.Resource != "tenants" || item.Records != 1 {
		t.Fatalf("demo data item mismatch: %+v", item)
	}
	if !strings.Contains(recorder.Body.String(), `"title":{"zh":"演示租户","en":"Demo Tenants"}`) {
		t.Fatalf("demo data wire title casing mismatch: %s", recorder.Body.String())
	}
	if item.Title.ZH != "演示租户" || item.Title.EN != "Demo Tenants" {
		t.Fatalf("demo data localized title = %+v, want zh/en title", item.Title)
	}
}

func TestAdminDemoDataApplyWritesDeclaredRecords(t *testing.T) {
	server := newTestServer(ServerOptions{Capabilities: []capability.Manifest{demoDataTestManifest()}})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/admin/demo-data/demo-seed/demo-tenants/apply", nil)

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("POST demo data apply status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	var applied demoDataApplyTestPayload
	if err := json.Unmarshal(recorder.Body.Bytes(), &applied); err != nil {
		t.Fatalf("decode demo apply: %v body = %s", err, recorder.Body.String())
	}
	if applied.Data.Applied != 1 || applied.Data.Resource != "tenants" {
		t.Fatalf("demo apply payload mismatch: %+v", applied.Data)
	}

	secondApplyRecorder := httptest.NewRecorder()
	secondApplyRequest := httptest.NewRequest(http.MethodPost, "/api/admin/demo-data/demo-seed/demo-tenants/apply", nil)
	server.Router().ServeHTTP(secondApplyRecorder, secondApplyRequest)

	if secondApplyRecorder.Code != http.StatusOK {
		t.Fatalf("POST demo data second apply status = %d body = %s", secondApplyRecorder.Code, secondApplyRecorder.Body.String())
	}
	var secondApplied demoDataApplyTestPayload
	if err := json.Unmarshal(secondApplyRecorder.Body.Bytes(), &secondApplied); err != nil {
		t.Fatalf("decode second demo apply: %v body = %s", err, secondApplyRecorder.Body.String())
	}
	if secondApplied.Data.Applied != 1 || secondApplied.Data.Resource != "tenants" {
		t.Fatalf("second demo apply payload mismatch: %+v", secondApplied.Data)
	}

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/api/admin/resources/tenants", nil)
	server.Router().ServeHTTP(listRecorder, listRequest)

	if listRecorder.Code != http.StatusOK {
		t.Fatalf("GET tenants status = %d body = %s", listRecorder.Code, listRecorder.Body.String())
	}
	var tenants adminResourceListTestPayload
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &tenants); err != nil {
		t.Fatalf("decode tenants: %v body = %s", err, listRecorder.Body.String())
	}
	if !hasTestRecordID(tenants.Data.Items, "tenant-demo-acme") {
		t.Fatalf("tenants missing applied demo record: %+v", tenants.Data.Items)
	}
	if countTestRecordID(tenants.Data.Items, "tenant-demo-acme") != 1 {
		t.Fatalf("tenants duplicated applied demo record: %+v", tenants.Data.Items)
	}
}

func TestAdminDemoDataApplyRequiresPermission(t *testing.T) {
	server := newTestServer(ServerOptions{Capabilities: []capability.Manifest{demoDataTestManifest()}})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/admin/demo-data/demo-seed/demo-tenants/apply", nil)
	request.Header.Set("X-Platform-User", "ops")

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("POST demo data apply as ops status = %d body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestAuthProvidersEndpointReturnsCapabilityDeclaredProviders(t *testing.T) {
	server := newTestServer(ServerOptions{Capabilities: []capability.Manifest{adminAudienceAuthProviderTestManifest()}})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/auth/providers", nil)

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("GET auth providers status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	var payload authProviderListTestPayload
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode auth providers: %v body = %s", err, recorder.Body.String())
	}
	if len(payload.Data.Items) != 2 {
		t.Fatalf("auth providers = %+v, want demo and oidc", payload.Data.Items)
	}
	if payload.Data.Items[0].ID != "demo" || !payload.Data.Items[0].Configured || !payload.Data.Items[0].Enabled {
		t.Fatalf("first auth provider = %+v, want configured demo provider", payload.Data.Items[0])
	}
	if !strings.Contains(recorder.Body.String(), `"title":{"zh":"演示登录","en":"Demo Login"}`) {
		t.Fatalf("auth provider wire title casing mismatch: %s", recorder.Body.String())
	}
	if payload.Data.Items[0].Title.ZH != "演示登录" || payload.Data.Items[0].Title.EN != "Demo Login" {
		t.Fatalf("first auth provider localized title = %+v, want zh/en title", payload.Data.Items[0].Title)
	}
	if payload.Data.Items[1].ID != "oidc" || !payload.Data.Items[1].Configured {
		t.Fatalf("second auth provider = %+v, want configured oidc provider", payload.Data.Items[1])
	}
	if strings.Contains(recorder.Body.String(), `"id":"wechat"`) {
		t.Fatalf("auth providers leaked app-only wechat provider: %s", recorder.Body.String())
	}
}

func TestAuthProvidersEndpointCachesDiscovery(t *testing.T) {
	cacheStore := cache.NewMemoryStore(cache.MemoryStoreOptions{})
	server := newTestServer(ServerOptions{Capabilities: []capability.Manifest{authProviderTestManifest()}, Cache: cacheStore})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/auth/providers", nil)

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("GET auth providers status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	value, ok, err := cacheStore.Get(request.Context(), "admin:auth-providers")
	if err != nil {
		t.Fatalf("cache get auth providers error = %v", err)
	}
	if !ok || !strings.Contains(string(value), `"id":"demo"`) {
		t.Fatalf("auth provider cache = %q, %v; want cached demo provider", string(value), ok)
	}
}

func TestDisabledDemoAuthProviderDoesNotLeakDiscoveryOrLogin(t *testing.T) {
	server := newTestServer(ServerOptions{
		Capabilities:            []capability.Manifest{authProviderTestManifest()},
		DisableDemoAuthProvider: true,
	})
	providersRecorder := httptest.NewRecorder()
	providersRequest := httptest.NewRequest(http.MethodGet, "/api/auth/providers", nil)

	server.Router().ServeHTTP(providersRecorder, providersRequest)

	if providersRecorder.Code != http.StatusOK {
		t.Fatalf("GET auth providers status = %d body = %s", providersRecorder.Code, providersRecorder.Body.String())
	}
	var payload authProviderListTestPayload
	if err := json.Unmarshal(providersRecorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode auth providers: %v body = %s", err, providersRecorder.Body.String())
	}
	ids := make([]string, 0, len(payload.Data.Items))
	for _, item := range payload.Data.Items {
		ids = append(ids, item.ID)
	}
	if containsString(ids, "demo") {
		t.Fatalf("auth provider ids = %+v, want no demo provider", ids)
	}
	if containsString(ids, "wechat") {
		t.Fatalf("auth provider ids = %+v, want app-only wechat provider hidden", ids)
	}

	loginBody := bytes.NewBufferString(`{"provider":"demo","username":"ops"}`)
	loginRecorder := httptest.NewRecorder()
	loginRequest := httptest.NewRequest(http.MethodPost, "/api/auth/login", loginBody)
	loginRequest.Header.Set("Content-Type", "application/json")

	server.Router().ServeHTTP(loginRecorder, loginRequest)

	if loginRecorder.Code != http.StatusBadRequest {
		t.Fatalf("POST auth login status = %d body = %s, want 400", loginRecorder.Code, loginRecorder.Body.String())
	}
	if !strings.Contains(loginRecorder.Body.String(), "AUTH_PROVIDER_NOT_FOUND") {
		t.Fatalf("POST auth login body = %s, want AUTH_PROVIDER_NOT_FOUND", loginRecorder.Body.String())
	}
}

func TestDisabledCapabilitiesDoNotLeakAuthProviders(t *testing.T) {
	server := newTestServer(ServerOptions{Capabilities: capabilitiesFromConfigForTest(t, []string{"dictionary", "tenant", "identity", "session", "rbac", "menu", "admin-shell"})})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/auth/providers", nil)

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("GET auth providers status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	var payload authProviderListTestPayload
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode auth providers: %v body = %s", err, recorder.Body.String())
	}
	ids := make([]string, 0, len(payload.Data.Items))
	for _, item := range payload.Data.Items {
		ids = append(ids, item.ID)
	}
	if !containsString(ids, "demo") {
		t.Fatalf("auth provider ids = %+v, want demo from enabled session capability", ids)
	}
	if containsString(ids, "wechat") {
		t.Fatalf("auth provider ids = %+v, want no wechat provider from disabled wechat-login capability", ids)
	}
}

func TestAuthLoginWithDemoProviderReturnsRoleBackedSessionToken(t *testing.T) {
	server := newTestServer(ServerOptions{Capabilities: []capability.Manifest{authProviderTestManifest()}})
	loginBody := bytes.NewBufferString(`{"provider":"demo","username":"ops"}`)
	loginRecorder := httptest.NewRecorder()
	loginRequest := httptest.NewRequest(http.MethodPost, "/api/auth/login", loginBody)
	loginRequest.Header.Set("Content-Type", "application/json")

	server.Router().ServeHTTP(loginRecorder, loginRequest)

	if loginRecorder.Code != http.StatusOK {
		t.Fatalf("POST auth login status = %d body = %s", loginRecorder.Code, loginRecorder.Body.String())
	}
	var login authLoginTestPayload
	if err := json.Unmarshal(loginRecorder.Body.Bytes(), &login); err != nil {
		t.Fatalf("decode auth login: %v body = %s", err, loginRecorder.Body.String())
	}
	if login.Data.Token == "" {
		t.Fatalf("auth login token is empty")
	}
	if strings.Count(login.Data.Token, ".") != 2 {
		t.Fatalf("auth login token = %q, want jwt bearer token", login.Data.Token)
	}
	claims, err := server.tokens.Parse(login.Data.Token)
	if err != nil {
		t.Fatalf("parse auth login jwt: %v", err)
	}
	if claims.TokenType != authjwt.TokenTypeAdmin || claims.SessionID == "" || claims.Username != "ops" {
		t.Fatalf("auth login claims = %+v, want admin ops session claims", claims)
	}
	if login.Data.ExpiresAt.IsZero() {
		t.Fatalf("auth login expiresAt is zero")
	}
	if login.Data.Principal.User.Username != "ops" {
		t.Fatalf("login principal username = %q, want ops", login.Data.Principal.User.Username)
	}
	if !containsString(login.Data.Principal.Roles, "operator") || !containsString(login.Data.Principal.Permissions, "admin:tenant:read") {
		t.Fatalf("login principal = %+v, want operator permissions", login.Data.Principal)
	}

	sessionRecorder := httptest.NewRecorder()
	sessionRequest := httptest.NewRequest(http.MethodGet, "/api/admin/session/current", nil)
	sessionRequest.Header.Set("Authorization", "Bearer "+login.Data.Token)
	server.Router().ServeHTTP(sessionRecorder, sessionRequest)

	if sessionRecorder.Code != http.StatusOK {
		t.Fatalf("GET current session with token status = %d body = %s", sessionRecorder.Code, sessionRecorder.Body.String())
	}
	var session currentSessionTestPayload
	if err := json.Unmarshal(sessionRecorder.Body.Bytes(), &session); err != nil {
		t.Fatalf("decode current session: %v body = %s", err, sessionRecorder.Body.String())
	}
	if session.Data.User.Username != "ops" {
		t.Fatalf("token session username = %q, want ops", session.Data.User.Username)
	}
}

func TestAdminOIDCStartRejectsUnavailableProviderResolverAndMalformedChallenge(t *testing.T) {
	validChallenge := strings.Repeat("a", 43)
	tests := []struct {
		name       string
		provider   string
		challenge  string
		manifest   capability.Manifest
		resolver   AdminIdentityResolver
		wantStatus int
		wantCode   string
	}{
		{
			name: "app-only provider", provider: "wechat", challenge: validChallenge,
			manifest: adminAudienceAuthProviderTestManifest(), resolver: successfulAdminIdentityResolver(),
			wantStatus: http.StatusBadRequest, wantCode: "AUTH_PROVIDER_NOT_FOUND",
		},
		{
			name: "disabled provider", provider: "oidc", challenge: validChallenge,
			manifest: adminOIDCProviderManifest(false, true), resolver: successfulAdminIdentityResolver(),
			wantStatus: http.StatusBadRequest, wantCode: "AUTH_PROVIDER_NOT_FOUND",
		},
		{
			name: "unconfigured provider", provider: "oidc", challenge: validChallenge,
			manifest: adminOIDCProviderManifest(true, false), resolver: successfulAdminIdentityResolver(),
			wantStatus: http.StatusBadRequest, wantCode: "AUTH_PROVIDER_NOT_CONFIGURED",
		},
		{
			name: "missing resolver", provider: "oidc", challenge: validChallenge,
			manifest: adminOIDCProviderManifest(true, true), resolver: nil,
			wantStatus: http.StatusNotImplemented, wantCode: "AUTH_PROVIDER_RESOLVER_NOT_CONFIGURED",
		},
		{
			name: "malformed challenge", provider: "oidc", challenge: "not-s256",
			manifest: adminOIDCProviderManifest(true, true),
			resolver: adminIdentityResolverFunc{
				start: func(context.Context, AdminIdentityStartInput) (AdminIdentityStart, error) {
					return AdminIdentityStart{}, ErrAdminIdentityInvalid
				},
				resolve: func(context.Context, AdminIdentityResolveInput) (AdminIdentity, error) {
					return AdminIdentity{}, nil
				},
			},
			wantStatus: http.StatusBadRequest, wantCode: "AUTH_PROVIDER_START_INVALID",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := newTestServer(ServerOptions{
				Capabilities:          []capability.Manifest{test.manifest},
				AdminIdentityResolver: test.resolver,
			})
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/api/auth/providers/"+test.provider+"/start", bytes.NewBufferString(`{"codeChallenge":"`+test.challenge+`"}`))
			request.Header.Set("Content-Type", "application/json")

			server.Router().ServeHTTP(recorder, request)

			assertAuthErrorResponse(t, recorder, test.wantStatus, test.wantCode)
			if strings.Contains(recorder.Body.String(), test.challenge) || strings.Contains(recorder.Body.String(), "client-secret") {
				t.Fatalf("admin oidc start error leaked sensitive input: %s", recorder.Body.String())
			}
		})
	}
}

func TestAdminOIDCStartReturnsResolverTransactionWithoutCredentials(t *testing.T) {
	expiresAt := time.Date(2026, time.July, 11, 10, 5, 0, 0, time.UTC)
	challenge := "47DEQpj8HBSa-_TImW-5JCeuQeRkm5NMpJWZG3hSuFU"
	var captured AdminIdentityStartInput
	server := newTestServer(ServerOptions{
		Capabilities: []capability.Manifest{adminOIDCProviderManifest(true, true)},
		AdminIdentityResolver: adminIdentityResolverFunc{
			start: func(_ context.Context, input AdminIdentityStartInput) (AdminIdentityStart, error) {
				captured = input
				return AdminIdentityStart{
					AuthorizationURL: "https://id.example/authorize?state=state-exact",
					State:            "state-exact",
					ExpiresAt:        expiresAt,
				}, nil
			},
			resolve: func(context.Context, AdminIdentityResolveInput) (AdminIdentity, error) {
				return AdminIdentity{}, nil
			},
		},
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/auth/providers/oidc/start", bytes.NewBufferString(`{"codeChallenge":"`+challenge+`"}`))
	request.Header.Set("Content-Type", "application/json")

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("POST admin oidc start status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	if captured.Provider.ID != "oidc" || captured.Provider.Kind != "oidc" || captured.CodeChallenge != challenge {
		t.Fatalf("captured admin oidc start = %+v, want oidc provider and exact challenge", captured)
	}
	var payload adminIdentityStartTestPayload
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode admin oidc start: %v body = %s", err, recorder.Body.String())
	}
	if payload.Data.AuthorizationURL != "https://id.example/authorize?state=state-exact" || payload.Data.State != "state-exact" || !payload.Data.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("admin oidc start payload = %+v, want exact resolver transaction", payload.Data)
	}
	for _, sensitive := range []string{"client-secret", "codeVerifier", "password", "accessToken", "refreshToken"} {
		if strings.Contains(recorder.Body.String(), sensitive) {
			t.Fatalf("admin oidc start response leaked credential field %q: %s", sensitive, recorder.Body.String())
		}
	}
}

func TestAppAuthLoginRejectsAdminOnlyOIDCProvider(t *testing.T) {
	server := newTestServer(ServerOptions{
		Capabilities: []capability.Manifest{adminAudienceAuthProviderTestManifest()},
		AppIdentityResolver: appIdentityResolverFunc(func(context.Context, AppIdentityResolveInput) (AppIdentity, error) {
			t.Fatalf("app resolver must not receive admin-only oidc provider")
			return AppIdentity{}, nil
		}),
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/app/auth/login", bytes.NewBufferString(`{"provider":"oidc","code":"admin-code"}`))
	request.Header.Set("Content-Type", "application/json")

	server.Router().ServeHTTP(recorder, request)

	assertAuthErrorResponse(t, recorder, http.StatusBadRequest, "APP_AUTH_PROVIDER_NOT_FOUND")
	if strings.Contains(recorder.Body.String(), "admin-code") {
		t.Fatalf("app oidc rejection leaked code: %s", recorder.Body.String())
	}
}

func TestAdminOIDCAuthLoginRejectsMissingTransactionResolverBindingAndDisabledPrincipal(t *testing.T) {
	sensitiveCode := "oidc-code-sensitive"
	sensitiveState := "oidc-state-sensitive"
	sensitiveVerifier := "oidc-verifier-sensitive"
	sensitiveIssuer := "https://issuer-sensitive.example"
	sensitiveSubject := "subject-sensitive"

	t.Run("missing state", func(t *testing.T) {
		server := newTestServer(ServerOptions{
			Capabilities:          []capability.Manifest{adminOIDCProviderManifest(true, true)},
			AdminIdentityResolver: successfulAdminIdentityResolver(),
		})
		recorder := postAdminOIDCLoginForTest(server, `{"provider":"oidc","code":"`+sensitiveCode+`","codeVerifier":"`+sensitiveVerifier+`"}`)
		assertAuthErrorResponse(t, recorder, http.StatusBadRequest, "AUTH_IDENTITY_TRANSACTION_REQUIRED")
		assertResponseRedactsValues(t, recorder.Body.String(), sensitiveCode, sensitiveVerifier)
	})

	t.Run("missing verifier", func(t *testing.T) {
		server := newTestServer(ServerOptions{
			Capabilities:          []capability.Manifest{adminOIDCProviderManifest(true, true)},
			AdminIdentityResolver: successfulAdminIdentityResolver(),
		})
		recorder := postAdminOIDCLoginForTest(server, `{"provider":"oidc","code":"`+sensitiveCode+`","state":"`+sensitiveState+`"}`)
		assertAuthErrorResponse(t, recorder, http.StatusBadRequest, "AUTH_IDENTITY_TRANSACTION_REQUIRED")
		assertResponseRedactsValues(t, recorder.Body.String(), sensitiveCode, sensitiveState)
	})

	t.Run("missing resolver", func(t *testing.T) {
		server := newTestServer(ServerOptions{
			Capabilities: []capability.Manifest{adminOIDCProviderManifest(true, true)},
		})
		recorder := postAdminOIDCLoginForTest(server, adminOIDCLoginBody(sensitiveCode, sensitiveState, sensitiveVerifier))
		assertAuthErrorResponse(t, recorder, http.StatusNotImplemented, "AUTH_PROVIDER_RESOLVER_NOT_CONFIGURED")
		assertResponseRedactsValues(t, recorder.Body.String(), sensitiveCode, sensitiveState, sensitiveVerifier)
	})

	t.Run("invalid identity", func(t *testing.T) {
		server := newTestServer(ServerOptions{
			Capabilities: []capability.Manifest{adminOIDCProviderManifest(true, true)},
			AdminIdentityResolver: adminIdentityResolverFunc{
				start: func(context.Context, AdminIdentityStartInput) (AdminIdentityStart, error) {
					return AdminIdentityStart{}, nil
				},
				resolve: func(context.Context, AdminIdentityResolveInput) (AdminIdentity, error) {
					return AdminIdentity{}, errors.Join(ErrAdminIdentityInvalid, errors.New("identity-detail-sensitive"))
				},
			},
		})
		recorder := postAdminOIDCLoginForTest(server, adminOIDCLoginBody(sensitiveCode, sensitiveState, sensitiveVerifier))
		assertAuthErrorResponse(t, recorder, http.StatusBadRequest, "AUTH_IDENTITY_INVALID")
		assertResponseRedactsValues(t, recorder.Body.String(), sensitiveCode, sensitiveState, sensitiveVerifier, "identity-detail-sensitive")
	})

	t.Run("invalid transaction", func(t *testing.T) {
		server := newTestServer(ServerOptions{
			Capabilities: []capability.Manifest{adminOIDCProviderManifest(true, true)},
			AdminIdentityResolver: adminIdentityResolverFunc{
				start: func(context.Context, AdminIdentityStartInput) (AdminIdentityStart, error) {
					return AdminIdentityStart{}, nil
				},
				resolve: func(context.Context, AdminIdentityResolveInput) (AdminIdentity, error) {
					return AdminIdentity{}, errors.Join(ErrAdminIdentityTransaction, errors.New("transaction-detail-sensitive"))
				},
			},
		})
		recorder := postAdminOIDCLoginForTest(server, adminOIDCLoginBody(sensitiveCode, sensitiveState, sensitiveVerifier))
		assertAuthErrorResponse(t, recorder, http.StatusUnauthorized, "AUTH_IDENTITY_TRANSACTION_INVALID")
		assertResponseRedactsValues(t, recorder.Body.String(), sensitiveCode, sensitiveState, sensitiveVerifier, "transaction-detail-sensitive")
	})

	t.Run("resolver error", func(t *testing.T) {
		bindingCalled := false
		server := newTestServer(ServerOptions{
			Capabilities: []capability.Manifest{adminOIDCProviderManifest(true, true)},
			AdminIdentityResolver: adminIdentityResolverFunc{
				start: func(context.Context, AdminIdentityStartInput) (AdminIdentityStart, error) {
					return AdminIdentityStart{}, nil
				},
				resolve: func(context.Context, AdminIdentityResolveInput) (AdminIdentity, error) {
					return AdminIdentity{}, errors.New("provider exchange exposed client-secret-sensitive")
				},
			},
			AdminIdentityBindings: adminIdentityBindingStoreFunc{resolve: func(context.Context, AdminIdentityBindingInput) (AdminIdentityBinding, error) {
				bindingCalled = true
				return AdminIdentityBinding{}, nil
			}},
		})
		recorder := postAdminOIDCLoginForTest(server, adminOIDCLoginBody(sensitiveCode, sensitiveState, sensitiveVerifier))
		assertAuthErrorResponse(t, recorder, http.StatusBadGateway, "AUTH_PROVIDER_RESOLVE_FAILED")
		if bindingCalled {
			t.Fatalf("binding store called after resolver failure")
		}
		assertResponseRedactsValues(t, recorder.Body.String(), sensitiveCode, sensitiveState, sensitiveVerifier, "client-secret-sensitive")
	})

	t.Run("missing binding", func(t *testing.T) {
		var captured AdminIdentityBindingInput
		server := newTestServer(ServerOptions{
			Capabilities: []capability.Manifest{adminOIDCProviderManifest(true, true)},
			AdminIdentityResolver: adminIdentityResolverFunc{
				start: func(context.Context, AdminIdentityStartInput) (AdminIdentityStart, error) {
					return AdminIdentityStart{}, nil
				},
				resolve: func(context.Context, AdminIdentityResolveInput) (AdminIdentity, error) {
					return AdminIdentity{Issuer: sensitiveIssuer, ProviderSubject: sensitiveSubject}, nil
				},
			},
			AdminIdentityBindings: adminIdentityBindingStoreFunc{resolve: func(_ context.Context, input AdminIdentityBindingInput) (AdminIdentityBinding, error) {
				captured = input
				return AdminIdentityBinding{}, ErrAdminIdentityBindingInvalid
			}},
		})
		recorder := postAdminOIDCLoginForTest(server, adminOIDCLoginBody(sensitiveCode, sensitiveState, sensitiveVerifier))
		assertAuthErrorResponse(t, recorder, http.StatusUnauthorized, "AUTH_IDENTITY_NOT_BOUND")
		if captured.Provider.ID != "oidc" || captured.Issuer != sensitiveIssuer || captured.ProviderSubject != sensitiveSubject {
			t.Fatalf("captured binding input = %+v, want resolved oidc tuple", captured)
		}
		assertResponseRedactsValues(t, recorder.Body.String(), sensitiveCode, sensitiveState, sensitiveVerifier, sensitiveIssuer, sensitiveSubject)
	})

	t.Run("binding persistence failure", func(t *testing.T) {
		server := newTestServer(ServerOptions{
			Capabilities:          []capability.Manifest{adminOIDCProviderManifest(true, true)},
			AdminIdentityResolver: successfulAdminIdentityResolver(),
			AdminIdentityBindings: adminIdentityBindingStoreFunc{resolve: func(context.Context, AdminIdentityBindingInput) (AdminIdentityBinding, error) {
				return AdminIdentityBinding{}, errors.New("binding-storage-detail-sensitive")
			}},
		})
		recorder := postAdminOIDCLoginForTest(server, adminOIDCLoginBody(sensitiveCode, sensitiveState, sensitiveVerifier))
		assertAuthErrorResponse(t, recorder, http.StatusInternalServerError, "AUTH_IDENTITY_BINDING_FAILED")
		assertResponseRedactsValues(t, recorder.Body.String(), sensitiveCode, sensitiveState, sensitiveVerifier, "binding-storage-detail-sensitive")
	})

	t.Run("disabled principal", func(t *testing.T) {
		server := newTestServer(ServerOptions{
			Capabilities:          []capability.Manifest{adminOIDCProviderManifest(true, true)},
			AdminIdentityResolver: successfulAdminIdentityResolver(),
			AdminIdentityBindings: adminIdentityBindingStoreFunc{resolve: func(context.Context, AdminIdentityBindingInput) (AdminIdentityBinding, error) {
				return AdminIdentityBinding{Username: "ops"}, nil
			}},
		})
		disableAdminUserForTest(t, server.resources, "ops")
		recorder := postAdminOIDCLoginForTest(server, adminOIDCLoginBody(sensitiveCode, sensitiveState, sensitiveVerifier))
		assertAuthErrorResponse(t, recorder, http.StatusUnauthorized, "AUTH_INVALID_CREDENTIALS")
		assertResponseRedactsValues(t, recorder.Body.String(), sensitiveCode, sensitiveState, sensitiveVerifier)
	})
}

func TestAdminOIDCAuthLoginUsesAtomicBindingAndExistingAdminSessionAuditPath(t *testing.T) {
	manifests := configuredAdminOIDCPlatformManifests(t)
	resources := adminresource.NewStoreFromCapabilities(manifests)
	provider := authProviderByIDForTest(t, manifests, "oidc")
	loginAt := time.Date(2026, time.July, 11, 11, 0, 0, 0, time.UTC)
	issuer := "https://issuer-sensitive.example"
	subject := "subject-sensitive"
	bindings := NewResourceAdminIdentityBindingStore(resources, func() time.Time { return loginAt })
	if _, err := bindings.ProvisionAdminIdentityBinding(context.Background(), AdminIdentityProvisionInput{
		Provider: provider, Issuer: issuer, ProviderSubject: subject, Username: "ops", Now: loginAt.Add(-time.Hour),
	}); err != nil {
		t.Fatalf("ProvisionAdminIdentityBinding() error = %v", err)
	}

	code := "oidc-code-sensitive"
	state := "oidc-state-sensitive"
	verifier := "oidc-verifier-sensitive"
	var captured AdminIdentityResolveInput
	bus := cache.NewMemoryInvalidationBus()
	invalidated := false
	server := newTestServer(ServerOptions{
		Capabilities:          manifests,
		Resources:             resources,
		InvalidationBus:       bus,
		AdminIdentityBindings: bindings,
		Now:                   func() time.Time { return loginAt },
		AdminIdentityResolver: adminIdentityResolverFunc{
			start: func(context.Context, AdminIdentityStartInput) (AdminIdentityStart, error) {
				return AdminIdentityStart{}, nil
			},
			resolve: func(_ context.Context, input AdminIdentityResolveInput) (AdminIdentity, error) {
				captured = input
				return AdminIdentity{Issuer: issuer, ProviderSubject: subject}, nil
			},
		},
	})
	if err := bus.SubscribeInvalidations(context.Background(), func(_ context.Context, event cache.InvalidationEvent) {
		if event.Resource == sessionInvalidationResource {
			invalidated = true
		}
	}); err != nil {
		t.Fatalf("SubscribeInvalidations() error = %v", err)
	}

	recorder := postAdminOIDCLoginForTest(server, adminOIDCLoginBody(code, state, verifier))

	if recorder.Code != http.StatusOK {
		t.Fatalf("POST admin oidc login status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	if captured.Provider.ID != "oidc" || captured.Code != code || captured.State != state || captured.CodeVerifier != verifier {
		t.Fatalf("captured oidc resolve input = %+v, want exact exchange transaction", captured)
	}
	var login authLoginTestPayload
	if err := json.Unmarshal(recorder.Body.Bytes(), &login); err != nil {
		t.Fatalf("decode admin oidc login: %v body = %s", err, recorder.Body.String())
	}
	claims, err := server.tokens.Parse(login.Data.Token)
	if err != nil {
		t.Fatalf("parse admin oidc jwt: %v", err)
	}
	if claims.TokenType != authjwt.TokenTypeAdmin || claims.TenantID != platformTenant || claims.Username != "ops" || claims.SessionID == "" {
		t.Fatalf("admin oidc jwt claims = %+v, want existing admin session shape", claims)
	}
	if login.Data.Principal.User.Username != "ops" || !containsString(login.Data.Principal.Permissions, "admin:tenant:read") {
		t.Fatalf("admin oidc principal = %+v, want validated ops principal", login.Data.Principal)
	}
	if !invalidated {
		t.Fatalf("admin oidc login did not publish session invalidation")
	}

	bindingRecords, err := resources.List(adminIdentitiesResource)
	if err != nil || len(bindingRecords) != 1 || bindingRecords[0].Values["lastLoginAt"] != loginAt.Format(time.RFC3339) {
		t.Fatalf("atomic binding resolve records = %+v, %v, want updated lastLoginAt", bindingRecords, err)
	}
	auditRecords, err := resources.List("audit-logs")
	if err != nil {
		t.Fatalf("List(audit-logs) error = %v", err)
	}
	var loginAudit *adminresource.Record
	for index := range auditRecords {
		if auditRecords[index].Values["action"] == "auth.login" {
			loginAudit = &auditRecords[index]
		}
	}
	if loginAudit == nil || loginAudit.Values["actor"] != "ops" || loginAudit.Values["provider"] != "oidc" || loginAudit.Values["sessionId"] != "" {
		t.Fatalf("admin oidc audit = %+v, want credential-free auth.login shape", loginAudit)
	}
	serializedAudit, err := json.Marshal(loginAudit)
	if err != nil {
		t.Fatalf("marshal auth.login audit: %v", err)
	}
	assertResponseRedactsValues(t, string(serializedAudit), code, state, verifier, issuer, subject, login.Data.Token)
}

func TestAuthLoginWithDemoProviderRejectsDisabledPrincipal(t *testing.T) {
	server := newTestServer(ServerOptions{Capabilities: []capability.Manifest{authProviderTestManifest()}})
	disableAdminUserForTest(t, server.resources, "ops")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"provider":"demo","username":"ops"}`))
	request.Header.Set("Content-Type", "application/json")
	server.Router().ServeHTTP(recorder, request)

	assertAuthErrorResponse(t, recorder, http.StatusUnauthorized, "AUTH_INVALID_CREDENTIALS")
}

func TestRepeatedAdminLoginsPersistDistinctAuditCodes(t *testing.T) {
	capabilities := capabilitiesFromConfigForTest(t, []string{"dictionary", "tenant", "identity", "session", "rbac", "audit"})
	resources := openGORMAdminResourceStoreForHTTPTest(t, filepath.Join(t.TempDir(), "admin-resources.db"), capabilities)
	server := newTestServer(ServerOptions{Capabilities: capabilities, Resources: resources})

	loginForTest(t, server, "admin")
	loginForTest(t, server, "admin")

	audits, err := resources.List("audit-logs")
	if err != nil {
		t.Fatalf("List(audit-logs) error = %v", err)
	}
	codes := map[string]struct{}{}
	for _, audit := range audits {
		if audit.Values["action"] != "auth.login" {
			continue
		}
		if !strings.HasPrefix(audit.Code, "auth.login.") {
			t.Fatalf("auth login audit code = %q, want unique auth.login prefix", audit.Code)
		}
		codes[audit.Code] = struct{}{}
	}
	if len(codes) != 2 {
		t.Fatalf("auth login audit codes = %+v, want two distinct persisted events", codes)
	}
}

func TestAuthLoginCleansUpIssuedSessionWhenAuditFails(t *testing.T) {
	for _, test := range []struct {
		name              string
		cleanupErr        error
		wantCode          string
		wantInvalidations int
		wantResolvable    bool
	}{
		{name: "cleanup succeeds", wantCode: "AUTH_AUDIT_FAILED", wantInvalidations: 2},
		{
			name: "cleanup fails", cleanupErr: errors.New("revoke-detail-sensitive"),
			wantCode: "AUTH_SESSION_CLEANUP_FAILED", wantInvalidations: 1, wantResolvable: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			capabilities := capabilitiesFromConfigForTest(t, []string{"dictionary", "tenant", "identity", "session", "rbac", "audit"})
			adminRepository := &controllableAdminResourceRepository{}
			resources, err := adminresource.NewRepositoryBackedStoreFromCapabilities(adminRepository, capabilities)
			if err != nil {
				t.Fatalf("NewRepositoryBackedStoreFromCapabilities() error = %v", err)
			}
			sessionRepository := newControllableSessionRepository()
			sessionRepository.revokeErr = test.cleanupErr
			sessions, err := session.NewRepositoryBackedStore(session.Options{TTL: time.Hour}, sessionRepository)
			if err != nil {
				t.Fatalf("NewRepositoryBackedStore() error = %v", err)
			}
			bus := cache.NewMemoryInvalidationBus()
			invalidations := 0
			server := newTestServer(ServerOptions{
				Capabilities:    capabilities,
				Resources:       resources,
				Sessions:        sessions,
				InvalidationBus: bus,
			})
			if err := bus.SubscribeInvalidations(context.Background(), func(_ context.Context, event cache.InvalidationEvent) {
				if event.Resource == sessionInvalidationResource {
					invalidations++
				}
			}); err != nil {
				t.Fatalf("SubscribeInvalidations() error = %v", err)
			}
			adminRepository.saveErr = errors.New("audit-save-detail-sensitive")

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"provider":"demo","username":"ops"}`))
			request.Header.Set("Content-Type", "application/json")
			server.Router().ServeHTTP(recorder, request)

			assertAuthErrorResponse(t, recorder, http.StatusInternalServerError, test.wantCode)
			if len(sessionRepository.sessions) != 1 {
				t.Fatalf("issued sessions = %d, want one attempted login session", len(sessionRepository.sessions))
			}
			var issuedDigest string
			for tokenDigest := range sessionRepository.sessions {
				issuedDigest = tokenDigest
			}
			_, resolvable, err := sessionRepository.Resolve(context.Background(), issuedDigest, time.Now())
			if err != nil {
				t.Fatalf("repository Resolve(issued) error = %v", err)
			}
			if resolvable != test.wantResolvable {
				t.Fatalf("issued session resolvable = %v, want %v", resolvable, test.wantResolvable)
			}
			if invalidations != test.wantInvalidations {
				t.Fatalf("session invalidations = %d, want %d", invalidations, test.wantInvalidations)
			}
			assertResponseRedactsValues(t, recorder.Body.String(), issuedDigest, "audit-save-detail-sensitive", "revoke-detail-sensitive")
		})
	}
}

func TestAuthLogoutRevokesSessionToken(t *testing.T) {
	server := newTestServer(ServerOptions{Capabilities: []capability.Manifest{authProviderTestManifest()}})
	login := loginForTest(t, server, "ops")

	logoutRecorder := httptest.NewRecorder()
	logoutRequest := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	logoutRequest.Header.Set("Authorization", "Bearer "+login.Data.Token)
	server.Router().ServeHTTP(logoutRecorder, logoutRequest)

	if logoutRecorder.Code != http.StatusOK {
		t.Fatalf("POST auth logout status = %d body = %s", logoutRecorder.Code, logoutRecorder.Body.String())
	}

	sessionRecorder := httptest.NewRecorder()
	sessionRequest := httptest.NewRequest(http.MethodGet, "/api/admin/session/current", nil)
	sessionRequest.Header.Set("Authorization", "Bearer "+login.Data.Token)
	server.Router().ServeHTTP(sessionRecorder, sessionRequest)

	if sessionRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("GET current session after logout status = %d body = %s, want 401", sessionRecorder.Code, sessionRecorder.Body.String())
	}
}

func TestAuthRefreshRenewsAdminSessionToken(t *testing.T) {
	now := time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC)
	server := newTestServer(ServerOptions{
		Capabilities: []capability.Manifest{authProviderTestManifest()},
		SessionTTL:   time.Hour,
		Now: func() time.Time {
			return now
		},
	})
	login := loginForTest(t, server, "ops")
	loginClaims, err := server.tokens.Parse(login.Data.Token)
	if err != nil {
		t.Fatalf("parse login token: %v", err)
	}

	now = now.Add(45 * time.Minute)
	refreshRecorder := httptest.NewRecorder()
	refreshRequest := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
	refreshRequest.Header.Set("Authorization", "Bearer "+login.Data.Token)
	server.Router().ServeHTTP(refreshRecorder, refreshRequest)

	if refreshRecorder.Code != http.StatusOK {
		t.Fatalf("POST auth refresh status = %d body = %s", refreshRecorder.Code, refreshRecorder.Body.String())
	}
	var refreshed authLoginTestPayload
	if err := json.Unmarshal(refreshRecorder.Body.Bytes(), &refreshed); err != nil {
		t.Fatalf("decode auth refresh: %v body = %s", err, refreshRecorder.Body.String())
	}
	if refreshed.Data.Token == "" || refreshed.Data.Token == login.Data.Token {
		t.Fatalf("refresh token = %q, want new non-empty token different from login token", refreshed.Data.Token)
	}
	if !refreshed.Data.ExpiresAt.After(login.Data.ExpiresAt) {
		t.Fatalf("refresh expiresAt = %s, want after original %s", refreshed.Data.ExpiresAt, login.Data.ExpiresAt)
	}
	if refreshed.Data.Principal.User.Username != "ops" {
		t.Fatalf("refresh principal username = %q, want ops", refreshed.Data.Principal.User.Username)
	}
	refreshClaims, err := server.tokens.Parse(refreshed.Data.Token)
	if err != nil {
		t.Fatalf("parse refreshed token: %v", err)
	}
	if refreshClaims.TokenType != authjwt.TokenTypeAdmin || refreshClaims.Username != "ops" || refreshClaims.SessionID != loginClaims.SessionID {
		t.Fatalf("refresh claims = %+v, want same admin ops session %q", refreshClaims, loginClaims.SessionID)
	}

	now = login.Data.ExpiresAt.Add(30 * time.Minute)
	oldRecorder := httptest.NewRecorder()
	oldRequest := httptest.NewRequest(http.MethodGet, "/api/admin/session/current", nil)
	oldRequest.Header.Set("Authorization", "Bearer "+login.Data.Token)
	server.Router().ServeHTTP(oldRecorder, oldRequest)
	if oldRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("GET current session with original token status = %d body = %s, want 401", oldRecorder.Code, oldRecorder.Body.String())
	}

	newRecorder := httptest.NewRecorder()
	newRequest := httptest.NewRequest(http.MethodGet, "/api/admin/session/current", nil)
	newRequest.Header.Set("Authorization", "Bearer "+refreshed.Data.Token)
	server.Router().ServeHTTP(newRecorder, newRequest)
	if newRecorder.Code != http.StatusOK {
		t.Fatalf("GET current session with refreshed token status = %d body = %s", newRecorder.Code, newRecorder.Body.String())
	}

	logoutRecorder := httptest.NewRecorder()
	logoutRequest := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	logoutRequest.Header.Set("Authorization", "Bearer "+refreshed.Data.Token)
	server.Router().ServeHTTP(logoutRecorder, logoutRequest)
	if logoutRecorder.Code != http.StatusOK {
		t.Fatalf("POST auth logout with refreshed token status = %d body = %s", logoutRecorder.Code, logoutRecorder.Body.String())
	}

	revokedRecorder := httptest.NewRecorder()
	revokedRequest := httptest.NewRequest(http.MethodGet, "/api/admin/session/current", nil)
	revokedRequest.Header.Set("Authorization", "Bearer "+refreshed.Data.Token)
	server.Router().ServeHTTP(revokedRecorder, revokedRequest)
	if revokedRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("GET current session after refreshed logout status = %d body = %s, want 401", revokedRecorder.Code, revokedRecorder.Body.String())
	}
}

func TestAuthRefreshRepositoryErrorReturnsInternalAuthError(t *testing.T) {
	now := time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC)
	repository := newControllableSessionRepository()
	sessions, err := session.NewRepositoryBackedStore(session.Options{TTL: time.Hour, Now: func() time.Time { return now }}, repository)
	if err != nil {
		t.Fatalf("NewRepositoryBackedStore() error = %v", err)
	}
	server := newTestServer(ServerOptions{
		Capabilities: []capability.Manifest{authProviderTestManifest()},
		Sessions:     sessions,
		Now:          func() time.Time { return now },
	})
	login := loginForTest(t, server, "ops")
	repository.renewErr = errors.New("renew unavailable")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
	request.Header.Set("Authorization", "Bearer "+login.Data.Token)
	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("POST auth refresh repository error status = %d body = %s, want 500", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"code":"AUTH_SESSION_RENEW_FAILED"`) {
		t.Fatalf("POST auth refresh repository error body = %s, want AUTH_SESSION_RENEW_FAILED", recorder.Body.String())
	}
}

func TestAuthRefreshResolveRepositoryErrorReturnsInternalAuthError(t *testing.T) {
	server, repository, token := newAdminSessionWriteTestServer(t)
	repository.resolveErr = errors.New("resolve unavailable")

	recorder := performBearerRequest(server, http.MethodPost, "/api/auth/refresh", token)

	assertAuthErrorResponse(t, recorder, http.StatusInternalServerError, "AUTH_SESSION_RENEW_FAILED")
}

func TestAuthLogoutRepositoryErrorReturnsInternalAuthError(t *testing.T) {
	now := time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC)
	repository := newControllableSessionRepository()
	sessions, err := session.NewRepositoryBackedStore(session.Options{TTL: time.Hour, Now: func() time.Time { return now }}, repository)
	if err != nil {
		t.Fatalf("NewRepositoryBackedStore() error = %v", err)
	}
	server := newTestServer(ServerOptions{
		Capabilities: []capability.Manifest{authProviderTestManifest()},
		Sessions:     sessions,
		Now:          func() time.Time { return now },
	})
	login := loginForTest(t, server, "ops")
	repository.revokeErr = errors.New("revoke unavailable")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	request.Header.Set("Authorization", "Bearer "+login.Data.Token)
	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("POST auth logout repository error status = %d body = %s, want 500", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"code":"AUTH_SESSION_REVOKE_FAILED"`) {
		t.Fatalf("POST auth logout repository error body = %s, want AUTH_SESSION_REVOKE_FAILED", recorder.Body.String())
	}
	if _, ok := sessions.Resolve(loginSessionIDForTest(t, server, login.Data.Token)); !ok {
		t.Fatal("admin session became inactive after failed repository revoke")
	}
}

func TestAuthLogoutResolveRepositoryErrorReturnsInternalAuthError(t *testing.T) {
	server, repository, token := newAdminSessionWriteTestServer(t)
	repository.resolveErr = errors.New("resolve unavailable")

	recorder := performBearerRequest(server, http.MethodPost, "/api/auth/logout", token)

	assertAuthErrorResponse(t, recorder, http.StatusInternalServerError, "AUTH_SESSION_REVOKE_FAILED")
}

func TestAdminSessionWriteHandlersKeepNotFoundResolveUnauthorized(t *testing.T) {
	for _, endpoint := range []string{"/api/auth/refresh", "/api/auth/logout"} {
		t.Run(endpoint, func(t *testing.T) {
			server, repository, token := newAdminSessionWriteTestServer(t)
			delete(repository.sessions, session.DigestToken(loginSessionIDForTest(t, server, token)))

			recorder := performBearerRequest(server, http.MethodPost, endpoint, token)

			assertAuthErrorResponse(t, recorder, http.StatusUnauthorized, "AUTH_UNAUTHORIZED")
		})
	}
}

func TestAppAuthLoginIssuesAppTokenAndCurrentSession(t *testing.T) {
	server := newTestServer(ServerOptions{Capabilities: []capability.Manifest{authProviderTestManifest()}})
	loginBody := bytes.NewBufferString(`{"username":"guest-alpha"}`)
	loginRecorder := httptest.NewRecorder()
	loginRequest := httptest.NewRequest(http.MethodPost, "/api/app/auth/login", loginBody)
	loginRequest.Header.Set("Content-Type", "application/json")

	server.Router().ServeHTTP(loginRecorder, loginRequest)

	if loginRecorder.Code != http.StatusOK {
		t.Fatalf("POST app auth login status = %d body = %s", loginRecorder.Code, loginRecorder.Body.String())
	}
	var login appLoginTestPayload
	if err := json.Unmarshal(loginRecorder.Body.Bytes(), &login); err != nil {
		t.Fatalf("decode app auth login: %v body = %s", err, loginRecorder.Body.String())
	}
	if login.Data.Token == "" || login.Data.ExpiresAt.IsZero() {
		t.Fatalf("app auth login missing token/expiresAt: %+v", login.Data)
	}
	claims, err := server.tokens.Parse(login.Data.Token)
	if err != nil {
		t.Fatalf("parse app auth login jwt: %v", err)
	}
	if claims.TokenType != authjwt.TokenTypeApp || claims.SessionID == "" || claims.Username != "guest-alpha" || claims.TenantID != "app" {
		t.Fatalf("app auth login claims = %+v, want app guest-alpha session claims", claims)
	}
	if login.Data.Session.Username != "guest-alpha" || login.Data.Session.TenantID != "app" || login.Data.Session.UserID == "" {
		t.Fatalf("app login session = %+v, want app guest-alpha identity", login.Data.Session)
	}

	sessionRecorder := httptest.NewRecorder()
	sessionRequest := httptest.NewRequest(http.MethodGet, "/api/app/session/current", nil)
	sessionRequest.Header.Set("Authorization", "Bearer "+login.Data.Token)
	server.Router().ServeHTTP(sessionRecorder, sessionRequest)

	if sessionRecorder.Code != http.StatusOK {
		t.Fatalf("GET app current session status = %d body = %s", sessionRecorder.Code, sessionRecorder.Body.String())
	}
	var session appSessionTestPayload
	if err := json.Unmarshal(sessionRecorder.Body.Bytes(), &session); err != nil {
		t.Fatalf("decode app current session: %v body = %s", err, sessionRecorder.Body.String())
	}
	if session.Data.Username != "guest-alpha" || session.Data.TenantID != "app" || session.Data.SessionID != claims.SessionID {
		t.Fatalf("app current session = %+v, want issued app session", session.Data)
	}
}

func TestAppAuthLoginWithConfiguredWechatProviderUsesIdentityResolver(t *testing.T) {
	var captured AppIdentityResolveInput
	server := newTestServer(ServerOptions{
		Capabilities: []capability.Manifest{configuredWechatAuthProviderManifest()},
		AppIdentityResolver: appIdentityResolverFunc(func(ctx context.Context, input AppIdentityResolveInput) (AppIdentity, error) {
			captured = input
			return AppIdentity{
				Username:             "guest-wechat-06005",
				ProviderSubject:      "server_openid_store_06005",
				ProviderUnionSubject: "server_unionid_store_06005",
			}, nil
		}),
	})
	loginBody := bytes.NewBufferString(`{"provider":"wechat","code":"wx-code"}`)
	loginRecorder := httptest.NewRecorder()
	loginRequest := httptest.NewRequest(http.MethodPost, "/api/app/auth/login", loginBody)
	loginRequest.Header.Set("Content-Type", "application/json")

	server.Router().ServeHTTP(loginRecorder, loginRequest)

	if loginRecorder.Code != http.StatusOK {
		t.Fatalf("POST app auth login with wechat status = %d body = %s", loginRecorder.Code, loginRecorder.Body.String())
	}
	if captured.Provider.ID != "wechat" || captured.Provider.Kind != "wechat" || captured.Code != "wx-code" {
		t.Fatalf("captured resolver input = %+v, want wechat provider and code", captured)
	}
	if strings.Contains(loginRecorder.Body.String(), "server_openid_store_06005") || strings.Contains(loginRecorder.Body.String(), "server_unionid_store_06005") {
		t.Fatalf("app auth login response leaked provider subject: %s", loginRecorder.Body.String())
	}
	var login appLoginTestPayload
	if err := json.Unmarshal(loginRecorder.Body.Bytes(), &login); err != nil {
		t.Fatalf("decode app auth login: %v body = %s", err, loginRecorder.Body.String())
	}
	if login.Data.Session.Username != "guest-wechat-06005" || login.Data.Session.TenantID != appTenant {
		t.Fatalf("app login session = %+v, want resolver username in app tenant", login.Data.Session)
	}
	claims, err := server.tokens.Parse(login.Data.Token)
	if err != nil {
		t.Fatalf("parse app auth login jwt: %v", err)
	}
	if claims.TokenType != authjwt.TokenTypeApp || claims.Username != "guest-wechat-06005" || claims.TenantID != appTenant {
		t.Fatalf("app auth login claims = %+v, want resolver-backed app session", claims)
	}

	auditRecorder := httptest.NewRecorder()
	auditRequest := httptest.NewRequest(http.MethodGet, "/api/admin/resources/audit-logs", nil)
	server.Router().ServeHTTP(auditRecorder, auditRequest)
	if auditRecorder.Code != http.StatusOK {
		t.Fatalf("GET audit-logs status = %d body = %s", auditRecorder.Code, auditRecorder.Body.String())
	}
	if strings.Contains(auditRecorder.Body.String(), "server_openid_store_06005") || strings.Contains(auditRecorder.Body.String(), "server_unionid_store_06005") {
		t.Fatalf("audit response leaked provider subject: %s", auditRecorder.Body.String())
	}
	if !strings.Contains(auditRecorder.Body.String(), `"provider":"wechat"`) {
		t.Fatalf("audit response = %s, want provider marker", auditRecorder.Body.String())
	}
}

func TestAppAuthLoginRestoresStoredProviderBinding(t *testing.T) {
	rawSubject := "server-openid-store-06005"
	rawUnionSubject := "server-unionid-store-06005"
	callCount := 0
	server := newTestServer(ServerOptions{
		Capabilities: configuredWechatPlatformManifests(t),
		AppIdentityResolver: appIdentityResolverFunc(func(ctx context.Context, input AppIdentityResolveInput) (AppIdentity, error) {
			callCount++
			username := "guest-wechat-first"
			if callCount > 1 {
				username = "guest-wechat-second"
			}
			return AppIdentity{
				Username:             username,
				ProviderSubject:      rawSubject,
				ProviderUnionSubject: rawUnionSubject,
			}, nil
		}),
	})

	first := appWechatLoginForTest(t, server, "wx-code-1")
	second := appWechatLoginForTest(t, server, "wx-code-2")

	if first.Data.Session.Username != "guest-wechat-first" {
		t.Fatalf("first app login username = %q, want initial resolver username", first.Data.Session.Username)
	}
	if second.Data.Session.Username != "guest-wechat-first" {
		t.Fatalf("second app login username = %q, want stored binding username", second.Data.Session.Username)
	}
	records, err := server.resources.List("app-identities")
	if err != nil {
		t.Fatalf("List(app-identities) error = %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("app identity binding count = %d, want 1: %+v", len(records), records)
	}
	record := records[0]
	if record.Code == "" || record.Values["providerSubjectHash"] == "" || record.Values["maskedSubject"] == "" {
		t.Fatalf("app identity binding = %+v, want code, subject hash and masked subject", record)
	}
	if record.Values["appUsername"] != "guest-wechat-first" || record.Values["provider"] != "wechat" || record.Values["providerKind"] != "wechat" {
		t.Fatalf("app identity binding values = %+v, want wechat binding for first username", record.Values)
	}
	encoded, err := json.Marshal(records)
	if err != nil {
		t.Fatalf("marshal app identity records: %v", err)
	}
	if strings.Contains(string(encoded), rawSubject) || strings.Contains(string(encoded), rawUnionSubject) {
		t.Fatalf("app identity binding leaked raw provider subject: %s", string(encoded))
	}
}

func TestAppAuthLoginCreatesSeparateBindingsForDifferentProviderSubjects(t *testing.T) {
	server := newTestServer(ServerOptions{
		Capabilities: configuredWechatPlatformManifests(t),
		AppIdentityResolver: appIdentityResolverFunc(func(ctx context.Context, input AppIdentityResolveInput) (AppIdentity, error) {
			if input.Code == "wx-alpha" {
				return AppIdentity{Username: "guest-alpha", ProviderSubject: "server-openid-alpha"}, nil
			}
			return AppIdentity{Username: "guest-beta", ProviderSubject: "server-openid-beta"}, nil
		}),
	})

	alpha := appWechatLoginForTest(t, server, "wx-alpha")
	beta := appWechatLoginForTest(t, server, "wx-beta")

	if alpha.Data.Session.Username != "guest-alpha" || beta.Data.Session.Username != "guest-beta" {
		t.Fatalf("app usernames = %q/%q, want separate provider bindings", alpha.Data.Session.Username, beta.Data.Session.Username)
	}
	records, err := server.resources.List("app-identities")
	if err != nil {
		t.Fatalf("List(app-identities) error = %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("app identity binding count = %d, want 2: %+v", len(records), records)
	}
	if records[0].Values["providerSubjectHash"] == records[1].Values["providerSubjectHash"] {
		t.Fatalf("different provider subjects produced same binding hash: %+v", records)
	}
}

func TestAppAuthLoginRejectsWechatIdentityWithoutProviderSubject(t *testing.T) {
	server := newTestServer(ServerOptions{
		Capabilities: configuredWechatPlatformManifests(t),
		AppIdentityResolver: appIdentityResolverFunc(func(ctx context.Context, input AppIdentityResolveInput) (AppIdentity, error) {
			return AppIdentity{Username: "guest-wechat-no-subject"}, nil
		}),
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/app/auth/login", bytes.NewBufferString(`{"provider":"wechat","code":"wx-code"}`))
	request.Header.Set("Content-Type", "application/json")

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("POST app auth login without provider subject status = %d body = %s, want 401", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "APP_AUTH_IDENTITY_INVALID") {
		t.Fatalf("POST app auth login without provider subject body = %s, want invalid identity error", recorder.Body.String())
	}
}

func TestAppAuthLoginRejectsUnconfiguredProvider(t *testing.T) {
	server := newTestServer(ServerOptions{Capabilities: []capability.Manifest{authProviderTestManifest()}})
	loginBody := bytes.NewBufferString(`{"provider":"wechat","code":"wx-code"}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/app/auth/login", loginBody)
	request.Header.Set("Content-Type", "application/json")

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("POST app auth login with unconfigured provider status = %d body = %s, want 400", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "APP_AUTH_PROVIDER_NOT_CONFIGURED") {
		t.Fatalf("POST app auth login with unconfigured provider body = %s, want provider not configured error", recorder.Body.String())
	}
}

func TestAppAuthLoginRejectsMissingWechatCode(t *testing.T) {
	server := newTestServer(ServerOptions{
		Capabilities: []capability.Manifest{configuredWechatAuthProviderManifest()},
		AppIdentityResolver: appIdentityResolverFunc(func(ctx context.Context, input AppIdentityResolveInput) (AppIdentity, error) {
			t.Fatalf("resolver should not be called without wechat code")
			return AppIdentity{}, nil
		}),
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/app/auth/login", bytes.NewBufferString(`{"provider":"wechat"}`))
	request.Header.Set("Content-Type", "application/json")

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("POST app auth login without wechat code status = %d body = %s, want 400", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "APP_AUTH_CODE_REQUIRED") {
		t.Fatalf("POST app auth login without wechat code body = %s, want code required error", recorder.Body.String())
	}
}

func TestAppAuthLoginReportsResolverErrors(t *testing.T) {
	server := newTestServer(ServerOptions{
		Capabilities: []capability.Manifest{configuredWechatAuthProviderManifest()},
		AppIdentityResolver: appIdentityResolverFunc(func(ctx context.Context, input AppIdentityResolveInput) (AppIdentity, error) {
			return AppIdentity{}, errors.New("wechat exchange unavailable")
		}),
	})
	loginBody := bytes.NewBufferString(`{"provider":"wechat","code":"wx-code"}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/app/auth/login", loginBody)
	request.Header.Set("Content-Type", "application/json")

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("POST app auth login resolver error status = %d body = %s, want 502", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "APP_AUTH_PROVIDER_RESOLVE_FAILED") {
		t.Fatalf("POST app auth login resolver error body = %s, want stable resolver error", recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "wechat exchange unavailable") {
		t.Fatalf("POST app auth login resolver error leaked adapter detail: %s", recorder.Body.String())
	}
}

func TestAppRoutesRequireManifestDeclaration(t *testing.T) {
	server := newTestServer(ServerOptions{Capabilities: []capability.Manifest{{ID: "no-app-routes"}}})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/app/auth/login", bytes.NewBufferString(`{"username":"ghost"}`))
	request.Header.Set("Content-Type", "application/json")

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("POST undeclared app login status = %d body = %s, want 404", recorder.Code, recorder.Body.String())
	}
}

func TestDeclaredAppRouteWithoutHandlerReportsConfigurationError(t *testing.T) {
	server := newTestServer(ServerOptions{
		Capabilities: []capability.Manifest{
			{
				ID: "feedback",
				App: capability.AppSurface{Routes: []capability.AppRoute{
					{
						Method:      http.MethodGet,
						Path:        "/api/app/feedback/health",
						Auth:        capability.AppRouteAuthPublic,
						Description: capability.Text("反馈健康检查。", "Feedback health check."),
					},
				}},
			},
		},
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/app/feedback/health", nil)

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotImplemented {
		t.Fatalf("GET declared app route without handler status = %d body = %s, want 501", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "APP_ROUTE_HANDLER_NOT_CONFIGURED") {
		t.Fatalf("GET declared app route without handler body = %s, want configuration error", recorder.Body.String())
	}
}

func TestAppRouteRegistrationUsesManifestSessionAndPermissionPolicy(t *testing.T) {
	server := newTestServer(ServerOptions{
		Capabilities: []capability.Manifest{
			authProviderTestManifest(),
			{
				ID: "orders",
				App: capability.AppSurface{Routes: []capability.AppRoute{
					{
						Method:      http.MethodGet,
						Path:        "/api/app/orders",
						Auth:        capability.AppRouteAuthSession,
						Permission:  "app:orders:read",
						Description: capability.Text("读取订单。", "Read orders."),
					},
				}},
			},
		},
		AppRoutes: []AppRouteRegistration{
			{
				Method: http.MethodGet,
				Path:   "/api/app/orders",
				Handler: func(ctx *gin.Context) {
					appSession, ok := AppSessionFromContext(ctx)
					if !ok {
						ctx.JSON(http.StatusInternalServerError, Response[gin.H]{
							Error: &ErrorBody{Code: "TEST_APP_SESSION_MISSING", Message: "app session missing"},
						})
						return
					}
					ctx.JSON(http.StatusOK, Response[gin.H]{
						Data: gin.H{"user": appSession.Username, "tenant": appTenant},
					})
				},
			},
		},
		Authorizer: allowAppPermissionAuthorizer{
			user:       "app:buyer",
			tenant:     appTenant,
			permission: "app:orders:read",
			action:     "read",
		},
	})
	login := appLoginForTest(t, server, "buyer")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/app/orders", nil)
	request.Header.Set("Authorization", "Bearer "+login.Data.Token)
	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("GET app orders status = %d body = %s, want 200", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"user":"buyer"`) || !strings.Contains(recorder.Body.String(), `"tenant":"app"`) {
		t.Fatalf("GET app orders body = %s, want app session data", recorder.Body.String())
	}
}

func TestAppRouteRegistrationRejectsMissingAppPermission(t *testing.T) {
	server := newTestServer(ServerOptions{
		Capabilities: []capability.Manifest{
			authProviderTestManifest(),
			{
				ID: "orders",
				App: capability.AppSurface{Routes: []capability.AppRoute{
					{
						Method:      http.MethodGet,
						Path:        "/api/app/orders",
						Auth:        capability.AppRouteAuthSession,
						Permission:  "app:orders:read",
						Description: capability.Text("读取订单。", "Read orders."),
					},
				}},
			},
		},
		AppRoutes: []AppRouteRegistration{
			{
				Method: http.MethodGet,
				Path:   "/api/app/orders",
				Handler: func(ctx *gin.Context) {
					ctx.JSON(http.StatusOK, Response[gin.H]{Data: gin.H{"unexpected": true}})
				},
			},
		},
		Authorizer: denyAllAuthorizer{},
	})
	login := appLoginForTest(t, server, "buyer")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/app/orders", nil)
	request.Header.Set("Authorization", "Bearer "+login.Data.Token)
	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("GET app orders without permission status = %d body = %s, want 403", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "unexpected") {
		t.Fatalf("GET app orders without permission reached handler: %s", recorder.Body.String())
	}
}

func TestAppRouteRegistrationIgnoresUndeclaredHandlers(t *testing.T) {
	server := newTestServer(ServerOptions{
		Capabilities: []capability.Manifest{authProviderTestManifest()},
		AppRoutes: []AppRouteRegistration{
			{
				Method: http.MethodGet,
				Path:   "/api/app/hidden",
				Handler: func(ctx *gin.Context) {
					ctx.JSON(http.StatusOK, Response[gin.H]{Data: gin.H{"unexpected": true}})
				},
			},
		},
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/app/hidden", nil)

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("GET undeclared handler status = %d body = %s, want 404", recorder.Code, recorder.Body.String())
	}
}

func TestAppSessionRejectsAdminJWTAndAPIToken(t *testing.T) {
	server := newTestServer(ServerOptions{Capabilities: capabilitiesFromConfigForTest(t, []string{
		"tenant", "identity", "session", "rbac", "menu", "api-resource", "dictionary", "parameter", "audit", "admin-shell", "system-admin",
	})})
	adminLogin := loginForTest(t, server, "admin")
	apiToken, _ := createAdminAPITokenForTest(t, server, "admin:tenant:read")

	for _, tt := range []struct {
		name  string
		token string
	}{
		{name: "admin jwt", token: adminLogin.Data.Token},
		{name: "admin api token", token: apiToken},
	} {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/api/app/session/current", nil)
			request.Header.Set("Authorization", "Bearer "+tt.token)
			server.Router().ServeHTTP(recorder, request)

			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("GET app current session with %s status = %d body = %s, want 401", tt.name, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestAppPhoneVerificationAndBindingUseMaskedRecords(t *testing.T) {
	server := newTestServer(ServerOptions{Capabilities: capabilitiesFromConfigForTest(t, []string{
		"dictionary", "tenant", "identity", "session", "rbac", "audit", "app-phone",
	})})
	login := appLoginForTest(t, server, "guest-alpha")
	rawPhone := "13800138000"

	verificationRecorder := httptest.NewRecorder()
	verificationRequest := httptest.NewRequest(http.MethodPost, "/api/app/identity/phone-verifications", bytes.NewBufferString(`{"phone":"`+rawPhone+`","purpose":"bind"}`))
	verificationRequest.Header.Set("Content-Type", "application/json")
	verificationRequest.Header.Set("Authorization", "Bearer "+login.Data.Token)
	server.Router().ServeHTTP(verificationRecorder, verificationRequest)

	if verificationRecorder.Code != http.StatusCreated {
		t.Fatalf("POST app phone verification status = %d body = %s, want 201", verificationRecorder.Code, verificationRecorder.Body.String())
	}
	if strings.Contains(verificationRecorder.Body.String(), rawPhone) {
		t.Fatalf("phone verification response leaked raw phone: %s", verificationRecorder.Body.String())
	}
	var verification appPhoneVerificationTestPayload
	if err := json.Unmarshal(verificationRecorder.Body.Bytes(), &verification); err != nil {
		t.Fatalf("decode app phone verification: %v body = %s", err, verificationRecorder.Body.String())
	}
	if verification.Data.DebugCode == "" || verification.Data.PhoneHash != "" || verification.Data.MaskedPhone != "138****8000" || verification.Data.Purpose != "bind" || verification.Data.ExpiresAt.IsZero() {
		t.Fatalf("phone verification payload = %+v, want masked phone, bind purpose, debug code and expiry without hash", verification.Data)
	}

	bindingRecorder := httptest.NewRecorder()
	bindingRequest := httptest.NewRequest(http.MethodPost, "/api/app/identity/phone-bindings", bytes.NewBufferString(`{"phone":"`+rawPhone+`","code":"`+verification.Data.DebugCode+`"}`))
	bindingRequest.Header.Set("Content-Type", "application/json")
	bindingRequest.Header.Set("Authorization", "Bearer "+login.Data.Token)
	server.Router().ServeHTTP(bindingRecorder, bindingRequest)

	if bindingRecorder.Code != http.StatusCreated {
		t.Fatalf("POST app phone binding status = %d body = %s, want 201", bindingRecorder.Code, bindingRecorder.Body.String())
	}
	if strings.Contains(bindingRecorder.Body.String(), rawPhone) || strings.Contains(bindingRecorder.Body.String(), verification.Data.DebugCode) {
		t.Fatalf("phone binding response leaked raw phone or verification code: %s", bindingRecorder.Body.String())
	}
	var binding appPhoneBindingTestPayload
	if err := json.Unmarshal(bindingRecorder.Body.Bytes(), &binding); err != nil {
		t.Fatalf("decode app phone binding: %v body = %s", err, bindingRecorder.Body.String())
	}
	if binding.Data.AppUsername != "guest-alpha" || binding.Data.PhoneHash != "" || binding.Data.MaskedPhone != verification.Data.MaskedPhone || binding.Data.BoundAt.IsZero() {
		t.Fatalf("phone binding payload = %+v, want guest-alpha masked binding", binding.Data)
	}

	for _, resource := range []string{"app-phone-verifications", "app-phone-bindings"} {
		records, err := server.resources.List(resource)
		if err != nil {
			t.Fatalf("List(%s) error = %v", resource, err)
		}
		encoded, err := json.Marshal(records)
		if err != nil {
			t.Fatalf("marshal %s records: %v", resource, err)
		}
		if strings.Contains(string(encoded), rawPhone) || strings.Contains(string(encoded), verification.Data.DebugCode) {
			t.Fatalf("%s leaked raw phone or code: %s", resource, string(encoded))
		}
	}
}

func TestAppPhoneBindingRejectsDuplicatePhoneHash(t *testing.T) {
	server := newTestServer(ServerOptions{Capabilities: capabilitiesFromConfigForTest(t, []string{
		"dictionary", "tenant", "identity", "session", "rbac", "audit", "app-phone",
	})})
	rawPhone := "13800138000"
	alpha := appLoginForTest(t, server, "guest-alpha")
	alphaVerification := createAppPhoneVerificationForTest(t, server, alpha.Data.Token, rawPhone)
	createAppPhoneBindingForTest(t, server, alpha.Data.Token, rawPhone, alphaVerification.Data.DebugCode)

	beta := appLoginForTest(t, server, "guest-beta")
	betaVerification := createAppPhoneVerificationForTest(t, server, beta.Data.Token, rawPhone)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/app/identity/phone-bindings", bytes.NewBufferString(`{"phone":"`+rawPhone+`","code":"`+betaVerification.Data.DebugCode+`"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+beta.Data.Token)
	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("POST duplicate app phone binding status = %d body = %s, want 409", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "APP_PHONE_ALREADY_BOUND") {
		t.Fatalf("POST duplicate app phone binding body = %s, want duplicate error", recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), rawPhone) || strings.Contains(recorder.Body.String(), betaVerification.Data.DebugCode) {
		t.Fatalf("duplicate app phone binding leaked raw phone or code: %s", recorder.Body.String())
	}
}

func TestAppPhoneVerificationRateLimitUsesUserAndPhoneWindow(t *testing.T) {
	now := time.Date(2026, 7, 6, 9, 0, 0, 0, time.UTC)
	senderCalls := 0
	server := newTestServer(ServerOptions{
		Capabilities: capabilitiesFromConfigForTest(t, []string{
			"dictionary", "tenant", "identity", "session", "rbac", "audit", "app-phone",
		}),
		Now: func() time.Time {
			return now
		},
		PhoneVerificationSender: phoneVerificationSenderTestStub{
			kind: "sms-vendor",
			send: func(context.Context, string, string, string) error {
				senderCalls++
				return nil
			},
		},
	})
	login := appLoginForTest(t, server, "guest-alpha")
	rawPhone := "13800138000"

	for attempt := 0; attempt < 3; attempt++ {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/app/identity/phone-verifications", bytes.NewBufferString(`{"phone":"`+rawPhone+`","purpose":"bind"}`))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Authorization", "Bearer "+login.Data.Token)
		server.Router().ServeHTTP(recorder, request)
		if recorder.Code != http.StatusCreated {
			t.Fatalf("POST app phone verification attempt %d status = %d body = %s, want 201", attempt+1, recorder.Code, recorder.Body.String())
		}
	}

	limitedRecorder := httptest.NewRecorder()
	limitedRequest := httptest.NewRequest(http.MethodPost, "/api/app/identity/phone-verifications", bytes.NewBufferString(`{"phone":"`+rawPhone+`","purpose":"bind"}`))
	limitedRequest.Header.Set("Content-Type", "application/json")
	limitedRequest.Header.Set("Authorization", "Bearer "+login.Data.Token)
	server.Router().ServeHTTP(limitedRecorder, limitedRequest)

	if limitedRecorder.Code != http.StatusTooManyRequests {
		t.Fatalf("POST app phone verification over limit status = %d body = %s, want 429", limitedRecorder.Code, limitedRecorder.Body.String())
	}
	if !strings.Contains(limitedRecorder.Body.String(), "APP_PHONE_VERIFICATION_RATE_LIMITED") {
		t.Fatalf("POST app phone verification over limit body = %s, want rate limit error", limitedRecorder.Body.String())
	}
	if strings.Contains(limitedRecorder.Body.String(), rawPhone) {
		t.Fatalf("rate limit response leaked raw phone: %s", limitedRecorder.Body.String())
	}
	if senderCalls != appPhoneVerificationRateLimit {
		t.Fatalf("sender calls after rate limit = %d, want %d", senderCalls, appPhoneVerificationRateLimit)
	}

	now = now.Add(11 * time.Minute)
	nextRecorder := httptest.NewRecorder()
	nextRequest := httptest.NewRequest(http.MethodPost, "/api/app/identity/phone-verifications", bytes.NewBufferString(`{"phone":"`+rawPhone+`","purpose":"bind"}`))
	nextRequest.Header.Set("Content-Type", "application/json")
	nextRequest.Header.Set("Authorization", "Bearer "+login.Data.Token)
	server.Router().ServeHTTP(nextRecorder, nextRequest)

	if nextRecorder.Code != http.StatusCreated {
		t.Fatalf("POST app phone verification after window status = %d body = %s, want 201", nextRecorder.Code, nextRecorder.Body.String())
	}
	if senderCalls != appPhoneVerificationRateLimit+1 {
		t.Fatalf("sender calls after next window = %d, want %d", senderCalls, appPhoneVerificationRateLimit+1)
	}
}

func TestAppPhoneVerificationDoesNotCallSenderForRejectedInput(t *testing.T) {
	senderCalls := 0
	server := newTestServer(ServerOptions{
		Capabilities: capabilitiesFromConfigForTest(t, []string{
			"dictionary", "tenant", "identity", "session", "rbac", "audit", "app-phone",
		}),
		PhoneVerificationSender: phoneVerificationSenderTestStub{
			kind: "sms-vendor",
			send: func(context.Context, string, string, string) error {
				senderCalls++
				return nil
			},
		},
	})
	login := appLoginForTest(t, server, "guest-alpha")
	for _, body := range []string{
		`{"phone":"invalid","purpose":"bind"}`,
		`{"phone":"13800138000","purpose":"login"}`,
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/app/identity/phone-verifications", bytes.NewBufferString(body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Authorization", "Bearer "+login.Data.Token)
		server.Router().ServeHTTP(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("POST rejected app phone verification status = %d body = %s, want 400", recorder.Code, recorder.Body.String())
		}
	}
	if senderCalls != 0 {
		t.Fatalf("sender calls for rejected input = %d, want 0", senderCalls)
	}
}

func TestAppPhoneVerificationSenderFailureDoesNotPersistRecord(t *testing.T) {
	var deliveredPhone string
	var deliveredCode string
	server := newTestServer(ServerOptions{
		Capabilities: capabilitiesFromConfigForTest(t, []string{
			"dictionary", "tenant", "identity", "session", "rbac", "audit", "app-phone",
		}),
		PhoneVerificationSender: phoneVerificationSenderTestStub{
			kind: "sms-vendor",
			send: func(_ context.Context, phone string, _ string, code string) error {
				deliveredPhone = phone
				deliveredCode = code
				return errors.New("delivery unavailable")
			},
		},
	})
	login := appLoginForTest(t, server, "guest-alpha")
	rawPhone := "13800138000"

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/app/identity/phone-verifications", bytes.NewBufferString(`{"phone":"`+rawPhone+`","purpose":"bind"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+login.Data.Token)
	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("POST failed app phone delivery status = %d body = %s, want 502", recorder.Code, recorder.Body.String())
	}
	if deliveredPhone != rawPhone || deliveredCode == "" {
		t.Fatalf("sender received phone=%q code=%q, want normalized phone and generated code", deliveredPhone, deliveredCode)
	}
	if strings.Contains(recorder.Body.String(), rawPhone) || strings.Contains(recorder.Body.String(), deliveredCode) {
		t.Fatalf("delivery failure response leaked phone or code: %s", recorder.Body.String())
	}
	records, err := server.resources.List(appPhoneVerificationsResource)
	if err != nil {
		t.Fatalf("List(app-phone-verifications) error = %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("app phone verification records = %+v, want none after delivery failure", records)
	}
}

func TestAppPhoneVerificationWithProviderStoresOnlyVersionedDigestsAndOmitsDebugCode(t *testing.T) {
	var deliveredCode string
	server := newTestServer(ServerOptions{
		Capabilities: capabilitiesFromConfigForTest(t, []string{
			"dictionary", "tenant", "identity", "session", "rbac", "audit", "app-phone",
		}),
		PhoneVerificationSender: phoneVerificationSenderTestStub{
			kind: "sms-vendor",
			send: func(_ context.Context, _ string, _ string, code string) error {
				deliveredCode = code
				return nil
			},
		},
	})
	login := appLoginForTest(t, server, "guest-alpha")
	rawPhone := "138 0013-8000"

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/app/identity/phone-verifications", bytes.NewBufferString(`{"phone":"`+rawPhone+`","purpose":"bind"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+login.Data.Token)
	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("POST provider app phone verification status = %d body = %s, want 201", recorder.Code, recorder.Body.String())
	}
	if deliveredCode == "" || strings.Contains(recorder.Body.String(), deliveredCode) || strings.Contains(recorder.Body.String(), "debugCode") {
		t.Fatalf("provider response leaked debug code field/value: %s", recorder.Body.String())
	}
	records, err := server.resources.List(appPhoneVerificationsResource)
	if err != nil || len(records) != 1 {
		t.Fatalf("List(app-phone-verifications) records=%+v error=%v, want one", records, err)
	}
	values := records[0].Values
	if values["maskedPhone"] != "138****8000" || !strings.HasPrefix(values["phoneHash"], "v1:hmac-sha256:phone:") || !strings.HasPrefix(values["codeHash"], "v1:hmac-sha256:code:") {
		t.Fatalf("stored phone verification values = %+v, want masked phone and versioned HMACs", values)
	}
	encoded, err := json.Marshal(records)
	if err != nil {
		t.Fatalf("marshal verification records: %v", err)
	}
	for _, forbidden := range []string{rawPhone, "13800138000", deliveredCode} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("stored verification leaked %q: %s", forbidden, encoded)
		}
	}
}

func TestAppPhoneVerificationDoesNotTrustMutableSenderKindForDebugDisclosure(t *testing.T) {
	sender := &mutablePhoneVerificationSenderTestStub{kind: "sms-vendor"}
	server := newTestServer(ServerOptions{
		Capabilities: capabilitiesFromConfigForTest(t, []string{
			"dictionary", "tenant", "identity", "session", "rbac", "audit", "app-phone",
		}),
		PhoneVerificationSender: sender,
		DebugCodeEnabled:        false,
	})
	login := appLoginForTest(t, server, "guest-alpha")
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/app/identity/phone-verifications", bytes.NewBufferString(`{"phone":"13800138000","purpose":"bind"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+login.Data.Token)
	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("POST mutable provider verification status=%d body=%s, want 201", recorder.Code, recorder.Body.String())
	}
	if sender.Kind() != PhoneVerificationProviderDebug {
		t.Fatalf("mutable sender kind=%q, want debug after Send", sender.Kind())
	}
	if strings.Contains(recorder.Body.String(), "debugCode") {
		t.Fatalf("mutable production sender exposed debugCode: %s", recorder.Body.String())
	}
}

func TestAppPhoneVerificationFailsClosedWhenProtectionDependencyIsMissing(t *testing.T) {
	capabilities := capabilitiesFromConfigForTest(t, []string{
		"dictionary", "tenant", "identity", "session", "rbac", "audit", "app-phone",
	})
	tests := []struct {
		name    string
		options ServerOptions
	}{
		{
			name: "protector",
			options: ServerOptions{
				Capabilities:            capabilities,
				PhoneVerificationSender: NewDebugPhoneVerificationSender(),
				DebugCodeEnabled:        true,
			},
		},
		{
			name: "sender",
			options: ServerOptions{
				Capabilities:   capabilities,
				PhoneProtector: NewHMACPhoneProtector([]byte(strings.Repeat("p", 32)), []byte(strings.Repeat("c", 32))),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.options.AllowInsecureHeaderAuth = true
			server := New(tt.options)
			login := appLoginForTest(t, server, "guest-alpha")
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/api/app/identity/phone-verifications", bytes.NewBufferString(`{"phone":"13800138000","purpose":"bind"}`))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Authorization", "Bearer "+login.Data.Token)
			server.Router().ServeHTTP(recorder, request)

			if recorder.Code != http.StatusServiceUnavailable || !strings.Contains(recorder.Body.String(), "APP_PHONE_VERIFICATION_UNAVAILABLE") {
				t.Fatalf("POST missing %s status=%d body=%s, want fail-closed 503", tt.name, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestAppPhoneVerificationProtectionFailureDoesNotCallSenderOrPersist(t *testing.T) {
	senderCalls := 0
	server := newTestServer(ServerOptions{
		Capabilities: capabilitiesFromConfigForTest(t, []string{
			"dictionary", "tenant", "identity", "session", "rbac", "audit", "app-phone",
		}),
		PhoneProtector: phoneProtectorTestStub{
			phoneDigest: func(string) (string, error) { return "", errors.New("protection unavailable") },
			codeDigest:  func(string, string, string) (string, error) { return "", errors.New("unexpected") },
		},
		PhoneVerificationSender: phoneVerificationSenderTestStub{
			kind: "sms-vendor",
			send: func(context.Context, string, string, string) error {
				senderCalls++
				return nil
			},
		},
	})
	login := appLoginForTest(t, server, "guest-alpha")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/app/identity/phone-verifications", bytes.NewBufferString(`{"phone":"13800138000","purpose":"bind"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+login.Data.Token)
	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable || senderCalls != 0 {
		t.Fatalf("protection failure status=%d senderCalls=%d body=%s, want 503 and zero calls", recorder.Code, senderCalls, recorder.Body.String())
	}
	records, err := server.resources.List(appPhoneVerificationsResource)
	if err != nil || len(records) != 0 {
		t.Fatalf("records=%+v error=%v, want none after protection failure", records, err)
	}
}

func TestAppPhoneRoutesRequireEnabledCapability(t *testing.T) {
	server := newTestServer(ServerOptions{Capabilities: capabilitiesFromConfigForTest(t, []string{"dictionary", "tenant", "identity", "session", "rbac", "audit"})})
	login := appLoginForTest(t, server, "guest-alpha")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/app/identity/phone-verifications", bytes.NewBufferString(`{"phone":"13800138000"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+login.Data.Token)
	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("POST disabled app phone verification status = %d body = %s, want 404", recorder.Code, recorder.Body.String())
	}
}

func TestAppAuthLogoutRevokesOnlyAppSession(t *testing.T) {
	server := newTestServer(ServerOptions{Capabilities: []capability.Manifest{authProviderTestManifest()}})
	loginBody := bytes.NewBufferString(`{"username":"guest-alpha"}`)
	loginRecorder := httptest.NewRecorder()
	loginRequest := httptest.NewRequest(http.MethodPost, "/api/app/auth/login", loginBody)
	loginRequest.Header.Set("Content-Type", "application/json")
	server.Router().ServeHTTP(loginRecorder, loginRequest)
	if loginRecorder.Code != http.StatusOK {
		t.Fatalf("POST app auth login status = %d body = %s", loginRecorder.Code, loginRecorder.Body.String())
	}
	var login appLoginTestPayload
	if err := json.Unmarshal(loginRecorder.Body.Bytes(), &login); err != nil {
		t.Fatalf("decode app auth login: %v body = %s", err, loginRecorder.Body.String())
	}

	logoutRecorder := httptest.NewRecorder()
	logoutRequest := httptest.NewRequest(http.MethodPost, "/api/app/auth/logout", nil)
	logoutRequest.Header.Set("Authorization", "Bearer "+login.Data.Token)
	server.Router().ServeHTTP(logoutRecorder, logoutRequest)

	if logoutRecorder.Code != http.StatusOK {
		t.Fatalf("POST app auth logout status = %d body = %s", logoutRecorder.Code, logoutRecorder.Body.String())
	}

	sessionRecorder := httptest.NewRecorder()
	sessionRequest := httptest.NewRequest(http.MethodGet, "/api/app/session/current", nil)
	sessionRequest.Header.Set("Authorization", "Bearer "+login.Data.Token)
	server.Router().ServeHTTP(sessionRecorder, sessionRequest)

	if sessionRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("GET app current session after logout status = %d body = %s, want 401", sessionRecorder.Code, sessionRecorder.Body.String())
	}
}

func TestAppAuthLogoutRepositoryErrorReturnsInternalAuthError(t *testing.T) {
	now := time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC)
	repository := newControllableSessionRepository()
	sessions, err := session.NewRepositoryBackedStore(session.Options{TTL: time.Hour, Now: func() time.Time { return now }}, repository)
	if err != nil {
		t.Fatalf("NewRepositoryBackedStore() error = %v", err)
	}
	server := newTestServer(ServerOptions{
		Capabilities: []capability.Manifest{authProviderTestManifest()},
		Sessions:     sessions,
		Now:          func() time.Time { return now },
	})
	login := appLoginForTest(t, server, "guest-alpha")
	repository.revokeErr = errors.New("revoke unavailable")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/app/auth/logout", nil)
	request.Header.Set("Authorization", "Bearer "+login.Data.Token)
	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("POST app auth logout repository error status = %d body = %s, want 500", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"code":"APP_AUTH_SESSION_REVOKE_FAILED"`) {
		t.Fatalf("POST app auth logout repository error body = %s, want APP_AUTH_SESSION_REVOKE_FAILED", recorder.Body.String())
	}
	if _, ok := sessions.Resolve(loginSessionIDForTest(t, server, login.Data.Token)); !ok {
		t.Fatal("app session became inactive after failed repository revoke")
	}
}

func TestAppAuthLogoutResolveRepositoryErrorReturnsInternalAuthError(t *testing.T) {
	server, repository, token := newAppSessionWriteTestServer(t)
	repository.resolveErr = errors.New("resolve unavailable")

	recorder := performBearerRequest(server, http.MethodPost, "/api/app/auth/logout", token)

	assertAuthErrorResponse(t, recorder, http.StatusInternalServerError, "APP_AUTH_SESSION_REVOKE_FAILED")
}

func TestAppAuthLogoutResolveRepositoryErrorDoesNotInvokeOverrideHandler(t *testing.T) {
	now := time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC)
	repository := newControllableSessionRepository()
	sessions, err := session.NewRepositoryBackedStore(session.Options{TTL: time.Hour, Now: func() time.Time { return now }}, repository)
	if err != nil {
		t.Fatalf("NewRepositoryBackedStore() error = %v", err)
	}
	overrideCalls := 0
	server := newTestServer(ServerOptions{
		Capabilities: []capability.Manifest{authProviderTestManifest()},
		Sessions:     sessions,
		Now:          func() time.Time { return now },
		AppRoutes: []AppRouteRegistration{{
			Method: http.MethodPost,
			Path:   "/api/app/auth/logout",
			Handler: func(ctx *gin.Context) {
				overrideCalls++
				ctx.JSON(http.StatusAccepted, Response[gin.H]{Data: gin.H{"override": true}})
			},
		}},
	})
	token := appLoginForTest(t, server, "guest-alpha").Data.Token
	repository.resolveErr = errors.New("resolve unavailable")

	recorder := performBearerRequest(server, http.MethodPost, "/api/app/auth/logout", token)

	if overrideCalls != 0 {
		t.Fatalf("App logout override handler calls = %d, want 0 on repository Resolve error", overrideCalls)
	}
	assertAuthErrorResponse(t, recorder, http.StatusInternalServerError, "APP_AUTH_SESSION_REVOKE_FAILED")
}

func TestAppAuthLogoutKeepsNotFoundResolveUnauthorized(t *testing.T) {
	server, repository, token := newAppSessionWriteTestServer(t)
	delete(repository.sessions, session.DigestToken(loginSessionIDForTest(t, server, token)))

	recorder := performBearerRequest(server, http.MethodPost, "/api/app/auth/logout", token)

	assertAuthErrorResponse(t, recorder, http.StatusUnauthorized, "AUTH_UNAUTHORIZED")
}

func TestAppAuthLogoutKeepsInvalidJWTAndWrongTokenTypeUnauthorized(t *testing.T) {
	server := newTestServer(ServerOptions{Capabilities: []capability.Manifest{authProviderTestManifest()}})
	adminToken := loginForTest(t, server, "ops").Data.Token

	for name, token := range map[string]string{
		"invalid JWT":      "not-a-jwt",
		"admin token type": adminToken,
	} {
		t.Run(name, func(t *testing.T) {
			recorder := performBearerRequest(server, http.MethodPost, "/api/app/auth/logout", token)

			assertAuthErrorResponse(t, recorder, http.StatusUnauthorized, "AUTH_UNAUTHORIZED")
		})
	}
}

func TestAuthLoginAndLogoutWriteAuditRecords(t *testing.T) {
	server := newTestServer(ServerOptions{Capabilities: []capability.Manifest{authProviderTestManifest()}})
	login := loginForTest(t, server, "ops")
	logoutRequest := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	logoutRequest.Header.Set("Authorization", "Bearer "+login.Data.Token)
	server.Router().ServeHTTP(httptest.NewRecorder(), logoutRequest)

	auditRecorder := httptest.NewRecorder()
	auditRequest := httptest.NewRequest(http.MethodGet, "/api/admin/resources/audit-logs", nil)
	server.Router().ServeHTTP(auditRecorder, auditRequest)

	if auditRecorder.Code != http.StatusOK {
		t.Fatalf("GET audit-logs status = %d body = %s", auditRecorder.Code, auditRecorder.Body.String())
	}
	var payload adminResourceListTestPayload
	if err := json.Unmarshal(auditRecorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode audit: %v body = %s", err, auditRecorder.Body.String())
	}
	if !hasTestRecordAction(payload.Data.Items, "auth.login") {
		t.Fatalf("audit records missing auth.login: %+v", payload.Data.Items)
	}
	if !hasTestRecordAction(payload.Data.Items, "auth.logout") {
		t.Fatalf("audit records missing auth.logout: %+v", payload.Data.Items)
	}
}

func TestAuditCapabilityExposesAuditLogsResource(t *testing.T) {
	server := newTestServer(ServerOptions{Capabilities: capabilitiesFromConfigForTest(t, []string{"dictionary", "tenant", "identity", "rbac", "audit"})})

	auditLogsRecorder := httptest.NewRecorder()
	auditLogsRequest := httptest.NewRequest(http.MethodGet, "/api/admin/resources/audit-logs", nil)
	server.Router().ServeHTTP(auditLogsRecorder, auditLogsRequest)
	if auditLogsRecorder.Code != http.StatusOK {
		t.Fatalf("GET audit-logs status = %d body = %s", auditLogsRecorder.Code, auditLogsRecorder.Body.String())
	}
	var auditLogs adminResourceListTestPayload
	if err := json.Unmarshal(auditLogsRecorder.Body.Bytes(), &auditLogs); err != nil {
		t.Fatalf("decode audit-logs: %v body = %s", err, auditLogsRecorder.Body.String())
	}
	if auditLogs.Data.Resource != "audit-logs" || !hasTestRecordCode(auditLogs.Data.Items, "platform.bootstrap") {
		t.Fatalf("audit-logs payload = %+v, want canonical audit logs resource with bootstrap row", auditLogs.Data)
	}

	legacyRecorder := httptest.NewRecorder()
	legacyRequest := httptest.NewRequest(http.MethodGet, "/api/admin/resources/audit", nil)
	server.Router().ServeHTTP(legacyRecorder, legacyRequest)
	if legacyRecorder.Code != http.StatusNotFound {
		t.Fatalf("GET legacy audit status = %d body = %s, want 404", legacyRecorder.Code, legacyRecorder.Body.String())
	}
}

func TestAuthSessionExpires(t *testing.T) {
	now := time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC)
	server := newTestServer(ServerOptions{
		Capabilities: []capability.Manifest{authProviderTestManifest()},
		SessionTTL:   time.Hour,
		Now: func() time.Time {
			return now
		},
	})
	login := loginForTest(t, server, "ops")
	now = now.Add(2 * time.Hour)

	sessionRecorder := httptest.NewRecorder()
	sessionRequest := httptest.NewRequest(http.MethodGet, "/api/admin/session/current", nil)
	sessionRequest.Header.Set("Authorization", "Bearer "+login.Data.Token)
	server.Router().ServeHTTP(sessionRecorder, sessionRequest)

	if sessionRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("GET expired session status = %d body = %s, want 401", sessionRecorder.Code, sessionRecorder.Body.String())
	}
}

func TestCurrentSessionRejectsNonAdminJWT(t *testing.T) {
	server := newTestServer(ServerOptions{Capabilities: []capability.Manifest{authProviderTestManifest()}})
	issued, err := server.sessions.Issue("ops")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	token, _, err := server.tokens.Sign(authjwt.Subject{
		UserID:    "ops",
		TenantID:  platformTenant,
		Username:  "ops",
		SessionID: issued.Token,
		TokenType: authjwt.TokenTypeApp,
	}, time.Hour)
	if err != nil {
		t.Fatalf("Sign() error = %v", err)
	}

	sessionRecorder := httptest.NewRecorder()
	sessionRequest := httptest.NewRequest(http.MethodGet, "/api/admin/session/current", nil)
	sessionRequest.Header.Set("Authorization", "Bearer "+token)
	server.Router().ServeHTTP(sessionRecorder, sessionRequest)

	if sessionRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("GET current session with app jwt status = %d body = %s, want 401", sessionRecorder.Code, sessionRecorder.Body.String())
	}
}

func TestAuthLoginRejectsUnconfiguredProvider(t *testing.T) {
	server := newTestServer(ServerOptions{Capabilities: []capability.Manifest{authProviderTestManifest()}})
	loginBody := bytes.NewBufferString(`{"provider":"wechat","code":"wx-code"}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", loginBody)
	request.Header.Set("Content-Type", "application/json")

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("POST auth login with unconfigured provider status = %d body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestCurrentSessionEndpointReturnsRoleBackedPermissions(t *testing.T) {
	server := newTestServer(ServerOptions{Capabilities: []capability.Manifest{{ID: "tenant"}}})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/admin/session/current", nil)
	request.Header.Set("X-Platform-User", "ops")

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("GET current session status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	var payload currentSessionTestPayload
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode current session: %v body = %s", err, recorder.Body.String())
	}
	if payload.Data.User.Username != "ops" {
		t.Fatalf("session username = %q, want ops", payload.Data.User.Username)
	}
	if payload.Data.User.TenantCode != "platform" || payload.Data.User.OrgUnitCode != "platform-ops" || payload.Data.User.AreaCode != "110000" {
		t.Fatalf("session user scope = %+v, want platform/platform-ops/110000", payload.Data.User)
	}
	if !containsString(payload.Data.Roles, "operator") {
		t.Fatalf("session roles = %+v, want operator", payload.Data.Roles)
	}
	if !containsString(payload.Data.Permissions, "admin:tenant:read") {
		t.Fatalf("session permissions = %+v, want admin:tenant:read", payload.Data.Permissions)
	}
}

func TestCurrentSessionUsesPermissionExpansionCacheAndInvalidatesAfterRoleUpdate(t *testing.T) {
	cacheStore := cache.NewMeteredStore("memory", cache.NewMemoryStore(cache.MemoryStoreOptions{}))
	server := newTestServer(ServerOptions{Capabilities: []capability.Manifest{{ID: "tenant"}}, Cache: cacheStore})

	firstRecorder := httptest.NewRecorder()
	firstRequest := httptest.NewRequest(http.MethodGet, "/api/admin/session/current", nil)
	firstRequest.Header.Set("X-Platform-User", "ops")
	server.Router().ServeHTTP(firstRecorder, firstRequest)
	if firstRecorder.Code != http.StatusOK {
		t.Fatalf("first current session status = %d body = %s", firstRecorder.Code, firstRecorder.Body.String())
	}

	secondRecorder := httptest.NewRecorder()
	secondRequest := httptest.NewRequest(http.MethodGet, "/api/admin/session/current", nil)
	secondRequest.Header.Set("X-Platform-User", "ops")
	server.Router().ServeHTTP(secondRecorder, secondRequest)
	if secondRecorder.Code != http.StatusOK {
		t.Fatalf("second current session status = %d body = %s", secondRecorder.Code, secondRecorder.Body.String())
	}

	stats := cacheStore.Stats()
	if stats.Hits == 0 || stats.Misses == 0 {
		t.Fatalf("principal cache stats = %+v, want hit and miss after repeated current-session reads", stats)
	}

	updateBody := bytes.NewBufferString(`{"name":"Operator","status":"enabled","description":"Updated operator","values":{"groupCode":"operations","dataScope":"current_org","permissions":"admin:user:read"}}`)
	updateRecorder := httptest.NewRecorder()
	updateRequest := httptest.NewRequest(http.MethodPut, "/api/admin/resources/roles/role-operator", updateBody)
	updateRequest.Header.Set("Content-Type", "application/json")
	server.Router().ServeHTTP(updateRecorder, updateRequest)
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("PUT operator role status = %d body = %s", updateRecorder.Code, updateRecorder.Body.String())
	}

	afterUpdateRecorder := httptest.NewRecorder()
	afterUpdateRequest := httptest.NewRequest(http.MethodGet, "/api/admin/session/current", nil)
	afterUpdateRequest.Header.Set("X-Platform-User", "ops")
	server.Router().ServeHTTP(afterUpdateRecorder, afterUpdateRequest)
	if afterUpdateRecorder.Code != http.StatusOK {
		t.Fatalf("current session after role update status = %d body = %s", afterUpdateRecorder.Code, afterUpdateRecorder.Body.String())
	}
	var payload currentSessionTestPayload
	if err := json.Unmarshal(afterUpdateRecorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode current session after update: %v body = %s", err, afterUpdateRecorder.Body.String())
	}
	if !containsString(payload.Data.Permissions, "admin:user:read") || containsString(payload.Data.Permissions, "admin:tenant:read") {
		t.Fatalf("permissions after role update = %+v, want refreshed permissions", payload.Data.Permissions)
	}
}

func TestCacheStatsEndpointReturnsMeteredCacheStats(t *testing.T) {
	cacheStore := cache.NewMeteredStore("memory", cache.NewMemoryStore(cache.MemoryStoreOptions{}))
	server := newTestServer(ServerOptions{Capabilities: []capability.Manifest{{ID: "tenant"}}, Cache: cacheStore})

	for i := 0; i < 2; i++ {
		request := httptest.NewRequest(http.MethodGet, "/api/admin/session/current", nil)
		request.Header.Set("X-Platform-User", "ops")
		recorder := httptest.NewRecorder()
		server.Router().ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("current session status = %d body = %s", recorder.Code, recorder.Body.String())
		}
	}

	statsRecorder := httptest.NewRecorder()
	statsRequest := httptest.NewRequest(http.MethodGet, "/api/platform/cache/stats", nil)
	server.Router().ServeHTTP(statsRecorder, statsRequest)
	if statsRecorder.Code != http.StatusOK {
		t.Fatalf("cache stats status = %d body = %s", statsRecorder.Code, statsRecorder.Body.String())
	}
	var payload cacheStatsTestPayload
	if err := json.Unmarshal(statsRecorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode cache stats: %v body = %s", err, statsRecorder.Body.String())
	}
	if payload.Data.Driver != "memory" || payload.Data.Hits == 0 || payload.Data.Misses == 0 || payload.Data.Sets == 0 {
		t.Fatalf("cache stats = %+v, want memory stats with hits, misses and sets", payload.Data)
	}
}

func TestAdminMenusEndpointFiltersByCurrentUserPermissions(t *testing.T) {
	server := newTestServer(ServerOptions{Capabilities: []capability.Manifest{{ID: "tenant"}}})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/admin/menus", nil)
	request.Header.Set("X-Platform-User", "ops")

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("GET menus status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	var payload adminMenuListTestPayload
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode menus: %v body = %s", err, recorder.Body.String())
	}
	routes := make([]string, 0, len(payload.Data.Items))
	for _, item := range payload.Data.Items {
		routes = append(routes, item.Route)
	}
	if !containsString(routes, "/capabilities") || !containsString(routes, "/tenants") {
		t.Fatalf("filtered menu routes = %+v, want capabilities and tenants", routes)
	}
	if containsString(routes, "/users") || containsString(routes, "/roles") {
		t.Fatalf("filtered menu routes = %+v, want no user or role menus", routes)
	}
	for _, item := range payload.Data.Items {
		if item.Route == "/tenants" && item.Parent == "" {
			t.Fatalf("tenant menu parent is empty: %+v", item)
		}
		if item.Route == "/tenants" && (item.IsExternal || !item.CacheEnabled) {
			t.Fatalf("tenant menu external/cache mismatch: %+v", item)
		}
	}
}

func TestAdminMenusEndpointReflectsUpdatedRolePermissions(t *testing.T) {
	server := newTestServer(ServerOptions{
		Capabilities: []capability.Manifest{authProviderTestManifest(), {ID: "tenant"}},
		Cache:        cache.NewMemoryStore(cache.MemoryStoreOptions{}),
	})
	primeRecorder := httptest.NewRecorder()
	primeRequest := httptest.NewRequest(http.MethodGet, "/api/admin/menus", nil)
	primeRequest.Header.Set("X-Platform-User", "ops")
	server.Router().ServeHTTP(primeRecorder, primeRequest)
	if primeRecorder.Code != http.StatusOK {
		t.Fatalf("prime menus status = %d body = %s", primeRecorder.Code, primeRecorder.Body.String())
	}

	updateBody := bytes.NewBufferString(`{"name":"Operator","status":"enabled","description":"Updated operator","values":{"groupCode":"operations","dataScope":"current_org","permissions":"admin:user:read"}}`)
	updateRecorder := httptest.NewRecorder()
	updateRequest := httptest.NewRequest(http.MethodPut, "/api/admin/resources/roles/role-operator", updateBody)
	updateRequest.Header.Set("Content-Type", "application/json")

	server.Router().ServeHTTP(updateRecorder, updateRequest)

	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("PUT operator role status = %d body = %s", updateRecorder.Code, updateRecorder.Body.String())
	}

	menuRecorder := httptest.NewRecorder()
	menuRequest := httptest.NewRequest(http.MethodGet, "/api/admin/menus", nil)
	menuRequest.Header.Set("X-Platform-User", "ops")
	server.Router().ServeHTTP(menuRecorder, menuRequest)

	if menuRecorder.Code != http.StatusOK {
		t.Fatalf("GET menus status = %d body = %s", menuRecorder.Code, menuRecorder.Body.String())
	}
	var payload adminMenuListTestPayload
	if err := json.Unmarshal(menuRecorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode menus: %v body = %s", err, menuRecorder.Body.String())
	}
	routes := make([]string, 0, len(payload.Data.Items))
	for _, item := range payload.Data.Items {
		routes = append(routes, item.Route)
	}
	if !containsString(routes, "/users") {
		t.Fatalf("filtered menu routes = %+v, want users after role permission update", routes)
	}
	if containsString(routes, "/tenants") {
		t.Fatalf("filtered menu routes = %+v, want tenants removed after role permission update", routes)
	}
}

func TestRolePermissionUpdateRefreshesSessionMenusAndResourceActions(t *testing.T) {
	server := newTestServer(ServerOptions{
		Capabilities: []capability.Manifest{authProviderTestManifest(), {ID: "tenant"}},
		Cache:        cache.NewMemoryStore(cache.MemoryStoreOptions{}),
	})
	login := loginForTest(t, server, "ops")
	token := login.Data.Token

	queryTenantsRecorder := httptest.NewRecorder()
	queryTenantsRequest := httptest.NewRequest(http.MethodPost, "/api/admin/resources/tenants/query", bytes.NewBufferString(`{"page":1,"pageSize":10}`))
	queryTenantsRequest.Header.Set("Authorization", "Bearer "+token)
	queryTenantsRequest.Header.Set("Content-Type", "application/json")
	server.Router().ServeHTTP(queryTenantsRecorder, queryTenantsRequest)
	if queryTenantsRecorder.Code != http.StatusOK {
		t.Fatalf("initial tenant query status = %d body = %s, want 200", queryTenantsRecorder.Code, queryTenantsRecorder.Body.String())
	}

	queryUsersRecorder := httptest.NewRecorder()
	queryUsersRequest := httptest.NewRequest(http.MethodPost, "/api/admin/resources/users/query", bytes.NewBufferString(`{"page":1,"pageSize":10}`))
	queryUsersRequest.Header.Set("Authorization", "Bearer "+token)
	queryUsersRequest.Header.Set("Content-Type", "application/json")
	server.Router().ServeHTTP(queryUsersRecorder, queryUsersRequest)
	if queryUsersRecorder.Code != http.StatusForbidden {
		t.Fatalf("initial users query status = %d body = %s, want 403", queryUsersRecorder.Code, queryUsersRecorder.Body.String())
	}

	updateBody := bytes.NewBufferString(`{"name":"Operator","status":"enabled","description":"Updated operator","values":{"groupCode":"operations","dataScope":"current_org","permissions":"admin:user:read"}}`)
	updateRecorder := httptest.NewRecorder()
	updateRequest := httptest.NewRequest(http.MethodPut, "/api/admin/resources/roles/role-operator", updateBody)
	updateRequest.Header.Set("Content-Type", "application/json")
	server.Router().ServeHTTP(updateRecorder, updateRequest)
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("PUT operator role status = %d body = %s", updateRecorder.Code, updateRecorder.Body.String())
	}

	sessionRecorder := httptest.NewRecorder()
	sessionRequest := httptest.NewRequest(http.MethodGet, "/api/admin/session/current", nil)
	sessionRequest.Header.Set("Authorization", "Bearer "+token)
	server.Router().ServeHTTP(sessionRecorder, sessionRequest)
	if sessionRecorder.Code != http.StatusOK {
		t.Fatalf("current session after role update status = %d body = %s", sessionRecorder.Code, sessionRecorder.Body.String())
	}
	var session currentSessionTestPayload
	if err := json.Unmarshal(sessionRecorder.Body.Bytes(), &session); err != nil {
		t.Fatalf("decode session after role update: %v body = %s", err, sessionRecorder.Body.String())
	}
	if !containsString(session.Data.Permissions, "admin:user:read") || containsString(session.Data.Permissions, "admin:tenant:read") {
		t.Fatalf("permissions after role update = %+v, want user read only", session.Data.Permissions)
	}

	menuRecorder := httptest.NewRecorder()
	menuRequest := httptest.NewRequest(http.MethodGet, "/api/admin/menus", nil)
	menuRequest.Header.Set("Authorization", "Bearer "+token)
	server.Router().ServeHTTP(menuRecorder, menuRequest)
	if menuRecorder.Code != http.StatusOK {
		t.Fatalf("menus after role update status = %d body = %s", menuRecorder.Code, menuRecorder.Body.String())
	}
	var menus adminMenuListTestPayload
	if err := json.Unmarshal(menuRecorder.Body.Bytes(), &menus); err != nil {
		t.Fatalf("decode menus after role update: %v body = %s", err, menuRecorder.Body.String())
	}
	routes := make([]string, 0, len(menus.Data.Items))
	for _, item := range menus.Data.Items {
		routes = append(routes, item.Route)
	}
	if !containsString(routes, "/users") || containsString(routes, "/tenants") {
		t.Fatalf("menu routes after role update = %+v, want users and no tenants", routes)
	}

	afterUsersRecorder := httptest.NewRecorder()
	afterUsersRequest := httptest.NewRequest(http.MethodPost, "/api/admin/resources/users/query", bytes.NewBufferString(`{"page":1,"pageSize":10}`))
	afterUsersRequest.Header.Set("Authorization", "Bearer "+token)
	afterUsersRequest.Header.Set("Content-Type", "application/json")
	server.Router().ServeHTTP(afterUsersRecorder, afterUsersRequest)
	if afterUsersRecorder.Code != http.StatusOK {
		t.Fatalf("users query after role update status = %d body = %s, want 200", afterUsersRecorder.Code, afterUsersRecorder.Body.String())
	}

	afterTenantsRecorder := httptest.NewRecorder()
	afterTenantsRequest := httptest.NewRequest(http.MethodPost, "/api/admin/resources/tenants/query", bytes.NewBufferString(`{"page":1,"pageSize":10}`))
	afterTenantsRequest.Header.Set("Authorization", "Bearer "+token)
	afterTenantsRequest.Header.Set("Content-Type", "application/json")
	server.Router().ServeHTTP(afterTenantsRecorder, afterTenantsRequest)
	if afterTenantsRecorder.Code != http.StatusForbidden {
		t.Fatalf("tenant query after role update status = %d body = %s, want 403", afterTenantsRecorder.Code, afterTenantsRecorder.Body.String())
	}
}

func TestRoleDenyPermissionsOverrideWildcardAllows(t *testing.T) {
	server := newTestServer(ServerOptions{
		Capabilities: []capability.Manifest{authProviderTestManifest(), {ID: "tenant"}},
		Cache:        cache.NewMemoryStore(cache.MemoryStoreOptions{}),
	})
	login := loginForTest(t, server, "ops")
	token := login.Data.Token

	updateBody := bytes.NewBufferString(`{"name":"Operator","status":"enabled","description":"Deny tenant read","values":{"groupCode":"operations","dataScope":"current_org","permissions":"admin:*","denyPermissions":"admin:tenant:read"}}`)
	updateRecorder := httptest.NewRecorder()
	updateRequest := httptest.NewRequest(http.MethodPut, "/api/admin/resources/roles/role-operator", updateBody)
	updateRequest.Header.Set("Content-Type", "application/json")
	server.Router().ServeHTTP(updateRecorder, updateRequest)
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("PUT operator role with deny status = %d body = %s", updateRecorder.Code, updateRecorder.Body.String())
	}

	sessionRecorder := httptest.NewRecorder()
	sessionRequest := httptest.NewRequest(http.MethodGet, "/api/admin/session/current", nil)
	sessionRequest.Header.Set("Authorization", "Bearer "+token)
	server.Router().ServeHTTP(sessionRecorder, sessionRequest)
	if sessionRecorder.Code != http.StatusOK {
		t.Fatalf("current session after deny update status = %d body = %s", sessionRecorder.Code, sessionRecorder.Body.String())
	}
	var session currentSessionTestPayload
	if err := json.Unmarshal(sessionRecorder.Body.Bytes(), &session); err != nil {
		t.Fatalf("decode session after deny update: %v body = %s", err, sessionRecorder.Body.String())
	}
	if !containsString(session.Data.Permissions, "admin:*") {
		t.Fatalf("session permissions after deny update = %+v, want admin:*", session.Data.Permissions)
	}
	if !containsString(session.Data.DeniedPermissions, "admin:tenant:read") {
		t.Fatalf("session denied permissions after deny update = %+v, want admin:tenant:read", session.Data.DeniedPermissions)
	}

	menuRecorder := httptest.NewRecorder()
	menuRequest := httptest.NewRequest(http.MethodGet, "/api/admin/menus", nil)
	menuRequest.Header.Set("Authorization", "Bearer "+token)
	server.Router().ServeHTTP(menuRecorder, menuRequest)
	if menuRecorder.Code != http.StatusOK {
		t.Fatalf("menus after deny update status = %d body = %s", menuRecorder.Code, menuRecorder.Body.String())
	}
	var menus adminMenuListTestPayload
	if err := json.Unmarshal(menuRecorder.Body.Bytes(), &menus); err != nil {
		t.Fatalf("decode menus after deny update: %v body = %s", err, menuRecorder.Body.String())
	}
	routes := make([]string, 0, len(menus.Data.Items))
	for _, item := range menus.Data.Items {
		routes = append(routes, item.Route)
	}
	if containsString(routes, "/tenants") {
		t.Fatalf("menu routes after deny update = %+v, want tenant menu denied", routes)
	}
	if !containsString(routes, "/users") {
		t.Fatalf("menu routes after deny update = %+v, want user menu allowed by wildcard", routes)
	}

	tenantRecorder := httptest.NewRecorder()
	tenantRequest := httptest.NewRequest(http.MethodPost, "/api/admin/resources/tenants/query", bytes.NewBufferString(`{"page":1,"pageSize":10}`))
	tenantRequest.Header.Set("Authorization", "Bearer "+token)
	tenantRequest.Header.Set("Content-Type", "application/json")
	server.Router().ServeHTTP(tenantRecorder, tenantRequest)
	if tenantRecorder.Code != http.StatusForbidden {
		t.Fatalf("tenant query after deny update status = %d body = %s, want 403", tenantRecorder.Code, tenantRecorder.Body.String())
	}

	userRecorder := httptest.NewRecorder()
	userRequest := httptest.NewRequest(http.MethodPost, "/api/admin/resources/users/query", bytes.NewBufferString(`{"page":1,"pageSize":10}`))
	userRequest.Header.Set("Authorization", "Bearer "+token)
	userRequest.Header.Set("Content-Type", "application/json")
	server.Router().ServeHTTP(userRecorder, userRequest)
	if userRecorder.Code != http.StatusOK {
		t.Fatalf("users query after deny update status = %d body = %s, want 200", userRecorder.Code, userRecorder.Body.String())
	}
}

func TestAdminResourceQueryAppliesRoleDataScope(t *testing.T) {
	server := newTestServer(ServerOptions{
		Capabilities: capabilitiesFromConfigForTest(t, []string{"dictionary", "tenant", "identity", "session", "rbac", "menu", "admin-shell"}),
	})
	updateBody := bytes.NewBufferString(`{"name":"Operator","status":"enabled","description":"Scoped operator","values":{"groupCode":"operations","dataScope":"current_org","permissions":"admin:user:read"}}`)
	updateRecorder := httptest.NewRecorder()
	updateRequest := httptest.NewRequest(http.MethodPut, "/api/admin/resources/roles/role-operator", updateBody)
	updateRequest.Header.Set("Content-Type", "application/json")
	server.Router().ServeHTTP(updateRecorder, updateRequest)
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("PUT operator role status = %d body = %s", updateRecorder.Code, updateRecorder.Body.String())
	}

	queryRecorder := httptest.NewRecorder()
	queryRequest := httptest.NewRequest(http.MethodPost, "/api/admin/resources/users/query", bytes.NewBufferString(`{"page":1,"pageSize":10}`))
	queryRequest.Header.Set("Content-Type", "application/json")
	queryRequest.Header.Set("X-Platform-User", "ops")
	server.Router().ServeHTTP(queryRecorder, queryRequest)
	if queryRecorder.Code != http.StatusOK {
		t.Fatalf("POST users query as ops status = %d body = %s", queryRecorder.Code, queryRecorder.Body.String())
	}
	var payload adminResourceQueryTestPayload
	if err := json.Unmarshal(queryRecorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode scoped users query: %v body = %s", err, queryRecorder.Body.String())
	}
	if payload.Data.Total != 1 || !hasTestRecordCode(payload.Data.Items, "ops") || hasTestRecordCode(payload.Data.Items, "admin") {
		t.Fatalf("scoped users query = total %d items %+v, want only ops", payload.Data.Total, payload.Data.Items)
	}
}

func TestDisabledCapabilitiesDoNotLeakAdminSurfaceOrDemoData(t *testing.T) {
	server := newTestServer(ServerOptions{Capabilities: capabilitiesFromConfigForTest(t, []string{"dictionary", "tenant", "identity", "session", "rbac", "menu", "admin-shell"})})

	menuRecorder := httptest.NewRecorder()
	menuRequest := httptest.NewRequest(http.MethodGet, "/api/admin/menus", nil)
	server.Router().ServeHTTP(menuRecorder, menuRequest)

	if menuRecorder.Code != http.StatusOK {
		t.Fatalf("GET menus status = %d body = %s", menuRecorder.Code, menuRecorder.Body.String())
	}
	var menus adminMenuListTestPayload
	if err := json.Unmarshal(menuRecorder.Body.Bytes(), &menus); err != nil {
		t.Fatalf("decode menus: %v body = %s", err, menuRecorder.Body.String())
	}
	routes := make([]string, 0, len(menus.Data.Items))
	for _, item := range menus.Data.Items {
		routes = append(routes, item.Route)
	}
	if containsString(routes, "/demo-data") || containsString(routes, "/monitoring") {
		t.Fatalf("menu routes = %+v, want no disabled capability routes", routes)
	}

	schemaRecorder := httptest.NewRecorder()
	schemaRequest := httptest.NewRequest(http.MethodGet, "/api/admin/resources/demo-data/schema", nil)
	server.Router().ServeHTTP(schemaRecorder, schemaRequest)
	if schemaRecorder.Code != http.StatusNotFound {
		t.Fatalf("GET disabled demo-data schema status = %d body = %s, want 404", schemaRecorder.Code, schemaRecorder.Body.String())
	}

	demoRecorder := httptest.NewRecorder()
	demoRequest := httptest.NewRequest(http.MethodGet, "/api/admin/demo-data", nil)
	server.Router().ServeHTTP(demoRecorder, demoRequest)
	if demoRecorder.Code != http.StatusOK {
		t.Fatalf("GET demo data status = %d body = %s", demoRecorder.Code, demoRecorder.Body.String())
	}
	var datasets demoDataListTestPayload
	if err := json.Unmarshal(demoRecorder.Body.Bytes(), &datasets); err != nil {
		t.Fatalf("decode demo data: %v body = %s", err, demoRecorder.Body.String())
	}
	if len(datasets.Data.Items) != 0 {
		t.Fatalf("demo data items = %+v, want none from disabled demo-data capability", datasets.Data.Items)
	}

	permissionRecorder := httptest.NewRecorder()
	permissionRequest := httptest.NewRequest(http.MethodGet, "/api/admin/resources/permissions", nil)
	server.Router().ServeHTTP(permissionRecorder, permissionRequest)
	if permissionRecorder.Code != http.StatusOK {
		t.Fatalf("GET permissions status = %d body = %s", permissionRecorder.Code, permissionRecorder.Body.String())
	}
	var permissions adminResourceListTestPayload
	if err := json.Unmarshal(permissionRecorder.Body.Bytes(), &permissions); err != nil {
		t.Fatalf("decode permissions: %v body = %s", err, permissionRecorder.Body.String())
	}
	if hasTestRecordCode(permissions.Data.Items, "admin:demo-data:read") || hasTestRecordCode(permissions.Data.Items, "admin:monitoring:read") {
		t.Fatalf("permissions = %+v, want no permission codes from disabled capabilities", permissions.Data.Items)
	}
}

func TestDisabledAppCapabilityDoesNotLeakAdminRuntimeSurface(t *testing.T) {
	server := newTestServer(ServerOptions{Capabilities: capabilitiesFromConfigWithAppsForTest(t, []string{"dictionary", "tenant", "identity", "session", "rbac", "menu", "admin-shell"})})

	menuRecorder := httptest.NewRecorder()
	menuRequest := httptest.NewRequest(http.MethodGet, "/api/admin/menus", nil)
	server.Router().ServeHTTP(menuRecorder, menuRequest)

	if menuRecorder.Code != http.StatusOK {
		t.Fatalf("GET menus status = %d body = %s", menuRecorder.Code, menuRecorder.Body.String())
	}
	var menus adminMenuListTestPayload
	if err := json.Unmarshal(menuRecorder.Body.Bytes(), &menus); err != nil {
		t.Fatalf("decode menus: %v body = %s", err, menuRecorder.Body.String())
	}
	routes := make([]string, 0, len(menus.Data.Items))
	for _, item := range menus.Data.Items {
		routes = append(routes, item.Route)
	}
	for _, route := range []string{"/tasks", "/role-applications", "/support-tickets"} {
		if containsString(routes, route) {
			t.Fatalf("menu routes = %+v, want no route from disabled external business capability", routes)
		}
	}

	schemaRecorder := httptest.NewRecorder()
	schemaRequest := httptest.NewRequest(http.MethodGet, "/api/admin/resources/tasks/schema", nil)
	server.Router().ServeHTTP(schemaRecorder, schemaRequest)
	if schemaRecorder.Code != http.StatusNotFound {
		t.Fatalf("GET disabled tasks schema status = %d body = %s, want 404", schemaRecorder.Code, schemaRecorder.Body.String())
	}

	queryRecorder := httptest.NewRecorder()
	queryRequest := httptest.NewRequest(http.MethodPost, "/api/admin/resources/tasks/query", bytes.NewBufferString(`{"page":1,"pageSize":10}`))
	server.Router().ServeHTTP(queryRecorder, queryRequest)
	if queryRecorder.Code != http.StatusNotFound {
		t.Fatalf("POST disabled tasks query status = %d body = %s, want 404", queryRecorder.Code, queryRecorder.Body.String())
	}

	demoRecorder := httptest.NewRecorder()
	demoRequest := httptest.NewRequest(http.MethodGet, "/api/admin/demo-data", nil)
	server.Router().ServeHTTP(demoRecorder, demoRequest)
	if demoRecorder.Code != http.StatusOK {
		t.Fatalf("GET demo data status = %d body = %s", demoRecorder.Code, demoRecorder.Body.String())
	}
	var datasets demoDataListTestPayload
	if err := json.Unmarshal(demoRecorder.Body.Bytes(), &datasets); err != nil {
		t.Fatalf("decode demo data: %v body = %s", err, demoRecorder.Body.String())
	}
	for _, item := range datasets.Data.Items {
		if item.ID == "zshenmez-demo-tasks" || item.Resource == "tasks" {
			t.Fatalf("demo data items = %+v, want no disabled external business datasets", datasets.Data.Items)
		}
	}

	permissionRecorder := httptest.NewRecorder()
	permissionRequest := httptest.NewRequest(http.MethodGet, "/api/admin/resources/permissions", nil)
	server.Router().ServeHTTP(permissionRecorder, permissionRequest)
	if permissionRecorder.Code != http.StatusOK {
		t.Fatalf("GET permissions status = %d body = %s", permissionRecorder.Code, permissionRecorder.Body.String())
	}
	var permissions adminResourceListTestPayload
	if err := json.Unmarshal(permissionRecorder.Body.Bytes(), &permissions); err != nil {
		t.Fatalf("decode permissions: %v body = %s", err, permissionRecorder.Body.String())
	}
	for _, code := range []string{"admin:task:read", "admin:role-application:read", "admin:support-ticket:read"} {
		if hasTestRecordCode(permissions.Data.Items, code) {
			t.Fatalf("permissions = %+v, want no permission codes from disabled external business capability", permissions.Data.Items)
		}
	}
}

func TestAdminResourceListEndpointReturnsSeedRows(t *testing.T) {
	server := newTestServer(ServerOptions{Capabilities: []capability.Manifest{{ID: "tenant"}}})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/admin/resources/tenants", nil)

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /api/admin/resources/tenants status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	var payload adminResourceListTestPayload
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode resource list: %v body = %s", err, recorder.Body.String())
	}
	if payload.Data.Resource != "tenants" {
		t.Fatalf("resource = %q, want tenants", payload.Data.Resource)
	}
	if len(payload.Data.Items) == 0 {
		t.Fatalf("expected seed rows, got none")
	}
	if payload.Data.Items[0].ID == "" || payload.Data.Items[0].Name == "" {
		t.Fatalf("seed row missing stable id or name: %+v", payload.Data.Items[0])
	}
}

func TestAdminResourceQueryEndpointReturnsPaginatedRows(t *testing.T) {
	server := newTestServer(ServerOptions{Capabilities: []capability.Manifest{{ID: "tenant"}}})
	body := bytes.NewBufferString(`{"conditions":[{"field":"status","operator":"=","value":"enabled"}],"sort":[{"field":"updatedAt","order":"desc"}],"page":1,"pageSize":2}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/admin/resources/tenants/query", body)
	request.Header.Set("Content-Type", "application/json")

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("POST resource query status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	var payload adminResourceQueryTestPayload
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode resource query: %v body = %s", err, recorder.Body.String())
	}
	if payload.Data.Resource != "tenants" || payload.Data.Page != 1 || payload.Data.PageSize != 2 {
		t.Fatalf("query metadata = %+v, want tenants page 1 pageSize 2", payload.Data)
	}
	if payload.Data.Total < len(payload.Data.Items) || len(payload.Data.Items) > 2 {
		t.Fatalf("query pagination mismatch: %+v", payload.Data)
	}
}

func TestAdminResourceQueryEndpointRejectsUnsafeField(t *testing.T) {
	server := newTestServer(ServerOptions{Capabilities: []capability.Manifest{{ID: "tenant"}}})
	body := bytes.NewBufferString(`{"conditions":[{"field":"password","operator":"=","value":"secret"}],"page":1,"pageSize":10}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/admin/resources/tenants/query", body)
	request.Header.Set("Content-Type", "application/json")

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("POST unsafe resource query status = %d body = %s, want 400", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "ADMIN_RESOURCE_INVALID_RECORD") {
		t.Fatalf("unsafe query body = %s", recorder.Body.String())
	}
}

func TestAdminResourceQueryEndpointRequiresReadPermission(t *testing.T) {
	server := newTestServer(ServerOptions{
		Capabilities: []capability.Manifest{{ID: "tenant"}},
		Authorizer:   denyAllAuthorizer{},
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/admin/resources/tenants/query", bytes.NewBufferString(`{"page":1,"pageSize":10}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Platform-User", "admin")

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("POST query with denying authorizer status = %d body = %s, want 403", recorder.Code, recorder.Body.String())
	}
}

func TestAdminResourceWriteRequiresActionPermission(t *testing.T) {
	server := newTestServer(ServerOptions{Capabilities: []capability.Manifest{{ID: "tenant"}}})
	createBody := bytes.NewBufferString(`{"code":"tenant-nope","name":"Tenant Nope","status":"enabled","description":"No permission"}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/admin/resources/tenants", createBody)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Platform-User", "ops")

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("POST tenant as ops status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "ADMIN_FORBIDDEN") {
		t.Fatalf("POST tenant forbidden body = %s", recorder.Body.String())
	}
}

func TestPolicyReviewApproveEndpointAppliesRoleChangeAndInvalidatesCaches(t *testing.T) {
	capabilities := capabilitiesFromConfigForTest(t, []string{"dictionary", "tenant", "identity", "rbac", "audit", "policy-review"})
	resources := adminresource.NewStoreFromCapabilities(capabilities)
	cacheStore := cache.NewMemoryStore(cache.MemoryStoreOptions{})
	server := newTestServer(ServerOptions{Capabilities: capabilities, Resources: resources, Cache: cacheStore})

	review, err := resources.Create("policy-reviews", adminresource.WriteInput{
		Code:        "PR-HTTP-1001",
		Name:        "Approve operator user read",
		Status:      "enabled",
		Description: "HTTP policy review approval fixture.",
		Values: map[string]string{
			"policyType":      "role_permission",
			"requestedAction": "update",
			"reviewStatus":    "pending",
			"roleCode":        "operator",
			"permissionCodes": "admin:user:read",
			"requestedBy":     "admin",
		},
	})
	if err != nil {
		t.Fatalf("Create(policy-reviews) error = %v", err)
	}

	sessionRecorder := httptest.NewRecorder()
	sessionRequest := httptest.NewRequest(http.MethodGet, "/api/admin/session/current", nil)
	sessionRequest.Header.Set("X-Platform-User", "ops")
	server.Router().ServeHTTP(sessionRecorder, sessionRequest)
	if sessionRecorder.Code != http.StatusOK {
		t.Fatalf("GET current session status = %d body = %s", sessionRecorder.Code, sessionRecorder.Body.String())
	}
	if _, ok, err := cacheStore.Get(sessionRequest.Context(), cacheKeyPrincipalPrefix+"ops"); err != nil || !ok {
		t.Fatalf("principal cache before approval ok = %v err = %v, want primed", ok, err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/admin/policy-reviews/"+review.ID+"/approve", nil)
	request.Header.Set("X-Platform-User", "admin")
	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("POST policy review approve status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	var payload policyReviewApproveTestPayload
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode approve payload: %v body = %s", err, recorder.Body.String())
	}
	if payload.Data.Review.Values["reviewStatus"] != "approved" || payload.Data.Review.Values["reviewedBy"] != "admin" {
		t.Fatalf("review response values = %+v, want approved by admin", payload.Data.Review.Values)
	}
	if payload.Data.Role.Code != "operator" || payload.Data.Role.Values["permissions"] != "admin:user:read" {
		t.Fatalf("role response = %+v, want operator permissions updated", payload.Data.Role)
	}
	if payload.Data.Audit.Values["action"] != "policy-review.approve" || payload.Data.Audit.Values["targetCode"] != "operator" {
		t.Fatalf("audit response values = %+v, want policy approval audit", payload.Data.Audit.Values)
	}

	roles, err := resources.List("roles")
	if err != nil {
		t.Fatalf("List(roles) error = %v", err)
	}
	var operator *adminresource.Record
	for index := range roles {
		if roles[index].Code == "operator" {
			operator = &roles[index]
			break
		}
	}
	if operator == nil || operator.Values["permissions"] != "admin:user:read" {
		t.Fatalf("operator role = %+v, want approved permissions applied", operator)
	}
	auditLogs, err := resources.List("audit-logs")
	if err != nil {
		t.Fatalf("List(audit-logs) error = %v", err)
	}
	var foundAudit bool
	for _, audit := range auditLogs {
		if audit.Code == "policy-review:PR-HTTP-1001:approved" {
			foundAudit = true
			break
		}
	}
	if !foundAudit {
		t.Fatalf("audit logs = %+v, want policy review approval audit", auditLogs)
	}
	if _, ok, err := cacheStore.Get(request.Context(), cacheKeyPrincipalPrefix+"ops"); err != nil || ok {
		t.Fatalf("principal cache after approval ok = %v err = %v, want invalidated", ok, err)
	}
}

func TestPolicyReviewRequestRejectAndExportEndpoints(t *testing.T) {
	capabilities := capabilitiesFromConfigForTest(t, []string{"dictionary", "tenant", "identity", "rbac", "audit", "policy-review"})
	resources := adminresource.NewStoreFromCapabilities(capabilities)
	server := newTestServer(ServerOptions{Capabilities: capabilities, Resources: resources, Cache: cache.NewMemoryStore(cache.MemoryStoreOptions{})})

	review, err := resources.Create("policy-reviews", adminresource.WriteInput{
		Code:        "PR-HTTP-1002",
		Name:        "Reject operator tenant delete",
		Status:      "enabled",
		Description: "HTTP policy review request/reject fixture.",
		Values: map[string]string{
			"policyType":      "deny_permission",
			"requestedAction": "update",
			"reviewStatus":    "draft",
			"roleCode":        "operator",
			"permissionCodes": "admin:tenant:delete",
			"requestedBy":     "ops",
		},
	})
	if err != nil {
		t.Fatalf("Create(policy-reviews) error = %v", err)
	}

	requestRecorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/admin/policy-reviews/"+review.ID+"/request", nil)
	request.Header.Set("X-Platform-User", "admin")
	server.Router().ServeHTTP(requestRecorder, request)
	if requestRecorder.Code != http.StatusOK {
		t.Fatalf("POST policy review request status = %d body = %s", requestRecorder.Code, requestRecorder.Body.String())
	}
	var requested policyReviewActionTestPayload
	if err := json.Unmarshal(requestRecorder.Body.Bytes(), &requested); err != nil {
		t.Fatalf("decode request payload: %v body = %s", err, requestRecorder.Body.String())
	}
	if requested.Data.Review.Values["reviewStatus"] != "pending" || requested.Data.Review.Values["requestedBy"] != "admin" || requested.Data.Audit.Values["action"] != "policy-review.request" {
		t.Fatalf("request payload = %+v, want pending request audit", requested.Data)
	}

	rejectRecorder := httptest.NewRecorder()
	rejectRequest := httptest.NewRequest(http.MethodPost, "/api/admin/policy-reviews/"+review.ID+"/reject", strings.NewReader(`{"reason":"too broad"}`))
	rejectRequest.Header.Set("Content-Type", "application/json")
	rejectRequest.Header.Set("X-Platform-User", "admin")
	server.Router().ServeHTTP(rejectRecorder, rejectRequest)
	if rejectRecorder.Code != http.StatusOK {
		t.Fatalf("POST policy review reject status = %d body = %s", rejectRecorder.Code, rejectRecorder.Body.String())
	}
	var rejected policyReviewActionTestPayload
	if err := json.Unmarshal(rejectRecorder.Body.Bytes(), &rejected); err != nil {
		t.Fatalf("decode reject payload: %v body = %s", err, rejectRecorder.Body.String())
	}
	if rejected.Data.Review.Values["reviewStatus"] != "rejected" || rejected.Data.Review.Values["rejectionReason"] != "too broad" || rejected.Data.Audit.Values["action"] != "policy-review.reject" {
		t.Fatalf("reject payload = %+v, want rejected audit", rejected.Data)
	}

	exportRecorder := httptest.NewRecorder()
	exportRequest := httptest.NewRequest(http.MethodGet, "/api/admin/policy-reviews/export", nil)
	exportRequest.Header.Set("X-Platform-User", "admin")
	server.Router().ServeHTTP(exportRecorder, exportRequest)
	if exportRecorder.Code != http.StatusOK {
		t.Fatalf("GET policy review export status = %d body = %s", exportRecorder.Code, exportRecorder.Body.String())
	}
	var exported policyReviewExportTestPayload
	if err := json.Unmarshal(exportRecorder.Body.Bytes(), &exported); err != nil {
		t.Fatalf("decode export payload: %v body = %s", err, exportRecorder.Body.String())
	}
	if exported.Data.ExportedBy != "admin" || exported.Data.ExportedAt == "" || !hasAdminRecordCode(exported.Data.Reviews, "PR-HTTP-1002") {
		t.Fatalf("export payload = %+v, want exported review", exported.Data)
	}
	if !hasAdminRecordCode(exported.Data.Audits, "policy-review:PR-HTTP-1002:requested") || !hasAdminRecordCode(exported.Data.Audits, "policy-review:PR-HTTP-1002:rejected") {
		t.Fatalf("export audits = %+v, want request and reject audits", exported.Data.Audits)
	}
}

func hasAdminRecordCode(records []adminResourceRecordTest, code string) bool {
	for _, record := range records {
		if record.Code == code {
			return true
		}
	}
	return false
}

func TestAdminResourceAuthorizationUsesConfiguredAuthorizer(t *testing.T) {
	server := newTestServer(ServerOptions{
		Capabilities: []capability.Manifest{{ID: "tenant"}},
		Authorizer:   denyAllAuthorizer{},
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/admin/resources/tenants", nil)
	request.Header.Set("X-Platform-User", "admin")

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("GET tenants with denying authorizer status = %d body = %s, want 403", recorder.Code, recorder.Body.String())
	}
}

func TestAdminResourceSchemaEndpointReturnsFieldContract(t *testing.T) {
	cacheStore := cache.NewMemoryStore(cache.MemoryStoreOptions{})
	server := newTestServer(ServerOptions{Capabilities: []capability.Manifest{{ID: "api-resource"}}, Cache: cacheStore})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/admin/resources/api-resources/schema", nil)

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("GET schema status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	var payload adminResourceSchemaTestPayload
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode schema: %v body = %s", err, recorder.Body.String())
	}
	if payload.Data.Resource != "api-resources" {
		t.Fatalf("schema resource = %q, want api-resources", payload.Data.Resource)
	}
	if payload.Data.Permissions.Read != "admin:api-resource:read" || payload.Data.Permissions.Create != "admin:api-resource:create" {
		t.Fatalf("schema permissions mismatch: %+v", payload.Data.Permissions)
	}
	if len(payload.Data.SearchFields) != 4 {
		t.Fatalf("search fields = %+v, want 4 configured fields", payload.Data.SearchFields)
	}
	if payload.Data.FormLayout != "two-column-density" {
		t.Fatalf("form layout = %q, want two-column-density", payload.Data.FormLayout)
	}
	var codeFieldFound, methodFieldFound bool
	for _, field := range payload.Data.Fields {
		if field.Key == "code" && field.Required && field.Searchable && field.InTable && field.InForm {
			codeFieldFound = true
		}
		if field.Key == "method" && field.Type == "select" && field.Source == "values" && field.Required && field.Options[0].Value == "GET" {
			methodFieldFound = true
		}
	}
	if !codeFieldFound || !methodFieldFound {
		t.Fatalf("schema fields missing expected code/method contracts: %+v", payload.Data.Fields)
	}
	value, ok, err := cacheStore.Get(request.Context(), "admin:schema:api-resources")
	if err != nil {
		t.Fatalf("cache get schema error = %v", err)
	}
	if !ok || !strings.Contains(string(value), `"resource":"api-resources"`) {
		t.Fatalf("schema cache = %q, %v; want cached api resource schema", string(value), ok)
	}
}

func TestFileResourceUsesGenericResourceRoutes(t *testing.T) {
	server := newTestServer(ServerOptions{
		Capabilities: capabilitiesFromConfigForTest(t, []string{"tenant", "identity", "session", "rbac", "menu", "audit", "dictionary", "parameter", "file-storage", "admin-shell"}),
	})
	schemaRecorder := httptest.NewRecorder()
	schemaRequest := httptest.NewRequest(http.MethodGet, "/api/admin/resources/files/schema", nil)
	server.Router().ServeHTTP(schemaRecorder, schemaRequest)
	if schemaRecorder.Code != http.StatusOK {
		t.Fatalf("GET files schema status = %d body = %s", schemaRecorder.Code, schemaRecorder.Body.String())
	}
	var schemaPayload adminResourceSchemaTestPayload
	if err := json.Unmarshal(schemaRecorder.Body.Bytes(), &schemaPayload); err != nil {
		t.Fatalf("decode files schema: %v body = %s", err, schemaRecorder.Body.String())
	}
	if schemaPayload.Data.Resource != "files" || schemaPayload.Data.Permissions.Read != "admin:file:read" {
		t.Fatalf("files schema mismatch: %+v", schemaPayload.Data)
	}

	queryRecorder := httptest.NewRecorder()
	queryRequest := httptest.NewRequest(http.MethodPost, "/api/admin/resources/files/query", strings.NewReader(`{"page":1,"pageSize":10}`))
	queryRequest.Header.Set("Content-Type", "application/json")
	server.Router().ServeHTTP(queryRecorder, queryRequest)
	if queryRecorder.Code != http.StatusOK {
		t.Fatalf("POST files query status = %d body = %s", queryRecorder.Code, queryRecorder.Body.String())
	}
}

func TestPermissionCatalogListUsesCacheAndInvalidatesAfterPermissionUpdate(t *testing.T) {
	cacheStore := cache.NewMemoryStore(cache.MemoryStoreOptions{})
	server := newTestServer(ServerOptions{
		Capabilities: capabilitiesFromConfigForTest(t, []string{"dictionary", "tenant", "identity", "rbac"}),
		Cache:        cacheStore,
	})

	primeRecorder := httptest.NewRecorder()
	primeRequest := httptest.NewRequest(http.MethodGet, "/api/admin/resources/permissions", nil)
	server.Router().ServeHTTP(primeRecorder, primeRequest)
	if primeRecorder.Code != http.StatusOK {
		t.Fatalf("GET permissions status = %d body = %s", primeRecorder.Code, primeRecorder.Body.String())
	}
	if _, ok, err := cacheStore.Get(primeRequest.Context(), "admin:permissions:list"); err != nil || !ok {
		t.Fatalf("permission catalog cache ok = %v err = %v, want cached list", ok, err)
	}

	updateBody := bytes.NewBufferString(`{"code":"admin:tenant:read","name":"Tenant Read Updated","status":"enabled","description":"Updated permission","values":{"capability":"tenant","resource":"tenants","action":"read","prefix":"admin:tenant"}}`)
	updateRecorder := httptest.NewRecorder()
	updateRequest := httptest.NewRequest(http.MethodPut, "/api/admin/resources/permissions/permission-admin-tenant-read", updateBody)
	updateRequest.Header.Set("Content-Type", "application/json")
	server.Router().ServeHTTP(updateRecorder, updateRequest)
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("PUT permission status = %d body = %s", updateRecorder.Code, updateRecorder.Body.String())
	}
	if _, ok, err := cacheStore.Get(updateRequest.Context(), "admin:permissions:list"); err != nil || ok {
		t.Fatalf("permission catalog cache after update ok = %v err = %v, want invalidated cache", ok, err)
	}

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/api/admin/resources/permissions", nil)
	server.Router().ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("GET permissions after update status = %d body = %s", listRecorder.Code, listRecorder.Body.String())
	}
	var payload adminResourceListTestPayload
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode permissions after update: %v body = %s", err, listRecorder.Body.String())
	}
	if !hasTestRecordName(payload.Data.Items, "Tenant Read Updated") {
		t.Fatalf("permissions after update = %+v, want updated permission name", payload.Data.Items)
	}
}

func TestRoleUpdateInvalidatesPrincipalAndMenuCaches(t *testing.T) {
	cacheStore := cache.NewMemoryStore(cache.MemoryStoreOptions{})
	server := newTestServer(ServerOptions{
		Capabilities: []capability.Manifest{authProviderTestManifest(), {ID: "tenant"}},
		Cache:        cacheStore,
	})
	login := loginForTest(t, server, "ops")
	token := login.Data.Token

	sessionRecorder := httptest.NewRecorder()
	sessionRequest := httptest.NewRequest(http.MethodGet, "/api/admin/session/current", nil)
	sessionRequest.Header.Set("Authorization", "Bearer "+token)
	server.Router().ServeHTTP(sessionRecorder, sessionRequest)
	if sessionRecorder.Code != http.StatusOK {
		t.Fatalf("current session status = %d body = %s", sessionRecorder.Code, sessionRecorder.Body.String())
	}
	if _, ok, err := cacheStore.Get(sessionRequest.Context(), "admin:principal:ops"); err != nil || !ok {
		t.Fatalf("principal cache ok = %v err = %v, want cached principal", ok, err)
	}

	menuRecorder := httptest.NewRecorder()
	menuRequest := httptest.NewRequest(http.MethodGet, "/api/admin/menus", nil)
	menuRequest.Header.Set("Authorization", "Bearer "+token)
	server.Router().ServeHTTP(menuRecorder, menuRequest)
	if menuRecorder.Code != http.StatusOK {
		t.Fatalf("menus status = %d body = %s", menuRecorder.Code, menuRecorder.Body.String())
	}
	if _, ok, err := cacheStore.Get(menuRequest.Context(), "admin:menus:ops"); err != nil || !ok {
		t.Fatalf("menu cache ok = %v err = %v, want cached menus", ok, err)
	}

	updateBody := bytes.NewBufferString(`{"name":"Operator","status":"enabled","description":"Updated operator","values":{"groupCode":"operations","dataScope":"current_org","permissions":"admin:user:read"}}`)
	updateRecorder := httptest.NewRecorder()
	updateRequest := httptest.NewRequest(http.MethodPut, "/api/admin/resources/roles/role-operator", updateBody)
	updateRequest.Header.Set("Content-Type", "application/json")
	server.Router().ServeHTTP(updateRecorder, updateRequest)
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("PUT operator role status = %d body = %s", updateRecorder.Code, updateRecorder.Body.String())
	}

	if _, ok, err := cacheStore.Get(updateRequest.Context(), "admin:principal:ops"); err != nil || ok {
		t.Fatalf("principal cache after role update ok = %v err = %v, want invalidated principal", ok, err)
	}
	if _, ok, err := cacheStore.Get(updateRequest.Context(), "admin:menus:ops"); err != nil || ok {
		t.Fatalf("menu cache after role update ok = %v err = %v, want invalidated menus", ok, err)
	}
}

func TestDistributedInvalidationClearsPeerPrincipalMenuAndPolicyCaches(t *testing.T) {
	capabilities := []capability.Manifest{authProviderTestManifest(), {ID: "tenant"}}
	resources := adminresource.NewStoreFromCapabilities(capabilities)
	bus := cache.NewMemoryInvalidationBus()
	writerCache := cache.NewMemoryStore(cache.MemoryStoreOptions{})
	readerCache := cache.NewMemoryStore(cache.MemoryStoreOptions{})
	writer := newTestServer(ServerOptions{
		Capabilities:    capabilities,
		Resources:       resources,
		Cache:           writerCache,
		InvalidationBus: bus,
	})
	reader := newTestServer(ServerOptions{
		Capabilities:    capabilities,
		Resources:       resources,
		Cache:           readerCache,
		InvalidationBus: bus,
	})
	login := loginForTest(t, reader, "ops")
	token := login.Data.Token

	sessionRecorder := httptest.NewRecorder()
	sessionRequest := httptest.NewRequest(http.MethodGet, "/api/admin/session/current", nil)
	sessionRequest.Header.Set("Authorization", "Bearer "+token)
	reader.Router().ServeHTTP(sessionRecorder, sessionRequest)
	if sessionRecorder.Code != http.StatusOK {
		t.Fatalf("reader current session status = %d body = %s", sessionRecorder.Code, sessionRecorder.Body.String())
	}
	menuRecorder := httptest.NewRecorder()
	menuRequest := httptest.NewRequest(http.MethodGet, "/api/admin/menus", nil)
	menuRequest.Header.Set("Authorization", "Bearer "+token)
	reader.Router().ServeHTTP(menuRecorder, menuRequest)
	if menuRecorder.Code != http.StatusOK {
		t.Fatalf("reader menus status = %d body = %s", menuRecorder.Code, menuRecorder.Body.String())
	}
	tenantRecorder := httptest.NewRecorder()
	tenantRequest := httptest.NewRequest(http.MethodPost, "/api/admin/resources/tenants/query", bytes.NewBufferString(`{"page":1,"pageSize":10}`))
	tenantRequest.Header.Set("Authorization", "Bearer "+token)
	tenantRequest.Header.Set("Content-Type", "application/json")
	reader.Router().ServeHTTP(tenantRecorder, tenantRequest)
	if tenantRecorder.Code != http.StatusOK {
		t.Fatalf("reader tenant query status = %d body = %s, want 200 before role update", tenantRecorder.Code, tenantRecorder.Body.String())
	}
	if _, ok, err := readerCache.Get(sessionRequest.Context(), "admin:principal:ops"); err != nil || !ok {
		t.Fatalf("reader principal cache ok = %v err = %v, want cached principal", ok, err)
	}
	if _, ok, err := readerCache.Get(menuRequest.Context(), "admin:menus:ops"); err != nil || !ok {
		t.Fatalf("reader menu cache ok = %v err = %v, want cached menus", ok, err)
	}

	updateBody := bytes.NewBufferString(`{"name":"Operator","status":"enabled","description":"Updated operator","values":{"groupCode":"operations","dataScope":"current_org","permissions":"admin:user:read"}}`)
	updateRecorder := httptest.NewRecorder()
	updateRequest := httptest.NewRequest(http.MethodPut, "/api/admin/resources/roles/role-operator", updateBody)
	updateRequest.Header.Set("Content-Type", "application/json")
	writer.Router().ServeHTTP(updateRecorder, updateRequest)
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("writer role update status = %d body = %s", updateRecorder.Code, updateRecorder.Body.String())
	}

	if _, ok, err := readerCache.Get(updateRequest.Context(), "admin:principal:ops"); err != nil || ok {
		t.Fatalf("reader principal cache after peer update ok = %v err = %v, want invalidated", ok, err)
	}
	if _, ok, err := readerCache.Get(updateRequest.Context(), "admin:menus:ops"); err != nil || ok {
		t.Fatalf("reader menu cache after peer update ok = %v err = %v, want invalidated", ok, err)
	}

	afterTenantRecorder := httptest.NewRecorder()
	afterTenantRequest := httptest.NewRequest(http.MethodPost, "/api/admin/resources/tenants/query", bytes.NewBufferString(`{"page":1,"pageSize":10}`))
	afterTenantRequest.Header.Set("Authorization", "Bearer "+token)
	afterTenantRequest.Header.Set("Content-Type", "application/json")
	reader.Router().ServeHTTP(afterTenantRecorder, afterTenantRequest)
	if afterTenantRecorder.Code != http.StatusForbidden {
		t.Fatalf("reader tenant query after peer role update status = %d body = %s, want 403", afterTenantRecorder.Code, afterTenantRecorder.Body.String())
	}
}

func TestDistributedInvalidationReloadsIndependentGORMAdminResourceStore(t *testing.T) {
	capabilities := []capability.Manifest{authProviderTestManifest(), {ID: "tenant"}}
	databasePath := filepath.Join(t.TempDir(), "admin-resources.db")
	writerResources := openGORMAdminResourceStoreForHTTPTest(t, databasePath, capabilities)
	readerResources := openGORMAdminResourceStoreForHTTPTest(t, databasePath, capabilities)
	bus := cache.NewMemoryInvalidationBus()
	writer := newTestServer(ServerOptions{
		Capabilities:    capabilities,
		Resources:       writerResources,
		Cache:           cache.NewMemoryStore(cache.MemoryStoreOptions{}),
		InvalidationBus: bus,
	})
	reader := newTestServer(ServerOptions{
		Capabilities:    capabilities,
		Resources:       readerResources,
		Cache:           cache.NewMemoryStore(cache.MemoryStoreOptions{}),
		InvalidationBus: bus,
	})
	login := loginForTest(t, reader, "ops")

	beforeRecorder := httptest.NewRecorder()
	beforeRequest := httptest.NewRequest(http.MethodPost, "/api/admin/resources/tenants/query", bytes.NewBufferString(`{"page":1,"pageSize":10}`))
	beforeRequest.Header.Set("Authorization", "Bearer "+login.Data.Token)
	beforeRequest.Header.Set("Content-Type", "application/json")
	reader.Router().ServeHTTP(beforeRecorder, beforeRequest)
	if beforeRecorder.Code != http.StatusOK {
		t.Fatalf("reader tenant query before peer role update status = %d body = %s, want 200", beforeRecorder.Code, beforeRecorder.Body.String())
	}

	updateRecorder := httptest.NewRecorder()
	updateRequest := httptest.NewRequest(http.MethodPut, "/api/admin/resources/roles/role-operator", bytes.NewBufferString(`{"name":"Operator","status":"enabled","description":"Updated operator","values":{"groupCode":"operations","dataScope":"current_org","permissions":"admin:user:read"}}`))
	updateRequest.Header.Set("Content-Type", "application/json")
	writer.Router().ServeHTTP(updateRecorder, updateRequest)
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("writer role update status = %d body = %s", updateRecorder.Code, updateRecorder.Body.String())
	}

	afterRecorder := httptest.NewRecorder()
	afterRequest := httptest.NewRequest(http.MethodPost, "/api/admin/resources/tenants/query", bytes.NewBufferString(`{"page":1,"pageSize":10}`))
	afterRequest.Header.Set("Authorization", "Bearer "+login.Data.Token)
	afterRequest.Header.Set("Content-Type", "application/json")
	reader.Router().ServeHTTP(afterRecorder, afterRequest)
	if afterRecorder.Code != http.StatusForbidden {
		t.Fatalf("reader tenant query after peer role update status = %d body = %s, want 403", afterRecorder.Code, afterRecorder.Body.String())
	}
}

func TestDistributedInvalidationPreservesDerivedCachesWhenAdminReloadFails(t *testing.T) {
	capabilities := []capability.Manifest{authProviderTestManifest(), {ID: "tenant"}}
	repository := &controllableAdminResourceRepository{}
	resources, err := adminresource.NewRepositoryBackedStoreFromCapabilities(repository, capabilities)
	if err != nil {
		t.Fatalf("NewRepositoryBackedStoreFromCapabilities() error = %v", err)
	}
	cacheStore := cache.NewMemoryStore(cache.MemoryStoreOptions{})
	bus := cache.NewMemoryInvalidationBus()
	server := newTestServer(ServerOptions{
		Capabilities:    capabilities,
		Resources:       resources,
		Cache:           cacheStore,
		InvalidationBus: bus,
	})
	ctx := context.Background()
	if err := cacheStore.Set(ctx, cacheKeyPrincipalPrefix+"ops", []byte(`{"cached":true}`), time.Hour); err != nil {
		t.Fatalf("cache principal error = %v", err)
	}
	if err := cacheStore.Set(ctx, cacheKeyMenusPrefix+"ops", []byte(`{"cached":true}`), time.Hour); err != nil {
		t.Fatalf("cache menus error = %v", err)
	}
	if _, ok := server.policyAuthorizerForRequest(); !ok {
		t.Fatal("prime policy authorizer = false")
	}
	policyBefore := server.policyAuthorizer
	repository.loadErr = errors.New("reload unavailable")

	if err := bus.PublishInvalidation(ctx, cache.InvalidationEvent{Resource: "roles"}); err != nil {
		t.Fatalf("PublishInvalidation() error = %v", err)
	}

	if _, ok, err := cacheStore.Get(ctx, cacheKeyPrincipalPrefix+"ops"); err != nil || !ok {
		t.Fatalf("principal cache after reload error ok = %v err = %v, want preserved", ok, err)
	}
	if _, ok, err := cacheStore.Get(ctx, cacheKeyMenusPrefix+"ops"); err != nil || !ok {
		t.Fatalf("menu cache after reload error ok = %v err = %v, want preserved", ok, err)
	}
	if server.policyAuthorizer != policyBefore {
		t.Fatal("policy authorizer changed after admin resource reload error")
	}
}

func TestDistributedSessionInvalidationReloadsPeerSessionStore(t *testing.T) {
	now := time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC)
	capabilities := []capability.Manifest{authProviderTestManifest()}
	bus := cache.NewMemoryInvalidationBus()
	repository := session.NewFileRepository(filepath.Join(t.TempDir(), "sessions.json"))
	writerSessions, err := session.NewRepositoryBackedStore(session.Options{TTL: time.Hour, Now: func() time.Time { return now }}, repository)
	if err != nil {
		t.Fatalf("NewRepositoryBackedStore(writer) error = %v", err)
	}
	readerSessions, err := session.NewRepositoryBackedStore(session.Options{TTL: time.Hour, Now: func() time.Time { return now }}, repository)
	if err != nil {
		t.Fatalf("NewRepositoryBackedStore(reader) error = %v", err)
	}
	writer := newTestServer(ServerOptions{
		Capabilities:    capabilities,
		Sessions:        writerSessions,
		InvalidationBus: bus,
		Now:             func() time.Time { return now },
	})
	reader := newTestServer(ServerOptions{
		Capabilities:    capabilities,
		Sessions:        readerSessions,
		InvalidationBus: bus,
		Now:             func() time.Time { return now },
	})

	login := loginForTest(t, writer, "ops")
	sessionRecorder := httptest.NewRecorder()
	sessionRequest := httptest.NewRequest(http.MethodGet, "/api/admin/session/current", nil)
	sessionRequest.Header.Set("Authorization", "Bearer "+login.Data.Token)
	reader.Router().ServeHTTP(sessionRecorder, sessionRequest)
	if sessionRecorder.Code != http.StatusOK {
		t.Fatalf("reader current session after peer login status = %d body = %s", sessionRecorder.Code, sessionRecorder.Body.String())
	}

	logoutRecorder := httptest.NewRecorder()
	logoutRequest := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	logoutRequest.Header.Set("Authorization", "Bearer "+login.Data.Token)
	writer.Router().ServeHTTP(logoutRecorder, logoutRequest)
	if logoutRecorder.Code != http.StatusOK {
		t.Fatalf("writer auth logout status = %d body = %s", logoutRecorder.Code, logoutRecorder.Body.String())
	}

	revokedRecorder := httptest.NewRecorder()
	revokedRequest := httptest.NewRequest(http.MethodGet, "/api/admin/session/current", nil)
	revokedRequest.Header.Set("Authorization", "Bearer "+login.Data.Token)
	reader.Router().ServeHTTP(revokedRecorder, revokedRequest)
	if revokedRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("reader current session after peer logout status = %d body = %s, want 401", revokedRecorder.Code, revokedRecorder.Body.String())
	}
}

func TestSettingsSchemaEndpointReturnsBrandingFieldContract(t *testing.T) {
	server := newTestServer(ServerOptions{Capabilities: []capability.Manifest{{ID: "parameter"}}})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/admin/resources/settings/schema", nil)

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("GET settings schema status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	var payload adminResourceSchemaTestPayload
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode settings schema: %v body = %s", err, recorder.Body.String())
	}
	var productNameFound, defaultThemeFound bool
	for _, field := range payload.Data.Fields {
		if field.Key == "productName" && field.Type == "text" && field.Source == "values" && field.Required {
			productNameFound = true
		}
		if field.Key == "defaultTheme" && field.Type == "select" && field.Source == "values" && len(field.Options) >= 4 {
			defaultThemeFound = true
		}
	}
	if !productNameFound || !defaultThemeFound {
		t.Fatalf("settings schema missing branding fields: %+v", payload.Data.Fields)
	}
}

func TestWriteAdminResourceErrorMapsRevisionConflict(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	writeAdminResourceError(ctx, &adminresource.RevisionConflictError{Expected: 3, Actual: 4})

	if recorder.Code != http.StatusConflict {
		t.Fatalf("revision conflict status = %d body = %s, want 409", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"code":"ADMIN_RESOURCE_REVISION_CONFLICT"`) {
		t.Fatalf("revision conflict body = %s, want ADMIN_RESOURCE_REVISION_CONFLICT", recorder.Body.String())
	}
}

func TestAdminResourceCreateUpdateDelete(t *testing.T) {
	server := newTestServer(ServerOptions{Capabilities: []capability.Manifest{{ID: "dictionary"}}})
	createBody := bytes.NewBufferString(`{"code":"demo-status","name":"Demo Status","status":"enabled","description":"Demo status dictionary","values":{"scope":"global"}}`)
	createRecorder := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/admin/resources/dictionary-parameters", createBody)
	createRequest.Header.Set("Content-Type", "application/json")

	server.Router().ServeHTTP(createRecorder, createRequest)

	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("POST resource status = %d body = %s", createRecorder.Code, createRecorder.Body.String())
	}
	var createdPayload adminResourceRecordTestPayload
	if err := json.Unmarshal(createRecorder.Body.Bytes(), &createdPayload); err != nil {
		t.Fatalf("decode created resource: %v body = %s", err, createRecorder.Body.String())
	}
	created := createdPayload.Data.Record
	if created.ID == "" || created.Code != "demo-status" || created.Values["scope"] != "global" {
		t.Fatalf("created record mismatch: %+v", created)
	}

	updateBody := bytes.NewBufferString(`{"name":"Demo Status Updated","status":"disabled","description":"Updated","values":{"scope":"tenant"}}`)
	updateRecorder := httptest.NewRecorder()
	updateRequest := httptest.NewRequest(http.MethodPut, "/api/admin/resources/dictionary-parameters/"+created.ID, updateBody)
	updateRequest.Header.Set("Content-Type", "application/json")

	server.Router().ServeHTTP(updateRecorder, updateRequest)

	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("PUT resource status = %d body = %s", updateRecorder.Code, updateRecorder.Body.String())
	}
	var updatedPayload adminResourceRecordTestPayload
	if err := json.Unmarshal(updateRecorder.Body.Bytes(), &updatedPayload); err != nil {
		t.Fatalf("decode updated resource: %v body = %s", err, updateRecorder.Body.String())
	}
	updated := updatedPayload.Data.Record
	if updated.Name != "Demo Status Updated" || updated.Status != "disabled" || updated.Values["scope"] != "tenant" {
		t.Fatalf("updated record mismatch: %+v", updated)
	}

	deleteRecorder := httptest.NewRecorder()
	deleteRequest := httptest.NewRequest(http.MethodDelete, "/api/admin/resources/dictionary-parameters/"+created.ID, nil)

	server.Router().ServeHTTP(deleteRecorder, deleteRequest)

	if deleteRecorder.Code != http.StatusOK {
		t.Fatalf("DELETE resource status = %d body = %s", deleteRecorder.Code, deleteRecorder.Body.String())
	}
	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/api/admin/resources/dictionary-parameters", nil)
	server.Router().ServeHTTP(listRecorder, listRequest)
	if strings.Contains(listRecorder.Body.String(), created.ID) {
		t.Fatalf("deleted record still present: %s", listRecorder.Body.String())
	}
}

func TestAdminResourceWriteOperationsRecordAuditEvents(t *testing.T) {
	server := newTestServer(ServerOptions{Capabilities: []capability.Manifest{{ID: "dictionary"}, {ID: "audit"}}})
	createBody := bytes.NewBufferString(`{"code":"audit-status","name":"Audit Status","status":"enabled","description":"Audit status dictionary","values":{"scope":"global"}}`)
	createRecorder := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/admin/resources/dictionary-parameters", createBody)
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.Header.Set("X-Platform-User", "admin")

	server.Router().ServeHTTP(createRecorder, createRequest)

	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("POST resource status = %d body = %s", createRecorder.Code, createRecorder.Body.String())
	}
	var createdPayload adminResourceRecordTestPayload
	if err := json.Unmarshal(createRecorder.Body.Bytes(), &createdPayload); err != nil {
		t.Fatalf("decode created resource: %v body = %s", err, createRecorder.Body.String())
	}
	created := createdPayload.Data.Record

	updateBody := bytes.NewBufferString(`{"name":"Audit Status Updated","status":"disabled","description":"Updated","values":{"scope":"tenant"}}`)
	updateRecorder := httptest.NewRecorder()
	updateRequest := httptest.NewRequest(http.MethodPut, "/api/admin/resources/dictionary-parameters/"+created.ID, updateBody)
	updateRequest.Header.Set("Content-Type", "application/json")
	updateRequest.Header.Set("X-Platform-User", "admin")
	server.Router().ServeHTTP(updateRecorder, updateRequest)
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("PUT resource status = %d body = %s", updateRecorder.Code, updateRecorder.Body.String())
	}

	deleteRecorder := httptest.NewRecorder()
	deleteRequest := httptest.NewRequest(http.MethodDelete, "/api/admin/resources/dictionary-parameters/"+created.ID, nil)
	deleteRequest.Header.Set("X-Platform-User", "admin")
	server.Router().ServeHTTP(deleteRecorder, deleteRequest)
	if deleteRecorder.Code != http.StatusOK {
		t.Fatalf("DELETE resource status = %d body = %s", deleteRecorder.Code, deleteRecorder.Body.String())
	}

	auditRecorder := httptest.NewRecorder()
	auditRequest := httptest.NewRequest(http.MethodGet, "/api/admin/resources/audit-logs", nil)
	auditRequest.Header.Set("X-Platform-User", "admin")
	server.Router().ServeHTTP(auditRecorder, auditRequest)
	if auditRecorder.Code != http.StatusOK {
		t.Fatalf("GET audit-logs status = %d body = %s", auditRecorder.Code, auditRecorder.Body.String())
	}
	var auditPayload adminResourceListTestPayload
	if err := json.Unmarshal(auditRecorder.Body.Bytes(), &auditPayload); err != nil {
		t.Fatalf("decode audit records: %v body = %s", err, auditRecorder.Body.String())
	}
	for _, action := range []string{"admin_resource.create", "admin_resource.update", "admin_resource.delete"} {
		if !hasAdminResourceAuditRecord(auditPayload.Data.Items, action, "dictionary-parameters", created.ID, created.Code, "admin") {
			t.Fatalf("audit records missing %s for created record %+v: %+v", action, created, auditPayload.Data.Items)
		}
	}

	queryBody := bytes.NewBufferString(`{"conditions":[{"field":"action","operator":"=","value":"admin_resource.create"},{"field":"resource","operator":"=","value":"dictionary-parameters"}],"sort":[{"field":"createdAt","order":"desc"}],"page":1,"pageSize":10}`)
	queryRecorder := httptest.NewRecorder()
	queryRequest := httptest.NewRequest(http.MethodPost, "/api/admin/resources/audit-logs/query", queryBody)
	queryRequest.Header.Set("Content-Type", "application/json")
	queryRequest.Header.Set("X-Platform-User", "admin")
	server.Router().ServeHTTP(queryRecorder, queryRequest)
	if queryRecorder.Code != http.StatusOK {
		t.Fatalf("POST audit-logs query status = %d body = %s", queryRecorder.Code, queryRecorder.Body.String())
	}
	var queryPayload adminResourceQueryTestPayload
	if err := json.Unmarshal(queryRecorder.Body.Bytes(), &queryPayload); err != nil {
		t.Fatalf("decode audit query records: %v body = %s", err, queryRecorder.Body.String())
	}
	if queryPayload.Data.Total != 1 || !hasAdminResourceAuditRecord(queryPayload.Data.Items, "admin_resource.create", "dictionary-parameters", created.ID, created.Code, "admin") {
		t.Fatalf("audit query payload = %+v, want create audit for %+v", queryPayload.Data, created)
	}
	if createdAt := queryPayload.Data.Items[0].Values["createdAt"]; createdAt == "" {
		t.Fatalf("audit query createdAt is empty: %+v", queryPayload.Data.Items[0])
	}
}

func TestAdminResourceCreateValidatesSchemaRequiredValueFields(t *testing.T) {
	server := newTestServer(ServerOptions{Capabilities: []capability.Manifest{{ID: "api-resource"}}})
	createBody := bytes.NewBufferString(`{"code":"POST:/api/demo","name":"Demo API","status":"enabled","description":"Demo API","values":{}}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/admin/resources/api-resources", createBody)
	request.Header.Set("Content-Type", "application/json")

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("POST api resource without required method status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "method is required") {
		t.Fatalf("missing required method error body = %s", recorder.Body.String())
	}
}

func TestAdminAPITokenCreateReturnsSecretOnceAndRevokeKeepsSanitizedRecord(t *testing.T) {
	server := newTestServer(ServerOptions{Capabilities: capabilitiesFromConfigForTest(t, []string{
		"tenant", "identity", "session", "rbac", "menu", "api-resource", "dictionary", "parameter", "audit", "admin-shell", "system-admin",
	})})
	createBody := bytes.NewBufferString(`{"name":"Integration Token","status":"active","description":"Issued for integration tests.","values":{"scope":"admin:tenant:read,admin:api-resource:read","expiresAt":"2026-12-31T00:00:00Z"}}`)
	createRecorder := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/admin/resources/api-tokens", createBody)
	createRequest.Header.Set("Content-Type", "application/json")

	server.Router().ServeHTTP(createRecorder, createRequest)

	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("POST api token status = %d body = %s", createRecorder.Code, createRecorder.Body.String())
	}
	var created adminResourceRecordTestPayload
	if err := json.Unmarshal(createRecorder.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode api token create: %v body = %s", err, createRecorder.Body.String())
	}
	if !strings.HasPrefix(created.Data.Token, "pgo_") {
		t.Fatalf("issued token = %q, want pgo_ prefix", created.Data.Token)
	}
	record := created.Data.Record
	if record.ID == "" || record.Values["tokenPrefix"] == "" || !strings.HasPrefix(created.Data.Token, record.Values["tokenPrefix"]) {
		t.Fatalf("created api token record missing id/prefix: token=%q record=%+v", created.Data.Token, record)
	}
	if record.Values["tokenHash"] != "" || record.Values["token"] != "" {
		t.Fatalf("created api token leaked token material: %+v", record.Values)
	}

	queryRecorder := httptest.NewRecorder()
	queryRequest := httptest.NewRequest(http.MethodPost, "/api/admin/resources/api-tokens/query", bytes.NewBufferString(`{"page":1,"pageSize":10}`))
	queryRequest.Header.Set("Content-Type", "application/json")
	server.Router().ServeHTTP(queryRecorder, queryRequest)
	if queryRecorder.Code != http.StatusOK {
		t.Fatalf("POST api token query status = %d body = %s", queryRecorder.Code, queryRecorder.Body.String())
	}
	if strings.Contains(queryRecorder.Body.String(), created.Data.Token) || strings.Contains(queryRecorder.Body.String(), "tokenHash") {
		t.Fatalf("api token query leaked secret or hash: %s", queryRecorder.Body.String())
	}

	deleteRecorder := httptest.NewRecorder()
	deleteRequest := httptest.NewRequest(http.MethodDelete, "/api/admin/resources/api-tokens/"+record.ID, nil)
	server.Router().ServeHTTP(deleteRecorder, deleteRequest)
	if deleteRecorder.Code != http.StatusOK {
		t.Fatalf("DELETE api token status = %d body = %s", deleteRecorder.Code, deleteRecorder.Body.String())
	}
	if !strings.Contains(deleteRecorder.Body.String(), `"revoked":true`) {
		t.Fatalf("DELETE api token body = %s, want revoked true", deleteRecorder.Body.String())
	}

	revokedRecorder := httptest.NewRecorder()
	revokedRequest := httptest.NewRequest(http.MethodPost, "/api/admin/resources/api-tokens/query", bytes.NewBufferString(`{"conditions":[{"field":"id","operator":"=","value":"`+record.ID+`"}],"page":1,"pageSize":1}`))
	revokedRequest.Header.Set("Content-Type", "application/json")
	server.Router().ServeHTTP(revokedRecorder, revokedRequest)
	if revokedRecorder.Code != http.StatusOK {
		t.Fatalf("POST revoked api token query status = %d body = %s", revokedRecorder.Code, revokedRecorder.Body.String())
	}
	var revoked adminResourceQueryTestPayload
	if err := json.Unmarshal(revokedRecorder.Body.Bytes(), &revoked); err != nil {
		t.Fatalf("decode revoked api token query: %v body = %s", err, revokedRecorder.Body.String())
	}
	if revoked.Data.Total != 1 || len(revoked.Data.Items) != 1 || revoked.Data.Items[0].Status != "revoked" {
		t.Fatalf("revoked api token query = %+v, want retained revoked record", revoked.Data)
	}
	if revoked.Data.Items[0].Values["tokenHash"] != "" || strings.Contains(revokedRecorder.Body.String(), created.Data.Token) {
		t.Fatalf("revoked api token query leaked token material: %s", revokedRecorder.Body.String())
	}
}

func TestAdminAPITokenCreateRejectsUnknownScope(t *testing.T) {
	server := newTestServer(ServerOptions{Capabilities: capabilitiesFromConfigForTest(t, []string{
		"tenant", "identity", "session", "rbac", "menu", "api-resource", "dictionary", "parameter", "audit", "admin-shell", "system-admin",
	})})
	createBody := bytes.NewBufferString(`{"name":"Bad Token","status":"active","description":"Unknown scope.","values":{"scope":"admin:missing:read","expiresAt":"2026-12-31T00:00:00Z"}}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/admin/resources/api-tokens", createBody)
	request.Header.Set("Content-Type", "application/json")

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("POST api token with unknown scope status = %d body = %s, want 400", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "unknown api token scope") {
		t.Fatalf("unknown scope body = %s", recorder.Body.String())
	}
}

func createAdminAPITokenForTest(t *testing.T, server *Server, scope string) (string, string) {
	t.Helper()
	createBody := bytes.NewBufferString(`{"name":"Integration Token","status":"active","description":"Issued for integration tests.","values":{"scope":"` + scope + `","expiresAt":"2026-12-31T00:00:00Z"}}`)
	createRecorder := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/admin/resources/api-tokens", createBody)
	createRequest.Header.Set("Content-Type", "application/json")

	server.Router().ServeHTTP(createRecorder, createRequest)

	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("POST api token status = %d body = %s", createRecorder.Code, createRecorder.Body.String())
	}
	var created adminResourceRecordTestPayload
	if err := json.Unmarshal(createRecorder.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode api token create: %v body = %s", err, createRecorder.Body.String())
	}
	if created.Data.Token == "" || created.Data.Record.ID == "" {
		t.Fatalf("created api token missing token or id: %+v", created.Data)
	}
	return created.Data.Token, created.Data.Record.ID
}

func TestAdminAPITokenAuthorizesScopedResourceCalls(t *testing.T) {
	server := newTestServer(ServerOptions{Capabilities: capabilitiesFromConfigForTest(t, []string{
		"tenant", "identity", "session", "rbac", "menu", "api-resource", "dictionary", "parameter", "audit", "admin-shell", "system-admin",
	})})
	token, recordID := createAdminAPITokenForTest(t, server, "admin:tenant:read")

	tenantRecorder := httptest.NewRecorder()
	tenantRequest := httptest.NewRequest(http.MethodGet, "/api/admin/resources/tenants", nil)
	tenantRequest.Header.Set("Authorization", "Bearer "+token)
	server.Router().ServeHTTP(tenantRecorder, tenantRequest)

	if tenantRecorder.Code != http.StatusOK {
		t.Fatalf("GET tenants with api token status = %d body = %s", tenantRecorder.Code, tenantRecorder.Body.String())
	}
	if strings.Contains(tenantRecorder.Body.String(), token) || strings.Contains(tenantRecorder.Body.String(), "tokenHash") {
		t.Fatalf("resource response leaked api token material: %s", tenantRecorder.Body.String())
	}

	apiResourceRecorder := httptest.NewRecorder()
	apiResourceRequest := httptest.NewRequest(http.MethodGet, "/api/admin/resources/api-resources", nil)
	apiResourceRequest.Header.Set("Authorization", "Bearer "+token)
	server.Router().ServeHTTP(apiResourceRecorder, apiResourceRequest)
	if apiResourceRecorder.Code != http.StatusForbidden {
		t.Fatalf("GET api-resources with tenant-scoped token status = %d body = %s, want 403", apiResourceRecorder.Code, apiResourceRecorder.Body.String())
	}

	sessionRecorder := httptest.NewRecorder()
	sessionRequest := httptest.NewRequest(http.MethodGet, "/api/admin/session/current", nil)
	sessionRequest.Header.Set("Authorization", "Bearer "+token)
	server.Router().ServeHTTP(sessionRecorder, sessionRequest)
	if sessionRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("GET current session with api token status = %d body = %s, want 401", sessionRecorder.Code, sessionRecorder.Body.String())
	}

	deleteRecorder := httptest.NewRecorder()
	deleteRequest := httptest.NewRequest(http.MethodDelete, "/api/admin/resources/api-tokens/"+recordID, nil)
	server.Router().ServeHTTP(deleteRecorder, deleteRequest)
	if deleteRecorder.Code != http.StatusOK {
		t.Fatalf("DELETE api token status = %d body = %s", deleteRecorder.Code, deleteRecorder.Body.String())
	}

	revokedRecorder := httptest.NewRecorder()
	revokedRequest := httptest.NewRequest(http.MethodGet, "/api/admin/resources/tenants", nil)
	revokedRequest.Header.Set("Authorization", "Bearer "+token)
	server.Router().ServeHTTP(revokedRecorder, revokedRequest)
	if revokedRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("GET tenants with revoked api token status = %d body = %s, want 401", revokedRecorder.Code, revokedRecorder.Body.String())
	}
}

func TestAdminAPITokenUpdateCannotReplaceTokenMaterial(t *testing.T) {
	server := newTestServer(ServerOptions{Capabilities: capabilitiesFromConfigForTest(t, []string{
		"tenant", "identity", "session", "rbac", "menu", "api-resource", "dictionary", "parameter", "audit", "admin-shell", "system-admin",
	})})
	token, recordID := createAdminAPITokenForTest(t, server, "admin:tenant:read")
	updateBody := bytes.NewBufferString(`{"name":"Updated Integration Token","status":"active","description":"Updated token.","values":{"scope":"admin:tenant:read","tokenPrefix":"pgo_attacker","tokenHash":"` + hashAPIToken("pgo_attacker") + `","token":"pgo_attacker"}}`)
	updateRecorder := httptest.NewRecorder()
	updateRequest := httptest.NewRequest(http.MethodPut, "/api/admin/resources/api-tokens/"+recordID, updateBody)
	updateRequest.Header.Set("Content-Type", "application/json")

	server.Router().ServeHTTP(updateRecorder, updateRequest)

	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("PUT api token status = %d body = %s", updateRecorder.Code, updateRecorder.Body.String())
	}
	if strings.Contains(updateRecorder.Body.String(), "tokenHash") || strings.Contains(updateRecorder.Body.String(), "pgo_attacker") {
		t.Fatalf("PUT api token leaked or accepted attacker token material: %s", updateRecorder.Body.String())
	}

	tenantRecorder := httptest.NewRecorder()
	tenantRequest := httptest.NewRequest(http.MethodGet, "/api/admin/resources/tenants", nil)
	tenantRequest.Header.Set("Authorization", "Bearer "+token)
	server.Router().ServeHTTP(tenantRecorder, tenantRequest)
	if tenantRecorder.Code != http.StatusOK {
		t.Fatalf("GET tenants with original api token after update status = %d body = %s", tenantRecorder.Code, tenantRecorder.Body.String())
	}
}

func TestAdminFileUploadContentAndDelete(t *testing.T) {
	fileStore := storage.NewLocalObjectStore(storage.LocalObjectStoreOptions{
		BaseDir: t.TempDir(),
	})
	server := newTestServer(ServerOptions{
		Capabilities: capabilitiesFromConfigForTest(t, []string{"tenant", "identity", "session", "rbac", "menu", "audit", "dictionary", "parameter", "file-storage", "admin-shell"}),
		FileStorage:  fileStore,
	})
	body, contentType := multipartUploadBody(t, "report.txt", "hello file storage")
	uploadRecorder := httptest.NewRecorder()
	uploadRequest := httptest.NewRequest(http.MethodPost, "/api/admin/files/upload", body)
	uploadRequest.Header.Set("Content-Type", contentType)

	server.Router().ServeHTTP(uploadRecorder, uploadRequest)

	if uploadRecorder.Code != http.StatusCreated {
		t.Fatalf("POST file upload status = %d body = %s", uploadRecorder.Code, uploadRecorder.Body.String())
	}
	var uploaded adminResourceRecordTestPayload
	if err := json.Unmarshal(uploadRecorder.Body.Bytes(), &uploaded); err != nil {
		t.Fatalf("decode upload response: %v body = %s", err, uploadRecorder.Body.String())
	}
	record := uploaded.Data.Record
	if record.ID == "" || record.Name != "report.txt" || record.Values["storageKey"] != "" || record.Values["storagePath"] != "" || record.Values["publicUrl"] != "" || record.Values["size"] != "18" {
		t.Fatalf("uploaded record mismatch: %+v", record)
	}
	storedRecord, err := server.adminResourceRecordByID("files", record.ID)
	if err != nil {
		t.Fatalf("read stored file record: %v", err)
	}
	storageKey := storedRecord.Values["storageKey"]
	if storageKey == "" {
		t.Fatal("stored file record missing internal storage key")
	}
	if storedRecord.Values["storagePath"] != "" || storedRecord.Values["publicUrl"] != "" {
		t.Fatalf("stored file record contains physical/public location: %+v", storedRecord.Values)
	}

	contentRecorder := httptest.NewRecorder()
	contentRequest := httptest.NewRequest(http.MethodGet, "/api/admin/files/"+record.ID+"/content", nil)
	server.Router().ServeHTTP(contentRecorder, contentRequest)

	if contentRecorder.Code != http.StatusOK {
		t.Fatalf("GET file content status = %d body = %s", contentRecorder.Code, contentRecorder.Body.String())
	}
	if contentRecorder.Body.String() != "hello file storage" {
		t.Fatalf("file content = %q", contentRecorder.Body.String())
	}

	deleteRecorder := httptest.NewRecorder()
	deleteRequest := httptest.NewRequest(http.MethodDelete, "/api/admin/resources/files/"+record.ID, nil)
	server.Router().ServeHTTP(deleteRecorder, deleteRequest)

	if deleteRecorder.Code != http.StatusOK {
		t.Fatalf("DELETE file status = %d body = %s", deleteRecorder.Code, deleteRecorder.Body.String())
	}
	if _, err := fileStore.Open(deleteRequest.Context(), storageKey); !errors.Is(err, storage.ErrObjectNotFound) {
		t.Fatalf("Open(deleted file object) error = %v, want ErrObjectNotFound", err)
	}

	auditRequest := httptest.NewRequest(http.MethodGet, "/api/admin/resources/audit-logs", nil)
	auditRecorder := httptest.NewRecorder()
	server.Router().ServeHTTP(auditRecorder, auditRequest)
	if auditRecorder.Code != http.StatusOK {
		t.Fatalf("GET audit-logs status = %d body = %s", auditRecorder.Code, auditRecorder.Body.String())
	}
	var audits adminResourceListTestPayload
	if err := json.Unmarshal(auditRecorder.Body.Bytes(), &audits); err != nil {
		t.Fatalf("decode audit logs: %v body = %s", err, auditRecorder.Body.String())
	}
	for _, action := range []string{"file.upload", "file.content", "file.delete"} {
		if !hasAdminResourceAuditRecord(audits.Data.Items, action, "files", record.ID, record.Code, "admin") {
			t.Fatalf("audit logs missing %s for uploaded file: %+v", action, audits.Data.Items)
		}
	}
	for _, audit := range audits.Data.Items {
		if audit.Values["resource"] == "files" && audit.Values["targetName"] != "" {
			t.Fatalf("file audit retained targetName: %+v", audit)
		}
	}
}

type recordingObjectStore struct {
	saveCalls   int
	deleteCalls int
	deleteErr   error
}

func (store *recordingObjectStore) Save(_ context.Context, input storage.ObjectSaveInput) (storage.ObjectMetadata, error) {
	store.saveCalls++
	content, err := io.ReadAll(input.Reader)
	if err != nil {
		return storage.ObjectMetadata{}, err
	}
	return storage.ObjectMetadata{Driver: "recording", Key: "private/object", SizeBytes: int64(len(content))}, nil
}

func (*recordingObjectStore) Open(context.Context, string) (io.ReadCloser, error) {
	return nil, storage.ErrObjectNotFound
}

func (store *recordingObjectStore) Delete(context.Context, string) error {
	store.deleteCalls++
	return store.deleteErr
}

func TestAdminFileUploadRejectsOversizeBeforeObjectCreation(t *testing.T) {
	fileStore := &recordingObjectStore{}
	server := newTestServer(ServerOptions{
		Capabilities: capabilitiesFromConfigForTest(t, []string{"tenant", "identity", "session", "rbac", "menu", "audit", "dictionary", "parameter", "file-storage", "admin-shell"}),
		FileStorage:  fileStore,
		UploadPolicy: UploadPolicy{
			MaxBytes:          128,
			AllowedMediaTypes: map[string]struct{}{"text/plain": {}},
		},
	})
	body, contentType := multipartUploadBodyWithMediaType(t, "report.txt", "text/plain", strings.Repeat("x", 512))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/admin/files/upload", body)
	request.Header.Set("Content-Type", contentType)

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("POST oversized file status = %d body = %s, want 413", recorder.Code, recorder.Body.String())
	}
	if fileStore.saveCalls != 0 {
		t.Fatalf("object store Save calls = %d, want 0", fileStore.saveCalls)
	}
}

func TestAdminFileUploadHonorsFileSizeBoundary(t *testing.T) {
	const maxBytes = int64(64)
	for _, tt := range []struct {
		name       string
		size       int
		wantStatus int
		wantSaves  int
	}{
		{name: "exact limit", size: int(maxBytes), wantStatus: http.StatusCreated, wantSaves: 1},
		{name: "one byte over", size: int(maxBytes) + 1, wantStatus: http.StatusRequestEntityTooLarge, wantSaves: 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fileStore := &recordingObjectStore{}
			server := newTestServer(ServerOptions{
				Capabilities: capabilitiesFromConfigForTest(t, []string{"tenant", "identity", "session", "rbac", "menu", "audit", "dictionary", "parameter", "file-storage", "admin-shell"}),
				FileStorage:  fileStore,
				UploadPolicy: UploadPolicy{MaxBytes: maxBytes, AllowedMediaTypes: map[string]struct{}{"text/plain": {}}},
			})
			body, contentType := multipartUploadBodyWithMediaType(t, "report.txt", "text/plain", strings.Repeat("x", tt.size))
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/api/admin/files/upload", body)
			request.Header.Set("Content-Type", contentType)

			server.Router().ServeHTTP(recorder, request)

			if recorder.Code != tt.wantStatus {
				t.Fatalf("POST boundary file status = %d body = %s, want %d", recorder.Code, recorder.Body.String(), tt.wantStatus)
			}
			if fileStore.saveCalls != tt.wantSaves {
				t.Fatalf("object store Save calls = %d, want %d", fileStore.saveCalls, tt.wantSaves)
			}
		})
	}
}

func TestAdminFileUploadDeletesObjectWhenMetadataCreateFails(t *testing.T) {
	capabilities := capabilitiesFromConfigForTest(t, []string{"tenant", "identity", "session", "rbac", "menu", "audit", "dictionary", "parameter", "file-storage", "admin-shell"})
	repository := &controllableAdminResourceRepository{}
	resources, err := adminresource.NewRepositoryBackedStoreFromCapabilities(repository, capabilities)
	if err != nil {
		t.Fatalf("NewRepositoryBackedStoreFromCapabilities() error = %v", err)
	}
	repository.saveErr = errors.New("metadata persistence failed")
	fileStore := &recordingObjectStore{}
	server := newTestServer(ServerOptions{Capabilities: capabilities, Resources: resources, FileStorage: fileStore})
	body, contentType := multipartUploadBodyWithMediaType(t, "report.txt", "text/plain", "private")
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/admin/files/upload", body)
	request.Header.Set("Content-Type", contentType)

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("POST file with metadata failure status = %d body = %s, want 500", recorder.Code, recorder.Body.String())
	}
	if fileStore.saveCalls != 1 || fileStore.deleteCalls != 1 {
		t.Fatalf("object store calls save/delete = %d/%d, want 1/1", fileStore.saveCalls, fileStore.deleteCalls)
	}
}

func TestAdminFileUploadReportsRollbackFailureWithoutLeakingDetails(t *testing.T) {
	capabilities := capabilitiesFromConfigForTest(t, []string{"tenant", "identity", "session", "rbac", "menu", "audit", "dictionary", "parameter", "file-storage", "admin-shell"})
	repository := &controllableAdminResourceRepository{}
	resources, err := adminresource.NewRepositoryBackedStoreFromCapabilities(repository, capabilities)
	if err != nil {
		t.Fatalf("NewRepositoryBackedStoreFromCapabilities() error = %v", err)
	}
	repository.saveErr = errors.New("metadata-private-detail")
	fileStore := &recordingObjectStore{deleteErr: errors.New("delete-private/object-secret")}
	server := newTestServer(ServerOptions{Capabilities: capabilities, Resources: resources, FileStorage: fileStore})
	body, contentType := multipartUploadBodyWithMediaType(t, "report.txt", "text/plain", "private")
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/admin/files/upload", body)
	request.Header.Set("Content-Type", contentType)

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError || !strings.Contains(recorder.Body.String(), `"code":"ADMIN_FILE_ROLLBACK_FAILED"`) {
		t.Fatalf("POST file rollback failure status/body = %d/%s", recorder.Code, recorder.Body.String())
	}
	for _, secret := range []string{"metadata-private-detail", "delete-private", "private/object"} {
		if strings.Contains(recorder.Body.String(), secret) {
			t.Fatalf("rollback response leaked %q: %s", secret, recorder.Body.String())
		}
	}
	if fileStore.saveCalls != 1 || fileStore.deleteCalls != 1 {
		t.Fatalf("object store calls save/delete = %d/%d, want 1/1", fileStore.saveCalls, fileStore.deleteCalls)
	}
}

func TestAppFileUploadReportsRollbackFailureWithoutLeakingDetails(t *testing.T) {
	capabilities := capabilitiesFromConfigForTest(t, []string{"dictionary", "tenant", "identity", "session", "rbac", "menu", "audit", "parameter", "file-storage", "admin-shell"})
	repository := &controllableAdminResourceRepository{}
	resources, err := adminresource.NewRepositoryBackedStoreFromCapabilities(repository, capabilities)
	if err != nil {
		t.Fatalf("NewRepositoryBackedStoreFromCapabilities() error = %v", err)
	}
	fileStore := &recordingObjectStore{deleteErr: errors.New("delete-private/object-secret")}
	server := newTestServer(ServerOptions{Capabilities: capabilities, Resources: resources, FileStorage: fileStore})
	login := appLoginForTest(t, server, "buyer")
	repository.saveErr = errors.New("metadata-private-detail")
	body, contentType := multipartUploadBodyWithMediaType(t, "report.txt", "text/plain", "private")
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/app/files", body)
	request.Header.Set("Authorization", "Bearer "+login.Data.Token)
	request.Header.Set("Content-Type", contentType)

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError || !strings.Contains(recorder.Body.String(), `"code":"APP_FILE_ROLLBACK_FAILED"`) {
		t.Fatalf("POST app file rollback failure status/body = %d/%s", recorder.Code, recorder.Body.String())
	}
	for _, secret := range []string{"metadata-private-detail", "delete-private", "private/object"} {
		if strings.Contains(recorder.Body.String(), secret) {
			t.Fatalf("app rollback response leaked %q: %s", secret, recorder.Body.String())
		}
	}
	if fileStore.saveCalls != 1 || fileStore.deleteCalls != 1 {
		t.Fatalf("object store calls save/delete = %d/%d, want 1/1", fileStore.saveCalls, fileStore.deleteCalls)
	}
}

func TestAdminFileUploadRejectsSpoofedOrDisallowedMIME(t *testing.T) {
	tests := []struct {
		name     string
		declared string
		content  string
		allowed  map[string]struct{}
	}{
		{
			name:     "spoofed declaration",
			declared: "image/png",
			content:  "plain text payload",
			allowed:  map[string]struct{}{"image/png": {}, "text/plain": {}},
		},
		{
			name:     "disallowed detected type",
			declared: "application/pdf",
			content:  "%PDF-1.7\n",
			allowed:  map[string]struct{}{"text/plain": {}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fileStore := &recordingObjectStore{}
			server := newTestServer(ServerOptions{
				Capabilities: capabilitiesFromConfigForTest(t, []string{"tenant", "identity", "session", "rbac", "menu", "audit", "dictionary", "parameter", "file-storage", "admin-shell"}),
				FileStorage:  fileStore,
				UploadPolicy: UploadPolicy{MaxBytes: 1 << 20, AllowedMediaTypes: tt.allowed},
			})
			body, contentType := multipartUploadBodyWithMediaType(t, "upload.bin", tt.declared, tt.content)
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/api/admin/files/upload", body)
			request.Header.Set("Content-Type", contentType)

			server.Router().ServeHTTP(recorder, request)

			if recorder.Code != http.StatusUnsupportedMediaType {
				t.Fatalf("POST invalid MIME status = %d body = %s, want 415", recorder.Code, recorder.Body.String())
			}
			if fileStore.saveCalls != 0 {
				t.Fatalf("object store Save calls = %d, want 0", fileStore.saveCalls)
			}
		})
	}
}

func TestAdminFileDeleteDoesNotAuditFailedObjectDeletion(t *testing.T) {
	fileStore := storage.NewLocalObjectStore(storage.LocalObjectStoreOptions{
		BaseDir: t.TempDir(),
	})
	server := newTestServer(ServerOptions{
		Capabilities: capabilitiesFromConfigForTest(t, []string{"tenant", "identity", "session", "rbac", "menu", "audit", "dictionary", "parameter", "file-storage", "admin-shell"}),
		FileStorage:  fileStore,
	})
	body, contentType := multipartUploadBody(t, "report.txt", "hello")
	uploadRecorder := httptest.NewRecorder()
	uploadRequest := httptest.NewRequest(http.MethodPost, "/api/admin/files/upload", body)
	uploadRequest.Header.Set("Content-Type", contentType)
	server.Router().ServeHTTP(uploadRecorder, uploadRequest)
	if uploadRecorder.Code != http.StatusCreated {
		t.Fatalf("POST file upload status = %d body = %s", uploadRecorder.Code, uploadRecorder.Body.String())
	}
	var uploaded adminResourceRecordTestPayload
	if err := json.Unmarshal(uploadRecorder.Body.Bytes(), &uploaded); err != nil {
		t.Fatalf("decode upload response: %v body = %s", err, uploadRecorder.Body.String())
	}
	record := uploaded.Data.Record
	storedRecord, err := server.adminResourceRecordByID("files", record.ID)
	if err != nil {
		t.Fatalf("read stored file record: %v", err)
	}
	if err := fileStore.Delete(uploadRequest.Context(), storedRecord.Values["storageKey"]); err != nil {
		t.Fatalf("Delete(uploaded object before API delete) error = %v", err)
	}

	deleteRecorder := httptest.NewRecorder()
	deleteRequest := httptest.NewRequest(http.MethodDelete, "/api/admin/resources/files/"+record.ID, nil)
	server.Router().ServeHTTP(deleteRecorder, deleteRequest)

	if deleteRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("DELETE file with missing object status = %d body = %s, want 500", deleteRecorder.Code, deleteRecorder.Body.String())
	}
	auditRequest := httptest.NewRequest(http.MethodGet, "/api/admin/resources/audit-logs", nil)
	auditRecorder := httptest.NewRecorder()
	server.Router().ServeHTTP(auditRecorder, auditRequest)
	if auditRecorder.Code != http.StatusOK {
		t.Fatalf("GET audit-logs status = %d body = %s", auditRecorder.Code, auditRecorder.Body.String())
	}
	var audits adminResourceListTestPayload
	if err := json.Unmarshal(auditRecorder.Body.Bytes(), &audits); err != nil {
		t.Fatalf("decode audit logs: %v body = %s", err, auditRecorder.Body.String())
	}
	if hasAdminResourceAuditRecord(audits.Data.Items, "file.delete", "files", record.ID, record.Code, "admin") {
		t.Fatalf("audit logs recorded failed file delete: %+v", audits.Data.Items)
	}
}

func TestAdminFileUploadRequiresEnabledFileStorageCapability(t *testing.T) {
	server := newTestServer(ServerOptions{Capabilities: capabilitiesFromConfigForTest(t, []string{"dictionary", "tenant", "identity", "session", "rbac", "menu", "admin-shell"})})
	body, contentType := multipartUploadBody(t, "report.txt", "hello")
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/admin/files/upload", body)
	request.Header.Set("Content-Type", contentType)

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("POST file upload without capability status = %d body = %s, want 404", recorder.Code, recorder.Body.String())
	}
}

func TestAppFileUploadAndContentUseAppSession(t *testing.T) {
	fileStore := storage.NewLocalObjectStore(storage.LocalObjectStoreOptions{
		BaseDir: t.TempDir(),
	})
	server := newTestServer(ServerOptions{
		Capabilities: capabilitiesFromConfigForTest(t, []string{"dictionary", "tenant", "identity", "session", "rbac", "menu", "audit", "parameter", "file-storage", "admin-shell"}),
		FileStorage:  fileStore,
	})
	login := appLoginForTest(t, server, "buyer")

	body, contentType := multipartUploadBody(t, "avatar.png", "hello app file")
	uploadRecorder := httptest.NewRecorder()
	uploadRequest := httptest.NewRequest(http.MethodPost, "/api/app/files", body)
	uploadRequest.Header.Set("Authorization", "Bearer "+login.Data.Token)
	uploadRequest.Header.Set("Content-Type", contentType)

	server.Router().ServeHTTP(uploadRecorder, uploadRequest)

	if uploadRecorder.Code != http.StatusCreated {
		t.Fatalf("POST app file upload status = %d body = %s", uploadRecorder.Code, uploadRecorder.Body.String())
	}
	var uploaded adminResourceRecordTestPayload
	if err := json.Unmarshal(uploadRecorder.Body.Bytes(), &uploaded); err != nil {
		t.Fatalf("decode app upload response: %v body = %s", err, uploadRecorder.Body.String())
	}
	record := uploaded.Data.Record
	if record.ID == "" || record.Name != "avatar.png" || record.Values["storageKey"] != "" || record.Values["tenantId"] != "" || record.Values["sessionId"] != "" || record.Values["uploadedBy"] != "buyer" {
		t.Fatalf("app uploaded record mismatch: %+v", record)
	}
	storedRecord, err := server.adminResourceRecordByID("files", record.ID)
	if err != nil {
		t.Fatalf("read stored app file record: %v", err)
	}
	if storedRecord.Values["storagePath"] != "" || storedRecord.Values["publicUrl"] != "" || storedRecord.Values["sessionId"] != "" {
		t.Fatalf("stored app file record contains prohibited location/session data: %+v", storedRecord.Values)
	}

	contentRecorder := httptest.NewRecorder()
	contentRequest := httptest.NewRequest(http.MethodGet, "/api/app/files/"+record.ID+"/content", nil)
	contentRequest.Header.Set("Authorization", "Bearer "+login.Data.Token)
	server.Router().ServeHTTP(contentRecorder, contentRequest)

	if contentRecorder.Code != http.StatusOK {
		t.Fatalf("GET app file content status = %d body = %s", contentRecorder.Code, contentRecorder.Body.String())
	}
	if contentRecorder.Body.String() != "hello app file" {
		t.Fatalf("app file content = %q", contentRecorder.Body.String())
	}

	otherLogin := appLoginForTest(t, server, "other")
	otherRecorder := httptest.NewRecorder()
	otherRequest := httptest.NewRequest(http.MethodGet, "/api/app/files/"+record.ID+"/content", nil)
	otherRequest.Header.Set("Authorization", "Bearer "+otherLogin.Data.Token)
	server.Router().ServeHTTP(otherRecorder, otherRequest)

	if otherRecorder.Code != http.StatusNotFound {
		t.Fatalf("GET app file content from another app user status = %d body = %s, want 404", otherRecorder.Code, otherRecorder.Body.String())
	}
}

func TestAppFileMetadataOmitsSessionPathAndPublicURL(t *testing.T) {
	fileStore := storage.NewLocalObjectStore(storage.LocalObjectStoreOptions{BaseDir: t.TempDir()})
	server := newTestServer(ServerOptions{
		Capabilities: capabilitiesFromConfigForTest(t, []string{"dictionary", "tenant", "identity", "session", "rbac", "menu", "audit", "parameter", "file-storage", "admin-shell"}),
		FileStorage:  fileStore,
	})
	login := appLoginForTest(t, server, "buyer")
	body, contentType := multipartUploadBodyWithMediaType(t, "../../unsafe report.txt", "text/plain", "private")
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/app/files", body)
	request.Header.Set("Authorization", "Bearer "+login.Data.Token)
	request.Header.Set("Content-Type", contentType)

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("POST app file status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	var uploaded adminResourceRecordTestPayload
	if err := json.Unmarshal(recorder.Body.Bytes(), &uploaded); err != nil {
		t.Fatalf("decode app upload response: %v", err)
	}
	if uploaded.Data.Record.Name != "unsafe_report.txt" {
		t.Fatalf("uploaded filename = %q, want sanitized filename", uploaded.Data.Record.Name)
	}
	for _, prohibited := range []string{"sessionId", "storagePath", "publicUrl", "storageKey", "tenantId"} {
		if strings.Contains(recorder.Body.String(), prohibited) {
			t.Fatalf("app file response contains prohibited field %q: %s", prohibited, recorder.Body.String())
		}
	}
	stored, err := server.adminResourceRecordByID("files", uploaded.Data.Record.ID)
	if err != nil {
		t.Fatalf("read stored app file: %v", err)
	}
	for _, prohibited := range []string{"sessionId", "storagePath", "publicUrl"} {
		if stored.Values[prohibited] != "" {
			t.Fatalf("stored app file contains %s: %+v", prohibited, stored.Values)
		}
	}
	if stored.Values["ownerId"] != appUserID("buyer") {
		t.Fatalf("stored app file ownerId = %q, want stable app user ID", stored.Values["ownerId"])
	}
}

func TestAppFileContentReturnsNotFoundForCrossUser(t *testing.T) {
	server := newTestServer(ServerOptions{
		Capabilities: capabilitiesFromConfigForTest(t, []string{"dictionary", "tenant", "identity", "session", "rbac", "menu", "audit", "parameter", "file-storage", "admin-shell"}),
		FileStorage:  storage.NewLocalObjectStore(storage.LocalObjectStoreOptions{BaseDir: t.TempDir()}),
	})
	owner := appLoginForTest(t, server, "owner")
	body, contentType := multipartUploadBodyWithMediaType(t, "private.txt", "text/plain", "private")
	uploadRecorder := httptest.NewRecorder()
	uploadRequest := httptest.NewRequest(http.MethodPost, "/api/app/files", body)
	uploadRequest.Header.Set("Authorization", "Bearer "+owner.Data.Token)
	uploadRequest.Header.Set("Content-Type", contentType)
	server.Router().ServeHTTP(uploadRecorder, uploadRequest)
	if uploadRecorder.Code != http.StatusCreated {
		t.Fatalf("POST owner file status = %d body = %s", uploadRecorder.Code, uploadRecorder.Body.String())
	}
	var uploaded adminResourceRecordTestPayload
	if err := json.Unmarshal(uploadRecorder.Body.Bytes(), &uploaded); err != nil {
		t.Fatalf("decode owner upload response: %v", err)
	}
	stored, err := server.adminResourceRecordByID("files", uploaded.Data.Record.ID)
	if err != nil {
		t.Fatalf("read stored owner file: %v", err)
	}
	stored.Values["uploadedBy"] = "other"
	if _, err := server.resources.UpdateInternal("files", stored.ID, adminresource.WriteInput{
		Code: stored.Code, Name: stored.Name, Status: stored.Status, Description: stored.Description, Values: stored.Values,
	}); err != nil {
		t.Fatalf("update display uploader: %v", err)
	}

	other := appLoginForTest(t, server, "other")
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/app/files/"+uploaded.Data.Record.ID+"/content", nil)
	request.Header.Set("Authorization", "Bearer "+other.Data.Token)
	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("GET cross-user file status = %d body = %s, want 404", recorder.Code, recorder.Body.String())
	}
}

func TestAppFileUploadRequiresEnabledFileStorageCapability(t *testing.T) {
	server := newTestServer(ServerOptions{Capabilities: capabilitiesFromConfigForTest(t, []string{"dictionary", "tenant", "identity", "session", "rbac", "menu", "admin-shell"})})
	login := appLoginForTest(t, server, "buyer")
	body, contentType := multipartUploadBody(t, "avatar.png", "hello")
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/app/files", body)
	request.Header.Set("Authorization", "Bearer "+login.Data.Token)
	request.Header.Set("Content-Type", contentType)

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("POST app file upload without capability status = %d body = %s, want 404", recorder.Code, recorder.Body.String())
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func multipartUploadBody(t *testing.T, filename string, content string) (*bytes.Buffer, string) {
	return multipartUploadBodyWithMediaType(t, filename, http.DetectContentType([]byte(content)), content)
}

func multipartUploadBodyWithMediaType(t *testing.T, filename string, mediaType string, content string) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{"name": "file", "filename": filename}))
	header.Set("Content-Type", mediaType)
	part, err := writer.CreatePart(header)
	if err != nil {
		t.Fatalf("CreateFormFile() error = %v", err)
	}
	if _, err := part.Write([]byte(content)); err != nil {
		t.Fatalf("write multipart content error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("multipart Close() error = %v", err)
	}
	return body, writer.FormDataContentType()
}

func hasTestRecordID(records []adminResourceRecordTest, id string) bool {
	for _, record := range records {
		if record.ID == id {
			return true
		}
	}
	return false
}

func countTestRecordID(records []adminResourceRecordTest, id string) int {
	count := 0
	for _, record := range records {
		if record.ID == id {
			count++
		}
	}
	return count
}

func hasTestRecordCode(records []adminResourceRecordTest, code string) bool {
	for _, record := range records {
		if record.Code == code {
			return true
		}
	}
	return false
}

func hasTestRecordAction(records []adminResourceRecordTest, action string) bool {
	for _, record := range records {
		if record.Values["action"] == action {
			return true
		}
	}
	return false
}

func hasTestRecordName(records []adminResourceRecordTest, name string) bool {
	for _, record := range records {
		if record.Name == name {
			return true
		}
	}
	return false
}

func hasAdminResourceAuditRecord(records []adminResourceRecordTest, action string, resource string, targetID string, targetCode string, actor string) bool {
	for _, record := range records {
		if record.Values["action"] == action &&
			record.Values["resource"] == resource &&
			record.Values["targetId"] == targetID &&
			record.Values["targetCode"] == targetCode &&
			record.Values["actor"] == actor {
			return true
		}
	}
	return false
}

func capabilitiesFromConfigForTest(t *testing.T, enabled []string) []capability.Manifest {
	t.Helper()
	return resolvedCapabilitiesForTest(t, enabled, nil)
}

func capabilitiesFromConfigWithAppsForTest(t *testing.T, enabled []string) []capability.Manifest {
	t.Helper()
	return resolvedCapabilitiesForTest(t, enabled, apps.DefaultManifests())
}

func resolvedCapabilitiesForTest(t *testing.T, enabled []string, additional []capability.Manifest) []capability.Manifest {
	t.Helper()
	registry := capability.NewRegistry()
	for _, manifest := range append(core.DefaultManifests(), additional...) {
		if err := registry.Register(manifest); err != nil {
			t.Fatalf("Register(%s) error = %v", manifest.ID, err)
		}
	}
	ids := make([]capability.ID, 0, len(enabled))
	for _, id := range enabled {
		ids = append(ids, capability.ID(id))
	}
	manifests, err := registry.ResolveEnabled(ids)
	if err != nil {
		t.Fatalf("ResolveEnabled() error = %v", err)
	}
	return manifests
}

func configuredWechatPlatformManifests(t *testing.T) []capability.Manifest {
	t.Helper()
	manifests := capabilitiesFromConfigForTest(t, []string{"dictionary", "tenant", "identity", "session", "rbac", "audit", "wechat-login"})
	for manifestIndex := range manifests {
		for providerIndex := range manifests[manifestIndex].AuthProviders {
			if manifests[manifestIndex].AuthProviders[providerIndex].ID == "wechat" {
				manifests[manifestIndex].AuthProviders[providerIndex].Configured = true
			}
		}
	}
	return manifests
}

func loginForTest(t *testing.T, server *Server, username string) authLoginTestPayload {
	t.Helper()
	loginBody := bytes.NewBufferString(`{"provider":"demo","username":"` + username + `"}`)
	loginRecorder := httptest.NewRecorder()
	loginRequest := httptest.NewRequest(http.MethodPost, "/api/auth/login", loginBody)
	loginRequest.Header.Set("Content-Type", "application/json")

	server.Router().ServeHTTP(loginRecorder, loginRequest)

	if loginRecorder.Code != http.StatusOK {
		t.Fatalf("POST auth login status = %d body = %s", loginRecorder.Code, loginRecorder.Body.String())
	}
	var login authLoginTestPayload
	if err := json.Unmarshal(loginRecorder.Body.Bytes(), &login); err != nil {
		t.Fatalf("decode auth login: %v body = %s", err, loginRecorder.Body.String())
	}
	return login
}

func appWechatLoginForTest(t *testing.T, server *Server, code string) appLoginTestPayload {
	t.Helper()
	loginBody := bytes.NewBufferString(`{"provider":"wechat","code":"` + code + `"}`)
	loginRecorder := httptest.NewRecorder()
	loginRequest := httptest.NewRequest(http.MethodPost, "/api/app/auth/login", loginBody)
	loginRequest.Header.Set("Content-Type", "application/json")

	server.Router().ServeHTTP(loginRecorder, loginRequest)

	if loginRecorder.Code != http.StatusOK {
		t.Fatalf("POST app wechat auth login status = %d body = %s", loginRecorder.Code, loginRecorder.Body.String())
	}
	var login appLoginTestPayload
	if err := json.Unmarshal(loginRecorder.Body.Bytes(), &login); err != nil {
		t.Fatalf("decode app wechat auth login: %v body = %s", err, loginRecorder.Body.String())
	}
	return login
}

func appLoginForTest(t *testing.T, server *Server, username string) appLoginTestPayload {
	t.Helper()
	loginBody := bytes.NewBufferString(`{"username":"` + username + `"}`)
	loginRecorder := httptest.NewRecorder()
	loginRequest := httptest.NewRequest(http.MethodPost, "/api/app/auth/login", loginBody)
	loginRequest.Header.Set("Content-Type", "application/json")

	server.Router().ServeHTTP(loginRecorder, loginRequest)

	if loginRecorder.Code != http.StatusOK {
		t.Fatalf("POST app auth login status = %d body = %s", loginRecorder.Code, loginRecorder.Body.String())
	}
	var login appLoginTestPayload
	if err := json.Unmarshal(loginRecorder.Body.Bytes(), &login); err != nil {
		t.Fatalf("decode app auth login: %v body = %s", err, loginRecorder.Body.String())
	}
	return login
}

func createAppPhoneVerificationForTest(t *testing.T, server *Server, token string, phone string) appPhoneVerificationTestPayload {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/app/identity/phone-verifications", bytes.NewBufferString(`{"phone":"`+phone+`","purpose":"bind"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("POST app phone verification status = %d body = %s, want 201", recorder.Code, recorder.Body.String())
	}
	var payload appPhoneVerificationTestPayload
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode app phone verification: %v body = %s", err, recorder.Body.String())
	}
	return payload
}

func createAppPhoneBindingForTest(t *testing.T, server *Server, token string, phone string, code string) appPhoneBindingTestPayload {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/app/identity/phone-bindings", bytes.NewBufferString(`{"phone":"`+phone+`","code":"`+code+`"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("POST app phone binding status = %d body = %s, want 201", recorder.Code, recorder.Body.String())
	}
	var payload appPhoneBindingTestPayload
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode app phone binding: %v body = %s", err, recorder.Body.String())
	}
	return payload
}

func demoDataTestManifest() capability.Manifest {
	return capability.Manifest{
		ID: "demo-seed",
		Admin: capability.AdminSurface{
			Resources: []capability.AdminResource{
				{
					Resource:         "tenants",
					Title:            capability.Text("租户", "Tenants"),
					Description:      capability.Text("租户空间。", "Tenant spaces."),
					PermissionPrefix: "admin:tenant",
					Menu:             capability.AdminMenu{Route: "/tenants", Parent: "identity", Group: "foundation", Icon: "tenants", Order: 30},
				},
			},
		},
		DemoData: []capability.DemoDataSet{
			{
				ID:          "demo-tenants",
				Title:       capability.Text("演示租户", "Demo Tenants"),
				Description: capability.Text("租户演示数据。", "Tenant demo data."),
				Resource:    "tenants",
				Records: []capability.DemoRecord{
					{
						ID:          "tenant-demo-acme",
						Code:        "demo-acme",
						Name:        "Demo Acme Tenant",
						Status:      "enabled",
						Description: "Demo tenant applied from capability demo data.",
						Values:      map[string]string{"isolation": "sandbox"},
					},
				},
			},
		},
	}
}

func authProviderTestManifest() capability.Manifest {
	return capability.Manifest{
		ID: "auth-test",
		App: capability.AppSurface{Routes: []capability.AppRoute{
			{
				Method:      http.MethodPost,
				Path:        "/api/app/auth/login",
				Auth:        capability.AppRouteAuthPublic,
				Description: capability.Text("App 登录。", "App login."),
			},
			{
				Method:      http.MethodGet,
				Path:        "/api/app/session/current",
				Auth:        capability.AppRouteAuthSession,
				Description: capability.Text("读取 App 当前会话。", "Read current app session."),
			},
			{
				Method:      http.MethodPost,
				Path:        "/api/app/auth/logout",
				Auth:        capability.AppRouteAuthSession,
				Description: capability.Text("退出 App 会话。", "Log out app session."),
			},
		}},
		AuthProviders: []capability.AuthProvider{
			{
				ID:          "demo",
				Kind:        "demo",
				Title:       capability.Text("演示登录", "Demo Login"),
				Description: capability.Text("本地开发演示账号登录。", "Local demo account login."),
				Enabled:     true,
				Configured:  true,
				Audiences:   []capability.AuthProviderAudience{capability.AuthProviderAudienceAdmin, capability.AuthProviderAudienceApp},
			},
			{
				ID:          "wechat",
				Kind:        "wechat",
				Title:       capability.Text("微信登录", "WeChat Login"),
				Description: capability.Text("微信 code 换取登录态。", "WeChat code exchange login."),
				Enabled:     true,
				Configured:  false,
				Audiences:   []capability.AuthProviderAudience{capability.AuthProviderAudienceApp},
			},
		},
	}
}

type controllableSessionRepository struct {
	sessions   map[string]session.StoredSession
	resolveErr error
	renewErr   error
	revokeErr  error
}

type controllableAdminResourceRepository struct {
	snapshot adminresource.ResourceSnapshot
	loadErr  error
	saveErr  error
}

func (r *controllableAdminResourceRepository) Load(context.Context) (adminresource.ResourceSnapshot, error) {
	if r.loadErr != nil {
		return adminresource.ResourceSnapshot{}, r.loadErr
	}
	return r.snapshot, nil
}

func (r *controllableAdminResourceRepository) Save(_ context.Context, snapshot adminresource.ResourceSnapshot) (uint64, error) {
	if r.saveErr != nil {
		return 0, r.saveErr
	}
	r.snapshot = snapshot
	r.snapshot.Revision++
	return r.snapshot.Revision, nil
}

func newControllableSessionRepository() *controllableSessionRepository {
	return &controllableSessionRepository{sessions: map[string]session.StoredSession{}}
}

func (r *controllableSessionRepository) Load(context.Context) (session.Snapshot, error) {
	return session.Snapshot{Sessions: cloneTestSessions(r.sessions)}, nil
}

func (r *controllableSessionRepository) Create(_ context.Context, created session.StoredSession) error {
	r.sessions[created.TokenDigest] = created
	return nil
}

func (r *controllableSessionRepository) Resolve(_ context.Context, tokenDigest string, now time.Time) (session.StoredSession, bool, error) {
	if r.resolveErr != nil {
		return session.StoredSession{}, false, r.resolveErr
	}
	current, ok := r.sessions[tokenDigest]
	if !ok || !current.RevokedAt.IsZero() || !now.Before(current.ExpiresAt) {
		return session.StoredSession{}, false, nil
	}
	return current, true, nil
}

func newAdminSessionWriteTestServer(t *testing.T) (*Server, *controllableSessionRepository, string) {
	t.Helper()
	now := time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC)
	repository := newControllableSessionRepository()
	sessions, err := session.NewRepositoryBackedStore(session.Options{TTL: time.Hour, Now: func() time.Time { return now }}, repository)
	if err != nil {
		t.Fatalf("NewRepositoryBackedStore() error = %v", err)
	}
	server := newTestServer(ServerOptions{
		Capabilities: []capability.Manifest{authProviderTestManifest()},
		Sessions:     sessions,
		Now:          func() time.Time { return now },
	})
	return server, repository, loginForTest(t, server, "ops").Data.Token
}

func newAppSessionWriteTestServer(t *testing.T) (*Server, *controllableSessionRepository, string) {
	t.Helper()
	now := time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC)
	repository := newControllableSessionRepository()
	sessions, err := session.NewRepositoryBackedStore(session.Options{TTL: time.Hour, Now: func() time.Time { return now }}, repository)
	if err != nil {
		t.Fatalf("NewRepositoryBackedStore() error = %v", err)
	}
	server := newTestServer(ServerOptions{
		Capabilities: []capability.Manifest{authProviderTestManifest()},
		Sessions:     sessions,
		Now:          func() time.Time { return now },
	})
	return server, repository, appLoginForTest(t, server, "guest-alpha").Data.Token
}

func performBearerRequest(server *Server, method string, path string, token string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, nil)
	request.Header.Set("Authorization", "Bearer "+token)
	server.Router().ServeHTTP(recorder, request)
	return recorder
}

func assertAuthErrorResponse(t *testing.T, recorder *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if recorder.Code != status {
		t.Fatalf("auth response status = %d body = %s, want %d", recorder.Code, recorder.Body.String(), status)
	}
	if !strings.Contains(recorder.Body.String(), `"code":"`+code+`"`) {
		t.Fatalf("auth response body = %s, want %s", recorder.Body.String(), code)
	}
}

func (r *controllableSessionRepository) Renew(_ context.Context, tokenDigest string, now time.Time, expiresAt time.Time) (session.StoredSession, bool, error) {
	if r.renewErr != nil {
		return session.StoredSession{}, false, r.renewErr
	}
	current, ok, err := r.Resolve(context.Background(), tokenDigest, now)
	if err != nil || !ok {
		return session.StoredSession{}, ok, err
	}
	current.ExpiresAt = expiresAt
	r.sessions[tokenDigest] = current
	return current, true, nil
}

func (r *controllableSessionRepository) Revoke(_ context.Context, tokenDigest string, now time.Time) (session.StoredSession, bool, error) {
	if r.revokeErr != nil {
		return session.StoredSession{}, false, r.revokeErr
	}
	current, ok, err := r.Resolve(context.Background(), tokenDigest, now)
	if err != nil || !ok {
		return session.StoredSession{}, ok, err
	}
	current.RevokedAt = now
	r.sessions[tokenDigest] = current
	return current, true, nil
}

func cloneTestSessions(source map[string]session.StoredSession) map[string]session.StoredSession {
	cloned := make(map[string]session.StoredSession, len(source))
	for tokenDigest, current := range source {
		cloned[tokenDigest] = current
	}
	return cloned
}

func loginSessionIDForTest(t *testing.T, server *Server, token string) string {
	t.Helper()
	claims, err := server.tokens.Parse(token)
	if err != nil {
		t.Fatalf("parse login token: %v", err)
	}
	return claims.SessionID
}

func openGORMAdminResourceStoreForHTTPTest(t *testing.T, databasePath string, capabilities []capability.Manifest) *adminresource.Store {
	t.Helper()
	db, err := storage.OpenGORM(storage.Config{Driver: "sqlite", DSN: databasePath})
	if err != nil {
		t.Fatalf("OpenGORM(sqlite) error = %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	repository, err := adminresource.NewGORMAdminResourceRepository(context.Background(), db)
	if err != nil {
		t.Fatalf("NewGORMAdminResourceRepository() error = %v", err)
	}
	store, err := adminresource.NewRepositoryBackedStoreFromCapabilities(repository, capabilities)
	if err != nil {
		t.Fatalf("NewRepositoryBackedStoreFromCapabilities() error = %v", err)
	}
	return store
}

func configuredWechatAuthProviderManifest() capability.Manifest {
	manifest := authProviderTestManifest()
	for index := range manifest.AuthProviders {
		if manifest.AuthProviders[index].ID == "wechat" {
			manifest.AuthProviders[index].Configured = true
		}
	}
	return manifest
}

func adminAudienceAuthProviderTestManifest() capability.Manifest {
	manifest := authProviderTestManifest()
	manifest.AuthProviders = append(manifest.AuthProviders, capability.AuthProvider{
		ID:          "oidc",
		Kind:        "oidc",
		Title:       capability.Text("企业单点登录", "Enterprise SSO"),
		Description: capability.Text("通过 OpenID Connect 登录管理台。", "Sign in to Admin through OpenID Connect."),
		Enabled:     true,
		Configured:  true,
		Audiences:   []capability.AuthProviderAudience{capability.AuthProviderAudienceAdmin},
	})
	return manifest
}

func adminOIDCProviderManifest(enabled bool, configured bool) capability.Manifest {
	manifest := adminAudienceAuthProviderTestManifest()
	manifest.AuthProviders = manifest.AuthProviders[len(manifest.AuthProviders)-1:]
	manifest.AuthProviders[0].Enabled = enabled
	manifest.AuthProviders[0].Configured = configured
	return manifest
}

func configuredAdminOIDCPlatformManifests(t *testing.T) []capability.Manifest {
	t.Helper()
	manifests := capabilitiesFromConfigForTest(t, []string{"dictionary", "tenant", "identity", "session", "rbac", "audit", "admin-oidc"})
	for manifestIndex := range manifests {
		for providerIndex := range manifests[manifestIndex].AuthProviders {
			if manifests[manifestIndex].AuthProviders[providerIndex].ID == "oidc" {
				manifests[manifestIndex].AuthProviders[providerIndex].Configured = true
			}
		}
	}
	return manifests
}

func authProviderByIDForTest(t *testing.T, manifests []capability.Manifest, providerID string) capability.AuthProvider {
	t.Helper()
	for _, manifest := range manifests {
		for _, provider := range manifest.AuthProviders {
			if provider.ID == providerID {
				return provider
			}
		}
	}
	t.Fatalf("auth provider %q not found", providerID)
	return capability.AuthProvider{}
}

func successfulAdminIdentityResolver() AdminIdentityResolver {
	return adminIdentityResolverFunc{
		start: func(context.Context, AdminIdentityStartInput) (AdminIdentityStart, error) {
			return AdminIdentityStart{
				AuthorizationURL: "https://id.example/authorize?state=state-exact",
				State:            "state-exact",
				ExpiresAt:        time.Date(2026, time.July, 11, 12, 0, 0, 0, time.UTC),
			}, nil
		},
		resolve: func(context.Context, AdminIdentityResolveInput) (AdminIdentity, error) {
			return AdminIdentity{Issuer: "https://id.example", ProviderSubject: "subject-123"}, nil
		},
	}
}

func postAdminOIDCLoginForTest(server *Server, body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	server.Router().ServeHTTP(recorder, request)
	return recorder
}

func adminOIDCLoginBody(code string, state string, verifier string) string {
	return `{"provider":"oidc","code":"` + code + `","state":"` + state + `","codeVerifier":"` + verifier + `"}`
}

func disableAdminUserForTest(t *testing.T, store *adminresource.Store, username string) {
	t.Helper()
	users, err := store.List("users")
	if err != nil {
		t.Fatalf("List(users) error = %v", err)
	}
	for _, user := range users {
		if user.Code != username {
			continue
		}
		if _, err := store.Update("users", user.ID, adminresource.WriteInput{
			Code: user.Code, Name: user.Name, Status: "disabled", Description: user.Description, Values: user.Values,
		}); err != nil {
			t.Fatalf("Update(disable user %s) error = %v", username, err)
		}
		return
	}
	t.Fatalf("user %q not found", username)
}

func assertResponseRedactsValues(t *testing.T, value string, sensitive ...string) {
	t.Helper()
	for _, item := range sensitive {
		if item != "" && strings.Contains(value, item) {
			t.Fatalf("response exposed sensitive admin oidc value")
		}
	}
}
