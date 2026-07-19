package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
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
	Type          string                         `json:"type"`
	Value         string                         `json:"value"`
	TransactionID string                         `json:"transactionId"`
	Code          string                         `json:"code"`
	Encrypted     *credentialauth.SecretEnvelope `json:"encrypted,omitempty"`
}

type credentialAuthChallengeRequest struct {
	ID                string `json:"id"`
	Kind              string `json:"kind"`
	Proof             string `json:"proof"`
	ClientFingerprint string `json:"clientFingerprint"`
}

type credentialAuthChallengeStartRequest struct {
	Kind              string `json:"kind"`
	Purpose           string `json:"purpose"`
	ClientFingerprint string `json:"clientFingerprint"`
}

type credentialAuthChallengeStartResponse struct {
	ID         string            `json:"id"`
	Kind       string            `json:"kind"`
	Purpose    string            `json:"purpose"`
	Prompt     string            `json:"prompt"`
	Parameters map[string]string `json:"parameters,omitempty"`
	ExpiresAt  time.Time         `json:"expiresAt"`
	DebugProof string            `json:"debugProof,omitempty"`
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

type credentialAuthSecretKeyResponse struct {
	Version   string    `json:"version"`
	Algorithm string    `json:"algorithm"`
	KeyID     string    `json:"keyId"`
	PublicKey string    `json:"publicKey"`
	ExpiresAt time.Time `json:"expiresAt"`
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
	Service                 *credentialauth.Service
	IdentifierHasher        credentialauth.IdentifierHasher
	SecretTransport         *credentialauth.SecretTransport
	SMSOTPHasher            credentialAuthSMSOTPHasher
	ChallengeProofHasher    credentialauth.ChallengeProofHasher
	SMSSender               notification.SMSSender
	LoginTemplateID         string
	DebugCodeEnabled        bool
	CodeGenerator           func() (string, error)
	Now                     func() time.Time
	SMSOTPTTL               time.Duration
	MaxSMSOTPAttempts       int
	MaxChallengeAttempts    int
	ChallengeTTL            time.Duration
	PasswordHashParams      credentialauth.Argon2idParams
	PasswordParamsVersion   string
	RequireEncryptedSecrets bool
}

func (s *Server) credentialAuthSecretKey(ctx *gin.Context) {
	runtime := s.credentialAuth
	if runtime == nil || runtime.SecretTransport == nil {
		writePlatformError(ctx, errorcode.CodeAuthProviderNotConfigured)
		return
	}
	key := runtime.SecretTransport.PublicKey()
	ctx.JSON(http.StatusOK, Response[credentialAuthSecretKeyResponse]{Data: credentialAuthSecretKeyResponse{
		Version:   key.Version,
		Algorithm: key.Algorithm,
		KeyID:     key.KeyID,
		PublicKey: key.PublicKey,
		ExpiresAt: key.ExpiresAt,
	}})
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
	if secretType != "password" {
		writePlatformError(ctx, errorcode.CodeAuthInvalidRequest)
		return
	}
	if !s.verifyCredentialAuthLoginChallenge(ctx, input.Challenge) {
		return
	}
	if runtime.RequireEncryptedSecrets {
		if credentialAuthPlaintextSecretPresent(input.Secret) {
			writePlatformError(ctx, errorcode.CodeAuthInvalidRequest)
			return
		}
		secret, err := s.decryptCredentialAuthSecret(ctx, input.Secret, provider.ID, "password", string(spec.IdentifierType))
		if err != nil {
			writeCredentialAuthLoginError(ctx, s.internalErrorSink, err)
			return
		}
		input.Secret.Value = secret
	} else if strings.TrimSpace(input.Secret.Value) == "" {
		secret, err := s.decryptCredentialAuthSecret(ctx, input.Secret, provider.ID, "password", string(spec.IdentifierType))
		if err != nil {
			writeCredentialAuthLoginError(ctx, s.internalErrorSink, err)
			return
		}
		input.Secret.Value = secret
	}
	if strings.TrimSpace(input.Secret.Value) == "" {
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
	if !s.requireCredentialAuthSecureTransport(ctx) {
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
	secretType := strings.TrimSpace(input.Secret.Type)
	if secretType == "" {
		secretType = "sms-otp"
	}
	if secretType != "sms-otp" || transactionID == "" {
		writePlatformError(ctx, errorcode.CodeAuthInvalidRequest)
		return
	}
	if !s.verifyCredentialAuthLoginChallenge(ctx, input.Challenge) {
		return
	}
	code := strings.TrimSpace(input.Secret.Code)
	if runtime.RequireEncryptedSecrets {
		if credentialAuthPlaintextSecretPresent(input.Secret) {
			writePlatformError(ctx, errorcode.CodeAuthInvalidRequest)
			return
		}
		secret, err := s.decryptCredentialAuthSecret(ctx, input.Secret, provider.ID, "sms-otp", string(spec.IdentifierType))
		if err != nil {
			writeCredentialAuthLoginError(ctx, s.internalErrorSink, err)
			return
		}
		code = secret
	} else if code == "" {
		secret, err := s.decryptCredentialAuthSecret(ctx, input.Secret, provider.ID, "sms-otp", string(spec.IdentifierType))
		if err != nil {
			writeCredentialAuthLoginError(ctx, s.internalErrorSink, err)
			return
		}
		code = secret
	}
	if strings.TrimSpace(code) == "" {
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

func (s *Server) credentialAuthChallengeStart(ctx *gin.Context) {
	var input credentialAuthChallengeStartRequest
	if err := ctx.ShouldBindJSON(&input); err != nil {
		writePlatformError(ctx, errorcode.CodeAuthInvalidRequest)
		return
	}
	if !s.enforceRateLimit(ctx, ratelimit.OperationCredentialChallenge, rateLimitClientIP(ctx), "credential-challenge") {
		return
	}
	if !s.requireCredentialAuthSecureTransport(ctx) {
		return
	}
	runtime := s.credentialAuth
	if runtime == nil || runtime.Service == nil {
		writePlatformError(ctx, errorcode.CodeAuthProviderNotConfigured)
		return
	}
	kind := credentialauth.ChallengeKind(strings.TrimSpace(input.Kind))
	if kind == "" {
		kind = credentialauth.ChallengeKindCaptcha
	}
	purpose := credentialauth.ChallengePurpose(strings.TrimSpace(input.Purpose))
	if purpose == "" {
		purpose = credentialauth.ChallengePurposeLogin
	}
	created, err := runtime.Service.CreateCredentialChallenge(ctx.Request.Context(), credentialauth.CreateCredentialChallengeInput{
		Kind:                  kind,
		Purpose:               purpose,
		ClientFingerprintHash: credentialAuthClientFingerprintHash(input.ClientFingerprint),
		TTL:                   runtime.ChallengeTTL,
	})
	if err != nil {
		writeCredentialAuthLoginError(ctx, s.internalErrorSink, err)
		return
	}
	response := credentialAuthChallengeStartResponse{
		ID:         created.ChallengeID,
		Kind:       string(created.Kind),
		Purpose:    string(created.Purpose),
		Prompt:     created.Prompt,
		Parameters: created.Parameters,
		ExpiresAt:  created.ExpiresAt,
	}
	if runtime.DebugCodeEnabled {
		response.DebugProof = created.Proof
	}
	ctx.JSON(http.StatusCreated, Response[credentialAuthChallengeStartResponse]{Data: response})
}

func (s *Server) verifyCredentialAuthLoginChallenge(ctx *gin.Context, input credentialAuthChallengeRequest) bool {
	challengeID := strings.TrimSpace(input.ID)
	kind := credentialauth.ChallengeKind(strings.TrimSpace(input.Kind))
	proof := strings.TrimSpace(input.Proof)
	if challengeID == "" && kind == "" && proof == "" && strings.TrimSpace(input.ClientFingerprint) == "" {
		return true
	}
	runtime := s.credentialAuth
	if runtime == nil || runtime.Service == nil || runtime.ChallengeProofHasher == nil {
		writePlatformError(ctx, errorcode.CodeAuthProviderNotConfigured)
		return false
	}
	if challengeID == "" || kind == "" || proof == "" {
		writePlatformError(ctx, errorcode.CodeAuthInvalidRequest)
		return false
	}
	answerDigest, err := runtime.ChallengeProofHasher.HashChallengeProof(kind, credentialauth.ChallengePurposeLogin, challengeID, proof)
	if err != nil {
		writeCredentialAuthLoginError(ctx, s.internalErrorSink, err)
		return false
	}
	if err := runtime.Service.VerifyCredentialChallenge(ctx.Request.Context(), credentialauth.CredentialChallengeProof{
		ChallengeID:           challengeID,
		Kind:                  kind,
		Purpose:               credentialauth.ChallengePurposeLogin,
		AnswerDigest:          answerDigest,
		ClientFingerprintHash: credentialAuthClientFingerprintHash(input.ClientFingerprint),
		MaxAttempts:           runtime.MaxChallengeAttempts,
	}); err != nil {
		writeCredentialAuthLoginError(ctx, s.internalErrorSink, err)
		return false
	}
	return true
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

func (s *Server) requireCredentialAuthSecureTransport(ctx *gin.Context) bool {
	if credentialAuthSecureTransport(ctx.Request, s.security) {
		return true
	}
	writePlatformError(ctx, errorcode.CodeAuthInvalidRequest)
	return false
}

func (s *Server) decryptCredentialAuthSecret(ctx *gin.Context, input credentialAuthSecretRequest, providerID string, secretType string, identifierType string) (string, error) {
	runtime := s.credentialAuth
	if runtime == nil || runtime.SecretTransport == nil {
		return "", credentialauth.ErrInvalidSecret
	}
	if input.Encrypted == nil {
		return "", credentialauth.ErrInvalidSecret
	}
	return runtime.SecretTransport.Decrypt(ctx.Request.Context(), *input.Encrypted, credentialAuthSecretAAD(providerID, secretType, identifierType))
}

func credentialAuthPlaintextSecretPresent(input credentialAuthSecretRequest) bool {
	return strings.TrimSpace(input.Value) != "" || strings.TrimSpace(input.Code) != ""
}

func credentialAuthSecretAAD(providerID string, secretType string, identifierType string) string {
	return strings.TrimSpace(providerID) + "\x00" + strings.TrimSpace(secretType) + "\x00" + strings.TrimSpace(identifierType)
}

func credentialAuthSecureTransport(request *http.Request, security SecurityOptions) bool {
	if request == nil {
		return false
	}
	if requestUsesHTTPS(request, trustedProxyPrefixes(security.TrustedProxies)) {
		return true
	}
	peer, ok := directPeerAddress(request.RemoteAddr)
	return ok && peer.IsLoopback()
}

func credentialAuthClientFingerprintHash(fingerprint string) string {
	fingerprint = strings.TrimSpace(fingerprint)
	if fingerprint == "" {
		return ""
	}
	digest := sha256.Sum256([]byte(fmt.Sprintf("platform-go credential-auth challenge fingerprint v1\x00%d:%s", len(fingerprint), fingerprint)))
	return "v1:sha256:client-fingerprint:" + hex.EncodeToString(digest[:])
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
	case errors.Is(err, credentialauth.ErrInvalidInput),
		errors.Is(err, credentialauth.ErrInvalidSecret):
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
