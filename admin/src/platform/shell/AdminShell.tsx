import {
  ApartmentOutlined,
  ApiOutlined,
  AppstoreOutlined,
  AuditOutlined,
  BellOutlined,
  BookOutlined,
  BranchesOutlined,
  CaretLeftOutlined,
  CaretRightOutlined,
  CloudServerOutlined,
  CloseOutlined,
  ControlOutlined,
  DatabaseOutlined,
  DownOutlined,
  GlobalOutlined,
  HomeOutlined,
  MenuFoldOutlined,
  MenuOutlined,
  MenuUnfoldOutlined,
  MoonOutlined,
  PartitionOutlined,
  PushpinOutlined,
  SafetyCertificateOutlined,
  SearchOutlined,
  SettingOutlined,
  SunOutlined,
  TeamOutlined,
  UploadOutlined,
  UserOutlined,
} from "@ant-design/icons";
import { Avatar, Button, Drawer, Dropdown, Input, Space, Tag, Tooltip, Typography, type MenuProps } from "antd";
import { useEffect, useMemo, useRef, useState, type CSSProperties, type ReactNode } from "react";
import type { AdminCurrentSession, BrandingConfig } from "../api/client";
import type { Dictionary, Language } from "../i18n";
import { themeTokens, type AdminLayoutMode, type ThemeName } from "../theme";
import type { AdminResourceDefinition } from "../resources/registry";
import { PlatformDropdownPanel, PlatformDropdownPlugin, SystemSettingsDrawer, type AdminUIConfig } from "../ui";

const HOME_ROUTE = "/overview";

type AdminShellProps = {
  resources: AdminResourceDefinition[];
  language: Language;
  dictionary: Dictionary;
  themeName: ThemeName;
  layoutMode: AdminLayoutMode;
  uiConfig: AdminUIConfig;
  branding: BrandingConfig | null;
  session: AdminCurrentSession;
  activeRoute: string;
  onLanguageChange: (language: Language) => void;
  onThemeChange: (theme: ThemeName) => void;
  onLayoutModeChange: (mode: AdminLayoutMode) => void;
  onUIConfigChange: (config: AdminUIConfig) => void;
  onRouteChange: (route: string) => void;
  onLogout: () => void;
  children: ReactNode;
};

const iconMap = {
  overview: HomeOutlined,
  tenants: ApartmentOutlined,
  users: UserOutlined,
  roles: TeamOutlined,
  menus: AppstoreOutlined,
  capabilities: PartitionOutlined,
  audit: AuditOutlined,
  apiResources: ApiOutlined,
  dictParams: DatabaseOutlined,
  monitoring: CloudServerOutlined,
  settings: SettingOutlined,
  upload: UploadOutlined,
} as const;

const groupOrder: Array<AdminResourceDefinition["group"]> = ["foundation", "governance", "operations", "security"];
const navParentOrder = [
  "runtime",
  "identity",
  "access",
  "resources",
  "audit",
  "configuration",
  "governance",
  "system",
  "logs",
  "storage",
  "security",
  "release",
  "business",
  "business/access",
  "business/content",
  "business/dispatch",
  "business/fulfillment",
  "business/support",
];

export function AdminShell({
  resources,
  language,
  dictionary,
  themeName,
  layoutMode,
  uiConfig,
  branding,
  session,
  activeRoute,
  onLanguageChange,
  onThemeChange,
  onLayoutModeChange,
  onUIConfigChange,
  onRouteChange,
  onLogout,
  children,
}: AdminShellProps) {
  const [mobileNavOpen, setMobileNavOpen] = useState(false);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [openContext, setOpenContext] = useState<"mobile-work" | "mobile-runtime" | null>(null);
  const [globalSearchQuery, setGlobalSearchQuery] = useState("");
  const workTabsRef = useRef<HTMLElement | null>(null);
  const [workTabsOverflow, setWorkTabsOverflow] = useState({ left: false, right: false });
  const [openTabRoutes, setOpenTabRoutes] = useState<string[]>(() => uniqueRoutes([HOME_ROUTE, activeRoute]));
  const mainRef = useRef<HTMLElement | null>(null);
  const previousRouteRef = useRef(activeRoute);
  const pendingDrawerRouteFocusRef = useRef(false);
  const activeResource = resources.find((resource) => resource.route === activeRoute) ?? resources[0];
  const resourcesByRoute = useMemo(() => new Map(resources.map((resource) => [resource.route, resource])), [resources]);
  const groupedResources = useMemo(
    () =>
      groupOrder
        .map((group) => ({
          group,
          label: dictionary[group],
          resources: resources.filter((resource) => resource.group === group),
        }))
        .filter((group) => group.resources.length > 0),
    [dictionary, resources],
  );
  const activeGroup = groupedResources.find((group) => group.resources.some((resource) => resource.route === activeRoute)) ?? groupedResources[0];
  const globalSearchResults = useMemo(() => {
    const normalizedQuery = globalSearchQuery.trim().toLowerCase();
    if (!normalizedQuery) return [];
    return resources.filter((resource) =>
      [resource.name, resource.title.zh, resource.title.en, resource.description.zh, resource.description.en]
        .some((value) => value.toLowerCase().includes(normalizedQuery)),
    ).slice(0, 8);
  }, [globalSearchQuery, resources]);
  const globalSearchMenuItems = globalSearchResults.length > 0
    ? globalSearchResults.map((resource) => ({ key: resource.route, label: resource.title[language] }))
    : [{ key: "__no_results__", label: dictionary.globalSearchNoResults, disabled: true }];
  const openTabs = openTabRoutes
    .map((route) => resourcesByRoute.get(route))
    .filter((resource): resource is AdminResourceDefinition => Boolean(resource));
  const targetLanguage = language === "zh" ? "en" : "zh";
  const displayName = session.user.name || session.user.username || dictionary.admin;
  const watermarkText = `${branding?.shortName || "platform-go"} · ${displayName}`;
  const showScreenWatermark = uiConfig.watermark && uiConfig.watermarkScopes.includes("screen");
  const avatarLetter = (displayName.trim()[0] || "A").toUpperCase();
  const shellStyle = {
    "--sidebar-width": `${uiConfig.sidebarCollapsed ? 64 : uiConfig.sidebarWidth}px`,
    "--menu-item-height": `${uiConfig.menuItemHeight}px`,
    "--primary": uiConfig.customPrimary,
  } as CSSProperties;

  useEffect(() => {
    if (!resourcesByRoute.has(activeRoute)) {
      return;
    }
    setOpenTabRoutes((current) => uniqueRoutes([HOME_ROUTE, ...current.filter((route) => resourcesByRoute.has(route)), activeRoute]));
  }, [activeRoute, resourcesByRoute]);

  useEffect(() => {
    const updateWorkTabsOverflow = () => {
      const element = workTabsRef.current;
      if (!element) return;
      setWorkTabsOverflow({ left: element.scrollLeft > 2, right: element.scrollLeft + element.clientWidth < element.scrollWidth - 2 });
    };
    updateWorkTabsOverflow();
    window.addEventListener("resize", updateWorkTabsOverflow);
    return () => window.removeEventListener("resize", updateWorkTabsOverflow);
  }, [openTabs.length, uiConfig.showWorkTabs]);

  useEffect(() => {
    if (previousRouteRef.current === activeRoute) {
      return;
    }
    previousRouteRef.current = activeRoute;
    if (pendingDrawerRouteFocusRef.current) {
      return;
    }
    window.requestAnimationFrame(() => mainRef.current?.focus({ preventScroll: true }));
  }, [activeRoute]);

  const openResource = (resource: AdminResourceDefinition) => {
    if (resource.isExternal) {
      window.open(resource.route, "_blank", "noopener,noreferrer");
      return;
    }
    onRouteChange(resource.route);
  };

  const openResourceFromMobileDrawer = (resource: AdminResourceDefinition) => {
    pendingDrawerRouteFocusRef.current = !resource.isExternal && resource.route !== activeRoute;
    openResource(resource);
    setMobileNavOpen(false);
  };

  const handleMobileDrawerOpenChange = (open: boolean) => {
    if (open || !pendingDrawerRouteFocusRef.current) {
      return;
    }
    pendingDrawerRouteFocusRef.current = false;
    window.requestAnimationFrame(() => mainRef.current?.focus({ preventScroll: true }));
  };

  const closeWorkTab = (route: string) => {
    if (route === HOME_ROUTE) {
      return;
    }
    const nextRoutes = uniqueRoutes([HOME_ROUTE, ...openTabRoutes.filter((tabRoute) => tabRoute !== route && tabRoute !== HOME_ROUTE)]);
    const fallbackRoute = nextRoutes.at(-1) ?? HOME_ROUTE;
    setOpenTabRoutes(nextRoutes);
    if (route === activeRoute && fallbackRoute !== activeRoute) {
      onRouteChange(fallbackRoute);
    }
  };

  const handleMobileWorkTabClose = (route: string) => {
    setOpenContext(null);
    closeWorkTab(route);
  };

  const closeWorkTabs = (mode: "current" | "others" | "all" | "left" | "right", route: string) => {
    const index = openTabRoutes.indexOf(route);
    const nextRoutes = (() => {
      switch (mode) {
      case "current":
        return openTabRoutes.filter((tabRoute) => tabRoute !== route || tabRoute === HOME_ROUTE);
      case "others":
        return openTabRoutes.filter((tabRoute) => tabRoute === HOME_ROUTE || tabRoute === route);
      case "all":
        return [HOME_ROUTE];
      case "left":
        return openTabRoutes.filter((tabRoute, tabIndex) => tabRoute === HOME_ROUTE || tabIndex >= index);
      case "right":
        return openTabRoutes.filter((tabRoute, tabIndex) => tabRoute === HOME_ROUTE || tabIndex <= index);
      }
    })();
    const normalizedRoutes = uniqueRoutes([HOME_ROUTE, ...nextRoutes]).filter((tabRoute) => resourcesByRoute.has(tabRoute));
    const fallbackRoute = normalizedRoutes.includes(activeRoute) ? activeRoute : normalizedRoutes.at(-1) ?? HOME_ROUTE;
    setOpenTabRoutes(normalizedRoutes);
    if (fallbackRoute !== activeRoute) {
      onRouteChange(fallbackRoute);
    }
  };

  const changeTheme = (nextTheme: ThemeName) => {
    onThemeChange(nextTheme);
    onUIConfigChange({ ...uiConfig, customPrimary: themeTokens[nextTheme].primary });
  };
  const toggleSidebar = () => onUIConfigChange({ ...uiConfig, sidebarCollapsed: !uiConfig.sidebarCollapsed });

  const shellClass = [
    "platform-shell",
    `layout-${layoutMode}`,
    uiConfig.sidebarCollapsed && layoutMode !== "top" ? "sider-collapsed" : "",
    uiConfig.pageTransition ? "transition-enabled" : "",
    showScreenWatermark ? "watermark-enabled" : "",
    uiConfig.visualAid ? "visual-aid-enabled" : "",
  ]
    .filter(Boolean)
    .join(" ");

  return (
    <div className={shellClass} data-theme={themeName} data-layout={layoutMode} data-density={uiConfig.density} style={shellStyle}>
      <a className="platform-skip-link" href="#platform-main-content">
        {dictionary.skipToContent}
      </a>
      {showScreenWatermark ? (
        <div className="platform-watermark-layer" aria-hidden="true" data-count={uiConfig.watermarkCount} data-scope="viewport">
          {Array.from({ length: uiConfig.watermarkCount }, (_, index) => (
            <span key={`${watermarkText}-${index}`}>{watermarkText}</span>
          ))}
        </div>
      ) : null}
      <aside className="platform-sider" aria-label={dictionary.primaryNavigation}>
        <Brand
          dictionary={dictionary}
          branding={branding}
          compact={layoutMode === "split" || uiConfig.sidebarCollapsed}
          collapsed={uiConfig.sidebarCollapsed}
          collapseLabel={dictionary.collapseSidebar}
          expandLabel={dictionary.expandSidebar}
          onToggleCollapse={toggleSidebar}
        />
        {layoutMode === "split" ? (
          <SplitPrimaryNav groupedResources={groupedResources} activeRoute={activeRoute} onResourceOpen={openResource} />
        ) : (
          <SideNavigation
            groupedResources={layoutMode === "mixed" && activeGroup ? [activeGroup] : groupedResources}
            activeRoute={activeRoute}
            language={language}
            dictionary={dictionary}
            collapsed={uiConfig.sidebarCollapsed}
            onResourceOpen={openResource}
          />
        )}
        <div className="platform-version">platform-go v0.1.0</div>
      </aside>

      {layoutMode === "split" ? (
        <aside className="platform-secondary-nav" aria-label={dictionary.secondaryNavigation}>
          <Typography.Text className="secondary-nav-title">{activeGroup?.label}</Typography.Text>
          <SideNavigation
            groupedResources={activeGroup ? [activeGroup] : []}
            activeRoute={activeRoute}
            language={language}
            dictionary={dictionary}
            onResourceOpen={openResource}
          />
        </aside>
      ) : null}

      <main ref={mainRef} className="platform-main" id="platform-main-content" tabIndex={-1}>
        <header className="platform-topbar">
          <div className="topbar-left">
            <Button
              aria-label={dictionary.openMobileNavigation}
              className="mobile-nav-button"
              icon={<MenuOutlined />}
              onClick={() => setMobileNavOpen(true)}
            />
            <BreadcrumbLabel dictionary={dictionary} activeTitle={activeResource?.title[language] ?? ""} />
          </div>
          <Dropdown
            open={Boolean(globalSearchQuery.trim())}
            menu={{
              items: globalSearchMenuItems,
              onClick: ({ key }) => {
                const resource = resourcesByRoute.get(String(key));
                if (resource) openResource(resource);
                setGlobalSearchQuery("");
              },
            }}
            placement="bottomLeft"
            trigger={[]}
          >
            <Input
              aria-label={dictionary.topSearch}
              autoComplete="off"
              className="global-search desktop-global-search"
              id="platform-global-search"
              name="globalSearch"
              prefix={<SearchOutlined />}
              suffix={<span className="keyboard-hint">⌘ {dictionary.commandHint}</span>}
              placeholder={dictionary.topSearch}
              value={globalSearchQuery}
              onChange={(event) => setGlobalSearchQuery(event.target.value)}
              onPressEnter={() => {
                const resource = globalSearchResults[0];
                if (resource) {
                  openResource(resource);
                  setGlobalSearchQuery("");
                }
              }}
            />
          </Dropdown>
          <Space className="topbar-actions" size={8}>
            <Tooltip title={`${dictionary.switchLanguage}: ${targetLanguage === "zh" ? dictionary.cn : dictionary.en}`}>
              <Button
                aria-label={dictionary.switchLanguage}
                className="topbar-icon-button language-toggle-button"
                icon={<GlobalOutlined />}
                onClick={() => onLanguageChange(targetLanguage)}
              />
            </Tooltip>
            <Tooltip title={dictionary.alerts}>
              <Button aria-label={dictionary.alerts} className="topbar-icon-button" icon={<BellOutlined />} />
            </Tooltip>
            <Tooltip title={themeName === "black" ? dictionary.switchToDayMode : dictionary.switchToNightMode}>
              <Button
                aria-label={themeName === "black" ? dictionary.switchToDayMode : dictionary.switchToNightMode}
                className="topbar-icon-button theme-toggle-button"
                icon={themeName === "black" ? <SunOutlined /> : <MoonOutlined />}
                onClick={() => changeTheme(themeName === "black" ? "tech" : "black")}
              />
            </Tooltip>
            <Button className="user-menu-trigger" aria-label={dictionary.userSettings} onClick={() => setSettingsOpen(true)}>
              <Avatar size={28} className="admin-avatar">
                {avatarLetter}
              </Avatar>
              <span className="user-menu-name">{displayName}</span>
              <SettingOutlined />
            </Button>
          </Space>
        </header>

        {layoutMode === "top" || layoutMode === "mixed" ? (
          <section className={layoutMode === "mixed" ? "platform-top-nav compact" : "platform-top-nav"} aria-label={dictionary.primaryNavigation}>
            <TopNavigation
              groupedResources={groupedResources}
              activeRoute={activeRoute}
              language={language}
              dictionary={dictionary}
              onResourceOpen={openResource}
            />
          </section>
        ) : null}

        <section className="platform-mobile-contextbar">
          <PlatformDropdownPlugin
            open={openContext === "mobile-work"}
            content={(
              <PlatformDropdownPanel
                className="mobile-context-panel"
                title={dictionary.mobileWorkContext}
                description={`${activeGroup?.label ?? dictionary.foundation} · ${activeResource?.title[language] ?? ""}`}
                width={320}
              >
                <div className="mobile-work-context-list">
                  {openTabs.map((resource) => {
                    const Icon = iconMap[resource.icon as keyof typeof iconMap] ?? BookOutlined;
                    const isPinned = resource.route === HOME_ROUTE;
                    return (
                      <div className={resource.route === activeRoute ? "mobile-work-context-item active" : "mobile-work-context-item"} key={resource.route}>
                        <button
                          className="mobile-work-context-route"
                          type="button"
                          onClick={() => {
                            openResource(resource);
                            setOpenContext(null);
                          }}
                        >
                          <Icon />
                          <span>{resource.title[language]}</span>
                        </button>
                        {isPinned ? null : (
                          <button
                            aria-label={`${dictionary.closeTab}: ${resource.title[language]}`}
                            className="mobile-work-context-close"
                            type="button"
                            onClick={() => handleMobileWorkTabClose(resource.route)}
                          >
                            <CloseOutlined />
                          </button>
                        )}
                      </div>
                    );
                  })}
                </div>
              </PlatformDropdownPanel>
            )}
            placement="bottomLeft"
            onOpenChange={(open) => setOpenContext(open ? "mobile-work" : null)}
          >
            <Button className="platform-mobile-context-button" aria-label={dictionary.mobileWorkContext}>
              <span>{activeGroup?.label ?? dictionary.foundation}</span>
              <strong>{activeResource?.title[language] ?? ""}</strong>
              <DownOutlined />
            </Button>
          </PlatformDropdownPlugin>
          <PlatformDropdownPlugin
            open={openContext === "mobile-runtime"}
            content={(
              <PlatformDropdownPanel
                className="mobile-context-panel"
                title={dictionary.mobileRuntimeContext}
                description={`${dictionary.environment}: ${dictionary.production} · ${dictionary.tenant}: ${dictionary.platformTenant}`}
                width={320}
                footer={<Tag>{dictionary.readOnlyContext}</Tag>}
              >
                <div className="mobile-runtime-context-list">
                  <div>
                    <Typography.Text type="secondary">{dictionary.environment}</Typography.Text>
                    <strong>{dictionary.production}</strong>
                  </div>
                  <div>
                    <Typography.Text type="secondary">{dictionary.tenant}</Typography.Text>
                    <strong>{`${dictionary.platformTenant} (platform)`}</strong>
                  </div>
                </div>
              </PlatformDropdownPanel>
            )}
            placement="bottomRight"
            onOpenChange={(open) => setOpenContext(open ? "mobile-runtime" : null)}
          >
            <div className="platform-mobile-context-button context-readonly" aria-label={dictionary.mobileRuntimeContext}>
              <span>{dictionary.environment}</span>
              <strong>{`${dictionary.production} · ${dictionary.platformTenant}`}</strong>
              <Tag>{dictionary.readOnlyContext}</Tag>
            </div>
          </PlatformDropdownPlugin>
        </section>

        <section className={uiConfig.showWorkTabs ? "platform-workbar" : "platform-workbar without-tabs"}>
          {uiConfig.showWorkTabs ? (
            <nav ref={workTabsRef} className="platform-work-tabs" aria-label={dictionary.workTabs} onScroll={() => {
              const element = workTabsRef.current;
              if (element) setWorkTabsOverflow({ left: element.scrollLeft > 2, right: element.scrollLeft + element.clientWidth < element.scrollWidth - 2 });
            }}>
              {openTabs.map((resource) => {
                const Icon = iconMap[resource.icon as keyof typeof iconMap] ?? BookOutlined;
                const isPinned = resource.route === HOME_ROUTE;
                return (
                  <Dropdown
                    key={resource.route}
                    trigger={["contextMenu"]}
                    menu={{
                      items: workTabMenuItems(dictionary, resource.route, openTabRoutes),
                      onClick: ({ key }) => closeWorkTabs(key as "current" | "others" | "all" | "left" | "right", resource.route),
                    }}
                  >
                    <div className={resource.route === activeRoute ? "work-tab active" : "work-tab"}>
                      <button className="work-tab-label" type="button" onClick={() => openResource(resource)}>
                        <Icon />
                        <span>{resource.title[language]}</span>
                        {isPinned ? (
                          <Tooltip title={dictionary.pinnedTab}>
                            <PushpinOutlined className="work-tab-pin" />
                          </Tooltip>
                        ) : null}
                      </button>
                      {isPinned ? null : (
                        <Tooltip title={dictionary.closeTab}>
                          <button
                            aria-label={`${dictionary.closeTab}: ${resource.title[language]}`}
                            className="work-tab-close"
                            type="button"
                            onClick={() => closeWorkTab(resource.route)}
                          >
                            <CloseOutlined />
                          </button>
                        </Tooltip>
                      )}
                    </div>
                  </Dropdown>
                );
              })}
            </nav>
          ) : null}
          {uiConfig.showWorkTabs && workTabsOverflow.left ? (
            <button className="work-tabs-scroll-button left" aria-label={dictionary.scrollWorkTabsLeft} type="button" onClick={() => workTabsRef.current?.scrollBy({ left: -240, behavior: "smooth" })}>
              <CaretLeftOutlined />
            </button>
          ) : null}
          {uiConfig.showWorkTabs && workTabsOverflow.right ? (
            <button className="work-tabs-scroll-button right" aria-label={dictionary.scrollWorkTabsRight} type="button" onClick={() => workTabsRef.current?.scrollBy({ left: 240, behavior: "smooth" })}>
              <CaretRightOutlined />
            </button>
          ) : null}
          <div className="context-controls">
            <div className="context-chip context-readonly" aria-label={dictionary.environmentContext}>
              <span>{dictionary.environment}</span>
              <strong>{dictionary.production}</strong>
              <Tag>{dictionary.readOnlyContext}</Tag>
            </div>
            <div className="context-chip context-readonly" aria-label={dictionary.tenantContext}>
              <span>{dictionary.tenant}</span>
              <strong>{`${dictionary.platformTenant} (platform)`}</strong>
              <Tag>{dictionary.readOnlyContext}</Tag>
            </div>
          </div>
        </section>

        <section className="platform-content">
          {children}
        </section>
      </main>

      <Drawer
        title={branding?.shortName || branding?.productName || "platform-go"}
        getContainer={false}
        open={mobileNavOpen}
        placement="left"
        rootStyle={{ position: "absolute" }}
        width={320}
        afterOpenChange={handleMobileDrawerOpenChange}
        onClose={() => setMobileNavOpen(false)}
      >
        <Input
          aria-label={dictionary.topSearch}
          autoComplete="off"
          className="global-search mobile-global-search"
          id="platform-mobile-global-search"
          name="mobileGlobalSearch"
          prefix={<SearchOutlined />}
          placeholder={dictionary.topSearch}
        />
        <SideNavigation
          groupedResources={groupedResources}
          activeRoute={activeRoute}
          language={language}
          dictionary={dictionary}
          onResourceOpen={openResourceFromMobileDrawer}
        />
      </Drawer>
      <SystemSettingsDrawer
        open={settingsOpen}
        language={language}
        dictionary={dictionary}
        themeName={themeName}
        layoutMode={layoutMode}
        uiConfig={uiConfig}
        branding={branding}
        session={session}
        onClose={() => setSettingsOpen(false)}
        onThemeChange={changeTheme}
        onLayoutModeChange={onLayoutModeChange}
        onUIConfigChange={onUIConfigChange}
        onLogout={onLogout}
      />
    </div>
  );
}

function Brand({
  dictionary,
  branding,
  compact,
  collapsed,
  collapseLabel,
  expandLabel,
  onToggleCollapse,
}: {
  dictionary: Dictionary;
  branding: BrandingConfig | null;
  compact?: boolean;
  collapsed: boolean;
  collapseLabel: string;
  expandLabel: string;
  onToggleCollapse: () => void;
}) {
  const title = branding?.shortName || branding?.productName || "platform-go";
  const subtitle = branding?.productName || dictionary.appSubtitle;
  const label = collapsed ? expandLabel : collapseLabel;
  return (
    <div className={compact ? "platform-brand compact" : "platform-brand"}>
      <div className="platform-brand-main">
        <div className="platform-logo">
          {branding?.logoUrl ? <img alt="" src={branding.logoUrl} /> : <ControlOutlined />}
        </div>
        {compact ? null : (
          <div>
            <Typography.Text className="platform-title">{title}</Typography.Text>
            <Typography.Text className="platform-subtitle">{subtitle}</Typography.Text>
          </div>
        )}
      </div>
      <Tooltip title={label}>
        <Button
          aria-label={label}
          className="brand-collapse-button"
          icon={collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
          onClick={onToggleCollapse}
        />
      </Tooltip>
    </div>
  );
}

function SideNavigation({
  groupedResources,
  activeRoute,
  language,
  dictionary,
  collapsed = false,
  onResourceOpen,
}: {
  groupedResources: Array<{ group: AdminResourceDefinition["group"]; label: string; resources: AdminResourceDefinition[] }>;
  activeRoute: string;
  language: Language;
  dictionary: Dictionary;
  collapsed?: boolean;
  onResourceOpen: (resource: AdminResourceDefinition) => void;
}) {
  if (collapsed) {
    return (
      <div className="side-nav collapsed-flat">
        {groupedResources.map((group) => (
          <div className="side-nav-group" key={group.group}>
            <Typography.Text className="side-nav-group-title">{group.label}</Typography.Text>
            <div className="side-nav-tree">
              {group.resources.map((resource) => (
                <SideNavResourceButton
                  key={resource.route}
                  resource={resource}
                  active={resource.route === activeRoute}
                  language={language}
                  onResourceOpen={onResourceOpen}
                />
              ))}
            </div>
          </div>
        ))}
      </div>
    );
  }

  return (
    <div className="side-nav">
      {groupedResources.map((group) => {
        const tree = buildNavigationTree(group.resources, dictionary, language);
        return (
          <div className="side-nav-group" key={group.group}>
            <Typography.Text className="side-nav-group-title">{group.label}</Typography.Text>
            <NavTree nodes={tree} activeRoute={activeRoute} language={language} onResourceOpen={onResourceOpen} />
          </div>
        );
      })}
    </div>
  );
}

function SplitPrimaryNav({
  groupedResources,
  activeRoute,
  onResourceOpen,
}: {
  groupedResources: Array<{ group: AdminResourceDefinition["group"]; label: string; resources: AdminResourceDefinition[] }>;
  activeRoute: string;
  onResourceOpen: (resource: AdminResourceDefinition) => void;
}) {
  const groupIcons = {
    foundation: AppstoreOutlined,
    governance: SafetyCertificateOutlined,
    operations: CloudServerOutlined,
    security: BranchesOutlined,
  };
  return (
    <div className="split-primary-nav">
      {groupedResources.map((group) => {
        const Icon = groupIcons[group.group];
        const isActive = group.resources.some((resource) => resource.route === activeRoute);
        return (
          <button
            aria-label={group.label}
            className={isActive ? "split-primary-item active" : "split-primary-item"}
            key={group.group}
            title={group.label}
            type="button"
            onClick={() => {
              const firstResource = group.resources[0];
              if (firstResource) {
                onResourceOpen(firstResource);
              }
            }}
          >
            <Icon />
            <span>{group.label}</span>
          </button>
        );
      })}
    </div>
  );
}

function TopNavigation({
  groupedResources,
  activeRoute,
  language,
  dictionary,
  onResourceOpen,
}: {
  groupedResources: Array<{ group: AdminResourceDefinition["group"]; label: string; resources: AdminResourceDefinition[] }>;
  activeRoute: string;
  language: Language;
  dictionary: Dictionary;
  onResourceOpen: (resource: AdminResourceDefinition) => void;
}) {
  return (
    <div className="top-resource-nav">
      {groupedResources.map((group) => {
        const active = group.resources.some((resource) => resource.route === activeRoute);
        return (
          <Dropdown
            key={group.group}
            menu={{
              items: group.resources.map((resource) => {
                const Icon = iconMap[resource.icon as keyof typeof iconMap] ?? BookOutlined;
                return {
                  key: resource.route,
                  icon: <Icon />,
                  label: resource.title[language],
                };
              }),
              onClick: ({ key }) => {
                const resource = group.resources.find((item) => item.route === key);
                if (resource) {
                  onResourceOpen(resource);
                }
              },
            }}
            overlayClassName="platform-dropdown-overlay"
            trigger={["click"]}
          >
            <Button className={active ? "top-resource-nav-item active" : "top-resource-nav-item"}>
              <span>{group.label}</span>
              <DownOutlined />
            </Button>
          </Dropdown>
        );
      })}
    </div>
  );
}

function BreadcrumbLabel({ dictionary, activeTitle }: { dictionary: Dictionary; activeTitle: string }) {
  return (
    <div className="breadcrumb-label">
      <Typography.Text>{dictionary.allSystems}</Typography.Text>
      <Typography.Text className="breadcrumb-current">{activeTitle}</Typography.Text>
    </div>
  );
}

type NavNode = {
  key: string;
  label: string;
  order: number;
  resource?: AdminResourceDefinition;
  children: NavNode[];
};

function buildNavigationTree(resources: AdminResourceDefinition[], dictionary: Dictionary, language: Language) {
  const roots: NavNode[] = [];

  resources.forEach((resource, resourceIndex) => {
    const parentPath = (resource.parent ?? "")
      .split("/")
      .map((segment) => segment.trim())
      .filter(Boolean);
    let siblings = roots;
    let nodePath = "";

    parentPath.forEach((segment) => {
      nodePath = nodePath ? `${nodePath}/${segment}` : segment;
      let node = siblings.find((item) => item.key === nodePath);
      if (!node) {
        node = {
          key: nodePath,
          label: navigationParentLabel(dictionary, nodePath),
          order: navParentOrder.includes(nodePath) ? navParentOrder.indexOf(nodePath) : navParentOrder.length,
          children: [],
        };
        siblings.push(node);
      }
      siblings = node.children;
    });

    siblings.push({
      key: resource.route,
      label: resource.title[language],
      order: navParentOrder.length + resourceIndex,
      resource,
      children: [],
    });
  });

  return sortNavNodes(roots);
}

function sortNavNodes(nodes: NavNode[]): NavNode[] {
  return [...nodes]
    .sort((left, right) => left.order - right.order || left.label.localeCompare(right.label))
    .map((node) => ({ ...node, children: sortNavNodes(node.children) }));
}

function NavTree({
  nodes,
  activeRoute,
  language,
  onResourceOpen,
}: {
  nodes: NavNode[];
  activeRoute: string;
  language: Language;
  onResourceOpen: (resource: AdminResourceDefinition) => void;
}) {
  return (
    <div className="side-nav-tree">
      {nodes.map((node) => {
        if (node.resource) {
          return (
            <SideNavResourceButton
              key={node.key}
              resource={node.resource}
              active={node.resource.route === activeRoute}
              language={language}
              onResourceOpen={onResourceOpen}
            />
          );
        }

        return (
          <details className="side-nav-branch" key={node.key} open={nodeHasActive(node, activeRoute)}>
            <summary>{node.label}</summary>
            <NavTree nodes={node.children} activeRoute={activeRoute} language={language} onResourceOpen={onResourceOpen} />
          </details>
        );
      })}
    </div>
  );
}

function SideNavResourceButton({
  resource,
  active,
  language,
  onResourceOpen,
}: {
  resource: AdminResourceDefinition;
  active: boolean;
  language: Language;
  onResourceOpen: (resource: AdminResourceDefinition) => void;
}) {
  const Icon = iconMap[resource.icon as keyof typeof iconMap] ?? BookOutlined;
  return (
    <button
      aria-label={resource.title[language]}
      className={active ? "side-nav-item active" : "side-nav-item"}
      title={resource.title[language]}
      type="button"
      onClick={() => onResourceOpen(resource)}
    >
      <Icon />
      <span className="side-nav-label">{resource.title[language]}</span>
      {resource.isExternal ? <GlobalOutlined className="side-nav-extra-icon" /> : null}
    </button>
  );
}

function nodeHasActive(node: NavNode, activeRoute: string): boolean {
  return node.children.some((child) => child.resource?.route === activeRoute || nodeHasActive(child, activeRoute));
}

function navigationParentLabel(dictionary: Dictionary, nodePath: string) {
  const labels: Record<string, string> = {
    runtime: dictionary.navRuntime,
    identity: dictionary.navIdentity,
    access: dictionary.navAccess,
    resources: dictionary.navResources,
    audit: dictionary.navAudit,
    configuration: dictionary.navConfiguration,
    governance: dictionary.navGovernance,
    system: dictionary.navSystem,
    logs: dictionary.navLogs,
    storage: dictionary.navStorage,
    security: dictionary.navSecurity,
    release: dictionary.navRelease,
    business: dictionary.navBusiness,
    "business/access": dictionary.navBusinessAccess,
    "business/content": dictionary.navBusinessContent,
    "business/dispatch": dictionary.navBusinessDispatch,
    "business/fulfillment": dictionary.navBusinessFulfillment,
    "business/support": dictionary.navBusinessSupport,
  };
  return labels[nodePath] ?? nodePath.split("/").at(-1)?.replace(/[-_]/g, " ") ?? nodePath;
}

function workTabMenuItems(dictionary: Dictionary, route: string, routes: string[]): MenuProps["items"] {
  const index = routes.indexOf(route);
  const isPinned = route === HOME_ROUTE;
  return [
    { key: "current", label: dictionary.closeCurrentTab, disabled: isPinned },
    { key: "others", label: dictionary.closeOtherTabs, disabled: routes.length <= 1 },
    { key: "all", label: dictionary.closeAllTabs, disabled: routes.length <= 1 },
    { type: "divider" },
    { key: "left", label: dictionary.closeTabsToLeft, disabled: index <= 1 },
    { key: "right", label: dictionary.closeTabsToRight, disabled: index < 0 || index >= routes.length - 1 },
  ];
}

function uniqueRoutes(routes: string[]) {
  return routes.filter((route, index) => route && routes.indexOf(route) === index);
}
