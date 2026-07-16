import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";
import { fileURLToPath, pathToFileURL } from "node:url";
import path from "node:path";

const root = path.dirname(path.dirname(fileURLToPath(import.meta.url)));
const modelURL = pathToFileURL(path.join(root, "admin/src/platform/ui/treeTransferModel.ts")).href;

function runModel(testBody) {
  const body = `import assert from "node:assert/strict";\nimport { buildTreeTransferIndex, deriveTreeTransferSelection, leafValues, filteredNodeKeys } from ${JSON.stringify(modelURL)};\n${testBody}`;
  const result = spawnSync(process.execPath, ["--experimental-strip-types", "--input-type=module"], { input: body, encoding: "utf8", maxBuffer: 16 * 1024 * 1024 });
  assert.equal(result.status, 0, result.error?.message || result.stderr || result.stdout || `signal=${result.signal}`);
}

function fixture() {
  const nodes = [];
  for (let branch = 0; branch < 100; branch += 1) {
    const branchKey = `branch-${branch}`;
    nodes.push({ key: branchKey, kind: "branch", label: `Branch ${branch}` });
    for (let leaf = 0; leaf < 100; leaf += 1) {
      nodes.push({ key: `${branchKey}-leaf-${leaf}`, parentKey: branchKey, kind: "leaf", label: `Leaf ${branch}-${leaf}` });
    }
  }
  return nodes;
}

describe("tree transfer indexed model", () => {
  it("derives exact full and half branch state for 10,000 nodes and 2,000 selections", () => {
    const nodes = fixture();
    const selected = nodes.filter((node) => node.kind === "leaf" && Number(node.key.split("-").at(-1)) < 20).map((node) => node.key);
    const visible = new Set(nodes.map((node) => node.key));
    runModel(`
      const nodes = [];
      for (let branch = 0; branch < 100; branch += 1) {
        const branchKey = \`branch-\${branch}\`;
        nodes.push({ key: branchKey, kind: "branch", label: \`Branch \${branch}\` });
        for (let leaf = 0; leaf < 100; leaf += 1) nodes.push({ key: \`\${branchKey}-leaf-\${leaf}\`, parentKey: branchKey, kind: "leaf", label: \`Leaf \${branch}-\${leaf}\` });
      }
      const selected = nodes.filter((node) => node.kind === "leaf" && Number(node.key.split("-").at(-1)) < 20).map((node) => node.key);
      const index = buildTreeTransferIndex(nodes);
      assert.equal(index.byKey.size, 10100);
      assert.equal(index.leafKeys.size, 10000);
      assert.equal(index.childrenByParent.get("branch-0").length, 100);
      assert.equal(index.leafDescendantsByBranch.get("branch-0").length, 100);
      const result = deriveTreeTransferSelection(index, selected, new Set(nodes.map((node) => node.key)), index.leafKeys);
      assert.equal(result.checkedKeys.filter((key) => /^branch-\\d+$/.test(key)).length, 0);
      assert.equal(result.halfCheckedKeys.length, 100);
      assert.equal(result.checkedKeys.filter((key) => key.includes("-leaf-")).length, 2000);
    `);
  });

  it("keeps hidden selections out of visible checks while preserving them in normalized values", () => {
    const nodes = fixture();
    const selected = nodes.filter((node) => node.kind === "leaf" && (node.key.startsWith("branch-0-") || node.key === "branch-1-leaf-99")).map((node) => node.key);
    const visible = new Set(nodes.filter((node) => node.key.startsWith("branch-0")).map((node) => node.key));
    runModel(`
      const nodes = ${JSON.stringify(nodes)};
      const selected = ${JSON.stringify(selected)};
      const index = buildTreeTransferIndex(nodes);
      const visible = new Set(${JSON.stringify([...visible])});
      const result = deriveTreeTransferSelection(index, selected, visible, index.leafKeys);
      assert.equal(result.checkedKeys.includes("branch-1-leaf-99"), false);
      assert.equal(result.checkedKeys.includes("branch-0"), true);
      assert.equal(leafValues(index, selected).length, 101);
    `);
  });

  it("filters matching nodes and ancestors with one indexed walk", () => {
    const nodes = fixture();
    runModel(`
      const nodes = ${JSON.stringify(nodes)};
      const index = buildTreeTransferIndex(nodes);
      const keys = filteredNodeKeys(index, "Leaf 42-7");
      assert.equal(keys.has("branch-42"), true);
      assert.equal(keys.has("branch-42-leaf-7"), true);
      assert.equal(keys.size, 12);
      assert.equal(filteredNodeKeys(index, "").size, 10100);
    `);
  });
});
