import assert from "node:assert/strict";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, it } from "node:test";
import { pathToFileURL } from "node:url";

const repoRoot = path.resolve(import.meta.dirname, "..");
const moduleURL = pathToFileURL(path.join(repoRoot, "admin/src/platform/security/sensitiveRevealOIDC.ts")).href;

function runProbe(body) {
  return spawnSync(
    process.execPath,
    ["--experimental-strip-types", "--input-type=module", "--eval", body],
    { cwd: repoRoot, encoding: "utf8" },
  );
}

describe("sensitive reveal OIDC transaction", () => {
  it("uses an isolated Web Crypto PKCE transaction and validates the authorization boundary", () => {
    const result = runProbe(`
      import assert from "node:assert/strict";
      import {
        beginSensitiveRevealOIDC,
        SENSITIVE_REVEAL_OIDC_PENDING_KEY,
      } from ${JSON.stringify(moduleURL)};

      assert.notEqual(SENSITIVE_REVEAL_OIDC_PENDING_KEY, "platform.auth.oidc.pending");
      const values = new Map();
      const events = [];
      const storage = {
        getItem: (key) => values.get(key) ?? null,
        setItem: (key, value) => { events.push("store"); values.set(key, value); },
        removeItem: (key) => values.delete(key),
      };
      const input = {
        challengeId: "challenge-1",
        challengeToken: "challenge-token",
        challengeExpiresAt: "2030-01-02T00:00:00.000Z",
        resource: "people",
        recordId: "person-1",
        field: "identityNumber",
        purpose: "support-case",
        provider: "oidc",
        returnPath: "/people/person-1?tab=details",
        completedFactors: ["admin-sms-otp-v1"],
      };
      await beginSensitiveRevealOIDC(input, async (startInput) => {
        events.push("start");
        assert.deepEqual({ ...startInput, codeChallenge: undefined }, {
          challengeId: "challenge-1",
          challengeToken: "challenge-token",
          resource: "people",
          recordId: "person-1",
          field: "identityNumber",
          purpose: "support-case",
          provider: "oidc",
          codeChallenge: undefined,
        });
        assert.match(startInput.codeChallenge, /^[A-Za-z0-9_-]{43}$/);
        return {
          challengeId: "challenge-1",
          transactionToken: "transaction-token",
          authorizationUrl: "https://id.example/authorize",
          state: "state-exact",
          expiresAt: "2030-01-01T00:00:00.000Z",
        };
      }, {
        allowLoopbackHTTP: false,
        storage,
        now: () => Date.parse("2029-01-01T00:00:00.000Z"),
        crypto: {
          getRandomValues: (bytes) => { events.push("random"); bytes.fill(1); return bytes; },
          subtle: {
            digest: async (algorithm, bytes) => {
              events.push("digest");
              assert.equal(algorithm, "SHA-256");
              assert.equal(bytes.byteLength, 43);
              return Uint8Array.from({ length: 32 }, () => 2).buffer;
            },
          },
        },
        navigate: (url) => { events.push("navigate"); assert.equal(url, "https://id.example/authorize"); },
      });
      assert.deepEqual(events, ["random", "digest", "start", "store", "navigate"]);
      const pending = JSON.parse(values.get(SENSITIVE_REVEAL_OIDC_PENDING_KEY));
      assert.deepEqual(Object.keys(pending).sort(), [
        "challengeExpiresAt", "challengeId", "challengeToken", "codeVerifier", "completedFactors", "expiresAt", "field", "provider",
        "purpose", "recordId", "resource", "returnPath", "state", "transactionToken",
      ]);
      assert.equal(pending.codeVerifier.length, 43);
      assert.equal("value" in pending, false);
      assert.equal("grantToken" in pending, false);

      for (const authorizationUrl of ["not a URL", "javascript:alert(1)", "http://id.example/authorize", "ftp://127.0.0.1/authorize"]) {
        const rejectedEvents = [];
        await assert.rejects(
          beginSensitiveRevealOIDC(input, async () => ({
            challengeId: "challenge-1",
            transactionToken: "transaction-token",
            authorizationUrl,
            state: "state",
            expiresAt: "2030-01-01T00:00:00.000Z",
          }), {
            allowLoopbackHTTP: false,
            storage: { ...storage, setItem: () => rejectedEvents.push("store") },
            now: () => Date.parse("2029-01-01T00:00:00.000Z"),
            crypto: {
              getRandomValues: (bytes) => bytes,
              subtle: { digest: async () => new Uint8Array(32).buffer },
            },
            navigate: () => rejectedEvents.push("navigate"),
          }),
          (error) => error instanceof Error && error.message === "OIDC authorization URL is not trusted" && !error.message.includes(authorizationUrl),
        );
        assert.deepEqual(rejectedEvents, []);
      }
    `);

    assert.equal(result.status, 0, result.stderr || result.stdout);
  });

  it("cleans callback parameters before consuming strict pending state and returns completion context", () => {
    const result = runProbe(`
      import assert from "node:assert/strict";
      import {
        clearPendingSensitiveRevealOIDC,
        consumePendingSensitiveRevealOIDC,
        hasPendingSensitiveRevealOIDC,
        SensitiveRevealOIDCCallbackError,
        SENSITIVE_REVEAL_OIDC_PENDING_KEY,
      } from ${JSON.stringify(moduleURL)};

      const validPending = {
        challengeId: "challenge-1",
        challengeToken: "challenge-token",
        challengeExpiresAt: "2030-01-02T00:00:00.000Z",
        resource: "people",
        recordId: "person-1",
        field: "identityNumber",
        purpose: "support-case",
        returnPath: "/people/person-1?tab=details",
        completedFactors: ["admin-sms-otp-v1"],
        transactionToken: "transaction-token",
        provider: "oidc",
        state: "state-exact",
        codeVerifier: "verifier",
        expiresAt: "2030-01-01T00:00:00.000Z",
      };
      const makeScenario = (pending = validPending) => {
        const values = new Map([[SENSITIVE_REVEAL_OIDC_PENDING_KEY, JSON.stringify(pending)]]);
        const events = [];
        const completeInputs = [];
        const storage = {
          getItem: (key) => { events.push("read"); return values.get(key) ?? null; },
          setItem: (key, value) => values.set(key, value),
          removeItem: (key) => { events.push("remove"); values.delete(key); },
        };
        return {
          events,
          values,
          storage,
          completeInputs,
          complete: async (input) => { events.push("complete"); completeInputs.push(input); return { policySatisfied: true, grantToken: "grant" }; },
          options: {
            storage,
            cleanupURL: () => events.push("cleanup"),
            now: () => Date.parse("2029-01-01T00:00:00.000Z"),
          },
        };
      };

      const noCallback = makeScenario();
      assert.equal(await consumePendingSensitiveRevealOIDC("?tab=details", noCallback.complete, noCallback.options), null);
      assert.deepEqual(noCallback.events, []);

      for (const search of ["?error=access_denied&state=state-exact", "?error=&state=state-exact", "?code=one&code=two&state=state-exact", "?code=one&state=a&state=b"]) {
        const scenario = makeScenario();
        await assert.rejects(
          consumePendingSensitiveRevealOIDC(search, scenario.complete, scenario.options),
          (error) => error instanceof SensitiveRevealOIDCCallbackError && error.reason === "callback",
        );
        assert.deepEqual(scenario.events, ["cleanup", "read", "remove"]);
      }

      for (const pending of [
        { ...validPending, grantToken: "must-not-be-stored" },
        { ...validPending, completedFactors: ["admin-sms-otp-v1", 1] },
        { ...validPending, completedFactors: ["unknown-factor"] },
        { ...validPending, codeVerifier: "" },
        { ...validPending, challengeExpiresAt: "not-a-date" },
        { ...validPending, expiresAt: "not-a-date" },
        { ...validPending, returnPath: "https://evil.example/collect" },
      ]) {
        const scenario = makeScenario(pending);
        await assert.rejects(
          consumePendingSensitiveRevealOIDC("?code=code&state=state-exact", scenario.complete, scenario.options),
          (error) => error instanceof SensitiveRevealOIDCCallbackError && error.reason === "transaction",
        );
        assert.deepEqual(scenario.events, ["cleanup", "read", "remove"]);
      }

      const stateMismatch = makeScenario();
      await assert.rejects(
        consumePendingSensitiveRevealOIDC("?code=code&state=wrong", stateMismatch.complete, stateMismatch.options),
        (error) => error instanceof SensitiveRevealOIDCCallbackError && error.reason === "state",
      );
      assert.deepEqual(stateMismatch.events, ["cleanup", "read", "remove"]);

      const expired = makeScenario();
      expired.options.now = () => Date.parse("2031-01-01T00:00:00.000Z");
      await assert.rejects(
        consumePendingSensitiveRevealOIDC("?code=code&state=state-exact", expired.complete, expired.options),
        (error) => error instanceof SensitiveRevealOIDCCallbackError && error.reason === "expired",
      );
      assert.deepEqual(expired.events, ["cleanup", "read", "remove"]);

      const success = makeScenario();
      const completed = await consumePendingSensitiveRevealOIDC("?code=code-exact&state=state-exact", success.complete, success.options);
      assert.deepEqual(success.events, ["cleanup", "read", "remove", "complete"]);
      assert.deepEqual(success.completeInputs, [{
        challengeId: "challenge-1",
        challengeToken: "challenge-token",
        resource: "people",
        recordId: "person-1",
        field: "identityNumber",
        purpose: "support-case",
        transactionToken: "transaction-token",
        provider: "oidc",
        code: "code-exact",
        state: "state-exact",
        codeVerifier: "verifier",
      }]);
      assert.deepEqual(completed, {
        context: {
          challengeId: "challenge-1",
          challengeToken: "challenge-token",
          challengeExpiresAt: "2030-01-02T00:00:00.000Z",
          resource: "people",
          recordId: "person-1",
          field: "identityNumber",
          purpose: "support-case",
          returnPath: "/people/person-1?tab=details",
          completedFactors: ["admin-sms-otp-v1"],
        },
        completion: { policySatisfied: true, grantToken: "grant" },
      });
      assert.equal(hasPendingSensitiveRevealOIDC(success.storage), false);
      success.storage.setItem(SENSITIVE_REVEAL_OIDC_PENDING_KEY, "pending");
      assert.equal(hasPendingSensitiveRevealOIDC(success.storage), true);
      clearPendingSensitiveRevealOIDC(success.storage);
      assert.equal(hasPendingSensitiveRevealOIDC(success.storage), false);
    `);

    assert.equal(result.status, 0, result.stderr || result.stdout);
  });
});
