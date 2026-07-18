import {
  ApiOutlined,
  EyeOutlined,
} from "@ant-design/icons";
import { Modal, Progress, Segmented, Space, Tag, Typography } from "antd";
import { useEffect, useMemo, useState } from "react";
import type { PluginManagementStatus } from "../api/client";
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
  pluginManagementStatus: PluginManagementStatus | null;
};

type CapabilityFilter = "all" | CapabilityKind;

export function CapabilityConsole({
  capabilities,
  optionalCapabilities,
  language,
  dictionary,
  loading,
  error,
  pluginManagementStatus,
}: CapabilityConsoleProps) {
  const [query, setQuery] = useState("");
  const [filter, setFilter] = useState<CapabilityFilter>("all");
  const [tableFilters, setTableFilters] = useState<Record<string, PlatformDataTableFilterValue>>({});
  const [selectedID, setSelectedID] = useState("");
  const [detailOpen, setDetailOpen] = useState(false);
  const allCapabilities = capabilities;
  const catalogCapabilities = useMemo(() => {
    const seen = new Set(allCapabilities.map((capability) => capability.id));
    return [
      ...allCapabilities,
      ...optionalCapabilities.filter((capability) => {
        if (seen.has(capability.id)) {
          return false;
        }
        seen.add(capability.id);
        return true;
      }),
    ];
  }, [allCapabilities, optionalCapabilities]);
  const status = useMemo(
    () => normalizePluginManagementStatus(pluginManagementStatus, allCapabilities),
    [allCapabilities, pluginManagementStatus],
  );
  const currentCapabilityIDs = useMemo(() => new Set(status.currentCapabilities), [status.currentCapabilities]);
  const desiredCapabilityIDs = useMemo(() => new Set(status.desiredCapabilities), [status.desiredCapabilities]);

  const filteredCapabilities = useMemo(() => {
    const normalizedQuery = query.trim().toLowerCase();
    return catalogCapabilities.filter((capability) => {
      const matchesFilter = matchesCapabilitySegmentFilter(capability, filter, currentCapabilityIDs);
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
  }, [catalogCapabilities, currentCapabilityIDs, filter, language, query, tableFilters]);

  const selectedCapability = useMemo(() => {
    return (
      catalogCapabilities.find((capability) => capability.id === selectedID) ??
      filteredCapabilities[0] ??
      catalogCapabilities[0]
    );
  }, [catalogCapabilities, filteredCapabilities, selectedID]);

  useEffect(() => {
    if (!selectedID && catalogCapabilities[0]) {
      setSelectedID(catalogCapabilities[0].id);
    }
  }, [catalogCapabilities, selectedID]);

  const openCapabilityDetail = (capabilityID: string) => {
    setSelectedID(capabilityID);
    setDetailOpen(true);
  };

  const enabledCount = catalogCapabilities.filter((capability) => currentCapabilityIDs.has(capability.id)).length;
  const optionalCount = catalogCapabilities.filter((capability) => capability.kind === "optional" && !currentCapabilityIDs.has(capability.id)).length;
  const disabledCount = catalogCapabilities.filter((capability) => !currentCapabilityIDs.has(capability.id) && !desiredCapabilityIDs.has(capability.id)).length;
  const domainCount = new Set(catalogCapabilities.map((capability) => capability.domain.en)).size;
  const healthyCount = catalogCapabilities.filter((capability) => capability.health === "healthy").length;
  const healthPercent = Math.round((healthyCount / Math.max(catalogCapabilities.length, 1)) * 100);
  const installedPluginCount = catalogCapabilities.filter((capability) => currentCapabilityIDs.has(capability.id) && capability.kind === "plugin").length;

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
        options: Array.from(new Map(catalogCapabilities.map((capability) => [capability.domain.en, capability.domain])).values()).map((domain) => ({
          value: domain.en,
          label: domain[language],
        })),
      },
    ],
    [catalogCapabilities, dictionary, language],
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
      title: dictionary.description,
      dataIndex: "description",
      key: "description",
      width: 260,
      render: (_: unknown, record: CapabilityView) => <PlatformOverflowText value={record.description[language]} />,
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
      dataIndex: "id",
      key: "status",
      width: 130,
      render: (_: unknown, record: CapabilityView) => capabilityRuntimeStatusTag(record, currentCapabilityIDs, desiredCapabilityIDs, dictionary),
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
            onClick={() => openCapabilityDetail(record.id)}
          />
        </Space>
      ),
    },
  ];

  return (
    <AdminPage
      className="capability-console"
      title={dictionary.pageTitle}
      description={dictionary.pageSubtitle}
      summary={
        <div className="summary-band">
          <AdminMetricStrip
            className="summary-metrics"
            items={[
              { key: "total", label: dictionary.totalCapabilities, value: catalogCapabilities.length },
              { key: "enabled", label: dictionary.enabled, value: enabledCount, tone: "accent" },
              { key: "optional", label: dictionary.optional, value: optionalCount, tone: "warning" },
              { key: "disabled", label: dictionary.disabled, value: disabledCount },
              { key: "domains", label: dictionary.domains, value: domainCount },
              { key: "installed", label: dictionary.installedPlugins, value: installedPluginCount },
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
      <PluginManagementPanel status={status} dictionary={dictionary} />

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
              selectedColumns: (selected, total) =>
                formatTemplate(dictionary.selectedColumns, { selected: String(selected), total: String(total) }),
              renderedColumns: (rendered, selected) =>
                formatTemplate(dictionary.renderedColumns, { rendered: String(rendered), selected: String(selected) }),
              hiddenAtCurrentWidth: dictionary.hiddenAtCurrentWidth,
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
                    onMouseDown={(event) => {
                      if (event.button === 0) {
                        openCapabilityDetail(capability.id);
                      }
                    }}
                    onPointerDown={(event) => {
                      if (event.button === 0) {
                        openCapabilityDetail(capability.id);
                      }
                    }}
                    onClick={() => openCapabilityDetail(capability.id)}
                  >
                    <span>
                      <strong>{capability.label[language]}</strong>
                      <em>{capability.id}</em>
                    </span>
                    <span className="mobile-capability-meta">
                      <Tag className={`kind-tag kind-${capability.kind}`}>{kindLabel(dictionary, capability.kind)}</Tag>
                      {capabilityRuntimeStatusTag(capability, currentCapabilityIDs, desiredCapabilityIDs, dictionary)}
                    </span>
                    <EyeOutlined className="mobile-detail-icon" />
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
            scrollX={1180}
            searchPlaceholder={dictionary.searchCapability}
            searchValue={query}
            selectedRowKey={selectedCapability?.id}
            onClearFilters={() => setTableFilters({})}
            onFilterChange={(key, value) => setTableFilters((current) => ({ ...current, [key]: value }))}
            onRowClick={(record) => openCapabilityDetail(record.id)}
            onSearchChange={setQuery}
          />

        </div>
      </div>
      <Modal
        centered
        className="capability-detail-modal"
        destroyOnHidden
        footer={null}
        open={detailOpen && Boolean(selectedCapability)}
        title={null}
        width={560}
        onCancel={() => setDetailOpen(false)}
      >
        <CapabilityInspector capability={selectedCapability} dictionary={dictionary} language={language} />
      </Modal>
    </AdminPage>
  );
}

function PluginManagementPanel({
  status,
  dictionary,
}: {
  status: PluginManagementStatus;
  dictionary: Dictionary;
}) {
  const restartRequired = status.restartRequiredForChanges;
  return (
    <section className={status.pendingRestart ? "plugin-management-panel pending" : "plugin-management-panel"}>
      <div className="plugin-management-header">
        <div>
          <Typography.Text strong>{dictionary.pluginManagementV1Title}</Typography.Text>
          <Typography.Paragraph>{dictionary.pluginManagementV1Description}</Typography.Paragraph>
        </div>
        <Space wrap>
          <Tag className={status.pendingRestart ? "resource-status status-recorded" : "resource-status status-healthy"}>
            {status.pendingRestart ? dictionary.restartPending : dictionary.noPendingRestart}
          </Tag>
          <Tag>{restartRequired ? dictionary.pluginV1RestartRequired : dictionary.no}</Tag>
        </Space>
      </div>

      <dl className="plugin-management-grid">
        <div>
          <dt>{dictionary.operationMode}</dt>
          <dd>{status.operationMode}</dd>
        </div>
        <div>
          <dt>{dictionary.activation}</dt>
          <dd>{status.activation}</dd>
        </div>
        <div>
          <dt>{dictionary.progressTransport}</dt>
          <dd>{status.progressTransport}</dd>
        </div>
        <div>
          <dt>{dictionary.runtimeHotInstall}</dt>
          <dd>{status.runtimeHotInstall ? dictionary.yes : dictionary.no}</dd>
        </div>
        <div>
          <dt>{dictionary.runtimeHotUninstall}</dt>
          <dd>{status.runtimeHotUninstall ? dictionary.yes : dictionary.no}</dd>
        </div>
        <div>
          <dt>{dictionary.remoteRepositoryPull}</dt>
          <dd>{status.remoteRepositoryPull ? dictionary.yes : dictionary.no}</dd>
        </div>
        <div>
          <dt>{dictionary.restartRequired}</dt>
          <dd>{restartRequired ? dictionary.yes : dictionary.no}</dd>
        </div>
        <div>
          <dt>{dictionary.pendingRestart}</dt>
          <dd>{status.pendingRestart ? dictionary.yes : dictionary.no}</dd>
        </div>
        <div>
          <dt>{dictionary.lockStatus}</dt>
          <dd>{formatLockStatus(status.lockStatus, dictionary)}</dd>
        </div>
        <div>
          <dt>{dictionary.source}</dt>
          <dd>{status.source}</dd>
        </div>
      </dl>

      <div className="plugin-capability-state">
        <PluginCapabilityList label={dictionary.currentCapabilities} ids={status.currentCapabilities} emptyText={dictionary.noDependency} />
        <PluginCapabilityList label={dictionary.desiredCapabilities} ids={status.desiredCapabilities} emptyText={dictionary.noDependency} />
      </div>
    </section>
  );
}

function PluginCapabilityList({
  label,
  ids,
  emptyText,
}: {
  label: string;
  ids: string[];
  emptyText: string;
}) {
  const visibleIDs = ids.slice(0, 18);
  return (
    <div className="plugin-capability-list">
      <Typography.Text strong>{label}</Typography.Text>
      <div className="plugin-capability-tags">
        {visibleIDs.length > 0 ? visibleIDs.map((id) => <Tag key={id}>{id}</Tag>) : <Typography.Text className="secondary-text">{emptyText}</Typography.Text>}
        {ids.length > visibleIDs.length ? <Tag>{`+${ids.length - visibleIDs.length}`}</Tag> : null}
      </div>
    </div>
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
    return <div className="capability-detail-panel empty" />;
  }

  const dependencyChain = capability.dependencies.length > 0 ? [...capability.dependencies, capability.id] : [capability.id];

  return (
    <div className="capability-detail-panel">
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
        <Typography.Text strong>{dictionary.providedApis}</Typography.Text>
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

    </div>
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

function matchesCapabilitySegmentFilter(capability: CapabilityView, filter: CapabilityFilter, currentCapabilityIDs: Set<string>) {
  switch (filter) {
  case "all":
    return true;
  case "plugin":
    return capability.kind === "plugin" || capability.kind === "optional";
  case "optional":
    return !currentCapabilityIDs.has(capability.id);
  default:
    return capability.kind === filter;
  }
}

function capabilityRuntimeStatusTag(
  capability: CapabilityView,
  currentCapabilityIDs: Set<string>,
  desiredCapabilityIDs: Set<string>,
  dictionary: Dictionary,
) {
  const enabled = currentCapabilityIDs.has(capability.id);
  const pending = !enabled && desiredCapabilityIDs.has(capability.id);
  if (pending) {
    return <Tag className="resource-status status-recorded">{dictionary.restartPending}</Tag>;
  }
  if (enabled) {
    return <span className="status-dot status-enabled">{dictionary.enabled}</span>;
  }
  return <span className="status-dot status-disabled">{dictionary.notEnabled}</span>;
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

function normalizePluginManagementStatus(status: PluginManagementStatus | null, capabilities: CapabilityView[]): PluginManagementStatus {
  const currentCapabilities = uniqueCapabilityIDs(status?.currentCapabilities, capabilities.map((capability) => capability.id));
  const desiredCapabilities = uniqueCapabilityIDs(status?.desiredCapabilities, currentCapabilities);
  const restartRequired = status ? Boolean(status.restartRequiredForChanges) : true;
  return {
    operationMode: status?.operationMode || "restart-required-desired-state",
    activation: status?.activation || "manual-restart",
    progressTransport: status?.progressTransport || "http-polling",
    runtimeHotInstall: Boolean(status?.runtimeHotInstall),
    runtimeHotUninstall: Boolean(status?.runtimeHotUninstall),
    remoteRepositoryPull: Boolean(status?.remoteRepositoryPull),
    restartRequiredForChanges: restartRequired,
    pendingRestart: Boolean(status?.pendingRestart || !sameCapabilitySet(currentCapabilities, desiredCapabilities)),
    lockStatus: normalizeLockStatus(status?.lockStatus),
    source: status?.source || "/api/capabilities",
    currentCapabilities,
    desiredCapabilities,
  };
}

function normalizeLockStatus(value: unknown): PluginManagementStatus["lockStatus"] {
  if (!value || typeof value !== "object") {
    return { configured: false, exists: false, valid: false };
  }
  const record = value as Record<string, unknown>;
  return {
    configured: Boolean(record.configured),
    path: typeof record.path === "string" ? record.path : undefined,
    exists: Boolean(record.exists),
    valid: Boolean(record.valid),
    error: typeof record.error === "string" ? record.error : undefined,
  };
}

function formatLockStatus(status: PluginManagementStatus["lockStatus"], dictionary: Dictionary) {
  if (!status.configured) {
    return dictionary.lockNotConfigured;
  }
  if (!status.exists) {
    return dictionary.lockConfiguredMissing;
  }
  if (!status.valid) {
    return status.error ? `${dictionary.lockConfiguredInvalid}: ${status.error}` : dictionary.lockConfiguredInvalid;
  }
  return dictionary.lockConfiguredValid;
}

function uniqueCapabilityIDs(values: unknown, fallback: string[]) {
  if (!Array.isArray(values)) {
    return fallback;
  }
  return Array.from(new Set(values.filter((value): value is string => typeof value === "string" && value.trim() !== "").map((value) => value.trim())));
}

function sameCapabilitySet(left: string[], right: string[]) {
  if (left.length !== right.length) {
    return false;
  }
  const rightSet = new Set(right);
  return left.every((value) => rightSet.has(value));
}
