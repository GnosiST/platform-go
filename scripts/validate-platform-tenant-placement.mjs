import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const read = (file) => fs.readFileSync(path.resolve(root, file), "utf8");
const contract = JSON.parse(read("resources/platform-tenant-placement.json"));
const source = read("internal/platform/tenantplacement/store.go");
const gorm = read("internal/platform/tenantplacement/gorm_store.go");
const docs = read("docs/platform-tenant-placement.md");
const errors = [];
if (contract.version !== 1 || contract.context !== "trusted-kernel-execution-context-only" || contract.clientPhysicalRouting !== "forbidden") errors.push("tenant placement contract must retain trusted-context and physical-routing boundary");
if (JSON.stringify(contract.placementKey) !== JSON.stringify(["tenantCode", "capabilityId"]) || contract.transition !== "version-monotonic-audited" || contract.override !== "authorized-exact-scope-max-15m-audited") errors.push("tenant placement contract must retain placement, transition and override boundaries");
for (const snippet of ["kernel.ExecutionContext", "PermissionOverride", "maximumOverrideTTL", "func (s *Store) Transition", "func (s *Store) Resolve"]) if (!source.includes(snippet)) errors.push(`tenant placement source must include ${snippet}`);
for (const snippet of ["PrepareGORMStore", "OpenGORMStore", "platform_tenant_placements", "platform_tenant_placement_audits", "Transaction(func(tx *gorm.DB)"]) if (!gorm.includes(snippet)) errors.push(`tenant placement persistence must include ${snippet}`);
for (const snippet of ["ExecutionContext", "fifteen minutes", "cannot supply a datasource"]) if (!docs.includes(snippet)) errors.push(`tenant placement docs must include ${snippet}`);
if (errors.length) { console.error(errors.map((item) => `- ${item}`).join("\n")); process.exit(1); }
console.log("Validated tenant placement contract");
