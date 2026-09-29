"use client";

import { formatCompact, formatMoney, level, type Command, type GameState, type Ruleset } from "@/lib/sandbox";

export const KIND_DRAG_TYPE = "application/x-forgelab-kind";

export function SandboxPalette({
  rules,
  cash,
  onPlace,
}: {
  rules: Ruleset;
  cash: number;
  onPlace: (kind: string) => void;
}) {
  return (
    <aside className="sb-palette">
      <h3>Build</h3>
      <p className="sb-hint">Drag onto the canvas or click to place. Wire components by dragging between handles.</p>
      {rules.kinds
        .filter((k) => k.name !== "internet")
        .map((k) => {
          const affordable = cash >= k.buildCost;
          return (
            <button
              key={k.name}
              className="sb-kind secondary"
              draggable={affordable}
              disabled={!affordable}
              onDragStart={(e) => {
                e.dataTransfer.setData(KIND_DRAG_TYPE, k.name);
                e.dataTransfer.effectAllowed = "move";
              }}
              onClick={() => onPlace(k.name)}
              title={`${formatCompact(k.capacity)} ops/s · ${k.serviceMs} ms · complexity ${k.complexity}`}
            >
              <span className="sb-kind-label">{k.label}</span>
              <span className="sb-kind-cost">
                {formatMoney(k.buildCost)} + {formatMoney(k.costPerHour)}/h
              </span>
            </button>
          );
        })}
    </aside>
  );
}

export function SandboxInspector({
  game,
  rules,
  selected,
  onCommand,
}: {
  game: GameState;
  rules: Ruleset;
  selected: string | null;
  onCommand: (c: Command) => void;
}) {
  const node = game.nodes.find((n) => n.id === selected);
  if (!node) {
    return (
      <aside className="sb-inspector">
        <h3>Inspector</h3>
        <p className="sb-hint">
          Select a component to see its load and change it. A new world is empty: connect the Internet to an
          application instance, give it a database and object storage, and users start paying.
        </p>
      </aside>
    );
  }
  const kind = rules.kinds.find((k) => k.name === node.kind);
  const stats = game.flow.nodes.find((s) => s.id === node.id);
  const size = rules.sizes.find((s) => s.name === node.size);
  const internet = node.kind === "internet";
  const replicaCost = (kind?.buildCost ?? 0) * (size?.costFactor ?? 1);
  const downstream = game.edges.filter((e) => e.from === node.id).map((e) => e.to);

  return (
    <aside className="sb-inspector">
      <h3>{kind?.label ?? node.kind}</h3>
      <div className="meta">{node.id}</div>
      {stats && (
        <table>
          <tbody>
            <tr>
              <th>Offered</th>
              <td className="num">{formatCompact(stats.offered)}/s</td>
            </tr>
            <tr>
              <th>Served</th>
              <td className="num">{formatCompact(stats.served)}/s</td>
            </tr>
            {!internet && (
              <>
                <tr>
                  <th>Dropped</th>
                  <td className={`num ${stats.dropped > 0.01 ? "bad" : ""}`}>{formatCompact(stats.dropped)}/s</td>
                </tr>
                <tr>
                  <th>Capacity</th>
                  <td className="num">{formatCompact(stats.capacity)}/s</td>
                </tr>
                <tr>
                  <th>Utilization</th>
                  <td className={`num ${level(stats.utilization)}`}>{(stats.utilization * 100).toFixed(0)}%</td>
                </tr>
                <tr>
                  <th>Latency</th>
                  <td className="num">{stats.latencyMs.toFixed(1)} ms</td>
                </tr>
                {node.kind === "queue" && (
                  <tr>
                    <th>Backlog</th>
                    <td className="num">{formatCompact(stats.backlog ?? 0)}</td>
                  </tr>
                )}
                <tr>
                  <th>Cost</th>
                  <td className="num">{formatMoney(stats.costPerHour)}/h</td>
                </tr>
              </>
            )}
          </tbody>
        </table>
      )}

      {!internet && (
        <div className="sb-actions">
          <label>
            Size
            <select value={node.size} onChange={(e) => onCommand({ type: "resize", node: node.id, size: e.target.value })}>
              {rules.sizes.map((s) => (
                <option key={s.name} value={s.name}>
                  {s.name} (×{s.capacityFactor} capacity, ×{s.costFactor} cost)
                </option>
              ))}
            </select>
          </label>
          <div className="sb-replicas">
            Replicas
            <button
              className="secondary"
              disabled={node.replicas <= 1}
              onClick={() => onCommand({ type: "scale", node: node.id, replicas: node.replicas - 1 })}
            >
              −
            </button>
            <strong>{node.replicas}</strong>
            <button
              className="secondary"
              disabled={node.replicas >= rules.maxReplicas || game.meters.cash < replicaCost}
              onClick={() => onCommand({ type: "scale", node: node.id, replicas: node.replicas + 1 })}
              title={`+1 replica costs ${formatMoney(replicaCost)}`}
            >
              +
            </button>
          </div>
          <button className="secondary danger" onClick={() => onCommand({ type: "remove", node: node.id })}>
            Remove
          </button>
        </div>
      )}

      <h4>Sends traffic to</h4>
      {downstream.length === 0 ? (
        <p className="sb-hint">
          Nothing yet. It can connect to:{" "}
          {(kind?.connectsTo ?? []).map((c) => rules.kinds.find((k) => k.name === c)?.label ?? c).join(", ") || "—"}
        </p>
      ) : (
        <ul className="sb-links">
          {downstream.map((to) => (
            <li key={to}>
              {to}{" "}
              <button className="secondary" onClick={() => onCommand({ type: "disconnect", from: node.id, to })}>
                disconnect
              </button>
            </li>
          ))}
        </ul>
      )}
    </aside>
  );
}
