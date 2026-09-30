// Durations from the API are integer nanoseconds (Go time.Duration).
export function formatDuration(ns: number): string {
  if (!Number.isFinite(ns) || ns < 0) return "n/a";
  const totalSeconds = Math.round(ns / 1e9);
  if (totalSeconds < 60) return `${totalSeconds}s`;
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  return seconds === 0 ? `${minutes}m` : `${minutes}m${seconds}s`;
}

export function formatPercent(fraction: number): string {
  return Number.isFinite(fraction) ? `${(fraction * 100).toFixed(fraction < 0.1 ? 1 : 0)}%` : "n/a";
}
