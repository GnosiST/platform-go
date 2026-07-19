package httpapi

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/GnosiST/platform-go/internal/platform/adminresource"
	"github.com/GnosiST/platform-go/internal/platform/capability"
	"github.com/GnosiST/platform-go/internal/platform/dataprotection"
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
	CapabilityID           string                   `json:"capabilityId"`
	CapabilityName         string                   `json:"capabilityName"`
	CapabilityVersion      string                   `json:"capabilityVersion,omitempty"`
	Resource               string                   `json:"resource"`
	Title                  capability.LocalizedText `json:"title"`
	Description            capability.LocalizedText `json:"description"`
	Route                  string                   `json:"route,omitempty"`
	Group                  string                   `json:"group,omitempty"`
	PermissionPrefix       string                   `json:"permissionPrefix"`
	ReadOnly               bool                     `json:"readOnly,omitempty"`
	Writable               bool                     `json:"writable"`
	RuntimeApplyMode       string                   `json:"runtimeApplyMode"`
	RestartRequired        bool                     `json:"restartRequired"`
	PendingRestart         bool                     `json:"pendingRestart"`
	ValidationEndpoint     string                   `json:"validationEndpoint,omitempty"`
	TestConnectionEndpoint string                   `json:"testConnectionEndpoint,omitempty"`
	Schema                 adminresource.Schema     `json:"schema"`
	RecordCount            int                      `json:"recordCount"`
	Records                []adminresource.Record   `json:"records"`
}

type adminSettingsMutationResponse struct {
	Resource        string               `json:"resource"`
	Record          adminresource.Record `json:"record"`
	RestartRequired bool                 `json:"restartRequired"`
	PendingRestart  bool                 `json:"pendingRestart"`
}

type adminSettingsValidationResponse struct {
	Resource        string                     `json:"resource"`
	ID              string                     `json:"id"`
	Status          string                     `json:"status"`
	Valid           bool                       `json:"valid"`
	RestartRequired bool                       `json:"restartRequired"`
	PendingRestart  bool                       `json:"pendingRestart"`
	Checks          []adminSettingsConfigCheck `json:"checks"`
}

type adminSettingsTestConnectionResponse struct {
	Resource        string                     `json:"resource"`
	ID              string                     `json:"id"`
	Status          string                     `json:"status"`
	Supported       bool                       `json:"supported"`
	Connected       bool                       `json:"connected"`
	Mode            string                     `json:"mode"`
	RestartRequired bool                       `json:"restartRequired"`
	PendingRestart  bool                       `json:"pendingRestart"`
	Checks          []adminSettingsConfigCheck `json:"checks"`
}

type adminSettingsConfigCheck struct {
	Key     string `json:"key"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type adminSettingsUpdateRequest struct {
	Code        *string                    `json:"code,omitempty"`
	Name        *string                    `json:"name,omitempty"`
	Status      *string                    `json:"status,omitempty"`
	Description *string                    `json:"description,omitempty"`
	Values      map[string]json.RawMessage `json:"values,omitempty"`
}

const (
	adminSettingsApplyModeDynamic         = "dynamic"
	adminSettingsApplyModeRestartRequired = "restart-required"
	adminSettingsCheckOK                  = "ok"
	adminSettingsCheckWarning             = "warning"
	adminSettingsCheckInvalid             = "invalid"
	adminSettingsStatusValid              = "valid"
	adminSettingsStatusInvalid            = "invalid"
	adminSettingsStatusUnsupported        = "unsupported"
	adminSettingsStatusDryRun             = "dry-run"
)

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
		restartRequired := settingsResourceRestartRequired(contribution.manifest, contribution.resource)
		pendingRestart := s.settingsResourcePendingRestart(contribution.resource.Resource, projected, restartRequired)
		capabilityIDs[string(contribution.manifest.ID)] = struct{}{}
		recordCount += len(projected)
		items = append(items, adminSettingsResourceItem{
			CapabilityID:           string(contribution.manifest.ID),
			CapabilityName:         contribution.manifest.Name,
			CapabilityVersion:      contribution.manifest.Version,
			Resource:               contribution.resource.Resource,
			Title:                  contribution.resource.Title,
			Description:            contribution.resource.Description,
			Route:                  contribution.resource.Menu.Route,
			Group:                  contribution.resource.Menu.Group,
			PermissionPrefix:       contribution.resource.PermissionPrefix,
			ReadOnly:               contribution.resource.ReadOnly,
			Writable:               !contribution.resource.ReadOnly && strings.TrimSpace(schema.Permissions.Update) != "",
			RuntimeApplyMode:       settingsRuntimeApplyMode(restartRequired),
			RestartRequired:        restartRequired,
			PendingRestart:         pendingRestart,
			ValidationEndpoint:     settingsValidationEndpoint(contribution.resource.Resource),
			TestConnectionEndpoint: settingsTestConnectionEndpoint(contribution.resource.Resource),
			Schema:                 schema,
			RecordCount:            len(projected),
			Records:                projected,
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
	restartRequired := settingsResourceRestartRequired(contribution.manifest, contribution.resource)
	if restartRequired {
		s.markSettingsPendingRestart(resource, mutation.Record.ID)
	}
	projected, err := s.resources.ProjectRecord(resource, mutation.Record, adminresource.ProjectionResponse)
	if err != nil {
		writeAdminResourceError(ctx, s.internalErrorSink, err)
		return
	}
	projected = scrubSettingsProjectedRecords(schema, []adminresource.Record{projected})[0]
	s.invalidateCachesForResource(ctx.Request.Context(), resource)
	ctx.JSON(http.StatusOK, Response[adminSettingsMutationResponse]{
		Data: adminSettingsMutationResponse{
			Resource:        resource,
			Record:          projected,
			RestartRequired: restartRequired,
			PendingRestart:  s.settingsRecordPendingRestart(resource, projected.ID, projected, restartRequired),
		},
	})
}

func (s *Server) adminSettingsValidateConfig(ctx *gin.Context) {
	target, ok := s.settingsActionTarget(ctx)
	if !ok {
		return
	}
	checks := validateSettingsConfigChecks(target.schema, target.record)
	status := adminSettingsStatusValid
	if settingsChecksHaveInvalid(checks) {
		status = adminSettingsStatusInvalid
	}
	ctx.JSON(http.StatusOK, Response[adminSettingsValidationResponse]{
		Data: adminSettingsValidationResponse{
			Resource:        target.resource,
			ID:              target.record.ID,
			Status:          status,
			Valid:           status == adminSettingsStatusValid,
			RestartRequired: target.restartRequired,
			PendingRestart:  s.settingsRecordPendingRestart(target.resource, target.record.ID, target.record, target.restartRequired),
			Checks:          checks,
		},
	})
}

func (s *Server) adminSettingsTestConnection(ctx *gin.Context) {
	target, ok := s.settingsActionTarget(ctx)
	if !ok {
		return
	}
	checks := validateSettingsConfigChecks(target.schema, target.record)
	response := adminSettingsTestConnectionResponse{
		Resource:        target.resource,
		ID:              target.record.ID,
		Status:          adminSettingsStatusUnsupported,
		Supported:       false,
		Connected:       false,
		Mode:            "preflight",
		RestartRequired: target.restartRequired,
		PendingRestart:  s.settingsRecordPendingRestart(target.resource, target.record.ID, target.record, target.restartRequired),
		Checks:          checks,
	}
	if settingsChecksHaveInvalid(checks) {
		response.Status = adminSettingsStatusInvalid
		ctx.JSON(http.StatusOK, Response[adminSettingsTestConnectionResponse]{Data: response})
		return
	}
	if target.resource != "notification-providers" {
		response.Checks = append(response.Checks, adminSettingsConfigCheck{Key: "adapter", Status: adminSettingsCheckWarning, Message: "This configuration resource does not expose a runtime connection test."})
		ctx.JSON(http.StatusOK, Response[adminSettingsTestConnectionResponse]{Data: response})
		return
	}
	channel := strings.TrimSpace(target.record.Values["channel"])
	provider := strings.TrimSpace(target.record.Values["provider"])
	if channel != "sms" {
		response.Checks = append(response.Checks, adminSettingsConfigCheck{Key: "adapter", Status: adminSettingsCheckWarning, Message: "Only SMS provider connection preflight is available in v1.1; email and WeChat adapters remain configuration contracts."})
		ctx.JSON(http.StatusOK, Response[adminSettingsTestConnectionResponse]{Data: response})
		return
	}
	switch provider {
	case "aliyun", "tencent", "mock-local":
		response.Status = adminSettingsStatusDryRun
		response.Supported = true
		response.Connected = true
		response.Mode = "dry-run"
		response.Checks = append(response.Checks, adminSettingsConfigCheck{Key: "adapter", Status: adminSettingsCheckOK, Message: "SMS provider configuration passed local adapter preflight without contacting the external vendor."})
	default:
		response.Checks = append(response.Checks, adminSettingsConfigCheck{Key: "adapter", Status: adminSettingsCheckWarning, Message: "The selected notification provider does not have a v1.1 runtime adapter preflight."})
	}
	ctx.JSON(http.StatusOK, Response[adminSettingsTestConnectionResponse]{Data: response})
}

type settingsActionTarget struct {
	resource        string
	record          adminresource.Record
	schema          adminresource.Schema
	restartRequired bool
}

func (s *Server) settingsActionTarget(ctx *gin.Context) (settingsActionTarget, bool) {
	resource := strings.TrimSpace(ctx.Param("resource"))
	id := strings.TrimSpace(ctx.Param("id"))
	contribution, ok := s.settingsResourceContribution(resource)
	if !ok {
		writeAdminResourceError(ctx, s.internalErrorSink, adminresource.ErrUnknownResource)
		return settingsActionTarget{}, false
	}
	if !s.authorize(ctx, "admin:settings:update") {
		return settingsActionTarget{}, false
	}
	if !s.authorizeAdminResource(ctx, resource, "update") {
		return settingsActionTarget{}, false
	}
	record, err := s.resources.InternalRecord(resource, id)
	if err != nil {
		writeAdminResourceError(ctx, s.internalErrorSink, err)
		return settingsActionTarget{}, false
	}
	schema, err := s.resources.Schema(resource)
	if err != nil {
		writeAdminResourceError(ctx, s.internalErrorSink, err)
		return settingsActionTarget{}, false
	}
	return settingsActionTarget{
		resource:        resource,
		record:          record,
		schema:          schema,
		restartRequired: settingsResourceRestartRequired(contribution.manifest, contribution.resource),
	}, true
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
	if !field.InForm || field.ReadOnly {
		return false
	}
	if sensitivity == capability.FieldSensitivityPublic && storageMode == capability.FieldStoragePlain {
		return true
	}
	return settingsFieldWriteOnlySecret(field, sensitivity, storageMode)
}

func settingsFieldWriteOnlySecret(field adminresource.FieldDefinition, sensitivity string, storageMode string) bool {
	responseMode := strings.TrimSpace(field.ResponseMode)
	if responseMode == "" {
		responseMode = capability.FieldProjectionFull
	}
	return sensitivity == capability.FieldSensitivitySecret &&
		storageMode == capability.FieldStorageEncrypted &&
		responseMode == capability.FieldProjectionOmitted &&
		field.Protection != nil
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

func settingsRuntimeApplyMode(restartRequired bool) string {
	if restartRequired {
		return adminSettingsApplyModeRestartRequired
	}
	return adminSettingsApplyModeDynamic
}

func settingsResourceRestartRequired(manifest capability.Manifest, resource capability.AdminResource) bool {
	switch manifest.ID {
	case "credential-auth", "notification":
		return true
	}
	switch resource.Resource {
	case "notification-channels", "notification-providers", "notification-send-policies", "notification-templates", "credential-auth-settings":
		return true
	default:
		return false
	}
}

func settingsValidationEndpoint(resource string) string {
	if strings.TrimSpace(resource) == "" {
		return ""
	}
	return "/api/admin/settings/" + resource + "/:id/validate-config"
}

func settingsTestConnectionEndpoint(resource string) string {
	if resource != "notification-providers" {
		return ""
	}
	return "/api/admin/settings/" + resource + "/:id/test-connect"
}

func (s *Server) markSettingsPendingRestart(resource string, id string) {
	resource = strings.TrimSpace(resource)
	id = strings.TrimSpace(id)
	if resource == "" || id == "" {
		return
	}
	s.settingsMu.Lock()
	defer s.settingsMu.Unlock()
	if s.settingsPendingRestart == nil {
		s.settingsPendingRestart = map[string]map[string]struct{}{}
	}
	if s.settingsPendingRestart[resource] == nil {
		s.settingsPendingRestart[resource] = map[string]struct{}{}
	}
	s.settingsPendingRestart[resource][id] = struct{}{}
}

func (s *Server) settingsResourcePendingRestart(resource string, records []adminresource.Record, restartRequired bool) bool {
	if !restartRequired {
		return false
	}
	for _, record := range records {
		if s.settingsRecordPendingRestart(resource, record.ID, record, restartRequired) {
			return true
		}
	}
	return false
}

func (s *Server) settingsRecordPendingRestart(resource string, id string, record adminresource.Record, restartRequired bool) bool {
	if !restartRequired {
		return false
	}
	resource = strings.TrimSpace(resource)
	id = strings.TrimSpace(id)
	s.settingsMu.Lock()
	_, pending := s.settingsPendingRestart[resource][id]
	s.settingsMu.Unlock()
	if pending {
		return true
	}
	updatedAt, err := time.Parse(time.RFC3339, strings.TrimSpace(record.UpdatedAt))
	if err != nil {
		return false
	}
	return updatedAt.After(s.startedAt)
}

func validateSettingsConfigChecks(schema adminresource.Schema, record adminresource.Record) []adminSettingsConfigCheck {
	checks := make([]adminSettingsConfigCheck, 0)
	for _, rawField := range schema.Fields {
		field := rawField
		if !field.Required {
			continue
		}
		value := strings.TrimSpace(settingsRecordFieldValue(record, field))
		if value == "" {
			checks = append(checks, adminSettingsConfigCheck{Key: field.Key, Status: adminSettingsCheckInvalid, Message: "Required configuration value is missing."})
		}
	}
	for _, rawField := range schema.Fields {
		field := rawField
		if field.Source != "values" || field.StorageMode != capability.FieldStorageEncrypted {
			continue
		}
		value := strings.TrimSpace(record.Values[field.Key])
		if value == "" {
			continue
		}
		if !dataprotection.IsEnvelope(value) {
			checks = append(checks, adminSettingsConfigCheck{Key: field.Key, Status: adminSettingsCheckInvalid, Message: "Encrypted secret is not stored as a protected envelope."})
			continue
		}
		checks = append(checks, adminSettingsConfigCheck{Key: field.Key, Status: adminSettingsCheckOK, Message: "Secret is stored as a protected envelope and is not returned by settings runtime responses."})
	}
	if schema.Resource == "notification-providers" {
		checks = append(checks, validateNotificationProviderSettingsChecks(record)...)
	}
	if schema.Resource == "credential-auth-settings" {
		checks = append(checks, validateCredentialAuthSettingsChecks(record)...)
	}
	if len(checks) == 0 {
		checks = append(checks, adminSettingsConfigCheck{Key: "schema", Status: adminSettingsCheckOK, Message: "Configuration record satisfies the active schema contract."})
	}
	return checks
}

func settingsRecordFieldValue(record adminresource.Record, field adminresource.FieldDefinition) string {
	if field.Source == "values" {
		return record.Values[field.Key]
	}
	switch field.Key {
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
		return ""
	}
}

func validateNotificationProviderSettingsChecks(record adminresource.Record) []adminSettingsConfigCheck {
	provider := strings.TrimSpace(record.Values["provider"])
	channel := strings.TrimSpace(record.Values["channel"])
	checks := make([]adminSettingsConfigCheck, 0, 3)
	switch channel {
	case "sms", "email", "wechat_official", "wechat_miniapp", "in_app":
		checks = append(checks, adminSettingsConfigCheck{Key: "channel", Status: adminSettingsCheckOK, Message: "Notification channel is recognized by the platform contract."})
	default:
		checks = append(checks, adminSettingsConfigCheck{Key: "channel", Status: adminSettingsCheckInvalid, Message: "Notification channel is not recognized by the platform contract."})
	}
	switch provider {
	case "local", "aliyun", "tencent", "mock-local", "smtp", "wechat-official", "wechat-miniapp":
		checks = append(checks, adminSettingsConfigCheck{Key: "provider", Status: adminSettingsCheckOK, Message: "Notification provider is recognized by the platform contract."})
	default:
		checks = append(checks, adminSettingsConfigCheck{Key: "provider", Status: adminSettingsCheckInvalid, Message: "Notification provider is not recognized by the platform contract."})
	}
	if channel == "sms" && (provider == "aliyun" || provider == "tencent") {
		if strings.TrimSpace(record.Values["accessKey"]) == "" || strings.TrimSpace(record.Values["accessSecret"]) == "" {
			checks = append(checks, adminSettingsConfigCheck{Key: "credentials", Status: adminSettingsCheckInvalid, Message: "Aliyun and Tencent SMS providers require accessKey and accessSecret before production use."})
		}
	}
	if channel == "email" || provider == "smtp" {
		checks = append(checks, adminSettingsConfigCheck{Key: "adapter", Status: adminSettingsCheckWarning, Message: "SMTP is a v1.1 configuration contract; a real sending adapter remains a follow-up runtime slice."})
	}
	if strings.HasPrefix(channel, "wechat") || strings.HasPrefix(provider, "wechat") {
		checks = append(checks, adminSettingsConfigCheck{Key: "adapter", Status: adminSettingsCheckWarning, Message: "WeChat message providers are v1.1 configuration contracts; real sending adapters remain follow-up runtime slices."})
	}
	return checks
}

func validateCredentialAuthSettingsChecks(record adminresource.Record) []adminSettingsConfigCheck {
	checks := make([]adminSettingsConfigCheck, 0, 4)
	if strings.TrimSpace(record.Values["challengeMode"]) == "risk-based" {
		checks = append(checks, adminSettingsConfigCheck{Key: "challengeMode", Status: adminSettingsCheckWarning, Message: "Risk-based challenge mode requires a future risk signal adapter before it can be stronger than after-failure mode."})
	}
	if strings.TrimSpace(record.Values["secretTransport"]) != "ecdh-a256gcm-v1" {
		checks = append(checks, adminSettingsConfigCheck{Key: "secretTransport", Status: adminSettingsCheckInvalid, Message: "Credential-auth v1 requires ecdh-a256gcm-v1 application-layer secret transport."})
	}
	if strings.TrimSpace(record.Values["passwordAlgorithm"]) != "argon2id" {
		checks = append(checks, adminSettingsConfigCheck{Key: "passwordAlgorithm", Status: adminSettingsCheckInvalid, Message: "Credential-auth v1 requires argon2id password credentials."})
	}
	return checks
}

func settingsChecksHaveInvalid(checks []adminSettingsConfigCheck) bool {
	for _, check := range checks {
		if check.Status == adminSettingsCheckInvalid {
			return true
		}
	}
	return false
}
