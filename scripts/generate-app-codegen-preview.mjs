import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, "..");
const contractPath = path.join(repoRoot, "resources", "generated", "app-route-contract.json");
const generatedDir = path.join(repoRoot, "resources", "generated");
const generatedPath = path.join(generatedDir, "app-codegen-preview.json");

const contract = JSON.parse(fs.readFileSync(contractPath, "utf8"));
const routes = contract.routes ?? [];

function pascalCase(value) {
  return String(value)
    .split(/[^a-zA-Z0-9]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join("");
}

function camelCase(...parts) {
  const value = pascalCase(parts.join(" "));
  return value.charAt(0).toLowerCase() + value.slice(1);
}

function routeAction(route) {
  const segments = route.path.replace(/^\/api\/app\/?/, "").split("/").filter(Boolean);
  const last = segments.at(-1) ?? "call";
  if (route.method === "GET" && last === "current") return "current";
  if (route.method === "DELETE") return "delete";
  if (route.method === "POST" && last === "logout") return "logout";
  if (route.method === "POST" && last === "login") return "login";
  if (route.method === "POST") return "create";
  if (route.method === "PUT" || route.method === "PATCH") return "update";
  return last;
}

function operationId(route) {
  const segments = route.path.replace(/^\/api\/app\/?/, "").split("/").filter(Boolean);
  return camelCase("app", segments.at(0) ?? route.capabilityId, routeAction(route));
}

function authModes() {
  return routes.reduce((counts, route) => {
    counts[route.auth] = (counts[route.auth] ?? 0) + 1;
    return counts;
  }, {});
}

function routeTarget(route) {
  return {
    capabilityId: route.capabilityId,
    method: route.method,
    path: route.path,
    auth: route.auth,
    permission: route.permission ?? "",
    operationId: operationId(route),
    securityDomain: "app",
    client: {
      basePath: "/api/app",
      tokenType: route.auth === "public" ? "none" : "app",
      apiClientFile: "future-app/src/platform/api/appClient.ts",
    },
    docs: {
      openapi: "resources/generated/openapi.app.json",
    },
  };
}

const previewRoutes = routes.map(routeTarget).sort((a, b) => `${a.path} ${a.method}`.localeCompare(`${b.path} ${b.method}`));

const preview = {
  generatedBy: "scripts/generate-app-codegen-preview.mjs",
  source: "resources/generated/app-route-contract.json",
  sourceVersion: contract.sourceVersion,
  securityDomain: "app",
  summary: {
    routeCount: previewRoutes.length,
    capabilities: contract.capabilities ?? [],
    permissions: contract.permissions ?? [],
    authModes: authModes(),
  },
  guardrails: [
    "This preview is read-only and must not overwrite app or business frontend files.",
    "Generated clients must stay behind the app API boundary; pages must not call request, upload or Authorization headers directly.",
    "App routes use tokenType=app; admin JWTs and pgo_ API tokens must not be accepted by generated app clients.",
    "Source-writing code generation remains deferred until route handlers and app identity binding are stable.",
  ],
  routes: previewRoutes,
};

const output = `${JSON.stringify(preview, null, 2)}\n`;
if (process.argv.includes("--stdout")) {
  process.stdout.write(output);
} else {
  fs.mkdirSync(generatedDir, { recursive: true });
  fs.writeFileSync(generatedPath, output);
  console.log(`Generated ${path.relative(repoRoot, generatedPath)}`);
}
