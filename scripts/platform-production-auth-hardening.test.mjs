import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");

function runValidator(args = []) {
  return spawnSync(process.execPath, ["scripts/validate-platform-production-auth-hardening.mjs", ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

function runScript(script, args = []) {
  return spawnSync(process.execPath, [script, ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

function readJSON(relativePath) {
  return JSON.parse(fs.readFileSync(path.join(repoRoot, relativePath), "utf8"));
}

function tempJSON(name, value) {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-production-auth-hardening-"));
  const filePath = path.join(tempDir, name);
  fs.writeFileSync(filePath, `${JSON.stringify(value, null, 2)}\n`);
  return filePath;
}

function tempText(name, value) {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-production-auth-hardening-doc-"));
  const filePath = path.join(tempDir, name);
  fs.writeFileSync(filePath, value);
  return filePath;
}

describe("validate-platform-production-auth-hardening", () => {
  it("accepts the current production auth hardening contract", () => {
    const result = runValidator();

    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Validated platform production auth hardening/);
  });

  it("generates a non-mutating production auth promotion review packet", () => {
    const result = runScript("scripts/generate-production-auth-promotion-review.mjs", ["--stdout"]);

    assert.equal(result.status, 0, result.stderr);
    const review = JSON.parse(result.stdout);
    assert.equal(review.generatedBy, "scripts/generate-production-auth-promotion-review.mjs");
    assert.equal(review.mode.dryRun, true);
    assert.equal(review.mode.runtimeMutation, "disabled");
    assert.equal(review.mode.refreshTokenFamilyRuntime, "disabled");
    assert.equal(review.mode.providerRuntimeMutation, "disabled");
    assert.equal(review.currentRuntime.refreshTokenFamilyStatus, "implemented-disabled");
    assert.equal(review.currentRuntime.refreshTokenFamilyPromotionStatus, "implemented");
    assert.equal(review.currentRuntime.notARefreshTokenFamily, true);
    assert.equal(review.approvalPackage.status, "blocked");
    assert.equal(review.manualReview.decision, "not-approved");
    assert.ok(review.approvalPackage.missingEvidence.includes("runtime-test-output"));
    assert.ok(review.blockers.includes("production approval package is blocked"));
    assert.ok(review.preflight.tokenRotationPolicyCommands.includes("production-auth-promotion-review"));
    const oidc = review.providerPromotionMatrix.providers.find((provider) => provider.id === "oidc");
    assert.deepEqual(oidc.audiences, ["admin"]);
    assert.equal(oidc.productionLikeRehearsalRequired, true);
  });

  it("rejects production auth promotion reviews that weaken typed OIDC projection invariants", () => {
    const review = readJSON("resources/generated/production-auth-promotion-review.json");
    const oidc = review.providerPromotionMatrix.providers.find((provider) => provider.id === "oidc");
    oidc.capability = "session";
    oidc.kind = "saml";
    oidc.productionUsage = "local-harness-only";
    oidc.adapterBoundary = "httpapi.AppIdentityResolver";
    oidc.audiences = ["app"];
    oidc.configKeys = [
      "PLATFORM_ADMIN_OIDC_ISSUER_URL",
      "PLATFORM_ADMIN_OIDC_CLIENT_ID",
      "PLATFORM_ADMIN_OIDC_CLIENT_SECRET",
      "PLATFORM_ADMIN_OIDC_REDIRECT_URL",
      "PLATFORM_ADMIN_OIDC_REDIRECT_URL",
    ];
    oidc.requiredControls = oidc.requiredControls.map((control, index) => (index === oidc.requiredControls.length - 1 ? oidc.requiredControls[0] : control));
    oidc.requiresSecretOwner = false;
    oidc.rotationRunbookRequired = false;
    oidc.subjectRedactionRequired = false;
    oidc.unconfiguredProviderRejectionRequired = false;
    oidc.errorNormalizationRequired = false;
    oidc.productionLikeRehearsalRequired = false;
    oidc.rawCredentialExposureAllowed = true;
    oidc.rawSubjectExposureAllowed = true;
    const reviewPath = tempJSON("production-auth-promotion-review.json", review);

    const result = runValidator(["--promotion-review", reviewPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /production auth promotion review provider oidc capability must match production auth hardening contract/);
    assert.match(result.stderr, /production auth promotion review provider oidc kind must match production auth hardening contract/);
    assert.match(result.stderr, /production auth promotion review provider oidc productionUsage must match production auth hardening contract/);
    assert.match(result.stderr, /production auth promotion review provider oidc adapterBoundary must match production auth hardening contract/);
    assert.match(result.stderr, /production auth promotion review provider oidc audiences must match production auth hardening contract/);
    assert.match(result.stderr, /production auth promotion review provider oidc configKeys must match production auth hardening contract/);
    assert.match(result.stderr, /production auth promotion review provider oidc requiredControls must match production auth hardening contract/);
    assert.match(result.stderr, /production auth promotion review provider oidc requiresSecretOwner must match production auth hardening contract/);
    assert.match(result.stderr, /production auth promotion review provider oidc rawCredentialExposureAllowed must match production auth hardening contract/);
    assert.match(result.stderr, /production auth promotion review provider oidc productionLikeRehearsalRequired must match production auth hardening contract/);
  });

  it("rejects production auth contracts without a non-mutating promotion review declaration", () => {
    const contract = readJSON("resources/platform-production-auth-hardening.json");
    delete contract.promotionReview;
    const contractPath = tempJSON("platform-production-auth-hardening.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /promotionReview\.path must point to resources\/generated\/production-auth-promotion-review\.json/);
    assert.match(result.stderr, /promotionReview\.generator must be scripts\/generate-production-auth-promotion-review\.mjs/);
    assert.match(result.stderr, /promotionReview\.decision must stay not-approved/);
    assert.match(result.stderr, /promotionReview\.runtimeMutation must stay disabled/);
  });

  it("rejects production auth promotion reviews that approve active blockers", () => {
    const review = readJSON("resources/generated/production-auth-promotion-review.json");
    review.mode.runtimeMutation = "enabled";
    review.manualReview.decision = "approved";
    review.blockers = [];
    review.summary.blockerCount = 0;
    review.approvalPackage.missingEvidence = [];
    const reviewPath = tempJSON("production-auth-promotion-review.json", review);

    const result = runValidator(["--promotion-review", reviewPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /production auth promotion review mode\.runtimeMutation must stay disabled/);
    assert.match(result.stderr, /production auth promotion review approvalPackage\.missingEvidence must reflect incomplete approval evidence/);
    assert.match(result.stderr, /production auth promotion review blockers must include production approval package is blocked/);
    assert.match(result.stderr, /production auth promotion review manualReview\.decision must stay not-approved/);
    assert.match(result.stderr, /production auth promotion review summary\.blockerCount must reflect active blockers/);
  });

  it("rejects refresh-token family enablement without promotion", () => {
    const contract = readJSON("resources/platform-production-auth-hardening.json");
    contract.sessionCredentialPolicy.refreshTokenFamily.status = "enabled";
    contract.sessionCredentialPolicy.refreshTokenFamily.defaultRuntime = "enabled";
    const contractPath = tempJSON("platform-production-auth-hardening.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /refreshTokenFamily status must stay implemented-disabled until production approval is attached/);
    assert.match(result.stderr, /refreshTokenFamily defaultRuntime must stay disabled until production approval is attached/);
  });

  it("rejects session stores that persist raw handles or preserve legacy sessions", () => {
    const contract = readJSON("resources/platform-production-auth-hardening.json");
    contract.sessionCredentialPolicy.sessionStore.tokenPersistence = "raw-token";
    contract.sessionCredentialPolicy.sessionStore.rawTokenPersistenceAllowed = true;
    contract.sessionCredentialPolicy.sessionStore.legacyRawSessionMigration = "preserve";
    const contractPath = tempJSON("platform-production-auth-hardening.json", contract);
    const review = readJSON("resources/generated/production-auth-promotion-review.json");
    review.sources.productionAuthHardening = path.relative(repoRoot, contractPath).split(path.sep).join("/");
    const reviewPath = tempJSON("production-auth-promotion-review.json", review);

    const result = runValidator(["--contract", contractPath, "--promotion-review", reviewPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /sessionCredentialPolicy.sessionStore.tokenPersistence must stay sha256:v1-digest-only/);
    assert.match(result.stderr, /sessionCredentialPolicy.sessionStore.rawTokenPersistenceAllowed must stay false/);
    assert.match(result.stderr, /sessionCredentialPolicy.sessionStore.legacyRawSessionMigration must stay replace-and-revoke/);
  });

  it("rejects refresh-token family policies without a production session specification", () => {
    const contract = readJSON("resources/platform-production-auth-hardening.json");
    delete contract.sessionCredentialPolicy.productionSessionPolicy;
    contract.sessionCredentialPolicy.refreshTokenFamily.specification = "docs/missing-session-policy.md";
    contract.sessionCredentialPolicy.refreshTokenFamily.requiredProductionEnablementEvidence = ["hashed refresh-token-family storage"];
    const contractPath = tempJSON("platform-production-auth-hardening.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /sessionCredentialPolicy\.productionSessionPolicy\.status must stay specified/);
    assert.match(result.stderr, /sessionCredentialPolicy\.productionSessionPolicy\.path is missing or unsafe/);
    assert.match(result.stderr, /sessionCredentialPolicy\.refreshTokenFamily\.specification must match productionSessionPolicy\.path/);
    assert.match(result.stderr, /sessionCredentialPolicy\.refreshTokenFamily\.requiredProductionEnablementEvidence must include rotation lineage and reuse-detection tests/);
  });

  it("rejects refresh-token family policies without a promotion readiness contract", () => {
    const contract = readJSON("resources/platform-production-auth-hardening.json");
    contract.sessionCredentialPolicy.refreshTokenFamily.promotionReadinessContract = "resources/missing-refresh-token-family-promotion.json";
    contract.validators = contract.validators.filter((validator) => validator !== "scripts/validate-platform-refresh-token-family-promotion.mjs");
    const contractPath = tempJSON("platform-production-auth-hardening.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /promotionReadinessContract must point to resources\/platform-refresh-token-family-promotion\.json/);
    assert.match(result.stderr, /promotionReadinessContract is missing or unsafe/);
  });

  it("rejects production session specifications that drop mandatory policy decisions", () => {
    const contract = readJSON("resources/platform-production-auth-hardening.json");
    contract.sessionCredentialPolicy.productionSessionPolicy.requiredDecisions = ["current-runtime-sliding-renewal-boundary"];
    contract.sessionCredentialPolicy.productionSessionPolicy.runtimePromotion = "runtime-enabled";
    const contractPath = tempJSON("platform-production-auth-hardening.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /sessionCredentialPolicy\.productionSessionPolicy\.requiredDecisions must include refresh-token-family-data-model/);
    assert.match(result.stderr, /sessionCredentialPolicy\.productionSessionPolicy\.requiredDecisions must include redis-invalidation-not-source-of-truth/);
    assert.match(result.stderr, /sessionCredentialPolicy\.productionSessionPolicy\.runtimePromotion must stay blocked-until-production-approval-package-approved/);
  });

  it("rejects production auth promotion contracts without structured approval evidence", () => {
    const contract = readJSON("resources/platform-production-auth-hardening.json");
    delete contract.productionPromotionApprovalPackage;
    const contractPath = tempJSON("platform-production-auth-hardening.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /productionPromotionApprovalPackage\.status must stay blocked before promotion/);
    assert.match(result.stderr, /productionPromotionApprovalPackage\.requiredApprovals must include security-owner/);
    assert.match(result.stderr, /productionPromotionApprovalPackage\.requiredEvidence must include runtime-test-output/);
  });

  it("rejects production auth promotion contracts that claim text-only approval", () => {
    const contract = readJSON("resources/platform-production-auth-hardening.json");
    contract.productionPromotionApprovalPackage = {
      status: "blocked",
      sourceOfTruth: "external-review-artifacts",
      defaultRuntimeMutation: "forbidden",
      requiredApprovals: ["security-owner", "platform-architect", "operations-owner"],
      requiredEvidence: [
        {
          id: "session-policy-review",
          owner: "security-owner",
          evidenceKind: "signed-security-review",
          description: "Security review for the current JWT/session boundary and future token-family promotion.",
        },
        {
          id: "runtime-test-output",
          owner: "security-owner",
          evidenceKind: "test-output",
          description: "Captured output for session, refresh-token-family, provider, Redis invalidation and audit redaction tests.",
        },
      ],
      completedEvidence: [],
      prohibitedEvidence: ["text-only approval", "single-person self approval"],
      mustNotEnableRefreshTokenFamily: true,
      mustNotEnableUnreviewedProvider: true,
    };
    contract.productionPromotionApprovalPackage.status = "ready";
    contract.productionPromotionApprovalPackage.defaultRuntimeMutation = "enabled";
    contract.productionPromotionApprovalPackage.completedEvidence = ["session-policy-review"];
    contract.productionPromotionApprovalPackage.prohibitedEvidence = contract.productionPromotionApprovalPackage.prohibitedEvidence.filter(
      (item) => item !== "text-only approval",
    );
    const contractPath = tempJSON("platform-production-auth-hardening.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /productionPromotionApprovalPackage\.status must stay blocked before promotion/);
    assert.match(result.stderr, /productionPromotionApprovalPackage\.defaultRuntimeMutation must stay forbidden/);
    assert.match(result.stderr, /productionPromotionApprovalPackage\.completedEvidence must stay empty before promotion/);
    assert.match(result.stderr, /productionPromotionApprovalPackage\.prohibitedEvidence must include text-only approval/);
  });

  it("rejects promotion approval packages without an evidence artifact schema", () => {
    const contract = readJSON("resources/platform-production-auth-hardening.json");
    delete contract.productionPromotionApprovalPackage.completedEvidenceSchema;
    const contractPath = tempJSON("platform-production-auth-hardening.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /completedEvidenceSchema\.requiredFields must include artifactURI/);
    assert.match(result.stderr, /completedEvidenceSchema\.requiredFields must include rollbackCommands/);
    assert.match(result.stderr, /completedEvidenceSchema\.requiredFields must include providerRotationRunbookRefs/);
    assert.match(result.stderr, /completedEvidenceSchema\.requiredFields must include refreshTokenFamilyTestRefs/);
    assert.match(result.stderr, /completedEvidenceSchema\.requiredFields must include providerIds/);
    assert.match(result.stderr, /completedEvidenceSchema\.requiredFields must include providerControls/);
    assert.match(result.stderr, /completedEvidenceSchema\.requiredFields must include runtimeTestRefs/);
    assert.match(result.stderr, /completedEvidenceSchema\.approvalRules must include approvedBy-must-not-equal-owner/);
    assert.match(result.stderr, /completedEvidenceSchema\.approvalRules must include refresh-token-family-tests-required-before-runtime-mutation/);
    assert.match(result.stderr, /completedEvidenceSchema\.approvalRules must include redacted-audit-sample-required-before-promotion/);
    assert.match(result.stderr, /completedEvidenceSchema\.approvalRules must include provider-controls-covered-before-promotion/);
    assert.match(result.stderr, /completedEvidenceSchema\.approvalRules must include provider-runtime-tests-required-before-promotion/);
    assert.match(result.stderr, /completedEvidenceSchema\.forbiddenFields must include refreshToken/);
    assert.match(result.stderr, /completedEvidenceSchema\.forbiddenFields must include tokenHash/);
    assert.match(result.stderr, /completedEvidenceSchema\.forbiddenFields must include rawSubject/);
  });

  it("rejects promotion approval packages without a strong artifact hash policy", () => {
    const contract = readJSON("resources/platform-production-auth-hardening.json");
    delete contract.productionPromotionApprovalPackage.completedEvidenceSchema.artifactHashPolicy;
    contract.productionPromotionApprovalPackage.completedEvidenceSchema.approvalRules =
      contract.productionPromotionApprovalPackage.completedEvidenceSchema.approvalRules.filter(
        (rule) => rule !== "artifact-hash-must-be-sha256-hex",
      );
    const contractPath = tempJSON("platform-production-auth-hardening.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /completedEvidenceSchema\.artifactHashPolicy\.algorithm must be sha256/);
    assert.match(result.stderr, /completedEvidenceSchema\.artifactHashPolicy\.format must be prefix-hex/);
    assert.match(result.stderr, /completedEvidenceSchema\.artifactHashPolicy\.hexLength must be 64/);
    assert.match(result.stderr, /completedEvidenceSchema\.approvalRules must include artifact-hash-must-be-sha256-hex/);
  });

  it("rejects promotion approval packages without an external artifact URI policy", () => {
    const contract = readJSON("resources/platform-production-auth-hardening.json");
    delete contract.productionPromotionApprovalPackage.completedEvidenceSchema.artifactURIPolicy;
    contract.productionPromotionApprovalPackage.completedEvidenceSchema.approvalRules =
      contract.productionPromotionApprovalPackage.completedEvidenceSchema.approvalRules.filter(
        (rule) => rule !== "artifact-uri-must-be-external-review-artifact",
      );
    const contractPath = tempJSON("platform-production-auth-hardening.json", contract);
    const review = readJSON("resources/generated/production-auth-promotion-review.json");
    review.sources.productionAuthHardening = path.relative(repoRoot, contractPath).split(path.sep).join("/");
    delete review.approvalPackage.completedEvidenceSchema.artifactURIPolicy;
    review.approvalPackage.completedEvidenceSchema.approvalRules =
      review.approvalPackage.completedEvidenceSchema.approvalRules.filter(
        (rule) => rule !== "artifact-uri-must-be-external-review-artifact",
      );
    const reviewPath = tempJSON("production-auth-promotion-review.json", review);

    const result = runValidator(["--contract", contractPath, "--promotion-review", reviewPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /completedEvidenceSchema\.artifactURIPolicy\.sourceOfTruth must be external-review-artifacts/);
    assert.match(result.stderr, /completedEvidenceSchema\.artifactURIPolicy\.allowedSchemes must include https/);
    assert.match(result.stderr, /completedEvidenceSchema\.artifactURIPolicy\.allowedSchemes must include s3/);
    assert.match(result.stderr, /completedEvidenceSchema\.artifactURIPolicy\.forbidLocalhost must be true/);
    assert.match(result.stderr, /completedEvidenceSchema\.approvalRules must include artifact-uri-must-be-external-review-artifact/);
  });

  it("rejects provider adapters without secret rotation and subject redaction policy", () => {
    const contract = readJSON("resources/platform-production-auth-hardening.json");
    contract.providerAdapterPolicy.requiredControls = contract.providerAdapterPolicy.requiredControls.filter(
      (control) => control !== "secret-rotation-plan" && control !== "provider-subject-redaction",
    );
    const contractPath = tempJSON("platform-production-auth-hardening.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /providerAdapterPolicy.requiredControls must include secret-rotation-plan/);
    assert.match(result.stderr, /providerAdapterPolicy.requiredControls must include provider-subject-redaction/);
  });

  it("rejects step-up verification policy drift", () => {
    const contract = readJSON("resources/platform-production-auth-hardening.json");
    contract.stepUpVerificationPolicy.conditionalConfigKeys = [];
    contract.stepUpVerificationPolicy.secretSeparation = ["phone"];
    contract.stepUpVerificationPolicy.smsProviderPolicy.stockProcess = "production-sms-bundled";
    contract.stepUpVerificationPolicy.smsProviderPolicy.failClosedWithoutRegisteredSender = false;
    contract.stepUpVerificationPolicy.verifiedPhoneBinding.currentPhoneDigestMustMatchStoredVerifiedDigest = false;
    const contractPath = tempJSON("platform-production-auth-hardening.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /stepUpVerificationPolicy\.conditionalConfigKeys must include PLATFORM_SENSITIVE_REVEAL_HMAC_KEY/);
    assert.match(result.stderr, /stepUpVerificationPolicy\.secretSeparation must include rate-limit/);
    assert.match(result.stderr, /stepUpVerificationPolicy\.smsProviderPolicy\.stockProcess must stay debug-only-local-harness/);
    assert.match(result.stderr, /stepUpVerificationPolicy\.smsProviderPolicy\.failClosedWithoutRegisteredSender must stay true/);
    assert.match(result.stderr, /stepUpVerificationPolicy\.verifiedPhoneBinding\.currentPhoneDigestMustMatchStoredVerifiedDigest must stay true/);
  });

  it("rejects provider runtime policies that allow unconfigured login or raw subject exposure", () => {
    const contract = readJSON("resources/platform-production-auth-hardening.json");
    contract.providerRuntimePolicy.defaultDenyUnconfiguredProviders = false;
    contract.providerRuntimePolicy.rawSubjectStorage = "raw";
    contract.providerRuntimePolicy.responseRawSubjectAllowed = true;
    contract.providerRuntimePolicy.auditRawSubjectAllowed = true;
    contract.providerRuntimePolicy.adapterRegistration = "global-import";
    const contractPath = tempJSON("platform-production-auth-hardening.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /providerRuntimePolicy.defaultDenyUnconfiguredProviders must stay true/);
    assert.match(result.stderr, /providerRuntimePolicy.rawSubjectStorage must stay hash-and-mask-only/);
    assert.match(result.stderr, /providerRuntimePolicy.responseRawSubjectAllowed must stay false/);
    assert.match(result.stderr, /providerRuntimePolicy.auditRawSubjectAllowed must stay false/);
    assert.match(result.stderr, /providerRuntimePolicy.adapterRegistration must stay manifest-declared-and-composition-root-injected/);
  });

  it("rejects provider runtime policies without required hardening tests", () => {
    const contract = readJSON("resources/platform-production-auth-hardening.json");
    contract.providerRuntimePolicy.requiredTests = ["subject-redaction"];
    contract.providerAdapterPolicy.productionPromotionRequires = contract.providerAdapterPolicy.productionPromotionRequires.filter(
      (item) => item !== "contract tests for unconfigured provider rejection",
    );
    const contractPath = tempJSON("platform-production-auth-hardening.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /providerRuntimePolicy.requiredTests must include unconfigured-provider-rejection/);
    assert.match(result.stderr, /providerRuntimePolicy.requiredTests must include configured-provider-only-login/);
    assert.match(result.stderr, /providerRuntimePolicy.requiredTests must include provider-error-normalization/);
    assert.match(result.stderr, /providerAdapterPolicy.productionPromotionRequires must include contract tests for unconfigured provider rejection/);
  });

  it("rejects provider promotion matrices that promote demo login or expose raw subjects", () => {
    const contract = readJSON("resources/platform-production-auth-hardening.json");
    const demo = contract.providerPromotionMatrix.providers.find((provider) => provider.id === "demo");
    demo.productionUsage = "optional-production-provider";
    demo.rawSubjectExposureAllowed = true;
    const contractPath = tempJSON("platform-production-auth-hardening.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /providerPromotionMatrix provider demo productionUsage must stay local-harness-only/);
    assert.match(result.stderr, /providerPromotionMatrix provider demo rawSubjectExposureAllowed must stay false/);
  });

  it("rejects provider promotion matrices without typed audience and rehearsal fields", () => {
    const contract = readJSON("resources/platform-production-auth-hardening.json");
    const demo = contract.providerPromotionMatrix.providers.find((provider) => provider.id === "demo");
    const wechat = contract.providerPromotionMatrix.providers.find((provider) => provider.id === "wechat");
    delete demo.audiences;
    delete wechat.productionLikeRehearsalRequired;
    const contractPath = tempJSON("platform-production-auth-hardening.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /providerPromotionMatrix provider demo audiences must declare at least one typed audience/);
    assert.match(result.stderr, /providerPromotionMatrix provider wechat productionLikeRehearsalRequired must be boolean/);
  });

  it("rejects provider promotion matrices that drop wechat production controls", () => {
    const contract = readJSON("resources/platform-production-auth-hardening.json");
    const wechat = contract.providerPromotionMatrix.providers.find((provider) => provider.id === "wechat");
    wechat.configKeys = ["PLATFORM_WECHAT_MINIAPP_APP_ID"];
    wechat.requiredControls = ["configured-provider-only-login"];
    wechat.requiresSecretOwner = false;
    wechat.rotationRunbookRequired = false;
    wechat.unconfiguredProviderRejectionRequired = false;
    const contractPath = tempJSON("platform-production-auth-hardening.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /providerPromotionMatrix provider wechat configKeys must include PLATFORM_WECHAT_MINIAPP_SECRET/);
    assert.match(result.stderr, /providerPromotionMatrix provider wechat requiredControls must include secret-rotation-plan/);
    assert.match(result.stderr, /providerPromotionMatrix provider wechat requiresSecretOwner must stay true/);
    assert.match(result.stderr, /providerPromotionMatrix provider wechat rotationRunbookRequired must stay true/);
    assert.match(result.stderr, /providerPromotionMatrix provider wechat unconfiguredProviderRejectionRequired must stay true/);
  });

  it("requires the Admin-only OIDC provider promotion contract", () => {
    const contract = readJSON("resources/platform-production-auth-hardening.json");
    const oidc = contract.providerPromotionMatrix.providers.find((provider) => provider.id === "oidc");

    assert.ok(oidc, "providerPromotionMatrix must include oidc");
    assert.equal(oidc.capability, "admin-oidc");
    assert.equal(oidc.kind, "oidc");
    assert.equal(oidc.productionUsage, "optional-production-provider");
    assert.equal(oidc.adapterBoundary, "httpapi.AdminIdentityResolver");
    assert.deepEqual(oidc.audiences, ["admin"]);
    assert.deepEqual(oidc.configKeys, [
      "PLATFORM_ADMIN_OIDC_ISSUER_URL",
      "PLATFORM_ADMIN_OIDC_CLIENT_ID",
      "PLATFORM_ADMIN_OIDC_CLIENT_SECRET",
      "PLATFORM_ADMIN_OIDC_REDIRECT_URL",
      "PLATFORM_ADMIN_OIDC_SCOPES",
    ]);
    for (const control of [
      "admin-audience-only",
      "configured-provider-only-discovery-and-exchange",
      "issuer-validation",
      "signature-validation",
      "audience-validation",
      "nonce-validation",
      "state-validation",
      "pkce-s256-validation",
      "exact-redirect-url-validation",
      "explicit-identity-binding",
      "disabled-user-rejection",
      "provider-subject-redaction",
      "raw-provider-subject-never-in-response",
      "raw-provider-subject-never-in-audit",
      "provider-specific-error-normalization",
      "audit-redaction",
      "production-like-runtime-rehearsal",
    ]) {
      assert.ok(oidc.requiredControls.includes(control), `OIDC controls must include ${control}`);
    }
    assert.equal(oidc.requiresSecretOwner, true);
    assert.equal(oidc.rotationRunbookRequired, true);
    assert.equal(oidc.subjectRedactionRequired, true);
    assert.equal(oidc.unconfiguredProviderRejectionRequired, true);
    assert.equal(oidc.errorNormalizationRequired, true);
    assert.equal(oidc.productionLikeRehearsalRequired, true);
    assert.equal(oidc.rawCredentialExposureAllowed, false);
    assert.equal(oidc.rawSubjectExposureAllowed, false);
  });

  it("rejects OIDC promotion matrices that weaken Admin isolation or production evidence", () => {
    const contract = readJSON("resources/platform-production-auth-hardening.json");
    const oidc = contract.providerPromotionMatrix.providers.find((provider) => provider.id === "oidc");
    assert.ok(oidc, "providerPromotionMatrix must include oidc");
    oidc.capability = "session";
    oidc.kind = "saml";
    oidc.audiences = ["admin", "app"];
    oidc.configKeys = [
      "PLATFORM_ADMIN_OIDC_ISSUER_URL",
      "PLATFORM_ADMIN_OIDC_CLIENT_ID",
      "PLATFORM_ADMIN_OIDC_CLIENT_SECRET",
      "PLATFORM_ADMIN_OIDC_REDIRECT_URL",
      "PLATFORM_ADMIN_OIDC_SCOPES",
      "PLATFORM_ADMIN_OIDC_UNREVIEWED_EXTRA",
    ];
    oidc.requiredControls = ["issuer-validation"];
    oidc.requiresSecretOwner = false;
    oidc.rotationRunbookRequired = false;
    oidc.productionLikeRehearsalRequired = false;
    const contractPath = tempJSON("platform-production-auth-hardening.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /providerPromotionMatrix provider oidc capability must stay admin-oidc/);
    assert.match(result.stderr, /providerPromotionMatrix provider oidc kind must stay oidc/);
    assert.match(result.stderr, /providerPromotionMatrix provider oidc audiences must stay admin-only/);
    assert.match(result.stderr, /providerPromotionMatrix provider oidc configKeys must exactly match the approved Admin OIDC keys/);
    assert.match(result.stderr, /providerPromotionMatrix provider oidc requiredControls must include pkce-s256-validation/);
    assert.match(result.stderr, /providerPromotionMatrix provider oidc requiresSecretOwner must stay true/);
    assert.match(result.stderr, /providerPromotionMatrix provider oidc rotationRunbookRequired must stay true/);
    assert.match(result.stderr, /providerPromotionMatrix provider oidc productionLikeRehearsalRequired must stay true/);
  });

  it("rejects duplicate OIDC config keys that replace an approved key", () => {
    const contract = readJSON("resources/platform-production-auth-hardening.json");
    const oidc = contract.providerPromotionMatrix.providers.find((provider) => provider.id === "oidc");
    oidc.configKeys = [
      "PLATFORM_ADMIN_OIDC_ISSUER_URL",
      "PLATFORM_ADMIN_OIDC_CLIENT_ID",
      "PLATFORM_ADMIN_OIDC_CLIENT_SECRET",
      "PLATFORM_ADMIN_OIDC_REDIRECT_URL",
      "PLATFORM_ADMIN_OIDC_REDIRECT_URL",
    ];
    const contractPath = tempJSON("platform-production-auth-hardening.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /providerPromotionMatrix provider oidc configKeys must exactly match the approved Admin OIDC keys/);
  });

  it("rejects provider promotion matrices without source-backed evidence", () => {
    const contract = readJSON("resources/platform-production-auth-hardening.json");
    const wechat = contract.providerPromotionMatrix.providers.find((provider) => provider.id === "wechat");
    wechat.runtimeEvidence.push({ path: "internal/platform/authprovider/wechat/resolver.go", contains: "MissingWechatPromotionEvidence" });
    const contractPath = tempJSON("platform-production-auth-hardening.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /providerPromotionMatrix provider wechat runtime evidence internal\/platform\/authprovider\/wechat\/resolver\.go must include MissingWechatPromotionEvidence/);
  });

  it("rejects manifest-declared auth providers missing from the provider promotion matrix", () => {
    const audit = readJSON("resources/generated/platform-capability-audit.json");
    const session = audit.capabilities.find((capability) => capability.id === "session");
    session.authProviders.push("enterprise-sso");
    audit.authProviderCount += 1;
    const auditPath = tempJSON("platform-capability-audit.json", audit);

    const result = runValidator(["--capability-audit", auditPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /providerPromotionMatrix must include manifest-declared provider enterprise-sso/);
  });

  it("rejects provider promotion matrices without manifest coverage policy", () => {
    const contract = readJSON("resources/platform-production-auth-hardening.json");
    contract.providerPromotionMatrix.manifestCoverage = {
      source: "resources/missing-platform-capability-audit.json",
      required: false,
    };
    const contractPath = tempJSON("platform-production-auth-hardening.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /manifestCoverage\.source must stay resources\/generated\/platform-capability-audit\.json/);
    assert.match(result.stderr, /manifestCoverage\.required must stay true/);
    assert.match(result.stderr, /manifestCoverage\.policy is required/);
  });

  it("rejects production runtime policies that stop forbidding demo capabilities", () => {
    const contract = readJSON("resources/platform-production-auth-hardening.json");
    contract.providerRuntimePolicy.productionForbiddenCapabilities = [];
    contract.providerRuntimePolicy.productionForbiddenAuthProviders = [];
    const contractPath = tempJSON("platform-production-auth-hardening.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /providerRuntimePolicy.productionForbiddenCapabilities must include demo-data/);
    assert.match(result.stderr, /providerRuntimePolicy.productionForbiddenAuthProviders must include demo/);
  });

  it("accepts gofmt-aligned DisableDemoAuthProvider wiring", () => {
    const main = fs.readFileSync(path.join(repoRoot, "cmd/platform-api/main.go"), "utf8");
    const mainPath = tempText(
      "main.go",
      main.replace(/DisableDemoAuthProvider:\s*cfg\.DisableDemoAuthProvider\b/, "DisableDemoAuthProvider:\t\tcfg.DisableDemoAuthProvider"),
    );

    const result = runValidator(["--main-go", mainPath]);

    assert.equal(result.status, 0, result.stderr);
  });

  it("rejects API composition roots that omit DisableDemoAuthProvider wiring", () => {
    const main = fs.readFileSync(path.join(repoRoot, "cmd/platform-api/main.go"), "utf8");
    const mainPath = tempText(
      "main.go",
      main.replace(/\s*DisableDemoAuthProvider:\s*cfg\.DisableDemoAuthProvider,?/, ""),
    );

    const result = runValidator(["--main-go", mainPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /cmd\/platform-api\/main\.go must pass DisableDemoAuthProvider into httpapi\.ServerOptions/);
  });

  it("rejects auth audit policies that allow raw credential fields or drop runtime evidence", () => {
    const contract = readJSON("resources/platform-production-auth-hardening.json");
    contract.auditPolicy.allowedAuthAuditFields = ["actor", "action", "resource", "jwt", "sessionId"];
    contract.auditPolicy.sessionIdentifier = "rawSessionToken";
    contract.auditPolicy.runtimeEvidence = ["missing/server.go"];
    const contractPath = tempJSON("platform-production-auth-hardening.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /auditPolicy.allowedAuthAuditFields must not include forbidden raw field jwt/);
    assert.match(result.stderr, /auditPolicy.allowedAuthAuditFields must not include forbidden raw field sessionId/);
    assert.match(result.stderr, /auditPolicy.sessionIdentifier must stay none/);
    assert.match(result.stderr, /auditPolicy.runtimeEvidence path is missing or unsafe: missing\/server.go/);
  });

  it("accepts credential-free auth audits without a session identifier", () => {
    const contract = readJSON("resources/platform-production-auth-hardening.json");
    contract.auditPolicy.allowedAuthAuditFields = ["actor", "action", "resource", "targetId", "outcome", "eventId", "reasonCode", "createdAt"];
    contract.auditPolicy.sessionIdentifier = "none";
    const contractPath = tempJSON("platform-production-auth-hardening.json", contract);
    const review = readJSON("resources/generated/production-auth-promotion-review.json");
    review.sources.productionAuthHardening = path.relative(repoRoot, contractPath).split(path.sep).join("/");
    const reviewPath = tempJSON("production-auth-promotion-review.json", review);

    const result = runValidator(["--contract", contractPath, "--promotion-review", reviewPath]);

    assert.equal(result.status, 0, result.stderr);
  });

  it("rejects session and OIDC documentation that restores audit session identifiers", () => {
    const sessionPolicy = fs.readFileSync(path.join(repoRoot, "docs/platform-roadmap.md"), "utf8");
    const oidcDesign = fs.readFileSync(path.join(repoRoot, "docs/platform-roadmap.md"), "utf8");
    const adminResourceSchema = fs.readFileSync(path.join(repoRoot, "docs/admin-resource-schema.md"), "utf8");
    const sessionPolicyPath = tempText("session-policy.md", sessionPolicy
      .replace("Persisted session identifiers use the canonical `sha256:v1:` prefix followed by exactly 64 lowercase hexadecimal characters.", "Persisted session identifiers use a digest.")
      .replace("Audit records must not store the raw session handle, its digest, or any shortened derivative.", "Audit records may store a shortened session id."));
    const oidcDesignPath = tempText("oidc-design.md", oidcDesign.replace("OIDC audit records must not store the raw session handle, its digest, or any shortened derivative.", "OIDC audit records may store a shortened session id."));
    const adminResourceSchemaPath = tempText("admin-resource-schema.md", adminResourceSchema.replace("The audit schema does not expose `sessionId`", "The audit schema exposes `sessionId`"));

    const result = runValidator([
      "--session-policy-doc", sessionPolicyPath,
      "--oidc-design-doc", oidcDesignPath,
      "--admin-resource-schema-doc", adminResourceSchemaPath,
    ]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /session policy must define canonical sha256:v1 digests with 64 lowercase hexadecimal characters/);
    assert.match(result.stderr, /session policy must forbid raw session handles, digests and shortened derivatives in audits/);
    assert.match(result.stderr, /OIDC design must forbid raw session handles, digests and shortened derivatives in audits/);
    assert.match(result.stderr, /admin resource schema must state that audit schema has no sessionId field/);
  });

  it("accepts task graph closeout while refresh-token-family runtime remains disabled by default", () => {
    const contract = readJSON("resources/platform-production-auth-hardening.json");
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    const task = graph.tasks.find((item) => item.id === contract.taskGraph.taskId);
    task.status = "implemented";
    const graphPath = tempJSON("platform-foundation-task-graph.json", graph);

    const result = runValidator(["--task-graph", graphPath]);

    assert.equal(result.status, 0, result.stderr);
  });

  it("rejects preview task graph tracking when the contract stops allowing preview", () => {
    const contract = readJSON("resources/platform-production-auth-hardening.json");
    const graph = readJSON("resources/platform-foundation-task-graph.json");
    const task = graph.tasks.find((item) => item.id === contract.taskGraph.taskId);
    task.status = "preview";
    contract.taskGraph.allowedStatusesBeforePromotion = ["deferred", "planned"];
    const graphPath = tempJSON("platform-foundation-task-graph.json", graph);
    const contractPath = tempJSON("platform-production-auth-hardening.json", contract);

    const result = runValidator(["--contract", contractPath, "--task-graph", graphPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /production-auth-provider-hardening has unsupported status preview/);
  });

  it("rejects production readiness policies that stop covering token rotation", () => {
    const contract = readJSON("resources/platform-production-auth-hardening.json");
    const readiness = readJSON("resources/platform-production-readiness.json");
    readiness.operationPolicies = readiness.operationPolicies.filter((policy) => policy.id !== "token-rotation");
    const readinessPath = tempJSON("platform-production-readiness.json", readiness);
    const contractPath = tempJSON("platform-production-auth-hardening.json", contract);

    const result = runValidator(["--contract", contractPath, "--production-readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /production readiness must include operation policy token-rotation/);
  });
});
