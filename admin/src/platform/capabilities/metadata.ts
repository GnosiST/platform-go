import type { CapabilityItem, CapabilityMenuContribution, CapabilityResourceContribution } from "../api/client";
import type { Language } from "../i18n";

export type CapabilityKind = "core" | "plugin" | "optional" | "disabled";
export type CapabilityHealth = "healthy" | "warning" | "error";

type Localized = Record<Language, string>;
type CapabilityMetadata = {
  label: Localized;
  description: Localized;
  domain: Localized;
  group: "identity" | "authorization" | "system" | "observability" | "business";
  kind: CapabilityKind;
  health: CapabilityHealth;
  owner: string;
  dependencies: string[];
  apis: Array<{ method: string; path: string; summary: Localized }>;
  adminResources?: CapabilityResourceContribution[];
  menuRoutes?: CapabilityMenuContribution[];
  permissions?: string[];
  configResources?: CapabilityResourceContribution[];
  serviceOperations?: string[];
  authProviders?: string[];
};

export type CapabilityView = CapabilityItem & {
  label: Localized;
  description: Localized;
  domain: Localized;
  group: "identity" | "authorization" | "system" | "observability" | "business";
  kind: CapabilityKind;
  health: CapabilityHealth;
  owner: string;
  dependencies: string[];
  apis: Array<{ method: string; path: string; summary: Localized }>;
};

const capabilityMetadata: Record<string, CapabilityMetadata> = {
  tenant: {
    label: { zh: "租户", en: "Tenant" },
    description: { zh: "多租户隔离、租户生命周期与平台空间管理。", en: "Tenant isolation, lifecycle, and platform space management." },
    domain: { zh: "身份与访问", en: "Identity & Access" },
    group: "identity",
    kind: "core",
    health: "healthy",
    owner: "Platform Team",
    dependencies: [],
    apis: [
      { method: "GET", path: "/api/tenants", summary: { zh: "租户列表", en: "List tenants" } },
      { method: "POST", path: "/api/tenants", summary: { zh: "创建租户", en: "Create tenant" } },
    ],
  },
  identity: {
    label: { zh: "身份与组织", en: "Identity & Organization" },
    description: { zh: "用户、组织机构树、账号身份与统一认证能力。", en: "Users, organization-unit hierarchy, account identities, and unified authentication." },
    domain: { zh: "身份与访问", en: "Identity & Access" },
    group: "identity",
    kind: "core",
    health: "healthy",
    owner: "Platform Team",
    dependencies: ["tenant"],
    apis: [
      { method: "GET", path: "/api/users", summary: { zh: "用户列表", en: "List users" } },
      { method: "POST", path: "/api/auth/login", summary: { zh: "登录", en: "Login" } },
    ],
  },
  session: {
    label: { zh: "会话", en: "Session" },
    description: { zh: "登录会话、令牌生命周期和单点登录支撑。", en: "Login sessions, token lifecycle, and single sign-on support." },
    domain: { zh: "身份与访问", en: "Identity & Access" },
    group: "identity",
    kind: "core",
    health: "healthy",
    owner: "Platform Team",
    dependencies: ["identity"],
    apis: [
      { method: "POST", path: "/api/auth/refresh", summary: { zh: "续期会话", en: "Refresh session" } },
      { method: "POST", path: "/api/auth/logout", summary: { zh: "退出登录", en: "Logout" } },
      { method: "DELETE", path: "/api/sessions/{id}", summary: { zh: "撤销会话", en: "Revoke session" } },
    ],
  },
  rbac: {
    label: { zh: "RBAC", en: "RBAC" },
    description: { zh: "基于角色的访问控制、权限码与策略刷新。", en: "Role-based access control, permission codes, and policy refresh." },
    domain: { zh: "授权管理", en: "Authorization" },
    group: "authorization",
    kind: "core",
    health: "healthy",
    owner: "Platform Team",
    dependencies: ["tenant", "identity"],
    apis: [
      { method: "GET", path: "/api/rbac/roles", summary: { zh: "角色列表", en: "List roles" } },
      { method: "PUT", path: "/api/rbac/policies", summary: { zh: "更新策略", en: "Update policies" } },
    ],
  },
  menu: {
    label: { zh: "菜单", en: "Menu" },
    description: { zh: "后台菜单、路由入口与导航资源注册。", en: "Admin menus, route entries, and navigation resource registration." },
    domain: { zh: "系统管理", en: "System Management" },
    group: "system",
    kind: "core",
    health: "healthy",
    owner: "Platform Team",
    dependencies: ["rbac"],
    apis: [{ method: "GET", path: "/api/menus", summary: { zh: "菜单树", en: "Menu tree" } }],
  },
  "api-resource": {
    label: { zh: "API 资源", en: "API Resource" },
    description: { zh: "接口资源注册、权限映射与调用边界。", en: "API registration, permission mapping, and invocation boundaries." },
    domain: { zh: "系统管理", en: "System Management" },
    group: "system",
    kind: "core",
    health: "healthy",
    owner: "Platform Team",
    dependencies: ["rbac"],
    apis: [{ method: "GET", path: "/api/resources", summary: { zh: "接口资源", en: "API resources" } }],
  },
  audit: {
    label: { zh: "审计", en: "Audit" },
    description: { zh: "操作审计、日志留痕和安全追踪。", en: "Operation audit, activity trails, and security tracing." },
    domain: { zh: "可观测性", en: "Observability" },
    group: "observability",
    kind: "core",
    health: "warning",
    owner: "Platform Team",
    dependencies: ["tenant", "identity"],
    apis: [{ method: "GET", path: "/api/admin/resources/audit-logs", summary: { zh: "审计日志", en: "Audit logs" } }],
  },
  dictionary: {
    label: { zh: "字典", en: "Dictionary" },
    description: { zh: "基础数据字典、枚举项和展示文案。", en: "Base dictionaries, enums, and display labels." },
    domain: { zh: "系统管理", en: "System Management" },
    group: "system",
    kind: "core",
    health: "healthy",
    owner: "Platform Team",
    dependencies: ["tenant"],
    apis: [{ method: "GET", path: "/api/dictionaries", summary: { zh: "字典列表", en: "List dictionaries" } }],
  },
  parameter: {
    label: { zh: "参数", en: "Parameter" },
    description: { zh: "系统参数、敏感配置和运行开关。", en: "System parameters, sensitive config, and runtime switches." },
    domain: { zh: "系统管理", en: "System Management" },
    group: "system",
    kind: "core",
    health: "healthy",
    owner: "Platform Team",
    dependencies: ["tenant", "audit"],
    apis: [{ method: "GET", path: "/api/parameters", summary: { zh: "参数列表", en: "List parameters" } }],
  },
  "admin-shell": {
    label: { zh: "管理后台 Shell", en: "Admin Shell" },
    description: { zh: "后台框架、主题、国际化、导航和资源承载。", en: "Admin frame, themes, i18n, navigation, and resource hosting." },
    domain: { zh: "系统管理", en: "System Management" },
    group: "system",
    kind: "core",
    health: "healthy",
    owner: "Platform Team",
    dependencies: ["identity", "session", "rbac", "menu"],
    apis: [{ method: "GET", path: "/api/capabilities", summary: { zh: "能力清单", en: "Capability list" } }],
  },
  "system-admin": {
    label: { zh: "系统管理", en: "System Admin" },
    description: { zh: "租户、用户、角色、菜单、字典、参数与审计资源。", en: "Tenant, user, role, menu, dictionary, parameter, and audit resources." },
    domain: { zh: "系统管理", en: "System Management" },
    group: "system",
    kind: "core",
    health: "healthy",
    owner: "Platform Team",
    dependencies: ["admin-shell", "api-resource", "dictionary", "parameter", "audit"],
    apis: [{ method: "GET", path: "/api/system/resources", summary: { zh: "系统资源", en: "System resources" } }],
  },
};

export const optionalCapabilities: CapabilityView[] = [
  makeOptional("wechat-login", { zh: "微信登录", en: "WeChat Login" }, { zh: "小程序登录、OpenID 绑定与访客会话。", en: "Mini-program login, OpenID binding, and guest sessions." }),
  makeOptional("file-storage", { zh: "文件存储", en: "File Storage" }, { zh: "本地与 S3 兼容上传、预览、下载和删除。", en: "Local and S3-compatible upload, preview, download, and delete." }),
  makeOptional("branding", { zh: "品牌配置", en: "Branding" }, { zh: "产品名称、Logo、主题和登录页文案。", en: "Product name, logo, theme, and login copy." }, {
    configResources: [
      resourceContribution("branding", "/branding", { zh: "品牌配置", en: "Branding" }, "admin:branding"),
    ],
    permissions: ["admin:branding:read", "admin:branding:create", "admin:branding:update", "admin:branding:delete"],
  }),
  makeOptional("demo-data", { zh: "演示数据", en: "Demo Data" }, { zh: "演示数据包、重置行为和 fixture 槽位。", en: "Demo packs, reset behavior, and fixture slots." }),
  makeOptional("policy-review", { zh: "策略评审", en: "Policy Review" }, { zh: "角色、权限和数据范围变更的可选审批台账。", en: "Optional approval ledger for role, permission, and data-scope changes." }, {
    menuRoutes: [menuContribution("/policy-reviews", { zh: "策略评审", en: "Policy Reviews" }, "admin:policy-review:read")],
    adminResources: [resourceContribution("policy-reviews", "/policy-reviews", { zh: "策略评审", en: "Policy Reviews" }, "admin:policy-review")],
    permissions: ["admin:policy-review:read", "admin:policy-review:approve"],
  }),
  makeOptional("personnel", { zh: "人员与岗位", en: "Personnel & Positions" }, { zh: "扩展人员档案、岗位和任职关系；默认平台底座已提供组织机构。", en: "Adds personnel profiles, positions, and assignments; organization units are already part of the default platform foundation." }, {
    adminResources: [
      resourceContribution("personnel", "/personnel", { zh: "人员档案", en: "Personnel" }, "admin:personnel"),
      resourceContribution("positions", "/positions", { zh: "岗位", en: "Positions" }, "admin:position"),
      resourceContribution("position-assignments", "/position-assignments", { zh: "任职关系", en: "Position Assignments" }, "admin:position-assignment"),
    ],
    permissions: ["admin:personnel:read", "admin:position:read", "admin:position-assignment:read"],
  }),
  makeOptional("notification", { zh: "通知中心", en: "Notification" }, { zh: "站内通知、模板和投递记录，可供平台能力和业务能力复用。", en: "In-app notifications, templates, and delivery records reusable by platform and business capabilities." }, {
    menuRoutes: [menuContribution("/message-center", { zh: "消息中心", en: "Message Center" }, "admin:message-center:read")],
    configResources: [
      resourceContribution("notification-channels", "/notification-channels", { zh: "消息渠道", en: "Notification Channels" }, "admin:notification-channel"),
      resourceContribution("notification-providers", "/notification-providers", { zh: "消息供应商", en: "Notification Providers" }, "admin:notification-provider"),
      resourceContribution("notification-send-policies", "/notification-send-policies", { zh: "发送策略", en: "Send Policies" }, "admin:notification-send-policy"),
    ],
    adminResources: [
      resourceContribution("message-center", "/message-center", { zh: "消息中心", en: "Message Center" }, "admin:message-center", true),
      resourceContribution("notification-templates", "/notification-templates", { zh: "通知模板", en: "Notification Templates" }, "admin:notification-template"),
      resourceContribution("notifications", "/notifications", { zh: "通知", en: "Notifications" }, "admin:notification"),
      resourceContribution("notification-deliveries", "/notification-deliveries", { zh: "通知投递", en: "Notification Deliveries" }, "admin:notification-delivery", true),
    ],
    permissions: [
      "admin:message-center:read",
      "admin:notification-channel:read",
      "admin:notification-provider:read",
      "admin:notification-send-policy:read",
      "admin:notification-template:read",
      "admin:notification:read",
      "admin:notification-delivery:read",
    ],
    serviceOperations: ["notification.send", "notification.provider.validate"],
  }),
  makeOptional("job", { zh: "任务调度", en: "Job Scheduling" }, { zh: "任务定义、运行记录和尝试台账，可按需接入调度执行器。", en: "Job definitions, run records, and attempt ledgers with pluggable scheduler execution later." }, {
    adminResources: [
      resourceContribution("job-definitions", "/job-definitions", { zh: "任务定义", en: "Job Definitions" }, "admin:job-definition"),
      resourceContribution("job-runs", "/job-runs", { zh: "任务运行", en: "Job Runs" }, "admin:job-run", true),
      resourceContribution("job-run-attempts", "/job-run-attempts", { zh: "运行尝试", en: "Job Run Attempts" }, "admin:job-run-attempt", true),
    ],
    permissions: ["admin:job-definition:read", "admin:job-run:read", "admin:job-run-attempt:read"],
    serviceOperations: ["job.enqueue", "job.retry"],
  }),
  makeOptional("workflow", { zh: "工作流", en: "Workflow" }, { zh: "流程定义、任务调度和审批动作。", en: "Workflow definitions, job scheduling, and approval actions." }),
];

export function enrichCapabilities(items: CapabilityItem[]): CapabilityView[] {
  return items.map((item) => {
    const metadata: CapabilityMetadata = capabilityMetadata[item.id] ?? {
      label: { zh: item.name || item.id, en: item.name || item.id },
      description: { zh: "通过能力规范注册的平台能力。", en: "Platform capability registered through capability conventions." },
      domain: { zh: "业务扩展", en: "Business Extension" },
      group: "business" as const,
      kind: "plugin" as const,
      health: "healthy" as const,
      owner: "Platform Team",
      dependencies: [],
      apis: [],
    };
    return {
      ...item,
      ...metadata,
      dependencies: item.dependencies ?? metadata.dependencies,
      adminResources: item.adminResources ?? metadata.adminResources ?? [],
      menuRoutes: item.menuRoutes ?? metadata.menuRoutes ?? [],
      permissions: item.permissions ?? metadata.permissions ?? [],
      configResources: item.configResources ?? metadata.configResources ?? [],
      serviceOperations: item.serviceOperations ?? metadata.serviceOperations ?? [],
      authProviders: item.authProviders ?? metadata.authProviders ?? [],
    };
  });
}

function makeOptional(
  id: string,
  label: Localized,
  description: Localized,
  contributions: Partial<Pick<CapabilityView, "adminResources" | "menuRoutes" | "permissions" | "configResources" | "serviceOperations" | "authProviders">> = {},
): CapabilityView {
  return {
    id,
    name: label.en,
    version: "0.1.0",
    label,
    description,
    domain: { zh: "插件能力", en: "Plugin Capability" },
    group: "business",
    kind: "optional",
    health: "healthy",
    owner: "Platform Team",
    dependencies: ["identity", "audit"],
    apis: [],
    adminResources: contributions.adminResources ?? [],
    menuRoutes: contributions.menuRoutes ?? [],
    permissions: contributions.permissions ?? [],
    configResources: contributions.configResources ?? [],
    serviceOperations: contributions.serviceOperations ?? [],
    authProviders: contributions.authProviders ?? [],
  };
}

function resourceContribution(
  resource: string,
  route: string,
  title: Localized,
  permissionPrefix: string,
  readOnly = false,
): CapabilityResourceContribution {
  return { resource, route, title, permissionPrefix, readOnly };
}

function menuContribution(route: string, title: Localized, permission: string): CapabilityMenuContribution {
  return { route, title, permission };
}
