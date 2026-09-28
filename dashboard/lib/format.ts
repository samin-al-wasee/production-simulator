const UNITS = ["B", "KiB", "MiB", "GiB", "TiB", "PiB"];

export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes < 0) return "n/a";
  let value = bytes;
  let unit = 0;
  while (value >= 1024 && unit < UNITS.length - 1) {
    value /= 1024;
    unit++;
  }
  const digits = value >= 100 || unit === 0 ? 0 : value >= 10 ? 1 : 2;
  return `${value.toFixed(digits)} ${UNITS[unit]}`;
}

export function formatCores(cores: number): string {
  if (!Number.isFinite(cores)) return "n/a";
  return cores >= 100 ? cores.toFixed(0) : cores.toFixed(2).replace(/\.?0+$/, "");
}

export function formatCount(value: number): string {
  return Number.isFinite(value) ? Math.round(value).toLocaleString("en-US") : "n/a";
}

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
