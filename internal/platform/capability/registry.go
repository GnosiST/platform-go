package capability

import (
	"context"
	"fmt"
	"strings"
)

type Registry struct {
	manifests map[ID]Manifest
}

func NewRegistry() *Registry {
	return &Registry{manifests: map[ID]Manifest{}}
}

func (r *Registry) Register(manifest Manifest) error {
	id := strings.TrimSpace(string(manifest.ID))
	if id == "" {
		return fmt.Errorf("capability id is required")
	}
	if !validCapabilityIdentifier(id) {
		return fmt.Errorf("capability %q id must use lowercase letters, numbers, and hyphens", manifest.ID)
	}
	manifest.ID = ID(id)
	version := strings.TrimSpace(manifest.Version)
	if version == "" {
		return fmt.Errorf("capability %q version is required", manifest.ID)
	}
	if !validCapabilityVersion(version) {
		return fmt.Errorf("capability %q version must use numeric semver", manifest.ID)
	}
	manifest.Version = version
	if err := normalizeCapabilityDependencies(&manifest); err != nil {
		return err
	}
	if _, exists := r.manifests[manifest.ID]; exists {
		return fmt.Errorf("capability %q already registered", manifest.ID)
	}
	r.manifests[manifest.ID] = manifest
	return nil
}

func normalizeCapabilityDependencies(manifest *Manifest) error {
	seen := map[ID]struct{}{}
	for index, dependency := range manifest.Dependencies {
		normalized := strings.TrimSpace(string(dependency))
		if normalized == "" {
			return fmt.Errorf("capability %q dependency id is required", manifest.ID)
		}
		if !validCapabilityIdentifier(normalized) {
			return fmt.Errorf("capability %q dependency %q must use lowercase letters, numbers, and hyphens", manifest.ID, dependency)
		}
		dependencyID := ID(normalized)
		if dependencyID == manifest.ID {
			return fmt.Errorf("capability %q cannot depend on itself", manifest.ID)
		}
		if _, exists := seen[dependencyID]; exists {
			return fmt.Errorf("capability %q duplicate dependency %q", manifest.ID, dependencyID)
		}
		seen[dependencyID] = struct{}{}
		manifest.Dependencies[index] = dependencyID
	}
	return nil
}

func validCapabilityIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if char >= 'a' && char <= 'z' {
			continue
		}
		if char >= '0' && char <= '9' {
			continue
		}
		if char == '-' {
			continue
		}
		return false
	}
	return true
}

func validCapabilityVersion(value string) bool {
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return false
	}
	for _, part := range parts {
		if part == "" {
			return false
		}
		for _, char := range part {
			if char < '0' || char > '9' {
				return false
			}
		}
	}
	return true
}

func (r *Registry) ResolveEnabled(enabled []ID) ([]Manifest, error) {
	normalizedEnabled, err := normalizeEnabledCapabilities(enabled)
	if err != nil {
		return nil, err
	}
	enabledSet := map[ID]bool{}
	for _, id := range normalizedEnabled {
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

	for _, id := range normalizedEnabled {
		if err := visit(id); err != nil {
			return nil, err
		}
	}
	if err := ValidateAdminSurface(ordered); err != nil {
		return nil, err
	}
	if err := ValidateAppSurface(ordered); err != nil {
		return nil, err
	}
	if err := ValidateLifecycleDeclarations(ordered); err != nil {
		return nil, err
	}
	if err := ValidateAuthProviderDeclarations(ordered); err != nil {
		return nil, err
	}
	if err := ValidateDemoDataDeclarations(ordered); err != nil {
		return nil, err
	}
	return ordered, nil
}

func normalizeEnabledCapabilities(enabled []ID) ([]ID, error) {
	if len(enabled) == 0 {
		return nil, fmt.Errorf("enabled capabilities must not be empty")
	}
	normalized := make([]ID, 0, len(enabled))
	seen := map[ID]struct{}{}
	for _, id := range enabled {
		value := strings.TrimSpace(string(id))
		if value == "" {
			return nil, fmt.Errorf("enabled capability id is required")
		}
		if !validCapabilityIdentifier(value) {
			return nil, fmt.Errorf("enabled capability %q must use lowercase letters, numbers, and hyphens", id)
		}
		capabilityID := ID(value)
		if _, exists := seen[capabilityID]; exists {
			return nil, fmt.Errorf("duplicate enabled capability %q", capabilityID)
		}
		seen[capabilityID] = struct{}{}
		normalized = append(normalized, capabilityID)
	}
	return normalized, nil
}

func (r *Registry) RunLifecycle(ctx context.Context, enabled []ID, runtime Runtime) error {
	ordered, err := r.ResolveEnabled(enabled)
	if err != nil {
		return err
	}
	return RunLifecycle(ctx, ordered, runtime)
}

func RunLifecycle(ctx context.Context, ordered []Manifest, runtime Runtime) error {
	phases := []func(Hooks) Hook{
		func(h Hooks) Hook { return h.Configure },
		nil,
		nil,
		func(h Hooks) Hook { return h.RegisterServices },
		func(h Hooks) Hook { return h.RegisterRoutes },
		func(h Hooks) Hook { return h.RegisterAdmin },
		func(h Hooks) Hook { return h.Start },
	}
	for index, phase := range phases {
		for _, manifest := range ordered {
			if index == 1 {
				if err := runManifestMigrations(ctx, runtime, manifest); err != nil {
					return err
				}
				if manifest.Hooks.Migrate != nil {
					if err := manifest.Hooks.Migrate(ctx, runtime); err != nil {
						return fmt.Errorf("capability %q lifecycle hook failed: %w", manifest.ID, err)
					}
				}
				continue
			}
			if index == 2 {
				if err := runManifestSeeds(ctx, runtime, manifest); err != nil {
					return err
				}
				if manifest.Hooks.Seed != nil {
					if err := manifest.Hooks.Seed(ctx, runtime); err != nil {
						return fmt.Errorf("capability %q lifecycle hook failed: %w", manifest.ID, err)
					}
				}
				continue
			}
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

func runManifestMigrations(ctx context.Context, runtime Runtime, manifest Manifest) error {
	for _, migration := range manifest.Migrations {
		if err := runtime.RunMigration(ctx, MigrationExecution{CapabilityID: manifest.ID, Migration: migration}); err != nil {
			return fmt.Errorf("capability %q migration %q failed: %w", manifest.ID, migration.ID, err)
		}
	}
	return nil
}

func runManifestSeeds(ctx context.Context, runtime Runtime, manifest Manifest) error {
	for _, seed := range manifest.Seeds {
		if err := runtime.RunSeed(ctx, SeedExecution{CapabilityID: manifest.ID, Seed: seed}); err != nil {
			return fmt.Errorf("capability %q seed %q failed: %w", manifest.ID, seed.ID, err)
		}
	}
	return nil
}
