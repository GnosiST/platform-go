import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");

function runValidator(args = []) {
  return spawnSync(process.execPath, ["scripts/validate-platform-deployment-topology.mjs", ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

function readJSON(relativePath) {
  return JSON.parse(fs.readFileSync(path.join(repoRoot, relativePath), "utf8"));
}

function tempJSON(name, value) {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-deployment-topology-"));
  const filePath = path.join(tempDir, name);
  fs.writeFileSync(filePath, `${JSON.stringify(value, null, 2)}\n`);
  return filePath;
}

describe("validate-platform-deployment-topology", () => {
  it("accepts the current deployment topology contract", () => {
    const result = runValidator();

    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /Validated platform deployment topology/);
  });

  it("rejects making Vercel mandatory for the platform foundation", () => {
    const contract = readJSON("resources/platform-deployment-topology.json");
    contract.decision.vercelRequired = true;
    contract.vercelPolicy.admin.required = true;
    const contractPath = tempJSON("platform-deployment-topology.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /decision\.vercelRequired must stay false/);
    assert.match(result.stderr, /vercelPolicy\.admin\.required must stay false/);
  });

  it("rejects promoting Vercel Go runtime as the default API deployment", () => {
    const contract = readJSON("resources/platform-deployment-topology.json");
    contract.decision.defaultApiRuntime = "vercel-go-runtime";
    const fullstack = contract.topologies.find((item) => item.id === "fullstack-vercel-go-runtime");
    fullstack.status = "recommended";
    fullstack.api.defaultDeployment = true;
    const contractPath = tempJSON("platform-deployment-topology.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /decision\.defaultApiRuntime must stay long-lived-service/);
    assert.match(result.stderr, /fullstack-vercel-go-runtime status must stay not-default/);
    assert.match(result.stderr, /fullstack-vercel-go-runtime api\.defaultDeployment must stay false/);
  });

  it("rejects selecting the optional Vercel split topology as the default scheme A deployment", () => {
    const contract = readJSON("resources/platform-deployment-topology.json");
    contract.decision.selectedTopology = "split-admin-vercel-api-service";
    contract.deploymentPackage.selectedTopology = "split-admin-vercel-api-service";
    const contractPath = tempJSON("platform-deployment-topology.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /decision\.selectedTopology must stay single-service-production/);
    assert.match(result.stderr, /deploymentPackage\.selectedTopology must stay single-service-production/);
  });

  it("rejects deployment contracts that omit production runtime requirements", () => {
    const contract = readJSON("resources/platform-deployment-topology.json");
    contract.productionApiRequirements.requiredEnv = contract.productionApiRequirements.requiredEnv.filter(
      (item) => item !== "PLATFORM_CACHE_DRIVER" && item !== "PLATFORM_DISABLE_DEMO_AUTH_PROVIDER",
    );
    contract.productionApiRequirements.forbiddenProductionCapabilities = [];
    const contractPath = tempJSON("platform-deployment-topology.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /productionApiRequirements\.requiredEnv must include PLATFORM_CACHE_DRIVER/);
    assert.match(result.stderr, /productionApiRequirements\.requiredEnv must include PLATFORM_DISABLE_DEMO_AUTH_PROVIDER/);
    assert.match(result.stderr, /productionApiRequirements\.forbiddenProductionCapabilities must include demo-data/);
  });

  it("rejects deployment packages that drop the standard production files", () => {
    const contract = readJSON("resources/platform-deployment-topology.json");
    contract.deploymentPackage.dockerfile = "missing.Dockerfile";
    contract.deploymentPackage.dockerTargets.api = "vercel-go-runtime";
    const contractPath = tempJSON("platform-deployment-topology.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /deploymentPackage\.dockerfile must stay Dockerfile/);
    assert.match(result.stderr, /deploymentPackage\.dockerfile path is missing or unsafe/);
    assert.match(result.stderr, /deploymentPackage\.dockerTargets\.api must stay api/);
  });

  it("rejects deployment contracts that drop the Vercel admin-only adapter template", () => {
    const contract = readJSON("resources/platform-deployment-topology.json");
    contract.vercelPolicy.admin.adapterTemplate = "missing.vercel.json";
    contract.vercelPolicy.admin.adapterScope = "fullstack";
    const contractPath = tempJSON("platform-deployment-topology.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /vercelPolicy\.admin\.adapterTemplate must be deploy\/vercel\/admin\.vercel\.json/);
    assert.match(result.stderr, /vercelPolicy\.admin\.adapterScope must stay admin-static-only/);
    assert.match(result.stderr, /vercel admin adapter template path is missing or unsafe/);
  });

  it("rejects weakening the Vercel admin adapter package boundary", () => {
    const contract = readJSON("resources/platform-deployment-topology.json");
    contract.vercelPolicy.admin.adapterPackage.status = "default";
    contract.vercelPolicy.admin.adapterPackage.template = "admin/vercel.json";
    contract.vercelPolicy.admin.adapterPackage.copyTarget = "vercel.json";
    contract.vercelPolicy.admin.adapterPackage.installation = "always-install";
    contract.vercelPolicy.admin.adapterPackage.defaultIncludedInProduction = true;
    contract.vercelPolicy.admin.adapterPackage.apiBindingModes = ["api-rewrite"];
    contract.vercelPolicy.admin.adapterPackage.forbiddenRuntimeWiring = ["functions"];
    const contractPath = tempJSON("platform-deployment-topology.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /vercelPolicy\.admin\.adapterPackage\.status must stay implemented/);
    assert.match(result.stderr, /vercelPolicy\.admin\.adapterPackage\.template must match vercelPolicy\.admin\.adapterTemplate/);
    assert.match(result.stderr, /vercelPolicy\.admin\.adapterPackage\.copyTarget must stay admin\/vercel\.json/);
    assert.match(result.stderr, /vercelPolicy\.admin\.adapterPackage\.installation must stay copy-into-admin-project-only-when-vercel-is-selected/);
    assert.match(result.stderr, /vercelPolicy\.admin\.adapterPackage\.defaultIncludedInProduction must stay false/);
    assert.match(result.stderr, /vercelPolicy\.admin\.adapterPackage\.apiBindingModes must include absolute-api-base-env/);
    assert.match(result.stderr, /vercelPolicy\.admin\.adapterPackage\.forbiddenRuntimeWiring must include vercel-go-runtime/);
  });

  it("rejects Vercel admin adapter templates that include API runtime wiring", () => {
    const tempDir = fs.mkdtempSync(path.join(repoRoot, "tmp", "deployment-topology-vercel-test-"));
    try {
      const templatePath = path.join(tempDir, "admin.vercel.json");
      fs.writeFileSync(
        templatePath,
        `${JSON.stringify(
          {
            framework: "vite",
            buildCommand: "npm run build",
            outputDirectory: "dist",
            rewrites: [
              {
                source: "/api/(.*)",
                destination: "https://api.example.com/api/$1",
              },
              {
                source: "/(.*)",
                destination: "/index.html",
              },
            ],
            functions: {
              "api/*.go": {
                runtime: "go1.x",
              },
            },
            env: {
              PLATFORM_API_RUNTIME: "cmd/platform-api go build @vercel/go vercel-go-runtime",
            },
          },
          null,
          2,
        )}\n`,
      );
      const contract = readJSON("resources/platform-deployment-topology.json");
      contract.vercelPolicy.admin.adapterTemplate = path.relative(repoRoot, templatePath);
      const contractPath = tempJSON("platform-deployment-topology.json", contract);

      const result = runValidator(["--contract", contractPath]);

      assert.notEqual(result.status, 0, result.stdout);
      assert.match(result.stderr, /vercel admin adapter must not declare API rewrites/);
      assert.match(result.stderr, /vercel admin adapter must not declare functions/);
      assert.match(result.stderr, /vercel admin adapter must not include API runtime snippet cmd\/platform-api/);
      assert.match(result.stderr, /vercel admin adapter must not include API runtime snippet go build/);
      assert.match(result.stderr, /vercel admin adapter must not include API runtime snippet @vercel\/go/);
      assert.match(result.stderr, /vercel admin adapter must not include API runtime snippet vercel-go-runtime/);
    } finally {
      fs.rmSync(tempDir, { recursive: true, force: true });
    }
  });

  it("rejects production env templates that enable demo data", () => {
    const tempDir = fs.mkdtempSync(path.join(repoRoot, "tmp", "deployment-topology-test-"));
    try {
      const envPath = path.join(tempDir, "production.example.env");
      fs.writeFileSync(
        envPath,
        [
          "PLATFORM_RUNTIME_ENV=production",
          "PLATFORM_CACHE_DRIVER=redis",
          "PLATFORM_DISABLE_DEMO_AUTH_PROVIDER=true",
          "PLATFORM_CAPABILITIES=tenant,demo-data,identity",
          "",
        ].join("\n"),
      );
      const contract = readJSON("resources/platform-deployment-topology.json");
      contract.deploymentPackage.envTemplate = path.relative(repoRoot, envPath);
      const contractPath = tempJSON("platform-deployment-topology.json", contract);

      const result = runValidator(["--contract", contractPath]);

      assert.notEqual(result.status, 0, result.stdout);
      assert.match(result.stderr, /deploymentPackage\.envTemplate must stay deploy\/env\/production\.example\.env/);
      assert.match(result.stderr, /deploymentPackage\.envTemplate PLATFORM_CAPABILITIES must not include demo-data/);
    } finally {
      fs.rmSync(tempDir, { recursive: true, force: true });
    }
  });

  it("rejects production readiness without the deployment topology preflight", () => {
    const readiness = readJSON("resources/platform-production-readiness.json");
    readiness.preflightCommands = readiness.preflightCommands.filter((item) => item.id !== "deployment-topology");
    const readinessPath = tempJSON("platform-production-readiness.json", readiness);

    const result = runValidator(["--readiness", readinessPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /production readiness preflight must include deployment-topology/);
  });

  it("rejects engineering matrices that do not cite the deployment topology gate", () => {
    const matrix = readJSON("resources/platform-engineering-capabilities.json");
    matrix.capabilities = matrix.capabilities.filter((item) => item.id !== "deployment-topology-gate");
    const matrixPath = tempJSON("platform-engineering-capabilities.json", matrix);

    const result = runValidator(["--matrix", matrixPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /engineering capabilities must include deployment-topology-gate/);
  });
});
