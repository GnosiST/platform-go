export type SliderChallengeGeometry = {
  imageWidth: number;
  tileWidth: number;
};

export type SliderChallengeTrackRect = {
  left: number;
  width: number;
};

export type SliderChallengeTrackElement = {
  clientLeft: number;
  clientWidth: number;
  getBoundingClientRect: () => SliderChallengeTrackRect;
};

export function sliderTrackContentRect(track: SliderChallengeTrackElement | null): SliderChallengeTrackRect | null {
  if (!track) return null;
  const rect = track.getBoundingClientRect();
  return {
    left: rect.left + Math.max(0, track.clientLeft),
    width: track.clientWidth > 0 ? track.clientWidth : rect.width,
  };
}

export function sliderTileXFromTrackPointer(
  clientX: number,
  track: SliderChallengeTrackRect | null,
  geometry: SliderChallengeGeometry,
  maxX: number,
  pointerOffset = 0,
) {
  if (!track || track.width <= 0) return 0;
  const imageX = (clientX - track.left) * (geometry.imageWidth / track.width);
  const grabOffset = Number.isFinite(pointerOffset) ? pointerOffset : 0;
  return clampSliderCoordinate(imageX - geometry.tileWidth / 2 - grabOffset, 0, maxX);
}

export function sliderHandleGrabOffsetFromTrackPointer(
  clientX: number,
  track: SliderChallengeTrackRect | null,
  geometry: SliderChallengeGeometry,
  tileX: number,
) {
  if (!track || track.width <= 0) return 0;
  const imageX = (clientX - track.left) * (geometry.imageWidth / track.width);
  const handleCenterX = tileX + geometry.tileWidth / 2;
  const grabOffset = imageX - handleCenterX;
  return Number.isFinite(grabOffset) ? grabOffset : 0;
}

function clampSliderCoordinate(value: number, min: number, max: number) {
  if (!Number.isFinite(value)) return min;
  return Math.min(Math.max(value, min), max);
}
