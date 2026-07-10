package capability

import (
	"strings"
	"testing"
)

func TestValidateDemoDataDeclarationsAcceptsEnabledResourceDatasets(t *testing.T) {
	manifests := []Manifest{
		{
			ID: "tenant",
			Admin: AdminSurface{
				Resources: []AdminResource{validAdminResource("tenants", "/tenants", "admin:tenant")},
			},
			DemoData: []DemoDataSet{validDemoDataSet("platform-demo-tenants", "tenants")},
		},
	}

	if err := ValidateDemoDataDeclarations(manifests); err != nil {
		t.Fatalf("ValidateDemoDataDeclarations() error = %v", err)
	}
}

func TestValidateDemoDataDeclarationsRejectsMissingTargetResource(t *testing.T) {
	manifests := []Manifest{
		{
			ID:       "demo-data",
			DemoData: []DemoDataSet{validDemoDataSet("platform-demo-tenants", "tenants")},
		},
	}

	err := ValidateDemoDataDeclarations(manifests)

	if err == nil {
		t.Fatalf("ValidateDemoDataDeclarations() error = nil, want missing resource")
	}
	if !strings.Contains(err.Error(), `demo data "platform-demo-tenants" resource "tenants" is not enabled`) {
		t.Fatalf("ValidateDemoDataDeclarations() error = %v, want missing resource error", err)
	}
}

func TestValidateDemoDataDeclarationsRejectsInvalidDatasetID(t *testing.T) {
	manifests := []Manifest{
		{
			ID:       "tenant",
			Admin:    AdminSurface{Resources: []AdminResource{validAdminResource("tenants", "/tenants", "admin:tenant")}},
			DemoData: []DemoDataSet{validDemoDataSet("Platform Demo Tenants", "tenants")},
		},
	}

	err := ValidateDemoDataDeclarations(manifests)

	if err == nil {
		t.Fatalf("ValidateDemoDataDeclarations() error = nil, want dataset id format error")
	}
	if !strings.Contains(err.Error(), "demo data id must use lowercase letters, numbers or hyphens") {
		t.Fatalf("ValidateDemoDataDeclarations() error = %v, want dataset id format error", err)
	}
}

func TestValidateDemoDataDeclarationsRejectsDuplicateTrimmedDatasetIDs(t *testing.T) {
	manifests := []Manifest{
		{
			ID:    "tenant",
			Admin: AdminSurface{Resources: []AdminResource{validAdminResource("tenants", "/tenants", "admin:tenant")}},
			DemoData: []DemoDataSet{
				validDemoDataSet("platform-demo-tenants", "tenants"),
				validDemoDataSet(" platform-demo-tenants ", "tenants"),
			},
		},
	}

	err := ValidateDemoDataDeclarations(manifests)

	if err == nil {
		t.Fatalf("ValidateDemoDataDeclarations() error = nil, want duplicate dataset id")
	}
	if !strings.Contains(err.Error(), `demo data "platform-demo-tenants" already registered`) {
		t.Fatalf("ValidateDemoDataDeclarations() error = %v, want duplicate dataset id error", err)
	}
}

func TestValidateDemoDataDeclarationsRejectsDuplicateRecordIDs(t *testing.T) {
	dataset := validDemoDataSet("platform-demo-tenants", "tenants")
	dataset.Records = append(dataset.Records, DemoRecord{ID: dataset.Records[0].ID, Code: "demo-other", Name: "Other Tenant"})
	manifests := []Manifest{
		{
			ID:       "tenant",
			Admin:    AdminSurface{Resources: []AdminResource{validAdminResource("tenants", "/tenants", "admin:tenant")}},
			DemoData: []DemoDataSet{dataset},
		},
	}

	err := ValidateDemoDataDeclarations(manifests)

	if err == nil {
		t.Fatalf("ValidateDemoDataDeclarations() error = nil, want duplicate record id")
	}
	if !strings.Contains(err.Error(), `record id "tenant-demo" is duplicated`) {
		t.Fatalf("ValidateDemoDataDeclarations() error = %v, want duplicate record id error", err)
	}
}

func TestValidateDemoDataDeclarationsRejectsDuplicateRecordCodes(t *testing.T) {
	dataset := validDemoDataSet("platform-demo-tenants", "tenants")
	dataset.Records = append(dataset.Records, DemoRecord{ID: "tenant-other", Code: dataset.Records[0].Code, Name: "Other Tenant"})
	manifests := []Manifest{
		{
			ID:       "tenant",
			Admin:    AdminSurface{Resources: []AdminResource{validAdminResource("tenants", "/tenants", "admin:tenant")}},
			DemoData: []DemoDataSet{dataset},
		},
	}

	err := ValidateDemoDataDeclarations(manifests)

	if err == nil {
		t.Fatalf("ValidateDemoDataDeclarations() error = nil, want duplicate record code")
	}
	if !strings.Contains(err.Error(), `record code "demo" is duplicated`) {
		t.Fatalf("ValidateDemoDataDeclarations() error = %v, want duplicate record code error", err)
	}
}

func TestValidateDemoDataDeclarationsRejectsSensitiveRecordValueFields(t *testing.T) {
	for _, field := range []string{"password", "apiToken", "wechatOpenID", "verificationCode"} {
		t.Run(field, func(t *testing.T) {
			dataset := validDemoDataSet("platform-demo-tenants", "tenants")
			dataset.Records[0].Values = map[string]string{field: "demo-secret"}
			manifests := []Manifest{
				{
					ID:       "tenant",
					Admin:    AdminSurface{Resources: []AdminResource{validAdminResource("tenants", "/tenants", "admin:tenant")}},
					DemoData: []DemoDataSet{dataset},
				},
			}

			err := ValidateDemoDataDeclarations(manifests)
			if err == nil {
				t.Fatalf("ValidateDemoDataDeclarations() error = nil, want sensitive field error")
			}
			if !strings.Contains(err.Error(), "must not include sensitive field") {
				t.Fatalf("ValidateDemoDataDeclarations() error = %v, want sensitive field error", err)
			}
		})
	}
}

func TestResolveEnabledValidatesDemoDataTargetResources(t *testing.T) {
	registry := NewRegistry()
	demoData := testManifest("demo-data")
	demoData.DemoData = []DemoDataSet{validDemoDataSet("platform-demo-tenants", "tenants")}
	mustRegister(t, registry, demoData)

	_, err := registry.ResolveEnabled([]ID{"demo-data"})

	if err == nil {
		t.Fatalf("ResolveEnabled() error = nil, want missing demo data target resource")
	}
	if !strings.Contains(err.Error(), `demo data "platform-demo-tenants" resource "tenants" is not enabled`) {
		t.Fatalf("ResolveEnabled() error = %v, want missing target resource error", err)
	}
}

func validDemoDataSet(id string, resource string) DemoDataSet {
	return DemoDataSet{
		ID:          id,
		Title:       Text("平台演示租户", "Platform Demo Tenants"),
		Description: Text("用于本地演示和底座验证的租户数据。", "Tenant data for local demos and platform validation."),
		Resource:    resource,
		Records: []DemoRecord{
			{ID: "tenant-demo", Code: "demo", Name: "Demo Tenant", Status: "enabled"},
		},
	}
}
