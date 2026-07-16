package httpapi

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/GnosiST/platform-go/internal/platform/approute"
	"github.com/GnosiST/platform-go/internal/platform/capability"
	"github.com/GnosiST/platform-go/internal/platform/errorcode"
	"github.com/GnosiST/platform-go/internal/platform/session"
)

type AppRouteRegistration = approute.Registration

type AppRouteHandlerCoverageResult struct {
	DeclaredCount int
	CoveredCount  int
	CoveredRoutes []string
	MissingRoutes []string
}

type appRouteKey struct {
	method string
	path   string
}

func AppSessionFromContext(ctx *gin.Context) (session.Session, bool) {
	return approute.SessionFromContext(ctx)
}

func AppRouteHandlerCoverage(manifests []capability.Manifest, extra []AppRouteRegistration) (AppRouteHandlerCoverageResult, error) {
	contracts, err := capability.AppRouteContracts(manifests)
	if err != nil {
		return AppRouteHandlerCoverageResult{}, err
	}
	handlers := map[appRouteKey]struct{}{}
	for _, registration := range platformCoreAppRouteRegistrations() {
		key := appRouteRegistrationKey(registration.Method, registration.Path)
		if key.method != "" && key.path != "" {
			handlers[key] = struct{}{}
		}
	}
	for key := range appRouteHandlerMap(extra) {
		handlers[key] = struct{}{}
	}

	coverage := AppRouteHandlerCoverageResult{DeclaredCount: len(contracts)}
	for _, contract := range contracts {
		name := appRouteName(contract.Method, contract.Path)
		if _, ok := handlers[appRouteRegistrationKey(contract.Method, contract.Path)]; ok {
			coverage.CoveredRoutes = append(coverage.CoveredRoutes, name)
			continue
		}
		coverage.MissingRoutes = append(coverage.MissingRoutes, name)
	}
	sort.Strings(coverage.CoveredRoutes)
	sort.Strings(coverage.MissingRoutes)
	coverage.CoveredCount = len(coverage.CoveredRoutes)
	return coverage, nil
}

func (s *Server) defaultAppRouteHandlers(extra []AppRouteRegistration) map[appRouteKey]gin.HandlerFunc {
	handlers := appRouteHandlerMap(s.platformCoreAppRouteHandlers())
	for key, handler := range appRouteHandlerMap(extra) {
		handlers[key] = handler
	}
	return handlers
}

func (s *Server) platformCoreAppRouteHandlers() []AppRouteRegistration {
	return []AppRouteRegistration{
		{Method: http.MethodPost, Path: "/api/app/auth/login", Handler: s.appAuthLogin},
		{Method: http.MethodPost, Path: "/api/app/auth/logout", Handler: s.appAuthLogout},
		{Method: http.MethodGet, Path: "/api/app/session/current", Handler: s.appCurrentSession},
		{Method: http.MethodPost, Path: "/api/app/identity/phone-verifications", Handler: s.appPhoneCreateVerification},
		{Method: http.MethodPost, Path: "/api/app/identity/phone-bindings", Handler: s.appPhoneCreateBinding},
		{Method: http.MethodPost, Path: "/api/app/files", Handler: s.appFileUpload},
		{Method: http.MethodGet, Path: "/api/app/files/:id/content", Handler: s.appFileContent},
	}
}

func platformCoreAppRouteRegistrations() []AppRouteRegistration {
	noop := func(*gin.Context) {}
	return []AppRouteRegistration{
		{Method: http.MethodPost, Path: "/api/app/auth/login", Handler: noop},
		{Method: http.MethodPost, Path: "/api/app/auth/logout", Handler: noop},
		{Method: http.MethodGet, Path: "/api/app/session/current", Handler: noop},
		{Method: http.MethodPost, Path: "/api/app/identity/phone-verifications", Handler: noop},
		{Method: http.MethodPost, Path: "/api/app/identity/phone-bindings", Handler: noop},
		{Method: http.MethodPost, Path: "/api/app/files", Handler: noop},
		{Method: http.MethodGet, Path: "/api/app/files/:id/content", Handler: noop},
	}
}

func appRouteHandlerMap(registrations []AppRouteRegistration) map[appRouteKey]gin.HandlerFunc {
	handlers := map[appRouteKey]gin.HandlerFunc{}
	for _, registration := range registrations {
		if registration.Handler == nil {
			continue
		}
		key := appRouteRegistrationKey(registration.Method, registration.Path)
		if key.method == "" || key.path == "" {
			continue
		}
		handlers[key] = registration.Handler
	}
	return handlers
}

func (s *Server) registerManifestAppRoutes(api *gin.RouterGroup) {
	routes, err := capability.AppRouteContracts(s.capabilities)
	if err != nil {
		panic(fmt.Sprintf("invalid app route manifest: %v", err))
	}
	for _, route := range routes {
		route := route
		handler := s.appRoutes[appRouteRegistrationKey(route.Method, route.Path)]
		if handler == nil {
			handler = appRouteHandlerNotConfigured(route)
		}
		api.Handle(route.Method, appRouteRelativePath(route.Path), s.withAppRoutePolicy(route, handler))
	}
}

func (s *Server) withAppRoutePolicy(route capability.AppRouteContract, handler gin.HandlerFunc) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if route.Auth != capability.AppRouteAuthSession {
			handler(ctx)
			return
		}
		appSession, ok, err := s.appSessionFromBearerContext(ctx)
		if err != nil {
			if isAppLogoutRoute(route) {
				writePlatformErrorWithCause(ctx, s.internalErrorSink, errorcode.CodeAppAuthSessionRevokeFailed, err)
				return
			}
			writePlatformError(ctx, errorcode.CodeAuthUnauthorized)
			return
		}
		if !ok {
			writePlatformError(ctx, errorcode.CodeAuthUnauthorized)
			return
		}
		if !s.canApp(appSession, route.Permission) {
			writePlatformError(ctx, errorcode.CodeAppForbidden)
			return
		}
		ctx.Set(approute.SessionContextKey, appSession)
		handler(ctx)
	}
}

func isAppLogoutRoute(route capability.AppRouteContract) bool {
	return strings.EqualFold(strings.TrimSpace(route.Method), http.MethodPost) && strings.TrimSpace(route.Path) == "/api/app/auth/logout"
}

func (s *Server) canApp(appSession session.Session, permission string) bool {
	if strings.TrimSpace(permission) == "" {
		return true
	}
	authorizer := s.authorizer
	if authorizer == nil {
		var err error
		authorizer, err = s.resources.CasbinAuthorizer()
		if err != nil {
			return false
		}
	}
	return authorizer.Can(appUserID(appUsername(appSession.Username)), appTenant, permission, actionFromPermission(permission))
}

func appRouteHandlerNotConfigured(_ capability.AppRouteContract) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		writePlatformError(ctx, errorcode.CodeAppRouteHandlerNotConfigured)
	}
}

func appRouteRegistrationKey(method string, path string) appRouteKey {
	return appRouteKey{method: strings.ToUpper(strings.TrimSpace(method)), path: strings.TrimSpace(path)}
}

func appRouteName(method string, path string) string {
	return strings.ToUpper(strings.TrimSpace(method)) + " " + strings.TrimSpace(path)
}

func appRouteRelativePath(path string) string {
	path = strings.TrimSpace(path)
	relative := strings.TrimPrefix(path, "/api")
	if relative == "" {
		return "/"
	}
	return relative
}
