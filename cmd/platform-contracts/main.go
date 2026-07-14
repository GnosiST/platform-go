package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"platform-go/internal/apps"
	"platform-go/internal/platform/bootstrap"
	"platform-go/internal/platform/capability"
	"platform-go/internal/platform/config"
	"platform-go/internal/platform/httpapi"
)

const (
	defaultAppRouteContractPath      = "resources/generated/app-route-contract.json"
	defaultAdminResourceContractPath = "resources/generated/admin-capability-resource-contract.json"
	defaultServiceContractPath       = "resources/generated/platform-service-contract.json"
	defaultPlatformAuditPath         = "resources/generated/platform-capability-audit.json"
)

type appRouteContractDocument struct {
	GeneratedBy   string                  `json:"generatedBy"`
	Source        string                  `json:"source"`
	SourceMode    string                  `json:"sourceMode"`
	SourceVersion string                  `json:"sourceVersion"`
	RouteCount    int                     `json:"routeCount"`
	Capabilities  []string                `json:"capabilities"`
	Permissions   []string                `json:"permissions"`
	Routes        []appRouteContractEntry `json:"routes"`
}

type appRouteContractEntry struct {
	CapabilityID string                   `json:"capabilityId"`
	Method       string                   `json:"method"`
	Path         string                   `json:"path"`
	Auth         capability.AppRouteAuth  `json:"auth"`
	Permission   string                   `json:"permission,omitempty"`
	Description  capability.LocalizedText `json:"description"`
}

type adminResourceContractDocument struct {
	GeneratedBy   string                       `json:"generatedBy"`
	Source        string                       `json:"source"`
	SourceMode    string                       `json:"sourceMode"`
	SourceVersion string                       `json:"sourceVersion"`
	ResourceCount int                          `json:"resourceCount"`
	Capabilities  []string                     `json:"capabilities"`
	Permissions   []string                     `json:"permissions"`
	Resources     []adminResourceContractEntry `json:"resources"`
}

type adminResourceContractEntry struct {
	CapabilityID     string                                  `json:"capabilityId"`
	Resource         string                                  `json:"resource"`
	Title            capability.LocalizedText                `json:"title"`
	Description      capability.LocalizedText                `json:"description"`
	PermissionPrefix string                                  `json:"permissionPrefix"`
	Permissions      map[string]string                       `json:"permissions"`
	Menu             adminMenuContract                       `json:"menu"`
	FormGroups       []adminFormGroupContract                `json:"formGroups,omitempty"`
	Fields           []adminFieldContract                    `json:"fields,omitempty"`
	SearchFields     []string                                `json:"searchFields,omitempty"`
	DefaultSortKey   string                                  `json:"defaultSortKey,omitempty"`
	Protection       *capability.AdminResourceProtection     `json:"protection,omitempty"`
	Deletion         *capability.AdminResourceDeletionPolicy `json:"deletion"`
}

type adminMenuContract struct {
	Route    string `json:"route,omitempty"`
	Parent   string `json:"parent,omitempty"`
	Group    string `json:"group,omitempty"`
	Icon     string `json:"icon,omitempty"`
	Order    int    `json:"order,omitempty"`
	External bool   `json:"external,omitempty"`
	Cache    bool   `json:"cache,omitempty"`
}

type adminFormGroupContract struct {
	Key         string                   `json:"key"`
	Label       capability.LocalizedText `json:"label"`
	Description capability.LocalizedText `json:"description"`
}

type adminFieldContract struct {
	Key          string                           `json:"key"`
	Label        capability.LocalizedText         `json:"label"`
	Type         string                           `json:"type"`
	Source       string                           `json:"source,omitempty"`
	Group        string                           `json:"group,omitempty"`
	Help         capability.LocalizedText         `json:"help"`
	Required     bool                             `json:"required,omitempty"`
	ReadOnly     bool                             `json:"readOnly,omitempty"`
	Searchable   bool                             `json:"searchable,omitempty"`
	Filterable   bool                             `json:"filterable,omitempty"`
	Sortable     bool                             `json:"sortable,omitempty"`
	Localizable  bool                             `json:"localizable,omitempty"`
	InTable      bool                             `json:"inTable,omitempty"`
	InForm       bool                             `json:"inForm,omitempty"`
	InDetail     bool                             `json:"inDetail,omitempty"`
	Width        int                              `json:"width,omitempty"`
	Options      []adminFieldOptionContract       `json:"options,omitempty"`
	Relation     *adminFieldRelationContract      `json:"relation,omitempty"`
	Validation   adminFieldValidationContract     `json:"validation,omitempty"`
	Sensitivity  string                           `json:"sensitivity"`
	StorageMode  string                           `json:"storageMode"`
	ResponseMode string                           `json:"responseMode"`
	ExportMode   string                           `json:"exportMode"`
	Protection   *capability.AdminFieldProtection `json:"protection,omitempty"`
}

type adminFieldOptionContract struct {
	Value string                   `json:"value"`
	Label capability.LocalizedText `json:"label"`
}

type adminFieldRelationContract struct {
	Resource    string                             `json:"resource"`
	ValueField  string                             `json:"valueField"`
	LabelField  string                             `json:"labelField"`
	Multiple    bool                               `json:"multiple,omitempty"`
	Filters     []adminFieldRelationFilterContract `json:"filters,omitempty"`
	SortField   string                             `json:"sortField,omitempty"`
	SortOrder   string                             `json:"sortOrder,omitempty"`
	Display     string                             `json:"display,omitempty"`
	ParentField string                             `json:"parentField,omitempty"`
	PathField   string                             `json:"pathField,omitempty"`
	RootValue   string                             `json:"rootValue,omitempty"`
}

type adminFieldRelationFilterContract struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

type adminFieldValidationContract struct {
	MinLength int      `json:"minLength,omitempty"`
	MaxLength int      `json:"maxLength,omitempty"`
	Min       *float64 `json:"min,omitempty"`
	Max       *float64 `json:"max,omitempty"`
	Pattern   string   `json:"pattern,omitempty"`
}

type platformAuditDocument struct {
	GeneratedBy             string                    `json:"generatedBy"`
	Source                  string                    `json:"source"`
	SourceMode              string                    `json:"sourceMode"`
	SourceVersion           string                    `json:"sourceVersion"`
	Status                  string                    `json:"status"`
	CapabilityCount         int                       `json:"capabilityCount"`
	ResourceCount           int                       `json:"resourceCount"`
	RouteCount              int                       `json:"routeCount"`
	AppRouteHandlerCount    int                       `json:"appRouteHandlerCount"`
	MissingAppRouteHandlers []string                  `json:"missingAppRouteHandlers,omitempty"`
	AdminPermissionCount    int                       `json:"adminPermissionCount"`
	AppPermissionCount      int                       `json:"appPermissionCount"`
	AuthProviderCount       int                       `json:"authProviderCount"`
	DemoDataSetCount        int                       `json:"demoDataSetCount"`
	MigrationCount          int                       `json:"migrationCount"`
	SeedCount               int                       `json:"seedCount"`
	ServiceCount            int                       `json:"serviceCount"`
	ServiceOperationCount   int                       `json:"serviceOperationCount"`
	ServiceEventCount       int                       `json:"serviceEventCount"`
	Capabilities            []platformAuditCapability `json:"capabilities"`
}

type platformAuditCapability struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	Version           string   `json:"version"`
	Dependencies      []string `json:"dependencies,omitempty"`
	AdminResources    []string `json:"adminResources,omitempty"`
	AppRoutes         []string `json:"appRoutes,omitempty"`
	AppRouteHandlers  []string `json:"appRouteHandlers,omitempty"`
	AuthProviders     []string `json:"authProviders,omitempty"`
	DemoDataSets      []string `json:"demoDataSets,omitempty"`
	Migrations        []string `json:"migrations,omitempty"`
	Seeds             []string `json:"seeds,omitempty"`
	ServiceOperations []string `json:"serviceOperations,omitempty"`
	ServiceEvents     []string `json:"serviceEvents,omitempty"`
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		return errors.New("contract command is required")
	}
	switch args[0] {
	case "app-routes":
		return runAppRoutes(args[1:], stdout, stderr)
	case "admin-resources":
		return runAdminResources(args[1:], stdout, stderr)
	case "service-manifests":
		return runServiceManifests(args[1:], stdout, stderr)
	case "audit":
		return runAudit(args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unknown contract command %q", args[0])
	}
}

func runServiceManifests(args []string, stdout io.Writer, stderr io.Writer) error {
	flags := flag.NewFlagSet("service-manifests", flag.ContinueOnError)
	flags.SetOutput(stderr)
	outputPath := flags.String("output", defaultServiceContractPath, "output service contract path")
	stdoutMode := flags.Bool("stdout", false, "write service contract to stdout")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("unexpected service-manifests arguments: %v", flags.Args())
	}

	manifests, err := enabledManifests()
	if err != nil {
		return err
	}
	document, err := capability.ServiceContractDocumentFromManifests(manifests)
	if err != nil {
		return err
	}
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(document); err != nil {
		return err
	}
	if *stdoutMode {
		_, err = stdout.Write(output.Bytes())
		return err
	}
	if err := os.MkdirAll(filepath.Dir(*outputPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(*outputPath, output.Bytes(), 0644)
}

func runAppRoutes(args []string, stdout io.Writer, stderr io.Writer) error {
	flags := flag.NewFlagSet("app-routes", flag.ContinueOnError)
	flags.SetOutput(stderr)
	outputPath := flags.String("output", defaultAppRouteContractPath, "output contract path")
	stdoutMode := flags.Bool("stdout", false, "write contract to stdout")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("unexpected app-routes arguments: %v", flags.Args())
	}

	manifests, err := enabledManifests()
	if err != nil {
		return err
	}
	document, err := buildAppRouteContractDocument(manifests)
	if err != nil {
		return err
	}
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(document); err != nil {
		return err
	}
	if *stdoutMode {
		_, err = stdout.Write(output.Bytes())
		return err
	}
	if err := os.MkdirAll(filepath.Dir(*outputPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(*outputPath, output.Bytes(), 0644)
}

func runAdminResources(args []string, stdout io.Writer, stderr io.Writer) error {
	flags := flag.NewFlagSet("admin-resources", flag.ContinueOnError)
	flags.SetOutput(stderr)
	outputPath := flags.String("output", defaultAdminResourceContractPath, "output contract path")
	stdoutMode := flags.Bool("stdout", false, "write contract to stdout")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("unexpected admin-resources arguments: %v", flags.Args())
	}

	manifests, err := enabledManifests()
	if err != nil {
		return err
	}
	document, err := buildAdminResourceContractDocument(manifests)
	if err != nil {
		return err
	}
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(document); err != nil {
		return err
	}
	if *stdoutMode {
		_, err = stdout.Write(output.Bytes())
		return err
	}
	if err := os.MkdirAll(filepath.Dir(*outputPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(*outputPath, output.Bytes(), 0644)
}

func runAudit(args []string, stdout io.Writer, stderr io.Writer) error {
	flags := flag.NewFlagSet("audit", flag.ContinueOnError)
	flags.SetOutput(stderr)
	outputPath := flags.String("output", defaultPlatformAuditPath, "output audit path")
	stdoutMode := flags.Bool("stdout", false, "write audit to stdout")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("unexpected audit arguments: %v", flags.Args())
	}

	manifests, err := enabledManifests()
	if err != nil {
		return err
	}
	document, err := buildPlatformAuditDocument(manifests)
	if err != nil {
		return err
	}
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(document); err != nil {
		return err
	}
	if *stdoutMode {
		_, err = stdout.Write(output.Bytes())
		return err
	}
	if err := os.MkdirAll(filepath.Dir(*outputPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(*outputPath, output.Bytes(), 0644)
}

func enabledManifests() ([]capability.Manifest, error) {
	return bootstrap.CapabilitiesFromConfig(config.Load(), apps.DefaultManifests()...)
}

func buildAppRouteContractDocument(manifests []capability.Manifest) (appRouteContractDocument, error) {
	contracts, err := capability.AppRouteContracts(manifests)
	if err != nil {
		return appRouteContractDocument{}, err
	}
	routes := make([]appRouteContractEntry, 0, len(contracts))
	for _, contract := range contracts {
		routes = append(routes, appRouteContractEntry{
			CapabilityID: string(contract.CapabilityID),
			Method:       contract.Method,
			Path:         contract.Path,
			Auth:         contract.Auth,
			Permission:   contract.Permission,
			Description:  contract.Description,
		})
	}
	return appRouteContractDocument{
		GeneratedBy:   "cmd/platform-contracts app-routes",
		Source:        "capability.Manifest.App.Routes",
		SourceMode:    "go-manifest",
		SourceVersion: sourceVersion(manifests),
		RouteCount:    len(routes),
		Capabilities:  uniqueStringsFromRoutes(routes, func(route appRouteContractEntry) string { return route.CapabilityID }),
		Permissions:   uniqueStringsFromRoutes(routes, func(route appRouteContractEntry) string { return route.Permission }),
		Routes:        routes,
	}, nil
}

func buildAdminResourceContractDocument(manifests []capability.Manifest) (adminResourceContractDocument, error) {
	if err := capability.ValidateAdminSurface(manifests); err != nil {
		return adminResourceContractDocument{}, err
	}
	resources := make([]adminResourceContractEntry, 0)
	for _, manifest := range manifests {
		for _, resource := range manifest.Admin.Resources {
			resources = append(resources, adminResourceContractEntry{
				CapabilityID:     string(manifest.ID),
				Resource:         resource.Resource,
				Title:            resource.Title,
				Description:      resource.Description,
				PermissionPrefix: resource.PermissionPrefix,
				Permissions:      adminResourcePermissions(resource),
				Menu:             adminMenuFromCapability(resource.Menu),
				FormGroups:       adminFormGroupsFromCapability(resource.FormGroups),
				Fields:           adminFieldsFromCapability(resource.Fields),
				SearchFields:     append([]string(nil), resource.SearchFields...),
				DefaultSortKey:   resource.DefaultSortKey,
				Protection:       resource.Protection,
				Deletion:         resource.Deletion,
			})
		}
	}
	sort.Slice(resources, func(i, j int) bool {
		if resources[i].Resource == resources[j].Resource {
			return resources[i].CapabilityID < resources[j].CapabilityID
		}
		return resources[i].Resource < resources[j].Resource
	})
	return adminResourceContractDocument{
		GeneratedBy:   "cmd/platform-contracts admin-resources",
		Source:        "capability.Manifest.Admin.Resources",
		SourceMode:    "go-manifest",
		SourceVersion: sourceVersion(manifests),
		ResourceCount: len(resources),
		Capabilities:  uniqueStringsFromAdminResources(resources, func(resource adminResourceContractEntry) string { return resource.CapabilityID }),
		Permissions:   uniqueAdminResourcePermissions(resources),
		Resources:     resources,
	}, nil
}

func buildPlatformAuditDocument(manifests []capability.Manifest) (platformAuditDocument, error) {
	adminDocument, err := buildAdminResourceContractDocument(manifests)
	if err != nil {
		return platformAuditDocument{}, err
	}
	appDocument, err := buildAppRouteContractDocument(manifests)
	if err != nil {
		return platformAuditDocument{}, err
	}
	serviceDocument, err := capability.ServiceContractDocumentFromManifests(manifests)
	if err != nil {
		return platformAuditDocument{}, err
	}
	handlerCoverage, err := httpapi.AppRouteHandlerCoverage(manifests, apps.DefaultAppRoutes(nil))
	if err != nil {
		return platformAuditDocument{}, err
	}
	if err := capability.ValidateLifecycleDeclarations(manifests); err != nil {
		return platformAuditDocument{}, err
	}
	if err := capability.ValidateAuthProviderDeclarations(manifests); err != nil {
		return platformAuditDocument{}, err
	}
	if err := capability.ValidateDemoDataDeclarations(manifests); err != nil {
		return platformAuditDocument{}, err
	}

	capabilities := make([]platformAuditCapability, 0, len(manifests))
	var authProviderCount int
	var demoDataSetCount int
	var migrationCount int
	var seedCount int
	var serviceOperationCount int
	var serviceEventCount int
	for _, manifest := range manifests {
		authProviderCount += len(manifest.AuthProviders)
		demoDataSetCount += len(manifest.DemoData)
		migrationCount += len(manifest.Migrations)
		seedCount += len(manifest.Seeds)
		serviceOperationCount += len(manifest.Service.Operations)
		serviceEventCount += len(manifest.Service.Events)
		capabilities = append(capabilities, platformAuditCapability{
			ID:                string(manifest.ID),
			Name:              manifest.Name,
			Version:           manifest.Version,
			Dependencies:      capabilityIDs(manifest.Dependencies),
			AdminResources:    adminResourceNames(manifest.Admin.Resources),
			AppRoutes:         appRouteNames(manifest.App.Routes),
			AppRouteHandlers:  appRouteHandlersForCapability(manifest.App.Routes, handlerCoverage.CoveredRoutes),
			AuthProviders:     authProviderIDs(manifest.AuthProviders),
			DemoDataSets:      demoDataSetIDs(manifest.DemoData),
			Migrations:        migrationIDs(manifest.Migrations),
			Seeds:             seedIDs(manifest.Seeds),
			ServiceOperations: serviceOperationIDs(manifest.Service.Operations),
			ServiceEvents:     serviceEventIDs(manifest.Service.Events),
		})
	}

	return platformAuditDocument{
		GeneratedBy:             "cmd/platform-contracts audit",
		Source:                  "capability.Manifest",
		SourceMode:              "go-manifest",
		SourceVersion:           sourceVersion(manifests),
		Status:                  "pass",
		CapabilityCount:         len(manifests),
		ResourceCount:           adminDocument.ResourceCount,
		RouteCount:              appDocument.RouteCount,
		AppRouteHandlerCount:    handlerCoverage.CoveredCount,
		MissingAppRouteHandlers: handlerCoverage.MissingRoutes,
		AdminPermissionCount:    len(adminDocument.Permissions),
		AppPermissionCount:      len(appDocument.Permissions),
		AuthProviderCount:       authProviderCount,
		DemoDataSetCount:        demoDataSetCount,
		MigrationCount:          migrationCount,
		SeedCount:               seedCount,
		ServiceCount:            len(serviceDocument.Services),
		ServiceOperationCount:   serviceOperationCount,
		ServiceEventCount:       serviceEventCount,
		Capabilities:            capabilities,
	}, nil
}

func serviceOperationIDs(operations []capability.ServiceOperation) []string {
	ids := make([]string, 0, len(operations))
	for _, operation := range operations {
		ids = append(ids, operation.ID)
	}
	sort.Strings(ids)
	return ids
}

func serviceEventIDs(events []capability.ServiceEvent) []string {
	ids := make([]string, 0, len(events))
	for _, event := range events {
		ids = append(ids, event.ID)
	}
	sort.Strings(ids)
	return ids
}

func adminResourcePermissions(resource capability.AdminResource) map[string]string {
	prefix := resource.PermissionPrefix
	permissions := map[string]string{
		"read":   prefix + ":read",
		"create": prefix + ":create",
		"update": prefix + ":update",
	}
	if resource.Deletion == nil {
		return permissions
	}
	switch resource.Deletion.Mode {
	case capability.AdminDeletionDisabled, capability.AdminDeletionAppendOnly:
	case capability.AdminDeletionSoftDelete, capability.AdminDeletionTombstone:
		permissions["delete"] = prefix + ":delete"
		permissions["restore"] = prefix + ":restore"
		permissions["purge"] = prefix + ":purge"
	case capability.AdminDeletionRevoke:
		permissions["delete"] = prefix + ":delete"
		if resource.Deletion.RetentionDays > 0 {
			permissions["purge"] = prefix + ":purge"
		}
	default:
		permissions["delete"] = prefix + ":delete"
	}
	return permissions
}

func adminMenuFromCapability(menu capability.AdminMenu) adminMenuContract {
	return adminMenuContract{
		Route:    menu.Route,
		Parent:   menu.Parent,
		Group:    menu.Group,
		Icon:     menu.Icon,
		Order:    menu.Order,
		External: menu.External,
		Cache:    menu.Cache,
	}
}

func adminFormGroupsFromCapability(groups []capability.AdminFormGroup) []adminFormGroupContract {
	contracts := make([]adminFormGroupContract, 0, len(groups))
	for _, group := range groups {
		contracts = append(contracts, adminFormGroupContract{
			Key:         group.Key,
			Label:       group.Label,
			Description: group.Description,
		})
	}
	return contracts
}

func adminFieldsFromCapability(fields []capability.AdminField) []adminFieldContract {
	contracts := make([]adminFieldContract, 0, len(fields))
	for _, field := range fields {
		contracts = append(contracts, adminFieldContract{
			Key:          field.Key,
			Label:        field.Label,
			Type:         field.Type,
			Source:       field.Source,
			Group:        field.Group,
			Help:         field.Help,
			Required:     field.Required,
			ReadOnly:     field.ReadOnly,
			Searchable:   field.Searchable,
			Filterable:   field.Filterable,
			Sortable:     field.Sortable,
			Localizable:  field.Localizable,
			InTable:      field.InTable,
			InForm:       field.InForm,
			InDetail:     field.InDetail,
			Width:        field.Width,
			Options:      adminFieldOptionsFromCapability(field.Options),
			Relation:     adminFieldRelationFromCapability(field.Relation),
			Validation:   adminFieldValidationFromCapability(field.Validation),
			Sensitivity:  defaultCapabilityFieldPolicy(field.Sensitivity, capability.FieldSensitivityPublic),
			StorageMode:  defaultCapabilityFieldPolicy(field.StorageMode, capability.FieldStoragePlain),
			ResponseMode: defaultCapabilityFieldPolicy(field.ResponseMode, capability.FieldProjectionFull),
			ExportMode:   defaultCapabilityFieldPolicy(field.ExportMode, capability.FieldProjectionFull),
			Protection:   field.Protection,
		})
	}
	return contracts
}

func defaultCapabilityFieldPolicy(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func adminFieldOptionsFromCapability(options []capability.AdminFieldOption) []adminFieldOptionContract {
	contracts := make([]adminFieldOptionContract, 0, len(options))
	for _, option := range options {
		contracts = append(contracts, adminFieldOptionContract{
			Value: option.Value,
			Label: option.Label,
		})
	}
	return contracts
}

func adminFieldRelationFromCapability(relation *capability.AdminFieldRelation) *adminFieldRelationContract {
	if relation == nil {
		return nil
	}
	return &adminFieldRelationContract{
		Resource:    relation.Resource,
		ValueField:  relation.ValueField,
		LabelField:  relation.LabelField,
		Multiple:    relation.Multiple,
		Filters:     adminFieldRelationFiltersFromCapability(relation.Filters),
		SortField:   relation.SortField,
		SortOrder:   relation.SortOrder,
		Display:     relation.Display,
		ParentField: relation.ParentField,
		PathField:   relation.PathField,
		RootValue:   relation.RootValue,
	}
}

func adminFieldRelationFiltersFromCapability(filters []capability.AdminFieldRelationFilter) []adminFieldRelationFilterContract {
	contracts := make([]adminFieldRelationFilterContract, 0, len(filters))
	for _, filter := range filters {
		contracts = append(contracts, adminFieldRelationFilterContract{
			Field:    filter.Field,
			Operator: filter.Operator,
			Value:    filter.Value,
		})
	}
	return contracts
}

func adminFieldValidationFromCapability(validation capability.AdminFieldValidation) adminFieldValidationContract {
	return adminFieldValidationContract{
		MinLength: validation.MinLength,
		MaxLength: validation.MaxLength,
		Min:       validation.Min,
		Max:       validation.Max,
		Pattern:   validation.Pattern,
	}
}

func sourceVersion(manifests []capability.Manifest) string {
	versions := uniqueStringsFromManifests(manifests, func(manifest capability.Manifest) string { return manifest.Version })
	if len(versions) == 1 {
		return versions[0]
	}
	if len(versions) == 0 {
		return ""
	}
	return "mixed"
}

func uniqueStringsFromManifests(manifests []capability.Manifest, value func(capability.Manifest) string) []string {
	items := make([]string, 0, len(manifests))
	for _, manifest := range manifests {
		items = append(items, value(manifest))
	}
	return uniqueSortedStrings(items)
}

func uniqueStringsFromRoutes(routes []appRouteContractEntry, value func(appRouteContractEntry) string) []string {
	items := make([]string, 0, len(routes))
	for _, route := range routes {
		items = append(items, value(route))
	}
	return uniqueSortedStrings(items)
}

func uniqueStringsFromAdminResources(resources []adminResourceContractEntry, value func(adminResourceContractEntry) string) []string {
	items := make([]string, 0, len(resources))
	for _, resource := range resources {
		items = append(items, value(resource))
	}
	return uniqueSortedStrings(items)
}

func uniqueAdminResourcePermissions(resources []adminResourceContractEntry) []string {
	items := make([]string, 0, len(resources)*4)
	for _, resource := range resources {
		for _, permission := range resource.Permissions {
			items = append(items, permission)
		}
	}
	return uniqueSortedStrings(items)
}

func capabilityIDs(ids []capability.ID) []string {
	items := make([]string, 0, len(ids))
	for _, id := range ids {
		items = append(items, string(id))
	}
	return uniqueSortedStrings(items)
}

func adminResourceNames(resources []capability.AdminResource) []string {
	items := make([]string, 0, len(resources))
	for _, resource := range resources {
		items = append(items, resource.Resource)
	}
	return uniqueSortedStrings(items)
}

func appRouteNames(routes []capability.AppRoute) []string {
	items := make([]string, 0, len(routes))
	for _, route := range routes {
		items = append(items, strings.ToUpper(route.Method)+" "+route.Path)
	}
	return uniqueSortedStrings(items)
}

func appRouteHandlersForCapability(routes []capability.AppRoute, coveredRoutes []string) []string {
	covered := map[string]struct{}{}
	for _, route := range coveredRoutes {
		covered[route] = struct{}{}
	}
	items := make([]string, 0, len(routes))
	for _, route := range appRouteNames(routes) {
		if _, ok := covered[route]; ok {
			items = append(items, route)
		}
	}
	return uniqueSortedStrings(items)
}

func authProviderIDs(providers []capability.AuthProvider) []string {
	items := make([]string, 0, len(providers))
	for _, provider := range providers {
		items = append(items, provider.ID)
	}
	return uniqueSortedStrings(items)
}

func demoDataSetIDs(datasets []capability.DemoDataSet) []string {
	items := make([]string, 0, len(datasets))
	for _, dataset := range datasets {
		items = append(items, dataset.ID)
	}
	return uniqueSortedStrings(items)
}

func migrationIDs(migrations []capability.Migration) []string {
	items := make([]string, 0, len(migrations))
	for _, migration := range migrations {
		items = append(items, migration.ID)
	}
	return uniqueSortedStrings(items)
}

func seedIDs(seeds []capability.Seed) []string {
	items := make([]string, 0, len(seeds))
	for _, seed := range seeds {
		items = append(items, seed.ID)
	}
	return uniqueSortedStrings(items)
}

func uniqueSortedStrings(items []string) []string {
	seen := map[string]struct{}{}
	for _, item := range items {
		if item == "" {
			continue
		}
		seen[item] = struct{}{}
	}
	values := make([]string, 0, len(seen))
	for item := range seen {
		values = append(values, item)
	}
	sort.Strings(values)
	return values
}
