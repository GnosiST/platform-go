import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");

function read(relativePath) {
  return fs.readFileSync(path.join(repoRoot, relativePath), "utf8");
}

const graph = JSON.parse(read("resources/platform-foundation-task-graph.json"));
const implemented = graph.tasks.filter((task) => task.status === "implemented");
const unfinished = graph.tasks.filter((task) => task.status !== "implemented");
const summary = `${graph.tasks.length} total / ${implemented.length} implemented / ${unfinished.length} controlled unfinished`;
const publishedNodeIDs = [
  "open-source-portability",
  "public-docs-community",
  "public-docs-site",
  "github-release-publication",
];

describe("public documentation surface", () => {
  it("keeps public status summaries aligned with the task graph", () => {
    for (const relativePath of [
      "README.md",
      "README.en.md",
      "docs/platform-foundation-task-map.md",
      "docs/platform-data-governance-and-integrations-assessment.md",
      "docs/platform-roadmap.md",
    ]) {
      const source = read(relativePath);
      assert.ok(source.includes(summary), `${relativePath} must include ${summary}`);
      assert.doesNotMatch(source, /67 total \/ (?:52|54) implemented \/ (?:15|13) controlled unfinished|exact 13-node remainder|ordered 15-node remainder|14 unfinished nodes|All 15 remaining nodes|remaining 15 nodes/i);
    }

    for (const relativePath of ["README.md", "README.en.md", "docs/platform-roadmap.md"]) {
      const source = read(relativePath);
      for (const nodeID of publishedNodeIDs) {
        assert.match(source, new RegExp("`" + nodeID + "`[^.\\n]*`implemented`"));
      }
    }
  });

  it("renders the landing page through the shared Docusaurus layout", () => {
    const source = read("website/src/pages/index.tsx");

    assert.ok(source.includes("import Layout from '@theme/Layout';"));
    assert.ok(source.includes("<Layout title={pageTitle} description={pageDescription}>"));
    assert.ok(source.includes('<main className="platform-home">'));
    assert.match(source, /const pageTitle = text\(/);
    assert.match(source, /const pageDescription = text\(/);
  });
});
