import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";
import { fileURLToPath, pathToFileURL } from "node:url";
import path from "node:path";

const root = path.dirname(path.dirname(fileURLToPath(import.meta.url)));

function runTypeScript(relativePath, body) {
  const moduleURL = pathToFileURL(path.join(root, relativePath)).href;
  const result = spawnSync(
    process.execPath,
    ["--experimental-strip-types", "--input-type=module", "--eval", body(moduleURL)],
    { encoding: "utf8", maxBuffer: 16 * 1024 * 1024 },
  );
  assert.equal(result.status, 0, result.error?.message || result.stderr || result.stdout || `signal=${result.signal}`);
}

describe("platform tree hierarchy projection", () => {
  it("keeps legacy menu leaves visible when their directory record is missing", () => {
    runTypeScript("admin/src/platform/resources/menuTreeProjection.ts", (moduleURL) => `
      import assert from "node:assert/strict";
      import { pageMenuCodes, projectMenuTreeNodes } from ${JSON.stringify(moduleURL)};

      const nodes = projectMenuTreeNodes(
        [{ code: "users", name: "Users", status: "enabled", nodeType: "page", parentCode: "identity" }],
        ["users"],
        { historicalLabel: "Historical", disabledReason: "Disabled", missingReason: "Missing" },
      );
      const directory = nodes.find((node) => node.kind === "branch" && node.label === "identity");
      const page = nodes.find((node) => node.key === "users");

      assert.ok(directory, "missing parent directory must be projected as a visible branch");
      assert.equal(page?.parentKey, directory.key);
      assert.deepEqual(pageMenuCodes(nodes, nodes.map((node) => node.key)), ["users"]);
    `);
  });

  it("keeps a dangling Transfer branch visible without changing half-selection", () => {
    const projectionURL = pathToFileURL(path.join(root, "admin/src/platform/ui/treeTransferProjection.ts")).href;
    runTypeScript("admin/src/platform/ui/treeTransferModel.ts", (modelURL) => `
      import assert from "node:assert/strict";
      import { buildTreeTransferIndex, deriveTreeTransferSelection } from ${JSON.stringify(modelURL)};
      import { treeTransferRootKeys } from ${JSON.stringify(projectionURL)};

      const nodes = [
        { key: "orphan-branch", parentKey: "missing-parent", kind: "branch", label: "Orphan" },
        { key: "leaf-a", parentKey: "orphan-branch", kind: "leaf", label: "A" },
        { key: "leaf-b", parentKey: "orphan-branch", kind: "leaf", label: "B" },
        { key: "orphan-leaf", parentKey: "missing-leaf-parent", kind: "leaf", label: "Orphan leaf" },
      ];
      const index = buildTreeTransferIndex(nodes);
      const roots = treeTransferRootKeys([...index.byKey.values()]);
      const selection = deriveTreeTransferSelection(index, ["leaf-a", "orphan-leaf"], new Set(index.byKey.keys()), index.leafKeys);

      assert.deepEqual(roots, ["orphan-branch", "orphan-leaf"]);
      assert.deepEqual(selection.checkedKeys, ["leaf-a", "orphan-leaf"]);
      assert.deepEqual(selection.halfCheckedKeys, ["orphan-branch"]);
    `);
  });

  it("hydrates ancestors for paged, searched, and selected tree relation records", () => {
    runTypeScript("admin/src/platform/resources/treeRelationAncestors.ts", (moduleURL) => `
      import assert from "node:assert/strict";
      import { hydrateTreeRelationAncestors } from ${JSON.stringify(moduleURL)};

      const records = [
        { code: "search-result", parentCode: "team" },
        { code: "selected-outside-page", parentCode: "team" },
      ];
      const available = new Map([
        ["team", { code: "team", parentCode: "root" }],
        ["root", { code: "root", parentCode: "" }],
      ]);
      const loaded = [];
      const result = await hydrateTreeRelationAncestors(
        records,
        (record) => record.code,
        (record) => record.parentCode,
        async (value) => {
          loaded.push(value);
          return available.get(value);
        },
      );

      assert.deepEqual(result.map((record) => record.code), ["search-result", "selected-outside-page", "team", "root"]);
      assert.deepEqual(loaded, ["team", "root"], "each missing ancestor must be loaded once");
    `);
  });
});
