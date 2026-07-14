package capability

import (
	"strings"
	"testing"
)

func TestValidateServiceContractsAcceptsFivePlaneStandard(t *testing.T) {
	manifest := serviceContractTestManifest("files")
	if err := ValidateServiceContracts([]Manifest{manifest}); err != nil {
		t.Fatalf("ValidateServiceContracts() error = %v", err)
	}

	manifest.Service.Operations = append(manifest.Service.Operations,
		serviceContractTestOperation("admin-list", ServiceOperationQuery, ServicePlaneAdmin, ServiceIdentityManagementUser, []ServiceAuthMode{ServiceAuthAdminSession}),
		serviceContractTestOperation("control-health", ServiceOperationQuery, ServicePlaneControl, ServiceIdentityWorkload, []ServiceAuthMode{ServiceAuthWorkloadJWT}),
	)
	if err := ValidateServiceContracts([]Manifest{manifest}); err != nil {
		t.Fatalf("ValidateServiceContracts(five planes) error = %v", err)
	}
}

func TestValidateServiceContractsKeepsEmptySurfaceBackwardCompatible(t *testing.T) {
	if err := ValidateServiceContracts([]Manifest{{ID: "legacy", Version: "0.1.0"}}); err != nil {
		t.Fatalf("ValidateServiceContracts(empty) error = %v", err)
	}
}

func TestValidateServiceContractsRejectsPartiallyDeclaredSurface(t *testing.T) {
	manifest := Manifest{ID: "partial", Version: "0.1.0", Service: ServiceSurface{Owner: "platform-test"}}
	if err := ValidateServiceContracts([]Manifest{manifest}); err == nil || !strings.Contains(err.Error(), "service id must use") {
		t.Fatalf("ValidateServiceContracts(partial) error = %v", err)
	}
}

func TestValidateServiceContractsRejectsUnsafeTenantAndIdentityContracts(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Manifest)
		want string
	}{
		{
			name: "client physical routing",
			edit: func(manifest *Manifest) { manifest.Service.TenantContext.ClientPhysicalRoutingSelectable = true },
			want: "must forbid client physical routing selection",
		},
		{
			name: "missing forbidden shard field",
			edit: func(manifest *Manifest) {
				manifest.Service.TenantContext.ForbiddenClientFields = []string{"dsn", "datasource", "database", "schema"}
			},
			want: "forbidden client fields must cover physical routing",
		},
		{name: "workload using admin session", edit: func(manifest *Manifest) {
			manifest.Service.Operations[0].AuthModes = []ServiceAuthMode{ServiceAuthAdminSession}
		}, want: `identity mode "workload" cannot use auth mode "admin-session"`},
		{name: "workload using api token", edit: func(manifest *Manifest) {
			manifest.Service.Operations[0].AuthModes = []ServiceAuthMode{ServiceAuthAPIToken}
		}, want: `identity mode "workload" cannot use auth mode "api-token"`},
		{name: "unsupported anonymous authentication", edit: func(manifest *Manifest) {
			manifest.Service.AuthModes = []ServiceAuthMode{ServiceAuthMode("none")}
			manifest.Service.Operations[0].AuthModes = []ServiceAuthMode{ServiceAuthMode("none")}
		}, want: `auth mode none is invalid`},
		{name: "management user using oauth2", edit: func(manifest *Manifest) {
			manifest.Service.Operations[0].IdentityMode = ServiceIdentityManagementUser
			manifest.Service.Operations[0].AuthModes = []ServiceAuthMode{ServiceAuthOAuth2ClientCredentials}
			manifest.Service.AuthModes = append(manifest.Service.AuthModes, ServiceAuthOAuth2ClientCredentials)
		}, want: `identity mode "management-user" cannot use auth mode "oauth2-client-credentials"`},
		{
			name: "tenant scope missing",
			edit: func(manifest *Manifest) { manifest.Service.Operations[0].DataScopes = nil },
			want: "tenant-required operation must declare data scopes",
		},
		{
			name: "physical routing argument",
			edit: func(manifest *Manifest) {
				manifest.Service.Operations[0].RequestSchema.RequiredFields = []string{"datasource"}
			},
			want: "schema must not expose physical routing field",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest := serviceContractTestManifest("files")
			tt.edit(&manifest)
			err := ValidateServiceContracts([]Manifest{manifest})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ValidateServiceContracts() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestValidateServiceContractsRequiresExplicitBoundSuccessStatus(t *testing.T) {
	manifest := serviceContractTestManifest("upload")
	operation := &manifest.Service.Operations[0]
	operation.RuntimeStatus = ServiceRuntimeBound
	operation.Method = "POST"
	operation.Path = "/api/app/files"
	operation.RequestMediaType = "multipart/form-data"
	operation.ResponseMediaType = "application/json"

	if err := ValidateServiceContracts([]Manifest{manifest}); err == nil || !strings.Contains(err.Error(), "success status must be between 200 and 299") {
		t.Fatalf("ValidateServiceContracts(missing success status) error = %v", err)
	}

	operation.SuccessStatus = 201
	if err := ValidateServiceContracts([]Manifest{manifest}); err != nil {
		t.Fatalf("ValidateServiceContracts(explicit success status) error = %v", err)
	}

	operation.RuntimeStatus = ServiceRuntimeContractOnly
	operation.Method = ""
	operation.Path = ""
	operation.RequestMediaType = ""
	operation.ResponseMediaType = ""
	if err := ValidateServiceContracts([]Manifest{manifest}); err == nil || !strings.Contains(err.Error(), "contract-only operation must not claim an HTTP binding") {
		t.Fatalf("ValidateServiceContracts(contract-only success status) error = %v", err)
	}
}

func TestValidateServiceContractsRejectsGlobalContractIdentifierCollisions(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Manifest, Manifest)
		want string
	}{
		{name: "operation id", edit: func(second *Manifest, first Manifest) {
			second.Service.Operations[0].ID = first.Service.Operations[0].ID
		}, want: "operation id"},
		{name: "event id", edit: func(second *Manifest, first Manifest) {
			second.Service.Events[0].ID = first.Service.Events[0].ID
		}, want: "event id"},
		{name: "event name", edit: func(second *Manifest, first Manifest) {
			second.Service.Events[0].Name = first.Service.Events[0].Name
		}, want: "event name"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			first := serviceContractTestManifest("alpha")
			second := serviceContractTestManifest("zeta")
			tt.edit(&second, first)
			err := ValidateServiceContracts([]Manifest{first, second})
			if err == nil || !strings.Contains(err.Error(), tt.want) || !strings.Contains(err.Error(), "already declared") {
				t.Fatalf("ValidateServiceContracts() error = %v, want global %s collision", err, tt.want)
			}
		})
	}
}

func TestValidateServiceContractsRejectsInvalidEventAndDeprecation(t *testing.T) {
	manifest := serviceContractTestManifest("files")
	manifest.Service.Events[0].Name = "platform.file.stored"
	if err := ValidateServiceContracts([]Manifest{manifest}); err == nil || !strings.Contains(err.Error(), "name must end with its version") {
		t.Fatalf("ValidateServiceContracts(event version) error = %v", err)
	}

	manifest = serviceContractTestManifest("files")
	manifest.Service.Operations[0].Compatibility = ServiceCompatibility{Mode: "semver", Deprecated: true, ReplacedBy: "store-file-v2"}
	if err := ValidateServiceContracts([]Manifest{manifest}); err == nil || !strings.Contains(err.Error(), "must declare RFC3339 sunset") {
		t.Fatalf("ValidateServiceContracts(deprecation) error = %v", err)
	}
}

func TestServiceContractDocumentFromManifestsIsDeterministicAndExplicit(t *testing.T) {
	alpha := serviceContractTestManifest("alpha")
	zeta := serviceContractTestManifest("zeta")
	forward, err := ServiceContractDocumentFromManifests([]Manifest{zeta, alpha})
	if err != nil {
		t.Fatalf("ServiceContractDocumentFromManifests() error = %v", err)
	}
	reverse, err := ServiceContractDocumentFromManifests([]Manifest{alpha, zeta})
	if err != nil {
		t.Fatalf("ServiceContractDocumentFromManifests(reverse) error = %v", err)
	}
	if forward.ContractHash != reverse.ContractHash {
		t.Fatalf("contract hashes differ: %q != %q", forward.ContractHash, reverse.ContractHash)
	}
	if len(forward.Services) != 2 || forward.Services[0].CapabilityID != "alpha" || forward.Services[1].CapabilityID != "zeta" {
		t.Fatalf("services = %+v, want deterministic capability order", forward.Services)
	}
	if forward.EventEnvelope.RuntimeStatus != "schema-only" || forward.TraceContext.Standard != "W3C Trace Context" {
		t.Fatalf("document standards = %+v/%+v", forward.EventEnvelope, forward.TraceContext)
	}
	if forward.EventEnvelope.TenantContextRequirement != "required-only-for-tenant-required-events" {
		t.Fatalf("event envelope tenant requirement = %q", forward.EventEnvelope.TenantContextRequirement)
	}
	if forward.TenantContext.ClientPhysicalRoutingSelectable {
		t.Fatalf("tenant context must fail closed: %+v", forward.TenantContext)
	}
}

func serviceContractTestManifest(id string) Manifest {
	return Manifest{
		ID: ID(id), Name: id, Version: "0.1.0",
		Service: ServiceSurface{
			ID: id, Owner: "platform-test", Audiences: []ServiceAudience{ServiceAudienceInternal}, Stability: ServiceStabilityStable, Version: "1.0.0",
			IdentityModes: []ServiceIdentityMode{ServiceIdentityManagementUser, ServiceIdentityWorkload},
			AuthModes:     []ServiceAuthMode{ServiceAuthAdminSession, ServiceAuthAPIToken, ServiceAuthWorkloadJWT},
			TenantContext: DefaultTrustedTenantContext(),
			Operations:    []ServiceOperation{serviceContractTestOperation("store-"+id, ServiceOperationCommand, ServicePlaneData, ServiceIdentityWorkload, []ServiceAuthMode{ServiceAuthWorkloadJWT})},
			Events: []ServiceEvent{{
				ID: id + "-stored", Name: "platform." + id + ".stored.v1", Version: 1, Direction: ServiceEventPublish, RuntimeStatus: ServiceRuntimeContractOnly,
				TenantMode: ServiceTenantRequired, DataScopes: []string{"tenant"}, PayloadSchema: ServicePayloadSchema{Ref: "#/schemas/FileStoredEvent", RequiredFields: []string{"fileId"}, PII: ServicePIIPersonal},
				EnvelopeVersion: "1.0", TraceContext: []string{"traceparent", "tracestate"}, Compatibility: ServiceCompatibility{Mode: "semver"}, Description: Text("文件已存储。", "File stored."),
			}},
			SLA: ServiceSLA{AvailabilityTarget: "99.9%", LatencyP95MS: 500}, Compatibility: ServiceCompatibility{Mode: "semver"},
			RuntimeBoundary: ServiceRuntimeBoundary{ContractExecution: "deferred", IdentityProtocols: "declaration-only", EventDelivery: "not-implemented", DatasourceRouting: "not-implemented", RuntimeSourceWriting: "disabled"},
		},
	}
}

func serviceContractTestOperation(id string, kind ServiceOperationKind, plane ServicePlane, identity ServiceIdentityMode, auth []ServiceAuthMode) ServiceOperation {
	return ServiceOperation{
		ID: id, Kind: kind, Plane: plane, RuntimeStatus: ServiceRuntimeContractOnly, IdentityMode: identity, AuthModes: auth,
		TenantMode: ServiceTenantRequired, DataScopes: []string{"tenant"}, RequestSchema: ServicePayloadSchema{Ref: "#/schemas/StoreFileRequest", RequiredFields: []string{"fileId"}, PII: ServicePIIPersonal},
		ResponseSchema: ServicePayloadSchema{Ref: "#/schemas/FileRecord", RequiredFields: []string{"fileId"}, PII: ServicePIIPersonal},
		Reliability:    ServiceReliability{Idempotency: "required-key", OptimisticConcurrency: "version", TimeoutMilliseconds: 2000, MaxRetries: 2, RateLimitPerMinute: 120, CostLimit: 10},
		Compatibility:  ServiceCompatibility{Mode: "semver"}, Description: Text("测试操作。", "Test operation."),
	}
}
