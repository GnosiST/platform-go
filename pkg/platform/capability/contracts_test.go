package capability

import (
	"context"
	"testing"
)

func TestPublicCapabilityContractsExposeOnboardingSurface(t *testing.T) {
	resource := AdminResource{
		Resource:         "public-contract-items",
		Title:            Text("公开合同项", "Public Contract Items"),
		Description:      Text("验证公开 capability 包覆盖外部接入声明面。", "Verifies the public capability package covers external onboarding declarations."),
		PermissionPrefix: "admin:public-contract-item",
		Deletion:         &AdminResourceDeletionPolicy{Mode: AdminDeletionSoftDelete, PolicyVersion: 1},
		Menu:             AdminMenu{Route: "/public-contract-items", Group: "business", Icon: "appstore"},
		Fields: []AdminField{
			{Key: "code", Label: Text("编码", "Code"), Type: "text", Source: "record", Required: true, Sensitivity: FieldSensitivityPublic, StorageMode: FieldStoragePlain, ResponseMode: FieldProjectionFull, ExportMode: FieldProjectionFull},
			{Key: "name", Label: Text("名称", "Name"), Type: "text", Source: "record", Required: true, Sensitivity: FieldSensitivityPublic, StorageMode: FieldStoragePlain, ResponseMode: FieldProjectionFull, ExportMode: FieldProjectionFull},
		},
		SearchFields:   []string{"code", "name"},
		DefaultSortKey: "code",
	}
	manifest := Manifest{
		ID:      "public-contract",
		Name:    "Public Contract",
		Version: "0.1.0",
		Admin:   AdminSurface{Resources: []AdminResource{resource}},
		App: AppSurface{Routes: []AppRoute{{
			Method:      "GET",
			Path:        "/api/app/public-contract/items",
			Auth:        AppRouteAuthSession,
			Permission:  "app:public-contract:read",
			Description: Text("读取公开合同项。", "Read public contract items."),
		}}},
		Service: ServiceSurface{
			ID:            "public-contract-service",
			Owner:         "public-contract",
			Audiences:     []ServiceAudience{ServiceAudienceInternal},
			Stability:     ServiceStabilityExperimental,
			Version:       "0.1.0",
			IdentityModes: []ServiceIdentityMode{ServiceIdentityWorkload},
			AuthModes:     []ServiceAuthMode{ServiceAuthWorkloadJWT},
			TenantContext: DefaultTrustedTenantContext(),
			Operations: []ServiceOperation{{
				ID: "public-contract-read", Kind: ServiceOperationQuery, Plane: ServicePlaneData, RuntimeStatus: ServiceRuntimeContractOnly,
				IdentityMode: ServiceIdentityWorkload, AuthModes: []ServiceAuthMode{ServiceAuthWorkloadJWT}, TenantMode: ServiceTenantRequired,
				DataScopes: []string{"tenant"}, RequestSchema: ServicePayloadSchema{Ref: "#/schemas/PublicContractQuery", PII: ServicePIINone},
				ResponseSchema: ServicePayloadSchema{Ref: "#/schemas/PublicContractPage", PII: ServicePIINone},
				Reliability:    ServiceReliability{Idempotency: "none", OptimisticConcurrency: "none", TimeoutMilliseconds: 1000, MaxRetries: 0, RateLimitPerMinute: 60, CostLimit: 1},
				Compatibility:  ServiceCompatibility{Mode: "semver"}, Description: Text("读取合同。", "Read contract."),
			}},
			SLA:           ServiceSLA{AvailabilityTarget: "99.0%", LatencyP95MS: 500},
			Compatibility: ServiceCompatibility{Mode: "semver"},
			RuntimeBoundary: ServiceRuntimeBoundary{
				ContractExecution:    "contract-only",
				IdentityProtocols:    "workload-jwt declaration",
				EventDelivery:        "contract-only",
				DatasourceRouting:    "trusted tenant context only",
				RuntimeSourceWriting: "disabled",
			},
		},
		Migrations: []Migration{{ID: "public-contract-0001", Description: "Validate public migration.", Up: func(context.Context, Runtime) error { return nil }}},
		Seeds:      []Seed{{ID: "public-contract-seed-0001", Description: "Validate public seed.", Run: func(context.Context, Runtime) error { return nil }}},
		DemoData: []DemoDataSet{{
			ID:          "public-contract-demo",
			Title:       Text("演示", "Demo"),
			Description: Text("演示数据。", "Demo data."),
			Resource:    "public-contract-items",
			Records:     []DemoRecord{{ID: "public-contract-demo-item", Code: "demo-item", Name: "Demo Item"}},
		}},
	}

	registry := NewRegistry()
	if err := registry.Register(manifest); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	resolved, err := registry.ResolveEnabled([]ID{"public-contract"})
	if err != nil {
		t.Fatalf("ResolveEnabled() error = %v", err)
	}
	if _, err := AppRouteContracts(resolved); err != nil {
		t.Fatalf("AppRouteContracts() error = %v", err)
	}
	if document, err := ServiceContractDocumentFromManifests(resolved); err != nil || len(document.Services) != 1 {
		t.Fatalf("ServiceContractDocumentFromManifests() = %+v, %v; want one service", document, err)
	}

	history := NewMemoryLifecycleHistory()
	executor := NewRecordedLifecycleExecutor(history)
	runtime := Runtime{MigrationExecutor: executor, SeedExecutor: executor}
	if err := RunLifecycle(context.Background(), resolved, runtime); err != nil {
		t.Fatalf("RunLifecycle() error = %v", err)
	}
	records := history.Records()
	if len(records) != 2 {
		t.Fatalf("lifecycle records = %+v, want migration and seed records", records)
	}
	if records[0].Kind != LifecycleKindMigration || records[1].Kind != LifecycleKindSeed {
		t.Fatalf("lifecycle record kinds = %+v, want migration then seed", records)
	}
}
