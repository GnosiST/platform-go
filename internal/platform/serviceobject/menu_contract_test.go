package serviceobject

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"platform-go/internal/platform/kernel"
)

func TestRegistryValidatesAndClonesAdditionalPermissions(t *testing.T) {
	valid := []PermissionRequirement{{Permission: "admin:menu-button:read", Action: "read"}}

	for name, register := range map[string]func([]PermissionRequirement) (*Registry, error){
		"query": func(requirements []PermissionRequirement) (*Registry, error) {
			definition := ReferenceQueryDefinition()
			definition.AdditionalPermissions = requirements
			return NewRegistry([]QueryDefinition{definition}, nil)
		},
		"command": func(requirements []PermissionRequirement) (*Registry, error) {
			definition := ReferenceCommandDefinition()
			definition.AdditionalPermissions = requirements
			return NewRegistry(nil, []CommandDefinition{definition})
		},
		"domain command": func(requirements []PermissionRequirement) (*Registry, error) {
			definition := referenceDomainCommandDefinition()
			definition.AdditionalPermissions = requirements
			return NewRegistryWithDomainCommands(nil, nil, []DomainCommandDefinition{definition})
		},
	} {
		t.Run(name+" rejects invalid requirements", func(t *testing.T) {
			invalid := [][]PermissionRequirement{
				{{Permission: "", Action: "read"}},
				{{Permission: "admin:menu button:read", Action: "read"}},
				{{Permission: "admin:menu-button:read", Action: "bad action"}},
				{{Permission: "admin:menu-button:read", Action: "read"}, {Permission: "admin:menu-button:read", Action: "read"}},
			}
			for _, requirements := range invalid {
				if _, err := register(requirements); !errors.Is(err, ErrDefinitionInvalid) {
					t.Fatalf("register(%#v) error = %v, want ErrDefinitionInvalid", requirements, err)
				}
			}
		})

		t.Run(name+" rejects the primary requirement", func(t *testing.T) {
			var primary PermissionRequirement
			switch name {
			case "query":
				definition := ReferenceQueryDefinition()
				primary = PermissionRequirement{Permission: definition.Permission, Action: definition.Action}
			case "command":
				definition := ReferenceCommandDefinition()
				primary = PermissionRequirement{Permission: definition.Permission, Action: definition.Action}
			default:
				definition := referenceDomainCommandDefinition()
				primary = PermissionRequirement{Permission: definition.Permission, Action: definition.Action}
			}
			if _, err := register([]PermissionRequirement{primary}); !errors.Is(err, ErrDefinitionInvalid) {
				t.Fatalf("register(primary duplicate) error = %v, want ErrDefinitionInvalid", err)
			}
		})

		t.Run(name+" deep clones requirements", func(t *testing.T) {
			requirements := append([]PermissionRequirement(nil), valid...)
			registry, err := register(requirements)
			if err != nil {
				t.Fatalf("register() error = %v", err)
			}
			requirements[0].Permission = "mutated"
			var stored []PermissionRequirement
			switch name {
			case "query":
				definition, _ := registry.query(ReferenceQueryID, ReferenceVersion)
				stored = definition.AdditionalPermissions
				if stored[0].Permission != valid[0].Permission {
					t.Fatalf("registered requirements = %#v, want %#v", stored, valid)
				}
				definition.AdditionalPermissions[0].Permission = "lookup-mutated"
				again, _ := registry.query(ReferenceQueryID, ReferenceVersion)
				if again.AdditionalPermissions[0].Permission != valid[0].Permission {
					t.Fatalf("query lookup mutation changed registry: %#v", again.AdditionalPermissions)
				}
			case "command":
				definition, _ := registry.command(ReferenceCommandID, ReferenceVersion)
				stored = definition.AdditionalPermissions
				if stored[0].Permission != valid[0].Permission {
					t.Fatalf("registered requirements = %#v, want %#v", stored, valid)
				}
				definition.AdditionalPermissions[0].Permission = "lookup-mutated"
				again, _ := registry.command(ReferenceCommandID, ReferenceVersion)
				if again.AdditionalPermissions[0].Permission != valid[0].Permission {
					t.Fatalf("command lookup mutation changed registry: %#v", again.AdditionalPermissions)
				}
			default:
				definition, _ := registry.domainCommand(referenceDomainCommandID, ReferenceVersion)
				stored = definition.AdditionalPermissions
				if stored[0].Permission != valid[0].Permission {
					t.Fatalf("registered requirements = %#v, want %#v", stored, valid)
				}
				definition.AdditionalPermissions[0].Permission = "lookup-mutated"
				again, _ := registry.domainCommand(referenceDomainCommandID, ReferenceVersion)
				if again.AdditionalPermissions[0].Permission != valid[0].Permission {
					t.Fatalf("domain lookup mutation changed registry: %#v", again.AdditionalPermissions)
				}
			}
		})
	}
}

func TestRegistryAndProjectionSupportMenuDefinitionAndStringSetResults(t *testing.T) {
	definition := ReferenceQueryDefinition()
	definition.ResultSchema = append(definition.ResultSchema,
		ResultField{Name: "tags", Type: ValueStringSet},
		ResultField{Name: "definition", Type: ValueMenuDefinition},
	)
	if _, err := NewRegistry([]QueryDefinition{definition}, nil); err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	menu := validServiceMenuDefinition()
	values, err := projectValues(map[string]any{"tags": []string{"page", "managed"}, "definition": menu}, definition.ResultSchema)
	if err != nil {
		t.Fatalf("projectValues() error = %v", err)
	}
	if !reflect.DeepEqual(values["tags"], []string{"page", "managed"}) || !reflect.DeepEqual(values["definition"], menu) {
		t.Fatalf("projectValues() = %#v", values)
	}
	if _, err := projectValues(map[string]any{"tags": "page", "definition": map[string]any{"id": menu.ID}}, definition.ResultSchema); !errors.Is(err, ErrExecutionFailed) {
		t.Fatalf("projectValues(untyped) error = %v, want ErrExecutionFailed", err)
	}
	if _, err := projectValues(map[string]any{"tags": []string{"page", "page"}, "definition": menu}, definition.ResultSchema); !errors.Is(err, ErrExecutionFailed) {
		t.Fatalf("projectValues(duplicate string set) error = %v, want ErrExecutionFailed", err)
	}
	tooMany := make([]string, maximumDomainCommandItems+1)
	for index := range tooMany {
		tooMany[index] = string(rune(index + 1))
	}
	if _, err := projectValues(map[string]any{"tags": tooMany, "definition": menu}, definition.ResultSchema); !errors.Is(err, ErrExecutionFailed) {
		t.Fatalf("projectValues(oversized string set) error = %v, want ErrExecutionFailed", err)
	}
}

func TestMenuDefinitionArgumentRequiresTypedValidDTO(t *testing.T) {
	definitions := []ArgumentDefinition{{Name: "definition", Type: ValueMenuDefinition, Required: true}}
	valid := validServiceMenuDefinition()
	validated, err := validateArguments(definitions, map[string]any{"definition": valid})
	if err != nil {
		t.Fatalf("validateArguments(valid) error = %v", err)
	}
	if !reflect.DeepEqual(validated["definition"], valid) {
		t.Fatalf("validated definition = %#v, want %#v", validated["definition"], valid)
	}
	if _, err := validateArguments(definitions, map[string]any{"definition": map[string]any{"id": valid.ID}}); !errors.Is(err, ErrRequestInvalid) {
		t.Fatalf("validateArguments(map) error = %v, want ErrRequestInvalid", err)
	}

	invalid := map[string]func(*MenuDefinition){
		"empty metadata":                  func(definition *MenuDefinition) { definition.Name = "" },
		"dynamic route":                   func(definition *MenuDefinition) { definition.Node.Route = "/users/:id" },
		"parameter key starts with digit": func(definition *MenuDefinition) { definition.Node.Parameters[0].Key = "1mode" },
		"parameter key exceeds limit":     func(definition *MenuDefinition) { definition.Node.Parameters[0].Key = "a" + strings.Repeat("b", 64) },
		"insecure external URL": func(definition *MenuDefinition) {
			definition.Node.External = true
			definition.Node.ExternalURL = "http://example.com/users"
			definition.Node.OpenMode = MenuOpenModeNewTab
			definition.Node.Route = ""
			definition.Node.ComponentKey = ""
			definition.Node.ResourceCode = ""
		},
		"untyped parameter":  func(definition *MenuDefinition) { definition.Node.Parameters[0].Value = true },
		"physical parameter": func(definition *MenuDefinition) { definition.Node.Parameters[0].Key = "physical-routing" },
		"incomplete button":  func(definition *MenuDefinition) { definition.Buttons[0].PermissionCode = "" },
		"directory buttons": func(definition *MenuDefinition) {
			definition.Node.NodeType = MenuNodeTypeDirectory
			definition.Node.Route = ""
			definition.Node.ComponentKey = ""
			definition.Node.ResourceCode = ""
			definition.Node.Parameters = nil
		},
	}
	for name, mutate := range invalid {
		t.Run(name, func(t *testing.T) {
			definition := validServiceMenuDefinition()
			mutate(&definition)
			if _, err := validateArguments(definitions, map[string]any{"definition": definition}); !errors.Is(err, ErrRequestInvalid) {
				t.Fatalf("validateArguments() error = %v, want ErrRequestInvalid", err)
			}
		})
	}

	tooMany := validServiceMenuDefinition()
	tooMany.Node.Parameters = make([]MenuParameter, MaximumMenuParameters+1)
	for index := range tooMany.Node.Parameters {
		tooMany.Node.Parameters[index] = MenuParameter{Key: "parameter-" + string(rune('a'+index)), Type: MenuParameterTypeString, Value: "value"}
	}
	if _, err := validateArguments(definitions, map[string]any{"definition": tooMany}); !errors.Is(err, ErrRequestInvalid) {
		t.Fatalf("validateArguments(too many parameters) error = %v, want ErrRequestInvalid", err)
	}

	cloned := cloneValidatedArguments(validated)
	clonedDefinition := cloned["definition"].(MenuDefinition)
	clonedDefinition.Node.Parameters[0].Value = "changed"
	clonedDefinition.Buttons[0].LabelEN = "Changed"
	original := validated["definition"].(MenuDefinition)
	if original.Node.Parameters[0].Value != "active" || original.Buttons[0].LabelEN != "Export" {
		t.Fatalf("clone mutation changed original menu definition: %#v", original)
	}
}

func TestMenuDefinitionArgumentRejectsExecutableAndPhysicalStringParameterValues(t *testing.T) {
	definitions := []ArgumentDefinition{{Name: "definition", Type: ValueMenuDefinition, Required: true}}
	unsafeValues := map[string]string{
		"script tag":          `<script>alert("x")</script>`,
		"script URI":          `javascript:alert("x")`,
		"script keyword":      `script`,
		"expression":          `${tenant.id}`,
		"expression keyword":  `expression`,
		"template expression": `{{ currentUser.id }}`,
		"SQL":                 `SELECT * FROM platform_admin_users`,
		"SQL keyword":         `sql`,
		"route parameter":     `/users/:id`,
		"route expression":    `/users/{id}`,
		"route wildcard":      `/users/*`,
		"datasource routing":  `datasource=primary`,
		"datasource keyword":  `datasource`,
		"shard routing":       `shard:tenant-42`,
		"shard keyword":       `shard`,
		"database routing":    `database=platform`,
		"database keyword":    `database`,
		"schema routing":      `{"schema":"public"}`,
		"schema keyword":      `schema`,
	}
	for name, value := range unsafeValues {
		t.Run(name, func(t *testing.T) {
			definition := validServiceMenuDefinition()
			definition.Node.Parameters[0].Value = value
			if _, err := validateArguments(definitions, map[string]any{"definition": definition}); !errors.Is(err, ErrRequestInvalid) {
				t.Fatalf("validateArguments(%q) error = %v, want ErrRequestInvalid", value, err)
			}
		})
	}

	for name, value := range map[string]string{
		"ordinary value": "active",
		"word substring": "selection",
		"camel-case key": "schemaVersion",
		"static path":    "/users/profile",
	} {
		t.Run("allows "+name, func(t *testing.T) {
			definition := validServiceMenuDefinition()
			definition.Node.Parameters[0].Value = value
			if _, err := validateArguments(definitions, map[string]any{"definition": definition}); err != nil {
				t.Fatalf("validateArguments(%q) error = %v", value, err)
			}
		})
	}
}

func TestDecodeCommandRequestConvertsStrictMenuDefinitionJSONToTypedDTO(t *testing.T) {
	payload := `{
		"commandId":"platform.navigation.menu-definition.replace",
		"version":"1.0.0",
		"arguments":{"definition":{
			"id":"menu-users","name":"Users","description":"Manage users","updatedAt":"2026-07-15T00:00:00Z",
			"node":{"code":"users","parentCode":"","nodeType":"page","titleZh":"Users ZH","titleEn":"Users","descriptionZh":"","descriptionEn":"","status":"enabled","icon":"users","sortOrder":1,"route":"/users","componentKey":"users","resourceCode":"users","external":false,"externalUrl":"","openMode":"","parameters":[{"key":"page","type":"number","value":1}],"cacheEnabled":true,"hidden":false,"activeMenuCode":"","breadcrumbVisible":true},
			"buttons":[{"menuCode":"users","buttonKey":"export","labelZh":"Export ZH","labelEn":"Export","action":"export","sortOrder":1,"status":"enabled","permissionCode":"page:users:export"}]
		}},
		"idempotencyKey":"replace-users"
	}`
	request, err := DecodeCommandRequest(strings.NewReader(payload))
	if err != nil {
		t.Fatalf("DecodeCommandRequest() error = %v", err)
	}
	if _, ok := request.Arguments["definition"].(map[string]any); !ok {
		t.Fatalf("decoded definition type = %T, want map[string]any before contract validation", request.Arguments["definition"])
	}
	validated, err := validateArguments([]ArgumentDefinition{{Name: "definition", Type: ValueMenuDefinition, Required: true}}, request.Arguments)
	if err != nil {
		t.Fatalf("validateArguments(decoded) error = %v", err)
	}
	definition, ok := validated["definition"].(MenuDefinition)
	if !ok || definition.Node.Parameters[0].Type != MenuParameterTypeNumber {
		t.Fatalf("validated decoded definition = %#v", validated["definition"])
	}

	for name, changed := range map[string]string{
		"string passthrough": `{"commandId":"platform.navigation.menu-definition.replace","version":"1.0.0","arguments":{"definition":"{}"}}`,
		"unknown node field": strings.Replace(payload, `"breadcrumbVisible":true`, `"breadcrumbVisible":true,"datasource":"primary"`, 1),
		"incomplete button":  strings.Replace(payload, `,"permissionCode":"page:users:export"`, ``, 1),
	} {
		t.Run(name, func(t *testing.T) {
			decoded, err := DecodeCommandRequest(strings.NewReader(changed))
			if err != nil {
				t.Fatalf("DecodeCommandRequest() error = %v", err)
			}
			if _, err := validateArguments([]ArgumentDefinition{{Name: "definition", Type: ValueMenuDefinition, Required: true}}, decoded.Arguments); !errors.Is(err, ErrRequestInvalid) {
				t.Fatalf("validateArguments() error = %v, want ErrRequestInvalid", err)
			}
		})
	}

	plain, err := DecodeCommandRequest(strings.NewReader(`{"commandId":"example.replace","version":"1.0.0","arguments":{"definition":"plain text"}}`))
	if err != nil || plain.Arguments["definition"] != "plain text" {
		t.Fatalf("DecodeCommandRequest(non-menu definition) = %#v, %v", plain.Arguments, err)
	}
}

func TestRuntimeRequiresEveryAdditionalPermission(t *testing.T) {
	requirements := []PermissionRequirement{
		{Permission: "admin:menu-button:read", Action: "read"},
		{Permission: "admin:menu-api:read", Action: "read"},
	}

	query := ReferenceQueryDefinition()
	query.AdditionalPermissions = requirements
	command := ReferenceCommandDefinition()
	command.AdditionalPermissions = requirements
	domain := referenceDomainCommandDefinition()
	domain.AdditionalPermissions = requirements

	for name, invoke := range map[string]func(*Runtime) error{
		"query": func(runtime *Runtime) error {
			_, err := runtime.ExecuteQuery(referenceExecution(query.Permission, query.Action, "tenant-1"), QueryRequest{QueryID: query.ID, Version: query.Version})
			return err
		},
		"command": func(runtime *Runtime) error {
			_, err := runtime.ExecuteCommand(referenceExecution(command.Permission, command.Action, "tenant-1"), CommandRequest{CommandID: command.ID, Version: command.Version, Arguments: map[string]any{"code": "A", "name": "B"}, IdempotencyKey: "key"})
			return err
		},
		"domain command": func(runtime *Runtime) error {
			_, err := runtime.ExecuteCommand(referenceDomainInvocation(), CommandRequest{CommandID: domain.ID, Version: domain.Version, Arguments: map[string]any{"roleIds": []string{"role-a"}}, IdempotencyKey: "key"})
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			registry, err := NewRegistryWithDomainCommands([]QueryDefinition{query}, []CommandDefinition{command}, []DomainCommandDefinition{domain})
			if err != nil {
				t.Fatalf("NewRegistryWithDomainCommands() error = %v", err)
			}
			allowed := map[string]bool{
				query.Permission + "\x00" + query.Action:                     true,
				command.Permission + "\x00" + command.Action:                 true,
				domain.Permission + "\x00" + domain.Action:                   true,
				requirements[0].Permission + "\x00" + requirements[0].Action: true,
				requirements[1].Permission + "\x00" + requirements[1].Action: true,
			}
			authorizer := AuthorizerFunc(func(_ context.Context, _ kernel.ExecutionContext, permission string, action string) bool {
				return allowed[permission+"\x00"+action]
			})
			runtime, err := NewRuntimeWithDomainCommands(registry, authorizer, &querySpy{}, commandExecutorFunc(func(context.Context, CommandPlan) (CommandResult, error) {
				return CommandResult{Values: map[string]any{"affected": int64(1)}}, nil
			}), &domainCommandSpy{result: CommandResult{Values: map[string]any{"affected": int64(1)}}}, NewMemoryIdempotencyStore())
			if err != nil {
				t.Fatalf("NewRuntimeWithDomainCommands() error = %v", err)
			}
			if err := invoke(runtime); err != nil {
				t.Fatalf("invoke(all allowed) error = %v", err)
			}
			allowed[requirements[1].Permission+"\x00"+requirements[1].Action] = false
			if err := invoke(runtime); !errors.Is(err, ErrObjectUnavailable) {
				t.Fatalf("invoke(additional denied) error = %v, want ErrObjectUnavailable", err)
			}
		})
	}
}

func validServiceMenuDefinition() MenuDefinition {
	return MenuDefinition{
		ID: "menu-users", Name: "Users", Description: "Manage users", UpdatedAt: "2026-07-15T00:00:00Z",
		Node: MenuNode{
			Code: "users", NodeType: MenuNodeTypePage, TitleZH: "Users ZH", TitleEN: "Users", Status: "enabled",
			Route: "/users", ComponentKey: "users", ResourceCode: "users", Parameters: []MenuParameter{{Key: "tab", Type: MenuParameterTypeString, Value: "active"}},
			CacheEnabled: true, BreadcrumbVisible: true,
		},
		Buttons: []PageButton{{MenuCode: "users", ButtonKey: "export", LabelZH: "Export ZH", LabelEN: "Export", Action: "export", SortOrder: 1, Status: "enabled", PermissionCode: "page:users:export"}},
	}
}

type commandExecutorFunc func(context.Context, CommandPlan) (CommandResult, error)

func (f commandExecutorFunc) ExecuteCommand(ctx context.Context, plan CommandPlan) (CommandResult, error) {
	return f(ctx, plan)
}
