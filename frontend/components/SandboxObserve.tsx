"use client";

import { useEffect, useState } from "react";
import { Num } from "./SandboxInternet";
import { formatCompact, problems, type AlertRule, type Monitoring, type SLO, type GameState, type NodeSample, type Span } from "@/lib/sandbox";

type Tab = "failures" | "metrics" | "traces" | "logs" | "alerts" | "slos" | "timeline";

// ObservePanel is what the player's telemetry captured (v14): why requests
// fail and where, monitored components' metrics over time, traces of
// requests through the system, and log lines. What was not captured is
// counted, not shown.
export function ObservePanel({
  game,
  onClose,
  onMonitor,
  initial = "failures",
}: {
  game: GameState;
  onClose: () => void;
  onMonitor: (m: Monitoring) => Promise<string | null>;
  initial?: Tab;
}) {
  const [tab, setTab] = useState<Tab>(initial);
  useEffect(() => {
    const esc = (e: KeyboardEvent) => {
      if (e.key !== "Escape" || document.querySelector("dialog[open]")) return;
      e.stopPropagation();
      onClose();
    };
    window.addEventListener("keydown", esc, true);
    return () => window.removeEventListener("keydown", esc, true);
  }, [onClose]);
  return (
    <div className="sb-inner sb-observe" aria-label="Observe">
      <div className="sb-inner-bar">
        <button className="secondary" onClick={onClose}>
          ← System
        </button>
        <strong>Observe</strong>
        <div className="sb-tabs" role="tablist">
          {(["failures", "metrics", "traces", "logs", "alerts", "slos", "timeline"] as Tab[]).map((t) => (
            <button key={t} role="tab" aria-selected={tab === t} className={tab === t ? "" : "secondary"} onClick={() => setTab(t)}>
              {t === "slos" ? "SLOs" : t[0].toUpperCase() + t.slice(1)}
            </button>
          ))}
        </div>
        <span className="sb-hint">only what your telemetry captured · Esc to go back</span>
      </div>
      <div className="sb-observe-body">
        {tab === "failures" && <Failures game={game} />}
        {tab === "metrics" && <Metrics game={game} />}
        {tab === "traces" && <Traces game={game} />}
        {tab === "logs" && <Logs game={game} />}
        {tab === "alerts" && <Alerts game={game} onMonitor={onMonitor} />}
        {tab === "slos" && <SLOs game={game} onMonitor={onMonitor} />}
        {tab === "timeline" && <Timeline game={game} />}
      </div>
    </div>
  );
}

function Failures({ game }: { game: GameState }) {
  const causes = game.report?.causes ?? [];
  const seen = causes.filter((c) => c.seen && !c.async);
  const unseen = causes.filter((c) => !c.seen && !c.async).reduce((a, c) => a + c.rps, 0);
  const async = causes.filter((c) => c.async && c.seen);
  return (
    <>
      <h4>Why requests fail</h4>
      {seen.length === 0 ? (
        <p className="sb-hint">No failure was logged this tick.</p>
      ) : (
        <table className="sb-breakdown sb-causes">
          <thead>
            <tr>
              <th>Reason</th>
              <th>Where</th>
              <th>Detail</th>
              <th className="num">Rate</th>
            </tr>
          </thead>
          <tbody>
            {seen.map((c) => (
              <tr key={`${c.reason}@${c.place}`}>
                <td className="bad">{c.reason}</td>
                <td>{c.place}</td>
                <td className="sb-hint">{c.detail}</td>
                <td className="num">{formatCompact(c.rps)}/s</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      {unseen > 0.01 && (
        <p className="sb-hint warn" role="status">
          {formatCompact(unseen)}/s more fail where nothing logs errors to a log store: turn on error logs there to see why.
        </p>
      )}
      {async.length > 0 && (
        <>
          <h4>Lost in the background</h4>
          <table className="sb-breakdown">
            <tbody>
              {async.map((c) => (
                <tr key={`${c.reason}@${c.place}@${c.detail}`}>
                  <td className="warn">{c.reason}</td>
                  <td>{c.place}</td>
                  <td className="sb-hint">{c.detail}</td>
                  <td className="num">{formatCompact(c.rps)}/s</td>
                </tr>
              ))}
            </tbody>
          </table>
        </>
      )}
    </>
  );
}

// fromZero draws values against an axis that starts at zero, so a small
// change looks small.
function fromZero(values: number[], width: number, height: number): string {
  const max = Math.max(...values, 0);
  const step = values.length > 1 ? width / (values.length - 1) : 0;
  return values.map((v, i) => `${(i * step).toFixed(1)},${(max > 0 ? height - (v / max) * height : height).toFixed(1)}`).join(" ");
}

function Chart({ title, values, format }: { title: string; values: number[]; format: (v: number) => string }) {
  const max = Math.max(...values, 0);
  const last = values[values.length - 1] ?? 0;
  return (
    <div className="sb-chart">
      <div className="sb-chart-head">
        <span>{title}</span>
        <strong>{format(last)}</strong>
        <span className="sb-hint">max {format(max)}</span>
      </div>
      <svg viewBox="0 0 300 60" preserveAspectRatio="none" aria-label={`${title} over time`}>
        <polyline points={fromZero(values, 300, 60)} />
      </svg>
    </div>
  );
}

function Metrics({ game }: { game: GameState }) {
  const ids = Object.keys(game.nodeHistory ?? {}).filter((id) => (game.nodeHistory?.[id]?.length ?? 0) > 0);
  const [id, setId] = useState(ids[0] ?? "");
  const h: NodeSample[] = game.nodeHistory?.[id] ?? [];
  if (ids.length === 0) return <p className="sb-hint">No component reports metrics to a metrics store yet.</p>;
  return (
    <>
      <label className="sb-field">
        <span>Component</span>
        <select aria-label="Component" value={id} onChange={(e) => setId(e.target.value)}>
          {ids.map((x) => (
            <option key={x}>{x}</option>
          ))}
        </select>
      </label>
      <p className="sb-hint">One sample per tick over the last {h.length} ticks.</p>
      <div className="sb-charts">
        <Chart title="Requests" values={h.map((s) => s.rps)} format={(v) => `${formatCompact(v)}/s`} />
        <Chart title="Errors" values={h.map((s) => s.errors)} format={(v) => `${formatCompact(v)}/s`} />
        <Chart title="Latency" values={h.map((s) => s.latencyMs)} format={(v) => `${v.toFixed(0)} ms`} />
        <Chart title="Utilization" values={h.map((s) => s.utilization)} format={(v) => `${Math.round(v * 100)}%`} />
      </div>
    </>
  );
}

function rows(span: Span, depth: number, out: { span: Span; depth: number }[]) {
  out.push({ span, depth });
  for (const c of span.children ?? []) rows(c, depth + 1, out);
  return out;
}

function Traces({ game }: { game: GameState }) {
  const traces = (game.report?.traces ?? []).filter((t) => t.seen);
  const [k, setK] = useState(0);
  if (traces.length === 0) return <p className="sb-hint">No trace was kept: sample traces on the component traffic reaches first, with a trace backend in the system.</p>;
  const t = traces[Math.min(k, traces.length - 1)];
  const total = Math.max(t.root.durationMs, 1e-9);
  return (
    <>
      <label className="sb-field">
        <span>Trace</span>
        <select aria-label="Trace" value={Math.min(k, traces.length - 1)} onChange={(e) => setK(Number(e.target.value))}>
          {traces.map((x, i) => (
            <option key={`${x.source} ${x.endpoint}`} value={i}>
              {x.source} → {x.endpoint} ({formatCompact(x.rate)}/s kept)
            </option>
          ))}
        </select>
      </label>
      <div className="sb-waterfall" aria-label="Trace waterfall">
        {rows(t.root, 0, []).map(({ span, depth }, i) => (
          <div className="sb-span" key={i}>
            <div className="sb-span-name" style={{ paddingLeft: depth * 14 }} title={span.place}>
              {span.name}
              {span.async ? " (async)" : ""}
            </div>
            <div className="sb-span-track">
              <div
                className={`sb-span-bar ${span.ok < 0.99 ? "bad" : ""} ${span.async ? "async" : ""}`}
                style={{ left: `${(span.startMs / total) * 100}%`, width: `${Math.max(0.5, (span.durationMs / total) * 100)}%` }}
              />
            </div>
            <div className="sb-span-ms">
              {span.durationMs.toFixed(1)} ms{span.ok < 0.99 ? ` · ${Math.round(span.ok * 100)}% ok` : ""}
            </div>
          </div>
        ))}
      </div>
    </>
  );
}

function Logs({ game }: { game: GameState }) {
  const [level, setLevel] = useState("all");
  const logs = (game.report?.logs ?? []).filter((l) => level === "all" || l.level === level);
  return (
    <>
      <label className="sb-field">
        <span>Level</span>
        <select aria-label="Level" value={level} onChange={(e) => setLevel(e.target.value)}>
          {["all", "error", "warn", "info", "debug"].map((l) => (
            <option key={l}>{l}</option>
          ))}
        </select>
      </label>
      {logs.length === 0 ? (
        <p className="sb-hint">No log line was kept: set a log level on a component, with a log store in the system.</p>
      ) : (
        <table className="sb-breakdown sb-logs">
          <tbody>
            {logs.map((l, i) => (
              <tr key={i}>
                <td className={l.level === "error" ? "bad" : l.level === "warn" ? "warn" : "sb-hint"}>{l.level.toUpperCase()}</td>
                <td>{l.place}</td>
                <td>{l.message}</td>
                <td className="num">× {formatCompact(l.rate)}/s</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </>
  );
}

const METRICS = ["error-rate", "errors", "rps", "latency", "p95", "utilization", "health"];

function clock(game: GameState, tick: number, tickSeconds: number) {
  const sec = 8 * 3600 + tick * tickSeconds;
  const day = Math.floor(sec / 86400) + 1;
  const h = Math.floor((sec % 86400) / 3600);
  const m = Math.floor((sec % 3600) / 60);
  return `day ${day} ${String(h).padStart(2, "0")}:${String(m).padStart(2, "0")}`;
}

// Alerts lists the player's rules and where each stands, and adds or removes
// them; a rule on what nobody monitors has no data and never fires.
export function Alerts({ game, onMonitor }: { game: GameState; onMonitor: (m: Monitoring) => Promise<string | null> }) {
  const m: Monitoring = game.monitoring ?? { alerts: [], slos: [] };
  const states = new Map((game.alerts ?? []).map((s) => [s.rule, s]));
  const [draft, setDraft] = useState<AlertRule>({ name: "", node: "", metric: "error-rate", op: ">", threshold: 0.05, forTicks: 2 });
  const [errors, setErrors] = useState<string[]>([]);
  const save = async (alerts: AlertRule[]) => {
    const err = await onMonitor({ alerts, slos: m.slos ?? [] });
    setErrors(err ? problems(err) : []);
    return !err;
  };
  return (
    <>
      <h4>Alert rules</h4>
      {(m.alerts ?? []).length === 0 ? (
        <p className="sb-hint">No rules yet. A rule fires when a metric stays past its threshold; it can only see what is monitored.</p>
      ) : (
        <table className="sb-breakdown sb-alerts">
          <tbody>
            {(m.alerts ?? []).map((r) => {
              const s = states.get(r.name);
              const cls = s?.state === "firing" ? "bad" : s?.state === "pending" ? "warn" : s?.state === "no data" ? "sb-hint" : "ok";
              return (
                <tr key={r.name}>
                  <td className={cls}>{s?.state ?? "no data"}</td>
                  <td>{r.name}</td>
                  <td className="sb-hint">
                    {r.node || "system"} {r.metric} {r.op} {r.threshold} for {r.forTicks} {r.forTicks === 1 ? "tick" : "ticks"}
                  </td>
                  <td className="num">{s && s.state !== "no data" ? (Math.abs(s.value) < 10 ? Number(s.value.toPrecision(3)) : formatCompact(s.value)) : "—"}</td>
                  <td>
                    <button className="secondary danger" onClick={() => save((m.alerts ?? []).filter((x) => x.name !== r.name))}>
                      remove
                    </button>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      )}
      <div className="sb-row">
        <label className="sb-field">
          <span>Name</span>
          <input type="text" aria-label="Alert name" value={draft.name} onChange={(e) => setDraft({ ...draft, name: e.target.value })} />
        </label>
        <label className="sb-field">
          <span>Watch</span>
          <select aria-label="Alert component" value={draft.node} onChange={(e) => setDraft({ ...draft, node: e.target.value })}>
            <option value="">system</option>
            {game.nodes
              .filter((n) => n.kind !== "traffic")
              .map((n) => (
                <option key={n.id}>{n.id}</option>
              ))}
          </select>
        </label>
        <label className="sb-field">
          <span>Metric</span>
          <select aria-label="Alert metric" value={draft.metric} onChange={(e) => setDraft({ ...draft, metric: e.target.value })}>
            {METRICS.map((x) => (
              <option key={x}>{x}</option>
            ))}
          </select>
        </label>
        <label className="sb-field">
          <span>Condition</span>
          <select aria-label="Alert condition" value={draft.op} onChange={(e) => setDraft({ ...draft, op: e.target.value as ">" | "<" })}>
            <option>{">"}</option>
            <option>{"<"}</option>
          </select>
        </label>
        <Num label="Threshold" value={draft.threshold} step={0.01} onChange={(threshold) => setDraft({ ...draft, threshold })} />
        <Num label="For (ticks)" value={draft.forTicks} onChange={(forTicks) => setDraft({ ...draft, forTicks })} />
        <button
          onClick={async () => {
            if (await save([...(m.alerts ?? []), { ...draft, node: draft.node || undefined }])) setDraft({ ...draft, name: "" });
          }}
        >
          Add rule
        </button>
      </div>
      {errors.length > 0 && (
        <ul className="sb-errors" role="alert">
          {errors.map((e) => (
            <li key={e}>{e}</li>
          ))}
        </ul>
      )}
      {(game.alertLog ?? []).length > 0 && (
        <>
          <h4>Fired</h4>
          <table className="sb-breakdown">
            <tbody>
              {[...(game.alertLog ?? [])].reverse().map((e, i) => (
                <tr key={i}>
                  <td className={e.resolved === undefined ? "bad" : "sb-hint"}>{e.rule}</td>
                  <td>fired {clock(game, e.fired, 300)}</td>
                  <td>{e.resolved === undefined ? "still firing" : `resolved ${clock(game, e.resolved, 300)}`}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </>
      )}
    </>
  );
}

// SLOs measures each objective over its window from what was observed.
export function SLOs({ game, onMonitor }: { game: GameState; onMonitor: (m: Monitoring) => Promise<string | null> }) {
  const m: Monitoring = game.monitoring ?? { alerts: [], slos: [] };
  const [draft, setDraft] = useState<SLO>({ name: "", node: "", target: 0.99, windowTicks: 288 });
  const [errors, setErrors] = useState<string[]>([]);
  const save = async (slos: SLO[]) => {
    const err = await onMonitor({ alerts: m.alerts ?? [], slos });
    setErrors(err ? problems(err) : []);
    return !err;
  };
  return (
    <>
      <h4>Service level objectives</h4>
      {(game.slos ?? []).length === 0 ? (
        <p className="sb-hint">No SLOs yet. An SLO is an availability target over a window, measured from what is monitored.</p>
      ) : (
        <table className="sb-breakdown sb-slos">
          <tbody>
            {(game.slos ?? []).map((s) => (
              <tr key={s.name}>
                <td>{s.name}</td>
                <td className="num">
                  {(s.availability * 100).toFixed(2)}% of {(s.target * 100).toFixed(2)}%
                </td>
                <td className={`num ${s.budgetLeft < 0 ? "bad" : s.budgetLeft < 0.25 ? "warn" : "ok"}`}>
                  {s.budgetLeft < 0 ? "budget exhausted" : `budget ${Math.round(s.budgetLeft * 100)}% left`}
                </td>
                <td className={`num ${s.burnRate > 1 ? "bad" : ""}`}>burn ×{s.burnRate.toFixed(1)}</td>
                <td className="sb-hint">{Math.round(s.observed * 100)}% of the window observed</td>
                <td>
                  <button className="secondary danger" onClick={() => save((m.slos ?? []).filter((x) => x.name !== s.name))}>
                    remove
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      <div className="sb-row">
        <label className="sb-field">
          <span>Name</span>
          <input type="text" aria-label="SLO name" value={draft.name} onChange={(e) => setDraft({ ...draft, name: e.target.value })} />
        </label>
        <label className="sb-field">
          <span>For</span>
          <select aria-label="SLO component" value={draft.node} onChange={(e) => setDraft({ ...draft, node: e.target.value })}>
            <option value="">system</option>
            {game.nodes
              .filter((n) => n.kind !== "traffic")
              .map((n) => (
                <option key={n.id}>{n.id}</option>
              ))}
          </select>
        </label>
        <Num label="Target %" value={Math.round(draft.target * 100000) / 1000} step={0.01} max={99.999} onChange={(v) => setDraft({ ...draft, target: v / 100 })} />
        <Num label="Window (ticks)" value={draft.windowTicks} onChange={(windowTicks) => setDraft({ ...draft, windowTicks })} />
        <button
          onClick={async () => {
            if (await save([...(m.slos ?? []), { ...draft, node: draft.node || undefined }])) setDraft({ ...draft, name: "" });
          }}
        >
          Add SLO
        </button>
      </div>
      {errors.length > 0 && (
        <ul className="sb-errors" role="alert">
          {errors.map((e) => (
            <li key={e}>{e}</li>
          ))}
        </ul>
      )}
    </>
  );
}

// Timeline lines up events, alerts, and the player's commands.
export function Timeline({ game }: { game: GameState }) {
  const entries = [...(game.timeline ?? [])].reverse();
  if (entries.length === 0) return <p className="sb-hint">Nothing has happened yet.</p>;
  return (
    <table className="sb-breakdown sb-timeline">
      <tbody>
        {entries.map((e, i) => (
          <tr key={i}>
            <td className="sb-hint">{clock(game, e.tick, 300)}</td>
            <td className={e.kind === "alert" ? "bad" : e.kind === "event" ? "warn" : ""}>{e.kind}</td>
            <td>{e.text}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}
