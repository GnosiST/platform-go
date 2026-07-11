import { Refine, type ResourceProps } from "@refinedev/core";
import { useNotificationProvider } from "@refinedev/antd";
import routerProvider from "@refinedev/react-router";
import { App as AntdApp, Empty, Typography } from "antd";
import { useCallback, useEffect, useMemo, useState } from "react";
import { BrowserRouter, Navigate, Route, Routes, useLocation, useNavigate } from "react-router-dom";
import {
  ADMIN_SESSION_EXPIRED_EVENT,
  AdminAPIError,
  clearAuthToken,
  getBrandingConfig,
  getAuthToken,
  getCurrentAdminSession,
  listAdminMenus,
  listAuthProviders,
  listCapabilities,
  logoutCurrentSession,
  type BrandingConfig,
  type AdminMenuItem,
  type AdminCurrentSession,
  type AuthProvider,
  type CapabilityItem,
} from "./platform/api/client";
import { CapabilityConsole } from "./platform/capabilities/CapabilityConsole";
import { enrichCapabilities, optionalCapabilities, type CapabilityView } from "./platform/capabilities/metadata";
import { dictionaries, type Dictionary, type Language } from "./platform/i18n";
import { APIDocsPage } from "./platform/api-docs/APIDocsPage";
import { DemoDataConsole } from "./platform/demo-data/DemoDataConsole";
import { DashboardHome } from "./platform/dashboard/DashboardHome";
import { AdminLoginView } from "./platform/auth/AdminLoginView";
import { PolicyReviewConsole } from "./platform/policy-review/PolicyReviewConsole";
import { coreResources, type AdminResourceDefinition } from "./platform/resources/registry";
import { ResourceRoutePage } from "./platform/refine/ResourceRoutePage";
import { accessControlProvider, authProvider, dataProvider } from "./platform/refine";
import { AdminShell } from "./platform/shell/AdminShell";
import { themeTokens, type AdminLayoutMode, type ThemeName } from "./platform/theme";
import { AdminDesignProvider, defaultAdminUIConfig, type AdminUIConfig } from "./platform/ui";

const adminPreferenceStorageKeys = {
  language: "platform-go.admin.language",
  theme: "platform-go.admin.theme",
  layout: "platform-go.admin.layout",
  ui: "platform-go.admin.ui",
} as const;

export default function App() {
  return (
    <BrowserRouter>
      <PlatformApp />
    </BrowserRouter>
  );
}

function PlatformApp() {
  const [capabilityItems, setCapabilityItems] = useState<CapabilityItem[]>([]);
  const [resources, setResources] = useState<AdminResourceDefinition[]>(coreResources);
  const [permissions, setPermissions] = useState<string[]>(["*"]);
  const [deniedPermissions, setDeniedPermissions] = useState<string[]>([]);
  const [session, setSession] = useState<AdminCurrentSession | null>(null);
  const [authProviders, setAuthProviders] = useState<AuthProvider[]>([]);
  const [branding, setBranding] = useState<BrandingConfig | null>(null);
  const [loading, setLoading] = useState(true);
  const [authLoading, setAuthLoading] = useState(true);
  const [error, setError] = useState("");
  const [authError, setAuthError] = useState("");
  const [language, setLanguageState] = useState<Language>(readStoredLanguage);
  const [themeName, setThemeNameState] = useState<ThemeName>(readStoredThemeName);
  const [layoutMode, setLayoutModeState] = useState<AdminLayoutMode>(readStoredLayoutMode);
  const [uiConfig, setUIConfigState] = useState<AdminUIConfig>(readStoredUIConfig);
  const location = useLocation();
  const navigate = useNavigate();
  const activeRoute = routeFromPathname(location.pathname);

  const dictionary = dictionaries[language];
  const capabilities = useMemo(() => enrichCapabilities(capabilityItems), [capabilityItems]);
  const refineResources = useMemo(() => resources.map(resourceDefinitionToRefineResource), [resources]);
  const hasStoredTheme = useMemo(() => readStorageValue(adminPreferenceStorageKeys.theme) !== null, []);
  const changeLanguage = useCallback((nextLanguage: Language) => {
    setLanguageState(nextLanguage);
    writeStorageValue(adminPreferenceStorageKeys.language, nextLanguage);
  }, []);
  const changeLayoutMode = useCallback((nextLayoutMode: AdminLayoutMode) => {
    setLayoutModeState(nextLayoutMode);
    writeStorageValue(adminPreferenceStorageKeys.layout, nextLayoutMode);
  }, []);
  const changeUIConfig = useCallback((nextConfig: AdminUIConfig) => {
    const normalizedConfig = normalizeUIConfig(nextConfig);
    setUIConfigState(normalizedConfig);
    writeStorageValue(adminPreferenceStorageKeys.ui, JSON.stringify(normalizedConfig));
  }, []);
  const applyThemeName = useCallback((nextThemeName: ThemeName) => {
    const normalizedThemeName = normalizeThemeName(nextThemeName);
    setThemeNameState(normalizedThemeName);
    writeStorageValue(adminPreferenceStorageKeys.theme, normalizedThemeName);
    setUIConfigState((current) => {
      const nextConfig = { ...current, customPrimary: themeTokens[normalizedThemeName].primary };
      writeStorageValue(adminPreferenceStorageKeys.ui, JSON.stringify(nextConfig));
      return nextConfig;
    });
  }, []);
  const navigateToRoute = useCallback((route: string, mode: "push" | "replace" = "push") => {
    const nextRoute = normalizeRoute(route);
    if (location.pathname === nextRoute) {
      return;
    }
    navigate(
      {
        pathname: nextRoute,
        search: location.search,
        hash: location.hash,
      },
      { replace: mode === "replace" },
    );
  }, [location.hash, location.pathname, location.search, navigate]);

  useEffect(() => {
    const handleSessionExpired = () => {
      setSession(null);
      setPermissions([]);
      setDeniedPermissions([]);
      setCapabilityItems([]);
      setResources(coreResources);
      setAuthError(dictionary.sessionExpired);
      setError("");
      setLoading(false);
    };
    window.addEventListener(ADMIN_SESSION_EXPIRED_EVENT, handleSessionExpired);
    return () => window.removeEventListener(ADMIN_SESSION_EXPIRED_EVENT, handleSessionExpired);
  }, [dictionary.sessionExpired]);

  useEffect(() => {
    Promise.all([getBrandingConfig(), listAuthProviders()])
      .then(([nextBranding, providers]) => {
        setBranding(nextBranding);
        if (!hasStoredTheme) {
          applyThemeName(normalizeThemeName(nextBranding.defaultTheme));
        }
        setAuthProviders(providers.items);
        setAuthError((current) => current === dictionary.sessionExpired ? current : "");
      })
      .catch((nextError: unknown) => {
        setAuthError((current) => current === dictionary.sessionExpired
          ? current
          : nextError instanceof Error ? nextError.message : dictionary.authProvidersLoadFailed);
      })
      .finally(() => setAuthLoading(false));
  }, [applyThemeName, dictionary.authProvidersLoadFailed, dictionary.sessionExpired, hasStoredTheme]);

  const loadAdminWorkspace = () => {
    setLoading(true);
    return Promise.all([listCapabilities(), listAdminMenus(), getCurrentAdminSession(), getBrandingConfig()])
      .then(([items, menus, session, nextBranding]) => {
        setCapabilityItems(items);
        setResources(menus.items.map(menuItemToResourceDefinition));
        setPermissions(session.permissions);
        setDeniedPermissions(session.deniedPermissions ?? []);
        setSession(session);
        setBranding(nextBranding);
        if (!hasStoredTheme) {
          applyThemeName(normalizeThemeName(nextBranding.defaultTheme));
        }
        setError("");
      })
      .catch((nextError: unknown) => {
        if (nextError instanceof AdminAPIError && nextError.statusCode === 401) {
          setAuthError(dictionary.sessionExpired);
          setError("");
          return;
        }
        clearAuthToken();
        setSession(null);
        setDeniedPermissions([]);
        setError(nextError instanceof Error ? nextError.message : dictionary.capabilityListLoadFailed);
      })
      .finally(() => setLoading(false));
  };

  const logout = () => {
    void logoutCurrentSession().finally(() => {
      setSession(null);
      setPermissions([]);
      setDeniedPermissions([]);
      setResources(coreResources);
      navigateToRoute("/overview", "replace");
    });
  };

  useEffect(() => {
    if (!getAuthToken()) {
      setLoading(false);
      return;
    }
    void loadAdminWorkspace();
  }, []);

  useEffect(() => {
    if (loading) {
      return;
    }
    const locationRoute = routeFromPathname(location.pathname);
    if (resources.some((resource) => resource.route === locationRoute)) {
      return;
    }
    if (resources.some((resource) => resource.route === activeRoute)) {
      return;
    }
    navigateToRoute(resources[0]?.route ?? "/capabilities", "replace");
  }, [activeRoute, loading, location.pathname, navigateToRoute, resources]);

  if (!getAuthToken() || !session) {
    return (
      <PlatformRefineRuntime resources={refineResources} language={language} themeName={themeName} customPrimary={uiConfig.customPrimary}>
          <AdminLoginView
            language={language}
            dictionary={dictionary}
            branding={branding}
            providers={authProviders}
            loading={authLoading}
            error={authError || error}
            themeName={themeName}
            onLanguageChange={changeLanguage}
            onThemeChange={applyThemeName}
            onLoginSuccess={(nextSession) => {
              setSession(nextSession);
              setPermissions(nextSession.permissions);
              setDeniedPermissions(nextSession.deniedPermissions ?? []);
              void loadAdminWorkspace();
            }}
          />
      </PlatformRefineRuntime>
    );
  }

  return (
    <PlatformRefineRuntime resources={refineResources} language={language} themeName={themeName} customPrimary={uiConfig.customPrimary}>
      <AdminShell
        resources={resources}
        language={language}
        dictionary={dictionary}
        themeName={themeName}
        layoutMode={layoutMode}
        branding={branding}
        session={session}
        activeRoute={activeRoute}
        onLanguageChange={changeLanguage}
        uiConfig={uiConfig}
        onThemeChange={applyThemeName}
        onLayoutModeChange={changeLayoutMode}
        onUIConfigChange={changeUIConfig}
        onRouteChange={navigateToRoute}
        onLogout={logout}
      >
        <PlatformRoutePages
          activeRoute={activeRoute}
          capabilityItems={capabilityItems}
          capabilities={capabilities}
          dictionary={dictionary}
          error={error}
          language={language}
          loading={loading}
          permissions={permissions}
          deniedPermissions={deniedPermissions}
          resources={resources}
          session={session}
          onRouteChange={navigateToRoute}
        />
      </AdminShell>
    </PlatformRefineRuntime>
  );
}

function PlatformRoutePages({
  activeRoute,
  capabilityItems,
  capabilities,
  dictionary,
  error,
  language,
  loading,
  permissions,
  deniedPermissions,
  resources,
  session,
  onRouteChange,
}: {
  activeRoute: string;
  capabilityItems: CapabilityItem[];
  capabilities: CapabilityView[];
  dictionary: Dictionary;
  error: string;
  language: Language;
  loading: boolean;
  permissions: string[];
  deniedPermissions: string[];
  resources: AdminResourceDefinition[];
  session: AdminCurrentSession;
  onRouteChange: (route: string, mode?: "push" | "replace") => void;
}) {
  const policyReviewResource = resources.find((resource) => resource.route === "/policy-reviews");
  const resourceRoutes = resources.filter((resource) => isInternalResourceRoute(resource) && !isCustomRoute(resource.route) && resource.route !== "/policy-reviews");

  return (
    <Routes>
      <Route path="/" element={<Navigate replace to="/overview" />} />
      <Route
        path="/overview"
        element={(
          <DashboardHome
            language={language}
            dictionary={dictionary}
            session={session}
            capabilities={capabilityItems}
            resources={resources}
            permissions={permissions}
            onRouteChange={onRouteChange}
          />
        )}
      />
      <Route
        path="/capabilities"
        element={(
          <CapabilityConsole
            capabilities={capabilities}
            optionalCapabilities={optionalCapabilities}
            language={language}
            dictionary={dictionary}
            loading={loading}
            error={error}
          />
        )}
      />
      <Route path="/demo-data" element={<DemoDataConsole language={language} dictionary={dictionary} permissions={permissions} deniedPermissions={deniedPermissions} />} />
      <Route path="/api-docs" element={<APIDocsPage language={language} dictionary={dictionary} />} />
      {policyReviewResource ? (
        <Route
          path="/policy-reviews"
          element={(
            <PolicyReviewConsole
              resource={policyReviewResource}
              language={language}
              dictionary={dictionary}
              permissions={permissions}
              deniedPermissions={deniedPermissions}
            />
          )}
        />
      ) : null}
      {resourceRoutes.map((resource) => (
        <Route
          key={resource.route}
          path={resource.route}
          element={(
            <ResourceRoutePage
              resource={resource}
              availableResourceRoutes={resourceRoutes.map((item) => item.route)}
              language={language}
              dictionary={dictionary}
              permissions={permissions}
              deniedPermissions={deniedPermissions}
            />
          )}
        />
      ))}
      <Route path="*" element={<EmptyResource resources={resources} route={activeRoute} language={language} dictionary={dictionary} />} />
    </Routes>
  );
}

function PlatformRefineRuntime({
  resources,
  language,
  themeName,
  customPrimary,
  children,
}: {
  resources: ResourceProps[];
  language: Language;
  themeName: ThemeName;
  customPrimary?: string;
  children: React.ReactNode;
}) {
  return (
    <AdminDesignProvider language={language} themeName={themeName} customPrimary={customPrimary}>
      <AntdApp>
        <Refine
          accessControlProvider={accessControlProvider}
          authProvider={authProvider}
          dataProvider={dataProvider}
          notificationProvider={useNotificationProvider}
          resources={resources}
          routerProvider={routerProvider}
          options={{
            syncWithLocation: true,
            warnWhenUnsavedChanges: true,
            projectId: "platform-go",
            title: { text: "platform-go" },
          }}
        >
          {children}
        </Refine>
      </AntdApp>
    </AdminDesignProvider>
  );
}

function readStoredLanguage(): Language {
  const storedLanguage = readStorageValue(adminPreferenceStorageKeys.language);
  return storedLanguage === "en" ? "en" : "zh";
}

function readStoredThemeName(): ThemeName {
  return normalizeThemeName(readStorageValue(adminPreferenceStorageKeys.theme) ?? "tech");
}

function readStoredLayoutMode(): AdminLayoutMode {
  const storedLayoutMode = readStorageValue(adminPreferenceStorageKeys.layout);
  if (storedLayoutMode === "side" || storedLayoutMode === "top" || storedLayoutMode === "mixed" || storedLayoutMode === "split") {
    return storedLayoutMode;
  }
  return "mixed";
}

function readStoredUIConfig(): AdminUIConfig {
  const rawConfig = readStorageValue(adminPreferenceStorageKeys.ui);
  const themePrimary = themeTokens[readStoredThemeName()].primary;
  if (!rawConfig) {
    return { ...defaultAdminUIConfig, customPrimary: themePrimary };
  }
  try {
    return normalizeUIConfig(JSON.parse(rawConfig));
  } catch {
    return { ...defaultAdminUIConfig, customPrimary: themePrimary };
  }
}

function normalizeUIConfig(value: unknown): AdminUIConfig {
  if (!value || typeof value !== "object") {
    return defaultAdminUIConfig;
  }
  const config = value as Partial<AdminUIConfig>;
  return {
    density: config.density === "comfortable" ? "comfortable" : "compact",
    showWorkTabs: typeof config.showWorkTabs === "boolean" ? config.showWorkTabs : defaultAdminUIConfig.showWorkTabs,
    pageTransition: typeof config.pageTransition === "boolean" ? config.pageTransition : defaultAdminUIConfig.pageTransition,
    sidebarCollapsed: typeof config.sidebarCollapsed === "boolean" ? config.sidebarCollapsed : defaultAdminUIConfig.sidebarCollapsed,
    showLayoutLegend: typeof config.showLayoutLegend === "boolean" ? config.showLayoutLegend : defaultAdminUIConfig.showLayoutLegend,
    watermark: typeof config.watermark === "boolean" ? config.watermark : defaultAdminUIConfig.watermark,
    visualAid: typeof config.visualAid === "boolean" ? config.visualAid : defaultAdminUIConfig.visualAid,
    sidebarWidth: clampNumber(config.sidebarWidth, 220, 304, defaultAdminUIConfig.sidebarWidth),
    menuItemHeight: clampNumber(config.menuItemHeight, 34, 48, defaultAdminUIConfig.menuItemHeight),
    customPrimary: typeof config.customPrimary === "string" && config.customPrimary.trim() ? config.customPrimary : defaultAdminUIConfig.customPrimary,
  };
}

function readStorageValue(key: string) {
  try {
    return window.localStorage.getItem(key);
  } catch {
    return null;
  }
}

function writeStorageValue(key: string, value: string) {
  try {
    window.localStorage.setItem(key, value);
  } catch {
    // Preferences are an enhancement; blocked storage should not break the admin shell.
  }
}

function menuItemToResourceDefinition(item: AdminMenuItem): AdminResourceDefinition {
  return {
    name: item.resource || item.name,
    route: item.route,
    parent: item.parent,
    isExternal: item.isExternal,
    cacheEnabled: item.cacheEnabled,
    title: item.title,
    description: item.description,
    permission: item.permission,
    group: normalizeResourceGroup(item.group),
    icon: item.icon,
  };
}

function resourceDefinitionToRefineResource(resource: AdminResourceDefinition): ResourceProps {
  return {
    name: resource.name,
    list: resource.route,
    meta: {
      label: resource.title.zh,
      parent: resource.parent,
      permission: resource.permission,
      group: resource.group,
      icon: resource.icon,
      keepAlive: resource.cacheEnabled,
      external: resource.isExternal,
    },
  };
}

function isCustomRoute(route: string) {
  return route === "/overview" || route === "/capabilities" || route === "/demo-data" || route === "/api-docs";
}

function isInternalResourceRoute(resource: AdminResourceDefinition) {
  return !resource.isExternal && resource.route.startsWith("/");
}

function normalizeResourceGroup(group: string): AdminResourceDefinition["group"] {
  if (group === "foundation" || group === "governance" || group === "operations" || group === "security") {
    return group;
  }
  return "foundation";
}

function normalizeThemeName(theme: string): ThemeName {
  if (theme === "tech" || theme === "white" || theme === "black" || theme === "warm") {
    return theme;
  }
  return "tech";
}

function clampNumber(value: unknown, min: number, max: number, fallback: number) {
  if (typeof value !== "number" || !Number.isFinite(value)) {
    return fallback;
  }
  return Math.min(Math.max(value, min), max);
}

function routeFromPathname(pathname: string) {
  return normalizeRoute(pathname);
}

function normalizeRoute(route: string) {
  const normalized = route.trim().replace(/\/+$/, "");
  if (normalized === "" || normalized === "/") {
    return "/overview";
  }
  return normalized.startsWith("/") ? normalized : `/${normalized}`;
}

function EmptyResource({
  resources,
  route,
  language,
  dictionary,
}: {
  resources: AdminResourceDefinition[];
  route: string;
  language: Language;
  dictionary: Dictionary;
}) {
  const resource = resources.find((item) => item.route === route);
  if (!resource) {
    return <Empty />;
  }
  return (
    <section className="resource-placeholder">
      <Typography.Title level={1}>{resource.title[language]}</Typography.Title>
      <Typography.Paragraph>{resource.description[language]}</Typography.Paragraph>
      <Empty description={dictionary.resourcePlaceholderDescription} />
    </section>
  );
}
