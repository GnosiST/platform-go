import {
  BgColorsOutlined,
  DownloadOutlined,
  EyeOutlined,
  LayoutOutlined,
  LogoutOutlined,
  ReloadOutlined,
  SettingOutlined,
  ThunderboltOutlined,
  UploadOutlined,
} from "@ant-design/icons";
import { Avatar, Button, ColorPicker, Divider, Drawer, InputNumber, Segmented, Slider, Space, Switch, Tabs, Tag, Typography } from "antd";
import { useMemo, type ReactNode } from "react";
import type { AdminCurrentSession, BrandingConfig } from "../api/client";
import type { Dictionary, Language } from "../i18n";
import { adminLayoutModes, themeNames, type AdminLayoutMode, type ThemeName } from "../theme";

export type AdminUIConfig = {
  density: "compact" | "comfortable";
  showWorkTabs: boolean;
  pageTransition: boolean;
  sidebarCollapsed: boolean;
  showLayoutLegend: boolean;
  watermark: boolean;
  visualAid: boolean;
  sidebarWidth: number;
  menuItemHeight: number;
  customPrimary: string;
};

export const defaultAdminUIConfig: AdminUIConfig = {
  density: "compact",
  showWorkTabs: true,
  pageTransition: true,
  sidebarCollapsed: false,
  showLayoutLegend: true,
  watermark: false,
  visualAid: false,
  sidebarWidth: 248,
  menuItemHeight: 40,
  customPrimary: "#1d63ed",
};

type SystemSettingsDrawerProps = {
  open: boolean;
  language: Language;
  dictionary: Dictionary;
  themeName: ThemeName;
  layoutMode: AdminLayoutMode;
  uiConfig: AdminUIConfig;
  branding: BrandingConfig | null;
  session: AdminCurrentSession;
  onClose: () => void;
  onThemeChange: (theme: ThemeName) => void;
  onLayoutModeChange: (mode: AdminLayoutMode) => void;
  onUIConfigChange: (config: AdminUIConfig) => void;
  onLogout: () => void;
};

export function SystemSettingsDrawer({
  open,
  language,
  dictionary,
  themeName,
  layoutMode,
  uiConfig,
  branding,
  session,
  onClose,
  onThemeChange,
  onLayoutModeChange,
  onUIConfigChange,
  onLogout,
}: SystemSettingsDrawerProps) {
  const displayName = session.user.name || session.user.username || dictionary.admin;
  const avatarLetter = (displayName.trim()[0] || "A").toUpperCase();
  const layoutOptions = useMemo(
    () => adminLayoutModes.map((mode) => ({ label: layoutLabel(dictionary, mode), value: mode })),
    [dictionary],
  );
  const densityOptions = useMemo(
    () => [
      { label: dictionary.densityCompact, value: "compact" },
      { label: dictionary.densityComfortable, value: "comfortable" },
    ],
    [dictionary],
  );

  const updateConfig = (patch: Partial<AdminUIConfig>) => onUIConfigChange({ ...uiConfig, ...patch });
  const exportConfig = () => {
    const blob = new Blob([JSON.stringify({ themeName, layoutMode, uiConfig }, null, 2)], {
      type: "application/json",
    });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = "platform-admin-ui-config.json";
    link.click();
    URL.revokeObjectURL(url);
  };
  const importConfig = () => {
    const raw = window.prompt(dictionary.pasteConfigJson);
    if (!raw) {
      return;
    }
    try {
      const parsed = JSON.parse(raw) as Partial<{
        themeName: ThemeName;
        layoutMode: AdminLayoutMode;
        uiConfig: AdminUIConfig;
      }>;
      if (parsed.themeName && themeNames.includes(parsed.themeName)) {
        onThemeChange(parsed.themeName);
      }
      if (parsed.layoutMode && adminLayoutModes.includes(parsed.layoutMode)) {
        onLayoutModeChange(parsed.layoutMode);
      }
      if (parsed.uiConfig) {
        onUIConfigChange({ ...defaultAdminUIConfig, ...parsed.uiConfig });
      }
    } catch {
      window.alert(dictionary.invalidConfigJson);
    }
  };
  const resetConfig = () => {
    onThemeChange("tech");
    onLayoutModeChange("mixed");
    onUIConfigChange(defaultAdminUIConfig);
  };

  return (
    <Drawer
      className="system-settings-drawer"
      title={
        <div className="settings-drawer-title">
          <SettingOutlined />
          <span>{dictionary.userSettings}</span>
        </div>
      }
      getContainer={false}
      open={open}
      placement="right"
      rootStyle={{ position: "absolute" }}
      width={560}
      onClose={onClose}
    >
      <div className="settings-profile-card">
        <Avatar size={40} className="admin-avatar">
          {avatarLetter}
        </Avatar>
        <div className="settings-profile-main">
          <Typography.Text strong>{displayName}</Typography.Text>
          <Typography.Text type="secondary">{branding?.productName || "platform-go"}</Typography.Text>
          <div className="settings-profile-tags">
            <Tag>{themeLabel(dictionary, themeName)}</Tag>
            <Tag>{layoutLabel(dictionary, layoutMode)}</Tag>
            <Tag>{language === "zh" ? dictionary.cn : dictionary.en}</Tag>
          </div>
        </div>
      </div>

      <div className="settings-status-grid">
        <div>
          <Typography.Text>{dictionary.theme}</Typography.Text>
          <strong>{themeLabel(dictionary, themeName)}</strong>
        </div>
        <div>
          <Typography.Text>{dictionary.layout}</Typography.Text>
          <strong>{layoutLabel(dictionary, layoutMode)}</strong>
        </div>
        <div>
          <Typography.Text>{dictionary.densitySetting}</Typography.Text>
          <strong>{uiConfig.density === "compact" ? dictionary.densityCompact : dictionary.densityComfortable}</strong>
        </div>
      </div>

      <Tabs
        className="settings-tabs"
        items={[
          {
            key: "appearance",
            label: dictionary.settingsAppearance,
            icon: <BgColorsOutlined />,
            children: (
              <div className="settings-section">
                <SettingRow label={dictionary.theme}>
                  <div className="theme-swatch-grid">
                    {themeNames.map((name) => (
                      <button
                        aria-label={themeLabel(dictionary, name)}
                        className={themeName === name ? `theme-swatch theme-${name} active` : `theme-swatch theme-${name}`}
                        key={name}
                        type="button"
                        onClick={() => onThemeChange(name)}
                      >
                        <span />
                        <strong>{themeLabel(dictionary, name)}</strong>
                      </button>
                    ))}
                  </div>
                </SettingRow>
                <SettingRow label={dictionary.customPrimary}>
                  <ColorPicker value={uiConfig.customPrimary} onChange={(_, css) => updateConfig({ customPrimary: css })} />
                </SettingRow>
                <SettingRow label={dictionary.currentColor}>
                  <Typography.Text code>{uiConfig.customPrimary}</Typography.Text>
                </SettingRow>
              </div>
            ),
          },
          {
            key: "layout",
            label: dictionary.settingsLayout,
            icon: <LayoutOutlined />,
            children: (
              <div className="settings-section">
                <SettingRow label={dictionary.layoutMode}>
                  <Segmented block options={layoutOptions} value={layoutMode} onChange={(value) => onLayoutModeChange(value as AdminLayoutMode)} />
                </SettingRow>
                <div className="settings-switch-grid">
                  <SettingSwitchCard
                    checked={uiConfig.showWorkTabs}
                    hint={dictionary.workTabsDescription}
                    icon={<LayoutOutlined />}
                    label={dictionary.workTabsSetting}
                    onChange={(checked) => updateConfig({ showWorkTabs: checked })}
                  />
                  <SettingSwitchCard
                    checked={uiConfig.sidebarCollapsed}
                    icon={<LayoutOutlined />}
                    label={dictionary.sidebarCollapsed}
                    onChange={(checked) => updateConfig({ sidebarCollapsed: checked })}
                  />
                  <SettingSwitchCard
                    checked={uiConfig.showLayoutLegend}
                    hint={dictionary.showLayoutLegendDescription}
                    icon={<LayoutOutlined />}
                    label={dictionary.showLayoutLegend}
                    onChange={(checked) => updateConfig({ showLayoutLegend: checked })}
                  />
                </div>
                {uiConfig.showLayoutLegend ? <LayoutLegend dictionary={dictionary} active={layoutMode} onChange={onLayoutModeChange} /> : null}
                <SettingRow label={dictionary.densitySetting}>
                  <Segmented block options={densityOptions} value={uiConfig.density} onChange={(value) => updateConfig({ density: value as AdminUIConfig["density"] })} />
                </SettingRow>
                <SettingRow label={dictionary.sidebarWidth}>
                  <Slider min={220} max={304} value={uiConfig.sidebarWidth} onChange={(value) => updateConfig({ sidebarWidth: value })} />
                </SettingRow>
                <SettingRow label={dictionary.menuItemHeight}>
                  <InputNumber
                    aria-label={dictionary.menuItemHeight}
                    min={34}
                    max={48}
                    name="menuItemHeight"
                    value={uiConfig.menuItemHeight}
                    onChange={(value) => updateConfig({ menuItemHeight: value ?? defaultAdminUIConfig.menuItemHeight })}
                  />
                </SettingRow>
              </div>
            ),
          },
          {
            key: "general",
            label: dictionary.settingsGeneral,
            icon: <SettingOutlined />,
            children: (
              <div className="settings-section">
                <SettingRow label={dictionary.system}>
                  <Typography.Text>{branding?.productName || "platform-go"}</Typography.Text>
                </SettingRow>
                <SettingRow label={dictionary.version}>
                  <Typography.Text code>0.1.0</Typography.Text>
                </SettingRow>
                <Divider />
                <Space wrap>
                  <Button icon={<ReloadOutlined />} onClick={resetConfig}>
                    {dictionary.resetConfig}
                  </Button>
                  <Button icon={<DownloadOutlined />} onClick={exportConfig}>
                    {dictionary.exportConfig}
                  </Button>
                  <Button icon={<UploadOutlined />} onClick={importConfig}>
                    {dictionary.importConfig}
                  </Button>
                </Space>
                <Divider />
                <Button danger icon={<LogoutOutlined />} onClick={onLogout}>
                  {dictionary.logout}
                </Button>
              </div>
            ),
          },
          {
            key: "assist",
            label: dictionary.settingsAccessibility,
            icon: <EyeOutlined />,
            children: (
              <div className="settings-section">
                <div className="settings-switch-grid">
                  <SettingSwitchCard
                    checked={uiConfig.pageTransition}
                    hint={dictionary.pageTransitionDescription}
                    icon={<ThunderboltOutlined />}
                    label={dictionary.pageTransition}
                    onChange={(checked) => updateConfig({ pageTransition: checked })}
                  />
                  <SettingSwitchCard
                    checked={uiConfig.watermark}
                    hint={dictionary.watermarkDescription}
                    icon={<EyeOutlined />}
                    label={dictionary.watermark}
                    onChange={(checked) => updateConfig({ watermark: checked })}
                  />
                  <SettingSwitchCard
                    checked={uiConfig.visualAid}
                    hint={dictionary.visualAidDescription}
                    icon={<EyeOutlined />}
                    label={dictionary.visualAid}
                    onChange={(checked) => updateConfig({ visualAid: checked })}
                  />
                </div>
              </div>
            ),
          },
        ]}
      />
    </Drawer>
  );
}

function LayoutLegend({
  dictionary,
  active,
  onChange,
}: {
  dictionary: Dictionary;
  active: AdminLayoutMode;
  onChange: (mode: AdminLayoutMode) => void;
}) {
  return (
    <div className="layout-legend-grid" aria-label={dictionary.layoutLegend}>
      {adminLayoutModes.map((mode) => (
        <button
          className={active === mode ? `layout-legend-card ${mode} active` : `layout-legend-card ${mode}`}
          key={mode}
          type="button"
          onClick={() => onChange(mode)}
        >
          <span className="layout-legend-visual">
            <i />
            <b />
            <em />
          </span>
          <strong>{layoutLabel(dictionary, mode)}</strong>
        </button>
      ))}
    </div>
  );
}

function SettingRow({ label, hint, children }: { label: string; hint?: string; children: ReactNode }) {
  return (
    <div className="settings-row">
      <div>
        <Typography.Text>{label}</Typography.Text>
        {hint ? <Typography.Text type="secondary">{hint}</Typography.Text> : null}
      </div>
      <div>{children}</div>
    </div>
  );
}

function SettingSwitchCard({
  icon,
  label,
  hint,
  checked,
  onChange,
}: {
  icon: ReactNode;
  label: string;
  hint?: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
}) {
  return (
    <div className={checked ? "settings-switch-card active" : "settings-switch-card"}>
      <span className="settings-switch-icon">{icon}</span>
      <div>
        <Typography.Text strong>{label}</Typography.Text>
        {hint ? <Typography.Text type="secondary">{hint}</Typography.Text> : null}
      </div>
      <Switch checked={checked} onChange={onChange} />
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
