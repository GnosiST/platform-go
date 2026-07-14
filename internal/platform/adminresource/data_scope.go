package adminresource

import (
	"slices"
	"strings"

	"platform-go/internal/platform/rbac"
)

const (
	dataScopeAll                     = "all"
	dataScopeCurrentOrg              = "current_org"
	dataScopeCurrentAndChildren      = "current_and_children"
	dataScopeCustomOrgs              = "custom_orgs"
	dataScopeCurrentArea             = "current_area"
	dataScopeCurrentAndChildrenAreas = "current_and_children_areas"
	dataScopeCustomAreas             = "custom_areas"
	dataScopeSelf                    = "self"
)

type dataScopePolicy struct {
	all        bool
	tenantCode string
	orgCodes   map[string]struct{}
	areaCodes  map[string]struct{}
	self       bool
	username   string
	userID     string
}

func (s *Store) ListForPrincipal(resource string, principal rbac.Principal) ([]Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	items, ok := s.resources[resource]
	if !ok {
		return nil, ErrUnknownResource
	}
	return cloneRecords(s.filterRecordsForPrincipalLocked(resource, visibleRecords(resource, items), principal)), nil
}

func (s *Store) InternalRecordForPrincipal(resource string, id string, principal rbac.Principal) (Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	items, ok := s.resources[resource]
	if !ok {
		return Record{}, ErrUnknownResource
	}
	index := recordIndexByID(items, id)
	if index < 0 {
		return Record{}, ErrRecordNotFound
	}
	record := items[index]
	schema, ok := s.schemas[resource]
	if !ok {
		return Record{}, ErrUnknownResource
	}
	if resourceHasDataScopeFields(resource, schema) {
		policy := s.dataScopePolicyLocked(principal)
		if !policy.all && !policy.allowsRecord(resource, schema, record) {
			return Record{}, ErrRecordNotFound
		}
	}
	return cloneRecord(record), nil
}

func (s *Store) filterRecordsForPrincipalLocked(resource string, items []Record, principal rbac.Principal) []Record {
	if len(items) == 0 {
		return items
	}
	schema, ok := s.schemas[resource]
	if !ok || !resourceHasDataScopeFields(resource, schema) {
		return items
	}
	policy := s.dataScopePolicyLocked(principal)
	if policy.all {
		return items
	}
	filtered := make([]Record, 0, len(items))
	for _, item := range items {
		if policy.allowsRecord(resource, schema, item) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func (s *Store) dataScopePolicyLocked(principal rbac.Principal) dataScopePolicy {
	policy := dataScopePolicy{
		tenantCode: strings.TrimSpace(principal.User.TenantCode),
		orgCodes:   map[string]struct{}{},
		areaCodes:  map[string]struct{}{},
		username:   strings.TrimSpace(principal.User.Username),
		userID:     strings.TrimSpace(principal.User.ID),
	}
	for _, roleCode := range principal.Roles {
		role := findRecordByCode(visibleRecords("roles", s.resources["roles"]), roleCode)
		if role == nil || role.Status == "disabled" {
			continue
		}
		scope := strings.TrimSpace(role.Values["dataScope"])
		if scope == "" {
			scope = dataScopeAll
		}
		switch scope {
		case dataScopeAll:
			policy.all = true
			return policy
		case dataScopeCurrentOrg:
			policy.addOrgCode(principal.User.OrgUnitCode)
		case dataScopeCurrentAndChildren:
			policy.addOrgCodes(s.orgUnitAndDescendantCodesLocked(principal.User.OrgUnitCode)...)
		case dataScopeCustomOrgs:
			policy.addOrgCodes(rbac.ParsePermissionList(role.Values["dataScopeOrgCodes"])...)
		case dataScopeCurrentArea:
			policy.addAreaCode(principal.User.AreaCode)
		case dataScopeCurrentAndChildrenAreas:
			policy.addAreaCodes(s.areaCodeAndDescendantCodesLocked(principal.User.AreaCode)...)
		case dataScopeCustomAreas:
			policy.addAreaCodes(rbac.ParsePermissionList(role.Values["dataScopeAreaCodes"])...)
		case dataScopeSelf:
			policy.self = true
		}
	}
	return policy
}

func (p *dataScopePolicy) addOrgCode(code string) {
	code = strings.TrimSpace(code)
	if code == "" {
		return
	}
	p.orgCodes[code] = struct{}{}
}

func (p *dataScopePolicy) addOrgCodes(codes ...string) {
	for _, code := range codes {
		p.addOrgCode(code)
	}
}

func (p *dataScopePolicy) addAreaCode(code string) {
	code = strings.TrimSpace(code)
	if code == "" {
		return
	}
	p.areaCodes[code] = struct{}{}
}

func (p *dataScopePolicy) addAreaCodes(codes ...string) {
	for _, code := range codes {
		p.addAreaCode(code)
	}
}

func (s *Store) orgUnitAndDescendantCodesLocked(root string) []string {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil
	}
	codes := []string{root}
	seen := map[string]struct{}{root: {}}
	for {
		added := false
		for _, org := range visibleRecords("org-units", s.resources["org-units"]) {
			parentCode := strings.TrimSpace(org.Values["parentCode"])
			if parentCode == "" {
				continue
			}
			if _, ok := seen[parentCode]; !ok {
				continue
			}
			if _, ok := seen[org.Code]; ok {
				continue
			}
			seen[org.Code] = struct{}{}
			codes = append(codes, org.Code)
			added = true
		}
		if !added {
			return codes
		}
	}
}

func (s *Store) areaCodeAndDescendantCodesLocked(root string) []string {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil
	}
	codes := []string{root}
	seen := map[string]struct{}{root: {}}
	for {
		added := false
		for _, area := range visibleRecords("area-codes", s.resources["area-codes"]) {
			parentCode := strings.TrimSpace(area.Values["parentCode"])
			if parentCode == "" {
				continue
			}
			if _, ok := seen[parentCode]; !ok {
				continue
			}
			if _, ok := seen[area.Code]; ok {
				continue
			}
			seen[area.Code] = struct{}{}
			codes = append(codes, area.Code)
			added = true
		}
		if !added {
			return codes
		}
	}
}

func (p dataScopePolicy) allowsRecord(resource string, schema Schema, record Record) bool {
	if p.tenantCode != "" {
		if resource == "tenants" {
			if !strings.EqualFold(record.Code, p.tenantCode) {
				return false
			}
		} else if schemaHasField(schema, "tenantCode") && !strings.EqualFold(recordValue(record, "tenantCode"), p.tenantCode) {
			return false
		}
	}

	orgCode, hasOrgScope := recordOrgCode(resource, schema, record)
	areaCode, hasAreaScope := recordAreaCode(resource, schema, record)
	if len(p.orgCodes) > 0 {
		if hasOrgScope {
			if _, ok := p.orgCodes[orgCode]; !ok {
				return false
			}
		}
	}
	if len(p.areaCodes) > 0 {
		if hasAreaScope {
			if _, ok := p.areaCodes[areaCode]; !ok {
				return false
			}
		}
	}
	if p.self && selfOwnedRecord(resource, schema, record, p) {
		return true
	}
	if len(p.orgCodes) > 0 || len(p.areaCodes) > 0 {
		return true
	}
	if hasOrgScope {
		return false
	}
	if hasAreaScope {
		return false
	}
	return p.tenantCode != "" && (resource == "tenants" || schemaHasField(schema, "tenantCode"))
}

func recordOrgCode(resource string, schema Schema, record Record) (string, bool) {
	if resource == "org-units" {
		return strings.TrimSpace(record.Code), true
	}
	if schemaHasField(schema, "orgUnitCode") {
		return strings.TrimSpace(recordValue(record, "orgUnitCode")), true
	}
	return "", false
}

func recordAreaCode(resource string, schema Schema, record Record) (string, bool) {
	if resource == "area-codes" {
		return strings.TrimSpace(record.Code), true
	}
	if schemaHasField(schema, "areaCode") {
		return strings.TrimSpace(recordValue(record, "areaCode")), true
	}
	return "", false
}

func selfOwnedRecord(resource string, schema Schema, record Record, policy dataScopePolicy) bool {
	if resource == "users" && policy.username != "" && strings.EqualFold(record.Code, policy.username) {
		return true
	}
	for _, key := range []string{"userCode", "ownerCode", "owner", "createdBy"} {
		if !schemaHasField(schema, key) {
			continue
		}
		value := recordValue(record, key)
		if policy.username != "" && strings.EqualFold(value, policy.username) {
			return true
		}
		if policy.userID != "" && strings.EqualFold(value, policy.userID) {
			return true
		}
	}
	return false
}

func resourceHasDataScopeFields(resource string, schema Schema) bool {
	return resource == "tenants" ||
		resource == "org-units" ||
		resource == "area-codes" ||
		schemaHasField(schema, "tenantCode") ||
		schemaHasField(schema, "areaCode") ||
		schemaHasField(schema, "orgUnitCode") ||
		slices.ContainsFunc(schema.Fields, func(field FieldDefinition) bool {
			return field.Key == "userCode" || field.Key == "ownerCode" || field.Key == "owner" || field.Key == "createdBy"
		})
}

func schemaHasField(schema Schema, key string) bool {
	return slices.ContainsFunc(schema.Fields, func(field FieldDefinition) bool {
		return field.Key == key
	})
}

func recordValue(record Record, key string) string {
	switch key {
	case "id":
		return record.ID
	case "code":
		return record.Code
	case "name":
		return record.Name
	case "status":
		return record.Status
	case "description":
		return record.Description
	case "updatedAt":
		return record.UpdatedAt
	default:
		return strings.TrimSpace(record.Values[key])
	}
}
