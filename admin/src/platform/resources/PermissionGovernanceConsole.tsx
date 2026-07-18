import { Descriptions, Spin, Tag, Typography } from "antd";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { queryAdminResource, type AdminResourceRecord } from "../api/client";
import type { Dictionary, Language } from "../i18n";
import { hasPermission } from "../refine";
import {
  AdminFeedback,
  AdminListPanel,
  AdminPage,
  AdminTreeWorkbench,
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

export function PermissionGovernanceConsole({ resource, language, dictionary, permissions, deniedPermissions }: PermissionGovernanceConsoleProps) {
  const [records, setRecords] = useState<AdminResourceRecord[]>([]);
  const [selectedKey, setSelectedKey] = useState("");
  const [search, setSearch] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const requestRef = useRef(0);
  const canRead = hasPermission(permissions, "admin:permission:read", deniedPermissions);

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

  const treeModel = useMemo(
    () => projectPermissionTree(records, search, dictionary, language),
    [dictionary, language, records, search],
  );

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

  const selectedRecord = treeModel.permissions.get(selectedKey);
  const selectedSummary = treeModel.summaries.get(selectedKey);
  const detail = loading ? (
    <AdminListPanel className="permission-governance-detail" title={dictionary.permissionDetailTitle}>
      <div className="loading-panel" aria-live="polite"><Spin size="small" /></div>
    </AdminListPanel>
  ) : selectedRecord ? (
    <PermissionDetail record={selectedRecord} dictionary={dictionary} language={language} />
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
      <AdminFeedback className="api-alert permission-readonly-alert" type="info" message={dictionary.permissionReadOnlySeededTitle} description={dictionary.permissionReadOnlySeededDescription} />
      <AdminTreeWorkbench
        ariaLabel={dictionary.permissionTreeAriaLabel}
        detail={detail}
        emptyText={dictionary.emptyData}
        loading={loading}
        nodes={treeModel.nodes}
        searchLabel={dictionary.permissionTreeSearch}
        searchPlaceholder={dictionary.permissionTreeSearchPlaceholder}
        searchValue={search}
        selectedKey={selectedKey}
        title={dictionary.permissionTreeTitle}
        onSearchChange={setSearch}
        onSelect={setSelectedKey}
      />
    </AdminPage>
  );
}

function PermissionDetail({ record, dictionary, language }: { record: AdminResourceRecord; dictionary: Dictionary; language: Language }) {
  const resourceType = valueOf(record, "resourceType") || "api";
  return (
    <AdminListPanel className="permission-governance-detail" title={dictionary.permissionDetailTitle}>
      <div className="permission-governance-detail-body">
        <div className="permission-governance-detail-heading">
          <div>
            <Typography.Text className="permission-governance-detail-kicker" type="secondary">{permissionTypeLabel(resourceType, dictionary)}</Typography.Text>
            <Typography.Title level={4}>{localizedPermissionName(record, language)}</Typography.Title>
            <Typography.Text code>{record.code}</Typography.Text>
          </div>
          <Tag color={record.status === "enabled" ? "success" : "default"}>{statusLabel(record.status, dictionary)}</Tag>
        </div>
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
    const permissionKey = `permission:${record.code}`;
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
