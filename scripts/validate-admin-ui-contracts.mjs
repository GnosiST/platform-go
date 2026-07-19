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
  login: readSource("admin/src/platform/auth/AdminLoginView.tsx"),
  oidcPolicy: readSource("admin/src/platform/auth/oidcPolicy.ts"),
  capabilityConsole: readSource("admin/src/platform/capabilities/CapabilityConsole.tsx"),
  capabilityMetadata: readSource("admin/src/platform/capabilities/metadata.ts"),
  client: readSource("admin/src/platform/api/client.ts"),
  dataProvider: readSource("admin/src/platform/refine/dataProvider.ts"),
  dashboard: readSource("admin/src/platform/dashboard/DashboardHome.tsx"),
  designProvider: readSource("admin/src/platform/ui/AdminDesignProvider.tsx"),
  organizationRBAC: readSource("admin/src/platform/api/organizationRBAC.ts"),
  sessionExpiry: readSource("admin/src/platform/api/sessionExpiry.ts"),
  i18n: readSource("admin/src/platform/i18n.ts"),
  primitives: readSource("admin/src/platform/ui/AdminPrimitives.tsx"),
  resourceConsole: readSource("admin/src/platform/resources/GenericResourceConsole.tsx"),
  relationOptionSearch: readSource("admin/src/platform/resources/relationOptionSearch.ts"),
  resourceExperience: readSource("admin/src/platform/resources/resourceExperience.ts"),
  organizationUserExperience: readSource("admin/src/platform/resources/organizationUserExperience.tsx"),
  roleGovernance: readSource("admin/src/platform/resources/RoleGovernanceConsole.tsx"),
  roleGovernanceRuntime: readSource("admin/src/platform/resources/roleGovernanceRuntime.ts"),
  rolePermissionWriteMode: readSource("admin/src/platform/resources/rolePermissionWriteMode.ts"),
  rolePermissionWorkflow: readSource("admin/src/platform/resources/rolePermissionWorkflow.ts"),
  roleManagementNavigation: readSource("admin/src/platform/resources/roleManagementNavigation.ts"),
  sessionConsole: readSource("admin/src/platform/resources/SessionConsole.tsx"),
  permissionGovernance: readSource("admin/src/platform/resources/PermissionGovernanceConsole.tsx"),
  menuGovernance: readSourceOptional("admin/src/platform/resources/MenuGovernanceConsole.tsx"),
  menuGovernanceRuntime: readSourceOptional("admin/src/platform/resources/menuGovernanceRuntime.ts"),
  menuGovernanceValidation: readSourceOptional("admin/src/platform/resources/menuGovernanceValidation.ts"),
  resourceRoute: readSource("admin/src/platform/refine/ResourceRoutePage.tsx"),
  sensitiveRevealModal: readSource("admin/src/platform/resources/SensitiveFieldRevealModal.tsx"),
  sensitiveRevealOIDC: readSource("admin/src/platform/security/sensitiveRevealOIDC.ts"),
  shell: readSource("admin/src/platform/shell/AdminShell.tsx"),
  settings: readSource("admin/src/platform/ui/SystemSettingsDrawer.tsx"),
  table: readSource("admin/src/platform/ui/PlatformDataTable.tsx"),
  pagination: readSource("admin/src/platform/ui/PlatformPaginationBar.tsx"),
  policyReview: readSource("admin/src/platform/policy-review/PolicyReviewConsole.tsx"),
  resourceForm: readSource("admin/src/platform/ui/PlatformResourceForm.tsx"),
  formSlotRegistry: readSource("admin/src/platform/ui/formSlotRegistry.tsx"),
  treeSelect: readSource("admin/src/platform/ui/PlatformTreeSelect.tsx"),
  treeTransfer: readSource("admin/src/platform/ui/PlatformTreeTransfer.tsx"),
  treeWorkbench: readSource("admin/src/platform/ui/AdminTreeWorkbench.tsx"),
  uiIndex: readSource("admin/src/platform/ui/index.ts"),
  styles: readSource("admin/src/styles.css"),
};

const failures = [];
const roleAuthorizationOpen = sourceRange(files.roleGovernance, "const openAuthorization", "const saveAuthorization");
const roleAuthorizationSave = sourceRange(files.roleGovernance, "const saveAuthorization", "const openMenus");
const roleAuthorizationModal = sourceRange(files.roleGovernance, "function AuthorizationModal", "function MenuVisibilityModal");
const roleMenuModal = sourceRange(files.roleGovernance, "function MenuVisibilityModal", "async function assignmentPermissionRecords");
const legacyRolePermissionInput = sourceRange(files.rolePermissionWorkflow, "function legacyRolePermissionInput", "function uniqueSorted");
const mobileStyles = extractCssBlock(files.styles, "@media (max-width: 767px)");
const tabletWorkbenchStyles = extractCssBlock(files.styles, "@media (max-width: 1023px)");
const tabletLoginStyles = extractCssBlock(files.styles, "@media (max-width: 1024px)");

requireIncludes(files.app, "readStoredUIConfig", "App must keep persisted admin UI configuration.");
requireIncludes(files.app, "writeStorageValue(adminPreferenceStorageKeys.ui", "App must persist admin UI configuration changes.");
requireIncludes(files.app, "defaultAdminUIConfig", "App must fall back to the shared default admin UI config.");
requireIncludes(files.app, "disableTelemetry: true", "The reusable Admin foundation must disable Refine third-party telemetry by default.");
requireIncludes(files.designProvider, "document.body.dataset.theme = themeName;", "Admin theme tokens must propagate to body-portaled overlays.");
requireIncludes(files.designProvider, "delete document.body.dataset.theme;", "Admin theme propagation must clean up its body theme marker.");
requireIncludes(files.app, "PolicyReviewConsole", "App must mount the policy-review custom governance console when the resource is enabled.");
requireIncludes(files.app, 'resource.route !== "/policy-reviews"', "Generic resource routing must not also mount policy-reviews when the custom console is active.");
requireIncludes(files.app, "projectRoleManagementNavigation", "App must use the approved role-management navigation projection.");
requireIncludes(files.app, "resolveRoleManagementActiveRoute", "App must map legacy role URLs to the projected navigation entry.");
requireRegex(
  files.app,
  /const navigationResources = useMemo\([\s\S]*?projectRoleManagementNavigation\(resources,/,
  "App must derive visible navigation from the complete authorized resource list.",
);
requireIncludes(
  files.app,
  "const navigationActiveRoute = resolveRoleManagementActiveRoute(activeRoute, navigationResources);",
  "App must resolve the active shell route against projected role-management navigation.",
);
requireRegex(
  files.app,
  /<AdminShell[^>]*resources=\{navigationResources\}[^>]*>/,
  "AdminShell must receive only the projected role-management navigation resources.",
);
requireRegex(
  files.app,
  /<AdminShell[^>]*activeRoute=\{navigationActiveRoute\}[^>]*>/,
  "AdminShell must highlight the projected role-management route.",
);
requireIncludes(
  files.app,
  "const refineResources = useMemo(() => resources.map(resourceDefinitionToRefineResource), [resources]);",
  "Refine resource registration must retain the complete authorized resource list.",
);
requireRegex(
  files.app,
  /<PlatformRoutePages\s+activeRoute=\{activeRoute\}[\s\S]*?permissions=\{permissions\}\s+deniedPermissions=\{deniedPermissions\}\s+exportWatermark=\{uiConfig\.watermark && uiConfig\.watermarkScopes\.includes\("export"\)\}\s+resources=\{resources\}\s+session=\{session\}/,
  "Platform route pages must retain the complete authorized resource list with its permissions.",
);
requireIncludes(files.app, "resources.some((resource) => resource.route === locationRoute)", "Route validation must retain the complete authorized resource list.");
requireIncludes(files.app, "sensitiveRevealResourceRoute(resources,", "Sensitive-field resume routing must retain the complete authorized resource list.");
requireIncludes(files.dashboard, "projectRoleManagementNavigation(resources,", "Dashboard role management must use the approved navigation projection.");
requireIncludes(files.dashboard, "const roleManagementResource = selectRoleManagementNavigationResource(", "Dashboard role management must use the shared deterministic resource selector.");
requireIncludes(files.dashboard, "label: dictionary.roleManagement", "Dashboard role management must use the shared visible label.");
requireIncludes(files.dashboard, "route: roleManagementResource.route", "Dashboard role management must target the projected authorized role resource.");
requireNotIncludes(files.dashboard, '{ key: "roles", label: dictionary.roles, route: "/roles"', "Dashboard must not restore the duplicate roles-only quick action.");
requireIncludes(files.roleManagementNavigation, 'const ROLES_ROUTE = "/roles";', "Role-management navigation must preserve the roles route.");
requireIncludes(files.roleManagementNavigation, 'const ROLE_GROUPS_ROUTE = "/role-groups";', "Role-management navigation must preserve the role-groups route.");
requireIncludes(files.roleManagementNavigation, "export function selectRoleManagementNavigationResource", "Role-management navigation must expose the shared deterministic resource selector.");
requireIncludes(files.roleManagementNavigation, 'Omit<T, "title"> & { title: RoleManagementNavigationTitle }', "Role-management projection must expose the replaced title type explicitly.");
requireIncludes(files.roleManagementNavigation, "): ProjectedRoleManagementNavigationResource<T>[]", "Role-management projection must return its explicit replaced-title type.");
requireNotIncludes(files.roleManagementNavigation, "Object.assign", "Role-management projection must not hide a narrowed title subtype through Object.assign.");
requireNotIncludes(files.roleManagementNavigation, "as T", "Role-management projection must not cast the replaced title back to an arbitrary input subtype.");
requireIncludes(files.resourceRoute, 'resource.route === "/roles" || resource.route === "/role-groups"', "Both legacy role routes must retain the shared route adapter.");
requireIncludes(files.resourceRoute, 'resource.route === "/permissions"', "The permissions route must use the dedicated permission governance console.");
requireIncludes(files.resourceRoute, "<PermissionGovernanceConsole", "The permissions route must render PermissionGovernanceConsole instead of GenericResourceConsole.");
requireCountExactly(files.i18n, "roleManagement:", 2, "Role management navigation must declare matching Chinese and English dictionary keys.");
requireIncludes(files.i18n, 'roleManagement: "角色"', "Role navigation must declare the concise Chinese label.");
requireIncludes(files.i18n, 'roleManagement: "Roles"', "Role navigation must declare the concise English label.");
requireIncludes(files.i18n, 'roleGovernanceTitle: "角色管理"', "Shared role governance page must display Role Management as its Chinese H1.");
requireIncludes(files.i18n, 'roleGovernanceTitle: "Role Management"', "Shared role governance page must display Role Management as its English H1.");
requireIncludes(files.roleGovernance, "title={dictionary.roleGovernanceTitle}", "Shared role governance page must keep its localized H1 binding.");
requireIncludes(files.client, "export class AdminAPIError", "Admin API failures must expose typed status codes.");
requireIncludes(files.client, "type PlatformErrorCode", "Admin API failures must consume the generated PlatformErrorCode type.");
requireIncludes(files.client, "type PlatformErrorBody", "Admin API responses must consume the generated PlatformErrorBody type.");
requireIncludes(files.client, "isPlatformErrorCode", "Admin API responses must validate upstream error-code membership at runtime.");
requireIncludes(files.client, "readonly requestId: string", "Admin API errors must carry the server request ID.");
requireIncludes(files.client, "readonly traceId: string", "Admin API errors must carry the server trace ID.");
requireIncludes(files.client, 'response.headers.get("X-Request-ID")', "Malformed Admin errors must read request correlation from response headers.");
requireIncludes(files.client, 'response.headers.get("traceparent")', "Malformed Admin errors must read trace correlation from traceparent.");
requireIncludes(files.client, 'code: isPlatformErrorCode(error.code) ? error.code : "INTERNAL_ERROR"', "Unknown Admin error codes must normalize to INTERNAL_ERROR.");
requireIncludes(files.client, "ADMIN_SESSION_EXPIRED_EVENT", "The shared client must expose the session-expired event contract.");
requireIncludes(files.client, "shouldExpireAdminSession", "The shared client must centralize session-expiry decisions.");
requireIncludes(files.client, "handleUnauthorizedResponse(response.status, requestToken, error.code)", "Session expiry must consider the structured API error code.");
requireIncludes(files.sessionExpiry, 'errorCode !== "ADMIN_SENSITIVE_REVEAL_VERIFICATION_FAILED"', "Failed sensitive reveal verification must preserve the authenticated Admin session.");
requireIncludes(files.sessionExpiry, "errorCode?: PlatformErrorCode", "Session expiry must consume the generated PlatformErrorCode type.");
requireIncludes(files.client, "statusCode", "Admin API errors must carry HTTP status.");
requireIncludes(files.client, "dispatchEvent", "Stored-token 401 responses must notify the app.");
requireIncludes(files.app, "ADMIN_SESSION_EXPIRED_EVENT", "App must listen for shared session expiry.");
requireIncludes(files.app, "dictionary.sessionExpired", "Session expiry feedback must be localized.");
requireIncludes(files.sessionExpiry, "currentToken === requestToken", "Session expiry must clear only the exact token used by the failed request.");
requireNotIncludes(files.client, "hadToken", "Session expiry handling must retain the exact request token instead of a boolean token flag.");
requireIncludes(files.client, 'const { auth = "stored-token", ...fetchInit } = init;', "Request must separate the platform auth mode from native fetch options.");
requireNotIncludes(files.client, "...init,", "Platform-only request options must not be forwarded to fetch.");
requireIncludes(files.client, 'return request<AuthProviderList>("/auth/providers", { auth: "none" });', "Auth provider discovery must explicitly avoid stored-token authentication.");
requireRegex(
  files.client,
  /request<AuthLoginResult>\("\/auth\/login",\s*\{[\s\S]*?auth:\s*"none"/,
  "Auth login must explicitly avoid stored-token authentication.",
);
requireIncludes(files.client, "audiences: string[];", "AuthProvider must expose its declared audiences to the Admin client.");
requireIncludes(files.login, "filterAdminAuthProviders(providers)", "Admin login must consume provider audiences before selection and rendering.");
requireIncludes(files.oidcPolicy, "assertAdminAuthProvider(provider);", "OIDC start must reject providers without the Admin audience.");
requireIncludes(files.client, "export function startAdminAuthProvider", "The Admin client must expose provider-start support.");
requireRegex(
  files.client,
  /request<AuthProviderStartResult>\(`\/auth\/providers\/\$\{encodeURIComponent\(provider\)\}\/start`,\s*\{[\s\S]*?auth:\s*"none"[\s\S]*?JSON\.stringify\(\{ codeChallenge \}\)/,
  "Admin provider start must post the PKCE challenge without stored-token authentication.",
);
requireIncludes(files.authProvider, "crypto.getRandomValues(new Uint8Array(size))", "OIDC login must generate verifier bytes with Web Crypto.");
requireIncludes(files.authProvider, 'crypto.subtle.digest("SHA-256"', "OIDC login must derive an S256 challenge with Web Crypto.");
requireIncludes(files.authProvider, "window.sessionStorage.setItem", "OIDC pending transactions must use tab-scoped sessionStorage.");
requireRegex(
  files.oidcPolicy,
  /JSON\.stringify\(\{\s*provider:\s*provider\.id,\s*state:\s*started\.state,\s*codeVerifier,\s*expiresAt:\s*started\.expiresAt,?\s*\}\)/,
  "OIDC pending transactions must store only provider, state, codeVerifier, and expiresAt.",
);
requireIncludes(files.oidcPolicy, "callbackState !== pending.state", "OIDC callback state must use an exact comparison.");
requireIncludes(files.authProvider, "window.sessionStorage.removeItem", "OIDC terminal and recovery paths must clear the pending transaction.");
requireIncludes(files.authProvider, "window.history.replaceState", "OIDC callbacks must remove callback values from browser history before exchange.");
requireOrder(
  files.oidcPolicy,
  "dependencies.cleanupURL()",
  "dependencies.readPending()",
  "OIDC callbacks must remove callback values before reading pending transaction state.",
);
requireOrder(
  files.oidcPolicy,
  "dependencies.cleanupURL()",
  "dependencies.exchange({",
  "OIDC callbacks must remove callback values from browser history before exchange.",
);
requireIncludes(files.authProvider, "beginOIDCLoginTransaction(provider,", "The production OIDC start wrapper must use the injected transaction implementation.");
requireIncludes(files.authProvider, "consumePendingOIDCLoginTransaction(search,", "The production OIDC callback wrapper must use the injected transaction implementation.");
requireIncludes(files.authProvider, "allowLoopbackHTTP: import.meta.env.DEV", "Loopback HTTP authorization URLs must be enabled only in verified development mode.");
requireIncludes(files.authProvider, "state?: string; codeVerifier?: string", "Refine login input must forward OIDC state and verifier values.");
requireIncludes(files.app, "search={location.search}", "App must pass the current callback search string to the login view.");
requireIncludes(files.app, "if (!getAuthToken() || !session || loading)", "Unauthenticated callback routes must not be normalized before OIDC URL cleanup.");
requireIncludes(files.login, 'selectedProvider.kind === "demo"', "The username form must render only for the demo provider.");
requireIncludes(files.login, 'selectedProvider.kind === "oidc"', "The OIDC action must render only for an OIDC provider.");
requireIncludes(files.login, 'className="login-oidc-action"', "OIDC providers must expose one full-width login action.");
requireNotIncludes(files.login, "Input.Password", "OIDC login must not retain the disabled password field.");
requireIncludes(files.login, 'aria-live="polite"', "OIDC callback progress and failure must use a polite live region.");
requireIncludes(files.login, 'className="login-error-heading"', "OIDC callback failures must expose a stable error heading.");
requireIncludes(files.login, "tabIndex={-1}", "The OIDC callback error heading must be programmatically focusable.");
requireIncludes(files.login, "focus({ preventScroll: true })", "OIDC callback failure must focus its heading without a scroll jump.");
requireIncludes(files.login, 'className="login-recovery-action"', "OIDC callback failures must provide an explicit recovery action.");
requireRegex(
  files.login,
  /const submit = async \(values: LoginFormValues\) => \{\s*if \(!submissionLockRef\.current\.acquire\(\)\) return;/,
  "Demo login must acquire the synchronous submission lock before its first await.",
);
requireRegex(
  files.login,
  /const startOIDC = async \(\) => \{\s*if \(!submissionLockRef\.current\.acquire\(\)\) return;/,
  "OIDC start must acquire the synchronous submission lock before its first await.",
);
requireIncludes(files.login, "useRef(createSubmissionLock())", "Admin login must use the executable synchronous submission lock helper.");
requireIncludes(files.login, "useRef(createSingleUseGuard())", "Admin callback processing must use the executable single-use guard for StrictMode replay.");
requireIncludes(files.login, "callbackGuardRef.current.acquire()", "Admin callback processing must acquire its single-use guard before exchange.");
requireIncludes(files.oidcPolicy, "validateOIDCAuthorizationURL(started.authorizationUrl,", "OIDC start must validate the authorization URL before browser navigation.");
requireOrder(
  files.oidcPolicy,
  "validateOIDCAuthorizationURL(started.authorizationUrl,",
  "dependencies.storePending",
  "OIDC authorization URL validation must happen before pending transaction storage.",
);
requireOrder(
  files.oidcPolicy,
  "validateOIDCAuthorizationURL(started.authorizationUrl,",
  "dependencies.navigate",
  "OIDC authorization URL validation must happen before browser navigation.",
);
requireIncludes(files.login, "loginHeadingRef.current?.focus({ preventScroll: true })", "Explicit OIDC recovery must restore focus predictably without scrolling.");
requireIncludes(files.login, "setCallbackFailure(callbackFailureReason(nextError))", "OIDC callback failures must store a stable error category instead of localized copy.");
requireIncludes(files.login, "callbackErrorMessage(dictionary, callbackFailure)", "OIDC callback failure copy must derive from the current dictionary.");
for (const key of [
  "loginOIDCContinue",
  "loginOIDCStarting",
  "loginOIDCCallbackProgress",
  "loginOIDCCallbackFailed",
  "loginOIDCTransactionInvalid",
  "loginOIDCTransactionExpired",
  "loginOIDCRecovery",
]) {
  requireCountExactly(files.i18n, `${key}:`, 2, `Admin login i18n key ${key} must exist in matching Chinese and English dictionaries.`);
}
requireIncludes(files.app, "const [sessionExpired, setSessionExpired] = useState(false);", "App must keep session expiry in stable non-localized state.");
requireIncludes(files.app, "setSessionExpired(true);", "Session expiry recovery must set the stable expiry state.");
requireCountExactly(files.app, "setSessionExpired(true);", 1, "Only the exact-token session-expired event may set App expiry state.");
requireIncludes(
  files.app,
  "setSensitiveRevealOIDCResume(null);\n      clearPendingSensitiveRevealOIDC();",
  "Session expiry must let the reveal callback cleanup remove code and state before mounting the login flow.",
);
requireIncludes(files.app, "sessionExpired ? dictionary.sessionExpired : authError || error", "Session expiry display must use the current localized dictionary and override provider errors.");
requireIncludes(files.app, "setSessionExpired(false);", "Successful login must clear stable session expiry state.");
requireNotIncludes(files.app, "current === dictionary.sessionExpired", "App must not identify session expiry by comparing localized strings.");
requireIncludes(files.app, "hasSensitiveRevealOIDCCallback(window.location.search)", "App must identify reveal callbacks before the login view mounts.");
requireOrder(
  files.app,
  "if (sensitiveRevealOIDCCallbackPending)",
  "if (!getAuthToken() || !session)",
  "Reveal callbacks must remain isolated from the login OIDC callback consumer.",
);
requireRegex(
  files.app,
  /navigate\(\{ pathname: route, search: "", hash: "" \}, \{ replace: true \}\)/,
  "Reveal callbacks must clear callback parameters through React Router after exchange.",
);
requireIncludes(files.resourceRoute, "oidcResume={sensitiveRevealOIDCResume}", "Resource routes must pass reveal resume state only to the mounted resource console.");
requireIncludes(
  files.resourceRoute,
  'experienceKey={resource.route === "/org-units" || resource.route === "/users" ? "organization-user" : undefined}',
  "Organization and user routes must inject the shared organization-user experience.",
);
requireIncludes(files.resourceConsole, "useOrganizationUserExperience({", "GenericResourceConsole must consume the organization-user experience through one stable hook.");
requireIncludes(files.resourceConsole, "experience.submit({ editingRecord, input, values, persist })", "Organization and user writes must pass through the experience submit boundary.");
requireIncludes(files.resourceConsole, "const effectiveCanDelete = canDelete && experience.allowDelete;", "GenericResourceConsole must enforce experience delete policy.");
requireIncludes(files.resourceConsole, "canUpdate && experience.allowStatusToggle", "GenericResourceConsole must enforce experience status policy.");
requireIncludes(files.resourceExperience, "initialValues?: (values: ResourceFormValues, editingRecord: AdminResourceRecord | null) => ResourceFormValues;", "Resource experiences must be able to provide deterministic create and edit initial values.");
requireIncludes(files.organizationUserExperience, 'record ? values : { ...values, tenantCode: "", orgUnitCode: undefined, roles: [] }', "New organization-scoped users must start without an implicitly selected organization, tenant, or role.");
requireIncludes(files.resourceConsole, "for (let currentPage = 1; ; currentPage += 1)", "Organization and user context options must load every generic-resource page.");
requireIncludes(files.resourceConsole, "records.length >= result.total", "Organization and user context pagination must stop only after the full result set is loaded.");
requireIncludes(files.organizationRBAC, "new AdminServiceObjectClient(transport)", "Organization RBAC UI calls must use the generated service-object client.");
requireIncludes(files.organizationRBAC, 'path.startsWith("/api/") ? path.slice(4) : path', "Organization RBAC transport must avoid duplicating the shared /api prefix.");
requireIncludes(files.organizationRBAC, "collectPages(async (page, pageSize)", "Organization role pools and conflict details must collect every service-object page.");
requireIncludes(files.organizationUserExperience, 'form.setFieldValue("tenantCode", derivedTenantCode)', "User tenant must be derived from the selected organization.");
requireIncludes(
  files.organizationUserExperience,
  '<Input readOnly aria-readonly="true" placeholder={dictionary.userDerivedTenantPending} />',
  "Derived user tenant must remain visibly and semantically read-only.",
);
requireIncludes(files.organizationUserExperience, "disabled={!selectedOrgUnitCode || rolePoolLoading}", "User roles must remain disabled until an organization is selected and its role pool is ready.");
requireIncludes(files.organizationUserExperience, 'setFieldError(form, "roles", message)', "Out-of-pool roles must block submission with a field-level error.");
requireIncludes(files.organizationUserExperience, 'aria-describedby="organization-role-pool-status"', "The user role selector must reference its async role-pool status.");
requireIncludes(files.organizationUserExperience, '<div aria-live="polite" id="organization-role-pool-status"', "Role-pool status changes must use a polite live region.");
requireIncludes(files.organizationUserExperience, 'values: omitValue(context.input.values, "roleGroupCodes")', "Generic organization CRUD must not carry role-group bindings.");
requireIncludes(files.organizationUserExperience, "prepareOrganizationRoleGroupChange", "Organization role-group changes must use the domain prepare contract.");
requireIncludes(files.organizationUserExperience, "replaceOrganizationRoleGroups(preview)", "Organization role-group changes must use the domain apply contract.");
requireIncludes(files.organizationUserExperience, "allowDelete: !active", "Organization and user generic delete actions must stay disabled.");
requireIncludes(files.organizationUserExperience, "allowStatusToggle: !active", "Organization and user generic status toggles must stay disabled.");
requireIncludes(files.organizationUserExperience, "rolePoolRequest.current === requestID", "Role-pool loading must discard stale organization responses.");
requireIncludes(files.organizationUserExperience, "rolePoolRequest.current += 1", "Closing or clearing an organization must invalidate in-flight role-pool requests.");
requireIncludes(files.organizationUserExperience, "App.useApp()", "Organization authorization confirmations must use the active AntD application context.");
requireIncludes(files.organizationUserExperience, "cancelText", "Organization authorization confirmations must expose localized cancellation copy.");
requireIncludes(files.organizationUserExperience, "hasMetadataChanges(context.editingRecord", "Authorization and metadata changes must be submitted separately.");
requireRegex(
  files.organizationUserExperience,
  /await replaceOrganizationRoleGroups\(preview\);\s*return recordWithValues/,
  "Organization role-group apply must not be followed by a second generic metadata write.",
);
requireRegex(
  files.organizationUserExperience,
  /await changeUserOrganization\(preview\);\s*return recordWithValues/,
  "User authorization apply must not be followed by a second generic metadata write.",
);
requireIncludes(files.organizationUserExperience, "conflicts.length !== initialImpact.conflictCount", "Organization conflict remediation must reject incomplete conflict pages.");
requireIncludes(files.organizationUserExperience, "aria-invalid={invalidSelectedRoles.length > 0}", "Out-of-pool user roles must expose their invalid state semantically.");
requireIncludes(files.organizationUserExperience, '<ul className="organization-role-pool-list"', "Organization role-pool provenance must use list semantics.");
requireIncludes(files.organizationUserExperience, "organization-role-pool-metrics", "Organization role-pool provenance must expose compact summary metrics.");
requireIncludes(files.organizationUserExperience, "organizationRolePoolSummary", "Organization role-pool summary must use localized copy.");
requireIncludes(files.organizationUserExperience, 'role="textbox" aria-readonly="true"', "Read-only role values must expose a valid read-only widget semantic.");
requireIncludes(files.resourceRoute, 'resource.route === "/roles" || resource.route === "/role-groups"', "Role and role-group routes must share the role governance console.");
requireIncludes(files.resourceRoute, 'resource.route === "/menus"', "The menus route must use the dedicated menu governance console.");
requireIncludes(files.resourceRoute, "<MenuGovernanceConsole", "The menus route must render MenuGovernanceConsole instead of GenericResourceConsole.");
requireIncludes(files.menuGovernance, "export function MenuGovernanceConsole", "MenuGovernanceConsole must expose the dedicated menus workbench entry point.");
requireIncludes(files.menuGovernance, 'type MenuEditorMode = "create-directory" | "create-page" | "edit-directory" | "edit-page";', "Menu governance must expose explicit directory and page create/edit modes.");
requireIncludes(files.menuGovernance, 'const directoryMode = editor?.mode === "create-directory" || editor?.mode === "edit-directory";', "Directory authoring must use an explicit mode boundary.");
requireIncludes(files.menuGovernance, "{!directoryMode ? (", "Page-only route, parameter, and button controls must stay hidden during directory authoring.");
requireIncludes(files.menuGovernance, 'route: "",', "Directory definitions must clear route metadata before submission.");
requireIncludes(files.menuGovernance, 'componentKey: "",', "Directory definitions must clear component metadata before submission.");
requireIncludes(files.menuGovernance, 'externalUrl: "",', "Directory definitions must clear external URL metadata before submission.");
requireIncludes(files.menuGovernance, 'parameters: [],', "Directory definitions must clear page parameters before submission.");
requireIncludes(files.menuGovernance, 'buttons: directoryMode ? [] :', "Directory definitions must clear page buttons before submission.");
requireIncludes(files.menuGovernance, 'nodeType(record) === "directory"', "Page parent choices must be restricted to directory nodes.");
requireIncludes(files.menuGovernance, 'isLeaf: nodeType(record) === "page"', "Page menu nodes must remain tree leaves.");
requireNotIncludes(files.menuGovernance, 'name="permission"', "Legacy menu permission must remain read-only and absent from authoring.");
requireIncludes(files.menuGovernance, "AdminTreeWorkbench", "Menu governance must reuse the platform tree workbench wrapper.");
requireIncludes(files.treeWorkbench, "const [activeKey, setActiveKey] = useState<string | null>(null);", "Tree workbench keyboard focus must track an active node.");
requireIncludes(files.treeWorkbench, "setActiveKey(selectedKey || firstTreeKey(treeData));", "Tree workbench keyboard focus must synchronize with the selected menu node.");
requireIncludes(files.treeWorkbench, "activeKey={activeKey}", "Tree workbench must expose the selected menu node to Ant Tree keyboard handling.");
requireIncludes(files.treeWorkbench, "onActiveChange={(key) => setActiveKey(String(key))}", "Tree workbench must preserve Ant Tree arrow-key navigation after selection synchronization.");
requireIncludes(files.menuGovernance, 'const canRead = hasPermission(permissions, "admin:menu:read", deniedPermissions);', "Menu governance read access must respect allowed and denied menu permissions.");
requireIncludes(files.menuGovernance, 'const canCreate = hasPermission(permissions, "admin:menu:create", deniedPermissions);', "Menu creation must require admin:menu:create and respect denied permissions.");
requireIncludes(files.menuGovernance, 'const canUpdate = hasPermission(permissions, "admin:menu:update", deniedPermissions);', "Menu updates must require admin:menu:update and respect denied permissions.");
requireIncludes(files.menuGovernance, "getMenuDefinition", "Menu governance must load selected button metadata through the atomic menu-definition service object.");
requireIncludes(files.menuGovernance, "resolveMenuGovernanceWriteMode(schema)", "Menu governance must derive legacy or target writes from the authoritative runtime schema.");
requireIncludes(files.menuGovernance, "projectMenuGovernanceRecords(rawRecords, nextWriteMode, menuDirectoryLabels())", "Legacy menu snapshots must project missing directory ancestors with localized labels before tree rendering.");
requireIncludes(files.menuGovernance, 'if (menuWriteMode === "legacy")', "Menu governance must keep legacy updates separate from target service-object writes.");
requireIncludes(files.menuGovernance, "setSelectedDefinition(legacyMenuDefinition(selectedRecord))", "Legacy menu snapshots must render a compatible definition instead of raising a service-object error.");
requireIncludes(files.menuGovernance, 'canCreate && menuWriteMode === "target" && (records.length === 0 || menuDefinitionWritable)', "Legacy menu snapshots must not expose target-only create actions, while an empty target tree can create its first node.");
requireIncludes(files.menuGovernance, "canUpdate && menuDefinitionWritable", "Legacy menu snapshots must not expose target-only update actions.");
requireIncludes(files.menuGovernance, "createMenuDefinition", "Menu governance must create menus through the atomic menu-definition service object.");
requireIncludes(files.menuGovernance, "replaceMenuDefinition", "Menu governance must replace menus through the atomic menu-definition service object.");
requireIncludes(files.menuGovernance, 'createAdminResource("menus", input)', "Editing a projected legacy directory must materialize one real directory record through generic create.");
requireIncludes(files.menuGovernance, 'updateAdminResource("menus", legacyRecord.id, input)', "Existing legacy menu metadata updates must use the current schema and record without touching target definitions.");
requireIncludes(files.menuGovernance, "isSyntheticLegacyDirectory(legacyRecord)", "Legacy directory materialization must remain distinct from updates to existing records.");
requireIncludes(files.menuGovernance, "writeMode: menuWriteMode", "Menu editors must snapshot their runtime write mode when opened.");
requireIncludes(files.menuGovernance, "freshWriteMode !== editor.writeMode", "Menu saves must reject runtime mode changes instead of switching write paths mid-edit.");
requireIncludes(files.menuGovernance, "legacyRecord.updatedAt !== editor.updatedAt", "Legacy menu saves must reject stale record snapshots.");
requireIncludes(files.menuGovernance, 'editor?.writeMode === "target"', "Legacy menu editors must not expose target-only page button authoring.");
requireNotIncludes(files.menuGovernance, '<Form.Item name="activeMenuCode"', "Active-menu routing metadata must not be exposed as ordinary tree or permission configuration.");
requireIncludes(files.menuGovernance, 'activeMenuCode: existing?.node.activeMenuCode ?? ""', "Hidden-route active menu metadata must be preserved when ordinary menu metadata is edited.");
requireIncludes(files.menuGovernanceRuntime, 'return schema.fields.find((field) => field.key === "nodeType")?.required ? "target" : "legacy";', "Menu governance runtime mode must be derived from target nodeType requirements.");
requireIncludes(files.menuGovernanceRuntime, "missingDirectories.add(directoryCode)", "Legacy menu projection must materialize missing directory ancestors.");
requireIncludes(files.menuGovernanceRuntime, 'field.source === "values" && !field.readOnly && field.sensitivity === "public"', "Legacy menu updates must submit only declared public writable value fields.");
requireNotIncludes(files.menuGovernanceRuntime, "pageButtons: JSON.stringify", "Legacy menu updates must not submit read-only page button projections.");
requireIncludes(files.organizationRBAC, "client.getMenuDefinition", "The menu-definition query wrapper must use the generated service-object client.");
requireIncludes(files.organizationRBAC, "client.createMenuDefinition", "The menu-definition create wrapper must use the generated service-object client.");
requireIncludes(files.organizationRBAC, "client.replaceMenuDefinition", "The menu-definition replace wrapper must use the generated service-object client.");
requireIncludes(files.organizationRBAC, "arguments: { definition, expectedRevision }", "Menu-definition creation and replacement must carry the caller's current global revision.");
requireIncludes(files.menuGovernance, "createMenuDefinition(definition, selectedRevision)", "Menu creation must use the most recent trusted global menu revision.");
requireIncludes(files.menuGovernance, 'type MenuParameterType = "string" | "number" | "boolean";', "Page parameters must use the bounded string, number, or boolean type set.");
requireIncludes(files.menuGovernance, "SAFE_PARAMETER_KEY", "Page parameter keys must use an explicit safe-key contract.");
requireIncludes(files.menuGovernance, "isForbiddenMenuParameterStringValue(value)", "Page parameters must reject scripts, expressions, SQL, and physical routing inputs.");
requireIncludes(files.menuGovernanceValidation, "FORBIDDEN_PARAMETER_WORD", "Menu parameter validation must keep an explicit forbidden-word contract aligned with the backend.");
requireIncludes(files.menuGovernanceValidation, '"vbscript:"', "Menu parameter validation must reject executable URI schemes aligned with the backend.");
requireIncludes(files.menuGovernanceValidation, '"data:text/html"', "Menu parameter validation must reject executable data URLs aligned with the backend.");
requireIncludes(files.menuGovernance, "isSafeInternalMenuRoute(route)", "Internal menu routes must use route-specific validation instead of parameter-value keyword blocking.");
requireNotIncludes(files.menuGovernance, "FORBIDDEN_PARAMETER_INPUT", "Menu governance must not reuse parameter keyword blocking for literal routes.");
requireIncludes(files.menuGovernance, "duplicateParameterKey", "Page parameters must reject duplicate keys.");
requireIncludes(files.menuGovernance, '<Form.List name="parameters"', "Page parameters must be controlled typed form rows.");
requireIncludes(files.menuGovernance, "parameterValueControl(parameterType", "Page parameter values must render controls that preserve their selected type.");
requireIncludes(files.menuGovernance, "button.menuCode === values.code.trim()", "Page-button metadata must point to the current menu code.");
requireIncludes(files.menuGovernance, "button.permissionCode.trim()", "Each page button must carry exactly one explicit permission code.");
requireIncludes(files.menuGovernance, "duplicateButtonKey", "Page buttons must reject duplicate stable button keys.");
requireIncludes(files.menuGovernance, '<Form.List name="buttons">', "Page buttons must be controlled rows inside the selected page editor.");
requireIncludes(files.menuGovernance, "dictionary.menuButtonAuthorizationBoundary", "Page-button editing must state that visibility metadata does not authorize APIs.");
requireIncludes(files.permissionGovernance, "export function PermissionGovernanceConsole", "PermissionGovernanceConsole must expose the dedicated permissions workbench entry point.");
requireIncludes(files.permissionGovernance, "AdminTreeWorkbench", "Permission governance must reuse the platform tree workbench wrapper.");
requireIncludes(files.permissionGovernance, 'const canRead = hasPermission(permissions, "admin:permission:read", deniedPermissions);', "Permission governance read access must respect allowed and denied permission catalog access.");
requireIncludes(files.permissionGovernance, 'queryAdminResource("permissions"', "Permission governance must use the current generic permissions resource query contract.");
requireIncludes(files.permissionGovernance, "projectPermissionTree(records, search, dictionary, language)", "Permission governance must project permission records into the grouped tree model.");
requireIncludes(files.permissionGovernance, 'const typeKey = `permission-type:${resourceType}`;', "Permission governance must group by resource type before capability and resource.");
requireIncludes(files.permissionGovernance, 'const capabilityKey = `${typeKey}:capability:${capability}`;', "Permission governance must group by capability under resource type.");
requireIncludes(files.permissionGovernance, 'const resourceKey = `${capabilityKey}:resource:${resource}`;', "Permission governance must group by resource under capability.");
requireIncludes(files.permissionGovernance, "const leftLevel = permissionTreeNodeLevel(left);", "Permission governance tree sorting must use explicit hierarchy levels.");
requireIncludes(files.permissionGovernance, "typeRank(permissionTreeResourceType(left))", "Permission governance tree sorting must keep resource-type groups stable.");
requireNotIncludes(files.permissionGovernance, 'String(left.key).split(":").length', "Permission governance tree sorting must not derive hierarchy depth from colon-delimited permission codes.");
requireIncludes(files.permissionGovernance, "permissionSeededGuardTitle", "Permission governance must state that system-generated permissions are contract-locked.");
requireIncludes(files.permissionGovernance, "CUSTOM_API_PERMISSION_CAPABILITY", "Permission governance must isolate hand-maintained entries behind an explicit custom API capability marker.");
requireIncludes(files.permissionGovernance, 'createAdminResource("permissions"', "Permission governance must support controlled custom API permission creation.");
requireIncludes(files.permissionGovernance, 'updateAdminResource("permissions"', "Permission governance must support controlled custom API permission updates.");
requireIncludes(files.permissionGovernance, "isCustomAPIPermission(record)", "Permission governance must guard edit actions to custom API permission records.");
requireIncludes(files.permissionGovernance, "if (!canCreate) {", "Permission governance must fail closed before opening custom permission creation without create access.");
requireIncludes(files.permissionGovernance, "if (!canUpdate) {", "Permission governance must fail closed before opening custom permission editing without update access.");
requireIncludes(files.permissionGovernance, "if ((editor.record && !canUpdate) || (!editor.record && !canCreate)) {", "Permission governance must guard the custom permission submit path against stale unauthorized states.");
requireIncludes(files.permissionGovernance, "dictionary.permissionSystemLockedDescription", "Permission governance must explain why system-generated permission records cannot be edited directly.");
requireIncludes(files.menuGovernance, "const menuListRequest = useRef(0);", "Menu governance must track the latest tree search request.");
requireIncludes(files.menuGovernance, "const definitionRequest = useRef(0);", "Menu governance must track the latest selected-definition request.");
requireIncludes(files.menuGovernance, "const editorSession = useRef(0);", "Menu saves must ignore stale mutation completions from a closed or replaced editor session.");
requireIncludes(files.menuGovernance, "if (editorSession.current !== sessionID) return;", "Menu saves must ignore stale mutation completions from a closed or replaced editor session.");
requireIncludes(files.menuGovernance, "const savingRef = useRef(false);", "Menu saves must acquire a synchronous single-flight lock before confirmation or mutation awaits.");
requireIncludes(files.menuGovernance, "if (savingRef.current) return;", "Menu saves must acquire a synchronous single-flight lock before confirmation or mutation awaits.");
requireIncludes(files.menuGovernance, "savingRef.current = true;", "Menu saves must acquire a synchronous single-flight lock before confirmation or mutation awaits.");
requireIncludes(files.menuGovernance, "setDefinitionRefresh((current) => current + 1);", "Menu saves must reload the normalized definition and its new global revision even when selection stays unchanged.");
requireIncludes(files.menuGovernance, "if (!canRead || menuListRequest.current !== requestID) return;", "Menu governance must fail closed before requesting records without read access.");
requireCountAtLeast(files.menuGovernance, "if (menuListRequest.current !== requestID) return;", 2, "Menu search must discard stale responses before changing loading, error, data, or selection.");
requireCountAtLeast(files.menuGovernance, "if (definitionRequest.current !== requestID) return;", 2, "Selected menu loading must discard stale responses before changing detail state.");
requireIncludes(files.menuGovernance, "setDefinitionLoading(false);\n      setSelectedDefinition(null);\n      setSelectedRevision(0);", "Clearing menu selection must also clear loading and revision state.");
requireIncludes(files.menuGovernance, "setSelectedDefinition(null);\n    setSelectedRevision(0);\n    setMenuDefinitionWritable(false);\n    setDefinitionLoading(true);", "Starting a target menu definition load must clear stale detail, revision, and write state.");
requireIncludes(files.menuGovernance, "returnFocusRef.current?.focus({ preventScroll: true });", "Closing the menu editor must restore focus without scrolling.");
requireIncludes(files.menuGovernance, "returnFocusRef.current = detailFocusRef.current;", "Successful menu saves must restore focus to a stable detail target that survives definition refresh.");
requireIncludes(files.menuGovernance, "await confirmMenuParentChange(modal.confirm, dictionary, editor.definition, definition, records)", "Menu parent changes must require an explicit localized structural confirmation.");
requireIncludes(files.menuGovernance, "duplicateButtonPermission(form, index, dictionary.menuButtonPermissionDuplicate)", "Page-button permission codes must expose duplicate validation before submission.");
requireRegex(files.styles, /\.menu-governance-detail \.admin-list-actions \.ant-btn,[\s\S]*?min-height:\s*44px;/, "Menu governance actions must expose 44px targets.");
requireRegex(files.styles, /\.menu-governance-modal \.ant-modal-close,[\s\S]*?\.menu-governance-modal \.ant-modal-footer \.ant-btn,[\s\S]*?\.menu-governance-modal \.ant-checkbox-wrapper,[\s\S]*?min-height:\s*44px;/, "Menu governance modal controls must expose 44px targets.");
requireRegex(files.styles, /\.admin-tree-workbench-tree \.ant-tree-switcher\s*\{[\s\S]*?min-width:\s*44px;[\s\S]*?min-height:\s*44px;/, "Tree workbench expanders must expose a 44px pointer target.");
requireRegex(files.styles, /\.menu-governance-form-list-row\s*\{[\s\S]*?grid-template-columns:/, "Menu parameter and button rows must use a stable responsive grid.");
requireRegex(files.styles, /@media screen and \(max-width:\s*767px\)[\s\S]*?\.menu-governance-form-list-row\s*\{[\s\S]*?grid-template-columns:\s*minmax\(0,\s*1fr\);/, "Menu parameter and button rows must stack without horizontal overflow on mobile.");
requireCssRule(
  tabletWorkbenchStyles,
  ".admin-tree-workbench-navigation .admin-list-toolbar .ant-input-affix-wrapper",
  ["min-height: 44px;"],
  "Tree workbench search must expose a 44px tablet touch target.",
);
for (const key of [
  "menuGovernanceTitle",
  "menuTreeAriaLabel",
  "menuAddDirectory",
  "menuAddPage",
  "menuParameters",
  "menuPageButtons",
  "menuButtonAuthorizationBoundary",
  "menuSaveSucceeded",
]) {
  requireCountExactly(files.i18n, `${key}:`, 2, `Menu governance i18n key ${key} must exist in matching Chinese and English dictionaries.`);
}
for (const key of [
  "permissionGovernanceTitle",
  "permissionTreeAriaLabel",
  "permissionTreeTitle",
  "permissionSeededGuardTitle",
  "permissionAddCustomAPI",
  "permissionEditCustomAPI",
  "permissionSystemLockedDescription",
  "permissionResourceType",
  "permissionGroupTotal",
]) {
  requireCountExactly(files.i18n, `${key}:`, 2, `Permission governance i18n key ${key} must exist in matching Chinese and English dictionaries.`);
}
requireIncludes(files.settings, "LayoutModeSelector", "SystemSettingsDrawer must use the illustrated layout-mode selector.");
requireIncludes(files.settings, "settings-summary-groups", "SystemSettingsDrawer must keep the grouped summary layout.");
requireIncludes(files.settings, "dictionary.interfacePreferences", "System settings summary must classify interface preferences.");
requireIncludes(files.settings, "dictionary.runtimeContext", "System settings summary must classify runtime context.");
requireIncludes(files.settings, "SettingSummaryItem label={dictionary.environmentContext}", "System settings summary must include environment context.");
requireIncludes(files.settings, "SettingSummaryItem label={dictionary.tenantContext}", "System settings summary must include tenant context.");
requireNotIncludes(files.settings, "settings-status-grid", "System settings must not restore the old cramped status grid.");
requireIncludes(files.settings, "aria-pressed={active === mode}", "Layout-mode options must expose their selected state to assistive technology.");
requireIncludes(files.settings, "layoutDescription(dictionary, mode)", "Layout-mode options must include the approved explanatory copy.");
requireIncludes(files.settings, "showPreviews={uiConfig.showLayoutLegend}", "The persisted layout-legend preference must control preview visibility without hiding the selector.");
requireRegex(files.styles, /\.layout-mode-option\s*\{[\s\S]*?grid-template-columns:\s*minmax\([^;]+\)\s+minmax\([^;]+\)\s+44px/, "Layout-mode options must keep preview, copy, and selection state in stable columns.");
requireIncludes(files.roleGovernance, "AdminTreeWorkbench", "Role governance must use the platform tree workbench wrapper.");
requireIncludes(files.roleGovernance, "PlatformTreeTransfer", "Role permission and menu entry points must use the platform Tree Transfer wrapper.");
requireIncludes(files.primitives, "export function AdminModal", "Shared platform dialogs must expose the AdminModal wrapper.");
requireIncludes(files.primitives, '<Modal className={cx("admin-modal", className)}', "AdminModal must remain the single Ant Modal boundary for shared platform dialogs.");
requireIncludes(files.primitives, "return <AdminModal", "AdminFormModal must build on the shared AdminModal wrapper.");
requireIncludes(files.roleGovernance, "AdminModal", "Role governance dialogs must use the shared AdminModal wrapper.");
requireNotIncludes(files.roleGovernance, "<Modal", "Role governance must not bypass the shared AdminModal wrapper.");
requireIncludes(files.roleGovernance, "role-governance-action-groups", "Role governance actions must be grouped by authorization and lifecycle responsibility.");
requireIncludes(files.roleGovernance, "roleGovernanceAuthorizationActions", "Role governance authorization actions must have a localized group label.");
requireIncludes(files.roleGovernance, "roleGovernanceLifecycleActions", "Role governance lifecycle actions must have a localized group label.");
requireIncludes(files.roleGovernance, 'const canReadGroups = hasPermission(permissions, "admin:role-group:read", deniedPermissions);', "Role governance must derive role-group read access from the active permission set.");
requireIncludes(files.roleGovernance, 'const canReadRoles = hasPermission(permissions, "admin:role:read", deniedPermissions);', "Role governance must derive role read access from the active permission set.");
requireIncludes(files.roleGovernance, 'const canReadTenants = hasPermission(permissions, "admin:tenant:read", deniedPermissions);', "Role-group creation must derive tenant read access from the active permission set.");
requireRegex(
  files.roleGovernance,
  /const canCreateGroup = runtimeSchemaReady[\s\S]*?groupWriteMode !== "readonly"[\s\S]*?hasPermission\(permissions, "admin:role-group:create", deniedPermissions\)[\s\S]*?\(groupWriteMode !== "target" \|\| canReadTenants\);/,
  "Target tenant-scoped role-group creation must require tenant read access while legacy metadata remains writable.",
);
requireIncludes(files.roleGovernance, 'const canReadAuthorizationInputs = hasPermission(permissions, "admin:permission:read", deniedPermissions) && hasPermission(permissions, "admin:org-unit:read", deniedPermissions) && hasPermission(permissions, "admin:area-code:read", deniedPermissions);', "Role permission assignment must require every resource read permission used by its editor.");
requireIncludes(files.roleGovernance, 'const canReadMenus = hasPermission(permissions, "admin:menu:read", deniedPermissions);', "Read-only role menu assignment must require menu read access.");
requireIncludes(files.roleGovernance, '{canReadMenus ? <AdminActionButton ref={menuTriggerRef}', "Role menu assignment must be hidden when menu records cannot be read.");
requireIncludes(files.roleGovernance, '{canReadAuthorizationInputs ? <AdminActionButton ref={authorizationTriggerRef}', "Role permission assignment must be hidden when its supporting records cannot be read.");
requireRegex(
  files.roleGovernance,
  /canReadGroups\s*\?\s*loadAllRecords\("role-groups"\)\s*:\s*Promise\.resolve\(\[\]\)[\s\S]*?canReadRoles\s*\?\s*loadAllRecords\("roles", query \? \[query\] : undefined\)\s*:\s*Promise\.resolve\(\[\]\)/,
  "Role governance must not request role or role-group resources the current principal cannot read.",
);
requireIncludes(files.roleGovernance, "const governanceRequest = useRef(0);", "Role governance must track the latest tree request.");
requireIncludes(files.roleGovernance, "if (governanceRequest.current !== requestID) return;", "Role governance must discard stale role and role-group search responses.");
requireRegex(files.roleGovernance, /loadGovernance[\s\S]*?if \(governanceRequest\.current !== requestID\) return;\s*setLoading\(true\);/, "A stale debounced role-governance request must not re-enter the loading state.");
requireIncludes(files.rolePermissionWorkflow, "allowPermissionCodes: authorization.allow", "Role policy prepare must include allowed permissions.");
requireIncludes(files.rolePermissionWorkflow, "denyPermissionCodes: authorization.deny", "Role policy prepare must include denied permissions.");
requireIncludes(files.rolePermissionWorkflow, "dataScope: authorization.dataScope", "Role policy prepare must include data scope.");
requireIncludes(files.rolePermissionWorkflow, "await clients.replace(preview)", "Role policy changes must apply through the reviewed domain command.");
for (const key of ["permissions", "denyPermissions", "dataScope", "dataScopeOrgCodes", "dataScopeAreaCodes"]) {
  requireIncludes(files.rolePermissionWriteMode, `"${key}"`, `Role permission write-mode resolver must inspect ${key}.`);
}
requireIncludes(files.rolePermissionWriteMode, 'field?.inForm !== false && field?.readOnly !== true', "Legacy role permission mode must require every policy field to be form-writable and not read-only.");
requireIncludes(files.rolePermissionWriteMode, 'field?.inForm !== true && field?.readOnly === true', "Target role permission mode must treat omitted false inForm values as excluded from forms and require every policy field to be read-only.");
requireIncludes(files.rolePermissionWriteMode, 'if (fields.some((field) => !field)) return "readonly";', "Missing role permission policy fields must resolve to readonly.");
for (const resource of ["role-groups", "roles", "menus"]) {
  requireIncludes(files.roleGovernance, `getAdminResourceSchema("${resource}")`, `Role governance runtime must load the trusted ${resource} schema.`);
}
requireIncludes(files.roleGovernance, "resolveRoleGovernanceRuntime(roleGroupSchema, roleSchema, menuSchema)", "Role governance modes must resolve from one consistent schema snapshot.");
requireIncludes(files.roleGovernanceRuntime, "const targetIdentityRuntime = groupWriteMode === \"target\" && permissionWriteMode === \"target-domain\";", "Target role lifecycle must require both role-group and role policy ownership cutover.");
requireIncludes(files.roleGovernanceRuntime, "roleMenuTargetEnabled: targetIdentityRuntime && targetMenuSchema", "Role-menu writes must require the complete identity and menu schema cutover.");
requireIncludes(files.roleGovernance, "const runtimeSchemaRequest = useRef(0);", "Role governance schema loading must track stale requests.");
requireIncludes(files.roleGovernance, "if (runtimeSchemaRequest.current !== requestID) return;", "Role governance schema loading must discard stale or unmounted results.");
requireIncludes(files.roleGovernance, "return () => { runtimeSchemaRequest.current += 1; };", "Role governance schema loading must invalidate pending work on unmount.");
requireRegex(
  roleAuthorizationOpen,
  /const writeMode = permissionWriteMode;[\s\S]*?loadRolePermissionCatalog\(writeMode, role\.code, \{[\s\S]*?target: assignmentPermissionRecords,[\s\S]*?generic: \(\) => loadAllRecords\("permissions"\),[\s\S]*?\}\)[\s\S]*?setAuthorization\(\{\s*role,\s*writeMode,/,
  "Permission catalogs must use the role schema write mode instead of the menu migration gate.",
);
requireIncludes(files.rolePermissionWorkflow, 'return writeMode === "target-domain" ? sources.target(roleCode) : sources.generic();', "Permission catalogs must select their source from the snapshotted role permission mode.");
requireNotIncludes(roleAuthorizationOpen, "roleMenuMigrationWriteEnabled", "Permission catalogs must use the role schema write mode instead of the menu migration gate.");
requireIncludes(
  roleAuthorizationSave,
  'if (!authorization || !canUpdateRole || authorization.role.status !== "enabled" || authorization.writeMode === "readonly") return;',
  "Role permission saves must reject readonly, unauthorized and disabled-role writes.",
);
requireRegex(
  roleAuthorizationSave,
  /executeRolePermissionWrite\(authorization, canUpdateRole, \{[\s\S]*?updateAdminResource,[\s\S]*?prepare: prepareRolePermissionChange,[\s\S]*?impact: getRolePermissionChangeImpact,[\s\S]*?replace: replaceRolePermissions,[\s\S]*?\}\)/,
  "Role permission saves must use the executable workflow with the production clients.",
);
requireIncludes(files.rolePermissionWorkflow, 'if (!canUpdateRole || authorization.role.status !== "enabled" || authorization.writeMode === "readonly") return "blocked"', "Role permission workflow must reject readonly, unauthorized and disabled-role writes.");
requireRegex(files.rolePermissionWorkflow, /if \(authorization\.writeMode === "legacy-generic"\) \{[\s\S]*?await clients\.updateAdminResource\("roles", authorization\.role\.id, legacyRolePermissionInput\(authorization\)\);[\s\S]*?return "applied" as const;[\s\S]*?\}[\s\S]*?const preview = await clients\.prepare[\s\S]*?await clients\.replace\(preview\);/, "Role permission writes must keep legacy generic and target domain-command paths mutually exclusive.");
requireNotRegex(files.rolePermissionWorkflow, /await clients\.replace\(preview\);[\s\S]*?clients\.updateAdminResource/, "Role policy apply must not be followed by a second generic role mutation.");
for (const field of ["code", "name", "status", "description"]) {
  requireIncludes(legacyRolePermissionInput, `${field}: authorization.role.${field}`, `Legacy role permission writes must preserve the public ${field} field.`);
}
requireIncludes(legacyRolePermissionInput, "...authorization.role.values,", "Legacy role permission writes must preserve the complete public values snapshot.");
for (const field of ["permissions", "denyPermissions", "dataScope", "dataScopeOrgCodes", "dataScopeAreaCodes"]) {
  requireIncludes(legacyRolePermissionInput, `${field}:`, `Legacy role permission writes must submit ${field}.`);
}
requireIncludes(legacyRolePermissionInput, 'uniqueSorted(authorization.allow).join(",")', "Legacy allowed permissions must use the existing delimited storage format.");
requireIncludes(legacyRolePermissionInput, 'uniqueSorted(authorization.deny).join(",")', "Legacy denied permissions must use the existing delimited storage format.");
requireIncludes(files.roleGovernance, '{canReadAuthorizationInputs ? <AdminActionButton ref={authorizationTriggerRef} disabled={!runtimeSchemaReady} icon={<SafetyCertificateOutlined', "Readable role permission workflows must stay available for inspection after runtime schemas are ready.");
requireIncludes(roleAuthorizationModal, "footer={readOnly ? <Button onClick={onCancel}>{dictionary.close}</Button> : undefined}", "Read-only role permission inspection must expose a close-only footer.");
requireIncludes(roleAuthorizationModal, 'readOnly={readOnly}', "Read-only role permission inspection must disable Tree Transfer and data-scope controls.");
requireCountAtLeast(roleAuthorizationModal, "disabled={readOnly}", 3, "Read-only role permission inspection must disable every data-scope control.");
requireIncludes(roleAuthorizationModal, "dictionary.rolePermissionReadonlyTitle", "Read-only role permission inspection must expose a localized reason.");
for (const key of ["rolePermissionReadonlyTitle", "rolePermissionReadonlySchemaDescription", "rolePermissionReadonlyAccessDescription", "rolePermissionReadonlyDisabledDescription"]) {
  requireCountExactly(files.i18n, `${key}:`, 2, `Role permission read-only i18n key ${key} must exist in matching Chinese and English dictionaries.`);
}
requireIncludes(files.roleGovernance, "sameRoleGroupBoundary(group, moveSourceGroup)", "Role move options must stay inside the current scope and tenant boundary.");
requireIncludes(files.roleGovernance, 'const lifecycleDisabled = !roleLifecycleTargetEnabled || !canUpdateRole || record.status !== "enabled";', "Disabled roles and non-target runtimes must not expose lifecycle mutations.");
requireCountAtLeast(files.roleGovernance, "disabled={lifecycleDisabled}", 2, "Role lifecycle actions must share the fail-closed disabled state.");
requireIncludes(files.roleGovernance, "resolveRoleMenuAccess(roleMenuTargetEnabled, canAssignMenus, record.status)", "Role menu entry points must use the shared access resolver.");
requireIncludes(files.roleGovernance, "resolveRoleMenuAccess(roleMenuTargetEnabled, canAssignMenus, menuAssignment?.role.status ?? \"\")", "Role menu modals must use the shared access resolver.");
requireIncludes(files.roleGovernance, "!resolveRoleMenuAccess(roleMenuTargetEnabled, canAssignMenus, menuAssignment.role.status).editable", "Role menu saves must use the shared access resolver.");
requireIncludes(files.roleGovernance, "menuAccess.editable ? dictionary.assignMenus : dictionary.viewMenus", "Read-only role menu entry points must use the localized View Menus label.");
requireIncludes(roleMenuModal, "readOnly={menuAccess.readOnly}", "Role menu Tree Transfer must use the resolved read-only state.");
requireIncludes(roleMenuModal, "readOnlyMessage={readOnlyReason}", "Role menu inspection must expose the state-specific localized read-only reason.");
for (const key of ["roleMenuLegacyReadonlyDescription", "roleMenuReadonlyAccessDescription", "roleMenuReadonlyDisabledDescription"]) {
  requireIncludes(files.roleGovernance, `dictionary.${key}`, `Role menu inspection must use ${key} when that read-only state applies.`);
}
requireIncludes(
  files.roleGovernance,
  "permissionTreeNodes(permissionCatalog, dictionary, language, uniqueSorted([...authorization.allow, ...authorization.deny]))",
  "Role authorization must project disabled and missing historical permissions into the Tree Transfer catalog.",
);
requireIncludes(files.roleGovernance, "dictionary.rolePermissionHistoricalDisabled", "Disabled historical permissions must remain removable but unavailable for assignment.");
requireIncludes(files.roleGovernance, "availableDisabledReason: dictionary.rolePermissionHistoricalMissing", "Missing historical permissions must remain removable but unavailable for assignment.");
requireIncludes(files.treeTransfer, "const preserved = value.filter((key) => !mutableVisibleSet.has(key));", "Filtered Tree Transfer changes must preserve selections outside the current result.");
requireNotIncludes(files.treeTransfer, "visibleCheckedKeys", "Tree Transfer must not retain the unused visibleCheckedKeys projection.");
requireIncludes(files.treeTransfer, "const preservedDisabled = value.filter((key) => !mutableLeafKeySet.has(key));", "Tree Transfer bulk operations must preserve disabled selections.");
requireIncludes(files.treeTransfer, "returnFocusRef?.current?.focus({ preventScroll: true });", "Closing Tree Transfer workflows must restore focus without scrolling.");
requireIncludes(files.treeTransfer, "virtual={virtual}", "Tree Transfer must virtualize large trees through the platform component.");
requireIncludes(files.treeTransfer, "platform-tree-transfer-mobile-tabs", "Tree Transfer must switch to a mobile single-pane tab layout.");
requireIncludes(files.treeTransfer, "node.kind === \"leaf\" && !node.disabledReason && !node.availableDisabledReason", "Tree Transfer bulk assignment must exclude unavailable historical selections.");
requireIncludes(files.treeTransfer, "const unavailable = Boolean(node.disabledReason || !selectedOnly && node.availableDisabledReason);", "Tree Transfer historical selections must be disabled only in the available pane.");
requireIncludes(files.treeTransfer, "disabled: unavailable", "Tree Transfer must expose pane-specific unavailable semantics to Ant Tree.");
requireIncludes(files.treeWorkbench, 'aria-label={ariaLabel}', "The role tree workbench must expose an accessible tree label.");
requireIncludes(files.treeWorkbench, "workbenchTreeData(nodes, selectedKey)", "Tree workbench data must project the active selection into node semantics.");
requireIncludes(files.treeWorkbench, '"aria-selected": node.key === selectedKey', "Tree workbench nodes must expose explicit aria-selected state.");
requireIncludes(files.treeWorkbench, '"aria-level": depth', "Tree workbench nodes must expose their explicit hierarchy depth.");
requireIncludes(files.treeWorkbench, "children.map((child) => build(child, depth + 1))", "Tree workbench descendants must increment their aria-level.");
requireIncludes(files.roleGovernance, 'className="role-governance-detail-focus-target" tabIndex={-1}', "Role governance must expose a stable programmatic detail focus target.");
for (const triggerRef of ["metadataTriggerRef", "moveTriggerRef", "authorizationTriggerRef", "menuTriggerRef"]) {
  requireIncludes(files.roleGovernance, `afterClose={() => restoreRoleModalFocus(${triggerRef}.current, detailFocusRef.current)}`, `Role modal ${triggerRef} must restore focus to its connected trigger or the detail fallback after close.`);
}
requireCountExactly(files.roleGovernance, "focusTriggerAfterClose={false}", 4, "Role modals must disable Ant automatic trigger focus when using explicit restoration.");
requireNotIncludes(roleAuthorizationModal, "returnFocusRef=", "Role authorization must not delegate focus restoration to Tree Transfer.");
requireNotIncludes(roleMenuModal, "returnFocusRef=", "Role menu visibility must not delegate focus restoration to Tree Transfer.");
requireCountExactly(files.roleGovernance, "cancelText={dictionary.cancel}", 4, "Role governance modals with cancel actions must use the localized cancel label.");
requireIncludes(roleMenuModal, "footer={!menuAccess.showSave ? <Button onClick={onClose}>{dictionary.close}</Button> : undefined}", "Read-only role menu inspection must retain a localized close-only footer.");
requireRegex(
  files.styles,
  /\.role-authorization-modal,\s*\.role-menu-visibility-modal\s*\{[\s\S]*?top:\s*16px;[\s\S]*?padding-bottom:\s*0;[\s\S]*?\}[\s\S]*?\.role-authorization-modal \.ant-modal-content,\s*\.role-menu-visibility-modal \.ant-modal-content\s*\{[\s\S]*?display:\s*flex;[\s\S]*?max-height:\s*calc\(100dvh - 32px\);[\s\S]*?flex-direction:\s*column;[\s\S]*?overflow:\s*hidden;[\s\S]*?\}/,
  "Large role governance modals must fit inside the dynamic viewport with bounded flex content.",
);
requireRegex(
  files.styles,
  /\.role-authorization-modal :is\(\.ant-modal-header, \.ant-modal-footer\),\s*\.role-menu-visibility-modal :is\(\.ant-modal-header, \.ant-modal-footer\)\s*\{[\s\S]*?flex:\s*0 0 auto;[\s\S]*?\}[\s\S]*?\.role-authorization-modal \.ant-modal-body,\s*\.role-menu-visibility-modal \.ant-modal-body\s*\{[\s\S]*?min-height:\s*0;[\s\S]*?overflow:\s*auto;[\s\S]*?\}/,
  "Large role governance modal headers and footers must stay visible while the body scrolls.",
);
requireIncludes(files.roleGovernance, "<Typography.Title level={4}>", "Role detail must use the compact platform title hierarchy.");
requireIncludes(files.roleGovernance, 'className="role-governance-facts"', "Role summaries must use the compact fact-card layout.");
requireIncludes(files.roleGovernance, "roleStatusLabel(record.status, dictionary)", "Role status summaries must be localized.");
requireIncludes(files.roleGovernance, "roleGroupScopeLabel(valueOf(record, \"scopeType\"), dictionary)", "Role-group scope summaries must be localized.");
requireIncludes(files.roleGovernance, "roleDataScopeLabel(valueOf(record, \"dataScope\"), dictionary)", "Role data-scope summaries must be localized.");
requireIncludes(files.roleGovernanceRuntime, "label: localizedGovernanceName(group, language)", "Role tree nodes must separate display names from codes.");
requireIncludes(files.roleGovernanceRuntime, "subtitle: group.code", "Role tree nodes must expose role-group codes as secondary text.");
requireIncludes(files.roleGovernanceRuntime, "subtitle: role.code", "Role tree nodes must expose role codes as secondary text.");
requireIncludes(files.menuGovernance, "subtitle: record.code", "Menu tree nodes must expose menu codes as secondary text.");
requireIncludes(files.menuGovernance, 'meta: nodeType(record) === "directory" ? dictionary.menuDirectory', "Menu tree nodes must expose directory or route metadata.");
requireIncludes(files.roleGovernance, 'className="role-governance-access-control"', "Role access controls must live in their own unframed section.");
requireIncludes(files.roleGovernance, 'className="role-governance-lifecycle"', "Role lifecycle actions must remain separate from authorization.");
requireNotIncludes(files.roleGovernance, "role-governance-command-bar", "Role actions must not collapse back into one command row.");
requireIncludes(files.styles, ".admin-tree-workbench {", "Role governance must define a stable tree/detail layout.");
requireRegex(files.styles, /\.admin-tree-workbench\s*\{[\s\S]*?grid-template-columns:\s*clamp\(320px,\s*32vw,\s*440px\)\s+minmax\(0,\s*1fr\);/, "Desktop tree workbench navigation must stay wide enough for structured tree nodes.");
requireCssRule(files.styles, ".admin-tree-workbench-tree", ["max-height: min(640px, calc(100vh - 280px));", "overflow: auto;", "overscroll-behavior: contain;"], "Tree workbench navigation must scroll internally instead of stretching long trees.");
requireRegex(files.styles, /\.role-governance-detail-focus-target,[\s\S]*?\.role-governance-detail,[\s\S]*?\.permission-governance-detail\s*\{[\s\S]*?min-height:\s*360px;/, "Role and permission detail states must keep a stable minimum height.");
requireRegex(files.styles, /\.admin-tree-workbench-node-label,[\s\S]*?overflow-wrap:\s*anywhere;[\s\S]*?white-space:\s*normal;/, "Tree node labels must wrap long names instead of truncating core context.");
requireRegex(files.styles, /\.admin-tree-workbench-node-subtitle\s*\{[\s\S]*?overflow-wrap:\s*anywhere;/, "Tree node subtitles must wrap long codes.");
requireRegex(files.styles, /\.admin-tree-workbench-node-meta\s*\{[\s\S]*?flex-wrap:\s*wrap;/, "Tree node metadata must wrap instead of clipping status or route context.");
requireRegex(files.styles, /@media \(min-width:\s*1024px\)[\s\S]*?\.admin-tree-workbench-detail\s*\{[\s\S]*?position:\s*sticky;/, "Tree workbench detail may stick only on desktop.");
requireRegex(files.styles, /@media screen and \(max-width:\s*767px\)[\s\S]*?\.platform-tree-transfer-toolbar\s*\{[\s\S]*?position:\s*sticky;[\s\S]*?grid-template-columns:\s*repeat\(2,\s*minmax\(0,\s*1fr\)\);/, "Mobile Tree Transfer toolbar must stay visible as a stable two-column control area.");
requireRegex(files.styles, /@media screen and \(max-width:\s*767px\)[\s\S]*?\.platform-tree-transfer-toolbar \.ant-input-affix-wrapper\s*\{[\s\S]*?grid-column:\s*1 \/ -1;[\s\S]*?width:\s*100%;/, "Mobile Tree Transfer search must span the full toolbar width.");
requireRegex(files.styles, /@media screen and \(max-width:\s*767px\)[\s\S]*?\.platform-tree-transfer-toolbar \.ant-space\s*\{[\s\S]*?grid-template-columns:\s*repeat\(2,\s*minmax\(0,\s*1fr\)\);/, "Mobile Tree Transfer bulk actions must stay in a two-column row.");
requireIncludes(files.styles, ".platform-tree-transfer-pane {", "Tree Transfer must define stable pane dimensions.");
requireRegex(files.styles, /\.platform-tree-transfer-mobile-tabs \.ant-tabs-tab \{[\s\S]*?min-height:\s*44px;/, "Tree Transfer mobile tabs must expose 44px targets.");
requireRegex(files.styles, /\.admin-tree-workbench-navigation \.admin-list-actions \.ant-btn,[\s\S]*?min-height:\s*44px;/, "Role tree creation actions must expose 44px targets.");
requireRegex(files.styles, /\.role-governance-detail \.admin-list-actions \.ant-btn,[\s\S]*?min-height:\s*44px;/, "Role detail actions must expose 44px targets.");
requireRegex(
  files.styles,
  /\.platform-tree-transfer-pane \.ant-tree-checkbox\s*\{[\s\S]*?width:\s*44px;[\s\S]*?min-width:\s*44px;[\s\S]*?height:\s*44px;/,
  "Tree Transfer checkboxes must expose a real 44px by 44px pointer target.",
);
for (const key of [
  "roleGovernanceTitle",
  "roleTreeAriaLabel",
  "assignPermissions",
  "assignMenus",
  "viewMenus",
  "roleAccessControl",
  "roleLifecycle",
  "roleMenuReadonlyTitle",
  "roleMenuReadonlyAccessDescription",
  "roleMenuReadonlyDisabledDescription",
  "roleMenuLegacyReadonlyDescription",
  "rolePermissionHistoricalDisabled",
  "rolePermissionHistoricalMissing",
  "transferSelectedCount",
  "roleGovernanceAuthorizationActions",
  "roleGovernanceLifecycleActions",
  "organizationRolePoolSummary",
  "organizationRolePoolDescription",
  "organizationRolePoolRoleCount",
  "organizationRolePoolGroupCount",
]) {
  requireCountExactly(files.i18n, `${key}:`, 2, `Role governance i18n key ${key} must exist in matching Chinese and English dictionaries.`);
}
requireRegex(
  files.resourceConsole,
  /const canRevealField = useCallback\([\s\S]*?field\.inDetail && field\.reveal && permissionAllows\(permissions, field\.reveal\.permission, deniedPermissions\)/,
  "Sensitive reveal actions must require detail visibility, manifest declaration, and the declared permission.",
);
requireRegex(
  files.resourceConsole,
  /function DetailFieldsPanel\([\s\S]*?className="sensitive-field-reveal-trigger"[\s\S]*?dictionary\.sensitiveRevealAction/,
  "Sensitive reveal actions must be rendered only by the detail field panel.",
);
requireCountExactly(files.resourceConsole, 'className="sensitive-field-reveal-trigger"', 1, "Sensitive reveal must expose exactly one detail-only trigger implementation.");
requireIncludes(files.resourceConsole, "provider.getOne<AdminResourceRecord>", "OIDC resume must hydrate a detail record that is outside the current list page.");
requireIncludes(files.resourceConsole, "setSensitiveRevealTarget(null);", "Closing the detail context must also close the sensitive reveal modal.");
requireNotIncludes(files.app, "revealedValue", "App state must never retain sensitive plaintext.");
requireNotIncludes(files.resourceConsole, "revealedValue", "Resource page state must never retain sensitive plaintext.");
requireNotIncludes(files.sensitiveRevealOIDC, "revealedValue", "OIDC pending state must never retain sensitive plaintext.");
requireIncludes(files.sensitiveRevealModal, 'document.visibilityState === "hidden"', "Sensitive plaintext must be cleared when the page becomes hidden.");
requireIncludes(files.sensitiveRevealModal, "operationGenerationRef", "Sensitive reveal requests must use an operation generation guard.");
requireRegex(
  files.sensitiveRevealModal,
  /const result = await revealAdminSensitiveField[\s\S]*?if \(operationGenerationRef\.current !== generation\) return;/,
  "Reveal responses must be discarded after close, hide, or target changes.",
);
requireIncludes(
  files.sensitiveRevealModal,
  "result.copyAllowed && policy?.copyAllowed && field.reveal?.copyAllowed",
  "Sensitive plaintext copy must require the response, policy, and field contract to allow it.",
);
requireRegex(files.styles, /\.sensitive-field-reveal-trigger\.ant-btn\s*\{[\s\S]*?min-width:\s*44px;[\s\S]*?min-height:\s*44px;/, "Sensitive reveal detail triggers must keep a 44px pointer target.");
requireRegex(files.styles, /@media \(max-width: 640px\)[\s\S]*?\.sensitive-reveal-modal[\s\S]*?width:\s*calc\(100vw - 24px\)/, "Sensitive reveal modal must remain near-full-width on small screens.");
requireIncludes(files.client, "parsePlatformResponse", "Direct fetch helpers must share response normalization.");
requireIncludes(files.authProvider, "error instanceof AdminAPIError", "Refine auth errors must use the typed admin API error contract.");
requireIncludes(files.capabilityMetadata, 'label: { zh: "身份与组织", en: "Identity & Organization" }', "Core identity capability must make default organization ownership explicit.");
requireIncludes(files.capabilityMetadata, 'makeOptional("personnel", { zh: "人员与岗位", en: "Personnel & Positions" }', "Optional personnel capability must not be labeled as the organization capability.");
requireIncludes(files.capabilityMetadata, "默认平台底座已提供组织机构", "Optional personnel copy must state that organization units are part of the default foundation.");
requireIncludes(files.capabilityConsole, "PlatformDataTable", "Capability console must render capabilities through the shared list/table surface.");
requireIncludes(files.capabilityConsole, "openCapabilityDetail", "Capability console must keep a single detail-opening path.");
requireIncludes(files.capabilityConsole, "onRowClick={(record) => openCapabilityDetail(record.id)}", "Capability list rows must open the detail modal.");
requireIncludes(files.capabilityConsole, "mobileCards={(items) =>", "Capability console must keep mobile cards as direct detail entry points.");
requireCountAtLeast(files.capabilityConsole, "openCapabilityDetail(capability.id)", 2, "Capability mobile cards must open the detail modal from pointer and click paths.");
requireIncludes(files.capabilityConsole, 'className="capability-detail-modal"', "Capability detail must render in a modal card.");
requireIncludes(files.capabilityConsole, "<CapabilityInspector", "Capability detail modal must reuse the capability inspector content.");
requireNotIncludes(files.capabilityConsole, "<aside className=\"capability-inspector\"", "Capability console must not restore a permanent right-side detail inspector.");

requireIncludes(files.shell, "SystemSettingsDrawer", "AdminShell must expose account/system settings through the shared drawer.");
requireIncludes(files.shell, "profile-menu-trigger", "AdminShell must keep the personal profile trigger in the topbar.");
requireIncludes(files.shell, "settings-trigger-button", "AdminShell must keep a separate system settings trigger in the topbar.");
requireOrder(files.shell, 'className="topbar-icon-button settings-trigger-button"', 'className="profile-menu-trigger"', "AdminShell must keep the profile avatar as the right-most topbar action.");
requireIncludes(files.shell, 'trigger={["click"]}', "Profile dropdown must be click-triggered.");
requireNotIncludes(files.shell, 'trigger={["click", "hover"]}', "Profile dropdown must not open on hover.");
requireNotIncludes(files.shell, "profile-menu-name", "Profile avatar trigger must not render the user name.");
requireIncludes(files.shell, "handleProfileOutsidePointerDown", "Profile dropdown must close when clicking outside the profile card.");
requireIncludes(files.shell, 'document.addEventListener("pointerdown", handleProfileOutsidePointerDown, true)', "Profile outside-click handling must run before portal overlays swallow blank-area clicks.");
requireIncludes(files.shell, "ProfileEditorModal", "AdminShell must expose a full profile editor modal from the profile panel.");
requireIncludes(files.shell, "dictionary.changePassword", "Profile editor modal must expose a change-password action slot.");
requireIncludes(files.shell, "dictionary.resetPassword", "Profile editor modal must expose a reset-password action slot.");
requireCssRule(files.styles, ".profile-editor-modal .ant-modal-body", ["max-height: min(720px, calc(100vh - 260px));", "overflow-y: auto;", "overscroll-behavior: contain;"], "Profile editor modal body must stay scrollable within the viewport.");
requireIncludes(files.shell, "roles.map((role) => <Tag key={role}>{role}</Tag>)", "Profile summary must display role names instead of a role count.");
requireIncludes(files.shell, 'bodyClassName="profile-summary-body"', "Profile summary dropdown must scroll its body instead of clipping fields.");
requireIncludes(files.shell, 'maxHeight="min(640px, calc(100vh - 72px))"', "Profile summary dropdown must reserve enough viewport height.");
requireCssRule(files.styles, ".platform-dropdown-panel", ["display: flex;", "flex-direction: column;", "overflow: hidden;"], "Platform dropdown panel must keep header/body/footer in a scroll-safe column.");
requireCssRule(files.styles, ".platform-dropdown-panel-body", ["flex: 1 1 auto;", "min-height: 0;", "overflow: auto;"], "Platform dropdown body must own overflow scrolling.");
requireCssRule(files.styles, ".profile-summary-body", ["overflow-y: auto;", "overscroll-behavior: contain;"], "Profile summary body must remain scrollable.");
requireIncludes(files.shell, "language-toggle-button", "AdminShell must use an icon language toggle, not a language dropdown.");
requireCountExactly(files.shell, 'className="brand-collapse-button"', 1, "AdminShell must expose one sidebar collapse affordance in the brand area.");
requireNotIncludes(files.shell, 'className="desktop-sider-toggle"', "AdminShell must not render a second desktop sidebar collapse affordance in the topbar.");
requireIncludes(files.shell, "platform-work-tabs", "AdminShell must keep browser-like work tabs.");
requireIncludes(files.shell, 'className="platform-work-tabs-shell"', "AdminShell must keep work-tab overflow controls in a stable shell.");
requireIncludes(files.shell, "const [workTabsScroll, setWorkTabsScroll]", "AdminShell must track work-tab overflow state for directional scrolling.");
requireIncludes(files.shell, "ResizeObserver", "AdminShell must update work-tab overflow when the workbar resizes.");
requireIncludes(files.shell, "MutationObserver", "AdminShell must update work-tab overflow when tabs are added or removed.");
requireIncludes(files.shell, "onWheel={handleWorkTabsWheel}", "AdminShell must allow wheel scrolling inside overflowing work tabs.");
requireIncludes(files.shell, 'aria-current={active ? "page" : undefined}', "AdminShell work tabs must expose the active tab state.");
requireIncludes(files.shell, "disabled={!workTabsScroll.left}", "AdminShell must disable the left work-tab scroll button when it cannot scroll.");
requireIncludes(files.shell, "disabled={!workTabsScroll.right}", "AdminShell must disable the right work-tab scroll button when it cannot scroll.");
requireIncludes(files.shell, "workTabMenuItems", "AdminShell work tabs must keep the close/current/left/right context menu.");
requireIncludes(files.shell, "dictionary.globalSearchNoResults", "Global search must expose a localized no-results state.");
requireIncludes(files.shell, "open={Boolean(globalSearchQuery.trim())}", "Global search must keep its result surface open for no-results feedback.");
requireIncludes(files.shell, "items: globalSearchMenuItems", "Global search must use the result-or-empty menu contract.");
requireIncludes(files.i18n, 'topSearch: "搜索可访问页面..."', "Global search copy must describe page navigation instead of unavailable data, API or doc search.");
requireIncludes(files.i18n, 'topSearch: "Search accessible pages..."', "Global search copy must describe page navigation instead of unavailable data, API or doc search.");
requireNotIncludes(files.i18n, 'topSearch: "搜索能力、接口、文档..."', "Global search must not imply unavailable API or document search.");
requireNotIncludes(files.i18n, 'topSearch: "Search capabilities, APIs, docs..."', "Global search must not imply unavailable API or document search.");
requireCountExactly(files.i18n, "sessionRefreshList:", 2, "Session console must declare localized refresh action copy.");
requireCountExactly(files.i18n, "sessionColumnSettings:", 2, "Session console must declare localized column action copy.");
requireIncludes(files.sessionConsole, "refresh: dictionary.sessionRefreshList", "Session console refresh action must use read-only session-specific copy.");
requireIncludes(files.sessionConsole, "columns: dictionary.sessionColumnSettings", "Session console column action must use read-only session-specific copy.");
requireIncludes(files.shell, "buildNavigationTree", "AdminShell must build multi-level navigation from resource parents.");
requireIncludes(files.shell, "operations: dictionary.operations", "AdminShell must localize the legacy operations navigation directory.");
requireIncludes(files.i18n, "operations: { zh: dictionaries.zh.operations, en: dictionaries.en.operations }", "Menu governance must localize projected legacy operations directories in both languages.");
requireIncludes(files.shell, '"business/access"', "AdminShell must label business/access through a full parent path.");
requireIncludes(files.shell, '"business/dispatch"', "AdminShell must label business/dispatch through a full parent path.");
requireNotIncludes(files.shell, "access: dictionary.navBusinessAccess", "AdminShell must not reuse plain access for business navigation labels.");
requireIncludes(files.shell, 'href="#platform-main-content"', "AdminShell must expose a skip-to-content link.");
requireIncludes(files.shell, 'id="platform-main-content"', "AdminShell main region must expose a stable focus target.");
requireCountExactly(files.shell, 'className="platform-watermark-layer"', 1, "AdminShell must render exactly one screen watermark layer.");
requireRegex(
  files.shell,
  /<div className=\{shellClass\}[^>]*>\s*<a className="platform-skip-link"[\s\S]*?<\/a>\s*\{showScreenWatermark\s*\?\s*\(\s*<div className="platform-watermark-layer"/,
  "Screen watermark must be mounted directly under .platform-shell so it covers navigation, data surfaces, and overlays.",
);
requireIncludes(files.shell, 'data-scope="viewport"', "Screen watermark must declare viewport scope.");
requireNotRegex(
  files.shell,
  /<section className="platform-content">[\s\S]*?className="platform-watermark-layer"[\s\S]*?<\/section>/,
  "Screen watermark must not be nested under .platform-content.",
);
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
requireRegex(
  files.styles,
  /\.platform-sider\s*\{[\s\S]*?height:\s*100dvh;[\s\S]*?min-height:\s*0;[\s\S]*?overflow:\s*hidden;/,
  "Desktop sidebar must stay within the viewport so expanded navigation can scroll.",
);
requireRegex(
  files.styles,
  /\.side-nav\s*\{[\s\S]*?min-height:\s*0;[\s\S]*?overflow-y:\s*auto;/,
  "Sidebar navigation must own vertical scrolling inside its fixed-height sidebar.",
);
requireRegex(
  files.styles,
  /\.platform-secondary-nav\s*\{[\s\S]*?height:\s*100dvh;[\s\S]*?min-height:\s*0;[\s\S]*?overflow:\s*hidden;/,
  "Split-layout secondary navigation must preserve the same viewport scroll boundary.",
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
requireIncludes(files.settings, "<Switch aria-label={label}", "Shared settings switches must expose their visible label as an accessible name.");
requireIncludes(files.settings, 'className="settings-switch-hit-target"', "Shared settings switches must expose a 44px pointer target without replacing the native switch.");
requireIncludes(files.settings, 'role="group"', "Watermark scope choices must expose group semantics for their accessible name.");
requireRegex(files.styles, /\.settings-switch-hit-target\s*\{[\s\S]*?min-height:\s*44px[\s\S]*?min-width:\s*44px/, "Settings switch pointer targets must be at least 44px by 44px.");

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
requireIncludes(files.table, "Grid.useBreakpoint()", "PlatformDataTable must use AntD breakpoint state for rendered-column clarity.");
requireIncludes(files.table, "effectiveResponsiveBreakpoints", "PlatformDataTable must share one effective responsive rule for rendering and column settings.");
requireIncludes(files.table, "columnRenderedAtCurrentWidth", "PlatformDataTable must compute columns rendered at the current width.");
requireIncludes(files.table, "labels.selectedColumns", "Column settings must distinguish selected columns.");
requireIncludes(files.table, "labels.renderedColumns", "Column settings must report currently rendered columns.");
requireIncludes(files.table, "labels.hiddenAtCurrentWidth", "Column settings must explain breakpoint-hidden selected columns.");
requireRegex(
  files.table,
  /function columnPlainTextLabel<[\s\S]*?typeof column\.title === "string" \|\| typeof column\.title === "number"[\s\S]*?String\(column\.key\)/,
  "Column settings must derive stable plain-text labels and fall back to the column key for non-text titles.",
);
requireRegex(
  files.table,
  /const checkboxAccessibleLabel = hiddenAtCurrentWidth \? `\$\{columnLabel\} \$\{labels\.hiddenAtCurrentWidth\}` : columnLabel;[\s\S]*?<Checkbox aria-label=\{checkboxAccessibleLabel\}/,
  "Column settings must include the breakpoint-hidden state in each hidden checkbox accessible name.",
);
requireIncludes(
  files.table,
  '<span className="platform-column-option-label" title={columnLabel}>',
  "Column settings must expose each full plain-text label through the option title attribute.",
);
requireIncludes(
  files.table,
  ".some((breakpoint) => screens[breakpoint])",
  "PlatformDataTable must treat a column as rendered when any responsive breakpoint is active.",
);
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
requireIncludes(files.client, "export type AdminResourceFieldMasking", "Admin API client must expose field masking metadata.");
requireIncludes(
  files.client,
  'strategy: "partial-v1" | "phone-v1" | "email-v1" | "identity-cn-v1" | "address-cn-v1";',
  "Admin field masking metadata must expose every supported versioned strategy.",
);
for (const key of ["preservePrefix", "preserveSuffix", "maskLength"]) {
  requireIncludes(files.client, `${key}?: number;`, `Admin field masking metadata must expose optional ${key}.`);
}
requireIncludes(files.client, "replacement?: string;", "Admin field masking metadata must expose an optional replacement rune.");
requireIncludes(files.client, "masking?: AdminResourceFieldMasking;", "AdminResourceField must carry optional masking metadata.");
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
requireIncludes(files.resourceConsole, "operator: isEncryptedExactMatchField(field)", "Encrypted resource filters must submit exact-match conditions.");
requireIncludes(files.resourceConsole, 'isEncryptedExactMatchField(field) && match[2] !== "="', "Encrypted resource query syntax must allow equality only.");
requireIncludes(files.resourceConsole, 'field.storageMode === "encrypted" && Boolean(field.protection?.blindIndexNamespace)', "Encrypted resource filters must be driven by declared blind-index metadata.");
requireIncludes(files.treeSelect, "export function PlatformTreeSelect", "PlatformTreeSelect must exist as the shared tree relation control.");
requireIncludes(files.treeSelect, "platformPopupContainer", "PlatformTreeSelect dropdowns must render inside the platform popup container.");
requireIncludes(files.treeSelect, "treeDataFromOptions", "PlatformTreeSelect must build nested tree data from parentValue metadata.");

requireIncludes(files.primitives, "forceRender = true", "AdminFormModal must force render by default so hidden forms can receive edit values.");
requireIncludes(files.primitives, "destroyOnHidden = true", "AdminFormModal must destroy hidden forms by default to avoid stale field state.");

requireIncludes(files.resourceConsole, "const provider = dataProvider();", "GenericResourceConsole relation options must use the Refine data provider.");
requireIncludes(files.resourceConsole, "provider.getList<AdminResourceRecord>", "Relation option loading must query target resources through dataProvider.getList.");
requireNotIncludes(files.resourceConsole, "queryAdminResource(", "GenericResourceConsole must not bypass Refine CRUD for relation option loading.");
requireIncludes(files.resourceConsole, "dictionary.relationOptionsLoadFailed", "Relation option failures must use localized i18n copy.");
requireIncludes(files.resourceConsole, "RELATION_OPTION_PAGE_SIZE = 30", "Relation option loading must use a bounded server page instead of a fixed first-100 fetch.");
requireIncludes(files.resourceConsole, "pagination: { currentPage: 1, pageSize: 1, mode: \"server\" }", "Selected relation values outside the search page must be hydrated through a targeted server query.");
requireIncludes(files.resourceConsole, "keywords: input.search?.trim() ? [input.search.trim()] : undefined", "Relation option search must reach the structured server keyword contract.");
requireIncludes(files.resourceConsole, "relationSearchScheduler.isCurrent", "Relation option search must discard stale responses.");
requireIncludes(files.resourceConsole, "const RELATION_OPTION_SEARCH_DELAY_MS = 250;", "Relation option search must use the platform 250ms debounce interval.");
requireIncludes(files.resourceConsole, "createRelationOptionSearchScheduler", "Relation option search must use the shared session-aware scheduler.");
requireRegex(
  files.resourceConsole,
  /const closeFormModal = \(\) => \{\s*relationSearchScheduler\.invalidateAll\(\);\s*setRelationLoadingFields\(\{\}\);\s*setModalOpen\(false\);\s*\};/,
  "Resource form close must invalidate relation searches and clear their loading state.",
);
requireIncludes(files.resourceConsole, "onCancel={closeFormModal}", "Resource form cancellation must close the relation search session.");
requireRegex(
  files.resourceConsole,
  /setSelectedID\(String\(result\.id\)\);\s*closeFormModal\(\);/,
  "A successful save must close the relation search session.",
);
requireRegex(
  files.resourceConsole,
  /useEffect\(\(\) => \{\s*relationSearchScheduler\.invalidateAll\(\);\s*setRelationLoadingFields\(\{\}\);\s*return \(\) => \{\s*relationSearchScheduler\.invalidateAll\(\);\s*\};\s*\}, \[relationSearchScheduler, resourceKey\]\);/,
  "Resource cleanup must invalidate relation searches on resource change and unmount.",
);
requireIncludes(files.relationOptionSearch, "currentGenerations", "Relation option search must isolate current work by field generation.");
requireIncludes(files.relationOptionSearch, "timers", "Relation option search must debounce independently per relation field.");
requireIncludes(files.relationOptionSearch, "clearTimer(pendingTimer)", "Relation option search must cancel superseded pending requests.");
requireIncludes(files.relationOptionSearch, "currentGenerations.clear()", "Relation option search must invalidate in-flight generations when a form session closes.");
requireIncludes(files.resourceConsole, "relationValuesFromInput(form.getFieldValue(field.key))", "Relation option search must retain selected values while the remote option page changes.");
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
requireIncludes(files.resourceConsole, 'const initializationKey = editingRecord ? `edit:${editingRecord.id}` : `create:${resourceKey}`;', "Form initialization must be scoped to the current create or edit modal lifecycle.");
requireIncludes(files.resourceConsole, "if (initializedFormKeyRef.current === initializationKey)", "Async relation and organization context updates must not reset an already initialized form.");
requireIncludes(files.resourceConsole, 'const values = editingRecord ? formValuesFromRecord(editingRecord, formFields) : defaultFormValues(formFields);', "Editing a resource must hydrate form values from the selected record while create forms use schema defaults.");
requireIncludes(files.resourceConsole, "return experience.initialValues?.(values, editingRecord) ?? values;", "Generic resource forms must apply experience-owned initial values before opening.");
requireIncludes(files.resourceConsole, "initializedFormKeyRef.current = initializationKey;", "Form initialization must record the active modal lifecycle before resetting fields.");
requireOrder(files.resourceConsole, "initializedFormKeyRef.current = initializationKey;", "form.resetFields();", "Form initialization must record its lifecycle key before resetting fields.");
requireIncludes(files.resourceConsole, "form.setFieldsValue(activeFormInitialValues)", "Form initialization must apply the active resource experience initial values.");
requireOrder(files.resourceConsole, 'initializedFormKeyRef.current = "";', "form.setFieldsValue(activeFormInitialValues)", "Closing a form must clear its initialization key before a later modal lifecycle can initialize.");
requireIncludes(files.resourceConsole, "field.type === \"multiselect\"", "Form hydration must preserve multiselect arrays for relation fields.");
requireIncludes(files.resourceConsole, "type SchemaValue = string | number | boolean | null | SchemaValue[] | { [key: string]: SchemaValue };", "Generic resource writes must define a JSON-compatible typed schema value boundary.");
requireIncludes(files.resourceConsole, "function schemaValueFromFormValue(value: unknown): SchemaValue | undefined", "Generic resource writes must normalize form values through the schema value boundary.");
requireIncludes(files.resourceConsole, "const nestedValues: Record<string, SchemaValue> = {};", "Generic resource writes must preserve typed nested schema values.");
requireIncludes(files.resourceConsole, "return isSchemaValue(value) ? value : undefined;", "Generic resource writes must reject non-schema values without flattening valid arrays, booleans or numbers.");
requireIncludes(files.resourceConsole, "function parseListValue(value: string)", "Generic resource hydration must parse JSON arrays while keeping legacy delimiter values compatible.");
requireNotIncludes(files.resourceConsole, "function serializeFieldValue", "Generic resource writes must not reintroduce string-only serialization.");
requireNotIncludes(files.resourceConsole, "JSON.stringify(raw.map((item) => String(item)))", "Generic resource writes must not stringify multiselect arrays.");
requireIncludes(files.dataProvider, "type SchemaValue = string | number | boolean | null | SchemaValue[] | { [key: string]: SchemaValue };", "Refine writes must preserve JSON-compatible schema values.");
requireIncludes(files.dataProvider, "isSchemaValueMap(source.values)", "Refine writes must accept typed schema value maps from resource forms.");
requireIncludes(files.dataProvider, "if (isSchemaValue(value))", "Refine writes must keep valid non-string field values from variables.");
requireNotIncludes(files.dataProvider, "storageValue(value)", "Refine writes must not stringify typed schema values.");
requireRegex(
  files.resourceConsole,
  /function formValueFromRecord\(record: AdminResourceRecord, field: AdminResourceField\) \{\s*if \(field\.storageMode === "encrypted"\) \{\s*return undefined;\s*\}/,
  "Encrypted edit fields must hydrate blank instead of reusing projected values or create defaults.",
);
requireIncludes(
  files.resourceConsole,
  'field.required && !(editing && field.storageMode === "encrypted")',
  "Encrypted edit fields must allow a blank value to preserve the current secret.",
);
requireIncludes(files.resourceConsole, "parts.push(dictionary.encryptedFieldEditHint);", "Encrypted edit fields must expose the localized blank-preserves-current-value hint.");
requireCountExactly(files.i18n, "encryptedFieldEditHint:", 2, "Encrypted edit field guidance must exist in matching Chinese and English dictionaries.");
requireIncludes(
  files.resourceConsole,
  'schema.fields.filter((field) => field.inTable && field.responseMode !== "omitted")',
  "Generic resource tables must not render omitted response fields.",
);
requireIncludes(
  files.resourceConsole,
  'schema.fields.filter((field) => field.inDetail && field.responseMode !== "omitted")',
  "Generic resource details must not render omitted response fields.",
);
requireIncludes(
  files.resourceConsole,
  "inputFromRecord(record, schema.fields, { status: nextStatus })",
  "Status updates must use schema-aware record input filtering.",
);
requireRegex(
  files.resourceConsole,
  /function inputFromRecord\([\s\S]*?fields\s*\.filter\(\(field\) => field\.source === "values" && !field\.readOnly && field\.sensitivity === "public" && field\.storageMode !== "encrypted" && field\.responseMode !== "omitted" && field\.responseMode !== "privileged"\)[\s\S]*?values: \{ \.\.\.safeValues, \.\.\.\(overrides\.values \?\? \{\}\) \}/,
  "Status updates must exclude encrypted, hidden and non-writable values from mutation payloads.",
);
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
requireIncludes(files.policyReview, 'permissionAllows(permissions, "admin:policy-review:export", deniedPermissions)', "PolicyReviewConsole must derive export access from the dedicated permission.");
requireIncludes(files.client, "export function exportAdminPolicyReviews({ watermark }: { watermark: boolean })", "Policy review export must require an explicit watermark intent.");
requireIncludes(files.client, '`/admin/policy-reviews/export?watermark=${watermark}`', "Policy review export must pass watermark intent through the dedicated query parameter.");
requireIncludes(files.app, 'uiConfig.watermark && uiConfig.watermarkScopes.includes("export")', "App must derive policy-review export watermark intent from the shared UI configuration.");
requireIncludes(files.policyReview, "exportAdminPolicyReviews({ watermark: exportWatermark })", "PolicyReviewConsole must pass the derived export watermark intent to the API client.");
requireRegex(files.policyReview, /\{canExport\s*\?\s*\([\s\S]*?<AdminActionButton[\s\S]*?policyReviewExportEvidence[\s\S]*?\)\s*:\s*null\}/, "PolicyReviewConsole must remove the export button from unauthorized focus order.");
requireIncludes(files.policyReview, 'valueOf(audit, "targetId") === review.id', "PolicyReviewConsole audit matching must use stable target IDs.");
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
requireIncludes(files.styles, ".context-chip:not(.context-readonly):hover", "Read-only runtime context chips must not expose interactive hover feedback.");
requireIncludes(files.styles, ".context-chip:not(.context-readonly):focus-visible", "Read-only runtime context chips must not expose interactive focus feedback.");
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
requireCssRule(
  files.styles,
  ".platform-watermark-layer",
  ["position: fixed;", "inset: 0;", "z-index: 2200;", "pointer-events: none;"],
  "Screen watermark must remain a fixed, full-viewport, non-interactive overlay above the Admin shell.",
);
requireRegex(
  files.styles,
  /\.platform-watermark-layer\[data-count="16"\] span:nth-child\(-n \+ 4\)[\s\S]*?align-self:\s*start;/,
  "Multi-watermark layouts must place their first row against the viewport edge so the topbar is visibly watermarked.",
);
requireRegex(
  files.styles,
  /\.platform-watermark-layer\[data-count="16"\] span:nth-child\(4n \+ 1\)[\s\S]*?justify-self:\s*start;/,
  "Multi-watermark layouts must place their first column against the viewport edge so the sidebar is visibly watermarked.",
);
requireRegex(
  files.styles,
  /@media\s*\(max-width:\s*768px\)[\s\S]*?\.platform-watermark-layer\[data-count="16"\]\s*\{[\s\S]*?grid-template-columns:\s*repeat\(2,\s*minmax\(0,\s*1fr\)\);[\s\S]*?grid-template-rows:\s*repeat\(8,\s*minmax\(0,\s*1fr\)\);/,
  "Narrow viewports must reflow sixteen watermarks to two columns so attribution text remains readable.",
);
requireRegex(files.styles, /:focus-visible[\s\S]*outline:\s*2px solid var\(--primary\)/, "Visible focus must be a default platform behavior.");
requireRegex(files.styles, /@media\s*\(prefers-reduced-motion:\s*reduce\)/, "styles.css must respect reduced motion.");
requireRegex(
  files.styles,
  /@media\s*\(prefers-reduced-motion:\s*reduce\)[\s\S]*\.login-page \*[\s\S]*transition-duration:\s*0\.01ms !important;/,
  "Reduced motion must suppress non-essential login transitions.",
);
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
requireCssRule(mobileStyles, ".platform-topbar .profile-menu-trigger", ["min-width: 44px;", "height: 44px;", "min-height: 44px;"], "Mobile profile trigger must expose a 44px touch target.");
requireCssRule(mobileStyles, ".platform-topbar .settings-trigger-button", ["min-width: 44px;", "height: 44px;", "min-height: 44px;"], "Mobile settings trigger must expose a 44px touch target.");
requireCssRule(
  mobileStyles,
  ".login-submit,\n  .login-oidc-action,\n  .login-recovery-action",
  ["min-height: 44px;"],
  "Mobile login submit, OIDC, and recovery actions must expose 44px touch targets.",
);
requireCssRule(
  tabletLoginStyles,
  ".login-submit,\n  .login-oidc-action,\n  .login-recovery-action",
  ["min-height: 44px;"],
  "Tablet login submit, OIDC, and recovery actions must expose 44px touch targets.",
);
requireCssRule(
  tabletLoginStyles,
  ".login-panel-toolbar .topbar-icon-button,\n  .login-theme-swatch",
  ["min-width: 44px;", "min-height: 44px;"],
  "Tablet login toolbar controls must expose 44px touch targets.",
);
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

function readSourceOptional(relativePath) {
  try {
    return readSource(relativePath);
  } catch (error) {
    if (error && typeof error === "object" && "code" in error && error.code === "ENOENT") return "";
    throw error;
  }
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

function requireNotRegex(source, pattern, message) {
  if (pattern.test(source)) {
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

function sourceRange(source, start, end) {
  const startIndex = source.indexOf(start);
  const endIndex = source.indexOf(end, startIndex + start.length);
  if (startIndex === -1 || endIndex === -1) return "";
  return source.slice(startIndex, endIndex);
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
