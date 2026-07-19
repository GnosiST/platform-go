import {
  BellOutlined,
  CheckCircleOutlined,
  MailOutlined,
  MessageOutlined,
  ReloadOutlined,
  SendOutlined,
  SettingOutlined,
  WechatOutlined,
} from "@ant-design/icons";
import { Button, Empty, Space, Tabs, Tag, Tooltip, Typography } from "antd";
import { useEffect, useMemo, useState, type ReactNode } from "react";
import {
  AdminAPIError,
  queryAdminResource,
  type AdminResourceRecord,
} from "../api/client";
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

type MessageCenterConsoleProps = {
  language: Language;
  dictionary: Dictionary;
  resources: AdminResourceDefinition[];
  onRouteChange: (route: string, mode?: "push" | "replace") => void;
};

type MessageCenterResourceKey =
  | "channels"
  | "providers"
  | "templates"
  | "policies"
  | "notifications"
  | "deliveries";

type MessageCenterResourceConfig = {
  key: MessageCenterResourceKey;
  resource: string;
  route: string;
  title: (dictionary: Dictionary) => string;
  description: (dictionary: Dictionary) => string;
};

type MessageCenterRecords = Record<MessageCenterResourceKey, AdminResourceRecord[]>;
type MessageCenterErrors = Partial<Record<MessageCenterResourceKey, string>>;

const PAGE_SIZE = 200;

const resourceConfigs: MessageCenterResourceConfig[] = [
  {
    key: "channels",
    resource: "notification-channels",
    route: "/notification-channels",
    title: (dictionary) => dictionary.messageCenterChannels,
    description: (dictionary) => dictionary.messageCenterChannelsDescription,
  },
  {
    key: "providers",
    resource: "notification-providers",
    route: "/notification-providers",
    title: (dictionary) => dictionary.messageCenterProviders,
    description: (dictionary) => dictionary.messageCenterProvidersDescription,
  },
  {
    key: "templates",
    resource: "notification-templates",
    route: "/notification-templates",
    title: (dictionary) => dictionary.messageCenterTemplates,
    description: (dictionary) => dictionary.messageCenterTemplatesDescription,
  },
  {
    key: "policies",
    resource: "notification-send-policies",
    route: "/notification-send-policies",
    title: (dictionary) => dictionary.messageCenterPolicies,
    description: (dictionary) => dictionary.messageCenterPoliciesDescription,
  },
  {
    key: "notifications",
    resource: "notifications",
    route: "/notifications",
    title: (dictionary) => dictionary.messageCenterNotifications,
    description: (dictionary) => dictionary.messageCenterNotificationsDescription,
  },
  {
    key: "deliveries",
    resource: "notification-deliveries",
    route: "/notification-deliveries",
    title: (dictionary) => dictionary.messageCenterDeliveries,
    description: (dictionary) => dictionary.messageCenterDeliveriesDescription,
  },
];

export function MessageCenterConsole({ language, dictionary, resources, onRouteChange }: MessageCenterConsoleProps) {
  const [records, setRecords] = useState<MessageCenterRecords>(() => emptyMessageCenterRecords());
  const [errors, setErrors] = useState<MessageCenterErrors>({});
  const [loading, setLoading] = useState(true);
  const resourceRoutes = useMemo(() => new Set(resources.map((resource) => resource.route)), [resources]);
  const availableConfigs = useMemo(
    () => resourceConfigs.filter((config) => resourceRoutes.has(config.route)),
    [resourceRoutes],
  );
  const metrics = useMemo(() => messageCenterMetrics(records), [records]);

  const load = async () => {
    setLoading(true);
    const nextRecords = emptyMessageCenterRecords();
    const nextErrors: MessageCenterErrors = {};
    await Promise.all(resourceConfigs.map(async (config) => {
      if (!resourceRoutes.has(config.route)) {
        return;
      }
      try {
        nextRecords[config.key] = await loadAllRecords(config.resource);
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

  const openResource = (config: MessageCenterResourceConfig) => onRouteChange(config.route);
  const errorMessages = Object.entries(errors).filter(([, message]) => Boolean(message));

  return (
    <AdminPage
      className="message-center-console"
      title={dictionary.messageCenterTitle}
      description={dictionary.messageCenterDescription}
      actions={(
        <AdminActionButton icon={<ReloadOutlined />} label={dictionary.refresh} loading={loading} onClick={() => void load()}>
          {dictionary.refresh}
        </AdminActionButton>
      )}
      summary={(
        <AdminMetricStrip
          columns="repeat(4, minmax(0, 1fr))"
          items={[
            { key: "channels", label: dictionary.messageCenterMetricChannels, value: metrics.channelCount },
            { key: "providers", label: dictionary.messageCenterMetricProviders, value: metrics.providerCount },
            { key: "templates", label: dictionary.messageCenterMetricTemplates, value: metrics.templateCount },
            {
              key: "deliveryHealth",
              label: dictionary.messageCenterMetricDeliveryHealth,
              value: metrics.failedDeliveryCount > 0 ? metrics.failedDeliveryCount : dictionary.healthy,
              tone: metrics.failedDeliveryCount > 0 ? "warning" : "default",
            },
          ]}
        />
      )}
    >
      {errorMessages.length > 0 ? (
        <AdminFeedback
          type="warning"
          message={dictionary.messageCenterPartialLoadFailed}
          description={errorMessages.map(([key, message]) => `${resourceLabel(key as MessageCenterResourceKey, dictionary)}: ${message}`).join("; ")}
        />
      ) : null}
      <AdminListPanel
        className="message-center-overview"
        title={dictionary.messageCenterRuntimeOverview}
        actions={(
          <Space size={8} wrap>
            <Tag color={resourceRoutes.has("/notification-channels") ? "success" : "warning"}>
              {resourceRoutes.has("/notification-channels") ? dictionary.messageCenterConfigurable : dictionary.messageCenterMissingChannels}
            </Tag>
            <Tag color={resourceRoutes.has("/notification-providers") ? "success" : "warning"}>
              {resourceRoutes.has("/notification-providers") ? dictionary.messageCenterProviderAccountsReady : dictionary.messageCenterMissingProviders}
            </Tag>
          </Space>
        )}
      >
        <div className="message-center-channel-strip">
          {channelCards(records, dictionary).map((card) => (
            <div className="message-center-channel-card" key={card.key}>
              <div className="message-center-channel-icon">{card.icon}</div>
              <div>
                <Typography.Text strong>{card.label}</Typography.Text>
                <Typography.Paragraph type="secondary">{card.description}</Typography.Paragraph>
              </div>
              <Tag color={card.ready ? "success" : "default"}>{card.ready ? dictionary.configured : dictionary.notConfigured}</Tag>
            </div>
          ))}
        </div>
      </AdminListPanel>
      {availableConfigs.length === 0 ? (
        <AdminListPanel>
          <Empty description={dictionary.messageCenterNoResources} />
        </AdminListPanel>
      ) : (
        <Tabs
          className="message-center-tabs"
          items={availableConfigs.map((config) => ({
            key: config.key,
            label: config.title(dictionary),
            children: (
              <MessageCenterTab
                config={config}
                dictionary={dictionary}
                language={language}
                loading={loading}
                records={records[config.key]}
                onOpen={() => openResource(config)}
              />
            ),
          }))}
        />
      )}
    </AdminPage>
  );
}

function MessageCenterTab({
  config,
  dictionary,
  language,
  loading,
  records,
  onOpen,
}: {
  config: MessageCenterResourceConfig;
  dictionary: Dictionary;
  language: Language;
  loading: boolean;
  records: AdminResourceRecord[];
  onOpen: () => void;
}) {
  return (
    <AdminListPanel
      title={config.title(dictionary)}
      toolbar={<Typography.Text type="secondary">{config.description(dictionary)}</Typography.Text>}
      actions={(
        <Button icon={<SettingOutlined />} onClick={onOpen}>
          {dictionary.messageCenterManageResource}
        </Button>
      )}
    >
      <PlatformDataTable
        columns={columnsFor(config.key, dictionary, language)}
        dataSource={records}
        rowKey="id"
        loading={loading}
        labels={tableLabels(dictionary)}
        pagination={{ pageSize: 10, total: records.length }}
        rowActions={(record) => (
          <Space size={6}>
            <Tooltip title={dictionary.messageCenterOpenRecordInResource}>
              <Button size="small" type="text" onClick={onOpen}>
                {dictionary.viewRecord}
              </Button>
            </Tooltip>
            {config.key === "providers" ? (
              <Tooltip title={dictionary.messageCenterTestSendUnavailable}>
                <Button disabled icon={<SendOutlined />} size="small" type="text">
                  {dictionary.messageCenterTestSend}
                </Button>
              </Tooltip>
            ) : null}
          </Space>
        )}
        rowActionsColumnWidth={config.key === "providers" ? 180 : 112}
        emptyState={<Empty description={dictionary.emptyData} />}
      />
    </AdminListPanel>
  );
}

function columnsFor(key: MessageCenterResourceKey, dictionary: Dictionary, language: Language): PlatformDataTableColumn<AdminResourceRecord>[] {
  const common: PlatformDataTableColumn<AdminResourceRecord>[] = [
    {
      title: dictionary.code,
      key: "code",
      dataIndex: "code",
      width: 180,
      priority: "essential",
      render: (_value, record) => <PlatformOverflowText code value={record.code} />,
    },
    {
      title: dictionary.recordName,
      key: "name",
      dataIndex: "name",
      width: 220,
      priority: "essential",
      render: (_value, record) => <PlatformOverflowText strong value={localizedName(record, language)} />,
    },
    {
      title: dictionary.status,
      key: "status",
      dataIndex: "status",
      width: 120,
      priority: "standard",
      render: (_value, record) => <Tag color={record.status === "enabled" ? "success" : "default"}>{statusLabel(record.status, dictionary)}</Tag>,
    },
  ];
  if (key === "channels") {
    return [
      ...common,
      valueColumn("channel", dictionary.messageCenterChannel, 140),
      valueColumn("deliveryMode", dictionary.messageCenterDeliveryMode, 160),
      valueColumn("providerPolicy", dictionary.messageCenterProviderPolicy, 180),
    ];
  }
  if (key === "providers") {
    return [
      ...common,
      valueColumn("channel", dictionary.messageCenterChannel, 130),
      valueColumn("provider", dictionary.messageCenterProvider, 160),
      valueColumn("runtimeMode", dictionary.messageCenterRuntimeMode, 160),
      {
        title: dictionary.messageCenterSecretStatus,
        key: "secretStatus",
        width: 150,
        render: (_value, record) => (
          <Tag color={valueOf(record, "credentialStatus") === "configured" ? "success" : "warning"}>
            {valueOf(record, "credentialStatus") === "configured" ? dictionary.configured : dictionary.notConfigured}
          </Tag>
        ),
      },
    ];
  }
  if (key === "templates") {
    return [
      ...common,
      valueColumn("tenantCode", dictionary.tenant, 140),
      valueColumn("channel", dictionary.messageCenterChannel, 130),
      valueColumn("titleTemplate", dictionary.messageCenterTemplateTitle, 220),
    ];
  }
  if (key === "policies") {
    return [
      ...common,
      valueColumn("channel", dictionary.messageCenterChannel, 130),
      valueColumn("providerCode", dictionary.messageCenterProviderAccount, 180),
      valueColumn("rateLimitPerMinute", dictionary.messageCenterRateLimit, 160),
    ];
  }
  if (key === "deliveries") {
    return [
      ...common,
      valueColumn("channel", dictionary.messageCenterChannel, 130),
      valueColumn("deliveryStatus", dictionary.messageCenterDeliveryStatus, 150),
      valueColumn("lastAttemptAt", dictionary.messageCenterLastAttempt, 180),
      valueColumn("errorMessage", dictionary.messageCenterErrorMessage, 260),
    ];
  }
  return [
    ...common,
    valueColumn("tenantCode", dictionary.tenant, 140),
    valueColumn("templateCode", dictionary.messageCenterTemplate, 180),
    valueColumn("recipientUserCode", dictionary.messageCenterRecipient, 180),
    valueColumn("priority", dictionary.messageCenterPriority, 120),
  ];
}

function valueColumn(key: string, title: ReactNode, width: number): PlatformDataTableColumn<AdminResourceRecord> {
  return {
    title,
    key,
    width,
    priority: "standard",
    render: (_value, record) => <PlatformOverflowText value={valueOf(record, key) || "-"} />,
  };
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

function channelCards(records: MessageCenterRecords, dictionary: Dictionary) {
  const providersByChannel = new Set(records.providers.map((record) => valueOf(record, "channel")).filter(Boolean));
  const channels = records.channels.length > 0
    ? records.channels.map((record) => valueOf(record, "channel") || record.code)
    : ["in_app", "sms", "email", "wechat_official", "wechat_miniapp"];
  return channels.map((channel) => ({
    key: channel,
    label: channelLabel(channel, dictionary),
    description: channelDescription(channel, dictionary),
    icon: channelIcon(channel),
    ready: channel === "in_app" || providersByChannel.has(channel) || records.channels.some((record) => (valueOf(record, "channel") || record.code) === channel && record.status === "enabled"),
  }));
}

function channelIcon(channel: string) {
  if (channel === "sms") return <MessageOutlined />;
  if (channel === "email") return <MailOutlined />;
  if (channel.startsWith("wechat")) return <WechatOutlined />;
  if (channel === "in_app") return <BellOutlined />;
  return <CheckCircleOutlined />;
}

function messageCenterMetrics(records: MessageCenterRecords) {
  return {
    channelCount: records.channels.length,
    providerCount: records.providers.length,
    templateCount: records.templates.length,
    failedDeliveryCount: records.deliveries.filter((record) => ["failed", "error", "dead"].includes(valueOf(record, "deliveryStatus"))).length,
  };
}

async function loadAllRecords(resource: string) {
  const records: AdminResourceRecord[] = [];
  for (let page = 1; ; page += 1) {
    try {
      const result = await queryAdminResource(resource, {
        page,
        pageSize: PAGE_SIZE,
        sort: [{ field: "updatedAt", order: "desc" }],
      });
      records.push(...result.items);
      if (result.items.length < PAGE_SIZE || records.length >= result.total) {
        return records;
      }
    } catch (error) {
      if (error instanceof AdminAPIError && error.statusCode === 404) {
        return records;
      }
      throw error;
    }
  }
}

function emptyMessageCenterRecords(): MessageCenterRecords {
  return {
    channels: [],
    providers: [],
    templates: [],
    policies: [],
    notifications: [],
    deliveries: [],
  };
}

function localizedName(record: AdminResourceRecord, language: Language) {
  return valueOf(record, language === "zh" ? "nameZh" : "nameEn") || record.name || record.code;
}

function resourceLabel(key: MessageCenterResourceKey, dictionary: Dictionary) {
  return resourceConfigs.find((config) => config.key === key)?.title(dictionary) ?? key;
}

function channelLabel(channel: string, dictionary: Dictionary) {
  const labels: Record<string, string> = {
    in_app: dictionary.messageCenterChannelInApp,
    sms: dictionary.messageCenterChannelSMS,
    email: dictionary.messageCenterChannelEmail,
    wechat_official: dictionary.messageCenterChannelWechatOfficial,
    wechat_miniapp: dictionary.messageCenterChannelWechatMiniapp,
  };
  return labels[channel] ?? channel;
}

function channelDescription(channel: string, dictionary: Dictionary) {
  const descriptions: Record<string, string> = {
    in_app: dictionary.messageCenterChannelInAppDescription,
    sms: dictionary.messageCenterChannelSMSDescription,
    email: dictionary.messageCenterChannelEmailDescription,
    wechat_official: dictionary.messageCenterChannelWechatOfficialDescription,
    wechat_miniapp: dictionary.messageCenterChannelWechatMiniappDescription,
  };
  return descriptions[channel] ?? dictionary.messageCenterChannelCustomDescription;
}

function statusLabel(status: string, dictionary: Dictionary) {
  if (status === "enabled" || status === "active") return dictionary.enabled;
  if (status === "disabled") return dictionary.disabled;
  if (status === "pending") return dictionary.pendingRestart;
  if (status === "failed") return dictionary.error;
  return status || "-";
}

function valueOf(record: AdminResourceRecord, key: string) {
  return record.values?.[key] ?? (record as unknown as Record<string, string | undefined>)[key] ?? "";
}

function formatTemplate(template: string, values: Record<string, string>) {
  return Object.entries(values).reduce((result, [key, value]) => result.replaceAll(`{${key}}`, value), template);
}
