import {
  DeleteOutlined,
  EditOutlined,
  FileAddOutlined,
  FolderAddOutlined,
  PlusOutlined,
} from "@ant-design/icons";
import {
  Descriptions,
  Form,
  Input,
  InputNumber,
  Select,
  Space,
  Spin,
  Switch,
  Tag,
  Typography,
} from "antd";
import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import {
  queryAdminResource,
  type AdminResourceRecord,
} from "../api/client";
import {
  createMenuDefinition,
  getMenuDefinition,
  replaceMenuDefinition,
  type MenuDefinition,
  type MenuParameter,
  type PageButton,
} from "../api/organizationRBAC";
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

type MenuGovernanceConsoleProps = {
  resource: AdminResourceDefinition;
  availableResourceRoutes: string[];
  language: Language;
  dictionary: Dictionary;
  permissions: string[];
  deniedPermissions: string[];
};

type MenuEditorMode = "create-directory" | "create-page" | "edit-directory" | "edit-page";
type MenuParameterType = "string" | "number" | "boolean";

type MenuParameterValue = {
  key: string;
  type: MenuParameterType;
  value: string | number | boolean;
};

type MenuButtonValue = {
  menuCode?: string;
  buttonKey: string;
  labelZh: string;
  labelEn: string;
  action: string;
  sortOrder: number;
  status: "enabled" | "disabled";
  permissionCode: string;
};

type MenuEditorValues = {
  code: string;
  parentCode?: string;
  titleZh: string;
  titleEn: string;
  descriptionZh?: string;
  descriptionEn?: string;
  status: "enabled" | "disabled";
  icon?: string;
  sortOrder?: number;
  external?: boolean;
  route?: string;
  componentKey?: string;
  resourceCode?: string;
  externalUrl?: string;
  openMode?: "same-tab" | "new-tab";
  parameters?: MenuParameterValue[];
  cacheEnabled?: boolean;
  hidden?: boolean;
  activeMenuCode?: string;
  breadcrumbVisible?: boolean;
  buttons?: MenuButtonValue[];
};

type MenuEditorState = {
  mode: MenuEditorMode;
  definition?: MenuDefinition;
  revision: number;
};

const SAFE_PARAMETER_KEY = /^[A-Za-z][A-Za-z0-9_.-]{0,63}$/;
const SAFE_CODE = /^[A-Za-z0-9][A-Za-z0-9:._/-]{0,190}$/;
const FORBIDDEN_PARAMETER_INPUT = /(?:<script|javascript:|\$\{|\{\{|\}\}|\b(?:select|insert|update|delete|drop|alter|exec|union|datasource|shard|database|schema|sql|script|expression)\b|(?:^|\/)[:*][A-Za-z])/i;

export function MenuGovernanceConsole({ resource, availableResourceRoutes, language, dictionary, permissions, deniedPermissions }: MenuGovernanceConsoleProps) {
  const [records, setRecords] = useState<AdminResourceRecord[]>([]);
  const [selectedKey, setSelectedKey] = useState("");
  const [selectedDefinition, setSelectedDefinition] = useState<MenuDefinition | null>(null);
  const [selectedRevision, setSelectedRevision] = useState(0);
  const [definitionRefresh, setDefinitionRefresh] = useState(0);
  const [search, setSearch] = useState("");
  const [loading, setLoading] = useState(true);
  const [definitionLoading, setDefinitionLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [editor, setEditor] = useState<MenuEditorState | null>(null);
  const [form] = Form.useForm<MenuEditorValues>();
  const menuListRequest = useRef(0);
  const definitionRequest = useRef(0);
  const returnFocusRef = useRef<HTMLElement | null>(null);
  const canRead = hasPermission(permissions, "admin:menu:read", deniedPermissions);
  const canCreate = hasPermission(permissions, "admin:menu:create", deniedPermissions);
  const canUpdate = hasPermission(permissions, "admin:menu:update", deniedPermissions);
  const directoryMode = editor?.mode === "create-directory" || editor?.mode === "edit-directory";
  const externalPage = Form.useWatch("external", form) ?? false;

  const loadMenus = useCallback(async (query = "", requestID = ++menuListRequest.current) => {
    if (menuListRequest.current !== requestID) return;
    setLoading(true);
    try {
      const nextRecords = await loadAllMenus();
      const visibleRecords = filterMenuRecords(nextRecords, query);
      if (menuListRequest.current !== requestID) return;
      setRecords(nextRecords);
      setError("");
      setSelectedKey((current) => {
        if (current && visibleRecords.some((record) => record.code === current)) return current;
        return visibleRecords[0]?.code ?? "";
      });
    } catch (nextError) {
      if (menuListRequest.current !== requestID) return;
      setError(errorMessage(nextError, dictionary.menuLoadFailed));
    } finally {
      if (menuListRequest.current === requestID) setLoading(false);
    }
  }, [dictionary.menuLoadFailed]);

  useEffect(() => {
    const requestID = ++menuListRequest.current;
    const timer = window.setTimeout(() => void loadMenus(search, requestID), 250);
    return () => window.clearTimeout(timer);
  }, [loadMenus, search]);

  useEffect(() => {
    if (!selectedKey || !canRead) {
      definitionRequest.current += 1;
      setSelectedDefinition(null);
      return;
    }
    const requestID = ++definitionRequest.current;
    if (definitionRequest.current !== requestID) return;
    setDefinitionLoading(true);
    void getMenuDefinition(selectedKey)
      .then((result) => {
        if (definitionRequest.current !== requestID) return;
        setSelectedDefinition(result.definition);
        setSelectedRevision(result.revision);
        setError("");
      })
      .catch((nextError) => {
        if (definitionRequest.current !== requestID) return;
        setSelectedDefinition(null);
        setError(errorMessage(nextError, dictionary.menuLoadFailed));
      })
      .finally(() => {
        if (definitionRequest.current === requestID) setDefinitionLoading(false);
      });
  }, [canRead, definitionRefresh, dictionary.menuLoadFailed, selectedKey]);

  const directoryRecords = useMemo(() => records.filter((record) => nodeType(record) === "directory"), [records]);
  const pageRecords = useMemo(() => records.filter((record) => nodeType(record) === "page"), [records]);
  const nodes = useMemo(() => menuTreeNodes(filterMenuRecords(records, search), language), [language, records, search]);
  const componentOptions = useMemo(
    () => availableResourceRoutes.map((route) => ({ label: route, value: route.replace(/^\/+/, "") })).filter((option) => option.value),
    [availableResourceRoutes],
  );
  const excludedParentCodes = useMemo(
    () => editor?.definition ? new Set([editor.definition.node.code, ...descendantCodes(editor.definition.node.code, records)]) : new Set<string>(),
    [editor?.definition, records],
  );
  const parentOptions = useMemo(
    () => directoryRecords
      .filter((record) => !excludedParentCodes.has(record.code))
      .map((record) => ({ label: localizedTitle(record, language), value: record.code })),
    [directoryRecords, excludedParentCodes, language],
  );

  const openEditor = (mode: MenuEditorMode, trigger: HTMLElement, definition?: MenuDefinition, revision = 0) => {
    returnFocusRef.current = trigger;
    setEditor({ mode, definition, revision });
    form.setFieldsValue(definition ? editorValues(definition) : defaultEditorValues(mode, selectedDefinition));
  };

  const closeEditor = () => {
    setEditor(null);
    form.resetFields();
  };

  const restoreEditorFocus = () => {
    returnFocusRef.current?.focus({ preventScroll: true });
  };

  const saveMenu = async (values: MenuEditorValues) => {
    if (!editor) return;
    const definition = buildMenuDefinition(values, directoryMode, editor.definition);
    setSaving(true);
    try {
      if (editor.mode.startsWith("create-")) {
        await createMenuDefinition(definition, selectedRevision);
      } else {
        await replaceMenuDefinition(definition, editor.revision);
      }
      closeEditor();
      setNotice(dictionary.menuSaveSucceeded);
      setSelectedKey(definition.node.code);
      await loadMenus(search);
      setDefinitionRefresh((current) => current + 1);
    } catch (nextError) {
      setError(errorMessage(nextError, dictionary.menuSaveFailed));
    } finally {
      setSaving(false);
    }
  };

  if (!canRead) {
    return <AdminFeedback type="warning" message={dictionary.noPermission} description={dictionary.menuReadPermissionRequired} />;
  }

  const detail = definitionLoading ? (
    <AdminListPanel className="menu-governance-detail" title={dictionary.menuDetailTitle}>
      <div className="loading-panel" aria-live="polite"><Spin size="small" /></div>
    </AdminListPanel>
  ) : selectedDefinition ? (
    <MenuDefinitionDetail
      definition={selectedDefinition}
      dictionary={dictionary}
      language={language}
      canUpdate={canUpdate}
      onEdit={(trigger) => openEditor(
        selectedDefinition.node.nodeType === "directory" ? "edit-directory" : "edit-page",
        trigger,
        selectedDefinition,
        selectedRevision,
      )}
    />
  ) : (
    <AdminListPanel className="menu-governance-detail" title={dictionary.menuDetailTitle}>
      <div className="menu-governance-empty">{dictionary.emptyData}</div>
    </AdminListPanel>
  );

  return (
    <AdminPage title={dictionary.menuGovernanceTitle} description={dictionary.menuGovernanceDescription}>
      {error ? <AdminFeedback className="api-alert" type="warning" message={dictionary.menuSaveFailed} description={error} closable onClose={() => setError("")} /> : null}
      {notice ? <AdminFeedback className="api-alert" type="success" message={notice} closable onClose={() => setNotice("")} /> : null}
      <AdminTreeWorkbench
        actions={canCreate ? (
          <Space size={6} wrap>
            <AdminActionButton
              disabled={records.length > 0 && !selectedDefinition}
              icon={<FolderAddOutlined />}
              label={dictionary.menuAddDirectory}
              onClick={(event) => openEditor("create-directory", event.currentTarget)}
            >{dictionary.menuAddDirectory}</AdminActionButton>
            <AdminActionButton
              disabled={records.length > 0 && !selectedDefinition}
              icon={<FileAddOutlined />}
              label={dictionary.menuAddPage}
              onClick={(event) => openEditor("create-page", event.currentTarget)}
            >{dictionary.menuAddPage}</AdminActionButton>
          </Space>
        ) : null}
        ariaLabel={dictionary.menuTreeAriaLabel}
        title={dictionary.menuTreeTitle}
        searchLabel={dictionary.menuTreeSearch}
        searchPlaceholder={dictionary.menuTreeSearchPlaceholder}
        emptyText={dictionary.emptyData}
        nodes={nodes}
        selectedKey={selectedKey}
        searchValue={search}
        loading={loading}
        detail={detail}
        onSearchChange={setSearch}
        onSelect={setSelectedKey}
      />

      <AdminFormModal
        className="menu-governance-modal"
        open={Boolean(editor)}
        title={editorTitle(editor?.mode, dictionary)}
        width={920}
        okText={dictionary.save}
        cancelText={dictionary.cancel}
        confirmLoading={saving}
        onCancel={closeEditor}
        onOk={() => form.submit()}
        afterClose={restoreEditorFocus}
      >
        <Form<MenuEditorValues> form={form} layout="vertical" onFinish={(values) => void saveMenu(values)}>
          <fieldset className="menu-governance-form-section">
            <legend>{dictionary.menuBasicInformation}</legend>
            <div className="menu-governance-form-grid">
              <Form.Item name="code" label={dictionary.code} rules={[requiredRule(dictionary.requiredField), safeCodeRule(dictionary.menuCodeInvalid)]}>
                <Input autoComplete="off" disabled={Boolean(editor?.definition)} />
              </Form.Item>
              <Form.Item
                name="parentCode"
                label={dictionary.menuParent}
                rules={directoryMode ? [] : [requiredRule(dictionary.requiredField)]}
              >
                <Select
                  allowClear={directoryMode}
                  getPopupContainer={platformPopupContainer}
                  options={parentOptions}
                  placeholder={dictionary.menuParentPlaceholder}
                  showSearch
                  optionFilterProp="label"
                />
              </Form.Item>
              <Form.Item name="titleZh" label={dictionary.menuTitleZh} rules={[requiredRule(dictionary.requiredField)]}>
                <Input autoComplete="off" />
              </Form.Item>
              <Form.Item name="titleEn" label={dictionary.menuTitleEn} rules={[requiredRule(dictionary.requiredField)]}>
                <Input autoComplete="off" />
              </Form.Item>
              <Form.Item name="descriptionZh" label={dictionary.menuDescriptionZh}>
                <Input.TextArea autoSize={{ minRows: 2, maxRows: 4 }} />
              </Form.Item>
              <Form.Item name="descriptionEn" label={dictionary.menuDescriptionEn}>
                <Input.TextArea autoSize={{ minRows: 2, maxRows: 4 }} />
              </Form.Item>
              <Form.Item name="icon" label={dictionary.menuIcon}>
                <Input autoComplete="off" />
              </Form.Item>
              <Form.Item name="sortOrder" label={dictionary.menuSortOrder}>
                <InputNumber min={0} max={1_000_000} precision={0} />
              </Form.Item>
              <Form.Item name="status" label={dictionary.status} rules={[requiredRule(dictionary.requiredField)]}>
                <Select getPopupContainer={platformPopupContainer} options={statusOptions(dictionary)} />
              </Form.Item>
            </div>
          </fieldset>

          {!directoryMode ? (
            <>
              <fieldset className="menu-governance-form-section">
                <legend>{dictionary.menuPageSettings}</legend>
                <div className="menu-governance-switch-row">
                  <Form.Item name="external" label={dictionary.menuExternal} valuePropName="checked"><Switch /></Form.Item>
                  <Form.Item name="cacheEnabled" label={dictionary.menuCacheEnabled} valuePropName="checked"><Switch disabled={externalPage} /></Form.Item>
                  <Form.Item name="hidden" label={dictionary.menuHidden} valuePropName="checked"><Switch /></Form.Item>
                  <Form.Item name="breadcrumbVisible" label={dictionary.menuBreadcrumbVisible} valuePropName="checked"><Switch /></Form.Item>
                </div>
                {externalPage ? (
                  <div className="menu-governance-form-grid">
                    <Form.Item name="externalUrl" label={dictionary.menuExternalUrl} rules={[requiredRule(dictionary.requiredField), httpsRule(dictionary.menuHttpsRequired)]}>
                      <Input type="url" autoComplete="url" />
                    </Form.Item>
                    <Form.Item name="openMode" label={dictionary.menuOpenMode} rules={[requiredRule(dictionary.requiredField)]}>
                      <Select getPopupContainer={platformPopupContainer} options={openModeOptions(dictionary)} />
                    </Form.Item>
                  </div>
                ) : (
                  <div className="menu-governance-form-grid">
                    <Form.Item name="route" label={dictionary.menuRoute} normalize={normalizeRoute} rules={[requiredRule(dictionary.requiredField), routeRule(dictionary.menuRouteInvalid)]}>
                      <Input autoComplete="off" placeholder="/resource" />
                    </Form.Item>
                    <Form.Item name="componentKey" label={dictionary.menuComponentKey} rules={[requiredRule(dictionary.requiredField), registeredKeyRule(componentOptions, dictionary.menuRegisteredKeyRequired)]}>
                      <Select getPopupContainer={platformPopupContainer} options={componentOptions} showSearch optionFilterProp="label" />
                    </Form.Item>
                    <Form.Item name="resourceCode" label={dictionary.menuResourceCode} rules={[registeredKeyRule(componentOptions, dictionary.menuRegisteredKeyRequired, true)]}>
                      <Select allowClear getPopupContainer={platformPopupContainer} options={componentOptions} showSearch optionFilterProp="label" />
                    </Form.Item>
                    <Form.Item name="activeMenuCode" label={dictionary.menuActiveCode}>
                      <Select
                        allowClear
                        getPopupContainer={platformPopupContainer}
                        options={pageRecords
                          .filter((record) => record.status === "enabled" && record.code !== editor?.definition?.node.code)
                          .map((record) => ({ label: localizedTitle(record, language), value: record.code }))}
                        showSearch
                        optionFilterProp="label"
                      />
                    </Form.Item>
                  </div>
                )}
              </fieldset>

              <fieldset className="menu-governance-form-section">
                <legend>{dictionary.menuParameters}</legend>
                <Typography.Paragraph type="secondary">{dictionary.menuParametersDescription}</Typography.Paragraph>
                <Form.List name="parameters"
                  rules={[{ validator: async (_, parameters?: MenuParameterValue[]) => {
                    if (!parameters || parameters.length <= 32) return;
                    throw new Error(dictionary.menuParameterLimit);
                  } }]}
                >
                  {(fields, { add, remove }, { errors }) => (
                    <div className="menu-governance-form-list">
                      {fields.map((field, index) => (
                        <div className="menu-governance-form-list-row parameter-row" key={field.key}>
                          <Form.Item
                            name={[field.name, "key"]}
                            label={dictionary.menuParameterKey}
                            rules={[
                              requiredRule(dictionary.requiredField),
                              safeParameterKeyRule(dictionary.menuParameterKeyInvalid),
                              duplicateParameterKey(form, index, dictionary.menuParameterKeyDuplicate),
                            ]}
                          ><Input autoComplete="off" /></Form.Item>
                          <Form.Item name={[field.name, "type"]} label={dictionary.type} rules={[requiredRule(dictionary.requiredField)]}>
                            <Select
                              getPopupContainer={platformPopupContainer}
                              options={parameterTypeOptions(dictionary)}
                              onChange={(type: MenuParameterType) => form.setFieldValue(["parameters", field.name, "value"], defaultParameterValue(type))}
                            />
                          </Form.Item>
                          <Form.Item noStyle shouldUpdate={(previous, current) => previous.parameters?.[field.name]?.type !== current.parameters?.[field.name]?.type}>
                            {({ getFieldValue }) => parameterValueControl(getFieldValue(["parameters", field.name, "type"]), field.name, dictionary)}
                          </Form.Item>
                          <AdminActionButton danger icon={<DeleteOutlined />} label={dictionary.remove} onClick={() => remove(field.name)} />
                        </div>
                      ))}
                      <Form.ErrorList errors={errors} />
                      <AdminActionButton disabled={fields.length >= 32} icon={<PlusOutlined />} label={dictionary.menuAddParameter} onClick={() => add({ key: "", type: "string", value: "" })}>
                        {dictionary.menuAddParameter}
                      </AdminActionButton>
                    </div>
                  )}
                </Form.List>
              </fieldset>

              <fieldset className="menu-governance-form-section">
                <legend>{dictionary.menuPageButtons}</legend>
                <AdminFeedback type="info" message={dictionary.menuButtonAuthorizationBoundary} description={dictionary.menuButtonAuthorizationDescription} />
                <Form.List name="buttons">
                  {(fields, { add, remove }) => (
                    <div className="menu-governance-form-list">
                      {fields.map((field, index) => (
                        <div className="menu-governance-form-list-row button-row" key={field.key}>
                          <Form.Item
                            name={[field.name, "buttonKey"]}
                            label={dictionary.menuButtonKey}
                            rules={[requiredRule(dictionary.requiredField), safeCodeRule(dictionary.menuButtonKeyInvalid), duplicateButtonKey(form, index, dictionary.menuButtonKeyDuplicate)]}
                          ><Input autoComplete="off" /></Form.Item>
                          <Form.Item name={[field.name, "labelZh"]} label={dictionary.menuButtonLabelZh} rules={[requiredRule(dictionary.requiredField)]}><Input autoComplete="off" /></Form.Item>
                          <Form.Item name={[field.name, "labelEn"]} label={dictionary.menuButtonLabelEn} rules={[requiredRule(dictionary.requiredField)]}><Input autoComplete="off" /></Form.Item>
                          <Form.Item name={[field.name, "action"]} label={dictionary.actions} rules={[requiredRule(dictionary.requiredField), safeCodeRule(dictionary.menuButtonActionInvalid)]}><Input autoComplete="off" /></Form.Item>
                          <Form.Item name={[field.name, "permissionCode"]} label={dictionary.menuButtonPermission} rules={[requiredRule(dictionary.requiredField), safeCodeRule(dictionary.menuButtonPermissionInvalid)]}><Input autoComplete="off" /></Form.Item>
                          <Form.Item name={[field.name, "sortOrder"]} label={dictionary.menuSortOrder}><InputNumber min={0} max={1_000_000} precision={0} /></Form.Item>
                          <Form.Item name={[field.name, "status"]} label={dictionary.status} rules={[requiredRule(dictionary.requiredField)]}><Select getPopupContainer={platformPopupContainer} options={statusOptions(dictionary)} /></Form.Item>
                          <AdminActionButton danger icon={<DeleteOutlined />} label={dictionary.remove} onClick={() => remove(field.name)} />
                        </div>
                      ))}
                      <AdminActionButton
                        icon={<PlusOutlined />}
                        label={dictionary.menuAddButton}
                        onClick={() => add({ buttonKey: "", labelZh: "", labelEn: "", action: "", permissionCode: "", sortOrder: 0, status: "enabled" })}
                      >{dictionary.menuAddButton}</AdminActionButton>
                    </div>
                  )}
                </Form.List>
              </fieldset>
            </>
          ) : null}
        </Form>
      </AdminFormModal>
    </AdminPage>
  );
}

function MenuDefinitionDetail({ definition, dictionary, language, canUpdate, onEdit }: {
  definition: MenuDefinition;
  dictionary: Dictionary;
  language: Language;
  canUpdate: boolean;
  onEdit: (trigger: HTMLElement) => void;
}) {
  const node = definition.node;
  const title = language === "zh" ? node.titleZh : node.titleEn;
  return (
    <AdminListPanel
      className="menu-governance-detail"
      title={dictionary.menuDetailTitle}
      actions={canUpdate ? (
        <AdminActionButton icon={<EditOutlined />} label={dictionary.edit} onClick={(event) => onEdit(event.currentTarget)}>{dictionary.edit}</AdminActionButton>
      ) : null}
    >
      <div className="menu-governance-detail-body">
        <div className="menu-governance-detail-heading">
          <div>
            <Typography.Title level={4}>{title || definition.name}</Typography.Title>
            <Typography.Text code>{node.code}</Typography.Text>
          </div>
          <Space size={6} wrap>
            <Tag color={node.nodeType === "directory" ? "blue" : "green"}>{node.nodeType === "directory" ? dictionary.menuDirectory : dictionary.menuPage}</Tag>
            <Tag>{node.status === "enabled" ? dictionary.enabled : dictionary.disabled}</Tag>
          </Space>
        </div>
        <Descriptions bordered column={{ xs: 1, sm: 1, md: 2 }} size="small">
          <Descriptions.Item label={dictionary.menuParent}>{node.parentCode || "-"}</Descriptions.Item>
          <Descriptions.Item label={dictionary.menuSortOrder}>{node.sortOrder}</Descriptions.Item>
          <Descriptions.Item label={dictionary.menuRoute}>{node.nodeType === "page" ? node.external ? node.externalUrl : node.route : dictionary.menuDirectoryNoNavigation}</Descriptions.Item>
          <Descriptions.Item label={dictionary.menuComponentKey}>{node.componentKey || "-"}</Descriptions.Item>
          <Descriptions.Item label={dictionary.menuParameters}>{node.parameters.length}</Descriptions.Item>
          <Descriptions.Item label={dictionary.menuPageButtons}>{definition.buttons.length}</Descriptions.Item>
        </Descriptions>
        {node.nodeType === "page" ? (
          <section className="menu-governance-button-summary" aria-label={dictionary.menuPageButtons}>
            <Typography.Text strong>{dictionary.menuPageButtons}</Typography.Text>
            {definition.buttons.length > 0 ? (
              <ul>
                {definition.buttons.map((button) => (
                  <li key={button.buttonKey}>
                    <span>{language === "zh" ? button.labelZh : button.labelEn}</span>
                    <Typography.Text code>{button.permissionCode}</Typography.Text>
                  </li>
                ))}
              </ul>
            ) : <Typography.Text type="secondary">{dictionary.emptyData}</Typography.Text>}
          </section>
        ) : null}
      </div>
    </AdminListPanel>
  );
}

function buildMenuDefinition(values: MenuEditorValues, directoryMode: boolean, existing?: MenuDefinition): MenuDefinition {
  const code = values.code.trim();
  const parameters: MenuParameter[] = directoryMode ? [] : (values.parameters ?? []).map(toMenuParameter);
  const buttons: PageButton[] = directoryMode ? [] : (values.buttons ?? []).map((button) => ({
    menuCode: button.menuCode === values.code.trim() ? button.menuCode : code,
    buttonKey: button.buttonKey.trim(),
    labelZh: button.labelZh.trim(),
    labelEn: button.labelEn.trim(),
    action: button.action.trim(),
    sortOrder: button.sortOrder ?? 0,
    status: button.status,
    permissionCode: button.permissionCode.trim(),
  }));
  const external = !directoryMode && Boolean(values.external);
  const titleEn = values.titleEn.trim();
  const descriptionEn = values.descriptionEn?.trim() ?? "";
  return {
    id: existing?.id ?? `menu-${code}`,
    name: titleEn || values.titleZh.trim(),
    description: descriptionEn || values.descriptionZh?.trim() || "",
    updatedAt: existing?.updatedAt ?? "",
    node: {
      code,
      parentCode: values.parentCode?.trim() ?? "",
      nodeType: directoryMode ? "directory" : "page",
      titleZh: values.titleZh.trim(),
      titleEn,
      descriptionZh: values.descriptionZh?.trim() ?? "",
      descriptionEn,
      status: values.status,
      icon: values.icon?.trim() ?? "",
      sortOrder: values.sortOrder ?? 0,
      route: "",
      componentKey: "",
      resourceCode: "",
      external: directoryMode ? false : external,
      externalUrl: "",
      openMode: directoryMode ? "" : external ? values.openMode ?? "same-tab" : "",
      parameters: [],
      cacheEnabled: directoryMode ? false : !external && Boolean(values.cacheEnabled),
      hidden: directoryMode ? false : Boolean(values.hidden),
      activeMenuCode: directoryMode ? "" : values.activeMenuCode?.trim() ?? "",
      breadcrumbVisible: directoryMode ? false : Boolean(values.breadcrumbVisible),
      ...(!directoryMode ? {
        route: external ? "" : normalizeRoute(values.route ?? ""),
        componentKey: external ? "" : values.componentKey?.trim() ?? "",
        resourceCode: external ? "" : values.resourceCode?.trim() ?? "",
        externalUrl: external ? values.externalUrl?.trim() ?? "" : "",
        parameters,
      } : {}),
    },
    buttons: directoryMode ? [] : buttons,
  };
}

function editorValues(definition: MenuDefinition): MenuEditorValues {
  const node = definition.node;
  return {
    code: node.code,
    parentCode: node.parentCode || undefined,
    titleZh: node.titleZh,
    titleEn: node.titleEn,
    descriptionZh: node.descriptionZh,
    descriptionEn: node.descriptionEn,
    status: node.status,
    icon: node.icon,
    sortOrder: node.sortOrder,
    external: node.external,
    route: node.route,
    componentKey: node.componentKey,
    resourceCode: node.resourceCode,
    externalUrl: node.externalUrl,
    openMode: node.openMode || "same-tab",
    parameters: node.parameters.map((parameter) => ({ ...parameter })),
    cacheEnabled: node.cacheEnabled,
    hidden: node.hidden,
    activeMenuCode: node.activeMenuCode,
    breadcrumbVisible: node.breadcrumbVisible,
    buttons: definition.buttons.map((button) => ({ ...button })),
  };
}

function defaultEditorValues(mode: MenuEditorMode, selected: MenuDefinition | null): MenuEditorValues {
  const selectedDirectory = selected?.node.nodeType === "directory" ? selected.node.code : undefined;
  return {
    code: "",
    parentCode: selectedDirectory,
    titleZh: "",
    titleEn: "",
    descriptionZh: "",
    descriptionEn: "",
    status: "enabled",
    icon: "",
    sortOrder: 0,
    external: false,
    route: "",
    componentKey: "",
    resourceCode: "",
    externalUrl: "",
    openMode: "same-tab",
    parameters: [],
    cacheEnabled: true,
    hidden: false,
    activeMenuCode: "",
    breadcrumbVisible: true,
    buttons: [],
  };
}

function menuTreeNodes(records: AdminResourceRecord[], language: Language): AdminTreeWorkbenchNode[] {
  const visibleCodes = new Set(records.map((record) => record.code));
  const childCounts = new Map<string, number>();
  for (const record of records) {
    const parentCode = valueOf(record, "parentCode");
    if (parentCode) childCounts.set(parentCode, (childCounts.get(parentCode) ?? 0) + 1);
  }
  return [...records]
    .sort((left, right) => numberValue(left, "order") - numberValue(right, "order") || left.code.localeCompare(right.code))
    .map((record) => ({
      key: record.code,
      parentKey: visibleCodes.has(valueOf(record, "parentCode")) ? valueOf(record, "parentCode") : undefined,
      kind: nodeType(record) === "directory" ? "group" : "item",
      label: localizedTitle(record, language),
      searchText: `${record.code} ${record.name}`,
      status: record.status,
      childCount: nodeType(record) === "directory" ? childCounts.get(record.code) ?? 0 : undefined,
      isLeaf: nodeType(record) === "page",
    }));
}

async function loadAllMenus() {
  const records: AdminResourceRecord[] = [];
  const pageSize = 1000;
  for (let page = 1; ; page += 1) {
    const result = await queryAdminResource("menus", {
      page,
      pageSize,
      sort: [{ field: "order", order: "asc" }],
    });
    records.push(...result.items);
    if (result.items.length < pageSize || records.length >= result.total) return records;
  }
}

function filterMenuRecords(records: AdminResourceRecord[], query: string) {
  const normalized = query.trim().toLocaleLowerCase();
  if (!normalized) return records;
  const byCode = new Map(records.map((record) => [record.code, record]));
  const visible = new Set<string>();
  for (const record of records) {
    const searchText = [record.code, record.name, valueOf(record, "titleZh"), valueOf(record, "titleEn")].join(" ").toLocaleLowerCase();
    if (!searchText.includes(normalized)) continue;
    visible.add(record.code);
    let parentCode = valueOf(record, "parentCode");
    while (parentCode && !visible.has(parentCode)) {
      visible.add(parentCode);
      const parent = byCode.get(parentCode);
      parentCode = parent ? valueOf(parent, "parentCode") : "";
    }
    if (nodeType(record) === "directory") {
      descendantCodes(record.code, records).forEach((code) => visible.add(code));
    }
  }
  return records.filter((record) => visible.has(record.code));
}

function descendantCodes(code: string, records: AdminResourceRecord[]) {
  const descendants: string[] = [];
  const pending = [code];
  while (pending.length > 0) {
    const parent = pending.shift();
    for (const record of records) {
      if (valueOf(record, "parentCode") !== parent || descendants.includes(record.code)) continue;
      descendants.push(record.code);
      pending.push(record.code);
    }
  }
  return descendants;
}

function nodeType(record: AdminResourceRecord) {
  return valueOf(record, "nodeType") === "directory" ? "directory" : "page";
}

function localizedTitle(record: AdminResourceRecord, language: Language) {
  return valueOf(record, language === "zh" ? "titleZh" : "titleEn") || record.name || record.code;
}

function valueOf(record: AdminResourceRecord, key: string) {
  return record.values?.[key]?.trim() ?? "";
}

function numberValue(record: AdminResourceRecord, key: string) {
  const value = Number(valueOf(record, key));
  return Number.isFinite(value) ? value : 0;
}

function normalizeRoute(value: string) {
  const route = value.trim();
  if (!route) return "";
  return `/${route.replace(/^\/+/, "").replace(/\/{2,}/g, "/")}`;
}

function parameterValueControl(parameterType: MenuParameterType | undefined, fieldName: number, dictionary: Dictionary): ReactNode {
  if (parameterType === "number") {
    return <Form.Item name={[fieldName, "value"]} label={dictionary.menuParameterValue} rules={[requiredRule(dictionary.requiredField)]}><InputNumber /></Form.Item>;
  }
  if (parameterType === "boolean") {
    return <Form.Item name={[fieldName, "value"]} label={dictionary.menuParameterValue} rules={[requiredRule(dictionary.requiredField)]}><Select getPopupContainer={platformPopupContainer} options={[{ label: dictionary.yes, value: true }, { label: dictionary.no, value: false }]} /></Form.Item>;
  }
  return <Form.Item name={[fieldName, "value"]} label={dictionary.menuParameterValue} rules={[requiredRule(dictionary.requiredField), staticValueRule(dictionary.menuParameterValueInvalid)]}><Input autoComplete="off" /></Form.Item>;
}

function requiredRule(message: string) {
  return { required: true, message };
}

function safeCodeRule(message: string) {
  return { validator: async (_: unknown, value?: string) => {
    if (!value || SAFE_CODE.test(value.trim())) return;
    throw new Error(message);
  } };
}

function safeParameterKeyRule(message: string) {
  return { validator: async (_: unknown, value?: string) => {
    if (!value || SAFE_PARAMETER_KEY.test(value.trim()) && !FORBIDDEN_PARAMETER_INPUT.test(value.trim())) return;
    throw new Error(message);
  } };
}

function staticValueRule(message: string) {
  return { validator: async (_: unknown, value?: string) => {
    if (typeof value !== "string" || !FORBIDDEN_PARAMETER_INPUT.test(value)) return;
    throw new Error(message);
  } };
}

function duplicateParameterKey(form: ReturnType<typeof Form.useForm<MenuEditorValues>>[0], index: number, message: string) {
  return { validator: async (_: unknown, value?: string) => {
    if (!value) return;
    const parameters = (form.getFieldValue("parameters") ?? []) as MenuParameterValue[];
    const duplicate = parameters.some((parameter, parameterIndex) => parameterIndex !== index && parameter?.key?.trim() === value.trim());
    if (duplicate) throw new Error(message);
  } };
}

function duplicateButtonKey(form: ReturnType<typeof Form.useForm<MenuEditorValues>>[0], index: number, message: string) {
  return { validator: async (_: unknown, value?: string) => {
    if (!value) return;
    const buttons = (form.getFieldValue("buttons") ?? []) as MenuButtonValue[];
    const duplicate = buttons.some((button, buttonIndex) => buttonIndex !== index && button?.buttonKey?.trim() === value.trim());
    if (duplicate) throw new Error(message);
  } };
}

function routeRule(message: string) {
  return { validator: async (_: unknown, value?: string) => {
    const route = normalizeRoute(value ?? "");
    if (!route || route.startsWith("/") && !/[?#{}:*]/.test(route) && !FORBIDDEN_PARAMETER_INPUT.test(route)) return;
    throw new Error(message);
  } };
}

function httpsRule(message: string) {
  return { validator: async (_: unknown, value?: string) => {
    if (!value) return;
    try {
      if (new URL(value).protocol === "https:") return;
    } catch {
      // The localized validation message below owns recovery guidance.
    }
    throw new Error(message);
  } };
}

function registeredKeyRule(options: Array<{ value: string }>, message: string, optional = false) {
  const allowed = new Set(options.map((option) => option.value));
  return { validator: async (_: unknown, value?: string) => {
    if (optional && !value || value && allowed.has(value)) return;
    throw new Error(message);
  } };
}

function defaultParameterValue(type: MenuParameterType) {
  if (type === "number") return 0;
  if (type === "boolean") return false;
  return "";
}

function toMenuParameter(parameter: MenuParameterValue): MenuParameter {
  const key = parameter.key.trim();
  if (parameter.type === "number") return { key, type: "number", value: Number(parameter.value) };
  if (parameter.type === "boolean") return { key, type: "boolean", value: Boolean(parameter.value) };
  return { key, type: "string", value: String(parameter.value) };
}

function statusOptions(dictionary: Dictionary) {
  return [{ label: dictionary.enabled, value: "enabled" }, { label: dictionary.disabled, value: "disabled" }];
}

function openModeOptions(dictionary: Dictionary) {
  return [{ label: dictionary.menuOpenSameTab, value: "same-tab" }, { label: dictionary.menuOpenNewTab, value: "new-tab" }];
}

function parameterTypeOptions(dictionary: Dictionary) {
  return [
    { label: dictionary.menuParameterTypeString, value: "string" },
    { label: dictionary.menuParameterTypeNumber, value: "number" },
    { label: dictionary.menuParameterTypeBoolean, value: "boolean" },
  ];
}

function editorTitle(mode: MenuEditorMode | undefined, dictionary: Dictionary) {
  if (mode === "create-directory") return dictionary.menuCreateDirectory;
  if (mode === "create-page") return dictionary.menuCreatePage;
  if (mode === "edit-directory") return dictionary.menuEditDirectory;
  return dictionary.menuEditPage;
}

function errorMessage(error: unknown, fallback: string) {
  return error instanceof Error && error.message ? error.message : fallback;
}
