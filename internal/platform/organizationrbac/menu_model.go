package organizationrbac

type MenuNodeType string

const (
	MenuNodeTypeDirectory  MenuNodeType = "directory"
	MenuNodeTypePage       MenuNodeType = "page"
	RoleMenuStoredNodeType              = MenuNodeTypePage
)

type MenuParameterType string

const (
	MenuParameterTypeString  MenuParameterType = "string"
	MenuParameterTypeNumber  MenuParameterType = "number"
	MenuParameterTypeBoolean MenuParameterType = "boolean"
	MaximumMenuParameters                      = 32
)

type MenuOpenMode string

const (
	MenuOpenModeSameTab MenuOpenMode = "same-tab"
	MenuOpenModeNewTab  MenuOpenMode = "new-tab"
)

type MenuServingMode string

const (
	MenuServingModeLegacy   MenuServingMode = "legacy"
	MenuServingModeDualRead MenuServingMode = "dual-read"
	MenuServingModeTarget   MenuServingMode = "target"
	DefaultMenuServingMode                  = MenuServingModeLegacy
)

type MenuParameter struct {
	Key   string
	Type  MenuParameterType
	Value any
}

type MenuNode struct {
	Code              string
	ParentCode        string
	NodeType          MenuNodeType
	TitleZH           string
	TitleEN           string
	DescriptionZH     string
	DescriptionEN     string
	Status            string
	Icon              string
	SortOrder         int
	Route             string
	ComponentKey      string
	ResourceCode      string
	External          bool
	ExternalURL       string
	OpenMode          MenuOpenMode
	Parameters        []MenuParameter
	CacheEnabled      bool
	Hidden            bool
	ActiveMenuCode    string
	BreadcrumbVisible bool
	LegacyPermission  string
}

type PageButton struct {
	MenuCode       string
	ButtonKey      string
	LabelZH        string
	LabelEN        string
	Action         string
	SortOrder      int
	Status         string
	PermissionCode string
}

type RoleMenuBinding struct {
	RoleCode string
	MenuCode string
}
