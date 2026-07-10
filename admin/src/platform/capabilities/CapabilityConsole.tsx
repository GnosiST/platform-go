import {
  ApiOutlined,
  CloudUploadOutlined,
  CopyOutlined,
  DownloadOutlined,
  EditOutlined,
  EyeOutlined,
  FileSearchOutlined,
  PlusOutlined,
  SettingOutlined,
  StopOutlined,
} from "@ant-design/icons";
import { Button, Progress, Segmented, Space, Tag, Typography } from "antd";
import { useEffect, useMemo, useState } from "react";
import type { Dictionary, Language } from "../i18n";
import {
  AdminActionButton,
  AdminFeedback,
  AdminMetricStrip,
  AdminPage,
  PlatformDataTable,
  PlatformOverflowText,
  type PlatformDataTableColumn,
  type PlatformDataTableFilterField,
  type PlatformDataTableFilterValue,
} from "../ui";
import type { CapabilityKind, CapabilityView } from "./metadata";

type CapabilityConsoleProps = {
  capabilities: CapabilityView[];
  optionalCapabilities: CapabilityView[];
  language: Language;
  dictionary: Dictionary;
  loading: boolean;
  error: string;
};

type CapabilityFilter = "all" | CapabilityKind;

export function CapabilityConsole({
  capabilities,
  optionalCapabilities,
  language,
  dictionary,
  loading,
  error,
}: CapabilityConsoleProps) {
  const [query, setQuery] = useState("");
  const [filter, setFilter] = useState<CapabilityFilter>("all");
  const [tableFilters, setTableFilters] = useState<Record<string, PlatformDataTableFilterValue>>({});
  const [selectedID, setSelectedID] = useState("");
  const [installedOptionals, setInstalledOptionals] = useState<string[]>([]);

  const installedOptionalViews = useMemo(
    () =>
      optionalCapabilities
        .filter((capability) => installedOptionals.includes(capability.id))
        .map((capability) => ({ ...capability, kind: "plugin" as const })),
    [installedOptionals, optionalCapabilities],
  );

  const allCapabilities = useMemo(
    () => [...capabilities, ...installedOptionalViews],
    [capabilities, installedOptionalViews],
  );

  const filteredCapabilities = useMemo(() => {
    const normalizedQuery = query.trim().toLowerCase();
    return allCapabilities.filter((capability) => {
      const matchesFilter = filter === "all" || capability.kind === filter;
      const haystack = [
        capability.id,
        capability.name,
        capability.label.zh,
        capability.label.en,
        capability.domain.zh,
        capability.domain.en,
        capability.description.zh,
        capability.description.en,
      ]
        .join(" ")
        .toLowerCase();
      return matchesFilter && matchesCapabilityFilters(capability, tableFilters, language) && (!normalizedQuery || haystack.includes(normalizedQuery));
    });
  }, [allCapabilities, filter, language, query, tableFilters]);

  const selectedCapability = useMemo(() => {
    return (
      allCapabilities.find((capability) => capability.id === selectedID) ??
      filteredCapabilities[0] ??
      allCapabilities[0]
    );
  }, [allCapabilities, filteredCapabilities, selectedID]);

  useEffect(() => {
    if (!selectedID && allCapabilities[0]) {
      setSelectedID(allCapabilities[0].id);
    }
  }, [allCapabilities, selectedID]);

  const enabledCount = allCapabilities.filter((capability) => capability.kind !== "disabled").length;
  const optionalCount = optionalCapabilities.length - installedOptionals.length;
  const disabledCount = allCapabilities.filter((capability) => capability.kind === "disabled").length;
  const domainCount = new Set(allCapabilities.map((capability) => capability.domain.en)).size;
  const healthyCount = allCapabilities.filter((capability) => capability.health === "healthy").length;
  const healthPercent = Math.round((healthyCount / Math.max(allCapabilities.length, 1)) * 100);

  const filterFields = useMemo<PlatformDataTableFilterField[]>(
    () => [
      {
        key: "kind",
        label: dictionary.type,
        type: "select",
        options: [
          { value: "core", label: dictionary.core },
          { value: "plugin", label: dictionary.plugin },
          { value: "optional", label: dictionary.optionalTab },
          { value: "disabled", label: dictionary.disabledTab },
        ],
      },
      {
        key: "health",
        label: dictionary.healthStatus,
        type: "select",
        options: [
          { value: "healthy", label: dictionary.healthy },
          { value: "warning", label: dictionary.warning },
          { value: "error", label: dictionary.error },
        ],
      },
      {
        key: "domain",
        label: dictionary.domain,
        type: "select",
        options: Array.from(new Map(allCapabilities.map((capability) => [capability.domain.en, capability.domain])).values()).map((domain) => ({
          value: domain.en,
          label: domain[language],
        })),
      },
    ],
    [allCapabilities, dictionary, language],
  );

  const columns: PlatformDataTableColumn<CapabilityView>[] = [
    {
      title: dictionary.capability,
      dataIndex: "label",
      key: "label",
      width: 150,
      render: (_: unknown, record: CapabilityView) => (
        <div className="capability-name-cell">
          <PlatformOverflowText strong value={record.label[language]} />
          <PlatformOverflowText className="secondary-text" value={record.label[language === "zh" ? "en" : "zh"]} />
        </div>
      ),
    },
    {
      title: dictionary.code,
      dataIndex: "id",
      key: "id",
      width: 130,
      render: (id: string) => <PlatformOverflowText code value={id} />,
    },
    {
      title: dictionary.domain,
      dataIndex: "domain",
      key: "domain",
      width: 150,
      render: (_: unknown, record: CapabilityView) => <PlatformOverflowText value={record.domain[language]} />,
    },
    {
      title: dictionary.type,
      dataIndex: "kind",
      key: "kind",
      width: 130,
      render: (kind: CapabilityKind) => <Tag className={`kind-tag kind-${kind}`}>{kindLabel(dictionary, kind)}</Tag>,
    },
    {
      title: dictionary.status,
      dataIndex: "kind",
      key: "status",
      width: 110,
      render: (kind: CapabilityKind) => (
        <span className={`status-dot status-${kind === "disabled" ? "disabled" : "enabled"}`}>
          {kind === "disabled" ? dictionary.disabled : dictionary.enabled}
        </span>
      ),
    },
    {
      title: dictionary.version,
      dataIndex: "version",
      key: "version",
      width: 90,
      render: (version: string) => <PlatformOverflowText value={version} />,
    },
    {
      title: dictionary.actions,
      key: "actions",
      fixed: "right" as const,
      width: 100,
      render: (_: unknown, record: CapabilityView) => (
        <Space size={4}>
          <AdminActionButton
            icon={<EyeOutlined />}
            label={dictionary.openDetail}
            size="small"
            type="text"
            onClick={() => setSelectedID(record.id)}
          />
          <AdminActionButton icon={<EditOutlined />} label={dictionary.edit} size="small" type="text" />
          <AdminActionButton icon={<CopyOutlined />} label={dictionary.copy} size="small" type="text" />
        </Space>
      ),
    },
  ];

  return (
    <AdminPage
      className="capability-console"
      title={dictionary.pageTitle}
      description={dictionary.pageSubtitle}
      actions={
        <Space>
          <AdminActionButton icon={<DownloadOutlined />} label={dictionary.import}>
            {dictionary.import}
          </AdminActionButton>
          <AdminActionButton icon={<PlusOutlined />} label={dictionary.addCapability} type="primary">
            {dictionary.addCapability}
          </AdminActionButton>
        </Space>
      }
      summary={
        <div className="summary-band">
          <AdminMetricStrip
            className="summary-metrics"
            items={[
              { key: "total", label: dictionary.totalCapabilities, value: allCapabilities.length },
              { key: "enabled", label: dictionary.enabled, value: enabledCount, tone: "accent" },
              { key: "optional", label: dictionary.optional, value: optionalCount, tone: "warning" },
              { key: "disabled", label: dictionary.disabled, value: disabledCount },
              { key: "domains", label: dictionary.domains, value: domainCount },
              { key: "installed", label: dictionary.installedPlugins, value: installedOptionals.length },
            ]}
          />
          <div className="health-panel">
            <div>
              <Typography.Text strong>{dictionary.installHealth}</Typography.Text>
              <div className="health-legend">
                <span className="legend-item healthy">{dictionary.healthy}</span>
                <span className="legend-item warning">{dictionary.warning}</span>
                <span className="legend-item error">{dictionary.error}</span>
              </div>
            </div>
            <Progress percent={healthPercent} type="circle" size={82} strokeColor="var(--success)" />
          </div>
        </div>
      }
    >
      {error ? <AdminFeedback className="api-alert" type="warning" message={dictionary.apiUnavailable} description={error} /> : null}

      <div className="console-grid">
        <div className="capability-workspace-stack">
          <PlatformDataTable
            actions={
              <Segmented
                value={filter}
                onChange={(value) => setFilter(value as CapabilityFilter)}
                options={[
                  { value: "all", label: dictionary.all },
                  { value: "core", label: dictionary.core },
                  { value: "plugin", label: dictionary.plugin },
                  { value: "optional", label: dictionary.optionalTab },
                  { value: "disabled", label: dictionary.disabledTab },
                ]}
              />
            }
            className="capability-workspace"
            columns={columns}
            dataSource={filteredCapabilities}
            filterFields={filterFields}
            filterValues={tableFilters}
            labels={{
              search: dictionary.searchCapability,
              refresh: dictionary.refresh,
              columns: dictionary.tableColumns,
              rowActions: dictionary.actions,
              selected: (count) => formatTemplate(dictionary.selectedItems, { count: String(count) }),
              selectRow: (key) => formatTemplate(dictionary.selectRow, { key }),
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
              activeFilters: (count) => formatTemplate(dictionary.activeFilters, { count: String(count) }),
              pageSize: dictionary.pageSize,
              goToPage: dictionary.goToPage,
              page: dictionary.page,
              paginationRange: dictionary.paginationRange,
              visibleColumns: (visible, total) => formatTemplate(dictionary.visibleColumns, { visible: String(visible), total: String(total) }),
              selectAllColumns: dictionary.selectAllColumns,
              resetColumns: dictionary.resetColumns,
            }}
            loading={loading}
            mobileCards={(items) => (
              <div className="capability-mobile-list">
                {items.map((capability) => (
                  <button
                    className={capability.id === selectedCapability?.id ? "mobile-capability-card active" : "mobile-capability-card"}
                    key={capability.id}
                    type="button"
                    onClick={() => setSelectedID(capability.id)}
                  >
                    <span>
                      <strong>{capability.label[language]}</strong>
                      <em>{capability.id}</em>
                    </span>
                    <Tag className={`kind-tag kind-${capability.kind}`}>{kindLabel(dictionary, capability.kind)}</Tag>
                  </button>
                ))}
              </div>
            )}
            pagination={{
              defaultPageSize: 10,
              showTotal: (total, range) =>
                formatTemplate(dictionary.paginationRange, {
                  start: String(range[0]),
                  end: String(range[1]),
                  total: String(total),
                }),
            }}
            rowKey="id"
            scrollX={940}
            searchPlaceholder={dictionary.searchCapability}
            searchValue={query}
            selectedRowKey={selectedCapability?.id}
            onClearFilters={() => setTableFilters({})}
            onFilterChange={(key, value) => setTableFilters((current) => ({ ...current, [key]: value }))}
            onRowClick={(record) => setSelectedID(record.id)}
            onSearchChange={setQuery}
          />

          <section className="optional-section">
            <div className="section-title-row">
              <Typography.Title level={3}>{dictionary.optionalCapabilities}</Typography.Title>
              <Typography.Text>{dictionary.marketplaceHint}</Typography.Text>
            </div>
            <div className="optional-grid">
              {optionalCapabilities.map((capability) => {
                const installed = installedOptionals.includes(capability.id);
                return (
                  <article className="optional-card" key={capability.id}>
                    <CloudUploadOutlined />
                    <div>
                      <Typography.Text strong>{capability.label[language]}</Typography.Text>
                      <Typography.Text className="secondary-text">{capability.version}</Typography.Text>
                    </div>
                    <Button
                      size="small"
                      type={installed ? "default" : "primary"}
                      onClick={() =>
                        setInstalledOptionals((current) =>
                          current.includes(capability.id)
                            ? current.filter((id) => id !== capability.id)
                            : [...current, capability.id],
                        )
                      }
                    >
                      {installed ? dictionary.enabled : dictionary.install}
                    </Button>
                  </article>
                );
              })}
            </div>
          </section>
        </div>

        <CapabilityInspector capability={selectedCapability} dictionary={dictionary} language={language} />
      </div>
    </AdminPage>
  );
}

function CapabilityInspector({
  capability,
  dictionary,
  language,
}: {
  capability?: CapabilityView;
  dictionary: Dictionary;
  language: Language;
}) {
  if (!capability) {
    return <aside className="capability-inspector empty" />;
  }

  const dependencyChain = capability.dependencies.length > 0 ? [...capability.dependencies, capability.id] : [capability.id];

  return (
    <aside className="capability-inspector">
      <div className="inspector-header">
        <div className="inspector-icon">
          <ApiOutlined />
        </div>
        <div>
          <Typography.Title level={3}>{capability.label[language]}</Typography.Title>
          <Tag className={`kind-tag kind-${capability.kind}`}>{kindLabel(dictionary, capability.kind)}</Tag>
        </div>
      </div>

      <dl className="detail-list">
        <div>
          <dt>{dictionary.code}</dt>
          <dd>{capability.id}</dd>
        </div>
        <div>
          <dt>{dictionary.domain}</dt>
          <dd>{capability.domain[language]}</dd>
        </div>
        <div>
          <dt>{dictionary.version}</dt>
          <dd>{capability.version}</dd>
        </div>
        <div>
          <dt>{dictionary.owner}</dt>
          <dd>{capability.owner}</dd>
        </div>
        <div>
          <dt>{dictionary.description}</dt>
          <dd>{capability.description[language]}</dd>
        </div>
      </dl>

      <section className="inspector-section">
        <Typography.Text strong>{dictionary.dependencyChain}</Typography.Text>
        <div className="dependency-chain">
          {dependencyChain.map((id, index) => (
            <span className={id === capability.id ? "dependency-node current" : "dependency-node"} key={`${id}-${index}`}>
              {id}
            </span>
          ))}
        </div>
      </section>

      <section className="inspector-section">
        <div className="section-title-row compact">
          <Typography.Text strong>{dictionary.providedApis}</Typography.Text>
          <Button size="small" type="link">
            {dictionary.viewDocs}
          </Button>
        </div>
        <div className="api-list">
          {capability.apis.length > 0 ? (
            capability.apis.map((api) => (
              <div className="api-row" key={`${api.method}-${api.path}`}>
                <Tag>{api.method}</Tag>
                <span>{api.path}</span>
                <small>{api.summary[language]}</small>
              </div>
            ))
          ) : (
            <Typography.Text className="secondary-text">{dictionary.noDependency}</Typography.Text>
          )}
        </div>
      </section>

      <div className="inspector-actions">
        <Button icon={<StopOutlined />} danger>
          {dictionary.disable}
        </Button>
        <Button icon={<SettingOutlined />}>{dictionary.configure}</Button>
        <Button icon={<FileSearchOutlined />}>{dictionary.viewDocs}</Button>
      </div>
    </aside>
  );
}

function matchesCapabilityFilters(
  capability: CapabilityView,
  filters: Record<string, PlatformDataTableFilterValue>,
  language: Language,
) {
  return Object.entries(filters).every(([key, value]) => {
    if (!filterValueActive(value)) {
      return true;
    }
    if (typeof value !== "string") {
      return true;
    }
    switch (key) {
    case "kind":
      return capability.kind === value;
    case "health":
      return capability.health === value;
    case "domain":
      return capability.domain.en === value || capability.domain[language] === value;
    default:
      return true;
    }
  });
}

function filterValueActive(value: PlatformDataTableFilterValue) {
  if (typeof value === "string") {
    return value.trim() !== "";
  }
  return Boolean(value.from || value.to);
}

function kindLabel(dictionary: Dictionary, kind: CapabilityKind) {
  const labels = {
    core: dictionary.coreCapability,
    plugin: dictionary.pluginCapability,
    optional: dictionary.optionalCapability,
    disabled: dictionary.disabledCapability,
  };
  return labels[kind];
}

function formatTemplate(template: string, values: Record<string, string>) {
  return Object.entries(values).reduce((result, [key, value]) => result.replaceAll(`{${key}}`, value), template);
}
