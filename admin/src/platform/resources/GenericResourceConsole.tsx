import {
  DeleteOutlined,
  DownloadOutlined,
  EditOutlined,
  EyeOutlined,
  FileImageOutlined,
  FilePdfOutlined,
  FileTextOutlined,
  FileUnknownOutlined,
  MoreOutlined,
  PlusOutlined,
  UploadOutlined,
} from "@ant-design/icons";
import { useCreate, useDataProvider, useDelete, useList, useUpdate, type CrudSort, type DataProvider, type HttpError } from "@refinedev/core";
import { Button, Drawer, Dropdown, Form, Input, InputNumber, Modal, Popconfirm, Select, Space, Switch, Tabs, Tag, Typography, type FormInstance, type MenuProps, type TableProps } from "antd";
import type { Rule } from "antd/es/form";
import { useCallback, useEffect, useMemo, useRef, useState, type ComponentProps, type Key, type ReactNode } from "react";
import {
  executeAdminResourceAction,
  getAdminFileBlob,
  getAdminResourceSchema,
  uploadAdminFile,
  type AdminResourceAction,
  type AdminResourceField,
  type AdminResourceInput,
  type AdminResourcePanel,
  type AdminResourceQueryCondition,
  type AdminResourceQueryInput,
  type AdminResourceQuerySort,
  type AdminResourceRecord,
  type AdminResourceSchema,
} from "../api/client";
import { dictionaries, type Dictionary, type Language } from "../i18n";
import {
  AdminActionButton,
  AdminFeedback,
  AdminFormModal,
  AdminPage,
  PlatformOverflowText,
  PlatformDataTable,
  PlatformResourceForm,
  PlatformTreeSelect,
  defaultAdminFormSlotRegistry,
  platformPopupContainer,
  type AdminFormRuntimeSlotRegistry,
  type PlatformDataTableColumn,
  type PlatformDataTableColumnPriority,
  type PlatformDataTableFilterField,
  type PlatformDataTableFilterValue,
  type PlatformResourceFormLayoutPreset,
  type PlatformResourceFormSection,
  type PlatformResourceFormSlots,
} from "../ui";
import type { AdminResourceDefinition } from "./registry";

type GenericResourceConsoleProps = {
  resource: AdminResourceDefinition;
  availableResourceRoutes?: string[];
  language: Language;
  dictionary: Dictionary;
  permissions?: string[];
  deniedPermissions?: string[];
};

type ResourceFormValues = Record<string, string | string[] | boolean | number | undefined>;

type ResourceFormSection = PlatformResourceFormSection<AdminResourceField>;

type RelationOptionResult = {
  fieldKey: string;
  options: NonNullable<AdminResourceField["options"]>;
};

type FilePreviewState = {
  recordId: string;
  status: "idle" | "loading" | "ready" | "unsupported" | "error";
  kind?: "image" | "text" | "pdf" | "unsupported";
  url?: string;
  text?: string;
  error?: string;
};

const FOCUSABLE_RESOURCE_FORM_CONTROL_SELECTOR = [
  '.resource-form-fields input:not([type="hidden"]):not([disabled]):not([readonly])',
  ".resource-form-fields textarea:not([disabled]):not([readonly])",
  ".resource-form-fields select:not([disabled])",
  '.resource-form-fields button:not([disabled]):not([aria-disabled="true"])',
  '.resource-form-fields [role="combobox"]:not([disabled]):not([readonly]):not([aria-disabled="true"])',
  '.resource-form-fields [tabindex]:not([tabindex="-1"]):not([disabled]):not([readonly]):not([aria-disabled="true"])',
].join(", ");

export function GenericResourceConsole({ resource, availableResourceRoutes = [], language, dictionary, permissions = ["*"], deniedPermissions = [] }: GenericResourceConsoleProps) {
  const resourceKey = resourceKeyFromRoute(resource.route);
  const isFileResource = resourceKey === "files";
  const hasAuditResource = availableResourceRoutes.includes("/audit-logs");
  const fallbackSchema = useMemo(() => createFallbackSchema(resourceKey, resource), [resource, resourceKey]);
  const [schema, setSchema] = useState<AdminResourceSchema>(fallbackSchema);
  const [saving, setSaving] = useState(false);
  const [fileUploading, setFileUploading] = useState(false);
  const [fileDownloadingID, setFileDownloadingID] = useState("");
  const [filePreview, setFilePreview] = useState<FilePreviewState>({ recordId: "", status: "idle" });
  const [fileAuditRecords, setFileAuditRecords] = useState<AdminResourceRecord[]>([]);
  const [fileAuditLoading, setFileAuditLoading] = useState(false);
  const [fileAuditError, setFileAuditError] = useState("");
  const [error, setError] = useState("");
  const [query, setQuery] = useState("");
  const [filters, setFilters] = useState<Record<string, PlatformDataTableFilterValue>>({});
  const [schemaReady, setSchemaReady] = useState(false);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [sortState, setSortState] = useState<AdminResourceQuerySort[]>([]);
  const [selectedID, setSelectedID] = useState("");
  const [editingRecord, setEditingRecord] = useState<AdminResourceRecord | null>(null);
  const [modalOpen, setModalOpen] = useState(false);
  const [detailOpen, setDetailOpen] = useState(false);
  const [detailRecordID, setDetailRecordID] = useState("");
  const [issuedToken, setIssuedToken] = useState("");
  const [togglingRecordID, setTogglingRecordID] = useState("");
  const [actionExecutingKey, setActionExecutingKey] = useState("");
  const [form] = Form.useForm<ResourceFormValues>();
  const watchedFormValues = Form.useWatch([], form) as ResourceFormValues | undefined;
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const dataProvider = useDataProvider();

  const relationOptionSignature = useMemo(() => relationSignature(schema.fields), [schema.fields]);
  const tableFields = useMemo(() => schema.fields.filter((field) => field.inTable), [schema.fields]);
  const formFields = useMemo(() => schema.fields.filter((field) => field.inForm && !field.readOnly), [schema.fields]);
  const formSections = useMemo(() => resourceFormSections(formFields, schema.formGroups ?? [], language, dictionary), [dictionary, formFields, language, schema.formGroups]);
  const formLayoutPreset = useMemo(() => normalizeFormLayoutPreset(schema.formLayout, formFields), [formFields, schema.formLayout]);
  const activeFormInitialValues = useMemo(() => (editingRecord ? formValuesFromRecord(editingRecord, formFields) : defaultFormValues(formFields)), [editingRecord, formFields]);
  const runtimeFormValues = useMemo(() => ({ ...activeFormInitialValues, ...(watchedFormValues ?? {}) }), [activeFormInitialValues, watchedFormValues]);
  const detailFields = useMemo(() => schema.fields.filter((field) => field.inDetail), [schema.fields]);
  const canCreate = useMemo(() => permissionAllows(permissions, schema.permissions.create, deniedPermissions), [deniedPermissions, permissions, schema.permissions.create]);
  const canUpdate = useMemo(() => permissionAllows(permissions, schema.permissions.update, deniedPermissions), [deniedPermissions, permissions, schema.permissions.update]);
  const canDelete = useMemo(() => permissionAllows(permissions, schema.permissions.delete, deniedPermissions), [deniedPermissions, permissions, schema.permissions.delete]);
  const filterFields = useMemo(() => resourceFilterFields(schema.fields, language, dictionary), [dictionary, language, schema.fields]);
  const customRowActions = useMemo(
    () => (schema.actions ?? []).filter((action) => action.kind === "row" && permissionAllows(permissions, action.permission, deniedPermissions)),
    [deniedPermissions, permissions, schema.actions],
  );
  const customBatchActions = useMemo(
    () => (schema.actions ?? []).filter((action) => action.kind === "batch" && permissionAllows(permissions, action.permission, deniedPermissions)),
    [deniedPermissions, permissions, schema.actions],
  );
  const customPanels = useMemo(
    () => (schema.panels ?? []).filter((panel) => !panel.permission || permissionAllows(permissions, panel.permission, deniedPermissions)),
    [deniedPermissions, permissions, schema.panels],
  );
  const runtimeFormSlots = useMemo(
    () =>
      buildRuntimeFormSlots({
        descriptors: schema.runtimeSlots ?? [],
        registry: defaultAdminFormSlotRegistry,
        dictionary,
        language,
        fields: formFields,
        sections: formSections,
        record: editingRecord,
        formValues: runtimeFormValues,
        schemaPermissions: schema.permissions,
        permissions,
        deniedPermissions,
      }),
    [deniedPermissions, dictionary, editingRecord, formFields, formSections, language, permissions, runtimeFormValues, schema.permissions, schema.runtimeSlots],
  );
  const resourceQuery = useMemo(
    () => buildResourceQuery(schema, query, filters, page, pageSize, sortState),
    [filters, page, pageSize, query, schema, sortState],
  );
  const listQuery = useList<AdminResourceRecord, HttpError, AdminResourceRecord>({
    resource: resourceKey,
    pagination: {
      currentPage: page,
      pageSize,
      mode: "server",
    },
    sorters: querySortToCrudSort(sortState),
    meta: {
      keywords: resourceQuery.keywords,
      conditions: resourceQuery.conditions,
    },
    queryOptions: {
      enabled: schemaReady,
    },
  });
  const createResource = useCreate<AdminResourceRecord, HttpError, AdminResourceInput>();
  const updateResource = useUpdate<AdminResourceRecord, HttpError, AdminResourceInput>();
  const deleteResource = useDelete<AdminResourceRecord, HttpError, AdminResourceInput>();
  const items = useMemo(() => listQuery.result.data ?? [], [listQuery.result.data]);
  const total = listQuery.result.total ?? items.length;
  const loading = !schemaReady || listQuery.query.isLoading || listQuery.query.isFetching;
  const listError = listQuery.query.error;
  const errorMessage = error || (listError instanceof Error ? listError.message : "");

  useEffect(() => {
    let cancelled = false;
    setSchema(fallbackSchema);
    setSchemaReady(false);
    setSelectedID("");
    setQuery("");
    setFilters({});
    setPage(1);
    setPageSize(10);
    setSortState([]);
    getAdminResourceSchema(resourceKey)
      .then((nextSchema) => {
        if (cancelled) {
          return;
        }
        setSchema(normalizeResourceSchema(nextSchema));
        setError("");
      })
      .catch((nextError: unknown) => {
        if (cancelled) {
          return;
        }
        setError(nextError instanceof Error ? nextError.message : dictionary.loadResourceFailed);
      })
      .finally(() => {
        if (!cancelled) {
          setSchemaReady(true);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [dictionary.loadResourceFailed, fallbackSchema, resourceKey]);

  useEffect(() => {
    const relationFields = schema.fields.filter(hasRelationSource);
    if (!schemaReady || relationFields.length === 0) {
      return;
    }
    let cancelled = false;
    const provider = dataProvider();
    Promise.all(relationFields.map((field) => loadRelationOptions(field, provider)))
      .then((results) => {
        if (cancelled) {
          return;
        }
        setSchema((currentSchema) => mergeRelationOptions(currentSchema, results));
      })
      .catch((nextError: unknown) => {
        if (cancelled) {
          return;
        }
        setError(nextError instanceof Error ? nextError.message : dictionary.relationOptionsLoadFailed);
      });
    return () => {
      cancelled = true;
    };
  }, [dataProvider, dictionary.relationOptionsLoadFailed, relationOptionSignature, schemaReady]);

  const refreshResource = useCallback(async () => {
    if (!schemaReady) {
      return;
    }
    await listQuery.query.refetch();
  }, [listQuery.query, schemaReady]);

  useEffect(() => {
    if (items.length === 0) {
      setSelectedID("");
      return;
    }
    setSelectedID((current) => (current && items.some((item) => item.id === current) ? current : items[0]?.id ?? ""));
  }, [items]);

  const selectedRecord = useMemo(
    () => items.find((item) => item.id === selectedID) ?? items[0],
    [items, selectedID],
  );
  const detailRecord = useMemo(
    () => items.find((item) => item.id === detailRecordID) ?? selectedRecord,
    [detailRecordID, items, selectedRecord],
  );
  const canReadAuditLogs = useMemo(() => hasAuditResource && permissionAllows(permissions, "admin:audit-log:read", deniedPermissions), [deniedPermissions, hasAuditResource, permissions]);
  const openCreate = () => {
    setEditingRecord(null);
    setModalOpen(true);
  };

  const openEdit = (record: AdminResourceRecord) => {
    setEditingRecord(record);
    setModalOpen(true);
  };

  const openDetail = (record: AdminResourceRecord) => {
    setSelectedID(record.id);
    setDetailRecordID(record.id);
    setDetailOpen(true);
  };

  useEffect(() => {
    if (!modalOpen) {
      return;
    }
    if (editingRecord) {
      form.setFieldsValue(formValuesFromRecord(editingRecord, formFields));
      return;
    }
    form.resetFields();
    form.setFieldsValue(defaultFormValues(formFields));
  }, [editingRecord, form, formFields, modalOpen]);

  const submitForm = async (values: ResourceFormValues) => {
    const input = inputFromFormValues(values, formFields);
    setSaving(true);
    try {
      const result = editingRecord
        ? await updateResource.mutateAsync({ resource: resourceKey, id: editingRecord.id, values: input })
        : await createResource.mutateAsync({ resource: resourceKey, values: input });
      await refreshResource();
      setSelectedID(String(result.data.id));
      setModalOpen(false);
      const issuedToken = issuedTokenFromRefineRecord(result.data);
      if (issuedToken) {
        setIssuedToken(issuedToken);
      }
      setError("");
    } catch (nextError) {
      setError(nextError instanceof Error ? nextError.message : dictionary.loadResourceFailed);
    } finally {
      setSaving(false);
    }
  };

  const removeRecord = async (record: AdminResourceRecord) => {
    try {
      await deleteResource.mutateAsync({ resource: resourceKey, id: record.id });
      await refreshResource();
      setSelectedID((current) => (current === record.id ? "" : current));
      setError("");
    } catch (nextError) {
      setError(nextError instanceof Error ? nextError.message : dictionary.loadResourceFailed);
    }
  };

  const removeSelectedRecords = async (selectedKeys: Key[], clearSelection: () => void) => {
    try {
      for (const key of selectedKeys) {
        await deleteResource.mutateAsync({ resource: resourceKey, id: String(key) });
      }
      const selected = new Set(selectedKeys.map(String));
      await refreshResource();
      setSelectedID((current) => (selected.has(current) ? "" : current));
      clearSelection();
      setError("");
    } catch (nextError) {
      setError(nextError instanceof Error ? nextError.message : dictionary.loadResourceFailed);
    }
  };

  const uploadFile = async (file: File) => {
    setFileUploading(true);
    try {
      const result = await uploadAdminFile(file);
      await refreshResource();
      setSelectedID(String(result.record.id));
      setDetailRecordID(String(result.record.id));
      setDetailOpen(true);
      setError("");
    } catch (nextError) {
      setError(nextError instanceof Error ? nextError.message : dictionary.fileUploadFailed);
    } finally {
      setFileUploading(false);
    }
  };

  const handleFileInputChange: ComponentProps<"input">["onChange"] = (event) => {
    const file = event.currentTarget.files?.[0];
    event.currentTarget.value = "";
    if (file) {
      void uploadFile(file);
    }
  };

  const loadFilePreview = useCallback(async (record: AdminResourceRecord) => {
    const kind = filePreviewKind(record);
    if (kind === "unsupported") {
      setFilePreview({ recordId: record.id, status: "unsupported", kind: "unsupported" });
      return;
    }
    setFilePreview((current) => {
      if (current.url) {
        URL.revokeObjectURL(current.url);
      }
      return { recordId: record.id, status: "loading", kind };
    });
    try {
      const blob = await getAdminFileBlob(record.id);
      if (kind === "text") {
        const text = await blob.text();
        setFilePreview({ recordId: record.id, status: "ready", kind, text: text.slice(0, 16000) });
        return;
      }
      setFilePreview({ recordId: record.id, status: "ready", kind, url: URL.createObjectURL(blob) });
    } catch (nextError) {
      setFilePreview({
        recordId: record.id,
        status: "error",
        kind,
        error: nextError instanceof Error ? nextError.message : dictionary.filePreviewFailed,
      });
    }
  }, [dictionary.filePreviewFailed]);

  const downloadFile = useCallback(async (record: AdminResourceRecord) => {
    setFileDownloadingID(record.id);
    try {
      const blob = await getAdminFileBlob(record.id);
      downloadBlob(blob, fileRecordName(record));
      setError("");
    } catch (nextError) {
      setError(nextError instanceof Error ? nextError.message : dictionary.fileDownloadFailed);
    } finally {
      setFileDownloadingID("");
    }
  }, [dictionary.fileDownloadFailed]);

  useEffect(() => {
    if (!isFileResource || !detailOpen || !detailRecord) {
      return;
    }
    if (filePreview.recordId === detailRecord.id && filePreview.status !== "idle") {
      return;
    }
    void loadFilePreview(detailRecord);
  }, [detailOpen, detailRecord, filePreview.recordId, filePreview.status, isFileResource, loadFilePreview]);

  useEffect(() => {
    if (!isFileResource || !detailOpen || !detailRecord?.id || !canReadAuditLogs) {
      setFileAuditRecords([]);
      setFileAuditError("");
      setFileAuditLoading(false);
      return;
    }
    let cancelled = false;
    const provider = dataProvider();
    setFileAuditLoading(true);
    setFileAuditError("");
    provider.getList<AdminResourceRecord>({
      resource: "audit-logs",
      pagination: {
        currentPage: 1,
        pageSize: 8,
        mode: "server",
      },
      sorters: [{ field: "createdAt", order: "desc" }],
      meta: {
        conditions: [{ field: "targetId", operator: "=", value: detailRecord.id }],
      },
    })
      .then((result) => {
        if (!cancelled) {
          setFileAuditRecords(result.data);
        }
      })
      .catch((nextError: unknown) => {
        if (!cancelled) {
          setFileAuditRecords([]);
          setFileAuditError(nextError instanceof Error ? nextError.message : dictionary.fileAuditLoadFailed);
        }
      })
      .finally(() => {
        if (!cancelled) {
          setFileAuditLoading(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [canReadAuditLogs, dataProvider, detailOpen, detailRecord?.id, dictionary.fileAuditLoadFailed, isFileResource]);

  useEffect(() => () => {
    if (filePreview.url) {
      URL.revokeObjectURL(filePreview.url);
    }
  }, [filePreview.url]);

  const toggleRecordStatus = async (record: AdminResourceRecord, checked: boolean) => {
    const nextStatus = checked ? "enabled" : "disabled";
    if (record.status === nextStatus) {
      return;
    }
    setTogglingRecordID(record.id);
    try {
      const result = await updateResource.mutateAsync({ resource: resourceKey, id: record.id, values: inputFromRecord(record, { status: nextStatus }) });
      await refreshResource();
      setSelectedID(String(result.data.id));
      setError("");
    } catch (nextError) {
      setError(nextError instanceof Error ? nextError.message : dictionary.loadResourceFailed);
    } finally {
      setTogglingRecordID("");
    }
  };

  const runCustomAction = async (action: AdminResourceAction, record: AdminResourceRecord) => {
    const actionLabel = localizedText(action.label, language);
    if (!action.route) {
      setError(formatTemplate(dictionary.customActionUnavailable, { action: actionLabel, record: record.code }));
      return;
    }
    setActionExecutingKey(actionExecutionKey(action, record));
    try {
      const result = await executeAdminResourceAction(action, record, { source: "admin-resource-console" });
      if (action.refresh !== false) {
        await refreshResource();
      }
      if (result.record?.id) {
        setSelectedID(String(result.record.id));
      }
      setError(formatTemplate(dictionary.customActionSucceeded, { action: actionLabel, record: record.code }));
    } catch (nextError) {
      setError(nextError instanceof Error ? nextError.message : dictionary.customActionFailed);
    } finally {
      setActionExecutingKey("");
    }
  };

  const runBatchCustomAction = async (action: AdminResourceAction, selectedKeys: Key[], clearSelection: () => void) => {
    const actionLabel = localizedText(action.label, language);
    if (!action.route) {
      setError(formatTemplate(dictionary.customBatchActionUnavailable, { action: actionLabel, count: String(selectedKeys.length) }));
      return;
    }
    setActionExecutingKey(actionExecutionKey(action));
    try {
      for (const key of selectedKeys) {
        // Batch actions reuse row-action route templates until a capability declares a dedicated batch endpoint.
        await executeAdminResourceAction(action, { id: String(key), code: String(key), name: String(key), status: "", updatedAt: "" }, { source: "admin-resource-console" });
      }
      if (action.refresh !== false) {
        await refreshResource();
      }
      clearSelection();
      setError(formatTemplate(dictionary.customActionSucceeded, { action: actionLabel, record: String(selectedKeys.length) }));
    } catch (nextError) {
      setError(nextError instanceof Error ? nextError.message : dictionary.customActionFailed);
    } finally {
      setActionExecutingKey("");
    }
  };

  const renderResourceRowActions = (record: AdminResourceRecord) => (
    <Space size={4}>
      <AdminActionButton
        icon={<EyeOutlined />}
        label={dictionary.viewRecord}
        size="small"
        type="text"
        onClick={(event) => {
          event.stopPropagation();
          openDetail(record);
        }}
      />
      {isFileResource ? (
        <AdminActionButton
          icon={<DownloadOutlined />}
          label={dictionary.fileDownload}
          loading={fileDownloadingID === record.id}
          size="small"
          type="text"
          onClick={(event) => {
            event.stopPropagation();
            void downloadFile(record);
          }}
        />
      ) : null}
      {canUpdate ? (
        <AdminActionButton
          icon={<EditOutlined />}
          label={dictionary.editRecord}
          size="small"
          type="text"
          onClick={(event) => {
            event.stopPropagation();
            openEdit(record);
          }}
        />
      ) : null}
      {canDelete ? (
        <Popconfirm title={dictionary.deleteConfirm} okText={dictionary.deleteRecord} cancelText={dictionary.cancel} onConfirm={() => removeRecord(record)}>
          <AdminActionButton
            danger
            icon={<DeleteOutlined />}
            label={dictionary.deleteRecord}
            size="small"
            type="text"
            onClick={(event) => event.stopPropagation()}
          />
        </Popconfirm>
      ) : null}
      <CustomRowActionOverflow
        actions={customRowActions}
        dictionary={dictionary}
        executingKey={actionExecutingKey}
        language={language}
        onExecute={runCustomAction}
        record={record}
        onUnavailable={(message) => setError(message)}
      />
    </Space>
  );

  const columns: PlatformDataTableColumn<AdminResourceRecord>[] = [
    ...tableFields.map((field, index) => ({
      title: localizedText(field.label, language),
      key: field.key,
      priority: tableColumnPriority(index),
      width: field.width,
      ellipsis: field.type === "textarea",
      dataIndex: field.key,
      sorter: isSortableResourceField(field),
      sortOrder: tableSortOrder(sortState, field.key),
      render: (_: unknown, record: AdminResourceRecord) =>
        renderFieldValue(record, field, language, dictionary, {
          canToggleStatus: canUpdate && canToggleStatusField(field, record),
          toggling: togglingRecordID === record.id,
          onToggleStatus: toggleRecordStatus,
        }),
    })),
  ];

  const handleTableChange: TableProps<AdminResourceRecord>["onChange"] = (nextPagination, _tableFilters, sorter) => {
    setPage(nextPagination.current ?? 1);
    setPageSize(nextPagination.pageSize ?? 10);
    setSortState(sortersFromTableChange(sorter));
  };

  return (
    <AdminPage
      className="generic-resource-console"
      title={resource.title[language]}
    >
      {errorMessage ? <AdminFeedback className="api-alert" type="warning" message={dictionary.loadResourceFailed} description={errorMessage} /> : null}

      <div className="resource-grid single">
        <PlatformDataTable
          title={dictionary.resourceList}
          columns={columns}
          dataSource={items}
          rowKey="id"
          loading={loading}
          selectedRowKey={selectedRecord?.id}
          searchValue={query}
          searchPlaceholder={dictionary.searchResource}
          labels={{
            search: dictionary.searchResource,
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
            visibleColumns: (visible, total) => formatTemplate(dictionary.visibleColumns, { visible: String(visible), total: String(total) }),
            selectAllColumns: dictionary.selectAllColumns,
            resetColumns: dictionary.resetColumns,
          }}
          filterFields={filterFields}
          filterValues={filters}
          rowActions={renderResourceRowActions}
          rowActionsColumnTitle={dictionary.actions}
          rowActionsColumnWidth={isFileResource ? 196 : 164}
          actions={
            canCreate ? (
              isFileResource ? (
                <>
                  <input
                    ref={fileInputRef}
                    aria-label={dictionary.fileUploadInput}
                    className="file-upload-input"
                    type="file"
                    onChange={handleFileInputChange}
                  />
                  <AdminActionButton
                    icon={<UploadOutlined />}
                    label={dictionary.fileUpload}
                    loading={fileUploading}
                    type="primary"
                    onClick={() => fileInputRef.current?.click()}
                  >
                    {dictionary.fileUpload}
                  </AdminActionButton>
                </>
              ) : (
                <AdminActionButton icon={<PlusOutlined />} label={dictionary.addRecord} type="primary" onClick={openCreate}>
                  {dictionary.addRecord}
                </AdminActionButton>
              )
            ) : null
          }
          batchActions={(selectedKeys, clearSelection) =>
            (
              <>
                <CustomBatchActions
                  actions={customBatchActions}
                  dictionary={dictionary}
                  executingKey={actionExecutingKey}
                  language={language}
                  onExecute={runBatchCustomAction}
                  selectedKeys={selectedKeys}
                  clearSelection={clearSelection}
                  onUnavailable={(message) => setError(message)}
                />
                {canDelete ? (
                  <Popconfirm
                    title={dictionary.deleteConfirm}
                    okText={dictionary.deleteRecord}
                    cancelText={dictionary.cancel}
                    onConfirm={() => removeSelectedRecords(selectedKeys, clearSelection)}
                  >
                    <Button danger icon={<DeleteOutlined />} size="small">
                      {dictionary.deleteRecord}
                    </Button>
                  </Popconfirm>
                ) : null}
              </>
            )
          }
          pagination={{
            current: page,
            pageSize,
            total,
            showTotal: (total, range) => tableRangeLabel(total, range, dictionary),
          }}
          mobileCards={(items) =>
            items.map((item) => (
              <button
                className={item.id === selectedRecord?.id ? "mobile-resource-card active" : "mobile-resource-card"}
                key={item.id}
                type="button"
                onClick={() => openDetail(item)}
              >
                {isFileResource ? (
                  <FileMobileCardContent dictionary={dictionary} language={language} record={item} />
                ) : (
                  <>
                    <span>
                      <strong>{localizedRecordValue(item, "name", language)}</strong>
                      <em>{item.code}</em>
                    </span>
                    <StatusTag status={item.status} dictionary={dictionary} />
                  </>
                )}
              </button>
            ))
          }
          onSearchChange={(value) => {
            setQuery(value);
            setPage(1);
          }}
          onFilterChange={(key, value) => {
            setFilters((current) => ({ ...current, [key]: value }));
            setPage(1);
          }}
          onClearFilters={() => {
            setFilters({});
            setPage(1);
          }}
          onRefresh={refreshResource}
          onRowClick={(record) => setSelectedID(record.id)}
          onTableChange={handleTableChange}
        />
      </div>

      <Drawer
        className="resource-detail-drawer"
        title={dictionary.resourceDetail}
        getContainer={false}
        open={detailOpen}
        placement="right"
        rootStyle={{ position: "absolute" }}
        width={520}
        onClose={() => setDetailOpen(false)}
      >
        <ResourceInspector
          detailFields={detailFields}
          dictionary={dictionary}
          fileAuditError={isFileResource && !hasAuditResource ? dictionary.fileAuditNotExposed : fileAuditError}
          fileAuditLoading={fileAuditLoading}
          fileAuditRecords={fileAuditRecords}
          filePreview={filePreview}
          isFileResource={isFileResource}
          language={language}
          permissions={schema.permissions}
          panels={customPanels}
          record={detailRecord}
          canUpdate={canUpdate}
          onDownload={(record) => void downloadFile(record)}
          onEdit={(record) => {
            setDetailOpen(false);
            openEdit(record);
          }}
          onReloadPreview={(record) => void loadFilePreview(record)}
        />
      </Drawer>

      <AdminFormModal
        title={editingRecord ? dictionary.editRecord : dictionary.addRecord}
        open={modalOpen}
        width={formModalWidth(formLayoutPreset)}
        okText={dictionary.save}
        cancelText={dictionary.cancel}
        confirmLoading={saving}
        afterOpenChange={(open) => {
          if (!open) {
            return;
          }
          requestAnimationFrame(() => {
            focusFirstEditableFormField(form, formFields);
          });
        }}
        onCancel={() => setModalOpen(false)}
        onOk={() => form.submit()}
      >
        <PlatformResourceForm
          form={form}
          initialValues={activeFormInitialValues}
          key={editingRecord?.id ?? "create"}
          layoutPreset={formLayoutPreset}
          sections={formSections}
          renderField={(field) => <FieldInput field={field} language={language} />}
          renderFieldExtra={(field) => (field.help ? localizedText(field.help, language) : undefined)}
          renderFieldLabel={(field) => localizedText(field.label, language)}
          rules={(field) => fieldRules(field, language, dictionary)}
          slots={runtimeFormSlots}
          getValuePropName={(field) => (field.type === "switch" ? "checked" : undefined)}
          onFinish={submitForm}
        />
      </AdminFormModal>
      <Modal
        className="one-time-secret-modal"
        title={dictionary.oneTimeSecretTitle}
        open={Boolean(issuedToken)}
        okText={dictionary.copy}
        cancelText={dictionary.close}
        onCancel={() => setIssuedToken("")}
        onOk={() => {
          copyIssuedToken(issuedToken);
          setIssuedToken("");
        }}
      >
        <AdminFeedback type="warning" message={dictionary.oneTimeSecretWarning} description={dictionary.oneTimeSecretDescription} />
        <Input.TextArea readOnly rows={3} value={issuedToken} />
      </Modal>
    </AdminPage>
  );
}

function ResourceInspector({
  detailFields,
  record,
  dictionary,
  fileAuditError,
  fileAuditLoading,
  fileAuditRecords,
  filePreview,
  isFileResource,
  language,
  permissions,
  panels,
  canUpdate,
  onDownload,
  onEdit,
  onReloadPreview,
}: {
  detailFields: AdminResourceField[];
  record?: AdminResourceRecord;
  dictionary: Dictionary;
  fileAuditError: string;
  fileAuditLoading: boolean;
  fileAuditRecords: AdminResourceRecord[];
  filePreview: FilePreviewState;
  isFileResource: boolean;
  language: Language;
  permissions: AdminResourceSchema["permissions"];
  panels: AdminResourcePanel[];
  canUpdate: boolean;
  onDownload: (record: AdminResourceRecord) => void;
  onEdit: (record: AdminResourceRecord) => void;
  onReloadPreview: (record: AdminResourceRecord) => void;
}) {
  if (!record) {
    return <aside className="resource-inspector empty" />;
  }
  if (isFileResource) {
    return (
      <FileResourceInspector
        detailFields={detailFields}
        dictionary={dictionary}
        fileAuditError={fileAuditError}
        fileAuditLoading={fileAuditLoading}
        fileAuditRecords={fileAuditRecords}
        filePreview={filePreview}
        language={language}
        record={record}
        canUpdate={canUpdate}
        onDownload={onDownload}
        onEdit={onEdit}
        onReloadPreview={onReloadPreview}
      />
    );
  }
  return (
    <aside className="resource-inspector">
      <div className="inspector-header">
        <div>
          <Typography.Title level={3}>{localizedRecordValue(record, "name", language)}</Typography.Title>
          <StatusTag status={record.status} dictionary={dictionary} />
        </div>
      </div>
      <Tabs
        className="resource-inspector-tabs"
        items={[
          {
            key: "fields",
            label: dictionary.detailPanel,
            children: <DetailFieldsPanel detailFields={detailFields} dictionary={dictionary} language={language} record={record} />,
          },
          {
            key: "permissions",
            label: dictionary.permissionPanel,
            children: <PermissionPanel permissions={permissions} />,
          },
          ...panels
            .slice()
            .sort((left, right) => (left.order ?? 0) - (right.order ?? 0))
            .map((panel) => ({
              key: panel.key,
              label: localizedText(panel.label, language),
              children: <ResourceCustomPanel dictionary={dictionary} language={language} panel={panel} record={record} />,
            })),
        ]}
        size="small"
      />
      {canUpdate ? (
        <div className="inspector-actions">
          <AdminActionButton icon={<EditOutlined />} label={dictionary.editRecord} type="primary" onClick={() => onEdit(record)}>
            {dictionary.editRecord}
          </AdminActionButton>
        </div>
      ) : null}
    </aside>
  );
}

function FileResourceInspector({
  detailFields,
  record,
  dictionary,
  fileAuditError,
  fileAuditLoading,
  fileAuditRecords,
  filePreview,
  language,
  canUpdate,
  onDownload,
  onEdit,
  onReloadPreview,
}: {
  detailFields: AdminResourceField[];
  record: AdminResourceRecord;
  dictionary: Dictionary;
  fileAuditError: string;
  fileAuditLoading: boolean;
  fileAuditRecords: AdminResourceRecord[];
  filePreview: FilePreviewState;
  language: Language;
  canUpdate: boolean;
  onDownload: (record: AdminResourceRecord) => void;
  onEdit: (record: AdminResourceRecord) => void;
  onReloadPreview: (record: AdminResourceRecord) => void;
}) {
  const previewKind = filePreviewKind(record);
  return (
    <aside className="resource-inspector file-resource-inspector">
      <div className="inspector-header file-inspector-header">
        <div className={`file-preview-icon kind-${previewKind}`}>
          <FileKindIcon kind={previewKind} />
        </div>
        <div>
          <Typography.Title level={3}>{fileRecordName(record, language)}</Typography.Title>
          <div className="file-inspector-meta">
            <Tag>{fileMimeType(record) || dictionary.fileTypeUnknown}</Tag>
            <Tag>{formatBytes(fileSize(record))}</Tag>
            <StatusTag status={record.status} dictionary={dictionary} />
          </div>
        </div>
      </div>
      <Tabs
        className="resource-inspector-tabs file-inspector-tabs"
        items={[
          {
            key: "metadata",
            label: dictionary.fileMetadata,
            children: <FileMetadataPanel detailFields={detailFields} dictionary={dictionary} language={language} record={record} />,
          },
          {
            key: "preview",
            label: dictionary.filePreview,
            children: <FilePreviewPanel dictionary={dictionary} language={language} preview={filePreview} record={record} onReload={() => onReloadPreview(record)} />,
          },
          {
            key: "audit",
            label: dictionary.fileAudit,
            children: (
              <FileAuditPanel
                dictionary={dictionary}
                error={fileAuditError}
                language={language}
                loading={fileAuditLoading}
                records={fileAuditRecords}
              />
            ),
          },
        ]}
        size="small"
      />
      <div className="inspector-actions file-inspector-actions">
        <AdminActionButton icon={<DownloadOutlined />} label={dictionary.fileDownload} onClick={() => onDownload(record)}>
          {dictionary.fileDownload}
        </AdminActionButton>
        {canUpdate ? (
          <AdminActionButton icon={<EditOutlined />} label={dictionary.editRecord} type="primary" onClick={() => onEdit(record)}>
            {dictionary.editRecord}
          </AdminActionButton>
        ) : null}
      </div>
    </aside>
  );
}

function FileMetadataPanel({
  detailFields,
  record,
  dictionary,
  language,
}: {
  detailFields: AdminResourceField[];
  record: AdminResourceRecord;
  dictionary: Dictionary;
  language: Language;
}) {
  const fields = detailFields.filter((field) => !["status"].includes(field.key));
  return (
    <section className="file-metadata-panel">
      <div className="file-metadata-grid">
        <FileMetadataItem label={dictionary.fileName} value={fileRecordName(record, language)} />
        <FileMetadataItem label={dictionary.fileMimeType} value={fileMimeType(record) || dictionary.fileTypeUnknown} />
        <FileMetadataItem label={dictionary.fileSize} value={formatBytes(fileSize(record))} />
        <FileMetadataItem label={dictionary.fileStorageDriver} value={fileStorageDriver(record) || "-"} />
      </div>
      <DetailFieldsPanel detailFields={fields} dictionary={dictionary} language={language} record={record} />
    </section>
  );
}

function FileMetadataItem({ label, value }: { label: string; value: string }) {
  return (
    <div className="file-metadata-item">
      <span>{label}</span>
      <PlatformOverflowText value={value} />
    </div>
  );
}

function FilePreviewPanel({
  dictionary,
  language,
  preview,
  record,
  onReload,
}: {
  dictionary: Dictionary;
  language: Language;
  preview: FilePreviewState;
  record: AdminResourceRecord;
  onReload: () => void;
}) {
  const kind = filePreviewKind(record);
  if (preview.status === "loading" || (preview.recordId !== record.id && kind !== "unsupported")) {
    return <AdminFeedback type="info" message={dictionary.filePreviewLoading} description={dictionary.filePreviewLoadingDescription} />;
  }
  if (preview.status === "error") {
    return (
      <AdminFeedback
        type="warning"
        message={dictionary.filePreviewFailed}
        description={preview.error || dictionary.filePreviewUnsupportedDescription}
        action={<Button size="small" onClick={onReload}>{dictionary.refresh}</Button>}
      />
    );
  }
  if (preview.status === "unsupported" || kind === "unsupported") {
    return (
      <div className="file-preview-fallback">
        <FileUnknownOutlined />
        <Typography.Text strong>{dictionary.filePreviewUnsupportedTitle}</Typography.Text>
        <Typography.Text type="secondary">{dictionary.filePreviewUnsupportedDescription}</Typography.Text>
      </div>
    );
  }
  if (preview.kind === "image" && preview.url) {
    return (
      <div className="file-preview-surface image">
        <img alt={formatTemplate(dictionary.filePreviewImageAlt, { name: fileRecordName(record, language) })} src={preview.url} />
      </div>
    );
  }
  if (preview.kind === "pdf" && preview.url) {
    return (
      <div className="file-preview-surface pdf">
        <iframe src={preview.url} title={formatTemplate(dictionary.filePreviewPdfTitle, { name: fileRecordName(record, language) })} />
      </div>
    );
  }
  if (preview.kind === "text") {
    return (
      <div className="file-preview-surface text">
        <Typography.Text strong>{dictionary.filePreviewTextTitle}</Typography.Text>
        <pre>{preview.text || ""}</pre>
      </div>
    );
  }
  return <AdminFeedback type="info" message={dictionary.filePreviewLoading} description={dictionary.filePreviewLoadingDescription} />;
}

function FileAuditPanel({
  dictionary,
  error,
  language,
  loading,
  records,
}: {
  dictionary: Dictionary;
  error: string;
  language: Language;
  loading: boolean;
  records: AdminResourceRecord[];
}) {
  if (loading) {
    return <AdminFeedback type="info" message={dictionary.fileAuditLoading} description={dictionary.fileAuditLoadingDescription} />;
  }
  if (error) {
    return (
      <div className="file-audit-fallback">
        <AdminFeedback
          type="info"
          message={dictionary.fileAuditUnavailable}
          description={formatTemplate(dictionary.fileAuditUnavailableDescription, { error })}
        />
        <FileExpectedAuditEvents dictionary={dictionary} />
      </div>
    );
  }
  if (records.length === 0) {
    return (
      <div className="file-audit-fallback">
        <AdminFeedback type="info" message={dictionary.fileAuditEmpty} description={dictionary.fileAuditEmptyDescription} />
        <FileExpectedAuditEvents dictionary={dictionary} />
      </div>
    );
  }
  return (
    <div className="file-audit-timeline">
      {records.map((record) => (
        <div className="file-audit-row" key={record.id}>
          <span className="timeline-dot" />
          <div>
            <Typography.Text strong>{fileAuditActionLabel(record.values?.action || record.code, dictionary)}</Typography.Text>
            <Typography.Text type="secondary">{formatDate(record.values?.createdAt || record.updatedAt)}</Typography.Text>
            <PlatformOverflowText value={localizedRecordValue(record, "targetName", language) || record.values?.targetCode || record.code} />
          </div>
        </div>
      ))}
    </div>
  );
}

function FileExpectedAuditEvents({ dictionary }: { dictionary: Dictionary }) {
  return (
    <div className="file-audit-timeline expected">
      {["file.upload", "file.content", "file.delete"].map((action) => (
        <div className="file-audit-row" key={action}>
          <span className="timeline-dot" />
          <div>
            <Typography.Text strong>{fileAuditActionLabel(action, dictionary)}</Typography.Text>
            <Typography.Text type="secondary">{action}</Typography.Text>
          </div>
        </div>
      ))}
    </div>
  );
}

function FileMobileCardContent({ record, dictionary, language }: { record: AdminResourceRecord; dictionary: Dictionary; language: Language }) {
  const kind = filePreviewKind(record);
  return (
    <>
      <div className={`mobile-file-icon kind-${kind}`}>
        <FileKindIcon kind={kind} />
      </div>
      <span>
        <strong>{fileRecordName(record, language)}</strong>
        <em>{fileMimeType(record) || dictionary.fileTypeUnknown} · {formatBytes(fileSize(record))}</em>
      </span>
      <StatusTag status={record.status} dictionary={dictionary} />
    </>
  );
}

function FileKindIcon({ kind }: { kind: "image" | "text" | "pdf" | "unsupported" }) {
  switch (kind) {
  case "image":
    return <FileImageOutlined />;
  case "pdf":
    return <FilePdfOutlined />;
  case "text":
    return <FileTextOutlined />;
  default:
    return <FileUnknownOutlined />;
  }
}

function DetailFieldsPanel({
  detailFields,
  record,
  dictionary,
  language,
}: {
  detailFields: AdminResourceField[];
  record: AdminResourceRecord;
  dictionary: Dictionary;
  language: Language;
}) {
  return (
    <dl className="detail-list">
      {detailFields.map((field) => (
        <div key={field.key}>
          <dt>{localizedText(field.label, language)}</dt>
          <dd>{renderPlainFieldValue(record, field, language, dictionary)}</dd>
        </div>
      ))}
    </dl>
  );
}

function PermissionPanel({ permissions }: { permissions: AdminResourceSchema["permissions"] }) {
  return (
    <section className="inspector-section">
      <div className="resource-values-list">
        {Object.entries(permissions).map(([action, permission]) => (
          <div key={action}>
            <Typography.Text code>{action}</Typography.Text>
            <span>{permission}</span>
          </div>
        ))}
      </div>
    </section>
  );
}

function ResourceCustomPanel({
  panel,
  record,
  dictionary,
  language,
}: {
  panel: AdminResourcePanel;
  record: AdminResourceRecord;
  dictionary: Dictionary;
  language: Language;
}) {
  const emptyText = panel.empty ? localizedText(panel.empty, language) : dictionary.customPanelEmpty;
  return (
    <section className={`inspector-section resource-custom-panel panel-${panel.kind}`}>
      <Typography.Text strong>{localizedText(panel.label, language)}</Typography.Text>
      {panel.kind === "audit" ? (
        <div className="resource-panel-timeline">
          <span className="timeline-dot" />
          <div>
            <Typography.Text>{dictionary.auditPanelTitle}</Typography.Text>
            <Typography.Text className="secondary-text">{formatTemplate(dictionary.auditPanelHint, { code: record.code })}</Typography.Text>
          </div>
        </div>
      ) : panel.kind === "approval" ? (
        <AdminFeedback type="info" message={dictionary.approvalPanelTitle} description={emptyText} />
      ) : panel.kind === "files" ? (
        <AdminFeedback type="info" message={dictionary.filesPanelTitle} description={emptyText} />
      ) : (
        <AdminFeedback type="info" message={dictionary.pluginPanelTitle} description={emptyText} />
      )}
    </section>
  );
}

function CustomRowActionOverflow({
  actions,
  dictionary,
  executingKey,
  language,
  onExecute,
  record,
  onUnavailable,
}: {
  actions: AdminResourceAction[];
  dictionary: Dictionary;
  executingKey: string;
  language: Language;
  onExecute: (action: AdminResourceAction, record: AdminResourceRecord) => void;
  record: AdminResourceRecord;
  onUnavailable: (message: string) => void;
}) {
  if (actions.length === 0) {
    return null;
  }
  const menuItems: MenuProps["items"] = actions.map((action) => ({
    key: action.key,
    label: localizedText(action.label, language),
    danger: action.tone === "danger",
  }));
  return (
    <Dropdown
      menu={{
        items: menuItems,
        onClick: ({ domEvent, key }) => {
          domEvent.stopPropagation();
          const action = actions.find((item) => item.key === key);
          if (action?.confirm) {
            Modal.confirm({
              title: localizedText(action.confirm.title, language),
              content: action.confirm.description ? localizedText(action.confirm.description, language) : undefined,
              okText: action.confirm.okText ? localizedText(action.confirm.okText, language) : localizedText(action.label, language),
              cancelText: dictionary.cancel,
              okButtonProps: { danger: action.tone === "danger" },
              onOk: () => onExecute(action, record),
            });
            return;
          }
          if (action?.route) {
            onExecute(action, record);
            return;
          }
          const label = action ? localizedText(action.label, language) : String(key);
          onUnavailable(formatTemplate(dictionary.customActionUnavailable, { action: label, record: record.code }));
        },
      }}
      trigger={["click"]}
    >
      <AdminActionButton
        icon={<MoreOutlined />}
        label={dictionary.moreActions}
        loading={Boolean(actions.find((action) => actionExecutionKey(action, record) === executingKey))}
        size="small"
        type="text"
        onClick={(event) => event.stopPropagation()}
      />
    </Dropdown>
  );
}

function CustomBatchActions({
  actions,
  clearSelection,
  dictionary,
  executingKey,
  language,
  onExecute,
  selectedKeys,
  onUnavailable,
}: {
  actions: AdminResourceAction[];
  clearSelection: () => void;
  dictionary: Dictionary;
  executingKey: string;
  language: Language;
  onExecute: (action: AdminResourceAction, selectedKeys: Key[], clearSelection: () => void) => void;
  selectedKeys: Key[];
  onUnavailable: (message: string) => void;
}) {
  return (
    <>
      {actions.map((action) => (
        <Button
          danger={action.tone === "danger"}
          key={action.key}
          loading={actionExecutionKey(action) === executingKey}
          size="small"
          type={action.tone === "primary" ? "primary" : "default"}
          onClick={() => {
            if (action.confirm) {
              Modal.confirm({
                title: localizedText(action.confirm.title, language),
                content: action.confirm.description ? localizedText(action.confirm.description, language) : undefined,
                okText: action.confirm.okText ? localizedText(action.confirm.okText, language) : localizedText(action.label, language),
                cancelText: dictionary.cancel,
                okButtonProps: { danger: action.tone === "danger" },
                onOk: () => onExecute(action, selectedKeys, clearSelection),
              });
              return;
            }
            if (action.route) {
              onExecute(action, selectedKeys, clearSelection);
              return;
            }
            onUnavailable(formatTemplate(dictionary.customBatchActionUnavailable, {
              action: localizedText(action.label, language),
              count: String(selectedKeys.length),
            }));
          }}
        >
          {localizedText(action.label, language)}
        </Button>
      ))}
    </>
  );
}

async function loadRelationOptions(field: AdminResourceField, provider: DataProvider): Promise<RelationOptionResult> {
  const relation = field.relation;
  if (!relation) {
    return { fieldKey: field.key, options: field.options ?? [] };
  }
  const sortField = relation.sortField || relation.labelField || relation.valueField;
  const result = await provider.getList<AdminResourceRecord>({
    resource: relation.resource,
    pagination: {
      currentPage: 1,
      pageSize: 100,
      mode: "server",
    },
    sorters: sortField ? [{ field: sortField, order: relation.sortOrder === "desc" ? "desc" : "asc" }] : undefined,
    meta: {
      conditions: relation.filters ?? [],
    },
  });
  const dynamicOptions = result.data.map((record) => relationOptionFromRecord(record, relation));
  return {
    fieldKey: field.key,
    options: mergeFieldOptions(dynamicOptions, field.options ?? []),
  };
}

function mergeRelationOptions(schema: AdminResourceSchema, results: RelationOptionResult[]): AdminResourceSchema {
  const optionsByField = new Map(results.map((result) => [result.fieldKey, result.options]));
  return {
    ...schema,
    fields: schema.fields.map((field) => {
      const options = optionsByField.get(field.key);
      return options ? { ...field, options } : field;
    }),
  };
}

function relationOptionFromRecord(record: AdminResourceRecord, relation: NonNullable<AdminResourceField["relation"]>): NonNullable<AdminResourceField["options"]>[number] {
  const value = recordValueByField(record, relation.valueField);
  const labelZH = relationDisplayLabel(record, value, relation.labelField, "zh");
  const labelEN = relationDisplayLabel(record, value, relation.labelField, "en");
  const parentValue = relation.parentField ? recordValueByField(record, relation.parentField) : undefined;
  const pathValue = relation.pathField ? recordValueByField(record, relation.pathField) : undefined;
  return { value, label: { zh: labelZH, en: labelEN }, parentValue, pathValue };
}

function relationDisplayLabel(record: AdminResourceRecord, value: string, labelField: string, language: Language) {
  const label = labelField === "name" || labelField === "description"
    ? localizedRecordValue(record, labelField, language)
    : recordValueByField(record, labelField);
  const display = label || value;
  return display && display !== value ? `${display} (${value})` : display;
}

function recordValueByField(record: AdminResourceRecord, fieldKey: string) {
  if (fieldKey === "id") {
    return record.id;
  }
  if (fieldKey === "code") {
    return record.code;
  }
  if (fieldKey === "name") {
    return record.name;
  }
  if (fieldKey === "status") {
    return record.status;
  }
  if (fieldKey === "description") {
    return record.description ?? "";
  }
  if (fieldKey === "updatedAt") {
    return record.updatedAt;
  }
  return record.values?.[fieldKey] ?? "";
}

function mergeFieldOptions(
  dynamicOptions: NonNullable<AdminResourceField["options"]>,
  fallbackOptions: NonNullable<AdminResourceField["options"]>,
) {
  const seen = new Set<string>();
  const merged: NonNullable<AdminResourceField["options"]> = [];
  for (const option of [...dynamicOptions, ...fallbackOptions]) {
    if (!option.value || seen.has(option.value)) {
      continue;
    }
    seen.add(option.value);
    merged.push(option);
  }
  return merged;
}

function hasRelationSource(field: AdminResourceField) {
  return Boolean(field.relation?.resource && field.relation.valueField && field.relation.labelField);
}

function relationSignature(fields: AdminResourceField[]) {
  return fields
    .filter(hasRelationSource)
    .map((field) => `${field.key}:${field.relation?.resource}:${field.relation?.valueField}:${field.relation?.labelField}:${field.relation?.multiple ? "multi" : "single"}:${JSON.stringify(field.relation?.filters ?? [])}:${field.relation?.sortField ?? ""}:${field.relation?.sortOrder ?? ""}:${field.relation?.display ?? ""}:${field.relation?.parentField ?? ""}:${field.relation?.pathField ?? ""}:${field.relation?.rootValue ?? ""}`)
    .sort()
    .join("|");
}

type FieldInputProps = {
  field: AdminResourceField;
  language: Language;
} & Record<string, unknown>;

function FieldInput({ field, language, ...controlProps }: FieldInputProps) {
  const maxLength = field.validation?.maxLength;
  const numericMin = field.validation?.min;
  const numericMax = field.validation?.max;
  if (field.type === "textarea") {
    return <Input.TextArea {...(controlProps as ComponentProps<typeof Input.TextArea>)} maxLength={maxLength} rows={3} showCount={Boolean(maxLength)} />;
  }
  if (field.type === "switch") {
    return <Switch {...(controlProps as ComponentProps<typeof Switch>)} aria-label={localizedText(field.label, language)} />;
  }
  if (field.type === "number") {
    return <InputNumber {...(controlProps as ComponentProps<typeof InputNumber>)} className="resource-number-input" max={numericMax} min={numericMin} />;
  }
  if (field.type === "color") {
    return <Input {...(controlProps as ComponentProps<typeof Input>)} type="color" />;
  }
  if (field.type === "multiselect") {
    if (isTreeRelationField(field)) {
      return (
        <PlatformTreeSelect
          {...(controlProps as ComponentProps<typeof PlatformTreeSelect>)}
          multiple
          options={treeSelectOptions(field, language)}
        />
      );
    }
    return (
      <Select
        {...(controlProps as ComponentProps<typeof Select>)}
        mode="multiple"
        allowClear
        getPopupContainer={platformPopupContainer}
        maxTagCount="responsive"
        optionFilterProp="label"
        options={(field.options ?? []).map((option) => ({
          value: option.value,
          label: localizedText(option.label, language),
        }))}
        showSearch
      />
    );
  }
  if (field.type === "select") {
    if (isTreeRelationField(field)) {
      return (
        <PlatformTreeSelect
          {...(controlProps as ComponentProps<typeof PlatformTreeSelect>)}
          allowClear={!field.required}
          options={treeSelectOptions(field, language)}
        />
      );
    }
    return (
      <Select
        {...(controlProps as ComponentProps<typeof Select>)}
        allowClear={!field.required}
        getPopupContainer={platformPopupContainer}
        optionFilterProp="label"
        options={(field.options ?? []).map((option) => ({
          value: option.value,
          label: localizedText(option.label, language),
        }))}
        showSearch
      />
    );
  }
  return <Input {...(controlProps as ComponentProps<typeof Input>)} maxLength={maxLength} showCount={Boolean(maxLength)} />;
}

function isTreeRelationField(field: AdminResourceField) {
  return field.relation?.display === "tree";
}

function treeSelectOptions(field: AdminResourceField, language: Language) {
  return (field.options ?? []).map((option) => ({
    value: option.value,
    label: localizedText(option.label, language),
    parentValue: option.parentValue,
    pathValue: option.pathValue,
  }));
}

function resourceFormSections(
  fields: AdminResourceField[],
  groups: NonNullable<AdminResourceSchema["formGroups"]>,
  language: Language,
  dictionary: Dictionary,
): ResourceFormSection[] {
  if (fields.length === 0) {
    return [];
  }
  const groupByKey = new Map(groups.map((group) => [group.key, group]));
  const grouped = fields.some((field) => field.group) || groups.length > 0;
  if (!grouped) {
    return [{ key: "default", fields }];
  }

  const buckets = new Map<string, AdminResourceField[]>();
  for (const field of fields) {
    const key = field.group || "default";
    buckets.set(key, [...(buckets.get(key) ?? []), field]);
  }

  const orderedKeys = [
    ...groups.map((group) => group.key).filter((key) => buckets.has(key)),
    ...Array.from(buckets.keys()).filter((key) => key === "default" || !groupByKey.has(key)),
  ];

  return orderedKeys.map((key) => {
    const group = groupByKey.get(key);
    if (group) {
      return {
        key,
        label: localizedText(group.label, language),
        description: group.description ? localizedText(group.description, language) : undefined,
        fields: buckets.get(key) ?? [],
      };
    }
    return {
      key,
      label: key === "default" ? dictionary.formAdditionalGroup : key,
      fields: buckets.get(key) ?? [],
    };
  });
}

function fieldRules(field: AdminResourceField, language: Language, dictionary: Dictionary): Rule[] | undefined {
  const label = localizedText(field.label, language);
  const rules: Rule[] = [];
  if (field.required) {
    rules.push({ required: true, message: requiredMessage(field, language) });
  }
  if (field.validation?.minLength) {
    rules.push({
      min: field.validation.minLength,
      message: formatTemplate(dictionary.validationMinLength, { label, min: String(field.validation.minLength) }),
    });
  }
  if (field.validation?.maxLength) {
    rules.push({
      max: field.validation.maxLength,
      message: formatTemplate(dictionary.validationMaxLength, { label, max: String(field.validation.maxLength) }),
    });
  }
  if (typeof field.validation?.min === "number") {
    rules.push({
      type: "number",
      min: field.validation.min,
      message: formatTemplate(dictionary.validationMin, { label, min: String(field.validation.min) }),
    });
  }
  if (typeof field.validation?.max === "number") {
    rules.push({
      type: "number",
      max: field.validation.max,
      message: formatTemplate(dictionary.validationMax, { label, max: String(field.validation.max) }),
    });
  }
  if (field.validation?.pattern) {
    const pattern = safeRegExp(field.validation.pattern);
    if (pattern) {
      rules.push({
        pattern,
        message: formatTemplate(dictionary.validationPattern, { label }),
      });
    }
  }
  return rules.length > 0 ? rules : undefined;
}

function safeRegExp(pattern: string) {
  try {
    return new RegExp(pattern);
  } catch {
    return null;
  }
}

function resourceFilterFields(fields: AdminResourceField[], language: Language, dictionary: Dictionary): PlatformDataTableFilterField[] {
  return fields
    .filter(isFilterableResourceField)
    .slice(0, 8)
    .map((field) => {
      if (field.type === "select" || field.type === "multiselect" || field.key === "status") {
        return {
          key: field.key,
          label: localizedText(field.label, language),
          type: isTreeRelationField(field) ? "treeSelect" : "select",
          placeholder: dictionary.filterKeyword,
          options: isTreeRelationField(field)
            ? treeSelectOptions(field, language)
            : (field.options ?? []).map((option) => ({
                value: option.value,
                label: localizedText(option.label, language),
              })),
        };
      }
      if (field.type === "datetime") {
        return {
          key: field.key,
          label: localizedText(field.label, language),
          type: "dateRange",
        };
      }
      if (field.type === "number") {
        return {
          key: field.key,
          label: localizedText(field.label, language),
          type: "numberRange",
        };
      }
      if (field.type === "switch") {
        return {
          key: field.key,
          label: localizedText(field.label, language),
          type: "select",
          placeholder: dictionary.filterKeyword,
          options: [
            { value: "true", label: dictionary.yes },
            { value: "false", label: dictionary.no },
          ],
        };
      }
      return {
        key: field.key,
        label: localizedText(field.label, language),
        type: "text",
        placeholder: dictionary.filterKeyword,
      };
    });
}

function buildResourceQuery(
  schema: AdminResourceSchema,
  query: string,
  filters: Record<string, PlatformDataTableFilterValue>,
  page: number,
  pageSize: number,
  sort: AdminResourceQuerySort[],
): AdminResourceQueryInput {
  const parsed = parseSafeQuery(query, schema.fields);
  return {
    keywords: parsed.terms,
    conditions: [
      ...parsed.conditions.map((condition) => ({
        field: condition.field,
        operator: queryOperatorToAdminOperator(condition.operator),
        value: condition.value,
      })),
      ...filterConditionsFromValues(filters, schema.fields),
    ],
    sort,
    page,
    pageSize,
  };
}

function parseSafeQuery(query: string, fields: AdminResourceField[]) {
  const fieldKeys = new Set(["id", ...fields.filter(isQueryableResourceField).map((field) => field.key)]);
  const tokens = query.match(/"[^"]+"|'[^']+'|\S+/g) ?? [];
  const conditions: ParsedQueryCondition[] = [];
  const terms: string[] = [];

  for (const token of tokens) {
    const normalized = stripQuotes(token.trim());
    const match = normalized.match(/^([a-zA-Z0-9_.-]+)(>=|<=|!=|>|<|=|:|~)(.+)$/);
    if (!match || !fieldKeys.has(match[1])) {
      if (normalized) {
        terms.push(normalized);
      }
      continue;
    }
    conditions.push({
      field: match[1],
      operator: match[2] as QueryOperator,
      value: stripQuotes(match[3].trim()),
    });
  }

  return { conditions, terms };
}

type QueryOperator = "=" | "!=" | ":" | "~" | ">=" | "<=" | ">" | "<";

type ParsedQueryCondition = { field: string; operator: QueryOperator; value: string };

function queryOperatorToAdminOperator(operator: QueryOperator): AdminResourceQueryCondition["operator"] {
  switch (operator) {
  case ":":
  case "~":
    return "contains";
  default:
    return operator;
  }
}

function filterConditionsFromValues(
  filters: Record<string, PlatformDataTableFilterValue>,
  fields: AdminResourceField[],
): AdminResourceQueryCondition[] {
  return Object.entries(filters).flatMap(([fieldKey, filterValue]) => {
    if (!filterValueActive(filterValue)) {
      return [];
    }
    const field = fields.find((item) => item.key === fieldKey);
    if (!field || !isFilterableResourceField(field)) {
      return [];
    }
    if (typeof filterValue === "string") {
      return [
        {
          field: field.key,
          operator: field.type === "select" || field.type === "multiselect" || field.type === "switch" || field.key === "status" ? "=" : "contains",
          value: filterValue,
        },
      ];
    }
    return [
      filterValue.from ? { field: field.key, operator: ">=", value: filterValue.from } : null,
      filterValue.to ? { field: field.key, operator: "<=", value: filterValue.to } : null,
    ].filter((condition): condition is AdminResourceQueryCondition => Boolean(condition));
  });
}

function tableSortOrder(sort: AdminResourceQuerySort[], field: string): "ascend" | "descend" | null {
  const active = sort.find((item) => item.field === field);
  if (!active) {
    return null;
  }
  return active.order === "desc" ? "descend" : "ascend";
}

type TableSorter<T extends object> = Parameters<NonNullable<TableProps<T>["onChange"]>>[2];

function sortersFromTableChange(sorter: TableSorter<AdminResourceRecord>): AdminResourceQuerySort[] {
  const sorters = Array.isArray(sorter) ? sorter : [sorter];
  return sorters.flatMap((item) => {
    const field = typeof item.field === "string" ? item.field : String(item.columnKey ?? "");
    if (!field || !item.order) {
      return [];
    }
    return [{ field, order: item.order === "descend" ? "desc" : "asc" }];
  });
}

function querySortToCrudSort(sort: AdminResourceQuerySort[]): CrudSort[] {
  return sort.map((item) => ({
    field: item.field,
    order: item.order === "desc" ? "desc" : "asc",
  }));
}

function issuedTokenFromRefineRecord(record: AdminResourceRecord) {
  const token = (record as AdminResourceRecord & { __platformIssuedToken?: unknown }).__platformIssuedToken;
  return typeof token === "string" ? token : "";
}

function renderFieldValue(
  record: AdminResourceRecord,
  field: AdminResourceField,
  language: Language,
  dictionary: Dictionary,
  statusControls?: {
    canToggleStatus: boolean;
    toggling: boolean;
    onToggleStatus: (record: AdminResourceRecord, checked: boolean) => void;
  },
) {
  if (field.key === "name") {
    const displayName = localizedRecordValue(record, "name", language);
    return (
      <div className="resource-name-cell">
        <PlatformOverflowText value={displayName} strong />
        <PlatformOverflowText className="secondary-text" value={record.id} />
      </div>
    );
  }
  if (field.key === "status") {
    if (statusControls?.canToggleStatus) {
      return (
        <span className="status-switch-cell" onClick={(event) => event.stopPropagation()}>
          <Switch
            aria-label={dictionary.statusToggle}
            checked={record.status === "enabled"}
            loading={statusControls.toggling}
            size="small"
            onChange={(checked) => statusControls.onToggleStatus(record, checked)}
          />
        </span>
      );
    }
    return <StatusTag status={record.status} dictionary={dictionary} />;
  }
  const value = renderPlainFieldValue(record, field, language, dictionary);
  if (field.key === "code") {
    return <PlatformOverflowText code value={value} />;
  }
  if (field.type === "select") {
    return <Tag>{value}</Tag>;
  }
  if (field.type === "multiselect") {
    const values = splitList(getRecordFieldValue(record, field));
    if (values.length === 0) {
      return "-";
    }
    return (
      <Space size={4} wrap>
        {values.map((item) => (
          <Tag key={item}>{optionLabel(field, item, language)}</Tag>
        ))}
      </Space>
    );
  }
  if (field.type === "switch") {
    return (
      <Switch
        aria-label={localizedText(field.label, language)}
        checked={isTruthyValue(getRecordFieldValue(record, field))}
        disabled
        size="small"
      />
    );
  }
  if (field.type === "color") {
    const color = getRecordFieldValue(record, field);
    return color ? (
      <span className="resource-color-cell">
        <i style={{ background: color }} />
        <Typography.Text code>{color}</Typography.Text>
      </span>
    ) : (
      "-"
    );
  }
  return withOverflowTooltip(value);
}

function renderPlainFieldValue(
  record: AdminResourceRecord,
  field: AdminResourceField,
  language: Language,
  dictionary: Dictionary,
) {
  const value = getRecordFieldValue(record, field);
  if (!value) {
    return "-";
  }
  if (field.localizable) {
    return localizedRecordValue(record, field.key, language);
  }
  if (field.key === "status") {
    return statusLabel(dictionary, value);
  }
  if (field.type === "datetime") {
    return formatDate(value);
  }
  if (field.type === "select") {
    return optionLabel(field, value, language);
  }
  if (field.type === "multiselect") {
    return splitList(value)
      .map((item) => optionLabel(field, item, language))
      .join(", ");
  }
  if (field.type === "switch") {
    return boolLabel(dictionary, value);
  }
  return value;
}

function withOverflowTooltip(value: string) {
  return <PlatformOverflowText value={value} />;
}

function getRecordFieldValue(record: AdminResourceRecord, field?: AdminResourceField) {
  if (!field) {
    return "";
  }
  if (field.source === "values") {
    return record.values?.[field.key] ?? "";
  }
  const value = record[field.key as keyof AdminResourceRecord];
  return typeof value === "string" ? value : "";
}

function localizedRecordValue(record: AdminResourceRecord, key: string, language: Language) {
  const suffix = language === "zh" ? "Zh" : "En";
  const fallbackSuffix = language === "zh" ? "En" : "Zh";
  return record.values?.[`${key}${suffix}`] || record.values?.[`${key}${fallbackSuffix}`] || record.values?.[key] || recordFieldValue(record, key);
}

function recordFieldValue(record: AdminResourceRecord, key: string) {
  const value = record[key as keyof AdminResourceRecord];
  return typeof value === "string" ? value : "";
}

function formValuesFromRecord(record: AdminResourceRecord, fields: AdminResourceField[]) {
  return Object.fromEntries(
    fields.map((field) => [field.key, formValueFromRecord(record, field)]),
  );
}

function formValueFromRecord(record: AdminResourceRecord, field: AdminResourceField) {
  const value = getRecordFieldValue(record, field);
  if (field.type === "multiselect") {
    return splitList(value);
  }
  if (field.type === "switch") {
    return isTruthyValue(value);
  }
  if (field.type === "number") {
    return value === "" ? undefined : Number(value);
  }
  return value;
}

function defaultFormValues(fields: AdminResourceField[]) {
  return Object.fromEntries(fields.map((field) => [field.key, defaultFieldValue(field)]));
}

function defaultFieldValue(field: AdminResourceField) {
  if (field.type === "multiselect") {
    return [];
  }
  if (field.type === "switch") {
    return false;
  }
  if (field.type === "number") {
    return undefined;
  }
  if (field.key === "status") {
    return "enabled";
  }
  return field.options?.[0]?.value;
}

function inputFromFormValues(values: ResourceFormValues, fields: AdminResourceField[]): AdminResourceInput {
  const input: AdminResourceInput = { name: String(values.name ?? "") };
  const nestedValues: Record<string, string> = {};
  for (const field of fields) {
    const raw = values[field.key];
    const value = Array.isArray(raw) ? raw.join(",") : raw == null ? "" : String(raw);
    if (field.source === "values") {
      if (value.trim() !== "") {
        nestedValues[field.key] = value;
      }
      continue;
    }
    switch (field.key) {
    case "code":
      input.code = value;
      break;
    case "name":
      input.name = value;
      break;
    case "status":
      input.status = value;
      break;
    case "description":
      input.description = value;
      break;
    }
  }
  if (Object.keys(nestedValues).length > 0) {
    input.values = nestedValues;
  }
  return input;
}

function inputFromRecord(record: AdminResourceRecord, overrides: Partial<AdminResourceInput> = {}): AdminResourceInput {
  return {
    code: overrides.code ?? record.code,
    name: overrides.name ?? record.name,
    status: overrides.status ?? record.status,
    description: overrides.description ?? record.description ?? "",
    values: { ...(record.values ?? {}), ...(overrides.values ?? {}) },
  };
}

function canToggleStatusField(field: AdminResourceField, record: AdminResourceRecord) {
  if (field.key !== "status" || field.source !== "record") {
    return false;
  }
  const values = new Set((field.options ?? []).map((option) => option.value));
  return values.has("enabled") && values.has("disabled") && (record.status === "enabled" || record.status === "disabled");
}

function isQueryableResourceField(field: AdminResourceField) {
  return Boolean(field.searchable || isFilterableResourceField(field));
}

function isFilterableResourceField(field: AdminResourceField) {
  return Boolean(
    field.filterable ||
      field.searchable ||
      field.key === "status" ||
      field.type === "select" ||
      field.type === "multiselect" ||
      field.type === "switch" ||
      field.type === "datetime" ||
      field.type === "number",
  );
}

function isSortableResourceField(field: AdminResourceField) {
  if (field.sortable) {
    return true;
  }
  if (field.key === "id" || field.key === "updatedAt" || field.type === "datetime" || field.type === "number") {
    return true;
  }
  return Boolean(field.inTable && field.type !== "textarea" && field.type !== "multiselect");
}

function splitList(value: string) {
  return value
    .split(/[,\n\t ]+/)
    .map((item) => item.trim())
    .filter(Boolean);
}

function optionLabel(field: AdminResourceField, value: string, language: Language) {
  return field.options?.find((option) => option.value === value)?.label
    ? localizedText(field.options.find((option) => option.value === value)!.label, language)
    : value;
}

function localizedText(value: { zh: string; en: string }, language: Language) {
  return value[language] || value.zh || value.en;
}

function permissionAllows(permissions: string[], permission: string, deniedPermissions: string[] = []) {
  if (!permission) {
    return false;
  }
  if (matchesAnyPermission(deniedPermissions, permission)) {
    return false;
  }
  return matchesAnyPermission(permissions, permission);
}

function actionExecutionKey(action: AdminResourceAction, record?: AdminResourceRecord) {
  return `${action.key}:${record?.id ?? "*"}`;
}

function matchesAnyPermission(permissions: string[], permission: string) {
  return permissions.some((granted) => {
    if (granted === "*" || granted === permission) {
      return true;
    }
    if (granted.endsWith(":*")) {
      return permission.startsWith(granted.slice(0, -1));
    }
    return false;
  });
}

function requiredMessage(field: AdminResourceField, language: Language) {
  const label = localizedText(field.label, language);
  return formatTemplate(dictionaries[language].requiredField, { label });
}

function createFallbackSchema(resourceKey: string, resource: AdminResourceDefinition): AdminResourceSchema {
  const permissionPrefix = resource.permission.replace(/:read$/, "");
  return {
    resource: resourceKey,
    title: resource.title,
    description: resource.description,
    permissions: {
      read: resource.permission,
      create: `${permissionPrefix}:create`,
      update: `${permissionPrefix}:update`,
      delete: `${permissionPrefix}:delete`,
    },
    formGroups: fallbackFormGroups(),
    formLayout: "two-column-density",
    fields: [
      fallbackField("name", localizedFieldText("recordName"), "text", "record", true, 180),
      fallbackField("code", localizedFieldText("recordCode"), "text", "record", true, 180),
      fallbackField("status", localizedFieldText("status"), "select", "record", false, 120, [
        { value: "enabled", label: localizedFieldText("statusEnabled") },
        { value: "disabled", label: localizedFieldText("statusDisabled") },
        { value: "healthy", label: localizedFieldText("statusHealthy") },
        { value: "recorded", label: localizedFieldText("statusRecorded") },
      ]),
      fallbackField("description", localizedFieldText("description"), "textarea", "record", false, 280),
      fallbackField("nameZh", localizedFieldText("recordNameZh"), "text", "values", false, 180, undefined, false, true, false),
      fallbackField("nameEn", localizedFieldText("recordNameEn"), "text", "values", false, 180, undefined, false, true, false),
      fallbackField("descriptionZh", localizedFieldText("recordDescriptionZh"), "textarea", "values", false, 240, undefined, false, true, false),
      fallbackField("descriptionEn", localizedFieldText("recordDescriptionEn"), "textarea", "values", false, 240, undefined, false, true, false),
      fallbackField("updatedAt", localizedFieldText("updatedAt"), "datetime", "record", false, 180, undefined, false, false, true),
    ],
    searchFields: ["name", "code", "status", "description"],
    defaultSortKey: "updatedAt",
  };
}

function normalizeResourceSchema(schema: AdminResourceSchema): AdminResourceSchema {
  const fields = uniqueResourceFields(schema.fields).map((field) => ({
    ...field,
    group: field.group ?? defaultFormGroupForField(field.key),
  }));
  return {
    ...schema,
    formGroups: schema.formGroups && schema.formGroups.length > 0 ? schema.formGroups : fallbackFormGroups(),
    fields,
  };
}

function uniqueResourceFields(fields: AdminResourceField[]) {
  const seen = new Set<string>();
  return fields.filter((field) => {
    if (seen.has(field.key)) {
      return false;
    }
    seen.add(field.key);
    return true;
  });
}

function buildRuntimeFormSlots({
  descriptors,
  registry,
  dictionary,
  language,
  fields,
  sections,
  record,
  formValues,
  schemaPermissions,
  permissions,
  deniedPermissions,
}: {
  descriptors: NonNullable<AdminResourceSchema["runtimeSlots"]>;
  registry: AdminFormRuntimeSlotRegistry;
  dictionary: Dictionary;
  language: Language;
  fields: AdminResourceField[];
  sections: ResourceFormSection[];
  record: AdminResourceRecord | null;
  formValues: ResourceFormValues;
  schemaPermissions: AdminResourceSchema["permissions"];
  permissions: string[];
  deniedPermissions: string[];
}): PlatformResourceFormSlots<AdminResourceField> | undefined {
  const visibleDescriptors = descriptors
    .filter((descriptor) => runtimeSlotVisible(descriptor.visibleWhen, record))
    .filter((descriptor) => !descriptor.permission || permissionAllows(permissions, descriptor.permission, deniedPermissions))
    .slice()
    .sort((left, right) => (left.order ?? 0) - (right.order ?? 0));
  if (visibleDescriptors.length === 0) {
    return undefined;
  }
  const renderDescriptor = (descriptor: NonNullable<AdminResourceSchema["runtimeSlots"]>[number], extra?: { defaultControl?: ReactNode; field?: AdminResourceField }) =>
    registry.render(descriptor, {
      dictionary,
      language,
      fields,
      sections,
      record,
      formValues,
      permissions: schemaPermissions,
      defaultControl: extra?.defaultControl,
      field: extra?.field,
    });
  const renderStack = (items: ReactNode[]) => (items.length > 0 ? <div className="runtime-slot-stack">{items}</div> : null);
  const byRegion = (region: NonNullable<AdminResourceSchema["runtimeSlots"]>[number]["region"]) =>
    visibleDescriptors.filter((descriptor) => descriptor.region === region);
  const renderRegion = (region: NonNullable<AdminResourceSchema["runtimeSlots"]>[number]["region"]) =>
    renderStack(byRegion(region).map((descriptor) => <div key={runtimeSlotKey(descriptor)}>{renderDescriptor(descriptor)}</div>));
  const renderSectionRegion = (region: "form.section.before" | "form.section.after", section: ResourceFormSection) =>
    renderStack(
      byRegion(region)
        .filter((descriptor) => descriptor.targetSection === section.key)
        .map((descriptor) => <div key={runtimeSlotKey(descriptor)}>{renderDescriptor(descriptor)}</div>),
    );
  return {
    header: renderRegion("form.header"),
    footer: renderRegion("form.footer"),
    sidePreview: renderRegion("side.preview"),
    sectionBefore: (section) => renderSectionRegion("form.section.before", section),
    sectionAfter: (section) => renderSectionRegion("form.section.after", section),
    fieldControl: (field, defaultControl) => {
      const fieldDescriptors = byRegion("field.control").filter((descriptor) => descriptor.targetField === field.key);
      if (fieldDescriptors.length === 0) {
        return defaultControl;
      }
      return fieldDescriptors.reduce((control, descriptor) => renderDescriptor(descriptor, { defaultControl: control, field }) ?? control, defaultControl);
    },
  };
}

function runtimeSlotVisible(visibleWhen: string | undefined, record: AdminResourceRecord | null) {
  switch (visibleWhen) {
  case "create":
    return !record;
  case "edit":
  case "hasRecord":
    return Boolean(record);
  case "noRecord":
    return !record;
  default:
    return true;
  }
}

function runtimeSlotKey(descriptor: NonNullable<AdminResourceSchema["runtimeSlots"]>[number]) {
  return `${descriptor.slotId}:${descriptor.region}:${descriptor.targetSection ?? ""}:${descriptor.targetField ?? ""}`;
}

function normalizeFormLayoutPreset(layout: AdminResourceSchema["formLayout"], formFields: AdminResourceField[]): PlatformResourceFormLayoutPreset {
  if (layout === "single-column" || layout === "grouped-sections" || layout === "two-column-density" || layout === "side-detail-preview") {
    return layout;
  }
  if (formFields.length >= 6) {
    return "two-column-density";
  }
  return "grouped-sections";
}

function tableColumnPriority(index: number): PlatformDataTableColumnPriority {
  if (index < 4) return "essential";
  if (index < 7) return "standard";
  return "extended";
}

function focusFirstEditableFormField(form: FormInstance<ResourceFormValues>, fields: AdminResourceField[]) {
  const firstField = fields[0];
  if (!firstField) {
    return;
  }
  const fieldInstance = form.getFieldInstance(firstField.key) as { focus?: (options?: FocusOptions) => void } | undefined;
  if (fieldInstance?.focus) {
    fieldInstance.focus({ preventScroll: true });
    return;
  }
  const visibleModal = Array.from(document.querySelectorAll<HTMLElement>(".admin-form-modal")).find(
    (modal) => modal.getClientRects().length > 0,
  );
  const fieldControl = visibleModal?.querySelector<HTMLElement>(FOCUSABLE_RESOURCE_FORM_CONTROL_SELECTOR);
  fieldControl?.focus({ preventScroll: true });
}

function formModalWidth(layout: PlatformResourceFormLayoutPreset) {
  if (layout === "side-detail-preview") {
    return 960;
  }
  return layout === "two-column-density" ? 760 : 560;
}

function fallbackField(
  key: string,
  label: { zh: string; en: string },
  type: AdminResourceField["type"],
  source: AdminResourceField["source"],
  required: boolean,
  width: number,
  options?: AdminResourceField["options"],
  inTable = true,
  inForm = true,
  inDetail = true,
): AdminResourceField {
  return {
    key,
    label,
    type,
    source,
    required,
    searchable: inTable,
    group: defaultFormGroupForField(key),
    inTable,
    inForm,
    inDetail,
    width,
    options,
  };
}

function fallbackFormGroups(): NonNullable<AdminResourceSchema["formGroups"]> {
  return [
    {
      key: "basic",
      label: localizedFieldText("formDefaultGroup"),
      description: localizedFieldText("formDefaultGroupDescription"),
    },
    {
      key: "localization",
      label: localizedFieldText("formLocalizedGroup"),
      description: localizedFieldText("formLocalizedGroupDescription"),
    },
  ];
}

function defaultFormGroupForField(key: string) {
  if (["name", "code", "status", "description"].includes(key)) {
    return "basic";
  }
  if (["nameZh", "nameEn", "descriptionZh", "descriptionEn", "titleZh", "titleEn"].includes(key)) {
    return "localization";
  }
  return undefined;
}

function fileRecordName(record: AdminResourceRecord, language: Language = "zh") {
  return localizedRecordValue(record, "name", language) || record.name || record.code || record.id;
}

function fileMimeType(record: AdminResourceRecord) {
  return record.values?.mimeType || "";
}

function fileSize(record: AdminResourceRecord) {
  const raw = Number(record.values?.size ?? 0);
  return Number.isFinite(raw) && raw > 0 ? raw : 0;
}

function fileStorageDriver(record: AdminResourceRecord) {
  return record.values?.storageDriver || "";
}

function filePreviewKind(record: AdminResourceRecord): "image" | "text" | "pdf" | "unsupported" {
  const mimeType = fileMimeType(record).toLowerCase();
  const name = fileRecordName(record).toLowerCase();
  if (mimeType.startsWith("image/") || /\.(png|jpe?g|gif|webp|svg)$/.test(name)) {
    return "image";
  }
  if (mimeType === "application/pdf" || name.endsWith(".pdf")) {
    return "pdf";
  }
  if (
    mimeType.startsWith("text/") ||
    ["application/json", "application/xml", "application/javascript", "application/x-ndjson"].includes(mimeType) ||
    /\.(txt|md|json|csv|log|xml|yml|yaml)$/.test(name)
  ) {
    return "text";
  }
  return "unsupported";
}

function formatBytes(bytes: number) {
  if (!bytes) {
    return "0 B";
  }
  const units = ["B", "KB", "MB", "GB", "TB"];
  const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  const value = bytes / 1024 ** index;
  return `${value >= 10 || index === 0 ? value.toFixed(0) : value.toFixed(1)} ${units[index]}`;
}

function downloadBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = filename || "download";
  document.body.append(anchor);
  anchor.click();
  anchor.remove();
  URL.revokeObjectURL(url);
}

function fileAuditActionLabel(action: string, dictionary: Dictionary) {
  const labels: Record<string, string> = {
    "file.upload": dictionary.fileOperationUpload,
    "file.content": dictionary.fileOperationContent,
    "file.delete": dictionary.fileOperationDelete,
  };
  return labels[action] ?? action;
}

function StatusTag({ status, dictionary }: { status: string; dictionary: Dictionary }) {
  return <Tag className={`resource-status status-${status}`}>{statusLabel(dictionary, status)}</Tag>;
}

function statusLabel(dictionary: Dictionary, status: string) {
  const labels: Record<string, string> = {
    enabled: dictionary.statusEnabled,
    disabled: dictionary.statusDisabled,
    healthy: dictionary.statusHealthy,
    recorded: dictionary.statusRecorded,
  };
  return labels[status] ?? status;
}

function boolLabel(dictionary: Dictionary, value: string) {
  return isTruthyValue(value) ? dictionary.yes : dictionary.no;
}

function tableRangeLabel(total: number, range: [number, number], dictionary: Dictionary) {
  return formatTemplate(dictionary.paginationTotal, {
    start: String(range[0]),
    end: String(range[1]),
    total: String(total),
  });
}

function resourceKeyFromRoute(route: string) {
  return route.replace(/^\//, "");
}

function formatDate(value?: string) {
  if (!value) {
    return "-";
  }
  return value.replace("T", " ").replace("Z", "");
}

function stripQuotes(value: string) {
  if ((value.startsWith("\"") && value.endsWith("\"")) || (value.startsWith("'") && value.endsWith("'"))) {
    return value.slice(1, -1);
  }
  return value;
}

function isTruthyValue(value: string) {
  return ["1", "true", "yes", "enabled", "on"].includes(value.toLowerCase());
}

function filterValueActive(value: PlatformDataTableFilterValue) {
  if (typeof value === "string") {
    return value.trim() !== "";
  }
  return Boolean(value.from || value.to);
}

function localizedFieldText(key: keyof Dictionary) {
  return {
    zh: String(dictionaries.zh[key]),
    en: String(dictionaries.en[key]),
  };
}

function formatTemplate(template: string, values: Record<string, string>) {
  return Object.entries(values).reduce((result, [key, value]) => result.replaceAll(`{${key}}`, value), template);
}

function copyIssuedToken(token: string) {
  if (!token) {
    return;
  }
  void navigator.clipboard?.writeText(token);
}
