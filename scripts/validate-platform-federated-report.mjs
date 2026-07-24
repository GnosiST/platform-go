import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const contract = JSON.parse(fs.readFileSync(path.join(root, "resources/platform-federated-report.json"), "utf8"));
const source = fs.readFileSync(path.join(root, "internal/platform/federatedreport/gorm_executor.go"), "utf8") + fs.readFileSync(path.join(root, "internal/platform/federatedreport/gorm_store.go"), "utf8");
const docs = fs.readFileSync(path.join(root, "docs/platform-federated-report.md"), "utf8");
const errors = [];
if (contract.version !== 1 || contract.definitions !== "persisted-versioned-server-registered" || contract.execution !== "read-only-registered-gorm-sources" || contract.tenantScope !== "trusted-context-required" || contract.queryInput !== "report-id-and-version-only" || contract.resultProjection !== "allowlisted-fields-only") errors.push("federated report contract must retain registered read-only boundaries");
for (const snippet of ["func PrepareGORMDefinitionStore", "func (s *GORMDefinitionStore) Runtime", "Where(\"tenant_code = ?\"", "Select(strings.Join(e.binding.Fields", "NewGORMSourceExecutors"]) if (!source.includes(snippet)) errors.push(`federated source must include ${snippet}`);
for (const snippet of ["cannot provide SQL", "trusted tenant context", "relational stores remain authoritative"]) if (!docs.includes(snippet)) errors.push(`federated docs must include ${snippet}`);
if (errors.length) { console.error(errors.map((item) => `- ${item}`).join("\n")); process.exit(1); }
console.log("Validated federated report contract");
