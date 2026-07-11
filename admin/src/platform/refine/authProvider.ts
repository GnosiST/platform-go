import type { AuthProvider } from "@refinedev/core";
import {
  AdminAPIError,
  clearAuthToken,
  getAuthToken,
  getCurrentAdminSession,
  loginWithAuthProvider,
  logoutCurrentSession,
} from "../api/client";

export const authProvider: AuthProvider = {
  login: async ({ provider = "demo", username = "admin", code }: { provider?: string; username?: string; code?: string }) => {
    try {
      await loginWithAuthProvider({ provider, username, code });
      window.dispatchEvent(new Event("platform:auth:login"));
      return { success: true, redirectTo: "/" };
    } catch (error) {
      return {
        success: false,
        error: {
          name: "Login failed",
          message: error instanceof Error ? error.message : "Unable to sign in",
        },
      };
    }
  },
  logout: async () => {
    await logoutCurrentSession();
    return { success: true, redirectTo: "/login" };
  },
  check: async () => {
    if (!getAuthToken()) {
      return { authenticated: false, redirectTo: "/login" };
    }
    try {
      await getCurrentAdminSession();
      return { authenticated: true };
    } catch {
      clearAuthToken();
      return { authenticated: false, redirectTo: "/login" };
    }
  },
  getPermissions: async () => {
    if (!getAuthToken()) {
      return [];
    }
    const session = await getCurrentAdminSession();
    return session.permissions;
  },
  getIdentity: async () => {
    if (!getAuthToken()) {
      return null;
    }
    const session = await getCurrentAdminSession();
    return {
      id: session.user.id,
      name: session.user.name || session.user.username,
      username: session.user.username,
      roles: session.roles,
    };
  },
  onError: async (error) => {
    if (error instanceof AdminAPIError && error.statusCode === 401) {
      return { logout: true, redirectTo: "/login" };
    }
    return {};
  },
};
