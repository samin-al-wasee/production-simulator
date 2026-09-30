import { describe, expect, it } from "vitest";
import { percent } from "./progress";

describe("percent", () => {
  it("rounds and guards against empty totals", () => {
    expect(percent(1, 3)).toBe(33);
    expect(percent(21, 21)).toBe(100);
    expect(percent(0, 0)).toBe(0);
  });
});
