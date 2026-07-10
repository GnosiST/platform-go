# Platform Go Stack Alignment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Correct `platform-go` so its foundation matches the `jiedanshi/platform` reference stack before more UI or business feature work is added.

**Architecture:** Keep the existing capability concept, but align infrastructure with the source framework: Gin + GORM + Casbin + JWT on the backend and Refine + React + Ant Design on the admin frontend. The first slice adds real stack seams and the admin resource manifest/generation loop while keeping the current app runnable; later slices migrate persistence, authorization and UI pages behind those seams.

**Tech Stack:** Go 1.26, Gin, GORM, Casbin, JWT, Redis, TypeScript, React, Refine, Ant Design, Vite.

**Status on 2026-07-06:** Completed as the stack-alignment baseline. The current codebase has GORM storage, Casbin authorization, JWT admin/app tokens, Refine runtime adapters and the admin resource generation loop in place. Follow-up work is tracked in `docs/platform-roadmap.md` and `docs/platform-foundation-task-map.md`; do not treat the historical task body below as active work.

---

## Scope Check

This plan does not rewrite every existing handler or resource in one pass. It builds the stack alignment skeleton and guardrails:

- GORM dependency and storage opener.
- Casbin dependency and policy enforcer wrapper.
- JWT token service.
- Refine dependencies and provider entrypoints.
- Admin resource manifest, generated contract, preview, scaffold dry-run safety plan, generated scaffold file package, scaffold draft and validator.
- Roadmap/docs alignment.

Source-writing code generation, arbitrary form slots and full system module migration are later plans.

## File Structure

Create:

- `internal/platform/storage/gorm.go` - GORM opener for mysql, postgres and sqlite.
- `internal/platform/storage/gorm_test.go` - SQLite opener tests.
- `internal/platform/authz/casbin.go` - Casbin enforcer wrapper.
- `internal/platform/authz/casbin_test.go` - policy enforcement tests.
- `internal/platform/authjwt/token.go` - JWT signing and parsing service.
- `internal/platform/authjwt/token_test.go` - token tests.
- `resources/admin-resources.json` - platform base admin resource manifest.
- `scripts/generate-admin-resource-contract.mjs` - platform base plus enabled capability admin resources to stable contract.
- `scripts/generate-admin-codegen-preview.mjs` - contract to readonly preview.
- `scripts/generate-admin-scaffold-plan.mjs` - preview to dry-run safety plan.
- `scripts/generate-admin-scaffold-files.mjs` - dry-run safety plan to generated scaffold file package.
- `scripts/generate-admin-scaffold-draft.mjs` - preview plus safety plan to scaffold checklist.
- `scripts/validate-admin-resources.mjs` - manifest and generated output validator.
- `resources/generated/admin-resource-contract.json` - generated stable contract.
- `resources/generated/admin-codegen-preview.json` - generated readonly preview.
- `resources/generated/admin-scaffold-plan.json` - generated scaffold dry-run safety plan.
- `resources/generated/admin-scaffold-files.json` - generated scaffold file manifest.
- `resources/generated/admin-scaffold-draft.md` - generated scaffold checklist.
- `admin/src/platform/refine/dataProvider.ts` - Refine data provider adapter over the generic admin resource API, including structured keyword/condition meta for schema-driven lists.
- `admin/src/platform/refine/authProvider.ts` - Refine auth adapter over existing token/session APIs.
- `admin/src/platform/refine/accessControlProvider.ts` - Refine permission adapter.

Modify:

- `go.mod` / `go.sum` - add GORM, Casbin and JWT dependencies.
- `admin/package.json` / lockfile - add Refine and React Router dependencies.
- `internal/platform/config/config.go` and tests - add database/JWT config keys.
- `README.md` - document corrected stack and validation commands.
- `docs/platform-roadmap.md` - keep stack correction as P0.

## Task 1: Add Backend Stack Dependencies

- [x] **Step 1: Add dependencies**

Run:

```bash
rtk go get github.com/casbin/casbin/v2@v2.135.0 github.com/golang-jwt/jwt/v5@v5.3.1 gorm.io/gorm@v1.31.1 gorm.io/driver/mysql@v1.6.0 gorm.io/driver/postgres@v1.6.0 gorm.io/driver/sqlite@v1.6.0 github.com/go-sql-driver/mysql@v1.9.3
```

Expected: `go.mod` includes direct requirements for Casbin, JWT and GORM packages.

- [x] **Step 2: Verify module graph**

Run:

```bash
rtk go mod tidy
rtk go test ./...
```

Expected: all existing Go tests pass.

## Task 2: Add GORM Storage Opener

- [x] **Step 1: Write failing tests**

Create `internal/platform/storage/gorm_test.go`:

```go
package storage

import "testing"

func TestOpenGORMRejectsUnknownDriver(t *testing.T) {
	_, err := OpenGORM(Config{Driver: "mongo", DSN: "memory"})
	if err == nil {
		t.Fatalf("OpenGORM() error = nil, want unknown driver")
	}
}

func TestOpenGORMOpensSQLiteMemory(t *testing.T) {
	db, err := OpenGORM(Config{Driver: "sqlite", DSN: ":memory:"})
	if err != nil {
		t.Fatalf("OpenGORM(sqlite) error = %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB() error = %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := sqlDB.Ping(); err != nil {
		t.Fatalf("sqlite ping error = %v", err)
	}
}
```

- [x] **Step 2: Run failing test**

Run:

```bash
rtk go test ./internal/platform/storage
```

Expected: FAIL because `OpenGORM` and `Config` are undefined.

- [x] **Step 3: Implement storage opener**

Create `internal/platform/storage/gorm.go`:

```go
package storage

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var ErrUnknownDriver = errors.New("unknown gorm driver")

type Config struct {
	Driver string
	DSN    string
}

func OpenGORM(config Config) (*gorm.DB, error) {
	driver := strings.TrimSpace(config.Driver)
	dsn := strings.TrimSpace(config.DSN)
	if dsn == "" {
		dsn = defaultDSN(driver)
	}
	switch driver {
	case "mysql":
		return gorm.Open(mysql.Open(dsn), &gorm.Config{})
	case "postgres":
		return gorm.Open(postgres.Open(dsn), &gorm.Config{})
	case "sqlite":
		return gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnknownDriver, driver)
	}
}

func defaultDSN(driver string) string {
	if driver == "sqlite" {
		return ":memory:"
	}
	return ""
}
```

- [x] **Step 4: Verify**

Run:

```bash
rtk go test ./internal/platform/storage
```

Expected: PASS.

## Task 3: Add Casbin Authorization Wrapper

- [x] **Step 1: Write failing tests**

Create `internal/platform/authz/casbin_test.go`:

```go
package authz

import "testing"

func TestCasbinAuthorizerAllowsRolePolicy(t *testing.T) {
	authorizer, err := NewCasbinAuthorizer([]RolePolicy{
		{RoleCode: "admin", Tenant: "platform", Permission: "admin:user:read", Action: "read"},
	}, []UserRole{
		{User: "user:1", RoleCode: "admin", Tenant: "platform"},
	})
	if err != nil {
		t.Fatalf("NewCasbinAuthorizer() error = %v", err)
	}
	if !authorizer.Can("user:1", "platform", "admin:user:read", "read") {
		t.Fatalf("Can() = false, want true")
	}
	if authorizer.Can("user:1", "platform", "admin:user:write", "write") {
		t.Fatalf("Can(write) = true, want false")
	}
}
```

- [x] **Step 2: Run failing test**

Run:

```bash
rtk go test ./internal/platform/authz
```

Expected: FAIL because `NewCasbinAuthorizer`, `RolePolicy` and `UserRole` are undefined.

- [x] **Step 3: Implement authorizer**

Create `internal/platform/authz/casbin.go` with the Casbin model from `jiedanshi/platform/api/internal/auth/casbin.go`, adapted to string tenant and user keys:

```go
package authz

import (
	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
)

type RolePolicy struct {
	RoleCode   string
	Tenant     string
	Permission string
	Action     string
}

type UserRole struct {
	User     string
	RoleCode string
	Tenant   string
}

type CasbinAuthorizer struct {
	enforcer *casbin.Enforcer
}

func NewCasbinAuthorizer(policies []RolePolicy, roles []UserRole) (*CasbinAuthorizer, error) {
	m, err := model.NewModelFromString(`
[request_definition]
r = sub, tenant, obj, act

[policy_definition]
p = role, tenant, obj, act

[role_definition]
g = _, _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.role, r.tenant) && r.tenant == p.tenant && r.obj == p.obj && (p.act == "*" || r.act == p.act)
`)
	if err != nil {
		return nil, err
	}
	enforcer, err := casbin.NewEnforcer(m)
	if err != nil {
		return nil, err
	}
	for _, policy := range policies {
		if _, err := enforcer.AddPolicy(policy.RoleCode, policy.Tenant, policy.Permission, policy.Action); err != nil {
			return nil, err
		}
	}
	for _, role := range roles {
		if _, err := enforcer.AddGroupingPolicy(role.User, role.RoleCode, role.Tenant); err != nil {
			return nil, err
		}
	}
	return &CasbinAuthorizer{enforcer: enforcer}, nil
}

func (a *CasbinAuthorizer) Can(user string, tenant string, permission string, action string) bool {
	if a == nil || a.enforcer == nil {
		return false
	}
	ok, err := a.enforcer.Enforce(user, tenant, permission, action)
	return err == nil && ok
}
```

- [x] **Step 4: Verify**

Run:

```bash
rtk go test ./internal/platform/authz
```

Expected: PASS.

## Task 4: Add JWT Token Service

- [x] **Step 1: Write failing tests**

Create `internal/platform/authjwt/token_test.go`:

```go
package authjwt

import (
	"testing"
	"time"
)

func TestTokenServiceSignsAndParsesAdminToken(t *testing.T) {
	service := NewService("secret", func() time.Time { return time.Unix(100, 0).UTC() })
	token, expiresAt, err := service.Sign(Subject{
		UserID: "user-1", TenantID: "platform", Username: "admin", SessionID: "session-1", TokenType: TokenTypeAdmin,
	}, time.Hour)
	if err != nil {
		t.Fatalf("Sign() error = %v", err)
	}
	if expiresAt.IsZero() {
		t.Fatalf("expiresAt is zero")
	}
	claims, err := service.Parse(token)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if claims.UserID != "user-1" || claims.TokenType != TokenTypeAdmin {
		t.Fatalf("claims = %+v, want admin user-1", claims)
	}
}
```

- [x] **Step 2: Run failing test**

Run:

```bash
rtk go test ./internal/platform/authjwt
```

Expected: FAIL because JWT service types are undefined.

- [x] **Step 3: Implement token service**

Create `internal/platform/authjwt/token.go`:

```go
package authjwt

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type TokenType string

const (
	TokenTypeAdmin TokenType = "admin"
	TokenTypeApp   TokenType = "app"
)

type Subject struct {
	UserID    string
	TenantID  string
	Username  string
	SessionID string
	TokenType TokenType
}

type Claims struct {
	UserID    string    `json:"userId"`
	TenantID  string    `json:"tenantId"`
	Username  string    `json:"username"`
	SessionID string    `json:"sessionId"`
	TokenType TokenType `json:"tokenType"`
	jwt.RegisteredClaims
}

type Service struct {
	secret string
	now    func() time.Time
}

func NewService(secret string, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{secret: secret, now: now}
}

func (s *Service) Sign(subject Subject, ttl time.Duration) (string, time.Time, error) {
	if ttl <= 0 {
		ttl = time.Hour
	}
	expiresAt := s.now().Add(ttl)
	claims := Claims{
		UserID: subject.UserID, TenantID: subject.TenantID, Username: subject.Username, SessionID: subject.SessionID, TokenType: subject.TokenType,
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(expiresAt), IssuedAt: jwt.NewNumericDate(s.now())},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(s.secret))
	return signed, expiresAt, err
}

func (s *Service) Parse(tokenValue string) (Claims, error) {
	parsed, err := jwt.ParseWithClaims(tokenValue, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected jwt signing method")
		}
		return []byte(s.secret), nil
	})
	if err != nil {
		return Claims{}, err
	}
	claims, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return Claims{}, errors.New("invalid jwt token")
	}
	return *claims, nil
}
```

- [x] **Step 4: Verify**

Run:

```bash
rtk go test ./internal/platform/authjwt
```

Expected: PASS.

## Task 5: Add Refine Dependencies And Provider Entry Points

- [x] **Step 1: Add dependencies**

Run:

```bash
rtk npm --prefix admin install @refinedev/core@5.0.12 @refinedev/antd@6.0.3 @refinedev/react-router@2.0.4 react-router@7.9.6 react-router-dom@7.9.6
```

Expected: `admin/package.json` includes Refine packages.

- [x] **Step 2: Add provider adapter files**

Create `admin/src/platform/refine/dataProvider.ts`, `authProvider.ts` and `accessControlProvider.ts` as thin adapters over the current API client. The data provider must carry list/create/update/delete through Refine's data-provider contract instead of leaving placeholder exports.

- [x] **Step 3: Verify**

Run:

```bash
rtk npm --prefix admin run typecheck
rtk npm --prefix admin run build
```

Expected: PASS.

## Task 6: Port Admin Resource Manifest Loop

- [x] **Step 1: Create system-only manifest**

Create `resources/admin-resources.json` with system resources only:

- dashboard;
- roles;
- roleGroups;
- menus;
- apiResources;
- users;
- tenants;
- orgUnits;
- permissions;
- dictionaries;
- parameters;
- auditLogs;
- loginLogs;
- errorLogs;
- files;
- sessions;
- versions;
- apiTokens.

- [x] **Step 2: Port generation scripts**

Create scripts equivalent to the `jiedanshi/platform/scripts` manifest loop, adapted to this repo root:

```bash
rtk node scripts/generate-admin-resource-contract.mjs
rtk node scripts/generate-admin-codegen-preview.mjs
rtk node scripts/generate-admin-scaffold-plan.mjs
rtk node scripts/generate-admin-scaffold-files.mjs
rtk node scripts/generate-admin-scaffold-draft.mjs
rtk node scripts/validate-admin-resources.mjs
```

Expected: generated files are stable and validator passes.

## Task 7: Update Verification Gates

- [x] **Step 1: Add README commands**

Add:

```bash
rtk node scripts/validate-admin-resources.mjs
```

to the verification section.

- [x] **Step 2: Run full gate**

Run:

```bash
rtk go test ./...
rtk npm --prefix admin run typecheck
rtk npm --prefix admin run build
rtk node scripts/validate-admin-resources.mjs
rtk git diff --check
```

Expected: all pass.

## Task 8: Stop And Replan Migration

Status: completed by the current roadmap/task-map split. The next slices are no longer the broad migration items below; they are tracked as production auth hardening, policy refresh/invalidation, Refine form/action convergence, generator hardening and detachable `external-business-capability` parity.

After Task 7, do not keep adding UI features on the old stack. Write the next plan for:

1. GORM-backed system resources.
2. Casbin-backed authorization replacement.
3. JWT auth replacement.
4. Refine App integration and dynamic resources.
5. Approved UI componentization on Refine.
