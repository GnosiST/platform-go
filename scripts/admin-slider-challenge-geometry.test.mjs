import assert from "node:assert/strict";
import fs from "node:fs";
import { describe, it } from "node:test";
import { pathToFileURL } from "node:url";
import path from "node:path";

const repoRoot = path.resolve(import.meta.dirname, "..");
const geometryURL = pathToFileURL(path.join(repoRoot, "admin/src/platform/auth/sliderChallengeGeometry.ts")).href;
const { sliderHandleGrabOffsetFromTrackPointer, sliderTileXFromTrackPointer, sliderTrackContentRect } = await import(geometryURL);

function clientXForTileX(tileX, track, pointerOffset = 0) {
  return track.left + ((tileX + 30 + pointerOffset) / 300) * track.width;
}

describe("slider challenge geometry", () => {
  it("binds pointer mapping to the same track that owns the pointer events", () => {
    const loginSource = fs.readFileSync(path.join(repoRoot, "admin/src/platform/auth/AdminLoginView.tsx"), "utf8");

    assert.match(loginSource, /ref=\{trackRef\}/);
    assert.match(loginSource, /const pointerGrabOffsetRef = useRef\(0\)/);
    assert.match(loginSource, /sliderHandleGrabOffsetFromTrackPointer\(event\.clientX, track, material, boundedTileX\)/);
    assert.match(loginSource, /sliderTileXFromTrackPointer\([\s\S]*?pointerGrabOffsetRef\.current,[\s\S]*?\)/);
    assert.doesNotMatch(loginSource, /stageRef/);
  });

  it("uses the bordered track content box as the canonical coordinate system", () => {
    const contentRect = sliderTrackContentRect({
      clientLeft: 1,
      clientWidth: 318,
      getBoundingClientRect: () => ({ left: 42, width: 320 }),
    });

    assert.deepEqual(contentRect, { left: 43, width: 318 });
    assert.equal(
      Math.round(sliderTileXFromTrackPointer(clientXForTileX(173, contentRect), contentRect, { imageWidth: 300, tileWidth: 60 }, 240)),
      173,
    );
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

  it("preserves the grab point when the handle is dragged off-center", () => {
    const track = { left: 42, width: 320 };
    const targetTileX = 173;
    const pointerX = clientXForTileX(targetTileX, track, 18);
    const pointerOffset = sliderHandleGrabOffsetFromTrackPointer(
      pointerX,
      track,
      { imageWidth: 300, tileWidth: 60 },
      targetTileX,
    );

    const actual = sliderTileXFromTrackPointer(
      pointerX,
      track,
      { imageWidth: 300, tileWidth: 60 },
      240,
      pointerOffset,
    );

    assert.equal(Math.round(pointerOffset), 18);
    assert.equal(Math.round(actual), targetTileX);
  });

  it("preserves grabs beyond the narrow source tile on a production-scale track", () => {
    const track = { left: 42, width: 318 };
    const geometry = { imageWidth: 720, tileWidth: 56 };
    const targetTileX = 401;
    const pointerOffset = (24 / track.width) * geometry.imageWidth;
    const pointerX = track.left + ((targetTileX + geometry.tileWidth / 2 + pointerOffset) / geometry.imageWidth) * track.width;

    const grabOffset = sliderHandleGrabOffsetFromTrackPointer(pointerX, track, geometry, targetTileX);
    const actual = sliderTileXFromTrackPointer(pointerX, track, geometry, geometry.imageWidth - geometry.tileWidth, grabOffset);

    assert.equal(Math.round(grabOffset), Math.round(pointerOffset));
    assert.equal(Math.round(actual), targetTileX);
  });

  it("clamps pointer positions at the tile travel bounds", () => {
    const track = { left: 42, width: 320 };

    assert.equal(sliderTileXFromTrackPointer(track.left, track, { imageWidth: 300, tileWidth: 60 }, 240), 0);
    assert.equal(sliderTileXFromTrackPointer(track.left + track.width, track, { imageWidth: 300, tileWidth: 60 }, 240), 240);
  });
});
