import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const contract = JSON.parse(fs.readFileSync(path.join(root, "resources/platform-outbox.json"), "utf8"));
const source = ["internal/platform/outbox/gorm_store.go", "internal/platform/outbox/consumer_ledger.go", "internal/platform/outbox/saga.go", "internal/platform/integration/rabbitmq.go"].map((file) => fs.readFileSync(path.join(root, file), "utf8")).join("\n");
const docs = fs.readFileSync(path.join(root, "docs/platform-outbox.md"), "utf8");
const errors = [];

if (contract.version !== 1 || contract.defaultRuntime !== "disabled" || contract.envelope !== "versioned-validated-trace-context" || contract.delivery !== "gorm-transaction-lease-retry-dead-letter-replay" || contract.consumerIdempotency !== "persistent-consumer-event-receipt" || contract.saga !== "durable-audit-caller-owned-compensation" || contract.mqAdapter !== "rabbitmq-durable-topic-exchange") errors.push("outbox contract must retain default-off delivery, durable consumer receipts and caller-owned compensation");
for (const snippet of ["AppendInTransaction", "StatusDeadLetter", "func (l *ConsumerLedger) Apply", "platform_outbox_consumer_receipts", "func (r *SagaRunner) Run", "SagaStatusCompensated", "func (r *RabbitMQ) Publish"]) if (!source.includes(snippet)) errors.push(`outbox source must include ${snippet}`);
for (const snippet of ["same caller-owned GORM transaction", "ConsumerLedger.Apply", "Completed duplicate deliveries skip", "caller-owned compensation", "default-off"]) if (!docs.includes(snippet)) errors.push(`outbox docs must include ${snippet}`);
if (errors.length) { console.error(errors.map((item) => `- ${item}`).join("\n")); process.exit(1); }
console.log("Validated transactional outbox contract");
