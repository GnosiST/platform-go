import { DatabaseOutlined, PlayCircleOutlined, ReloadOutlined } from "@ant-design/icons";
import { Popconfirm, Space, Table, Tag, Typography } from "antd";
import { useCallback, useEffect, useMemo, useState } from "react";
import {
  applyAdminDemoData,
  listAdminDemoData,
  type AdminDemoDataItem,
  type AdminDemoDataApplyResult,
} from "../api/client";
import type { Dictionary, Language } from "../i18n";
import { AdminActionButton, AdminFeedback, AdminListPanel, AdminMetricStrip, AdminPage } from "../ui";

type DemoDataConsoleProps = {
  language: Language;
  dictionary: Dictionary;
  permissions?: string[];
  deniedPermissions?: string[];
};

export function DemoDataConsole({ language, dictionary, permissions = ["*"], deniedPermissions = [] }: DemoDataConsoleProps) {
  const [items, setItems] = useState<AdminDemoDataItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [applyingID, setApplyingID] = useState("");
  const [error, setError] = useState("");
  const [result, setResult] = useState<AdminDemoDataApplyResult | null>(null);
  const canApply = useMemo(() => permissionAllows(permissions, "admin:demo-data:apply", deniedPermissions), [deniedPermissions, permissions]);

  const loadDemoData = useCallback(() => {
    setLoading(true);
    listAdminDemoData()
      .then((payload) => {
        setItems(payload.items);
        setError("");
      })
      .catch((nextError: unknown) => {
        setError(nextError instanceof Error ? nextError.message : dictionary.loadDemoDataFailed);
      })
      .finally(() => setLoading(false));
  }, [dictionary.loadDemoDataFailed]);

  useEffect(() => {
    loadDemoData();
  }, [loadDemoData]);

  const applyDataset = async (item: AdminDemoDataItem) => {
    setApplyingID(item.id);
    try {
      const nextResult = await applyAdminDemoData(item.capabilityId, item.id);
      setResult(nextResult);
      setError("");
    } catch (nextError) {
      setError(nextError instanceof Error ? nextError.message : dictionary.applyDemoDataFailed);
    } finally {
      setApplyingID("");
    }
  };

  const columns = [
    {
      title: dictionary.demoDataSet,
      key: "dataset",
      render: (_: unknown, item: AdminDemoDataItem) => (
        <div className="resource-name-cell">
          <Typography.Text strong>{localizedText(item.title, language)}</Typography.Text>
          <Typography.Text className="secondary-text">{item.id}</Typography.Text>
        </div>
      ),
    },
    {
      title: dictionary.capability,
      dataIndex: "capabilityId",
      key: "capabilityId",
      width: 160,
      render: (value: string) => <Typography.Text code>{value}</Typography.Text>,
    },
    {
      title: dictionary.resource,
      dataIndex: "resource",
      key: "resource",
      width: 150,
      render: (value: string) => <Tag>{value}</Tag>,
    },
    {
      title: dictionary.records,
      dataIndex: "records",
      key: "records",
      width: 110,
    },
    {
      title: dictionary.description,
      key: "description",
      ellipsis: true,
      render: (_: unknown, item: AdminDemoDataItem) => localizedText(item.description, language),
    },
    {
      title: dictionary.actions,
      key: "actions",
      fixed: "right" as const,
      width: 130,
      render: (_: unknown, item: AdminDemoDataItem) =>
        canApply ? (
          <Popconfirm
            title={dictionary.applyDemoDataConfirm}
            okText={dictionary.apply}
            cancelText={dictionary.cancel}
            onConfirm={() => applyDataset(item)}
          >
            <AdminActionButton
              icon={<PlayCircleOutlined />}
              label={dictionary.apply}
              loading={applyingID === item.id}
              size="small"
              type="primary"
            >
              {dictionary.apply}
            </AdminActionButton>
          </Popconfirm>
        ) : (
          <Typography.Text className="secondary-text">{dictionary.noPermission}</Typography.Text>
        ),
    },
  ];

  return (
    <AdminPage
      className="demo-data-console"
      title={dictionary.demoData}
      description={dictionary.demoDataDescription}
      actions={
        <AdminActionButton icon={<ReloadOutlined />} label={dictionary.refresh} onClick={loadDemoData}>
          {dictionary.refresh}
        </AdminActionButton>
      }
      summary={
        <AdminMetricStrip
          columns={3}
          items={[
            { key: "datasets", label: dictionary.demoDataSets, value: items.length },
            { key: "records", label: dictionary.records, value: items.reduce((sum, item) => sum + item.records, 0), tone: "accent" },
            { key: "permission", label: dictionary.applyPermission, value: canApply ? dictionary.enabled : dictionary.disabled },
          ]}
        />
      }
    >
      {error ? <AdminFeedback className="api-alert" type="warning" message={dictionary.loadDemoDataFailed} description={error} /> : null}
      {result ? (
        <AdminFeedback
          className="api-alert"
          type="success"
          message={dictionary.applyDemoDataSucceeded}
          description={`${result.resource}: ${result.applied}`}
        />
      ) : null}

      <AdminListPanel
        className="demo-data-workspace"
        title={dictionary.demoDataSets}
        actions={<DatabaseOutlined className="secondary-text" />}
      >
        <Table
          className="resource-table platform-data-table"
          columns={columns}
          dataSource={items}
          loading={loading}
          pagination={false}
          rowKey={(item) => `${item.capabilityId}:${item.id}`}
          scroll={{ x: 980 }}
          size="small"
        />
      </AdminListPanel>
    </AdminPage>
  );
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
