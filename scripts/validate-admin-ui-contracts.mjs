import { readFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

function argValue(name, fallback) {
  const index = process.argv.indexOf(name);
  if (index === -1) return fallback;
  return process.argv[index + 1] ?? "";
}

const defaultRoot = dirname(dirname(fileURLToPath(import.meta.url)));
const root = resolve(defaultRoot, argValue("--root", defaultRoot));

const files = {
  app: readSource("admin/src/App.tsx"),
  authProvider: readSource("admin/src/platform/refine/authProvider.ts"),
  capabilityMetadata: readSource("admin/src/platform/capabilities/metadata.ts"),
  client: readSource("admin/src/platform/api/client.ts"),
  primitives: readSource("admin/src/platform/ui/AdminPrimitives.tsx"),
  resourceConsole: readSource("admin/src/platform/resources/GenericResourceConsole.tsx"),
  shell: readSource("admin/src/platform/shell/AdminShell.tsx"),
  settings: readSource("admin/src/platform/ui/SystemSettingsDrawer.tsx"),
  table: readSource("admin/src/platform/ui/PlatformDataTable.tsx"),
  pagination: readSource("admin/src/platform/ui/PlatformPaginationBar.tsx"),
  policyReview: readSource("admin/src/platform/policy-review/PolicyReviewConsole.tsx"),
  resourceForm: readSource("admin/src/platform/ui/PlatformResourceForm.tsx"),
  formSlotRegistry: readSource("admin/src/platform/ui/formSlotRegistry.tsx"),
  treeSelect: readSource("admin/src/platform/ui/PlatformTreeSelect.tsx"),
  uiIndex: readSource("admin/src/platform/ui/index.ts"),
  styles: readSource("admin/src/styles.css"),
};

const failures = [];
const mobileStyles = extractCssBlock(files.styles, "@media (max-width: 767px)");

requireIncludes(files.app, "readStoredUIConfig", "App must keep persisted admin UI configuration.");
requireIncludes(files.app, "writeStorageValue(adminPreferenceStorageKeys.ui", "App must persist admin UI configuration changes.");
requireIncludes(files.app, "defaultAdminUIConfig", "App must fall back to the shared default admin UI config.");
requireIncludes(files.app, "PolicyReviewConsole", "App must mount the policy-review custom governance console when the resource is enabled.");
requireIncludes(files.app, 'resource.route !== "/policy-reviews"', "Generic resource routing must not also mount policy-reviews when the custom console is active.");
requireIncludes(files.client, "export class AdminAPIError", "Admin API failures must expose typed status codes.");
requireIncludes(files.client, "ADMIN_SESSION_EXPIRED_EVENT", "The shared client must expose the session-expired event contract.");
requireIncludes(files.client, "statusCode", "Admin API errors must carry HTTP status.");
requireIncludes(files.client, "dispatchEvent", "Stored-token 401 responses must notify the app.");
requireIncludes(files.app, "ADMIN_SESSION_EXPIRED_EVENT", "App must listen for shared session expiry.");
requireIncludes(files.app, "dictionary.sessionExpired", "Session expiry feedback must be localized.");
requireRegex(
  files.client,
  /statusCode !== 401\s*\|\|\s*!requestToken\s*\|\|\s*getAuthToken\(\) !== requestToken/,
  "Session expiry must clear only the exact token used by the failed request.",
);
requireNotIncludes(files.client, "hadToken", "Session expiry handling must retain the exact request token instead of a boolean token flag.");
requireIncludes(files.client, 'const { auth = "stored-token", ...fetchInit } = init;', "Request must separate the platform auth mode from native fetch options.");
requireNotIncludes(files.client, "...init,", "Platform-only request options must not be forwarded to fetch.");
requireIncludes(files.client, 'return request<AuthProviderList>("/auth/providers", { auth: "none" });', "Auth provider discovery must explicitly avoid stored-token authentication.");
requireRegex(
  files.client,
  /request<AuthLoginResult>\("\/auth\/login",\s*\{[\s\S]*?auth:\s*"none"/,
  "Auth login must explicitly avoid stored-token authentication.",
);
requireIncludes(files.app, "const [sessionExpired, setSessionExpired] = useState(false);", "App must keep session expiry in stable non-localized state.");
requireIncludes(files.app, "setSessionExpired(true);", "Session expiry recovery must set the stable expiry state.");
requireCountExactly(files.app, "setSessionExpired(true);", 1, "Only the exact-token session-expired event may set App expiry state.");
requireIncludes(files.app, "sessionExpired ? dictionary.sessionExpired : authError || error", "Session expiry display must use the current localized dictionary and override provider errors.");
requireIncludes(files.app, "setSessionExpired(false);", "Successful login must clear stable session expiry state.");
requireNotIncludes(files.app, "current === dictionary.sessionExpired", "App must not identify session expiry by comparing localized strings.");
requireIncludes(files.client, "parsePlatformResponse", "Direct fetch helpers must share response normalization.");
requireIncludes(files.authProvider, "error instanceof AdminAPIError", "Refine auth errors must use the typed admin API error contract.");
requireIncludes(files.capabilityMetadata, 'label: { zh: "身份与组织", en: "Identity & Organization" }', "Core identity capability must make default organization ownership explicit.");
requireIncludes(files.capabilityMetadata, 'makeOptional("personnel", { zh: "人员与岗位", en: "Personnel & Positions" }', "Optional personnel capability must not be labeled as the organization capability.");
requireIncludes(files.capabilityMetadata, "默认平台底座已提供组织机构", "Optional personnel copy must state that organization units are part of the default foundation.");

requireIncludes(files.shell, "SystemSettingsDrawer", "AdminShell must expose account/system settings through the shared drawer.");
requireIncludes(files.shell, "user-menu-trigger", "AdminShell must keep the avatar/settings trigger in the topbar.");
requireIncludes(files.shell, "language-toggle-button", "AdminShell must use an icon language toggle, not a language dropdown.");
requireIncludes(files.shell, "desktop-sider-toggle", "AdminShell must keep a desktop sidebar collapse affordance.");
requireIncludes(files.shell, "brand-collapse-button", "AdminShell must keep the brand-area sidebar collapse affordance.");
requireIncludes(files.shell, "platform-work-tabs", "AdminShell must keep browser-like work tabs.");
requireIncludes(files.shell, "workTabMenuItems", "AdminShell work tabs must keep the close/current/left/right context menu.");
requireIncludes(files.shell, "buildNavigationTree", "AdminShell must build multi-level navigation from resource parents.");
requireIncludes(files.shell, '"business/access"', "AdminShell must label business/access through a full parent path.");
requireIncludes(files.shell, '"business/dispatch"', "AdminShell must label business/dispatch through a full parent path.");
requireNotIncludes(files.shell, "access: dictionary.navBusinessAccess", "AdminShell must not reuse plain access for business navigation labels.");
requireIncludes(files.shell, 'href="#platform-main-content"', "AdminShell must expose a skip-to-content link.");
requireIncludes(files.shell, 'id="platform-main-content"', "AdminShell main region must expose a stable focus target.");
requireIncludes(files.shell, "previousRouteRef", "AdminShell must move focus only after actual route changes.");
requireIncludes(files.shell, "pendingDrawerRouteFocusRef", "Drawer route changes must defer main focus until the mobile navigation closes.");
requireIncludes(files.shell, "if (pendingDrawerRouteFocusRef.current)", "Route focus must remain deferred while mobile navigation is closing.");
requireIncludes(files.shell, "pendingDrawerRouteFocusRef.current = !resource.isExternal && resource.route !== activeRoute", "Mobile navigation must mark only actual internal route changes for deferred focus.");
requireIncludes(files.shell, "afterOpenChange={handleMobileDrawerOpenChange}", "Mobile navigation must focus main through the Drawer close lifecycle.");
requireIncludes(files.shell, "dictionary.openMobileNavigation", "Mobile navigation must use an explicit localized accessible name.");
requireIncludes(files.shell, "dictionary.alerts", "The alert icon control must use an explicit localized accessible name.");
requireIncludes(files.shell, "platform-mobile-contextbar", "AdminShell must provide the approved compact mobile context bar.");
requireRegex(
  files.shell,
  /const handleMobileWorkTabClose = \(route: string\) => \{\s*setOpenContext\(null\);\s*closeWorkTab\(route\);\s*\};/,
  "Mobile work-tab close must dismiss its context panel before closing the tab.",
);
requireIncludes(
  files.shell,
  "onClick={() => handleMobileWorkTabClose(resource.route)}",
  "Mobile work-tab close controls must use the context-closing handler.",
);

for (const key of [
  "appearance",
  "layout",
  "general",
  "assist",
  "themeNames.map",
  "customPrimary",
  "adminLayoutModes.map",
  "showWorkTabs",
  "sidebarCollapsed",
  "showLayoutLegend",
  "density",
  "sidebarWidth",
  "menuItemHeight",
  "pageTransition",
  "watermark",
  "visualAid",
  "exportConfig",
  "importConfig",
  "resetConfig",
]) {
  requireIncludes(files.settings, key, `SystemSettingsDrawer must keep ${key} support.`);
}

requireIncludes(files.table, "PlatformPaginationBar", "PlatformDataTable must use the shared pagination bar.");
requireIncludes(files.table, "PlatformDropdownPlugin", "PlatformDataTable must use shared dropdown plugins for filters and columns.");
requireIncludes(files.table, "platform-column-menu", "PlatformDataTable must keep column visibility settings.");
requireIncludes(files.table, "filterFields", "PlatformDataTable must keep advanced filter fields.");
requireIncludes(files.table, "toolbarExtra", "PlatformDataTable must expose a toolbarExtra slot for page-specific controls.");
requireIncludes(files.table, "batchActions", "PlatformDataTable must keep batch action slots.");
requireIncludes(files.table, "rowActions", "PlatformDataTable must expose a rowActions slot instead of forcing callers to fork columns.");
requireIncludes(files.table, "expandedRow", "PlatformDataTable must expose an expandedRow slot for optional row detail.");
requireIncludes(files.table, "inlineEditor", "PlatformDataTable must expose an inlineEditor extension point that is disabled by default.");
requireIncludes(files.table, "detailDrawer", "PlatformDataTable must expose a detailDrawer slot for caller-owned inspectors.");
requireIncludes(files.table, "emptyState", "PlatformDataTable must expose an emptyState slot while keeping the default empty state.");
requireIncludes(files.table, "mobileCards", "PlatformDataTable must keep mobile card rendering.");
requireOrder(files.table, "mobileCards(dataSource)", "<PlatformPaginationBar", "PlatformDataTable must render mobile cards before pagination.");
requireIncludes(files.table, 'export type PlatformDataTableColumnPriority = "essential" | "standard" | "extended"', "PlatformDataTable must expose responsive priority tiers.");
requireIncludes(files.table, "responsiveBreakpointsForPriority", "PlatformDataTable must map priority to AntD breakpoints.");
requireIncludes(files.table, "column.responsive ?? responsiveBreakpointsForPriority(column.priority)", "PlatformDataTable must preserve caller-provided responsive breakpoints.");
requireIncludes(files.resourceConsole, "tableColumnPriority(index)", "Generic resource tables must derive priority from schema order.");
requireIncludes(files.resourceConsole, "form.getFieldInstance", "Resource modals must focus the first editable schema field.");
requireIncludes(files.resourceConsole, "afterOpenChange={(open) => {", "Resource modals must focus through the AntD open lifecycle.");
requireIncludes(files.resourceConsole, "focusFirstEditableFormField(form, formFields);", "Resource modals must invoke the shared first-field focus helper after opening.");
requireIncludes(files.resourceConsole, 'document.querySelectorAll<HTMLElement>(".admin-form-modal")', "Resource modal fallback focus must inspect only admin form modal roots.");
requireIncludes(files.resourceConsole, "modal.getClientRects().length > 0", "Resource modal fallback focus must select the currently visible modal.");
requireIncludes(files.resourceConsole, "visibleModal?.querySelector<HTMLElement>(FOCUSABLE_RESOURCE_FORM_CONTROL_SELECTOR)", "Resource modal fallback focus must stay scoped to the visible modal.");
requireIncludes(files.resourceConsole, 'input:not([type="hidden"]):not([disabled]):not([readonly])', "Resource modal fallback focus must select the first enabled editable form control.");
requireIncludes(files.resourceConsole, '[tabindex]:not([tabindex="-1"]):not([disabled]):not([readonly]):not([aria-disabled="true"])', "Resource modal fallback focus must exclude disabled generic focus targets.");
requireIncludes(files.resourceConsole, "fieldInstance.focus({ preventScroll: true });", "Resource modal instance focus must prevent scroll jumps.");
requireIncludes(files.resourceConsole, "fieldControl?.focus({ preventScroll: true });", "Resource modal fallback focus must prevent scroll jumps.");
requireNotIncludes(files.resourceConsole, "document.getElementById(firstField.key)", "Resource modal fallback focus must not depend on a global field id.");

requireIncludes(files.client, "export type AdminResourceFieldRelation", "Admin API client must expose resource field relation metadata.");
requireIncludes(files.client, "relation?: AdminResourceFieldRelation", "AdminResourceField must carry optional relation metadata.");
requireIncludes(files.client, 'display?: "select" | "tree"', "AdminResourceFieldRelation must expose tree relation display metadata.");
requireIncludes(files.client, "parentField?: string", "AdminResourceFieldRelation must expose tree relation parent fields.");
requireIncludes(files.client, "parentValue?: string", "AdminResourceField options must carry dynamic tree parent values.");
requireIncludes(files.client, "export type AdminResourceAction", "Admin API client must expose custom resource action metadata.");
requireIncludes(files.client, "export type AdminResourcePanel", "Admin API client must expose custom resource panel metadata.");
requireIncludes(files.client, "actions?: AdminResourceAction[]", "AdminResourceSchema must carry optional custom action metadata.");
requireIncludes(files.client, "panels?: AdminResourcePanel[]", "AdminResourceSchema must carry optional custom panel metadata.");
requireIncludes(files.client, "formLayout?: AdminResourceFormLayout", "AdminResourceSchema must carry the schema-driven form layout preset.");
requireIncludes(files.client, "executeAdminResourceAction", "Admin API client must expose a unified custom resource action executor.");
requireIncludes(files.client, "action.route.replace(\":id\", encodeURIComponent(record.id))", "Custom resource action executor must safely substitute record ids in declared routes.");
for (const key of [
  "requestAdminPolicyReview",
  "approveAdminPolicyReview",
  "rejectAdminPolicyReview",
  "exportAdminPolicyReviews",
]) {
  requireIncludes(files.client, key, `Admin API client must expose ${key}.`);
}

requireIncludes(files.uiIndex, 'export * from "./PlatformTreeSelect"', "PlatformTreeSelect must be exported as a shared platform primitive.");
requireIncludes(files.uiIndex, 'export * from "./PlatformResourceForm"', "PlatformResourceForm must be exported as a shared platform primitive.");
requireIncludes(files.uiIndex, 'export * from "./formSlotRegistry"', "Runtime form slot registry must be exported as a shared platform primitive.");
requireIncludes(files.resourceForm, "export function PlatformResourceForm", "PlatformResourceForm must exist as the shared schema form primitive.");
requireIncludes(files.resourceForm, "layoutPreset = \"single-column\"", "PlatformResourceForm must default to the single-column layout preset.");
requireIncludes(files.resourceForm, "fieldControl?: (field: TField, defaultControl: ReactNode) => ReactNode", "PlatformResourceForm must support controlled source-level field control slots.");
requireIncludes(files.resourceForm, "sectionBefore?:", "PlatformResourceForm must support controlled source-level section-before slots.");
requireIncludes(files.resourceForm, "sectionAfter?:", "PlatformResourceForm must support controlled source-level section-after slots.");
requireIncludes(files.resourceForm, "sidePreview?: ReactNode", "PlatformResourceForm must support controlled side preview slots.");
requireIncludes(files.resourceForm, 'layoutPreset === "side-detail-preview"', "PlatformResourceForm must render the side-detail-preview layout explicitly.");
requireIncludes(files.resourceForm, "getValuePropName?.(field)", "PlatformResourceForm must allow AntD valuePropName binding per field.");
requireIncludes(files.formSlotRegistry, "createAdminFormSlotRegistry", "Runtime form slot registry must expose createAdminFormSlotRegistry.");
requireIncludes(files.formSlotRegistry, "defaultAdminFormSlotRegistry", "Runtime form slot registry must expose default platform slots.");
requireIncludes(files.formSlotRegistry, "platform.record-summary", "Runtime form slot registry must include the platform record summary slot.");
requireIncludes(files.formSlotRegistry, "platform.permission-summary", "Runtime form slot registry must include the platform permission summary slot.");
requireIncludes(files.formSlotRegistry, "platform.localized-preview", "Runtime form slot registry must include the localized preview slot.");
requireIncludes(files.resourceConsole, "normalizeFormLayoutPreset(schema.formLayout, formFields)", "GenericResourceConsole must map schema formLayout to platform form presets.");
requireIncludes(files.resourceConsole, "width={formModalWidth(formLayoutPreset)}", "GenericResourceConsole must size form modals according to the active form layout.");
requireIncludes(files.resourceConsole, "buildRuntimeFormSlots", "GenericResourceConsole must build runtime form slots from schema descriptors.");
requireIncludes(files.resourceConsole, "schema.runtimeSlots", "GenericResourceConsole must consume schema-declared runtime slots.");
requireIncludes(files.resourceConsole, "defaultAdminFormSlotRegistry", "GenericResourceConsole must use the default platform runtime slot registry.");
requireIncludes(files.treeSelect, "export function PlatformTreeSelect", "PlatformTreeSelect must exist as the shared tree relation control.");
requireIncludes(files.treeSelect, "platformPopupContainer", "PlatformTreeSelect dropdowns must render inside the platform popup container.");
requireIncludes(files.treeSelect, "treeDataFromOptions", "PlatformTreeSelect must build nested tree data from parentValue metadata.");

requireIncludes(files.primitives, "forceRender = true", "AdminFormModal must force render by default so hidden forms can receive edit values.");
requireIncludes(files.primitives, "destroyOnHidden = true", "AdminFormModal must destroy hidden forms by default to avoid stale field state.");

requireIncludes(files.resourceConsole, "const provider = dataProvider();", "GenericResourceConsole relation options must use the Refine data provider.");
requireIncludes(files.resourceConsole, "provider.getList<AdminResourceRecord>", "Relation option loading must query target resources through dataProvider.getList.");
requireNotIncludes(files.resourceConsole, "queryAdminResource(", "GenericResourceConsole must not bypass Refine CRUD for relation option loading.");
requireIncludes(files.resourceConsole, "dictionary.relationOptionsLoadFailed", "Relation option failures must use localized i18n copy.");
requireIncludes(files.resourceConsole, "mergeRelationOptions(currentSchema, results)", "Relation option loading must merge dynamic options back into the schema.");
requireIncludes(files.resourceConsole, "relation.parentField", "GenericResourceConsole relation options must preserve tree parent values.");
requireIncludes(files.resourceConsole, "relation.pathField", "GenericResourceConsole relation options must preserve tree path values.");
requireIncludes(files.resourceConsole, "isTreeRelationField(field)", "GenericResourceConsole must route tree relations through shared tree controls.");
requireIncludes(files.resourceConsole, "<PlatformTreeSelect", "GenericResourceConsole must render tree relation fields with PlatformTreeSelect.");
requireIncludes(files.resourceConsole, "function FieldInput({ field, language, ...controlProps }", "FieldInput must accept AntD Form-injected control props.");
requireCountAtLeast(files.resourceConsole, "{...(controlProps as ComponentProps<typeof Select>)}", 2, "FieldInput must pass Form control props into both select and multiselect fields.");
requireCountAtLeast(files.resourceConsole, "{...(controlProps as ComponentProps<typeof PlatformTreeSelect>)}", 2, "FieldInput must pass Form control props into tree select fields.");
for (const [component, label] of [
  ["Input.TextArea", "textarea"],
  ["Switch", "switch"],
  ["InputNumber", "number"],
  ["Input>", "input"],
]) {
  requireIncludes(files.resourceConsole, `controlProps as ComponentProps<typeof ${component}`, `FieldInput must pass Form control props into ${label} fields.`);
}
requireIncludes(files.resourceConsole, "<PlatformResourceForm", "GenericResourceConsole must render schema forms through PlatformResourceForm.");
requireIncludes(files.resourceConsole, 'getValuePropName={(field) => (field.type === "switch" ? "checked" : undefined)}', "Switch form fields must bind AntD Form values through checked.");
requireIncludes(files.resourceConsole, "if (!modalOpen)", "GenericResourceConsole must only reset or set form values when the form modal is open.");
requireIncludes(files.resourceConsole, "form.setFieldsValue(formValuesFromRecord(editingRecord, formFields))", "Editing a resource must hydrate form values from the selected record.");
requireIncludes(files.resourceConsole, "field.type === \"multiselect\"", "Form hydration must preserve multiselect arrays for relation fields.");
requireIncludes(files.resourceConsole, "field.localizable", "GenericResourceConsole must render any schema-declared localizable field using the active language.");
requireIncludes(files.resourceConsole, "localizedRecordValue(record, field.key, language)", "GenericResourceConsole must not limit localized display to name/description fields.");
requireIncludes(files.resourceConsole, "uniqueResourceFields(schema.fields)", "GenericResourceConsole must normalize duplicate schema field keys before rendering forms.");
requireIncludes(files.resourceConsole, "schema.actions", "GenericResourceConsole must render schema-declared resource actions.");
requireIncludes(files.resourceConsole, "schema.panels", "GenericResourceConsole must render schema-declared resource panels.");
requireIncludes(files.resourceConsole, "rowActions={renderResourceRowActions}", "GenericResourceConsole must attach default row commands through PlatformDataTable rowActions.");
requireIncludes(files.resourceConsole, "Dropdown", "GenericResourceConsole must use AntD Dropdown for overflow custom row actions.");
requireIncludes(files.resourceConsole, "Tabs", "GenericResourceConsole detail drawer must render tabbed panels.");
requireIncludes(files.resourceConsole, "executeAdminResourceAction(", "GenericResourceConsole must execute schema-declared routed actions through the shared API client.");
requireIncludes(files.resourceConsole, "dictionary.customActionSucceeded", "Routed custom actions must report localized success feedback.");
requireIncludes(files.resourceConsole, "action.confirm", "Routed custom actions must honor schema-declared confirmations.");
requireIncludes(files.resourceConsole, "dictionary.customActionUnavailable", "Unsupported custom actions must use localized unavailable copy.");
requireIncludes(files.resourceConsole, "dictionary.customPanelEmpty", "Unsupported or empty custom panels must use localized empty copy.");
for (const key of [
  "PolicyReviewConsole",
  "PlatformDataTable",
  "AdminMetricStrip",
  "queryAdminResource(\"policy-reviews\"",
  "queryAdminResource(\"audit-logs\"",
  'sort: [{ field: "createdAt", order: "desc" }]',
  "requestAdminPolicyReview",
  "approveAdminPolicyReview",
  "rejectAdminPolicyReview",
  "exportAdminPolicyReviews",
  "dictionary.policyReviewTitle",
  "dictionary.policyReviewRejectDescription",
]) {
  requireIncludes(files.policyReview, key, `PolicyReviewConsole must keep ${key}.`);
}
requireNotIncludes(files.policyReview, "fetch(", "PolicyReviewConsole must use the shared platform API client instead of direct fetch.");
requireIncludes(files.table, 'type?: "text" | "select" | "treeSelect"', "PlatformDataTable filters must keep treeSelect support.");
requireIncludes(files.table, "type === \"treeSelect\"", "PlatformDataTable must render tree relation filters with PlatformTreeSelect.");

requireIncludes(files.pagination, "platformPopupContainer", "PlatformPaginationBar selects must render inside the platform popup container.");
requireIncludes(files.pagination, "showQuickJumper", "PlatformPaginationBar must keep quick jumper support.");
requireIncludes(files.pagination, "showSizeChanger", "PlatformPaginationBar must keep page-size changer support.");
requireIncludes(files.pagination, "clampPage", "PlatformPaginationBar must clamp page changes.");

for (const [token, expected] of [
  ["--table-head-height", "30px"],
  ["--table-row-height", "30px"],
  ["--table-head-font-size", "11.75px"],
  ["--table-body-font-size", "12px"],
  ["--pagination-bar-height", "34px"],
  ["--pagination-item-size", "22px"],
  ["--pagination-font-size", "11px"],
]) {
  requireRegex(files.styles, new RegExp(`${escapeRegExp(token)}:\\s*${escapeRegExp(expected)};`), `styles.css must keep ${token}: ${expected}.`);
}
requireIncludes(files.styles, ".platform-pagination-bar .ant-pagination.ant-pagination-mini", "styles.css must override AntD mini pagination inside the platform pagination bar.");
requireIncludes(files.styles, ".platform-table-row-actions", "styles.css must keep row action slot alignment for PlatformDataTable.");
requireIncludes(files.styles, "grid-template-columns: minmax(var(--pagination-side-width), 1fr) max-content minmax(var(--pagination-side-width), 1fr);", "PlatformPaginationBar must keep a true centered pager axis on desktop.");
requireIncludes(files.styles, "padding: 4px calc(var(--pagination-side-width) + 18px);", "Direct AntD table pagination must reserve side zones so the pager stays centered.");
requireIncludes(files.styles, "@media (max-width: 767px)", "styles.css must keep mobile responsive rules.");
requireIncludes(files.styles, ".platform-pagination-main", "styles.css must keep centered pagination main region.");
requireIncludes(files.styles, ".platform-mobile-list", "styles.css must keep mobile list/card styling.");
requireIncludes(files.styles, ".resource-form-slot", "styles.css must keep source-level resource form slot containers.");
requireIncludes(files.styles, ".resource-form.layout-two-column-density .resource-form-fields", "styles.css must keep the dense two-column resource form layout preset.");
requireIncludes(files.styles, ".resource-form-preview-rail", "styles.css must keep the side-detail-preview rail styling.");
requireIncludes(files.styles, ".system-settings-drawer", "styles.css must keep settings drawer styling.");
requireIncludes(files.styles, ".page-heading p.ant-typography", "styles.css must keep compact mobile page descriptions.");
requireIncludes(files.styles, "-webkit-line-clamp: 2", "Mobile page descriptions must stay capped so lists enter the first screens faster.");
requireIncludes(files.styles, ".resource-page-tools {\n    flex: none;", "Mobile resource page tools must not carry desktop flex-basis into vertical layout.");
requireRegex(
  files.styles,
  /@media\s*\(max-width:\s*1023px\)[\s\S]*\.platform-shell\.sider-collapsed:not\(\.layout-top\):not\(\.layout-split\)[\s\S]*grid-template-columns:\s*minmax\(0,\s*1fr\);/,
  "Mobile responsive rules must explicitly override collapsed sidebar grid columns.",
);
requireIncludes(files.styles, ".dashboard-chart {\n    height: 150px;", "Mobile dashboard chart must keep a compact height.");
requireIncludes(files.styles, ".health-panel .ant-progress", "Mobile health panel must keep compact progress styling.");
requireIncludes(files.styles, ".platform-skip-link", "styles.css must expose skip-link focus behavior.");
requireRegex(files.styles, /:focus-visible[\s\S]*outline:\s*2px solid var\(--primary\)/, "Visible focus must be a default platform behavior.");
requireRegex(files.styles, /@media\s*\(prefers-reduced-motion:\s*reduce\)/, "styles.css must respect reduced motion.");
requireRegex(
  files.styles,
  /@media\s*\(prefers-reduced-motion:\s*reduce\)\s*\{[\s\S]*:where\(\.ant-modal-root, \.ant-drawer-root, \.ant-dropdown, \.ant-popover, \.ant-tooltip, \.ant-select-dropdown\),[\s\S]*:where\(\.ant-modal-root, \.ant-drawer-root, \.ant-dropdown, \.ant-popover, \.ant-tooltip, \.ant-select-dropdown\) \*,[\s\S]*:where\(\.ant-modal-root, \.ant-drawer-root, \.ant-dropdown, \.ant-popover, \.ant-tooltip, \.ant-select-dropdown\) \*::before,[\s\S]*:where\(\.ant-modal-root, \.ant-drawer-root, \.ant-dropdown, \.ant-popover, \.ant-tooltip, \.ant-select-dropdown\) \*::after[\s\S]*transition-duration:\s*0\.01ms !important;/,
  "Reduced motion must cover used body-portaled AntD motion roots.",
);
requireRegex(files.styles, /@media\s*\(max-width:\s*1023px\)[\s\S]*min-height:\s*44px/, "Responsive shell controls must use 44px minimum targets.");
requireRegex(
  files.styles,
  /@media\s*\(max-width:\s*1023px\)[\s\S]*\.mobile-global-search\s*\{[^}]*min-height:\s*44px;/,
  "Mobile Drawer search must use a 44px minimum target below the desktop breakpoint.",
);
requireCssRule(mobileStyles, ".platform-data-table-panel .platform-table-search", ["min-height: 44px;"], "Mobile resource search must expose a 44px touch target.");
requireCssRule(mobileStyles, ".platform-data-table-panel .table-actions .ant-btn", ["min-width: 44px;", "min-height: 44px;"], "Mobile resource table actions must expose 44px touch targets.");
requireCssRule(
  mobileStyles,
  ".platform-pagination-main :where(.ant-pagination-prev, .ant-pagination-item, .ant-pagination-jump-prev, .ant-pagination-jump-next, .ant-pagination-next, .ant-pagination-item-link, a)",
  ["min-width: 44px;", "min-height: 44px;"],
  "Mobile pagination main controls must expose 44px touch targets.",
);
requireCssRule(mobileStyles, ".platform-pagination-jumper .ant-input-number", ["width: 44px;", "min-width: 44px;", "min-height: 44px;"], "Mobile pagination quick jumper must expose a 44px touch target.");
for (const [selector, label] of [
  [".system-settings-drawer .ant-drawer-close", "close control"],
  [".settings-tabs .ant-tabs-tab", "tab"],
  [".settings-tabs .ant-tabs-tab-btn", "tab button"],
  [".settings-tabs .ant-tabs-nav-more", "overflow control"],
]) {
  requireCssRule(mobileStyles, selector, ["min-width: 44px;", "min-height: 44px;"], `Mobile settings Drawer ${label} must expose a 44px touch target.`);
}
requireOrder(
  files.styles,
  "@media (max-width: 767px)",
  ".dashboard-hero-panel .page-eyebrow",
  "Dashboard eyebrow hiding must be scoped to the mobile breakpoint.",
);
requireCountExactly(
  files.styles,
  ".dashboard-hero-panel .page-eyebrow",
  1,
  "Dashboard eyebrow hiding must have exactly one mobile-only rule.",
);

if (failures.length > 0) {
  console.error("Admin UI contract validation failed:");
  for (const failure of failures) {
    console.error(`- ${failure}`);
  }
  process.exit(1);
}

console.log("Admin UI contract validation passed.");

function readSource(relativePath) {
  return readFileSync(join(root, relativePath), "utf8");
}

function requireIncludes(source, needle, message) {
  if (!source.includes(needle)) {
    failures.push(message);
  }
}

function requireNotIncludes(source, needle, message) {
  if (source.includes(needle)) {
    failures.push(message);
  }
}

function requireRegex(source, pattern, message) {
  if (!pattern.test(source)) {
    failures.push(message);
  }
}

function requireCssRule(source, selector, declarations, message) {
  const match = source.match(new RegExp(`${escapeRegExp(selector)}\\s*\\{([^}]*)\\}`));
  if (!match || declarations.some((declaration) => !match[1].includes(declaration))) {
    failures.push(message);
  }
}

function extractCssBlock(source, marker) {
  const markerIndex = source.indexOf(marker);
  if (markerIndex === -1) return "";
  const openIndex = source.indexOf("{", markerIndex);
  if (openIndex === -1) return "";
  let depth = 1;
  for (let index = openIndex + 1; index < source.length; index += 1) {
    if (source[index] === "{") depth += 1;
    if (source[index] === "}") depth -= 1;
    if (depth === 0) return source.slice(openIndex + 1, index);
  }
  return "";
}

function requireOrder(source, first, second, message) {
  const firstIndex = source.indexOf(first);
  const secondIndex = source.indexOf(second);
  if (firstIndex === -1 || secondIndex === -1 || firstIndex > secondIndex) {
    failures.push(message);
  }
}

function requireCountAtLeast(source, needle, expectedCount, message) {
  if (source.split(needle).length - 1 < expectedCount) {
    failures.push(message);
  }
}

function requireCountExactly(source, needle, expectedCount, message) {
  if (source.split(needle).length - 1 !== expectedCount) {
    failures.push(message);
  }
}

function escapeRegExp(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}
