import { shouldExpireAdminSession } from "./sessionExpiry";
import {
  isPlatformErrorCode,
  type PlatformErrorBody,
  type PlatformErrorCode,
} from "../../../../resources/generated/error-sdk/typescript/errorContract";

const API_BASE = import.meta.env.VITE_PLATFORM_API_BASE ?? "/api";
const AUTH_TOKEN_KEY = "platform.auth.token";

export const ADMIN_SESSION_EXPIRED_EVENT = "platform:auth:session-expired";

export class AdminAPIError extends Error {
  constructor(
    message: string,
    readonly statusCode: number,
    readonly code: PlatformErrorCode,
    readonly requestId: string,
    readonly traceId: string,
  ) {
    super(message);
    this.name = "AdminAPIError";
  }
}

export type PlatformResponse<T> = {
  data?: T;
  error?: PlatformErrorBody;
};

export type CapabilityItem = {
  id: string;
  name: string;
  version: string;
  dependencies?: string[];
  adminResources?: CapabilityResourceContribution[];
  menuRoutes?: CapabilityMenuContribution[];
  permissions?: string[];
  configResources?: CapabilityResourceContribution[];
  serviceOperations?: string[];
  authProviders?: string[];
};

export type CapabilityResourceContribution = {
  resource: string;
  title: LocalizedText;
  route?: string;
  permissionPrefix?: string;
  readOnly?: boolean;
};

export type CapabilityMenuContribution = {
  route: string;
  title: LocalizedText;
  permission?: string;
};

export type PluginManagementLockStatus = {
  configured: boolean;
  path?: string;
  exists: boolean;
  valid: boolean;
  error?: string;
};

export type PluginManagementStatus = {
  operationMode: string;
  activation: string;
  progressTransport: string;
  runtimeHotInstall: boolean;
  runtimeHotUninstall: boolean;
  remoteRepositoryPull: boolean;
  restartRequiredForChanges: boolean;
  pendingRestart: boolean;
  lockStatus: PluginManagementLockStatus;
  source: string;
  currentCapabilities: string[];
  desiredCapabilities: string[];
};

export type AdminSettingsMetrics = {
  capabilities: number;
  resources: number;
  records: number;
};

export type AdminSettingsResourceItem = {
  capabilityId: string;
  capabilityName: string;
  capabilityVersion?: string;
  resource: string;
  title: LocalizedText;
  description: LocalizedText;
  route?: string;
  group?: string;
  permissionPrefix: string;
  readOnly?: boolean;
  writable: boolean;
  runtimeApplyMode: "dynamic" | "restart-required" | string;
  restartRequired: boolean;
  pendingRestart: boolean;
  validationEndpoint?: string;
  testConnectionEndpoint?: string;
  schema: AdminResourceSchema;
  recordCount: number;
  records: AdminResourceRecord[];
};

export type AdminSettingsRuntime = {
  items: AdminSettingsResourceItem[];
  metrics: AdminSettingsMetrics;
};

export type AdminSettingsUpdateInput = {
  code?: string;
  name?: string;
  status?: string;
  description?: string;
  values?: Record<string, unknown>;
};

export type AdminSettingsMutationResult = AdminResourceMutation & {
  restartRequired: boolean;
  pendingRestart: boolean;
};

export type AdminSettingsConfigCheck = {
  key: string;
  status: "ok" | "warning" | "invalid" | string;
  message: string;
};

export type AdminSettingsValidationResult = {
  resource: string;
  id: string;
  status: "valid" | "invalid" | string;
  valid: boolean;
  restartRequired: boolean;
  pendingRestart: boolean;
  checks: AdminSettingsConfigCheck[];
};

export type AdminSettingsTestConnectionResult = {
  resource: string;
  id: string;
  status: "dry-run" | "invalid" | "unsupported" | string;
  supported: boolean;
  connected: boolean;
  mode: string;
  restartRequired: boolean;
  pendingRestart: boolean;
  checks: AdminSettingsConfigCheck[];
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
  identifier?: {
    type: "username" | "phone" | "email";
    value: string;
  };
  secret?: AuthLoginSecretInput;
  challenge?: {
    id: string;
    kind: string;
    proof: string;
    clientFingerprint?: string;
  };
};

export type AuthLoginSecretInput =
  | {
    type: "password";
    value: string;
  }
  | {
    type: "sms-otp";
    transactionId: string;
    code: string;
  };

type AuthLoginRequest = Omit<AuthLoginInput, "secret"> & {
  secret?: AuthLoginRequestSecret;
};

type AuthLoginRequestSecret =
  | {
    type: "password";
    encrypted: CredentialSecretEnvelope;
  }
  | {
    type: "sms-otp";
    transactionId: string;
    encrypted: CredentialSecretEnvelope;
  };

export type CredentialSecretKey = {
  version: "pgo-auth-secret-v1";
  algorithm: "ECDH-P256-HKDF-SHA256+A256GCM";
  keyId: string;
  publicKey: string;
  expiresAt: string;
};

export type CredentialSecretEnvelope = {
  version: "pgo-auth-secret-v1";
  algorithm: "ECDH-P256-HKDF-SHA256+A256GCM";
  keyId: string;
  clientPublicKey: string;
  salt: string;
  nonce: string;
  ciphertext: string;
};

export type AuthProviderStartResult = {
  authorizationUrl: string;
  state: string;
  expiresAt: string;
};

export type CredentialSMSOTPStartInput = {
  provider: string;
  identifier: {
    type: "phone";
    value: string;
  };
};

export type CredentialSMSOTPStartResult = {
  transactionId: string;
  maskedIdentifier: string;
  expiresAt: string;
  debugCode?: string;
};

export type CredentialChallengeStartInput = {
  kind?: "captcha" | "slider" | string;
  purpose?: "login" | string;
  clientFingerprint?: string;
};

export type CredentialChallengeStartResult = {
  id: string;
  kind: string;
  purpose: string;
  prompt: string;
  parameters?: Record<string, unknown>;
  expiresAt: string;
  debugVisible?: boolean;
  debugProof?: string;
};

export type CredentialEncryptedPasswordSecret = {
  type: "password" | "current-password" | "new-password" | string;
  encrypted: CredentialSecretEnvelope;
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

export type AdminProfileCredentialStatus = {
  passwordChange: "credential-auth-not-connected" | "credential-auth-ready" | string;
  passwordReset: "credential-auth-not-connected" | "credential-auth-ready" | string;
  message: string;
};

export type AdminProfile = {
  id: string;
  username: string;
  name: string;
  nickname: string;
  avatarUrl: string;
  phone: string;
  maskedPhone?: string;
  email: string;
  maskedEmail?: string;
  address: string;
  tenantCode?: string;
  orgUnitCode?: string;
  areaCode?: string;
  credentials: AdminProfileCredentialStatus;
};

export type AdminProfileResult = {
  profile: AdminProfile;
};

export type AdminProfileUpdateInput = {
  avatarUrl?: string;
  name?: string;
  nickname?: string;
  phone?: string;
  email?: string;
  address?: string;
};

export type AdminCurrentPasswordChangeInput = {
  currentPassword: string;
  newPassword: string;
};

export type AdminProfilePasswordResetInput = {
  newPassword: string;
};

export type AdminPasswordMutationResult = {
  profile?: AdminProfile;
  changed?: boolean;
  reset?: boolean;
  credentials?: AdminProfileCredentialStatus;
  mustChange?: boolean;
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

export type AdminResourceFieldProtection = {
  format: "aes-256-gcm-v1";
  normalization: "raw-v1" | "trim-v1" | "email-v1" | "phone-e164-cn-v1" | "identity-cn-v1";
  blindIndexNamespace?: string;
};

export type AdminResourceFieldMasking = {
  strategy: "partial-v1" | "phone-v1" | "email-v1" | "identity-cn-v1" | "address-cn-v1";
  preservePrefix?: number;
  preserveSuffix?: number;
  maskLength?: number;
  replacement?: string;
};

export type AdminResourceFieldReveal = {
  policyId: string;
  permission: string;
  copyAllowed?: boolean;
};

export type AdminResourceProtection = {
  schemaVersion: number;
  scope: "global" | "tenant-field";
  tenantField?: string;
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
  protection?: AdminResourceFieldProtection;
  masking?: AdminResourceFieldMasking;
  reveal?: AdminResourceFieldReveal;
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
  protection?: AdminResourceProtection;
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
  watermark: {
    applied: boolean;
    product: string;
    exportedBy: string;
    exportedAt: string;
  };
  reviews: AdminResourceRecord[];
  audits: AdminResourceRecord[];
};

export type AdminSensitiveRevealPurpose = {
  code: string;
  label: LocalizedText;
};

export type AdminSensitiveRevealProvider = {
  id: string;
  title: LocalizedText;
};

export type AdminSensitiveRevealFactor = {
  type: "oidc-reauth-v1" | "admin-sms-otp-v1";
  available: boolean;
  providers?: AdminSensitiveRevealProvider[];
  maskedDestination?: string;
};

export type AdminSensitiveRevealPolicy = {
  policyId: string;
  mode: "anyOf" | "allOf";
  purposes: AdminSensitiveRevealPurpose[];
  factors: AdminSensitiveRevealFactor[];
  challengeTtlSeconds: number;
  grantTtlSeconds: number;
  copyAllowed: boolean;
};

export type AdminSensitiveRevealChallenge = {
  challengeId: string;
  challengeToken: string;
  policyId: string;
  mode: "anyOf" | "allOf";
  factors: AdminSensitiveRevealFactor["type"][];
  expiresAt: string;
};

export type AdminSensitiveRevealOIDCStart = {
  challengeId: string;
  transactionToken: string;
  authorizationUrl: string;
  state: string;
  expiresAt: string;
};

export type AdminSensitiveRevealSMSStart = {
  challengeId: string;
  transactionToken: string;
  maskedPhone: string;
  expiresAt: string;
  debugCode?: string;
};

export type AdminSensitiveRevealFactorComplete = {
  challengeId: string;
  policySatisfied: boolean;
  grantToken?: string;
  grantExpiresAt?: string;
};

export type AdminSensitiveRevealValue = {
  field: string;
  value: string;
  copyAllowed: boolean;
};

export type MessageCenterChannel = "in_app" | "sms" | "email" | "wechat_official" | "wechat_miniapp";

export type MessageCenterTestSendInput = {
  channel: MessageCenterChannel;
  tenantCode?: string;
  recipient: string;
  templateId: string;
  templateParams?: Record<string, string>;
  title?: string;
  body?: string;
};

export type MessageCenterTestSendResult = {
  notification: AdminResourceRecord;
  delivery: AdminResourceRecord;
  receipt: {
    channel: string;
    provider: string;
    messageId: string;
    status: string;
    redactedTarget: string;
  };
};

export type MessageCenterDeliveriesRunInput = {
  limit?: number;
};

export type MessageCenterDeliveriesRunResult = {
  scanned: number;
  attempted: number;
  delivered: number;
  failed: number;
  skipped: number;
};

type PlatformResponseMode = "data" | "raw" | "blob";
type PlatformRequestInit = RequestInit & {
  auth?: "stored-token" | "none";
};

function handleUnauthorizedResponse(statusCode: number, requestToken: string, errorCode: PlatformErrorCode) {
  if (!shouldExpireAdminSession({ statusCode, requestToken, currentToken: getAuthToken(), errorCode })) {
    return;
  }
  clearAuthToken();
  window.dispatchEvent(new Event(ADMIN_SESSION_EXPIRED_EVENT));
}

const requestIdPattern = /^req_[0-9a-f]{32}$/;
const traceIdPattern = /^[0-9a-f]{32}$/;
const traceparentPattern = /^00-([0-9a-f]{32})-([0-9a-f]{16})-[0-9a-f]{2}$/;

function correlationValue(value: unknown, pattern: RegExp) {
  return typeof value === "string" && pattern.test(value) ? value : "";
}

function traceIdFromTraceparent(value: string) {
  const match = value.match(traceparentPattern);
  if (!match || /^0+$/.test(match[1]) || /^0+$/.test(match[2])) return "";
  return match[1];
}

function normalizedErrorBody(payload: unknown, response: Response): PlatformErrorBody {
  const candidate = payload && typeof payload === "object" && "error" in payload
    ? (payload as { error?: unknown }).error
    : undefined;
  const error = candidate && typeof candidate === "object"
    ? candidate as Record<string, unknown>
    : {};
  const traceparent = response.headers.get("traceparent") ?? "";
  return {
    code: isPlatformErrorCode(error.code) ? error.code : "INTERNAL_ERROR",
    message: typeof error.message === "string" ? error.message : `HTTP ${response.status}`,
    requestId: correlationValue(error.requestId, requestIdPattern)
      || correlationValue(response.headers.get("X-Request-ID"), requestIdPattern),
    traceId: correlationValue(error.traceId, traceIdPattern)
      || traceIdFromTraceparent(traceparent),
  };
}

export async function parsePlatformResponse<T>(response: Response, requestToken: string, mode: PlatformResponseMode = "data"): Promise<T> {
  if (mode === "blob" && response.ok) {
    return (await response.blob()) as T;
  }

  let payload: unknown;
  try {
    payload = await response.json();
  } catch {
    payload = undefined;
  }

  const hasError = Boolean(payload && typeof payload === "object" && "error" in payload && (payload as PlatformResponse<unknown>).error);
  if (!response.ok || hasError || payload === undefined) {
    const error = normalizedErrorBody(payload, response);
    handleUnauthorizedResponse(response.status, requestToken, error.code);
    throw new AdminAPIError(error.message, response.status, error.code, error.requestId, error.traceId);
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

export function getPluginManagementStatus() {
  return request<PluginManagementStatus>("/admin/plugin-management/status");
}

export async function getFrontendVersion() {
  const response = await fetch(`/version.json?t=${Date.now()}`, { cache: "no-store" });
  if (!response.ok) {
    return null;
  }
  return response.json() as Promise<unknown>;
}

export function getBrandingConfig() {
  return request<BrandingConfig>("/platform/branding");
}

export function listAuthProviders() {
  return request<AuthProviderList>("/auth/providers", { auth: "none" });
}

export function getCredentialSecretKey() {
  return request<CredentialSecretKey>("/auth/credential-secret-key", { auth: "none" });
}

export function startAdminAuthProvider(provider: string, codeChallenge: string) {
  return request<AuthProviderStartResult>(`/auth/providers/${encodeURIComponent(provider)}/start`, {
    auth: "none",
    method: "POST",
    body: JSON.stringify({ codeChallenge }),
  });
}

export function startCredentialSMSOTP(input: CredentialSMSOTPStartInput) {
  return request<CredentialSMSOTPStartResult>("/auth/sms-otp/start", {
    auth: "none",
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function startCredentialChallenge(input: CredentialChallengeStartInput = {}) {
  return request<CredentialChallengeStartResult>("/auth/challenges", {
    auth: "none",
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function loginWithAuthProvider(input: AuthLoginInput) {
  const body = await withEncryptedCredentialSecret(input);
  const result = await request<AuthLoginResult>("/auth/login", {
    auth: "none",
    method: "POST",
    body: JSON.stringify(body),
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

export function getCurrentAdminProfile() {
  return request<AdminProfileResult>("/admin/profile/current");
}

export function updateCurrentAdminProfile(input: AdminProfileUpdateInput) {
  return request<AdminProfileResult>("/admin/profile/current", {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function changeCurrentAdminPassword(input: AdminCurrentPasswordChangeInput) {
  const currentSecret = await encryptedPasswordSecret("profile-password-change", "current-password", "admin-profile", input.currentPassword);
  const newSecret = await encryptedPasswordSecret("profile-password-change", "new-password", "admin-profile", input.newPassword);
  return request<AdminPasswordMutationResult>("/admin/profile/current/password/change", {
    method: "POST",
    body: JSON.stringify({ currentSecret, newSecret } satisfies AdminCurrentPasswordChangeRequest),
  });
}

export async function resetAdminProfilePassword(id: string, input: AdminProfilePasswordResetInput) {
  const newSecret = await encryptedPasswordSecret("profile-password-reset", "new-password", "admin-profile", input.newPassword);
  return request<AdminPasswordMutationResult>(`/admin/profile/${encodeURIComponent(id)}/password/reset`, {
    method: "POST",
    body: JSON.stringify({ newSecret } satisfies AdminProfilePasswordResetRequest),
  });
}

export function testSendMessageCenter(input: MessageCenterTestSendInput) {
  return request<MessageCenterTestSendResult>("/admin/message-center/test-send", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function runMessageCenterDeliveries(input: MessageCenterDeliveriesRunInput = {}) {
  return request<MessageCenterDeliveriesRunResult>("/admin/message-center/deliveries/run", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function listAdminMenus() {
  return request<AdminMenuList>("/admin/menus");
}

export function getAdminSettingsRuntime() {
  return request<AdminSettingsRuntime>("/admin/settings");
}

export function updateAdminSettingsResource(resource: string, id: string, input: AdminSettingsUpdateInput) {
  return request<AdminSettingsMutationResult>(`/admin/settings/${resource}/${id}` as `/${string}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export function validateAdminSettingsResourceConfig(resource: string, id: string) {
  return request<AdminSettingsValidationResult>(`/admin/settings/${resource}/${id}/validate-config` as `/${string}`, {
    method: "POST",
  });
}

export function testConnectAdminSettingsResource(resource: string, id: string) {
  return request<AdminSettingsTestConnectionResult>(`/admin/settings/${resource}/${id}/test-connect` as `/${string}`, {
    method: "POST",
  });
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

export function getAdminSensitiveRevealPolicy(resource: string, id: string, field: string) {
  return request<AdminSensitiveRevealPolicy>(`${adminSensitiveRevealFieldPath(resource, id, field)}/reveal-policy` as `/${string}`);
}

export function createAdminSensitiveRevealChallenge(resource: string, id: string, field: string, purpose: string) {
  return request<AdminSensitiveRevealChallenge>(`${adminSensitiveRevealFieldPath(resource, id, field)}/reveal/challenges` as `/${string}`, {
    method: "POST",
    body: JSON.stringify({ purpose }),
  });
}

export function startAdminSensitiveRevealOIDC(
  resource: string,
  id: string,
  field: string,
  challengeId: string,
  input: { challengeToken: string; purpose: string; provider: string; codeChallenge: string },
) {
  return request<AdminSensitiveRevealOIDCStart>(`${adminSensitiveRevealChallengePath(resource, id, field, challengeId)}/factors/oidc/start` as `/${string}`, {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function completeAdminSensitiveRevealOIDC(
  resource: string,
  id: string,
  field: string,
  challengeId: string,
  input: { challengeToken: string; purpose: string; transactionToken: string; provider: string; code: string; state: string; codeVerifier: string },
) {
  return request<AdminSensitiveRevealFactorComplete>(`${adminSensitiveRevealChallengePath(resource, id, field, challengeId)}/factors/oidc/complete` as `/${string}`, {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function startAdminSensitiveRevealSMS(
  resource: string,
  id: string,
  field: string,
  challengeId: string,
  input: { challengeToken: string; purpose: string },
) {
  return request<AdminSensitiveRevealSMSStart>(`${adminSensitiveRevealChallengePath(resource, id, field, challengeId)}/factors/sms/start` as `/${string}`, {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function completeAdminSensitiveRevealSMS(
  resource: string,
  id: string,
  field: string,
  challengeId: string,
  input: { challengeToken: string; purpose: string; transactionToken: string; code: string },
) {
  return request<AdminSensitiveRevealFactorComplete>(`${adminSensitiveRevealChallengePath(resource, id, field, challengeId)}/factors/sms/complete` as `/${string}`, {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function revealAdminSensitiveField(resource: string, id: string, field: string, input: { purpose: string; grantToken: string }) {
  return request<AdminSensitiveRevealValue>(`${adminSensitiveRevealFieldPath(resource, id, field)}/reveal` as `/${string}`, {
    method: "POST",
    body: JSON.stringify(input),
  });
}

function adminSensitiveRevealFieldPath(resource: string, id: string, field: string) {
  return `/admin/resources/${encodeURIComponent(resource)}/${encodeURIComponent(id)}/fields/${encodeURIComponent(field)}`;
}

function adminSensitiveRevealChallengePath(resource: string, id: string, field: string, challengeId: string) {
  return `${adminSensitiveRevealFieldPath(resource, id, field)}/reveal/challenges/${encodeURIComponent(challengeId)}`;
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

export function exportAdminPolicyReviews({ watermark }: { watermark: boolean }) {
  return request<AdminPolicyReviewExport>(`/admin/policy-reviews/export?watermark=${watermark}`, {
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

type AdminCurrentPasswordChangeRequest = {
  currentSecret: CredentialEncryptedPasswordSecret;
  newSecret: CredentialEncryptedPasswordSecret;
};

type AdminProfilePasswordResetRequest = {
  newSecret: CredentialEncryptedPasswordSecret;
};

async function withEncryptedCredentialSecret(input: AuthLoginInput): Promise<AuthLoginRequest> {
  if (!input.secret) return { ...input, secret: undefined };
  if (!input.identifier) {
    throw new Error("Credential identifier is required before secret encryption.");
  }
  const secret = input.secret;
  const identifierType = input.identifier.type;
  if (secret.type === "password") {
    const encrypted = await encryptCredentialSecret(input.provider, "password", identifierType, secret.value);
    return {
      ...input,
      secret: { type: "password", encrypted },
    };
  }
  if (secret.type === "sms-otp") {
    const encrypted = await encryptCredentialSecret(input.provider, "sms-otp", identifierType, secret.code);
    return {
      ...input,
      secret: { type: "sms-otp", transactionId: secret.transactionId, encrypted },
    };
  }
  throw new Error("Unsupported credential secret type.");
}

async function encryptedPasswordSecret(
  provider: string,
  secretType: "password" | "current-password" | "new-password",
  identifierType: "username" | "phone" | "email" | "admin-profile",
  value: string,
): Promise<CredentialEncryptedPasswordSecret> {
  return {
    type: secretType,
    encrypted: await encryptCredentialSecret(provider, secretType, identifierType, value),
  };
}

async function encryptCredentialSecret(
  provider: string,
  secretType: "password" | "sms-otp" | "current-password" | "new-password",
  identifierType: "username" | "phone" | "email" | "admin-profile",
  value: string,
): Promise<CredentialSecretEnvelope> {
  if (!globalThis.crypto?.subtle) {
    throw new Error("Credential encryption is unavailable in this browser context.");
  }
  const key = await getCredentialSecretKey();
  const serverPublicKey = await globalThis.crypto.subtle.importKey(
    "raw",
    base64URLToBytes(key.publicKey),
    { name: "ECDH", namedCurve: "P-256" },
    false,
    [],
  );
  const clientKeyPair = await globalThis.crypto.subtle.generateKey(
    { name: "ECDH", namedCurve: "P-256" },
    true,
    ["deriveBits"],
  );
  const sharedSecret = await globalThis.crypto.subtle.deriveBits(
    { name: "ECDH", public: serverPublicKey },
    clientKeyPair.privateKey,
    256,
  );
  const salt = globalThis.crypto.getRandomValues(new Uint8Array(16));
  const nonce = globalThis.crypto.getRandomValues(new Uint8Array(12));
  const aad = `${provider}\u0000${secretType}\u0000${identifierType}`;
  const hkdfKey = await globalThis.crypto.subtle.importKey("raw", sharedSecret, "HKDF", false, ["deriveKey"]);
  const aesKey = await globalThis.crypto.subtle.deriveKey(
    {
      name: "HKDF",
      hash: "SHA-256",
      salt,
      info: utf8Bytes(`platform-go credential-auth secret v1\u0000${key.keyId}\u0000${aad}`),
    },
    hkdfKey,
    { name: "AES-GCM", length: 256 },
    false,
    ["encrypt"],
  );
  const ciphertext = await globalThis.crypto.subtle.encrypt(
    { name: "AES-GCM", iv: nonce, additionalData: utf8Bytes(aad) },
    aesKey,
    utf8Bytes(value),
  );
  const clientPublicKey = await globalThis.crypto.subtle.exportKey("raw", clientKeyPair.publicKey);
  return {
    version: key.version,
    algorithm: key.algorithm,
    keyId: key.keyId,
    clientPublicKey: bytesToBase64URL(new Uint8Array(clientPublicKey)),
    salt: bytesToBase64URL(salt),
    nonce: bytesToBase64URL(nonce),
    ciphertext: bytesToBase64URL(new Uint8Array(ciphertext)),
  };
}

function utf8Bytes(value: string) {
  return new TextEncoder().encode(value);
}

function base64URLToBytes(value: string) {
  const padded = value.replace(/-/g, "+").replace(/_/g, "/").padEnd(Math.ceil(value.length / 4) * 4, "=");
  const binary = atob(padded);
  const bytes = new Uint8Array(binary.length);
  for (let index = 0; index < binary.length; index += 1) {
    bytes[index] = binary.charCodeAt(index);
  }
  return bytes;
}

function bytesToBase64URL(bytes: Uint8Array) {
  let binary = "";
  for (const byte of bytes) {
    binary += String.fromCharCode(byte);
  }
  return btoa(binary).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/g, "");
}
