// Package app exposes a small public composition surface for downstream App
// handlers. It deliberately uses net/http and capability contracts only.
package app

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/GnosiST/platform-go/pkg/platform/capability"
)

type identityContextKey struct{}

// Identity is the trusted authenticated request identity supplied by a
// downstream authentication middleware.
type Identity struct {
	SubjectID   string
	TenantID    string
	Permissions []string
}

// Authenticated reports whether this identity has an authenticated subject.
func (identity Identity) Authenticated() bool {
	return strings.TrimSpace(identity.SubjectID) != ""
}

// HasPermission reports whether the identity has the exact App permission.
func (identity Identity) HasPermission(permission string) bool {
	permission = strings.TrimSpace(permission)
	if permission == "" {
		return true
	}
	for _, granted := range identity.Permissions {
		if strings.TrimSpace(granted) == permission {
			return true
		}
	}
	return false
}

// WithIdentity attaches a trusted authenticated identity to a request context.
// Authentication middleware must derive this value from verified credentials,
// never directly from client-controlled request values.
func WithIdentity(ctx context.Context, identity Identity) context.Context {
	identity.Permissions = append([]string(nil), identity.Permissions...)
	return context.WithValue(ctx, identityContextKey{}, identity)
}

// IdentityFromContext returns the typed request identity supplied by middleware.
func IdentityFromContext(ctx context.Context) (Identity, bool) {
	identity, ok := ctx.Value(identityContextKey{}).(Identity)
	if !ok {
		return Identity{}, false
	}
	identity.Permissions = append([]string(nil), identity.Permissions...)
	return identity, true
}

// HandlerFunc is a downstream App route handler. The public router enforces
// the manifest's session and permission policy before invoking it.
type HandlerFunc func(context.Context, Identity, http.ResponseWriter, *http.Request)

// Registration binds one declared capability App route to its downstream
// handler. Method and Path must exactly match the capability manifest.
type Registration struct {
	Method  string
	Path    string
	Handler HandlerFunc
}

// Router mounts manifest-declared App routes using the standard library HTTP
// server. It has no platform storage, framework or authentication dependency.
type Router struct {
	handler http.Handler
}

// NewRouter registers manifests through the public capability registry and
// mounts exactly one handler for every declared App route.
func NewRouter(manifests []capability.Manifest, registrations []Registration) (*Router, error) {
	resolved, err := resolveManifests(manifests)
	if err != nil {
		return nil, err
	}
	contracts, err := capability.AppRouteContracts(resolved)
	if err != nil {
		return nil, err
	}

	declared := make(map[string]capability.AppRouteContract, len(contracts))
	for _, contract := range contracts {
		declared[routeKey(contract.Method, contract.Path)] = contract
	}
	registered := make(map[string]struct{}, len(registrations))
	patterns := make(map[string]string, len(registrations))
	mux := http.NewServeMux()
	for _, registration := range registrations {
		key := routeKey(registration.Method, registration.Path)
		contract, exists := declared[key]
		if !exists {
			return nil, fmt.Errorf("app handler %q is not declared by an enabled capability manifest", key)
		}
		if registration.Handler == nil {
			return nil, fmt.Errorf("app handler %q is required", key)
		}
		if _, duplicate := registered[key]; duplicate {
			return nil, fmt.Errorf("app handler %q is registered more than once", key)
		}
		pattern, err := serveMuxPattern(contract.Method, contract.Path)
		if err != nil {
			return nil, err
		}
		patternKey := comparablePattern(pattern)
		if existing, duplicate := patterns[patternKey]; duplicate {
			return nil, fmt.Errorf("app handlers %q and %q map to the same HTTP pattern %q", existing, key, pattern)
		}
		mux.Handle(pattern, withPolicy(contract, registration.Handler))
		registered[key] = struct{}{}
		patterns[patternKey] = key
	}
	for key := range declared {
		if _, exists := registered[key]; !exists {
			return nil, fmt.Errorf("declared app route %q has no handler registration", key)
		}
	}
	return &Router{handler: mux}, nil
}

// ServeHTTP lets a downstream composition root mount Router directly into its
// own HTTP server or test harness.
func (router *Router) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	router.handler.ServeHTTP(writer, request)
}

func resolveManifests(manifests []capability.Manifest) ([]capability.Manifest, error) {
	registry := capability.NewRegistry()
	enabled := make([]capability.ID, 0, len(manifests))
	for _, manifest := range manifests {
		if err := registry.Register(manifest); err != nil {
			return nil, err
		}
		enabled = append(enabled, manifest.ID)
	}
	return registry.ResolveEnabled(enabled)
}

func withPolicy(contract capability.AppRouteContract, handler HandlerFunc) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		identity, present := IdentityFromContext(request.Context())
		if contract.Auth == capability.AppRouteAuthSession && (!present || !identity.Authenticated()) {
			http.Error(writer, "authentication required", http.StatusUnauthorized)
			return
		}
		if contract.Permission != "" && !identity.HasPermission(contract.Permission) {
			http.Error(writer, "permission denied", http.StatusForbidden)
			return
		}
		handler(request.Context(), identity, writer, request)
	})
}

func routeKey(method string, path string) string {
	return strings.ToUpper(strings.TrimSpace(method)) + " " + strings.TrimSpace(path)
}

func serveMuxPattern(method string, path string) (string, error) {
	segments := strings.Split(strings.TrimSpace(path), "/")
	for index, segment := range segments {
		if !strings.HasPrefix(segment, ":") {
			continue
		}
		name := strings.TrimPrefix(segment, ":")
		if name == "" || strings.ContainsAny(name, "{}./") {
			return "", fmt.Errorf("app route %q has unsupported path parameter %q", path, segment)
		}
		segments[index] = "{" + name + "}"
	}
	return strings.ToUpper(strings.TrimSpace(method)) + " " + strings.Join(segments, "/"), nil
}

func comparablePattern(pattern string) string {
	segments := strings.Split(pattern, "/")
	for index, segment := range segments {
		if strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") {
			segments[index] = "{}"
		}
	}
	return strings.Join(segments, "/")
}
