import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const contract = JSON.parse(fs.readFileSync(path.join(root, "resources/platform-sharding.json"), "utf8"));
const source = fs.readFileSync(path.join(root, "internal/platform/sharding/gorm_controller.go"), "utf8");
const docs = fs.readFileSync(path.join(root, "docs/platform-sharding.md"), "utf8");
const errors = [];
if (contract.version !== 1 || contract.routing !== "trusted-tenant-capability-versioned-placement" || contract.globalID !== "cryptographic-gid" || contract.migration !== "persistent-checkpoint-audit-compensation" || contract.clientPhysicalRouting !== "forbidden") errors.push("sharding contract must retain trusted routing and persistent migration boundaries");
for (const snippet of ["func PrepareGORMController", "func (c *GORMController) Checkpoint", "checkpoint.Sequence <= migration.Checkpoint.Sequence", "func (c *GORMController) Compensate", "platform_shard_migration_audits"]) if (!source.includes(snippet)) errors.push(`sharding source must include ${snippet}`);
for (const snippet of ["trusted tenant context", "monotonic", "same GORM transaction", "does not claim a distributed transaction"]) if (!docs.includes(snippet)) errors.push(`sharding docs must include ${snippet}`);
if (errors.length) { console.error(errors.map((item) => `- ${item}`).join("\n")); process.exit(1); }
console.log("Validated sharding contract");
