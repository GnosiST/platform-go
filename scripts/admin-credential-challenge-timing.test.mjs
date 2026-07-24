import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { pathToFileURL } from "node:url";
import path from "node:path";

const repoRoot = path.resolve(import.meta.dirname, "..");
const timingURL = pathToFileURL(path.join(repoRoot, "admin/src/platform/auth/credentialChallengeTiming.ts")).href;
const { credentialChallengeSecondsRemaining, formatCredentialChallengeCountdown } = await import(timingURL);

describe("credential challenge timing", () => {
  it("derives countdown seconds from the absolute deadline", () => {
    const now = Date.parse("2026-07-25T12:00:00.000Z");

    assert.equal(credentialChallengeSecondsRemaining("2026-07-25T12:00:01.001Z", now), 2);
    assert.equal(credentialChallengeSecondsRemaining("2026-07-25T12:00:00.001Z", now), 1);
    assert.equal(credentialChallengeSecondsRemaining("2026-07-25T12:00:00.000Z", now), 0);
    assert.equal(credentialChallengeSecondsRemaining("2026-07-25T11:59:59.999Z", now), 0);
  });

  it("formats stable minute and second columns", () => {
    assert.equal(formatCredentialChallengeCountdown(300), "5:00");
    assert.equal(formatCredentialChallengeCountdown(9), "0:09");
    assert.equal(formatCredentialChallengeCountdown(0), "0:00");
  });
});
