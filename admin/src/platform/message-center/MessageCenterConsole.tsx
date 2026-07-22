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
import { App, Button, Descriptions, Empty, Form, Input, Select, Space, Tabs, Tag, Timeline, Tooltip, Typography } from "antd";
import { useEffect, useMemo, useState, type ReactNode } from "react";
import {
  AdminAPIError,
  queryAdminResource,
  retryMessageCenterDelivery,
  runMessageCenterDeliveries,
  testSendMessageCenter,
  type AdminResourceRecord,
  type MessageCenterDeliveriesRunResult,
  type MessageCenterChannel,
  type MessageCenterTestSendResult,
} from "../api/client";
import type { Dictionary, Language } from "../i18n";
import {
  AdminActionButton,
  AdminFeedback,
  AdminFormModal,
  AdminListPanel,
  AdminMetricStrip,
  AdminModal,
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
  operationalState: MessageCenterChannelOperationalState;
  statusLabel: string;
  statusTone: string;
  statusDetail: string;
  testConnectLabel: string;
  testConnectEnabled: boolean;
};
type MessageCenterChannelOperationalState = "runtime-ready" | "configuration-placeholder" | "configuration-slot";
type MessageCenterRuntimeRow = {
  channel: string;
  label: string;
  icon: ReactNode;
  configStatus: string;
  testSendStatus: string;
  workerStatus: string;
  supplierBoundary: string;
  tone: string;
};
type MessageCenterNotificationPayload = {
  channel?: string;
  redactedTarget?: string;
  templateId?: string;
  templateParamKeys?: string[];
  purpose?: string;
  templateParams?: Record<string, unknown>;
};

type MessageCenterTestSendForm = {
  channel: MessageCenterChannel;
  tenantCode: string;
  recipient: string;
  templateId: string;
  templateParams: string;
  title: string;
  body: string;
};

type MessageCenterRetryForm = {
  recipient: string;
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
  const [retryForm] = Form.useForm<MessageCenterRetryForm>();
  const [records, setRecords] = useState<MessageCenterRecords>(() => emptyMessageCenterRecords());
  const [errors, setErrors] = useState<MessageCenterErrors>({});
  const [loading, setLoading] = useState(true);
  const [testSendOpen, setTestSendOpen] = useState(false);
  const [testSendSubmitting, setTestSendSubmitting] = useState(false);
  const [testSendResult, setTestSendResult] = useState<MessageCenterTestSendResult | null>(null);
  const [deliveryRunSubmitting, setDeliveryRunSubmitting] = useState(false);
  const [deliveryRunResult, setDeliveryRunResult] = useState<MessageCenterDeliveriesRunResult | null>(null);
  const [deliveryDetailRecord, setDeliveryDetailRecord] = useState<AdminResourceRecord | null>(null);
  const [retryRecord, setRetryRecord] = useState<AdminResourceRecord | null>(null);
  const [retrySubmitting, setRetrySubmitting] = useState(false);
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
  const openTestSend = (record?: AdminResourceRecord, config?: MessageCenterResourceConfig, channel?: MessageCenterChannel) => {
    setTestSendResult(null);
    testSendForm.setFieldsValue(messageCenterTestSendInitialValues(record, config, language, channel));
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
        channel: values.channel,
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
  const openRetryDelivery = (record: AdminResourceRecord) => {
    setRetryRecord(record);
    retryForm.setFieldsValue({ recipient: "" });
  };
  const closeRetryDelivery = () => {
    if (retrySubmitting) {
      return;
    }
    setRetryRecord(null);
  };
  const submitRetryDelivery = async () => {
    if (!retryRecord) {
      return;
    }
    const values = await retryForm.validateFields();
    setRetrySubmitting(true);
    try {
      await retryMessageCenterDelivery(retryRecord.id, { recipient: values.recipient });
      toast.success(dictionary.messageCenterRetryQueued);
      setRetryRecord(null);
      await load();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : dictionary.messageCenterRetryFailed);
    } finally {
      setRetrySubmitting(false);
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
            deferred: String(deliveryRunResult.deferred ?? 0),
            skipped: String(deliveryRunResult.skipped),
          })}
        />
      ) : null}
      <MessageCenterRuntimeMatrix dictionary={dictionary} records={records} />
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
            <div className={`message-center-channel-card channel-state-${card.operationalState}`} key={card.key}>
              <div className="message-center-channel-icon">{card.icon}</div>
              <div>
                <Typography.Text strong>{card.label}</Typography.Text>
                <Typography.Paragraph type="secondary">{card.description}</Typography.Paragraph>
              </div>
              <Space className="message-center-channel-runtime" direction="vertical" size={6} align="end">
                <Tag color={card.statusTone}>{card.statusLabel}</Tag>
                <Typography.Text type="secondary">{card.statusDetail}</Typography.Text>
                <Button
                  disabled={!card.testConnectEnabled}
                  size="small"
                  type={card.testConnectEnabled ? "default" : "text"}
                  onClick={() => openTestSend(undefined, undefined, normalizeMessageCenterChannel(card.key))}
                >
                  {card.testConnectLabel}
                </Button>
              </Space>
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
                onInspectDelivery={(record) => setDeliveryDetailRecord(record)}
                onRetryDelivery={openRetryDelivery}
              />
            ),
          }))}
        />
      )}
      <AdminFormModal
        open={testSendOpen}
        destroyOnHidden={false}
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
              <Select options={messageCenterChannelOptions(dictionary)} />
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
      </AdminFormModal>
      <AdminFormModal
        open={Boolean(retryRecord)}
        destroyOnHidden={false}
        preset="form"
        title={dictionary.messageCenterRetryDeliveryTitle}
        okText={dictionary.messageCenterRetryDelivery}
        cancelText={dictionary.cancel}
        confirmLoading={retrySubmitting}
        onCancel={closeRetryDelivery}
        onOk={() => void submitRetryDelivery()}
      >
        <AdminFeedback
          type="info"
          message={dictionary.messageCenterRetryRecipientRequired}
          description={dictionary.messageCenterRetryRecipientDescription}
        />
        <Form form={retryForm} layout="vertical" requiredMark={false}>
          <Form.Item label={dictionary.messageCenterRecipient} name="recipient" rules={[{ required: true }]}>
            <Input placeholder={dictionary.messageCenterRecipientPlaceholder} />
          </Form.Item>
        </Form>
      </AdminFormModal>
      <MessageCenterDeliveryDetailModal
        dictionary={dictionary}
        language={language}
        record={deliveryDetailRecord}
        records={records}
        onClose={() => setDeliveryDetailRecord(null)}
      />
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
  onInspectDelivery,
  onRetryDelivery,
}: {
  config: MessageCenterResourceConfig;
  dictionary: Dictionary;
  language: Language;
  loading: boolean;
  records: AdminResourceRecord[];
  onOpen: () => void;
  onTestSend: (record: AdminResourceRecord) => void;
  onInspectDelivery: (record: AdminResourceRecord) => void;
  onRetryDelivery: (record: AdminResourceRecord) => void;
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
            {config.key === "deliveries" ? (
              <Button size="small" type="text" onClick={() => onInspectDelivery(record)}>
                {dictionary.messageCenterDeliveryDetail}
              </Button>
            ) : null}
            {config.key === "deliveries" && canRetryDelivery(record) ? (
              <Button icon={<ReloadOutlined />} size="small" type="text" onClick={() => onRetryDelivery(record)}>
                {dictionary.messageCenterRetryDelivery}
              </Button>
            ) : null}
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
        rowActionsColumnWidth={messageCenterRowActionsWidth(config.key)}
        emptyState={<Empty description={dictionary.emptyData} />}
      />
    </AdminListPanel>
  );
}

function MessageCenterRuntimeMatrix({ dictionary, records }: { dictionary: Dictionary; records: MessageCenterRecords }) {
  const rows = messageCenterRuntimeRows(records, dictionary);
  return (
    <AdminListPanel
      className="message-center-runtime-matrix"
      title={dictionary.messageCenterRuntimeMatrixTitle}
      toolbar={<Typography.Text type="secondary">{dictionary.messageCenterRuntimeMatrixDescription}</Typography.Text>}
    >
      <div className="message-center-runtime-grid">
        {rows.map((row) => (
          <div className="message-center-runtime-row" key={row.channel}>
            <span className="message-center-channel-icon">{row.icon}</span>
            <div>
              <Typography.Text strong>{row.label}</Typography.Text>
              <Typography.Text type="secondary">{row.configStatus}</Typography.Text>
            </div>
            <Space size={6} wrap>
              <Tag color={row.tone}>{row.testSendStatus}</Tag>
              <Tag>{row.workerStatus}</Tag>
              <Tag color={row.supplierBoundary === dictionary.messageCenterSupplierLiveRequired ? "warning" : "default"}>
                {row.supplierBoundary}
              </Tag>
            </Space>
          </div>
        ))}
      </div>
    </AdminListPanel>
  );
}

function MessageCenterDeliveryDetailModal({
  dictionary,
  language,
  record,
  records,
  onClose,
}: {
  dictionary: Dictionary;
  language: Language;
  record: AdminResourceRecord | null;
  records: MessageCenterRecords;
  onClose: () => void;
}) {
  const notice = record ? notificationForDelivery(record, records.notifications) : undefined;
  const payload = notice ? safeNotificationPayload(notice) : undefined;
  const policies = record ? matchedPoliciesForDelivery(record, notice, records) : [];
  const templateParamKeys = payloadTemplateParamKeys(payload);
  return (
    <AdminModal
      open={Boolean(record)}
      preset="detail"
      size="lg"
      title={dictionary.messageCenterDeliveryDetailTitle}
      footer={<Button onClick={onClose}>{dictionary.close}</Button>}
      onCancel={onClose}
    >
      {record ? (
        <Space className="message-center-delivery-detail" direction="vertical" size={12}>
          <Descriptions bordered column={{ xs: 1, md: 2 }} size="small">
            <Descriptions.Item label={dictionary.code}>{record.code}</Descriptions.Item>
            <Descriptions.Item label={dictionary.messageCenterDeliveryStatus}>
              <Tag color={deliveryStatusTone(valueOf(record, "deliveryStatus"))}>{valueOf(record, "deliveryStatus") || "-"}</Tag>
            </Descriptions.Item>
            <Descriptions.Item label={dictionary.messageCenterChannel}>{channelLabel(valueOf(record, "channel"), dictionary)}</Descriptions.Item>
            <Descriptions.Item label={dictionary.messageCenterProvider}>{valueOf(record, "provider") || "-"}</Descriptions.Item>
            <Descriptions.Item label={dictionary.messageCenterDeliveryTarget}>{sanitizeMessageCenterTarget(valueOf(record, "channel"), valueOf(record, "target"))}</Descriptions.Item>
            <Descriptions.Item label={dictionary.messageCenterAttempts}>{valueOf(record, "attempts") || "0"}</Descriptions.Item>
            <Descriptions.Item label={dictionary.messageCenterLastAttempt}>{valueOf(record, "lastAttemptAt") || "-"}</Descriptions.Item>
            <Descriptions.Item label={dictionary.messageCenterProviderMessageId}>{valueOf(record, "providerMessageId") || "-"}</Descriptions.Item>
            <Descriptions.Item label={dictionary.messageCenterNextRetry}>{valueOf(record, "nextRetryAt") || "-"}</Descriptions.Item>
            <Descriptions.Item label={dictionary.messageCenterRetryBackoff}>
              {retryBackoffLabel(valueOf(record, "retryBackoffSeconds"), dictionary)}
            </Descriptions.Item>
            <Descriptions.Item label={dictionary.messageCenterRuntimeCapability}>
              {deliveryRuntimeCapability(record, records, dictionary)}
            </Descriptions.Item>
            <Descriptions.Item label={dictionary.messageCenterSafeFailureReason}>
              {safeFailureReason(valueOf(record, "errorMessage"), dictionary)}
            </Descriptions.Item>
            <Descriptions.Item label={dictionary.messageCenterLinkedNotification}>
              {notice ? localizedName(notice, language) : valueOf(record, "notificationCode") || "-"}
            </Descriptions.Item>
            <Descriptions.Item label={dictionary.messageCenterTemplateId}>
              {valueOf(record, "templateId") || payload?.templateId || (notice ? valueOf(notice, "templateCode") : "") || "-"}
            </Descriptions.Item>
          </Descriptions>
          <div className="message-center-detail-section">
            <Typography.Text strong>{dictionary.messageCenterPolicyHits}</Typography.Text>
            <Typography.Text type="secondary">{dictionary.messageCenterPolicyHitDescription}</Typography.Text>
            <Space size={6} wrap>
              {policies.length > 0 ? policies.map((policy) => (
                <Tag key={policy.id || policy.code}>{localizedName(policy, language)} · {policyLimitLabel(policy, dictionary)}</Tag>
              )) : <Tag>{dictionary.messageCenterNoPolicyHit}</Tag>}
            </Space>
          </div>
          <div className="message-center-detail-section">
            <Typography.Text strong>{dictionary.messageCenterSendLog}</Typography.Text>
            <Descriptions bordered column={{ xs: 1, md: 2 }} size="small">
              <Descriptions.Item label={dictionary.messageCenterProviderStatus}>{valueOf(record, "providerStatus") || "-"}</Descriptions.Item>
              <Descriptions.Item label={dictionary.messageCenterRetryRequestedAt}>{valueOf(record, "retryRequestedAt") || "-"}</Descriptions.Item>
              <Descriptions.Item label={dictionary.loggingCenterRequestId}>{valueOf(record, "requestId") || "-"}</Descriptions.Item>
              <Descriptions.Item label={dictionary.loggingCenterTraceId}>{valueOf(record, "traceId") || "-"}</Descriptions.Item>
            </Descriptions>
          </div>
          <div className="message-center-detail-section">
            <Typography.Text strong>{dictionary.messageCenterDeliveryTimeline}</Typography.Text>
            <Timeline
              items={deliveryTimelineItems(record, dictionary).map((item) => ({
                color: item.color,
                children: (
                  <Space size={8} wrap>
                    <Typography.Text>{item.label}</Typography.Text>
                    <Typography.Text type="secondary">{item.value}</Typography.Text>
                  </Space>
                ),
              }))}
            />
          </div>
          <div className="message-center-detail-section">
            <Typography.Text strong>{dictionary.messageCenterTemplateParamKeys}</Typography.Text>
            <Space size={6} wrap>
              {templateParamKeys.length > 0 ? templateParamKeys.map((key) => <Tag key={key}>{key}</Tag>) : <Tag>-</Tag>}
              <Typography.Text type="secondary">{dictionary.messageCenterNoTemplateParamValues}</Typography.Text>
            </Space>
          </div>
        </Space>
      ) : null}
    </AdminModal>
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
      valueColumn("nextRetryAt", dictionary.messageCenterNextRetry, 180),
      {
        title: dictionary.messageCenterRetryBackoff,
        key: "retryBackoffSeconds",
        width: 150,
        priority: "standard",
        render: (_value, record) => retryBackoffLabel(valueOf(record, "retryBackoffSeconds"), dictionary),
      },
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
    activeRecords(records.providers)
      .map((record) => valueOf(record, "channel"))
      .filter(Boolean),
  );
  const configuredChannels = new Set(
    activeRecords(records.channels)
      .map((record) => valueOf(record, "channel") || record.code)
      .filter(Boolean),
  );
  const channels = records.channels.length > 0
    ? records.channels.map((record) => valueOf(record, "channel") || record.code)
    : ["in_app", "sms", "email", "wechat_official", "wechat_miniapp"];
  return channels.map((channel) => {
    const smsRuntimeReady = channel === "sms" && providersByChannel.has("sms");
    const runtimeReady = channel === "in_app" || smsRuntimeReady;
    const configuredOnly = configuredChannels.has(channel) || providersByChannel.has(channel);
    const operationalState = channelOperationalState(channel, runtimeReady, configuredOnly);
    return {
      key: channel,
      label: channelLabel(channel, dictionary),
      description: channelDescription(channel, dictionary),
      icon: channelIcon(channel),
      operationalState,
      statusLabel: runtimeReady
        ? dictionary.messageCenterChannelRuntimeReady
        : configuredOnly
          ? dictionary.messageCenterChannelConfiguredOnly
          : dictionary.messageCenterChannelConfigSlot,
      statusTone: runtimeReady ? "success" : configuredOnly ? "processing" : "default",
      statusDetail: channelOperationalDetail(channel, operationalState, dictionary),
      testConnectLabel: channel === "sms"
        ? dictionary.messageCenterDryRun
        : dictionary.messageCenterLocalDryRun,
      testConnectEnabled: true,
    };
  });
}

function channelOperationalState(channel: string, runtimeReady: boolean, configuredOnly: boolean): MessageCenterChannelOperationalState {
  if (runtimeReady) return "runtime-ready";
  if (channel === "email" || channel.startsWith("wechat")) return "configuration-placeholder";
  return configuredOnly ? "configuration-placeholder" : "configuration-slot";
}

function channelOperationalDetail(channel: string, state: MessageCenterChannelOperationalState, dictionary: Dictionary) {
  if (channel === "in_app") return dictionary.messageCenterChannelInAppRuntimeDetail;
  if (channel === "sms" && state === "runtime-ready") return dictionary.messageCenterChannelSMSRuntimeDetail;
  if (channel === "sms") return dictionary.messageCenterChannelSMSDryRunDetail;
  if (channel === "email" || channel.startsWith("wechat")) return dictionary.messageCenterChannelPlaceholderDetail;
  return dictionary.messageCenterChannelConfigSlotDetail;
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

function messageCenterRowActionsWidth(key: MessageCenterResourceKey) {
  if (key === "deliveries") return 250;
  return supportsTrialEntry(key) || key === "providers" ? 220 : 112;
}

function messageCenterRuntimeRows(records: MessageCenterRecords, dictionary: Dictionary): MessageCenterRuntimeRow[] {
  const enabledChannels = new Set(activeRecords(records.channels).map((record) => valueOf(record, "channel") || record.code));
  const providersByChannel = activeRecords(records.providers).reduce<Record<string, AdminResourceRecord[]>>((grouped, record) => {
    const channel = valueOf(record, "channel");
    if (!channel) return grouped;
    grouped[channel] = [...(grouped[channel] ?? []), record];
    return grouped;
  }, {});
  const policiesByChannel = activeRecords(records.policies).reduce<Record<string, AdminResourceRecord[]>>((grouped, record) => {
    const channel = valueOf(record, "channel");
    if (!channel) return grouped;
    grouped[channel] = [...(grouped[channel] ?? []), record];
    return grouped;
  }, {});
  return (["in_app", "sms", "email", "wechat_official", "wechat_miniapp"] as const).map((channel) => {
    const providers = providersByChannel[channel] ?? [];
    const policies = policiesByChannel[channel] ?? [];
    const hasChannelConfig = enabledChannels.has(channel);
    const configStatus = hasChannelConfig
      ? formatTemplate(dictionary.messageCenterConfigSummary, {
        providers: String(providers.length),
        policies: String(policies.length),
      })
      : dictionary.messageCenterNoChannelConfig;
    if (channel === "in_app") {
      return {
        channel,
        label: channelLabel(channel, dictionary),
        icon: channelIcon(channel),
        configStatus,
        testSendStatus: dictionary.messageCenterDryRunAvailable,
        workerStatus: dictionary.messageCenterWorkerRecordsOnly,
        supplierBoundary: dictionary.messageCenterSupplierNone,
        tone: "success",
      };
    }
    if (channel === "sms") {
      const hasConfiguredProvider = providers.some((record) => valueOf(record, "credentialStatus") === "configured");
      const hasMockLocal = providers.some((record) => valueOf(record, "provider") === "mock-local");
      return {
        channel,
        label: channelLabel(channel, dictionary),
        icon: channelIcon(channel),
        configStatus,
        testSendStatus: hasConfiguredProvider ? dictionary.messageCenterSMSTestReady : dictionary.messageCenterSMSProviderRequired,
        workerStatus: hasConfiguredProvider ? dictionary.messageCenterWorkerReady : dictionary.messageCenterWorkerNeedsProvider,
        supplierBoundary: hasMockLocal ? dictionary.messageCenterSupplierNone : dictionary.messageCenterSupplierLiveRequired,
        tone: hasConfiguredProvider ? "success" : "warning",
      };
    }
    return {
      channel,
      label: channelLabel(channel, dictionary),
      icon: channelIcon(channel),
      configStatus,
      testSendStatus: dictionary.messageCenterDryRunAvailable,
      workerStatus: dictionary.messageCenterAdapterRequired,
      supplierBoundary: dictionary.messageCenterSupplierLiveRequired,
      tone: providers.length > 0 ? "processing" : "default",
    };
  });
}

function notificationForDelivery(delivery: AdminResourceRecord, notifications: AdminResourceRecord[]) {
  const notificationCode = valueOf(delivery, "notificationCode");
  return notifications.find((record) => record.code === notificationCode || record.id === notificationCode);
}

function safeNotificationPayload(record: AdminResourceRecord): MessageCenterNotificationPayload {
  const raw = valueOf(record, "payload");
  if (!raw) return {};
  try {
    const parsed = JSON.parse(raw) as unknown;
    return parsed && typeof parsed === "object" && !Array.isArray(parsed)
      ? parsed as MessageCenterNotificationPayload
      : {};
  } catch {
    return {};
  }
}

function payloadTemplateParamKeys(payload: MessageCenterNotificationPayload | undefined) {
  const keys = new Set<string>();
  for (const key of payload?.templateParamKeys ?? []) {
    if (key.trim()) keys.add(key.trim());
  }
  for (const key of Object.keys(payload?.templateParams ?? {})) {
    if (key.trim()) keys.add(key.trim());
  }
  return [...keys].sort((left, right) => left.localeCompare(right));
}

function matchedPoliciesForDelivery(delivery: AdminResourceRecord, notice: AdminResourceRecord | undefined, records: MessageCenterRecords) {
  const channel = valueOf(delivery, "channel");
  const provider = valueOf(delivery, "provider");
  const noticePayload = notice ? safeNotificationPayload(notice) : {};
  const templateCandidates = new Set([
    valueOf(delivery, "templateCode"),
    valueOf(delivery, "templateId"),
    notice ? valueOf(notice, "templateCode") : "",
    notice ? valueOf(notice, "templateId") : "",
    noticePayload.templateId ?? "",
  ].filter(Boolean));
  const providerCodes = new Set(
    records.providers
      .filter((record) => valueOf(record, "channel") === channel && valueOf(record, "provider") === provider)
      .map((record) => record.code),
  );
  if (provider) providerCodes.add(provider);
  return activeRecords(records.policies).filter((policy) => {
    if (valueOf(policy, "channel") !== channel) return false;
    const policyTemplate = valueOf(policy, "templateCode");
    if (policyTemplate && !templateCandidates.has(policyTemplate)) return false;
    const policyProvider = valueOf(policy, "providerCode");
    return !policyProvider || providerCodes.has(policyProvider);
  });
}

function activeRecords(records: AdminResourceRecord[]) {
  return records.filter(recordOperationallyEnabled);
}

function recordOperationallyEnabled(record: AdminResourceRecord) {
  const enabledValue = valueOf(record, "enabled").trim().toLowerCase();
  return ["enabled", "active"].includes(record.status) && !["0", "false", "no", "disabled", "off"].includes(enabledValue);
}

function policyLimitLabel(policy: AdminResourceRecord, dictionary: Dictionary) {
  return formatTemplate(dictionary.messageCenterPolicyLimitSummary, {
    attempts: valueOf(policy, "maxAttempts") || "-",
    rate: valueOf(policy, "rateLimitPerMinute") || "-",
    retry: retryBackoffLabel(valueOf(policy, "retryIntervalSeconds"), dictionary),
    quiet: valueOf(policy, "quietHours") || dictionary.no,
  });
}

function canRetryDelivery(record: AdminResourceRecord) {
  return valueOf(record, "deliveryStatus") === "failed";
}

function retryBackoffLabel(rawSeconds: string, dictionary: Dictionary) {
  const seconds = Number.parseInt(rawSeconds.trim(), 10);
  if (!Number.isFinite(seconds) || seconds <= 0) {
    return "-";
  }
  if (seconds < 60) {
    return formatTemplate(dictionary.messageCenterRetryBackoffSeconds, { seconds: String(seconds) });
  }
  const minutes = Math.round(seconds / 60);
  return formatTemplate(dictionary.messageCenterRetryBackoffMinutes, { minutes: String(minutes) });
}

function deliveryTimelineItems(record: AdminResourceRecord, dictionary: Dictionary) {
  const items: Array<{ key: string; label: string; value: string; color: string }> = [
    {
      key: "created",
      label: dictionary.messageCenterTimelineCreated,
      value: record.updatedAt || "-",
      color: "blue",
    },
    {
      key: "lastAttempt",
      label: dictionary.messageCenterTimelineAttempted,
      value: valueOf(record, "lastAttemptAt") || "-",
      color: valueOf(record, "lastAttemptAt") ? "blue" : "gray",
    },
    {
      key: "nextRetry",
      label: dictionary.messageCenterTimelineNextRetry,
      value: valueOf(record, "nextRetryAt") || "-",
      color: valueOf(record, "nextRetryAt") ? "orange" : "gray",
    },
    {
      key: "delivered",
      label: dictionary.messageCenterTimelineDelivered,
      value: valueOf(record, "deliveredAt") || "-",
      color: valueOf(record, "deliveredAt") ? "green" : "gray",
    },
  ];
  const retryRequestedAt = valueOf(record, "retryRequestedAt");
  if (retryRequestedAt) {
    items.splice(2, 0, {
      key: "retryRequested",
      label: dictionary.messageCenterTimelineRetryRequested,
      value: retryRequestedAt,
      color: "blue",
    });
  }
  return items;
}

function deliveryRuntimeCapability(record: AdminResourceRecord, records: MessageCenterRecords, dictionary: Dictionary) {
  const channel = valueOf(record, "channel");
  const provider = valueOf(record, "provider");
  const errorMessage = valueOf(record, "errorMessage");
  if (errorMessage === "notification delivery sender unavailable") {
    return dictionary.messageCenterAdapterRequired;
  }
  if (channel === "in_app") {
    return dictionary.messageCenterWorkerRecordsOnly;
  }
  if (channel === "sms") {
    const providerRecord = records.providers.find((item) => valueOf(item, "channel") === channel && valueOf(item, "provider") === provider);
    if (provider === "mock-local" || providerRecord?.code === "sms-mock-local") {
      return dictionary.messageCenterSupplierNone;
    }
    return valueOf(providerRecord ?? emptyAdminResourceRecord(), "credentialStatus") === "configured"
      ? dictionary.messageCenterWorkerReady
      : dictionary.messageCenterSupplierLiveRequired;
  }
  if (valueOf(record, "providerMessageId").includes("dry-run")) {
    return dictionary.messageCenterDryRunAvailable;
  }
  return dictionary.messageCenterAdapterRequired;
}

function emptyAdminResourceRecord(): AdminResourceRecord {
  return { id: "", code: "", name: "", status: "", description: "", updatedAt: "", values: {} };
}

function safeFailureReason(value: string, dictionary: Dictionary) {
  const safe = value.trim();
  if (!safe) return "-";
  if (safe === "notification delivery sender unavailable") return dictionary.messageCenterAdapterRequired;
  if (safe === "message center test send failed") return dictionary.messageCenterTestSendFailedReason;
  return dictionary.messageCenterDeliveryFailedReason;
}

function deliveryStatusTone(status: string) {
  if (status === "delivered") return "success";
  if (status === "failed") return "error";
  if (status === "pending") return "processing";
  return "default";
}

function sanitizeMessageCenterTarget(channel: string, target: string) {
  const trimmed = target.trim();
  if (!trimmed) return "-";
  if (channel === "email") {
    const at = trimmed.lastIndexOf("@");
    if (at > 0 && at < trimmed.length - 1) {
      return `${trimmed.slice(0, Math.min(2, at))}***${trimmed.slice(at)}`;
    }
  }
  const runes = [...trimmed];
  if (runes.length <= 4) return "****";
  return `****${runes.slice(-4).join("")}`;
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
  channel: MessageCenterChannel | undefined,
): MessageCenterTestSendForm {
  const templateId = record && config?.key === "templates"
    ? record.code
    : record
      ? valueOf(record, "templateCode")
      : "";
  return {
    channel: channel ?? normalizeMessageCenterChannel(record ? valueOf(record, "channel") : ""),
    tenantCode: record ? valueOf(record, "tenantCode") || "platform" : "platform",
    recipient: "",
    templateId,
    templateParams: "{}",
    title: record ? localizedName(record, language) : "Message center test",
    body: record?.description || "",
  };
}

function messageCenterChannelOptions(dictionary: Dictionary) {
  return (["in_app", "sms", "email", "wechat_official", "wechat_miniapp"] as MessageCenterChannel[]).map((channel) => ({
    value: channel,
    label: channelLabel(channel, dictionary),
  }));
}

function normalizeMessageCenterChannel(channel: string): MessageCenterChannel {
  if (channel === "in_app" || channel === "sms" || channel === "email" || channel === "wechat_official" || channel === "wechat_miniapp") {
    return channel;
  }
  return "sms";
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
