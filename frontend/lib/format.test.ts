import { describe, expect, it } from "vitest";
import { formatDuration, formatPercent } from "./format";

describe("formatDuration", () => {
  it("renders nanoseconds compactly", () => {
    expect(formatDuration(45e9)).toBe("45s");
    expect(formatDuration(120e9)).toBe("2m");
    expect(formatDuration(125e9)).toBe("2m5s");
    expect(formatDuration(-1)).toBe("n/a");
  });
});

describe("formatPercent", () => {
  it("adds precision only for small values", () => {
    expect(formatPercent(0.052)).toBe("5.2%");
    expect(formatPercent(0.5)).toBe("50%");
  });
});
