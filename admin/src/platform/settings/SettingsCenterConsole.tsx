import { ApiOutlined, BellOutlined, BgColorsOutlined, CheckCircleOutlined, DatabaseOutlined, LockOutlined, ReloadOutlined, SettingOutlined } from "@ant-design/icons";
import { Button, Empty, Space, Tag, Typography } from "antd";
import { useCallback, useEffect, useMemo, useState, type ReactNode } from "react";
import { getAdminSettingsRuntime, type AdminResourceRecord, type AdminSettingsResourceItem } from "../api/client";
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
  group: SettingsResourceGroup;
  icon: ReactNode;
  title: string;
  description: string;
  source: "catalog" | "manifest";
  order: number;
  records: AdminResourceRecord[];
  recordCount: number;
  enabledCount: number;
  writable: boolean;
  fieldCount: number;
  updatedAt: string;
};

type SettingsResourceGroup = "core" | "message" | "capability";
type SettingsErrorState = Partial<Record<string, string>>;

type KnownSettingsResourceConfig = {
  route: string;
  capability: string;
  group: SettingsResourceGroup;
  icon: ReactNode;
  title: (dictionary: Dictionary) => string;
  description: (dictionary: Dictionary) => string;
};

const knownSettingsResourceCatalog: KnownSettingsResourceConfig[] = [
  {
    route: "/parameters",
    capability: "parameter",
    group: "core",
    icon: <DatabaseOutlined />,
    title: (dictionary) => dictionary.settingsCenterParameters,
    description: (dictionary) => dictionary.settingsCenterParametersDescription,
  },
  {
    route: "/branding",
    capability: "parameter",
    group: "core",
    icon: <BgColorsOutlined />,
    title: (dictionary) => dictionary.settingsCenterBranding,
    description: (dictionary) => dictionary.settingsCenterBrandingDescription,
  },
  {
    route: "/dictionary-parameters",
    capability: "dictionary",
    group: "core",
    icon: <DatabaseOutlined />,
    title: (dictionary) => dictionary.settingsCenterDictionaryParameters,
    description: (dictionary) => dictionary.settingsCenterDictionaryParametersDescription,
  },
  {
    route: "/credential-auth-settings",
    capability: "credential-auth",
    group: "core",
    icon: <LockOutlined />,
    title: (dictionary) => dictionary.settingsCenterCredentialAuth,
    description: (dictionary) => dictionary.settingsCenterCredentialAuthDescription,
  },
  {
    route: "/notification-channels",
    capability: "notification",
    group: "message",
    icon: <BellOutlined />,
    title: (dictionary) => dictionary.settingsCenterNotificationChannels,
    description: (dictionary) => dictionary.settingsCenterNotificationChannelsDescription,
  },
  {
    route: "/notification-providers",
    capability: "notification",
    group: "message",
    icon: <ApiOutlined />,
    title: (dictionary) => dictionary.settingsCenterNotificationProviders,
    description: (dictionary) => dictionary.settingsCenterNotificationProvidersDescription,
  },
  {
    route: "/notification-send-policies",
    capability: "notification",
    group: "message",
    icon: <SettingOutlined />,
    title: (dictionary) => dictionary.settingsCenterNotificationPolicies,
    description: (dictionary) => dictionary.settingsCenterNotificationPoliciesDescription,
  },
  {
    route: "/notification-templates",
    capability: "notification",
    group: "message",
    icon: <BellOutlined />,
    title: (dictionary) => dictionary.settingsCenterNotificationTemplates,
    description: (dictionary) => dictionary.settingsCenterNotificationTemplatesDescription,
  },
];

export function SettingsCenterConsole({ language, dictionary, resources, onRouteChange }: SettingsCenterConsoleProps) {
  const [runtimeItems, setRuntimeItems] = useState<AdminSettingsResourceItem[]>([]);
  const [errors, setErrors] = useState<SettingsErrorState>({});
  const [loading, setLoading] = useState(true);
  const availableConfigs = useMemo(
    () => projectSettingsResourceConfigs(runtimeItems, resources, dictionary, language),
    [dictionary, language, resources, runtimeItems],
  );
  const rows = useMemo(
    () => availableConfigs.map((config) => settingsRow(config, dictionary)),
    [availableConfigs, dictionary],
  );
  const groups = useMemo(() => groupSettingsResourceConfigs(availableConfigs, dictionary), [availableConfigs, dictionary]);
  const metrics = useMemo(() => {
    const recordCount = availableConfigs.reduce((total, config) => total + config.recordCount, 0);
    return {
      resources: availableConfigs.length,
      capabilities: new Set(availableConfigs.map((config) => config.capability)).size,
      records: recordCount,
      writableResources: availableConfigs.filter((config) => config.writable).length,
      manifestResources: availableConfigs.filter((config) => config.source === "manifest").length,
      warnings: Object.values(errors).filter(Boolean).length,
    };
  }, [availableConfigs, errors]);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const runtime = await getAdminSettingsRuntime();
      setRuntimeItems(runtime.items ?? []);
      setErrors({});
    } catch (error) {
      setRuntimeItems([]);
      setErrors({ runtime: error instanceof Error ? error.message : dictionary.loadResourceFailed });
    } finally {
      setLoading(false);
    }
  }, [dictionary.loadResourceFailed]);

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
      <SettingsCenterRuntimeSummary dictionary={dictionary} metrics={metrics} />
      <AdminListPanel className="settings-center-map" title={dictionary.settingsCenterDynamicMap}>
        {availableConfigs.length === 0 ? (
          <Empty description={dictionary.settingsCenterNoResources} />
        ) : (
          <Space direction="vertical" size={14} style={{ width: "100%" }}>
            {groups.map((group) => (
              <section key={group.key} aria-label={group.title}>
                <Space direction="vertical" size={8} style={{ width: "100%" }}>
                  <Space align="baseline" size={8} wrap>
                    <Typography.Text strong>{group.title}</Typography.Text>
                    <Typography.Text type="secondary">{group.description}</Typography.Text>
                    <Tag>{group.configs.length}</Tag>
                  </Space>
                  <div className="settings-center-card-grid">
                    {group.configs.map((config) => (
                      <button className="settings-center-card" key={config.key} type="button" onClick={() => onRouteChange(config.route)}>
                        <span className="settings-center-card-icon">{config.icon}</span>
                        <span>
                          <Typography.Text strong>{config.title}</Typography.Text>
                          <Typography.Text type="secondary">{config.description}</Typography.Text>
                          <Typography.Text className="secondary-text" code>{config.resource}</Typography.Text>
                        </span>
                        <Space direction="vertical" size={2} align="end">
                          <Tag>{config.recordCount}</Tag>
                          <Tag color={config.writable ? "success" : "default"}>{config.writable ? dictionary.writable : dictionary.readOnly}</Tag>
                          <Tag color={config.source === "catalog" ? "blue" : "default"}>{sourceLabel(config.source, dictionary)}</Tag>
                        </Space>
                      </button>
                    ))}
                  </div>
                </Space>
              </section>
            ))}
          </Space>
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
  writable: string;
  source: string;
  fields: number;
  updatedAt: string;
};

function SettingsCenterRuntimeSummary({
  dictionary,
  metrics,
}: {
  dictionary: Dictionary;
  metrics: {
    resources: number;
    capabilities: number;
    records: number;
    writableResources: number;
    manifestResources: number;
  };
}) {
  return (
    <AdminListPanel
      className="settings-center-runtime-summary"
      title={dictionary.settingsCenterSystemSettings}
      toolbar={<Typography.Text type="secondary">{dictionary.settingsCenterSystemSettingsDescription}</Typography.Text>}
    >
      <div className="settings-center-runtime-grid">
        <div className="settings-center-runtime-card">
          <span className="settings-center-card-icon"><SettingOutlined /></span>
          <div>
            <Typography.Text strong>{dictionary.settingsCenterSystemEntry}</Typography.Text>
            <Typography.Text type="secondary">{dictionary.settingsCenterSystemEntryDescription}</Typography.Text>
          </div>
          <Tag color="success">/settings</Tag>
        </div>
        <div className="settings-center-runtime-card">
          <span className="settings-center-card-icon"><CheckCircleOutlined /></span>
          <div>
            <Typography.Text strong>{dictionary.settingsCenterRuntimeProjection}</Typography.Text>
            <Typography.Text type="secondary">
              {formatTemplate(dictionary.settingsCenterRuntimeProjectionDescription, {
                resources: String(metrics.resources),
                capabilities: String(metrics.capabilities),
                manifestResources: String(metrics.manifestResources),
              })}
            </Typography.Text>
          </div>
          <Tag>{formatTemplate(dictionary.settingsCenterWritableCount, { count: String(metrics.writableResources) })}</Tag>
        </div>
        <div className="settings-center-runtime-card">
          <span className="settings-center-card-icon"><BgColorsOutlined /></span>
          <div>
            <Typography.Text strong>{dictionary.interfacePreferences}</Typography.Text>
            <Typography.Text type="secondary">{dictionary.settingsCenterInterfacePreferenceBoundary}</Typography.Text>
          </div>
          <Tag color="default">{dictionary.userSettings}</Tag>
        </div>
      </div>
    </AdminListPanel>
  );
}

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
      title: dictionary.writable,
      key: "writable",
      dataIndex: "writable",
      width: 120,
      priority: "extended",
    },
    {
      title: dictionary.source,
      key: "source",
      dataIndex: "source",
      width: 120,
      priority: "extended",
      render: (_value, row) => <Tag color={row.source === dictionary.settingsCenterCatalogSource ? "blue" : "default"}>{row.source}</Tag>,
    },
    {
      title: dictionary.fields,
      key: "fields",
      dataIndex: "fields",
      width: 100,
      priority: "extended",
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

function settingsRow(config: SettingsResourceConfig, dictionary: Dictionary): SettingsRow {
  return {
    key: config.key,
    route: config.route,
    title: config.title,
    capability: config.capability,
    description: config.description,
    records: config.recordCount,
    enabled: config.enabledCount,
    writable: config.writable ? dictionary.writable : dictionary.readOnly,
    source: sourceLabel(config.source, dictionary),
    fields: config.fieldCount,
    updatedAt: config.updatedAt,
  };
}

function projectSettingsResourceConfigs(
  runtimeItems: AdminSettingsResourceItem[],
  resources: AdminResourceDefinition[],
  dictionary: Dictionary,
  language: Language,
): SettingsResourceConfig[] {
  const resourcesByName = new Map(resources.map((resource) => [resource.name, resource]));
  const catalogByRoute = new Map(knownSettingsResourceCatalog.map((catalog, index) => [catalog.route, { catalog, index }]));
  const seenRoutes = new Set<string>();
  const configs = runtimeItems.flatMap((item, index) => {
    const route = item.route || resourcesByName.get(item.resource)?.route || `/${item.resource}`;
    if (!route || seenRoutes.has(route)) return [];
    seenRoutes.add(route);
    const catalogMatch = catalogByRoute.get(route);
    const records = item.records ?? [];
    return [{
      key: route,
      route,
      resource: item.resource,
      capability: item.capabilityId,
      group: catalogMatch?.catalog.group ?? inferRuntimeSettingsResourceGroup(item),
      icon: catalogMatch?.catalog.icon ?? iconForRuntimeSettingsResource(item),
      title: catalogMatch?.catalog.title(dictionary) ?? localizedText(item.title, language, item.resource),
      description: catalogMatch?.catalog.description(dictionary) ?? localizedText(item.description, language, ""),
      source: catalogMatch ? "catalog" as const : "manifest" as const,
      order: itemOrder(item, catalogMatch?.index, index),
      records,
      recordCount: item.recordCount ?? records.length,
      enabledCount: records.filter((record) => record.status === "enabled" || record.status === "active").length,
      writable: item.writable,
      fieldCount: item.schema?.fields?.length ?? 0,
      updatedAt: records.map((record) => record.updatedAt).filter(Boolean).sort().at(-1) ?? "",
    }];
  });
  return configs.sort((left, right) => left.order - right.order || left.title.localeCompare(right.title));
}

function inferRuntimeSettingsResourceGroup(item: AdminSettingsResourceItem): SettingsResourceGroup {
  if (item.capabilityId === "notification" || item.resource.startsWith("notification-")) return "message";
  if (item.capabilityId === "parameter" || item.capabilityId === "dictionary") return "core";
  return "capability";
}

function groupSettingsResourceConfigs(configs: SettingsResourceConfig[], dictionary: Dictionary) {
  const groups: Array<{
    key: SettingsResourceGroup;
    title: string;
    description: string;
    configs: SettingsResourceConfig[];
  }> = [
    {
      key: "core",
      title: dictionary.settingsCenterCoreGroup,
      description: dictionary.settingsCenterCoreGroupDescription,
      configs: [],
    },
    {
      key: "message",
      title: dictionary.settingsCenterMessageGroup,
      description: dictionary.settingsCenterMessageGroupDescription,
      configs: [],
    },
    {
      key: "capability",
      title: dictionary.settingsCenterCapabilityGroup,
      description: dictionary.settingsCenterCapabilityGroupDescription,
      configs: [],
    },
  ];
  const byKey = new Map(groups.map((group) => [group.key, group]));
  for (const config of configs) {
    byKey.get(config.group)?.configs.push(config);
  }
  return groups.filter((group) => group.configs.length > 0);
}

function sourceLabel(source: SettingsResourceConfig["source"], dictionary: Dictionary) {
  return source === "catalog" ? dictionary.settingsCenterCatalogSource : dictionary.settingsCenterManifestSource;
}

function iconForRuntimeSettingsResource(item: AdminSettingsResourceItem) {
  if (item.resource === "branding") return <BgColorsOutlined />;
  if (item.resource.includes("provider")) return <ApiOutlined />;
  if (item.resource.startsWith("notification-")) return <BellOutlined />;
  if (item.resource.includes("dictionary") || item.resource.includes("parameter") || item.resource.includes("area-code")) return <DatabaseOutlined />;
  return <SettingOutlined />;
}

function itemOrder(item: AdminSettingsResourceItem, catalogIndex: number | undefined, fallbackIndex: number) {
  if (typeof catalogIndex === "number") return catalogIndex * 10;
  return 1000 + fallbackIndex;
}

function settingsResourceLabel(key: string, configs: SettingsResourceConfig[]) {
  return configs.find((config) => config.key === key)?.title ?? key;
}

function localizedText(value: { zh?: string; en?: string } | undefined, language: Language, fallback: string) {
  if (!value) return fallback;
  return value[language] || value.zh || value.en || fallback;
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
