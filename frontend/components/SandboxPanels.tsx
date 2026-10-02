"use client";

import {
  BACKENDS,
  formatCompact,
  seen,
  formatMoney,
  level,
  lockedBy,
  type Command,
  type GameState,
  type GoalStatus,
  type Ruleset,
} from "@/lib/sandbox";
import { AppPanel } from "./SandboxApp";
import { InternetPanel } from "./SandboxInternet";
import { ListenerSection } from "./SandboxConn";
import { CachePanel } from "./SandboxCache";
import { DbPanel } from "./SandboxDb";
import { QueuePanel, WorkerPanel } from "./SandboxQueue";
import { StoragePanel } from "./SandboxStorage";
import { StreamPanel } from "./SandboxStream";
import { EdgeNodePanel } from "./SandboxEdge";
import { TrafficPanel } from "./SandboxTraffic";
import { BackendPanel, TelemetrySection } from "./SandboxTelemetry";

export const KIND_DRAG_TYPE = "application/x-forgelab-kind";

export function SandboxPalette({
  rules,
  cash,
  goals,
  onPlace,
}: {
  rules: Ruleset;
  cash: number;
  goals: GoalStatus[];
  onPlace: (kind: string) => void;
}) {
  return (
    <aside className="sb-palette">
      <h3>Build</h3>
      <p className="sb-hint">Drag onto the canvas or click to place. Wire components by dragging between handles.</p>
      {rules.kinds
        .filter((k) => k.name !== "internet")
        .map((k) => {
          const locked = lockedBy(k, goals);
          const affordable = cash >= k.buildCost && !locked;
          return (
            <button
              key={k.name}
              className="sb-kind secondary"
              data-kind={k.name}
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
                {locked
                  ? `🔒 goal: ${locked.title}`
                  : k.name === "traffic"
                    ? "free · one population of users"
                    : `${formatMoney(k.buildCost)} + ${formatMoney(k.costPerHour)}/h`}
              </span>
            </button>
          );
        })}
    </aside>
  );
}

// blind is the game without node id's live numbers: what the player sees of
// a component nobody monitors (v14). Its bill is still known.
function blind(game: GameState, id: string): GameState {
  return {
    ...game,
    flow: {
      ...game.flow,
      nodes: game.flow.nodes.map((s) =>
        s.id === id ? { id, offered: 0, served: 0, dropped: 0, capacity: 0, utilization: 0, latencyMs: 0, costPerHour: s.costPerHour, obs: s.obs } : s,
      ),
      edges: game.flow.edges?.filter((e) => e.from !== id && e.to !== id),
    },
  };
}

export function SandboxInspector({
  game,
  rules,
  selected,
  onCommand,
  onConfigure,
}: {
  game: GameState;
  rules: Ruleset;
  selected: string | null;
  onCommand: (c: Command) => void;
  onConfigure: (c: Command) => Promise<string | null>;
}) {
  const node = game.nodes.find((n) => n.id === selected);
  if (!node) {
    return (
      <aside className="sb-inspector">
        <h3>Inspector</h3>
        <p className="sb-hint">
          Select a component to see its load and change it.{" "}
          {rules.client
            ? "A new world is empty: place Traffic for your users, connect it to an application instance, give that a database and object storage, and users start paying."
            : "A new world is empty: connect the Internet to an application instance, give it a database and object storage, and users start paying."}
        </p>
      </aside>
    );
  }
  const kind = rules.kinds.find((k) => k.name === node.kind);
  // From v14 a component's live numbers are seen only while monitored; its
  // panels then get a view of the game without them.
  const visible =
    node.kind === "traffic" ? !rules.telemetry || !!game.meters.monitored : seen(rules, game.flow.nodes.find((s) => s.id === node.id));
  const view = visible ? game : blind(game, node.id);
  const stats = view.flow.nodes.find((s) => s.id === node.id);
  const size = rules.sizes.find((s) => s.name === node.size);
  const internet = node.kind === "internet";
  // A traffic component is a source too: nothing to size, scale, or serve;
  // managed object storage (v10) has no sizes or replicas either.
  const source = internet || node.kind === "traffic";
  const managed = (node.kind === "object-storage" && !!rules.storage) || (node.kind === "cdn" && !!rules.lb);
  const replicaCost = (kind?.buildCost ?? 0) * (size?.costFactor ?? 1);
  const downstream = game.edges.filter((e) => e.from === node.id).map((e) => e.to);
  // Mirrors the engine's rule so the button is only offered when it can work;
  // the engine still validates the command.
  const senders = new Set(game.edges.filter((e) => e.to === node.id).map((e) => e.from));
  const hasReplica = game.nodes.some(
    (n) => n.kind === "db-replica" && !n.down && game.edges.some((e) => e.to === n.id && senders.has(e.from)),
  );
  const downBy = (effect: string) =>
    (game.events ?? []).some((e) => e.phase === "active" && e.effect === effect && e.hits?.some((h) => h.node === node.id));
  const crashed = downBy("crash");
  const zoned = downBy("zone");
  const respond = (action: "restart" | "failover" | "rate-limit" | "lift-rate-limit") =>
    onCommand({ type: "respond", action, node: node.id });

  return (
    <aside className="sb-inspector">
      <h3>{kind?.label ?? node.kind}</h3>
      <div className="meta">{node.id}</div>
      {stats && visible && node.kind !== "traffic" && (
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
            {!source && (
              <>
                <tr>
                  <th>Dropped</th>
                  <td className={`num ${stats.dropped > 0.01 ? "bad" : ""}`}>{formatCompact(stats.dropped)}/s</td>
                </tr>
                {!!stats.attack && (
                  <tr>
                    <th>Attack</th>
                    <td className="num bad">{formatCompact(stats.attack)}/s</td>
                  </tr>
                )}
                {!!stats.blocked && (
                  <tr>
                    <th>Blocked</th>
                    <td className="num">{formatCompact(stats.blocked)}/s</td>
                  </tr>
                )}
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

      {internet && <InternetPanel game={game} rules={rules} onConfigure={onConfigure} />}
      <ListenerSection key={node.id} rules={rules} node={node} onConfigure={onConfigure} />
      {!visible && (
        <p className="sb-hint" role="status">
          Not monitored: turn on its metrics below, with a metrics store in the system, to see its load, latency, and errors.
        </p>
      )}
      {(node.kind === "db-primary" || node.kind === "db-replica") && rules.db && <DbPanel game={view} rules={rules} node={node} onConfigure={onConfigure} />}
      {rules.lb && ["load-balancer", "api-gateway", "cdn"].includes(node.kind) && <EdgeNodePanel game={view} rules={rules} node={node} onConfigure={onConfigure} />}
      {node.kind === "event-stream" && rules.stream && <StreamPanel game={view} rules={rules} node={node} onConfigure={onConfigure} />}
      {node.kind === "queue" && rules.queue && <QueuePanel game={view} rules={rules} node={node} onConfigure={onConfigure} />}
      {node.kind === "worker" && rules.worker && <WorkerPanel game={view} rules={rules} node={node} onConfigure={onConfigure} />}
      {node.kind === "object-storage" && rules.storage && <StoragePanel game={view} rules={rules} node={node} onConfigure={onConfigure} />}
      {node.kind === "cache" && rules.cache && <CachePanel game={view} rules={rules} node={node} onConfigure={onConfigure} />}
      {node.kind === "traffic" && <TrafficPanel game={view} rules={rules} node={node} onConfigure={onConfigure} />}
      {node.kind === "app-instance" && <AppPanel game={view} rules={rules} node={node} stats={stats} onConfigure={onConfigure} />}
      {BACKENDS.includes(node.kind) && <BackendPanel game={game} node={node} />}
      {rules.telemetry && node.kind !== "traffic" && !BACKENDS.includes(node.kind) && (
        <TelemetrySection key={`telemetry:${node.id}`} game={game} rules={rules} node={node} onConfigure={onConfigure} />
      )}

      {(node.kind === "traffic" || managed) && (
        <div className="sb-actions">
          <button className="secondary danger" onClick={() => onCommand({ type: "remove", node: node.id })}>
            Remove
          </button>
        </div>
      )}
      {!source && !managed && (
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

      {!source && (crashed || zoned || node.kind === "db-primary" || node.kind === "api-gateway") && (
        <div className="sb-respond">
          <h4>Respond</h4>
          {crashed && (
            <button className="secondary" onClick={() => respond("restart")}>
              Restart ({node.replicas - (node.downReplicas ?? 0)}/{node.replicas} up)
            </button>
          )}
          {zoned && (
            <p className="sb-hint">
              Replicas lost with their zone come back when the zone does; a restart cannot help. More replicas spread the risk.
            </p>
          )}
          {node.kind === "db-primary" && (
            <button
              className="secondary"
              disabled={!hasReplica}
              onClick={() => respond("failover")}
              title={hasReplica ? "Promote the healthiest read replica" : "Needs a healthy read replica wired to the same senders"}
            >
              Fail over to a replica
            </button>
          )}
          {node.kind === "api-gateway" && (
            <button className="secondary" onClick={() => respond(node.rateLimited ? "lift-rate-limit" : "rate-limit")}>
              {node.rateLimited ? "Lift rate limit" : "Rate limit"}
            </button>
          )}
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
