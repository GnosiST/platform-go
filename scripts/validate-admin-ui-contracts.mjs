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
  capabilityMetadata: readSource("admin/src/platform/capabilities/metadata.ts"),
  client: readSource("admin/src/platform/api/client.ts"),
  organizationRBAC: readSource("admin/src/platform/api/organizationRBAC.ts"),
  sessionExpiry: readSource("admin/src/platform/api/sessionExpiry.ts"),
  i18n: readSource("admin/src/platform/i18n.ts"),
  primitives: readSource("admin/src/platform/ui/AdminPrimitives.tsx"),
  resourceConsole: readSource("admin/src/platform/resources/GenericResourceConsole.tsx"),
  resourceExperience: readSource("admin/src/platform/resources/resourceExperience.ts"),
  organizationUserExperience: readSource("admin/src/platform/resources/organizationUserExperience.tsx"),
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
  uiIndex: readSource("admin/src/platform/ui/index.ts"),
  styles: readSource("admin/src/styles.css"),
};

const failures = [];
const mobileStyles = extractCssBlock(files.styles, "@media (max-width: 767px)");
const tabletLoginStyles = extractCssBlock(files.styles, "@media (max-width: 1024px)");

requireIncludes(files.app, "readStoredUIConfig", "App must keep persisted admin UI configuration.");
requireIncludes(files.app, "writeStorageValue(adminPreferenceStorageKeys.ui", "App must persist admin UI configuration changes.");
requireIncludes(files.app, "defaultAdminUIConfig", "App must fall back to the shared default admin UI config.");
requireIncludes(files.app, "disableTelemetry: true", "The reusable Admin foundation must disable Refine third-party telemetry by default.");
requireIncludes(files.app, "PolicyReviewConsole", "App must mount the policy-review custom governance console when the resource is enabled.");
requireIncludes(files.app, 'resource.route !== "/policy-reviews"', "Generic resource routing must not also mount policy-reviews when the custom console is active.");
requireIncludes(files.client, "export class AdminAPIError", "Admin API failures must expose typed status codes.");
requireIncludes(files.client, "ADMIN_SESSION_EXPIRED_EVENT", "The shared client must expose the session-expired event contract.");
requireIncludes(files.client, "shouldExpireAdminSession", "The shared client must centralize session-expiry decisions.");
requireIncludes(files.client, "handleUnauthorizedResponse(response.status, requestToken, error?.code)", "Session expiry must consider the structured API error code.");
requireIncludes(files.sessionExpiry, 'errorCode !== "ADMIN_SENSITIVE_REVEAL_VERIFICATION_FAILED"', "Failed sensitive reveal verification must preserve the authenticated Admin session.");
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
requireIncludes(files.organizationUserExperience, 'role="textbox" aria-readonly="true"', "Read-only role values must expose a valid read-only widget semantic.");
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
