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
import { assertAdminAuthProvider, validateOIDCAuthorizationURL } from "../auth/oidcPolicy";

const OIDC_TRANSACTION_KEY = "platform.auth.oidc.pending";

type PendingOIDCLogin = {
  provider: string;
  state: string;
  codeVerifier: string;
  expiresAt: string;
};

export type OIDCCallbackFailure = "callback" | "state" | "expired";

export class OIDCCallbackError extends Error {
  constructor(readonly reason: OIDCCallbackFailure) {
    super(reason);
    this.name = "OIDCCallbackError";
  }
}

export async function beginOIDCLogin(provider: AuthProvider) {
  assertAdminAuthProvider(provider);
  const verifierBytes = crypto.getRandomValues(new Uint8Array(32));
  const codeVerifier = base64URL(verifierBytes);
  const digest = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(codeVerifier));
  const codeChallenge = base64URL(new Uint8Array(digest));
  const started = await startAdminAuthProvider(provider.id, codeChallenge);
  const authorizationURL = validateOIDCAuthorizationURL(started.authorizationUrl);
  window.sessionStorage.setItem(
    OIDC_TRANSACTION_KEY,
    JSON.stringify({
      provider: provider.id,
      state: started.state,
      codeVerifier,
      expiresAt: started.expiresAt,
    }),
  );
  window.location.assign(authorizationURL);
}

export async function consumePendingOIDCLogin(search: string): Promise<AuthLoginResult | null> {
  const params = new URLSearchParams(search);
  const code = params.get("code");
  const callbackState = params.get("state");
  const providerError = params.get("error");
  if (!code && !callbackState && !providerError) {
    return null;
  }

  window.history.replaceState(window.history.state, "", "/login");
  window.dispatchEvent(new PopStateEvent("popstate"));

  const rawPending = window.sessionStorage.getItem(OIDC_TRANSACTION_KEY);
  window.sessionStorage.removeItem(OIDC_TRANSACTION_KEY);
  if (providerError || !code || !callbackState) {
    throw new OIDCCallbackError("callback");
  }

  const pending = parsePendingOIDCLogin(rawPending);
  if (!pending || callbackState !== pending.state) {
    throw new OIDCCallbackError("state");
  }
  if (!Number.isFinite(Date.parse(pending.expiresAt)) || Date.parse(pending.expiresAt) <= Date.now()) {
    throw new OIDCCallbackError("expired");
  }

  return loginWithAuthProvider({
    provider: pending.provider,
    code,
    state: callbackState,
    codeVerifier: pending.codeVerifier,
  });
}

export function clearPendingOIDCLogin() {
  window.sessionStorage.removeItem(OIDC_TRANSACTION_KEY);
}

function parsePendingOIDCLogin(rawPending: string | null): PendingOIDCLogin | null {
  if (!rawPending) return null;
  try {
    const value = JSON.parse(rawPending) as Partial<PendingOIDCLogin>;
    const keys = Object.keys(value);
    if (
      keys.length !== 4 ||
      !keys.every((key) => ["provider", "state", "codeVerifier", "expiresAt"].includes(key)) ||
      typeof value.provider !== "string" ||
      typeof value.state !== "string" ||
      typeof value.codeVerifier !== "string" ||
      typeof value.expiresAt !== "string"
    ) {
      return null;
    }
    return value as PendingOIDCLogin;
  } catch {
    return null;
  }
}

function base64URL(bytes: Uint8Array) {
  let binary = "";
  for (const byte of bytes) binary += String.fromCharCode(byte);
  return window.btoa(binary).replaceAll("+", "-").replaceAll("/", "_").replace(/=+$/u, "");
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
