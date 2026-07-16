package approute

import (
	"github.com/gin-gonic/gin"

	"github.com/GnosiST/platform-go/internal/platform/session"
)

const SessionContextKey = "platform.appSession"

type Registration struct {
	Method  string
	Path    string
	Handler gin.HandlerFunc
}

func SessionFromContext(ctx *gin.Context) (session.Session, bool) {
	value, ok := ctx.Get(SessionContextKey)
	if !ok {
		return session.Session{}, false
	}
	appSession, ok := value.(session.Session)
	return appSession, ok
}
