package capability

import (
	"fmt"
	"slices"
	"strings"

	"platform-go/internal/platform/masking"
)

var adminRelationRecordFields = []string{"id", "code", "name", "status", "description", "updatedAt"}
var adminFormLayouts = []string{"single-column", "grouped-sections", "two-column-density", "side-detail-preview"}
var adminRelationFilterOperators = []string{"contains", "=", "!=", ">", ">=", "<", "<="}
var adminRuntimeSlotRegions = []string{"form.header", "form.section.before", "form.section.after", "form.footer", "field.control", "side.preview"}
var adminRuntimeSlotDataBindingModes = []string{"record", "formValues", "resource", "none"}
var adminRuntimeSlotVariants = []string{"compact", "info", "warning", "preview", "inline"}
var adminFieldSensitivities = []string{FieldSensitivityPublic, FieldSensitivityInternal, FieldSensitivityPersonal, FieldSensitivitySensitive, FieldSensitivitySecret}
var adminFieldStorageModes = []string{FieldStoragePlain, FieldStorageMasked, FieldStorageHashed, FieldStorageEncrypted}
var adminFieldProjectionModes = []string{FieldProjectionFull, FieldProjectionMasked, FieldProjectionPrivileged, FieldProjectionOmitted}
var adminFieldProtectionFormats = []string{"aes-256-gcm-v1"}
var adminFieldProtectionNormalizations = []string{"raw-v1", "trim-v1", "email-v1", "phone-e164-cn-v1", "identity-cn-v1"}
var adminResourceProtectionScopes = []string{"global", "tenant-field"}
var adminRevealModes = []string{AdminRevealModeAnyOf, AdminRevealModeAllOf}
var adminRevealFactors = []string{AdminRevealFactorOIDCReauthentication, AdminRevealFactorSMSOTP}

func ValidateAdminSurface(manifests []Manifest) error {
	resources := map[string]ID{}
	routes := map[string]ID{}
	permissionPrefixes := map[string]ID{}
	revealPolicies := map[string]AdminRevealPolicy{}
	for _, manifest := range manifests {
		for _, policy := range manifest.Admin.RevealPolicies {
			if err := validateAdminRevealPolicy(manifest.ID, policy); err != nil {
				return err
			}
			if _, exists := revealPolicies[policy.ID]; exists {
				return fmt.Errorf("capability %q admin reveal policy %q is already registered", manifest.ID, policy.ID)
			}
			revealPolicies[policy.ID] = policy
		}
	}
	for _, manifest := range manifests {
		for _, resource := range manifest.Admin.Resources {
			if err := validateAdminResource(manifest.ID, resource, revealPolicies); err != nil {
				return err
			}
			if owner, exists := resources[resource.Resource]; exists {
				return fmt.Errorf("capability %q admin resource %q already registered by capability %q", manifest.ID, resource.Resource, owner)
			}
			resources[resource.Resource] = manifest.ID
			if resource.Menu.Route != "" {
				if owner, exists := routes[resource.Menu.Route]; exists {
					return fmt.Errorf("capability %q admin menu route %q already registered by capability %q", manifest.ID, resource.Menu.Route, owner)
				}
				routes[resource.Menu.Route] = manifest.ID
			}
			if owner, exists := permissionPrefixes[resource.PermissionPrefix]; exists {
				return fmt.Errorf("capability %q admin permission prefix %q already registered by capability %q", manifest.ID, resource.PermissionPrefix, owner)
			}
			permissionPrefixes[resource.PermissionPrefix] = manifest.ID
		}
	}
	if err := validateAdminRelationTargets(manifests, resources); err != nil {
		return err
	}
	return nil
}

func validateAdminRelationTargets(manifests []Manifest, resources map[string]ID) error {
	fieldsByResource := adminFieldsByResource(manifests)
	for _, manifest := range manifests {
		for _, resource := range manifest.Admin.Resources {
			for _, field := range resource.Fields {
				if field.Relation == nil {
					continue
				}
				target := strings.TrimSpace(field.Relation.Resource)
				if _, exists := resources[target]; exists {
					if err := validateAdminRelationTargetFields(manifest.ID, resource.Resource, field.Key, target, *field.Relation, fieldsByResource[target]); err != nil {
						return err
					}
					continue
				}
				return fmt.Errorf("capability %q admin resource %q field %q relation target resource %q is not enabled", manifest.ID, resource.Resource, field.Key, target)
			}
		}
	}
	return nil
}

func adminFieldsByResource(manifests []Manifest) map[string]map[string]struct{} {
	fieldsByResource := map[string]map[string]struct{}{}
	for _, manifest := range manifests {
		for _, resource := range manifest.Admin.Resources {
			fields := map[string]struct{}{}
			for _, key := range adminRelationRecordFields {
				fields[key] = struct{}{}
			}
			for _, field := range resource.Fields {
				key := strings.TrimSpace(field.Key)
				if key == "" {
					continue
				}
				fields[key] = struct{}{}
			}
			fieldsByResource[resource.Resource] = fields
		}
	}
	return fieldsByResource
}

func validateAdminRelationTargetFields(owner ID, resource string, field string, target string, relation AdminFieldRelation, targetFields map[string]struct{}) error {
	for _, item := range []struct {
		label string
		key   string
	}{
		{label: "value", key: relation.ValueField},
		{label: "label", key: relation.LabelField},
		{label: "sort", key: relation.SortField},
		{label: "parent", key: relation.ParentField},
		{label: "path", key: relation.PathField},
	} {
		if err := validateAdminRelationTargetField(owner, resource, field, target, item.label, item.key, targetFields); err != nil {
			return err
		}
	}
	for _, filter := range relation.Filters {
		if err := validateAdminRelationTargetField(owner, resource, field, target, "filter", filter.Field, targetFields); err != nil {
			return err
		}
	}
	return nil
}

func validateAdminRelationTargetField(owner ID, resource string, field string, target string, label string, key string, targetFields map[string]struct{}) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}
	if _, exists := targetFields[key]; exists {
		return nil
	}
	validFields := make([]string, 0, len(targetFields))
	for validField := range targetFields {
		validFields = append(validFields, validField)
	}
	slices.Sort(validFields)
	return fmt.Errorf("capability %q admin resource %q field %q relation %s field %q is not declared by target resource %q; declared fields: %s", owner, resource, field, label, key, target, strings.Join(validFields, ","))
}

func validateAdminResource(owner ID, resource AdminResource, revealPolicies map[string]AdminRevealPolicy) error {
	switch {
	case strings.TrimSpace(resource.Resource) == "":
		return fmt.Errorf("capability %q admin resource key is required", owner)
	case strings.TrimSpace(resource.Title.ZH) == "" || strings.TrimSpace(resource.Title.EN) == "":
		return fmt.Errorf("capability %q admin resource %q title is required", owner, resource.Resource)
	case strings.TrimSpace(resource.Description.ZH) == "" || strings.TrimSpace(resource.Description.EN) == "":
		return fmt.Errorf("capability %q admin resource %q description is required", owner, resource.Resource)
	case strings.TrimSpace(resource.PermissionPrefix) == "":
		return fmt.Errorf("capability %q admin resource %q permission prefix is required", owner, resource.Resource)
	}
	if !validAdminPermissionPrefix(resource.PermissionPrefix) {
		return fmt.Errorf("capability %q admin resource %q permission prefix must match admin:<resource>", owner, resource.Resource)
	}
	if layout := strings.TrimSpace(resource.FormLayout); layout != "" && !slices.Contains(adminFormLayouts, layout) {
		return fmt.Errorf("capability %q admin resource %q form layout must be one of %s", owner, resource.Resource, strings.Join(adminFormLayouts, ","))
	}
	if err := validateAdminMenu(owner, resource); err != nil {
		return err
	}
	if err := validateAdminFormGroups(owner, resource); err != nil {
		return err
	}
	if err := validateAdminResourceFields(owner, resource, revealPolicies); err != nil {
		return err
	}
	if err := validateAdminResourceFieldReferences(owner, resource); err != nil {
		return err
	}
	if err := validateAdminResourceActions(owner, resource); err != nil {
		return err
	}
	if err := validateAdminResourcePanels(owner, resource); err != nil {
		return err
	}
	if err := validateAdminRuntimeSlots(owner, resource); err != nil {
		return err
	}
	return nil
}

func validateAdminFormGroups(owner ID, resource AdminResource) error {
	seen := map[string]struct{}{}
	for _, group := range resource.FormGroups {
		key := strings.TrimSpace(group.Key)
		switch {
		case key == "":
			return fmt.Errorf("capability %q admin resource %q form group key is required", owner, resource.Resource)
		case !hasLocalizedText(group.Label):
			return fmt.Errorf("capability %q admin resource %q form group %s label is required", owner, resource.Resource, key)
		case !hasOptionalLocalizedText(group.Description):
			return fmt.Errorf("capability %q admin resource %q form group %s description must declare zh/en text", owner, resource.Resource, key)
		}
		if _, exists := seen[key]; exists {
			return fmt.Errorf("capability %q admin resource %q duplicate form group key %q", owner, resource.Resource, key)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func validateAdminResourceFields(owner ID, resource AdminResource, revealPolicies map[string]AdminRevealPolicy) error {
	seen := map[string]struct{}{}
	blindIndexNamespaces := map[string]string{}
	hasEncryptedField := false
	groupKeys := declaredAdminFormGroupKeys(resource)
	for _, field := range resource.Fields {
		key := strings.TrimSpace(field.Key)
		if key != "" {
			if _, exists := seen[key]; exists {
				return fmt.Errorf("capability %q admin resource %q duplicate field key %q", owner, resource.Resource, key)
			}
			seen[key] = struct{}{}
		}
		if len(groupKeys) > 0 {
			group := strings.TrimSpace(field.Group)
			if group != "" {
				if _, ok := groupKeys[group]; !ok {
					return fmt.Errorf("capability %q admin resource %q field %s references unknown form group %s", owner, resource.Resource, field.Key, group)
				}
			}
		}
		if err := validateAdminField(owner, resource.Resource, field); err != nil {
			return err
		}
		if err := validateAdminFieldReveal(owner, resource, field, revealPolicies); err != nil {
			return err
		}
		if defaultAdminFieldPolicy(field.StorageMode, FieldStoragePlain) == FieldStorageEncrypted {
			hasEncryptedField = true
			if field.Searchable {
				return fmt.Errorf("capability %q admin resource %q field %q encrypted fields cannot use keyword search", owner, resource.Resource, field.Key)
			}
			if field.Sortable {
				return fmt.Errorf("capability %q admin resource %q field %q encrypted fields cannot be sorted", owner, resource.Resource, field.Key)
			}
			namespace := strings.TrimSpace(field.Protection.BlindIndexNamespace)
			if field.Filterable && namespace == "" {
				return fmt.Errorf("capability %q admin resource %q field %q encrypted filtering requires a blindIndexNamespace", owner, resource.Resource, field.Key)
			}
			if namespace != "" {
				if previous, exists := blindIndexNamespaces[namespace]; exists {
					return fmt.Errorf("capability %q admin resource %q duplicate blindIndexNamespace %q on fields %q and %q", owner, resource.Resource, namespace, previous, field.Key)
				}
				blindIndexNamespaces[namespace] = field.Key
			}
		}
	}
	return validateAdminResourceProtection(owner, resource, hasEncryptedField)
}

func validateAdminRevealPolicy(owner ID, policy AdminRevealPolicy) error {
	policyID := strings.TrimSpace(policy.ID)
	if !validAdminProtectionName(policyID) {
		return fmt.Errorf("capability %q admin reveal policy id must be canonical lowercase kebab-case", owner)
	}
	if !slices.Contains(adminRevealModes, strings.TrimSpace(policy.Mode)) {
		return fmt.Errorf("capability %q admin reveal policy %q mode must be one of %s", owner, policyID, strings.Join(adminRevealModes, ","))
	}
	if len(policy.Factors) == 0 {
		return fmt.Errorf("capability %q admin reveal policy %q factors are required", owner, policyID)
	}
	seenFactors := map[string]struct{}{}
	for _, factor := range policy.Factors {
		factor = strings.TrimSpace(factor)
		if !slices.Contains(adminRevealFactors, factor) {
			return fmt.Errorf("capability %q admin reveal policy %q factor %q is unsupported", owner, policyID, factor)
		}
		if _, exists := seenFactors[factor]; exists {
			return fmt.Errorf("capability %q admin reveal policy %q duplicate factor %q", owner, policyID, factor)
		}
		seenFactors[factor] = struct{}{}
	}
	if len(policy.Purposes) == 0 {
		return fmt.Errorf("capability %q admin reveal policy %q purposes are required", owner, policyID)
	}
	seenPurposes := map[string]struct{}{}
	for _, purpose := range policy.Purposes {
		code := strings.TrimSpace(purpose.Code)
		if !validAdminProtectionName(code) || !hasLocalizedText(purpose.Label) {
			return fmt.Errorf("capability %q admin reveal policy %q purpose must declare a canonical code and zh/en label", owner, policyID)
		}
		if _, exists := seenPurposes[code]; exists {
			return fmt.Errorf("capability %q admin reveal policy %q duplicate purpose %q", owner, policyID, code)
		}
		seenPurposes[code] = struct{}{}
	}
	if policy.ChallengeTTLSeconds < 60 || policy.ChallengeTTLSeconds > 600 {
		return fmt.Errorf("capability %q admin reveal policy %q challenge TTL must be between 60 and 600 seconds", owner, policyID)
	}
	if policy.GrantTTLSeconds < 30 || policy.GrantTTLSeconds > 300 || policy.GrantTTLSeconds >= policy.ChallengeTTLSeconds {
		return fmt.Errorf("capability %q admin reveal policy %q grant TTL must be between 30 and 300 seconds and shorter than the challenge TTL", owner, policyID)
	}
	return nil
}

func validateAdminFieldReveal(owner ID, resource AdminResource, field AdminField, revealPolicies map[string]AdminRevealPolicy) error {
	if field.Reveal == nil {
		return nil
	}
	storageMode := defaultAdminFieldPolicy(field.StorageMode, FieldStoragePlain)
	responseMode := defaultAdminFieldPolicy(field.ResponseMode, FieldProjectionFull)
	exportMode := defaultAdminFieldPolicy(field.ExportMode, FieldProjectionFull)
	if storageMode != FieldStorageEncrypted {
		return fmt.Errorf("capability %q admin resource %q field %q reveal metadata requires encrypted storage", owner, resource.Resource, field.Key)
	}
	if responseMode != FieldProjectionMasked && responseMode != FieldProjectionPrivileged {
		return fmt.Errorf("capability %q admin resource %q field %q reveal metadata requires a masked or privileged response", owner, resource.Resource, field.Key)
	}
	if exportMode == FieldProjectionFull || exportMode == FieldProjectionPrivileged {
		return fmt.Errorf("capability %q admin resource %q field %q reveal metadata cannot expose plaintext exports", owner, resource.Resource, field.Key)
	}
	policyID := strings.TrimSpace(field.Reveal.PolicyID)
	if _, exists := revealPolicies[policyID]; !exists {
		return fmt.Errorf("capability %q admin resource %q field %q references unknown reveal policy %q", owner, resource.Resource, field.Key, policyID)
	}
	permission := strings.TrimSpace(field.Reveal.Permission)
	if permission == "" {
		return fmt.Errorf("capability %q admin resource %q field %q reveal permission is required", owner, resource.Resource, field.Key)
	}
	if err := validateAdminPermission(owner, resource.Resource, fmt.Sprintf("field %s reveal", field.Key), resource.PermissionPrefix, permission); err != nil {
		return err
	}
	return nil
}

func validateAdminResourceFieldReferences(owner ID, resource AdminResource) error {
	fieldKeys := adminResourceFieldKeys(resource)
	fields := make(map[string]AdminField, len(resource.Fields))
	for _, field := range resource.Fields {
		fields[strings.TrimSpace(field.Key)] = field
	}
	for _, field := range resource.SearchFields {
		field = strings.TrimSpace(field)
		if field == "" {
			return fmt.Errorf("capability %q admin resource %q search field is required", owner, resource.Resource)
		}
		if _, ok := fieldKeys[field]; !ok {
			return fmt.Errorf("capability %q admin resource %q search field %s is not declared", owner, resource.Resource, field)
		}
		if candidate, ok := fields[field]; ok && defaultAdminFieldPolicy(candidate.StorageMode, FieldStoragePlain) == FieldStorageEncrypted {
			return fmt.Errorf("capability %q admin resource %q field %q encrypted fields cannot use keyword search", owner, resource.Resource, field)
		}
	}
	defaultSortKey := strings.TrimSpace(resource.DefaultSortKey)
	if defaultSortKey != "" {
		if _, ok := fieldKeys[defaultSortKey]; !ok {
			return fmt.Errorf("capability %q admin resource %q default sort key %s is not declared", owner, resource.Resource, defaultSortKey)
		}
		if candidate, ok := fields[defaultSortKey]; ok && defaultAdminFieldPolicy(candidate.StorageMode, FieldStoragePlain) == FieldStorageEncrypted {
			return fmt.Errorf("capability %q admin resource %q field %q encrypted fields cannot be sorted", owner, resource.Resource, defaultSortKey)
		}
	}
	return nil
}

func validateAdminResourceProtection(owner ID, resource AdminResource, hasEncryptedField bool) error {
	protection := resource.Protection
	if protection == nil {
		if hasEncryptedField {
			return fmt.Errorf("capability %q admin resource %q encrypted fields require resource protection metadata", owner, resource.Resource)
		}
		return nil
	}
	if protection.SchemaVersion == 0 {
		return fmt.Errorf("capability %q admin resource %q protection schemaVersion is required", owner, resource.Resource)
	}
	scope := strings.TrimSpace(protection.Scope)
	if scope == "" {
		return fmt.Errorf("capability %q admin resource %q protection scope is required", owner, resource.Resource)
	}
	if !slices.Contains(adminResourceProtectionScopes, scope) {
		return fmt.Errorf("capability %q admin resource %q protection scope is unsupported", owner, resource.Resource)
	}
	tenantField := strings.TrimSpace(protection.TenantField)
	if scope == "global" {
		if tenantField != "" {
			return fmt.Errorf("capability %q admin resource %q global protection scope cannot declare tenantField", owner, resource.Resource)
		}
		return nil
	}
	if tenantField == "" {
		return fmt.Errorf("capability %q admin resource %q protection tenantField is required for tenant-field scope", owner, resource.Resource)
	}
	for _, field := range resource.Fields {
		if strings.TrimSpace(field.Key) != tenantField {
			continue
		}
		if defaultAdminFieldPolicy(field.StorageMode, FieldStoragePlain) != FieldStoragePlain || field.Protection != nil {
			return fmt.Errorf("capability %q admin resource %q protection tenantField %q must use plain storage", owner, resource.Resource, tenantField)
		}
		if !field.Required {
			return fmt.Errorf("capability %q admin resource %q protection tenantField %q must be required", owner, resource.Resource, tenantField)
		}
		return nil
	}
	return fmt.Errorf("capability %q admin resource %q protection tenantField %q is not declared", owner, resource.Resource, tenantField)
}

func validateAdminResourceActions(owner ID, resource AdminResource) error {
	seen := map[string]struct{}{}
	for _, action := range resource.Actions {
		key := strings.TrimSpace(action.Key)
		switch {
		case key == "":
			return fmt.Errorf("capability %q admin resource %q action key is required", owner, resource.Resource)
		case strings.TrimSpace(action.Label.ZH) == "" || strings.TrimSpace(action.Label.EN) == "":
			return fmt.Errorf("capability %q admin resource %q action %q label is required", owner, resource.Resource, key)
		case strings.TrimSpace(action.Permission) == "":
			return fmt.Errorf("capability %q admin resource %q action %q permission is required", owner, resource.Resource, key)
		}
		if _, exists := seen[key]; exists {
			return fmt.Errorf("capability %q admin resource %q duplicate action key %q", owner, resource.Resource, key)
		}
		seen[key] = struct{}{}
		if err := validateAdminResourceAction(owner, resource.Resource, resource.PermissionPrefix, action); err != nil {
			return err
		}
	}
	return nil
}

func validateAdminResourceAction(owner ID, resource string, permissionPrefix string, action AdminResourceAction) error {
	kind := strings.TrimSpace(action.Kind)
	tone := strings.TrimSpace(action.Tone)
	method := strings.ToUpper(strings.TrimSpace(action.Method))
	route := strings.TrimSpace(action.Route)
	if kind == "" {
		kind = "row"
	}
	if tone == "" {
		tone = "default"
	}
	if err := validateAdminPermission(owner, resource, fmt.Sprintf("action %s", action.Key), permissionPrefix, action.Permission); err != nil {
		return err
	}
	switch kind {
	case "row", "batch", "resource":
	default:
		return fmt.Errorf("capability %q admin resource %q action %q kind must be row, batch or resource", owner, resource, action.Key)
	}
	switch tone {
	case "default", "primary", "danger", "warning":
	default:
		return fmt.Errorf("capability %q admin resource %q action %q tone must be default, primary, danger or warning", owner, resource, action.Key)
	}
	if route != "" {
		if !strings.HasPrefix(route, "/api/admin/") {
			return fmt.Errorf("capability %q admin resource %q action %q route must start with /api/admin/", owner, resource, action.Key)
		}
		switch method {
		case "GET", "POST", "PUT", "PATCH", "DELETE":
		default:
			return fmt.Errorf("capability %q admin resource %q action %q method must be GET, POST, PUT, PATCH or DELETE", owner, resource, action.Key)
		}
	}
	if tone == "danger" && action.Confirm == nil {
		return fmt.Errorf("capability %q admin resource %q action %q danger action requires confirmation", owner, resource, action.Key)
	}
	if action.Confirm != nil && (strings.TrimSpace(action.Confirm.Title.ZH) == "" || strings.TrimSpace(action.Confirm.Title.EN) == "") {
		return fmt.Errorf("capability %q admin resource %q action %q confirmation title is required", owner, resource, action.Key)
	}
	return nil
}

func validateAdminResourcePanels(owner ID, resource AdminResource) error {
	seen := map[string]struct{}{}
	for _, panel := range resource.Panels {
		key := strings.TrimSpace(panel.Key)
		switch {
		case key == "":
			return fmt.Errorf("capability %q admin resource %q panel key is required", owner, resource.Resource)
		case strings.TrimSpace(panel.Label.ZH) == "" || strings.TrimSpace(panel.Label.EN) == "":
			return fmt.Errorf("capability %q admin resource %q panel %q label is required", owner, resource.Resource, key)
		}
		if _, exists := seen[key]; exists {
			return fmt.Errorf("capability %q admin resource %q duplicate panel key %q", owner, resource.Resource, key)
		}
		seen[key] = struct{}{}
		if err := validateAdminResourcePanel(owner, resource.Resource, resource.PermissionPrefix, panel); err != nil {
			return err
		}
	}
	return nil
}

func validateAdminResourcePanel(owner ID, resource string, permissionPrefix string, panel AdminResourcePanel) error {
	kind := strings.TrimSpace(panel.Kind)
	if kind == "" {
		kind = "custom"
	}
	if strings.TrimSpace(panel.Permission) != "" {
		if err := validateAdminPermission(owner, resource, fmt.Sprintf("panel %s", panel.Key), permissionPrefix, panel.Permission); err != nil {
			return err
		}
	}
	switch kind {
	case "fields", "permissions", "audit", "approval", "files", "custom":
	default:
		return fmt.Errorf("capability %q admin resource %q panel %q kind must be fields, permissions, audit, approval, files or custom", owner, resource, panel.Key)
	}
	component := strings.TrimSpace(panel.Component)
	if strings.Contains(component, "/") || strings.Contains(component, "\\") || strings.Contains(component, ".") {
		return fmt.Errorf("capability %q admin resource %q panel %q component must be a semantic key", owner, resource, panel.Key)
	}
	return nil
}

func validateAdminRuntimeSlots(owner ID, resource AdminResource) error {
	fieldKeys := adminResourceFieldKeys(resource)
	groupKeys := adminResourceGroupKeys(resource)
	seen := map[string]struct{}{}
	for _, slot := range resource.RuntimeSlots {
		slotID := strings.TrimSpace(slot.SlotID)
		switch {
		case slotID == "":
			return fmt.Errorf("capability %q admin resource %q runtime slot id is required", owner, resource.Resource)
		case strings.Contains(slotID, "/") || strings.Contains(slotID, "\\"):
			return fmt.Errorf("capability %q admin resource %q runtime slot %s id must be a semantic key", owner, resource.Resource, slotID)
		case strings.TrimSpace(slot.Label.ZH) == "" || strings.TrimSpace(slot.Label.EN) == "":
			return fmt.Errorf("capability %q admin resource %q runtime slot %s label is required", owner, resource.Resource, slotID)
		case strings.TrimSpace(slot.Description.ZH) == "" || strings.TrimSpace(slot.Description.EN) == "":
			return fmt.Errorf("capability %q admin resource %q runtime slot %s description is required", owner, resource.Resource, slotID)
		case strings.TrimSpace(slot.Permission) == "":
			return fmt.Errorf("capability %q admin resource %q runtime slot %s permission is required", owner, resource.Resource, slotID)
		}
		if err := validateAdminPermission(owner, resource.Resource, fmt.Sprintf("runtime slot %s", slotID), resource.PermissionPrefix, slot.Permission); err != nil {
			return err
		}
		region := strings.TrimSpace(slot.Region)
		if !slices.Contains(adminRuntimeSlotRegions, region) {
			return fmt.Errorf("capability %q admin resource %q runtime slot %s region must be one of %s", owner, resource.Resource, slotID, strings.Join(adminRuntimeSlotRegions, ","))
		}
		if variant := strings.TrimSpace(slot.Variant); variant != "" && !slices.Contains(adminRuntimeSlotVariants, variant) {
			return fmt.Errorf("capability %q admin resource %q runtime slot %s variant must be one of %s", owner, resource.Resource, slotID, strings.Join(adminRuntimeSlotVariants, ","))
		}
		if mode := strings.TrimSpace(slot.DataBinding.Mode); mode != "" && !slices.Contains(adminRuntimeSlotDataBindingModes, mode) {
			return fmt.Errorf("capability %q admin resource %q runtime slot %s data binding mode must be one of %s", owner, resource.Resource, slotID, strings.Join(adminRuntimeSlotDataBindingModes, ","))
		}
		if strings.HasPrefix(region, "form.section.") {
			targetSection := strings.TrimSpace(slot.TargetSection)
			if targetSection == "" {
				return fmt.Errorf("capability %q admin resource %q runtime slot %s targetSection is required for section slots", owner, resource.Resource, slotID)
			}
			if len(groupKeys) > 0 {
				if _, ok := groupKeys[targetSection]; !ok {
					return fmt.Errorf("capability %q admin resource %q runtime slot %s targetSection %q is not declared", owner, resource.Resource, slotID, targetSection)
				}
			}
		}
		if region == "field.control" {
			targetField := strings.TrimSpace(slot.TargetField)
			if targetField == "" {
				return fmt.Errorf("capability %q admin resource %q runtime slot %s targetField is required for field.control", owner, resource.Resource, slotID)
			}
			if _, ok := fieldKeys[targetField]; !ok {
				return fmt.Errorf("capability %q admin resource %q runtime slot %s targetField %q is not declared", owner, resource.Resource, slotID, targetField)
			}
		}
		for _, field := range slot.DataBinding.Fields {
			field = strings.TrimSpace(field)
			if field == "" {
				return fmt.Errorf("capability %q admin resource %q runtime slot %s data binding field is required", owner, resource.Resource, slotID)
			}
			if _, ok := fieldKeys[field]; !ok {
				return fmt.Errorf("capability %q admin resource %q runtime slot %s data binding field %q is not declared", owner, resource.Resource, slotID, field)
			}
		}
		key := slotID + ":" + region + ":" + strings.TrimSpace(slot.TargetSection) + ":" + strings.TrimSpace(slot.TargetField)
		if _, exists := seen[key]; exists {
			return fmt.Errorf("capability %q admin resource %q duplicate runtime slot %s", owner, resource.Resource, slotID)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func adminResourceFieldKeys(resource AdminResource) map[string]struct{} {
	keys := map[string]struct{}{}
	for _, key := range adminRelationRecordFields {
		keys[key] = struct{}{}
	}
	for _, field := range resource.Fields {
		key := strings.TrimSpace(field.Key)
		if key != "" {
			keys[key] = struct{}{}
		}
	}
	return keys
}

func adminResourceGroupKeys(resource AdminResource) map[string]struct{} {
	keys := map[string]struct{}{}
	for _, group := range resource.FormGroups {
		key := strings.TrimSpace(group.Key)
		if key != "" {
			keys[key] = struct{}{}
		}
	}
	for _, field := range resource.Fields {
		key := strings.TrimSpace(field.Group)
		if key != "" {
			keys[key] = struct{}{}
		}
	}
	return keys
}

func declaredAdminFormGroupKeys(resource AdminResource) map[string]struct{} {
	keys := map[string]struct{}{}
	for _, group := range resource.FormGroups {
		key := strings.TrimSpace(group.Key)
		if key != "" {
			keys[key] = struct{}{}
		}
	}
	return keys
}

func validateAdminMenu(owner ID, resource AdminResource) error {
	if resource.Menu.Route == "" && resource.Menu.Group == "" && resource.Menu.Icon == "" {
		return nil
	}
	switch {
	case strings.TrimSpace(resource.Menu.Route) == "":
		return fmt.Errorf("capability %q admin resource %q menu route is required", owner, resource.Resource)
	case !validAdminMenuRoute(resource.Menu):
		return fmt.Errorf("capability %q admin resource %q menu route must start with / or be an http(s) URL when external", owner, resource.Resource)
	case strings.TrimSpace(resource.Menu.Group) == "":
		return fmt.Errorf("capability %q admin resource %q menu group is required", owner, resource.Resource)
	case strings.TrimSpace(resource.Menu.Icon) == "":
		return fmt.Errorf("capability %q admin resource %q menu icon is required", owner, resource.Resource)
	}
	return nil
}

func validAdminMenuRoute(menu AdminMenu) bool {
	route := strings.TrimSpace(menu.Route)
	if strings.HasPrefix(route, "/") {
		return true
	}
	if !menu.External {
		return false
	}
	return strings.HasPrefix(route, "https://") || strings.HasPrefix(route, "http://")
}

func validAdminPermissionPrefix(prefix string) bool {
	prefix = strings.TrimSpace(prefix)
	segments := strings.Split(prefix, ":")
	return len(segments) == 2 && segments[0] == "admin" && validAdminPermissionSegment(segments[1])
}

func validateAdminPermission(owner ID, resource string, label string, permissionPrefix string, permission string) error {
	permission = strings.TrimSpace(permission)
	expectedPrefix := strings.TrimSpace(permissionPrefix) + ":"
	if !strings.HasPrefix(permission, expectedPrefix) {
		return fmt.Errorf("capability %q admin resource %q %s permission must start with %q", owner, resource, label, expectedPrefix)
	}
	suffix := strings.TrimPrefix(permission, expectedPrefix)
	if !validAdminPermissionSegment(suffix) {
		return fmt.Errorf("capability %q admin resource %q %s permission suffix must use lowercase letters, numbers or hyphens", owner, resource, label)
	}
	return nil
}

func validAdminPermissionSegment(segment string) bool {
	if segment == "" {
		return false
	}
	for _, char := range segment {
		if char >= 'a' && char <= 'z' {
			continue
		}
		if char >= '0' && char <= '9' {
			continue
		}
		if char == '-' {
			continue
		}
		return false
	}
	return true
}

func validateAdminField(owner ID, resource string, field AdminField) error {
	switch {
	case strings.TrimSpace(field.Key) == "":
		return fmt.Errorf("capability %q admin resource %q field key is required", owner, resource)
	case strings.TrimSpace(field.Label.ZH) == "" || strings.TrimSpace(field.Label.EN) == "":
		return fmt.Errorf("capability %q admin resource %q field %q label is required", owner, resource, field.Key)
	case field.Source != "record" && field.Source != "values":
		return fmt.Errorf("capability %q admin resource %q field %q source must be record or values", owner, resource, field.Key)
	case field.Type != "text" &&
		field.Type != "textarea" &&
		field.Type != "select" &&
		field.Type != "multiselect" &&
		field.Type != "datetime" &&
		field.Type != "switch" &&
		field.Type != "number" &&
		field.Type != "color":
		return fmt.Errorf("capability %q admin resource %q field %q type is unsupported", owner, resource, field.Key)
	}
	if (field.Type == "select" || field.Type == "multiselect") && len(field.Options) == 0 && field.Relation == nil {
		return fmt.Errorf("capability %q admin resource %q field %q select options are required", owner, resource, field.Key)
	}
	if !hasOptionalLocalizedText(field.Help) {
		return fmt.Errorf("capability %q admin resource %q field %s help must declare zh/en text", owner, resource, field.Key)
	}
	if err := validateAdminFieldPolicy(owner, resource, field); err != nil {
		return err
	}
	for _, option := range field.Options {
		value := strings.TrimSpace(option.Value)
		switch {
		case value == "":
			return fmt.Errorf("capability %q admin resource %q field %s option value is required", owner, resource, field.Key)
		case !hasLocalizedText(option.Label):
			return fmt.Errorf("capability %q admin resource %q field %s option %s label is required", owner, resource, field.Key, value)
		}
	}
	if err := validateAdminFieldRelation(owner, resource, field); err != nil {
		return err
	}
	return nil
}

func validateAdminFieldPolicy(owner ID, resource string, field AdminField) error {
	sensitivity := defaultAdminFieldPolicy(field.Sensitivity, FieldSensitivityPublic)
	storageMode := defaultAdminFieldPolicy(field.StorageMode, FieldStoragePlain)
	responseMode := defaultAdminFieldPolicy(field.ResponseMode, FieldProjectionFull)
	exportMode := defaultAdminFieldPolicy(field.ExportMode, FieldProjectionFull)
	if !slices.Contains(adminFieldSensitivities, sensitivity) {
		return fmt.Errorf("capability %q admin resource %q field %q sensitivity is unsupported", owner, resource, field.Key)
	}
	if !slices.Contains(adminFieldStorageModes, storageMode) {
		return fmt.Errorf("capability %q admin resource %q field %q storageMode is unsupported", owner, resource, field.Key)
	}
	if !slices.Contains(adminFieldProjectionModes, responseMode) {
		return fmt.Errorf("capability %q admin resource %q field %q responseMode is unsupported", owner, resource, field.Key)
	}
	if !slices.Contains(adminFieldProjectionModes, exportMode) {
		return fmt.Errorf("capability %q admin resource %q field %q exportMode is unsupported", owner, resource, field.Key)
	}
	if storageMode == FieldStorageEncrypted && field.Source != "values" {
		return fmt.Errorf("capability %q admin resource %q field %q encrypted storage requires values source", owner, resource, field.Key)
	}
	if (sensitivity == FieldSensitivitySensitive || sensitivity == FieldSensitivitySecret) && field.Source == "record" {
		return fmt.Errorf("capability %q admin resource %q field %q sensitive or secret values cannot use record storage", owner, resource, field.Key)
	}
	if sensitivity == FieldSensitivityPersonal && storageMode == FieldStoragePlain {
		return fmt.Errorf("capability %q admin resource %q field %q personal values require masked or protected storage", owner, resource, field.Key)
	}
	if (sensitivity == FieldSensitivitySensitive || sensitivity == FieldSensitivitySecret) && storageMode == FieldStoragePlain {
		return fmt.Errorf("capability %q admin resource %q field %q sensitive or secret values require protected storage", owner, resource, field.Key)
	}
	if storageMode == FieldStorageMasked && sensitivity != FieldSensitivityPersonal {
		return fmt.Errorf("capability %q admin resource %q field %q masked storage requires personal sensitivity", owner, resource, field.Key)
	}
	if storageMode == FieldStorageMasked && (!isAdminMaskedProjection(responseMode) || !isAdminMaskedProjection(exportMode)) {
		return fmt.Errorf("capability %q admin resource %q field %q masked storage must use masked or omitted response and export", owner, resource, field.Key)
	}
	if storageMode == FieldStorageHashed && (responseMode != FieldProjectionOmitted || exportMode != FieldProjectionOmitted) {
		return fmt.Errorf("capability %q admin resource %q field %q hashed storage must be omitted from response and export", owner, resource, field.Key)
	}
	if (responseMode == FieldProjectionMasked || exportMode == FieldProjectionMasked) && storageMode != FieldStorageMasked && storageMode != FieldStorageEncrypted {
		return fmt.Errorf("capability %q admin resource %q field %q masked projection requires masked or encrypted storage", owner, resource, field.Key)
	}
	if storageMode == FieldStorageEncrypted && (!isAdminEncryptedProjection(responseMode) || !isAdminEncryptedProjection(exportMode)) {
		return fmt.Errorf("capability %q admin resource %q field %q encrypted storage must use masked, privileged or omitted response and export", owner, resource, field.Key)
	}
	if err := validateAdminFieldProtection(owner, resource, field, storageMode); err != nil {
		return err
	}
	if err := validateAdminFieldMasking(owner, resource, field, storageMode, responseMode, exportMode); err != nil {
		return err
	}
	return nil
}

func validateAdminFieldMasking(owner ID, resource string, field AdminField, storageMode string, responseMode string, exportMode string) error {
	maskedProjection := responseMode == FieldProjectionMasked || exportMode == FieldProjectionMasked
	if field.Masking == nil {
		if storageMode == FieldStorageEncrypted && maskedProjection {
			return fmt.Errorf("capability %q admin resource %q field %q encrypted masked projection requires masking metadata", owner, resource, field.Key)
		}
		return nil
	}
	if storageMode != FieldStorageEncrypted {
		return fmt.Errorf("capability %q admin resource %q field %q masking metadata requires encrypted storage", owner, resource, field.Key)
	}
	if !maskedProjection {
		return fmt.Errorf("capability %q admin resource %q field %q masking metadata requires a masked response or export", owner, resource, field.Key)
	}
	policy := masking.Policy{
		Strategy: field.Masking.Strategy, PreservePrefix: field.Masking.PreservePrefix, PreserveSuffix: field.Masking.PreserveSuffix,
		MaskLength: field.Masking.MaskLength, Replacement: field.Masking.Replacement,
	}
	if err := masking.NewRuntime().Validate(policy); err != nil {
		return fmt.Errorf("capability %q admin resource %q field %q %v", owner, resource, field.Key, err)
	}
	return nil
}

func validateAdminFieldProtection(owner ID, resource string, field AdminField, storageMode string) error {
	if storageMode != FieldStorageEncrypted {
		if field.Protection != nil {
			return fmt.Errorf("capability %q admin resource %q field %q protection metadata requires encrypted storage", owner, resource, field.Key)
		}
		return nil
	}
	if field.Protection == nil {
		return fmt.Errorf("capability %q admin resource %q field %q encrypted storage requires protection metadata", owner, resource, field.Key)
	}
	format := strings.TrimSpace(field.Protection.Format)
	if format == "" {
		return fmt.Errorf("capability %q admin resource %q field %q protection format is required", owner, resource, field.Key)
	}
	if !slices.Contains(adminFieldProtectionFormats, format) {
		return fmt.Errorf("capability %q admin resource %q field %q protection format is unsupported", owner, resource, field.Key)
	}
	normalization := strings.TrimSpace(field.Protection.Normalization)
	if normalization == "" {
		return fmt.Errorf("capability %q admin resource %q field %q protection normalization is required", owner, resource, field.Key)
	}
	if !slices.Contains(adminFieldProtectionNormalizations, normalization) {
		return fmt.Errorf("capability %q admin resource %q field %q protection normalization is unsupported", owner, resource, field.Key)
	}
	namespace := strings.TrimSpace(field.Protection.BlindIndexNamespace)
	if namespace != "" && !validAdminProtectionName(namespace) {
		return fmt.Errorf("capability %q admin resource %q field %q blindIndexNamespace must be canonical lowercase kebab-case", owner, resource, field.Key)
	}
	return nil
}

func isAdminEncryptedProjection(mode string) bool {
	return mode == FieldProjectionMasked || mode == FieldProjectionPrivileged || mode == FieldProjectionOmitted
}

func validAdminProtectionName(value string) bool {
	if value == "" || strings.Trim(value, " ") != value || strings.HasPrefix(value, "-") || strings.HasSuffix(value, "-") || strings.Contains(value, "--") {
		return false
	}
	return validAdminPermissionSegment(value)
}

func isAdminMaskedProjection(mode string) bool {
	return mode == FieldProjectionMasked || mode == FieldProjectionOmitted
}

func defaultAdminFieldPolicy(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func hasLocalizedText(value LocalizedText) bool {
	return strings.TrimSpace(value.ZH) != "" && strings.TrimSpace(value.EN) != ""
}

func hasOptionalLocalizedText(value LocalizedText) bool {
	if strings.TrimSpace(value.ZH) == "" && strings.TrimSpace(value.EN) == "" {
		return true
	}
	return hasLocalizedText(value)
}

func validateAdminFieldRelation(owner ID, resource string, field AdminField) error {
	if field.Relation == nil {
		return nil
	}
	relation := field.Relation
	switch {
	case field.Type != "select" && field.Type != "multiselect":
		return fmt.Errorf("capability %q admin resource %q field %q relation requires select or multiselect type", owner, resource, field.Key)
	case relation.Multiple && field.Type != "multiselect":
		return fmt.Errorf("capability %q admin resource %q field %q multiple relation requires multiselect type", owner, resource, field.Key)
	case strings.TrimSpace(relation.Resource) == "":
		return fmt.Errorf("capability %q admin resource %q field %q relation resource is required", owner, resource, field.Key)
	case strings.TrimSpace(relation.ValueField) == "":
		return fmt.Errorf("capability %q admin resource %q field %q relation value field is required", owner, resource, field.Key)
	case strings.TrimSpace(relation.LabelField) == "":
		return fmt.Errorf("capability %q admin resource %q field %q relation label field is required", owner, resource, field.Key)
	case relation.SortOrder != "" && relation.SortOrder != "asc" && relation.SortOrder != "desc":
		return fmt.Errorf("capability %q admin resource %q field %q relation sort order must be asc or desc", owner, resource, field.Key)
	case relation.Display != "" && relation.Display != "select" && relation.Display != "tree":
		return fmt.Errorf("capability %q admin resource %q field %q relation display must be select or tree", owner, resource, field.Key)
	case relation.Display == "tree" && strings.TrimSpace(relation.ParentField) == "":
		return fmt.Errorf("capability %q admin resource %q field %q tree relation parent field is required", owner, resource, field.Key)
	}
	for _, filter := range relation.Filters {
		operator := strings.TrimSpace(filter.Operator)
		if strings.TrimSpace(filter.Field) == "" || operator == "" {
			return fmt.Errorf("capability %q admin resource %q field %q relation filters require field and operator", owner, resource, field.Key)
		}
		if !slices.Contains(adminRelationFilterOperators, operator) {
			return fmt.Errorf("capability %q admin resource %q field %q relation filter operator must be one of %s", owner, resource, field.Key, strings.Join(adminRelationFilterOperators, ","))
		}
	}
	return nil
}
