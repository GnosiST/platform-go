import { InputNumber, Pagination, Select, Typography, type TablePaginationConfig } from "antd";
import { useEffect, useMemo, useState, type ReactNode } from "react";
import { platformPopupContainer } from "./PlatformDropdownPlugin";

export type PlatformPaginationLabels = {
  pageSize: string;
  goToPage: string;
  page: string;
  paginationRange: string;
};

export type PlatformPaginationBarProps = {
  current: number;
  pageSize: number;
  total: number;
  labels: PlatformPaginationLabels;
  config?: TablePaginationConfig;
  disabled?: boolean;
  onChange: (page: number, pageSize: number) => void;
};

export function PlatformPaginationBar({
  current,
  pageSize,
  total,
  labels,
  config,
  disabled,
  onChange,
}: PlatformPaginationBarProps) {
  const pageSizeOptions = useMemo(() => normalizePageSizeOptions(config?.pageSizeOptions), [config?.pageSizeOptions]);
  const maxPage = Math.max(1, Math.ceil(total / Math.max(pageSize, 1)));
  const safeCurrent = clampPage(current, maxPage);
  const [jumpPage, setJumpPage] = useState<number | null>(safeCurrent);
  const showSizeChanger = config?.showSizeChanger !== false;
  const showQuickJumper = config?.showQuickJumper !== false;
  const controlDisabled = disabled || config?.disabled;
  const showTotal = config?.showTotal;
  const rangeStart = total === 0 ? 0 : (safeCurrent - 1) * pageSize + 1;
  const rangeEnd = Math.min(total, safeCurrent * pageSize);
  const totalText =
    showTotal?.(total, [rangeStart, rangeEnd]) ??
    formatTemplate(labels.paginationRange, {
      start: String(rangeStart),
      end: String(rangeEnd),
      total: String(total),
    });

  useEffect(() => {
    setJumpPage(safeCurrent);
  }, [safeCurrent]);

  if (config?.hideOnSinglePage && total <= pageSize) {
    return null;
  }

  const applyJump = () => {
    if (!jumpPage) {
      setJumpPage(safeCurrent);
      return;
    }
    onChange(clampPage(jumpPage, maxPage), pageSize);
  };

  return (
    <div className="platform-pagination-bar">
      <div className="platform-pagination-meta">
        <Typography.Text className="platform-pagination-total">{totalText as ReactNode}</Typography.Text>
        {showSizeChanger ? (
          <Select
            aria-label={labels.pageSize}
            className="platform-pagination-size"
            disabled={controlDisabled}
            getPopupContainer={platformPopupContainer}
            options={pageSizeOptions.map((size) => ({
              value: size,
              label: `${size}${labels.pageSize}`,
            }))}
            size="small"
            value={pageSize}
            onChange={(nextSize) => onChange(1, nextSize)}
          />
        ) : null}
      </div>
      <div className="platform-pagination-main">
        <Pagination
          current={safeCurrent}
          disabled={controlDisabled}
          pageSize={pageSize}
          responsive
          showLessItems
          showQuickJumper={false}
          showSizeChanger={false}
          showTitle={false}
          size="small"
          total={total}
          onChange={(nextPage) => onChange(nextPage, pageSize)}
        />
      </div>
      {showQuickJumper ? (
        <div className="platform-pagination-jumper">
          <Typography.Text>{labels.goToPage}</Typography.Text>
          <InputNumber
            aria-label={labels.goToPage}
            controls={false}
            disabled={controlDisabled}
            max={maxPage}
            min={1}
            name="paginationJumpPage"
            size="small"
            value={jumpPage}
            onChange={(value) => setJumpPage(typeof value === "number" ? value : null)}
            onBlur={applyJump}
            onKeyDown={(event) => {
              if (event.key === "Enter") {
                applyJump();
              }
            }}
          />
          <Typography.Text>{labels.page}</Typography.Text>
        </div>
      ) : null}
    </div>
  );
}

function normalizePageSizeOptions(options?: TablePaginationConfig["pageSizeOptions"]) {
  const rawOptions = options && options.length > 0 ? options : [10, 20, 50, 100];
  return rawOptions.map((value) => Number(value)).filter((value) => Number.isFinite(value) && value > 0);
}

function clampPage(page: number, maxPage: number) {
  return Math.min(Math.max(Math.trunc(page), 1), maxPage);
}

function formatTemplate(template: string, values: Record<string, string>) {
  return Object.entries(values).reduce((result, [key, value]) => result.replaceAll(`{${key}}`, value), template);
}
