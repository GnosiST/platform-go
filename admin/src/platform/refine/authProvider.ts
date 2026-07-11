import type { AuthProvider as RefineAuthProvider } from "@refinedev/core";
import {
  AdminAPIError,
  clearAuthToken,
  getAuthToken,
  getCurrentAdminSession,
  loginWithAuthProvider,
  logoutCurrentSession,
  startAdminAuthProvider,
  type AuthProvider,
  type AuthLoginResult,
} from "../api/client";
import {
  beginOIDCLoginTransaction,
  consumePendingOIDCLoginTransaction,
} from "../auth/oidcPolicy";

const OIDC_TRANSACTION_KEY = "platform.auth.oidc.pending";

export async function beginOIDCLogin(provider: AuthProvider) {
  return beginOIDCLoginTransaction(provider, {
    allowLoopbackHTTP: import.meta.env.DEV,
    randomBytes: (size) => crypto.getRandomValues(new Uint8Array(size)),
    digestSHA256: async (input) => new Uint8Array(await crypto.subtle.digest("SHA-256", Uint8Array.from(input).buffer)),
    startProvider: startAdminAuthProvider,
    storePending: (serializedTransaction) => window.sessionStorage.setItem(OIDC_TRANSACTION_KEY, serializedTransaction),
    navigate: (authorizationURL) => window.location.assign(authorizationURL),
  });
}

export async function consumePendingOIDCLogin(search: string): Promise<AuthLoginResult | null> {
  return consumePendingOIDCLoginTransaction(search, {
    cleanupURL: () => {
      window.history.replaceState(window.history.state, "", "/login");
      window.dispatchEvent(new PopStateEvent("popstate"));
    },
    readPending: () => window.sessionStorage.getItem(OIDC_TRANSACTION_KEY),
    removePending: () => window.sessionStorage.removeItem(OIDC_TRANSACTION_KEY),
    now: Date.now,
    exchange: loginWithAuthProvider,
  });
}

export function clearPendingOIDCLogin() {
  window.sessionStorage.removeItem(OIDC_TRANSACTION_KEY);
}

export const authProvider: RefineAuthProvider = {
  login: async ({ provider = "demo", username = "admin", code, state, codeVerifier }: { provider?: string; username?: string; code?: string; state?: string; codeVerifier?: string }) => {
    try {
      await loginWithAuthProvider({ provider, username, code, state, codeVerifier });
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
