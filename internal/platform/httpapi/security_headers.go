package httpapi

import (
	"bytes"
	"io"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"strings"

	"github.com/gin-gonic/gin"
)

const defaultHTTPMaxBodyBytes = int64(1 << 20)

type SecurityOptions struct {
	RequireHTTPS     bool
	PublicBaseURL    string
	TrustedProxies   []string
	MaxJSONBodyBytes int64
}

func securityHeaders(options SecurityOptions) gin.HandlerFunc {
	trusted := trustedProxyPrefixes(options.TrustedProxies)
	publicBaseURL := strings.TrimRight(strings.TrimSpace(options.PublicBaseURL), "/")
	return func(ctx *gin.Context) {
		ctx.Header("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'; sandbox allow-downloads")
		ctx.Header("X-Content-Type-Options", "nosniff")
		ctx.Header("X-Frame-Options", "DENY")
		ctx.Header("Referrer-Policy", "no-referrer")

		secure := requestUsesHTTPS(ctx.Request, trusted)
		if options.RequireHTTPS && !secure {
			ctx.Redirect(http.StatusPermanentRedirect, publicBaseURL+ctx.Request.URL.RequestURI())
			ctx.Abort()
			return
		}
		if secure {
			ctx.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		ctx.Next()
	}
}

func jsonRequestBodyLimit(maxBytes int64) gin.HandlerFunc {
	if maxBytes <= 0 {
		maxBytes = defaultHTTPMaxBodyBytes
	}
	return func(ctx *gin.Context) {
		if ctx.Request.Body == nil || !isJSONContentType(ctx.GetHeader("Content-Type")) {
			ctx.Next()
			return
		}
		if ctx.Request.ContentLength > maxBytes {
			writeRequestBodyTooLarge(ctx)
			return
		}
		body, err := io.ReadAll(io.LimitReader(ctx.Request.Body, maxBytes+1))
		_ = ctx.Request.Body.Close()
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusBadRequest, Response[any]{Error: &ErrorBody{Code: "REQUEST_BODY_INVALID", Message: "request body is invalid"}})
			return
		}
		if int64(len(body)) > maxBytes {
			writeRequestBodyTooLarge(ctx)
			return
		}
		ctx.Request.Body = io.NopCloser(bytes.NewReader(body))
		ctx.Request.ContentLength = int64(len(body))
		ctx.Next()
	}
}

func writeRequestBodyTooLarge(ctx *gin.Context) {
	ctx.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, Response[any]{Error: &ErrorBody{Code: "REQUEST_BODY_TOO_LARGE", Message: "request body exceeds configured limit"}})
}

func isJSONContentType(raw string) bool {
	mediaType, _, err := mime.ParseMediaType(raw)
	return err == nil && (mediaType == "application/json" || strings.HasSuffix(mediaType, "+json"))
}

func requestUsesHTTPS(request *http.Request, trusted []netip.Prefix) bool {
	if request.TLS != nil {
		return true
	}
	peer, ok := directPeerAddress(request.RemoteAddr)
	if !ok || !prefixesContain(trusted, peer) {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(request.Header.Get("X-Forwarded-Proto")), "https")
}

func directPeerAddress(remoteAddr string) (netip.Addr, bool) {
	host, _, err := net.SplitHostPort(strings.TrimSpace(remoteAddr))
	if err != nil {
		return netip.Addr{}, false
	}
	address, err := netip.ParseAddr(host)
	return address, err == nil
}

func trustedProxyPrefixes(values []string) []netip.Prefix {
	prefixes := make([]netip.Prefix, 0, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if prefix, err := netip.ParsePrefix(value); err == nil && prefix.Bits() > 0 {
			prefixes = append(prefixes, prefix.Masked())
			continue
		}
		if address, err := netip.ParseAddr(value); err == nil {
			prefixes = append(prefixes, netip.PrefixFrom(address, address.BitLen()))
		}
	}
	return prefixes
}

func prefixesContain(prefixes []netip.Prefix, address netip.Addr) bool {
	for _, prefix := range prefixes {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}

func configureTrustedProxies(router *gin.Engine, proxies []string) {
	if err := router.SetTrustedProxies(proxies); err != nil {
		_ = router.SetTrustedProxies(nil)
	}
}
