package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunAppRoutesWritesManifestDerivedContract(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "app-route-contract.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := run([]string{"app-routes", "--output", outputPath}, &stdout, &stderr); err != nil {
		t.Fatalf("run(app-routes) error = %v, stderr = %s", err, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty when writing to file", stdout.String())
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile(output) error = %v", err)
	}
	var document struct {
		GeneratedBy  string `json:"generatedBy"`
		Source       string `json:"source"`
		SourceMode   string `json:"sourceMode"`
		RouteCount   int    `json:"routeCount"`
		Capabilities []string
		Permissions  []string
		Routes       []struct {
			CapabilityID string `json:"capabilityId"`
			Method       string `json:"method"`
			Path         string `json:"path"`
			Auth         string `json:"auth"`
			Permission   string `json:"permission,omitempty"`
			Description  struct {
				ZH string `json:"zh"`
				EN string `json:"en"`
			} `json:"description"`
		} `json:"routes"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("Unmarshal(output) error = %v", err)
	}

	if document.GeneratedBy != "cmd/platform-contracts app-routes" {
		t.Fatalf("generatedBy = %q", document.GeneratedBy)
	}
	if document.Source != "capability.Manifest.App.Routes" || document.SourceMode != "go-manifest" {
		t.Fatalf("source = %q/%q, want Go manifest source", document.Source, document.SourceMode)
	}
	if document.RouteCount != len(document.Routes) || document.RouteCount == 0 {
		t.Fatalf("routeCount = %d, routes = %d", document.RouteCount, len(document.Routes))
	}
	if !containsString(document.Capabilities, "session") {
		t.Fatalf("capabilities = %+v, want session", document.Capabilities)
	}

	var loginFound bool
	for _, route := range document.Routes {
		if route.CapabilityID == "session" && route.Method == "POST" && route.Path == "/api/app/auth/login" {
			loginFound = true
			if route.Auth != "public" || route.Permission != "" || route.Description.EN == "" || route.Description.ZH == "" {
				t.Fatalf("login route = %+v, want public route with localized description and no permission", route)
			}
		}
	}
	if !loginFound {
		t.Fatalf("routes = %+v, want app login route", document.Routes)
	}
}

func TestRunAppRoutesRejectsUnknownBusinessCapability(t *testing.T) {
	t.Setenv("PLATFORM_CAPABILITIES", "dictionary,tenant,identity,session,rbac,menu,audit,admin-shell,external-ordering")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := run([]string{"app-routes", "--stdout"}, &stdout, &stderr); err == nil {
		t.Fatalf("run(app-routes --stdout) error = nil, want unknown business capability rejection")
	}
}

func TestRunAdminResourcesWritesEnabledCapabilityContract(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "admin-capability-resources.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := run([]string{"admin-resources", "--output", outputPath}, &stdout, &stderr); err != nil {
		t.Fatalf("run(admin-resources) error = %v, stderr = %s", err, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty when writing to file", stdout.String())
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile(output) error = %v", err)
	}
	var document struct {
		GeneratedBy   string   `json:"generatedBy"`
		Source        string   `json:"source"`
		SourceMode    string   `json:"sourceMode"`
		ResourceCount int      `json:"resourceCount"`
		Capabilities  []string `json:"capabilities"`
		Permissions   []string `json:"permissions"`
		Resources     []struct {
			CapabilityID     string `json:"capabilityId"`
			Resource         string `json:"resource"`
			PermissionPrefix string `json:"permissionPrefix"`
			Menu             struct {
				Route string `json:"route"`
			} `json:"menu"`
			Fields []struct {
				Key      string `json:"key"`
				Required bool   `json:"required,omitempty"`
				Relation *struct {
					Resource string `json:"resource"`
				} `json:"relation,omitempty"`
			} `json:"fields"`
		} `json:"resources"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("Unmarshal(output) error = %v", err)
	}

	if document.GeneratedBy != "cmd/platform-contracts admin-resources" {
		t.Fatalf("generatedBy = %q", document.GeneratedBy)
	}
	if document.Source != "capability.Manifest.Admin.Resources" || document.SourceMode != "go-manifest" {
		t.Fatalf("source = %q/%q, want Go manifest source", document.Source, document.SourceMode)
	}
	if document.ResourceCount != len(document.Resources) || document.ResourceCount == 0 {
		t.Fatalf("resourceCount = %d, resources = %d", document.ResourceCount, len(document.Resources))
	}
	if containsString(document.Capabilities, "external-ordering") {
		t.Fatalf("capabilities = %+v, want no built-in business capability", document.Capabilities)
	}

	for _, resource := range document.Resources {
		if resource.Resource == "tasks" || resource.Resource == "role-applications" || resource.Resource == "support-tickets" {
			t.Fatalf("resources = %+v, want no zshenmez business resources", document.Resources)
		}
	}
	for _, resource := range document.Resources {
		if resource.Resource != "users" && resource.Resource != "org-units" {
			continue
		}
		field := contractResourceField(resource.Fields, "tenantCode")
		if field == nil || !field.Required {
			t.Fatalf("%s tenantCode field = %+v, want required tenant ownership in generated capability contract", resource.Resource, field)
		}
	}
}

func TestRunAdminResourcesStdoutWritesSingleJSONDocument(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := run([]string{"admin-resources", "--stdout"}, &stdout, &stderr); err != nil {
		t.Fatalf("run(admin-resources --stdout) error = %v, stderr = %s", err, stderr.String())
	}

	decoder := json.NewDecoder(bytes.NewReader(stdout.Bytes()))
	var document map[string]any
	if err := decoder.Decode(&document); err != nil {
		t.Fatalf("Decode(stdout) error = %v, stdout = %s", err, stdout.String())
	}
	var extra map[string]any
	if err := decoder.Decode(&extra); err != io.EOF {
		t.Fatalf("Decode(extra) error = %v, want EOF; stdout = %s", err, stdout.String())
	}
}

func TestRunAuditWritesPlatformCapabilitySummary(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := run([]string{"audit", "--stdout"}, &stdout, &stderr); err != nil {
		t.Fatalf("run(audit) error = %v, stderr = %s", err, stderr.String())
	}

	var document struct {
		GeneratedBy          string `json:"generatedBy"`
		Source               string `json:"source"`
		SourceMode           string `json:"sourceMode"`
		Status               string `json:"status"`
		CapabilityCount      int    `json:"capabilityCount"`
		ResourceCount        int    `json:"resourceCount"`
		RouteCount           int    `json:"routeCount"`
		AppRouteHandlerCount int    `json:"appRouteHandlerCount"`
		AdminPermissionCount int    `json:"adminPermissionCount"`
		AppPermissionCount   int    `json:"appPermissionCount"`
		AuthProviderCount    int    `json:"authProviderCount"`
		DemoDataSetCount     int    `json:"demoDataSetCount"`
		MigrationCount       int    `json:"migrationCount"`
		SeedCount            int    `json:"seedCount"`
		Capabilities         []struct {
			ID               string   `json:"id"`
			AdminResources   []string `json:"adminResources,omitempty"`
			AppRoutes        []string `json:"appRoutes,omitempty"`
			AppRouteHandlers []string `json:"appRouteHandlers,omitempty"`
			AuthProviders    []string `json:"authProviders,omitempty"`
			DemoDataSets     []string `json:"demoDataSets,omitempty"`
			Migrations       []string `json:"migrations,omitempty"`
			Seeds            []string `json:"seeds,omitempty"`
		} `json:"capabilities"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &document); err != nil {
		t.Fatalf("Unmarshal(stdout) error = %v, stdout = %s", err, stdout.String())
	}

	if document.GeneratedBy != "cmd/platform-contracts audit" {
		t.Fatalf("generatedBy = %q", document.GeneratedBy)
	}
	if document.Source != "capability.Manifest" || document.SourceMode != "go-manifest" || document.Status != "pass" {
		t.Fatalf("audit source/status = %q/%q/%q", document.Source, document.SourceMode, document.Status)
	}
	if document.CapabilityCount != len(document.Capabilities) || document.CapabilityCount == 0 {
		t.Fatalf("capabilityCount = %d, capabilities = %d", document.CapabilityCount, len(document.Capabilities))
	}
	if document.ResourceCount == 0 || document.RouteCount == 0 || document.AdminPermissionCount == 0 {
		t.Fatalf("audit counts = resources:%d routes:%d adminPermissions:%d", document.ResourceCount, document.RouteCount, document.AdminPermissionCount)
	}
	if document.AppRouteHandlerCount != document.RouteCount {
		t.Fatalf("appRouteHandlerCount = %d, routeCount = %d, want all declared app routes covered", document.AppRouteHandlerCount, document.RouteCount)
	}
	if document.AppPermissionCount != 0 {
		t.Fatalf("appPermissionCount = %d, want no built-in business app permissions", document.AppPermissionCount)
	}
	if document.AuthProviderCount == 0 || document.MigrationCount == 0 || document.SeedCount == 0 {
		t.Fatalf("audit lifecycle/auth counts = auth:%d migrations:%d seeds:%d", document.AuthProviderCount, document.MigrationCount, document.SeedCount)
	}
	if !auditCapabilityHasResource(document.Capabilities, "identity", "org-units") {
		t.Fatalf("capabilities = %+v, want identity org-units resource", document.Capabilities)
	}
	if !auditCapabilityHasResource(document.Capabilities, "rbac", "role-groups") {
		t.Fatalf("capabilities = %+v, want rbac role-groups resource", document.Capabilities)
	}
	if !auditCapabilityHasResource(document.Capabilities, "dictionary", "area-codes") {
		t.Fatalf("capabilities = %+v, want dictionary area-codes resource", document.Capabilities)
	}
	if auditCapabilityHasResource(document.Capabilities, "external-ordering", "tasks") {
		t.Fatalf("capabilities = %+v, want no zshenmez business resource in platform audit", document.Capabilities)
	}
}

func TestRunRejectsUnknownContractCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := run([]string{"unknown"}, &stdout, &stderr)

	if err == nil {
		t.Fatalf("run(unknown) error = nil, want error")
	}
	if !strings.Contains(err.Error(), "unknown contract command") {
		t.Fatalf("run(unknown) error = %v", err)
	}
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func auditCapabilityHasResource(capabilities []struct {
	ID               string   `json:"id"`
	AdminResources   []string `json:"adminResources,omitempty"`
	AppRoutes        []string `json:"appRoutes,omitempty"`
	AppRouteHandlers []string `json:"appRouteHandlers,omitempty"`
	AuthProviders    []string `json:"authProviders,omitempty"`
	DemoDataSets     []string `json:"demoDataSets,omitempty"`
	Migrations       []string `json:"migrations,omitempty"`
	Seeds            []string `json:"seeds,omitempty"`
}, capabilityID string, resource string) bool {
	for _, capability := range capabilities {
		if capability.ID == capabilityID && containsString(capability.AdminResources, resource) {
			return true
		}
	}
	return false
}

func auditCapabilityHasAppRoute(capabilities []struct {
	ID               string   `json:"id"`
	AdminResources   []string `json:"adminResources,omitempty"`
	AppRoutes        []string `json:"appRoutes,omitempty"`
	AppRouteHandlers []string `json:"appRouteHandlers,omitempty"`
	AuthProviders    []string `json:"authProviders,omitempty"`
	DemoDataSets     []string `json:"demoDataSets,omitempty"`
	Migrations       []string `json:"migrations,omitempty"`
	Seeds            []string `json:"seeds,omitempty"`
}, capabilityID string, route string) bool {
	for _, capability := range capabilities {
		if capability.ID == capabilityID && containsString(capability.AppRoutes, route) {
			return true
		}
	}
	return false
}

func auditCapabilityHasAppRouteHandler(capabilities []struct {
	ID               string   `json:"id"`
	AdminResources   []string `json:"adminResources,omitempty"`
	AppRoutes        []string `json:"appRoutes,omitempty"`
	AppRouteHandlers []string `json:"appRouteHandlers,omitempty"`
	AuthProviders    []string `json:"authProviders,omitempty"`
	DemoDataSets     []string `json:"demoDataSets,omitempty"`
	Migrations       []string `json:"migrations,omitempty"`
	Seeds            []string `json:"seeds,omitempty"`
}, capabilityID string, route string) bool {
	for _, capability := range capabilities {
		if capability.ID == capabilityID && containsString(capability.AppRouteHandlers, route) {
			return true
		}
	}
	return false
}

func contractHasAppRoute(routes []struct {
	CapabilityID string `json:"capabilityId"`
	Method       string `json:"method"`
	Path         string `json:"path"`
	Auth         string `json:"auth"`
	Permission   string `json:"permission,omitempty"`
}, capabilityID string, method string, path string, permission string) bool {
	for _, route := range routes {
		if route.CapabilityID == capabilityID && route.Method == method && route.Path == path && route.Permission == permission && route.Auth == "session" {
			return true
		}
	}
	return false
}

func contractResourceHasField(fields []struct {
	Key      string `json:"key"`
	Required bool   `json:"required,omitempty"`
	Relation *struct {
		Resource string `json:"resource"`
	} `json:"relation,omitempty"`
}, target string) bool {
	return contractResourceField(fields, target) != nil
}

func contractResourceField(fields []struct {
	Key      string `json:"key"`
	Required bool   `json:"required,omitempty"`
	Relation *struct {
		Resource string `json:"resource"`
	} `json:"relation,omitempty"`
}, target string) *struct {
	Key      string `json:"key"`
	Required bool   `json:"required,omitempty"`
	Relation *struct {
		Resource string `json:"resource"`
	} `json:"relation,omitempty"`
} {
	for _, field := range fields {
		if field.Key == target {
			return &field
		}
	}
	return nil
}
