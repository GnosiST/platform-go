import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");

function runValidator(args = []) {
  return spawnSync(process.execPath, ["scripts/validate-platform-form-schema-layout-slots.mjs", ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

function readJSON(relativePath) {
  return JSON.parse(fs.readFileSync(path.join(repoRoot, relativePath), "utf8"));
}

function tempJSON(name, value) {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-form-schema-layout-slots-"));
  const filePath = path.join(tempDir, name);
  fs.writeFileSync(filePath, `${JSON.stringify(value, null, 2)}\n`);
  return filePath;
}

describe("validate-platform-form-schema-layout-slots", () => {
  it("accepts the current form layout and slot contract", () => {
    const result = runValidator();

    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Validated form schema layout and slot contract/);
  });

  it("rejects uncontrolled runtime slots, source writing or component injection", () => {
    const contract = readJSON("resources/platform-form-schema-layout-slots.json");
    contract.promotionState.runtimeSlots = "enabled";
    contract.promotionState.visualImplementation = "requires-product-design";
    contract.promotionState.sourceWriting = "enabled";
    contract.promotionState.arbitraryComponentInjection = "allowed";
    const contractPath = tempJSON("platform-form-schema-layout-slots.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /promotionState\.runtimeSlots must be controlled/);
    assert.match(result.stderr, /promotionState\.visualImplementation must be implemented/);
    assert.match(result.stderr, /promotionState\.sourceWriting must stay disabled/);
    assert.match(result.stderr, /promotionState\.arbitraryComponentInjection must stay forbidden/);
  });

  it("rejects layout presets that skip product-design promotion", () => {
    const contract = readJSON("resources/platform-form-schema-layout-slots.json");
    const twoColumn = contract.allowedLayoutPresets.find((preset) => preset.id === "two-column-density");
    twoColumn.status = "requires-product-design";
    const sidePreview = contract.allowedLayoutPresets.find((preset) => preset.id === "side-detail-preview");
    sidePreview.status = "requires-product-design";
    const singleColumn = contract.allowedLayoutPresets.find((preset) => preset.id === "single-column");
    singleColumn.default = false;
    const contractPath = tempJSON("platform-form-schema-layout-slots.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /allowedLayoutPresets\.single-column must remain the default layout/);
    assert.match(result.stderr, /allowedLayoutPresets\.two-column-density\.status must stay implemented/);
    assert.match(result.stderr, /allowedLayoutPresets\.side-detail-preview\.status must stay implemented/);
  });

  it("rejects slot regions that regress without i18n or ownership", () => {
    const contract = readJSON("resources/platform-form-schema-layout-slots.json");
    const headerSlot = contract.allowedSlotRegions.find((slot) => slot.id === "form.header");
    headerSlot.status = "deferred";
    headerSlot.owner = "";
    headerSlot.requiredCapabilities = [];
    const contractPath = tempJSON("platform-form-schema-layout-slots.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /allowedSlotRegions\.form\.header\.status must stay implemented/);
    assert.match(result.stderr, /allowedSlotRegions\.form\.header must declare owner/);
    assert.match(result.stderr, /allowedSlotRegions\.form\.header must require i18n/);
  });

  it("rejects contracts without the runtime slot descriptor shape", () => {
    const contract = readJSON("resources/platform-form-schema-layout-slots.json");
    delete contract.runtimeSlotDescriptor;
    const contractPath = tempJSON("platform-form-schema-layout-slots.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /runtimeSlotDescriptor must declare slotId, region, label, description, permission, visibleWhen, targetSection, targetField, dataBinding, variant, order/);
  });

  it("rejects contracts without the side preview slot region", () => {
    const contract = readJSON("resources/platform-form-schema-layout-slots.json");
    contract.allowedSlotRegions = contract.allowedSlotRegions.filter((slot) => slot.id !== "side.preview");
    const contractPath = tempJSON("platform-form-schema-layout-slots.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /allowedSlotRegions must include side\.preview/);
  });

  it("rejects contracts that drop forbidden patterns or promotion gates", () => {
    const contract = readJSON("resources/platform-form-schema-layout-slots.json");
    contract.forbiddenPatterns = ["raw-script-slots"];
    contract.requiredPromotionGates = ["validate-admin-i18n"];
    const contractPath = tempJSON("platform-form-schema-layout-slots.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /forbiddenPatterns must include/);
    assert.match(result.stderr, /requiredPromotionGates must include/);
  });

  it("rejects contracts that no longer map to current form source evidence", () => {
    const contract = readJSON("resources/platform-form-schema-layout-slots.json");
    contract.requiredSourceEvidence[0].contains = "missing form group implementation";
    const contractPath = tempJSON("platform-form-schema-layout-slots.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /requiredSourceEvidence internal\/platform\/adminresource\/schema\.go is missing snippet missing form group implementation/);
  });

  it("rejects missing browser evidence for dense form promotion", () => {
    const contract = readJSON("resources/platform-form-schema-layout-slots.json");
    contract.browserEvidence = [
      {
        path: "tmp/product-design/form-schema-layout-20260707/missing.png",
        viewport: "1440x1024",
        assertions: [],
      },
    ];
    const contractPath = tempJSON("platform-form-schema-layout-slots.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /browserEvidence must include desktop and mobile screenshots/);
    assert.match(result.stderr, /browserEvidence path is missing or unsafe/);
    assert.match(result.stderr, /browserEvidence .* must declare assertions/);
    assert.match(result.stderr, /browserEvidence must include 390x844/);
  });

  it("accepts portable external browser evidence URIs", () => {
    const contract = readJSON("resources/platform-form-schema-layout-slots.json");
    contract.browserEvidence[0].path = "external-review-artifacts://platform-go/form-schema-layout/2026-07-07/desktop.png";
    contract.browserEvidence[1].path = "external-review-artifacts://platform-go/form-schema-layout/2026-07-07/mobile.png";
    const contractPath = tempJSON("platform-form-schema-layout-slots.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.equal(result.status, 0, result.stderr);
  });

  it("rejects contracts without the shared platform form component and controlled source-level slots", () => {
    const contract = readJSON("resources/platform-form-schema-layout-slots.json");
    contract.supportedToday.frontendFunctions = contract.supportedToday.frontendFunctions.filter((item) => item !== "PlatformResourceForm");
    contract.promotionState.sourceLevelReactSlots = "absent";
    const contractPath = tempJSON("platform-form-schema-layout-slots.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /supportedToday\.frontendFunctions must include PlatformResourceForm/);
    assert.match(result.stderr, /normalizeFormLayoutPreset/);
    assert.match(result.stderr, /promotionState\.sourceLevelReactSlots must be controlled/);
  });
});
