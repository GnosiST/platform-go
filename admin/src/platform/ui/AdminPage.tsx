import { Typography } from "antd";
import type { CSSProperties, ReactNode } from "react";
import { ResourcePageHeader } from "./ResourcePageHeader";

type AdminPageDensity = "compact" | "comfortable";

type AdminPageProps = {
  title: ReactNode;
  description?: ReactNode;
  actions?: ReactNode;
  summary?: ReactNode;
  eyebrow?: ReactNode;
  extra?: ReactNode;
  children: ReactNode;
  className?: string;
  contentClassName?: string;
  density?: AdminPageDensity;
};

type AdminMetricTone = "default" | "accent" | "warning" | "danger";

export type AdminMetricItem = {
  key: string;
  label: ReactNode;
  value: ReactNode;
  tone?: AdminMetricTone;
  extra?: ReactNode;
  className?: string;
};

type AdminMetricStripProps = {
  items: AdminMetricItem[];
  className?: string;
  columns?: number | string;
};

export function AdminPage({
  title,
  description,
  actions,
  summary,
  eyebrow,
  extra,
  children,
  className,
  contentClassName,
  density = "compact",
}: AdminPageProps) {
  return (
    <section className={cx("admin-page", `density-${density}`, className)}>
      <ResourcePageHeader className="page-heading" title={title} description={description} eyebrow={eyebrow} actions={actions} extra={extra} />
      {summary ? <div className="admin-page-summary">{summary}</div> : null}
      <div className={cx("admin-page-content", contentClassName)}>{children}</div>
    </section>
  );
}

export function AdminMetricStrip({ items, className, columns }: AdminMetricStripProps) {
  const style = columns
    ? ({
        "--metric-columns": typeof columns === "number" ? `repeat(${columns}, minmax(0, 1fr))` : columns,
      } as CSSProperties)
    : undefined;

  return (
    <div className={cx("admin-metric-strip", className)} style={style}>
      {items.map((item) => (
        <div className={cx("metric", item.tone && `tone-${item.tone}`, item.className)} key={item.key}>
          <Typography.Text>{item.label}</Typography.Text>
          <strong className="metric-value">{item.value}</strong>
          {item.extra ? <div className="metric-extra">{item.extra}</div> : null}
        </div>
      ))}
    </div>
  );
}

function cx(...classes: Array<string | false | null | undefined>) {
  return classes.filter(Boolean).join(" ");
}
