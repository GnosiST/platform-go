package adminresource

import (
	"sort"
	"strings"

	"platform-go/internal/platform/capability"
)

var permissionActions = []struct {
	Key     string
	LabelZH string
	LabelEN string
}{
	{Key: "read", LabelZH: "读取", LabelEN: "Read"},
	{Key: "create", LabelZH: "创建", LabelEN: "Create"},
	{Key: "update", LabelZH: "更新", LabelEN: "Update"},
	{Key: "delete", LabelZH: "删除", LabelEN: "Delete"},
}

func permissionCatalogFromCapabilities(manifests []capability.Manifest, updatedAt string) []Record {
	records := make([]Record, 0)
	seen := map[string]struct{}{}
	for _, manifest := range manifests {
		for _, resource := range manifest.Admin.Resources {
			if strings.TrimSpace(resource.Resource) == "" || strings.TrimSpace(resource.PermissionPrefix) == "" {
				continue
			}
			title := localizedTextFromCapability(resource.Title)
			for _, action := range permissionActions {
				code := resource.PermissionPrefix + ":" + action.Key
				if _, exists := seen[code]; exists {
					continue
				}
				seen[code] = struct{}{}
				records = append(records, seed(
					"permission-"+strings.NewReplacer(":", "-", "*", "all").Replace(code),
					code,
					title.EN+" "+action.LabelEN,
					"enabled",
					title.EN+" "+action.LabelEN+" permission.",
					updatedAt,
					withLocalizedValues(map[string]string{
						"capability": string(manifest.ID),
						"resource":   resource.Resource,
						"action":     action.Key,
						"prefix":     resource.PermissionPrefix,
					}, title.ZH+action.LabelZH, title.EN+" "+action.LabelEN, title.ZH+action.LabelZH+"权限。", title.EN+" "+action.LabelEN+" permission."),
				))
			}
			for _, action := range resource.Actions {
				code := strings.TrimSpace(action.Permission)
				if code == "" {
					continue
				}
				if _, exists := seen[code]; exists {
					continue
				}
				seen[code] = struct{}{}
				actionKey := strings.TrimSpace(action.Key)
				label := localizedTextFromCapability(action.Label)
				records = append(records, seed(
					"permission-"+strings.NewReplacer(":", "-", "*", "all").Replace(code),
					code,
					title.EN+" "+label.EN,
					"enabled",
					title.EN+" "+label.EN+" permission.",
					updatedAt,
					withLocalizedValues(map[string]string{
						"capability": string(manifest.ID),
						"resource":   resource.Resource,
						"action":     actionKey,
						"prefix":     resource.PermissionPrefix,
					}, title.ZH+label.ZH, title.EN+" "+label.EN, title.ZH+label.ZH+"权限。", title.EN+" "+label.EN+" permission."),
				))
			}
		}
	}
	sort.SliceStable(records, func(i int, j int) bool {
		return records[i].Code < records[j].Code
	})
	return records
}

func permissionOptionsFromCapabilities(manifests []capability.Manifest) []FieldOption {
	options := []FieldOption{
		option("*", "全部权限", "All Permissions"),
		option("admin:*", "后台全部权限", "All Admin Permissions"),
	}
	records := permissionCatalogFromCapabilities(manifests, "")
	for _, record := range records {
		options = append(options, option(record.Code, record.Code, record.Code))
	}
	return options
}
