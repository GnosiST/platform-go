package sensitivemigration

import (
	"fmt"
	"slices"
	"strings"

	"platform-go/internal/platform/capability"
	"platform-go/internal/platform/dataprotection"
)

type Plan struct {
	Resources []ResourcePlan
}

type ResourcePlan struct {
	Resource      string
	Scope         string
	TenantField   string
	SchemaVersion uint32
	Fields        []FieldPlan
}

type FieldPlan struct {
	Key    string
	Policy dataprotection.FieldPolicy
}

func PlanFromManifests(manifests []capability.Manifest) (Plan, error) {
	if err := capability.ValidateAdminSurface(manifests); err != nil {
		return Plan{}, err
	}

	plan := Plan{}
	seenResources := map[string]struct{}{}
	for _, manifest := range manifests {
		for _, resource := range manifest.Admin.Resources {
			fields := make([]FieldPlan, 0)
			for _, field := range resource.Fields {
				if strings.TrimSpace(field.StorageMode) != capability.FieldStorageEncrypted {
					continue
				}
				fields = append(fields, FieldPlan{
					Key: strings.TrimSpace(field.Key),
					Policy: dataprotection.FieldPolicy{
						Format:              strings.TrimSpace(field.Protection.Format),
						Normalization:       strings.TrimSpace(field.Protection.Normalization),
						BlindIndexNamespace: strings.TrimSpace(field.Protection.BlindIndexNamespace),
					},
				})
			}
			if len(fields) == 0 {
				continue
			}
			resourceKey := strings.TrimSpace(resource.Resource)
			if _, exists := seenResources[resourceKey]; exists {
				return Plan{}, fmt.Errorf("sensitive migration plan duplicate resource %q", resourceKey)
			}
			seenResources[resourceKey] = struct{}{}

			slices.SortFunc(fields, func(left FieldPlan, right FieldPlan) int {
				return strings.Compare(left.Key, right.Key)
			})
			plan.Resources = append(plan.Resources, ResourcePlan{
				Resource:      resourceKey,
				Scope:         strings.TrimSpace(resource.Protection.Scope),
				TenantField:   strings.TrimSpace(resource.Protection.TenantField),
				SchemaVersion: resource.Protection.SchemaVersion,
				Fields:        fields,
			})
		}
	}

	if len(plan.Resources) == 0 {
		return Plan{}, fmt.Errorf("sensitive migration plan has no encrypted fields")
	}
	slices.SortFunc(plan.Resources, func(left ResourcePlan, right ResourcePlan) int {
		return strings.Compare(left.Resource, right.Resource)
	})
	return plan, nil
}
