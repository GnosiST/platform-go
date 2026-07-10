import type { AccessControlProvider } from "@refinedev/core";
import { getAuthToken, getCurrentAdminSession } from "../api/client";
import { hasPermission, permissionForRefineAction } from "./permissions";

export const accessControlProvider: AccessControlProvider = {
  can: async (params) => {
    if (!getAuthToken()) {
      return { can: false, reason: "Unauthenticated" };
    }
    const permission = permissionForRefineAction(params);
    if (!permission) {
      return { can: true };
    }
    const session = await getCurrentAdminSession();
    return {
      can: hasPermission(session.permissions, permission, session.deniedPermissions ?? []),
      reason: permission,
    };
  },
  options: {
    buttons: {
      enableAccessControl: true,
      hideIfUnauthorized: true,
    },
  },
};
