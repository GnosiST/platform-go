type FocusTarget = Pick<HTMLElement, "focus" | "isConnected">;

export type RoleMenuReadOnlyReason = "legacy" | "access" | "disabled" | null;

export function restoreRoleModalFocus(trigger: FocusTarget | null | undefined, detail: FocusTarget | null | undefined) {
  const target = trigger?.isConnected ? trigger : detail?.isConnected ? detail : null;
  if (!target) return "none" as const;
  target.focus({ preventScroll: true });
  return target === trigger ? "trigger" as const : "detail" as const;
}

export function resolveRoleMenuAccess(writeEnabled: boolean, canAssignMenus: boolean, roleStatus: string) {
  let readOnlyReason: RoleMenuReadOnlyReason = null;
  if (roleStatus !== "enabled") readOnlyReason = "disabled";
  else if (!canAssignMenus) readOnlyReason = "access";
  else if (!writeEnabled) readOnlyReason = "legacy";

  const editable = readOnlyReason === null;
  return {
    editable,
    readOnly: !editable,
    readOnlyReason,
    showSave: editable,
  };
}
