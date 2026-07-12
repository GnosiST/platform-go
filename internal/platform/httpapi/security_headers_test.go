package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSecurityHeadersRequireTrustedHTTPSContextForHSTS(t *testing.T) {
	router := gin.New()
	router.Use(securityHeaders(SecurityOptions{TrustedProxies: []string{"10.20.0.0/16"}}))
	router.GET("/content", func(ctx *gin.Context) {
		ctx.Data(http.StatusOK, "text/html", []byte("<script>alert(1)</script>"))
	})

	trusted := httptest.NewRequest(http.MethodGet, "/content", nil)
	trusted.RemoteAddr = "10.20.1.4:443"
	trusted.Header.Set("X-Forwarded-Proto", "https")
	trustedRecorder := httptest.NewRecorder()
	router.ServeHTTP(trustedRecorder, trusted)

	if got := trustedRecorder.Header().Get("Strict-Transport-Security"); got == "" {
		t.Fatal("trusted HTTPS response is missing HSTS")
	}
	for name, want := range map[string]string{
		"Content-Security-Policy": "sandbox",
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":         "DENY",
		"Referrer-Policy":         "no-referrer",
	} {
		if got := trustedRecorder.Header().Get(name); !strings.Contains(got, want) {
			t.Fatalf("%s = %q, want containing %q", name, got, want)
		}
	}

	untrusted := httptest.NewRequest(http.MethodGet, "/content", nil)
	untrusted.RemoteAddr = "198.51.100.7:443"
	untrusted.Header.Set("X-Forwarded-Proto", "https")
	untrustedRecorder := httptest.NewRecorder()
	router.ServeHTTP(untrustedRecorder, untrusted)
	if got := untrustedRecorder.Header().Get("Strict-Transport-Security"); got != "" {
		t.Fatalf("untrusted forwarded response HSTS = %q, want empty", got)
	}
}

func TestJSONRequestBodyLimitRejectsOversizeBeforeHandler(t *testing.T) {
	called := false
	router := gin.New()
	router.Use(jsonRequestBodyLimit(16))
	router.POST("/json", func(ctx *gin.Context) {
		called = true
		ctx.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodPost, "/json", strings.NewReader(`{"message":"body is too large"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d body = %s, want 413", recorder.Code, recorder.Body.String())
	}
	if called {
		t.Fatal("oversized request reached handler")
	}
	if !strings.Contains(recorder.Body.String(), "REQUEST_BODY_TOO_LARGE") {
		t.Fatalf("body = %s, want stable error code", recorder.Body.String())
	}
}

func TestJSONRequestBodyLimitRejectsChunkedOversizeBeforeHandler(t *testing.T) {
	called := false
	router := gin.New()
	router.Use(jsonRequestBodyLimit(16))
	router.POST("/json", func(ctx *gin.Context) {
		called = true
		ctx.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodPost, "/json", strings.NewReader(`{"message":"body is too large"}`))
	request.ContentLength = -1
	request.Header.Set("Content-Type", "application/problem+json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusRequestEntityTooLarge || called {
		t.Fatalf("status = %d called = %t body = %s, want 413 before handler", recorder.Code, called, recorder.Body.String())
	}
}

func TestForwardedProtoFromUntrustedPeerCannotClaimHTTPS(t *testing.T) {
	router := gin.New()
	router.Use(securityHeaders(SecurityOptions{
		RequireHTTPS:   true,
		PublicBaseURL:  "https://platform.example.test",
		TrustedProxies: []string{"10.20.0.0/16"},
	}))
	router.GET("/api/health", func(ctx *gin.Context) { ctx.Status(http.StatusNoContent) })

	request := httptest.NewRequest(http.MethodGet, "/api/health?probe=1", nil)
	request.RemoteAddr = "198.51.100.7:443"
	request.Header.Set("X-Forwarded-Proto", "https")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusPermanentRedirect {
		t.Fatalf("status = %d, want 308", recorder.Code)
	}
	if got := recorder.Header().Get("Location"); got != "https://platform.example.test/api/health?probe=1" {
		t.Fatalf("Location = %q", got)
	}
	if got := recorder.Header().Get("Strict-Transport-Security"); got != "" {
		t.Fatalf("HSTS = %q, want empty for untrusted HTTP context", got)
	}
}

func TestProductionEdgeRedirectsUntrustedHTTPAndEmitsHSTSAfterTrustedHTTPS(t *testing.T) {
	server := New(ServerOptions{Security: SecurityOptions{
		RequireHTTPS:     true,
		PublicBaseURL:    "https://platform.example.test",
		TrustedProxies:   []string{"10.20.0.0/16"},
		MaxJSONBodyBytes: 1 << 20,
	}})

	httpRequest := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	httpRequest.RemoteAddr = "198.51.100.7:80"
	httpRecorder := httptest.NewRecorder()
	server.Router().ServeHTTP(httpRecorder, httpRequest)
	if httpRecorder.Code != http.StatusPermanentRedirect {
		t.Fatalf("untrusted HTTP status = %d, want 308", httpRecorder.Code)
	}

	httpsRequest := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	httpsRequest.RemoteAddr = "10.20.1.4:443"
	httpsRequest.Header.Set("X-Forwarded-Proto", "https")
	httpsRecorder := httptest.NewRecorder()
	server.Router().ServeHTTP(httpsRecorder, httpsRequest)
	if httpsRecorder.Code != http.StatusOK {
		t.Fatalf("trusted HTTPS status = %d body = %s", httpsRecorder.Code, httpsRecorder.Body.String())
	}
	if got := httpsRecorder.Header().Get("Strict-Transport-Security"); got == "" {
		t.Fatal("trusted HTTPS response is missing HSTS")
	}
}
