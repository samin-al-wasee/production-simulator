"use client";

import { useState } from "react";
import type { LearningStatus } from "@/lib/api";
import { completeExercise } from "@/lib/client";
import { evidenceLabel, percent } from "@/lib/progress";

export function LearningPath({ initial }: { initial: LearningStatus }) {
  const [status, setStatus] = useState(initial);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState<string | null>(null);

  async function complete(id: string) {
    setBusy(id);
    setError(null);
    try {
      setStatus(await completeExercise(id));
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(null);
    }
  }

  return (
    <>
      <p className="lead">
        <strong>
          {status.done} of {status.total}
        </strong>{" "}
        exercises complete ({percent(status.done, status.total)}%).
        {status.next && (
          <>
            {" "}
            Next: <code>{status.next}</code>.
          </>
        )}
      </p>
      <progress value={status.done} max={status.total} style={{ width: "100%" }} aria-label="Overall progress" />
      {error && (
        <div className="notice" role="alert">
          {error}
        </div>
      )}
      {status.stages.map((stage) => (
        <section key={stage.id} className="card">
          <h3>
            {stage.title}{" "}
            <span className={`status ${stage.complete ? "ok" : "warn"}`}>
              {stage.done}/{stage.total}
            </span>
          </h3>
          <div className="meta">{stage.goal}</div>
          <table>
            <tbody>
              {stage.exercises.map((e) => (
                <tr key={e.id}>
                  <td style={{ width: 28 }}>{e.done ? <span className="status ok">✓</span> : "○"}</td>
                  <td>
                    <strong>{e.title}</strong>
                    <div className="legend">
                      <code>{e.how}</code> · {evidenceLabel(e.evidence)}
                      {e.done && e.completedAt && ` · done ${e.completedAt.slice(0, 10)} (${e.completedBy})`}
                    </div>
                  </td>
                  <td className="num">
                    {!e.done && (
                      <button className="secondary" onClick={() => complete(e.id)} disabled={busy === e.id}>
                        Mark complete
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </section>
      ))}
    </>
  );
}
