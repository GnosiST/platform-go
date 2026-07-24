export function credentialChallengeSecondsRemaining(expiresAt: string | undefined, now = Date.now()) {
  const deadline = Date.parse(expiresAt ?? "");
  if (!Number.isFinite(deadline)) return null;
  return Math.max(0, Math.ceil((deadline - now) / 1000));
}

export function formatCredentialChallengeCountdown(seconds: number) {
  const boundedSeconds = Math.max(0, Math.floor(Number.isFinite(seconds) ? seconds : 0));
  const minutes = Math.floor(boundedSeconds / 60);
  return `${minutes}:${String(boundedSeconds % 60).padStart(2, "0")}`;
}
