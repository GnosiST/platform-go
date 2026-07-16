package httpapi

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/GnosiST/platform-go/internal/platform/capability"
)

func TestAppRouteHandlerCoverageReportsMissingDeclaredHandlers(t *testing.T) {
	coverage, err := AppRouteHandlerCoverage([]capability.Manifest{
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
	}, nil)
	if err != nil {
		t.Fatalf("AppRouteHandlerCoverage() error = %v", err)
	}

	if coverage.DeclaredCount != 1 || coverage.CoveredCount != 0 {
		t.Fatalf("coverage counts = declared:%d covered:%d, want 1/0", coverage.DeclaredCount, coverage.CoveredCount)
	}
	if !containsAppRouteName(coverage.MissingRoutes, "GET /api/app/orders") {
		t.Fatalf("missing routes = %+v, want GET /api/app/orders", coverage.MissingRoutes)
	}
}

func TestAppRouteHandlerCoverageIncludesCoreAndExternalHandlers(t *testing.T) {
	coverage, err := AppRouteHandlerCoverage([]capability.Manifest{
		{
			ID: "session",
			App: capability.AppSurface{Routes: []capability.AppRoute{
				{
					Method:      http.MethodPost,
					Path:        "/api/app/auth/login",
					Auth:        capability.AppRouteAuthPublic,
					Description: capability.Text("App 登录。", "App login."),
				},
			}},
		},
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
	}, []AppRouteRegistration{
		{
			Method: http.MethodGet,
			Path:   "/api/app/orders",
			Handler: func(ctx *gin.Context) {
				ctx.Status(http.StatusNoContent)
			},
		},
	})
	if err != nil {
		t.Fatalf("AppRouteHandlerCoverage() error = %v", err)
	}

	if coverage.DeclaredCount != 2 || coverage.CoveredCount != 2 || len(coverage.MissingRoutes) != 0 {
		t.Fatalf("coverage = %+v, want all declared routes covered", coverage)
	}
	if !containsAppRouteName(coverage.CoveredRoutes, "POST /api/app/auth/login") || !containsAppRouteName(coverage.CoveredRoutes, "GET /api/app/orders") {
		t.Fatalf("covered routes = %+v, want core and external handlers", coverage.CoveredRoutes)
	}
}

func TestAppRouteHandlerCoverageIncludesCoreFileStorageHandlers(t *testing.T) {
	coverage, err := AppRouteHandlerCoverage([]capability.Manifest{
		{
			ID: "file-storage",
			App: capability.AppSurface{Routes: []capability.AppRoute{
				{
					Method:      http.MethodPost,
					Path:        "/api/app/files",
					Auth:        capability.AppRouteAuthSession,
					Description: capability.Text("上传 App 文件。", "Upload app file."),
				},
				{
					Method:      http.MethodGet,
					Path:        "/api/app/files/:id/content",
					Auth:        capability.AppRouteAuthSession,
					Description: capability.Text("读取 App 文件内容。", "Read app file content."),
				},
			}},
		},
	}, nil)
	if err != nil {
		t.Fatalf("AppRouteHandlerCoverage() error = %v", err)
	}

	if coverage.DeclaredCount != 2 || coverage.CoveredCount != 2 || len(coverage.MissingRoutes) != 0 {
		t.Fatalf("coverage = %+v, want file-storage app routes covered by core handlers", coverage)
	}
	if !containsAppRouteName(coverage.CoveredRoutes, "POST /api/app/files") || !containsAppRouteName(coverage.CoveredRoutes, "GET /api/app/files/:id/content") {
		t.Fatalf("covered routes = %+v, want file-storage app routes", coverage.CoveredRoutes)
	}
}

func containsAppRouteName(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
