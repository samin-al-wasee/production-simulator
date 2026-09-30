import { describe, expect, it } from "vitest";
import {
  canConnect,
  conditionText,
  describeEvent,
  goalProgress,
  lockedBy,
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
  describePattern,
  fromDraft,
  hasBlankNumber,
  percentTotal,
  problems,
  toDraft,
  toPercent,
  type TrafficConfig,
  type GoalStatus,
  type Kind,
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

describe("goals", () => {
  const users = { label: "Users", value: 4200, min: 10_000, met: false };
  const health = { label: "Health", value: 91, min: 80, met: true };
  const goal = (over: Partial<GoalStatus>): GoalStatus => ({ id: "startup-tier", title: "Startup", description: "", conditions: [users], ...over });

  it("shows a condition's value against its bounds", () => {
    expect(conditionText(users)).toBe("Users: 4200 / 10.0k");
    expect(conditionText({ label: "App replicas", value: 1, min: 2, met: false })).toBe("App replicas: 1 / 2");
    expect(conditionText({ label: "LB ops/s", value: 0, min: 0.001, met: false })).toBe("LB ops/s: 0 (> 0)");
    expect(conditionText({ label: "App utilization", unit: "ratio", value: 0.62, min: 0.001, max: 0.99, met: true })).toBe("App utilization: 62% (> 0, ≤ 99%)");
    expect(conditionText({ label: "Cost ÷ revenue", unit: "ratio", value: 0.72, max: 0.5, met: false })).toBe("Cost ÷ revenue: 72% (≤ 50%)");
    expect(conditionText({ label: "Cost ÷ revenue", unit: "ratio", value: Number.MAX_VALUE, max: 0.5, met: false })).toBe("Cost ÷ revenue: n/a (≤ 50%)");
  });

  it("measures progress, counting met conditions as done", () => {
    expect(goalProgress(goal({}))).toBeCloseTo(0.42);
    expect(goalProgress(goal({ conditions: [users, health] }))).toBeCloseTo(0.71);
    expect(goalProgress(goal({ achievedAt: 12 }))).toBe(1);
  });

  it("finds the goal that still locks a kind", () => {
    const cache: Kind = { name: "cache", label: "Cache", capacity: 1, serviceMs: 1, costPerHour: 1, buildCost: 1, complexity: 1, connectsTo: null, unlockedBy: "startup-tier" };
    expect(lockedBy(cache, [goal({})])?.title).toBe("Startup");
    expect(lockedBy(cache, [goal({ achievedAt: 3 })])).toBeUndefined();
    expect(lockedBy({ ...cache, unlockedBy: undefined }, [goal({})])).toBeUndefined();
  });
});

describe("traffic drafts", () => {
  const tc: TrafficConfig = {
    source: "market",
    pattern: { shape: "constant", rps: 100 },
    endpoints: [
      { method: "GET", path: "/a", cacheable: true },
      { method: "POST", path: "/b" },
    ],
    groups: [
      { name: "Web", share: 0.7, endpoints: [{ name: "GET /a", share: 0.35 }, { name: "POST /b", share: 0.65 }], regions: [{ name: "asia", share: 1 }] },
      { name: "Bots", share: 0.3, retries: 2, endpoints: [{ name: "GET /a", share: 1 }], regions: [{ name: "europe", share: 1 }] },
    ],
  };

  it("edits percentages without float noise and converts back to shares", () => {
    expect(toPercent(0.07)).toBe(7);
    expect(toPercent(0.35)).toBe(35);
    const d = toDraft(tc);
    expect(d.groups[0].key).not.toBe(d.groups[1].key);
    expect(d.groups[0].endpoints).toEqual([35, 65]);
    expect(d.groups[1].endpoints).toEqual([100, 0]);
    expect(d.groups[1].retries).toBe(2);
    expect(fromDraft(d)).toEqual({
      ...tc,
      groups: [
        { ...tc.groups[0], retries: 0 },
        { ...tc.groups[1] },
      ],
    });
  });

  it("follows an endpoint renamed in the draft and never corrects a total", () => {
    const d = toDraft(tc);
    d.endpoints[0].path = "/c";
    d.groups[0].share = 60;
    const out = fromDraft(d);
    expect(out.groups[0].endpoints[0]).toEqual({ name: "GET /c", share: 0.35 });
    expect(out.groups[0].share).toBe(0.6);
    expect(percentTotal(d.groups.map((g) => g.share))).toBe(90);
    expect(percentTotal([70, 20, 8, 2])).toBe(100);
  });

  it("does not change the configuration it was made from", () => {
    const d = toDraft(tc);
    d.pattern.rps = 5;
    d.endpoints[0].method = "PUT";
    expect(tc.pattern.rps).toBe(100);
    expect(tc.endpoints[0].method).toBe("GET");
  });

  it("finds empty number fields but not the word null in a path", () => {
    expect(hasBlankNumber({ ...tc, endpoints: [{ method: "GET", path: "/nullable" }] })).toBe(false);
    expect(hasBlankNumber({ ...tc, pattern: { shape: "constant", rps: NaN } })).toBe(true);
  });

  it("splits a rejection into its problems", () => {
    expect(problems("invalid command: a must sum to 100% (now 90%); group 1: name must be set")).toEqual([
      "a must sum to 100% (now 90%)",
      "group 1: name must be set",
    ]);
  });

  it("describes patterns", () => {
    expect(describePattern({ shape: "spike", rps: 100, peakRps: 500, startMinutes: 30, minutes: 60 })).toBe(
      "100 RPS, 500 RPS for 60 min after 30 min",
    );
    expect(describePattern({ shape: "schedule", rps: 0, schedule: [{ hour: 9, rps: 1000 }, { hour: 18.5, rps: 20000 }] })).toBe(
      "09:00 1000 RPS, 18:30 20.0k RPS",
    );
  });
});
