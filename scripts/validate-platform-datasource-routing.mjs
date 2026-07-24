import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const contract = JSON.parse(fs.readFileSync(path.join(root, "resources/platform-datasource-routing.json"), "utf8"));
const source = fs.readFileSync(path.join(root, "internal/platform/datasource/routing.go"), "utf8");
const docs = fs.readFileSync(path.join(root, "docs/platform-datasource-routing.md"), "utf8");
const errors = [];
if (contract.version !== 1 || contract.writes !== "primary-only" || contract.transactions !== "primary-only" || contract.reads !== "healthy-lag-compliant-replica-or-primary" || contract.readAfterWrite !== "bounded-primary-sticky-after-commit" || contract.replicaFailure !== "fallback-primary") errors.push("datasource routing contract must retain primary, sticky and fallback boundaries");
for (const snippet of ["request.Operation == OperationWrite || request.Transaction", "health.Lag > r.policy.MaximumReplicaLag", "return r.runtime.DBForGroup(request.GroupID)", "func (r *Router) RecordWrite", "only after the selected primary transaction commits"]) if (!source.includes(snippet)) errors.push(`datasource routing source must include ${snippet}`);
for (const snippet of ["Writes and all transactions use the group primary", "fall back to primary", "successful primary commit"]) if (!docs.includes(snippet)) errors.push(`datasource routing docs must include ${snippet}`);
if (errors.length) { console.error(errors.map((item) => `- ${item}`).join("\n")); process.exit(1); }
console.log("Validated datasource routing contract");
