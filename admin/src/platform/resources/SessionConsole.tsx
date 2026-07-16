import type { HttpError } from "@refinedev/core";
import { useList } from "@refinedev/core";
import { Tag, Typography } from "antd";
import { useState } from "react";
import type { AdminResourceRecord } from "../api/client";
import type { Dictionary, Language } from "../i18n";
import {
  AdminFeedback,
  AdminPage,
  PlatformDataTable,
  PlatformOverflowText,
  type PlatformDataTableColumn,
} from "../ui";
import type { AdminResourceDefinition } from "./registry";

type SessionRecord = AdminResourceRecord & {
  username?: unknown;
  tokenType?: unknown;
  expiresAt?: unknown;
};

type SessionConsoleProps = {
  resource: AdminResourceDefinition;
  language: Language;
  dictionary: Dictionary;
};

export function SessionConsole({ resource, language, dictionary }: SessionConsoleProps) {
  const [query, setQuery] = useState("");
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const listQuery = useList<SessionRecord, HttpError, SessionRecord>({
    resource: resource.name,
    pagination: { currentPage: page, pageSize, mode: "server" },
    meta: { keywords: query.trim() ? [query.trim()] : undefined },
  });
  const items = listQuery.result.data ?? [];
  const total = listQuery.result.total ?? items.length;
  const error = listQuery.query.error instanceof Error ? listQuery.query.error.message : "";
  const loading = listQuery.query.isLoading || listQuery.query.isFetching;
  const columns: PlatformDataTableColumn<SessionRecord>[] = [
    {
      title: dictionary.sessionUser,
      key: "username",
      dataIndex: "username",
      priority: "essential",
      render: (_value, record) => <PlatformOverflowText value={recordValue(record.username ?? record.name ?? record.code)} />,
    },
    {
      title: dictionary.sessionTokenType,
      key: "tokenType",
      dataIndex: "tokenType",
      priority: "standard",
      render: (_value, record) => <PlatformOverflowText value={recordValue(record.tokenType)} />,
    },
    {
      title: dictionary.sessionExpiresAt,
      key: "expiresAt",
      dataIndex: "expiresAt",
      priority: "standard",
      render: (_value, record) => <PlatformOverflowText value={recordValue(record.expiresAt)} />,
    },
    {
      title: dictionary.status,
      key: "status",
      dataIndex: "status",
      priority: "essential",
      render: (value) => <Tag>{recordValue(value)}</Tag>,
    },
  ];

  return (
    <AdminPage
      className="session-console"
      title={resource.title[language]}
      description={dictionary.sessionReadOnlyDescription}
      extra={<Tag>{dictionary.sessionReadOnlyBadge}</Tag>}
    >
      {error ? <AdminFeedback className="api-alert" type="warning" message={dictionary.loadResourceFailed} description={error} /> : null}
      <PlatformDataTable
        title={dictionary.sessionReadOnlyBadge}
        columns={columns}
        dataSource={items}
        rowKey="id"
        loading={loading}
        searchValue={query}
        searchPlaceholder={dictionary.searchResource}
        labels={tableLabels(dictionary)}
        emptyState={<Typography.Text type="secondary">{dictionary.sessionNoRecords}</Typography.Text>}
        pagination={{
          current: page,
          pageSize,
          total,
          showTotal: (nextTotal, range) => formatTemplate(dictionary.paginationRange, {
            start: String(range[0]),
            end: String(range[1]),
            total: String(nextTotal),
          }),
        }}
        mobileCards={(records) => (
          <div className="session-mobile-list">
            {records.map((record) => (
              <div className="session-mobile-card" key={record.id}>
                <strong>{recordValue(record.username ?? record.name ?? record.code)}</strong>
                <span>{recordValue(record.tokenType)}</span>
                <Tag>{recordValue(record.status)}</Tag>
              </div>
            ))}
          </div>
        )}
        onRefresh={() => void listQuery.query.refetch()}
        onSearchChange={(value) => {
          setQuery(value);
          setPage(1);
        }}
        onTableChange={(pagination) => {
          setPage(pagination.current ?? 1);
          setPageSize(pagination.pageSize ?? 10);
        }}
      />
    </AdminPage>
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
    empty: dictionary.sessionNoRecords,
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
    selectedColumns: (selected: number, total: number) => formatTemplate(dictionary.selectedColumns, { selected: String(selected), total: String(total) }),
    renderedColumns: (rendered: number, selected: number) => formatTemplate(dictionary.renderedColumns, { rendered: String(rendered), selected: String(selected) }),
    hiddenAtCurrentWidth: dictionary.hiddenAtCurrentWidth,
    selectAllColumns: dictionary.selectAllColumns,
    resetColumns: dictionary.resetColumns,
  };
}

function recordValue(value: unknown) {
  return value === undefined || value === null || value === "" ? "-" : String(value);
}

function formatTemplate(template: string, values: Record<string, string>) {
  return Object.entries(values).reduce((result, [key, value]) => result.replaceAll(`{${key}}`, value), template);
}
