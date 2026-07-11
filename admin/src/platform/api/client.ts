const API_BASE = import.meta.env.VITE_PLATFORM_API_BASE ?? "/api";
const AUTH_TOKEN_KEY = "platform.auth.token";

export const ADMIN_SESSION_EXPIRED_EVENT = "platform:auth:session-expired";

export class AdminAPIError extends Error {
  constructor(
    message: string,
    readonly statusCode: number,
    readonly code = "",
  ) {
    super(message);
    this.name = "AdminAPIError";
  }
}

export type PlatformResponse<T> = {
  data?: T;
  error?: {
    code: string;
    message: string;
  };
};

export type CapabilityItem = {
  id: string;
  name: string;
  version: string;
};

export type BrandingConfig = {
  productName: string;
  shortName: string;
  logoUrl: string;
  faviconUrl: string;
  primaryColor: string;
  defaultTheme: string;
  loginTitle: string;
  loginSubtitle: string;
  supportEmail: string;
};

export type AuthProvider = {
  id: string;
  kind: "demo" | "wechat" | string;
  title: LocalizedText;
  description: LocalizedText;
  enabled: boolean;
  configured: boolean;
  audiences: string[];
  configKeys?: string[];
};

export type AuthProviderList = {
  items: AuthProvider[];
};

export type AuthLoginInput = {
  provider: string;
  username?: string;
  code?: string;
  state?: string;
  codeVerifier?: string;
};

export type AuthProviderStartResult = {
  authorizationUrl: string;
  state: string;
  expiresAt: string;
};

export type AuthLoginResult = {
  token: string;
  expiresAt: string;
  principal: AdminCurrentSession;
};

export type AdminCurrentSession = {
  user: {
    id: string;
    username: string;
    name: string;
    tenantCode?: string;
    orgUnitCode?: string;
    areaCode?: string;
  };
  roles: string[];
  permissions: string[];
  deniedPermissions?: string[];
};

export type AdminMenuItem = {
  name: string;
  route: string;
  parent: string;
  isExternal: boolean;
  cacheEnabled: boolean;
  resource: string;
  title: LocalizedText;
  description: LocalizedText;
  permission: string;
  group: string;
  icon: string;
  order: number;
};

export type AdminMenuList = {
  items: AdminMenuItem[];
};

export type AdminDemoDataItem = {
  id: string;
  capabilityId: string;
  resource: string;
  title: LocalizedText;
  description: LocalizedText;
  records: number;
};

export type AdminDemoDataList = {
  items: AdminDemoDataItem[];
};

export type AdminDemoDataApplyResult = {
  id: string;
  capabilityId: string;
  resource: string;
  applied: number;
};

export type AdminOpenAPIOperation = {
  operationId?: string;
  summary?: string;
  description?: string;
  tags?: string[];
  "x-platform-resource"?: string;
  "x-platform-permission"?: string;
  "x-platform-codegen-mode"?: string;
};

export type AdminOpenAPIDocument = {
  openapi: string;
  info: {
    title: string;
    version: string;
    description?: string;
  };
  paths: Record<string, Record<string, AdminOpenAPIOperation>>;
  tags?: Array<{ name: string; description?: string }>;
  components?: {
    schemas?: Record<string, unknown>;
    securitySchemes?: Record<string, unknown>;
  };
  "x-generated-by"?: string;
  "x-source"?: string;
  "x-source-version"?: string;
};

export type AdminResourceRecord = {
  id: string;
  code: string;
  name: string;
  status: string;
  description?: string;
  updatedAt: string;
  values?: Record<string, string>;
};

export type AdminResourceInput = {
  code?: string;
  name: string;
  status?: string;
  description?: string;
  values?: Record<string, string>;
};

export type LocalizedText = {
  zh: string;
  en: string;
};

export type AdminResourceFieldOption = {
  value: string;
  label: LocalizedText;
};

export type AdminResourceFormGroup = {
  key: string;
  label: LocalizedText;
  description?: LocalizedText;
};

export type AdminResourceFormLayout = "single-column" | "grouped-sections" | "two-column-density" | "side-detail-preview";
export type AdminResourceRuntimeSlotRegion = "form.header" | "form.section.before" | "form.section.after" | "form.footer" | "field.control" | "side.preview";
export type AdminResourceRuntimeSlotVariant = "compact" | "info" | "warning" | "preview" | "inline";
export type AdminResourceRuntimeSlotDataBindingMode = "record" | "formValues" | "resource" | "none";

export type AdminResourceRuntimeSlotDataBinding = {
  mode?: AdminResourceRuntimeSlotDataBindingMode;
  fields?: string[];
};

export type AdminResourceRuntimeSlot = {
  slotId: string;
  region: AdminResourceRuntimeSlotRegion;
  label: LocalizedText;
  description: LocalizedText;
  permission?: string;
  visibleWhen?: string;
  targetSection?: string;
  targetField?: string;
  dataBinding?: AdminResourceRuntimeSlotDataBinding;
  variant?: AdminResourceRuntimeSlotVariant;
  order?: number;
};

export type AdminResourceFieldValidation = {
  minLength?: number;
  maxLength?: number;
  min?: number;
  max?: number;
  pattern?: string;
};

export type AdminResourceFieldRelationFilter = {
  field: string;
  operator: AdminResourceQueryOperator;
  value: string;
};

export type AdminResourceFieldRelation = {
  resource: string;
  valueField: string;
  labelField: string;
  multiple?: boolean;
  filters?: AdminResourceFieldRelationFilter[];
  sortField?: string;
  sortOrder?: "asc" | "desc";
  display?: "select" | "tree";
  parentField?: string;
  pathField?: string;
  rootValue?: string;
};

export type AdminResourceField = {
  key: string;
  label: LocalizedText;
  type: "text" | "textarea" | "select" | "multiselect" | "datetime" | "switch" | "number" | "color";
  source: "record" | "values";
  group?: string;
  help?: LocalizedText;
  required?: boolean;
  readOnly?: boolean;
  searchable?: boolean;
  filterable?: boolean;
  sortable?: boolean;
  localizable?: boolean;
  inTable?: boolean;
  inForm?: boolean;
  inDetail?: boolean;
  width?: number;
  options?: Array<AdminResourceFieldOption & { parentValue?: string; pathValue?: string }>;
  relation?: AdminResourceFieldRelation;
  validation?: AdminResourceFieldValidation;
  sensitivity: "public" | "internal" | "personal" | "sensitive" | "secret";
  storageMode: "plain" | "masked" | "hashed" | "encrypted";
  responseMode: "full" | "masked" | "privileged" | "omitted";
  exportMode: "full" | "masked" | "privileged" | "omitted";
};

export type AdminResourcePermissions = {
  read: string;
  create: string;
  update: string;
  delete: string;
};

export type AdminResourceActionConfirm = {
  title: LocalizedText;
  description?: LocalizedText;
  okText?: LocalizedText;
};

export type AdminResourceAction = {
  key: string;
  label: LocalizedText;
  kind: "row" | "batch" | "resource";
  tone?: "default" | "primary" | "danger" | "warning";
  icon?: string;
  permission: string;
  route?: string;
  method?: "GET" | "POST" | "PUT" | "PATCH" | "DELETE";
  confirm?: AdminResourceActionConfirm;
  auditAction?: string;
  refresh?: boolean;
};

export type AdminResourcePanel = {
  key: string;
  label: LocalizedText;
  kind: "fields" | "permissions" | "audit" | "approval" | "files" | "custom";
  permission?: string;
  component?: string;
  order?: number;
  empty?: LocalizedText;
};

export type AdminResourceSchema = {
  resource: string;
  title: LocalizedText;
  description: LocalizedText;
  permissions: AdminResourcePermissions;
  formGroups?: AdminResourceFormGroup[];
  formLayout?: AdminResourceFormLayout;
  fields: AdminResourceField[];
  actions?: AdminResourceAction[];
  panels?: AdminResourcePanel[];
  runtimeSlots?: AdminResourceRuntimeSlot[];
  searchFields: string[];
  defaultSortKey?: string;
};

export type AdminResourceList = {
  resource: string;
  items: AdminResourceRecord[];
};

export type AdminResourceQueryOperator = "contains" | "=" | "!=" | ">" | ">=" | "<" | "<=";

export type AdminResourceQueryCondition = {
  field: string;
  operator: AdminResourceQueryOperator;
  value: string;
};

export type AdminResourceQuerySort = {
  field: string;
  order: "asc" | "desc";
};

export type AdminResourceQueryInput = {
  keywords?: string[];
  conditions?: AdminResourceQueryCondition[];
  sort?: AdminResourceQuerySort[];
  page?: number;
  pageSize?: number;
};

export type AdminResourceQueryResult = AdminResourceList & {
  total: number;
  page: number;
  pageSize: number;
};

export type AdminResourceMutation = {
  resource: string;
  record: AdminResourceRecord;
  token?: string;
};

export type AdminResourceActionResult = {
  record?: AdminResourceRecord;
  transition?: unknown;
};

export type AdminPolicyReviewActionResult = {
  review: AdminResourceRecord;
  role?: AdminResourceRecord;
  audit?: AdminResourceRecord;
};

export type AdminPolicyReviewExport = {
  exportedBy: string;
  exportedAt: string;
  reviews: AdminResourceRecord[];
  audits: AdminResourceRecord[];
};

type PlatformResponseMode = "data" | "raw" | "blob";
type PlatformRequestInit = RequestInit & {
  auth?: "stored-token" | "none";
};

function handleUnauthorizedResponse(statusCode: number, requestToken: string) {
  if (statusCode !== 401 || !requestToken || getAuthToken() !== requestToken) {
    return;
  }
  clearAuthToken();
  window.dispatchEvent(new Event(ADMIN_SESSION_EXPIRED_EVENT));
}

async function parsePlatformResponse<T>(response: Response, requestToken: string, mode: PlatformResponseMode = "data"): Promise<T> {
  if (mode === "blob" && response.ok) {
    return (await response.blob()) as T;
  }

  let payload: unknown;
  try {
    payload = await response.json();
  } catch {
    payload = undefined;
  }

  const error = payload && typeof payload === "object" && "error" in payload
    ? (payload as PlatformResponse<unknown>).error
    : undefined;
  if (!response.ok || error || payload === undefined) {
    handleUnauthorizedResponse(response.status, requestToken);
    throw new AdminAPIError(error?.message ?? `HTTP ${response.status}`, response.status, error?.code);
  }

  if (mode === "raw") {
    return payload as T;
  }
  return (payload as PlatformResponse<T>).data as T;
}

export async function request<T>(path: `/${string}`, init: PlatformRequestInit = {}): Promise<T> {
  const { auth = "stored-token", ...fetchInit } = init;
  const requestToken = auth === "stored-token" ? getAuthToken() : "";
  const headers = new Headers(fetchInit.headers);
  if (fetchInit.body && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  if (requestToken && !headers.has("Authorization")) {
    headers.set("Authorization", `Bearer ${requestToken}`);
  }
  const response = await fetch(`${API_BASE}${path}`, {
    ...fetchInit,
    headers,
  });
  return parsePlatformResponse<T>(response, requestToken);
}

export function getAuthToken() {
  if (typeof window === "undefined") {
    return "";
  }
  return window.localStorage.getItem(AUTH_TOKEN_KEY) ?? "";
}

export function setAuthToken(token: string) {
  if (typeof window === "undefined") {
    return;
  }
  if (token) {
    window.localStorage.setItem(AUTH_TOKEN_KEY, token);
    return;
  }
  window.localStorage.removeItem(AUTH_TOKEN_KEY);
}

export function clearAuthToken() {
  setAuthToken("");
}

export function listCapabilities() {
  return request<CapabilityItem[]>("/capabilities");
}

export function getBrandingConfig() {
  return request<BrandingConfig>("/platform/branding");
}

export function listAuthProviders() {
  return request<AuthProviderList>("/auth/providers", { auth: "none" });
}

export function startAdminAuthProvider(provider: string, codeChallenge: string) {
  return request<AuthProviderStartResult>(`/auth/providers/${encodeURIComponent(provider)}/start`, {
    auth: "none",
    method: "POST",
    body: JSON.stringify({ codeChallenge }),
  });
}

export async function loginWithAuthProvider(input: AuthLoginInput) {
  const result = await request<AuthLoginResult>("/auth/login", {
    auth: "none",
    method: "POST",
    body: JSON.stringify(input),
  });
  setAuthToken(result.token);
  return result;
}

export async function refreshCurrentSession() {
  const result = await request<AuthLoginResult>("/auth/refresh", {
    method: "POST",
  });
  setAuthToken(result.token);
  return result;
}

export async function logoutCurrentSession() {
  try {
    await request<{ revoked: boolean }>("/auth/logout", {
      method: "POST",
    });
  } finally {
    clearAuthToken();
  }
}

export function getCurrentAdminSession() {
  return request<AdminCurrentSession>("/admin/session/current");
}

export function listAdminMenus() {
  return request<AdminMenuList>("/admin/menus");
}

export function listAdminDemoData() {
  return request<AdminDemoDataList>("/admin/demo-data");
}

export async function getAdminOpenAPI() {
  const requestToken = getAuthToken();
  const response = await fetch(`${API_BASE}/openapi.json`, {
    headers: requestToken ? { Authorization: `Bearer ${requestToken}` } : undefined,
  });
  return parsePlatformResponse<AdminOpenAPIDocument>(response, requestToken, "raw");
}

export function applyAdminDemoData(capabilityId: string, datasetId: string) {
  return request<AdminDemoDataApplyResult>(`/admin/demo-data/${capabilityId}/${datasetId}/apply` as `/${string}`, {
    method: "POST",
  });
}

export function listAdminResource(resource: string) {
  return request<AdminResourceList>(`/admin/resources/${resource}` as `/${string}`);
}

export function queryAdminResource(resource: string, input: AdminResourceQueryInput) {
  return request<AdminResourceQueryResult>(`/admin/resources/${resource}/query` as `/${string}`, {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function getAdminResourceSchema(resource: string) {
  return request<AdminResourceSchema>(`/admin/resources/${resource}/schema` as `/${string}`);
}

export function createAdminResource(resource: string, input: AdminResourceInput) {
  return request<AdminResourceMutation>(`/admin/resources/${resource}` as `/${string}`, {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function updateAdminResource(resource: string, id: string, input: AdminResourceInput) {
  return request<AdminResourceMutation>(`/admin/resources/${resource}/${id}` as `/${string}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export function deleteAdminResource(resource: string, id: string) {
  return request<{ deleted: boolean; resource: string }>(`/admin/resources/${resource}/${id}` as `/${string}`, {
    method: "DELETE",
  });
}

export function executeAdminResourceAction(action: AdminResourceAction, record: AdminResourceRecord, metadata: Record<string, string> = {}) {
  if (!action.route) {
    throw new Error("Custom action route is not declared.");
  }
  const path = action.route.replace(":id", encodeURIComponent(record.id)).replace(/^\/api/, "") as `/${string}`;
  const method = action.method ?? "POST";
  return request<AdminResourceActionResult>(path, {
    method,
    body: method === "GET" || method === "DELETE" ? undefined : JSON.stringify(metadata),
  });
}

export function requestAdminPolicyReview(id: string) {
  return request<AdminPolicyReviewActionResult>(`/admin/policy-reviews/${encodeURIComponent(id)}/request` as `/${string}`, {
    method: "POST",
  });
}

export function approveAdminPolicyReview(id: string) {
  return request<AdminPolicyReviewActionResult>(`/admin/policy-reviews/${encodeURIComponent(id)}/approve` as `/${string}`, {
    method: "POST",
  });
}

export function rejectAdminPolicyReview(id: string, reason: string) {
  return request<AdminPolicyReviewActionResult>(`/admin/policy-reviews/${encodeURIComponent(id)}/reject` as `/${string}`, {
    method: "POST",
    body: JSON.stringify({ reason }),
  });
}

export function exportAdminPolicyReviews() {
  return request<AdminPolicyReviewExport>("/admin/policy-reviews/export", {
    method: "GET",
  });
}

export async function uploadAdminFile(file: File) {
  const requestToken = getAuthToken();
  const formData = new FormData();
  formData.append("file", file);
  const response = await fetch(`${API_BASE}/admin/files/upload`, {
    method: "POST",
    headers: requestToken ? { Authorization: `Bearer ${requestToken}` } : undefined,
    body: formData,
  });
  return parsePlatformResponse<AdminResourceMutation>(response, requestToken);
}

export function adminFileContentUrl(id: string) {
  return `${API_BASE}/admin/files/${encodeURIComponent(id)}/content`;
}

export async function getAdminFileBlob(id: string) {
  const requestToken = getAuthToken();
  const response = await fetch(adminFileContentUrl(id), {
    headers: requestToken ? { Authorization: `Bearer ${requestToken}` } : undefined,
  });
  return parsePlatformResponse<Blob>(response, requestToken, "blob");
}
