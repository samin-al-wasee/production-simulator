"use client";

import { useState } from "react";
import {
  formatCompact,
  formatMoney,
  healthLevel,
  problems,
  toPercent,
  type Command,
  type GameState,
  type Ruleset,
  type SandboxNode,
  type Telemetry,
} from "@/lib/sandbox";
import { Num } from "./SandboxInternet";

const pct = (v: number) => `${Math.round(v * 100)}%`;

// TelemetrySection is what a component reports (v14): metrics, logs, and
// traces, each off until the player turns it on, and how much of each a
// backend kept.
export function TelemetrySection({
  game,
  rules,
  node,
  onConfigure,
}: {
  game: GameState;
  rules: Ruleset;
  node: SandboxNode;
  onConfigure: (c: Command) => Promise<string | null>;
}) {
  const base = node.telemetry ?? rules.telemetry;
  const [edit, setEdit] = useState<Telemetry | null>(null);
  const [errors, setErrors] = useState<string[]>([]);
  if (!base) return null;
  const o = game.flow.nodes.find((s) => s.id === node.id)?.obs;
  const t = edit ?? base;
  const set = (p: Partial<Telemetry>) => setEdit({ ...t, ...p });
  return (
    <details className="sb-section sb-telemetry" open={!o?.metrics}>
      <summary>Telemetry</summary>
      {o && (
        <table className="sb-breakdown">
          <tbody>
            <tr>
              <th>Metrics</th>
              <td className={`num ${o.metrics && o.metricsCoverage < 1 ? "warn" : ""}`}>
                {o.metrics ? `every ${base.resolutionSeconds} s · ${o.metricsCoverage > 0 ? `${pct(o.metricsCoverage)} kept` : "no metrics store"}` : "off"}
              </td>
            </tr>
            <tr>
              <th>Logs</th>
              <td className={`num ${o.logLevel !== "off" && o.logCoverage < 1 ? "warn" : ""}`}>
                {o.logLevel === "off" ? "off" : `${o.logLevel} · ${formatCompact(o.logLines)} lines/s · ${o.logCoverage > 0 ? `${pct(o.logCoverage)} kept` : "no log store"}`}
              </td>
            </tr>
            <tr>
              <th>Traces</th>
              <td className={`num ${o.traceSampling > 0 && o.traceCoverage < 1 ? "warn" : ""}`}>
                {o.traceSampling > 0 ? `${pct(o.traceSampling)} sampled · ${formatCompact(o.spans)} spans/s · ${o.traceCoverage > 0 ? `${pct(o.traceCoverage)} kept` : "no trace backend"}` : "off"}
              </td>
            </tr>
          </tbody>
        </table>
      )}
      <div className="sb-row">
        <label>
          <input type="checkbox" aria-label="Metrics" checked={t.metrics} onChange={(e) => set({ metrics: e.target.checked })} /> Metrics
        </label>
        <Num label="Resolution (s)" value={t.resolutionSeconds} onChange={(resolutionSeconds) => set({ resolutionSeconds })} />
      </div>
      <div className="sb-row">
        <label className="sb-field">
          <span>Log level</span>
          <select aria-label="Log level" value={t.logLevel} onChange={(e) => set({ logLevel: e.target.value as Telemetry["logLevel"] })}>
            {["off", "error", "warn", "info", "debug"].map((l) => (
              <option key={l}>{l}</option>
            ))}
          </select>
        </label>
        <Num label="Logs kept %" value={toPercent(t.logSampling)} max={100} onChange={(v) => set({ logSampling: v / 100 })} />
        <Num label="Traces sampled %" value={toPercent(t.traceSampling)} step={0.1} max={100} onChange={(v) => set({ traceSampling: v / 100 })} />
      </div>
      <p className="sb-hint">
        Nothing is seen until it is reported: metrics show this component&apos;s numbers once a metrics store keeps them. Logs and
        traces cost CPU on applications and workers, and every signal costs ingest at its backend.
      </p>
      {errors.length > 0 && (
        <ul className="sb-errors" role="alert">
          {errors.map((e) => (
            <li key={e}>{e}</li>
          ))}
        </ul>
      )}
      {edit && (
        <div className="sb-row">
          <button className="secondary" onClick={() => setEdit(null)}>
            Cancel
          </button>
          <button
            onClick={async () => {
              const err = await onConfigure({ type: "configure", node: node.id, telemetry: edit });
              if (err) setErrors(problems(err));
              else {
                setErrors([]);
                setEdit(null);
              }
            }}
          >
            Apply telemetry
          </button>
        </div>
      )}
    </details>
  );
}

// BackendPanel is a telemetry backend's part of the inspector (v14).
export function BackendPanel({ game, node }: { game: GameState; node: SandboxNode }) {
  const b = game.flow.nodes.find((s) => s.id === node.id)?.backend;
  if (!b) return null;
  const unit = node.kind === "metrics-store" ? "samples/s" : node.kind === "log-store" ? "lines/s" : "spans/s";
  return (
    <div className="sb-internet">
      <table>
        <tbody>
          <tr>
            <th>Health</th>
            <td className={`num ${healthLevel(b.health)}`}>{b.health}</td>
          </tr>
          <tr>
            <th>Ingest</th>
            <td className="num">
              {formatCompact(b.ingest)} of {formatCompact(b.capacity)} {unit}
            </td>
          </tr>
          {b.dropped > 0.01 && (
            <tr>
              <th>Dropped</th>
              <td className="num bad">
                {formatCompact(b.dropped)} {unit}
              </td>
            </tr>
          )}
          <tr>
            <th>Stored</th>
            <td className="num">{b.gbPerDay.toFixed(2)} GB/day</td>
          </tr>
          <tr>
            <th>Cost</th>
            <td className="num">{formatMoney(b.cost)}/h</td>
          </tr>
        </tbody>
      </table>
      <p className="sb-hint">Every instrumented component sends here; more replicas or a larger size take in more.</p>
    </div>
  );
}
