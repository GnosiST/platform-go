import { Typography } from "antd";
import type { ReactNode } from "react";

type ResourcePageHeaderProps = {
  title: ReactNode;
  description?: ReactNode;
  eyebrow?: ReactNode;
  search?: ReactNode;
  actions?: ReactNode;
  extra?: ReactNode;
  className?: string;
};

export function ResourcePageHeader({ title, description, eyebrow, search, actions, extra, className }: ResourcePageHeaderProps) {
  return (
    <header className={cx("resource-page-header", className)}>
      <div className="resource-page-heading">
        {eyebrow ? <Typography.Text className="page-eyebrow">{eyebrow}</Typography.Text> : null}
        {typeof title === "string" ? <Typography.Title level={1}>{title}</Typography.Title> : title}
        {description ? <Typography.Paragraph>{description}</Typography.Paragraph> : null}
      </div>
      {extra ? <div className="resource-page-extra">{extra}</div> : null}
      {(search || actions) ? (
        <div className="resource-page-tools">
          {search ? <div className="resource-page-search">{search}</div> : null}
          {actions ? <div className="resource-page-actions">{actions}</div> : null}
        </div>
      ) : null}
    </header>
  );
}

function cx(...classes: Array<string | false | null | undefined>) {
  return classes.filter(Boolean).join(" ");
}
