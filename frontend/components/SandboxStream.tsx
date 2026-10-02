"use client";

import { useEffect, useRef, useState } from "react";
import {
  formatCompact,
  hasBlankNumber,
  healthLevel,
  problems,
  toPercent,
  type Command,
  type GameState,
  type Ruleset,
  type SandboxNode,
  type StreamConfig,
} from "@/lib/sandbox";
import { InnerView, Num, graph } from "./SandboxInternet";

const lagText = (s: number) => (s >= 3600 ? `${(s / 3600).toFixed(1)} h` : s >= 60 ? `${(s / 60).toFixed(1)} min` : `${s.toFixed(1)} s`);

export function streamConfig(game: GameState, rules: Ruleset, node: SandboxNode): StreamConfig | undefined {
  return node.stream ?? (game.ruleset === rules.version ? rules.stream : undefined);
}

// StreamPanel is an event stream's part of the inspector (v12): what it
// takes, and how far behind each consumer group is.
export function StreamPanel({
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
  const cfg = streamConfig(game, rules, node);
  const s = game.flow.nodes.find((x) => x.id === node.id)?.stream;
  if (!cfg) return null;
  return (
    <div className="sb-internet">
      <div className="meta">
        {cfg.partitions} partitions · {cfg.retentionHours} h retention · {cfg.messageKb} KB events{cfg.keySkew > 0 ? ` · hottest key ${toPercent(cfg.keySkew)}%` : ""}
      </div>
      {s && (
        <table>
          <tbody>
            <tr>
              <th>Health</th>
              <td className={`num ${healthLevel(s.health)}`}>{s.health}</td>
            </tr>
            <tr>
              <th>Produced</th>
              <td className="num">
                {formatCompact(s.produced)}/s of {formatCompact(s.capacity)}/s ({s.bottleneck})
              </td>
            </tr>
            {s.throttled > 0.01 && (
              <tr>
                <th>Throttled</th>
                <td className="num bad">{formatCompact(s.throttled)}/s</td>
              </tr>
            )}
            <tr>
              <th>Stored</th>
              <td className="num">{formatCompact(s.storedMb)} MB</td>
            </tr>
          </tbody>
        </table>
      )}
      {!!s?.groups?.length && (
        <>
          <h4>Consumer groups</h4>
          <table className="sb-breakdown">
            <tbody>
              {s.groups.map((gs) => (
                <tr key={gs.consumer} title={`${gs.active} of ${gs.members} members active`}>
                  <th>{gs.consumer}</th>
                  <td className="num">{formatCompact(gs.consumed)}/s</td>
                  <td className={`num ${gs.lost > 0.01 ? "bad" : gs.lagSeconds > 60 ? "warn" : ""}`}>
                    {gs.lost > 0.01 ? `losing ${formatCompact(gs.lost)}/s` : `lag ${lagText(gs.lagSeconds)}`}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </>
      )}
      <button onClick={() => setOpen(true)}>Configure stream</button>
      {open && <StreamDialog config={cfg} onApply={(stream) => onConfigure({ type: "configure", node: node.id, stream })} onClose={() => setOpen(false)} />}
    </div>
  );
}

function StreamDialog({ config, onApply, onClose }: { config: StreamConfig; onApply: (c: StreamConfig) => Promise<string | null>; onClose: () => void }) {
  const dialog = useRef<HTMLDialogElement>(null);
  const [c, setC] = useState<StreamConfig>(() => ({ ...config }));
  const [errors, setErrors] = useState<string[]>([]);
  useEffect(() => {
    const d = dialog.current;
    d?.showModal();
    return () => d?.close();
  }, []);
  const set = (p: Partial<StreamConfig>) => setC((x) => ({ ...x, ...p }));
  const apply = async () => {
    if (hasBlankNumber(c)) return setErrors(["Fill in every number field."]);
    const err = await onApply(c);
    if (err) setErrors(problems(err));
    else onClose();
  };
  return (
    <dialog ref={dialog} className="sb-dialog" aria-label="Event stream" onCancel={onClose}>
      <h3>Event stream</h3>
      <p className="sb-hint">
        Simulated log (one topic): each partition takes 10 MB/s, and the hottest key decides how evenly they fill. Every component
        connected from the stream is a consumer group and reads every event; a group uses at most one member per partition. Events
        older than the retention are deleted, read or not.
      </p>
      <div className="sb-row">
        <Num label="Partitions" value={c.partitions} onChange={(partitions) => set({ partitions })} />
        <Num label="Retention (hours)" value={c.retentionHours} onChange={(retentionHours) => set({ retentionHours })} />
        <Num label="Event size (KB)" value={c.messageKb} step={0.1} onChange={(messageKb) => set({ messageKb })} />
        <Num label="Hottest key %" value={toPercent(c.keySkew)} step={0.1} max={100} onChange={(v) => set({ keySkew: v / 100 })} />
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

// StreamView opens an event stream: producers into partitions, and every
// consumer group reading all of it at its own pace.
export function StreamView({ game, node, onBack }: { game: GameState; node: SandboxNode; onBack: () => void }) {
  const s = game.flow.nodes.find((x) => x.id === node.id)?.stream;
  if (!s) return null;
  const n = (v = 0) => formatCompact(v);
  const groups = s.groups ?? [];
  const g = graph(s.produced > 0);
  g.column([{ id: "prod", title: "Producers", sub: `${n(s.produced)}/s${s.throttled > 0.01 ? ` · ${n(s.throttled)}/s throttled` : ""}`, cls: s.throttled > 0.01 ? "bad" : undefined }], "input");
  g.column([{ id: "log", title: "Partitions", sub: `${n(s.capacity)}/s by ${s.bottleneck} · hottest takes ${Math.round(s.hotShare * 100)}%` }]);
  g.column(
    groups.map((gs) => ({
      id: `g:${gs.consumer}`,
      title: gs.consumer,
      sub: `${n(gs.consumed)}/s · ${gs.active}/${gs.members} active · ${gs.lost > 0.01 ? `losing ${n(gs.lost)}/s` : `lag ${lagText(gs.lagSeconds)}`}`,
      cls: gs.lost > 0.01 ? "bad" : gs.lagSeconds > 60 ? "warn" : undefined,
    })),
    "output",
  );
  g.link("prod", "log", 1);
  // Every group reads the whole log: each edge carries all of it.
  for (const gs of groups) g.link("log", `g:${gs.consumer}`, 1, false);
  return (
    <InnerView
      title={`Inside ${node.id}`}
      hint={
        <>
          <span className={healthLevel(s.health)}>{s.health}</span> · every consumer group reads every event · {n(s.storedMb)} MB kept
        </>
      }
      graph={g}
      onBack={onBack}
    />
  );
}
