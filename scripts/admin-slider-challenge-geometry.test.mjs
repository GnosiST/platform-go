import assert from "node:assert/strict";
import fs from "node:fs";
import { describe, it } from "node:test";
import { pathToFileURL } from "node:url";
import path from "node:path";

const repoRoot = path.resolve(import.meta.dirname, "..");
const geometryURL = pathToFileURL(path.join(repoRoot, "admin/src/platform/auth/sliderChallengeGeometry.ts")).href;
const { sliderTileXFromTrackPointer } = await import(geometryURL);

function clientXForTileX(tileX, track) {
  return track.left + ((tileX + 30) / 300) * track.width;
}

describe("slider challenge geometry", () => {
  it("binds pointer mapping to the same track that owns the pointer events", () => {
    const loginSource = fs.readFileSync(path.join(repoRoot, "admin/src/platform/auth/AdminLoginView.tsx"), "utf8");

    assert.match(loginSource, /ref=\{trackRef\}/);
    assert.match(loginSource, /sliderTileXFromTrackPointer\(event\.clientX, trackRef\.current\?\.getBoundingClientRect\(\) \?\? null, material, maxX\)/);
    assert.doesNotMatch(loginSource, /stageRef/);
  });

  it("uses the track as the canonical pointer coordinate system when desktop columns differ", () => {
    const track = { left: 356, width: 404 };
    const targetTileX = 173;

    const actual = sliderTileXFromTrackPointer(clientXForTileX(targetTileX, track), track, { imageWidth: 300, tileWidth: 60 }, 240);

    assert.equal(actual, targetTileX);
  });

  it("preserves the same image-space result for narrow and wide track widths", () => {
    const targetTileX = 118;
    const tracks = [
      { left: 20, width: 220 },
      { left: 284, width: 496 },
    ];

    for (const track of tracks) {
      const draftTileX = sliderTileXFromTrackPointer(clientXForTileX(targetTileX, track), track, { imageWidth: 300, tileWidth: 60 }, 240);

      assert.equal(Math.round(draftTileX), targetTileX);
    }
  });

  it("clamps pointer positions at the tile travel bounds", () => {
    const track = { left: 42, width: 320 };

    assert.equal(sliderTileXFromTrackPointer(track.left, track, { imageWidth: 300, tileWidth: 60 }, 240), 0);
    assert.equal(sliderTileXFromTrackPointer(track.left + track.width, track, { imageWidth: 300, tileWidth: 60 }, 240), 240);
  });
});
