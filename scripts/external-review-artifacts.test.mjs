import assert from "node:assert/strict";
import { describe, it } from "node:test";

import { isExternalReviewArtifactURI } from "./external-review-artifacts.mjs";

describe("external review artifact URI", () => {
  it("accepts project-scoped image evidence", () => {
    assert.equal(
      isExternalReviewArtifactURI("external-review-artifacts://platform-go/admin-ui/2026-07-16/evidence.png"),
      true,
    );
  });

  it("rejects unrelated, traversing or decorated evidence references", () => {
    for (const value of [
      "external-review-artifacts://other-project/admin-ui/2026-07-16/evidence.png",
      "external-review-artifacts://platform-go/admin-ui/../evidence.png",
      "external-review-artifacts://platform-go/admin-ui/2026-07-16/evidence.png?token=secret",
      "external-review-artifacts://platform-go/admin-ui/2026-07-16/evidence.txt",
      "https://example.test/evidence.png",
      null,
    ]) {
      assert.equal(isExternalReviewArtifactURI(value), false, String(value));
    }
  });
});
