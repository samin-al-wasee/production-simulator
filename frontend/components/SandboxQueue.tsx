"use client";

import { useEffect, useRef, useState } from "react";
import {
  DEPS,
  formatCompact,
  hasBlankNumber,
  healthLevel,
  problems,
  toPercent,
  type Command,
  type GameState,
  type QueueConfig,
  type Ruleset,
  type SandboxNode,
  type WorkerConfig,
} from "@/lib/sandbox";
import { Check, bottleneckLabel } from "./SandboxApp";
import { InnerView, Num, graph } from "./SandboxInternet";

const pct = (v: number) => `${Math.round(v * 100)}%`;

export function queueConfig(game: GameState, rules: Ruleset, node: SandboxNode): QueueConfig | undefined {
  return node.queue ?? (game.ruleset === rules.version ? rules.queue : undefined);
}

export function workerConfig(game: GameState, rules: Ruleset, node: SandboxNode): WorkerConfig | undefined {
  return node.worker ?? (game.ruleset === rules.version ? rules.worker : undefined);
}

function useDialog() {
  const ref = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    const d = ref.current;
    d?.showModal();
    return () => d?.close();
  }, []);
  return ref;
}

function Errors({ errors }: { errors: string[] }) {
  if (errors.length === 0) return null;
  return (
    <ul className="sb-errors" role="alert">
      {errors.map((e) => (
        <li key={e}>{e}</li>
      ))}
    </ul>
  );
}

function Table({ rows }: { rows: [string, string, string?][] }) {
  return (
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
  );
}

// QueuePanel is a message queue's part of the inspector (v11).
export function QueuePanel({
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
  const cfg = queueConfig(game, rules, node);
  const q = game.flow.nodes.find((s) => s.id === node.id)?.queue;
  if (!cfg) return null;
  return (
    <div className="sb-internet">
      <div className="meta">
        {cfg.engine} · visibility {cfg.visibilitySeconds} s · up to {cfg.maxDeliveries} deliveries · holds {formatCompact(cfg.maxBacklog)}
      </div>
      {q && (
        <Table
          rows={[
            ["Health", q.health, healthLevel(q.health)],
            ["Published", `${formatCompact(q.published)}/s`],
            ...(q.rejected > 0.01 ? ([["Rejected", `${formatCompact(q.rejected)}/s (full)`, "bad"]] as [string, string, string][]) : []),
            ["Delivered", `${formatCompact(q.delivered)}/s`],
            ["Redelivered", `${formatCompact(q.redelivered)}/s · workers fail ${pct(q.workerFailure)}`, q.redelivered > 0.01 ? "warn" : undefined],
            ["Dead letters", `${formatCompact(q.deadLettered)}/s · ${formatCompact(q.deadLetters)} kept`, q.deadLettered > 0.01 ? "bad" : undefined],
            ["Backlog", `${formatCompact(q.backlog)} / ${formatCompact(q.maxBacklog)}`, q.backlog > 0.85 * q.maxBacklog ? "bad" : q.backlog > 1 ? "warn" : undefined],
            ["Delay", `${q.delaySeconds.toFixed(1)} s`, q.delaySeconds > 60 ? "warn" : undefined],
          ]}
        />
      )}
      <button onClick={() => setOpen(true)}>Configure queue</button>
      {open && <QueueDialog config={cfg} onApply={(queue) => onConfigure({ type: "configure", node: node.id, queue })} onClose={() => setOpen(false)} />}
    </div>
  );
}

function QueueDialog({ config, onApply, onClose }: { config: QueueConfig; onApply: (c: QueueConfig) => Promise<string | null>; onClose: () => void }) {
  const dialog = useDialog();
  const [c, setC] = useState<QueueConfig>(() => ({ ...config }));
  const [errors, setErrors] = useState<string[]>([]);
  const set = (p: Partial<QueueConfig>) => setC((x) => ({ ...x, ...p }));
  const apply = async () => {
    if (hasBlankNumber(c)) return setErrors(["Fill in every number field."]);
    const err = await onApply(c);
    if (err) setErrors(problems(err));
    else onClose();
  };
  return (
    <dialog ref={dialog} className="sb-dialog" aria-label="Message queue" onCancel={onClose}>
      <h3>Message queue</h3>
      <p className="sb-hint">
        Simulated at-least-once delivery: a message a worker fails, or holds past the visibility timeout, is delivered again, up to
        the maximum; then it goes to the dead-letter queue. Publishers only need the queue to accept the message.
      </p>
      <div className="sb-row">
        <label className="sb-field">
          <span>Engine</span>
          <select aria-label="Engine" value={c.engine} onChange={(e) => set({ engine: e.target.value })}>
            <option>RabbitMQ</option>
            <option>SQS</option>
          </select>
        </label>
        <Num label="Max backlog (per replica)" value={c.maxBacklog} onChange={(maxBacklog) => set({ maxBacklog })} />
        <Num label="Visibility timeout (s)" value={c.visibilitySeconds} onChange={(visibilitySeconds) => set({ visibilitySeconds })} />
        <Num label="Max deliveries" value={c.maxDeliveries} onChange={(maxDeliveries) => set({ maxDeliveries })} />
      </div>
      <Errors errors={errors} />
      <div className="sb-dialog-actions">
        <button className="secondary" onClick={onClose}>
          Cancel
        </button>
        <button onClick={apply}>Apply</button>
      </div>
    </dialog>
  );
}

// WorkerPanel is a background worker's part of the inspector (v11): it runs
// its handler like an application's route.
export function WorkerPanel({
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
  const cfg = workerConfig(game, rules, node);
  const a = game.flow.nodes.find((s) => s.id === node.id)?.app;
  if (!cfg) return null;
  const h = cfg.handler;
  return (
    <div className="sb-internet">
      <div className="meta">
        {cfg.concurrency} at a time per replica · handler {h.baseMs} ms, {h.cpuMs} CPU-ms{h.deps?.length ? ` · ${h.deps.join(", ")}` : ""}
      </div>
      {a && (
        <Table
          rows={[
            ["Health", a.health, healthLevel(a.health)],
            ["Bottleneck", bottleneckLabel(a.bottleneck)],
            ["Capacity", `${formatCompact(a.capacity)} messages/s`],
            ["CPU", `${a.cpuUsed.toFixed(2)} / ${a.cpuTotal} vCPU`],
            ["In flight", formatCompact(a.active)],
            ["Acknowledged", `${formatCompact(a.success)}/s`],
            ["Failed", `${formatCompact(a.errors + a.timeouts)}/s`, a.errors + a.timeouts > 0.01 ? "bad" : undefined],
          ]}
        />
      )}
      <button onClick={() => setOpen(true)}>Configure worker</button>
      {open && <WorkerDialog config={cfg} onApply={(worker) => onConfigure({ type: "configure", node: node.id, worker })} onClose={() => setOpen(false)} />}
    </div>
  );
}

function WorkerDialog({ config, onApply, onClose }: { config: WorkerConfig; onApply: (c: WorkerConfig) => Promise<string | null>; onClose: () => void }) {
  const dialog = useDialog();
  const [c, setC] = useState<WorkerConfig>(() => structuredClone(config));
  const [errors, setErrors] = useState<string[]>([]);
  const h = c.handler;
  const setH = (p: Partial<typeof h>) => setC((x) => ({ ...x, handler: { ...x.handler, ...p } }));
  const apply = async () => {
    if (hasBlankNumber(c)) return setErrors(["Fill in every number field."]);
    const err = await onApply(c);
    if (err) setErrors(problems(err));
    else onClose();
  };
  return (
    <dialog ref={dialog} className="sb-dialog" aria-label="Background worker" onCancel={onClose}>
      <h3>Background worker</h3>
      <p className="sb-hint">
        Simulated consumer: each replica handles up to <em>concurrency</em> messages at once, one process per vCPU. Handling one costs
        what its handler costs and calls; a message not acknowledged within the queue&apos;s visibility timeout is delivered again.
      </p>
      <div className="sb-row">
        <Num label="Concurrency" value={c.concurrency} onChange={(concurrency) => setC((x) => ({ ...x, concurrency }))} />
      </div>
      <div className="sb-grid">
        <Num label="Handler time (ms)" value={h.baseMs} onChange={(baseMs) => setH({ baseMs })} />
        <Num label="Handler CPU (ms)" value={h.cpuMs} onChange={(cpuMs) => setH({ cpuMs })} />
        <Num label="Handler memory (MB)" value={h.memoryMb} onChange={(memoryMb) => setH({ memoryMb })} />
        <Num label="Handler errors %" value={toPercent(h.errorRate ?? 0)} step={0.1} max={100} onChange={(v) => setH({ errorRate: v / 100 })} />
      </div>
      <div className="sb-row">
        {DEPS.filter((d) => d !== "queue").map((d) => (
          <Check
            key={d}
            label={d}
            checked={(h.deps ?? []).includes(d)}
            onChange={(on) => setH({ deps: on ? [...(h.deps ?? []), d] : (h.deps ?? []).filter((x) => x !== d) })}
          />
        ))}
      </div>
      <Errors errors={errors} />
      <div className="sb-dialog-actions">
        <button className="secondary" onClick={onClose}>
          Cancel
        </button>
        <button onClick={apply}>Apply</button>
      </div>
    </dialog>
  );
}

// QueueView opens a queue: publishes in, the backlog, deliveries to the
// workers, the redelivery loop, and the dead-letter queue.
export function QueueView({ game, node, onBack }: { game: GameState; node: SandboxNode; onBack: () => void }) {
  const q = game.flow.nodes.find((s) => s.id === node.id)?.queue;
  if (!q) return null;
  const n = (v = 0) => formatCompact(v);
  const workers = game.edges.filter((e) => e.from === node.id).map((e) => e.to);
  const g = graph(q.delivered > 0);
  const total = Math.max(q.delivered, 1e-9);
  g.column([
    { id: "pub", title: "Publishers", sub: `${n(q.published)}/s${q.rejected > 0.01 ? ` · ${n(q.rejected)}/s rejected` : ""}`, cls: q.rejected > 0.01 ? "bad" : undefined },
    ...(q.redelivered > 0.01 ? [{ id: "again", title: "Redeliveries", sub: `${n(q.redelivered)}/s`, cls: "warn" }] : []),
  ], "input");
  g.column([{ id: "backlog", title: "Backlog", sub: `${n(q.backlog)} / ${n(q.maxBacklog)} · ${q.delaySeconds.toFixed(1)} s delay`, cls: q.backlog > 1 ? "warn" : undefined }]);
  g.column([
    { id: "work", title: workers.length ? `→ ${workers.join(", ")}` : "→ no workers", sub: `${n(q.delivered)}/s delivered · fail ${pct(q.workerFailure)}`, cls: workers.length ? undefined : "bad" },
  ]);
  g.column([
    { id: "ack", title: "Acknowledged", sub: `${n(q.delivered * (1 - q.workerFailure))}/s` },
    ...(q.deadLettered > 0.001 ? [{ id: "dlq", title: "Dead-letter queue", sub: `${n(q.deadLettered)}/s · ${n(q.deadLetters)} kept`, cls: "bad" }] : []),
  ], "output");
  g.link("pub", "backlog", (q.published - q.rejected) / total);
  if (q.redelivered > 0.01) g.link("again", "backlog", q.redelivered / total);
  g.link("backlog", "work", 1);
  g.link("work", "ack", 1 - q.workerFailure);
  if (q.deadLettered > 0.001) g.link("work", "dlq", Math.max(0.05, q.deadLettered / total));
  return (
    <InnerView
      title={`Inside ${node.id}`}
      hint={
        <>
          <span className={healthLevel(q.health)}>{q.health}</span> · at-least-once · redelivered failures loop back to the backlog
        </>
      }
      graph={g}
      onBack={onBack}
    />
  );
}
