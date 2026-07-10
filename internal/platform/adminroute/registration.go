package adminroute

import "github.com/gin-gonic/gin"

type Registration struct {
	Method     string
	Path       string
	Permission string
	Handler    gin.HandlerFunc
}
