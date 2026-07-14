export const SENSITIVE_REVEAL_OIDC_PENDING_KEY = "platform.security.sensitive-reveal.oidc.pending.v1";

export type SensitiveRevealFactorType = "oidc-reauth-v1" | "admin-sms-otp-v1";

export type SensitiveRevealOIDCContext = {
  challengeId: string;
  challengeToken: string;
  challengeExpiresAt: string;
  resource: string;
  recordId: string;
  field: string;
  purpose: string;
  returnPath: string;
  completedFactors: SensitiveRevealFactorType[];
};

export type SensitiveRevealOIDCBeginInput = SensitiveRevealOIDCContext & {
  provider: string;
};

export type SensitiveRevealOIDCStartInput = {
  challengeId: string;
  challengeToken: string;
  resource: string;
  recordId: string;
  field: string;
  purpose: string;
  provider: string;
  codeChallenge: string;
};

export type SensitiveRevealOIDCStartResult = {
  challengeId: string;
  transactionToken: string;
  authorizationUrl: string;
  state: string;
  expiresAt: string;
};

export type SensitiveRevealOIDCCompleteInput = {
  challengeId: string;
  challengeToken: string;
  resource: string;
  recordId: string;
  field: string;
  purpose: string;
  transactionToken: string;
  provider: string;
  code: string;
  state: string;
  codeVerifier: string;
};

export type SensitiveRevealOIDCCompletion<TResult> = {
  context: SensitiveRevealOIDCContext;
  completion: TResult;
};

export type SensitiveRevealOIDCResume<TResult> = {
  context: SensitiveRevealOIDCContext;
  completion?: TResult;
  error?: SensitiveRevealOIDCCallbackFailure;
};

type SensitiveRevealOIDCStorage = Pick<Storage, "getItem" | "setItem" | "removeItem">;

export type SensitiveRevealOIDCBeginOptions = {
  allowLoopbackHTTP?: boolean;
  storage?: SensitiveRevealOIDCStorage;
  crypto?: Pick<Crypto, "getRandomValues" | "subtle">;
  navigate?: (authorizationURL: string) => void;
  now?: () => number;
};

export type SensitiveRevealOIDCConsumeOptions = {
  storage?: SensitiveRevealOIDCStorage;
  cleanupURL?: () => void;
  now?: () => number;
};

type PendingSensitiveRevealOIDC = SensitiveRevealOIDCContext & {
  transactionToken: string;
  provider: string;
  state: string;
  codeVerifier: string;
  expiresAt: string;
};

export type SensitiveRevealOIDCCallbackFailure = "callback" | "transaction" | "state" | "expired" | "exchange";

export class SensitiveRevealOIDCCallbackError extends Error {
  readonly reason: SensitiveRevealOIDCCallbackFailure;
  readonly context?: SensitiveRevealOIDCContext;

  constructor(reason: SensitiveRevealOIDCCallbackFailure, context?: SensitiveRevealOIDCContext) {
    super(reason);
    this.name = "SensitiveRevealOIDCCallbackError";
    this.reason = reason;
    this.context = context;
  }
}

const pendingKeys = [
  "challengeId",
  "challengeToken",
  "challengeExpiresAt",
  "resource",
  "recordId",
  "field",
  "purpose",
  "returnPath",
  "completedFactors",
  "transactionToken",
  "provider",
  "state",
  "codeVerifier",
  "expiresAt",
] as const;

const callbackParameterNames = ["code", "state", "error", "error_description", "error_uri", "iss", "session_state"] as const;
const invalidTransactionError = "Sensitive reveal OIDC transaction is invalid";
const untrustedAuthorizationURLError = "OIDC authorization URL is not trusted";

export async function beginSensitiveRevealOIDC(
  input: SensitiveRevealOIDCBeginInput,
  startAPI: (input: SensitiveRevealOIDCStartInput) => Promise<SensitiveRevealOIDCStartResult>,
  options: SensitiveRevealOIDCBeginOptions = {},
) {
  assertBeginInput(input);
  const cryptoAPI = options.crypto ?? globalThis.crypto;
  if (!cryptoAPI?.subtle) throw new Error("Web Crypto is unavailable");

  const verifierBytes = cryptoAPI.getRandomValues(new Uint8Array(32));
  const codeVerifier = base64URL(verifierBytes);
  const digest = await cryptoAPI.subtle.digest("SHA-256", Uint8Array.from(new TextEncoder().encode(codeVerifier)).buffer);
  const codeChallenge = base64URL(new Uint8Array(digest));
  const started = await startAPI({
    challengeId: input.challengeId,
    challengeToken: input.challengeToken,
    resource: input.resource,
    recordId: input.recordId,
    field: input.field,
    purpose: input.purpose,
    provider: input.provider,
    codeChallenge,
  });
  assertStartResult(started, input.challengeId, options.now?.() ?? Date.now());
  const authorizationURL = validateAuthorizationURL(
    started.authorizationUrl,
    options.allowLoopbackHTTP ?? Boolean(import.meta.env?.DEV),
  );
  const pending: PendingSensitiveRevealOIDC = {
    challengeId: input.challengeId,
    challengeToken: input.challengeToken,
    challengeExpiresAt: input.challengeExpiresAt,
    resource: input.resource,
    recordId: input.recordId,
    field: input.field,
    purpose: input.purpose,
    returnPath: input.returnPath,
    completedFactors: [...input.completedFactors],
    transactionToken: started.transactionToken,
    provider: input.provider,
    state: started.state,
    codeVerifier,
    expiresAt: started.expiresAt,
  };

  resolveStorage(options.storage).setItem(SENSITIVE_REVEAL_OIDC_PENDING_KEY, JSON.stringify(pending));
  (options.navigate ?? defaultNavigate)(authorizationURL);
}

export async function consumePendingSensitiveRevealOIDC<TResult>(
  search: string,
  completeAPI: (input: SensitiveRevealOIDCCompleteInput) => Promise<TResult>,
  options: SensitiveRevealOIDCConsumeOptions = {},
): Promise<SensitiveRevealOIDCCompletion<TResult> | null> {
  const params = new URLSearchParams(search);
  const hasCallback = params.has("code") || params.has("state") || params.has("error");
  if (!hasCallback) return null;

  (options.cleanupURL ?? defaultCleanupURL)();
  const storage = resolveStorage(options.storage);
  const rawPending = storage.getItem(SENSITIVE_REVEAL_OIDC_PENDING_KEY);
  storage.removeItem(SENSITIVE_REVEAL_OIDC_PENDING_KEY);
  const pending = parsePending(rawPending);
  const context = pending ? pendingContext(pending) : undefined;

  const codes = params.getAll("code");
  const states = params.getAll("state");
  if (params.has("error") || codes.length !== 1 || states.length !== 1 || !codes[0] || !states[0]) {
    throw new SensitiveRevealOIDCCallbackError("callback", context);
  }

  if (!pending) throw new SensitiveRevealOIDCCallbackError("transaction");
  if (states[0] !== pending.state) throw new SensitiveRevealOIDCCallbackError("state", context);
  if (Date.parse(pending.expiresAt) <= (options.now?.() ?? Date.now())) {
    throw new SensitiveRevealOIDCCallbackError("expired", context);
  }

  let completion: TResult;
  try {
    completion = await completeAPI({
      challengeId: pending.challengeId,
      challengeToken: pending.challengeToken,
      resource: pending.resource,
      recordId: pending.recordId,
      field: pending.field,
      purpose: pending.purpose,
      transactionToken: pending.transactionToken,
      provider: pending.provider,
      code: codes[0],
      state: states[0],
      codeVerifier: pending.codeVerifier,
    });
  } catch {
    throw new SensitiveRevealOIDCCallbackError("exchange", context);
  }
  return {
    context: pendingContext(pending),
    completion,
  };
}

export function clearPendingSensitiveRevealOIDC(storage?: SensitiveRevealOIDCStorage) {
  resolveStorage(storage).removeItem(SENSITIVE_REVEAL_OIDC_PENDING_KEY);
}

export function hasPendingSensitiveRevealOIDC(storage?: SensitiveRevealOIDCStorage) {
  return resolveStorage(storage).getItem(SENSITIVE_REVEAL_OIDC_PENDING_KEY) !== null;
}

function assertBeginInput(input: SensitiveRevealOIDCBeginInput) {
  const values = [
    input.challengeId,
    input.challengeToken,
    input.challengeExpiresAt,
    input.resource,
    input.recordId,
    input.field,
    input.purpose,
    input.provider,
    input.returnPath,
  ];
  if (
    !values.every(isNonEmptyString) ||
    !Number.isFinite(Date.parse(input.challengeExpiresAt)) ||
    !isSafeReturnPath(input.returnPath) ||
    !isSensitiveRevealFactorArray(input.completedFactors)
  ) {
    throw new Error(invalidTransactionError);
  }
}

function assertStartResult(result: SensitiveRevealOIDCStartResult, expectedChallengeId: string, now: number) {
  const expiresAt = Date.parse(result.expiresAt);
  if (
    result.challengeId !== expectedChallengeId ||
    !isNonEmptyString(result.transactionToken) ||
    !isNonEmptyString(result.authorizationUrl) ||
    !isNonEmptyString(result.state) ||
    !isNonEmptyString(result.expiresAt) ||
    !Number.isFinite(expiresAt) ||
    expiresAt <= now
  ) {
    throw new Error(invalidTransactionError);
  }
}

function parsePending(rawPending: string | null): PendingSensitiveRevealOIDC | null {
  if (!rawPending) return null;
  try {
    const value = JSON.parse(rawPending) as Record<string, unknown>;
    if (!value || typeof value !== "object" || Array.isArray(value)) return null;
    const keys = Object.keys(value);
    if (keys.length !== pendingKeys.length || !keys.every((key) => pendingKeys.includes(key as (typeof pendingKeys)[number]))) {
      return null;
    }
    const stringKeys = pendingKeys.filter((key) => key !== "completedFactors");
    if (
      !stringKeys.every((key) => isNonEmptyString(value[key])) ||
      !isSafeReturnPath(value.returnPath) ||
      !isSensitiveRevealFactorArray(value.completedFactors)
    ) {
      return null;
    }
    const expiresAt = Date.parse(value.expiresAt as string);
    const challengeExpiresAt = Date.parse(value.challengeExpiresAt as string);
    if (!Number.isFinite(expiresAt) || !Number.isFinite(challengeExpiresAt)) return null;
    return value as PendingSensitiveRevealOIDC;
  } catch {
    return null;
  }
}

function pendingContext(pending: PendingSensitiveRevealOIDC): SensitiveRevealOIDCContext {
  return {
    challengeId: pending.challengeId,
    challengeToken: pending.challengeToken,
    challengeExpiresAt: pending.challengeExpiresAt,
    resource: pending.resource,
    recordId: pending.recordId,
    field: pending.field,
    purpose: pending.purpose,
    returnPath: pending.returnPath,
    completedFactors: [...pending.completedFactors],
  };
}

function resolveStorage(storage?: SensitiveRevealOIDCStorage) {
  if (storage) return storage;
  if (typeof window === "undefined") throw new Error("sessionStorage is unavailable");
  return window.sessionStorage;
}

function defaultNavigate(authorizationURL: string) {
  window.location.assign(authorizationURL);
}

function defaultCleanupURL() {
  const currentURL = new URL(window.location.href);
  for (const name of callbackParameterNames) currentURL.searchParams.delete(name);
  window.history.replaceState(window.history.state, "", `${currentURL.pathname}${currentURL.search}${currentURL.hash}`);
}

function validateAuthorizationURL(rawURL: string, allowLoopbackHTTP: boolean) {
  try {
    const authorizationURL = new URL(rawURL);
    if (authorizationURL.protocol === "https:") return authorizationURL.toString();
    if (allowLoopbackHTTP && authorizationURL.protocol === "http:" && isLoopbackHostname(authorizationURL.hostname)) {
      return authorizationURL.toString();
    }
  } catch {
    // Normalize malformed and untrusted values to one sanitized browser-boundary error.
  }
  throw new Error(untrustedAuthorizationURLError);
}

function isLoopbackHostname(hostname: string) {
  const normalizedHostname = hostname.toLowerCase();
  return (
    normalizedHostname === "localhost" ||
    normalizedHostname.endsWith(".localhost") ||
    normalizedHostname === "[::1]" ||
    normalizedHostname === "::1" ||
    /^127(?:\.\d{1,3}){3}$/u.test(normalizedHostname)
  );
}

function isNonEmptyString(value: unknown): value is string {
  return typeof value === "string" && value.trim() !== "";
}

function isSensitiveRevealFactorArray(value: unknown): value is SensitiveRevealFactorType[] {
  return Array.isArray(value) && value.every((factor) => factor === "oidc-reauth-v1" || factor === "admin-sms-otp-v1");
}

function isSafeReturnPath(value: unknown): value is string {
  if (!isNonEmptyString(value) || !value.startsWith("/") || value.startsWith("//") || value.includes("\\")) return false;
  return new URL(value, "https://platform.invalid").origin === "https://platform.invalid";
}

function base64URL(bytes: Uint8Array) {
  let binary = "";
  for (const byte of bytes) binary += String.fromCharCode(byte);
  return globalThis.btoa(binary).replaceAll("+", "-").replaceAll("/", "_").replace(/=+$/u, "");
}
