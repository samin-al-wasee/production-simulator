import { describe, expect, it } from "vitest";
import { evidenceLabel, percent } from "./progress";

describe("percent", () => {
  it("rounds and guards against empty totals", () => {
    expect(percent(1, 3)).toBe(33);
    expect(percent(21, 21)).toBe(100);
    expect(percent(0, 0)).toBe(0);
  });
});

describe("evidenceLabel", () => {
  it("describes how each evidence type completes", () => {
    expect(evidenceLabel({ type: "experiment", name: "db-outage" })).toContain("db-outage");
    expect(evidenceLabel({ type: "benchmark" })).toContain("benchmark");
    expect(evidenceLabel({ type: "manual" })).toContain("yourself");
  });
});
