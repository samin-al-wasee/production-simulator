"use client";

import { useEffect, useRef, useState } from "react";
import {
  describePattern,
  endpointName,
  formatClock,
  formatCompact,
  fromDraft,
  hasBlankNumber,
  newGroupKey,
  percentTotal,
  problems,
  toDraft,
  type GameState,
  type GroupDraft,
  type Pattern,
  type PatternShape,
  type Ruleset,
  type SandboxNode,
  type TrafficConfig,
  type Rate,
  type TrafficDraft,
} from "@/lib/sandbox";

const SHAPES: { shape: PatternShape; label: string }[] = [
  { shape: "constant", label: "Constant" },
  { shape: "ramp", label: "Gradual (ramp up or down)" },
  { shape: "spike", label: "Spike" },
  { shape: "burst", label: "Bursts" },
  { shape: "periodic", label: "Periodic wave" },
  { shape: "schedule", label: "Daily schedule" },
];

const METHODS = ["GET", "POST", "PUT", "PATCH", "DELETE"];

// Which pattern fields each shape uses, and what they mean for it.
const FIELDS: Record<PatternShape, [keyof Pattern, string][]> = {
  constant: [["rps", "Requests/s"]],
  ramp: [["rps", "From (req/s)"], ["peakRps", "To (req/s)"], ["minutes", "Over (min)"]],
  spike: [["rps", "Base (req/s)"], ["peakRps", "Peak (req/s)"], ["startMinutes", "Starts after (min)"], ["minutes", "Lasts (min)"]],
  burst: [["rps", "Base (req/s)"], ["peakRps", "Burst (req/s)"], ["minutes", "Burst lasts (min)"], ["periodMinutes", "Every (min)"]],
  periodic: [["rps", "Low (req/s)"], ["peakRps", "High (req/s)"], ["periodMinutes", "Period (min)"]],
  schedule: [],
};

// A new shape starts from the current rate so switching is not a surprise.
function reshape(p: Pattern, shape: PatternShape): Pattern {
  const rps = Number.isFinite(p.rps) ? p.rps : 0;
  return {
    shape,
    rps,
    peakRps: p.peakRps ?? rps * 2,
    startMinutes: p.startMinutes ?? 30,
    minutes: p.minutes ?? 60,
    periodMinutes: p.periodMinutes ?? 120,
    schedule: p.schedule ?? [
      { hour: 9, rps },
      { hour: 18, rps: rps * 2 },
    ],
  };
}

function Num({
  label,
  value,
  onChange,
  step = 1,
  min = 0,
  max,
}: {
  label: string;
  value: number | undefined;
  onChange: (v: number) => void;
  step?: number;
  min?: number;
  max?: number;
}) {
  return (
    <label className="sb-field">
      <span>{label}</span>
      <input
        type="number"
        aria-label={label}
        value={value !== undefined && Number.isFinite(value) ? value : ""}
        step={step}
        min={min}
        max={max}
        onChange={(e) => onChange(e.target.valueAsNumber)}
      />
    </label>
  );
}

function Total({ values, label }: { values: number[]; label: string }) {
  const total = percentTotal(values);
  return (
    <div className={`sb-total ${total === 100 ? "ok" : "warn"}`}>
      {label}: {total}%{total !== 100 && " (must be 100%)"}
    </div>
  );
}

function Breakdown({ title, rows, total }: { title: string; rows?: Rate[]; total: number }) {
  if (!rows || rows.length === 0) return null;
  return (
    <>
      <h4>{title}</h4>
      <table className="sb-breakdown">
        <tbody>
          {rows.map((r) => (
            <tr key={r.name}>
              <th>{r.name}</th>
              <td className="num">{formatCompact(r.rps)}/s</td>
              <td className="num">{total > 0 ? Math.round((r.rps / total) * 100) : 0}%</td>
            </tr>
          ))}
        </tbody>
      </table>
    </>
  );
}

// InternetPanel is the Internet's part of the inspector: what it sends now,
// and the way into its configuration.
export function InternetPanel({
  game,
  rules,
  node,
  onConfigure,
}: {
  game: GameState;
  rules: Ruleset;
  node: SandboxNode;
  onConfigure: (tc: TrafficConfig) => Promise<string | null>;
}) {
  const [open, setOpen] = useState(false);
  const t = game.flow.traffic;
  // The palette's ruleset is the latest; an older game may have none.
  const config = node.traffic ?? (game.ruleset === rules.version ? rules.traffic : undefined);
  return (
    <div className="sb-internet">
      {t && (
        <table>
          <tbody>
            <tr>
              <th>Source</th>
              <td>{t.source === "configured" ? "Load test" : "Market (your users)"}</td>
            </tr>
            {config?.source === "configured" && (
              <tr>
                <th>Pattern</th>
                <td>{describePattern(config.pattern)}</td>
              </tr>
            )}
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
            <tr>
              <th title="Requests in flight: attempts per second × mean latency (Little's law)">In flight</th>
              <td className="num">{formatCompact(t.concurrency)}</td>
            </tr>
          </tbody>
        </table>
      )}
      {config ? (
        <button onClick={() => setOpen(true)}>Configure traffic</button>
      ) : (
        <p className="sb-hint">Ruleset {game.ruleset} has no traffic configuration; start a new game to configure the Internet.</p>
      )}
      {t && (
        <details className="sb-section">
          <summary>Traffic breakdown</summary>
          <Breakdown title="By group" rows={t.groups} total={t.rps} />
          <Breakdown title="By region" rows={t.regions} total={t.rps} />
          <Breakdown title="By endpoint" rows={t.endpoints} total={t.rps} />
        </details>
      )}
      {open && config && (
        <TrafficDialog rules={rules} config={config} onConfigure={onConfigure} onClose={() => setOpen(false)} />
      )}
    </div>
  );
}

function TrafficDialog({
  rules,
  config,
  onConfigure,
  onClose,
}: {
  rules: Ruleset;
  config: TrafficConfig;
  onConfigure: (tc: TrafficConfig) => Promise<string | null>;
  onClose: () => void;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  // The draft is taken once: live updates must not overwrite the player's edits.
  const [draft, setDraft] = useState<TrafficDraft>(() => toDraft(config));
  const [errors, setErrors] = useState<string[]>([]);
  const [busy, setBusy] = useState(false);
  const regions = rules.regions ?? [];
  const maxRetries = rules.maxRetries ?? 0;

  useEffect(() => {
    const d = dialog.current;
    d?.showModal();
    return () => d?.close();
  }, []);

  const set = (f: (d: TrafficDraft) => TrafficDraft) => setDraft((d) => f(d));
  const setPattern = (p: Partial<Pattern>) => set((d) => ({ ...d, pattern: { ...d.pattern, ...p } }));
  const setGroup = (i: number, g: Partial<GroupDraft>) =>
    set((d) => ({ ...d, groups: d.groups.map((x, j) => (j === i ? { ...x, ...g } : x)) }));

  const apply = async () => {
    const tc = fromDraft(draft);
    // The market ignores the pattern, whose fields are then hidden; an
    // unfinished one is left as it was rather than sent.
    if (tc.source === "market" && hasBlankNumber({ ...tc, groups: [], endpoints: [] })) tc.pattern = config.pattern;
    // An empty number field would reach the engine as 0; ask for it instead.
    if (hasBlankNumber(tc)) {
      setErrors(["Fill in every number field."]);
      return;
    }
    setBusy(true);
    const err = await onConfigure(tc);
    setBusy(false);
    if (err) setErrors(problems(err));
    else onClose();
  };

  const p = draft.pattern;
  const configured = draft.source === "configured";
  // A new schedule step starts on the hour after the last one.
  const lastStep = p.schedule?.[p.schedule.length - 1];
  const nextHour = lastStep ? Math.floor(lastStep.hour) + 1 : 0;
  return (
    <dialog ref={dialog} className="sb-dialog" aria-label="Internet traffic" onCancel={onClose}>
      <h3>Internet traffic</h3>
      <p className="sb-hint">
        Simulated: the engine turns this configuration into load on your components each tick. Groups, requests, and regions
        live inside the Internet, not on the canvas.
      </p>

      <fieldset className="sb-source">
        <legend>Volume</legend>
        <label>
          <input type="radio" name="source" checked={!configured} onChange={() => set((d) => ({ ...d, source: "market" }))} />
          Market: follows your users and the time of day
        </label>
        <label>
          <input type="radio" name="source" checked={configured} onChange={() => set((d) => ({ ...d, source: "configured" }))} />
          Load test: you set the rate and its pattern
        </label>
        {configured ? (
          <>
            <p className="sb-hint">
              A load test earns nothing, and users, satisfaction, and goals hold still until you switch back. Events still happen.
              The pattern starts when you apply it.
            </p>
            <label className="sb-field">
              <span>Pattern</span>
              <select aria-label="Pattern" value={p.shape} onChange={(e) => setPattern(reshape(p, e.target.value as PatternShape))}>
                {SHAPES.map((s) => (
                  <option key={s.shape} value={s.shape}>
                    {s.label}
                  </option>
                ))}
              </select>
            </label>
            <div className="sb-row">
              {FIELDS[p.shape].map(([key, label]) => (
                <Num
                  key={key}
                  label={label}
                  value={p[key] as number | undefined}
                  max={key === "rps" || key === "peakRps" ? rules.maxTrafficRps : undefined}
                  onChange={(v) => setPattern({ [key]: v })}
                />
              ))}
            </div>
            {p.shape === "schedule" && (
              <div className="sb-schedule">
                {(p.schedule ?? []).map((s, i) => (
                  <div className="sb-row" key={i}>
                    <label className="sb-field">
                      <span>From</span>
                      <input
                        type="time"
                        aria-label={`Step ${i + 1} time`}
                        value={formatClock(1, s.hour).slice(-5)}
                        onChange={(e) => {
                          const [h, m] = e.target.value.split(":").map(Number);
                          const hour = h + m / 60;
                          setPattern({ schedule: p.schedule!.map((x, j) => (j === i ? { ...x, hour } : x)) });
                        }}
                      />
                    </label>
                    <Num
                      label={`Step ${i + 1} req/s`}
                      value={s.rps}
                      onChange={(rps) => setPattern({ schedule: p.schedule!.map((x, j) => (j === i ? { ...x, rps } : x)) })}
                    />
                    <button
                      className="secondary"
                      disabled={(p.schedule ?? []).length <= 1}
                      onClick={() => setPattern({ schedule: p.schedule!.filter((_, j) => j !== i) })}
                    >
                      remove
                    </button>
                  </div>
                ))}
                <button
                  className="secondary"
                  disabled={nextHour >= 24}
                  onClick={() => setPattern({ schedule: [...(p.schedule ?? []), { hour: nextHour, rps: lastStep?.rps ?? 0 }] })}
                >
                  Add step
                </button>
              </div>
            )}
          </>
        ) : (
          <p className="sb-hint">RPS = active users × requests per user × time of day × events. Grow it by keeping users happy.</p>
        )}
      </fieldset>

      <details className="sb-section" open>
        <summary>Traffic groups</summary>
        <p className="sb-hint">Populations of clients that share the volume. Each has its own requests and regions.</p>
        {draft.groups.map((g, i) => (
          <div className="sb-group" key={g.key}>
            <div className="sb-row">
              <label className="sb-field grow">
                <span>Name</span>
                <input type="text" aria-label={`Group ${i + 1} name`} value={g.name} onChange={(e) => setGroup(i, { name: e.target.value })} />
              </label>
              <Num label={`Group ${i + 1} share %`} value={g.share} step={0.1} max={100} onChange={(share) => setGroup(i, { share })} />
              <button
                className="secondary danger"
                disabled={draft.groups.length <= 1}
                onClick={() => set((d) => ({ ...d, groups: d.groups.filter((_, j) => j !== i) }))}
              >
                remove
              </button>
            </div>
            <details>
              <summary>
                Requests and regions of {g.name || `group ${i + 1}`}
              </summary>
              <div className="sb-grid">
                {draft.endpoints.map((e, k) => (
                  <Num
                    key={k}
                    label={`${endpointName(e)} %`}
                    value={g.endpoints[k]}
                    step={0.1}
                    max={100}
                    onChange={(v) => setGroup(i, { endpoints: g.endpoints.map((x, j) => (j === k ? v : x)) })}
                  />
                ))}
              </div>
              <Total label="Requests" values={draft.endpoints.map((_, k) => g.endpoints[k] ?? 0)} />
              <div className="sb-grid">
                {regions.map((r) => (
                  <Num
                    key={r}
                    label={`${r} %`}
                    value={g.regions[r] ?? 0}
                    step={0.1}
                    max={100}
                    onChange={(v) => setGroup(i, { regions: { ...g.regions, [r]: v } })}
                  />
                ))}
              </div>
              <Total label="Regions" values={regions.map((r) => g.regions[r] ?? 0)} />
            </details>
          </div>
        ))}
        <Total label="Groups" values={draft.groups.map((g) => g.share)} />
        <button
          className="secondary"
          disabled={draft.groups.length >= 8}
          onClick={() =>
            set((d) => ({
              ...d,
              groups: [
                ...d.groups,
                {
                  key: newGroupKey(),
                  name: `Group ${d.groups.length + 1}`,
                  share: 0,
                  retries: 0,
                  endpoints: d.endpoints.map(() => 0),
                  regions: {},
                },
              ],
            }))
          }
        >
          Add group
        </button>
      </details>

      <details className="sb-section">
        <summary>Endpoints</summary>
        <p className="sb-hint">
          GET is a read, anything else a write. A CDN can answer cacheable reads; storage requests also fetch from object storage.
        </p>
        {draft.endpoints.map((e, k) => (
          <div className="sb-row" key={k}>
            <select
              aria-label={`Endpoint ${k + 1} method`}
              value={e.method}
              onChange={(ev) =>
                set((d) => ({
                  ...d,
                  endpoints: d.endpoints.map((x, j) =>
                    j === k ? { ...x, method: ev.target.value, cacheable: ev.target.value === "GET" ? x.cacheable : false } : x,
                  ),
                }))
              }
            >
              {METHODS.map((m) => (
                <option key={m}>{m}</option>
              ))}
            </select>
            <input
              type="text"
              aria-label={`Endpoint ${k + 1} path`}
              className="grow"
              value={e.path}
              onChange={(ev) => set((d) => ({ ...d, endpoints: d.endpoints.map((x, j) => (j === k ? { ...x, path: ev.target.value } : x)) }))}
            />
            <label>
              <input
                type="checkbox"
                checked={!!e.cacheable}
                disabled={e.method !== "GET"}
                onChange={(ev) =>
                  set((d) => ({ ...d, endpoints: d.endpoints.map((x, j) => (j === k ? { ...x, cacheable: ev.target.checked } : x)) }))
                }
              />
              cacheable
            </label>
            <label>
              <input
                type="checkbox"
                checked={!!e.storage}
                onChange={(ev) =>
                  set((d) => ({ ...d, endpoints: d.endpoints.map((x, j) => (j === k ? { ...x, storage: ev.target.checked } : x)) }))
                }
              />
              storage
            </label>
            <button
              className="secondary danger"
              disabled={draft.endpoints.length <= 1}
              onClick={() =>
                set((d) => ({
                  ...d,
                  endpoints: d.endpoints.filter((_, j) => j !== k),
                  groups: d.groups.map((g) => ({ ...g, endpoints: g.endpoints.filter((_, j) => j !== k) })),
                }))
              }
            >
              remove
            </button>
          </div>
        ))}
        <button
          className="secondary"
          disabled={draft.endpoints.length >= 12}
          onClick={() =>
            set((d) => ({
              ...d,
              endpoints: [...d.endpoints, { method: "GET", path: `/new-${d.endpoints.length + 1}` }],
              groups: d.groups.map((g) => ({ ...g, endpoints: [...g.endpoints, 0] })),
            }))
          }
        >
          Add endpoint
        </button>
      </details>

      <details className="sb-section">
        <summary>Advanced: client retries</summary>
        <p className="sb-hint">
          A client retries a failed request up to this many times. Retries rescue brief failures but add load when the system is
          already failing, which can turn a slowdown into an outage.
        </p>
        {draft.groups.map((g, i) => (
          <label className="sb-field inline" key={g.key}>
            <span>{g.name || `Group ${i + 1}`}</span>
            <select aria-label={`${g.name} retries`} value={g.retries} onChange={(e) => setGroup(i, { retries: Number(e.target.value) })}>
              {Array.from({ length: maxRetries + 1 }, (_, n) => (
                <option key={n} value={n}>
                  {n === 0 ? "no retries" : `${n} ${n === 1 ? "retry" : "retries"}`}
                </option>
              ))}
            </select>
          </label>
        ))}
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
