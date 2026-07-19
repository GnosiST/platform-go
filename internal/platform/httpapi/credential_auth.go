package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/GnosiST/platform-go/internal/platform/adminresource"
	"github.com/GnosiST/platform-go/internal/platform/capability"
	"github.com/GnosiST/platform-go/internal/platform/credentialauth"
	"github.com/GnosiST/platform-go/internal/platform/errorcode"
	"github.com/GnosiST/platform-go/internal/platform/notification"
	"github.com/GnosiST/platform-go/internal/platform/ratelimit"
	"github.com/GnosiST/platform-go/internal/platform/rbac"
)

const (
	authProviderKindCredentialPassword = "credential-password"
	authProviderKindCredentialSMSOTP   = "credential-sms-otp"
	credentialAuthLoginPurpose         = "login"
	defaultCredentialAuthSMSOTPTTL     = 5 * time.Minute
)

type credentialAuthIdentifierRequest struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type credentialAuthSecretRequest struct {
	Type          string `json:"type"`
	Value         string `json:"value"`
	TransactionID string `json:"transactionId"`
	Code          string `json:"code"`
}

type credentialAuthChallengeRequest struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Proof string `json:"proof"`
}

type credentialAuthSMSOTPStartRequest struct {
	Provider   string                          `json:"provider"`
	Identifier credentialAuthIdentifierRequest `json:"identifier"`
}

type credentialAuthSMSOTPStartResponse struct {
	TransactionID    string    `json:"transactionId"`
	MaskedIdentifier string    `json:"maskedIdentifier"`
	ExpiresAt        time.Time `json:"expiresAt"`
	DebugCode        string    `json:"debugCode,omitempty"`
}

type credentialAuthProviderSpec struct {
	ID             string
	Kind           string
	IdentifierType credentialauth.IdentifierType
	SecretType     string
}

type credentialAuthSMSOTPHasher interface {
	HashSMSOTP(phoneHash string, challengeID string, code string) (string, error)
}

type CredentialAuthRuntime struct {
	Service           *credentialauth.Service
	IdentifierHasher  credentialauth.IdentifierHasher
	SMSOTPHasher      credentialAuthSMSOTPHasher
	SMSSender         notification.SMSSender
	LoginTemplateID   string
	DebugCodeEnabled  bool
	CodeGenerator     func() (string, error)
	Now               func() time.Time
	SMSOTPTTL         time.Duration
	MaxSMSOTPAttempts int
}

func (s *Server) credentialAuthPasswordLogin(ctx *gin.Context, provider capability.AuthProvider, input authLoginRequest) {
	runtime := s.credentialAuth
	if runtime == nil || runtime.Service == nil {
		writePlatformError(ctx, errorcode.CodeAuthProviderNotConfigured)
		return
	}
	spec, ok := credentialAuthProviderSpecFor(provider)
	if !ok || spec.SecretType != "password" {
		writePlatformError(ctx, errorcode.CodeAuthProviderUnsupported)
		return
	}
	identifier, ok := credentialAuthIdentifierFromRequest(input.Identifier, spec.IdentifierType)
	if !ok {
		writePlatformError(ctx, errorcode.CodeAuthInvalidRequest)
		return
	}
	secretType := strings.TrimSpace(input.Secret.Type)
	if secretType == "" {
		secretType = "password"
	}
	if secretType != "password" || strings.TrimSpace(input.Secret.Value) == "" {
		writePlatformError(ctx, errorcode.CodeAuthInvalidRequest)
		return
	}
	result, err := runtime.Service.VerifyPassword(ctx.Request.Context(), credentialauth.PasswordLoginInput{
		Identifier: identifier,
		Secret:     input.Secret.Value,
	})
	if err != nil {
		writeCredentialAuthLoginError(ctx, s.internalErrorSink, err)
		return
	}
	principal, err := s.adminPrincipalFromCredential(ctx.Request.Context(), result.Principal)
	if err != nil {
		writePlatformError(ctx, errorcode.CodeAuthInvalidCredentials)
		return
	}
	s.issueAdminLogin(ctx, principal, provider)
}

func (s *Server) credentialAuthSMSOTPStart(ctx *gin.Context) {
	var input credentialAuthSMSOTPStartRequest
	if err := ctx.ShouldBindJSON(&input); err != nil {
		writePlatformError(ctx, errorcode.CodeAuthInvalidRequest)
		return
	}
	providerDimension := strings.ToLower(strings.TrimSpace(input.Provider))
	if providerDimension == "" {
		providerDimension = "unknown"
	}
	identifierDimension := strings.TrimSpace(input.Identifier.Value)
	if identifierDimension == "" {
		identifierDimension = "missing"
	}
	if !s.enforceRateLimit(ctx, ratelimit.OperationPhoneVerificationRequest, rateLimitClientIP(ctx), providerDimension, identifierDimension, credentialAuthLoginPurpose) {
		return
	}
	provider, ok := s.findAuthProvider(input.Provider, capability.AuthProviderAudienceAdmin)
	if !ok {
		writePlatformError(ctx, errorcode.CodeAuthProviderNotFound)
		return
	}
	if !provider.Configured {
		writePlatformError(ctx, errorcode.CodeAuthProviderNotConfigured)
		return
	}
	spec, ok := credentialAuthProviderSpecFor(provider)
	if !ok || spec.SecretType != "sms-otp" {
		writePlatformError(ctx, errorcode.CodeAuthProviderUnsupported)
		return
	}
	runtime := s.credentialAuth
	if runtime == nil || runtime.Service == nil || runtime.IdentifierHasher == nil || runtime.SMSOTPHasher == nil || runtime.SMSSender == nil || strings.TrimSpace(runtime.LoginTemplateID) == "" {
		writePlatformError(ctx, errorcode.CodeAuthProviderNotConfigured)
		return
	}
	identifier, ok := credentialAuthIdentifierFromRequest(input.Identifier, credentialauth.IdentifierTypePhone)
	if !ok {
		writePlatformError(ctx, errorcode.CodeAuthInvalidRequest)
		return
	}
	normalized, err := credentialauth.NormalizeIdentifier(identifier)
	if err != nil {
		writePlatformError(ctx, errorcode.CodeAuthInvalidRequest)
		return
	}
	resolved, found, err := runtime.Service.ResolveIdentifier(ctx.Request.Context(), identifier)
	if err != nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAuthProviderResolveFailed, err)
		return
	}
	if !found || credentialauth.RecordStatus(strings.TrimSpace(string(resolved.Status))) != credentialauth.StatusEnabled || resolved.Principal.Type != credentialauth.PrincipalTypeAdmin {
		writePlatformError(ctx, errorcode.CodeAuthInvalidCredentials)
		return
	}
	phoneHash, err := runtime.IdentifierHasher.HashIdentifier(credentialauth.IdentifierTypePhone, normalized.Value)
	if err != nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAuthProviderResolveFailed, err)
		return
	}
	challengeID, err := newCredentialAuthTransactionID()
	if err != nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAuthProviderResolveFailed, err)
		return
	}
	codeGenerator := runtime.CodeGenerator
	if codeGenerator == nil {
		codeGenerator = newAppPhoneDebugCode
	}
	code, err := codeGenerator()
	if err != nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAuthProviderResolveFailed, err)
		return
	}
	codeDigest, err := runtime.SMSOTPHasher.HashSMSOTP(phoneHash, challengeID, code)
	if err != nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAuthProviderResolveFailed, err)
		return
	}
	now := credentialAuthRuntimeNow(runtime).UTC()
	ttl := runtime.SMSOTPTTL
	if ttl <= 0 {
		ttl = defaultCredentialAuthSMSOTPTTL
	}
	receipt, err := runtime.SMSSender.SendSMS(ctx.Request.Context(), notification.SMSMessage{
		TenantCode: platformTenant,
		Recipient:  normalized.Value,
		TemplateID: strings.TrimSpace(runtime.LoginTemplateID),
		TemplateParams: map[string]string{
			"code": code,
		},
		Purpose: credentialAuthLoginPurpose,
		TraceID: correlationFromGinContext(ctx).TraceID,
	})
	if err != nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAuthProviderResolveFailed, err)
		return
	}
	if err := runtime.Service.PutSMSOTPChallenge(ctx.Request.Context(), credentialauth.SMSOTPChallenge{
		ChallengeID: challengeID,
		PhoneHash:   phoneHash,
		CodeDigest:  codeDigest,
		ExpiresAt:   now.Add(ttl),
		MessageID:   receipt.MessageID,
		Status:      credentialauth.StatusEnabled,
	}); err != nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAuthProviderResolveFailed, err)
		return
	}
	response := credentialAuthSMSOTPStartResponse{
		TransactionID:    challengeID,
		MaskedIdentifier: normalized.MaskedIdentifier,
		ExpiresAt:        now.Add(ttl),
	}
	if runtime.DebugCodeEnabled {
		response.DebugCode = code
	}
	ctx.JSON(http.StatusCreated, Response[credentialAuthSMSOTPStartResponse]{Data: response})
}

func (s *Server) credentialAuthSMSOTPLogin(ctx *gin.Context, provider capability.AuthProvider, input authLoginRequest) {
	runtime := s.credentialAuth
	if runtime == nil || runtime.Service == nil || runtime.IdentifierHasher == nil || runtime.SMSOTPHasher == nil {
		writePlatformError(ctx, errorcode.CodeAuthProviderNotConfigured)
		return
	}
	spec, ok := credentialAuthProviderSpecFor(provider)
	if !ok || spec.SecretType != "sms-otp" {
		writePlatformError(ctx, errorcode.CodeAuthProviderUnsupported)
		return
	}
	identifier, ok := credentialAuthIdentifierFromRequest(input.Identifier, credentialauth.IdentifierTypePhone)
	if !ok {
		writePlatformError(ctx, errorcode.CodeAuthInvalidRequest)
		return
	}
	transactionID := strings.TrimSpace(input.Secret.TransactionID)
	code := strings.TrimSpace(input.Secret.Code)
	secretType := strings.TrimSpace(input.Secret.Type)
	if secretType == "" {
		secretType = "sms-otp"
	}
	if secretType != "sms-otp" || transactionID == "" || code == "" {
		writePlatformError(ctx, errorcode.CodeAuthInvalidRequest)
		return
	}
	normalized, err := credentialauth.NormalizeIdentifier(identifier)
	if err != nil {
		writePlatformError(ctx, errorcode.CodeAuthInvalidRequest)
		return
	}
	phoneHash, err := runtime.IdentifierHasher.HashIdentifier(credentialauth.IdentifierTypePhone, normalized.Value)
	if err != nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAuthProviderResolveFailed, err)
		return
	}
	codeDigest, err := runtime.SMSOTPHasher.HashSMSOTP(phoneHash, transactionID, code)
	if err != nil {
		writePlatformError(ctx, errorcode.CodeAuthInvalidRequest)
		return
	}
	if err := runtime.Service.VerifySMSOTP(ctx.Request.Context(), credentialauth.SMSOTPProof{
		ChallengeID: transactionID,
		PhoneHash:   phoneHash,
		CodeDigest:  codeDigest,
		MaxAttempts: runtime.MaxSMSOTPAttempts,
	}); err != nil {
		writeCredentialAuthLoginError(ctx, s.internalErrorSink, err)
		return
	}
	resolved, found, err := runtime.Service.ResolveIdentifier(ctx.Request.Context(), identifier)
	if err != nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAuthProviderResolveFailed, err)
		return
	}
	if !found || credentialauth.RecordStatus(strings.TrimSpace(string(resolved.Status))) != credentialauth.StatusEnabled {
		writePlatformError(ctx, errorcode.CodeAuthInvalidCredentials)
		return
	}
	principal, err := s.adminPrincipalFromCredential(ctx.Request.Context(), resolved.Principal)
	if err != nil {
		writePlatformError(ctx, errorcode.CodeAuthInvalidCredentials)
		return
	}
	s.issueAdminLogin(ctx, principal, provider)
}

func credentialAuthProviderSpecFor(provider capability.AuthProvider) (credentialAuthProviderSpec, bool) {
	id := strings.TrimSpace(provider.ID)
	kind := strings.ToLower(strings.TrimSpace(provider.Kind))
	switch id {
	case "username-password":
		return credentialAuthProviderSpec{ID: id, Kind: kind, IdentifierType: credentialauth.IdentifierTypeUsername, SecretType: "password"}, kind == authProviderKindCredentialPassword
	case "phone-password":
		return credentialAuthProviderSpec{ID: id, Kind: kind, IdentifierType: credentialauth.IdentifierTypePhone, SecretType: "password"}, kind == authProviderKindCredentialPassword
	case "email-password":
		return credentialAuthProviderSpec{ID: id, Kind: kind, IdentifierType: credentialauth.IdentifierTypeEmail, SecretType: "password"}, kind == authProviderKindCredentialPassword
	case "phone-sms-otp":
		return credentialAuthProviderSpec{ID: id, Kind: kind, IdentifierType: credentialauth.IdentifierTypePhone, SecretType: "sms-otp"}, kind == authProviderKindCredentialSMSOTP
	default:
		return credentialAuthProviderSpec{}, false
	}
}

func credentialAuthIdentifierFromRequest(input credentialAuthIdentifierRequest, expected credentialauth.IdentifierType) (credentialauth.Identifier, bool) {
	identifierType := credentialauth.IdentifierType(strings.TrimSpace(input.Type))
	if identifierType == "" {
		identifierType = expected
	}
	if identifierType != expected || strings.TrimSpace(input.Value) == "" {
		return credentialauth.Identifier{}, false
	}
	return credentialauth.Identifier{Type: identifierType, Value: input.Value}, true
}

func (s *Server) adminPrincipalFromCredential(ctx context.Context, principal credentialauth.PrincipalRef) (rbac.Principal, error) {
	principal, err := normalizeCredentialPrincipalForHTTP(principal)
	if err != nil || principal.Type != credentialauth.PrincipalTypeAdmin {
		return rbac.Principal{}, credentialauth.ErrCredentialRejected
	}
	if current, err := adminresource.ValidateAdminPrincipal(s.resources, principal.ID); err == nil {
		return current, nil
	}
	users, err := s.resources.List("users")
	if err != nil {
		return rbac.Principal{}, err
	}
	for _, user := range users {
		if user.ID != principal.ID {
			continue
		}
		if err := ctx.Err(); err != nil {
			return rbac.Principal{}, err
		}
		return adminresource.ValidateAdminPrincipal(s.resources, user.Code)
	}
	return rbac.Principal{}, credentialauth.ErrCredentialRejected
}

func writeCredentialAuthLoginError(ctx *gin.Context, sink InternalErrorSink, err error) {
	switch {
	case errors.Is(err, credentialauth.ErrInvalidInput):
		writePlatformError(ctx, errorcode.CodeAuthInvalidRequest)
	case errors.Is(err, credentialauth.ErrCredentialRejected),
		errors.Is(err, credentialauth.ErrCredentialLocked),
		errors.Is(err, credentialauth.ErrChallengeRejected),
		errors.Is(err, credentialauth.ErrChallengeExpired),
		errors.Is(err, credentialauth.ErrChallengeConsumed):
		writePlatformError(ctx, errorcode.CodeAuthInvalidCredentials)
	default:
		writePlatformErrorWithCause(ctx, sink, errorcode.CodeAuthProviderResolveFailed, err)
	}
}

func credentialAuthRuntimeNow(runtime *CredentialAuthRuntime) time.Time {
	if runtime != nil && runtime.Now != nil {
		return runtime.Now()
	}
	return time.Now()
}

func newCredentialAuthTransactionID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return "otp-" + hex.EncodeToString(raw[:]), nil
}

func normalizeCredentialPrincipalForHTTP(principal credentialauth.PrincipalRef) (credentialauth.PrincipalRef, error) {
	normalized := credentialauth.PrincipalRef{Type: credentialauth.PrincipalType(strings.TrimSpace(string(principal.Type))), ID: strings.TrimSpace(principal.ID)}
	if normalized.ID == "" {
		return credentialauth.PrincipalRef{}, credentialauth.ErrInvalidInput
	}
	switch normalized.Type {
	case credentialauth.PrincipalTypeAdmin, credentialauth.PrincipalTypeApp:
		return normalized, nil
	default:
		return credentialauth.PrincipalRef{}, credentialauth.ErrInvalidInput
	}
}
