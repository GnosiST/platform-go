import {
  ApiOutlined,
  EyeOutlined,
} from "@ant-design/icons";
import { Progress, Segmented, Space, Tag, Typography } from "antd";
import { useEffect, useMemo, useState, type ReactNode } from "react";
import type { PluginManagementStatus } from "../api/client";
import type { Dictionary, Language } from "../i18n";
import {
  AdminActionButton,
  AdminFeedback,
  AdminModal,
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

type CapabilityFilter = "all" | "enabled" | "not-enabled" | "pending-restart" | CapabilityKind;
type CapabilityRestartState = "stable" | "enable-pending" | "disable-pending";

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
      const matchesFilter = matchesCapabilitySegmentFilter(capability, filter, currentCapabilityIDs, desiredCapabilityIDs);
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
      return matchesFilter && matchesCapabilityFilters(capability, tableFilters, language, currentCapabilityIDs, desiredCapabilityIDs) && (!normalizedQuery || haystack.includes(normalizedQuery));
    });
  }, [catalogCapabilities, currentCapabilityIDs, desiredCapabilityIDs, filter, language, query, tableFilters]);

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
  const pendingRestartCount = catalogCapabilities.filter((capability) => capabilityRestartState(capability, currentCapabilityIDs, desiredCapabilityIDs) !== "stable").length;
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
        key: "status",
        label: dictionary.status,
        type: "select",
        options: [
          { value: "enabled", label: dictionary.enabled },
          { value: "pending-restart", label: dictionary.restartPending },
          { value: "not-enabled", label: dictionary.notEnabled },
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
              { key: "pending", label: dictionary.pendingRestart, value: pendingRestartCount, tone: pendingRestartCount > 0 ? "warning" : "default" },
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
      <PluginManagementPanel status={status} dictionary={dictionary} capabilities={catalogCapabilities} language={language} />

      <div className="console-grid">
        <div className="capability-workspace-stack">
          <PlatformDataTable
            actions={
              <Segmented
                value={filter}
                onChange={(value) => setFilter(value as CapabilityFilter)}
                options={[
                  { value: "all", label: dictionary.all },
                  { value: "enabled", label: dictionary.enabled },
                  { value: "pending-restart", label: dictionary.restartPending },
                  { value: "not-enabled", label: dictionary.notEnabled },
                  { value: "core", label: dictionary.core },
                  { value: "plugin", label: dictionary.plugin },
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
      <AdminModal
        centered
        className="capability-detail-modal"
        destroyOnHidden
        footer={null}
        open={detailOpen && Boolean(selectedCapability)}
        preset="detail"
        title={null}
        onCancel={() => setDetailOpen(false)}
      >
        <CapabilityInspector
          capability={selectedCapability}
          currentCapabilityIDs={currentCapabilityIDs}
          desiredCapabilityIDs={desiredCapabilityIDs}
          dictionary={dictionary}
          language={language}
        />
      </AdminModal>
    </AdminPage>
  );
}

function PluginManagementPanel({
  capabilities,
  status,
  dictionary,
  language,
}: {
  capabilities: CapabilityView[];
  status: PluginManagementStatus;
  dictionary: Dictionary;
  language: Language;
}) {
  const restartRequired = status.restartRequiredForChanges;
  const changes = pluginRestartImpactPreview(status, capabilities, language);
  const facts = [
    { label: dictionary.operationMode, value: status.operationMode },
    { label: dictionary.activation, value: status.activation },
    { label: dictionary.currentCapabilities, value: String(status.currentCapabilities.length) },
    { label: dictionary.desiredCapabilities, value: String(status.desiredCapabilities.length) },
    { label: dictionary.lockStatus, value: formatLockStatus(status.lockStatus, dictionary) },
    { label: dictionary.source, value: status.source },
  ];
  return (
    <section className={status.pendingRestart ? "plugin-management-panel pending" : "plugin-management-panel"}>
      <div className="plugin-management-copy">
        <div className="plugin-management-title-row">
          <Typography.Text strong>{dictionary.pluginManagementV1Title}</Typography.Text>
          <Tag className={status.pendingRestart ? "resource-status status-recorded" : "resource-status status-healthy"}>
            {status.pendingRestart ? dictionary.restartPending : dictionary.noPendingRestart}
          </Tag>
          <Tag>{restartRequired ? dictionary.pluginV1RestartRequired : dictionary.no}</Tag>
        </div>
        <Typography.Text type="secondary">{dictionary.pluginManagementListHint}</Typography.Text>
      </div>

      <dl className="plugin-management-facts">
        {facts.map((fact) => (
          <div key={fact.label}>
            <dt>{fact.label}</dt>
            <dd>{fact.value}</dd>
          </div>
        ))}
      </dl>
      {status.pendingRestart ? (
        <div className="plugin-restart-callout">
          <Typography.Text strong>{dictionary.pluginManualRestartRequired}</Typography.Text>
          <Typography.Text type="secondary">{dictionary.pluginManualRestartHint}</Typography.Text>
        </div>
      ) : null}
      {changes.length > 0 ? (
        <div className="plugin-restart-preview" aria-label={dictionary.pluginRestartImpactPreview}>
          <Typography.Text strong>{dictionary.pluginRestartImpactPreview}</Typography.Text>
          <div className="plugin-restart-preview-list">
            {changes.map((change) => (
              <div className="plugin-restart-preview-row" key={change.id}>
                <Tag color={change.action === "enable" ? "success" : "warning"}>
                  {change.action === "enable" ? dictionary.capabilityEnableAfterRestart : dictionary.capabilityDisableAfterRestart}
                </Tag>
                <PlatformOverflowText strong value={change.label} />
                <Typography.Text type="secondary">
                  {formatTemplate(dictionary.pluginRestartImpactCounts, {
                    menus: String(change.menuRoutes),
                    resources: String(change.adminResources),
                    configResources: String(change.configResources),
                    permissions: String(change.permissions),
                    authProviders: String(change.authProviders),
                  })}
                </Typography.Text>
              </div>
            ))}
          </div>
        </div>
      ) : null}
    </section>
  );
}

function CapabilityInspector({
  capability,
  currentCapabilityIDs,
  desiredCapabilityIDs,
  dictionary,
  language,
}: {
  capability?: CapabilityView;
  currentCapabilityIDs: Set<string>;
  desiredCapabilityIDs: Set<string>;
  dictionary: Dictionary;
  language: Language;
}) {
  if (!capability) {
    return <div className="capability-detail-panel empty" />;
  }

  const dependencyChain = capability.dependencies.length > 0 ? [...capability.dependencies, capability.id] : [capability.id];
  const configResources = capability.configResources ?? [];
  const menuRoutes = capability.menuRoutes ?? [];
  const adminResources = capability.adminResources ?? [];
  const permissions = capability.permissions ?? [];
  const serviceOperations = capability.serviceOperations ?? [];
  const authProviders = capability.authProviders ?? [];
  const restartState = capabilityRestartState(capability, currentCapabilityIDs, desiredCapabilityIDs);

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

      {restartState !== "stable" ? (
        <AdminFeedback
          type="warning"
          message={restartState === "enable-pending" ? dictionary.capabilityEnableAfterRestart : dictionary.capabilityDisableAfterRestart}
          description={dictionary.capabilityRestartActivationHint}
        />
      ) : null}

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
        <Typography.Text strong>{dictionary.capabilityInstallImpact}</Typography.Text>
        <Typography.Paragraph className="secondary-text">{dictionary.capabilityRestartActivationHint}</Typography.Paragraph>
        <div className="capability-impact-grid">
          <CapabilityImpactBlock title={dictionary.capabilityMenus} empty={menuRoutes.length === 0} emptyText={dictionary.capabilityNoContributions}>
            {menuRoutes.map((route) => (
              <div className="capability-impact-row" key={route.route}>
                <PlatformOverflowText strong value={route.title[language]} />
                <Typography.Text code>{route.route}</Typography.Text>
              </div>
            ))}
          </CapabilityImpactBlock>
          <CapabilityImpactBlock title={dictionary.capabilityConfigResources} empty={configResources.length === 0} emptyText={dictionary.capabilityNoContributions}>
            {configResources.map((resource) => (
              <div className="capability-impact-row" key={resource.resource}>
                <PlatformOverflowText strong value={resource.title[language]} />
                <Typography.Text code>{resource.route || resource.resource}</Typography.Text>
              </div>
            ))}
          </CapabilityImpactBlock>
          <CapabilityImpactBlock title={dictionary.capabilityResources} empty={adminResources.length === 0} emptyText={dictionary.capabilityNoContributions}>
            {adminResources.map((resource) => (
              <Tag key={resource.resource}>{resource.resource}</Tag>
            ))}
          </CapabilityImpactBlock>
          <CapabilityImpactBlock title={dictionary.capabilityPermissions} empty={permissions.length === 0} emptyText={dictionary.capabilityNoContributions}>
            {permissions.slice(0, 18).map((permission) => (
              <Typography.Text code key={permission}>{permission}</Typography.Text>
            ))}
            {permissions.length > 18 ? <Tag>+{permissions.length - 18}</Tag> : null}
          </CapabilityImpactBlock>
          <CapabilityImpactBlock title={dictionary.capabilityServiceOperations} empty={serviceOperations.length === 0} emptyText={dictionary.capabilityNoContributions}>
            {serviceOperations.map((operation) => (
              <Tag key={operation}>{operation}</Tag>
            ))}
          </CapabilityImpactBlock>
          <CapabilityImpactBlock title={dictionary.capabilityAuthProviders} empty={authProviders.length === 0} emptyText={dictionary.capabilityNoContributions}>
            {authProviders.map((provider) => (
              <Tag key={provider}>{provider}</Tag>
            ))}
          </CapabilityImpactBlock>
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

function CapabilityImpactBlock({
  title,
  empty,
  emptyText,
  children,
}: {
  title: string;
  empty: boolean;
  emptyText: string;
  children: ReactNode;
}) {
  return (
    <div className="capability-impact-block">
      <Typography.Text strong>{title}</Typography.Text>
      <div className="capability-impact-content">
        {empty ? <Typography.Text className="secondary-text">{emptyText}</Typography.Text> : children}
      </div>
    </div>
  );
}

function matchesCapabilityFilters(
  capability: CapabilityView,
  filters: Record<string, PlatformDataTableFilterValue>,
  language: Language,
  currentCapabilityIDs: Set<string>,
  desiredCapabilityIDs: Set<string>,
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
    case "status":
      return matchesCapabilityStatus(capability, value, currentCapabilityIDs, desiredCapabilityIDs);
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

function matchesCapabilitySegmentFilter(
  capability: CapabilityView,
  filter: CapabilityFilter,
  currentCapabilityIDs: Set<string>,
  desiredCapabilityIDs: Set<string>,
) {
  switch (filter) {
  case "all":
    return true;
  case "enabled":
  case "not-enabled":
  case "pending-restart":
    return matchesCapabilityStatus(capability, filter, currentCapabilityIDs, desiredCapabilityIDs);
  case "plugin":
    return capability.kind === "plugin" || capability.kind === "optional";
  case "optional":
    return !currentCapabilityIDs.has(capability.id);
  default:
    return capability.kind === filter;
  }
}

function matchesCapabilityStatus(
  capability: CapabilityView,
  status: string,
  currentCapabilityIDs: Set<string>,
  desiredCapabilityIDs: Set<string>,
) {
  const enabled = currentCapabilityIDs.has(capability.id);
  const desired = desiredCapabilityIDs.has(capability.id);
  const pendingRestart = enabled !== desired;
  switch (status) {
  case "enabled":
    return enabled && desired;
  case "pending-restart":
    return pendingRestart;
  case "not-enabled":
    return !enabled && !desired;
  default:
    return true;
  }
}

function capabilityRuntimeStatusTag(
  capability: CapabilityView,
  currentCapabilityIDs: Set<string>,
  desiredCapabilityIDs: Set<string>,
  dictionary: Dictionary,
) {
  const state = capabilityRestartState(capability, currentCapabilityIDs, desiredCapabilityIDs);
  if (state === "enable-pending") {
    return <Tag className="resource-status status-recorded">{dictionary.capabilityEnableAfterRestart}</Tag>;
  }
  if (state === "disable-pending") {
    return <Tag className="resource-status status-recorded">{dictionary.capabilityDisableAfterRestart}</Tag>;
  }
  if (currentCapabilityIDs.has(capability.id)) {
    return <span className="status-dot status-enabled">{dictionary.enabled}</span>;
  }
  return <span className="status-dot status-disabled">{dictionary.notEnabled}</span>;
}

function capabilityRestartState(
  capability: CapabilityView,
  currentCapabilityIDs: Set<string>,
  desiredCapabilityIDs: Set<string>,
): CapabilityRestartState {
  const enabled = currentCapabilityIDs.has(capability.id);
  const desired = desiredCapabilityIDs.has(capability.id);
  if (!enabled && desired) return "enable-pending";
  if (enabled && !desired) return "disable-pending";
  return "stable";
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

function pluginRestartImpactPreview(status: PluginManagementStatus, capabilities: CapabilityView[], language: Language) {
  const current = new Set(status.currentCapabilities);
  const desired = new Set(status.desiredCapabilities);
  const capabilityByID = new Map(capabilities.map((capability) => [capability.id, capability]));
  return restartChangeIDs(current, desired).map((id) => {
    const capability = capabilityByID.get(id);
    return {
      id,
      action: desired.has(id) ? "enable" as const : "disable" as const,
      label: capability?.label[language] ?? id,
      menuRoutes: capability?.menuRoutes?.length ?? 0,
      adminResources: capability?.adminResources?.length ?? 0,
      configResources: capability?.configResources?.length ?? 0,
      permissions: capability?.permissions?.length ?? 0,
      authProviders: capability?.authProviders?.length ?? 0,
    };
  });
}

function restartChangeIDs(current: Set<string>, desired: Set<string>) {
  return Array.from(new Set([...current, ...desired]))
    .filter((id) => current.has(id) !== desired.has(id))
    .sort();
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
