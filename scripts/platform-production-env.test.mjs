import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");

function runValidator(args = []) {
  return spawnSync(process.execPath, ["scripts/validate-platform-production-env.mjs", ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

function tempEnv(source) {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-production-env-"));
  const filePath = path.join(tempDir, "production.env");
  fs.writeFileSync(filePath, source);
  return { tempDir, filePath };
}

const validStrictEnv = [
  "PLATFORM_RUNTIME_ENV=production",
  "PLATFORM_HTTP_ADDR=0.0.0.0:9200",
  "PLATFORM_PUBLIC_BASE_URL=https://platform.example.test",
  "PLATFORM_TRUSTED_PROXIES=10.20.0.0/16",
  "PLATFORM_INTERNAL_SUBNET=10.20.0.0/16",
  "PLATFORM_ADMIN_PROXY_IP=10.20.1.4",
  "PLATFORM_HTTP_MAX_BODY_BYTES=1048576",
  "PLATFORM_JWT_SECRET=prod-jwt-signing-value-with-strong-length-001",
  "PLATFORM_CAPABILITIES=tenant,identity,session,rbac,menu,api-resource,audit,wechat-login,dictionary,parameter,file-storage,admin-shell,system-admin",
  "PLATFORM_DISABLE_DEMO_AUTH_PROVIDER=true",
  "PLATFORM_ADMIN_RESOURCE_DRIVER=mysql",
  "PLATFORM_ADMIN_RESOURCE_DSN=platform:strong-db-pass@tcp(platform-mysql:3306)/platform?charset=utf8mb4&parseTime=True&loc=Local",
  "PLATFORM_SESSION_DRIVER=mysql",
  "PLATFORM_SESSION_DSN=platform:strong-db-pass@tcp(platform-mysql:3306)/platform?charset=utf8mb4&parseTime=True&loc=Local",
  "PLATFORM_LIFECYCLE_HISTORY_DRIVER=mysql",
  "PLATFORM_LIFECYCLE_HISTORY_DSN=platform:strong-db-pass@tcp(platform-mysql:3306)/platform?charset=utf8mb4&parseTime=True&loc=Local",
  "PLATFORM_CACHE_DRIVER=redis",
  "PLATFORM_REDIS_ADDR=platform-redis:6379",
  "PLATFORM_FILE_STORAGE_DRIVER=s3",
  "PLATFORM_FILE_MAX_UPLOAD_BYTES=10485760",
  "PLATFORM_FILE_ALLOWED_MIME_TYPES=application/pdf,image/jpeg,image/png,text/plain",
  "PLATFORM_FILE_STORAGE_S3_ENDPOINT=https://s3.example.test",
  "PLATFORM_FILE_STORAGE_S3_REGION=us-east-1",
  "PLATFORM_FILE_STORAGE_S3_BUCKET=platform",
  "PLATFORM_FILE_STORAGE_S3_SERVER_SIDE_ENCRYPTION=AES256",
  "PLATFORM_FILE_STORAGE_S3_KMS_KEY_ID=",
  "MYSQL_ROOT_PASSWORD=strong-root-password-value",
  "MYSQL_DATABASE=platform",
  "MYSQL_USER=platform",
  "MYSQL_PASSWORD=strong-db-pass",
  "",
].join("\n");

describe("validate-platform-production-env", () => {
  it("accepts the standard production env template without strict secret checks", () => {
    const result = runValidator();

    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Validated production env deploy\/env\/production\.example\.env \(template\)/);
  });

  it("rejects the standard template when strict secret checks are enabled", () => {
    const result = runValidator(["--strict-secrets"]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /PLATFORM_JWT_SECRET must not be a placeholder/);
    assert.match(result.stderr, /MYSQL_ROOT_PASSWORD must not be a placeholder/);
    assert.match(result.stderr, /MYSQL_PASSWORD must not be a placeholder/);
  });

  it("accepts a private production env with strict secret checks", () => {
    const { tempDir, filePath } = tempEnv(validStrictEnv);
    try {
      const result = runValidator(["--env-file", filePath, "--strict-secrets"]);

      assert.equal(result.status, 0, result.stderr);
      assert.match(result.stdout, /strict-secrets/);
    } finally {
      fs.rmSync(tempDir, { recursive: true, force: true });
    }
  });

  it("rejects production env files that enable demo data or demo auth", () => {
    const source = validStrictEnv
      .replace("system-admin", "system-admin,demo-data")
      .replace("PLATFORM_DISABLE_DEMO_AUTH_PROVIDER=true", "PLATFORM_DISABLE_DEMO_AUTH_PROVIDER=false");
    const { tempDir, filePath } = tempEnv(source);
    try {
      const result = runValidator(["--env-file", filePath, "--strict-secrets"]);

      assert.notEqual(result.status, 0, result.stdout);
      assert.match(result.stderr, /PLATFORM_DISABLE_DEMO_AUTH_PROVIDER must be true/);
      assert.match(result.stderr, /PLATFORM_CAPABILITIES must not include demo-data/);
    } finally {
      fs.rmSync(tempDir, { recursive: true, force: true });
    }
  });

  it("rejects missing Redis and GORM production settings", () => {
    const source = validStrictEnv
      .replace("PLATFORM_CACHE_DRIVER=redis", "PLATFORM_CACHE_DRIVER=memory")
      .replace("PLATFORM_SESSION_DRIVER=mysql", "PLATFORM_SESSION_DRIVER=file")
      .replace("PLATFORM_ADMIN_RESOURCE_DSN=platform:strong-db-pass@tcp(platform-mysql:3306)/platform?charset=utf8mb4&parseTime=True&loc=Local\n", "");
    const { tempDir, filePath } = tempEnv(source);
    try {
      const result = runValidator(["--env-file", filePath, "--strict-secrets"]);

      assert.notEqual(result.status, 0, result.stdout);
      assert.match(result.stderr, /PLATFORM_ADMIN_RESOURCE_DSN is required/);
      assert.match(result.stderr, /PLATFORM_SESSION_DRIVER must be mysql, postgres, or sqlite/);
      assert.match(result.stderr, /PLATFORM_CACHE_DRIVER must be redis/);
    } finally {
      fs.rmSync(tempDir, { recursive: true, force: true });
    }
  });

  it("rejects unsafe production file upload and S3 settings", () => {
    const source = validStrictEnv
      .replace("PLATFORM_FILE_MAX_UPLOAD_BYTES=10485760", "PLATFORM_FILE_MAX_UPLOAD_BYTES=1073741824")
      .replace("PLATFORM_FILE_ALLOWED_MIME_TYPES=application/pdf,image/jpeg,image/png,text/plain\n", "")
      .replace("PLATFORM_FILE_STORAGE_S3_ENDPOINT=https://s3.example.test", "PLATFORM_FILE_STORAGE_S3_ENDPOINT=http://s3.example.test")
      .replace("PLATFORM_FILE_STORAGE_S3_SERVER_SIDE_ENCRYPTION=AES256\n", "");
    const { tempDir, filePath } = tempEnv(source);
    try {
      const result = runValidator(["--env-file", filePath, "--strict-secrets"]);

      assert.notEqual(result.status, 0, result.stdout);
      assert.match(result.stderr, /PLATFORM_FILE_MAX_UPLOAD_BYTES must be between 1 and 104857600/);
      assert.match(result.stderr, /PLATFORM_FILE_ALLOWED_MIME_TYPES is required/);
      assert.match(result.stderr, /PLATFORM_FILE_STORAGE_S3_ENDPOINT must use https/);
      assert.match(result.stderr, /PLATFORM_FILE_STORAGE_S3_SERVER_SIDE_ENCRYPTION is required/);
    } finally {
      fs.rmSync(tempDir, { recursive: true, force: true });
    }
  });

  it("rejects unsafe production transport security settings", () => {
    const source = validStrictEnv
      .replace("PLATFORM_PUBLIC_BASE_URL=https://platform.example.test", "PLATFORM_PUBLIC_BASE_URL=https://platform.example.test/")
      .replace("PLATFORM_TRUSTED_PROXIES=10.20.0.0/16", "PLATFORM_TRUSTED_PROXIES=0.0.0.0/0,999.999.999.999/24")
      .replace("PLATFORM_HTTP_MAX_BODY_BYTES=1048576", "PLATFORM_HTTP_MAX_BODY_BYTES=1073741824");
    const { tempDir, filePath } = tempEnv(source);
    try {
      const result = runValidator(["--env-file", filePath, "--strict-secrets"]);

      assert.notEqual(result.status, 0, result.stdout);
      assert.match(result.stderr, /PLATFORM_PUBLIC_BASE_URL must be an absolute HTTPS origin/);
      assert.match(result.stderr, /PLATFORM_TRUSTED_PROXIES must not trust all addresses/);
      assert.match(result.stderr, /PLATFORM_TRUSTED_PROXIES contains invalid IP or CIDR 999\.999\.999\.999\/24/);
      assert.match(result.stderr, /PLATFORM_HTTP_MAX_BODY_BYTES must be between 1 and 104857600/);
    } finally {
      fs.rmSync(tempDir, { recursive: true, force: true });
    }
  });

  it("rejects cumulative trust-all proxy policies", () => {
    const source = validStrictEnv.replace(
      "PLATFORM_TRUSTED_PROXIES=10.20.0.0/16",
      "PLATFORM_TRUSTED_PROXIES=0.0.0.0/1,128.0.0.0/1,::/1,8000::/1",
    );
    const { tempDir, filePath } = tempEnv(source);
    try {
      const result = runValidator(["--env-file", filePath, "--strict-secrets"]);

      assert.notEqual(result.status, 0, result.stdout);
      assert.match(result.stderr, /PLATFORM_TRUSTED_PROXIES must not cumulatively trust all IPv4 addresses/);
      assert.match(result.stderr, /PLATFORM_TRUSTED_PROXIES must not cumulatively trust all IPv6 addresses/);
    } finally {
      fs.rmSync(tempDir, { recursive: true, force: true });
    }
  });

  it("rejects an Admin proxy outside the trusted proxy policy", () => {
    const source = validStrictEnv.replace("PLATFORM_ADMIN_PROXY_IP=10.20.1.4", "PLATFORM_ADMIN_PROXY_IP=192.0.2.10");
    const { tempDir, filePath } = tempEnv(source);
    try {
      const result = runValidator(["--env-file", filePath, "--strict-secrets"]);

      assert.notEqual(result.status, 0, result.stdout);
      assert.match(result.stderr, /PLATFORM_ADMIN_PROXY_IP must be contained in PLATFORM_TRUSTED_PROXIES/);
    } finally {
      fs.rmSync(tempDir, { recursive: true, force: true });
    }
  });

  it("rejects unsafe app-phone protection settings", () => {
    const source = validStrictEnv
      .replace("system-admin", "system-admin,app-phone")
      .replace(
        "PLATFORM_DISABLE_DEMO_AUTH_PROVIDER=true",
        [
          "PLATFORM_DISABLE_DEMO_AUTH_PROVIDER=true",
          "PLATFORM_PHONE_HMAC_KEY=short-shared-key",
          "PLATFORM_PHONE_CODE_HMAC_KEY=short-shared-key",
          "PLATFORM_PHONE_VERIFICATION_PROVIDER=debug",
        ].join("\n"),
      );
    const { tempDir, filePath } = tempEnv(source);
    try {
      const result = runValidator(["--env-file", filePath, "--strict-secrets"]);

      assert.notEqual(result.status, 0, result.stdout);
      assert.match(result.stderr, /PLATFORM_PHONE_HMAC_KEY must be at least 32 bytes/);
      assert.match(result.stderr, /PLATFORM_PHONE_CODE_HMAC_KEY must be at least 32 bytes/);
      assert.match(result.stderr, /phone and code HMAC keys must be distinct/);
      assert.match(result.stderr, /PLATFORM_PHONE_VERIFICATION_PROVIDER must not be debug in production/);
    } finally {
      fs.rmSync(tempDir, { recursive: true, force: true });
    }
  });

  it("rejects app-phone without protection settings", () => {
    const source = validStrictEnv.replace("system-admin", "system-admin,app-phone");
    const { tempDir, filePath } = tempEnv(source);
    try {
      const result = runValidator(["--env-file", filePath, "--strict-secrets"]);

      assert.notEqual(result.status, 0, result.stdout);
      assert.match(result.stderr, /PLATFORM_PHONE_HMAC_KEY is required/);
      assert.match(result.stderr, /PLATFORM_PHONE_CODE_HMAC_KEY is required/);
      assert.match(result.stderr, /PLATFORM_PHONE_VERIFICATION_PROVIDER is required/);
    } finally {
      fs.rmSync(tempDir, { recursive: true, force: true });
    }
  });

  it("rejects an explicitly empty app-phone provider", () => {
    const source = validStrictEnv
      .replace("system-admin", "system-admin,app-phone")
      .replace(
        "PLATFORM_DISABLE_DEMO_AUTH_PROVIDER=true",
        [
          "PLATFORM_DISABLE_DEMO_AUTH_PROVIDER=true",
          "PLATFORM_PHONE_HMAC_KEY=phone-production-key-material-000001",
          "PLATFORM_PHONE_CODE_HMAC_KEY=code-production-key-material-000002",
          "PLATFORM_PHONE_VERIFICATION_PROVIDER=",
        ].join("\n"),
      );
    const { tempDir, filePath } = tempEnv(source);
    try {
      const result = runValidator(["--env-file", filePath, "--strict-secrets"]);

      assert.notEqual(result.status, 0, result.stdout);
      assert.match(result.stderr, /PLATFORM_PHONE_VERIFICATION_PROVIDER must not be empty/);
    } finally {
      fs.rmSync(tempDir, { recursive: true, force: true });
    }
  });

  it("rejects the unknown app-phone provider sentinel", () => {
    const source = validStrictEnv
      .replace("system-admin", "system-admin,app-phone")
      .replace(
        "PLATFORM_DISABLE_DEMO_AUTH_PROVIDER=true",
        [
          "PLATFORM_DISABLE_DEMO_AUTH_PROVIDER=true",
          "PLATFORM_PHONE_HMAC_KEY=phone-production-key-material-000001",
          "PLATFORM_PHONE_CODE_HMAC_KEY=code-production-key-material-000002",
          "PLATFORM_PHONE_VERIFICATION_PROVIDER=unknown",
        ].join("\n"),
      );
    const { tempDir, filePath } = tempEnv(source);
    try {
      const result = runValidator(["--env-file", filePath, "--strict-secrets"]);

      assert.notEqual(result.status, 0, result.stdout);
      assert.match(result.stderr, /PLATFORM_PHONE_VERIFICATION_PROVIDER must identify a configured provider/);
    } finally {
      fs.rmSync(tempDir, { recursive: true, force: true });
    }
  });

  it("rejects a placeholder phone HMAC key even when it meets the length floor", () => {
    const source = validStrictEnv
      .replace("system-admin", "system-admin,app-phone")
      .replace(
        "PLATFORM_DISABLE_DEMO_AUTH_PROVIDER=true",
        [
          "PLATFORM_DISABLE_DEMO_AUTH_PROVIDER=true",
          "PLATFORM_PHONE_HMAC_KEY=replace-with-phone-key-material-000001",
          "PLATFORM_PHONE_CODE_HMAC_KEY=code-production-key-material-000002",
          "PLATFORM_PHONE_VERIFICATION_PROVIDER=sms-vendor",
        ].join("\n"),
      );
    const { tempDir, filePath } = tempEnv(source);
    try {
      const result = runValidator(["--env-file", filePath, "--strict-secrets"]);

      assert.notEqual(result.status, 0, result.stdout);
      assert.match(result.stderr, /PLATFORM_PHONE_HMAC_KEY must not be a placeholder/);
    } finally {
      fs.rmSync(tempDir, { recursive: true, force: true });
    }
  });

  it("rejects a placeholder code HMAC key independently", () => {
    const source = validStrictEnv
      .replace("system-admin", "system-admin,app-phone")
      .replace(
        "PLATFORM_DISABLE_DEMO_AUTH_PROVIDER=true",
        [
          "PLATFORM_DISABLE_DEMO_AUTH_PROVIDER=true",
          "PLATFORM_PHONE_HMAC_KEY=phone-production-key-material-000001",
          "PLATFORM_PHONE_CODE_HMAC_KEY=replace-with-code-key-material-000002",
          "PLATFORM_PHONE_VERIFICATION_PROVIDER=sms-vendor",
        ].join("\n"),
      );
    const { tempDir, filePath } = tempEnv(source);
    try {
      const result = runValidator(["--env-file", filePath, "--strict-secrets"]);

      assert.notEqual(result.status, 0, result.stdout);
      assert.match(result.stderr, /PLATFORM_PHONE_CODE_HMAC_KEY must not be a placeholder/);
    } finally {
      fs.rmSync(tempDir, { recursive: true, force: true });
    }
  });
});
