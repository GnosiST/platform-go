package adminresource

import (
	"sort"
	"strconv"
	"strings"

	"github.com/GnosiST/platform-go/internal/platform/capability"
	"github.com/GnosiST/platform-go/internal/platform/rbac"
)

type MenuItem struct {
	Name              string        `json:"name"`
	NodeType          string        `json:"nodeType"`
	Route             string        `json:"route"`
	Parent            string        `json:"parent"`
	ParentCode        string        `json:"parentCode"`
	ComponentKey      string        `json:"componentKey"`
	ResourceCode      string        `json:"resourceCode"`
	IsExternal        bool          `json:"isExternal"`
	ExternalURL       string        `json:"externalUrl"`
	OpenMode          string        `json:"openMode"`
	Parameters        string        `json:"parameters"`
	CacheEnabled      bool          `json:"cacheEnabled"`
	Hidden            bool          `json:"hidden"`
	ActiveMenuCode    string        `json:"activeMenuCode"`
	BreadcrumbVisible bool          `json:"breadcrumbVisible"`
	PageButtons       string        `json:"pageButtons"`
	Resource          string        `json:"resource"`
	Title             LocalizedText `json:"title"`
	Description       LocalizedText `json:"description"`
	Permission        string        `json:"permission"`
	Group             string        `json:"group"`
	Icon              string        `json:"icon"`
	Order             int           `json:"order"`
}

func (s *Store) MenuItemsForPrincipal(principal rbac.Principal) []MenuItem {
	s.mu.Lock()
	defer s.mu.Unlock()
	policy := rbac.NewPolicySetWithDeny(principal.Permissions, principal.DeniedPermissions)
	items := make([]MenuItem, 0)
	for _, record := range visibleRecords("menus", s.resources["menus"]) {
		if record.Status == "disabled" {
			continue
		}
		item := menuItemFromRecord(record)
		if item.Route == "" || item.Permission == "" || !s.permissionCodeEnabledLocked(item.Permission) || !policy.Allows(item.Permission) {
			continue
		}
		items = append(items, item)
	}
	sort.SliceStable(items, func(i int, j int) bool {
		if items[i].Order == items[j].Order {
			return items[i].Name < items[j].Name
		}
		return items[i].Order < items[j].Order
	})
	return items
}

func menuItemFromRecord(record Record) MenuItem {
	return MenuItem{
		Name:              record.Code,
		NodeType:          valueWithFallback(record.Values["nodeType"], "page"),
		Route:             record.Values["route"],
		Parent:            record.Values["parent"],
		ParentCode:        valueWithFallback(record.Values["parentCode"], record.Values["parent"]),
		ComponentKey:      valueWithFallback(record.Values["componentKey"], record.Values["resource"]),
		ResourceCode:      valueWithFallback(record.Values["resourceCode"], record.Values["resource"]),
		IsExternal:        parseBool(record.Values["isExternal"]),
		ExternalURL:       record.Values["externalUrl"],
		OpenMode:          record.Values["openMode"],
		Parameters:        record.Values["parameters"],
		CacheEnabled:      parseBoolDefault(record.Values["cacheEnabled"], true),
		Hidden:            parseBool(record.Values["hidden"]),
		ActiveMenuCode:    record.Values["activeMenuCode"],
		BreadcrumbVisible: parseBoolDefault(record.Values["breadcrumbVisible"], true),
		PageButtons:       record.Values["pageButtons"],
		Resource:          record.Values["resource"],
		Title:             text(record.Values["titleZh"], record.Values["titleEn"]),
		Description:       text(record.Values["descriptionZh"], record.Values["descriptionEn"]),
		Permission:        record.Values["permission"],
		Group:             record.Values["group"],
		Icon:              record.Values["icon"],
		Order:             parseOrder(record.Values["order"]),
	}
}

func menuRecordFromCapability(resource capability.AdminResource, updatedAt string) (Record, bool) {
	if resource.Menu.Route == "" {
		return Record{}, false
	}
	title := localizedTextFromCapability(resource.Title)
	description := localizedTextFromCapability(resource.Description)
	return seedMenu(
		"menu-"+resource.Resource,
		resource.Resource,
		title.EN,
		description.EN,
		updatedAt,
		resource.Menu.Route,
		resource.Resource,
		resource.PermissionPrefix+":read",
		resource.Menu.Group,
		resource.Menu.Icon,
		strconv.Itoa(resource.Menu.Order),
		title.ZH,
		title.EN,
		description.ZH,
		description.EN,
		resource.Menu.Parent,
		boolString(resource.Menu.External),
		boolString(resource.Menu.Cache || !resource.Menu.External),
	), true
}

func parseBool(value string) bool {
	return strings.EqualFold(strings.TrimSpace(value), "true") || strings.TrimSpace(value) == "1"
}

func parseBoolDefault(value string, fallback bool) bool {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return parseBool(value)
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func parseOrder(value string) int {
	order, err := strconv.Atoi(value)
	if err != nil {
		return 9999
	}
	return order
}
