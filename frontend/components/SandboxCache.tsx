"use client";

import { useEffect, useRef, useState } from "react";
import {
  formatCompact,
  hasBlankNumber,
  healthLevel,
  problems,
  type CacheConfig,
  type Command,
  type GameState,
  type Ruleset,
  type SandboxNode,
} from "@/lib/sandbox";
import { InnerView, Num, graph } from "./SandboxInternet";

const pct = (v: number) => `${(v * 100).toFixed(0)}%`;
const mb = (v: number) => (v >= 1024 ? `${(v / 1024).toFixed(1)} GB` : `${formatCompact(v)} MB`);

export function cacheConfig(game: GameState, rules: Ruleset, node: SandboxNode): CacheConfig | undefined {
  return node.cache ?? (game.ruleset === rules.version ? rules.cache : undefined);
}

// CachePanel is a cache's part of the inspector (v9).
export function CachePanel({
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
  const cfg = cacheConfig(game, rules, node);
  const c = game.flow.nodes.find((s) => s.id === node.id)?.cache;
  if (!cfg) return null;
  const rows: [string, string, string?][] = c
    ? [
        ["Health", c.health, healthLevel(c.health)],
        ["Hit ratio", `${pct(c.hitRatio)} = fits ${pct(c.fits)} · fresh ${pct(c.fresh)}${c.warmth < 1 ? ` · warm ${pct(c.warmth)}` : ""}`, c.hitRatio < 0.5 ? "warn" : undefined],
        ["Hits / misses", `${formatCompact(c.hits)}/s · ${formatCompact(c.misses)}/s`],
        ["Memory", `${mb(c.memoryUsedMb)} / ${mb(c.memoryTotalMb)} · ${formatCompact(c.keys)} keys`],
        ...(c.evictions > 0.01 ? ([["Evictions", `${formatCompact(c.evictions)}/s`, "warn"]] as [string, string, string][]) : []),
        ["Bottleneck", c.bottleneck === "cpu" ? "CPU" : "network"],
        ["CPU", `${c.cpuUsed.toFixed(3)} / ${c.cpuTotal} vCPU`],
        ["Network", `${formatCompact(c.networkMbps)} / ${formatCompact(c.networkTotalMbps)} Mbps`],
        ["Connections", `${Math.round(c.connections)} / ${c.maxConnections}`, c.refused > 0.01 ? "bad" : undefined],
        ["Latency", `${c.latencyMs.toFixed(2)} ms`],
      ]
    : [];
  return (
    <div className="sb-internet">
      <div className="meta">
        {cfg.engine} · {cfg.eviction.toUpperCase()} · TTL {formatCompact(cfg.ttlSeconds)} s · {cfg.valueKb} KB values
      </div>
      {c && (
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
      <button onClick={() => setOpen(true)}>Configure cache</button>
      {open && <CacheDialog config={cfg} onApply={(cache) => onConfigure({ type: "configure", node: node.id, cache })} onClose={() => setOpen(false)} />}
    </div>
  );
}

function CacheDialog({ config, onApply, onClose }: { config: CacheConfig; onApply: (c: CacheConfig) => Promise<string | null>; onClose: () => void }) {
  const dialog = useRef<HTMLDialogElement>(null);
  const [c, setC] = useState<CacheConfig>(() => ({ ...config }));
  const [errors, setErrors] = useState<string[]>([]);
  useEffect(() => {
    const d = dialog.current;
    d?.showModal();
    return () => d?.close();
  }, []);
  const set = (p: Partial<CacheConfig>) => setC((x) => ({ ...x, ...p }));
  const apply = async () => {
    if (hasBlankNumber(c)) return setErrors(["Fill in every number field."]);
    const err = await onApply(c);
    if (err) setErrors(problems(err));
    else onClose();
  };
  return (
    <dialog ref={dialog} className="sb-dialog" aria-label="Cache" onCancel={onClose}>
      <h3>Cache</h3>
      <p className="sb-hint">
        Simulated: 90% of the size&apos;s memory holds values. The hit ratio is the share of the working set that fits (skewed by the
        eviction policy) times the chance a hot key is read again before it expires. A new or restarted cache starts empty.
      </p>
      <div className="sb-row">
        <label className="sb-field">
          <span>Engine</span>
          <select aria-label="Engine" value={c.engine} onChange={(e) => set({ engine: e.target.value })}>
            <option>Redis</option>
            <option>Memcached</option>
          </select>
        </label>
        <label className="sb-field">
          <span>Eviction</span>
          <select aria-label="Eviction" value={c.eviction} onChange={(e) => set({ eviction: e.target.value as CacheConfig["eviction"] })}>
            <option value="lru">LRU: evict the least recently used</option>
            <option value="lfu">LFU: keep the most frequently used</option>
            <option value="none">none: stop admitting once full</option>
          </select>
        </label>
      </div>
      <div className="sb-row">
        <Num label="TTL (s)" value={c.ttlSeconds} onChange={(ttlSeconds) => set({ ttlSeconds })} />
        <Num label="Value size (KB)" value={c.valueKb} step={0.1} onChange={(valueKb) => set({ valueKb })} />
        <Num label="Max connections" value={c.maxConnections} onChange={(maxConnections) => set({ maxConnections })} />
      </div>
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

// CacheView opens a cache: reads split into hits and misses, and why.
export function CacheView({ game, node, onBack }: { game: GameState; node: SandboxNode; onBack: () => void }) {
  const c = game.flow.nodes.find((s) => s.id === node.id)?.cache;
  if (!c) return null;
  const n = (v = 0) => formatCompact(v);
  const g = graph(c.hits + c.misses > 0);
  const behind = game.edges.filter((e) => e.from === node.id).map((e) => e.to);
  g.column([{ id: "in", title: "Reads", sub: `${n(c.hits + c.misses)}/s · ${Math.round(c.connections)} / ${c.maxConnections} connections`, cls: c.refused > 0.01 ? "bad" : undefined }], "input");
  g.column([{ id: "mem", title: "Memory", sub: `${mb(c.memoryUsedMb)} / ${mb(c.memoryTotalMb)} · fits ${pct(c.fits)} of the working set${c.warmth < 1 ? ` · warm ${pct(c.warmth)}` : ""}`, cls: c.fits < 1 ? "warn" : undefined }]);
  g.column([
    { id: "hit", title: "Hits", sub: `${n(c.hits)}/s · ${c.latencyMs.toFixed(2)} ms` },
    { id: "miss", title: "Misses", sub: `${n(c.misses)}/s · fresh ${pct(c.fresh)}${c.evictions > 0.01 ? ` · evicting ${n(c.evictions)}/s` : ""}`, cls: c.hitRatio < 0.5 ? "warn" : undefined },
  ]);
  g.column([{ id: "db", title: behind.length ? `→ ${behind.join(", ")}` : "→ no database", sub: "misses read through" }], "output");
  g.link("in", "mem", 1);
  g.link("mem", "hit", c.hitRatio);
  g.link("mem", "miss", 1 - c.hitRatio);
  g.link("miss", "db", 1 - c.hitRatio);
  return (
    <InnerView
      title={`Inside ${node.id}`}
      hint={
        <>
          <span className={healthLevel(c.health)}>{c.health}</span> · hit ratio {pct(c.hitRatio)} · bottleneck {c.bottleneck === "cpu" ? "CPU" : "network"}
        </>
      }
      graph={g}
      onBack={onBack}
    />
  );
}
