package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/GnosiST/platform-go/internal/platform/adminresource"
	"github.com/GnosiST/platform-go/internal/platform/authjwt"
)

const (
	requestLogResource = "request-logs"
	errorLogResource   = "error-logs"
)

type RequestLogEvent struct {
	Domain       string
	Actor        string
	Method       string
	Route        string
	StatusCode   int
	Latency      time.Duration
	RequestID    string
	TraceID      string
	ClientIPHash string
	CreatedAt    time.Time
}

type RequestLogSink interface {
	RecordRequest(context.Context, RequestLogEvent)
}

type resourcePlatformLogSink struct {
	resources *adminresource.Store
	now       func() time.Time
}

func (sink resourcePlatformLogSink) Record(ctx context.Context, event InternalErrorEvent) {
	if sink.resources == nil {
		return
	}
	createdAt := sink.nowUTC()
	eventID := strings.TrimSpace(event.EventID)
	if eventID == "" {
		eventID = platformLogCode("error")
	}
	values := map[string]string{
		"level":          "error",
		"message":        string(event.Code),
		"errorCode":      string(event.Code),
		"owner":          strings.TrimSpace(event.Owner),
		"category":       string(event.Category),
		"retryPolicy":    string(event.RetryPolicy),
		"redactionClass": string(event.RedactionClass),
		"eventId":        eventID,
		"createdAt":      createdAt.Format(time.RFC3339),
	}
	if strings.TrimSpace(event.RequestID) != "" {
		values["requestId"] = strings.TrimSpace(event.RequestID)
	}
	if strings.TrimSpace(event.TraceID) != "" {
		values["traceId"] = strings.TrimSpace(event.TraceID)
	}
	input := adminresource.WriteInput{
		Code:        eventID,
		Name:        "Platform Error " + string(event.Code),
		Status:      "open",
		Description: "Platform internal error recorded with public-safe diagnostics.",
		Values:      values,
	}
	_, err := sink.resources.CreateInternal(errorLogResource, input)
	if err != nil && errors.Is(err, adminresource.ErrInvalidRecord) {
		input.Values = nil
		_, err = sink.resources.CreateInternal(errorLogResource, input)
	}
	if err != nil && !errors.Is(err, adminresource.ErrUnknownResource) {
		return
	}
}

func (sink resourcePlatformLogSink) RecordRequest(ctx context.Context, event RequestLogEvent) {
	if sink.resources == nil {
		return
	}
	createdAt := event.CreatedAt.UTC()
	if createdAt.IsZero() {
		createdAt = sink.nowUTC()
	}
	statusCode := event.StatusCode
	if statusCode <= 0 {
		statusCode = http.StatusOK
	}
	route := strings.TrimSpace(event.Route)
	if route == "" {
		route = "unmatched"
	}
	values := map[string]string{
		"domain":       valueWithDefault(event.Domain, "public"),
		"method":       valueWithDefault(event.Method, "UNKNOWN"),
		"route":        route,
		"statusCode":   strconv.Itoa(statusCode),
		"latencyMs":    strconv.FormatInt(maxInt64(0, event.Latency.Milliseconds()), 10),
		"actor":        valueWithDefault(event.Actor, "anonymous"),
		"requestId":    strings.TrimSpace(event.RequestID),
		"traceId":      strings.TrimSpace(event.TraceID),
		"clientIpHash": strings.TrimSpace(event.ClientIPHash),
		"createdAt":    createdAt.Format(time.RFC3339),
	}
	code := platformLogCode("request")
	_, err := sink.resources.CreateInternal(requestLogResource, adminresource.WriteInput{
		Code:        code,
		Name:        values["method"] + " " + route + " -> " + values["statusCode"],
		Status:      requestLogStatus(statusCode),
		Description: "HTTP request log entry.",
		Values:      values,
	})
	if err != nil && !errors.Is(err, adminresource.ErrUnknownResource) {
		return
	}
}

func (sink resourcePlatformLogSink) nowUTC() time.Time {
	if sink.now != nil {
		return sink.now().UTC()
	}
	return time.Now().UTC()
}

func requestLoggingMiddleware(server *Server) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		startedAt := server.now().UTC()
		ctx.Next()
		if server == nil || server.requestLogSink == nil || ctx.Request == nil {
			return
		}
		path := ""
		if ctx.Request.URL != nil {
			path = ctx.Request.URL.Path
		}
		if !strings.HasPrefix(path, "/api") {
			return
		}
		endedAt := server.now().UTC()
		if endedAt.Before(startedAt) {
			endedAt = startedAt
		}
		correlation := correlationFromGinContext(ctx)
		domain, actor := server.requestLogPrincipal(ctx)
		server.requestLogSink.RecordRequest(ctx.Request.Context(), RequestLogEvent{
			Domain:       domain,
			Actor:        actor,
			Method:       ctx.Request.Method,
			Route:        requestLogRoute(ctx, path),
			StatusCode:   ctx.Writer.Status(),
			Latency:      endedAt.Sub(startedAt),
			RequestID:    correlation.RequestID,
			TraceID:      correlation.TraceID,
			ClientIPHash: requestLogClientIPHash(ctx),
			CreatedAt:    startedAt,
		})
	}
}

func (s *Server) requestLogPrincipal(ctx *gin.Context) (string, string) {
	domain := requestLogDomain(ctx)
	token, hasBearer := bearerToken(ctx.GetHeader("Authorization"))
	if hasBearer && strings.HasPrefix(token, apiTokenPrefix) {
		if record, valid := s.resolveAPITokenRecord(token); valid && strings.TrimSpace(record.Code) != "" {
			return domain, "api-token:" + record.Code
		}
		return domain, "api-token"
	}
	if hasBearer {
		if claims, err := s.tokens.Parse(token); err == nil {
			switch claims.TokenType {
			case authjwt.TokenTypeAdmin:
				if authSession, ok, _ := s.authSessionFromBearerContext(ctx); ok {
					return "admin", valueWithDefault(authSession.Username, claims.Username)
				}
				return "admin", valueWithDefault(claims.Username, "admin-session")
			case authjwt.TokenTypeApp:
				if appSession, ok, _ := s.appSessionFromBearerContext(ctx); ok {
					return "app", valueWithDefault(appSession.Username, claims.Username)
				}
				return "app", valueWithDefault(claims.Username, "app-session")
			}
		}
		return domain, "bearer"
	}
	if s.allowInsecureHeaderAuth {
		if user := strings.TrimSpace(ctx.GetHeader("X-Platform-User")); user != "" {
			return "admin", user
		}
	}
	return domain, "anonymous"
}

func requestLogDomain(ctx *gin.Context) string {
	if ctx == nil || ctx.Request == nil || ctx.Request.URL == nil {
		return "public"
	}
	path := ctx.Request.URL.Path
	switch {
	case strings.HasPrefix(path, "/api/admin"):
		return "admin"
	case strings.HasPrefix(path, "/api/app"):
		return "app"
	case strings.HasPrefix(path, "/api/auth"):
		return "auth"
	case strings.HasPrefix(path, "/api/platform"), strings.HasPrefix(path, "/api/capabilities"), strings.HasPrefix(path, "/api/openapi"), strings.HasPrefix(path, "/api/health"):
		return "platform"
	default:
		return "public"
	}
}

func requestLogRoute(ctx *gin.Context, fallbackPath string) string {
	route := strings.TrimSpace(ctx.FullPath())
	if route != "" {
		return route
	}
	fallbackPath = strings.TrimSpace(fallbackPath)
	if fallbackPath == "" {
		return "unmatched"
	}
	return fallbackPath
}

func requestLogClientIPHash(ctx *gin.Context) string {
	ip := strings.TrimSpace(rateLimitClientIP(ctx))
	if ip == "" {
		return ""
	}
	digest := sha256.Sum256([]byte("platform-go:request-log-client-ip:v1\x00" + ip))
	return "v1:sha256:client-ip:" + hex.EncodeToString(digest[:])
}

func requestLogStatus(statusCode int) string {
	switch {
	case statusCode >= 500:
		return "error"
	case statusCode >= 400:
		return "rejected"
	default:
		return "success"
	}
}

func platformLogCode(prefix string) string {
	var raw [12]byte
	if _, err := rand.Read(raw[:]); err == nil {
		return strings.TrimSpace(prefix) + "-" + hex.EncodeToString(raw[:])
	}
	return strings.TrimSpace(prefix) + "-" + strconv.FormatInt(time.Now().UnixNano(), 10)
}

func valueWithDefault(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}
	return strings.TrimSpace(fallback)
}

func maxInt64(left int64, right int64) int64 {
	if left > right {
		return left
	}
	return right
}
