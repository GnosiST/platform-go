import type { Language } from "../i18n";

type LocalizedText = Record<Language, string>;

export type AdminResourceDefinition = {
  name: string;
  route: string;
  title: LocalizedText;
  description: LocalizedText;
  permission: string;
  group: "foundation" | "governance" | "operations" | "security";
  icon: string;
};

export const coreResources: AdminResourceDefinition[] = [
  {
    name: "overview",
    route: "/overview",
    title: { zh: "概览", en: "Overview" },
    description: { zh: "平台底座运行概览。", en: "Platform foundation overview." },
    permission: "admin:overview:read",
    group: "foundation",
    icon: "overview",
  },
  {
    name: "tenants",
    route: "/tenants",
    title: { zh: "租户", en: "Tenants" },
    description: { zh: "租户空间与隔离边界。", en: "Tenant spaces and isolation boundaries." },
    permission: "admin:tenant:read",
    group: "foundation",
    icon: "tenants",
  },
  {
    name: "users",
    route: "/users",
    title: { zh: "用户", en: "Users" },
    description: { zh: "用户、账号和身份档案。", en: "Users, accounts, and identity profiles." },
    permission: "admin:user:read",
    group: "foundation",
    icon: "users",
  },
  {
    name: "roles",
    route: "/roles",
    title: { zh: "角色", en: "Roles" },
    description: { zh: "角色、权限和授权策略。", en: "Roles, permissions, and authorization policies." },
    permission: "admin:role:read",
    group: "foundation",
    icon: "roles",
  },
  {
    name: "menus",
    route: "/menus",
    title: { zh: "菜单", en: "Menus" },
    description: { zh: "后台菜单和资源入口。", en: "Admin menus and resource entries." },
    permission: "admin:menu:read",
    group: "foundation",
    icon: "menus",
  },
  {
    name: "capabilities",
    route: "/capabilities",
    title: { zh: "能力清单", en: "Capabilities" },
    description: { zh: "查看当前平台启用的能力包。", en: "View enabled platform capability packages." },
    permission: "admin:capability:read",
    group: "foundation",
    icon: "capabilities",
  },
  {
    name: "audit",
    route: "/audit",
    title: { zh: "审计", en: "Audit" },
    description: { zh: "操作审计和日志留痕。", en: "Operation audit and activity trails." },
    permission: "admin:audit:read",
    group: "governance",
    icon: "audit",
  },
  {
    name: "apiResources",
    route: "/api-resources",
    title: { zh: "API 资源", en: "API Resources" },
    description: { zh: "接口资源、权限码和调用边界。", en: "API resources, permission codes, and invocation boundaries." },
    permission: "admin:api-resource:read",
    group: "governance",
    icon: "apiResources",
  },
  {
    name: "dictParams",
    route: "/dictionary-parameters",
    title: { zh: "字典参数", en: "Dict & Params" },
    description: { zh: "字典、参数和配置项。", en: "Dictionaries, parameters, and configuration items." },
    permission: "admin:dictionary:read",
    group: "governance",
    icon: "dictParams",
  },
  {
    name: "monitoring",
    route: "/monitoring",
    title: { zh: "监控", en: "Monitoring" },
    description: { zh: "实例、健康与告警。", en: "Instances, health, and alerts." },
    permission: "admin:monitoring:read",
    group: "operations",
    icon: "monitoring",
  },
  {
    name: "settings",
    route: "/settings",
    title: { zh: "设置", en: "Settings" },
    description: { zh: "平台配置和品牌设置。", en: "Platform configuration and branding." },
    permission: "admin:settings:read",
    group: "security",
    icon: "settings",
  },
];
