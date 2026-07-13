# Mask Strategy Runtime Design

## Goal

Provide one manifest-driven masking runtime for arbitrary sensitive Admin resource fields. List, query/detail, overflow Tooltip and export projections must receive the same server-produced masked value without field-name inference or client-side security decisions.

## Decisions

- Field names are never security signals. A field opts in through explicit `sensitivity`, `storageMode`, `responseMode`, `exportMode`, `protection` and `masking` metadata.
- `masked` storage remains a legacy pass-through for values that were irreversibly masked before persistence.
- Encrypted fields may use `masked`, `privileged` or `omitted` projection. An encrypted `masked` projection requires a versioned masking policy.
- Ordinary masked projection decrypts only inside the backend process, immediately applies the selected strategy and returns only the masked result. Any missing runtime, invalid envelope, unsupported strategy or mask failure aborts the projection without plaintext fallback.
- The first strategy set is `partial-v1`, `phone-v1`, `email-v1`, `identity-cn-v1` and `address-cn-v1`. `partial-v1` accepts explicit prefix, suffix and fixed mask-length settings; the other strategies are stable presets. All strategies are Unicode rune aware and conceal at least one rune for non-empty input.
- Strategy identifiers are versioned. Changing presentation semantics requires a new strategy identifier rather than silently changing an existing one.
- Query and export paths must project once. Repeated projection is removed instead of making the masking runtime guess whether input is ciphertext or already masked text.
- Mutation response projection is preflighted before in-memory state or persistent snapshots change. A reveal or mask failure returns an error without committing the record or its audit entry.
- Updates preserve unsubmitted stored values, including read-only and omitted fields. The Admin client never masks or reveals values: it renders the server result, keeps encrypted edit fields blank, and sends only externally writable public values in unrelated status updates.
- Schema reads deep-clone masking metadata, and Go plus JavaScript validators accept only one visible graphic replacement rune.

## Boundaries

- No reveal HTTP endpoint, step-up verification, short-lived grant or plaintext browser state is added here.
- Existing pre-masked phone and provider-subject records remain compatible but cannot be revealed because their original values are not stored.
- Generic export availability is not expanded. This node governs `ProjectionExport` wherever an export exists; it does not claim CSV, XLSX, PDF or arbitrary-file export support.
- Masking policy is presentation metadata. Classification, normalization, envelope format and AAD rules remain protected-data migration boundaries.

## Acceptance

- Arbitrary field keys can declare a supported masking strategy without any phone, email, identity or address name check.
- Capability, generated Admin contract, OpenAPI extension and TypeScript types preserve the masking policy.
- Encrypted response and export projection produce the configured masked value and never return an envelope or plaintext.
- List/query/detail data and overflow Tooltip consume the same server value; query and policy-review export no longer double-project records.
- Editing or toggling a record does not send a masked encrypted value back as new plaintext.
- Editing or toggling a record preserves required hashes, storage keys and other values that are intentionally absent from the response.
- A failed mutation projection commits neither the record nor its audit event, and callers cannot mutate the Store's masking policy through a returned schema clone.
- Missing or invalid masking policy fails manifest validation and runtime projection closed.
