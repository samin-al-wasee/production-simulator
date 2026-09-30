export interface Span {
  name: string;
  start: number;
  end: number;
}

export interface Bar extends Span {
  leftPercent: number;
  widthPercent: number;
}

// Positions spans on a shared 0..total axis. Zero-length spans keep a minimal
// visible width so they are not lost.
export function layoutBars(spans: Span[], total: number): Bar[] {
  const scale = total > 0 ? total : 1;
  return spans.map((s) => {
    const minWidth = 0.5;
    const left = Math.min((s.start / scale) * 100, 100 - minWidth);
    const width = Math.min(Math.max(((s.end - s.start) / scale) * 100, minWidth), 100 - left);
    return { ...s, leftPercent: left, widthPercent: width };
  });
}
