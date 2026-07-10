# Refine Custom Panels And Actions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** Add schema-driven custom row actions, batch actions and drawer panels to the generic Refine admin resource console.

**Architecture:** Extend the platform admin resource schema with metadata-only action and panel contracts, validate those contracts in Go and Node gates, then render them through reusable Ant Design wrappers in `GenericResourceConsole`. Capability-owned handlers remain behind neutral admin routes; platform UI only declares and invokes allowed actions.

**Tech Stack:** Go, Gin, GORM-ready platform contracts, Refine, React, Ant Design, TypeScript, Node validation scripts.

---

## File Structure

- Modify `internal/platform/capability/manifest.go` to add action and panel declaration structs.
- Modify `internal/platform/capability/admin_contract.go` to validate action and panel metadata.
- Modify `internal/platform/adminresource/schema.go` to expose action and panel metadata through resource schemas.
- Modify `admin/src/platform/api/client.ts` to add TypeScript schema types.
- Modify `admin/src/platform/resources/GenericResourceConsole.tsx` to render action overflow, batch actions and drawer tabs.
- Modify `admin/src/platform/i18n.ts` to add Chinese and English labels.
- Modify `admin/src/styles.css` to style action bars and drawer tabs with existing tokens.
- Modify `scripts/validate-admin-resources.mjs` and related tests to gate resource JSON metadata.
- Modify `scripts/validate-admin-ui-contracts.mjs` to enforce i18n, drawer tab and action component boundaries.
- Update `resources/admin-resources.json` with one platform sample resource action/panel set.
- Update `docs/admin-ui-foundation.md`, `docs/platform-foundation-task-map.md` and `resources/platform-foundation-task-graph.json` after implementation evidence exists.

## Task 1: Contract Types And Go Validation

**Files:**
- Modify: `internal/platform/capability/manifest.go`
- Modify: `internal/platform/capability/admin_contract.go`
- Test: `internal/platform/capability/admin_contract_test.go`

- [x] **Step 1: Write failing Go tests**

Add tests that prove:

```go
func TestValidateAdminSurfaceRejectsDuplicateResourceActionKeys(t *testing.T) {
	manifest := Manifest{ID: "demo", Admin: AdminSurface{Resources: []AdminResource{{
		Resource: "demo-resources",
		Title: Text("演示资源", "Demo Resources"),
		Description: Text("演示。", "Demo."),
		PermissionPrefix: "admin:demo",
		Menu: AdminMenu{Route: "/demo-resources", Group: "demo", Icon: "demo"},
		Actions: []AdminResourceAction{
			{Key: "approve", Label: Text("通过", "Approve"), Kind: "row", Permission: "admin:demo:update", Method: "POST", Route: "/api/admin/demo-resources/:id/approve"},
			{Key: "approve", Label: Text("再次通过", "Approve Again"), Kind: "row", Permission: "admin:demo:update", Method: "POST", Route: "/api/admin/demo-resources/:id/approve-again"},
		},
	}}}}
	if err := ValidateAdminSurface([]Manifest{manifest}); err == nil || !strings.Contains(err.Error(), "duplicate action key") {
		t.Fatalf("ValidateAdminSurface error = %v, want duplicate action key", err)
	}
}

func TestValidateAdminSurfaceRejectsDangerActionWithoutConfirm(t *testing.T) {
	manifest := Manifest{ID: "demo", Admin: AdminSurface{Resources: []AdminResource{{
		Resource: "demo-resources",
		Title: Text("演示资源", "Demo Resources"),
		Description: Text("演示。", "Demo."),
		PermissionPrefix: "admin:demo",
		Menu: AdminMenu{Route: "/demo-resources", Group: "demo", Icon: "demo"},
		Actions: []AdminResourceAction{{
			Key: "close",
			Label: Text("关闭", "Close"),
			Kind: "row",
			Tone: "danger",
			Permission: "admin:demo:update",
			Method: "POST",
			Route: "/api/admin/demo-resources/:id/close",
		}},
	}}}}
	if err := ValidateAdminSurface([]Manifest{manifest}); err == nil || !strings.Contains(err.Error(), "danger action requires confirmation") {
		t.Fatalf("ValidateAdminSurface error = %v, want confirmation error", err)
	}
}

func TestValidateAdminSurfaceAcceptsResourcePanels(t *testing.T) {
	manifest := Manifest{ID: "demo", Admin: AdminSurface{Resources: []AdminResource{{
		Resource: "demo-resources",
		Title: Text("演示资源", "Demo Resources"),
		Description: Text("演示。", "Demo."),
		PermissionPrefix: "admin:demo",
		Menu: AdminMenu{Route: "/demo-resources", Group: "demo", Icon: "demo"},
		Panels: []AdminResourcePanel{{
			Key: "audit",
			Label: Text("审计", "Audit"),
			Kind: "audit",
			Permission: "admin:demo:read",
			Component: "audit-timeline",
			Order: 30,
		}},
	}}}}
	if err := ValidateAdminSurface([]Manifest{manifest}); err != nil {
		t.Fatalf("ValidateAdminSurface error = %v", err)
	}
}
```

- [x] **Step 2: Run tests and verify RED**

Run: `rtk go test ./internal/platform/capability`

Expected: compile fails because `AdminResourceAction` and `AdminResourcePanel` are undefined.

- [x] **Step 3: Implement minimal contract structs**

Add metadata structs in `manifest.go`:

```go
type AdminResource struct {
	Resource         string
	Title            LocalizedText
	Description      LocalizedText
	PermissionPrefix string
	Menu             AdminMenu
	FormGroups       []AdminFormGroup
	Fields           []AdminField
	Actions          []AdminResourceAction
	Panels           []AdminResourcePanel
	SearchFields     []string
	DefaultSortKey   string
}

type AdminResourceAction struct {
	Key         string
	Label       LocalizedText
	Kind        string
	Tone        string
	Icon        string
	Permission  string
	Route       string
	Method      string
	Confirm     *AdminActionConfirm
	AuditAction string
	Refresh     bool
}

type AdminActionConfirm struct {
	Title       LocalizedText
	Description LocalizedText
	OkText      LocalizedText
}

type AdminResourcePanel struct {
	Key        string
	Label      LocalizedText
	Kind       string
	Permission string
	Component  string
	Order      int
	Empty      LocalizedText
}
```

Add validation helpers in `admin_contract.go` for action keys, kinds, tones, route/method, confirm and panel metadata.

- [x] **Step 4: Run tests and verify GREEN**

Run: `rtk go test ./internal/platform/capability`

Expected: pass.

## Task 2: Schema Export

**Files:**
- Modify: `internal/platform/adminresource/schema.go`
- Test: `internal/platform/adminresource/store_test.go`

- [x] **Step 1: Write failing schema test**

Add a test that creates a capability resource with one action and one panel, builds a store from capabilities and asserts `Store.Schema(resource).Actions` and `Panels` are cloned and present.

- [x] **Step 2: Run test and verify RED**

Run: `rtk go test ./internal/platform/adminresource`

Expected: compile fails because schema action/panel fields do not exist.

- [x] **Step 3: Implement schema structs and cloning**

Add `ResourceActionDefinition`, `ResourceActionConfirm`, `ResourcePanelDefinition` to `schema.go`, add `Actions` and `Panels` to `Schema`, copy them from capability resources in `schemaFromCapabilityResource`, and deep clone them in `cloneSchema`.

- [x] **Step 4: Run tests and verify GREEN**

Run: `rtk go test ./internal/platform/adminresource`

Expected: pass.

## Task 3: Resource Manifest And Node Validation

**Files:**
- Modify: `resources/admin-resources.json`
- Modify: `scripts/validate-admin-resources.mjs`
- Test: `scripts/validate-admin-resources.test.mjs`

- [x] **Step 1: Write failing validator tests**

Add tests that reject duplicate action keys, danger actions without confirm metadata, unsupported panel kinds and component values containing `/`, `\\` or `.`.

- [x] **Step 2: Run tests and verify RED**

Run: `rtk node --test scripts/validate-admin-resources.test.mjs`

Expected: fails because validator does not inspect `actions` or `panels`.

- [x] **Step 3: Implement JSON validator rules**

Validate optional `actions` and `panels` arrays on resources. Enforce localized labels, supported enum values, route prefix `/api/admin/`, permission presence and safe component keys.

- [x] **Step 4: Add one sample resource metadata set**

Add to `menus` or `org-units`:

```json
"actions": [
  {
    "key": "copy-config",
    "label": { "zh": "复制配置", "en": "Copy Config" },
    "kind": "row",
    "tone": "default",
    "icon": "copy",
    "permission": "admin:menu:read",
    "refresh": false
  }
],
"panels": [
  {
    "key": "audit",
    "label": { "zh": "审计", "en": "Audit" },
    "kind": "audit",
    "permission": "admin:menu:read",
    "component": "audit-timeline",
    "order": 30
  }
]
```

- [x] **Step 5: Run validator tests and resource validator**

Run:

```bash
rtk node --test scripts/validate-admin-resources.test.mjs
rtk node scripts/validate-admin-resources.mjs
```

Expected: both pass.

## Task 4: Frontend Types And Rendering

**Files:**
- Modify: `admin/src/platform/api/client.ts`
- Modify: `admin/src/platform/resources/GenericResourceConsole.tsx`
- Modify: `admin/src/platform/i18n.ts`
- Modify: `admin/src/styles.css`
- Modify: `scripts/validate-admin-ui-contracts.mjs`

- [x] **Step 1: Add failing UI contract validator checks**

Update `validate-admin-ui-contracts.mjs` to require `GenericResourceConsole.tsx` to reference schema `actions`, schema `panels`, `Dropdown`, `Tabs`, and localized custom-action labels from `dictionary`.

- [x] **Step 2: Run validator and verify RED**

Run: `rtk node scripts/validate-admin-ui-contracts.mjs`

Expected: fails because custom action/panel rendering is missing.

- [x] **Step 3: Add TypeScript schema types**

Add `AdminResourceAction`, `AdminResourceActionConfirm` and `AdminResourcePanel` to `client.ts`, then add optional `actions?: AdminResourceAction[]` and `panels?: AdminResourcePanel[]` to `AdminResourceSchema`.

- [x] **Step 4: Render row action overflow and batch command bar**

In `GenericResourceConsole.tsx`, merge default actions with `schema.actions`. Render row actions in the actions column using `AdminActionButton` for primary visible actions and AntD `Dropdown` for overflow. Render `kind === "batch"` actions inside `PlatformDataTable.batchActions`.

- [x] **Step 5: Render drawer tabs**

Replace single `ResourceInspector` content with AntD `Tabs`. Always render details and permissions. Render schema panels by kind. For `audit`, `approval`, `files` and unknown `custom`, show localized safe placeholders until owning capabilities provide concrete data slots.

- [x] **Step 6: Add i18n keys**

Add Chinese and English dictionary keys for custom action unavailable, action failed, drawer tabs, approval panel, file panel, audit panel and plugin panel empty states.

- [x] **Step 7: Run UI validators and build**

Run:

```bash
rtk node scripts/validate-admin-i18n.mjs
rtk node scripts/validate-admin-ui-contracts.mjs
rtk npm --prefix admin run build
```

Expected: all pass.

## Task 5: Generated Artifacts And Documentation

**Files:**
- Modify generated resources under `resources/generated/`
- Modify: `docs/admin-ui-foundation.md`
- Modify: `docs/platform-foundation-task-map.md`
- Modify: `resources/platform-foundation-task-graph.json`

- [x] **Step 1: Regenerate contracts**

Run:

```bash
rtk node scripts/generate-admin-resource-contract.mjs
rtk node scripts/generate-admin-openapi.mjs
rtk node scripts/generate-admin-codegen-preview.mjs
rtk node scripts/generate-admin-scaffold-plan.mjs
rtk node scripts/generate-admin-scaffold-draft.mjs
```

Expected: generated files include action/panel metadata where relevant and no source-writing promotion occurs.

- [x] **Step 2: Update docs**

Document the action/panel schema rules, frontend rendering defaults, i18n gate and capability boundary in `docs/admin-ui-foundation.md` and the task map.

- [x] **Step 3: Promote task graph node when evidence exists**

Current result: `resources/platform-foundation-task-graph.json` marks `refine-custom-panels-and-actions` as `implemented` because the metadata contract, sample menu action/panel, enterprise `policy-review` custom routes, UI renderer, i18n, validators, build and browser evidence are present. At this plan's original checkpoint it did not complete `policy-review-custom-ui`; that dedicated approval console is now implemented by the separate `policy-review-custom-ui` node with its own browser evidence.

## Task 6: Browser QA And Final Verification

**Files:**
- Create screenshots under `tmp/product-design/refine-custom-panels-and-actions-20260707/`

- [x] **Step 1: Start admin dev server**

Run: `rtk npm --prefix admin run dev -- --host 127.0.0.1`

Expected: local Vite URL is printed.

- [x] **Step 2: Use the in-app browser for visual QA**

Open the local URL in the existing in-app browser. Capture desktop and narrow viewport screenshots for a resource list with drawer tabs, row overflow and batch selection.

- [x] **Step 3: Run full relevant verification**

Run:

```bash
rtk go test ./...
rtk node scripts/validate-admin-resources.mjs
rtk node scripts/validate-admin-i18n.mjs
rtk node scripts/validate-admin-ui-contracts.mjs
rtk node scripts/validate-platform-foundation-task-graph.mjs
rtk npm --prefix admin run build
rtk git diff --check
rtk codegraph sync .
rtk codegraph status
```

Expected: all pass and CodeGraph is up to date.

## Completion Evidence

- Contract and schema evidence: `AdminResourceAction` and `AdminResourcePanel` exist in Go and TypeScript contracts, are cloned into schemas, and are included in generated admin resource contracts.
- Runtime evidence: `GenericResourceConsole` renders schema-declared row actions, batch actions, overflow dropdowns and drawer tabs, and executes routed actions through `executeAdminResourceAction`.
- Capability evidence: the optional `policy-review` platform governance capability exposes request, approve, reject and export routes in the enterprise profile without shipping business workflows.
- Visual evidence: screenshots exist under `tmp/product-design/refine-custom-panels-and-actions-20260707/` for desktop row actions, overflow actions, drawer tabs, audit panel, batch command bar and mobile behavior.
- Verification evidence: focused Go tests, Node tests, admin resource validation, admin UI contract validation and admin i18n validation pass for this node.

## Self-Review

- Spec coverage: action metadata, panel metadata, permissions, i18n, audit route metadata, Refine UI rendering, validators, docs and browser QA are covered.
- Scope boundary: zshenmez custom business panels remain downstream capability work. Policy-review UI and file preview workflow were later implemented as platform-governance/generic-resource nodes with their own evidence gates.
- Placeholder scan: no task uses undefined implementation placeholders; every deferred behavior has a safe placeholder UI or follow-on node.
