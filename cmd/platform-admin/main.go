package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"platform-go/internal/apps"
	"platform-go/internal/platform/adminresource"
	"platform-go/internal/platform/bootstrap"
	"platform-go/internal/platform/capability"
	"platform-go/internal/platform/config"
	"platform-go/internal/platform/datalifecycle"
	"platform-go/internal/platform/dataprotection"
	"platform-go/internal/platform/httpapi"
	"platform-go/internal/platform/organizationrbac"
	"platform-go/internal/platform/sensitivemigration"
)

const (
	bindAdminOIDCCommand             = "bind-admin-oidc"
	sensitiveDataMigrationCommand    = "sensitive-data-migrate"
	dataLifecycleCommand             = "data-lifecycle"
	organizationRBACMigrationCommand = "organization-rbac-migrate"
	dataLifecyclePrepareOperation    = "prepare"
	dataLifecyclePromoteOperation    = "promote"
	maximumLifecyclePolicyBytes      = 1 << 20
)

type adminResourcesLoader func(config.Config, []capability.Manifest, dataprotection.Runtime) (*adminresource.Store, error)

type sensitiveMigrationSession interface {
	PlanHash() string
	Run(context.Context, sensitivemigration.Options) (sensitivemigration.Report, error)
	Close() error
}

type sensitiveMigrationOpener func(config.Config, ...capability.Manifest) (sensitiveMigrationSession, error)

type dataLifecycleSession interface {
	Policy() datalifecycle.PolicySnapshot
	PolicyFingerprint() string
	Run(context.Context, datalifecycle.Options) (datalifecycle.Report, error)
	Promote(context.Context, datalifecycle.PromotionRequest) (datalifecycle.Promotion, error)
	Close() error
}

type dataLifecyclePreparer func(context.Context, config.Config) error
type dataLifecycleOpener func(config.Config, ...capability.Manifest) (dataLifecycleSession, error)

type organizationRBACMigrationSession interface {
	RunMigration(context.Context, organizationrbac.MigrationMode, organizationrbac.MigrationManifest, organizationrbac.MigrationEvidence) (organizationrbac.MigrationReport, error)
	Close() error
}

type organizationRBACPreparer func(context.Context, config.Config) error
type organizationRBACMigrationOpener func(context.Context, config.Config) (organizationRBACMigrationSession, error)

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdin, os.Stdout, os.Stderr, config.Load); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, loadConfig func() config.Config) error {
	if len(args) > 0 && args[0] == organizationRBACMigrationCommand {
		return runOrganizationRBACMigration(ctx, args, stdout, loadConfig, bootstrap.PrepareOrganizationRBAC,
			func(ctx context.Context, cfg config.Config) (organizationRBACMigrationSession, error) {
				runtime, err := bootstrap.OpenOrganizationRBACMigration(ctx, cfg)
				if err != nil {
					return nil, err
				}
				return &organizationRBACMigrationAdapter{runtime: runtime}, nil
			})
	}
	return runWithPlatformDependencies(ctx, args, stdin, stdout, stderr, loadConfig, bootstrap.AdminResourcesFromConfig,
		func(cfg config.Config, manifests ...capability.Manifest) (sensitiveMigrationSession, error) {
			return bootstrap.OpenSensitiveDataMigration(cfg, manifests...)
		}, bootstrap.PrepareDataLifecycle,
		func(cfg config.Config, manifests ...capability.Manifest) (dataLifecycleSession, error) {
			return bootstrap.OpenDataLifecycle(cfg, manifests...)
		})
}

type organizationRBACMigrationAdapter struct {
	runtime *bootstrap.OrganizationRBACMigration
}

func (a *organizationRBACMigrationAdapter) RunMigration(ctx context.Context, mode organizationrbac.MigrationMode, manifest organizationrbac.MigrationManifest, evidence organizationrbac.MigrationEvidence) (organizationrbac.MigrationReport, error) {
	return a.runtime.Repository.RunMigration(ctx, mode, manifest, evidence)
}

func (a *organizationRBACMigrationAdapter) Close() error { return a.runtime.Close() }

func runWithAdminResources(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, loadConfig func() config.Config, loadAdminResources adminResourcesLoader) error {
	return runWithDependencies(ctx, args, stdin, stdout, stderr, loadConfig, loadAdminResources,
		func(cfg config.Config, manifests ...capability.Manifest) (sensitiveMigrationSession, error) {
			return bootstrap.OpenSensitiveDataMigration(cfg, manifests...)
		})

}

func runWithDependencies(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, loadConfig func() config.Config, loadAdminResources adminResourcesLoader, openSensitiveMigration sensitiveMigrationOpener) error {
	return runWithPlatformDependencies(ctx, args, stdin, stdout, stderr, loadConfig, loadAdminResources, openSensitiveMigration,
		bootstrap.PrepareDataLifecycle,
		func(cfg config.Config, manifests ...capability.Manifest) (dataLifecycleSession, error) {
			return bootstrap.OpenDataLifecycle(cfg, manifests...)
		})
}

func runWithPlatformDependencies(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, loadConfig func() config.Config, loadAdminResources adminResourcesLoader, openSensitiveMigration sensitiveMigrationOpener, prepareDataLifecycle dataLifecyclePreparer, openDataLifecycle dataLifecycleOpener) error {
	if len(args) == 0 {
		return errors.New("expected bind-admin-oidc, sensitive-data-migrate, or data-lifecycle command")
	}
	switch args[0] {
	case bindAdminOIDCCommand:
		return runBindAdminOIDC(ctx, args, stdin, stdout, loadConfig, loadAdminResources)
	case sensitiveDataMigrationCommand:
		return runSensitiveDataMigration(ctx, args, stdout, stderr, loadConfig, openSensitiveMigration)
	case dataLifecycleCommand:
		return runDataLifecycle(ctx, args, stdout, stderr, loadConfig, prepareDataLifecycle, openDataLifecycle)
	default:
		return errors.New("expected bind-admin-oidc, sensitive-data-migrate, or data-lifecycle command")
	}
}

func runBindAdminOIDC(ctx context.Context, args []string, stdin io.Reader, stdout io.Writer, loadConfig func() config.Config, loadAdminResources adminResourcesLoader) error {
	flags := flag.NewFlagSet(bindAdminOIDCCommand, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	providerID := flags.String("provider", "", "configured Admin OIDC provider ID")
	issuer := flags.String("issuer", "", "configured OIDC issuer")
	username := flags.String("username", "", "existing platform username")
	subjectStdin := flags.Bool("subject-stdin", false, "read the raw OIDC subject from stdin")
	if err := flags.Parse(args[1:]); err != nil {
		return errors.New("invalid bind-admin-oidc arguments")
	}
	if len(flags.Args()) != 0 {
		return errors.New("positional arguments are not accepted")
	}
	if strings.TrimSpace(*providerID) == "" || strings.TrimSpace(*issuer) == "" || strings.TrimSpace(*username) == "" {
		return errors.New("provider, issuer, and username are required")
	}
	if !*subjectStdin {
		return errors.New("subject-stdin is required")
	}
	if loadConfig == nil {
		return errors.New("platform configuration is unavailable")
	}
	if loadAdminResources == nil {
		return errors.New("persistent Admin resource storage is unavailable")
	}

	cfg := loadConfig()
	if err := cfg.ValidateRuntime(); err != nil {
		return fmt.Errorf("invalid platform configuration: %w", err)
	}
	if strings.TrimSpace(cfg.AdminResourceDriver) == "" && strings.TrimSpace(cfg.AdminResourceFile) == "" {
		return errors.New("persistent Admin resource storage is required")
	}

	manifests, err := bootstrap.CapabilitiesFromConfig(cfg, apps.DefaultManifests()...)
	if err != nil {
		return fmt.Errorf("resolve platform capabilities: %w", err)
	}
	provider, ok := findAdminOIDCProvider(manifests, *providerID)
	if !ok || strings.TrimSpace(*issuer) != strings.TrimSpace(cfg.AdminOIDCIssuerURL) {
		return errors.New("configured Admin OIDC provider is unavailable")
	}

	dataProtection, err := bootstrap.DataProtectionRuntimeFromConfig(cfg)
	if err != nil {
		return fmt.Errorf("build data protection runtime: %w", err)
	}
	resources, err := loadAdminResources(cfg, manifests, dataProtection)
	if err != nil {
		return fmt.Errorf("open persistent Admin resource store: %w", err)
	}
	subjectBytes, err := io.ReadAll(stdin)
	if err != nil {
		return errors.New("read OIDC subject from stdin")
	}
	subject := strings.TrimSpace(string(subjectBytes))
	if subject == "" {
		return errors.New("OIDC subject from stdin is empty")
	}

	bindings := httpapi.NewResourceAdminIdentityBindingStore(resources, time.Now)
	binding, err := bindings.ProvisionAdminIdentityBinding(ctx, httpapi.AdminIdentityProvisionInput{
		Provider:        provider,
		Issuer:          *issuer,
		ProviderSubject: subject,
		Username:        *username,
	})
	if err != nil {
		if binding.RecordID != "" {
			if auditErr := recordBindingProvisionAudit(ctx, resources, binding.RecordID, provider.ID, "", adminresource.AdminIdentityBindingAuditOutcomeConflict); auditErr != nil {
				return errors.New("record Admin OIDC binding conflict audit")
			}
		}
		return errors.New("Admin OIDC binding provisioning was rejected")
	}
	if err := recordBindingProvisionAudit(ctx, resources, binding.RecordID, provider.ID, binding.Username, adminresource.AdminIdentityBindingAuditOutcomeBound); err != nil {
		return errors.New("record Admin OIDC binding provisioning audit")
	}
	_, err = fmt.Fprintf(stdout, "provider=%s username=%s\n", provider.ID, binding.Username)
	return err
}

func runSensitiveDataMigration(ctx context.Context, args []string, stdout, stderr io.Writer, loadConfig func() config.Config, openSensitiveMigration sensitiveMigrationOpener) error {
	_ = stderr
	flags := flag.NewFlagSet(sensitiveDataMigrationCommand, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	modeValue := flags.String("mode", "", "migration mode")
	runID := flags.String("run-id", "", "immutable migration run ID")
	actor := flags.String("actor", "", "operator actor ID")
	reason := flags.String("reason", "", "approved migration reason")
	approvalRef := flags.String("approval-ref", "", "approval reference")
	backupURI := flags.String("backup-uri", "", "external backup URI")
	backupHash := flags.String("backup-sha256", "", "external backup SHA-256")
	restoreEvidence := flags.String("restore-evidence-ref", "", "restore rehearsal evidence reference")
	maintenanceConfirmed := flags.Bool("maintenance-window-confirmed", false, "confirm maintenance window")
	batchSize := flags.Int("batch-size", sensitivemigration.DefaultBatchSize, "migration batch size")
	if err := flags.Parse(args[1:]); err != nil {
		return errors.New("invalid sensitive-data-migrate arguments")
	}
	if len(flags.Args()) != 0 {
		return errors.New("positional arguments are not accepted")
	}
	mode := sensitivemigration.Mode(*modeValue)
	if !validSensitiveMigrationMode(mode) {
		return errors.New("invalid sensitive-data-migrate mode")
	}
	if *batchSize < 1 || *batchSize > sensitivemigration.MaximumBatchSize {
		return errors.New("invalid sensitive-data-migrate batch size")
	}
	if loadConfig == nil || openSensitiveMigration == nil {
		return errors.New("sensitive data migration bootstrap is unavailable")
	}

	session, err := openSensitiveMigration(loadConfig(), apps.DefaultManifests()...)
	if err != nil || session == nil {
		return errors.New("sensitive data migration bootstrap failed")
	}
	closed := false
	defer func() {
		if !closed {
			_ = session.Close()
		}
	}()

	request := sensitivemigration.RunRequest{
		RunID: *runID, PlanHash: session.PlanHash(), ActorID: *actor, Reason: *reason,
		ApprovalRef: *approvalRef, BackupURI: *backupURI, BackupHash: *backupHash,
		RestoreEvidence: *restoreEvidence, MaintenanceConfirmed: *maintenanceConfirmed,
	}
	switch mode {
	case sensitivemigration.ModeVerify:
		if !sensitivemigration.ValidRunIdentity(request) {
			return errors.New("sensitive data migration run identity is required")
		}
	case sensitivemigration.ModePrepare, sensitivemigration.ModeApply, sensitivemigration.ModeRehearseRestore, sensitivemigration.ModeRollback:
		if !sensitivemigration.ValidMutationRequest(request) {
			return errors.New("sensitive data migration approval evidence is required")
		}
	}
	report, err := session.Run(ctx, sensitivemigration.Options{Mode: mode, BatchSize: *batchSize, Request: request})
	if err != nil {
		return errors.New("sensitive data migration failed")
	}
	if err := json.NewEncoder(stdout).Encode(report); err != nil {
		return errors.New("write sensitive data migration report")
	}
	closeErr := session.Close()
	closed = true
	if closeErr != nil {
		return errors.New("close sensitive data migration storage")
	}
	return nil
}

func runOrganizationRBACMigration(ctx context.Context, args []string, stdout io.Writer, loadConfig func() config.Config, prepare organizationRBACPreparer, open organizationRBACMigrationOpener) error {
	flags := flag.NewFlagSet(organizationRBACMigrationCommand, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	modeValue := flags.String("mode", "", "prepare, inventory, verify, apply, or rollback")
	manifestPath := flags.String("manifest", "", "versioned migration manifest JSON")
	runID := flags.String("run-id", "", "immutable migration run ID")
	actor := flags.String("actor", "", "operator actor ID")
	reason := flags.String("reason", "", "approved migration reason")
	approvalRef := flags.String("approval-ref", "", "approval reference")
	backupURI := flags.String("backup-uri", "", "external backup URI")
	backupHash := flags.String("backup-sha256", "", "external backup SHA-256")
	checkpointRef := flags.String("checkpoint-ref", "", "reviewed database checkpoint or restore rehearsal reference")
	if err := flags.Parse(args[1:]); err != nil || len(flags.Args()) != 0 {
		return errors.New("invalid organization-rbac-migrate arguments")
	}
	if loadConfig == nil || prepare == nil || open == nil {
		return errors.New("organization rbac migration bootstrap is unavailable")
	}
	cfg := loadConfig()
	if *modeValue == "prepare" {
		if strings.TrimSpace(*manifestPath) != "" || strings.TrimSpace(*runID) != "" {
			return errors.New("organization rbac prepare does not accept manifest or run identity")
		}
		if err := prepare(ctx, cfg); err != nil {
			return errors.New("organization rbac migration prepare failed")
		}
		return json.NewEncoder(stdout).Encode(map[string]string{"mode": "prepare", "status": "prepared"})
	}
	mode := organizationrbac.MigrationMode(*modeValue)
	switch mode {
	case organizationrbac.MigrationInventory, organizationrbac.MigrationVerify, organizationrbac.MigrationApply, organizationrbac.MigrationRollback:
	default:
		return errors.New("invalid organization rbac migration mode")
	}
	if strings.TrimSpace(*manifestPath) == "" {
		return errors.New("organization rbac migration manifest is required")
	}
	manifestFile, err := os.Open(*manifestPath)
	if err != nil {
		return errors.New("open organization rbac migration manifest")
	}
	defer func() { _ = manifestFile.Close() }()
	var manifest organizationrbac.MigrationManifest
	decoder := json.NewDecoder(io.LimitReader(manifestFile, maximumLifecyclePolicyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return errors.New("decode organization rbac migration manifest")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return errors.New("decode organization rbac migration manifest")
	}
	session, err := open(ctx, cfg)
	if err != nil || session == nil {
		return errors.New("organization rbac migration bootstrap failed")
	}
	closed := false
	defer func() {
		if !closed {
			_ = session.Close()
		}
	}()
	evidence := organizationrbac.MigrationEvidence{
		RunID: *runID, ActorID: *actor, Reason: *reason, ApprovalRef: *approvalRef,
		BackupURI: *backupURI, BackupSHA256: *backupHash, CheckpointRef: *checkpointRef, AppliedAt: time.Now().UTC(),
	}
	report, err := session.RunMigration(ctx, mode, manifest, evidence)
	if err != nil {
		return errors.New("organization rbac migration failed")
	}
	if err := json.NewEncoder(stdout).Encode(report); err != nil {
		return errors.New("write organization rbac migration report")
	}
	if err := session.Close(); err != nil {
		return errors.New("close organization rbac migration storage")
	}
	closed = true
	return nil
}

type lifecycleOperationOutput struct {
	Operation string `json:"operation"`
	Status    string `json:"status"`
}

type lifecyclePromotionOutput struct {
	Operation           string    `json:"operation"`
	Status              string    `json:"status"`
	DatasourceID        string    `json:"datasourceId"`
	CurrentFingerprint  string    `json:"currentFingerprint"`
	PromotedFingerprint string    `json:"promotedFingerprint"`
	ImpactReportHash    string    `json:"impactReportHash"`
	PromotedAt          time.Time `json:"promotedAt"`
}

func runDataLifecycle(ctx context.Context, args []string, stdout, stderr io.Writer, loadConfig func() config.Config, prepareDataLifecycle dataLifecyclePreparer, openDataLifecycle dataLifecycleOpener) error {
	_ = stderr
	flags := flag.NewFlagSet(dataLifecycleCommand, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	operation := flags.String("operation", "", "maintenance operation")
	runID := flags.String("run-id", "", "immutable maintenance run ID")
	dryRunID := flags.String("dry-run-id", "", "completed dry-run ID approved for apply")
	owner := flags.String("owner", "", "maintenance lease owner ID")
	batchSize := flags.Int("batch-size", datalifecycle.DefaultBatchSize, "maintenance batch size")
	maxRetries := flags.Int("max-retries", 0, "bounded retry count")
	currentPolicyFile := flags.String("current-policy-file", "", "reviewed current policy JSON file")
	promotedFingerprint := flags.String("promoted-fingerprint", "", "exact proposed policy fingerprint")
	impactReportHash := flags.String("impact-report-hash", "", "immutable impact report hash")
	actor := flags.String("actor", "", "operator actor ID")
	reason := flags.String("reason", "", "approved promotion reason")
	approvalRef := flags.String("approval-ref", "", "approval reference")
	confirmApply := flags.Bool("confirm-apply", false, "confirm destructive apply")
	if err := flags.Parse(args[1:]); err != nil {
		return errors.New("invalid data-lifecycle arguments")
	}
	if len(flags.Args()) != 0 || !validDataLifecycleOperation(*operation) ||
		*batchSize < 1 || *batchSize > datalifecycle.MaximumBatchSize || *maxRetries < 0 || *maxRetries > datalifecycle.MaximumRetries ||
		!dataLifecycleFlagsAllowed(flags, *operation) {
		return errors.New("invalid data-lifecycle arguments")
	}
	if loadConfig == nil {
		return errors.New("data lifecycle bootstrap is unavailable")
	}

	if *operation == dataLifecyclePrepareOperation {
		if prepareDataLifecycle == nil {
			return errors.New("data lifecycle bootstrap is unavailable")
		}
		if err := prepareDataLifecycle(ctx, loadConfig()); err != nil {
			return errors.New("data lifecycle prepare failed")
		}
		return encodeDataLifecycleJSON(stdout, lifecycleOperationOutput{Operation: *operation, Status: datalifecycle.StatusCompleted})
	}
	if *operation == string(datalifecycle.ModeApply) && !*confirmApply {
		return errors.New("data lifecycle apply confirmation is required")
	}

	var currentPolicy *datalifecycle.PolicySnapshot
	var currentFingerprint string
	if *operation == dataLifecyclePromoteOperation || *operation == string(datalifecycle.ModeApply) {
		if !validLifecyclePromotionInputs(*currentPolicyFile, *promotedFingerprint, *impactReportHash, *actor, *reason, *approvalRef) {
			return errors.New("data lifecycle promotion evidence is required")
		}
		policy, err := readLifecyclePolicyFile(*currentPolicyFile)
		if err != nil {
			return errors.New("invalid data lifecycle current policy file")
		}
		currentPolicy = &policy
		currentFingerprint, err = datalifecycle.PolicyFingerprint(policy)
		if err != nil {
			return errors.New("invalid data lifecycle current policy file")
		}
	}
	if *operation != dataLifecyclePromoteOperation && (!validLifecycleIdentity(*runID) || !validLifecycleIdentity(*owner)) {
		return errors.New("data lifecycle run identity is required")
	}
	if *operation == string(datalifecycle.ModeApply) && !validLifecycleIdentity(*dryRunID) {
		return errors.New("data lifecycle dry-run identity is required")
	}
	if openDataLifecycle == nil {
		return errors.New("data lifecycle bootstrap is unavailable")
	}

	session, err := openDataLifecycle(loadConfig(), apps.DefaultManifests()...)
	if err != nil || session == nil {
		return errors.New("data lifecycle bootstrap failed")
	}
	closed := false
	defer func() {
		if !closed {
			_ = session.Close()
		}
	}()

	policy := session.Policy()
	fingerprint, err := datalifecycle.PolicyFingerprint(policy)
	if err != nil || fingerprint != session.PolicyFingerprint() {
		return errors.New("data lifecycle policy unavailable")
	}
	if currentPolicy != nil && *promotedFingerprint != fingerprint {
		return errors.New("data lifecycle promoted fingerprint mismatch")
	}

	var output any
	if *operation == dataLifecyclePromoteOperation {
		promotion, promoteErr := session.Promote(ctx, datalifecycle.PromotionRequest{
			Enabled: true, DatasourceID: datalifecycle.DefaultDatasourceID,
			CurrentPolicy: *currentPolicy, ProposedPolicy: policy,
			ImpactReportHash: *impactReportHash, ActorID: *actor, Reason: *reason,
			ApprovalRef: *approvalRef, PromotedFingerprint: *promotedFingerprint,
		})
		if promoteErr != nil {
			return errors.New("data lifecycle promotion failed")
		}
		output = lifecyclePromotionOutput{
			Operation: *operation, Status: datalifecycle.StatusCompleted,
			DatasourceID: promotion.DatasourceID, CurrentFingerprint: promotion.CurrentFingerprint,
			PromotedFingerprint: promotion.PromotedFingerprint, ImpactReportHash: promotion.ImpactReportHash,
			PromotedAt: promotion.PromotedAt,
		}
	} else {
		options := datalifecycle.Options{
			Enabled: true, Mode: datalifecycle.Mode(*operation), RunID: *runID, OwnerID: *owner,
			DatasourceID: datalifecycle.DefaultDatasourceID, BatchSize: *batchSize, MaxRetries: *maxRetries,
			Policy: policy, PolicyFingerprint: fingerprint,
		}
		if *operation == string(datalifecycle.ModeApply) {
			options.Promotion = datalifecycle.PromotionApproval{
				ImpactReportHash: *impactReportHash, ActorID: *actor, Reason: *reason,
				ApprovalRef: *approvalRef, PromotedFingerprint: *promotedFingerprint,
				CurrentFingerprint: currentFingerprint, DryRunID: *dryRunID,
			}
		}
		report, runErr := session.Run(ctx, options)
		if runErr != nil {
			return errors.New("data lifecycle operation failed")
		}
		output = report
	}
	if err := session.Close(); err != nil {
		closed = true
		return errors.New("close data lifecycle storage")
	}
	closed = true
	return encodeDataLifecycleJSON(stdout, output)
}

func validDataLifecycleOperation(operation string) bool {
	switch operation {
	case dataLifecyclePrepareOperation, dataLifecyclePromoteOperation,
		string(datalifecycle.ModeImpact), string(datalifecycle.ModeDryRun), string(datalifecycle.ModeApply):
		return true
	default:
		return false
	}
}

func dataLifecycleFlagsAllowed(flags *flag.FlagSet, operation string) bool {
	allowed := map[string]struct{}{"operation": {}}
	switch operation {
	case dataLifecyclePrepareOperation:
	case string(datalifecycle.ModeImpact), string(datalifecycle.ModeDryRun):
		for _, name := range []string{"run-id", "owner", "batch-size", "max-retries"} {
			allowed[name] = struct{}{}
		}
	case dataLifecyclePromoteOperation:
		for _, name := range []string{"current-policy-file", "promoted-fingerprint", "impact-report-hash", "actor", "reason", "approval-ref"} {
			allowed[name] = struct{}{}
		}
	case string(datalifecycle.ModeApply):
		for _, name := range []string{
			"run-id", "dry-run-id", "owner", "batch-size", "max-retries", "current-policy-file", "promoted-fingerprint",
			"impact-report-hash", "actor", "reason", "approval-ref", "confirm-apply",
		} {
			allowed[name] = struct{}{}
		}
	}
	valid := true
	flags.Visit(func(value *flag.Flag) {
		if _, ok := allowed[value.Name]; !ok {
			valid = false
		}
	})
	return valid
}

func validLifecyclePromotionInputs(policyFile, fingerprint, impactHash, actor, reason, approvalRef string) bool {
	return validLifecycleText(policyFile) && validLifecycleDigest(fingerprint) && validLifecycleDigest(impactHash) &&
		validLifecycleIdentity(actor) && validLifecycleEvidenceText(reason) && validLifecycleEvidenceText(approvalRef)
}

func validLifecycleIdentity(value string) bool {
	if value == "" || len(value) > 128 || value != strings.TrimSpace(value) {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' || character == '-' || character == '_' || character == '.' {
			continue
		}
		return false
	}
	return true
}

func validLifecycleText(value string) bool {
	return value != "" && value == strings.TrimSpace(value)
}

func validLifecycleEvidenceText(value string) bool {
	return validLifecycleText(value) && len(value) <= 191
}

func validLifecycleDigest(value string) bool {
	if len(value) != 71 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	for _, character := range value[len("sha256:"):] {
		if character < '0' || character > '9' && character < 'a' || character > 'f' {
			return false
		}
	}
	return true
}

func readLifecyclePolicyFile(path string) (datalifecycle.PolicySnapshot, error) {
	file, err := os.Open(path)
	if err != nil {
		return datalifecycle.PolicySnapshot{}, err
	}
	defer func() { _ = file.Close() }()
	payload, err := io.ReadAll(io.LimitReader(file, maximumLifecyclePolicyBytes+1))
	if err != nil || len(payload) > maximumLifecyclePolicyBytes {
		return datalifecycle.PolicySnapshot{}, errors.New("invalid policy file")
	}
	if err := rejectDuplicateLifecycleJSONKeys(payload); err != nil {
		return datalifecycle.PolicySnapshot{}, err
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(payload, &document); err != nil || !exactLifecycleJSONKeys(document, "version", "resources") {
		return datalifecycle.PolicySnapshot{}, errors.New("invalid policy document")
	}
	var version uint32
	var resources []json.RawMessage
	if err := json.Unmarshal(document["version"], &version); err != nil {
		return datalifecycle.PolicySnapshot{}, errors.New("invalid policy version")
	}
	if err := json.Unmarshal(document["resources"], &resources); err != nil {
		return datalifecycle.PolicySnapshot{}, errors.New("invalid policy resources")
	}
	policy := datalifecycle.PolicySnapshot{Version: version, Resources: make([]datalifecycle.ResourcePolicy, 0, len(resources))}
	for _, rawResource := range resources {
		var resource map[string]json.RawMessage
		if err := json.Unmarshal(rawResource, &resource); err != nil ||
			!exactLifecycleJSONKeys(resource, "resource", "mode", "policyVersion", "retentionDays", "autoPurge") {
			return datalifecycle.PolicySnapshot{}, errors.New("invalid policy resource")
		}
		var item datalifecycle.ResourcePolicy
		if err := json.Unmarshal(resource["resource"], &item.Resource); err != nil {
			return datalifecycle.PolicySnapshot{}, errors.New("invalid policy resource name")
		}
		if err := json.Unmarshal(resource["mode"], &item.Mode); err != nil {
			return datalifecycle.PolicySnapshot{}, errors.New("invalid policy resource deletion mode")
		}
		if err := json.Unmarshal(resource["policyVersion"], &item.PolicyVersion); err != nil {
			return datalifecycle.PolicySnapshot{}, errors.New("invalid resource policy version")
		}
		if err := json.Unmarshal(resource["retentionDays"], &item.RetentionDays); err != nil {
			return datalifecycle.PolicySnapshot{}, errors.New("invalid resource retention")
		}
		if err := json.Unmarshal(resource["autoPurge"], &item.AutoPurge); err != nil {
			return datalifecycle.PolicySnapshot{}, errors.New("invalid resource purge policy")
		}
		policy.Resources = append(policy.Resources, datalifecycle.ResourcePolicy{
			Resource: item.Resource, Mode: item.Mode, PolicyVersion: item.PolicyVersion,
			RetentionDays: item.RetentionDays, AutoPurge: item.AutoPurge,
		})
	}
	if _, err := datalifecycle.PolicyFingerprint(policy); err != nil {
		return datalifecycle.PolicySnapshot{}, err
	}
	return policy, nil
}

func exactLifecycleJSONKeys(document map[string]json.RawMessage, expected ...string) bool {
	if len(document) != len(expected) {
		return false
	}
	for _, key := range expected {
		if _, ok := document[key]; !ok {
			return false
		}
	}
	return true
}

func rejectDuplicateLifecycleJSONKeys(payload []byte) error {
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	var readValue func() error
	readValue = func() error {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delimiter, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		switch delimiter {
		case '{':
			seen := map[string]struct{}{}
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return err
				}
				key, ok := keyToken.(string)
				if !ok {
					return errors.New("invalid object key")
				}
				if _, duplicate := seen[key]; duplicate {
					return errors.New("duplicate object key")
				}
				seen[key] = struct{}{}
				if err := readValue(); err != nil {
					return err
				}
			}
			closing, err := decoder.Token()
			if err != nil || closing != json.Delim('}') {
				return errors.New("invalid object")
			}
		case '[':
			for decoder.More() {
				if err := readValue(); err != nil {
					return err
				}
			}
			closing, err := decoder.Token()
			if err != nil || closing != json.Delim(']') {
				return errors.New("invalid array")
			}
		default:
			return errors.New("invalid JSON delimiter")
		}
		return nil
	}
	if err := readValue(); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return errors.New("invalid trailing JSON content")
	}
	return nil
}

func encodeDataLifecycleJSON(stdout io.Writer, value any) error {
	if err := json.NewEncoder(stdout).Encode(value); err != nil {
		return errors.New("write data lifecycle report")
	}
	return nil
}

func validSensitiveMigrationMode(mode sensitivemigration.Mode) bool {
	switch mode {
	case sensitivemigration.ModeInventory, sensitivemigration.ModeDryRun, sensitivemigration.ModePrepare,
		sensitivemigration.ModeApply, sensitivemigration.ModeVerify, sensitivemigration.ModeRehearseRestore,
		sensitivemigration.ModeRollback:
		return true
	default:
		return false
	}
}

func findAdminOIDCProvider(manifests []capability.Manifest, providerID string) (capability.AuthProvider, bool) {
	providerID = strings.TrimSpace(providerID)
	for _, manifest := range manifests {
		for _, provider := range manifest.AuthProviders {
			if provider.ID == providerID && provider.Kind == "oidc" && provider.Enabled && provider.Configured && provider.SupportsAudience(capability.AuthProviderAudienceAdmin) {
				return provider, true
			}
		}
	}
	return capability.AuthProvider{}, false
}

func recordBindingProvisionAudit(ctx context.Context, resources *adminresource.Store, bindingRecordID string, providerID string, username string, outcome string) error {
	_, err := resources.EnsureAdminIdentityBindingAudit(ctx, adminresource.AdminIdentityBindingAuditInput{
		BindingRecordID: bindingRecordID,
		Provider:        providerID,
		Username:        username,
		Outcome:         outcome,
		Now:             time.Now(),
	})
	if errors.Is(err, adminresource.ErrUnknownResource) {
		return nil
	}
	return err
}
