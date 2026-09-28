"use client";

import { useState } from "react";
import { simulatePipeline, type PipelineRun } from "@/lib/client";
import type { PipelineInfo } from "@/lib/api";
import { formatDuration } from "@/lib/format";
import { layoutBars } from "@/lib/timeline";

export function PipelineSimulator({ pipelines }: { pipelines: PipelineInfo[] }) {
  const [name, setName] = useState(pipelines[0]?.name ?? "");
  const [seed, setSeed] = useState(1);
  const [warmCache, setWarmCache] = useState(false);
  const [badRelease, setBadRelease] = useState(false);
  const [failStep, setFailStep] = useState("");
  const [run, setRun] = useState<PipelineRun | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function simulate() {
    setBusy(true);
    setError(null);
    try {
      setRun(await simulatePipeline(name, { seed, warmCache, badRelease, failStep }));
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  const bars = run
    ? layoutBars(
        run.stages.map((s) => ({ name: s.name, start: s.start, end: s.end })),
        run.duration,
      )
    : [];
  const statusClass = run?.status === "succeeded" ? "ok" : run?.status === "rolled-back" ? "warn" : "bad";

  return (
    <>
      <div className="controls">
        <label>
          Pipeline
          <select value={name} onChange={(e) => setName(e.target.value)}>
            {pipelines.map((p) => (
              <option key={p.name} value={p.name}>
                {p.name}
              </option>
            ))}
          </select>
        </label>
        <label>
          Seed
          <input type="number" value={seed} onChange={(e) => setSeed(Number(e.target.value))} style={{ width: 80 }} />
        </label>
        <label>
          <input type="checkbox" checked={warmCache} onChange={(e) => setWarmCache(e.target.checked)} /> Warm cache
        </label>
        <label>
          <input type="checkbox" checked={badRelease} onChange={(e) => setBadRelease(e.target.checked)} /> Bad release
        </label>
        <label>
          Fail step
          <input type="text" value={failStep} onChange={(e) => setFailStep(e.target.value)} placeholder="e.g. unit" style={{ width: 120 }} />
        </label>
        <button onClick={simulate} disabled={busy || !name}>
          {busy ? "Simulating…" : "Simulate run"}
        </button>
      </div>
      {error && (
        <div className="notice" role="alert">
          {error}
        </div>
      )}
      {run && (
        <section className="panel" aria-label="Pipeline run">
          <h3>
            {run.pipeline}: <span className={`status ${statusClass}`}>{run.status.toUpperCase()}</span> in{" "}
            {formatDuration(run.duration)} <span className="legend">(virtual time; nothing was executed)</span>
          </h3>
          <div className="bars" role="img" aria-label="Stage timeline">
            {bars.map((b, i) => (
              <div
                key={b.name}
                className={`bar ${run.stages[i].status}`}
                style={{ left: `${b.leftPercent}%`, width: `${b.widthPercent}%` }}
                title={`${b.name}: ${run.stages[i].status}`}
              />
            ))}
          </div>
          <table>
            <thead>
              <tr>
                <th>Stage / step</th>
                <th>Status</th>
                <th className="num">Attempts</th>
                <th className="num">Start</th>
                <th className="num">End</th>
              </tr>
            </thead>
            <tbody>
              {run.stages.flatMap((s) => [
                <tr key={s.name}>
                  <td>
                    <strong>{s.name}</strong>
                  </td>
                  <td>
                    <span className={`status ${s.status === "succeeded" ? "ok" : s.status === "skipped" ? "warn" : s.status === "rolled-back" ? "warn" : "bad"}`}>
                      {s.status}
                    </span>
                  </td>
                  <td className="num" />
                  <td className="num">{formatDuration(s.start)}</td>
                  <td className="num">{formatDuration(s.end)}</td>
                </tr>,
                ...(s.steps ?? []).map((st) => (
                  <tr key={`${s.name}/${st.name}`}>
                    <td style={{ paddingLeft: 24 }}>{st.name}</td>
                    <td>
                      <span className={`status ${st.status === "succeeded" ? "ok" : "bad"}`}>{st.status}</span>
                    </td>
                    <td className="num">{st.attempts}</td>
                    <td className="num">{formatDuration(st.start)}</td>
                    <td className="num">{formatDuration(st.end)}</td>
                  </tr>
                )),
              ])}
            </tbody>
          </table>
          <h3 style={{ marginTop: 16 }}>Events</h3>
          <pre className="log">
            {run.events.map((e) => `${formatDuration(e.at).padStart(6)}  ${e.stage.padEnd(8)} ${e.message}`).join("\n")}
          </pre>
        </section>
      )}
    </>
  );
}
