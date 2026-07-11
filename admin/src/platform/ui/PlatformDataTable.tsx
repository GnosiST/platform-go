import {
  AppstoreOutlined,
  ClearOutlined,
  ColumnHeightOutlined,
  FilterOutlined,
  QuestionCircleOutlined,
  ReloadOutlined,
  SearchOutlined,
} from "@ant-design/icons";
import {
  Button,
  Checkbox,
  Empty,
  Grid,
  Input,
  InputNumber,
  Select,
  Space,
  Spin,
  Table,
  Tag,
  Tooltip,
  Typography,
  type CheckboxProps,
  type TableColumnsType,
  type TablePaginationConfig,
  type TableProps,
} from "antd";
import { useEffect, useId, useMemo, useState, type Key, type ReactNode } from "react";
import { AdminActionButton, AdminListPanel, PlatformOverflowText } from "./AdminPrimitives";
import { PlatformDropdownPanel } from "./PlatformDropdownPanel";
import { PlatformDropdownPlugin, platformPopupContainer } from "./PlatformDropdownPlugin";
import { PlatformPaginationBar } from "./PlatformPaginationBar";
import { PlatformTreeSelect, type PlatformTreeSelectOption } from "./PlatformTreeSelect";

export type PlatformDataTableColumnPriority = "essential" | "standard" | "extended";

export type PlatformDataTableColumn<T extends object> = TableColumnsType<T>[number] & {
  key: Key;
  defaultHidden?: boolean;
  lockVisible?: boolean;
  priority?: PlatformDataTableColumnPriority;
};

type PlatformBreakpoint = NonNullable<PlatformDataTableColumn<Record<string, unknown>>["responsive"]>[number];
type PlatformBreakpointState = Partial<Record<PlatformBreakpoint, boolean>>;

export type PlatformDataTableFilterValue = string | { from?: string; to?: string };

export type PlatformDataTableFilterField = {
  key: string;
  label: ReactNode;
  type?: "text" | "select" | "treeSelect" | "dateRange" | "numberRange";
  placeholder?: string;
  options?: Array<{ value: string; label: ReactNode }> | PlatformTreeSelectOption[];
};

export type PlatformDataTableLabels = {
  search: string;
  refresh: string;
  columns: string;
  rowActions: string;
  selected: (count: number) => string;
  selectRow: (key: string) => string;
  clearSelection: string;
  empty: string;
  filters: string;
  clearFilters: string;
  querySyntax: string;
  querySyntaxHint: string;
  filterStartDate: string;
  filterEndDate: string;
  filterMin: string;
  filterMax: string;
  filterNoFields: string;
  activeFilters: (count: number) => string;
  pageSize: string;
  goToPage: string;
  page: string;
  paginationRange: string;
  selectedColumns: (selected: number, total: number) => string;
  renderedColumns: (rendered: number, selected: number) => string;
  hiddenAtCurrentWidth: string;
  selectAllColumns: string;
  resetColumns: string;
};

export type PlatformDataTableInlineEditorParams<T extends object> = {
  record: T;
  value: unknown;
  column: PlatformDataTableColumn<T>;
  index: number;
  defaultNode: ReactNode;
};

export type PlatformDataTableProps<T extends object> = {
  title?: ReactNode;
  columns: PlatformDataTableColumn<T>[];
  dataSource: T[];
  rowKey: string | ((record: T) => Key);
  loading?: boolean;
  selectedRowKey?: Key;
  searchValue?: string;
  searchPlaceholder?: string;
  labels: PlatformDataTableLabels;
  actions?: ReactNode;
  footer?: ReactNode;
  className?: string;
  pagination?: false | TablePaginationConfig;
  scrollX?: number;
  filterFields?: PlatformDataTableFilterField[];
  filterValues?: Record<string, PlatformDataTableFilterValue>;
  toolbarExtra?: ReactNode;
  batchActions?: (selectedKeys: Key[], clearSelection: () => void) => ReactNode;
  rowActions?: (record: T) => ReactNode;
  rowActionsColumnTitle?: ReactNode;
  rowActionsColumnWidth?: number;
  expandedRow?: (record: T) => ReactNode;
  inlineEditor?: (params: PlatformDataTableInlineEditorParams<T>) => ReactNode;
  detailDrawer?: ReactNode;
  emptyState?: ReactNode;
  mobileCards?: (items: T[]) => ReactNode;
  onSearchChange?: (value: string) => void;
  onFilterChange?: (key: string, value: PlatformDataTableFilterValue) => void;
  onClearFilters?: () => void;
  onRefresh?: () => void;
  onRowClick?: (record: T) => void;
  onTableChange?: TableProps<T>["onChange"];
};

export function PlatformDataTable<T extends object>({
  title,
  columns,
  dataSource,
  rowKey,
  loading = false,
  selectedRowKey,
  searchValue,
  searchPlaceholder,
  labels,
  actions,
  footer,
  className,
  pagination,
  scrollX = 920,
  filterFields = [],
  filterValues = {},
  toolbarExtra,
  batchActions,
  rowActions,
  rowActionsColumnTitle,
  rowActionsColumnWidth = 112,
  expandedRow,
  inlineEditor,
  detailDrawer,
  emptyState,
  mobileCards,
  onSearchChange,
  onFilterChange,
  onClearFilters,
  onRefresh,
  onRowClick,
  onTableChange,
}: PlatformDataTableProps<T>) {
  const screens = Grid.useBreakpoint();
  const tableID = useId().replace(/:/g, "");
  const columnsSignature = columns.map((column) => String(column.key)).join("|");
  const defaultVisibleColumnKeys = useMemo(() => columns.filter((column) => !column.defaultHidden).map((column) => column.key), [columnsSignature]);
  const initialPage = pagination !== false ? pagination?.current ?? pagination?.defaultCurrent ?? 1 : 1;
  const initialPageSize = pagination !== false ? pagination?.pageSize ?? pagination?.defaultPageSize ?? 10 : 10;
  const [visibleColumnKeys, setVisibleColumnKeys] = useState<Key[]>(() => defaultVisibleColumnKeys);
  const [selectedKeys, setSelectedKeys] = useState<Key[]>([]);
  const [internalPage, setInternalPage] = useState(initialPage);
  const [internalPageSize, setInternalPageSize] = useState(initialPageSize);
  const [openPlugin, setOpenPlugin] = useState<"filters" | "columns" | null>(null);
  const selectedColumns = useMemo(
    () => columns.filter((column) => column.lockVisible || visibleColumnKeys.includes(column.key)),
    [columns, visibleColumnKeys],
  );
  const renderedColumnKeys = useMemo(
    () => new Set(selectedColumns.filter((column) => columnRenderedAtCurrentWidth(column, screens)).map((column) => column.key)),
    [screens, selectedColumns],
  );
  const displayColumns = useMemo(
    () => selectedColumns.map((column) => withResponsivePriority(withDefaultOverflowRenderer(column, inlineEditor))),
    [inlineEditor, selectedColumns],
  );
  const tableColumns = useMemo(() => {
    if (!rowActions) {
      return displayColumns;
    }
    return [
      ...displayColumns,
      {
        title: rowActionsColumnTitle ?? labels.rowActions,
        key: "platform-row-actions",
        fixed: "right" as const,
        width: rowActionsColumnWidth,
        lockVisible: true,
        render: (_value: unknown, record: T) => <div className="platform-table-row-actions">{rowActions(record)}</div>,
      },
    ] satisfies PlatformDataTableColumn<T>[];
  }, [displayColumns, labels.rowActions, rowActions, rowActionsColumnTitle, rowActionsColumnWidth]);
  const activeFilterCount = Object.values(filterValues).filter(filterValueActive).length;
  const clearSelection = () => setSelectedKeys([]);

  const mergedPagination =
    pagination === false
      ? false
      : ({
          defaultPageSize: 10,
          pageSizeOptions: [10, 20, 50, 100],
          showQuickJumper: true,
          showLessItems: true,
          showSizeChanger: true,
          showTitle: false,
          responsive: true,
          showTotal: (total: number, range: [number, number]) =>
            formatTemplate(labels.paginationRange, {
              start: String(range[0]),
              end: String(range[1]),
              total: String(total),
            }),
          ...pagination,
        } satisfies TablePaginationConfig);
  const paginationTotal = mergedPagination ? Number(mergedPagination.total ?? dataSource.length) : dataSource.length;
  const currentPageSize = mergedPagination ? Number(mergedPagination.pageSize ?? internalPageSize) : internalPageSize;
  const maxPage = Math.max(1, Math.ceil(paginationTotal / Math.max(currentPageSize, 1)));
  const currentPage = mergedPagination ? clampPage(Number(mergedPagination.current ?? internalPage), maxPage) : internalPage;
  const usesServerPagination = Boolean(onTableChange);
  const pageDataSource = useMemo(() => {
    if (!mergedPagination || usesServerPagination) {
      return dataSource;
    }
    const start = (currentPage - 1) * currentPageSize;
    return dataSource.slice(start, start + currentPageSize);
  }, [currentPage, currentPageSize, dataSource, mergedPagination, usesServerPagination]);

  useEffect(() => {
    setVisibleColumnKeys(defaultVisibleColumnKeys);
  }, [columnsSignature, defaultVisibleColumnKeys]);

  useEffect(() => {
    if (!mergedPagination || usesServerPagination || mergedPagination.current) {
      return;
    }
    if (internalPage > maxPage) {
      setInternalPage(maxPage);
    }
  }, [internalPage, maxPage, mergedPagination, usesServerPagination]);

  const updatePage = (nextPage: number, nextPageSize: number) => {
    const safePage = clampPage(nextPage, Math.max(1, Math.ceil(paginationTotal / Math.max(nextPageSize, 1))));
    if (!mergedPagination) {
      return;
    }
    if (mergedPagination.current == null) {
      setInternalPage(safePage);
    }
    if (mergedPagination.pageSize == null) {
      setInternalPageSize(nextPageSize);
    }
    dispatchTableChange(
      onTableChange,
      {
        ...mergedPagination,
        current: safePage,
        pageSize: nextPageSize,
        total: paginationTotal,
      },
      dataSource,
    );
  };

  const handleTableChange: TableProps<T>["onChange"] = (nextPagination, tableFilters, sorter, extra) => {
    onTableChange?.(
      {
        ...(mergedPagination || {}),
        ...nextPagination,
        current: nextPagination.current ?? currentPage,
        pageSize: nextPagination.pageSize ?? currentPageSize,
        total: paginationTotal,
      },
      tableFilters,
      sorter,
      extra,
    );
  };

  const columnMenu = (
    <PlatformDropdownPanel
      className="platform-column-menu"
      title={labels.columns}
      headerExtra={
        <Space align="end" direction="vertical" size={0}>
          <Typography.Text type="secondary">{labels.selectedColumns(selectedColumns.length, columns.length)}</Typography.Text>
          <Typography.Text type="secondary">{labels.renderedColumns(renderedColumnKeys.size, selectedColumns.length)}</Typography.Text>
        </Space>
      }
      width={282}
      maxHeight="min(440px, calc(100vh - 140px))"
      bodyClassName="platform-column-menu-body"
      footer={
        <Space size={8}>
          <Button size="small" onClick={() => setVisibleColumnKeys(columns.map((column) => column.key))}>
            {labels.selectAllColumns}
          </Button>
          <Button size="small" onClick={() => setVisibleColumnKeys(defaultVisibleColumnKeys)}>
            {labels.resetColumns}
          </Button>
        </Space>
      }
    >
      <Checkbox.Group name={`${tableID}-column-visibility`} value={visibleColumnKeys} onChange={(values) => setVisibleColumnKeys(values)}>
        <Space className="platform-column-list" direction="vertical" size={4}>
          {columns.map((column) => {
            const hiddenAtCurrentWidth =
              (column.lockVisible || visibleColumnKeys.includes(column.key)) && !renderedColumnKeys.has(column.key);
            return (
              <Checkbox aria-label={String(column.title)} disabled={column.lockVisible} key={column.key} value={column.key}>
                <span className="platform-column-option">
                  <span className="platform-column-option-label">{column.title as ReactNode}</span>
                  {hiddenAtCurrentWidth ? (
                    <Typography.Text className="platform-column-option-state" type="secondary">
                      {labels.hiddenAtCurrentWidth}
                    </Typography.Text>
                  ) : null}
                </span>
              </Checkbox>
            );
          })}
        </Space>
      </Checkbox.Group>
    </PlatformDropdownPanel>
  );

  const filterMenu = (
    <PlatformDropdownPanel
      className="platform-filter-menu"
      title={labels.filters}
      headerExtra={activeFilterCount > 0 ? <Tag color="blue">{labels.activeFilters(activeFilterCount)}</Tag> : null}
      width="min(440px, calc(100vw - 32px))"
      maxHeight="min(560px, calc(100vh - 140px))"
      bodyClassName="platform-filter-menu-body"
      footer={
        <Button block disabled={activeFilterCount === 0} icon={<ClearOutlined />} size="small" onClick={onClearFilters}>
          {labels.clearFilters}
        </Button>
      }
    >
      {filterFields.length === 0 ? (
        <Typography.Text type="secondary">{labels.filterNoFields}</Typography.Text>
      ) : (
        <div className="platform-filter-grid">
          {filterFields.map((field) => (
            <FilterControl
              key={field.key}
              field={field}
              labels={labels}
              value={filterValues[field.key]}
              onChange={(value) => onFilterChange?.(field.key, value)}
            />
          ))}
        </div>
      )}
    </PlatformDropdownPanel>
  );

  const toolbarActions = (
    <>
      {onSearchChange ? (
        <Input
          aria-label={labels.search}
          autoComplete="off"
          className="capability-search platform-table-search"
          id={`${tableID}-search`}
          name="tableSearch"
          prefix={<SearchOutlined />}
          suffix={
            <Tooltip title={labels.querySyntaxHint}>
              <QuestionCircleOutlined aria-label={labels.querySyntax} />
            </Tooltip>
          }
          placeholder={searchPlaceholder ?? labels.search}
          value={searchValue}
          onChange={(event) => onSearchChange(event.target.value)}
        />
      ) : null}
      {filterFields.length > 0 ? (
        <PlatformDropdownPlugin
          open={openPlugin === "filters"}
          content={filterMenu}
          onOpenChange={(open) => setOpenPlugin(open ? "filters" : null)}
        >
          <Tooltip title={labels.filters}>
            <Button aria-label={labels.filters} icon={<FilterOutlined />}>
              {activeFilterCount > 0 ? activeFilterCount : null}
            </Button>
          </Tooltip>
        </PlatformDropdownPlugin>
      ) : null}
      {onRefresh ? <AdminActionButton icon={<ReloadOutlined />} label={labels.refresh} onClick={onRefresh} /> : null}
      <PlatformDropdownPlugin
        open={openPlugin === "columns"}
        content={columnMenu}
        onOpenChange={(open) => setOpenPlugin(open ? "columns" : null)}
      >
        <Tooltip title={labels.columns}>
          <Button aria-label={labels.columns} icon={<ColumnHeightOutlined />} />
        </Tooltip>
      </PlatformDropdownPlugin>
      {toolbarExtra}
      {actions}
    </>
  );

  const selectionBar = selectedKeys.length > 0 ? (
    <div className="platform-table-selection-bar">
      <Typography.Text>{labels.selected(selectedKeys.length)}</Typography.Text>
      <div className="platform-table-batch-actions">
        {batchActions?.(selectedKeys, clearSelection)}
        <Button icon={<ClearOutlined />} size="small" onClick={clearSelection}>
          {labels.clearSelection}
        </Button>
      </div>
    </div>
  ) : null;

  return (
    <AdminListPanel className={cx("platform-data-table-panel", className)} title={title} actions={toolbarActions} footer={footer}>
      {selectionBar}
      {loading ? (
        <div className="loading-panel">
          <Spin />
        </div>
      ) : dataSource.length === 0 ? (
        emptyState ?? <Empty className="platform-table-empty" description={labels.empty} image={Empty.PRESENTED_IMAGE_SIMPLE} />
      ) : (
        <>
          <Table<T>
            className="resource-table platform-data-table"
            columns={tableColumns}
            dataSource={pageDataSource}
            expandable={expandedRow ? { expandedRowRender: expandedRow } : undefined}
            pagination={false}
            rowKey={rowKey}
            rowSelection={
                  batchActions
                ? {
                    getTitleCheckboxProps: () =>
                      ({ "aria-label": labels.selectRow("all"), name: `${tableID}-row-selection-all` }) as Partial<
                        Omit<CheckboxProps, "checked" | "defaultChecked">
                      >,
                    preserveSelectedRowKeys: true,
                    selectedRowKeys: selectedKeys,
                    getCheckboxProps: (record) =>
                      ({ "aria-label": labels.selectRow(String(recordKey(record, rowKey))), name: `${tableID}-row-selection` }) as Partial<
                        Omit<CheckboxProps, "checked" | "defaultChecked">
                      >,
                    onChange: (keys) => setSelectedKeys(keys),
                  }
                : undefined
            }
            rowClassName={(record) => (recordKey(record, rowKey) === selectedRowKey ? "selected-row" : "")}
            scroll={{ x: scrollX }}
            showSorterTooltip
            size="small"
            sortDirections={["ascend", "descend"]}
            tableLayout="fixed"
            onChange={handleTableChange}
            onRow={(record) => ({
              onClick: () => onRowClick?.(record),
            })}
          />
          {detailDrawer}
          {mobileCards ? (
            <div className="resource-mobile-list platform-mobile-list">
              <div className="platform-mobile-list-label">
                <AppstoreOutlined />
              </div>
              {mobileCards(dataSource)}
            </div>
          ) : null}
          {mergedPagination ? (
            <PlatformPaginationBar
              config={mergedPagination}
              current={currentPage}
              disabled={loading}
              labels={{
                pageSize: labels.pageSize,
                goToPage: labels.goToPage,
                page: labels.page,
                paginationRange: labels.paginationRange,
              }}
              pageSize={currentPageSize}
              total={paginationTotal}
              onChange={updatePage}
            />
          ) : null}
        </>
      )}
    </AdminListPanel>
  );
}

function responsiveBreakpointsForPriority(priority: PlatformDataTableColumnPriority | undefined) {
  if (priority === "standard") return ["xl", "xxl"] as const;
  if (priority === "extended") return ["xxl"] as const;
  return ["md", "lg", "xl", "xxl"] as const;
}

function effectiveResponsiveBreakpoints<T extends object>(column: PlatformDataTableColumn<T>) {
  return [...(column.responsive ?? responsiveBreakpointsForPriority(column.priority))];
}

function withResponsivePriority<T extends object>(column: PlatformDataTableColumn<T>): PlatformDataTableColumn<T> {
  return { ...column, responsive: effectiveResponsiveBreakpoints(column) };
}

function columnRenderedAtCurrentWidth<T extends object>(column: PlatformDataTableColumn<T>, screens: PlatformBreakpointState) {
  return effectiveResponsiveBreakpoints(column).some((breakpoint) => screens[breakpoint]);
}

function FilterControl({
  field,
  labels,
  value,
  onChange,
}: {
  field: PlatformDataTableFilterField;
  labels: PlatformDataTableLabels;
  value?: PlatformDataTableFilterValue;
  onChange: (value: PlatformDataTableFilterValue) => void;
}) {
  const type = field.type ?? "text";
  const controlID = `platform-filter-${field.key.replace(/[^a-zA-Z0-9_-]/g, "-")}`;
  return (
    <label className="platform-filter-control">
      <Typography.Text>{field.label}</Typography.Text>
      {type === "select" ? (
        <Select
          allowClear
          getPopupContainer={platformPopupContainer}
          id={controlID}
          optionFilterProp="label"
          options={field.options ?? []}
          placeholder={field.placeholder}
          value={typeof value === "string" && value ? value : undefined}
          onChange={(nextValue) => onChange(nextValue ?? "")}
        />
      ) : type === "treeSelect" ? (
        <PlatformTreeSelect
          id={controlID}
          options={(field.options ?? []) as PlatformTreeSelectOption[]}
          placeholder={field.placeholder}
          value={typeof value === "string" && value ? value : undefined}
          onChange={(nextValue) => onChange(typeof nextValue === "string" ? nextValue : "")}
        />
      ) : type === "dateRange" ? (
        <div className="platform-date-range-control">
          <Input
            aria-label={`${field.label} ${labels.filterStartDate}`}
            autoComplete="off"
            id={`${controlID}-from`}
            name={`${field.key}From`}
            type="date"
            value={isRangeValue(value) ? value.from ?? "" : ""}
            onChange={(event) => onChange({ ...(isRangeValue(value) ? value : {}), from: event.target.value })}
          />
          <Input
            aria-label={`${field.label} ${labels.filterEndDate}`}
            autoComplete="off"
            id={`${controlID}-to`}
            name={`${field.key}To`}
            type="date"
            value={isRangeValue(value) ? value.to ?? "" : ""}
            onChange={(event) => onChange({ ...(isRangeValue(value) ? value : {}), to: event.target.value })}
          />
        </div>
      ) : type === "numberRange" ? (
        <div className="platform-date-range-control">
          <InputNumber
            aria-label={`${field.label} ${labels.filterMin}`}
            controls={false}
            id={`${controlID}-min`}
            name={`${field.key}Min`}
            placeholder={labels.filterMin}
            value={isRangeValue(value) && value.from ? Number(value.from) : null}
            onChange={(nextValue) => onChange({ ...(isRangeValue(value) ? value : {}), from: nextValue == null ? "" : String(nextValue) })}
          />
          <InputNumber
            aria-label={`${field.label} ${labels.filterMax}`}
            controls={false}
            id={`${controlID}-max`}
            name={`${field.key}Max`}
            placeholder={labels.filterMax}
            value={isRangeValue(value) && value.to ? Number(value.to) : null}
            onChange={(nextValue) => onChange({ ...(isRangeValue(value) ? value : {}), to: nextValue == null ? "" : String(nextValue) })}
          />
        </div>
      ) : (
        <Input
          allowClear
          autoComplete="off"
          id={controlID}
          name={field.key}
          placeholder={field.placeholder}
          value={typeof value === "string" ? value : ""}
          onChange={(event) => onChange(event.target.value)}
        />
      )}
    </label>
  );
}

function recordKey<T extends object>(record: T, rowKey: string | ((record: T) => Key)) {
  if (typeof rowKey === "function") {
    return rowKey(record);
  }
  return (record as Record<string, Key>)[rowKey];
}

function filterValueActive(value: PlatformDataTableFilterValue) {
  if (typeof value === "string") {
    return value.trim() !== "";
  }
  return Boolean(value.from || value.to);
}

function isRangeValue(value: PlatformDataTableFilterValue | undefined): value is { from?: string; to?: string } {
  return typeof value === "object" && value !== null;
}

function dispatchTableChange<T extends object>(
  onTableChange: TableProps<T>["onChange"] | undefined,
  pagination: TablePaginationConfig,
  dataSource: T[],
) {
  if (!onTableChange) {
    return;
  }
  type TableChangeHandler = NonNullable<TableProps<T>["onChange"]>;
  onTableChange(
    pagination,
    {},
    [] as Parameters<TableChangeHandler>[2],
    {
      currentDataSource: dataSource,
      action: "paginate",
    } as Parameters<TableChangeHandler>[3],
  );
}

function withDefaultOverflowRenderer<T extends object>(
  column: PlatformDataTableColumn<T>,
  inlineEditor?: (params: PlatformDataTableInlineEditorParams<T>) => ReactNode,
): PlatformDataTableColumn<T> {
  if (("children" in column && column.children) || ("render" in column && column.render)) {
    return column;
  }
  return {
    ...column,
    render: (value: unknown, record: T, index: number) => {
      const defaultNode = renderDefaultCell(value);
      return inlineEditor?.({ record, value, column, index, defaultNode }) ?? defaultNode;
    },
  } as PlatformDataTableColumn<T>;
}

function renderDefaultCell(value: unknown) {
  if (value == null || value === "") {
    return "-";
  }
  if (typeof value === "string" || typeof value === "number" || typeof value === "boolean") {
    return <PlatformOverflowText value={String(value)} />;
  }
  return value as ReactNode;
}

function clampPage(page: number, maxPage: number) {
  return Math.min(Math.max(Math.trunc(page), 1), maxPage);
}

function formatTemplate(template: string, values: Record<string, string>) {
  return Object.entries(values).reduce((result, [key, value]) => result.replaceAll(`{${key}}`, value), template);
}

function cx(...classes: Array<string | false | null | undefined>) {
  return classes.filter(Boolean).join(" ");
}
