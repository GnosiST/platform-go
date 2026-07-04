import {
  CopyOutlined,
  DeleteOutlined,
  EditOutlined,
  PlusOutlined,
  ReloadOutlined,
  SearchOutlined,
} from "@ant-design/icons";
import { Alert, Button, Form, Input, Modal, Popconfirm, Space, Spin, Table, Tag, Typography } from "antd";
import { useCallback, useEffect, useMemo, useState } from "react";
import {
  createAdminResource,
  deleteAdminResource,
  listAdminResource,
  updateAdminResource,
  type AdminResourceInput,
  type AdminResourceRecord,
} from "../api/client";
import type { Dictionary, Language } from "../i18n";
import type { AdminResourceDefinition } from "./registry";

type GenericResourceConsoleProps = {
  resource: AdminResourceDefinition;
  language: Language;
  dictionary: Dictionary;
};

type ResourceFormValues = {
  code?: string;
  name: string;
  status?: string;
  description?: string;
  valuesText?: string;
};

const statusOptions = ["enabled", "disabled", "healthy", "recorded"];

export function GenericResourceConsole({ resource, language, dictionary }: GenericResourceConsoleProps) {
  const [items, setItems] = useState<AdminResourceRecord[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [query, setQuery] = useState("");
  const [selectedID, setSelectedID] = useState("");
  const [editingRecord, setEditingRecord] = useState<AdminResourceRecord | null>(null);
  const [modalOpen, setModalOpen] = useState(false);
  const [form] = Form.useForm<ResourceFormValues>();

  const resourceKey = resourceKeyFromRoute(resource.route);

  const loadItems = useCallback(() => {
    setLoading(true);
    listAdminResource(resourceKey)
      .then((payload) => {
        setItems(payload.items);
        setError("");
        setSelectedID((current) => current || payload.items[0]?.id || "");
      })
      .catch((nextError: unknown) => {
        setError(nextError instanceof Error ? nextError.message : dictionary.loadResourceFailed);
      })
      .finally(() => setLoading(false));
  }, [dictionary.loadResourceFailed, resourceKey]);

  useEffect(() => {
    setItems([]);
    setSelectedID("");
    setQuery("");
    loadItems();
  }, [loadItems]);

  const filteredItems = useMemo(() => {
    const normalizedQuery = query.trim().toLowerCase();
    if (!normalizedQuery) {
      return items;
    }
    return items.filter((item) =>
      [item.id, item.code, item.name, item.status, item.description, ...Object.values(item.values ?? {})]
        .join(" ")
        .toLowerCase()
        .includes(normalizedQuery),
    );
  }, [items, query]);

  const selectedRecord = useMemo(
    () => items.find((item) => item.id === selectedID) ?? filteredItems[0] ?? items[0],
    [filteredItems, items, selectedID],
  );

  const activeCount = items.filter((item) => item.status !== "disabled").length;
  const latestUpdate = items
    .map((item) => item.updatedAt)
    .filter(Boolean)
    .sort()
    .at(-1);

  const openCreate = () => {
    setEditingRecord(null);
    form.setFieldsValue({ status: "enabled", valuesText: "" });
    setModalOpen(true);
  };

  const openEdit = (record: AdminResourceRecord) => {
    setEditingRecord(record);
    form.setFieldsValue({
      code: record.code,
      name: record.name,
      status: record.status,
      description: record.description,
      valuesText: formatValues(record.values),
    });
    setModalOpen(true);
  };

  const submitForm = async (values: ResourceFormValues) => {
    const input: AdminResourceInput = {
      code: values.code,
      name: values.name,
      status: values.status,
      description: values.description,
      values: parseValues(values.valuesText),
    };
    setSaving(true);
    try {
      const result = editingRecord
        ? await updateAdminResource(resourceKey, editingRecord.id, input)
        : await createAdminResource(resourceKey, input);
      setItems((current) =>
        editingRecord
          ? current.map((item) => (item.id === result.record.id ? result.record : item))
          : [result.record, ...current],
      );
      setSelectedID(result.record.id);
      setModalOpen(false);
      setError("");
    } catch (nextError) {
      setError(nextError instanceof Error ? nextError.message : dictionary.loadResourceFailed);
    } finally {
      setSaving(false);
    }
  };

  const removeRecord = async (record: AdminResourceRecord) => {
    await deleteAdminResource(resourceKey, record.id);
    setItems((current) => current.filter((item) => item.id !== record.id));
    setSelectedID((current) => (current === record.id ? "" : current));
  };

  const columns = [
    {
      title: dictionary.recordName,
      dataIndex: "name",
      key: "name",
      width: 180,
      render: (_: unknown, record: AdminResourceRecord) => (
        <div className="resource-name-cell">
          <Typography.Text strong>{record.name}</Typography.Text>
          <Typography.Text className="secondary-text">{record.id}</Typography.Text>
        </div>
      ),
    },
    {
      title: dictionary.recordCode,
      dataIndex: "code",
      key: "code",
      width: 170,
      render: (code: string) => <Typography.Text code>{code}</Typography.Text>,
    },
    {
      title: dictionary.status,
      dataIndex: "status",
      key: "status",
      width: 120,
      render: (status: string) => <StatusTag status={status} dictionary={dictionary} />,
    },
    {
      title: dictionary.description,
      dataIndex: "description",
      key: "description",
      ellipsis: true,
    },
    {
      title: dictionary.actions,
      key: "actions",
      fixed: "right" as const,
      width: 128,
      render: (_: unknown, record: AdminResourceRecord) => (
        <Space size={4}>
          <Button icon={<CopyOutlined />} size="small" type="text" onClick={() => setSelectedID(record.id)} />
          <Button icon={<EditOutlined />} size="small" type="text" onClick={() => openEdit(record)} />
          <Popconfirm title={dictionary.deleteConfirm} okText={dictionary.deleteRecord} cancelText={dictionary.cancel} onConfirm={() => removeRecord(record)}>
            <Button danger icon={<DeleteOutlined />} size="small" type="text" />
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <section className="generic-resource-console">
      <div className="page-heading">
        <div>
          <Typography.Title level={1}>{resource.title[language]}</Typography.Title>
          <Typography.Paragraph>{resource.description[language]}</Typography.Paragraph>
        </div>
        <Space className="page-actions">
          <Button icon={<ReloadOutlined />} onClick={loadItems}>
            {dictionary.refresh}
          </Button>
          <Button icon={<PlusOutlined />} type="primary" onClick={openCreate}>
            {dictionary.addRecord}
          </Button>
        </Space>
      </div>

      {error ? <Alert className="api-alert" type="warning" message={dictionary.loadResourceFailed} description={error} showIcon /> : null}

      <div className="resource-summary-band">
        <ResourceMetric label={dictionary.totalRecords} value={items.length} />
        <ResourceMetric label={dictionary.activeRecords} value={activeCount} accent />
        <ResourceMetric label={dictionary.latestUpdate} value={latestUpdate ? formatDate(latestUpdate) : "-"} />
      </div>

      <div className="resource-grid">
        <div className="resource-workspace">
          <div className="table-toolbar">
            <Typography.Text strong>{dictionary.resourceList}</Typography.Text>
            <div className="table-actions">
              <Input
                className="capability-search"
                prefix={<SearchOutlined />}
                placeholder={dictionary.searchResource}
                value={query}
                onChange={(event) => setQuery(event.target.value)}
              />
              <Button icon={<ReloadOutlined />} onClick={loadItems} />
            </div>
          </div>

          {loading ? (
            <div className="loading-panel">
              <Spin />
            </div>
          ) : (
            <>
              <Table
                className="resource-table"
                columns={columns}
                dataSource={filteredItems}
                pagination={false}
                rowKey="id"
                rowClassName={(record) => (record.id === selectedRecord?.id ? "selected-row" : "")}
                scroll={{ x: 920 }}
                onRow={(record) => ({
                  onClick: () => setSelectedID(record.id),
                })}
              />
              <div className="resource-mobile-list">
                {filteredItems.map((item) => (
                  <button
                    className={item.id === selectedRecord?.id ? "mobile-resource-card active" : "mobile-resource-card"}
                    key={item.id}
                    type="button"
                    onClick={() => setSelectedID(item.id)}
                  >
                    <span>
                      <strong>{item.name}</strong>
                      <em>{item.code}</em>
                    </span>
                    <StatusTag status={item.status} dictionary={dictionary} />
                  </button>
                ))}
              </div>
            </>
          )}
        </div>

        <ResourceInspector record={selectedRecord} dictionary={dictionary} onEdit={openEdit} />
      </div>

      <Modal
        title={editingRecord ? dictionary.editRecord : dictionary.addRecord}
        open={modalOpen}
        okText={dictionary.save}
        cancelText={dictionary.cancel}
        confirmLoading={saving}
        onCancel={() => setModalOpen(false)}
        onOk={() => form.submit()}
      >
        <Form form={form} layout="vertical" onFinish={submitForm}>
          <Form.Item label={dictionary.recordName} name="name" rules={[{ required: true, message: dictionary.recordNameRequired }]}>
            <Input />
          </Form.Item>
          <Form.Item label={dictionary.recordCode} name="code">
            <Input />
          </Form.Item>
          <Form.Item label={dictionary.status} name="status">
            <Input list="resource-status-options" />
          </Form.Item>
          <datalist id="resource-status-options">
            {statusOptions.map((status) => (
              <option key={status} value={status} />
            ))}
          </datalist>
          <Form.Item label={dictionary.description} name="description">
            <Input.TextArea rows={3} />
          </Form.Item>
          <Form.Item label={dictionary.values} name="valuesText">
            <Input.TextArea rows={4} placeholder={dictionary.valuesPlaceholder} />
          </Form.Item>
        </Form>
      </Modal>
    </section>
  );
}

function ResourceMetric({ label, value, accent }: { label: string; value: number | string; accent?: boolean }) {
  return (
    <div className="metric">
      <Typography.Text>{label}</Typography.Text>
      <strong className={accent ? "accent" : ""}>{value}</strong>
    </div>
  );
}

function ResourceInspector({
  record,
  dictionary,
  onEdit,
}: {
  record?: AdminResourceRecord;
  dictionary: Dictionary;
  onEdit: (record: AdminResourceRecord) => void;
}) {
  if (!record) {
    return <aside className="resource-inspector empty" />;
  }
  const values = Object.entries(record.values ?? {});
  return (
    <aside className="resource-inspector">
      <div className="inspector-header">
        <div>
          <Typography.Title level={3}>{record.name}</Typography.Title>
          <StatusTag status={record.status} dictionary={dictionary} />
        </div>
      </div>
      <dl className="detail-list">
        <div>
          <dt>{dictionary.recordCode}</dt>
          <dd>{record.code}</dd>
        </div>
        <div>
          <dt>{dictionary.status}</dt>
          <dd>{statusLabel(dictionary, record.status)}</dd>
        </div>
        <div>
          <dt>{dictionary.updatedAt}</dt>
          <dd>{formatDate(record.updatedAt)}</dd>
        </div>
        <div>
          <dt>{dictionary.description}</dt>
          <dd>{record.description || "-"}</dd>
        </div>
      </dl>
      <section className="inspector-section">
        <Typography.Text strong>{dictionary.values}</Typography.Text>
        {values.length > 0 ? (
          <div className="resource-values-list">
            {values.map(([key, value]) => (
              <div key={key}>
                <Typography.Text code>{key}</Typography.Text>
                <span>{value}</span>
              </div>
            ))}
          </div>
        ) : (
          <Typography.Text className="secondary-text">{dictionary.noValues}</Typography.Text>
        )}
      </section>
      <div className="inspector-actions">
        <Button icon={<EditOutlined />} type="primary" onClick={() => onEdit(record)}>
          {dictionary.editRecord}
        </Button>
      </div>
    </aside>
  );
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

function resourceKeyFromRoute(route: string) {
  return route.replace(/^\//, "");
}

function formatDate(value?: string) {
  if (!value) {
    return "-";
  }
  return value.replace("T", " ").replace("Z", "");
}

function parseValues(value?: string) {
  const lines = (value ?? "")
    .split("\n")
    .map((line) => line.trim())
    .filter(Boolean);
  if (lines.length === 0) {
    return undefined;
  }
  return Object.fromEntries(
    lines.map((line) => {
      const separator = line.indexOf("=");
      if (separator < 0) {
        return [line, ""];
      }
      return [line.slice(0, separator).trim(), line.slice(separator + 1).trim()];
    }),
  );
}

function formatValues(values?: Record<string, string>) {
  return Object.entries(values ?? {})
    .map(([key, value]) => `${key}=${value}`)
    .join("\n");
}
