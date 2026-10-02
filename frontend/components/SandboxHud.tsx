"use client";

import {
  SPEEDS,
  formatClock,
  formatCompact,
  formatMoney,
  scoreLevel,
  sparkline,
  type GameState,
  type Meters,
  type Ruleset,
} from "@/lib/sandbox";
import { formatPercent } from "@/lib/format";

const DARK = "Not monitored: no component that traffic reaches first sends metrics to a metrics store";

function Tile({ label, value, cls, series, dark }: { label: string; value: string; cls?: string; series?: number[]; dark?: boolean }) {
  if (dark) {
    return (
      <div className="sb-tile dark" title={DARK}>
        <div className="sb-tile-label">{label}</div>
        <div className="sb-tile-value">—</div>
      </div>
    );
  }
  return (
    <div className="sb-tile">
      <div className="sb-tile-label">{label}</div>
      <div className={`sb-tile-value ${cls ?? ""}`}>{value}</div>
      {series && series.length > 1 && (
        <svg viewBox="0 0 100 20" preserveAspectRatio="none" className="sb-spark" aria-hidden>
          <polyline points={sparkline(series, 100, 20)} />
        </svg>
      )}
    </div>
  );
}

export function SandboxHud({
  game,
  rules,
  onSpeed,
  onSkip,
  onObserve,
}: {
  game: GameState;
  rules: Ruleset;
  onSpeed: (speed: number) => void;
  onSkip: (ticks: number) => void;
  onObserve?: () => void;
}) {
  const m = game.meters;
  // From v14 the technical meters need monitoring; business numbers do not.
  const dark = !!rules.telemetry && !m.monitored;
  const h = game.history ?? [];
  const series = (f: (m: Meters) => number) => h.map(f);
  const profit = m.revenuePerHour - m.costPerHour;
  return (
    <div className="sb-hud">
      <div className="sb-clock">
        <span className="badge" title="Every Sandbox value is modelled by the core engine, not measured">
          simulated
        </span>
        {m.loadTest && (
          <span className="badge load-test" title="Traffic set by you (a load test): it earns nothing, and users and goals hold still">
            load test
          </span>
        )}
        <strong>{formatClock(m.day, m.hour)}</strong>
        <span className="sb-tier">{m.tier}</span>
        <div className="sb-speed" role="group" aria-label="Speed">
          {SPEEDS.map((s) => (
            <button
              key={s}
              className={game.speed === s ? "" : "secondary"}
              onClick={() => onSpeed(s)}
              disabled={game.status !== "running"}
            >
              {s === 0 ? "❚❚" : `${s}×`}
            </button>
          ))}
          <button className="secondary" onClick={() => onSkip(12)} disabled={game.status !== "running"}>
            +1h
          </button>
          <button className="secondary" onClick={() => onSkip(288)} disabled={game.status !== "running"}>
            +1 day
          </button>
          {rules.telemetry && onObserve && (
            <button onClick={onObserve}>
              Observe
              {(game.alerts ?? []).some((a) => a.state === "firing") && (
                <span className="badge firing"> {(game.alerts ?? []).filter((a) => a.state === "firing").length} firing</span>
              )}
            </button>
          )}
        </div>
      </div>
      <div className="sb-tiles">
        <Tile label="Cash" value={formatMoney(m.cash)} cls={m.cash < 0 ? "bad" : ""} series={series((x) => x.cash)} />
        <Tile
          label="Profit / h"
          value={formatMoney(profit)}
          cls={profit < 0 ? "bad" : "ok"}
          series={series((x) => x.revenuePerHour - x.costPerHour)}
        />
        <Tile label="Revenue / h" value={formatMoney(m.revenuePerHour)} />
        <Tile label="Cost / h" value={formatMoney(m.costPerHour)} />
        <Tile label="Users" value={formatCompact(m.users)} series={series((x) => x.users)} />
        <Tile label="RPS" value={formatCompact(m.rps)} series={series((x) => x.rps)} dark={dark} />
        {!!m.attackRps && !dark && <Tile label="Attack RPS" value={formatCompact(m.attackRps)} cls="bad" />}
        <Tile
          dark={dark}
          label="p95 latency"
          value={`${m.p95LatencyMs.toFixed(0)} ms`}
          series={series((x) => x.p95LatencyMs)}
        />
        <Tile
          dark={dark}
          label="Errors"
          value={formatPercent(m.errorRate)}
          cls={m.errorRate > 0.05 ? "bad" : m.errorRate > 0.01 ? "warn" : "ok"}
          series={series((x) => x.errorRate)}
        />
        <Tile dark={dark} label="Health" value={m.health.toFixed(0)} cls={scoreLevel(m.health)} series={series((x) => x.health)} />
        <Tile
          label="Satisfaction"
          value={m.satisfaction.toFixed(0)}
          cls={scoreLevel(m.satisfaction)}
          series={series((x) => x.satisfaction)}
        />
        <Tile label="Popularity" value={m.popularity.toFixed(0)} series={series((x) => x.popularity)} />
        <Tile label="Complexity" value={m.complexity.toFixed(1)} />
      </div>
    </div>
  );
}
