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
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/GnosiST/platform-go/internal/platform/adminresource"
	"github.com/GnosiST/platform-go/internal/platform/authjwt"
	"github.com/GnosiST/platform-go/internal/platform/cache"
	"github.com/GnosiST/platform-go/internal/platform/capability"
	"github.com/GnosiST/platform-go/internal/platform/dataprotection"
	"github.com/GnosiST/platform-go/internal/platform/errorcode"
	"github.com/GnosiST/platform-go/internal/platform/kernel"
	"github.com/GnosiST/platform-go/internal/platform/notification"
	"github.com/GnosiST/platform-go/internal/platform/ratelimit"
	"github.com/GnosiST/platform-go/internal/platform/rbac"
	"github.com/GnosiST/platform-go/internal/platform/sensitivereveal"
	"github.com/GnosiST/platform-go/internal/platform/serviceobject"
	"github.com/GnosiST/platform-go/internal/platform/session"
	"github.com/GnosiST/platform-go/internal/platform/storage"
)

type ServerOptions struct {
	Capabilities             []capability.Manifest
	Resources                *adminresource.Store
	DataProtection           dataprotection.Runtime
	Sessions                 *session.Store
	Cache                    cache.Store
	InvalidationBus          cache.InvalidationBus
	CacheTTL                 time.Duration
	FileStorage              storage.ObjectStore
	UploadPolicy             UploadPolicy
	InternalErrorSink        InternalErrorSink
	FileCleanupSink          FileCleanupSink
	AdminRoutes              []AdminRouteRegistration
	AppRoutes                []AppRouteRegistration
	AdminIdentityResolver    AdminIdentityResolver
	AdminIdentityBindings    AdminIdentityBindingStore
	CredentialAuth           *CredentialAuthRuntime
	AppIdentityResolver      AppIdentityResolver
	AppIdentityBindings      AppIdentityBindingStore
	PhoneProtector           PhoneProtector
	PhoneVerificationSender  PhoneVerificationSender
	NotificationSMSSender    notification.SMSSender
	AdminStepUpPhoneResolver AdminStepUpPhoneResolver
	SensitiveReveal          *sensitivereveal.Runtime
	ServiceObjects           *serviceobject.Runtime
	DebugCodeEnabled         bool
	SessionTTL               time.Duration
	JWTSecret                string
	OpenAPIDocument          []byte
	CapabilityLockFile       string
	CapabilityConfigSource   string
	TokenService             *authjwt.Service
	Now                      func() time.Time
	Authorizer               Authorizer
	AllowInsecureHeaderAuth  bool
	DisableDemoAuthProvider  bool
	Security                 SecurityOptions
	RateLimiter              ratelimit.Limiter
	RateLimitKeyBuilder      *ratelimit.KeyBuilder
	AdminMenuServingMode     AdminMenuServingMode
	AdminMenuResolver        AdminMenuResolver
	AdminMenuComparisonSink  any
	tokenService             authTokenService
	appPhoneCodeGenerator    func() (string, error)
}

type authTokenService interface {
	Sign(authjwt.Subject, time.Duration) (string, time.Time, error)
	Parse(string) (authjwt.Claims, error)
}

type Authorizer interface {
	Can(user string, tenant string, permission string, action string) bool
}

type InternalErrorEvent struct {
	Code           errorcode.Code
	EventID        string
	Err            error
	Owner          string
	Category       errorcode.Category
	RetryPolicy    errorcode.RetryPolicy
	RedactionClass errorcode.RedactionClass
	RequestID      string
	TraceID        string
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
	router                   *gin.Engine
	capabilities             []capability.Manifest
	resources                *adminresource.Store
	sessions                 *session.Store
	cache                    cache.Store
	invalidationBus          cache.InvalidationBus
	cacheStats               cache.StatsProvider
	cacheTTL                 time.Duration
	fileStorage              storage.ObjectStore
	uploadPolicy             UploadPolicy
	internalErrorSink        InternalErrorSink
	fileCleanupSink          FileCleanupSink
	appRoutes                map[appRouteKey]gin.HandlerFunc
	adminIdentityResolver    AdminIdentityResolver
	adminIdentityBindings    AdminIdentityBindingStore
	credentialAuth           *CredentialAuthRuntime
	appIdentityResolver      AppIdentityResolver
	appIdentityBindings      AppIdentityBindingStore
	phoneProtector           PhoneProtector
	phoneVerificationSender  PhoneVerificationSender
	notificationSMSSender    notification.SMSSender
	appPhoneCodeGenerator    func() (string, error)
	adminStepUpPhoneResolver AdminStepUpPhoneResolver
	sensitiveReveal          *sensitivereveal.Runtime
	serviceObjects           *serviceobject.Runtime
	debugCodeEnabled         bool
	tokens                   authTokenService
	now                      func() time.Time
	openAPIDocument          []byte
	capabilityLockFile       string
	capabilityConfigSource   string
	authorizer               Authorizer
	policyMu                 sync.Mutex
	policyAuthorizer         Authorizer
	allowInsecureHeaderAuth  bool
	disableDemoAuthProvider  bool
	security                 SecurityOptions
	profileRoutesRegistered  bool
	rateLimiter              ratelimit.Limiter
	rateLimitKeyBuilder      *ratelimit.KeyBuilder
	adminMenuServingMode     AdminMenuServingMode
	adminMenuResolver        AdminMenuResolver
	adminMenuComparisonSink  any
}

const (
	cacheKeyBranding                  = "admin:branding"
	cacheKeyAuthProviders             = "admin:auth-providers"
	cacheKeyMenusPrefix               = "admin:menus:"
	cacheKeyPrincipalPrefix           = "admin:principal:"
	cacheKeySchemaPrefix              = "admin:schema:"
	cacheKeyPermissionsList           = "admin:permissions:list"
	defaultPlatformCacheTTL           = 5 * time.Minute
	defaultJWTSecret                  = "dev-platform-go-secret"
	defaultRateLimitHMACKey           = "dev-platform-rate-limit-key-00001"
	platformTenant                    = "platform"
	appTenant                         = "app"
	apiTokensResource                 = "api-tokens"
	apiTokenPrefix                    = "pgo_"
	sessionInvalidationResource       = "sessions"
	authorizationInvalidationResource = "authorization"
	systemActorID                     = "system:platform"
)

func New(options ServerOptions) *Server {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	configureTrustedProxies(router, options.Security.TrustedProxies)
	router.Use(requestCorrelation())
	router.Use(recoveryMiddleware(options.InternalErrorSink))
	router.Use(securityHeaders(options.Security))
	router.Use(jsonRequestBodyLimit(options.Security.MaxBodyBytes))
	resources := options.Resources
	if resources == nil {
		var err error
		resources, err = adminresource.NewStoreFromCapabilitiesWithProtection(options.Capabilities, options.DataProtection)
		if err != nil {
			panic("httpapi: build admin resources: " + err.Error())
		}
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
	var tokens authTokenService
	if options.tokenService != nil {
		tokens = options.tokenService
	} else if options.TokenService != nil {
		tokens = options.TokenService
	} else {
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
	appPhoneCodeGenerator := options.appPhoneCodeGenerator
	if appPhoneCodeGenerator == nil {
		appPhoneCodeGenerator = newAppPhoneDebugCode
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
	rateLimiter := options.RateLimiter
	if rateLimiter == nil {
		rateLimiter = ratelimit.NewMemoryLimiter(ratelimit.MemoryOptions{Now: options.Now})
	}
	rateLimitKeyBuilder := options.RateLimitKeyBuilder
	if rateLimitKeyBuilder == nil {
		rateLimitKeyBuilder, _ = ratelimit.NewKeyBuilder([]byte(defaultRateLimitHMACKey))
	}
	server := &Server{
		router:                   router,
		capabilities:             options.Capabilities,
		resources:                resources,
		sessions:                 sessions,
		cache:                    cacheStore,
		invalidationBus:          options.InvalidationBus,
		cacheStats:               cacheStats,
		cacheTTL:                 cacheTTL,
		fileStorage:              fileStorage,
		uploadPolicy:             normalizeUploadPolicy(options.UploadPolicy),
		internalErrorSink:        options.InternalErrorSink,
		fileCleanupSink:          fileCleanupSink,
		adminIdentityResolver:    options.AdminIdentityResolver,
		adminIdentityBindings:    adminIdentityBindings,
		credentialAuth:           options.CredentialAuth,
		appIdentityResolver:      options.AppIdentityResolver,
		appIdentityBindings:      appIdentityBindings,
		phoneProtector:           options.PhoneProtector,
		phoneVerificationSender:  options.PhoneVerificationSender,
		notificationSMSSender:    options.NotificationSMSSender,
		appPhoneCodeGenerator:    appPhoneCodeGenerator,
		adminStepUpPhoneResolver: options.AdminStepUpPhoneResolver,
		sensitiveReveal:          options.SensitiveReveal,
		debugCodeEnabled:         options.DebugCodeEnabled,
		tokens:                   tokens,
		now:                      now,
		openAPIDocument:          append([]byte(nil), options.OpenAPIDocument...),
		capabilityLockFile:       strings.TrimSpace(options.CapabilityLockFile),
		capabilityConfigSource:   strings.TrimSpace(options.CapabilityConfigSource),
		authorizer:               options.Authorizer,
		allowInsecureHeaderAuth:  options.AllowInsecureHeaderAuth,
		disableDemoAuthProvider:  options.DisableDemoAuthProvider,
		security:                 options.Security,
		rateLimiter:              rateLimiter,
		rateLimitKeyBuilder:      rateLimitKeyBuilder,
		adminMenuServingMode:     options.AdminMenuServingMode,
		adminMenuResolver:        options.AdminMenuResolver,
		adminMenuComparisonSink:  options.AdminMenuComparisonSink,
	}
	if options.ServiceObjects != nil {
		server.serviceObjects = options.ServiceObjects.WithAuthorizer(adminServiceObjectAuthorizer{server: server})
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
	api.GET("/auth/credential-secret-key", s.credentialAuthSecretKey)
	api.POST("/auth/providers/:provider/start", s.authProviderStart)
	api.POST("/auth/sms-otp/start", s.credentialAuthSMSOTPStart)
	api.POST("/auth/login", s.authLogin)
	api.POST("/auth/refresh", s.authRefresh)
	api.POST("/auth/logout", s.authLogout)
	s.registerManifestAppRoutes(api)
	api.GET("/admin/session/current", s.adminCurrentSession)
	s.registerAdminProfileRoutes(api)
	api.GET("/admin/menus", s.adminMenus)
	api.GET("/admin/settings", s.adminSettingsRuntime)
	api.PUT("/admin/settings/:resource/:id", s.adminSettingsUpdate)
	api.GET("/admin/plugin-management/status", s.adminPluginManagementStatus)
	api.POST("/admin/message-center/test-send", s.adminMessageCenterTestSend)
	api.GET("/admin/demo-data", s.adminDemoDataList)
	api.POST("/admin/demo-data/:capability/:dataset/apply", s.adminDemoDataApply)
	api.GET("/admin/policy-reviews/export", s.adminPolicyReviewExport)
	api.POST("/admin/policy-reviews/:id/request", s.adminPolicyReviewRequest)
	api.POST("/admin/policy-reviews/:id/approve", s.adminPolicyReviewApprove)
	api.POST("/admin/policy-reviews/:id/reject", s.adminPolicyReviewReject)
	api.POST("/admin/files/upload", s.adminFileUpload)
	api.GET("/admin/files/:id/content", s.adminFileContent)
	api.POST("/admin/service-objects/query", s.adminServiceObjectQuery)
	api.POST("/admin/service-objects/command", s.adminServiceObjectCommand)
	adminResources := api.Group("/admin/resources")
	adminResources.GET("/:resource/schema", s.adminResourceSchema)
	adminResources.POST("/:resource/query", s.adminResourceQuery)
	adminResources.GET("/:resource", s.adminResourceList)
	adminResources.POST("/:resource", s.adminResourceCreate)
	adminResources.PUT("/:resource/:id", s.adminResourceUpdate)
	adminResources.DELETE("/:resource/:id", s.adminResourceDelete)
	adminResources.POST("/:resource/:id/restore", s.adminResourceRestore)
	adminResources.GET("/:resource/:id/fields/:field/reveal-policy", s.adminSensitiveRevealPolicy)
	adminResources.POST("/:resource/:id/fields/:field/reveal/challenges", s.adminSensitiveRevealChallenge)
	adminResources.POST("/:resource/:id/fields/:field/reveal/challenges/:challenge/factors/oidc/start", s.adminSensitiveRevealOIDCStart)
	adminResources.POST("/:resource/:id/fields/:field/reveal/challenges/:challenge/factors/oidc/complete", s.adminSensitiveRevealOIDCComplete)
	adminResources.POST("/:resource/:id/fields/:field/reveal/challenges/:challenge/factors/sms/start", s.adminSensitiveRevealSMSStart)
	adminResources.POST("/:resource/:id/fields/:field/reveal/challenges/:challenge/factors/sms/complete", s.adminSensitiveRevealSMSComplete)
	adminResources.POST("/:resource/:id/fields/:field/reveal", s.adminSensitiveReveal)
	s.registerAdminRoutes(api, adminRoutes)
}

func (s *Server) health(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, Response[gin.H]{Data: gin.H{"ok": true, "service": "platform-go"}})
}

func (s *Server) enforceRateLimit(ctx *gin.Context, operation ratelimit.Operation, dimensions ...string) bool {
	return s.enforceRateLimitWithSink(ctx, nil, operation, dimensions...)
}

func (s *Server) enforceAdminRateLimit(ctx *gin.Context, operation ratelimit.Operation, dimensions ...string) bool {
	return s.enforceRateLimitWithSink(ctx, s.internalErrorSink, operation, dimensions...)
}

func (s *Server) enforceRateLimitWithSink(ctx *gin.Context, sink InternalErrorSink, operation ratelimit.Operation, dimensions ...string) bool {
	policy, ok := ratelimit.PolicyFor(operation)
	if !ok || s.rateLimiter == nil || s.rateLimitKeyBuilder == nil {
		writePlatformError(ctx, errorcode.CodeRateLimitUnavailable)
		return false
	}
	key, err := s.rateLimitKeyBuilder.Build(operation, dimensions...)
	if err != nil {
		writeRateLimitDependencyError(ctx, sink, err)
		return false
	}
	decision, err := s.rateLimiter.Allow(ctx.Request.Context(), key, policy.Limit, policy.Window)
	if err != nil {
		writeRateLimitDependencyError(ctx, sink, err)
		return false
	}
	if decision.Allowed {
		return true
	}
	retryAfter := int64((decision.RetryAfter + time.Second - 1) / time.Second)
	if retryAfter < 1 {
		retryAfter = 1
	}
	ctx.Header("Retry-After", strconv.FormatInt(retryAfter, 10))
	writePlatformError(ctx, errorcode.CodeRateLimited)
	return false
}

func writeRateLimitDependencyError(ctx *gin.Context, sink InternalErrorSink, cause error) {
	if sink == nil {
		writePlatformError(ctx, errorcode.CodeRateLimitUnavailable)
		return
	}
	writePlatformErrorWithCause(ctx, sink, errorcode.CodeRateLimitUnavailable, cause)
}

func rateLimitClientIP(ctx *gin.Context) string {
	if ctx == nil {
		return ""
	}
	address := net.ParseIP(strings.TrimSpace(ctx.ClientIP()))
	if address == nil {
		return ""
	}
	return address.String()
}

func (s *Server) openapi(ctx *gin.Context) {
	if !s.authorize(ctx, "admin:api-docs:read") {
		return
	}
	if len(s.openAPIDocument) == 0 {
		writePlatformError(ctx, errorcode.CodeOpenAPINotConfigured)
		return
	}
	ctx.Data(http.StatusOK, "application/json; charset=utf-8", s.openAPIDocument)
}

func (s *Server) capabilitiesList(ctx *gin.Context) {
	if !s.authorize(ctx, "admin:capability:read") {
		return
	}
	type resourceItem struct {
		Resource         string                   `json:"resource"`
		Title            capability.LocalizedText `json:"title"`
		Route            string                   `json:"route,omitempty"`
		PermissionPrefix string                   `json:"permissionPrefix,omitempty"`
		ReadOnly         bool                     `json:"readOnly,omitempty"`
	}
	type menuItem struct {
		Route      string                   `json:"route"`
		Title      capability.LocalizedText `json:"title"`
		Permission string                   `json:"permission,omitempty"`
	}
	type item struct {
		ID                capability.ID   `json:"id"`
		Name              string          `json:"name"`
		Version           string          `json:"version"`
		Dependencies      []capability.ID `json:"dependencies,omitempty"`
		AdminResources    []resourceItem  `json:"adminResources,omitempty"`
		MenuRoutes        []menuItem      `json:"menuRoutes,omitempty"`
		Permissions       []string        `json:"permissions,omitempty"`
		ConfigResources   []resourceItem  `json:"configResources,omitempty"`
		ServiceOperations []string        `json:"serviceOperations,omitempty"`
		AuthProviders     []string        `json:"authProviders,omitempty"`
	}
	items := make([]item, 0, len(s.capabilities))
	for _, manifest := range s.capabilities {
		next := item{
			ID:                manifest.ID,
			Name:              manifest.Name,
			Version:           manifest.Version,
			Dependencies:      append([]capability.ID(nil), manifest.Dependencies...),
			ServiceOperations: capabilityServiceOperationIDs(manifest.Service.Operations),
			AuthProviders:     capabilityAuthProviderIDs(manifest.AuthProviders),
		}
		for _, resource := range manifest.Admin.Resources {
			resourceContribution := resourceItem{
				Resource:         resource.Resource,
				Title:            resource.Title,
				Route:            resource.Menu.Route,
				PermissionPrefix: resource.PermissionPrefix,
				ReadOnly:         resource.ReadOnly,
			}
			next.AdminResources = append(next.AdminResources, resourceContribution)
			next.Permissions = append(next.Permissions, capabilityAdminResourcePermissions(resource)...)
			if resource.Menu.Route != "" {
				next.MenuRoutes = append(next.MenuRoutes, menuItem{
					Route:      resource.Menu.Route,
					Title:      resource.Title,
					Permission: resource.PermissionPrefix + ":read",
				})
			}
			if isCapabilityConfigResource(resource) {
				next.ConfigResources = append(next.ConfigResources, resourceContribution)
			}
		}
		items = append(items, next)
	}
	ctx.JSON(http.StatusOK, Response[[]item]{Data: items})
}

func capabilityAdminResourcePermissions(resource capability.AdminResource) []string {
	permissions := []string{resource.PermissionPrefix + ":read"}
	if resource.ReadOnly {
		return permissions
	}
	permissions = append(permissions, resource.PermissionPrefix+":create", resource.PermissionPrefix+":update")
	if resource.Deletion != nil && resource.Deletion.Mode != capability.AdminDeletionDisabled {
		permissions = append(permissions, resource.PermissionPrefix+":delete")
	}
	for _, action := range resource.Actions {
		if action.Permission != "" {
			permissions = append(permissions, action.Permission)
		}
	}
	return permissions
}

func isCapabilityConfigResource(resource capability.AdminResource) bool {
	return resource.Menu.Parent == "configuration" || resource.Resource == "settings"
}

func capabilityServiceOperationIDs(operations []capability.ServiceOperation) []string {
	ids := make([]string, 0, len(operations))
	for _, operation := range operations {
		if operation.ID != "" {
			ids = append(ids, operation.ID)
		}
	}
	return ids
}

func capabilityAuthProviderIDs(providers []capability.AuthProvider) []string {
	ids := make([]string, 0, len(providers))
	for _, provider := range providers {
		if provider.ID != "" {
			ids = append(ids, provider.ID)
		}
	}
	return ids
}

type pluginManagementStatusResponse struct {
	OperationMode             string                   `json:"operationMode"`
	Activation                string                   `json:"activation"`
	ProgressTransport         string                   `json:"progressTransport"`
	RuntimeHotInstall         bool                     `json:"runtimeHotInstall"`
	RuntimeHotUninstall       bool                     `json:"runtimeHotUninstall"`
	RemoteRepositoryPull      bool                     `json:"remoteRepositoryPull"`
	RestartRequiredForChanges bool                     `json:"restartRequiredForChanges"`
	Source                    string                   `json:"source"`
	LockStatus                pluginManagementLockInfo `json:"lockStatus"`
	CurrentCapabilities       []string                 `json:"currentCapabilities"`
	DesiredCapabilities       []string                 `json:"desiredCapabilities"`
	PendingRestart            bool                     `json:"pendingRestart"`
}

type pluginManagementLockInfo struct {
	Configured bool   `json:"configured"`
	Path       string `json:"path,omitempty"`
	Exists     bool   `json:"exists"`
	Valid      bool   `json:"valid"`
	Error      string `json:"error,omitempty"`
}

func (s *Server) adminPluginManagementStatus(ctx *gin.Context) {
	if !s.authorize(ctx, "admin:capability:read") {
		return
	}
	current := s.currentCapabilityIDs()
	desired := append([]string(nil), current...)
	source := strings.TrimSpace(s.capabilityConfigSource)
	if source == "" {
		source = "manual"
	}
	lock := pluginManagementLockInfo{}
	if s.capabilityLockFile != "" {
		lock.Configured = true
		lock.Path = s.capabilityLockFile
		if _, err := os.Stat(s.capabilityLockFile); err == nil {
			lock.Exists = true
		}
		lockFile, err := capability.LoadLockFile(s.capabilityLockFile)
		if err != nil {
			lock.Valid = false
			lock.Error = err.Error()
		} else {
			lock.Valid = true
			desired = capability.IDsToStrings(lockFile.Capabilities)
		}
	}
	ctx.JSON(http.StatusOK, Response[pluginManagementStatusResponse]{Data: pluginManagementStatusResponse{
		OperationMode:             "restart-required-desired-state",
		Activation:                "manual-restart",
		ProgressTransport:         "http-polling",
		RuntimeHotInstall:         false,
		RuntimeHotUninstall:       false,
		RemoteRepositoryPull:      false,
		RestartRequiredForChanges: true,
		Source:                    source,
		LockStatus:                lock,
		CurrentCapabilities:       current,
		DesiredCapabilities:       desired,
		PendingRestart:            lock.Valid && !sameStringSet(current, desired),
	}})
}

func (s *Server) currentCapabilityIDs() []string {
	ids := make([]string, 0, len(s.capabilities))
	for _, manifest := range s.capabilities {
		ids = append(ids, string(manifest.ID))
	}
	return ids
}

func sameStringSet(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	counts := map[string]int{}
	for _, value := range left {
		counts[strings.TrimSpace(value)]++
	}
	for _, value := range right {
		key := strings.TrimSpace(value)
		if counts[key] == 0 {
			return false
		}
		counts[key]--
	}
	return true
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

type adminServiceObjectAuthorizer struct {
	server *Server
}

func (a adminServiceObjectAuthorizer) Can(_ context.Context, execution kernel.ExecutionContext, permission string, action string) bool {
	if a.server == nil || strings.TrimSpace(execution.Actor.Username) == "" {
		return false
	}
	tenant := strings.TrimSpace(execution.TenantScope.TenantCode)
	if execution.TenantScope.PlatformWide {
		tenant = platformTenant
	}
	if tenant == "" {
		return false
	}
	authorizer, ok := a.server.policyAuthorizerForRequest()
	return ok && authorizer.Can(execution.Actor.Username, tenant, permission, action)
}

func (s *Server) adminServiceObjectQuery(ctx *gin.Context) {
	invocation, ok := s.adminServiceObjectInvocation(ctx)
	if !ok {
		return
	}
	if !s.enforceAdminRateLimit(ctx, ratelimit.OperationAdminServiceObjectQuery, rateLimitClientIP(ctx), invocation.Execution.Actor.Username) {
		return
	}
	if s.serviceObjects == nil {
		writeServiceObjectError(ctx, s.internalErrorSink, serviceobject.ErrObjectUnavailable)
		return
	}
	request, err := serviceobject.DecodeQueryRequest(ctx.Request.Body)
	if err != nil {
		writeServiceObjectError(ctx, s.internalErrorSink, err)
		return
	}
	result, err := s.serviceObjects.ExecuteQuery(invocation, request)
	if err != nil {
		writeServiceObjectError(ctx, s.internalErrorSink, err)
		return
	}
	ctx.JSON(http.StatusOK, Response[serviceobject.QueryResult]{Data: result})
}

func (s *Server) adminServiceObjectCommand(ctx *gin.Context) {
	invocation, ok := s.adminServiceObjectInvocation(ctx)
	if !ok {
		return
	}
	if !s.enforceAdminRateLimit(ctx, ratelimit.OperationAdminServiceObjectCommand, rateLimitClientIP(ctx), invocation.Execution.Actor.Username) {
		return
	}
	if s.serviceObjects == nil {
		writeServiceObjectError(ctx, s.internalErrorSink, serviceobject.ErrObjectUnavailable)
		return
	}
	request, err := serviceobject.DecodeCommandRequest(ctx.Request.Body)
	if err != nil {
		writeServiceObjectError(ctx, s.internalErrorSink, err)
		return
	}
	result, err := s.serviceObjects.ExecuteCommand(invocation, request)
	if err != nil {
		writeServiceObjectError(ctx, s.internalErrorSink, err)
		return
	}
	if isAuthorizationServiceObjectCommand(request.CommandID) {
		s.invalidateCachesForResource(ctx.Request.Context(), authorizationInvalidationResource)
	}
	ctx.JSON(http.StatusOK, Response[serviceobject.CommandResult]{Data: result})
}

func isAuthorizationServiceObjectCommand(commandID string) bool {
	if strings.HasSuffix(commandID, ".prepare") {
		return false
	}
	return strings.HasPrefix(commandID, "platform.identity.") ||
		strings.HasPrefix(commandID, "platform.authorization.") ||
		strings.HasPrefix(commandID, "platform.navigation.")
}

func (s *Server) adminServiceObjectInvocation(ctx *gin.Context) (serviceobject.Invocation, bool) {
	if !s.refreshAdminResourceState(ctx, errorcode.CodeAdminAuthStateRefreshFailed) {
		return serviceobject.Invocation{}, false
	}
	principal, ok := s.currentPrincipal(ctx)
	if !ok {
		writePlatformError(ctx, errorcode.CodeAuthUnauthorized)
		return serviceobject.Invocation{}, false
	}
	tenantID := strings.TrimSpace(principal.User.TenantCode)
	var tenantScope kernel.TenantScope
	switch strings.TrimSpace(principal.User.ScopeType) {
	case "platform":
		if tenantID != "" && tenantID != platformTenant {
			writePlatformError(ctx, errorcode.CodeAuthUnauthorized)
			return serviceobject.Invocation{}, false
		}
		tenantID = ""
		tenantScope = kernel.TenantScope{TenantCode: platformTenant, PlatformWide: true}
	case "tenant":
		if tenantID == "" || tenantID == platformTenant || strings.TrimSpace(principal.User.OrgUnitCode) == "" {
			writePlatformError(ctx, errorcode.CodeAuthUnauthorized)
			return serviceobject.Invocation{}, false
		}
		tenantScope = kernel.TenantScope{TenantID: 1, TenantCode: tenantID}
	default:
		writePlatformError(ctx, errorcode.CodeAuthUnauthorized)
		return serviceobject.Invocation{}, false
	}
	principalScope := s.resources.DataScopeForPrincipal(principal)
	return serviceobject.Invocation{
		Execution: kernel.ExecutionContext{
			Context:          ctx.Request.Context(),
			Actor:            kernel.Actor{Username: principal.User.Username, Kind: kernel.ActorKindUser},
			TenantScope:      tenantScope,
			PermissionIntent: kernel.PermissionIntent{Code: "service-object", Action: "execute"},
		},
		TenantID: tenantID,
		Scope: serviceobject.ScopeConstraint{
			All: principalScope.All, Self: principalScope.Self,
			OrgCodes: principalScope.OrgCodes, AreaCodes: principalScope.AreaCodes,
			ActorIdentifiers: principalScope.ActorIdentifiers,
		},
	}, true
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
	ExportedBy string                                    `json:"exportedBy"`
	ExportedAt string                                    `json:"exportedAt"`
	Watermark  adminresource.PolicyReviewExportWatermark `json:"watermark"`
	Reviews    []adminresource.Record                    `json:"reviews"`
	Audits     []adminresource.Record                    `json:"audits"`
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
	Provider     string                          `json:"provider"`
	Username     string                          `json:"username"`
	Code         string                          `json:"code"`
	State        string                          `json:"state"`
	CodeVerifier string                          `json:"codeVerifier"`
	Identifier   credentialAuthIdentifierRequest `json:"identifier"`
	Secret       credentialAuthSecretRequest     `json:"secret"`
	Challenge    credentialAuthChallengeRequest  `json:"challenge"`
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
		writePlatformError(ctx, errorcode.CodeAuthProviderStartInvalid)
		return
	}
	if !s.enforceAdminRateLimit(ctx, ratelimit.OperationAdminOIDCStart, rateLimitClientIP(ctx), strings.ToLower(strings.TrimSpace(ctx.Param("provider")))) {
		return
	}
	provider, ok := s.findAuthProvider(ctx.Param("provider"), capability.AuthProviderAudienceAdmin)
	if !ok {
		writePlatformError(ctx, errorcode.CodeAuthProviderNotFound)
		return
	}
	if !provider.Configured {
		writePlatformError(ctx, errorcode.CodeAuthProviderNotConfigured)
		return
	}
	if provider.Kind != "oidc" {
		writePlatformError(ctx, errorcode.CodeAuthProviderUnsupported)
		return
	}
	if s.adminIdentityResolver == nil {
		writePlatformError(ctx, errorcode.CodeAuthProviderResolverNotConfigured)
		return
	}
	started, err := s.adminIdentityResolver.StartAdminIdentity(ctx.Request.Context(), AdminIdentityStartInput{
		Provider:      provider,
		CodeChallenge: strings.TrimSpace(input.CodeChallenge),
	})
	if errors.Is(err, ErrAdminIdentityInvalid) || errors.Is(err, ErrAdminIdentityTransaction) {
		writePlatformError(ctx, errorcode.CodeAuthProviderStartInvalid)
		return
	}
	if err != nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAuthProviderStartFailed, err)
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
		writePlatformError(ctx, errorcode.CodeAuthInvalidRequest)
		return
	}
	usernameDimension := strings.TrimSpace(input.Username)
	if usernameDimension == "" {
		usernameDimension = "provider-flow"
	}
	providerDimension := strings.ToLower(strings.TrimSpace(input.Provider))
	if providerDimension == "" {
		providerDimension = "unknown"
	}
	if !s.enforceAdminRateLimit(ctx, ratelimit.OperationAdminLogin, rateLimitClientIP(ctx), providerDimension, usernameDimension) {
		return
	}
	if !s.refreshAdminResourceState(ctx, errorcode.CodeAuthStateRefreshFailed) {
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
	switch provider.Kind {
	case "demo":
		username := strings.TrimSpace(input.Username)
		if username == "" {
			username = "admin"
		}
		principal, err := adminresource.ValidateAdminPrincipal(s.resources, username)
		if err != nil {
			writePlatformError(ctx, errorcode.CodeAuthInvalidCredentials)
			return
		}
		s.issueAdminLogin(ctx, principal, provider)
	case "oidc":
		if strings.TrimSpace(input.Code) == "" || strings.TrimSpace(input.State) == "" || strings.TrimSpace(input.CodeVerifier) == "" {
			writePlatformError(ctx, errorcode.CodeAuthIdentityTransactionRequired)
			return
		}
		if s.adminIdentityResolver == nil {
			writePlatformError(ctx, errorcode.CodeAuthProviderResolverNotConfigured)
			return
		}
		identity, err := s.adminIdentityResolver.ResolveAdminIdentity(ctx.Request.Context(), AdminIdentityResolveInput{
			Provider:     provider,
			Code:         strings.TrimSpace(input.Code),
			State:        strings.TrimSpace(input.State),
			CodeVerifier: strings.TrimSpace(input.CodeVerifier),
		})
		if errors.Is(err, ErrAdminIdentityInvalid) {
			writePlatformError(ctx, errorcode.CodeAuthIdentityInvalid)
			return
		}
		if errors.Is(err, ErrAdminIdentityTransaction) {
			writePlatformError(ctx, errorcode.CodeAuthIdentityTransactionInvalid)
			return
		}
		if err != nil {
			writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAuthProviderResolveFailed, err)
			return
		}
		binding, err := s.adminIdentityBindings.ResolveAdminIdentityBinding(ctx.Request.Context(), AdminIdentityBindingInput{
			Provider:        provider,
			Issuer:          identity.Issuer,
			ProviderSubject: identity.ProviderSubject,
			Now:             s.now().UTC(),
		})
		if errors.Is(err, ErrAdminIdentityBindingInvalid) {
			writePlatformError(ctx, errorcode.CodeAuthIdentityNotBound)
			return
		}
		if err != nil {
			writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAuthIdentityBindingFailed, err)
			return
		}
		principal, err := adminresource.ValidateAdminPrincipal(s.resources, binding.Username)
		if err != nil {
			writePlatformError(ctx, errorcode.CodeAuthInvalidCredentials)
			return
		}
		s.issueAdminLogin(ctx, principal, provider)
	case authProviderKindCredentialPassword:
		if !s.requireCredentialAuthSecureTransport(ctx) {
			return
		}
		s.credentialAuthPasswordLogin(ctx, provider, input)
	case authProviderKindCredentialSMSOTP:
		if !s.requireCredentialAuthSecureTransport(ctx) {
			return
		}
		s.credentialAuthSMSOTPLogin(ctx, provider, input)
	default:
		writePlatformError(ctx, errorcode.CodeAuthProviderUnsupported)
	}
}

func (s *Server) issueAdminLogin(ctx *gin.Context, principal rbac.Principal, provider capability.AuthProvider) {
	issued, err := s.sessions.Issue(principal.User.Username)
	if err != nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAuthSessionIssueFailed, err)
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
		if cleanupErr := s.cleanupIssuedAdminSession(ctx.Request.Context(), issued.Token); cleanupErr != nil {
			writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAuthSessionCleanupFailed, cleanupErr)
			return
		}
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAuthTokenSignFailed, err)
		return
	}
	if err := s.recordAudit(ctx, "auth.login", principal.User.ID, principal.User.ID, "success", "authenticated"); err != nil {
		if cleanupErr := s.cleanupIssuedAdminSession(ctx.Request.Context(), issued.Token); cleanupErr != nil {
			writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAuthSessionCleanupFailed, cleanupErr)
			return
		}
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAuthAuditFailed, err)
		return
	}
	ctx.JSON(http.StatusOK, Response[authLoginResponse]{
		Data: authLoginResponse{Token: token, ExpiresAt: issued.ExpiresAt, Principal: principal},
	})
}

func (s *Server) cleanupIssuedAdminSession(ctx context.Context, token string) error {
	revoked, err := s.sessions.RevokeContext(context.WithoutCancel(ctx), token)
	if err != nil {
		return err
	}
	if !revoked {
		return errors.New("issued session cleanup was not acknowledged")
	}
	s.publishSessionInvalidation(context.WithoutCancel(ctx))
	return nil
}

func (s *Server) authRefresh(ctx *gin.Context) {
	authSession, ok, err := s.authSessionFromBearerContext(ctx)
	if err != nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAuthSessionRenewFailed, err)
		return
	}
	if !ok {
		writePlatformError(ctx, errorcode.CodeAuthUnauthorized)
		return
	}
	if !s.refreshAdminResourceState(ctx, errorcode.CodeAuthStateRefreshFailed) {
		return
	}
	principal := s.currentPrincipalForUsername(ctx.Request.Context(), authSession.Username)
	if principal.User.ID == "" || len(principal.Permissions) == 0 {
		writePlatformError(ctx, errorcode.CodeAuthUnauthorized)
		return
	}
	if err := s.recordAudit(ctx, "auth.refresh", principal.User.ID, principal.User.ID, "allowed", "renewal-approved"); err != nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAuthAuditFailed, err)
		return
	}
	renewed, ok, err := s.sessions.RenewContext(ctx.Request.Context(), authSession.Token)
	if err != nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAuthSessionRenewFailed, err)
		return
	}
	if !ok {
		writePlatformError(ctx, errorcode.CodeAuthUnauthorized)
		return
	}
	s.publishSessionInvalidation(ctx.Request.Context())
	tokenTTL := renewed.ExpiresAt.Sub(s.now().UTC())
	token, _, err := s.tokens.Sign(authjwt.Subject{
		UserID:    principal.User.ID,
		TenantID:  platformTenant,
		Username:  principal.User.Username,
		SessionID: renewed.Token,
		TokenType: authjwt.TokenTypeAdmin,
	}, tokenTTL)
	if err != nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAuthTokenSignFailed, err)
		return
	}
	ctx.JSON(http.StatusOK, Response[authLoginResponse]{
		Data: authLoginResponse{Token: token, ExpiresAt: renewed.ExpiresAt, Principal: principal},
	})
}

func (s *Server) authLogout(ctx *gin.Context) {
	authSession, ok, err := s.authSessionFromBearerContext(ctx)
	if err != nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAuthSessionRevokeFailed, err)
		return
	}
	if !ok {
		writePlatformError(ctx, errorcode.CodeAuthUnauthorized)
		return
	}
	if !s.refreshAdminResourceState(ctx, errorcode.CodeAuthStateRefreshFailed) {
		return
	}
	principal := s.currentPrincipalForUsername(ctx.Request.Context(), authSession.Username)
	if principal.User.ID == "" {
		writePlatformError(ctx, errorcode.CodeAuthUnauthorized)
		return
	}
	if err := s.recordAudit(ctx, "auth.logout", principal.User.ID, principal.User.ID, "allowed", "revocation-approved"); err != nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAuthAuditFailed, err)
		return
	}
	revoked, err := s.sessions.RevokeContext(ctx.Request.Context(), authSession.Token)
	if err != nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAuthSessionRevokeFailed, err)
		return
	}
	if !revoked {
		writePlatformError(ctx, errorcode.CodeAuthUnauthorized)
		return
	}
	s.publishSessionInvalidation(ctx.Request.Context())
	ctx.JSON(http.StatusOK, Response[gin.H]{Data: gin.H{"revoked": true}})
}

func (s *Server) appAuthLogin(ctx *gin.Context) {
	var input appLoginRequest
	if err := ctx.ShouldBindJSON(&input); err != nil {
		writePlatformError(ctx, errorcode.CodeAppAuthInvalidRequest)
		return
	}
	providerDimension := strings.ToLower(strings.TrimSpace(input.Provider))
	if providerDimension == "" {
		providerDimension = "local"
	}
	if !s.enforceRateLimit(ctx, ratelimit.OperationAppLogin, rateLimitClientIP(ctx), providerDimension, appUsername(input.Username)) {
		return
	}
	username, _, ok := s.resolveAppLoginIdentity(ctx, input)
	if !ok {
		return
	}
	issued, err := s.sessions.Issue(username)
	if err != nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAppAuthSessionIssueFailed, err)
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
		if cleanupErr := s.cleanupIssuedAdminSession(ctx.Request.Context(), issued.Token); cleanupErr != nil {
			writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAppAuthSessionCleanupFailed, cleanupErr)
			return
		}
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAppAuthTokenSignFailed, err)
		return
	}
	actorID := appUserID(username)
	if err := s.recordAudit(ctx, "app.auth.login", actorID, actorID, "success", "authenticated"); err != nil {
		if cleanupErr := s.cleanupIssuedAdminSession(ctx.Request.Context(), issued.Token); cleanupErr != nil {
			writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAppAuthSessionCleanupFailed, cleanupErr)
			return
		}
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAppAuthAuditFailed, err)
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
		provider, ok := s.findAuthProvider("demo", capability.AuthProviderAudienceApp)
		if !ok || provider.Kind != "demo" {
			writePlatformError(ctx, errorcode.CodeAppAuthProviderNotFound)
			return "", "", false
		}
		if !provider.Configured {
			writePlatformError(ctx, errorcode.CodeAppAuthProviderNotConfigured)
			return "", "", false
		}
		return appUsername(input.Username), "", true
	}
	provider, ok := s.findAuthProvider(providerID, capability.AuthProviderAudienceApp)
	if !ok || !provider.Enabled {
		writePlatformError(ctx, errorcode.CodeAppAuthProviderNotFound)
		return "", "", false
	}
	if !provider.Configured {
		writePlatformError(ctx, errorcode.CodeAppAuthProviderNotConfigured)
		return "", "", false
	}
	if provider.Kind == "wechat" && strings.TrimSpace(input.Code) == "" {
		writePlatformError(ctx, errorcode.CodeAppAuthCodeRequired)
		return "", "", false
	}
	if s.appIdentityResolver == nil {
		writePlatformError(ctx, errorcode.CodeAppAuthProviderResolverNotConfigured)
		return "", "", false
	}
	identity, err := s.appIdentityResolver.ResolveAppIdentity(ctx.Request.Context(), AppIdentityResolveInput{
		Provider:     provider,
		Code:         strings.TrimSpace(input.Code),
		UsernameHint: strings.TrimSpace(input.Username),
	})
	if errors.Is(err, ErrAppIdentityInvalid) {
		writePlatformError(ctx, errorcode.CodeAppAuthIdentityInvalid)
		return "", "", false
	}
	if err != nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAppAuthProviderResolveFailed, err)
		return "", "", false
	}
	if strings.TrimSpace(identity.ProviderSubject) == "" {
		writePlatformError(ctx, errorcode.CodeAppAuthIdentityInvalid)
		return "", "", false
	}
	binding, err := s.appIdentityBindings.ResolveAppIdentityBinding(ctx.Request.Context(), AppIdentityBindingInput{
		Provider:        provider,
		ProviderSubject: strings.TrimSpace(identity.ProviderSubject),
		UsernameHint:    strings.TrimSpace(identity.Username),
		Now:             s.now(),
	})
	if errors.Is(err, ErrAppIdentityInvalid) {
		writePlatformError(ctx, errorcode.CodeAppAuthIdentityInvalid)
		return "", "", false
	}
	if err != nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAppAuthIdentityBindingFailed, err)
		return "", "", false
	}
	if strings.TrimSpace(binding.Username) == "" {
		writePlatformError(ctx, errorcode.CodeAppAuthIdentityInvalid)
		return "", "", false
	}
	return appUsername(binding.Username), provider.ID, true
}

func (s *Server) appAuthLogout(ctx *gin.Context) {
	appSession, ok, err := s.appSessionFromBearerContext(ctx)
	if err != nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAppAuthSessionRevokeFailed, err)
		return
	}
	if !ok {
		writePlatformError(ctx, errorcode.CodeAuthUnauthorized)
		return
	}
	actorID := appUserID(appSession.Username)
	if err := s.recordAudit(ctx, "app.auth.logout", actorID, actorID, "allowed", "revocation-approved"); err != nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAppAuthAuditFailed, err)
		return
	}
	revoked, err := s.sessions.RevokeContext(ctx.Request.Context(), appSession.Token)
	if err != nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAppAuthSessionRevokeFailed, err)
		return
	}
	if !revoked {
		writePlatformError(ctx, errorcode.CodeAuthUnauthorized)
		return
	}
	s.publishSessionInvalidation(ctx.Request.Context())
	ctx.JSON(http.StatusOK, Response[gin.H]{Data: gin.H{"revoked": true}})
}

func (s *Server) appCurrentSession(ctx *gin.Context) {
	appSession, ok := s.appSessionFromBearer(ctx)
	if !ok {
		writePlatformError(ctx, errorcode.CodeAuthUnauthorized)
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
		writeAdminResourceError(ctx, s.internalErrorSink, err)
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
			writeAdminResourceError(ctx, s.internalErrorSink, err)
			return
		}
		items, err = s.projectAdminResourceRecords(resource, items, adminresource.ProjectionResponse)
		if err != nil {
			writeAdminResourceError(ctx, s.internalErrorSink, err)
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
		writeAdminResourceError(ctx, s.internalErrorSink, err)
		return
	}
	items, err = s.projectAdminResourceRecords(resource, items, adminresource.ProjectionResponse)
	if err != nil {
		writeAdminResourceError(ctx, s.internalErrorSink, err)
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
		writeAdminResourceError(ctx, s.internalErrorSink, adminresource.ErrInvalidRecord)
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
		writeAdminResourceError(ctx, s.internalErrorSink, err)
		return
	}
	ctx.JSON(http.StatusOK, Response[adminResourceQueryResponse]{
		Data: adminResourceQueryResponse{
			Resource: result.Resource,
			Items:    result.Items,
			Total:    result.Total,
			Page:     result.Page,
			PageSize: result.PageSize,
		},
	})
}

type adminResourceWriteRequest struct {
	Code        string                     `json:"code"`
	Name        string                     `json:"name"`
	Status      string                     `json:"status"`
	Description string                     `json:"description"`
	Values      map[string]json.RawMessage `json:"values"`
}

func bindAdminResourceWriteInput(ctx *gin.Context) (adminresource.WriteInput, error) {
	var request adminResourceWriteRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		return adminresource.WriteInput{}, err
	}
	values, err := normalizeAdminResourceWriteValues(request.Values)
	if err != nil {
		return adminresource.WriteInput{}, err
	}
	return adminresource.WriteInput{
		Code:        request.Code,
		Name:        request.Name,
		Status:      request.Status,
		Description: request.Description,
		Values:      values,
	}, nil
}

func normalizeAdminResourceWriteValues(rawValues map[string]json.RawMessage) (map[string]string, error) {
	if len(rawValues) == 0 {
		return nil, nil
	}
	values := make(map[string]string, len(rawValues))
	for key, raw := range rawValues {
		value, err := normalizeAdminResourceWriteValue(raw)
		if err != nil {
			return nil, err
		}
		values[key] = value
	}
	return values, nil
}

func normalizeAdminResourceWriteValue(raw json.RawMessage) (string, error) {
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return "", err
	}
	switch typed := value.(type) {
	case nil:
		return "", nil
	case string:
		return typed, nil
	case bool:
		return strconv.FormatBool(typed), nil
	case json.Number:
		return typed.String(), nil
	case []any, map[string]any:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return "", err
		}
		return string(encoded), nil
	default:
		return "", adminresource.ErrInvalidRecord
	}
}

func (s *Server) adminResourceCreate(ctx *gin.Context) {
	resource := ctx.Param("resource")
	if !s.authorizeAdminResource(ctx, resource, "create") {
		return
	}
	input, err := bindAdminResourceWriteInput(ctx)
	if err != nil {
		writeAdminResourceError(ctx, s.internalErrorSink, adminresource.ErrInvalidRecord)
		return
	}
	if resource == apiTokensResource {
		issued, token, err := s.issueAdminAPIToken(ctx.Request.Context(), s.auditActorID(ctx), input)
		if err != nil {
			writeAdminResourceError(ctx, s.internalErrorSink, err)
			return
		}
		s.invalidateCachesForResource(ctx.Request.Context(), resource)
		projected, err := s.resources.ProjectRecord(resource, issued, adminresource.ProjectionResponse)
		if err != nil {
			writeAdminResourceError(ctx, s.internalErrorSink, err)
			return
		}
		ctx.JSON(http.StatusCreated, Response[adminResourceRecordResponse]{
			Data: adminResourceRecordResponse{Resource: resource, Record: projected, Token: token},
		})
		return
	}
	mutation, err := s.resources.CreateWithAudit(resource, input, s.mutationAuditEvent(ctx, "admin_resource.create", resource, "created"))
	if err != nil {
		writeAdminResourceError(ctx, s.internalErrorSink, err)
		return
	}
	record := mutation.Record
	s.invalidateCachesForResource(ctx.Request.Context(), resource)
	ctx.JSON(http.StatusCreated, Response[adminResourceRecordResponse]{
		Data: adminResourceRecordResponse{Resource: resource, Record: record},
	})
}

func (s *Server) adminResourceUpdate(ctx *gin.Context) {
	resource := ctx.Param("resource")
	id := ctx.Param("id")
	if !s.authorizeAdminResource(ctx, resource, "update") {
		return
	}
	input, err := bindAdminResourceWriteInput(ctx)
	if err != nil {
		writeAdminResourceError(ctx, s.internalErrorSink, adminresource.ErrInvalidRecord)
		return
	}
	if resource == apiTokensResource {
		record, err := s.updateAdminAPIToken(ctx.Request.Context(), s.auditActorID(ctx), id, input)
		if err != nil {
			writeAdminResourceError(ctx, s.internalErrorSink, err)
			return
		}
		s.invalidateCachesForResource(ctx.Request.Context(), resource)
		projected, err := s.resources.ProjectRecord(resource, record, adminresource.ProjectionResponse)
		if err != nil {
			writeAdminResourceError(ctx, s.internalErrorSink, err)
			return
		}
		ctx.JSON(http.StatusOK, Response[adminResourceRecordResponse]{
			Data: adminResourceRecordResponse{Resource: resource, Record: projected},
		})
		return
	}
	mutation, err := s.resources.UpdateWithAudit(resource, id, input, s.mutationAuditEvent(ctx, "admin_resource.update", resource, "updated"))
	if err != nil {
		writeAdminResourceError(ctx, s.internalErrorSink, err)
		return
	}
	record := mutation.Record
	s.invalidateCachesForResource(ctx.Request.Context(), resource)
	ctx.JSON(http.StatusOK, Response[adminResourceRecordResponse]{
		Data: adminResourceRecordResponse{Resource: resource, Record: record},
	})
}

func (s *Server) adminPolicyReviewApprove(ctx *gin.Context) {
	if !s.authorizeAdminResource(ctx, "policy-reviews", "update") {
		return
	}
	userCode := s.businessUserCode(ctx)
	if userCode == "" {
		writePlatformError(ctx, errorcode.CodeAdminForbidden)
		return
	}
	result, err := s.resources.ApprovePolicyReviewContext(ctx.Request.Context(), ctx.Param("id"), userCode, s.auditActorID(ctx))
	if err != nil {
		writeAdminResourceError(ctx, s.internalErrorSink, err)
		return
	}
	for _, resource := range []string{"policy-reviews", "roles", "audit-logs"} {
		s.invalidateCachesForResource(ctx.Request.Context(), resource)
	}
	review, err := s.resources.ProjectRecord("policy-reviews", result.Review, adminresource.ProjectionResponse)
	if err != nil {
		writeAdminResourceError(ctx, s.internalErrorSink, err)
		return
	}
	role, err := s.resources.ProjectRecord("roles", result.Role, adminresource.ProjectionResponse)
	if err != nil {
		writeAdminResourceError(ctx, s.internalErrorSink, err)
		return
	}
	audit, err := s.resources.ProjectRecord("audit-logs", result.Audit, adminresource.ProjectionResponse)
	if err != nil {
		writeAdminResourceError(ctx, s.internalErrorSink, err)
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
	userCode := s.businessUserCode(ctx)
	if userCode == "" {
		writePlatformError(ctx, errorcode.CodeAdminForbidden)
		return
	}
	result, err := s.resources.RequestPolicyReviewContext(ctx.Request.Context(), ctx.Param("id"), userCode, s.auditActorID(ctx))
	if err != nil {
		writeAdminResourceError(ctx, s.internalErrorSink, err)
		return
	}
	for _, resource := range []string{"policy-reviews", "audit-logs"} {
		s.invalidateCachesForResource(ctx.Request.Context(), resource)
	}
	review, err := s.resources.ProjectRecord("policy-reviews", result.Review, adminresource.ProjectionResponse)
	if err != nil {
		writeAdminResourceError(ctx, s.internalErrorSink, err)
		return
	}
	audit, err := s.resources.ProjectRecord("audit-logs", result.Audit, adminresource.ProjectionResponse)
	if err != nil {
		writeAdminResourceError(ctx, s.internalErrorSink, err)
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
		writeAdminResourceError(ctx, s.internalErrorSink, adminresource.ErrInvalidRecord)
		return
	}
	userCode := s.businessUserCode(ctx)
	if userCode == "" {
		writePlatformError(ctx, errorcode.CodeAdminForbidden)
		return
	}
	result, err := s.resources.RejectPolicyReviewContext(ctx.Request.Context(), ctx.Param("id"), userCode, s.auditActorID(ctx), input.Reason)
	if err != nil {
		writeAdminResourceError(ctx, s.internalErrorSink, err)
		return
	}
	for _, resource := range []string{"policy-reviews", "audit-logs"} {
		s.invalidateCachesForResource(ctx.Request.Context(), resource)
	}
	review, err := s.resources.ProjectRecord("policy-reviews", result.Review, adminresource.ProjectionResponse)
	if err != nil {
		writeAdminResourceError(ctx, s.internalErrorSink, err)
		return
	}
	audit, err := s.resources.ProjectRecord("audit-logs", result.Audit, adminresource.ProjectionResponse)
	if err != nil {
		writeAdminResourceError(ctx, s.internalErrorSink, err)
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
	if !s.authorize(ctx, "admin:policy-review:export") {
		return
	}
	userCode := s.businessUserCode(ctx)
	if userCode == "" {
		writePlatformError(ctx, errorcode.CodeAdminForbidden)
		return
	}
	watermarkApplied := false
	query, queryErr := url.ParseQuery(ctx.Request.URL.RawQuery)
	if queryErr != nil {
		writePlatformError(ctx, errorcode.CodeAdminPolicyReviewWatermarkInvalid)
		return
	}
	if values, exists := query["watermark"]; exists {
		if len(values) != 1 || (values[0] != "true" && values[0] != "false") {
			writePlatformError(ctx, errorcode.CodeAdminPolicyReviewWatermarkInvalid)
			return
		}
		watermarkApplied = values[0] == "true"
	}
	result, err := s.resources.ExportPolicyReviewsContext(ctx.Request.Context(), userCode, s.auditActorID(ctx), watermarkApplied)
	if err != nil {
		writeAdminResourceError(ctx, s.internalErrorSink, err)
		return
	}
	s.invalidateCachesForResource(ctx.Request.Context(), "audit-logs")
	ctx.JSON(http.StatusOK, Response[policyReviewExportResponse]{
		Data: policyReviewExportResponse{
			ExportedBy: result.ExportedBy,
			ExportedAt: result.ExportedAt,
			Watermark:  result.Watermark,
			Reviews:    result.Reviews,
			Audits:     result.Audits,
		},
	})
}

func (s *Server) adminResourceDelete(ctx *gin.Context) {
	resource := ctx.Param("resource")
	if !s.authorizeAdminResource(ctx, resource, "delete") {
		return
	}
	if adminresource.RequiresGovernedLifecycleCommand(resource) {
		writeAdminResourceError(ctx, s.internalErrorSink, adminresource.ErrDomainOwnedMutation)
		return
	}
	if resource == "files" {
		s.deleteAdminFile(ctx, ctx.Param("id"))
		return
	}
	if resource == apiTokensResource {
		if err := s.revokeAdminAPIToken(ctx.Request.Context(), s.auditActorID(ctx), ctx.Param("id")); err != nil {
			writeAdminResourceError(ctx, s.internalErrorSink, err)
			return
		}
		s.invalidateCachesForResource(ctx.Request.Context(), resource)
		ctx.JSON(http.StatusOK, Response[gin.H]{Data: gin.H{"resource": resource, "revoked": true}})
		return
	}
	if _, err := s.resources.DeleteWithAudit(resource, ctx.Param("id"), s.mutationAuditEvent(ctx, "admin_resource.delete", resource, "deleted")); err != nil {
		writeAdminResourceError(ctx, s.internalErrorSink, err)
		return
	}
	s.invalidateCachesForResource(ctx.Request.Context(), resource)
	ctx.JSON(http.StatusOK, Response[gin.H]{Data: gin.H{"resource": resource, "deleted": true}})
}

func (s *Server) adminResourceRestore(ctx *gin.Context) {
	resource := ctx.Param("resource")
	principal, hasPrincipal, ok := s.authorizeAdminResourcePrincipal(ctx, resource, "restore")
	if !ok {
		return
	}
	if adminresource.RequiresGovernedLifecycleCommand(resource) {
		writeAdminResourceError(ctx, s.internalErrorSink, adminresource.ErrDomainOwnedMutation)
		return
	}
	if hasPrincipal {
		if _, err := s.resources.InternalRecordForPrincipal(resource, ctx.Param("id"), principal); err != nil {
			writeAdminResourceError(ctx, s.internalErrorSink, err)
			return
		}
	}
	if resource == "files" {
		s.restoreAdminFile(ctx, ctx.Param("id"))
		return
	}
	result, err := s.resources.RestoreWithAudit(resource, ctx.Param("id"), s.mutationAuditEvent(ctx, "admin_resource.restore", resource, "restored"))
	if err != nil {
		writeAdminResourceError(ctx, s.internalErrorSink, err)
		return
	}
	s.invalidateCachesForResource(ctx.Request.Context(), resource)
	ctx.JSON(http.StatusOK, Response[gin.H]{Data: gin.H{
		"resource": resource,
		"restored": true,
		"record":   result.Record,
	}})
}

func (s *Server) restoreAdminFile(ctx *gin.Context, id string) {
	record, err := s.resources.InternalRecord("files", id)
	if err != nil {
		writeAdminResourceError(ctx, s.internalErrorSink, err)
		return
	}
	key := fileStorageKey(record)
	if key == "" {
		writePlatformError(ctx, errorcode.CodeAdminFileRestoreUnavailable)
		return
	}
	body, err := s.fileStorage.Open(ctx.Request.Context(), key)
	if errors.Is(err, storage.ErrObjectNotFound) {
		writePlatformError(ctx, errorcode.CodeAdminFileRestoreUnavailable)
		return
	}
	if err != nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAdminFileRestoreFailed, err)
		return
	}
	_ = body.Close()
	result, err := s.resources.RestoreWithAudit("files", id, s.mutationAuditEvent(ctx, "file.restore", "files", "restored"))
	if err != nil {
		writeAdminResourceError(ctx, s.internalErrorSink, err)
		return
	}
	s.invalidateCachesForResource(ctx.Request.Context(), "files")
	ctx.JSON(http.StatusOK, Response[gin.H]{Data: gin.H{
		"resource": "files",
		"restored": true,
		"record":   result.Record,
	}})
}

func (s *Server) adminFileUpload(ctx *gin.Context) {
	if !s.authorizeAdminResource(ctx, "files", "create") {
		return
	}
	if !s.enforceAdminRateLimit(ctx, ratelimit.OperationAdminUpload, rateLimitClientIP(ctx), s.auditActorID(ctx)) {
		return
	}
	upload, err := readValidatedUpload(ctx, s.uploadPolicy, adminUploadErrorCodes)
	if err != nil {
		writePlatformError(ctx, uploadPolicyErrorCode(err))
		return
	}
	defer upload.Close()

	metadata, err := s.fileStorage.Save(ctx.Request.Context(), storage.ObjectSaveInput{
		FileName:    upload.FileName,
		ContentType: upload.ContentType,
		Reader:      upload.Reader,
	})
	if err != nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAdminFileSaveFailed, err)
		return
	}
	mutation, err := s.resources.CreateInternalWithAudit("files", adminresource.WriteInput{
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
	}, s.mutationAuditEvent(ctx, "file.upload", "files", "uploaded"))
	if err != nil {
		if rollbackErr := s.fileStorage.Delete(ctx.Request.Context(), metadata.Key); rollbackErr != nil {
			s.recordFileCleanup(ctx, metadata.Key)
			writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAdminFileRollbackFailed, rollbackErr)
			return
		}
		writeAdminResourceError(ctx, s.internalErrorSink, err)
		return
	}
	record := mutation.Record
	s.invalidateCachesForResource(ctx.Request.Context(), "files")
	projected, err := s.resources.ProjectRecord("files", record, adminresource.ProjectionResponse)
	if err != nil {
		writeAdminResourceError(ctx, s.internalErrorSink, err)
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
		writeAdminResourceError(ctx, s.internalErrorSink, err)
		return
	}
	key := fileStorageKey(record)
	if key == "" {
		writePlatformError(ctx, errorcode.CodeAdminFileObjectNotFound)
		return
	}
	body, err := s.fileStorage.Open(ctx.Request.Context(), key)
	if errors.Is(err, storage.ErrObjectNotFound) {
		writePlatformError(ctx, errorcode.CodeAdminFileObjectNotFound)
		return
	}
	if err != nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAdminFileOpenFailed, err)
		return
	}
	defer body.Close()
	if err := s.recordFileAudit(ctx, "file.content", record); err != nil {
		writeAdminResourceError(ctx, s.internalErrorSink, err)
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
		writePlatformError(ctx, errorcode.CodeAuthUnauthorized)
		return
	}
	username := appUsername(appSession.Username)
	if !s.enforceRateLimit(ctx, ratelimit.OperationAppUpload, rateLimitClientIP(ctx), username) {
		return
	}
	upload, err := readValidatedUpload(ctx, s.uploadPolicy, appUploadErrorCodes)
	if err != nil {
		writePlatformError(ctx, uploadPolicyErrorCode(err))
		return
	}
	defer upload.Close()

	metadata, err := s.fileStorage.Save(ctx.Request.Context(), storage.ObjectSaveInput{
		FileName:    upload.FileName,
		ContentType: upload.ContentType,
		Reader:      upload.Reader,
	})
	if err != nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAppFileSaveFailed, err)
		return
	}
	mutation, err := s.resources.CreateInternalWithAudit("files", adminresource.WriteInput{
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
			"createdAt":     s.now().UTC().Format(time.RFC3339),
		},
	}, requestAuditEvent(ctx.Request.Context(), adminresource.AuditEvent{Actor: appUserID(username), Action: "file.upload", Resource: "files", Result: "success", ReasonCode: "uploaded"}))
	if err != nil {
		if rollbackErr := s.fileStorage.Delete(ctx.Request.Context(), metadata.Key); rollbackErr != nil {
			s.recordFileCleanup(ctx, metadata.Key)
			writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAppFileRollbackFailed, rollbackErr)
			return
		}
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAppFileMetadataFailed, err)
		return
	}
	record := mutation.Record
	s.invalidateCachesForResource(ctx.Request.Context(), "files")
	projected, err := s.resources.ProjectRecord("files", record, adminresource.ProjectionResponse)
	if err != nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAppFileMetadataFailed, err)
		return
	}
	ctx.JSON(http.StatusCreated, Response[adminResourceRecordResponse]{
		Data: adminResourceRecordResponse{Resource: "files", Record: projected},
	})
}

func (s *Server) appFileContent(ctx *gin.Context) {
	appSession, ok := AppSessionFromContext(ctx)
	if !ok {
		writePlatformError(ctx, errorcode.CodeAuthUnauthorized)
		return
	}
	if !s.refreshAdminResourceState(ctx, errorcode.CodeAppFileStateRefreshFailed) {
		return
	}
	record, err := s.adminResourceRecordByID("files", ctx.Param("id"))
	if err != nil {
		code := appFileRecordErrorCode(err)
		if code == errorcode.CodeAppFileNotFound {
			writePlatformError(ctx, code)
		} else {
			writePlatformErrorWithCause(ctx, s.internalErrorSink, code, err)
		}
		return
	}
	username := appUsername(appSession.Username)
	if !appFileVisibleToSession(record, username) {
		writePlatformError(ctx, errorcode.CodeAppFileNotFound)
		return
	}
	key := fileStorageKey(record)
	if key == "" {
		writePlatformError(ctx, errorcode.CodeAppFileObjectNotFound)
		return
	}
	body, err := s.fileStorage.Open(ctx.Request.Context(), key)
	if errors.Is(err, storage.ErrObjectNotFound) {
		writePlatformError(ctx, errorcode.CodeAppFileObjectNotFound)
		return
	}
	if err != nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAppFileOpenFailed, err)
		return
	}
	defer body.Close()
	if err := s.recordFileAuditForActor(ctx, "file.content", appUserID(username), record); err != nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAppFileMetadataFailed, err)
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
		return ownerID == appUserID(username) || ownerID == legacyAppUserID(username)
	}
	return strings.TrimSpace(record.Values["uploadedBy"]) == username
}

func appFileRecordErrorCode(err error) errorcode.Code {
	if errors.Is(err, adminresource.ErrRecordNotFound) || errors.Is(err, adminresource.ErrUnknownResource) {
		return errorcode.CodeAppFileNotFound
	}
	return errorcode.CodeAppFileMetadataFailed
}

func legacyAppUserID(username string) string {
	return "app:" + appUsername(username)
}

func (s *Server) deleteAdminFile(ctx *gin.Context, id string) {
	mutation, err := s.resources.TombstoneFileWithAudit(id, s.mutationAuditEvent(ctx, "file.delete.request", "files", "cleanup-pending"))
	if err != nil {
		writeAdminResourceError(ctx, s.internalErrorSink, err)
		return
	}
	s.invalidateCachesForResource(ctx.Request.Context(), "files")
	ctx.JSON(http.StatusOK, Response[gin.H]{Data: gin.H{
		"resource":   "files",
		"deleted":    true,
		"mode":       capability.AdminDeletionTombstone,
		"purgeAfter": mutation.Record.PurgeAfter,
	}})
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
	mutation, err := s.resources.CreateInternalWithAudit(apiTokensResource, input, requestAuditEvent(ctx, adminresource.AuditEvent{
		Actor: actor, Action: "api_token.create", Resource: apiTokensResource, Result: "success", ReasonCode: "issued",
	}))
	if err != nil {
		return adminresource.Record{}, "", err
	}
	return mutation.Record, token, nil
}

func (s *Server) revokeAdminAPIToken(ctx context.Context, actor string, id string) error {
	record, err := s.adminResourceRecordByID(apiTokensResource, id)
	if err != nil {
		return err
	}
	values := cloneStringMap(record.Values)
	values["revokedAt"] = s.now().UTC().Format(time.RFC3339)
	_, err = s.resources.UpdateInternalWithAudit(apiTokensResource, id, adminresource.WriteInput{
		Code:        record.Code,
		Name:        record.Name,
		Status:      "revoked",
		Description: record.Description,
		Values:      values,
	}, requestAuditEvent(ctx, adminresource.AuditEvent{Actor: actor, Action: "api_token.revoke", Resource: apiTokensResource, Result: "success", ReasonCode: "revoked"}))
	if err != nil {
		return err
	}
	return nil
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
	mutation, err := s.resources.UpdateInternalWithAudit(apiTokensResource, id, input, requestAuditEvent(ctx, adminresource.AuditEvent{
		Actor: actor, Action: "api_token.update", Resource: apiTokensResource, Result: "success", ReasonCode: "updated",
	}))
	if err != nil {
		return adminresource.Record{}, err
	}
	return mutation.Record, nil
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
	record, ok := s.resolveAPITokenRecord(token)
	if !ok {
		return nil, false
	}
	return splitAPITokenScopes(record.Values["scope"]), true
}

func (s *Server) resolveAPITokenRecord(token string) (adminresource.Record, bool) {
	token = strings.TrimSpace(token)
	if token == "" || !strings.HasPrefix(token, apiTokenPrefix) {
		return adminresource.Record{}, false
	}
	tokenPrefix := apiTokenPrefixValue(token)
	tokenHash := hashAPIToken(token)
	records, err := s.resources.List(apiTokensResource)
	if err != nil {
		return adminresource.Record{}, false
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
			return adminresource.Record{}, false
		}
		return record, true
	}
	return adminresource.Record{}, false
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

func (s *Server) auditActorID(ctx *gin.Context) string {
	if ctx == nil {
		return systemActorID
	}
	if token, ok := bearerToken(ctx.GetHeader("Authorization")); ok && strings.HasPrefix(token, apiTokenPrefix) {
		if record, valid := s.resolveAPITokenRecord(token); valid {
			return record.ID
		}
	}
	if principal, ok := ctx.Get("platform.principal"); ok {
		if typed, ok := principal.(rbac.Principal); ok && strings.TrimSpace(typed.User.ID) != "" {
			return strings.TrimSpace(typed.User.ID)
		}
	}
	if principal, ok := ctx.Get("principal"); ok {
		if typed, ok := principal.(rbac.Principal); ok && strings.TrimSpace(typed.User.ID) != "" {
			return strings.TrimSpace(typed.User.ID)
		}
	}
	if principal, ok := s.currentPrincipal(ctx); ok && strings.TrimSpace(principal.User.ID) != "" {
		return strings.TrimSpace(principal.User.ID)
	}
	return systemActorID
}

func (s *Server) businessUserCode(ctx *gin.Context) string {
	if ctx == nil {
		return ""
	}
	for _, key := range []string{"platform.principal", "principal"} {
		if value, ok := ctx.Get(key); ok {
			if principal, ok := value.(rbac.Principal); ok && strings.TrimSpace(principal.User.Username) != "" {
				return strings.TrimSpace(principal.User.Username)
			}
		}
	}
	if principal, ok := s.currentPrincipal(ctx); ok {
		return strings.TrimSpace(principal.User.Username)
	}
	return ""
}

func (s *Server) adminCurrentSession(ctx *gin.Context) {
	if !s.refreshAdminResourceState(ctx, errorcode.CodeAdminAuthStateRefreshFailed) {
		return
	}
	principal, ok := s.currentPrincipal(ctx)
	if !ok {
		writePlatformError(ctx, errorcode.CodeAuthUnauthorized)
		return
	}
	ctx.JSON(http.StatusOK, Response[rbac.Principal]{Data: principal})
}

func (s *Server) adminMenus(ctx *gin.Context) {
	if !s.refreshAdminResourceState(ctx, errorcode.CodeAdminAuthStateRefreshFailed) {
		return
	}
	principal, ok := s.currentPrincipal(ctx)
	if !ok {
		writePlatformError(ctx, errorcode.CodeAuthUnauthorized)
		return
	}
	items, err := s.resolveAdminMenus(ctx.Request.Context(), principal)
	if err != nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAdminMenuResolutionFailed, err)
		return
	}
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
		writePlatformError(ctx, errorcode.CodeAdminDemoDataNotFound)
		return
	}
	result, err := s.resources.ApplyDemoDataSet(dataset)
	if err != nil {
		writeAdminResourceError(ctx, s.internalErrorSink, err)
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

func (s *Server) recordAudit(ctx *gin.Context, action string, actorID string, targetID string, outcome string, reasonCode string) error {
	_, err := s.resources.RecordAudit(requestAuditEvent(ctx.Request.Context(), adminresource.AuditEvent{
		Actor: actorID, Action: action, Resource: "auth", TargetID: targetID,
		Result: outcome, ReasonCode: reasonCode,
	}))
	return err
}

func (s *Server) recordFileAudit(ctx *gin.Context, action string, record adminresource.Record) error {
	return s.recordFileAuditForActor(ctx, action, s.auditActorID(ctx), record)
}

func (s *Server) recordFileAuditForActor(ctx *gin.Context, action string, actor string, record adminresource.Record) error {
	if action == "" {
		return nil
	}
	actor = strings.TrimSpace(actor)
	if actor == "" {
		actor = systemActorID
	}
	_, err := s.resources.RecordAudit(requestAuditEvent(ctx.Request.Context(), adminresource.AuditEvent{
		Actor: actor, Action: action, Resource: "files", TargetID: record.ID,
		Result: "success", ReasonCode: "content-authorized",
	}))
	return err
}

func requestAuditEvent(ctx context.Context, event adminresource.AuditEvent) adminresource.AuditEvent {
	if correlation, ok := kernel.CorrelationFromContext(ctx); ok {
		event.RequestID = correlation.RequestID
		event.TraceID = correlation.TraceID
	}
	return event
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
	appSession, ok, _ := s.appSessionFromBearerContext(ctx)
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
	normalized := appUsername(username)
	digest := sha256.Sum256([]byte("platform-go:app-user:v1\x00" + normalized))
	return "app-user:v1:" + hex.EncodeToString(digest[:])
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
	if !s.refreshAdminResourceState(ctx, errorcode.CodeAdminAuthStateRefreshFailed) {
		return false
	}
	if token, ok := bearerToken(ctx.GetHeader("Authorization")); ok && strings.HasPrefix(token, apiTokenPrefix) {
		allowed, valid := s.authorizeAPIToken(token, permission)
		if !valid {
			writePlatformError(ctx, errorcode.CodeAuthUnauthorized)
			return false
		}
		if allowed {
			return true
		}
		writePlatformError(ctx, errorcode.CodeAdminForbidden)
		return false
	}
	principal, ok := s.currentPrincipal(ctx)
	if !ok {
		writePlatformError(ctx, errorcode.CodeAuthUnauthorized)
		return false
	}
	if s.can(principal, permission) {
		return true
	}
	writePlatformError(ctx, errorcode.CodeAdminForbidden)
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
	if !s.refreshAdminResourceState(ctx, errorcode.CodeAdminAuthStateRefreshFailed) {
		return rbac.Principal{}, false, false
	}
	schema, err := s.resources.Schema(resource)
	if err != nil {
		writeAdminResourceError(ctx, s.internalErrorSink, err)
		return rbac.Principal{}, false, false
	}
	permission := permissionForAction(schema.Permissions, action)
	if token, ok := bearerToken(ctx.GetHeader("Authorization")); ok && strings.HasPrefix(token, apiTokenPrefix) {
		allowed, valid := s.authorizeAPIToken(token, permission)
		if !valid {
			writePlatformError(ctx, errorcode.CodeAuthUnauthorized)
			return rbac.Principal{}, false, false
		}
		if allowed {
			return rbac.Principal{}, false, true
		}
		writePlatformError(ctx, errorcode.CodeAdminForbidden)
		return rbac.Principal{}, false, false
	}
	principal, ok := s.currentPrincipal(ctx)
	if !ok {
		writePlatformError(ctx, errorcode.CodeAuthUnauthorized)
		return rbac.Principal{}, false, false
	}
	if s.can(principal, permission) {
		return principal, true, true
	}
	writePlatformError(ctx, errorcode.CodeAdminForbidden)
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
	case "restore":
		return permissions.Restore
	case "purge":
		return permissions.Purge
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
	if s.resources.RepositoryBacked() {
		return s.resources.CurrentPrincipal(username)
	}
	return cachedJSONValue(ctx, s.cache, cacheKeyPrincipalPrefix+username, s.cacheTTL, func() rbac.Principal {
		return s.resources.CurrentPrincipal(username)
	})
}

func (s *Server) refreshAdminResourceState(ctx *gin.Context, code errorcode.Code) bool {
	if err := s.refreshResourceState(ctx); err != nil {
		writePlatformErrorWithCause(ctx, s.internalErrorSink, code, err)
		return false
	}
	return true
}

func (s *Server) refreshResourceState(ctx *gin.Context) error {
	changed, err := s.resources.RefreshContext(ctx.Request.Context())
	if err != nil {
		return err
	}
	if changed {
		s.invalidatePolicyAuthorizer()
		_ = s.cache.DeletePrefix(ctx.Request.Context(), cacheKeyPrincipalPrefix)
		_ = s.cache.DeletePrefix(ctx.Request.Context(), cacheKeyMenusPrefix)
	}
	return nil
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
	case authorizationInvalidationResource, "org-units", "role-groups", "roles", "permissions", "users":
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

func (s *Server) recordInternalError(ctx *gin.Context, code errorcode.Code, err error) {
	if err == nil {
		return
	}
	recordPlatformError(ctx, s.internalErrorSink, registeredErrorDefinition(code), err)
}

func internalErrorEventID(ctx *gin.Context) string {
	if ctx != nil && ctx.Request != nil {
		if correlation, ok := kernel.CorrelationFromContext(ctx.Request.Context()); ok {
			digest := sha256.Sum256([]byte("platform-go:request-correlation:v1\x00" + correlation.RequestID))
			return "request:v1:" + hex.EncodeToString(digest[:])
		}
	}
	var suffix [12]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "event-unavailable"
	}
	return "event-" + hex.EncodeToString(suffix[:])
}

func (s *Server) mutationAuditEvent(ctx *gin.Context, action string, resource string, reasonCode string) adminresource.AuditEvent {
	correlation := correlationFromGinContext(ctx)
	return adminresource.AuditEvent{
		Actor:      s.auditActorID(ctx),
		Action:     action,
		Resource:   resource,
		Result:     "success",
		EventID:    internalErrorEventID(ctx),
		ReasonCode: reasonCode,
		RequestID:  correlation.RequestID,
		TraceID:    correlation.TraceID,
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
		s.recordInternalError(ctx, errorcode.CodeInternal, err)
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
