import { ApiOutlined, CopyOutlined, DownloadOutlined, LinkOutlined, ReloadOutlined } from "@ant-design/icons";
import { App, Button, Space, Tag, Typography } from "antd";
import { useCallback, useEffect, useMemo, useState } from "react";
import { getAdminOpenAPI, type AdminOpenAPIDocument, type AdminOpenAPIOperation } from "../api/client";
import type { Dictionary, Language } from "../i18n";
import {
  AdminActionButton,
  AdminFeedback,
  AdminMetricStrip,
  AdminPage,
  PlatformDataTable,
  PlatformOverflowText,
  type PlatformDataTableColumn,
  type PlatformDataTableFilterField,
  type PlatformDataTableFilterValue,
} from "../ui";

type APIDocsPageProps = {
  language: Language;
  dictionary: Dictionary;
};

type APIOperationRow = {
  key: string;
  method: string;
  path: string;
  operationId: string;
  permission: string;
  resource: string;
  description: string;
};

const methodTones: Record<string, string> = {
  GET: "blue",
  POST: "green",
  PUT: "gold",
  PATCH: "purple",
  DELETE: "red",
};

export function APIDocsPage({ dictionary }: APIDocsPageProps) {
  const { message } = App.useApp();
  const [document, setDocument] = useState<AdminOpenAPIDocument | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [query, setQuery] = useState("");
  const [filters, setFilters] = useState<Record<string, PlatformDataTableFilterValue>>({});

  const loadDocument = useCallback(() => {
    setLoading(true);
    getAdminOpenAPI()
      .then((nextDocument) => {
        setDocument(nextDocument);
        setError("");
      })
      .catch((nextError: unknown) => {
        setError(nextError instanceof Error ? nextError.message : dictionary.apiDocsLoadFailed);
      })
      .finally(() => setLoading(false));
  }, [dictionary.apiDocsLoadFailed]);

  useEffect(() => {
    loadDocument();
  }, [loadDocument]);

  const operations = useMemo(() => operationRows(document), [document]);
  const filteredOperations = useMemo(
    () => operations.filter((operation) => matchesOperationQuery(operation, query)).filter((operation) => matchesOperationFilters(operation, filters)),
    [filters, operations, query],
  );
  const schemaCount = Object.keys(document?.components?.schemas ?? {}).length;
  const tagCount = document?.tags?.length ?? 0;
  const securityCount = Object.keys(document?.components?.securitySchemes ?? {}).length;

  const columns: PlatformDataTableColumn<APIOperationRow>[] = [
    {
      title: dictionary.apiDocsMethod,
      key: "method",
      dataIndex: "method",
      width: 104,
      render: (method: string) => <Tag color={methodTones[method] ?? "default"}>{method}</Tag>,
      sorter: (left, right) => left.method.localeCompare(right.method),
    },
    {
      title: dictionary.apiDocsPath,
      key: "path",
      dataIndex: "path",
      width: 320,
      render: (path: string) => <Typography.Text code>{path}</Typography.Text>,
      sorter: (left, right) => left.path.localeCompare(right.path),
    },
    {
      title: dictionary.apiDocsOperation,
      key: "operationId",
      dataIndex: "operationId",
      width: 220,
      render: (operationId: string) => <PlatformOverflowText value={operationId || "-"} code />,
      sorter: (left, right) => left.operationId.localeCompare(right.operationId),
    },
    {
      title: dictionary.resource,
      key: "resource",
      dataIndex: "resource",
      width: 160,
      render: (resource: string) => <PlatformOverflowText value={resource || "-"} />,
      sorter: (left, right) => left.resource.localeCompare(right.resource),
    },
    {
      title: dictionary.permissionCodes,
      key: "permission",
      dataIndex: "permission",
      width: 220,
      render: (permission: string) => <PlatformOverflowText value={permission || "-"} code />,
      sorter: (left, right) => left.permission.localeCompare(right.permission),
    },
    {
      title: dictionary.description,
      key: "description",
      dataIndex: "description",
      render: (description: string) => <PlatformOverflowText value={description || "-"} />,
    },
  ];
  const filterFields: PlatformDataTableFilterField[] = [
    {
      key: "method",
      label: dictionary.apiDocsMethod,
      type: "select",
      placeholder: dictionary.filterKeyword,
      options: Object.keys(methodTones).map((method) => ({ value: method, label: method })),
    },
    { key: "resource", label: dictionary.resource, type: "text", placeholder: dictionary.filterKeyword },
    { key: "permission", label: dictionary.permissionCodes, type: "text", placeholder: dictionary.filterKeyword },
  ];

  const copyDocument = () => {
    if (!document) {
      return;
    }
    navigator.clipboard
      .writeText(JSON.stringify(document, null, 2))
      .then(() => message.success(dictionary.apiDocsCopied))
      .catch(() => message.error(dictionary.apiDocsCopyFailed));
  };

  const downloadDocument = () => {
    if (!document) {
      return;
    }
    const blob = new Blob([JSON.stringify(document, null, 2)], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const link = window.document.createElement("a");
    link.href = url;
    link.download = "openapi.admin.json";
    link.click();
    URL.revokeObjectURL(url);
  };

  return (
    <AdminPage
      className="api-docs-page"
      title={dictionary.apiDocsTitle}
      description={dictionary.apiDocsDescription}
      actions={
        <Space size={8}>
          <AdminActionButton icon={<ReloadOutlined />} label={dictionary.refresh} onClick={loadDocument} />
          <AdminActionButton disabled={!document} icon={<CopyOutlined />} label={dictionary.copy} onClick={copyDocument}>
            {dictionary.copy}
          </AdminActionButton>
          <AdminActionButton disabled={!document} icon={<DownloadOutlined />} label={dictionary.apiDocsDownloadJson} onClick={downloadDocument}>
            {dictionary.apiDocsDownloadJson}
          </AdminActionButton>
          <Button href="/api/openapi.json" icon={<LinkOutlined />} target="_blank">
            {dictionary.apiDocsOpenJson}
          </Button>
        </Space>
      }
      summary={
        <AdminMetricStrip
          columns={5}
          items={[
            { key: "openapi", label: dictionary.apiDocsSpecVersion, value: document?.openapi ?? "-", tone: "accent" },
            { key: "version", label: dictionary.apiDocsApiVersion, value: document?.info.version ?? "-" },
            { key: "paths", label: dictionary.apiDocsPathCount, value: operations.length },
            { key: "schemas", label: dictionary.apiDocsSchemaCount, value: schemaCount },
            { key: "tags", label: dictionary.apiDocsTagCount, value: tagCount },
          ]}
        />
      }
    >
      {error ? <AdminFeedback className="api-alert" type="warning" message={dictionary.apiDocsLoadFailed} description={error} /> : null}
      <section className="api-docs-source">
        <div>
          <ApiOutlined />
          <span>{dictionary.apiDocsSource}</span>
          <Typography.Text code>{document?.["x-source"] ?? "-"}</Typography.Text>
        </div>
        <div>
          <span>{dictionary.apiDocsGeneratedBy}</span>
          <Typography.Text code>{document?.["x-generated-by"] ?? "-"}</Typography.Text>
        </div>
        <div>
          <span>{dictionary.apiDocsSecurity}</span>
          <Typography.Text>{securityCount > 0 ? dictionary.apiDocsBearerAuth : "-"}</Typography.Text>
        </div>
      </section>
      <PlatformDataTable<APIOperationRow>
        className="api-docs-table"
        columns={columns}
        dataSource={filteredOperations}
        loading={loading}
        rowKey="key"
        searchValue={query}
        searchPlaceholder={dictionary.apiDocsSearch}
        labels={{
          search: dictionary.apiDocsSearch,
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
        title={dictionary.apiDocsOperationList}
        filterFields={filterFields}
        filterValues={filters}
        scrollX={1180}
        onSearchChange={setQuery}
        onFilterChange={(key, value) => setFilters((current) => ({ ...current, [key]: value }))}
        onClearFilters={() => setFilters({})}
        onRefresh={loadDocument}
      />
    </AdminPage>
  );
}

function operationRows(document: AdminOpenAPIDocument | null): APIOperationRow[] {
  if (!document) {
    return [];
  }
  return Object.entries(document.paths)
    .flatMap(([path, methods]) =>
      Object.entries(methods).map(([method, operation]) => operationRow(path, method, operation)),
    )
    .sort((left, right) => left.path.localeCompare(right.path) || left.method.localeCompare(right.method));
}

function operationRow(path: string, method: string, operation: AdminOpenAPIOperation): APIOperationRow {
  const normalizedMethod = method.toUpperCase();
  return {
    key: `${normalizedMethod} ${path}`,
    method: normalizedMethod,
    path,
    operationId: operation.operationId ?? "",
    permission: operation["x-platform-permission"] ?? "",
    resource: operation["x-platform-resource"] ?? operation.tags?.[0] ?? "",
    description: operation.summary ?? firstDescriptionLine(operation.description),
  };
}

function firstDescriptionLine(description?: string) {
  return description?.split("\n").find(Boolean) ?? "";
}

function matchesOperationQuery(row: APIOperationRow, query: string) {
  const parsed = parseOperationQuery(query);
  if (parsed.conditions.some((condition) => !matchesOperationCondition(row, condition))) {
    return false;
  }
  if (parsed.terms.length === 0) {
    return true;
  }
  const haystack = operationSearchValues(row).join(" ").toLowerCase();
  return parsed.terms.every((term) => haystack.includes(term.toLowerCase()));
}

function matchesOperationFilters(row: APIOperationRow, filters: Record<string, PlatformDataTableFilterValue>) {
  return Object.entries(filters).every(([key, value]) => {
    if (!filterValueActive(value)) {
      return true;
    }
    const actual = String(row[key as keyof APIOperationRow] ?? "");
    return typeof value === "string" ? actual.toLowerCase().includes(value.toLowerCase()) : true;
  });
}

type OperationQueryOperator = "=" | "!=" | ":" | "~" | ">=" | "<=" | ">" | "<";
type OperationQueryCondition = { field: keyof APIOperationRow; operator: OperationQueryOperator; value: string };

const operationQueryFields = new Set<keyof APIOperationRow>(["method", "path", "operationId", "permission", "resource", "description"]);

function parseOperationQuery(query: string) {
  const tokens = query.match(/"[^"]+"|'[^']+'|\S+/g) ?? [];
  const conditions: OperationQueryCondition[] = [];
  const terms: string[] = [];

  for (const token of tokens) {
    const normalized = stripQuotes(token.trim());
    const match = normalized.match(/^([a-zA-Z0-9_.-]+)(>=|<=|!=|>|<|=|:|~)(.+)$/);
    if (!match || !operationQueryFields.has(match[1] as keyof APIOperationRow)) {
      if (normalized) {
        terms.push(normalized);
      }
      continue;
    }
    conditions.push({
      field: match[1] as keyof APIOperationRow,
      operator: match[2] as OperationQueryOperator,
      value: stripQuotes(match[3].trim()),
    });
  }

  return { conditions, terms };
}

function matchesOperationCondition(row: APIOperationRow, condition: OperationQueryCondition) {
  const actual = String(row[condition.field] ?? "").toLowerCase();
  const expected = condition.value.toLowerCase();
  switch (condition.operator) {
  case "=":
    return actual === expected;
  case "!=":
    return actual !== expected;
  case ":":
  case "~":
    return actual.includes(expected);
  case ">=":
    return compareScalar(actual, expected) >= 0;
  case "<=":
    return compareScalar(actual, expected) <= 0;
  case ">":
    return compareScalar(actual, expected) > 0;
  case "<":
    return compareScalar(actual, expected) < 0;
  }
}

function operationSearchValues(row: APIOperationRow) {
  return [row.method, row.path, row.operationId, row.permission, row.resource, row.description];
}

function filterValueActive(value: PlatformDataTableFilterValue) {
  if (typeof value === "string") {
    return value.trim() !== "";
  }
  return Boolean(value.from || value.to);
}

function stripQuotes(value: string) {
  if ((value.startsWith("\"") && value.endsWith("\"")) || (value.startsWith("'") && value.endsWith("'"))) {
    return value.slice(1, -1);
  }
  return value;
}

function compareScalar(left: string, right: string) {
  const leftNumber = Number(left);
  const rightNumber = Number(right);
  if (Number.isFinite(leftNumber) && Number.isFinite(rightNumber)) {
    return leftNumber - rightNumber;
  }
  return left.localeCompare(right, undefined, { numeric: true });
}

function formatTemplate(template: string, values: Record<string, string>) {
  return Object.entries(values).reduce((result, [key, value]) => result.replaceAll(`{${key}}`, value), template);
}
