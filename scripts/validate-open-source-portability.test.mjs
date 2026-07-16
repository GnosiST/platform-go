import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";

const repoRoot = path.resolve(import.meta.dirname, "..");

function fixture(files) {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), "platform-open-source-"));
  for (const [name, contents] of Object.entries(files)) {
    const target = path.join(directory, name);
    fs.mkdirSync(path.dirname(target), { recursive: true });
    fs.writeFileSync(target, contents);
  }
  return directory;
}

function run(root, args = []) {
  return spawnSync(process.execPath, ["scripts/validate-open-source-portability.mjs", "--root", root, ...args], {
    cwd: repoRoot,
    encoding: "utf8",
  });
}

describe("validate-open-source-portability", () => {
  it("accepts a clean release fixture with the expected module", () => {
    const files = {
      "go.mod": "module github.com/GnosiST/platform-go\n",
      LICENSE: "Apache License\n",
      NOTICE: "GnosiST\n",
      "CONTRIBUTING.md": "# Contributing\n",
      "SECURITY.md": "# Security\n",
      "CODE_OF_CONDUCT.md": "# Code of Conduct\n",
      "SUPPORT.md": "# Support\n",
      "GOVERNANCE.md": "# Governance\n",
      "CHANGELOG.md": "# Changelog\n",
      "resources/reference-snapshot/manifest.json": JSON.stringify({
        root: "resources/reference-snapshot/zshenmez",
        files: ["docs/reference.md"],
      }),
      "resources/reference-snapshot/zshenmez/docs/reference.md": "reference\n",
    };
    const root = fixture(files);
    try {
      const result = run(root, ["--strict", "--expect-module", "github.com/GnosiST/platform-go"]);
      assert.equal(result.status, 0, result.stderr);
    } finally {
      fs.rmSync(root, { recursive: true, force: true });
    }
  });

  it("rejects high-confidence credentials", () => {
    const fakeToken = ["ghp_", "123456789012345678901234567890123456"].join("");
    const root = fixture({ "README.md": `token=${fakeToken}\n` });
    try {
      const result = run(root);
      assert.notEqual(result.status, 0);
      assert.match(result.stderr, /GitHub token detected/);
    } finally {
      fs.rmSync(root, { recursive: true, force: true });
    }
  });

  it("reports private paths as warnings by default and errors in strict mode", () => {
    const root = fixture({ "notes.md": "captured from /Users/alice/project\n" });
    try {
      const normal = run(root);
      assert.equal(normal.status, 0, normal.stderr);
      assert.match(normal.stderr, /machine-specific absolute path/);
      const strict = run(root, ["--strict-paths"]);
      assert.notEqual(strict.status, 0);
      assert.match(strict.stderr, /machine-specific absolute path/);
    } finally {
      fs.rmSync(root, { recursive: true, force: true });
    }
  });

  it("requires the release file set in strict mode", () => {
    const root = fixture({ "go.mod": "module github.com/GnosiST/platform-go\n" });
    try {
      const result = run(root, ["--strict"]);
      assert.notEqual(result.status, 0);
      assert.match(result.stderr, /required release file is missing: LICENSE/);
    } finally {
      fs.rmSync(root, { recursive: true, force: true });
    }
  });

  it("requires a tracked reference snapshot manifest in strict mode", () => {
    const root = fixture({
      "go.mod": "module github.com/GnosiST/platform-go\n",
      LICENSE: "Apache License\n",
      NOTICE: "GnosiST\n",
      "CONTRIBUTING.md": "# Contributing\n",
      "SECURITY.md": "# Security\n",
      "CODE_OF_CONDUCT.md": "# Code of Conduct\n",
      "SUPPORT.md": "# Support\n",
      "GOVERNANCE.md": "# Governance\n",
      "CHANGELOG.md": "# Changelog\n",
    });
    try {
      const result = run(root, ["--strict"]);
      assert.notEqual(result.status, 0);
      assert.match(result.stderr, /reference snapshot manifest is missing/);
    } finally {
      fs.rmSync(root, { recursive: true, force: true });
    }
  });
});
