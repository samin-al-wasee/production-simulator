"use client";

import { useEffect, useRef, useState } from "react";
import {
  formatCompact,
  hasBlankNumber,
  healthLevel,
  problems,
  type Command,
  type DBConfig,
  type GameState,
  type QueryProfile,
  type Ruleset,
  type SandboxNode,
} from "@/lib/sandbox";
import { InnerView, Num, graph } from "./SandboxInternet";

const BOTTLENECK: Record<string, string> = { cpu: "CPU", iops: "disk IOPS", locks: "row locks" };

// dbConfig is a database's configuration: its own once set, else the
// ruleset's, under the same version rule as the other components.
export function dbConfig(game: GameState, rules: Ruleset, node: SandboxNode): DBConfig | undefined {
  return node.db ?? (game.ruleset === rules.version ? rules.db : undefined);
}

const mb = (v: number) => (v >= 1024 ? `${(v / 1024).toFixed(1)} GB` : `${formatCompact(v)} MB`);

// DbPanel is a database's part of the inspector (v8): every value from the
// engine, and the way into its configuration.
export function DbPanel({
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
  const [open, setOpen] = useState(false);
  const cfg = dbConfig(game, rules, node);
  const d = game.flow.nodes.find((s) => s.id === node.id)?.db;
  if (!cfg) return null;
  const rows: [string, string, string?][] = d
    ? [
        ["Health", d.health, healthLevel(d.health)],
        ["Bottleneck", BOTTLENECK[d.bottleneck] ?? d.bottleneck],
        ["CPU", `${d.cpuUsed.toFixed(2)} / ${d.cpuTotal} vCPU`],
        ["Disk", `${formatCompact(d.iopsUsed)} / ${formatCompact(d.iopsTotal)} IOPS`, d.iopsUsed > 0.85 * d.iopsTotal ? "warn" : undefined],
        ["Buffer hit ratio", `${(d.hitRatio * 100).toFixed(0)}%`, d.hitRatio < 0.9 ? "warn" : undefined],
        ["Data", `${mb(d.dataMb)} · working set ${mb(d.workingSetMb)} · buffer pool ${mb(d.bufferPoolMb)}`],
        ["Connections", `${Math.round(d.connections)} / ${d.maxConnections}`, d.refused > 0.01 ? "bad" : undefined],
        ...(d.refused > 0.01 ? ([["Refused", `${formatCompact(d.refused)}/s`, "bad"]] as [string, string, string][]) : []),
        ["Reads", `${formatCompact(d.reads)}/s · ${d.readMs.toFixed(1)} ms`],
        ["Writes", `${formatCompact(d.writes)}/s · ${d.writeMs.toFixed(1)} ms`],
        ["Queue wait", `${d.waitMs.toFixed(1)} ms`, d.waitMs > 10 ? "warn" : undefined],
        ...(d.lockWaitMs > 0.05 ? ([["Lock wait", `${d.lockWaitMs.toFixed(1)} ms`, "warn"]] as [string, string, string][]) : []),
        ...(d.applying !== undefined
          ? ([["Replication", `${formatCompact(d.applying)} writes/s applied · ${(d.lagSeconds ?? 0).toFixed(1)} s behind`, (d.lagSeconds ?? 0) > 10 ? "bad" : undefined]] as [string, string, string?][])
          : []),
      ]
    : [];
  return (
    <div className="sb-internet">
      <div className="meta">
        {cfg.engine} · max {cfg.maxConnections} connections · reads {cfg.read.indexed ? "indexed" : "unindexed"}, writes{" "}
        {cfg.write.indexed ? "indexed" : "unindexed"}
      </div>
      {d && (
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
      <button onClick={() => setOpen(true)}>Configure database</button>
      {open && <DbDialog config={cfg} onApply={(db) => onConfigure({ type: "configure", node: node.id, db })} onClose={() => setOpen(false)} />}
    </div>
  );
}

function Query({ label, q, onChange }: { label: string; q: QueryProfile; onChange: (q: QueryProfile) => void }) {
  return (
    <div className="sb-row">
      <Num label={`${label} CPU (ms)`} value={q.cpuMs} step={0.1} onChange={(cpuMs) => onChange({ ...q, cpuMs })} />
      <Num label={`${label} pages`} value={q.pages} onChange={(pages) => onChange({ ...q, pages })} />
      <label>
        <input type="checkbox" checked={q.indexed} onChange={(e) => onChange({ ...q, indexed: e.target.checked })} /> {label} uses an index
      </label>
    </div>
  );
}

function DbDialog({ config, onApply, onClose }: { config: DBConfig; onApply: (c: DBConfig) => Promise<string | null>; onClose: () => void }) {
  const dialog = useRef<HTMLDialogElement>(null);
  const [c, setC] = useState<DBConfig>(() => structuredClone(config));
  const [errors, setErrors] = useState<string[]>([]);
  useEffect(() => {
    const d = dialog.current;
    d?.showModal();
    return () => d?.close();
  }, []);
  const set = (p: Partial<DBConfig>) => setC((x) => ({ ...x, ...p }));
  const apply = async () => {
    if (hasBlankNumber(c)) return setErrors(["Fill in every number field."]);
    const err = await onApply(c);
    if (err) setErrors(problems(err));
    else onClose();
  };
  return (
    <dialog ref={dialog} className="sb-dialog" aria-label="Database" onCancel={onClose}>
      <h3>Database</h3>
      <p className="sb-hint">
        Simulated: CPU, memory, and disk IOPS come from the size; 75% of memory caches pages. Data grows with your users, so a query
        that fits in memory today reads from disk tomorrow.
      </p>
      <div className="sb-row">
        <label className="sb-field">
          <span>Engine</span>
          <select aria-label="Engine" value={c.engine} onChange={(e) => set({ engine: e.target.value })}>
            <option>PostgreSQL</option>
            <option>MySQL</option>
          </select>
        </label>
        <Num label="Max connections" value={c.maxConnections} onChange={(maxConnections) => set({ maxConnections })} />
      </div>
      <p className="sb-hint">Every caller replica opens its connection&apos;s pool; above the maximum, that share of queries is refused.</p>
      <details className="sb-section" open>
        <summary>Queries</summary>
        <Query label="Read" q={c.read} onChange={(read) => set({ read })} />
        <Query label="Write" q={c.write} onChange={(write) => set({ write })} />
        <p className="sb-hint">
          An indexed query touches its pages; without an index it scans a page per MB of data as well. A write also logs (one disk
          write) and writes its pages back.
        </p>
      </details>
      <details className="sb-section" open>
        <summary>Locks</summary>
        <div className="sb-row">
          <Num label="Hot rows" value={c.hotRows} onChange={(hotRows) => set({ hotRows })} />
          <Num label="Lock time (ms)" value={c.lockMs} step={0.1} onChange={(lockMs) => set({ lockMs })} />
        </div>
        <p className="sb-hint">Writes contend for the hot rows: at most hot rows × 1000 ÷ lock time writes per second on a primary.</p>
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
        <button onClick={apply}>Apply</button>
      </div>
    </dialog>
  );
}

// DbView opens a database: a query's path through its connections, queue,
// CPU, buffer cache, disk, and locks, with the engine's values.
export function DbView({ game, node, onBack }: { game: GameState; node: SandboxNode; onBack: () => void }) {
  const d = game.flow.nodes.find((s) => s.id === node.id)?.db;
  if (!d) return null;
  const n = (v = 0) => formatCompact(v);
  const total = d.reads + d.writes;
  const at = (b: string) => (d.bottleneck === b && total > 0.85 * d.capacity ? "bad" : undefined);
  const g = graph(total > 0);
  const replica = d.applying !== undefined;
  g.column([
    {
      id: "conn",
      title: "Connections",
      sub: `${Math.round(d.connections)} / ${d.maxConnections} open${d.refused > 0.01 ? ` · ${n(d.refused)}/s refused` : ""}`,
      cls: d.refused > 0.01 ? "bad" : undefined,
    },
    ...(replica ? [{ id: "repl", title: "Replication", sub: `${n(d.applying)} writes/s · ${(d.lagSeconds ?? 0).toFixed(1)} s behind`, cls: (d.lagSeconds ?? 0) > 10 ? "bad" : undefined }] : []),
  ], "input");
  g.column([{ id: "queue", title: "Queue", sub: `waits ${d.waitMs.toFixed(1)} ms`, cls: d.waitMs > 10 ? "warn" : undefined }]);
  g.column([{ id: "cpu", title: "CPU", sub: `${d.cpuUsed.toFixed(2)} / ${d.cpuTotal} vCPU`, cls: at("cpu") }]);
  g.column([{ id: "cache", title: "Buffer cache", sub: `${(d.hitRatio * 100).toFixed(0)}% hits · ${mb(d.bufferPoolMb)} for ${mb(d.workingSetMb)}`, cls: d.hitRatio < 0.9 ? "warn" : undefined }]);
  g.column([
    { id: "disk", title: "Disk", sub: `${n(d.iopsUsed)} / ${n(d.iopsTotal)} IOPS`, cls: at("iops") },
    ...(d.writes > 0 ? [{ id: "locks", title: "Row locks", sub: `writes wait ${d.lockWaitMs.toFixed(1)} ms`, cls: at("locks") }] : []),
  ]);
  g.column([{ id: "out", title: "Response", sub: `reads ${d.readMs.toFixed(1)} ms · writes ${d.writeMs.toFixed(1)} ms` }], "output");
  const rs = total > 0 ? d.reads / total : 1;
  g.link("conn", "queue", 1);
  if (replica) g.link("repl", "cpu", 0.3);
  g.link("queue", "cpu", 1);
  g.link("cpu", "cache", 1);
  g.link("cache", "disk", Math.max(0.05, 1 - d.hitRatio));
  g.link("cache", "out", rs * d.hitRatio);
  if (d.writes > 0) {
    g.link("cpu", "locks", 1 - rs);
    g.link("locks", "out", 1 - rs);
  }
  g.link("disk", "out", Math.max(0.05, 1 - d.hitRatio));
  return (
    <InnerView
      title={`Inside ${node.id}`}
      hint={
        <>
          <span className={healthLevel(d.health)}>{d.health}</span> · bottleneck {BOTTLENECK[d.bottleneck] ?? d.bottleneck} · capacity {n(d.capacity)}/s · {n(d.reads)} reads/s,{" "}
          {n(d.writes)} writes/s
        </>
      }
      graph={g}
      onBack={onBack}
    />
  );
}
