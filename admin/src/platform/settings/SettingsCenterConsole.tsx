import { ApiOutlined, BellOutlined, BgColorsOutlined, DatabaseOutlined, ReloadOutlined, SettingOutlined } from "@ant-design/icons";
import { Button, Empty, Space, Tag, Typography } from "antd";
import { useCallback, useEffect, useMemo, useState, type ReactNode } from "react";
import { queryAdminResource, type AdminResourceRecord } from "../api/client";
import type { Dictionary, Language } from "../i18n";
import {
  AdminActionButton,
  AdminFeedback,
  AdminListPanel,
  AdminMetricStrip,
  AdminPage,
  PlatformDataTable,
  PlatformOverflowText,
  type PlatformDataTableColumn,
} from "../ui";
import type { AdminResourceDefinition } from "../resources/registry";

type SettingsCenterConsoleProps = {
  language: Language;
  dictionary: Dictionary;
  resources: AdminResourceDefinition[];
  onRouteChange: (route: string, mode?: "push" | "replace") => void;
};

type SettingsResourceConfig = {
  key: string;
  route: string;
  resource: string;
  capability: string;
  icon: ReactNode;
  title: string;
  description: string;
  source: "catalog" | "manifest";
  order: number;
};

type SettingsResourceState = Record<string, AdminResourceRecord[]>;
type SettingsErrorState = Partial<Record<string, string>>;

const PAGE_SIZE = 100;

type KnownSettingsResourceConfig = {
  route: string;
  capability: string;
  icon: ReactNode;
  title: (dictionary: Dictionary) => string;
  description: (dictionary: Dictionary) => string;
};

const knownSettingsResourceCatalog: KnownSettingsResourceConfig[] = [
  {
    route: "/parameters",
    capability: "parameter",
    icon: <DatabaseOutlined />,
    title: (dictionary) => dictionary.settingsCenterParameters,
    description: (dictionary) => dictionary.settingsCenterParametersDescription,
  },
  {
    route: "/branding",
    capability: "parameter",
    icon: <BgColorsOutlined />,
    title: (dictionary) => dictionary.settingsCenterBranding,
    description: (dictionary) => dictionary.settingsCenterBrandingDescription,
  },
  {
    route: "/notification-channels",
    capability: "notification",
    icon: <BellOutlined />,
    title: (dictionary) => dictionary.settingsCenterNotificationChannels,
    description: (dictionary) => dictionary.settingsCenterNotificationChannelsDescription,
  },
  {
    route: "/notification-providers",
    capability: "notification",
    icon: <ApiOutlined />,
    title: (dictionary) => dictionary.settingsCenterNotificationProviders,
    description: (dictionary) => dictionary.settingsCenterNotificationProvidersDescription,
  },
  {
    route: "/notification-send-policies",
    capability: "notification",
    icon: <SettingOutlined />,
    title: (dictionary) => dictionary.settingsCenterNotificationPolicies,
    description: (dictionary) => dictionary.settingsCenterNotificationPoliciesDescription,
  },
];

export function SettingsCenterConsole({ language, dictionary, resources, onRouteChange }: SettingsCenterConsoleProps) {
  const [records, setRecords] = useState<SettingsResourceState>({});
  const [errors, setErrors] = useState<SettingsErrorState>({});
  const [loading, setLoading] = useState(true);
  const availableConfigs = useMemo(
    () => projectSettingsResourceConfigs(resources, dictionary, language),
    [dictionary, language, resources],
  );
  const rows = useMemo(
    () => availableConfigs.map((config) => settingsRow(config, records[config.key] ?? [])),
    [availableConfigs, records],
  );
  const metrics = useMemo(() => {
    const recordCount = availableConfigs.reduce((total, config) => total + (records[config.key]?.length ?? 0), 0);
    return {
      resources: availableConfigs.length,
      capabilities: new Set(availableConfigs.map((config) => config.capability)).size,
      records: recordCount,
      warnings: Object.values(errors).filter(Boolean).length,
    };
  }, [availableConfigs, errors, records]);

  const load = useCallback(async () => {
    setLoading(true);
    const nextRecords = emptyState(availableConfigs);
    const nextErrors: SettingsErrorState = {};
    await Promise.all(availableConfigs.map(async (config) => {
      try {
        nextRecords[config.key] = await loadAllRecords(config.resource);
      } catch (error) {
        nextErrors[config.key] = error instanceof Error ? error.message : dictionary.loadResourceFailed;
      }
    }));
    setRecords(nextRecords);
    setErrors(nextErrors);
    setLoading(false);
  }, [availableConfigs, dictionary.loadResourceFailed]);

  useEffect(() => {
    void load();
  }, [load]);

  const errorMessages = Object.entries(errors).filter(([, message]) => Boolean(message));

  return (
    <AdminPage
      className="settings-center-console"
      title={dictionary.settingsCenterTitle}
      description={dictionary.settingsCenterDescription}
      actions={(
        <AdminActionButton icon={<ReloadOutlined />} label={dictionary.refresh} loading={loading} onClick={() => void load()}>
          {dictionary.refresh}
        </AdminActionButton>
      )}
      summary={(
        <AdminMetricStrip
          columns="repeat(4, minmax(0, 1fr))"
          items={[
            { key: "resources", label: dictionary.settingsCenterMetricResources, value: metrics.resources },
            { key: "capabilities", label: dictionary.settingsCenterMetricCapabilities, value: metrics.capabilities },
            { key: "records", label: dictionary.settingsCenterMetricRecords, value: metrics.records },
            {
              key: "warnings",
              label: dictionary.settingsCenterMetricWarnings,
              value: metrics.warnings > 0 ? metrics.warnings : dictionary.healthy,
              tone: metrics.warnings > 0 ? "warning" : "default",
            },
          ]}
        />
      )}
    >
      {errorMessages.length > 0 ? (
        <AdminFeedback
          type="warning"
          message={dictionary.settingsCenterPartialLoadFailed}
          description={errorMessages.map(([key, message]) => `${settingsResourceLabel(key, availableConfigs)}: ${message}`).join("; ")}
        />
      ) : null}
      <AdminListPanel className="settings-center-map" title={dictionary.settingsCenterDynamicMap}>
        {availableConfigs.length === 0 ? (
          <Empty description={dictionary.settingsCenterNoResources} />
        ) : (
          <div className="settings-center-card-grid">
            {availableConfigs.map((config) => (
              <button className="settings-center-card" key={config.key} type="button" onClick={() => onRouteChange(config.route)}>
                <span className="settings-center-card-icon">{config.icon}</span>
                <span>
                  <Typography.Text strong>{config.title}</Typography.Text>
                  <Typography.Text type="secondary">{config.description}</Typography.Text>
                </span>
                <Tag>{records[config.key]?.length ?? 0}</Tag>
              </button>
            ))}
          </div>
        )}
      </AdminListPanel>
      <AdminListPanel
        className="settings-center-resource-table"
        title={dictionary.settingsCenterResourceList}
        toolbar={<Typography.Text type="secondary">{dictionary.settingsCenterResourceListDescription}</Typography.Text>}
      >
        <PlatformDataTable
          columns={columns(dictionary, language)}
          dataSource={rows}
          rowKey="key"
          loading={loading}
          labels={tableLabels(dictionary)}
          pagination={{ pageSize: 10, total: rows.length }}
          rowActions={(row) => (
            <Space size={6}>
              <Button size="small" type="text" onClick={() => onRouteChange(row.route)}>
                {dictionary.settingsCenterManage}
              </Button>
            </Space>
          )}
          rowActionsColumnWidth={128}
          emptyState={<Empty description={dictionary.emptyData} />}
        />
      </AdminListPanel>
    </AdminPage>
  );
}

type SettingsRow = {
  key: string;
  route: string;
  title: string;
  capability: string;
  description: string;
  records: number;
  enabled: number;
  updatedAt: string;
};

function columns(dictionary: Dictionary, language: Language): PlatformDataTableColumn<SettingsRow>[] {
  return [
    {
      title: dictionary.settingsCenterConfigItem,
      key: "title",
      dataIndex: "title",
      width: 240,
      priority: "essential",
      render: (_value, row) => (
        <div className="settings-center-config-cell">
          <PlatformOverflowText strong value={row.title} />
          <Typography.Text type="secondary">{row.description}</Typography.Text>
        </div>
      ),
    },
    {
      title: dictionary.capability,
      key: "capability",
      dataIndex: "capability",
      width: 150,
      priority: "standard",
      render: (_value, row) => <Tag>{row.capability}</Tag>,
    },
    {
      title: dictionary.records,
      key: "records",
      dataIndex: "records",
      width: 120,
      priority: "standard",
    },
    {
      title: dictionary.enabled,
      key: "enabled",
      dataIndex: "enabled",
      width: 120,
      priority: "standard",
    },
    {
      title: dictionary.updatedAt,
      key: "updatedAt",
      dataIndex: "updatedAt",
      width: 180,
      priority: language === "zh" ? "extended" : "standard",
      render: (_value, row) => <PlatformOverflowText value={row.updatedAt || "-"} />,
    },
  ];
}

function settingsRow(config: SettingsResourceConfig, records: AdminResourceRecord[]): SettingsRow {
  return {
    key: config.key,
    route: config.route,
    title: config.title,
    capability: config.capability,
    description: config.description,
    records: records.length,
    enabled: records.filter((record) => record.status === "enabled" || record.status === "active").length,
    updatedAt: records.map((record) => record.updatedAt).filter(Boolean).sort().at(-1) ?? "",
  };
}

async function loadAllRecords(resource: string) {
  const records: AdminResourceRecord[] = [];
  for (let page = 1; ; page += 1) {
    const result = await queryAdminResource(resource, {
      page,
      pageSize: PAGE_SIZE,
      sort: [{ field: "updatedAt", order: "desc" }],
    });
    records.push(...result.items);
    if (result.items.length < PAGE_SIZE || records.length >= result.total) {
      return records;
    }
  }
}

function projectSettingsResourceConfigs(resources: AdminResourceDefinition[], dictionary: Dictionary, language: Language): SettingsResourceConfig[] {
  const resourcesByRoute = new Map(resources.map((resource) => [resource.route, resource]));
  const seenRoutes = new Set<string>();
  const catalogConfigs = knownSettingsResourceCatalog.flatMap((catalog, index) => {
    const resource = resourcesByRoute.get(catalog.route);
    if (!resource) return [];
    seenRoutes.add(resource.route);
    return [{
      key: resource.route,
      route: resource.route,
      resource: resource.name,
      capability: catalog.capability,
      icon: catalog.icon,
      title: catalog.title(dictionary),
      description: catalog.description(dictionary),
      source: "catalog" as const,
      order: index * 10,
    }];
  });
  const manifestConfigs = resources
    .filter((resource) => !seenRoutes.has(resource.route) && isCapabilityConfigurationResource(resource))
    .map((resource, index) => ({
      key: resource.route,
      route: resource.route,
      resource: resource.name,
      capability: inferCapabilityLabel(resource),
      icon: iconForSettingsResource(resource),
      title: resource.title[language] || resource.title.zh || resource.title.en || resource.name,
      description: resource.description[language] || resource.description.zh || resource.description.en || "",
      source: "manifest" as const,
      order: 1000 + index,
    }));
  return [...catalogConfigs, ...manifestConfigs].sort((left, right) => left.order - right.order || left.title.localeCompare(right.title));
}

function isCapabilityConfigurationResource(resource: AdminResourceDefinition) {
  if (resource.isExternal || !resource.route.startsWith("/") || resource.route === "/settings") {
    return false;
  }
  return resource.parent === "configuration" || /(?:settings|config|parameter|provider|channel|policy|template)/iu.test(resource.name);
}

function inferCapabilityLabel(resource: AdminResourceDefinition) {
  if (resource.name.startsWith("notification-")) return "notification";
  if (resource.name === "branding" || resource.name === "parameters" || resource.name === "settings") return "parameter";
  if (resource.name.includes("dictionary") || resource.name.includes("area-code")) return "dictionary";
  const permissionResource = resource.permission.match(/^admin:([^:]+):read$/u)?.[1];
  return permissionResource || resource.parent || resource.group;
}

function iconForSettingsResource(resource: AdminResourceDefinition) {
  if (resource.name === "branding") return <BgColorsOutlined />;
  if (resource.name.includes("provider")) return <ApiOutlined />;
  if (resource.name.startsWith("notification-")) return <BellOutlined />;
  if (resource.name.includes("dictionary") || resource.name.includes("parameter") || resource.name.includes("area-code")) return <DatabaseOutlined />;
  return <SettingOutlined />;
}

function emptyState(configs: SettingsResourceConfig[]): SettingsResourceState {
  return Object.fromEntries(configs.map((config) => [config.key, []]));
}

function settingsResourceLabel(key: string, configs: SettingsResourceConfig[]) {
  return configs.find((config) => config.key === key)?.title ?? key;
}

function tableLabels(dictionary: Dictionary) {
  return {
    search: dictionary.searchResource,
    refresh: dictionary.refresh,
    columns: dictionary.tableColumns,
    rowActions: dictionary.actions,
    selected: (count: number) => formatTemplate(dictionary.selectedItems, { count: String(count) }),
    selectRow: (key: string) => formatTemplate(dictionary.selectRow, { key }),
    clearSelection: dictionary.clearSelection,
    empty: dictionary.emptyData,
    filters: dictionary.advancedFilters,
    clearFilters: dictionary.clearFilters,
    querySyntax: dictionary.querySyntax,
    querySyntaxHint: dictionary.querySyntaxHint,
    filterStartDate: dictionary.filterStartDate,
    filterEndDate: dictionary.filterEndDate,
    filterMin: dictionary.filterMin,
    filterMax: dictionary.filterMax,
    filterNoFields: dictionary.filterNoFields,
    activeFilters: (count: number) => formatTemplate(dictionary.activeFilters, { count: String(count) }),
    pageSize: dictionary.pageSize,
    goToPage: dictionary.goToPage,
    page: dictionary.page,
    paginationRange: dictionary.paginationRange,
    selectedColumns: (selected: number, total: number) =>
      formatTemplate(dictionary.selectedColumns, { selected: String(selected), total: String(total) }),
    renderedColumns: (rendered: number, selected: number) =>
      formatTemplate(dictionary.renderedColumns, { rendered: String(rendered), selected: String(selected) }),
    hiddenAtCurrentWidth: dictionary.hiddenAtCurrentWidth,
    selectAllColumns: dictionary.selectAllColumns,
    resetColumns: dictionary.resetColumns,
  };
}

function formatTemplate(template: string, values: Record<string, string>) {
  return Object.entries(values).reduce((result, [key, value]) => result.replaceAll(`{${key}}`, value), template);
}
