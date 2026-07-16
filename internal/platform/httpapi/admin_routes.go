package httpapi

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/GnosiST/platform-go/internal/platform/adminroute"
)

type AdminRouteRegistration = adminroute.Registration

func (s *Server) registerAdminRoutes(api *gin.RouterGroup, routes []AdminRouteRegistration) {
	for _, route := range routes {
		route := route
		if route.Handler == nil {
			continue
		}
		method := strings.ToUpper(strings.TrimSpace(route.Method))
		if method == "" {
			method = http.MethodPost
		}
		path := adminRouteRelativePath(route.Path)
		api.Handle(method, path, s.withAdminRoutePolicy(route))
	}
}

func (s *Server) withAdminRoutePolicy(route AdminRouteRegistration) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if strings.TrimSpace(route.Permission) != "" && !s.authorize(ctx, route.Permission) {
			return
		}
		route.Handler(ctx)
	}
}

func adminRouteRelativePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "/"
	}
	relative := strings.TrimPrefix(path, "/api")
	if relative == "" {
		return "/"
	}
	return relative
}
