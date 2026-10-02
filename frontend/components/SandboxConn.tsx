"use client";

import { useEffect, useRef, useState } from "react";
import {
  formatCompact,
  hasBlankNumber,
  problems,
  type Command,
  type Connection,
  type GameState,
  type Listener,
  type Ruleset,
  type SandboxNode,
} from "@/lib/sandbox";
import { Num } from "./SandboxInternet";

// EdgePanel is the inspector for a selected connection (v7): what it carried
// and how it fared, and its client side, which adopted the target's listener.
export function EdgePanel({
  game,
  rules,
  from,
  to,
  onConfigure,
  onCommand,
}: {
  game: GameState;
  rules: Ruleset;
  from: string;
  to: string;
  onConfigure: (c: Command) => Promise<string | null>;
  onCommand: (c: Command) => void;
}) {
  const [open, setOpen] = useState(false);
  const edge = game.edges.find((e) => e.from === from && e.to === to);
  const s = game.flow.edges?.find((e) => e.from === from && e.to === to);
  if (!edge) return null;
  const c = edge.conn;
  const failing = s && s.rps > 0 ? s.errors / s.rps : 0;
  return (
    <aside className="sb-inspector">
      <h3>Connection</h3>
      <div className="meta">
        {from} → {to}
      </div>
      {s?.problem && (
        <p className="sb-hint bad" role="status">
          {s.problem}
        </p>
      )}
      {s && (
        <table>
          <tbody>
            <tr>
              <th>Attempts</th>
              <td className="num">{formatCompact(s.rps)}/s</td>
            </tr>
            {!!s.retryRps && (
              <tr>
                <th>Retries</th>
                <td className="num warn">{formatCompact(s.retryRps)}/s</td>
              </tr>
            )}
            <tr>
              <th>Failed</th>
              <td className={`num ${failing > 0.01 ? "bad" : ""}`}>
                {formatCompact(s.errors)}/s ({Math.round(failing * 100)}%)
              </td>
            </tr>
            <tr>
              <th>Latency</th>
              <td className="num">{s.latencyMs.toFixed(1)} ms</td>
            </tr>
          </tbody>
        </table>
      )}
      {c ? (
        <>
          <div className="meta">
            {c.protocol} · port {c.port} · {c.tls ? "TLS" : "no TLS"} · pool {c.pool} · timeout {formatCompact(c.timeoutMs)} ms
            {c.retries ? ` · ${c.retries} retries` : ""}
          </div>
          <button onClick={() => setOpen(true)}>Configure connection</button>
        </>
      ) : (
        <p className="sb-hint">This connection is set on its source: configure the traffic component.</p>
      )}
      <div className="sb-actions">
        <button className="secondary danger" onClick={() => onCommand({ type: "disconnect", from, to })}>
          Disconnect
        </button>
      </div>
      {open && c && (
        <ConnDialog
          rules={rules}
          conn={c}
          onApply={(connection) => onConfigure({ type: "configure", from, to, connection })}
          onClose={() => setOpen(false)}
        />
      )}
    </aside>
  );
}

function ConnDialog({
  rules,
  conn,
  onApply,
  onClose,
}: {
  rules: Ruleset;
  conn: Connection;
  onApply: (c: Connection) => Promise<string | null>;
  onClose: () => void;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  const [c, setC] = useState<Connection>(() => ({ ...conn }));
  const [errors, setErrors] = useState<string[]>([]);
  useEffect(() => {
    const d = dialog.current;
    d?.showModal();
    return () => d?.close();
  }, []);
  const set = (p: Partial<Connection>) => setC((x) => ({ ...x, ...p }));
  const apply = async () => {
    if (hasBlankNumber(c)) return setErrors(["Fill in every number field."]);
    const err = await onApply(c);
    if (err) setErrors(problems(err));
    else onClose();
  };
  return (
    <dialog ref={dialog} className="sb-dialog" aria-label="Connection" onCancel={onClose}>
      <h3>Connection</h3>
      <p className="sb-hint">
        Simulated: the client side of this edge. It took the target&apos;s protocol, port, and TLS when connected and follows them;
        change them here only to break the contract on purpose.
      </p>
      <div className="sb-row">
        <label className="sb-field">
          <span>Protocol</span>
          <select aria-label="Protocol" value={c.protocol} onChange={(e) => set({ protocol: e.target.value })}>
            {(rules.wireProtocols ?? []).map((p) => (
              <option key={p}>{p}</option>
            ))}
          </select>
        </label>
        <Num label="Port" value={c.port} max={65535} onChange={(port) => set({ port })} />
        <label>
          <input type="checkbox" checked={c.tls} onChange={(e) => set({ tls: e.target.checked })} /> TLS
        </label>
      </div>
      <div className="sb-row">
        <Num label="Pool (per replica)" value={c.pool} onChange={(pool) => set({ pool })} />
        <Num label="Timeout (ms)" value={c.timeoutMs} onChange={(timeoutMs) => set({ timeoutMs })} />
        <Num label="Retries" value={c.retries ?? 0} max={rules.maxRetries} onChange={(retries) => set({ retries })} />
      </div>
      <p className="sb-hint">
        Each caller replica keeps up to <em>pool</em> connections open; a call holds one for its whole latency, so a small pool to a
        slow target limits the caller. A call slower than the timeout fails; retries recover failed calls but add load.
      </p>
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

// ListenerSection shows what a data component accepts connections on and
// lets the player change it; its callers follow (v7).
export function ListenerSection({
  rules,
  node,
  onConfigure,
}: {
  rules: Ruleset;
  node: SandboxNode;
  onConfigure: (c: Command) => Promise<string | null>;
}) {
  const base = rules.listeners?.[node.kind];
  const [edit, setEdit] = useState<Listener | null>(null);
  const [errors, setErrors] = useState<string[]>([]);
  if (!base) return null;
  const l = node.listener ?? base;
  if (!edit) {
    return (
      <div className="sb-listener">
        <div className="meta">
          Listens on {l.protocol} · port {l.port} · {l.tls ? "TLS" : "no TLS"}{" "}
          <button className="link" onClick={() => setEdit({ ...l })}>
            change
          </button>
        </div>
      </div>
    );
  }
  return (
    <div className="sb-listener">
      <div className="sb-row">
        <label className="sb-field">
          <span>Protocol</span>
          <select aria-label="Listener protocol" value={edit.protocol} onChange={(e) => setEdit({ ...edit, protocol: e.target.value })}>
            {(rules.wireProtocols ?? []).map((p) => (
              <option key={p}>{p}</option>
            ))}
          </select>
        </label>
        <Num label="Listener port" value={edit.port} max={65535} onChange={(port) => setEdit({ ...edit, port })} />
        <label>
          <input type="checkbox" checked={edit.tls} onChange={(e) => setEdit({ ...edit, tls: e.target.checked })} /> TLS
        </label>
      </div>
      {errors.length > 0 && (
        <ul className="sb-errors" role="alert">
          {errors.map((e) => (
            <li key={e}>{e}</li>
          ))}
        </ul>
      )}
      <div className="sb-row">
        <button className="secondary" onClick={() => setEdit(null)}>
          Cancel
        </button>
        <button
          onClick={async () => {
            const err = await onConfigure({ type: "configure", node: node.id, listener: edit });
            if (err) setErrors(problems(err));
            else setEdit(null);
          }}
        >
          Apply
        </button>
      </div>
    </div>
  );
}
