export type WatermarkCount = 1 | 4 | 9 | 16;
export type WatermarkScope = "screen" | "export";

export type AdminUIConfig = {
  density: "compact" | "comfortable";
  showWorkTabs: boolean;
  pageTransition: boolean;
  sidebarCollapsed: boolean;
  showLayoutLegend: boolean;
  watermark: boolean;
  watermarkCount: WatermarkCount;
  watermarkScopes: WatermarkScope[];
  visualAid: boolean;
  sidebarWidth: number;
  menuItemHeight: number;
  customPrimary: string;
};

export const watermarkCounts: WatermarkCount[] = [1, 4, 9, 16];
export const watermarkScopes: WatermarkScope[] = ["screen", "export"];

export const defaultAdminUIConfig: AdminUIConfig = {
  density: "compact",
  showWorkTabs: true,
  pageTransition: true,
  sidebarCollapsed: false,
  showLayoutLegend: true,
  watermark: false,
  watermarkCount: 1,
  watermarkScopes: ["screen"],
  visualAid: false,
  sidebarWidth: 248,
  menuItemHeight: 40,
  customPrimary: "#1d63ed",
};

export function normalizeAdminUIConfig(value: unknown): AdminUIConfig {
  if (!value || typeof value !== "object") {
    return { ...defaultAdminUIConfig, watermarkScopes: [...defaultAdminUIConfig.watermarkScopes] };
  }
  const config = value as Partial<AdminUIConfig>;
  const hasWatermarkScopes = Array.isArray(config.watermarkScopes);
  const normalizedScopes = hasWatermarkScopes
    ? watermarkScopes.filter((scope) => config.watermarkScopes?.includes(scope))
    : [...defaultAdminUIConfig.watermarkScopes];
  return {
    density: config.density === "comfortable" ? "comfortable" : "compact",
    showWorkTabs: typeof config.showWorkTabs === "boolean" ? config.showWorkTabs : defaultAdminUIConfig.showWorkTabs,
    pageTransition: typeof config.pageTransition === "boolean" ? config.pageTransition : defaultAdminUIConfig.pageTransition,
    sidebarCollapsed: typeof config.sidebarCollapsed === "boolean" ? config.sidebarCollapsed : defaultAdminUIConfig.sidebarCollapsed,
    showLayoutLegend: typeof config.showLayoutLegend === "boolean" ? config.showLayoutLegend : defaultAdminUIConfig.showLayoutLegend,
    watermark: typeof config.watermark === "boolean" ? config.watermark : defaultAdminUIConfig.watermark,
    watermarkCount: watermarkCounts.includes(config.watermarkCount as WatermarkCount) ? config.watermarkCount as WatermarkCount : defaultAdminUIConfig.watermarkCount,
    watermarkScopes: normalizedScopes,
    visualAid: typeof config.visualAid === "boolean" ? config.visualAid : defaultAdminUIConfig.visualAid,
    sidebarWidth: clampNumber(config.sidebarWidth, 220, 304, defaultAdminUIConfig.sidebarWidth),
    menuItemHeight: clampNumber(config.menuItemHeight, 34, 48, defaultAdminUIConfig.menuItemHeight),
    customPrimary: typeof config.customPrimary === "string" && config.customPrimary.trim() ? config.customPrimary : defaultAdminUIConfig.customPrimary,
  };
}

function clampNumber(value: unknown, min: number, max: number, fallback: number) {
  if (typeof value !== "number" || !Number.isFinite(value)) {
    return fallback;
  }
  return Math.min(max, Math.max(min, value));
}
