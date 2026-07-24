import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const contract = JSON.parse(fs.readFileSync(path.join(root, "resources/platform-xa.json"), "utf8"));
const source = fs.readFileSync(path.join(root, "internal/platform/xa/mysql_resource_manager.go"), "utf8") + fs.readFileSync(path.join(root, "internal/platform/xa/gorm_recovery_log.go"), "utf8");
const docs = fs.readFileSync(path.join(root, "docs/platform-xa.md"), "utf8");
const errors = [];
if (contract.version !== 1 || contract.defaultRuntime !== "disabled" || contract.certifiedAdapter !== "mysql-xa-two-instance-smoke" || contract.recovery !== "persistent-heuristic-log" || contract.businessDefault !== "outbox-or-saga") errors.push("XA contract must retain default-off, certified and recovery boundaries");
for (const snippet of ["XA START", "XA PREPARE", "func (m *MySQLResourceManager) Commit", "func (m *MySQLResourceManager) Rollback", "platform_xa_recovery_records"]) if (!source.includes(snippet)) errors.push(`XA source must include ${snippet}`);
for (const snippet of ["disabled by default", "two ephemeral MySQL instances", "persistent recovery log", "Outbox or Saga"]) if (!docs.includes(snippet)) errors.push(`XA docs must include ${snippet}`);
if (errors.length) { console.error(errors.map((item) => `- ${item}`).join("\n")); process.exit(1); }
console.log("Validated XA contract");
