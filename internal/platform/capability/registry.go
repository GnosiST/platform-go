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
