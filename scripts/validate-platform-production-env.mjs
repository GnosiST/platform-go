import fs from "node:fs";
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

  validateDriverPair(env, "PLATFORM_ADMIN_RESOURCE_DRIVER", "PLATFORM_ADMIN_RESOURCE_DSN", errors);
  validateDriverPair(env, "PLATFORM_SESSION_DRIVER", "PLATFORM_SESSION_DSN", errors);
  validateDriverPair(env, "PLATFORM_LIFECYCLE_HISTORY_DRIVER", "PLATFORM_LIFECYCLE_HISTORY_DSN", errors);

  if (requireKey(env, "PLATFORM_CACHE_DRIVER", errors) !== "redis") {
    errors.push("PLATFORM_CACHE_DRIVER must be redis");
  }
  if (requireKey(env, "PLATFORM_REDIS_ADDR", errors).trim() === "") {
    errors.push("PLATFORM_REDIS_ADDR must not be empty");
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
  if (capabilities.includes("app-phone")) {
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
