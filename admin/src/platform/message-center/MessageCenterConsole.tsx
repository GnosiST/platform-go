import {
  ApiOutlined,
  BellOutlined,
  CheckCircleOutlined,
  MailOutlined,
  MessageOutlined,
  ReloadOutlined,
  SendOutlined,
  SettingOutlined,
  WechatOutlined,
} from "@ant-design/icons";
import { App, Button, Empty, Form, Input, Select, Space, Tabs, Tag, Tooltip, Typography } from "antd";
import { useEffect, useMemo, useState, type ReactNode } from "react";
import {
  AdminAPIError,
  queryAdminResource,
  runMessageCenterDeliveries,
  testSendMessageCenter,
  type AdminResourceRecord,
  type MessageCenterDeliveriesRunResult,
  type MessageCenterTestSendResult,
} from "../api/client";
import type { Dictionary, Language } from "../i18n";
import {
  AdminActionButton,
  AdminFeedback,
  AdminListPanel,
  AdminModal,
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
type MessageCenterChannelCard = {
  key: string;
  label: string;
  description: string;
  icon: ReactNode;
  statusLabel: string;
  statusTone: string;
};

type MessageCenterTestSendForm = {
  channel: "sms";
  tenantCode: string;
  recipient: string;
  templateId: string;
  templateParams: string;
  title: string;
  body: string;
};

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
  const { message: toast } = App.useApp();
  const [testSendForm] = Form.useForm<MessageCenterTestSendForm>();
  const [records, setRecords] = useState<MessageCenterRecords>(() => emptyMessageCenterRecords());
  const [errors, setErrors] = useState<MessageCenterErrors>({});
  const [loading, setLoading] = useState(true);
  const [testSendOpen, setTestSendOpen] = useState(false);
  const [testSendSubmitting, setTestSendSubmitting] = useState(false);
  const [testSendResult, setTestSendResult] = useState<MessageCenterTestSendResult | null>(null);
  const [deliveryRunSubmitting, setDeliveryRunSubmitting] = useState(false);
  const [deliveryRunResult, setDeliveryRunResult] = useState<MessageCenterDeliveriesRunResult | null>(null);
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
  const openTestSend = (record?: AdminResourceRecord, config?: MessageCenterResourceConfig) => {
    setTestSendResult(null);
    testSendForm.setFieldsValue(messageCenterTestSendInitialValues(record, config, language));
    setTestSendOpen(true);
  };
  const closeTestSend = () => {
    if (testSendSubmitting) {
      return;
    }
    setTestSendOpen(false);
  };
  const submitTestSend = async () => {
    const values = await testSendForm.validateFields();
    setTestSendSubmitting(true);
    try {
      const result = await testSendMessageCenter({
        channel: "sms",
        tenantCode: values.tenantCode,
        recipient: values.recipient,
        templateId: values.templateId,
        templateParams: parseTemplateParams(values.templateParams),
        title: values.title,
        body: values.body,
      });
      setTestSendResult(result);
      toast.success(dictionary.messageCenterTestSendSuccess);
      await load();
    } finally {
      setTestSendSubmitting(false);
    }
  };
  const runDeliveryWorker = async () => {
    setDeliveryRunSubmitting(true);
    try {
      const result = await runMessageCenterDeliveries();
      setDeliveryRunResult(result);
      toast.success(dictionary.messageCenterDeliveryRunSuccess);
      await load();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : dictionary.messageCenterDeliveryRunFailed);
    } finally {
      setDeliveryRunSubmitting(false);
    }
  };
  const errorMessages = Object.entries(errors).filter(([, message]) => Boolean(message));

  return (
    <AdminPage
      className="message-center-console"
      title={dictionary.messageCenterTitle}
      description={dictionary.messageCenterDescription}
      actions={(
        <Space size={8} wrap>
          <Button icon={<ReloadOutlined />} loading={deliveryRunSubmitting} onClick={() => void runDeliveryWorker()}>
            {dictionary.messageCenterRunDeliveries}
          </Button>
          <Button icon={<SendOutlined />} onClick={() => openTestSend()}>
            {dictionary.messageCenterDryRun}
          </Button>
          <AdminActionButton icon={<ReloadOutlined />} label={dictionary.refresh} loading={loading} onClick={() => void load()}>
            {dictionary.refresh}
          </AdminActionButton>
        </Space>
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
      <AdminFeedback
        type="info"
        message={dictionary.messageCenterTrialReadyTitle}
        description={dictionary.messageCenterTrialReadyDescription}
      />
      {deliveryRunResult ? (
        <AdminFeedback
          type={deliveryRunResult.failed > 0 ? "warning" : "success"}
          message={dictionary.messageCenterDeliveryRunResult}
          description={formatTemplate(dictionary.messageCenterDeliveryRunResultDescription, {
            attempted: String(deliveryRunResult.attempted),
            delivered: String(deliveryRunResult.delivered),
            failed: String(deliveryRunResult.failed),
            skipped: String(deliveryRunResult.skipped),
          })}
        />
      ) : null}
      <MessageCenterClosedLoop
        dictionary={dictionary}
        records={records}
        resourceRoutes={resourceRoutes}
        onOpen={openResource}
      />
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
              <Tag color={card.statusTone}>{card.statusLabel}</Tag>
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
                onTestSend={(record) => openTestSend(record, config)}
              />
            ),
          }))}
        />
      )}
      <AdminModal
        open={testSendOpen}
        preset="form"
        size="lg"
        title={dictionary.messageCenterTestSendTitle}
        okText={dictionary.messageCenterTestSend}
        cancelText={dictionary.cancel}
        confirmLoading={testSendSubmitting}
        onCancel={closeTestSend}
        onOk={() => void submitTestSend()}
      >
        <Typography.Paragraph type="secondary">{dictionary.messageCenterTestSendDescription}</Typography.Paragraph>
        <Form form={testSendForm} layout="vertical" requiredMark={false}>
          <Space className="message-center-test-send-grid" direction="vertical" size={0}>
            <Form.Item label={dictionary.messageCenterChannel} name="channel" rules={[{ required: true }]}>
              <Select
                options={[{ value: "sms", label: dictionary.messageCenterChannelSMS }]}
              />
            </Form.Item>
            <Form.Item label={dictionary.tenant} name="tenantCode">
              <Input />
            </Form.Item>
            <Form.Item label={dictionary.messageCenterRecipient} name="recipient" rules={[{ required: true }]}>
              <Input placeholder={dictionary.messageCenterRecipientPlaceholder} />
            </Form.Item>
            <Form.Item label={dictionary.messageCenterTemplateId} name="templateId" rules={[{ required: true }]}>
              <Input />
            </Form.Item>
            <Form.Item label={dictionary.messageCenterTitleField} name="title">
              <Input />
            </Form.Item>
            <Form.Item label={dictionary.messageCenterBodyField} name="body">
              <Input.TextArea autoSize={{ minRows: 3, maxRows: 5 }} />
            </Form.Item>
            <Form.Item
              label={dictionary.messageCenterTemplateParams}
              name="templateParams"
              rules={[{
                validator: async (_rule, value) => {
                  try {
                    parseTemplateParams(value);
                  } catch {
                    throw new Error(dictionary.messageCenterTemplateParamsInvalid);
                  }
                },
              }]}
            >
              <Input.TextArea autoSize={{ minRows: 3, maxRows: 5 }} placeholder={dictionary.messageCenterTemplateParamsPlaceholder} />
            </Form.Item>
          </Space>
        </Form>
        {testSendResult ? (
          <AdminFeedback
            type="success"
            message={dictionary.messageCenterTestSendResult}
            description={formatTemplate(dictionary.messageCenterTestSendResultDescription, {
              provider: testSendResult.receipt.provider || "-",
              status: testSendResult.receipt.status || "-",
              target: testSendResult.receipt.redactedTarget || "-",
              messageId: testSendResult.receipt.messageId || "-",
            })}
          />
        ) : null}
      </AdminModal>
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
  onTestSend,
}: {
  config: MessageCenterResourceConfig;
  dictionary: Dictionary;
  language: Language;
  loading: boolean;
  records: AdminResourceRecord[];
  onOpen: () => void;
  onTestSend: (record: AdminResourceRecord) => void;
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
              <Button icon={<SendOutlined />} size="small" type="text" onClick={() => onTestSend(record)}>
                {dictionary.messageCenterTestSend}
              </Button>
            ) : null}
            {supportsTrialEntry(config.key) ? (
              <Button icon={<SendOutlined />} size="small" type="text" onClick={() => onTestSend(record)}>
                {dictionary.messageCenterDryRun}
              </Button>
            ) : null}
          </Space>
        )}
        rowActionsColumnWidth={supportsTrialEntry(config.key) || config.key === "providers" ? 220 : 112}
        emptyState={<Empty description={dictionary.emptyData} />}
      />
    </AdminListPanel>
  );
}

function MessageCenterClosedLoop({
  dictionary,
  records,
  resourceRoutes,
  onOpen,
}: {
  dictionary: Dictionary;
  records: MessageCenterRecords;
  resourceRoutes: Set<string>;
  onOpen: (config: MessageCenterResourceConfig) => void;
}) {
  const steps = messageCenterClosedLoopSteps(resourceConfigs, records, resourceRoutes, dictionary);
  return (
    <AdminListPanel
      className="message-center-closed-loop"
      title={dictionary.messageCenterClosedLoopTitle}
      toolbar={<Typography.Text type="secondary">{dictionary.messageCenterClosedLoopDescription}</Typography.Text>}
    >
      <div className="message-center-channel-strip">
        {steps.map((step) => {
          const content = (
            <>
              <span className="message-center-channel-icon">{step.icon}</span>
              <div className="settings-center-config-cell">
                <Typography.Text strong>{step.title}</Typography.Text>
                <Typography.Text type="secondary">{step.description}</Typography.Text>
              </div>
              <Space direction="vertical" size={2} align="end">
                <Tag color={step.available ? "success" : "warning"}>
                  {step.available ? dictionary.messageCenterResourceConnected : dictionary.messageCenterResourceMissing}
                </Tag>
                <Tag>{formatTemplate(dictionary.messageCenterRecordCount, { count: String(step.count) })}</Tag>
              </Space>
            </>
          );
          if (!step.available) {
            return (
              <div className="message-center-channel-card" key={step.key} aria-label={step.title}>
                {content}
              </div>
            );
          }
          return (
            <button className="settings-center-card" key={step.key} type="button" onClick={() => onOpen(step.config)}>
              {content}
            </button>
          );
        })}
      </div>
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

function channelCards(records: MessageCenterRecords, dictionary: Dictionary): MessageCenterChannelCard[] {
  const providersByChannel = new Set(
    records.providers
      .filter((record) => ["enabled", "active"].includes(record.status))
      .map((record) => valueOf(record, "channel"))
      .filter(Boolean),
  );
  const configuredChannels = new Set(
    records.channels
      .filter((record) => ["enabled", "active"].includes(record.status))
      .map((record) => valueOf(record, "channel") || record.code)
      .filter(Boolean),
  );
  const channels = records.channels.length > 0
    ? records.channels.map((record) => valueOf(record, "channel") || record.code)
    : ["in_app", "sms", "email", "wechat_official", "wechat_miniapp"];
  return channels.map((channel) => {
    const runtimeReady = channel === "in_app" || (channel === "sms" && providersByChannel.has("sms"));
    const configuredOnly = configuredChannels.has(channel) || providersByChannel.has(channel);
    return {
      key: channel,
      label: channelLabel(channel, dictionary),
      description: channelDescription(channel, dictionary),
      icon: channelIcon(channel),
      statusLabel: runtimeReady
        ? dictionary.messageCenterChannelRuntimeReady
        : configuredOnly
          ? dictionary.messageCenterChannelConfiguredOnly
          : dictionary.messageCenterChannelConfigSlot,
      statusTone: runtimeReady ? "success" : configuredOnly ? "processing" : "default",
    };
  });
}

function messageCenterClosedLoopSteps(
  configs: MessageCenterResourceConfig[],
  records: MessageCenterRecords,
  resourceRoutes: Set<string>,
  dictionary: Dictionary,
) {
  return configs.map((config) => ({
    key: config.key,
    config,
    title: config.title(dictionary),
    description: config.description(dictionary),
    available: resourceRoutes.has(config.route),
    count: records[config.key].length,
    icon: workflowStepIcon(config.key),
  }));
}

function workflowStepIcon(key: MessageCenterResourceKey) {
  if (key === "channels") return <BellOutlined />;
  if (key === "providers") return <ApiOutlined />;
  if (key === "templates") return <MailOutlined />;
  if (key === "policies") return <SettingOutlined />;
  if (key === "deliveries") return <CheckCircleOutlined />;
  return <SendOutlined />;
}

function supportsTrialEntry(key: MessageCenterResourceKey) {
  return key === "templates" || key === "policies";
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

function messageCenterTestSendInitialValues(
  record: AdminResourceRecord | undefined,
  config: MessageCenterResourceConfig | undefined,
  language: Language,
): MessageCenterTestSendForm {
  const templateId = record && config?.key === "templates"
    ? record.code
    : record
      ? valueOf(record, "templateCode")
      : "";
  return {
    channel: "sms",
    tenantCode: record ? valueOf(record, "tenantCode") || "platform" : "platform",
    recipient: "",
    templateId,
    templateParams: "{}",
    title: record ? localizedName(record, language) : "Message center SMS test",
    body: record?.description || "",
  };
}

function parseTemplateParams(raw: string | undefined): Record<string, string> | undefined {
  const trimmed = String(raw ?? "").trim();
  if (trimmed === "" || trimmed === "{}") {
    return undefined;
  }
  const parsed = JSON.parse(trimmed) as unknown;
  if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
    throw new Error("template params must be a JSON object");
  }
  return Object.fromEntries(
    Object.entries(parsed as Record<string, unknown>)
      .filter(([key]) => key.trim() !== "")
      .map(([key, value]) => [key, value == null ? "" : String(value)]),
  );
}

function formatTemplate(template: string, values: Record<string, string>) {
  return Object.entries(values).reduce((result, [key, value]) => result.replaceAll(`{${key}}`, value), template);
}
