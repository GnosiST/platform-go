import { ConfigProvider, theme as antdTheme, type ThemeConfig } from "antd";
import enUS from "antd/locale/en_US";
import zhCN from "antd/locale/zh_CN";
import { useLayoutEffect, useMemo, type ReactNode } from "react";
import type { Language } from "../i18n";
import { themeTokens, type ThemeName } from "../theme";

type AdminDesignProviderProps = {
  language: Language;
  themeName: ThemeName;
  customPrimary?: string;
  children: ReactNode;
};

const adminFontFamily =
  'Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif';

export function AdminDesignProvider({ language, themeName, customPrimary, children }: AdminDesignProviderProps) {
  const tokens = themeTokens[themeName];
  const colorPrimary = customPrimary?.trim() || tokens.primary;

  useLayoutEffect(() => {
    const previousTheme = document.body.dataset.theme;
    document.body.dataset.theme = themeName;

    return () => {
      if (document.body.dataset.theme !== themeName) return;
      if (previousTheme) {
        document.body.dataset.theme = previousTheme;
        return;
      }
      delete document.body.dataset.theme;
    };
  }, [themeName]);

  const theme = useMemo<ThemeConfig>(
    () => ({
      algorithm: themeName === "black" ? antdTheme.darkAlgorithm : antdTheme.defaultAlgorithm,
      token: {
        borderRadius: 8,
        colorPrimary,
        colorSuccess: tokens.success,
        colorWarning: tokens.warning,
        colorError: tokens.error,
        colorText: tokens.text,
        colorTextSecondary: tokens.muted,
        colorBgLayout: tokens.page,
        colorBgContainer: tokens.surface,
        colorBorder: tokens.border,
        controlHeight: 32,
        fontFamily: adminFontFamily,
        fontSize: 14,
      },
      components: {
        Alert: {
          borderRadiusLG: 8,
        },
        Button: {
          borderRadius: 8,
          controlHeight: 32,
          controlHeightSM: 26,
        },
        Modal: {
          borderRadiusLG: 8,
        },
        Pagination: {
          itemSizeSM: 22,
        },
        Table: {
          borderColor: tokens.border,
          cellPaddingBlock: 4,
          cellPaddingBlockMD: 4,
          cellPaddingBlockSM: 3,
          cellPaddingInline: 8,
          cellPaddingInlineMD: 8,
          cellPaddingInlineSM: 8,
          cellFontSizeSM: 12,
          headerBg: tokens.elevated,
          headerColor: tokens.muted,
          headerSplitColor: tokens.border,
          rowHoverBg: tokens.selected,
        },
      },
    }),
    [colorPrimary, themeName, tokens],
  );

  return (
    <ConfigProvider locale={language === "zh" ? zhCN : enUS} theme={theme}>
      {children}
    </ConfigProvider>
  );
}
