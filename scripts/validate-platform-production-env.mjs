import fs from "node:fs";
import { createECDH } from "node:crypto";
import { isIP } from "node:net";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, "..");

function argValue(name, fallback) {
  const index = process.argv.indexOf(name);
  if (index === -1) return fallback;
  return process.argv[index + 1] ?? "";
}

function hasFlag(name) {
  return process.argv.includes(name);
}

const envFile = path.resolve(repoRoot, argValue("--env-file", "deploy/env/production.example.env"));
const readinessPath = path.resolve(repoRoot, argValue("--readiness", "resources/platform-production-readiness.json"));
const strictSecrets = hasFlag("--strict-secrets");
const composeProfile = !hasFlag("--no-compose");

function values(items) {
  return Array.isArray(items) ? items.filter(Boolean) : [];
}

function parseEnv(source) {
  const env = new Map();
  const errors = [];
  const lines = source.split(/\r?\n/);
  for (let index = 0; index < lines.length; index += 1) {
    const rawLine = lines[index];
    const lineNumber = index + 1;
    const trimmed = rawLine.trim();
    if (trimmed === "" || trimmed.startsWith("#")) {
      continue;
    }
    const normalized = trimmed.startsWith("export ") ? trimmed.slice("export ".length).trim() : trimmed;
    const separator = normalized.indexOf("=");
    if (separator === -1) {
      errors.push(`line ${lineNumber} must use KEY=value syntax`);
      continue;
    }
    const key = normalized.slice(0, separator).trim();
    let value = normalized.slice(separator + 1).trim();
    if (!/^[A-Z0-9_]+$/.test(key)) {
      errors.push(`line ${lineNumber} has invalid env key ${key || "<empty>"}`);
      continue;
    }
    if ((value.startsWith('"') && value.endsWith('"')) || (value.startsWith("'") && value.endsWith("'"))) {
      value = value.slice(1, -1);
    }
    if (env.has(key)) {
      errors.push(`env key ${key} is duplicated`);
    }
    env.set(key, value);
  }
  return { env, errors };
}

function isTruthy(value) {
  return ["true", "1", "yes", "on"].includes(String(value).trim().toLowerCase());
}

function splitCapabilities(value) {
  return String(value ?? "")
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean);
}

function validateNotificationSMS(env, capabilities, errors) {
  const rawProvider = env.get("PLATFORM_NOTIFICATION_SMS_PROVIDER") ?? "";
  const provider = rawProvider.trim().toLowerCase();
  const rawLoginTemplate = env.get("PLATFORM_NOTIFICATION_SMS_LOGIN_TEMPLATE_ID") ?? "";
  const loginTemplate = rawLoginTemplate.trim();
  if (provider === "" && loginTemplate === "") {
    return;
  }
  if (!capabilities.includes("notification")) {
    errors.push("PLATFORM_NOTIFICATION_SMS_PROVIDER requires notification capability");
  }
  if (rawProvider !== provider) {
    errors.push("PLATFORM_NOTIFICATION_SMS_PROVIDER must be canonical trimmed lowercase");
  }
  if (provider === "") {
    errors.push("PLATFORM_NOTIFICATION_SMS_PROVIDER must not be empty when notification SMS is configured");
  } else if (!["aliyun", "tencent", "mock-local"].includes(provider)) {
    errors.push("PLATFORM_NOTIFICATION_SMS_PROVIDER must be aliyun, tencent, or mock-local");
  }
  if (provider === "mock-local") {
    errors.push("PLATFORM_NOTIFICATION_SMS_PROVIDER must not be mock-local in production");
  }
  if (loginTemplate === "") {
    errors.push("PLATFORM_NOTIFICATION_SMS_LOGIN_TEMPLATE_ID must not be empty when notification SMS is configured");
  }
  if (rawLoginTemplate !== loginTemplate) {
    errors.push("PLATFORM_NOTIFICATION_SMS_LOGIN_TEMPLATE_ID must be trimmed");
  }
}

function credentialAuthProviderEnabled(env) {
  return [
    "PLATFORM_CREDENTIAL_AUTH_USERNAME_PASSWORD",
    "PLATFORM_CREDENTIAL_AUTH_PHONE_PASSWORD",
    "PLATFORM_CREDENTIAL_AUTH_EMAIL_PASSWORD",
    "PLATFORM_CREDENTIAL_AUTH_PHONE_SMS_OTP",
  ].some((key) => isTruthy(env.get(key)));
}

function decodeCredentialAuthPrivateKey(value) {
  if (/^[A-Za-z0-9_-]+$/.test(value)) {
    const decoded = Buffer.from(value, "base64url");
    if (decoded.toString("base64url") === value) {
      return decoded;
    }
  }
  if (/^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$/.test(value)) {
    const decoded = Buffer.from(value, "base64");
    if (decoded.toString("base64") === value) {
      return decoded;
    }
  }
  throw new Error("invalid base64 encoding");
}

function validateCredentialAuth(env, capabilities, errors) {
  const usernamePasswordEnabled = isTruthy(env.get("PLATFORM_CREDENTIAL_AUTH_USERNAME_PASSWORD"));
  const phonePasswordEnabled = isTruthy(env.get("PLATFORM_CREDENTIAL_AUTH_PHONE_PASSWORD"));
  const emailPasswordEnabled = isTruthy(env.get("PLATFORM_CREDENTIAL_AUTH_EMAIL_PASSWORD"));
  const phoneSMSOTPEnabled = isTruthy(env.get("PLATFORM_CREDENTIAL_AUTH_PHONE_SMS_OTP"));
  if (!credentialAuthProviderEnabled(env)) {
    return;
  }
  if (!capabilities.includes("credential-auth")) {
    errors.push("credential-auth provider configuration requires credential-auth capability");
  }
  const driver = env.get("PLATFORM_CREDENTIAL_AUTH_REPOSITORY_DRIVER")?.trim() ?? "";
  const dsn = env.get("PLATFORM_CREDENTIAL_AUTH_REPOSITORY_DSN")?.trim() ?? "";
  if (driver === "") {
    errors.push("PLATFORM_CREDENTIAL_AUTH_REPOSITORY_DRIVER is required when credential-auth is enabled");
  } else if (!["mysql", "postgres", "sqlite"].includes(driver)) {
    errors.push("PLATFORM_CREDENTIAL_AUTH_REPOSITORY_DRIVER must be mysql, postgres, or sqlite");
  }
  if (dsn === "") {
    errors.push("PLATFORM_CREDENTIAL_AUTH_REPOSITORY_DSN is required when credential-auth is enabled");
  } else if (strictSecrets && isPlaceholderSecret(dsn)) {
    errors.push("PLATFORM_CREDENTIAL_AUTH_REPOSITORY_DSN must not contain placeholder credentials when --strict-secrets is used");
  }
  const identifierHMACKey = env.get("PLATFORM_CREDENTIAL_AUTH_IDENTIFIER_HMAC_KEY") ?? "";
  if (identifierHMACKey.trim() === "") {
    errors.push("PLATFORM_CREDENTIAL_AUTH_IDENTIFIER_HMAC_KEY is required when credential-auth is enabled");
  } else if (Buffer.byteLength(identifierHMACKey, "utf8") < 32) {
    errors.push("PLATFORM_CREDENTIAL_AUTH_IDENTIFIER_HMAC_KEY must be at least 32 bytes");
  } else if (strictSecrets && isPlaceholderSecret(identifierHMACKey)) {
    errors.push("PLATFORM_CREDENTIAL_AUTH_IDENTIFIER_HMAC_KEY must not be a placeholder when --strict-secrets is used");
  }
  const keyID = env.get("PLATFORM_CREDENTIAL_AUTH_SECRET_TRANSPORT_KEY_ID")?.trim() ?? "";
  if (keyID === "") {
    errors.push("PLATFORM_CREDENTIAL_AUTH_SECRET_TRANSPORT_KEY_ID is required when credential-auth is enabled");
  }
  const privateKey = env.get("PLATFORM_CREDENTIAL_AUTH_SECRET_TRANSPORT_PRIVATE_KEY")?.trim() ?? "";
  if (privateKey === "") {
    errors.push("PLATFORM_CREDENTIAL_AUTH_SECRET_TRANSPORT_PRIVATE_KEY is required when credential-auth is enabled");
  } else {
    try {
      const decoded = decodeCredentialAuthPrivateKey(privateKey);
      if (decoded.length !== 32) {
        throw new Error("invalid length");
      }
      createECDH("prime256v1").setPrivateKey(decoded);
    } catch {
      errors.push("PLATFORM_CREDENTIAL_AUTH_SECRET_TRANSPORT_PRIVATE_KEY must be a valid base64-encoded 32-byte P-256 private key");
    }
  }
  if (usernamePasswordEnabled || phonePasswordEnabled || emailPasswordEnabled) {
    if ((env.get("PLATFORM_CREDENTIAL_AUTH_BOOTSTRAP_ADMIN_USERNAME") ?? "").trim() === "") {
      errors.push("PLATFORM_CREDENTIAL_AUTH_BOOTSTRAP_ADMIN_USERNAME is required for credential password providers");
    }
    if ((env.get("PLATFORM_CREDENTIAL_AUTH_BOOTSTRAP_ADMIN_PASSWORD") ?? "").trim() === "") {
      errors.push("PLATFORM_CREDENTIAL_AUTH_BOOTSTRAP_ADMIN_PASSWORD is required for credential password providers");
    }
  }
  if ((phonePasswordEnabled || phoneSMSOTPEnabled) && (env.get("PLATFORM_CREDENTIAL_AUTH_BOOTSTRAP_ADMIN_PHONE") ?? "").trim() === "") {
    errors.push("PLATFORM_CREDENTIAL_AUTH_BOOTSTRAP_ADMIN_PHONE is required for credential phone providers");
  }
  if (emailPasswordEnabled && (env.get("PLATFORM_CREDENTIAL_AUTH_BOOTSTRAP_ADMIN_EMAIL") ?? "").trim() === "") {
    errors.push("PLATFORM_CREDENTIAL_AUTH_BOOTSTRAP_ADMIN_EMAIL is required for credential email password providers");
  }
  if (phoneSMSOTPEnabled) {
    if (!capabilities.includes("notification")) {
      errors.push("credential-auth phone SMS OTP requires notification capability");
    }
    if ((env.get("PLATFORM_NOTIFICATION_SMS_PROVIDER") ?? "").trim() === "") {
      errors.push("PLATFORM_NOTIFICATION_SMS_PROVIDER is required for credential-auth phone SMS OTP");
    }
    if ((env.get("PLATFORM_NOTIFICATION_SMS_LOGIN_TEMPLATE_ID") ?? "").trim() === "") {
      errors.push("PLATFORM_NOTIFICATION_SMS_LOGIN_TEMPLATE_ID is required for credential-auth phone SMS OTP");
    }
  }
}

function isPlaceholderSecret(value) {
  const normalized = String(value ?? "").trim().toLowerCase();
  if (normalized === "") {
    return true;
  }
  return [
    "replace-with",
    "change-me",
    "changeme",
    "placeholder",
    "at-least-32-characters",
    "private-root-password",
    "platform_pass",
    "password123",
  ].some((marker) => normalized.includes(marker));
}

function requireKey(env, key, errors) {
  if (!env.has(key)) {
    errors.push(`${key} is required`);
    return "";
  }
  return env.get(key) ?? "";
}

function isCanonicalDataKeyID(value) {
  const normalized = String(value ?? "");
  if (normalized === "" || normalized !== normalized.trim().toLowerCase()) {
    return false;
  }
  return /^[a-z0-9](?:[a-z0-9.:-]*[a-z0-9])?$/.test(normalized);
}

function duplicateJSONKeys(raw) {
  const seen = new Set();
  const duplicates = new Set();
  const keyPattern = /"((?:\\.|[^"\\])*)"\s*:/gu;
  for (const match of String(raw ?? "").matchAll(keyPattern)) {
    let key;
    try {
      key = JSON.parse(`"${match[1]}"`);
    } catch {
      continue;
    }
    if (seen.has(key)) {
      duplicates.add(key);
    }
    seen.add(key);
  }
  return duplicates;
}

function parseDataKeyring(raw, label, errors) {
  if (duplicateJSONKeys(raw).size > 0) {
    errors.push(`${label} key IDs must be unique`);
  }
  let parsed;
  try {
    parsed = JSON.parse(raw);
  } catch {
    errors.push(`${label} keyring JSON is invalid`);
    return new Map();
  }
  if (parsed === null || Array.isArray(parsed) || typeof parsed !== "object" || Object.keys(parsed).length === 0) {
    errors.push(`${label} keyring must be a non-empty object`);
    return new Map();
  }
  const result = new Map();
  for (const [id, encoded] of Object.entries(parsed)) {
    if (!isCanonicalDataKeyID(id)) {
      errors.push(`${label} key ID must be canonical`);
      continue;
    }
    if (typeof encoded !== "string" || !/^[A-Za-z0-9+/]{43}=$/.test(encoded)) {
      errors.push(`${label} key material must be base64-encoded 32-byte values`);
      continue;
    }
    const material = Buffer.from(encoded, "base64");
    if (material.length !== 32 || material.toString("base64") !== encoded) {
      errors.push(`${label} key material must be base64-encoded 32-byte values`);
      continue;
    }
    if (strictSecrets && isPlaceholderSecret(material.toString("utf8"))) {
      errors.push(`${label} key material must not be a placeholder when --strict-secrets is used`);
    }
    result.set(id, encoded);
  }
  return result;
}

function validateDriverPair(env, driverKey, dsnKey, errors) {
  const driver = requireKey(env, driverKey, errors);
  const dsn = requireKey(env, dsnKey, errors);
  if (driver && !["mysql", "postgres", "sqlite"].includes(driver)) {
    errors.push(`${driverKey} must be mysql, postgres, or sqlite`);
  }
  if (driver && dsn.trim() === "") {
    errors.push(`${dsnKey} is required when ${driverKey} is set`);
  }
  if (strictSecrets && isPlaceholderSecret(dsn)) {
    errors.push(`${dsnKey} must not contain placeholder credentials when --strict-secrets is used`);
  }
}

function parseIPv4(address) {
  return Uint8Array.from(address.split(".").map(Number));
}

function parseIPv6(address) {
  let value = address;
  if (value.includes(".")) {
    const split = value.lastIndexOf(":");
    const ipv4 = parseIPv4(value.slice(split + 1));
    value = `${value.slice(0, split)}:${((ipv4[0] << 8) | ipv4[1]).toString(16)}:${((ipv4[2] << 8) | ipv4[3]).toString(16)}`;
  }
  const halves = value.split("::");
  const left = halves[0] ? halves[0].split(":") : [];
  const right = halves.length > 1 && halves[1] ? halves[1].split(":") : [];
  const missing = 8 - left.length - right.length;
  const groups = halves.length === 1 ? left : [...left, ...Array(missing).fill("0"), ...right];
  const bytes = new Uint8Array(16);
  groups.forEach((group, index) => {
    const value = Number.parseInt(group || "0", 16);
    bytes[index * 2] = value >> 8;
    bytes[index * 2 + 1] = value & 0xff;
  });
  return bytes;
}

function parseTrustedProxy(value) {
  const slash = value.lastIndexOf("/");
  const address = slash === -1 ? value : value.slice(0, slash);
  const family = isIP(address);
  if (family === 0 || address.includes("%")) {
    return null;
  }
  const maxBits = family === 4 ? 32 : 128;
  const bitsText = slash === -1 ? String(maxBits) : value.slice(slash + 1);
  if (!/^[0-9]+$/.test(bitsText)) {
    return null;
  }
  const bits = Number(bitsText);
  if (bits < 0 || bits > maxBits) {
    return null;
  }
  return { family, bytes: family === 4 ? parseIPv4(address) : parseIPv6(address), bits };
}

function newCoverageNode() {
  return { covered: false, children: [null, null] };
}

function insertCoverage(root, proxy) {
  let node = root;
  if (node.covered) return;
  for (let depth = 0; depth < proxy.bits; depth += 1) {
    const bit = (proxy.bytes[Math.floor(depth / 8)] >> (7 - (depth % 8))) & 1;
    node.children[bit] ??= newCoverageNode();
    node = node.children[bit];
    if (node.covered) return;
  }
  node.covered = true;
  node.children = [null, null];
  collapseCoverage(root, proxy.bytes, proxy.bits, 0);
}

function collapseCoverage(node, bytes, bits, depth) {
  if (depth < bits) {
    const bit = (bytes[Math.floor(depth / 8)] >> (7 - (depth % 8))) & 1;
    collapseCoverage(node.children[bit], bytes, bits, depth + 1);
  }
  if (node.children[0]?.covered && node.children[1]?.covered) {
    node.covered = true;
    node.children = [null, null];
  }
}

function addressInPrefix(address, proxy) {
  if (!address || address.family !== proxy.family) return false;
  for (let bit = 0; bit < proxy.bits; bit += 1) {
    const mask = 1 << (7 - (bit % 8));
    if ((address.bytes[Math.floor(bit / 8)] & mask) !== (proxy.bytes[Math.floor(bit / 8)] & mask)) return false;
  }
  return true;
}

function normalizedProxyKey(proxy) {
  const bytes = Uint8Array.from(proxy.bytes);
  const wholeBytes = Math.floor(proxy.bits / 8);
  const remainingBits = proxy.bits % 8;
  if (remainingBits > 0) {
    bytes[wholeBytes] &= (0xff << (8 - remainingBits)) & 0xff;
  }
  for (let index = wholeBytes + (remainingBits > 0 ? 1 : 0); index < bytes.length; index += 1) {
    bytes[index] = 0;
  }
  return `${proxy.family}/${proxy.bits}/${Buffer.from(bytes).toString("hex")}`;
}

function hasCanonicalPrefixAddress(proxy) {
  const normalized = normalizedProxyKey(proxy).split("/").at(-1);
  return normalized === Buffer.from(proxy.bytes).toString("hex");
}

function canonicalProxyValue(value, proxy) {
  if (!proxy || !hasCanonicalPrefixAddress(proxy)) return "";
  let address;
  if (proxy.family === 4) {
    address = Array.from(proxy.bytes).join(".");
  } else {
    const groups = Array.from({ length: 8 }, (_, index) => ((proxy.bytes[index * 2] << 8) | proxy.bytes[index * 2 + 1]).toString(16));
    let bestStart = -1;
    let bestLength = 0;
    for (let start = 0; start < groups.length;) {
      if (groups[start] !== "0") {
        start += 1;
        continue;
      }
      let end = start;
      while (end < groups.length && groups[end] === "0") end += 1;
      if (end - start > bestLength && end - start >= 2) {
        bestStart = start;
        bestLength = end - start;
      }
      start = end;
    }
    address = bestStart === -1
      ? groups.join(":")
      : `${groups.slice(0, bestStart).join(":")}::${groups.slice(bestStart + bestLength).join(":")}`;
  }
  return value.includes("/") ? `${address}/${proxy.bits}` : address;
}

function isInvalidEdgeAddress(proxy) {
  if (!proxy) return true;
  if (proxy.family === 4) {
    return proxy.bytes.every((byte) => byte === 0) || proxy.bytes[0] === 127 || (proxy.bytes[0] >= 224 && proxy.bytes[0] <= 239);
  }
  const unspecified = proxy.bytes.every((byte) => byte === 0);
  const loopback = proxy.bytes.slice(0, -1).every((byte) => byte === 0) && proxy.bytes.at(-1) === 1;
  return unspecified || loopback || proxy.bytes[0] === 0xff;
}

function validateRequiredReadinessEnv(env, readiness, errors) {
  for (const item of values(readiness.requiredEnv)) {
    requireKey(env, item.name, errors);
  }
  requireKey(env, "PLATFORM_CAPABILITIES", errors);
}

function validatePlatformEnv(env, errors) {
  if (requireKey(env, "PLATFORM_RUNTIME_ENV", errors) !== "production") {
    errors.push("PLATFORM_RUNTIME_ENV must be production");
  }
  const jwtSecret = requireKey(env, "PLATFORM_JWT_SECRET", errors);
  if (jwtSecret.length < 32) {
    errors.push("PLATFORM_JWT_SECRET must be at least 32 characters");
  }
  if (jwtSecret === "dev-platform-go-secret") {
    errors.push("PLATFORM_JWT_SECRET must not use the development default");
  }
  if (strictSecrets && isPlaceholderSecret(jwtSecret)) {
    errors.push("PLATFORM_JWT_SECRET must not be a placeholder when --strict-secrets is used");
  }

  const dataKeyProvider = requireKey(env, "PLATFORM_DATA_KEY_PROVIDER", errors);
  if (dataKeyProvider && dataKeyProvider !== "env-aes256") {
    errors.push("PLATFORM_DATA_KEY_PROVIDER must be env-aes256 in production");
  }
  const activeEncryptionKeyID = requireKey(env, "PLATFORM_DATA_ENCRYPTION_ACTIVE_KEY_ID", errors);
  const activeBlindIndexKeyID = requireKey(env, "PLATFORM_DATA_BLIND_INDEX_ACTIVE_KEY_ID", errors);
  if (activeEncryptionKeyID && !isCanonicalDataKeyID(activeEncryptionKeyID)) {
    errors.push("PLATFORM_DATA_ENCRYPTION_ACTIVE_KEY_ID must be canonical");
  }
  if (activeBlindIndexKeyID && !isCanonicalDataKeyID(activeBlindIndexKeyID)) {
    errors.push("PLATFORM_DATA_BLIND_INDEX_ACTIVE_KEY_ID must be canonical");
  }
  const encryptionKeys = parseDataKeyring(requireKey(env, "PLATFORM_DATA_ENCRYPTION_KEYRING_JSON", errors), "data encryption", errors);
  const blindIndexKeys = parseDataKeyring(requireKey(env, "PLATFORM_DATA_BLIND_INDEX_KEYRING_JSON", errors), "data blind-index", errors);
  if (activeEncryptionKeyID && !encryptionKeys.has(activeEncryptionKeyID)) {
    errors.push("active encryption key is unavailable");
  }
  if (activeBlindIndexKeyID && !blindIndexKeys.has(activeBlindIndexKeyID)) {
    errors.push("active blind-index key is unavailable");
  }
  const encryptionMaterial = new Set(encryptionKeys.values());
  if ([...blindIndexKeys.values()].some((material) => encryptionMaterial.has(material))) {
    errors.push("data encryption and blind-index key material must be distinct");
  }

  validateDriverPair(env, "PLATFORM_ADMIN_RESOURCE_DRIVER", "PLATFORM_ADMIN_RESOURCE_DSN", errors);
  validateDriverPair(env, "PLATFORM_SESSION_DRIVER", "PLATFORM_SESSION_DSN", errors);
  validateDriverPair(env, "PLATFORM_LIFECYCLE_HISTORY_DRIVER", "PLATFORM_LIFECYCLE_HISTORY_DSN", errors);

  if (requireKey(env, "PLATFORM_CACHE_DRIVER", errors) !== "redis") {
    errors.push("PLATFORM_CACHE_DRIVER must be redis");
  }
  if (requireKey(env, "PLATFORM_REDIS_ADDR", errors).trim() === "") {
    errors.push("PLATFORM_REDIS_ADDR must not be empty");
  }
  const rateLimitHMACKey = requireKey(env, "PLATFORM_RATE_LIMIT_HMAC_KEY", errors);
  if (Buffer.byteLength(rateLimitHMACKey, "utf8") < 32) {
    errors.push("PLATFORM_RATE_LIMIT_HMAC_KEY must be at least 32 bytes");
  }
  if (strictSecrets && isPlaceholderSecret(rateLimitHMACKey)) {
    errors.push("PLATFORM_RATE_LIMIT_HMAC_KEY must not be a placeholder when --strict-secrets is used");
  }
  const phoneHMACKey = env.get("PLATFORM_PHONE_HMAC_KEY") ?? "";
  const phoneCodeHMACKey = env.get("PLATFORM_PHONE_CODE_HMAC_KEY") ?? "";
  if (rateLimitHMACKey && (rateLimitHMACKey === phoneHMACKey || rateLimitHMACKey === phoneCodeHMACKey)) {
    errors.push("PLATFORM_RATE_LIMIT_HMAC_KEY must be distinct from phone and code HMAC keys");
  }
  const sensitiveRevealHMACKey = env.get("PLATFORM_SENSITIVE_REVEAL_HMAC_KEY") ?? "";
  if (sensitiveRevealHMACKey !== "") {
    if (Buffer.byteLength(sensitiveRevealHMACKey, "utf8") < 32) {
      errors.push("PLATFORM_SENSITIVE_REVEAL_HMAC_KEY must be at least 32 bytes");
    }
    if ([env.get("PLATFORM_JWT_SECRET") ?? "", phoneHMACKey, phoneCodeHMACKey, rateLimitHMACKey].includes(sensitiveRevealHMACKey)) {
      errors.push("PLATFORM_SENSITIVE_REVEAL_HMAC_KEY must be distinct from JWT, phone, code, and rate-limit keys");
    }
    if (strictSecrets && isPlaceholderSecret(sensitiveRevealHMACKey)) {
      errors.push("PLATFORM_SENSITIVE_REVEAL_HMAC_KEY must not be a placeholder when --strict-secrets is used");
    }
  }
  if (!isTruthy(requireKey(env, "PLATFORM_DISABLE_DEMO_AUTH_PROVIDER", errors))) {
    errors.push("PLATFORM_DISABLE_DEMO_AUTH_PROVIDER must be true");
  }

  const publicBaseURL = requireKey(env, "PLATFORM_PUBLIC_BASE_URL", errors).trim();
  try {
    const parsed = new URL(publicBaseURL);
    if (parsed.protocol !== "https:" || parsed.username || parsed.password || parsed.search || parsed.hash || parsed.pathname !== "/" || publicBaseURL.endsWith("/")) {
      errors.push("PLATFORM_PUBLIC_BASE_URL must be an absolute HTTPS origin");
    }
  } catch {
    errors.push("PLATFORM_PUBLIC_BASE_URL must be an absolute HTTPS origin");
  }
  const trustedProxies = requireKey(env, "PLATFORM_TRUSTED_PROXIES", errors)
    .split(",")
    .map((item) => item.trim());
  if (trustedProxies.length === 0 || trustedProxies.some((item) => item === "")) {
    errors.push("PLATFORM_TRUSTED_PROXIES must not be empty");
  }
  const parsedTrustedProxies = [];
  const coverage = { 4: newCoverageNode(), 6: newCoverageNode() };
  const directTrustAll = new Set();
  const normalizedProxies = new Set();
  for (const proxy of trustedProxies) {
    const parsed = parseTrustedProxy(proxy);
    if (!parsed) {
      errors.push(`PLATFORM_TRUSTED_PROXIES contains invalid IP or CIDR ${proxy}`);
    } else if (parsed.bits === 0) {
      errors.push("PLATFORM_TRUSTED_PROXIES must not trust all addresses");
      directTrustAll.add(parsed.family);
    } else {
      const normalized = normalizedProxyKey(parsed);
      if (normalizedProxies.has(normalized)) {
        errors.push(`PLATFORM_TRUSTED_PROXIES contains duplicate normalized prefix ${proxy}`);
      } else {
        normalizedProxies.add(normalized);
        parsedTrustedProxies.push(parsed);
        insertCoverage(coverage[parsed.family], parsed);
      }
    }
  }
  if (coverage[4].covered && !directTrustAll.has(4)) {
    errors.push("PLATFORM_TRUSTED_PROXIES must not cumulatively trust all IPv4 addresses");
  }
  if (coverage[6].covered && !directTrustAll.has(6)) {
    errors.push("PLATFORM_TRUSTED_PROXIES must not cumulatively trust all IPv6 addresses");
  }
  const edgeTrustedProxyValue = requireKey(env, "PLATFORM_EDGE_TRUSTED_PROXY", errors).trim();
  const edgeTrustedProxy = parseTrustedProxy(edgeTrustedProxyValue);
  if (edgeTrustedProxyValue.includes("/") || isInvalidEdgeAddress(edgeTrustedProxy) || canonicalProxyValue(edgeTrustedProxyValue, edgeTrustedProxy) !== edgeTrustedProxyValue) {
    errors.push("PLATFORM_EDGE_TRUSTED_PROXY must be one canonical IP address");
  }
  if (composeProfile) {
    const adminProxyValue = requireKey(env, "PLATFORM_ADMIN_PROXY_IP", errors).trim();
    const adminProxyFamily = isIP(adminProxyValue);
    const adminProxy = adminProxyFamily === 0 ? null : {
      family: adminProxyFamily,
      bytes: adminProxyFamily === 4 ? parseIPv4(adminProxyValue) : parseIPv6(adminProxyValue),
    };
    if (!adminProxy || !parsedTrustedProxies.some((proxy) => addressInPrefix(adminProxy, proxy))) {
      errors.push("PLATFORM_ADMIN_PROXY_IP must be contained in PLATFORM_TRUSTED_PROXIES");
    }
    const internalSubnetValue = requireKey(env, "PLATFORM_INTERNAL_SUBNET", errors).trim();
    const internalSubnet = parseTrustedProxy(internalSubnetValue);
    if (!internalSubnet || internalSubnet.bits === 0 || canonicalProxyValue(internalSubnetValue, internalSubnet) !== internalSubnetValue) {
      errors.push("PLATFORM_INTERNAL_SUBNET must be one canonical narrow CIDR");
    } else if (!edgeTrustedProxy || edgeTrustedProxy.bits < internalSubnet.bits || !addressInPrefix(edgeTrustedProxy, internalSubnet)) {
      errors.push("PLATFORM_EDGE_TRUSTED_PROXY must be contained in PLATFORM_INTERNAL_SUBNET");
    }
  }
  const maxBodyBytes = Number(requireKey(env, "PLATFORM_HTTP_MAX_BODY_BYTES", errors));
  if (!Number.isSafeInteger(maxBodyBytes) || maxBodyBytes < 1 || maxBodyBytes > 100 * 1024 * 1024) {
    errors.push("PLATFORM_HTTP_MAX_BODY_BYTES must be between 1 and 104857600");
  }

  const maxUploadBytes = Number(requireKey(env, "PLATFORM_FILE_MAX_UPLOAD_BYTES", errors));
  if (!Number.isSafeInteger(maxUploadBytes) || maxUploadBytes < 1 || maxUploadBytes > 100 * 1024 * 1024) {
    errors.push("PLATFORM_FILE_MAX_UPLOAD_BYTES must be between 1 and 104857600");
  }
  const allowedMIMETypes = requireKey(env, "PLATFORM_FILE_ALLOWED_MIME_TYPES", errors)
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean);
  if (allowedMIMETypes.length === 0) {
    errors.push("PLATFORM_FILE_ALLOWED_MIME_TYPES must not be empty");
  }
  for (const mediaType of allowedMIMETypes) {
    if (!/^[a-z0-9][a-z0-9!#$&^_.+-]*\/[a-z0-9][a-z0-9!#$&^_.+-]*$/.test(mediaType)) {
      errors.push(`PLATFORM_FILE_ALLOWED_MIME_TYPES contains invalid canonical media type ${mediaType}`);
    }
  }
  if (env.has("PLATFORM_FILE_STORAGE_PUBLIC_URL")) {
    errors.push("PLATFORM_FILE_STORAGE_PUBLIC_URL must not be configured");
  }
  const fileStorageDriver = requireKey(env, "PLATFORM_FILE_STORAGE_DRIVER", errors);
  if (!["local", "s3"].includes(fileStorageDriver)) {
    errors.push("PLATFORM_FILE_STORAGE_DRIVER must be local or s3");
  }
  if (fileStorageDriver === "s3") {
    requireKey(env, "PLATFORM_FILE_STORAGE_S3_REGION", errors);
    requireKey(env, "PLATFORM_FILE_STORAGE_S3_BUCKET", errors);
    const endpoint = env.get("PLATFORM_FILE_STORAGE_S3_ENDPOINT")?.trim() ?? "";
    if (endpoint !== "" && !endpoint.startsWith("https://")) {
      errors.push("PLATFORM_FILE_STORAGE_S3_ENDPOINT must use https in production");
    }
    const encryption = requireKey(env, "PLATFORM_FILE_STORAGE_S3_SERVER_SIDE_ENCRYPTION", errors);
    if (!["AES256", "aws:kms"].includes(encryption)) {
      errors.push("PLATFORM_FILE_STORAGE_S3_SERVER_SIDE_ENCRYPTION must be AES256 or aws:kms");
    }
    if (encryption === "aws:kms" && requireKey(env, "PLATFORM_FILE_STORAGE_S3_KMS_KEY_ID", errors).trim() === "") {
      errors.push("PLATFORM_FILE_STORAGE_S3_KMS_KEY_ID must not be empty for aws:kms");
    }
  }

  const capabilities = splitCapabilities(requireKey(env, "PLATFORM_CAPABILITIES", errors));
  if (capabilities.length === 0) {
    errors.push("PLATFORM_CAPABILITIES must not be empty");
  }
  if (capabilities.includes("demo-data")) {
    errors.push("PLATFORM_CAPABILITIES must not include demo-data in production");
  }
  validateNotificationSMS(env, capabilities, errors);
  validateCredentialAuth(env, capabilities, errors);
  const adminStepUpPhoneKeys = [
    "PLATFORM_ADMIN_STEP_UP_PHONE_RESOURCE",
    "PLATFORM_ADMIN_STEP_UP_PHONE_ACTOR_FIELD",
    "PLATFORM_ADMIN_STEP_UP_PHONE_FIELD",
    "PLATFORM_ADMIN_STEP_UP_PHONE_VERIFIED_AT_FIELD",
    "PLATFORM_ADMIN_STEP_UP_PHONE_VERIFIED_DIGEST_FIELD",
  ];
  const configuredAdminStepUpPhoneKeys = adminStepUpPhoneKeys.filter((key) => (env.get(key) ?? "").trim() !== "");
  if (configuredAdminStepUpPhoneKeys.length > 0 && configuredAdminStepUpPhoneKeys.length !== adminStepUpPhoneKeys.length) {
    errors.push("all PLATFORM_ADMIN_STEP_UP_PHONE_* values must be configured together");
  }
  const adminStepUpPhoneConfigured = configuredAdminStepUpPhoneKeys.length === adminStepUpPhoneKeys.length;
  if (adminStepUpPhoneConfigured && sensitiveRevealHMACKey === "") {
    errors.push("PLATFORM_SENSITIVE_REVEAL_HMAC_KEY is required when Admin step-up phone is configured");
  }
  if (capabilities.includes("app-phone") || adminStepUpPhoneConfigured) {
    const phoneKey = requireKey(env, "PLATFORM_PHONE_HMAC_KEY", errors);
    const codeKey = requireKey(env, "PLATFORM_PHONE_CODE_HMAC_KEY", errors);
    const rawProvider = requireKey(env, "PLATFORM_PHONE_VERIFICATION_PROVIDER", errors);
    const provider = rawProvider.trim().toLowerCase();
    if (Buffer.byteLength(phoneKey, "utf8") < 32) {
      errors.push("PLATFORM_PHONE_HMAC_KEY must be at least 32 bytes");
    }
    if (Buffer.byteLength(codeKey, "utf8") < 32) {
      errors.push("PLATFORM_PHONE_CODE_HMAC_KEY must be at least 32 bytes");
    }
    if (phoneKey === codeKey) {
      errors.push("phone and code HMAC keys must be distinct");
    }
    if (rawProvider.trim() === "") {
      errors.push("PLATFORM_PHONE_VERIFICATION_PROVIDER must not be empty");
    }
    if (rawProvider !== provider) {
      errors.push("PLATFORM_PHONE_VERIFICATION_PROVIDER must be canonical trimmed lowercase");
    }
    if (provider === "unknown") {
      errors.push("PLATFORM_PHONE_VERIFICATION_PROVIDER must identify a configured provider");
    }
    if (provider === "debug") {
      errors.push("PLATFORM_PHONE_VERIFICATION_PROVIDER must not be debug in production");
    }
    if (strictSecrets && isPlaceholderSecret(phoneKey)) {
      errors.push("PLATFORM_PHONE_HMAC_KEY must not be a placeholder when --strict-secrets is used");
    }
    if (strictSecrets && isPlaceholderSecret(codeKey)) {
      errors.push("PLATFORM_PHONE_CODE_HMAC_KEY must not be a placeholder when --strict-secrets is used");
    }
  }
}

function validateComposeEnv(env, errors) {
  if (!composeProfile) {
    return;
  }
  const rootPassword = requireKey(env, "MYSQL_ROOT_PASSWORD", errors);
  const mysqlPassword = requireKey(env, "MYSQL_PASSWORD", errors);
  requireKey(env, "MYSQL_DATABASE", errors);
  requireKey(env, "MYSQL_USER", errors);
  if (strictSecrets && isPlaceholderSecret(rootPassword)) {
    errors.push("MYSQL_ROOT_PASSWORD must not be a placeholder when --strict-secrets is used");
  }
  if (strictSecrets && isPlaceholderSecret(mysqlPassword)) {
    errors.push("MYSQL_PASSWORD must not be a placeholder when --strict-secrets is used");
  }
}

function validate() {
  const errors = [];
  if (!fs.existsSync(envFile)) {
    return { errors: [`env file is missing: ${path.relative(repoRoot, envFile)}`] };
  }
  if (!fs.existsSync(readinessPath)) {
    return { errors: [`readiness contract is missing: ${path.relative(repoRoot, readinessPath)}`] };
  }
  const parsed = parseEnv(fs.readFileSync(envFile, "utf8"));
  errors.push(...parsed.errors);
  const readiness = JSON.parse(fs.readFileSync(readinessPath, "utf8"));
  validateRequiredReadinessEnv(parsed.env, readiness, errors);
  validatePlatformEnv(parsed.env, errors);
  validateComposeEnv(parsed.env, errors);
  return { errors };
}

const { errors } = validate();
if (errors.length > 0) {
  for (const error of errors) {
    console.error(error);
  }
  process.exit(1);
}

console.log(
  `Validated production env ${path.relative(repoRoot, envFile)} (${strictSecrets ? "strict-secrets" : "template"})`,
);
