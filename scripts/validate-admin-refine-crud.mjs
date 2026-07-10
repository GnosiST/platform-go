import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const root = dirname(dirname(fileURLToPath(import.meta.url)));
const consolePath = join(root, "admin/src/platform/resources/GenericResourceConsole.tsx");
const dataProviderPath = join(root, "admin/src/platform/refine/dataProvider.ts");

const failures = [];
const consoleSource = readFileSync(consolePath, "utf8");
const dataProviderSource = readFileSync(dataProviderPath, "utf8");

for (const hook of ["useList", "useCreate", "useUpdate", "useDelete"]) {
  if (!consoleSource.includes(hook)) {
    failures.push(`GenericResourceConsole must use Refine ${hook} for resource CRUD convergence.`);
  }
}

for (const directClient of ["queryAdminResource", "createAdminResource", "updateAdminResource", "deleteAdminResource"]) {
  if (consoleSource.includes(directClient)) {
    failures.push(`GenericResourceConsole must not call ${directClient} directly; route it through Refine dataProvider hooks.`);
  }
}

if (!dataProviderSource.includes("meta?.keywords")) {
  failures.push("Refine dataProvider getList must pass structured keyword search through meta.keywords.");
}

if (!dataProviderSource.includes("meta?.conditions")) {
  failures.push("Refine dataProvider getList must pass structured safe-query conditions through meta.conditions.");
}

if (failures.length > 0) {
  console.error("Admin Refine CRUD validation failed:");
  for (const failure of failures) {
    console.error(`- ${failure}`);
  }
  process.exit(1);
}

console.log("Admin Refine CRUD validation passed.");
