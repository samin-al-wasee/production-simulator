// Browser-side helpers; calls go through the /api/forgelab rewrite.

export interface RunStep {
  probe: string;
  phase: string;
  status: number;
  passed: boolean;
  detail: string;
}

export interface ExperimentRun {
  id: string;
  experiment: string;
  status: "running" | "passed" | "failed" | "error";
  log: string;
  error?: string;
  report?: { experiment: string; steps: RunStep[]; reverted: boolean; passed: boolean };
  startedAt: string;
  finishedAt?: string;
}

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

export async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`/api/forgelab${path}`, init);
  const body = (await res.json().catch(() => ({}))) as T & { error?: string };
  if (!res.ok) throw new Error(body.error ?? `${res.status} ${res.statusText}`);
  return body;
}

export function startExperiment(name: string): Promise<{ id: string }> {
  return request(`/experiments/${encodeURIComponent(name)}/runs`, { method: "POST" });
}

export function getExperimentRun(id: string): Promise<ExperimentRun> {
  return request(`/experiment-runs/${encodeURIComponent(id)}`);
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
