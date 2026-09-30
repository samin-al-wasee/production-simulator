"use client";

import { Handle, Position, type Node, type NodeProps } from "@xyflow/react";
import { formatCompact, level, type NodeStats } from "@/lib/sandbox";

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
  const internet = data.kind === "internet";
  const util = s?.utilization ?? 0;
  const cls = data.down || data.downReplicas > 0 ? "bad" : level(util);
  return (
    <div className={`sb-node lvl-${cls}${selected ? " selected" : ""}${internet ? " internet" : ""}`}>
      {data.target && <Handle type="target" position={Position.Left} />}
      <div className="sb-node-title">{data.label}</div>
      <div className="sb-node-meta">
        {internet ? (data.loadTest ? "traffic source · load test" : "traffic source") : `${id} · ${data.size}${data.replicas > 1 ? ` ×${data.replicas}` : ""}`}
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
      {data.down && <div className="sb-node-meta bad">DOWN</div>}
      {!data.down && data.downReplicas > 0 && (
        <div className="sb-node-meta bad">
          {data.replicas - data.downReplicas}/{data.replicas} replicas up
        </div>
      )}
      {data.rateLimited && <div className="sb-node-meta">rate-limited</div>}
      {data.source && <Handle type="source" position={Position.Right} />}
    </div>
  );
}
