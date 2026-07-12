package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"platform-go/internal/platform/adminresource"
	"platform-go/internal/platform/authjwt"
	"platform-go/internal/platform/cache"
	"platform-go/internal/platform/capability"
	"platform-go/internal/platform/rbac"
	"platform-go/internal/platform/session"
	"platform-go/internal/platform/storage"
)

type ServerOptions struct {
	Capabilities            []capability.Manifest
	Resources               *adminresource.Store
	Sessions                *session.Store
	Cache                   cache.Store
	InvalidationBus         cache.InvalidationBus
	CacheTTL                time.Duration
	FileStorage             storage.ObjectStore
	UploadPolicy            UploadPolicy
	InternalErrorSink       InternalErrorSink
	FileCleanupSink         FileCleanupSink
	AdminRoutes             []AdminRouteRegistration
	AppRoutes               []AppRouteRegistration
	AdminIdentityResolver   AdminIdentityResolver
	AdminIdentityBindings   AdminIdentityBindingStore
	AppIdentityResolver     AppIdentityResolver
	AppIdentityBindings     AppIdentityBindingStore
	PhoneProtector          PhoneProtector
	PhoneVerificationSender PhoneVerificationSender
	DebugCodeEnabled        bool
	SessionTTL              time.Duration
	JWTSecret               string
	OpenAPIDocument         []byte
	TokenService            *authjwt.Service
	Now                     func() time.Time
	Authorizer              Authorizer
	AllowInsecureHeaderAuth bool
	DisableDemoAuthProvider bool
}

type Authorizer interface {
	Can(user string, tenant string, permission string, action string) bool
}

type InternalErrorEvent struct {
	Code string
	Err  error
}

type InternalErrorSink interface {
	Record(context.Context, InternalErrorEvent)
}

type FileCleanupRecord struct {
	ObjectIdentifier string
	ReasonCode       string
	RetryStatus      string
	CreatedAt        time.Time
}

type FileCleanupSink interface {
	Record(context.Context, FileCleanupRecord) error
}

type Server struct {
	router                  *gin.Engine
	capabilities            []capability.Manifest
	resources               *adminresource.Store
	sessions                *session.Store
	cache                   cache.Store
	invalidationBus         cache.InvalidationBus
	cacheStats              cache.StatsProvider
	cacheTTL                time.Duration
	fileStorage             storage.ObjectStore
	uploadPolicy            UploadPolicy
	internalErrorSink       InternalErrorSink
	fileCleanupSink         FileCleanupSink
	appRoutes               map[appRouteKey]gin.HandlerFunc
	adminIdentityResolver   AdminIdentityResolver
	adminIdentityBindings   AdminIdentityBindingStore
	appIdentityResolver     AppIdentityResolver
	appIdentityBindings     AppIdentityBindingStore
	phoneProtector          PhoneProtector
	phoneVerificationSender PhoneVerificationSender
	debugCodeEnabled        bool
	tokens                  *authjwt.Service
	now                     func() time.Time
	openAPIDocument         []byte
	authorizer              Authorizer
	policyMu                sync.Mutex
	policyAuthorizer        Authorizer
	allowInsecureHeaderAuth bool
	disableDemoAuthProvider bool
}

const (
	cacheKeyBranding            = "admin:branding"
	cacheKeyAuthProviders       = "admin:auth-providers"
	cacheKeyMenusPrefix         = "admin:menus:"
	cacheKeyPrincipalPrefix     = "admin:principal:"
	cacheKeySchemaPrefix        = "admin:schema:"
	cacheKeyPermissionsList     = "admin:permissions:list"
	defaultPlatformCacheTTL     = 5 * time.Minute
	defaultJWTSecret            = "dev-platform-go-secret"
	platformTenant              = "platform"
	appTenant                   = "app"
	apiTokensResource           = "api-tokens"
	apiTokenPrefix              = "pgo_"
	sessionInvalidationResource = "sessions"
	appLogoutResolveErrorKey    = "platform.app.logout.session.resolve-error"
)

func New(options ServerOptions) *Server {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	resources := options.Resources
	if resources == nil {
		resources = adminresource.NewStoreFromCapabilities(options.Capabilities)
	}
	sessions := options.Sessions
	if sessions == nil {
		sessions = session.NewStore(session.Options{TTL: options.SessionTTL, Now: options.Now})
	}
	cacheStore := options.Cache
	if cacheStore == nil {
		cacheStore = cache.NewMeteredStore("noop", cache.NewNoopStore())
	}
	cacheStats, ok := cacheStore.(cache.StatsProvider)
	if !ok {
		cacheStore = cache.NewMeteredStore("custom", cacheStore)
		cacheStats = cacheStore.(cache.StatsProvider)
	}
	cacheTTL := options.CacheTTL
	if cacheTTL <= 0 {
		cacheTTL = defaultPlatformCacheTTL
	}
	fileStorage := options.FileStorage
	if fileStorage == nil {
		fileStorage = storage.NewLocalObjectStore(storage.LocalObjectStoreOptions{})
	}
	tokens := options.TokenService
	if tokens == nil {
		jwtSecret := strings.TrimSpace(options.JWTSecret)
		if jwtSecret == "" {
			jwtSecret = defaultJWTSecret
		}
		tokens = authjwt.NewService(jwtSecret, options.Now)
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	fileCleanupSink := options.FileCleanupSink
	if fileCleanupSink == nil {
		fileCleanupSink = resourceFileCleanupSink{resources: resources}
	}
	adminIdentityBindings := options.AdminIdentityBindings
	if adminIdentityBindings == nil {
		adminIdentityBindings = NewResourceAdminIdentityBindingStore(resources, now)
	}
	appIdentityBindings := options.AppIdentityBindings
	if appIdentityBindings == nil {
		appIdentityBindings = newResourceAppIdentityBindingStore(resources, now)
	}
	server := &Server{
		router:                  router,
		capabilities:            options.Capabilities,
		resources:               resources,
		sessions:                sessions,
		cache:                   cacheStore,
		invalidationBus:         options.InvalidationBus,
		cacheStats:              cacheStats,
		cacheTTL:                cacheTTL,
		fileStorage:             fileStorage,
		uploadPolicy:            normalizeUploadPolicy(options.UploadPolicy),
		internalErrorSink:       options.InternalErrorSink,
		fileCleanupSink:         fileCleanupSink,
		adminIdentityResolver:   options.AdminIdentityResolver,
		adminIdentityBindings:   adminIdentityBindings,
		appIdentityResolver:     options.AppIdentityResolver,
		appIdentityBindings:     appIdentityBindings,
		phoneProtector:          options.PhoneProtector,
		phoneVerificationSender: options.PhoneVerificationSender,
		debugCodeEnabled:        options.DebugCodeEnabled,
		tokens:                  tokens,
		now:                     now,
		openAPIDocument:         append([]byte(nil), options.OpenAPIDocument...),
		authorizer:              options.Authorizer,
		allowInsecureHeaderAuth: options.AllowInsecureHeaderAuth,
		disableDemoAuthProvider: options.DisableDemoAuthProvider,
	}
	server.appRoutes = server.defaultAppRouteHandlers(options.AppRoutes)
	server.subscribeInvalidations()
	server.routes(options.AdminRoutes)
	return server
}

func (s *Server) Router() *gin.Engine {
	return s.router
}

func (s *Server) Run(addr string) error {
	return s.router.Run(addr)
}

func (s *Server) routes(adminRoutes []AdminRouteRegistration) {
	api := s.router.Group("/api")
	api.GET("/health", s.health)
	api.GET("/openapi.json", s.openapi)
	api.GET("/capabilities", s.capabilitiesList)
	api.GET("/platform/branding", s.platformBranding)
	api.GET("/platform/cache/stats", s.platformCacheStats)
	api.GET("/auth/providers", s.authProviders)
	api.POST("/auth/providers/:provider/start", s.authProviderStart)
	api.POST("/auth/login", s.authLogin)
	api.POST("/auth/refresh", s.authRefresh)
	api.POST("/auth/logout", s.authLogout)
	s.registerManifestAppRoutes(api)
	api.GET("/admin/session/current", s.adminCurrentSession)
	api.GET("/admin/menus", s.adminMenus)
	api.GET("/admin/demo-data", s.adminDemoDataList)
	api.POST("/admin/demo-data/:capability/:dataset/apply", s.adminDemoDataApply)
	api.GET("/admin/policy-reviews/export", s.adminPolicyReviewExport)
	api.POST("/admin/policy-reviews/:id/request", s.adminPolicyReviewRequest)
	api.POST("/admin/policy-reviews/:id/approve", s.adminPolicyReviewApprove)
	api.POST("/admin/policy-reviews/:id/reject", s.adminPolicyReviewReject)
	api.POST("/admin/files/upload", s.adminFileUpload)
	api.GET("/admin/files/:id/content", s.adminFileContent)
	adminResources := api.Group("/admin/resources")
	adminResources.GET("/:resource/schema", s.adminResourceSchema)
	adminResources.POST("/:resource/query", s.adminResourceQuery)
	adminResources.GET("/:resource", s.adminResourceList)
	adminResources.POST("/:resource", s.adminResourceCreate)
	adminResources.PUT("/:resource/:id", s.adminResourceUpdate)
	adminResources.DELETE("/:resource/:id", s.adminResourceDelete)
	s.registerAdminRoutes(api, adminRoutes)
}

func (s *Server) health(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, Response[gin.H]{Data: gin.H{"ok": true, "service": "platform-go"}})
}

func (s *Server) openapi(ctx *gin.Context) {
	if !s.authorize(ctx, "admin:api-docs:read") {
		return
	}
	if len(s.openAPIDocument) == 0 {
		ctx.JSON(http.StatusNotFound, Response[gin.H]{
			Error: &ErrorBody{Code: "OPENAPI_NOT_CONFIGURED", Message: "openapi document is not configured"},
		})
		return
	}
	ctx.Data(http.StatusOK, "application/json; charset=utf-8", s.openAPIDocument)
}

func (s *Server) capabilitiesList(ctx *gin.Context) {
	if !s.authorize(ctx, "admin:capability:read") {
		return
	}
	type item struct {
		ID      capability.ID `json:"id"`
		Name    string        `json:"name"`
		Version string        `json:"version"`
	}
	items := make([]item, 0, len(s.capabilities))
	for _, manifest := range s.capabilities {
		items = append(items, item{ID: manifest.ID, Name: manifest.Name, Version: manifest.Version})
	}
	ctx.JSON(http.StatusOK, Response[[]item]{Data: items})
}

func (s *Server) platformBranding(ctx *gin.Context) {
	branding := cachedJSONValue(ctx.Request.Context(), s.cache, cacheKeyBranding, s.cacheTTL, s.resources.BrandingConfig)
	ctx.JSON(http.StatusOK, Response[adminresource.BrandingConfig]{Data: branding})
}

func (s *Server) platformCacheStats(ctx *gin.Context) {
	if !s.authorize(ctx, "admin:monitoring:read") {
		return
	}
	ctx.JSON(http.StatusOK, Response[cache.Stats]{Data: s.cacheStats.Stats()})
}

type adminMenuListResponse struct {
	Items []adminresource.MenuItem `json:"items"`
}

type adminResourceListResponse struct {
	Resource string                 `json:"resource"`
	Items    []adminresource.Record `json:"items"`
}

type adminResourceQueryResponse struct {
	Resource string                 `json:"resource"`
	Items    []adminresource.Record `json:"items"`
	Total    int                    `json:"total"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"pageSize"`
}

type adminResourceRecordResponse struct {
	Resource string               `json:"resource"`
	Record   adminresource.Record `json:"record"`
	Token    string               `json:"token,omitempty"`
}

type policyReviewApproveResponse struct {
	Review adminresource.Record `json:"review"`
	Role   adminresource.Record `json:"role"`
	Audit  adminresource.Record `json:"audit,omitempty"`
}

type policyReviewActionResponse struct {
	Review adminresource.Record `json:"review"`
	Audit  adminresource.Record `json:"audit,omitempty"`
}

type policyReviewExportResponse struct {
	ExportedBy string                 `json:"exportedBy"`
	ExportedAt string                 `json:"exportedAt"`
	Reviews    []adminresource.Record `json:"reviews"`
	Audits     []adminresource.Record `json:"audits"`
}

type policyReviewRejectRequest struct {
	Reason string `json:"reason"`
}

type adminDemoDataListResponse struct {
	Items []adminDemoDataItem `json:"items"`
}

type adminDemoDataItem struct {
	ID           string                      `json:"id"`
	CapabilityID capability.ID               `json:"capabilityId"`
	Resource     string                      `json:"resource"`
	Title        adminresource.LocalizedText `json:"title"`
	Description  adminresource.LocalizedText `json:"description"`
	Records      int                         `json:"records"`
}

type adminDemoDataApplyResponse struct {
	ID           string        `json:"id"`
	CapabilityID capability.ID `json:"capabilityId"`
	Resource     string        `json:"resource"`
	Applied      int           `json:"applied"`
}

type authProviderListResponse struct {
	Items []capability.AuthProvider `json:"items"`
}

type authLoginRequest struct {
	Provider     string `json:"provider"`
	Username     string `json:"username"`
	Code         string `json:"code"`
	State        string `json:"state"`
	CodeVerifier string `json:"codeVerifier"`
}

type authLoginResponse struct {
	Token     string         `json:"token"`
	ExpiresAt time.Time      `json:"expiresAt"`
	Principal rbac.Principal `json:"principal"`
}

type authProviderStartRequest struct {
	CodeChallenge string `json:"codeChallenge"`
}

type authProviderStartResponse struct {
	AuthorizationURL string    `json:"authorizationUrl"`
	State            string    `json:"state"`
	ExpiresAt        time.Time `json:"expiresAt"`
}

type appLoginRequest struct {
	Provider string `json:"provider"`
	Username string `json:"username"`
	Code     string `json:"code"`
}

type appLoginResponse struct {
	Token     string             `json:"token"`
	ExpiresAt time.Time          `json:"expiresAt"`
	Session   appSessionResponse `json:"session"`
}

type appSessionResponse struct {
	UserID    string `json:"userId"`
	Username  string `json:"username"`
	TenantID  string `json:"tenantId"`
	SessionID string `json:"sessionId"`
}

func (s *Server) authProviders(ctx *gin.Context) {
	providers := cachedJSONValue(ctx.Request.Context(), s.cache, cacheKeyAuthProviders, s.cacheTTL, func() []capability.AuthProvider {
		providers := make([]capability.AuthProvider, 0)
		for _, manifest := range s.capabilities {
			for _, provider := range manifest.AuthProviders {
				if !s.authProviderAvailable(provider) || !provider.SupportsAudience(capability.AuthProviderAudienceAdmin) {
					continue
				}
				providers = append(providers, provider)
			}
		}
		return providers
	})
	ctx.JSON(http.StatusOK, Response[authProviderListResponse]{Data: authProviderListResponse{Items: providers}})
}

func (s *Server) authProviderStart(ctx *gin.Context) {
	var input authProviderStartRequest
	if err := ctx.ShouldBindJSON(&input); err != nil {
		writeAuthError(ctx, http.StatusBadRequest, "AUTH_PROVIDER_START_INVALID", "invalid auth provider start request")
		return
	}
	provider, ok := s.findAuthProvider(ctx.Param("provider"), capability.AuthProviderAudienceAdmin)
	if !ok {
		writeAuthError(ctx, http.StatusBadRequest, "AUTH_PROVIDER_NOT_FOUND", "auth provider not found")
		return
	}
	if !provider.Configured {
		writeAuthError(ctx, http.StatusBadRequest, "AUTH_PROVIDER_NOT_CONFIGURED", "auth provider is not configured")
		return
	}
	if provider.Kind != "oidc" {
		writeAuthError(ctx, http.StatusBadRequest, "AUTH_PROVIDER_UNSUPPORTED", "auth provider is not supported")
		return
	}
	if s.adminIdentityResolver == nil {
		writeAuthError(ctx, http.StatusNotImplemented, "AUTH_PROVIDER_RESOLVER_NOT_CONFIGURED", "auth provider resolver is not configured")
		return
	}
	started, err := s.adminIdentityResolver.StartAdminIdentity(ctx.Request.Context(), AdminIdentityStartInput{
		Provider:      provider,
		CodeChallenge: strings.TrimSpace(input.CodeChallenge),
	})
	if errors.Is(err, ErrAdminIdentityInvalid) || errors.Is(err, ErrAdminIdentityTransaction) {
		writeAuthError(ctx, http.StatusBadRequest, "AUTH_PROVIDER_START_INVALID", "invalid auth provider start request")
		return
	}
	if err != nil {
		writeAuthError(ctx, http.StatusBadGateway, "AUTH_PROVIDER_START_FAILED", "auth provider start failed")
		return
	}
	ctx.JSON(http.StatusOK, Response[authProviderStartResponse]{Data: authProviderStartResponse{
		AuthorizationURL: started.AuthorizationURL,
		State:            started.State,
		ExpiresAt:        started.ExpiresAt,
	}})
}

func (s *Server) authLogin(ctx *gin.Context) {
	var input authLoginRequest
	if err := ctx.ShouldBindJSON(&input); err != nil {
		writeAuthError(ctx, http.StatusBadRequest, "AUTH_INVALID_REQUEST", "invalid auth login request")
		return
	}
	provider, ok := s.findAuthProvider(input.Provider, capability.AuthProviderAudienceAdmin)
	if !ok {
		writeAuthError(ctx, http.StatusBadRequest, "AUTH_PROVIDER_NOT_FOUND", "auth provider not found")
		return
	}
	if !provider.Configured {
		writeAuthError(ctx, http.StatusBadRequest, "AUTH_PROVIDER_NOT_CONFIGURED", "auth provider is not configured")
		return
	}
	switch provider.Kind {
	case "demo":
		username := strings.TrimSpace(input.Username)
		if username == "" {
			username = "admin"
		}
		principal, err := adminresource.ValidateAdminPrincipal(s.resources, username)
		if err != nil {
			writeAuthError(ctx, http.StatusUnauthorized, "AUTH_INVALID_CREDENTIALS", "invalid demo account")
			return
		}
		s.issueAdminLogin(ctx, principal, provider)
	case "oidc":
		if strings.TrimSpace(input.Code) == "" || strings.TrimSpace(input.State) == "" || strings.TrimSpace(input.CodeVerifier) == "" {
			writeAuthError(ctx, http.StatusBadRequest, "AUTH_IDENTITY_TRANSACTION_REQUIRED", "auth identity transaction is required")
			return
		}
		if s.adminIdentityResolver == nil {
			writeAuthError(ctx, http.StatusNotImplemented, "AUTH_PROVIDER_RESOLVER_NOT_CONFIGURED", "auth provider resolver is not configured")
			return
		}
		identity, err := s.adminIdentityResolver.ResolveAdminIdentity(ctx.Request.Context(), AdminIdentityResolveInput{
			Provider:     provider,
			Code:         strings.TrimSpace(input.Code),
			State:        strings.TrimSpace(input.State),
			CodeVerifier: strings.TrimSpace(input.CodeVerifier),
		})
		if errors.Is(err, ErrAdminIdentityInvalid) {
			writeAuthError(ctx, http.StatusBadRequest, "AUTH_IDENTITY_INVALID", "invalid admin identity")
			return
		}
		if errors.Is(err, ErrAdminIdentityTransaction) {
			writeAuthError(ctx, http.StatusUnauthorized, "AUTH_IDENTITY_TRANSACTION_INVALID", "invalid admin identity transaction")
			return
		}
		if err != nil {
			writeAuthError(ctx, http.StatusBadGateway, "AUTH_PROVIDER_RESOLVE_FAILED", "auth provider resolve failed")
			return
		}
		binding, err := s.adminIdentityBindings.ResolveAdminIdentityBinding(ctx.Request.Context(), AdminIdentityBindingInput{
			Provider:        provider,
			Issuer:          identity.Issuer,
			ProviderSubject: identity.ProviderSubject,
			Now:             s.now().UTC(),
		})
		if errors.Is(err, ErrAdminIdentityBindingInvalid) {
			writeAuthError(ctx, http.StatusUnauthorized, "AUTH_IDENTITY_NOT_BOUND", "admin identity is not bound")
			return
		}
		if err != nil {
			writeAuthError(ctx, http.StatusInternalServerError, "AUTH_IDENTITY_BINDING_FAILED", "admin identity binding failed")
			return
		}
		principal, err := adminresource.ValidateAdminPrincipal(s.resources, binding.Username)
		if err != nil {
			writeAuthError(ctx, http.StatusUnauthorized, "AUTH_INVALID_CREDENTIALS", "invalid admin account")
			return
		}
		s.issueAdminLogin(ctx, principal, provider)
	default:
		writeAuthError(ctx, http.StatusBadRequest, "AUTH_PROVIDER_UNSUPPORTED", "auth provider is not supported")
	}
}

func (s *Server) issueAdminLogin(ctx *gin.Context, principal rbac.Principal, provider capability.AuthProvider) {
	issued, err := s.sessions.Issue(principal.User.Username)
	if err != nil {
		writeAuthError(ctx, http.StatusInternalServerError, "AUTH_SESSION_ISSUE_FAILED", "session issue failed")
		return
	}
	s.publishSessionInvalidation(ctx.Request.Context())
	token, _, err := s.tokens.Sign(authjwt.Subject{
		UserID:    principal.User.ID,
		TenantID:  platformTenant,
		Username:  principal.User.Username,
		SessionID: issued.Token,
		TokenType: authjwt.TokenTypeAdmin,
	}, issued.ExpiresAt.Sub(issued.IssuedAt))
	if err != nil {
		if !s.cleanupIssuedAdminSession(ctx.Request.Context(), issued.Token) {
			writeAuthError(ctx, http.StatusInternalServerError, "AUTH_SESSION_CLEANUP_FAILED", "session cleanup failed")
			return
		}
		writeAuthError(ctx, http.StatusInternalServerError, "AUTH_TOKEN_SIGN_FAILED", "auth token sign failed")
		return
	}
	if err := s.recordAudit("auth.login", "Auth Login", principal.User.Username, provider.ID); err != nil {
		if !s.cleanupIssuedAdminSession(ctx.Request.Context(), issued.Token) {
			writeAuthError(ctx, http.StatusInternalServerError, "AUTH_SESSION_CLEANUP_FAILED", "session cleanup failed")
			return
		}
		writeAuthError(ctx, http.StatusInternalServerError, "AUTH_AUDIT_FAILED", "auth audit failed")
		return
	}
	ctx.JSON(http.StatusOK, Response[authLoginResponse]{
		Data: authLoginResponse{Token: token, ExpiresAt: issued.ExpiresAt, Principal: principal},
	})
}

func (s *Server) cleanupIssuedAdminSession(ctx context.Context, token string) bool {
	revoked, err := s.sessions.RevokeContext(context.WithoutCancel(ctx), token)
	if err != nil || !revoked {
		return false
	}
	s.publishSessionInvalidation(context.WithoutCancel(ctx))
	return true
}

func (s *Server) authRefresh(ctx *gin.Context) {
	authSession, ok, err := s.authSessionFromBearerContext(ctx)
	if err != nil {
		writeAuthError(ctx, http.StatusInternalServerError, "AUTH_SESSION_RENEW_FAILED", "session renewal failed")
		return
	}
	if !ok {
		writeUnauthorized(ctx)
		return
	}
	renewed, ok, err := s.sessions.RenewContext(ctx.Request.Context(), authSession.Token)
	if err != nil {
		writeAuthError(ctx, http.StatusInternalServerError, "AUTH_SESSION_RENEW_FAILED", "session renewal failed")
		return
	}
	if !ok {
		writeUnauthorized(ctx)
		return
	}
	s.publishSessionInvalidation(ctx.Request.Context())
	principal := s.currentPrincipalForUsername(ctx.Request.Context(), renewed.Username)
	if principal.User.Username == "" || len(principal.Permissions) == 0 {
		writeUnauthorized(ctx)
		return
	}
	tokenTTL := renewed.ExpiresAt.Sub(s.now().UTC())
	token, _, err := s.tokens.Sign(authjwt.Subject{
		UserID:    principal.User.ID,
		TenantID:  platformTenant,
		Username:  principal.User.Username,
		SessionID: renewed.Token,
		TokenType: authjwt.TokenTypeAdmin,
	}, tokenTTL)
	if err != nil {
		writeAuthError(ctx, http.StatusInternalServerError, "AUTH_TOKEN_SIGN_FAILED", "auth token sign failed")
		return
	}
	if err := s.recordAudit("auth.refresh", "Auth Refresh", principal.User.Username, ""); err != nil {
		writeAuthError(ctx, http.StatusInternalServerError, "AUTH_AUDIT_FAILED", "auth audit failed")
		return
	}
	ctx.JSON(http.StatusOK, Response[authLoginResponse]{
		Data: authLoginResponse{Token: token, ExpiresAt: renewed.ExpiresAt, Principal: principal},
	})
}

func (s *Server) authLogout(ctx *gin.Context) {
	authSession, ok, err := s.authSessionFromBearerContext(ctx)
	if err != nil {
		writeAuthError(ctx, http.StatusInternalServerError, "AUTH_SESSION_REVOKE_FAILED", "session revoke failed")
		return
	}
	if !ok {
		writeUnauthorized(ctx)
		return
	}
	revoked, err := s.sessions.RevokeContext(ctx.Request.Context(), authSession.Token)
	if err != nil {
		writeAuthError(ctx, http.StatusInternalServerError, "AUTH_SESSION_REVOKE_FAILED", "session revoke failed")
		return
	}
	if !revoked {
		writeUnauthorized(ctx)
		return
	}
	s.publishSessionInvalidation(ctx.Request.Context())
	if err := s.recordAudit("auth.logout", "Auth Logout", authSession.Username, ""); err != nil {
		writeAuthError(ctx, http.StatusInternalServerError, "AUTH_AUDIT_FAILED", "auth audit failed")
		return
	}
	ctx.JSON(http.StatusOK, Response[gin.H]{Data: gin.H{"revoked": true}})
}

func (s *Server) appAuthLogin(ctx *gin.Context) {
	var input appLoginRequest
	if err := ctx.ShouldBindJSON(&input); err != nil {
		writeAuthError(ctx, http.StatusBadRequest, "APP_AUTH_INVALID_REQUEST", "invalid app auth login request")
		return
	}
	username, providerID, ok := s.resolveAppLoginIdentity(ctx, input)
	if !ok {
		return
	}
	issued, err := s.sessions.Issue(username)
	if err != nil {
		writeAuthError(ctx, http.StatusInternalServerError, "APP_AUTH_SESSION_ISSUE_FAILED", "app session issue failed")
		return
	}
	s.publishSessionInvalidation(ctx.Request.Context())
	token, _, err := s.tokens.Sign(authjwt.Subject{
		UserID:    appUserID(username),
		TenantID:  appTenant,
		Username:  username,
		SessionID: issued.Token,
		TokenType: authjwt.TokenTypeApp,
	}, issued.ExpiresAt.Sub(issued.IssuedAt))
	if err != nil {
		writeAuthError(ctx, http.StatusInternalServerError, "APP_AUTH_TOKEN_SIGN_FAILED", "app auth token sign failed")
		return
	}
	if err := s.recordAudit("app.auth.login", "App Auth Login", username, providerID); err != nil {
		writeAuthError(ctx, http.StatusInternalServerError, "APP_AUTH_AUDIT_FAILED", "app auth audit failed")
		return
	}
	ctx.JSON(http.StatusOK, Response[appLoginResponse]{
		Data: appLoginResponse{
			Token:     token,
			ExpiresAt: issued.ExpiresAt,
			Session:   appSessionResponseFromSession(issued),
		},
	})
}

func (s *Server) resolveAppLoginIdentity(ctx *gin.Context, input appLoginRequest) (string, string, bool) {
	providerID := strings.TrimSpace(input.Provider)
	if providerID == "" {
		return appUsername(input.Username), "", true
	}
	provider, ok := s.findAuthProvider(providerID, capability.AuthProviderAudienceApp)
	if !ok || !provider.Enabled {
		writeAuthError(ctx, http.StatusBadRequest, "APP_AUTH_PROVIDER_NOT_FOUND", "app auth provider not found")
		return "", "", false
	}
	if !provider.Configured {
		writeAuthError(ctx, http.StatusBadRequest, "APP_AUTH_PROVIDER_NOT_CONFIGURED", "app auth provider is not configured")
		return "", "", false
	}
	if provider.Kind == "wechat" && strings.TrimSpace(input.Code) == "" {
		writeAuthError(ctx, http.StatusBadRequest, "APP_AUTH_CODE_REQUIRED", "app auth code is required")
		return "", "", false
	}
	if s.appIdentityResolver == nil {
		writeAuthError(ctx, http.StatusNotImplemented, "APP_AUTH_PROVIDER_RESOLVER_NOT_CONFIGURED", "app auth provider resolver is not configured")
		return "", "", false
	}
	identity, err := s.appIdentityResolver.ResolveAppIdentity(ctx.Request.Context(), AppIdentityResolveInput{
		Provider:     provider,
		Code:         strings.TrimSpace(input.Code),
		UsernameHint: strings.TrimSpace(input.Username),
	})
	if errors.Is(err, ErrAppIdentityInvalid) {
		writeAuthError(ctx, http.StatusUnauthorized, "APP_AUTH_IDENTITY_INVALID", "invalid app identity")
		return "", "", false
	}
	if err != nil {
		writeAuthError(ctx, http.StatusBadGateway, "APP_AUTH_PROVIDER_RESOLVE_FAILED", "app auth provider resolve failed")
		return "", "", false
	}
	if strings.TrimSpace(identity.ProviderSubject) == "" {
		writeAuthError(ctx, http.StatusUnauthorized, "APP_AUTH_IDENTITY_INVALID", "invalid app identity")
		return "", "", false
	}
	binding, err := s.appIdentityBindings.ResolveAppIdentityBinding(ctx.Request.Context(), AppIdentityBindingInput{
		Provider:        provider,
		ProviderSubject: strings.TrimSpace(identity.ProviderSubject),
		UsernameHint:    strings.TrimSpace(identity.Username),
		Now:             s.now(),
	})
	if errors.Is(err, ErrAppIdentityInvalid) {
		writeAuthError(ctx, http.StatusUnauthorized, "APP_AUTH_IDENTITY_INVALID", "invalid app identity")
		return "", "", false
	}
	if err != nil {
		writeAuthError(ctx, http.StatusInternalServerError, "APP_AUTH_IDENTITY_BINDING_FAILED", "app identity binding failed")
		return "", "", false
	}
	if strings.TrimSpace(binding.Username) == "" {
		writeAuthError(ctx, http.StatusUnauthorized, "APP_AUTH_IDENTITY_INVALID", "invalid app identity")
		return "", "", false
	}
	return appUsername(binding.Username), provider.ID, true
}

func (s *Server) appAuthLogout(ctx *gin.Context) {
	appSession, ok, err := s.appSessionFromBearerContext(ctx)
	if err != nil {
		writeAuthError(ctx, http.StatusInternalServerError, "APP_AUTH_SESSION_REVOKE_FAILED", "app session revoke failed")
		return
	}
	if !ok {
		writeUnauthorized(ctx)
		return
	}
	revoked, err := s.sessions.RevokeContext(ctx.Request.Context(), appSession.Token)
	if err != nil {
		writeAuthError(ctx, http.StatusInternalServerError, "APP_AUTH_SESSION_REVOKE_FAILED", "app session revoke failed")
		return
	}
	if !revoked {
		writeUnauthorized(ctx)
		return
	}
	s.publishSessionInvalidation(ctx.Request.Context())
	if err := s.recordAudit("app.auth.logout", "App Auth Logout", appSession.Username, ""); err != nil {
		writeAuthError(ctx, http.StatusInternalServerError, "APP_AUTH_AUDIT_FAILED", "app auth audit failed")
		return
	}
	ctx.JSON(http.StatusOK, Response[gin.H]{Data: gin.H{"revoked": true}})
}

func (s *Server) appCurrentSession(ctx *gin.Context) {
	appSession, ok := s.appSessionFromBearer(ctx)
	if !ok {
		writeUnauthorized(ctx)
		return
	}
	ctx.JSON(http.StatusOK, Response[appSessionResponse]{Data: appSessionResponseFromSession(appSession)})
}

func (s *Server) adminResourceSchema(ctx *gin.Context) {
	resource := ctx.Param("resource")
	schema, err := cachedJSONResult(ctx.Request.Context(), s.cache, cacheKeySchemaPrefix+resource, s.cacheTTL, func() (adminresource.Schema, error) {
		return s.resources.Schema(resource)
	})
	if err != nil {
		writeAdminResourceError(ctx, err)
		return
	}
	if !s.authorize(ctx, schema.Permissions.Read) {
		return
	}
	ctx.JSON(http.StatusOK, Response[adminresource.Schema]{Data: schema})
}

func (s *Server) adminResourceList(ctx *gin.Context) {
	resource := ctx.Param("resource")
	principal, hasPrincipal, ok := s.authorizeAdminResourcePrincipal(ctx, resource, "read")
	if !ok {
		return
	}
	if resource == "permissions" {
		items, err := cachedJSONResult(ctx.Request.Context(), s.cache, cacheKeyPermissionsList, s.cacheTTL, func() ([]adminresource.Record, error) {
			return s.resources.List(resource)
		})
		if err != nil {
			writeAdminResourceError(ctx, err)
			return
		}
		items, err = s.projectAdminResourceRecords(resource, items, adminresource.ProjectionResponse)
		if err != nil {
			writeAdminResourceError(ctx, err)
			return
		}
		ctx.JSON(http.StatusOK, Response[adminResourceListResponse]{
			Data: adminResourceListResponse{Resource: resource, Items: items},
		})
		return
	}
	var (
		items []adminresource.Record
		err   error
	)
	if hasPrincipal {
		items, err = s.resources.ListForPrincipal(resource, principal)
	} else {
		items, err = s.resources.List(resource)
	}
	if err != nil {
		writeAdminResourceError(ctx, err)
		return
	}
	items, err = s.projectAdminResourceRecords(resource, items, adminresource.ProjectionResponse)
	if err != nil {
		writeAdminResourceError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, Response[adminResourceListResponse]{
		Data: adminResourceListResponse{Resource: resource, Items: items},
	})
}

func (s *Server) adminResourceQuery(ctx *gin.Context) {
	resource := ctx.Param("resource")
	principal, hasPrincipal, ok := s.authorizeAdminResourcePrincipal(ctx, resource, "read")
	if !ok {
		return
	}
	var input adminresource.QueryInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		writeAdminResourceError(ctx, adminresource.ErrInvalidRecord)
		return
	}
	var (
		result adminresource.QueryResult
		err    error
	)
	if hasPrincipal {
		result, err = s.resources.QueryForPrincipal(resource, input, principal)
	} else {
		result, err = s.resources.Query(resource, input)
	}
	if err != nil {
		writeAdminResourceError(ctx, err)
		return
	}
	items, err := s.projectAdminResourceRecords(resource, result.Items, adminresource.ProjectionResponse)
	if err != nil {
		writeAdminResourceError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, Response[adminResourceQueryResponse]{
		Data: adminResourceQueryResponse{
			Resource: result.Resource,
			Items:    items,
			Total:    result.Total,
			Page:     result.Page,
			PageSize: result.PageSize,
		},
	})
}

func (s *Server) adminResourceCreate(ctx *gin.Context) {
	resource := ctx.Param("resource")
	if !s.authorizeAdminResource(ctx, resource, "create") {
		return
	}
	var input adminresource.WriteInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		writeAdminResourceError(ctx, adminresource.ErrInvalidRecord)
		return
	}
	if resource == apiTokensResource {
		issued, token, err := s.issueAdminAPIToken(ctx.Request.Context(), s.currentActor(ctx), input)
		if err != nil {
			writeAdminResourceError(ctx, err)
			return
		}
		s.invalidateCachesForResource(ctx.Request.Context(), resource)
		projected, err := s.resources.ProjectRecord(resource, issued, adminresource.ProjectionResponse)
		if err != nil {
			writeAdminResourceError(ctx, err)
			return
		}
		ctx.JSON(http.StatusCreated, Response[adminResourceRecordResponse]{
			Data: adminResourceRecordResponse{Resource: resource, Record: projected, Token: token},
		})
		return
	}
	record, err := s.resources.Create(resource, input)
	if err != nil {
		writeAdminResourceError(ctx, err)
		return
	}
	s.recordAdminResourceAudit(ctx, "create", resource, record)
	s.invalidateCachesForResource(ctx.Request.Context(), resource)
	projected, err := s.resources.ProjectRecord(resource, record, adminresource.ProjectionResponse)
	if err != nil {
		writeAdminResourceError(ctx, err)
		return
	}
	ctx.JSON(http.StatusCreated, Response[adminResourceRecordResponse]{
		Data: adminResourceRecordResponse{Resource: resource, Record: projected},
	})
}

func (s *Server) adminResourceUpdate(ctx *gin.Context) {
	resource := ctx.Param("resource")
	id := ctx.Param("id")
	if !s.authorizeAdminResource(ctx, resource, "update") {
		return
	}
	var input adminresource.WriteInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		writeAdminResourceError(ctx, adminresource.ErrInvalidRecord)
		return
	}
	if resource == apiTokensResource {
		record, err := s.updateAdminAPIToken(ctx.Request.Context(), s.currentActor(ctx), id, input)
		if err != nil {
			writeAdminResourceError(ctx, err)
			return
		}
		s.invalidateCachesForResource(ctx.Request.Context(), resource)
		projected, err := s.resources.ProjectRecord(resource, record, adminresource.ProjectionResponse)
		if err != nil {
			writeAdminResourceError(ctx, err)
			return
		}
		ctx.JSON(http.StatusOK, Response[adminResourceRecordResponse]{
			Data: adminResourceRecordResponse{Resource: resource, Record: projected},
		})
		return
	}
	record, err := s.resources.Update(resource, id, input)
	if err != nil {
		writeAdminResourceError(ctx, err)
		return
	}
	s.recordAdminResourceAudit(ctx, "update", resource, record)
	s.invalidateCachesForResource(ctx.Request.Context(), resource)
	projected, err := s.resources.ProjectRecord(resource, record, adminresource.ProjectionResponse)
	if err != nil {
		writeAdminResourceError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, Response[adminResourceRecordResponse]{
		Data: adminResourceRecordResponse{Resource: resource, Record: projected},
	})
}

func (s *Server) adminPolicyReviewApprove(ctx *gin.Context) {
	if !s.authorizeAdminResource(ctx, "policy-reviews", "update") {
		return
	}
	result, err := s.resources.ApprovePolicyReview(ctx.Param("id"), s.currentActor(ctx))
	if err != nil {
		writeAdminResourceError(ctx, err)
		return
	}
	for _, resource := range []string{"policy-reviews", "roles", "audit-logs"} {
		s.invalidateCachesForResource(ctx.Request.Context(), resource)
	}
	review, err := s.resources.ProjectRecord("policy-reviews", result.Review, adminresource.ProjectionResponse)
	if err != nil {
		writeAdminResourceError(ctx, err)
		return
	}
	role, err := s.resources.ProjectRecord("roles", result.Role, adminresource.ProjectionResponse)
	if err != nil {
		writeAdminResourceError(ctx, err)
		return
	}
	audit, err := s.resources.ProjectRecord("audit-logs", result.Audit, adminresource.ProjectionResponse)
	if err != nil {
		writeAdminResourceError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, Response[policyReviewApproveResponse]{
		Data: policyReviewApproveResponse{
			Review: review,
			Role:   role,
			Audit:  audit,
		},
	})
}

func (s *Server) adminPolicyReviewRequest(ctx *gin.Context) {
	if !s.authorizeAdminResource(ctx, "policy-reviews", "update") {
		return
	}
	result, err := s.resources.RequestPolicyReview(ctx.Param("id"), s.currentActor(ctx))
	if err != nil {
		writeAdminResourceError(ctx, err)
		return
	}
	for _, resource := range []string{"policy-reviews", "audit-logs"} {
		s.invalidateCachesForResource(ctx.Request.Context(), resource)
	}
	review, err := s.resources.ProjectRecord("policy-reviews", result.Review, adminresource.ProjectionResponse)
	if err != nil {
		writeAdminResourceError(ctx, err)
		return
	}
	audit, err := s.resources.ProjectRecord("audit-logs", result.Audit, adminresource.ProjectionResponse)
	if err != nil {
		writeAdminResourceError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, Response[policyReviewActionResponse]{
		Data: policyReviewActionResponse{
			Review: review,
			Audit:  audit,
		},
	})
}

func (s *Server) adminPolicyReviewReject(ctx *gin.Context) {
	if !s.authorizeAdminResource(ctx, "policy-reviews", "update") {
		return
	}
	var input policyReviewRejectRequest
	if err := ctx.ShouldBindJSON(&input); err != nil {
		writeAdminResourceError(ctx, adminresource.ErrInvalidRecord)
		return
	}
	result, err := s.resources.RejectPolicyReview(ctx.Param("id"), s.currentActor(ctx), input.Reason)
	if err != nil {
		writeAdminResourceError(ctx, err)
		return
	}
	for _, resource := range []string{"policy-reviews", "audit-logs"} {
		s.invalidateCachesForResource(ctx.Request.Context(), resource)
	}
	review, err := s.resources.ProjectRecord("policy-reviews", result.Review, adminresource.ProjectionResponse)
	if err != nil {
		writeAdminResourceError(ctx, err)
		return
	}
	audit, err := s.resources.ProjectRecord("audit-logs", result.Audit, adminresource.ProjectionResponse)
	if err != nil {
		writeAdminResourceError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, Response[policyReviewActionResponse]{
		Data: policyReviewActionResponse{
			Review: review,
			Audit:  audit,
		},
	})
}

func (s *Server) adminPolicyReviewExport(ctx *gin.Context) {
	if !s.authorizeAdminResource(ctx, "policy-reviews", "read") {
		return
	}
	result, err := s.resources.ExportPolicyReviews(s.currentActor(ctx))
	if err != nil {
		writeAdminResourceError(ctx, err)
		return
	}
	s.invalidateCachesForResource(ctx.Request.Context(), "audit-logs")
	reviews := make([]adminresource.Record, 0, len(result.Reviews))
	for _, review := range result.Reviews {
		projected, projectErr := s.resources.ProjectRecord("policy-reviews", review, adminresource.ProjectionExport)
		if projectErr != nil {
			writeAdminResourceError(ctx, projectErr)
			return
		}
		reviews = append(reviews, projected)
	}
	audits := make([]adminresource.Record, 0, len(result.Audits))
	for _, audit := range result.Audits {
		projected, projectErr := s.resources.ProjectRecord("audit-logs", audit, adminresource.ProjectionExport)
		if projectErr != nil {
			writeAdminResourceError(ctx, projectErr)
			return
		}
		audits = append(audits, projected)
	}
	ctx.JSON(http.StatusOK, Response[policyReviewExportResponse]{
		Data: policyReviewExportResponse{
			ExportedBy: result.ExportedBy,
			ExportedAt: result.ExportedAt,
			Reviews:    reviews,
			Audits:     audits,
		},
	})
}

func (s *Server) adminResourceDelete(ctx *gin.Context) {
	resource := ctx.Param("resource")
	if !s.authorizeAdminResource(ctx, resource, "delete") {
		return
	}
	if resource == "files" {
		if !s.deleteAdminFileObject(ctx, ctx.Param("id")) {
			return
		}
	}
	if resource == apiTokensResource {
		if err := s.revokeAdminAPIToken(ctx.Request.Context(), s.currentActor(ctx), ctx.Param("id")); err != nil {
			writeAdminResourceError(ctx, err)
			return
		}
		s.invalidateCachesForResource(ctx.Request.Context(), resource)
		ctx.JSON(http.StatusOK, Response[gin.H]{Data: gin.H{"resource": resource, "revoked": true}})
		return
	}
	record, err := s.adminResourceRecordByID(resource, ctx.Param("id"))
	if err != nil {
		writeAdminResourceError(ctx, err)
		return
	}
	if err := s.resources.Delete(resource, ctx.Param("id")); err != nil {
		writeAdminResourceError(ctx, err)
		return
	}
	if resource == "files" {
		if err := s.recordFileAudit(ctx, "file.delete", record); err != nil {
			writeAdminResourceError(ctx, err)
			return
		}
	} else {
		s.recordAdminResourceAudit(ctx, "delete", resource, record)
	}
	s.invalidateCachesForResource(ctx.Request.Context(), resource)
	ctx.JSON(http.StatusOK, Response[gin.H]{Data: gin.H{"resource": resource, "deleted": true}})
}

func (s *Server) adminFileUpload(ctx *gin.Context) {
	if !s.authorizeAdminResource(ctx, "files", "create") {
		return
	}
	upload, err := readValidatedUpload(ctx, s.uploadPolicy, "ADMIN_FILE")
	if err != nil {
		writeUploadPolicyError(ctx, err)
		return
	}
	defer upload.Close()

	metadata, err := s.fileStorage.Save(ctx.Request.Context(), storage.ObjectSaveInput{
		FileName:    upload.FileName,
		ContentType: upload.ContentType,
		Reader:      upload.Reader,
	})
	if err != nil {
		s.recordInternalError(ctx, "ADMIN_FILE_SAVE_FAILED", err)
		writeFileError(ctx, http.StatusInternalServerError, "ADMIN_FILE_SAVE_FAILED", "file save failed")
		return
	}
	record, err := s.resources.CreateInternal("files", adminresource.WriteInput{
		Code:        fmt.Sprintf("file-%d", s.now().UTC().UnixNano()),
		Name:        upload.FileName,
		Status:      "enabled",
		Description: "Uploaded file object.",
		Values: map[string]string{
			"mimeType":      upload.ContentType,
			"size":          strconv.FormatInt(metadata.SizeBytes, 10),
			"storageDriver": metadata.Driver,
			"storageKey":    metadata.Key,
			"createdAt":     time.Now().UTC().Format(time.RFC3339),
		},
	})
	if err != nil {
		if rollbackErr := s.fileStorage.Delete(ctx.Request.Context(), metadata.Key); rollbackErr != nil {
			s.recordInternalError(ctx, "ADMIN_FILE_METADATA_CREATE_FAILED", err)
			s.recordInternalError(ctx, "ADMIN_FILE_ROLLBACK_DELETE_FAILED", rollbackErr)
			s.recordFileCleanup(ctx, metadata.Key)
			writeFileError(ctx, http.StatusInternalServerError, "ADMIN_FILE_ROLLBACK_FAILED", "file upload rollback failed")
			return
		}
		writeAdminResourceError(ctx, err)
		return
	}
	if err := s.recordFileAudit(ctx, "file.upload", record); err != nil {
		writeAdminResourceError(ctx, err)
		return
	}
	s.invalidateCachesForResource(ctx.Request.Context(), "files")
	projected, err := s.resources.ProjectRecord("files", record, adminresource.ProjectionResponse)
	if err != nil {
		writeAdminResourceError(ctx, err)
		return
	}
	ctx.JSON(http.StatusCreated, Response[adminResourceRecordResponse]{
		Data: adminResourceRecordResponse{Resource: "files", Record: projected},
	})
}

func (s *Server) adminFileContent(ctx *gin.Context) {
	if !s.authorizeAdminResource(ctx, "files", "read") {
		return
	}
	record, err := s.adminResourceRecordByID("files", ctx.Param("id"))
	if err != nil {
		writeAdminResourceError(ctx, err)
		return
	}
	key := fileStorageKey(record)
	if key == "" {
		writeFileError(ctx, http.StatusNotFound, "ADMIN_FILE_OBJECT_NOT_FOUND", "file object not found")
		return
	}
	body, err := s.fileStorage.Open(ctx.Request.Context(), key)
	if errors.Is(err, storage.ErrObjectNotFound) {
		writeFileError(ctx, http.StatusNotFound, "ADMIN_FILE_OBJECT_NOT_FOUND", "file object not found")
		return
	}
	if err != nil {
		s.recordInternalError(ctx, "ADMIN_FILE_OPEN_FAILED", err)
		writeFileError(ctx, http.StatusInternalServerError, "ADMIN_FILE_OPEN_FAILED", "file open failed")
		return
	}
	defer body.Close()
	if err := s.recordFileAudit(ctx, "file.content", record); err != nil {
		writeAdminResourceError(ctx, err)
		return
	}

	fileName := fileRecordFileName(record)
	contentType := fileRecordContentType(record)
	ctx.Header("X-Content-Type-Options", "nosniff")
	ctx.Header("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": fileName}))
	ctx.Header("Content-Type", contentType)
	ctx.Status(http.StatusOK)
	_, _ = io.Copy(ctx.Writer, body)
}

func (s *Server) appFileUpload(ctx *gin.Context) {
	appSession, ok := AppSessionFromContext(ctx)
	if !ok {
		writeUnauthorized(ctx)
		return
	}
	upload, err := readValidatedUpload(ctx, s.uploadPolicy, "APP_FILE")
	if err != nil {
		writeUploadPolicyError(ctx, err)
		return
	}
	defer upload.Close()

	metadata, err := s.fileStorage.Save(ctx.Request.Context(), storage.ObjectSaveInput{
		FileName:    upload.FileName,
		ContentType: upload.ContentType,
		Reader:      upload.Reader,
	})
	if err != nil {
		s.recordInternalError(ctx, "APP_FILE_SAVE_FAILED", err)
		writeFileError(ctx, http.StatusInternalServerError, "APP_FILE_SAVE_FAILED", "file save failed")
		return
	}
	username := appUsername(appSession.Username)
	record, err := s.resources.CreateInternal("files", adminresource.WriteInput{
		Code:        fmt.Sprintf("file-%d", s.now().UTC().UnixNano()),
		Name:        upload.FileName,
		Status:      "enabled",
		Description: "Uploaded app file object.",
		Values: map[string]string{
			"mimeType":      upload.ContentType,
			"size":          strconv.FormatInt(metadata.SizeBytes, 10),
			"storageDriver": metadata.Driver,
			"storageKey":    metadata.Key,
			"tenantId":      appTenant,
			"ownerId":       appUserID(username),
			"uploadedBy":    username,
			"createdAt":     s.now().UTC().Format(time.RFC3339),
		},
	})
	if err != nil {
		if rollbackErr := s.fileStorage.Delete(ctx.Request.Context(), metadata.Key); rollbackErr != nil {
			s.recordInternalError(ctx, "APP_FILE_METADATA_CREATE_FAILED", err)
			s.recordInternalError(ctx, "APP_FILE_ROLLBACK_DELETE_FAILED", rollbackErr)
			s.recordFileCleanup(ctx, metadata.Key)
			writeFileError(ctx, http.StatusInternalServerError, "APP_FILE_ROLLBACK_FAILED", "file upload rollback failed")
			return
		}
		writeAdminResourceError(ctx, err)
		return
	}
	if err := s.recordFileAuditForActor("file.upload", username, record); err != nil {
		writeAdminResourceError(ctx, err)
		return
	}
	s.invalidateCachesForResource(ctx.Request.Context(), "files")
	projected, err := s.resources.ProjectRecord("files", record, adminresource.ProjectionResponse)
	if err != nil {
		writeAdminResourceError(ctx, err)
		return
	}
	ctx.JSON(http.StatusCreated, Response[adminResourceRecordResponse]{
		Data: adminResourceRecordResponse{Resource: "files", Record: projected},
	})
}

func (s *Server) appFileContent(ctx *gin.Context) {
	appSession, ok := AppSessionFromContext(ctx)
	if !ok {
		writeUnauthorized(ctx)
		return
	}
	record, err := s.adminResourceRecordByID("files", ctx.Param("id"))
	if err != nil {
		writeFileError(ctx, http.StatusNotFound, "APP_FILE_NOT_FOUND", "file not found")
		return
	}
	username := appUsername(appSession.Username)
	if !appFileVisibleToSession(record, username) {
		writeFileError(ctx, http.StatusNotFound, "APP_FILE_NOT_FOUND", "file not found")
		return
	}
	key := fileStorageKey(record)
	if key == "" {
		writeFileError(ctx, http.StatusNotFound, "APP_FILE_OBJECT_NOT_FOUND", "file object not found")
		return
	}
	body, err := s.fileStorage.Open(ctx.Request.Context(), key)
	if errors.Is(err, storage.ErrObjectNotFound) {
		writeFileError(ctx, http.StatusNotFound, "APP_FILE_OBJECT_NOT_FOUND", "file object not found")
		return
	}
	if err != nil {
		s.recordInternalError(ctx, "APP_FILE_OPEN_FAILED", err)
		writeFileError(ctx, http.StatusInternalServerError, "APP_FILE_OPEN_FAILED", "file open failed")
		return
	}
	defer body.Close()
	if err := s.recordFileAuditForActor("file.content", username, record); err != nil {
		writeAdminResourceError(ctx, err)
		return
	}

	fileName := fileRecordFileName(record)
	contentType := fileRecordContentType(record)
	ctx.Header("X-Content-Type-Options", "nosniff")
	ctx.Header("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": fileName}))
	ctx.Header("Content-Type", contentType)
	ctx.Status(http.StatusOK)
	_, _ = io.Copy(ctx.Writer, body)
}

func appFileVisibleToSession(record adminresource.Record, username string) bool {
	if strings.TrimSpace(record.Values["tenantId"]) != appTenant {
		return false
	}
	ownerID := strings.TrimSpace(record.Values["ownerId"])
	if ownerID != "" {
		return ownerID == appUserID(username)
	}
	return strings.TrimSpace(record.Values["uploadedBy"]) == username
}

func (s *Server) deleteAdminFileObject(ctx *gin.Context, id string) bool {
	record, err := s.adminResourceRecordByID("files", id)
	if err != nil {
		writeAdminResourceError(ctx, err)
		return false
	}
	key := fileStorageKey(record)
	if key == "" {
		return true
	}
	if err := s.fileStorage.Delete(ctx.Request.Context(), key); err != nil {
		s.recordInternalError(ctx, "ADMIN_FILE_DELETE_FAILED", err)
		writeFileError(ctx, http.StatusInternalServerError, "ADMIN_FILE_DELETE_FAILED", "file delete failed")
		return false
	}
	return true
}

func (s *Server) adminResourceRecordByID(resource string, id string) (adminresource.Record, error) {
	items, err := s.resources.List(resource)
	if err != nil {
		return adminresource.Record{}, err
	}
	for _, item := range items {
		if item.ID == id {
			return item, nil
		}
	}
	return adminresource.Record{}, adminresource.ErrRecordNotFound
}

func (s *Server) issueAdminAPIToken(ctx context.Context, actor string, input adminresource.WriteInput) (adminresource.Record, string, error) {
	values := cloneStringMap(input.Values)
	scopes, err := s.validateAPITokenScopes(values["scope"])
	if err != nil {
		return adminresource.Record{}, "", err
	}
	if expiresAt := strings.TrimSpace(values["expiresAt"]); expiresAt != "" {
		normalized, err := normalizeAPITokenExpiresAt(expiresAt)
		if err != nil {
			return adminresource.Record{}, "", err
		}
		values["expiresAt"] = normalized
	}
	token, err := generateAPIToken()
	if err != nil {
		return adminresource.Record{}, "", err
	}
	prefix := apiTokenPrefixValue(token)
	values["scope"] = strings.Join(scopes, ",")
	values["tokenPrefix"] = prefix
	values["tokenHash"] = hashAPIToken(token)
	values["createdAt"] = s.now().UTC().Format(time.RFC3339)
	delete(values, "token")
	if strings.TrimSpace(input.Status) == "" {
		input.Status = "active"
	}
	if strings.TrimSpace(input.Code) == "" {
		input.Code = prefix
	}
	input.Values = values
	record, err := s.resources.CreateInternal(apiTokensResource, input)
	if err != nil {
		return adminresource.Record{}, "", err
	}
	if err := s.recordAudit("api_token.create", "API Token Issued", actor, ""); err != nil {
		return adminresource.Record{}, "", err
	}
	return record, token, nil
}

func (s *Server) revokeAdminAPIToken(ctx context.Context, actor string, id string) error {
	record, err := s.adminResourceRecordByID(apiTokensResource, id)
	if err != nil {
		return err
	}
	values := cloneStringMap(record.Values)
	values["revokedAt"] = s.now().UTC().Format(time.RFC3339)
	_, err = s.resources.UpdateInternal(apiTokensResource, id, adminresource.WriteInput{
		Code:        record.Code,
		Name:        record.Name,
		Status:      "revoked",
		Description: record.Description,
		Values:      values,
	})
	if err != nil {
		return err
	}
	return s.recordAudit("api_token.revoke", "API Token Revoked", actor, "")
}

func (s *Server) updateAdminAPIToken(ctx context.Context, actor string, id string, input adminresource.WriteInput) (adminresource.Record, error) {
	existing, err := s.adminResourceRecordByID(apiTokensResource, id)
	if err != nil {
		return adminresource.Record{}, err
	}
	values := cloneStringMap(input.Values)
	if strings.TrimSpace(values["scope"]) == "" {
		values["scope"] = existing.Values["scope"]
	}
	scopes, err := s.validateAPITokenScopes(values["scope"])
	if err != nil {
		return adminresource.Record{}, err
	}
	values["scope"] = strings.Join(scopes, ",")
	if expiresAt := strings.TrimSpace(values["expiresAt"]); expiresAt != "" {
		normalized, err := normalizeAPITokenExpiresAt(expiresAt)
		if err != nil {
			return adminresource.Record{}, err
		}
		values["expiresAt"] = normalized
	} else if existing.Values["expiresAt"] != "" {
		values["expiresAt"] = existing.Values["expiresAt"]
	}
	for _, key := range []string{"tokenPrefix", "tokenHash", "createdAt", "revokedAt"} {
		if existing.Values[key] != "" {
			values[key] = existing.Values[key]
		} else {
			delete(values, key)
		}
	}
	delete(values, "token")
	if strings.TrimSpace(input.Code) == "" {
		input.Code = existing.Code
	}
	if strings.TrimSpace(input.Name) == "" {
		input.Name = existing.Name
	}
	status := strings.TrimSpace(input.Status)
	if existing.Status == "revoked" {
		status = "revoked"
	} else if status == "" {
		status = existing.Status
	}
	input.Status = status
	input.Values = values
	record, err := s.resources.UpdateInternal(apiTokensResource, id, input)
	if err != nil {
		return adminresource.Record{}, err
	}
	if err := s.recordAudit("api_token.update", "API Token Updated", actor, ""); err != nil {
		return adminresource.Record{}, err
	}
	return record, nil
}

func (s *Server) validateAPITokenScopes(scopeValue string) ([]string, error) {
	scopes := splitAPITokenScopes(scopeValue)
	if len(scopes) == 0 {
		return nil, adminresource.ValidationError{Field: "scope"}
	}
	permissions, err := s.resources.List("permissions")
	if err != nil {
		return nil, err
	}
	allowed := map[string]struct{}{}
	for _, permission := range permissions {
		code := strings.TrimSpace(permission.Code)
		if code != "" {
			allowed[code] = struct{}{}
		}
	}
	for _, scope := range scopes {
		if _, ok := allowed[scope]; ok {
			continue
		}
		return nil, adminresource.ValidationError{Field: "unknown api token scope"}
	}
	return scopes, nil
}

func (s *Server) authorizeAPIToken(token string, permission string) (bool, bool) {
	scopes, ok := s.resolveAPITokenScopes(token)
	if !ok {
		return false, false
	}
	for _, scope := range scopes {
		if scope == permission {
			return true, true
		}
	}
	return false, true
}

func (s *Server) resolveAPITokenScopes(token string) ([]string, bool) {
	token = strings.TrimSpace(token)
	if token == "" || !strings.HasPrefix(token, apiTokenPrefix) {
		return nil, false
	}
	tokenPrefix := apiTokenPrefixValue(token)
	tokenHash := hashAPIToken(token)
	records, err := s.resources.List(apiTokensResource)
	if err != nil {
		return nil, false
	}
	for _, record := range records {
		values := record.Values
		if record.Status != "active" || strings.TrimSpace(values["tokenPrefix"]) != tokenPrefix {
			continue
		}
		storedHash := strings.TrimSpace(values["tokenHash"])
		if storedHash == "" || subtle.ConstantTimeCompare([]byte(storedHash), []byte(tokenHash)) != 1 {
			continue
		}
		if apiTokenExpired(values["expiresAt"], s.now()) {
			return nil, false
		}
		return splitAPITokenScopes(values["scope"]), true
	}
	return nil, false
}

func apiTokenExpired(expiresAt string, now time.Time) bool {
	expiresAt = strings.TrimSpace(expiresAt)
	if expiresAt == "" {
		return false
	}
	parsed, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return true
	}
	return !now.UTC().Before(parsed.UTC())
}

func (s *Server) projectAdminResourceRecords(resource string, records []adminresource.Record, purpose adminresource.ProjectionPurpose) ([]adminresource.Record, error) {
	items := make([]adminresource.Record, 0, len(records))
	for _, record := range records {
		projected, err := s.resources.ProjectRecord(resource, record, purpose)
		if err != nil {
			return nil, err
		}
		items = append(items, projected)
	}
	return items, nil
}

func splitAPITokenScopes(scopeValue string) []string {
	fields := strings.FieldsFunc(scopeValue, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\t' || r == ' '
	})
	scopes := make([]string, 0, len(fields))
	seen := map[string]struct{}{}
	for _, field := range fields {
		scope := strings.TrimSpace(field)
		if scope == "" {
			continue
		}
		if _, exists := seen[scope]; exists {
			continue
		}
		seen[scope] = struct{}{}
		scopes = append(scopes, scope)
	}
	return scopes
}

func normalizeAPITokenExpiresAt(value string) (string, error) {
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.UTC().Format(time.RFC3339), nil
	}
	if parsed, err := time.Parse("2006-01-02", value); err == nil {
		return parsed.UTC().Format(time.RFC3339), nil
	}
	return "", adminresource.ValidationError{Field: "expiresAt"}
}

func generateAPIToken() (string, error) {
	randomBytes := make([]byte, 24)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}
	return apiTokenPrefix + base64.RawURLEncoding.EncodeToString(randomBytes), nil
}

func apiTokenPrefixValue(token string) string {
	if len(token) <= 12 {
		return token
	}
	return token[:12]
}

func hashAPIToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return map[string]string{}
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func (s *Server) currentActor(ctx *gin.Context) string {
	if ctx == nil {
		return "system"
	}
	if principal, ok := ctx.Get("platform.principal"); ok {
		if typed, ok := principal.(rbac.Principal); ok && strings.TrimSpace(typed.User.Username) != "" {
			return strings.TrimSpace(typed.User.Username)
		}
	}
	if principal, ok := ctx.Get("principal"); ok {
		if typed, ok := principal.(rbac.Principal); ok && strings.TrimSpace(typed.User.Username) != "" {
			return strings.TrimSpace(typed.User.Username)
		}
	}
	if principal, ok := s.currentPrincipal(ctx); ok && strings.TrimSpace(principal.User.Username) != "" {
		return strings.TrimSpace(principal.User.Username)
	}
	return "system"
}

func (s *Server) adminCurrentSession(ctx *gin.Context) {
	principal, ok := s.currentPrincipal(ctx)
	if !ok {
		writeUnauthorized(ctx)
		return
	}
	ctx.JSON(http.StatusOK, Response[rbac.Principal]{Data: principal})
}

func (s *Server) adminMenus(ctx *gin.Context) {
	principal, ok := s.currentPrincipal(ctx)
	if !ok {
		writeUnauthorized(ctx)
		return
	}
	items := cachedJSONValue(ctx.Request.Context(), s.cache, adminMenusCacheKey(principal), s.cacheTTL, func() []adminresource.MenuItem {
		return s.resources.MenuItemsForPrincipal(principal)
	})
	ctx.JSON(http.StatusOK, Response[adminMenuListResponse]{
		Data: adminMenuListResponse{Items: items},
	})
}

func (s *Server) adminDemoDataList(ctx *gin.Context) {
	if !s.authorize(ctx, "admin:demo-data:read") {
		return
	}
	items := make([]adminDemoDataItem, 0)
	for _, manifest := range s.capabilities {
		for _, dataset := range manifest.DemoData {
			items = append(items, adminDemoDataItem{
				ID:           dataset.ID,
				CapabilityID: manifest.ID,
				Resource:     dataset.Resource,
				Title:        localizedTextFromCapability(dataset.Title),
				Description:  localizedTextFromCapability(dataset.Description),
				Records:      len(dataset.Records),
			})
		}
	}
	ctx.JSON(http.StatusOK, Response[adminDemoDataListResponse]{Data: adminDemoDataListResponse{Items: items}})
}

func (s *Server) adminDemoDataApply(ctx *gin.Context) {
	if !s.authorize(ctx, "admin:demo-data:apply") {
		return
	}
	capabilityID := capability.ID(ctx.Param("capability"))
	datasetID := ctx.Param("dataset")
	dataset, ok := s.findDemoDataSet(capabilityID, datasetID)
	if !ok {
		ctx.JSON(http.StatusNotFound, Response[gin.H]{
			Error: &ErrorBody{Code: "ADMIN_DEMO_DATA_NOT_FOUND", Message: "demo data set not found"},
		})
		return
	}
	result, err := s.resources.ApplyDemoDataSet(dataset)
	if err != nil {
		writeAdminResourceError(ctx, err)
		return
	}
	s.invalidateCachesForResource(ctx.Request.Context(), result.Resource)
	ctx.JSON(http.StatusOK, Response[adminDemoDataApplyResponse]{
		Data: adminDemoDataApplyResponse{
			ID:           dataset.ID,
			CapabilityID: capabilityID,
			Resource:     result.Resource,
			Applied:      result.Applied,
		},
	})
}

func (s *Server) findDemoDataSet(capabilityID capability.ID, datasetID string) (capability.DemoDataSet, bool) {
	for _, manifest := range s.capabilities {
		if manifest.ID != capabilityID {
			continue
		}
		for _, dataset := range manifest.DemoData {
			if dataset.ID == datasetID {
				return dataset, true
			}
		}
	}
	return capability.DemoDataSet{}, false
}

func (s *Server) findAuthProvider(providerID string, audience capability.AuthProviderAudience) (capability.AuthProvider, bool) {
	providerID = strings.TrimSpace(providerID)
	for _, manifest := range s.capabilities {
		for _, provider := range manifest.AuthProviders {
			if provider.ID == providerID && s.authProviderAvailable(provider) && provider.SupportsAudience(audience) {
				return provider, true
			}
		}
	}
	return capability.AuthProvider{}, false
}

func (s *Server) authProviderAvailable(provider capability.AuthProvider) bool {
	if !provider.Enabled {
		return false
	}
	return !(s.disableDemoAuthProvider && provider.Kind == "demo")
}

func (s *Server) recordAudit(code string, name string, username string, provider string) error {
	auditCode, err := newAuthAuditCode(code)
	if err != nil {
		return err
	}
	values := map[string]string{
		"actor":     username,
		"action":    code,
		"resource":  "auth",
		"createdAt": s.now().UTC().Format(time.RFC3339),
	}
	if provider != "" {
		values["provider"] = provider
	}
	_, err = s.resources.CreateInternal("audit-logs", adminresource.WriteInput{
		Code:        auditCode,
		Name:        name,
		Status:      "recorded",
		Description: "Authentication event recorded by platform auth.",
		Values:      values,
	})
	if errors.Is(err, adminresource.ErrUnknownResource) {
		return nil
	}
	return err
}

func newAuthAuditCode(action string) (string, error) {
	var suffix [12]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "", err
	}
	return action + "." + hex.EncodeToString(suffix[:]), nil
}

func (s *Server) recordAdminResourceAudit(ctx *gin.Context, action string, resource string, record adminresource.Record) {
	if resource == "audit" || resource == "audit-logs" || action == "" {
		return
	}
	_, err := s.resources.CreateInternal("audit-logs", adminresource.WriteInput{
		Code:        "admin_resource." + action + "." + resource + "." + record.ID,
		Name:        "Admin Resource " + strings.ToUpper(action[:1]) + action[1:],
		Status:      "recorded",
		Description: "Admin resource write operation recorded by platform admin.",
		Values: map[string]string{
			"actor":      s.currentActor(ctx),
			"action":     "admin_resource." + action,
			"resource":   resource,
			"targetId":   record.ID,
			"targetCode": record.Code,
			"createdAt":  s.now().UTC().Format(time.RFC3339),
		},
	})
	if errors.Is(err, adminresource.ErrUnknownResource) {
		return
	}
}

func (s *Server) recordFileAudit(ctx *gin.Context, action string, record adminresource.Record) error {
	return s.recordFileAuditForActor(action, s.currentActor(ctx), record)
}

func (s *Server) recordFileAuditForActor(action string, actor string, record adminresource.Record) error {
	if action == "" {
		return nil
	}
	actor = strings.TrimSpace(actor)
	if actor == "" {
		actor = "system"
	}
	_, err := s.resources.CreateInternal("audit-logs", adminresource.WriteInput{
		Code:        action + "." + record.ID,
		Name:        "File Operation",
		Status:      "recorded",
		Description: "File operation recorded by platform admin.",
		Values: map[string]string{
			"actor":      actor,
			"action":     action,
			"resource":   "files",
			"targetId":   record.ID,
			"targetCode": record.Code,
			"createdAt":  s.now().UTC().Format(time.RFC3339),
		},
	})
	if errors.Is(err, adminresource.ErrUnknownResource) {
		return nil
	}
	return err
}

func (s *Server) currentPrincipal(ctx *gin.Context) (rbac.Principal, bool) {
	if _, hasBearer := bearerToken(ctx.GetHeader("Authorization")); hasBearer {
		authSession, ok := s.authSessionFromBearer(ctx)
		if !ok {
			return rbac.Principal{}, false
		}
		return s.currentPrincipalForUsername(ctx.Request.Context(), authSession.Username), true
	}
	if !s.allowInsecureHeaderAuth {
		return rbac.Principal{}, false
	}
	return s.currentPrincipalForUsername(ctx.Request.Context(), ctx.GetHeader("X-Platform-User")), true
}

func (s *Server) authSessionFromBearer(ctx *gin.Context) (session.Session, bool) {
	authSession, ok, err := s.authSessionFromBearerContext(ctx)
	if err != nil {
		return session.Session{}, false
	}
	return authSession, ok
}

func (s *Server) authSessionFromBearerContext(ctx *gin.Context) (session.Session, bool, error) {
	token, ok := bearerToken(ctx.GetHeader("Authorization"))
	if !ok {
		return session.Session{}, false, nil
	}
	claims, err := s.tokens.Parse(token)
	if err != nil {
		return session.Session{}, false, nil
	}
	if claims.TokenType != authjwt.TokenTypeAdmin || strings.TrimSpace(claims.SessionID) == "" {
		return session.Session{}, false, nil
	}
	if claims.TenantID != "" && claims.TenantID != platformTenant {
		return session.Session{}, false, nil
	}
	authSession, ok, err := s.sessions.ResolveContext(ctx.Request.Context(), claims.SessionID)
	if err != nil {
		return session.Session{}, false, err
	}
	if !ok {
		return session.Session{}, false, nil
	}
	if claims.Username != "" && claims.Username != authSession.Username {
		return session.Session{}, false, nil
	}
	return authSession, true, nil
}

func (s *Server) appSessionFromBearer(ctx *gin.Context) (session.Session, bool) {
	appSession, ok, err := s.appSessionFromBearerContext(ctx)
	if err != nil {
		if ctx.Request.Method == http.MethodPost && ctx.Request.URL.Path == "/api/app/auth/logout" {
			ctx.Set(appLogoutResolveErrorKey, true)
		}
		return session.Session{}, false
	}
	return appSession, ok
}

func (s *Server) appSessionFromBearerContext(ctx *gin.Context) (session.Session, bool, error) {
	token, ok := bearerToken(ctx.GetHeader("Authorization"))
	if !ok {
		return session.Session{}, false, nil
	}
	claims, err := s.tokens.Parse(token)
	if err != nil {
		return session.Session{}, false, nil
	}
	if claims.TokenType != authjwt.TokenTypeApp || strings.TrimSpace(claims.SessionID) == "" {
		return session.Session{}, false, nil
	}
	if claims.TenantID != "" && claims.TenantID != appTenant {
		return session.Session{}, false, nil
	}
	appSession, ok, err := s.sessions.ResolveContext(ctx.Request.Context(), claims.SessionID)
	if err != nil {
		return session.Session{}, false, err
	}
	if !ok {
		return session.Session{}, false, nil
	}
	if claims.Username != "" && claims.Username != appSession.Username {
		return session.Session{}, false, nil
	}
	return appSession, true, nil
}

func appUsername(raw string) string {
	username := strings.TrimSpace(raw)
	if username == "" {
		return "guest"
	}
	return username
}

func appUserID(username string) string {
	return "app:" + username
}

func appSessionResponseFromSession(appSession session.Session) appSessionResponse {
	username := appUsername(appSession.Username)
	return appSessionResponse{
		UserID:    appUserID(username),
		Username:  username,
		TenantID:  appTenant,
		SessionID: appSession.Token,
	}
}

func (s *Server) authorize(ctx *gin.Context, permission string) bool {
	if token, ok := bearerToken(ctx.GetHeader("Authorization")); ok && strings.HasPrefix(token, apiTokenPrefix) {
		allowed, valid := s.authorizeAPIToken(token, permission)
		if !valid {
			writeUnauthorized(ctx)
			return false
		}
		if allowed {
			return true
		}
		writeForbidden(ctx)
		return false
	}
	principal, ok := s.currentPrincipal(ctx)
	if !ok {
		writeUnauthorized(ctx)
		return false
	}
	if s.can(principal, permission) {
		return true
	}
	writeForbidden(ctx)
	return false
}

func (s *Server) can(principal rbac.Principal, permission string) bool {
	authorizer, ok := s.policyAuthorizerForRequest()
	if !ok {
		return false
	}
	return authorizer.Can(principal.User.Username, platformTenant, permission, actionFromPermission(permission))
}

func (s *Server) policyAuthorizerForRequest() (Authorizer, bool) {
	if s.authorizer != nil {
		return s.authorizer, true
	}
	s.policyMu.Lock()
	defer s.policyMu.Unlock()
	if s.policyAuthorizer != nil {
		return s.policyAuthorizer, true
	}
	authorizer, err := s.resources.CasbinAuthorizer()
	if err != nil {
		return nil, false
	}
	s.policyAuthorizer = authorizer
	return authorizer, true
}

func (s *Server) authorizeAdminResource(ctx *gin.Context, resource string, action string) bool {
	_, _, ok := s.authorizeAdminResourcePrincipal(ctx, resource, action)
	return ok
}

func (s *Server) authorizeAdminResourcePrincipal(ctx *gin.Context, resource string, action string) (rbac.Principal, bool, bool) {
	schema, err := s.resources.Schema(resource)
	if err != nil {
		writeAdminResourceError(ctx, err)
		return rbac.Principal{}, false, false
	}
	permission := permissionForAction(schema.Permissions, action)
	if token, ok := bearerToken(ctx.GetHeader("Authorization")); ok && strings.HasPrefix(token, apiTokenPrefix) {
		allowed, valid := s.authorizeAPIToken(token, permission)
		if !valid {
			writeUnauthorized(ctx)
			return rbac.Principal{}, false, false
		}
		if allowed {
			return rbac.Principal{}, false, true
		}
		writeForbidden(ctx)
		return rbac.Principal{}, false, false
	}
	principal, ok := s.currentPrincipal(ctx)
	if !ok {
		writeUnauthorized(ctx)
		return rbac.Principal{}, false, false
	}
	if s.can(principal, permission) {
		return principal, true, true
	}
	writeForbidden(ctx)
	return rbac.Principal{}, false, false
}

func actionFromPermission(permission string) string {
	permission = strings.TrimSpace(permission)
	if permission == "" || permission == "*" || strings.HasSuffix(permission, "*") {
		return "*"
	}
	index := strings.LastIndex(permission, ":")
	if index < 0 || index == len(permission)-1 {
		return "*"
	}
	return permission[index+1:]
}

func permissionForAction(permissions adminresource.ActionPermissions, action string) string {
	switch action {
	case "read":
		return permissions.Read
	case "create":
		return permissions.Create
	case "update":
		return permissions.Update
	case "delete":
		return permissions.Delete
	default:
		return ""
	}
}

func localizedTextFromCapability(value capability.LocalizedText) adminresource.LocalizedText {
	return adminresource.LocalizedText{ZH: value.ZH, EN: value.EN}
}

func cachedJSONValue[T any](ctx context.Context, store cache.Store, key string, ttl time.Duration, load func() T) T {
	if value, ok, err := store.Get(ctx, key); err == nil && ok {
		var decoded T
		if json.Unmarshal(value, &decoded) == nil {
			return decoded
		}
	}
	loaded := load()
	if encoded, err := json.Marshal(loaded); err == nil {
		_ = store.Set(ctx, key, encoded, ttl)
	}
	return loaded
}

func cachedJSONResult[T any](ctx context.Context, store cache.Store, key string, ttl time.Duration, load func() (T, error)) (T, error) {
	if value, ok, err := store.Get(ctx, key); err == nil && ok {
		var decoded T
		if json.Unmarshal(value, &decoded) == nil {
			return decoded, nil
		}
	}
	loaded, err := load()
	if err != nil {
		var zero T
		return zero, err
	}
	if encoded, err := json.Marshal(loaded); err == nil {
		_ = store.Set(ctx, key, encoded, ttl)
	}
	return loaded, nil
}

func (s *Server) currentPrincipalForUsername(ctx context.Context, username string) rbac.Principal {
	username = strings.TrimSpace(username)
	if username == "" {
		username = "admin"
	}
	return cachedJSONValue(ctx, s.cache, cacheKeyPrincipalPrefix+username, s.cacheTTL, func() rbac.Principal {
		return s.resources.CurrentPrincipal(username)
	})
}

func adminMenusCacheKey(principal rbac.Principal) string {
	username := strings.TrimSpace(principal.User.Username)
	if username == "" {
		username = "anonymous"
	}
	return cacheKeyMenusPrefix + username
}

func (s *Server) invalidateCachesForResource(ctx context.Context, resource string) {
	s.invalidateCachesForResourceLocal(ctx, resource)
	if s.invalidationBus != nil {
		_ = s.invalidationBus.PublishInvalidation(ctx, cache.InvalidationEvent{Resource: resource})
	}
}

func (s *Server) publishSessionInvalidation(ctx context.Context) {
	if s.invalidationBus != nil {
		_ = s.invalidationBus.PublishInvalidation(ctx, cache.InvalidationEvent{Resource: sessionInvalidationResource})
	}
}

func (s *Server) subscribeInvalidations() {
	if s.invalidationBus == nil {
		return
	}
	_ = s.invalidationBus.SubscribeInvalidations(context.Background(), func(ctx context.Context, event cache.InvalidationEvent) {
		if event.Resource == sessionInvalidationResource {
			_ = s.sessions.Reload()
			return
		}
		if err := s.resources.Reload(); err != nil {
			return
		}
		s.invalidateCachesForResourceLocal(ctx, event.Resource)
	})
}

func (s *Server) invalidateCachesForResourceLocal(ctx context.Context, resource string) {
	switch resource {
	case "settings", "branding":
		_ = s.cache.Delete(ctx, cacheKeyBranding)
	case "menus":
		_ = s.cache.DeletePrefix(ctx, cacheKeyMenusPrefix)
	case "roles", "permissions", "users":
		s.invalidatePolicyAuthorizer()
		_ = s.cache.DeletePrefix(ctx, cacheKeyMenusPrefix)
		_ = s.cache.DeletePrefix(ctx, cacheKeyPrincipalPrefix)
		if resource == "permissions" {
			_ = s.cache.Delete(ctx, cacheKeyPermissionsList)
			_ = s.cache.DeletePrefix(ctx, cacheKeySchemaPrefix)
		}
	}
}

func (s *Server) invalidatePolicyAuthorizer() {
	if s.authorizer != nil {
		return
	}
	s.policyMu.Lock()
	defer s.policyMu.Unlock()
	s.policyAuthorizer = nil
}

func writeForbidden(ctx *gin.Context) {
	ctx.JSON(http.StatusForbidden, Response[gin.H]{
		Error: &ErrorBody{Code: "ADMIN_FORBIDDEN", Message: "permission denied"},
	})
}

func writeUnauthorized(ctx *gin.Context) {
	if resolveError, exists := ctx.Get(appLogoutResolveErrorKey); exists {
		if flagged, ok := resolveError.(bool); ok && flagged {
			writeAuthError(ctx, http.StatusInternalServerError, "APP_AUTH_SESSION_REVOKE_FAILED", "app session revoke failed")
			return
		}
	}
	ctx.JSON(http.StatusUnauthorized, Response[gin.H]{
		Error: &ErrorBody{Code: "AUTH_UNAUTHORIZED", Message: "unauthorized"},
	})
}

func writeAuthError(ctx *gin.Context, status int, code string, message string) {
	ctx.JSON(status, Response[gin.H]{
		Error: &ErrorBody{Code: code, Message: message},
	})
}

func bearerToken(header string) (string, bool) {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	return token, token != ""
}

func fileStorageKey(record adminresource.Record) string {
	if key := strings.TrimSpace(record.Values["storageKey"]); key != "" {
		return key
	}
	return strings.TrimSpace(record.Code)
}

func fileRecordFileName(record adminresource.Record) string {
	if name := strings.TrimSpace(record.Name); name != "" {
		return name
	}
	if name := strings.TrimSpace(record.Values["filename"]); name != "" {
		return name
	}
	return "file"
}

func fileRecordContentType(record adminresource.Record) string {
	contentType := strings.TrimSpace(record.Values["mimeType"])
	if contentType == "" {
		return "application/octet-stream"
	}
	return contentType
}

func writeFileError(ctx *gin.Context, status int, code string, message string) {
	ctx.JSON(status, Response[gin.H]{
		Error: &ErrorBody{Code: code, Message: message},
	})
}

func (s *Server) recordInternalError(ctx *gin.Context, code string, err error) {
	if err == nil {
		return
	}
	_ = ctx.Error(fmt.Errorf("%s: %w", code, err))
	if s.internalErrorSink != nil {
		s.internalErrorSink.Record(ctx.Request.Context(), InternalErrorEvent{Code: code, Err: err})
	}
}

func (s *Server) recordFileCleanup(ctx *gin.Context, objectKey string) {
	record := FileCleanupRecord{
		ObjectIdentifier: safeFileObjectIdentifier(objectKey),
		ReasonCode:       "metadata-create-failed-object-delete-failed",
		RetryStatus:      "pending",
		CreatedAt:        s.now().UTC(),
	}
	if err := s.fileCleanupSink.Record(ctx.Request.Context(), record); err != nil {
		s.recordInternalError(ctx, "FILE_CLEANUP_RECORD_FAILED", err)
	}
}

func safeFileObjectIdentifier(objectKey string) string {
	base := path.Base(strings.TrimSpace(objectKey))
	if len(base) == 64 {
		if decoded, err := hex.DecodeString(base); err == nil && len(decoded) == 32 && base == strings.ToLower(base) {
			return "object:" + base
		}
	}
	digest := sha256.Sum256([]byte(objectKey))
	return "sha256:" + hex.EncodeToString(digest[:])
}

type resourceFileCleanupSink struct {
	resources *adminresource.Store
}

func (sink resourceFileCleanupSink) Record(_ context.Context, record FileCleanupRecord) error {
	_, err := sink.resources.CreateInternal("error-logs", adminresource.WriteInput{
		Code:        "file.cleanup." + strings.ReplaceAll(record.ObjectIdentifier, ":", "."),
		Name:        "File Cleanup Required",
		Status:      record.RetryStatus,
		Description: record.ReasonCode,
	})
	return err
}

func writeUploadPolicyError(ctx *gin.Context, err error) {
	var policyErr *uploadPolicyError
	if errors.As(err, &policyErr) {
		writeFileError(ctx, policyErr.Status, policyErr.Code, policyErr.Message)
		return
	}
	writeFileError(ctx, http.StatusBadRequest, "FILE_UPLOAD_INVALID", "invalid file upload")
}

func writeAdminResourceError(ctx *gin.Context, err error) {
	status := http.StatusInternalServerError
	code := "ADMIN_RESOURCE_ERROR"
	message := err.Error()
	switch {
	case errors.Is(err, adminresource.ErrUnknownResource):
		status = http.StatusNotFound
		code = "ADMIN_RESOURCE_NOT_FOUND"
		message = "admin resource not found"
	case errors.Is(err, adminresource.ErrRecordNotFound):
		status = http.StatusNotFound
		code = "ADMIN_RESOURCE_RECORD_NOT_FOUND"
		message = "admin resource record not found"
	case errors.Is(err, adminresource.ErrRevisionConflict):
		status = http.StatusConflict
		code = "ADMIN_RESOURCE_REVISION_CONFLICT"
		message = "admin resource revision conflict"
	case errors.Is(err, adminresource.ErrInvalidRecord):
		status = http.StatusBadRequest
		code = "ADMIN_RESOURCE_INVALID_RECORD"
		if errors.Is(err, adminresource.ErrInvalidRecord) && err != adminresource.ErrInvalidRecord {
			message = err.Error()
		} else {
			message = "invalid admin resource record"
		}
	}
	ctx.JSON(status, Response[gin.H]{Error: &ErrorBody{Code: code, Message: message}})
}
