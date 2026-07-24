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
const implementedPublicNodeIDs = [
  "open-source-portability",
  "public-docs-community",
  "public-docs-site",
  "github-release-publication",
];
const publicDocIDs = [
  "getting-started",
  "business-integration",
  "authentication",
  "configuration",
  "production-checklist",
  "support-matrix",
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
      for (const nodeID of implementedPublicNodeIDs) {
        assert.match(source, new RegExp("`" + nodeID + "`[^.\\n]*`implemented`"));
      }
    }

    assert.match(read("README.md"), /v0\.1\.0 已正式发布/);
    assert.match(read("README.en.md"), /v0\.1\.0 is formally published/i);
    assert.match(read("docs/platform-roadmap.md"), /v0\.1\.0 is published and verified/i);
  });

  it("publishes v0.1.0 without advancing the version", () => {
    const adminPackage = JSON.parse(read("admin/package.json"));
    const websitePackage = JSON.parse(read("website/package.json"));
    const home = read("website/src/pages/index.tsx");
    const config = read("website/docusaurus.config.ts");
    const changelog = read("CHANGELOG.md");

    assert.equal(adminPackage.version, "0.1.0");
    assert.equal(websitePackage.version, "0.1.0");
    assert.match(home, /v0\.1\.0 (?:已发布|released)/);
    assert.doesNotMatch(home, /v0\.1\.[1-9]\d*/);
    assert.match(config, /label: 'v0\.1\.0', href: 'https:\/\/github\.com\/GnosiST\/platform-go\/releases\/tag\/v0\.1\.0'/);
    assert.match(changelog, /## \[Unreleased\][\s\S]*## \[0\.1\.0\] - 2026-07-16/);
    assert.doesNotMatch(changelog, /## \[0\.1\.1\]/);
  });

  it("renders the landing page through the shared Docusaurus layout", () => {
    const source = read("website/src/pages/index.tsx");

    assert.ok(source.includes("import Layout from '@theme/Layout';"));
    assert.ok(source.includes("<Layout title={pageTitle} description={pageDescription}>"));
    assert.ok(source.includes('<main className="platform-home">'));
    assert.match(source, /const pageTitle = text\(/);
    assert.match(source, /const pageDescription = text\(/);
  });

  it("keeps task-oriented bilingual docs and social metadata publishable", () => {
    const sidebar = read("website/sidebars.ts");
    const config = read("website/docusaurus.config.ts");
    const chineseRoadmap = read("website/docs/roadmap.md");
    const englishRoadmap = read("website/i18n/en/docusaurus-plugin-content-docs/current/roadmap.md");

    for (const docID of publicDocIDs) {
      assert.ok(fs.existsSync(path.join(repoRoot, "website/docs", `${docID}.md`)), `Chinese ${docID} doc must exist`);
      assert.ok(fs.existsSync(path.join(repoRoot, "website/i18n/en/docusaurus-plugin-content-docs/current", `${docID}.md`)), `English ${docID} doc must exist`);
      assert.match(sidebar, new RegExp(`'${docID}'`), `${docID} must appear in the sidebar`);
    }

    assert.ok(fs.existsSync(path.join(repoRoot, "website/static/img/platform-go-social.png")), "social image must exist");
    assert.match(config, /image: 'img\/platform-go-social\.png'/);
    for (const source of [chineseRoadmap, englishRoadmap]) {
      assert.match(source, /67 total \/ 67 implemented \/ 0 controlled unfinished/);
      assert.doesNotMatch(source, /future nodes|后续独立节点/);
    }
  });
});
