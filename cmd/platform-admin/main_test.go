package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"platform-go/internal/apps"
	"platform-go/internal/platform/adminresource"
	"platform-go/internal/platform/bootstrap"
	"platform-go/internal/platform/capability"
	"platform-go/internal/platform/config"
	"platform-go/internal/platform/datalifecycle"
	"platform-go/internal/platform/dataprotection"
	"platform-go/internal/platform/sensitivemigration"
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

func TestRunSensitiveDataMigrationAcceptsExactModesAndEmitsOneJSONReport(t *testing.T) {
	for _, mode := range []sensitivemigration.Mode{
		sensitivemigration.ModeInventory,
		sensitivemigration.ModeDryRun,
		sensitivemigration.ModePrepare,
		sensitivemigration.ModeApply,
		sensitivemigration.ModeVerify,
		sensitivemigration.ModeRehearseRestore,
		sensitivemigration.ModeRollback,
	} {
		t.Run(string(mode), func(t *testing.T) {
			session := &fakeSensitiveMigrationSession{
				planHash: "sha256:" + strings.Repeat("a", 64),
				report:   sensitivemigration.Report{RunID: "run-1", Mode: mode, Status: sensitivemigration.StatusCompleted},
			}
			args := []string{"sensitive-data-migrate", "--mode", string(mode), "--batch-size", "25"}
			switch mode {
			case sensitivemigration.ModePrepare, sensitivemigration.ModeApply, sensitivemigration.ModeRehearseRestore, sensitivemigration.ModeRollback:
				args = append(args, mutationCLIArgs()...)
			case sensitivemigration.ModeVerify:
				args = append(args, "--run-id", "run-1")
			}
			result := executeWithSensitiveMigration(t, args, session, nil)
			if result.err != nil {
				t.Fatalf("run() error = %v", result.err)
			}
			if result.stderr != "" || strings.Count(result.stdout, "\n") != 1 {
				t.Fatalf("stdout = %q stderr = %q, want one JSON line and empty stderr", result.stdout, result.stderr)
			}
			var report sensitivemigration.Report
			if err := json.Unmarshal([]byte(result.stdout), &report); err != nil {
				t.Fatalf("stdout is not a JSON Report: %v", err)
			}
			if report != session.report {
				t.Fatalf("report = %+v, want %+v", report, session.report)
			}
			if session.options.Mode != mode || session.options.BatchSize != 25 || session.closes != 1 {
				t.Fatalf("session options = %+v closes=%d", session.options, session.closes)
			}
		})
	}
}

func TestRunSensitiveDataMigrationRejectsMalformedOrIncompleteArgumentsWithoutValues(t *testing.T) {
	secret := "sensitive-cli-token-and-dsn"
	for _, tc := range []struct {
		name string
		args []string
	}{
		{name: "unknown mode", args: []string{"sensitive-data-migrate", "--mode", secret}},
		{name: "unknown flag", args: []string{"sensitive-data-migrate", "--mode", "inventory", "--unknown-" + secret}},
		{name: "positional", args: []string{"sensitive-data-migrate", "--mode", "inventory", secret}},
		{name: "malformed batch", args: []string{"sensitive-data-migrate", "--mode", "inventory", "--batch-size", secret}},
		{name: "zero batch", args: []string{"sensitive-data-migrate", "--mode", "inventory", "--batch-size", "0"}},
		{name: "small batch", args: []string{"sensitive-data-migrate", "--mode", "inventory", "--batch-size", "-1"}},
		{name: "large batch", args: []string{"sensitive-data-migrate", "--mode", "inventory", "--batch-size", "1001"}},
		{name: "verify missing run", args: []string{"sensitive-data-migrate", "--mode", "verify"}},
		{name: "prepare missing approvals", args: []string{"sensitive-data-migrate", "--mode", "prepare", "--run-id", "run-1"}},
		{name: "apply missing approvals", args: []string{"sensitive-data-migrate", "--mode", "apply", "--run-id", "run-1"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			session := &fakeSensitiveMigrationSession{planHash: "sha256:" + strings.Repeat("a", 64)}
			result := executeWithSensitiveMigration(t, tc.args, session, nil)
			if result.err == nil {
				t.Fatal("run() error = nil")
			}
			combined := result.stdout + result.stderr + result.err.Error()
			if strings.Contains(combined, secret) {
				t.Fatalf("error output exposed raw argument: %q", combined)
			}
			if result.stdout != "" || result.stderr != "" {
				t.Fatalf("stdout=%q stderr=%q, want empty on error", result.stdout, result.stderr)
			}
		})
	}
}

func TestRunSensitiveDataMigrationClosesSessionAndNormalizesOperationalErrors(t *testing.T) {
	secret := "database-secret-dsn"
	t.Run("open", func(t *testing.T) {
		result := executeWithSensitiveMigration(t, []string{"sensitive-data-migrate", "--mode", "inventory"}, nil, errors.New(secret))
		assertSensitiveMigrationOperationalError(t, result, secret)
	})
	t.Run("run", func(t *testing.T) {
		session := &fakeSensitiveMigrationSession{planHash: "sha256:" + strings.Repeat("a", 64), err: errors.New(secret)}
		result := executeWithSensitiveMigration(t, []string{"sensitive-data-migrate", "--mode", "inventory"}, session, nil)
		assertSensitiveMigrationOperationalError(t, result, secret)
		if session.closes != 1 {
			t.Fatalf("Close() calls = %d, want 1", session.closes)
		}
	})
}

func TestRunSensitiveDataMigrationEmitsReportBeforeNormalizedCloseFailure(t *testing.T) {
	secret := "sensitive-close-failure-marker"
	session := &fakeSensitiveMigrationSession{
		planHash: "sha256:" + strings.Repeat("a", 64),
		report: sensitivemigration.Report{
			RunID: "run-close-failure", Mode: sensitivemigration.ModeInventory, Status: sensitivemigration.StatusCompleted,
		},
		closeErr: errors.New(secret),
	}
	result := executeWithSensitiveMigration(t, []string{"sensitive-data-migrate", "--mode", "inventory"}, session, nil)
	if result.err == nil || result.err.Error() != "close sensitive data migration storage" {
		t.Fatalf("run() error = %v, want normalized close failure", result.err)
	}
	if strings.Contains(result.err.Error()+result.stdout+result.stderr, secret) {
		t.Fatalf("result exposed close failure value: %+v", result)
	}
	if result.stderr != "" || strings.Count(result.stdout, "\n") != 1 {
		t.Fatalf("stdout=%q stderr=%q, want one JSON line and empty stderr", result.stdout, result.stderr)
	}
	var report sensitivemigration.Report
	if err := json.Unmarshal([]byte(result.stdout), &report); err != nil {
		t.Fatalf("stdout is not a JSON Report: %v", err)
	}
	if report != session.report || session.closes != 1 {
		t.Fatalf("report=%+v closes=%d, want %+v and one close", report, session.closes, session.report)
	}
}

func TestRunDataLifecycleAcceptsExactOperationsAndEmitsOneSanitizedJSONObject(t *testing.T) {
	proposed := lifecycleTestPolicy(30)
	proposedFingerprint, err := datalifecycle.PolicyFingerprint(proposed)
	if err != nil {
		t.Fatal(err)
	}
	current := lifecycleTestPolicy(90)
	currentPolicyFile := writeLifecyclePolicyFile(t, `{
		"version": 1,
		"resources": [{"resource":"files","mode":"tombstone","policyVersion":1,"retentionDays":90,"autoPurge":true}]
	}`)
	impactHash := "sha256:" + strings.Repeat("a", 64)
	promotion := datalifecycle.Promotion{
		DatasourceID: datalifecycle.DefaultDatasourceID, CurrentFingerprint: lifecyclePolicyFingerprint(t, current),
		PromotedFingerprint: proposedFingerprint, ImpactReportHash: impactHash,
		ActorID: "security-admin", Reason: "approved-retention-change", ApprovalRef: "CAB-2026-0714",
		PromotedAt: time.Date(2026, 7, 14, 9, 0, 0, 0, time.UTC),
	}

	t.Run("prepare", func(t *testing.T) {
		var prepareCalls int
		result := executeWithDataLifecycle(t, []string{"data-lifecycle", "--operation", "prepare"}, nil, nil,
			func(context.Context, config.Config) error {
				prepareCalls++
				return nil
			})
		if result.err != nil || prepareCalls != 1 {
			t.Fatalf("result=%+v prepareCalls=%d", result, prepareCalls)
		}
		assertSingleLifecycleJSON(t, result, map[string]any{"operation": "prepare", "status": "completed"})
	})

	for _, mode := range []datalifecycle.Mode{datalifecycle.ModeImpact, datalifecycle.ModeDryRun} {
		t.Run(string(mode), func(t *testing.T) {
			session := &fakeDataLifecycleSession{
				policy: proposed, fingerprint: proposedFingerprint,
				report: datalifecycle.Report{
					DatasourceID: datalifecycle.DefaultDatasourceID, RunID: "retention-run-1",
					Mode: mode, Status: datalifecycle.StatusCompleted, PolicyFingerprint: proposedFingerprint,
				},
			}
			result := executeWithDataLifecycle(t, []string{
				"data-lifecycle", "--operation", string(mode), "--run-id", "retention-run-1",
				"--owner", "maintenance-1", "--batch-size", "25", "--max-retries", "2",
			}, session, nil, nil)
			if result.err != nil {
				t.Fatalf("run() error = %v", result.err)
			}
			if !session.options.Enabled || session.options.Mode != mode || session.options.RunID != "retention-run-1" ||
				session.options.OwnerID != "maintenance-1" || session.options.BatchSize != 25 || session.options.MaxRetries != 2 {
				t.Fatalf("options = %+v", session.options)
			}
			assertSingleLifecycleReport(t, result, session.report)
			if session.closes != 1 {
				t.Fatalf("Close() calls = %d, want 1", session.closes)
			}
		})
	}

	t.Run("promote", func(t *testing.T) {
		session := &fakeDataLifecycleSession{policy: proposed, fingerprint: proposedFingerprint, promotion: promotion}
		result := executeWithDataLifecycle(t, append([]string{
			"data-lifecycle", "--operation", "promote",
		}, lifecyclePromotionArgs(currentPolicyFile, proposedFingerprint, impactHash)...), session, nil, nil)
		if result.err != nil {
			t.Fatalf("run() error = %v", result.err)
		}
		if !session.promotionRequest.Enabled || session.promotionRequest.DatasourceID != datalifecycle.DefaultDatasourceID ||
			session.promotionRequest.CurrentPolicy.Version != current.Version ||
			session.promotionRequest.CurrentPolicy.Resources[0].Mode != capability.AdminDeletionTombstone ||
			session.promotionRequest.PromotedFingerprint != proposedFingerprint ||
			session.promotionRequest.ImpactReportHash != impactHash {
			t.Fatalf("promotion request = %+v", session.promotionRequest)
		}
		if session.promotionRequest.ActorID != "security-admin" || session.promotionRequest.Reason != "approved-retention-change" ||
			session.promotionRequest.ApprovalRef != "CAB-2026-0714" {
			t.Fatalf("promotion evidence = %+v", session.promotionRequest)
		}
		var output lifecyclePromotionOutput
		assertSingleLifecycleJSONInto(t, result, &output)
		if output.PromotedFingerprint != proposedFingerprint || output.ImpactReportHash != impactHash {
			t.Fatalf("promotion output = %+v", output)
		}
		if strings.Contains(result.stdout, "security-admin") || strings.Contains(result.stdout, "CAB-2026-0714") || strings.Contains(result.stdout, "approved-retention-change") {
			t.Fatalf("promotion output exposed approval evidence: %s", result.stdout)
		}
	})

	t.Run("apply", func(t *testing.T) {
		session := &fakeDataLifecycleSession{
			policy: proposed, fingerprint: proposedFingerprint,
			report: datalifecycle.Report{
				DatasourceID: datalifecycle.DefaultDatasourceID, RunID: "apply-run-1", Mode: datalifecycle.ModeApply,
				Status: datalifecycle.StatusCompleted, PolicyFingerprint: proposedFingerprint,
			},
		}
		args := []string{
			"data-lifecycle", "--operation", "apply", "--run-id", "apply-run-1", "--owner", "maintenance-1",
			"--dry-run-id", "dry-run-1", "--batch-size", "50", "--max-retries", "1", "--confirm-apply",
		}
		args = append(args, lifecyclePromotionArgs(currentPolicyFile, proposedFingerprint, impactHash)...)
		result := executeWithDataLifecycle(t, args, session, nil, nil)
		if result.err != nil {
			t.Fatalf("run() error = %v", result.err)
		}
		if session.options.Mode != datalifecycle.ModeApply ||
			session.options.Promotion.PromotedFingerprint != proposedFingerprint ||
			session.options.Promotion.CurrentFingerprint != lifecyclePolicyFingerprint(t, current) ||
			session.options.Promotion.DryRunID != "dry-run-1" ||
			session.options.Promotion.ImpactReportHash != impactHash || session.options.Promotion.ActorID != "security-admin" {
			t.Fatalf("apply options = %+v", session.options)
		}
		assertSingleLifecycleReport(t, result, session.report)
	})
}

func TestRunDataLifecycleRejectsMalformedIncompleteAndOperationIncompatibleArguments(t *testing.T) {
	policy := lifecycleTestPolicy(30)
	fingerprint := lifecyclePolicyFingerprint(t, policy)
	policyFile := writeLifecyclePolicyFile(t, `{"version":1,"resources":[{"resource":"files","mode":"tombstone","policyVersion":1,"retentionDays":90,"autoPurge":true}]}`)
	impactHash := "sha256:" + strings.Repeat("a", 64)
	secret := "sensitive-lifecycle-cli-value"
	validApply := []string{
		"data-lifecycle", "--operation", "apply", "--run-id", "apply-run-1", "--dry-run-id", "dry-run-1", "--owner", "maintenance-1", "--confirm-apply",
	}
	validApply = append(validApply, lifecyclePromotionArgs(policyFile, fingerprint, impactHash)...)

	tests := []struct {
		name           string
		args           []string
		expectedCloses int
	}{
		{name: "unknown operation", args: []string{"data-lifecycle", "--operation", secret}},
		{name: "unknown flag", args: []string{"data-lifecycle", "--operation", "impact", "--unknown-" + secret}},
		{name: "positional", args: []string{"data-lifecycle", "--operation", "impact", secret}},
		{name: "missing operation", args: []string{"data-lifecycle"}},
		{name: "impact missing run", args: []string{"data-lifecycle", "--operation", "impact", "--owner", "maintenance-1"}},
		{name: "dry run missing owner", args: []string{"data-lifecycle", "--operation", "dry-run", "--run-id", "run-1"}},
		{name: "zero batch", args: []string{"data-lifecycle", "--operation", "impact", "--run-id", "run-1", "--owner", "owner-1", "--batch-size", "0"}},
		{name: "large batch", args: []string{"data-lifecycle", "--operation", "impact", "--run-id", "run-1", "--owner", "owner-1", "--batch-size", "1001"}},
		{name: "negative retries", args: []string{"data-lifecycle", "--operation", "impact", "--run-id", "run-1", "--owner", "owner-1", "--max-retries", "-1"}},
		{name: "large retries", args: []string{"data-lifecycle", "--operation", "impact", "--run-id", "run-1", "--owner", "owner-1", "--max-retries", "6"}},
		{name: "prepare with run", args: []string{"data-lifecycle", "--operation", "prepare", "--run-id", "run-1"}},
		{name: "impact with promotion", args: append([]string{"data-lifecycle", "--operation", "impact", "--run-id", "run-1", "--owner", "owner-1"}, lifecyclePromotionArgs(policyFile, fingerprint, impactHash)...)},
		{name: "promote with run", args: append([]string{"data-lifecycle", "--operation", "promote", "--run-id", "run-1"}, lifecyclePromotionArgs(policyFile, fingerprint, impactHash)...)},
		{name: "promote missing policy", args: []string{"data-lifecycle", "--operation", "promote", "--promoted-fingerprint", fingerprint, "--impact-report-hash", impactHash, "--actor", "actor-1", "--reason", "approved", "--approval-ref", "CAB-1"}},
		{name: "apply without confirmation", args: removeLifecycleArgument(validApply, "--confirm-apply")},
		{name: "apply missing dry run", args: removeLifecycleArgument(validApply, "--dry-run-id", "dry-run-1")},
		{name: "apply missing exact field", args: removeLifecycleArgument(validApply, "--approval-ref", "CAB-2026-0714")},
		{name: "apply fingerprint mismatch", args: replaceLifecycleArgument(validApply, fingerprint, "sha256:"+strings.Repeat("b", 64)), expectedCloses: 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			session := &fakeDataLifecycleSession{policy: policy, fingerprint: fingerprint}
			result := executeWithDataLifecycle(t, tc.args, session, nil, func(context.Context, config.Config) error { return nil })
			assertDataLifecycleRejected(t, result, secret)
			if session.runCalls != 0 || session.promoteCalls != 0 || session.closes != tc.expectedCloses {
				t.Fatalf("session was used for rejected input: %+v", session)
			}
		})
	}
}

func TestRunDataLifecycleStrictlyBoundsAndDecodesCurrentPolicyFile(t *testing.T) {
	policy := lifecycleTestPolicy(30)
	fingerprint := lifecyclePolicyFingerprint(t, policy)
	impactHash := "sha256:" + strings.Repeat("a", 64)
	oversized := writeLifecyclePolicyFile(t, strings.Repeat(" ", maximumLifecyclePolicyBytes+1))
	for _, tc := range []struct {
		name    string
		content string
		path    string
	}{
		{name: "unknown field", content: `{"version":1,"resources":[],"dsn":"secret-dsn"}`},
		{name: "case variant field", content: `{"Version":1,"resources":[]}`},
		{name: "duplicate field", content: `{"version":1,"version":2,"resources":[]}`},
		{name: "duplicate resource field", content: `{"version":1,"resources":[{"resource":"files","resource":"users","policyVersion":1,"retentionDays":90,"autoPurge":true}]}`},
		{name: "missing deletion mode", content: `{"version":1,"resources":[{"resource":"files","policyVersion":1,"retentionDays":90,"autoPurge":true}]}`},
		{name: "invalid deletion mode", content: `{"version":1,"resources":[{"resource":"files","mode":"physical-delete","policyVersion":1,"retentionDays":90,"autoPurge":true}]}`},
		{name: "trailing object", content: `{"version":1,"resources":[]} {"version":2}`},
		{name: "wrong shape", content: `{"version":"1","resources":[]}`},
		{name: "oversized", path: oversized},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := tc.path
			if path == "" {
				path = writeLifecyclePolicyFile(t, tc.content)
			}
			args := append([]string{"data-lifecycle", "--operation", "promote"}, lifecyclePromotionArgs(path, fingerprint, impactHash)...)
			session := &fakeDataLifecycleSession{policy: policy, fingerprint: fingerprint}
			result := executeWithDataLifecycle(t, args, session, nil, nil)
			assertDataLifecycleRejected(t, result, "secret-dsn", tc.content)
			if session.promoteCalls != 0 || session.closes != 0 {
				t.Fatalf("session was used for invalid policy: %+v", session)
			}
		})
	}
}

func TestRunDataLifecycleNormalizesOperationalErrorsAndClosesSession(t *testing.T) {
	policy := lifecycleTestPolicy(30)
	fingerprint := lifecyclePolicyFingerprint(t, policy)
	secret := "sensitive-lifecycle-dependency-error"
	baseArgs := []string{"data-lifecycle", "--operation", "impact", "--run-id", "run-1", "--owner", "owner-1"}
	currentPolicyFile := writeLifecyclePolicyFile(t, `{"version":1,"resources":[{"resource":"files","mode":"tombstone","policyVersion":1,"retentionDays":90,"autoPurge":true}]}`)
	impactHash := "sha256:" + strings.Repeat("a", 64)

	t.Run("prepare", func(t *testing.T) {
		result := executeWithDataLifecycle(t, []string{"data-lifecycle", "--operation", "prepare"}, nil, nil,
			func(context.Context, config.Config) error { return errors.New(secret) })
		assertDataLifecycleRejected(t, result, secret)
	})
	t.Run("open", func(t *testing.T) {
		result := executeWithDataLifecycle(t, baseArgs, nil, errors.New(secret), nil)
		assertDataLifecycleRejected(t, result, secret)
	})
	t.Run("run", func(t *testing.T) {
		session := &fakeDataLifecycleSession{policy: policy, fingerprint: fingerprint, runErr: errors.New(secret)}
		result := executeWithDataLifecycle(t, baseArgs, session, nil, nil)
		assertDataLifecycleRejected(t, result, secret)
		if session.closes != 1 {
			t.Fatalf("Close() calls = %d, want 1", session.closes)
		}
	})
	t.Run("promote", func(t *testing.T) {
		session := &fakeDataLifecycleSession{policy: policy, fingerprint: fingerprint, promoteErr: errors.New(secret)}
		args := append([]string{"data-lifecycle", "--operation", "promote"}, lifecyclePromotionArgs(currentPolicyFile, fingerprint, impactHash)...)
		result := executeWithDataLifecycle(t, args, session, nil, nil)
		assertDataLifecycleRejected(t, result, secret)
		if session.closes != 1 {
			t.Fatalf("Close() calls = %d, want 1", session.closes)
		}
	})
	t.Run("close", func(t *testing.T) {
		session := &fakeDataLifecycleSession{
			policy: policy, fingerprint: fingerprint,
			report:   datalifecycle.Report{DatasourceID: "default", RunID: "run-1", Mode: datalifecycle.ModeImpact, Status: datalifecycle.StatusCompleted},
			closeErr: errors.New(secret),
		}
		result := executeWithDataLifecycle(t, baseArgs, session, nil, nil)
		assertDataLifecycleRejected(t, result, secret)
		if session.closes != 1 {
			t.Fatalf("Close() calls = %d, want 1", session.closes)
		}
	})
}

type fakeDataLifecycleSession struct {
	policy           datalifecycle.PolicySnapshot
	fingerprint      string
	options          datalifecycle.Options
	report           datalifecycle.Report
	promotionRequest datalifecycle.PromotionRequest
	promotion        datalifecycle.Promotion
	runErr           error
	promoteErr       error
	closeErr         error
	runCalls         int
	promoteCalls     int
	closes           int
}

func (s *fakeDataLifecycleSession) Policy() datalifecycle.PolicySnapshot { return s.policy }
func (s *fakeDataLifecycleSession) PolicyFingerprint() string            { return s.fingerprint }

func (s *fakeDataLifecycleSession) Run(_ context.Context, options datalifecycle.Options) (datalifecycle.Report, error) {
	s.runCalls++
	s.options = options
	return s.report, s.runErr
}

func (s *fakeDataLifecycleSession) Promote(_ context.Context, request datalifecycle.PromotionRequest) (datalifecycle.Promotion, error) {
	s.promoteCalls++
	s.promotionRequest = request
	return s.promotion, s.promoteErr
}

func (s *fakeDataLifecycleSession) Close() error {
	s.closes++
	return s.closeErr
}

func executeWithDataLifecycle(t *testing.T, args []string, session dataLifecycleSession, openErr error, prepare dataLifecyclePreparer) executionResult {
	t.Helper()
	if prepare == nil {
		prepare = func(context.Context, config.Config) error { return nil }
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runWithPlatformDependencies(context.Background(), args, strings.NewReader(""), &stdout, &stderr,
		func() config.Config { return config.Config{} }, nil, nil, prepare,
		func(config.Config, ...capability.Manifest) (dataLifecycleSession, error) { return session, openErr })
	return executionResult{stdout: stdout.String(), stderr: stderr.String(), err: err}
}

func lifecycleTestPolicy(retentionDays int) datalifecycle.PolicySnapshot {
	return datalifecycle.PolicySnapshot{Version: 1, Resources: []datalifecycle.ResourcePolicy{{
		Resource: "files", Mode: capability.AdminDeletionTombstone, PolicyVersion: 1, RetentionDays: retentionDays, AutoPurge: true,
	}}}
}

func lifecyclePolicyFingerprint(t *testing.T, policy datalifecycle.PolicySnapshot) string {
	t.Helper()
	fingerprint, err := datalifecycle.PolicyFingerprint(policy)
	if err != nil {
		t.Fatal(err)
	}
	return fingerprint
}

func writeLifecyclePolicyFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "current-policy.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func lifecyclePromotionArgs(policyFile, fingerprint, impactHash string) []string {
	return []string{
		"--current-policy-file", policyFile,
		"--promoted-fingerprint", fingerprint,
		"--impact-report-hash", impactHash,
		"--actor", "security-admin",
		"--reason", "approved-retention-change",
		"--approval-ref", "CAB-2026-0714",
	}
}

func removeLifecycleArgument(args []string, target ...string) []string {
	result := append([]string(nil), args...)
	for index := 0; index <= len(result)-len(target); index++ {
		if strings.Join(result[index:index+len(target)], "\x00") == strings.Join(target, "\x00") {
			return append(result[:index], result[index+len(target):]...)
		}
	}
	return result
}

func replaceLifecycleArgument(args []string, oldValue, newValue string) []string {
	result := append([]string(nil), args...)
	for index, value := range result {
		if value == oldValue {
			result[index] = newValue
			break
		}
	}
	return result
}

func assertSingleLifecycleReport(t *testing.T, result executionResult, expected datalifecycle.Report) {
	t.Helper()
	var report datalifecycle.Report
	assertSingleLifecycleJSONInto(t, result, &report)
	if !reflect.DeepEqual(report, expected) {
		t.Fatalf("report = %+v, want %+v", report, expected)
	}
}

func assertSingleLifecycleJSON(t *testing.T, result executionResult, expected map[string]any) {
	t.Helper()
	var output map[string]any
	assertSingleLifecycleJSONInto(t, result, &output)
	for key, value := range expected {
		if output[key] != value {
			t.Fatalf("output[%q] = %#v, want %#v", key, output[key], value)
		}
	}
}

func assertSingleLifecycleJSONInto(t *testing.T, result executionResult, target any) {
	t.Helper()
	if result.err != nil || result.stderr != "" || strings.Count(result.stdout, "\n") != 1 {
		t.Fatalf("result = %+v, want one JSON line", result)
	}
	if err := json.Unmarshal([]byte(result.stdout), target); err != nil {
		t.Fatalf("stdout is not valid JSON: %v", err)
	}
}

func assertDataLifecycleRejected(t *testing.T, result executionResult, values ...string) {
	t.Helper()
	if result.err == nil || result.stdout != "" || result.stderr != "" {
		t.Fatalf("result = %+v, want normalized failure and empty output", result)
	}
	for _, value := range values {
		if value != "" && strings.Contains(result.err.Error()+result.stdout+result.stderr, value) {
			t.Fatalf("result exposed sensitive value %q: %+v", value, result)
		}
	}
}

type fakeSensitiveMigrationSession struct {
	planHash string
	options  sensitivemigration.Options
	report   sensitivemigration.Report
	err      error
	closeErr error
	closes   int
}

func (s *fakeSensitiveMigrationSession) PlanHash() string { return s.planHash }

func (s *fakeSensitiveMigrationSession) Run(_ context.Context, options sensitivemigration.Options) (sensitivemigration.Report, error) {
	s.options = options
	return s.report, s.err
}

func (s *fakeSensitiveMigrationSession) Close() error {
	s.closes++
	return s.closeErr
}

func executeWithSensitiveMigration(t *testing.T, args []string, session sensitiveMigrationSession, openErr error) executionResult {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runWithDependencies(context.Background(), args, strings.NewReader(""), &stdout, &stderr, func() config.Config { return config.Config{} }, nil,
		func(config.Config, ...capability.Manifest) (sensitiveMigrationSession, error) {
			return session, openErr
		})
	return executionResult{stdout: stdout.String(), stderr: stderr.String(), err: err}
}

func mutationCLIArgs() []string {
	return []string{
		"--run-id", "run-1",
		"--actor", "operator-1",
		"--reason", "approved-maintenance",
		"--approval-ref", "approval-1",
		"--backup-uri", "s3://backup/platform-1",
		"--backup-sha256", "sha256:" + strings.Repeat("b", 64),
		"--restore-evidence-ref", "restore-test-1",
		"--maintenance-window-confirmed",
	}
}

func assertSensitiveMigrationOperationalError(t *testing.T, result executionResult, secret string) {
	t.Helper()
	if result.err == nil {
		t.Fatal("run() error = nil")
	}
	if strings.Contains(result.err.Error(), secret) || result.stdout != "" || result.stderr != "" {
		t.Fatalf("result = %+v, want value-free error and empty output", result)
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
