package httpapi

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"platform-go/internal/platform/adminresource"
	"platform-go/internal/platform/capability"
)

type ServerOptions struct {
	Capabilities []capability.Manifest
	Resources    *adminresource.Store
}

type Server struct {
	router       *gin.Engine
	capabilities []capability.Manifest
	resources    *adminresource.Store
}

func New(options ServerOptions) *Server {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	resources := options.Resources
	if resources == nil {
		resources = adminresource.NewStore()
	}
	server := &Server{router: router, capabilities: options.Capabilities, resources: resources}
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
	adminResources := api.Group("/admin/resources")
	adminResources.GET("/:resource", s.adminResourceList)
	adminResources.POST("/:resource", s.adminResourceCreate)
	adminResources.PUT("/:resource/:id", s.adminResourceUpdate)
	adminResources.DELETE("/:resource/:id", s.adminResourceDelete)
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

type adminResourceListResponse struct {
	Resource string                 `json:"resource"`
	Items    []adminresource.Record `json:"items"`
}

type adminResourceRecordResponse struct {
	Resource string               `json:"resource"`
	Record   adminresource.Record `json:"record"`
}

func (s *Server) adminResourceList(ctx *gin.Context) {
	resource := ctx.Param("resource")
	items, err := s.resources.List(resource)
	if err != nil {
		writeAdminResourceError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, Response[adminResourceListResponse]{
		Data: adminResourceListResponse{Resource: resource, Items: items},
	})
}

func (s *Server) adminResourceCreate(ctx *gin.Context) {
	resource := ctx.Param("resource")
	var input adminresource.WriteInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		writeAdminResourceError(ctx, adminresource.ErrInvalidRecord)
		return
	}
	record, err := s.resources.Create(resource, input)
	if err != nil {
		writeAdminResourceError(ctx, err)
		return
	}
	ctx.JSON(http.StatusCreated, Response[adminResourceRecordResponse]{
		Data: adminResourceRecordResponse{Resource: resource, Record: record},
	})
}

func (s *Server) adminResourceUpdate(ctx *gin.Context) {
	resource := ctx.Param("resource")
	id := ctx.Param("id")
	var input adminresource.WriteInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		writeAdminResourceError(ctx, adminresource.ErrInvalidRecord)
		return
	}
	record, err := s.resources.Update(resource, id, input)
	if err != nil {
		writeAdminResourceError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, Response[adminResourceRecordResponse]{
		Data: adminResourceRecordResponse{Resource: resource, Record: record},
	})
}

func (s *Server) adminResourceDelete(ctx *gin.Context) {
	resource := ctx.Param("resource")
	if err := s.resources.Delete(resource, ctx.Param("id")); err != nil {
		writeAdminResourceError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, Response[gin.H]{Data: gin.H{"resource": resource, "deleted": true}})
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
	case errors.Is(err, adminresource.ErrInvalidRecord):
		status = http.StatusBadRequest
		code = "ADMIN_RESOURCE_INVALID_RECORD"
		message = "admin resource name is required"
	}
	ctx.JSON(status, Response[gin.H]{Error: &ErrorBody{Code: code, Message: message}})
}
