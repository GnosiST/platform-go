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

function tempText(name, value) {
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-deployment-topology-"));
  const filePath = path.join(tempDir, name);
  fs.writeFileSync(filePath, value);
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
      (item) =>
        item !== "PLATFORM_CACHE_DRIVER" &&
        item !== "PLATFORM_DISABLE_DEMO_AUTH_PROVIDER" &&
        item !== "PLATFORM_PUBLIC_BASE_URL" &&
        item !== "PLATFORM_TRUSTED_PROXIES" &&
        item !== "PLATFORM_HTTP_MAX_BODY_BYTES" &&
        item !== "PLATFORM_RATE_LIMIT_HMAC_KEY" &&
        item !== "PLATFORM_EDGE_TRUSTED_PROXY",
    );
    contract.productionApiRequirements.forbiddenProductionCapabilities = [];
    const contractPath = tempJSON("platform-deployment-topology.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /productionApiRequirements\.requiredEnv must include PLATFORM_CACHE_DRIVER/);
    assert.match(result.stderr, /productionApiRequirements\.requiredEnv must include PLATFORM_DISABLE_DEMO_AUTH_PROVIDER/);
    assert.match(result.stderr, /productionApiRequirements\.requiredEnv must include PLATFORM_PUBLIC_BASE_URL/);
    assert.match(result.stderr, /productionApiRequirements\.requiredEnv must include PLATFORM_TRUSTED_PROXIES/);
    assert.match(result.stderr, /productionApiRequirements\.requiredEnv must include PLATFORM_HTTP_MAX_BODY_BYTES/);
    assert.match(result.stderr, /productionApiRequirements\.requiredEnv must include PLATFORM_RATE_LIMIT_HMAC_KEY/);
		assert.match(result.stderr, /productionApiRequirements\.requiredEnv must include PLATFORM_EDGE_TRUSTED_PROXY/);
    assert.match(result.stderr, /productionApiRequirements\.forbiddenProductionCapabilities must include demo-data/);
  });

  it("rejects deployment contracts that omit conditional reveal and step-up configuration", () => {
    const contract = readJSON("resources/platform-deployment-topology.json");
    contract.productionApiRequirements.conditionalEnv = [];
    const contractPath = tempJSON("platform-deployment-topology.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /productionApiRequirements\.conditionalEnv must include PLATFORM_SENSITIVE_REVEAL_HMAC_KEY/);
    assert.match(result.stderr, /productionApiRequirements\.conditionalEnv must include PLATFORM_ADMIN_STEP_UP_PHONE_VERIFIED_DIGEST_FIELD/);
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

  it("rejects active Nginx upload locations and upload directory aliases", () => {
    const current = fs.readFileSync(path.join(repoRoot, "deploy/nginx/platform.conf"), "utf8");
    const variants = [
      "location /uploads { proxy_pass http://platform_api; }",
      "location /private-files/ { alias /var/lib/platform-go/uploads; }",
      "location /private-files/ { root /srv/platform/uploads/; }",
    ];

    for (const directive of variants) {
      const nginxPath = tempText("platform.conf", `${current}\n${directive}\n`);
      const result = runValidator(["--admin-proxy", nginxPath]);

      assert.notEqual(result.status, 0, `${directive}\n${result.stdout}`);
      assert.match(result.stderr, /admin proxy must not expose upload storage/);
    }
  });

  it("rejects an Admin proxy without reviewed TLS edge controls", () => {
    const current = fs.readFileSync(path.join(repoRoot, "deploy/nginx/platform.conf"), "utf8");
    const unsafe = current
      .replace('  if ($platform_edge_https = 0) { return 308 ${PLATFORM_PUBLIC_BASE_URL}$request_uri; }\n', "")
      .replace('    proxy_set_header X-Forwarded-Proto $platform_forwarded_proto;\n', "")
      .replace('  add_header Strict-Transport-Security $platform_hsts always;\n', "")
      .replace(/  add_header Content-Security-Policy .*\n/, "");
    const nginxPath = tempText("platform.conf", unsafe);

    const result = runValidator(["--admin-proxy", nginxPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /admin proxy must redirect requests without the reviewed HTTPS edge signal/);
    assert.match(result.stderr, /admin proxy must forward only the normalized HTTPS edge signal/);
    assert.match(result.stderr, /admin proxy must emit HSTS and Content-Security-Policy/);
  });

  it("rejects an Admin proxy that does not rebuild a trusted canonical client IP", () => {
    const current = fs.readFileSync(path.join(repoRoot, "deploy/nginx/platform.conf"), "utf8");
    const unsafe = current
      .replace("set_real_ip_from ${PLATFORM_EDGE_TRUSTED_PROXY};\n", "")
      .replace("real_ip_header X-Forwarded-For;\n", "")
      .replace("real_ip_recursive on;\n", "")
      .replace("proxy_set_header X-Forwarded-For $remote_addr;", "proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;");
    const nginxPath = tempText("platform.conf", unsafe);

    const result = runValidator(["--admin-proxy", nginxPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /admin proxy must trust only PLATFORM_EDGE_TRUSTED_PROXY for real client IP/);
    assert.match(result.stderr, /admin proxy must overwrite X-Forwarded-For with one canonical client IP/);
  });

  it("rejects forwarded HTTPS that is not gated by the original trusted edge peer", () => {
    const current = fs.readFileSync(path.join(repoRoot, "deploy/nginx/platform.conf"), "utf8");
    const unsafe = current
      .replace("geo $realip_remote_addr $platform_edge_peer_trusted {\n", "geo $remote_addr $platform_edge_peer_trusted {\n")
      .replace('map "$platform_edge_peer_trusted:$http_x_forwarded_proto" $platform_forwarded_proto {', "map $http_x_forwarded_proto $platform_forwarded_proto {");
    const nginxPath = tempText("platform.conf", unsafe);

    const result = runValidator(["--admin-proxy", nginxPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /admin proxy must accept forwarded protocol only from PLATFORM_EDGE_TRUSTED_PROXY/);
  });

  it("rejects Host-derived redirects and unconditional HSTS", () => {
    const current = fs.readFileSync(path.join(repoRoot, "deploy/nginx/platform.conf"), "utf8");
    const unsafe = current
      .replace("return 308 ${PLATFORM_PUBLIC_BASE_URL}$request_uri;", "return 308 https://$host$request_uri;")
      .replace('add_header Strict-Transport-Security $platform_hsts always;', 'add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;');
    const nginxPath = tempText("platform.conf", unsafe);

    const result = runValidator(["--admin-proxy", nginxPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /admin proxy redirect must use PLATFORM_PUBLIC_BASE_URL instead of request Host/);
    assert.match(result.stderr, /admin proxy HSTS must be conditional on the trusted HTTPS edge signal/);
  });

  it("rejects case-insensitive Nginx edge signal maps", () => {
    const current = fs.readFileSync(path.join(repoRoot, "deploy/nginx/platform.conf"), "utf8");
    const unsafe = current
      .replace('~^https$ "https";', 'https "https";')
      .replace('~^http$ "http";', 'http "http";');
    const nginxPath = tempText("platform.conf", unsafe);

    const result = runValidator(["--admin-proxy", nginxPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /admin proxy must use case-sensitive canonical http and https edge signal regexes/);
  });

  it("renders only case-sensitive canonical edge signal patterns", () => {
    const source = fs.readFileSync(path.join(repoRoot, "deploy/nginx/platform.conf"), "utf8");
    const rendered = spawnSync("envsubst", ["${PLATFORM_PUBLIC_BASE_URL} ${PLATFORM_EDGE_TRUSTED_PROXY}"], {
      cwd: repoRoot,
      encoding: "utf8",
      env: {
        ...process.env,
        PLATFORM_PUBLIC_BASE_URL: "https://platform.example.test",
        PLATFORM_EDGE_TRUSTED_PROXY: "172.30.0.1",
      },
      input: source,
    });

    assert.equal(rendered.status, 0, rendered.stderr);
    assert.match(rendered.stdout, /~\^https\$ "https";/);
    assert.match(rendered.stdout, /~\^http\$ "http";/);
    assert.doesNotMatch(rendered.stdout, /~\*\^https\$/);
    assert.match(rendered.stdout, /return 308 https:\/\/platform\.example\.test\$request_uri;/);
    assert.match(rendered.stdout, /set_real_ip_from 172\.30\.0\.1;/);
    assert.match(rendered.stdout, /proxy_set_header X-Forwarded-For \$remote_addr;/);
    assert.doesNotMatch(rendered.stdout, /proxy_add_x_forwarded_for/);
    for (const value of ["HTTPS", "Https", "https,http", "https, https", "https http"]) {
      assert.equal(/^https$/.test(value), false, `${value} must not enable HTTPS or HSTS`);
    }
  });

  it("rejects bypassing the official Nginx template entrypoint", () => {
    const contract = readJSON("resources/platform-deployment-topology.json");
    contract.deploymentPackage.requiredSourceSnippets = contract.deploymentPackage.requiredSourceSnippets.filter(
      (item) => item.contains !== "COPY deploy/nginx/platform.conf /etc/nginx/templates/default.conf.template",
    );
    const contractPath = tempJSON("platform-deployment-topology.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /Admin image must install the Nginx config as an envsubst template/);
  });

  it("rejects a production API healthcheck outside the loopback exception", () => {
    const current = fs.readFileSync(path.join(repoRoot, "deploy/compose/docker-compose.prod.yml"), "utf8");
    const unsafe = current.replace("http://127.0.0.1:9200/api/health", "http://platform-api:9200/api/health");
    const composePath = tempText("docker-compose.prod.yml", unsafe);

    const result = runValidator(["--compose", composePath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /platform-api healthcheck must use the direct loopback HTTP exception/);
  });

  it("rejects a standard env template whose Admin proxy is outside trusted proxies", () => {
    const current = fs.readFileSync(path.join(repoRoot, "deploy/env/production.example.env"), "utf8");
    const unsafe = current.replace("PLATFORM_TRUSTED_PROXIES=172.30.0.10", "PLATFORM_TRUSTED_PROXIES=172.30.0.11");
    const envPath = tempText("production.example.env", unsafe);

    const result = runValidator(["--env-template", envPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /standard production env must trust PLATFORM_ADMIN_PROXY_IP/);
  });

  it("rejects a standard env template whose edge peer is a CIDR or outside the fixed internal subnet", () => {
    const current = fs.readFileSync(path.join(repoRoot, "deploy/env/production.example.env"), "utf8");
    for (const value of ["172.30.0.1/32", "192.0.2.1"]) {
      const unsafe = current.replace("PLATFORM_EDGE_TRUSTED_PROXY=172.30.0.1", `PLATFORM_EDGE_TRUSTED_PROXY=${value}`);
      const envPath = tempText("production.example.env", unsafe);

      const result = runValidator(["--env-template", envPath]);

      assert.notEqual(result.status, 0, result.stdout);
      assert.match(result.stderr, /standard production edge peer must be one IP contained in PLATFORM_INTERNAL_SUBNET/);
    }
  });

  it("rejects active Admin file-storage volume mounts in Compose", () => {
    const current = fs.readFileSync(path.join(repoRoot, "deploy/compose/docker-compose.prod.yml"), "utf8");
    const mounts = ["platform_uploads:/var/lib/platform-go/uploads:ro", "private_assets:/app/.platform/uploads:ro"];

    for (const mount of mounts) {
      const compose = current.replace("    ports:\n", `    volumes:\n      - ${mount}\n    ports:\n`);
      const composePath = tempText("docker-compose.prod.yml", compose);
      const result = runValidator(["--compose", composePath]);

      assert.notEqual(result.status, 0, `${mount}\n${result.stdout}`);
      assert.match(result.stderr, /Admin service must not mount file storage/);
    }
  });

  it("allows unrelated Admin volumes in Compose", () => {
    const current = fs.readFileSync(path.join(repoRoot, "deploy/compose/docker-compose.prod.yml"), "utf8");
    const compose = current.replace("    ports:\n", "    volumes:\n      - admin_cache:/var/cache/nginx\n    ports:\n");
    const composePath = tempText("docker-compose.prod.yml", compose);

    const result = runValidator(["--compose", composePath]);

    assert.equal(result.status, 0, result.stderr);
  });

  it("rejects Compose without a deterministic trusted-proxy network", () => {
    const current = fs.readFileSync(path.join(repoRoot, "deploy/compose/docker-compose.prod.yml"), "utf8");
    const compose = current.replace(/\nnetworks:\n[\s\S]*$/, "\n");
    const composePath = tempText("docker-compose.prod.yml", compose);

    const result = runValidator(["--compose", composePath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /compose default network must declare the reviewed PLATFORM_INTERNAL_SUBNET/);
  });

  it("rejects public upload environment mappings in any Compose service", () => {
    const current = fs.readFileSync(path.join(repoRoot, "deploy/compose/docker-compose.prod.yml"), "utf8");
    const compose = current.replace(
      "      PLATFORM_RUNTIME_ENV: ${PLATFORM_RUNTIME_ENV:?required}\n",
      "      PLATFORM_RUNTIME_ENV: ${PLATFORM_RUNTIME_ENV:?required}\n      PLATFORM_FILE_STORAGE_PUBLIC_URL: /uploads\n",
    );
    const composePath = tempText("docker-compose.prod.yml", compose);

    const result = runValidator(["--compose", composePath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /compose file must not configure PLATFORM_FILE_STORAGE_PUBLIC_URL/);
  });

  it("rejects production Compose without the shared rate-limit HMAC key", () => {
    const current = fs.readFileSync(path.join(repoRoot, "deploy/compose/docker-compose.prod.yml"), "utf8");
    const compose = current.replace("      PLATFORM_RATE_LIMIT_HMAC_KEY: ${PLATFORM_RATE_LIMIT_HMAC_KEY:?required}\n", "");
    const composePath = tempText("docker-compose.prod.yml", compose);

    const result = runValidator(["--compose", composePath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /platform-api must receive PLATFORM_RATE_LIMIT_HMAC_KEY/);
  });

  it("rejects production Compose without the retention runner control", () => {
    const current = fs.readFileSync(path.join(repoRoot, "deploy/compose/docker-compose.prod.yml"), "utf8");
    const composePath = tempText(
      "docker-compose.prod.yml",
      current.replace(/^\s+PLATFORM_RETENTION_RUNNER_ENABLED:.*\n/m, ""),
    );

    const result = runValidator(["--compose", composePath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /platform-api must receive PLATFORM_RETENTION_RUNNER_ENABLED/);
  });

  it("rejects production Compose without data protection mappings", () => {
    const current = fs.readFileSync(path.join(repoRoot, "deploy/compose/docker-compose.prod.yml"), "utf8");
    const mapping = "      PLATFORM_DATA_ENCRYPTION_KEYRING_JSON: ${PLATFORM_DATA_ENCRYPTION_KEYRING_JSON:?required}\n";
    const composePath = tempText("docker-compose.prod.yml", current.replace(mapping, ""));

    const result = runValidator(["--compose", composePath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /platform-api must receive PLATFORM_DATA_ENCRYPTION_KEYRING_JSON/);
  });

  it("rejects production Compose without API or Admin edge trust mappings", () => {
    const current = fs.readFileSync(path.join(repoRoot, "deploy/compose/docker-compose.prod.yml"), "utf8");
    const mapping = "      PLATFORM_EDGE_TRUSTED_PROXY: ${PLATFORM_EDGE_TRUSTED_PROXY:?required}\n";
    const apiMissing = current.replace(mapping, "");
    const adminMissing = current.replace(`${mapping}    ports:\n`, "    ports:\n");

    for (const [service, compose] of [["platform-api", apiMissing], ["platform-admin", adminMissing]]) {
      const composePath = tempText("docker-compose.prod.yml", compose);
      const result = runValidator(["--compose", composePath]);

      assert.notEqual(result.status, 0, result.stdout);
      assert.match(result.stderr, new RegExp(`${service} must receive PLATFORM_EDGE_TRUSTED_PROXY`));
    }
  });

  it("ignores commented Nginx and Compose upload examples", () => {
    const nginx = fs.readFileSync(path.join(repoRoot, "deploy/nginx/platform.conf"), "utf8");
    const nginxPath = tempText(
      "platform.conf",
      `${nginx}\n# location /uploads { alias /var/lib/platform-go/uploads; }\n# root /srv/platform/uploads;\n`,
    );
    const compose = fs.readFileSync(path.join(repoRoot, "deploy/compose/docker-compose.prod.yml"), "utf8");
    const composePath = tempText(
      "docker-compose.prod.yml",
      `${compose}\n# PLATFORM_FILE_STORAGE_PUBLIC_URL=/uploads\n# platform_uploads:/var/lib/platform-go/uploads:ro\n`,
    );

    const result = runValidator(["--admin-proxy", nginxPath, "--compose", composePath]);

    assert.equal(result.status, 0, result.stderr);
  });

  it("rejects malformed Compose YAML", () => {
    const composePath = tempText("docker-compose.prod.yml", "services:\n  platform-admin: [\n");

    const result = runValidator(["--compose", composePath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /compose file must be valid YAML/);
  });

  it("rejects topology contracts that declare a public upload alias", () => {
	const contract = readJSON("resources/platform-deployment-topology.json");
	contract.deploymentPackage.sameOrigin.uploadAlias = "/uploads/";
	const contractPath = tempJSON("platform-deployment-topology.json", contract);

	const result = runValidator(["--contract", contractPath]);

	assert.notEqual(result.status, 0, result.stdout);
	assert.match(result.stderr, /deploymentPackage\.sameOrigin must not declare uploadAlias/);
  });

  it("rejects production env templates without bounded private upload policy", () => {
	const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "platform-private-files-"));
	try {
	  const envPath = path.join(tempDir, "production.example.env");
	  fs.writeFileSync(
		envPath,
		[
		  "PLATFORM_RUNTIME_ENV=production",
		  "PLATFORM_CACHE_DRIVER=redis",
		  "PLATFORM_DISABLE_DEMO_AUTH_PROVIDER=true",
		  "PLATFORM_CAPABILITIES=tenant,identity,file-storage",
		  "PLATFORM_FILE_STORAGE_DRIVER=s3",
		  "",
		].join("\n"),
	  );

	  const result = runValidator(["--env-template", envPath]);

	  assert.notEqual(result.status, 0, result.stdout);
	  assert.match(result.stderr, /production env must configure PLATFORM_FILE_MAX_UPLOAD_BYTES/);
	  assert.match(result.stderr, /production env must configure PLATFORM_FILE_ALLOWED_MIME_TYPES/);
	  assert.match(result.stderr, /production env must configure PLATFORM_FILE_STORAGE_S3_SERVER_SIDE_ENCRYPTION/);
	} finally {
	  fs.rmSync(tempDir, { recursive: true, force: true });
	}
  });

  it("rejects deployment contracts that do not require the Admin operator CLI in the API image", () => {
    const contract = readJSON("resources/platform-deployment-topology.json");
    contract.deploymentPackage.requiredSourceSnippets = contract.deploymentPackage.requiredSourceSnippets.filter(
      (item) => !item.contains.includes("platform-admin") && !item.contains.includes('ENTRYPOINT ["platform-api"]'),
    );
    const contractPath = tempJSON("platform-deployment-topology.json", contract);

    const result = runValidator(["--contract", contractPath]);

    assert.notEqual(result.status, 0, result.stdout);
    assert.match(result.stderr, /deploymentPackage\.requiredSourceSnippets must require building \/out\/platform-admin/);
    assert.match(result.stderr, /deploymentPackage\.requiredSourceSnippets must require copying \/app\/platform-admin/);
    assert.match(result.stderr, /deploymentPackage\.requiredSourceSnippets must preserve platform-api as the default entrypoint/);
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
