import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const contract = JSON.parse(fs.readFileSync(path.join(root, "resources/platform-search-projection.json"), "utf8"));
const source = fs.readFileSync(path.join(root, "internal/platform/searchprojection/gorm_store.go"), "utf8");
const docs = fs.readFileSync(path.join(root, "docs/platform-search-projection.md"), "utf8");
const errors = [];

if (contract.version !== 1 || contract.input !== "versioned-outbox-envelope" || contract.mapping !== "server-registered-field-allowlist" || contract.tenantScope !== "trusted-envelope-tenant-only" || contract.idempotency !== "persistent-consumer-event-receipt" || contract.deletion !== "tombstone-and-adapter-delete" || contract.rebuild !== "bounded-relational-projection-replay" || contract.sourceOfTruth !== "relational-business-store" || contract.httpSurface !== "none") errors.push("search projection contract must retain registered mapping, tenant, replay and source-of-truth boundaries");
for (const snippet of ["func (s *GORMStore) Apply", "s.ledger.Apply", "func (s *GORMStore) Rebuild", "maxRebuildDocuments", "DeletedAt", "TenantID: envelope.TenantCode"]) if (!source.includes(snippet)) errors.push(`search projection source must include ${snippet}`);
for (const snippet of ["server-registered", "trusted tenant", "completed duplicate is skipped", "tombstone", "bounded", "relational business store remains authoritative"]) if (!docs.includes(snippet)) errors.push(`search projection docs must include ${snippet}`);
if (errors.length) { console.error(errors.map((item) => `- ${item}`).join("\n")); process.exit(1); }
console.log("Validated asynchronous search projection contract");
