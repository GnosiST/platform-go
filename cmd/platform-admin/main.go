package main

import (
	"context"
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
	"platform-go/internal/platform/dataprotection"
	"platform-go/internal/platform/httpapi"
)

const bindAdminOIDCCommand = "bind-admin-oidc"

type adminResourcesLoader func(config.Config, []capability.Manifest, dataprotection.Runtime) (*adminresource.Store, error)

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdin, os.Stdout, os.Stderr, config.Load); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, loadConfig func() config.Config) error {
	return runWithAdminResources(ctx, args, stdin, stdout, stderr, loadConfig, bootstrap.AdminResourcesFromConfig)
}

func runWithAdminResources(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, loadConfig func() config.Config, loadAdminResources adminResourcesLoader) error {
	if len(args) == 0 || args[0] != bindAdminOIDCCommand {
		return errors.New("expected bind-admin-oidc command")
	}

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
