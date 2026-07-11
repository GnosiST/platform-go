package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"platform-go/internal/apps"
	"platform-go/internal/platform/adminresource"
	"platform-go/internal/platform/bootstrap"
	"platform-go/internal/platform/config"
)

const (
	testIssuer  = "https://id.example/realms/platform"
	testSubject = "subject-raw-value-9f23c"
)

func TestRunBindAdminOIDCRequiresSubjectStdin(t *testing.T) {
	cfg := testConfig(t)
	result := execute(t, cfg, []string{"bind-admin-oidc", "--provider", "oidc", "--issuer", testIssuer, "--username", "admin"}, testSubject)
	assertRejectedWithoutSecrets(t, result, testSubject, testIssuer)
}

func TestRunBindAdminOIDCRejectsEmptyStdin(t *testing.T) {
	cfg := testConfig(t)
	result := execute(t, cfg, bindArgs("admin"), " \n\t")
	if result.err == nil {
		t.Fatal("run() error = nil, want empty subject rejection")
	}
}

func TestRunBindAdminOIDCRejectsSubjectArguments(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{name: "subject flag", args: append(bindArgs("admin"), "--subject", testSubject)},
		{name: "positional subject", args: append(bindArgs("admin"), testSubject)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testConfig(t)
			result := execute(t, cfg, tc.args, "")
			assertRejectedWithoutSecrets(t, result, testSubject, testIssuer)
		})
	}
}

func TestRunBindAdminOIDCRejectsUnknownProvider(t *testing.T) {
	cfg := testConfig(t)
	args := bindArgs("admin")
	args[2] = "unknown"
	result := execute(t, cfg, args, testSubject)
	assertRejectedWithoutSecrets(t, result, testSubject, testIssuer)
}

func TestRunBindAdminOIDCRejectsIncompleteOIDCConfig(t *testing.T) {
	cfg := testConfig(t)
	cfg.AdminOIDCClientSecret = ""
	result := execute(t, cfg, bindArgs("admin"), testSubject)
	assertRejectedWithoutSecrets(t, result, testSubject, testIssuer)
}

func TestRunBindAdminOIDCRejectsInvalidAdminPrincipal(t *testing.T) {
	var normalizedError string
	for _, tc := range []struct {
		name     string
		username string
		prepare  func(*testing.T, config.Config)
	}{
		{name: "missing user", username: "missing"},
		{name: "disabled user", username: "admin", prepare: func(t *testing.T, cfg config.Config) {
			updateUser(t, cfg, "admin", func(record *adminresource.Record) { record.Status = "disabled" })
		}},
		{name: "user without permissions", username: "admin", prepare: func(t *testing.T, cfg config.Config) {
			updateUser(t, cfg, "admin", func(record *adminresource.Record) {
				record.Values["role"] = "missing-role"
				record.Values["roles"] = "missing-role"
			})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testConfig(t)
			if tc.prepare != nil {
				tc.prepare(t, cfg)
			}
			result := execute(t, cfg, bindArgs(tc.username), testSubject)
			assertRejectedWithoutSecrets(t, result, testSubject, testIssuer)
			if normalizedError == "" {
				normalizedError = errorText(result.err)
			} else if errorText(result.err) != normalizedError {
				t.Fatalf("error = %q, want normalized %q", result.err, normalizedError)
			}
		})
	}
}

func TestRunBindAdminOIDCIsIdempotentAndRedactsRawIdentity(t *testing.T) {
	cfg := testConfig(t)
	first := execute(t, cfg, bindArgs("admin"), testSubject)
	if first.err != nil {
		t.Fatalf("first run() error = %v, stderr = %q", first.err, first.stderr)
	}
	second := execute(t, cfg, bindArgs("admin"), testSubject)
	if second.err != nil {
		t.Fatalf("second run() error = %v, stderr = %q", second.err, second.stderr)
	}
	for _, result := range []executionResult{first, second} {
		if result.stdout != "provider=oidc username=admin\n" {
			t.Fatalf("stdout = %q, want provider and username only", result.stdout)
		}
		if result.stderr != "" {
			t.Fatalf("stderr = %q, want empty", result.stderr)
		}
		assertRedacted(t, []string{testSubject, testIssuer}, result.stdout, result.stderr)
	}

	store := loadStore(t, cfg)
	bindings, err := store.List("admin-identities")
	if err != nil {
		t.Fatalf("List(admin-identities) error = %v", err)
	}
	if len(bindings) != 1 || bindings[0].Values["platformUsername"] != "admin" {
		t.Fatalf("admin identity bindings = %+v, want one admin binding", bindings)
	}
	audits, err := store.List("audit-logs")
	if err != nil {
		t.Fatalf("List(audit-logs) error = %v", err)
	}
	encoded, err := json.Marshal(struct {
		Bindings []adminresource.Record `json:"bindings"`
		Audits   []adminresource.Record `json:"audits"`
	}{Bindings: bindings, Audits: audits})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	assertRedacted(t, []string{testSubject, testIssuer}, string(encoded), readFile(t, cfg.AdminResourceFile))
	if !hasProvisionAudit(audits, "admin", "oidc") {
		t.Fatalf("audit logs = %+v, want redacted provisioning audit", audits)
	}
}

func TestRunBindAdminOIDCRejectsConflictingBindingWithoutRawIdentityLeak(t *testing.T) {
	cfg := testConfig(t)
	if result := execute(t, cfg, bindArgs("admin"), testSubject); result.err != nil {
		t.Fatalf("initial run() error = %v", result.err)
	}
	conflict := execute(t, cfg, bindArgs("ops"), testSubject)
	assertRejectedWithoutSecrets(t, conflict, testSubject, testIssuer)
	assertRedacted(t, []string{testSubject, testIssuer}, conflict.stdout, conflict.stderr, errorText(conflict.err), readFile(t, cfg.AdminResourceFile))

	store := loadStore(t, cfg)
	bindings, err := store.List("admin-identities")
	if err != nil {
		t.Fatalf("List(admin-identities) error = %v", err)
	}
	if len(bindings) != 1 || bindings[0].Values["platformUsername"] != "admin" {
		t.Fatalf("bindings after conflict = %+v, want original admin binding", bindings)
	}
}

type executionResult struct {
	stdout string
	stderr string
	err    error
}

func execute(t *testing.T, cfg config.Config, args []string, stdin string) executionResult {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := run(context.Background(), args, strings.NewReader(stdin), &stdout, &stderr, func() config.Config { return cfg })
	return executionResult{stdout: stdout.String(), stderr: stderr.String(), err: err}
}

func bindArgs(username string) []string {
	return []string{"bind-admin-oidc", "--provider", "oidc", "--issuer", testIssuer, "--username", username, "--subject-stdin"}
}

func testConfig(t *testing.T) config.Config {
	t.Helper()
	return config.Config{
		RuntimeEnvironment:      config.RuntimeEnvironmentTest,
		HTTPAddr:                "127.0.0.1:9200",
		Capabilities:            []string{"dictionary", "tenant", "identity", "session", "rbac", "audit", "admin-oidc"},
		AdminResourceFile:       filepath.Join(t.TempDir(), "admin-resources.json"),
		JWTSecret:               "test-platform-admin-secret",
		CacheDefaultTTL:         time.Minute,
		AdminOIDCIssuerURL:      testIssuer,
		AdminOIDCClientID:       "platform-admin",
		AdminOIDCClientSecret:   "client-secret",
		AdminOIDCRedirectURL:    "http://127.0.0.1:3000/login",
		AdminOIDCScopes:         []string{"openid", "profile", "email"},
		DisableDemoAuthProvider: true,
	}
}

func updateUser(t *testing.T, cfg config.Config, username string, mutate func(*adminresource.Record)) {
	t.Helper()
	store := loadStore(t, cfg)
	users, err := store.List("users")
	if err != nil {
		t.Fatalf("List(users) error = %v", err)
	}
	for _, user := range users {
		if user.Code != username {
			continue
		}
		mutate(&user)
		if _, err := store.Update("users", user.ID, adminresource.WriteInput{
			Code: user.Code, Name: user.Name, Status: user.Status, Description: user.Description, Values: user.Values,
		}); err != nil {
			t.Fatalf("Update(users, %s) error = %v", username, err)
		}
		return
	}
	t.Fatalf("user %q not found", username)
}

func loadStore(t *testing.T, cfg config.Config) *adminresource.Store {
	t.Helper()
	manifests, err := bootstrap.CapabilitiesFromConfig(cfg, apps.DefaultManifests()...)
	if err != nil {
		t.Fatalf("CapabilitiesFromConfig() error = %v", err)
	}
	store, err := bootstrap.AdminResourcesFromConfig(cfg, manifests)
	if err != nil {
		t.Fatalf("AdminResourcesFromConfig() error = %v", err)
	}
	return store
}

func hasProvisionAudit(records []adminresource.Record, username string, provider string) bool {
	for _, record := range records {
		if record.Values["action"] == "admin_identity.bind" && record.Values["actor"] == username && record.Values["provider"] == provider {
			return true
		}
	}
	return false
}

func assertRejectedWithoutSecrets(t *testing.T, result executionResult, secrets ...string) {
	t.Helper()
	if result.err == nil {
		t.Fatal("run() error = nil, want rejection")
	}
	if result.stdout != "" {
		t.Fatalf("stdout = %q, want empty on rejection", result.stdout)
	}
	assertRedacted(t, secrets, result.stdout, result.stderr, errorText(result.err))
}

func assertRedacted(t *testing.T, secrets []string, contents ...string) {
	t.Helper()
	for _, content := range contents {
		for _, secret := range secrets {
			if secret != "" && strings.Contains(content, secret) {
				t.Fatalf("sensitive identity value leaked in %q", content)
			}
		}
	}
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return ""
	}
	if err != nil {
		t.Fatalf("os.ReadFile(%s) error = %v", path, err)
	}
	return string(content)
}
