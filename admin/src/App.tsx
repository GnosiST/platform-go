import { ConfigProvider, Empty, Typography, theme as antdTheme } from "antd";
import enUS from "antd/locale/en_US";
import zhCN from "antd/locale/zh_CN";
import { useEffect, useMemo, useState } from "react";
import { listCapabilities, type CapabilityItem } from "./platform/api/client";
import { CapabilityConsole } from "./platform/capabilities/CapabilityConsole";
import { enrichCapabilities, optionalCapabilities } from "./platform/capabilities/metadata";
import { dictionaries, type Language } from "./platform/i18n";
import { coreResources } from "./platform/resources/registry";
import { AdminShell } from "./platform/shell/AdminShell";
import { themeTokens, type AdminLayoutMode, type ThemeName } from "./platform/theme";

export default function App() {
  const [capabilityItems, setCapabilityItems] = useState<CapabilityItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [language, setLanguage] = useState<Language>("zh");
  const [themeName, setThemeName] = useState<ThemeName>("tech");
  const [layoutMode, setLayoutMode] = useState<AdminLayoutMode>("mixed");
  const [activeRoute, setActiveRoute] = useState("/capabilities");

  const dictionary = dictionaries[language];
  const tokens = themeTokens[themeName];
  const capabilities = useMemo(() => enrichCapabilities(capabilityItems), [capabilityItems]);

  useEffect(() => {
    listCapabilities()
      .then((items) => {
        setCapabilityItems(items);
        setError("");
      })
      .catch((nextError: unknown) => {
        setError(nextError instanceof Error ? nextError.message : "Capability list failed to load");
      })
      .finally(() => setLoading(false));
  }, []);

  return (
    <ConfigProvider
      locale={language === "zh" ? zhCN : enUS}
      theme={{
        algorithm: themeName === "black" ? antdTheme.darkAlgorithm : antdTheme.defaultAlgorithm,
        token: {
          borderRadius: 8,
          colorPrimary: tokens.primary,
          colorSuccess: tokens.success,
          colorWarning: tokens.warning,
          colorError: tokens.error,
          colorText: tokens.text,
          colorTextSecondary: tokens.muted,
          colorBgLayout: tokens.page,
          colorBgContainer: tokens.surface,
          colorBorder: tokens.border,
          fontFamily: 'Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif',
        },
      }}
    >
      <AdminShell
        resources={coreResources}
        language={language}
        dictionary={dictionary}
        themeName={themeName}
        layoutMode={layoutMode}
        activeRoute={activeRoute}
        onLanguageChange={setLanguage}
        onThemeChange={setThemeName}
        onLayoutModeChange={setLayoutMode}
        onRouteChange={setActiveRoute}
      >
        {activeRoute === "/capabilities" ? (
          <CapabilityConsole
            capabilities={capabilities}
            optionalCapabilities={optionalCapabilities}
            language={language}
            dictionary={dictionary}
            loading={loading}
            error={error}
          />
        ) : (
          <EmptyResource route={activeRoute} language={language} />
        )}
      </AdminShell>
    </ConfigProvider>
  );
}

function EmptyResource({ route, language }: { route: string; language: Language }) {
  const resource = coreResources.find((item) => item.route === route);
  if (!resource) {
    return <Empty />;
  }
  return (
    <section className="resource-placeholder">
      <Typography.Title level={1}>{resource.title[language]}</Typography.Title>
      <Typography.Paragraph>{resource.description[language]}</Typography.Paragraph>
      <Empty description={language === "zh" ? "该资源页将在后续能力切片中接入。" : "This resource page will be connected in a later capability slice."} />
    </section>
  );
}
