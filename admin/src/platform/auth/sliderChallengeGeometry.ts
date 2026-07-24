export type SliderChallengeGeometry = {
  imageWidth: number;
  tileWidth: number;
};

export type SliderChallengeTrackRect = {
  left: number;
  width: number;
};

export function sliderTileXFromTrackPointer(
  clientX: number,
  track: SliderChallengeTrackRect | null,
  geometry: SliderChallengeGeometry,
  maxX: number,
) {
  if (!track || track.width <= 0) return 0;
  const imageX = (clientX - track.left) * (geometry.imageWidth / track.width);
  return clampSliderCoordinate(imageX - geometry.tileWidth / 2, 0, maxX);
}

function clampSliderCoordinate(value: number, min: number, max: number) {
  if (!Number.isFinite(value)) return min;
  return Math.min(Math.max(value, min), max);
}
