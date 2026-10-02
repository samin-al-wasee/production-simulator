"use client";

import { useEffect, useRef, useState } from "react";
import {
  formatCompact,
  formatMoney,
  hasBlankNumber,
  healthLevel,
  problems,
  type CDNConfig,
  type Command,
  type GameState,
  type GatewayConfig,
  type LBConfig,
  type Ruleset,
  type SandboxNode,
} from "@/lib/sandbox";
import { InnerView, Num, graph } from "./SandboxInternet";

type Edge = { lb: LBConfig; gateway: GatewayConfig; cdn: CDNConfig };

// edgeConfig is an edge component's configuration: its own once set, else
// the ruleset's (v13).
function edgeConfig<K extends keyof Edge>(game: GameState, rules: Ruleset, node: SandboxNode, key: K): Edge[K] | undefined {
  return (node[key] as Edge[K] | undefined) ?? (game.ruleset === rules.version ? (rules[key] as Edge[K] | undefined) : undefined);
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

function Dialog({
  label,
  hint,
  onApply,
  onClose,
  children,
}: {
  label: string;
  hint: string;
  onApply: () => Promise<string | null>;
  onClose: () => void;
  children: React.ReactNode;
}) {
  const dialog = useDialog();
  const [errors, setErrors] = useState<string[]>([]);
  return (
    <dialog ref={dialog} className="sb-dialog" aria-label={label} onCancel={onClose}>
      <h3>{label}</h3>
      <p className="sb-hint">{hint}</p>
      {children}
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
        <button
          onClick={async () => {
            const err = await onApply();
            if (err) setErrors(problems(err));
            else onClose();
          }}
        >
          Apply
        </button>
      </div>
    </dialog>
  );
}

function Targets({ game, node }: { game: GameState; node: SandboxNode }) {
  const e = game.flow.nodes.find((s) => s.id === node.id)?.edge;
  if (!e?.targets?.length) return null;
  return (
    <>
      <h4>Targets</h4>
      <table className="sb-breakdown">
        <tbody>
          {e.targets.map((t) => (
            <tr key={t.node}>
              <th>{t.node}</th>
              <td className="num">{formatCompact(t.rps)}/s</td>
              <td className={`num ${t.healthy ? "" : "bad"}`}>
                {Math.round(t.share * 100)}%{t.healthy ? "" : " · down"}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </>
  );
}

// EdgeNodePanel is the inspector part of a load balancer, gateway, or CDN.
export function EdgeNodePanel({
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
  const e = game.flow.nodes.find((s) => s.id === node.id)?.edge;
  const send = (c: Partial<Edge>) => onConfigure({ type: "configure", node: node.id, ...c });
  if (node.kind === "load-balancer") {
    const cfg = edgeConfig(game, rules, node, "lb");
    if (!cfg) return null;
    return (
      <div className="sb-internet">
        <div className="meta">
          {cfg.algorithm} · health checks {cfg.healthChecks ? "on" : "off"}
          {e && <> · <span className={healthLevel(e.health)}>{e.health}</span></>}
        </div>
        <Targets game={game} node={node} />
        <button onClick={() => setOpen(true)}>Configure load balancer</button>
        {open && <LBDialog config={cfg} onApply={(lb) => send({ lb })} onClose={() => setOpen(false)} />}
      </div>
    );
  }
  if (node.kind === "api-gateway") {
    const cfg = edgeConfig(game, rules, node, "gateway");
    if (!cfg) return null;
    return (
      <div className="sb-internet">
        <div className="meta">
          {cfg.routes.map((r) => `${r.prefix} → ${r.service}`).join(" · ")}
          {cfg.auth ? " · auth" : ""}
          {cfg.rateLimitRps ? ` · limit ${formatCompact(cfg.rateLimitRps)}/s` : ""}
        </div>
        {e && (
          <table>
            <tbody>
              <tr>
                <th>Health</th>
                <td className={`num ${healthLevel(e.health)}`}>{e.health}</td>
              </tr>
              {!!e.notFound && (
                <tr>
                  <th>Not found (404)</th>
                  <td className="num bad">{formatCompact(e.notFound)}/s</td>
                </tr>
              )}
              {!!e.limited && (
                <tr>
                  <th>Rate limited (429)</th>
                  <td className="num warn">{formatCompact(e.limited)}/s</td>
                </tr>
              )}
            </tbody>
          </table>
        )}
        <Targets game={game} node={node} />
        <button onClick={() => setOpen(true)}>Configure gateway</button>
        {open && <GatewayDialog config={cfg} onApply={(gateway) => send({ gateway })} onClose={() => setOpen(false)} />}
      </div>
    );
  }
  const cfg = edgeConfig(game, rules, node, "cdn");
  if (!cfg) return null;
  return (
    <div className="sb-internet">
      <div className="meta">
        TTL {formatCompact(cfg.ttlSeconds)} s · {cfg.cacheable?.length ? cfg.cacheable.join(", ") : "every GET"} · managed: priced by use
      </div>
      {e && (
        <table>
          <tbody>
            <tr>
              <th>Health</th>
              <td className={`num ${healthLevel(e.health)}`}>{e.health}</td>
            </tr>
            <tr>
              <th>Hit ratio</th>
              <td className="num">{Math.round((e.hitRatio ?? 0) * 100)}%</td>
            </tr>
            <tr>
              <th>Hits / misses</th>
              <td className="num">
                {formatCompact(e.hits ?? 0)}/s · {formatCompact(e.misses ?? 0)}/s to the origin
              </td>
            </tr>
            <tr>
              <th>Egress</th>
              <td className="num">{formatCompact(e.egressMbps ?? 0)} Mbps</td>
            </tr>
            <tr>
              <th>Cost</th>
              <td className="num">{formatMoney(e.cost ?? 0)}/h</td>
            </tr>
          </tbody>
        </table>
      )}
      <button onClick={() => setOpen(true)}>Configure CDN</button>
      {open && <CDNDialog config={cfg} onApply={(cdn) => send({ cdn })} onClose={() => setOpen(false)} />}
    </div>
  );
}

function LBDialog({ config, onApply, onClose }: { config: LBConfig; onApply: (c: LBConfig) => Promise<string | null>; onClose: () => void }) {
  const [c, setC] = useState<LBConfig>(() => ({ ...config }));
  return (
    <Dialog
      label="Load balancer"
      hint="Simulated: round robin gives every replica the same share whatever its size; least connections follows capacity. Without health checks a failed target keeps its share, and those requests fail."
      onApply={() => onApply(c)}
      onClose={onClose}
    >
      <div className="sb-row">
        <label className="sb-field">
          <span>Algorithm</span>
          <select aria-label="Algorithm" value={c.algorithm} onChange={(e) => setC({ ...c, algorithm: e.target.value as LBConfig["algorithm"] })}>
            <option value="least-connections">least connections</option>
            <option value="round-robin">round robin</option>
          </select>
        </label>
        <label>
          <input type="checkbox" checked={c.healthChecks} onChange={(e) => setC({ ...c, healthChecks: e.target.checked })} /> Health checks
        </label>
      </div>
    </Dialog>
  );
}

function GatewayDialog({ config, onApply, onClose }: { config: GatewayConfig; onApply: (c: GatewayConfig) => Promise<string | null>; onClose: () => void }) {
  const [c, setC] = useState<GatewayConfig>(() => structuredClone(config));
  const setRoute = (i: number, p: Partial<GatewayConfig["routes"][number]>) =>
    setC((x) => ({ ...x, routes: x.routes.map((r, j) => (j === i ? { ...r, ...p } : r)) }));
  return (
    <Dialog
      label="API gateway"
      hint="Simulated: each request goes to the service of the longest matching path prefix (an application's name); no match is a 404 at the gateway. Above the rate limit it answers 429."
      onApply={() => (hasBlankNumber(c) ? Promise.resolve("Fill in every number field.") : onApply(c))}
      onClose={onClose}
    >
      {c.routes.map((r, i) => (
        <div className="sb-row" key={i}>
          <label className="sb-field">
            <span>Prefix</span>
            <input type="text" aria-label={`Route ${i + 1} prefix`} value={r.prefix} onChange={(e) => setRoute(i, { prefix: e.target.value })} />
          </label>
          <label className="sb-field grow">
            <span>Service</span>
            <input type="text" aria-label={`Route ${i + 1} service`} value={r.service} onChange={(e) => setRoute(i, { service: e.target.value })} />
          </label>
          <button className="secondary danger" disabled={c.routes.length <= 1} onClick={() => setC((x) => ({ ...x, routes: x.routes.filter((_, j) => j !== i) }))}>
            remove
          </button>
        </div>
      ))}
      <button className="secondary" onClick={() => setC((x) => ({ ...x, routes: [...x.routes, { prefix: "/new", service: "API" }] }))}>
        Add route
      </button>
      <div className="sb-row">
        <label>
          <input type="checkbox" checked={c.auth} onChange={(e) => setC({ ...c, auth: e.target.checked })} /> Authenticate at the gateway
        </label>
        <Num label="Rate limit (req/s, 0 = none)" value={c.rateLimitRps ?? 0} onChange={(rateLimitRps) => setC({ ...c, rateLimitRps })} />
      </div>
    </Dialog>
  );
}

function CDNDialog({ config, onApply, onClose }: { config: CDNConfig; onApply: (c: CDNConfig) => Promise<string | null>; onClose: () => void }) {
  const [c, setC] = useState<CDNConfig>(() => structuredClone(config));
  const [list, setList] = useState((config.cacheable ?? []).join("\n"));
  return (
    <Dialog
      label="CDN"
      hint="Simulated edge cache: a cacheable endpoint's request is a hit when its response is still fresh (TTL against how often each object is asked for); misses go to the origin. Priced per GB and per request."
      onApply={() => {
        const out = { ...c, cacheable: list.split("\n").map((s) => s.trim()).filter(Boolean) };
        return hasBlankNumber(out) ? Promise.resolve("Fill in every number field.") : onApply(out);
      }}
      onClose={onClose}
    >
      <div className="sb-row">
        <Num label="TTL (s)" value={c.ttlSeconds} onChange={(ttlSeconds) => setC({ ...c, ttlSeconds })} />
        <Num label="Objects per endpoint" value={c.objectsPerEndpoint} onChange={(objectsPerEndpoint) => setC({ ...c, objectsPerEndpoint })} />
        <Num label="Object size (KB)" value={c.objectKb} onChange={(objectKb) => setC({ ...c, objectKb })} />
      </div>
      <label className="sb-field">
        <span>Cacheable endpoints, one per line (empty: every GET)</span>
        <textarea aria-label="Cacheable endpoints" rows={4} value={list} onChange={(e) => setList(e.target.value)} />
      </label>
    </Dialog>
  );
}

// EdgeView opens a load balancer, gateway, or CDN: what came in, what it
// answered or turned away itself, and where it sent the rest.
export function EdgeView({ game, node, onBack }: { game: GameState; node: SandboxNode; onBack: () => void }) {
  const s = game.flow.nodes.find((x) => x.id === node.id);
  const e = s?.edge;
  if (!s || !e) return null;
  const n = (v = 0) => formatCompact(v);
  const g = graph(s.served > 0);
  const kind = node.kind === "load-balancer" ? "Load balancer" : node.kind === "api-gateway" ? "Gateway" : "CDN";
  g.column([{ id: "in", title: "Requests", sub: `${n(s.offered)}/s` }], "input");
  const self = [
    ...(e.hits ? [{ id: "hits", title: "Edge hits", sub: `${n(e.hits)}/s · ${Math.round((e.hitRatio ?? 0) * 100)}%` }] : []),
    ...(e.notFound ? [{ id: "nf", title: "Not found", sub: `${n(e.notFound)}/s · 404`, cls: "bad" }] : []),
    ...(e.limited ? [{ id: "rl", title: "Rate limited", sub: `${n(e.limited)}/s · 429`, cls: "warn" }] : []),
  ];
  g.column([{ id: "node", title: kind, sub: `${s.latencyMs.toFixed(1)} ms` }, ...self]);
  g.column((e.targets ?? []).map((t) => ({ id: `t:${t.node}`, title: t.node, sub: `${n(t.rps)}/s · ${Math.round(t.share * 100)}%${t.healthy ? "" : " · down"}`, cls: t.healthy ? undefined : "bad" })), "output");
  const total = Math.max(s.offered, 1e-9);
  const passed = (e.targets ?? []).reduce((a, t) => a + t.rps, 0);
  g.link("in", "node", passed / total);
  for (const x of self) g.link("in", x.id, ((x.id === "hits" ? e.hits : x.id === "nf" ? e.notFound : e.limited) ?? 0) / total);
  for (const t of e.targets ?? []) g.link("node", `t:${t.node}`, t.rps / total);
  return (
    <InnerView
      title={`Inside ${node.id}`}
      hint={
        <>
          <span className={healthLevel(e.health)}>{e.health}</span> · {kind.toLowerCase()} · {n(s.offered)}/s in
        </>
      }
      graph={g}
      onBack={onBack}
    />
  );
}
