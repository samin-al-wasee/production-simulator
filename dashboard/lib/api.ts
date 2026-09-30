// Server-side client for the forgelab core API. The dashboard only renders
// what the API returns; it never recomputes simulation results.

export const API_URL = process.env.FORGELAB_API_URL ?? "http://127.0.0.1:8090";

export interface PipelineInfo {
  name: string;
  path: string;
  stages: string[];
}

export interface LearningExercise {
  id: string;
  title: string;
  how: string;
  evidence: { type: "manual" };
  done: boolean;
  completedAt?: string;
  completedBy?: string;
}

export interface LearningStage {
  id: string;
  title: string;
  goal: string;
  done: number;
  total: number;
  complete: boolean;
  exercises: LearningExercise[];
}

export interface LearningStatus {
  stages: LearningStage[];
  done: number;
  total: number;
  next: string;
}

export type Result<T> = { ok: true; data: T } | { ok: false; error: string };

async function get<T>(path: string): Promise<T> {
  const res = await fetch(`${API_URL}/api/v1${path}`, { cache: "no-store" });
  if (!res.ok) {
    let detail = res.statusText;
    try {
      detail = ((await res.json()) as { error?: string }).error ?? detail;
    } catch {
      // keep the status text
    }
    throw new Error(`${res.status} ${detail}`);
  }
  return (await res.json()) as T;
}

// load never throws: pages render an explanatory notice when the API is down.
export async function load<T>(path: string): Promise<Result<T>> {
  try {
    return { ok: true, data: await get<T>(path) };
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    return { ok: false, error: message.includes("fetch failed") ? `forgelab API unreachable at ${API_URL}` : message };
  }
}
