import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");

function runValidator(args = []) {
  return spawnSync(process.execPath, ["scripts/validate-platform-governance-topology.mjs", ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

function readJSON(relativePath) {
  return JSON.parse(fs.readFileSync(path.join(repoRoot, relativePath), "utf8"));
}

function tempJSON(name, value) {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-governance-topology-"));
  const filePath = path.join(tempDir, name);
  fs.writeFileSync(filePath, `${JSON.stringify(value, null, 2)}\n`);
  return filePath;
}

function tempDirectory() {
  return fs.mkdtempSync(path.join(os.tmpdir(), "platform-governance-topology-"));
}

function writeBrokenManifest(mutator) {
  const manifest = readJSON("resources/admin-resources.json");
  mutator(manifest);
  return tempJSON("admin-resources.json", manifest);
}

describe("validate-platform-governance-topology", () => {
  it("accepts the current governance topology", () => {
    const result = runValidator();

    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Validated platform governance topology/);
  });

  it("rejects default tenant area codes that become mandatory", () => {
    const manifestPath = writeBrokenManifest((manifest) => {
      const tenants = manifest.resources.find((resource) => resource.code === "tenants");
      const areaCode = tenants.schema.fields.find((field) => field.key === "areaCode");
      areaCode.required = true;
    });

    const result = runValidator(["--manifest", manifestPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /tenants\.areaCode must stay optional by default/);
  });

  it("rejects role groups that gain permission or inheritance semantics", () => {
    const manifestPath = writeBrokenManifest((manifest) => {
      const roleGroups = manifest.resources.find((resource) => resource.code === "role-groups");
      roleGroups.schema.fields.push({
        key: "inheritedRoleCodes",
        label: { zh: "继承角色", en: "Inherited Roles" },
        type: "multiselect",
        source: "values",
        form: true,
      });
    });

    const result = runValidator(["--manifest", manifestPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /role-groups must remain classification-only/);
    assert.match(result.stderr, /inheritedRoleCodes/);
  });

  it("rejects role-group policies that own permissions, memberships or data scopes", () => {
    const topology = readJSON("resources/platform-governance-topology.json");
    topology.roleGroupPolicy.permissionOwner = "role-groups";
    topology.roleGroupPolicy.membershipOwner = "role-groups.roleCodes";
    topology.roleGroupPolicy.dataScopeOwner = "role-groups.dataScope";
    const topologyPath = tempJSON("platform-governance-topology.json", topology);

    const result = runValidator(["--topology", topologyPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /roleGroupPolicy\.permissionOwner must stay roles/);
    assert.match(result.stderr, /roleGroupPolicy\.membershipOwner must stay users\.roles/);
    assert.match(result.stderr, /roleGroupPolicy\.dataScopeOwner must stay roles\.dataScope/);
  });

  it("records the current organization model only as migration source and protects the approved target", () => {
    const topology = readJSON("resources/platform-governance-topology.json");
    assert.equal(topology.organizationRbacMenuMigration.status, "backend-organization-role-and-menu-implemented-e2e-verified");
    assert.equal(topology.organizationRbacMenuMigration.designStatus, "frozen");
    assert.equal(topology.organizationRbacMenuMigration.designContract, "resources/platform-organization-rbac-menu-contract.json");
    assert.equal(topology.organizationRbacMenuMigration.sourceModel.roleGroupHierarchy, "nested-parentCode");
    assert.equal(topology.organizationRbacMenuMigration.targetModel.roleGroupHierarchy, "flat-two-level-role-group-to-role");
    assert.equal(topology.organizationRbacMenuMigration.targetModel.userTenant, "derived-from-primary-organization");
    assert.equal(topology.organizationRbacMenuMigration.targetModel.roleMenuBinding, "role_menu");

    topology.organizationRbacMenuMigration.targetModel.roleOwnership = "many-role-groups";
    topology.organizationRbacMenuMigration.targetModel.organizationBindingInheritance = "ancestor-union";
    topology.organizationRbacMenuMigration.targetModel.federationAndXa = "allowed-for-authorization";
    const topologyPath = tempJSON("platform-governance-topology.json", topology);
    const result = runValidator(["--topology", topologyPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /targetModel\.roleOwnership must stay exactly-one-role-group/);
    assert.match(result.stderr, /targetModel\.organizationBindingInheritance must stay none-direct-bindings-only/);
    assert.match(result.stderr, /targetModel\.federationAndXa must stay forbidden-for-authorization/);
  });

  it("rejects role resources whose role-group selector is not tree-shaped", () => {
    const manifestPath = writeBrokenManifest((manifest) => {
      const roles = manifest.resources.find((resource) => resource.code === "roles");
      const groupCode = roles.schema.fields.find((field) => field.key === "groupCode");
      groupCode.relation.display = "";
      delete groupCode.relation.parentField;
    });

    const result = runValidator(["--manifest", manifestPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /roles\.groupCode relation\.display must be tree/);
    assert.match(result.stderr, /roles\.groupCode relation\.parentField must be parentCode/);
  });

  it("rejects default profiles that stop excluding personnel", () => {
    const profiles = readJSON("resources/platform-capability-profiles.json");
    const defaultProfile = profiles.profiles.find((profile) => profile.id === "platform-default");
    defaultProfile.mustExcludeCapabilities = defaultProfile.mustExcludeCapabilities.filter((capability) => capability !== "personnel");
    const profilesPath = tempJSON("platform-capability-profiles.json", profiles);

    const result = runValidator(["--profiles", profilesPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /profile platform-default must exclude optional capability personnel/);
  });

  it("rejects personnel-ready profiles that omit shared governance resources", () => {
    const profiles = readJSON("resources/platform-capability-profiles.json");
    const personnelProfile = profiles.profiles.find((profile) => profile.id === "platform-personnel-ready");
    personnelProfile.mustIncludeResources = personnelProfile.mustIncludeResources.filter((resource) => resource !== "area-codes");
    const profilesPath = tempJSON("platform-capability-profiles.json", profiles);

    const result = runValidator(["--profiles", profilesPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /profile platform-personnel-ready must include personnel governance resource area-codes/);
  });

  it("rejects personnel profile contracts that drop the address-code relation", () => {
    const contract = {
      resources: [
        {
          resource: "personnel-profiles",
          fields: [
            { key: "tenantCode", relation: { resource: "tenants" } },
            { key: "orgUnitCode", relation: { resource: "org-units", display: "tree", parentField: "parentCode" } },
            { key: "userCode", relation: { resource: "users" } },
          ],
        },
        {
          resource: "positions",
          fields: [
            { key: "tenantCode", relation: { resource: "tenants" } },
            { key: "orgUnitCode", relation: { resource: "org-units", display: "tree", parentField: "parentCode" } },
          ],
        },
        {
          resource: "position-assignments",
          fields: [
            { key: "personnelCode", required: true, relation: { resource: "personnel-profiles" } },
            { key: "positionCode", required: true, relation: { resource: "positions" } },
            { key: "tenantCode", relation: { resource: "tenants" } },
            { key: "orgUnitCode", relation: { resource: "org-units", display: "tree", parentField: "parentCode" } },
          ],
        },
      ],
    };
    const contractPath = tempJSON("personnel-admin-contract.json", contract);

    const result = runValidator(["--personnel-admin-contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /personnel-profiles\.areaCode is missing/);
  });

  it("rejects org-unit topology that regresses to tenant-only organization support", () => {
    const topology = readJSON("resources/platform-governance-topology.json");
    topology.orgUnitPolicy.mode = "tenant-only";
    topology.orgUnitPolicy.levels = ["organization"];
    topology.orgUnitPolicy.requiredDefault = false;
    const topologyPath = tempJSON("platform-governance-topology.json", topology);

    const result = runValidator(["--topology", topologyPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /orgUnitPolicy\.mode must stay single-tree/);
    assert.match(result.stderr, /orgUnitPolicy\.levels must include group, company, branch, organization, department, team, store, custom/);
    assert.match(result.stderr, /orgUnitPolicy\.requiredDefault must stay true/);
  });

  it("rejects org-unit contracts that omit common institution levels", () => {
    const manifestPath = writeBrokenManifest((manifest) => {
      const orgUnits = manifest.resources.find((resource) => resource.code === "org-units");
      const typeField = orgUnits.schema.fields.find((field) => field.key === "type");
      typeField.options = ["organization", "department", "team"];
    });

    const result = runValidator(["--manifest", manifestPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /org-units\.type must include options group, company, branch, organization, department, team, store, custom/);
  });

  it("rejects default topology that stops requiring governance primitives", () => {
    const topology = readJSON("resources/platform-governance-topology.json");
    topology.defaultFoundation.mustIncludeResources = ["tenants", "users", "roles"];
    topology.profileRequirements.defaultMustIncludeResources = ["tenants", "users", "roles"];
    const topologyPath = tempJSON("platform-governance-topology.json", topology);

    const result = runValidator(["--topology", topologyPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /defaultFoundation\.mustIncludeResources must include org-units/);
    assert.match(result.stderr, /defaultFoundation\.mustIncludeResources must include role-groups/);
    assert.match(result.stderr, /defaultFoundation\.mustIncludeResources must include area-codes/);
    assert.match(result.stderr, /profileRequirements\.defaultMustIncludeResources must include org-units/);
  });

  it("rejects org units that are not tenant-owned", () => {
    const manifestPath = writeBrokenManifest((manifest) => {
      const orgUnits = manifest.resources.find((resource) => resource.code === "org-units");
      const tenantCode = orgUnits.schema.fields.find((field) => field.key === "tenantCode");
      delete tenantCode.required;
    });

    const result = runValidator(["--manifest", manifestPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /org-units\.tenantCode must be required/);
  });

  it("rejects generated capability contracts that drift from tenant-owned account and org units", () => {
    const contract = readJSON("resources/generated/admin-capability-resource-contract.json");
    for (const resourceCode of ["users", "org-units"]) {
      const resource = contract.resources.find((item) => item.resource === resourceCode);
      const tenantCode = resource.fields.find((field) => field.key === "tenantCode");
      delete tenantCode.required;
    }
    const contractPath = tempJSON("admin-capability-resource-contract.json", contract);

    const result = runValidator(["--capability-admin-contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /users\.tenantCode must be required/);
    assert.match(result.stderr, /org-units\.tenantCode must be required/);
  });

  it("rejects default topology that allows parallel organization resources", () => {
    const topology = readJSON("resources/platform-governance-topology.json");
    topology.orgUnitPolicy.forbiddenDefaultResources = topology.orgUnitPolicy.forbiddenDefaultResources.filter((resource) => resource !== "organizations");
    topology.defaultFoundation.mustExcludeResources = topology.defaultFoundation.mustExcludeResources.filter((resource) => resource !== "organizations");
    const topologyPath = tempJSON("platform-governance-topology.json", topology);

    const result = runValidator(["--topology", topologyPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /orgUnitPolicy\.forbiddenDefaultResources must include organizations/);
    assert.match(result.stderr, /default foundation must exclude parallel governance resource organizations/);
  });

  it("rejects default topology that promotes personnel resources", () => {
    const topology = readJSON("resources/platform-governance-topology.json");
    topology.defaultFoundation.mustIncludeResources.push("personnel-profiles");
    topology.defaultFoundation.mustExcludeResources = topology.defaultFoundation.mustExcludeResources.filter((resource) => resource !== "positions");
    topology.profileRequirements.defaultMustExcludeCapabilities = [];
    const topologyPath = tempJSON("platform-governance-topology.json", topology);

    const result = runValidator(["--topology", topologyPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /defaultFoundation\.mustIncludeResources must not include optional personnel resource personnel-profiles/);
    assert.match(result.stderr, /defaultFoundation\.mustExcludeResources must include optional personnel resource positions/);
    assert.match(result.stderr, /profileRequirements\.defaultMustExcludeCapabilities must include personnel/);
  });

  it("rejects area codes that become implicit authorization", () => {
    const topology = readJSON("resources/platform-governance-topology.json");
    topology.areaCodePolicy.implicitAuthorization = true;
    const topologyPath = tempJSON("platform-governance-topology.json", topology);

    const result = runValidator(["--topology", topologyPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /areaCodePolicy\.implicitAuthorization must stay false/);
  });

  it("rejects area-code policies that stop declaring shared consumers", () => {
    const topology = readJSON("resources/platform-governance-topology.json");
    topology.areaCodePolicy.defaultResourceRequired = false;
    topology.areaCodePolicy.attachmentRequiredByDefault = true;
    topology.areaCodePolicy.defaultConsumers = ["tenants", "users"];
    topology.areaCodePolicy.optionalConsumers = [];
    topology.areaCodePolicy.levels = ["country", "city"];
    const topologyPath = tempJSON("platform-governance-topology.json", topology);

    const result = runValidator(["--topology", topologyPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /areaCodePolicy\.defaultResourceRequired must stay true/);
    assert.match(result.stderr, /areaCodePolicy\.attachmentRequiredByDefault must stay false/);
    assert.match(result.stderr, /areaCodePolicy\.defaultConsumers must include org-units/);
    assert.match(result.stderr, /areaCodePolicy\.optionalConsumers must include personnel-profiles/);
    assert.match(result.stderr, /areaCodePolicy\.levels must include country, province, city, district, street, custom/);
  });

  it("rejects area-code policies that remove attachment fields or promote detailed addresses into the default foundation", () => {
    const topology = readJSON("resources/platform-governance-topology.json");
    topology.areaCodePolicy.attachmentFields = ["tenants.areaCode", "users.areaCode"];
    topology.areaCodePolicy.authorizationConsumers = [];
    topology.areaCodePolicy.detailedAddressPolicy.defaultFoundation = "included";
    topology.areaCodePolicy.detailedAddressPolicy.promotionRule = "promote-on-first-request";
    topology.defaultFoundation.mustExcludeResources = topology.defaultFoundation.mustExcludeResources.filter((resource) => resource !== "user-addresses");
    const topologyPath = tempJSON("platform-governance-topology.json", topology);

    const result = runValidator(["--topology", topologyPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /areaCodePolicy\.attachmentFields must include org-units\.areaCode/);
    assert.match(result.stderr, /areaCodePolicy\.attachmentFields must include personnel-profiles\.areaCode/);
    assert.match(result.stderr, /areaCodePolicy\.authorizationConsumers must include roles\.dataScopeAreaCodes/);
    assert.match(result.stderr, /areaCodePolicy\.detailedAddressPolicy\.defaultFoundation must stay excluded/);
    assert.match(result.stderr, /areaCodePolicy\.detailedAddressPolicy\.promotionRule must stay promote-only-after-two-reusable-capabilities/);
    assert.match(result.stderr, /defaultFoundation\.mustExcludeResources must include detailed address resource user-addresses/);
  });

  it("rejects governance evaluation drift toward tenant-only, role inheritance or implicit address-code authorization", () => {
    const topology = readJSON("resources/platform-governance-topology.json");
    topology.governanceEvaluation.tenantOnlySupport.decision = "accepted";
    topology.governanceEvaluation.tenantOnlySupport.requiredDefaultResource = "tenants";
    topology.governanceEvaluation.tenantOnlySupport.supportedOrgUnitTypes = ["organization", "department"];
    topology.governanceEvaluation.roleGroups.decision = "supported-as-policy-owner";
    topology.governanceEvaluation.roleGroups.forbiddenSemantics = ["role-membership"];
    topology.governanceEvaluation.areaCodes.defaultResourceRequired = false;
    topology.governanceEvaluation.areaCodes.attachmentRequiredByDefault = true;
    topology.governanceEvaluation.areaCodes.implicitAuthorization = true;
    topology.governanceEvaluation.areaCodes.detailedAddressesDefault = "included";
    topology.governanceEvaluation.personnel.decision = "default-foundation";
    const topologyPath = tempJSON("platform-governance-topology.json", topology);

    const result = runValidator(["--topology", topologyPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /governanceEvaluation\.tenantOnlySupport\.decision must stay rejected/);
    assert.match(result.stderr, /governanceEvaluation\.tenantOnlySupport\.requiredDefaultResource must stay org-units/);
    assert.match(result.stderr, /governanceEvaluation\.tenantOnlySupport\.supportedOrgUnitTypes must include group, company, branch, organization, department, team, store, custom/);
    assert.match(result.stderr, /governanceEvaluation\.roleGroups\.decision must stay supported-as-classification/);
    assert.match(result.stderr, /governanceEvaluation\.roleGroups\.forbiddenSemantics must include permission-grant/);
    assert.match(result.stderr, /governanceEvaluation\.areaCodes\.defaultResourceRequired must match areaCodePolicy\.defaultResourceRequired/);
    assert.match(result.stderr, /governanceEvaluation\.areaCodes\.attachmentRequiredByDefault must match areaCodePolicy\.attachmentRequiredByDefault/);
    assert.match(result.stderr, /governanceEvaluation\.areaCodes\.implicitAuthorization must match areaCodePolicy\.implicitAuthorization/);
    assert.match(result.stderr, /governanceEvaluation\.areaCodes\.detailedAddressesDefault must match areaCodePolicy\.detailedAddressPolicy\.defaultFoundation/);
    assert.match(result.stderr, /governanceEvaluation\.personnel\.decision must stay optional-capability/);
  });
});
