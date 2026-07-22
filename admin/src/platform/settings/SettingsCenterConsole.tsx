import { ApiOutlined, BellOutlined, BgColorsOutlined, CheckCircleOutlined, DatabaseOutlined, LockOutlined, ReloadOutlined, SettingOutlined } from "@ant-design/icons";
import { Button, Empty, Form, Input, InputNumber, Select, Space, Switch, Tag, Typography } from "antd";
import type { FormInstance } from "antd";
import type { Rule } from "antd/es/form";
import { useCallback, useEffect, useMemo, useState, type ReactNode } from "react";
import {
  getAdminSettingsRuntime,
  testConnectAdminSettingsResource,
  updateAdminSettingsResource,
  validateAdminSettingsResourceConfig,
  type AdminResourceField,
  type AdminResourceRecord,
  type AdminResourceSchema,
  type AdminSettingsConfigCheck,
  type AdminSettingsUpdateInput,
  type AdminSettingsResourceItem,
  type AdminSettingsTestConnectionResult,
  type AdminSettingsValidationResult,
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
  schema: AdminResourceSchema;
  recordCount: number;
  enabledCount: number;
  writable: boolean;
  runtimeApplyMode: string;
  restartRequired: boolean;
  pendingRestart: boolean;
  fieldCount: number;
  updatedAt: string;
};

type SettingsResourceGroup = "core" | "message" | "capability";
type SettingsErrorState = Partial<Record<string, string>>;
type SettingsFormValue = string | number | boolean | string[] | undefined;
type SettingsFormValues = Record<string, SettingsFormValue>;
type SettingsOperationResult =
  | { kind: "save"; message: string; restartRequired: boolean; pendingRestart: boolean; checks?: AdminSettingsConfigCheck[] }
  | { kind: "validate"; result: AdminSettingsValidationResult }
  | { kind: "test"; result: AdminSettingsTestConnectionResult };

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
  const [form] = Form.useForm<SettingsFormValues>();
  const [runtimeItems, setRuntimeItems] = useState<AdminSettingsResourceItem[]>([]);
  const [errors, setErrors] = useState<SettingsErrorState>({});
  const [loading, setLoading] = useState(true);
  const [activeConfigKey, setActiveConfigKey] = useState<string | null>(null);
  const [activeRecordID, setActiveRecordID] = useState<string | null>(null);
  const [modalError, setModalError] = useState("");
  const [operationResult, setOperationResult] = useState<SettingsOperationResult | null>(null);
  const [saving, setSaving] = useState(false);
  const [validating, setValidating] = useState(false);
  const [testingConnection, setTestingConnection] = useState(false);
  const availableConfigs = useMemo(
    () => projectSettingsResourceConfigs(runtimeItems, resources, dictionary, language),
    [dictionary, language, resources, runtimeItems],
  );
  const activeConfig = useMemo(
    () => availableConfigs.find((config) => config.key === activeConfigKey) ?? null,
    [activeConfigKey, availableConfigs],
  );
  const activeRecord = useMemo(
    () => activeConfig?.records.find((record) => record.id === activeRecordID) ?? activeConfig?.records[0] ?? null,
    [activeConfig, activeRecordID],
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
      restartRequiredResources: availableConfigs.filter((config) => config.restartRequired).length,
      pendingRestartResources: availableConfigs.filter((config) => config.pendingRestart).length,
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

  useEffect(() => {
    if (!activeConfig || !activeRecord) return;
    setActiveRecordID(activeRecord.id);
    form.setFieldsValue(settingsFormInitialValues(activeRecord, activeConfig.schema?.fields ?? []));
    setModalError("");
    setOperationResult(null);
  }, [activeConfig, activeRecord, form]);

  const openConfig = useCallback((config: SettingsResourceConfig) => {
    const firstRecord = config.records[0] ?? null;
    setActiveConfigKey(config.key);
    setActiveRecordID(firstRecord?.id ?? null);
    setModalError("");
    setOperationResult(null);
  }, []);

  const closeConfig = useCallback(() => {
    setActiveConfigKey(null);
    setActiveRecordID(null);
    setModalError("");
    setOperationResult(null);
    form.resetFields();
  }, [form]);

  const saveActiveConfig = useCallback(async () => {
    if (!activeConfig || !activeRecord) return;
    setSaving(true);
    setModalError("");
    try {
      const values = await form.validateFields();
      const result = await updateAdminSettingsResource(
        activeConfig.resource,
        activeRecord.id,
        settingsUpdateInputFromForm(values, activeRecord, activeConfig.schema?.fields ?? []),
      );
      setOperationResult({
        kind: "save",
        message: dictionary.settingsCenterSaveSucceeded,
        restartRequired: result.restartRequired,
        pendingRestart: result.pendingRestart,
      });
      await load();
      setActiveConfigKey(activeConfig.key);
      setActiveRecordID(result.record.id);
    } catch (error) {
      setModalError(error instanceof Error ? error.message : dictionary.saveFailed);
    } finally {
      setSaving(false);
    }
  }, [activeConfig, activeRecord, dictionary.saveFailed, dictionary.settingsCenterSaveSucceeded, form, load]);

  const validateActiveConfig = useCallback(async () => {
    if (!activeConfig || !activeRecord) return;
    setValidating(true);
    setModalError("");
    try {
      const result = await validateAdminSettingsResourceConfig(activeConfig.resource, activeRecord.id);
      setOperationResult({ kind: "validate", result });
    } catch (error) {
      setModalError(error instanceof Error ? error.message : dictionary.settingsCenterValidateFailed);
    } finally {
      setValidating(false);
    }
  }, [activeConfig, activeRecord, dictionary.settingsCenterValidateFailed]);

  const testActiveConfigConnection = useCallback(async () => {
    if (!activeConfig || !activeRecord) return;
    setTestingConnection(true);
    setModalError("");
    try {
      const result = await testConnectAdminSettingsResource(activeConfig.resource, activeRecord.id);
      setOperationResult({ kind: "test", result });
    } catch (error) {
      setModalError(error instanceof Error ? error.message : dictionary.settingsCenterTestConnectFailed);
    } finally {
      setTestingConnection(false);
    }
  }, [activeConfig, activeRecord, dictionary.settingsCenterTestConnectFailed]);

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
                      <button className="settings-center-card" key={config.key} type="button" onClick={() => openConfig(config)}>
                        <span className="settings-center-card-icon">{config.icon}</span>
                        <span>
                          <Typography.Text strong>{config.title}</Typography.Text>
                          <Typography.Text type="secondary">{config.description}</Typography.Text>
                          <Typography.Text className="secondary-text" code>{config.resource}</Typography.Text>
                        </span>
                        <Space direction="vertical" size={2} align="end">
                          <Tag>{config.recordCount}</Tag>
                          <Tag color={config.writable ? "success" : "default"}>{config.writable ? dictionary.writable : dictionary.readOnly}</Tag>
                          <Tag color={config.pendingRestart ? "warning" : config.restartRequired ? "default" : "success"}>
                            {settingsApplyModeLabel(config, dictionary)}
                          </Tag>
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
              <Button size="small" type="text" onClick={() => {
                const config = availableConfigs.find((item) => item.key === row.key);
                if (config) openConfig(config);
              }}>
                {dictionary.settingsCenterConfigure}
              </Button>
              <Button size="small" type="text" onClick={() => onRouteChange(row.route)}>
                {dictionary.settingsCenterManage}
              </Button>
            </Space>
          )}
          rowActionsColumnWidth={168}
          emptyState={<Empty description={dictionary.emptyData} />}
        />
      </AdminListPanel>
      <SettingsConfigModal
        activeConfig={activeConfig}
        activeRecord={activeRecord}
        dictionary={dictionary}
        form={form}
        language={language}
        modalError={modalError}
        operationResult={operationResult}
        saving={saving}
        validating={validating}
        testingConnection={testingConnection}
        onClose={closeConfig}
        onManage={() => {
          if (activeConfig) onRouteChange(activeConfig.route);
        }}
        onRecordChange={(id) => setActiveRecordID(id)}
        onSave={() => void saveActiveConfig()}
        onTestConnection={() => void testActiveConfigConnection()}
        onValidate={() => void validateActiveConfig()}
      />
    </AdminPage>
  );
}

function SettingsConfigModal({
  activeConfig,
  activeRecord,
  dictionary,
  form,
  language,
  modalError,
  operationResult,
  saving,
  validating,
  testingConnection,
  onClose,
  onManage,
  onRecordChange,
  onSave,
  onTestConnection,
  onValidate,
}: {
  activeConfig: SettingsResourceConfig | null;
  activeRecord: AdminResourceRecord | null;
  dictionary: Dictionary;
  form: FormInstance<SettingsFormValues>;
  language: Language;
  modalError: string;
  operationResult: SettingsOperationResult | null;
  saving: boolean;
  validating: boolean;
  testingConnection: boolean;
  onClose: () => void;
  onManage: () => void;
  onRecordChange: (id: string) => void;
  onSave: () => void;
  onTestConnection: () => void;
  onValidate: () => void;
}) {
  const fields = useMemo(() => settingsFormFields(activeConfig), [activeConfig]);
  const open = Boolean(activeConfig);
  return (
    <AdminModal
      className="settings-config-modal"
      footer={(
        <Space size={8} wrap>
          <Button onClick={onClose}>{dictionary.cancel}</Button>
          <Button onClick={onManage}>{dictionary.settingsCenterManage}</Button>
          <Button disabled={!activeRecord} loading={validating} onClick={onValidate}>
            {dictionary.settingsCenterValidate}
          </Button>
          <Button disabled={!activeRecord} loading={testingConnection} onClick={onTestConnection}>
            {dictionary.settingsCenterTestConnect}
          </Button>
          <Button disabled={!activeConfig?.writable || !activeRecord} loading={saving} type="primary" onClick={onSave}>
            {dictionary.save}
          </Button>
        </Space>
      )}
      open={open}
      preset="form"
      size="xl"
      title={activeConfig?.title ?? dictionary.settingsCenterConfigure}
      onCancel={onClose}
    >
      {!activeConfig ? null : (
        <Space className="settings-config-modal-stack" direction="vertical" size={12}>
          <div className="settings-config-modal-summary">
            <span className="settings-center-card-icon">{activeConfig.icon}</span>
            <div>
              <Typography.Text strong>{activeConfig.title}</Typography.Text>
              <Typography.Text type="secondary">{activeConfig.description}</Typography.Text>
              <Typography.Text className="secondary-text" code>{activeConfig.resource}</Typography.Text>
            </div>
            <Space size={6} wrap>
              <Tag>{activeConfig.capability}</Tag>
              <Tag color={activeConfig.writable ? "success" : "default"}>{activeConfig.writable ? dictionary.writable : dictionary.readOnly}</Tag>
              <Tag color={activeConfig.pendingRestart ? "warning" : activeConfig.restartRequired ? "default" : "success"}>
                {settingsApplyModeLabel(activeConfig, dictionary)}
              </Tag>
            </Space>
          </div>
          {activeConfig.records.length > 1 ? (
            <div className="settings-config-record-selector">
              <Typography.Text type="secondary">{dictionary.settingsCenterRecord}</Typography.Text>
              <Select
                value={activeRecord?.id}
                options={activeConfig.records.map((record) => ({
                  value: record.id,
                  label: `${record.name || record.code || record.id} (${record.status})`,
                }))}
                onChange={onRecordChange}
              />
            </div>
          ) : null}
          {modalError ? <AdminFeedback type="error" message={dictionary.settingsCenterOperationFailed} description={modalError} /> : null}
          {operationResult ? <SettingsOperationResultPanel dictionary={dictionary} result={operationResult} /> : null}
          {activeRecord && fields.length > 0 ? (
            <Form className="settings-config-form" form={form} layout="vertical">
              <div className="settings-config-form-grid">
                {fields.map((field) => (
                  <Form.Item
                    extra={localizedText(field.help, language, "")}
                    key={field.key}
                    label={localizedText(field.label, language, field.key)}
                    name={field.key}
                    rules={settingsFieldRules(field, dictionary, language)}
                    valuePropName={field.type === "switch" ? "checked" : undefined}
                  >
                    {settingsFieldControl(field, dictionary, language)}
                  </Form.Item>
                ))}
              </div>
            </Form>
          ) : (
            <Empty description={activeRecord ? dictionary.settingsCenterNoEditableFields : dictionary.settingsCenterNoRecords} />
          )}
        </Space>
      )}
    </AdminModal>
  );
}

function SettingsOperationResultPanel({ dictionary, result }: { dictionary: Dictionary; result: SettingsOperationResult }) {
  if (result.kind === "save") {
    return (
      <AdminFeedback
        type="success"
        message={result.message}
        description={result.pendingRestart ? dictionary.restartPending : result.restartRequired ? dictionary.settingsCenterRestartApplyMode : dictionary.settingsCenterDynamicApplyMode}
      />
    );
  }
  const payload = result.result;
  return (
    <AdminFeedback
      type={payload.status === "invalid" ? "warning" : "success"}
      message={result.kind === "validate" ? dictionary.settingsCenterValidationResult : dictionary.settingsCenterTestConnectResult}
      description={(
        <div className="settings-config-checks">
          <Space size={6} wrap>
            <Tag color={payload.status === "invalid" ? "warning" : payload.status === "unsupported" ? "default" : "success"}>{payload.status}</Tag>
            {"connected" in payload ? <Tag color={payload.connected ? "success" : "default"}>{payload.mode}</Tag> : null}
            <Tag color={payload.pendingRestart ? "warning" : "default"}>{payload.pendingRestart ? dictionary.restartPending : dictionary.settingsCenterApplyMode}</Tag>
          </Space>
          {payload.checks.length > 0 ? (
            <div className="settings-config-check-list">
              {payload.checks.map((check) => (
                <div className="settings-config-check" key={`${check.key}-${check.message}`}>
                  <Tag color={check.status === "invalid" ? "warning" : check.status === "ok" ? "success" : "default"}>{check.key}</Tag>
                  <Typography.Text>{check.message}</Typography.Text>
                </div>
              ))}
            </div>
          ) : null}
        </div>
      )}
    />
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
  applyMode: string;
  applyModeTone: "default" | "success" | "warning";
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
    restartRequiredResources: number;
    pendingRestartResources: number;
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
          <Space size={6} wrap>
            <Tag>{formatTemplate(dictionary.settingsCenterWritableCount, { count: String(metrics.writableResources) })}</Tag>
            <Tag>{formatTemplate(dictionary.settingsCenterRestartRequiredCount, { count: String(metrics.restartRequiredResources) })}</Tag>
            <Tag color={metrics.pendingRestartResources > 0 ? "warning" : "default"}>
              {formatTemplate(dictionary.settingsCenterPendingRestartCount, { count: String(metrics.pendingRestartResources) })}
            </Tag>
          </Space>
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
      title: dictionary.settingsCenterApplyMode,
      key: "applyMode",
      dataIndex: "applyMode",
      width: 130,
      priority: "extended",
      render: (_value, row) => <Tag color={row.applyModeTone === "warning" ? "warning" : row.applyModeTone === "success" ? "success" : "default"}>{row.applyMode}</Tag>,
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
    applyMode: settingsApplyModeLabel(config, dictionary),
    applyModeTone: config.pendingRestart ? "warning" : config.restartRequired ? "default" : "success",
    fields: config.fieldCount,
    updatedAt: config.updatedAt,
  };
}

function settingsFormFields(config: SettingsResourceConfig | null) {
  return (config?.schema?.fields ?? []).filter((field) => field.inForm !== false && (field.source === "record" || field.source === "values"));
}

function settingsFormInitialValues(record: AdminResourceRecord, fields: AdminResourceField[]): SettingsFormValues {
  const values: SettingsFormValues = {};
  for (const field of fields) {
    const raw = settingsRecordFieldValue(record, field);
    values[field.key] = settingsFieldInitialValue(field, raw);
  }
  return values;
}

function settingsUpdateInputFromForm(values: SettingsFormValues, record: AdminResourceRecord, fields: AdminResourceField[]): AdminSettingsUpdateInput {
  const input: AdminSettingsUpdateInput = {};
  const nextValues: Record<string, unknown> = {};
  for (const field of fields) {
    if (field.readOnly || field.inForm === false) continue;
    const value = values[field.key];
    if (field.source === "record") {
      if (field.key === "code") input.code = stringFormValue(value);
      if (field.key === "name") input.name = stringFormValue(value);
      if (field.key === "status") input.status = stringFormValue(value);
      if (field.key === "description") input.description = stringFormValue(value);
      continue;
    }
    if (field.source !== "values") continue;
    if (settingsSecretField(field) && stringFormValue(value) === "" && !record.values?.[field.key]) {
      continue;
    }
    nextValues[field.key] = settingsSubmitValue(field, value);
  }
  if (Object.keys(nextValues).length > 0) {
    input.values = nextValues;
  }
  return input;
}

function settingsRecordFieldValue(record: AdminResourceRecord, field: AdminResourceField) {
  if (field.source === "record") {
    return String(record[field.key as keyof AdminResourceRecord] ?? "");
  }
  return record.values?.[field.key] ?? "";
}

function settingsFieldInitialValue(field: AdminResourceField, raw: string) {
  if (field.type === "switch") return raw === "true" || raw === "enabled" || raw === "1";
  if (field.type === "number") {
    const parsed = Number(raw);
    return Number.isFinite(parsed) ? parsed : undefined;
  }
  if (field.type === "multiselect") {
    try {
      const parsed = JSON.parse(raw);
      if (Array.isArray(parsed)) return parsed.map(String);
    } catch {
      // fall through to comma-separated legacy values
    }
    return raw ? raw.split(",").map((item) => item.trim()).filter(Boolean) : [];
  }
  return raw;
}

function settingsSubmitValue(field: AdminResourceField, value: unknown) {
  if (field.type === "switch") return Boolean(value);
  if (field.type === "number") return value === undefined || value === null || value === "" ? "" : Number(value);
  if (field.type === "multiselect") return Array.isArray(value) ? value : [];
  return stringFormValue(value);
}

function stringFormValue(value: unknown) {
  if (value === undefined || value === null) return "";
  return String(value).trim();
}

function settingsSecretField(field: AdminResourceField) {
  return field.sensitivity === "secret" || field.storageMode === "encrypted" || field.responseMode === "omitted";
}

function settingsFieldRules(field: AdminResourceField, dictionary: Dictionary, language: Language): Rule[] | undefined {
  const rules: Rule[] = [];
  const label = localizedText(field.label, language, field.key);
  if (field.required && !settingsSecretField(field)) {
    rules.push({ required: true, message: formatTemplate(dictionary.requiredField, { label }) });
  }
  if (field.validation?.minLength) {
    rules.push({ min: field.validation.minLength, message: formatTemplate(dictionary.requiredField, { label }) });
  }
  if (field.validation?.maxLength) {
    rules.push({ max: field.validation.maxLength, message: formatTemplate(dictionary.requiredField, { label }) });
  }
  return rules.length > 0 ? rules : undefined;
}

function settingsFieldControl(field: AdminResourceField, dictionary: Dictionary, language: Language) {
  const disabled = field.readOnly;
  if (field.type === "switch") return <Switch disabled={disabled} />;
  if (field.type === "number") return <InputNumber disabled={disabled} style={{ width: "100%" }} />;
  if (field.type === "textarea") return <Input.TextArea autoSize={{ minRows: 2, maxRows: 5 }} disabled={disabled} />;
  if (field.type === "select" || field.type === "multiselect") {
    return (
      <Select
        disabled={disabled}
        mode={field.type === "multiselect" ? "multiple" : undefined}
        options={(field.options ?? []).map((option) => ({ value: option.value, label: localizedText(option.label, language, option.value) }))}
      />
    );
  }
  if (settingsSecretField(field)) {
    return <Input.Password autoComplete="new-password" disabled={disabled} placeholder={dictionary.settingsCenterSecretPreserveHint} />;
  }
  return <Input disabled={disabled} />;
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
      schema: item.schema,
      recordCount: item.recordCount ?? records.length,
      enabledCount: records.filter((record) => record.status === "enabled" || record.status === "active").length,
      writable: item.writable,
      runtimeApplyMode: item.runtimeApplyMode,
      restartRequired: item.restartRequired,
      pendingRestart: item.pendingRestart,
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

function settingsApplyModeLabel(config: Pick<SettingsResourceConfig, "runtimeApplyMode" | "restartRequired" | "pendingRestart">, dictionary: Dictionary) {
  if (config.pendingRestart) {
    return dictionary.restartPending;
  }
  if (config.restartRequired || config.runtimeApplyMode.includes("restart")) {
    return dictionary.settingsCenterRestartApplyMode;
  }
  return dictionary.settingsCenterDynamicApplyMode;
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
