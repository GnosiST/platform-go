package capability

import (
	"fmt"
	"strings"
)

func ValidateDemoDataDeclarations(manifests []Manifest) error {
	datasets := map[string]ID{}
	resources := demoDataResources(manifests)
	for _, manifest := range manifests {
		for _, dataset := range manifest.DemoData {
			if err := validateDemoDataSet(manifest.ID, dataset, resources); err != nil {
				return err
			}
			datasetID := strings.TrimSpace(dataset.ID)
			if owner, exists := datasets[datasetID]; exists {
				return fmt.Errorf("capability %q demo data %q already registered by capability %q", manifest.ID, datasetID, owner)
			}
			datasets[datasetID] = manifest.ID
		}
	}
	return nil
}

func demoDataResources(manifests []Manifest) map[string]ID {
	resources := map[string]ID{}
	for _, manifest := range manifests {
		for _, resource := range manifest.Admin.Resources {
			key := strings.TrimSpace(resource.Resource)
			if key != "" {
				resources[key] = manifest.ID
			}
		}
	}
	return resources
}

func validateDemoDataSet(owner ID, dataset DemoDataSet, resources map[string]ID) error {
	datasetID := strings.TrimSpace(dataset.ID)
	switch {
	case datasetID == "":
		return fmt.Errorf("capability %q demo data id is required", owner)
	case !validDemoDataIdentifier(datasetID):
		return fmt.Errorf("capability %q demo data id must use lowercase letters, numbers or hyphens", owner)
	case strings.TrimSpace(dataset.Title.ZH) == "" || strings.TrimSpace(dataset.Title.EN) == "":
		return fmt.Errorf("capability %q demo data %q title is required", owner, dataset.ID)
	case strings.TrimSpace(dataset.Description.ZH) == "" || strings.TrimSpace(dataset.Description.EN) == "":
		return fmt.Errorf("capability %q demo data %q description is required", owner, dataset.ID)
	case strings.TrimSpace(dataset.Resource) == "":
		return fmt.Errorf("capability %q demo data %q resource is required", owner, dataset.ID)
	case len(dataset.Records) == 0:
		return fmt.Errorf("capability %q demo data %q records are required", owner, dataset.ID)
	}
	resource := strings.TrimSpace(dataset.Resource)
	if _, exists := resources[resource]; !exists {
		return fmt.Errorf("capability %q demo data %q resource %q is not enabled", owner, dataset.ID, resource)
	}
	recordIDs := map[string]struct{}{}
	recordCodes := map[string]struct{}{}
	for _, record := range dataset.Records {
		id := strings.TrimSpace(record.ID)
		code := strings.TrimSpace(record.Code)
		if id == "" {
			return fmt.Errorf("capability %q demo data %q record id is required", owner, dataset.ID)
		}
		if _, exists := recordIDs[id]; exists {
			return fmt.Errorf("capability %q demo data %q record id %q is duplicated", owner, dataset.ID, id)
		}
		recordIDs[id] = struct{}{}
		if code == "" {
			return fmt.Errorf("capability %q demo data %q record %q code is required", owner, dataset.ID, record.ID)
		}
		if _, exists := recordCodes[code]; exists {
			return fmt.Errorf("capability %q demo data %q record code %q is duplicated", owner, dataset.ID, code)
		}
		recordCodes[code] = struct{}{}
		if strings.TrimSpace(record.Name) == "" {
			return fmt.Errorf("capability %q demo data %q record %q name is required", owner, dataset.ID, record.ID)
		}
	}
	return nil
}

func validDemoDataIdentifier(value string) bool {
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
