import { readFileSync, existsSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const root = dirname(dirname(fileURLToPath(import.meta.url)));
const appPath = join(root, "admin/src/App.tsx");
const routePagePath = join(root, "admin/src/platform/refine/ResourceRoutePage.tsx");

const failures = [];

if (!existsSync(routePagePath)) {
  failures.push("admin/src/platform/refine/ResourceRoutePage.tsx must own schema-driven resource route pages.");
} else {
  const routePage = readFileSync(routePagePath, "utf8");
  if (!routePage.includes("useResourceParams(")) {
    failures.push("ResourceRoutePage must read Refine resource metadata with useResourceParams().");
  }
  if (!routePage.includes("useCan(")) {
    failures.push("ResourceRoutePage must guard read access with Refine useCan().");
  }
  if (!routePage.includes("GenericResourceConsole")) {
    failures.push("ResourceRoutePage must keep the existing GenericResourceConsole adapter until full Refine page migration.");
  }
}

const app = readFileSync(appPath, "utf8");
if (!app.includes("ResourceRoutePage")) {
  failures.push("App.tsx must render schema-driven routes through ResourceRoutePage.");
}
if (/function\s+RefineResourcePage\b/.test(app)) {
  failures.push("App.tsx must not keep the local RefineResourcePage adapter; move it to platform/refine.");
}

if (failures.length > 0) {
  console.error("Admin Refine runtime validation failed:");
  for (const failure of failures) {
    console.error(`- ${failure}`);
  }
  process.exit(1);
}

console.log("Admin Refine runtime validation passed.");
