# Platform Go Kernel Core Skeleton Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first runnable `platform-go` framework skeleton with kernel types, capability lifecycle registration, default core capability manifests, HTTP introspection endpoints, and a compilable admin shell.

**Architecture:** This plan implements the first slice of the approved capability architecture. It creates a monolithic deployable skeleton with engineering-level capability packages, typed internal interfaces, lifecycle validation, and a thin HTTP adapter. Business features, optional WeChat/file/branding implementations, and `zshenmez-business` parity migration are intentionally deferred to later plans.

**Tech Stack:** Go 1.26, Gin, TypeScript, React, Vite, Ant Design.

---

## Scope Check

The approved architecture covers multiple subsystems. This plan only implements the first independently testable subsystem:

- Platform kernel primitives.
- Capability manifest and registry.
- Default core governance manifests as declarations only.
- HTTP health and capability introspection.
- Minimal admin frontend shell and capability list.

This plan does not implement full tenant/user/RBAC persistence, WeChat login, file storage, code generation, demo seed, or `zshenmez-business`.

## File Structure

Create:

- `go.mod` - root Go module and dependencies.
- `cmd/platform-api/main.go` - runnable API command.
- `internal/platform/kernel/context.go` - actor, tenant scope, permission intent and execution context.
- `internal/platform/kernel/errors.go` - typed platform errors.
- `internal/platform/kernel/uow.go` - Unit of Work interface and no-op implementation for skeleton composition.
- `internal/platform/kernel/context_test.go` - context and permission intent tests.
- `internal/platform/kernel/uow_test.go` - Unit of Work tests.
- `internal/platform/capability/manifest.go` - capability metadata, hooks and lifecycle phases.
- `internal/platform/capability/registry.go` - registration, dependency resolution and lifecycle execution.
- `internal/platform/capability/registry_test.go` - dependency graph and lifecycle tests.
- `internal/platform/config/config.go` - environment config for HTTP address and enabled capabilities.
- `internal/platform/config/config_test.go` - config parsing tests.
- `internal/platform/core/capabilities.go` - first-party core governance capability manifests.
- `internal/platform/core/capabilities_test.go` - default core dependency validation tests.
- `internal/platform/httpapi/response.go` - shared HTTP response shape.
- `internal/platform/httpapi/server.go` - Gin server with health and capability endpoints.
- `internal/platform/httpapi/server_test.go` - HTTP endpoint tests.
- `admin/package.json` - admin frontend scripts and dependencies.
- `admin/tsconfig.json` - TypeScript config.
- `admin/vite.config.ts` - Vite config.
- `admin/index.html` - Vite HTML entrypoint.
- `admin/src/main.tsx` - React entrypoint.
- `admin/src/App.tsx` - admin app composition.
- `admin/src/platform/api/client.ts` - typed platform API client.
- `admin/src/platform/shell/AdminShell.tsx` - minimal admin shell.
- `admin/src/platform/resources/registry.ts` - frontend resource definition types and starter registry.
- `admin/src/styles.css` - minimal shell styling.
- `README.md` - start and verification commands for this new repo.

## Task 1: Kernel Execution Context And Errors

**Files:**
- Create: `go.mod`
- Create: `internal/platform/kernel/context_test.go`
- Create: `internal/platform/kernel/context.go`
- Create: `internal/platform/kernel/errors.go`

- [ ] **Step 1: Write failing context tests**

Create `go.mod`:

```go
module platform-go

go 1.26
```

Create `internal/platform/kernel/context_test.go`:

```go
package kernel

import (
	"context"
	"testing"
)

func TestExecutionContextRequiresActorTenantAndPermissionForPermissionedCall(t *testing.T) {
	exec := ExecutionContext{
		Context: context.Background(),
		Actor: Actor{
			ID:       1001,
			Username: "admin",
			Kind:     ActorKindUser,
		},
		TenantScope: TenantScope{TenantID: 1, PlatformWide: true},
		PermissionIntent: PermissionIntent{
			Code:   "admin:tenant:read",
			Action: "read",
		},
	}

	if err := exec.ValidatePermissioned(); err != nil {
		t.Fatalf("ValidatePermissioned() error = %v", err)
	}
}

func TestExecutionContextRejectsMissingActor(t *testing.T) {
	exec := ExecutionContext{
		Context:          context.Background(),
		TenantScope:      TenantScope{TenantID: 1},
		PermissionIntent: PermissionIntent{Code: "admin:user:read", Action: "read"},
	}

	err := exec.ValidatePermissioned()
	if err == nil {
		t.Fatalf("ValidatePermissioned() error = nil, want missing actor")
	}
	if !IsCode(err, ErrCodeInvalidExecutionContext) {
		t.Fatalf("ValidatePermissioned() error = %v, want %s", err, ErrCodeInvalidExecutionContext)
	}
}

func TestExecutionContextRejectsMissingTenantScope(t *testing.T) {
	exec := ExecutionContext{
		Context: context.Background(),
		Actor:   Actor{ID: 1001, Username: "admin", Kind: ActorKindUser},
		PermissionIntent: PermissionIntent{
			Code:   "admin:user:read",
			Action: "read",
		},
	}

	err := exec.ValidatePermissioned()
	if err == nil {
		t.Fatalf("ValidatePermissioned() error = nil, want missing tenant")
	}
	if !IsCode(err, ErrCodeInvalidExecutionContext) {
		t.Fatalf("ValidatePermissioned() error = %v, want %s", err, ErrCodeInvalidExecutionContext)
	}
}
```

- [ ] **Step 2: Run the failing tests**

Run:

```bash
rtk go test ./internal/platform/kernel
```

Expected: FAIL because `ExecutionContext`, `Actor`, `TenantScope`, `PermissionIntent`, `IsCode`, and `ErrCodeInvalidExecutionContext` are undefined.

- [ ] **Step 3: Implement context and error types**

Create `internal/platform/kernel/errors.go`:

```go
package kernel

import "fmt"

const (
	ErrCodeInvalidExecutionContext = "INVALID_EXECUTION_CONTEXT"
	ErrCodeValidation              = "VALIDATION_ERROR"
	ErrCodeNotFound                = "NOT_FOUND"
	ErrCodeConflict                = "CONFLICT"
	ErrCodeForbidden               = "FORBIDDEN"
	ErrCodeInternal                = "INTERNAL_ERROR"
)

type Error struct {
	Code    string
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func NewError(code string, message string) *Error {
	return &Error{Code: code, Message: message}
}

func WrapError(code string, message string, cause error) *Error {
	return &Error{Code: code, Message: message, Cause: cause}
}

func IsCode(err error, code string) bool {
	platformErr, ok := err.(*Error)
	return ok && platformErr.Code == code
}
```

Create `internal/platform/kernel/context.go`:

```go
package kernel

import "context"

type ActorKind string

const (
	ActorKindUser   ActorKind = "user"
	ActorKindSystem ActorKind = "system"
)

type Actor struct {
	ID       int64
	Username string
	Kind     ActorKind
}

func (a Actor) Empty() bool {
	return a.ID == 0 && a.Username == "" && a.Kind == ""
}

type TenantScope struct {
	TenantID     int64
	PlatformWide bool
}

func (s TenantScope) Empty() bool {
	return s.TenantID == 0 && !s.PlatformWide
}

type PermissionIntent struct {
	Code   string
	Action string
}

func (p PermissionIntent) Empty() bool {
	return p.Code == "" || p.Action == ""
}

type ExecutionContext struct {
	Context          context.Context
	Actor            Actor
	TenantScope      TenantScope
	PermissionIntent PermissionIntent
	UoW              UnitOfWork
}

func (e ExecutionContext) BaseContext() context.Context {
	if e.Context == nil {
		return context.Background()
	}
	return e.Context
}

func (e ExecutionContext) ValidatePermissioned() error {
	if e.Actor.Empty() {
		return NewError(ErrCodeInvalidExecutionContext, "actor is required")
	}
	if e.TenantScope.Empty() {
		return NewError(ErrCodeInvalidExecutionContext, "tenant scope is required")
	}
	if e.PermissionIntent.Empty() {
		return NewError(ErrCodeInvalidExecutionContext, "permission intent is required")
	}
	return nil
}
```

- [ ] **Step 4: Run tests**

Run:

```bash
rtk go test ./internal/platform/kernel
```

Expected: FAIL because `UnitOfWork` is undefined. That failure is acceptable and will be resolved in Task 2.

- [ ] **Step 5: Commit**

Do not commit yet. Commit after Task 2 when the kernel package compiles.

## Task 2: Unit Of Work Skeleton

**Files:**
- Create: `internal/platform/kernel/uow_test.go`
- Create: `internal/platform/kernel/uow.go`
- Modify: `internal/platform/kernel/context.go`

- [ ] **Step 1: Write failing Unit of Work tests**

Create `internal/platform/kernel/uow_test.go`:

```go
package kernel

import (
	"context"
	"errors"
	"testing"
)

func TestNoopUnitOfWorkRunsCallback(t *testing.T) {
	uow := NoopUnitOfWork{}
	called := false

	err := uow.Do(context.Background(), func(ctx context.Context) error {
		called = true
		return nil
	})

	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	if !called {
		t.Fatalf("Do() did not call callback")
	}
}

func TestNoopUnitOfWorkReturnsCallbackError(t *testing.T) {
	uow := NoopUnitOfWork{}
	want := errors.New("boom")

	err := uow.Do(context.Background(), func(ctx context.Context) error {
		return want
	})

	if !errors.Is(err, want) {
		t.Fatalf("Do() error = %v, want %v", err, want)
	}
}
```

- [ ] **Step 2: Run the failing tests**

Run:

```bash
rtk go test ./internal/platform/kernel
```

Expected: FAIL because `NoopUnitOfWork` and `UnitOfWork` are undefined.

- [ ] **Step 3: Implement Unit of Work**

Create `internal/platform/kernel/uow.go`:

```go
package kernel

import "context"

type UnitOfWork interface {
	Do(ctx context.Context, fn func(context.Context) error) error
}

type NoopUnitOfWork struct{}

func (NoopUnitOfWork) Do(ctx context.Context, fn func(context.Context) error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	return fn(ctx)
}
```

- [ ] **Step 4: Run tests**

Run:

```bash
rtk go test ./internal/platform/kernel
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
rtk git add go.mod internal/platform/kernel
rtk git commit -m "feat: add platform kernel primitives"
```

## Task 3: Capability Manifest And Registry

**Files:**
- Create: `internal/platform/capability/manifest.go`
- Create: `internal/platform/capability/registry.go`
- Create: `internal/platform/capability/registry_test.go`

- [ ] **Step 1: Write failing registry tests**

Create `internal/platform/capability/registry_test.go`:

```go
package capability

import (
	"context"
	"reflect"
	"testing"
)

func TestRegistryResolvesDependenciesInOrder(t *testing.T) {
	registry := NewRegistry()
	mustRegister(t, registry, Manifest{ID: "audit"})
	mustRegister(t, registry, Manifest{ID: "file-storage", Dependencies: []ID{"audit"}})

	ordered, err := registry.ResolveEnabled([]ID{"file-storage", "audit"})
	if err != nil {
		t.Fatalf("ResolveEnabled() error = %v", err)
	}
	got := []ID{ordered[0].ID, ordered[1].ID}
	want := []ID{"audit", "file-storage"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ResolveEnabled() = %#v, want %#v", got, want)
	}
}

func TestRegistryFailsWhenDependencyMissing(t *testing.T) {
	registry := NewRegistry()
	mustRegister(t, registry, Manifest{ID: "wechat-login", Dependencies: []ID{"identity"}})

	_, err := registry.ResolveEnabled([]ID{"wechat-login"})
	if err == nil {
		t.Fatalf("ResolveEnabled() error = nil, want missing dependency")
	}
}

func TestRegistryFailsOnDependencyCycle(t *testing.T) {
	registry := NewRegistry()
	mustRegister(t, registry, Manifest{ID: "a", Dependencies: []ID{"b"}})
	mustRegister(t, registry, Manifest{ID: "b", Dependencies: []ID{"a"}})

	_, err := registry.ResolveEnabled([]ID{"a", "b"})
	if err == nil {
		t.Fatalf("ResolveEnabled() error = nil, want cycle")
	}
}

func TestRunLifecycleCallsHooksInDependencyOrder(t *testing.T) {
	registry := NewRegistry()
	var calls []string
	mustRegister(t, registry, Manifest{
		ID: "audit",
		Hooks: Hooks{
			Configure: func(context.Context, Runtime) error {
				calls = append(calls, "audit.configure")
				return nil
			},
			Start: func(context.Context, Runtime) error {
				calls = append(calls, "audit.start")
				return nil
			},
		},
	})
	mustRegister(t, registry, Manifest{
		ID:           "files",
		Dependencies: []ID{"audit"},
		Hooks: Hooks{
			Configure: func(context.Context, Runtime) error {
				calls = append(calls, "files.configure")
				return nil
			},
			Start: func(context.Context, Runtime) error {
				calls = append(calls, "files.start")
				return nil
			},
		},
	})

	err := registry.RunLifecycle(context.Background(), []ID{"files", "audit"}, Runtime{})
	if err != nil {
		t.Fatalf("RunLifecycle() error = %v", err)
	}
	want := []string{"audit.configure", "files.configure", "audit.start", "files.start"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
}

func mustRegister(t *testing.T, registry *Registry, manifest Manifest) {
	t.Helper()
	if err := registry.Register(manifest); err != nil {
		t.Fatalf("Register(%q) error = %v", manifest.ID, err)
	}
}
```

- [ ] **Step 2: Run the failing tests**

Run:

```bash
rtk go test ./internal/platform/capability
```

Expected: FAIL because the capability package is not implemented.

- [ ] **Step 3: Implement manifest types**

Create `internal/platform/capability/manifest.go`:

```go
package capability

import "context"

type ID string

type Manifest struct {
	ID           ID
	Name         string
	Version      string
	Dependencies []ID
	Hooks        Hooks
}

type Hooks struct {
	Configure        Hook
	Migrate          Hook
	Seed             Hook
	RegisterServices Hook
	RegisterRoutes   Hook
	RegisterAdmin    Hook
	Start            Hook
}

type Hook func(context.Context, Runtime) error

type Runtime struct{}
```

- [ ] **Step 4: Implement registry**

Create `internal/platform/capability/registry.go`:

```go
package capability

import (
	"context"
	"fmt"
)

type Registry struct {
	manifests map[ID]Manifest
}

func NewRegistry() *Registry {
	return &Registry{manifests: map[ID]Manifest{}}
}

func (r *Registry) Register(manifest Manifest) error {
	if manifest.ID == "" {
		return fmt.Errorf("capability id is required")
	}
	if _, exists := r.manifests[manifest.ID]; exists {
		return fmt.Errorf("capability %q already registered", manifest.ID)
	}
	r.manifests[manifest.ID] = manifest
	return nil
}

func (r *Registry) ResolveEnabled(enabled []ID) ([]Manifest, error) {
	enabledSet := map[ID]bool{}
	for _, id := range enabled {
		enabledSet[id] = true
	}

	var ordered []Manifest
	visiting := map[ID]bool{}
	visited := map[ID]bool{}

	var visit func(ID) error
	visit = func(id ID) error {
		if visited[id] {
			return nil
		}
		if visiting[id] {
			return fmt.Errorf("capability dependency cycle at %q", id)
		}
		manifest, ok := r.manifests[id]
		if !ok {
			return fmt.Errorf("capability %q is not registered", id)
		}
		visiting[id] = true
		for _, dependency := range manifest.Dependencies {
			if !enabledSet[dependency] {
				return fmt.Errorf("capability %q requires disabled dependency %q", id, dependency)
			}
			if err := visit(dependency); err != nil {
				return err
			}
		}
		visiting[id] = false
		visited[id] = true
		ordered = append(ordered, manifest)
		return nil
	}

	for _, id := range enabled {
		if err := visit(id); err != nil {
			return nil, err
		}
	}
	return ordered, nil
}

func (r *Registry) RunLifecycle(ctx context.Context, enabled []ID, runtime Runtime) error {
	ordered, err := r.ResolveEnabled(enabled)
	if err != nil {
		return err
	}
	phases := []func(Hooks) Hook{
		func(h Hooks) Hook { return h.Configure },
		func(h Hooks) Hook { return h.Migrate },
		func(h Hooks) Hook { return h.Seed },
		func(h Hooks) Hook { return h.RegisterServices },
		func(h Hooks) Hook { return h.RegisterRoutes },
		func(h Hooks) Hook { return h.RegisterAdmin },
		func(h Hooks) Hook { return h.Start },
	}
	for _, phase := range phases {
		for _, manifest := range ordered {
			hook := phase(manifest.Hooks)
			if hook == nil {
				continue
			}
			if err := hook(ctx, runtime); err != nil {
				return fmt.Errorf("capability %q lifecycle hook failed: %w", manifest.ID, err)
			}
		}
	}
	return nil
}
```

- [ ] **Step 5: Run tests**

Run:

```bash
rtk go test ./internal/platform/capability
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
rtk git add internal/platform/capability
rtk git commit -m "feat: add capability registry"
```

## Task 4: Config Parsing

**Files:**
- Create: `internal/platform/config/config.go`
- Create: `internal/platform/config/config_test.go`

- [ ] **Step 1: Write failing config tests**

Create `internal/platform/config/config_test.go`:

```go
package config

import (
	"reflect"
	"testing"
)

func TestLoadUsesDefaults(t *testing.T) {
	t.Setenv("PLATFORM_HTTP_ADDR", "")
	t.Setenv("PLATFORM_CAPABILITIES", "")

	cfg := Load()

	if cfg.HTTPAddr != "127.0.0.1:9200" {
		t.Fatalf("HTTPAddr = %q", cfg.HTTPAddr)
	}
	if len(cfg.Capabilities) == 0 {
		t.Fatalf("Capabilities is empty")
	}
}

func TestLoadParsesCapabilities(t *testing.T) {
	t.Setenv("PLATFORM_CAPABILITIES", "tenant, identity, audit")

	cfg := Load()
	want := []string{"tenant", "identity", "audit"}
	if !reflect.DeepEqual(cfg.Capabilities, want) {
		t.Fatalf("Capabilities = %#v, want %#v", cfg.Capabilities, want)
	}
}
```

- [ ] **Step 2: Run the failing tests**

Run:

```bash
rtk go test ./internal/platform/config
```

Expected: FAIL because config package is not implemented.

- [ ] **Step 3: Implement config**

Create `internal/platform/config/config.go`:

```go
package config

import (
	"os"
	"strings"
)

type Config struct {
	HTTPAddr     string
	Capabilities []string
}

var defaultCapabilities = []string{
	"tenant",
	"identity",
	"session",
	"rbac",
	"menu",
	"api-resource",
	"audit",
	"dictionary",
	"parameter",
	"admin-shell",
	"system-admin",
}

func Load() Config {
	return Config{
		HTTPAddr:     env("PLATFORM_HTTP_ADDR", "127.0.0.1:9200"),
		Capabilities: csvEnv("PLATFORM_CAPABILITIES", defaultCapabilities),
	}
}

func env(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func csvEnv(key string, fallback []string) []string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return append([]string(nil), fallback...)
	}
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			items = append(items, item)
		}
	}
	return items
}
```

- [ ] **Step 4: Run tests**

Run:

```bash
rtk go test ./internal/platform/config
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
rtk git add internal/platform/config
rtk git commit -m "feat: add platform config loading"
```

## Task 5: Core Governance Manifests

**Files:**
- Create: `internal/platform/core/capabilities.go`
- Create: `internal/platform/core/capabilities_test.go`

- [ ] **Step 1: Write failing core manifest test**

Create `internal/platform/core/capabilities_test.go`:

```go
package core

import (
	"testing"

	"platform-go/internal/platform/capability"
)

func TestDefaultManifestsResolve(t *testing.T) {
	registry := capability.NewRegistry()
	for _, manifest := range DefaultManifests() {
		if err := registry.Register(manifest); err != nil {
			t.Fatalf("Register(%q) error = %v", manifest.ID, err)
		}
	}

	enabled := make([]capability.ID, 0, len(DefaultManifests()))
	for _, manifest := range DefaultManifests() {
		enabled = append(enabled, manifest.ID)
	}

	ordered, err := registry.ResolveEnabled(enabled)
	if err != nil {
		t.Fatalf("ResolveEnabled() error = %v", err)
	}
	if len(ordered) != len(DefaultManifests()) {
		t.Fatalf("ResolveEnabled() returned %d manifests, want %d", len(ordered), len(DefaultManifests()))
	}
	if ordered[0].ID != "tenant" {
		t.Fatalf("first core capability = %q, want tenant", ordered[0].ID)
	}
}
```

- [ ] **Step 2: Run the failing test**

Run:

```bash
rtk go test ./internal/platform/core
```

Expected: FAIL because `DefaultManifests` is undefined.

- [ ] **Step 3: Implement core manifests**

Create `internal/platform/core/capabilities.go`:

```go
package core

import "platform-go/internal/platform/capability"

func DefaultManifests() []capability.Manifest {
	return []capability.Manifest{
		{ID: "tenant", Name: "Tenant", Version: "0.1.0"},
		{ID: "identity", Name: "Identity", Version: "0.1.0", Dependencies: []capability.ID{"tenant"}},
		{ID: "session", Name: "Session", Version: "0.1.0", Dependencies: []capability.ID{"identity"}},
		{ID: "rbac", Name: "RBAC", Version: "0.1.0", Dependencies: []capability.ID{"tenant", "identity"}},
		{ID: "menu", Name: "Menu", Version: "0.1.0", Dependencies: []capability.ID{"rbac"}},
		{ID: "api-resource", Name: "API Resource", Version: "0.1.0", Dependencies: []capability.ID{"rbac"}},
		{ID: "audit", Name: "Audit", Version: "0.1.0", Dependencies: []capability.ID{"tenant", "identity"}},
		{ID: "dictionary", Name: "Dictionary", Version: "0.1.0", Dependencies: []capability.ID{"tenant"}},
		{ID: "parameter", Name: "Parameter", Version: "0.1.0", Dependencies: []capability.ID{"tenant", "audit"}},
		{ID: "admin-shell", Name: "Admin Shell", Version: "0.1.0", Dependencies: []capability.ID{"identity", "session", "rbac", "menu"}},
		{ID: "system-admin", Name: "System Admin", Version: "0.1.0", Dependencies: []capability.ID{"admin-shell", "api-resource", "dictionary", "parameter", "audit"}},
	}
}
```

- [ ] **Step 4: Run tests**

Run:

```bash
rtk go test ./internal/platform/core
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
rtk git add internal/platform/core
rtk git commit -m "feat: declare core governance capabilities"
```

## Task 6: HTTP Runtime Skeleton

**Files:**
- Modify: `go.mod`
- Create: `internal/platform/httpapi/response.go`
- Create: `internal/platform/httpapi/server.go`
- Create: `internal/platform/httpapi/server_test.go`

- [ ] **Step 1: Add Gin dependency**

Run:

```bash
rtk go get github.com/gin-gonic/gin@v1.12.0
```

Expected: `go.mod` and `go.sum` are updated.

- [ ] **Step 2: Write failing HTTP tests**

Create `internal/platform/httpapi/server_test.go`:

```go
package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"platform-go/internal/platform/capability"
)

func TestHealthEndpoint(t *testing.T) {
	server := New(ServerOptions{Capabilities: []capability.Manifest{{ID: "tenant"}}})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /api/health status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"ok":true`) {
		t.Fatalf("GET /api/health body = %s", recorder.Body.String())
	}
}

func TestCapabilitiesEndpoint(t *testing.T) {
	server := New(ServerOptions{Capabilities: []capability.Manifest{{ID: "tenant"}, {ID: "identity"}}})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/capabilities", nil)

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /api/capabilities status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"id":"tenant"`) || !strings.Contains(body, `"id":"identity"`) {
		t.Fatalf("GET /api/capabilities body = %s", body)
	}
}
```

- [ ] **Step 3: Run the failing tests**

Run:

```bash
rtk go test ./internal/platform/httpapi
```

Expected: FAIL because HTTP server package is not implemented.

- [ ] **Step 4: Implement HTTP response and server**

Create `internal/platform/httpapi/response.go`:

```go
package httpapi

type Response[T any] struct {
	Data  T          `json:"data,omitempty"`
	Error *ErrorBody `json:"error,omitempty"`
}

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
```

Create `internal/platform/httpapi/server.go`:

```go
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
```

- [ ] **Step 5: Run tests**

Run:

```bash
rtk go test ./internal/platform/httpapi
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
rtk git add go.mod go.sum internal/platform/httpapi
rtk git commit -m "feat: add platform HTTP runtime"
```

## Task 7: API Command Composition

**Files:**
- Create: `cmd/platform-api/main.go`

- [ ] **Step 1: Create runnable API command**

Create `cmd/platform-api/main.go`:

```go
package main

import (
	"log"

	"platform-go/internal/platform/capability"
	"platform-go/internal/platform/config"
	"platform-go/internal/platform/core"
	"platform-go/internal/platform/httpapi"
)

func main() {
	cfg := config.Load()
	registry := capability.NewRegistry()
	for _, manifest := range core.DefaultManifests() {
		if err := registry.Register(manifest); err != nil {
			log.Fatalf("register capability %q: %v", manifest.ID, err)
		}
	}
	enabled := make([]capability.ID, 0, len(cfg.Capabilities))
	for _, id := range cfg.Capabilities {
		enabled = append(enabled, capability.ID(id))
	}
	ordered, err := registry.ResolveEnabled(enabled)
	if err != nil {
		log.Fatalf("resolve capabilities: %v", err)
	}
	server := httpapi.New(httpapi.ServerOptions{Capabilities: ordered})
	if err := server.Run(cfg.HTTPAddr); err != nil {
		log.Fatalf("run platform api: %v", err)
	}
}
```

- [ ] **Step 2: Run full Go tests**

Run:

```bash
rtk go test ./...
```

Expected: PASS.

- [ ] **Step 3: Smoke the API command**

Run:

```bash
rtk proxy sh -lc 'PLATFORM_HTTP_ADDR=127.0.0.1:19200 go run ./cmd/platform-api & pid=$!; sleep 1; curl -fsS http://127.0.0.1:19200/api/health; kill "$pid"; wait "$pid" 2>/dev/null || true'
```

Expected: the command prints a JSON health response containing `"ok":true` and exits after stopping the temporary server.

- [ ] **Step 4: Commit**

```bash
rtk git add cmd/platform-api
rtk git commit -m "feat: wire platform API command"
```

## Task 8: Admin Frontend Skeleton

**Files:**
- Create: `admin/package.json`
- Create: `admin/tsconfig.json`
- Create: `admin/vite.config.ts`
- Create: `admin/index.html`
- Create: `admin/src/main.tsx`
- Create: `admin/src/App.tsx`
- Create: `admin/src/platform/api/client.ts`
- Create: `admin/src/platform/shell/AdminShell.tsx`
- Create: `admin/src/platform/resources/registry.ts`
- Create: `admin/src/styles.css`

- [ ] **Step 1: Create admin package files**

Create `admin/package.json`:

```json
{
  "name": "platform-go-admin",
  "version": "0.1.0",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite --host 127.0.0.1 --port 9202",
    "typecheck": "tsc --noEmit",
    "build": "tsc --noEmit && vite build"
  },
  "dependencies": {
    "@vitejs/plugin-react": "^4.3.4",
    "antd": "^5.29.3",
    "vite": "^5.2.8",
    "react": "^18.3.1",
    "react-dom": "^18.3.1"
  },
  "devDependencies": {
    "@types/node": "^24.0.0",
    "@types/react": "^18.3.18",
    "@types/react-dom": "^18.3.5",
    "typescript": "^5.9.3"
  }
}
```

Create `admin/tsconfig.json`:

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "useDefineForClassFields": true,
    "lib": ["ES2022", "DOM", "DOM.Iterable"],
    "allowJs": false,
    "skipLibCheck": true,
    "esModuleInterop": true,
    "allowSyntheticDefaultImports": true,
    "strict": true,
    "forceConsistentCasingInFileNames": true,
    "module": "ESNext",
    "moduleResolution": "Node",
    "resolveJsonModule": true,
    "isolatedModules": true,
    "noEmit": true,
    "jsx": "react-jsx"
  },
  "include": ["src"],
  "references": []
}
```

Create `admin/vite.config.ts`:

```ts
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

export default defineConfig({
  plugins: [react()],
  server: {
    host: "127.0.0.1",
    port: 9202,
  },
});
```

Create `admin/index.html`:

```html
<!doctype html>
<html lang="zh-CN">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>platform-go admin</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

- [ ] **Step 2: Create admin source files**

Create `admin/src/platform/resources/registry.ts`:

```ts
export type AdminResourceDefinition = {
  name: string;
  route: string;
  title: string;
  description: string;
  permission: string;
};

export const coreResources: AdminResourceDefinition[] = [
  {
    name: "capabilities",
    route: "/capabilities",
    title: "能力清单",
    description: "查看当前平台启用的能力包。",
    permission: "admin:capability:read",
  },
];
```

Create `admin/src/platform/api/client.ts`:

```ts
const API_BASE = import.meta.env.VITE_PLATFORM_API_BASE ?? "http://127.0.0.1:9200/api";

export type PlatformResponse<T> = {
  data?: T;
  error?: {
    code: string;
    message: string;
  };
};

export type CapabilityItem = {
  id: string;
  name: string;
  version: string;
};

export async function request<T>(path: `/${string}`): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`);
  const payload = (await response.json()) as PlatformResponse<T>;
  if (!response.ok || payload.error) {
    throw new Error(payload.error?.message ?? `HTTP ${response.status}`);
  }
  return payload.data as T;
}

export function listCapabilities() {
  return request<CapabilityItem[]>("/capabilities");
}
```

Create `admin/src/platform/shell/AdminShell.tsx`:

```tsx
import { Layout, Menu, Typography } from "antd";
import type { AdminResourceDefinition } from "../resources/registry";

type AdminShellProps = {
  resources: AdminResourceDefinition[];
  children: React.ReactNode;
};

export function AdminShell({ resources, children }: AdminShellProps) {
  return (
    <Layout className="platform-shell">
      <Layout.Sider width={248} className="platform-sider">
        <div className="platform-brand">
          <div className="platform-logo">P</div>
          <div>
            <Typography.Text className="platform-title">platform-go</Typography.Text>
            <Typography.Text className="platform-subtitle">Capability Admin</Typography.Text>
          </div>
        </div>
        <Menu
          mode="inline"
          selectedKeys={[resources[0]?.route ?? "/"]}
          items={resources.map((resource) => ({
            key: resource.route,
            label: resource.title,
          }))}
        />
      </Layout.Sider>
      <Layout>
        <header className="platform-topbar">
          <Typography.Text className="platform-topbar-title">通用平台底座</Typography.Text>
        </header>
        <Layout.Content className="platform-content">{children}</Layout.Content>
      </Layout>
    </Layout>
  );
}
```

Create `admin/src/App.tsx`:

```tsx
import { Alert, Card, List, Spin, Typography } from "antd";
import { useEffect, useState } from "react";
import { listCapabilities, type CapabilityItem } from "./platform/api/client";
import { coreResources } from "./platform/resources/registry";
import { AdminShell } from "./platform/shell/AdminShell";

export default function App() {
  const [capabilities, setCapabilities] = useState<CapabilityItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    listCapabilities()
      .then((items) => {
        setCapabilities(items);
        setError("");
      })
      .catch((nextError: unknown) => {
        setError(nextError instanceof Error ? nextError.message : "能力清单加载失败");
      })
      .finally(() => setLoading(false));
  }, []);

  return (
    <AdminShell resources={coreResources}>
      <section className="platform-page">
        <Typography.Title level={2}>能力清单</Typography.Title>
        <Typography.Paragraph>当前页面验证 admin shell、平台 API client 和 capability introspection 的最小闭环。</Typography.Paragraph>
        {error ? <Alert type="warning" message="无法连接平台 API" description={error} showIcon /> : null}
        <Card>
          {loading ? (
            <Spin />
          ) : (
            <List
              dataSource={capabilities}
              renderItem={(item) => (
                <List.Item>
                  <List.Item.Meta title={item.name || item.id} description={`${item.id} · ${item.version || "0.1.0"}`} />
                </List.Item>
              )}
            />
          )}
        </Card>
      </section>
    </AdminShell>
  );
}
```

Create `admin/src/main.tsx`:

```tsx
import React from "react";
import ReactDOM from "react-dom/client";
import App from "./App";
import "./styles.css";

ReactDOM.createRoot(document.getElementById("root") as HTMLElement).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
```

Create `admin/src/styles.css`:

```css
body {
  margin: 0;
  font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
  background: #f5f7fb;
}

.platform-shell {
  min-height: 100vh;
}

.platform-sider {
  background: #101828 !important;
}

.platform-brand {
  display: flex;
  gap: 12px;
  align-items: center;
  height: 72px;
  padding: 0 20px;
}

.platform-logo {
  display: grid;
  width: 36px;
  height: 36px;
  place-items: center;
  border-radius: 8px;
  background: #2f80ed;
  color: #fff;
  font-weight: 700;
}

.platform-title,
.platform-subtitle {
  display: block;
  color: #fff;
}

.platform-subtitle {
  opacity: 0.64;
  font-size: 12px;
}

.platform-topbar {
  display: flex;
  align-items: center;
  height: 56px;
  padding: 0 24px;
  border-bottom: 1px solid #e4e7ec;
  background: #fff;
}

.platform-topbar-title {
  font-weight: 600;
}

.platform-content {
  padding: 24px;
}

.platform-page {
  max-width: 960px;
}
```

- [ ] **Step 3: Install and typecheck admin**

Run:

```bash
rtk npm --prefix admin install
rtk npm --prefix admin run typecheck
```

Expected: typecheck PASS.

- [ ] **Step 4: Build admin**

Run:

```bash
rtk npm --prefix admin run build
```

Expected: build PASS.

- [ ] **Step 5: Commit**

```bash
rtk git add admin
rtk git commit -m "feat: add admin shell skeleton"
```

## Task 9: README And Full Verification

**Files:**
- Create: `README.md`

- [ ] **Step 1: Write README**

Create `README.md`:

```markdown
# platform-go

Reusable operations platform foundation extracted from the `zshenmez` platform work.

## Current Slice

This repository currently contains the first framework skeleton:

- platform kernel primitives;
- capability registry and dependency resolution;
- default core governance manifests;
- Gin HTTP runtime with health and capability introspection;
- minimal React admin shell.

## API

```bash
rtk proxy sh -lc 'PLATFORM_HTTP_ADDR=127.0.0.1:19200 go run ./cmd/platform-api & pid=$!; sleep 1; curl -fsS http://127.0.0.1:19200/api/health; kill "$pid"; wait "$pid" 2>/dev/null || true'
```

Default API address:

```text
http://127.0.0.1:9200/api
```

Useful endpoints:

```text
GET /api/health
GET /api/capabilities
```

## Admin

```bash
rtk npm --prefix admin install
rtk npm --prefix admin run dev
```

Default admin address:

```text
http://127.0.0.1:9202
```

## Verification

```bash
rtk go test ./...
rtk npm --prefix admin run typecheck
rtk npm --prefix admin run build
rtk git diff --check
```
```

- [ ] **Step 2: Run full verification**

Run:

```bash
rtk go test ./...
rtk npm --prefix admin run typecheck
rtk npm --prefix admin run build
rtk git diff --check
```

Expected: all commands PASS.

- [ ] **Step 3: Check git status**

Run:

```bash
rtk git status --short
```

Expected: only planned files are modified or untracked.

- [ ] **Step 4: Commit**

```bash
rtk git add README.md
rtk git commit -m "docs: add platform skeleton startup guide"
```

## Final Acceptance

Run:

```bash
rtk go test ./...
rtk npm --prefix admin run build
rtk git status --short
```

Expected:

- Go tests pass.
- Admin build passes.
- Git status is clean after the final commits.
