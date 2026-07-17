import assert from "node:assert/strict";
import fs from "node:fs";
import test from "node:test";

const root = new URL("..", import.meta.url);
const dataProviderSource = fs.readFileSync(new URL("admin/src/platform/refine/dataProvider.ts", root), "utf8");
const resourceConsoleSource = fs.readFileSync(new URL("admin/src/platform/resources/GenericResourceConsole.tsx", root), "utf8");

test("ADM-010 preserves JSON-compatible schema values through provider normalization", () => {
  assert.match(dataProviderSource, /type SchemaValue\s*=/, "provider must define a JSON-compatible schema value boundary");
  assert.match(dataProviderSource, /isSchemaValueMap\(/, "provider must validate schema value maps without narrowing them to strings");
  assert.doesNotMatch(
    dataProviderSource,
    /if \(typeof value === "string"\) \{\s*input\.values/s,
    "provider must not discard non-string values from variables",
  );
});

test("ADM-010 keeps form schema values typed instead of joining or stringifying them", () => {
  assert.match(resourceConsoleSource, /type SchemaValue\s*=/, "console must define a JSON-compatible schema value boundary");
  assert.match(resourceConsoleSource, /schemaValueFromFormValue\(/, "form serialization must use schema-aware value normalization");
  assert.doesNotMatch(
    resourceConsoleSource,
    /const value = Array\.isArray\(raw\) \? raw\.join\(,\) : raw == null \? "" : String\(raw\);/,
    "form serialization must not flatten arrays and non-string values",
  );
});

test("ADM-011 loads relation options with server search and a bounded page", () => {
  assert.match(resourceConsoleSource, /RELATION_OPTION_PAGE_SIZE\s*=\s*\d+/, "relation loading must use a named bounded page size");
  assert.doesNotMatch(resourceConsoleSource, /pageSize:\s*100\b/, "relation loading must not use the old fixed first-100 request");
  assert.match(resourceConsoleSource, /keywords:\s*input\.search/, "relation search must be sent through the provider contract");
  assert.match(resourceConsoleSource, /selectedValues/, "relation loading must account for selected values outside the current page");
});

test("ADM-011 wires remote relation search into select controls", () => {
  assert.match(resourceConsoleSource, /onRelationSearch/, "relation controls must receive a remote search callback");
  assert.match(resourceConsoleSource, /filterOption=\{onRelationSearch \? false : undefined\}/, "relation selects must disable local-only filtering");
  assert.match(resourceConsoleSource, /onSearch=\{onRelationSearch\}/, "relation selects must trigger server search");
});
