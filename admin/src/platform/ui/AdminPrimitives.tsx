import { Alert, Button, Modal, Tooltip, Typography, type AlertProps, type ButtonProps, type ModalProps } from "antd";
import { forwardRef, type ComponentRef, type ReactNode } from "react";

type AdminListPanelProps = {
  title?: ReactNode;
  toolbar?: ReactNode;
  actions?: ReactNode;
  footer?: ReactNode;
  children: ReactNode;
  className?: string;
  bodyClassName?: string;
};

type AdminActionButtonProps = ButtonProps & {
  label: string;
  tooltip?: ReactNode;
};

type AdminFeedbackProps = AlertProps & {
  compact?: boolean;
};

export type AdminModalSize = "sm" | "md" | "lg" | "xl";

export type AdminModalPreset = "form" | "detail" | "confirm";

type AdminModalProps = ModalProps & {
  size?: AdminModalSize;
  preset?: AdminModalPreset;
};

type AdminFormModalProps = AdminModalProps & {
  children: ReactNode;
};

type PlatformOverflowTextProps = {
  value: ReactNode;
  tooltip?: ReactNode;
  className?: string;
  strong?: boolean;
  code?: boolean;
};

const ADMIN_MODAL_SIZE_WIDTH: Record<AdminModalSize, number> = {
  sm: 420,
  md: 560,
  lg: 760,
  xl: 960,
};

const ADMIN_MODAL_PRESET_SIZE: Record<AdminModalPreset, AdminModalSize> = {
  form: "md",
  detail: "lg",
  confirm: "sm",
};

export function AdminListPanel({
  title,
  toolbar,
  actions,
  footer,
  children,
  className,
  bodyClassName,
}: AdminListPanelProps) {
  return (
    <section className={cx("admin-list-panel", className)}>
      {(title || toolbar || actions) && (
        <div className="table-toolbar admin-list-toolbar">
          {title ? (
            <Typography.Text strong className="admin-list-title">
              {title}
            </Typography.Text>
          ) : null}
          {toolbar ? <div className="admin-list-toolbar-main">{toolbar}</div> : null}
          {actions ? <div className="table-actions admin-list-actions">{actions}</div> : null}
        </div>
      )}
      <div className={cx("admin-list-body", bodyClassName)}>{children}</div>
      {footer ? <div className="admin-list-footer">{footer}</div> : null}
    </section>
  );
}

export const AdminActionButton = forwardRef<ComponentRef<typeof Button>, AdminActionButtonProps>(function AdminActionButton(
  { label, tooltip, children, ...buttonProps },
  ref,
) {
  const button = (
    <Button ref={ref} aria-label={label} {...buttonProps}>
      {children}
    </Button>
  );

  return tooltip ? <Tooltip title={tooltip}>{button}</Tooltip> : button;
});

export function AdminFeedback({ className, compact = true, ...alertProps }: AdminFeedbackProps) {
  return <Alert className={cx("admin-feedback", compact && "compact", className)} showIcon {...alertProps} />;
}

export function AdminModal({ className, preset, size, width, ...modalProps }: AdminModalProps) {
  const resolvedSize = size ?? (preset ? ADMIN_MODAL_PRESET_SIZE[preset] : "md");
  const resolvedWidth = width ?? ADMIN_MODAL_SIZE_WIDTH[resolvedSize];

  return (
    <Modal
      className={cx("admin-modal", `admin-modal-size-${resolvedSize}`, preset && `admin-modal-preset-${preset}`, className)}
      width={resolvedWidth}
      {...modalProps}
    />
  );
}

export function AdminFormModal({ className, preset = "form", destroyOnHidden = true, forceRender = true, ...modalProps }: AdminFormModalProps) {
  return (
    <AdminModal
      className={cx("admin-form-modal", className)}
      destroyOnHidden={destroyOnHidden}
      forceRender={forceRender}
      preset={preset}
      {...modalProps}
    />
  );
}

export function PlatformOverflowText({ value, tooltip = value, className, strong, code }: PlatformOverflowTextProps) {
  if (!value || value === "-") {
    return value;
  }
  const content =
    strong || code ? (
      <Typography.Text className={cx("table-cell-ellipsis", className)} code={code} strong={strong}>
        {value}
      </Typography.Text>
    ) : (
      <span className={cx("table-cell-ellipsis", className)}>{value}</span>
    );
  return (
    <Tooltip title={tooltip}>
      {content}
    </Tooltip>
  );
}

function cx(...classes: Array<string | false | null | undefined>) {
  return classes.filter(Boolean).join(" ");
}
