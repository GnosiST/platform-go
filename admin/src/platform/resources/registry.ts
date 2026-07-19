import type { Language } from "../i18n";

type LocalizedText = Record<Language, string>;

export type AdminResourceDefinition = {
  name: string;
  route: string;
  parent?: string;
  isExternal?: boolean;
  cacheEnabled?: boolean;
  title: LocalizedText;
  description: LocalizedText;
  permission: string;
  group: "foundation" | "governance" | "operations" | "security";
  icon: string;
};

const loggingCenterResource: AdminResourceDefinition = {
  name: "loggingCenter",
  route: "/logging-center",
  parent: "logs",
  cacheEnabled: true,
  title: { zh: "日志中心", en: "Logging Center" },
  description: { zh: "聚合审计、登录、错误和请求日志入口。", en: "Aggregated entry for audit, login, error, and request logs." },
  permission: "admin:audit-log:read",
  group: "operations",
  icon: "audit",
};

export const coreResources: AdminResourceDefinition[] = [
  {
    name: "overview",
    route: "/overview",
    parent: "runtime",
    cacheEnabled: true,
    title: { zh: "概览", en: "Overview" },
    description: { zh: "平台底座运行概览。", en: "Platform foundation overview." },
    permission: "admin:overview:read",
    group: "foundation",
    icon: "overview",
  },
  {
    name: "tenants",
    route: "/tenants",
    parent: "identity",
    cacheEnabled: true,
    title: { zh: "租户", en: "Tenants" },
    description: { zh: "租户空间与隔离边界。", en: "Tenant spaces and isolation boundaries." },
    permission: "admin:tenant:read",
    group: "foundation",
    icon: "tenants",
  },
  {
    name: "users",
    route: "/users",
    parent: "identity",
    cacheEnabled: true,
    title: { zh: "用户", en: "Users" },
    description: { zh: "用户、账号和身份档案。", en: "Users, accounts, and identity profiles." },
    permission: "admin:user:read",
    group: "foundation",
    icon: "users",
  },
  {
    name: "appIdentities",
    route: "/app-identities",
    parent: "identity",
    cacheEnabled: true,
    title: { zh: "App 身份绑定", en: "App Identities" },
    description: { zh: "App 登录 provider 与平台 App 用户的安全绑定。", en: "Safe bindings between app login providers and platform app users." },
    permission: "admin:app-identity:read",
    group: "foundation",
    icon: "users",
  },
  {
    name: "appPhoneVerifications",
    route: "/app-phone-verifications",
    parent: "identity",
    cacheEnabled: true,
    title: { zh: "App 手机验证", en: "App Phone Verifications" },
    description: { zh: "App 手机验证码请求记录。", en: "App phone verification requests." },
    permission: "admin:app-phone-verification:read",
    group: "foundation",
    icon: "capabilities",
  },
  {
    name: "appPhoneBindings",
    route: "/app-phone-bindings",
    parent: "identity",
    cacheEnabled: true,
    title: { zh: "App 手机绑定", en: "App Phone Bindings" },
    description: { zh: "App 用户与手机号的安全绑定。", en: "Safe bindings between app users and phone numbers." },
    permission: "admin:app-phone-binding:read",
    group: "foundation",
    icon: "users",
  },
  {
    name: "orgUnits",
    route: "/org-units",
    parent: "identity",
    cacheEnabled: true,
    title: { zh: "组织机构", en: "Org Units" },
    description: { zh: "租户下的机构、部门和团队层级。", en: "Organization, department, and team hierarchy under tenants." },
    permission: "admin:org-unit:read",
    group: "foundation",
    icon: "tenants",
  },
  {
    name: "roleGroups",
    route: "/role-groups",
    parent: "access",
    cacheEnabled: true,
    title: { zh: "角色组", en: "Role Groups" },
    description: { zh: "角色分类、治理和授权维护。", en: "Role classification, governance, and authorization maintenance." },
    permission: "admin:role-group:read",
    group: "foundation",
    icon: "roles",
  },
  {
    name: "roles",
    route: "/roles",
    parent: "access",
    cacheEnabled: true,
    title: { zh: "角色", en: "Roles" },
    description: { zh: "角色、权限和授权策略。", en: "Roles, permissions, and authorization policies." },
    permission: "admin:role:read",
    group: "foundation",
    icon: "roles",
  },
  {
    name: "menus",
    route: "/menus",
    parent: "access",
    cacheEnabled: true,
    title: { zh: "菜单", en: "Menus" },
    description: { zh: "后台菜单和资源入口。", en: "Admin menus and resource entries." },
    permission: "admin:menu:read",
    group: "foundation",
    icon: "menus",
  },
  {
    name: "capabilities",
    route: "/capabilities",
    parent: "runtime",
    cacheEnabled: true,
    title: { zh: "能力清单", en: "Capabilities" },
    description: { zh: "查看当前平台启用的能力包。", en: "View enabled platform capability packages." },
    permission: "admin:capability:read",
    group: "foundation",
    icon: "capabilities",
  },
  loggingCenterResource,
  {
    name: "auditLogs",
    route: "/audit-logs",
    parent: "audit",
    cacheEnabled: true,
    title: { zh: "审计日志", en: "Audit Logs" },
    description: { zh: "操作审计和日志留痕。", en: "Operation audit and activity trails." },
    permission: "admin:audit-log:read",
    group: "governance",
    icon: "audit",
  },
  {
    name: "loginLogs",
    route: "/login-logs",
    parent: "logs",
    cacheEnabled: true,
    title: { zh: "登录日志", en: "Login Logs" },
    description: { zh: "登录认证记录和安全追踪。", en: "Login authentication records and security tracing." },
    permission: "admin:login-log:read",
    group: "operations",
    icon: "audit",
  },
  {
    name: "errorLogs",
    route: "/error-logs",
    parent: "logs",
    cacheEnabled: true,
    title: { zh: "错误日志", en: "Error Logs" },
    description: { zh: "运行错误、异常和排查记录。", en: "Runtime errors, exceptions, and troubleshooting records." },
    permission: "admin:error-log:read",
    group: "operations",
    icon: "audit",
  },
  {
    name: "requestLogs",
    route: "/request-logs",
    parent: "logs",
    cacheEnabled: true,
    title: { zh: "请求日志", en: "Request Logs" },
    description: { zh: "HTTP 请求、状态、耗时和调用主体追踪。", en: "HTTP requests, status, latency, and actor tracing." },
    permission: "admin:request-log:read",
    group: "operations",
    icon: "apiResources",
  },
  {
    name: "sessions",
    route: "/sessions",
    parent: "security",
    cacheEnabled: true,
    title: { zh: "在线会话", en: "Sessions" },
    description: { zh: "展示后端返回的在线会话只读记录。", en: "Display read-only online session records returned by the backend." },
    permission: "admin:session:read",
    group: "operations",
    icon: "wifi",
  },
  {
    name: "apiResources",
    route: "/api-resources",
    parent: "resources",
    cacheEnabled: true,
    title: { zh: "API 资源", en: "API Resources" },
    description: { zh: "接口资源、权限码和调用边界。", en: "API resources, permission codes, and invocation boundaries." },
    permission: "admin:api-resource:read",
    group: "governance",
    icon: "apiResources",
  },
  {
    name: "dictParams",
    route: "/dictionary-parameters",
    parent: "resources",
    cacheEnabled: true,
    title: { zh: "字典参数", en: "Dict & Params" },
    description: { zh: "字典、参数和配置项。", en: "Dictionaries, parameters, and configuration items." },
    permission: "admin:dictionary:read",
    group: "governance",
    icon: "dictParams",
  },
  {
    name: "areaCodes",
    route: "/area-codes",
    parent: "configuration",
    cacheEnabled: true,
    title: { zh: "地址码", en: "Area Codes" },
    description: { zh: "租户、机构和人员引用的通用区域编码。", en: "Common area codes referenced by tenants, org units, and users." },
    permission: "admin:area-code:read",
    group: "governance",
    icon: "dictParams",
  },
  {
    name: "monitoring",
    route: "/monitoring",
    parent: "runtime",
    cacheEnabled: true,
    title: { zh: "监控", en: "Monitoring" },
    description: { zh: "实例、健康与告警。", en: "Instances, health, and alerts." },
    permission: "admin:monitoring:read",
    group: "operations",
    icon: "monitoring",
  },
  {
    name: "settings",
    route: "/settings",
    parent: "configuration",
    cacheEnabled: true,
    title: { zh: "系统设置", en: "System Settings" },
    description: { zh: "系统级配置、品牌、参数和能力配置入口。", en: "System-level configuration, branding, parameters, and capability settings." },
    permission: "admin:settings:read",
    group: "governance",
    icon: "settings",
  },
];

export function isLoggingCenterResourceRoute(route: string) {
  return [
    "/audit-logs",
    "/login-logs",
    "/error-logs",
    "/request-logs",
  ].includes(route);
}

export function projectLoggingCenterResource(resources: AdminResourceDefinition[]) {
  if (resources.some((resource) => resource.route === loggingCenterResource.route)) {
    return resources;
  }
  const loggingResources = resources.filter((resource) => isLoggingCenterResourceRoute(resource.route));
  if (loggingResources.length === 0) {
    return resources;
  }
  return [
    ...resources,
    {
      ...loggingCenterResource,
      permission: loggingResources[0].permission,
    },
  ];
}
