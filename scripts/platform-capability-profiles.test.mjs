import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");

function runValidator(args = []) {
  return spawnSync(process.execPath, ["scripts/validate-platform-capability-profiles.mjs", ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

function tempJSON(name, value) {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-capability-profiles-"));
  const filePath = path.join(tempDir, name);
  fs.writeFileSync(filePath, `${JSON.stringify(value, null, 2)}\n`);
  return filePath;
}

function readProfiles() {
  return JSON.parse(fs.readFileSync(path.join(repoRoot, "resources/platform-capability-profiles.json"), "utf8"));
}

describe("validate-platform-capability-profiles", () => {
  it("accepts current capability profiles", () => {
    const result = runValidator();

    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Validated \d+ platform capability profiles/);
  });

  it("rejects runtime default profile drift from config defaults", () => {
    const profiles = readProfiles();
    const defaultProfile = profiles.profiles.find((profile) => profile.id === profiles.runtimeDefault);
    defaultProfile.capabilities = defaultProfile.capabilities.filter((capability) => capability !== "parameter");
    const profilesPath = tempJSON("profiles.json", profiles);

    const result = runValidator(["--profiles", profilesPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /profile platform-default capabilities must match config defaultCapabilities/);
  });

  it("rejects business capabilities in non-business default profiles", () => {
    const profiles = readProfiles();
    const defaultProfile = profiles.profiles.find((profile) => profile.id === profiles.runtimeDefault);
    defaultProfile.capabilities.push("external-business-capability");
    const profilesPath = tempJSON("profiles.json", profiles);

    const result = runValidator(["--profiles", profilesPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /profile platform-default is default and must not include business capabilities/);
  });

  it("rejects business profiles without declared additional manifests", () => {
    const profiles = readProfiles();
    profiles.profiles.push({
      id: "external-business-test",
      label: { zh: "外部业务测试", en: "External Business Test" },
      purpose: "Invalid profile used by the test.",
      business: true,
      capabilities: ["external-business-capability"],
    });
    const profilesPath = tempJSON("profiles.json", profiles);

    const result = runValidator(["--profiles", profilesPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /profile external-business-test includes business capabilities and must declare requiresAdditionalManifests/);
  });

  it("rejects additional manifest paths outside the repository", () => {
    const profiles = readProfiles();
    profiles.profiles.push({
      id: "external-business-test",
      label: { zh: "外部业务测试", en: "External Business Test" },
      purpose: "Invalid profile used by the test.",
      business: true,
      capabilities: ["external-business-capability"],
      requiresAdditionalManifests: ["../zshenmez"],
    });
    const profilesPath = tempJSON("profiles.json", profiles);

    const result = runValidator(["--profiles", profilesPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /profile external-business-test requiresAdditionalManifests path is missing or unsafe: \.\.\/zshenmez/);
  });

  it("rejects additional manifest directories without Go manifests", () => {
    const profiles = readProfiles();
    profiles.profiles.push({
      id: "external-business-test",
      label: { zh: "外部业务测试", en: "External Business Test" },
      purpose: "Invalid profile used by the test.",
      business: true,
      capabilities: ["external-business-capability"],
      requiresAdditionalManifests: ["docs"],
    });
    const profilesPath = tempJSON("profiles.json", profiles);

    const result = runValidator(["--profiles", profilesPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /profile external-business-test requiresAdditionalManifests docs must contain at least one Go manifest file/);
  });

  it("rejects profiles whose capability dependencies cannot resolve", () => {
    const profiles = readProfiles();
    const minimalProfile = profiles.profiles.find((profile) => profile.id === "minimal-admin");
    minimalProfile.capabilities = ["session"];
    const profilesPath = tempJSON("profiles.json", profiles);

    const result = runValidator(["--profiles", profilesPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /profile minimal-admin failed capability audit/);
  });

  it("keeps policy review as an optional enterprise governance profile", () => {
    const profiles = readProfiles();
    const defaultProfile = profiles.profiles.find((profile) => profile.id === profiles.runtimeDefault);
    const minimalProfile = profiles.profiles.find((profile) => profile.id === "minimal-admin");
    const enterpriseProfile = profiles.profiles.find((profile) => profile.id === "enterprise-governance");

    assert.ok(defaultProfile.mustExcludeCapabilities.includes("policy-review"));
    assert.ok(minimalProfile.mustExcludeCapabilities.includes("policy-review"));
    assert.ok(enterpriseProfile, "enterprise-governance profile is required");
    assert.ok(enterpriseProfile.capabilities.includes("policy-review"));
    assert.ok(enterpriseProfile.mustIncludeResources.includes("policy-reviews"));
    assert.ok(!enterpriseProfile.business, "policy-review must remain platform governance, not business");
  });

  it("keeps personnel as an optional platform profile outside default contracts", () => {
    const profiles = readProfiles();
    const defaultProfile = profiles.profiles.find((profile) => profile.id === profiles.runtimeDefault);
    const minimalProfile = profiles.profiles.find((profile) => profile.id === "minimal-admin");
    const appReadyProfile = profiles.profiles.find((profile) => profile.id === "platform-app-ready");
    const personnelProfile = profiles.profiles.find((profile) => profile.id === "platform-personnel-ready");

    for (const profile of [defaultProfile, minimalProfile, appReadyProfile]) {
      assert.ok(profile.mustExcludeCapabilities.includes("personnel"), `${profile.id} must exclude personnel`);
      assert.ok(!profile.capabilities.includes("personnel"), `${profile.id} must not enable personnel`);
    }
    assert.ok(personnelProfile, "platform-personnel-ready profile is required");
    assert.ok(personnelProfile.capabilities.includes("personnel"));
    assert.ok(personnelProfile.mustIncludeResources.includes("personnel-profiles"));
    assert.ok(personnelProfile.mustIncludeResources.includes("positions"));
    assert.ok(personnelProfile.mustIncludeResources.includes("position-assignments"));
    assert.ok(!personnelProfile.business, "personnel is a reusable platform extension, not a business capability");
  });

  it("rejects foundation profiles that stop declaring organization, role-group or area governance resources", () => {
    const profiles = readProfiles();
    const minimalProfile = profiles.profiles.find((profile) => profile.id === "minimal-admin");
    minimalProfile.mustIncludeResources = minimalProfile.mustIncludeResources.filter((resource) => resource !== "org-units");
    const profilesPath = tempJSON("profiles.json", profiles);

    const result = runValidator(["--profiles", profilesPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /profile minimal-admin must declare governance resource org-units in mustIncludeResources/);
  });

  it("rejects personnel-ready profiles that omit shared account, organization or area resources", () => {
    const profiles = readProfiles();
    const personnelProfile = profiles.profiles.find((profile) => profile.id === "platform-personnel-ready");
    personnelProfile.mustIncludeResources = personnelProfile.mustIncludeResources.filter((resource) => resource !== "area-codes");
    const profilesPath = tempJSON("profiles.json", profiles);

    const result = runValidator(["--profiles", profilesPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /profile platform-personnel-ready must declare personnel governance resource area-codes in mustIncludeResources/);
  });

  it("keeps notification as an optional platform profile outside default contracts", () => {
    const profiles = readProfiles();
    const defaultProfile = profiles.profiles.find((profile) => profile.id === profiles.runtimeDefault);
    const minimalProfile = profiles.profiles.find((profile) => profile.id === "minimal-admin");
    const appReadyProfile = profiles.profiles.find((profile) => profile.id === "platform-app-ready");
    const personnelProfile = profiles.profiles.find((profile) => profile.id === "platform-personnel-ready");
    const notificationProfile = profiles.profiles.find((profile) => profile.id === "platform-notification-ready");

    for (const profile of [defaultProfile, minimalProfile, appReadyProfile, personnelProfile]) {
      assert.ok(profile.mustExcludeCapabilities.includes("notification"), `${profile.id} must exclude notification`);
      assert.ok(!profile.capabilities.includes("notification"), `${profile.id} must not enable notification`);
    }
    assert.ok(notificationProfile, "platform-notification-ready profile is required");
    assert.ok(notificationProfile.capabilities.includes("notification"));
    assert.ok(notificationProfile.mustIncludeResources.includes("notification-templates"));
    assert.ok(notificationProfile.mustIncludeResources.includes("notifications"));
    assert.ok(notificationProfile.mustIncludeResources.includes("notification-deliveries"));
    assert.ok(!notificationProfile.business, "notification is a reusable platform extension, not a business capability");
  });

  it("keeps job as an optional platform profile outside default contracts", () => {
    const profiles = readProfiles();
    const defaultProfile = profiles.profiles.find((profile) => profile.id === profiles.runtimeDefault);
    const minimalProfile = profiles.profiles.find((profile) => profile.id === "minimal-admin");
    const appReadyProfile = profiles.profiles.find((profile) => profile.id === "platform-app-ready");
    const personnelProfile = profiles.profiles.find((profile) => profile.id === "platform-personnel-ready");
    const notificationProfile = profiles.profiles.find((profile) => profile.id === "platform-notification-ready");
    const jobProfile = profiles.profiles.find((profile) => profile.id === "platform-job-ready");

    for (const profile of [defaultProfile, minimalProfile, appReadyProfile, personnelProfile, notificationProfile]) {
      assert.ok(profile.mustExcludeCapabilities.includes("job"), `${profile.id} must exclude job`);
      assert.ok(!profile.capabilities.includes("job"), `${profile.id} must not enable job`);
    }
    assert.ok(jobProfile, "platform-job-ready profile is required");
    assert.ok(jobProfile.capabilities.includes("job"));
    assert.ok(jobProfile.mustIncludeResources.includes("job-definitions"));
    assert.ok(jobProfile.mustIncludeResources.includes("job-runs"));
    assert.ok(jobProfile.mustIncludeResources.includes("job-run-attempts"));
    assert.ok(!jobProfile.business, "job is a reusable platform extension, not a business capability");
  });
});
