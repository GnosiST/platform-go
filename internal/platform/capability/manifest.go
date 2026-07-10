package capability

import "context"

type ID string

type Manifest struct {
	ID            ID
	Name          string
	Version       string
	Dependencies  []ID
	Admin         AdminSurface
	App           AppSurface
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
	Resources []AdminResource
}

type AdminResource struct {
	Resource         string
	Title            LocalizedText
	Description      LocalizedText
	PermissionPrefix string
	Menu             AdminMenu
	FormGroups       []AdminFormGroup
	FormLayout       string
	Fields           []AdminField
	Actions          []AdminResourceAction
	Panels           []AdminResourcePanel
	RuntimeSlots     []AdminRuntimeSlot
	SearchFields     []string
	DefaultSortKey   string
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
	Key         string
	Label       LocalizedText
	Type        string
	Source      string
	Group       string
	Help        LocalizedText
	Required    bool
	ReadOnly    bool
	Searchable  bool
	Filterable  bool
	Sortable    bool
	Localizable bool
	InTable     bool
	InForm      bool
	InDetail    bool
	Width       int
	Options     []AdminFieldOption
	Relation    *AdminFieldRelation
	Validation  AdminFieldValidation
}

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

type AuthProvider struct {
	ID          string        `json:"id"`
	Kind        string        `json:"kind"`
	Title       LocalizedText `json:"title"`
	Description LocalizedText `json:"description"`
	Enabled     bool          `json:"enabled"`
	Configured  bool          `json:"configured"`
	ConfigKeys  []string      `json:"configKeys,omitempty"`
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
