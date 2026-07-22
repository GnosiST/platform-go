package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/GnosiST/platform-go/pkg/platform/capability"
)

func TestRouterMountsDeclaredSessionRouteWithTypedIdentity(t *testing.T) {
	manifests := []capability.Manifest{{
		ID:      "catalog",
		Name:    "Catalog",
		Version: "0.1.0",
		App: capability.AppSurface{Routes: []capability.AppRoute{{
			Method:      http.MethodGet,
			Path:        "/api/app/catalog/items",
			Auth:        capability.AppRouteAuthSession,
			Permission:  "app:catalog:read",
			Description: capability.Text("读取目录。", "Read catalog."),
		}}},
	}}
	router, err := NewRouter(manifests, []Registration{{
		Method: http.MethodGet,
		Path:   "/api/app/catalog/items",
		Handler: func(ctx context.Context, identity Identity, writer http.ResponseWriter, _ *http.Request) {
			if identity.SubjectID != "user-1" || identity.TenantID != "tenant-a" || !identity.HasPermission("app:catalog:read") {
				t.Fatalf("handler identity = %+v, want typed authenticated catalog identity", identity)
			}
			writer.WriteHeader(http.StatusNoContent)
		},
	}})
	if err != nil {
		t.Fatalf("NewRouter() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/app/catalog/items", nil)
	request = request.WithContext(WithIdentity(request.Context(), Identity{
		SubjectID:   "user-1",
		TenantID:    "tenant-a",
		Permissions: []string{"app:catalog:read"},
	}))
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("GET catalog items status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
}

func TestRouterRejectsMissingSessionIdentityBeforeHandler(t *testing.T) {
	manifests := []capability.Manifest{{
		ID:      "catalog",
		Name:    "Catalog",
		Version: "0.1.0",
		App: capability.AppSurface{Routes: []capability.AppRoute{{
			Method:      http.MethodGet,
			Path:        "/api/app/catalog/items",
			Auth:        capability.AppRouteAuthSession,
			Description: capability.Text("读取目录。", "Read catalog."),
		}}},
	}}
	called := false
	router, err := NewRouter(manifests, []Registration{{
		Method: http.MethodGet,
		Path:   "/api/app/catalog/items",
		Handler: func(context.Context, Identity, http.ResponseWriter, *http.Request) {
			called = true
		},
	}})
	if err != nil {
		t.Fatalf("NewRouter() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/app/catalog/items", nil))

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("GET catalog items without identity status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
	if called {
		t.Fatal("session handler ran without a typed identity")
	}
}

func TestNewRouterRejectsUndeclaredRegistration(t *testing.T) {
	_, err := NewRouter([]capability.Manifest{{ID: "catalog", Name: "Catalog", Version: "0.1.0"}}, []Registration{{
		Method:  http.MethodGet,
		Path:    "/api/app/catalog/items",
		Handler: func(context.Context, Identity, http.ResponseWriter, *http.Request) {},
	}})
	if err == nil {
		t.Fatal("NewRouter() error = nil, want undeclared registration error")
	}
}

func TestNewRouterRejectsEquivalentStandardLibraryPatterns(t *testing.T) {
	manifests := []capability.Manifest{{
		ID:      "catalog",
		Name:    "Catalog",
		Version: "0.1.0",
		App: capability.AppSurface{Routes: []capability.AppRoute{
			{Method: http.MethodGet, Path: "/api/app/catalog/items/:id", Auth: capability.AppRouteAuthPublic, Description: capability.Text("读取目录。", "Read catalog.")},
			{Method: http.MethodGet, Path: "/api/app/catalog/items/:code", Auth: capability.AppRouteAuthPublic, Description: capability.Text("读取目录。", "Read catalog.")},
		}},
	}}
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("NewRouter() panicked for equivalent patterns: %v", recovered)
		}
	}()

	_, err := NewRouter(manifests, []Registration{
		{Method: http.MethodGet, Path: "/api/app/catalog/items/:id", Handler: func(context.Context, Identity, http.ResponseWriter, *http.Request) {}},
		{Method: http.MethodGet, Path: "/api/app/catalog/items/:code", Handler: func(context.Context, Identity, http.ResponseWriter, *http.Request) {}},
	})
	if err == nil {
		t.Fatal("NewRouter() error = nil, want equivalent pattern error")
	}
}
