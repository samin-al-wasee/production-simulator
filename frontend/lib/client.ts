// Browser-side helpers; calls go through the /api/forgelab rewrite.

export interface PipelineEvent {
  at: number;
  stage: string;
  message: string;
}

export interface PipelineRun {
  pipeline: string;
  status: "succeeded" | "failed" | "rolled-back";
  duration: number;
  stages: {
    name: string;
    status: "succeeded" | "failed" | "skipped" | "rolled-back";
    start: number;
    end: number;
    steps?: { name: string; status: string; attempts: number; start: number; end: number }[];
  }[];
  events: PipelineEvent[];
}

// UnauthorizedError marks a 401: the caller is anonymous where the API needs a
// session, so the UI shows a sign-in prompt rather than a raw error (ADR-0033).
export class UnauthorizedError extends Error {}

export async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`/api/forgelab${path}`, init);
  const body = (await res.json().catch(() => ({}))) as T & { error?: string };
  if (res.status === 401) throw new UnauthorizedError(body.error ?? "sign in to continue");
  if (!res.ok) throw new Error(body.error ?? `${res.status} ${res.statusText}`);
  return body;
}

export interface PipelineOptions {
  seed: number;
  warmCache: boolean;
  badRelease: boolean;
  failStep: string;
}

export function simulatePipeline(name: string, options: PipelineOptions): Promise<PipelineRun> {
  return request(`/pipelines/${encodeURIComponent(name)}/runs`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(options),
  });
}

export function completeExercise(id: string): Promise<import("./api").LearningStatus> {
  return request(`/learning/${encodeURIComponent(id)}/complete`, { method: "POST" });
}
