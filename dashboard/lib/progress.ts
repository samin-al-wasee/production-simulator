export function percent(done: number, total: number): number {
  if (total <= 0) return 0;
  return Math.round((done / total) * 100);
}

export function evidenceLabel(e: { type: string; name?: string }): string {
  switch (e.type) {
    case "experiment":
      return `completes when the ${e.name} experiment passes`;
    case "benchmark":
      return "completes when a benchmark meets its SLOs";
    default:
      return "mark complete yourself";
  }
}
