package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"platform-go/internal/apps"
	"platform-go/internal/platform/adminresource"
	"platform-go/internal/platform/bootstrap"
	"platform-go/internal/platform/capability"
	"platform-go/internal/platform/config"
	"platform-go/internal/platform/dataprotection"
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

func TestRunBindAdminOIDCRedactsMalformedArgumentTokens(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{name: "invalid subject stdin bool", args: append(bindArgs("admin")[:len(bindArgs("admin"))-1], "--subject-stdin="+testSubject)},
		{name: "raw subject embedded in unknown flag name", args: append(bindArgs("admin"), "--unknown-"+testSubject)},
		{name: "raw subject embedded in unknown subject flag", args: append(bindArgs("admin"), "--subject="+testSubject)},
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
	if !hasProvisionAudit(audits, "user-admin") {
		t.Fatalf("audit logs = %+v, want redacted provisioning audit", audits)
	}
	boundAudits := bindingAudits(audits, adminresource.AdminIdentityBindingAuditOutcomeBound)
	if len(boundAudits) != 1 {
		t.Fatalf("bound audits = %+v, want one across replay", boundAudits)
	}
	assertCLIBindingAuditRedacted(t, boundAudits[0], bindings[0], "admin", "ops")
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
	audits, err := store.List("audit-logs")
	if err != nil {
		t.Fatalf("List(audit-logs) error = %v", err)
	}
	conflictAudits := bindingAudits(audits, adminresource.AdminIdentityBindingAuditOutcomeConflict)
	if len(conflictAudits) != 1 {
		t.Fatalf("conflict audits = %+v, want one", conflictAudits)
	}
	assertCLIBindingAuditRedacted(t, conflictAudits[0], bindings[0], "admin", "ops")
	if conflictAudits[0].Values["actor"] != "system:platform" {
		t.Fatalf("conflict audit actor = %q, want explicit system ID", conflictAudits[0].Values["actor"])
	}
}

func TestRunBindAdminOIDCAuditFailureCanRetryWithoutDuplicates(t *testing.T) {
	cfg := testConfig(t)
	repository := newCLIAuditRepository()
	loader := func(_ config.Config, manifests []capability.Manifest, protection dataprotection.Runtime) (*adminresource.Store, error) {
		return adminresource.NewRepositoryBackedStoreFromCapabilitiesWithProtection(repository, manifests, protection)
	}
	repository.failSaveNumber(2, errors.New("sensitive audit failure "+testSubject))

	failed := executeWithResources(t, cfg, bindArgs("admin"), testSubject, loader)
	assertRejectedWithoutSecrets(t, failed, testSubject, testIssuer)
	if errorText(failed.err) != "record Admin OIDC binding provisioning audit" {
		t.Fatalf("success audit failure error = %q, want sanitized error", failed.err)
	}
	succeeded := executeWithResources(t, cfg, bindArgs("admin"), testSubject, loader)
	if succeeded.err != nil {
		t.Fatalf("success audit retry error = %v", succeeded.err)
	}
	if replayed := executeWithResources(t, cfg, bindArgs("admin"), testSubject, loader); replayed.err != nil {
		t.Fatalf("success audit replay error = %v", replayed.err)
	}

	repository.failNextSaveWith(errors.New("sensitive conflict audit failure " + testSubject))
	conflictFailed := executeWithResources(t, cfg, bindArgs("ops"), testSubject, loader)
	assertRejectedWithoutSecrets(t, conflictFailed, testSubject, testIssuer)
	if errorText(conflictFailed.err) != "record Admin OIDC binding conflict audit" {
		t.Fatalf("conflict audit failure error = %q, want sanitized error", conflictFailed.err)
	}
	conflictRetried := executeWithResources(t, cfg, bindArgs("ops"), testSubject, loader)
	assertRejectedWithoutSecrets(t, conflictRetried, testSubject, testIssuer)
	if errorText(conflictRetried.err) != "Admin OIDC binding provisioning was rejected" {
		t.Fatalf("conflict retry error = %q, want normalized rejection", conflictRetried.err)
	}

	store, err := loader(cfg, testManifests(t, cfg), nil)
	if err != nil {
		t.Fatalf("load store after retries error = %v", err)
	}
	audits, err := store.List("audit-logs")
	if err != nil {
		t.Fatalf("List(audit-logs) error = %v", err)
	}
	if got := len(bindingAudits(audits, adminresource.AdminIdentityBindingAuditOutcomeBound)); got != 1 {
		t.Fatalf("bound audit count = %d, want 1", got)
	}
	if got := len(bindingAudits(audits, adminresource.AdminIdentityBindingAuditOutcomeConflict)); got != 1 {
		t.Fatalf("conflict audit count = %d, want 1", got)
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

func executeWithResources(t *testing.T, cfg config.Config, args []string, stdin string, loader adminResourcesLoader) executionResult {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runWithAdminResources(context.Background(), args, strings.NewReader(stdin), &stdout, &stderr, func() config.Config { return cfg }, loader)
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
	manifests := testManifests(t, cfg)
	store, err := bootstrap.AdminResourcesFromConfig(cfg, manifests, nil)
	if err != nil {
		t.Fatalf("AdminResourcesFromConfig() error = %v", err)
	}
	return store
}

func testManifests(t *testing.T, cfg config.Config) []capability.Manifest {
	t.Helper()
	manifests, err := bootstrap.CapabilitiesFromConfig(cfg, apps.DefaultManifests()...)
	if err != nil {
		t.Fatalf("CapabilitiesFromConfig() error = %v", err)
	}
	return manifests
}

func hasProvisionAudit(records []adminresource.Record, actorID string) bool {
	for _, record := range records {
		if record.Values["action"] == "admin_identity.bind" && record.Values["actor"] == actorID && record.Values["provider"] == "" {
			return true
		}
	}
	return false
}

func bindingAudits(records []adminresource.Record, outcome string) []adminresource.Record {
	matching := make([]adminresource.Record, 0)
	for _, record := range records {
		if record.Values["action"] == "admin_identity.bind" && record.Values["outcome"] == outcome {
			matching = append(matching, record)
		}
	}
	return matching
}

func assertCLIBindingAuditRedacted(t *testing.T, audit adminresource.Record, binding adminresource.Record, usernames ...string) {
	t.Helper()
	for _, value := range []string{testSubject, testIssuer, binding.Values["issuerHash"], binding.Values["providerSubjectHash"]} {
		if value != "" && (strings.Contains(audit.Code, value) || containsMapValue(audit.Values, value)) {
			t.Fatalf("audit exposed raw identity or hash: %+v", audit)
		}
	}
	if audit.Values["outcome"] == adminresource.AdminIdentityBindingAuditOutcomeConflict {
		for _, username := range usernames {
			if containsMapValue(audit.Values, username) {
				t.Fatalf("conflict audit exposed username: %+v", audit)
			}
		}
	}
}

func containsMapValue(values map[string]string, expected string) bool {
	for _, value := range values {
		if value == expected {
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

type cliAuditRepository struct {
	mu              sync.Mutex
	snapshot        adminresource.ResourceSnapshot
	saveCount       int
	failAtSave      int
	failAtSaveErr   error
	failNextSaveErr error
}

func newCLIAuditRepository() *cliAuditRepository {
	return &cliAuditRepository{snapshot: adminresource.ResourceSnapshot{Resources: map[string][]adminresource.Record{}}}
}

func (r *cliAuditRepository) Load(context.Context) (adminresource.ResourceSnapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return cloneCLIResourceSnapshot(r.snapshot), nil
}

func (r *cliAuditRepository) Save(_ context.Context, snapshot adminresource.ResourceSnapshot) (uint64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.saveCount++
	if r.failAtSave == r.saveCount {
		err := r.failAtSaveErr
		if err == nil {
			err = errors.New("injected save failure")
		}
		r.failAtSaveErr = nil
		return 0, err
	}
	if r.failNextSaveErr != nil {
		err := r.failNextSaveErr
		r.failNextSaveErr = nil
		return 0, err
	}
	if snapshot.Revision != r.snapshot.Revision {
		return 0, &adminresource.RevisionConflictError{Expected: snapshot.Revision, Actual: r.snapshot.Revision}
	}
	snapshot.Revision++
	r.snapshot = cloneCLIResourceSnapshot(snapshot)
	return snapshot.Revision, nil
}

func (r *cliAuditRepository) failSaveNumber(number int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failAtSave = number
	r.failAtSaveErr = err
}

func (r *cliAuditRepository) failNextSaveWith(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failAtSave = 0
	r.failNextSaveErr = err
}

func cloneCLIResourceSnapshot(snapshot adminresource.ResourceSnapshot) adminresource.ResourceSnapshot {
	cloned := adminresource.ResourceSnapshot{Revision: snapshot.Revision, NextID: snapshot.NextID, Resources: make(map[string][]adminresource.Record, len(snapshot.Resources))}
	for resource, records := range snapshot.Resources {
		items := make([]adminresource.Record, 0, len(records))
		for _, record := range records {
			values := make(map[string]string, len(record.Values))
			for key, value := range record.Values {
				values[key] = value
			}
			record.Values = values
			items = append(items, record)
		}
		cloned.Resources[resource] = items
	}
	return cloned
}
