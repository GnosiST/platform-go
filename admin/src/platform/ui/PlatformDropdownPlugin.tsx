import { Dropdown, type DropdownProps } from "antd";
import type { ReactElement, ReactNode } from "react";

type PlatformDropdownPluginProps = {
  open?: boolean;
  content: ReactNode;
  children: ReactElement;
  trigger?: DropdownProps["trigger"];
  overlayClassName?: string;
  placement?: DropdownProps["placement"];
  onOpenChange?: DropdownProps["onOpenChange"];
};

export function PlatformDropdownPlugin({
  open,
  content,
  children,
  trigger = ["click"],
  overlayClassName,
  placement,
  onOpenChange,
}: PlatformDropdownPluginProps) {
  return (
    <Dropdown
      getPopupContainer={platformPopupContainer}
      open={open}
      overlayClassName={cx("platform-dropdown-overlay", overlayClassName)}
      placement={placement}
      popupRender={() => content}
      trigger={trigger}
      onOpenChange={onOpenChange}
    >
      {children}
    </Dropdown>
  );
}

export function platformPopupContainer(triggerNode: HTMLElement) {
  const shell = triggerNode.closest(".platform-shell");
  return shell instanceof HTMLElement ? shell : document.body;
}

function cx(...classes: Array<string | false | null | undefined>) {
  return classes.filter(Boolean).join(" ");
}
