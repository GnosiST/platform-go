package capability

import (
	"context"
	"slices"
)

type ID string

type Manifest struct {
	ID            ID
	Name          string
	Version       string
	Dependencies  []ID
	Admin         AdminSurface
	App           AppSurface
	Service       ServiceSurface
	AuthProviders []AuthProvider
	Migrations    []Migration
	Seeds         []Seed
	DemoData      []DemoDataSet
	Hooks         Hooks
}

type LocalizedText struct {
	ZH string `json:"zh"`
	EN string `json:"en"`
}

type AdminSurface struct {
	Resources      []AdminResource
	RevealPolicies []AdminRevealPolicy
}

type AdminResource struct {
	Resource         string
	Title            LocalizedText
	Description      LocalizedText
	PermissionPrefix string
	ReadOnly         bool
	Deletion         *AdminResourceDeletionPolicy
	Menu             AdminMenu
	FormGroups       []AdminFormGroup
	FormLayout       string
	Fields           []AdminField
	Actions          []AdminResourceAction
	Panels           []AdminResourcePanel
	RuntimeSlots     []AdminRuntimeSlot
	SearchFields     []string
	DefaultSortKey   string
	Protection       *AdminResourceProtection
}

type AdminResourceDeletionPolicy struct {
	Mode               string `json:"mode"`
	PolicyVersion      uint32 `json:"policyVersion"`
	RetentionDays      int    `json:"retentionDays,omitempty"`
	AutoPurge          bool   `json:"autoPurge,omitempty"`
	RestrictReferences bool   `json:"restrictReferences,omitempty"`
}

type AdminMenu struct {
	Route    string
	Parent   string
	Group    string
	Icon     string
	Order    int
	External bool
	Cache    bool
}

type AdminField struct {
	Key          string
	Label        LocalizedText
	Type         string
	Source       string
	Group        string
	Help         LocalizedText
	Required     bool
	ReadOnly     bool
	Searchable   bool
	Filterable   bool
	Sortable     bool
	Localizable  bool
	InTable      bool
	InForm       bool
	InDetail     bool
	Width        int
	Options      []AdminFieldOption
	Relation     *AdminFieldRelation
	Validation   AdminFieldValidation
	Sensitivity  string
	StorageMode  string
	ResponseMode string
	ExportMode   string
	Protection   *AdminFieldProtection
	Masking      *AdminFieldMasking
	Reveal       *AdminFieldReveal
}

type AdminFieldProtection struct {
	Format              string `json:"format"`
	Normalization       string `json:"normalization"`
	BlindIndexNamespace string `json:"blindIndexNamespace,omitempty"`
}

type AdminFieldMasking struct {
	Strategy       string `json:"strategy"`
	PreservePrefix int    `json:"preservePrefix,omitempty"`
	PreserveSuffix int    `json:"preserveSuffix,omitempty"`
	MaskLength     int    `json:"maskLength,omitempty"`
	Replacement    string `json:"replacement,omitempty"`
}

type AdminFieldReveal struct {
	PolicyID    string `json:"policyId"`
	Permission  string `json:"permission"`
	CopyAllowed bool   `json:"copyAllowed,omitempty"`
}

type AdminRevealPolicy struct {
	ID                  string
	Mode                string
	Factors             []string
	Purposes            []AdminRevealPurpose
	ChallengeTTLSeconds int
	GrantTTLSeconds     int
}

type AdminRevealPurpose struct {
	Code  string
	Label LocalizedText
}

type AdminResourceProtection struct {
	SchemaVersion uint32 `json:"schemaVersion"`
	Scope         string `json:"scope"`
	TenantField   string `json:"tenantField,omitempty"`
}

const (
	FieldSensitivityPublic    = "public"
	FieldSensitivityInternal  = "internal"
	FieldSensitivityPersonal  = "personal"
	FieldSensitivitySensitive = "sensitive"
	FieldSensitivitySecret    = "secret"

	FieldStoragePlain     = "plain"
	FieldStorageMasked    = "masked"
	FieldStorageHashed    = "hashed"
	FieldStorageEncrypted = "encrypted"

	FieldProjectionFull       = "full"
	FieldProjectionMasked     = "masked"
	FieldProjectionPrivileged = "privileged"
	FieldProjectionOmitted    = "omitted"

	AdminRevealModeAnyOf = "anyOf"
	AdminRevealModeAllOf = "allOf"

	AdminRevealFactorOIDCReauthentication = "oidc-reauth-v1"
	AdminRevealFactorSMSOTP               = "admin-sms-otp-v1"

	AdminDeletionDisabled   = "disabled"
	AdminDeletionAppendOnly = "append-only"
	AdminDeletionRestrict   = "restrict"
	AdminDeletionSoftDelete = "soft-delete"
	AdminDeletionRevoke     = "revoke"
	AdminDeletionTombstone  = "tombstone"
	AdminDeletionHardDelete = "hard-delete"
)

type AdminFieldRelation struct {
	Resource    string
	ValueField  string
	LabelField  string
	Multiple    bool
	Filters     []AdminFieldRelationFilter
	SortField   string
	SortOrder   string
	Display     string
	ParentField string
	PathField   string
	RootValue   string
}

type AdminFieldRelationFilter struct {
	Field    string
	Operator string
	Value    string
}

type AdminFormGroup struct {
	Key         string
	Label       LocalizedText
	Description LocalizedText
}

type AdminResourceAction struct {
	Key         string
	Label       LocalizedText
	Kind        string
	Tone        string
	Icon        string
	Permission  string
	Route       string
	Method      string
	Confirm     *AdminActionConfirm
	AuditAction string
	Refresh     bool
}

type AdminActionConfirm struct {
	Title       LocalizedText
	Description LocalizedText
	OkText      LocalizedText
}

type AdminResourcePanel struct {
	Key        string
	Label      LocalizedText
	Kind       string
	Permission string
	Component  string
	Order      int
	Empty      LocalizedText
}

type AdminRuntimeSlot struct {
	SlotID        string
	Region        string
	Label         LocalizedText
	Description   LocalizedText
	Permission    string
	VisibleWhen   string
	TargetSection string
	TargetField   string
	DataBinding   AdminRuntimeSlotDataBinding
	Variant       string
	Order         int
}

type AdminRuntimeSlotDataBinding struct {
	Mode   string
	Fields []string
}

type AdminFieldValidation struct {
	MinLength int
	MaxLength int
	Min       *float64
	Max       *float64
	Pattern   string
}

type AdminFieldOption struct {
	Value string
	Label LocalizedText
}

type AppSurface struct {
	Routes []AppRoute
}

type AppRouteAuth string

const (
	AppRouteAuthPublic  AppRouteAuth = "public"
	AppRouteAuthSession AppRouteAuth = "session"
)

type AppRoute struct {
	Method      string
	Path        string
	Auth        AppRouteAuth
	Permission  string
	Description LocalizedText
}

type AuthProviderAudience string

const (
	AuthProviderAudienceAdmin AuthProviderAudience = "admin"
	AuthProviderAudienceApp   AuthProviderAudience = "app"
)

type AuthProvider struct {
	ID          string                 `json:"id"`
	Kind        string                 `json:"kind"`
	Title       LocalizedText          `json:"title"`
	Description LocalizedText          `json:"description"`
	Enabled     bool                   `json:"enabled"`
	Configured  bool                   `json:"configured"`
	ConfigKeys  []string               `json:"configKeys,omitempty"`
	Audiences   []AuthProviderAudience `json:"audiences"`
}

func (p AuthProvider) SupportsAudience(audience AuthProviderAudience) bool {
	return slices.Contains(p.Audiences, audience)
}

type Migration struct {
	ID          string
	Description string
	Up          LifecycleStep
}

type Seed struct {
	ID          string
	Description string
	Run         LifecycleStep
}

type DemoDataSet struct {
	ID          string
	Title       LocalizedText
	Description LocalizedText
	Resource    string
	Records     []DemoRecord
}

type DemoRecord struct {
	ID          string
	Code        string
	Name        string
	Status      string
	Description string
	Values      map[string]string
}

type Hooks struct {
	Configure        Hook
	Migrate          Hook
	Seed             Hook
	RegisterServices Hook
	RegisterRoutes   Hook
	RegisterAdmin    Hook
	Start            Hook
}

type Hook func(context.Context, Runtime) error
type LifecycleStep func(context.Context, Runtime) error

func Text(zh string, en string) LocalizedText {
	return LocalizedText{ZH: zh, EN: en}
}
