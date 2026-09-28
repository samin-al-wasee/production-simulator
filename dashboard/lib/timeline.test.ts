import { describe, expect, it } from "vitest";
import { layoutBars } from "./timeline";

describe("layoutBars", () => {
  it("scales spans onto 0..100", () => {
    const bars = layoutBars(
      [
        { name: "build", start: 0, end: 50 },
        { name: "test", start: 50, end: 100 },
      ],
      100,
    );
    expect(bars[0]).toMatchObject({ leftPercent: 0, widthPercent: 50 });
    expect(bars[1]).toMatchObject({ leftPercent: 50, widthPercent: 50 });
  });

  it("keeps zero-length spans visible", () => {
    const [bar] = layoutBars([{ name: "skipped", start: 100, end: 100 }], 100);
    expect(bar.widthPercent).toBeGreaterThan(0);
    expect(bar.leftPercent + bar.widthPercent).toBeLessThanOrEqual(100);
  });

  it("tolerates an empty axis", () => {
    const [bar] = layoutBars([{ name: "x", start: 0, end: 0 }], 0);
    expect(Number.isFinite(bar.leftPercent)).toBe(true);
  });
});
