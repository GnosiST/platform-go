import {
  ApartmentOutlined,
  ApiOutlined,
  AppstoreOutlined,
  AuditOutlined,
  BellOutlined,
  BookOutlined,
  BranchesOutlined,
  CloudServerOutlined,
  ControlOutlined,
  DatabaseOutlined,
  FileSearchOutlined,
  GlobalOutlined,
  HomeOutlined,
  MenuFoldOutlined,
  MenuOutlined,
  MoonOutlined,
  PartitionOutlined,
  SafetyCertificateOutlined,
  SearchOutlined,
  SettingOutlined,
  TeamOutlined,
  UserOutlined,
} from "@ant-design/icons";
import { Avatar, Badge, Button, Drawer, Input, Select, Segmented, Space, Tooltip, Typography } from "antd";
import { useMemo, useState, type ReactNode } from "react";
import type { Dictionary, Language } from "../i18n";
import { adminLayoutModes, themeNames, type AdminLayoutMode, type ThemeName } from "../theme";
import type { AdminResourceDefinition } from "../resources/registry";

type AdminShellProps = {
  resources: AdminResourceDefinition[];
  language: Language;
  dictionary: Dictionary;
  themeName: ThemeName;
  layoutMode: AdminLayoutMode;
  activeRoute: string;
  onLanguageChange: (language: Language) => void;
  onThemeChange: (theme: ThemeName) => void;
  onLayoutModeChange: (mode: AdminLayoutMode) => void;
  onRouteChange: (route: string) => void;
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
} as const;

const groupOrder: Array<AdminResourceDefinition["group"]> = ["foundation", "governance", "operations", "security"];

export function AdminShell({
  resources,
  language,
  dictionary,
  themeName,
  layoutMode,
  activeRoute,
  onLanguageChange,
  onThemeChange,
  onLayoutModeChange,
  onRouteChange,
  children,
}: AdminShellProps) {
  const [mobileNavOpen, setMobileNavOpen] = useState(false);
  const activeResource = resources.find((resource) => resource.route === activeRoute) ?? resources[0];
  const groupedResources = useMemo(
    () =>
      groupOrder.map((group) => ({
        group,
        label: dictionary[group],
        resources: resources.filter((resource) => resource.group === group),
      })),
    [dictionary, resources],
  );

  const shellClass = `platform-shell layout-${layoutMode}`;

  return (
    <div className={shellClass} data-theme={themeName} data-layout={layoutMode}>
      <aside className="platform-sider" aria-label="Primary navigation">
        <Brand dictionary={dictionary} compact={layoutMode === "split"} />
        {layoutMode === "split" ? (
          <SplitPrimaryNav groupedResources={groupedResources} activeRoute={activeRoute} onRouteChange={onRouteChange} />
        ) : (
          <SideNavigation
            groupedResources={groupedResources}
            activeRoute={activeRoute}
            language={language}
            onRouteChange={onRouteChange}
          />
        )}
        <div className="platform-version">platform-go v0.1.0</div>
      </aside>

      {layoutMode === "split" ? (
        <aside className="platform-secondary-nav" aria-label="Secondary navigation">
          <Typography.Text className="secondary-nav-title">{dictionary.foundation}</Typography.Text>
          <SideNavigation
            groupedResources={groupedResources.filter((group) => group.group === "foundation")}
            activeRoute={activeRoute}
            language={language}
            onRouteChange={onRouteChange}
          />
        </aside>
      ) : null}

      <main className="platform-main">
        <header className="platform-topbar">
          <div className="topbar-left">
            <Button className="mobile-nav-button" icon={<MenuOutlined />} onClick={() => setMobileNavOpen(true)} />
            <BreadcrumbLabel dictionary={dictionary} activeTitle={activeResource?.title[language] ?? ""} />
          </div>
          <Input
            className="global-search"
            prefix={<SearchOutlined />}
            suffix={<span className="keyboard-hint">⌘ {dictionary.commandHint}</span>}
            placeholder={dictionary.topSearch}
          />
          <Space className="topbar-actions" size={8}>
            <Select
              aria-label={dictionary.theme}
              className="theme-select"
              value={themeName}
              onChange={onThemeChange}
              options={themeNames.map((name) => ({ value: name, label: themeLabel(dictionary, name) }))}
            />
            <Select
              aria-label={dictionary.language}
              className="language-select"
              value={language}
              onChange={onLanguageChange}
              options={[
                { value: "zh", label: dictionary.cn },
                { value: "en", label: dictionary.en },
              ]}
            />
            <Tooltip title={dictionary.alerts}>
              <Badge count={3} size="small">
                <Button icon={<BellOutlined />} />
              </Badge>
            </Tooltip>
            <Avatar className="admin-avatar">A</Avatar>
            <Typography.Text className="admin-name">{dictionary.admin}</Typography.Text>
          </Space>
        </header>

        <nav className="platform-resource-tabs" aria-label={dictionary.historyTabs}>
          {resources.slice(0, 8).map((resource) => (
            <button
              key={resource.route}
              className={resource.route === activeRoute ? "resource-tab active" : "resource-tab"}
              type="button"
              onClick={() => onRouteChange(resource.route)}
            >
              {resource.title[language]}
            </button>
          ))}
        </nav>

        <section className="platform-toolbar-band">
          <Segmented
            className="layout-switcher"
            value={layoutMode}
            onChange={(value) => onLayoutModeChange(value as AdminLayoutMode)}
            options={adminLayoutModes.map((mode) => ({ value: mode, label: layoutLabel(dictionary, mode) }))}
          />
          <div className="context-controls">
            <Select
              className="context-select"
              value="prod"
              options={[{ value: "prod", label: dictionary.production }]}
              aria-label={dictionary.environment}
            />
            <Select
              className="context-select"
              value="platform"
              options={[{ value: "platform", label: `${dictionary.platformTenant} (platform)` }]}
              aria-label={dictionary.tenant}
            />
          </div>
        </section>

        <section className="platform-content">{children}</section>
      </main>

      <Drawer
        title="platform-go"
        open={mobileNavOpen}
        placement="left"
        width={320}
        onClose={() => setMobileNavOpen(false)}
      >
        <SideNavigation
          groupedResources={groupedResources}
          activeRoute={activeRoute}
          language={language}
          onRouteChange={(route) => {
            onRouteChange(route);
            setMobileNavOpen(false);
          }}
        />
      </Drawer>
    </div>
  );
}

function Brand({ dictionary, compact }: { dictionary: Dictionary; compact?: boolean }) {
  return (
    <div className={compact ? "platform-brand compact" : "platform-brand"}>
      <div className="platform-logo">
        <ControlOutlined />
      </div>
      {compact ? null : (
        <div>
          <Typography.Text className="platform-title">platform-go</Typography.Text>
          <Typography.Text className="platform-subtitle">{dictionary.appSubtitle}</Typography.Text>
        </div>
      )}
    </div>
  );
}

function SideNavigation({
  groupedResources,
  activeRoute,
  language,
  onRouteChange,
}: {
  groupedResources: Array<{ group: AdminResourceDefinition["group"]; label: string; resources: AdminResourceDefinition[] }>;
  activeRoute: string;
  language: Language;
  onRouteChange: (route: string) => void;
}) {
  return (
    <div className="side-nav">
      {groupedResources.map((group) => (
        <div className="side-nav-group" key={group.group}>
          <Typography.Text className="side-nav-group-title">{group.label}</Typography.Text>
          {group.resources.map((resource) => {
            const Icon = iconMap[resource.icon as keyof typeof iconMap] ?? BookOutlined;
            return (
              <button
                className={resource.route === activeRoute ? "side-nav-item active" : "side-nav-item"}
                key={resource.route}
                type="button"
                onClick={() => onRouteChange(resource.route)}
              >
                <Icon />
                <span>{resource.title[language]}</span>
              </button>
            );
          })}
        </div>
      ))}
    </div>
  );
}

function SplitPrimaryNav({
  groupedResources,
  activeRoute,
  onRouteChange,
}: {
  groupedResources: Array<{ group: AdminResourceDefinition["group"]; label: string; resources: AdminResourceDefinition[] }>;
  activeRoute: string;
  onRouteChange: (route: string) => void;
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
            onClick={() => onRouteChange(group.resources[0]?.route ?? "/capabilities")}
          >
            <Icon />
            <span>{group.label}</span>
          </button>
        );
      })}
    </div>
  );
}

function BreadcrumbLabel({ dictionary, activeTitle }: { dictionary: Dictionary; activeTitle: string }) {
  return (
    <div className="breadcrumb-label">
      <Typography.Text>{dictionary.allSystems}</Typography.Text>
      <MenuFoldOutlined />
      <Typography.Text className="breadcrumb-current">{activeTitle}</Typography.Text>
    </div>
  );
}

function themeLabel(dictionary: Dictionary, themeName: ThemeName) {
  const labels = {
    tech: dictionary.themeTech,
    white: dictionary.themeWhite,
    black: dictionary.themeBlack,
    warm: dictionary.themeWarm,
  };
  return labels[themeName];
}

function layoutLabel(dictionary: Dictionary, mode: AdminLayoutMode) {
  const labels = {
    side: dictionary.layoutSide,
    top: dictionary.layoutTop,
    mixed: dictionary.layoutMixed,
    split: dictionary.layoutSplit,
  };
  return labels[mode];
}
