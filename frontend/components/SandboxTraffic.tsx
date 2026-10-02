"use client";

import { useEffect, useRef, useState } from "react";
import {
  clientConfig,
  describePattern,
  formatCompact,
  hasBlankNumber,
  percentTotal,
  problems,
  toPercent,
  type ClientConfig,
  type Command,
  type GameState,
  type Ruleset,
  type SandboxNode,
} from "@/lib/sandbox";
import { InnerView, Num, VolumeFields, graph } from "./SandboxInternet";

const METHODS = ["GET", "POST", "PUT", "PATCH", "DELETE"];

const TYPE_LABEL: Record<string, string> = { web: "Web", mobile: "Mobile", api: "API client", bot: "Bot" };

export const clientTypeLabel = (t: string) => TYPE_LABEL[t] ?? t;

// TrafficPanel is a traffic component's part of the inspector: who its
// clients are, how their requests fare, and the way into its configuration.
export function TrafficPanel({
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
  const c = clientConfig(game, rules, node);
  const t = game.flow.nodes.find((s) => s.id === node.id)?.traffic;
  if (!c) return null;
  const failures: [string, number | undefined][] = [
    ["Refused", t?.refused],
    ["Not found (404)", t?.notFound],
    ["Rejected", t?.rejected],
    ["Timed out", t?.timeouts],
    ["Errors", t?.errors],
  ];
  return (
    <div className="sb-internet">
      <div className="meta">
        {c.name} · {clientTypeLabel(c.clientType)} · {c.region} · {c.protocol} {c.scheme}:{c.port}
      </div>
      {t?.problem && (
        <p className="sb-hint bad" role="status">
          {t.problem}
        </p>
      )}
      {c.endpoints.length === 0 && (
        <p className="sb-hint">
          Connect it to an application instance{rules.lb ? " (or a load balancer, gateway, or CDN in front of one)" : ""}: it takes
          on that component&apos;s protocol, port, and scheme, and one endpoint per route of the app, which you can then change.
        </p>
      )}
      {t && (
        <table>
          <tbody>
            <tr>
              <th>Source</th>
              <td>{c.source === "configured" ? `Load test: ${describePattern(c.pattern)}` : "Market (your users)"}</td>
            </tr>
            <tr>
              <th>Requests</th>
              <td className="num">{formatCompact(t.rps)}/s</td>
            </tr>
            {!!t.retryRps && (
              <tr>
                <th>Retries</th>
                <td className="num warn">{formatCompact(t.retryRps)}/s</td>
              </tr>
            )}
            {!!t.attackRps && (
              <tr>
                <th>Attack</th>
                <td className="num bad">{formatCompact(t.attackRps)}/s</td>
              </tr>
            )}
            <tr>
              <th>Succeeded</th>
              <td className="num">{formatCompact(t.success)}/s</td>
            </tr>
            {failures
              .filter(([, v]) => (v ?? 0) > 0.01)
              .map(([k, v]) => (
                <tr key={k}>
                  <th>{k}</th>
                  <td className="num bad">{formatCompact(v ?? 0)}/s</td>
                </tr>
              ))}
            <tr>
              <th>Latency</th>
              <td className="num">{t.latencyMs.toFixed(0)} ms</td>
            </tr>
            <tr>
              <th title="Requests in flight: attempts per second × mean latency (Little's law)">In flight</th>
              <td className="num">{formatCompact(t.concurrency)}</td>
            </tr>
          </tbody>
        </table>
      )}
      <button onClick={() => setOpen(true)}>Configure traffic</button>
      {open && (
        <ClientDialog
          rules={rules}
          config={c}
          onApply={(client) => onConfigure({ type: "configure", node: node.id, client })}
          onClose={() => setOpen(false)}
        />
      )}
    </div>
  );
}

interface EndpointRow {
  method: string;
  path: string;
  share: number;
}

function ClientDialog({
  rules,
  config,
  onApply,
  onClose,
}: {
  rules: Ruleset;
  config: ClientConfig;
  onApply: (c: ClientConfig) => Promise<string | null>;
  onClose: () => void;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  // Taken once, so live updates never overwrite the player's edits.
  const [c, setC] = useState<ClientConfig>(() => structuredClone(config));
  const [rows, setRows] = useState<EndpointRow[]>(() =>
    config.endpoints.map((e) => {
      const [method, ...path] = e.name.split(" ");
      return { method, path: path.join(" "), share: toPercent(e.share) };
    }),
  );
  const [errors, setErrors] = useState<string[]>([]);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    const d = dialog.current;
    d?.showModal();
    return () => d?.close();
  }, []);

  const set = (p: Partial<ClientConfig>) => setC((x) => ({ ...x, ...p }));
  const setRow = (i: number, p: Partial<EndpointRow>) => setRows((rs) => rs.map((r, j) => (j === i ? { ...r, ...p } : r)));
  const total = percentTotal(rows.map((r) => r.share));

  const apply = async () => {
    const out: ClientConfig = { ...c, endpoints: rows.map((r) => ({ name: `${r.method} ${r.path}`, share: r.share / 100 })) };
    // The market ignores the pattern, whose fields are then hidden.
    if (out.source === "market" && hasBlankNumber(out.pattern)) out.pattern = config.pattern;
    if (hasBlankNumber(out)) {
      setErrors(["Fill in every number field."]);
      return;
    }
    setBusy(true);
    const err = await onApply(out);
    setBusy(false);
    if (err) setErrors(problems(err));
    else onClose();
  };

  const select = (label: string, value: string, options: [string, string][], onChange: (v: string) => void) => (
    <label className="sb-field">
      <span>{label}</span>
      <select aria-label={label} value={value} onChange={(e) => onChange(e.target.value)}>
        {options.map(([v, text]) => (
          <option key={v} value={v}>
            {text}
          </option>
        ))}
      </select>
    </label>
  );

  return (
    <dialog ref={dialog} className="sb-dialog" aria-label="Traffic component" onCancel={onClose}>
      <h3>Traffic component</h3>
      <p className="sb-hint">
        Simulated: one population of clients. It sends to the one component it is connected to, which must speak its
        protocol, listen on its port, match its scheme, and route its endpoints, or the requests fail.
      </p>

      <details className="sb-section" open>
        <summary>Clients</summary>
        <div className="sb-row">
          <label className="sb-field">
            <span>Name</span>
            <input type="text" aria-label="Name" value={c.name} onChange={(e) => set({ name: e.target.value })} />
          </label>
          {select("Client type", c.clientType, (rules.clientTypes ?? []).map((w) => [w.name, `${clientTypeLabel(w.name)} (${toPercent(w.share)}% of users)`]), (clientType) => set({ clientType }))}
          {select("Region", c.region, (rules.regionShares ?? []).map((w) => [w.name, `${w.name} (${toPercent(w.share)}% of users)`]), (region) => set({ region }))}
        </div>
        <p className="sb-hint">
          From the market, this component takes its client type&apos;s share of your users times its region&apos;s share. Two
          components of the same type and region split it.
        </p>
      </details>

      <details className="sb-section" open>
        <summary>Connection</summary>
        <div className="sb-row">
          {select("Protocol", c.protocol, (rules.protocols ?? []).map((p) => [p, p]), (protocol) => set({ protocol }))}
          {select("Scheme", c.scheme, [["https", "https (TLS)"], ["http", "http (plain)"]], (scheme) => set({ scheme: scheme as ClientConfig["scheme"] }))}
          <Num label="Port" value={c.port} max={65535} onChange={(port) => set({ port })} />
          <Num label="Client timeout (ms)" value={c.timeoutMs} onChange={(timeoutMs) => set({ timeoutMs })} />
          {select(
            "Retries",
            String(c.retries ?? 0),
            Array.from({ length: (rules.maxRetries ?? 0) + 1 }, (_, n) => [String(n), n === 0 ? "no retries" : `${n} ${n === 1 ? "retry" : "retries"}`]),
            (v) => set({ retries: Number(v) }),
          )}
        </div>
        <label>
          <input type="checkbox" checked={c.keepAlive} onChange={(e) => set({ keepAlive: e.target.checked })} /> Keep-alive (reuse
          connections when the app does too)
        </label>
        <p className="sb-hint">
          The shorter of this timeout and the app&apos;s decides success; a client that gives up still costs the app the work.
        </p>
      </details>

      <VolumeFields
        source={c.source}
        pattern={c.pattern}
        maxRps={rules.maxTrafficRps}
        onSource={(source) => set({ source })}
        onPattern={(p) => setC((x) => ({ ...x, pattern: { ...x.pattern, ...p } }))}
      />

      <details className="sb-section" open>
        <summary>Requests</summary>
        <p className="sb-hint">
          What these clients ask for. Connecting to an app fills this in from its routes. GET is a read, anything else a write. An
          endpoint the app has no route for fails with 404.
        </p>
        {rows.map((r, i) => (
          <div className="sb-row" key={i}>
            <select aria-label={`Endpoint ${i + 1} method`} value={r.method} onChange={(e) => setRow(i, { method: e.target.value })}>
              {METHODS.map((m) => (
                <option key={m}>{m}</option>
              ))}
            </select>
            <input type="text" aria-label={`Endpoint ${i + 1} path`} className="grow" value={r.path} onChange={(e) => setRow(i, { path: e.target.value })} />
            <Num label={`Endpoint ${i + 1} %`} value={r.share} step={0.1} max={100} onChange={(share) => setRow(i, { share })} />
            <button className="secondary danger" disabled={rows.length <= 1} onClick={() => setRows((rs) => rs.filter((_, j) => j !== i))}>
              remove
            </button>
          </div>
        ))}
        <div className={`sb-total ${total === 100 ? "ok" : "warn"}`}>
          Requests: {total}%{total !== 100 && " (must be 100%)"}
        </div>
        <button className="secondary" disabled={rows.length >= 12} onClick={() => setRows((rs) => [...rs, { method: "GET", path: `/new-${rs.length + 1}`, share: 0 }])}>
          Add endpoint
        </button>
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
        <button onClick={apply} disabled={busy}>
          Apply
        </button>
      </div>
    </dialog>
  );
}

// TrafficView opens a traffic component: its population, what it asks for,
// and whether the component it sends to accepts it. Rates are the engine's;
// edge widths and labels show the configured shares.
export function TrafficView({ game, rules, node, onBack }: { game: GameState; rules: Ruleset; node: SandboxNode; onBack: () => void }) {
  const c = clientConfig(game, rules, node);
  if (!c) return null;
  const t = game.flow.nodes.find((s) => s.id === node.id)?.traffic;
  const to = game.edges.find((e) => e.from === node.id)?.to;
  const notFound = new Set(game.flow.nodes.find((s) => s.id === to)?.app?.routes?.filter((r) => r.notFound).map((r) => r.endpoint));
  const g = graph((t?.rps ?? 0) > 0 && !t?.problem);

  const n = (v = 0) => formatCompact(v);
  g.column([{
    id: "clients",
    title: c.name,
    sub: `${clientTypeLabel(c.clientType)} · ${c.region} · ${n(t?.rps)}/s${t?.retryRps ? ` +${n(t.retryRps)} retries` : ""}`,
  }], "input");
  g.column(c.endpoints.map((e) => {
    const missing = notFound.has(e.name);
    return { id: `e:${e.name}`, title: e.name, sub: missing ? "404: no route" : `${Math.round(e.share * 100)}% · ${n((t?.rps ?? 0) * e.share)}/s`, cls: missing ? "bad" : undefined };
  }));
  g.column([{
    id: "out",
    title: to ?? "Not connected",
    sub: t?.problem ?? `${n(t?.success)}/s ok · ${(t?.latencyMs ?? 0).toFixed(0)} ms`,
    cls: t?.problem ? "bad" : undefined,
  }], "output");
  // Shares are in the nodes; edges only show the flow, and a refused
  // connection carries none.
  for (const e of c.endpoints) {
    g.link("clients", `e:${e.name}`, e.share, false);
    if (!t?.problem && !notFound.has(e.name)) g.link(`e:${e.name}`, "out", e.share, false);
  }

  const conn = `${c.protocol} · ${c.scheme}:${c.port} · ${c.keepAlive ? "keep-alive" : "no keep-alive"} · timeout ${n(c.timeoutMs)} ms`;
  return <InnerView title={`Inside ${node.id}`} hint={`${conn} · ${c.source === "configured" ? "load test" : "market"}`} graph={g} onBack={onBack} />;
}
