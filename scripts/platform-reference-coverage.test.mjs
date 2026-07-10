import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");

function runValidator(args = []) {
  return spawnSync(process.execPath, ["scripts/validate-platform-reference-coverage.mjs", ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

function tempJSON(name, value) {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-reference-coverage-"));
  const filePath = path.join(tempDir, name);
  fs.writeFileSync(filePath, `${JSON.stringify(value, null, 2)}\n`);
  return filePath;
}

function readJSON(relativePath) {
  return JSON.parse(fs.readFileSync(path.join(repoRoot, relativePath), "utf8"));
}

describe("validate-platform-reference-coverage", () => {
  it("accepts current platform coverage", () => {
    const result = runValidator();

    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Validated \d+ platform reference coverage areas/);
  });

  it("rejects missing platform resources", () => {
    const coverage = readJSON("resources/platform-reference-coverage.json");
    coverage.foundation[0].platformResources = ["missingOverview"];
    const coveragePath = tempJSON("coverage.json", coverage);

    const result = runValidator(["--coverage", coveragePath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /missing platform admin resource missingOverview/);
  });

  it("rejects missing required foundation coverage areas", () => {
    const coverage = readJSON("resources/platform-reference-coverage.json");
    coverage.foundation = coverage.foundation.filter((area) => area.area !== "demo-data");
    const coveragePath = tempJSON("coverage.json", coverage);

    const result = runValidator(["--coverage", coveragePath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /missing required foundation coverage area demo-data/);
  });

  it("rejects required foundation areas without capability mappings", () => {
    const coverage = readJSON("resources/platform-reference-coverage.json");
    const fileStorage = coverage.foundation.find((area) => area.area === "file-storage");
    fileStorage.capabilities = [];
    const coveragePath = tempJSON("coverage.json", coverage);

    const result = runValidator(["--coverage", coveragePath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /foundation area file-storage must declare at least one platform capability/);
  });

  it("rejects file storage foundation coverage without app upload and content routes", () => {
    const coverage = readJSON("resources/platform-reference-coverage.json");
    const fileStorage = coverage.foundation.find((area) => area.area === "file-storage");
    delete fileStorage.appRoutes;
    const coveragePath = tempJSON("coverage.json", coverage);

    const result = runValidator(["--coverage", coveragePath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /foundation area file-storage must declare app route POST \/api\/app\/files/);
    assert.match(result.stderr, /foundation area file-storage must declare app route GET \/api\/app\/files\/:id\/content/);
  });

  it("rejects reference business resources in the default platform contract", () => {
    const adminContract = readJSON("resources/generated/admin-resource-contract.json");
    adminContract.resources.push({
      name: "tasks",
      code: "tasks",
      permissions: { read: "admin:task:read" },
    });
    const adminContractPath = tempJSON("admin-resource-contract.json", adminContract);

    const result = runValidator(["--admin-contract", adminContractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /resource tasks must stay out of the default platform contract unless external-business-capability is enabled/);
  });

  it("rejects personnel extension resources in the default platform contract", () => {
    const adminContract = readJSON("resources/generated/admin-resource-contract.json");
    adminContract.resources.push({
      name: "employees",
      code: "employees",
      permissions: { read: "admin:employee:read" },
    });
    const adminContractPath = tempJSON("admin-resource-contract.json", adminContract);

    const result = runValidator(["--admin-contract", adminContractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /extension boundary personnel-and-positions resource employees must stay out of the default platform contract unless personnel is enabled/);
  });

  it("rejects extension boundaries without explicit capability ownership", () => {
    const coverage = readJSON("resources/platform-reference-coverage.json");
    const personnelBoundary = coverage.extensionBoundary.find((boundary) => boundary.area === "personnel-and-positions");
    personnelBoundary.expectedCapability = "";
    const coveragePath = tempJSON("coverage.json", coverage);

    const result = runValidator(["--coverage", coveragePath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /extension boundary personnel-and-positions must declare expectedCapability/);
  });

  it("rejects business boundaries whose capability is missing from capability profiles", () => {
    const coverage = readJSON("resources/platform-reference-coverage.json");
    coverage.businessBoundary[0].expectedCapability = "missing-business";
    const coveragePath = tempJSON("coverage.json", coverage);

    const result = runValidator(["--coverage", coveragePath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /business boundary business-access expectedCapability missing-business must be declared in platform capability profile businessCapabilities/);
  });

  it("rejects reference business boundaries owned by concrete business capabilities", () => {
    const coverage = readJSON("resources/platform-reference-coverage.json");
    coverage.businessBoundary[0].expectedCapability = "dispatch-business";
    const coveragePath = tempJSON("coverage.json", coverage);
    const profiles = readJSON("resources/platform-capability-profiles.json");
    profiles.businessCapabilities.push("dispatch-business");
    const profilesPath = tempJSON("profiles.json", profiles);

    const result = runValidator(["--coverage", coveragePath, "--profiles", profilesPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /business boundary business-access must stay owned by external-business-capability outside platform-go/);
  });

  it("rejects reference business parity owned by a different capability than its boundary", () => {
    const coverage = readJSON("resources/platform-reference-coverage.json");
    const taskParity = coverage.referenceResourceParity.find((item) => item.referenceResource === "tasks");
    taskParity.expectedCapability = "dispatch-business";
    const coveragePath = tempJSON("coverage.json", coverage);

    const result = runValidator(["--coverage", coveragePath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /reference resource parity tasks expectedCapability dispatch-business must match business boundary external-business-capability/);
  });

  it("rejects profile business capabilities missing from reference coverage", () => {
    const profiles = readJSON("resources/platform-capability-profiles.json");
    profiles.businessCapabilities.push("orphan-business");
    const profilesPath = tempJSON("profiles.json", profiles);

    const result = runValidator(["--profiles", profilesPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /business capability orphan-business from platform capability profiles is missing from reference businessBoundary/);
  });

  it("rejects external business capability resources missing from the reference coverage boundary", () => {
    const coverage = readJSON("resources/platform-reference-coverage.json");
    const supportBoundary = coverage.businessBoundary.find((boundary) => boundary.area === "business-support");
    supportBoundary.referenceResources = [];
    const coveragePath = tempJSON("coverage.json", coverage);
    const audit = readJSON("resources/generated/platform-capability-audit.json");
    audit.capabilities.push({
      id: "external-business-capability",
      name: "External Business Capability",
      version: "0.1.0",
      adminResources: ["support-tickets"],
    });
    const auditPath = tempJSON("external-business-audit.json", audit);

    const result = runValidator(["--coverage", coveragePath, "--audit", auditPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /coverage for external-business-capability is missing capability resource support-tickets/);
  });

  it("rejects business capability app routes missing from the reference coverage boundary", () => {
    const coverage = readJSON("resources/platform-reference-coverage.json");
    for (const boundary of coverage.businessBoundary) {
      delete boundary.appRoutes;
    }
    const coveragePath = tempJSON("coverage.json", coverage);
    const audit = readJSON("resources/generated/platform-capability-audit.json");
    audit.capabilities.push({
      id: "external-business-capability",
      name: "External Business Capability",
      version: "0.1.0",
      adminResources: [
        "role-applications",
        "public-profiles",
        "portfolio-works",
        "favorites",
        "tasks",
        "transfer-applications",
        "transfer-edges",
        "task-check-ins",
        "task-completion-confirmations",
        "support-tickets",
      ],
      appRoutes: ["POST /api/app/zshenmez/tasks/:taskCode/transfer-applications"],
    });
    const auditPath = tempJSON("audit.json", audit);

    const result = runValidator(["--coverage", coveragePath, "--audit", auditPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /coverage for external-business-capability is missing capability app route POST \/api\/app\/zshenmez\/tasks\/:taskCode\/transfer-applications/);
  });

  it("rejects capability app routes that are missing runtime handlers", () => {
    const audit = readJSON("resources/generated/platform-capability-audit.json");
    audit.missingAppRouteHandlers = ["POST /api/app/zshenmez/tasks"];
    const auditPath = tempJSON("audit.json", audit);

    const result = runValidator(["--audit", auditPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /missing runtime app route handler POST \/api\/app\/zshenmez\/tasks/);
  });

  it("rejects reference resources missing from the parity table", () => {
    const coverage = readJSON("resources/platform-reference-coverage.json");
    coverage.referenceResourceParity = coverage.referenceResourceParity.filter((item) => item.referenceResource !== "files");
    const coveragePath = tempJSON("coverage.json", coverage);

    const result = runValidator(["--coverage", coveragePath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /reference resource parity files/);
  });

  it("rejects business parity resources that leak into the default platform contract", () => {
    const adminContract = readJSON("resources/generated/admin-resource-contract.json");
    adminContract.resources.push({
      name: "supportTickets",
      code: "support-tickets",
      permissions: { read: "admin:support-ticket:read" },
    });
    const adminContractPath = tempJSON("admin-resource-contract.json", adminContract);

    const result = runValidator(["--admin-contract", adminContractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /reference resource parity support-tickets must stay out of the default platform admin contract/);
  });

  it("rejects reference manifest resources that have no parity classification", () => {
    const manifest = {
      resources: [
        ...readJSON("resources/platform-reference-coverage.json").referenceResourceParity.map((item) => ({
          code: item.referenceResource,
        })),
        { code: "storage-settings" },
      ],
    };
    const manifestPath = tempJSON("zshenmez-admin-resources.json", manifest);

    const result = runValidator(["--reference-manifest", manifestPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /reference manifest resource storagesettings is missing from referenceResourceParity/);
  });

  it("uses the reference discovery manifest by default to reject unclassified reference drift", () => {
    const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-reference-drift-"));
    try {
      fs.mkdirSync(path.join(tempDir, "resources"), { recursive: true });
      const manifest = {
        resources: [
          ...readJSON("resources/platform-reference-coverage.json").referenceResourceParity.map((item) => ({
            code: item.referenceResource,
          })),
          { code: "new-common-setting" },
        ],
      };
      fs.writeFileSync(path.join(tempDir, "resources", "admin-resources.json"), `${JSON.stringify(manifest, null, 2)}\n`);

      const discovery = readJSON("resources/platform-reference-discovery.json");
      discovery.reference.root = tempDir;
      const discoveryPath = tempJSON("platform-reference-discovery.json", discovery);

      const result = runValidator(["--reference-discovery", discoveryPath]);

      assert.notEqual(result.status, 0, result.stdout);
      assert.match(result.stderr, /reference manifest resource newcommonsetting is missing from referenceResourceParity/);
    } finally {
      fs.rmSync(tempDir, { recursive: true, force: true });
    }
  });

  it("rejects required non-resource reference capabilities that are not explicitly classified", () => {
    const coverage = readJSON("resources/platform-reference-coverage.json");
    delete coverage.nonResourceParity;
    const coveragePath = tempJSON("coverage.json", coverage);

    const result = runValidator(["--coverage", coveragePath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /reference non-resource capability storage-settings is missing from nonResourceParity/);
  });

  it("rejects app phone reference coverage that is not owned by the optional app-phone profile", () => {
    const coverage = readJSON("resources/platform-reference-coverage.json");
    coverage.nonResourceParity = currentNonResourceParityFixture();
    const appPhone = coverage.nonResourceParity.find((item) => item.referenceCapability === "app-phone-binding");
    appPhone.expectedCapability = "identity";
    appPhone.expectedProfile = "platform-default";
    const coveragePath = tempJSON("coverage.json", coverage);

    const result = runValidator(["--coverage", coveragePath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /reference non-resource capability app-phone-binding must be owned by optional capability app-phone/);
    assert.match(result.stderr, /reference non-resource capability app-phone-binding must be enabled through profile platform-app-ready/);
  });

  it("rejects detailed address coverage that leaks into the default platform foundation", () => {
    const coverage = readJSON("resources/platform-reference-coverage.json");
    coverage.nonResourceParity = currentNonResourceParityFixture();
    const userAddresses = coverage.nonResourceParity.find((item) => item.referenceCapability === "user-addresses");
    userAddresses.classification = "foundation";
    userAddresses.defaultPlatformPolicy = "included";
    const coveragePath = tempJSON("coverage.json", coverage);

    const result = runValidator(["--coverage", coveragePath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /reference non-resource capability user-addresses must stay outside the default platform foundation/);
  });

  it("rejects user org membership coverage that is not explicitly kept out of the default foundation", () => {
    const coverage = readJSON("resources/platform-reference-coverage.json");
    coverage.nonResourceParity = currentNonResourceParityFixture();
    const membership = coverage.nonResourceParity.find((item) => item.referenceCapability === "user-org-memberships");
    membership.classification = "foundation";
    membership.defaultPlatformPolicy = "included";
    const coveragePath = tempJSON("coverage.json", coverage);

    const result = runValidator(["--coverage", coveragePath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /reference non-resource capability user-org-memberships must stay outside the default platform foundation/);
  });
});

function currentNonResourceParityFixture() {
  return [
    {
      referenceCapability: "storage-settings",
      referenceModules: ["api/internal/model/model.go", "api/internal/store/store.go"],
      classification: "foundation",
      foundationArea: "dictionary-parameters-branding",
      platformResources: ["settings", "parameters", "files"],
      capabilities: ["parameter", "file-storage"],
    },
    {
      referenceCapability: "app-phone-binding",
      referenceModules: ["docs/api-contract.md"],
      classification: "extension",
      extensionArea: "app-phone-identity",
      expectedCapability: "app-phone",
      expectedProfile: "platform-app-ready",
      appRoutes: ["POST /api/app/identity/phone-verifications", "POST /api/app/identity/phone-bindings"],
      defaultPlatformPolicy: "excluded",
    },
    {
      referenceCapability: "user-addresses",
      referenceModules: ["api/internal/model/model.go"],
      classification: "extension",
      extensionArea: "detailed-addresses",
      expectedCapability: "owning-capability",
      defaultPlatformPolicy: "excluded",
      note: "Area codes are foundation master data; detailed addresses stay in the owning capability until promoted by repeated reusable demand.",
    },
    {
      referenceCapability: "user-org-memberships",
      referenceModules: ["api/internal/database/entities.go", "docs/data-model.md", "docs/platform-architecture.md"],
      classification: "extension",
      extensionArea: "multi-org-membership",
      expectedCapability: "owning-capability",
      defaultPlatformPolicy: "excluded",
      note: "The default foundation exposes a primary org-unit field on users; multi-org memberships stay in an owning extension until repeated reusable demand justifies promotion.",
    },
  ];
}
