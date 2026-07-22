package httpapi

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"unicode"

	"github.com/gin-gonic/gin"

	"github.com/GnosiST/platform-go/internal/platform/adminresource"
	"github.com/GnosiST/platform-go/internal/platform/credentialauth"
	"github.com/GnosiST/platform-go/internal/platform/errorcode"
	"github.com/GnosiST/platform-go/internal/platform/ratelimit"
	"github.com/GnosiST/platform-go/internal/platform/rbac"
)

const adminProfileUsersResource = "users"

const (
	adminProfilePasswordChangeProvider        = "profile-password-change"
	adminProfilePasswordResetProvider         = "profile-password-reset"
	adminProfilePasswordCurrentSecretType     = "current-password"
	adminProfilePasswordNewSecretType         = "new-password"
	adminProfilePasswordSecretIdentifierType  = "admin-profile"
	adminProfilePasswordParamsVersionFallback = "argon2id-default"
	adminProfileCredentialStatusReady         = "credential-auth-ready"
)

type adminProfileResponse struct {
	Profile adminProfile `json:"profile"`
}

type adminProfile struct {
	ID          string                       `json:"id"`
	Username    string                       `json:"username"`
	Name        string                       `json:"name"`
	Nickname    string                       `json:"nickname"`
	AvatarURL   string                       `json:"avatarUrl"`
	Phone       string                       `json:"phone"`
	MaskedPhone string                       `json:"maskedPhone,omitempty"`
	Email       string                       `json:"email"`
	MaskedEmail string                       `json:"maskedEmail,omitempty"`
	Address     string                       `json:"address"`
	TenantCode  string                       `json:"tenantCode,omitempty"`
	OrgUnitCode string                       `json:"orgUnitCode,omitempty"`
	AreaCode    string                       `json:"areaCode,omitempty"`
	Credentials adminProfileCredentialStatus `json:"credentials"`
}

type adminProfileCredentialStatus struct {
	PasswordChange string `json:"passwordChange"`
	PasswordReset  string `json:"passwordReset"`
	Message        string `json:"message"`
}

type adminProfileUpdateRequest struct {
	AvatarURL *string `json:"avatarUrl,omitempty"`
	Avatar    *string `json:"avatar,omitempty"`
	Name      *string `json:"name,omitempty"`
	Nickname  *string `json:"nickname,omitempty"`
	Phone     *string `json:"phone,omitempty"`
	Email     *string `json:"email,omitempty"`
	Address   *string `json:"address,omitempty"`
}

type adminProfilePasswordChangeRequest struct {
	CurrentSecret credentialAuthSecretRequest `json:"currentSecret"`
	NewSecret     credentialAuthSecretRequest `json:"newSecret"`
}

type adminProfilePasswordResetRequest struct {
	NewSecret credentialAuthSecretRequest `json:"newSecret"`
}

type adminProfilePasswordMutationResponse struct {
	Credentials adminProfileCredentialStatus `json:"credentials"`
	MustChange  bool                         `json:"mustChange"`
}

// RegisterAdminProfileRoutes wires the current-user profile slice without
// requiring changes to the default server route table.
func (s *Server) RegisterAdminProfileRoutes() {
	api := s.router.Group("/api")
	s.registerAdminProfileRoutes(api)
}

func (s *Server) registerAdminProfileRoutes(api *gin.RouterGroup) {
	if s.profileRoutesRegistered {
		return
	}
	s.profileRoutesRegistered = true
	api.GET("/admin/profile/current", s.adminProfileCurrent)
	api.PUT("/admin/profile/current", s.adminProfileUpdateCurrent)
	api.PUT("/admin/profile/:id", s.adminProfileUpdateByID)
	api.POST("/admin/profile/current/password/change", s.adminProfilePasswordChangeCurrent)
	api.POST("/admin/profile/:id/password/reset", s.adminProfilePasswordResetByID)
}

func (s *Server) adminProfileCurrent(ctx *gin.Context) {
	principal, record, ok := s.currentAdminProfileRecord(ctx)
	if !ok {
		return
	}
	profile := s.adminProfileFromRecord(record, principal)
	ctx.JSON(http.StatusOK, Response[adminProfileResponse]{Data: adminProfileResponse{Profile: profile}})
}

func (s *Server) adminProfileUpdateCurrent(ctx *gin.Context) {
	s.updateAdminProfile(ctx, "")
}

func (s *Server) adminProfileUpdateByID(ctx *gin.Context) {
	s.updateAdminProfile(ctx, ctx.Param("id"))
}

func (s *Server) adminProfilePasswordChangeCurrent(ctx *gin.Context) {
	principal, record, ok := s.currentAdminProfileRecord(ctx)
	if !ok {
		return
	}
	runtime, ok := s.adminProfileCredentialAuthRuntime(ctx)
	if !ok {
		return
	}
	if !s.requireCredentialAuthSecureTransport(ctx) {
		return
	}
	if !s.enforceAdminRateLimit(ctx, ratelimit.OperationAdminProfilePassword, rateLimitClientIP(ctx), principal.User.Username, "change") {
		return
	}
	var input adminProfilePasswordChangeRequest
	decoder := json.NewDecoder(ctx.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writePlatformError(ctx, errorcode.CodeRequestBodyInvalid)
		return
	}
	currentSecret, err := s.decryptAdminProfilePasswordSecret(ctx, input.CurrentSecret, adminProfilePasswordChangeProvider, adminProfilePasswordCurrentSecretType)
	if err != nil {
		writeCredentialAuthLoginError(ctx, s.internalErrorSink, err)
		return
	}
	newSecret, err := s.decryptAdminProfilePasswordSecret(ctx, input.NewSecret, adminProfilePasswordChangeProvider, adminProfilePasswordNewSecretType)
	if err != nil || !validAdminProfilePassword(newSecret) {
		writePlatformError(ctx, errorcode.CodeAuthInvalidRequest)
		return
	}
	if _, err := runtime.Service.VerifyPassword(ctx.Request.Context(), credentialauth.PasswordLoginInput{
		Identifier: credentialauth.Identifier{Type: credentialauth.IdentifierTypeUsername, Value: principal.User.Username},
		Secret:     currentSecret,
	}); err != nil {
		_ = s.recordAdminProfilePasswordAudit(ctx, principal.User.ID, "admin_profile.password.change", "failure", "current-password-rejected")
		writeCredentialAuthLoginError(ctx, s.internalErrorSink, err)
		return
	}
	if err := s.putAdminProfilePasswordCredential(ctx, runtime, record, newSecret, false); err != nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAuthProviderResolveFailed, err)
		return
	}
	if err := s.recordAdminProfilePasswordAudit(ctx, record.ID, "admin_profile.password.change", "success", "password-changed"); err != nil {
		writeAdminResourceError(ctx, s.internalErrorSink, err)
		return
	}
	ctx.JSON(http.StatusOK, Response[adminProfilePasswordMutationResponse]{Data: adminProfilePasswordMutationResponse{
		Credentials: s.adminProfileCredentialStatus(principal),
		MustChange:  false,
	}})
}

func (s *Server) adminProfilePasswordResetByID(ctx *gin.Context) {
	principal, ok := s.authorizeAdminBearerSession(ctx, "admin:user:update")
	if !ok {
		return
	}
	runtime, ok := s.adminProfileCredentialAuthRuntime(ctx)
	if !ok {
		return
	}
	if !s.requireCredentialAuthSecureTransport(ctx) {
		return
	}
	targetID := strings.TrimSpace(ctx.Param("id"))
	if targetID == "" || targetID == "current" {
		targetID = principal.User.ID
	}
	if !s.enforceAdminRateLimit(ctx, ratelimit.OperationAdminProfilePassword, rateLimitClientIP(ctx), principal.User.Username, "reset", targetID) {
		return
	}
	target, err := s.resources.InternalRecord(adminProfileUsersResource, targetID)
	if err != nil {
		writeAdminResourceError(ctx, s.internalErrorSink, err)
		return
	}
	var input adminProfilePasswordResetRequest
	decoder := json.NewDecoder(ctx.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writePlatformError(ctx, errorcode.CodeRequestBodyInvalid)
		return
	}
	newSecret, err := s.decryptAdminProfilePasswordSecret(ctx, input.NewSecret, adminProfilePasswordResetProvider, adminProfilePasswordNewSecretType)
	if err != nil || !validAdminProfilePassword(newSecret) {
		writePlatformError(ctx, errorcode.CodeAuthInvalidRequest)
		return
	}
	if err := s.putAdminProfilePasswordCredential(ctx, runtime, target, newSecret, true); err != nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAuthProviderResolveFailed, err)
		return
	}
	if err := s.recordAdminProfilePasswordAudit(ctx, target.ID, "admin_profile.password.reset", "success", "password-reset"); err != nil {
		writeAdminResourceError(ctx, s.internalErrorSink, err)
		return
	}
	ctx.JSON(http.StatusOK, Response[adminProfilePasswordMutationResponse]{Data: adminProfilePasswordMutationResponse{
		Credentials: s.adminProfileCredentialStatus(principal),
		MustChange:  true,
	}})
}

func (s *Server) updateAdminProfile(ctx *gin.Context, targetID string) {
	principal, record, ok := s.currentAdminProfileRecord(ctx)
	if !ok {
		return
	}
	targetID = strings.TrimSpace(targetID)
	if targetID != "" && targetID != "current" && targetID != record.ID {
		writePlatformError(ctx, errorcode.CodeAdminForbidden)
		return
	}
	var input adminProfileUpdateRequest
	decoder := json.NewDecoder(ctx.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeAdminResourceError(ctx, s.internalErrorSink, adminresource.ErrInvalidRecord)
		return
	}
	writeInput, err := s.adminProfileWriteInput(record, input)
	if err != nil {
		writeAdminResourceError(ctx, s.internalErrorSink, err)
		return
	}
	mutation, err := s.resources.UpdateInternalWithAudit(adminProfileUsersResource, record.ID, writeInput, requestAuditEvent(ctx.Request.Context(), adminresource.AuditEvent{
		Actor:      principal.User.ID,
		Action:     "admin_profile.update",
		Resource:   adminProfileUsersResource,
		Result:     "success",
		ReasonCode: "profile-updated",
	}))
	if err != nil {
		writeAdminResourceError(ctx, s.internalErrorSink, err)
		return
	}
	if err := s.syncAdminProfileCredentialIdentifiers(ctx, record, mutation.Record, input); err != nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAuthProviderResolveFailed, err)
		return
	}
	s.invalidateCachesForResource(ctx.Request.Context(), adminProfileUsersResource)
	profile := s.adminProfileFromRecord(mutation.Record, principal)
	ctx.JSON(http.StatusOK, Response[adminProfileResponse]{Data: adminProfileResponse{Profile: profile}})
}

func (s *Server) currentAdminProfileRecord(ctx *gin.Context) (rbac.Principal, adminresource.Record, bool) {
	if !s.refreshAdminResourceState(ctx, errorcode.CodeAdminAuthStateRefreshFailed) {
		return rbac.Principal{}, adminresource.Record{}, false
	}
	authSession, ok := s.authSessionFromBearer(ctx)
	if !ok {
		writePlatformError(ctx, errorcode.CodeAuthUnauthorized)
		return rbac.Principal{}, adminresource.Record{}, false
	}
	principal := s.currentPrincipalForUsername(ctx.Request.Context(), authSession.Username)
	if strings.TrimSpace(principal.User.ID) == "" {
		writePlatformError(ctx, errorcode.CodeAuthUnauthorized)
		return rbac.Principal{}, adminresource.Record{}, false
	}
	record, err := s.resources.InternalRecord(adminProfileUsersResource, principal.User.ID)
	if err != nil {
		writeAdminResourceError(ctx, s.internalErrorSink, err)
		return rbac.Principal{}, adminresource.Record{}, false
	}
	if record.ID != principal.User.ID || record.Code != principal.User.Username {
		writePlatformError(ctx, errorcode.CodeAuthUnauthorized)
		return rbac.Principal{}, adminresource.Record{}, false
	}
	return principal, record, true
}

func (s *Server) adminProfileCredentialAuthRuntime(ctx *gin.Context) (*CredentialAuthRuntime, bool) {
	runtime := s.credentialAuth
	if runtime == nil || runtime.Service == nil || runtime.SecretTransport == nil {
		writePlatformError(ctx, errorcode.CodeAuthProviderNotConfigured)
		return nil, false
	}
	return runtime, true
}

func (s *Server) decryptAdminProfilePasswordSecret(ctx *gin.Context, input credentialAuthSecretRequest, provider string, expectedSecretType string) (string, error) {
	secretType := strings.TrimSpace(input.Type)
	if secretType == "" {
		secretType = expectedSecretType
	}
	if secretType != expectedSecretType {
		return "", credentialauth.ErrInvalidSecret
	}
	if s.credentialAuth != nil && s.credentialAuth.RequireEncryptedSecrets {
		if credentialAuthPlaintextSecretPresent(input) {
			return "", credentialauth.ErrInvalidSecret
		}
		return s.decryptCredentialAuthSecret(ctx, input, provider, expectedSecretType, adminProfilePasswordSecretIdentifierType)
	}
	if strings.TrimSpace(input.Value) != "" {
		return input.Value, nil
	}
	return s.decryptCredentialAuthSecret(ctx, input, provider, expectedSecretType, adminProfilePasswordSecretIdentifierType)
}

func validAdminProfilePassword(value string) bool {
	trimmed := strings.TrimSpace(value)
	if len([]rune(trimmed)) < 8 || len([]rune(trimmed)) > 128 {
		return false
	}
	return strings.IndexFunc(trimmed, unicode.IsControl) < 0
}

func (s *Server) putAdminProfilePasswordCredential(ctx *gin.Context, runtime *CredentialAuthRuntime, user adminresource.Record, password string, mustChange bool) error {
	principal := credentialauth.PrincipalRef{Type: credentialauth.PrincipalTypeAdmin, ID: strings.TrimSpace(user.ID)}
	if _, err := runtime.Service.RegisterIdentifier(ctx.Request.Context(), credentialauth.RegisterIdentifierInput{
		Principal:  principal,
		Identifier: credentialauth.Identifier{Type: credentialauth.IdentifierTypeUsername, Value: user.Code},
		Status:     credentialauth.StatusEnabled,
	}); err != nil {
		return err
	}
	params := runtime.PasswordHashParams
	passwordHash, err := credentialauth.HashPasswordArgon2id(password, params)
	if err != nil {
		return err
	}
	paramsVersion := strings.TrimSpace(runtime.PasswordParamsVersion)
	if paramsVersion == "" {
		paramsVersion = adminProfilePasswordParamsVersionFallback
	}
	return runtime.Service.PutPasswordCredential(ctx.Request.Context(), credentialauth.PasswordCredential{
		Principal:         principal,
		PasswordHash:      passwordHash,
		Algorithm:         credentialauth.PasswordAlgorithmArgon2id,
		ParamsVersion:     paramsVersion,
		PasswordUpdatedAt: credentialAuthRuntimeNow(runtime).UTC(),
		MustChange:        mustChange,
		Status:            credentialauth.StatusEnabled,
	})
}

func (s *Server) syncAdminProfileCredentialIdentifiers(ctx *gin.Context, before adminresource.Record, after adminresource.Record, input adminProfileUpdateRequest) error {
	runtime := s.credentialAuth
	if runtime == nil || runtime.Service == nil {
		return nil
	}
	principal := credentialauth.PrincipalRef{Type: credentialauth.PrincipalTypeAdmin, ID: strings.TrimSpace(after.ID)}
	if principal.ID == "" {
		return nil
	}
	if input.Phone != nil {
		if err := s.syncAdminProfileCredentialIdentifier(ctx, runtime, principal, credentialauth.IdentifierTypePhone, before.Values["phone"], after.Values["phone"]); err != nil {
			return err
		}
	}
	if input.Email != nil {
		if err := s.syncAdminProfileCredentialIdentifier(ctx, runtime, principal, credentialauth.IdentifierTypeEmail, before.Values["email"], after.Values["email"]); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) syncAdminProfileCredentialIdentifier(ctx *gin.Context, runtime *CredentialAuthRuntime, principal credentialauth.PrincipalRef, identifierType credentialauth.IdentifierType, before string, after string) error {
	before = strings.TrimSpace(before)
	after = strings.TrimSpace(after)
	if before == after {
		return nil
	}
	if after != "" {
		if _, err := runtime.Service.RegisterIdentifier(ctx.Request.Context(), credentialauth.RegisterIdentifierInput{
			Principal:  principal,
			Identifier: credentialauth.Identifier{Type: identifierType, Value: after},
			Status:     credentialauth.StatusEnabled,
		}); err != nil {
			return err
		}
	}
	if before != "" {
		if _, err := runtime.Service.RegisterIdentifier(ctx.Request.Context(), credentialauth.RegisterIdentifierInput{
			Principal:  principal,
			Identifier: credentialauth.Identifier{Type: identifierType, Value: before},
			Status:     credentialauth.StatusDisabled,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) recordAdminProfilePasswordAudit(ctx *gin.Context, targetID string, action string, result string, reasonCode string) error {
	correlation := correlationFromGinContext(ctx)
	_, err := s.resources.RecordAudit(requestAuditEvent(ctx.Request.Context(), adminresource.AuditEvent{
		Actor:      s.auditActorID(ctx),
		Action:     action,
		Resource:   adminProfileUsersResource,
		TargetID:   strings.TrimSpace(targetID),
		Result:     result,
		EventID:    internalErrorEventID(ctx),
		ReasonCode: reasonCode,
		RequestID:  correlation.RequestID,
		TraceID:    correlation.TraceID,
	}))
	return err
}

func (s *Server) adminProfileWriteInput(record adminresource.Record, input adminProfileUpdateRequest) (adminresource.WriteInput, error) {
	schema, err := s.resources.Schema(adminProfileUsersResource)
	if err != nil {
		return adminresource.WriteInput{}, err
	}
	valueFields := adminProfileValueFields(schema)
	values := adminProfileDeclaredValues(record.Values, valueFields)
	if values == nil {
		values = map[string]string{}
	}
	name := record.Name
	if input.Name != nil {
		normalized, err := normalizeAdminProfileText(*input.Name, 80)
		if err != nil || normalized == "" {
			return adminresource.WriteInput{}, adminresource.ErrInvalidRecord
		}
		name = normalized
	} else if input.Nickname != nil {
		normalized, err := normalizeAdminProfileText(*input.Nickname, 80)
		if err != nil || normalized == "" {
			return adminresource.WriteInput{}, adminresource.ErrInvalidRecord
		}
		name = normalized
	}
	if err := setAdminProfileAvatar(values, valueFields, input); err != nil {
		return adminresource.WriteInput{}, err
	}
	if input.Phone != nil {
		phone, err := normalizeAdminProfileIdentifier(credentialauth.IdentifierTypePhone, *input.Phone)
		if err != nil {
			return adminresource.WriteInput{}, err
		}
		if err := setAdminProfileValue(values, valueFields, "phone", phone); err != nil {
			return adminresource.WriteInput{}, err
		}
	}
	if input.Email != nil {
		email, err := normalizeAdminProfileIdentifier(credentialauth.IdentifierTypeEmail, *input.Email)
		if err != nil {
			return adminresource.WriteInput{}, err
		}
		if err := setAdminProfileValue(values, valueFields, "email", email); err != nil {
			return adminresource.WriteInput{}, err
		}
	}
	if input.Address != nil {
		address, err := normalizeAdminProfileText(*input.Address, 240)
		if err != nil {
			return adminresource.WriteInput{}, err
		}
		if err := setAdminProfileValue(values, valueFields, "address", address); err != nil {
			return adminresource.WriteInput{}, err
		}
	}
	return adminresource.WriteInput{
		Code:        record.Code,
		Name:        name,
		Status:      record.Status,
		Description: record.Description,
		Values:      values,
	}, nil
}

func adminProfileValueFields(schema adminresource.Schema) map[string]struct{} {
	fields := make(map[string]struct{}, len(schema.Fields))
	for _, field := range schema.Fields {
		if field.Source == "values" {
			fields[field.Key] = struct{}{}
		}
	}
	return fields
}

func adminProfileDeclaredValues(existing map[string]string, valueFields map[string]struct{}) map[string]string {
	values := make(map[string]string)
	for key, value := range existing {
		if _, ok := valueFields[key]; ok {
			values[key] = value
		}
	}
	if len(values) == 0 {
		return nil
	}
	return values
}

func setAdminProfileAvatar(values map[string]string, valueFields map[string]struct{}, input adminProfileUpdateRequest) error {
	if input.AvatarURL == nil && input.Avatar == nil {
		return nil
	}
	raw := input.AvatarURL
	if raw == nil {
		raw = input.Avatar
	}
	avatarURL, err := normalizeAdminProfileAvatarURL(*raw)
	if err != nil {
		return err
	}
	key := "avatarUrl"
	if _, ok := valueFields[key]; !ok {
		key = "avatar"
	}
	return setAdminProfileValue(values, valueFields, key, avatarURL)
}

func setAdminProfileValue(values map[string]string, valueFields map[string]struct{}, key string, value string) error {
	if _, ok := valueFields[key]; !ok {
		return adminresource.ErrInvalidRecord
	}
	if values == nil {
		return adminresource.ErrInvalidRecord
	}
	values[key] = value
	return nil
}

func normalizeAdminProfileIdentifier(identifierType credentialauth.IdentifierType, value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", nil
	}
	normalized, err := credentialauth.NormalizeIdentifier(credentialauth.Identifier{Type: identifierType, Value: value})
	if err != nil {
		return "", adminresource.ErrInvalidRecord
	}
	return normalized.Value, nil
}

func normalizeAdminProfileText(value string, maxLength int) (string, error) {
	normalized := strings.TrimSpace(value)
	if len([]rune(normalized)) > maxLength || strings.IndexFunc(normalized, unicode.IsControl) >= 0 {
		return "", adminresource.ErrInvalidRecord
	}
	return normalized, nil
}

func normalizeAdminProfileAvatarURL(value string) (string, error) {
	normalized, err := normalizeAdminProfileText(value, 2048)
	if err != nil || normalized == "" {
		return normalized, err
	}
	if strings.HasPrefix(normalized, "/") && !strings.HasPrefix(normalized, "//") {
		return normalized, nil
	}
	parsed, err := url.Parse(normalized)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", adminresource.ErrInvalidRecord
	}
	return normalized, nil
}

func (s *Server) adminProfileFromRecord(record adminresource.Record, principal rbac.Principal) adminProfile {
	values := record.Values
	avatarURL := strings.TrimSpace(values["avatarUrl"])
	if avatarURL == "" {
		avatarURL = strings.TrimSpace(values["avatar"])
	}
	return adminProfile{
		ID:          record.ID,
		Username:    record.Code,
		Name:        record.Name,
		Nickname:    record.Name,
		AvatarURL:   avatarURL,
		Phone:       strings.TrimSpace(values["phone"]),
		MaskedPhone: maskedAdminProfileIdentifier(credentialauth.IdentifierTypePhone, values["phone"]),
		Email:       strings.TrimSpace(values["email"]),
		MaskedEmail: maskedAdminProfileIdentifier(credentialauth.IdentifierTypeEmail, values["email"]),
		Address:     strings.TrimSpace(values["address"]),
		TenantCode:  strings.TrimSpace(values["tenantCode"]),
		OrgUnitCode: strings.TrimSpace(values["orgUnitCode"]),
		AreaCode:    strings.TrimSpace(values["areaCode"]),
		Credentials: s.adminProfileCredentialStatus(principal),
	}
}

func maskedAdminProfileIdentifier(identifierType credentialauth.IdentifierType, value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	normalized, err := credentialauth.NormalizeIdentifier(credentialauth.Identifier{Type: identifierType, Value: value})
	if err != nil {
		return ""
	}
	return normalized.MaskedIdentifier
}

func (s *Server) adminProfileCredentialStatus(_ rbac.Principal) adminProfileCredentialStatus {
	if s.credentialAuth == nil || s.credentialAuth.Service == nil || s.credentialAuth.SecretTransport == nil {
		return adminProfileCredentialStatus{
			PasswordChange: "credential-auth-not-connected",
			PasswordReset:  "credential-auth-not-connected",
			Message:        "credential-auth is not connected for profile password changes",
		}
	}
	return adminProfileCredentialStatus{
		PasswordChange: adminProfileCredentialStatusReady,
		PasswordReset:  adminProfileCredentialStatusReady,
		Message:        "credential-auth profile password change/reset routes are available",
	}
}
