package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"platform-go/internal/platform/adminresource"
	"platform-go/internal/platform/core"
)

func TestServerRegistersInjectedAdminRoutesBehindPermissionPolicy(t *testing.T) {
	called := false
	server := New(ServerOptions{
		Capabilities:            core.DefaultManifests(),
		Resources:               adminresource.NewStoreFromCapabilities(core.DefaultManifests()),
		AllowInsecureHeaderAuth: true,
		AdminRoutes: []AdminRouteRegistration{
			{
				Method:     http.MethodPost,
				Path:       "/api/admin/custom-actions/ping",
				Permission: "admin:tenant:read",
				Handler: func(ctx *gin.Context) {
					called = true
					ctx.JSON(http.StatusOK, Response[gin.H]{Data: gin.H{"ok": true}})
				},
			},
		},
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/admin/custom-actions/ping", nil)
	request.Header.Set("X-Platform-User", "admin")

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("POST injected admin route status = %d body = %s, want 200", recorder.Code, recorder.Body.String())
	}
	if !called {
		t.Fatalf("injected admin route handler was not called")
	}
}

func TestServerRejectsInjectedAdminRoutesWithoutPermission(t *testing.T) {
	called := false
	server := New(ServerOptions{
		Capabilities:            core.DefaultManifests(),
		Resources:               adminresource.NewStoreFromCapabilities(core.DefaultManifests()),
		AllowInsecureHeaderAuth: true,
		AdminRoutes: []AdminRouteRegistration{
			{
				Method:     http.MethodPost,
				Path:       "/api/admin/custom-actions/secure",
				Permission: "admin:tenant:delete",
				Handler: func(ctx *gin.Context) {
					called = true
					ctx.Status(http.StatusNoContent)
				},
			},
		},
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/admin/custom-actions/secure", nil)
	request.Header.Set("X-Platform-User", "ops")

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("POST forbidden injected admin route status = %d body = %s, want 403", recorder.Code, recorder.Body.String())
	}
	if called {
		t.Fatalf("forbidden injected admin route handler should not be called")
	}
}
