"use client";

import { useEffect, useRef, useState } from "react";
import { getExperimentRun, startExperiment, type ExperimentRun } from "@/lib/client";
import type { ExperimentInfo } from "@/lib/api";

const POLL_MS = 1000;

export function ExperimentCard({ experiment, runsEnabled }: { experiment: ExperimentInfo; runsEnabled: boolean }) {
  const [run, setRun] = useState<ExperimentRun | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [starting, setStarting] = useState(false);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(
    () => () => {
      if (timer.current) clearTimeout(timer.current);
    },
    [],
  );

  async function poll(id: string) {
    try {
      const next = await getExperimentRun(id);
      setRun(next);
      if (next.status === "running") {
        timer.current = setTimeout(() => poll(id), POLL_MS);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function start() {
    setError(null);
    setStarting(true);
    try {
      const { id } = await startExperiment(experiment.name);
      await poll(id);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setStarting(false);
    }
  }

  const running = starting || run?.status === "running";
  const statusClass = run?.status === "passed" ? "ok" : run?.status === "running" ? "warn" : "bad";

  return (
    <article className="card">
      <h3>{experiment.name}</h3>
      <div className="meta">
        {experiment.fault} on {experiment.target} for {experiment.duration} · {experiment.path}
      </div>
      <p>{experiment.hypothesis}</p>
      <div className="controls">
        <button onClick={start} disabled={!runsEnabled || running} title={runsEnabled ? undefined : "Start the API with -enable-runs"}>
          {running ? "Running…" : "Run experiment"}
        </button>
        {!runsEnabled && <span className="legend">Runs are disabled (start the API with -enable-runs).</span>}
        {run && <span className={`status ${statusClass}`}>{run.status.toUpperCase()}</span>}
      </div>
      {error && (
        <div className="notice" role="alert">
          {error}
        </div>
      )}
      {run?.error && <div className="notice">{run.error}</div>}
      {run && run.log && <pre className="log">{run.log}</pre>}
      {run?.report && (
        <table>
          <thead>
            <tr>
              <th>Phase</th>
              <th>Probe</th>
              <th>Result</th>
              <th>Detail</th>
            </tr>
          </thead>
          <tbody>
            {run.report.steps.map((s, i) => (
              <tr key={`${s.probe}-${i}`}>
                <td>{s.phase}</td>
                <td>{s.probe}</td>
                <td>
                  <span className={`status ${s.passed ? "ok" : "bad"}`}>{s.passed ? "PASS" : "FAIL"}</span>
                </td>
                <td>{s.detail}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </article>
  );
}
