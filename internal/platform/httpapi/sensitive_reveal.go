package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"platform-go/internal/platform/adminresource"
	"platform-go/internal/platform/capability"
	"platform-go/internal/platform/errorcode"
	"platform-go/internal/platform/ratelimit"
	"platform-go/internal/platform/rbac"
	"platform-go/internal/platform/sensitivereveal"
	"platform-go/internal/platform/session"
)

const adminSensitiveRevealSMSPurpose = "sensitive-reveal"

const sensitiveRevealAuditTimeout = 2 * time.Second

type adminSensitiveRevealPurpose struct {
	Code  string                   `json:"code"`
	Label capability.LocalizedText `json:"label"`
}

type adminSensitiveRevealProvider struct {
	ID    string                   `json:"id"`
	Title capability.LocalizedText `json:"title"`
}

type adminSensitiveRevealFactor struct {
	Type              string                         `json:"type"`
	Available         bool                           `json:"available"`
	Providers         []adminSensitiveRevealProvider `json:"providers,omitempty"`
	MaskedDestination string                         `json:"maskedDestination,omitempty"`
}

type adminSensitiveRevealPolicyResponse struct {
	PolicyID            string                        `json:"policyId"`
	Mode                string                        `json:"mode"`
	Purposes            []adminSensitiveRevealPurpose `json:"purposes"`
	Factors             []adminSensitiveRevealFactor  `json:"factors"`
	ChallengeTTLSeconds int                           `json:"challengeTtlSeconds"`
	GrantTTLSeconds     int                           `json:"grantTtlSeconds"`
	CopyAllowed         bool                          `json:"copyAllowed"`
}

type adminSensitiveRevealChallengeRequest struct {
	Purpose string `json:"purpose"`
}

type adminSensitiveRevealChallengeResponse struct {
	ChallengeID    string    `json:"challengeId"`
	ChallengeToken string    `json:"challengeToken"`
	PolicyID       string    `json:"policyId"`
	Mode           string    `json:"mode"`
	Factors        []string  `json:"factors"`
	ExpiresAt      time.Time `json:"expiresAt"`
}

type adminSensitiveRevealFactorRequest struct {
	ChallengeToken string `json:"challengeToken"`
	Purpose        string `json:"purpose"`
}

type adminSensitiveRevealOIDCStartRequest struct {
	adminSensitiveRevealFactorRequest
	Provider      string `json:"provider"`
	CodeChallenge string `json:"codeChallenge"`
}

type adminSensitiveRevealOIDCStartResponse struct {
	ChallengeID      string    `json:"challengeId"`
	TransactionToken string    `json:"transactionToken"`
	AuthorizationURL string    `json:"authorizationUrl"`
	State            string    `json:"state"`
	ExpiresAt        time.Time `json:"expiresAt"`
}

type adminSensitiveRevealOIDCCompleteRequest struct {
	adminSensitiveRevealFactorRequest
	TransactionToken string `json:"transactionToken"`
	Provider         string `json:"provider"`
	Code             string `json:"code"`
	State            string `json:"state"`
	CodeVerifier     string `json:"codeVerifier"`
}

type adminSensitiveRevealSMSStartResponse struct {
	ChallengeID      string    `json:"challengeId"`
	TransactionToken string    `json:"transactionToken"`
	MaskedPhone      string    `json:"maskedPhone"`
	ExpiresAt        time.Time `json:"expiresAt"`
	DebugCode        string    `json:"debugCode,omitempty"`
}

type adminSensitiveRevealSMSCompleteRequest struct {
	adminSensitiveRevealFactorRequest
	TransactionToken string `json:"transactionToken"`
	Code             string `json:"code"`
}

type adminSensitiveRevealFactorCompleteResponse struct {
	ChallengeID     string    `json:"challengeId"`
	PolicySatisfied bool      `json:"policySatisfied"`
	GrantToken      string    `json:"grantToken,omitempty"`
	GrantExpiresAt  time.Time `json:"grantExpiresAt,omitempty"`
}

type adminSensitiveRevealRequest struct {
	Purpose    string `json:"purpose"`
	GrantToken string `json:"grantToken"`
}

type adminSensitiveRevealResponse struct {
	Field       string `json:"field"`
	Value       string `json:"value"`
	CopyAllowed bool   `json:"copyAllowed"`
}

type adminSensitiveRevealTarget struct {
	resource    string
	recordID    string
	field       adminresource.FieldDefinition
	principal   rbac.Principal
	authSession session.Session
	scope       sensitivereveal.Scope
}

func (s *Server) adminSensitiveRevealPolicy(ctx *gin.Context) {
	noStoreSensitiveReveal(ctx)
	target, ok := s.resolveAdminSensitiveRevealTarget(ctx, "", false)
	if !ok {
		return
	}
	policy, ok := s.findSensitiveRevealPolicy(target.field.Reveal.PolicyID)
	if !ok {
		writeSensitiveRevealError(ctx, s.internalErrorSink, sensitivereveal.ErrPolicyNotFound)
		return
	}
	factors := make([]adminSensitiveRevealFactor, 0, len(policy.Factors))
	for _, factor := range policy.Factors {
		descriptor := adminSensitiveRevealFactor{Type: strings.TrimSpace(factor)}
		switch descriptor.Type {
		case capability.AdminRevealFactorOIDCReauthentication:
			descriptor.Providers = s.adminSensitiveRevealOIDCProviders()
			_, descriptor.Available = s.adminIdentityResolver.(AdminStepUpIdentityResolver)
			descriptor.Available = descriptor.Available && len(descriptor.Providers) > 0
		case capability.AdminRevealFactorSMSOTP:
			if s.adminStepUpPhoneResolver != nil && s.phoneProtector != nil && s.phoneVerificationSender != nil {
				phone, err := s.adminStepUpPhoneResolver.ResolveVerifiedAdminPhone(ctx.Request.Context(), target.principal.User.Username)
				if err == nil {
					descriptor.Available = true
					descriptor.MaskedDestination = phone.MaskedPhone
				}
			}
		}
		factors = append(factors, descriptor)
	}
	purposes := make([]adminSensitiveRevealPurpose, 0, len(policy.Purposes))
	for _, purpose := range policy.Purposes {
		purposes = append(purposes, adminSensitiveRevealPurpose{Code: purpose.Code, Label: purpose.Label})
	}
	noStoreSensitiveReveal(ctx)
	ctx.JSON(http.StatusOK, Response[adminSensitiveRevealPolicyResponse]{Data: adminSensitiveRevealPolicyResponse{
		PolicyID: policy.ID, Mode: policy.Mode, Purposes: purposes, Factors: factors,
		ChallengeTTLSeconds: policy.ChallengeTTLSeconds, GrantTTLSeconds: policy.GrantTTLSeconds,
		CopyAllowed: target.field.Reveal.CopyAllowed,
	}})
}

func (s *Server) adminSensitiveRevealChallenge(ctx *gin.Context) {
	noStoreSensitiveReveal(ctx)
	var input adminSensitiveRevealChallengeRequest
	if err := bindSensitiveRevealJSON(ctx, &input); err != nil {
		writeSensitiveRevealError(ctx, s.internalErrorSink, sensitivereveal.ErrInvalidScope)
		return
	}
	target, ok := s.resolveAdminSensitiveRevealTarget(ctx, input.Purpose, true)
	if !ok {
		return
	}
	if !s.enforceRateLimit(ctx, ratelimit.OperationSensitiveRevealChallenge, rateLimitClientIP(ctx), target.scope.Actor, target.resource, target.recordID, target.field.Key) {
		return
	}
	result, err := s.sensitiveReveal.BeginChallenge(ctx.Request.Context(), sensitivereveal.BeginChallengeRequest{
		PolicyID: target.field.Reveal.PolicyID,
		Scope:    target.scope,
	})
	if err != nil {
		writeSensitiveRevealError(ctx, s.internalErrorSink, err)
		return
	}
	factors := make([]string, 0, len(result.Factors))
	for _, factor := range result.Factors {
		publicFactor, ok := publicAdminSensitiveRevealFactor(factor.Factor)
		if !ok {
			writeSensitiveRevealError(ctx, s.internalErrorSink, sensitivereveal.ErrInvalidConfiguration)
			return
		}
		factors = append(factors, publicFactor)
	}
	noStoreSensitiveReveal(ctx)
	ctx.JSON(http.StatusCreated, Response[adminSensitiveRevealChallengeResponse]{Data: adminSensitiveRevealChallengeResponse{
		ChallengeID: result.ChallengeID, ChallengeToken: result.ChallengeToken, PolicyID: result.PolicyID,
		Mode: string(result.Mode), Factors: factors, ExpiresAt: result.ExpiresAt,
	}})
}

func (s *Server) adminSensitiveRevealOIDCStart(ctx *gin.Context) {
	noStoreSensitiveReveal(ctx)
	var input adminSensitiveRevealOIDCStartRequest
	if err := bindSensitiveRevealJSON(ctx, &input); err != nil {
		writeSensitiveRevealError(ctx, s.internalErrorSink, sensitivereveal.ErrInvalidScope)
		return
	}
	target, ok := s.resolveAdminSensitiveRevealTarget(ctx, input.Purpose, true)
	if !ok {
		return
	}
	if !s.enforceRateLimit(ctx, ratelimit.OperationSensitiveRevealFactor, rateLimitClientIP(ctx), target.scope.Actor, "oidc-start") {
		return
	}
	provider, ok := s.findAuthProvider(input.Provider, capability.AuthProviderAudienceAdmin)
	resolver, resolverOK := s.adminIdentityResolver.(AdminStepUpIdentityResolver)
	if !ok || provider.Kind != "oidc" || !provider.Configured || !resolverOK {
		writeSensitiveRevealError(ctx, s.internalErrorSink, ErrAdminIdentityInvalid)
		return
	}
	started, err := resolver.StartAdminStepUpIdentity(ctx.Request.Context(), AdminIdentityStartInput{
		Provider: provider, CodeChallenge: strings.TrimSpace(input.CodeChallenge),
	})
	if err != nil {
		writeSensitiveRevealError(ctx, s.internalErrorSink, err)
		return
	}
	factor, err := s.sensitiveReveal.BeginFactor(ctx.Request.Context(), sensitivereveal.BeginFactorRequest{
		ChallengeToken: strings.TrimSpace(input.ChallengeToken), ExpectedChallengeID: strings.TrimSpace(ctx.Param("challenge")), Factor: sensitivereveal.FactorOIDCReauthentication,
	})
	if err != nil {
		writeSensitiveRevealError(ctx, s.internalErrorSink, err)
		return
	}
	noStoreSensitiveReveal(ctx)
	ctx.JSON(http.StatusCreated, Response[adminSensitiveRevealOIDCStartResponse]{Data: adminSensitiveRevealOIDCStartResponse{
		ChallengeID: factor.ChallengeID, TransactionToken: factor.TransactionToken,
		AuthorizationURL: started.AuthorizationURL, State: started.State, ExpiresAt: minSensitiveRevealExpiry(started.ExpiresAt, factor.ExpiresAt),
	}})
}

func (s *Server) adminSensitiveRevealOIDCComplete(ctx *gin.Context) {
	noStoreSensitiveReveal(ctx)
	var input adminSensitiveRevealOIDCCompleteRequest
	if err := bindSensitiveRevealJSON(ctx, &input); err != nil {
		writeSensitiveRevealError(ctx, s.internalErrorSink, sensitivereveal.ErrInvalidScope)
		return
	}
	target, ok := s.resolveAdminSensitiveRevealTarget(ctx, input.Purpose, true)
	if !ok {
		return
	}
	if !s.enforceRateLimit(ctx, ratelimit.OperationSensitiveRevealFactor, rateLimitClientIP(ctx), target.scope.Actor, "oidc-complete") {
		return
	}
	provider, ok := s.findAuthProvider(input.Provider, capability.AuthProviderAudienceAdmin)
	resolver, resolverOK := s.adminIdentityResolver.(AdminStepUpIdentityResolver)
	if !ok || provider.Kind != "oidc" || !provider.Configured || !resolverOK {
		writeSensitiveRevealError(ctx, s.internalErrorSink, ErrAdminIdentityInvalid)
		return
	}
	identity, err := resolver.ResolveAdminStepUpIdentity(ctx.Request.Context(), AdminIdentityResolveInput{
		Provider: provider, Code: strings.TrimSpace(input.Code), State: strings.TrimSpace(input.State), CodeVerifier: strings.TrimSpace(input.CodeVerifier),
	})
	if err != nil {
		writeSensitiveRevealError(ctx, s.internalErrorSink, err)
		return
	}
	binding, err := s.adminIdentityBindings.ResolveAdminIdentityBinding(ctx.Request.Context(), AdminIdentityBindingInput{
		Provider: provider, Issuer: identity.Issuer, ProviderSubject: identity.ProviderSubject, Now: s.now().UTC(),
	})
	if err != nil {
		writeSensitiveRevealError(ctx, s.internalErrorSink, err)
		return
	}
	if strings.TrimSpace(binding.Username) != strings.TrimSpace(target.principal.User.Username) {
		writeSensitiveRevealError(ctx, s.internalErrorSink, sensitivereveal.ErrVerificationFailed)
		return
	}
	result, err := s.sensitiveReveal.CompleteFactor(ctx.Request.Context(), sensitivereveal.CompleteFactorRequest{
		ChallengeToken: strings.TrimSpace(input.ChallengeToken), ExpectedChallengeID: strings.TrimSpace(ctx.Param("challenge")), TransactionToken: strings.TrimSpace(input.TransactionToken), Verified: true,
	})
	if err != nil {
		writeSensitiveRevealError(ctx, s.internalErrorSink, err)
		return
	}
	writeSensitiveRevealFactorComplete(ctx, result)
}

func (s *Server) adminSensitiveRevealSMSStart(ctx *gin.Context) {
	noStoreSensitiveReveal(ctx)
	var input adminSensitiveRevealFactorRequest
	if err := bindSensitiveRevealJSON(ctx, &input); err != nil {
		writeSensitiveRevealError(ctx, s.internalErrorSink, sensitivereveal.ErrInvalidScope)
		return
	}
	target, ok := s.resolveAdminSensitiveRevealTarget(ctx, input.Purpose, true)
	if !ok {
		return
	}
	if !s.enforceRateLimit(ctx, ratelimit.OperationSensitiveRevealFactor, rateLimitClientIP(ctx), target.scope.Actor, "sms-start") {
		return
	}
	if s.adminStepUpPhoneResolver == nil || s.phoneProtector == nil || s.phoneVerificationSender == nil {
		writeSensitiveRevealUnavailable(ctx, s.internalErrorSink, nil)
		return
	}
	phone, err := s.adminStepUpPhoneResolver.ResolveVerifiedAdminPhone(ctx.Request.Context(), target.principal.User.Username)
	if err != nil {
		writeSensitiveRevealUnavailable(ctx, s.internalErrorSink, err)
		return
	}
	phoneDigest, err := s.phoneProtector.PhoneDigest(phone.Phone)
	if err != nil {
		writeSensitiveRevealUnavailable(ctx, s.internalErrorSink, err)
		return
	}
	verificationCode, err := newAppPhoneDebugCode()
	if err != nil {
		writeSensitiveRevealError(ctx, s.internalErrorSink, err)
		return
	}
	codeDigest, err := s.phoneProtector.CodeDigest(phoneDigest, sensitiveRevealSMSCodePurpose(target.scope.Purpose), verificationCode)
	if err != nil {
		writeSensitiveRevealUnavailable(ctx, s.internalErrorSink, err)
		return
	}
	factor, err := s.sensitiveReveal.BeginFactor(ctx.Request.Context(), sensitivereveal.BeginFactorRequest{
		ChallengeToken: strings.TrimSpace(input.ChallengeToken), ExpectedChallengeID: strings.TrimSpace(ctx.Param("challenge")), Factor: sensitivereveal.FactorSMSOTP, VerificationSecret: codeDigest,
	})
	if err != nil {
		writeSensitiveRevealError(ctx, s.internalErrorSink, err)
		return
	}
	if err := s.phoneVerificationSender.Send(ctx.Request.Context(), phone.Phone, adminSensitiveRevealSMSPurpose, verificationCode); err != nil {
		cancelErr := s.sensitiveReveal.CancelFactor(ctx.Request.Context(), sensitivereveal.CancelFactorRequest{
			ChallengeToken:      strings.TrimSpace(input.ChallengeToken),
			ExpectedChallengeID: strings.TrimSpace(ctx.Param("challenge")),
			TransactionToken:    factor.TransactionToken,
			Reason:              sensitivereveal.FactorCancelReasonDeliveryFailed,
		})
		if cancelErr != nil {
			writeSensitiveRevealError(ctx, s.internalErrorSink, cancelErr)
			return
		}
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAdminSensitiveRevealDeliveryFailed, err)
		return
	}
	response := adminSensitiveRevealSMSStartResponse{
		ChallengeID: factor.ChallengeID, TransactionToken: factor.TransactionToken, MaskedPhone: phone.MaskedPhone, ExpiresAt: factor.ExpiresAt,
	}
	if s.debugCodeEnabled && s.phoneVerificationSender.Kind() == PhoneVerificationProviderDebug {
		response.DebugCode = verificationCode
	}
	noStoreSensitiveReveal(ctx)
	ctx.JSON(http.StatusCreated, Response[adminSensitiveRevealSMSStartResponse]{Data: response})
}

func (s *Server) adminSensitiveRevealSMSComplete(ctx *gin.Context) {
	noStoreSensitiveReveal(ctx)
	var input adminSensitiveRevealSMSCompleteRequest
	if err := bindSensitiveRevealJSON(ctx, &input); err != nil {
		writeSensitiveRevealError(ctx, s.internalErrorSink, sensitivereveal.ErrInvalidScope)
		return
	}
	target, ok := s.resolveAdminSensitiveRevealTarget(ctx, input.Purpose, true)
	if !ok {
		return
	}
	if !s.enforceRateLimit(ctx, ratelimit.OperationSensitiveRevealFactor, rateLimitClientIP(ctx), target.scope.Actor, "sms-complete") {
		return
	}
	if s.adminStepUpPhoneResolver == nil || s.phoneProtector == nil {
		writeSensitiveRevealUnavailable(ctx, s.internalErrorSink, nil)
		return
	}
	phone, err := s.adminStepUpPhoneResolver.ResolveVerifiedAdminPhone(ctx.Request.Context(), target.principal.User.Username)
	if err != nil {
		writeSensitiveRevealUnavailable(ctx, s.internalErrorSink, err)
		return
	}
	phoneDigest, err := s.phoneProtector.PhoneDigest(phone.Phone)
	if err != nil {
		writeSensitiveRevealUnavailable(ctx, s.internalErrorSink, err)
		return
	}
	codeDigest, err := s.phoneProtector.CodeDigest(phoneDigest, sensitiveRevealSMSCodePurpose(target.scope.Purpose), input.Code)
	if err != nil {
		writeSensitiveRevealError(ctx, s.internalErrorSink, sensitivereveal.ErrVerificationFailed)
		return
	}
	result, err := s.sensitiveReveal.CompleteFactor(ctx.Request.Context(), sensitivereveal.CompleteFactorRequest{
		ChallengeToken: strings.TrimSpace(input.ChallengeToken), ExpectedChallengeID: strings.TrimSpace(ctx.Param("challenge")), TransactionToken: strings.TrimSpace(input.TransactionToken), VerificationProof: codeDigest,
	})
	if err != nil {
		writeSensitiveRevealError(ctx, s.internalErrorSink, err)
		return
	}
	writeSensitiveRevealFactorComplete(ctx, result)
}

func (s *Server) adminSensitiveReveal(ctx *gin.Context) {
	noStoreSensitiveReveal(ctx)
	var input adminSensitiveRevealRequest
	if err := bindSensitiveRevealJSON(ctx, &input); err != nil {
		writeSensitiveRevealError(ctx, s.internalErrorSink, sensitivereveal.ErrInvalidScope)
		return
	}
	target, ok := s.resolveAdminSensitiveRevealTarget(ctx, input.Purpose, true)
	if !ok {
		return
	}
	if !s.enforceRateLimit(ctx, ratelimit.OperationSensitiveRevealConsume, rateLimitClientIP(ctx), target.scope.Actor, target.resource, target.recordID, target.field.Key) {
		return
	}
	grant, err := s.sensitiveReveal.ConsumeGrant(ctx.Request.Context(), sensitivereveal.ConsumeGrantRequest{
		GrantToken: strings.TrimSpace(input.GrantToken), Scope: target.scope,
	})
	if err != nil {
		writeSensitiveRevealError(ctx, s.internalErrorSink, err)
		return
	}
	value, revealErr := s.resources.RevealProtectedField(ctx.Request.Context(), adminresource.ProtectedFieldRevealRequest{
		Resource: target.resource, RecordID: target.recordID, Field: target.field.Key, Purpose: adminresource.ProtectedFieldPurposeSensitiveReveal,
	})
	if revealErr != nil {
		reason := sensitivereveal.RevealReasonProtectedValueUnavailable
		if errors.Is(revealErr, adminresource.ErrProtectedFieldDecryptionFailed) {
			reason = sensitivereveal.RevealReasonDecryptionFailed
		}
		_ = s.recordSensitiveRevealResult(ctx.Request.Context(), grant.GrantID, target.scope, false, reason)
		writeSensitiveRevealError(ctx, s.internalErrorSink, revealErr)
		return
	}
	payload, err := json.Marshal(Response[adminSensitiveRevealResponse]{Data: adminSensitiveRevealResponse{
		Field: target.field.Key, Value: value, CopyAllowed: target.field.Reveal.CopyAllowed,
	}})
	if err != nil {
		_ = s.recordSensitiveRevealResult(ctx.Request.Context(), grant.GrantID, target.scope, false, sensitivereveal.RevealReasonProjectionFailed)
		writeSensitiveRevealError(ctx, s.internalErrorSink, err)
		return
	}
	noStoreSensitiveReveal(ctx)
	ctx.Header("Content-Type", "application/json; charset=utf-8")
	ctx.Status(http.StatusOK)
	written, writeErr := ctx.Writer.Write(payload)
	success := writeErr == nil && written == len(payload)
	reason := sensitivereveal.RevealReasonCompleted
	if !success {
		reason = sensitivereveal.RevealReasonResponseAborted
	}
	if auditErr := s.recordSensitiveRevealResult(ctx.Request.Context(), grant.GrantID, target.scope, success, reason); auditErr != nil {
		_ = ctx.Error(auditErr)
	}
	if writeErr != nil {
		_ = ctx.Error(writeErr)
	}
}

func (s *Server) recordSensitiveRevealResult(requestCtx context.Context, grantID string, scope sensitivereveal.Scope, success bool, reason string) error {
	auditCtx, cancel := context.WithTimeout(context.WithoutCancel(requestCtx), sensitiveRevealAuditTimeout)
	defer cancel()
	return s.sensitiveReveal.RecordRevealResult(auditCtx, grantID, scope, success, reason)
}

func (s *Server) resolveAdminSensitiveRevealTarget(ctx *gin.Context, purpose string, requirePurpose bool) (adminSensitiveRevealTarget, bool) {
	if s.sensitiveReveal == nil {
		writeSensitiveRevealUnavailable(ctx, s.internalErrorSink, nil)
		return adminSensitiveRevealTarget{}, false
	}
	if !s.refreshSensitiveRevealState(ctx) {
		return adminSensitiveRevealTarget{}, false
	}
	authSession, ok, err := s.authSessionFromBearerContext(ctx)
	if err != nil {
		writeSensitiveRevealError(ctx, s.internalErrorSink, err)
		return adminSensitiveRevealTarget{}, false
	}
	if !ok {
		writePlatformError(ctx, errorcode.CodeAuthUnauthorized)
		return adminSensitiveRevealTarget{}, false
	}
	principal := s.currentPrincipalForUsername(ctx.Request.Context(), authSession.Username)
	if strings.TrimSpace(principal.User.ID) == "" {
		writePlatformError(ctx, errorcode.CodeAuthUnauthorized)
		return adminSensitiveRevealTarget{}, false
	}
	resource := strings.TrimSpace(ctx.Param("resource"))
	recordID := strings.TrimSpace(ctx.Param("id"))
	fieldKey := strings.TrimSpace(ctx.Param("field"))
	schema, err := s.resources.Schema(resource)
	if err != nil {
		writeAdminResourceError(ctx, s.internalErrorSink, err)
		return adminSensitiveRevealTarget{}, false
	}
	field, found := sensitiveRevealField(schema, fieldKey)
	if !found || field.Reveal == nil {
		writeSensitiveRevealError(ctx, s.internalErrorSink, adminresource.ErrRecordNotFound)
		return adminSensitiveRevealTarget{}, false
	}
	if !s.can(principal, schema.Permissions.Read) || !s.can(principal, field.Reveal.Permission) {
		writePlatformError(ctx, errorcode.CodeAdminForbidden)
		return adminSensitiveRevealTarget{}, false
	}
	items, err := s.resources.ListForPrincipal(resource, principal)
	if err != nil {
		writeAdminResourceError(ctx, s.internalErrorSink, err)
		return adminSensitiveRevealTarget{}, false
	}
	if !sensitiveRevealRecordExists(items, recordID) {
		writeAdminResourceError(ctx, s.internalErrorSink, adminresource.ErrRecordNotFound)
		return adminSensitiveRevealTarget{}, false
	}
	purpose = strings.TrimSpace(purpose)
	if requirePurpose && purpose == "" {
		writeSensitiveRevealError(ctx, s.internalErrorSink, sensitivereveal.ErrInvalidScope)
		return adminSensitiveRevealTarget{}, false
	}
	tenant := strings.TrimSpace(principal.User.TenantCode)
	if tenant == "" {
		tenant = platformTenant
	}
	target := adminSensitiveRevealTarget{
		resource: resource, recordID: recordID, field: field, principal: principal, authSession: authSession,
		scope: sensitivereveal.Scope{
			Actor: principal.User.ID, SessionDigest: session.DigestToken(authSession.Token), Tenant: tenant,
			Resource: resource, Record: recordID, Field: field.Key, Purpose: purpose, Permission: field.Reveal.Permission,
		},
	}
	return target, true
}

func (s *Server) refreshSensitiveRevealState(ctx *gin.Context) bool {
	changed, err := s.resources.RefreshContext(ctx.Request.Context())
	if err != nil {
		noStoreSensitiveReveal(ctx)
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAdminSensitiveRevealStateRefreshFailed, err)
		return false
	}
	if changed {
		s.invalidatePolicyAuthorizer()
		_ = s.cache.DeletePrefix(ctx.Request.Context(), cacheKeyPrincipalPrefix)
		_ = s.cache.DeletePrefix(ctx.Request.Context(), cacheKeyMenusPrefix)
	}
	return true
}

func (s *Server) findSensitiveRevealPolicy(policyID string) (capability.AdminRevealPolicy, bool) {
	policyID = strings.TrimSpace(policyID)
	for _, manifest := range s.capabilities {
		for _, policy := range manifest.Admin.RevealPolicies {
			if strings.TrimSpace(policy.ID) == policyID {
				return policy, true
			}
		}
	}
	return capability.AdminRevealPolicy{}, false
}

func (s *Server) adminSensitiveRevealOIDCProviders() []adminSensitiveRevealProvider {
	providers := make([]adminSensitiveRevealProvider, 0)
	for _, manifest := range s.capabilities {
		for _, provider := range manifest.AuthProviders {
			if provider.Kind != "oidc" || !provider.Configured || !provider.SupportsAudience(capability.AuthProviderAudienceAdmin) || !s.authProviderAvailable(provider) {
				continue
			}
			providers = append(providers, adminSensitiveRevealProvider{ID: provider.ID, Title: provider.Title})
		}
	}
	return providers
}

func sensitiveRevealField(schema adminresource.Schema, fieldKey string) (adminresource.FieldDefinition, bool) {
	for _, field := range schema.Fields {
		if strings.TrimSpace(field.Key) == strings.TrimSpace(fieldKey) {
			return field, true
		}
	}
	return adminresource.FieldDefinition{}, false
}

func sensitiveRevealRecordExists(items []adminresource.Record, recordID string) bool {
	for _, item := range items {
		if strings.TrimSpace(item.ID) == strings.TrimSpace(recordID) {
			return true
		}
	}
	return false
}

func sensitiveRevealSMSCodePurpose(purpose string) string {
	return adminSensitiveRevealSMSPurpose + ":" + strings.TrimSpace(purpose)
}

func publicAdminSensitiveRevealFactor(factor sensitivereveal.Factor) (string, bool) {
	switch factor {
	case sensitivereveal.FactorOIDCReauthentication:
		return capability.AdminRevealFactorOIDCReauthentication, true
	case sensitivereveal.FactorSMSOTP:
		return capability.AdminRevealFactorSMSOTP, true
	default:
		return "", false
	}
}

func bindSensitiveRevealJSON(ctx *gin.Context, target any) error {
	decoder := json.NewDecoder(ctx.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("sensitive reveal request must contain one JSON value")
		}
		return err
	}
	return nil
}

func minSensitiveRevealExpiry(first time.Time, second time.Time) time.Time {
	if first.IsZero() || (!second.IsZero() && second.Before(first)) {
		return second
	}
	return first
}

func writeSensitiveRevealFactorComplete(ctx *gin.Context, result sensitivereveal.CompleteFactorResult) {
	noStoreSensitiveReveal(ctx)
	ctx.JSON(http.StatusOK, Response[adminSensitiveRevealFactorCompleteResponse]{Data: adminSensitiveRevealFactorCompleteResponse{
		ChallengeID: result.ChallengeID, PolicySatisfied: result.PolicySatisfied,
		GrantToken: result.GrantToken, GrantExpiresAt: result.GrantExpiresAt,
	}})
}

func noStoreSensitiveReveal(ctx *gin.Context) {
	ctx.Header("Cache-Control", "no-store")
	ctx.Header("Pragma", "no-cache")
}
