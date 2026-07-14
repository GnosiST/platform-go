package capability

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"
	"time"
)

const ServiceContractVersion = "1.0.0"

type ServicePlane string

const (
	ServicePlaneAdmin    ServicePlane = "admin"
	ServicePlaneData     ServicePlane = "service"
	ServicePlaneControl  ServicePlane = "control"
	ServicePlaneExternal ServicePlane = "external"
	ServicePlaneEvent    ServicePlane = "event"
)

type ServiceAudience string

const (
	ServiceAudienceOperator ServiceAudience = "operator"
	ServiceAudienceInternal ServiceAudience = "internal"
	ServiceAudiencePartner  ServiceAudience = "partner"
	ServiceAudiencePublic   ServiceAudience = "public"
)

type ServiceStability string

const (
	ServiceStabilityExperimental ServiceStability = "experimental"
	ServiceStabilityBeta         ServiceStability = "beta"
	ServiceStabilityStable       ServiceStability = "stable"
)

type ServiceIdentityMode string

const (
	ServiceIdentityManagementUser ServiceIdentityMode = "management-user"
	ServiceIdentityWorkload       ServiceIdentityMode = "workload"
)

type ServiceAuthMode string

const (
	ServiceAuthAdminSession            ServiceAuthMode = "admin-session"
	ServiceAuthAppSession              ServiceAuthMode = "app-session"
	ServiceAuthAPIToken                ServiceAuthMode = "api-token"
	ServiceAuthOAuth2ClientCredentials ServiceAuthMode = "oauth2-client-credentials"
	ServiceAuthMTLS                    ServiceAuthMode = "mtls"
	ServiceAuthWorkloadJWT             ServiceAuthMode = "workload-jwt"
)

type ServiceTenantMode string

const (
	ServiceTenantNone     ServiceTenantMode = "none"
	ServiceTenantRequired ServiceTenantMode = "required"
	ServiceTenantOptional ServiceTenantMode = "optional"
	ServiceTenantPlatform ServiceTenantMode = "platform"
)

type ServiceOperationKind string

const (
	ServiceOperationCommand ServiceOperationKind = "command"
	ServiceOperationQuery   ServiceOperationKind = "query"
)

type ServiceRuntimeStatus string

const (
	ServiceRuntimeBound        ServiceRuntimeStatus = "bound"
	ServiceRuntimeContractOnly ServiceRuntimeStatus = "contract-only"
)

type ServiceEventDirection string

const (
	ServiceEventPublish   ServiceEventDirection = "publish"
	ServiceEventSubscribe ServiceEventDirection = "subscribe"
)

type ServicePIIClass string

const (
	ServicePIINone      ServicePIIClass = "none"
	ServicePIIPersonal  ServicePIIClass = "personal"
	ServicePIISensitive ServicePIIClass = "sensitive"
	ServicePIISecret    ServicePIIClass = "secret"
)

type ServiceSurface struct {
	ID              string                 `json:"id,omitempty"`
	Owner           string                 `json:"owner,omitempty"`
	Audiences       []ServiceAudience      `json:"audiences,omitempty"`
	Stability       ServiceStability       `json:"stability,omitempty"`
	Version         string                 `json:"version,omitempty"`
	IdentityModes   []ServiceIdentityMode  `json:"identityModes,omitempty"`
	AuthModes       []ServiceAuthMode      `json:"authModes,omitempty"`
	TenantContext   TrustedTenantContext   `json:"tenantContext,omitempty"`
	Operations      []ServiceOperation     `json:"operations,omitempty"`
	Events          []ServiceEvent         `json:"events,omitempty"`
	SLA             ServiceSLA             `json:"sla,omitempty"`
	Compatibility   ServiceCompatibility   `json:"compatibility,omitempty"`
	RuntimeBoundary ServiceRuntimeBoundary `json:"runtimeBoundary,omitempty"`
}

type TrustedTenantContext struct {
	Fields                          []string `json:"fields,omitempty"`
	Provenance                      []string `json:"provenance,omitempty"`
	ClientPhysicalRoutingSelectable bool     `json:"clientPhysicalRoutingSelectable"`
	ForbiddenClientFields           []string `json:"forbiddenClientFields,omitempty"`
}

type ServiceOperation struct {
	ID                string               `json:"id"`
	Kind              ServiceOperationKind `json:"kind"`
	Plane             ServicePlane         `json:"plane"`
	RuntimeStatus     ServiceRuntimeStatus `json:"runtimeStatus"`
	IdentityMode      ServiceIdentityMode  `json:"identityMode"`
	AuthModes         []ServiceAuthMode    `json:"authModes"`
	TenantMode        ServiceTenantMode    `json:"tenantMode"`
	Permissions       []string             `json:"permissions,omitempty"`
	DataScopes        []string             `json:"dataScopes,omitempty"`
	Method            string               `json:"method,omitempty"`
	Path              string               `json:"path,omitempty"`
	RequestMediaType  string               `json:"requestMediaType,omitempty"`
	ResponseMediaType string               `json:"responseMediaType,omitempty"`
	SuccessStatus     int                  `json:"successStatus,omitempty"`
	RequestSchema     ServicePayloadSchema `json:"requestSchema"`
	ResponseSchema    ServicePayloadSchema `json:"responseSchema"`
	Reliability       ServiceReliability   `json:"reliability"`
	Compatibility     ServiceCompatibility `json:"compatibility"`
	Description       LocalizedText        `json:"description"`
}

type ServiceEvent struct {
	ID              string                `json:"id"`
	Name            string                `json:"name"`
	Version         uint32                `json:"version"`
	Direction       ServiceEventDirection `json:"direction"`
	RuntimeStatus   ServiceRuntimeStatus  `json:"runtimeStatus"`
	TenantMode      ServiceTenantMode     `json:"tenantMode"`
	Permissions     []string              `json:"permissions,omitempty"`
	DataScopes      []string              `json:"dataScopes,omitempty"`
	PayloadSchema   ServicePayloadSchema  `json:"payloadSchema"`
	EnvelopeVersion string                `json:"envelopeVersion"`
	TraceContext    []string              `json:"traceContext"`
	Compatibility   ServiceCompatibility  `json:"compatibility"`
	Description     LocalizedText         `json:"description"`
}

type ServicePayloadSchema struct {
	Ref            string          `json:"ref"`
	RequiredFields []string        `json:"requiredFields,omitempty"`
	PII            ServicePIIClass `json:"pii"`
}

type ServiceReliability struct {
	Idempotency           string `json:"idempotency"`
	OptimisticConcurrency string `json:"optimisticConcurrency"`
	TimeoutMilliseconds   int    `json:"timeoutMilliseconds"`
	MaxRetries            int    `json:"maxRetries"`
	RateLimitPerMinute    int    `json:"rateLimitPerMinute"`
	CostLimit             int    `json:"costLimit"`
}

type ServiceSLA struct {
	AvailabilityTarget string `json:"availabilityTarget"`
	LatencyP95MS       int    `json:"latencyP95Ms"`
}

type ServiceCompatibility struct {
	Mode       string `json:"mode"`
	Deprecated bool   `json:"deprecated"`
	SunsetAt   string `json:"sunsetAt,omitempty"`
	ReplacedBy string `json:"replacedBy,omitempty"`
}

type ServiceRuntimeBoundary struct {
	ContractExecution    string `json:"contractExecution"`
	IdentityProtocols    string `json:"identityProtocols"`
	EventDelivery        string `json:"eventDelivery"`
	DatasourceRouting    string `json:"datasourceRouting"`
	RuntimeSourceWriting string `json:"runtimeSourceWriting"`
}

type ServiceContractDocument struct {
	GeneratedBy     string                       `json:"generatedBy"`
	Source          string                       `json:"source"`
	SourceMode      string                       `json:"sourceMode"`
	ContractVersion string                       `json:"contractVersion"`
	SourceVersion   string                       `json:"sourceVersion"`
	ContractHash    string                       `json:"contractHash"`
	Policies        ServiceContractPolicies      `json:"policies"`
	TenantContext   TrustedTenantContext         `json:"tenantContext"`
	TraceContext    ServiceTraceContextStandard  `json:"traceContext"`
	EventEnvelope   ServiceEventEnvelopeStandard `json:"eventEnvelope"`
	Services        []ServiceContractEntry       `json:"services"`
}

type ServiceContractPolicies struct {
	Planes        []ServicePlane         `json:"planes"`
	IdentityModes []ServiceIdentityMode  `json:"identityModes"`
	AuthModes     []ServiceAuthMode      `json:"authModes"`
	TenantModes   []ServiceTenantMode    `json:"tenantModes"`
	RuntimeStatus []ServiceRuntimeStatus `json:"runtimeStatuses"`
}

type ServiceTraceContextStandard struct {
	Standard string   `json:"standard"`
	Fields   []string `json:"fields"`
}

type ServiceEventEnvelopeStandard struct {
	Version                  string   `json:"version"`
	RequiredFields           []string `json:"requiredFields"`
	TenantContextRequirement string   `json:"tenantContextRequirement"`
	RuntimeStatus            string   `json:"runtimeStatus"`
}

type ServiceContractEntry struct {
	CapabilityID string `json:"capabilityId"`
	ServiceSurface
}

var (
	servicePlanes                  = []ServicePlane{ServicePlaneAdmin, ServicePlaneData, ServicePlaneControl, ServicePlaneExternal, ServicePlaneEvent}
	serviceIdentityModes           = []ServiceIdentityMode{ServiceIdentityManagementUser, ServiceIdentityWorkload}
	serviceAuthModes               = []ServiceAuthMode{ServiceAuthAdminSession, ServiceAuthAppSession, ServiceAuthAPIToken, ServiceAuthOAuth2ClientCredentials, ServiceAuthMTLS, ServiceAuthWorkloadJWT}
	serviceTenantModes             = []ServiceTenantMode{ServiceTenantNone, ServiceTenantRequired, ServiceTenantOptional, ServiceTenantPlatform}
	serviceRuntimeStatuses         = []ServiceRuntimeStatus{ServiceRuntimeBound, ServiceRuntimeContractOnly}
	trustedTenantFields            = []string{"tenantId", "tenantCode", "organizationId", "configurationVersion"}
	trustedTenantProvenance        = []string{"authenticated-identity", "trusted-gateway", "authorized-control-plane-override"}
	forbiddenPhysicalRoutingFields = []string{"dsn", "datasource", "database", "schema", "shard"}
	traceContextFields             = []string{"traceparent", "tracestate"}
	eventEnvelopeFields            = []string{"eventId", "eventType", "eventVersion", "occurredAt", "producer", "tenantContext", "traceContext", "payload"}
)

func DefaultTrustedTenantContext() TrustedTenantContext {
	return TrustedTenantContext{
		Fields:                append([]string(nil), trustedTenantFields...),
		Provenance:            append([]string(nil), trustedTenantProvenance...),
		ForbiddenClientFields: append([]string(nil), forbiddenPhysicalRoutingFields...),
	}
}

func ValidateServiceContracts(manifests []Manifest) error {
	seenServices := map[string]ID{}
	seenOperations := map[string]string{}
	seenEventIDs := map[string]string{}
	seenEventNames := map[string]string{}
	for _, manifest := range manifests {
		service := manifest.Service
		if reflect.ValueOf(service).IsZero() {
			continue
		}
		if err := validateServiceSurface(manifest.ID, service); err != nil {
			return err
		}
		if owner, exists := seenServices[service.ID]; exists {
			return fmt.Errorf("capability %q service id %q already declared by capability %q", manifest.ID, service.ID, owner)
		}
		seenServices[service.ID] = manifest.ID
		for _, operation := range service.Operations {
			if owner, exists := seenOperations[operation.ID]; exists {
				return fmt.Errorf("capability %q service %q operation id %q already declared by %s", manifest.ID, service.ID, operation.ID, owner)
			}
			seenOperations[operation.ID] = fmt.Sprintf("capability %q service %q", manifest.ID, service.ID)
		}
		for _, event := range service.Events {
			if owner, exists := seenEventIDs[event.ID]; exists {
				return fmt.Errorf("capability %q service %q event id %q already declared by %s", manifest.ID, service.ID, event.ID, owner)
			}
			seenEventIDs[event.ID] = fmt.Sprintf("capability %q service %q", manifest.ID, service.ID)
			if owner, exists := seenEventNames[event.Name]; exists {
				return fmt.Errorf("capability %q service %q event name %q already declared by %s", manifest.ID, service.ID, event.Name, owner)
			}
			seenEventNames[event.Name] = fmt.Sprintf("capability %q service %q", manifest.ID, service.ID)
		}
	}
	return nil
}

func validateServiceSurface(capabilityID ID, service ServiceSurface) error {
	if service.ID != strings.TrimSpace(service.ID) || !validCapabilityIdentifier(service.ID) {
		return fmt.Errorf("capability %q service id must use lowercase letters, numbers, and hyphens", capabilityID)
	}
	if strings.TrimSpace(service.Owner) == "" {
		return fmt.Errorf("capability %q service %q owner is required", capabilityID, service.ID)
	}
	if !validCapabilityVersion(service.Version) {
		return fmt.Errorf("capability %q service %q version must use numeric semver", capabilityID, service.ID)
	}
	if !slices.Contains([]ServiceStability{ServiceStabilityExperimental, ServiceStabilityBeta, ServiceStabilityStable}, service.Stability) {
		return fmt.Errorf("capability %q service %q stability is invalid", capabilityID, service.ID)
	}
	if err := requireUniqueKnown(service.Audiences, []ServiceAudience{ServiceAudienceOperator, ServiceAudienceInternal, ServiceAudiencePartner, ServiceAudiencePublic}, "audience"); err != nil {
		return fmt.Errorf("capability %q service %q %w", capabilityID, service.ID, err)
	}
	if err := requireUniqueKnown(service.IdentityModes, serviceIdentityModes, "identity mode"); err != nil {
		return fmt.Errorf("capability %q service %q %w", capabilityID, service.ID, err)
	}
	if err := requireUniqueKnown(service.AuthModes, serviceAuthModes, "auth mode"); err != nil {
		return fmt.Errorf("capability %q service %q %w", capabilityID, service.ID, err)
	}
	if err := validateServiceAuthModes(service.IdentityModes, service.AuthModes); err != nil {
		return fmt.Errorf("capability %q service %q %w", capabilityID, service.ID, err)
	}
	if err := validateTrustedTenantContext(capabilityID, service.ID, service.TenantContext); err != nil {
		return err
	}
	if len(service.Operations) == 0 && len(service.Events) == 0 {
		return fmt.Errorf("capability %q service %q must declare an operation or event", capabilityID, service.ID)
	}
	if err := validateServiceSLA(capabilityID, service.ID, service.SLA); err != nil {
		return err
	}
	if err := validateServiceCompatibility(capabilityID, service.ID, "service", service.Compatibility); err != nil {
		return err
	}
	if service.RuntimeBoundary.ContractExecution == "" || service.RuntimeBoundary.IdentityProtocols == "" || service.RuntimeBoundary.EventDelivery == "" || service.RuntimeBoundary.DatasourceRouting == "" || service.RuntimeBoundary.RuntimeSourceWriting == "" {
		return fmt.Errorf("capability %q service %q runtime boundary must be explicit", capabilityID, service.ID)
	}

	seen := map[string]struct{}{}
	for _, operation := range service.Operations {
		if _, exists := seen[operation.ID]; exists {
			return fmt.Errorf("capability %q service %q duplicate operation id %q", capabilityID, service.ID, operation.ID)
		}
		seen[operation.ID] = struct{}{}
		if err := validateServiceOperation(capabilityID, service, operation); err != nil {
			return err
		}
	}
	for _, event := range service.Events {
		if _, exists := seen[event.ID]; exists {
			return fmt.Errorf("capability %q service %q duplicate contract id %q", capabilityID, service.ID, event.ID)
		}
		seen[event.ID] = struct{}{}
		if err := validateServiceEvent(capabilityID, service, event); err != nil {
			return err
		}
	}
	return nil
}

func validateTrustedTenantContext(capabilityID ID, serviceID string, context TrustedTenantContext) error {
	if context.ClientPhysicalRoutingSelectable {
		return fmt.Errorf("capability %q service %q tenant context must forbid client physical routing selection", capabilityID, serviceID)
	}
	if !sameStringSet(context.Fields, trustedTenantFields) {
		return fmt.Errorf("capability %q service %q tenant context fields must match the trusted standard", capabilityID, serviceID)
	}
	if !sameStringSet(context.Provenance, trustedTenantProvenance) {
		return fmt.Errorf("capability %q service %q tenant context provenance must match the trusted standard", capabilityID, serviceID)
	}
	if !sameStringSet(context.ForbiddenClientFields, forbiddenPhysicalRoutingFields) {
		return fmt.Errorf("capability %q service %q forbidden client fields must cover physical routing", capabilityID, serviceID)
	}
	return nil
}

func validateServiceOperation(capabilityID ID, service ServiceSurface, operation ServiceOperation) error {
	prefix := fmt.Sprintf("capability %q service %q operation %q", capabilityID, service.ID, operation.ID)
	if !validCapabilityIdentifier(operation.ID) {
		return fmt.Errorf("%s id must use lowercase letters, numbers, and hyphens", prefix)
	}
	if operation.Kind != ServiceOperationCommand && operation.Kind != ServiceOperationQuery {
		return fmt.Errorf("%s kind is invalid", prefix)
	}
	if operation.Plane == ServicePlaneEvent || !slices.Contains(servicePlanes, operation.Plane) {
		return fmt.Errorf("%s plane must be admin, service, control, or external", prefix)
	}
	if !slices.Contains(serviceRuntimeStatuses, operation.RuntimeStatus) {
		return fmt.Errorf("%s runtime status is invalid", prefix)
	}
	if !slices.Contains(service.IdentityModes, operation.IdentityMode) {
		return fmt.Errorf("%s identity mode must be declared by the service", prefix)
	}
	if err := requireUniqueKnown(operation.AuthModes, service.AuthModes, "auth mode"); err != nil {
		return fmt.Errorf("%s %w", prefix, err)
	}
	if err := validateOperationAuthModes(operation.IdentityMode, operation.AuthModes); err != nil {
		return fmt.Errorf("%s %w", prefix, err)
	}
	if !slices.Contains(serviceTenantModes, operation.TenantMode) {
		return fmt.Errorf("%s tenant mode is invalid", prefix)
	}
	if operation.TenantMode == ServiceTenantRequired && len(operation.DataScopes) == 0 {
		return fmt.Errorf("%s tenant-required operation must declare data scopes", prefix)
	}
	if operation.RuntimeStatus == ServiceRuntimeBound {
		if strings.TrimSpace(operation.Method) == "" || !strings.HasPrefix(operation.Path, "/api/") {
			return fmt.Errorf("%s bound operation must declare an HTTP method and /api/ path", prefix)
		}
		if operation.ResponseMediaType == "" || (operation.Method != "GET" && operation.Method != "DELETE" && operation.RequestMediaType == "") {
			return fmt.Errorf("%s bound operation must declare request and response media types", prefix)
		}
		if operation.SuccessStatus < 200 || operation.SuccessStatus > 299 {
			return fmt.Errorf("%s bound operation success status must be between 200 and 299", prefix)
		}
	} else if operation.Method != "" || operation.Path != "" || operation.RequestMediaType != "" || operation.ResponseMediaType != "" || operation.SuccessStatus != 0 {
		return fmt.Errorf("%s contract-only operation must not claim an HTTP binding", prefix)
	}
	if err := validatePermissionList(prefix, operation.Permissions); err != nil {
		return err
	}
	if err := validatePayloadSchema(prefix+" request", operation.RequestSchema); err != nil {
		return err
	}
	if err := validatePayloadSchema(prefix+" response", operation.ResponseSchema); err != nil {
		return err
	}
	if err := validateReliability(prefix, operation.Reliability); err != nil {
		return err
	}
	return validateServiceCompatibility(capabilityID, service.ID, "operation "+operation.ID, operation.Compatibility)
}

func validateServiceAuthModes(identityModes []ServiceIdentityMode, authModes []ServiceAuthMode) error {
	for _, authMode := range authModes {
		compatible := false
		for _, identityMode := range identityModes {
			if slices.Contains(allowedAuthModes(identityMode), authMode) {
				compatible = true
				break
			}
		}
		if !compatible {
			return fmt.Errorf("auth mode %q is incompatible with declared identity modes", authMode)
		}
	}
	return nil
}

func validateOperationAuthModes(identityMode ServiceIdentityMode, authModes []ServiceAuthMode) error {
	allowed := allowedAuthModes(identityMode)
	for _, authMode := range authModes {
		if !slices.Contains(allowed, authMode) {
			return fmt.Errorf("identity mode %q cannot use auth mode %q", identityMode, authMode)
		}
	}
	return nil
}

func allowedAuthModes(identityMode ServiceIdentityMode) []ServiceAuthMode {
	switch identityMode {
	case ServiceIdentityManagementUser:
		return []ServiceAuthMode{ServiceAuthAdminSession, ServiceAuthAppSession, ServiceAuthAPIToken}
	case ServiceIdentityWorkload:
		return []ServiceAuthMode{ServiceAuthOAuth2ClientCredentials, ServiceAuthMTLS, ServiceAuthWorkloadJWT}
	default:
		return nil
	}
}

func validateServiceEvent(capabilityID ID, service ServiceSurface, event ServiceEvent) error {
	prefix := fmt.Sprintf("capability %q service %q event %q", capabilityID, service.ID, event.ID)
	if !validCapabilityIdentifier(event.ID) {
		return fmt.Errorf("%s id must use lowercase letters, numbers, and hyphens", prefix)
	}
	if event.Version == 0 || !strings.HasSuffix(event.Name, fmt.Sprintf(".v%d", event.Version)) {
		return fmt.Errorf("%s name must end with its version", prefix)
	}
	if event.Direction != ServiceEventPublish && event.Direction != ServiceEventSubscribe {
		return fmt.Errorf("%s direction is invalid", prefix)
	}
	if !slices.Contains(serviceRuntimeStatuses, event.RuntimeStatus) {
		return fmt.Errorf("%s runtime status is invalid", prefix)
	}
	if !slices.Contains(serviceTenantModes, event.TenantMode) {
		return fmt.Errorf("%s tenant mode is invalid", prefix)
	}
	if event.TenantMode == ServiceTenantRequired && len(event.DataScopes) == 0 {
		return fmt.Errorf("%s tenant-required event must declare data scopes", prefix)
	}
	if event.EnvelopeVersion != "1.0" {
		return fmt.Errorf("%s envelope version must be 1.0", prefix)
	}
	if !sameStringSet(event.TraceContext, traceContextFields) {
		return fmt.Errorf("%s trace context must use W3C traceparent and tracestate", prefix)
	}
	if err := validatePermissionList(prefix, event.Permissions); err != nil {
		return err
	}
	if err := validatePayloadSchema(prefix+" payload", event.PayloadSchema); err != nil {
		return err
	}
	return validateServiceCompatibility(capabilityID, service.ID, "event "+event.ID, event.Compatibility)
}

func validatePayloadSchema(prefix string, schema ServicePayloadSchema) error {
	if !strings.HasPrefix(schema.Ref, "#/schemas/") {
		return fmt.Errorf("%s schema ref must use #/schemas/", prefix)
	}
	if !slices.Contains([]ServicePIIClass{ServicePIINone, ServicePIIPersonal, ServicePIISensitive, ServicePIISecret}, schema.PII) {
		return fmt.Errorf("%s schema PII classification is invalid", prefix)
	}
	for _, field := range schema.RequiredFields {
		if slices.Contains(forbiddenPhysicalRoutingFields, strings.ToLower(field)) {
			return fmt.Errorf("%s schema must not expose physical routing field %q", prefix, field)
		}
	}
	return uniqueNonEmptyStrings(schema.RequiredFields, prefix+" required fields")
}

func validateReliability(prefix string, reliability ServiceReliability) error {
	if !slices.Contains([]string{"none", "required-key", "server-derived"}, reliability.Idempotency) {
		return fmt.Errorf("%s idempotency mode is invalid", prefix)
	}
	if !slices.Contains([]string{"none", "etag", "version"}, reliability.OptimisticConcurrency) {
		return fmt.Errorf("%s optimistic concurrency mode is invalid", prefix)
	}
	if reliability.TimeoutMilliseconds <= 0 || reliability.MaxRetries < 0 || reliability.RateLimitPerMinute <= 0 || reliability.CostLimit <= 0 {
		return fmt.Errorf("%s reliability limits must be explicit positive values", prefix)
	}
	return nil
}

func validateServiceSLA(capabilityID ID, serviceID string, sla ServiceSLA) error {
	if strings.TrimSpace(sla.AvailabilityTarget) == "" || sla.LatencyP95MS <= 0 {
		return fmt.Errorf("capability %q service %q SLA must be explicit", capabilityID, serviceID)
	}
	return nil
}

func validateServiceCompatibility(capabilityID ID, serviceID string, subject string, compatibility ServiceCompatibility) error {
	if compatibility.Mode != "semver" {
		return fmt.Errorf("capability %q service %q %s compatibility mode must be semver", capabilityID, serviceID, subject)
	}
	if !compatibility.Deprecated && (compatibility.SunsetAt != "" || compatibility.ReplacedBy != "") {
		return fmt.Errorf("capability %q service %q %s cannot declare sunset metadata unless deprecated", capabilityID, serviceID, subject)
	}
	if compatibility.Deprecated {
		if strings.TrimSpace(compatibility.ReplacedBy) == "" {
			return fmt.Errorf("capability %q service %q %s deprecated contract must declare replacement", capabilityID, serviceID, subject)
		}
		if _, err := time.Parse(time.RFC3339, compatibility.SunsetAt); err != nil {
			return fmt.Errorf("capability %q service %q %s deprecated contract must declare RFC3339 sunset", capabilityID, serviceID, subject)
		}
	}
	return nil
}

func validatePermissionList(prefix string, permissions []string) error {
	if err := uniqueNonEmptyStrings(permissions, prefix+" permissions"); err != nil {
		return err
	}
	for _, permission := range permissions {
		if strings.ContainsAny(permission, " \t\r\n") || !strings.Contains(permission, ":") {
			return fmt.Errorf("%s permission %q is invalid", prefix, permission)
		}
	}
	return nil
}

func uniqueNonEmptyStrings(values []string, label string) error {
	seen := map[string]struct{}{}
	for _, value := range values {
		if value == "" || value != strings.TrimSpace(value) {
			return fmt.Errorf("%s must not contain blank values", label)
		}
		if _, exists := seen[value]; exists {
			return fmt.Errorf("%s contains duplicate %q", label, value)
		}
		seen[value] = struct{}{}
	}
	return nil
}

func requireUniqueKnown[T comparable](values []T, allowed []T, label string) error {
	if len(values) == 0 {
		return fmt.Errorf("%s is required", label)
	}
	seen := map[T]struct{}{}
	for _, value := range values {
		if !slices.Contains(allowed, value) {
			return fmt.Errorf("%s %v is invalid", label, value)
		}
		if _, exists := seen[value]; exists {
			return fmt.Errorf("duplicate %s %v", label, value)
		}
		seen[value] = struct{}{}
	}
	return nil
}

func sameStringSet(actual []string, expected []string) bool {
	if len(actual) != len(expected) {
		return false
	}
	copyActual := append([]string(nil), actual...)
	copyExpected := append([]string(nil), expected...)
	sort.Strings(copyActual)
	sort.Strings(copyExpected)
	return slices.Equal(copyActual, copyExpected)
}

func ServiceContractDocumentFromManifests(manifests []Manifest) (ServiceContractDocument, error) {
	if err := ValidateServiceContracts(manifests); err != nil {
		return ServiceContractDocument{}, err
	}
	services := make([]ServiceContractEntry, 0)
	versions := make([]string, 0)
	for _, manifest := range manifests {
		if manifest.Service.ID == "" {
			continue
		}
		service := cloneServiceSurface(manifest.Service)
		sort.Slice(service.Operations, func(i, j int) bool { return service.Operations[i].ID < service.Operations[j].ID })
		sort.Slice(service.Events, func(i, j int) bool { return service.Events[i].ID < service.Events[j].ID })
		services = append(services, ServiceContractEntry{CapabilityID: string(manifest.ID), ServiceSurface: service})
		versions = append(versions, manifest.Version)
	}
	sort.Slice(services, func(i, j int) bool { return services[i].CapabilityID < services[j].CapabilityID })
	sourceVersion := "mixed"
	if len(versions) == 1 {
		sourceVersion = versions[0]
	}
	payload, err := json.Marshal(services)
	if err != nil {
		return ServiceContractDocument{}, err
	}
	hash := sha256.Sum256(payload)
	return ServiceContractDocument{
		GeneratedBy:     "cmd/platform-contracts service-manifests",
		Source:          "capability.Manifest.Service",
		SourceMode:      "go-manifest",
		ContractVersion: ServiceContractVersion,
		SourceVersion:   sourceVersion,
		ContractHash:    "sha256:" + hex.EncodeToString(hash[:]),
		Policies: ServiceContractPolicies{
			Planes: append([]ServicePlane(nil), servicePlanes...), IdentityModes: append([]ServiceIdentityMode(nil), serviceIdentityModes...), AuthModes: append([]ServiceAuthMode(nil), serviceAuthModes...), TenantModes: append([]ServiceTenantMode(nil), serviceTenantModes...), RuntimeStatus: append([]ServiceRuntimeStatus(nil), serviceRuntimeStatuses...),
		},
		TenantContext: DefaultTrustedTenantContext(),
		TraceContext:  ServiceTraceContextStandard{Standard: "W3C Trace Context", Fields: append([]string(nil), traceContextFields...)},
		EventEnvelope: ServiceEventEnvelopeStandard{
			Version:                  "1.0",
			RequiredFields:           append([]string(nil), eventEnvelopeFields...),
			TenantContextRequirement: "required-only-for-tenant-required-events",
			RuntimeStatus:            "schema-only",
		},
		Services: services,
	}, nil
}

func cloneServiceSurface(service ServiceSurface) ServiceSurface {
	clone := service
	clone.Audiences = append([]ServiceAudience(nil), service.Audiences...)
	clone.IdentityModes = append([]ServiceIdentityMode(nil), service.IdentityModes...)
	clone.AuthModes = append([]ServiceAuthMode(nil), service.AuthModes...)
	clone.TenantContext.Fields = append([]string(nil), service.TenantContext.Fields...)
	clone.TenantContext.Provenance = append([]string(nil), service.TenantContext.Provenance...)
	clone.TenantContext.ForbiddenClientFields = append([]string(nil), service.TenantContext.ForbiddenClientFields...)
	clone.Operations = append([]ServiceOperation(nil), service.Operations...)
	clone.Events = append([]ServiceEvent(nil), service.Events...)
	return clone
}
