import fs from "node:fs";
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
