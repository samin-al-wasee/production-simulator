import { describe, expect, it } from "vitest";
import { formatBytes, formatCores, formatCount, formatDuration, formatPercent } from "./format";

describe("formatBytes", () => {
  it("uses binary units", () => {
    expect(formatBytes(0)).toBe("0 B");
    expect(formatBytes(1024)).toBe("1.00 KiB");
    expect(formatBytes(16 * 1024 ** 3)).toBe("16.0 GiB");
    expect(formatBytes(14144 * 1024 ** 3)).toBe("13.8 TiB");
  });
  it("rejects invalid values", () => {
    expect(formatBytes(-1)).toBe("n/a");
    expect(formatBytes(Number.NaN)).toBe("n/a");
  });
});

describe("formatCores", () => {
  it("trims trailing zeros", () => {
    expect(formatCores(12)).toBe("12");
    expect(formatCores(1.5)).toBe("1.5");
    expect(formatCores(3536)).toBe("3536");
  });
});

describe("formatCount", () => {
  it("groups thousands", () => {
    expect(formatCount(2048000)).toBe("2,048,000");
  });
});

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
