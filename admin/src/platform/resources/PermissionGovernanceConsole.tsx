import { EditOutlined, PlusOutlined } from "@ant-design/icons";
import { Descriptions, Form, Input, Select, Space, Spin, Tag, Typography } from "antd";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  createAdminResource,
  queryAdminResource,
  updateAdminResource,
  type AdminResourceInput,
  type AdminResourceRecord,
} from "../api/client";
import type { Dictionary, Language } from "../i18n";
import { hasPermission } from "../refine";
import {
  AdminActionButton,
  AdminFeedback,
  AdminFormModal,
  AdminListPanel,
  AdminPage,
  AdminTreeWorkbench,
  platformPopupContainer,
  type AdminTreeWorkbenchNode,
} from "../ui";
import type { AdminResourceDefinition } from "./registry";

type PermissionGovernanceConsoleProps = {
  resource: AdminResourceDefinition;
  language: Language;
  dictionary: Dictionary;
  permissions: string[];
  deniedPermissions: string[];
};

type PermissionTreeSummary = {
  key: string;
  title: string;
  subtitle: string;
  resourceType: string;
  capability?: string;
  resource?: string;
  count: number;
  records: AdminResourceRecord[];
};

type PermissionTreeModel = {
  nodes: AdminTreeWorkbenchNode[];
  summaries: Map<string, PermissionTreeSummary>;
  permissions: Map<string, AdminResourceRecord>;
  firstKey: string;
};

type PermissionEditorValues = {
  code: string;
  name: string;
  nameZh?: string;
  nameEn?: string;
  resource: string;
  action: string;
  status: "enabled" | "disabled";
  description?: string;
  descriptionZh?: string;
  descriptionEn?: string;
};

type PermissionEditorState = {
  record?: AdminResourceRecord;
  sessionID: number;
};

const CUSTOM_API_PERMISSION_CAPABILITY = "custom-api";
const CUSTOM_PERMISSION_CODE = /^admin:[a-z0-9][a-z0-9-]*:[a-z0-9][a-z0-9-]*(?::[a-z0-9][a-z0-9-]*)*$/;
const CUSTOM_PERMISSION_SEGMENT = /^[a-z0-9][a-z0-9-]*$/;

export function PermissionGovernanceConsole({ resource, language, dictionary, permissions, deniedPermissions }: PermissionGovernanceConsoleProps) {
  const [records, setRecords] = useState<AdminResourceRecord[]>([]);
  const [selectedKey, setSelectedKey] = useState("");
  const [search, setSearch] = useState("");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [editor, setEditor] = useState<PermissionEditorState | null>(null);
  const [form] = Form.useForm<PermissionEditorValues>();
  const requestRef = useRef(0);
  const editorSessionRef = useRef(0);
  const savingRef = useRef(false);
  const generatedPermissionCodeRef = useRef("");
  const canRead = hasPermission(permissions, "admin:permission:read", deniedPermissions);
  const canCreate = hasPermission(permissions, "admin:permission:create", deniedPermissions);
  const canUpdate = hasPermission(permissions, "admin:permission:update", deniedPermissions);

  const loadPermissions = useCallback(async (requestID = ++requestRef.current) => {
    if (!canRead || requestRef.current !== requestID) return;
    setLoading(true);
    try {
      const nextRecords = await loadAllPermissions();
      if (requestRef.current !== requestID) return;
      setRecords(nextRecords);
      setError("");
    } catch (nextError) {
      if (requestRef.current !== requestID) return;
      setError(nextError instanceof Error ? nextError.message : dictionary.loadResourceFailed);
    } finally {
      if (requestRef.current === requestID) setLoading(false);
    }
  }, [canRead, dictionary.loadResourceFailed]);

  useEffect(() => {
    const requestID = ++requestRef.current;
    if (!canRead) {
      setRecords([]);
      setSelectedKey("");
      setLoading(false);
      return;
    }
    void loadPermissions(requestID);
    return () => { requestRef.current += 1; };
  }, [canRead, loadPermissions]);

  useEffect(() => {
    if (!editor) return;
    form.resetFields();
    form.setFieldsValue(permissionEditorValues(editor.record));
  }, [editor, form]);

  const treeModel = useMemo(
    () => projectPermissionTree(records, search, dictionary, language),
    [dictionary, language, records, search],
  );
  const permissionTreeSummary = useMemo(() => ({
    api: records.filter((record) => (valueOf(record, "resourceType") || "api") === "api").length,
    pageButton: records.filter((record) => valueOf(record, "resourceType") === "page-button").length,
    custom: records.filter(isCustomAPIPermission).length,
    system: records.filter((record) => !isCustomAPIPermission(record)).length,
  }), [records]);

  useEffect(() => {
    if (loading) return;
    setSelectedKey((current) => {
      if (current && treeModel.nodes.some((node) => node.key === current && node.selectable !== false)) return current;
      return treeModel.firstKey;
    });
  }, [loading, treeModel.firstKey, treeModel.nodes]);

  if (!canRead) {
    return <AdminFeedback type="warning" message={dictionary.noPermission} description={resource.permission} />;
  }

  const openCreate = () => {
    if (!canCreate) {
      setError(dictionary.noPermission);
      return;
    }
    setError("");
    setNotice("");
    generatedPermissionCodeRef.current = "";
    setEditor({ sessionID: ++editorSessionRef.current });
  };

  const openEdit = (record: AdminResourceRecord) => {
    if (!isCustomAPIPermission(record)) {
      setError(dictionary.permissionSystemLockedDescription);
      return;
    }
    if (!canUpdate) {
      setError(dictionary.noPermission);
      return;
    }
    setError("");
    setNotice("");
    setEditor({ record, sessionID: ++editorSessionRef.current });
  };

  const closeEditor = () => {
    editorSessionRef.current += 1;
    savingRef.current = false;
    generatedPermissionCodeRef.current = "";
    setSaving(false);
    setEditor(null);
    form.resetFields();
  };

  const syncPermissionCode = useCallback((changedValues: Partial<PermissionEditorValues>, values: PermissionEditorValues) => {
    if (editor?.record || (!("resource" in changedValues) && !("action" in changedValues))) return;
    const resourceCode = values.resource?.trim() ?? "";
    const actionCode = values.action?.trim() ?? "";
    if (!CUSTOM_PERMISSION_SEGMENT.test(resourceCode) || !CUSTOM_PERMISSION_SEGMENT.test(actionCode)) return;
    const nextCode = `admin:${resourceCode}:${actionCode}`;
    const currentCode = values.code?.trim() ?? "";
    if (currentCode && currentCode !== generatedPermissionCodeRef.current) return;
    generatedPermissionCodeRef.current = nextCode;
    form.setFieldValue("code", nextCode);
  }, [editor?.record, form]);

  const savePermission = async (values: PermissionEditorValues) => {
    if (!editor || savingRef.current) return;
    if (editor.record && !isCustomAPIPermission(editor.record)) {
      setError(dictionary.permissionSystemLockedDescription);
      return;
    }
    if ((editor.record && !canUpdate) || (!editor.record && !canCreate)) {
      setError(dictionary.noPermission);
      return;
    }
    savingRef.current = true;
    const sessionID = editor.sessionID;
    setSaving(true);
    try {
      const input = buildPermissionInput(values, dictionary);
      const result = editor.record
        ? await updateAdminResource("permissions", editor.record.id, input)
        : await createAdminResource("permissions", input);
      if (editorSessionRef.current !== sessionID) return;
      closeEditor();
      setNotice(dictionary.permissionSaveSucceeded);
      setSelectedKey(permissionNodeKey(result.record.code));
      await loadPermissions();
    } catch (nextError) {
      if (editorSessionRef.current !== sessionID) return;
      setError(nextError instanceof Error ? nextError.message : dictionary.permissionSaveFailed);
    } finally {
      if (editorSessionRef.current === sessionID) {
        savingRef.current = false;
        setSaving(false);
      }
    }
  };

  const selectedRecord = treeModel.permissions.get(selectedKey);
  const selectedSummary = treeModel.summaries.get(selectedKey);
  const detail = loading ? (
    <AdminListPanel className="permission-governance-detail" title={dictionary.permissionDetailTitle}>
      <div className="loading-panel" aria-live="polite"><Spin size="small" /></div>
    </AdminListPanel>
  ) : selectedRecord ? (
    <PermissionDetail
      record={selectedRecord}
      dictionary={dictionary}
      language={language}
      canUpdate={canUpdate && isCustomAPIPermission(selectedRecord)}
      onEdit={openEdit}
    />
  ) : selectedSummary ? (
    <PermissionSummary summary={selectedSummary} dictionary={dictionary} language={language} />
  ) : (
    <AdminListPanel className="permission-governance-detail" title={dictionary.permissionDetailTitle}>
      <div className="permission-governance-empty">{dictionary.emptyData}</div>
    </AdminListPanel>
  );

  return (
    <AdminPage title={dictionary.permissionGovernanceTitle} description={dictionary.permissionGovernanceDescription}>
      {error ? <AdminFeedback className="api-alert" type="warning" message={dictionary.loadResourceFailed} description={error} closable onClose={() => setError("")} /> : null}
      {notice ? <AdminFeedback className="api-alert" type="success" message={notice} closable onClose={() => setNotice("")} /> : null}
      <AdminFeedback className="api-alert permission-contract-alert" type="info" message={dictionary.permissionSeededGuardTitle} description={dictionary.permissionSeededGuardDescription} />
      <AdminTreeWorkbench
        actions={canCreate ? (
          <AdminActionButton disabled={loading} icon={<PlusOutlined />} label={dictionary.permissionAddCustomAPI} onClick={openCreate}>
            {dictionary.permissionAddCustomAPI}
          </AdminActionButton>
        ) : null}
        ariaLabel={dictionary.permissionTreeAriaLabel}
        detail={detail}
        emptyText={dictionary.emptyData}
        loading={loading}
        nodes={treeModel.nodes}
        searchLabel={dictionary.permissionTreeSearch}
        searchPlaceholder={dictionary.permissionTreeSearchPlaceholder}
        searchValue={search}
        selectedKey={selectedKey}
        summary={(
          <Space size={[6, 6]} wrap>
            <Tag>{dictionary.permissionTypeAPI}: {permissionTreeSummary.api}</Tag>
            <Tag>{dictionary.permissionTypePageButton}: {permissionTreeSummary.pageButton}</Tag>
            <Tag color="processing">{dictionary.permissionCustomManagedTag}: {permissionTreeSummary.custom}</Tag>
            <Tag>{dictionary.permissionSystemManagedTag}: {permissionTreeSummary.system}</Tag>
          </Space>
        )}
        title={dictionary.permissionTreeTitle}
        onSearchChange={setSearch}
        onSelect={setSelectedKey}
      />
      <AdminFormModal
        className="permission-governance-modal"
        open={Boolean(editor)}
        title={editor?.record ? dictionary.permissionEditCustomAPI : dictionary.permissionAddCustomAPI}
        width={760}
        okText={dictionary.save}
        cancelText={dictionary.cancel}
        confirmLoading={saving}
        cancelButtonProps={{ disabled: saving }}
        closable={!saving}
        keyboard={!saving}
        maskClosable={!saving}
        okButtonProps={{ disabled: editor?.record ? !canUpdate : !canCreate }}
        onCancel={() => { if (!saving) closeEditor(); }}
        onOk={() => form.submit()}
      >
        <Form<PermissionEditorValues> form={form} layout="vertical" onFinish={(values) => void savePermission(values)} onValuesChange={syncPermissionCode}>
          <div className="permission-governance-form-grid">
            <Form.Item
              name="code"
              label={dictionary.permissionCode}
              rules={[requiredRule(dictionary.requiredField, dictionary.permissionCode), permissionCodeRule(dictionary.permissionCustomCodeInvalid)]}
            >
              <Input autoComplete="off" disabled={Boolean(editor?.record)} placeholder="admin:orders:refund" />
            </Form.Item>
            <Form.Item
              name="resource"
              label={dictionary.permissionResource}
              rules={[
                requiredRule(dictionary.requiredField, dictionary.permissionResource),
                permissionSegmentRule(dictionary.permissionCustomResourceInvalid),
                resourceMatchesPermissionCodeRule(form, dictionary.permissionCustomCodeResourceMismatch),
              ]}
            >
              <Input autoComplete="off" placeholder="orders" />
            </Form.Item>
            <Form.Item
              name="action"
              label={dictionary.permissionAction}
              rules={[
                requiredRule(dictionary.requiredField, dictionary.permissionAction),
                permissionSegmentRule(dictionary.permissionCustomActionInvalid),
                actionMatchesPermissionCodeRule(form, dictionary.permissionCustomCodeActionMismatch),
              ]}
            >
              <Input autoComplete="off" placeholder="refund" />
            </Form.Item>
            <Form.Item name="status" label={dictionary.status} rules={[requiredRule(dictionary.requiredField, dictionary.status)]}>
              <Select getPopupContainer={platformPopupContainer} options={permissionStatusOptions(dictionary)} />
            </Form.Item>
            <Form.Item name="name" label={dictionary.recordName} rules={[requiredRule(dictionary.requiredField, dictionary.recordName)]}>
              <Input autoComplete="off" />
            </Form.Item>
            <Form.Item name="nameZh" label={dictionary.recordNameZh}>
              <Input autoComplete="off" />
            </Form.Item>
            <Form.Item name="nameEn" label={dictionary.recordNameEn}>
              <Input autoComplete="off" />
            </Form.Item>
            <Form.Item name="description" label={dictionary.description}>
              <Input.TextArea autoSize={{ minRows: 2, maxRows: 4 }} />
            </Form.Item>
            <Form.Item name="descriptionZh" label={dictionary.recordDescriptionZh}>
              <Input.TextArea autoSize={{ minRows: 2, maxRows: 4 }} />
            </Form.Item>
            <Form.Item name="descriptionEn" label={dictionary.recordDescriptionEn}>
              <Input.TextArea autoSize={{ minRows: 2, maxRows: 4 }} />
            </Form.Item>
          </div>
        </Form>
      </AdminFormModal>
    </AdminPage>
  );
}

function PermissionDetail({
  record,
  dictionary,
  language,
  canUpdate,
  onEdit,
}: {
  record: AdminResourceRecord;
  dictionary: Dictionary;
  language: Language;
  canUpdate: boolean;
  onEdit: (record: AdminResourceRecord) => void;
}) {
  const resourceType = valueOf(record, "resourceType") || "api";
  const customPermission = isCustomAPIPermission(record);
  return (
    <AdminListPanel
      className="permission-governance-detail"
      title={dictionary.permissionDetailTitle}
      actions={canUpdate ? (
        <AdminActionButton icon={<EditOutlined />} label={dictionary.permissionEditCustomAPI} onClick={() => onEdit(record)}>
          {dictionary.editRecord}
        </AdminActionButton>
      ) : null}
    >
      <div className="permission-governance-detail-body">
        <div className="permission-governance-detail-heading">
          <div>
            <Typography.Text className="permission-governance-detail-kicker" type="secondary">{permissionTypeLabel(resourceType, dictionary)}</Typography.Text>
            <Typography.Title level={4}>{localizedPermissionName(record, language)}</Typography.Title>
            <Typography.Text code>{record.code}</Typography.Text>
          </div>
          <Space size={6} wrap>
            <Tag color={customPermission ? "processing" : "default"}>{customPermission ? dictionary.permissionCustomManagedTag : dictionary.permissionSystemManagedTag}</Tag>
            <Tag color={record.status === "enabled" ? "success" : "default"}>{statusLabel(record.status, dictionary)}</Tag>
          </Space>
        </div>
        {!customPermission ? (
          <AdminFeedback type="info" message={dictionary.permissionSystemLockedTitle} description={dictionary.permissionSystemLockedDescription} />
        ) : null}
        <Descriptions className="permission-governance-facts" bordered column={{ xs: 1, md: 2 }} size="small">
          <PermissionFact label={dictionary.permissionResourceType} value={permissionTypeLabel(resourceType, dictionary)} />
          <PermissionFact label={dictionary.permissionCapability} value={valueOf(record, "capability") || "-"} />
          <PermissionFact label={dictionary.permissionResource} value={valueOf(record, "resource") || "-"} />
          <PermissionFact label={dictionary.permissionAction} value={valueOf(record, "action") || "-"} />
          <PermissionFact label={dictionary.permissionPrefix} value={valueOf(record, "prefix") || "-"} />
          <PermissionFact label={dictionary.status} value={statusLabel(record.status, dictionary)} />
          {resourceType === "page-button" ? (
            <>
              <PermissionFact label={dictionary.permissionMenuCode} value={valueOf(record, "menuCode") || "-"} />
              <PermissionFact label={dictionary.permissionButtonKey} value={valueOf(record, "buttonKey") || "-"} />
            </>
          ) : null}
        </Descriptions>
        <div className="permission-governance-description">
          <Typography.Text type="secondary">{dictionary.description}</Typography.Text>
          <Typography.Paragraph>{localizedPermissionDescription(record, language) || "-"}</Typography.Paragraph>
        </div>
      </div>
    </AdminListPanel>
  );
}

function PermissionSummary({ summary, dictionary, language }: { summary: PermissionTreeSummary; dictionary: Dictionary; language: Language }) {
  const previewRecords = summary.records.slice(0, 12);
  return (
    <AdminListPanel className="permission-governance-detail" title={dictionary.permissionSummaryTitle}>
      <div className="permission-governance-detail-body">
        <div className="permission-governance-detail-heading">
          <div>
            <Typography.Text className="permission-governance-detail-kicker" type="secondary">{summary.subtitle}</Typography.Text>
            <Typography.Title level={4}>{summary.title}</Typography.Title>
          </div>
          <Tag>{dictionary.permissionGroupTotal.replace("{count}", String(summary.count))}</Tag>
        </div>
        <Descriptions className="permission-governance-facts" bordered column={{ xs: 1, md: 2 }} size="small">
          <PermissionFact label={dictionary.permissionResourceType} value={permissionTypeLabel(summary.resourceType, dictionary)} />
          <PermissionFact label={dictionary.permissionCapability} value={summary.capability || "-"} />
          <PermissionFact label={dictionary.permissionResource} value={summary.resource || "-"} />
          <PermissionFact label={dictionary.permissionGroupChildren} value={String(summary.count)} />
        </Descriptions>
        <section className="permission-governance-list" aria-label={dictionary.permissionCodes}>
          <Typography.Text strong>{dictionary.permissionCodes}</Typography.Text>
          {previewRecords.length > 0 ? (
            <ul>
              {previewRecords.map((record) => (
                <li key={record.code}>
                  <span>{localizedPermissionName(record, language)}</span>
                  <Typography.Text code>{record.code}</Typography.Text>
                </li>
              ))}
            </ul>
          ) : <Typography.Text type="secondary">{dictionary.emptyData}</Typography.Text>}
        </section>
      </div>
    </AdminListPanel>
  );
}

function PermissionFact({ label, value }: { label: string; value: string }) {
  return <Descriptions.Item className="permission-governance-fact" label={label}>{value}</Descriptions.Item>;
}

function projectPermissionTree(records: AdminResourceRecord[], search: string, dictionary: Dictionary, language: Language): PermissionTreeModel {
  const normalized = search.trim().toLocaleLowerCase();
  const visible = records
    .filter((record) => !normalized || permissionSearchText(record, dictionary, language).includes(normalized))
    .sort(comparePermissionRecords);
  const summaries = new Map<string, PermissionTreeSummary>();
  const permissions = new Map<string, AdminResourceRecord>();
  const nodes = new Map<string, AdminTreeWorkbenchNode>();

  const ensureSummary = (input: Omit<PermissionTreeSummary, "count" | "records">) => {
    const existing = summaries.get(input.key);
    if (existing) return existing;
    const summary: PermissionTreeSummary = { ...input, count: 0, records: [] };
    summaries.set(input.key, summary);
    return summary;
  };

  for (const record of visible) {
    const resourceType = valueOf(record, "resourceType") || "api";
    const capability = valueOf(record, "capability") || dictionary.uncategorized;
    const resource = valueOf(record, "resource") || valueOf(record, "prefix") || dictionary.uncategorized;
    const typeKey = `permission-type:${resourceType}`;
    const capabilityKey = `${typeKey}:capability:${capability}`;
    const resourceKey = `${capabilityKey}:resource:${resource}`;
    const permissionKey = permissionNodeKey(record.code);
    const typeSummary = ensureSummary({
      key: typeKey,
      title: permissionTypeLabel(resourceType, dictionary),
      subtitle: dictionary.permissionResourceType,
      resourceType,
    });
    const capabilitySummary = ensureSummary({
      key: capabilityKey,
      title: capability,
      subtitle: dictionary.permissionCapability,
      resourceType,
      capability,
    });
    const resourceSummary = ensureSummary({
      key: resourceKey,
      title: resource,
      subtitle: dictionary.permissionResource,
      resourceType,
      capability,
      resource,
    });
    for (const summary of [typeSummary, capabilitySummary, resourceSummary]) {
      summary.count += 1;
      summary.records.push(record);
    }
    nodes.set(typeKey, {
      key: typeKey,
      kind: "group",
      label: typeSummary.title,
      subtitle: typeSummary.subtitle,
      childCount: typeSummary.count,
    });
    nodes.set(capabilityKey, {
      key: capabilityKey,
      parentKey: typeKey,
      kind: "group",
      label: capability,
      subtitle: dictionary.permissionCapability,
      childCount: capabilitySummary.count,
    });
    nodes.set(resourceKey, {
      key: resourceKey,
      parentKey: capabilityKey,
      kind: "group",
      label: resource,
      subtitle: valueOf(record, "prefix") || dictionary.permissionResource,
      childCount: resourceSummary.count,
    });
    nodes.set(permissionKey, {
      key: permissionKey,
      parentKey: resourceKey,
      kind: "item",
      label: localizedPermissionName(record, language),
      subtitle: record.code,
      meta: valueOf(record, "action") || dictionary.permissionAction,
      searchText: permissionSearchText(record, dictionary, language),
      status: record.status,
      statusLabel: statusLabel(record.status, dictionary),
      isLeaf: true,
    });
    permissions.set(permissionKey, record);
  }

  const orderedNodes = [...nodes.values()].sort(comparePermissionNodes);
  return {
    nodes: orderedNodes,
    summaries,
    permissions,
    firstKey: orderedNodes.find((node) => node.selectable !== false)?.key ?? "",
  };
}

async function loadAllPermissions() {
  const records: AdminResourceRecord[] = [];
  const pageSize = 1000;
  for (let page = 1; ; page += 1) {
    const result = await queryAdminResource("permissions", {
      page,
      pageSize,
      sort: [
        { field: "resourceType", order: "asc" },
        { field: "resource", order: "asc" },
        { field: "code", order: "asc" },
      ],
    });
    records.push(...result.items);
    if (result.items.length < pageSize || records.length >= result.total) return records;
  }
}

function buildPermissionInput(values: PermissionEditorValues, dictionary: Dictionary): AdminResourceInput {
  const code = values.code.trim();
  const resource = values.resource.trim();
  const action = values.action.trim();
  const parts = permissionPartsFromCode(code);
  if (parts.resource !== resource) {
    throw new Error(dictionary.permissionCustomCodeResourceMismatch);
  }
  if (parts.action !== action) {
    throw new Error(dictionary.permissionCustomCodeActionMismatch);
  }
  return {
    code,
    name: values.name.trim(),
    status: values.status,
    description: values.description?.trim() ?? "",
    values: cleanValues({
      resourceType: "api",
      capability: CUSTOM_API_PERMISSION_CAPABILITY,
      resource,
      action,
      prefix: parts.prefix,
      nameZh: values.nameZh?.trim() ?? "",
      nameEn: values.nameEn?.trim() ?? "",
      descriptionZh: values.descriptionZh?.trim() ?? "",
      descriptionEn: values.descriptionEn?.trim() ?? "",
    }),
  };
}

function permissionEditorValues(record?: AdminResourceRecord): PermissionEditorValues {
  return {
    code: record?.code ?? "",
    name: record?.name ?? "",
    nameZh: valueOfOptional(record, "nameZh"),
    nameEn: valueOfOptional(record, "nameEn"),
    resource: valueOfOptional(record, "resource"),
    action: valueOfOptional(record, "action"),
    status: record?.status === "disabled" ? "disabled" : "enabled",
    description: record?.description ?? "",
    descriptionZh: valueOfOptional(record, "descriptionZh"),
    descriptionEn: valueOfOptional(record, "descriptionEn"),
  };
}

function isCustomAPIPermission(record: AdminResourceRecord) {
  return (valueOf(record, "resourceType") || "api") === "api" && valueOf(record, "capability") === CUSTOM_API_PERMISSION_CAPABILITY;
}

function permissionPartsFromCode(code: string) {
  const segments = code.split(":");
  const resource = segments[1] ?? "";
  const action = segments.at(-1) ?? "";
  return { prefix: segments.slice(0, -1).join(":"), resource, action };
}

function permissionNodeKey(code: string) {
  return `permission:${code}`;
}

function cleanValues(values: Record<string, string>) {
  return Object.fromEntries(Object.entries(values).filter(([, value]) => value.trim() !== ""));
}

function permissionStatusOptions(dictionary: Dictionary) {
  return [
    { label: dictionary.enabled, value: "enabled" },
    { label: dictionary.disabled, value: "disabled" },
  ];
}

function requiredRule(message: string, label: string) {
  return { required: true, message: message.replace("{label}", label) };
}

function permissionCodeRule(message: string) {
  return { validator: async (_: unknown, value?: string) => {
    if (!value || CUSTOM_PERMISSION_CODE.test(value.trim())) return;
    throw new Error(message);
  } };
}

function permissionSegmentRule(message: string) {
  return { validator: async (_: unknown, value?: string) => {
    if (!value || CUSTOM_PERMISSION_SEGMENT.test(value.trim())) return;
    throw new Error(message);
  } };
}

function actionMatchesPermissionCodeRule(form: ReturnType<typeof Form.useForm<PermissionEditorValues>>[0], message: string) {
  return { validator: async (_: unknown, value?: string) => {
    const code = String(form.getFieldValue("code") ?? "").trim();
    if (!code || !value || permissionPartsFromCode(code).action === value.trim()) return;
    throw new Error(message);
  } };
}

function resourceMatchesPermissionCodeRule(form: ReturnType<typeof Form.useForm<PermissionEditorValues>>[0], message: string) {
  return { validator: async (_: unknown, value?: string) => {
    const code = String(form.getFieldValue("code") ?? "").trim();
    if (!code || !value || permissionPartsFromCode(code).resource === value.trim()) return;
    throw new Error(message);
  } };
}

function comparePermissionRecords(left: AdminResourceRecord, right: AdminResourceRecord) {
  return typeRank(valueOf(left, "resourceType")) - typeRank(valueOf(right, "resourceType"))
    || valueOf(left, "capability").localeCompare(valueOf(right, "capability"))
    || valueOf(left, "resource").localeCompare(valueOf(right, "resource"))
    || left.code.localeCompare(right.code);
}

function comparePermissionNodes(left: AdminTreeWorkbenchNode, right: AdminTreeWorkbenchNode) {
  const leftLevel = permissionTreeNodeLevel(left);
  const rightLevel = permissionTreeNodeLevel(right);
  return typeRank(permissionTreeResourceType(left)) - typeRank(permissionTreeResourceType(right))
    || leftLevel - rightLevel
    || String(left.parentKey ?? "").localeCompare(String(right.parentKey ?? ""))
    || String(left.label).localeCompare(String(right.label));
}

function permissionTreeNodeLevel(node: AdminTreeWorkbenchNode) {
  if (String(node.key).startsWith("permission:")) return 3;
  const parentKey = String(node.parentKey ?? "");
  if (!parentKey) return 0;
  if (parentKey.includes(":capability:")) return 2;
  return 1;
}

function permissionTreeResourceType(node: AdminTreeWorkbenchNode) {
  const key = String(node.key);
  const parentKey = String(node.parentKey ?? "");
  const source = key.startsWith("permission-type:") ? key : parentKey;
  if (!source.startsWith("permission-type:")) return "";
  return source.slice("permission-type:".length).split(":capability:")[0] ?? "";
}

function permissionSearchText(record: AdminResourceRecord, dictionary: Dictionary, language: Language) {
  return [
    record.code,
    record.name,
    localizedPermissionName(record, language),
    localizedPermissionDescription(record, language),
    valueOf(record, "resourceType"),
    permissionTypeLabel(valueOf(record, "resourceType"), dictionary),
    valueOf(record, "capability"),
    valueOf(record, "resource"),
    valueOf(record, "action"),
    valueOf(record, "prefix"),
    valueOf(record, "menuCode"),
    valueOf(record, "buttonKey"),
  ].join(" ").toLocaleLowerCase();
}

function localizedPermissionName(record: AdminResourceRecord, language: Language) {
  const localizedKeys = language === "zh" ? ["nameZh", "titleZh"] : ["nameEn", "titleEn"];
  for (const key of localizedKeys) {
    const value = valueOf(record, key);
    if (value) return value;
  }
  return record.name || record.code;
}

function localizedPermissionDescription(record: AdminResourceRecord, language: Language) {
  const value = valueOf(record, language === "zh" ? "descriptionZh" : "descriptionEn");
  return value || record.description || "";
}

function permissionTypeLabel(resourceType: string, dictionary: Dictionary) {
  if (resourceType === "page-button") return dictionary.permissionTypePageButton;
  if (resourceType === "api" || !resourceType) return dictionary.permissionTypeAPI;
  return resourceType;
}

function statusLabel(status: string, dictionary: Dictionary) {
  if (status === "enabled") return dictionary.enabled;
  if (status === "disabled") return dictionary.disabled;
  return status || "-";
}

function typeRank(resourceType: string) {
  if (resourceType === "api" || !resourceType) return 0;
  if (resourceType === "page-button") return 1;
  return 2;
}

function valueOf(record: AdminResourceRecord, key: string) {
  return record.values?.[key]?.trim() ?? "";
}

function valueOfOptional(record: AdminResourceRecord | undefined, key: string) {
  return record ? valueOf(record, key) : "";
}
