package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"platform-go/internal/platform/capability"
)

type ServerOptions struct {
	Capabilities []capability.Manifest
}

type Server struct {
	router       *gin.Engine
	capabilities []capability.Manifest
}

func New(options ServerOptions) *Server {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	server := &Server{router: router, capabilities: options.Capabilities}
	server.routes()
	return server
}

func (s *Server) Router() *gin.Engine {
	return s.router
}

func (s *Server) Run(addr string) error {
	return s.router.Run(addr)
}

func (s *Server) routes() {
	api := s.router.Group("/api")
	api.GET("/health", s.health)
	api.GET("/capabilities", s.capabilitiesList)
}

func (s *Server) health(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, Response[gin.H]{Data: gin.H{"ok": true, "service": "platform-go"}})
}

func (s *Server) capabilitiesList(ctx *gin.Context) {
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
