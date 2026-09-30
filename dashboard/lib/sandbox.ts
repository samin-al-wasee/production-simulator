// Sandbox game client and display helpers. Every number comes from the core
// engine; this file only formats it and sends commands.

import { request } from "./client";

export interface Kind {
  name: string;
  label: string;
  capacity: number;
  serviceMs: number;
  costPerHour: number;
  buildCost: number;
  complexity: number;
  connectsTo: string[] | null;
  hitRatio?: number;
  maxBacklog?: number;
}

export interface Size {
  name: string;
  capacityFactor: number;
  costFactor: number;
}

export interface Ruleset {
  version: string;
  kinds: Kind[];
  sizes: Size[];
  maxReplicas: number;
  sloP95Ms: number;
  startingCash: number;
  tickSeconds: number;
  eventGraceTicks?: number;
  recoveryTicks?: number;
}

export interface SandboxNode {
  id: string;
  kind: string;
  size: string;
  replicas: number;
  x: number;
  y: number;
  downReplicas?: number;
  down?: boolean;
  rateLimited?: boolean;
  backlog?: number;
}

export interface Edge {
  from: string;
  to: string;
}

export interface NodeStats {
  id: string;
  offered: number;
  served: number;
  dropped: number;
  attack?: number;
  blocked?: number;
  capacity: number;
  utilization: number;
  latencyMs: number;
  backlog?: number;
  costPerHour: number;
}

export interface Meters {
  tick: number;
  day: number;
  hour: number;
  rps: number;
  attackRps?: number;
  successRps: number;
  users: number;
  activeUsers: number;
  engagement: number;
  p95LatencyMs: number;
  errorRate: number;
  health: number;
  satisfaction: number;
  popularity: number;
  complexity: number;
  tier: string;
  revenuePerHour: number;
  costPerHour: number;
  cash: number;
}

export type EventPhase = "upcoming" | "active" | "recovering" | "over";

export interface SandboxEvent {
  id: number;
  card: string;
  label: string;
  effect: string;
  start: number;
  end: number;
  magnitude: number;
  target?: string;
  hits?: { node: string; replicas: number }[];
  phase: EventPhase;
  lowestHealth: number;
  outcome?: "recovered" | "unrecovered";
}

export interface GameState {
  id: string;
  simulated: true;
  ruleset: string;
  seed: number;
  status: "running" | "bankrupt";
  speed: number;
  revision: number;
  tick: number;
  meters: Meters;
  nodes: SandboxNode[];
  edges: Edge[];
  flow: { rps: number; attackRps?: number; successRps: number; errorRate: number; p95LatencyMs: number; nodes: NodeStats[] };
  history: Meters[] | null;
  events: SandboxEvent[] | null;
}

export type Command =
  | { type: "place"; kind: string; size?: string; x: number; y: number }
  | { type: "remove"; node: string }
  | { type: "connect" | "disconnect"; from: string; to: string }
  | { type: "resize"; node: string; size: string }
  | { type: "scale"; node: string; replicas: number }
  | { type: "move"; node: string; x: number; y: number }
  | { type: "respond"; action: "restart" | "failover" | "rate-limit" | "lift-rate-limit"; node: string };

export const SPEEDS = [0, 1, 2, 4, 8] as const;

const json = (body: unknown): RequestInit => ({
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify(body),
});

const game = (id: string) => `/sandbox/games/${encodeURIComponent(id)}`;

export const sandboxApi = {
  ruleset: () => request<Ruleset>("/sandbox/ruleset"),
  list: () => request<{ id: string; status: string; tick: number }[]>("/sandbox/games"),
  create: () => request<GameState>("/sandbox/games", json({})),
  get: (id: string) => request<GameState>(game(id)),
  remove: (id: string) => fetch(`/api/forgelab${game(id)}`, { method: "DELETE" }),
  command: (id: string, c: Command) => request<{ node?: string; state: GameState }>(`${game(id)}/commands`, json(c)),
  speed: (id: string, speed: number) => request<GameState>(`${game(id)}/speed`, json({ speed })),
  step: (id: string, ticks: number) => request<GameState>(`${game(id)}/step`, json({ ticks })),
  save: (id: string) => request<{ path: string }>(`${game(id)}/save`, { method: "POST" }),
  streamUrl: (id: string) => `/api/forgelab${game(id)}/stream`,
};

export function formatMoney(value: number): string {
  if (!Number.isFinite(value)) return "n/a";
  const sign = value < 0 ? "-" : "";
  const abs = Math.abs(value);
  if (abs >= 1e6) return `${sign}$${(abs / 1e6).toFixed(2)}M`;
  if (abs >= 1e4) return `${sign}$${(abs / 1e3).toFixed(1)}k`;
  return `${sign}$${abs.toFixed(abs >= 100 ? 0 : 2)}`;
}

export function formatCompact(value: number): string {
  if (!Number.isFinite(value)) return "n/a";
  const abs = Math.abs(value);
  if (abs >= 1e9) return `${(value / 1e9).toFixed(2)}B`;
  if (abs >= 1e6) return `${(value / 1e6).toFixed(2)}M`;
  if (abs >= 1e4) return `${(value / 1e3).toFixed(1)}k`;
  return abs >= 100 ? value.toFixed(0) : value.toFixed(1);
}

export function formatClock(day: number, hour: number): string {
  const h = Math.floor(hour);
  const m = Math.round((hour - h) * 60);
  const [hh, mm] = m === 60 ? [h + 1, 0] : [h, m];
  return `Day ${day} · ${String(hh % 24).padStart(2, "0")}:${String(mm).padStart(2, "0")}`;
}

// level maps a utilization (0 = idle, 1 = saturated) to a status class.
export function level(utilization: number): "ok" | "warn" | "bad" {
  if (utilization >= 1) return "bad";
  if (utilization >= 0.8) return "warn";
  return "ok";
}

// scoreLevel maps a 0-100 score (health, satisfaction) to a status class.
export function scoreLevel(score: number): "ok" | "warn" | "bad" {
  if (score >= 80) return "ok";
  if (score >= 50) return "warn";
  return "bad";
}

// sparkline returns an SVG polyline "points" string for values scaled into a
// width × height box; a flat series sits in the middle.
export function sparkline(values: number[], width: number, height: number): string {
  if (values.length === 0) return "";
  const min = Math.min(...values);
  const max = Math.max(...values);
  const span = max - min;
  const step = values.length > 1 ? width / (values.length - 1) : 0;
  return values
    .map((v, i) => {
      const y = span === 0 ? height / 2 : height - ((v - min) / span) * height;
      return `${(i * step).toFixed(1)},${y.toFixed(1)}`;
    })
    .join(" ");
}

// canConnect reports whether the ruleset allows traffic from one kind to another.
export function canConnect(kinds: Kind[], from: string, to: string): boolean {
  return kinds.find((k) => k.name === from)?.connectsTo?.includes(to) ?? false;
}

// Node footprint on the canvas, used to keep click-placed nodes apart.
export const NODE_W = 200;
export const NODE_H = 110;

// freeSpot returns the first position near a point, scanning right then down
// in node-sized steps, that does not overlap an existing node.
export function freeSpot(near: { x: number; y: number }, nodes: { x: number; y: number }[]): { x: number; y: number } {
  const start = { x: Math.round(near.x - NODE_W / 2), y: Math.round(near.y - NODE_H / 2) };
  for (let ring = 0; ring < 20; ring++) {
    for (let row = 0; row <= ring; row++) {
      const c = { x: start.x + ring * NODE_W, y: start.y + row * NODE_H };
      if (!nodes.some((n) => Math.abs(n.x - c.x) < NODE_W && Math.abs(n.y - c.y) < NODE_H)) return c;
    }
  }
  return start;
}

// newest keeps the later of two snapshots of the same game. Stream events and
// command responses travel separately and can arrive out of order.
export function newest(current: GameState, next: GameState): GameState {
  if (next.id !== current.id) return next;
  return next.revision >= current.revision ? next : current;
}

// formatDuration renders simulated seconds as "45m", "2h 05m", or "1d 4h".
export function formatDuration(seconds: number): string {
  const m = Math.max(0, Math.round(seconds / 60));
  if (m < 60) return `${m}m`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h}h ${String(m % 60).padStart(2, "0")}m`;
  return `${Math.floor(h / 24)}d ${h % 24}h`;
}

const pct = (v: number) => `${Math.round(v * 100)}%`;

// describeEvent states in words which model input an event changed.
export function describeEvent(e: SandboxEvent, kinds: Kind[] = []): string {
  const m = e.magnitude;
  switch (e.effect) {
    case "traffic":
      return `real traffic ×${m.toFixed(1)}`;
    case "attack":
      return `attack traffic at ${m.toFixed(1)}× real traffic, earning nothing`;
    case "crash":
      return `${e.target}: 1 replica down`;
    case "zone":
      return `${e.hits?.length ?? 0} components lose ${pct(m)} of their replicas (rounded up)`;
    case "slowdown":
      return `${e.target}: ${m.toFixed(1)}× slower, capacity ÷${m.toFixed(1)}`;
    case "hit-ratio":
      return `${e.target}: hit ratio falls to ${pct(m)}`;
    case "capacity":
      return `${e.target}: capacity ×${m.toFixed(2)}`;
    case "cost":
      return `${kinds.find((k) => k.name === e.target)?.label ?? e.target} running cost ×${m.toFixed(1)}`;
    case "third-party":
      return `${pct(m)} of requests fail whatever the design`;
  }
  return e.effect;
}

// eventTiming says when an event starts, ends, or is judged, in simulated time.
export function eventTiming(e: SandboxEvent, tick: number, tickSeconds: number, recoveryTicks: number): string {
  const until = (t: number) => formatDuration((t - tick) * tickSeconds);
  switch (e.phase) {
    case "upcoming":
      return `starts in ${until(e.start)}`;
    case "active":
      return `ends in ${until(e.end)}`;
    case "recovering":
      return `judged in ${until(e.end + recoveryTicks)}`;
  }
  return e.outcome === "recovered" ? "recovered" : `not recovered (health fell to ${e.lowestHealth.toFixed(0)})`;
}
