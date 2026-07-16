export function isExternalReviewArtifactURI(value) {
  return typeof value === "string"
    && /^external-review-artifacts:\/\/platform-go\/(?:[A-Za-z0-9][A-Za-z0-9._-]*\/)+[A-Za-z0-9][A-Za-z0-9._-]*\.(?:png|jpe?g|webp)$/.test(value);
}
