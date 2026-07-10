import { Typography } from "antd";
import type { CSSProperties, ReactNode } from "react";

export type PlatformDropdownPanelProps = {
  title?: ReactNode;
  description?: ReactNode;
  headerExtra?: ReactNode;
  footer?: ReactNode;
  children: ReactNode;
  className?: string;
  bodyClassName?: string;
  footerClassName?: string;
  width?: CSSProperties["width"];
  maxHeight?: CSSProperties["maxHeight"];
};

export function PlatformDropdownPanel({
  title,
  description,
  headerExtra,
  footer,
  children,
  className,
  bodyClassName,
  footerClassName,
  width,
  maxHeight,
}: PlatformDropdownPanelProps) {
  const style = {
    ...(width ? { width } : {}),
    ...(maxHeight ? { maxHeight } : {}),
  } satisfies CSSProperties;

  return (
    <div className={cx("platform-dropdown-panel", className)} style={style}>
      {title || description || headerExtra ? (
        <div className="platform-dropdown-panel-header">
          {title || headerExtra ? (
            <div className="platform-dropdown-panel-title">
              {title ? <Typography.Text strong>{title}</Typography.Text> : null}
              {headerExtra}
            </div>
          ) : null}
          {description ? <Typography.Text type="secondary">{description}</Typography.Text> : null}
        </div>
      ) : null}
      <div className={cx("platform-dropdown-panel-body", bodyClassName)}>{children}</div>
      {footer ? <div className={cx("platform-dropdown-panel-footer", footerClassName)}>{footer}</div> : null}
    </div>
  );
}

function cx(...classes: Array<string | false | null | undefined>) {
  return classes.filter(Boolean).join(" ");
}
