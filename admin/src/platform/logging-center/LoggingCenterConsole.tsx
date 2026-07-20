import {
  ApiOutlined,
  AuditOutlined,
  CopyOutlined,
  EyeOutlined,
  LinkOutlined,
  LoginOutlined,
  ReloadOutlined,
  WarningOutlined,
} from "@ant-design/icons";
import { App, Button, Descriptions, Empty, Space, Tag, Tooltip, Typography } from "antd";
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
  AdminModal,
  AdminPage,
  PlatformDataTable,
  PlatformOverflowText,
  type PlatformDataTableColumn,
  type PlatformDataTableFilterField,
  type PlatformDataTableFilterValue,
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

type LoggingCorrelationKey = "requestId" | "traceId";

type LoggingCorrelationEntry = {
  key: LoggingCorrelationKey;
  label: string;
  value: string;
};

type LoggingDetailEntry = {
  key: string;
  label: string;
  value: string;
};

const PAGE_SIZE = 50;

const loggingSensitiveFieldPattern = /(?:authorization|credential|password|passwd|pwd|otp|secret|token|phone|mobile|email|mail|recipient|clientIp|ipAddress|^ip$)/iu;

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
  const { message: toast } = App.useApp();
  const [records, setRecords] = useState<LoggingCenterRecords>(() => emptyLoggingCenterRecords());
  const [errors, setErrors] = useState<LoggingCenterErrors>({});
  const [loading, setLoading] = useState(true);
  const [selectedItem, setSelectedItem] = useState<LoggingCenterTimelineItem | null>(null);
  const [searchValue, setSearchValue] = useState("");
  const [filterValues, setFilterValues] = useState<Record<string, PlatformDataTableFilterValue>>({});
  const resourceRoutes = useMemo(() => new Set(resources.map((resource) => resource.route)), [resources]);
  const availableConfigs = useMemo(
    () => loggingResourceConfigs.filter((config) => resourceRoutes.has(config.route)),
    [resourceRoutes],
  );
  const timelineItems = useMemo(() => latestLoggingTimelineItems(records), [records]);
  const filteredTimelineItems = useMemo(
    () => filterLoggingTimelineItems(timelineItems, filterValues, searchValue),
    [filterValues, searchValue, timelineItems],
  );
  const filterFields = useMemo(
    () => loggingFilterFields(availableConfigs, timelineItems, dictionary),
    [availableConfigs, dictionary, timelineItems],
  );

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

  const updateFilter = (key: string, value: PlatformDataTableFilterValue) => {
    setFilterValues((current) => ({ ...current, [key]: value }));
  };
  const copyCorrelation = async (label: string, value: string) => {
    if (!value || value === "-") {
      return;
    }
    try {
      await navigator.clipboard.writeText(value);
      toast.success(formatTemplate(dictionary.loggingCenterCorrelationCopied, { field: label }));
    } catch {
      toast.error(dictionary.loggingCenterCorrelationCopyFailed);
    }
  };
  const focusCorrelation = (key: LoggingCorrelationKey, value: string) => {
    setFilterValues((current) => ({ ...current, [key]: value }));
    setSearchValue("");
    setSelectedItem(null);
    toast.success(formatTemplate(dictionary.loggingCenterCorrelationFocused, { value }));
  };
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
        title={dictionary.loggingCenterUnifiedEvents}
        toolbar={<Typography.Text type="secondary">{dictionary.loggingCenterUnifiedEventsDescription}</Typography.Text>}
      >
        <PlatformDataTable
          columns={loggingTimelineColumns(dictionary, language, onRouteChange, setSelectedItem, copyCorrelation, focusCorrelation)}
          dataSource={filteredTimelineItems}
          rowKey="key"
          loading={loading}
          labels={tableLabels(dictionary)}
          searchValue={searchValue}
          searchPlaceholder={dictionary.loggingCenterSearchPlaceholder}
          filterFields={filterFields}
          filterValues={filterValues}
          pagination={{ pageSize: 10, total: filteredTimelineItems.length }}
          emptyState={<Empty description={dictionary.emptyData} />}
          onClearFilters={() => setFilterValues({})}
          onFilterChange={updateFilter}
          onRefresh={() => void load()}
          onRowClick={setSelectedItem}
          onSearchChange={setSearchValue}
        />
      </AdminListPanel>
      <LoggingRecordDetailModal
        item={selectedItem}
        dictionary={dictionary}
        language={language}
        onClose={() => setSelectedItem(null)}
        onCopyCorrelation={copyCorrelation}
        onFocusCorrelation={focusCorrelation}
        onOpenResource={(route) => onRouteChange(route)}
      />
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
    .sort((left, right) => timestampOf(right.record) - timestampOf(left.record));
}

function loggingTimelineColumns(
  dictionary: Dictionary,
  language: Language,
  onRouteChange: (route: string, mode?: "push" | "replace") => void,
  onOpenDetail: (item: LoggingCenterTimelineItem) => void,
  onCopyCorrelation: (label: string, value: string) => Promise<void>,
  onFocusCorrelation: (key: LoggingCorrelationKey, value: string) => void,
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
      width: 280,
      priority: "extended",
      render: (_value, item) => (
        <LoggingCorrelationActions
          dictionary={dictionary}
          record={item.record}
          onCopy={onCopyCorrelation}
          onFocus={onFocusCorrelation}
        />
      ),
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
      width: 112,
      lockVisible: true,
      render: (_value, item) => (
        <Space size={2}>
          <Tooltip title={dictionary.loggingCenterOpenDetail}>
            <Button
              aria-label={dictionary.loggingCenterOpenDetail}
              icon={<EyeOutlined />}
              size="small"
              type="text"
              onClick={(event) => {
                event.stopPropagation();
                onOpenDetail(item);
              }}
            />
          </Tooltip>
          <Tooltip title={dictionary.loggingCenterOpenResource}>
            <Button
              aria-label={dictionary.loggingCenterOpenResource}
              icon={<ApiOutlined />}
              size="small"
              type="text"
              onClick={(event) => {
                event.stopPropagation();
                onRouteChange(item.config.route);
              }}
            />
          </Tooltip>
        </Space>
      ),
    },
  ];
}

function LoggingCorrelationActions({
  record,
  dictionary,
  onCopy,
  onFocus,
}: {
  record: AdminResourceRecord;
  dictionary: Dictionary;
  onCopy: (label: string, value: string) => Promise<void>;
  onFocus: (key: LoggingCorrelationKey, value: string) => void;
}) {
  const entries = correlationEntries(record, dictionary);
  if (entries.length === 0) {
    return <Typography.Text type="secondary">-</Typography.Text>;
  }
  return (
    <Space className="logging-center-correlation-cell" size={4} wrap>
      {entries.map((entry) => (
        <span className="logging-center-correlation-chip" key={entry.key}>
          <Typography.Text className="logging-center-correlation-value" code title={entry.value}>
            {entry.value}
          </Typography.Text>
          <Tooltip title={formatTemplate(dictionary.loggingCenterCopyCorrelation, { field: entry.label })}>
            <Button
              aria-label={formatTemplate(dictionary.loggingCenterCopyCorrelation, { field: entry.label })}
              icon={<CopyOutlined />}
              size="small"
              type="text"
              onClick={(event) => {
                event.stopPropagation();
                void onCopy(entry.label, entry.value);
              }}
            />
          </Tooltip>
          <Tooltip title={formatTemplate(dictionary.loggingCenterFocusCorrelation, { field: entry.label })}>
            <Button
              aria-label={formatTemplate(dictionary.loggingCenterFocusCorrelation, { field: entry.label })}
              icon={<LinkOutlined />}
              size="small"
              type="text"
              onClick={(event) => {
                event.stopPropagation();
                onFocus(entry.key, entry.value);
              }}
            />
          </Tooltip>
        </span>
      ))}
    </Space>
  );
}

function LoggingRecordDetailModal({
  item,
  dictionary,
  language,
  onClose,
  onCopyCorrelation,
  onFocusCorrelation,
  onOpenResource,
}: {
  item: LoggingCenterTimelineItem | null;
  dictionary: Dictionary;
  language: Language;
  onClose: () => void;
  onCopyCorrelation: (label: string, value: string) => Promise<void>;
  onFocusCorrelation: (key: LoggingCorrelationKey, value: string) => void;
  onOpenResource: (route: string) => void;
}) {
  if (!item) {
    return null;
  }
  const status = statusSummary(item.record);
  const details = safeLoggingDetailEntries(item.record, dictionary);
  return (
    <AdminModal
      className="logging-center-detail-modal"
      destroyOnHidden
      footer={[
        <Button key="close" onClick={onClose}>
          {dictionary.close}
        </Button>,
        <Button key="resource" icon={<ApiOutlined />} onClick={() => onOpenResource(item.config.route)}>
          {dictionary.loggingCenterOpenResource}
        </Button>,
      ]}
      open={Boolean(item)}
      preset="detail"
      title={dictionary.loggingCenterDetailTitle}
      onCancel={onClose}
    >
      <Space className="logging-center-detail-stack" direction="vertical" size={12}>
        <div className="logging-center-detail-heading">
          <Space size={6} wrap>
            <Tag>{item.config.title(dictionary)}</Tag>
            <Tag color={statusTone(status)}>{status || "-"}</Tag>
          </Space>
          <Typography.Title level={5}>{eventSummary(item.record)}</Typography.Title>
        </div>
        <Descriptions bordered column={1} size="small">
          <Descriptions.Item label={dictionary.resource}>{item.config.title(dictionary)}</Descriptions.Item>
          <Descriptions.Item label={dictionary.loggingCenterEvent}>{eventSummary(item.record)}</Descriptions.Item>
          <Descriptions.Item label={dictionary.loggingCenterActor}>{actorSummary(item.record)}</Descriptions.Item>
          <Descriptions.Item label={dictionary.status}>{status || "-"}</Descriptions.Item>
          <Descriptions.Item label={dictionary.loggingCenterTime}>{formatDateTime(timeSummary(item.record), language)}</Descriptions.Item>
          <Descriptions.Item label={dictionary.loggingCenterCorrelation}>
            <LoggingCorrelationActions
              dictionary={dictionary}
              record={item.record}
              onCopy={onCopyCorrelation}
              onFocus={onFocusCorrelation}
            />
          </Descriptions.Item>
        </Descriptions>
        <Typography.Text strong>{dictionary.loggingCenterDetailFields}</Typography.Text>
        {details.length > 0 ? (
          <div className="logging-center-detail-fields">
            {details.map((entry) => (
              <div className="logging-center-detail-field" key={entry.key}>
                <Typography.Text type="secondary">{entry.label}</Typography.Text>
                <Typography.Text code={looksLikeIdentifier(entry.key)}>{entry.value}</Typography.Text>
              </div>
            ))}
          </div>
        ) : (
          <Empty description={dictionary.loggingCenterNoSafeDetails} image={Empty.PRESENTED_IMAGE_SIMPLE} />
        )}
      </Space>
    </AdminModal>
  );
}

function loggingFilterFields(
  availableConfigs: LoggingResourceConfig[],
  items: LoggingCenterTimelineItem[],
  dictionary: Dictionary,
): PlatformDataTableFilterField[] {
  return [
    {
      key: "resource",
      label: dictionary.resource,
      type: "select",
      options: availableConfigs.map((config) => ({ value: config.key, label: config.title(dictionary) })),
    },
    {
      key: "status",
      label: dictionary.status,
      type: "select",
      options: uniqueOptions(items.map((item) => statusSummary(item.record))),
    },
    {
      key: "actor",
      label: dictionary.loggingCenterActor,
      type: "text",
      placeholder: dictionary.loggingCenterActorFilterPlaceholder,
    },
    {
      key: "requestId",
      label: dictionary.loggingCenterRequestId,
      type: "text",
      placeholder: dictionary.loggingCenterRequestId,
    },
    {
      key: "traceId",
      label: dictionary.loggingCenterTraceId,
      type: "text",
      placeholder: dictionary.loggingCenterTraceId,
    },
  ];
}

function filterLoggingTimelineItems(
  items: LoggingCenterTimelineItem[],
  filters: Record<string, PlatformDataTableFilterValue>,
  searchValue: string,
) {
  const search = normalizeForMatch(searchValue);
  return items.filter((item) => {
    if (!matchesFilter(item.config.key, filters.resource)) return false;
    if (!matchesFilter(statusSummary(item.record), filters.status)) return false;
    if (!matchesFilter(actorSummary(item.record), filters.actor)) return false;
    if (!matchesFilter(valueOf(item.record, "requestId"), filters.requestId)) return false;
    if (!matchesFilter(valueOf(item.record, "traceId"), filters.traceId)) return false;
    return search === "" || normalizeForMatch(loggingSearchText(item)).includes(search);
  });
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

function timeSummary(record: AdminResourceRecord) {
  return firstValue(record, ["createdAt", "occurredAt", "timestamp", "lastAttemptAt", "updatedAt"]);
}

function firstValue(record: AdminResourceRecord, keys: string[]) {
  for (const key of keys) {
    const value = safeLoggingValue(key, valueOf(record, key));
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

function correlationEntries(record: AdminResourceRecord, dictionary: Dictionary): LoggingCorrelationEntry[] {
  return [
    { key: "requestId" as const, label: dictionary.loggingCenterRequestId, value: valueOf(record, "requestId").trim() },
    { key: "traceId" as const, label: dictionary.loggingCenterTraceId, value: valueOf(record, "traceId").trim() },
  ].filter((entry) => entry.value !== "");
}

function safeLoggingDetailEntries(record: AdminResourceRecord, dictionary: Dictionary): LoggingDetailEntry[] {
  const entries: LoggingDetailEntry[] = [];
  const seen = new Set<string>();
  for (const key of ["id", "code", "name", "status", "description", "updatedAt"]) {
    appendLoggingDetail(entries, seen, dictionary, key, record[key as keyof AdminResourceRecord]);
  }
  for (const [key, value] of Object.entries(record.values ?? {})) {
    appendLoggingDetail(entries, seen, dictionary, key, value);
  }
  return entries;
}

function appendLoggingDetail(
  entries: LoggingDetailEntry[],
  seen: Set<string>,
  dictionary: Dictionary,
  key: string,
  rawValue: unknown,
) {
  const value = safeLoggingValue(key, rawValue);
  if (!value || seen.has(key)) {
    return;
  }
  seen.add(key);
  entries.push({
    key,
    label: loggingFieldLabel(key, dictionary),
    value,
  });
}

function safeLoggingValue(key: string, rawValue: unknown) {
  if (loggingSensitiveFieldPattern.test(key)) {
    return "";
  }
  const value = typeof rawValue === "string" ? rawValue.trim() : "";
  if (!value) {
    return "";
  }
  return redactSensitiveValue(value);
}

function redactSensitiveValue(value: string) {
  return value
    .replace(/[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}/giu, "[redacted]")
    .replace(/((?:password|passwd|pwd|otp|phone|mobile|email)\s*[:=]\s*)[^\s,;&}]+/giu, "$1[redacted]")
    .replace(/(?:\+?86[-\s]?)?1[3-9]\d{9}/gu, "[redacted]");
}

function loggingSearchText(item: LoggingCenterTimelineItem) {
  const detailValues = [
    ...["id", "code", "name", "status", "description", "updatedAt"].map((key) =>
      safeLoggingValue(key, item.record[key as keyof AdminResourceRecord]),
    ),
    ...Object.entries(item.record.values ?? {}).map(([key, value]) => safeLoggingValue(key, value)),
  ].filter(Boolean);
  return [
    item.config.resource,
    item.config.key,
    eventSummary(item.record),
    actorSummary(item.record),
    statusSummary(item.record),
    valueOf(item.record, "requestId"),
    valueOf(item.record, "traceId"),
    ...detailValues,
  ].join(" ");
}

function loggingFieldLabel(key: string, dictionary: Dictionary) {
  const labels: Record<string, string> = {
    code: dictionary.code,
    status: dictionary.status,
    description: dictionary.description,
    updatedAt: dictionary.updatedAt,
    actor: dictionary.loggingCenterActor,
    requestId: dictionary.loggingCenterRequestId,
    traceId: dictionary.loggingCenterTraceId,
  };
  return labels[key] ?? key.replace(/([a-z])([A-Z])/g, "$1 $2");
}

function matchesFilter(value: string, filter: PlatformDataTableFilterValue | undefined) {
  if (typeof filter !== "string" || filter.trim() === "") {
    return true;
  }
  return normalizeForMatch(value).includes(normalizeForMatch(filter));
}

function normalizeForMatch(value: string) {
  return value.trim().toLowerCase();
}

function uniqueOptions(values: string[]) {
  return Array.from(new Set(values.filter((value) => value && value !== "-")))
    .sort((left, right) => left.localeCompare(right))
    .map((value) => ({ value, label: value }));
}

function looksLikeIdentifier(key: string) {
  return /(?:id|code|route|method|event)/iu.test(key);
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
  const numericStatus = Number(normalized);
  if (Number.isFinite(numericStatus)) {
    if (numericStatus >= 500) return "error";
    if (numericStatus >= 400) return "warning";
    if (numericStatus >= 200 && numericStatus < 400) return "success";
  }
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

function formatTemplate(template: string, values: Record<string, string>) {
  return Object.entries(values).reduce((result, [key, value]) => result.replaceAll(`{${key}}`, value), template);
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
