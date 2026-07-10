package capability

import (
	"fmt"
	"strings"
)

func ValidateLifecycleDeclarations(manifests []Manifest) error {
	migrations := map[string]ID{}
	seeds := map[string]ID{}
	steps := map[string]struct {
		owner ID
		kind  string
	}{}
	for _, manifest := range manifests {
		for _, migration := range manifest.Migrations {
			if err := validateMigration(manifest.ID, migration); err != nil {
				return err
			}
			migrationID := strings.TrimSpace(migration.ID)
			if owner, exists := migrations[migrationID]; exists {
				return fmt.Errorf("capability %q migration %q already registered by capability %q", manifest.ID, migrationID, owner)
			}
			if previous, exists := steps[migrationID]; exists && previous.kind != "migration" {
				return fmt.Errorf("capability %q lifecycle step %q is declared as both %s and migration", manifest.ID, migrationID, previous.kind)
			}
			migrations[migrationID] = manifest.ID
			steps[migrationID] = struct {
				owner ID
				kind  string
			}{owner: manifest.ID, kind: "migration"}
		}
		for _, seed := range manifest.Seeds {
			if err := validateSeed(manifest.ID, seed); err != nil {
				return err
			}
			seedID := strings.TrimSpace(seed.ID)
			if owner, exists := seeds[seedID]; exists {
				return fmt.Errorf("capability %q seed %q already registered by capability %q", manifest.ID, seedID, owner)
			}
			if previous, exists := steps[seedID]; exists && previous.kind != "seed" {
				return fmt.Errorf("capability %q lifecycle step %q is declared as both %s and seed", manifest.ID, seedID, previous.kind)
			}
			seeds[seedID] = manifest.ID
			steps[seedID] = struct {
				owner ID
				kind  string
			}{owner: manifest.ID, kind: "seed"}
		}
	}
	return nil
}

func validateMigration(owner ID, migration Migration) error {
	migrationID := strings.TrimSpace(migration.ID)
	switch {
	case migrationID == "":
		return fmt.Errorf("capability %q migration id is required", owner)
	case !validLifecycleStepID(migrationID):
		return fmt.Errorf("capability %q migration id must use lowercase letters, numbers or hyphens", owner)
	case strings.TrimSpace(migration.Description) == "":
		return fmt.Errorf("capability %q migration %q description is required", owner, migration.ID)
	case migration.Up == nil:
		return fmt.Errorf("capability %q migration %q up function is required", owner, migration.ID)
	}
	return nil
}

func validateSeed(owner ID, seed Seed) error {
	seedID := strings.TrimSpace(seed.ID)
	switch {
	case seedID == "":
		return fmt.Errorf("capability %q seed id is required", owner)
	case !validLifecycleStepID(seedID):
		return fmt.Errorf("capability %q seed id must use lowercase letters, numbers or hyphens", owner)
	case strings.TrimSpace(seed.Description) == "":
		return fmt.Errorf("capability %q seed %q description is required", owner, seed.ID)
	case seed.Run == nil:
		return fmt.Errorf("capability %q seed %q run function is required", owner, seed.ID)
	}
	return nil
}

func validLifecycleStepID(id string) bool {
	if id == "" {
		return false
	}
	for _, char := range id {
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
