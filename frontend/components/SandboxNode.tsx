"use client";

import { Handle, Position, type Node, type NodeProps } from "@xyflow/react";
import { formatCompact, healthLevel, level, type NodeStats } from "@/lib/sandbox";

export type SandboxNodeData = {
  label: string;
  kind: string;
  size: string;
  replicas: number;
  downReplicas: number;
  down: boolean;
  rateLimited: boolean;
  loadTest: boolean;
  source: boolean;
  target: boolean;
  stats?: NodeStats;
};

export type SandboxFlowNode = Node<SandboxNodeData, "component">;

export function SandboxNode({ id, data, selected }: NodeProps<SandboxFlowNode>) {
  const s = data.stats;
  // The Internet and traffic components are sources: no size, no capacity.
  const internet = data.kind === "internet" || data.kind === "traffic";
  const problem = s?.traffic?.problem;
  const util = s?.utilization ?? 0;
  const health = s?.app?.health ?? s?.db?.health ?? s?.cache?.health ?? s?.storage?.health ?? s?.queue?.health;
  // A traffic component is coloured by how its requests fare.
  const failing = !!s?.traffic && s.traffic.success < s.traffic.rps * 0.99;
  const cls = data.down || data.downReplicas > 0 || problem ? "bad" : health ? healthLevel(health) : s?.traffic ? (failing ? "warn" : "ok") : level(util);
  return (
    <div className={`sb-node lvl-${cls}${selected ? " selected" : ""}${internet ? " internet" : ""}`}>
      {data.target && <Handle type="target" position={Position.Left} />}
      <div className="sb-node-title">{data.label}</div>
      <div className="sb-node-meta">
        {internet
          ? `${data.kind === "traffic" ? `${id} · ` : ""}${data.loadTest ? "traffic source · load test" : "traffic source"}`
          : `${id} · ${data.size}${data.replicas > 1 ? ` ×${data.replicas}` : ""}`}
      </div>
      {s && (
        <>
          {!internet && (
            <div className="sb-util" title={`utilization ${(util * 100).toFixed(0)}%`}>
              <span className={cls} style={{ width: `${Math.min(util, 1) * 100}%` }} />
            </div>
          )}
          <div className="sb-node-meta">
            {formatCompact(s.served)}/s
            {!internet && ` · ${s.latencyMs.toFixed(0)} ms`}
            {s.dropped > 0.01 && <span className="bad"> · {formatCompact(s.dropped)}/s dropped</span>}
            {!!s.attack && s.attack > 0.01 && <span className="bad"> · {formatCompact(s.attack)}/s attack</span>}
            {!!s.backlog && ` · ${formatCompact(s.backlog)} queued`}
          </div>
        </>
      )}
      {problem && <div className="sb-node-meta bad">{problem}</div>}
      {data.down && <div className="sb-node-meta bad">DOWN</div>}
      {!data.down && data.downReplicas > 0 && (
        <div className="sb-node-meta bad">
          {data.replicas - data.downReplicas}/{data.replicas} replicas up
        </div>
      )}
      {data.rateLimited && <div className="sb-node-meta">rate-limited</div>}
      {health && health !== "healthy" && <div className={`sb-node-meta ${cls}`}>{health}</div>}
      {data.source && <Handle type="source" position={Position.Right} />}
    </div>
  );
}
