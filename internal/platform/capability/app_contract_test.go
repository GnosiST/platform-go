package capability

import (
	"strings"
	"testing"
)

func TestValidateAppSurfaceAcceptsSessionRoutes(t *testing.T) {
	manifests := []Manifest{
		{
			ID: "feedback",
			App: AppSurface{
				Routes: []AppRoute{
					{
						Method:      "GET",
						Path:        "/api/app/feedback/tickets",
						Auth:        AppRouteAuthSession,
						Permission:  "app:feedback-ticket:read",
						Description: Text("读取反馈工单。", "Read feedback tickets."),
					},
				},
			},
		},
	}

	if err := ValidateAppSurface(manifests); err != nil {
		t.Fatalf("ValidateAppSurface() error = %v", err)
	}
}

func TestValidateAppSurfaceRejectsDuplicateMethodPath(t *testing.T) {
	manifests := []Manifest{
		{ID: "a", App: AppSurface{Routes: []AppRoute{validAppRoute("GET", "/api/app/shared")}}},
		{ID: "b", App: AppSurface{Routes: []AppRoute{validAppRoute("GET", "/api/app/shared")}}},
	}

	err := ValidateAppSurface(manifests)
	if err == nil {
		t.Fatalf("ValidateAppSurface() error = nil, want duplicate app route")
	}
	if !strings.Contains(err.Error(), `app route "GET /api/app/shared" already registered`) {
		t.Fatalf("ValidateAppSurface() error = %v, want duplicate app route", err)
	}
}

func TestValidateAppSurfaceRejectsRoutesOutsideAppDomain(t *testing.T) {
	manifests := []Manifest{
		{ID: "feedback", App: AppSurface{Routes: []AppRoute{validAppRoute("GET", "/api/admin/feedback")}}},
	}

	err := ValidateAppSurface(manifests)
	if err == nil {
		t.Fatalf("ValidateAppSurface() error = nil, want app path domain error")
	}
	if !strings.Contains(err.Error(), "app route path must start with /api/app/") {
		t.Fatalf("ValidateAppSurface() error = %v, want app path domain error", err)
	}
}

func TestValidateAppSurfaceRejectsQueryOrFragmentPaths(t *testing.T) {
	for _, path := range []string{
		"/api/app/feedback/tickets?tenant=platform",
		"/api/app/feedback/tickets#detail",
	} {
		t.Run(path, func(t *testing.T) {
			manifests := []Manifest{
				{ID: "feedback", App: AppSurface{Routes: []AppRoute{validAppRoute("GET", path)}}},
			}

			err := ValidateAppSurface(manifests)
			if err == nil {
				t.Fatalf("ValidateAppSurface() error = nil, want static path error")
			}
			if !strings.Contains(err.Error(), "app route path must not include query or fragment") {
				t.Fatalf("ValidateAppSurface() error = %v, want static path error", err)
			}
		})
	}
}

func TestValidateAppSurfaceRejectsMissingAuthMode(t *testing.T) {
	route := validAppRoute("GET", "/api/app/feedback/tickets")
	route.Auth = ""
	manifests := []Manifest{{ID: "feedback", App: AppSurface{Routes: []AppRoute{route}}}}

	err := ValidateAppSurface(manifests)
	if err == nil {
		t.Fatalf("ValidateAppSurface() error = nil, want missing auth mode")
	}
	if !strings.Contains(err.Error(), "auth mode is required") {
		t.Fatalf("ValidateAppSurface() error = %v, want missing auth mode", err)
	}
}

func TestValidateAppSurfaceRejectsAdminPermissionCodes(t *testing.T) {
	route := validAppRoute("GET", "/api/app/feedback/tickets")
	route.Permission = "admin:feedback-ticket:read"
	manifests := []Manifest{{ID: "feedback", App: AppSurface{Routes: []AppRoute{route}}}}

	err := ValidateAppSurface(manifests)
	if err == nil {
		t.Fatalf("ValidateAppSurface() error = nil, want app permission prefix error")
	}
	if !strings.Contains(err.Error(), "permission must start with app:") {
		t.Fatalf("ValidateAppSurface() error = %v, want app permission prefix error", err)
	}
}

func TestValidateAppSurfaceRejectsInvalidAppPermissionCodes(t *testing.T) {
	for _, permission := range []string{
		"app:feedback",
		"app:Feedback:read",
		"app:feedback:read:own",
		"app:feedback:read_all",
	} {
		t.Run(permission, func(t *testing.T) {
			route := validAppRoute("GET", "/api/app/feedback/tickets")
			route.Permission = permission
			manifests := []Manifest{{ID: "feedback", App: AppSurface{Routes: []AppRoute{route}}}}

			err := ValidateAppSurface(manifests)
			if err == nil {
				t.Fatalf("ValidateAppSurface() error = nil, want app permission format error")
			}
			if !strings.Contains(err.Error(), "permission must match app:<domain>:<action>") {
				t.Fatalf("ValidateAppSurface() error = %v, want app permission format error", err)
			}
		})
	}
}

func TestValidateAppSurfaceRejectsMissingLocalizedDescription(t *testing.T) {
	route := validAppRoute("GET", "/api/app/feedback/tickets")
	route.Description.EN = ""
	manifests := []Manifest{{ID: "feedback", App: AppSurface{Routes: []AppRoute{route}}}}

	err := ValidateAppSurface(manifests)
	if err == nil {
		t.Fatalf("ValidateAppSurface() error = nil, want localized description error")
	}
	if !strings.Contains(err.Error(), "description is required") {
		t.Fatalf("ValidateAppSurface() error = %v, want localized description error", err)
	}
}

func TestResolveEnabledValidatesAppSurface(t *testing.T) {
	registry := NewRegistry()
	a := testManifest("a")
	a.App = AppSurface{Routes: []AppRoute{validAppRoute("POST", "/api/app/shared")}}
	mustRegister(t, registry, a)
	b := testManifest("b")
	b.App = AppSurface{Routes: []AppRoute{validAppRoute("POST", "/api/app/shared")}}
	mustRegister(t, registry, b)

	_, err := registry.ResolveEnabled([]ID{"a", "b"})
	if err == nil {
		t.Fatalf("ResolveEnabled() error = nil, want duplicate app route")
	}
	if !strings.Contains(err.Error(), `app route "POST /api/app/shared" already registered`) {
		t.Fatalf("ResolveEnabled() error = %v, want duplicate app route", err)
	}
}

func TestAppRouteContractsExportsNormalizedStableMetadata(t *testing.T) {
	manifests := []Manifest{
		{
			ID: "orders",
			App: AppSurface{
				Routes: []AppRoute{
					{
						Method:      "post",
						Path:        " /api/app/orders ",
						Auth:        AppRouteAuthSession,
						Permission:  " app:order:create ",
						Description: Text("创建订单。", "Create order."),
					},
				},
			},
		},
		{
			ID: "feedback",
			App: AppSurface{
				Routes: []AppRoute{
					{
						Method:      "GET",
						Path:        "/api/app/feedback/tickets",
						Auth:        AppRouteAuthSession,
						Permission:  "app:feedback-ticket:read",
						Description: Text("读取反馈工单。", "Read feedback tickets."),
					},
				},
			},
		},
	}

	contracts, err := AppRouteContracts(manifests)

	if err != nil {
		t.Fatalf("AppRouteContracts() error = %v", err)
	}
	if len(contracts) != 2 {
		t.Fatalf("AppRouteContracts() returned %d contracts, want 2", len(contracts))
	}
	first := contracts[0]
	if first.CapabilityID != "feedback" || first.Method != "GET" || first.Path != "/api/app/feedback/tickets" {
		t.Fatalf("first app route contract = %+v, want sorted feedback GET route", first)
	}
	second := contracts[1]
	if second.CapabilityID != "orders" || second.Method != "POST" || second.Path != "/api/app/orders" || second.Permission != "app:order:create" {
		t.Fatalf("second app route contract = %+v, want normalized order route", second)
	}
	if second.Auth != AppRouteAuthSession || second.Description.EN != "Create order." {
		t.Fatalf("second app route metadata = %+v, want auth and localized description", second)
	}
}

func TestAppRouteContractsRejectsInvalidRoutes(t *testing.T) {
	_, err := AppRouteContracts([]Manifest{
		{ID: "broken", App: AppSurface{Routes: []AppRoute{validAppRoute("TRACE", "/api/app/broken")}}},
	})

	if err == nil {
		t.Fatalf("AppRouteContracts() error = nil, want invalid route")
	}
	if !strings.Contains(err.Error(), "method") {
		t.Fatalf("AppRouteContracts() error = %v, want method error", err)
	}
}

func validAppRoute(method string, path string) AppRoute {
	return AppRoute{
		Method:      method,
		Path:        path,
		Auth:        AppRouteAuthSession,
		Permission:  "app:feedback-ticket:read",
		Description: Text("描述", "Description"),
	}
}
