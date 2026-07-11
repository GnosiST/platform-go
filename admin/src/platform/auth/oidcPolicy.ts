type AuthProviderWithAudiences = {
  id: string;
  audiences: readonly string[];
};

type AuthProviderStartResult = {
  authorizationUrl: string;
  state: string;
  expiresAt: string;
};

type OIDCExchangeInput = {
  provider: string;
  code: string;
  state: string;
  codeVerifier: string;
};

type OIDCLoginStartDependencies = {
  allowLoopbackHTTP: boolean;
  randomBytes: (size: number) => Uint8Array;
  digestSHA256: (input: Uint8Array) => Promise<Uint8Array>;
  startProvider: (provider: string, codeChallenge: string) => Promise<AuthProviderStartResult>;
  storePending: (serializedTransaction: string) => void;
  navigate: (authorizationURL: string) => void;
};

type OIDCCallbackDependencies<TResult> = {
  cleanupURL: () => void;
  readPending: () => string | null;
  removePending: () => void;
  now: () => number;
  exchange: (input: OIDCExchangeInput) => Promise<TResult>;
};

type PendingOIDCLogin = {
  provider: string;
  state: string;
  codeVerifier: string;
  expiresAt: string;
};

export type OIDCCallbackFailure = "callback" | "state" | "expired";

export class OIDCCallbackError extends Error {
  readonly reason: OIDCCallbackFailure;

  constructor(reason: OIDCCallbackFailure) {
    super(reason);
    this.name = "OIDCCallbackError";
    this.reason = reason;
  }
}

const untrustedAuthorizationURLError = "OIDC authorization URL is not trusted";

export function filterAdminAuthProviders<T extends AuthProviderWithAudiences>(providers: readonly T[]) {
  return providers.filter((provider) => provider.audiences.includes("admin"));
}

export function assertAdminAuthProvider(provider: AuthProviderWithAudiences) {
  if (!provider.audiences.includes("admin")) {
    throw new Error("OIDC provider is not available for Admin login");
  }
}

export function createSubmissionLock() {
  let locked = false;
  return {
    acquire() {
      if (locked) return false;
      locked = true;
      return true;
    },
    release() {
      locked = false;
    },
  };
}

export function createSingleUseGuard() {
  let used = false;
  return {
    acquire() {
      if (used) return false;
      used = true;
      return true;
    },
  };
}

export function validateOIDCAuthorizationURL(rawURL: string, options: { allowLoopbackHTTP?: boolean } = {}) {
  try {
    const authorizationURL = new URL(rawURL);
    if (authorizationURL.protocol === "https:") {
      return authorizationURL.toString();
    }
    if (options.allowLoopbackHTTP && authorizationURL.protocol === "http:" && isLoopbackHostname(authorizationURL.hostname)) {
      return authorizationURL.toString();
    }
  } catch {
    // Normalize malformed and untrusted values to one sanitized browser-boundary error.
  }
  throw new Error(untrustedAuthorizationURLError);
}

export async function beginOIDCLoginTransaction(provider: AuthProviderWithAudiences, dependencies: OIDCLoginStartDependencies) {
  assertAdminAuthProvider(provider);
  const verifierBytes = dependencies.randomBytes(32);
  const codeVerifier = base64URL(verifierBytes);
  const digest = await dependencies.digestSHA256(new TextEncoder().encode(codeVerifier));
  const codeChallenge = base64URL(digest);
  const started = await dependencies.startProvider(provider.id, codeChallenge);
  const authorizationURL = validateOIDCAuthorizationURL(started.authorizationUrl, {
    allowLoopbackHTTP: dependencies.allowLoopbackHTTP,
  });
  dependencies.storePending(
    JSON.stringify({
      provider: provider.id,
      state: started.state,
      codeVerifier,
      expiresAt: started.expiresAt,
    }),
  );
  dependencies.navigate(authorizationURL);
}

export function hasOIDCCallbackParams(search: string) {
  const params = new URLSearchParams(search);
  return ["code", "state", "error"].some((key) => params.has(key));
}

export async function consumePendingOIDCLoginTransaction<TResult>(search: string, dependencies: OIDCCallbackDependencies<TResult>): Promise<TResult | null> {
  const params = new URLSearchParams(search);
  const code = params.get("code");
  const callbackState = params.get("state");
  const providerError = params.get("error");
  if (!code && !callbackState && !providerError) {
    return null;
  }

  dependencies.cleanupURL();
  const rawPending = dependencies.readPending();
  dependencies.removePending();
  if (providerError || !code || !callbackState) {
    throw new OIDCCallbackError("callback");
  }

  const pending = parsePendingOIDCLogin(rawPending);
  if (!pending || callbackState !== pending.state) {
    throw new OIDCCallbackError("state");
  }
  if (!Number.isFinite(Date.parse(pending.expiresAt)) || Date.parse(pending.expiresAt) <= dependencies.now()) {
    throw new OIDCCallbackError("expired");
  }

  return dependencies.exchange({
    provider: pending.provider,
    code,
    state: callbackState,
    codeVerifier: pending.codeVerifier,
  });
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
  return globalThis.btoa(binary).replaceAll("+", "-").replaceAll("/", "_").replace(/=+$/u, "");
}

function isLoopbackHostname(hostname: string) {
  const normalizedHostname = hostname.toLowerCase();
  if (
    normalizedHostname === "localhost" ||
    normalizedHostname.endsWith(".localhost") ||
    normalizedHostname === "[::1]" ||
    normalizedHostname === "::1"
  ) {
    return true;
  }
  return /^127(?:\.\d{1,3}){3}$/u.test(normalizedHostname);
}
