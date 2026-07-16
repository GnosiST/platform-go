import {
  BgColorsOutlined,
  CheckOutlined,
  DownloadOutlined,
  EyeOutlined,
  LayoutOutlined,
  LogoutOutlined,
  ReloadOutlined,
  SettingOutlined,
  ThunderboltOutlined,
  UploadOutlined,
} from "@ant-design/icons";
import { Avatar, Button, Checkbox, ColorPicker, Divider, Drawer, InputNumber, Segmented, Slider, Space, Switch, Tabs, Tag, Typography } from "antd";
import { useMemo, type ReactNode } from "react";
import type { AdminCurrentSession, BrandingConfig } from "../api/client";
import type { Dictionary, Language } from "../i18n";
import { adminLayoutModes, themeNames, type AdminLayoutMode, type ThemeName } from "../theme";
import {
  defaultAdminUIConfig,
  normalizeAdminUIConfig,
  watermarkCounts,
  watermarkScopes,
  type AdminUIConfig,
  type WatermarkCount,
  type WatermarkScope,
} from "./adminUIConfig";

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
        onUIConfigChange(normalizeAdminUIConfig(parsed.uiConfig));
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
                <LayoutModeSelector
                  active={layoutMode}
                  dictionary={dictionary}
                  showPreviews={uiConfig.showLayoutLegend}
                  onChange={onLayoutModeChange}
                />
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
                {uiConfig.watermark ? (
                  <div className="watermark-settings-group">
                    <SettingRow label={dictionary.watermarkScopes} hint={dictionary.watermarkScopesDescription}>
                      <div aria-label={dictionary.watermarkScopes} role="group">
                        <Checkbox.Group
                          className="watermark-scope-options"
                          options={watermarkScopes.map((scope) => ({
                            label: scope === "screen" ? dictionary.watermarkScopeScreen : dictionary.watermarkScopeExport,
                            value: scope,
                          }))}
                          value={uiConfig.watermarkScopes}
                          onChange={(values) => updateConfig({ watermarkScopes: values as WatermarkScope[] })}
                        />
                      </div>
                    </SettingRow>
                    {uiConfig.watermarkScopes.includes("screen") ? (
                      <SettingRow label={dictionary.watermarkCount} hint={dictionary.watermarkCountDescription}>
                        <Segmented
                          aria-label={dictionary.watermarkCount}
                          className="watermark-count-control"
                          options={watermarkCounts}
                          value={uiConfig.watermarkCount}
                          onChange={(value) => updateConfig({ watermarkCount: value as WatermarkCount })}
                        />
                      </SettingRow>
                    ) : null}
                    <Typography.Text className="watermark-format-note" type="secondary">
                      {dictionary.watermarkExportFormatNote}
                    </Typography.Text>
                  </div>
                ) : null}
              </div>
            ),
          },
        ]}
      />
    </Drawer>
  );
}

function LayoutModeSelector({
  dictionary,
  active,
  showPreviews,
  onChange,
}: {
  dictionary: Dictionary;
  active: AdminLayoutMode;
  showPreviews: boolean;
  onChange: (mode: AdminLayoutMode) => void;
}) {
  return (
    <section className="layout-mode-selector" aria-labelledby="layout-mode-selector-title">
      <header className="layout-mode-selector-header">
        <Typography.Title id="layout-mode-selector-title" level={5}>{dictionary.layoutMode}</Typography.Title>
        <Typography.Text type="secondary">{dictionary.layoutModeDescription}</Typography.Text>
      </header>
      <div className={showPreviews ? "layout-mode-options" : "layout-mode-options no-previews"}>
        {adminLayoutModes.map((mode) => (
          <button
            aria-pressed={active === mode}
            className={active === mode ? `layout-mode-option ${mode} active` : `layout-mode-option ${mode}`}
            key={mode}
            type="button"
            onClick={() => onChange(mode)}
          >
            {showPreviews ? (
              <span className="layout-mode-preview" aria-hidden="true">
                <i />
                <b />
                <em />
              </span>
            ) : null}
            <span className="layout-mode-copy">
              <strong>{layoutLabel(dictionary, mode)}</strong>
              <span>{layoutDescription(dictionary, mode)}</span>
            </span>
            <span className="layout-mode-state" aria-hidden="true">
              {active === mode ? <CheckOutlined /> : null}
            </span>
          </button>
        ))}
      </div>
    </section>
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
      <label className="settings-switch-hit-target">
        <Switch aria-label={label} checked={checked} onChange={onChange} />
      </label>
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

function layoutDescription(dictionary: Dictionary, mode: AdminLayoutMode) {
  const descriptions = {
    side: dictionary.layoutSideDescription,
    top: dictionary.layoutTopDescription,
    mixed: dictionary.layoutMixedDescription,
    split: dictionary.layoutSplitDescription,
  };
  return descriptions[mode];
}
