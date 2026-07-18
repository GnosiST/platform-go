// Package capability exposes stable business-neutral capability contracts for
// downstream modules. Runtime implementation remains under internal/.
package capability

import (
	"context"
	"database/sql"
	"time"

	internal "github.com/GnosiST/platform-go/internal/platform/capability"
	"gorm.io/gorm"
)

type (
	ID                           = internal.ID
	Manifest                     = internal.Manifest
	LocalizedText                = internal.LocalizedText
	AdminSurface                 = internal.AdminSurface
	AdminResource                = internal.AdminResource
	AdminResourceDeletionPolicy  = internal.AdminResourceDeletionPolicy
	AdminMenu                    = internal.AdminMenu
	AdminField                   = internal.AdminField
	AdminFieldProtection         = internal.AdminFieldProtection
	AdminFieldMasking            = internal.AdminFieldMasking
	AdminFieldReveal             = internal.AdminFieldReveal
	AdminRevealPolicy            = internal.AdminRevealPolicy
	AdminRevealPurpose           = internal.AdminRevealPurpose
	AdminResourceProtection      = internal.AdminResourceProtection
	AdminFieldRelation           = internal.AdminFieldRelation
	AdminFieldRelationFilter     = internal.AdminFieldRelationFilter
	AdminFormGroup               = internal.AdminFormGroup
	AdminResourceAction          = internal.AdminResourceAction
	AdminActionConfirm           = internal.AdminActionConfirm
	AdminResourcePanel           = internal.AdminResourcePanel
	AdminRuntimeSlot             = internal.AdminRuntimeSlot
	AdminRuntimeSlotDataBinding  = internal.AdminRuntimeSlotDataBinding
	AdminFieldValidation         = internal.AdminFieldValidation
	AdminFieldOption             = internal.AdminFieldOption
	AppSurface                   = internal.AppSurface
	AppRouteAuth                 = internal.AppRouteAuth
	AppRoute                     = internal.AppRoute
	AppRouteContract             = internal.AppRouteContract
	AuthProviderAudience         = internal.AuthProviderAudience
	AuthProvider                 = internal.AuthProvider
	Migration                    = internal.Migration
	Seed                         = internal.Seed
	DemoDataSet                  = internal.DemoDataSet
	DemoRecord                   = internal.DemoRecord
	Hooks                        = internal.Hooks
	Hook                         = internal.Hook
	LifecycleStep                = internal.LifecycleStep
	Runtime                      = internal.Runtime
	LifecycleKind                = internal.LifecycleKind
	LifecycleRecord              = internal.LifecycleRecord
	LifecycleHistory             = internal.LifecycleHistory
	MemoryLifecycleHistory       = internal.MemoryLifecycleHistory
	FileLifecycleHistory         = internal.FileLifecycleHistory
	SQLLifecycleHistory          = internal.SQLLifecycleHistory
	GORMLifecycleHistory         = internal.GORMLifecycleHistory
	RecordedLifecycleExecutor    = internal.RecordedLifecycleExecutor
	MigrationExecution           = internal.MigrationExecution
	SeedExecution                = internal.SeedExecution
	MigrationExecutor            = internal.MigrationExecutor
	SeedExecutor                 = internal.SeedExecutor
	MigrationExecutorFunc        = internal.MigrationExecutorFunc
	SeedExecutorFunc             = internal.SeedExecutorFunc
	Registry                     = internal.Registry
	ServicePlane                 = internal.ServicePlane
	ServiceAudience              = internal.ServiceAudience
	ServiceStability             = internal.ServiceStability
	ServiceIdentityMode          = internal.ServiceIdentityMode
	ServiceAuthMode              = internal.ServiceAuthMode
	ServiceTenantMode            = internal.ServiceTenantMode
	ServiceOperationKind         = internal.ServiceOperationKind
	ServiceRuntimeStatus         = internal.ServiceRuntimeStatus
	ServiceEventDirection        = internal.ServiceEventDirection
	ServicePIIClass              = internal.ServicePIIClass
	ServiceSurface               = internal.ServiceSurface
	TrustedTenantContext         = internal.TrustedTenantContext
	ServiceOperation             = internal.ServiceOperation
	ServiceEvent                 = internal.ServiceEvent
	ServicePayloadSchema         = internal.ServicePayloadSchema
	ServiceReliability           = internal.ServiceReliability
	ServiceSLA                   = internal.ServiceSLA
	ServiceCompatibility         = internal.ServiceCompatibility
	ServiceRuntimeBoundary       = internal.ServiceRuntimeBoundary
	ServiceContractDocument      = internal.ServiceContractDocument
	ServiceContractPolicies      = internal.ServiceContractPolicies
	ServiceTraceContextStandard  = internal.ServiceTraceContextStandard
	ServiceEventEnvelopeStandard = internal.ServiceEventEnvelopeStandard
	ServiceContractEntry         = internal.ServiceContractEntry
)

const (
	FieldSensitivityPublic    = internal.FieldSensitivityPublic
	FieldSensitivityInternal  = internal.FieldSensitivityInternal
	FieldSensitivityPersonal  = internal.FieldSensitivityPersonal
	FieldSensitivitySensitive = internal.FieldSensitivitySensitive
	FieldSensitivitySecret    = internal.FieldSensitivitySecret

	FieldStoragePlain     = internal.FieldStoragePlain
	FieldStorageMasked    = internal.FieldStorageMasked
	FieldStorageHashed    = internal.FieldStorageHashed
	FieldStorageEncrypted = internal.FieldStorageEncrypted

	FieldProjectionFull       = internal.FieldProjectionFull
	FieldProjectionMasked     = internal.FieldProjectionMasked
	FieldProjectionPrivileged = internal.FieldProjectionPrivileged
	FieldProjectionOmitted    = internal.FieldProjectionOmitted

	AdminRevealModeAnyOf = internal.AdminRevealModeAnyOf
	AdminRevealModeAllOf = internal.AdminRevealModeAllOf

	AdminRevealFactorOIDCReauthentication = internal.AdminRevealFactorOIDCReauthentication
	AdminRevealFactorSMSOTP               = internal.AdminRevealFactorSMSOTP

	AdminDeletionDisabled   = internal.AdminDeletionDisabled
	AdminDeletionAppendOnly = internal.AdminDeletionAppendOnly
	AdminDeletionRestrict   = internal.AdminDeletionRestrict
	AdminDeletionSoftDelete = internal.AdminDeletionSoftDelete
	AdminDeletionRevoke     = internal.AdminDeletionRevoke
	AdminDeletionTombstone  = internal.AdminDeletionTombstone
	AdminDeletionHardDelete = internal.AdminDeletionHardDelete

	MaximumAdminRetentionDays = internal.MaximumAdminRetentionDays

	AppRouteAuthPublic  = internal.AppRouteAuthPublic
	AppRouteAuthSession = internal.AppRouteAuthSession

	AuthProviderAudienceAdmin = internal.AuthProviderAudienceAdmin
	AuthProviderAudienceApp   = internal.AuthProviderAudienceApp

	ServiceContractVersion = internal.ServiceContractVersion

	ServicePlaneAdmin    = internal.ServicePlaneAdmin
	ServicePlaneData     = internal.ServicePlaneData
	ServicePlaneControl  = internal.ServicePlaneControl
	ServicePlaneExternal = internal.ServicePlaneExternal
	ServicePlaneEvent    = internal.ServicePlaneEvent

	ServiceAudienceOperator = internal.ServiceAudienceOperator
	ServiceAudienceInternal = internal.ServiceAudienceInternal
	ServiceAudiencePartner  = internal.ServiceAudiencePartner
	ServiceAudiencePublic   = internal.ServiceAudiencePublic

	ServiceStabilityExperimental = internal.ServiceStabilityExperimental
	ServiceStabilityBeta         = internal.ServiceStabilityBeta
	ServiceStabilityStable       = internal.ServiceStabilityStable

	ServiceIdentityManagementUser = internal.ServiceIdentityManagementUser
	ServiceIdentityWorkload       = internal.ServiceIdentityWorkload

	ServiceAuthAdminSession            = internal.ServiceAuthAdminSession
	ServiceAuthAppSession              = internal.ServiceAuthAppSession
	ServiceAuthAPIToken                = internal.ServiceAuthAPIToken
	ServiceAuthOAuth2ClientCredentials = internal.ServiceAuthOAuth2ClientCredentials
	ServiceAuthMTLS                    = internal.ServiceAuthMTLS
	ServiceAuthWorkloadJWT             = internal.ServiceAuthWorkloadJWT

	ServiceTenantNone     = internal.ServiceTenantNone
	ServiceTenantRequired = internal.ServiceTenantRequired
	ServiceTenantOptional = internal.ServiceTenantOptional
	ServiceTenantPlatform = internal.ServiceTenantPlatform

	ServiceOperationCommand = internal.ServiceOperationCommand
	ServiceOperationQuery   = internal.ServiceOperationQuery

	ServiceRuntimeBound        = internal.ServiceRuntimeBound
	ServiceRuntimeContractOnly = internal.ServiceRuntimeContractOnly

	ServiceEventPublish   = internal.ServiceEventPublish
	ServiceEventSubscribe = internal.ServiceEventSubscribe

	ServicePIINone      = internal.ServicePIINone
	ServicePIIPersonal  = internal.ServicePIIPersonal
	ServicePIISensitive = internal.ServicePIISensitive
	ServicePIISecret    = internal.ServicePIISecret

	LifecycleKindMigration = internal.LifecycleKindMigration
	LifecycleKindSeed      = internal.LifecycleKindSeed
)

func Text(zh string, en string) LocalizedText {
	return internal.Text(zh, en)
}

func NewRegistry() *Registry {
	return internal.NewRegistry()
}

func RunLifecycle(ctx context.Context, ordered []Manifest, runtime Runtime) error {
	return internal.RunLifecycle(ctx, ordered, runtime)
}

func NewMemoryLifecycleHistory() *MemoryLifecycleHistory {
	return internal.NewMemoryLifecycleHistory()
}

func NewFileLifecycleHistory(path string) (*FileLifecycleHistory, error) {
	return internal.NewFileLifecycleHistory(path)
}

func NewSQLLifecycleHistory(ctx context.Context, db *sql.DB) (*SQLLifecycleHistory, error) {
	return internal.NewSQLLifecycleHistory(ctx, db)
}

func NewGORMLifecycleHistory(ctx context.Context, db *gorm.DB) (*GORMLifecycleHistory, error) {
	return internal.NewGORMLifecycleHistory(ctx, db)
}

func NewRecordedLifecycleExecutor(history LifecycleHistory) RecordedLifecycleExecutor {
	return internal.NewRecordedLifecycleExecutor(history)
}

func AdminRetentionDuration(days int) (time.Duration, bool) {
	return internal.AdminRetentionDuration(days)
}

func IsAdminDeletionMode(mode string) bool {
	return internal.IsAdminDeletionMode(mode)
}

func SupportsAdminAutoPurge(resource string, mode string) bool {
	return internal.SupportsAdminAutoPurge(resource, mode)
}

func ValidateAdminSurface(manifests []Manifest) error {
	return internal.ValidateAdminSurface(manifests)
}

func ValidateAppSurface(manifests []Manifest) error {
	return internal.ValidateAppSurface(manifests)
}

func AppRouteContracts(manifests []Manifest) ([]AppRouteContract, error) {
	return internal.AppRouteContracts(manifests)
}

func ValidateAuthProviderDeclarations(manifests []Manifest) error {
	return internal.ValidateAuthProviderDeclarations(manifests)
}

func ValidateDemoDataDeclarations(manifests []Manifest) error {
	return internal.ValidateDemoDataDeclarations(manifests)
}

func ValidateLifecycleDeclarations(manifests []Manifest) error {
	return internal.ValidateLifecycleDeclarations(manifests)
}

func DefaultTrustedTenantContext() TrustedTenantContext {
	return internal.DefaultTrustedTenantContext()
}

func ValidateServiceContracts(manifests []Manifest) error {
	return internal.ValidateServiceContracts(manifests)
}

func ServiceContractDocumentFromManifests(manifests []Manifest) (ServiceContractDocument, error) {
	return internal.ServiceContractDocumentFromManifests(manifests)
}
