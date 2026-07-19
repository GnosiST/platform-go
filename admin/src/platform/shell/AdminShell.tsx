import {
  ApartmentOutlined,
  ApiOutlined,
  AppstoreOutlined,
  AuditOutlined,
  BellOutlined,
  BookOutlined,
  BranchesOutlined,
  CloudServerOutlined,
  CloseOutlined,
  ControlOutlined,
  DatabaseOutlined,
  DownOutlined,
  EditOutlined,
  GlobalOutlined,
  HomeOutlined,
  LeftOutlined,
  MenuFoldOutlined,
  MenuOutlined,
  MenuUnfoldOutlined,
  MoonOutlined,
  PartitionOutlined,
  PushpinOutlined,
  ReloadOutlined,
  RightOutlined,
  SafetyCertificateOutlined,
  SearchOutlined,
  SettingOutlined,
  SunOutlined,
  TeamOutlined,
  UploadOutlined,
  UserOutlined,
} from "@ant-design/icons";
import { Alert, App, Avatar, Button, Drawer, Dropdown, Form, Input, Space, Tag, Tooltip, Typography, type MenuProps } from "antd";
import { useEffect, useMemo, useRef, useState, type CSSProperties, type ReactNode, type WheelEvent } from "react";
import {
  changeCurrentAdminPassword,
  getCurrentAdminProfile,
  getFrontendVersion,
  updateCurrentAdminProfile,
  type AdminCurrentSession,
  type AdminProfile,
  type BrandingConfig,
} from "../api/client";
import type { Dictionary, Language } from "../i18n";
import { themeTokens, type AdminLayoutMode, type ThemeName } from "../theme";
import type { AdminResourceDefinition } from "../resources/registry";
import { AdminModal, PlatformDropdownPanel, PlatformDropdownPlugin, SystemSettingsDrawer, type AdminUIConfig } from "../ui";

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
  "operations",
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

const aggregatedWorkbenchChildRoutes = new Set([
  "/branding",
  "/parameters",
  "/notification-channels",
  "/notification-providers",
  "/notification-send-policies",
  "/notification-templates",
  "/notifications",
  "/notification-deliveries",
]);

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
  const [profileOpen, setProfileOpen] = useState(false);
  const [profileEditorOpen, setProfileEditorOpen] = useState(false);
  const [profileSnapshot, setProfileSnapshot] = useState<AdminProfile | null>(null);
  const [openContext, setOpenContext] = useState<"mobile-work" | null>(null);
  const [globalSearchQuery, setGlobalSearchQuery] = useState("");
  const workTabsRef = useRef<HTMLElement | null>(null);
  const [workTabsScroll, setWorkTabsScroll] = useState({ left: false, right: false });
  const [openTabRoutes, setOpenTabRoutes] = useState<string[]>(() => uniqueRoutes([HOME_ROUTE, activeRoute]));
  const [frontendUpdateAvailable, setFrontendUpdateAvailable] = useState(false);
  const mainRef = useRef<HTMLElement | null>(null);
  const previousRouteRef = useRef(activeRoute);
  const pendingDrawerRouteFocusRef = useRef(false);
  const frontendVersionSignatureRef = useRef("");
  const activeResource = resources.find((resource) => resource.route === activeRoute) ?? resources[0];
  const resourcesByRoute = useMemo(() => new Map(resources.map((resource) => [resource.route, resource])), [resources]);
  const navigationResources = useMemo(
    () => resources.filter((resource) => !aggregatedWorkbenchChildRoutes.has(resource.route)),
    [resources],
  );
  const groupedResources = useMemo(
    () =>
      groupOrder
        .map((group) => ({
          group,
          label: dictionary[group],
          resources: navigationResources.filter((resource) => resource.group === group),
        }))
        .filter((group) => group.resources.length > 0),
    [dictionary, navigationResources],
  );
  const activeGroup = groupedResources.find((group) => group.resources.some((resource) => resource.route === activeRoute))
    ?? groupedResources.find((group) => group.group === activeResource?.group)
    ?? groupedResources[0];
  const globalSearchResults = useMemo(() => {
    const normalizedQuery = globalSearchQuery.trim().toLowerCase();
    if (!normalizedQuery) return [];
    return navigationResources.filter((resource) =>
      [resource.name, resource.title.zh, resource.title.en, resource.description.zh, resource.description.en]
        .some((value) => value.toLowerCase().includes(normalizedQuery)),
    ).slice(0, 8);
  }, [globalSearchQuery, navigationResources]);
  const globalSearchMenuItems = globalSearchResults.length > 0
    ? globalSearchResults.map((resource) => ({ key: resource.route, label: resource.title[language] }))
    : [{ key: "__no_results__", label: dictionary.globalSearchNoResults, disabled: true }];
  const openTabs = openTabRoutes
    .map((route) => resourcesByRoute.get(route))
    .filter((resource): resource is AdminResourceDefinition => Boolean(resource));
  const targetLanguage = language === "zh" ? "en" : "zh";
  useEffect(() => {
    setProfileSnapshot(null);
  }, [session.user.id]);
  const displayName = profileSnapshot?.name || session.user.name || session.user.username || dictionary.admin;
  const watermarkText = `${branding?.shortName || "platform-go"} · ${displayName}`;
  const showScreenWatermark = uiConfig.watermark && uiConfig.watermarkScopes.includes("screen");
  const avatarUrl = profileSnapshot?.avatarUrl || "";
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
    if (previousRouteRef.current === activeRoute) {
      return;
    }
    previousRouteRef.current = activeRoute;
    if (pendingDrawerRouteFocusRef.current) {
      return;
    }
    window.requestAnimationFrame(() => mainRef.current?.focus({ preventScroll: true }));
  }, [activeRoute]);

  useEffect(() => {
    const nav = workTabsRef.current;
    if (!nav || !uiConfig.showWorkTabs) {
      setWorkTabsScroll({ left: false, right: false });
      return;
    }

    let frame = 0;
    const updateScrollState = () => {
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(() => {
        const maxScrollLeft = Math.max(0, nav.scrollWidth - nav.clientWidth);
        setWorkTabsScroll({
          left: nav.scrollLeft > 1,
          right: maxScrollLeft - nav.scrollLeft > 1,
        });
      });
    };
    const resizeObserver = new ResizeObserver(updateScrollState);
    const mutationObserver = new MutationObserver(updateScrollState);
    resizeObserver.observe(nav);
    mutationObserver.observe(nav, { childList: true, subtree: true });
    nav.addEventListener("scroll", updateScrollState, { passive: true });
    updateScrollState();

    return () => {
      cancelAnimationFrame(frame);
      resizeObserver.disconnect();
      mutationObserver.disconnect();
      nav.removeEventListener("scroll", updateScrollState);
    };
  }, [openTabs.length, uiConfig.showWorkTabs, language]);

  useEffect(() => {
    workTabsRef.current?.querySelector<HTMLElement>(".work-tab.active")?.scrollIntoView({ block: "nearest", inline: "nearest" });
  }, [activeRoute, openTabs.length]);

  useEffect(() => {
    let cancelled = false;
    const checkFrontendVersion = async () => {
      try {
        const signature = frontendVersionSignature(await getFrontendVersion());
        if (!signature || cancelled) {
          return;
        }
        if (!frontendVersionSignatureRef.current) {
          frontendVersionSignatureRef.current = signature;
          return;
        }
        if (frontendVersionSignatureRef.current !== signature) {
          setFrontendUpdateAvailable(true);
        }
      } catch {
        // Static version polling is best-effort and must not disrupt admin work.
      }
    };
    const handleVisibilityChange = () => {
      if (document.visibilityState === "visible") {
        void checkFrontendVersion();
      }
    };
    void checkFrontendVersion();
    const interval = window.setInterval(checkFrontendVersion, 300000);
    document.addEventListener("visibilitychange", handleVisibilityChange);
    return () => {
      cancelled = true;
      window.clearInterval(interval);
      document.removeEventListener("visibilitychange", handleVisibilityChange);
    };
  }, []);

  useEffect(() => {
    if (!profileOpen) {
      return;
    }

    const handleProfileOutsidePointerDown = (event: PointerEvent) => {
      const target = event.target;
      if (!(target instanceof Element)) {
        return;
      }
      if (target.closest(".profile-menu-trigger") || target.closest(".profile-summary-panel")) {
        return;
      }
      setProfileOpen(false);
    };
    const handleProfileEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setProfileOpen(false);
      }
    };

    document.addEventListener("pointerdown", handleProfileOutsidePointerDown, true);
    document.addEventListener("keydown", handleProfileEscape);
    return () => {
      document.removeEventListener("pointerdown", handleProfileOutsidePointerDown, true);
      document.removeEventListener("keydown", handleProfileEscape);
    };
  }, [profileOpen]);

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
  const scrollWorkTabs = (direction: -1 | 1) => {
    const nav = workTabsRef.current;
    if (!nav) {
      return;
    }
    nav.scrollBy({ left: direction * Math.max(nav.clientWidth * 0.75, 160), behavior: "smooth" });
  };
  const handleWorkTabsWheel = (event: WheelEvent<HTMLElement>) => {
    const delta = Math.abs(event.deltaY) > Math.abs(event.deltaX) ? event.deltaY : event.deltaX;
    if (!delta || (!workTabsScroll.left && !workTabsScroll.right)) {
      return;
    }
    event.preventDefault();
    event.currentTarget.scrollBy({ left: delta, behavior: "auto" });
  };

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
            <Tooltip title={dictionary.userSettings}>
              <Button
                aria-label={dictionary.userSettings}
                className="topbar-icon-button settings-trigger-button"
                icon={<SettingOutlined />}
                onClick={() => {
                  setProfileOpen(false);
                  setSettingsOpen(true);
                }}
              />
            </Tooltip>
            <PlatformDropdownPlugin
              open={profileOpen}
              content={(
                <ProfileSummaryPanel
                  avatarLetter={avatarLetter}
                  avatarUrl={avatarUrl}
                  dictionary={dictionary}
                  displayName={displayName}
                  profile={profileSnapshot}
                  session={session}
                  onEditProfile={() => {
                    setProfileOpen(false);
                    setProfileEditorOpen(true);
                  }}
                />
              )}
              placement="bottomRight"
              trigger={["click"]}
              onOpenChange={setProfileOpen}
            >
              <Button className="profile-menu-trigger" aria-label={dictionary.personalProfile}>
                <Avatar size={28} className="admin-avatar" src={avatarUrl || undefined}>
                  {avatarLetter}
                </Avatar>
              </Button>
            </PlatformDropdownPlugin>
          </Space>
        </header>

        {layoutMode === "top" || layoutMode === "mixed" ? (
          <section className={layoutMode === "mixed" ? "platform-top-nav compact" : "platform-top-nav"} aria-label={dictionary.primaryNavigation}>
            <TopNavigation
              groupedResources={groupedResources}
              activeRoute={activeRoute}
              language={language}
              dictionary={dictionary}
              compact={layoutMode === "mixed"}
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
        </section>

        {uiConfig.showWorkTabs ? (
          <section className="platform-workbar">
            <div className="platform-work-tabs-shell">
              {workTabsScroll.left || workTabsScroll.right ? (
                <Button
                  aria-label={dictionary.scrollWorkTabsLeft}
                  className="work-tabs-scroll-button"
                  disabled={!workTabsScroll.left}
                  icon={<LeftOutlined />}
                  onClick={() => scrollWorkTabs(-1)}
                />
              ) : null}
              <nav ref={workTabsRef} className="platform-work-tabs" aria-label={dictionary.workTabs} onWheel={handleWorkTabsWheel}>
                {openTabs.map((resource) => {
                  const Icon = iconMap[resource.icon as keyof typeof iconMap] ?? BookOutlined;
                  const isPinned = resource.route === HOME_ROUTE;
                  const active = resource.route === activeRoute;
                  return (
                    <Dropdown
                      key={resource.route}
                      trigger={["contextMenu"]}
                      menu={{
                        items: workTabMenuItems(dictionary, resource.route, openTabRoutes),
                        onClick: ({ key }) => closeWorkTabs(key as "current" | "others" | "all" | "left" | "right", resource.route),
                      }}
                    >
                      <div className={active ? "work-tab active" : "work-tab"}>
                        <button
                          aria-current={active ? "page" : undefined}
                          className="work-tab-label"
                          type="button"
                          onClick={() => openResource(resource)}
                        >
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
              {workTabsScroll.left || workTabsScroll.right ? (
                <Button
                  aria-label={dictionary.scrollWorkTabsRight}
                  className="work-tabs-scroll-button"
                  disabled={!workTabsScroll.right}
                  icon={<RightOutlined />}
                  onClick={() => scrollWorkTabs(1)}
                />
              ) : null}
            </div>
          </section>
        ) : null}

        {frontendUpdateAvailable ? (
          <Alert
            action={(
              <Button icon={<ReloadOutlined />} size="small" type="primary" onClick={() => window.location.reload()}>
                {dictionary.reloadPage}
              </Button>
            )}
            className="platform-update-alert"
            description={dictionary.frontendUpdateDescription}
            message={dictionary.frontendUpdateAvailable}
            showIcon
            type="info"
          />
        ) : null}

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
      <ProfileEditorModal
        open={profileEditorOpen}
        avatarLetter={avatarLetter}
        avatarUrl={avatarUrl}
        dictionary={dictionary}
        displayName={displayName}
        profile={profileSnapshot}
        session={session}
        onClose={() => setProfileEditorOpen(false)}
        onProfileSaved={setProfileSnapshot}
      />
    </div>
  );
}

function ProfileSummaryPanel({
  avatarLetter,
  avatarUrl,
  dictionary,
  displayName,
  profile,
  session,
  onEditProfile,
}: {
  avatarLetter: string;
  avatarUrl: string;
  dictionary: Dictionary;
  displayName: string;
  profile: AdminProfile | null;
  session: AdminCurrentSession;
  onEditProfile: () => void;
}) {
  const tenantCode = profile?.tenantCode || session.user.tenantCode || "platform";
  const tenantDisplay = tenantCode === "platform" ? `${dictionary.platformTenant} (platform)` : tenantCode;
  const organizationDisplay = profile?.orgUnitCode || session.user.orgUnitCode || dictionary.notConfigured;
  const roles = normalizeProfileRoleLabels(session.roles);
  const profileFields = [
    { label: dictionary.avatar, value: avatarUrl || dictionary.notConfigured },
    { label: dictionary.nickname, value: profile?.nickname || profile?.name || session.user.name || dictionary.notConfigured },
    { label: dictionary.phone, value: profile?.phone || dictionary.notConfigured },
    { label: dictionary.email, value: profile?.email || dictionary.notConfigured },
    { label: dictionary.address, value: profile?.address || dictionary.notConfigured },
  ];

  return (
    <PlatformDropdownPanel
      className="profile-summary-panel"
      bodyClassName="profile-summary-body"
      title={dictionary.personalProfile}
      description={session.user.username || displayName}
      width={460}
      maxHeight="min(720px, calc(100vh - 32px))"
      footer={(
        <Button block icon={<EditOutlined />} onClick={onEditProfile}>
          {dictionary.editProfile}
        </Button>
      )}
    >
      <div className="profile-summary-header">
        <Avatar size={44} className="admin-avatar" src={avatarUrl || undefined}>
          {avatarLetter}
        </Avatar>
        <div>
          <Typography.Text strong>{displayName}</Typography.Text>
          <Typography.Text type="secondary">{session.user.username || dictionary.notConfigured}</Typography.Text>
        </div>
      </div>
      <dl className="profile-summary-facts">
        <ProfileFact label={dictionary.tenant} value={tenantDisplay} />
        <ProfileFact label={dictionary.organization} value={organizationDisplay} />
        <div className="profile-summary-fact">
          <dt>{dictionary.roles}</dt>
          <dd className="profile-role-list">
            {roles.length > 0 ? roles.map((role) => <Tag key={role}>{role}</Tag>) : dictionary.notConfigured}
          </dd>
        </div>
      </dl>
      <div className="profile-field-grid">
        {profileFields.map((field) => (
          <div key={field.label}>
            <Typography.Text type="secondary">{field.label}</Typography.Text>
            <strong>{field.value}</strong>
          </div>
        ))}
      </div>
    </PlatformDropdownPanel>
  );
}

function ProfileFact({ label, value }: { label: string; value: string }) {
  return (
    <div className="profile-summary-fact">
      <dt>{label}</dt>
      <dd>{value}</dd>
    </div>
  );
}

function ProfileEditorModal({
  open,
  avatarLetter,
  avatarUrl,
  dictionary,
  displayName,
  profile,
  session,
  onClose,
  onProfileSaved,
}: {
  open: boolean;
  avatarLetter: string;
  avatarUrl: string;
  dictionary: Dictionary;
  displayName: string;
  profile: AdminProfile | null;
  session: AdminCurrentSession;
  onClose: () => void;
  onProfileSaved: (profile: AdminProfile) => void;
}) {
  const { message } = App.useApp();
  const [form] = Form.useForm<ProfileEditorFormValues>();
  const [passwordForm] = Form.useForm<ProfilePasswordFormValues>();
  const [loadedProfile, setLoadedProfile] = useState<AdminProfile | null>(profile);
  const [profileLoading, setProfileLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [changingPassword, setChangingPassword] = useState(false);
  const [profileError, setProfileError] = useState("");
  const watchedAvatarUrl = Form.useWatch("avatarUrl", form) as string | undefined;
  const effectiveProfile = loadedProfile ?? profile;
  const tenantCode = effectiveProfile?.tenantCode || session.user.tenantCode || "platform";
  const tenantDisplay = tenantCode === "platform" ? `${dictionary.platformTenant} (platform)` : tenantCode;
  const organizationDisplay = effectiveProfile?.orgUnitCode || session.user.orgUnitCode || dictionary.notConfigured;
  const roles = normalizeProfileRoleLabels(session.roles);
  const avatarPreviewUrl = (watchedAvatarUrl ?? avatarUrl).trim();
  const credentialStatus = effectiveProfile?.credentials.message || dictionary.passwordActionUnavailable;

  useEffect(() => {
    if (!open) {
      return;
    }
    form.setFieldsValue(profileFormValues(profile, session));
    passwordForm.resetFields();
    setLoadedProfile(profile);
    setProfileError("");
    setProfileLoading(true);
    let cancelled = false;
    getCurrentAdminProfile()
      .then((result) => {
        if (cancelled) return;
        setLoadedProfile(result.profile);
        onProfileSaved(result.profile);
        form.setFieldsValue(profileFormValues(result.profile, session));
      })
      .catch((error: unknown) => {
        if (cancelled) return;
        setProfileError(profileRequestErrorMessage(error, dictionary.profileLoadFailed));
      })
      .finally(() => {
        if (!cancelled) {
          setProfileLoading(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [dictionary.profileLoadFailed, form, open, passwordForm, session.user.id]);

  const handleSave = () => {
    void form.validateFields()
      .then((values) => {
        setSaving(true);
        return updateCurrentAdminProfile(profileUpdateInput(values));
      })
      .then((result) => {
        setLoadedProfile(result.profile);
        onProfileSaved(result.profile);
        form.setFieldsValue(profileFormValues(result.profile, session));
        setProfileError("");
        message.success(dictionary.profileSaveSuccess);
      })
      .catch((error: unknown) => {
        if (isProfileFormValidationError(error)) {
          return;
        }
        message.error(profileRequestErrorMessage(error, dictionary.profileSaveFailed));
      })
      .finally(() => setSaving(false));
  };

  const handleChangePassword = () => {
    void passwordForm.validateFields()
      .then((values) => {
        setChangingPassword(true);
        return changeCurrentAdminPassword({
          currentPassword: values.currentPassword,
          newPassword: values.newPassword,
        });
      })
      .then((result) => {
        if (result.profile) {
          setLoadedProfile(result.profile);
          onProfileSaved(result.profile);
        }
        passwordForm.resetFields();
        message.success(dictionary.passwordChangeSuccess);
      })
      .catch((error: unknown) => {
        if (isProfileFormValidationError(error)) {
          return;
        }
        message.error(profileRequestErrorMessage(error, dictionary.passwordChangeFailed));
      })
      .finally(() => setChangingPassword(false));
  };

  return (
    <AdminModal
      className="profile-editor-modal"
      title={dictionary.editProfile}
      open={open}
      size="xl"
      width={980}
      footer={[
        <Button key="close" onClick={onClose}>{dictionary.close}</Button>,
        <Button key="save" loading={saving} type="primary" onClick={handleSave}>{dictionary.saveProfile}</Button>,
      ]}
      onCancel={onClose}
    >
      <div className="profile-editor-content">
        {profileError ? (
          <Alert showIcon type="warning" message={dictionary.profileLoadFailed} description={profileError} />
        ) : null}
        <section className="profile-editor-section">
          <header className="profile-editor-section-header">
            <Avatar size={52} className="admin-avatar" src={avatarPreviewUrl || undefined}>{avatarLetter}</Avatar>
            <div>
              <Typography.Text type="secondary">{dictionary.profileBasics}</Typography.Text>
              <Typography.Text strong>{displayName}</Typography.Text>
              <Typography.Text type="secondary">{session.user.username || dictionary.notConfigured}</Typography.Text>
            </div>
          </header>
          <Form form={form} layout="vertical" className="profile-editor-grid" disabled={profileLoading || saving}>
            <Form.Item className="profile-editor-field" label={dictionary.username}>
              <Input readOnly value={session.user.username || dictionary.notConfigured} />
            </Form.Item>
            <Form.Item className="profile-editor-field" label={dictionary.nickname} name="nickname" rules={[{ required: true, message: dictionary.profileNameRequired }]}>
              <Input autoComplete="name" maxLength={80} />
            </Form.Item>
            <Form.Item className="profile-editor-field" label={dictionary.avatar} name="avatarUrl">
              <Input autoComplete="photo" maxLength={2048} />
            </Form.Item>
            <Form.Item className="profile-editor-field" label={dictionary.phone} name="phone" rules={[{ validator: optionalPhoneValidator(dictionary.profilePhoneInvalid) }]}>
              <Input autoComplete="tel" maxLength={32} />
            </Form.Item>
            <Form.Item className="profile-editor-field" label={dictionary.email} name="email" rules={[{ type: "email", message: dictionary.profileEmailInvalid }]}>
              <Input autoComplete="email" maxLength={254} />
            </Form.Item>
            <Form.Item className="profile-editor-field full" label={dictionary.address} name="address">
              <Input.TextArea autoComplete="street-address" autoSize={{ minRows: 2, maxRows: 4 }} maxLength={240} />
            </Form.Item>
          </Form>
        </section>
        <section className="profile-editor-section">
          <Typography.Text strong>{dictionary.profileIdentityContext}</Typography.Text>
          <div className="profile-editor-context">
            <ProfileContextItem label={dictionary.tenant} value={tenantDisplay} />
            <ProfileContextItem label={dictionary.organization} value={organizationDisplay} />
            <div>
              <Typography.Text type="secondary">{dictionary.roles}</Typography.Text>
              <div className="profile-editor-role-list">
                {roles.length > 0 ? roles.map((role) => <Tag key={role}>{role}</Tag>) : <Typography.Text>{dictionary.notConfigured}</Typography.Text>}
              </div>
            </div>
          </div>
        </section>
        <section className="profile-editor-section">
          <Typography.Text strong>{dictionary.accountSecurity}</Typography.Text>
          <Alert showIcon type="info" message={dictionary.passwordChangeReady} description={credentialStatus} />
          <Form form={passwordForm} layout="vertical" className="profile-editor-grid" disabled={profileLoading || changingPassword}>
            <Form.Item
              className="profile-editor-field"
              label={dictionary.currentPassword}
              name="currentPassword"
              rules={[{ required: true, message: dictionary.currentPasswordRequired }]}
            >
              <Input.Password autoComplete="current-password" placeholder={dictionary.currentPasswordPlaceholder} />
            </Form.Item>
            <Form.Item
              className="profile-editor-field"
              label={dictionary.newPassword}
              name="newPassword"
              rules={[
                { required: true, message: dictionary.newPasswordRequired },
                { min: 8, message: formatTemplate(dictionary.validationMinLength, { label: dictionary.newPassword, min: "8" }) },
              ]}
            >
              <Input.Password autoComplete="new-password" placeholder={dictionary.newPasswordPlaceholder} />
            </Form.Item>
            <Form.Item
              className="profile-editor-field"
              dependencies={["newPassword"]}
              label={dictionary.confirmPassword}
              name="confirmPassword"
              rules={[
                { required: true, message: dictionary.confirmPasswordRequired },
                ({ getFieldValue }) => ({
                  validator: async (_rule, value) => {
                    if (!value || getFieldValue("newPassword") === value) {
                      return;
                    }
                    throw new Error(dictionary.passwordConfirmMismatch);
                  },
                }),
              ]}
            >
              <Input.Password autoComplete="new-password" placeholder={dictionary.confirmPasswordPlaceholder} />
            </Form.Item>
          </Form>
          <Space wrap className="profile-editor-security-actions">
            <Button type="primary" loading={changingPassword} onClick={handleChangePassword}>{dictionary.changePassword}</Button>
            <Button disabled>{dictionary.resetPassword}</Button>
          </Space>
          <Typography.Text type="secondary">{dictionary.passwordEncryptedSecretHint}</Typography.Text>
        </section>
      </div>
    </AdminModal>
  );
}

type ProfileEditorFormValues = {
  avatarUrl?: string;
  nickname?: string;
  phone?: string;
  email?: string;
  address?: string;
};

type ProfilePasswordFormValues = {
  currentPassword: string;
  newPassword: string;
  confirmPassword: string;
};

function profileFormValues(profile: AdminProfile | null, session: AdminCurrentSession): ProfileEditorFormValues {
  return {
    avatarUrl: profile?.avatarUrl ?? "",
    nickname: profile?.nickname || profile?.name || session.user.name || "",
    phone: profile?.phone ?? "",
    email: profile?.email ?? "",
    address: profile?.address ?? "",
  };
}

function profileUpdateInput(values: ProfileEditorFormValues) {
  return {
    avatarUrl: trimProfileFormValue(values.avatarUrl),
    nickname: trimProfileFormValue(values.nickname),
    phone: trimProfileFormValue(values.phone),
    email: trimProfileFormValue(values.email),
    address: trimProfileFormValue(values.address),
  };
}

function trimProfileFormValue(value: string | undefined) {
  return (value ?? "").trim();
}

function optionalPhoneValidator(message: string) {
  return async (_rule: unknown, value?: string) => {
    const normalized = trimProfileFormValue(value);
    if (!normalized) {
      return;
    }
    const compact = normalized.replace(/[ ()-]/g, "");
    if (!/^\+?\d{6,18}$/.test(compact)) {
      throw new Error(message);
    }
  };
}

function isProfileFormValidationError(error: unknown) {
  return Boolean(error && typeof error === "object" && "errorFields" in error);
}

function profileRequestErrorMessage(error: unknown, fallback: string) {
  if (error instanceof Error && error.message.trim()) {
    return error.message;
  }
  return fallback;
}

function formatTemplate(template: string, values: Record<string, string>) {
  return Object.entries(values).reduce((result, [key, value]) => result.replaceAll(`{${key}}`, value), template);
}

function normalizeProfileRoleLabels(rawRoles: unknown) {
  if (!Array.isArray(rawRoles)) {
    return [];
  }
  return rawRoles
    .map((role) => {
      if (typeof role === "string") {
        const value = role.trim();
        return /^\d+$/.test(value) ? "" : value;
      }
      if (!role || typeof role !== "object") {
        return "";
      }
      const record = role as Record<string, unknown>;
      for (const key of ["name", "label", "code", "id"]) {
        const value = record[key];
        if (typeof value === "string" && value.trim() && !/^\d+$/.test(value.trim())) {
          return value.trim();
        }
      }
      return "";
    })
    .filter(Boolean);
}

function ProfileContextItem({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <Typography.Text type="secondary">{label}</Typography.Text>
      <strong>{value}</strong>
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
  compact = false,
  onResourceOpen,
}: {
  groupedResources: Array<{ group: AdminResourceDefinition["group"]; label: string; resources: AdminResourceDefinition[] }>;
  activeRoute: string;
  language: Language;
  dictionary: Dictionary;
  compact?: boolean;
  onResourceOpen: (resource: AdminResourceDefinition) => void;
}) {
  return (
    <div className="top-resource-nav">
      {groupedResources.map((group) => {
        const active = group.resources.some((resource) => resource.route === activeRoute);
        const firstResource = group.resources[0];
        if (compact) {
          return (
            <Button
              aria-current={active ? "page" : undefined}
              aria-label={`${group.label}: ${firstResource?.title[language] ?? ""}`}
              className={active ? "top-resource-nav-item active" : "top-resource-nav-item"}
              key={group.group}
              onClick={() => firstResource && onResourceOpen(firstResource)}
            >
              <span>{group.label}</span>
            </Button>
          );
        }
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
    operations: dictionary.operations,
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

function frontendVersionSignature(value: unknown) {
  if (!value || typeof value !== "object") {
    return "";
  }
  const record = value as Record<string, unknown>;
  return ["version", "buildId", "commit", "builtAt"]
    .map((key) => (typeof record[key] === "string" ? record[key] : ""))
    .filter(Boolean)
    .join(":");
}
