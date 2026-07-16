package sensitivemigration

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/GnosiST/platform-go/internal/platform/capability"
	"github.com/GnosiST/platform-go/internal/platform/dataprotection"
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

type canonicalPlanResource struct {
	Resource      string               `json:"resource"`
	Scope         string               `json:"scope"`
	TenantField   string               `json:"tenantField"`
	SchemaVersion uint32               `json:"schemaVersion"`
	Fields        []canonicalPlanField `json:"fields"`
}

type canonicalPlanField struct {
	Key                 string `json:"key"`
	Format              string `json:"format"`
	Normalization       string `json:"normalization"`
	BlindIndexNamespace string `json:"blindIndexNamespace"`
}

func PlanHash(plan Plan) string {
	resources := make([]canonicalPlanResource, 0, len(plan.Resources))
	for _, resource := range plan.Resources {
		fields := make([]canonicalPlanField, 0, len(resource.Fields))
		for _, field := range resource.Fields {
			fields = append(fields, canonicalPlanField{
				Key: field.Key, Format: field.Policy.Format, Normalization: field.Policy.Normalization,
				BlindIndexNamespace: field.Policy.BlindIndexNamespace,
			})
		}
		slices.SortFunc(fields, func(left canonicalPlanField, right canonicalPlanField) int {
			return strings.Compare(left.Key, right.Key)
		})
		resources = append(resources, canonicalPlanResource{
			Resource: resource.Resource, Scope: resource.Scope, TenantField: resource.TenantField,
			SchemaVersion: resource.SchemaVersion, Fields: fields,
		})
	}
	slices.SortFunc(resources, func(left canonicalPlanResource, right canonicalPlanResource) int {
		return strings.Compare(left.Resource, right.Resource)
	})
	payload, _ := json.Marshal(resources)
	digest := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(digest[:])
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
