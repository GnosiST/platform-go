import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");

function runValidator(args = []) {
  return spawnSync(process.execPath, ["scripts/validate-platform-capability-contracts.mjs", ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

function tempJSON(name, value) {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-capability-contracts-"));
  const filePath = path.join(tempDir, name);
  fs.writeFileSync(filePath, `${JSON.stringify(value, null, 2)}\n`);
  return filePath;
}

function readJSON(relativePath) {
  return JSON.parse(fs.readFileSync(path.join(repoRoot, relativePath), "utf8"));
}

describe("validate-platform-capability-contracts", () => {
  it("accepts the current platform capability contracts", () => {
    const result = runValidator();

    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Validated \d+ platform capability contracts/);
  });

  it("rejects profile capabilities that are not classified", () => {
    const contracts = readJSON("resources/platform-capability-contracts.json");
    contracts.capabilities = contracts.capabilities.filter((capability) => capability.id !== "notification");
    const contractsPath = tempJSON("platform-capability-contracts.json", contracts);

    const result = runValidator(["--contracts", contractsPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /capability notification appears in profiles but is missing from platform capability contracts/);
  });

  it("rejects default-enabled capabilities removed from the runtime default", () => {
    const contracts = readJSON("resources/platform-capability-contracts.json");
    const profiles = readJSON("resources/platform-capability-profiles.json");
    profiles.profiles.find((profile) => profile.id === profiles.runtimeDefault).capabilities = profiles.profiles
      .find((profile) => profile.id === profiles.runtimeDefault)
      .capabilities.filter((capability) => capability !== "parameter");
    const contractsPath = tempJSON("platform-capability-contracts.json", contracts);
    const profilesPath = tempJSON("platform-capability-profiles.json", profiles);

    const result = runValidator(["--contracts", contractsPath, "--profiles", profilesPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /capability contract parameter is default-enabled and must be in config defaults and runtime default profile/);
  });

  it("rejects profile-only capabilities leaking into the default profile", () => {
    const profiles = readJSON("resources/platform-capability-profiles.json");
    profiles.profiles.find((profile) => profile.id === profiles.runtimeDefault).capabilities.push("job");
    const profilesPath = tempJSON("platform-capability-profiles.json", profiles);

    const result = runValidator(["--profiles", profilesPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /capability contract job is profile-only and must not be enabled by default/);
  });

  it("classifies admin OIDC as a production profile-only platform capability", () => {
    const contracts = readJSON("resources/platform-capability-contracts.json");
    const adminOIDC = contracts.capabilities.find((capability) => capability.id === "admin-oidc");

    assert.ok(adminOIDC, "admin-oidc contract is required");
    assert.equal(adminOIDC.classification, "optional-platform");
    assert.equal(adminOIDC.profilePolicy, "profile-only");
    assert.ok(adminOIDC.includedInProfiles.includes("production-admin-oidc-ready"));
    assert.deepEqual(adminOIDC.authProviders, ["oidc"]);
    assert.ok(adminOIDC.adminResources.includes("admin-identities"));
  });

  it("classifies parameter as the productized system settings center", () => {
    const contracts = readJSON("resources/platform-capability-contracts.json");
    const parameter = contracts.capabilities.find((capability) => capability.id === "parameter");

    assert.ok(parameter, "parameter contract is required");
    assert.equal(parameter.classification, "foundation-default");
    assert.equal(parameter.profilePolicy, "default-enabled");
    assert.ok(parameter.adminResources.includes("settings"));
    assert.ok(parameter.adminResources.includes("parameters"));
    assert.ok(parameter.adminResources.includes("branding"));
    assert.match(parameter.boundary, /system settings center/);
    assert.match(parameter.boundary, /\/settings workbench/);
    assert.match(parameter.boundary, /enabled capability configuration resources/);
    assert.match(parameter.boundary, /topbar settings remain interface preferences/);
  });

  it("classifies notification as a productized message-center capability", () => {
    const contracts = readJSON("resources/platform-capability-contracts.json");
    const notification = contracts.capabilities.find((capability) => capability.id === "notification");

    assert.ok(notification, "notification contract is required");
    assert.equal(notification.classification, "optional-platform");
    assert.equal(notification.profilePolicy, "profile-only");
    assert.ok(notification.includedInProfiles.includes("platform-notification-ready"));
    for (const resource of [
      "message-center",
      "notification-channels",
      "notification-providers",
      "notification-send-policies",
      "notification-templates",
      "notifications",
      "notification-deliveries",
    ]) {
      assert.ok(notification.adminResources.includes(resource), `notification contract missing ${resource}`);
    }
    assert.match(notification.boundary, /Aliyun/);
    assert.match(notification.boundary, /generic SMTP/);
    assert.match(notification.boundary, /WeChat channels/);
    assert.match(notification.boundary, /encrypted, masked or omitted/);
    assert.equal(notification.productization.workbenchRoute, "/message-center");
    assert.equal(notification.productization.settingsCenterRoute, "/settings");
    assert.equal(notification.productization.settingsAggregation, "dynamic-enabled-capability-config");
    assert.deepEqual(notification.productization.channels, ["in_app", "sms", "email", "wechat_official", "wechat_miniapp"]);
    assert.deepEqual(notification.productization.settingsConfigResources, [
      "notification-channels",
      "notification-providers",
      "notification-send-policies",
      "notification-templates",
    ]);
    const providersByChannel = new Map(notification.productization.providerFamilies.map((family) => [family.channel, family.providers]));
    assert.deepEqual(providersByChannel.get("sms"), ["aliyun", "tencent", "mock-local"]);
    assert.deepEqual(providersByChannel.get("email"), ["smtp"]);
    assert.deepEqual(providersByChannel.get("wechat_official"), ["wechat-official"]);
    assert.deepEqual(providersByChannel.get("wechat_miniapp"), ["wechat-miniapp"]);
  });

  it("rejects notification productization that regresses to SMS-only or drops common provider configuration", () => {
    const contracts = readJSON("resources/platform-capability-contracts.json");
    const notification = contracts.capabilities.find((capability) => capability.id === "notification");
    notification.productization.channels = ["sms"];
    notification.productization.providerFamilies = notification.productization.providerFamilies.filter((family) => family.channel === "sms");
    notification.productization.providerFamilies[0].providers = ["mock-local"];
    const contractsPath = tempJSON("platform-capability-contracts.json", contracts);

    const result = runValidator(["--contracts", contractsPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /notification\.productization\.channels must include email/);
    assert.match(result.stderr, /notification\.productization\.channels must include wechat_official/);
    assert.match(result.stderr, /notification\.productization\.providerFamilies\[sms\]\.providers must include aliyun/);
    assert.match(result.stderr, /notification\.productization\.providerFamilies must include channel email/);
    assert.match(result.stderr, /notification\.productization\.providerFamilies must include channel wechat_miniapp/);
  });

  it("rejects declared contract surface drift from audited manifests", () => {
    const contracts = readJSON("resources/platform-capability-contracts.json");
    contracts.capabilities.find((capability) => capability.id === "rbac").adminResources =
      contracts.capabilities.find((capability) => capability.id === "rbac").adminResources.filter((resource) => resource !== "roles");
    const contractsPath = tempJSON("platform-capability-contracts.json", contracts);

    const result = runValidator(["--contracts", contractsPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /capability contract rbac\.adminResources must match capability audit output/);
  });

  it("rejects external business capability inside non-business profiles", () => {
    const profiles = readJSON("resources/platform-capability-profiles.json");
    profiles.profiles.find((profile) => profile.id === "minimal-admin").capabilities.push("external-business-capability");
    const profilesPath = tempJSON("platform-capability-profiles.json", profiles);

    const result = runValidator(["--profiles", profilesPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /capability contract external-business-capability must not appear in non-business profile minimal-admin/);
  });
});
