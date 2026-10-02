"use client";

import { useEffect, useRef, useState } from "react";
import {
  DEPS,
  appConfig,
  formatCompact,
  hasBlankNumber,
  healthLevel,
  problems,
  toPercent,
  type AppConfig,
  type AppRoute,
  type Call,
  type Command,
  type GameState,
  type NodeStats,
  type Processing,
  type Ruleset,
  type SandboxNode,
} from "@/lib/sandbox";
import { InnerView, Num, graph } from "./SandboxInternet";

const BOTTLENECK: Record<string, string> = {
  cpu: "CPU",
  slots: "workers / concurrency slots",
  connections: "connections",
  "network-in": "inbound network",
  "network-out": "outbound network",
};

// bottleneckLabel names a bottleneck; a connection pool is named by its target.
export const bottleneckLabel = (b: string) => (b.startsWith("pool:") ? `connection pool to ${b.slice(5)}` : (BOTTLENECK[b] ?? b));

// The canvas kinds a route's dependency reaches, in order of preference: a
// cache call falls back to the database, a write goes to a queue when one is
// connected.
// ponytail: mirrors depTargets in backend/internal/sandbox/app.go; move to the API if they drift
const DEP_KINDS: Record<string, string[][]> = {
  cache: [["cache"], ["db-primary", "db-replica"]],
  "db-read": [["db-primary", "db-replica"]],
  "db-write": [["queue"], ["db-primary"]],
  queue: [["queue"]],
  storage: [["object-storage"]],
};

// trafficInputs are the traffic components sending to an application, with
// the engine's view of each (v6).
function trafficInputs(game: GameState, id: string) {
  return game.edges
    .filter((e) => e.to === id && game.nodes.find((n) => n.id === e.from)?.kind === "traffic")
    .map((e) => ({ id: e.from, stats: game.flow.nodes.find((s) => s.id === e.from)?.traffic }));
}

// AppView opens an application instance: a request's path through its
// connections, backlog, workers, middleware, routes, and dependencies. Every
// value is the engine's, over all replicas; edges show each route's share.
export function AppView({
  game,
  rules,
  node,
  onBack,
}: {
  game: GameState;
  rules: Ruleset;
  node: SandboxNode;
  onBack: () => void;
}) {
  const config = appConfig(game, rules, node);
  if (!config) return null;
  const a = game.flow.nodes.find((s) => s.id === node.id)?.app;
  const at = (b: string) => (a?.bottleneck === b ? "bad" : undefined);
  const g = graph((a?.success ?? 0) + (a?.errors ?? 0) + (a?.timeouts ?? 0) > 0);
  const total = (a?.routes ?? []).reduce((sum, r) => sum + r.rps, 0);
  const n = (v = 0) => formatCompact(v);
  const inputs = trafficInputs(game, node.id);
  if (inputs.length > 0) {
    g.column(inputs.map(({ id, stats: t }) => ({
      id: `src:${id}`,
      title: id,
      sub: t?.problem ?? `${n((t?.rps ?? 0) + (t?.retryRps ?? 0))}/s`,
      cls: t?.problem ? "bad" : undefined,
    })), "input");
  }

  g.column([{
    id: "in",
    title: "Connections",
    sub: `${n(total)}/s · ${n(a?.connections)} open${a?.rejected ? ` · ${n(a.rejected)}/s rejected` : ""}${config.tls ? " · TLS" : ""}${config.keepAlive ? " · keep-alive" : ""}`,
    cls: at("connections") ?? at("network-in") ?? (a && a.rejected > 0.01 ? "bad" : undefined),
  }], inputs.length > 0 ? undefined : "input");
  const sent = inputs.reduce((sum, { stats: t }) => sum + (t?.problem ? 0 : (t?.rps ?? 0) + (t?.retryRps ?? 0)), 0);
  for (const { id, stats: t } of inputs) {
    if (!t?.problem) g.link(`src:${id}`, "in", sent > 0 ? ((t?.rps ?? 0) + (t?.retryRps ?? 0)) / sent : 1 / inputs.length);
  }
  g.column([{
    id: "backlog",
    title: "Backlog",
    sub: `${n(a?.queued)} / ${config.backlog} queued · waits ${(a?.waitMs ?? 0).toFixed(0)} ms`,
    cls: a && a.queued > 0.5 ? "warn" : undefined,
  }]);
  g.column([{
    id: "workers",
    title: config.processing === "sync" ? `${config.workers} sync workers` : `${config.workers} async workers × ${config.maxConcurrency}`,
    sub: `${n(a?.active)} in flight · ${(a?.cpuUsed ?? 0).toFixed(2)} / ${a?.cpuTotal ?? 0} vCPU · ${n(a?.memoryMb)} / ${n(a?.memoryTotalMb)} MB`,
    cls: at("cpu") ?? at("slots") ?? (a?.outOfMemory ? "bad" : undefined),
  }]);
  g.link("in", "backlog", 1);
  g.link("backlog", "workers", 1);
  let last = "workers";
  const mw = (config.middleware ?? []).map((name) => rules.middleware?.find((x) => x.name === name) ?? { name, label: name, ms: 0, cpuMs: 0 });
  if (mw.length) {
    g.column([{
      id: "mw",
      title: `Middleware: ${mw.map((m) => m.label).join(" → ")}`,
      sub: `+${mw.reduce((t, m) => t + m.ms, 0)} ms · ${mw.reduce((t, m) => t + m.cpuMs, 0)} CPU-ms per request`,
    }]);
    g.link(last, "mw", 1);
    last = "mw";
  }

  const routes: { endpoint: string; rps: number; errors: number; timeouts: number; latencyMs: number; notFound?: boolean }[] =
    a?.routes ?? config.routes.map((r) => ({ endpoint: r.endpoint, rps: 0, errors: 0, timeouts: 0, latencyMs: 0 }));
  g.column(routes.map((r) => ({
    id: `rt:${r.endpoint}`,
    title: r.endpoint,
    sub: r.notFound
      ? `${n(r.rps)}/s · 404: no route`
      : `${n(r.rps)}/s · ${r.latencyMs.toFixed(0)} ms${r.errors > 0.01 ? ` · ${n(r.errors)}/s errors` : ""}${r.timeouts > 0.01 ? ` · ${n(r.timeouts)}/s timeouts` : ""}`,
    cls: r.errors + r.timeouts > 0.01 ? "bad" : undefined,
  })));
  const share = (rps: number) => (total > 0 ? rps / total : 1 / routes.length);
  const depsOf = (endpoint: string) =>
    (config.routes.find((r) => r.endpoint === endpoint) ?? config.routes.find((r) => r.endpoint === "*"))?.deps ?? [];
  for (const r of routes) g.link(last, `rt:${r.endpoint}`, share(r.rps));

  const used = DEPS.filter((d) => routes.some((r) => depsOf(r.endpoint).includes(d)));
  const targets = (dep: string) => {
    const out = game.edges.filter((e) => e.from === node.id).map((e) => game.nodes.find((x) => x.id === e.to));
    for (const kinds of DEP_KINDS[dep]) {
      const hit = out.filter((x) => x && kinds.includes(x.kind)).map((x) => x!.id);
      if (hit.length) return hit;
    }
    return [];
  };
  const routeOf = (endpoint: string) => config.routes.find((r) => r.endpoint === endpoint) ?? config.routes.find((r) => r.endpoint === "*");
  if (rules.listeners) {
    // From v7 every connection reports what it carried: one node per
    // target, linked from the routes whose dependencies or calls reach it.
    const out = game.edges.filter((e) => e.from === node.id);
    g.column(out.map((e) => {
      const es = game.flow.edges?.find((x) => x.from === e.from && x.to === e.to);
      const to = game.nodes.find((x) => x.id === e.to);
      const failing = !!es && es.rps > 0 && es.errors / es.rps > 0.01;
      const name = to?.kind === "app-instance" ? appConfig(game, rules, to)?.name : undefined;
      return {
        id: `t:${e.to}`,
        title: name ? `${e.to} (${name})` : e.to,
        sub: es?.problem ?? `${n(es?.rps)}/s · ${(es?.latencyMs ?? 0).toFixed(0)} ms${failing ? ` · ${n(es!.errors)}/s failed` : ""}${e.conn ? ` · ${e.conn.protocol}` : ""}`,
        cls: es?.problem || failing ? "bad" : undefined,
      };
    }));
    for (const r of routes) {
      const rt = routeOf(r.endpoint);
      const reach = new Set<string>();
      for (const d of rt?.deps ?? []) targets(d).forEach((id) => reach.add(id));
      for (const cl of rt?.calls ?? []) {
        for (const e of out) {
          const to = game.nodes.find((x) => x.id === e.to);
          if (to?.kind === "app-instance" && appConfig(game, rules, to)?.name === cl.service) reach.add(e.to);
        }
      }
      for (const id of reach) g.link(`rt:${r.endpoint}`, `t:${id}`, share(r.rps));
    }
  } else {
    g.column(used.map((d) => {
      const to = targets(d);
      return { id: `d:${d}`, title: d, sub: to.length ? `→ ${to.join(", ")}` : "not connected: calls fail", cls: to.length ? undefined : "bad" };
    }));
    for (const r of routes) for (const d of depsOf(r.endpoint)) g.link(`rt:${r.endpoint}`, `d:${d}`, share(r.rps));
  }

  g.column([{
    id: "out",
    title: "Response",
    sub: `${n(a?.success)}/s ok · ${n(a?.errors)}/s errors · ${n(a?.timeouts)}/s timeouts`,
    cls: at("network-out") ?? ((a?.errors ?? 0) + (a?.timeouts ?? 0) > 0.01 ? "bad" : undefined),
  }], "output");
  for (const r of routes) g.link(`rt:${r.endpoint}`, "out", share(r.rps));

  return (
    <InnerView
      title={`Inside ${node.id}`}
      hint={
        <>
          {config.name} · {config.framework} · ×{node.replicas}
          {a && <> · <span className={healthLevel(a.health)}>{a.health}</span>, bottleneck {bottleneckLabel(a.bottleneck)}</>} · values over all replicas
        </>
      }
      graph={g}
      onBack={onBack}
    />
  );
}

// AppPanel is an application instance's part of the inspector: its runtime
// state, every value from the engine, and the way into its configuration.
export function AppPanel({
  game,
  rules,
  node,
  stats,
  onConfigure,
}: {
  game: GameState;
  rules: Ruleset;
  node: SandboxNode;
  stats?: NodeStats;
  onConfigure: (c: Command) => Promise<string | null>;
}) {
  const [open, setOpen] = useState(false);
  const config = appConfig(game, rules, node);
  const a = stats?.app;
  const size = rules.sizes.find((s) => s.name === node.size);
  const inputs = trafficInputs(game, node.id);
  if (!config) return null;
  const rows: [string, string, string?][] = a
    ? [
        ["Health", a.health, healthLevel(a.health)],
        ["Bottleneck", bottleneckLabel(a.bottleneck)],
        ["CPU", `${a.cpuUsed.toFixed(2)} / ${a.cpuTotal} vCPU`],
        ["Memory", `${formatCompact(a.memoryMb)} / ${formatCompact(a.memoryTotalMb)} MB`, a.outOfMemory ? "bad" : undefined],
        ["In flight", formatCompact(a.active)],
        ["Queued", `${formatCompact(a.queued)} · waits ${a.waitMs.toFixed(0)} ms`, a.queued > 0.5 ? "warn" : undefined],
        ["Connections", formatCompact(a.connections)],
        ["Succeeded", `${formatCompact(a.success)}/s`],
        ["Errors", `${formatCompact(a.errors)}/s`, a.errors > 0.01 ? "bad" : undefined],
        ["Timed out", `${formatCompact(a.timeouts)}/s`, a.timeouts > 0.01 ? "bad" : undefined],
        ["Rejected", `${formatCompact(a.rejected)}/s`, a.rejected > 0.01 ? "bad" : undefined],
      ]
    : [];
  return (
    <div className="sb-internet">
      <div className="meta">
        {config.name} · {config.framework} · {config.processing}, {config.workers} workers · {size?.vcpu} vCPU, {size?.memoryGb} GB
      </div>
      {a && (
        <table>
          <tbody>
            {rows.map(([k, v, cls]) => (
              <tr key={k}>
                <th>{k}</th>
                <td className={`num ${cls ?? ""}`}>{v}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      <button onClick={() => setOpen(true)}>Configure app</button>
      {inputs.length > 0 && (
        <details className="sb-section">
          <summary>Inputs</summary>
          <table className="sb-breakdown">
            <tbody>
              {inputs.map(({ id, stats: t }) => (
                <tr key={id} title={t?.problem}>
                  <th>{id}</th>
                  <td className={`num ${t?.problem ? "bad" : ""}`}>{t?.problem ? "refused" : `${formatCompact((t?.rps ?? 0) + (t?.retryRps ?? 0))}/s`}</td>
                  <td className="num">{t?.problem ? "" : `${formatCompact(t?.success ?? 0)}/s ok`}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </details>
      )}
      {a?.routes && (
        <details className="sb-section">
          <summary>Routes</summary>
          <table className="sb-breakdown">
            <tbody>
              {a.routes.map((r) => (
                <tr key={r.endpoint} title={`ok ${formatCompact(r.success)}/s · errors ${formatCompact(r.errors)}/s · timeouts ${formatCompact(r.timeouts)}/s · rejected ${formatCompact(r.rejected)}/s`}>
                  <th>{r.endpoint}</th>
                  <td className="num">{formatCompact(r.rps)}/s</td>
                  <td className="num">{r.latencyMs.toFixed(0)} ms</td>
                </tr>
              ))}
            </tbody>
          </table>
        </details>
      )}
      {open && (
        <AppDialog
          rules={rules}
          config={config}
          memoryMb={(size?.memoryGb ?? 0) * 1024}
          onApply={(app) => onConfigure({ type: "configure", node: node.id, app })}
          onClose={() => setOpen(false)}
        />
      )}
    </div>
  );
}

function Text({ label, value, onChange }: { label: string; value: string; onChange: (v: string) => void }) {
  return (
    <label className="sb-field">
      <span>{label}</span>
      <input type="text" aria-label={label} value={value} onChange={(e) => onChange(e.target.value)} />
    </label>
  );
}

export function Check({ label, checked, onChange }: { label: string; checked: boolean; onChange: (v: boolean) => void }) {
  return (
    <label>
      <input type="checkbox" checked={checked} onChange={(e) => onChange(e.target.checked)} /> {label}
    </label>
  );
}

// NewAppDialog is how an application instance is placed when the ruleset
// offers templates: what it serves (an application type, which sets its
// routes) on which stack (which sets how it serves them), or a configuration
// defined by hand. Nothing is placed until the player confirms, and the
// engine validates either.
export function NewAppDialog({
  rules,
  onPlace,
  onClose,
}: {
  rules: Ruleset;
  onPlace: (app: AppConfig) => Promise<string | null>;
  onClose: () => void;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  const types = rules.appTypes ?? [];
  const stacks = rules.appStacks ?? [];
  const [type, setType] = useState(0);
  const [stack, setStack] = useState(0);
  const [manual, setManual] = useState(false);
  const [errors, setErrors] = useState<string[]>([]);
  const [busy, setBusy] = useState(false);
  const memoryMb = (rules.sizes[0]?.memoryGb ?? 0) * 1024;

  useEffect(() => {
    const d = dialog.current;
    d?.showModal();
    return () => d?.close();
  }, [manual]);

  if (manual && rules.app) {
    return <AppDialog rules={rules} config={rules.app} memoryMb={memoryMb} title="New application instance" onApply={onPlace} onClose={onClose} />;
  }
  const chosen = (): AppConfig | undefined => {
    const s = stacks[stack]?.app;
    const t = types[type];
    return s && t && { ...s, name: `${t.name} API`, routes: t.routes };
  };
  const place = async () => {
    const app = chosen();
    if (!app) return;
    setBusy(true);
    const err = await onPlace(app);
    setBusy(false);
    if (err) setErrors(problems(err));
    else onClose();
  };
  const t = types[type];
  return (
    <dialog ref={dialog} className="sb-dialog" aria-label="New application instance" onCancel={onClose}>
      <h3>New application instance</h3>
      <p className="sb-hint">
        Pick what it serves and the stack it runs on, then tune it later with Configure app, or{" "}
        <button className="link" onClick={() => setManual(true)}>
          define everything manually
        </button>
        .
      </p>
      <div className="sb-template-grid">
      <fieldset className="sb-templates">
        <legend>Application type</legend>
        {types.map((x, i) => (
          <label key={x.name} className={`sb-template ${type === i ? "selected" : ""}`}>
            <input type="radio" name="app-type" checked={type === i} onChange={() => setType(i)} />
            <span>
              <strong>{x.name}</strong>
              <span className="sb-hint">{x.description}</span>
            </span>
          </label>
        ))}
      </fieldset>
      <fieldset className="sb-templates">
        <legend>Stack</legend>
        {stacks.map((x, i) => (
          <label key={x.name} className={`sb-template ${stack === i ? "selected" : ""}`}>
            <input type="radio" name="app-stack" checked={stack === i} onChange={() => setStack(i)} />
            <span>
              <strong>{x.name}</strong>
              <span className="sb-hint">
                {x.app.framework} · {x.app.processing}, {x.app.workers} {x.app.workers === 1 ? "worker" : "workers"}
                {x.app.processing === "async" ? ` × ${x.app.maxConcurrency}` : ""} · port {x.app.port} · {x.app.middleware?.length ?? 0} middleware
              </span>
            </span>
          </label>
        ))}
      </fieldset>
      </div>
      {t && (
        <p className="sb-hint">
          {t.name} routes, with a typical client&apos;s share of requests:{" "}
          {t.routes
            .filter((r) => r.endpoint !== "*")
            .map((r) => `${r.endpoint}${r.share ? ` ${Math.round(r.share * 100)}%` : ""}`)
            .join(" · ")}
        </p>
      )}
      {errors.length > 0 && (
        <ul className="sb-errors" role="alert">
          {errors.map((e) => (
            <li key={e}>{e}</li>
          ))}
        </ul>
      )}
      <div className="sb-dialog-actions">
        <button className="secondary" onClick={onClose}>
          Cancel
        </button>
        <button onClick={place} disabled={busy}>
          Place
        </button>
      </div>
    </dialog>
  );
}

function AppDialog({
  rules,
  config,
  memoryMb,
  title = "Application instance",
  onApply,
  onClose,
}: {
  rules: Ruleset;
  config: AppConfig;
  memoryMb: number;
  title?: string;
  onApply: (c: AppConfig) => Promise<string | null>;
  onClose: () => void;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  // Taken once, so live updates never overwrite the player's edits.
  const [c, setC] = useState<AppConfig>(() => structuredClone({ ...config, middleware: config.middleware ?? [] }));
  const [errors, setErrors] = useState<string[]>([]);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    const d = dialog.current;
    d?.showModal();
    return () => d?.close();
  }, []);

  const set = (p: Partial<AppConfig>) => setC((x) => ({ ...x, ...p }));
  const setRoute = (i: number, p: Partial<AppRoute>) => setC((x) => ({ ...x, routes: x.routes.map((r, j) => (j === i ? { ...r, ...p } : r)) }));
  const mw = c.middleware ?? [];
  const idleMb = c.workers * (rules.appRuntime?.workerMemoryMb ?? 0);

  const apply = async () => {
    if (hasBlankNumber(c)) {
      setErrors(["Fill in every number field."]);
      return;
    }
    setBusy(true);
    const err = await onApply(c);
    setBusy(false);
    if (err) setErrors(problems(err));
    else onClose();
  };

  return (
    <dialog ref={dialog} className="sb-dialog" aria-label={title} onCancel={onClose}>
      <h3>{title}</h3>
      <p className="sb-hint">
        Simulated: one backend web/API service per replica. Its capacity is not a number you set; it comes from the CPU,
        workers, connections, and network below, under what each route costs. CPU and memory come from the size.
      </p>

      <details className="sb-section" open>
        <summary>Application</summary>
        <div className="sb-row">
          <Text label="Name" value={c.name} onChange={(name) => set({ name })} />
          <label className="sb-field">
            <span>Framework</span>
            <select
              aria-label="Framework"
              value={c.framework}
              onChange={(e) => {
                const f = rules.frameworks?.find((x) => x.name === e.target.value);
                set(f ? { framework: f.name, interface: f.interface, server: f.server, processing: f.processing, workers: f.workers, maxConcurrency: f.maxConcurrency } : { framework: e.target.value });
              }}
            >
              {(rules.frameworks ?? []).map((f) => (
                <option key={f.name}>{f.name}</option>
              ))}
            </select>
          </label>
          <Text label="Version" value={c.version} onChange={(version) => set({ version })} />
          <Text label="Environment" value={c.environment} onChange={(environment) => set({ environment })} />
        </div>
        <p className="sb-hint">A framework fills in its usual server and workers; the name, version, and environment are labels.</p>
      </details>

      <details className="sb-section" open>
        <summary>Server and concurrency</summary>
        <div className="sb-row">
          <label className="sb-field">
            <span>Processing</span>
            <select aria-label="Processing" value={c.processing} onChange={(e) => set({ processing: e.target.value as Processing })}>
              <option value="sync">sync: a worker per request, held while it waits</option>
              <option value="async">async: requests wait without holding a worker</option>
            </select>
          </label>
          <Num label="Workers" value={c.workers} onChange={(workers) => set({ workers })} />
          {c.processing === "async" && (
            <Num label="Max concurrent requests" value={c.maxConcurrency} onChange={(maxConcurrency) => set({ maxConcurrency })} />
          )}
          <Num label="Backlog" value={c.backlog} onChange={(backlog) => set({ backlog })} />
          <Num label="Max connections" value={c.maxConnections} onChange={(maxConnections) => set({ maxConnections })} />
          <Num label="Timeout (ms)" value={c.timeoutMs} onChange={(timeoutMs) => set({ timeoutMs })} />
        </div>
        <p className={`sb-hint ${idleMb > memoryMb ? "bad" : ""}`}>
          Workers alone take {c.workers} × {rules.appRuntime?.workerMemoryMb} MB = {formatCompact(idleMb)} MB of this size&apos;s{" "}
          {formatCompact(memoryMb)} MB; requests in flight and queued need the rest. Out of memory, the instance crashes and restarts.
        </p>
        <div className="sb-row">
          <Check label="TLS (handshake CPU per new connection)" checked={c.tls} onChange={(tls) => set({ tls })} />
          <Check label="Keep-alive (fewer handshakes, idle connections)" checked={c.keepAlive} onChange={(keepAlive) => set({ keepAlive })} />
        </div>
        <div className="sb-row">
          <Text label="Protocol" value={c.protocol} onChange={(protocol) => set({ protocol })} />
          <Num label="Port" value={c.port} onChange={(port) => set({ port })} />
          <Text label="Interface" value={c.interface} onChange={(i) => set({ interface: i })} />
          <Text label="Server" value={c.server} onChange={(server) => set({ server })} />
        </div>
      </details>

      <details className="sb-section">
        <summary>Middleware</summary>
        <p className="sb-hint">Runs before every route, in this order; each stage adds time and CPU to every request.</p>
        {(rules.middleware ?? []).map((m) => (
          <div className="sb-row" key={m.name}>
            <Check
              label={`${m.label} (+${m.ms} ms, ${m.cpuMs} CPU-ms)`}
              checked={mw.includes(m.name)}
              onChange={(on) => set({ middleware: on ? [...mw, m.name] : mw.filter((x) => x !== m.name) })}
            />
            {m.name === "rate-limit" && mw.includes(m.name) && (
              <Num label="Limit (req/s per replica)" value={c.rateLimitRps} onChange={(rateLimitRps) => set({ rateLimitRps })} />
            )}
          </div>
        ))}
      </details>

      <details className="sb-section">
        <summary>Routes</summary>
        <p className="sb-hint">
          What each endpoint costs and calls. The <code>*</code> route handles endpoints without their own. A call to a
          component that is not connected fails the request; a cache call falls back to the database.
          {rules.listeners && " A call to a service names it (its app name) and an endpoint; it reaches the connected instances with that name. An async call is sent and not waited for."}
        </p>
        {c.routes.map((r, i) => (
          <div className="sb-group" key={i}>
            <div className="sb-row">
              <Text label="Endpoint" value={r.endpoint} onChange={(endpoint) => setRoute(i, { endpoint })} />
              <button
                className="secondary danger"
                disabled={r.endpoint === "*"}
                onClick={() => setC((x) => ({ ...x, routes: x.routes.filter((_, j) => j !== i) }))}
              >
                remove
              </button>
            </div>
            <div className="sb-grid">
              <Num label="Base time (ms)" value={r.baseMs} onChange={(baseMs) => setRoute(i, { baseMs })} />
              <Num label="CPU time (ms)" value={r.cpuMs} onChange={(cpuMs) => setRoute(i, { cpuMs })} />
              <Num label="Memory (MB)" value={r.memoryMb} onChange={(memoryMb) => setRoute(i, { memoryMb })} />
              <Num label="Request (KB)" value={r.requestKb} onChange={(requestKb) => setRoute(i, { requestKb })} />
              <Num label="Response (KB)" value={r.responseKb} onChange={(responseKb) => setRoute(i, { responseKb })} />
              <Num label="Errors %" value={toPercent(r.errorRate ?? 0)} step={0.1} max={100} onChange={(v) => setRoute(i, { errorRate: v / 100 })} />
              <Num label="Typical share %" value={toPercent(r.share ?? 0)} step={0.1} max={100} onChange={(v) => setRoute(i, { share: v / 100 })} />
            </div>
            <div className="sb-row">
              {DEPS.map((d) => (
                <Check
                  key={d}
                  label={d}
                  checked={(r.deps ?? []).includes(d)}
                  onChange={(on) => setRoute(i, { deps: on ? [...(r.deps ?? []), d] : (r.deps ?? []).filter((x) => x !== d) })}
                />
              ))}
            </div>
            {rules.listeners && (
              <div className="sb-calls">
                {(r.calls ?? []).map((cl, k) => {
                  const setCall = (p: Partial<Call>) => setRoute(i, { calls: (r.calls ?? []).map((x, j) => (j === k ? { ...x, ...p } : x)) });
                  return (
                    <div className="sb-row" key={k}>
                      <Text label={`Route ${i + 1} call ${k + 1} service`} value={cl.service} onChange={(service) => setCall({ service })} />
                      <Text label={`Route ${i + 1} call ${k + 1} endpoint`} value={cl.endpoint} onChange={(endpoint) => setCall({ endpoint })} />
                      <Check label="async" checked={!!cl.async} onChange={(async) => setCall({ async })} />
                      <button className="secondary danger" onClick={() => setRoute(i, { calls: (r.calls ?? []).filter((_, j) => j !== k) })}>
                        remove call
                      </button>
                    </div>
                  );
                })}
                <button
                  className="secondary"
                  disabled={(r.calls ?? []).length >= 8}
                  onClick={() => setRoute(i, { calls: [...(r.calls ?? []), { service: "Orders API", endpoint: "POST /orders" }] })}
                >
                  Add call to a service
                </button>
              </div>
            )}
          </div>
        ))}
        <button
          className="secondary"
          onClick={() =>
            setC((x) => ({
              ...x,
              routes: [...x.routes, { endpoint: "GET /new", baseMs: 20, cpuMs: 10, memoryMb: 1, requestKb: 1, responseKb: 5, deps: [] }],
            }))
          }
        >
          Add route
        </button>
      </details>

      {errors.length > 0 && (
        <ul className="sb-errors" role="alert">
          {errors.map((e) => (
            <li key={e}>{e}</li>
          ))}
        </ul>
      )}
      <div className="sb-dialog-actions">
        <button className="secondary" onClick={onClose}>
          Cancel
        </button>
        <button onClick={apply} disabled={busy}>
          Apply
        </button>
      </div>
    </dialog>
  );
}
