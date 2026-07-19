package httpapi

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/GnosiST/platform-go/internal/platform/adminresource"
	"github.com/GnosiST/platform-go/internal/platform/capability"
)

type adminSettingsRuntimeResponse struct {
	Items   []adminSettingsResourceItem `json:"items"`
	Metrics adminSettingsMetrics        `json:"metrics"`
}

type adminSettingsMetrics struct {
	Capabilities int `json:"capabilities"`
	Resources    int `json:"resources"`
	Records      int `json:"records"`
}

type adminSettingsResourceItem struct {
	CapabilityID      string                   `json:"capabilityId"`
	CapabilityName    string                   `json:"capabilityName"`
	CapabilityVersion string                   `json:"capabilityVersion,omitempty"`
	Resource          string                   `json:"resource"`
	Title             capability.LocalizedText `json:"title"`
	Description       capability.LocalizedText `json:"description"`
	Route             string                   `json:"route,omitempty"`
	Group             string                   `json:"group,omitempty"`
	PermissionPrefix  string                   `json:"permissionPrefix"`
	ReadOnly          bool                     `json:"readOnly,omitempty"`
	Writable          bool                     `json:"writable"`
	Schema            adminresource.Schema     `json:"schema"`
	RecordCount       int                      `json:"recordCount"`
	Records           []adminresource.Record   `json:"records"`
}

type adminSettingsMutationResponse struct {
	Resource string               `json:"resource"`
	Record   adminresource.Record `json:"record"`
}

type adminSettingsUpdateRequest struct {
	Code        *string                    `json:"code,omitempty"`
	Name        *string                    `json:"name,omitempty"`
	Status      *string                    `json:"status,omitempty"`
	Description *string                    `json:"description,omitempty"`
	Values      map[string]json.RawMessage `json:"values,omitempty"`
}

func (s *Server) adminSettingsRuntime(ctx *gin.Context) {
	if !s.authorize(ctx, "admin:settings:read") {
		return
	}
	items := make([]adminSettingsResourceItem, 0)
	capabilityIDs := map[string]struct{}{}
	recordCount := 0
	for _, contribution := range s.settingsResourceContributions() {
		schema, err := s.resources.Schema(contribution.resource.Resource)
		if err != nil {
			writeAdminResourceError(ctx, s.internalErrorSink, err)
			return
		}
		records, err := s.resources.List(contribution.resource.Resource)
		if err != nil {
			writeAdminResourceError(ctx, s.internalErrorSink, err)
			return
		}
		projected, err := s.projectAdminResourceRecords(contribution.resource.Resource, records, adminresource.ProjectionResponse)
		if err != nil {
			writeAdminResourceError(ctx, s.internalErrorSink, err)
			return
		}
		projected = scrubSettingsProjectedRecords(schema, projected)
		capabilityIDs[string(contribution.manifest.ID)] = struct{}{}
		recordCount += len(projected)
		items = append(items, adminSettingsResourceItem{
			CapabilityID:      string(contribution.manifest.ID),
			CapabilityName:    contribution.manifest.Name,
			CapabilityVersion: contribution.manifest.Version,
			Resource:          contribution.resource.Resource,
			Title:             contribution.resource.Title,
			Description:       contribution.resource.Description,
			Route:             contribution.resource.Menu.Route,
			Group:             contribution.resource.Menu.Group,
			PermissionPrefix:  contribution.resource.PermissionPrefix,
			ReadOnly:          contribution.resource.ReadOnly,
			Writable:          !contribution.resource.ReadOnly && strings.TrimSpace(schema.Permissions.Update) != "",
			Schema:            schema,
			RecordCount:       len(projected),
			Records:           projected,
		})
	}
	ctx.JSON(http.StatusOK, Response[adminSettingsRuntimeResponse]{
		Data: adminSettingsRuntimeResponse{
			Items: items,
			Metrics: adminSettingsMetrics{
				Capabilities: len(capabilityIDs),
				Resources:    len(items),
				Records:      recordCount,
			},
		},
	})
}

func (s *Server) adminSettingsUpdate(ctx *gin.Context) {
	resource := strings.TrimSpace(ctx.Param("resource"))
	id := strings.TrimSpace(ctx.Param("id"))
	contribution, ok := s.settingsResourceContribution(resource)
	if !ok || contribution.resource.ReadOnly {
		writeAdminResourceError(ctx, s.internalErrorSink, adminresource.ErrUnknownResource)
		return
	}
	if !s.authorize(ctx, "admin:settings:update") {
		return
	}
	if !s.authorizeAdminResource(ctx, resource, "update") {
		return
	}
	existing, err := s.resources.InternalRecord(resource, id)
	if err != nil {
		writeAdminResourceError(ctx, s.internalErrorSink, err)
		return
	}
	var input adminSettingsUpdateRequest
	decoder := json.NewDecoder(ctx.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeAdminResourceError(ctx, s.internalErrorSink, adminresource.ErrInvalidRecord)
		return
	}
	schema, err := s.resources.Schema(resource)
	if err != nil {
		writeAdminResourceError(ctx, s.internalErrorSink, err)
		return
	}
	if err := validateSettingsUpdateRecordFields(schema, input); err != nil {
		writeAdminResourceError(ctx, s.internalErrorSink, err)
		return
	}
	writeInput, err := s.settingsWriteInput(resource, existing, input)
	if err != nil {
		writeAdminResourceError(ctx, s.internalErrorSink, err)
		return
	}
	mutation, err := s.resources.UpdateInternalWithAudit(resource, id, writeInput, s.mutationAuditEvent(ctx, "settings.update", resource, "updated"))
	if err != nil {
		writeAdminResourceError(ctx, s.internalErrorSink, err)
		return
	}
	projected, err := s.resources.ProjectRecord(resource, mutation.Record, adminresource.ProjectionResponse)
	if err != nil {
		writeAdminResourceError(ctx, s.internalErrorSink, err)
		return
	}
	s.invalidateCachesForResource(ctx.Request.Context(), resource)
	ctx.JSON(http.StatusOK, Response[adminSettingsMutationResponse]{
		Data: adminSettingsMutationResponse{Resource: resource, Record: projected},
	})
}

func (s *Server) settingsWriteInput(resource string, existing adminresource.Record, input adminSettingsUpdateRequest) (adminresource.WriteInput, error) {
	schema, err := s.resources.Schema(resource)
	if err != nil {
		return adminresource.WriteInput{}, err
	}
	code := existing.Code
	if input.Code != nil {
		code = strings.TrimSpace(*input.Code)
	}
	name := existing.Name
	if input.Name != nil {
		name = strings.TrimSpace(*input.Name)
	}
	status := existing.Status
	if input.Status != nil {
		status = strings.TrimSpace(*input.Status)
	}
	description := existing.Description
	if input.Description != nil {
		description = strings.TrimSpace(*input.Description)
	}
	submittedValues, err := normalizeAdminResourceWriteValues(input.Values)
	if err != nil {
		return adminresource.WriteInput{}, err
	}
	values, err := settingsUpdateValues(schema, existing.Values, submittedValues)
	if err != nil {
		return adminresource.WriteInput{}, err
	}
	return adminresource.WriteInput{
		Code:        code,
		Name:        name,
		Status:      status,
		Description: description,
		Values:      values,
	}, nil
}

func scrubSettingsProjectedRecords(schema adminresource.Schema, records []adminresource.Record) []adminresource.Record {
	visibleFields := map[string]struct{}{}
	for _, field := range schema.Fields {
		if field.Source == "values" && settingsFieldVisible(field) {
			visibleFields[field.Key] = struct{}{}
		}
	}
	for index := range records {
		for key := range records[index].Values {
			if _, ok := visibleFields[key]; !ok {
				delete(records[index].Values, key)
			}
		}
		if len(records[index].Values) == 0 {
			records[index].Values = nil
		}
	}
	return records
}

func settingsUpdateValues(schema adminresource.Schema, existing map[string]string, submitted map[string]string) (map[string]string, error) {
	values := map[string]string{}
	fields := map[string]adminresource.FieldDefinition{}
	for _, field := range schema.Fields {
		if field.Source != "values" {
			continue
		}
		fields[field.Key] = field
		if !settingsFieldWritable(field) {
			continue
		}
		if value, ok := existing[field.Key]; ok {
			values[field.Key] = value
		}
	}
	for key, value := range submitted {
		field, ok := fields[key]
		if !ok || !settingsFieldWritable(field) {
			return nil, adminresource.ErrInvalidRecord
		}
		values[key] = value
	}
	if len(values) == 0 {
		return nil, nil
	}
	return values, nil
}

func settingsFieldWritable(field adminresource.FieldDefinition) bool {
	sensitivity := strings.TrimSpace(field.Sensitivity)
	if sensitivity == "" {
		sensitivity = capability.FieldSensitivityPublic
	}
	storageMode := strings.TrimSpace(field.StorageMode)
	if storageMode == "" {
		storageMode = capability.FieldStoragePlain
	}
	return field.InForm && !field.ReadOnly && sensitivity == capability.FieldSensitivityPublic && storageMode == capability.FieldStoragePlain
}

func settingsFieldVisible(field adminresource.FieldDefinition) bool {
	sensitivity := strings.TrimSpace(field.Sensitivity)
	if sensitivity == "" {
		sensitivity = capability.FieldSensitivityPublic
	}
	storageMode := strings.TrimSpace(field.StorageMode)
	if storageMode == "" {
		storageMode = capability.FieldStoragePlain
	}
	responseMode := strings.TrimSpace(field.ResponseMode)
	if responseMode == "" {
		responseMode = capability.FieldProjectionFull
	}
	return sensitivity == capability.FieldSensitivityPublic &&
		storageMode != capability.FieldStorageEncrypted &&
		storageMode != capability.FieldStorageHashed &&
		responseMode != capability.FieldProjectionOmitted &&
		responseMode != capability.FieldProjectionPrivileged
}

func validateSettingsUpdateRecordFields(schema adminresource.Schema, input adminSettingsUpdateRequest) error {
	recordFields := map[string]adminresource.FieldDefinition{}
	for _, field := range schema.Fields {
		if field.Source == "record" {
			recordFields[field.Key] = field
		}
	}
	for key, provided := range map[string]bool{
		"code": input.Code != nil, "name": input.Name != nil, "status": input.Status != nil, "description": input.Description != nil,
	} {
		if !provided {
			continue
		}
		field, ok := recordFields[key]
		if !ok || !settingsFieldWritable(field) {
			return adminresource.ErrInvalidRecord
		}
	}
	return nil
}

type settingsResourceContribution struct {
	manifest capability.Manifest
	resource capability.AdminResource
}

func (s *Server) settingsResourceContributions() []settingsResourceContribution {
	contributions := make([]settingsResourceContribution, 0)
	for _, manifest := range s.capabilities {
		for _, resource := range manifest.Admin.Resources {
			if strings.TrimSpace(resource.Resource) == "" || !isCapabilityConfigResource(resource) {
				continue
			}
			contributions = append(contributions, settingsResourceContribution{manifest: manifest, resource: resource})
		}
	}
	sort.SliceStable(contributions, func(left, right int) bool {
		leftOrder := contributions[left].resource.Menu.Order
		rightOrder := contributions[right].resource.Menu.Order
		if leftOrder != rightOrder {
			return leftOrder < rightOrder
		}
		return contributions[left].resource.Resource < contributions[right].resource.Resource
	})
	return contributions
}

func (s *Server) settingsResourceContribution(resource string) (settingsResourceContribution, bool) {
	for _, contribution := range s.settingsResourceContributions() {
		if contribution.resource.Resource == resource {
			return contribution, true
		}
	}
	return settingsResourceContribution{}, false
}
