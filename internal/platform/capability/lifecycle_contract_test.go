package capability

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

func TestValidateLifecycleRejectsDuplicateMigrationIDs(t *testing.T) {
	manifests := []Manifest{
		{ID: "a", Migrations: []Migration{{ID: "20260704-create-table", Description: "create table", Up: noopLifecycleStep}}},
		{ID: "b", Migrations: []Migration{{ID: " 20260704-create-table ", Description: "create another table", Up: noopLifecycleStep}}},
	}

	err := ValidateLifecycleDeclarations(manifests)
	if err == nil {
		t.Fatalf("ValidateLifecycleDeclarations() error = nil, want duplicate migration")
	}
	if !strings.Contains(err.Error(), `migration "20260704-create-table" already registered`) {
		t.Fatalf("ValidateLifecycleDeclarations() error = %v, want duplicate migration", err)
	}
}

func TestValidateLifecycleRejectsInvalidStepIDs(t *testing.T) {
	manifests := []Manifest{
		{ID: "a", Migrations: []Migration{{ID: "20260704_create_table", Description: "create table", Up: noopLifecycleStep}}},
	}

	err := ValidateLifecycleDeclarations(manifests)
	if err == nil {
		t.Fatalf("ValidateLifecycleDeclarations() error = nil, want invalid step id")
	}
	if !strings.Contains(err.Error(), "migration id must use lowercase letters, numbers or hyphens") {
		t.Fatalf("ValidateLifecycleDeclarations() error = %v, want step id format error", err)
	}
}

func TestValidateLifecycleRejectsIncompleteSeed(t *testing.T) {
	manifests := []Manifest{
		{ID: "demo-seed", Seeds: []Seed{{ID: "demo-users"}}},
	}

	err := ValidateLifecycleDeclarations(manifests)
	if err == nil {
		t.Fatalf("ValidateLifecycleDeclarations() error = nil, want incomplete seed")
	}
	if !strings.Contains(err.Error(), "description is required") {
		t.Fatalf("ValidateLifecycleDeclarations() error = %v, want description required", err)
	}
}

func TestValidateLifecycleRejectsMigrationAndSeedSharingStepID(t *testing.T) {
	manifests := []Manifest{
		{
			ID:         "dictionary",
			Migrations: []Migration{{ID: "dictionary-defaults", Description: "create dictionary records", Up: noopLifecycleStep}},
			Seeds:      []Seed{{ID: "dictionary-defaults", Description: "seed dictionary records", Run: noopLifecycleStep}},
		},
	}

	err := ValidateLifecycleDeclarations(manifests)
	if err == nil {
		t.Fatalf("ValidateLifecycleDeclarations() error = nil, want migration/seed id conflict")
	}
	if !strings.Contains(err.Error(), `lifecycle step "dictionary-defaults" is declared as both migration and seed`) {
		t.Fatalf("ValidateLifecycleDeclarations() error = %v, want migration/seed id conflict", err)
	}
}

func TestRunLifecycleExecutesMigrationsAndSeedsInDependencyOrder(t *testing.T) {
	registry := NewRegistry()
	var calls []string
	identity := testManifest("identity")
	identity.Migrations = []Migration{
		{ID: "001-identity", Description: "create identity tables", Up: appendCall(&calls, "identity.migration")},
	}
	identity.Seeds = []Seed{
		{ID: "identity-default-users", Description: "seed default users", Run: appendCall(&calls, "identity.seed")},
	}
	mustRegister(t, registry, identity)
	wechatLogin := testManifest("wechat-login", ID("identity"))
	wechatLogin.Migrations = []Migration{
		{ID: "002-wechat", Description: "create wechat login tables", Up: appendCall(&calls, "wechat.migration")},
	}
	wechatLogin.Seeds = []Seed{
		{ID: "wechat-default-provider", Description: "seed wechat provider", Run: appendCall(&calls, "wechat.seed")},
	}
	mustRegister(t, registry, wechatLogin)

	err := registry.RunLifecycle(context.Background(), []ID{"wechat-login", "identity"}, Runtime{})
	if err != nil {
		t.Fatalf("RunLifecycle() error = %v", err)
	}
	want := []string{"identity.migration", "wechat.migration", "identity.seed", "wechat.seed"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
}

func TestRunLifecycleWrapsMigrationFailureWithCapabilityID(t *testing.T) {
	registry := NewRegistry()
	files := testManifest("files")
	files.Migrations = []Migration{
		{
			ID:          "001-files",
			Description: "create file tables",
			Up: func(context.Context, Runtime) error {
				return errLifecycleTestFailure
			},
		},
	}
	mustRegister(t, registry, files)

	err := registry.RunLifecycle(context.Background(), []ID{"files"}, Runtime{})
	if err == nil {
		t.Fatalf("RunLifecycle() error = nil, want migration failure")
	}
	if !strings.Contains(err.Error(), `capability "files" migration "001-files" failed`) {
		t.Fatalf("RunLifecycle() error = %v, want wrapped migration failure", err)
	}
}

var errLifecycleTestFailure = lifecycleTestError{}

type lifecycleTestError struct{}

func (lifecycleTestError) Error() string {
	return "lifecycle test failure"
}

func appendCall(calls *[]string, value string) LifecycleStep {
	return func(context.Context, Runtime) error {
		*calls = append(*calls, value)
		return nil
	}
}

func noopLifecycleStep(context.Context, Runtime) error {
	return nil
}
