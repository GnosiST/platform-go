import {
  ApiOutlined,
  AuditOutlined,
  LoginOutlined,
  ReloadOutlined,
  WarningOutlined,
} from "@ant-design/icons";
import { Button, Empty, Space, Tag, Tooltip, Typography } from "antd";
import { useEffect, useMemo, useState, type ReactNode } from "react";
import {
  queryAdminResource,
  type AdminResourceRecord,
  type AdminResourceQueryResult,
} from "../api/client";
import type { Dictionary, Language } from "../i18n";
import type { AdminResourceDefinition } from "../resources/registry";
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

type LoggingCenterConsoleProps = {
  language: Language;
  dictionary: Dictionary;
  resources: AdminResourceDefinition[];
  onRouteChange: (route: string, mode?: "push" | "replace") => void;
};

type LoggingResourceKey = "audit" | "login" | "error" | "request";

type LoggingResourceConfig = {
  key: LoggingResourceKey;
  resource: string;
  route: string;
  icon: ReactNode;
  title: (dictionary: Dictionary) => string;
  description: (dictionary: Dictionary) => string;
};

type LoggingResourceSnapshot = {
  items: AdminResourceRecord[];
  total: number;
};

type LoggingCenterRecords = Record<LoggingResourceKey, LoggingResourceSnapshot>;
type LoggingCenterErrors = Partial<Record<LoggingResourceKey, string>>;

type LoggingCenterTimelineItem = {
  key: string;
  config: LoggingResourceConfig;
  record: AdminResourceRecord;
};

const PAGE_SIZE = 50;

const loggingResourceConfigs: LoggingResourceConfig[] = [
  {
    key: "audit",
    resource: "audit-logs",
    route: "/audit-logs",
    icon: <AuditOutlined />,
    title: (dictionary) => dictionary.loggingCenterAuditLogs,
    description: (dictionary) => dictionary.loggingCenterAuditLogsDescription,
  },
  {
    key: "login",
    resource: "login-logs",
    route: "/login-logs",
    icon: <LoginOutlined />,
    title: (dictionary) => dictionary.loggingCenterLoginLogs,
    description: (dictionary) => dictionary.loggingCenterLoginLogsDescription,
  },
  {
    key: "error",
    resource: "error-logs",
    route: "/error-logs",
    icon: <WarningOutlined />,
    title: (dictionary) => dictionary.loggingCenterErrorLogs,
    description: (dictionary) => dictionary.loggingCenterErrorLogsDescription,
  },
  {
    key: "request",
    resource: "request-logs",
    route: "/request-logs",
    icon: <ApiOutlined />,
    title: (dictionary) => dictionary.loggingCenterRequestLogs,
    description: (dictionary) => dictionary.loggingCenterRequestLogsDescription,
  },
];

export function LoggingCenterConsole({ language, dictionary, resources, onRouteChange }: LoggingCenterConsoleProps) {
  const [records, setRecords] = useState<LoggingCenterRecords>(() => emptyLoggingCenterRecords());
  const [errors, setErrors] = useState<LoggingCenterErrors>({});
  const [loading, setLoading] = useState(true);
  const resourceRoutes = useMemo(() => new Set(resources.map((resource) => resource.route)), [resources]);
  const availableConfigs = useMemo(
    () => loggingResourceConfigs.filter((config) => resourceRoutes.has(config.route)),
    [resourceRoutes],
  );
  const timelineItems = useMemo(() => latestLoggingTimelineItems(records), [records]);

  const load = async () => {
    setLoading(true);
    const nextRecords = emptyLoggingCenterRecords();
    const nextErrors: LoggingCenterErrors = {};
    await Promise.all(loggingResourceConfigs.map(async (config) => {
      if (!resourceRoutes.has(config.route)) {
        return;
      }
      try {
        nextRecords[config.key] = await loadLoggingResource(config.resource);
      } catch (error) {
        nextErrors[config.key] = error instanceof Error ? error.message : dictionary.loadResourceFailed;
      }
    }));
    setRecords(nextRecords);
    setErrors(nextErrors);
    setLoading(false);
  };

  useEffect(() => {
    void load();
  }, [resourceRoutes]);

  const errorMessages = Object.entries(errors).filter(([, message]) => Boolean(message));

  return (
    <AdminPage
      className="logging-center-console"
      title={dictionary.loggingCenterTitle}
      description={dictionary.loggingCenterDescription}
      actions={(
        <AdminActionButton icon={<ReloadOutlined />} label={dictionary.refresh} loading={loading} onClick={() => void load()}>
          {dictionary.refresh}
        </AdminActionButton>
      )}
      summary={(
        <AdminMetricStrip
          columns="repeat(5, minmax(0, 1fr))"
          items={[
            { key: "resources", label: dictionary.loggingCenterMetricResources, value: availableConfigs.length },
            { key: "audit", label: dictionary.loggingCenterMetricAudits, value: records.audit.total },
            { key: "login", label: dictionary.loggingCenterMetricLogins, value: records.login.total },
            { key: "error", label: dictionary.loggingCenterMetricErrors, value: records.error.total, tone: records.error.total > 0 ? "warning" : "default" },
            { key: "request", label: dictionary.loggingCenterMetricRequests, value: records.request.total },
          ]}
        />
      )}
    >
      {errorMessages.length > 0 ? (
        <AdminFeedback
          type="warning"
          message={dictionary.loggingCenterPartialLoadFailed}
          description={errorMessages.map(([key, message]) => `${resourceLabel(key as LoggingResourceKey, dictionary)}: ${message}`).join("; ")}
        />
      ) : null}
      <AdminFeedback
        type="info"
        message={dictionary.loggingCenterContractTitle}
        description={dictionary.loggingCenterContractDescription}
      />
      <AdminListPanel
        title={dictionary.loggingCenterResourceEntryTitle}
        toolbar={<Typography.Text type="secondary">{dictionary.loggingCenterResourceEntryDescription}</Typography.Text>}
      >
        {availableConfigs.length === 0 ? (
          <Empty description={dictionary.loggingCenterNoResources} />
        ) : (
          <div className="logging-center-resource-grid">
            {loggingResourceConfigs.map((config) => (
              <LoggingResourceCard
                config={config}
                count={records[config.key].total}
                dictionary={dictionary}
                available={resourceRoutes.has(config.route)}
                key={config.key}
                onOpen={() => onRouteChange(config.route)}
              />
            ))}
          </div>
        )}
      </AdminListPanel>
      <AdminListPanel
        className="logging-center-timeline"
        title={dictionary.loggingCenterLatestEvents}
        toolbar={<Typography.Text type="secondary">{dictionary.loggingCenterLatestEventsDescription}</Typography.Text>}
      >
        <PlatformDataTable
          columns={loggingTimelineColumns(dictionary, language, onRouteChange)}
          dataSource={timelineItems}
          rowKey="key"
          loading={loading}
          labels={tableLabels(dictionary)}
          pagination={{ pageSize: 10, total: timelineItems.length }}
          emptyState={<Empty description={dictionary.emptyData} />}
        />
      </AdminListPanel>
    </AdminPage>
  );
}

function LoggingResourceCard({
  config,
  count,
  dictionary,
  available,
  onOpen,
}: {
  config: LoggingResourceConfig;
  count: number;
  dictionary: Dictionary;
  available: boolean;
  onOpen: () => void;
}) {
  const content = (
    <>
      <span className="logging-center-resource-icon">{config.icon}</span>
      <div className="settings-center-config-cell">
        <Typography.Text strong>{config.title(dictionary)}</Typography.Text>
        <Typography.Text type="secondary">{config.description(dictionary)}</Typography.Text>
      </div>
      <Space direction="vertical" size={2} align="end">
        <Tag color={available ? "success" : "warning"}>
          {available ? dictionary.loggingCenterResourceAvailable : dictionary.loggingCenterResourceMissing}
        </Tag>
        <Tag>{dictionary.loggingCenterRecordCount.replace("{count}", String(count))}</Tag>
      </Space>
    </>
  );
  if (!available) {
    return (
      <div className="logging-center-resource-card" aria-label={config.title(dictionary)}>
        {content}
      </div>
    );
  }
  return (
    <button className="logging-center-resource-card interactive" type="button" onClick={onOpen}>
      {content}
    </button>
  );
}

async function loadLoggingResource(resource: string): Promise<LoggingResourceSnapshot> {
  const result = await queryWithCreatedAtSort(resource).catch(() => queryAdminResource(resource, {
    page: 1,
    pageSize: PAGE_SIZE,
  }));
  return {
    items: result.items,
    total: result.total,
  };
}

function queryWithCreatedAtSort(resource: string): Promise<AdminResourceQueryResult> {
  return queryAdminResource(resource, {
    page: 1,
    pageSize: PAGE_SIZE,
    sort: [{ field: "createdAt", order: "desc" }],
  });
}

function latestLoggingTimelineItems(records: LoggingCenterRecords) {
  return loggingResourceConfigs
    .flatMap((config) => records[config.key].items.map((record) => ({
      key: `${config.key}:${record.id}`,
      config,
      record,
    })))
    .sort((left, right) => timestampOf(right.record) - timestampOf(left.record))
    .slice(0, 20);
}

function loggingTimelineColumns(
  dictionary: Dictionary,
  language: Language,
  onRouteChange: (route: string, mode?: "push" | "replace") => void,
): PlatformDataTableColumn<LoggingCenterTimelineItem>[] {
  return [
    {
      title: dictionary.resource,
      key: "resource",
      width: 160,
      priority: "essential",
      render: (_value, item) => <Tag>{item.config.title(dictionary)}</Tag>,
    },
    {
      title: dictionary.loggingCenterEvent,
      key: "event",
      width: 240,
      priority: "essential",
      render: (_value, item) => <PlatformOverflowText strong value={eventSummary(item.record)} />,
    },
    {
      title: dictionary.loggingCenterActor,
      key: "actor",
      width: 180,
      priority: "standard",
      render: (_value, item) => <PlatformOverflowText value={actorSummary(item.record)} />,
    },
    {
      title: dictionary.status,
      key: "status",
      width: 130,
      priority: "standard",
      render: (_value, item) => {
        const status = statusSummary(item.record);
        return <Tag color={statusTone(status)}>{status || "-"}</Tag>;
      },
    },
    {
      title: dictionary.loggingCenterCorrelation,
      key: "correlation",
      width: 220,
      priority: "extended",
      render: (_value, item) => <PlatformOverflowText code value={correlationSummary(item.record)} />,
    },
    {
      title: dictionary.loggingCenterTime,
      key: "time",
      width: 180,
      priority: "extended",
      render: (_value, item) => formatDateTime(timeSummary(item.record), language),
    },
    {
      title: dictionary.actions,
      key: "actions",
      width: 116,
      lockVisible: true,
      render: (_value, item) => (
        <Tooltip title={dictionary.loggingCenterOpenResource}>
          <Button size="small" type="text" onClick={() => onRouteChange(item.config.route)}>
            {dictionary.viewRecord}
          </Button>
        </Tooltip>
      ),
    },
  ];
}

function eventSummary(record: AdminResourceRecord) {
  return firstValue(record, ["route", "action", "event", "errorCode", "message", "name", "code"]);
}

function actorSummary(record: AdminResourceRecord) {
  return firstValue(record, ["actor", "username", "user", "provider"]);
}

function statusSummary(record: AdminResourceRecord) {
  return firstValue(record, ["statusCode", "outcome", "result", "status", "reasonCode", "errorType"]);
}

function correlationSummary(record: AdminResourceRecord) {
  return firstValue(record, ["requestId", "traceId", "eventId", "targetId"]);
}

function timeSummary(record: AdminResourceRecord) {
  return firstValue(record, ["createdAt", "occurredAt", "timestamp", "lastAttemptAt", "updatedAt"]);
}

function firstValue(record: AdminResourceRecord, keys: string[]) {
  for (const key of keys) {
    const value = valueOf(record, key);
    if (value) {
      return value;
    }
  }
  return "-";
}

function valueOf(record: AdminResourceRecord, key: string) {
  const direct = record[key as keyof AdminResourceRecord];
  if (typeof direct === "string" && direct.trim() !== "") {
    return direct;
  }
  const nested = record.values?.[key];
  return typeof nested === "string" ? nested : "";
}

function timestampOf(record: AdminResourceRecord) {
  const value = timeSummary(record);
  const timestamp = Date.parse(value);
  return Number.isFinite(timestamp) ? timestamp : 0;
}

function formatDateTime(value: string, language: Language) {
  if (!value || value === "-") return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat(language === "zh" ? "zh-CN" : "en-US", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

function statusTone(status: string) {
  const normalized = status.toLowerCase();
  if (/(error|fail|denied|blocked|exception)/u.test(normalized)) return "error";
  if (/(warn|retry|pending)/u.test(normalized)) return "warning";
  if (/(success|ok|pass|enabled)/u.test(normalized)) return "success";
  return "default";
}

function emptyLoggingCenterRecords(): LoggingCenterRecords {
  return {
    audit: { items: [], total: 0 },
    login: { items: [], total: 0 },
    error: { items: [], total: 0 },
    request: { items: [], total: 0 },
  };
}

function resourceLabel(key: LoggingResourceKey, dictionary: Dictionary) {
  const config = loggingResourceConfigs.find((item) => item.key === key);
  return config?.title(dictionary) ?? key;
}

function tableLabels(dictionary: Dictionary) {
  return {
    search: dictionary.searchResource,
    refresh: dictionary.refresh,
    columns: dictionary.tableColumns,
    rowActions: dictionary.actions,
    selected: (count: number) => dictionary.selectedItems.replace("{count}", String(count)),
    selectRow: (key: string) => dictionary.selectRow.replace("{key}", key),
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
    activeFilters: (count: number) => dictionary.activeFilters.replace("{count}", String(count)),
    pageSize: dictionary.pageSize,
    goToPage: dictionary.goToPage,
    page: dictionary.page,
    paginationRange: dictionary.paginationRange,
    selectedColumns: (selected: number, total: number) => dictionary.selectedColumns.replace("{selected}", String(selected)).replace("{total}", String(total)),
    renderedColumns: (rendered: number, selected: number) => dictionary.renderedColumns.replace("{rendered}", String(rendered)).replace("{selected}", String(selected)),
    hiddenAtCurrentWidth: dictionary.hiddenAtCurrentWidth,
    selectAllColumns: dictionary.selectAllColumns,
    resetColumns: dictionary.resetColumns,
  };
}
