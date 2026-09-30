import { describe, expect, it } from "vitest";
import {
  canConnect,
  describeEvent,
  eventTiming,
  formatClock,
  formatDuration,
  freeSpot,
  formatCompact,
  formatMoney,
  level,
  newest,
  scoreLevel,
  sparkline,
  type SandboxEvent,
} from "./sandbox";

describe("formatMoney", () => {
  it("abbreviates large amounts and keeps the sign", () => {
    expect(formatMoney(12.5)).toBe("$12.50");
    expect(formatMoney(710)).toBe("$710");
    expect(formatMoney(25_300)).toBe("$25.3k");
    expect(formatMoney(-2_500_000)).toBe("-$2.50M");
  });
});

describe("formatCompact", () => {
  it("shortens counts", () => {
    expect(formatCompact(18.83)).toBe("18.8");
    expect(formatCompact(15_611)).toBe("15.6k");
    expect(formatCompact(3_200_000)).toBe("3.20M");
  });
});

describe("formatClock", () => {
  it("renders day and wall time", () => {
    expect(formatClock(1, 8)).toBe("Day 1 · 08:00");
    expect(formatClock(3, 13.9999)).toBe("Day 3 · 14:00");
  });
});

describe("levels", () => {
  it("classifies utilization and scores", () => {
    expect([level(0.5), level(0.85), level(1.4)]).toEqual(["ok", "warn", "bad"]);
    expect([scoreLevel(95), scoreLevel(60), scoreLevel(10)]).toEqual(["ok", "warn", "bad"]);
  });
});

describe("sparkline", () => {
  it("scales values into the box, top is the maximum", () => {
    expect(sparkline([0, 10], 100, 20)).toBe("0.0,20.0 100.0,0.0");
  });
  it("draws a flat series in the middle", () => {
    expect(sparkline([5, 5, 5], 10, 20)).toBe("0.0,10.0 5.0,10.0 10.0,10.0");
  });
  it("handles empty input", () => {
    expect(sparkline([], 10, 10)).toBe("");
  });
});

describe("canConnect", () => {
  const kinds = [
    { name: "app-instance", connectsTo: ["db-primary"] },
    { name: "db-primary", connectsTo: null },
  ] as never;
  it("follows the ruleset", () => {
    expect(canConnect(kinds, "app-instance", "db-primary")).toBe(true);
    expect(canConnect(kinds, "db-primary", "app-instance")).toBe(false);
  });
});

describe("freeSpot", () => {
  it("uses the point itself when it is free", () => {
    expect(freeSpot({ x: 100, y: 55 }, [])).toEqual({ x: 0, y: 0 });
  });
  it("steps away from occupied spots without overlapping", () => {
    const nodes = [{ x: 0, y: 0 }, { x: 200, y: 0 }];
    const spot = freeSpot({ x: 100, y: 55 }, nodes);
    for (const n of nodes) {
      expect(Math.abs(n.x - spot.x) >= 200 || Math.abs(n.y - spot.y) >= 110).toBe(true);
    }
  });
});

describe("newest", () => {
  const snap = (id: string, revision: number) => ({ id, revision }) as never;
  it("ignores a snapshot older than the one shown", () => {
    expect(newest(snap("g", 5), snap("g", 4))).toEqual(snap("g", 5));
    expect(newest(snap("g", 5), snap("g", 6))).toEqual(snap("g", 6));
  });
  it("switches games regardless of revision", () => {
    expect(newest(snap("g", 5), snap("h", 0))).toEqual(snap("h", 0));
  });
});

describe("formatDuration", () => {
  it("renders simulated minutes, hours, and days", () => {
    expect(formatDuration(300)).toBe("5m");
    expect(formatDuration(7500)).toBe("2h 05m");
    expect(formatDuration(100_800)).toBe("1d 4h");
    expect(formatDuration(-60)).toBe("0m");
  });
});

describe("events", () => {
  const event = (over: Partial<SandboxEvent>): SandboxEvent => ({
    id: 1, card: "c", label: "L", effect: "traffic", start: 300, end: 324, magnitude: 4.26, phase: "active", lowestHealth: 42, ...over,
  });

  it("describes the model input an event changes", () => {
    expect(describeEvent(event({}))).toBe("real traffic ×4.3");
    expect(describeEvent(event({ effect: "crash", target: "app-instance-1" }))).toBe("app-instance-1: 1 replica down");
    expect(describeEvent(event({ effect: "third-party", magnitude: 0.25 }))).toBe("25% of requests fail whatever the design");
    expect(describeEvent(event({ effect: "cost", target: "cache", magnitude: 2 }), [
      { name: "cache", label: "Cache", capacity: 1, serviceMs: 1, costPerHour: 1, buildCost: 1, complexity: 1, connectsTo: null },
    ])).toBe("Cache running cost ×2.0");
  });

  it("times each phase in simulated time", () => {
    expect(eventTiming(event({ phase: "upcoming" }), 276, 300, 12)).toBe("starts in 2h 00m");
    expect(eventTiming(event({}), 312, 300, 12)).toBe("ends in 1h 00m");
    expect(eventTiming(event({ phase: "recovering" }), 330, 300, 12)).toBe("judged in 30m");
    expect(eventTiming(event({ phase: "over", outcome: "recovered" }), 400, 300, 12)).toBe("recovered");
    expect(eventTiming(event({ phase: "over", outcome: "unrecovered" }), 400, 300, 12)).toBe("not recovered (health fell to 42)");
  });
});
