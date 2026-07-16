package adminresource

import (
	"slices"
	"sort"
	"strings"

	"github.com/GnosiST/platform-go/internal/platform/capability"
	"github.com/GnosiST/platform-go/internal/platform/rbac"
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

var lifecyclePermissionActions = []struct {
	Key     string
	LabelZH string
	LabelEN string
}{
	{Key: "restore", LabelZH: "恢复", LabelEN: "Restore"},
}

func permissionCatalogFromCapabilities(manifests []capability.Manifest, updatedAt string) []Record {
	records := []Record{
		seedLocalized("permission-all", "*", "全部权限", "All Permissions", "enabled", "全部平台权限。", "All platform permissions.", updatedAt, map[string]string{"capability": "platform", "resource": "*", "action": "*", "prefix": "*", "resourceType": "api"}),
		seedLocalized("permission-admin-all", "admin:*", "全部后台权限", "All Admin Permissions", "enabled", "全部后台管理权限。", "All admin permissions.", updatedAt, map[string]string{"capability": "platform", "resource": "admin", "action": "*", "prefix": "admin", "resourceType": "api"}),
	}
	seen := map[string]struct{}{}
	for _, record := range records {
		seen[record.Code] = struct{}{}
	}
	for _, manifest := range manifests {
		for _, resource := range manifest.Admin.Resources {
			if strings.TrimSpace(resource.Resource) == "" || strings.TrimSpace(resource.PermissionPrefix) == "" {
				continue
			}
			title := localizedTextFromCapability(resource.Title)
			for _, action := range permissionActionsForResource(resource) {
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
						"capability":   string(manifest.ID),
						"resource":     resource.Resource,
						"action":       action.Key,
						"prefix":       resource.PermissionPrefix,
						"resourceType": "api",
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
						"capability":   string(manifest.ID),
						"resource":     resource.Resource,
						"action":       actionKey,
						"prefix":       resource.PermissionPrefix,
						"resourceType": "api",
					}, title.ZH+label.ZH, title.EN+" "+label.EN, title.ZH+label.ZH+"权限。", title.EN+" "+label.EN+" permission."),
				))
			}
			for _, field := range resource.Fields {
				if field.Reveal == nil {
					continue
				}
				code := strings.TrimSpace(field.Reveal.Permission)
				if code == "" {
					continue
				}
				if _, exists := seen[code]; exists {
					continue
				}
				seen[code] = struct{}{}
				label := localizedTextFromCapability(field.Label)
				records = append(records, seed(
					"permission-"+strings.NewReplacer(":", "-", "*", "all").Replace(code),
					code,
					title.EN+" "+label.EN+" Reveal",
					"enabled",
					title.EN+" "+label.EN+" sensitive reveal permission.",
					updatedAt,
					withLocalizedValues(map[string]string{
						"capability":   string(manifest.ID),
						"resource":     resource.Resource,
						"action":       "reveal",
						"prefix":       resource.PermissionPrefix,
						"resourceType": "api",
					}, title.ZH+label.ZH+"查看明文", title.EN+" "+label.EN+" Reveal", title.ZH+label.ZH+"敏感明文查看权限。", title.EN+" "+label.EN+" sensitive reveal permission."),
				))
			}
		}
	}
	sort.SliceStable(records, func(i int, j int) bool {
		return records[i].Code < records[j].Code
	})
	return records
}

func permissionCatalogFromSchemas(schemas map[string]Schema, updatedAt string) []Record {
	records := permissionCatalogFromCapabilities(nil, updatedAt)
	seen := make(map[string]struct{}, len(records))
	for _, record := range records {
		seen[record.Code] = struct{}{}
	}
	for resource, schema := range schemas {
		codes := []string{
			schema.Permissions.Read,
			schema.Permissions.Create,
			schema.Permissions.Update,
			schema.Permissions.Delete,
			schema.Permissions.Restore,
			schema.Permissions.Purge,
		}
		for _, action := range schema.Actions {
			codes = append(codes, action.Permission)
		}
		for _, panel := range schema.Panels {
			codes = append(codes, panel.Permission)
		}
		for _, slot := range schema.RuntimeSlots {
			codes = append(codes, slot.Permission)
		}
		for _, field := range schema.Fields {
			if field.Reveal != nil {
				codes = append(codes, field.Reveal.Permission)
			}
		}
		for _, code := range codes {
			code = strings.TrimSpace(code)
			if code == "" {
				continue
			}
			if _, exists := seen[code]; exists {
				continue
			}
			seen[code] = struct{}{}
			action := actionFromPermission(code)
			prefix := strings.TrimSuffix(code, ":"+action)
			records = append(records, seedLocalized(
				"permission-"+strings.NewReplacer(":", "-", "*", "all").Replace(code),
				code, code, code, "enabled", code+" permission.", code+" permission.", updatedAt,
				map[string]string{"capability": "platform", "resource": resource, "action": action, "prefix": prefix, "resourceType": "api"},
			))
		}
	}
	sort.SliceStable(records, func(i int, j int) bool { return records[i].Code < records[j].Code })
	return records
}

func appendPermissionCatalogCodes(records []Record, codes []string, updatedAt string) []Record {
	seen := make(map[string]struct{}, len(records))
	for _, record := range records {
		seen[record.Code] = struct{}{}
	}
	for _, code := range codes {
		code = strings.TrimSpace(code)
		if code == "" {
			continue
		}
		if _, exists := seen[code]; exists {
			continue
		}
		seen[code] = struct{}{}
		action := actionFromPermission(code)
		prefix := strings.TrimSuffix(code, ":"+action)
		resource := strings.TrimPrefix(prefix, "admin:")
		records = append(records, seedLocalized(
			"permission-"+strings.NewReplacer(":", "-", "*", "all").Replace(code),
			code, code, code, "enabled", code+" permission.", code+" permission.", updatedAt,
			map[string]string{"capability": "platform", "resource": resource, "action": action, "prefix": prefix, "resourceType": "api"},
		))
	}
	sort.SliceStable(records, func(i int, j int) bool { return records[i].Code < records[j].Code })
	return records
}

func permissionActionsForResource(resource capability.AdminResource) []struct {
	Key     string
	LabelZH string
	LabelEN string
} {
	actions := append([]struct {
		Key     string
		LabelZH string
		LabelEN string
	}(nil), permissionActions...)
	if resource.Deletion == nil {
		return actions[:3]
	}
	switch resource.Deletion.Mode {
	case capability.AdminDeletionDisabled, capability.AdminDeletionAppendOnly:
		actions = actions[:3]
	case capability.AdminDeletionSoftDelete, capability.AdminDeletionTombstone:
		actions = append(actions, lifecyclePermissionActions...)
	}
	return actions
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

func (s *Store) expandPermissionPoliciesLocked(policies []string) []string {
	enabledCodes := s.enabledExactPermissionCodesLocked()
	expanded := make([]string, 0, len(policies))
	for _, policy := range policies {
		if strings.Contains(policy, "*") && !s.wildcardPermissionPolicyActiveLocked(policy) {
			continue
		}
		matcher := rbac.NewPolicySet([]string{policy})
		for _, code := range enabledCodes {
			if matcher.Allows(code) && !slices.Contains(expanded, code) {
				expanded = append(expanded, code)
			}
		}
	}
	sort.Strings(expanded)
	return expanded
}

func (s *Store) activePermissionPoliciesLocked(policies []string) []string {
	enabledCodes := s.enabledExactPermissionCodesLocked()
	active := make([]string, 0, len(policies))
	for _, policy := range policies {
		policy = strings.TrimSpace(policy)
		if policy == "" {
			continue
		}
		if strings.Contains(policy, "*") {
			if !s.wildcardPermissionPolicyActiveLocked(policy) {
				continue
			}
			matcher := rbac.NewPolicySet([]string{policy})
			if !slices.Contains(active, policy) && slices.ContainsFunc(enabledCodes, matcher.Allows) {
				active = append(active, policy)
			}
			continue
		}
		if slices.Contains(enabledCodes, policy) && !slices.Contains(active, policy) {
			active = append(active, policy)
		}
	}
	sort.Strings(active)
	return active
}

func (s *Store) wildcardPermissionPolicyActiveLocked(policy string) bool {
	record := findRecordByCode(s.resources["permissions"], strings.TrimSpace(policy))
	return record == nil || record.Status == "enabled" && record.DeletedAt == ""
}

func (s *Store) inactivePermissionCodesLocked() []string {
	codes := make([]string, 0)
	for _, permission := range s.resources["permissions"] {
		code := strings.TrimSpace(permission.Code)
		if code == "" || strings.Contains(code, "*") || permission.Status == "enabled" && permission.DeletedAt == "" {
			continue
		}
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes
}

func (s *Store) enabledExactPermissionCodesLocked() []string {
	codes := make([]string, 0)
	for _, permission := range visibleRecords("permissions", s.resources["permissions"]) {
		code := strings.TrimSpace(permission.Code)
		if permission.Status != "enabled" || code == "" || strings.Contains(code, "*") {
			continue
		}
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes
}

func (s *Store) permissionCodeEnabledLocked(code string) bool {
	code = strings.TrimSpace(code)
	if code == "" || strings.Contains(code, "*") {
		return false
	}
	permission := findRecordByCode(visibleRecords("permissions", s.resources["permissions"]), code)
	return permission != nil && permission.Status == "enabled"
}
