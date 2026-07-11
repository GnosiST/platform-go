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
	"platform-go/internal/platform/httpapi"
)

const bindAdminOIDCCommand = "bind-admin-oidc"

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdin, os.Stdout, os.Stderr, config.Load); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, loadConfig func() config.Config) error {
	if len(args) == 0 || args[0] != bindAdminOIDCCommand {
		return errors.New("expected bind-admin-oidc command")
	}

	flags := flag.NewFlagSet(bindAdminOIDCCommand, flag.ContinueOnError)
	flags.SetOutput(stderr)
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

	resources, err := bootstrap.AdminResourcesFromConfig(cfg, manifests)
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
		return errors.New("Admin OIDC binding provisioning was rejected")
	}
	if err := recordBindingProvisionAudit(resources, provider.ID, binding.Username); err != nil {
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

func recordBindingProvisionAudit(resources *adminresource.Store, providerID string, username string) error {
	values := map[string]string{
		"action":    "admin_identity.bind",
		"resource":  "admin-identities",
		"provider":  strings.TrimSpace(providerID),
		"outcome":   "bound",
		"createdAt": time.Now().UTC().Format(time.RFC3339),
	}
	if strings.TrimSpace(username) != "" {
		values["actor"] = strings.TrimSpace(username)
	}
	_, err := resources.Create("audit-logs", adminresource.WriteInput{
		Code:        "admin_identity.bind",
		Name:        "Admin OIDC Binding Provisioned",
		Status:      "recorded",
		Description: "Admin OIDC identity binding provisioning event.",
		Values:      values,
	})
	if errors.Is(err, adminresource.ErrUnknownResource) {
		return nil
	}
	return err
}
