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
  unlockedBy?: string;
}

export interface Size {
  name: string;
  capacityFactor: number;
  costFactor: number;
  vcpu?: number;
  memoryGb?: number;
  networkMbps?: number;
  iops?: number;
}

export type Processing = "sync" | "async";
export const DEPS = ["cache", "db-read", "db-write", "queue", "storage"] as const;

export interface AppRoute {
  endpoint: string;
  baseMs: number;
  cpuMs: number;
  memoryMb: number;
  requestKb: number;
  responseKb: number;
  errorRate?: number;
  deps?: string[];
  share?: number;
  calls?: Call[];
}

// Call is a route's request to another service's endpoint (v7).
export interface Call {
  service: string;
  endpoint: string;
  async?: boolean;
}

// Listener is what a component accepts connections on; Connection is the
// client side of an edge (v7).
export interface Listener {
  protocol: string;
  port: number;
  tls: boolean;
}

export interface Connection extends Listener {
  pool: number;
  timeoutMs: number;
  retries?: number;
}

// QueryProfile, DBConfig, and DBStats are a database's model (v8).
export interface QueryProfile {
  cpuMs: number;
  pages: number;
  indexed: boolean;
}

export interface DBConfig {
  engine: string;
  maxConnections: number;
  read: QueryProfile;
  write: QueryProfile;
  hotRows: number;
  lockMs: number;
}

export interface DBStats {
  health: Health;
  bottleneck: string;
  capacity: number;
  cpuUsed: number;
  cpuTotal: number;
  iopsUsed: number;
  iopsTotal: number;
  hitRatio: number;
  dataMb: number;
  workingSetMb: number;
  bufferPoolMb: number;
  connections: number;
  maxConnections: number;
  refused: number;
  reads: number;
  writes: number;
  readMs: number;
  writeMs: number;
  waitMs: number;
  lockWaitMs: number;
  applying?: number;
  lagSeconds?: number;
}

// CacheConfig and CacheStats are a cache's model (v9).
export interface CacheConfig {
  engine: string;
  eviction: "lru" | "lfu" | "none";
  ttlSeconds: number;
  valueKb: number;
  maxConnections: number;
}

export interface CacheStats {
  health: Health;
  bottleneck: string;
  capacity: number;
  hitRatio: number;
  fits: number;
  fresh: number;
  warmth: number;
  memoryUsedMb: number;
  memoryTotalMb: number;
  keys: number;
  hits: number;
  misses: number;
  evictions: number;
  cpuUsed: number;
  cpuTotal: number;
  networkMbps: number;
  networkTotalMbps: number;
  connections: number;
  maxConnections: number;
  refused: number;
  latencyMs: number;
}

// StorageConfig and StorageStats are object storage's model (v10).
export interface StorageConfig {
  class: string;
  prefixes: number;
  objectKb: number;
}

export interface StorageClass {
  name: string;
  label: string;
  firstByteMs: number;
  gbMonth: number;
  perThousand: number;
  retrievalGb: number;
}

export interface StorageStats {
  health: Health;
  class: string;
  capacity: number;
  gets: number;
  throttled: number;
  firstByteMs: number;
  transferMs: number;
  latencyMs: number;
  storedGb: number;
  egressMbps: number;
  storageCost: number;
  requestCost: number;
  retrievalCost: number;
  egressCost: number;
}

// QueueConfig, WorkerConfig, and QueueStats are a queue's and a worker's
// model (v11); a worker reports its run as AppStats.
export interface QueueConfig {
  engine: string;
  maxBacklog: number;
  visibilitySeconds: number;
  maxDeliveries: number;
}

export interface WorkerConfig {
  concurrency: number;
  handler: AppRoute;
}

export interface QueueStats {
  health: Health;
  published: number;
  rejected: number;
  delivered: number;
  redelivered: number;
  deadLettered: number;
  deadLetters: number;
  backlog: number;
  maxBacklog: number;
  delaySeconds: number;
  workerFailure: number;
}

export interface EdgeStats {
  from: string;
  to: string;
  rps: number;
  errors: number;
  retryRps?: number;
  latencyMs: number;
  problem?: string;
}

export interface AppConfig {
  name: string;
  framework: string;
  version: string;
  environment: string;
  protocol: string;
  port: number;
  interface: string;
  server: string;
  processing: Processing;
  workers: number;
  maxConcurrency: number;
  backlog: number;
  maxConnections: number;
  timeoutMs: number;
  tls: boolean;
  keepAlive: boolean;
  middleware: string[] | null;
  rateLimitRps?: number;
  routes: AppRoute[];
}

export interface Middleware {
  name: string;
  label: string;
  ms: number;
  cpuMs: number;
}

export interface Framework {
  name: string;
  interface: string;
  server: string;
  processing: Processing;
  workers: number;
  maxConcurrency: number;
}

// AppStack is how a new application serves (a complete configuration whose
// routes the chosen AppType replaces); AppType is what it serves.
export interface AppStack {
  name: string;
  description: string;
  app: AppConfig;
}

export interface AppType {
  name: string;
  description: string;
  routes: AppRoute[];
}

export type Health = "starting" | "healthy" | "degraded" | "unhealthy" | "stopped";

export interface RouteStats {
  endpoint: string;
  rps: number;
  success: number;
  errors: number;
  timeouts: number;
  rejected: number;
  latencyMs: number;
  notFound?: boolean;
}

export interface AppStats {
  health: Health;
  bottleneck: string;
  capacity: number;
  cpuUsed: number;
  cpuTotal: number;
  memoryMb: number;
  memoryTotalMb: number;
  active: number;
  queued: number;
  connections: number;
  waitMs: number;
  success: number;
  errors: number;
  timeouts: number;
  rejected: number;
  outOfMemory?: boolean;
  routes: RouteStats[] | null;
}

export type TrafficSource = "market" | "configured";
export type PatternShape = "constant" | "ramp" | "spike" | "burst" | "periodic" | "schedule";

export interface Weight {
  name: string;
  share: number;
}

export interface Endpoint {
  method: string;
  path: string;
  cacheable?: boolean;
  storage?: boolean;
}

export interface TrafficGroup {
  name: string;
  share: number;
  retries?: number;
  endpoints: Weight[];
  regions: Weight[];
}

export interface Pattern {
  shape: PatternShape;
  rps: number;
  peakRps?: number;
  startMinutes?: number;
  minutes?: number;
  periodMinutes?: number;
  schedule?: { hour: number; rps: number }[];
}

export interface TrafficConfig {
  source: TrafficSource;
  pattern: Pattern;
  endpoints: Endpoint[];
  groups: TrafficGroup[];
}

export interface Rate {
  name: string;
  rps: number;
}

// Traffic is what the Internet, or every traffic component together, sent
// in a tick.
export interface Traffic {
  source: TrafficSource;
  rps: number;
  retryRps?: number;
  concurrency: number;
  groups?: Rate[];
  regions?: Rate[];
  endpoints?: Rate[];
  clientTypes?: Rate[];
  components?: Rate[];
}

export type Scheme = "http" | "https";

// ClientConfig is one traffic component: a single population of clients
// (v6 and later).
export interface ClientConfig {
  name: string;
  clientType: string;
  region: string;
  protocol: string;
  scheme: Scheme;
  port: number;
  keepAlive: boolean;
  timeoutMs: number;
  retries?: number;
  source: TrafficSource;
  pattern: Pattern;
  endpoints: Weight[];
}

// ClientStats is how a traffic component fared: successes are requests
// after retries, failures are attempts by reason.
export interface ClientStats {
  rps: number;
  retryRps?: number;
  attackRps?: number;
  success: number;
  refused?: number;
  notFound?: number;
  rejected?: number;
  timeouts?: number;
  errors?: number;
  latencyMs: number;
  concurrency: number;
  problem?: string;
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
  traffic?: TrafficConfig;
  regions?: string[];
  maxRetries?: number;
  maxTrafficRps?: number;
  app?: AppConfig;
  middleware?: Middleware[];
  frameworks?: Framework[];
  appStacks?: AppStack[];
  appTypes?: AppType[];
  appRuntime?: { workerMemoryMb: number };
  client?: ClientConfig;
  clientTypes?: Weight[];
  regionShares?: Weight[];
  protocols?: string[];
  wireProtocols?: string[];
  listeners?: Record<string, Listener>;
  connDefaults?: Connection;
  networkHopMs?: number;
  db?: DBConfig;
  cache?: CacheConfig;
  storage?: StorageConfig;
  storageRuntime?: { classes: StorageClass[]; getsPerPrefix: number };
  queue?: QueueConfig;
  worker?: WorkerConfig;
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
  traffic?: TrafficConfig;
  trafficSince?: number;
  app?: AppConfig;
  client?: ClientConfig;
  listener?: Listener;
  db?: DBConfig;
  cache?: CacheConfig;
  warmth?: number;
  storage?: StorageConfig;
  queue?: QueueConfig;
  worker?: WorkerConfig;
  deadLetters?: number;
}

export interface Edge {
  from: string;
  to: string;
  conn?: Connection;
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
  app?: AppStats;
  traffic?: ClientStats;
  db?: DBStats;
  cache?: CacheStats;
  storage?: StorageStats;
  queue?: QueueStats;
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
  loadTest?: boolean;
  revenuePerHour: number;
  costPerHour: number;
  cash: number;
}

export interface ConditionStatus {
  label: string;
  unit?: "ratio";
  value: number;
  min?: number;
  max?: number;
  met: boolean;
}

export interface GoalStatus {
  id: string;
  title: string;
  description: string;
  requires?: string;
  unlocks?: string[];
  achievedAt?: number;
  conditions: ConditionStatus[];
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
  targets?: string[];
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
  freeBuild?: boolean;
  status: "running" | "bankrupt";
  speed: number;
  revision: number;
  tick: number;
  meters: Meters;
  nodes: SandboxNode[];
  edges: Edge[];
  flow: {
    rps: number;
    attackRps?: number;
    successRps: number;
    errorRate: number;
    p95LatencyMs: number;
    nodes: NodeStats[];
    traffic?: Traffic;
    edges?: EdgeStats[];
  };
  history: Meters[] | null;
  events: SandboxEvent[] | null;
  goals: GoalStatus[] | null;
}

export type Command =
  | { type: "place"; kind: string; size?: string; x: number; y: number; app?: AppConfig }
  | { type: "remove"; node: string }
  | { type: "connect" | "disconnect"; from: string; to: string }
  | { type: "resize"; node: string; size: string }
  | { type: "scale"; node: string; replicas: number }
  | { type: "move"; node: string; x: number; y: number }
  | { type: "respond"; action: "restart" | "failover" | "rate-limit" | "lift-rate-limit"; node: string }
  | { type: "configure"; node: string; traffic?: TrafficConfig; app?: AppConfig; client?: ClientConfig; listener?: Listener; db?: DBConfig; cache?: CacheConfig; storage?: StorageConfig; queue?: QueueConfig; worker?: WorkerConfig }
  | { type: "configure"; from: string; to: string; connection: Connection };

export const SPEEDS = [0, 1, 2, 4, 8] as const;

const json = (body: unknown): RequestInit => ({
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify(body),
});

const game = (id: string) => `/sandbox/games/${encodeURIComponent(id)}`;

export const sandboxApi = {
  ruleset: (version?: string) => request<Ruleset>(`/sandbox/ruleset${version ? `?version=${encodeURIComponent(version)}` : ""}`),
  list: () => request<{ id: string; status: string; tick: number }[]>("/sandbox/games"),
  create: (freeBuild = false) => request<GameState>("/sandbox/games", json(freeBuild ? { freeBuild } : {})),
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
  const on = e.targets?.length ? `${e.targets.join(", ")}: ` : "";
  switch (e.effect) {
    case "traffic":
      return `${on}real traffic ×${m.toFixed(1)}`;
    case "attack":
      return `${on}attack traffic at ${m.toFixed(1)}× real traffic, earning nothing`;
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

// conditionText shows a goal condition's current value against its bounds.
// A tiny positive minimum means "more than zero".
export function conditionText(c: ConditionStatus): string {
  const show = (v: number) => {
    if (v >= 1e12) return "n/a";
    if (c.unit === "ratio") return `${Math.round(v * 100)}%`;
    return Number.isInteger(v) && Math.abs(v) < 1e4 ? String(v) : formatCompact(v);
  };
  const aboveZero = c.min !== undefined && c.min > 0 && c.min <= 0.001;
  if (c.min !== undefined && c.max === undefined && !aboveZero) return `${c.label}: ${show(c.value)} / ${show(c.min)}`;
  const bounds = [
    ...(c.min !== undefined ? [aboveZero ? "> 0" : `≥ ${show(c.min)}`] : []),
    ...(c.max !== undefined ? [`≤ ${show(c.max)}`] : []),
  ];
  return `${c.label}: ${show(c.value)} (${bounds.join(", ")})`;
}

// goalProgress is how far a goal's conditions are along, from 0 to 1: the
// mean of each condition's share of its minimum, counting met ones as done.
export function goalProgress(g: GoalStatus): number {
  if (g.achievedAt !== undefined) return 1;
  if (g.conditions.length === 0) return 0;
  const share = (c: ConditionStatus) => {
    if (c.met) return 1;
    if (c.min !== undefined && c.min > 0) return Math.min(Math.max(c.value / c.min, 0), 0.99);
    return 0;
  };
  return g.conditions.reduce((sum, c) => sum + share(c), 0) / g.conditions.length;
}

// lockedBy returns the unreached goal that still locks a kind, if any.
export function lockedBy(kind: Kind, goals: GoalStatus[]): GoalStatus | undefined {
  if (!kind.unlockedBy) return undefined;
  const goal = goals.find((g) => g.id === kind.unlockedBy);
  return goal && goal.achievedAt === undefined ? goal : undefined;
}

// internetConfig is the Internet's traffic configuration: its own once set,
// else the ruleset's. The palette's ruleset is the latest, so an older game
// has none.
export function internetConfig(game: GameState, rules: Ruleset): TrafficConfig | undefined {
  return game.nodes.find((n) => n.id === "internet")?.traffic ?? (game.ruleset === rules.version ? rules.traffic : undefined);
}

// appConfig is an application instance's configuration: its own once set,
// else the ruleset's, under the same version rule as internetConfig.
export function appConfig(game: GameState, rules: Ruleset, node: SandboxNode): AppConfig | undefined {
  return node.app ?? (game.ruleset === rules.version ? rules.app : undefined);
}

// clientConfig is a traffic component's configuration: its own once set,
// else the ruleset's, under the same version rule as internetConfig.
export function clientConfig(game: GameState, rules: Ruleset, node: SandboxNode): ClientConfig | undefined {
  return node.client ?? (game.ruleset === rules.version ? rules.client : undefined);
}

// routedStorage reports whether a game's applications, not its endpoints,
// decide which requests fetch from object storage (v5 and later).
export function routedStorage(game: GameState, rules: Ruleset): boolean {
  return game.ruleset === rules.version && !!rules.app;
}

// healthLevel maps an application's health to a status class.
export function healthLevel(h: Health): "ok" | "warn" | "bad" {
  if (h === "healthy") return "ok";
  if (h === "starting" || h === "degraded") return "warn";
  return "bad";
}

// The traffic form edits percentages; the engine takes shares from 0 to 1.
// These helpers only convert between the two and never correct a value: the
// engine validates the configuration and reports what is wrong.

export interface GroupDraft {
  // key tells rows apart while groups are added and removed.
  key: number;
  name: string;
  share: number;
  retries: number;
  // endpoints[i] is the percentage of the group's requests to endpoint i.
  endpoints: number[];
  regions: Record<string, number>;
}

export interface TrafficDraft {
  source: TrafficSource;
  pattern: Pattern;
  endpoints: Endpoint[];
  groups: GroupDraft[];
}

// toPercent turns a share into a percentage without float noise (0.07 → 7).
export function toPercent(share: number): number {
  return Math.round(share * 1e6) / 1e4;
}

export function endpointName(e: Endpoint): string {
  return `${e.method} ${e.path}`;
}

let draftKeys = 0;

// newGroupKey returns a key no other draft group has.
export function newGroupKey(): number {
  return ++draftKeys;
}

export function toDraft(tc: TrafficConfig): TrafficDraft {
  return {
    source: tc.source,
    pattern: { ...tc.pattern, schedule: tc.pattern.schedule?.map((s) => ({ ...s })) },
    endpoints: tc.endpoints.map((e) => ({ ...e })),
    groups: tc.groups.map((g) => ({
      key: newGroupKey(),
      name: g.name,
      share: toPercent(g.share),
      retries: g.retries ?? 0,
      endpoints: tc.endpoints.map((e) => toPercent(g.endpoints.find((w) => w.name === endpointName(e))?.share ?? 0)),
      regions: Object.fromEntries(g.regions.map((w) => [w.name, toPercent(w.share)])),
    })),
  };
}

// fromDraft lists every non-zero percentage; a group with none is sent as is
// so the engine can say what is missing.
export function fromDraft(d: TrafficDraft): TrafficConfig {
  const pick = (entries: [string, number][]): Weight[] =>
    entries.filter(([, v]) => v !== 0).map(([name, v]) => ({ name, share: v / 100 }));
  return {
    source: d.source,
    pattern: d.pattern,
    endpoints: d.endpoints,
    groups: d.groups.map((g) => ({
      name: g.name,
      share: g.share / 100,
      retries: g.retries,
      endpoints: pick(d.endpoints.map((e, i) => [endpointName(e), g.endpoints[i] ?? 0])),
      regions: pick(Object.entries(g.regions)),
    })),
  };
}

// percentTotal sums percentages for display, rounded like toPercent.
export function percentTotal(values: number[]): number {
  return Math.round(values.reduce((a, b) => a + (Number.isFinite(b) ? b : 0), 0) * 1e4) / 1e4;
}

// hasBlankNumber reports an empty or non-numeric field, which JSON would
// send as null and the engine would read as 0.
export function hasBlankNumber(tc: unknown): boolean {
  let blank = false;
  JSON.stringify(tc, (_, v) => {
    if (typeof v === "number" && !Number.isFinite(v)) blank = true;
    return v;
  });
  return blank;
}

// problems splits the engine's rejection of a configuration into its parts.
export function problems(message: string): string[] {
  return message
    .replace(/^invalid command: /, "")
    .split("; ")
    .filter((p) => p !== "");
}

// describePattern states a configured source's shape in words.
export function describePattern(p: Pattern): string {
  const r = (v?: number) => `${formatCompact(v ?? 0)} RPS`;
  switch (p.shape) {
    case "constant":
      return r(p.rps);
    case "ramp":
      return `${r(p.rps)} → ${r(p.peakRps)} over ${p.minutes} min`;
    case "spike":
      return `${r(p.rps)}, ${r(p.peakRps)} for ${p.minutes} min after ${p.startMinutes ?? 0} min`;
    case "burst":
      return `${r(p.rps)}, ${r(p.peakRps)} for ${p.minutes} of every ${p.periodMinutes} min`;
    case "periodic":
      return `${r(p.rps)} to ${r(p.peakRps)} every ${p.periodMinutes} min`;
    case "schedule":
      return (p.schedule ?? []).map((s) => `${formatClock(1, s.hour).slice(-5)} ${r(s.rps)}`).join(", ");
  }
}
