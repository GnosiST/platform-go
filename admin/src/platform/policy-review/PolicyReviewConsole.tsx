import {
  AuditOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  DownloadOutlined,
  FileSearchOutlined,
  ReloadOutlined,
  SendOutlined,
} from "@ant-design/icons";
import { Button, Descriptions, Input, Modal, Popconfirm, Space, Tag, Timeline, Typography } from "antd";
import { useCallback, useEffect, useMemo, useState } from "react";
import {
  approveAdminPolicyReview,
  exportAdminPolicyReviews,
  queryAdminResource,
  rejectAdminPolicyReview,
  requestAdminPolicyReview,
  type AdminPolicyReviewExport,
  type AdminResourceRecord,
} from "../api/client";
import type { Dictionary, Language } from "../i18n";
import type { AdminResourceDefinition } from "../resources/registry";
import {
  AdminActionButton,
  AdminFeedback,
  AdminListPanel,
  AdminMetricStrip,
  AdminPage,
  PlatformDataTable,
  PlatformOverflowText,
  type PlatformDataTableColumn,
  type PlatformDataTableFilterValue,
} from "../ui";

type PolicyReviewConsoleProps = {
  resource: AdminResourceDefinition;
  language: Language;
  dictionary: Dictionary;
  permissions: string[];
  deniedPermissions: string[];
};

type RejectState = {
  record: AdminResourceRecord;
  reason: string;
};

export function PolicyReviewConsole({ resource, language, dictionary, permissions, deniedPermissions }: PolicyReviewConsoleProps) {
  const [reviews, setReviews] = useState<AdminResourceRecord[]>([]);
  const [audits, setAudits] = useState<AdminResourceRecord[]>([]);
  const [selectedID, setSelectedID] = useState("");
  const [searchValue, setSearchValue] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [loading, setLoading] = useState(true);
  const [actingID, setActingID] = useState("");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [rejectState, setRejectState] = useState<RejectState | null>(null);

  const loadPolicyReviews = useCallback(() => {
    setLoading(true);
    return Promise.all([
      queryAdminResource("policy-reviews", {
        sort: [{ field: "updatedAt", order: "desc" }],
        page: 1,
        pageSize: 200,
      }),
      queryAdminResource("audit-logs", {
        sort: [{ field: "createdAt", order: "desc" }],
        page: 1,
        pageSize: 200,
      }).catch(() => ({ items: [], total: 0, page: 1, pageSize: 200, resource: "audit-logs" })),
    ])
      .then(([reviewResult, auditResult]) => {
        setReviews(reviewResult.items);
        setAudits(auditResult.items.filter(isPolicyReviewAudit));
        setSelectedID((current) => current || reviewResult.items[0]?.id || "");
        setError("");
      })
      .catch((nextError: unknown) => {
        setError(nextError instanceof Error ? nextError.message : dictionary.policyReviewLoadFailed);
      })
      .finally(() => setLoading(false));
  }, [dictionary.policyReviewLoadFailed]);

  useEffect(() => {
    void loadPolicyReviews();
  }, [loadPolicyReviews]);

	const canRead = permissionAllows(permissions, "admin:policy-review:read", deniedPermissions);
	const canUpdate = permissionAllows(permissions, "admin:policy-review:update", deniedPermissions);
	const canExport = permissionAllows(permissions, "admin:policy-review:export", deniedPermissions);
  const filteredReviews = useMemo(
    () =>
      reviews.filter((review) => {
        const statusMatches = !statusFilter || policyReviewStatus(review) === statusFilter;
        const keyword = searchValue.trim().toLowerCase();
        if (!statusMatches) {
          return false;
        }
        if (!keyword) {
          return true;
        }
        return [review.code, review.name, review.description, valueOf(review, "roleCode"), valueOf(review, "permissionCodes"), valueOf(review, "dataScope")]
          .join(" ")
          .toLowerCase()
          .includes(keyword);
      }),
    [reviews, searchValue, statusFilter],
  );
  const selectedReview = filteredReviews.find((review) => review.id === selectedID) ?? filteredReviews[0] ?? reviews[0];
  const selectedAudits = selectedReview ? audits.filter((audit) => auditForReview(audit, selectedReview)) : [];
  const counts = useMemo(() => policyReviewCounts(reviews), [reviews]);

  const executeReviewAction = async (record: AdminResourceRecord, action: "request" | "approve" | "reject", reason = "") => {
    setActingID(`${record.id}:${action}`);
    setNotice("");
    try {
      if (action === "request") {
        await requestAdminPolicyReview(record.id);
        setNotice(dictionary.policyReviewRequested);
      } else if (action === "approve") {
        await approveAdminPolicyReview(record.id);
        setNotice(dictionary.policyReviewApproved);
      } else {
        await rejectAdminPolicyReview(record.id, reason);
        setNotice(dictionary.policyReviewRejected);
      }
      await loadPolicyReviews();
    } catch (nextError) {
      setError(nextError instanceof Error ? nextError.message : dictionary.policyReviewActionFailed);
    } finally {
      setActingID("");
    }
  };

  const exportEvidence = async () => {
    setActingID("export");
    setNotice("");
    try {
      const payload = await exportAdminPolicyReviews();
      downloadPolicyReviewExport(payload);
      setNotice(dictionary.policyReviewExported);
      await loadPolicyReviews();
    } catch (nextError) {
      setError(nextError instanceof Error ? nextError.message : dictionary.policyReviewExportFailed);
    } finally {
      setActingID("");
    }
  };

  if (!canRead) {
    return <AdminFeedback type="warning" message={dictionary.noPermission} description={resource.permission} />;
  }

  return (
    <AdminPage
      className="policy-review-console"
      title={dictionary.policyReviewTitle}
      description={dictionary.policyReviewDescription}
      actions={(
        <Space size={8}>
          <AdminActionButton icon={<ReloadOutlined />} label={dictionary.refresh} loading={loading} onClick={() => void loadPolicyReviews()} />
		  {canExport ? (
			<AdminActionButton icon={<DownloadOutlined />} label={dictionary.policyReviewExportEvidence} loading={actingID === "export"} onClick={() => void exportEvidence()}>
			  {dictionary.policyReviewExportEvidence}
			</AdminActionButton>
		  ) : null}
        </Space>
      )}
      summary={(
        <AdminMetricStrip
          columns={4}
          items={[
            { key: "total", label: dictionary.totalRecords, value: counts.total },
            { key: "pending", label: dictionary.policyReviewPending, value: counts.pending, tone: "warning" },
            { key: "approved", label: dictionary.policyReviewApprovedStatus, value: counts.approved, tone: "accent" },
            { key: "rejected", label: dictionary.policyReviewRejectedStatus, value: counts.rejected, tone: "danger" },
          ]}
        />
      )}
    >
      {error ? <AdminFeedback className="api-alert" type="warning" message={dictionary.policyReviewActionFailed} description={error} /> : null}
      {notice ? <AdminFeedback className="api-alert" type="success" message={notice} /> : null}
      <div className="policy-review-workbench">
        <AdminListPanel className="policy-review-list-panel" title={dictionary.policyReviewQueue}>
          <PlatformDataTable<AdminResourceRecord>
            columns={policyReviewColumns(dictionary, language, canUpdate, actingID, executeReviewAction, (record) => setRejectState({ record, reason: "" }))}
            dataSource={filteredReviews}
            filterFields={[
              {
                key: "reviewStatus",
                label: dictionary.policyReviewStatus,
                type: "select",
                options: [
                  { value: "draft", label: dictionary.policyReviewDraft },
                  { value: "pending", label: dictionary.policyReviewPending },
                  { value: "approved", label: dictionary.policyReviewApprovedStatus },
                  { value: "rejected", label: dictionary.policyReviewRejectedStatus },
                ],
              },
            ]}
            filterValues={{ reviewStatus: statusFilter }}
            labels={tableLabels(dictionary)}
            loading={loading}
            pagination={{ pageSize: 10, total: filteredReviews.length }}
            rowKey="id"
            scrollX={1040}
            searchPlaceholder={dictionary.policyReviewSearch}
            searchValue={searchValue}
            selectedRowKey={selectedReview?.id}
            onClearFilters={() => setStatusFilter("")}
            onFilterChange={(_key, value) => setStatusFilter(typeof value === "string" ? value : "")}
            onRefresh={() => void loadPolicyReviews()}
            onRowClick={(record) => setSelectedID(record.id)}
            onSearchChange={setSearchValue}
          />
        </AdminListPanel>

        <PolicyReviewInspector dictionary={dictionary} language={language} review={selectedReview} audits={selectedAudits} />
      </div>
      <Modal
        className="policy-review-reject-modal"
        okButtonProps={{ disabled: !rejectState?.reason.trim(), loading: actingID.endsWith(":reject") }}
        okText={dictionary.policyReviewRejectReview}
        open={Boolean(rejectState)}
        title={dictionary.policyReviewRejectTitle}
        onCancel={() => setRejectState(null)}
        onOk={() => {
          if (!rejectState?.reason.trim()) {
            return;
          }
          void executeReviewAction(rejectState.record, "reject", rejectState.reason).then(() => setRejectState(null));
        }}
      >
        <Typography.Paragraph>{dictionary.policyReviewRejectDescription}</Typography.Paragraph>
        <Input.TextArea
          aria-label={dictionary.policyReviewReason}
          autoSize={{ minRows: 4, maxRows: 6 }}
          placeholder={dictionary.policyReviewReasonRequired}
          value={rejectState?.reason ?? ""}
          onChange={(event) => setRejectState((current) => (current ? { ...current, reason: event.target.value } : current))}
        />
      </Modal>
    </AdminPage>
  );
}

function policyReviewColumns(
  dictionary: Dictionary,
  language: Language,
  canUpdate: boolean,
  actingID: string,
  onAction: (record: AdminResourceRecord, action: "request" | "approve" | "reject") => Promise<void>,
  onReject: (record: AdminResourceRecord) => void,
): PlatformDataTableColumn<AdminResourceRecord>[] {
  return [
    {
      key: "name",
      title: dictionary.policyReviewPolicy,
      width: 230,
      sorter: (left, right) => left.name.localeCompare(right.name),
      render: (_value, record) => (
        <div className="resource-name-cell">
          <PlatformOverflowText strong value={record.name || record.code} />
          <Typography.Text className="secondary-text">{record.code}</Typography.Text>
        </div>
      ),
    },
    {
      key: "roleCode",
      title: dictionary.policyReviewTargetRole,
      width: 140,
      render: (_value, record) => <Tag>{valueOf(record, "roleCode") || "-"}</Tag>,
    },
    {
      key: "policyType",
      title: dictionary.policyReviewType,
      width: 160,
      render: (_value, record) => policyTypeLabel(dictionary, valueOf(record, "policyType")),
    },
    {
      key: "reviewStatus",
      title: dictionary.policyReviewStatus,
      width: 124,
      sorter: (left, right) => policyReviewStatus(left).localeCompare(policyReviewStatus(right)),
      render: (_value, record) => <PolicyReviewStatusTag dictionary={dictionary} status={policyReviewStatus(record)} />,
    },
    {
      key: "updatedAt",
      title: dictionary.updatedAt,
      width: 168,
      sorter: (left, right) => left.updatedAt.localeCompare(right.updatedAt),
      render: (_value, record) => formatDateTime(record.updatedAt, language),
    },
    {
      key: "actions",
      title: dictionary.actions,
      width: 220,
      fixed: "right",
      render: (_value, record) => (
        <Space size={6} onClick={(event) => event.stopPropagation()}>
          {policyReviewStatus(record) === "draft" ? (
            <Popconfirm
              title={dictionary.policyReviewRequestReview}
              okText={dictionary.policyReviewRequestReview}
              cancelText={dictionary.cancel}
              disabled={!canUpdate}
              onConfirm={() => void onAction(record, "request")}
            >
              <AdminActionButton disabled={!canUpdate} icon={<SendOutlined />} label={dictionary.policyReviewRequestReview} loading={actingID === `${record.id}:request`} size="small" />
            </Popconfirm>
          ) : null}
          {policyReviewStatus(record) === "pending" ? (
            <>
              <Popconfirm
                title={dictionary.policyReviewApproveReview}
                okText={dictionary.policyReviewApproveReview}
                cancelText={dictionary.cancel}
                disabled={!canUpdate}
                onConfirm={() => void onAction(record, "approve")}
              >
                <AdminActionButton disabled={!canUpdate} icon={<CheckCircleOutlined />} label={dictionary.policyReviewApproveReview} loading={actingID === `${record.id}:approve`} size="small" type="primary" />
              </Popconfirm>
              <AdminActionButton danger disabled={!canUpdate} icon={<CloseCircleOutlined />} label={dictionary.policyReviewRejectReview} size="small" onClick={() => onReject(record)} />
            </>
          ) : null}
          {policyReviewStatus(record) !== "draft" && policyReviewStatus(record) !== "pending" ? (
            <Typography.Text className="secondary-text">{statusLabel(dictionary, policyReviewStatus(record))}</Typography.Text>
          ) : null}
        </Space>
      ),
    },
  ];
}

function PolicyReviewInspector({
  dictionary,
  language,
  review,
  audits,
}: {
  dictionary: Dictionary;
  language: Language;
  review?: AdminResourceRecord;
  audits: AdminResourceRecord[];
}) {
  if (!review) {
    return (
      <AdminListPanel className="policy-review-inspector" title={dictionary.policyReviewInspector}>
        <AdminFeedback type="info" message={dictionary.policyReviewNoSelection} />
      </AdminListPanel>
    );
  }

  return (
    <AdminListPanel className="policy-review-inspector" title={dictionary.policyReviewInspector} actions={<FileSearchOutlined className="secondary-text" />}>
      <Descriptions bordered size="small" column={1}>
        <Descriptions.Item label={dictionary.policyReviewPolicy}>{review.name}</Descriptions.Item>
        <Descriptions.Item label={dictionary.policyReviewStatus}><PolicyReviewStatusTag dictionary={dictionary} status={policyReviewStatus(review)} /></Descriptions.Item>
        <Descriptions.Item label={dictionary.policyReviewTargetRole}>{valueOf(review, "roleCode") || "-"}</Descriptions.Item>
        <Descriptions.Item label={dictionary.policyReviewRequestedBy}>{valueOf(review, "requestedBy") || "-"}</Descriptions.Item>
        <Descriptions.Item label={dictionary.policyReviewReviewedBy}>{valueOf(review, "reviewedBy") || "-"}</Descriptions.Item>
      </Descriptions>

      <section className="policy-review-diff">
        <Typography.Text strong>{dictionary.policyReviewDiff}</Typography.Text>
        <div className="policy-review-diff-grid">
          <div>
            <Typography.Text type="secondary">{dictionary.policyReviewType}</Typography.Text>
            <strong>{policyTypeLabel(dictionary, valueOf(review, "policyType"))}</strong>
          </div>
          <div>
            <Typography.Text type="secondary">{dictionary.policyReviewRequestedAction}</Typography.Text>
            <strong>{valueOf(review, "requestedAction") || "-"}</strong>
          </div>
          <div>
            <Typography.Text type="secondary">{dictionary.policyReviewPermissionCodes}</Typography.Text>
            <PlatformOverflowText code value={valueOf(review, "permissionCodes") || "-"} />
          </div>
          <div>
            <Typography.Text type="secondary">{dictionary.policyReviewDataScope}</Typography.Text>
            <PlatformOverflowText value={dataScopeSummary(dictionary, review)} />
          </div>
        </div>
      </section>

      <section className="policy-review-audit">
        <Typography.Text strong>
          <AuditOutlined /> {dictionary.policyReviewAuditTrail}
        </Typography.Text>
        {audits.length === 0 ? (
          <AdminFeedback type="info" message={dictionary.policyReviewNoAudit} />
        ) : (
          <Timeline
            items={audits.map((audit) => ({
              children: (
                <div className="policy-review-audit-item">
                  <Typography.Text strong>{audit.name || valueOf(audit, "action")}</Typography.Text>
                  <Typography.Text className="secondary-text">{formatDateTime(audit.updatedAt, language)}</Typography.Text>
                  <Typography.Text className="secondary-text">{valueOf(audit, "actor") || "-"}</Typography.Text>
                </div>
              ),
            }))}
          />
        )}
      </section>
    </AdminListPanel>
  );
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

function policyReviewCounts(reviews: AdminResourceRecord[]) {
  return reviews.reduce(
    (counts, review) => {
      const status = policyReviewStatus(review);
      counts.total += 1;
      if (status === "pending") {
        counts.pending += 1;
      } else if (status === "approved") {
        counts.approved += 1;
      } else if (status === "rejected") {
        counts.rejected += 1;
      }
      return counts;
    },
    { total: 0, pending: 0, approved: 0, rejected: 0 },
  );
}

function PolicyReviewStatusTag({ dictionary, status }: { dictionary: Dictionary; status: string }) {
  const color = status === "approved" ? "green" : status === "pending" ? "gold" : status === "rejected" ? "red" : "default";
  return <Tag color={color}>{statusLabel(dictionary, status)}</Tag>;
}

function policyReviewStatus(record: AdminResourceRecord) {
  return valueOf(record, "reviewStatus") || record.status || "draft";
}

function statusLabel(dictionary: Dictionary, status: string) {
  switch (status) {
  case "draft":
    return dictionary.policyReviewDraft;
  case "pending":
    return dictionary.policyReviewPending;
  case "approved":
    return dictionary.policyReviewApprovedStatus;
  case "rejected":
    return dictionary.policyReviewRejectedStatus;
  default:
    return status || "-";
  }
}

function policyTypeLabel(dictionary: Dictionary, policyType: string) {
  switch (policyType) {
  case "role_permission":
    return dictionary.policyReviewRolePermission;
  case "deny_permission":
    return dictionary.policyReviewDenyPermission;
  case "data_scope":
    return dictionary.policyReviewDataScope;
  default:
    return policyType || "-";
  }
}

function dataScopeSummary(dictionary: Dictionary, record: AdminResourceRecord) {
  const dataScope = valueOf(record, "dataScope");
  const orgCodes = valueOf(record, "dataScopeOrgCodes");
  const areaCodes = valueOf(record, "dataScopeAreaCodes");
  return [dataScope, orgCodes && `${dictionary.policyReviewOrgScope}: ${orgCodes}`, areaCodes && `${dictionary.policyReviewAreaScope}: ${areaCodes}`]
    .filter(Boolean)
    .join(" · ") || "-";
}

function auditForReview(audit: AdminResourceRecord, review: AdminResourceRecord) {
	return valueOf(audit, "targetId") === review.id;
}

function isPolicyReviewAudit(record: AdminResourceRecord) {
  return valueOf(record, "provider") === "policy-review" || valueOf(record, "action").startsWith("policy-review.");
}

function valueOf(record: AdminResourceRecord, key: string) {
  return record.values?.[key] || "";
}

function permissionAllows(permissions: string[], permission: string, deniedPermissions: string[] = []) {
  if (deniedPermissions.some((granted) => granted === "*" || granted === permission || (granted.endsWith(":*") && permission.startsWith(granted.slice(0, -1))))) {
    return false;
  }
  return permissions.some((granted) => granted === "*" || granted === permission || (granted.endsWith(":*") && permission.startsWith(granted.slice(0, -1))));
}

function downloadPolicyReviewExport(payload: AdminPolicyReviewExport) {
  const blob = new Blob([JSON.stringify(payload, null, 2)], { type: "application/json" });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = `policy-review-export-${payload.exportedAt || new Date().toISOString()}.json`;
  link.click();
  URL.revokeObjectURL(url);
}

function formatDateTime(value: string, language: Language) {
  if (!value) {
    return "-";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return new Intl.DateTimeFormat(language === "zh" ? "zh-CN" : "en-US", {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(date);
}

function formatTemplate(template: string, values: Record<string, string>) {
  return Object.entries(values).reduce((result, [key, value]) => result.replaceAll(`{${key}}`, value), template);
}
