"use client";

import { useEffect, useRef, useState } from "react";
import {
  formatCompact,
  formatMoney,
  hasBlankNumber,
  healthLevel,
  problems,
  type Command,
  type GameState,
  type Ruleset,
  type SandboxNode,
  type StorageConfig,
} from "@/lib/sandbox";
import { InnerView, Num, graph } from "./SandboxInternet";

export function storageConfig(game: GameState, rules: Ruleset, node: SandboxNode): StorageConfig | undefined {
  return node.storage ?? (game.ruleset === rules.version ? rules.storage : undefined);
}

// StoragePanel is object storage's part of the inspector (v10): requests
// against the prefixes' rate, latency by part, and the bill by part.
export function StoragePanel({
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
  const cfg = storageConfig(game, rules, node);
  const s = game.flow.nodes.find((x) => x.id === node.id)?.storage;
  if (!cfg) return null;
  const label = rules.storageRuntime?.classes.find((c) => c.name === cfg.class)?.label ?? cfg.class;
  const rows: [string, string, string?][] = s
    ? [
        ["Health", s.health, healthLevel(s.health)],
        ["Requests", `${formatCompact(s.gets)}/s of ${formatCompact(s.capacity)}/s`, s.throttled > 0.01 ? "bad" : undefined],
        ...(s.throttled > 0.01 ? ([["Throttled", `${formatCompact(s.throttled)}/s`, "bad"]] as [string, string, string][]) : []),
        ["Latency", `${s.latencyMs.toFixed(0)} ms = first byte ${s.firstByteMs.toFixed(0)} + transfer ${s.transferMs.toFixed(0)} (+ queue)`],
        ["Stored", `${formatCompact(s.storedGb)} GB`],
        ["Egress", `${formatCompact(s.egressMbps)} Mbps`],
        ["Cost", `${formatMoney(s.storageCost + s.requestCost + s.retrievalCost + s.egressCost)}/h`],
        ["… stored", `${formatMoney(s.storageCost)}/h`],
        ["… requests", `${formatMoney(s.requestCost)}/h`],
        ...(s.retrievalCost > 0 ? ([["… retrieval", `${formatMoney(s.retrievalCost)}/h`]] as [string, string][]) : []),
        ["… egress", `${formatMoney(s.egressCost)}/h`, s.egressCost > 1 ? "warn" : undefined],
      ]
    : [];
  return (
    <div className="sb-internet">
      <div className="meta">
        {label} · {cfg.prefixes} {cfg.prefixes === 1 ? "prefix" : "prefixes"} · {formatCompact(cfg.objectKb)} KB objects · managed: no sizes or replicas
      </div>
      {s && (
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
      <button onClick={() => setOpen(true)}>Configure storage</button>
      {open && (
        <StorageDialog rules={rules} config={cfg} onApply={(storage) => onConfigure({ type: "configure", node: node.id, storage })} onClose={() => setOpen(false)} />
      )}
    </div>
  );
}

function StorageDialog({
  rules,
  config,
  onApply,
  onClose,
}: {
  rules: Ruleset;
  config: StorageConfig;
  onApply: (c: StorageConfig) => Promise<string | null>;
  onClose: () => void;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  const [c, setC] = useState<StorageConfig>(() => ({ ...config }));
  const [errors, setErrors] = useState<string[]>([]);
  useEffect(() => {
    const d = dialog.current;
    d?.showModal();
    return () => d?.close();
  }, []);
  const set = (p: Partial<StorageConfig>) => setC((x) => ({ ...x, ...p }));
  const apply = async () => {
    if (hasBlankNumber(c)) return setErrors(["Fill in every number field."]);
    const err = await onApply(c);
    if (err) setErrors(problems(err));
    else onClose();
  };
  return (
    <dialog ref={dialog} className="sb-dialog" aria-label="Object storage" onCancel={onClose}>
      <h3>Object storage</h3>
      <p className="sb-hint">
        Simulated managed storage: each prefix sustains {formatCompact(rules.storageRuntime?.getsPerPrefix ?? 0)} GETs per second, a read
        costs the class&apos;s first byte plus the transfer, and you pay for what is stored, requested, retrieved, and sent out.
      </p>
      <div className="sb-row">
        <label className="sb-field">
          <span>Class</span>
          <select aria-label="Class" value={c.class} onChange={(e) => set({ class: e.target.value })}>
            {(rules.storageRuntime?.classes ?? []).map((x) => (
              <option key={x.name} value={x.name}>
                {x.label}: {x.firstByteMs} ms first byte, ${x.gbMonth}/GB-month
              </option>
            ))}
          </select>
        </label>
        <Num label="Prefixes" value={c.prefixes} onChange={(prefixes) => set({ prefixes })} />
        <Num label="Object size (KB)" value={c.objectKb} onChange={(objectKb) => set({ objectKb })} />
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

// StorageView opens object storage: GETs through the prefixes to the first
// byte and the transfer, and what they cost.
export function StorageView({ game, node, onBack }: { game: GameState; node: SandboxNode; onBack: () => void }) {
  const s = game.flow.nodes.find((x) => x.id === node.id)?.storage;
  if (!s) return null;
  const n = (v = 0) => formatCompact(v);
  const g = graph(s.gets > 0);
  g.column([{ id: "in", title: "GETs", sub: `${n(s.gets)}/s` }], "input");
  g.column([
    { id: "prefix", title: "Prefixes", sub: `${n(s.capacity)}/s sustained`, cls: s.throttled > 0.01 ? "bad" : undefined },
    ...(s.throttled > 0.01 ? [{ id: "throttle", title: "Throttled", sub: `${n(s.throttled)}/s · SlowDown`, cls: "bad" }] : []),
  ]);
  g.column([{ id: "fb", title: "First byte", sub: `${s.firstByteMs.toFixed(0)} ms` }]);
  g.column([{ id: "tx", title: "Transfer", sub: `${s.transferMs.toFixed(0)} ms · ${n(s.egressMbps)} Mbps out` }]);
  g.column([{ id: "out", title: "Bill", sub: `${formatMoney(s.storageCost + s.requestCost + s.retrievalCost + s.egressCost)}/h · egress ${formatMoney(s.egressCost)}/h` }], "output");
  const ok = s.gets > 0 ? (s.gets - s.throttled) / s.gets : 1;
  g.link("in", "prefix", ok);
  if (s.throttled > 0.01) g.link("in", "throttle", 1 - ok);
  g.link("prefix", "fb", ok);
  g.link("fb", "tx", ok);
  g.link("tx", "out", ok);
  return (
    <InnerView
      title={`Inside ${node.id}`}
      hint={
        <>
          <span className={healthLevel(s.health)}>{s.health}</span> · {s.class} · latency {s.latencyMs.toFixed(0)} ms · {n(s.storedGb)} GB stored
        </>
      }
      graph={g}
      onBack={onBack}
    />
  );
}
