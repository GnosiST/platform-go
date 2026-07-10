import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");

function runValidator(args = []) {
  return spawnSync(process.execPath, ["scripts/validate-platform-refresh-token-family-promotion.mjs", ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

function readJSON(relativePath) {
  return JSON.parse(fs.readFileSync(path.join(repoRoot, relativePath), "utf8"));
}

function tempJSON(name, value) {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-refresh-token-family-promotion-"));
  const filePath = path.join(tempDir, name);
  fs.writeFileSync(filePath, `${JSON.stringify(value, null, 2)}\n`);
  return filePath;
}

describe("validate-platform-refresh-token-family-promotion", () => {
  it("accepts the current refresh-token family promotion gate", () => {
    const result = runValidator();

    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Validated refresh-token family promotion gate/);
  });

  it("rejects attempts to change current runtime into refresh-token runtime", () => {
    const contract = readJSON("resources/platform-refresh-token-family-promotion.json");
    contract.currentRuntime.status = "refresh-token-family-enabled";
    contract.currentRuntime.notARefreshTokenFamily = false;
    contract.promotionState.implementationStatus = "blocked";
    contract.promotionState.defaultRuntimeMutation = "enabled";
    const contractPath = tempJSON("platform-refresh-token-family-promotion.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /currentRuntime\.status must stay sliding-renewal-only/);
    assert.match(result.stderr, /currentRuntime\.notARefreshTokenFamily must stay true/);
    assert.match(result.stderr, /promotionState\.implementationStatus must stay implemented/);
    assert.match(result.stderr, /promotionState\.defaultRuntimeMutation must stay forbidden/);
  });

  it("requires implemented runtime artifacts to remain disabled by default", () => {
    const contract = readJSON("resources/platform-refresh-token-family-promotion.json");
    contract.promotionState.refreshTokenFamilyStatus = "enabled";
    contract.promotionState.implementationStatus = "blocked";
    contract.promotionState.runtimeDefault = "enabled";
    contract.implementedRuntimeArtifacts = (contract.implementedRuntimeArtifacts ?? []).filter((item) => item.id !== "refresh-token-family-store");
    const contractPath = tempJSON("platform-refresh-token-family-promotion.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /promotionState\.refreshTokenFamilyStatus must stay implemented-disabled/);
    assert.match(result.stderr, /promotionState\.implementationStatus must stay implemented/);
    assert.match(result.stderr, /promotionState\.runtimeDefault must stay disabled/);
    assert.match(result.stderr, /implementedRuntimeArtifacts must include refresh-token-family-store/);
  });

  it("rejects unsafe refresh-token-family data models", () => {
    const contract = readJSON("resources/platform-refresh-token-family-promotion.json");
    contract.dataModelContract.separateFromSessionTable = false;
    contract.dataModelContract.rawTokenPersistenceAllowed = true;
    contract.dataModelContract.requiredFields = ["familyId", "tokenId"];
    contract.dataModelContract.forbiddenReadFields = [];
    const contractPath = tempJSON("platform-refresh-token-family-promotion.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /dataModelContract\.separateFromSessionTable must stay true/);
    assert.match(result.stderr, /dataModelContract\.rawTokenPersistenceAllowed must stay false/);
    assert.match(result.stderr, /dataModelContract\.requiredFields must include tokenHash/);
    assert.match(result.stderr, /dataModelContract\.forbiddenReadFields must include refreshToken/);
  });

  it("rejects missing rotation, reuse detection and revocation requirements", () => {
    const contract = readJSON("resources/platform-refresh-token-family-promotion.json");
    contract.rotationTransaction.requiredSteps = ["validate-current-token-hash"];
    contract.reuseDetection.requiredEffects = ["reject-with-stable-auth-error"];
    contract.revocationScopeMatrix = contract.revocationScopeMatrix.filter((item) => item.scope !== "reuse-detection");
    contract.redisConvergencePolicy.redisSourceOfTruth = true;
    const contractPath = tempJSON("platform-refresh-token-family-promotion.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /rotationTransaction\.requiredSteps must include publish-sessions-invalidation/);
    assert.match(result.stderr, /reuseDetection\.requiredEffects must include revoke-related-server-side-session/);
    assert.match(result.stderr, /revocationScopeMatrix must include reuse-detection/);
    assert.match(result.stderr, /redisConvergencePolicy\.redisSourceOfTruth must stay false/);
  });

  it("rejects audit or provider-rotation policies that expose secrets or couple unrelated rotations", () => {
    const contract = readJSON("resources/platform-refresh-token-family-promotion.json");
    contract.auditPolicy.allowedFields.push("refreshToken");
    contract.auditPolicy.forbiddenRawFields = ["jwt", "bearerToken", "refreshToken", "tokenHash", "apiToken", "openid", "unionid", "phone", "secret"];
    contract.providerRotationSeparation.providerCredentialRotationRevokesFamiliesByDefault = true;
    contract.providerRotationSeparation.apiTokenRotationTouchesSessionFamilies = true;
    const contractPath = tempJSON("platform-refresh-token-family-promotion.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /auditPolicy\.allowedFields must not include forbidden raw field refreshToken/);
    assert.match(result.stderr, /providerRotationSeparation\.providerCredentialRotationRevokesFamiliesByDefault must stay false/);
    assert.match(result.stderr, /providerRotationSeparation\.apiTokenRotationTouchesSessionFamilies must stay false/);
  });

  it("rejects promotion when production auth and production readiness are not linked", () => {
    const contract = readJSON("resources/platform-refresh-token-family-promotion.json");
    const productionAuth = readJSON("resources/platform-production-auth-hardening.json");
    delete productionAuth.sessionCredentialPolicy.refreshTokenFamily.promotionReadinessContract;
    const readiness = readJSON("resources/platform-production-readiness.json");
    readiness.preflightCommands = readiness.preflightCommands.filter((command) => command.id !== "refresh-token-family-promotion");
    const tokenRotation = readiness.operationPolicies.find((policy) => policy.id === "token-rotation");
    tokenRotation.preflightCommands = tokenRotation.preflightCommands.filter((command) => command !== "refresh-token-family-promotion");
    const contractPath = tempJSON("platform-refresh-token-family-promotion.json", contract);
    const productionAuthPath = tempJSON("platform-production-auth-hardening.json", productionAuth);
    const readinessPath = tempJSON("platform-production-readiness.json", readiness);

    const result = runValidator(["--contract", contractPath, "--production-auth", productionAuthPath, "--production-readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /promotionReadinessContract must point to resources\/platform-refresh-token-family-promotion\.json/);
    assert.match(result.stderr, /production readiness preflightCommands must include refresh-token-family-promotion/);
    assert.match(result.stderr, /token-rotation operation policy must include refresh-token-family-promotion preflight/);
  });

  it("allows production auth hardening closeout while refresh-token family production runtime remains disabled", () => {
    const contract = readJSON("resources/platform-refresh-token-family-promotion.json");
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    const task = graph.tasks.find((item) => item.id === "production-auth-provider-hardening");
    task.status = "implemented";
    const contractPath = tempJSON("platform-refresh-token-family-promotion.json", contract);
    const graphPath = tempJSON("platform-foundation-task-graph.json", graph);

    const result = runValidator(["--contract", contractPath, "--task-graph", graphPath]);

    assert.equal(result.status, 0, result.stderr);
  });
});
