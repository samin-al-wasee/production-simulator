"use client";

import { useEffect, useState } from "react";
import { formatCompact, type GameState, type NodeSample, type Span } from "@/lib/sandbox";

type Tab = "failures" | "metrics" | "traces" | "logs";

// ObservePanel is what the player's telemetry captured (v14): why requests
// fail and where, monitored components' metrics over time, traces of
// requests through the system, and log lines. What was not captured is
// counted, not shown.
export function ObservePanel({ game, onClose }: { game: GameState; onClose: () => void }) {
  const [tab, setTab] = useState<Tab>("failures");
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
          {(["failures", "metrics", "traces", "logs"] as Tab[]).map((t) => (
            <button key={t} role="tab" aria-selected={tab === t} className={tab === t ? "" : "secondary"} onClick={() => setTab(t)}>
              {t[0].toUpperCase() + t.slice(1)}
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
